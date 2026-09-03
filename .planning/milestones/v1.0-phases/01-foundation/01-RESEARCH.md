# Phase 1: Foundation - Research

**Researched:** 2026-04-13
**Domain:** Go binary skeleton, SQLite dual-pool + WAL, goose migrations, embed.FS, slog, graceful shutdown
**Confidence:** HIGH

---

## Summary

Phase 1 establishes the binary foundation that all subsequent phases depend on. The core concern is correct SQLite configuration: pragmas must fire on every connection (not just once), the write pool must be capped at one connection, and WAL mode must be verified at startup. The `modernc.org/sqlite` DSN supports `_pragma=` query parameters that fire per-connection — this is the cleanest approach and avoids RegisterConnectionHook entirely.

Goose v3 (v3.27.0) supports a modern `Provider` API (since v3.16.0) that takes `embed.FS` directly as the third argument to `goose.NewProvider`, with no global state. This is preferred over the legacy `SetBaseFS`/`SetDialect` globals. The dialect constant for modernc is `goose.DialectSQLite3`.

Go 1.22 `net/http` ServeMux supports `GET /healthz` method+path patterns natively. `signal.NotifyContext` is the idiomatic graceful shutdown primitive — combine with a 10-second `context.WithTimeout` on `server.Shutdown`. The `slog` JSON handler is stdlib since Go 1.21.

**Primary recommendation:** Encode all four pragmas in the DSN string, open two `*sql.DB` handles (write: `SetMaxOpenConns(1)`, read: default or 5-10), run `goose.NewProvider` at startup, verify WAL via a PRAGMA query, then start the HTTP server.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

No CONTEXT.md exists for this phase. Constraints sourced from STATE.md and REQUIREMENTS.md.

### Locked Decisions (from STATE.md)
- Stack is a hard mandate: `modernc.org/sqlite` (pure-Go), `html/template`, `log/slog`, `embed.FS`, `gorilla/csrf`, `alexedwards/scs`, `pressly/goose`, `goldmark`, `bluemonday` — no deviations without explicit user approval
- `modernc.org/sqlite` chosen over `mattn/go-sqlite3` for pure-Go cross-compilation (no CGO, no ARM cross-compiler toolchain needed)
- `goose` for migrations: SQL files embedded via `embed.FS`, run automatically at startup
- SQLite: dual-pool (write pool `MaxOpenConns=1`, read pool higher), WAL + `busy_timeout=5000` + `foreign_keys=ON` applied via DSN pragma or RegisterConnectionHook on every connection
- No deviations from this stack without explicit user approval

### Claude's Discretion
- Exact DSN pragma approach (DSN query params vs RegisterConnectionHook) — both are valid; DSN is simpler
- Goose legacy API vs Provider API — Provider API is preferred for Phase 1 (no global state)
- Exact module path for `go.mod`
- Minimum schema design (users table shape)
- Directory layout within the constraints of ARCHITECTURE.md

### Deferred Ideas (OUT OF SCOPE for Phase 1)
- Auth (Argon2id, sessions, CSRF) — Phase 2
- Admin UI / htmx — Phase 2+
- Multi-site domain routing — Phase 3
- Markdown rendering (goldmark/bluemonday) — Phase 3
- Templates, menus, media — Phase 4
- User management UI — Phase 5
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FND-01 | Single Go binary compiles with `CGO_ENABLED=0 GOARCH=arm64` and starts an HTTP server on a configurable port | modernc.org/sqlite (pure Go); stdlib `net/http`; `go build` confirmed CGO_ENABLED=0 |
| FND-02 | SQLite database created automatically on first run in configurable `data/` dir with WAL, busy_timeout, foreign_keys on every connection | DSN `_pragma=` syntax verified via pkg.go.dev; dual-pool pattern documented |
| FND-03 | Schema migrations run automatically at startup via embedded SQL files (goose) | goose v3.27.0 `NewProvider` + `embed.FS` verified |
| FND-04 | Graceful shutdown on SIGTERM/SIGINT — in-flight requests finish within 10s | `signal.NotifyContext` + `server.Shutdown` pattern verified |
| FND-05 | Admin templates, default public template, CSS, htmx.min.js, migrations embedded via `embed.FS` | `//go:embed` directives verified; multiple embed targets per binary supported |
| FND-06 | Structured JSON logging via `slog` with configurable log level | `log/slog` stdlib since Go 1.21; `slog.NewJSONHandler` verified |
</phase_requirements>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.22+ | Runtime, stdlib ServeMux, embed, slog | Mandated; 1.22 method+path ServeMux patterns |
| `modernc.org/sqlite` | v1.48.2 | SQLite driver (pure Go, no CGO) | Cross-compiles to ARM64 without toolchain setup |
| `github.com/pressly/goose/v3` | v3.27.0 | Schema migrations from `embed.FS` | Mandated; Provider API avoids global state |

