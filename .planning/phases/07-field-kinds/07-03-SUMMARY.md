---
phase: 07-field-kinds
plan: 03
subsystem: ui
tags: [go, html-template, field-kinds, escaping, xss, css, search-index]

# Dependency graph
requires:
  - phase: 07-field-kinds
    provides: "07-01's SplitValues/KindMulti (PlainText joins a multi-value with them) and the corrected REQUIREMENTS row that keeps `code` inside block kinds"
  - phase: 07-field-kinds
    provides: "07-02's migration 00046 and Def.RangeMin/RangeMax — this plan writes the clearing clause 07-02 deliberately left open"
provides:
  - "field.KindTime, field.KindRange, field.KindCode — the three constants every later wave may reference"
  - "Check cases for a time of day and for an inclusive numeric range whose reason names its bounds"
  - "field.ParseNumber and field.ParseTimeOfDay — the one reading of a number and of a clock time, shared by Check and by store.go's inversion check"
  - "MayControl excludes KindTime; KindRange deliberately stays in (D-08, pending 07-07's browser pass)"
  - "Resolve/List for all three: a Number carrying the typed Raw, a *time.Time pointer, a plain string"
  - "validate's third clearing clause — both bounds cleared for any kind that is not a Range"
  - "the time, number and fixed-width textarea controls in field_input.html, including the placeholder .feld-schalter--text depends on"
  - ".form-code in admin.css — a system monospace stack, nothing fetched at runtime"
  - "renderOwn's escaped <pre><code> for a Code field inside a block (D-06, FIELD-06)"
  - "PlainText's widened whitelist: KindCode and KindMulti in, KindTime and KindRange out"
affects: [07-04, 07-05, 07-06, 07-07, 08-snippets, 09-csv-import, 11-galerie]

actuals:
  tokens: 68000
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "One reading per quantity: ParseNumber is used for the entered value AND for both bounds, and store.go's inversion check now uses it too — two places that read the same digits differently is a bug that only shows on the one pair nobody rechecks by hand"
    - "A time of day is keyed on the string's length, not on time.Parse's leniency: its hour field accepts one digit, so \"9:30\" would silently pass as half past nine"
    - "A kind whose content must not be interpreted is escaped where the HTML freezes — in the block render path, not in the theme"
    - "A conditional attribute is proven by mutation: emitting min/max unconditionally must make the test fail, or the {{with}} is decoration"

key-files:
  created:
    - internal/admin/page_fields_kinds_test.go
  modified:
    - internal/field/field.go
    - internal/field/render.go
    - internal/field/store.go
    - internal/field/field_test.go
    - internal/field/store_test.go
    - internal/block/render.go
    - internal/block/block_test.go
    - cmd/holzcloud/templates/admin/field_input.html
    - cmd/holzcloud/assets/admin.css
    - internal/admin/field_defs_test.go
    - internal/bundle/bundle_test.go

key-decisions:
  - "PlainText's search whitelist gains KindCode and KindMulti and nothing else — recorded explicitly because no roadmap or context note made this call"
  - "A multi-valued field enters the search index joined with a space, via field.SplitValues, not as the stored newline column"
  - "ParseNumber and ParseTimeOfDay are the single readings; store.go's inversion check was switched onto ParseNumber so validate and Check cannot disagree about the same pair of digits"
  - "A time of day is refused unless it is exactly HH:MM or HH:MM:SS — time.Parse's hour field accepts one digit and would let \"9:30\" through"
  - "MayControl was split into two switch cases so the date/time exclusion carries its own reason; KindRange stays a legal controller (D-08), undecided here by design"
  - "The two bounds moved onto a Range field in three existing test suites — the new clearing clause removes them from any other kind"

patterns-established:
  - "A subtractive filter needs no edit for a new kind: SubKinds and BlockKinds were left untouched and a test asserts all three new kinds are in both"
  - "A test that measures an attribute's absence reads the element itself (from its < to its >), not a window around it — a neighbouring field's min=\"1\" is not a proof"

requirements-completed: [FIELD-04, FIELD-05, FIELD-06]

