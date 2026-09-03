# Phase 03: Multi-Site + Pages + Public Rendering - Research

**Researched:** 2026-04-14
**Domain:** Multi-tenant website routing, Markdown CMS, Go template rendering
**Confidence:** HIGH

## Summary

Phase 3 adds the core CMS functionality: websites with domains, pages with Markdown authoring, and public rendering. The entire stack is already established (Go stdlib, SQLite, htmx, goldmark, bluemonday) -- this phase is about data modeling, domain routing middleware, and a public template engine layered on top of the Phase 1-2 foundation.

The main architectural challenge is host-based routing middleware that resolves incoming requests to a website via cached domain lookup, then threading that website context through to both admin handlers (scoping page CRUD) and public handlers (rendering the correct template). The Markdown pipeline (goldmark -> bluemonday -> stored HTML) is straightforward. The public template engine needs a disk-first, embed-fallback resolution strategy.

**Primary recommendation:** Structure as 3 plans: (1) migrations + models + domain middleware, (2) admin CRUD for websites + pages, (3) public rendering engine + caching headers.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Websites table: id, name, description, active, created_at, updated_at. STRICT table.
- D-02: Website domains table: id, website_id (FK), domain (unique), is_primary, created_at.
- D-03: Admin UI for websites at `/admin/websites` with inline domain management.
- D-04: Website name required, description optional. Active flag controls public traffic.
- D-05: Middleware reads Host header, strips port, looks up domain in website_domains. Website stored in request context.
- D-06: Domain lookup uses read-through cache (sync.Map or similar) invalidated on domain CRUD.
- D-07: Unrecognized hosts return 404 with minimal error page.
- D-08: New package `internal/domain/` for resolver middleware and context helpers.
- D-09: Pages table: id, website_id (FK), title, slug, content_markdown, content_html, status, published_at, created_at, updated_at. STRICT table.
- D-10: Markdown pipeline: goldmark -> bluemonday UGCPolicy(). Store both markdown and sanitized HTML.
- D-11: Slug auto-generated from title. Unique per website_id. Editable.
- D-12: Admin page editor at `/admin/websites/{id}/pages` and `/admin/websites/{id}/pages/{id}/edit`.
- D-13: Public queries always `AND status = 'published'`. No draft preview URL.
- D-14: Status toggle in admin page list via htmx.
- D-15: Paginated page list, 20 per page, htmx partial swap.
- D-16: Inline editing: click title -> input field. Save on Enter/blur. Cancel on Escape.
- D-17: Page list columns: title, slug, status badge, published date, actions.
- D-18: Template resolution: disk (`data/templates/{website_id}/`) -> embedded default.
- D-19: Default template files: layout.html, page.html, home.html, 404.html.
- D-20: Template data: Site, Page, Menus (empty), Meta (canonical URL).
- D-21: New packages `internal/public/` and `internal/template/`.
- D-22: Cache-Control: public, max-age=300; ETag from content hash; Last-Modified; 304 support.
- D-23: Template static assets: immutable cache headers with content-hash busting.
- D-24: Unknown slugs render 404.html from site template with 404 status.

### Claude's Discretion
- Goldmark extensions (tables, autolinks, strikethrough)
- Page editor textarea height and preview mechanism
- Pagination style (numbered vs prev/next)
- Default template visual design (keep minimal)
- Homepage concept (flag on page vs separate)

