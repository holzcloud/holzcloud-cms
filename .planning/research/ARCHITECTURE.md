# Architecture Patterns

**Domain:** Multi-site self-hosted CMS (Go + htmx + SQLite)
**Researched:** 2026-04-13
**Confidence:** HIGH (Go stdlib patterns well-established; htmx patterns verified against official docs + community)

---

## Recommended Architecture

Single-process, server-rendered Go application. Front-controller pattern via `net/http`. One binary, one port, one SQLite file, one `data/` directory for uploads. All admin and public sites served from the same binary with separate middleware stacks applied in a shared mux.

---

## Package Layout

```
cmd/
  holzcloud/
    main.go              # wire dependencies, start server, signal handling

internal/
  config/
    config.go            # load from env vars; typed Config struct
  db/
    db.go                # open SQLite, run migrations, query helpers (one/all/exec)
    migrations/
      001_initial.sql
      002_add_media.sql
      ...                # versioned SQL files, embedded via embed.FS
  auth/
    session.go           # session cookie read/write
    password.go          # bcrypt via golang.org/x/crypto
    middleware.go        # RequireAdmin, RequireRole — wraps http.Handler
    csrf.go              # token generation, validation, form helper
  domain/
    resolver.go          # Host → websites.id lookup (DB query + in-memory cache)
    model.go             # Website, Domain value types
  web/
    router.go            # thin wrapper or direct net/http ServeMux usage
    middleware.go        # logging, recovery, site-resolver middleware
    response.go          # helpers: renderTemplate, renderPartial, sendError
    static.go            # serve embedded CSS/htmx.js with cache headers
  admin/
    handler.go           # AdminHandler struct, route registration method
    websites.go          # CRUD for websites + domains
    pages.go             # CRUD for pages (list, new, edit, delete)
    menus.go             # CRUD for menus + menu items
    templates.go         # manage per-site template directories
    media.go             # upload, list, delete media files
    users.go             # user management (admin only)
  public/
    handler.go           # PublicHandler struct, route registration method
    renderer.go          # resolve template dir, render page
  template/
    loader.go            # load Go templates from disk OR embedded FS with fallback
    engine.go            # parse + execute html/template; cache parsed templates
  media/
    store.go             # save/delete files under data/media/{website_id}/
    serve.go             # stream files with correct Content-Type + cache headers

templates/               # OUTSIDE internal — user-editable at runtime
  admin/                 # admin UI templates (embedded into binary)
    layout.html
    dashboard.html
    pages/
      list.html
      edit.html
    ...
  public/
    default/             # fallback public template (embedded into binary)
      layout.html
      page.html
      menu.html

assets/                  # OUTSIDE internal — embedded into binary at build time
  admin.css
  htmx.min.js            # vendored htmx (pinned version, no CDN)
  public.css             # base public styles (override per-site via template dir)

data/                    # runtime directory, NOT in repo, NOT embedded
  holzcloud.sqlite
  media/
    {website_id}/
      ...
  templates/             # user-installed per-site template overrides
    {template_slug}/
      layout.html
      page.html
      ...
```

---

## Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `cmd/holzcloud/main.go` | Wire deps, start HTTP server, catch OS signals, graceful shutdown | `internal/config`, `internal/db`, `internal/web`, `internal/admin`, `internal/public` |
| `internal/config` | Parse env vars into a typed struct; validate required fields at startup | Called once at boot by main |
| `internal/db` | Open SQLite, run versioned migrations on startup, query helpers (`One`, `All`, `Exec`) | Used by all handler packages; never imported by `internal/web` middleware |
| `internal/auth` | Session read/write, CSRF token lifecycle, password hash/verify, middleware factories | `internal/db` (session store can be DB or cookie-signed); `internal/web` middleware chain |
| `internal/domain` | Map request `Host` to `websites` row; cache with short TTL or invalidation on write | `internal/db`; called from site-resolver middleware |
| `internal/web` | mux construction, shared middleware (logging, panic recovery, site resolution), response helpers, static assets | Composes `internal/admin` and `internal/public` handlers |
| `internal/admin` | All `/admin/...` routes — CRUD handlers, htmx partials, form validation | `internal/db`, `internal/auth`, `internal/template`, `internal/media` |
| `internal/public` | Slug-based page rendering for resolved website | `internal/db`, `internal/template`, `internal/domain` |
| `internal/template` | Load + parse Go `html/template` sets; disk-first with embedded fallback | `internal/db` (reads `templates.storage_path`); used by admin + public |
| `internal/media` | File upload validation, storage under `data/media/`, streaming response | `internal/db`; used by `internal/admin` |

