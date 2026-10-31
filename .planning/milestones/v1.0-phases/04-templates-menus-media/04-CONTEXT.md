# Phase 04: Templates + Menus + Media - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Mode:** Auto-generated (--auto mode)

<domain>
## Phase Boundary

Admins can upload, activate, and delete template archives safely; editors can build hierarchical menus per website; editors can upload and manage media files per website.

Requirements: TPL-01, TPL-02, TPL-03, TPL-04, MENU-01, MENU-02, MENU-03, MENU-04, MEDIA-01, MEDIA-02, MEDIA-03, MEDIA-04

</domain>

<decisions>
## Implementation Decisions

### Template Upload & Management (TPL-01, TPL-02, TPL-03, TPL-04)
- **D-01:** Template upload via multipart form. Server extracts `.zip` archive with zip-slip prevention: every extracted path must be cleaned and must start with the target directory prefix. Reject entire upload if any path escapes.
- **D-02:** Extension allow-list for template files: `.html`, `.css`, `.js`, `.svg`, `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.ico`, `.woff`, `.woff2`, `.ttf`. Reject files with other extensions.
- **D-03:** Size cap: 10MB per template archive (configurable via env var `HOLZCLOUD_MAX_TEMPLATE_SIZE`).
- **D-04:** Template directory convention: must contain `layout.html` and `page.html` at root level. Optional: `home.html`, `404.html`, `assets/` subdirectory.
- **D-05:** Templates table: id, website_id (FK nullable — shared templates), name, slug, is_active (per website), created_at. Activation is per website: `website_templates` join table with website_id, template_id, active flag.
- **D-06:** Template storage: extracted to `data/templates/{template_slug}/`. Template loader from Phase 3 (`internal/template/`) already supports disk-first lookup — templates stored on disk, metadata in DB.
- **D-07:** Admin UI at `/admin/templates` — list all templates, upload form, activate/deactivate per website, delete with confirmation.
- **D-08:** Deleting a template removes disk files and DB record. Cannot delete the currently active template for any website.
[auto] Selected: recommended approach — zip extraction with security validation.