### Deferred Ideas (OUT OF SCOPE)
- SEO meta fields (V2-04)
- Scheduled publishing (V2-03)
- Content versioning (V2-02)
- Full-text search (V2-06)
- Draft preview URL
- Sitemap/robots.txt (V2-05)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SITE-01 | Admin CRUD for websites | D-01 through D-04; admin handler pattern from Phase 2 |
| SITE-02 | Multiple domains per website | D-02; website_domains table with unique constraint |
| SITE-03 | Host-based routing middleware | D-05, D-06, D-08; sync.Map cache pattern |
| SITE-04 | 404 for unrecognized hosts | D-07; minimal error page |
| PAGE-01 | Page CRUD within a website | D-09, D-12; scoped to website_id |
| PAGE-02 | Markdown -> sanitized HTML | D-10; goldmark + bluemonday pipeline |
| PAGE-03 | Unique slugs per website | D-11; composite unique constraint |
| PAGE-04 | Draft pages return 404 publicly | D-13; WHERE status='published' in all public queries |
| PAGE-05 | Paginated page list via htmx | D-15; partial swap pattern |
| PAGE-06 | Inline title/slug editing | D-16; htmx click-to-edit pattern |
| PUB-01 | Public rendering with correct template | D-18, D-21; template resolution chain |
| PUB-02 | Template engine with disk-first fallback | D-18, D-19; internal/template package |
| PUB-03 | Cache-Control + ETag + Last-Modified | D-22; MD5 of content_html for ETag |
| PUB-04 | Styled 404 for unknown slugs | D-24; site template 404.html |
| PUB-05 | Template static assets with cache headers | D-23; content-hash query string |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go 1.22+ stdlib-first, no ORM, direct SQL with prepared statements
- htmx 2.0.x only JS, self-hosted
- Plain CSS only, no preprocessors
- SQLite via modernc.org/sqlite, dual-pool, WAL, STRICT tables
- No npm/Node/bundlers
- goldmark + bluemonday for Markdown (never cast unsanitized output)
- `@layer` cascade, OKLCH tokens, 8px spacing, system font stack
- Handler pattern: `func (h *Handler) HandleX(w, r) error` with ErrHandler wrapper
- Templates: embed.FS at startup, `{{template "base" .}}` composition
- Vary: HX-Request on any handler branching on that header
- HX-Redirect for post-mutation htmx navigation
- hx-disabled-elt="this" on submit buttons

## Standard Stack

### Core (already in project)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| modernc.org/sqlite | v1.48.2 | SQLite driver | Already in go.mod [VERIFIED: go.mod] |
| alexedwards/scs/v2 | v2.9.0 | Sessions | Already in go.mod [VERIFIED: go.mod] |
| gorilla/csrf | v1.7.3 | CSRF protection | Already in go.mod [VERIFIED: go.mod] |
| pressly/goose/v3 | v3.27.0 | Migrations | Already in go.mod [VERIFIED: go.mod] |

### New Dependencies for Phase 3
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/yuin/goldmark | v1.8.2 | Markdown -> HTML | Page save pipeline [VERIFIED: go list -m -versions] |
| github.com/microcosm-cc/bluemonday | v1.0.27 | HTML sanitization | After goldmark, before storage [VERIFIED: go list -m -versions] |

**Installation:**
```bash
go get github.com/yuin/goldmark@v1.8.2
go get github.com/microcosm-cc/bluemonday@v1.0.27
```

## Architecture Patterns

### New Package Structure
```
internal/domain/           # Website model, domain resolver middleware, context helpers
internal/domain/resolver.go    # Host -> Website middleware with sync.Map cache
internal/domain/context.go     # WebsiteFromContext / WebsiteToContext helpers
internal/domain/models.go      # Website, Domain structs
internal/domain/store.go       # SQL queries for websites + domains

internal/page/             # Page model, CRUD, markdown pipeline
internal/page/models.go        # Page struct
internal/page/store.go         # SQL queries for pages
internal/page/markdown.go      # goldmark + bluemonday render function

internal/public/           # Public site handlers
internal/public/handler.go     # HandlePage, HandleHome, Handle404

internal/template/         # Template loader with disk/embed fallback
internal/template/loader.go    # Load + cache templates per website
```

### Pattern 1: Domain Resolver Middleware
**What:** Middleware that resolves Host header to a Website, stores in context.
**When to use:** Wraps all public routes (not admin routes).
```go
// [ASSUMED] — standard Go middleware pattern
type Resolver struct {
    cache sync.Map // domain string -> *Website
    db    *db.DB
}

func (res *Resolver) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        host := stripPort(r.Host)
        if ws, ok := res.cache.Load(host); ok {
            website := ws.(*Website)
            if !website.Active {
                http.NotFound(w, r)
                return
            }
            ctx := context.WithValue(r.Context(), websiteKey, website)
            next.ServeHTTP(w, r.WithContext(ctx))
            return
        }
        // Cache miss: query DB
        website, err := res.lookupDomain(r.Context(), host)
        if err != nil || website == nil {
            http.NotFound(w, r)
            return
        }
        res.cache.Store(host, website)
        if !website.Active {
            http.NotFound(w, r)
            return
        }
        ctx := context.WithValue(r.Context(), websiteKey, website)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func (res *Resolver) InvalidateCache() {
    res.cache = sync.Map{} // or Range + Delete
}
```

