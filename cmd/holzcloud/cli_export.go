package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/export"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
	"github.com/holzcloud/holzcloud-cms/internal/wording"
)

// cmdExport writes one website as plain files.
//
// It builds the same router the server runs and asks it, so there is no second
// renderer to keep in step with the first. What it leaves out is the session
// store on disk (sessions live in memory for the length of the run and nobody
// signs in), CSRF (nothing is posted) and the periodic jobs.
func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	site := fs.String("website", "", "website id or one of its domains")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *site == "" || fs.NArg() != 1 {
		return errors.New("usage: holzcloud export -website <id|domain> <empty-directory>")
	}
	dir := fs.Arg(0)

	cfg, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()
	// The log goes to stderr at warning level: the report below is the
	// output, and a line per request would bury it.
	slog.SetDefault(config.NewLogger("warn"))

	ctx := context.Background()
	domainStore := domain.NewStore(database)
	ws, host, err := exportTarget(ctx, domainStore, *site)
	if err != nil {
		return err
	}

	handler, closeFn, err := exportHandler(ctx, cfg, database, domainStore)
	if err != nil {
		return err
	}
	defer closeFn()

	started := time.Now()
	report, err := export.Run(ctx, export.Options{Handler: handler, Host: host, Dir: dir})
	if err != nil {
		return err
	}
	printExportReport(os.Stdout, ws, host, dir, report, time.Since(started))
	return nil
}

// exportTarget finds the website and the host name its pages are asked for
// under. The primary domain, because a website that redirects to its
// canonical host would otherwise answer every request with a redirect.
func exportTarget(ctx context.Context, store *domain.Store, site string) (*domain.Website, string, error) {
	var ws *domain.Website
	if id, err := strconv.ParseInt(site, 10, 64); err == nil {
		ws, err = store.GetWebsite(ctx, id)
		if err != nil {
			return nil, "", err
		}
	} else {
		ws, err = store.LookupDomain(ctx, strings.ToLower(site))
		if err != nil {
			return nil, "", err
		}
	}
	if ws == nil {
		return nil, "", fmt.Errorf("no website %q", site)
	}
	if !ws.Active {
		return nil, "", fmt.Errorf("website %q is switched off — an export would be its maintenance page", ws.Name)
	}
	host, err := store.PrimaryDomain(ctx, ws.ID)
	if err != nil {
		return nil, "", err
	}
	if host == "" {
		domains, err := store.ListDomains(ctx, ws.ID)
		if err != nil {
			return nil, "", err
		}
		if len(domains) == 0 {
			return nil, "", fmt.Errorf("website %q has no domain, so nothing answers for it", ws.Name)
		}
		host = domains[0].Domain
	}
	return ws, host, nil
}

