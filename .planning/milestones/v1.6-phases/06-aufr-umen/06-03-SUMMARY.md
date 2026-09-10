---
phase: 06-aufr-umen
plan: 03
subsystem: testing
tags: [go, encoding/json, i18n, tooling, round-trip-test, format-lock]

requires:
  - phase: 06-01
    provides: the amended ground truth (REQUIREMENTS, ROADMAP) that MAINT-01 and MAINT-02 are read against
provides:
  - "`tools/i18n/writeCatalog` serialises through `encoding/json` end to end; the per-string `quote` helper is gone"
  - "`tools/i18n/main_test.go` — the first test this command has ever had: a byte-for-byte round trip over all seven catalogues plus a seven-file count assertion"
  - "the package doc comment states which regional catalogues are generated and which are hand-maintained"
  - "the per-regional-file report line says the same on stdout on every run"
  - "evidence that the seven committed catalogues are byte-unchanged under both writing modes"
affects: [06-06, 06-07, i18n, ci]

actuals:
  tokens: 2000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Characterization round-trip test as a format lock: read the committed artefact, put it through the tool, compare bytes"
    - "A count assertion beside the per-file assertions, so a new file cannot slip past by not being looked at"

key-files:
  created:
    - tools/i18n/main_test.go
  modified:
    - tools/i18n/main.go

key-decisions:
  - "The RED gate was proven by drift, not by absence: D-13 mandates byte-identical output, so a lock test on already-canonical behaviour cannot fail before the implementation exists. Teeth were shown by reindenting and by de-newlining it-CH.json."
  - "Task 1 acceptance criterion 11 (deleting a key from en.json fails the test) is factually wrong and was not forced to pass — a round-trip test locks the format, not the key set. Measured, reported, and the real guard for a deleted key identified."
  - "The writer rewrite is committed as `refactor`, not `feat`: output is byte-identical by design, so `feat` would misdescribe it."

patterns-established:
  - "tools/ tests are English (following tools/mkbundle/pack_test.go) even where the research draft is German"
  - "A doc comment that names the near-misses (Encoder.SetIndent(\"\",\"\"), json.MarshalIndent) so the next reader does not 'simplify' into either"

requirements-completed: [MAINT-01, MAINT-02]

coverage:
  - id: D1
    description: "`writeCatalog` serialises the whole catalogue through `encoding/json` with `SetEscapeHTML(false)` then `json.Indent(dst, src, \"\", \"\")`; the per-string `quote` helper is deleted"
    requirement: MAINT-01
    verification:
      - kind: unit
        ref: "tools/i18n/main_test.go#TestCatalogsSurviveTheRoundTrip"
        status: pass
      - kind: other
        ref: "grep -c 'func quote' tools/i18n/main.go => 0; grep -c 'SetEscapeHTML(false)' => 1; grep -c 'json.Indent' => 1; MarshalIndent/SetIndent outside comments => 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "A round-trip test reads each of the seven committed catalogues, writes it back through the writer and compares byte for byte, and fails if a new locale appears outside the lock"
    requirement: MAINT-01
    verification:
      - kind: unit
        ref: "tools/i18n/main_test.go#TestCatalogsSurviveTheRoundTrip (7/7 byte-identical, count assertion seen == 7)"
        status: pass
      - kind: other
        ref: "negative control: reindent one line of it-CH.json => FAIL naming it-CH.json (2597 on disk, 2595 written); strip its final newline => FAIL (2594 on disk, 2595 written); git checkout => PASS"
        status: pass
    human_judgment: false
  - id: D3
    description: "The seven committed catalogue files change zero bytes as a result of the rewrite"
    requirement: MAINT-01
    verification:
      - kind: integration
        ref: "go run ./tools/i18n -write && go run ./tools/i18n -schweiz && git diff --exit-code -- internal/i18n/locales/ => exit 0"
        status: pass
      - kind: other
        ref: "jq -S semantic diff of each of the seven files against HEAD => no output for all seven (jq-1.7.1-apple)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The package doc comment explains that `de-CH.json` is rebuilt from a mechanical rule while `fr-CH.json` and `it-CH.json` are maintained by hand and only ever read"
    requirement: MAINT-02
    verification:
      - kind: other
        ref: "sed -n '1,30p' tools/i18n/main.go contains fr-CH (1), it-CH (1), de-CH (2), -schweiz (2)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The per-regional-file report line names the maintenance mode on every run, with the `%-12s` column alignment preserved"
    requirement: MAINT-02
    verification:
      - kind: integration
        ref: "go run ./tools/i18n => de-CH.json '… — wird von -schweiz erzeugt'; fr-CH.json and it-CH.json '… — nur gelesen, von Hand gepflegt'; all three second columns start at offset 13; longest line 79 chars"
        status: pass
    human_judgment: false
  - id: D6
    description: "The writer's doc comment describes the output shape the tool actually produces"
    requirement: MAINT-02
    verification:
      - kind: other
        ref: "grep -c 'sorted and indented' tools/i18n/main.go => 0; the replacement reads 'sorted and flush left, one key per line'"
        status: pass
    human_judgment: false

