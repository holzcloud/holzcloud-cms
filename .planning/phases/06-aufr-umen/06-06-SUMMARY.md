---
phase: 06-aufr-umen
plan: 06
subsystem: infra
tags: [ci, github-actions, wasm, reproducible-builds, gotoolchain, buildvcs, i18n, sha256]

requires:
  - phase: 06-02
    provides: "`tools/wasm` with the `goToolchain` pin and the measured D-05 PASS (cross-host byte equality, run 33866318077) that authorises the ~21 MB rebuild; the temporary falsification step this plan deletes"
  - phase: 06-03
    provides: "`tools/i18n -write` / `-schweiz` writing through `encoding/json`, and the measurement that the `-write` + `git diff` pair — not the round-trip test — is what catches a deleted key"
  - phase: 06-05
    provides: "`tools/wasm` write mode, `-check` over all ten artifacts, deterministic archive packing, and the recorded observation that `go run` collapses the child exit code to 1"
provides:
  - "The zero point: all six committed guest modules and all four archives rebuilt once with `GOTOOLCHAIN=go1.26.6`, no VCS stamp, current module path — in one commit containing nothing but the ten artifacts"
  - "`go run ./tools/wasm -check` as a blocking CI step in the `test` job, after `Vet` and before `Build`, therefore ahead of `go test ./...`"
  - "`go run ./tools/i18n -write` + `-schweiz` + `git diff --exit-code -- internal/i18n/locales/` as a blocking CI step, the twin of `Verify go.mod is tidy`"
  - "The temporary cross-host hash step from 06-02 removed; no CI run builds the six guests twice"
  - "A ten-row before/after record of sha256, size and embedded compiler for every committed build artifact"
affects: [06-07, ci, plugins]

actuals:
  tokens: 600
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "An artifact-only commit: a rebuild that changes no source is staged path by path and asserted to contain exactly its artifacts, so `git log -S` over source stays usable across a 21 MB blob"
    - "A generated artifact is guarded in CI by running its generator and requiring an empty diff — `go mod tidy`, `tools/i18n`, and now `tools/wasm -check` all share the shape"
    - "A CI step whose comment states what it does NOT prove, so the next reader does not mistake a green run for a stronger claim than it carries"

key-files:
  created: []
  modified:
    - plugins/bestellung/plugin.wasm
    - plugins/jahreszahl/plugin.wasm
    - plugins/kontaktformular/plugin.wasm
    - plugins/nicht-gefunden/plugin.wasm
    - plugins/suche/plugin.wasm
    - internal/plugin/testdata/echo.wasm
    - plugins/bestellung/bestellung.zip
    - plugins/jahreszahl/jahreszahl.zip
    - plugins/nicht-gefunden/nicht-gefunden.zip
    - plugins/suche/suche.zip
    - .github/workflows/ci.yml

key-decisions:
  - "The two new CI comments are written in German, as the plan's acceptance criteria and `06-RESEARCH.md` both specify verbatim — against `06-02`'s English-for-consistency deviation. The plan is the later contract and it tests for German explicitly; the English comment 06-02 wrote was attached to the step this plan deletes, so nothing it argued for survives to be made inconsistent. The file now mixes registers: English for the steps that predate the phase, German for the two the phase adds."
  - "The scope-boundary comment avoids the literal string `0 offen`, because the plan's own acceptance criterion requires `grep -c '0 offen' ci.yml` to report 0 while `06-RESEARCH.md`'s suggested wording contains it. The boundary is stated in full — a missing key is created with an empty value and the diff goes clean while the translation is still absent — without the token the criterion forbids."
  - "The rebuild commit uses `chore(06):`, not `chore(06-06):`, exactly as the plan's action text writes it. It records a one-time state change of the tree rather than a step of this plan's work, and the two CI commits carry the `(06-06)` scope."
  - "Task 1's proof that the e2e tests actually ran was strengthened beyond the acceptance criterion: `go test -v` was re-run and `--- SKIP` counted, because the five plugin tests self-skip when the module is missing and a green run of skipped tests is precisely the false pass MAINT-03 exists to remove. Zero skips."

