# Requirements: Holzcloud CMS

**Defined:** 2026-04-14 (v1.0) · 2026-09-02 (v1.5)
**Core Value:** One Go binary on a Raspberry Pi runs several websites without dependency soup.

## v1.0 Requirements — shipped 2026-04-14

### Foundation

- [x] **FND-01**: Single Go binary compiles with `CGO_ENABLED=0 GOARCH=arm64` and starts an HTTP server on a configurable port
- [x] **FND-02**: SQLite database created automatically on first run in a configurable `data/` directory with WAL mode, busy_timeout, and foreign_keys enforced on every connection
- [x] **FND-03**: Schema migrations run automatically at startup via embedded SQL files (goose); adding a new `.sql` file and rebuilding is all that's needed
- [x] **FND-04**: Graceful shutdown on SIGTERM/SIGINT — in-flight requests finish within 10s before process exits
- [x] **FND-05**: Admin templates, default public template, static assets (CSS, htmx.min.js), and migrations are embedded in the binary via `embed.FS`
- [x] **FND-06**: Structured JSON logging via `slog` with configurable log level

### Authentication & Security

- [x] **AUTH-01**: Admin user can log in with email and password; password verified with Argon2id
- [x] **AUTH-02**: Sessions stored server-side in SQLite via `alexedwards/scs`; session ID rotated on login
- [x] **AUTH-03**: All non-GET admin requests protected by CSRF token validated via `gorilla/csrf`; htmx requests send the token via `hx-headers` on `<body>`
- [x] **AUTH-04**: Secure session cookies: HttpOnly, SameSite=Lax, Secure in production
- [x] **AUTH-05**: Role-based access: admin (full) and editor (content only) roles enforced by middleware
- [x] **AUTH-06**: First-run bootstrap: if no users exist, the first visit to `/admin` shows a setup form to create the initial admin account

### Multi-Site

- [x] **SITE-01**: Admin can create, edit, and delete websites (name, description, active flag)
- [x] **SITE-02**: Admin can assign multiple domains to a website; each domain resolves to exactly one website
- [x] **SITE-03**: Incoming requests are routed to the correct website by matching `Host` header against the `website_domains` table (middleware)
- [x] **SITE-04**: Requests for unrecognized hosts return a 404 (or a configurable default site)

### Pages

- [x] **PAGE-01**: Editor can create, edit, and delete pages within a website; fields: title, slug, Markdown content, status (draft/published), publish date
- [x] **PAGE-02**: Markdown content is rendered to HTML via goldmark and sanitized via bluemonday on save; both raw Markdown and rendered HTML are stored
- [x] **PAGE-03**: Slugs are unique per website and auto-generated from the title (editable)
- [x] **PAGE-04**: Only published pages are visible on the public site; draft pages return 404 publicly
- [x] **PAGE-05**: Page list in admin supports pagination via htmx partial swap
- [x] **PAGE-06**: Inline editing of page title/slug from the list view via htmx (click-to-edit, save inline)

### Public Site

- [x] **PUB-01**: Public site renders pages using the active template for the resolved website
- [x] **PUB-02**: Template engine uses Go `html/template` with per-site template directories; disk-first, embedded default fallback
- [x] **PUB-03**: Public responses include `Cache-Control`, `ETag`, and `Last-Modified` headers for static and page responses
- [x] **PUB-04**: Unknown slugs return a styled 404 page using the site's active template
- [x] **PUB-05**: Static template assets (CSS, images within a template directory) are served with correct MIME and long cache headers

### Templates

- [x] **TPL-01**: Admin can upload a template as a `.zip` archive; server extracts safely (zip-slip prevention, extension allow-list, size cap)
- [x] **TPL-02**: Admin can activate one template per website; the active template is used for public rendering
- [x] **TPL-03**: Template directories follow a convention: `layout.html`, `page.html`, `404.html`, `assets/` subdirectory
- [x] **TPL-04**: Templates are listed and deletable from the admin UI

### Menus

- [x] **MENU-01**: Admin can create multiple menus per website with a location key (e.g. "main", "footer")
- [x] **MENU-02**: Menu items are hierarchical (parent/child) with sort order; CRUD via admin
- [x] **MENU-03**: Menu item reordering in admin (up/down buttons; drag-and-drop deferred to v2)
- [x] **MENU-04**: Public templates can render a menu by location key with nested `<ul>` output

### Media

- [x] **MEDIA-01**: Editor can upload images/files per website via multipart form; files stored in `data/media/{website_id}/`
- [x] **MEDIA-02**: Upload validates MIME type against an allow-list and enforces a per-file size limit
- [x] **MEDIA-03**: Media files are served with correct `Content-Type` and cache headers
- [x] **MEDIA-04**: Admin UI lists uploaded media per website with delete action

### Admin UI

- [x] **UI-01**: Admin layout: sidebar navigation (desktop), responsive on mobile, sticky header with page title + primary action
- [x] **UI-02**: Design system: OKLCH color tokens, `@layer` cascade, 8px spacing scale, system font stack, clean/minimal aesthetic (Linear/Ghost/Vercel direction)
- [x] **UI-03**: All admin interactions work without htmx (full-page fallback); htmx adds inline editing, partial pagination, and smoother transitions
- [x] **UI-04**: Flash messages (success/error/warning) after form actions
- [x] **UI-05**: Admin dashboard shows overview of all websites with quick-action links
- [x] **UI-06**: View transitions for admin navigation where browser supports it
- [x] **UI-07**: Loading indicators (`hx-indicator`) for all htmx requests

### Users

- [x] **USR-01**: Admin can list, create, edit, and delete users (name, email, password, role)
- [x] **USR-02**: Password changes require re-entering the current password (or admin override)

