---
phase: 09-csv-import
plan: 03
subsystem: csv-import
status: complete
tags: [csv, mapping, verdict, row-function, i18n, unicode]

requires:
  - internal/csv (plan 09-01) — Reader, Row, RowNumber, MaxCellBytes
  - internal/csvimport (plan 09-02) — Store, Upload
  - internal/field — SlugifyKey, CheckAll, Clean, JoinValues, SplitValues, Encode, Decode
  - internal/page — CreatePage, UpdatePage, TrashPage, PurgePage, Slugify, ValidateSlug, Transliterate, RenderMarkdown
  - internal/term — Normalize, EnsureNames, SetForPage, MaxPerPage
provides:
  - csvimport.Target, Column, Columns, Mappable, AutoMap, Mapping, Mapping.ColumnFor, Note
  - csvimport.foldHeader, settleMarks (unexported)
  - csvimport.Verdict, Outcome, Reason and sixteen codes, Verdict.Written, Verdict.GroupKey
  - csvimport.CheckRow, Writer, Writer.WriteRow, RowTerms, TermNames
  - csvimport.CollisionSkip, CollisionUpdate
affects:
  - plan 09-04 (the wizard screens) and 09-05 (the report) consume every symbol above
  - .planning/GLOSSARY.md gained ten entries

tech-stack:
  added: []
  patterns:
    - a reason is a code plus arguments; the sentence is a {{tf}} literal in a template
    - a closed vocabulary is reported when a cell falls outside it, never guessed
    - combining marks are settled before any fold, in one helper

key-files:
  created:
    - internal/csvimport/mapping.go
    - internal/csvimport/mapping_test.go
    - internal/csvimport/verdict.go
    - internal/csvimport/verdict_test.go
    - internal/csvimport/row.go
    - internal/csvimport/row_test.go
  modified:
    - .planning/GLOSSARY.md

decisions:
  - foldHeader composes combining marks onto their base letter rather than stripping them; stripping alone does not make the two spellings of one word meet
  - CheckRow takes the collision setting as a parameter, which the plan's signature omitted
  - the writer type is Writer, not Speicher/Store, because csvimport.Store is already the staging table
  - no display label ("Spalte N") is minted in Go; the template mints it from Column.Number
  - the update path replaces only what the mapping points at and leaves Kind empty

metrics:
  duration: ~1h
  completed: 2026-09-06

actuals:
  tokens: 21000
  tasks: 3
  commits: 4
---

# Phase 9 Plan 03: The mapping, the verdict and the row function — Summary

Everything between a parsed row and a `page.PageCreate`, in three files with no
screen in sight: a column addressed by its position, a verdict carrying a code
instead of a sentence, and one function that decides a row's fate which both the
dry run and the write call.

## What shipped

**`internal/csvimport/mapping.go`** — `foldHeader` (the one header fold),
`settleMarks` (shared with the cell folds), `Target` and its seven kinds,
`Column`/`Columns` addressed by index, `Mappable`, `AutoMap`, `Mapping` with
`Targets`, `Notes`, `Defaults` and `ColumnFor`.

**`internal/csvimport/verdict.go`** — `Outcome` (`create`/`update`/`skip`),
`Reason` with sixteen codes, `Verdict{Row, Outcome, Reason, Args}`,
`Verdict.Written()` and `Verdict.GroupKey()`. The file imports no `fmt`.

**`internal/csvimport/row.go`** — `CollisionSkip`/`CollisionUpdate` (the two
German values migration 00049 pins), `foldCell`, the status and janein
vocabularies, `termNames`, `RowTerms`, `TermNames`, `cellFor`, `CheckRow`, and
`Writer` with `WriteRow`, `update` and `setTerms`.

Tests: 40 in the package, all passing; the whole suite is 43 `ok` and no `FAIL`.

## Commits

| Task | Commit | Subject |
|---|---|---|
| 1 | `b7d6254` | feat(09-03): a column is a position, and a heading folds in one place |
| 2 | `b0edcb3` | feat(09-03): a row's outcome is a code and its arguments, never a sentence |
| 3 | `03d357a` | feat(09-03): one function decides a row, and the write is what it does not reach |
| — | `486c362` | docs(glossar): die zehn Woerter, die Welle 3 gepraegt hat |

