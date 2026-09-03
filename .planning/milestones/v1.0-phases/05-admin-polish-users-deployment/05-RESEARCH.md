# Phase 05: Admin Polish + Users + Deployment - Research

**Researched:** 2026-04-14
**Domain:** Admin UX polish, user management, production deployment
**Confidence:** HIGH

## Summary

Phase 5 is the final v1 phase. It covers three distinct areas: (1) UI polish -- no-JS fallback audit, flash message auto-dismiss, dashboard with stats, view transition names, loading indicators; (2) user CRUD with password change safety guards; (3) deployment artifacts (systemd, Caddy, backup docs). All patterns are well-established from prior phases. The codebase already has flash messages, htmx partial/full branching, OKLCH design tokens, and `@view-transition { navigation: auto; }` in CSS. The users table exists but lacks a `name` column -- a migration is needed.

**Primary recommendation:** Split into 3 plans: (1) UI polish (flash auto-dismiss, loading indicators, view transition names, dashboard stats, no-JS audit), (2) user CRUD + password management, (3) deployment files.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01/D-02/D-03: No-JS fallback audit -- all handlers already check HX-Request; verify non-htmx branch returns full page
- D-04/D-05/D-06: Flash messages -- CSS-animated auto-dismiss banners (5s), types: success/error/warning, OKLCH tokens. htmx: flash in swapped HTML or HX-Trigger
- D-07/D-08: Dashboard -- website count, total pages, total media, last 5 page edits. Cards per website with quick links. Single query with joins.
- D-09/D-10: View transitions -- `@view-transition { navigation: auto; }` (already exists). Add `view-transition-name` to sidebar, main content, page title. 3-4 named max.
- D-11/D-12: Loading indicators -- `hx-indicator` on all buttons/forms, `hx-disabled-elt="this"` on submits, global GitHub-style progress bar via `htmx-request` class
- D-13 through D-16: User CRUD at /admin/users. Roles: admin/editor. Password change: self requires current password, admin override does not. Cannot delete self. Cannot demote last admin.
- D-17 through D-21: Deployment in `deploy/` dir: holzcloud.service, Caddyfile.example, DEPLOY.md, backup.sh

### Claude's Discretion
- Spinner/loading indicator design
- Dashboard layout (grid vs list)
- View transition timing/easing
- Whether to add first-login password change prompt (recommend: no)

### Deferred Ideas (OUT OF SCOPE)
- Dark mode (V2-01)
- Activity log (V2-07)
- Email-based password reset (V2-08)
- 2FA/TOTP (V2-09)
- Docker deployment
- Maintenance mode per site (V2-13)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UI-03 | All admin interactions work without htmx | Existing HX-Request branching in RenderAdmin; audit all handlers for full-page fallback |
| UI-04 | Flash messages after form actions | Flash system exists in web/flash.go; CSS styles exist; need auto-dismiss animation + warning type + htmx delivery |
| UI-05 | Admin dashboard with website overview | dashboard.go exists as stub; need stats queries + template |
| UI-06 | View transitions for admin nav | `@view-transition { navigation: auto; }` already in CSS; add named transitions |
| UI-07 | Loading indicators for htmx requests | Add hx-indicator attrs + global progress bar CSS |
| USR-01 | User CRUD (list, create, edit, delete) | Users table exists (needs `name` column migration); follows established CRUD pattern |
| USR-02 | Password change with current password requirement | auth/password.go has HashPassword + VerifyPassword; need handler logic for self vs admin-override |
| DEP-01 | Single binary for linux/arm64 | Build command in CLAUDE.md; no new code needed, just document |
| DEP-02 | systemd unit with security hardening | Write holzcloud.service file |
| DEP-03 | Caddy reverse proxy config | Write Caddyfile.example |
| DEP-04 | Backup strategy for data/ | Write backup.sh + document in DEPLOY.md |
</phase_requirements>

## Standard Stack

No new dependencies. This phase uses only what is already in the project.

### Core (existing)
| Library | Purpose | Already Used |
|---------|---------|-------------|
| `alexedwards/scs/v2` | Sessions (flash messages, user context) | Yes |
| `golang.org/x/crypto/argon2` | Password hashing for user management | Yes |
| `gorilla/csrf` | CSRF protection on user forms | Yes |
| `pressly/goose/v3` | Migration for users.name column | Yes |
| `modernc.org/sqlite` | All queries | Yes |

[VERIFIED: codebase inspection of go.mod dependencies]

## Architecture Patterns

### Migration: Add `name` Column to Users

