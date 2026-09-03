# Phase 02: Auth + Admin Shell - Research

**Researched:** 2026-04-14
**Domain:** Authentication, sessions, CSRF, admin UI shell (Go + htmx + CSS)
**Confidence:** HIGH

## Summary

This phase wires authentication (Argon2id passwords, SCS sessions, gorilla/csrf), role-based middleware, first-run setup, and the admin layout shell with OKLCH design tokens. The stack is well-established and the libraries are stable. The main gotcha is that `scs/sqlite3store` pulls in `mattn/go-sqlite3` (CGO) as a module dependency, so we must write a trivial custom SCS store (~80 lines) that uses `database/sql` directly with our existing `modernc.org/sqlite` pools. gorilla/csrf and SCS operate on completely independent cookies -- no conflict.

**Primary recommendation:** Write a custom SCS SQLite store (copy the pattern from upstream, drop the CGO import), wire SCS as outermost middleware, gorilla/csrf inside it on `/admin/*` routes only, and use Go 1.22+ ServeMux method+path patterns for route grouping.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Single-page login at `/admin/login`. Email + password. Flash error "Invalid email or password" on failure. No rate limiting in v1.
- First-run setup: zero users -> `/admin/*` redirects to `/admin/setup`. Creates admin. Returns 404 once any user exists.
- SCS with SQLite store. Lifetime 24h, idle timeout 4h. Cookie `holzcloud_session`. HttpOnly=true, SameSite=Lax. Secure=true when `HOLZCLOUD_SECURE=true`. RenewToken on login.
- gorilla/csrf on all `/admin/*` routes. Token in `<body hx-headers>`. Hidden input fallback for non-htmx forms. 403 on failure.
- Two middleware layers: RequireAuth (redirect to login) and RequireAdmin (403 if role != admin). Role from session, not DB per request.
- Argon2id: memory=64MB, iterations=1, parallelism=2. Configurable via env vars. Target 200-500ms on Pi 5.
- Fixed sidebar 240px, collapsible on mobile via CSS-only (checkbox hack or :has() + details/summary). Nav links, user info, logout.
- OKLCH tokens as specified. @layer cascade. 8px spacing. System font stack. Container queries. View transitions. Light only.

### Claude's Discretion
- Password input masking/reveal toggle
- Exact sidebar animation/transition for mobile
- Flash message positioning (top-center vs inline)
- Login page layout/styling (keep minimal and centered)

### Deferred Ideas (OUT OF SCOPE)
- Dark mode (V2-01)
- 2FA/TOTP (V2-09)
- Rate limiting on login
- "Remember me" extended sessions
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AUTH-01 | Email + password login, Argon2id verification | Argon2id API + hash format documented below |
| AUTH-02 | Server-side sessions in SQLite via SCS, session ID rotated on login | Custom SCS store pattern + RenewToken API |
| AUTH-03 | CSRF on all non-GET admin requests, htmx sends via hx-headers on body | gorilla/csrf Protect + Token + hx-headers pattern |
| AUTH-04 | Secure session cookies: HttpOnly, SameSite=Lax, Secure in prod | SCS Cookie configuration |
| AUTH-05 | Role-based access: admin (full) and editor (content only) via middleware | Middleware chaining pattern with Go 1.22+ mux |
| AUTH-06 | First-run bootstrap: setup form when zero users | Setup handler + guard middleware pattern |
| UI-01 | Admin layout: sidebar nav, responsive, sticky header | CSS @layer + :has() sidebar pattern |
| UI-02 | Design system: OKLCH tokens, @layer cascade, 8px scale, system fonts | CSS architecture documented below |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `alexedwards/scs/v2` | v2.9.0 | Session management | [VERIFIED: go list -m] Standard Go session library, middleware-based |
| `gorilla/csrf` | v1.7.3 | CSRF protection | [VERIFIED: go list -m] Reads X-CSRF-Token header (htmx compatible) |
| `golang.org/x/crypto` | v0.50.0 | Argon2id password hashing | [VERIFIED: go list -m] Official Go crypto package |

### Important: No sqlite3store Module
Do NOT import `github.com/alexedwards/scs/sqlite3store`. It has a hard `go.mod` dependency on `mattn/go-sqlite3` which requires CGO. Instead, write a custom store in `internal/auth/sessionstore.go` (~80 lines). The upstream implementation uses only `database/sql` interfaces. [VERIFIED: read upstream source code]