patterns-established:
  - "A rebuild's before-state is captured from `-check` BEFORE the build runs, because after the build the committed side is gone and the record cannot be reconstructed"
  - "A blocking comparison step is placed where a failure still means something: ahead of the tests, not beside them"

requirements-completed: [MAINT-01, MAINT-03]

coverage:
  - id: D1
    description: "All ten committed build artifacts rebuilt once with the pinned compiler: six modules on go1.26.6, no VCS stamp, current module path, and four archives repacked to match"
    requirement: MAINT-03
    verification:
      - kind: integration
        ref: "go run ./tools/wasm -check => all ten 'aktuell', exit 0 (was: ten 'ist nicht aktuell' blocks, exit 1)"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -print-hashes => six modules all 'gebaut mit go1.26.6'; matches 06-02's cross-host set row for row (7431486c…, 629574a3…, 1bde1f71…, cb872953…, b5533c1b…, 82c0da9e…)"
        status: pass
      - kind: other
        ref: "per module: strings -a | grep -oE '^go1\\.[0-9]+(\\.[0-9]+)?$' | sort -u => go1.26.6 only, six of six; grep -c 'github.com/holzcloud/cms' => 0 six of six; grep -c 'vcs.revision' / 'vcs.modified' / 'vcs.time' => 0 eighteen of eighteen"
        status: pass
      - kind: other
        ref: "unzip -Z1 per archive => 'plugin.json plugin.wasm' (two flat entries); cmp extracted plugin.wasm against the .wasm beside it => identical, four of four"
        status: pass
    human_judgment: false
  - id: D2
    description: "The rebuild is one commit whose file list contains nothing but the ten artifacts"
    requirement: MAINT-03
    verification:
      - kind: other
        ref: "git show --stat --name-only --format= 0aa3c8c | wc -l => 10; | grep -vc -E '\\.(wasm|zip)$' => 0"
        status: pass
      - kind: other
        ref: "git status --porcelain before staging => exactly the ten artifact paths plus untouched .planning bookkeeping; the ten were staged path by path, never `git add .`"
        status: pass
    human_judgment: false
  - id: D3
    description: "The recompiled guests behave identically through the wazero host — the compiler change is proven, not assumed"
    requirement: MAINT-03
    verification:
      - kind: e2e
        ref: "go test ./internal/plugin/ ./internal/public/ => ok 22.980s / ok 12.359s, exit 0"
        status: pass
      - kind: e2e
        ref: "go test -v ./internal/plugin/ ./internal/public/ | grep -c -- '--- SKIP' => 0; the ten wasm-loading tests (TestEreignisErreichtDasPlugin, TestZweiPluginsFilternInStabilerReihenfolge, TestProtokollTraegtDenNamenDesPlugins, TestEigenerSpeicherIstProPluginUndWebsiteGetrennt, TestBeispielPluginLaeuftDurch, TestKontaktformularPluginNimmtNachrichtenAn, TestPluginsNeverSeeADraft, TestSuchePluginBeantwortetSuche, TestOhneSuchePluginKeineSuche, TestLogMissWithoutPluginsIsSafe) all PASS"
        status: pass
      - kind: integration
        ref: "go test ./... => exit 0, no failures; gofmt -l . empty; go vet ./... exit 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "`go run ./tools/wasm -check` is a blocking CI step on the critical path, and the temporary falsification step is gone"
    requirement: MAINT-03
    verification:
      - kind: other
        ref: "ci.yml: 'name: Vet' L53 < 'run: go run ./tools/wasm -check' L62 < '- name: Build' L81; grep -c 'Cross-host hash falsification' => 0; grep -c -- '-print-hashes' => 0"
        status: pass
      - kind: other
        ref: "yaml.safe_load(ci.yml): a test-job step with run == 'go run ./tools/wasm -check' exists, no step mentions -print-hashes => ok; grep -c 'uses:' => 3 (unchanged); grep -c 'cache-dependency-path' => 0"
        status: pass
      - kind: integration
        ref: "go run ./tools/wasm -check => exit 0 on the very commit that wires the step in"
        status: pass
    human_judgment: false
  - id: D5
    description: "The catalogue check is a permanent CI step with its scope boundary written beside it"
    requirement: MAINT-01
    verification:
      - kind: integration
        ref: "go run ./tools/i18n -write && go run ./tools/i18n -schweiz && git diff --exit-code -- internal/i18n/locales/ => exit 0, no diff"
        status: pass
      - kind: other
        ref: "yaml.safe_load(ci.yml): exactly one test-job step mentions tools/i18n; its run block contains -write, then -schweiz, then 'git diff --exit-code -- internal/i18n/locales/' in that order => ok"
        status: pass
      - kind: other
        ref: "ci.yml: 'go run ./tools/wasm -check' L62 < 'go run ./tools/i18n -write' L74 < '- name: Build' L81; grep -c '0 offen' => 0; grep -c 'uses:' => 3 (unchanged)"
        status: pass
      - kind: other
        ref: "the German comment above the step states it proves canonical format and step-with-source but NOT that every string is translated, and names why a blocking count would be wrong"
        status: pass
    human_judgment: false

