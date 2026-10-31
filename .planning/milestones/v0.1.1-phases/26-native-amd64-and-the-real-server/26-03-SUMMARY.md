---
phase: 26-native-amd64-and-the-real-server
plan: 03
subsystem: deploy/test documentation, planning status
tags: [native-amd64, readme, requirements, status-check, owner-pending, srv-01, srv-02, srv-03, sweep]
status: complete
requires:
  - phase: 26-02
    provides: native-amd64.sh record-check, status-check, appendix; deploy/NATIVE-AMD64-RUN.md in the pending form
provides:
  - deploy/test/README.md section "Phase 26: native amd64 and the real server" and its file-list bullet (insertion only)
  - .planning/REQUIREMENTS.md with SRV-01 to SRV-03 at "Pending owner", boxes unchecked, a note under the table
  - .planning/ROADMAP.md Owner-pending paragraph under the Phase 26 plan list
affects: [28 (DOC-01, DOC-02 cite deploy/NATIVE-AMD64-RUN.md; a docs/beta.md line moves only when its Status is recorded)]
tech-stack:
  added: []
  patterns: [a status value no workflow tool flips, a checker that holds two files to each other, a measured sweep with the numbers taken now]
key-files:
  created: []
  modified:
    - deploy/test/README.md
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
key-decisions:
  - "The README section opens with the hand-over (what exists, what does not) before it explains the button, so a reader of the file cannot take the instrument for a result"
  - "The ROADMAP plan checkboxes and the progress line are left to the workflow tools; only the Owner-pending paragraph was edited by hand"
requirements-completed: []
duration: "about 4 minutes after the start time was recorded (15:36:59Z to 15:40:40Z), plus reading the plan and context beforehand; the clock of the host was not synchronised with the earlier summaries"
completed: 2026-10-26
commits: 2
plan_head_before: 3c9a37fc123b25fe1241c9654c26e3ee82cdda39
plan_head_after: 7d8e43d65d0f1d4357de2522f4e9c88810e34adb
estimate:
  tokens: 45000
  tasks: 2
actuals:
  tokens: 2800
  tasks: 2
  commits: 2
---

# Phase 26 Plan 03: Honest status and the sweep Summary

The hand-over is closed and says what is true: the README documents the instrument, REQUIREMENTS.md reads `Pending owner` for SRV-01 to SRV-03 with the boxes unchecked, ROADMAP.md carries an Owner-pending note, and `native-amd64.sh status-check` holds the planning file and the record together. **No requirement is verified.** `requirements-completed` is empty; no native amd64 run and no result from a real server exists.

## What was done

- **Task 1** (`cda4e17`): `deploy/test/README.md` gained one bullet in its Files list and the section `## Phase 26: native amd64 and the real server` immediately before `## Pitfalls`; `git diff --numstat 05fc076 -- deploy/test/README.md` reads `37 0`, zero deleted lines. The section covers: what exists and that no result does; pressing the button (Actions tab or `gh workflow run native-amd64.yml --ref main`, the one input, the job summary page and the artifact `native-amd64-evidence`, nothing fires by itself); why the container path on the runner and what it leaves out (FUT-05); the verdict words with their exit codes and the VOID rule; the files of a run and what the bundle never holds; the six subcommands and the variables; the arm64 rehearsal command and that it proves the guard only; how a result is recorded; the frozen files and the command that shows them. `.planning/ROADMAP.md` gained one `**Owner-pending**:` paragraph under the Phase 26 plan list (2 lines added, nothing else of the file changed).
- **Task 2** (`7d8e43d`): baseline first. `bash deploy/test/native-amd64.sh status-check` failed with three problems (`SRV-01`, `SRV-02`, `SRV-03` "traceability cell reads 'Pending', expected 'Pending owner'"), exit 1, so the gate is real. Then the three cells in `.planning/REQUIREMENTS.md` were changed to `Pending owner`, the note was added directly under the table, and the last-updated line was set to 2026-10-26. The three boxes were never touched and read `- [ ]`; the Coverage numbers were not touched.

## The sweep (measured now, 2026-10-26, on this tree at `7d8e43d`)