coverage:
  - id: D1
    description: "A Time field accepts a time of day, carries no timezone, and empty is distinguishable from midnight"
    requirement: FIELD-04
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestZeitPruefung"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestZeitAufgeloest"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestZeitStehtAlsTextInDerListe"
        status: pass
    human_judgment: false
  - id: D2
    description: "A Time field is absent from the fields a condition may hang on, for the same reason a Date field is"
    requirement: FIELD-04
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestWoranEineBedingungHaengenDarf"
        status: pass
      - kind: other
        ref: "grep -n 'case KindGroup, KindSection, KindDate' internal/field/field.go (no output = pass)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A Range field is an input type=number with min, max and step — not a slider — and no JavaScript was added"
    requirement: FIELD-05
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_kinds_test.go#TestFeldartenImSeiteneditor"
        status: pass
      - kind: integration
        ref: "internal/admin/page_fields_kinds_test.go#TestFeldartenTragenKeinJavaScript"
        status: pass
    human_judgment: false
  - id: D4
    description: "CheckAll accepts a Range value at each bound and refuses one step either side, with a reason naming the bounds; equal bounds accept exactly one number; only-lower and only-upper behave as their mirror images"
    requirement: FIELD-05
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestBereichPruefung"
        status: pass
    human_judgment: false
  - id: D5
    description: "An empty Range value is not filled in — never coerced to zero and never to the lower bound; a Range prints the exact string that was typed"
    requirement: FIELD-05
    verification:
      - kind: unit
        ref: "internal/field/field_test.go#TestLeererBereichIstNichtNull"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestBereichDrucktDasGetippte"
        status: pass
    human_judgment: false
  - id: D6
    description: "The bounds ride on the input as min and max only where they exist, and the placeholder .feld-schalter--text depends on is present"
    requirement: FIELD-05
    verification:
      - kind: integration
        ref: "internal/admin/page_fields_kinds_test.go#TestFeldartenImSeiteneditor (mutation-proven: unconditional min/max makes it fail)"
        status: pass
      - kind: other
        ref: "grep -c 'placeholder=\" \"' cmd/holzcloud/templates/admin/field_input.html = 5"
        status: pass
    human_judgment: false
  - id: D7
    description: "HTML typed into a Code field appears verbatim and does not execute, including inside a block, and never passes through Markdown"
    requirement: FIELD-06
    verification:
      - kind: unit
        ref: "internal/block/block_test.go#TestCodeImBausteinWirdMaskiert"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestCodeIstRoherText (value resolves as string, never template.HTML)"
        status: pass
    human_judgment: false
  - id: D8
    description: "An empty Code field is skipped everywhere — Resolve yields the empty string, List drops the entry, the block render path writes no element"
    requirement: FIELD-06
    verification:
      - kind: unit
        ref: "internal/block/block_test.go#TestCodeImBausteinLeerErgibtNichts"
        status: pass
      - kind: unit
        ref: "internal/field/field_test.go#TestCodeIstRoherText"
        status: pass
    human_judgment: false
  - id: D9
    description: "The site search indexes a Code field's text and a multi-valued field's values, the latter joined with a space"
    verification:
      - kind: unit
        ref: "internal/block/block_test.go#TestPlainTextNimmtCodeUndMehrfachauswahl"
        status: pass
    human_judgment: false
  - id: D10
    description: "The three controls read and behave right on the screen — a time picker, a bounded number box, a fixed-width code box — and a bereich field really does show and hide its dependants in a browser"
    verification: []
    human_judgment: true
    rationale: "The tests prove the markup is present, carries the bounds, carries the placeholder and contains no script. Whether :placeholder-shown actually fires on a number input is D-08's open question and cannot be settled by reading — it is ROADMAP criterion 6 / QUAL-02, owned by plan 07-07, which also carries the written reversal path."

# Metrics
duration: 17 min
completed: 2026-09-05
status: complete
---

# Phase 7 Plan 03: The Three Small Kinds Summary

**`zeit`, `bereich` and `code` exist end to end — a time of day that tells empty from midnight, a bounded number input that is deliberately not a slider, and a Code field whose HTML is escaped in the block render path, where the bytes freeze.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-09-05T14:20Z
- **Completed:** 2026-09-05T14:37Z
- **Tasks:** 3 (2 TDD with RED/GREEN, 1 auto)
- **Files modified:** 12 (1 created, 11 modified)

## Accomplishments

