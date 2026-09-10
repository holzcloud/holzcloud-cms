---
phase: 07-field-kinds
plan: 02
subsystem: database
tags: [go, sqlite, goose, migration, html-template, field-kinds, bundle]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "07-01's KindMulti and the multi-value encoding — validate's clearing rule names KindMulti, and the round-trip tests use it"
provides:
  - "migration 00046: page_field_defs.darstellung, max_werte, min_wert, max_wert — four own columns, no constraint at the column head, a Down that reverses all four"
  - "Def.Display, Def.MaxValues, Def.RangeMin, Def.RangeMax"
  - "field.DisplayButtons and Def.IsButtonRow() — the button-row predicate minted at exactly one place, for 07-06's template and switchOf"
  - "field.ErrRangeInverted — the sentinel the definition screen hangs its own sentence on"
  - "all seven page_field_defs SQL sites in store.go carrying the four columns, with the five SELECT lists byte-identical to each other"
  - "validate: negative MaxValues refused, inverted numeric bounds refused, Display cleared for a non-Choice and MaxValues for a non-Multi"
  - "the four definition-form controls, with no JavaScript"
  - "bundle Field.display / max_values / min / max, all omitempty, carried at three export and three import sites"
affects: [07-03, 07-04, 07-05, 07-06, 07-07, 08-snippets, 09-csv-import]

actuals:
  tokens: 46027
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Five copies of one SELECT column list are edited as one replace-all, never five times by hand — a divergence between them is the silent-zero failure the whole plan guards against"
    - "New INSERT/UPDATE placeholders are appended ($12…$15) rather than renumbering an existing non-sequential list — the renumbering is the risk, not the addition"
    - "A behavioural read test over every read path, not a grep count: a SELECT can name a column and still never write it into the struct"
    - "A validation reason the screen must reword becomes an exported sentinel, so errors.Is reaches it — the store's sentence stays short, the screen's stays long"

key-files:
  created:
    - internal/db/migrations/00046_field_kinds.sql
    - internal/field/store_test.go
    - internal/admin/field_defs_test.go
  modified:
    - internal/field/field.go
    - internal/field/store.go
    - internal/admin/field.go
    - cmd/holzcloud/templates/admin/field_list.html
    - internal/bundle/format.go
    - internal/bundle/export.go
    - internal/bundle/import.go
    - internal/bundle/bundle_test.go

key-decisions:
  - "D-14 auto-selected as four-columns (workflow.auto_advance is true, gate=blocking). Matches the recorded decision verbatim; the door is one-way and wants a human glance."
  - "The plan's behaviour bullet 'after Create with all four set, Get returns all four unchanged' contradicts its own clearing rule — no kind keeps both Display and MaxValues. Resolved in favour of the clearing rule; the test uses a Choice/Multi pair per read path instead, which covers all four columns on every path."
  - "The four new columns go BETWEEN bedingung and COALESCE(block_type_id, 0) — the acceptance criterion forbidding the old SELECT shape fixes the position, it is not free."
  - "ErrRangeInverted is an exported sentinel rather than an inline errors.New, so HandleFieldSave can answer it with errors.Is in the register of its neighbouring cases."
  - "The two bounds get NO kind-clearing clause here and this file names no bounded kind: the constant is minted by 07-03 Task 1. The inverted-pair refusal names no kind and needs none."
  - "The shared label over the two bound boxes carries no for= and names the group via aria-labelledby — 07-01's Grouped() rule applied to the definition form."

patterns-established:
  - "Mutation-proof a mechanical edit: one site was temporarily broken to confirm the round-trip test fails, then restored — the counting criterion alone never proves teeth"
  - "A comment must not trip its own acceptance criterion: 'kein CHECK' in German prose broke grep -c CHECK = 0, so the reason is stated by pointing at 00028 instead"

requirements-completed: [FIELD-01, FIELD-02, FIELD-05]

