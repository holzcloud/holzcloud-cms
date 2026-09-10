# Holzcloud CMS — Project Context

**Created:** 2026-04-13 (autonomous initialization after technical pivot)
**Status:** v1.0 shipped 2026-04-14; v1.6 "Inhaltsmodell und Zugang" completed 2026-09-10. Software releases are tagged separately and have reached v1.9.
**Current milestone:** none active — next is v2.0 "The Codebase Speaks English" (Phase 12, not planned; opened with `/gsd-new-milestone`)
**Stack (HARD MANDATE):** Go + htmx + CSS + SQLite

## Vision

A minimal, self-hosted CMS that runs as a single Go binary on a small linux/amd64 server. Manages **multiple websites**, each with **multiple domains**, serving server-rendered HTML to the public and exposing a simple admin UI (htmx-driven) for authoring templates, menus, and pages.

## Core Value

One small binary runs several websites without dependency soup. Authors work through a clean, responsive admin UI; readers get fast server-rendered pages.

Checked at the v1.6 close and unchanged: every v1.6 feature was built inside that promise — single sign-on without an OIDC client or a runtime call, a gallery without JavaScript, a CSV import without a new dependency.

## Current State (after v1.6)

- **Code:** 110 399 lines of Go in 394 files; 53 migrations; eight shipped public themes; six wasm plugin guests rebuilt and compared in CI.
- **Admin languages:** de, en, es, fr, it with Swiss variants; the catalogue gate reads 1328 strings, 0 offen, 0 verwaist.
- **Delivered in v1.6:** the full field palette in every carrier (pages, snippets, own block kinds), CSV import, single sign-on through Authentik forward-auth, and a gallery with albums, lightbox and slideshow. Details in `.planning/MILESTONES.md`.
- **Known debt:** the milestone audit closed at `tech_debt`, 46/48 (`.planning/milestones/v1.6-MILESTONE-AUDIT.md`). `.planning/WINDOWS.md` holds 22 open of 29 entries — mostly Phase 10: counting gates that measure something other than their name, SSO follow-ups (no deprovisioning, rights rows without an actor, refusals not rate-limited), and German literals the gate cannot see.
- **Release vs. milestone numbers:** the planning milestone called v1.6 does not correspond to the release tag `v1.6`. They diverged at the v1.5 renumbering; no tag was created at this close.

## Next Milestone Goals (v2.0 — The Codebase Speaks English)

Not yet planned. What is already decided, and where it is written:

- **LANG-01 … LANG-08** — comments, identifiers, test names, SQL columns and catalogue keys in English; the template data contract in English as a deliberate breaking change (hence 2.0); a gate that keeps German out of Go source. Full text in `.planning/milestones/v1.6-REQUIREMENTS.md`; the phase and its nine success criteria in `.planning/ROADMAP.md`.
- **Carried from v1.6:** the 828 operator-facing strings no catalogue can see (`.planning/audits/v1.6-I18N-828.md`), the sentences built past the gate (`WINDOWS.md` 6, 18, 29), and GAL-07's wording.
- **Not decided yet:** whether the stored German vocabularies (field kinds, block kinds, `gilt_fuer`) turn too — LANG-08 requires the decision, not a particular answer.

<details>
<summary>Previous milestone framing — v1.6 Inhaltsmodell und Zugang, as written on 2026-09-03</summary>

**Goal:** A website describes its content model completely — every field kind an author
needs, in every carrier that holds fields, and content can also arrive as a table — and
whoever enters the admin may arrive through the sign-in the operator already runs.

**Target features:**
- The housekeeping that blocks the rest: the i18n writer's file format, a test suite that
  skips itself in silence, and planning notes that have gone stale