- **FIELD-06's sharpest requirement is closed where it actually had to be closed.** `renderOwn` gained `case field.KindCode`, emitting `<pre class="hc-eigen__code hc-eigen__code--<key>"><code>…</code></pre>` with the value through `html.EscapeString` and explicitly not through `prose`. That is the block render path — a block freezes to HTML when the page is saved, so escaping in the theme would have been too late. The test puts a `<script>`, a quote and an ampersand into a Code field and asserts the escaped forms are present, the raw tag is not, and no `<p>` appears at all: the test's Markdown double wraps everything in `<p>`, so a `<p>` would be proof the value took the Markdown route.
- **`bereich` is a bounded number input, mutation-proven.** `min` and `max` are emitted only under a `{{with}}` on the definition's bounds. Making them unconditional was tried and the test failed naming the unbounded field — the cheap grep would not have caught an empty `min=""`, which is a bound that is not one.
- **The placeholder D-08 depends on is there.** The `bereich` branch carries `{{if eq .Switch "text"}}placeholder=" "{{end}}` exactly as `zahl` and `langtext` do, and the test asserts both the attribute and the surrounding `.feld-schalter--text` wrapper on a field that actually has a dependant. Without it the premise D-08 rests on could not even be tested in a browser.
- **`zeit` is out of `MayControl()`, with the reason written down.** A time input never matches `:placeholder-shown` — the identical reason `KindDate` is already excluded. The exclusion is a second `case` rather than a fourth name on the first one, so the reason sits beside the pair it explains.
- **`bereich` stays a legal controller and was not decided here.** D-08 is a conditional decision; the browser pass in 07-07 settles it and carries the reversal path. Nothing in this plan asserts it either way beyond keeping the premise testable.
- **The bounds' clearing clause — the one piece 07-02 had to leave open — is written.** `validate` now clears `RangeMin` and `RangeMax` for any definition whose kind is not `KindRange`, beside its two siblings. A Range that sets only one bound keeps it: open at one end is a deliberate statement, not a half-filled pair.
- **The search-index decision was made explicitly, because nothing had made it.** See Decisions Made.

## Task Commits

1. **Task 1 RED: the three kinds before they exist** — `2a20c47` (test) — the field package failed to build on `KindTime`, `KindRange`, `KindCode`
2. **Task 1 GREEN: constants, Kinds rows, Check, MayControl, Resolve/List, validate** — `210a6b9` (feat)
3. **Task 2: the three controls in the page editor** — `a9f7533` (feat)
4. **Task 3 RED: a Code field in a block before it has its element** — `349268c` (test)
5. **Task 3 GREEN: the escaped `<pre><code>` and the widened whitelist** — `b1de6c3` (feat)
6. **T-07-11's missing gate: a Code value is a `string`, not `template.HTML`** — `384dd3a` (test)

No REFACTOR commit in either TDD cycle: neither implementation needed cleanup.

## Files Created/Modified

- `internal/field/field.go` — `KindTime`/`KindRange`/`KindCode` with German doc comments stating why each and not the obvious alternative; three `Kinds` rows placed beside their relatives; `MayControl` split into two cases with `KindTime` excluded; `Check` cases for a time of day and an inclusive range; `ParseNumber`, `ParseTimeOfDay` and `rangeReason`
- `internal/field/render.go` — `KindRange` joined to `case KindNumber` so `Number.Raw` stays the typed string; `case KindTime` yielding a `*time.Time` or a typed nil; `List` gives a time entry its `Text` and leaves the date arm alone
- `internal/field/store.go` — `validate`'s third clearing clause; the inversion check switched onto `ParseNumber`; `strconv` dropped as it became unused
- `internal/field/field_test.go` — eight new German-named tests covering every bullet of Task 1's `<behavior>`, plus the widened `MayControl` table
- `internal/field/store_test.go` — a `bereichFeld` fixture and a `pruefeBereich` gate on all six read paths; the two clearing cases the plan asked for
- `internal/block/render.go` — `case field.KindCode` in `renderOwn`; `PlainText`'s whitelist gains `KindCode` and `KindMulti`
- `internal/block/block_test.go` — the escaping test, the empty-field test, the `PlainText` test, and an `artMitCode` fixture
- `cmd/holzcloud/templates/admin/field_input.html` — three branches placed beside their relatives: `code` after `langtext`, `bereich` after `zahl`, `zeit` after `datum`
- `cmd/holzcloud/assets/admin.css` — `.form-code` with `var(--font-mono)`, `tab-size: 4` and `white-space: pre`; the four `.feld-schalter--*` rules untouched
- `internal/admin/page_fields_kinds_test.go` *(new)* — the rendered-HTML assertions for all three kinds and the no-JavaScript gate
- `internal/admin/field_defs_test.go`, `internal/bundle/bundle_test.go` — fixtures moved onto a Range field (see Deviations)

