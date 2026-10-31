# Phase 03: Multi-Site + Pages + Public Rendering - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Mode:** Auto-generated (--auto mode)

<domain>
## Phase Boundary

Admins can manage multiple websites with multiple domains; editors can author and publish Markdown pages; public visitors see rendered pages served through the correct site's template, with draft pages returning 404.

Requirements: SITE-01, SITE-02, SITE-03, SITE-04, PAGE-01, PAGE-02, PAGE-03, PAGE-04, PAGE-05, PAGE-06, PUB-01, PUB-02, PUB-03, PUB-04, PUB-05

</domain>

<decisions>
## Implementation Decisions

### Website CRUD (SITE-01, SITE-02)
- **D-01:** Websites table: id, name, description, active (boolean), created_at, updated_at. STRICT table.
- **D-02:** Website domains table: id, website_id (FK), domain (unique), is_primary (boolean), created_at. One website can have many domains.
- **D-03:** Admin UI for websites at `/admin/websites` — list view with create/edit/delete. Edit form includes inline domain management (add/remove domains).
- **D-04:** Website name is required, description optional. Active flag controls whether the site serves public traffic.
[auto] Selected: recommended approach — standard CRUD with domain association table.

### Host-Based Routing (SITE-03, SITE-04)
- **D-05:** Middleware reads `Host` header, strips port, looks up domain in `website_domains` table. Matched website stored in request context via `context.WithValue`.
- **D-06:** Domain lookup uses a read-through cache (sync.Map or similar) invalidated on domain CRUD operations. Avoids DB query per request.
- **D-07:** Unrecognized hosts return 404 with a minimal error page (no site data leaked). No configurable default site in v1.
- **D-08:** New package `internal/domain/` for resolver middleware and website context helpers.
[auto] Selected: recommended approach — middleware with cached domain lookup.

### Page Authoring (PAGE-01, PAGE-02, PAGE-03)
- **D-09:** Pages table: id, website_id (FK), title, slug, content_markdown, content_html, status ('draft'/'published'), published_at, created_at, updated_at. STRICT table.
- **D-10:** Markdown → HTML pipeline: goldmark renders raw HTML, bluemonday sanitizes with `UGCPolicy()` (strips scripts, on* attributes). Both raw markdown and sanitized HTML stored on save.
- **D-11:** Slug auto-generated from title on creation (lowercase, hyphens, ASCII transliteration). Editable. Unique constraint per website_id.
- **D-12:** Admin page editor at `/admin/websites/{id}/pages` — list and `/admin/websites/{id}/pages/{id}/edit` — edit form with title, slug, content textarea, status dropdown, publish date.
[auto] Selected: recommended approach — store both markdown and rendered HTML.

### Draft/Published Workflow (PAGE-04)
- **D-13:** Public queries always include `AND status = 'published'`. Draft pages return 404 on public site. No preview URL for drafts in v1.
- **D-14:** Status toggle in admin page list via htmx (quick publish/unpublish without opening editor).
[auto] Selected: recommended approach — strict published-only public queries.

### Admin Page List (PAGE-05, PAGE-06)
- **D-15:** Paginated page list with htmx partial swap. 20 pages per page. Pagination controls load next/prev via `hx-get` with `hx-target` on the table body.
- **D-16:** Inline editing: click page title → transforms to input field via htmx swap. Save via Enter or blur. Slug field shown below title, also editable inline. Cancel via Escape.
- **D-17:** Page list shows: title, slug, status badge, published date, actions (edit, delete). Sortable by title/date (server-side).
[auto] Selected: recommended approach — htmx partial pagination + click-to-edit.

### Public Template Engine (PUB-01, PUB-02)
- **D-18:** Template resolution order: disk (`data/templates/{website_id}/{template_name}/`) → embedded default (`templates/public/default/`). This allows custom templates while ensuring the default always works.
- **D-19:** Default template: `layout.html` (base with header/nav/footer), `page.html` (single page), `home.html` (homepage), `404.html` (not found). Minimal, clean CSS included.
- **D-20:** Template data struct includes: Site (name, description), Page (title, content HTML, published date), Menus (empty for now — Phase 4), Meta (canonical URL, etc).
- **D-21:** New package `internal/public/` for public site handlers. New package `internal/template/` for template loading and rendering.
[auto] Selected: recommended approach — disk-first with embedded fallback.