duration: 12 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 06: The Zero Point and the Two Blocking CI Steps Summary

**All ten committed build artifacts rebuilt once with `GOTOOLCHAIN=go1.26.6` in a commit containing nothing else, and `go run ./tools/wasm -check` now runs in CI after `Vet` and before `Build` — so from this commit on, no test can go green against a module built before the code it validates.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-09-04T11:25:00Z
- **Completed:** 2026-09-04T11:37:00Z
- **Tasks:** 3
- **Files modified:** 11 (ten artifacts + `.github/workflows/ci.yml`)

## The ten-row before/after record

Before-state read from `go run ./tools/wasm -check` against the committed tree
**before** the build ran — after it, the committed side no longer exists and the
record cannot be reconstructed. Sizes in bytes.

| Artifact | sha256 before | Bytes before | Go before | sha256 after | Bytes after | Go after |
|---|---|---|---|---|---|---|
| `plugins/bestellung/plugin.wasm` | `5e92a872bbf8b829…4ce641` | 4 230 537 | **go1.24.7** | `7431486cf1c60ab5…db6411` | 4 573 328 | go1.26.6 |
| `plugins/bestellung/bestellung.zip` | `aa71fa04ebd3b591…b6d29d` | 1 186 526 | — | `5fa31b9115ca3750…cf0551` | 1 288 040 | — |
| `plugins/jahreszahl/plugin.wasm` | `b3c00c92bd930a8e…5abccd` | 3 097 425 | **go1.24.7** | `629574a3b612c073…9fd7b82` | 3 313 021 | go1.26.6 |
| `plugins/jahreszahl/jahreszahl.zip` | `16a7376ef13c9433…fb047c` | 884 930 | — | `05dfbfe354792532…419b466` | 952 064 | — |
| `plugins/kontaktformular/plugin.wasm` | `56a67bd858ce1423…f74f4a` | 4 702 759 | **go1.26.4** | `1bde1f7106739059…768a28d` | 4 703 040 | go1.26.6 |
| `plugins/nicht-gefunden/plugin.wasm` | `958aaa8d7fe667f1…4ed608` | 3 367 421 | **go1.24.7** | `cb872953acb13ca2…39bdd1` | 3 587 610 | go1.26.6 |
| `plugins/nicht-gefunden/nicht-gefunden.zip` | `962247a9bc38f262…089839c` | 959 575 | — | `9b29c655a49a375f…6b7c07f` | 1 028 702 | — |
| `plugins/suche/plugin.wasm` | `1c3c4d2b8f0f46b9…570b0e` | 3 093 975 | **go1.24.7** | `b5533c1b30289542…7b6d821` | 3 419 051 | go1.26.6 |
| `plugins/suche/suche.zip` | `8dbc8cb9f913fc19…8ae4ddd5` | 883 268 | — | `fcdc8e3768e2704a…f8c64039` | 976 749 | — |
| `internal/plugin/testdata/echo.wasm` | `b13b065c248d72d8…5616d1` | 3 251 976 | **go1.26.2** | `82c0da9e5b9dbca8…a6df0c` | 3 253 624 | go1.26.6 |

The sha256 columns are truncated to 16 + 6 characters for reading; every
comparison below was made on the full 64-character values.

