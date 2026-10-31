---
phase: 04-templates-menus-media
plan: 03
subsystem: media-admin, template-loader, public-menus, route-wiring
tags: [media, upload, serve, template-resolver, menu-rendering, routes]
dependency_graph:
  requires: [internal/tmplmgr, internal/menu, internal/media, internal/admin, internal/public, internal/template]
  provides: [admin/media.go, media_list.html, updated-loader, public-menus, all-phase4-routes]
  affects: [cmd/holzcloud/main.go, internal/config/config.go]
tech_stack:
  added: []
  patterns: [MaxBytesReader-upload, magic-byte-mime, immutable-cache, template-resolver-interface, menu-funcmap-helper]
key_files:
  created:
    - internal/admin/media.go
    - cmd/holzcloud/templates/admin/media_list.html
  modified:
    - internal/admin/handler.go
    - internal/config/config.go
    - internal/template/loader.go
    - internal/public/handler.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/assets/admin.css
decisions:
  - "Media serve endpoint placed on admin Handler (reuses mediaStore + config) but registered as public route without auth"
  - "TemplateResolver interface defined in template package to avoid import cycle"
  - "Menu loading in public handler fetches all location keys per request (acceptable for small menu count)"
metrics:
  duration: "235s"
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 6
---

# Phase 04 Plan 03: Media Admin + Loader Integration + Route Wiring Summary

Media admin handlers with MIME-validated upload and immutable-cache serving, template loader upgraded to DB-backed slug resolution via TemplateResolver interface, public pages render menus via {{menu .Menus "main"}} FuncMap helper, all Phase 4 routes wired.

## What Was Built

### Media Admin (internal/admin/media.go)
- **HandleMediaList**: Lists media per website with grid template
- **HandleMediaUpload**: POST with `http.MaxBytesReader` (5MB), `media.ValidateMIME` magic bytes (not Content-Type header), UUID-prefixed filename, disk write with cleanup on DB failure
- **HandleMediaDelete**: Removes DB record + disk file via store.Delete
- **HandleMediaServe**: Public endpoint, filename path-separator validation (T-04-11), Content-Type from DB (T-04-13), `Cache-Control: public, max-age=31536000, immutable`

### Media List Template (media_list.html)
- Upload form with file input at top
- Grid layout via `.media-grid` CSS class
- Image thumbnails with aspect-ratio: 1 / object-fit: cover
- File icon SVG for non-image types (PDF)
- Copy URL button (clipboard API) for Markdown insertion
- Delete with confirmation

### Config Extension
- `MaxMediaSize` field (default 5MB) from `HOLZCLOUD_MAX_MEDIA_SIZE` env var

### Template Loader (internal/template/loader.go)
- **TemplateResolver interface**: `ActiveTemplateSlug(ctx, websiteID) (string, error)` -- tmplmgr.Store implements this
- **loadTemplates** now calls resolver to get slug, resolves disk path as `data/templates/{slug}/` instead of `data/templates/{websiteID}/`
- **RenderPage/Render404** accept context parameter for DB resolver calls
- **PageData.Menus** changed from `[]MenuItem` to `map[string][]menu.MenuNode`
- **MenuItem stub removed** -- replaced by menu.MenuNode
- **"menu" FuncMap helper** calls `menu.RenderMenu(menus, locationKey)` for `{{menu .Menus "main"}}`

### Public Handler (internal/public/handler.go)
- Added `menuStore *menu.Store` field
- **loadMenus** helper: fetches all menus for website, builds `map[string][]MenuNode`
- HandleHome and HandlePage inject menus into PageData before rendering
- Updated RenderPage/Render404 calls to pass request context

### Route Wiring (cmd/holzcloud/main.go)
- Media admin routes: GET list, POST upload, POST delete (auth-protected)
- Media serve: GET /media/{websiteID}/{filename} (public, no auth)
- Template routes already wired in 04-02, confirmed present
- Menu routes already wired in 04-02, confirmed present
- mediaStore created and passed to admin.NewHandler
- menuStore passed to public.NewHandler
- tmplmgr.Store passed as TemplateResolver to tmpl.NewLoader

### CSS
- `.media-grid`: CSS grid with auto-fill responsive columns
- `.media-card`: Card with thumbnail, metadata, actions sections
- `.media-card__icon`: Centered SVG icon for non-image files
- `.copy-url-btn`: Compact copy button
- `.form-row`: Flex layout for inline upload form

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 6fbdd46 | Media admin handlers, serve endpoint, template, config |
| 2 | 8d69b32 | Template loader DB integration, public menu rendering, route wiring |

## Verification

- `go build ./cmd/holzcloud/...` -- PASS
- `go vet ./...` -- PASS
- HandleMediaList, HandleMediaUpload, HandleMediaDelete, HandleMediaServe exist -- VERIFIED
- HandleMediaUpload uses http.MaxBytesReader -- VERIFIED
- HandleMediaUpload calls media.ValidateMIME (not Content-Type header) -- VERIFIED
- HandleMediaServe sets Cache-Control immutable header -- VERIFIED
- HandleMediaServe validates filename has no path separators -- VERIFIED
- media_list.html exists with media-grid class -- VERIFIED
- config.go has MaxMediaSize field -- VERIFIED
- TemplateResolver interface with ActiveTemplateSlug -- VERIFIED
- loadTemplates uses resolver for slug-based path -- VERIFIED
- funcMap includes "menu" helper -- VERIFIED
- PageData.Menus is map[string][]menu.MenuNode -- VERIFIED
- Public handler loads menus and sets on PageData -- VERIFIED
- All template, menu, media admin routes registered -- VERIFIED
- GET /media/{websiteID}/{filename} registered as public route -- VERIFIED
- tmplmgr.Store passed as TemplateResolver to NewLoader -- VERIFIED

## Known Stubs

None. All handlers are fully wired to data layer.

## Threat Flags

None. All security surfaces covered by plan's threat model (T-04-11 through T-04-15).