duration: 8 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 03: i18n Format Lock and Self-Description Summary

**`tools/i18n` now serialises catalogues through `encoding/json` end to end, its first-ever test locks the format of all seven committed catalogues byte for byte, and the tool says out loud — in the doc comment and on stdout — that it generates `de-CH.json` and only ever reads `fr-CH.json` and `it-CH.json`.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-09-04T10:34:00Z (approx.)
- **Completed:** 2026-09-04T10:42:35Z
- **Tasks:** 3
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- `writeCatalog` is `encoding/json` end to end: `json.NewEncoder` with `SetEscapeHTML(false)` into a `bytes.Buffer`, then the free function `json.Indent` with an empty prefix and an empty indent, then `os.WriteFile` at the existing `0o644`. The per-string `quote` helper — the same technique one string at a time — is deleted, and its rationale paragraph moved onto the writer.
- `tools/i18n/main_test.go` is the first test this command has ever had. It reads the seven real catalogues, puts each through `readCatalog` → `writeCatalog` → `t.TempDir()`, and compares bytes. All seven round-trip identically.
- The lock has a count assertion (`seen != 7`), so an eighth locale cannot join `internal/i18n/locales/` outside the lock.
- The package doc comment and the per-regional-file report line both now state which regional catalogues are written and which are not (D-16, D-17).
- The seven committed catalogues are **byte-unchanged**, proven mechanically under both writing modes and cross-checked with a `jq -S` semantic diff.

## Task Commits

1. **Task 1 (RED): round-trip lock over all seven catalogues** — `a3fbe67` (test)
2. **Task 1 (writer): serialise through `encoding/json`** — `af4dbd1` (refactor)
3. **Task 2: doc comment + report line self-description** — `8ea8d70` (docs)
4. **Task 3: evidence only — no files changed, results recorded below**

## Files Created/Modified

- `tools/i18n/main_test.go` (new, 82 lines) — `TestCatalogsSurviveTheRoundTrip`; English throughout, following `tools/mkbundle/pack_test.go`
- `tools/i18n/main.go` — `writeCatalog` rewritten, `quote` deleted, `bytes` imported, package doc paragraph added, report line extended

## Task 3 Evidence

### Byte identity of the seven catalogues

`go run ./tools/i18n -write` → `go run ./tools/i18n -schweiz` → `git diff --exit-code -- internal/i18n/locales/` — **exit status 0**, run twice (after Task 1 and again at the end of Task 3).

| File | Bytes | Keys (reported) |
|------|-------|-----------------|
| `de-CH.json` | 12 471 | 51 Abweichungen |
| `en.json` | 96 517 | 1128 übersetzt, 0 offen |
| `es.json` | 101 265 | 1128 übersetzt, 0 offen |
| `fr-CH.json` | 1 089 | 4 Abweichungen |
| `fr.json` | 103 465 | 1128 übersetzt, 0 offen |
| `it-CH.json` | 2 595 | 9 Abweichungen |
| `it.json` | 100 848 | 1128 übersetzt, 0 offen |
| **total** | **418 250** | |

These match the committed sizes exactly — the sizes are unchanged from `HEAD` before this plan.

### Semantic diff (the check the roadmap demands before any catalogue could be committed)

`jq` **is** installed (`jq-1.7.1-apple`), so the fallback was not needed. For each of the seven files, `diff <(jq -S . "$f") <(git show "HEAD:$f" | jq -S .)` produced **no output**. Content and format are both unchanged, so there is no catalogue commit to make and PITFALLS #25's "reformat is its own commit" rule has nothing to apply to.

### The lock has teeth

Two independent negative controls against the new writer:

| Corruption of `it-CH.json` | `go test ./tools/i18n/` |
|---|---|
| one line reindented by two spaces | **FAIL** — `it-CH.json: writeCatalog does not return the file it read (2597 bytes on disk, 2595 written)` |
| final newline stripped | **FAIL** — `it-CH.json: writeCatalog does not return the file it read (2594 bytes on disk, 2595 written)` |
| restored with `git checkout --` | **PASS** |

`git status --porcelain -- internal/i18n/locales/` prints nothing at the end of the task, and `go test ./tools/i18n/` exits 0.

## Decisions Made

- **The RED gate is proven by drift, not by absence.** Task 1 is marked `tdd="true"`, but the classic RED — a test that fails because the implementation is missing — is unreachable here by construction: D-13 requires the new writer to produce byte-identical output, and quick task `260903-bsk` already made the committed catalogues match the old writer. A lock test on already-canonical behaviour passes on the day it is written, and that is correct. So the test was committed first (`a3fbe67`), run against the **old** writer to prove it characterises current behaviour, and its ability to fail was demonstrated by corrupting a real file. Then the writer was replaced and the same test re-run. That ordering keeps the safety net that RED exists to provide.
- **The writer rewrite is `refactor`, not `feat`.** Output is byte-identical by design; `feat` would claim a behaviour change that measurement says does not exist. CLAUDE.md's commit-type table puts "no behaviour change" under `refactor`.
- **`sort` stays imported and the manual key sort is gone from the writer.** `main()` still calls `sort.Strings` at two places; `encoding/json` sorts map keys byte-wise, which is the same order.

