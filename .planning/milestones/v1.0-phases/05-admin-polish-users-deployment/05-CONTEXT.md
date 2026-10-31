# Phase 05: Admin Polish + Users + Deployment - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Mode:** Auto-generated (--auto mode)

<domain>
## Phase Boundary

The admin UI works fully without JavaScript and is enhanced by htmx; all interactions have loading indicators and flash messages; admins can manage users; the binary ships with production-ready systemd and Caddy configuration.

Requirements: UI-03, UI-04, UI-05, UI-06, UI-07, USR-01, USR-02, DEP-01, DEP-02, DEP-03, DEP-04

</domain>

<decisions>
## Implementation Decisions

### Progressive Enhancement / No-JS Fallback (UI-03)
- **D-01:** All admin forms already use standard `<form>` with `method="POST"` and `action` attributes. htmx attributes (`hx-post`, `hx-swap`, etc.) are additive. With JS disabled, forms submit normally and POST-redirect-GET works via flash messages.
- **D-02:** Audit all existing admin handlers: ensure every htmx-enhanced action has a working non-htmx fallback path. Handlers already check `HX-Request` header — ensure the non-htmx branch returns full page, not partial.
- **D-03:** Inline editing (page title/slug) degrades to "click → navigate to edit form" when JS disabled. The `<a>` link to edit page is the default; htmx intercepts and swaps inline.
[auto] Selected: recommended approach — audit existing handlers, ensure full-page fallback.

### Flash Messages (UI-04)
- **D-04:** Flash messages already implemented in `internal/web/flash.go` via SCS session. Ensure every POST handler (create, update, delete, publish, upload, activate) sets a flash message on success and error.
- **D-05:** Flash message UI: top-of-content-area banner, auto-dismiss after 5 seconds via CSS animation (no JS needed). Types: success (green), error (red), warning (yellow). Use OKLCH tokens.
- **D-06:** For htmx requests: flash message injected into partial response via `HX-Trigger` header or included in the swapped HTML.
[auto] Selected: recommended approach — CSS-animated auto-dismiss banners.

### Dashboard (UI-05)
- **D-07:** Admin dashboard at `/admin/` shows: website count, total pages, total media files, recent activity (last 5 page edits). Each website shown as a card with quick links to pages, menus, templates, media.
- **D-08:** Dashboard data loaded via single query joining websites with counts. No real-time updates — page load only.
[auto] Selected: recommended approach — overview cards with quick actions.

### View Transitions (UI-06)
- **D-09:** Add `@view-transition { navigation: auto; }` to admin CSS. Progressive enhancement — browsers that support it get smooth transitions, others get instant navigation.
- **D-10:** Add `view-transition-name` to key elements: sidebar (persist), main content area (slide), page title (morph). Keep it minimal — 3-4 named transitions max.
[auto] Selected: recommended approach — CSS-only view transitions, progressive enhancement.

### Loading Indicators (UI-07)
- **D-11:** Add `hx-indicator` to all htmx-enabled buttons/forms. Indicator is a small spinner or opacity change on the triggering element. Use `hx-disabled-elt="this"` on all submit buttons (already in CLAUDE.md).
- **D-12:** Global indicator: subtle top-of-page progress bar (like GitHub) for navigation requests. CSS-only animation triggered by `htmx-request` class.
[auto] Selected: recommended approach — per-element + global progress bar.

### User Management (USR-01, USR-02)
- **D-13:** Users table already exists (from Phase 2 auth). Admin UI at `/admin/users` — list, create, edit, delete users. Fields: name, email, password, role (admin/editor).
- **D-14:** Create user: admin sets initial password. Edit user: admin can change name, email, role. Password change: separate form.
- **D-15:** Password change rules: user changing own password must enter current password first. Admin changing another user's password does NOT need current password (admin override).
- **D-16:** Cannot delete own account. Cannot demote last admin.
[auto] Selected: recommended approach — standard user CRUD with safety guards.

