---
phase: 04-templates-menus-media
plan: 01
subsystem: tmplmgr, menu, media
tags: [migration, models, stores, upload, security]
dependency_graph:
  requires: [internal/db, internal/page]
  provides: [internal/tmplmgr, internal/menu, internal/media]
  affects: [internal/template/loader.go]
tech_stack:
  added: []
  patterns: [zip-slip-prevention, magic-byte-mime, hierarchical-tree-build, atomic-extraction]
key_files:
  created:
    - internal/db/migrations/00005_templates_menus_media.sql
    - internal/tmplmgr/model.go
    - internal/tmplmgr/store.go
    - internal/tmplmgr/upload.go
    - internal/menu/model.go
    - internal/menu/store.go
    - internal/menu/render.go
    - internal/media/model.go
    - internal/media/store.go
    - internal/media/upload.go
  modified: []
decisions:
  - "Template store uses ON CONFLICT DO UPDATE for activation upsert"
  - "Menu tree built in-memory from flat query (no recursive CTE needed for max 3 levels)"
  - "SVG MIME fallback checks text/* prefix broadly, not just text/xml"
metrics:
  duration: "160s"
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 10
  files_modified: 0
---

# Phase 04 Plan 01: Schema + Data Layer Summary

Migration 00005 with 5 STRICT tables (templates, website_templates, menus, menu_items, media) plus 3 new packages with full CRUD stores, zip-slip-safe template extraction, magic-byte MIME validation, and hierarchical menu rendering.

## What Was Built

### Migration (00005_templates_menus_media.sql)
- 5 STRICT tables with proper FK constraints
- `website_templates.template_id` uses ON DELETE RESTRICT (cannot delete active templates)
- Partial unique index `idx_website_active_template` ensures one active template per website
- `menu_items.parent_id` self-references with ON DELETE CASCADE
- `media` indexed on `website_id`

### Template Manager (internal/tmplmgr)
- **store.go**: Full CRUD, per-website activation via transaction (deactivate all then upsert), `ActiveTemplateSlug` resolver for loader integration, `IsActiveAnywhere` guard for safe deletion
- **upload.go**: Zip extraction with zip-slip prevention (filepath.Clean + strings.HasPrefix), 13-extension allow-list, temp-dir extraction with atomic os.Rename, structure validation (layout.html + page.html required)

### Menu System (internal/menu)
- **store.go**: Full CRUD for menus and items, `SwapSortOrder` in transaction, `GetMenuTree` with flat query + in-memory tree build, joins pages table for page_slug on published pages only
- **render.go**: `RenderMenu` outputs nested `<ul><li>` HTML, max 3 levels, handles page/url/custom item types, all output HTML-escaped via `template.HTMLEscapeString`

### Media System (internal/media)
- **store.go**: CRUD with disk file cleanup on delete
- **upload.go**: `ValidateMIME` uses `http.DetectContentType` on first 512 bytes (not Content-Type header), SVG extension fallback for text/* detection, `GenerateFilename` with 16-byte crypto/rand hex prefix, `StoreFile` with io.LimitReader size enforcement

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 14eec9b | Migration + models for templates, menus, media |
| 2 | a348f9a | Stores + upload logic for tmplmgr, menu, media |

## Verification

- `go build ./internal/tmplmgr/... ./internal/menu/... ./internal/media/...` -- PASS
- `go vet ./internal/tmplmgr/... ./internal/menu/... ./internal/media/...` -- PASS
- Migration contains all 5 tables with STRICT keyword -- VERIFIED
- Zip-slip prevention (strings.HasPrefix) in upload.go -- VERIFIED
- http.DetectContentType used for MIME validation -- VERIFIED
- crypto/rand used for filename generation -- VERIFIED
- Menu render limits to 3 levels -- VERIFIED
- SwapSortOrder uses transaction -- VERIFIED

## Known Stubs

None. All packages are fully implemented data-layer code.

## Self-Check: PASSED
