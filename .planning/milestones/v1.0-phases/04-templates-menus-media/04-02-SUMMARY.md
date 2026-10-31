---
phase: 04-templates-menus-media
plan: 02
subsystem: admin-template, admin-menu
tags: [admin-ui, template-upload, menu-crud, htmx, handlers]
dependency_graph:
  requires: [internal/tmplmgr, internal/menu, internal/web, internal/template]
  provides: [admin/template.go, admin/menu.go, template_list.html, menu_edit.html]
  affects: [cmd/holzcloud/main.go, internal/admin/handler.go, internal/config/config.go, assets/admin.css]
tech_stack:
  added: []
  patterns: [multipart-upload, MaxBytesReader-size-cap, tree-indentation-via-depth, reorder-via-swap]
key_files:
  created:
    - internal/admin/template.go
    - internal/admin/menu.go
    - cmd/holzcloud/templates/admin/template_list.html
    - cmd/holzcloud/templates/admin/template_upload.html
    - cmd/holzcloud/templates/admin/menu_list.html
    - cmd/holzcloud/templates/admin/menu_edit.html
  modified:
    - internal/admin/handler.go
    - internal/config/config.go
    - internal/web/render.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/assets/admin.css
decisions:
  - "Template slug generated via custom slugify function in admin package"
  - "Menu items displayed as flat list with depth-based padding-left rather than nested HTML"
  - "Reorder finds siblings by matching parent_id then locates adjacent item by index"
metrics:
  duration: "295s"
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 5
---

# Phase 04 Plan 02: Template + Menu Admin UI Summary

Admin handlers and templates for template upload/activate/delete and hierarchical menu CRUD with up/down reorder, integrated into existing admin shell with sidebar links and OKLCH-styled components.

## What Was Built

### Template Admin (internal/admin/template.go)
- **HandleTemplateList**: Lists all templates with per-website activation status and activate/deactivate/delete controls
- **HandleTemplateUpload**: GET shows form, POST processes multipart zip upload with `http.MaxBytesReader` size enforcement, slug uniqueness check, `tmplmgr.ExtractTemplate` for zip-slip-safe extraction, DB record creation, disk cleanup on failure
- **HandleTemplateActivate/Deactivate**: Toggle template activation per website with `loader.InvalidateTemplateCache` for immediate effect
- **HandleTemplateDelete**: Checks `IsActiveAnywhere` before deletion, flashes error if still active

### Menu Admin (internal/admin/menu.go)
- **HandleMenuList/Create**: List and create menus per website with location_key validation (lowercase, digits, hyphens)
- **HandleMenuEdit**: Loads menu items as flat tree with depth, loads published pages for page-link dropdown
- **HandleMenuUpdate/Delete**: Update menu settings or delete menu
- **HandleMenuItemCreate**: Creates items with type validation (page/url/custom), auto-calculates sort_order among siblings
- **HandleMenuItemUpdate/Delete**: Edit or remove individual items
- **HandleMenuItemReorder**: Reads direction=up|down, finds adjacent sibling, calls SwapSortOrder

### Config Extension
- `MaxTemplateSize` field (default 10MB) from `HOLZCLOUD_MAX_TEMPLATE_SIZE` env var

### Templates
- `template_list.html`: Table with name, slug, per-website activate/deactivate buttons, delete
- `template_upload.html`: Form with name + file input, enctype=multipart/form-data
- `menu_list.html`: Create form + table of menus per website
- `menu_edit.html`: Menu settings form, item tree with depth indentation, up/down reorder buttons, add item form with type/page/url/parent selects

### CSS
- `.menu-tree`, `.menu-item-row`: Flex layout with depth-based padding
- `.reorder-btn`: Compact 28px square arrow buttons
- `.item-type-badge`: Color-coded badges for page/url/custom types

### Route Wiring
- Template routes: GET/POST /admin/templates/*, POST /admin/templates/{id}/activate|deactivate|delete
- Menu routes: GET/POST /admin/websites/{id}/menus/*, items CRUD and reorder

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 858234e | Template admin handlers, upload, activate/deactivate, delete |
| 2 | f560ea4 | Menu admin handlers, hierarchical items, reorder, CSS |

## Verification

- `go build ./internal/admin/... ./cmd/holzcloud/...` -- PASS
- HandleTemplateList, HandleTemplateUpload, HandleTemplateActivate, HandleTemplateDelete exist -- VERIFIED
- HandleMenuList, HandleMenuEdit, HandleMenuItemCreate, HandleMenuItemDelete, HandleMenuItemReorder exist -- VERIFIED
- http.MaxBytesReader used in upload -- VERIFIED
- IsActiveAnywhere check before delete -- VERIFIED
- SwapSortOrder called with direction param -- VERIFIED
- Templates link in sidebar (already in base.html) -- VERIFIED
- .menu-tree and .reorder-btn styles in admin.css -- VERIFIED

## Known Stubs

None. All handlers are fully wired to data layer.

## Self-Check: PASSED
