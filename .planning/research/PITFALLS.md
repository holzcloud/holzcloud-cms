# Domain Pitfalls

**Domain:** Self-hosted multi-site CMS (Go + htmx + SQLite, Raspberry Pi 5)
**Researched:** 2026-04-13

---

## Critical Pitfalls

Mistakes that cause data loss, security compromise, or rewrites.

---

### Pitfall 1: SQLite "database is locked" under Go's connection pool

**What goes wrong:** Go's `database/sql` opens multiple connections by default. SQLite only allows one writer at a time. Even with WAL mode enabled, two goroutines racing to write produce `SQLITE_BUSY`. The built-in `busy_timeout` PRAGMA does NOT fire if the contention is from within the same process sharing the same connection pool—the error is returned immediately.

**Why it happens:** `database/sql` assigns connections from a pool. Two concurrent HTTP handlers each grab a connection and both try to write. WAL removes reader/writer contention but not writer/writer contention. Transaction upgrades (begin read → write) are an especially common source: SQLite returns SQLITE_BUSY immediately, not after the timeout.

**Consequences:** 500 errors on concurrent admin saves. Impossible to reproduce in single-user testing.

**Prevention:**
- Enable WAL mode at DB open time: `PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;`
- Set `PRAGMA busy_timeout=5000` on every new connection via a connection hook (modernc.org/sqlite supports `sqlite.RegisterConnectionHook`)
- Create **two** `*sql.DB` instances: one for writes with `SetMaxOpenConns(1)`, one for reads with a higher limit
- Never upgrade a read transaction to write; always use `BEGIN IMMEDIATE` when you know you will write
- Avoid long-lived read transactions that prevent WAL checkpointing

**Detection:** Sporadic 500s with "database is locked" in logs under concurrent load. WAL file growing unboundedly (leaked rows from `*sql.Rows` not closed prevent checkpoint).

**Phase:** Core data layer (Phase 1 / Foundation).

---

### Pitfall 2: modernc.org/sqlite connection hook pragmas not applied to all connections

**What goes wrong:** PRAGMA settings (WAL, busy_timeout, foreign_keys, cache_size) applied via a direct `db.Exec("PRAGMA ...")` call only affect the connection currently checked out. Other connections in the pool never see those settings.

**Why it happens:** `database/sql` is connection-pool-agnostic. A `db.Exec` call may get a different connection next time.

**Consequences:** Some requests run without WAL, without foreign key enforcement, without busy timeout—silently.

**Prevention:** Use `modernc.org/sqlite`'s `sqlite.RegisterConnectionHook` (or the DSN `?_pragma=...` query parameters supported by modernc v1.34+) to inject pragmas at connection-open time. Verify via `PRAGMA journal_mode` query after boot.

**Detection:** Run `PRAGMA journal_mode` in a loop from multiple goroutines; some return `delete` instead of `wal`.

**Phase:** Core data layer (Phase 1 / Foundation).

---

### Pitfall 3: Inserting Markdown-rendered HTML via `template.HTML` without sanitization

**What goes wrong:** Rendering user-authored Markdown through a library (e.g. `goldmark`) produces HTML. Injecting that HTML into a `html/template` template using `template.HTML(renderedString)` bypasses Go's auto-escape entirely. Any `<script>` tag or `javascript:` URL in the original Markdown becomes live XSS in the public page.

**Why it happens:** Developers correctly notice that auto-escape breaks rendered HTML, then cast to `template.HTML` to "fix" it—without realizing the cast is a trust declaration.

**Consequences:** Stored XSS on the public site. Any admin who can author pages can inject scripts visible to all readers.

**Prevention:**
1. Render Markdown → raw HTML string
2. Pass that string through `bluemonday` with a strict allow-list policy (`bluemonday.UGCPolicy()` as baseline, tighten per requirements)
3. Only then cast to `template.HTML` and pass to the template
4. Never allow `style` elements in the bluemonday policy — bluemonday has no CSS sanitizer; style content passes through unsanitized (this was CVE-2021-42576)
5. Run bluemonday **after** every Markdown processing step, never before

**Detection:** Author a page containing `<script>alert(1)</script>` in Markdown. If it fires on the public site, sanitization is missing or mis-ordered.

**Phase:** Page authoring / content rendering (Phase 3 or wherever Markdown is introduced).

---

### Pitfall 4: CSRF tokens not sent with htmx AJAX requests

**What goes wrong:** Traditional synchronised-token CSRF protection embeds a token in `<form>` hidden inputs. htmx AJAX requests (`hx-post`, `hx-put`, `hx-delete`) bypass normal form submission, so the token is never sent unless explicitly wired up.