// exportHandler wires the router the way main() does, minus what an export
// never reaches. The public side is built inside newRouter itself, so every
// store and setting it gets there it gets here too.
func exportHandler(ctx context.Context, cfg config.Config, database *db.DB, domainStore *domain.Store) (http.Handler, func(), error) {
	i18n.SetDir(filepath.Join(cfg.DataDir, i18n.DirName))

	adminTemplatesFS, err := fs.Sub(staticFS, "templates/admin")
	if err != nil {
		return nil, nil, err
	}
	adminTmpl, err := web.ParseAdminTemplates(adminTemplatesFS)
	if err != nil {
		return nil, nil, fmt.Errorf("parse admin templates: %w", err)
	}
	publicDefaultFS, err := fs.Sub(staticFS, "templates/public/default")
	if err != nil {
		return nil, nil, err
	}
	publicFS, err := fs.Sub(staticFS, "templates/public")
	if err != nil {
		return nil, nil, err
	}

	sm := scs.New()
	sm.Store = memstore.New()

	csrfKey, err := auth.LoadOrGenerateCSRFKey(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	domainResolver := domain.NewResolver(domainStore)
	pageStore := page.NewStore(database)
	tmplStore := tmplmgr.NewStore(database, cfg.DataDir)
	menuStore := menu.NewStore(database)
	albumStore := album.NewStore(database)
	mediaStore := media.NewStore(database)
	snippetStore := snippet.NewStore(database)
	termStore := term.NewStore(database)
	productStore := shop.NewStore(database)
	cartStore := shop.NewCartStore(productStore)
	orderStore := shop.NewOrderStore(cartStore)
	shareSigner := sharelink.New(append([]byte("share:"), csrfKey...))
	unlockSigner := sharelink.New(append([]byte("unlock:"), csrfKey...))
	templateLoader := tmpl.NewLoader(cfg.DataDir, publicDefaultFS, publicFS, tmplStore)
	templateLoader.SetWording(wording.NewStore(database))

	adminHandler := admin.NewHandler(database, sm, adminTmpl, argon2ParamsFor(cfg), domainStore, domainResolver, pageStore, tmplStore, menuStore, mediaStore, snippetStore, termStore, shareSigner, templateLoader, &cfg, auth.NewLoginThrottle(10, 100, 15*time.Minute), web.NewClientIPResolver(cfg.TrustedProxies))
	adminHandler.SetAlbumStore(albumStore)

	closeFn := func() {}
	var pluginManager *plugin.Manager
	var pluginRT *plugin.Runtime
	pluginStore := plugin.NewStore(database)
	if rt, err := plugin.NewRuntime(ctx, pluginStore, slog.Default()); err != nil {
		slog.Warn("plugin runtime unavailable — plugin pages are left out", "err", err)
	} else {
		rt.WithSettings(pluginSettings(domainStore))
		if m, err := plugin.NewManager(ctx, pluginStore, rt, cfg.DataDir, slog.Default()); err != nil {
			slog.Warn("plugin manager unavailable — plugin pages are left out", "err", err)
			rt.Close(ctx)
		} else {
			pluginManager, pluginRT = m, rt
			closeFn = func() { rt.Close(context.Background()) }
		}
	}

	passthrough := func(next http.Handler) http.Handler { return next }
	handler, err := newRouter(routerDeps{
		cfg:             cfg,
		database:        database,
		sm:              sm,
		adminHandler:    adminHandler,
		csrfMiddleware:  passthrough,
		setupGuard:      passthrough,
		domainResolver:  domainResolver,
		domainStore:     domainStore,
		pageStore:       pageStore,
		menuStore:       menuStore,
		albumStore:      albumStore,
		mediaStore:      mediaStore,
		snippetStore:    snippetStore,
		termStore:       termStore,
		productStore:    productStore,
		cartStore:       cartStore,
		orderStore:      orderStore,
		shareSigner:     shareSigner,
		unlockSigner:    unlockSigner,
		pluginRT:        pluginRT,
		plugins:         pluginManager,
		templateLoader:  templateLoader,
		publicDefaultFS: publicDefaultFS,
		clientIP:        web.NewClientIPResolver(cfg.TrustedProxies),
	})
	if err != nil {
		closeFn()
		return nil, nil, err
	}
	return handler, closeFn, nil
}

func printExportReport(w io.Writer, ws *domain.Website, host, dir string, r *export.Report, took time.Duration) {
	fmt.Fprintf(w, "exported %q (%s) to %s: %d files in %s\n", ws.Name, host, dir, len(r.Files), took.Round(time.Millisecond))
	if len(r.Forms) > 0 {
		fmt.Fprintf(w, "\n%d pages carry a form, which does nothing without the server:\n", len(r.Forms))
		for _, u := range r.Forms {
			fmt.Fprintf(w, "  %s\n", u)
		}
	}
	if len(r.Skipped) > 0 {
		fmt.Fprintf(w, "\n%d addresses were not exported:\n", len(r.Skipped))
		for _, s := range r.Skipped {
			fmt.Fprintf(w, "  %s — %s\n", s.URL, s.Reason)
		}
	}
	fmt.Fprintln(w, "\nNot in a static export: search, forms, protected pages, the shop's cart and checkout.")
}