| | Before | After | Δ |
|---|---|---|---|
| Six modules | 21 744 093 | 22 849 674 | **+1 105 581** |
| Four archives | 3 914 299 | 4 245 555 | +331 256 |
| **Total** | **25 658 392** (24.5 MiB) | **27 095 229** (25.8 MiB) | **+1 436 837** (+1.37 MiB) |

`06-RESEARCH.md` predicted "+1.1 MB" for the modules from a `go1.26.4` reference
build. The realised figure with the pinned `go1.26.6` is **+1 105 581 bytes** —
the estimate holds. The per-module sizes differ from the research's fresh column
by a few hundred bytes each (`jahreszahl` 3 313 021 against the measured
3 312 731), which is the go1.26.4→go1.26.6 difference the pin exists to remove
from the equation, not a surprise.

**All six fresh module hashes are identical to `06-02-SUMMARY.md`'s cross-host
set, row for row** — the same six values measured on darwin/arm64 *and* on
`ubuntu-latest` in run `33866318077`. Nothing changed underneath between the
falsification and the rebuild, which is exactly the precondition that authorised
this commit.

## Accomplishments

- **The zero point is set, and it is one commit.** `0aa3c8c` lists exactly ten
  paths, all `.wasm` or `.zip`, no source, no documentation, no bookkeeping. The
  files were staged path by path — never `git add .` — with `.planning/config.json`
  and `.planning/milestone.lock` deliberately left out of the index.
- **Three compilers became one.** The committed set carried `go1.24.7` (four
  files), `go1.26.2` (`echo`) and `go1.26.4` (`kontaktformular`). All six now
  report `go1.26.6`, the value of `goToolchain` in `tools/wasm/main.go`.
- **The pre-rename module path is gone.** Five of six modules still stamped
  `github.com/holzcloud/cms`, a path that has not existed since the rename.
  `grep -c` over each rebuilt module returns 0, six of six.
- **No VCS stamp anywhere.** `vcs.revision`, `vcs.modified` and `vcs.time` all
  return 0 hits across all six modules — eighteen checks, eighteen zeros. This is
  the property without which the byte comparison is mathematically unsatisfiable
  (`06-RESEARCH.md` Pitfall 1), and it is now measured on the committed files
  rather than on a build.
- **The four archives carry the modules beside them, byte for byte.** Each
  archive holds exactly two flat entries, `plugin.json` then `plugin.wasm`, and
  `cmp` against the file next to it is identical, four of four. MAINT-03's own
  defect one layer up is closed at the same moment the layer below it is.
- **The recompiled guests behave the same.** `go test ./internal/plugin/
  ./internal/public/` is green, and — beyond the acceptance criterion — a `-v`
  run counted **zero** `--- SKIP` lines, so the ten wasm-loading tests really
  executed the new bytes through wazero rather than quietly stepping aside.
- **The comparison is blocking and it is on the critical path.** `ci.yml` L62,
  after `Vet` (L53) and before `Build` (L81) and `Test`. An SDK change can no
  longer be validated against a module built before it.
- **The temporary step is gone.** `Cross-host hash falsification (temporary —
  remove in plan 06-06)` did its job, its verdict lives in `06-02-SUMMARY.md`,
  and keeping it would have built the six guests twice per job for nothing.
- **The catalogues get the treatment the module graph already had.** `ci.yml`
  L74–L76 runs `-write`, then `-schweiz`, then
  `git diff --exit-code -- internal/i18n/locales/` — the exact shape of `Verify
  go.mod is tidy` two steps above. `06-03` measured that this pair, and not the
  round-trip test, is what catches a deleted key; the pair is now permanent.
- **No new GitHub Action.** `grep -c 'uses:'` is 3 before and after; no
  `cache-dependency-path` was added and `cache: true` on `setup-go` is untouched.

## The `test` job as it now reads

```
checkout → Set up Go → Verify formatting → Verify go.mod is tidy → Vet
  → Verify the committed plugin modules are current      ← new (D-08)
  → Verify the message catalogues are in step            ← new (D-15)
  → Build → Test → Upload the binary
```