## Deviations from Plan

### Findings — no code deviation

**1. [Finding] Task 1 acceptance criterion 11 is factually wrong; it was measured rather than forced to pass**

- **Found during:** Task 1, verification loop
- **Criterion as written:** "Deleting one key from a copy of `en.json` and re-running the test makes it fail — the lock has teeth. Verify by hand once, then restore."
- **What was measured:** deleting line 5 of `en.json` (`"%d Dateien": "%d files",`) and re-running `go test ./tools/i18n/` → **PASS**, i.e. the criterion does not hold. The reason is structural, not a defect in the test: after a clean line deletion the file is still valid JSON in the tool's canonical format, so `readCatalog` → `writeCatalog` reproduces it byte for byte. A round-trip test locks the **format**, not the **key set**. The only way key deletion trips it is if the *last* entry is removed, leaving a trailing comma — which fails as a parse error, not as a format mismatch.
- **What the real guard is, measured:** with the same key deleted, `go run ./tools/i18n -write` re-adds it and `git diff --exit-code -- internal/i18n/locales/` exits **1**. So a deleted key is caught by the `-write` + `git diff` pair — which is exactly the CI step D-15 specifies and plan `06-06` installs. The criterion's *intent* (drift is caught) holds; its stated mechanism does not.
- **What was done instead:** the criterion's intent was satisfied by two negative controls that do exercise the format lock — reindenting a line and stripping the final newline, both of which fail the test naming `it-CH.json` with both byte lengths (see Task 3 Evidence). The file was restored with `git checkout --` and the test re-run green.
- **Files modified:** none — this is a measurement, and every corrupted file was restored. `git status --porcelain -- internal/i18n/locales/` is empty.
- **Action for a later plan:** none required. `06-06` already installs the CI step that is the real guard for a deleted key.

**2. [Finding] D-17a confirmed WITHDRAWN against this tree, as the plan instructed**

- **Found during:** Task 1, before editing
- **Measured:** `grep -n 'sorted and indented' tools/i18n/main.go` → **287**; `func quote` → 277; `func writeCatalog` → 289. Exactly what D-17a's withdrawal states, and the roadmap's `main.go:287` pointer was correct. The withdrawn "correction" to `:288` was not acted on.
- **Files modified:** none beyond the planned rewrite.

---

**Total deviations:** 0 code deviations. 2 findings recorded (1 incorrect acceptance criterion, measured and worked around without weakening the lock; 1 confirmation).
**Impact on plan:** None on scope or output. Every artefact the plan names exists, and the one criterion that could not be satisfied as written was replaced with two stronger controls rather than skipped.

## Issues Encountered

None. The `encoding/json` route reproduced the committed format on the first attempt, as `06-RESEARCH.md` measured it would.

## Verification Results

| Check | Result |
|---|---|
| `go test ./tools/i18n/ -run TestCatalogsSurviveTheRoundTrip -v` | `--- PASS`, exit 0; no `no test files`, no `no tests to run` |
| `go run ./tools/i18n -write && -schweiz && git diff --exit-code -- internal/i18n/locales/` | exit 0, no diff |
| `go vet ./tools/i18n/` | exit 0 |
| `gofmt -l tools/i18n/` | no output |
| `go build ./...` | exit 0 |
| `go test ./...` (whole repo) | no failures |
| `go run ./tools/i18n` regional lines | all three carry a maintenance-mode clause; `de-CH` names `-schweiz` |
| `git diff --name-only -- .planning/ROADMAP.md` | empty — this plan did not touch the roadmap |

## Self-Check: PASSED

- `tools/i18n/main_test.go` — FOUND on disk
- `tools/i18n/main.go` — FOUND on disk
- `a3fbe67`, `af4dbd1`, `8ea8d70` — all FOUND in `git log`
- All plan-level `<verification>` commands re-run at close-out; all pass
- All task `<acceptance_criteria>` re-run; all pass except Task 1 criterion 11, documented above as a measured-wrong criterion with two stronger substitutes

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **For `06-06` (CI):** D-15's CI step can be added as specified. The exact command chain is proven working on this tree: `go run ./tools/i18n -write`, `go run ./tools/i18n -schweiz`, `git diff --exit-code -- internal/i18n/locales/`. Note the scope boundary D-15 draws: this proves canonical format and step-with-source, not "0 offen".
- **For `06-07` (stale notes):** the ROADMAP planning note pointing at `tools/i18n/main.go:287` now has nothing to point at — the "sorted and indented" claim is gone. `06-07` Task 3 retires that note as planned. This plan deliberately left `.planning/ROADMAP.md` untouched.
- **No blockers.** `internal/i18n/locales/` is byte-identical to what it was before this plan, so nothing downstream needs to re-read a catalogue.

---
*Phase: 06-aufr-umen*
*Completed: 2026-09-04*
