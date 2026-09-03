# Coding Conventions

**Analysis Date:** 2026-08-22

## Naming Patterns

**Files:**
- Go source: `snake_case.go` — `internal/web/layoutdata.go`, `internal/admin/page_handler_test.go`
- One concern per file inside a package; the package is the unit of cohesion, not the file (`internal/web/flash.go`, `internal/web/headers.go`, `internal/web/render.go`)
- Platform-specific files use build-tag suffixes: `internal/web/diskspace.go` / `internal/web/diskspace_other.go`
- Templates: `snake_case.html` under `cmd/holzcloud/templates/admin/` and `cmd/holzcloud/templates/public/default/`

**Packages:**
- Short, lowercase, single word: `page`, `admin`, `public`, `tmplmgr`, `sharelink`, `db`, `web`
- All application code lives under `internal/`; only `sdk/` and `cmd/` are outside
- Import alias only when a package name collides with a local identifier: `tmpl "github.com/holzcloud/holzcloud-cms/internal/template"` (`internal/admin/page_handler_test.go:26`)

**Functions:**
- Exported: `PascalCase` with a doc comment starting with the identifier name
- Handlers: `Handle` + entity + action — `HandlePageList`, `HandlePageCreate`, `HandlePageStatusToggle` (`internal/admin/page.go`)
- Constructors: `NewX` returning `*X` — `NewStore`, `NewHandler`, `NewResolver`, `NewLoader`
- Setters for post-construction wiring: `SetPlugins`, `SetMail`, `SetAITokens` (`internal/admin/handler.go:196-211`)
- Unexported helpers are lowercase and terse: `scanPage`, `parseOptionalTime`, `seed`, `serve`

**Variables:**
- `camelCase`; short receiver names (`h`, `s`, `p`, `t`)
- Sentinel errors: `ErrX` at package level — `ErrConflict`, `ErrSlugTaken`, `ErrNotFound`, `ErrRouteTaken`
- SQL projections hoisted into package constants: `pageColumns` (`internal/page/store.go:73`)

**Types:**
- `PascalCase` nouns: `Store`, `Handler`, `Flash`, `Page`, `PageCreate`, `PageUpdate`
- Input structs for multi-field operations rather than long parameter lists: `PageCreate`, `PageUpdate` (`internal/page/store.go`)

**SQL:**
- `snake_case` table and column names; STRICT tables where possible
- Migrations are versioned `.sql` files in `internal/db/migrations/`, embedded and run with `pressly/goose/v3`

**CSS:**
- `kebab-case` classes (`.page-header`, `.nav-item.is-active`) in `cmd/holzcloud/assets/admin.css` and `bausteine.css`

## Code Style

**Formatting:**
- `gofmt` is mandatory and enforced in CI — the `Verify formatting` step in `.github/workflows/ci.yml` fails the build on any file listed by `gofmt -l .`
- Tabs for indentation (gofmt default); no line-length rule, but struct literals in tests are folded onto shared lines to keep cases readable

**Linting:**
- `go vet ./...` in both `.github/workflows/ci.yml` and `.github/workflows/security.yml`
- `go mod tidy` must be a no-op — CI runs it and then `git diff --exit-code -- go.mod go.sum`
- No golangci-lint or third-party linter configured; stdlib tooling only

**Stack constraints that act as style rules** (see `CLAUDE.md`):
- stdlib-first Go; a new dependency needs a real justification
- Plain CSS, no build step; htmx 2.0.x is the only JavaScript
- Nothing may be fetched from a third party at runtime — enforced by the CSP in `internal/web/headers.go` and by upload rejection in `internal/tmplmgr/external.go`

## Import Organization

**Order** (gofmt groups, blank line between):
1. Standard library — `context`, `database/sql`, `errors`, `fmt`, `log/slog`, `net/http`
2. Third-party — `github.com/alexedwards/scs/v2`, `github.com/pressly/goose/v3`
3. Internal — `github.com/holzcloud/holzcloud-cms/internal/...`

Example: `internal/public/handler.go:1-20`, `internal/page/store.go:1-12`.

**Path Aliases:**
- None. Full module paths (`github.com/holzcloud/holzcloud-cms/internal/page`) everywhere.

## Error Handling