Every commit staged named files only. `git show --stat` confirms nothing outside
`internal/csvimport/` and `.planning/GLOSSARY.md` was touched.

## Verification gate output, literal

Task 1:

```
=== gate 1: build/vet/gofmt ===        exit=0   (gofmt printed nothing)
=== gate 2: tests ===                  9 PASS, 0 FAIL, 0 "no tests to run"
=== gate 3: SlugifyKey outside mapping.go ===   3      <-- diverges, see below
=== gate 4: unicode.Mn ===             1
=== gate 5: go.mod/go.sum unchanged === exit=0
=== gate 6: net/http in test imports === 0
```

Task 2:

```
=== gate 1 build/vet/gofmt ===         exit=0
=== gate 2: "fmt" in verdict.go ===    0
=== gate 3: German sentence via Sprintf === 0
=== gate 4: reason code count ===      14      <-- at the time of the commit; 16 now
=== gate 5: package tests ===          ok      exit=0
```

Task 3:

```
=== gate 1 build/vet/gofmt ===         exit=0
=== gate 2 verbose tests ===           40 PASS, 0 FAIL / no-tests
=== gate 3 BeginTx in csv+csvimport === 0
=== gate 4 BeginTx files in internal/ === 14   (baseline 14, unchanged)
=== gate 5 JoinValues in row.go ===    1
=== gate 6 strings.Join with a newline === 0
=== gate 7 PurgePage in row.go ===     1
=== gate 8 fmt.Sprintf in row.go ===   0
=== gate 9 CheckAll in row.go ===      2
=== gate 10 whole suite ===            exit=0, 43 ok, no FAIL
```

Cross-cutting:

```
umlauts in the six files this wave wrote   0
internal/csv purity (foreign deps)         0
go.mod / go.sum diff                       exit=0
go run ./tools/i18n                        0 offen, 0 verwaist (unchanged)
```

## Counting divergences

### 1. Task 1's gate 3 measures 3 where its name measures 0 — it counts mentions, not calls

**Direction:** upward, from the expected 0 to 3.

**Cause:** the gate is `grep -rn 'SlugifyKey' internal/csvimport/ …`, a plain text
grep. All three hits are in `mapping_test.go` and none of them is a call site:
line 43 is inside a `t.Errorf` message, lines 45 and 61 are comment prose
explaining what `field.SlugifyKey` does. The property the gate is named for —
"a second caller folds a header without stripping combining marks first" —
holds: the honest form of the gate,

```
grep -rn 'field\.SlugifyKey(' internal/csvimport/ internal/admin/csvimport.go | grep -v '^internal/csvimport/mapping.go' | wc -l
```

prints **0**.

**Nothing was adjusted to satisfy the text gate.** Renaming the prose would be
exactly the "small adjustment" wave 2 was told not to make, and it would cost
the test file the sentences that explain why the fold exists. This is the third
gate in this phase whose command measures text where its name measures
structure. **The correct gate for plan 09-04 onwards is the parenthesised form
above.**

### 2. Task 2's reason-code gate reads 14 at commit time and 16 now

**Direction:** upward, away from the failure threshold. The gate requires at
least 10. Task 3 added `ReasonNotWritten` and `ReasonBodyUnreadable` because the
row function needed things to say that the plan's table did not list, which is
what the plan's own instruction ("add codes as the row function needs them")
asks for. Not a defect in either direction.

### 3. Task 1's `-run` pattern would have matched nothing

**Direction:** the gate would have failed with "no tests to run".

**Cause:** the plan prescribes `-run 'foldHeader|Columns|Kopf|Definition|Target|Felder'`,
which is a leftover from the German test names the plan was rewritten away from.
The equivalent English pattern was used and is recorded above:
`-run 'FoldHeader|Columns|Heading|Definition|Target|Fields'`, which matches all
nine of task 1's tests. Same nine tests, same property; only the pattern turned.

## Deviations from plan

### 1. [Rule 1 — Bug] `foldHeader` composes marks; stripping them does not close D-28's hole

**Found during:** Task 1, writing the NFD test.

**Issue:** the plan and D-28 prescribe stripping `unicode.Mn` before
`field.SlugifyKey`, in three lines. Measured against `SlugifyKey`, that does not
produce the behaviour the plan's own `<behavior>` block demands. `SlugifyKey`
writes `oe` for the single rune U+00F6, so the composed spelling folds to
`groesse`; strip the mark from the decomposed spelling and the base letter alone
folds to `grosse`. Two keys for one word, still not meeting, and the heading
still fails to match its own field.