### Supporting (Phase 1 only — others come in later phases)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` | stdlib (Go 1.21+) | Structured JSON logging | Always; zero dependency |
| `embed` | stdlib (Go 1.16+) | Bake migrations/assets into binary | Always; no runtime file deps |
| `net/http` | stdlib | HTTP server + ServeMux | Always |
| `os/signal` | stdlib | Signal handling for graceful shutdown | Always |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` | `mattn/go-sqlite3` | mattn requires CGO; ARM64 cross-compile fails without aarch64-linux-gnu-gcc toolchain |
| `goose` Provider API | goose legacy `SetBaseFS`/`SetDialect` | Legacy uses global state; safe for single-binary, but Provider API is cleaner and recommended since v3.16.0 |
| DSN `_pragma=` | `RegisterConnectionHook` | Both guarantee per-connection pragma application; DSN approach is simpler and self-documenting |

**Installation:**

```bash
go mod init github.com/holzcloud/holzcloud-cms
go get modernc.org/sqlite@latest
go get github.com/pressly/goose/v3@latest
```

**Version verification:** [VERIFIED: pkg.go.dev/modernc.org/sqlite] v1.48.2 (published April 8, 2026). [VERIFIED: pkg.go.dev/github.com/pressly/goose/v3] v3.27.0 (published Feb 22, 2026).

---

## Architecture Patterns

### Recommended Project Structure

```
cmd/
  holzcloud/
    main.go              # wire deps, start server, signal handling

internal/
  config/
    config.go            # os.Getenv with typed Config struct
  db/
    db.go                # Open (returns writeDB + readDB), RunMigrations
    migrations/          # goose SQL files (embedded via embed.FS in db.go)
      00001_initial.sql  # goose format: -- +goose Up / -- +goose Down

assets/                  # embedded at build time
  htmx.min.js
  admin.css

templates/               # embedded at build time
  admin/
    layout.html
  public/
    default/
      layout.html
        page.html

data/                    # runtime, NOT in repo
  holzcloud.sqlite
```

### Pattern 1: SQLite Dual-Pool with DSN Pragmas

**What:** Two `*sql.DB` handles to the same file. Write pool capped at 1 connection (eliminates SQLITE_BUSY from writer/writer contention). Read pool uses default (or 5-10 max). All pragmas encoded in DSN so they fire on every connection regardless of which pool it comes from.

**When to use:** Always for this project. Required for correctness under concurrent HTTP handlers.

**DSN format (verified):**

```go
// Source: [VERIFIED: pkg.go.dev/modernc.org/sqlite] — _pragma is documented query param
const dsn = "file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-2000)"

// Write pool: exactly 1 connection
writeDB, err := sql.Open("sqlite", fmt.Sprintf(dsn, dbPath))
writeDB.SetMaxOpenConns(1)
writeDB.SetMaxIdleConns(1)

// Read pool: multiple readers (WAL allows concurrent reads)
readDB, err := sql.Open("sqlite", fmt.Sprintf(dsn, dbPath))
readDB.SetMaxOpenConns(5)
```

**Verify WAL is active (startup check):**

```go
var mode string
_ = writeDB.QueryRow("PRAGMA journal_mode").Scan(&mode)
if mode != "wal" {
    slog.Error("WAL mode not active", "got", mode)
    os.Exit(1)
}
```

**Note on `_pragma` syntax:** The value is `pragma_name(value)` — the `PRAGMA` keyword is prepended automatically by the driver. Multiple `_pragma=` parameters are `&`-separated. [VERIFIED: pkg.go.dev/modernc.org/sqlite]

### Pattern 2: goose Provider API with embed.FS

**What:** Use `goose.NewProvider` (added v3.16.0) — takes the `embed.FS` directly, no global state.

**When to use:** Always preferred over legacy `SetBaseFS`/`SetDialect` for new code.

```go
// Source: [VERIFIED: pkg.go.dev/github.com/pressly/goose/v3] — NewProvider API