**Installation:**
```bash
go get github.com/alexedwards/scs/v2@v2.9.0
go get github.com/gorilla/csrf@v1.7.3
go get golang.org/x/crypto@v0.50.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/auth/
  sessionstore.go    -- Custom SCS SQLite store (no CGO)
  password.go        -- Argon2id hash/verify + format encoding
  middleware.go       -- RequireAuth, RequireAdmin, LoadUser
  session.go         -- SCS setup, session keys constants
internal/admin/
  handler.go         -- Admin handler struct
  login.go           -- HandleLoginForm, HandleLogin, HandleLogout
  setup.go           -- HandleSetupForm, HandleSetup
  dashboard.go       -- HandleDashboard (placeholder)
internal/web/
  flash.go           -- Flash message helpers (Get/Set via SCS)
  layoutdata.go      -- Base template data struct (CSRFToken, Flash, User, etc.)
  render.go          -- Template rendering helper
templates/admin/
  base.html          -- Admin layout shell (sidebar, header, content block)
  login.html         -- Login page (no sidebar)
  setup.html         -- First-run setup page
  dashboard.html     -- Dashboard placeholder
assets/
  admin.css          -- Full design system (@layer cascade, OKLCH tokens)
```

### Pattern 1: Custom SCS SQLite Store
**What:** A `database/sql`-only session store compatible with `modernc.org/sqlite`
**When to use:** Always -- replaces sqlite3store to avoid CGO

```go
// internal/auth/sessionstore.go
package auth

import (
    "database/sql"
    "log"
    "time"
)

// SQLiteStore implements scs.Store for modernc.org/sqlite.
type SQLiteStore struct {
    db          *sql.DB
    stopCleanup chan bool
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
    s := &SQLiteStore{db: db}
    s.stopCleanup = make(chan bool)
    go s.startCleanup(5 * time.Minute)
    return s
}

func (s *SQLiteStore) Find(token string) ([]byte, bool, error) {
    row := s.db.QueryRow(
        "SELECT data FROM sessions WHERE token = $1 AND julianday('now') < expiry", token)
    var data []byte
    err := row.Scan(&data)
    if err == sql.ErrNoRows {
        return nil, false, nil
    }
    if err != nil {
        return nil, false, err
    }
    return data, true, nil
}

func (s *SQLiteStore) Commit(token string, data []byte, expiry time.Time) error {
    _, err := s.db.Exec(
        "REPLACE INTO sessions (token, data, expiry) VALUES ($1, $2, julianday($3))",
        token, data, expiry.UTC().Format("2006-01-02T15:04:05.999"))
    return err
}

func (s *SQLiteStore) Delete(token string) error {
    _, err := s.db.Exec("DELETE FROM sessions WHERE token = $1", token)
    return err
}

func (s *SQLiteStore) All() (map[string][]byte, error) {
    rows, err := s.db.Query(
        "SELECT token, data FROM sessions WHERE julianday('now') < expiry")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    sessions := make(map[string][]byte)
    for rows.Next() {
        var token string
        var data []byte
        if err := rows.Scan(&token, &data); err != nil {
            return nil, err
        }
        sessions[token] = data
    }
    return sessions, rows.Err()
}

func (s *SQLiteStore) startCleanup(interval time.Duration) {
    ticker := time.NewTicker(interval)
    for {
        select {
        case <-ticker.C:
            s.db.Exec("DELETE FROM sessions WHERE expiry < julianday('now')")
        case <-s.stopCleanup:
            ticker.Stop()
            return
        }
    }
}

func (s *SQLiteStore) StopCleanup() { s.stopCleanup <- true }
```
[VERIFIED: pattern copied from upstream sqlite3store source, confirmed it uses only database/sql]

**Migration for sessions table:**
```sql
-- internal/db/migrations/002_sessions.sql
-- +goose Up
CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);
CREATE INDEX idx_sessions_expiry ON sessions(expiry);

-- +goose Down
DROP TABLE sessions;
```

### Pattern 2: SCS + gorilla/csrf Middleware Stack
**What:** Correct ordering of middleware -- SCS outermost, CSRF inside
**Critical:** SCS `LoadAndSave` must wrap everything because gorilla/csrf needs cookies written, and SCS needs to commit session changes after handler returns.