### Deployment Configuration (DEP-01, DEP-02, DEP-03, DEP-04)
- **D-17:** Build: `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud` — already defined in CLAUDE.md.
- **D-18:** systemd unit file: `holzcloud.service` with `ProtectSystem=strict`, `NoNewPrivileges=true`, `ReadWritePaths=/opt/holzcloud/data`, `PrivateTmp=true`. Working directory: `/opt/holzcloud/`.
- **D-19:** Caddy reverse proxy config: `holzcloud.example.com { reverse_proxy localhost:8080 }` with automatic HTTPS. Document multi-domain setup.
- **D-20:** Backup strategy: document `sqlite3 data/holzcloud.sqlite ".backup backup.sqlite"` + rsync `data/media/` and `data/templates/`. Cron example included.
- **D-21:** All deployment files in `deploy/` directory: `holzcloud.service`, `Caddyfile.example`, `DEPLOY.md` (setup guide), `backup.sh`.
[auto] Selected: recommended approach — production-hardened systemd + Caddy + documented backup.

### Claude's Discretion
- Exact spinner/loading indicator design — implementer decides
- Dashboard layout (grid vs list) — implementer decides
- View transition timing/easing — implementer decides
- Whether to add a "change password" prompt on first login — implementer decides (recommend: no, keep simple)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Configuration
- `.planning/REQUIREMENTS.md` — Full definitions for UI-03–07, USR-01–02, DEP-01–04
- `.planning/ROADMAP.md` — Phase 5 success criteria
- `CLAUDE.md` — Stack constraints, build command, security rules, htmx integration patterns

### Prior Phase Code
- `internal/web/flash.go` — Flash message system (extend for all handlers)
- `internal/web/render.go` — Template rendering (check htmx vs full page branching)
- `internal/admin/handler.go` — Handler struct, ErrHandler pattern
- `internal/admin/page.go` — Inline editing pattern (htmx fallback reference)
- `internal/admin/website.go` — CRUD handler pattern
- `internal/auth/middleware.go` — RequireAuth/RequireAdmin middleware
- `internal/auth/hash.go` — Argon2id password hashing
- `cmd/holzcloud/main.go` — Route wiring
- `assets/admin.css` — OKLCH tokens, @layer cascade, existing component styles
- `templates/admin/base.html` — Admin layout (add view transitions, loading indicators)

### Prior Decisions
- `.planning/phases/02-auth-admin-shell/02-CONTEXT.md` — Design tokens, admin layout, CSRF integration
- `.planning/phases/03-multi-site-pages-public-rendering/03-CONTEXT.md` — Page CRUD patterns
- `.planning/phases/04-templates-menus-media/04-CONTEXT.md` — Template/menu/media patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/web/flash.go` — Flash system exists, just needs consistent usage across all handlers
- `internal/admin/*.go` — All handler patterns established; user CRUD follows same pattern
- `internal/auth/hash.go` — Argon2id hashing ready for password change feature
- `assets/admin.css` — Full OKLCH design system with @layer cascade; extend for dashboard, indicators
- `templates/admin/base.html` — Layout with sidebar; add view transition names, global indicator

### Established Patterns
- htmx partial vs full page via `HX-Request` header check
- `hx-disabled-elt="this"` on submit buttons (CLAUDE.md mandate)
- Flash messages via SCS session
- Handler methods on `*Handler` struct with `ErrHandler` wrapper
- CSRF via `hx-headers` on `<body>`

### Integration Points
- `cmd/holzcloud/main.go` — Add user management routes
- `templates/admin/base.html` — Add view transitions, loading indicator, Users nav link
- `assets/admin.css` — Add flash animation, indicator styles, dashboard styles, view transition names
- Every existing admin handler — Audit for flash message consistency and no-JS fallback

</code_context>

<specifics>
## Specific Ideas

- Dashboard should feel like a command center — quick overview, not overwhelming
- Flash messages: subtle, not intrusive; auto-dismiss keeps the UI clean
- View transitions: subtle slide/fade, not flashy; progressive enhancement only
- Loading indicator: GitHub-style thin progress bar at top is perfect
- User management: simple table list, standard form, no complexity
- Deploy docs: written for someone SSHing into a fresh Pi 5

</specifics>

<deferred>
## Deferred Ideas

- Dark mode (V2-01)
- Activity log (V2-07)
- Email-based password reset (V2-08)
- 2FA/TOTP (V2-09)
- Docker deployment (out of scope per REQUIREMENTS.md)
- Maintenance mode per site (V2-13)

</deferred>

---

*Phase: 05-admin-polish-users-deployment*
*Context gathered: 2026-04-14*