### Pattern 2: Markdown Render Pipeline
**What:** goldmark -> bluemonday sanitization, returns safe HTML string.
```go
// [ASSUMED] — based on goldmark/bluemonday API patterns
import (
    "bytes"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/microcosm-cc/bluemonday"
)

var md = goldmark.New(
    goldmark.WithExtensions(
        extension.Table,
        extension.Strikethrough,
        extension.Linkify,
    ),
)

var sanitizer = bluemonday.UGCPolicy()

func RenderMarkdown(source string) (string, error) {
    var buf bytes.Buffer
    if err := md.Convert([]byte(source), &buf); err != nil {
        return "", err
    }
    return sanitizer.Sanitize(buf.String()), nil
}
```

### Pattern 3: Public Template Resolution
**What:** Load templates from disk first, fall back to embedded default.
```go
// [ASSUMED] — standard Go template + os.Stat pattern
func (l *Loader) LoadTemplate(websiteID int64, name string) (*template.Template, error) {
    // Try disk: data/templates/{websiteID}/{name}
    diskPath := filepath.Join(l.dataDir, "templates", strconv.FormatInt(websiteID, 10))
    if _, err := os.Stat(filepath.Join(diskPath, name)); err == nil {
        return template.ParseFiles(
            filepath.Join(diskPath, "layout.html"),
            filepath.Join(diskPath, name),
        )
    }
    // Fall back to embedded default
    return template.ParseFS(l.defaultFS, "layout.html", name)
}
```

### Pattern 4: ETag / 304 Response
**What:** Content-hash based ETag with conditional response.
```go
// [ASSUMED] — standard HTTP caching pattern
func serveWithETag(w http.ResponseWriter, r *http.Request, content []byte, modTime time.Time) {
    hash := md5.Sum(content)
    etag := `"` + hex.EncodeToString(hash[:]) + `"`
    
    w.Header().Set("ETag", etag)
    w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
    w.Header().Set("Cache-Control", "public, max-age=300")
    
    if match := r.Header.Get("If-None-Match"); match == etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    if since := r.Header.Get("If-Modified-Since"); since != "" {
        t, err := http.ParseTime(since)
        if err == nil && !modTime.After(t) {
            w.WriteHeader(http.StatusNotModified)
            return
        }
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write(content)
}
```

### Pattern 5: htmx Inline Edit (click-to-edit)
**What:** Click title -> swap to input, save on blur/Enter, cancel on Escape.
```html
<!-- Display mode -->
<td hx-get="/admin/websites/{{.WebsiteID}}/pages/{{.ID}}/edit-title"
    hx-trigger="click"
    hx-swap="innerHTML">
  {{.Title}}
</td>

<!-- Edit mode (returned by server) -->
<input type="text" name="title" value="{{.Title}}"
       hx-put="/admin/websites/{{.WebsiteID}}/pages/{{.ID}}/title"
       hx-trigger="blur, keyup[key=='Enter']"
       hx-swap="outerHTML"
       hx-target="closest td"
       onfocus="this.select()"
       onkeyup="if(event.key==='Escape'){htmx.ajax('GET',this.closest('td').getAttribute('hx-get'),{target:this.closest('td'),swap:'innerHTML'})}">
```

### Pattern 6: htmx Pagination
**What:** Server-rendered table body swapped via htmx.
```html
<div id="page-list">
  {{template "page-table-body" .}}
</div>

<!-- Pagination controls -->
<nav class="pagination">
  {{if .HasPrev}}
  <a hx-get="/admin/websites/{{.WebsiteID}}/pages?page={{.PrevPage}}"
     hx-target="#page-list" hx-swap="innerHTML"
     class="btn btn--sm">Previous</a>
  {{end}}
  <span class="text-muted">Page {{.Page}} of {{.TotalPages}}</span>
  {{if .HasNext}}
  <a hx-get="/admin/websites/{{.WebsiteID}}/pages?page={{.NextPage}}"
     hx-target="#page-list" hx-swap="innerHTML"
     class="btn btn--sm">Next</a>
  {{end}}
</nav>
```