---

## Request Flow

```
HTTP request
  │
  ▼
net/http mux (internal/web)
  │
  ├── Shared middleware stack (applied to all routes):
  │     1. Panic recovery (log + 500)
  │     2. Request logger
  │     3. Site resolver: Host → websites.id → stored in request context
  │
  ├─ /assets/* ──────────────────────────────────────────────────────────────▶ Static file handler
  │                                                                             (embed.FS, long cache)
  │
  ├─ /admin/* ────────────────────────────────────────────────────────────────▶ Admin middleware stack:
  │                                                                               4. Session read
  │                                                                               5. RequireAdmin gate
  │                                                                               6. CSRF validation (POST)
  │                                                                             Admin handler
  │                                                                               → DB query
  │                                                                               → renderTemplate or renderPartial
  │                                                                               → http.ResponseWriter
  │
  └─ /* (everything else) ──────────────────────────────────────────────────▶ Public handler:
                                                                                Read website from context
                                                                                Slug match → pages row
                                                                                Load per-site template
                                                                                Render full page
                                                                                → http.ResponseWriter
```

---

## Multi-Domain Resolution

**Strategy:** middleware, not per-handler logic.

A `SiteResolver` middleware runs before routing decisions. It reads `r.Host` (strip port), looks up `website_domains` JOIN `websites`, and stores the resolved `Website` in the request context via a typed key.

```go
// internal/domain/resolver.go

type contextKey struct{}

func Middleware(db *db.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            host := stripPort(r.Host)
            site, err := lookupByHost(db, host)
            if err != nil || site == nil {
                // admin subdomain or unknown host: allow through without site
                next.ServeHTTP(w, r)
                return
            }
            ctx := context.WithValue(r.Context(), contextKey{}, site)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func FromContext(ctx context.Context) *Website {
    v, _ := ctx.Value(contextKey{}).(*Website)
    return v
}
```

Cache the host→website mapping in a `sync.Map` with write-invalidation on domain changes. Short TTL (60s) is sufficient; Pi RAM is not the bottleneck.

Admin routes do not require a resolved site — the admin operates across all sites. Public routes 404 if no site resolves.

---

## Admin vs Public Routing — Same Binary, Same Port

Use `net/http`'s `ServeMux` (Go 1.22+ method+path patterns are sufficient; no third-party router needed).

```go
mux := http.NewServeMux()

// Static assets — no auth needed
mux.Handle("GET /assets/", web.StaticHandler(assets))

// Admin routes — wrapped with admin middleware stack
adminMw := chain(auth.SessionMiddleware(db), auth.RequireAdmin, auth.CSRFMiddleware)
admin.RegisterRoutes(mux, adminMw, adminHandler)
// Registers: GET /admin/, GET /admin/pages, POST /admin/pages, etc.

// Public catch-all — runs last
mux.Handle("/", chain(domain.Middleware(db), public.Handler(publicHandler)))
```

`chain()` is a simple variadic middleware composer (no framework needed, 10 lines of stdlib code).

Admin routes are prefixed `/admin/` — never overlap with public slug space.

---

## Session and CSRF Middleware Stack Order

Order matters. Apply in this sequence for admin routes:

1. **Panic recovery** — outermost, catches everything
2. **Logger** — after recovery so it logs recovered panics too
3. **Site resolver** — sets website in context (shared)
4. **Session read** — populates `auth.User` in context from cookie
5. **RequireAdmin gate** — redirects to `/admin/login` if no valid session
6. **CSRF validation** — on POST/PUT/DELETE only; compares token from form field vs session-stored token