**Why it happens:** Developers add CSRF middleware and test with full-page forms — tests pass. htmx partial requests are never tested. Middleware sees no token and either silently skips validation or rejects every htmx request.

**Consequences:** Either all htmx state-changing requests are rejected (functional breakage) or CSRF protection is disabled for them (security hole).

**Prevention:**
- Embed the CSRF token in an `hx-headers` attribute on a root element (e.g. `<body hx-headers='{"X-CSRF-Token": "{{ .CSRFToken }}"}'>`). htmx inherits headers from ancestor elements.
- Validate the `X-CSRF-Token` request header on the server for all non-GET requests, alongside the form field fallback for full-page posts.
- SameSite=Lax cookies alone are not sufficient when the admin is served over HTTP during development or via non-standard ports.

**Detection:** Open browser dev tools, trigger an htmx action, inspect request headers. If `X-CSRF-Token` is absent, CSRF is not being sent.

**Phase:** Auth / admin UI scaffolding (Phase 2).

---

### Pitfall 5: Host header spoofing breaks multi-site routing

**What goes wrong:** The router resolves which website to serve by reading the `Host` request header. An attacker sends an arbitrary `Host` value to a CMS endpoint. If the lookup returns nothing, the server might leak a default site or error in a way that reveals internal structure. If the lookup succeeds (e.g. the attacker knows a real domain), they can probe admin routes cross-domain.

**Why it happens:** Reverse proxies forward the original `Host` header unchanged unless explicitly overriding it. Without a trusted-proxy allowlist, forged headers from direct connections reach the application.

**Consequences:** Site bleed (serving site A's content at site B's URL), information leakage, potential authentication confusion across sites.

**Prevention:**
- In production, Caddy / Nginx should strip and rewrite `Host`. Document this requirement explicitly.
- In the Go handler, validate the resolved host against the known `website_domains` table. Unknown hosts → 404, never a default.
- Normalize host to lowercase and strip the port before lookup; `Example.com:443` and `example.com` must resolve to the same site.
- Trusted-proxy list for `X-Forwarded-Host`: only accept that header from `127.0.0.1` / the Caddy socket address.

**Detection:** `curl -H "Host: arbitrary.invalid" http://localhost:8080/` should return 404, not a valid page.

**Phase:** Multi-site routing / public serving (Phase 1 or 2).

---

### Pitfall 6: Zip-slip in template archive extraction

**What goes wrong:** A user uploads a `.zip` template package. The extraction code opens each entry and writes it to `templates/<website>/<name>/`. A crafted archive contains a path like `../../public/index.go`, overwriting source files or sensitive data.

**Why it happens:** Using `filepath.Join(destDir, entry.Name)` does not prevent traversal if `entry.Name` contains `../` components that survive the join.

**Consequences:** Arbitrary file write with the privileges of the Go process — potentially overwriting the binary, the SQLite database, or adding executable files served as static assets.

**Prevention:**
- After `filepath.Join`, call `filepath.Clean` and verify the result starts with the absolute `destDir` path + `/`: `strings.HasPrefix(resolved, destDir+string(os.PathSeparator))`
- Reject any archive entry whose cleaned path escapes the destination
- Enforce a total extracted-size limit (disk exhaustion) and a per-file extension allow-list (no `.go`, `.sh`, `.php` etc.)
- Run extraction as the CMS process user, not root; restrict write permissions on the binary directory

**Detection:** Upload an archive containing `../../evil.txt` as an entry name; verify the file is NOT created outside the template directory.

**Phase:** Template upload feature (Phase 3 or 4).

---

### Pitfall 7: Draft pages leaking to the public site

**What goes wrong:** The public rendering query fetches pages by slug without filtering on `status = 'published'`. Draft pages intended only for preview become publicly accessible via their slug.

**Why it happens:** Straightforward `WHERE slug = ?` query; status filter added as a post-launch "improvement" that was never added.

**Consequences:** Unpublished content (announcements, sensitive copy, placeholder text) visible to all readers.

**Prevention:**
- Include `AND status = 'published'` in every public-facing page query from day one
- Write a test that inserts a draft page and asserts a GET on its slug returns 404
- Admin preview routes must require a session (authenticated only)

**Detection:** Create a page in Draft status; request its slug on the public site without a session.

**Phase:** Public rendering (Phase 2–3).

---

## Moderate Pitfalls

---

### Pitfall 8: Session fixation — no session ID rotation on login

