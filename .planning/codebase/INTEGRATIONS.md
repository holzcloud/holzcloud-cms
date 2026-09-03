# External Integrations

**Analysis Date:** 2026-08-22

## Governing Constraint

Nothing may be fetched from a third party while the application runs (`CLAUDE.md`). No CDN, web fonts, remote images, analytics, or off-site iframes. Enforced twice:
- `internal/web/headers.go` - `Content-Security-Policy: default-src 'self'` on every response
- `internal/tmplmgr/external.go` - template archives are rejected at upload if they reference an external subresource

Grep for outbound HTTP clients across `internal/` and `cmd/` returns nothing: there is no `http.Get`, `http.Post`, or `http.Client` in application code. The only outbound connection the process makes is SMTP.

## APIs & External Services

**Outbound (server-initiated):**
- SMTP mail relay - the single egress path (`internal/mail/mail.go`). Uses stdlib `net/smtp` + `crypto/tls`. Off unless `HOLZCLOUD_SMTP_HOST` and `HOLZCLOUD_SMTP_FROM` are both set; with it off, invitation and password links are shown on screen / logged instead. Plain text only, one recipient per message, never HTML.
  - SDK/Client: Go stdlib, no third-party package
  - Auth: `HOLZCLOUD_SMTP_USER` / `HOLZCLOUD_SMTP_PASSWORD` (may be empty for a network-authenticated local relay)
  - TLS mode: `HOLZCLOUD_SMTP_TLS` = `starttls` (default) | `tls` | `none`
  - Queued and retried via `internal/mail/queue.go`

**Inbound (assistant-initiated):**
- Model Context Protocol (MCP) server at `/ai` - `internal/ai/mcp.go`, `internal/ai/tools.go`, `internal/ai/token.go`. JSON-RPC 2.0 over HTTP, protocol revision `2025-06-18`, hand-written (no MCP SDK dependency). Wired in `cmd/holzcloud/main.go:259-261`.
  - Direction is inwards only: the CMS never calls an AI provider and stores no provider API key.
  - Auth: bearer token, hashed and stored in SQLite (`internal/ai/token.go`, SHA-256 + `crypto/rand`)
  - Request cap: 4 MB (`MaxRequestBytes`)
  - Tools operate through the same stores, validation, slug rules, and revision history as the admin UI

**Content import:**
- WordPress WXR import - `internal/wxr/`. Parses uploaded `wp:`/`content:` namespaced RSS export files. File upload, not a network call to WordPress.

## Data Storage

**Databases:**
- SQLite via `modernc.org/sqlite` (pure Go). Opened in `internal/db/db.go` with a dual pool: write pool `SetMaxOpenConns(1)`, larger read pool.
  - Connection: local file under `HOLZCLOUD_DATA_DIR`
  - Pragmas on every connection: `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL`
  - Migrations: `pressly/goose/v3` over `embed.FS`, 38 files in `internal/db/migrations/`
  - Direct SQL only, no ORM; STRICT tables where possible
  - Maintenance: `internal/db/maintain.go`; `holzcloud compact`, `holzcloud check` CLI subcommands

**File Storage:**
- Local filesystem only, under `HOLZCLOUD_DATA_DIR` — SQLite database, media uploads, uploaded templates, plugin `.wasm` modules, pre-upgrade snapshots. No S3 or object store.
- Free-space floor enforced: `/readyz` reports unready below `HOLZCLOUD_MIN_FREE_BYTES` (512 MB default)

**Caching:**
- None. No Redis, no Memcached. Per-process in-memory state only.

## Authentication & Identity

**Auth Provider:**
- Custom, self-contained (`internal/auth/`). No OAuth, no external IdP.
  - Passwords: Argon2id via `golang.org/x/crypto/argon2`, parameters tunable for the Pi
  - Sessions: `alexedwards/scs/v2` stored server-side in SQLite; rotated on login
  - CSRF: `gorilla/csrf` on all state-changing requests, token relayed through `hx-headers` for htmx
  - Two-factor: TOTP (`internal/totp/`), enrolment QR rendered inline as SVG with `rsc.io/qr`; recovery via `holzcloud user 2fa disable`
  - Share links: signed, page-scoped, expiring tokens (`internal/sharelink/`) using an installation secret
  - AI access: separate bearer tokens (`internal/ai/token.go`)
- Session key, CSRF key, and password hashes live inside the SQLite database in the volume — a redeploy neither rotates nor leaks them (`k8s/10-secret.example.yaml`)

## Monitoring & Observability

**Error Tracking:**
- None. No Sentry or equivalent.

