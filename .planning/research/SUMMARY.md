# Research Synthesis

**Project:** Holzcloud CMS (Go + htmx + CSS + SQLite)
**Synthesized:** 2026-04-14

## Executive Summary

1. **Single Go binary** via `modernc.org/sqlite` (pure-Go, no CGO) + `embed.FS` for templates/assets/migrations. Cross-compiles to ARM64 with `CGO_ENABLED=0`.
2. **SQLite dual-pool pattern** is the #1 foundation requirement: write pool (`MaxOpenConns=1`), read pool (higher). WAL + `busy_timeout=5000` + `foreign_keys=ON` via DSN pragma on every connection.
3. **htmx + CSRF integration** requires `<body hx-headers='{"X-CSRF-Token":"..."}'>` — hidden form fields are NOT sent by htmx AJAX. Must be wired in Phase 2.
4. **Markdown rendering** always flows through goldmark → bluemonday → `template.HTML`. Never cast raw goldmark output to `template.HTML` without sanitization.
5. **Build order is foundation-first:** DB+migrations → auth+sessions → multi-site model → page CRUD → templates → menus → media → polish.

## Locked Technology Decisions

| Technology | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Backend, stdlib ServeMux routing |
| `modernc.org/sqlite` | v1.x | Pure-Go SQLite driver (Pi ARM64 friendly) |
| `alexedwards/scs/v2` + sqlite3store | latest | Server-side sessions in SQLite |
| `golang.org/x/crypto/argon2` | latest | Argon2id password hashing |
| `gorilla/csrf` | latest | CSRF middleware |
| `pressly/goose/v3` | latest | Embedded SQL migrations |
| `github.com/yuin/goldmark` | latest | Markdown → HTML |
| `github.com/microcosm-cc/bluemonday` | latest | HTML sanitization |
| `html/template` | stdlib | Server-side rendering |
| `log/slog` | stdlib | Structured logging |
| htmx | 2.0.x | Admin UI interactivity (self-hosted) |
| Plain CSS | — | OKLCH tokens, @layer, container queries |

## v1 Feature Scope

**Auth:** login/logout, Argon2id, SCS sessions, CSRF, admin+editor roles, session rotation
**Multi-site:** website CRUD, domain CRUD, Host→website middleware
**Pages:** slug/title/Markdown content, draft/published, per-site slugs, goldmark+bluemonday
**Public:** Host→site→slug→page rendering, per-site templates, 404, cache headers, ETag
**Templates:** zip upload (zip-slip safe), activate/list/delete, embed.FS fallback
**Menus:** hierarchical items, sort order, location keys, per-site
**Media:** multipart upload, MIME validation, per-site isolation, streaming serve
**Admin dashboard:** global + per-site overview, site switcher
**Users:** admin-only CRUD

## Top 10 Pitfalls

1. SQLite SQLITE_BUSY without dual-pool → Phase 1
2. XSS via template.HTML without bluemonday → Phase 4
3. CSRF missing from htmx requests → Phase 2
4. WAL pragmas not applied per-connection → Phase 1
5. Zip-slip in template upload → Phase 5
6. Draft pages leaking to public → Phase 3-4
7. Session fixation (no rotation on login) → Phase 2
8. Missing `Vary: HX-Request` header → Phase 2+
9. Double-submit on slow Pi → Phase 2+
10. Argon2id too slow on Pi without tuning → Phase 2

## UI Direction

- **Aesthetic:** schlicht/modern — Linear/Ghost/Vercel feel
- **CSS:** OKLCH tokens, `@layer` cascade, container queries, view transitions, `light-dark()` ready
- **Typography:** system font stack or locally-served variable font; modular scale via `clamp()`
- **Layout:** CSS grid + subgrid; 8px spacing base; generous whitespace
- **Motion:** view transitions for admin nav; `prefers-reduced-motion` honored

## Suggested Phase Structure

1. Foundation (DB, migrations, binary skeleton, embed.FS, graceful shutdown)
2. Auth + Admin Shell (Argon2id, sessions, CSRF, htmx wiring, admin layout)
3. Multi-Site Model + Domain Routing (website/domain CRUD, Host resolver, public 404)
4. Page Authoring + Public Rendering (CRUD, goldmark+bluemonday, template engine, slug routing)
5. Template Management (zip upload, activate, disk-first loader, cache)
6. Menu Builder (hierarchical CRUD, reorder, location keys, public rendering)
7. Media Uploads (multipart, MIME, per-site storage, serve)
8. Polish + Deployment (admin CSS tokens, view transitions, systemd, docs, user management)
