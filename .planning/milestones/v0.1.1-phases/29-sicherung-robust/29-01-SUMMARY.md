---
phase: 29-sicherung-robust
plan: 01
subsystem: deploy
tags: [backup, rsync, exit-trap, manifest, hermetic-test, tracer]
requires:
  - phase: 25-consistent-compatible-backups
    provides: the measured problems (8 of 40 nights refused with rsync 24; a directory without MANIFEST that blocked the next night)
provides:
  - deploy/backup.sh with run_rsync (the only caller of rsync), cleanup_failed_night (EXIT trap), OWN_DIR, the MANIFEST.partial rename and the refusal to start on an existing directory
  - deploy/test/backup-robust.sh, the hermetic proof, ending BACKUP-ROBUST OK (94 cases)
  - the five message strings of contract C-2, read by plans 29-02 and 29-03
affects: [29-02, 29-03, 29-04, 29-05, 29-08]
tech-stack:
  added: []
  patterns: [EXIT trap that decides by the absence of MANIFEST and never calls exit, a status captured with the rc=0; cmd || rc=$? form, MANIFEST written under another name and renamed, hermetic test with fakes on PATH]
key-files:
  created:
    - deploy/test/backup-robust.sh
  modified:
    - deploy/backup.sh
    - deploy/test/README.md
key-decisions:
  - "The cleanup decides by the absence of MANIFEST, never by $?, and never calls exit, so the status of the failure that brought it there is the script's status (rsync 11 stays 11, SIGTERM stays 143)."
  - "The refusal to start (status 1) comes after the free-space guard and before mkdir; mkdir is plain so that a race cannot reuse a directory; OWN_DIR is set only after that mkdir succeeded."
  - "run_rsync repeats status 24 exactly once, with no loop and no counter; any other non-zero status is passed on unrepeated."
requirements-completed: [ROB-01, ROB-02]
status: complete
commits: 3
plan_head_before: 4b8db071ec4c90c097ae530846bec521de75f84e
plan_head_after: c27c3e2eff12ae8235e13e65344f74a8235ff035
duration: 12min
completed: 2026-10-27
actuals:
  tokens: 7550
  tasks: 3
  commits: 3
---

# Phase 29 Plan 01: A backup that cleans up after itself Summary

**deploy/backup.sh repeats an rsync that ends with status 24 once, ends the night loudly with 24 if that recurs, and removes the directory of a night that failed before its MANIFEST; a hermetic test with fakes proves both in 94 assertions under bash 3.2.57 and bash 5.3.20, with no Docker and no root.**

## Case count and bash versions

`BACKUP-ROBUST OK (94 cases)` under both `/bin/bash` (3.2.57, macOS) and the bash on PATH (5.3.20). No behaviour differed. In Test 14 (SIGTERM during a copy, sent to the script and to the fake rsync) both end with status 143, the directory gone and the earlier night intact. One difference in output only: bash 3.2 prints a job-termination notice for the killed background job on its own stderr; the signalling and wait phase of the test is wrapped so that the notice goes nowhere.

## What was done

- **Task 1 (tracer), 1cbf7dd.** `OWN_DIR`, `cleanup_failed_night` on EXIT, the refusal to start when the directory exists, plain `mkdir`, the MANIFEST written as `MANIFEST.partial` and renamed. The test harness with the fake `rsync`, `holzcloud` and `date`; Tests 1 (success path unchanged, stdout line for line) and 2 (rsync 11 stays 11, directory removed, earlier night byte-identical).
- **Task 2, f8881ec.** `run_rsync WHAT RSYNC_ARGS...` is the only place that starts rsync; the loop of step 3 and the off-site copy go through it. Tests 3 to 9: 24 once then success, 24 twice (exactly two calls for the failing directory, none for the next), 23 and 11 not repeated, 24 in the second directory of the loop, and the three off-site cases (the directory keeps its MANIFEST and no cleanup line is printed).
- **Task 3, c27c3e2.** Tests 10 to 16: guard refusal, a taken name, a failed first step, a failed rename (the directory goes although `MANIFEST.partial` was in it), SIGTERM, the script text `nightly.sh` depends on, and a failing `find` that cannot fail the night. The header comment of `backup.sh` and the `## Phase 29: a backup that cleans up after itself` section of `deploy/test/README.md`, with the SIGKILL, out-of-memory and power-loss limit named.

## Verification

- `bash deploy/test/backup-robust.sh` and `/bin/bash deploy/test/backup-robust.sh`: `BACKUP-ROBUST OK (94 cases)`.
- `bash -n` on both scripts exits 0; the `MIN_FREE_MB` line reads 1024 through the harness's sed expression.
- `grep -cE '^[[:space:]]*run_rsync ' deploy/backup.sh` prints 2; `trap cleanup_failed_night EXIT` appears once; `MANIFEST.partial` twice.
- `git diff --stat HEAD` for `restore.sh`, `DEPLOY.md`, both units, `run.sh`, `nightly.sh`, `native-amd64.sh` and `.github` prints nothing.
- Expected red until plan 29-08: `28-check.sh beta evidence` and `frozen`, because `deploy/backup.sh` is a bound file of the earlier evidence. The checker was not touched.

## Deviations from Plan

None in the code or the tests. One process note: the first commit was blocked because `main` is a protected branch under the executor's commit guard (`git.allow_default_branch_commits` was not set). The work stopped, was reported, the owner set `git.allow_default_branch_commits: true` in `.planning/config.json`, and the plan then continued. That config change is the owner's and is not part of any task commit of this plan.

## Known Stubs

None.

## Threat Flags

None. The change adds no network endpoint, auth path or file access outside the backup root; the cleanup deletes only a path strictly below the backup root that this run itself made (T-29-01), and the named limit of T-29-05 (SIGKILL, out-of-memory kill, power loss) is recorded in the script comment and the README section.

## Self-Check: PASSED

- FOUND: deploy/test/backup-robust.sh
- FOUND: deploy/backup.sh (run_rsync, cleanup_failed_night, OWN_DIR, MANIFEST.partial)
- FOUND: deploy/test/README.md (## Phase 29 section)
- FOUND commits: 1cbf7dd, f8881ec, c27c3e2 (all ancestors of HEAD)
