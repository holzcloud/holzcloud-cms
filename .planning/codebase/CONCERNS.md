<!-- refreshed: 2026-08-22 -->
# Codebase Concerns

**Analysis Date:** 2026-08-22

This audit reconciles with the two hand-written documents already in the repo:

- `docs/offene-punkte.md` — the feature/work backlog (missing field kinds, CSV
  import, static export, snippet field types, Dependabot PRs). Those items are
  **not repeated here**; they are wanted work, not defects.
- `docs/vergleich-statamic.md` — the capability comparison against Statamic.
  Deliberate non-goals listed there (GraphQL, OAuth, ⌘K palette, passkeys,
  third-party embeds, per-block HTML templates) are **not** concerns.

What follows is what those two documents do not cover: defects, risk, coverage
gaps, and operational limits found by reading the code.

**Baseline health:** `go vet ./...` clean, `go test ./...` fully green,
`go run ./tools/i18n` reports `0 offen, 0 verwaist` for all five full
translations. No `TODO`/`FIXME`/`HACK` markers anywhere in 255 Go files. The
codebase is in good shape; the concerns below are specific, not systemic.

---

## Tech Debt

**`holzcloud rerender` predates the block editor:**
- Issue: `cmdRerender` selects `id, website_id, slug, content_markdown, content_html`
  only. It has no knowledge of the `blocks` column and no knowledge of the
  block renderer.
- Files: `cmd/holzcloud/cli.go:309-382`, `internal/admin/page_blocks.go:134-146`,
  `internal/block/render.go:66`
- Impact: see *Known Bugs* below — this is the mechanism behind the highest-risk
  finding in this document.
- Fix approach: the command should branch the same way `blockContent` does —
  decode `blocks`, and for a block page re-render via `block.Render` rather than
  `page.RenderMarkdown`. Better still, extract one shared "derive html from a
  stored page" function used by both the save path and the CLI, so the two can
  no longer drift.

**Derived HTML is stored, and only one of three producers can be replayed:**
- Issue: `content_html` is persisted for pages (`internal/page/store.go:75`),
  for snippets (`internal/snippet/store.go:51`), and produced by the block
  renderer. A change to goldmark, to the bluemonday policy, or to a block kind's
  markup does not reach existing rows until something replays it. Only the
  page-markdown producer has a replay command.
- Files: `cmd/holzcloud/cli.go` (no snippet or block replay),
  `internal/snippet/store.go:100,115`, `internal/block/render.go`
