package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	gorillacsrf "github.com/gorilla/csrf"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/jobs"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/public"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

//go:embed assets templates
var staticFS embed.FS

// Version and Commit are injected at build time:
//
//	go build -ldflags "-X main.Version=$(git describe --tags) -X main.Commit=$(git rev-parse --short HEAD)"
//
// Without them "which build is running on the server?" has no answer at all.
var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	// Subcommands run without an HTTP server. This is the recovery path: a
	// locked-out operator has no way back through the web interface, because
	// /admin/setup 404s once a user exists and there is no reset route.
	if handled, err := runCLI(os.Args); handled {
		if err != nil {
			// An empty message means the subcommand has already written its
			// own report and only needs the exit status — `template check`
			// prints a list of problems, and "error: " under it says nothing.
			if err.Error() != "" {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}
		return
	}

	cfg, cfgErr := config.Load()

	logger := config.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	// Report every bad value at once rather than silently falling back: a typo in
	// an Argon2 parameter used to weaken password hashing with no symptom.
	if cfgErr != nil {
		slog.Error("invalid configuration", "err", cfgErr)
		os.Exit(1)
	}
	slog.Info("starting", "version", Version, "commit", Commit)
	// The sidebar shows the version and the source-code address. See
	// web.SetBuild: for a modified version running as a service the AGPL
	// requires that its users can get at the source.
	web.SetBuild(Version, os.Getenv("HOLZCLOUD_SOURCE_URL"))
	slog.Info("configuration loaded", "config", cfg)

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		slog.Error("cannot create data dir", "err", err, "path", cfg.DataDir)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("cannot open database", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer database.Close()

	// Snapshot before migrating, so a half-applied migration is a two-command
	// recovery instead of a restore from last night's backup.
	var preUpgradeSnapshot string
	if pending, err := db.HasPendingMigrations(database.Write); err != nil {
		slog.Warn("cannot determine pending migrations", "err", err)
	} else if pending {
		name := fmt.Sprintf("pre-upgrade-%s-%s.sqlite", Version, time.Now().UTC().Format("20060102-150405"))
		snapshot, err := db.Backup(context.Background(), database, filepath.Join(cfg.DataDir, name))
		if err != nil {
			slog.Error("cannot snapshot before migrating", "err", err)
			os.Exit(1)
		}
		preUpgradeSnapshot = snapshot
		slog.Info("pre-upgrade snapshot written", "path", snapshot)
		if err := db.PruneSnapshots(cfg.DataDir, 5); err != nil {
			slog.Warn("cannot prune old snapshots", "err", err)
		}
	}

	if err := db.RunMigrations(database.Write); err != nil {
		slog.Error("migrations failed", "err", err)
		if preUpgradeSnapshot != "" {
			slog.Error("restore with: systemctl stop holzcloud && cp " +
				preUpgradeSnapshot + " " + cfg.DBPath + " && systemctl start holzcloud")
		}
		os.Exit(1)
	}

	// Report corruption without refusing to start: a partly readable database is
	// still serviceable while the operator restores it, and /readyz carries the
	// verdict so the problem is visible rather than silent.
	integrity, err := db.QuickCheck(context.Background(), database.Read)
	if err != nil {
		slog.Error("integrity check failed to run", "err", err)
		integrity = "unknown"
	} else if integrity != "ok" {
		slog.Error("database integrity check reported a problem", "result", integrity)
	}

	// Languages from disk before the templates are parsed: one set is built per
	// language, and a language that only turned up afterwards would have none.
	i18n.SetDir(filepath.Join(cfg.DataDir, i18n.DirName))

	// The name this installation carries. Read once and then kept in memory: it
	// stands on every screen, and one query per page view for a single word
	// would be one query too many.
	branding.SetDir(filepath.Join(cfg.DataDir, branding.DirName))
	branding.Load(context.Background(), database.Read)

	// Parse admin templates
	adminTemplatesFS, err := fs.Sub(staticFS, "templates/admin")
	if err != nil {
		slog.Error("cannot sub admin templates FS", "err", err)
		os.Exit(1)
	}
	adminTmpl, err := web.ParseAdminTemplates(adminTemplatesFS)
	if err != nil {
		slog.Error("cannot parse admin templates", "err", err)
		os.Exit(1)
	}

	// Session manager
	sessionStore := auth.NewSQLiteStore(database.Write)
	sm := auth.NewSessionManager(sessionStore, cfg.Secure)

	// CSRF middleware
	csrfKey, err := auth.LoadOrGenerateCSRFKey(cfg.DataDir)
	if err != nil {
		slog.Error("cannot load CSRF key", "err", err)
		os.Exit(1)
	}
	csrfProtect := gorillacsrf.Protect(csrfKey,
		gorillacsrf.Secure(cfg.Secure),
		gorillacsrf.Path("/admin"),
		gorillacsrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("CSRF validation failed", "path", r.URL.Path, "method", r.Method, "reason", gorillacsrf.FailureReason(r))
			http.Error(w, "Forbidden - CSRF validation failed", http.StatusForbidden)
		})),
	)
	// Wrap CSRF: set plaintext context BEFORE csrf.Protect runs validation
	csrfMiddleware := func(next http.Handler) http.Handler {
		protected := csrfProtect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Secure {
				r = gorillacsrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}

	// Domain and page stores
	domainStore := domain.NewStore(database)
	domainResolver := domain.NewResolver(domainStore)

	// The second half of the provisioning refusal, against the database rather
	// than the environment, and beside the other start-up checks rather than
	// inside newRouter — newRouter returns an error and is under test, and a
	// fatal exit does not belong in a function a test calls.
	if err := checkSSOWebsites(context.Background(), cfg, domainStore); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	pageStore := page.NewStore(database)

	// Template, menu, and media stores
	tmplStore := tmplmgr.NewStore(database, cfg.DataDir)
	menuStore := menu.NewStore(database)
	albumStore := album.NewStore(database)
	mediaStore := media.NewStore(database)
	snippetStore := snippet.NewStore(database)
	termStore := term.NewStore(database)
	productStore := shop.NewStore(database)
	cartStore := shop.NewCartStore(productStore)
	orderStore := shop.NewOrderStore(cartStore)
	// The outbox for the order confirmations. Its mail account is the core's
	// (see mailSender further down); with no account configured the outbox
	// stays put and the shop keeps working, only nobody hears about an order.
	outboxStore := outbox.NewStore(database)
	// Two signers with distinct labels: a preview token must never open a
	// protected page, and an unlock cookie must never show a draft.
	shareSigner := sharelink.New(append([]byte("share:"), csrfKey...))
	unlockSigner := sharelink.New(append([]byte("unlock:"), csrfKey...))
	// Public template loader (created early so admin handler can invalidate cache)
	publicDefaultFS, err := fs.Sub(staticFS, "templates/public/default")
	if err != nil {
		slog.Error("cannot sub public templates FS", "err", err)
		os.Exit(1)
	}
	publicFS, err := fs.Sub(staticFS, "templates/public")
	if err != nil {
		slog.Error("cannot sub public FS", "err", err)
		os.Exit(1)
	}
	templateLoader := tmpl.NewLoader(cfg.DataDir, publicDefaultFS, publicFS, tmplStore)

	// Seed built-in templates into DB (idempotent — marks existing as built-in)
	for _, bt := range tmpl.BuiltinTemplates {
		if _, err := tmplStore.CreateBuiltin(context.Background(), bt.Name, bt.Slug); err != nil {
			slog.Error("seed builtin template", "slug", bt.Slug, "err", err)
		}
	}

	// Admin handler
	argon2Params := auth.Argon2Params{
		Memory:      cfg.Argon2Memory,
		Iterations:  cfg.Argon2Iterations,
		Parallelism: cfg.Argon2Parallelism,
		SaltLength:  16,
		KeyLength:   32,
	}
	// Login throttling per 15 minutes: 10 failures from one client address, and
	// 100 against one account. The account limit is deliberately loose — a strict
	// one would let anyone lock out a known admin address on demand.
	loginThrottle := auth.NewLoginThrottle(10, 100, 15*time.Minute)
	clientIP := web.NewClientIPResolver(cfg.TrustedProxies)

	admin.SetVersion(Version)
	adminHandler := admin.NewHandler(database, sm, adminTmpl, argon2Params, domainStore, domainResolver, pageStore, tmplStore, menuStore, mediaStore, snippetStore, termStore, shareSigner, templateLoader, &cfg, loginThrottle, clientIP)
	// Not a nineteenth argument to NewHandler: the constructor is positional
	// and already eighteen long, and a new one would edit every call site
	// including cmd/holzcloud/main_test.go.
	adminHandler.SetAlbumStore(albumStore)
	adminHandler.SetProductStore(productStore)
	adminHandler.SetOrderStore(orderStore)
	adminHandler.SetOutbox(outboxStore)
	adminHandler.SetPayments(&payrexx.Client{
		Instance: cfg.PayrexxInstance,
		Secret:   cfg.PayrexxSecret,
		BaseURL:  cfg.PayrexxBaseURL,
	})

	// Plugins: storage, runtime, manager. A failure here is not fatal — a
	// server serving four websites should not fall over the plugin system,
	// which perhaps nobody uses. Without a manager the admin simply has no
	// plugin pages.
	// The outbox. Without HOLZCLOUD_SMTP_HOST it sends nothing, and then
	// everything behaves as before: an invitation link stands on the screen and
	// is passed on by hand.
	mailSender := mail.NewSender(mail.Config{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Password: cfg.SMTPPassword,
		From: cfg.SMTPFrom, FromName: cfg.SMTPFromName, TLS: cfg.SMTPTLS,
	})
	mailQueue := mail.NewQueue(database, mailSender, slog.Default())
	adminHandler.SetMail(mailQueue)

	// The connection for an AI assistant. It comes in from outside, with a key
	// somebody issued in the admin — this server calls nobody.
	aiTokens := ai.NewStore(database)
	adminHandler.SetAITokens(aiTokens)
	// The activity log. It hangs off nothing but the database, which is why it
	// stands here and not further down among the services that need each other.
	adminHandler.SetActivityStore(activity.NewStore(database))
	aiServer := ai.NewServer(aiTokens, "Holzcloud CMS", slog.Default(), ai.Tools(ai.Deps{
		Domains: domainStore, Pages: pageStore, Media: mediaStore, Fields: field.NewStore(database),
	}))

	pluginStore := plugin.NewStore(database)
	var pluginManager *plugin.Manager
	// Outside the block, because two of the host functions need the public
	// handler, which does not exist yet here. They are supplied further down; a
	// plugin only calls them inside a hook, so never before then.
	var pluginRT *plugin.Runtime
	if pluginRuntime, err := plugin.NewRuntime(context.Background(), pluginStore, slog.Default()); err != nil {
		slog.Error("plugin runtime unavailable", "err", err)
	} else {
		// A plugin may read a website's settings if it has the permission.
		// Handed in as a function, so that the plugin package does not depend
		// on the domain package and the two stay separately testable.
		pluginRuntime.WithSettings(func(ctx context.Context, websiteID int64) (plugin.SettingsResult, error) {
			ws, err := domainStore.GetWebsite(ctx, websiteID)
			if err != nil || ws == nil {
				return plugin.SettingsResult{}, fmt.Errorf("website %d not found", websiteID)
			}
			return plugin.SettingsResult{
				WebsiteID: ws.ID, Name: ws.Name, Description: ws.Description,
				Locale: ws.Locale, TimeZone: ws.TimeZone, BlogBase: ws.BlogBase,
				ContactEmail: ws.ContactEmail,
			}, nil
		})
		if m, err := plugin.NewManager(context.Background(), pluginStore, pluginRuntime, cfg.DataDir, slog.Default()); err != nil {
			slog.Error("plugin manager unavailable", "err", err)
			pluginRuntime.Close(context.Background())
		} else {
			pluginManager = m
			pluginRT = pluginRuntime
			adminHandler.SetPlugins(m)
			defer pluginRuntime.Close(context.Background())
		}
	}
	// Setup guard middleware: redirects to /admin/setup if no users, returns 404 on /admin/setup if users exist.
	// Once the first user exists the state can never go back, so the positive
	// result is latched and the per-request COUNT(*) disappears.
	var usersExist atomic.Bool
	setupGuard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exists := usersExist.Load()
			if !exists {
				var err error
				exists, err = admin.HasUsers(r.Context(), database)
				if err != nil {
					slog.Error("setup guard: cannot check users", "err", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if exists {
					usersExist.Store(true)
				}
			}
			isSetupPath := r.URL.Path == "/admin/setup"
			if !exists && !isSetupPath {
				http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
				return
			}
			if exists && isSetupPath {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Who may enter which website is needed by both the switcher and the router.
	userStore := user.NewStore(database, argon2Params)

	readiness := web.NewReadinessProbe(Version, Commit, integrity)
	readiness.MinFreeBytes = cfg.MinFreeBytes
	readiness.Ping = func(ctx context.Context) error {
		if err := database.Read.PingContext(ctx); err != nil {
			return err
		}
		return database.Write.PingContext(ctx)
	}
	readiness.Stats = func(ctx context.Context) (map[string]int64, error) {
		stats, err := db.Stats(ctx, database)
		if err != nil {
			return nil, err
		}
		return map[string]int64{
			"size_bytes":     stats.SizeBytes,
			"wal_bytes":      stats.WALBytes,
			"page_count":     stats.PageCount,
			"freelist_count": stats.FreelistCount,
		}, nil
	}
	readiness.FreeBytes = func() (uint64, error) { return web.FreeBytes(cfg.DataDir) }
	readiness.WriteProbe = func() error { return web.WriteProbe(cfg.DataDir) }

	handler, err := newRouter(routerDeps{
		cfg:             cfg,
		database:        database,
		sm:              sm,
		adminHandler:    adminHandler,
		csrfMiddleware:  csrfMiddleware,
		setupGuard:      setupGuard,
		domainResolver:  domainResolver,
		domainStore:     domainStore,
		userStore:       userStore,
		pageStore:       pageStore,
		menuStore:       menuStore,
		albumStore:      albumStore,
		mediaStore:      mediaStore,
		snippetStore:    snippetStore,
		termStore:       termStore,
		productStore:    productStore,
		cartStore:       cartStore,
		orderStore:      orderStore,
		outboxStore:     outboxStore,
		shareSigner:     shareSigner,
		unlockSigner:    unlockSigner,
		pluginRT:        pluginRT,
		mailQueue:       mailQueue,
		aiServer:        aiServer,
		templateLoader:  templateLoader,
		publicDefaultFS: publicDefaultFS,
		readiness:       readiness,
		clientIP:        clientIP,
		plugins:         pluginManager,
	})
	if err != nil {
		slog.Error("cannot build router", "err", err)
		os.Exit(1)
	}

	// Outermost: baseline security headers, then the SCS session middleware.
	srv := &http.Server{
		// net.JoinHostPort and not ":" + Port: it brackets IPv6 correctly, so
		// HOLZCLOUD_LISTEN=:: becomes [::]:8080 rather than a parse error.
		// One place decides what this process binds, and cfg.LogValue reports
		// it, so an operator who cannot reach the port has the answer in
		// journalctl instead of a guess.
		Addr:    net.JoinHostPort(cfg.Listen, cfg.Port),
		Handler: handler,
		// ReadTimeout covers the whole body in net/http, so a 5s value silently
		// aborted large uploads on a home connection. Slowloris on headers is
		// what the short timeout was actually meant for — that is ReadHeaderTimeout.
		// Total upload size stays bounded by MaxBytesReader in the handlers.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// One runner for every periodic task, so they all honour the shutdown
	// context instead of each carrying its own stop channel.
	runner := jobs.New(
		jobs.Job{
			Name:  "login-throttle-cleanup",
			Every: 15 * time.Minute,
			Fn:    func(context.Context) error { loginThrottle.Cleanup(); return nil },
		},
		jobs.Job{
			Name:  "session-cleanup",
			Every: 5 * time.Minute,
			Fn:    func(ctx context.Context) error { return sessionStore.DeleteExpired(ctx) },
		},
		jobs.Job{
			// Images that stand in the database without their dimensions get
			// them filled in afterwards — along with the scaled copies.
			//
			// The upload path measures every image; the path through an
			// imported archive never did. On a website built that way no image
			// has a width and a height in the HTML, the layout jumps while
			// loading, and there is no srcset — a phone loads every original at
			// full size.
			//
			// At start-up and hourly afterwards: the start brings an existing
			// installation into order, the rhythm catches what a later import
			// leaves behind. Once nothing is missing any more, the run is a
			// query with no hits.
			Name:       "media-backfill",
			Every:      time.Hour,
			RunAtStart: true,
			Fn: func(ctx context.Context) error {
				done, failed, err := media.Backfill(ctx, mediaStore, cfg.DataDir, cfg.MaxMegapixels, 100)
				if done > 0 || failed > 0 {
					slog.Info("media backfill", "ergaenzt", done, "fehlgeschlagen", failed)
				}
				return err
			},
		},
		jobs.Job{
			Name:  "database-maintenance",
			Every: 24 * time.Hour,
			Fn:    func(ctx context.Context) error { return db.Maintain(ctx, database) },
		},
		jobs.Job{
			// Often, because an invitation arriving a minute later is fine, and
			// one arriving an hour later is not.
			Name:  "mail-send",
			Every: 30 * time.Second,
			Fn:    mailQueue.Flush,
		},
		jobs.Job{
			Name: "outbox-dispatch",
			// A minute is the compromise: the confirmation should arrive while
			// the customer is still at the screen, but a mail server does not
			// need to be knocked on every second.
			Every: time.Minute,
			Fn:    (&outbox.Dispatcher{Store: outboxStore, Sender: outboxSender{mailSender}}).Run,
		},
		jobs.Job{
			Name:  "outbox-prune",
			Every: 24 * time.Hour,
			Fn: func(ctx context.Context) error {
				// Nur Zugestelltes. Ein Fehlschlag bleibt stehen, bis jemand
				// hingesehen hat.
				_, err := outboxStore.Prune(ctx, 90*24*time.Hour)
				return err
			},
		},
		jobs.Job{
			Name:  "mail-prune",
			Every: 12 * time.Hour,
			Fn:    mailQueue.Prune,
		},
		jobs.Job{
			Name:  "token-purge",
			Every: 6 * time.Hour,
			Fn: func(ctx context.Context) error {
				_, err := user.NewStore(database, argon2Params).PurgeExpiredTokens(ctx)
				return err
			},
		},
		jobs.Job{
			Name:  "csv-import-prune",
			Every: 6 * time.Hour,
			Fn: func(ctx context.Context) error {
				// Uploaded tables lying between the four screens of the
				// CSV import. Finished and abandoned alike: the row of a
				// finished import has already been cleared away, so what
				// this run finds is what somebody left standing.
				//
				// One day is the retention. Whoever leaves the tab open
				// over lunch still finds their import; whoever goes away
				// for a weekend does not — and uploading a file a second
				// time costs nothing, while ten megabytes per abandoned
				// attempt cost something in the long run.
				//
				// No RunAtStart, unlike media-backfill. A sweep at startup
				// would catch precisely the import that is running during
				// a deploy.
				//
				// And here stands the answer to the planning note that
				// argued against staging: "needs no sweep". The sweep is
				// this one here — eight lines and no new machinery. In
				// return the file can be carried across four screens,
				// which without it would not work at all, because a server
				// cannot fill in a file field.
				_, err := csvimport.NewStore(database).Prune(ctx, 24*time.Hour)
				return err
			},
		},
		jobs.Job{
			// Deleted pages are recoverable for a while and then really gone,
			// so the database does not keep growing with content nobody kept.
			Name:  "trash-purge",
			Every: 6 * time.Hour,
			Fn: func(ctx context.Context) error {
				n, err := pageStore.PurgeExpiredTrash(ctx, page.TrashRetention)
				if err != nil {
					return err
				}
				if n > 0 {
					slog.Info("purged expired trash", "pages", n)
				}
				return nil
			},
		},
		jobs.Job{
			Name:       "integrity-check",
			Every:      24 * time.Hour,
			RunAtStart: false,
			Fn: func(ctx context.Context) error {
				result, err := db.QuickCheck(ctx, database.Read)
				if err != nil {
					return err
				}
				readiness.SetIntegrity(result)
				if result != "ok" {
					slog.Error("database integrity check reported a problem", "result", result)
				}
				return nil
			},
		},
	)
	runner.Start(ctx)

	go func() {
		slog.Info("server starting", "addr", srv.Addr, "log_level", cfg.LogLevel)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("shutting down", "timeout_s", 10)
	runner.Wait()
	// Fold the WAL back before closing so the next start does not have to.
	if err := db.Maintain(shutdownCtx, database); err != nil {
		slog.Warn("shutdown maintenance failed", "err", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("shutdown complete")
}

// websiteLookup is the one thing checkSSOWebsites needs from the domain
// store, named as an interface so the check can be tested without a router, a
// session manager or a template set.
type websiteLookup interface {
	GetWebsite(ctx context.Context, id int64) (*domain.Website, error)
}

// checkSSOWebsites refuses to start when single sign-on names a website that
// does not exist: the default website of provisioning, or the website of any
// group in HOLZCLOUD_SSO_WEBSITE_GROUPS.
//
// config.Load refuses what the environment alone can answer. This is the half
// only the database can: an id nobody ever created satisfies every check in
// config.go. It is asked at start-up and not at the first sign-in, because at a
// sign-in the answer is a refused person and an INSERT failing on the foreign
// key — and before migration 00052, when "no rows" still meant "every website",
// an assignment the failure left empty. The default website has been checked
// here since plan 10-04; the website groups were not, until the Phase 10 code
// review found it (CR-02).
func checkSSOWebsites(ctx context.Context, cfg config.Config, websites websiteLookup) error {
	if cfg.SSOProvision {
		ws, err := websites.GetWebsite(ctx, cfg.SSODefaultWebsite)
		if err != nil {
			return fmt.Errorf("HOLZCLOUD_SSO_DEFAULT_WEBSITE=%d could not be checked against the database: %w",
				cfg.SSODefaultWebsite, err)
		}
		if ws == nil {
			return fmt.Errorf("HOLZCLOUD_SSO_DEFAULT_WEBSITE=%d names a website that does not exist: "+
				"an account provisioned into nothing is an account nobody can place",
				cfg.SSODefaultWebsite)
		}
	}
	if !cfg.SSOEnabled || len(cfg.SSOWebsiteGroups) == 0 {
		return nil
	}

	groups := make([]string, 0, len(cfg.SSOWebsiteGroups))
	for group := range cfg.SSOWebsiteGroups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	var missing []string
	for _, group := range groups {
		id := cfg.SSOWebsiteGroups[group]
		ws, err := websites.GetWebsite(ctx, id)
		if err != nil {
			return fmt.Errorf("HOLZCLOUD_SSO_WEBSITE_GROUPS: %s=%d could not be checked against the database: %w",
				group, id, err)
		}
		if ws == nil {
			missing = append(missing, fmt.Sprintf("%s=%d", group, id))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("HOLZCLOUD_SSO_WEBSITE_GROUPS names websites that do not exist: %s — "+
			"everybody in such a group would be turned away at sign-in without being told why",
			strings.Join(missing, ", "))
	}
	return nil
}

// routerDeps are everything newRouter needs. Bundling them keeps the signature
// stable as routes grow and lets the route-table test build the same handler
// main() serves.
type routerDeps struct {
	cfg            config.Config
	database       *db.DB
	sm             *scs.SessionManager
	adminHandler   *admin.Handler
	csrfMiddleware func(http.Handler) http.Handler
	setupGuard     func(http.Handler) http.Handler
	domainResolver *domain.Resolver
	domainStore    *domain.Store
	// userStore answers who may enter which website. Nil would mean the
	// restriction does not bite, so it is always set here.
	userStore *user.Store
	pageStore *page.Store
	menuStore *menu.Store
	// albumStore is read twice: the album routes reach it through adminHandler,
	// and the public handler expands a page's album markers with it at request
	// time (plan 11-05).
	albumStore      *album.Store
	mediaStore      *media.Store
	snippetStore    *snippet.Store
	termStore       *term.Store
	productStore    *shop.Store
	cartStore       *shop.CartStore
	orderStore      *shop.OrderStore
	outboxStore     *outbox.Store
	shareSigner     *sharelink.Signer
	unlockSigner    *sharelink.Signer
	templateLoader  *tmpl.Loader
	publicDefaultFS fs.FS
	readiness       *web.ReadinessProbe
	clientIP        *web.ClientIPResolver
	// plugins may be nil: then there are no hooks and no plugin pages, and
	// everything behaves as it did before the plugin system.
	plugins *plugin.Manager
	// pluginRT gets the host functions that need the public handler here. Nil
	// when the runtime did not come up.
	pluginRT  *plugin.Runtime
	mailQueue *mail.Queue
	// aiServer answers MCP under /ai. Nil would mean no connection for an
	// assistant, and then the address does not exist.
	aiServer *ai.Server
}

// pluginNavLinks turns the manager's entries into what the sidebar shows.
//
// A plugin screen belongs beside the rest of the menu, not behind the plugin
// list: whoever reads the enquiries looks for them under content, and the list
// of installed plugins is a screen for administrators.
func pluginNavLinks(m *plugin.Manager) func(int64) []web.NavLink {
	if m == nil {
		return nil
	}
	return func(websiteID int64) []web.NavLink {
		var out []web.NavLink
		for _, l := range m.AdminLinks() {
			url := "/admin/plugins/" + l.PluginID + "/bildschirm"
			if l.PerWebsite {
				if websiteID == 0 {
					continue
				}
				url = fmt.Sprintf("/admin/websites/%d/plugins/%s", websiteID, l.PluginID)
			}
			out = append(out, web.NavLink{Label: l.Label, URL: url, AdminOnly: l.AdminOnly})
		}
		return out
	}
}

// newRouter builds the fully wired handler.
//
// It is separate from main() so cmd/holzcloud can be tested at all: the missing
// requireAdmin on website deletion shipped unnoticed precisely because there was
// no test here that could see the route table.
func newRouter(d routerDeps) (http.Handler, error) {
	cfg := d.cfg
	database := d.database
	sm := d.sm
	adminHandler := d.adminHandler
	csrfMiddleware := d.csrfMiddleware
	setupGuard := d.setupGuard
	domainResolver := d.domainResolver
	pageStore := d.pageStore
	menuStore := d.menuStore
	mediaStore := d.mediaStore
	snippetStore := d.snippetStore
	termStore := d.termStore
	productStore := d.productStore
	cartStore := d.cartStore
	orderStore := d.orderStore
	templateLoader := d.templateLoader
	publicDefaultFS := d.publicDefaultFS
	_ = database

	requireAuth := auth.RequireAuth(sm, admin.NewUserLookup(database))
	// Runs inside RequireAuth: by then the session has a user, and an account
	// that owes a second factor is sent to the setup page before it reaches
	// anything else.
	requireSecondFactor := auth.RequireSecondFactor(sm, admin.NewSecondFactorLookup(database), cfg.SSOEnabled)
	// Site-level and template administration is admin-only; editors keep full
	// access to content (pages, menus, media).
	requireAdmin := auth.RequireAdmin(sm)
	// Whoever is responsible for one website only gets into that one only. One
	// check over the whole admin rather than in sixty handlers — see
	// RequireWebsiteAccess.
	requireWebsite := auth.RequireWebsiteAccess(sm, admin.NewWebsiteAccessLookup(database))
	// Before the few buttons that destroy something no backup brings back: the
	// password once more. Deliberately few — a confirmation that comes up
	// everywhere is one nobody reads any more.
	requireFresh := auth.RequireFreshPassword(sm)

	mux := http.NewServeMux()

	// Liveness: a constant 200 is correct here — systemd only needs to know the
	// process is answering at all.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	// Readiness: whether this process can actually serve.
	if d.readiness != nil {
		mux.HandleFunc("GET /readyz", d.readiness.Handler())
	}

	// Static assets
	assetsFS, err := fs.Sub(staticFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("sub assets FS: %w", err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))

	// The connection for an AI assistant.
	//
	// Outside the CSRF protection and outside the session, and both
	// deliberately: what signs in here is not a browser but a program with a
	// key in the request header. A form cannot set that header, so another
	// site cannot call this address in a signed-in user's name either — the
	// hole CSRF otherwise protects against does not exist here.
	if d.aiServer != nil {
		mux.Handle("/ai", d.aiServer)
	}

	// Admin routes — public (CSRF but no auth)
	adminPublicMux := http.NewServeMux()
	adminPublicMux.HandleFunc("GET /admin/login", adminHandler.ErrHandler(adminHandler.HandleLoginForm))
	adminPublicMux.HandleFunc("POST /admin/login", adminHandler.ErrHandler(adminHandler.HandleLogin))
	// The code prompt sits on the public mux: a session that has passed the
	// password but not the code deliberately has no user, so RequireAuth would
	// bounce it straight back to the login form it just came from.
	adminPublicMux.HandleFunc("GET /admin/2fa", adminHandler.ErrHandler(adminHandler.HandleTwoFactorVerify))
	adminPublicMux.HandleFunc("POST /admin/2fa", adminHandler.ErrHandler(adminHandler.HandleTwoFactorVerify))
	// One-time links. Someone following a reset link is by definition not
	// logged in, so these sit on the public admin mux.
	for _, purpose := range []struct{ path, kind string }{
		{"/admin/activate/{token}", "invite"},
		{"/admin/reset/{token}", "reset"},
	} {
		handler := adminHandler.ErrHandler(adminHandler.HandleSetPassword(purpose.kind))
		adminPublicMux.HandleFunc("GET "+purpose.path, handler)
		adminPublicMux.HandleFunc("POST "+purpose.path, handler)
	}
	adminPublicMux.HandleFunc("GET /admin/setup", adminHandler.ErrHandler(adminHandler.HandleSetupForm))
	adminPublicMux.HandleFunc("POST /admin/setup", adminHandler.ErrHandler(adminHandler.HandleSetup))

	// Admin routes — protected (CSRF + auth)
	adminProtectedMux := http.NewServeMux()
	adminProtectedMux.HandleFunc("POST /admin/logout", adminHandler.ErrHandler(adminHandler.HandleLogout))
	adminProtectedMux.HandleFunc("GET /admin/bestaetigen", adminHandler.ErrHandler(adminHandler.HandleConfirmPassword))
	adminProtectedMux.HandleFunc("POST /admin/bestaetigen", adminHandler.ErrHandler(adminHandler.HandleConfirmPassword))
	adminProtectedMux.HandleFunc("GET /admin/konto", adminHandler.ErrHandler(adminHandler.HandleAccount))
	adminProtectedMux.HandleFunc("POST /admin/konto/sprache", adminHandler.ErrHandler(adminHandler.HandleAccountLanguage))
	adminProtectedMux.HandleFunc("GET /admin/2fa/einrichten", adminHandler.ErrHandler(adminHandler.HandleTwoFactorSetup))
	adminProtectedMux.HandleFunc("POST /admin/2fa/einrichten", adminHandler.ErrHandler(adminHandler.HandleTwoFactorSetup))
	adminProtectedMux.HandleFunc("POST /admin/2fa/einrichten/neu", adminHandler.ErrHandler(adminHandler.HandleTwoFactorRestart))
	adminProtectedMux.HandleFunc("POST /admin/2fa/codes", adminHandler.ErrHandler(adminHandler.HandleRecoveryCodes))
	adminProtectedMux.HandleFunc("POST /admin/2fa/aus", adminHandler.ErrHandler(adminHandler.HandleTwoFactorDisable))
	adminProtectedMux.HandleFunc("GET /admin/", adminHandler.ErrHandler(adminHandler.HandleDashboard))

	// Website routes
	adminProtectedMux.HandleFunc("GET /admin/websites", adminHandler.ErrHandler(adminHandler.HandleWebsiteList))
	adminProtectedMux.Handle("GET /admin/websites/new", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteCreate))))
	adminProtectedMux.Handle("POST /admin/websites/new", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteCreate))))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}", adminHandler.ErrHandler(adminHandler.HandleWebsiteEdit))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}", adminHandler.ErrHandler(adminHandler.HandleWebsiteEdit))
	adminProtectedMux.Handle("POST /admin/websites/{id}/delete", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteDelete)))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/domains", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleDomainAdd))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/domains/{domainID}/delete", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleDomainRemove))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/domains/{domainID}/primary", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleDomainSetPrimary))))

	// Page routes
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages", adminHandler.ErrHandler(adminHandler.HandlePageList))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/uebersetzungen", adminHandler.ErrHandler(adminHandler.HandleTranslations))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages/new", adminHandler.ErrHandler(adminHandler.HandlePageCreate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/new", adminHandler.ErrHandler(adminHandler.HandlePageCreate))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages/{pageID}/edit", adminHandler.ErrHandler(adminHandler.HandlePageEdit))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/edit", adminHandler.ErrHandler(adminHandler.HandlePageEdit))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/delete", adminHandler.ErrHandler(adminHandler.HandlePageDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/status", adminHandler.ErrHandler(adminHandler.HandlePageStatusToggle))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages/{pageID}/edit-title", adminHandler.ErrHandler(adminHandler.HandlePageInlineEditTitle))
	adminProtectedMux.HandleFunc("PUT /admin/websites/{id}/pages/{pageID}/title", adminHandler.ErrHandler(adminHandler.HandlePageInlineEditSave))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/preview", adminHandler.ErrHandler(adminHandler.HandlePagePreview))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/bulk", adminHandler.ErrHandler(adminHandler.HandlePageBulk))
	// The list as one particular person needs it: their own columns and
	// remembered filters. Both belong to them, not to the website.
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/spalten", adminHandler.ErrHandler(adminHandler.HandlePageColumns))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/ansichten", adminHandler.ErrHandler(adminHandler.HandleSavedViewCreate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/ansichten/{viewID}/loeschen", adminHandler.ErrHandler(adminHandler.HandleSavedViewDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/share", adminHandler.ErrHandler(adminHandler.HandlePageShare))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/uebersetzen", adminHandler.ErrHandler(adminHandler.HandlePageTranslate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/duplicate", adminHandler.ErrHandler(adminHandler.HandlePageDuplicate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/review", adminHandler.ErrHandler(adminHandler.HandlePageReview))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/insert-media", adminHandler.ErrHandler(adminHandler.HandlePageInsertMedia))

	// Version history
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages/{pageID}/revisions", adminHandler.ErrHandler(adminHandler.HandlePageRevisions))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/revisions/{revID}/restore", adminHandler.ErrHandler(adminHandler.HandlePageRevisionRestore))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages/{pageID}/revisions/vergleich", adminHandler.ErrHandler(adminHandler.HandlePageRevisionCompare))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/pages/{pageID}/revisions/{revID}/beschriften", adminHandler.ErrHandler(adminHandler.HandlePageRevisionLabel))

	// Reusable text blocks
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/snippets", adminHandler.ErrHandler(adminHandler.HandleSnippetList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/snippets", adminHandler.ErrHandler(adminHandler.HandleSnippetList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/snippets/{snippetID}/delete", adminHandler.ErrHandler(adminHandler.HandleSnippetDelete))

	// Redirects
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/redirects", adminHandler.ErrHandler(adminHandler.HandleRedirectList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/redirects", adminHandler.ErrHandler(adminHandler.HandleRedirectList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/redirects/{redirectID}/delete", adminHandler.ErrHandler(adminHandler.HandleRedirectDelete))

	// Trash
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/trash", adminHandler.ErrHandler(adminHandler.HandleTrash))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/trash/{pageID}/restore", adminHandler.ErrHandler(adminHandler.HandleTrashRestore))
	adminProtectedMux.Handle("POST /admin/websites/{id}/trash/{pageID}/purge", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTrashPurge))))

	// Preview routes (renders public site without domain resolution)
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/preview", adminHandler.ErrHandler(adminHandler.HandlePreview))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/preview/t/{path...}", adminHandler.ErrHandler(adminHandler.HandlePreviewAsset))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/preview/{slug}", adminHandler.ErrHandler(adminHandler.HandlePreviewPage))

	// Design/template routes (per-website)
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/design", adminHandler.ErrHandler(adminHandler.HandleWebsiteDesign))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/design/activate", adminHandler.ErrHandler(adminHandler.HandleWebsiteDesignActivate))

	// Shop: Produktverwaltung und Einstellungen je Website
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/produkte", adminHandler.ErrHandler(adminHandler.HandleProductList))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/produkte/{productID}", adminHandler.ErrHandler(adminHandler.HandleProductForm))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/produkte/{productID}", adminHandler.ErrHandler(adminHandler.HandleProductForm))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/produkte/{productID}/delete", adminHandler.ErrHandler(adminHandler.HandleProductDelete))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/bestellungen", adminHandler.ErrHandler(adminHandler.HandleOrderList))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/bestellungen/{number}", adminHandler.ErrHandler(adminHandler.HandleOrderDetail))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/bestellungen/{number}", adminHandler.ErrHandler(adminHandler.HandleOrderDetail))
	// Invoice and delivery note for printing. A page of its own without the
	// admin's navigation, which has no business on paper.
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/bestellungen/{number}/{kind}", adminHandler.ErrHandler(adminHandler.HandleOrderDocument))
	adminProtectedMux.Handle("GET /admin/websites/{id}/shop", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleShopSettings))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/shop", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleShopSettings))))

	// Menu routes
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/menus", adminHandler.ErrHandler(adminHandler.HandleMenuList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus", adminHandler.ErrHandler(adminHandler.HandleMenuCreate))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/menus/{menuID}", adminHandler.ErrHandler(adminHandler.HandleMenuEdit))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/update", adminHandler.ErrHandler(adminHandler.HandleMenuUpdate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/delete", adminHandler.ErrHandler(adminHandler.HandleMenuDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items", adminHandler.ErrHandler(adminHandler.HandleMenuItemCreate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/update", adminHandler.ErrHandler(adminHandler.HandleMenuItemUpdate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/delete", adminHandler.ErrHandler(adminHandler.HandleMenuItemDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/reorder", adminHandler.ErrHandler(adminHandler.HandleMenuItemReorder))

	// The album area. Nine routes, the same nine menus has and for the same
	// reason: a named parent with an ordered child list needs list, create,
	// edit, update and delete, and four more on the children.
	//
	// The segment is "albums", English, like pages, media, menus, tags and
	// snippets — the five content areas an editor reaches from the same
	// navigation group. The three German ones (produkte, bestellungen,
	// uebersetzungen) came later and belong to the shop and the translation
	// matrix. Nothing here is stored, so the GLOSSARY's stored-value rule has
	// nothing to bite on and no bookmark exists to break.
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/albums", adminHandler.ErrHandler(adminHandler.HandleAlbumList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums", adminHandler.ErrHandler(adminHandler.HandleAlbumCreate))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/albums/{albumID}", adminHandler.ErrHandler(adminHandler.HandleAlbumEdit))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/update", adminHandler.ErrHandler(adminHandler.HandleAlbumUpdate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/delete", adminHandler.ErrHandler(adminHandler.HandleAlbumDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures", adminHandler.ErrHandler(adminHandler.HandleAlbumItemCreate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/update", adminHandler.ErrHandler(adminHandler.HandleAlbumItemUpdate))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/delete", adminHandler.ErrHandler(adminHandler.HandleAlbumItemDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/reorder", adminHandler.ErrHandler(adminHandler.HandleAlbumItemReorder))

	// Media routes
	adminProtectedMux.Handle("GET /admin/websites/{id}/export", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteExport))))
	adminProtectedMux.Handle("POST /admin/websites/import", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteImport))))
	adminProtectedMux.Handle("POST /admin/websites/import-wordpress", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWordPressImport))))
	// The CSV import. Screen 1 stands here beside its two siblings; every
	// screen with a token lies under /admin/csv-import/{token}, because
	// /admin/websites/import-csv/{token} collides with
	// GET /admin/websites/{id}/pages and would make newRouter crash at start-up
	// (D-36).
	adminProtectedMux.Handle("POST /admin/websites/import-csv", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleCSVImport))))
	adminProtectedMux.Handle("GET /admin/csv-import/{token}", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleCSVMapping))))
	adminProtectedMux.Handle("POST /admin/csv-import/{token}/probe", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleCSVDryRun))))
	adminProtectedMux.Handle("POST /admin/csv-import/{token}/start", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleCSVStart))))
	// The example file does NOT hang off the token (D-37): it helps in writing
	// the file and therefore has to be reachable before anything is uploaded. A
	// GET like all five existing downloads (D-34).
	adminProtectedMux.Handle("GET /admin/csv-vorlage", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleCSVExample))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/design/tokens", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteTokens))))
	// Fields of one's own. Whoever changes them changes what this website's
	// pages are made of — that is an administrator's business, not an editor's.
	// Content kinds of one's own. Like the fields, a screen for whoever
	adminProtectedMux.Handle("GET /admin/websites/{id}/inhaltsarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindList))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindSave))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten/{kindID}/loeschen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindDelete))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten/{kindID}/verschieben", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindMove))))

	// Block kinds of one's own. Reserved for the administrator like the content
	// kinds: a block kind is a promise to every theme of this website.
	adminProtectedMux.Handle("GET /admin/websites/{id}/bausteinarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBlockTypeList))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/bausteinarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBlockTypeSave))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/bausteinarten/{typeID}/loeschen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBlockTypeDelete))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/bausteinarten/{typeID}/verschieben", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBlockTypeMove))))
	adminProtectedMux.Handle("GET /admin/websites/{id}/felder", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleFieldList))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/felder", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleFieldSave))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/felder/{fieldID}/loeschen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleFieldDelete))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/felder/{fieldID}/verschieben", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleFieldMove))))

	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/tags", adminHandler.ErrHandler(adminHandler.HandleTermList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/tags/{termID}/rename", adminHandler.ErrHandler(adminHandler.HandleTermRename))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/tags/{termID}/delete", adminHandler.ErrHandler(adminHandler.HandleTermDelete))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/media", adminHandler.ErrHandler(adminHandler.HandleMediaList))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/media/upload", adminHandler.ErrHandler(adminHandler.HandleMediaUpload))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/media/{mediaID}/delete", adminHandler.ErrHandler(adminHandler.HandleMediaDelete))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/media/{mediaID}/meta", adminHandler.ErrHandler(adminHandler.HandleMediaMeta))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/media/{mediaID}/zuschnitt", adminHandler.ErrHandler(adminHandler.HandleMediaCrop))
	adminProtectedMux.HandleFunc("POST /admin/websites/{id}/media/{mediaID}/zuschnitt", adminHandler.ErrHandler(adminHandler.HandleMediaCropSave))
	adminProtectedMux.HandleFunc("GET /admin/websites/{id}/media/picker", adminHandler.ErrHandler(adminHandler.HandleMediaPicker))

	// User routes (admin-only except self password change)
	adminProtectedMux.Handle("GET /admin/users", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserList))))
	adminProtectedMux.Handle("GET /admin/users/new", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserCreate))))
	adminProtectedMux.Handle("POST /admin/users/new", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserCreate))))
	adminProtectedMux.Handle("GET /admin/users/{id}/edit", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserEdit))))
	adminProtectedMux.Handle("POST /admin/users/{id}/edit", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserEdit))))
	adminProtectedMux.Handle("POST /admin/users/{id}/delete", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserDelete)))))
	adminProtectedMux.Handle("POST /admin/users/{id}/link", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserLink))))
	adminProtectedMux.Handle("POST /admin/users/{id}/sessions/revoke", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleUserSessions))))
	adminProtectedMux.HandleFunc("GET /admin/users/{id}/password", adminHandler.ErrHandler(adminHandler.HandlePasswordChange))
	adminProtectedMux.HandleFunc("POST /admin/users/{id}/password", adminHandler.ErrHandler(adminHandler.HandlePasswordChange))

	// Template routes
	// Plugins. Installing, switching on and off and removing are an
	// administrator's business: it is code that runs on the server, and that is
	// not an editorial decision.
	// For administrators, not for editors: the mail server's address stands
	// here, and although the test send only goes to yourself it says whether
	// the setup stands.
	adminProtectedMux.Handle("GET /admin/mail", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailStatus))))
	adminProtectedMux.Handle("POST /admin/mail/test", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailTest))))
	adminProtectedMux.Handle("POST /admin/mail/retry", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailRetry))))

	// AI access. Issuing a key means allowing a program on somebody else's
	// machine to write on this website — that is an administrator's business
	// and not an editorial decision.
	adminProtectedMux.Handle("GET /admin/ai", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeys))))
	adminProtectedMux.Handle("POST /admin/ai/keys", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeyCreate)))))
	adminProtectedMux.Handle("POST /admin/ai/keys/{id}/revoke", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeyRevoke))))
	adminProtectedMux.Handle("GET /admin/plugins", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginList))))
	adminProtectedMux.Handle("POST /admin/plugins/upload", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginUpload))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/enable", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginEnable))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/websites", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginWebsites))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/remove", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginRemove)))))
	// A plugin's own screen is open to editors too, unless the manifest says
	// otherwise: content is maintained there.
	adminProtectedMux.HandleFunc("GET /admin/plugins/{id}/bildschirm", adminHandler.ErrHandler(adminHandler.HandlePluginScreen))
	adminProtectedMux.HandleFunc("POST /admin/plugins/{id}/bildschirm", adminHandler.ErrHandler(adminHandler.HandlePluginScreen))
	adminProtectedMux.HandleFunc("GET /admin/websites/{websiteID}/plugins/{id}", adminHandler.ErrHandler(adminHandler.HandlePluginScreen))
	adminProtectedMux.HandleFunc("POST /admin/websites/{websiteID}/plugins/{id}", adminHandler.ErrHandler(adminHandler.HandlePluginScreen))

	adminProtectedMux.HandleFunc("GET /admin/templates", adminHandler.ErrHandler(adminHandler.HandleTemplateList))
	adminProtectedMux.Handle("GET /admin/templates/upload", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateUpload))))
	adminProtectedMux.Handle("POST /admin/templates/upload", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateUpload))))
	// The authoring specification as plain text, so an admin can hand the whole
	// contract to an AI agent by copying one page rather than by finding the
	// project's source.
	adminProtectedMux.Handle("GET /admin/templates/spec", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateSpec))))
	adminProtectedMux.Handle("POST /admin/templates/{id}/activate", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateActivate))))
	adminProtectedMux.Handle("POST /admin/templates/{id}/deactivate", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateDeactivate))))
	adminProtectedMux.Handle("POST /admin/templates/{id}/delete", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleTemplateDelete))))

	// The admin's languages. Administrators only: a language file affects every
	// screen of every user.
	// The installation's brand: name, mark, logo.
	adminProtectedMux.Handle("GET /admin/protokoll", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleActivityList))))
	// Sweeping removes the trace of the other actions. It is therefore the one
	// place in the log that asks for the password again.
	adminProtectedMux.Handle("POST /admin/protokoll/aufraeumen", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleActivityPurge)))))
	adminProtectedMux.Handle("GET /admin/marke", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBranding))))
	adminProtectedMux.Handle("POST /admin/marke", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleBranding))))
	adminProtectedMux.Handle("GET /admin/sprachen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguages))))
	adminProtectedMux.Handle("POST /admin/sprachen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguageUpload))))
	adminProtectedMux.Handle("POST /admin/sprachen/neu-lesen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguageReload))))
	adminProtectedMux.Handle("GET /admin/sprachen/vorlage", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguageTemplate))))
	adminProtectedMux.Handle("GET /admin/sprachen/{code}/datei", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguageDownload))))
	adminProtectedMux.Handle("POST /admin/sprachen/{code}/loeschen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleLanguageDelete))))

	// Wire middleware: admin security headers wrap CSRF, setupGuard wraps both,
	// RequireAuth wraps protected only.
	// Public admin: headers -> CSRF -> setupGuard -> handler
	// The language sits right on the outside, before signing in too: the
	// sign-in screen is exactly the place where an unreadable language is worst
	// — from there no path leads to a setting. Nobody knows a user here, so
	// Accept-Language decides.
	adminPublic := i18n.Middleware(nil)(web.AdminHeaders(csrfMiddleware(setupGuard(adminPublicMux))))
	mux.Handle("/admin/login", adminPublic)
	mux.Handle("/admin/login/", adminPublic)
	// Exact path only — /admin/2fa/einrichten stays behind the auth chain.
	mux.Handle("/admin/2fa", adminPublic)
	mux.Handle("/admin/activate/", adminPublic)
	mux.Handle("/admin/reset/", adminPublic)
	mux.Handle("/admin/setup", adminPublic)
	mux.Handle("/admin/setup/", adminPublic)
	// The navigation needs the same two things on every screen: which websites
	// there are and which one is being worked on. Fetched here once rather than
	// in thirty handlers — otherwise the sidebar shows a website's sections
	// only where the handler happens to know one.
	// Inside, behind requireAuth: whoever is not signed in needs no list.
	var listWebsites func(context.Context) ([]domain.Website, error)
	if d.domainStore != nil {
		// Not the bare list: the switcher shows only what this person may
		// actually enter — otherwise every second entry leads to a 403.
		listWebsites = admin.NewNavWebsiteList(sm, d.domainStore, d.userStore)
	}
	withNav := web.WithNav(sm, listWebsites, pluginNavLinks(d.plugins))

	// The signed-in person's own language. Inside requireAuth, because before
	// that nobody knows who is reading; whoever has chosen nothing gets what
	// the browser brings along again.
	//
	// The language belongs to the person, not to the website: a German editor
	// and an English-speaking developer work on the same website, and both
	// should see their own admin.
	withLang := i18n.Middleware(admin.NewLanguageLookup(sm, database))

	// Protected admin: headers -> CSRF -> setupGuard -> forward auth -> requireAuth -> language -> website access -> nav -> handler
	//
	// Forward auth signs a session in and authorises nothing. Its position is
	// the whole design: outside requireAuth, so the session already carries the
	// account by the time RequireAuth reads it, and inside setupGuard, so a
	// first-run installation still goes to the setup form rather than signing a
	// stranger in against an empty database. Everything behind it —
	// RequireAuth, RequireSecondFactor, RequireWebsiteAccess — runs exactly as
	// it did before it existed, and removing this one call is how the whole
	// feature is switched off in an emergency.
	mux.Handle("/admin/", web.AdminHeaders(csrfMiddleware(setupGuard(adminHandler.ForwardAuthSignIn(requireAuth(requireSecondFactor(withLang(requireWebsite(withNav(adminProtectedMux))))))))))

	// The installation's logo. Public like the assets: it stands on the sign-in
	// screen too, and whoever sees that may see the picture on it.
	mux.HandleFunc("GET /admin/marke/logo", adminHandler.ErrHandler(adminHandler.HandleBrandingLogo))

	// Media serve (public, no auth)
	mux.HandleFunc("GET /media/{websiteID}/{filename}", adminHandler.ErrHandler(adminHandler.HandleMediaServe))
	// A plugin's assets. A path of their own rather than /assets, so that a
	// plugin cannot shadow the core's files.
	mux.HandleFunc("GET /plugin-assets/{id}/{path...}", adminHandler.ErrHandler(adminHandler.HandlePluginAsset))

	// Public site handler and routes
	publicHandler := public.NewHandler(pageStore, menuStore, mediaStore, snippetStore, templateLoader, domainResolver, cfg.DataDir, publicDefaultFS, cfg.Secure)
	// The manager is built further up, because the admin already needs it; here
	// the public side gets it. If it is nil, everything behaves as it did
	// before the plugins.
	publicHandler.SetPlugins(d.plugins)
	// Two host functions need the public handler: reading pages and rendering a
	// page in the website's theme. Only here does it exist, and a plugin calls
	// them inside a hook at the earliest anyway.
	publicHandler.SetNotify(d.domainStore, d.mailQueue)
	if d.pluginRT != nil {
		d.pluginRT.WithPages(publicHandler.PagesForPlugin)
		d.pluginRT.WithRender(publicHandler.RenderForPlugin)
		d.pluginRT.WithNotify(publicHandler.NotifyForPlugin)
	}

	// The resolver answers for a deactivated website and cannot render a theme
	// itself, so the public handler supplies the maintenance page.
	domainResolver.Secure = cfg.Secure
	domainResolver.Offline = http.HandlerFunc(publicHandler.HandleMaintenance)
	publicHandler.SetTermStore(termStore)
	// Without this the public side holds a nil album store, every album marker
	// on every page expands to nothing, and the tests in internal/public still
	// pass because they wire their own.
	publicHandler.SetAlbumStore(d.albumStore)
	publicHandler.SetFieldStore(field.NewStore(database))
	publicHandler.SetKindStore(kind.NewStore(database))
	publicHandler.SetProductStore(productStore)
	publicHandler.SetCartStore(cartStore)
	publicHandler.SetOrderStore(orderStore)
	publicHandler.SetOutbox(d.outboxStore)
	// Built from the environment, not from a store: the key must not be in
	// anything that gets backed up. An empty pair leaves the client
	// unconfigured, which every payment path checks for.
	publicHandler.SetPayments(&payrexx.Client{
		Instance: cfg.PayrexxInstance,
		Secret:   cfg.PayrexxSecret,
		BaseURL:  cfg.PayrexxBaseURL,
	})
	publicHandler.SetShareSigners(d.shareSigner, d.unlockSigner)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("GET /t/{path...}", publicHandler.ErrHandler(publicHandler.HandleTemplateAsset))
	publicMux.HandleFunc("GET /sitemap.xml", publicHandler.ErrHandler(publicHandler.HandleSitemap))
	publicMux.HandleFunc("GET /robots.txt", publicHandler.ErrHandler(publicHandler.HandleRobots))
	publicMux.HandleFunc("GET /feed.xml", publicHandler.ErrHandler(publicHandler.HandleFeed))
	publicMux.HandleFunc("GET /tag/{slug}", publicHandler.ErrHandler(publicHandler.HandleTag))
	publicMux.HandleFunc("POST /freischalten", publicHandler.ErrHandler(publicHandler.HandleUnlock))
	publicMux.HandleFunc("GET /vorschau/{token}", publicHandler.ErrHandler(publicHandler.HandleShareLink))
	publicMux.HandleFunc("POST /preise", publicHandler.ErrHandler(publicHandler.HandlePriceSwitch))
	publicMux.HandleFunc("GET /warenkorb", publicHandler.ErrHandler(publicHandler.HandleCart))
	publicMux.HandleFunc("POST /warenkorb/hinzufuegen", publicHandler.ErrHandler(publicHandler.HandleCartAdd))
	publicMux.HandleFunc("POST /warenkorb/menge", publicHandler.ErrHandler(publicHandler.HandleCartUpdate))
	publicMux.HandleFunc("POST /warenkorb/entfernen", publicHandler.ErrHandler(publicHandler.HandleCartRemove))
	publicMux.HandleFunc("GET /kasse", publicHandler.ErrHandler(publicHandler.HandleCheckout))
	publicMux.HandleFunc("POST /kasse", publicHandler.ErrHandler(publicHandler.HandleCheckout))
	publicMux.HandleFunc("GET /bestellung/{number}", publicHandler.ErrHandler(publicHandler.HandleOrderConfirmation))
	publicMux.HandleFunc("GET /zahlung/zurueck/{number}", publicHandler.ErrHandler(publicHandler.HandlePaymentReturn))
	// The provider's notification. No CSRF token — it comes from Payrexx, not
	// from a browser, and nothing in its body is believed anyway.
	publicMux.HandleFunc("POST /zahlung/payrexx", publicHandler.ErrHandler(publicHandler.HandlePaymentHook))
	publicMux.HandleFunc("GET /{slug}", publicHandler.ErrHandler(publicHandler.HandlePage))
	publicMux.HandleFunc("GET /{$}", publicHandler.ErrHandler(publicHandler.HandleHome))

	// Public routes: domain resolver middleware wraps public mux
	// Registered AFTER admin routes so /admin/ takes priority
	// The plugin layer lies inside the resolver (so the website is known) and
	// outside the router (so a plugin can claim an address the core does not
	// know at all).
	// The language layer lies between resolver and plugins: it needs the
	// website (which prefixes are languages hangs off it), and a plugin should
	// see the same address under /fr/… as under /… — otherwise every plugin
	// would have to know about multilingualism itself.
	//
	// The shop layer sits innermost, right before the router: the catalogue's
	// address is a setting of the website and therefore does not stand in the
	// route table — as a pattern it would be "/{base}/{slug}" and would collide
	// with "/t/{path...}", which Go's mux answers at start-up with a panic.
	// Checking the first segment against the website's setting is what the call
	// really needs.
	mux.Handle("/", domainResolver.Middleware(public.LocaleMiddleware(publicHandler.PluginMiddleware(publicHandler.ShopRoutes(publicMux)))))

	// Outermost first: the forward-auth strip, then an id for every request,
	// then the access log (so even a panicking request produces a line), then
	// recovery, then the security headers and the session middleware.
	//
	// ForwardAuth is above RequestID and not below it because an identity
	// header a client wrote must not be visible even to the access log — and
	// because it is wrapped around the whole mux rather than around "/admin/",
	// stripping identity headers is a property of this binary and not of one
	// route prefix. The half that signs somebody in cannot live here: it needs
	// the session, which sm.LoadAndSave supplies further in. That is plan
	// 10-03, and it goes into the admin chain.
	// The payment provider is allowed as a form target only where it is set up.
	// An installation without keys keeps the policy it always had.
	var paymentOrigins []string
	if cfg.PayrexxInstance != "" && cfg.PayrexxSecret != "" {
		paymentOrigins = append(paymentOrigins, web.PaymentFormAction)
	}
	handler := web.SecureHeadersWith(web.PublicCSP(paymentOrigins...))(sm.LoadAndSave(mux))
	handler = web.Recoverer(handler)
	handler = web.AccessLog(d.clientIP)(handler)
	handler = web.RequestID(d.clientIP)(handler)
	handler = web.ForwardAuth(d.clientIP, web.ForwardAuthOptions{
		Enabled: cfg.SSOEnabled,
		Secret:  cfg.SSOSecret,
	})(handler)
	return handler, nil
}

// outboxSender lets the shop's outbox use the core's mail account.
//
// The two were written apart and ask for slightly different things: the outbox
// wants a context and calls the question "Configured", the sender takes no
// context and calls it "Enabled". Rather than bend either package to the other,
// the three lines that reconcile them live here, where both are already known.
type outboxSender struct{ s *mail.Sender }

func (a outboxSender) Configured() bool { return a.s != nil && a.s.Enabled() }

func (a outboxSender) Send(_ context.Context, m mail.Message) error { return a.s.Send(m) }