**What goes wrong:** The session ID assigned before login (e.g. to store flash messages) remains the same after successful authentication. An attacker who pre-sets a known session ID (via network sniffing or XSS) hijacks the newly authenticated session.

**Prevention:** Regenerate the session ID immediately after `password.CompareHashAndPassword` succeeds. In practice: delete the old session record, create a new one with a new random token, set a fresh cookie.

**Phase:** Auth (Phase 2).

---

### Pitfall 9: bcrypt cost too high causes login timeout on Pi

**What goes wrong:** A bcrypt cost of 14+ on a Raspberry Pi 5 (ARM Cortex-A76 cores) takes several seconds per hash. Under even modest concurrent login attempts, the HTTP server goroutines pile up waiting for CPU. At cost 15+, a single login can block for 4–8 seconds, triggering client timeouts.

**Why it happens:** Default cost 10 is fine on x86 developer machines; cost scales to the dev machine, not the deployment target.

**Consequences:** Login timeouts, poor admin UX, accidental DoS from multiple concurrent login attempts.

**Prevention:**
- Benchmark at build time or startup: `bcrypt.GenerateFromPassword([]byte("benchmark"), cost)` and log the duration
- Target 200–500ms per hash on the Pi; cost 11–12 is a reasonable starting point for Pi 5 (ARM64)
- Make cost a configurable environment variable so it can be tuned per-deployment
- Do NOT go below cost 10 for security reasons

**Phase:** Auth (Phase 2). Flag for Pi hardware benchmarking.

---

### Pitfall 10: `HX-Request` header absent from cached responses — Vary header missing

**What goes wrong:** A reverse proxy or browser caches a full-page response served when `HX-Request` is absent. A subsequent htmx AJAX call hits the cache and receives a full HTML page instead of a partial fragment, inserting an entire `<html>` document inside a `<div>`.

**Prevention:** Any handler that returns different content based on `HX-Request` must add `Vary: HX-Request` to the response. Caddy and Nginx will then cache separately per header value.

**Phase:** Admin UI / any htmx handler (Phase 2+).

---

### Pitfall 11: Double-submit on slow Pi — htmx repeated clicks

**What goes wrong:** A form submission triggers an htmx request that takes 500ms+ to respond (DB write, template render). The user clicks again. Two identical POST requests are in flight. Both succeed; duplicate records are created or the second clobbers the first.

**Prevention:**
- Add `hx-disabled-elt="this"` (or `hx-indicator`) so the submit element is disabled while the request is in flight
- Use database unique constraints as the last line of defense (e.g. unique slug per website)
- Idempotency: edit/update operations are naturally idempotent if keyed by ID; new-record operations need either the UI lock or a uniqueness check

**Phase:** Admin UI forms (Phase 2+).

---

### Pitfall 12: Cross-compile failure when using `mattn/go-sqlite3` (CGO dependency)

**What goes wrong:** `mattn/go-sqlite3` requires CGO. Cross-compiling from macOS/Linux x86-64 to `GOARCH=arm64 GOOS=linux` with CGO enabled requires an ARM64 cross-compiler (`aarch64-linux-gnu-gcc`) installed on the build machine. Without it, `go build` fails with a confusing "no C compiler" error.

**Prevention:**
- Use `modernc.org/sqlite` (pure Go, no CGO) for unconditional cross-compile support with `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build`
- If `mattn` is ever chosen, document the cross-compiler toolchain requirement in the Makefile
- CI must build for `linux/arm64` explicitly to catch this before deployment

**Detection:** `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...` fails with a C compiler error.

**Phase:** Build / CI setup (Phase 1).

---

### Pitfall 13: File upload serving executable content or leaking paths

**What goes wrong:** Template or asset uploads are stored on disk. The static file handler serves any file under the upload directory. An attacker uploads a `.php` (or even `.go`) file and tricks a misconfigured reverse proxy into executing it.

**Prevention:**
- Serve uploaded files via a Go handler that sets `Content-Disposition: attachment` or an explicit safe `Content-Type`, not via Nginx `try_files`
- Set `X-Content-Type-Options: nosniff` on all responses
- Validate uploaded file content using `http.DetectContentType` on the first 512 bytes; reject anything not in the allow-list
- Store uploads outside the web root; never expose the storage directory directly

**Phase:** Template upload / asset handling (Phase 3–4).

---

### Pitfall 14: WAL file growing unboundedly — leaked `*sql.Rows`

