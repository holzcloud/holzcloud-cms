## Project

**Holzcloud CMS** — A minimal, self-hosted CMS for a small linux/amd64 server. Single Go binary. Manages multiple websites with multiple domains. Admin UI via htmx. Public site via server-rendered templates. SQLite storage.

### Hard Stack Constraints

- **Go 1.22+** — entire backend; stdlib-first
- **htmx 2.0.x** — only permitted JS (self-hosted, no CDN)
- **Plain CSS** — no Tailwind, Sass, PostCSS, or build tools
- **SQLite** — via `modernc.org/sqlite` (pure Go, no CGO)
- **No npm, no Node, no bundlers, no other JS frameworks**

### Nothing Loads at Runtime

**Nothing may be fetched from a third party while the application runs.** No CDN
scripts, no external stylesheets, no web fonts, no remote images, no analytics,
no iframes pointing off-site. Every subresource a browser requests must come
from this server's own origin. This applies to admin templates, the shipped
public templates, and user-uploaded templates alike.

Only two things may be downloaded **at build time**:

1. **Go modules** — normal `go get` / `go mod tidy`
2. **Fonts** — fetched once, committed to the repository and embedded via
   `embed.FS`, then served from `/assets/`. A font is never referenced by URL.
   A font belonging to a single theme may instead live in that theme's own
   directory and be served from `/t/fonts/…`, which is the shape
   TEMPLATE-SPEC §2.1 documents; it is still committed and still embedded, and
   `/t/` is the route that gives it the right MIME type and cache headers.

An outbound hyperlink (`<a href="https://…">`) is content, not a subresource,
and stays allowed.

Enforced in two places, keep both working:
- `internal/web/headers.go` — `Content-Security-Policy` (`default-src 'self'`)
  on every response, so a violation cannot execute in the browser.
- `internal/tmplmgr/external.go` — template archives are rejected at upload if
  they reference an external subresource, so the admin sees an error instead of
  a silently broken page.
- `internal/tmplmgr/script.go` — the same for JavaScript. Uploaded templates
  carry none: not a `.js` file, not an inline `<script>`, not an `onclick`
  attribute, not a `javascript:` URL. A `<script type="application/ld+json">`
  data block is exempt because the browser never executes one.

### Key Dependencies

| Package | Purpose |
|---------|---------|
| `modernc.org/sqlite` | Pure-Go SQLite driver (static build, no C toolchain) |
| `alexedwards/scs/v2` | Server-side sessions in SQLite |
| `golang.org/x/crypto/argon2` | Argon2id password hashing |
| `gorilla/csrf` | CSRF middleware |
| `pressly/goose/v3` | Embedded SQL migrations |
| `github.com/yuin/goldmark` | Markdown → HTML |
| `github.com/microcosm-cc/bluemonday` | HTML sanitization |

## Conventions

### Package Layout
```
cmd/holzcloud/main.go       — entry point
internal/config/             — env-based config
internal/db/                 — SQLite open, dual-pool, query helpers
internal/db/migrations/      — goose .sql files (embed.FS)
internal/auth/               — Argon2id, session, CSRF, middleware
internal/domain/             — Website, Domain models + resolver middleware
internal/admin/              — Admin handlers (CRUD)
internal/public/             — Public site handlers (page render)
internal/template/           — Template loader (embed + disk), render check
internal/tmplspec/           — The template authoring specification (embedded)
internal/media/              — Media upload/serve
internal/web/                — Shared: flash, layout data, helpers
templates/admin/             — Admin Go templates (embed.FS)
templates/public/default/    — Default public template (embed.FS)
assets/                      — admin.css, htmx.min.js (embed.FS)
data/                        — Runtime: SQLite DB + media + user templates
```

### Naming
- Go files: `snake_case.go`
- Packages: short, lowercase, single-word where possible
- Handlers: `func (h *Handler) HandlePageList(w, r)` — `Handle` prefix + entity + action
- Templates: `snake_case.html` in `templates/admin/` and `templates/public/`
- CSS classes: `kebab-case` (`.page-header`, `.nav-item.is-active`)
- SQL: `snake_case` for tables/columns; named params via `$name`

### Database
- **Dual-pool:** write pool (`SetMaxOpenConns(1)`), read pool (higher). WAL mode.
- **Pragmas on every connection:** `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL`
- **Migrations:** `pressly/goose/v3` with `embed.FS`; versioned `.sql` files in `internal/db/migrations/`
- **Direct SQL:** no ORM. Prepared statements. Always `defer rows.Close()`.
- **STRICT tables** where possible.

### htmx Integration
- Check `HX-Request: true` header to return partial vs full page
- Set `Vary: HX-Request` on any handler that branches on this header
- CSRF token in `<body hx-headers='{"X-CSRF-Token":"{{.CSRFToken}}"}'>` — server validates header
- Use `HX-Redirect` (not 302) for post-mutation navigation from htmx requests
- `hx-disabled-elt="this"` on all submit buttons to prevent double-submit

### Templates
- **The data contract is documented in exactly one place**: the structs in
  `internal/template/loader.go`, the fixtures `SampleData`/`MinimalData` in
  `sample.go`, and `internal/tmplspec/TEMPLATE-SPEC.md`. Tests tie the three
  together — a new field that is missing from a fixture or from the
  specification fails the suite, because a template author (often an AI agent)
  follows that document literally.
- **Uploads are rendered before they are accepted** (`template.Check`), against
  a full page and against one with every optional field empty. The second case
  is what catches `{{.Page.Next.URL}}`. `holzcloud template check` runs exactly
  the same checks without installing anything.
- Go `html/template` with `template.FuncMap` for helpers
- Admin templates parsed at startup from `embed.FS`
- Markdown → HTML: `goldmark` → `bluemonday` sanitization → `template.HTML` cast. **Never** cast unsanitized goldmark output.
- Layout composition via `{{template "base" .}}` + `{{define "content"}}...{{end}}`

### CSS
- `@layer` cascade: reset, tokens, base, layout, components, utilities
- OKLCH color tokens via custom properties
- Container queries for component-level responsiveness
- 8px spacing scale; system font stack
- View transitions (`@view-transition`) for admin navigation

### Error Handling
- Handlers return `error`; a wrapper writes the appropriate HTTP response
- Early returns / guard clauses preferred
- `slog.Error` for unexpected errors; flash messages for user-facing errors

### Security
- Argon2id for passwords (tune parameters to the server)
- Session rotation on login
- CSRF on all state-changing requests (including htmx)
- Zip-slip prevention on template upload
- Draft pages NEVER leak to public queries
- bluemonday sanitization on all user-generated HTML

### Build
- `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud`
- Cross-compile: `GOOS=linux GOARCH=amd64`
- All templates/assets/migrations embedded — binary is self-contained

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
