# Requirements: Holzcloud CMS

**Defined:** 2026-04-14 (v1.0) · 2026-09-03 (v1.6)
**Core Value:** One Go binary runs several websites without dependency soup.

> The v1.0 requirements below are a record of what shipped, and name arm64 because
> that was the target then. The target moved to linux/amd64 on 2026-09-03; the
> shipped text is left as it was rather than rewritten after the fact.

## v1.0 Requirements — shipped 2026-04-14

### Foundation

- [x] **FND-01**: Single Go binary compiles with `CGO_ENABLED=0 GOARCH=arm64` and starts an HTTP server on a configurable port
- [x] **FND-02**: SQLite database created automatically on first run in a configurable `data/` directory with WAL mode, busy_timeout, and foreign_keys enforced on every connection
- [x] **FND-03**: Schema migrations run automatically at startup via embedded SQL files (goose); adding a new `.sql` file and rebuilding is all that's needed
- [x] **FND-04**: Graceful shutdown on SIGTERM/SIGINT — in-flight requests finish within 10s before process exits
- [x] **FND-05**: Admin templates, default public template, static assets (CSS, htmx.min.js), and migrations are embedded in the binary via `embed.FS`
- [x] **FND-06**: Structured JSON logging via `slog` with configurable log level

### Authentication & Security

- [x] **AUTH-01**: Admin user can log in with email and password; password verified with Argon2id
- [x] **AUTH-02**: Sessions stored server-side in SQLite via `alexedwards/scs`; session ID rotated on login
- [x] **AUTH-03**: All non-GET admin requests protected by CSRF token validated via `gorilla/csrf`; htmx requests send the token via `hx-headers` on `<body>`
- [x] **AUTH-04**: Secure session cookies: HttpOnly, SameSite=Lax, Secure in production
- [x] **AUTH-05**: Role-based access: admin (full) and editor (content only) roles enforced by middleware
- [x] **AUTH-06**: First-run bootstrap: if no users exist, the first visit to `/admin` shows a setup form to create the initial admin account

### Multi-Site

- [x] **SITE-01**: Admin can create, edit, and delete websites (name, description, active flag)
- [x] **SITE-02**: Admin can assign multiple domains to a website; each domain resolves to exactly one website
- [x] **SITE-03**: Incoming requests are routed to the correct website by matching `Host` header against the `website_domains` table (middleware)
- [x] **SITE-04**: Requests for unrecognized hosts return a 404 (or a configurable default site)

### Pages

- [x] **PAGE-01**: Editor can create, edit, and delete pages within a website; fields: title, slug, Markdown content, status (draft/published), publish date
- [x] **PAGE-02**: Markdown content is rendered to HTML via goldmark and sanitized via bluemonday on save; both raw Markdown and rendered HTML are stored
- [x] **PAGE-03**: Slugs are unique per website and auto-generated from the title (editable)
- [x] **PAGE-04**: Only published pages are visible on the public site; draft pages return 404 publicly
- [x] **PAGE-05**: Page list in admin supports pagination via htmx partial swap
- [x] **PAGE-06**: Inline editing of page title/slug from the list view via htmx (click-to-edit, save inline)

### Public Site

- [x] **PUB-01**: Public site renders pages using the active template for the resolved website
- [x] **PUB-02**: Template engine uses Go `html/template` with per-site template directories; disk-first, embedded default fallback
- [x] **PUB-03**: Public responses include `Cache-Control`, `ETag`, and `Last-Modified` headers for static and page responses
- [x] **PUB-04**: Unknown slugs return a styled 404 page using the site's active template
- [x] **PUB-05**: Static template assets (CSS, images within a template directory) are served with correct MIME and long cache headers

### Templates

- [x] **TPL-01**: Admin can upload a template as a `.zip` archive; server extracts safely (zip-slip prevention, extension allow-list, size cap)
- [x] **TPL-02**: Admin can activate one template per website; the active template is used for public rendering
- [x] **TPL-03**: Template directories follow a convention: `layout.html`, `page.html`, `404.html`, `assets/` subdirectory
- [x] **TPL-04**: Templates are listed and deletable from the admin UI