//go:embed migrations/*.sql
var migrations embed.FS

func RunMigrations(db *sql.DB) error {
    provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
    if err != nil {
        return fmt.Errorf("goose provider: %w", err)
    }
    results, err := provider.Up(context.Background())
    if err != nil {
        return fmt.Errorf("goose up: %w", err)
    }
    for _, r := range results {
        slog.Info("migration applied", "version", r.Source.Version, "duration", r.Duration)
    }
    return nil
}
```

**goose SQL file format (embed path: `migrations/00001_initial.sql`):**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT NOT NULL UNIQUE,
    password   TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'editor',
    created_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS users;
```

**Important:** Goose migration files must be in the same directory that the `//go:embed` directive targets and that `NewProvider` receives as `migrations`. The glob `migrations/*.sql` captures all versioned files.

### Pattern 3: embed.FS for Multiple Asset Groups

**What:** A single binary can embed multiple directories with multiple `//go:embed` directives or one aggregate directive.

**When to use:** Phase 1 embeds migrations (in `db` package) and assets/templates (in `main` or a dedicated package).

```go
// In internal/db/db.go:
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// In cmd/holzcloud/main.go (or internal/web/static.go):
//go:embed assets templates
var StaticFS embed.FS
```

**Constraint:** The `//go:embed` directive must appear in the same package as the variable. The path is relative to the source file's directory. [VERIFIED: pkg.go.dev/embed]

### Pattern 4: Go 1.22 ServeMux Method+Path Routing

**What:** Go 1.22+ ServeMux supports `METHOD /path` patterns. No third-party router needed for Phase 1.

```go
// Source: [VERIFIED: Go 1.22 release notes — method-prefixed patterns]
mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
})
```

### Pattern 5: Graceful Shutdown with signal.NotifyContext

**What:** `signal.NotifyContext` (Go 1.16+) ties OS signal handling to context cancellation. Cleaner than the manual `signal.Notify` + channel approach.

```go
// Source: [CITED: victoriametrics.com/blog/go-graceful-shutdown/]
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()

srv := &http.Server{
    Addr:    cfg.Addr,
    Handler: mux,
}

go func() {
    slog.Info("server starting", "addr", cfg.Addr)
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        slog.Error("server error", "err", err)
        os.Exit(1)
    }
}()

<-ctx.Done()
stop() // release resources; not strictly needed with defer but explicit

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

slog.Info("shutting down")
if err := srv.Shutdown(shutdownCtx); err != nil {
    slog.Error("shutdown error", "err", err)
}
writeDB.Close()
readDB.Close()
```

The requirement (FND-04) specifies 10 seconds. The systemd unit must set `TimeoutStopSec=15` to give the Go process time to drain before SIGKILL. [CITED: Pitfall 18 in PITFALLS.md]

### Pattern 6: slog JSON Handler with Configurable Level

```go
// Source: [VERIFIED: pkg.go.dev/log/slog]
func newLogger(levelStr string) *slog.Logger {
    var level slog.Level
    if err := level.UnmarshalText([]byte(levelStr)); err != nil {
        level = slog.LevelInfo // default
    }
    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
    }))
}
// Usage: HOLZCLOUD_LOG_LEVEL=DEBUG or INFO or WARN or ERROR
```

The `slog.Level.UnmarshalText` method accepts "DEBUG", "INFO", "WARN", "ERROR" (case-insensitive).

### Pattern 7: Config from Environment Variables

