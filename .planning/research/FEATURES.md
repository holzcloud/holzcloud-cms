# Feature Landscape

**Domain:** Minimal self-hosted multi-site CMS
**Stack:** Go 1.22+ / htmx / SQLite / single binary
**Researched:** 2026-04-13
**Source:** PROJECT.md mandate + domain knowledge; no external research required for scope decisions

---

## Table Stakes

Features a CMS must have or it is not a CMS. Missing any of these = incomplete product.

| Feature | Why Expected | Complexity | v1? | Notes |
|---------|--------------|------------|-----|-------|
| Login / logout | Entry gate to all admin work | S | YES | Session cookie, bcrypt password hash |
| Session management | Keeps user logged in across requests | S | YES | Secure cookie flags; server-side session store in SQLite or in-memory map |
| CSRF protection | Every state-changing POST must be guarded | S | YES | Token in hidden field; middleware validates |
| Create / edit / delete pages | Core authoring action | M | YES | Slug, title, content (Markdown), draft/published |
| Slug routing on public site | Readers reach pages by URL | S | YES | Per-website slug namespace |
| Draft / published state | Authors can work without publishing | S | YES | Simple boolean or enum field |
| 404 handling | Graceful missing-page response | S | YES | Per-website 404 template or fallback |
| Create / edit / delete websites | Without this, multi-site is impossible | S | YES | name, slug-identifier, active flag |
| Assign domains to websites | Host-based routing requires domain → site mapping | S | YES | `website_domains` table; one site, many domains |
| Template assignment per website | Different sites can look different | M | YES | Active template stored in DB; resolved at request time |
| Server-rendered public pages | Readers get HTML, not JSON | M | YES | Go `html/template`; no client JS required |
| Admin layout / navigation | Authors need a coherent UI shell | M | YES | Shared layout template with site switcher |

---

## Differentiators

What makes Holzcloud stand out versus WordPress, Ghost, or a flat-file CMS.

| Feature | Value Proposition | Complexity | v1? | Notes |
|---------|-------------------|------------|-----|-------|
| Single Go binary | Zero dependency install; `scp` binary + run | S | YES | `go build`; no PHP, no Node, no Ruby |
| Pi 5 native ARM64 build | Genuinely runs well on $80 hardware | S | YES | `GOARCH=arm64 GOOS=linux go build` cross-compiles |
| Multi-site from one process | Run 10 sites on one Pi without 10 processes | M | YES | Host header → website resolver at request time |
| Multiple domains per website | Canonical + alias domains both work | S | YES | `website_domains` table; all domains map to same site |
| htmx-driven admin UI | Fast inline edits without full-page reloads | M | YES | Inline validation, partial list refresh, no SPA complexity |
| No external services | SQLite on disk; no Redis, no Postgres, no S3 | S | YES | Deployment is: binary + data/ directory |
| Template isolation per website | Each site has its own look; templates are plain Go template dirs | M | YES | Stored in `data/templates/<site-id>/`; safe path join |
| Markdown authoring | Lightweight, no WYSIWYG needed | S | YES | Server-side render (goldmark or stdlib-adjacent); stored as Markdown, rendered at request or on save |
| Hierarchical menus | Authors build nav without hardcoding | M | YES | Menu items with parent_id; ordered; attached to website |
| Per-site media uploads | Images per site stored in `data/uploads/<site-id>/` | M | YES | Multipart form; safe filename; served with correct MIME |

---

## Anti-Features

Deliberately excluded. Saying NO is as important as saying YES.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Visual drag-and-drop builder | Requires heavy JS; contradicts htmx-only rule | Markdown content + template system covers 95% of layout needs |
| Rich-text WYSIWYG (TipTap, Quill, CKEditor) | External JS bundles; complex sanitization; contradicts stack | Markdown textarea; authors learn Markdown or paste HTML sparingly |
| React / Vue / Svelte frontend | SPA complexity; build tooling; violates stack mandate | htmx + server-rendered partials |
| Multi-tenant billing / signup | Scope creep; single operator target | Admin creates sites manually; no public registration |
| Real-time collaboration | Requires websockets + conflict resolution; unnecessary for 1-person CMS | Single-author model is fine for v1 |
| External DB (Postgres, MySQL, Redis) | Defeats Pi-friendly zero-dependency goal | SQLite handles the load of a multi-site personal CMS trivially |
| Docker-first deployment | Adds complexity for the primary audience | Plain binary + systemd unit file |
| Bundlers / PostCSS / Tailwind | Build steps; contradicts "no build tools" | Handwritten CSS with custom properties |
| Plugin/extension system | Complex API surface; premature abstraction | Direct code changes; templates are the extension point |
| Email delivery (SMTP) | External service dependency; complex config | No password reset email in v1 (admin resets via CLI or direct DB update) |
| Full-text search | Requires SQLite FTS setup or external index | Not expected at personal-site scale; add in v2 if needed |
| RSS / sitemap generation | Nice-to-have, not core | v2; trivially added as a route returning XML |
| Scheduled publishing | Cron-level complexity for an edge case | Authors set published=true manually |
| Content versioning / history | Storage and UI complexity | v2 if requested; use git on the data directory as a workaround |