## Decisions Made

**The search-index whitelist — recorded because no roadmap or context note made this call.** `PlainText` decides what the site's own search sees, and its stated reason is that a picture id and a date are not things anybody searches for. Applied to this phase's kinds:

| Kind | In the index? | Why |
|---|---|---|
| `code` | **yes** | It holds words — an address block, a configuration line, a snippet. A page built from blocks would otherwise be invisible to its own site's search for exactly the content the author cared most about. |
| `mehrfachauswahl` | **yes**, joined with a space | Its values are words. Joined via `field.SplitValues` + `strings.Join`, so the separator is not spelled out a second time; the stored newline column would put a column into an excerpt where a sentence belongs. |
| `zeit` | no | A clock reading. The whitelist's own comment already explains why a date does not belong; a time is the same case. |
| `bereich` | no | A number. `"12"` in the excerpt of a recipe is worse than nothing — the comment's own example. |
| `schlagwort` | no | 07-05 excludes it from block kinds entirely, so it can never appear here. |

**`ParseNumber` and `ParseTimeOfDay` are the one reading each.** `Check` measures the entered value and both bounds through the same function, and `validate`'s inverted-pair check in `store.go` was switched onto it too. Before, `validate` used a bare `strconv.ParseFloat` while `Check` replaced the comma first — so `"0,5"`/`"0,2"` was an inverted pair that `validate` could not see and `Check` would then enforce backwards. Two places reading the same digits differently is the kind of bug that only surfaces on the one pair nobody rechecks by hand.

**A time of day is keyed on the string's length.** `time.Parse`'s hour field accepts one digit, so `time.Parse("15:04", "9:30")` succeeds and returns half past nine. FIELD-04's behaviour spec requires `"9:30"` to be refused, so the layout is chosen by `len(value)` — 5 or 8 — and anything else is refused before parsing. That also refuses a timezone suffix (`"09:30+02:00"`, length 11) without a second rule.

**`MayControl` became two switch cases.** The exclusion could not be written as `case KindGroup, KindSection, KindDate, KindTime:` — Task 1's acceptance gate greps for the literal `case KindGroup, KindSection, KindDate` and `grep` matches substrings, so that spelling would have tripped its own gate. Splitting it puts the shared reason (neither ever matches `:placeholder-shown`) beside the pair it explains, and states in the same comment why `KindRange` is deliberately absent.

**`bereich` was not decided here, on purpose.** D-08 keeps `MayControl()` true for it. The test's `darf` list names it, with a comment saying the browser pass in 07-07 is what settles it. Nothing was inferred from reading.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The new clearing clause broke three existing test suites**

- **Found during:** Task 1 (GREEN, immediately after `validate` gained its third clause)
- **Issue:** Wave 2 could not name a bounded kind, so every fixture it wrote hung `RangeMin`/`RangeMax` on whatever kind was convenient — a Choice and a Multi in `internal/field/store_test.go`, a Choice in `internal/admin/field_defs_test.go`, a Choice, a Multi and a group sub-field in `internal/bundle/bundle_test.go`, and a plain Text field in the bound-parameter test for T-07-05. The moment the clause exists, `validate` strips the bounds from all of them and eight assertions fail. The T-07-05 test was the worst of the three: a cleared value cannot break a SQL statement, so the test would have kept passing while proving nothing.
- **Fix:** The bounds moved onto a `bereich` field in each suite. `internal/field/store_test.go` gained a `bereichFeld` fixture and a `pruefeBereich` gate applied on all six read paths, so no read path lost coverage of `min_wert`/`max_wert` — it gained it, since each column now sits on the kind that owns it. `internal/admin/field_defs_test.go` got a third form submission for a Range field and its redraw check now opens that field, with the `darstellung` redraw moved to its own request against the Choice. `internal/bundle/bundle_test.go` got a `menge` page field and a `gmenge` group sub-field. The T-07-05 bound-parameter test became a `KindRange` definition, with a comment saying why.
- **Files modified:** `internal/field/store_test.go`, `internal/admin/field_defs_test.go`, `internal/bundle/bundle_test.go`
- **Verification:** `go test ./...` is green tree-wide. All four new columns are still asserted non-zero on each of the five store read paths and after `Update`.
- **Committed in:** `210a6b9`

