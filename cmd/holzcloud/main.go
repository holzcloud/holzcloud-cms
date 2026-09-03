package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	gorillacsrf "github.com/gorilla/csrf"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/config"
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
	// Die Seitenleiste zeigt Fassung und Quelltextadresse. Siehe web.SetBuild:
	// bei einer veränderten Fassung, die als Dienst läuft, verlangt die AGPL,
	// dass die Benutzer an den Quelltext kommen.
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

	// Sprachen von der Platte, bevor die Vorlagen geparst werden: je Sprache
	// entsteht ein eigener Satz, und eine Sprache, die erst danach auftaucht,
	// hätte keinen.
	i18n.SetDir(filepath.Join(cfg.DataDir, i18n.DirName))

	// Der Name, den diese Anlage trägt. Einmal gelesen und danach im Speicher:
	// er steht auf jedem Bildschirm, und eine Abfrage je Seitenaufruf für ein
	// Wort wäre eine Abfrage zu viel.
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
	pageStore := page.NewStore(database)

	// Template, menu, and media stores
	tmplStore := tmplmgr.NewStore(database, cfg.DataDir)
	menuStore := menu.NewStore(database)
	mediaStore := media.NewStore(database)
	snippetStore := snippet.NewStore(database)
	termStore := term.NewStore(database)
	productStore := shop.NewStore(database)
	cartStore := shop.NewCartStore(productStore)
	orderStore := shop.NewOrderStore(cartStore)
	// Der Postausgang für die Bestellbestätigungen. Das Mailkonto dazu ist das
	// des Kerns (siehe mailSender weiter unten); ohne eingerichtetes Konto
	// bleibt der Postausgang stehen und der Shop funktioniert weiter, nur
	// erfährt niemand von einer Bestellung.
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
	adminHandler.SetProductStore(productStore)
	adminHandler.SetOrderStore(orderStore)
	adminHandler.SetOutbox(outboxStore)
	adminHandler.SetPayments(&payrexx.Client{
		Instance: cfg.PayrexxInstance,
		Secret:   cfg.PayrexxSecret,
		BaseURL:  cfg.PayrexxBaseURL,
	})

	// Plugins: Speicher, Laufzeit, Manager. Ein Fehler hier ist nicht tödlich —
	// ein Server, der vier Websites bedient, soll nicht am Plugin-System
	// scheitern, das vielleicht niemand benutzt. Ohne Manager hat die
	// Verwaltung schlicht keine Plugin-Seiten.
	// Der Postausgang. Ohne HOLZCLOUD_SMTP_HOST verschickt er nichts, und dann
	// verhält sich alles wie vorher: ein Einladungslink steht auf dem Bildschirm
	// und wird von Hand weitergegeben.
	mailSender := mail.NewSender(mail.Config{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Password: cfg.SMTPPassword,
		From: cfg.SMTPFrom, FromName: cfg.SMTPFromName, TLS: cfg.SMTPTLS,
	})
	mailQueue := mail.NewQueue(database, mailSender, slog.Default())
	adminHandler.SetMail(mailQueue)

	// Der Anschluss für einen KI-Assistenten. Er kommt von aussen herein, mit
	// einem Schlüssel, den jemand in der Verwaltung ausgestellt hat — dieser
	// Server ruft nirgends an.
	aiTokens := ai.NewStore(database)
	adminHandler.SetAITokens(aiTokens)
	// Das Protokoll. Es hängt an nichts ausser der Datenbank, deshalb steht es
	// hier und nicht weiter unten bei den Diensten, die einander brauchen.
	adminHandler.SetActivityStore(activity.NewStore(database))
	aiServer := ai.NewServer(aiTokens, "Holzcloud CMS", slog.Default(), ai.Tools(ai.Deps{
		Domains: domainStore, Pages: pageStore, Media: mediaStore, Fields: field.NewStore(database),
	}))

	pluginStore := plugin.NewStore(database)
	var pluginManager *plugin.Manager
	// Ausserhalb des Blocks, weil zwei der Host-Funktionen den öffentlichen
	// Handler brauchen, den es hier noch nicht gibt. Sie werden weiter unten
	// nachgereicht; ein Plugin ruft sie erst in einem Haken auf, also nie vorher.
	var pluginRT *plugin.Runtime
	if pluginRuntime, err := plugin.NewRuntime(context.Background(), pluginStore, slog.Default()); err != nil {
		slog.Error("plugin runtime unavailable", "err", err)
	} else {
		// Ein Plugin darf die Einstellungen einer Website lesen, wenn es die
		// Berechtigung hat. Als Funktion hineingereicht, damit das Plugin-Paket
		// nicht vom Domain-Paket abhängt und die beiden getrennt prüfbar bleiben.
		pluginRuntime.WithSettings(func(ctx context.Context, websiteID int64) (plugin.SettingsResult, error) {
			ws, err := domainStore.GetWebsite(ctx, websiteID)
			if err != nil || ws == nil {
				return plugin.SettingsResult{}, fmt.Errorf("website %d nicht gefunden", websiteID)
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

	// Wer welche Website betreten darf, brauchen Umschalter und Router.
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
		Addr:    ":" + cfg.Port,
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
			Name:  "database-maintenance",
			Every: 24 * time.Hour,
			Fn:    func(ctx context.Context) error { return db.Maintain(ctx, database) },
		},
		jobs.Job{
			// Häufig, weil eine Einladung, die eine Minute später ankommt, in
			// Ordnung ist, und eine, die eine Stunde später ankommt, nicht.
			Name:  "mail-send",
			Every: 30 * time.Second,
			Fn:    mailQueue.Flush,
		},
		jobs.Job{
			Name: "outbox-dispatch",
			// Eine Minute ist der Kompromiss: die Bestätigung soll ankommen,
			// solange die Kundin noch am Bildschirm sitzt, aber ein Mailserver
			// muss nicht im Sekundentakt angeklopft bekommen.
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
	// userStore beantwortet, wer welche Website betreten darf. Nil hiesse: die
	// Einschränkung greift nicht, also wird sie hier immer gesetzt.
	userStore       *user.Store
	pageStore       *page.Store
	menuStore       *menu.Store
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
	// plugins darf nil sein: dann gibt es keine Haken und keine Plugin-Seiten,
	// und alles verhält sich wie vor dem Plugin-System.
	plugins *plugin.Manager
	// pluginRT bekommt hier die Host-Funktionen, die den öffentlichen Handler
	// brauchen. Nil, wenn die Laufzeit nicht hochkam.
	pluginRT  *plugin.Runtime
	mailQueue *mail.Queue
	// aiServer beantwortet MCP unter /ai. Nil hiesse: kein Anschluss für einen
	// Assistenten, und die Adresse gibt es dann nicht.
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
	requireSecondFactor := auth.RequireSecondFactor(sm, admin.NewSecondFactorLookup(database))
	// Site-level and template administration is admin-only; editors keep full
	// access to content (pages, menus, media).
	requireAdmin := auth.RequireAdmin(sm)
	// Wer nur für eine Website zuständig ist, kommt auch nur dort hinein. Eine
	// Prüfung über der ganzen Verwaltung statt in sechzig Handlern — siehe
	// RequireWebsiteAccess.
	requireWebsite := auth.RequireWebsiteAccess(sm, admin.NewWebsiteAccessLookup(database))
	// Vor den wenigen Knöpfen, die etwas zerstören, das keine Sicherung
	// zurückholt: noch einmal das Passwort. Absichtlich wenige — eine
	// Rückfrage, die überall kommt, liest niemand mehr.
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

	// Der Anschluss für einen KI-Assistenten.
	//
	// Ausserhalb des CSRF-Schutzes und ausserhalb der Sitzung, und beides mit
	// Absicht: hier meldet sich kein Browser an, sondern ein Programm mit einem
	// Schlüssel im Kopf der Anfrage. Ein Formular kann diesen Kopf nicht setzen,
	// also kann eine fremde Seite diese Adresse auch nicht im Namen eines
	// angemeldeten Benutzers aufrufen — die Lücke, gegen die CSRF sonst schützt,
	// gibt es hier gar nicht.
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
	// Die Liste, wie eine bestimmte Person sie braucht: eigene Spalten und
	// gemerkte Filter. Beides gehört zu ihr, nicht zur Website.
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
	// Rechnung und Lieferschein zum Ausdrucken. Eine eigene Seite ohne die
	// Navigation des Admin-Bereichs, die auf Papier nichts verloren hat.
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

	// Media routes
	adminProtectedMux.Handle("GET /admin/websites/{id}/export", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteExport))))
	adminProtectedMux.Handle("POST /admin/websites/import", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteImport))))
	adminProtectedMux.Handle("POST /admin/websites/import-wordpress", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWordPressImport))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/design/tokens", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleWebsiteTokens))))
	// Eigene Felder. Wer sie ändert, ändert, woraus die Seiten dieser Website
	// bestehen — das ist Verwaltersache, nicht Redaktion.
	// Eigene Inhaltsarten. Wie die Felder ein Bildschirm für den, der die
	// Website einrichtet, nicht für den, der sie füllt.
	adminProtectedMux.Handle("GET /admin/websites/{id}/inhaltsarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindList))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindSave))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten/{kindID}/loeschen", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindDelete))))
	adminProtectedMux.Handle("POST /admin/websites/{id}/inhaltsarten/{kindID}/verschieben", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleKindMove))))

	// Eigene Bausteinarten. Wie die Inhaltsarten dem Administrator vorbehalten:
	// eine Bausteinart ist eine Zusage an jedes Theme dieser Website.
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
	// Plugins. Einspielen, Ein- und Ausschalten und Entfernen sind
	// Administratorensache: es ist Code, der auf dem Server läuft, und das ist
	// keine redaktionelle Entscheidung.
	// Für Verwalter, nicht für Redakteure: hier steht die Anschrift des
	// Mailservers, und der Testversand geht zwar nur an einen selbst, sagt aber
	// aus, ob die Einrichtung steht.
	adminProtectedMux.Handle("GET /admin/mail", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailStatus))))
	adminProtectedMux.Handle("POST /admin/mail/test", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailTest))))
	adminProtectedMux.Handle("POST /admin/mail/retry", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleMailRetry))))

	// KI-Zugang. Einen Schlüssel auszustellen heisst, einem Programm auf einem
	// fremden Rechner das Schreiben auf dieser Website zu erlauben — das ist
	// Verwaltersache und keine redaktionelle Entscheidung.
	adminProtectedMux.Handle("GET /admin/ai", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeys))))
	adminProtectedMux.Handle("POST /admin/ai/keys", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeyCreate)))))
	adminProtectedMux.Handle("POST /admin/ai/keys/{id}/revoke", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleAIKeyRevoke))))
	adminProtectedMux.Handle("GET /admin/plugins", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginList))))
	adminProtectedMux.Handle("POST /admin/plugins/upload", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginUpload))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/enable", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginEnable))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/websites", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginWebsites))))
	adminProtectedMux.Handle("POST /admin/plugins/{id}/remove", requireAdmin(requireFresh(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandlePluginRemove)))))
	// Der eigene Bildschirm eines Plugins steht auch der Redaktion offen, wenn
	// das Manifest nichts anderes sagt: dort wird Inhalt gepflegt.
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

	// Die Sprachen der Verwaltung. Nur für Administratoren: eine Sprachdatei
	// wirkt auf alle Bildschirme aller Benutzer.
	// Die Marke der Anlage: Name, Zeichen, Logo.
	adminProtectedMux.Handle("GET /admin/protokoll", requireAdmin(http.HandlerFunc(adminHandler.ErrHandler(adminHandler.HandleActivityList))))
	// Das Aufräumen entfernt die Spur der übrigen Handlungen. Es ist deshalb
	// die eine Stelle im Protokoll, die das Passwort noch einmal verlangt.
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
	// Die Sprache liegt ganz aussen, auch vor der Anmeldung: der Anmeldebildschirm
	// ist genau der Ort, an dem eine unlesbare Sprache am schlimmsten ist — von
	// dort führt kein Weg zu einer Einstellung. Hier kennt niemand einen
	// Benutzer, also entscheidet Accept-Language.
	adminPublic := i18n.Middleware(nil)(web.AdminHeaders(csrfMiddleware(setupGuard(adminPublicMux))))
	mux.Handle("/admin/login", adminPublic)
	mux.Handle("/admin/login/", adminPublic)
	// Exact path only — /admin/2fa/einrichten stays behind the auth chain.
	mux.Handle("/admin/2fa", adminPublic)
	mux.Handle("/admin/activate/", adminPublic)
	mux.Handle("/admin/reset/", adminPublic)
	mux.Handle("/admin/setup", adminPublic)
	mux.Handle("/admin/setup/", adminPublic)
	// Die Navigation braucht auf jedem Bildschirm dieselben zwei Dinge: welche
	// Websites es gibt und an welcher gerade gearbeitet wird. Einmal hier
	// geholt statt in dreissig Handlern — sonst zeigt die Seitenleiste die
	// Abschnitte einer Website nur dort, wo der Handler zufällig eine kennt.
	// Innen, hinter requireAuth: wer nicht angemeldet ist, braucht keine Liste.
	var listWebsites func(context.Context) ([]domain.Website, error)
	if d.domainStore != nil {
		// Nicht die blosse Liste: der Umschalter zeigt nur, was diese Person
		// auch betreten darf — sonst führt jeder zweite Eintrag auf ein 403.
		listWebsites = admin.NewNavWebsiteList(sm, d.domainStore, d.userStore)
	}
	withNav := web.WithNav(sm, listWebsites, pluginNavLinks(d.plugins))

	// Die eigene Sprache des angemeldeten Menschen. Innerhalb von requireAuth,
	// weil vorher niemand weiss, wer da liest; wer nichts gewählt hat, bekommt
	// wieder das, was der Browser mitbringt.
	//
	// Die Sprache gehört zum Menschen, nicht zur Website: auf derselben Website
	// arbeiten eine deutsche Redaktorin und ein englischsprachiger Entwickler,
	// und beide sollen ihre eigene Verwaltung sehen.
	withLang := i18n.Middleware(admin.NewLanguageLookup(sm, database))

	// Protected admin: headers -> CSRF -> setupGuard -> requireAuth -> language -> website access -> nav -> handler
	mux.Handle("/admin/", web.AdminHeaders(csrfMiddleware(setupGuard(requireAuth(requireSecondFactor(withLang(requireWebsite(withNav(adminProtectedMux)))))))))

	// Das Logo der Anlage. Öffentlich wie die Beigaben: es steht auch auf dem
	// Anmeldebildschirm, und wer den sieht, darf auch das Bild darauf sehen.
	mux.HandleFunc("GET /admin/marke/logo", adminHandler.ErrHandler(adminHandler.HandleBrandingLogo))

	// Media serve (public, no auth)
	mux.HandleFunc("GET /media/{websiteID}/{filename}", adminHandler.ErrHandler(adminHandler.HandleMediaServe))
	// Beigaben eines Plugins. Eigener Pfad statt /assets, damit ein Plugin die
	// Dateien des Kerns nicht überschatten kann.
	mux.HandleFunc("GET /plugin-assets/{id}/{path...}", adminHandler.ErrHandler(adminHandler.HandlePluginAsset))

	// Public site handler and routes
	publicHandler := public.NewHandler(pageStore, menuStore, mediaStore, snippetStore, templateLoader, domainResolver, cfg.DataDir, publicDefaultFS, cfg.Secure)
	// Der Manager entsteht weiter oben, weil die Verwaltung ihn schon braucht;
	// hier bekommt ihn die öffentliche Seite. Ist er nil, verhält sich alles
	// wie vor den Plugins.
	publicHandler.SetPlugins(d.plugins)
	// Zwei Host-Funktionen brauchen den öffentlichen Handler: Seiten lesen und
	// eine Seite im Theme der Website ausgeben. Erst hier gibt es ihn, und ein
	// Plugin ruft sie ohnehin frühestens in einem Haken auf.
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
	// Die Plugin-Schicht liegt innerhalb des Auflösers (die Website ist also
	// bekannt) und ausserhalb des Verteilers (ein Plugin kann also eine Adresse
	// beanspruchen, die der Kern gar nicht kennt).
	// Die Sprachschicht liegt zwischen Auflöser und Plugins: sie braucht die
	// Website (welche Präfixe Sprachen sind, hängt an ihr), und ein Plugin soll
	// unter /fr/… dieselbe Adresse sehen wie unter /… — sonst müsste jedes
	// Plugin die Mehrsprachigkeit selbst kennen.
	//
	// Die Shop-Schicht sitzt zuinnerst, direkt vor dem Verteiler: die Adresse
	// des Katalogs ist eine Einstellung der Website und steht deshalb nicht in
	// der Routentabelle — als Muster wäre sie "/{base}/{slug}" und würde mit
	// "/t/{path...}" kollidieren, was Gos mux beim Start mit einem panic
	// quittiert. Das erste Segment gegen die Einstellung der Website zu
	// prüfen ist, was der Aufruf wirklich braucht.
	mux.Handle("/", domainResolver.Middleware(public.LocaleMiddleware(publicHandler.PluginMiddleware(publicHandler.ShopRoutes(publicMux)))))

	// Outermost first: an id for every request, then the access log (so even a
	// panicking request produces a line), then recovery, then the security
	// headers and the session middleware.
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