CSRF token is a cryptographically random value stored in the session and rendered as a hidden field in every form via a template helper function. Double-submit cookie pattern is NOT used — session-bound token is simpler and more robust with httpOnly cookies.

---

## Template System

**Recommended: dual-source loader — embedded defaults + disk overrides.**

Admin templates are embedded into the binary (`//go:embed templates/admin`). They are not user-editable by design; admin UI upgrades with the binary.

Public templates follow a disk-first, embedded-fallback strategy:

1. Check `data/templates/{template_slug}/` on disk (user-installed, editable without recompile)
2. Fall back to embedded `templates/public/default/` for any file not found on disk

This means the default site works out of the box on first run (single binary, zero config), and operators can install custom templates by dropping files in `data/templates/`.

```go
// internal/template/loader.go

func Load(slug string, diskRoot string, embedded embed.FS) (*template.Template, error) {
    // Try disk first
    path := filepath.Join(diskRoot, slug, "layout.html")
    if _, err := os.Stat(path); err == nil {
        return template.ParseGlob(filepath.Join(diskRoot, slug, "*.html"))
    }
    // Fall back to embedded default
    return template.ParseFS(embedded, "templates/public/default/*.html")
}
```

Parse templates once per request or cache parsed `*template.Template` by slug with invalidation on template-change admin action. For Pi: cache aggressively; re-parse only when the admin changes a template.

---

## Static Assets (CSS, htmx.js, User Uploads)

| Asset Type | Where Served | Cache Headers |
|------------|-------------|---------------|
| `admin.css`, `public.css` | Embedded in binary, `/assets/` route | `Cache-Control: public, max-age=31536000, immutable` (use content hash in filename for versioning) |
| `htmx.min.js` | Vendored in `assets/`, embedded | Same as above |
| User media uploads | `data/media/{website_id}/` on disk, `/media/{website_id}/{file}` route | `Cache-Control: public, max-age=86400` |

Vendor `htmx.min.js` into the repo — no CDN dependency, works air-gapped on Pi. Pin the version in a comment at the top of the file. Update manually when upgrading htmx.

