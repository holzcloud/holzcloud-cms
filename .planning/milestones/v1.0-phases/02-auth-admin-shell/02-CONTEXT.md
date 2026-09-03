# Phase 02: Auth + Admin Shell - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Mode:** Auto-generated (--auto mode)

<domain>
## Phase Boundary

Admins can log in securely, the session is rotated on login, all state-changing requests (including htmx AJAX) carry a validated CSRF token, and the admin UI has a complete layout shell with design tokens.

Requirements: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, UI-01, UI-02

</domain>

<decisions>
## Implementation Decisions

### Login Flow
**Decision:** Single-page login form at `/admin/login`. Email + password fields. On success → redirect to `/admin` dashboard. On failure → re-render form with flash error "Invalid email or password" (no indication which field is wrong). No rate limiting in v1 (accept risk for Pi-scale deployment).
**Auto-selected:** Recommended approach

### First-Run Setup (AUTH-06)
**Decision:** If zero users in DB, any request to `/admin/*` redirects to `/admin/setup`. Setup form: email + password + password confirmation. Creates admin-role user. After creation → auto-login → redirect to `/admin`. Setup route returns 404 once any user exists (prevents second setup).
**Auto-selected:** Recommended approach

### Session Configuration (AUTH-02, AUTH-04)
**Decision:** alexedwards/scs with SQLite store. Session lifetime: 24 hours. Idle timeout: 4 hours. Cookie name: `holzcloud_session`. HttpOnly=true, SameSite=Lax always. Secure=true when `HOLZCLOUD_SECURE=true` env var is set (operator controls this for local dev vs production). Session ID rotated via `scs.RenewToken()` on successful login.
**Auto-selected:** Recommended approach

### CSRF + htmx Integration (AUTH-03)
**Decision:** gorilla/csrf middleware on all `/admin/*` routes. Token injected into `<body hx-headers='{"X-CSRF-Token":"{{.CSRFToken}}"}'>` so all htmx AJAX requests carry it automatically. Non-htmx forms use a hidden `<input>` field as fallback. CSRF failure returns 403 with flash message.
**Auto-selected:** Recommended approach

### Role-Based Access (AUTH-05)
**Decision:** Two middleware layers: `RequireAuth` (redirects to login if no session) and `RequireAdmin` (returns 403 if role != admin). Editor can access content routes (pages, media). Admin can access everything including user management and site settings. Middleware checks role from session data, not DB query per request.
**Auto-selected:** Recommended approach

### Argon2id Parameters (AUTH-01)
**Decision:** Tune for Pi 5 target: memory=64MB, iterations=1, parallelism=2. Accept HOLZCLOUD_ARGON2_MEMORY, HOLZCLOUD_ARGON2_ITERATIONS, HOLZCLOUD_ARGON2_PARALLELISM env vars for tuning. Target 200-500ms per hash on Pi 5 (known risk from STATE.md).
**Auto-selected:** Recommended approach

### Admin Layout (UI-01)
**Decision:** Fixed sidebar on desktop (240px width), collapsible on mobile via CSS-only toggle (no JS required — use checkbox hack or :has() with details/summary). Sidebar contains: logo/app name, nav links (Dashboard, Websites, Pages, Templates, Menus, Media, Users), current user info + logout at bottom. Main content area with sticky header showing page title + primary action button. Full-page fallback works without htmx (links are regular `<a>` tags, forms are regular `<form>` posts).
**Auto-selected:** Recommended approach

### Design Tokens (UI-02)
**Decision:** OKLCH color system with CSS custom properties. Neutral palette (slate/gray tones) with single accent color (blue). @layer cascade: reset → tokens → base → layout → components → utilities. 8px spacing scale (--space-1: 8px through --space-8: 64px). System font stack. Container queries for component-level responsiveness. View transition API opt-in via @view-transition rule. Light theme only in v1 (dark mode is V2-01).

Color tokens:
- --color-bg: oklch(0.99 0 0) (near-white)
- --color-surface: oklch(1 0 0) (white)
- --color-border: oklch(0.9 0 0) (light gray)
- --color-text: oklch(0.2 0 0) (near-black)
- --color-text-muted: oklch(0.55 0 0)
- --color-accent: oklch(0.6 0.15 250) (blue)
- --color-accent-hover: oklch(0.55 0.15 250)
- --color-danger: oklch(0.6 0.2 25) (red)
- --color-success: oklch(0.65 0.15 145) (green)

**Auto-selected:** Recommended approach — clean/minimal aesthetic per user preference (Linear/Vercel/Ghost feel)

### Claude's Discretion
- Password input masking/reveal toggle — implementer decides
- Exact sidebar animation/transition for mobile — implementer decides
- Flash message positioning (top-center vs inline) — implementer decides
- Login page layout/styling — implementer decides (keep minimal and centered)

</decisions>

<code_context>
## Existing Code Insights

From Phase 1:
- `internal/config/config.go` — Config struct with env-based loading. Extend with HOLZCLOUD_SECURE and Argon2 params.
- `internal/db/db.go` — DB struct with Write/Read pools. Sessions and auth queries use these.
- `cmd/holzcloud/main.go` — Entry point. Wire auth middleware, session manager, CSRF here.
- Go 1.22+ ServeMux with method+path patterns already in use.

New packages needed:
- `internal/auth/` — Argon2id hashing, session setup, CSRF middleware, RequireAuth/RequireAdmin middleware
- `internal/admin/` — Admin handlers (login, setup, dashboard shell)
- `internal/web/` — Shared helpers (flash messages, layout data, template rendering)

</code_context>

<specifics>
## Specific Ideas

- Login page should be dead simple — centered card on neutral background, no sidebar
- Admin shell should feel like Linear/Ghost — lots of whitespace, restrained color, clear hierarchy
- Use `:has()` and container queries for responsive behavior where possible (modern CSS, no JS)
- View transitions for page-to-page navigation in admin (progressive enhancement)

</specifics>

<deferred>
## Deferred Ideas

- Dark mode (V2-01)
- 2FA/TOTP (V2-09)
- Rate limiting on login
- "Remember me" extended sessions

</deferred>