### Menus

- [x] **MENU-01**: Admin can create multiple menus per website with a location key (e.g. "main", "footer")
- [x] **MENU-02**: Menu items are hierarchical (parent/child) with sort order; CRUD via admin
- [x] **MENU-03**: Menu item reordering in admin (up/down buttons; drag-and-drop deferred to v2)
- [x] **MENU-04**: Public templates can render a menu by location key with nested `<ul>` output

### Media

- [x] **MEDIA-01**: Editor can upload images/files per website via multipart form; files stored in `data/media/{website_id}/`
- [x] **MEDIA-02**: Upload validates MIME type against an allow-list and enforces a per-file size limit
- [x] **MEDIA-03**: Media files are served with correct `Content-Type` and cache headers
- [x] **MEDIA-04**: Admin UI lists uploaded media per website with delete action

### Admin UI

- [x] **UI-01**: Admin layout: sidebar navigation (desktop), responsive on mobile, sticky header with page title + primary action
- [x] **UI-02**: Design system: OKLCH color tokens, `@layer` cascade, 8px spacing scale, system font stack, clean/minimal aesthetic (Linear/Ghost/Vercel direction)
- [x] **UI-03**: All admin interactions work without htmx (full-page fallback); htmx adds inline editing, partial pagination, and smoother transitions
- [x] **UI-04**: Flash messages (success/error/warning) after form actions
- [x] **UI-05**: Admin dashboard shows overview of all websites with quick-action links
- [x] **UI-06**: View transitions for admin navigation where browser supports it
- [x] **UI-07**: Loading indicators (`hx-indicator`) for all htmx requests

### Users

- [x] **USR-01**: Admin can list, create, edit, and delete users (name, email, password, role)
- [x] **USR-02**: Password changes require re-entering the current password (or admin override)

### Deployment

- [x] **DEP-01**: `go build` produces a single self-contained binary for linux/arm64
- [x] **DEP-02**: Example systemd unit file with security hardening (`ProtectSystem`, `NoNewPrivileges`)
- [x] **DEP-03**: Example Caddy reverse-proxy config for TLS termination
- [x] **DEP-04**: `data/` directory holds SQLite DB + media + user templates; documented backup strategy

## v1.6 Requirements — Inhaltsmodell und Zugang

Most of these come from `docs/offene-punkte.md`, points 1, 2, 3, 4 and 6. The single
sign-on work is new. Point 5 (static export) is deliberately excluded — see *Out of Scope*.

**Renumbered from v1.5.** The tags `v1.4` and `v1.5` are released and pushed, and
CHANGELOG.md carries `## 1.5 — 2026-09-03`. The milestone that used to be called v1.5
never left 0 %, so it was renumbered rather than shipped under a number that already
means something else. Phases 6–8 of that plan became Phases 7–9 here.

### Housekeeping

The four items this milestone must clear before it adds a single string. Two of them
were scoped on premises that turned out to be wrong — the research corrected them and
they are written here as they actually stand.