The existing users table (migration 00001) has: id, email, password, role, created_at. CONTEXT.md D-13 specifies "Fields: name, email, password, role." A new migration is needed:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN name;
```

File: `internal/db/migrations/00006_user_name.sql` [VERIFIED: 00005 is the last migration]

### User CRUD Pattern

Follows the exact pattern from website.go, page.go, etc.:

1. **Store** in a new `internal/user/` package (or inline in admin, but separate package is cleaner):
   - `List(ctx) ([]User, error)` -- all users
   - `GetByID(ctx, id) (*User, error)`
   - `GetByEmail(ctx, email) (*User, error)` -- already used in login.go
   - `Create(ctx, name, email, hash, role) (int64, error)`
   - `Update(ctx, id, name, email, role) error`
   - `UpdatePassword(ctx, id, hash) error`
   - `Delete(ctx, id) error`
   - `CountByRole(ctx, role) (int, error)` -- for "cannot demote last admin" guard

2. **Handlers** in `internal/admin/user.go`:
   - `HandleUserList` -- GET /admin/users
   - `HandleUserCreate` -- GET (form) + POST (create)
   - `HandleUserEdit` -- GET (form) + POST (update)
   - `HandleUserDelete` -- POST /admin/users/{id}/delete
   - `HandlePasswordChange` -- POST /admin/users/{id}/password

3. **Templates**: `user_list.html`, `user_form.html` (reused for create/edit), `user_password.html`

4. **Safety guards** (D-15, D-16):
   - Self-password-change: verify current password via `auth.VerifyPassword` before hashing new one
   - Admin-override: skip current password check when `session_user_id != target_user_id`
   - Cannot delete self: `if session_user_id == target_id { flash error; redirect }`
   - Cannot demote last admin: before role change, count admins; if target is admin and new role != admin and count == 1, reject

[VERIFIED: patterns from internal/admin/website.go, internal/admin/page.go]

### Dashboard Stats Query

Single query approach per D-08:

```sql
SELECT
    (SELECT COUNT(*) FROM websites) AS website_count,
    (SELECT COUNT(*) FROM pages) AS page_count,
    (SELECT COUNT(*) FROM media) AS media_count;
```

Plus per-website stats:

```sql
SELECT w.id, w.name,
    (SELECT COUNT(*) FROM pages p WHERE p.website_id = w.id) AS page_count,
    (SELECT COUNT(*) FROM media m WHERE m.website_id = w.id) AS media_count
