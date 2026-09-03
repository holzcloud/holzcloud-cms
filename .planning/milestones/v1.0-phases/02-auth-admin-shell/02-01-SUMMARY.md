---
phase: 02-auth-admin-shell
plan: 01
status: completed
---

# Phase 02 Plan 01: Auth Primitives Summary

JWT-free auth primitives: Argon2id password hashing with PHC string format, custom SCS SQLite session store (no CGO), session manager factory, CSRF key persistence, and RequireAuth/RequireAdmin middleware.

## Key Files

### Created

- `internal/auth/password.go` — Argon2id HashPassword/VerifyPassword with PHC format, configurable params
- `internal/auth/password_test.go` — 6 tests: format, verify correct/wrong, param parsing, salt uniqueness, invalid hash
- `internal/auth/sessionstore.go` — Custom SCS SQLite store using database/sql (no CGO), with cleanup goroutine
- `internal/auth/sessionstore_test.go` — 5 tests: commit+find, non-existent, delete, overwrite, expired
- `internal/auth/session.go` — NewSessionManager factory (24h lifetime, 4h idle, holzcloud_session cookie), session key constants
- `internal/auth/csrf.go` — LoadOrGenerateCSRFKey: reads/generates 32-byte key at data/csrf.key with 0600 perms
- `internal/auth/middleware.go` — RequireAuth (303 redirect), RequireAdmin (403), Chain helper, Middleware type
- `internal/auth/middleware_test.go` — 4 tests: auth redirect, auth pass-through, admin forbid editor, admin pass-through
- `internal/db/migrations/00002_sessions.sql` — Sessions table (token TEXT PK, data BLOB, expiry REAL) + index

### Modified

- `internal/config/config.go` — Added Secure, Argon2Memory, Argon2Iterations, Argon2Parallelism fields with env var loading

## Verification

All 15 tests pass:
```
go test ./internal/auth/ -v -count=1
PASS (0.658s)
```

`go build ./cmd/holzcloud` compiles successfully.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | a4b5555 | Argon2id password hashing, config extension, sessions migration |
| 2 | 00df8b3 | SCS SQLite store, session manager, CSRF key, auth middleware |

## Deviations

None — plan executed exactly as written.
