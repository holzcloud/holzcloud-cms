---
phase: 06-aufr-umen
plan: 01
subsystem: infra
tags: [planning, requirements, roadmap, wasm, gotoolchain, buildvcs, i18n]

# Dependency graph
requires:
  - phase: 06-aufr-umen
    provides: 06-CONTEXT.md decisions D-01…D-23 (locked) and 06-RESEARCH.md measurements
provides:
  - "REQUIREMENTS.md MAINT-03 covering all six committed .wasm modules, not a glob that misses echo.wasm"
  - "REQUIREMENTS.md MAINT-05 covering all seven codebase maps, pointing at the RESEARCH correction inventory"
  - "ROADMAP.md Phase 6 success criteria 3 and 5 stating what the phase actually delivers"
  - "ROADMAP.md Phase 6 planning notes naming `go run ./tools/wasm` as the build shape"
  - "ROADMAP.md Phase 6 planning notes carrying the GOTOOLCHAIN pin with its go.mod floor"
  - "ROADMAP.md Phase 6 planning notes carrying mandatory -buildvcs=false and its four locations"
  - "ROADMAP.md Phase 6 CI-budget note carrying the measured 4.6 s instead of the unrelated 297 s"
affects: [06-02, 06-03, 06-04, 06-05, 06-06, 06-07, phase-07, phase-08, phase-09, phase-10]

actuals:
  tokens: 2000
  tasks: 1
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Ground truth is amended in its own docs-only commit before any code is touched (D-01), carried by the wave graph rather than by prose"

key-files:
  created:
    - .planning/phases/06-aufr-umen/06-01-SUMMARY.md
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "The i18n pointer `tools/i18n/main.go:287` was left unchanged — D-17a is WITHDRAWN and a fresh grep confirmed 287 is correct. What the note now says instead is that the claim lives in the `writeCatalog` doc comment, not the package doc comment at :1–15."
  - "The literal string `plugins/build.sh` was kept out of the amended shape note entirely rather than being named as the thing that never existed, because an acceptance criterion greps the Phase 6 section for it and expects zero."

patterns-established:
  - "A success criterion never carries a count that can go stale on its own; it points at a line-referenced checklist (06-RESEARCH.md §\"MAINT-05 Correction Inventory\") that a later task executes mechanically."

requirements-completed: []

coverage:
  - id: D1
    description: "MAINT-03 in REQUIREMENTS.md covers all six committed WebAssembly modules — the five under plugins/ plus internal/plugin/testdata/echo.wasm — and states the rule that no committed .wasm may go stale unchecked"
    requirement: "MAINT-03"
    verification:
      - kind: other
        ref: "grep -c 'plugins/\\*/plugin.wasm' .planning/REQUIREMENTS.md → 0; grep 'MAINT-03' .planning/REQUIREMENTS.md | grep -c 'echo.wasm' → 1"
        status: pass
    human_judgment: false
  - id: D2
    description: "MAINT-05 in REQUIREMENTS.md covers all seven codebase maps and points at 06-RESEARCH.md §\"MAINT-05 Correction Inventory\" instead of naming a count of drifted facts"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "grep '\\*\\*MAINT-05\\*\\*' .planning/REQUIREMENTS.md | grep -c '06-RESEARCH.md' → 1; same line grep -c 'seven' → 1"
        status: pass
    human_judgment: false
  - id: D3
    description: "ROADMAP Phase 6 success criterion 3 widened to all six committed modules and adds the four .zip archives repacked by the same tool (D-23)"
    requirement: "MAINT-03"
    verification:
      - kind: other
        ref: "sed -n '/### Phase 6/,/### Phase 7/p' .planning/ROADMAP.md | grep -c 'plugins/\\*/plugin.wasm' → 0; criterion 3 names echo.wasm and the four archives"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROADMAP Phase 6 success criterion 5 names the drifted facts across all seven maps rather than six in ARCHITECTURE.md"
    requirement: "MAINT-05"
    verification:
      - kind: other
        ref: "git diff HEAD~1 -- .planning/ROADMAP.md — criterion 5 hunk at line 105"
        status: pass
    human_judgment: false
  - id: D5
    description: "ROADMAP Phase 6 planning notes name `tools/wasm` as a Go command invoked with `go run ./tools/wasm`, carry the GOTOOLCHAIN pin with its go.mod floor, and carry mandatory -buildvcs=false with its four locations"
    verification:
      - kind: other
        ref: "Phase 6 section: grep -c 'plugins/build.sh' → 0; grep -c 'tools/wasm' → 5; grep -c 'GOTOOLCHAIN' → 1; grep -c -- '-buildvcs=false' → 2"
        status: pass
    human_judgment: false
  - id: D6
    description: "The Phase 6 pointer into tools/i18n/main.go equals what a grep reports, and the note now says the claim is the writeCatalog doc comment rather than the package doc comment"
    requirement: "MAINT-02"
    verification:
      - kind: other
        ref: "grep -n 'sorted and indented' tools/i18n/main.go → 287 (single match); Phase 6 pointer → tools/i18n/main.go:287"
        status: pass
    human_judgment: false
  - id: D7
    description: "The amendment is the phase's first commit, is docs-only, and touches nothing outside the two documents"
    verification:
      - kind: other
        ref: "git show --stat --name-only --format= HEAD → exactly .planning/REQUIREMENTS.md and .planning/ROADMAP.md; ls .planning/phases/06-aufr-umen/ | grep -c SUMMARY → 0 before the commit"
        status: pass
    human_judgment: false