**Sentinel errors per package**, declared at the point of use with a human-readable message:
- `internal/page/store.go:159` `ErrConflict = errors.New("page was modified by someone else")`
- `internal/field/store.go:13-35` — a family of German-language sentinels (`ErrDuplicateKey`, `ErrConditionLoop`, …)
- Callers compare with `errors.Is`, never with string matching (`internal/page/store_test.go:71`)

**Handlers return `error`**; a single wrapper turns that into an HTTP response:
```go
func (h *Handler) ErrHandler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			slog.Error("handler error", "err", err, "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
```
`internal/admin/handler.go:187`. Routes wire it at registration: `adminProtectedMux.HandleFunc("GET /admin/websites/{id}/pages", adminHandler.ErrHandler(adminHandler.HandlePageList))` (`cmd/holzcloud/main.go:686`).

**Patterns:**
- Guard clauses and early returns; the happy path stays at the left margin
- Wrap with `fmt.Errorf("...: %w", err)` when crossing a layer boundary; ~612 `errors.New`/`fmt.Errorf` sites across `internal/`
- User-facing failures become flash messages, not HTTP errors: `web.SetFlashError(...)` then redirect (`internal/web/flash.go:25`)
- Non-fatal side-effect failures are logged and swallowed so the primary action still completes (`internal/admin/page.go:804-860`)
- `defer rows.Close()` on every query; no ORM, prepared statements only

## Logging

**Framework:** `log/slog` (stdlib), ~141 call sites in `internal/`.

**Patterns:**
- Structured key/value pairs, never formatted strings: `slog.Error("load page terms", "err", err, "page", pageID)`
- Message is a lowercase noun phrase describing the operation that failed
- `slog.Error` for unexpected failures only; expected user errors go to flash messages
- Request logging middleware lives in `internal/web/logging.go`

## Comments

**When to Comment:**
- Doc comments on every exported identifier, starting with the identifier name (`// Store handles SQL operations for pages.`)
- Comments explain *why*, and are written as full prose paragraphs. Long rationale comments are normal and expected — see `internal/admin/handler.go:196-204` (why `SetPlugins` exists instead of a constructor argument) and `internal/web/flash.go:11-15` (why flashes are translated on write).
- Test functions carry a comment stating the real-world scenario being defended (`internal/page/store_test.go:51`)
- Security-sensitive code documents its safety argument inline — `internal/web/render.go:84-88` explains why the `th` template func may return `template.HTML`

**JSDoc/TSDoc:** Not applicable — no JavaScript beyond vendored `htmx.min.js`.

## Function Design

**Size:** Handlers run long (100–200 lines) because they do parse → authorize → mutate → render in one readable sequence. Store methods stay short; shared row-scanning is factored out (`scanPage`).

**Parameters:** Context first (`ctx context.Context`), then IDs, then an input struct for anything with more than three fields. Constructors take explicit dependencies rather than a service locator — `NewHandler` in `internal/admin/handler.go` takes them all positionally.

**Return Values:** `(*T, error)` for lookups, `error` alone for mutations. Optional DB values are scanned into `sql.NullX` and converted to pointers or zero values in one place (`internal/page/store.go:28-72`).

## Module Design

**Exports:** Each `internal/` package exposes a `Store` (data) or `Handler` (HTTP) plus its models and sentinel errors. Cross-package coupling goes through exported helpers rather than duplicated SQL — `page.ColumnsFor(alias)` (`internal/page/store.go:75`) exists so a new page column reaches joining packages automatically.

**Barrel Files:** None. Go packages are the only grouping mechanism.

**Context values:** Typed helpers per package instead of raw keys — `domain.WebsiteToContext`, `i18n.Lang(ctx)`.

## Internationalisation

- Source strings are written in German and translated at the point they are stored or rendered
- Go side: `i18n.T(i18n.Lang(ctx), msg)` — used inside `web.SetFlash*` so the ~150 call sites stay plain German sentences
- Template side: `t` (plain), `th` (sentence with inline markup), `tf` (sentence with a value) registered in `internal/web/render.go:76`
- `go test ./internal/i18n/` guards catalogue completeness and rejects Swiss-German spellings (see `README.md:649`)

---

*Convention analysis: 2026-08-22*