coverage:
  - id: D1
    description: "Migration 00046 adds four columns with ALTER TABLE ADD COLUMN, no constraint at the column head, and a Down that reverses all four"
    requirement: FIELD-05
    verification:
      - kind: other
        ref: "grep -c 'ADD COLUMN' = 4; grep -c 'DROP COLUMN' = 4; grep -c 'CHECK' = 0"
        status: pass
      - kind: integration
        ref: "throwaway goose Up/Down run: version 46 applied, all four columns present in pragma_table_info, Down returns version 45 and removes all four"
        status: pass
    human_judgment: false
  - id: D2
    description: "All four properties survive every store read path — Get, List, Sub, OfBlockType, OfBlockTypes — and an Update that changes them; a field that sets none reads back as zero"
    requirement: FIELD-02
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestNeueSpalten"
        status: pass
    human_judgment: false
  - id: D3
    description: "The four new column values travel as bound $n parameters — a quote and a semicolon come back unchanged and the table survives (T-07-05)"
    verification:
      - kind: integration
        ref: "internal/field/store_test.go#TestNeueSpaltenSindGebundeneParameter"
        status: pass
    human_judgment: false
  - id: D4
    description: "validate refuses an inverted numeric bound pair with ErrRangeInverted and a negative MaxValues, and clears a property meaningless for the chosen kind instead of refusing it (T-07-06)"
    requirement: FIELD-05
    verification:
      - kind: unit
        ref: "internal/field/store_test.go#TestNeueSpaltenGeprueft"
        status: pass
    human_judgment: false
  - id: D5
    description: "An operator can set all four on the definition screen and they are still there after the redraw"
    requirement: FIELD-01
    verification:
      - kind: integration
        ref: "internal/admin/field_defs_test.go#TestFelddefinitionTraegtDieVierEigenschaften"
        status: pass
    human_judgment: false
  - id: D6
    description: "A lower bound above its upper one is refused, is not stored, and the reason reaches the person filling the form as a flash error"
    requirement: FIELD-05
    verification:
      - kind: integration
        ref: "internal/admin/field_defs_test.go#TestVerdrehteGrenzenWerdenGemeldetUndNichtGespeichert"
        status: pass
    human_judgment: false
  - id: D7
    description: "All four survive the bundle round trip at all three carrier shapes — page field, group sub-field, block-kind field"
    requirement: FIELD-02
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestNeueFeldeigenschaftenUeberlebenDieArchivreise"
        status: pass
      - kind: unit
        ref: "internal/bundle/bundle_test.go#TestManifestSchweigtUeberUngenutzteEigenschaften"
        status: pass
    human_judgment: false
  - id: D8
    description: "IsButtonRow is the one predicate for a Choice drawn as a row of buttons"
    requirement: FIELD-01
    verification:
      - kind: unit
        ref: "internal/field/store_test.go#TestIsButtonRow"
        status: pass
    human_judgment: false
  - id: D9
    description: "The four definition-form controls look right, read right and are usable — labels, hints, the two bound boxes side by side"
    requirement: FIELD-01
    verification: []
    human_judgment: true
    rationale: "The tests prove the markup is present, carries the stored values back, and contains no script. Whether the four controls read sensibly on the screen, whether the hints say enough about which kind each one is for, and whether the two bound boxes sit right in the form is the browser pass — ROADMAP criterion 6 / QUAL-02, owned by plan 07-07."

# Metrics
duration: 21 min
completed: 2026-09-05
status: complete
---

# Phase 7 Plan 02: Migration 00046 and the Four Columns Summary

**Migration `00046` gives `darstellung`, `max_werte`, `min_wert` and `max_wert` their own columns on `page_field_defs`, and all seven SQL sites in `internal/field/store.go`, the definition screen and both directions of the bundle carry them — proven by reading, not by counting.**

## Performance

- **Duration:** 21 min
- **Started:** 2026-09-05T13:55Z
- **Completed:** 2026-09-05T14:16Z
- **Tasks:** 3 (1 decision gate, 1 TDD task with RED/GREEN, 1 auto)
- **Files modified:** 11 (3 created, 8 modified)

## Accomplishments

- **The phase's one migration is in and reverses.** `00046` adds four columns with plain `ALTER TABLE ADD COLUMN` and no constraint at the column head, matching the precedent `00028` states for `art`. Every default preserves an existing field's behaviour, so no data migrates. Proven up *and* down against a real database file: version 46 applies, the four columns appear in `pragma_table_info`, `Down` removes all four and the recorded version returns to 45.
- **The seven SQL sites are all seven.** The five SELECT column lists were edited as one replace-all, so they remain byte-identical copies of one list; `scanDef` gained the four in the identical order; the `INSERT` and the `UPDATE` gained appended placeholders rather than a renumbering of their existing non-sequential ones. `grep -c 'bedingung, COALESCE(block_type_id, 0)'` is 0 — no list was left at the old shape.
- **The gate on that edit is behavioural.** `TestNeueSpalten` creates fields at every carrier shape and reads them back through `Get`, `List`, `Sub`, `OfBlockType`, `OfBlockTypes` and after an `Update`. A missed SELECT shows up as a zero value in exactly one assertion — which a grep count cannot find, because a SELECT can name a column and still never write it into the struct.
- **The bundle round trip was mutation-tested.** One of the six carrier sites was temporarily broken; the round-trip test failed naming the exact property and the exact carrier, then passed again once restored. The `Display:` counts of 3 and 3 are the cheap check; this is the one with teeth.
- **A manifest from a website that uses none of the four is unchanged.** `omitempty` on all four keys, asserted directly — a bundle exists to be read and repaired by hand, and a file that grows four empty keys per field is worse to read.
- **The definition screen carries all four with no JavaScript,** and refuses an inverted pair of bounds with a sentence written for the person filling the form in.