**What goes wrong:** An open `*sql.Rows` object holds a read transaction. SQLite cannot checkpoint (flush WAL to main db file) while any reader is open. A handler that returns early without calling `rows.Close()` (or forgets `defer rows.Close()`) leaks the transaction indefinitely.

**Consequences:** WAL file grows without bound; disk fills on the Pi; eventual read-only failure.

**Prevention:**
- Always `defer rows.Close()` immediately after `db.Query`
- Consider a wrapper that enforces close-on-return for query helpers
- Monitor WAL file size in production; alert if > 10MB

**Phase:** Core data layer (Phase 1).

---

### Pitfall 15: Reverse proxy real-IP confusion — rate limiting and audit logs record proxy IP

**What goes wrong:** The Go server logs `r.RemoteAddr`, which is the Caddy process address (`127.0.0.1`) not the real client IP. Rate limiting keyed on `r.RemoteAddr` limits everyone equally (the loopback address). Audit logs are meaningless.

**Prevention:**
- Read the real IP from `X-Forwarded-For` or `X-Real-IP`, but only when `r.RemoteAddr` is a trusted proxy address (127.0.0.1 or the Caddy Unix socket peer)
- Never trust forwarded IP headers from non-proxy connections; validate `RemoteAddr` first
- Caddy can be configured to set `X-Real-IP` to the actual client address; prefer that over the multi-value `X-Forwarded-For`

**Phase:** Deployment / reverse proxy integration (Phase 1 or final).

---

## Minor Pitfalls

---

### Pitfall 16: `embed.FS` vs external template files — dev vs production divergence

**What goes wrong:** Using `//go:embed templates/` bakes templates into the binary at compile time. During development, editing a template requires recompiling the binary. If the production binary and the on-disk templates diverge, restarts pick up the embedded (stale) version.

**Prevention:**
- Support a `--templates-dir` flag that, when set, serves templates from disk (dev mode); fall back to `embed.FS` when the flag is absent (production mode)
- Make the selection explicit in startup logs: "templates: embedded" vs "templates: /path/to/dir"

**Phase:** Build / template system (Phase 1–2).

---

### Pitfall 17: hx-boost Safari history pushState bug

**What goes wrong:** Safari on iOS skips history state entries created by `history.pushState()` if the call happens more than ~500ms after the last user interaction. htmx's `hx-boost` historically called `pushState` after receiving the response, which exceeds the threshold on slow connections. Navigation back/forward skips entries.

**Prevention:**
- Use htmx 1.9.10+ (fixed by moving pushState before response receipt) or htmx 2.x
- Test navigation on Safari iOS explicitly, not just desktop Chrome/Firefox

**Phase:** Admin UI / htmx setup (Phase 2).

---

### Pitfall 18: Graceful shutdown not draining in-flight requests

**What goes wrong:** `systemd` sends SIGTERM on `systemctl restart`. The Go binary exits immediately, dropping in-flight requests (ongoing form submissions, template renders). Users lose data mid-save.

