---
phase: 25-consistent-and-compatible-backups
verified: 2026-10-31T12:30:00Z
status: passed
score: 6/6 must-haves verified
covered_files:
  - .planning/phases/25-consistent-and-compatible-backups/25-01-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-01-SUMMARY.md
  - .planning/phases/25-consistent-and-compatible-backups/25-02-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-02-SUMMARY.md
  - .planning/phases/25-consistent-and-compatible-backups/25-03-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-03-SUMMARY.md
  - deploy/BACKUP-CONSISTENCY-RUN.md
  - deploy/backup.sh
  - deploy/restore.sh
  - deploy/test/README.md
  - deploy/test/nightly.sh
covered_digest: "v4:sha256:f0a15247544c724084cf471c92013537eaa1919bf98d6c8017862646bb281b25"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 7/7
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  note: "Regenerated because phase 29 changed deploy/backup.sh, deploy/test/nightly.sh and deploy/test/README.md after the first report. Truth 4 and truth 5 of the first report were point-in-time guards; they are restated below as 'did phase 25 edit a shipped file' (still true) and 'what changed since' (phase 29, deliberate)."
advisory:
  - finding: "WR-05 (open in 25-REVIEW-DISPOSITION.md): deploy/BACKUP-CONSISTENCY-RUN.md and deploy/test/README.md still state bound-file facts that phase 29 made false, and neither carries a pointer to deploy/BACKUP-ROBUST-RUN.md"
    category: other
    reason: "Record line 'No bound file changed after the evidence commit' and its git diff --quiet command: at HEAD that command exits 1 (backup.sh +89/-5, nightly.sh +577/-16). The record's F-3, F-4, F-5 and 'Left to the owner' paragraph describe backup.sh as it was at e86d490; at HEAD a first status 24 is repeated once and a failed night removes its own directory. README lines 365, 369, 409, 411 and 447 say 'exits 0', 'grew by additions' and 'SELFTEST OK (542 cases)'; the current selftest ends 646 cases. docs/beta.md (E-5, line 123) and deploy/BACKUP-ROBUST-RUN.md (line 3: 'The earlier record describes a backup.sh that no longer ships') do say it, so the supersession is documented elsewhere, but a reader who follows the phase 25 record's own command gets a failure with no hint why. Fix: a dated 'superseded by BACKUP-ROBUST-RUN.md for the script that ships' note at the top of the record and a 'at least 542' wording in the README."
    evidence_status: "reproduced: git diff --quiet e86d490 HEAD -- <bound files> exits 1; selftest prints 646"
  - finding: "The phase 25 outcome for CON-03 and F-3/F-5 describes behaviour the shipped script no longer has"
    category: other
    reason: "Not a defect of phase 25. F-3 (status 24 ends the night) and F-5 (a left directory makes the guard refuse later nights) were handed to the owner; phase 29 closed both in backup.sh (run_rsync repeats status 24 once; cleanup_failed_night removes the night's own directory) and re-ran CON-01 to CON-03 on the changed script at dd077e4 (CON-01 PASS, CON-02 PASS, CON-03 PASS, 96 steps played, 0 failed, in deploy/BACKUP-ROBUST-RUN.md). backup.sh and restore.sh are byte-identical between dd077e4 and HEAD."
    evidence_status: "git diff --quiet dd077e4 HEAD -- deploy/backup.sh deploy/restore.sh exits 0"
  - finding: "Open code-review warnings WR-01 to WR-04 and WR-06 (and IN-01 to IN-05) concern the harness, not the claims"
    category: other
    reason: "Per 25-REVIEW.md none turns a failure into a PASS: the verdict functions are conservative. They make the harness fail too often or lose evidence (WR-01 drops raw files for an AGREE run that leaves a MANIFEST-less directory, WR-02 runs consist_run with set -e off, WR-03 never checks that a new unit invocation started, WR-04 reads mechanism lines from the working tree not NEW_REF, WR-06 lacks a NAME != TGT_NAME guard before an rm -rf). None touches a number the record cites; the recorded runs have 0 BAD lines. They bear on the next evidence set, not on this one."
    evidence_status: "review read; headline numbers recomputed below, all agree"
behavior_unverified_items: []
---

# Phase 25: Consistent and compatible backups, Verification Report

**Phase Goal:** A backup made while the site is being written is either consistent or its window is named; a backup of the previous release restores.
**Verified:** 2026-10-31
**Status:** passed
**Re-verification:** Yes. The report of 2026-10-26 was regenerated against HEAD because phase 29 changed `deploy/backup.sh`, `deploy/test/nightly.sh` and `deploy/test/README.md` after it. All raw evidence was read again; no SUMMARY claim was used as evidence. Docker and the 35-minute chain were not run: the evidence set in `deploy/test/out/amd64-t25-*` is still on disk and was re-read.