FROM websites w ORDER BY w.name;
```

Recent activity (last 5 page edits):

```sql
SELECT p.id, p.title, p.updated_at, w.name AS website_name
FROM pages p JOIN websites w ON p.website_id = w.id
ORDER BY p.updated_at DESC LIMIT 5;
```

[ASSUMED: exact column names -- verify pages has updated_at]

### Flash Auto-Dismiss CSS

```css
@layer components {
    .flash {
        animation: flash-dismiss 5s ease-in-out forwards;
    }

    @keyframes flash-dismiss {
        0%, 80% { opacity: 1; max-height: 4rem; padding: var(--space-1) var(--space-2); margin: var(--space-2) var(--space-4); }
        100% { opacity: 0; max-height: 0; padding: 0; margin: 0; overflow: hidden; }
    }
}
```

No JS required -- pure CSS animation. [VERIFIED: CSS animations can handle opacity + max-height collapse]

### Loading Indicators

**Per-element:** htmx's built-in `hx-indicator` class. When a request is in-flight, htmx adds `htmx-request` class to the element. CSS:

```css
.htmx-indicator {
    display: none;
}
.htmx-request .htmx-indicator {
    display: inline-block;
}
.htmx-request.htmx-indicator {
    display: inline-block;
}
```

**Global progress bar** (GitHub-style):

```html
<!-- In base.html, before .admin-layout -->
<div class="global-progress" aria-hidden="true"></div>
```

```css
.global-progress {
    position: fixed; top: 0; left: 0; right: 0;
    height: 3px; z-index: 9999;
    background: var(--color-primary);
    transform: scaleX(0); transform-origin: left;
    transition: transform 0.3s ease;
}
body.htmx-request .global-progress {
    transform: scaleX(0.9);
    transition: transform 10s cubic-bezier(0.1, 0.05, 0, 1);
}
```

htmx adds `htmx-request` to `<body>` during requests when using `hx-indicator="body"` or by default on certain configurations. Actually, htmx adds `htmx-request` to the element that triggered the request. For a global indicator, use `hx-indicator` on `<body>`:

```html
<body hx-indicator=".global-progress" ...>
```

Wait -- htmx 2.x behavior: `hx-indicator` specifies which element gets the `htmx-request` class. But for the body-level approach, better to use htmx events via CSS. Actually the simplest approach: add `hx-indicator="closest body"` is not needed. htmx 2.x adds `htmx-request` to the triggering element by default. For a global bar, the pattern is:

```css
body:has(.htmx-request) .global-progress {
    transform: scaleX(0.9);
}
```

Using `:has()` is perfect here -- supported in all modern browsers and aligns with the project's modern CSS approach. [VERIFIED: `:has()` is in the project's approved modern CSS techniques per memory/modern_techniques.md]

### View Transition Names

Already have `@view-transition { navigation: auto; }`. Add named transitions for smooth morphing:

```css
.sidebar { view-transition-name: sidebar; }
.main-content { view-transition-name: main; }
.content-header h1 { view-transition-name: page-title; }
```

Optional animation customization:

```css
::view-transition-old(main) { animation: fade-out 0.15s ease; }
::view-transition-new(main) { animation: fade-in 0.15s ease; }
```

Keep minimal per D-10. [CITED: MDN View Transitions API]

### No-JS Fallback Audit

All admin handlers already use `web.RenderAdmin()` which checks `HX-Request` header and renders full page vs partial. The audit needs to verify:

1. Every POST handler redirects (303) on success for non-htmx requests (PRG pattern)
2. Every handler that returns partial HTML for htmx also has a full-page path
3. Inline editing (page title/slug) degrades to link-to-edit-form

Handlers to audit: [VERIFIED: from codebase]
- `internal/admin/website.go` -- CRUD + domain add/remove
- `internal/admin/page.go` -- CRUD + inline edit + status toggle + pagination
- `internal/admin/template.go` -- upload, activate, delete
- `internal/admin/menu.go` -- CRUD + item CRUD + reorder
- `internal/admin/media.go` -- upload, delete

Key check: POST handlers must use `http.Redirect(w, r, url, http.StatusSeeOther)` for non-htmx, and `HX-Redirect` header for htmx requests.

### Deployment Files

**systemd unit** (`deploy/holzcloud.service`):

```ini
[Unit]
Description=Holzcloud CMS
After=network.target

[Service]
Type=simple
User=holzcloud
Group=holzcloud
WorkingDirectory=/opt/holzcloud
ExecStart=/opt/holzcloud/holzcloud
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/holzcloud/data
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictNamespaces=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
```

[CITED: systemd.exec(5) man page for hardening flags]

**Caddy** (`deploy/Caddyfile.example`):

```
your-domain.example.com {
    reverse_proxy localhost:8080
}