For CSS versioning without a build tool: append a build-time constant (e.g., the binary's `version` string) as a query param: `/assets/admin.css?v=1.2.0`. The `immutable` flag + content-addressed filenames is ideal but requires a rename on change; query param is acceptable for a CMS of this size.

---

## Database Layer

**Direct `database/sql` with a thin helper wrapper. No ORM.**

```go
// internal/db/db.go

type DB struct { conn *sql.DB }

func (d *DB) One(dest any, query string, args ...any) error      // scan single row
func (d *DB) All(query string, args ...any) ([]map[string]any, error) // all rows as maps
func (d *DB) Exec(query string, args ...any) (sql.Result, error)
func (d *DB) LastInsertID(query string, args ...any) (int64, error)
```

Handlers use direct SQL with named params. No repository layer — controllers hold SQL inline (this is the explicit preference from PROJECT.md and matches the PHP legacy pattern).

### Migration Strategy

Use versioned SQL files embedded in the binary (`//go:embed migrations/*.sql`). Run automatically at startup. Track applied migrations in a `schema_migrations` table (one row per filename). This is the same pattern as `golang-migrate` but implemented in ~60 lines without the dependency.

```sql
-- internal/db/migrations/001_initial.sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS websites ( ... );
CREATE TABLE IF NOT EXISTS website_domains ( ... );
-- etc.
```

On startup: read embedded migration files sorted by name, check `schema_migrations`, run any not yet applied in order, record each. Wrap in a transaction per migration. If a migration fails, halt startup with a clear error message.

Do not use `goose` or `golang-migrate` — they are justified for larger teams but add dependency weight for a Pi-targeted single-binary CMS where the migration set is small and owned entirely by the project.

---

## Error Handling Pattern

Handlers return `error`; a central adapter translates errors to HTTP responses.

```go
// internal/web/response.go

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func Adapt(h HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            var he *HTTPError
            if errors.As(err, &he) {
                renderError(w, r, he.Code, he.Message)
                return
            }
            log.Printf("internal error: %v", err)
            renderError(w, r, 500, "Something went wrong")
        }
    }
}

type HTTPError struct { Code int; Message string }
func NotFound(msg string) *HTTPError { return &HTTPError{404, msg} }
func Forbidden() *HTTPError          { return &HTTPError{403, "Forbidden"} }
```

Handlers signal validation failures via flash messages + redirect (same as PHP legacy). Handlers signal hard errors by returning an `*HTTPError`. Unknown errors (DB failures, file I/O) return `error` and log at ERROR level; user sees a generic 500 page.

For admin htmx requests, check `HX-Request: true` header in the error handler — return an htmx-compatible error partial rather than a full page redirect when the request was a partial.

---

## htmx Integration Pattern

**Check `HX-Request` header to decide full page vs partial.**

```go
func isHXRequest(r *http.Request) bool {
    return r.Header.Get("HX-Request") == "true"
}
```

Handlers that support both full-page and partial rendering follow this pattern:

```go
func (h *AdminHandler) PagesList(w http.ResponseWriter, r *http.Request) error {
    pages, err := h.db.AllPages(siteID)
    if err != nil { return err }

    if isHXRequest(r) {
        return renderPartial(w, r, "admin/pages/list_rows.html", data)
    }
    return renderFull(w, r, "admin/pages/list.html", data)
}
```

**Which endpoints return partials vs full pages:**

| Endpoint | Full page | Partial (HX-Request) |
|----------|-----------|----------------------|
| `GET /admin/pages` | yes (initial load) | yes (pagination, filter) |
| `POST /admin/pages` (create) | redirect | `HX-Redirect` header → full page reload |
| `GET /admin/pages/{id}/edit` | yes | yes (inline edit form swap) |
| `PUT /admin/pages/{id}` | redirect | `HX-Redirect` or updated row partial |
| `DELETE /admin/pages/{id}` | redirect | `200 OK` with empty body (htmx removes element) |
| `GET /admin/pages/{id}/preview` | yes | yes (preview pane swap) |
| Public `GET /{slug}` | always full page | N/A (public site never uses htmx for page nav) |

Avoid `HX-Boost` on public pages — server-rendered public pages are already fast; boost adds complexity without meaningful benefit on a Pi.

Use `HX-Redirect` response header (not 302) when a POST needs to navigate to a different URL after htmx submission — keeps htmx in control of the history stack.

Do NOT use a third-party go-htmx wrapper library. The `HX-Request` header check and `HX-Redirect` header write are two lines each. The dependency is not worth it.

---

## Graceful Shutdown and Signal Handling

```go
// cmd/holzcloud/main.go

srv := &http.Server{Addr: cfg.Addr, Handler: mux}

go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatalf("server error: %v", err)
    }
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Printf("shutdown error: %v", err)
}
db.Close()
```

15-second shutdown window is sufficient for a CMS with no long-running requests. No systemd notify needed for initial versions; add `sd_notify` later if desired.

---

## Build Orientation: embed.FS vs External Templates

**Recommended split:**

| Asset | Embedded? | Rationale |
|-------|-----------|-----------|
| Admin templates (`templates/admin/`) | YES | Admin UI ships with binary; no user editing |
| Admin CSS, htmx.js (`assets/`) | YES | No build step; deterministic binary |
| Public default template (`templates/public/default/`) | YES | First-run works with zero config |
| User-installed public templates (`data/templates/`) | NO (disk) | Authors customize without recompile |
| User media uploads (`data/media/`) | NO (disk) | Runtime data, not source code |
| DB migrations (`internal/db/migrations/`) | YES | Schema must match binary version |

The binary is self-sufficient for a fresh install. Operators gain customization by placing files in `data/`. This is the right tradeoff for a Pi-deployed single-binary CMS.

---

## Testing Approach

Handler-level tests using `net/http/httptest` + in-memory SQLite.

```go
// internal/admin/pages_test.go

func TestPagesCreate(t *testing.T) {
    db := testDB(t)             // open :memory: SQLite, run migrations
    h := NewAdminHandler(db, testConfig())
    
    req := httptest.NewRequest("POST", "/admin/pages", formBody(...))
    req.Header.Set("X-CSRF-Token", seedCSRF(req))
    w := httptest.NewRecorder()
    
    h.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusSeeOther, w.Code)
    // verify DB row created
}
```

`testDB(t)` opens `file::memory:?cache=shared`, runs all migrations, returns `*db.DB`, registers `t.Cleanup(db.Close)`. Use `t.TempDir()` for tests that need file-based SQLite (media upload tests).

No mocking of the DB layer — SQLite in-memory is fast enough and tests real SQL. Mock only external I/O (file system for media tests using `t.TempDir()`).

---

## Scalability Considerations

| Concern | At 5 sites / Pi 5 | At 50 sites | At 500 sites |
|---------|-------------------|-------------|--------------|
| DB connections | 1 `*sql.DB` with WAL mode | Same | May need WAL + read connection pool |
| Template cache | Per-site `*template.Template` in `sync.Map` | Same, ~MB RAM | Fine |
| Media serving | Direct disk read | Same | Consider separate CDN |
| Multi-domain lookup | `sync.Map` cache | Same | Same |
| Binary size | ~15MB (embedded assets + binary) | Same | Same |

SQLite WAL mode is mandatory (`PRAGMA journal_mode=WAL`) — enables concurrent reads during writes. Set at DB open time.

---

## Build Order (Vertical Slices)

Implement in this order so each layer is testable before the next:

1. **`internal/config` + `internal/db`** — Config parsing + SQLite open + migrations + query helpers. Nothing works without these. Test: migration runs, schema exists.

2. **`cmd/holzcloud/main.go` skeleton** — Start HTTP server, serve a health check at `GET /healthz`. Verify binary builds and runs on ARM64 (cross-compile from dev machine). Proves the single-binary constraint.

3. **`internal/auth`** — Session cookies, password hash/verify, CSRF token. No UI yet; test via unit tests and `httptest`.

4. **`internal/web` middleware + admin login UI** — Mux construction, middleware chain, login form (full page, no htmx), session write on success, `RequireAdmin` gate. First working browser interaction.

5. **`internal/admin` — Websites + Domains CRUD** — Multi-site data model established. Everything downstream depends on `websites.id`.

6. **`internal/domain` resolver + `internal/public` basic renderer** — Resolve Host → website, render a placeholder page. Proves multi-domain routing works end-to-end.

7. **`internal/admin` — Pages CRUD** — Full create/edit/delete with htmx partials for list pagination. First htmx integration.

8. **`internal/template` + per-site templates** — Disk-first loader, embedded fallback, template cache. Public pages render with real templates.

9. **`internal/admin` — Menus CRUD** — Menu builder, menu rendering in public templates.

10. **`internal/media`** — File upload, serve, delete. Last because it requires everything above to be wired.

11. **`internal/admin` — Users + roles** — User management, role gates. Defer until after core content CRUD works to avoid blocking earlier phases on auth complexity.

---

## Sources

- [Organizing a Go module — golang.org](https://go.dev/doc/modules/layout)
- [Go Project Structure: Practices & Patterns — glukhov.org](https://www.glukhov.org/post/2025/12/go-project-structure/)
- [htmx docs — htmx.org](https://htmx.org/docs/)
- [go-htmx package — pkg.go.dev](https://pkg.go.dev/github.com/donseba/go-htmx) (reviewed but not recommended as dependency)
- [Using Go Embed Package for Template Rendering — andrew-mccall.com](https://andrew-mccall.com/blog/2025/01/using-go-embed-package-for-template-rendering/)
- [Embedded File Systems: Using embed.FS in Production — DEV Community](https://dev.to/rezmoss/embedded-file-systems-using-embedfs-in-production-89-2fpa)
- [Simple declarative schema migration for SQLite — david.rothlis.net](https://david.rothlis.net/declarative-schema-migration-for-sqlite/)
- [Mastering Database Migrations in Go with golang-migrate and SQLite — DEV Community](https://dev.to/ouma_ouma/mastering-database-migrations-in-go-with-golang-migrate-and-sqlite-3jhb)
- [How To Build a Web Application with HTMX and Go — DEV Community](https://dev.to/calvinmclean/how-to-build-a-web-application-with-htmx-and-go-3183)
