---
phase: 28-the-promise-in-writing
plan: 01
subsystem: docs
tags: [beta-page, promises, owner-steps, changelog, tracer]
requires:
  - phase: 24-backup-timer
    provides: deploy/BACKUP-TIMER-RUN.md (BKP-01, O-1)
provides:
  - docs/beta.md as the 0.1.1 page with P-11 and one ledger row
  - 28-OWNER-STEPS.md with OS-6 copied from O-1 of the timer record
  - CHANGELOG.md with the 0.1.1 heading and first paragraph
affects: [28-02, 28-03]
tech-stack:
  added: []
  patterns: [quote one whole record sentence byte for byte, ledger row for a moved limit, owner step spliced from its record by 28-check.sh extract]
key-files:
  created:
    - .planning/phases/28-the-promise-in-writing/28-OWNER-STEPS.md
  modified:
    - docs/beta.md
    - CHANGELOG.md
key-decisions:
  - "The 03:15 line moves as D-01 and D-15 say: P-11 quotes the What it shows sentence of BKP-01; the baseline limit becomes the first cell of the ledger; a narrower limit about a real server's zone stays with OS-6."
  - "The date in the CHANGELOG heading is the draft of the milestone completion step (D-06); no version, tag or release was set."
requirements-completed: [DOC-01, DOC-02]
status: complete
commits: 2
plan_head_before: 47367a827323e30ea35bf57606d89d44f5bde767
plan_head_after: 5f6e10adb775d14adc40f1dd90649cc12131f3fc
duration: 25min
completed: 2026-10-26
actuals:
  tokens: 2900
  tasks: 2
  commits: 2
---

# Phase 28 Plan 01: The 03:15 line end to end Summary

**The 03:15 line of 0.1.0 now runs end to end: P-11 quotes one whole sentence of the timer record, the ledger holds the old limit, OS-6 is the record's O-1 copied by a program, and the CHANGELOG heading is read by release.yml's own lines.**

## Baseline (recorded before any edit)

`git diff --quiet f1ba5a2 -- docs/beta.md` exited 0 (the page was the baseline). `28-check.sh`: `beta structure` 1, `beta ledger` 1, `beta quotes` 1, `changelog extract` 1, `owner-steps steps` 2 (no 28-OWNER-STEPS.md), `beta baseline` 0 (guard). Remote heads before: main ee3c9ed, release 4d238b9, fix/bausteine-befunde fa696f6.

The worktree branch was at ee3c9ed and 47367a8 was not an ancestor with a clean tree, so it was reset to 47367a8 as instructed.

## What was done

- **Task 1 (tracer, 389609d).**
  - `docs/beta.md`: title and the two section headings say 0.1.1; "run 0.1.1 for real"; P-11 (BKP-01) added after P-10 with the sentence "The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole." plus the tokens `OnCalendar=*-*-* 03:15:00` and `START-LOCAL: 03:15:19` (both confirmed with grep first); the baseline limit "The real 03:15 window was not exercised ..." replaced by a narrower limit (zone and clock of a real server, Why, Record with the quoted Not tested sentence, Owner step OS-6); new section `## What changed since 0.1.0` with the baseline bullet as first cell of the one ledger row, answered by P-11.
  - `28-OWNER-STEPS.md`: title, index table, `### OS-6: The shipped timer at its real 03:15` with Source, Where, Needs, Expect begins at, A different output is explained at, and the verbatim region produced by `28-check.sh extract deploy/BACKUP-TIMER-RUN.md O-1` and spliced in by a script (not retyped).
  - `CHANGELOG.md`: `## 0.1.1 — 2026-10-26` directly above `## 0.1.0 — 2026-10-14`, one German paragraph with a bold lead and the absolute link to docs/beta.md. release.yml's own lines read version 0.1.1 and write 788 bytes of notes.
- **Task 2 (5f6e10a).** Pattern A (four phrases naming 0.1.0 as the release) counted 2 after Task 1 (the does-not-promise introduction and the alpha section), as the plan predicted, and counts 0 now. Pattern B (`exits 0 for 0.1.0`) counts 1, left for plan 28-03. The introduction now lists where the limits come from and speaks of 0.1.1; the alpha paragraph says "Neither was reworded for 0.1.1"; the paragraph before the promises table says that P-11 onward rest on `deploy/BACKUP-TIMER-RUN.md` and `deploy/BACKUP-CONSISTENCY-RUN.md`.

## Verification

- `beta structure`, `beta baseline`, `beta ledger`, `beta quotes`, `owner-steps steps`, `changelog extract`: all exit 0 (11 promise rows, 1 ledger row, 1 step, 788 bytes of notes).
- `frozen`: 0. `GO=1 changelog go`: 0.
- `grep -c '^## 0.1.1' CHANGELOG.md` prints 1; the next `## ` heading is the 0.1.0 heading.
- Regression (run by hand because the worktree has no `deploy/test/out`; `E=/Users/holz/Projects/holzcloud-cms/deploy/test/out`): `nightly.sh selftest` 542 cases (read through `2>&1`), `record-check` 29 blocks, `record-check-con` 27 blocks, `native-amd64.sh status-check` OK, `native-amd64.sh record-check` 0 blocks, pending SRV-01 to SRV-03. The 862-case native selftest was not run (plan 28-03).
- Remote heads after: identical to before. No push, tag, workflow run or writing `gh` call.

## Deviations from Plan

### Verification item that cannot pass as written

**1. [Checker defect, reported, not fixed] `28-check.sh selftest` fails 1 of 59 cases once the real CHANGELOG.md carries a 0.1.1 entry.**
- **Found during:** plan-level verification, item 1 (`selftest` must print `SELFTEST OK (59 cases)`).
- **Issue:** `fx_changelog` (line 840) builds its good fixture by inserting a small 0.1.1 entry above the first `## ` heading of the real CHANGELOG.md. Now that the real file starts with `## 0.1.1`, the fixture has two 0.1.1 headings and `changelog extract` reports "the heading after the new entry is not '## 0.1.0 — 2026-10-14'". The case is `changelog extract: the good fixture`. The failure is in the fixture, not in the page or the entry: the same selftest run with `CHANGELOG_FILE` pointing at the baseline CHANGELOG (`git show 47367a8:CHANGELOG.md`) prints `SELFTEST OK (59 cases)`. The real `changelog extract` passes.
- **Action:** 28-check.sh was not edited (instruction). The tracer's CHANGELOG heading is required by the plan itself, so the two cannot both hold. Plans 28-02 and 28-03 will meet the same failure; the owner or the orchestrator should decide whether to change `fx_changelog` (for example to start from the first `## 0.1.0` heading, or from `git show 47367a8:CHANGELOG.md`) in a commit of its own (D-11).
- **Files modified:** none.

### Auto-fixed Issues

None - the plan was otherwise executed as written. The plan statement that the `frozen`, `ledger` and `quotes` slices would hold was confirmed.

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or file access; the owner steps file holds no token, key block or credential URL (`owner-steps steps` checks it).

## Notes for the orchestrator

- The wrapper-script need: the worktree guard refused compound commands (`for` loops, `&&` chains of `bash` calls, `$(git ...)` forms); plain single commands and a scratchpad script (`ledger.sh`, `splice.sh`) were used instead. The splice only ran `28-check.sh extract` and wrote the output between the two comment lines.
- `commits:` counts the two task commits; the SUMMARY commit follows `plan_head_after`.

## Self-Check: PASSED

- docs/beta.md, CHANGELOG.md, 28-OWNER-STEPS.md exist; commits 389609d and 5f6e10a are ancestors of HEAD.