### Hierarchical Menus (MENU-01, MENU-02, MENU-03, MENU-04)
- **D-09:** Menus table: id, website_id (FK), name, location_key (e.g., "main", "footer"), created_at. Unique constraint on (website_id, location_key).
- **D-10:** Menu items table: id, menu_id (FK), parent_id (FK nullable, self-referencing), title, url, sort_order (integer), created_at. Hierarchical via parent_id.
- **D-11:** Admin menu editor at `/admin/websites/{id}/menus` — list menus, create/edit menu, manage items. Items shown as indented tree. Add/edit/delete items inline.
- **D-12:** Reordering via up/down buttons per item (no drag-and-drop in v1 — that's V2-10). Server swaps sort_order values.
- **D-13:** Public rendering: template helper function `{{menu .Menus "main"}}` that outputs nested `<ul><li>` structure. Max depth: 3 levels.
- **D-14:** Menu items support: page link (select from published pages), external URL, or custom text (no link). Type field on menu_items.
[auto] Selected: recommended approach — parent_id tree with sort_order.

### Media Upload & Management (MEDIA-01, MEDIA-02, MEDIA-03, MEDIA-04)
- **D-15:** Media upload via multipart form. Files stored in `data/media/{website_id}/` with unique filename (UUID prefix to avoid collisions).
- **D-16:** MIME allow-list: `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `image/svg+xml`, `application/pdf`. Reject everything else. Validate by reading file header (magic bytes), not just Content-Type header.
- **D-17:** Per-file size limit: 5MB (configurable via `HOLZCLOUD_MAX_MEDIA_SIZE`).
- **D-18:** Media table: id, website_id (FK), filename, original_name, mime_type, size_bytes, created_at. Stores metadata; actual file on disk.
- **D-19:** Admin UI at `/admin/websites/{id}/media` — grid/list view of uploaded files, upload form, delete with confirmation. Image thumbnails shown inline.
- **D-20:** Media serving at `/media/{website_id}/{filename}` with `Content-Type` from DB, `Cache-Control: public, max-age=31536000, immutable`, and `Content-Disposition: inline`.
- **D-21:** Deleting media removes disk file and DB record.
[auto] Selected: recommended approach — UUID-prefixed filenames with magic byte validation.

### Claude's Discretion
- Thumbnail generation (serve originals or generate thumbnails) — implementer decides (recommend serving originals, no image processing in v1)
- Menu item icon support — implementer decides (recommend: no icons in v1)
- Template validation beyond file structure (e.g., parsing HTML for required blocks) — implementer decides
- Media list pagination threshold — implementer decides

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Configuration
- `.planning/REQUIREMENTS.md` — Full requirement definitions for TPL-01–04, MENU-01–04, MEDIA-01–04
- `.planning/ROADMAP.md` — Phase 4 success criteria
- `CLAUDE.md` — Stack constraints, security rules (zip-slip prevention), naming conventions

### Prior Phase Code
- `internal/template/loader.go` — Template loader from Phase 3 (disk-first/embed fallback — extend, don't replace)
- `internal/public/handler.go` — Public handlers (add menu data to template rendering)
- `internal/domain/store.go` — Website store pattern to follow
- `internal/page/store.go` — Page store pattern (CRUD + listing)
- `internal/admin/website.go` — Admin handler pattern for CRUD
- `internal/admin/page.go` — Admin handler pattern with htmx inline editing
- `internal/web/render.go` — Template rendering helpers
- `cmd/holzcloud/main.go` — Route wiring patterns
- `assets/admin.css` — OKLCH design system tokens

### Prior Decisions
- `.planning/phases/02-auth-admin-shell/02-CONTEXT.md` — Design tokens, admin layout
- `.planning/phases/03-multi-site-pages-public-rendering/03-CONTEXT.md` — Template resolution, caching patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/template/loader.go` — Already handles disk/embed template resolution; extend for menu helper functions
- `internal/domain/store.go` — Website CRUD store pattern; replicate for templates, menus, media
- `internal/page/store.go` — Page store with listing/pagination; replicate for media listing
- `internal/admin/website.go` + `page.go` — Handler patterns for CRUD with htmx
- `internal/web/render.go` — `RenderAdmin` helper for admin template rendering
- `assets/admin.css` — Existing component styles to extend for media grid, menu tree

### Established Patterns
- Goose migrations with STRICT tables, INTEGER for booleans
- Handler methods on `*Handler` struct with `ErrHandler` wrapper
- Flash messages via SCS session for POST-redirect-GET
- htmx partial swaps for list operations
- Domain-scoped routes: `/admin/websites/{id}/...`

### Integration Points
- `cmd/holzcloud/main.go` — Add template, menu, media admin routes + media serving route
- `internal/public/handler.go` — Inject menu data into PageData for public rendering
- `internal/template/loader.go` — Add menu rendering helper to FuncMap
- `templates/admin/base.html` — Add Templates, Menus, Media to sidebar nav

</code_context>

<specifics>
## Specific Ideas

- Template upload UX: simple file input, show extraction progress/result, list installed templates with activate button per website
- Menu editor: tree view with indentation, up/down arrows for reordering, add/edit item via inline form or modal-less htmx swap
- Media grid: thumbnail cards for images, file icon for non-images, click to copy URL for use in markdown content
- All admin UI maintains the same Linear/Ghost aesthetic from Phase 2

</specifics>

<deferred>
## Deferred Ideas

- Drag-and-drop menu reordering (V2-10)
- Template preview before activation (V2-12)
- Image thumbnail generation / resizing
- Media insertion button in page editor (could be Phase 5 polish)
- Template versioning / rollback

</deferred>

---

*Phase: 04-templates-menus-media*
*Context gathered: 2026-04-14*