- [x] **MAINT-01**: `tools/i18n` writes its catalogues through the standard library, and a round-trip test locks the format so a hand-translation pass cannot reintroduce drift. The committed catalogue files do not change — verified byte for byte, not assumed.
- [x] **MAINT-02**: `fr-CH.json` and `it-CH.json` are never written by the tool, only read. That is deliberate, but it is documented nowhere a person would look. Either the tool writes them or the tool says plainly that they are maintained by hand — and whichever it is, `tools/i18n`'s own doc comment stops claiming its output is indented.
- [x] **MAINT-03**: CI rebuilds all six committed WebAssembly modules — the five `plugin.wasm` files under `plugins/` plus `internal/plugin/testdata/echo.wasm` — and compares each result against the committed file, so a change to the SDK, or to the host calling convention `echo.wasm` exists to witness, can no longer be validated against a stale binary. The rule is the memorable one: no committed `.wasm` in this repository may go stale unchecked.
- [x] **MAINT-04**: The five tests that today skip themselves when a `plugin.wasm` is absent fail loudly in CI and stay forgiving on a contributor's machine, with a message that says how to build the missing file. In that order — the rebuild first, the promotion second.
- [x] **MAINT-05**: The planning and repository notes that have gone stale are corrected: `docs/offene-punkte.md` says migrations run to `00044` (they run to `00045`) and still lists the finished Dependabot item as work; all three `deferred-items.md` read as open although all three are closed; all **seven** codebase maps in `.planning/codebase/` carry an `Analysis Date` of 2026-08-22 and have drifted with the tree — countable facts and file references alike, with the same wrong numbers copied across several maps, and `INTEGRATIONS.md` still documenting a Kubernetes directory, a deployment workflow and an arm64 cross-compile job that no longer exist. The complete line-referenced checklist is `06-RESEARCH.md` §"MAINT-05 Correction Inventory"; the corrections are surgical and in place, and no map is ever regenerated.

### Field kinds

- [ ] **FIELD-01**: Author can configure a Choice field to render as a row of buttons instead of a dropdown. An optional one offers an explicit "no answer" choice, because a radio group cannot be un-set without JavaScript.
- [ ] **FIELD-02**: Author can define a Multiple-Choice field and pick several values at once; the value survives save and reload, and a theme can loop over it and print each one.
- [ ] **FIELD-03**: Author can define a Term field that picks from the tags a website already carries. The public page prints the term's **name**; renaming the term changes what the page shows without touching the page.
- [ ] **FIELD-04**: Author can define a Time field (`zeit`) and enter a time of day. It carries no timezone, and "empty" is distinguishable from midnight.
- [ ] **FIELD-05**: Author can define a Range field (`bereich`) bounded below and above. The chosen number is readable before saving, without JavaScript.
- [ ] **FIELD-06**: Author can define a Code field (`code`) — plain text, no Markdown, fixed-width type. HTML typed into it appears verbatim on the public page and does not execute, including when the field sits inside a block.
- [ ] **FIELD-07**: A multi-valued field's encoding is one mechanism, exported and shared: the page form, the renderer, the bundle round-trip and the CSV importer all read and write it through the same pair of functions, so none of them can invent a second spelling.
- [ ] **FIELD-08**: A field whose visibility depends on a Choice keeps working when that Choice is rendered as a button row. Field conditions are pure CSS and the existing rule matches an `<option>`, which a button row does not have.

### Text snippets

- [ ] **SNIP-01**: Admin can give a text snippet any field kind, not only Markdown, and fill the fields on the snippet screen.
- [ ] **SNIP-02**: A snippet's fields live in the field-definition table that already exists, with a `snippet_id` beside `website_id` — not in a third field table. The same field key can exist once on a page and once on a snippet of the same website without colliding.
- [ ] **SNIP-03**: A snippet's field values render on the public site through the same pipeline and the same sanitisation as page fields — one goldmark → bluemonday chain, not a second.
- [ ] **SNIP-04**: A snippet's fields never appear on a page's edit form, in a block kind's fields, or in a theme's page field list. The queries that select a carrier's fields exclude the other three namespaces explicitly.
- [ ] **SNIP-05**: Snippets that already exist keep working untouched — their Markdown body still renders wherever a theme calls them, no snippet key changes, and the admin performs no migration step. The published `.Site.Snippets` contract keeps its type; field values arrive beside it, not through it.

### Import