## Task Commits

1. **Task 1: D-14 decision gate** — no commit; auto-selected `four-columns` (see Decisions Made)
2. **Task 2 RED: the failing store test** — `5759e85` (test) — three packages failed to build on `Def.Display`, `DisplayButtons`, `RangeMin`, `RangeMax`, `MaxValues`
3. **Task 2 GREEN: migration, four properties, seven SQL sites, validate** — `1acb766` (feat)
4. **Task 3: the definition screen and the bundle** — `1ed84d9` (feat)

No REFACTOR commit: the implementation needed no cleanup.

## Files Created/Modified

- `internal/db/migrations/00046_field_kinds.sql` *(new)* — four `ADD COLUMN`, four reversing `DROP COLUMN`; German comments stating why the bounds are text and why no constraint sits at the column head
- `internal/field/field.go` — `Def.Display`, `Def.MaxValues`, `Def.RangeMin`, `Def.RangeMax`; `DisplayButtons = "knopfreihe"`; `func (d Def) IsButtonRow() bool`
- `internal/field/store.go` — the four columns at all seven SQL sites; `scanDef` extended in the identical order with a comment naming why the five lists must stay copies; `ErrRangeInverted`; `validate` gains the two refusals and the two clearing rules
- `internal/field/store_test.go` *(new)* — `TestNeueSpalten` over every read path, the bound-parameter test for T-07-05, the three `validate` rules, and `TestIsButtonRow`
- `internal/admin/field.go` — `HandleFieldSave` reads the four form values; the `ErrRangeInverted` flash case
- `internal/admin/field_defs_test.go` *(new)* — the form → storage → redraw test and the inverted-bounds refusal test
- `cmd/holzcloud/templates/admin/field_list.html` — a `darstellung` select, a `max_werte` number input, and the two bound boxes in a `role="group"` named by `aria-labelledby`; every string through `{{t "…"}}`
- `internal/bundle/format.go` — `display`, `max_values`, `min`, `max` on `Field`, all `omitempty`
- `internal/bundle/export.go` / `import.go` — three sites each
- `internal/bundle/bundle_test.go` — the three-carrier round-trip test and the omitempty assertion

## Decisions Made

**D-14 — four own columns (`four-columns`).** `darstellung`, `max_werte`, `min_wert` and `max_wert` get their own columns rather than a reuse of `auswahl`, which is read one option per line. Confirmed before writing: the tree ran to `00045` and `00046` was free.

> **⚠ This decision was auto-selected, not confirmed by a human.** The Task 1 checkpoint carried `gate="blocking"`, and `.planning/config.json` has `workflow.auto_advance: true` — so the checkpoint protocol auto-selects the first (recommended) option. The selected option is exactly the option `07-CONTEXT.md` D-14 and `REQUIREMENTS.md:317` record, so nothing diverged. But the reversibility rating is **one-way**: `00046` is now in the tree, and from the operator's next deploy it is history. A correction is `00047`, never an edit to this file. **If different column names or types were wanted, now is the last cheap moment to say so.**

**The bounds are text, not numbers.** "No lower bound" and "the lower bound is zero" are different facts. A numeric column with a default could not tell them apart; the empty string can.

**`ErrRangeInverted` is a sentinel.** The store keeps a short reason; the screen hangs its own longer sentence on `errors.Is`, in the register of the seven neighbouring flash cases. The store test asserts `errors.Is` and not just the text, because that unwrapped pass-through is the precondition for the screen's case ever firing.

**No kind-clearing clause for the two bounds, and no bounded kind named in this file.** `KindRange` is minted by plan 07-03 Task 1; naming it here would not compile, and this task's own `go build ./...` gate would report it. The inverted-pair refusal names no kind and needs no constant, so the bounds are still *validated* here.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's behaviour spec contradicted its own clearing rule**

