---
phase: 03-multi-site-pages-public-rendering
plan: 01
subsystem: domain-resolution-page-data
tags: [multi-site, domain-resolver, pages, markdown, migrations]
dependency_graph:
  requires: [internal/db/db.go]
  provides: [internal/domain, internal/page, migrations/00003, migrations/00004]
  affects: [cmd/holzcloud/main.go]
tech_stack:
  added: [goldmark v1.8.2, bluemonday v1.0.27]
  patterns: [sync.Map cache, context-based website resolution, goldmark+bluemonday pipeline]
key_files:
  created:
    - internal/db/migrations/00003_websites.sql
    - internal/db/migrations/00004_pages.sql
    - internal/domain/models.go
    - internal/domain/store.go
    - internal/domain/context.go
    - internal/domain/resolver.go
    - internal/page/models.go
    - internal/page/store.go
    - internal/page/slug.go
    - internal/page/markdown.go
  modified: [go.mod, go.sum]
decisions:
  - sync.Map cache with Range+Delete invalidation (not replacement) for domain resolver
  - Homepage resolved by slug "home" with fallback to first published page by created_at
  - Slug uniqueness collision handled by appending -2, -3 suffix in CreatePage
metrics:
  duration: 126s
  completed: "2026-04-14"
  tasks_completed: 2
  tasks_total: 2
  files_created: 10
  files_modified: 2
---

# Phase 03 Plan 01: Domain Resolution + Page Data Layer Summary

STRICT SQLite tables for websites/domains/pages, sync.Map-cached host resolver middleware, and goldmark+bluemonday markdown pipeline.

## What Was Built

### Migrations
- `00003_websites.sql`: STRICT tables for `websites` (name, description, active flag) and `website_domains` (unique domain per website, COLLATE NOCASE, CASCADE delete). Indexes on website_id and domain.
- `00004_pages.sql`: STRICT `pages` table with composite UNIQUE(website_id, slug), draft/published status CHECK constraint, indexes on (website_id, status) and (website_id, slug).

### internal/domain Package
- **models.go**: Website and Domain structs with typed fields.
- **store.go**: Full CRUD for websites and domains. `LookupDomain` does a JOIN query for the resolver. All reads use Read pool, writes use Write pool. Parameterized queries throughout.
- **context.go**: Unexported contextKey struct with `WebsiteFromContext`/`WebsiteToContext` helpers.
- **resolver.go**: `Resolver` with `sync.Map` cache. `Middleware` strips port from Host header via `net.SplitHostPort`, checks cache, falls back to DB lookup, returns 404 for unknown/inactive hosts with no site data leaked. `InvalidateCache` clears all entries.

### internal/page Package
- **models.go**: Page struct with all fields including nullable PublishedAt.
- **store.go**: Full CRUD with `ListPages` (paginated, optional status filter, 20/page default), `GetPublishedPage` (always enforces `status = 'published'`), `GetHomePage` (slug "home" then first published), `CreatePage` (auto-retries slug collisions), inline edit helpers (`UpdatePageTitle`, `UpdatePageStatus`).
- **slug.go**: `Slugify` function -- lowercase, non-alphanumeric to hyphens, collapse, trim, "untitled" fallback.
- **markdown.go**: Package-level goldmark instance (Table, Strikethrough, Linkify extensions) + bluemonday UGCPolicy. `RenderMarkdown` always returns sanitized HTML.

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|------------|
| T-03-01 Host spoofing | Port stripped via net.SplitHostPort, COLLATE NOCASE on domain column, exact match only |
| T-03-02 Info disclosure | Unknown hosts get generic 404, no website names or domain lists exposed |
| T-03-03 XSS via markdown | bluemonday UGCPolicy() applied after every goldmark render |
| T-03-04 SQL injection | All queries use parameterized $N placeholders, no string concatenation |
| T-03-05 Cache DoS | Accepted -- domain count is tiny on Pi; InvalidateCache clears all entries |

## Deviations from Plan

None -- plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7a83f94 | Migrations, models, stores for websites/domains/pages |
| 2 | e2494d1 | Domain resolver middleware and markdown pipeline |

## Verification

- `go build ./...` succeeds (full project including new packages)
- Migrations contain STRICT keyword and correct constraints
- LookupDomain uses parameterized JOIN query
- GetPublishedPage enforces `AND status = 'published'`
- RenderMarkdown output is sanitized via bluemonday
- Resolver returns http.NotFound for unknown hosts

## Self-Check: PASSED

All 10 created files exist. Both commits verified in git log.