- [ ] **IMP-01**: Admin can upload a CSV file and map each column to a target — title, body, a custom field, or explicitly nothing.
- [ ] **IMP-02**: A CSV import creates pages through the same path as any other creation, so slugging, validation and sanitisation apply unchanged.
- [ ] **IMP-03**: A CSV import reports per row what was created, updated or skipped, naming the row number and the reason. A file mixing good and bad rows imports the good ones and leaves nothing half-written.
- [ ] **IMP-04**: The import can target an existing website or create a new one, chosen on the first screen. Targeting an existing one is a deliberate departure from the rule that an importer always creates a new website; the reason that rule gives — every collision needs an answer — is answered by the update-or-skip choice and the dry run.
- [ ] **IMP-05**: Admin can run the whole file through validation without writing anything, and see what would happen, before committing to it.
- [ ] **IMP-06**: Columns are matched to fields automatically where the names correspond, ignoring case and accents; the admin can override every one of them.
- [ ] **IMP-07**: Admin can download an example CSV generated from that website's own field definitions, with the right column headings already in it.
- [ ] **IMP-08**: The mapping screen shows one real row from the file, navigable to the next, and each field can carry a default for cells that are empty or unmapped.
- [ ] **IMP-09**: An uploaded CSV is treated as hostile: bytes, rows and single cells are all capped; a byte-order mark from Excel does not break the first column; a stray quote does not swallow the rest of the file; a short row is reported by its row number; a NUL byte is refused.
- [ ] **IMP-10**: The import writes one transaction per row, never one for the whole file — the write pool admits a single connection, and a file-long transaction would block every other request on the machine.

### Single sign-on

Against a self-hosted Authentik, through its proxy provider in forward-auth mode. Not
an OIDC client inside the binary: that needs a dependency and an outbound call at
runtime, and is listed under what this project deliberately does not build.

- [ ] **SSO-01**: A person who has authenticated at Authentik reaches the admin without a second sign-in, because the reverse proxy passes their identity and the CMS believes it.
- [ ] **SSO-02**: A request that does not come from the reverse proxy has its identity headers ignored, and falls through to the ordinary password form — not to a refusal, because the way back in must not die with the proxy.
- [ ] **SSO-03**: The CMS strips every inbound identity header at the top of its chain, including the underscore spellings, before anything reads one. A misconfigured proxy is then a misconfiguration and not a way in.
- [ ] **SSO-04**: The proxy proves it is the proxy with a shared secret, compared in constant time and kept in the environment rather than the database, because the database is what ends up in every backup.
- [ ] **SSO-05**: An identity with no account is refused unless the operator has switched account creation on; with it on, the service refuses to start until it is told which website a new account belongs to. Silence must not mean "every website".
- [ ] **SSO-06**: Group membership decides role and website access, re-applied at every sign-in so that a demotion at the identity provider takes effect here, and every change of rights is written to the activity log.
- [ ] **SSO-07**: An Authentik session satisfies the second-factor requirement. Because that makes the second factor of this installation depend on the operator's Authentik enforcing one, the dependency is stated in `DEPLOY.md` and shown in the admin — not left in a source comment.
- [ ] **SSO-08**: Signing out signs the person out at Authentik too, so the next click does not silently sign them back in.
- [ ] **SSO-09**: With single sign-on switched off, nothing about signing in changes. The password path, the second factor, the recovery codes and the command-line way back in all behave exactly as they do today.
- [ ] **SSO-10**: The server binds to the loopback address by default. It listens on every interface today, which is harmless while a password is required and a total bypass the moment a header is believed.
- [ ] **SSO-11**: The shipped Caddy example strips the client's own identity headers explicitly, and `DEPLOY.md` names the minimum Caddy version — the `forward_auth` directive emits no such strip on its own, which is CVE-2026-30851.

### Throughout

- [ ] **QUAL-01**: Every string this milestone adds ships in all five languages — `go run ./tools/i18n` reports `0 offen, 0 verwaist` before a phase is done.
- [ ] **QUAL-02**: Every new field kind, every new screen and the sign-on path are exercised in the running application, not only in tests — the browser pass is what has caught the defects this project actually shipped.

## Later Requirements (deferred)

Trimmed 2026-09-02: revisions, the activity log, TOTP and password reset were built
between April and September and moved to Validated in PROJECT.md.

