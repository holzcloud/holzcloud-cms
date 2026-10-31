---
phase: 29-sicherung-robust
plan: 06
subsystem: planning
tags: [owner-steps, changelog, checker, selftest, verbatim, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: plan 29-05 record deploy/BACKUP-ROBUST-RUN.md (two numbered owner steps, evidence commit dd077e4)
provides:
  - OS-20 and OS-21 in 28-OWNER-STEPS.md, verbatim copies of items 1 and 2 of the record, made by program
  - 28-check.sh knows the record (ROB_REC): rec_extract, owner-steps complete, fixture and selftest cases
  - the 0.1.1 entry of CHANGELOG.md describes the changed backup.sh with the numbers of the new record
affects: [29-07, 29-08]
tech-stack:
  added: []
  patterns: [verbatim region copied by the checker's own extract verb, never retyped; a record step missing from the list is a selftest case that fails for its own message]
key-files:
  created: []
  modified:
    - .planning/phases/28-the-promise-in-writing/28-check.sh
    - .planning/phases/28-the-promise-in-writing/28-OWNER-STEPS.md
    - CHANGELOG.md
key-decisions:
  - "The record has two numbered owner steps, so the list gets two steps, OS-20 (item 1) and OS-21 (item 2); both Needs: nothing, because item 2 says 'after the first timer night' but names no earlier step of the list."
  - "ROB-04 is not ticked: the page docs/beta.md is plan 29-07, and the requirement names it."
requirements-completed: []
status: complete
commits: 3
plan_head_before: 104b656c7e1ea9cce25e6eb8227cf5291a3056bd
plan_head_after: a3b58173df28b406b987fe1e7731a787d5ba5345
duration: about 45 min
completed: 2026-10-29
actuals:
  tokens: 45000
  tasks: 3
  commits: 3
---

# Phase 29 Plan 06: The owner's steps and the release notes Summary

**OS-20 and OS-21 are byte-for-byte copies of the two owner steps of `deploy/BACKUP-ROBUST-RUN.md`, `28-check.sh` demands them, and the 0.1.1 entry of the CHANGELOG no longer describes the old `backup.sh`.**

## What was built

- **Task 1 (tracer), 8adeb1c.** `28-check.sh`: variable `ROB_REC`; the `rec_extract` branch of the consistency record now also serves `*/BACKUP-ROBUST-RUN.md` (same item format, no new extraction code); `check_owner_steps_complete` needs the record, lists its items among the references that each must have a step, and raises the minimum to 21 steps; the fixture `fx_owner` has OS-20 and OS-21 and 21 index rows; the comment at the top names the new record. `28-OWNER-STEPS.md`: `### OS-20` and `### OS-21` after OS-19, with the two index rows; each carries `Source: deploy/BACKUP-ROBUST-RUN.md item N`, `Where: real server`, `Needs: nothing`, one quoted span for `Expect begins at` and one for `A different output is explained at` (both found by the checker after the first command block), the commands of the item, the verbatim region produced by `bash 28-check.sh extract deploy/BACKUP-ROBUST-RUN.md 'item N'` redirected into the file, and one closing sentence naming the limit of the page and saying that OS-12 still serves for directories of the old script. The commands are read-only (a listing, a journal query, `systemctl status`) except the one run of `backup.sh` that item 1 names.
- **Task 2, 9799065.** The order text has a bullet for OS-20 and OS-21 (after the server runs this release's `backup.sh`, no GitHub step, neither needs the other). Selftest: a negative case for item 2 and one for an index without OS-20.
- **Task 3, a3b5817.** The six paragraphs of the 0.1.1 entry that described the old script were rewritten; the heading, the introduction and the 0.1.0 entry are untouched (`git diff -U0 CHANGELOG.md` shows hunks only on lines 23 to 33, all inside the entry).

## Selftest count of 28-check.sh

`bash 28-check.sh selftest` ends `SELFTEST FAILED (1 of 63 cases)`. Baseline at HEAD was `SELFTEST FAILED (1 of 59 cases)`, not `SELFTEST OK (59 cases)` as the plan said (see Deviations 1). The four cases added by this plan all pass: extractor on item 1 of the new record, the step of item 1 left out, the step of item 2 left out, the index without OS-20; the fixture also gained two steps (the existing good-fixture cases pass with them). The one failing case is the same as at baseline: `beta evidence on the good fixture`.

**Negative cases shown to fail for the right reason.** The extractor branch for `*/BACKUP-ROBUST-RUN.md` was temporarily removed (not committed, file restored from a copy; `git diff` afterwards held only the intended changes) and the selftest run: exactly two cases besides the pre-existing one failed, `the extractor finds item 1 of the robust record` and `owner steps: the good fixture, steps` (its OS-20 and OS-21 regions are then empty). The `complete` cases (a step left out, an index without OS-20) test the list against the record's numbered lines and do not use the extractor, so they stayed green, as designed; each of them fails on its own message when its transform is applied (`no owner step has 'Source: ... item N'`, `the index table at the top lists a different set`).

## CHANGELOG numbers and the line of the record each came from

Lines are those of `deploy/BACKUP-ROBUST-RUN.md` at a3b5817.

| In the entry | Value | Record line |
|--------------|-------|-------------|
| Files without a row, both stages | 38 of 80 (24 of 40 + 14 of 40; the sum is mine) | 13, 136, 151, 152 |
| Rows without a file, second stage | 16 of 40 | 13, 136, 152 |
| Runs in which database and files agreed | 26 of 80 (16 + 10; the sum is mine) | 136 |
| No backup refused | 0 of 80 | 136, 169 |
| Windows | 238 of 269 `overlaps-snapshot-step`, 28 `after-snapshot`, 3 `straddles-media-end` | 140 |
| Nights that met a real status 24 and completed | 8 of 40 (second stage), all eight with the file missing after the restore | 16, 122, 134 |
| Full-disk case | filesystem 1064 MB, floor 1024 MB, data 1225 MB, rsync and backup.sh status 11, unit `Result exit-code` | 15, 107, 241 |
| The next night | passed the guard, failed the same way, no directory | 120 |
| Status 24 by hand and under the unit | once: `Result=success`; twice: status 24, no directory, `Result exit-code` | 16, 81, 93 |
| Stand-in for rsync | named, DEVIATION[vanish-shim], F-9 | 51, 142 |
| Evidence commit | dd077e4 | 19 |
| Earlier set built from 85fa486; difference to dd077e4 | 11 files, 2856 insertions, 36 deletions (`git diff --stat 85fa486 dd077e4 -- cmd internal go.mod go.sum`, run by me) | not in the record; from git |

The old numbers 41 of 80 and 11 of 40 are gone (`grep -c '11 von 40'` and `grep -c '41 von'` print 0 in the entry).

## git diff --quiet v0.1.0 HEAD

`git diff --quiet v0.1.0 HEAD -- deploy/restore.sh deploy/holzcloud-backup.service deploy/holzcloud-backup.timer` printed nothing and exited 0, so the three files are byte for byte those of 0.1.0 and the sentence was written. `git diff --quiet v0.1.0 HEAD -- deploy/backup.sh` exited 1; `git diff --numstat` shows 89 insertions and 5 deletions. `git diff v0.1.0 HEAD -- deploy/backup.sh` was read in full: it holds the repeat of status 24 (`run_rsync`, also used for the off-site copy), the cleanup of the own directory plus a refusal to start when the directory of the night exists already (`mkdir` instead of `mkdir -p`), and the `MANIFEST.partial` rename. No other line changed, so "its other lines are unchanged" is true; the refusal to start is named in the entry together with the removal.

## Deviations from Plan

### Auto-fixed Issues

**1. [Baseline differs from the plan] The selftest was not green at the start**
- **Found during:** Task 1 (first run of the selftest)
- **Issue:** The plan states the baseline as `SELFTEST OK (59 cases)`. It was `SELFTEST FAILED (1 of 59 cases)`: the case `beta evidence on the good fixture` runs the real `git diff --quiet` commands of the evidence section against the real repository, and since plan 29-01 changed `deploy/backup.sh` (a bound file of the earlier evidence) three of them exit non-zero. `28-check.sh beta` shows the same three FAIL lines.
- **Fix:** none; out of scope. The failure is caused by an earlier plan and belongs to plans 29-07 (evidence section) and 29-08 (re-pin), whose SUMMARYs from 29-01 to 29-03 name it as expected red. The verification of this plan therefore reads "no case other than that one fails and the count grew 59 to 63", not `SELFTEST OK`.
- **Files modified:** none

**2. [Rule 1 - Bug] A veto point of the owner's list said the opposite of the record**
- **Found during:** Task 2
- **Issue:** the paragraph "`deploy/DEPLOY.md` is not edited" of 28-OWNER-STEPS.md listed the three gaps as "a vanished file ends the night, a failed night's directory blocks the following nights, and a restore does not report a row whose file is missing"; the first two are untrue of the script that ships since plan 29-01.
- **Fix:** the sentence now reads "a vanished file is repeated once, a failed night removes its own directory, and a restore does not report a row whose file is missing" as the three statements that `docs/beta.md` and the steps carry instead of DEPLOY.md. The text of OS-1 to OS-19 is untouched (`git diff -U0` shows hunks at the index table, the order text, the veto paragraph and the insertion before "## Veto points", none inside OS-1 to OS-19).
- **Files modified:** `.planning/phases/28-the-promise-in-writing/28-OWNER-STEPS.md`
- **Commit:** 9799065

**3. [Rule 2 - correctness] The entry names two facts the plan did not list**
- **Found during:** Task 3
- **Issue:** reading `git diff v0.1.0 HEAD -- deploy/backup.sh` showed that the script also refuses to start when the directory of the night exists, and that the repeat covers the off-site copy too, which the record says was played only by the hermetic test. Leaving these out would have made "its other lines are unchanged" misleading.
- **Fix:** both are in the paragraph "Ein volles Laufwerk und eine verschwundene Datei."
- **Commit:** a3b5817

## Known Stubs

None.

## Threat Flags

None. T-29-19: the region is copied by program and compared with `cmp` (`owner-steps steps` passes), and the existing rules against a force push, a deletion, a tag and a secret ran over the new steps. T-29-20: every number is listed above with its line; the `git diff --quiet` was run before the sentence about `restore.sh` and the units was written; the stand-in and the timer chain are named in the entry.

## Deferred / noted

- `CHANGELOG.md` paragraph "Was sich am Programm seit 0.1.0 geändert hat." does not name the EXIF orientation and colour-profile changes in `internal/media` that `git diff --stat v0.1.0 HEAD -- cmd internal go.mod go.sum` (41 files) contains; the plan asked only for the backup sentences, and the paragraph "Was die Belege nicht mehr decken." now names them as the difference between the two evidence builds. Whether the former should list them is the owner's call.
- Expected red until plans 29-07 and 29-08: `28-check.sh beta` (evidence section: three FAIL lines), the one selftest case, `frozen`, `regression`.

## Verification run

- `28-check.sh owner-steps`: both `ok:` lines (21 steps; complete), exit 0.
- `28-check.sh changelog`: three `ok:` lines (extract: 0.1.1, 10650 bytes of notes, 8 bold paragraphs; topics; go).
- `28-check.sh selftest`: `SELFTEST FAILED (1 of 63 cases)`, the failing case is the pre-existing one.
- Counts: headings `### OS-n:` 21; index rows 21; numbered lines of the record 2; steps with `Source: deploy/BACKUP-ROBUST-RUN.md item` 2.

## Self-Check: PASSED

- 28-check.sh, 28-OWNER-STEPS.md, CHANGELOG.md exist and were changed in 8adeb1c, 9799065, a3b5817 (all ancestors of HEAD).
- Not committed by design: `.planning/config.json`, `.planning/milestone.lock`.