Measured cost of the wasm step: about 4.6 s from a completely empty build cache
on a developer machine (`06-RESEARCH.md` §CI Cost), ~15–20 s cold on a hosted
runner and 2–4 s warm. The 297 s figure in `security.yml` belongs to the
race-detector run and is untouched by this plan.

## Task Commits

| Task | Commit | Type | What |
|---|---|---|---|
| 1 | `0aa3c8c` | chore | the ten rebuilt artifacts, and nothing else |
| 2 | `439ebd0` | ci | `-check` blocking after `Vet`; the temporary hash step deleted |
| 3 | `412f354` | ci | the catalogue step with its scope boundary in the comment |

## Files Created/Modified

- `plugins/{bestellung,jahreszahl,kontaktformular,nicht-gefunden,suche}/plugin.wasm` — rebuilt, go1.26.6, no VCS stamp
- `internal/plugin/testdata/echo.wasm` — rebuilt, go1.26.6, no VCS stamp
- `plugins/{bestellung,jahreszahl,nicht-gefunden,suche}/*.zip` — repacked deterministically around the new modules
- `.github/workflows/ci.yml` — one step removed, two added

## Decisions Made

1. **The two new comments are German.** `06-02` deviated to English on the
   grounds that all eleven comment blocks in the four workflow files are English.
   This plan's acceptance criteria test for a German comment explicitly, and
   `06-RESEARCH.md` supplies the German text. The plan is the later contract and
   the criterion is testable, so German it is. The one English comment 06-02
   wrote belonged to the step this plan deletes, so its argument does not survive
   to be contradicted. The consequence is real and worth naming: `ci.yml` now
   mixes registers — English for the steps that predate Phase 6, German for the
   two it adds. If that reads wrong to a human, the fix is two lines and it
   should be made deliberately, in one commit, for the whole file.
2. **The scope-boundary comment does not contain the string `0 offen`.** The
   plan's own acceptance criterion requires `grep -c '0 offen' ci.yml` to report
   0 (its purpose: no blocking translation-count assertion was smuggled in),
   while `06-RESEARCH.md`'s suggested comment text contains that exact string.
   Satisfying the criterion literally and the boundary substantively at the same
   time meant rewording: the comment says the step does not prove that every
   string is translated, explains that `-write` creates a missing key with an
   empty value so the diff goes clean while the translation is still absent, and
   says why a blocking count would be wrong (a translation legitimately in
   progress would turn the build red).
3. **`chore(06):`, not `chore(06-06):`, for the rebuild.** The plan writes it
   that way and the distinction is meaningful: the commit records a one-time
   state change of the tree, not a step of this plan's reasoning. The two CI
   commits carry the `(06-06)` scope.
4. **Skips were counted, not assumed.** The acceptance criterion asked only that
   `go test ./internal/plugin/ ./internal/public/` exit 0. Those five plugin
   tests self-skip when the module is missing (that is what `06-07` promotes), so
   a green exit alone would have been exactly the false pass this requirement
   exists to remove. `go test -v | grep -c -- '--- SKIP'` returns 0.

## Deviations from Plan

None — plan executed as written. The two wording judgements above (German
comments, avoiding the literal `0 offen`) are decisions taken *inside* the plan's
acceptance criteria, not departures from them; both criteria pass.

**Total deviations:** 0.
**Impact on plan:** none.

## Verification Results

| Check | Result |
|---|---|
| `go run ./tools/wasm -check` | **exit 0**, all ten `aktuell` (was: exit 1, ten mismatch blocks) |
| `go test ./...` | exit 0 |
| `go test ./internal/plugin/ ./internal/public/` | ok 22.980s / ok 12.359s |
| `go test -v … \| grep -c -- '--- SKIP'` | **0** |
| `go run ./tools/i18n -write` → `-schweiz` → `git diff --exit-code` | exit 0, no diff |
| `git show --stat --name-only --format= 0aa3c8c \| wc -l` | **10** |
| `… \| grep -vc -E '\.(wasm\|zip)$'` | **0** |
| embedded Go version, six modules | `go1.26.6` × 6, equal to the pin |
| `github.com/holzcloud/cms` hits, six modules | 0 × 6 |
| `vcs.revision` / `vcs.modified` / `vcs.time` hits | 0 × 18 |
| archive module vs. module beside it | identical × 4 |
| `ci.yml` step order | Vet 53 < wasm-check 62 < i18n 74 < Build 81 |
| `ci.yml` YAML parse + step assertions | `ok` (both python/yaml probes) |
| `grep -c 'uses:'` | 3, unchanged |
| `grep -c 'cache-dependency-path'` / `grep -c '0 offen'` | 0 / 0 |
| `grep -c 'Cross-host hash falsification'` / `-print-hashes` | 0 / 0 |
| `gofmt -l .` / `go vet ./...` | empty / exit 0 |

