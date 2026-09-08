# Roadmap: Holzcloud CMS

## Milestones

- ✅ **v1.0 — Core CMS** — Phases 1–5 (shipped 2026-04-14)
- 🚧 **v1.6 — Inhaltsmodell und Zugang** — Phases 6–10 (in progress)

Phase numbering continues from v1.0. It never restarts. The five v1.0 phase
directories are archived under `.planning/milestones/v1.0-phases/`.

There is no v1.5 milestone shell. The tags `v1.4` and `v1.5` are released and
pushed and CHANGELOG.md carries `## 1.5 — 2026-09-03`; the planned milestone
that used to be called v1.5 never left 0 %, so it was renumbered rather than
shipped under a number that already means something else. Its three phases moved
into this milestone as Phases 7, 8 and 9, and Phases 6 and 10 are new.

---

## Phases

**Phase Numbering:**

- Integer phases (6, 7, 8, 9, 10): planned milestone work
- Decimal phases (6.1, 6.2): urgent insertions, executed between their integers

<details>
<summary>✅ v1.0 — Core CMS (Phases 1–5) — SHIPPED 2026-04-14</summary>

Requirements FND, AUTH, SITE, PAGE, PUB, TPL, MENU, MEDIA, UI, USR and DEP —
all mapped, all complete, 13 plans. Goals, success criteria and plan records are
archived with the phase directories under `.planning/milestones/v1.0-phases/`.

- [x] **Phase 1: Foundation** — Binary skeleton, SQLite dual-pool + WAL, goose migrations, embed.FS, structured logging, graceful shutdown
- [x] **Phase 2: Auth + Admin Shell** — Login/logout, Argon2id, SCS sessions, CSRF + htmx wiring, admin layout and design tokens
- [x] **Phase 3: Multi-Site + Pages + Public Rendering** — Website/domain CRUD, Host-based routing, page authoring, slug routing, public template engine
- [x] **Phase 4: Templates + Menus + Media** — Template zip upload, activation, hierarchical menu builder, media upload and serving
- [x] **Phase 5: Admin Polish + Users + Deployment** — htmx progressive enhancement, view transitions, flash messages, dashboard, user management, systemd + Caddy configs

</details>

### 🚧 v1.6 — Inhaltsmodell und Zugang (In Progress)

**Milestone Goal:** A website describes its content model completely — every
field kind an author needs, in every carrier that holds fields, and content can
also arrive as a table — and whoever enters the admin may arrive through the
sign-in the operator already runs.

- [x] **Phase 6: Aufräumen** — The housekeeping that makes the rest measurable: the i18n writer's format locked by a test, CI rebuilding the plugin binaries it validates against, and the stale notes the later phases are planned against corrected
- [ ] **Phase 7: Field Kinds** — The palette an author reaches for: choice as a button row, a genuine multiple choice, terms as a field, plus `zeit`, `bereich` and `code` — and the multi-value encoding everything else depends on
- [ ] **Phase 8: Snippets Carry Fields** — A text snippet stops being one Markdown box and holds any field kind, reusing the field table that already exists
- [ ] **Phase 9: CSV Import** — Content arrives as a table: upload, map column to field, dry-run it, create pages the ordinary way, report every row
- [ ] **Phase 10: Authentik Forward-Auth** — Single sign-on taken as a header from the reverse proxy, with the trust boundary closed before the first header is read and the password path untouched behind it
- [x] **Phase 11: Galerie** — What the picture grid has been missing since it was built: enlarging an image, assembling an album once and using it on several pages, and paging through pictures instead of scrolling past them — all of it without a line of JavaScript

---

## Standing Gates (QUAL-01, QUAL-02)

Two of this milestone's forty-eight requirements are not deliverables. They are
gates that apply to **every** phase — which is all six.

- **QUAL-01** — every new string in all five languages; `go run ./tools/i18n`
  reports `0 offen, 0 verwaist`.
- **QUAL-02** — every new field kind, every new screen and the sign-on path
  exercised in the running application in a browser, not only in tests. Per
  `docs/offene-punkte.md`: the defects this project actually shipped were found
  by a browser pass, never by the suite.

**Decision on how they are mapped.** Mapping a recurring gate to the first phase
would tell Phases 7, 8, 9, 10 and 11 that the gate is already satisfied — the exact
failure mode to avoid. So they are handled two ways at once:

1. Each of Phases 6, 7, 8, 9 and 11 carries the gate **verbatim as its own final
   success criterion**, so no phase can be called done while its strings are
   untranslated or its screens unclicked.
2. For traceability's exactly-one-phase rule, QUAL-01 and QUAL-02 are formally
   assigned to **Phase 10**, the last phase to run — where they close
   milestone-wide and can no longer be deferred. Assigning them to the last
   rather than the first phase means they are never prematurely marked
   satisfied. Phase 11 carries a higher number but runs before Phase 10 (see
   *Execution Order*), so the close-out still covers it.

The Traceability table in REQUIREMENTS.md records this as
`Phase 10 (gate on 6, 7, 8, 9, 10, 11)`.

The gate's verbatim wording, identical in Phases 6 through 9 and in Phase 11:

> **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports
> `0 offen, 0 verwaist`, and everything this phase added that a person can see —
> every string, every control, every screen — has been driven once through the
> running application in a browser, not only through the test suite.

---

## Phase Details

### Phase 6: Aufräumen

**Goal**: The milestone's own ground truth is repaired before it adds a single string — the translation gate becomes meaningful, CI validates plugins against a binary it just built rather than one somebody committed, and no planning note that Phases 7–10 are planned against is still stale.
**Depends on**: Nothing (first phase of v1.6; Phase 5 shipped)
**Requirements**: MAINT-01, MAINT-02, MAINT-03, MAINT-04, MAINT-05
**Success Criteria** (what must be TRUE):

  1. `tools/i18n` writes its catalogues through the standard library, and running `-write` followed by `-schweiz` leaves an **empty `git diff`** — the committed catalogue files are proven unchanged byte for byte, not assumed. A round-trip test fails if a later hand-translation pass reintroduces drift.
  2. What happens to `fr-CH.json` and `it-CH.json` is written where a person would look for it: either the tool writes them, or it says plainly that they are maintained by hand — and `tools/i18n`'s own doc comment no longer claims its output is indented.
  3. CI rebuilds all six committed WebAssembly modules — the five `plugin.wasm` files under `plugins/` plus `internal/plugin/testdata/echo.wasm` — and compares each result against the committed file. An SDK change, or a change to the host calling convention, that would previously have validated green against a stale binary now turns CI red. The four committed `.zip` archives beside the plugins are repacked by the same tool, so a rebuild cannot leave archive and module disagreeing.
  4. The five tests that today skip themselves when a `plugin.wasm` is absent fail loudly in CI and stay forgiving on a contributor's machine, with a message naming the command that builds the missing file — and they were promoted **after** the rebuild landed, not before.
  5. The stale notes read true against the working tree: `docs/offene-punkte.md` names migration `00045` and no longer lists the finished Dependabot item as work, all three `deferred-items.md` entries read as closed, and the drifted facts and file references across all seven codebase maps in `.planning/codebase/` match what the code actually does — the complete line-referenced list is `06-RESEARCH.md` §"MAINT-05 Correction Inventory".
  6. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and everything this phase added that a person can see — every string, every control, every screen — has been driven once through the running application in a browser, not only through the test suite.

**Plans**: 7/7 plans executed, in 5 waves. `06-01` is a strict predecessor of all six others, so D-01's "before any code is touched" is carried by the wave graph, not only by prose. Wave 2 runs `06-02` ∥ `06-03` ∥ `06-04`.
**Wave 1**

- [x] 06-01-PLAN.md — Ground truth first (D-01): amend REQUIREMENTS.md and ROADMAP.md in one docs-only commit, before any code

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 06-02-PLAN.md — The tracer: `tools/wasm -print-hashes` end to end, plus the one-CI-run cross-host falsification gate that settles D-05
- [x] 06-03-PLAN.md — `tools/i18n` writes through `encoding/json`, a round-trip test over all seven catalogues locks the format, and the tool states which regional files it never writes
- [x] 06-04-PLAN.md — The stale notes that predate this phase: seven codebase maps corrected surgically, the finished Dependabot item retired, three `deferred-items.md` stamped closed

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 06-05-PLAN.md — `tools/wasm` complete: write, `-check`, `-out`, deterministic archive packing, and `-buildvcs=false` in all four documented build invocations

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 06-06-PLAN.md — The one-time rebuild of all ten build artifacts in an artifact-only commit, then the two permanent CI steps

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 06-07-PLAN.md — The five self-skipping tests promoted behind `HOLZCLOUD_TEST_REQUIRE_WASM`, every document this phase changed or invalidated brought true, and the standing gate

**Research flag**: none — housekeeping; both live questions were settled by the research and are recorded below.
**Planning notes**:

- **Both original premises were wrong and MAINT-01/MAINT-03 are already re-aimed. Do not re-discover this.**
  - *i18n:* the catalogues **already match** the tool's output. Verified twice, independently — a byte-level round-trip of all seven catalogues (7/7 `identical=true`) and an actual `-write` / `-schweiz` run producing an empty `git diff`. Settled by quick task `260903-bsk` on 2026-09-03 and recorded in `.planning/WINDOWS.md`. **Both sub-clauses are closed by `06-03`, and this note is retired rather than re-pointed** — it carried a line number into a file the phase then reshaped, and a fresh one would only start drifting again. (a) The indentation claim no longer exists: `writeCatalog`'s doc comment now describes the output as flush left, one entry per line, with the keys sorted byte-wise by `encoding/json` itself; the package doc comment never made a claim about output shape at all. The format is held by `TestCatalogsSurviveTheRoundTrip` over all seven catalogues in `tools/i18n/main_test.go`, not by a note here — a test fails when it drifts, a roadmap bullet does not. (b) The tool now states which regional files it writes and which it only reads: the package doc comment explains that `de-CH.json` is rebuilt by `-schweiz` because a mechanical rule exists for it while `fr-CH.json` and `it-CH.json` are vocabulary choices maintained by hand, and the per-file report line that crosses the screen on every run says so too — `nur gelesen, von Hand gepflegt` against `wird von -schweiz erzeugt`.
  - *wasm:* all five `plugin.wasm` files **are committed** and all five tests currently **pass**. The defect is that `.github/workflows/ci.yml` never rebuilds them, so an SDK change validates against a stale binary — a false pass, strictly worse than a skip.
