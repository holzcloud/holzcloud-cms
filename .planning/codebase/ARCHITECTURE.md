<!-- refreshed: 2026-08-22 -->
# Architecture

**Analysis Date:** 2026-08-22

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│         Single Go binary — `cmd/holzcloud/main.go`           │
│  embed.FS: templates/ assets/ + internal/db/migrations/      │
├──────────────────┬──────────────────┬───────────────────────┤
│   CLI subcmds    │  Admin UI (htmx) │   Public site (SSR)   │
│ `cmd/holzcloud/  │  `internal/      │  `internal/public/`   │
│  cli.go`         │   admin/`        │                       │
├──────────────────┴──────────────────┴───────────────────────┤
│  Global middleware chain — `cmd/holzcloud/main.go:newRouter` │
│  RequestID → AccessLog → Recoverer → SecureHeaders → session │
└────────┬─────────────────────┬──────────────────┬───────────┘
         │                     │                  │
         ▼                     ▼                  ▼
┌──────────────────┐  ┌──────────────────┐ ┌─────────────────┐
│ Admin branch     │  │ Public branch    │ │ MCP / probes    │
│ AdminHeaders →   │  │ domain.Resolver →│ │ `/ai` `/healthz`│
│ CSRF → setupGuard│  │ LocaleMiddleware │ │ `/readyz`       │
│ → RequireAuth →  │  │ → PluginMiddle-  │ │ `internal/ai/`  │
│ 2FA → i18n →     │  │ ware → publicMux │ │ `internal/web/  │
│ website access → │  │                  │ │  health.go`     │
│ nav → adminMux   │  │                  │ │                 │
└────────┬─────────┘  └────────┬─────────┘ └────────┬────────┘
         │                     │                    │
         ▼                     ▼                    ▼
┌─────────────────────────────────────────────────────────────┐
│  Domain packages (store + model + logic, one per concept)    │
│  page menu media term snippet field kind block domain user   │
│  tmplmgr plugin mail sharelink i18n branding locale bundle   │
├─────────────────────────────────────────────────────────────┤
│  Rendering: `internal/template/loader.go` (disk→embed),      │
│  `internal/web/` (admin templates, layout data, headers)     │
├─────────────────────────────────────────────────────────────┤
│  Plugins: wazero WASM runtime — `internal/plugin/runtime.go` │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  SQLite dual pool (write max 1 conn, read max 5), WAL        │
│  `internal/db/db.go` → `data/holzcloud.sqlite`               │
│  Media + user templates + i18n on disk under `data/`         │
└─────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Entry point | Config load, migrations, DI wiring, jobs, graceful shutdown | `cmd/holzcloud/main.go` |
| Router builder | Whole route table + middleware composition (testable apart from `main`) | `cmd/holzcloud/main.go` (`newRouter`, line ~572) |
| CLI | Recovery subcommands: `serve user backup migrate compact rerender thumbnails check` | `cmd/holzcloud/cli.go` |
| Config | Env-based config + validation + logger | `internal/config/config.go` |
| Database | Dual-pool open, goose migrations, backup/snapshot, integrity, maintenance | `internal/db/db.go`, `internal/db/maintain.go` |
| Auth | Argon2id, SCS sessions in SQLite, CSRF key, role/website/2FA middleware, login throttle | `internal/auth/` |
| Domain resolver | Host → Website lookup, context injection, canonical base, offline page | `internal/domain/resolver.go` |
| Admin handlers | All `/admin/*` CRUD screens (47 files) | `internal/admin/` |
| Public handlers | Page, home, archive, tag, feed, sitemap, robots, share links, gates | `internal/public/handler.go` |
| Template loader | Per-website public template resolution, parse + `sync.Map` cache, asset serving | `internal/template/loader.go` |
| Admin templates | Parsed once at startup from `embed.FS` | `internal/web/` (`ParseAdminTemplates`), `cmd/holzcloud/templates/admin/` |
| Plugin runtime | wazero WASM host, per-plugin instance, host functions, timeouts | `internal/plugin/runtime.go`, `internal/plugin/abi.go` |
| Plugin manager | Install/enable, manifest, admin links, hook dispatch | `internal/plugin/manager.go` |
| AI/MCP server | Token-authenticated MCP endpoint exposing domain/page/media/field tools | `internal/ai/mcp.go`, `internal/ai/tools.go` |
| Mail | SMTP sender + persistent queue drained by a job | `internal/mail/queue.go` |
| Jobs | Single ticker runner for all periodic work | `internal/jobs/runner.go` |
| Media | Upload, MIME allowlist, metadata strip, crop, responsive variants | `internal/media/` |
| Bundle / WXR | Site export/import, WordPress WXR ingest | `internal/bundle/`, `internal/wxr/` |