- **Found during:** Task 2 (writing the RED test)
- **Issue:** `<behavior>` says "After `Create` with all four properties set, `Get` returns all four unchanged", while `<action>` requires `validate` to clear `Display` for anything that is not `KindChoice` and `MaxValues` for anything that is not `KindMulti`. No kind keeps both — a Choice loses `MaxValues`, a Multiple Choice loses `Display`. A test written literally to the behaviour bullet could only pass by dropping the clearing rule, which mitigates T-07-06.
- **Fix:** Kept the clearing rule, which `<action>` states explicitly and the threat register depends on. The read-path test uses a **pair** of fields per scope — a Choice carrying `Display` + both bounds, a Multiple Choice carrying `MaxValues` + both bounds. Together they cover all four columns on every read path, so the gate the task exists for is unweakened. The clearing itself is asserted separately in `TestNeueSpaltenGeprueft`.
- **Files modified:** `internal/field/store_test.go`
- **Verification:** `go test ./internal/field/ -run TestNeueSpalten` passes; all four columns are read non-zero on each of the five read paths.
- **Committed in:** `5759e85` / `1acb766`

**2. [Rule 3 - Blocking] A German comment tripped its own acceptance criterion**

- **Found during:** Task 2 (acceptance-criteria gate)
- **Issue:** The criterion `grep -c 'CHECK' internal/db/migrations/00046_field_kinds.sql` must return 0. The migration's comment explained D-14's no-constraint precedent using the literal word, pushing the count to 1 — a false failure, since no constraint exists in the file. Same class of trap as 07-01's `SplitChoices` comment.
- **Fix:** Reworded to "keine Prüfregel am Spaltenkopf" and pointed the reader at the `art` column in `00028`, where the precedent is stated verbatim. The reason survives; the criterion measures what it meant to measure.
- **Files modified:** `internal/db/migrations/00046_field_kinds.sql`
- **Verification:** `grep -c 'CHECK' internal/db/migrations/00046_field_kinds.sql` returns 0; no constraint exists in the file.
- **Committed in:** `1acb766`

**3. [Rule 2 - Missing Critical] The inverted-bounds flash case had no reachable sentinel**

- **Found during:** Task 3
- **Issue:** The plan asks for "a flash-error case for the new `validate` reason … in the register of the existing cases". Every existing case is an `errors.Is` on an exported sentinel; `validate`'s new reason was a bare `errors.New`, which only `case err != nil` could catch. That fallback prints the store's own short sentence, so the screen could never say more than the store — and a later reword of the store's text would silently change what the operator reads.
- **Fix:** Exported `field.ErrRangeInverted`, returned it from `validate`, and added the `errors.Is` case with the longer sentence. The store test asserts `errors.Is` so the pass-through cannot regress unnoticed.
- **Files modified:** `internal/field/store.go`, `internal/admin/field.go`, `internal/field/store_test.go`
- **Verification:** `TestVerdrehteGrenzenWerdenGemeldetUndNichtGespeichert` reads the flash back off the session and asserts it names the bound; the definition is not stored.
- **Committed in:** `1ed84d9`

**4. [Rule 2 - Missing Critical] A shared label over two inputs would have carried a dangling `for=`**

- **Found during:** Task 3
- **Issue:** The plan asks for the two bound boxes "side by side" under one label. Written the house way — `<label class="form-label" for="feld-min-wert">Grenzen</label>` — the label names only the first box, and the second is unnamed. This is exactly the case 07-01 solved for the checkbox group.
- **Fix:** The label became a `<span class="form-label">` with an id, the pair sits in `role="group" aria-labelledby=`, and each box carries its own `aria-label`. No `for=` points at anything, and both boxes have an accessible name.
- **Files modified:** `cmd/holzcloud/templates/admin/field_list.html`
- **Verification:** Rendered form asserted in `TestFelddefinitionTraegtDieVierEigenschaften`; the browser half is 07-07's.
- **Committed in:** `1ed84d9`

**5. [Rule 1 - Bug] A test assertion measured the wrong scope**

- **Found during:** Task 3 (first run of the new admin test)
- **Issue:** The handler test asserted the rendered page contains no `<script`. It failed — because `HandleFieldList` renders the whole admin layout, which loads htmx. The assertion was measuring the layout, not the definition form.
- **Fix:** Removed the assertion from the handler test; the plan's own file-level criterion (`grep -c 'script\|onclick\|javascript:'` on the template, which returns 0) is the correct gate and already passes. The value-redraw assertions were tightened at the same time to look inside a 400-byte window around each named box, so a `value="1"` elsewhere in the document cannot pass for a proof.
- **Files modified:** `internal/admin/field_defs_test.go`
- **Verification:** Both new admin tests pass; the file-level grep still returns 0.
- **Committed in:** `1ed84d9`