---

## Feature Detail by Domain

### Auth & Access

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Login form + session | S | YES | — | POST /admin/login; set secure session cookie |
| Logout | S | YES | session | DELETE session; redirect to login |
| CSRF token middleware | S | YES | session | Token stored in session; validated on every POST |
| Password hashing | S | YES | — | `golang.org/x/crypto/bcrypt`; no plaintext ever stored |
| Role: admin | S | YES | auth | Full access to all websites and settings |
| Role: editor | M | YES | auth | Scoped to assigned websites; cannot manage users or system settings |
| Password reset (self-service email) | L | NO | email, SMTP | Excluded in v1; admin resets via CLI or direct DB |
| User invite flow | M | NO | email | v2; for now admin creates users directly |
| Two-factor authentication | L | NO | — | v2 at earliest |

**Dependency chain:** session → CSRF → auth → role gates → all admin actions.

### Multi-Site Model

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Create / edit / delete website | S | YES | auth (admin) | name, identifier, active flag |
| Add / remove domains per website | S | YES | website | `website_domains` table; primary + alias support |
| Host → website resolution | S | YES | domains | Middleware runs on every public request |
| Per-site content isolation | S | YES | website | All pages, menus, templates scoped by `website_id` |
| Site switcher in admin | S | YES | website, auth | Dropdown or sidebar; switches active site context |
| Per-site active template | S | YES | website, templates | FK to `templates.id` stored on website row |

### Template Management

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Upload template (zip or directory) | M | YES | website, auth (admin) | Unzip to `data/templates/<site-id>/<name>/`; safe path validation |
| Activate template for website | S | YES | website, templates | UPDATE websites SET template_id = ? |
| List templates for a website | S | YES | website, templates | Index page in admin |
| Delete template | S | YES | templates | Refuse if template is active on any site |
| Preview template | M | v2 | templates, pages | Requires sandbox render; defer |
| Multiple templates per site (switchable) | S | YES | templates | The data model supports it natively; UI lists all |
| Template partial overrides | M | v2 | templates | Override individual partials per site; nice but not critical |

**Template format:** directory containing `page.html`, `menu.html`, `layout.html` Go templates plus a `template.json` manifest. Served static assets live under `template/assets/`.

### Page Authoring

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Create page (slug, title, content) | S | YES | website, auth | Markdown textarea; slug auto-generated from title |
| Edit page | S | YES | pages | Same form as create |
| Delete page | S | YES | pages | Soft-delete or hard-delete; hard-delete simpler for v1 |
| Draft / published toggle | S | YES | pages | `status` enum: draft / published |
| Publish date | S | YES | pages | `published_at` timestamp; pages with future date treated as draft |
| Slug uniqueness per website | S | YES | pages | DB unique constraint on (website_id, slug) |
| Markdown render to HTML | S | YES | pages | Server-side at request time or cached on save; goldmark is the standard Go library |
| Page order / weight | S | v2 | pages | Needed for menus, not pages themselves |
| Page meta (SEO title, description) | S | v2 | pages | Useful but not blocking MVP |
| Inline htmx editing (click-to-edit fields) | M | YES | pages, htmx | Title / status editable inline; content editing in full form |

### Menu Builder

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Create menu | S | YES | website, auth | Name + website association |
| Add menu items (label, URL or page link) | S | YES | menu | `menu_items` with label, href, parent_id, sort_order |
| Hierarchical items (parent/child) | M | YES | menu_items | parent_id self-reference; render as nested `<ul>` |
| Reorder menu items | M | YES | menu_items | htmx drag or up/down buttons; sort_order integer |
| Assign menu to website location | S | YES | menu, website | Location key (e.g. "primary", "footer") stored per menu |
| Delete menu / item | S | YES | menu | Cascade delete items |
| Multiple menus per website | S | YES | menu | Supported by schema naturally |