## Pattern Overview

**Overall:** Modular monolith — layered request pipeline with per-concept packages, constructor injection wired entirely in `main()`, no framework and no ORM.

**Key Characteristics:**
- Stdlib `net/http` + Go 1.22 `ServeMux` method/pattern routing; all routes declared in one place (`newRouter`)
- Each concept package owns its model and its `Store` (`NewStore(*db.DB)`); handlers hold pointers to stores
- Multi-tenant by host: `domain.Resolver` puts the `*domain.Website` into the request context, everything downstream scopes by `website_id`
- Optional subsystems (plugins, mail, AI, terms, fields, kinds) are injected via `SetX` setters and may be `nil` — nil means the feature quietly does not exist
- Everything shippable is embedded: templates, assets, migrations, locale catalogs

## Layers

**Entry / wiring:**
- Purpose: build every dependency, register every route, run jobs, shut down cleanly
- Location: `cmd/holzcloud/main.go`
- Depends on: every `internal/` package
- Used by: nothing

**HTTP middleware:**
- Purpose: cross-cutting request concerns
- Location: `internal/web/` (`headers.go`, `logging.go`, `clientip.go`), `internal/auth/middleware.go`, `internal/domain/resolver.go`, `internal/i18n/context.go`, `internal/public/locale.go`
- Depends on: session manager, stores
- Used by: `newRouter`

**Handlers:**
- Purpose: parse request, call stores, render template or partial
- Location: `internal/admin/`, `internal/public/`, `internal/ai/`
- Depends on: stores, template loader, `internal/web` helpers

**Stores / domain logic:**
- Purpose: SQL access and per-concept rules; no HTTP types
- Location: `internal/page/store.go`, `internal/menu/store.go`, `internal/media/store.go`, `internal/domain/store.go`, `internal/user/store.go`, `internal/term/store.go`, `internal/field/store.go`, `internal/kind/store.go`, `internal/block/store.go`, `internal/snippet/store.go`, `internal/tmplmgr/store.go`, `internal/plugin/store.go`, `internal/ai/token.go`
- Depends on: `internal/db`

**Persistence:**
- Purpose: SQLite dual pool, migrations, backup, integrity
- Location: `internal/db/`, `internal/db/migrations/` (38 goose `.sql` files)

## Data Flow

### Public page request

1. `RequestID` → `AccessLog` → `Recoverer` → `SecureHeaders` → `sm.LoadAndSave` (`cmd/holzcloud/main.go:941-945`)
2. `domainResolver.Middleware` resolves host → `*domain.Website` into context, or serves the maintenance page (`internal/domain/resolver.go:48`)
3. `public.LocaleMiddleware` strips a `/fr/`-style prefix and records the locale (`internal/public/locale.go`)
4. `publicHandler.PluginMiddleware` gives plugins first refusal on the path (`internal/public/plugins.go`)
5. `publicMux` matches `GET /{slug}` → `HandlePage` (`internal/public/handler.go:193`)
6. Archive / custom-kind archive checked before pages; then `pageStore.GetPublishedPageIn` (drafts and other locales cannot be reached)
7. Password-protected pages divert to `serveGate` before content is loaded
8. Site data, snippets, menus, translations and meta assembled into `tmpl.PageData`
9. `loader.RenderPage(ctx, websiteID, "page.html", data)` — disk template if the website has one, else embedded default (`internal/template/loader.go:483`)
10. `serveCached` writes with ETag/Last-Modified (`internal/public/handler.go:351`)

### Admin request