# Multi-domain: add one block per domain
another-domain.example.com {
    reverse_proxy localhost:8080
}
```

Caddy handles TLS automatically via Let's Encrypt. [CITED: caddyserver.com/docs]

**Backup script** (`deploy/backup.sh`):

```bash
#!/bin/bash
set -euo pipefail
BACKUP_DIR="/opt/holzcloud/backups/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"
sqlite3 /opt/holzcloud/data/holzcloud.sqlite ".backup '$BACKUP_DIR/holzcloud.sqlite'"
cp -r /opt/holzcloud/data/media "$BACKUP_DIR/"
cp -r /opt/holzcloud/data/templates "$BACKUP_DIR/" 2>/dev/null || true
```

SD card wear concern: document USB SSD recommendation per STATE.md known risks. [VERIFIED: STATE.md mentions SD card fsync unreliability]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom hash | `auth.HashPassword` / `auth.VerifyPassword` (already exists) | Argon2id with proper constant-time comparison |
| Flash messages | Custom cookie system | `web.SetFlash*` / `web.GetFlash` (already exists) | Session-based, already wired |
| CSRF | Custom token | `gorilla/csrf` (already exists) | Proven library, htmx integration done |
| Loading indicators | Custom JS | htmx built-in `htmx-request` class + CSS | Zero custom JS needed |
| TLS certificates | Manual cert management | Caddy automatic HTTPS | Auto-renewal, no cron needed |

## Common Pitfalls

### Pitfall 1: Forgetting HX-Redirect for htmx POST responses
**What goes wrong:** htmx ignores 302/303 redirects from AJAX -- the redirect HTML loads inside the swap target.
**How to avoid:** For htmx requests, set `w.Header().Set("HX-Redirect", url)` and return 200. For non-htmx, use `http.Redirect()`.
**Warning signs:** After form submit, page content appears duplicated/nested.

### Pitfall 2: Last-admin demotion race condition
**What goes wrong:** Two admins simultaneously demote each other, leaving zero admins.
**How to avoid:** Check admin count inside the write transaction. With SQLite single-writer (MaxOpenConns=1 on write pool), this is naturally serialized.

### Pitfall 3: Flash messages lost on htmx partial swap
**What goes wrong:** Flash is set in session, but htmx swaps only a partial -- the flash div in base.html is not re-rendered.
**How to avoid:** For htmx responses after mutations, either (a) include flash HTML in the partial response via `hx-swap-oob`, or (b) use `HX-Trigger` header to fire a client event that fetches the flash. Option (a) is simpler and works without JS.

### Pitfall 4: View transition name conflicts
**What goes wrong:** Two elements on the same page with the same `view-transition-name` causes the transition to fail silently.
**How to avoid:** Only assign names to unique, persistent elements (sidebar, main content area, page title). Never on list items.

### Pitfall 5: SQLite backup while writes are active
**What goes wrong:** Copying the SQLite file directly during writes can produce a corrupt backup.
**How to avoid:** Use `.backup` command via `sqlite3` CLI which handles WAL correctly, or use the SQLite backup API.

## Code Examples

### htmx Flash via OOB Swap

For POST handlers responding to htmx requests, include the flash as an out-of-band swap:

```go
if r.Header.Get("HX-Request") == "true" {
    // Main response partial
    // Plus OOB flash
    fmt.Fprintf(w, `<div id="flash-area" hx-swap-oob="innerHTML">
        <div class="flash flash--success">%s</div>
    </div>`, template.HTMLEscapeString(msg))
    return nil
}
// Non-htmx: set flash in session, redirect
web.SetFlashSuccess(h.sm, r.Context(), msg)
http.Redirect(w, r, redirectURL, http.StatusSeeOther)
```

### Password Change Handler Pattern

```go
func (h *Handler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) error {
    targetID := /* from URL */
    sessionUserID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
    
    newPassword := r.FormValue("new_password")
    confirmPassword := r.FormValue("confirm_password")
    if newPassword != confirmPassword {
        // flash error, return
    }
    
    // Self-change requires current password
    if targetID == sessionUserID {
        currentPassword := r.FormValue("current_password")
        user, _ := h.userStore.GetByID(r.Context(), targetID)
        ok, _ := auth.VerifyPassword(currentPassword, user.Password)
        if !ok {
            // flash error: "Current password is incorrect"
            return
        }
    }
    
    hash, err := auth.HashPassword(newPassword, h.argon2Params)
    // ... update in DB
}
```

### Global Progress Bar with :has()

```css
.global-progress {
    position: fixed;
    top: 0; left: 0; right: 0;
    height: 3px;
    z-index: 9999;
    background: var(--color-primary);
    transform: scaleX(0);
    transform-origin: left;
    opacity: 0;
}

body:has(.htmx-request) .global-progress {
    opacity: 1;
    transform: scaleX(0.9);
    transition: transform 8s cubic-bezier(0.1, 0.05, 0, 1), opacity 0.2s;
}
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | pages table has `updated_at` column | Dashboard Stats Query | Dashboard recent activity query would fail; check migration 00004 |
| A2 | htmx 2.x adds `htmx-request` class to triggering element during requests | Loading Indicators | Progress bar CSS selector may need adjustment |

## Open Questions

1. **Does pages table have `updated_at`?**
   - What we know: Migration 00004 defines pages table; need to check
   - Recommendation: Verify in migration file; if missing, use `created_at` or add column

2. **Flash warning type**
   - D-05 mentions warning (yellow) type but current flash.go only has Error and Success
   - Recommendation: Add `SetFlashWarning` and `flash_warning` session key; add `.flash--warning` CSS

## Sources

### Primary (HIGH confidence)
- Codebase inspection: internal/admin/*.go, internal/auth/*.go, internal/web/*.go, cmd/holzcloud/templates/admin/*.html, cmd/holzcloud/assets/admin.css
- Migration files: internal/db/migrations/00001-00005

### Secondary (MEDIUM confidence)
- MDN View Transitions API documentation
- systemd.exec(5) hardening flags
- Caddy reverse proxy documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies, all patterns established
- Architecture: HIGH - follows existing CRUD patterns exactly
- Pitfalls: HIGH - well-known htmx + Go patterns
- Deployment: MEDIUM - systemd flags verified but exact Pi 5 behavior assumed

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (stable domain, no fast-moving dependencies)