## Requirements

- **MAINT-03 → Complete.** Declared by `06-01`, `06-02`, `06-05` and `06-06`; all
  four now have a SUMMARY. CI rebuilds every committed guest module and every
  archive and compares them against the tree before a single test runs.
- **MAINT-01 → Complete.** Declared by `06-03` and `06-06`; both now have a
  SUMMARY. `06-03` locked the writer's format with a round-trip test over all
  seven catalogues; this plan makes the `-write` + empty-diff pair a permanent CI
  step, which is the half `06-03` measured to be the real guard against a deleted
  key.

## Issues Encountered

None. No auth gates, no package installs, no architectural decisions, no fix
attempts.

## Known Stubs

None.

## Deferred / Out of Scope

- **Promoting the five self-skipping tests and setting
  `HOLZCLOUD_TEST_REQUIRE_WASM=1`** — plan `06-07`, deliberately not done here
  (D-12: rebuild-and-compare first, promotion second). Note the two are now
  visibly coupled: this plan measured zero skips locally *because* the modules
  are present, which is exactly the condition `06-07` will make mandatory on a
  runner.
- **The register split in `ci.yml`** (English steps, two German comments) — see
  decision 1. Left as the plan specified; a whole-file decision, if wanted, is
  `06-07`'s documentation pass or a later quick task.
- **Archive bytes depend on `compress/flate`**, i.e. on the toolchain running
  `tools/wasm`, not on the pinned guest compiler (`06-05`). The comparison is
  blocking from this commit on, so a future Go release that changed flate output
  would turn the four archive rows red. The remedy is the one the compiler pin
  already carries: repack in the commit that raises the version. Unchanged risk,
  now with teeth — worth a line in `06-07`'s documentation.
- **`go build ./tools/wasm` still drops an un-ignored native `./wasm` binary in
  the repository root**, as `./i18n` and `./mkbundle` do. Pre-existing,
  unrelated, carried forward from `06-02` and `06-05`.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

`06-07` is unblocked and inherits a tree in the state it assumes: ten artifacts
that are provably current, a green `-check` locally and in CI, and a CI job that
fails before the tests if any of that stops being true. Three things to carry
into it:

1. **The CI failure signal is "non-zero", not "2".** `go run` collapses the
   child's exit code to 1 (`06-05`). The step wired in here relies on the shell's
   default non-zero handling and tests for no specific code.
2. **`HOLZCLOUD_TEST_REQUIRE_WASM` is not set anywhere yet** — not in `ci.yml`,
   not in `security.yml`, not in `release.yml`. D-10 puts it in all three as a
   workflow-level `env:` beside `CGO_ENABLED: "0"`; that is `06-07`'s job.
3. **Documents this plan invalidated:** `06-02-SUMMARY.md` and
   `06-05-SUMMARY.md` describe a `ci.yml` that no longer exists (the temporary
   step) and a `-check` that is red by design (it is green now). Both are
   historical records and should stay as written; `06-07`'s documentation pass
   should make sure nothing *forward-looking* still points at them as current
   state.

## Self-Check: PASSED

All eleven modified files verified present on disk. Commits `0aa3c8c`,
`439ebd0` and `412f354` verified in `git log`. Plan-level verification re-run at
close-out: `go run ./tools/wasm -check` exit 0, `go test ./...` exit 0, the i18n
three-command chain exit 0 with no diff, the rebuild commit's file list exactly
ten `.wasm`/`.zip` paths, `ci.yml` parsing with both new steps in the required
order and no new action.

---
*Phase: 06-aufr-umen*
*Completed: 2026-09-04*