- **V2-01**: Dark mode via `light-dark()` token toggle
- **V2-03**: Scheduled page publishing (publish_at date)
- **V2-04**: SEO meta fields (title, description, og:image) per page
- **V2-05**: sitemap.xml + robots.txt generation per site
- **V2-06**: Full-text search via SQLite FTS5
- **V2-10**: Drag-and-drop menu reordering (Sortable htmx extension)
- **V2-11**: RSS feed per site
- **V2-12**: Template preview before activation
- **V2-13**: Maintenance mode per site
- **V2-14**: Saved, re-runnable import profiles — invites scheduling, which invites an outbound call
- **V2-15**: Per-locale snippet values — both Statamic and Craft localise their globals; here it meets the star-shaped locale model in ways not visible from outside, so it wants its own research
- **V2-16**: An arbitrary custom field as the CSV match key — a JSON scan per row
- **V2-17**: Revisions on snippets

## Out of Scope

| Feature | Reason |
|---------|--------|
| WYSIWYG / contenteditable | Requires heavy JS; contradicts htmx-only constraint |
| React / Vue / Svelte | SPA framework; contradicts server-rendered mandate |
| External databases | SQLite only; one file, no service to run beside it |
| npm / bundlers / Tailwind | No build tools; plain CSS only |
| Multi-tenant billing | Not a SaaS product |
| Static export | A second mode of operation beside the one that works — no forms, no search, no protected pages. `docs/offene-punkte.md` point 5. Excluded from v1.6 by explicit decision. |
| Authentik as an OIDC/OAuth client in the binary | A dependency, a client secret in the configuration, and an outbound call at runtime — the one rule this project does not break. Forward-auth delivers the same single sign-on with none of the three. |
| Verifying `X-authentik-jwt` | Verification means fetching JWKS at runtime or pinning a key. It protects nothing that the trust boundary does not already protect, and a half version — parsing without verifying — is worse than none, because it looks like a defence. |
| CSV "delete rows missing from the file" | One mis-mapped key column deletes a website. |
| Downloading images named by URL in a CSV | Nothing is fetched from a third party at runtime; the WordPress importer already refuses this and lists the URLs instead. |
| A live JavaScript readout beside the `bereich` slider | `internal/tmplmgr/script.go` rejects exactly that pattern in an uploaded template. Building it into the admin would be the project contradicting its own rule. |
| Real-time collaboration | Out of scope at this scale |
| Docker as primary deployment | Optional later; not the primary path |

## Traceability

v1.0 requirements were mapped to phases 1–5, all complete; that mapping is archived with
the phase directories under `.planning/milestones/v1.0-phases/`.

v1.6 phases continue the numbering at 6.

| Requirement | Phase | Status |
|-------------|-------|--------|
| MAINT-01 | Phase 6 | Complete |
| MAINT-02 | Phase 6 | Complete |
| MAINT-03 | Phase 6 | Complete |
| MAINT-04 | Phase 6 | Complete |
| MAINT-05 | Phase 6 | Complete |
| FIELD-01 | Phase 7 | Pending |
| FIELD-02 | Phase 7 | Pending |
| FIELD-03 | Phase 7 | Pending |
| FIELD-04 | Phase 7 | Pending |
| FIELD-05 | Phase 7 | Pending |
| FIELD-06 | Phase 7 | Pending |
| FIELD-07 | Phase 7 | Pending |
| FIELD-08 | Phase 7 | Pending |
| SNIP-01 | Phase 8 | Pending |
| SNIP-02 | Phase 8 | Pending |
| SNIP-03 | Phase 8 | Pending |
| SNIP-04 | Phase 8 | Pending |
| SNIP-05 | Phase 8 | Pending |
| IMP-01 | Phase 9 | Pending |
| IMP-02 | Phase 9 | Pending |
| IMP-03 | Phase 9 | Pending |
| IMP-04 | Phase 9 | Pending |
| IMP-05 | Phase 9 | Pending |
| IMP-06 | Phase 9 | Pending |
| IMP-07 | Phase 9 | Pending |
| IMP-08 | Phase 9 | Pending |
| IMP-09 | Phase 9 | Pending |
| IMP-10 | Phase 9 | Pending |
| SSO-01 | Phase 10 | Pending |
| SSO-02 | Phase 10 | Pending |
| SSO-03 | Phase 10 | Pending |
| SSO-04 | Phase 10 | Pending |
| SSO-05 | Phase 10 | Pending |
| SSO-06 | Phase 10 | Pending |
| SSO-07 | Phase 10 | Pending |
| SSO-08 | Phase 10 | Pending |
| SSO-09 | Phase 10 | Pending |
| SSO-10 | Phase 10 | Pending |
| SSO-11 | Phase 10 | Pending |
| QUAL-01 | Phase 10 (gate on 6, 7, 8, 9, 10) | Pending |
| QUAL-02 | Phase 10 (gate on 6, 7, 8, 9, 10) | Pending |