**2. [Rule 2 - Missing Critical] `validate` and `Check` read the same bounds differently**

- **Found during:** Task 1 (writing the `KindRange` Check arm)
- **Issue:** `Check` for `KindNumber` replaces a comma with a dot before parsing — that is what somebody with a keyboard here types. `validate`'s inverted-pair check used a bare `strconv.ParseFloat`, so `RangeMin: "0,5"`, `RangeMax: "0,2"` was stored without complaint and would then have been enforced by a `Check` arm that read both as numbers, refusing every value.
- **Fix:** `ParseNumber` was extracted as the single reading and used by both. The comment in `store.go` names the reason.
- **Files modified:** `internal/field/field.go`, `internal/field/store.go`
- **Verification:** `TestNeueSpaltenGeprueft` still passes on both its inversion cases; `TestBereichPruefung` accepts `"5,5"` between bounds 1 and 10.
- **Committed in:** `210a6b9`

**3. [Rule 1 - Bug] Two of this plan's own new assertions measured the wrong thing**

- **Found during:** Task 1 and Task 3 (first runs of the new tests)
- **Issue:** (a) `TestBereichPruefung` required every refusal reason to name a bound, including for `"viel"`, which is refused for not being a number at all. (b) `TestCodeImBausteinWirdMaskiert` asserted the output contains no `"<p"` to prove the value never went through Markdown — but `"<p"` is a prefix of `"<pre"`, the element the task exists to produce.
- **Fix:** (a) The reason check now branches: a non-number must name the number, a number outside its bounds must name a bound. (b) The Markdown check looks for `"<p>"` and `"<p "` — both of which the previous, wrong output did carry, so the assertion keeps its teeth.
- **Files modified:** `internal/field/field_test.go`, `internal/block/block_test.go`
- **Verification:** Re-ran RED for (b) mentally against the recorded failure output — the `default:` arm emits `<p class=`, which `"<p "` catches.
- **Committed in:** `210a6b9`, `b1de6c3`

**4. [Rule 2 - Missing Critical] T-07-11 had no gate at all**

- **Found during:** Task 3 (plan-level verification: "HTML typed into a `code` field appears escaped both on a plain page and inside a block")
- **Issue:** The block half is now covered. The plain-page half rests entirely on the value reaching a theme as a `string`, so that `html/template` escapes it contextually. Nothing asserted the type. A later retype to `template.HTML` anywhere on the path would not fail any test, because a `string` and a `template.HTML` print identically in every passing case — the difference only shows when somebody types a tag.
- **Fix:** `TestCodeIstRoherText` now asserts the resolved value is exactly a `string`. `grep -rn "template.HTML" internal/field/ internal/block/` returns nothing, so no cast exists on the path.
- **Files modified:** `internal/field/field_test.go`
- **Verification:** `go test ./internal/field/ -run TestCodeIstRoherText` passes.
- **Committed in:** `384dd3a`

---

**Total deviations:** 4 auto-fixed (2 bugs, 2 missing critical)
**Impact on plan:** Deviation 1 touched two files outside the plan's `files_modified` list — `internal/admin/field_defs_test.go` and `internal/bundle/bundle_test.go`. Both are test fixtures that this plan's own clearing clause invalidated; the alternative was a red tree. No production file outside the list was touched. No scope creep.

## Issues Encountered

- **The three constants sit at the end of the `const` block, not beside their relatives.** The block is a `gofmt`-aligned group, and a doc comment splits the alignment. `KindMulti` set the precedent in Wave 1 for the same reason. The `Kinds` slice — the list an operator actually sees — does place them beside their relatives: `Bereich` after `Zahl`, `Uhrzeit` after `Datum`, `Code` after `Langer Text`.
- **`SubKinds()` and `BlockKinds()` needed no edit at all**, exactly as the plan predicted. `TestNeueArtenStehenInBeidenListen` asserts all three are in both, `code` in particular — the standing proof that ROADMAP criterion 5 can even be stated.

## Known Stubs

None. Every branch added is wired to real data and covered by a passing test.

## Threat Flags

None new. The register's dispositions for this plan:

| Threat | Disposition | Status |
|---|---|---|
| T-07-10 (elevation via a Code field inside a block) | mitigate | `html.EscapeString` in the block render path, never `prose`. `TestCodeImBausteinWirdMaskiert` asserts a `<script>`, a quote and an ampersand come out escaped, the raw tag is absent, and no `<p>` appears. |
| T-07-11 (elevation via a Code field on a plain page) | mitigate | `Resolve` yields a `string`, asserted by type; `List` puts it in `Entry.Text`; no `template.HTML` cast exists in `internal/field/` or `internal/block/`. TEMPLATE-SPEC's per-kind section is 07-07's. |
| T-07-12 (tampering with a Range bound) | mitigate | Enforced server-side in `Check`, inclusively at each end. `TestBereichPruefung` asserts the bound passes and one step either side is refused, for five bound configurations. The `min`/`max` attributes are the browser's courtesy only. |
| T-07-13 (denial of service via a crafted time) | mitigate | The layout is chosen by string length and only 5- or 8-character values reach `time.Parse` at all; anything else is refused without parsing. |
| T-07-14 (information disclosure via the widened whitelist) | mitigate | `PlainText` runs over the blocks of the page being indexed; no new query and no change to the draft or visibility rules upstream. |
| T-07-SC (package-manager installs) | accept | No install of any kind. Every symbol is Go standard library or already in `go.mod`. |

## Checkpoint Gates

**None.** This plan carried no `checkpoint:*` task — all three tasks were `type="auto"`. No gate was auto-approved, and nothing was decided on the developer's behalf.

The one decision this plan deliberately did **not** make is D-08 (whether `bereich` may control dependent fields). It is not a checkpoint here; plan 07-07 settles it by observation in the browser and carries the written reversal path.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

**Ready for 07-04.**

- `field.KindTime`, `field.KindRange` and `field.KindCode` exist and every later wave may reference them. `MaxValueBytes` for a Code field is 07-04's, as the plan states.
- `validate` is now complete: all three clearing clauses stand side by side, and no piece of Wave 2 is left open.
- **Open, by design:** `go run ./tools/i18n` reports `20 offen` in each of en/es/fr/it — 15 inherited from Waves 1–2, 5 new here (three kind labels, three hints, minus overlaps). The translation gate is plan **07-07**'s, on its own commit, per the ROADMAP wave plan.
- **Open, by design:** the per-kind documentation tax — `SampleData`, `MinimalData` and `TEMPLATE-SPEC.md` rows for the three kinds. Paid once for the whole phase in 07-07. The `internal/tmplspec` suite does not fail on a missing kind (it ties struct paths, helpers and view files, not kinds), so nothing is red in the meantime.
- **Open, by design:** the browser pass (ROADMAP criterion 6 / QUAL-02), which is also what settles D-08. The three controls have never been seen in a browser — only through rendered-HTML assertions.

---
*Phase: 07-field-kinds*
*Completed: 2026-09-05*

## Self-Check: PASSED

- The created file exists on disk: `internal/admin/page_fields_kinds_test.go`. All eleven modified files exist.
- All six task commits are in the log: `2a20c47`, `210a6b9`, `a9f7533`, `349268c`, `b1de6c3`, `384dd3a`. No commit deleted a tracked file (`git diff --diff-filter=D 36afbc6..HEAD` is empty).
- Every task-level `<acceptance_criteria>` re-run and passing: `grep -c 'KindTime' internal/field/field.go` = 5; the old `MayControl` shape grep prints nothing; `grep -v '^\s*//' internal/field/store.go | grep -c 'KindRange'` = 1; the three `{{else if .Is …}}` branches present; `grep -c 'placeholder=" "'` = 5; `form-code` in `admin.css` with no `url(` (file-wide `url(` count 0, unchanged); `feld-schalter` count 5, unchanged; `PlainText` names `KindCode` and `KindMulti` and `grep -c KindTerm internal/block/render.go` = 0.
- Plan-level `<verification>`: `go build ./...` succeeds; `go vet ./...` is clean; `gofmt -l internal/ cmd/` prints nothing; `go test ./...` is green tree-wide with zero `--- FAIL`; the `internal/block` suite reports 28 passing tests and no `no test files`.
- The conditional `min`/`max` was proven by mutation: emitting them unconditionally makes `TestFeldartenImSeiteneditor` fail naming the unbounded field, and the change was reverted.