1. Same outer chain, then `web.AdminHeaders` → `csrfMiddleware` → `setupGuard` → `RequireAuth` → `RequireSecondFactor` → `i18n.Middleware` → `RequireWebsiteAccess` → nav middleware → `adminProtectedMux` (`cmd/holzcloud/main.go:882`)
2. Handler is wrapped by `adminHandler.ErrHandler`, which logs and returns 500 on a returned error (`internal/admin/handler.go:187`)
3. Handler branches on `HX-Request` to render a partial or a full page from the startup-parsed admin template set

### Plugin hook

1. Manager dispatches a hook to the wasm instance (`internal/plugin/manager.go`)
2. `Runtime` serialises calls per plugin, 2 s `CallTimeout`, 16 MB memory cap, 8 MB payload cap (`internal/plugin/runtime.go`)
3. Host functions (`WithSettings`, `WithPages`, `WithRender`, `WithNotify`) are injected as closures from `main` so `internal/plugin` depends on no domain package

**State Management:**
- Sessions in SQLite via `alexedwards/scs` with a custom store (`internal/auth/sessionstore.go`)
- Request-scoped values in `context.Context`: website (`internal/domain/context.go`), language (`internal/i18n/context.go`), locale (`internal/public/locale.go`)

## Key Abstractions

**Store:**
- Purpose: SQL gateway for one concept
- Examples: `internal/page/store.go`, `internal/domain/store.go`
- Pattern: `NewStore(*db.DB)`, methods take `ctx` first, direct SQL, no ORM

**Handler + ErrHandler:**
- Purpose: handlers return `error`; a wrapper converts it to a response
- Examples: `internal/admin/handler.go:187`, `internal/public/handler.go:102`

**Loader:**
- Purpose: resolve a website's public template set, disk-first with embed fallback, cached by `{websiteID, view, locale, timezone}`
- Examples: `internal/template/loader.go:394` (`cacheKey`), `:543` (`InvalidateTemplateCache`)

**Signer:**
- Purpose: HMAC tokens for preview links and unlock cookies, with distinct key labels
- Examples: `internal/sharelink/sharelink.go`, wired at `cmd/holzcloud/main.go:202`

**Job:**
- Purpose: named periodic task honouring the shutdown context
- Examples: `internal/jobs/runner.go`; nine jobs registered in `cmd/holzcloud/main.go`

## Entry Points

**`main()` — `cmd/holzcloud/main.go:59`:**
- Triggers: process start
- Responsibilities: CLI dispatch, config, data dir, DB open, pre-upgrade snapshot, migrations, integrity check, template parse, DI, router, jobs, HTTP server, graceful shutdown (WAL fold on exit)

**`runCLI()` — `cmd/holzcloud/cli.go:56`:**
- Triggers: any argv other than `serve`
- Responsibilities: recovery paths that need no HTTP server (user create/passwd/2fa, backup, migrate, compact, rerender, thumbnails, check)

**`newRouter()` — `cmd/holzcloud/main.go:572`:**
- Triggers: called by `main` and by the route-table test
- Responsibilities: the complete route table and middleware composition

**Build tools:** `tools/i18n/main.go` (catalog extraction), `tools/mkbundle/main.go` (site bundle build)

## Architectural Constraints

- **No runtime third-party fetch:** every subresource must come from this origin. Enforced by CSP in `internal/web/headers.go` and by upload-time rejection in `internal/tmplmgr/external.go`. Both must keep working.
- **No CGO, no JS beyond self-hosted htmx 2.x, no CSS build step.** SQLite is `modernc.org/sqlite`.
- **Write serialisation:** the write pool is capped at one connection with `_txlock=immediate`; the read pool at five (`internal/db/db.go`). Long write transactions block all writers.
- **Threading:** one goroutine per request plus the `jobs` runner. Media resizing is throttled to two concurrent operations (`internal/media/variants.go:91` `resizeSlots`). Plugin calls are serialised per plugin instance.
- **Package-level mutable state:** `internal/branding` (name, mark, dir — loaded once at startup) and `internal/i18n` (`SetDir`, catalog cache). Both are set before the server starts.
- **Optional dependencies are nil-able:** plugins, mail, AI tokens, term/field/kind stores. Every consumer must nil-check.
- **Route registration order matters:** `/admin/*` is registered before the catch-all `/` public branch.
- **Everything ships embedded:** adding a template, asset, locale or migration means adding it under an `embed.FS` root, not to a runtime path.