QUAL-01 and QUAL-02 are recurring gates, not deliverables. They are counted once — in
Phase 10, the last phase, where they close milestone-wide — and are additionally written
verbatim as the final success criterion of Phases 6 through 9. Assigning them to the last
phase rather than the first is deliberate: a gate mapped to Phase 6 would tell every later
phase it was already satisfied. See the *Standing Gates* section of `.planning/ROADMAP.md`.

**Coverage:**

- v1.6 requirements: 41 total
- Mapped to phases: 41
- Unmapped: 0
- Orphans: 0 · Duplicates: 0

**Phase distribution:**

| Phase | Name | Requirements | Count |
|-------|------|--------------|-------|
| 6 | Aufräumen | MAINT-01…05 | 5 |
| 7 | Field Kinds | FIELD-01…08 | 8 |
| 8 | Snippets Carry Fields | SNIP-01…05 | 5 |
| 9 | CSV Import | IMP-01…10 | 10 |
| 10 | Authentik Forward-Auth | SSO-01…11, QUAL-01, QUAL-02 | 13 |

## Decisions taken while defining these

Recorded here so a later reader does not reopen them.

| Decision | Why |
|----------|-----|
| A multi-valued field is stored one value per line, in the string slot that already exists | The delimiter is already illegal inside a value: the option list is read one per line, so a configured choice cannot contain a newline. Not "we hope nobody types one". Correct for closed vocabularies, which is all this milestone has. |
| Across a CSV cell the same values are separated by a pipe | A newline inside a CSV cell is legal but must be quoted and is invisible in a spreadsheet. One line of code, one sentence of documentation. |
| `darstellung`, `max_werte` and the `bereich` bounds get their own columns | The alternative was to reuse the `auswahl` column, which is read one option per line — a line that is not an option is exactly the ambiguity the encoding decision above was careful to avoid. Phase 7 therefore ships migration `00046`, Phase 8 `00047`. |
| A Term field stores the term's slug | Renaming a term already keeps its address, so the slug is the stable identity. An id cannot be typed into a spreadsheet cell; a name goes stale on rename. |
| `code` and `KindTerm` are excluded from block kinds where the reason applies | A block freezes to HTML when the page is saved, so a value that must follow a later rename cannot live in one. |
| `tools/i18n` gets a round-trip test, not only a corrected comment | The catalogues already match the tool's output; the test is what stops the drift returning through the next hand-translation pass. |
| The wasm rebuild in CI lands before the test skips are promoted | Promoting first gives a contributor with a partial checkout a red suite they cannot fix, and closes nothing — the hole is that CI never rebuilds. |
| An Authentik session satisfies the second factor, unconditionally | The operator's decision, taken 2026-09-03. It makes this installation's second factor depend on Authentik enforcing one, which is why SSO-07 requires that dependency to be stated in `DEPLOY.md` and shown in the admin rather than left in a comment. |
| The CSV import offers both an existing website and a new one | The operator's decision, taken 2026-09-03. Two paths, both of which must be exercised in the browser pass. |
| Accounts are created automatically only when switched on, and then only with an explicit default website | The operator's decision, taken 2026-09-03. "No website assigned" currently means "every website", so a freshly created account would otherwise reach everything. |

---
*Requirements defined: 2026-04-14 (v1.0), 2026-09-03 (v1.6)*
*Last updated: 2026-09-03 — v1.6 requirements defined after project research*