- Choice as a button row, and a genuine multiple choice (the first field value that is not a single string)
- Terms as a field kind, so a field can pick from the tags a page already carries
- The three small field kinds still missing: `zeit`, `bereich`, `code`
- Text snippets that carry every field kind, not only Markdown
- CSV import beside the WordPress (WXR) and bundle importers
- Single sign-on against a self-hosted Authentik, taken as a forward-auth header from the
  reverse proxy — explicitly not an OIDC client inside the binary

Most of these come from `docs/offene-punkte.md`, which states for each one what is missing,
where it belongs, and how big it is. That file stays the source of truth; this milestone is
the subset that was scoped in, plus the Authentik work, which is new.

**Why 1.6 and not 1.5.** The tags `v1.4` and `v1.5` are released and pushed, and
CHANGELOG.md carries `## 1.5 — 2026-09-03`. The planned milestone that used to be called
v1.5 never left 0 %, so it was renumbered rather than shipped under a number that already
means something else. No v1.5 milestone shell remains — its three phases moved into this
one as Phases 7, 8 and 9.

A gallery (Phase 11) was added during the milestone and shipped in it; the English
codebase (Phase 12) was added and then moved to v2.0 on 2026-09-08.

</details>

## Requirements

### Validated

Shipped and in use. v1.0 delivered items 1–7 below; the items without a version were built between
April and September 2026 outside the planning artefacts, which is why this section exists.

- ✓ Multi-website / multi-domain routing keyed off request Host — v1.0
- ✓ Admin UI: login, websites, domains, users, pages, menus, media — v1.0
- ✓ Per-website templates, uploaded as archives and checked before they are accepted — v1.0
- ✓ Page authoring in Markdown, draft/published, slug routing — v1.0
- ✓ Hierarchical menu builder per website — v1.0
- ✓ Auth: Argon2id, SCS sessions in SQLite, CSRF including htmx, role gates — v1.0
- ✓ Public rendering with `Cache-Control`, `ETag`, `Last-Modified` — v1.0
- ✓ Five interface languages (de, en, es, fr, it) with Swiss variants; per-website locales
- ✓ Revisions: history, diff with context lines, labels
- ✓ Activity log: who changed what, when, with filters
- ✓ Custom fields, field groups, sections and conditions — per website
- ✓ Custom content kinds and custom block kinds — per website
- ✓ Terms (tags) on pages
- ✓ Blocks: structured content elements rendered to HTML on save
- ✓ Text snippets with a Markdown body
- ✓ Shop: catalogue, orders, Swiss tax, invoice and prepayment, Payrexx as an addition
- ✓ Outbox: e-mail held until it has actually been sent
- ✓ Plugin system: WebAssembly guests via wazero, with an SDK (`sdk/`) and five plugins
- ✓ Import and export: WordPress WXR, and the project's own bundles
- ✓ Share links: signed URLs that show an unpublished page
- ✓ TOTP as a second factor
- ✓ Per-user rights: which websites, and publish yes/no
- ✓ Design tokens and branding per installation
- ✓ schema.org JSON-LD for search engines
- ✓ Background jobs, media variants, video from the site's own library
- ✓ Deployment: systemd unit, Caddy config, backup procedure
- ✓ Housekeeping gates: catalogue format locked by a test, CI rebuilds and compares the wasm guests and plugin archives, self-skipping tests fail on a runner — v1.6 (MAINT-01…05)
- ✓ Choice as a button row with an explicit empty choice; a field condition still works against it — v1.6 (FIELD-01, FIELD-08)
- ✓ Multiple choice with a server-side maximum, and one multi-value encoding shared by form, renderer, bundle and CSV importer — v1.6 (FIELD-02, FIELD-07)
- ✓ Term field: stores the slug, prints the current name — v1.6 (FIELD-03)
- ✓ Time, range and code field kinds, the code field escaped also inside a block — v1.6 (FIELD-04…06)
- ✓ Own block kinds hold their field values to the same check as a page — v1.6 (closed at the milestone audit)
- ✓ Text snippets carry every field kind, through the existing field table and the same sanitisation — v1.6 (SNIP-01…05)
- ✓ CSV import: upload, column mapping, dry run, new or existing website, a report for every row — v1.6 (IMP-01…10)
- ✓ Single sign-on through Authentik forward-auth: trusted peer and shared secret before any header is read, group-mapped roles and websites re-read on every request, identity bound by username and never by e-mail address, the password path unchanged — v1.6 (SSO-01…11)
- ✓ Gallery: lightbox without JavaScript, previous/next, albums per website placed on many pages, scroll-snap slideshow, album references surviving the bundle round trip — v1.6 (GAL-01…06; GAL-07 met in substance)

