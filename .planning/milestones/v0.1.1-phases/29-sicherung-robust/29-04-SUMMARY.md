---
phase: 29-sicherung-robust
plan: 04
subsystem: deploy
tags: [backup, evidence, con-chain, vanish, capfull, amd64]
requires:
  - phase: 29-sicherung-robust
    provides: plans 29-01 to 29-03 (run_rsync, the robust capfull assertions, the four vanish scenarios)
provides:
  - one complete evidence set from fresh containers against evidence commit dd077e4
  - the numbers for the record of plan 29-05 (Numbers and Stages below)
affects: [29-05, 29-07, 29-08]
tech-stack:
  added: []
  patterns: [evidence pinned to one commit by NEW_REF read from a file, fresh container and directory names per set]
key-files:
  created: []
  modified: []
key-decisions:
  - "No second pass was needed; the first set (names t29) is the kept set."
  - "The four containers were removed at the end of this plan (coordinator instruction, and the dispatch criterion 'leave no containers behind'), not by plan 29-05; the evidence directories stay."
requirements-completed: []
status: complete
commits: 0
plan_head_before: fc10d65b3917a6e42e673e130ec6ce9d4296ca89
plan_head_after: fc10d65b3917a6e42e673e130ec6ce9d4296ca89
duration: 40min of run time on the host clock, plus waiting
completed: 2026-10-29
actuals:
  tokens: 0
  tasks: 3
  commits: 0
---

# Phase 29 Plan 04: The evidence set for the changed backup.sh Summary

**One `nightly.sh con-chain` from fresh containers, binary pinned to evidence commit dd077e4, ended with chain exit status 0, CON-01 PASS, CON-02 PASS, CON-03 PASS, LEAK-CHECK OK, `STEPS: 96 played, 0 failed` and the four VANISH verdicts PASS.**

## Kept set

- **Evidence commit:** `dd077e4` (`deploy/test/out/amd64-t29-evidence-commit.txt`, one line). `BINARY: holzcloud v0.1.0-123-gdd077e4 (dd077e4)` in `consist.txt`; the log's first line reads `con-chain: NEW_REF pinned to dd077e4`.
- **Names:** containers `holzcloud-t29-src`, `-tgt`, `-old`, `-new`; directories `deploy/test/out/amd64-t29-src`, `-tgt`, `-old`, `-new`; wrapper files `amd64-t29-pass1.started`, `.finished`, `.status`, `.log`.
- **Host clock:** started 2026-10-27T07:52:12Z, finished 2026-10-27T08:32:05Z, status file 0.
- **Knobs:** `PLATFORM=linux/amd64 TARGET=systemd-base KEEP=1 CONSIST_RUNS=40 CONSIST_MIN_RUNS=30 CONSIST_RESTORE_EVERY=1`; the others at their defaults (`BULK_MB=100`, `CHURN_WORKERS=4`, `CHURN_PAUSE=0.05`, `CAPFULL_FLOOR_MB=1024`, `CAPFULL_MARGIN_MB=40`, `CAPFULL_DATA_MB=1150`, `COMPAT_TAG=v0.1.0`).
- **Second pass:** not needed. No finding of class 1 or 4, no fix commit.
- **HEAD moved during the run:** the orchestrator added `fc10d65 docs(roadmap): add phase 30 ...` on top of dd077e4. `git diff --quiet dd077e4 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh` exits 0 after the run, so every bound file is the committed text of dd077e4.

## Numbers

Verdicts (`amd64-t29-src/summary.txt`): `CON-01 PASS`, `CON-02 PASS`, `CON-03 PASS`, `LEAK-CHECK OK`, `STEPS: 96 played, 0 failed`. `chain-statuses.txt`: `prep=0 consist=0 window=0 capfull=0 compat-source=0 compat=0`. No `BAD` line.

**Consistency (CON-01), 40 runs per stage, all restored (RESTORED=40):**

| Stage | AGREE | ORPHAN-ONLY | ROW-WITHOUT-FILE | refused / restore failed | Uploads / page saves / deletes in window | Outcome |
|-------|-------|-------------|------------------|--------------------------|------------------------------------------|---------|
| adds | 16 | 24 | 0 | 0 / 0 | 65 / 24 / 0 | ORPHAN-ONLY-NAMED |
| deletes | 10 | 14 | 16 | 0 / 0 | 91 / 29 / 27 | ROW-WITHOUT-FILE-NAMED |

Backup status histogram in both stages: `0=40`. EXPOSED: 36 (adds), 38 (deletes).

**The three new counters (repeated / completed after the repeat / ended with status 24) and directories left without a MANIFEST:** adds 0 / 0 / 0 and 0; deletes 8 / 8 / 0 and 0. `MANIFEST-LESS-DIRS-STAGE-adds: 0` and `...-deletes: 0` each present once.

**Window:** C0 (nothing written) AGREE, C1 (upload in the window) ORPHAN-ONLY, C2 (delete in the window) ROW-WITHOUT-FILE.

**Compatibility (CON-02):** TAG=v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712), LEVEL-BEFORE=60, LEVEL-AFTER=60, LOGIN-ADMIN and LOGIN-EDITOR `HTTP 303 /admin/`, PAGES-SOURCE=8, PAGES-RESTORED=8, MEDIA-SERVED=4 of 4, CHECK=17 of 17.