### Deployment

- [x] **DEP-01**: `go build` produces a single self-contained binary for linux/arm64
- [x] **DEP-02**: Example systemd unit file with security hardening (`ProtectSystem`, `NoNewPrivileges`)
- [x] **DEP-03**: Example Caddy reverse-proxy config for TLS termination
- [x] **DEP-04**: `data/` directory holds SQLite DB + media + user templates; documented backup strategy

## v1.5 Requirements — Inhaltsmodell

The working list behind these is `docs/offene-punkte.md`, points 1, 2, 3, 4 and 6.

### Field kinds

- [ ] **FIELD-01**: Author can configure a Choice field to render as a row of buttons instead of a dropdown
- [ ] **FIELD-02**: Author can define a Multiple-Choice field and pick several values at once; the value survives save and reload
- [ ] **FIELD-03**: Author can define a Term field that picks from the tags a website already carries, and the rendered page shows the term's name
- [ ] **FIELD-04**: Author can define a Time field (`zeit`) and enter a time of day
- [ ] **FIELD-05**: Author can define a Range field (`bereich`) bounded below and above, entered as a slider
- [ ] **FIELD-06**: Author can define a Code field (`code`) — plain text, no Markdown, fixed-width type

### Text snippets

- [ ] **SNIP-01**: Admin can give a text snippet any field kind, not only Markdown
- [ ] **SNIP-02**: A snippet's fields reuse the existing field definitions rather than a third field table
- [ ] **SNIP-03**: A snippet's field values render on the public site through the same pipeline and the same sanitisation as page fields

### Import

- [ ] **IMP-01**: Admin can upload a CSV file on the website screen and map each column to a field — title, body, or a custom field
- [ ] **IMP-02**: A CSV import creates pages through the same path as any other creation, so validation and sanitisation apply unchanged
- [ ] **IMP-03**: A CSV import reports per row what was created and what was skipped, naming the row and the reason

### Throughout

- [ ] **QUAL-01**: Every string this milestone adds ships in all five languages — `go run ./tools/i18n` reports `0 offen, 0 verwaist` before a phase is done
- [ ] **QUAL-02**: Every new field kind is exercised in the running application, not only in tests — the browser pass is what has caught the defects this project actually shipped

## Later Requirements (deferred)

Trimmed 2026-09-02: revisions, the activity log, TOTP and password reset were built
between April and September and moved to Validated in PROJECT.md.

- **V2-01**: Dark mode via `light-dark()` token toggle
- **V2-03**: Scheduled page publishing (publish_at date)
- **V2-04**: SEO meta fields (title, description, og:image) per page
- **V2-05**: sitemap.xml + robots.txt generation per site
- **V2-06**: Full-text search via SQLite FTS5
- **V2-10**: Drag-and-drop menu reordering (Sortable htmx extension)
- **V2-11**: RSS feed per site
- **V2-12**: Template preview before activation
- **V2-13**: Maintenance mode per site

## Out of Scope

| Feature | Reason |
|---------|--------|
| WYSIWYG / contenteditable | Requires heavy JS; contradicts htmx-only constraint |
| React / Vue / Svelte | SPA framework; contradicts server-rendered mandate |
| External databases | SQLite only; Pi-friendly single-file |
| npm / bundlers / Tailwind | No build tools; plain CSS only |
| Multi-tenant billing | Not a SaaS product |
| Static export | A second mode of operation beside the one that works — no forms, no search, no protected pages. `docs/offene-punkte.md` point 5. |
| Real-time collaboration | Out of scope for Pi-scale |
| Docker as primary deployment | Optional later; not v1 primary |

## Traceability

v1.0 requirements were mapped to phases 1–5, all complete; that mapping is archived with
the phase directories under `.planning/milestones/v1.0-phases/`.

v1.5 phases continue the numbering at 6.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FIELD-01 | Phase 6 | Pending |
| FIELD-02 | Phase 6 | Pending |
| FIELD-03 | Phase 6 | Pending |
| FIELD-04 | Phase 6 | Pending |
| FIELD-05 | Phase 6 | Pending |
| FIELD-06 | Phase 6 | Pending |
| SNIP-01 | Phase 7 | Pending |
| SNIP-02 | Phase 7 | Pending |
| SNIP-03 | Phase 7 | Pending |
| IMP-01 | Phase 8 | Pending |
| IMP-02 | Phase 8 | Pending |
| IMP-03 | Phase 8 | Pending |
| QUAL-01 | Phase 8 (gate on 6, 7, 8) | Pending |
| QUAL-02 | Phase 8 (gate on 6, 7, 8) | Pending |

QUAL-01 and QUAL-02 are recurring gates, not deliverables. They are counted once —
in Phase 8, the last phase, where they close milestone-wide — and are additionally
written verbatim as the final success criterion of Phases 6 and 7. Assigning them to
the last phase rather than the first is deliberate: a gate mapped to Phase 6 would
tell Phases 7 and 8 it was already satisfied. See the *Standing Gates* section of
`.planning/ROADMAP.md`.

**Coverage:**
- v1.5 requirements: 14 total
- Mapped to phases: 14
- Unmapped: 0
- Orphans: 0 · Duplicates: 0

**Phase distribution:**

| Phase | Name | Requirements |
|-------|------|--------------|
| 6 | Field Kinds | FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06 |
| 7 | Snippets Carry Fields | SNIP-01, SNIP-02, SNIP-03 |
| 8 | CSV Import | IMP-01, IMP-02, IMP-03, QUAL-01, QUAL-02 |

---
*Requirements defined: 2026-04-14 (v1.0), 2026-09-02 (v1.5)*
*Last updated: 2026-09-03 — v1.5 roadmap created, traceability filled in*