**Prevention:**
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()
// start server in goroutine
<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```
Set `TimeoutStopSec=15` in the systemd unit file to give the process time to drain.

**Phase:** Deployment / binary packaging (Phase 1 or final).

---

### Pitfall 19: Public site template caching — stale content after publish

**What goes wrong:** Go `html/template` templates are parsed at startup and cached. If the admin publishes a page or edits a template, the cache holds the old version until restart.

**Prevention:**
- For pages (content stored in DB), no template caching issue; content is always fetched fresh from DB
- For template _files_ (`.html` layout files for a website), parse once at startup (or on template activation) and re-parse on template change events
- Consider an in-memory cache keyed by `(websiteID, templateVersion)` invalidated on admin template update; this avoids per-request disk reads

**Phase:** Template system / public rendering (Phase 2–3).

---

### Pitfall 20: SQLite on SD card / Pi — fsync and durability

**What goes wrong:** Raspberry Pi SD cards have unreliable fsync (the OS acknowledges writes before the card physically flushes). A power loss between WAL write and checkpoint can corrupt the database on some SD card + kernel combinations.

**Prevention:**
- Strongly recommend running SQLite on a USB SSD or NVMe HAT rather than the SD card
- Document this recommendation prominently in deployment notes
- With `PRAGMA synchronous=NORMAL` (recommended for WAL), the WAL itself is durable; the risk is losing the last checkpoint. For a CMS this is acceptable.
- Enable `PRAGMA wal_autocheckpoint=1000` (default) and keep write transactions short to bound potential data loss

**Phase:** Deployment notes (any phase; document in Phase 1 architecture).

---

### Pitfall 21: Accessible focus management after htmx swaps

**What goes wrong:** htmx replaces a chunk of the DOM. Screen readers and keyboard users lose focus context. The browser moves focus to `<body>` or nowhere. A form that was replaced with a success message is unreachable via keyboard without page reload.

**Prevention:**
- Use `hx-swap="outerHTML"` only when the replaced element can be given focus after swap
- Add `autofocus` to the first meaningful element in returned partials, or use `htmx:afterSwap` event to call `element.focus()`
- Test with VoiceOver / NVDA after every htmx interaction pattern introduced

**Phase:** Admin UI / htmx setup (Phase 2).

---

## Phase-Specific Warning Matrix

| Phase | Topic | Likely Pitfall | Mitigation |
|-------|-------|---------------|------------|
| 1 — Foundation | SQLite setup | SQLITE_BUSY without WAL + single-writer pool | Two DB pools, WAL pragma, busy_timeout |
| 1 — Foundation | Build / CI | CGO cross-compile failure | modernc.org/sqlite, CGO_ENABLED=0 |
| 1 — Foundation | Host resolution | Host header spoofing | Strict domain table lookup, 404 on unknown |
| 2 — Auth + Admin UI | Login | Session fixation, bcrypt timeout on Pi | Rotate session ID on login; benchmark cost |
| 2 — Auth + Admin UI | CSRF | htmx requests missing token | hx-headers on root element, server-side header check |
| 2 — Auth + Admin UI | htmx | Double-submit, missing Vary header | hx-disabled-elt, Vary: HX-Request |
| 3 — Content / Pages | XSS | template.HTML without bluemonday | Sanitize before trust-casting |
| 3 — Content / Pages | Drafts | Draft page leakage | status='published' filter in all public queries |
| 4 — Templates | Zip-slip | Archive path traversal | Clean + prefix check on every extracted path |
| 4 — Templates | MIME | Executable file served as asset | http.DetectContentType allow-list, nosniff header |
| Any — Deploy | IP logging | Proxy IP recorded instead of client | Trusted-proxy real-IP extraction |
| Any — Deploy | Shutdown | In-flight requests dropped | signal.NotifyContext + server.Shutdown |
| Any — Deploy | SD card | fsync unreliable | Document USB SSD recommendation |

---

## Sources

- SQLite WAL concurrency: https://sqlite.org/wal.html
- SQLITE_BUSY and transaction upgrades: https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/
- SQLite busy_timeout edge cases: https://berthub.eu/articles/posts/a-brief-post-on-sqlite3-database-locked-despite-timeout/
- modernc.org/sqlite Go setup: https://theitsolutions.io/blog/modernc.org-sqlite-with-go
- Go SQLite WAL + pool patterns: https://turriate.com/articles/making-sqlite-faster-in-go
- Leaked rows prevent checkpoint: https://turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang-72547ad625f1
- modernc vs mattn benchmark: https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html
- bluemonday XSS sanitization: https://github.com/microcosm-cc/bluemonday
- bluemonday style tag CVE: https://github.com/microcosm-cc/bluemonday/commit/c788a2a4d42e081ad54a31368478820bb4a42fb4
- Go template.HTML XSS bypass: https://www.sourcery.ai/vulnerabilities/go-template-html-vulnerable
- htmx CSRF patterns: https://htmx.org/essays/web-security-basics-with-htmx/
- CSRF prevention in Go: https://www.alexedwards.net/blog/preventing-csrf-in-go
- Zip-slip vulnerability: https://security.snyk.io/research/zip-slip-vulnerability
- mholt/archiver zip-slip CVE-2025-3445: https://github.com/advisories/GHSA-7vpp-9cxj-q8gv
- Go path traversal prevention: https://deepsource.com/directory/go/issues/GSC-G305
- OWASP session management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- bcrypt on small devices: https://github.com/photoprism/photoprism/issues/3718
- bcrypt cost factor guide: https://dev.to/nesniv/understanding-bcrypts-work-factor-and-choosing-the-right-value-103m
- htmx hx-boost Safari bug: https://github.com/bigskysoftware/htmx/pull/1087
- htmx HX-Request detection Go: https://pkg.go.dev/github.com/angelofallars/htmx-go
- X-Forwarded-For spoofing: https://httptoolkit.com/blog/what-is-x-forwarded-for/
- Go graceful shutdown: https://victoriametrics.com/blog/go-graceful-shutdown/
- MIME sniffing Go: https://destel.dev/blog/on-the-fly-content-type-detection-in-go
