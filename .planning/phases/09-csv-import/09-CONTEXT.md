# Phase 9: CSV Import - Context

**Gathered:** 2026-09-06
**Status:** Ready for planning

> Written in English to match `ROADMAP.md` and `REQUIREMENTS.md`, the documents
> downstream agents read alongside this one.
>
> **The developer asked for the whole milestone to be carried out
> autonomously.** Every decision below is therefore Claude's, taken against the
> tree rather than from preference, and each one names the evidence it rests on.
> The questions that would otherwise have been put to the developer are listed
> under *Claude's Discretion* with how each was settled. **Two of the roadmap's
> own planning notes are overridden here**; both overrides are stated with the
> evidence that forced them, rather than quietly applied.

<domain>
## Phase Boundary

Content arrives as a table. An admin uploads a CSV beside the existing WXR and
bundle importers, points each column at a target, sees what would happen before
anything is written, and gets pages created through the ordinary creation path
plus an honest per-row account of what happened.

Requirements: IMP-01 … IMP-10

**Not in this phase:** single sign-on (Phase 10), the gallery (Phase 11).
Deliberate anti-features, already decided in `ROADMAP.md` and not reopened:

- **No "delete rows missing from the file".** One mis-mapped key column would
  delete a website.
- **No downloading of images named by a URL in a cell.** Nothing is fetched from
  a third party at runtime; `internal/admin/wordpress.go:63-71` already refuses
  this for WXR and lists the addresses instead. This phase does the same.
- **No export.** The phase is an importer. The one thing it writes out is the
  example CSV of IMP-07, which is a template, not a dump.

</domain>

<decisions>
## Implementation Decisions

### The two roadmap notes this phase overrides

#### D-01: the file is carried server-side. "Re-submit the file with the mapping" cannot be built.

`ROADMAP.md` Phase 9 planning notes say:

> **Carrying the file between steps: do not put the parsed table in the
> session.** SCS stores sessions in SQLite; a 2 MB CSV in a session row is a bad
> day. **Re-submit the file with the mapping** — simpler, and needs no cleanup
> job.

The first half stands and is honoured. The second half does not survive contact
with the browser, for two reasons, the second of which is decisive:

1. **A server cannot fill in a file input.** `<input type="file">`'s value is not
   settable from markup or from script for security reasons, and a form POST
   loads a *new document* — so the file input rendered on screen 2 arrives
   empty. "Re-submit the file with the mapping" therefore means *the operator
   picks the same file again on every screen*: three picks for four screens.
2. **IMP-08 would be unbuildable.** It requires the mapping screen to show one
   real row *"navigable to the next"*. Every step forward is a request. Under
   re-submission every arrow click would demand a fresh file pick. The
   requirement and the note contradict each other, and the requirement wins.

**What is built instead:** the upload on screen 1 is staged server-side, and
screens 2, 3 and 4 are plain forms carrying a token — **no file input after
screen 1**. The note's actual concern — *do not put the parsed table in the
session* — is honoured exactly: the SCS session row gains nothing at all, and
what is staged is the **raw bytes**, never a parsed table.

- **Medium: a `csv_imports` table, not a file under `data/`.** One place to
  delete from, `ON DELETE CASCADE` from `users` so an account deletion cannot
  leave orphans, and the sweep is one `DELETE` rather than a directory walk that
  has to tell a stale staging file from a live one. The byte cap is 10 MB
  (D-06); SQLite carries a 10 MB BLOB without complaint, and it is written once
  and read three times.
- **The cleanup job the note worried about already has a home.** `cmd/holzcloud/main.go:443`
  registers a `jobs.Job` runner; `outbox-prune` (`:498`), `mail-prune` (`:510`) and
  `token-purge` (`:515`) are three existing prunes to copy. A `csv-import-prune` job
  beside them, dropping staged uploads older than a day, is ~8 lines and no new
  machinery. The note's "needs no cleanup job" was the argument *for*
  re-submission; the argument costs less than the note assumed and buys a
  requirement that is otherwise impossible.

#### D-02: one transaction per row is not achievable through the ordinary stores, and IMP-02 outranks it.

`IMP-10` and success criterion 5 both say *"one transaction per row, never one
for the whole file"*. The **"never one for the whole file"** half is absolute
and is delivered. The **"one per row"** half needs a correction:

Measured against the tree, one imported row already spans **two or three**
transactions, and none of them is ours to merge:

| Step | Where | Transaction |
|---|---|---|
| create the page | `internal/page/store.go:461` `CreatePage` | a single `INSERT`, autocommitted |
| the page's terms | `internal/term/store.go:119` `SetForPage` | opens its **own** `BeginTx` at `:120` |
| a term field's vocabulary | `internal/term/store.go:329` `EnsureNames` | opens its **own** `BeginTx` at `:330` |

Wrapping a row in one transaction would mean threading a `*sql.Tx` through
`page.Store` and `term.Store`, i.e. a second creation path beside the ordinary
one — **which is precisely what IMP-02 forbids** ("creates pages through the
same path as any other creation, so slugging, validation and sanitisation apply
unchanged"). Criterion 4 says the same thing in the goal's own words:
*"indistinguishable from hand-made ones … because the importer calls the same
creation path as any other creation."*

**So the guarantee this phase actually ships, and the words the plan must use:**

- **No transaction ever spans more than one row.** This is the constraint that
  matters and the one the write pool (`SetMaxOpenConns(1)`, `db.go:28`
  `_txlock=immediate`) makes load-bearing: a file-long transaction would block
  every other request on the machine, admin and public alike.
- **"Leaves nothing half-written" is delivered twice over: by validating the
  whole row before the first write, and by compensating if a later step of the
  row still fails.** `field.CheckAll` is the same validator the real write uses
  — which is exactly what makes the dry run trustworthy — so a row that fails it
  is skipped before `CreatePage` is called at all. For the residual window (the
  page is created, then `SetForPage` fails on a database error) the importer
  **undoes the row through the ordinary path**: `page.Store.TrashPage`
  (`store.go:801`) then `PurgePage` (`store.go:864`) — the same two steps an
  admin takes to delete a page and empty the trash. Both are single statements,
  neither does redirect bookkeeping, and **no new store method is needed**, so
  IMP-02 is not bent. If the compensation *itself* fails, the row is reported as
  a warning naming the page and what is missing, in the same voice
  `wordpress.go:149` already uses — a lie about the row would be worse than an
  ugly one.
- `REQUIREMENTS.md` IMP-10 gets an amendment stamp recording this, the way
  Phase 7's criterion 1 was amended. **The plan must not silently ship a weaker
  guarantee than the requirement's wording.**

### The shape

- **D-03: three layers, copying `internal/wxr` + `internal/admin/wordpress.go`,
  improving one thing.**
  - `internal/csv` — **pure**: takes an `io.Reader`, yields rows. No database,
    no HTTP, no `internal/page`. `wxr.Parse` materialises the whole document;
    `csv.Reader.Read()` genuinely streams, and this package keeps that property.
  - a one-row create returning a **reason string** — `wordpress.go:107`
    `importWordPressItem` is the exact shape: `""` means it worked, anything
    else is the reason it did not. Copy the signature, copy the discipline.
  - a handler that caps, parses, loops and reports.
- **D-04: `internal/csv` is the package name, beside `internal/wxr`.** It
  shadows the stdlib `encoding/csv` inside its own importers, which is why the
  stdlib import inside the package is aliased once and the package itself
  exposes domain types, not `csv.Reader`. `plugins/kontaktformular/csv.go`
  already writes CSV under this project's rules and is the writer-side model.
- **D-05: a third `<details>` panel on `website_list.html`.** Lines `4-22`
  (bundle) and `24-45` (WordPress) are two existing copies of exactly the panel
  needed; the third hangs the feature on the screen an admin already visits with
  no new navigation. **But** — unlike its two siblings the CSV panel leads to a
  *wizard*, so its submit goes to a screen, not to a report.

### The four screens

Screen 1 is a POST from the panel; screens 2–4 are GET/POST pairs on the
staging token. Routes, all under `/admin/websites/import-csv`:

| # | Route | What it does |
|---|---|---|
| 1 | `POST /admin/websites/import-csv` | cap, sniff, parse the header, stage, redirect to 2 |
| 2 | `GET /admin/websites/import-csv/{token}` | the mapping screen; `?zeile=N` steps the sample row (IMP-08) |
| 2b | `GET /admin/websites/import-csv/{token}/beispiel` | the example CSV download (IMP-07) — a GET, see D-34 |
| 3 | `POST /admin/websites/import-csv/{token}/probe` | the dry run — **writes nothing** (IMP-05) |
| 4 | `POST /admin/websites/import-csv/{token}/start` | the write, then the report |

- **D-06: every one of these is admin-only and every one goes into
  `TestRouteAuthorization`'s table at `cmd/holzcloud/main_test.go:158-172`.**
  Registering them in `newRouter` alone is not enough — the table is the safety
  net, and its own comment says it exists because website deletion once shipped
  reachable by any editor. The existing import routes at `main.go:852-853` are
  wrapped in `requireAdmin`; these match. **A staging token is additionally
  scoped to the user who created it**, so one admin cannot resume another's
  half-finished import.
- **D-07: the token is a random 128-bit value rendered as hex, not the row id.**
  A sequential id in a URL invites typing the neighbour's number; the ownership
  check makes that harmless, and the random token makes it uninteresting.

### The hostile file

`encoding/csv` has no limit anywhere. Every one of these is a named,
individually testable defence, and the plan should treat them as a checklist:

- **D-08: bytes — `http.MaxBytesReader(w, r.Body, 10<<20)`.** The same 10 MB as
  `wordpress.go:25`, for the same reason and with the same comment discipline.
- **D-09: rows — a `MaxRows` constant in `internal/csv`, in the shape of
  `wxr.MaxItems = 2000` (`wxr.go:30`), reported rather than silently applied**
  (`Export.Truncated` at `wxr.go:63` is the pattern). 5000 rows.
- **D-10: cells — 100 000 bytes.** One 9 MB cell is legal inside the byte cap.
- **D-11: the BOM is stripped from the first header cell.** This is the number
  one CSV import bug: Excel writes `﻿` and the first header arrives as
  `"﻿Titel"`, matching no mapping and silently losing the title column.
  Strip it once, at the reader, before the header is read — never at each
  comparison. `plugins/kontaktformular/csv.go:33` *writes* this BOM on purpose,
  which is the proof that files carrying it are the normal case here.
- **D-12: `LazyQuotes = true`.** An untrusted upload with a stray quote must
  produce a bad row, not swallow the rest of the file.
- **D-13: `FieldsPerRecord = -1`.** A short row becomes a *reported row with its
  row number* instead of an error carrying a line number.
- **D-14: count rows in our own loop.** Blank lines are skipped silently and a
  quoted field may span lines, so **row number ≠ line number**. Every message a
  person reads names the row. `csv.Reader.FieldPos` is available if a *line*
  ever needs naming as well; the row is what the requirement asks for.
- **D-15: a NUL byte is refused, not cleaned.** A file containing `\x00` is not
  a CSV somebody meant to upload; it is a binary file with the wrong extension
  or something worse. The check is on the raw bytes at stage time, before
  parsing, so it fails on screen 1 where the operator is still holding the file.
- **D-16: the example CSV is defused against formula injection.**
  `plugins/kontaktformular/csv.go:113` `entschaerfen` is the existing, commented
  implementation: a cell beginning `= + - @ \t \r` is prefixed with an
  apostrophe, because Excel and LibreOffice evaluate it as a formula on open.
  **That file is `package main` inside a wazero plugin and cannot be imported**,
  so `internal/csv` gets its own copy. That is a second copy of one rule and the
  plan must say so in the doc comment of both, naming the other — an
  uncommented duplicate is how the two drift.

### Mapping a column to a target

- **D-17: matching ignores case and accents by reusing `field.SlugifyKey`
  (`field.go:876`), not by adding a dependency.** It already folds
  `ä→ae ö→oe ü→ue ß→ss`, lowercases, and collapses spaces/hyphens/underscores
  to `_`. A header matches a field when `SlugifyKey(header)` equals `def.Key`
  **or** equals `SlugifyKey(def.Label)`. `golang.org/x/text` is an *indirect*
  dependency (`go.mod`) and stays one — `internal/template/dates.go:24` records
  that this project has already made that call once, deliberately.
- **D-18: which field kinds a column may be pointed at.** Decided against
  `field.Check` (`field.go:740`), which is the validator the write uses:

  | Kind | Mappable | Why |
  |---|---|---|
  | `text` `lang` `code` `zahl` `bereich` `datum` `zeit` | **yes** | a cell is text; `Check` validates it unchanged |
  | `janein` | **yes**, via D-19 | needs a parse, see below |
  | `auswahl` `mehrfachauswahl` | **yes** | the cell must match an option **exactly**; the dry run is what makes that survivable |
  | `link` | **yes** | `checkLink` accepts a path or an `http(s)://` address, which is what a cell holds |
  | `schlagwort` | **yes**, via D-20 | the cell holds a *name*; the importer derives the slug |
  | `bild` `verweis` | **no** | both store a numeric id that exists only in this installation. A cell naming a file would have to be fetched or guessed — the first is the forbidden third-party fetch, the second is worse |
  | `gruppe` `abschnitt` | **no** | a group is rows and a CSV row is flat; a section holds no value |

  Beside the fields there are four fixed targets: **Titel**, **Adresse (Slug)**,
  **Text (Markdown)**, **Zustand**, plus **Schlagwörter der Seite** — the page's
  own terms, the analogue of `wordpress.go:147`. And **"nichts"**, explicitly,
  which is what IMP-01 means by *"or explicitly nothing"*.

- **D-19: `janein` gets a real parse, and this is a trap worth naming.**
  `field.Check` has **no** `KindBool` case — any non-empty value passes — and
  both readers (`page_fields.go:281`, `render.go:126`) treat truth as
  `value != "" && value != "0"`. **A cell reading `nein` would therefore import
  as `true`.** The importer maps a closed vocabulary — `ja/nein`, `yes/no`,
  `true/false`, `wahr/falsch`, `1/0`, `x`/empty, case-insensitive — to the
  stored `1`/`""`, and **anything else is a reported row, not a guess.** This is
  user-visible and belongs in the mapping screen's help text and in the example
  CSV's header comment.
- **D-20: `schlagwort` carries the name, and the importer derives the slug.**
  The kind stores a *slug* (`field.go` `KindTerm` doc) and `Check` demands
  `page.Slugify(value) == value`. A spreadsheet cell holds "Möbel", not "moebel".
  So the importer runs `term.Normalize` (`term/store.go:66`) → `page.Slugify` →
  `term.EnsureNames` (`term/store.go:329`, which is `ON CONFLICT DO NOTHING` and
  never renames an existing label) before the value reaches `CheckAll`.
  **`internal/bundle` already does exactly this, and the plan should copy it
  rather than re-invent it:** `format.go:163-170` states that a term travels as
  its *name*; `import.go:289-329` `importTerms` normalises every manifest name
  and calls `EnsureNames` **before** any page is created; and
  `import.go:573-583` then stores a term *field's* value as
  `page.Slugify(val)`, with a comment saying why the two derivations agree by
  construction rather than by luck. The CSV importer's order is the same:
  **collect every term cell in the file and `EnsureNames` them once, before the
  row loop**, then slugify per value.
- **D-21: the multi-value wire format across a CSV cell is a pipe.**
  `strings.Join(strings.Split(cell, "|"), "\n")`, then `field.JoinValues`. A
  newline inside a cell is legal RFC 4180 but must be quoted and is invisible in
  a spreadsheet. **`field.JoinValues` (`field.go:973`) was already hardened for
  this caller in Phase 7** — its doc comment says so in as many words: *"Für
  einen späteren Aufrufer, dessen Werte aus einer CSV-Spalte stammen, ist sie
  die Grenze, hinter der er keinen zusätzlichen Wert prägen kann."* The
  importer must go **through** `JoinValues`, never build the newline string
  itself; that is the whole point of the Phase 7 fix.

### The dry run and the report

- **D-22: the dry run runs the identical row function, with the write suppressed
  by not being reached.** One function, `pruefeZeile`, returns a verdict per row
  — *anlegen / aktualisieren / übergehen* plus a reason — and screen 4 calls it
  and *then* writes. Two functions that decide the same thing would drift, and
  the drifted one is always the one the operator did not see.
- **D-23: every slug the database renamed is reported.** `CreatePage`
  (`store.go:469-489`) silently retries `-2`, `-3`, … up to
  `maxSlugAttempts = 100`, and returns the created page — so
  `created.Slug != wanted` is the whole test. Criterion 4 asks for this by name.
- **D-24: the report screen is its own template, not `import_report.html`.**
  The existing one (`cmd/holzcloud/templates/admin/import_report.html`) renders
  four counters and a flat `<ul>` of warnings, and its counters are
  `bundle.Report`'s. A CSV report is *per row* — 312 rows with 40 problems, in
  the roadmap's own words, is an information-design problem. Reusing that
  template would mean widening `bundle.Report` for a second, differently shaped
  consumer.
- **D-25: the report is grouped by reason, not listed by row, with the rows
  named inside each group.** "17 Zeilen: „Sorte": „Apfel" steht nicht zur
  Auswahl (Zeilen 4, 9, 12, …)" is one line a person can act on; forty separate
  lines saying the same thing is a wall. Rows that succeeded are a count plus a
  link to the page list, not 272 lines. **This is the screen the feature is
  judged by** and the plan should give it its own task.

### What the edge probe found that the decisions above had missed

The eight-category edge probe was run over IMP-01 … IMP-10 before planning.
**34 edges applicable, 34 closed** — 27 `resolved`/`explicit`, 7 `dismissed`
with a reason, **0 unresolved**. The full input, resolutions and coverage are
committed beside this file as `09-EDGES-requirements.json`,
`09-EDGES-resolutions.json` and `09-EDGES-coverage.json`, and the planner should
lift each `resolved`/`explicit` resolution into a check.

Five of them are things D-01 … D-25 did not say, and four are defects waiting to
happen:

- **D-26: the row number is the spreadsheet's row number — header is row 1, the
  first data row is row 2.** (IMP-03 / boundary.) Not `csv.Reader`'s record
  index, not the line number. Every requirement in this phase that says "row
  number" — IMP-03, IMP-09, criterion 3, criterion 5 — is wrong by one if this
  is not fixed in one place. **One helper mints the number, and the dry run, the
  sample-row stepper and the report all call it**, so the "Zeile 2" on the
  mapping screen is the same row as the "Zeile 2" in the report.
- **D-27: two columns with the same header do not collide, because a column is
  addressed by its index.** (IMP-01 / adjacency.) A file with two `Titel`
  columns shows two entries in the mapping list, distinguished by position, each
  pointable at a different target. For the *automatic* match the first wins and
  the second is left unmapped **with a sentence saying so** (IMP-06 /
  adjacency) — an unexplained blank looks like an oversight.
- **D-28: the header fold strips combining marks before `field.SlugifyKey`.**
  (IMP-06 / encoding.) `SlugifyKey` (`field.go:876`) matches the *single rune*
  `'ü'` (U+00FC). A header written on macOS, or exported by a tool that
  normalises to NFD, arrives as `u` + U+0308 — and `Grösse` would silently fail
  to match its own field, which is exactly the class of bug IMP-06 exists to
  prevent. The fix is `unicode.Mn` stripping before the fold: three lines,
  `unicode` only, **D-17 stands and no dependency is added.** The documented
  rule becomes: *strip combining marks, then `field.SlugifyKey`.*
- **D-29: the target and the mapping are re-checked at the dry run and again at
  the write.** (IMP-04 / concurrency.) The website, and every field definition
  the mapping names, can be deleted or changed between screen 1 and screen 4 —
  the wizard spans four requests and an unknown amount of wall time. A mapping
  naming a field that no longer exists is reported and its column falls back to
  unmapped; a target website that is gone ends the wizard with a message rather
  than a nil dereference.
- **D-30 (an argument for D-01 that D-01 did not make):** because the dry run and
  the write read **the same staged bytes**, the report cannot describe a
  different file than the one that gets written. (IMP-05 / concurrency.) Under
  the overridden "re-submit the file" design the operator could have picked a
  *different file* at the commit step and the dry run would have been describing
  the wrong one, silently. Staging is not only what makes IMP-08 buildable; it
  is what makes IMP-05 true.

Two resolutions are mechanical gates the plan should state as such:

- **IMP-09 / boundary** — every cap is tested at the boundary *and one step
  either side*: 10 MB / 10 MB + 1, `MaxRows` / `MaxRows` + 1, a 100 000-byte
  cell / 100 001. Six cases, not three.
- **IMP-10 / concurrency** — `grep -c 'BeginTx' internal/csv internal/admin/<the
  importer>` must measure **0**, and no call the row function makes may hold a
  transaction across two rows.

### What must not change

- `internal/wxr` and `internal/admin/wordpress.go` — copied from, not touched.
- `internal/bundle` — untouched. `bundle.Report` does not grow a CSV shape.
- `field.SplitValues` / `JoinValues` / `CheckAll` / `Clean` — **used**, never
  re-implemented and never adjusted for this caller. Phase 7 already did the
  adjusting.
- `page.CreatePage` — called, not extended, not given a transaction parameter
  (D-02).
- `internal/db/migrations/00046`, `00047`, `00048` — released; never edited.

### What the pattern map found, and the two traps that bite silently

`09-PATTERNS.md` is beside this file. Two of its findings change decisions above
rather than merely informing them, and both were re-verified here against the
tree before being adopted.

- **D-31: the four new templates must be added to `layoutPageNames`
  (`internal/web/render.go:46`) — a hand-maintained slice of 44 names.** A
  template that is missing from it renders as a bare fragment with no base
  layout: no navigation, no flash area, no CSRF body attribute. **The test suite
  cannot see this** — only a browser can. This is the project's recurring defect
  signature exactly ("a mechanism correct at every known site and silently wrong
  at one overlooked site"), and it is mechanically gated: **44 now, 48 after.**

- **D-32: a row verdict carries a reason CODE plus its arguments — never a
  formatted German sentence.** This **overrides half of D-03.** Copy
  `importWordPressItem`'s *discipline* (empty means it worked); do **not** copy
  its *type*.

  > Measured: `tools/i18n/main.go:47` collects `{{t}}`/`{{th}}`/`{{tf}}`
  > literals from templates, and `:53-65` collects the first string argument of
  > exactly **eight** named Go functions (`SetFlashError`, `SetFlashSuccess`,
  > `SetFlashWarning`, `Add`, `NewLayoutData`, `Titlef`, `T`, `N`). A string
  > built with `fmt.Sprintf` and appended to a report is **invisible to the
  > tool**. `wordpress.go:68-84` is the proof standing in the tree today: those
  > warnings are hard-coded German, and `go run ./tools/i18n` reports
  > `0 offen, 0 verwaist` anyway.

  Phase 9's report is *entirely* such strings. Left as `fmt.Sprintf`, **QUAL-01
  would report green while the whole report screen is German-only** — the same
  class of hole as Fenster Nr. 3 (`internal/field/field.go`'s untranslated
  rejection reasons). Twice is a pattern; a third time would be a decision. So a
  verdict is `{Row int, Outcome …, Reason ReasonCode, Args …}`, and the template
  renders it with `{{tf "…" .Args}}`.

- **D-33: the wizard pattern is invented here, so its shape is fixed by decision
  rather than by drift.** The pattern map searched every `Handle*` in
  `internal/admin`, plus `internal/public/checkout.go` and `internal/shop`, and
  found **no multi-screen wizard anywhere in this tree**. The two half-analogs
  disagree and neither can simply be copied:
  - `internal/admin/twofactor.go:138-207` is the **only** POST in the codebase
    that renders a *different* screen — but it keys its half-finished state on
    the **user id** (`EnsurePendingSecret`, `:154`), so one user can have only
    one enrolment in flight. **That line must be consciously dropped:** keyed the
    same way, a second CSV upload would silently destroy the first.
  - `internal/admin/confirm.go:27-73` is the model for one handler serving both
    GET and POST — but it carries a single URL, not a payload.

  Shape, fixed: **the token is a path segment**; **screen 2 is a GET** (so it is
  bookmarkable and `?zeile=N` steps the sample row without a POST); **screens 3
  and 4 are POSTs** (they are actions, and screen 4 is not repeatable by a
  refresh); and **a token the sweep has already removed renders a named "this
  upload has expired, please start again" screen, not a 404** — the operator did
  nothing wrong and a 404 says they did.

- **D-34: the example CSV is served by a GET, not a POST.** All five existing
  downloads in the tree are GETs (`media.go:377`, `bundle.go:48`,
  `template.go:361`, `language.go:74` and `:88`, `plugin.go:374`); a
  POST-that-downloads has no analog here. `language.go:88` is the closest model
  — `attachment; filename=…` with the name built from known-safe input — and
  `plugin.go:374`'s `safeDownloadName` is the escaping to copy if the filename
  ever carries the website's name.

- **D-35: the staging store lives in `internal/csvimport`, beside a pure
  `internal/csv`.** Build-order step 1 makes `internal/csv`'s purity
  load-bearing: the hostile-file checklist has to be testable without
  `db.Open`. Counter-evidence, named so the planner can overrule this
  knowingly: `internal/shop` puts `cart.go` and `pricing.go` in one package and
  is none the worse for it. Purity wins here because D-03 spends it.

- **A second, independent argument for D-24** (the report gets its own
  template): `import_report.html` uses `.import-summary` and `.import-warnings`,
  and **neither class has a single rule anywhere in `cmd/holzcloud/assets/`**.
  Reusing that template would mean inheriting two dead hooks and styling them
  for a screen they were not designed for.

### Baseline counts, measured against the pre-change tree

Phase 8 taught this the hard way: three of its five waves wrote counting gates
against a tree they had not measured, and each came out wrong by exactly what
the plan itself added. Wave 3 was the only wave with zero divergence, and it was
the only one whose plan tabulated line numbers first.

**Measured 2026-09-06 on `096a86c`, before any Phase 9 code exists.** Every
counting gate in a Phase 9 plan states its number as *this baseline **+** what
the plan itself adds*, never as an estimate:

| What | Command | Now |
|---|---|---|
| strings in source | `go run ./tools/i18n` | **1158** (`0 offen, 0 verwaist` in en/es/fr/it) |
| migrations | `ls internal/db/migrations/*.sql \| wc -l` | **48**, highest `00048_snippet_group_namespace.sql` |
| admin routes | `grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go` | **145** |
| `adminOnly` rows | the table at `main_test.go:158` | **14** |
| `<details>` panels | `grep -c '<details' …/website_list.html` | **2** |
| `jobs.Job{` entries | `grep -c 'jobs.Job{' cmd/holzcloud/main.go` | **11** |
| admin templates | `ls …/templates/admin/*.html \| wc -l` | **61** |
| `layoutPageNames` entries | the slice at `internal/web/render.go:46` | **44** — see D-31 |
| packages under `internal/` | `ls -d internal/*/ \| wc -l` | **38** |
| files using `BeginTx` | `grep -rln BeginTx internal/ --include='*.go' \| grep -v _test` | **14** |
| admin CSS files | `ls cmd/holzcloud/assets/*.css` | **2** |
| `encoding/csv` importers | `grep -rl 'encoding/csv' --include='*.go' .` | **1** (`plugins/kontaktformular/csv.go`) |

Baseline health at the same commit: `go build` clean, `go vet` clean,
`gofmt -l .` silent, `go test ./...` **0 failures**.

Note the last row against **IMP-10 / concurrency**: the importer must not add a
15th `BeginTx` file. That is the gate, and it is mechanical.

### Build order

1. **`internal/csv`, pure, with its own tests.** The hostile-file checklist
   (D-08 … D-16) is testable with no HTTP and no database, and it is the half
   most likely to be wrong. It lands first and alone.
2. **The migration + the staging store.** `00049_csv_imports.sql`, verified as
   the next free number (the tree runs to `00048`), plus the prune job.
3. **The row function and the mapping resolution** (D-17 … D-21) — everything
   between a parsed row and a `page.PageCreate`, still with no screens.
4. **The four screens**, the panel, and the example CSV.
5. **The report screen** (D-24, D-25) and the standing gate.

</decisions>

<references>
## Canonical References

### Phase definition
- `.planning/ROADMAP.md` — Phase 9 section: goal, six success criteria,
  planning notes. **Two notes overridden — see D-01 and D-02.**
- `.planning/REQUIREMENTS.md:140-149` — IMP-01 … IMP-10.

### The shape to copy
- `internal/wxr/wxr.go` — the pure parse layer. `MaxItems` at `:30`,
  `Truncated` at `:63`.
- `internal/admin/wordpress.go` — the handler. `MaxBytesReader` at `:25`, the
  one-row create returning a reason at `:107`, the "we do not fetch from third
  parties" report at `:63-71`.
- `plugins/kontaktformular/csv.go` — CSV *writing* under this project's rules:
  the Excel BOM at `:33`, `entschaerfen` at `:113`.

### The field system
- `internal/field/field.go` — `Check` `:740`, `SlugifyKey` `:876`,
  `SplitValues` `:940`, `JoinValues` `:973` (**already written for this
  caller**), `CheckAll` `:996`, `Clean` `:576`, `MaxValueBytes` `:227`.
- `internal/field/store.go` — `List` `:65` is the page's own fields.

### The creation path
- `internal/page/store.go:461` `CreatePage` — the silent slug retry at
  `:469-489`, `maxSlugAttempts = 100` at `:406`.
- `internal/page/slug.go` — `Slugify` `:108`, `Transliterate` `:89`,
  `ValidateSlug`, `MaxSlugLength = 200` `:54`.
- `internal/term/store.go` — `Normalize` `:66`, `EnsureNames` `:329`,
  `SetForPage`.

### Where it hangs
- `cmd/holzcloud/templates/admin/website_list.html:4-45` — the two existing
  `<details>` panels.
- `cmd/holzcloud/main.go:852-853` — the two existing import routes.
- `cmd/holzcloud/main.go:443-520` — the `jobs.Job` runner and three prunes.
- `cmd/holzcloud/main_test.go:158-172` — `TestRouteAuthorization`'s table.
- `internal/admin/handler.go:37-80` — the `Handler`; `pages`, `terms`, `fields`,
  `domains`, `resolver`, `db` and `cfg` are all already on it.

### Constraints
- `CLAUDE.md` — Go + htmx + plain CSS + SQLite; nothing fetched at runtime;
  every resource scoped to exactly one website; bluemonday on user HTML; draft
  pages never leak; a released migration is never edited.
- `internal/db/db.go:28` — the write pool's `_txlock=immediate`;
  `SetMaxOpenConns(1)`. This is why D-02 exists.

</references>

<insights>
## Existing Code Insights

### Reusable Assets

- **`field.CheckAll` is already the one shared validator** — the admin form, the
  bundle import and the assistant all go through it. That is what makes the dry
  run trustworthy at no cost: the dry run is the same call.
- **`field.JoinValues` was hardened in Phase 7 explicitly for this phase.** Its
  doc comment names the CSV caller. Using it is not a convenience, it is the
  reason the fix was made.
- **`entschaerfen` already exists**, commented, in the plugin. It has to be
  re-derived rather than imported, and both copies must name the other.
- **`term.EnsureNames` already handles "create the label if it is missing,
  never rename one that exists"** — exactly what a `Sorte` column needs.
- **The `jobs.Job` runner already has three prune jobs to copy.**

### Established Patterns

- **A one-row importer returns a reason string, not an error**
  (`wordpress.go:107`). Empty means it worked.
- **A cap is reported, never silently applied** (`wxr.Export.Truncated`).
- **A new route goes into `TestRouteAuthorization` in the same commit** as its
  registration.
- **Counting gates are measured line by line against the post-change tree.**
  Phase 8 wave 3 was the only wave with zero divergence, and it was the only one
  whose plan tabulated line numbers. Any gate in this phase that counts
  occurrences must name the file, the pattern and the expected number, computed
  against what the plan itself adds.
- **The browser pass runs after the review-fix round, not before.** In both
  Phase 7 and Phase 8 the verifier caught user-visible changes landing after the
  pass had already been signed off.

### Integration Points

- `cmd/holzcloud/main.go` — five new routes, one new job, one new store.
- `cmd/holzcloud/templates/admin/website_list.html` — the third panel.
- `internal/db/migrations/00049_csv_imports.sql` — new, next free number.
- Four new admin templates: mapping, dry run, report, plus whatever screen 1
  needs beyond the panel.

</insights>

<discretion>
## Claude's Discretion

Questions that would have gone to the developer, and how each was settled.

| # | Question | Settled | On what evidence |
|---|---|---|---|
| 1 | How does the file survive four screens? | **Staged server-side in a `csv_imports` table, swept by a prune job** (D-01) | A server cannot fill a file input, and IMP-08's "navigable to the next" would otherwise demand a file pick per click. The roadmap's actual concern — the parsed table in the SCS session — is honoured: raw bytes, own table |
| 2 | One transaction per row, literally? | **No — "never one for the whole file" is delivered; per-row atomicity is not, and IMP-10 gets an amendment stamp** (D-02) | A row already spans two or three transactions inside `page.Store` and `term.Store`. Merging them means a second creation path, which IMP-02 and criterion 4 both forbid in as many words |
| 3 | Which field kinds can a column feed? | **All but `bild`, `verweis`, `gruppe`, `abschnitt`** (D-18) | `field.Check`: the two excluded value kinds store a numeric id that exists only in this installation, and resolving a name to one would mean fetching or guessing |
| 4 | Accent-insensitive matching — add `golang.org/x/text`? | **No — reuse `field.SlugifyKey`** (D-17) | It already folds the accents this project cares about; `internal/template/dates.go:24` records the same call being made deliberately once before |
| 5 | Reuse `import_report.html`? | **No — its own template** (D-24) | The existing one renders `bundle.Report`'s four counters; a per-row report would mean widening a struct for a second, differently shaped consumer |
| 6 | What does a `janein` cell say? | **A closed vocabulary; anything else is a reported row** (D-19) | `field.Check` has no `KindBool` case and both readers treat non-empty-and-not-`0` as true, so a cell reading `nein` would import as `true` |
| 7 | Where does the panel hang? | **A third `<details>` on `website_list.html`** (D-05) | Two existing copies at `:4-22` and `:24-45`; no new navigation |

</discretion>

<deferred>
## Deferred Ideas

Named so they are not smuggled in, and so a later reader knows they were seen.

- **Updating an existing page from a CSV row.** IMP-04 and criterion 3 both
  mention *"updated"*, and screen 1's existing-website path needs an
  update-or-skip choice — so a minimal update path **is** in scope: match by
  slug, replace the mapped fields, leave everything else alone. What is
  **deferred** is anything cleverer: matching on an arbitrary key column,
  merging rather than replacing, or a per-column "only fill if empty".
- **Scheduled or repeated imports.** A CSV that is re-imported nightly is a
  synchronisation feature, and synchronisation needs the delete half that is an
  explicit anti-feature.
- **Tab- or semicolon-separated files.** `csv.Reader.Comma` makes this three
  lines, but every one of them is a guess about the file, and a wrong guess
  produces one column with the whole row in it. Deferred until somebody asks
  with a real file. *(If it lands, it lands as an explicit choice on screen 1,
  never as sniffing.)*
- **Importing snippets, menus or media from a CSV.** The phase imports pages.
- **`V2-18`** (snippet field image/ref/term values lost on a bundle round trip)
  is recorded in `REQUIREMENTS.md` and is not this phase's.

</deferred>