**Fix:** `settleMarks` puts a combining diaeresis back onto `a`, `o`, `u` and
their capitals — the three vowels `SlugifyKey` and `page.Transliterate` know as
single runes — and drops every other mark, keeping its base. `foldHeader` is
then `field.SlugifyKey(settleMarks(header))`. `unicode` only; `go.mod` untouched
and the gate on it green.

**Files:** `internal/csvimport/mapping.go`. **Commit:** `b7d6254`.

### 2. [Rule 1 — Bug] The plan's own separator expectation contradicts `SlugifyKey`

**Found during:** Task 1, first test run.

**Issue:** the plan states `foldHeader("Ti-tel")` returns `titel`. It returns
`ti_tel`: `SlugifyKey` collapses a space, a hyphen and an underscore each to a
single underscore rather than removing them. The matching is unaffected — a
field defined under the label `Ti-tel` carries the key `ti_tel` by the same
derivation — so the two agree by construction, which is the property that
matters.

**Fix:** the test asserts the true value and carries the explanation. The plan's
sentence is wrong about the helper it mandates reusing; nothing in the code was
bent to make the sentence true.

**Files:** `internal/csvimport/mapping_test.go`. **Commit:** `b7d6254`.

### 3. [Rule 3 — Blocking] `CheckRow` takes the collision setting

**Issue:** the plan's signature is
`CheckRow(defs, z, zu, vorhanden)`, and its own step 11 says the outcome is
decided "according to `Kollision`", which that signature cannot see.

**Fix:** a fifth parameter, `collision string`, plus exported constants
`CollisionSkip = "uebergehen"` and `CollisionUpdate = "aktualisieren"` carrying
the comment that these are values pinned by migration 00049's CHECK and are data
rather than identifiers.

**Files:** `internal/csvimport/row.go`. **Commit:** `03d357a`.

### 4. [Rule 3 — Blocking] The writer is `Writer`, not `Store`

**Issue:** the plan's artifact list names both `csvimport.Store` and
`Speicher.WriteRow`. `csvimport.Store` already exists — it is the staging table
shipped by wave 2 — so the name is taken.

**Fix:** `Writer{Pages *page.Store, Terms *term.Store}`, with the reason in its
doc comment and an entry in the glossary.

### 5. [Rule 2 — D-32] No display label is minted in Go

**Issue:** the plan's acceptance criterion says `Columns` names an empty heading
"Spalte N". Building that string in Go would put a sentence a person reads where
`tools/i18n` cannot see it — the exact hole D-32 exists to close — and
`fmt.Sprintf("Spalte %d", …)` would additionally trip task 2's own Sprintf gate,
which greps for `Spalte `.

**Fix:** `Column` carries `Index`, `Number` and the raw `Heading`, plus
`HasHeading()`. The fallback wording is a `{{tf}}` literal in the template of
plan 09-05, fed with `Number`. The doc comment says so.

### 6. [Rule 2] `Mapping.Notes` is `[]Note`, not `[]Reason`

**Issue:** the plan's struct has `Notes []Reason`, and the same plan requires
`ReasonColumnTaken` to carry the winning column's position as an argument. A
bare `Reason` cannot.

**Fix:** `Note{Reason Reason; Args []string}` — the same code-plus-arguments
shape as a verdict, for the same reason.

### 7. [Rule 2] `verdict_test.go` exists although the plan gives task 2 no test file

**Issue:** the plan lists only `verdict.go` for task 2, on the ground that the
row function's tests exercise it. They exercise the *codes*; they do not
exercise the type's own two promises — an empty reason means the row worked, and
two rows saying the same thing produce one grouping key.

**Fix:** four small tests, no database. A stated guarantee with no gate is not a
guarantee — wave 2's own words.

### 8. [Rule 2] `Mappable` refuses a kind this version does not know

Beyond the four exclusions D-18 names, an unrecognised kind is refused too: a
definition written by a newer version of the program would otherwise be fed a
cell nothing here can validate. `field.KnownKind` is the check.

### 9. [Rule 2] Two more reason codes