duration: 6 min
completed: 2026-09-04
status: complete
---

# Phase 6 Plan 01: Ground Truth First Summary

**REQUIREMENTS.md and ROADMAP.md amended to the locked Phase 6 decisions in one docs-only commit — six committed `.wasm` modules instead of a glob that missed `echo.wasm`, seven codebase maps instead of a stale count of six drifted facts, `go run ./tools/wasm` instead of a shell script that never existed, plus the `GOTOOLCHAIN` pin, mandatory `-buildvcs=false` and the measured 4.6 s CI cost.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-09-04T10:28:00Z
- **Completed:** 2026-09-04T10:34:22Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- **MAINT-03 no longer excludes `echo.wasm` by construction.** The requirement read `plugins/*/plugin.wasm`, a glob that literally cannot reach `internal/plugin/testdata/echo.wasm`, while MAINT-04's "five tests" does reach it. That asymmetry is the defect D-01 exists to close. The line now names all six committed modules and states the rule the decision gives: no committed `.wasm` in this repository may go stale unchecked.
- **MAINT-05 and success criterion 5 stopped carrying a count that was itself stale.** Both said `ARCHITECTURE.md` carries six drifted facts; the research counted more than twenty in that file alone, plus a `k8s/` directory, a `deploy.yml` workflow and an arm64 cross-compile job still documented in `INTEGRATIONS.md` that no longer exist. Both clauses now cover all seven maps and point at `06-RESEARCH.md` §"MAINT-05 Correction Inventory", a line-referenced checklist a later task executes mechanically. A criterion that points at a checklist cannot go stale the way a number can.
- **The build shape note names a file that exists.** It read `plugins/build.sh` — a script that never existed in this repository. It now names `tools/wasm`, a Go command invoked as `go run ./tools/wasm` (and `-check` in CI), with the reason: this repository's tooling is Go commands under `tools/` and shell lives only under `deploy/` (D-06).
- **Two facts that make the byte comparison possible at all are now written down where a planner reads.** The `GOTOOLCHAIN` pin lives inside the build tool, not in CI's `setup-go` and not as a `toolchain` line in `go.mod`, and it has a floor at the root `go.mod`'s `go` directive because `echo` lives in the root module (D-03, D-03a). And `-buildvcs=false` is mandatory rather than an optimisation: every committed module carries `vcs.revision` from the default `-buildvcs=auto`, so a CI rebuild happens at a different commit by construction and can never match — no toolchain pin rescues that (D-02a). The note names the four places the flag must appear.
- **The CI-budget note carries a measurement instead of a fear.** It warned about the 297 s recorded in `security.yml`; that figure is `go test -race ./internal/plugin/`, the wazero host under the race detector, which this step does not touch. The note now carries the measured 4.6 s for six wasip1 guests from an empty `GOCACHE`.

## Task Commits

1. **Task 1: Amend REQUIREMENTS.md and ROADMAP.md to the locked decisions (D-01)** — `4681e9f` (docs)

_This plan produces one commit by design: D-01 requires the amendment to be a single docs-only commit, and it is the phase's first._

## Files Created/Modified