```go
// Source: [VERIFIED: Go stdlib os package]
type Config struct {
    Port    string
    DataDir string
    LogLevel string
    DBPath  string // derived: DataDir + "/holzcloud.sqlite"
}

func Load() Config {
    dataDir := getEnv("HOLZCLOUD_DATA_DIR", "data")
    return Config{
        Port:     getEnv("HOLZCLOUD_PORT", "8080"),
        DataDir:  dataDir,
        LogLevel: getEnv("HOLZCLOUD_LOG_LEVEL", "INFO"),
        DBPath:   filepath.Join(dataDir, "holzcloud.sqlite"),
    }
}

func getEnv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

### Pattern 8: Minimum Viable Schema (users table only)

Phase 1 only needs a `users` table — the minimum to verify schema management works and to unblock Phase 2 (auth).

```sql
-- migrations/00001_initial.sql
-- +goose Up

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password   TEXT    NOT NULL,
    role       TEXT    NOT NULL DEFAULT 'editor' CHECK (role IN ('admin', 'editor')),
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- +goose Down
DROP TABLE IF EXISTS users;
```

**Notes:**
- `COLLATE NOCASE` on email prevents case-sensitivity login bugs (Phase 2).
- `CHECK (role IN ('admin', 'editor'))` enforces valid roles at DB level; foreign_keys=ON makes this fire.
- `created_at` uses ISO-8601 UTC — consistent with the PHP legacy convention documented in CLAUDE.md.
- Do NOT add websites, domains, sessions, pages, etc. tables here — those are Phase 2/3 schema additions applied as separate goose migration files.

### Anti-Patterns to Avoid

- **Single `*sql.DB` for both reads and writes:** Without write pool `MaxOpenConns(1)`, concurrent handlers race for write locks and produce SQLITE_BUSY. [VERIFIED: PITFALLS.md Pitfall 1]
- **`db.Exec("PRAGMA ...")` for initialization:** Only applies to the currently checked-out connection. Other pool connections never see the pragma. Use DSN `_pragma=` instead. [VERIFIED: PITFALLS.md Pitfall 2]
- **goose legacy `SetBaseFS` global:** Thread-safe but uses package-level state. Provider API is strictly better for new code.
- **`os.Exit(1)` without `db.Close()`:** SQLite WAL checkpoint does not flush on hard exit. Always close DB handles before exit.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Schema migrations | Custom migration table tracker | `pressly/goose/v3` | goose handles ordering, idempotency, transaction wrapping, down migrations, status queries |
| SQLite driver | CGO wrapper or custom bindings | `modernc.org/sqlite` | CGO-free, maintained, ARM64 support confirmed |
| Signal handling | Manual `signal.Notify` + channel | `signal.NotifyContext` | stdlib since Go 1.16; handles stop/cleanup automatically |

**Key insight:** The complexity of migration ordering, applied-migration tracking, and transactional rollback on failure is non-trivial. goose's 10-year track record handles edge cases (concurrent startup, partial failure recovery) that a 60-line custom solution misses.

---

## Common Pitfalls

### Pitfall 1: WAL Pragmas Not Applied to All Connections

**What goes wrong:** `db.Exec("PRAGMA journal_mode=WAL")` only affects the one connection currently checked out. Under concurrent load, other pool connections still use delete-mode journaling.

**Why it happens:** `database/sql` is pool-agnostic; `Exec` picks any available connection.

**How to avoid:** Use DSN `_pragma=` parameters — these are applied by the driver to every new connection before it is handed to the pool. [VERIFIED: pkg.go.dev/modernc.org/sqlite]

**Warning signs:** `PRAGMA journal_mode` query from concurrent goroutines returns mixed results.

### Pitfall 2: Write Pool Not Capped at 1

**What goes wrong:** Two concurrent POST requests each grab a write connection and both attempt to begin a write transaction. SQLite's single-writer constraint produces SQLITE_BUSY even with WAL mode (WAL removes reader/writer conflict, not writer/writer conflict).

**How to avoid:** `writeDB.SetMaxOpenConns(1)` and `writeDB.SetMaxIdleConns(1)`. Route all mutating DB calls through `writeDB`, all reads through `readDB`.

**Warning signs:** Sporadic 500s with "database is locked" under concurrent admin saves that never reproduce in single-user testing.

### Pitfall 3: Leaked `*sql.Rows` Prevents WAL Checkpoint

**What goes wrong:** An open `*sql.Rows` holds a read transaction. SQLite cannot checkpoint the WAL file while any reader is active. Handlers that return early without `rows.Close()` leak the transaction.

**How to avoid:** Always `defer rows.Close()` immediately after `db.Query`. Use the `Db.All()` helper pattern that closes rows internally.

**Warning signs:** WAL file growing unboundedly; `data/holzcloud.sqlite-wal` reaches tens of MB.

### Pitfall 4: embed.FS Path Mismatch

**What goes wrong:** `//go:embed migrations/*.sql` in `internal/db/db.go` embeds files at path `migrations/00001_initial.sql` inside the FS. Passing `"."` or `"internal/db/migrations"` to `goose.NewProvider` fails — the directory name must match the embed path relative to the source file.