**Logs:**
- `log/slog` to stdout; level from `HOLZCLOUD_LOG_LEVEL`. Collected by journald (systemd) or the cluster log pipeline (Kubernetes).

**Health:**
- `GET /healthz` - liveness (`cmd/holzcloud/main.go:610`)
- `GET /readyz` - readiness including free-disk floor (`cmd/holzcloud/main.go:616`)

## CI/CD & Deployment

**Hosting:**
- Raspberry Pi 5 via systemd behind Caddy (`deploy/holzcloud.service`, `deploy/Caddyfile.example`, `deploy/DEPLOY.md`)
- Kubernetes single-replica Deployment with shared Caddy (`k8s/20-app.yaml`, `k8s/30-caddy-shared.yaml`, `k8s/40-deployer-rbac.yaml`)

**Container Registry:**
- GitHub Container Registry (`ghcr.io/${{ github.repository }}`) — `.github/workflows/deploy.yml`

**CI Pipeline (GitHub Actions):**
- `.github/workflows/ci.yml` - gofmt check, `go mod tidy` diff check, `go vet`, build with version/commit ldflags, `go test ./...`, plus a `linux/arm64` cross-compile job
- `.github/workflows/deploy.yml` - triggered by a *successful* CI `workflow_run` on `main` (or manual dispatch); builds and pushes the image, then `kubectl rollout status`. Concurrency group `deploy-holzcloud`, `cancel-in-progress: false`.
- `.github/workflows/security.yml` - weekly (Sun 00:00) `go mod verify`, `go list -m -u all`, `go test -race` (the one place `CGO_ENABLED=1`), `go vet`

**Backups:**
- `deploy/backup.sh` + `holzcloud-backup.{service,timer}` — snapshot taken by the binary itself (`VACUUM INTO` through the pure-Go driver), integrity-checked before it counts as a backup. Optional `REMOTE_TARGET` rsync destination (e.g. a NAS), `RETENTION_DAYS` default 30. Restore via `deploy/restore.sh`.

## Environment Configuration

**Required env vars:**
- `HOLZCLOUD_DATA_DIR` - must be absolute in containers (`/data`); the relative default would resolve outside the volume
- `HOLZCLOUD_PORT` - default 8080
- `HOLZCLOUD_SECURE` - set true behind TLS so cookies get the Secure flag
- `HOLZCLOUD_TRUSTED_PROXIES` - CIDR list; controls whether forwarded client-IP headers are believed

**Optional (mail):** `HOLZCLOUD_SMTP_HOST`, `HOLZCLOUD_SMTP_PORT`, `HOLZCLOUD_SMTP_USER`, `HOLZCLOUD_SMTP_PASSWORD`, `HOLZCLOUD_SMTP_FROM`, `HOLZCLOUD_SMTP_FROM_NAME`, `HOLZCLOUD_SMTP_TLS`

**Secrets location:**
- Kubernetes `Secret` named `holzcloud-secrets` in namespace `holzcloud`, created from the command line (see `k8s/README.md`); `k8s/10-secret.example.yaml` is a template with placeholder values and is not applied.
- On the Pi: environment lines in `deploy/holzcloud.service`.
- No `.env` file exists in the repository.

## Webhooks & Callbacks

**Incoming:**
- `/ai` MCP endpoint (bearer-token, assistant-driven) — the only non-browser inbound API
- No payment, git, or third-party webhook receivers

**Outgoing:**
- None

## Plugins (sandboxed extensions, not external services)

WASM modules executed in-process by wazero (`internal/plugin/`), loaded from the data directory. Guests get WASI preview1, a 2-second call deadline, 16 MB of linear memory, and an 8 MB payload cap — they have no network access.

Shipped examples, each its own Go module producing `plugin.wasm` + `plugin.json`:
- `plugins/bestellung/` - orders/products (no payment provider integration; `verwaltung.go`, `produkte.go`)
- `plugins/kontaktformular/` - contact form with its own `migrations/`
- `plugins/suche/` - search
- `plugins/jahreszahl/` - year snippet
- `plugins/nicht-gefunden/` - 404 handling

Plugin authors depend only on `sdk/` (`github.com/holzcloud/holzcloud-cms/sdk`), which pulls in neither wazero nor the SQLite driver.

## Localization

Embedded JSON catalogs in `internal/i18n/locales/`: `de-CH`, `en`, `es`, `fr`, `fr-CH`, `it`, `it-CH`. Loaded from `embed.FS` with optional disk overrides (`internal/i18n/disk.go`); placeholder parity between catalogs is test-enforced (`internal/i18n/catalog_test.go`). No translation service is contacted.

---

*Integration audit: 2026-08-22*