| Check | Result |
|-------|--------|
| `native-amd64.sh status-check` | `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record` |
| `native-amd64.sh record-check` | `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03` |
| `native-amd64.sh selftest` | `SELFTEST OK (862 cases)` |
| `nightly.sh selftest` (with `2>&1`; the count is on stderr) | `SELFTEST OK (542 cases)` |
| `nightly.sh record-check` (`E` = main checkout's `deploy/test/out`) | `RECORD-CHECK OK 29 blocks` |
| `nightly.sh record-check-con` (same `E`) | `RECORD-CHECK OK 27 blocks` |
| removed lines against `b501b1b` in `nightly.sh`, captured to a variable before counting | `3` (tripwire 6) |
| `git diff --quiet 05fc076 --` the 14 frozen files (`backup.sh restore.sh DEPLOY.md holzcloud.service holzcloud-backup.service holzcloud-backup.timer run.sh nightly.sh docs/beta.md SECURITY-SCORE.md BACKUP-RUN.md BACKUP-TIMER-RUN.md BACKUP-CONSISTENCY-RUN.md DEPLOY-RUN.md`) | identical |
| `git diff --quiet f1ba5a2 -- docs/beta.md` | identical |
| `.github/workflows/ci.yml` against `05fc076` | identical |
| workflow lints (`yq`; `actionlint` is not installed) | one trigger `workflow_dispatch`; `permissions` holds `contents: read` and nothing else; `runs-on: ubuntu-latest`; `timeout-minutes` set; the `uses:` set of `native-amd64.yml` equals that of `ci.yml` (three official actions at the same commit hashes); no `secrets.` in the file |
| `git diff --name-only 05fc076` | `.github/workflows/native-amd64.yml`, `deploy/NATIVE-AMD64-RUN.md`, `deploy/test/README.md`, `deploy/test/native-amd64.sh`, and files under `.planning/`; nothing else |
| README against `05fc076` | `37 0` (additions, deletions) |
| REQUIREMENTS.md | three rows `Pending owner`, three boxes `- [ ]` |

## Closing order

Run after this SUMMARY was written, as the plan orders. The workflow's requirement step was played: `requirements.mark-complete SRV-01 SRV-02 SRV-03` answered `updated: false`, `marked_complete: []`, `not_found` all three, every `write_set` entry (checkbox and traceability, per requirement) `applied: false`. REQUIREMENTS.md was byte-unchanged afterwards (three boxes `- [ ]`, three cells `Pending owner`), so `D-06` holds as designed: the value `Pending owner` is one the tool does not flip. Then `bash deploy/test/native-amd64.sh status-check` was run again and ended `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`, exit 0. **No revert was needed** (`requirements.revert-phase` was not run, no row was flipped, no box checked). `roadmap.update-plan-progress 26` set the phase row to `3/3`, `In Progress`, the three plan boxes to `[x]` and the plans line to `3/3`; it did not mark the phase complete, which is the orchestrator's `phase.complete`. **That step must be followed by `status-check` once more** before the phase is called done: `phase.complete` was not run here and its effect on the cells is only predicted from D-06, not observed.

## Deviations from Plan

### Plan deviations and observations (no rule applied)

- **The plan's Task 2 verify line cannot pass as written in a worktree, as 26-01 and 26-02 already said.** `nightly.sh selftest | tail -n 2 | head -n 1` reads stdout, where only `SELFTEST OK` stands (the count is on stderr), and `nightly.sh record-check` / `record-check-con` need `E=/Users/holz/Projects/holzcloud-cms/deploy/test/out` because this worktree has none of the phase 24 and 25 scratch files. The sweep above ran those with `2>&1` and with `E`, and the rest of the verify line as written (the `awk` count of rows and boxes, the `yq` check, the `git diff` guards, the stray-file filter) passed in the same wrapper script.
- **`state.advance-plan` refused** with `ambiguous_position_phase` (three `Phase:` entries in the Current Position section of STATE.md: "Not started (defining requirements)", "none open. Milestone v2.1 is closed", "12 — The Codebase Speaks English"). This is the known defect TIDY-02 (phase 27) and was not fixed here (out of scope). `state.update-progress` found no `Progress:` line and skipped; `state.record-metric`, `state.record-session` and `roadmap.update-plan-progress` worked. The body of STATE.md still says "Where we are (corrected 2026-10-12)"; only frontmatter, the metrics row and the session lines moved. `state.add-decision` was not called (the two key decisions above are about this plan's wording only).
- **A stray one-line scratch file was written outside the repository** by mistake (`/private/tmp/claude-501/-Users-holz/Projects-holzcloud-cms-placeholder`, content `x`); the attempt to remove it was denied by the permission classifier and was not worked around. It is harmless and outside every tracked path.
- **The base commit was not an ancestor of `HEAD`** (`ee3c9ed`, tree clean), so `git reset --hard 3c9a37f` was run first, as instructed.
- **The Bash tool's worktree guard refused every command text that was not plain**, so the verifies, the sweep and the commits ran through wrapper scripts in the scratchpad, as the brief allowed.

## Known limits (stated, not hidden)

- Nothing here ran on amd64 hardware, on GitHub or on a real server; this plan edits documents and measures that nothing moved.
- The README states the exit code of `nightly.sh`'s count line as it is (stderr), so a reader following it in a worktree does not repeat the plan's mistake; it cannot make the earlier plans' verify lines pass.
- `actionlint` was not available; the workflow lint is `yq` plus the `uses:` comparison, as in plan 01.

## Threat model coverage

T-26-11 (an automatic step marks SRV-01 to SRV-03 Complete): the cells read `Pending owner`; `status-check` fails when a row reads Complete while the record says pending; the closing order below ran it after the requirement step. T-26-12 (a frozen file or an earlier record edited by accident): swept against `05fc076` and `f1ba5a2`, the 542/29/27/3 numbers measured, the changed-file list checked against the owned set, the README edit counted with zero deletions. T-26-SC accepted: documents only, nothing installed. No new security surface.

## Known Stubs

None. The `(nothing pasted yet)` blocks and `Evidence commit: none (pending owner)` in `deploy/NATIVE-AMD64-RUN.md` are the pending-owner form of plan 02, not stubs of this plan.

## Self-Check: PASSED

- FOUND: `deploy/test/README.md` with the section before `## Pitfalls`, `.planning/REQUIREMENTS.md` with three `Pending owner` cells, `.planning/ROADMAP.md` with the `**Owner-pending**:` paragraph
- FOUND commits: `cda4e17`, `7d8e43d` (both ancestors of HEAD; two commits counted from `plan_head_before`)
- `status-check` and `record-check` end OK; frozen files unchanged since `05fc076`, `docs/beta.md` since `f1ba5a2`; nothing pushed, nothing dispatched