- `.planning/REQUIREMENTS.md` — MAINT-03 widened to six committed modules; MAINT-05 widened to seven codebase maps with a pointer to the research inventory. Traceability table rows untouched (their Status column is phase-completion's job, not this task's).
- `.planning/ROADMAP.md` — Phase 6 only: success criteria 3 and 5 amended, build-shape note re-pointed at `tools/wasm`, i18n pointer note clarified, two new planning notes added (`GOTOOLCHAIN` pin, mandatory `-buildvcs=false`), CI-budget note given the measured figure.

## Decisions Made

- **The i18n line pointer was left at `:287`, not "corrected".** D-17a proposed moving it to `:288` and has been WITHDRAWN — the correction was itself the error. The plan required a grep rather than trust, and the grep agrees with the existing pointer: `grep -n 'sorted and indented' tools/i18n/main.go` reports exactly one line, **287** (`func quote` spans 277–285, `func writeCatalog` begins at 289). No disagreement to record. What the note now says plainly is the half of the research finding that does stand: the claim MAINT-02 removes is the `writeCatalog` doc comment, not the package doc comment at `:1–15`, which makes no claim about output shape at all.
- **The amended shape note does not contain the literal string `plugins/build.sh`.** The plan's prose invited naming it as the thing that never existed, but an acceptance criterion greps the Phase 6 section for that exact string and expects zero. The note says "the script this note used to name never existed (D-06)" instead — same information, and the criterion stays checkable.

## Deviations from Plan

None - plan executed exactly as written.

The one judgement call above (avoiding the literal `plugins/build.sh` in the replacement text) resolves a tension inside the plan itself between its prose and its acceptance criteria; it changes no fact and adds no scope, so it is recorded as a decision rather than a deviation.

## Issues Encountered

- **`state.advance-plan` and `state.update-progress` could not run against this project's `STATE.md`.** They returned `Cannot parse Current Plan or Total Plans in Phase from STATE.md` and `Progress field not found in STATE.md` respectively: this `STATE.md` writes its position as a prose `### Current Position` block and keeps its counters in the YAML frontmatter, not in the `Current Plan` / `Total Plans` / `Progress` fields those two handlers look for. Pre-existing shape, not introduced here. Worked around by editing the `### Current Position` block and the Milestone Map row for Phase 6 directly; `state.record-metric`, `state.record-session`, `state.add-decision` and `roadmap.update-plan-progress` all ran normally, and the frontmatter's `completed_plans` advanced to 1 on its own.
- **No requirement was marked complete, by design.** `requirements.ready-ids` reported `0/2` — MAINT-03 is also declared by `06-02`, `06-05` and `06-06`, and MAINT-05 by `06-04` and `06-07`, none of which have run. This plan amended the text of both requirements; it did not deliver either. They flip to Complete when the last declaring plan finishes.

## Threat Flags

None. This plan edits Markdown only; it installs no package, adds no endpoint and crosses no trust boundary. T-06-00 (a criterion asserting a scope the phase does not deliver) is mitigated as planned — every amended clause derives from a locked decision in `06-CONTEXT.md`, and the i18n pointer was re-derived by grep rather than copied. T-06-SC is accepted and unchanged: no package was installed.

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Wave 1 is complete and the ordering D-01 demands is now a fact in git: the phase's first commit is docs-only and touches nothing but the two source documents. Wave 2 (`06-02` ∥ `06-03` ∥ `06-04`) is unblocked.

Two things the next executors should carry:

- **`06-02`'s tracer is the phase's first code task.** Nothing before this commit touched code, and nothing in this commit did either.
- **The i18n pointer is settled.** `06-03` deletes the claim at `tools/i18n/main.go:287`; it should not spend time re-deriving whether 287 or 288 is right. It is 287, verified twice — once by the planner, once here.

## Self-Check: PASSED

- `.planning/phases/06-aufr-umen/06-01-SUMMARY.md` — FOUND on disk
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` — FOUND, both modified
- Commit `4681e9f` (task 1, docs-only amendment) — FOUND in `git log`
- Commit `fd8664e` (plan metadata) — FOUND in `git log`
- Plan-level verification re-run: `git show --stat --name-only --format= HEAD` at `4681e9f` listed exactly the two documents; every negative grep reported 0 and every positive grep reported at least 1; `git diff` touched no line inside Phases 7–10.

---
*Phase: 06-aufr-umen*
*Completed: 2026-09-04*