```go
// In cmd/holzcloud/main.go or a wiring function

import (
    "github.com/alexedwards/scs/v2"
    gorillacsrf "github.com/gorilla/csrf"
)

// Setup session manager
sessionManager := scs.New()
sessionManager.Store = auth.NewSQLiteStore(database.Write) // sessions write to DB
sessionManager.Lifetime = 24 * time.Hour
sessionManager.IdleTimeout = 4 * time.Hour
sessionManager.Cookie.Name = "holzcloud_session"
sessionManager.Cookie.HttpOnly = true
sessionManager.Cookie.SameSite = http.SameSiteLaxMode
sessionManager.Cookie.Secure = cfg.Secure // from HOLZCLOUD_SECURE env

// CSRF middleware -- needs a 32-byte auth key
csrfKey := loadOrGenerateCSRFKey(cfg) // persist in data/ dir
csrfMiddleware := gorillacsrf.Protect(csrfKey,
    gorillacsrf.Secure(cfg.Secure),
    gorillacsrf.Path("/admin"),
    gorillacsrf.ErrorHandler(http.HandlerFunc(csrfErrorHandler)),
)

// Route wiring
adminMux := http.NewServeMux()
adminMux.HandleFunc("GET /admin/login", adminHandler.HandleLoginForm)
adminMux.HandleFunc("POST /admin/login", adminHandler.HandleLogin)
adminMux.HandleFunc("POST /admin/logout", adminHandler.HandleLogout)
adminMux.HandleFunc("GET /admin/setup", adminHandler.HandleSetupForm)
adminMux.HandleFunc("POST /admin/setup", adminHandler.HandleSetup)
adminMux.HandleFunc("GET /admin/", adminHandler.HandleDashboard) // fallback

// Stack: SCS -> CSRF -> RequireAuth -> handler
// Login/setup routes need CSRF but NOT RequireAuth
mux.Handle("/admin/login", csrfMiddleware(adminMux))
mux.Handle("/admin/setup", csrfMiddleware(adminMux))
mux.Handle("/admin/", csrfMiddleware(requireAuth(adminMux)))

// SCS wraps the entire server
srv.Handler = sessionManager.LoadAndSave(mux)
```

**Key insight:** SCS `LoadAndSave` is an `http.Handler` middleware that must be outermost. gorilla/csrf uses its own independent `_gorilla_csrf` cookie for the CSRF token -- it does NOT conflict with the SCS session cookie. [VERIFIED: read gorilla/csrf store.go source]

### Pattern 3: Argon2id Hash Format
**What:** PHC string format for storing Argon2id hashes (portable, self-describing)

```go
// internal/auth/password.go
package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "fmt"
    "strings"

    "golang.org/x/crypto/argon2"
)

type Argon2Params struct {
    Memory      uint32 // KB
    Iterations  uint32
    Parallelism uint8
    SaltLength  uint32
    KeyLength   uint32
}

var DefaultParams = Argon2Params{
    Memory:      64 * 1024, // 64 MB
    Iterations:  1,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}

func HashPassword(password string, p Argon2Params) (string, error) {
    salt := make([]byte, p.SaltLength)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }
    hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, p.Memory, p.Iterations, p.Parallelism,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
    // Parse PHC format: $argon2id$v=19$m=65536,t=1,p=2$salt$hash
    parts := strings.Split(encoded, "$")
    if len(parts) != 6 {
        return false, fmt.Errorf("invalid hash format")
    }
    var p Argon2Params
    var version int
    fmt.Sscanf(parts[2], "v=%d", &version)
    fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism)
    salt, _ := base64.RawStdEncoding.DecodeString(parts[4])
    expectedHash, _ := base64.RawStdEncoding.DecodeString(parts[5])
    p.KeyLength = uint32(len(expectedHash))

    hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
    return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}
```
[VERIFIED: argon2.IDKey signature confirmed from x/crypto source]

### Pattern 4: Go 1.22+ Middleware Chaining
**What:** stdlib-only middleware pattern without third-party routers