### Caching Headers (PUB-03, PUB-05)
- **D-22:** Page responses: `Cache-Control: public, max-age=300` (5 min), `ETag` based on content hash (MD5 of rendered HTML), `Last-Modified` from page updated_at. Support `If-None-Match` / `If-Modified-Since` → 304.
- **D-23:** Template static assets: `Cache-Control: public, max-age=31536000, immutable` with content-hash in filename or query string for cache busting.
[auto] Selected: recommended approach — standard HTTP caching with ETag.

### Public 404 Page (PUB-04)
- **D-24:** Unknown slugs render `404.html` from the site's active template with 404 status code. If no custom 404 template, use the embedded default 404.
[auto] Selected: recommended approach.

### Claude's Discretion
- Exact goldmark extensions enabled (tables, autolinks, strikethrough) — implementer decides
- Page editor textarea height and any preview mechanism — implementer decides
- Pagination style (numbered pages vs prev/next only) — implementer decides
- Default template visual design — implementer decides (keep minimal, clean)
- Whether to use a homepage flag on a page or a separate homepage concept — implementer decides

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Configuration
- `.planning/REQUIREMENTS.md` — Full requirement definitions for SITE-01–04, PAGE-01–06, PUB-01–05
- `.planning/ROADMAP.md` — Phase 3 success criteria and plan structure
- `CLAUDE.md` — Stack constraints, naming conventions, database patterns, security rules

### Prior Phase Code
- `internal/db/db.go` — DB struct with Write/Read pools, connection setup
- `internal/db/migrations/` — Existing goose migration files (schema patterns to follow)
- `internal/auth/middleware.go` — RequireAuth/RequireAdmin middleware (pattern for new middleware)
- `internal/admin/handler.go` — Handler struct pattern, ErrHandler wrapper
- `internal/web/render.go` — Template parsing, layout data, flash helpers
- `templates/admin/base.html` — Admin base template (pattern for public templates)
- `assets/admin.css` — OKLCH design system tokens (reuse in admin page styling)

### Phase 2 Decisions
- `.planning/phases/02-auth-admin-shell/02-CONTEXT.md` — Design tokens, layout patterns, CSRF integration

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/web/render.go` — `ParseAdminTemplates`, `ErrHandler` wrapper — extend for public templates
- `internal/web/flash.go` — Flash message system — reuse in admin website/page CRUD
- `internal/web/layoutdata.go` — Layout data builder — extend for public site data
- `internal/admin/handler.go` — Handler struct pattern with DB + session manager + templates
- `internal/db/migrations/` — goose migration patterns to follow for new tables
- `assets/admin.css` — OKLCH tokens and @layer cascade — admin pages reuse existing styles

### Established Patterns
- Go 1.22+ ServeMux with `method path` patterns in `main.go`
- Handler methods on struct with `ErrHandler` wrapper: `func (h *Handler) HandleX(w, r) error`
- Templates parsed from `embed.FS` at startup, executed per-request with data struct
- Flash messages via session for POST-redirect-GET feedback
- CSRF via `hx-headers` on `<body>` for all htmx requests

### Integration Points
- `cmd/holzcloud/main.go` — Add domain resolver middleware, website/page admin routes, public routes
- `internal/db/migrations/` — Add migrations for websites, website_domains, pages tables
- Admin sidebar nav in `templates/admin/base.html` — Add Websites, Pages links
- Public routes registered after admin routes, catch-all for domain-resolved requests

</code_context>

<specifics>
## Specific Ideas

- Admin website/page management should maintain the same Linear/Ghost aesthetic from Phase 2
- Public default template should be extremely minimal — clean typography, whitespace, readable
- Markdown editor is a plain textarea (no WYSIWYG — out of scope per REQUIREMENTS.md)
- Inline editing on page list should feel snappy — htmx swap with minimal visual disruption

</specifics>

<deferred>
## Deferred Ideas

- SEO meta fields per page (V2-04)
- Scheduled publishing (V2-03)
- Content versioning (V2-02)
- Full-text search (V2-06)
- Draft preview URL
- Sitemap/robots.txt generation (V2-05)

</deferred>

---

*Phase: 03-multi-site-pages-public-rendering*
*Context gathered: 2026-04-14*