## Anti-Patterns

### Casting unsanitized Markdown output to `template.HTML`

**What happens:** goldmark output is cast directly instead of passing through bluemonday.
**Why it's wrong:** user-generated content becomes stored XSS on the public site.
**Do this instead:** goldmark → bluemonday → `template.HTML`, as in `internal/page/markdown.go:23-47`.

### Querying pages without the published/locale filter

**What happens:** a public code path calls a generic page getter and filters status in the template or not at all.
**Why it's wrong:** drafts and trashed content leak to visitors.
**Do this instead:** use `pageStore.GetPublishedPageIn` and the other published-scoped queries (`internal/public/handler.go:220`, `internal/page/access.go`).

### Doing per-request COUNT(*) or config lookups

**What happens:** a middleware re-queries state that can only change once.
**Why it's wrong:** every request pays for it on a Raspberry Pi.
**Do this instead:** latch it — see the `usersExist atomic.Bool` setup guard (`cmd/holzcloud/main.go:297`) and `branding.Load` at startup (`internal/branding/branding.go:83`).

### Registering a route inside a feature package

**What happens:** a package attaches handlers to a mux of its own.
**Why it's wrong:** the route table stops being reviewable, and auth middleware can be forgotten — a missing `requireAdmin` on website deletion shipped exactly this way.
**Do this instead:** add the route in `newRouter` (`cmd/holzcloud/main.go`) and cover it in `cmd/holzcloud/main_test.go`.

### Letting `internal/plugin` import a domain package

**What happens:** the runtime reaches into `internal/domain` or `internal/public` for host data.
**Why it's wrong:** creates an import cycle and makes both sides untestable in isolation.
**Do this instead:** pass a closure from `main` — `WithSettings`, `WithPages`, `WithRender`, `WithNotify` (`cmd/holzcloud/main.go:266`, `:904`).

### Branching on `HX-Request` without `Vary`

**What happens:** a handler returns a partial for htmx and a full page otherwise but omits `Vary: HX-Request`.
**Why it's wrong:** caches and the browser serve a fragment as a whole document.
**Do this instead:** set `Vary: HX-Request` on every branching handler.

## Error Handling

**Strategy:** handlers return `error`; `ErrHandler` logs with `slog.Error` (path, method) and writes a 500. Startup errors are fatal with an explanatory `slog.Error` and `os.Exit(1)`.

**Patterns:**
- Guard clauses and early returns; `fmt.Errorf("...: %w", err)` wrapping at every boundary
- User-facing errors go through flash messages (`internal/web/flash.go`), never a 500 page
- Degrade rather than die: plugin runtime, mail and AI failures are logged and the feature disappears
- Panics are caught by `web.Recoverer`; a pre-upgrade DB snapshot plus a printed restore command guards failed migrations

## Cross-Cutting Concerns

**Logging:** `log/slog` structured, level from config (`internal/config/config.go`); request id and access log middleware in `internal/web/logging.go`.
**Validation:** form parsing/validation helpers in `internal/web/forms.go`; slug and host normalisation in `internal/page/slug.go` and `internal/domain/normalize.go`; MIME allowlist in `internal/media/upload.go`.
**Authentication:** Argon2id + SCS sessions with rotation, optional TOTP (`internal/totp/`), CSRF on `/admin`, throttling, role/website/fresh-password guards in `internal/auth/middleware.go` and `elevate.go`.
**Security headers:** `web.SecureHeaders` / `web.AdminHeaders` with a `default-src 'self'` CSP (`internal/web/headers.go`).
**i18n:** embedded catalogs in `internal/i18n/locales/` plus disk overrides under `data/`; language chosen per request by middleware.
**Health:** `/healthz` constant 200; `/readyz` reports DB ping, integrity verdict, free disk and write probe (`internal/web/health.go`).

---

*Architecture analysis: 2026-08-22*