## What phase 29 changed in what phase 25 describes

| Subject | Phase 25 record says | HEAD | Effect on the phase 25 goal |
|---|---|---|---|
| `deploy/backup.sh` | Unchanged; bound to `e86d490` | Changed: `run_rsync` repeats status 24 once; `cleanup_failed_night` removes a failed night's own directory; MANIFEST written as `MANIFEST.partial` then renamed; refuses to start if its directory exists (`git diff --numstat e86d490 HEAD`: 89 added, 5 removed) | None on the goal. The measurements are of the script at `e86d490`; they are re-made on the changed script by phase 29 (below). |
| `deploy/restore.sh`, `DEPLOY.md`, `holzcloud.service`, both backup units, `run.sh` | Unchanged | Unchanged (`git diff --quiet e86d490 HEAD` exits 0 for each) | None |
| `deploy/test/nightly.sh` | 3664 added, 3 removed against `b501b1b`; selftest 542 cases | 4225 added, 3 removed against `b501b1b` (the tripwire `grep -c '^-[^-]'` still prints 3, limit 6); selftest ends `SELFTEST OK (646 cases)` | Record's selftest count and "bound file unchanged" are stale; the additions-only tripwire still holds |
| `deploy/test/README.md` | Phase 25 section with "exits 0" and "542 cases" | One line removed, 143 added; the Phase 29 section now exists | Stale statements, see WR-05 |
| F-3 (status 24 ends the night) and F-5 (left directory makes the guard refuse later nights) | Class 3, handed to the owner | Closed by phase 29: `backup.sh` repeats status 24 once and removes its own failed directory (`deploy/BACKUP-ROBUST-RUN.md`: `HAND-CLASS: FAILED-NO-DIR`, `AGAIN-CLASS: FAILED-NO-DIR`, 8 of 40 nights of stage deletes repeated and completed) | The behaviour the phase 25 record names no longer ships; the record's description is historical |
| `docs/beta.md` | "not edited here (phase 28)" | Edited by phases 28 and 29. Line 9 and P-16 to P-22 rest on `BACKUP-ROBUST-RUN.md`; E-5 (line 123) states that the `e86d490` set "no longer" binds P-16 to P-18 | The CON-01 half "docs/beta.md names the window" is now met (P-16 names `overlaps-snapshot-step`, `after-snapshot`, `straddles-media-end` and the run) |

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: a backup taken while pages are saved and media is uploaded is restored with database and media in agreement, or the disagreement is reproduced and its window named | VERIFIED | Re-read from `deploy/test/out/amd64-t25-src/consist.txt`: `CONSIST OK`, `MIN-RUNS: 30`, `BINARY: holzcloud v0.1.0-46-ge86d490`. Stage adds `RUNS=40 RESTORED=40 AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 UPLOADS-IN-WINDOW=84`. Stage deletes `RUNS=40 RESTORED=32 AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8 RESTORE-FAILED=0 UPLOADS-IN-WINDOW=96`. 220 `WINDOW-NAME` lines (the review recomputed the same), 0 `BAD` lines in `prep.txt`, `consist.txt`, `window.txt`, `capfull.txt`, `compat.txt`. `window.txt` ends `WINDOW OK` with controls C0 AGREE, C1 ORPHAN-ONLY, C2 ROW-WITHOUT-FILE. `summary.txt`: `CON-01 PASS`, `STEPS: 96 played, 0 failed`, `LEAK-CHECK OK`; `chain-statuses.txt`: all six stages 0. The roadmap's second branch (reproduced and named) is met, both directions seen. Still true at HEAD: phase 29 re-ran the same stage on the changed script (`CON-01 PASS`, stage adds 16 AGREE / 24 ORPHAN-ONLY, stage deletes 10 / 14 / 16, `BACKUP-ROBUST-RUN.md` line 13) and `docs/beta.md` P-16 and limit line 48 name the window. |
| 2 | SC2: a 0.1.0 backup restored by this release gives the same pages, the same login and the migration level the record states | VERIFIED | `amd64-t25-old/compat.txt` ends `COMPAT OK`: `RESTORE-RC: 0`; `STATE-SOURCE` and `STATE-RESTORED` both `LEVEL: 60 ... PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4`; `MEDIA-SERVED-EQUAL: 4 of 4` with matching sha256; setup form 404 after restore; `LEVEL-BEFORE: 60`, `LEVEL-AFTER: 60`; `PASS run.sh check ... 17 of 17`; `SCRIPTS-IDENTICAL` yes for `backup.sh` (`d9f807d8...`) and `restore.sh` (`9a411d72...`) at the tag and at `e86d490`. `deploy/restore.sh` is byte-identical at HEAD, so the restore half of this claim is unchanged. Phase 29 re-made the pair at `dd077e4` (`CON-02 PASS`); there `backup.sh` of the tag and of the release differ (F-7 of `BACKUP-ROBUST-RUN.md`), and the compatibility shown is old script and old binary against the new `restore.sh`. No migration separates the releases (both level 60); stated as not tested in the record and in `docs/beta.md` line 51. |
| 3 | SC3: a nearly full disk above `MIN_FREE_MB` produces a recorded outcome | VERIFIED | `capfull.txt` ends `CAPFULL OK`: `FREE-AT-GUARD-MB: 1064` against floor 1024, `HAND-GUARD: passed`, `HAND-BACKUP-RC: 11`, `HAND-RSYNC-STATUS: 11`, `HAND-NO-SPACE-LINES: 1`, `HAND-DIRECTORY-BYTES: 1098915817`, `HAND-DIRECTORY-FILES: 4221`; `EARLIER-BACKUPS-AFTER` is 66 directories and "the same directories as before". The outcome is recorded, as the criterion asks; it is not claimed harmless. At HEAD the outcome differs (the failed night now removes its directory and the next night is not refused): phase 29 re-played this at `dd077e4` (`CON-03 PASS`, `HAND-CLASS: FAILED-NO-DIR`, `UNIT-EXECMAINSTATUS: 11`; `BACKUP-ROBUST-RUN.md` line 15). The two sets are consistent with the two scripts they were made on. |
| 4 | Phase 25 edited no shipped file (`backup.sh`, `restore.sh`, `DEPLOY.md`, both units, `run.sh`) and not `docs/beta.md` | VERIFIED | `git log b501b1b..c4fb8cd -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md docs/beta.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` prints nothing (0 commits): across the phase 25 commits none of these files changed. The first report's wording "unchanged since the evidence commit" is NOT true at HEAD (`git diff --quiet e86d490 HEAD` exits 1 for `backup.sh`, `nightly.sh`; the cause is phase 29, table above). That is a statement about the files after phase 25, and is carried as advisory WR-05, not as a gap in the phase goal. |
| 5 | `nightly.sh` carries phase 25 as additions only | VERIFIED | `git diff --numstat b501b1b e86d490 -- deploy/test/nightly.sh` gives 3664 / 3; at HEAD against `b501b1b` it gives 4225 / 3 and `git diff -U0 b501b1b -- deploy/test/nightly.sh | grep -c '^-[^-]'` prints 3 (limit 6). Phase 24's code is not changed by either later phase. |
| 6 | The record claims no more than was measured, and its proofs reproduce | VERIFIED | Run now: `bash deploy/test/nightly.sh record-check-con` ends `RECORD-CHECK OK 27 blocks`; `record-check` ends `RECORD-CHECK OK 29 blocks`; `selftest` ends `SELFTEST OK (646 cases)` (the record says 542; the count only grew, see WR-05); `bash -n deploy/test/nightly.sh` passes. Every number I re-read from the raw files above equals the number in the record's Summary table. The "Not tested" and "Deviations named" sections remain conservative (tmpfs not a disk, emulation, not real traffic, no migration crossed, status 24 under the unit not played). |