- Impact: after a `goldmark`/`bluemonday` bump (PR #6 is open right now), older
  snippets and all block-rendered page HTML keep the old markup — including old
  sanitizer behaviour — indefinitely and invisibly.
- Fix approach: extend the replay command to cover snippets and blocks, and make
  `holzcloud check` report rows whose stored HTML differs from a fresh render.

**Admin handler package is the one large, thinly-tested surface:**
- Issue: `internal/admin` is 42 source files and the widest attack/regression
  surface in the program (auth-gated CRUD for every entity), with 5 test files.
- Files: `internal/admin/` — notably `page.go` (911 lines), `website.go` (590),
  `menu.go` (583), `user.go` (481), `twofactor.go` (457)
- Impact: covered under *Test Coverage Gaps*.
- Fix approach: the existing `internal/admin/page_handler_test.go` is a good
  harness pattern; reuse it per handler file rather than inventing a second one.

---

## Known Bugs

**`holzcloud rerender` destroys block-page content:**
- Symptoms: every page built with the block editor loses its rendered markup and
  is replaced with the markdown-rendered form of its plain-text projection —
  images, captions, videos, buttons and repeated groups become paragraphs of
  text. There is no undo; the write is a single committed transaction.
- Files: `cmd/holzcloud/cli.go:309-382` (select, render, `UPDATE pages SET
  content_html`), `internal/admin/page_blocks.go:134-146`
- Trigger: on save of a block page, `blockContent` stores
  `block.PlainText(blocks, set)` in `content_markdown` — a search/excerpt
  projection, *not* source markdown — and stores `block.Render(...)` output in
  `content_html`. `rerender` then reads `content_markdown`, runs
  `page.RenderMarkdown` over it, sees a difference for every block page, and
  overwrites `content_html`. Run `holzcloud rerender` on any installation with
  block pages to reproduce.
- Workaround: `holzcloud rerender -dry-run` first — a block page appearing in
  the "would change" list is the warning sign. Do not run the command without
  `-dry-run` until it is fixed. Recovery is a restore from `deploy/backup.sh`.
- Priority: **High.** This is a data-destroying command in the shipped CLI, it
  is listed in `holzcloud help` (`cmd/holzcloud/cli.go:38`) with no caveat, and
  it is not guarded by the destructive-action confirmation used elsewhere
  (`internal/admin/confirm.go`).

---

## Security Considerations

The security posture is deliberate and well implemented — Argon2id
(`internal/auth/password.go`), session rotation (`internal/auth/session.go`),
CSRF on state changes (`internal/auth/csrf.go`), a two-axis login throttle
(`internal/auth/ratelimit.go`), compulsory 2FA for administrators with a
shell-only recovery door (`cmd/holzcloud/cli.go:392+`), CSP `default-src 'self'`
(`internal/web/headers.go`), template-archive external-subresource rejection
(`internal/tmplmgr/external.go`), and a wazero sandbox with a call timeout,
memory cap and payload cap (`internal/plugin/runtime.go:24-33`). The following
are the residual edges.

**Sanitizer version is pinned into stored data:**
- Risk: `bluemonday` is applied once, at save, and the result is stored and
  later cast to `template.HTML` (`internal/template/loader.go:338`,
  `internal/snippet/store.go:170`, `internal/public/feed.go:99`). If a
  bluemonday advisory lands, upgrading the dependency does **not** re-clean
  anything already in the database.
- Files: `internal/page/store.go:471,601`, `internal/snippet/store.go:100,115`
- Current mitigation: CSP is a genuine second wall — a stored `<script>` still
  cannot execute under `default-src 'self'`.
- Recommendations: treat a bluemonday bump as requiring a data replay, and give
  that replay a command (see *Tech Debt*). Add a bluemonday version stamp to
  `holzcloud check` output so an operator can see whether stored HTML is stale.

**Auth throttling is per-process, in memory:**
- Risk: `LoginLimiter` keeps `map[string][]time.Time`
  (`internal/auth/ratelimit.go:68`). A restart clears every counter, so a
  crash-loop or a frequent redeploy resets an attacker's budget.
- Files: `internal/auth/ratelimit.go`
- Current mitigation: acceptable at the intended scale, and `replicas: 1` in
  `k8s/20-app.yaml:49` means there is no second process to disagree with.
- Recommendations: none required at current scale; record the constraint so
  nobody scales the deployment out without noticing (see *Scaling Limits*).

**MCP surface is a full write path into content:**
- Risk: `internal/ai` exposes tools that create and edit pages, authenticated by
  a bearer token (`internal/ai/token.go`). A leaked `hc_…` token is editor
  access to a website.
- Files: `internal/ai/token.go`, `internal/ai/mcp.go`, `internal/ai/tools.go` (755 lines)
- Current mitigation: strong — hashed storage, secret shown exactly once, per-token
  website scoping and a read-only flag enforced through one `Scope`
  (`internal/ai/token.go`), optional expiry, `MaxRequestBytes` cap, and all
  writes routed through the same stores, validation and revision history as the
  admin UI.
- Recommendations: the login throttle is not applied to token verification —
  add rate limiting to the MCP endpoint so a token cannot be brute-forced, and
  surface `LastUsedAt` prominently in the admin token list so an unexpected
  token becomes visible.

---

## Performance Bottlenecks

No measured bottleneck was found; the design consistently pre-computes the
expensive work. The two things worth watching:

**Plugin calls are serialised per plugin, inside the request:**
- Problem: one wasm instance has one linear memory, so calls into a given plugin
  take that plugin's mutex (`internal/plugin/runtime.go` — `instance.mu`).
  Concurrent visitors hitting a page with a plugin queue behind each other.
- Files: `internal/plugin/runtime.go`
- Cause: deliberate and correct; a wasm instance has no notion of two callers.
- Improvement path: a small pool of instances per plugin if a contact form ever
  becomes a hot path. `CallTimeout` (2s) bounds the worst case today.

**`internal/page/store.go` is the hot path and the largest file:**
- Problem: 1099 lines carrying a 30-column `pageColumns` list used by every read.
- Files: `internal/page/store.go:75`
- Cause: wide table, no ORM — a consequence of the stated conventions, not a
  mistake.
- Improvement path: leave it; note only that any new column widens every query
  in the program, so new per-page data belongs in `page_fields`, not in `pages`.

---

## Fragile Areas

**Block content round-trip:**
- Files: `internal/admin/page_blocks.go:134-146`, `internal/block/render.go`,
  `internal/block/encode` path, `cmd/holzcloud/cli.go:309`
- Why fragile: three representations of the same content (`blocks` JSON as
  source, `content_markdown` as a plain-text projection, `content_html` as
  output) with no single function that owns the derivation. The `rerender` bug
  above is exactly this fragility firing once.
- Safe modification: change block rendering only through `blockContent`, and
  add a test that asserts a block page's `content_markdown` is never treated as
  renderable source.
- Test coverage: `internal/block` at 63.5%; `internal/admin/page_blocks_test.go`
  exists but does not cover the CLI replay path at all.

**Migrations touching existing tables:**
- Files: `internal/db/migrations/` (38 files, through `00038_block_types.sql`);
  the precedents are `00029` and `00031`
- Why fragile: already documented in `docs/offene-punkte.md` — a table-level
  CHECK in SQLite can only be loosened by a full table rebuild, and `pages` has
  foreign-key children. This audit confirms it and adds nothing.
- Safe modification: read `00029` and `00031` before writing a migration that
  alters a table.

**`internal/admin/menu.go` and the import paths:**
- Files: `internal/admin/menu.go` (583 lines, `internal/menu` at 36.7% coverage),
  `internal/bundle/import.go` (727 lines), `internal/wxr`
- Why fragile: `docs/offene-punkte.md` records that this week's real defects —
  a post becoming a page on image insert, blocks missing from a bundle, menus
  colliding on import — were all found in the browser, not by tests. That is a
  coverage statement about exactly these files.
- Safe modification: exercise a real import against a real database after any
  change; the unit tests will not catch a collision.

---

## Scaling Limits

**Single writer, single replica:**
- Current capacity: one process, SQLite WAL, write pool at
  `SetMaxOpenConns(1)`; `k8s/20-app.yaml:49-51` pins `replicas: 1` with
  `strategy: Recreate` on a `ReadWriteOnce` PVC.
- Limit: horizontal scaling is not available and would corrupt state if
  attempted — a second replica means a second writer against one SQLite file,
  plus a second in-memory login throttle and a second plugin runtime.
- Scaling path: vertical only. The comment at `k8s/20-app.yaml:4` already says
  this; it is repeated here because the login throttle and the plugin instance
  cache are two more things that silently assume it.

**Media on the same volume as the database:**
- Current capacity: `data/media/<websiteID>/` alongside `data/holzcloud.sqlite`
  (`internal/media/crop.go:332`, `internal/media/variant_store.go:255`).
- Limit: the backup script guards against a half-written backup on a full disk
  (`deploy/backup.sh:35`), which is the right instinct — but media growth and
  database growth compete for one PVC (`k8s/20-app.yaml:14-18`).
- Scaling path: watch PVC headroom; media is the term that grows without bound.

---

## Dependencies at Risk

**Eight open Dependabot PRs — the list in `docs/offene-punkte.md` is stale:**
- Risk: that document names `#3`–`#8` (six PRs). The current set is
  **#3, #4, #5, #6, #8, #10, #11, #12** — three new ones have landed since it
  was written and `#7` and `#9` have gone. `docs/offene-punkte.md` §7 should be
  corrected to say eight.
- Impact: `golang.org/x/crypto 0.54.0 → 0.55.0` (#12) is the one with security
  weight; `modernc.org/sqlite 1.48.2 → 1.56.0` (#10) is now an eight-minor jump
  and is the one the open-items document rightly flags as needing a run against
  a real database file. `goldmark 1.8.2 → 1.8.5` (#6) touches stored HTML and
  should be paired with a replay (see *Tech Debt*).
- Migration plan: as `docs/offene-punkte.md` says — one at a time with
  `go test ./...` between. Add: after the goldmark bump, run
  `holzcloud rerender -dry-run` and read the diff rather than applying it, since
  the command is currently unsafe to apply.

**Go 1.26.2 in `go.mod` vs. Go 1.22+ in `CLAUDE.md`:**
- Risk: cosmetic drift — `go.mod` requires a far newer toolchain than the
  documented floor.
- Files: `go.mod:3`, `CLAUDE.md`
- Migration plan: update the stated constraint in `CLAUDE.md` to match `go.mod`.

---

## Missing Critical Features

Feature gaps are the subject of `docs/offene-punkte.md` and are not duplicated.
One operational gap that document does not cover:

**No integrity check for derived content:**
- Problem: `holzcloud check` verifies database integrity
  (`cmd/holzcloud/cli.go:251`) but nothing verifies that stored `content_html`
  still matches what the current renderer and sanitizer would produce.
- Blocks: safely upgrading goldmark or bluemonday, and detecting the damage the
  `rerender` bug would cause.

---

## Test Coverage Gaps

All tests pass; the gaps are about where the tests are not.

**`internal/admin` — 14.7%:**
- What's not tested: most of the authenticated CRUD surface. `website.go`,
  `menu.go`, `user.go`, `twofactor.go`, `field.go`, `kind.go`, `blocktype.go`,
  `bundle.go`, `wordpress.go` have no direct handler tests.
- Files: `internal/admin/` (42 source files, 5 test files)
- Risk: an authorisation or website-scoping mistake in a handler is caught only
  by a manual browser pass.
- Priority: **High** — this is the largest untested surface and the one behind
  the login.

**`internal/branding` — 0.0%:**
- Files: `internal/branding/`
- Risk: low in isolation, but it is the only package with no test at all.
- Priority: Low.

**`internal/kind` (28.9%), `internal/menu` (36.7%), `internal/field` (44.4%),
`internal/i18n` (44.5%), `internal/public` (44.7%), `internal/tmplmgr` (47.5%):**
- What's not tested: `kind` and `field` are the newest structural work (custom
  content types, sections, conditions, block types — migrations `00036`–`00038`)
  and carry the least coverage relative to their novelty. `menu` is named in
  `docs/offene-punkte.md` as the source of a real import defect. `tmplmgr` holds
  the zip-slip and external-subresource defences.
- Files: `internal/kind/`, `internal/menu/`, `internal/field/`, `internal/i18n/`,
  `internal/public/`, `internal/tmplmgr/`
- Risk: for `tmplmgr` specifically the untested half is security-relevant —
  `external.go` is one of the two places `CLAUDE.md` says must keep working.
- Priority: `tmplmgr` and `field` Medium-High; the rest Medium.

**CLI commands are untested end to end:**
- What's not tested: `cmd/holzcloud/cli.go` (544 lines) — `backup`, `migrate`,
  `compact`, `rerender`, `thumbnails`, `check`. Several write to the database.
- Files: `cmd/holzcloud/cli.go`
- Risk: demonstrated — the `rerender` defect is a CLI-only bug that no test
  could have caught because no test runs the command.
- Priority: **High** for the commands that write.

---

*Concerns audit: 2026-08-22*