```go
// Middleware type
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in order (first listed = outermost)
func Chain(h http.Handler, mw ...Middleware) http.Handler {
    for i := len(mw) - 1; i >= 0; i-- {
        h = mw[i](h)
    }
    return h
}

// RequireAuth middleware
func RequireAuth(sm *scs.SessionManager) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := sm.GetInt64(r.Context(), "user_id")
            if userID == 0 {
                http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// RequireAdmin middleware
func RequireAdmin(sm *scs.SessionManager) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role := sm.GetString(r.Context(), "user_role")
            if role != "admin" {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### Pattern 5: Template Layout Composition
**What:** Base template + content blocks for admin shell

```go
// templates/admin/base.html
{{define "base"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Holzcloud</title>
    <link rel="stylesheet" href="/assets/admin.css">
    <script src="/assets/htmx.min.js" defer></script>
</head>
<body hx-headers='{"X-CSRF-Token":"{{.CSRFToken}}}'>
    <div class="admin-layout">
        <aside class="sidebar">{{template "sidebar" .}}</aside>
        <main class="main-content">
            <header class="content-header">
                <h1>{{.Title}}</h1>
                {{template "actions" .}}
            </header>
            {{if .Flash.Error}}<div class="flash flash--error">{{.Flash.Error}}</div>{{end}}
            {{if .Flash.Success}}<div class="flash flash--success">{{.Flash.Success}}</div>{{end}}
            {{template "content" .}}
        </main>
    </div>
</body>
</html>
{{end}}
```

```go
// Template rendering helper
// internal/web/render.go
func RenderAdmin(w http.ResponseWriter, r *http.Request, sm *scs.SessionManager, tmpl *template.Template, name string, data map[string]any) {
    if data == nil {
        data = make(map[string]any)
    }
    data["CSRFToken"] = gorillacsrf.Token(r)
    data["Flash"] = map[string]string{
        "Error":   sm.PopString(r.Context(), "flash_error"),
        "Success": sm.PopString(r.Context(), "flash_success"),
    }
    // Check HX-Request for partial rendering
    if r.Header.Get("HX-Request") == "true" {
        w.Header().Set("Vary", "HX-Request")
        tmpl.ExecuteTemplate(w, name+"-partial", data)
        return
    }
    w.Header().Set("Vary", "HX-Request")
    tmpl.ExecuteTemplate(w, "base", data)
}
```

### Pattern 6: CSRF Key Persistence
**What:** gorilla/csrf needs a 32-byte key. Generate once, persist to `data/csrf.key`.

```go
func loadOrGenerateCSRFKey(dataDir string) ([]byte, error) {
    path := filepath.Join(dataDir, "csrf.key")
    key, err := os.ReadFile(path)
    if err == nil && len(key) == 32 {
        return key, nil
    }
    key = make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return nil, err
    }
    return key, os.WriteFile(path, key, 0o600)
}
```

### Anti-Patterns to Avoid
- **Importing `scs/sqlite3store` module:** Pulls in CGO. Write custom store instead.
- **CSRF token in form-only:** htmx AJAX does NOT send hidden form fields. Must use `hx-headers` on `<body>`.
- **Session queries on every request:** Store user_id and role IN the session. Only hit DB on login.
- **Argon2id with hardcoded params in hash check:** Always parse params from the stored PHC string so you can change them later.
- **gorilla/csrf Secure(true) in dev:** Will reject requests over plain HTTP. Must match `HOLZCLOUD_SECURE`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Session management | Custom cookie signing | `alexedwards/scs/v2` | Token rotation, idle timeout, cleanup |
| CSRF protection | Custom token generation | `gorilla/csrf` | Origin checking, timing-safe comparison, cookie signing |
| Password hashing | Custom bcrypt wrapper | `x/crypto/argon2` (Argon2id) | Memory-hard, side-channel resistant |
| Secure random bytes | `math/rand` | `crypto/rand` | Cryptographic randomness required |
| Cookie encoding | Manual base64 | `gorilla/securecookie` (used internally by csrf) | HMAC authentication |

## Common Pitfalls

### Pitfall 1: SCS Session Store Uses Write Pool
**What goes wrong:** Sessions do writes (Commit, Delete). If you pass the read pool, writes fail or cause WAL contention.
**How to avoid:** Pass `database.Write` to `NewSQLiteStore()`. The write pool has `MaxOpenConns(1)` which serializes writes correctly.

### Pitfall 2: gorilla/csrf Rejects htmx Requests
**What goes wrong:** htmx sends AJAX POST/PUT/DELETE but doesn't include CSRF token.
**Why it happens:** htmx doesn't send form hidden fields in AJAX; it uses request headers.
**How to avoid:** Set `hx-headers='{"X-CSRF-Token":"{{.CSRFToken}}"}'` on `<body>`. gorilla/csrf checks `X-CSRF-Token` header by default.
**Warning signs:** 403 errors on all htmx form submissions.

### Pitfall 3: CSRF Secure Flag Mismatch
**What goes wrong:** gorilla/csrf with `Secure(true)` rejects all requests in local dev (no TLS).
**How to avoid:** Pass the same `cfg.Secure` bool to both SCS cookie and gorilla/csrf `Secure()` option.

### Pitfall 4: Session Not Rotated on Login
**What goes wrong:** Session fixation attack -- attacker sets a known session ID before victim logs in.
**How to avoid:** Call `sm.RenewToken(r.Context())` immediately after successful password verification, before setting session values.

### Pitfall 5: Argon2id Memory on Pi 5
**What goes wrong:** 64MB memory cost could cause issues under concurrent logins on 4GB Pi.
**How to avoid:** The Pi 5 has 4-8GB RAM. 64MB per hash is fine for single-user admin. Make params configurable via env vars as decided. Only login triggers hashing.

### Pitfall 6: First-Run Setup Race Condition
**What goes wrong:** Two simultaneous requests to `/admin/setup` could both see zero users and both try to insert.
**How to avoid:** Use `INSERT ... WHERE NOT EXISTS (SELECT 1 FROM users)` or rely on unique email constraint + check row affected count.

### Pitfall 7: Missing Vary Header
**What goes wrong:** CDN/proxy caches full-page response, serves it for htmx partial request (or vice versa).
**How to avoid:** Set `Vary: HX-Request` on EVERY handler that branches on the `HX-Request` header.

## Code Examples

### Login Handler
```go
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) error {
    email := r.FormValue("email")
    password := r.FormValue("password")

    user, err := h.queries.GetUserByEmail(r.Context(), email)
    if err != nil {
        // Same error for not-found and wrong password
        h.sm.Put(r.Context(), "flash_error", "Invalid email or password")
        http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
        return nil
    }

    match, err := auth.VerifyPassword(password, user.PasswordHash)
    if err != nil || !match {
        h.sm.Put(r.Context(), "flash_error", "Invalid email or password")
        http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
        return nil
    }

    // Rotate session ID BEFORE setting values (prevents fixation)
    if err := h.sm.RenewToken(r.Context()); err != nil {
        return err
    }
    h.sm.Put(r.Context(), "user_id", user.ID)
    h.sm.Put(r.Context(), "user_role", user.Role)
    h.sm.Put(r.Context(), "user_email", user.Email)

    http.Redirect(w, r, "/admin/", http.StatusSeeOther)
    return nil
}
```

### CSS @layer Architecture
```css
/* assets/admin.css */
@layer reset, tokens, base, layout, components, utilities;