`ReasonNotWritten` (the store refused the row, or the row was taken back
cleanly) and `ReasonBodyUnreadable` (the Markdown could not be rendered). The
plan's table listed neither and instructs that codes be added as the row
function needs them. `ReasonCellTooLong`'s first argument is the column's
*position* and not its heading, because `CheckRow` is given a row and a mapping
and no header; widening the one decision of this phase for the sake of one
message was the worse trade, and the screen has the header anyway.

### 10. [Rule 2] The update path replaces only what the mapping points at

The plan says "only the mapped fields replaced" without saying how. `Writer.update`
starts from the page as it stands, overwrites the body, the status and the field
slots the mapping actually points at, leaves `Kind` empty so `page.UpdatePage`
keeps the page's classification, and keeps the excerpt, the schedule, the blocks
and the address. A mapped field column whose cell is empty *clears* its slot,
which is the difference between "the file says this is empty now" and "the file
says nothing about this". `TestUnmappedColumnsAreLeftAlone` is the gate.

### 11. [Rule 2] The compensation runs on the create path only

D-02 describes undoing a row through `TrashPage` then `PurgePage`. Applied to
the update path that would *delete a page the operator already had* because its
terms failed to write — catastrophic and clearly not what D-02 means. An update
whose terms fail is reported with `ReasonNotRolledBack` and the page is left
standing. `ReasonNotRolledBack`'s comment carries both cases.

### 12. [Rule 3] `settleMarks` extracted, and cells fold through `page.Transliterate`

The status and janein vocabularies have to recognise what an operator typed
however their editor normalised it, exactly as a heading does. They fold through
`foldCell = page.Transliterate(settleMarks(cell))` and deliberately **not**
through `foldHeader`: `SlugifyKey` is a key derivation and prefixes a leading
digit with `f`, so it would fold the cell `0` to `f0` and a column of noughts
would stop being recognisable. This also keeps the two vocabularies free of
literal umlauts, since `Transliterate` has already written `oe` by the time a
lookup happens.

### 13. [Rule 2] Term names settle their marks before `term.Normalize`

D-20's order is `term.Normalize` → `page.Slugify`. A term cell exported in NFD
would slugify to `mobel` where the composed spelling gives `moebel` — two terms
where the operator meant one. `settleMarks` runs first, for the same reason
D-28 gives for a heading. `TestTermNameBecomesASlug` writes one row composed and
one decomposed and asserts one term results.

## Notes for the next plans

- **Use the parenthesised SlugifyKey gate** (`grep 'field\.SlugifyKey('`), not
  the bare word. Recorded above as divergence 1.
- **`internal/csvimport/store.go` still writes the literal `"uebergehen"`** in
  `Stage`. `CollisionSkip` and `CollisionUpdate` now exist; wiring store.go to
  them is a one-line tidy that was out of this plan's file list. Logged as a
  deferred item, not a defect — the value is correct either way.
- **A required `gruppe` field refuses every row.** `field.CheckAll` is called
  with the full definition list, which is the point (one validator, no second
  spelling of the rules), and a group cannot be filled from a flat CSV row. The
  row is refused with `ReasonFieldRejected` naming the group, which the dry run
  shows before anything is written. Plan 09-05's mapping screen should say this
  where the operator can see it rather than leaving them to discover it in the
  report.
- **`Verdict.GroupKey` deliberately excludes the outcome.** The report groups by
  what went wrong and counts by what happened; those are two different
  questions and 09-05 needs both.

## Known Stubs

None. Every symbol this plan declares is reached by a test in this package.

## Threat Flags

None. No new network endpoint, no auth path, no file access and no schema
change. The body goes through `page.RenderMarkdown` — the one goldmark →
bluemonday chain — and no `template.HTML` cast exists anywhere on this road
(T-09-15). T-09-14 is measured by the `BeginTx` gates, both green.

## Self-Check: PASSED

- `internal/csvimport/mapping.go` — FOUND
- `internal/csvimport/mapping_test.go` — FOUND
- `internal/csvimport/verdict.go` — FOUND
- `internal/csvimport/verdict_test.go` — FOUND
- `internal/csvimport/row.go` — FOUND
- `internal/csvimport/row_test.go` — FOUND
- commit `b7d6254` — FOUND
- commit `b0edcb3` — FOUND
- commit `03d357a` — FOUND
- commit `486c362` — FOUND