### Active

Nothing is being built. The next milestone's requirements are listed under *Next Milestone Goals* and become active when `/gsd-new-milestone` writes them into a fresh `.planning/REQUIREMENTS.md`.

### Out of Scope

Beyond the v1 exclusions further down, the following were considered and left out. Every reason was re-read at the v1.6 close and still holds.

- **Static export** — a second mode of operation beside the one that works: it can serve
  neither forms, nor search, nor protected pages. If it is ever built, it must be an
  explicitly reduced output, not a second front door. Stays on `docs/offene-punkte.md`.
- **Authentik as an OIDC/OAuth client inside the binary** — `docs/offene-punkte.md` lists
  OAuth under what is deliberately not built, for the reason that holds here too: it is a
  second mode of operation beside the one that works, and it needs a dependency and an
  outbound call at runtime. The forward-auth header from the reverse proxy delivered the
  same single sign-on in v1.6 with neither.
- **Verifying `X-authentik-jwt`** — verification means fetching keys at runtime or pinning
  one; it protects nothing the trust boundary does not already protect, and parsing without
  verifying would look like protection.
- **CSV "delete rows missing from the file"** — one mis-mapped key column deletes a website.
- **Downloading images named by URL in a CSV** — nothing is fetched from a third party at
  runtime; the importer lists such URLs instead.
- **A live JavaScript readout beside the `bereich` input** — the project rejects exactly that
  pattern in uploaded templates and does not build it into its own admin.
- **Masonry or justified gallery layouts** — deliberately deselected when the gallery was
  scoped; the grid, the lightbox and the slideshow were built instead.

## Key Decisions

Decisions taken during v1.6 that a later reader should not reopen without a reason.

| Decision | Why | Outcome |
|----------|-----|---------|
| A multi-valued field is stored one value per line in the existing string slot; a CSV cell separates values with a pipe | The newline is already illegal inside a configured choice; a pipe is visible in a spreadsheet | ✓ Good — one pair of functions, no second spelling appeared |
| A term field stores the slug and prints the name; `KindTerm` is excluded from block kinds | The slug survives a rename; a block freezes to HTML on save and cannot follow one | ✓ Good |
| Single sign-on as a forward-auth header from the reverse proxy, not an OIDC client | No dependency, no client secret in the binary's configuration, no outbound call | ✓ Good — shipped with peer check, strip and constant-time secret |
| An Authentik session satisfies the second factor | The operator's decision (2026-09-03); the dependency on Authentik enforcing one is written into `deploy/DEPLOY.md` | ⚠️ Revisit if the identity provider ever changes |
| An SSO identity is bound to an account by username, never by e-mail address | An address that belongs to another account must not hand over that account | ✓ Good — refused as `not_linked`, linked explicitly by CLI |
| Whether an editor is limited to websites is stored explicitly (`websites_limited`) | "No assignment means every website" turned a deleted website into access to all of them | ✓ Good |
| The lightbox uses `:target`, the slideshow CSS scroll-snap | htmx only, and a public page must work with no script at all | ✓ Good — survives a blocked stylesheet and a disabled script engine |
| An album reference travels through a bundle by name; the address is re-derived on import | The same lesson Phase 7 learned on terms | ✓ Good |
| Phase 12 moved to v2.0; Phase 11 stayed in v1.6 | Phase 12 breaks the public template contract; Phase 11 had shipped | ✓ Good |
| No git tag at the v1.6 close | `v1.6` already names a published release; a published tag is never moved | — Pending: decide how planning milestones and release tags relate before v2.0 |

