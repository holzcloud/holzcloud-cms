# Technology Stack

**Project:** Holzcloud CMS
**Researched:** 2026-04-13
**Stack constraint:** Go + htmx + plain CSS + SQLite — MANDATED, no deviations

---

## Recommended Stack

### Runtime

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.22+ | Entire application | Mandated; 1.22 ServeMux improvements reduce chi dependency |
| SQLite | bundled | Single-file persistence | Pi-friendly, no external service |

### SQLite Driver

**Use: `modernc.org/sqlite` v1.x (pure Go)**

Do NOT use `mattn/go-sqlite3`.

Rationale:
- `mattn/go-sqlite3` requires cgo. Cross-compiling from macOS/Linux x86 to ARM64 (Pi 5) is painful — you must install a cross-compiler toolchain (`gcc-aarch64-linux-gnu`) and set `CGO_ENABLED=1 GOARCH=arm64 GOOS=linux CC=...`. One wrong env var produces a silent link error.
- `modernc.org/sqlite` compiles with `CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build ./...` — a single command, no toolchain setup.
- Performance penalty: modernc is ~2x slower on INSERT-heavy workloads and ~10–30% slower on SELECTs compared to mattn. For a CMS with admin-authored content (low write volume, moderate read volume), this is irrelevant.
- Pi 5 ARM64 is fully supported by modernc.

Pi-friendliness: HIGH. Cross-compilation is the decisive factor.

Confidence: HIGH (verified via benchmark repo + gogs migration issue + official pkg.go.dev docs)

### HTTP Routing

**Use: stdlib `net/http` ServeMux (Go 1.22+)**

Do NOT use gorilla/mux (maintenance burden, heavier). Do NOT add chi unless a concrete blocker appears.

Rationale:
- Go 1.22 ServeMux supports method-prefixed patterns (`GET /admin/{id}`) and URL path values via `r.PathValue("id")`. This covers ~95% of CMS routing needs.
- Zero dependency, zero binary bloat.
- Chi is the right escape hatch if you need middleware grouping with per-group middleware chains (e.g. auth middleware only on `/admin/...`). Chi's radix tree does not meaningfully outperform ServeMux for the request volumes a Pi CMS handles.
- Refactoring from stdlib ServeMux to chi is a 2-hour task, not a rewrite. Start stdlib, add chi only if middleware stacking becomes painful.

Known ServeMux gap: No built-in 405 Method Not Allowed response with custom body. Workaround: catch-all route or a thin middleware wrapper.

Pi-friendliness: HIGH. stdlib = smallest binary contribution.

Confidence: HIGH (alexedwards.net blog, calhoun.io comparison, Go 1.22 release notes)

### Session Management

**Use: `github.com/alexedwards/scs/v2` v2.x with `sqlite3store`**

Do NOT use gorilla/sessions.

Rationale:
- SCS uses ~25% the memory of gorilla/sessions in benchmarks (critical on 8 GB Pi shared across multiple sites).
- SCS stores session data server-side; only a random token goes to the cookie. This is the OWASP-recommended pattern.
- SCS has a native `sqlite3store` backend — no Redis, no Postgres needed. Session data lives in the same SQLite file.
- gorilla/sessions stores the entire session payload in the cookie (encrypted), which adds per-request crypto overhead and limits session size.
- SCS is actively maintained and stable (API frozen since v2).

Pi-friendliness: HIGH. Server-side sessions with SQLite store = no extra process.

Confidence: HIGH (official SCS README, benchmarks gist by author, pkg.go.dev)

### Password Hashing

**Use: `golang.org/x/crypto/argon2` (Argon2id variant)**

Do NOT use bcrypt for new projects (still safe but inferior).

Rationale:
- Argon2id won the Password Hashing Competition and is the NIST-recommended algorithm for new systems.
- Memory-hardness defeats GPU/ASIC parallel attacks that bcrypt cannot resist.
- OWASP minimum parameters for Argon2id: `m=19456` (19 MiB), `t=2`, `p=1`. Pi 5 has 8 GB RAM — this is trivially affordable even with dozens of concurrent login attempts.
- `golang.org/x/crypto` is a first-party extended standard library. No third-party dependency.
- bcrypt is not being dropped from Go — use it only if you're migrating a legacy system.

