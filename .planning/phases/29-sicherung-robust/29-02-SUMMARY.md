---
phase: 29-sicherung-robust
plan: 02
subsystem: deploy
tags: [backup, harness, capfull, consist, assertions, selftest, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: plan 29-01, deploy/backup.sh with run_rsync, the cleanup trap and the five contract strings
provides:
  - capfull_robust_verdict and capfull_next_night_verdict, pure verdicts with selftest cases
  - capfull keys REMOVAL-LINE, HAND-ROBUST, AGAIN-ROBUST, AGAIN-NEXT-NIGHT, UNIT-ROBUST
  - RUN-line tokens RETRIES= and LEFT=, consist_retry_counts and four per-stage key lines
  - the window stage's reading of the changed backup.sh
  - RC_HANDOVER, the hand-over heading of record_check as a knob
affects: [29-03, 29-04, 29-05, 29-08]
tech-stack:
  added: []
  patterns: [pure verdict function that refuses unknown words with status 1 and prints nothing, counters read from RUN lines by token name, a knob whose default reproduces the old behaviour line for line]
key-files:
  created: []
  modified:
    - deploy/test/nightly.sh
    - deploy/test/README.md
key-decisions:
  - "A failed night that leaves a directory is a BAD line in capfull (it was check_info before); the verdict functions return FAIL with the first broken property and are asserted one case per way to break."
  - "The new RUN-line tokens sit between RSYNC-STATUS= and DIR= so DIR= stays last and every by-name reader is unaffected."
  - "RC_HANDOVER replaces only the one required heading inside the existing loop; unset or empty it is the heading asked for before."
requirements-completed: [ROB-02]
status: complete
commits: 3
plan_head_before: aa70280952b18099861f46fef7b1dc745fec8894
plan_head_after: d4f809993a64b3ee8b30acbdb03cd2888f47438b
duration: 40min
completed: 2026-10-27
actuals:
  tokens: 7100
  tasks: 3
  commits: 3
---

# Phase 29 Plan 02: The harness asserts what the script promises Summary

**nightly.sh now fails a set in which a failed night leaves a directory, the next night is refused by the guard, or the sentinel of an earlier night is touched; consist counts repeats after rsync status 24, nights that still ended 24 and directories without a MANIFEST; the selftest ends SELFTEST OK (594 cases).**

## Selftest case count

`bash deploy/test/nightly.sh selftest` ends `SELFTEST OK (594 cases)`. Baseline at HEAD was 542. After Task 1: 574 (32 cases for the two capfull verdicts). After Task 2: 582 (8 for `consist_retry_counts` and the new RUN-line format). After Task 3: 594 (9 for `RC_HANDOVER`, 3 for the retries lines of `con_summarize`). Plan 29-08 pins 594.

## What was done

- **Task 1 (tracer), b740726.** `capfull_robust_verdict CLASS DIR_MADE REMOVAL_LINE SENTINEL_INTACT` and `capfull_next_night_verdict GUARD CLASS` beside `capfull_classify`, each refusing an unknown word with status 1 and nothing printed. `cf_judge` writes `LABEL-REMOVAL-LINE` (a `grep -c` number, so an unreadable file means `no`) and keeps the sentinel value it printed for that stage. `capfull_run` calls `capfull_report_robust` after HAND, AGAIN and UNIT and `capfull_next_night_verdict` after AGAIN; `FAILED-PARTIAL-DIR-LEFT` on the run by hand is now BAD.
- **Task 2, 234b6d2.** `consist_run` sets `R_RETRIES` (lines with `WARNING: rsync exited 24`) and `R_LEFT` (directories below the backup root without a MANIFEST, counted in the source container with one `find`); a count that cannot be read is a BAD line, not a zero that looks clean. `consist_finish` writes `RETRIES=n LEFT=m` between `RSYNC-STATUS=` and `DIR=`. `consist_retry_counts` and the four key lines `RETRIED-RUNS-STAGE-x`, `RETRY-COMPLETED-RUNS-STAGE-x`, `ABORTED-24-RUNS-STAGE-x`, `MANIFEST-LESS-DIRS-STAGE-x`; the last above 0 is BAD. The window stage's reading of `backup.sh` finds `set -euo pipefail`, `run_rsync "${name}" -a "${DATA_DIR}/${name}/"`, `run_rsync()` and the MANIFEST line and says what the script now does.
- **Task 3, d4f8099.** `RC_HANDOVER` in `record_check` (documented next to `RC_REQ_PREFIX`), the retries line per stage and the three verdicts on the `CAPFULL:` line of `summary.txt`, and a section in `deploy/test/README.md`.

## Rehearsals (as run, with exit statuses)

Task 1, a real capfull against the changed script in a throwaway amd64 container, exit 0, last line `CAPFULL OK`:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-t29-trace OUT_DIR=deploy/test/out/amd64-t29-trace KEEP=1 NEW_REF=aa70280 bash deploy/test/nightly.sh source capfull
```

HAND, AGAIN and UNIT each ended `FAILED-NO-DIR` (backup status 11, unit `Result=exit-code`, `ExecMainStatus=11`), `DIRECTORY-MADE: none`, `REMOVAL-LINE: yes`, `SENTINEL-INTACT: yes`, `HAND-ROBUST`, `AGAIN-NEXT-NIGHT`, `AGAIN-ROBUST` and `UNIT-ROBUST` all `PASS`; the guard passed on the next night. No defect of `deploy/backup.sh` showed. Container removed with `docker rm -f holzcloud-t29-trace`; `docker ps -a` shows none.

Task 2, exit 0, last line `WINDOW OK`:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-t29-tr-src OUT_DIR=deploy/test/out/amd64-t29-tr-src TGT_NAME=holzcloud-t29-tr-tgt TGT_OUT_DIR=deploy/test/out/amd64-t29-tr-tgt KEEP=1 NEW_REF=b740726 BULK_MB=100 CONSIST_RUNS=4 CONSIST_MIN_RUNS=1 CONSIST_RESTORE_EVERY=1 bash deploy/test/nightly.sh source target prep consist window
```

All 8 RUN lines carry `RETRIES=` and `LEFT=`; `MANIFEST-LESS-DIRS-STAGE-adds: 0`; both stages have the four new key lines. One real repeat happened in stage deletes (`RSYNC-STATUS=24 RETRIES=1`, the night completed after the repeat), so the counters saw a live case: `RETRIED-RUNS-STAGE-deletes: 1`, `RETRY-COMPLETED-RUNS-STAGE-deletes: 1`. The window file holds the new reading with `run_rsync` at `deploy/backup.sh:165` and `:100`. Both containers removed with `down`. The scratch directories `deploy/test/out/amd64-t29-trace`, `amd64-t29-tr-src` and `amd64-t29-tr-tgt` are rehearsals, are gitignored and are never cited in a record.

## Verification

- `bash deploy/test/nightly.sh selftest`: `SELFTEST OK (594 cases)`; `bash -n deploy/test/nightly.sh`: 0.
- `bash deploy/test/nightly.sh record-check`: `RECORD-CHECK OK 29 blocks`; `E="$PWD/deploy/test/out" bash deploy/test/nightly.sh record-check-con`: `RECORD-CHECK OK 27 blocks`.
- `git diff --stat HEAD -- deploy/backup.sh deploy/test/run.sh deploy/restore.sh deploy/DEPLOY.md`: nothing.
- Counts: `capfull_robust_verdict` 27 mentions, `consist_retry_counts` 9, `RC_HANDOVER` 15 in nightly.sh and 1 in the README, old window reading `src_line deploy/backup.sh 'rsync -a`: 0.
- Expected red until plan 29-08: `28-check.sh regression` (selftest count), `beta evidence`, `frozen`. None was touched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The existing `con_summarize CAPFULL` selftest case was anchored to the end of the old line**
- **Found during:** Task 3
- **Issue:** The plan extends the `CAPFULL:` line; the case greps it with a trailing `$`, so the extension would have failed the selftest.
- **Fix:** The new keys are appended at the end of the line and the fixture and the expected string of that one case carry them. No other line of the summary changed order or text.
- **Files modified:** `deploy/test/nightly.sh`
- **Commit:** d4f8099

**2. [Rule 4 - judgment, no architectural change] ROB-03 is not marked complete**
- **Found during:** state updates
- **Issue:** The plan lists ROB-03 in its requirements and `requirements.mark-complete` ticked it, but ROB-03 is the evidence set on the changed script (`con-chain` on a clean tree, a record). This plan only builds the instrumentation; the evidence comes with plans 29-04 and 29-05.
- **Fix:** The tick was reverted (`git checkout -- .planning/REQUIREMENTS.md`); ROB-03 stays `Pending` for plan 29-05 to close. ROB-02 was already complete.

None otherwise; the code and the plan agree.

## Known Stubs

None.

## Threat Flags

None. The new key lines carry numbers, class words and directory names only; `leak_check` still runs at the end of capfull and consist and the rehearsals ended `LEAK-CHECK OK`.

## Self-Check: PASSED

- FOUND: deploy/test/nightly.sh (capfull_robust_verdict, capfull_next_night_verdict, consist_retry_counts, RC_HANDOVER)
- FOUND: deploy/test/README.md (RC_HANDOVER)
- FOUND commits: b740726, 234b6d2, d4f8099 (all ancestors of HEAD)
