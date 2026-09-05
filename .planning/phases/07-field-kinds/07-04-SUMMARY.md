---
phase: 07-field-kinds
plan: 04
subsystem: api
tags: [go, field-kinds, multi-value, validation, mcp, ai-tools, htmx]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "07-01's SplitValues/JoinValues, KindMulti, Def.NameSuffix and the [] marker minted at one place — this plan carries that marker into the second naming path and counts against the encoding"
  - phase: 07-field-kinds
    provides: "07-02's Def.MaxValues, Def.Display, Def.RangeMin and Def.RangeMax — the four columns this plan enforces and reports"
  - phase: 07-field-kinds
    provides: "07-03's field.ParseNumber and the three small kinds — the register the new reasons are written in"
provides:
  - "max_werte enforced server-side in Check, counted with SplitValues; a maximum of zero still means no cap"
  - "a shared byte-budget guard before Check's per-kind switch: MaxValueBytes is the budget for a field's whole joined value, newlines included, measured in bytes"
  - "trimTo no longer shortens — the last silently-truncating path on the save route is gone (D-13 closed)"
  - "parseRowName reports the multi-value marker as a fourth return value, with every existing guard intact plus an empty-sub-name guard"
  - "groupView appends sub.NameSuffix() — the marker is minted at one place on both naming paths"
  - "felder_auflisten reports darstellung, max_werte, min_wert and max_wert, each omitted when unset, plus a one-sentence multi-value encoding note"
  - "ai.feldeigenschaften — the one place that describes a field's value shape, used for a page field and a group sub-field alike"
affects: [07-05, 07-06, 07-07, 08-snippets, 09-csv-import]

actuals:
  tokens: 40600
  tasks: 3
  commits: 7

tech-stack:
  added: []
  patterns:
    - "A limit is reported, never applied: a value that is too long is refused with a reason, because a shortened value is indistinguishable from one somebody typed"
    - "One helper describes a field to the assistant, called from the page-field arm and the group sub-field arm, so the two cannot drift apart"
    - "A marker is stripped after every guard, never before: prefix, part count and row bound decide first, and only then is the name believed"

key-files:
  created: []
  modified:
    - internal/field/field.go
    - internal/field/field_test.go
    - internal/admin/page_form.go
    - internal/admin/page_fields.go
    - internal/admin/page_fields_multi_test.go
    - internal/ai/tools.go
    - internal/ai/ai_test.go

key-decisions:
  - "The byte-budget reason names the limit and says that umlauts count double — the guard measures bytes, so a message promising 4000 characters would be a lie on any word with an umlaut"
  - "The value-count reason carries a singular form for a maximum of one; German grammar, not decoration, in a sentence the person filling the form reads"
  - "parseRowName gained a fourth guard: a sub-field name that is empty after the marker is stripped is refused, mirroring the feld_[] skip 07-01 added at the top-level path"
  - "The multi-value note is keyed mehrere_werte, in the register of nur_wenn_ausgefuellt, and is attached only to a field whose IsMultiValued() is true"
  - "cmd/holzcloud/templates/admin/field_input.html needed no change: its mehrfachauswahl branch renders .Name for the sentinel and every checkbox, and .Name now carries the marker in a group row too — read, not assumed"

patterns-established:
  - "Report-don't-truncate: the save path has no remaining place that silently shortens a stored value"
  - "One describer for the assistant: a new definition property is added to feldeigenschaften once and appears on page fields and group sub-fields together"

requirements-completed: [FIELD-02, FIELD-07]