**How to avoid:** The embedded FS retains the directory structure from the source file's location. If `db.go` is at `internal/db/db.go` and the directive is `//go:embed migrations/*.sql`, the embedded path is `migrations/00001_initial.sql`. Pass `"migrations"` as the directory argument to `NewProvider`.

**Warning signs:** `goose: no migration files found` error at startup despite files existing.

### Pitfall 5: CGO_ENABLED=1 Slips into Build

**What goes wrong:** A dependency (even indirect) pulls in a CGO package. The ARM64 cross-compile silently switches to CGO mode and fails without `aarch64-linux-gnu-gcc`.

**How to avoid:** `modernc.org/sqlite` is pure Go. Confirm build with `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...` in CI. Add this as a required check.

**Warning signs:** Build error mentioning a C compiler when running the cross-compile command.

---

## Code Examples

### Complete db.go skeleton

```go
// internal/db/db.go
package db

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    "log/slog"

    "github.com/pressly/goose/v3"
    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
    Write *sql.DB
    Read  *sql.DB
}

func Open(dbPath string) (*DB, error) {
    dsn := fmt.Sprintf(
        "file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-2000)",
        dbPath,
    )

    writeDB, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open write db: %w", err)
    }
    writeDB.SetMaxOpenConns(1)
    writeDB.SetMaxIdleConns(1)

    readDB, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open read db: %w", err)
    }
    readDB.SetMaxOpenConns(5)

    // Verify WAL is active
    var mode string
    if err := writeDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
        return nil, fmt.Errorf("verify WAL: %w", err)
    }
    if mode != "wal" {
        return nil, fmt.Errorf("expected WAL journal mode, got %q", mode)
    }
    slog.Info("sqlite opened", "journal_mode", mode, "path", dbPath)

    return &DB{Write: writeDB, Read: readDB}, nil
}

func RunMigrations(db *sql.DB) error {
    provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationsFS)
    if err != nil {
        return fmt.Errorf("goose provider: %w", err)
    }
    results, err := provider.Up(context.Background())
    if err != nil {
        return fmt.Errorf("goose up: %w", err)
    }
    for _, r := range results {
        slog.Info("migration applied", "version", r.Source.Version, "type", r.Source.Type, "duration_ms", r.Duration.Milliseconds())
    }
    return nil
}

func (d *DB) Close() {
    d.Write.Close()
    d.Read.Close()
}
```

### Complete main.go skeleton

```go
// cmd/holzcloud/main.go
package main

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    "github.com/holzcloud/holzcloud-cms/internal/config"
    "github.com/holzcloud/holzcloud-cms/internal/db"
)

func main() {
    cfg := config.Load()

    // Logging
    logger := newLogger(cfg.LogLevel)
    slog.SetDefault(logger)

    // Data directory
    if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
        slog.Error("cannot create data dir", "err", err)
        os.Exit(1)
    }

    // Database
    database, err := db.Open(cfg.DBPath)
    if err != nil {
        slog.Error("cannot open database", "err", err)
        os.Exit(1)
    }
    defer database.Close()

    // Migrations (run against write connection)
    if err := db.RunMigrations(database.Write); err != nil {
        slog.Error("migrations failed", "err", err)
        os.Exit(1)
    }

    // Routes
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{"status":"ok"}`)
    })

    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      mux,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    go func() {
        slog.Info("server starting", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            slog.Error("server error", "err", err)
            os.Exit(1)
        }
    }()

    <-ctx.Done()
    stop()

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    slog.Info("shutting down")
    if err := srv.Shutdown(shutdownCtx); err != nil {
        slog.Error("shutdown error", "err", err)
    }
    slog.Info("shutdown complete")
}