Pi-friendliness: MEDIUM. Argon2id is intentionally memory-hard. At OWASP minimums (19 MiB per hash), a Pi 5 handles this fine for a low-concurrency admin login. Do not use in a hot path.

Confidence: HIGH (OWASP cheat sheet, alexedwards.net argon2 guide, golang.org/x/crypto docs)

### CSRF Protection

**Use: `github.com/gorilla/csrf` v1.7.x**

Do NOT use Go 1.25's `http.CrossOriginProtection` yet (requires Go 1.25; the project targets 1.22+). Do NOT roll your own.

Rationale:
- gorilla/csrf is the most battle-tested CSRF middleware in the Go ecosystem, actively maintained post-gorilla-revival.
- It integrates cleanly with stdlib net/http via `http.Handler` wrapping — no router lock-in.
- It generates per-request tokens available via template helpers, which compose naturally with htmx's `hx-headers` or hidden form fields.
- `filippo.io/csrf` is a modern Fetch-metadata alternative but requires Go 1.25 and modern browsers only (pre-2020 browsers unprotected).
- For a CMS admin UI on a Pi (non-public, controlled access), gorilla/csrf is the safe, compatible choice.

Note for future phases: When the project moves to Go 1.25+, evaluate migrating to `http.CrossOriginProtection` to drop this dependency.

Pi-friendliness: HIGH. Pure middleware, negligible overhead.

Confidence: HIGH (gorilla/csrf GitHub, alexedwards.net CSRF article, Go 1.25 announcement)

### Database Migrations

**Use: `pressly/goose` v3.x with `//go:embed` — no CLI binary in production**

Do NOT use golang-migrate (heavier, "dirty" state locking is overkill for SQLite). Do NOT use raw schema.sql applied once (no upgrade path).

Rationale:
- Goose v3 natively supports `embed.FS` for baking migration SQL files into the binary. No external files needed at runtime.
- Pattern: `//go:embed migrations/*.sql` → pass `embed.FS` to `goose.SetBaseFS()` → call `goose.Up(db, "migrations")` at startup. The binary is self-contained.
- Goose supports both SQL and Go migrations. For a CMS schema, SQL-only migrations are sufficient and readable.
- golang-migrate's "dirty" state management (must manually clear on failed migration) is unnecessarily complex for a single-developer SQLite project.
- goose's timestamp-based versioning avoids conflicts if multiple branches add migrations simultaneously.

Pi-friendliness: HIGH. Migrations run at binary startup, no external process.

Confidence: HIGH (pressly/goose GitHub + embed docs, goose.io embed blog post)

### htmx

**Use: htmx 2.0.x — self-hosted, vendored into `static/js/htmx.min.js`**

Do NOT use CDN in production. Do NOT use htmx 1.x (deprecated).

Rationale:
- htmx 2.0.0 (released June 2024) is the current stable series. Latest patch: 2.0.8.
- htmx 4.0-alpha exists (2025) but is pre-release — do not use.
- Self-host: download `htmx.min.js` from jsDelivr, commit it to `static/js/`. The Pi serves it from disk with no external network call. CDN usage leaks visitor metadata to third parties and adds a network dependency.
- File size: ~50 KB minified. Negligible.
- No npm, no bundler. One script tag in the base layout. Done.

Pi-friendliness: HIGH. Local file serving, no CDN dependency.

Confidence: HIGH (htmx.org docs, jsDelivr, htmx GitHub)

### Markdown Rendering

**Use: `github.com/yuin/goldmark` v1.x + `github.com/microcosm-cc/bluemonday` v1.x**

Do NOT use blackfriday (not CommonMark-compliant, effectively deprecated in favor of goldmark). Do NOT render raw user HTML without sanitization.

Rationale:
- goldmark is CommonMark-compliant, extensible, and actively maintained. It is now the default Markdown parser in Hugo and was adopted by Gitea to replace blackfriday.
- goldmark ships with tables, strikethrough, task lists, and definition lists as extensions.
- goldmark renders to HTML but does NOT sanitize. Always pipe output through bluemonday.
- bluemonday v1.0.27+ is the standard Go HTML sanitizer, OWASP-inspired, allowlist-based. Use `bluemonday.UGCPolicy()` as the starting point for page content, then adjust as needed.
- Pipeline: `Markdown input → goldmark.Convert() → bluemonday.Sanitize() → store as content_html`.

Pi-friendliness: HIGH. Pure Go, minimal memory.