coverage:
  - id: D1
    description: "max_werte is enforced server-side: exactly the maximum is accepted, one more is refused with a German reason naming the field and the number, and a maximum of zero is no cap"
    requirement: FIELD-02
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestMehrfachauswahlHoechstzahl"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestMehrfachauswahlGenauAmRand"
        status: pass
    human_judgment: false
  - id: D2
    description: "MaxValueBytes is the budget for one field's whole joined value: exactly the limit is accepted, one byte more is refused, the newlines between values count, and the measure is bytes rather than runes"
    requirement: FIELD-07
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestGemeinsamesBytebudget"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestBytebudgetGiltAllenWertenZusammen"
        status: pass
    human_judgment: false
  - id: D3
    description: "Nothing on the save path shortens a value any more — Clean stores the full string, a group row likewise, and CheckAll reports the overflow under the field's own key (D-13)"
    requirement: FIELD-07
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestNichtsWirdMehrStillGekuerzt"
        status: pass
      - kind: other
        ref: "grep -c 'val\\[:MaxValueBytes\\]' internal/field/field.go = 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "A multi-valued field whose condition is not met is skipped by CheckAll entirely, cap and budget included, while the same field visible is checked"
    requirement: FIELD-02
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestVerstecktesMehrwertigesFeldWirdNichtGeprueft"
        status: pass
    human_judgment: false
  - id: D5
    description: "A multi-valued sub-field inside a group keeps all its values through form, request, storage and redraw, per row, with the sentinel in every row and the single-valued sub-fields untouched"
    requirement: FIELD-07
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_multi_test.go#TestMehrfachauswahlInEinerGruppe"
        status: pass
    human_judgment: false
  - id: D6
    description: "parseRowName reports the marker and keeps every guard: the gruppe. prefix, the three-part shape, the MaxRows bound, and a sub-field name that is empty after stripping (T-07-16)"
    requirement: FIELD-07
    verification:
      - kind: unit
        ref: "internal/admin/page_fields_multi_test.go#TestZeilennameMitMarkierung"
        status: pass
    human_judgment: false
  - id: D7
    description: "Cleared and absent stay distinguishable inside a group row exactly as on the page itself"
    requirement: FIELD-02
    verification:
      - kind: unit
        ref: "internal/admin/page_fields_multi_test.go#TestGruppenzeileGeleertOderAbwesend"
        status: pass
    human_judgment: false
  - id: D8
    description: "felder_auflisten reports the presentation, the value cap and both bounds, each omitted when unset, plus a one-sentence multi-value note carried only by a multi-valued field — on top-level fields and on a group's sub-fields alike"
    requirement: FIELD-02
    verification:
      - kind: integration
        ref: "internal/ai/ai_test.go#TestFelderAuflistenBeschreibtDieNeuenEigenschaften"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/ai/tools.go | grep -c 'IsMultiValued' = 1"
        status: pass
    human_judgment: false
  - id: D9
    description: "A page written through seite_anlegen with a multi-valued field spelled the way the note describes reads back through seite_lesen with the same values, and the cap the note announces is the cap the writing path applies"
    requirement: FIELD-02
    verification:
      - kind: integration
        ref: "internal/ai/ai_test.go#TestMehrwertigesFeldGehtDurchDieWerkzeugeUndZurueck"
        status: pass
    human_judgment: false
  - id: D10
    description: "No writing-tool schema was widened and no tool was added — felder stays a {\"kennung\": \"wert\"} object"
    verification:
      - kind: unit
        ref: "internal/ai/ai_test.go#TestJedesWerkzeugHatEinSchema"
        status: pass
      - kind: other
        ref: "git diff of internal/ai/tools.go touches no Tool Name and no \"felder\" property line"
        status: pass
    human_judgment: false

# Metrics
duration: 22 min
completed: 2026-09-05
status: complete
---

# Phase 7 Plan 04: Multiple Choice Closed Summary

**`max_werte` counted server-side against `SplitValues`, `MaxValueBytes` turned from a silent truncation into a reported refusal shared across a field's whole joined value, the `[]` marker carried into group rows through the same `NameSuffix` that mints it on the page, and `felder_auflisten` teaching an assistant the cap, both bounds, the presentation and how to spell a multi-valued value.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-05T14:33Z
- **Completed:** 2026-09-05T14:55Z
- **Tasks:** 3 (all TDD, RED then GREEN)
- **Files modified:** 7

## Accomplishments

