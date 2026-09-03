# Holzcloud CMS — Project Context

**Created:** 2026-04-13 (autonomous initialization after technical pivot)
**Status:** v1.0 shipped 2026-04-14; grown well past it since. Releases v1.4 and v1.5 are tagged and public.
**Current milestone:** v1.6 — Inhaltsmodell und Zugang (started 2026-09-03)
**Stack (HARD MANDATE):** Go + htmx + CSS + SQLite

## Vision

A minimal, self-hosted CMS that runs as a single Go binary on a small linux/amd64 server. Manages **multiple websites**, each with **multiple domains**, serving server-rendered HTML to the public and exposing a simple admin UI (htmx-driven) for authoring templates, menus, and pages.

## Core Value

One small binary runs several websites without dependency soup. Authors work through a clean, responsive admin UI; readers get fast server-rendered pages.

## Current Milestone: v1.6 Inhaltsmodell und Zugang

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

## Requirements

### Validated

Shipped and in use. v1.0 delivered items 1–7 below; everything after that was built between
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
- ✓ Text snippets (Markdown only — this milestone widens them)
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

### Active

The v1.6 target features above.

### Out of Scope

Beyond the v1 exclusions further down, the following were considered for v1.6 and left out:

- **Static export** — a second mode of operation beside the one that works: it can serve
  neither forms, nor search, nor protected pages. If it is ever built, it must be an
  explicitly reduced output, not a second front door. Stays on `docs/offene-punkte.md`.
- **Authentik as an OIDC/OAuth client inside the binary** — `docs/offene-punkte.md` lists
  OAuth under what is deliberately not built, for the reason that holds here too: it is a
  second mode of operation beside the one that works, and it needs a dependency and an
  outbound call at runtime. The forward-auth header from the reverse proxy delivers the
  same single sign-on with neither.

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
*Last updated: 2026-09-03 at the start of milestone v1.6*