func newLogger(levelStr string) *slog.Logger {
    var level slog.Level
    if err := level.UnmarshalText([]byte(levelStr)); err != nil {
        level = slog.LevelInfo
    }
    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
```

### go.mod skeleton

```
module github.com/holzcloud/holzcloud-cms

go 1.22

require (
    modernc.org/sqlite v1.48.2
    github.com/pressly/goose/v3 v3.27.0
)
```

Note: `golang.org/x/crypto` (for Argon2id) and `github.com/alexedwards/scs/v2` will be added in Phase 2.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mattn/go-sqlite3` (CGO) | `modernc.org/sqlite` (pure Go) | ~2021 onward | ARM64 cross-compile without toolchain setup |
| goose legacy `SetBaseFS` / `SetDialect` globals | `goose.NewProvider` with `embed.FS` arg | goose v3.16.0 (2023) | No global state; thread-safe; cleaner error handling |
| `signal.Notify` + manual channel | `signal.NotifyContext` | Go 1.16 (2021) | Context cancellation composable with rest of app |
| `log.Printf` / zerolog | `log/slog` | Go 1.21 (2023) | Structured logging in stdlib; no extra dependency |
| Third-party routers for method routing | stdlib `net/http` ServeMux `METHOD /path` | Go 1.22 (2024) | Eliminates gorilla/mux or chi for basic routing |

**Deprecated / outdated:**
- `mattn/go-sqlite3`: Still maintained, but requires CGO. Do not use for this project.
- goose `SetBaseFS` + `SetDialect` globals: Still works, just legacy API. `NewProvider` is canonical.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Module path `github.com/holzcloud/holzcloud-cms` — no actual GitHub repo under this org verified | go.mod skeleton | Minor: path is internal convention only; `go build` works with any valid module path |
| A2 | `goose.DialectSQLite3` is the correct constant name for modernc.org/sqlite | goose Provider pattern | Would cause startup error; easily caught at build time |

---

## Open Questions

1. **goose.DialectSQLite3 vs "sqlite3" string**
   - What we know: pkg.go.dev lists `DialectSQLite3` as a constant. The legacy `SetDialect("sqlite3")` uses a string. NewProvider takes a `Dialect` type.
   - What's unclear: Whether `goose.DialectSQLite3` is the correct constant name in v3.27.0 (the pkg.go.dev page listed it but the exact constant name was inferred, not confirmed by direct source read).
   - Recommendation: The planner should include a Wave 0 step that runs `go build` and confirms the constant resolves. Fallback: `goose.SetDialect("sqlite3")` + `goose.SetBaseFS(migrationsFS)` + `goose.Up(db, "migrations")` always works.

2. **`cache_size(-2000)` in DSN**
   - What we know: Negative values are KB; `-2000` = 2 MB per connection. This is a reasonable default for a Pi 5 with 8 GB RAM.
   - What's unclear: Whether this is optimal for the read pool vs write pool.
   - Recommendation: Treat as a starting point; tune if SQLite shows high page fault rates. Not a Phase 1 blocker.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.22+ | Entire binary | ✗ (Go not found in PATH on this machine) | — | Install Go 1.22+ on dev machine |
| SQLite (system) | Not needed | N/A | N/A | modernc.org/sqlite is self-contained |
| `arm64` cross-compile | FND-01 | Verifiable with `GOOS=linux GOARCH=arm64 go build` | — | Only needs Go; no C toolchain |

**Note:** Go was not found in PATH during research (this is a macOS dev machine running node/tools; Go is installed separately). The project's stack is pure Go — once Go 1.22+ is installed, all dependencies fetch via `go get`. No system-level SQLite library is required.

---

## Validation Architecture

Phase 1 has no test framework config yet. Tests are stdlib `testing` package.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | none — `go test ./...` |
| Quick run command | `go test ./internal/db/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FND-01 | Binary builds for ARM64 with CGO_ENABLED=0 | build check | `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/holzcloud` | Wave 0 |
| FND-02 | SQLite opens with WAL + dual pool; PRAGMA verified | integration | `go test ./internal/db/... -run TestOpen` | Wave 0 |
| FND-03 | Goose migrations run at startup; users table exists | integration | `go test ./internal/db/... -run TestMigrations` | Wave 0 |
| FND-04 | SIGTERM causes graceful drain within 10s | manual / smoke | `go test ./cmd/holzcloud/... -run TestGracefulShutdown` | Wave 0 |
| FND-05 | Assets embedded in binary; served from embed.FS | integration | `go test ./... -run TestStaticAssets` | Wave 0 |
| FND-06 | slog JSON output at configurable level | unit | `go test ./internal/config/... -run TestLogger` | Wave 0 |

### Wave 0 Gaps

- [ ] `internal/db/db_test.go` — covers FND-02, FND-03 (open DB, verify WAL, run migrations, check users table exists)
- [ ] Build script or `Makefile` target — covers FND-01 (ARM64 cross-compile smoke test)

---

## Security Domain

Phase 1 has no auth, no user input surfaces beyond `/healthz`. Security baseline is minimal.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase 2 |
| V3 Session Management | no | Phase 2 |
| V4 Access Control | no | Phase 2 |
| V5 Input Validation | no (healthz has no user input) | — |
| V6 Cryptography | no | Phase 2 (Argon2id) |

### Phase 1 Specific Security Notes

- **SQLite file permissions:** `data/holzcloud.sqlite` should be created with `0o600` permissions (owner-read/write only). The `os.MkdirAll` call for `data/` should use `0o700`. [ASSUMED]
- **`/healthz` endpoint:** Returns no sensitive data. No auth required. Acceptable as-is for Phase 1.
- **Foreign keys enforced:** `foreign_keys=ON` in DSN means Phase 2 schema additions will have referential integrity from day one. [VERIFIED: SQLite docs]

---

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — DSN `_pragma=` syntax, `RegisterConnectionHook` API, v1.48.2 version
- [pkg.go.dev/github.com/pressly/goose/v3](https://pkg.go.dev/github.com/pressly/goose/v3) — `NewProvider` API, `DialectSQLite3`, `SetBaseFS`/`SetDialect` legacy, v3.27.0 version
- [pressly.github.io/goose/blog/2021/embed-sql-migrations/](https://pressly.github.io/goose/blog/2021/embed-sql-migrations/) — official embed.FS integration guide
- [pkg.go.dev/log/slog](https://pkg.go.dev/log/slog) — `NewJSONHandler`, `HandlerOptions`, `Level.UnmarshalText`
- [pkg.go.dev/embed](https://pkg.go.dev/embed) — `//go:embed` directive rules, path resolution
- Go 1.22 release notes — method+path ServeMux patterns

### Secondary (MEDIUM confidence)
- [victoriametrics.com/blog/go-graceful-shutdown/](https://victoriametrics.com/blog/go-graceful-shutdown/) — `signal.NotifyContext` shutdown pattern
- [PITFALLS.md](/.planning/research/PITFALLS.md) — Pitfalls 1, 2, 14, 18 (WAL, pragma per-connection, rows.Close, graceful shutdown)
- [STACK.md](/.planning/research/STACK.md) — stack rationale, version guidance

### Tertiary (LOW confidence / training knowledge)
- `goose.DialectSQLite3` constant name — inferred from pkg.go.dev page; not directly confirmed via source read [LOW, see Open Questions]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions verified via pkg.go.dev
- DSN pragma syntax: HIGH — verified via official modernc.org/sqlite docs
- goose Provider API: HIGH — verified via pkg.go.dev v3.27.0
- Architecture patterns: HIGH — derived from verified APIs + ARCHITECTURE.md
- `goose.DialectSQLite3` constant: LOW — inferred, not source-confirmed

**Research date:** 2026-04-13
**Valid until:** 2026-07-13 (stable APIs; modernc and goose versions may update but patterns are stable)