- **The cap lives where it can actually work.** A checkbox group cannot be capped in markup without JavaScript, and this program carries none beyond htmx — so `Check` counts `SplitValues(value)` against `Def.MaxValues` and names both the field and the number in a sentence the person filling the form reads. A maximum of zero is no cap, which is what every field defined before this phase carries, so nothing existing changed behaviour.
- **D-13 is closed, and the ledger says so.** `trimTo` cut a value at `MaxValueBytes` silently. With `MaxValueBytes` now shared across all of one field's values, that truncation would have cut a value in half and stored the fragment as if somebody had typed it. The guard moved to the top of `Check`, before the per-kind switch, so it holds for every kind; `trimTo` only trims whitespace now, and its doc comment says why it must not shorten again. `.planning/WINDOWS.md` entry 2 is `fixed`, and the ledger has **zero open entries**.
- **The measurement is honest at both edges.** Exactly `MaxValueBytes` is accepted and one byte more is refused; for a multi-valued field the measured string is the joined one, newlines included, not the longest single value; and it is `len` on the Go string, so a value of umlauts reaches the limit at half the characters. The reason says so rather than promising 4000 characters it cannot deliver.
- **The marker survives the second naming path.** `groupView` appends `sub.NameSuffix()` — the marker is still never spelled at a call site — and `parseRowName` reports it as a fourth return value, stripped only after the prefix check, the three-part check and the `MaxRows` bound have all passed. A group row with three ticked boxes stores three values and redraws with three ticked, per row, with its own sentinel.
- **The AI round trip the ROADMAP's per-kind tax names is paid.** `felder_auflisten` now reports `darstellung`, `max_werte`, `min_wert` and `max_wert`, each omitted when it carries nothing, and a multi-valued field alone carries one sentence saying its values go one per line into the single string the writing tools take. The proving test writes such a field through `seite_anlegen` and reads it back through `seite_lesen` unchanged — and asserts that three values into a cap of two are refused, so the note and the guard cannot disagree.

## Task Commits

1. **Task 1 RED: the cap and the shared budget** — `f96acc2` (test) — six failing assertions in `internal/field/field_test.go`
2. **Task 1 GREEN: enforced and reported** — `e50572a` (feat) — `internal/field/field.go`
3. **Task 2 RED: the marker in the group row** — `dd33033` (test) — build failure on `parseRowName`'s arity, as intended
4. **Task 2 GREEN: minted once, read everywhere** — `fd89ebe` (feat) — `internal/admin/page_form.go`, `internal/admin/page_fields.go`
5. **Task 2 follow-up: the new guard proven** — `5ab1525` (test) — the empty-sub-name cases
6. **Task 3 RED: what the assistant must be told** — `d2392b7` (test) — `internal/ai/ai_test.go`
7. **Task 3 GREEN: the four properties and the note** — `9620a35` (feat) — `internal/ai/tools.go`

No REFACTOR commits: none of the three implementations needed cleanup, and TDD says commit only when something changed.

## Files Created/Modified

- `internal/field/field.go` — the byte-budget guard before `Check`'s per-kind switch; the `MaxValues` count inside the `KindMulti` case; `trimTo` reduced to whitespace trimming with a doc comment saying why it must not shorten again
- `internal/field/field_test.go` — six German-named tests: the cap table, the exact-count boundary, the byte boundary in ASCII and in umlauts, the joined-value budget, the removed truncation at `Clean` and in a group row, and the hidden multi-valued field
- `internal/admin/page_form.go` — `parseRowName` gains the `multi` return value via `strings.CutSuffix` after every guard, plus a refusal of an empty sub-field name; `fieldsFromRequest` branches onto `field.JoinValues` for a marked row and still loads no definitions
- `internal/admin/page_fields.go` — `groupView` appends `sub.NameSuffix()` to the row field's name
- `internal/admin/page_fields_multi_test.go` — the two-row group end to end, the `parseRowName` guard table, and the cleared-versus-absent pair for a row
- `internal/ai/tools.go` — `feldeigenschaften`, called from the page-field arm and from the group sub-field arm of `felderAuflisten`
- `internal/ai/ai_test.go` — `aufbauMitFeldern` (the existing `aufbau` builds `Deps` without `Fields`, so `felder_auflisten` would have reported nothing), the description test and the write-then-read round trip

