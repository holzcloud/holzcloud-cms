---
phase: 03-multi-site-pages-public-rendering
plan: 03
subsystem: public-rendering-engine
tags: [public, templates, caching, etag, 304, default-template]
dependency_graph:
  requires: [internal/domain, internal/page]
  provides: [internal/template, internal/public, default public template]
  affects: [cmd/holzcloud/main.go]
tech_stack:
  added: []
  patterns: [sync.Map template cache, ETag/Last-Modified/304 caching, disk-first embed-fallback template resolution]
key_files:
  created:
    - internal/template/loader.go
    - internal/public/handler.go
    - cmd/holzcloud/templates/public/default/home.html
    - cmd/holzcloud/templates/public/default/404.html
    - cmd/holzcloud/templates/public/default/style.css
  modified:
    - cmd/holzcloud/templates/public/default/layout.html
    - cmd/holzcloud/templates/public/default/page.html
    - cmd/holzcloud/main.go
decisions:
  - Template loader caches parsed templates per websiteID in sync.Map with InvalidateTemplateCache for future template upload
  - Public ContentHTML cast via template.HTML is safe because bluemonday sanitizes on save; safeHTML FuncMap helper available in templates
  - Default public template uses system font stack, 65ch max-width, neutral palette (no OKLCH for public simplicity)
  - Home route uses GET /{$} exact match to avoid catching all paths
metrics:
  duration: 175s
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 5
  files_modified: 3
---

# Phase 03 Plan 03: Public Rendering Engine Summary

Template loader with disk/embed fallback, public handlers with ETag+Last-Modified+304 caching, clean default template, and domain-resolved route wiring.

## What Was Built

### internal/template Package
- **loader.go**: `Loader` struct with `sync.Map` template cache keyed by websiteID. `NewLoader(dataDir, defaultFS)` constructor. Template resolution: checks disk path `{dataDir}/templates/{websiteID}/` first, falls back to embedded default FS. Parses layout.html + page.html + home.html + 404.html with `FuncMap` containing `safeHTML` (documented as safe due to bluemonday pre-sanitization) and `formatDate`. `RenderPage` renders to buffer for ETag computation. `Render404` renders 404 template with site name. `InvalidateTemplateCache` for future Phase 4 template upload.
- **PageData**: Site (Name, Description), Page (Title, ContentHTML, Slug, PublishedAt), Menus (empty slice for Phase 4), Meta (CanonicalURL).

### internal/public Package
- **handler.go**: `Handler` with pageStore + loader + dataDir + defaultFS. `ErrHandler` wrapper matching admin pattern.
- **HandleHome**: Gets website from context, calls `GetHomePage` (published-only), renders with home.html template and caching headers.
- **HandlePage**: Gets website from context, extracts slug via `r.PathValue("slug")`, calls `GetPublishedPage` (enforces published-only per T-03-11), renders with page.html template and caching headers. Unknown slugs render styled 404.
- **HandleTemplateAsset**: Serves static files from template directory. Disk-first with embed fallback. `Cache-Control: public, max-age=31536000, immutable` per D-23.
- **serveCached**: Sets `Cache-Control: public, max-age=300`, computes ETag (MD5 hex of content), sets Last-Modified from page.UpdatedAt. Handles `If-None-Match` -> 304 and `If-Modified-Since` -> 304. Sets `Vary: HX-Request`.

### Default Public Template
- **layout.html**: HTML5 with viewport meta, canonical URL, /t/style.css link. Header with site name/description, main content block, footer with copyright.
- **page.html**: Article with h1 title, formatted published date, content HTML.
- **home.html**: Homepage with title, content, or welcome fallback message.
- **404.html**: Centered 404 heading with "page not found" message and return home link.
- **style.css**: Minimal clean typography. System font stack, 65ch max-width, 1.6/1.7 line-height. Styles for headers, content elements (h2/h3, blockquote, code, pre, table, lists, images, hr), 404 page, and footer. Neutral color palette.

### Route Wiring (main.go)
- Created `tmpl.Loader` with `cfg.DataDir` and embedded default FS sub'd from `templates/public/default`.
- Created `public.Handler` with page store, template loader, data dir, and default FS.
- Public mux: `GET /t/{path...}` for template assets, `GET /{slug}` for pages, `GET /{$}` for home.
- Wrapped with `domainResolver.Middleware` and registered as catch-all `mux.Handle("/", ...)` AFTER all admin routes.

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|------------|
| T-03-11 Draft leakage | Only `GetPublishedPage` used in public handlers; always enforces `status='published'` |
| T-03-12 XSS via safeHTML | content_html pre-sanitized by bluemonday on save; comment documents safety invariant |
| T-03-13 ETag spoofing | ETag computed server-side from actual content MD5; client cannot forge |
| T-03-14 404 info disclosure | 404 page shows site name only, no page titles/slugs/draft content |
| T-03-16 Host tampering | Domain resolver strips port, does exact DB match; public routes only fire for known domains |

## Deviations from Plan

None -- plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 47d5b4a | Template loader, public handlers, default template, caching, route wiring |
| 2 | (auto-approved) | Human verify checkpoint -- auto-approved in auto mode |

## Verification

- `go build ./...` succeeds
- `internal/template/loader.go` contains `NewLoader`, `RenderPage`, `Render404`, `safeHTML` FuncMap with safety comment
- `internal/public/handler.go` uses `GetPublishedPage` exclusively, contains `Cache-Control`, `ETag`, `Last-Modified`, `304`, `max-age=31536000`
- `cmd/holzcloud/templates/public/default/404.html` exists with "not found" content
- `cmd/holzcloud/main.go` contains `domainResolver.Middleware` wrapping public routes

## Self-Check: PASSED

All 5 created files and 3 modified files verified. Commit 47d5b4a confirmed in git log.