@layer tokens {
    :root {
        /* Colors - OKLCH */
        --color-bg: oklch(0.99 0 0);
        --color-surface: oklch(1 0 0);
        --color-border: oklch(0.9 0 0);
        --color-text: oklch(0.2 0 0);
        --color-text-muted: oklch(0.55 0 0);
        --color-accent: oklch(0.6 0.15 250);
        --color-accent-hover: oklch(0.55 0.15 250);
        --color-danger: oklch(0.6 0.2 25);
        --color-success: oklch(0.65 0.15 145);

        /* Spacing - 8px scale */
        --space-1: 0.5rem;   /* 8px */
        --space-2: 1rem;     /* 16px */
        --space-3: 1.5rem;   /* 24px */
        --space-4: 2rem;     /* 32px */
        --space-5: 2.5rem;   /* 40px */
        --space-6: 3rem;     /* 48px */
        --space-7: 3.5rem;   /* 56px */
        --space-8: 4rem;     /* 64px */

        /* Typography */
        --font-sans: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
        --font-mono: ui-monospace, "Cascadia Code", "Fira Code", monospace;
        --text-sm: 0.875rem;
        --text-base: 1rem;
        --text-lg: 1.125rem;
        --text-xl: 1.25rem;
        --text-2xl: 1.5rem;

        /* Layout */
        --sidebar-width: 240px;
        --header-height: 56px;
        --radius-sm: 4px;
        --radius-md: 8px;
        --radius-lg: 12px;
    }
}