- **Ordering constraint, non-negotiable:** rebuild-and-hash-compare in CI **first**, promote the skips **second**. Promoting first gives a contributor with a partial checkout a red suite they cannot fix and closes nothing, because the hole is that CI never rebuilds. Shape: **`tools/wasm`**, a Go command invoked as `go run ./tools/wasm` (and `-check` in CI), plus a CI step, plus `HOLZCLOUD_TEST_REQUIRE_WASM=1`. Not a shell script under `plugins/`: this repository's own tooling is Go commands under `tools/` (`tools/i18n`, `tools/mkbundle`) while shell exists only under `deploy/` — and the script this note used to name never existed (D-06).
- **The toolchain pin lives inside the build tool (D-02, D-03, D-03a).** `tools/wasm` sets `GOTOOLCHAIN` to an exact patch level as a constant in the tool itself — not in CI's `setup-go`, and not as a `toolchain` line in `go.mod`. A contributor running the tool locally must produce the same bytes as the runner, or the fragility has only moved from `go.mod` into CI. The pin has a **floor**: it may never be lower than the `go` directive in the root `go.mod` (`go 1.26.6` today), because `internal/plugin/testdata/echo` lives in the root module and therefore cannot carry a `toolchain` line of its own. Raising that directive means raising the pin and rebuilding all six guests in the same commit.
- **`-buildvcs=false` is mandatory, and it is the precondition for the byte comparison rather than an optimisation (D-02a).** Every committed module today carries `vcs.revision`, `vcs.time` and `vcs.modified=true`, stamped by the default `-buildvcs=auto`. The revision is the git SHA at build time, so a CI rebuild happens at a different commit *by construction* and can never match the committed bytes — no toolchain pin rescues this. The flag must appear in all four places the build command is written down: `tools/wasm`, `plugins/README.md`, `internal/plugin/testdata/README.md`, and the `//go:generate` line in `internal/plugin/runtime_test.go`.
- **CI budget matters.** `security.yml:38–41` records `go test -race ./internal/plugin/` **alone at 297 s**. That figure belongs to the wazero host under the race detector, not to a compile, and this step does not touch it. Measured in `06-RESEARCH.md` §"CI Cost": the six wasip1 guests built from a completely empty `GOCACHE` cost about **4.6 s** of wall time (≈15–20 s cold on a hosted runner, 2–4 s warm). The budget is real but small; it is measured rather than feared.
- **Any catalogue reformat is its own commit touching nothing but the catalogues**, proven mechanically with a `jq -S` semantic diff *before* committing. 4 590 lines otherwise bury the next real change and `git log -S` cannot recover it. (PITFALLS #25 — a content change disguised as a format change.)
- Related trap while in `encoding/json`: `Encoder.SetIndent("", "")` is a **no-op** (flush-left-with-newlines needs the free function `json.Indent`), and `json.MarshalIndent` *always* HTML-escapes with no switch — which is exactly why the hand-rolled `quote()` exists. Do not "simplify" to it.
- This phase adds no user-visible strings, so the standing gate's translation half is expected to be trivially green. Run it anyway; it is what makes the number mean something in Phases 7–10.

### Phase 7: Field Kinds

**Goal**: A website's content model reaches every field kind this milestone names — the choice control an author actually wants, a value that holds more than one string, tags as a field, and the three small kinds still missing — and each one survives save, reload, public render and the bundle round trip.
**Depends on**: Phase 6 (the translation gate must be meaningful and the notes correct before the first new string lands)
**Requirements**: FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06, FIELD-07, FIELD-08
**Success Criteria** (what must be TRUE):

  1. A multi-valued field's encoding is **one** exported mechanism: the page form, the renderer and the bundle round trip all read and write it through the same pair of functions, so none can invent a second spelling — and Phase 9's importer inherits it rather than inventing a third. A checkbox group's three values arrive as three values. Clearing a group is distinguishable from a form that never carried it **where the form is read** — `fieldsFromRequest` sees the key present and empty for the one and absent for the other — and the storage layer does not carry that difference further, because its save path is a full replacement of the whole `fields` column.
  2. A Choice field configured as a button row renders as a row of buttons in the page editor instead of a dropdown; an optional one offers an explicit "no answer" choice, so a click can be undone without JavaScript. A field whose visibility hangs on that Choice keeps appearing and disappearing correctly — the condition rule matches a button row, not only an `<option>`.
  3. A Multiple-Choice field accepts several values in one save, shows the same values after reload, and a theme can loop over them and print each one; a single-value read of it prints a readable joined string rather than a raw blob.
  4. A Term field offers the tags the website already carries as a chooser rather than a free-text box, stores the term's **slug**, and prints the term's **name** on the public page — renaming the term afterwards changes what the page shows without touching the page. The kind is absent from a block kind's field chooser, for the same reason `KindRef` is.
  5. `zeit` accepts a time of day that carries no timezone and where empty is distinguishable from midnight; `bereich` accepts a number between its configured bounds and the chosen number is readable **before** saving, without JavaScript; `code` is plain fixed-width text that never passes through Markdown — HTML typed into it appears verbatim on the public page and does not execute, including when the field sits inside a block.
  6. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and everything this phase added that a person can see — every string, every control, every screen — has been driven once through the running application in a browser, not only through the test suite.

> **Amended 2026-09-06 — Kriterium 1.** Der Satz lautete bis hierher
> „…and clearing a group is distinguishable from a form that never carried it",
> ohne zu sagen, wo. Gemessen wurde: `internal/field/field.go:575-581` — `Clean`
> schreibt nur fort, was nach dem Trimmen nicht leer ist, also erzeugen der
> vorhandene leere Schlüssel und der fehlende Schlüssel aus `field.Encode`
> byteweise dasselbe JSON. Nachgemessen in `07-VERIFICATION.md` (Lücke 1).
> Gesenkt statt die Datenstruktur umgebaut, weil Phase 9 die Unterscheidung
> dort nicht braucht: **IMP-08** (`REQUIREMENTS.md:147`) löst dasselbe Problem
> auf der richtigen Ebene — „each field can carry a default for cells that are
> empty or unmapped", von der Betreiberin auf dem Zuordnungsbildschirm
> ausdrücklich und je Feld gewählt, bevor irgendetwas geschrieben wird. Aus
> einer Formularkodierungskonvention Bedeutung abzuleiten wäre der schlechtere
> Mechanismus. Behoben im selben Zug: quick task `260906-ds0`.

**Plans**: 7/7 plans executed, in 7 waves. The chain is real rather than conservative: `internal/field/field.go`, `internal/field/render.go` and `field_input.html` are each touched by five of the seven plans, so two plans in one wave would be two agents editing one kind list. `07-05` (`KindTerm`) is independent in substance and could run anywhere; it is sequenced only by that file overlap.

**Wave 1**

- [x] 07-01-PLAN.md — Ground truth (D-01) in its own docs commit, then the tracer: one `mehrfachauswahl` field wired end to end through `SplitValues`/`JoinValues`, the `feld_<key>[]` marker, `Entry.Values`, the checkbox group and the bundle round trip

**Wave 2** *(blocked on Wave 1)*

- [x] 07-02-PLAN.md — Migration `00046` (D-14) and its four columns through all seven `page_field_defs` SQL sites, the definition screen and the bundle

**Wave 3** *(blocked on Wave 2)*

- [x] 07-03-PLAN.md — `zeit`, `bereich` (a bounded number input, not a slider — D-07) and `code`, plus the block-render escaping that makes D-06 true inside a block

**Wave 4** *(blocked on Wave 3)*

- [x] 07-04-PLAN.md — The multiple-choice guards: `max_werte` enforced server-side (D-05) and `MaxValueBytes` shared across all values and **reported** rather than truncated (D-13); the marker carried into a group's rows

**Wave 5** *(blocked on Wave 4)*

- [x] 07-05-PLAN.md — `KindTerm` (D-09): the chooser, the slug stored, the name printed, and the one `BlockKinds()` exclusion

**Wave 6** *(blocked on Wave 5)*

- [x] 07-06-PLAN.md — The button row (D-04, D-11, D-12) and the trap it sets: `switchOf` widened to take the `Def`, a switch name per controlling kind, and `.feld-schalter--knopfreihe` — the conditional rules touched exactly once (D-10, FIELD-08)

**Wave 7** *(blocked on Wave 6)*

- [x] 07-07-PLAN.md — The per-kind tax (`TEMPLATE-SPEC.md`, `SampleData`, `MinimalData`), the translation gate on its own commit, and the browser pass that settles D-08 by observation

**UI hint**: yes
**Research flag**: `bereich` only — "the value must be visible without JS" has three candidate answers and the choice is a UI-design question, not a technical one. Worth a **UI-SPEC**. Everything else in this phase follows standard patterns: the neighbours agree, the encoding is decided, native controls do the work, Go's escaping is already correct by default, and `KindRef` is a complete end-to-end template (chooser at `page_fields.go:98–255`, resolution at `render.go:36–60` + `pagedata.go:84–114`).
**Planning notes**:

- **Build order — the dependencies are real:**
  1. **Multi-value encoding first.** `SplitValues`/`JoinValues` beside `SplitChoices`/`JoinChoices`; `page_form.go:150` and `:163` become `field.JoinValues(values)`; `Entry` gains `Values []string`; `Resolve`, `List` and `Filled` gain a `case []string`. **Everything in this phase and all of Phase 9 depend on it — settle it before anything else is written.**
  2. `zeit`, `bereich`, `code` — an hour each, and `bereich` carries the `MayControl()` fix.
  3. Multiple choice.
  4. Button row + the `switchOf` / CSS change — **last**, so the conditional-field rule is touched exactly once.
  5. `KindTerm` — copies `KindRef` wholesale, independent of 1–4, parallelizable.
  6. Fixtures + `TEMPLATE-SPEC.md` + i18n + the browser pass.
- **Budget the per-kind tax once and remember it.** Every new kind costs: `Kinds`, `SubKinds()` / `BlockKinds()` (**both subtractive — a new kind is in unless excluded**, which is why the exclusions below are easy to miss), `field_input.html`, `CheckAll`, `Resolve` / `Entry`, then `TEMPLATE-SPEC.md` + `SampleData` + `MinimalData` (**three tests tie those together and fail the build if one is missing**), then `tools/i18n -write` followed by `-schweiz`, then the bundle and AI round trip. Roughly as much again as the kind itself.
- **Phase 7 ships a migration — `00046`.** This overrides the earlier "no migration needed" note. The DECISION is already taken in REQUIREMENTS.md: `darstellung`, `max_werte` and the `bereich` bounds get their **own columns**. The alternative was to reuse the `auswahl` column, which is read one option per line — a line that is not an option is exactly the ambiguity the encoding decision was careful to avoid. `ALTER TABLE ADD COLUMN`, and explicitly **no CHECK**, per `00028:25–27`. (`page_field_defs.art` itself still needs nothing: `TEXT NOT NULL`, no CHECK.)
- **The two subtractive exclusions, both silent if missed:** `KindTerm` must be excluded from `BlockKinds()` **beside `KindRef`** — a block freezes to HTML on save, so a value that must follow a later rename cannot live in one. And `MayControl()` must return **`false`** for `bereich`, exactly as it already does for `KindDate` — an `<input type=range>` never matches `:placeholder-shown`.
- **Two named pitfalls with addresses.** `page_form.go:150` (`out.Values[key] = values[0]`) silently keeps the first of a checkbox group's values and drops the rest; `:163` has the identical line for group rows. The fix is *not* "load the defs here" — the function deliberately reads by prefix before definitions are loaded and that property is load-bearing. Encode multi-valuedness in the **form field name**, minted in the one place that mints names, `field.Def.FieldName()` (`field.go:346`). Separately, `admin.css:1104`'s `:has(> .form-group option[value=""]:checked)` has no `<option>` to match once a Choice is a radio row — `switchOf` (`page_fields.go:186–195`) must return a new switch name with a matching `.feld-schalter--knopfreihe` rule, or **every field hanging on that Choice silently breaks**.
- Two shapes that are table stakes and cheap: an optional button row must render an explicit "— keine Angabe —" option (a radio group cannot be un-set in plain HTML), and a **hidden sentinel input must precede a checkbox group** (an all-unchecked group submits nothing, so the server cannot tell "cleared" from "not on this form"). Five lines each; multi-hour bugs if missed.
- `MaxValueBytes` (4000) is now shared across all selected values — `CheckAll` should say so rather than truncate.
- **A `code` field inside a block kind** is the easy miss: blocks freeze to HTML on save, so the escaping must happen in the block render path, not in the theme. Confirm it is in the plan.
- No JavaScript beyond htmx: the button row is radio inputs, the multiple choice is checkboxes or a `<select multiple>`, the slider is `<input type=range>`. All submit as a plain form; htmx is enhancement only. A live JS readout beside the slider is explicitly out of scope — `internal/tmplmgr/script.go` rejects exactly that pattern in an uploaded template.

### Phase 8: Snippets Carry Fields

**Goal**: A text snippet stops being one Markdown box and becomes a small content model of its own — any field kind, defined in the field table that already exists, rendered through the pipeline and the sanitisation that already exist, with nothing that already works changing.
**Depends on**: Phase 7 (the snippet form is built once against a complete kind palette rather than revisited)
**Requirements**: SNIP-01, SNIP-02, SNIP-03, SNIP-04, SNIP-05
**Success Criteria** (what must be TRUE):

  1. An admin can give a text snippet any field kind — including every kind Phase 7 added — fill the fields on the snippet screen, and see the same values after reload.
  2. The schema shows **one** field-definition table, not a third: a snippet's fields live in the existing field-definition table with a `snippet_id` beside `website_id`, and the same field key exists once on a page and once on a snippet of the same website without colliding.
  3. A snippet's fields never appear on a page's edit form, in a block kind's field list, or in a theme's page field list — every query that selects a carrier's fields excludes the other three namespaces explicitly, and the page case is confirmed in a browser and not only in a test.
  4. A snippet's field values render on the public site through the same pipeline and the same sanitisation as page fields — one goldmark → bluemonday chain, not a second. A `<script>` put into a snippet's long-text field is sanitised away exactly as it is on a page.
  5. Snippets that already exist keep working untouched: their Markdown body still renders wherever a theme calls them, no snippet key changes, the admin performs no migration step, and the published `.Site.Snippets` contract keeps its type — field values arrive **beside** it, not through it.
  6. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and everything this phase added that a person can see — every string, every control, every screen — has been driven once through the running application in a browser, not only through the test suite.

**Plans**: 5/5 plans executed

Plans:
**Wave 1**

- [x] 08-01-PLAN.md — migration `00047`, the fourth namespace in `internal/field/store.go`, and one snippet field traced from the table to a theme

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-02-PLAN.md — `OfSnippets`, `Move`, `validate`, the per-carrier `MaxFields`, and one minting point for every public entry point

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 08-03-PLAN.md — the field screen's fourth mode (`?textbaustein=`), the way into it, and the authorisation tests

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 08-04-PLAN.md — the snippet value form, its sanitisation proof, and T-07-26 closed

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 08-05-PLAN.md — fixtures, `TEMPLATE-SPEC.md`, the bundle round trip, the catalogues and the browser pass

**UI hint**: yes
**Research flag**: none — migration `00038` is a line-for-line template and its own comment explains the operation.
**Planning notes**:

- **The migration is `00047`.** Phase 7 takes `00046` for `darstellung` / `max_werte` / the `bereich` bounds. Migrations currently stand at `00045`.
- **Correct the stale note before writing it.** The index to swap, `idx_page_field_defs_kennung_oben`, was **already replaced by `00038:52–56`** — it is *not* still as `00029` wrote it, and the v1.5 planning note was one migration behind. Read **`00029` and `00038`** before writing `00047`. `00038:44–46` explains the operation in the file itself: *"ein Austausch von zwei Indizes und kein Tabellenneubau."* This is the third instance of a pattern already walked twice (`00037`, `00038`).
- Migration contents: `ALTER TABLE page_field_defs ADD COLUMN snippet_id` — **must default to NULL**; SQLite refuses `ADD COLUMN` carrying `REFERENCES` together with any other default — plus the index swap, plus `ALTER TABLE snippets ADD COLUMN fields`.
- **Build order:** ① the migration alone, so a rollback is one file → ② **`internal/field/store.go:53` first, before anything else in the phase** → then `Def.SnippetID`, `scanDef`, `Create`, `Update`, `Move`, `validate`; this whole step must be invisible to pages and blocks → ③ the `snippet` store carries `fields` → ④ the admin screen (half the phase's effort) → ⑤ theme surface + fixtures + spec → ⑥ bundle round trip.
- **`internal/field/store.go:53` is the highest-consequence single edit in the phase.** It reads `WHERE website_id = $1 AND block_type_id IS NULL`; missing `AND snippet_id IS NULL` puts **every snippet field on every page's edit form** and in every theme's `.Page.Feldliste`. Silent, and visible only in a browser. The carrier discriminator lives in hand-written SQL at **seven sites**; `:53` is the one that "already works", which is why it is the one that ships broken.
- **`.Site.Snippets map[string]template.HTML` must not change type.** It is a published contract at `TEMPLATE-SPEC.md:212` that every installed theme indexes. Add a **parallel** map (`Site.Bausteinfelder` / `Bausteinliste`) instead. The snippet's Markdown body stays: a snippet becomes body + optional fields, exactly as a page is content + optional fields — zero migration for existing content.
- `page_field_defs` is already a **three-namespace** table (page fields `parent_id IS NULL AND block_type_id IS NULL`, group sub-fields, block-kind fields). This phase adds the fourth.
- Open question to settle **in this phase**: `MaxFields = 60` is one budget shared by three — soon four — carriers, counted as `SELECT COUNT(*) … WHERE website_id = $1` at `internal/field/store.go:217`. Scope the count per namespace, or keep one shared budget? Decide and write it down.
- The theme contract is test-enforced: `sample_test.go:17` and `spec_test.go:18` fail the build if a new field on `PageContent`/`Site` is missing from `SampleData`/`MinimalData` or from `TEMPLATE-SPEC.md`.
- `internal/admin` sits at 14.7 % coverage and is where authorisation lives. Reuse `internal/admin/page_handler_test.go` as the harness; do not invent a second.

### Phase 9: CSV Import

**Goal**: Content can arrive as a table — an admin uploads a CSV beside the existing WXR and bundle importers, points each column at a target, sees what would happen before anything is written, and gets pages created through the ordinary path plus an honest per-row account of what happened.
**Depends on**: **Phase 7 only** — for the multi-value encoding and for the term field. Needs **nothing** from Phase 8, so 8 and 9 may run in parallel once Phase 7's step ① has landed.
**Requirements**: IMP-01, IMP-02, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08, IMP-09, IMP-10
**Success Criteria** (what must be TRUE):

  1. Uploading a `.csv` leads through four screens — file, mapping, dry run, report. On the first screen the admin chooses between an **existing** website and a **new** one, and both paths reach the same mapping screen.
  2. The mapping screen lists the file's columns against targets — title, body, any custom field of that website, or explicitly nothing. Columns whose names correspond are matched automatically, ignoring case and accents, and every automatic match can be overridden. One real row of the file is shown and can be stepped to the next, and each field can carry a default for cells that are empty or unmapped. An example CSV generated from that website's own field definitions, with the right column headings already in it, is downloadable.
  3. The admin can run the whole file through validation **without writing anything** and see per row what would be created, updated or skipped — with the row number and the reason — before committing to it.
  4. Confirming the import creates pages indistinguishable from hand-made ones — slug generated and unique per website, Markdown rendered and sanitised, draft unless a status column says otherwise — because the importer calls the same creation path as any other creation. A file mixing good and bad rows imports the good ones, skips the rest with a named reason, reports every slug that was renamed on collision, and leaves nothing half-written.
  5. A hostile file is refused rather than believed: bytes, rows and single cells are all capped; a byte-order mark from Excel does not break the first column; a stray quote does not swallow the rest of the file; a short row is reported by its row number; a NUL byte is refused. And **no transaction ever spans more than one row** — a file-long transaction is the anti-feature this criterion exists to forbid. A row that fails after its page was created is undone through the ordinary delete path, so nothing is left half-written.
  6. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and everything this phase added that a person can see — every string, every control, every screen — has been driven once through the running application in a browser, not only through the test suite.

> **Amended 2026-09-06 — Kriterium 5, vor der Planung.** Der Satz lautete bis
> hierher „the write is **one transaction per row, never one for the whole
> file**". Gemessen wurde: eine eingelesene Zeile umfasst heute schon **zwei
> bis drei** Transaktionen, und keine davon gehört diesem Importeur —
> `internal/page/store.go:461` `CreatePage` ist ein einzelnes, selbst
> abschliessendes `INSERT`, `internal/term/store.go:119` `SetForPage` öffnet
> bei `:120` sein eigenes `BeginTx`, `EnsureNames` bei `:330` ebenso. Sie zu
> einer zusammenzuziehen hiesse, einen `*sql.Tx` durch `page.Store` und
> `term.Store` zu fädeln — also **einen zweiten Anlegepfad neben dem
> gewöhnlichen**, und genau den verbieten IMP-02 und Kriterium 4 wörtlich
> („calls the same creation path as any other creation"). Die Hälfte, auf die
> es ankommt, bleibt unverändert und absolut: **keine Transaktion über mehr als
> eine Zeile** — das ist die Regel, die der Schreib-Pool mit
> `SetMaxOpenConns(1)` (`internal/db/db.go:28`) tragend macht. „Nichts halb
> geschrieben" wird stattdessen zweifach eingelöst: die ganze Zeile wird über
> `field.CheckAll` geprüft, **bevor** irgendetwas geschrieben wird, und
> scheitert ein späterer Schritt trotzdem, wird die Zeile über den
> gewöhnlichen Löschweg zurückgenommen (`TrashPage` `store.go:801`, dann
> `PurgePage` `:864` — dieselben zwei Schritte, die eine Betreiberin von Hand
> geht). Hergeleitet in `.planning/phases/09-csv-import/09-CONTEXT.md`, D-02.
>
> **Nachtrag 2026-09-06, von der Verifikation gefunden: dieser Stempel zaehlte
> die Transaktionen auf und hat eine uebersehen.** Neben den drei genannten
> oeffnet `term.EnsureNames` (`internal/term/store.go:330`) eine **vierte**, und
> das Zaehl-Tor dieser Phase konnte sie nicht sehen, weil es „kein Aufruf, den
> die *Zeilenfunktion* macht" prueft — und `EnsureNames` wird nicht von ihr
> gerufen, sondern einmal davor (`internal/admin/csvimport.go:819`).
>
> **Gemessen statt geschaetzt, und die Messung entscheidet es:**
> `csvimport.TermNames` (`internal/csvimport/row.go:160-174`) entdoppelt nach
> `page.Slugify`, bevor die Liste hinausgeht. Die Transaktion umfasst also den
> **eigenen Wortschatz der Datei** — bei fuenftausend Zeilen mit einer
> Sortenspalte sind das die Sorten, nicht die Zeilen — und je Name eine
> `INSERT … ON CONFLICT DO NOTHING`. Das ist nicht die dateilange Transaktion,
> die dieses Kriterium verbietet.
>
> **Diese Annahme war falsch und ist Stunden spaeter von der Sicherheitspruefung
> widerlegt worden — mit einer Messung.** Sie steht hier stehen gelassen statt
> ueberschrieben, weil der Denkfehler lehrreicher ist als das Ergebnis: ich habe
> von „entdoppelt nach Kuerzel" auf „begrenzt" geschlossen, **ohne zu fragen, ob
> der Wortschatz selbst begrenzt ist.** Er ist es nicht. `RowTerms`
> (`internal/csvimport/row.go:142`) deckelt bei `term.MaxPerPage = 12`;
> **`TermNames` (`:191`) deckelt gar nicht.** Der Vorlauf legt damit Schlagwoerter
> an, auf die keine Zeile je zeigen kann — eine Zeile speichert hoechstens zwoelf.
>
> Gemessen auf derselben Maschine, unter derselben 10-MB-Grenze:
>
> | | Dauer | laengste Wartezeit eines anderen Schreibvorgangs |
> |---|---|---|
> | Schlagwort-Vorlauf, 10,0-MB-Datei, 1 166 000 Namen in einer Transaktion | 10,75 s | **10 746 ms** |
> | die eigentliche Schreibschleife, 5000 Zeilen | 1,10 s | **3,2 ms** |
>
> Und die Sitzungen liegen auf demselben Schreib-Pool (`main.go:175`), also steht
> in diesen zehn Sekunden auch die oeffentliche Seite. **Das ist genau die
> Blockade, die dieses Kriterium verbietet, nur an einer Stelle, an der das
> Zaehl-Tor nicht hinsieht.** Der `BeginTx`-Grep misst 0, weil die Transaktion in
> `internal/term/` liegt: ein Tor, das einen Stellvertreter beweist und nicht die
> Zusage.
>
> **Behoben statt angenommen** — siehe `09-VERIFICATION.md` und die
> Luecken-Runde.

**Plans**: 6 plans, in 6 waves. The chain is the build order fixed in `09-CONTEXT.md` and it is genuinely sequential: `internal/csv` must be right before anything reads a file, the staging table must exist before a screen can carry a token, and `internal/admin/csvimport.go` is touched by plans 04, 05 and 06, so two of them in one wave would be two agents editing one file. Step 5 of that build order is split into two plans — the report screen and the standing gate are different kinds of work, and the browser pass must run **after** the code-review fix round, which no plan that also writes code can promise.

Plans:
**Wave 1**

- [x] 09-01-PLAN.md — `internal/csv`, pure and standard-library only: the hostile-file checklist D-08…D-16, the row-number helper, the example-CSV writer

**Wave 2** *(blocked on Wave 1)*

- [x] 09-02-PLAN.md — migration `00049_csv_imports.sql`, the staging store in `internal/csvimport`, and the `csv-import-prune` job

**Wave 3** *(blocked on Wave 2)*

- [x] 09-03-PLAN.md — the mapping resolution and the row function: everything between a parsed row and a `page.PageCreate`, with no screens

**Wave 4** *(blocked on Wave 3)*

- [x] 09-04-PLAN.md — the third `<details>` panel, screen 1, the mapping screen, the expiry screen and the example CSV download

**Wave 5** *(blocked on Wave 4)*

- [x] 09-05-PLAN.md — the dry run, the write, and the report grouped by reason

**Wave 6** *(blocked on Wave 5, and on the code-review fix round)*

- [x] 09-06-PLAN.md — the five catalogues and the browser pass

> **Note added at planning, 2026-09-06 — D-06's route table cannot be
> registered.** `GET /admin/websites/import-csv/{token}` conflicts with
> `GET /admin/websites/{id}/pages` under Go 1.22's `ServeMux` — both match
> `/admin/websites/import-csv/pages` and neither is more specific — so
> `newRouter` **panics at startup**. Verified by registering this tree's 145
> existing admin patterns beside D-06's five. Screen 1 stays at
> `POST /admin/websites/import-csv` beside its two siblings; the four
> token-bearing screens are registered under `/admin/csv-import/{token}`, which
> registers cleanly. Recorded in `09-04-PLAN.md`.

**UI hint**: yes — **the largest UI surface in the milestone.** The mapping screen and the dry-run report are the two screens users will judge the feature by.
**Research flag**: the **dry-run report screen**. 312 rows with 40 problems is a real information-design problem, not a layout question. Worth a UI-SPEC or a `/gsd-discuss-phase` pass on that screen alone.
**Planning notes**:

- **Depends on Phase 7's step ①, and on nothing in Phase 8.** A column mapped to a Multiple-Choice field must write what `internal/field` reads. **7 before 9, always** — if 9 shipped first it would invent an encoding that 7 then inherits. A `Sorte` column is one of the two motivating examples in `docs/offene-punkte.md`, which is the second reason 7 comes first: without the term field, Phase 9 either omits it or builds a throwaway path.
- **Posture: strict at the gate, best-effort at the till.** Validate every row *before* writing anything, then offer a "trotzdem importieren" escape hatch, then write **one transaction per row, never one per file**. The write pool is `SetMaxOpenConns(1)`; a file-long transaction blocks every request on the box, admin and public alike. An all-or-nothing single-transaction write is an explicit anti-feature here.
- **Carrying the file between steps: do not put the parsed table in the session.** SCS stores sessions in SQLite; a 2 MB CSV in a session row is a bad day. ~~**Re-submit the file with the mapping** — simpler, and needs no cleanup job.~~ **Overridden 2026-09-06, vor der Planung:** die zweite Hälfte lässt sich nicht bauen. Ein Server kann ein `<input type="file">` nicht füllen (die Auszeichnung sieht das aus Sicherheitsgründen nicht vor), und ein Formular-POST lädt ein **neues** Dokument — die Betreiberin müsste dieselbe Datei auf jedem Bildschirm erneut auswählen. Entscheidend ist **IMP-08**: „shows one real row from the file, navigable to the next" — jeder Schritt vorwärts ist eine Anfrage, also verlangte jeder Klick eine neue Dateiauswahl. Die Anforderung gewinnt gegen die Notiz. Gebaut wird stattdessen: die Datei wird nach Bildschirm 1 **serverseitig zwischengelagert** (eigene Tabelle, **rohe Bytes**, nie die geparste Tabelle — die eigentliche Sorge dieser Notiz bleibt damit gewahrt), die Bildschirme 2–4 tragen nur eine Marke und **kein Dateifeld**, und das Aufräumen ist ein `jobs.Job` neben den drei, die es schon gibt (`cmd/holzcloud/main.go:443-520`). Hergeleitet in `.planning/phases/09-csv-import/09-CONTEXT.md`, D-01.
- **Copy the three-layer shape of `internal/wxr` + `internal/admin/wordpress.go`**: pure parse → a one-row create returning a reason string → a handler that caps, parses, loops and reports. Copy the shape; improve the mechanism — `csv.Reader.Read()` genuinely streams where `wxr.Parse` materialises the whole document. `internal/csv` sits beside `internal/wxr`; a third `<details>` panel on `website_list.html` copied from `:24–45` hangs it on the existing screen.
- **New routes must be added to `TestRouteAuthorization`'s table at `main_test.go:158–173`.** Registering them in `newRouter` alone is not enough.
- **The user decided the import offers BOTH an existing website and a new one, chosen on screen 1.** Two paths, and **both must be exercised in the browser pass**. Targeting an existing website is a deliberate departure from `internal/admin/wordpress.go`'s stated rule that an importer always creates a new one; the rule's own reason — every collision needs an answer — is answered here by screen 1's update-or-skip choice plus the dry run. **Write this into the plan as a decision**, or a future reader sees an inconsistency.
- **`encoding/csv` has no limit anywhere.** Cap bytes (`http.MaxBytesReader`, 10 MB, matching `wordpress.go:25`), rows, and cells (100 000 bytes — one 9 MB cell is legal within the byte cap). **The BOM is not stripped**: the first header cell arrives as `"﻿Titel"` and never matches a mapping — the number-one CSV import bug. `LazyQuotes = true` for untrusted uploads. `FieldsPerRecord = -1` so a short row is a *reported row* with a row number rather than a line number. Blank lines are skipped silently and a quoted field may span lines, so **row count ≠ line count — count rows yourself**. A NUL byte passes straight through; check for it. And `page.CreatePage` silently renames colliding slugs to `-2`, `-3` — report every one.
- **The multi-value wire format across a CSV cell is a pipe**, not a newline: a newline inside a cell is legal RFC 4180 but must be quoted and is invisible in a spreadsheet. `strings.Join(strings.Split(cell, "|"), "\n")`. One line of code, one sentence of user-facing documentation — but it *is* user-visible, so it belongs in the example CSV's header row comment and in the mapping screen's help text.
- The dry run is cheap because `field.CheckAll` is already the one shared validator — the same validator the real write uses, which is what makes the dry run trustworthy.
- Explicit anti-features, already decided: no "delete rows missing from the file" (one mis-mapped key column deletes a website), and no downloading of images named by URL in a cell (nothing is fetched from a third party at runtime; `wordpress.go` already refuses this and lists the URLs instead).
- `internal/admin` is at 14.7 % coverage and is where authorisation lives. Reuse `internal/admin/page_handler_test.go` as the harness.

### Phase 10: Authentik Forward-Auth

**Goal**: Whoever the operator's Authentik has already signed in reaches the admin without a second sign-in — and every route by which that trust could be forged is closed before the first header is read, with the password path untouched behind it.
**Depends on**: Nothing in this milestone. Its file set is **disjoint** from every other phase's — it touches only `auth/`, `web/`, `config/` and `main.go` — so it can move earlier if a second worker is available. Scheduled last because it carries the milestone's close-out gate.
**Requirements**: SSO-01, SSO-02, SSO-03, SSO-04, SSO-05, SSO-06, SSO-07, SSO-08, SSO-09, SSO-10, SSO-11, QUAL-01, QUAL-02
**Success Criteria** (what must be TRUE):

  1. A person who has authenticated at Authentik reaches the admin without a second sign-in. Their group membership decides role and website access, re-applied at **every** sign-in so a demotion at the identity provider takes effect here, and every change of rights is written to the activity log.
  2. **The acceptance test, one command:** from a second machine, over IPv4 **and** IPv6, `curl -H 'X-authentik-email: …' http://<server>:8080/admin/` returns a **login form, not a dashboard**. The server binds to loopback by default; the peer is checked before any header is read; every inbound identity header including its underscore spelling is stripped at the top of the chain; and the proxy proves it is the proxy with a shared secret compared in constant time and kept in the environment rather than the database. An untrusted peer carrying no identity header still falls through to the ordinary password form — not to a refusal, because the way back in must not die with the proxy.
  3. An identity with no account is refused unless the operator has switched account creation on; with it on, the service **refuses to start** until it is told which website a new account belongs to. Silence does not mean "every website". The shipped Caddy example strips the client's own identity headers explicitly, and `DEPLOY.md` names the minimum Caddy version.
  4. An Authentik session satisfies the second-factor requirement — and because that makes this installation's second factor depend on the operator's Authentik enforcing one, the dependency is stated in `DEPLOY.md` **and shown in the admin**, not left in a source comment. Signing out signs the person out at Authentik too, so the next click does not silently sign them back in.
  5. With single sign-on switched off, nothing about signing in changes: the password path, the second factor, the recovery codes and the command-line way back in all behave exactly as they do today — proven by a browser pass run with forward-auth **disabled**.
  6. **Milestone close-out** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist` across everything v1.6 added, and every field kind from Phase 7, the snippet fields from Phase 8, **both** import paths from Phase 9 and the sign-on path here have each been driven once through the running application in a browser.

**Plans**: 5/10 plans executed, in 9 waves. The wave structure **is** the eight-step build order below, which is why the waves are mostly sequential: each step is independently shippable and reversible, and step ② is explicitly the security core that must exist before anything can rest on it. Only wave 6 runs two plans together — the second factor and the sign-out share no file. GSD's tracer-first default is deliberately overruled for the same reason the build order gives: a tracer that signs somebody in end to end before the peer gate exists is not a thin slice of the finished system, it is the vulnerability with a test asserting it works.

Plans:
**Wave 1**

- [x] 10-01-PLAN.md — the settings and the two refusals to start: the SSO block, the loopback listen address, and provisioning that will not start without a named default website (SSO-04, SSO-05, SSO-10)

**Wave 2** *(blocked on Wave 1)*

- [x] 10-02-PLAN.md — the trust boundary: peer before header, the unconditional strip of every spelling, the constant-time secret, and the signed assertion deliberately not built — wired outermost, above `RequestID` (SSO-02, SSO-03, SSO-04)

**Wave 3** *(blocked on Wave 2)*

- [x] 10-03-PLAN.md — the fourth caller of `completeLogin`: `RenewToken`, the account match, the non-ASCII e-mail refusal, and a gate for each of the seven things that must not change (SSO-01, SSO-09)

**Wave 4** *(blocked on Wave 3)*

- [x] 10-04-PLAN.md — provisioning, and the inversion asked about through `NewWebsiteAccessLookup` itself (SSO-05)

**Wave 5** *(blocked on Wave 4)*

- [x] 10-05-PLAN.md — group → role and website, re-applied every sign-in, with the whole-element match and the refusal that closes D-01's second road (SSO-06)

**Wave 6** *(blocked on Wave 5; two plans, disjoint file sets)*

- [ ] 10-06-PLAN.md — the second factor: one predicate, five call sites, and the dependency shown on two admin screens (SSO-07)
- [ ] 10-07-PLAN.md — the sign-out, and the CSP pair this shape does not need (SSO-08)

**Wave 7** *(blocked on Wave 6)*

- [ ] 10-08-PLAN.md — the deployment documents: the Caddyfile with one explicit delete per copied header, the minimum Caddy version, and the operator's two verifications (SSO-07, SSO-11)

**Wave 8** *(blocked on Wave 7)*

- [ ] 10-09-PLAN.md — the catalogues, closed before the browser pass opens (QUAL-01)

**Wave 9** *(blocked on Wave 8, and on the code-review fix round)*

- [ ] 10-10-PLAN.md — three browser passes: forward auth on, forward auth off, and the v1.6 milestone close-out (SSO-01, SSO-02, SSO-08, SSO-09, QUAL-01, QUAL-02)

**Research flag**: **the whole phase wants `/gsd-discuss-phase`** — not for lack of research (the authentik contract is verified from its own source at two release tags) but because every remaining question here is a **policy decision**, not a lookup: what an SSO user with no matching website group gets, what `X-authentik-uid` looks like in the operator's own instance, and which Caddy the operator actually runs.
**Planning notes**:

- **The four-layer trust boundary, in this order.** They are not alternatives; each covers a different failure of the one before.
  1. **Validate the peer address first, before reading any header.** Reuse `web.ClientIPResolver.IsTrustedPeer(req)` (`internal/web/clientip.go:50–52`) — it reads `req.RemoteAddr`, which `net/http` sets from the accepted connection and a client cannot spoof; it is fed by `cfg.TrustedProxies` (default `127.0.0.1/32,::1/128`) and is already tested. **Reuse it — do not write a second trusted-peer check.** An **untrusted peer carrying no identity header must fall through to ordinary password login, never a 403**, or break-glass dies with it. So: untrusted peer → strip every identity header and continue as anonymous.
  2. **Strip inbound copies of every identity header unconditionally at the top of the chain — in the app, not only in Caddy** — including the underscore alias (`X_authentik_email`), because Go canonicalises `-` but not `_` and they are two distinct map keys. Three lines that cannot fail. This makes a wrong Caddyfile on someone else's server a misconfiguration rather than a bypass.
  3. **A shared secret** the proxy adds, compared with `crypto/subtle.ConstantTimeCompare`, kept in the **environment and out of the database** — as `PayrexxSecret` already is, because the database is what gets copied into every backup. This survives a peer-check mistake.
  4. **A signed assertion — deliberately NOT implemented for v1.6, with the reason recorded in code** so it is not relitigated.
- **The precedent to copy for layers 1–2 is exact: `web.RequestID` (`internal/web/logging.go:31–45`)** — `IsTrustedPeer` → sanitise → use. Note the distinction the codebase already draws: `/ai`'s `Authorization: Bearer` is a **secret the server verifies independently**; a forward-auth header is a **claim** whose only guarantee is who the peer is. Nothing in the codebase today trusts a header's claim for identity; this phase introduces the first one.
- **Where the middleware goes:** `cmd/holzcloud/main.go:1036` *(cited as `:968` until 2026-09-07)*. The admin chain there is **hand-nested, not built with `auth.Chain`** — the middleware goes in by editing that one line, **between `setupGuard` and `requireAuth`**, so that `RequireAuth` → `RequireSecondFactor` → `RequireAdmin` → `RequireWebsiteAccess` all run unchanged.
- **A forward-auth handler must be `completeLogin`'s FOURTH caller and nothing else** (`internal/admin/login.go:106`) — it is the single funnel where a session becomes signed in — **preceded by `sm.RenewToken`** for session fixation, or the SSO sign-in is invisible in `/admin/protokoll`.
- **Deliberately unchanged. If any of these need changing, the design is wrong:** `auth.RequireAuth`, `auth.RequireAdmin`, `auth.RequireWebsiteAccess`, `auth.RequireSecondFactor`, `completeLogin`'s body, `user.Rights` / `SetRights`, and the `users` schema.
- **Do NOT invent a third role.** `users.role` carries a table-level `CHECK (role IN ('admin','editor'))` at `00001:7`, and loosening a table-head CHECK in SQLite means a full rebuild of a table with foreign-key children.
- **`NewWebsiteAccessLookup` (`internal/admin/handler.go`) — `return assigned == 0 || mine > 0`. *(Cited as `:173` until 2026-09-07; the code is exactly as described, the line had moved.)*** "No assignment means every website" is correct for manual account creation and **inverts** under auto-provisioning: a freshly created SSO account has zero `user_websites` rows by construction, so the first stranger who authenticates at the IdP gets editor access to every website in the installation. This is the single most likely way this milestone ships a real vulnerability, because every part looks correct in isolation. **Do not change `NewWebsiteAccessLookup`** — that would lock out every existing editor. Instead: auto-provisioning **off by default**, and if on, refuse to start without an explicit default website. The project already refuses to start on a half-configured Payrexx pair; copy that.
- **The `form-action 'self'` trap.** `adminCSP` (`internal/web/headers.go:17`) carries `form-action 'self'`. `HandleLogout` is a **POST** (`internal/admin/login.go:128–136`, redirect at `:134`), and a redirect issued in response to a form submission is checked against `form-action` by some browsers — Safari among them. **This codebase already documents the mechanism from the Payrexx incident at `headers.go:41–53`**: *"the TWINT payment on an iPhone dies silently at the moment of the handover, and nothing in the server log says why."* The same-host `/outpost.goauthentik.io/sign_out` route keeps this same-origin and safe, but it breaks silently the moment a separate outpost host is supported. **Precedent for the fix: `PublicCSP(extraFormAction ...string)` + `SecureHeadersWith(csp)` (`headers.go:59–67`, `:82`, wired at `main.go:1132–1134` *(cited as `:1062–1066` until 2026-09-07)*)** — mirror it as `web.AdminCSP` / `web.AdminHeadersWith` at both `main.go:1004` and `:1036` *(cited as `:936`/`:968` until 2026-09-07)*, keeping `frame-ancestors 'none'`, `X-Frame-Options: DENY` and `Cache-Control: no-store` (asserted by `main_test.go:223–238` *(cited as `:220–234` until 2026-09-07)*). The cheaper alternative that avoids the CSP change entirely: answer the POST with a local `/admin/abgemeldet` page carrying a plain `<a>` — navigation by link is not checked against `form-action`.
- **Caddy CVE-2026-30851 (GHSA-7r4p-vjf4-gxv4).** `forward_auth … { copy_headers X-Foo }` generates a *conditional* set that only fires when the auth service returns `X-Foo`, and **no delete** for the client's inbound `X-Foo`. When the outpost answers 200 without that header — anonymous route, user with no email, any header authentik chose not to emit — the client's own value reaches the backend verbatim. Affected **v2.10.0–v2.11.1**, fixed in **v2.11.2**; latent since Nov 2024, so it is in whatever Caddy a typical operator has from the stable apt repo. `deploy/Caddyfile.example` needs an explicit `request_header -X-Authentik-…` line **per copied header**, plus the shared secret, and `DEPLOY.md` must state a minimum Caddy version of **2.11.2**. Companion advisory GHSA-f59h-q822-g45g / CVE-2026-52845 is the underscore variant and is why layer 2 must cover aliases.
- **USER DECISION, taken 2026-09-03: an Authentik session satisfies the second factor, unconditionally.** The change has **exactly one home — `auth.MustHaveSecondFactor` at `internal/auth/twofactor.go:44`** (`return role == "admin"`; one function, one caller at `:70`). Change it there or nowhere; a likely shape is `MustHaveSecondFactor(role string, viaSSO bool)` with a session key recording how the session was established. Because that decision makes this installation's second factor depend on the operator's Authentik enforcing one, **SSO-07 requires the dependency to be stated in `DEPLOY.md` and shown in the admin**, not left in a comment.
- **Build order, each step independently shippable and reversible:** ① config + a no-op middleware wired at `main.go:968` while it does nothing → ② **peer-gated header read — the security core, before anything can rely on it**; test *first* that an untrusted peer's header is ignored → ③ look up an existing user by e-mail, `RenewToken`, then `completeLogin` (no provisioning yet) → ④ provisioning (a random Argon2id hash into `users.password`; no migration needed, `00001:6` has no CHECK) → ⑤ group → rights sync, re-applied every sign-in (`SetRights` replaces wholesale, so it is idempotent) → ⑥ the TOTP policy, in `MustHaveSecondFactor` alone → ⑦ sign-out + the CSP pair (parallelizable from step ②) → ⑧ deployment docs + a browser pass **including a run with forward-auth disabled**.
- **The authentik contract, verified from source** at tags `version/2026.8.1` and `version-2025.8` (`internal/outpost/proxyv2/application/mode_common.go`, `getHeaders`): headers are `X-authentik-username`, `-email`, `-name`, `-uid` (the OIDC `sub`), `-groups`, `-entitlements`, `-jwt`, `-meta-*`. **`X-authentik-groups` joins group names with `|` (U+007C)** — from the source, not a blog; an empty header means no groups, so guard against `[""]`. Sign-out is `/outpost.goauthentik.io/sign_out`, routed on the application's own domain, therefore same-origin. Always `r.Header.Get`, never map indexing.
- **Do not verify the JWT, and do not implement a half version.** Verifying means fetching JWKS at runtime (the one rule this project does not break) or pinning a key. Without a signing key authentik signs proxy tokens *symmetrically with the client secret*, so "verify" means two modes plus holding the secret. The signature protects nothing the transport does not: if an attacker can set headers on that socket, they can talk to the CMS directly. Parsing a JWT without verifying its signature is **strictly worse** than not having it, because it looks like a defence.
- **Other pitfalls that must land in the plan:** logout that does not log out (the browser still holds the authentik cookie and is signed straight back in — redirect to the outpost's `sign_out`, build the target from the **request's own host, never from a header**, reusing `auth.backTo` / `auth.SafeReturn`; and use `HX-Redirect` + `Vary: HX-Request`, because logout is an htmx POST). **CSRF stays on every route** — forward auth makes CSRF *more* relevant, not less, because it adds a second ambient credential the browser attaches automatically. Group → role mapping must split on `|` and compare **whole elements** (a naive `strings.Contains` matches `not-holzcloud-admins`), must run on every request so demotion works, must log every role change, and the admin-group variable must have **no default**. E-mail is not a stable identity: SQLite's `COLLATE NOCASE` folds ASCII only, so `Müller@` and `müller@` are two rows and two admins. And the password path must survive, because `auth.RequireFreshPassword` (`elevate.go:57`) guards website deletion, user deletion, AI key creation and plugin removal by asking for a password an SSO account does not have.
- **Verify in the operator's own instance before pinning:** `X-authentik-uid`'s format depends on the provider's Subject mode (default: a hashed identifier) — MEDIUM confidence. Pin to username instead if unsure, and document that subject mode must not change after users are mapped.

### Phase 11: Galerie

**Goal**: A picture stops being a dead end. A visitor can enlarge one and page through the rest; an editor assembles an album once and uses it on several pages; and a gallery can be paged through instead of scrolled past — with no JavaScript anywhere, on a public page the template rules already forbid it on.
**Depends on**: **Phase 7**, not Phase 10 — an album is a list of images, and it must read and write that list through Phase 7's `SplitValues`/`JoinValues` (FIELD-07) rather than inventing a second spelling. Phase 7 also supplies the lesson this phase would otherwise learn the hard way: a reusable thing crossing the bundle needs its own translation, which is exactly where Phase 7's Term field first failed. Independent of Phases 8, 9 and 10.
**Requirements**: GAL-01, GAL-02, GAL-03, GAL-04, GAL-05, GAL-06, GAL-07
**Success Criteria** (what must be TRUE):

  1. A visitor can tap a picture in a gallery and see it large, with its caption, and get back — **and the browser's back button is what gets them back**, because the enlargement is a `:target` state and not a scripted overlay. From the large view, next and previous move through the gallery without returning to the grid first. Nothing about this needs JavaScript, and `internal/tmplmgr/script.go` would reject the scripted version in an uploaded template anyway.
  2. An editor assembles a **named album** once for a website and places it on several pages — the way menus and terms already work. Changing the album changes every page that carries it, without touching those pages. An album belongs to exactly **one** website and is invisible from every other, like every other resource in this CMS.
  3. **An album survives the bundle round trip, including after a rename.** This is the criterion that is easy to declare and easy to get wrong: the manifest carries a term by its *name* and re-derives the slug on import (`internal/bundle/format.go:152–159`, a deliberate and compatibility-bearing choice), and a reusable album must be translated on the way out and back the way `translateOut`/`translateIn` already translate `KindRef` and `KindImage`. The proving test **renames the album before exporting** — an un-renamed album round-trips even when the translation is missing, so a test without the rename proves nothing.
  4. A gallery can be shown as a **slideshow** instead of a grid: pictures side by side, snapping to their edges, reachable by keyboard and by touch. CSS `scroll-snap` does the whole job; the choice between grid and slideshow is a display mode on the block, the same shape Phase 7 gives `auswahl` with `darstellung`.
  5. The gallery block and the album read and write their image list through **one** mechanism, inherited from Phase 7 — not a second one. The existing `hc-galerie` markup, its `srcset`/`sizes`, its focus-point cropping and its own-aspect-ratio rendering from version 1.8 all keep working unchanged; an album is a new source for that list, not a new renderer.
  6. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and everything this phase added that a person can see — the large view, the album screens, the slideshow — has been driven once through the running application in a browser, not only through the test suite.

**Plans**: 7/7 plans executed, in 5 waves. Two independent verticals start together — the lightbox (renderer, stylesheet, specification) and the album (migration, store) share no file and neither can disprove the other. They meet in wave 3, where the album's pictures are expanded into a page at request time; the bundle round trip follows in wave 4 because it needs both the store's `Rename` and the block's album field; and the standing gate is last, because the browser pass must run **after** the code-review fix round, which no plan that also writes code can promise.

Plans:
**Wave 1**

- [x] 11-01-PLAN.md — the tracer: the renderer learns its position, one picture opens large with next/previous/close, the `:target` rules, and `TEMPLATE-SPEC.md` starts requiring `/assets/bausteine.css` (GAL-01, GAL-02)
- [x] 11-02-PLAN.md — migration `00050_albums.sql` and `internal/album`: the two-table shape from `menus`, the website scoping from `terms` (GAL-05)

**Wave 2** *(blocked on Wave 1)*

- [x] 11-03-PLAN.md — the admin area: nine routes, two screens, the navigation entry, and the album filed as content rather than admin-only (GAL-05)
- [x] 11-04-PLAN.md — the display mode: a new `Block.Display` field with all six of its landing places, and the `scroll-snap` slideshow (GAL-06)

**Wave 3** *(blocked on Wave 2)*

- [x] 11-05-PLAN.md — the album on a page: the marker at save, the expansion at request time, and the property test that changes an album without touching the page (GAL-03, GAL-07)

**Wave 4** *(blocked on Wave 3)*

- [x] 11-06-PLAN.md — the bundle round trip: the album in the manifest by name, and the proving test that renames before exporting (GAL-04)

**Wave 5** *(blocked on Wave 4, and on the code-review fix round)*

- [x] 11-07-PLAN.md — the standing gate: five catalogues, four glossary entries, and the browser pass with the unstyled case and the scroll-width measurement (QUAL-01, QUAL-02)

**UI hint**: yes
**Research flag**: **the lightbox markup only.** The `:target` pattern is well known but its accessible shape is not obvious — focus handling without script, what a screen reader announces when the overlay appears, and whether next/previous should be `<a>` elements pointing at sibling ids (they should). Worth a **UI-SPEC**. The album is a straight copy of an existing pattern (menus and terms are both "a named thing a website owns, assembled once, used in many places") and the slideshow is one CSS block.
**Scope note**: this phase does **not** fit the v1.6 milestone goal sentence, which is about the content model and about access. It was added to v1.6 by explicit developer decision on 2026-09-05, scheduled last, depending on nothing that Phases 8–10 produce and blocking nothing they need. Either extend the milestone goal or move this phase to v1.7 — but do not leave a later reader to discover the mismatch on their own.
**Planning notes**:

- **What already exists, measured rather than assumed.** `internal/block/block.go:46` defines `TypeGallery = "galerie"` ("Mehrere Bilder als Raster", `HasItems: true`); `internal/block/render.go:157–175` renders it as `<div class="hc-block hc-galerie hc-spalten-N">` of `<figure class="hc-galerie__bild">` with `srcset`/`sizes`, an optional `<figcaption>` and focus-point cropping; `cmd/holzcloud/assets/bausteine.css:106–144` carries the grid (`auto-fit`/`minmax`, 2/3/4 columns). Since version 1.8 each picture keeps its own aspect ratio. **Everything this phase adds happens *after* the grid** — none of the above is being rebuilt.
- **The media package is already strong and is not the work here:** `internal/media/` has `crop.go`, `responsive.go` (variants), `strip.go` (EXIF), `usage.go` (where a file is used), `variant_store.go` and `mp4.go`. The large view should serve an existing large variant, not mint a new size.
- **Deliberately excluded: richer layouts.** Masonry columns and justified rows were offered and declined on 2026-09-05. The grid stays as it is.
- **The lightbox must be `:target`, not `<dialog>`.** `dialog.showModal()` is JavaScript, and CLAUDE.md permits htmx only, as enhancement. `:target` costs one anchor per picture and one CSS rule, works with the back button for free, and is the only one of the two that can also live in an uploaded template — `internal/tmplmgr/script.go` rejects the scripted version by design.
- **A second gallery already exists and must not be forgotten:** the shop's product gallery (`internal/shop/product.go`, `.Product.Gallery`) is rendered by all seven themes as `.product__gallery`. Decide explicitly whether it inherits the large view — and record the decision either way, because "we only meant the block" is exactly the kind of silence that becomes a bug report.
- **An album pays the same tax terms and menus pay:** a table, a migration, an admin area, website scoping, the bundle round trip, `TEMPLATE-SPEC.md` + `SampleData` + `MinimalData` if a theme can reach it, and `tools/i18n -write` followed by `-schweiz`. Budget it once. Migrations will stand at `00047` after Phase 8; this phase should claim its number when it is planned, not before.

### Phase 12: The Codebase Speaks English

**Goal**: A stranger can read this repository. Every identifier, every comment, every test name, every SQL column and every catalogue key is English — and the template data contract is too, which is why the release carrying this phase is **2.0** and not 1.10.
**Depends on**: **Phase 9 and Phase 11** — not for a mechanism, but because a sweep over 84 753 lines of Go must not run while two phases are still adding to them. Phase 10 is independent of this one and may go either side of it.
**Requirements**: LANG-01, LANG-02, LANG-03, LANG-04, LANG-05, LANG-06, LANG-07, LANG-08
**Success Criteria** (what must be TRUE):

  1. `grep -rn '[äöüÄÖÜß]' --include='*.go' .` prints **nothing outside the catalogue files**. Every Go comment reads as English prose written for a reader, not as a machine translation of a German sentence — and every comment that carried a *reason* still carries it, at the same length. This repository's comments explain why a bound exists and what once went wrong; that is the most valuable thing in the files and an English reader needs it more, not less.
  2. Every identifier is English, and each German term maps to **exactly one** English word, fixed in `.planning/GLOSSARY.md`. `Kennung` is `key` everywhere or the phase has failed at its own purpose. A term translated during this phase that the glossary does not name is added to it in the same commit.
  3. **The template data contract is English and the break is announced.** `.Page.Felder` → `.Page.Fields`, `.Site.Bausteinfelder` → `.Site.SnippetFields`, `.Page.Uebersetzungen` → `.Page.Translations`, `.Page.Art` → `.Page.Kind`. All eight shipped themes are converted with it, `TEMPLATE-SPEC.md` documents only the English names, `CHANGELOG.md` names it as a breaking change under `## 2.0`, and the three places tied by tests — `internal/template/loader.go`, `SampleData`/`MinimalData` in `sample.go`, and the specification — agree. **A theme written against 1.x stops working, deliberately and in one release.**
  4. The German SQL column names are English, through **new** migrations — `kennung` → `key`, `art` → `kind`, `auswahl` → `choices`, `pflicht` → `required`, `hinweis` → `hint`, `gilt_fuer` → `applies_to`, `beschriftung` → `label`, `bedingung` → `condition`, `darstellung` → `display`, `min_wert`/`max_wert` → `range_min`/`range_max`, `max_werte` → `max_values`. No released migration is edited. Every hand-written SQL statement follows, including the six-statement carrier discriminator in `internal/field/store.go` that Phases 7 and 8 both had to reason about — and the `Down` half of each new migration is written with the care `00048` documents.
  5. **The catalogue's source language is English.** The 1158 German keys become English keys; a new `de.json` carries German as a translation; `de-CH.json` is derived from that instead of from source; `es/fr/it.json` are re-keyed through the old German→English mapping. The nine keys that collide are resolved individually, and **the three that collide because the existing translation is wrong are fixed, not merged** — `Beschriftung` and `Schlagwort` are not the same word, `Uhrzeit` and `Zeitpunkt` are not the same word, and `an.`/`fest.` translated to a bare `.` is a defect this phase inherits and repairs.
  6. **The stored German vocabularies are decided explicitly, not swept in and not silently left.** Measured 2026-09-06: only two CHECK constraints in 49 migrations carry German values, both from `00049`. But about **25 German strings in Go are stored data**, not identifiers — every field kind (`text`, `langtext`, `mehrfachauswahl`, `gruppe`, `schlagwort`, …), `gilt_fuer`'s three values, `knopfreihe`, and the seven block kinds. They sit in every existing database, they travel in every bundle ever exported, and the block kinds appear as CSS classes in all eight themes (`hc-aufruf` 20×, `hc-zitat` 13×, `hc-video` 13×, `hc-bildtext` 11×, `hc-karten` 7×, `hc-galerie` 6×, `hc-trenner` 4×). Turning them is a **data migration plus a bundle-compatibility question plus a second theme break** — a project of its own, not a rename. This phase writes down which way it goes and why. Leaving them German is a defensible answer; leaving them undiscussed is not.
  7. **German cannot come back unnoticed.** A mechanical gate fails the build if a German word enters Go source outside the catalogue files — the same shape as the existing i18n and CI gates, not a review convention. Without it the next phase writes German again and this one was a one-time cleanup instead of a change of language.
  8. **Standing gate** (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist` on every catalogue **after the source flip**, and every screen a person can see has been driven once through the running application in a browser — because renaming a template contract field that a screen reads is exactly the change no test catches and every visitor does.
  9. **Every operator-facing string is *collectable*, and the gate says so.** Measured 2026-09-08 by an adversarial sweep (`.planning/audits/v1.6-I18N-828.md`): **828 strings a person can read are in no catalogue at all** — not untranslated, *unreported*, because `tools/i18n` cannot see them. 403 lie outside its two directories (274 in `plugins/`, 116 in the public themes); the other 425 lie inside and are invisible anyway, because a sentence built with `fmt.Sprintf`, joined with `+`, returned from a helper, or surfaced through `err.Error()` is not a string literal at the argument index the collector reads. **This criterion is what turns criterion 5 from a re-keying into a translation that is actually complete**, and criterion 7's mechanical gate must grow a second half: not only "no German enters Go source", but "no operator-facing string is minted in a shape the collector cannot see". The two are different failures and only the first has a gate today.

**Plans**: TBD
**UI hint**: no new UI. But **every existing screen is in the blast radius** of criterion 3, so the browser half of the standing gate is larger here than in any phase that adds screens.
**Research flag**: none. Nothing here is unknown; it is large, and the risk is inconsistency rather than difficulty. The glossary exists to make it mechanical.
**Scope note**: this phase does **not** fit the v1.6 milestone goal sentence any more than Phase 11 does. It was added on **2026-09-06** by explicit developer decision — the project is open source now, so the code speaks English. It carries a **breaking change to a public contract**, so whatever milestone holds it, the release is **2.0**. Either extend the milestone goal, or move this phase and Phase 11 into a v2.0 milestone — but do not leave a later reader to find the mismatch on their own.
**Planning notes**:

- **Measured 2026-09-06, before planning.** 325 Go files, 84 753 lines. **191 files carry umlauts**, 2797 lines of them; German identifiers without umlauts add more (`Seite` 423×, `Feld` 274×, `Zeile` 154×, `Kennung` 153×). 37 of 868 test functions are plainly German. 12 German SQL column names across the migrations, `kennung` 26×. All eight themes read the German template contract. 1158 catalogue keys are German sentences and **there is no `de.json`** — German is the source language, which is exactly why criterion 5 is a re-keying and not a translation.
- **The catalogue flip is a bijection through `en.json`, and that is what makes it tractable.** Today `en.json` maps German→English; inverting it gives English→German, which *is* the new `de.json`. `es/fr/it.json` are re-keyed by looking each old German key up in `en.json`. Measured: of 1158 keys exactly **nine** English values are hit twice or more. Everything else is a clean 1:1. Do the transformation with a script, commit the script, and keep it — a second pass will want it.
- **Order matters and is not obvious.** The catalogue flip (criterion 5) must come **after** every user-visible German string has stopped moving, or the mapping is built against a moving target. The SQL rename (criterion 4) must come **before** the comment sweep touches `internal/field/store.go`, or the same file is rewritten twice. The template contract (criterion 3) is independent of both and should land in its own commit with the themes, because it is the one thing a user notices.
- **The 828 change the shape of this phase, not only its size.** Criterion 5 was written as a bijection through `en.json`, and it still is — for the 1158 keys that *are* in the catalogue. The 828 that are not have no counterpart to invert: each one has to be moved into a collectable shape first, and only then does it have a key to flip. So the order gains a step at the front, and it is the expensive one. Three bundles, cheapest first: **140 strings inside the collector's reach** (57 store errors surfacing through `err.Error()`, 83 assembled with `fmt.Sprintf` or `+`) are mechanical — the sentence moves from `errors.New` to a catalogue call at the caller, or from `fmt.Sprintf` into a `{{tf}}` with arguments. **154 shop-admin strings** are six screens that simply never went through a translation pass: large, easy, no decisions. **403 outside the reach** are not cleanup at all but two product decisions — does a plugin get a translation channel, and does a theme — both written up in `.planning/audits/v1.6-I18N-REICHWEITE.md`. The third bundle can be answered "no, and here is why" without blocking the first two.

- **Do not translate `.planning/`.** It is the project's own record, written as it was written, and rewriting history's language is how a record stops being one. New planning artifacts from this phase onward are English; the old ones stay.
- **The comment sweep is the part that can be done badly.** A translation that turns *„Die Spalte darf keinen anderen Vorgabewert als NULL haben — SQLite lässt ALTER TABLE ADD COLUMN nicht zu, wenn REFERENCES und ein Vorgabewert ungleich NULL zusammentreffen"* into *„Column has no default"* has destroyed the file. Budget for reading each comment and re-arguing it in English, not for find-and-replace.
- **Rename in one commit, translate in the next.** Learned on 2026-09-06 while turning `internal/csv`: `git mv` was used, but git infers renames by content similarity *at read time*, and a file that is renamed **and** rewritten in the same commit changes by more than half. `git log --follow` then loses the lineage unless the reader knows to pass `-M30%`. This phase renames far more than two files; splitting the two operations costs one extra commit per file and keeps `--follow` working at any threshold.
- **A German word that is *stored* is not an identifier — it is a value, and the glossary does not apply to it.** `.planning/GLOSSARY.md` carries this rule and the measurement behind it. It was written after a near miss: the glossary lists `übergehen` → `skip`, and `00049` pins `kollision IN ('uebergehen', 'aktualisieren')` — translating it in Go **compiles cleanly** and is then refused by SQLite at runtime, on a path no test in Phase 9 exercises. That was the one place where a plausible reading of the instructions produced a production bug instead of a compile error. Every sweep in this phase must be able to tell the two apart.
- **`internal/csv` and `internal/csvimport` were already turned**, on 2026-09-06, immediately after Phase 9's waves 1 and 2 shipped them — while nothing depended on their names. Phase 9's waves 3–6 are written in English from the start. So this phase inherits a tree that is already partly converted, and its first task is to **measure what remains** rather than to assume the numbers above still hold.
- **`internal/db/migrations/00049_csv_imports.sql` carries German comments and was deliberately left alone**: it had already run on a database when the decision was taken, and a released migration is never edited. Its comments belong to criterion 4's new-migration work or to nothing.

---

## Progress

**Execution Order:** 6 → 7 → (8 ∥ 9 ∥ 11) → 10 → 12

Two phases need Phase 7's multi-value encoding — Phase 7's build-order step ①:
**Phase 9** for the importer and **Phase 11** for an album's image list. Once that
has landed, Phases 8, 9 and 11 are independent of each other and may run in
parallel. Phase 10's file set is disjoint from every other phase's and can move
anywhere.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 2/2 | Complete | 2026-04-14 |
| 2. Auth + Admin Shell | v1.0 | 2/2 | Complete | 2026-04-14 |
| 3. Multi-Site + Pages + Public Rendering | v1.0 | 3/3 | Complete | 2026-04-14 |
| 4. Templates + Menus + Media | v1.0 | 3/3 | Complete | 2026-04-14 |
| 5. Admin Polish + Users + Deployment | v1.0 | 3/3 | Complete | 2026-04-14 |
| 6. Aufräumen | v1.6 | 7/7 | Complete | 2026-09-04 |
| 7. Field Kinds | v1.6 | 7/7 | Complete | 2026-09-06 |
| 8. Snippets Carry Fields | v1.6 | 5/5 | Complete | 2026-09-06 |
| 9. CSV Import | v1.6 | 6/6 | Complete | 2026-09-07 |
| 10. Authentik Forward-Auth | v1.6 | 5/10 | In Progress|  |
| 11. Galerie | v1.6 | 7/7 | Complete | 2026-09-08 |
| 12. The Codebase Speaks English | v1.6 | 0/TBD | Not planned | - |

---

## Coverage

All 48 v1.6 requirements are mapped to exactly one phase.

| Phase | Requirements | Count |
|-------|--------------|-------|
| 6. Aufräumen | MAINT-01, MAINT-02, MAINT-03, MAINT-04, MAINT-05 | 5 |
| 7. Field Kinds | FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06, FIELD-07, FIELD-08 | 8 |
| 8. Snippets Carry Fields | SNIP-01, SNIP-02, SNIP-03, SNIP-04, SNIP-05 | 5 |
| 9. CSV Import | IMP-01, IMP-02, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08, IMP-09, IMP-10 | 10 |
| 10. Authentik Forward-Auth | SSO-01, SSO-02, SSO-03, SSO-04, SSO-05, SSO-06, SSO-07, SSO-08, SSO-09, SSO-10, SSO-11, QUAL-01, QUAL-02 | 13 |
| 11. Galerie | GAL-01, GAL-02, GAL-03, GAL-04, GAL-05, GAL-06, GAL-07 | 7 |

**Mapped: 55 / 55. Orphans: 0. Duplicates: 0.**

QUAL-01 and QUAL-02 are counted once, in Phase 10, and additionally enforced
verbatim as the final success criterion of Phases 6, 7, 8, 9 and 11 — see
*Standing Gates* above.

**Migration numbers claimed by this milestone.** Migrations stand at `00045`.
Phase 7 takes **`00046`** (`darstellung`, `max_werte`, the `bereich` bounds);
Phase 8 takes **`00047`** (`snippet_id` + the index swap + `snippets.fields`).
Phases 6, 9 and 10 need none.

---
*Roadmap for v1.6 created 2026-09-04. The v1.0 section is retained for
phase-number continuity; the superseded v1.5 section it replaced described the
same middle three phases under the numbers 6–8.*