### Media / Asset Handling

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Upload image / file | M | YES | website, auth | Multipart; validate MIME; store in `data/uploads/<site-id>/` |
| Safe filename handling | S | YES | uploads | Reject path traversal; sanitize filename; prefer UUID-prefixed names |
| Serve uploaded assets | S | YES | uploads | Go `http.ServeFile`; set Content-Type from extension |
| List uploaded assets | S | YES | uploads | Simple file listing or DB-tracked; DB-tracked preferred for deletion |
| Delete asset | S | YES | uploads | Remove file; remove DB record |
| Image resize / thumbnails | L | NO | uploads | Out of scope; serve originals |
| CDN integration | L | NO | — | Out of scope; Pi serves assets directly |
| Per-site asset isolation | S | YES | website, uploads | `data/uploads/<site-id>/` enforces isolation |

### Public Rendering

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Host → website routing | S | YES | domains | Middleware; 404 website → generic error page |
| Slug → page lookup | S | YES | pages, website | SELECT WHERE website_id = ? AND slug = ? AND status = 'published' |
| Render page via active template | M | YES | templates, pages | Go `html/template` Execute with page data |
| 404 page (per-site or fallback) | S | YES | templates | Template "404.html" or fallback plain text |
| Cache-Control headers | S | YES | — | Short TTL (e.g. 60s) for public pages; ETag optional |
| Homepage route (/) | S | YES | pages, website | Slug "" or "home" convention; configurable per website |
| Asset serving from template dir | S | YES | templates | `/assets/` prefix → template asset directory |
| Robots.txt / favicon.ico | S | v2 | website | Per-site configurable; default fallback in v2 |
| HTTP → HTTPS redirect | S | YES | — | Handled by reverse proxy (Caddy); binary need not manage TLS |

### Admin Dashboard

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Global dashboard (all sites overview) | S | YES | website, auth | List of websites with page/domain counts; quick links |
| Per-site dashboard | S | YES | website, pages, menus | Recent pages, menu count, active template name |
| Quick actions (new page, new menu item) | S | YES | dashboard | Links; no special logic |
| User list (admin only) | S | YES | auth (admin) | Manage who can log in |
| Activity log | M | NO | — | v2; not critical for single-operator use |

### Settings

| Feature | Complexity | v1? | Dependencies | Notes |
|---------|------------|-----|-------------|-------|
| Website name / description | S | YES | website | Displayed in admin; available to templates as template var |
| Default template per website | S | YES | website, templates | FK on website row |
| Timezone per website | S | v2 | website | Rarely needed for static content sites; use UTC in v1 |
| Maintenance mode per website | M | v2 | website | Return 503; not blocking MVP |
| System settings (admin-only) | S | YES | auth (admin) | Global defaults; trusted proxy config; dev seed toggle |

---

## Feature Dependencies (DAG Summary)

```
auth (session + CSRF + password hash)
  └── role gates (admin / editor)
        └── all admin CRUD actions

website (create / resolve)
  ├── website_domains → public Host routing
  ├── pages (scoped by website_id)
  ├── menus (scoped by website_id)
  ├── templates (scoped by website_id, active template FK)
  └── uploads (scoped by website_id)

pages
  └── public slug routing → template render

templates
  └── public render (page + menu + asset serving)

menus + menu_items
  └── template render (menu variable injected into template context)
```

---

## MVP Recommendation (v1 Scope)

**Must ship in v1:**

1. Auth (login, session, CSRF, bcrypt, admin + editor roles)
2. Website + domain management (create sites, assign domains)
3. Page CRUD (slug, title, Markdown content, draft/published)
4. Template management (upload, activate, list, delete)
5. Public rendering (host routing, slug routing, template execution, 404, cache headers)
6. Menu builder (create, hierarchical items, reorder, assign to website location)
7. Media uploads (upload, serve, list, delete, per-site isolation)
8. Admin dashboard (global overview + per-site view)
9. System settings (site name, active template)

**Defer to v2:**

- Template preview / partial overrides
- Page meta (SEO title, description)
- Publish scheduling (future `published_at`)
- Robots.txt / sitemap / RSS
- Maintenance mode
- Timezone per website
- Full-text search
- Content versioning
- Activity log
- Email-based password reset / user invites
- 2FA

---

## Sources

- PROJECT.md (stack mandate, v1 out-of-scope list, key capabilities) — HIGH confidence
- Domain knowledge of self-hosted CMS conventions — MEDIUM confidence (standard table stakes are stable; differentiators verified against PROJECT.md mandate)