`cmd/holzcloud/templates/admin/field_input.html` is listed in the plan's `files_modified` and was **not** changed. Its `mehrfachauswahl` branch renders `{{.Name}}` for the hidden sentinel and for every checkbox, and `.Name` now carries the marker inside a group row as well. The branch was read to confirm this rather than assumed; the group test asserts the rendered names and the per-row sentinel.

## Decisions Made

**The byte reason says umlauts count double.** The guard is `len` on the Go string — bytes, because the limit exists to bound what goes into the database and the database counts bytes. A message promising "höchstens 4000 Zeichen" would be false for any value with an umlaut, and a limit somebody cannot predict is a limit they will hit twice.

**A singular form for a maximum of one.** "höchstens 1 Werte" is not a sentence. The reason branches on `MaxValues == 1`.

**`parseRowName` gained one guard it did not have.** A name like `gruppe.zeiten.0.` or `gruppe.zeiten.0.[]` used to produce a row entry under the empty key; `cleanRow` dropped it later, so nothing was ever stored, but the top-level path has refused the equivalent `feld_[]` since 07-01. The two paths now agree, and the guard table proves it.

**The note is keyed `mehrere_werte` and hangs off `IsMultiValued()` alone.** No other kind carries it — a note attached to every field is a note an assistant learns to ignore.

**The writing tools were not touched.** `felder` stays `{"kennung": "wert"}`. A second shape for a field taking several values would be the second spelling D-02 exists to prevent, and the round-trip test is what proves the single shape actually carries the encoding.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `parseRowName` accepted a row name with no sub-field name**

- **Found during:** Task 2 (widening `parseRowName`)
- **Issue:** The plan named three guards to preserve — prefix, three parts, `MaxRows` bound. None of them refuses `gruppe.zeiten.0.` or, after the marker is stripped, `gruppe.zeiten.0.[]`. Both produced a row entry keyed by the empty string. `cleanRow` filters by definition so nothing reached storage, but the top-level branch has skipped the equivalent empty key since 07-01, and a name-reading function that believes half a name is one that will be trusted by the next caller too.
- **Fix:** `if trimmed == "" { return "", 0, "", false, false }` after the `CutSuffix`, mirroring `fieldsFromRequest`'s `if trimmed == "" { continue }` exactly.
- **Files modified:** `internal/admin/page_form.go`, `internal/admin/page_fields_multi_test.go`
- **Verification:** both names are in `TestZeilennameMitMarkierung`'s rejection table; removing the guard fails the test.
- **Committed in:** `fd89ebe` (guard) and `5ab1525` (the proof)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Inside the function the task was already rewriting, and it closes a gap between the two naming paths that this plan exists to make symmetric. No file outside the plan's `files_modified` list was touched, and one file on that list needed no change.

## Issues Encountered

- **A write path that does not run `CheckAll` — named here rather than patched, as the plan instructs.** `internal/bundle/import.go`'s `importFieldValues` (`:420–439`) calls `field.Encode(data)` directly: no `CheckAll`, no `Clean`. It therefore never ran through `trimTo` either, so **removing the truncation changed nothing about it** — an over-long value in an imported manifest reached storage whole before this plan and still does. Every other write path is guarded: `internal/admin/page.go:444` and `:607` call `checkFields` before `field.Clean`, and `internal/ai/tools.go`'s `pruefeFelder` calls `CheckAll` before `Clean`. The bundle importer is a pre-existing gap, out of this plan's scope, and worth a decision in 07-05 (the bundle/Term wave) or as a phase-8 item — flagged, not fixed.
- **`aufbau` in `internal/ai/ai_test.go` builds `Deps` without `Fields`,** so `felder_auflisten` returns an empty list under it. A second helper, `aufbauMitFeldern`, was added rather than changing `aufbau`, which eleven existing tests depend on.
- **Execution incident, no lasting effect: `git stash` was run by accident** inside a compound shell command while inspecting the log. It stashed the single uncommitted working-tree change (`.planning/milestone.lock`). Detected immediately, inspected with `git stash show --stat`, restored with `git stash apply` and the entry dropped; `git stash list` is empty and the file is back exactly as it was. No committed work was involved. Recorded because a silent stash is precisely the kind of thing that looks like nothing and later looks like lost work.

