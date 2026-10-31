---
phase: 29-sicherung-robust
plan: 03
subsystem: deploy
tags: [backup, harness, capfull, vanish, status-24, stand-in, selftest, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: plan 29-01 run_rsync and the contract strings, plan 29-02 capfull_robust_verdict, cf_judge REMOVAL-LINE and capfull_report_robust
provides:
  - vanish_verdict and vanish_unit_verdict, pure verdicts with selftest cases for each way they fail
  - vanish_install, vanish_remove, vanish_gone, cf_run_unit, vanish_scenario
  - four capfull scenarios VANISH-ONCE, VANISH-TWICE, VANISH-ONCE-UNIT, VANISH-TWICE-UNIT
  - the VANISH line of summary.txt and DEVIATION[vanish-shim]
affects: [29-04, 29-05, 29-07, 29-08]
tech-stack:
  added: []
  patterns: [stand-in that runs the real tool first and then replaces its status, written by the harness and asserted absent on every exit path, one generic scenario function parameterised by label, kind and mode]
key-files:
  created: []
  modified:
    - deploy/test/nightly.sh
    - deploy/test/README.md
key-decisions:
  - "The stand-in runs the real /usr/bin/rsync first and only then replaces the status, so the copy has happened and what is played is the script's and the unit's handling of a 24, not rsync's detection."
  - "Under the unit the combined LABEL-VERDICT is the first of vanish_verdict and vanish_unit_verdict that is not PASS; the unit's own part is written as LABEL-UNIT-STATE-CHECK so that exactly four keys end in -VERDICT."
  - "vanish_scenario is one function for all four scenarios; stage (5) of capfull_run keeps its own inline unit code and cf_run_unit is a copy of its logic and bounds."
requirements-completed: []
status: complete
commits: 3
plan_head_before: c55a95f5956005809df6ce29cecb8e161509be00
plan_head_after: e4cc178c77e680d9e740955bb182922fc08ddd41
duration: 45min
completed: 2026-10-27
actuals:
  tokens: 8300
  tasks: 3
  commits: 3
---

# Phase 29 Plan 03: The real script and the real unit meet a status 24 Summary

**capfull now plays the shipped backup.sh, by hand and under holzcloud-backup.service, against a stand-in for rsync that delivers status 24 once (the night completes after the repeat) and twice (status 24, the copy named, no directory, Result=exit-code); the selftest ends SELFTEST OK (646 cases).**

## Selftest case count

`bash deploy/test/nightly.sh selftest` ends `SELFTEST OK (646 cases)`. Baseline (end of plan 29-02) was 594. After Task 1: 626 (32 cases for `vanish_verdict`). After Task 2: 644 (18 for `vanish_unit_verdict`). After Task 3: 646 (2 for the `VANISH:` line). `bash deploy/test/backup-robust.sh` ends `BACKUP-ROBUST OK (94 cases)`. Plan 29-08 pins 646 (it was told 594 by plan 29-02).

## What was done

- **Task 1 (tracer), 6225d8c.** `vanish_verdict KIND CLASS RC REPEAT_LINES GIVEUP_LINES` (pure, refuses an unknown word with status 1 and nothing printed). The stand-in `VANISH_SHIM_SH`, `vanish_install FAILS` (refuses to overwrite a `/usr/local/bin/rsync` it did not write), `vanish_remove` (removes the binary only if it carries its marker), `vanish_gone` (removes and asserts with `test ! -e` that all three files are gone). `vanish_scenario` plays one night by hand on a fresh roomy filesystem with its own sentinel. `DEVIATION[vanish-shim]` is named once before the first scenario; `capfull_cleanup` calls `vanish_remove` on every exit path.
- **Task 2, 88cf901.** `vanish_unit_verdict KIND RESULT EXECMAINSTATUS ACTIVESTATE`, `cf_run_unit LABEL TRANSCRIPT` (logic and bounds of stage (5) copied, stage (5) untouched), the `unit` mode of `vanish_scenario`, and the three further scenarios in the order VANISH-TWICE by hand, VANISH-ONCE-UNIT, VANISH-TWICE-UNIT. `VANISH-TWICE` and `VANISH-TWICE-UNIT` also run `capfull_report_robust` (`VANISH-TWICE-ROBUST`, `VANISH-TWICE-UNIT-ROBUST`).
- **Task 3, e4cc178.** The `VANISH:` line of `con_summarize` after the `CAPFULL:` line (the fixture of its selftest carries the keys), and a README section "The vanish scenarios of capfull since plan 29-03": the four scenarios, the stand-in and where it lives, its removal, the deviation by name, what is and is not proved.

## Rehearsals (as run, with exit statuses)

Both rehearsals ran `bash deploy/test/nightly.sh source capfull` against an amd64 `systemd-base` container (`PLATFORM=linux/amd64 TARGET=systemd-base KEEP=1`), exit 0, last line `CAPFULL OK`, and the container was removed with `docker rm -f`; `docker ps -a` shows no `holzcloud-t29` container.

- Task 1: `NAME=holzcloud-t29-trace2 OUT_DIR=deploy/test/out/amd64-t29-trace2 NEW_REF=6225d8c^` (HEAD before the commit, c55a95f). `VANISH-ONCE-BACKUP-RC: 0`, `VANISH-ONCE-RSYNC-STATUS: 24`, `VANISH-ONCE-MANIFEST: yes`, `VANISH-ONCE-VERDICT: PASS`, `VANISH-SHIM-REMOVED: yes`; one `WARNING: rsync exited 24` line in `capfull-vanish-once.txt`.
- Task 3: `NAME=holzcloud-t29-trace3 OUT_DIR=deploy/test/out/amd64-t29-trace3 NEW_REF=e4cc178`, on the committed text. Scratch directories under `deploy/test/out` are gitignored and are not cited anywhere.

The stand-in was reached in every scenario, by hand and under the unit (the repeat line count is 1 in all four transcripts, and the giving-up line count is 0 for the two ONCE and 1 for the two TWICE scenarios). The PATH did not need to be investigated.

### The four verdict lines (trace3)

```text
VANISH-ONCE-VERDICT: PASS
VANISH-TWICE-VERDICT: PASS
VANISH-ONCE-UNIT-VERDICT: PASS
VANISH-TWICE-UNIT-VERDICT: PASS
```

Behind them: `VANISH-ONCE-CLASS: COMPLETE` (RC 0, RSYNC-STATUS 24, repeat 1, giving-up 0); `VANISH-TWICE-CLASS: FAILED-NO-DIR` (RC 24, DIRECTORY-MADE none, repeat 1, giving-up 1, `VANISH-TWICE-ROBUST: PASS`); `VANISH-ONCE-UNIT-CLASS: COMPLETE` (`Result=success`, `ExecMainStatus=0`, `inactive`); `VANISH-TWICE-UNIT-CLASS: FAILED-NO-DIR` (`VANISH-TWICE-UNIT-RESULT: exit-code`, `ExecMainStatus=24`, `failed`, DIRECTORY-MADE none, `VANISH-TWICE-UNIT-ROBUST: PASS`). `VANISH-SHIM-REMOVED: yes`. `deviations.txt` holds one `DEVIATION[vanish-shim]` line.

The ten key lines of plan 29-02 in the same file: `HAND-REMOVAL-LINE`, `AGAIN-REMOVAL-LINE`, `UNIT-REMOVAL-LINE` all `yes`; `HAND-ROBUST`, `AGAIN-ROBUST`, `UNIT-ROBUST`, `AGAIN-NEXT-NIGHT` all `PASS`.

## Verification

- `bash -n deploy/test/nightly.sh`: 0; selftest `SELFTEST OK (646 cases)`; `backup-robust.sh` `BACKUP-ROBUST OK (94 cases)`.
- `git diff --stat HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/test/run.sh deploy/holzcloud-backup.service deploy/holzcloud-backup.timer`: nothing.
- `git status --porcelain`: only `.planning/config.json` and `.planning/milestone.lock`, which are not part of this plan and were not committed.
- Expected red until plan 29-08: `28-check.sh regression` (selftest count), `beta evidence`, `frozen`. None was touched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A missing space in a selftest line**
- **Found during:** Task 1
- **Issue:** One `st_eq` case had its expected string and its command substitution glued together, so the selftest died with an unbound `$3`.
- **Fix:** The space was added; the selftest ran to its end.
- **Commit:** 6225d8c

**2. [Plan wording] Acceptance check `grep -c '^VANISH: '` cannot match**
- **Found during:** Task 3
- **Issue:** The `VANISH:` line is an indented `echo` inside `con_summarize`, so a count anchored at the start of a line is 0. The line is there; it is covered by the selftest cases "con_summarize VANISH" and "con_summarize VANISH follows the CAPFULL line", and `grep -c 'echo "VANISH: ' deploy/test/nightly.sh` prints 1.
- **Fix:** None needed in the code.

**3. [Judgment] ROB-01 and ROB-03 are not marked complete**
- **Issue:** The plan lists ROB-01 and ROB-03 in its requirements. This plan only builds the instrumentation and rehearses it; the evidence on the committed text is made by plan 29-04 and closed by plan 29-05. Requirements were not ticked.

None otherwise.

## Known Stubs

None.

## Threat Flags

None. The stand-in exists only in the throwaway container (T-29-09 and T-29-10 are mitigated as planned: `vanish_gone` asserts absence after every scenario, `capfull_cleanup` removes it on every exit path, `DEVIATION[vanish-shim]` names it; T-29-12 by the repeat-line and giving-up-line counts in the verdicts). Both rehearsals ended `LEAK-CHECK OK`.

## Self-Check: PASSED

- FOUND: deploy/test/nightly.sh (vanish_verdict, vanish_unit_verdict, vanish_install, vanish_remove, cf_run_unit, vanish_scenario, `VANISH: ` line)
- FOUND: deploy/test/README.md (section on the vanish scenarios)
- FOUND commits: 6225d8c, 88cf901, e4cc178 (all ancestors of HEAD)