**Full disk (CON-03):** FLOOR-MB=1024, FREE-AT-GUARD-MB=1064, DATA-MB=1225, FS-SIZE-MB=1064, CONTROL-CLASS=COMPLETE, HAND-CLASS=FAILED-NO-DIR, AGAIN-GUARD=passed, AGAIN-CLASS=FAILED-NO-DIR, UNIT-CLASS=FAILED-NO-DIR, UNIT-RESULT=exit-code, UNIT-EXECMAINSTATUS=11, RESTORE-PARTIAL-EXIT=not-run (the run by hand left no directory), HAND-ROBUST=PASS, AGAIN-ROBUST=PASS, AGAIN-NEXT-NIGHT=PASS, UNIT-ROBUST=PASS.

**Vanish (status 24):** `VANISH: ONCE=PASS TWICE=PASS ONCE-UNIT=PASS TWICE-UNIT=PASS UNIT-TWICE-RESULT=exit-code SHIM-REMOVED=yes`. Behind it: ONCE COMPLETE; TWICE FAILED-NO-DIR; ONCE-UNIT COMPLETE (`Result=success`, `ExecMainStatus=0`); TWICE-UNIT FAILED-NO-DIR (`Result=exit-code`, `ExecMainStatus=24`). `DEVIATION[vanish-shim]` is in `deviations.txt` once.

**Evidence directories:** `amd64-t29-src` 415 files, 1792 KB; `-tgt` 26 files, 108 KB; `-old` 48 files, 232 KB; `-new` 29 files, 120 KB (total 2252 KB, 518 files). Scratch only, gitignored.

## Stages

Wall times from the `started` lines of the log (a stage ends when the next begins; compat ends at the `.finished` time). All on the host clock, 2026-10-27.

| Stage | Started (UTC) | Duration |
|-------|---------------|----------|
| source | 07:52:12 | 1 min 46 s |
| target | 07:53:58 | 27 s |
| prep | 07:54:25 | 7 s |
| consist | 07:54:32 | 32 min 59 s |
| window | 08:27:31 | 1 min 10 s |
| capfull | 08:28:41 | 59 s |
| compat-source | 08:29:40 | 54 s |
| compat | 08:30:34 | 1 min 31 s |
| **Total** | 07:52:12 to 08:32:05 | **39 min 53 s** |

## Findings

All class 3 (measured properties of the shipped scripts), none a failure of the set; the three `CLASSIFY` kinds in `summary.txt`:

- consist adds: 24 runs ORPHAN-ONLY; consist deletes: 14 runs ORPHAN-ONLY and 16 runs ROW-WITHOUT-FILE (class 3, the same properties the record of commit e86d490 named).
- capfull HAND, AGAIN and UNIT: `FAILED-NO-DIR` (class 3; this is the wanted outcome, the failed night leaves no directory).
- `UNIT-EXECMAINSTATUS=11` under the unit versus 24 for the vanish case is a measured property: the full-disk night ends with the status rsync gives for a write error.

No finding of class 1, 2 or 4.

## Post-run checks

- `bash deploy/test/native-amd64.sh status-check`: `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`.
- `bash deploy/test/nightly.sh record-check`: `RECORD-CHECK OK 29 blocks`.
- Before removal `docker ps -a` listed the four containers of the kept set; afterwards `docker ps -a | grep -c holzcloud-t29` is 0 (removed with the documented `down down-con` command; other, unrelated containers on the daemon were not touched).
- Pre-start: `bash -n` on `deploy/backup.sh`, `deploy/test/nightly.sh`, `deploy/test/backup-robust.sh` all 0; `BACKUP-ROBUST OK (94 cases)`; `SELFTEST OK (646 cases)`.

## Deviations from Plan

**1. [Judgment] `git status --porcelain` is not empty, only `.planning/config.json` and `.planning/milestone.lock` differ**
- **Found during:** Task 1 precondition, and Task 3.
- **Issue:** The precondition and Task 3 ask for an empty `git status --porcelain`. These two files belong to the orchestrator, are outside `deploy/`, are not shipped into the containers, and the orchestrator instructed that they not be committed.
- **Fix:** The check was limited to the bound files and to `deploy/` (the plan's own narrower form: `git status --porcelain -- <bound files>` empty, `git diff --quiet dd077e4 HEAD -- <bound files>` exit 0). `git status --porcelain` printed exactly these two lines before the start and after the run.

**2. [Judgment] Containers removed in this plan rather than in plan 29-05**
- The plan said to keep them until the record is proved. The coordinator instructed removal at the end of this plan, matching the dispatch criterion "leave no containers behind". The record needs the evidence directories, not the containers. Plan 29-05's `down down-con` step is safe when they are already gone, and its `grep -c holzcloud-t29` check prints 0.

**3. [Judgment] Requirement ROB-03 not ticked**
- The plan lists ROB-03. The evidence is made here but the record that cites it, and the checks that prove it, are plan 29-05. Not marked complete.

**4. [Note] HEAD moved from dd077e4 to fc10d65 during the run** (a docs-only roadmap commit made by the orchestrator). No bound file changed; the evidence still names dd077e4.

No auto-fixes, no tracked file edited, no commit by this plan other than the documentation commit for this SUMMARY.

## Known Stubs

None.

## Threat Flags

None. The run adds no new surface. T-29-13: bound files identical to dd077e4 before and after; T-29-14: new names, nothing from an earlier set cited; T-29-15: containers removed, no leak (`LEAK-CHECK OK`).

## Self-Check: PASSED

- FOUND: deploy/test/out/amd64-t29-evidence-commit.txt (dd077e4), amd64-t29-pass1.{started,finished,status,log}, amd64-t29-src/summary.txt, chain-statuses.txt
- FOUND commit dd077e4 (ancestor of HEAD); `docker ps -a` has no holzcloud-t29 container