**Score:** 6/6 truths verified, 0 behavior-unverified. The truths are about recorded runs, which are the behavioral evidence: each runs the shipped script against a real load, restores it with the shipped `restore.sh` and compares both directions, so no invariant here rests on symbol presence.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/BACKUP-CONSISTENCY-RUN.md` | The record with a byte-proven appendix and the hand-over | VERIFIED | Exists, 1373 lines; `record-check-con` proves 27 blocks. No `TBD`/`FIXME`/`XXX`. Stale on the bound-file statements (WR-05). |
| `deploy/test/nightly.sh` | `consist`, `window`, `capfull`, `compat-source`, `compat`, `con-chain`, `record-check-con` | VERIFIED | Present and syntactically valid; `selftest` 646 cases OK; the evidence set it produced is intact on disk. |
| `deploy/test/README.md` | Phase 25 section | VERIFIED, stale in places | Section present; "542 cases", "exits 0" and "grew by additions" were true when written (WR-05). |
| `deploy/test/out/amd64-t25-{src,tgt,old,new}`, `amd64-t25-evidence-commit.txt` | Raw evidence set | VERIFIED | All present; evidence commit file holds `e86d490`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `con-chain` | `summary.txt` verdicts | stage files and statuses | WIRED | `CON-01 PASS`, `CON-02 PASS`, `CON-03 PASS` in `summary.txt`; `chain-statuses.txt`: `prep=0 consist=0 window=0 capfull=0 compat-source=0 compat=0`. |
| Record blocks | scratch files | `record-check-con` | WIRED | 27 blocks proven byte for byte, run now. |
| Record | `docs/beta.md` | hand-over to phase 28 | WIRED | beta.md P-16 to P-18 and limit lines 48 to 55 cite the records; E-5 marks the phase 25 set as the earlier one. |

### Data-Flow Trace (Level 4)

Not applicable: the phase produces a harness and a record, not rendered dynamic data. The record's numbers trace to scratch files, which `record-check-con` proves.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Record equals its scratch files | `bash deploy/test/nightly.sh record-check-con` | `RECORD-CHECK OK 27 blocks` | PASS |
| Phase 24 record still proven | `bash deploy/test/nightly.sh record-check` | `RECORD-CHECK OK 29 blocks` | PASS |
| Harness self-test | `bash deploy/test/nightly.sh selftest` | `SELFTEST OK (646 cases)` | PASS |
| Harness parses | `bash -n deploy/test/nightly.sh` | exit 0 | PASS |
| Verdicts and statuses in the set | read `summary.txt`, `chain-statuses.txt` | CON-01/02/03 PASS, 0 failed, all stages 0 | PASS |
| Regression gate (given by the orchestrator) | `go test ./...` | 56 packages ok | PASS (not re-run) |

### Probe Execution

SKIPPED: phase 25 declares no `scripts/*/tests/probe-*.sh`. Its proofs are `record-check-con`, `record-check` and `selftest`, run above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CON-01 | 25-01, 25-03 | Backup under writes: agree, or the window is named with the run that provoked it | SATISFIED | Truth 1. The `docs/beta.md` half, deferred to phase 28 by the CONTEXT decision, is now met (P-16, limit line 48). |
| CON-02 | 25-02, 25-03 | A 0.1.0 backup restores with this release; pages, login, migration level recorded | SATISFIED | Truth 2. |
| CON-03 | 25-02, 25-03 | Nearly full disk above `MIN_FREE_MB` provoked, outcome recorded | SATISFIED | Truth 3. |

All three IDs appear in PLAN frontmatter (25-01: CON-01; 25-02: CON-02, CON-03; 25-03: CON-01 to CON-03) and in `REQUIREMENTS.md` lines 27 to 29 (`[x]`) and 106 to 108 (`Complete`). No requirement is mapped to phase 25 without a plan; no orphans. FUT-02 stays a future item: all 220 disagreements have a named window.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `deploy/BACKUP-CONSISTENCY-RUN.md` | 33 to 37, 41 to 43, 265, 267 | Statement made false by a later phase, no pointer to the successor | Warning (WR-05) | A reader following the record's own `git diff --quiet e86d490 HEAD` gets exit 1 and cannot tell evidence from HEAD |
| `deploy/test/README.md` | 365, 369, 409, 411, 447 | "exits 0", "grew by additions", "542 cases" | Warning (WR-05) | Same |
| `deploy/test/nightly.sh` | 3606 to 3796, 3905, 5983 to 6010, 6233 to 6258, 4616 to 4652, 3661 | WR-01 to WR-04, WR-06 of the review | Warning | Harness robustness for the next set; no cited number depends on them |
| `deploy/test/nightly.sh` | 174, 71, 6213 to 6229, 3596, 3794 | IN-01 to IN-05 | Info | Dead code, stale header, a step that is `not-run`, duplicated sequence, cleanup path |

No `TBD`, `FIXME` or `XXX` in the record. 0 blockers. The review found 0 critical issues; 6 warnings and 5 infos remain `open` in `25-REVIEW-DISPOSITION.md`.

### Human Verification Required

None. What a container cannot show (real disk with reserved blocks, production-sized media, a restore that crosses a migration, native timing, status 24 under the unit by a real vanished file) is in the record's *Not tested* list and in `docs/beta.md`'s limits (lines 49 to 55, 63, 64) with owner steps. It is not claimed and is not an unverified truth of this phase.

### Gaps Summary

No gaps. The three roadmap success criteria hold on the evidence set made at `e86d490`, whose raw files I re-read and whose record still proves byte for byte. They hold at HEAD as well: phase 29 changed the script and re-made all three on it, and `restore.sh` is untouched. The one live defect against the phase 25 deliverables is documentary (WR-05): the record and the README do not say that they describe `backup.sh` as of `e86d490` and that its F-3 and F-5 are closed by phase 29. It corrupts no measurement and `docs/beta.md` E-5 and `BACKUP-ROBUST-RUN.md` line 3 already state the supersession, so it is an advisory, not a BLOCKER. It is recommended to close it with a note at the top of the record and a "542 or more" wording in the README (a documentation edit that touches no bound file; the record and README are not bound files).

---

_Verified: 2026-10-31_
_Verifier: Claude (gsd-verifier)_