### Anti-Patterns to Avoid
- **Querying domain table on every request:** Use sync.Map cache, invalidate on CRUD. [ASSUMED]
- **Forgetting `AND status = 'published'` in public queries:** Draft leakage is a security issue per CLAUDE.md.
- **Casting goldmark output to template.HTML without bluemonday:** XSS vulnerability. Always sanitize first.
- **Using `context.WithValue` with string keys:** Use unexported struct type as key to prevent collisions. [ASSUMED]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Markdown rendering | Custom parser | goldmark v1.8.2 | Edge cases in CommonMark spec |
| HTML sanitization | Regex stripping | bluemonday v1.0.27 | XSS bypass via encoding tricks |
| Slug generation | Simple regex | Dedicated function with transliteration | Unicode edge cases, collision handling |
| HTTP caching logic | Custom date parsing | `http.ParseTime`, `http.TimeFormat` | RFC compliance for If-Modified-Since |
| Session management | Custom cookies | scs (already in use) | Fixation, rotation, cleanup |

## Common Pitfalls

### Pitfall 1: Draft Page Leakage
**What goes wrong:** Public query forgets `WHERE status = 'published'`, exposing drafts.
**Why it happens:** Adding a new public query path (homepage, sitemap) without the filter.
**How to avoid:** Single function for public page queries that always includes the filter. Never query pages table directly from public handlers.
**Warning signs:** Any `SELECT` on pages in `internal/public/` without status filter.

### Pitfall 2: sync.Map Cache Staleness
**What goes wrong:** Domain added/removed but cache still has old data. New domain 404s, deleted domain still resolves.
**Why it happens:** Forgetting to call InvalidateCache after domain CRUD.
**How to avoid:** Every domain write operation (add, remove, update) must call resolver.InvalidateCache(). Pass the resolver to the admin handler or use a callback.
**Warning signs:** Domain changes not taking effect until server restart.

### Pitfall 3: Template Parsing on Every Request
**What goes wrong:** Parsing templates from disk on every page view kills performance on Pi.
**Why it happens:** Not caching parsed templates.
**How to avoid:** Parse once on first use, cache in memory. Invalidate cache when template files change (Phase 4 template upload). For now, parse on startup or first request per website.
**Warning signs:** High latency on public page views.

### Pitfall 4: Slug Uniqueness Race Condition
**What goes wrong:** Two pages created simultaneously with the same title get the same slug.
**Why it happens:** Check-then-insert without constraint.
**How to avoid:** UNIQUE constraint on (website_id, slug) in SQLite. Handle constraint violation error gracefully (append -2, -3 suffix). Write pool serializes writes anyway.
**Warning signs:** SQLite UNIQUE constraint violation errors in logs.

### Pitfall 5: Missing Vary Header
**What goes wrong:** CDN or browser cache serves htmx partial as full page (or vice versa).
**Why it happens:** Handler returns different content based on HX-Request but doesn't set `Vary: HX-Request`.
**How to avoid:** Set `Vary: HX-Request` in every handler that branches on HX-Request. The existing `RenderAdmin` does this -- ensure public handler does too.
**Warning signs:** Broken page layout after back-button navigation.

### Pitfall 6: Port in Host Header
**What goes wrong:** Domain lookup fails because Host header includes `:8080` in development.
**Why it happens:** Go's `r.Host` includes the port.
**How to avoid:** Strip port from Host before domain lookup: `host, _, _ := net.SplitHostPort(r.Host)` with fallback to raw value if no port.

## SQL Schema

### Migration 00003_websites.sql
```sql
-- +goose Up
CREATE TABLE websites (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    active      INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE TABLE website_domains (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    domain     TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

CREATE INDEX idx_website_domains_website_id ON website_domains(website_id);
CREATE INDEX idx_website_domains_domain ON website_domains(domain);

-- +goose Down
DROP TABLE IF EXISTS website_domains;
DROP TABLE IF EXISTS websites;
```

### Migration 00004_pages.sql
```sql
-- +goose Up
CREATE TABLE pages (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    website_id       INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    title            TEXT    NOT NULL,
    slug             TEXT    NOT NULL,
    content_markdown TEXT    NOT NULL DEFAULT '',
    content_html     TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at     TEXT,
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(website_id, slug)
) STRICT;

CREATE INDEX idx_pages_website_status ON pages(website_id, status);
CREATE INDEX idx_pages_website_slug ON pages(website_id, slug);

-- +goose Down
DROP TABLE IF EXISTS pages;
```