---

**Total deviations:** 5 auto-fixed (2 bugs, 2 missing critical, 1 blocking)
**Impact on plan:** All five were inside this plan's own work. No file outside the plan's `files_modified` list was touched except `internal/admin/field_defs_test.go`, which is a new test the plan's own last acceptance criterion required a proof for. No scope creep.

## Issues Encountered

- **Line numbers in the plan's `<read_first>` pointers had drifted** after Wave 1, as the orchestrator warned. Every site was re-located by content: the five SELECT lists by their shared `pflicht, hinweis, auswahl` text, the builders in `export.go`/`import.go` by their struct literals. No behaviour affected.
- **The plan's `?bearbeiten=` was actually `?aendern=`.** Found when the redraw assertion returned an empty form. Corrected against the handler.
- **No existing test exercises any migration's `Down`.** `go test ./internal/db/...` only ever migrates from zero, so it cannot show that `00046` reverses. Proven with a throwaway in-package test (goose `Down` via the unexported `migrationProvider`), which was removed after it passed. This is a narrow instance of the structural blindness STATE.md already records under Known Risks — a suite that only migrates from zero.

## Known Stubs

None. Every branch added is wired to real data and covered by a passing test.

## Threat Flags

None. The surface this plan adds is exactly what `<threat_model>` anticipated:

| Threat | Disposition | Status |
|---|---|---|
| T-07-05 (tampering via the four new columns) | mitigate | All four travel as bound `$n` parameters. `TestNeueSpaltenSindGebundeneParameter` stores a quote and a semicolon and asserts both the value and the table survive. |
| T-07-06 (tampering via `validate`) | mitigate | Negative `MaxValues` refused, inverted numeric bounds refused, a property meaningless for the chosen kind cleared. The bounds' half of the clearing clause is 07-03 Task 1's, with the constant it names. |
| T-07-07 (`max_werte` from a hostile bundle) | mitigate | An `INTEGER` column, allocating nothing; a large value means "no practical cap", which is already the default. |
| T-07-08 (elevation via the definition screen) | mitigate | `HandleFieldSave` still resolves the website through `lookupWebsite`; `Update` still looks a field up by id **and** website. No shortcut was added — the four properties ride the existing path. |
| T-07-09 (repudiation, migration 00046) | accept | Goose records version 46; the `Down` reverses all four, verified. |
| T-07-SC (package-manager installs) | accept | No install of any kind. Every symbol is Go standard library or already in `go.mod`. |

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

**Ready for 07-03** (the three small kinds: `zeit`, `bereich`, `code`).

- The columns the remaining kinds need are in the table and reach every layer. `Def.RangeMin`/`RangeMax` are stored, read, screened and bundled; 07-03 Task 1 mints `KindRange` and adds the bounds' clearing clause, which is the only piece of this plan deliberately left open.
- `DisplayButtons` and `IsButtonRow()` are in place for 07-06's template branch and its widened `switchOf`; neither will have to spell the comparison again.
- **Open, by design:** `go run ./tools/i18n` reports `15 offen` in each of en/es/fr/it — 2 inherited from 07-01 and 13 new source strings from the four definition-form controls and their hints. The translation gate is plan **07-07**'s, on its own commit, per the ROADMAP wave plan.
- **Open, by design:** the browser pass (ROADMAP criterion 6 / QUAL-02). The four controls have never been seen in a browser — only through rendered-HTML assertions. That is 07-07's, and D-09 above records it.
- **Wants a human glance:** the D-14 auto-selection. It is the recorded decision and nothing diverged, but the gate was meant to be human-confirmed and a released migration is never edited.

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*

## Self-Check: PASSED

- All created files exist on disk: `internal/db/migrations/00046_field_kinds.sql`, `internal/field/store_test.go`, `internal/admin/field_defs_test.go`.
- All three task commits are in the log: `5759e85`, `1acb766`, `1ed84d9`. No commit deleted a tracked file.
- Every task-level `<acceptance_criteria>` re-run and passing, including all four column greps at 7, the old-SELECT-shape grep at 0, `CHECK` at 0, and `Display:` at 3/3.
- Plan-level `<verification>`: `go build ./...` succeeds; `go test ./...` exits 0 across the whole tree; `00046` applies from an empty database and reverses cleanly; the four properties survive create, all five read paths, update, the definition screen and the bundle round trip.
