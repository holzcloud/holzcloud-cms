---
phase: 05-admin-polish-users-deployment
plan: 01
subsystem: admin-ui-polish
tags: [flash, dashboard, view-transitions, loading-indicators, no-js-audit]
dependency_graph:
  requires: []
  provides: [flash-warning-type, dashboard-stats, view-transitions, progress-bar, no-js-fallback]
  affects: [admin-ux, all-admin-handlers]
tech_stack:
  added: []
  patterns: [css-only-flash-dismiss, has-selector-progress-bar, view-transition-names, oob-flash-swap]
key_files:
  created: []
  modified:
    - internal/web/flash.go
    - internal/web/render.go
    - internal/admin/dashboard.go
    - internal/admin/page.go
    - internal/admin/menu.go
    - internal/admin/website.go
    - cmd/holzcloud/templates/admin/base.html
    - cmd/holzcloud/templates/admin/dashboard.html
    - cmd/holzcloud/templates/admin/media_list.html
    - cmd/holzcloud/assets/admin.css
decisions:
  - Flash auto-dismiss uses pure CSS animation (5s hold + fade), no JS required
  - Global progress bar uses body:has(.htmx-request) selector for zero-JS activation
  - View transitions limited to 3 named elements (sidebar, main, page-title) per D-10
  - Dashboard uses 3 separate queries (global stats, per-website stats, recent activity) for clarity
metrics:
  duration_seconds: 233
  completed: "2026-04-14T15:43:41Z"
---

# Phase 05 Plan 01: Admin UI Polish Summary

Flash warning type, CSS auto-dismiss animation, global progress bar via :has(.htmx-request), view transition names on sidebar/main/title, dashboard with real website/page/media stats and per-website quick-action cards, no-JS fallback audit fixing 4 handler issues.

## Task Results

### Task 1: Flash system upgrade, CSS polish
**Commit:** c749ff9

- Added `SetFlashWarning` and `Flash.Warning` field to flash system
- Added `flash-area` div wrapper in base template for OOB swap targeting
- Added `global-progress` fixed div before admin layout
- CSS: `flash-dismiss` keyframe animation (80% hold, 20% fade+collapse)
- CSS: `.flash--warning` with OKLCH yellow tokens (`--color-warning`, `--color-warning-subtle`)
- CSS: Global progress bar with `body:has(.htmx-request)` activation
- CSS: `htmx-indicator` display toggle styles
- CSS: `view-transition-name` on `.sidebar`, `.main-content`, `.content-header h1`
- CSS: `fade-in`/`fade-out` keyframes for `::view-transition-old/new(main)`

### Task 2: Dashboard with stats + no-JS audit
**Commit:** 1a1807e

- Rewrote `HandleDashboard` with `DashboardData` struct containing global stats, per-website stats, recent pages
- Three SQL queries: global counts (single row), per-website counts (join), recent 5 pages (join + order)
- Dashboard template: stat cards grid, website cards with quick links (pages/menus/media/settings), recent activity list
- Empty state: welcome card with link to create first website

**No-JS audit fixes:**
- `HandlePageStatusToggle`: Added HX-Request check with non-htmx redirect fallback (was returning partial unconditionally)
- `HandleMenuItemReorder`: Added flash message on successful reorder
- `HandleDomainAdd`/`HandleDomainRemove`: Moved flash message setting before htmx branch so session flash is available for both paths
- `media_list`: Registered in render.go layout pages list (was missing, would cause runtime 500) and added `media_list-content` block for htmx partial rendering

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] media_list template not registered as layout page**
- **Found during:** Task 2 (no-JS audit)
- **Issue:** `media_list` was not in the `ParseAdminTemplates` layout pages list, meaning `RenderAdmin` would return "template not found" at runtime for any media list request
- **Fix:** Added `media_list` to the layout pages slice in `render.go` and added `media_list-content` block to the template
- **Files modified:** `internal/web/render.go`, `cmd/holzcloud/templates/admin/media_list.html`
- **Commit:** 1a1807e

## Self-Check: PASSED