**Note on STRICT tables:** SQLite STRICT mode requires explicit type affinity. Use `INTEGER` for booleans (0/1), `TEXT` for strings/dates. The existing users table does NOT use STRICT (it lacks the keyword), but CONTEXT.md decisions require it. [VERIFIED: internal/db/migrations/00001_initial.sql lacks STRICT keyword]

## Code Examples

### Slug Generation
```go
// [ASSUMED] — standard approach, no external dependency needed
import (
    "regexp"
    "strings"
)

var (
    reNonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
    reDashes      = regexp.MustCompile(`-{2,}`)
)

func Slugify(title string) string {
    s := strings.ToLower(strings.TrimSpace(title))
    s = reNonAlphaNum.ReplaceAllString(s, "-")
    s = reDashes.ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    if s == "" {
        s = "untitled"
    }
    return s
}
```

### Context Helpers
```go
// [ASSUMED] — standard Go context pattern
type contextKey struct{ name string }

var websiteKey = &contextKey{"website"}

func WebsiteFromContext(ctx context.Context) *Website {
    ws, _ := ctx.Value(websiteKey).(*Website)
    return ws
}

func WebsiteToContext(ctx context.Context, ws *Website) context.Context {
    return context.WithValue(ctx, websiteKey, ws)
}
```

### Public Route Wiring in main.go
```go
// [ASSUMED] — based on existing main.go patterns
// Domain resolver wraps public routes only
resolver := domain.NewResolver(database)

publicHandler := public.NewHandler(database, templateLoader)
publicMux := http.NewServeMux()
publicMux.HandleFunc("GET /{slug}", publicHandler.ErrHandler(publicHandler.HandlePage))
publicMux.HandleFunc("GET /", publicHandler.ErrHandler(publicHandler.HandleHome))

// Public routes: resolver middleware -> handler
// Must come AFTER admin routes in mux registration
mux.Handle("/", resolver.Middleware(publicMux))
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | goldmark extension API uses `extension.Table`, `extension.Strikethrough`, `extension.Linkify` | Architecture Patterns | Low -- API is stable, verify at import time |
| A2 | bluemonday `UGCPolicy()` is the right policy for CMS content | Architecture Patterns | Low -- UGCPolicy is the standard for user content |
| A3 | sync.Map is adequate for domain cache (low cardinality, read-heavy) | Architecture Patterns | Low -- dozens of domains max on Pi |
| A4 | SQLite STRICT tables with TEXT for dates work with strftime defaults | SQL Schema | Medium -- verify STRICT + DEFAULT interaction |

## Open Questions

1. **Homepage concept**
   - What we know: D-19 mentions `home.html` template. No homepage flag defined on pages.
   - What's unclear: Is homepage the page with slug "" or "/" or a special flag?
   - Recommendation: Use slug "" (empty) or a dedicated `is_home` boolean on pages. Implementer's discretion per CONTEXT.md. Simplest: treat root URL "/" as looking for a page with slug "home" or the first published page.

2. **Admin page routes: nested under websites or flat?**
   - What we know: D-12 says `/admin/websites/{id}/pages`. This nests pages under websites.
   - What's unclear: Does the sidebar "Pages" link go to a cross-website page list or require selecting a website first?
   - Recommendation: Sidebar "Pages" link could redirect to the first website's pages or show a website selector. Keep it simple -- go to website list, click website to see its pages.

## Sources

### Primary (HIGH confidence)
- `go.mod` -- verified current dependency versions
- `go list -m -versions` -- verified latest goldmark v1.8.2, bluemonday v1.0.27
- Existing codebase (`internal/admin/handler.go`, `internal/web/render.go`, `internal/auth/middleware.go`, `cmd/holzcloud/main.go`) -- established patterns

### Secondary (MEDIUM confidence)
- goldmark/bluemonday API patterns from training data [ASSUMED]
- sync.Map usage patterns [ASSUMED]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all deps verified in registry, existing patterns established
- Architecture: HIGH -- straightforward CRUD + middleware + template rendering
- Pitfalls: HIGH -- well-known patterns for Go web apps, specific to this codebase
- SQL schema: HIGH -- follows existing migration patterns, decisions locked

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (stable domain, no fast-moving dependencies)