## Known Stubs

None. Every branch added is reached by a passing test.

## Broken-Windows Ledger

`.planning/WINDOWS.md` entry **2** (`trimTo` truncating silently, logged by 07-01 as D-13's open half) is now **`fixed`**. The ledger's open count is **0**.

## Threat Flags

None. The surface is exactly what the plan's `<threat_model>` anticipated:

| Threat | Disposition | Status |
|---|---|---|
| T-07-15 (unbounded joined value) | mitigate | `Check` refuses a joined value over `MaxValueBytes` and a count over `Def.MaxValues`, both before anything is written; both boundaries are asserted at the limit and one past it. |
| T-07-16 (crafted row name) | mitigate | Prefix, three-part and `MaxRows` guards all survive the widening, the marker is stripped only after them, and an empty sub-field name is now refused too. The rejection table covers a fourth namespace, a non-numeric index, a negative index, `MaxRows` itself and far beyond it. |
| T-07-17 (a value not on the option list) | mitigate | Unchanged from 07-01 and still asserted: `Check`'s `KindMulti` case names the first submitted value that is not in `d.Choices`, and it runs before the count. |
| T-07-18 (the removed shortening) | mitigate | This is the plan's own change. No path on the save route writes a shortened value; the one that did now reports. |
| T-07-27 (the widened `felder_auflisten` map) | mitigate | The four added values are the website's own field configuration, beside the label, kind and full option list the tool already reported. `c.Scope.MaySee(a.Website)` still guards the call and no path was added that skips it. |
| T-07-SC (package installs) | accept | No package was installed; every symbol used is Go standard library or already in `go.mod`. No legitimacy checkpoint fired. |

## Checkpoint Gates

**This plan carried no checkpoint task** — three `type="auto" tdd="true"` tasks and nothing else. Nothing was auto-approved and nothing was auto-selected. (`workflow.auto_advance` is `true` in `.planning/config.json`, so a `gate="blocking"` checkpoint would have been auto-handled; none existed to handle.)

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

**Ready for 07-05.** Phase 7's build steps ① (the encoding) and ③ (multiple choice) are complete: the kind exists, both of its guards hold, it works identically on a page and inside a group, and an assistant can write into it correctly on the first try.

- **Open, by design:** 20 open translations in each of en/es/fr/it — unchanged by this plan, which added no `{{t …}}` string. The `Check` reasons are plain German like every other reason in `internal/field`. The translation gate is 07-07's.
- **Open, by design:** the browser pass. The group-row checkbox control has been asserted in rendered HTML, never seen. 07-07 owns it, together with D-08.
- **Flagged for a decision, not blocking:** the bundle importer writes page field values without `CheckAll` (see Issues Encountered). It is pre-existing and unchanged by this plan; 07-05 touches the bundle path and is the natural place to decide.
- **Still wanting a human glance:** 07-01's D-05 auto-selection. Nothing in this plan reopened it, and everything here assumes it — `mehrfachauswahl` is a kind of its own and `max_werte` belongs to it alone.

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*

## Self-Check: PASSED

- All seven modified files exist on disk; no file was claimed as created.
- All seven task commits are in `git log` (`f96acc2`, `e50572a`, `dd33033`, `fd89ebe`, `5ab1525`, `d2392b7`, `9620a35`).
- Every task-level `<acceptance_criteria>` re-run and passing, including the three greps: `val[:MaxValueBytes]` = 0, `"[]"` in `page_fields.go` = 0, `fieldStore|fields.List` in `page_form.go` = 0, `IsMultiValued` outside comments in `tools.go` = 1.
- Plan-level `<verification>`: `go build ./...` succeeds, `gofmt -l` prints nothing across `internal/` and `cmd/`, `go vet ./...` is silent, and `go test ./...` is green across the whole tree.
