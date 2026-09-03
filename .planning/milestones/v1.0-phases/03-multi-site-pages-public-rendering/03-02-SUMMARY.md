---
phase: 03-multi-site-pages-public-rendering
plan: 02
subsystem: admin-website-page-crud
tags: [admin, websites, pages, markdown, htmx, inline-edit, crud]
dependency_graph:
  requires: [internal/domain, internal/page, internal/web, internal/admin/handler.go]
  provides: [internal/admin/website.go, internal/admin/page.go, admin templates, admin routes]
  affects: [cmd/holzcloud/main.go, internal/web/render.go, internal/web/layoutdata.go]
tech_stack:
  added: []
  patterns: [htmx click-to-edit, htmx pagination partial swap, status toggle row swap, domain list partial swap]
key_files:
  created:
    - internal/admin/website.go
    - internal/admin/page.go
    - cmd/holzcloud/templates/admin/website_list.html
    - cmd/holzcloud/templates/admin/website_form.html
    - cmd/holzcloud/templates/admin/page_list.html
    - cmd/holzcloud/templates/admin/page_form.html
    - cmd/holzcloud/templates/admin/page_inline_edit.html
  modified:
    - internal/admin/handler.go
    - internal/web/render.go
    - internal/web/layoutdata.go
    - cmd/holzcloud/assets/admin.css
    - cmd/holzcloud/main.go
decisions:
  - RenderAdmin changed from LayoutData to any type to support extended template data structs
  - Page row partials rendered programmatically (strings.Builder) for status toggle and inline edit responses
  - CSRFTokenFromRequest helper added for programmatic HTML rendering outside templates
metrics:
  duration: 248s
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 7
  files_modified: 5
---

# Phase 03 Plan 02: Admin Website + Page CRUD Summary

Full admin CRUD for websites with inline domain management, and pages with Markdown pipeline, htmx pagination, inline title editing, and status toggle.

## What Was Built

### Website Management (internal/admin/website.go)
- **HandleWebsiteList**: Lists all websites with domain counts, active/inactive badge.
- **HandleWebsiteCreate**: GET renders form, POST validates name required, creates website, redirects to edit for domain setup.
- **HandleWebsiteEdit**: GET loads website + domains, renders form. POST updates name/description/active flag.
- **HandleWebsiteDelete**: Deletes website, invalidates resolver cache.
- **HandleDomainAdd**: Adds domain to website, invalidates resolver cache, returns domain list partial for htmx swap.
- **HandleDomainRemove**: Removes domain, invalidates resolver cache, returns domain list partial.

### Page Management (internal/admin/page.go)
- **HandlePageList**: Paginated list (20/page) with htmx partial swap for prev/next navigation.
- **HandlePageCreate**: Markdown rendered via goldmark+bluemonday on save. Slug auto-generated from title if empty. Sets published_at on publish.
- **HandlePageEdit**: Full page editor with title, slug, Markdown textarea, status dropdown. Preserves existing published_at.
- **HandlePageDelete**: Verifies page belongs to website before deletion.
- **HandlePageStatusToggle**: Toggles draft/published, sets published_at on first publish, returns updated row partial.
- **HandlePageInlineEditTitle**: Returns input field for click-to-edit via htmx.
- **HandlePageInlineEditSave**: Saves inline title/slug edit, re-slugifies if needed, returns display row.

### Templates
- **website_list.html**: Table with name, domain count, active badge, edit/pages/delete actions.
- **website_form.html**: Name/description/active form + inline domain management (add/remove via htmx).
- **page_list.html**: Table with title (click-to-edit), status badge, published date, edit/publish/delete actions. Pagination controls.
- **page_form.html**: Title, slug (auto-hint), Markdown textarea (tall, monospace), status dropdown.
- **page_inline_edit.html**: Input with hx-put on blur/Enter, Escape cancels.

### CSS Components
- Table styles (.table, .table-actions) with hover rows
- Badges (.badge--published green, .badge--draft muted, .badge--primary accent)
- Domain management (.domain-item, .domain-add-row)
- Inline edit (.page-title-cell hover, .form-input--inline)
- Pagination (.pagination centered flex)
- Form extras (.form-textarea--tall monospace, .form-select, .form-hint, .form-check)

### Infrastructure Changes
- **handler.go**: Added domain.Store, domain.Resolver, page.Store fields; updated NewHandler signature.
- **render.go**: Changed RenderAdmin data param from LayoutData to `any` for extended data structs. Added page template names to layout pages list.
- **layoutdata.go**: Added CSRFTokenFromRequest helper.
- **main.go**: Creates domain/page stores and resolver. Wires 18 new admin routes within CSRF+auth middleware.

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|------------|
| T-03-06 Injection | All SQL via parameterized queries in store layer; handlers never build SQL |
| T-03-07 XSS via markdown | RenderMarkdown called on every page create/edit; goldmark -> bluemonday |
| T-03-08 Auth bypass | All routes inside requireAuth middleware; CSRF on all POST/PUT |
| T-03-09 Cross-website access | Page ownership (website_id) verified before every mutation |
| T-03-10 Data leakage | Admin page queries scoped by website_id |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] RenderAdmin signature change**
- **Found during:** Task 1
- **Issue:** RenderAdmin accepted `LayoutData` but templates need extended data (WebsiteListData, PageFormData, etc.)
- **Fix:** Changed RenderAdmin data parameter from `LayoutData` to `any`
- **Files modified:** internal/web/render.go
- **Commit:** 99440ab

**2. [Rule 2 - Missing functionality] CSRFTokenFromRequest helper**
- **Found during:** Task 2
- **Issue:** Programmatic HTML rendering (page row partials) needs CSRF token outside template context
- **Fix:** Added CSRFTokenFromRequest(r) helper wrapping csrf.Token(r)
- **Files modified:** internal/web/layoutdata.go
- **Commit:** af89884

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 99440ab | Website CRUD handlers, templates, domain management |
| 2 | af89884 | Page CRUD handlers, templates, inline edit, status toggle, route wiring |

## Verification

- `go build ./...` succeeds (full project compiles)
- Website CRUD: create, edit (with domains), delete all have handlers and templates
- Page CRUD: create with markdown, edit, delete, status toggle, inline edit all have handlers
- All admin routes registered in main.go within CSRF+auth middleware
- Resolver.InvalidateCache called after domain add/remove and website delete
- RenderMarkdown called on page create and edit
- Slugify called when slug is empty
- Page ownership verified on edit, delete, status toggle, inline edit

## Self-Check: PASSED

All 7 created files exist. Both commits verified in git log.