Confidence: HIGH (goldmark GitHub, gitea PR #9533, bluemonday GitHub)

### HTML Templates

**Use: stdlib `html/template` with layout + partials composition via `define`/`block`**

Do NOT add a third-party template engine.

Pattern to use:

```
templates/
  layouts/
    base.html       ← defines {{block "title"}} {{block "content"}} etc.
  admin/
    dashboard.html  ← {{template "base.html" .}} + {{define "content"}}...{{end}}
  public/
    page.html
  partials/
    nav.html        ← {{define "nav"}}...{{end}}
```

Parse-at-startup via `embed.FS` + `template.ParseFS()`. Store the parsed template set in a struct accessible to handlers. Never parse per-request (performance + security).

Use `html/template`'s automatic context-aware escaping for all admin output. For public pages that store pre-sanitized HTML, use `template.HTML(content)` explicitly — this is intentional and safe ONLY after goldmark+bluemonday sanitization at write time.

Confidence: HIGH (html/template pkg.go.dev, alexedwards.net Let's Go sample, established patterns)

### Config Loading

**Use: stdlib `os.Getenv()` with a typed `Config` struct — no viper**

Do NOT use viper. Do NOT use cobra for a single-binary server.

Rationale:
- Viper adds ~15 MB of transitive dependencies for YAML/TOML/HCL parsing, remote config watching, and pflag integration. None of that is needed.
- For a Pi-deployed binary with a handful of env vars (`HOLZCLOUD_DB_PATH`, `HOLZCLOUD_LISTEN_ADDR`, `HOLZCLOUD_SESSION_KEY`, etc.), stdlib `os.Getenv()` with explicit defaults in a `config.go` file is cleaner, faster to load, and produces no additional binary bloat.
- Pattern: `func Load() Config { return Config{ DBPath: getEnv("HOLZCLOUD_DB_PATH", "data/holzcloud.sqlite"), ... } }` where `getEnv` returns a default if the var is empty.
- A `.env` file is loaded by the systemd unit's `EnvironmentFile=` directive. No Go dotenv library needed.

Pi-friendliness: HIGH. Zero overhead.

Confidence: HIGH (Go stdlib docs, 12-factor app alignment, PROJECT.md constraint)

### Logging

**Use: stdlib `log/slog` (Go 1.21+)**

Do NOT add zerolog or zap.

Rationale:
- slog is in the standard library since Go 1.21. Zero dependency, zero binary bloat.
- For a CMS where the primary "observability" is `journalctl -u holzcloud`, structured JSON logs via `slog.New(slog.NewJSONHandler(os.Stdout, nil))` are sufficient and parseable by standard tools.
- zerolog is ~20% faster in throughput benchmarks, but the difference matters at millions of log lines/second — irrelevant on a Pi CMS.
- slog supports both text and JSON handlers. Text for dev (`slog.NewTextHandler`), JSON for production.

Pi-friendliness: HIGH. Standard library = no extra binary size.

Confidence: HIGH (Go 1.21 release notes, betterstack.com slog guide, leapcell.io comparison)

### Live Reload (Dev Only)

**Use: `github.com/air-verse/air`**

Install as a dev tool only (`go install`), never as a project dependency.

Rationale:
- air is the de-facto standard live-reload tool for Go. Actively maintained (air-verse org, updated April 2025).
- Detects file changes, rebuilds, restarts. Works perfectly with a plain `go build` + binary execution model.
- Install with: `go install github.com/air-verse/air@latest`
- Configure via `.air.toml` in project root (excluded from production builds and deployment).
- reflex is a lighter alternative but has less documentation and fewer users. air's `.air.toml` is well-documented.

Note: air requires Go 1.22+, which matches the project minimum.

Pi-friendliness: N/A (dev machine only; Pi runs the compiled binary).

Confidence: HIGH (air-verse/air GitHub, active maintenance confirmed April 2025)

### Testing

**Use: stdlib `testing` + table-driven tests. Add `github.com/stretchr/testify/assert` for assertions only if the test verbosity becomes painful.**

Do NOT add testify as a hard dependency upfront. Do NOT use BDD frameworks (ginkgo/gomega).

Rationale:
- Go's stdlib testing package with table-driven tests is the idiomatic, universally understood pattern. It has no dependencies and produces no binary bloat (test binaries are separate).
- testify/assert reduces assertion boilerplate (`require.NoError(t, err)` vs multi-line `if err != nil { t.Fatal(...) }`). It is the right call when tests grow complex. Add it when you feel the pain, not before.
- testify/mock is heavier and encourages over-mocking. Prefer interface-based test doubles or SQLite in-memory databases (`?mode=memory&cache=shared`) for DB integration tests.
- BDD frameworks (ginkgo) are incompatible with the project's "minimal dependencies" philosophy.

Pi-friendliness: N/A (tests run on the developer's machine).

Confidence: HIGH (Go wiki TableDrivenTests, Dave Cheney blog, testify GitHub)

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| SQLite driver | modernc.org/sqlite | mattn/go-sqlite3 | Requires cgo; cross-compilation to ARM64 is painful |
| HTTP router | stdlib ServeMux | chi | No feature gap for this project's routes; add chi if middleware grouping becomes unwieldy |
| Sessions | alexedwards/scs | gorilla/sessions | gorilla stores full payload in cookie; 4x memory overhead |
| Password hashing | argon2id | bcrypt | bcrypt is CPU-hard only; argon2id adds memory-hardness against GPU attacks |
| Migrations | goose | golang-migrate | golang-migrate's "dirty" state locking is overkill for SQLite; goose embed support is simpler |
| Markdown | goldmark | blackfriday | blackfriday is not CommonMark-compliant and effectively abandoned |
| Config | stdlib os.Getenv | viper | viper adds 15 MB of transitive deps for YAML/remote config that this project does not need |
| Logging | slog | zerolog | zerolog is faster at high volume; irrelevant at Pi CMS traffic levels; slog avoids a dependency |
| Live reload | air | reflex | air has better documentation and is more actively maintained |

---

## Installation

```bash
# Core runtime dependencies (go.mod)
go get modernc.org/sqlite@latest
go get github.com/alexedwards/scs/v2@latest
go get github.com/gorilla/csrf@latest
go get github.com/pressly/goose/v3@latest
go get github.com/yuin/goldmark@latest
go get github.com/microcosm-cc/bluemonday@latest
go get golang.org/x/crypto@latest

# Dev tool (not in go.mod)
go install github.com/air-verse/air@latest
```

No npm. No Makefile required for the build. `go build -o holzcloud ./cmd/holzcloud` produces the production binary.

---

## ARM64 / Pi 5 Summary

| Concern | Decision | Impact |
|---------|----------|--------|
| Cross-compilation | modernc.org/sqlite (no cgo) | `CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build` works out of the box |
| Binary size | stdlib-first stack | Estimated <20 MB binary including all dependencies |
| Memory | SCS server-side sessions + slog | Low per-request allocation overhead |
| Argon2id at login | 19 MiB RAM per hash operation | Safe at low concurrency; do not expose login to brute-force (rate-limit or fail2ban) |
| No Docker required | Single binary + systemd | `go build` → `scp` → `systemctl restart holzcloud` |

---

## Sources

- modernc.org/sqlite vs mattn benchmark: https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html
- gogs migration to modernc: https://github.com/gogs/gogs/issues/7882
- Go 1.22 ServeMux vs chi: https://www.calhoun.io/go-servemux-vs-chi/ and https://www.alexedwards.net/blog/which-go-router-should-i-use
- SCS session manager: https://www.alexedwards.net/blog/scs-session-manager
- Argon2 in Go: https://www.alexedwards.net/blog/how-to-hash-and-verify-passwords-with-argon2-in-go
- OWASP Password Storage: https://guptadeepak.com/the-complete-guide-to-password-hashing-argon2-vs-bcrypt-vs-scrypt-vs-pbkdf2-2026/
- goose embed: https://pressly.github.io/goose/blog/2021/embed-sql-migrations/
- htmx 2.0 release: https://htmx.org/posts/2024-06-17-htmx-2-0-0-is-released/
- goldmark vs blackfriday (gitea): https://github.com/go-gitea/gitea/pull/9533
- bluemonday: https://github.com/microcosm-cc/bluemonday
- slog guide: https://betterstack.com/community/guides/logging/logging-in-go/
- air live reload: https://github.com/air-verse/air
- gorilla/csrf: https://github.com/gorilla/csrf
- Go 1.25 CrossOriginProtection: https://www.samueladebayo.dev/posts/golang-cross-origin-protection/