@layer layout {
    .admin-layout {
        display: grid;
        grid-template-columns: var(--sidebar-width) 1fr;
        min-height: 100dvh;
    }

    /* CSS-only mobile sidebar toggle using :has() + details/summary */
    .sidebar-toggle { display: none; }

    @media (max-width: 768px) {
        .admin-layout {
            grid-template-columns: 1fr;
        }
        .sidebar {
            position: fixed;
            inset-block: 0;
            inset-inline-start: calc(-1 * var(--sidebar-width));
            width: var(--sidebar-width);
            z-index: 100;
            transition: translate 0.2s ease;
        }
        /* :has() toggle -- checkbox in header toggles sidebar */
        .admin-layout:has(.sidebar-toggle:checked) .sidebar {
            translate: 100% 0;
        }
    }
}

/* View transitions (progressive enhancement) */
@view-transition { navigation: auto; }
```

### Users Migration
```sql
-- internal/db/migrations/003_users.sql
-- +goose Up
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'editor' CHECK(role IN ('admin', 'editor')),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
) STRICT;

-- +goose Down
DROP TABLE users;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| bcrypt | Argon2id | ~2019+ | Memory-hard, better resistance to GPU/ASIC attacks |
| gorilla/mux router | Go 1.22+ stdlib ServeMux | Go 1.22 (Feb 2024) | Method+path patterns, no third-party router needed |
| Tailwind/Sass | CSS @layer + OKLCH + container queries | 2023-2024 | Native cascade control, perceptually uniform colors |
| JS hamburger menu | CSS :has() toggle | 2023+ (baseline) | Zero JS for mobile nav |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Pi 5 handles 64MB Argon2id in 200-500ms with iterations=1, parallelism=2 | Argon2id Parameters | Hash too slow/fast -- params are env-configurable so low risk |
| A2 | `modernc.org/sqlite` handles `julianday()` function identically to C SQLite | Session Store | Session expiry checks could break -- very low risk, modernc is a faithful translation |

## Open Questions

1. **CSRF key storage location**
   - What we know: gorilla/csrf needs a persistent 32-byte key
   - Recommendation: Store in `data/csrf.key` (auto-generated on first run). This is decided by implementer.

2. **Session store cleanup goroutine and graceful shutdown**
   - What we know: The custom store starts a background ticker for expired session cleanup
   - Recommendation: Call `StopCleanup()` during graceful shutdown sequence

## Project Constraints (from CLAUDE.md)

- Go 1.22+ stdlib-first, no third-party router
- htmx 2.0.x is the ONLY permitted JS (self-hosted)
- Plain CSS only -- no Tailwind, Sass, PostCSS
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- No npm, no Node, no bundlers
- Dual-pool DB: write pool MaxOpenConns(1), read pool higher
- CSRF token in `<body hx-headers>` -- hidden form fields NOT sent by htmx AJAX
- Markdown sanitization: goldmark -> bluemonday -> template.HTML (not relevant this phase)
- Handlers return error; wrapper writes HTTP response
- `slog.Error` for unexpected; flash messages for user-facing

## Sources

### Primary (HIGH confidence)
- `alexedwards/scs` v2.9.0 -- read session.go, data.go source [VERIFIED]
- `gorilla/csrf` v1.7.3 -- read csrf.go, store.go source [VERIFIED]
- `golang.org/x/crypto/argon2` -- read argon2.go source [VERIFIED]
- `scs/sqlite3store` -- read full source, confirmed CGO dependency in go.mod [VERIFIED]

### Secondary (MEDIUM confidence)
- Go 1.22+ ServeMux patterns -- based on current Go docs and existing main.go usage [VERIFIED: main.go uses method+path patterns]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all libraries verified, versions confirmed, source code read
- Architecture: HIGH - patterns derived from library source code and existing codebase
- Pitfalls: HIGH - identified from source code analysis and known interaction patterns

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (stable libraries, slow-moving domain)
