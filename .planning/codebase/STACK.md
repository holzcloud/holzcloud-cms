# Technology Stack

**Analysis Date:** 2026-09-04

*Corrected surgically against the working tree in Phase 6 (plan `06-04`) — not regenerated.*

## Languages

**Primary:**
- Go 1.26.6 (`go.mod`) - Entire backend, CLI, plugin host. Stdlib-first; no web framework.
- SQL (SQLite dialect) - 45 goose migrations in `internal/db/migrations/` (`00001_*.sql` … `00045_pages_locale_unique.sql`)

**Secondary:**
- Go `html/template` - Admin UI (`cmd/holzcloud/templates/admin/`) and public site templates (`cmd/holzcloud/templates/public/{default,holzcloud,journal,magazine,midnight,rudel,schlicht,weide}/`)
- Plain CSS - `cmd/holzcloud/assets/admin.css`, `cmd/holzcloud/assets/bausteine.css`, per-template `style.css`. No Sass/PostCSS/Tailwind, no build step.
- JavaScript - htmx 2.0.x only, self-hosted at `cmd/holzcloud/assets/htmx.min.js`. No other JS permitted.
- WebAssembly (Go-compiled) - Plugins in `plugins/*/main.go` compiled to `plugin.wasm`

## Runtime

**Environment:**
- Go binary, `CGO_ENABLED=0`, statically linked. Primary target `linux/amd64` (retargeted from 64-bit ARM on 2026-09-03); the container image is built for the same target.
- Container base: `gcr.io/distroless/static-debian12:nonroot`, uid 65532 (`Dockerfile`)
- WASM guest runtime: wazero (pure Go, no CGO) with WASI preview1 (`internal/plugin/runtime.go`)

**Package Manager:**
- Go modules
- Lockfile: `go.sum` present; CI enforces `go mod tidy` cleanliness (`.github/workflows/ci.yml`)
- Second module: `sdk/go.mod` (`github.com/holzcloud/holzcloud-cms/sdk`, go 1.24) — deliberately separate so plugin authors do not pull in the CMS, wazero, or the SQLite driver
- Each plugin under `plugins/*/go.mod` is its own module

## Frameworks

**Core:**
- Go stdlib `net/http` with `http.ServeMux` method+pattern routing (`cmd/holzcloud/main.go`) - HTTP server and routing; no third-party router
- `github.com/alexedwards/scs/v2` v2.9.0 - Server-side sessions persisted in SQLite
- `github.com/gorilla/csrf` v1.7.3 - CSRF middleware, including htmx header validation

**Testing:**
- Go stdlib `testing` - Table-driven and behaviour-named tests co-located as `*_test.go` throughout `internal/`
- Race detector run weekly with `CGO_ENABLED=1` (`.github/workflows/security.yml`)
- Playwright MCP artifacts present in `.playwright-mcp/` (browser-driven exploratory checks, not part of `go test`)

**Build/Dev:**
- `go build -trimpath -ldflags="-s -w -X main.Version=… -X main.Commit=…"` - Single self-contained binary
- Docker multi-stage build (`Dockerfile`, `golang:1.26` builder, `linux/amd64`)
- `tools/mkbundle/main.go` - Bundle packaging helper
- `tools/i18n/main.go` - Translation catalog tooling
- `gofmt` and `go vet` enforced in CI

## Key Dependencies

**Critical:**
- `modernc.org/sqlite` v1.57.0 - Pure-Go SQLite driver; the reason the whole build is CGO-free and cross-compilable without a C toolchain
- `github.com/pressly/goose/v3` v3.27.3 - Embedded versioned SQL migrations from `embed.FS`
- `golang.org/x/crypto` v0.55.0 - Argon2id password hashing (`internal/auth/`)
- `github.com/tetratelabs/wazero` v1.12.0 - WASM plugin sandbox (`internal/plugin/runtime.go`); 2s call timeout, 256 memory pages (16 MB), 8 MB max payload
- `github.com/yuin/goldmark` v1.8.5 - Markdown → HTML
- `github.com/microcosm-cc/bluemonday` v1.0.27 - HTML sanitization; goldmark output is never cast to `template.HTML` unsanitized

**Infrastructure:**
- `golang.org/x/image` v0.45.0 - Image decoding/resizing for media thumbnails and crops (`internal/media/`)
- `golang.org/x/net` v0.58.0 - HTTP/net helpers
- `rsc.io/qr` v0.2.0 - Inline SVG QR codes for TOTP enrolment (`internal/totp/qr.go`)
- `github.com/gorilla/securecookie`, `github.com/gorilla/css`, `github.com/aymerick/douceur` - Indirect, via csrf/bluemonday

## Configuration

**Environment:**
- All configuration is environment-based, parsed and validated in `internal/config/config.go`. Invalid values are collected into an error list rather than silently defaulted.
- Key vars: `HOLZCLOUD_DATA_DIR` (default `data`, `/data` in container), `HOLZCLOUD_PORT` (8080), `HOLZCLOUD_LOG_LEVEL` (INFO), `HOLZCLOUD_SECURE`, `HOLZCLOUD_TRUSTED_PROXIES` (CIDR list), `HOLZCLOUD_MIN_FREE_BYTES` (512 MB)
- Limits: `HOLZCLOUD_MAX_TEMPLATE_SIZE` (10 MB), `HOLZCLOUD_MAX_MEDIA_SIZE` (5 MB), `HOLZCLOUD_MAX_VIDEO_SIZE` (64 MB), `HOLZCLOUD_MAX_MEGAPIXELS` (24, min 1)
- Password hashing: `HOLZCLOUD_ARGON2_MEMORY` (65536 KB), `HOLZCLOUD_ARGON2_ITERATIONS` (1), `HOLZCLOUD_ARGON2_PARALLELISM` (2)
- Mail: `HOLZCLOUD_SMTP_{HOST,PORT,USER,PASSWORD,FROM,FROM_NAME,TLS}`
- No `.env` file in the repository; secrets are supplied by the systemd unit (`deploy/holzcloud.service`)

**Build:**
- `go.mod` / `go.sum`
- `Dockerfile` (build args `VERSION`, `COMMIT`, `TARGETOS`, `TARGETARCH`)
- `.dockerignore`
- `.github/workflows/{ci,image,release,security}.yml`

## Platform Requirements

**Development:**
- Go toolchain matching `go.mod` (1.26.x); no Node, npm, or bundlers anywhere
- No C toolchain needed except to run `go test -race`

**Production:**
- A small `linux/amd64` server via systemd (`deploy/holzcloud.service`) behind Caddy (`deploy/Caddyfile.example`), or
- The container image from `.github/workflows/image.yml` (GHCR) — single instance only, because SQLite tolerates exactly one writer
- Persistent writable volume at `HOLZCLOUD_DATA_DIR`; container filesystem otherwise read-only

---

*Stack analysis: 2026-09-04*