## Mandatory Stack (unchangeable)

- **Backend:** Go (go.mod is at 1.26.6). Standard library first. Third-party libs only when clearly justified (SQLite driver, htmx-less validation helpers, password hashing via `golang.org/x/crypto`, etc.).
- **Persistence:** SQLite via `modernc.org/sqlite` — pure Go, `CGO_ENABLED=0`. Settled, not a preference: the cgo driver is not a fallback.
- **Frontend interactivity:** htmx only. No other JavaScript, no frameworks, no npm.
- **Styling:** plain CSS (custom properties + `@layer` allowed). No Tailwind, Sass, PostCSS, or bundlers.
- **Templating:** Go `html/template` for all server-rendered HTML.
- **Build:** `go build` → single binary. No Docker-by-default. No external build steps.

Anything outside this stack requires explicit user approval.

## Deployment Target

- Single binary on a small linux/amd64 server. (Retargeted from arm64/Pi on 2026-09-03;
  CI builds linux/amd64 and the release workflow publishes it on a `v*` tag.)
- Reverse proxy (Caddy or similar) in front for TLS + HTTP/2; binary listens on localhost:PORT.
  Since v1.6 the same proxy may carry single sign-on (Caddy ≥ 2.11.2 with Authentik's outpost).
- SQLite file + asset directory on local disk; no external services required.

## Key Capabilities (high-level)

1. **Multi-website / multi-domain routing:** one binary serves many sites keyed off request Host.
2. **Admin UI:** login, manage websites, domains, users, templates, pages, menus. htmx-driven editing (inline edits, pagination, form validation) with full-page fallback.
3. **Template system:** per-website templates (Go templates packaged in a directory) with partial overrides.
4. **Page authoring:** create / edit / publish pages with content (Markdown or safe HTML), slug routing, draft/published state.
5. **Menu builder:** hierarchical menus attached to websites.
6. **Auth + security:** session cookies, CSRF, password hashing, role-based gates, secure defaults.
7. **Public site rendering:** fast, cacheable server-rendered pages per website+domain.

## Project Constraints

- Runs on a small server — keep memory + CPU footprint small; prefer synchronous request handling; avoid heavy dependencies.
- Server-rendered first; htmx adds interactivity, never SPA state management.
- Security baseline: hashed passwords, CSRF on all state-changing requests, secure session cookies, role gates for admin endpoints.
- Single binary deployment + simple `data/` directory for SQLite + uploads.
- No build tools: CSS is handwritten; JS is one htmx `<script>` include; Go compiles with `go build`.

## Out of Scope (v1)

- SPA features (React/Vue/Svelte).
- External databases (Postgres, MySQL, Redis).
- Docker as primary deployment (allowed as a secondary option later).
- Build tooling, bundlers, transpilation.
- A visual drag-and-drop page builder (too JS-heavy).
- Rich-text WYSIWYG editors (use Markdown + server-side render).
- Multi-tenant billing / user self-service signup.

## Non-Goals

- Replacing WordPress feature-for-feature.
- Supporting thousands of concurrent authors.
- Real-time collaboration.

## Legacy

A prior PHP + SQLite implementation exists on the `legacy/php-stack` branch and is **not** being ported file-by-file. Planning restarts from scratch under the Go + htmx stack.

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

The April-to-September 2026 gap in Validated above is what happens when this does not
run: the code moved a long way and the planning artefacts said "v1.0 complete" the
whole time.

---

*Stack is mandated by the user. Every phase, research note, and plan must respect it. If a requirement seems to need something outside the stack, flag and propose a stack-compatible alternative.*

---
*Last updated: 2026-09-10 after the v1.6 milestone*
