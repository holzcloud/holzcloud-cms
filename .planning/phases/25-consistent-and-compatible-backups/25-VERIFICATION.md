---
phase: 25-consistent-and-compatible-backups
verified: 2026-10-26T16:30:00Z
status: passed
score: 7/7 must-haves verified
covered_files:
  - .planning/phases/25-consistent-and-compatible-backups/25-01-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-01-SUMMARY.md
  - .planning/phases/25-consistent-and-compatible-backups/25-02-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-02-SUMMARY.md
  - .planning/phases/25-consistent-and-compatible-backups/25-03-PLAN.md
  - .planning/phases/25-consistent-and-compatible-backups/25-03-SUMMARY.md
  - deploy/BACKUP-CONSISTENCY-RUN.md
  - deploy/test/README.md
  - deploy/test/nightly.sh
covered_digest: "v3:sha256:7f8d63ad8e66e504ad9d99c69353374c056ba0333a756c9f78c8e6a6d8100dfc"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "F-3 and F-5 are labelled class 3 ('matches what the file and its document say') although the record itself says DEPLOY.md is silent on both"
    category: other
    reason: "The label is stretched; the record states the documentation gap openly and hands it to phase 28, so nothing is hidden. Whether either is a class-4 defect (retention never runs on a failed night; a vanished file ends the night) is an owner decision the record already lists."
    evidence_status: "judgment, not a failed check"
  - finding: "F-6 (sixth restore within 5 minutes hits the start limit) rests on traces that no longer exist in deploy/test/out"
    category: other
    reason: "The record says outright that the traces are not part of the set and not cited, and shows only the unit values (StartLimitIntervalSec=300, StartLimitBurst=5, present in deploy/holzcloud.service). It is a reading of the unit, not a measurement of this set."
    evidence_status: "unit text confirmed; trace dir amd64-t25-trace-startlimit absent"
  - finding: "The F-3 excerpt (consist-deletes-run-9-backup.txt) is not contiguous: the raw file has three 'file has vanished' lines, the record's quote shows one, with no ellipsis"
    category: other
    reason: "Cosmetic. The full transcript is in the byte-proven appendix and the RC 24 / no later line claim holds."
    evidence_status: "raw file read"
---

# Phase 25: Consistent and compatible backups, Verification Report

**Phase Goal:** A backup made while the site is being written is either consistent or its window is named; a backup of the previous release restores.
**Verified:** 2026-10-26
**Status:** passed
**Re-verification:** No, initial verification

All checks were run by the verifier against the working tree and `deploy/test/out/amd64-t25-*`. The multi-hour chain and Docker were not run. The SUMMARY claims were not used as evidence.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: a backup taken while pages are saved and media is uploaded is restored; database and media agree, or the disagreement is reproduced and its window written down | VERIFIED | `consist.txt` ends `CONSIST OK`, `MIN-RUNS: 30`, 40 runs per stage. I recomputed from `consist-runs.txt`: adds 40 runs, 12 AGREE, 28 ORPHAN-ONLY, 40 restored, BACKUP-RC 0=40; deletes 40 runs, 8 AGREE, 13 ORPHAN-ONLY, 11 ROW-WITHOUT-FILE (MISSING=3 each, runs 3 7 8 11 12 21 24 30 32 35 38), 8 BACKUP-REFUSED (RC 24, no restore), 32 restored. Window-write sums match the record (84 uploads and 24 saves; 96, 27, 30 deletes). `WINDOW-NAME` lines: 220 (95 adds, 125 deletes; 163 upload and 57 delete), all `overlaps-snapshot-step`, 0 UNRESOLVED, 0 `BAD`. Controls in `window.txt`: C0 AGREE, C1 ORPHAN-ONLY, C2 ROW-WITHOUT-FILE, ends `WINDOW OK`. C2 consequence (404, 19 bytes; restore.sh exit 0; integrity ok; PAGE-REFS-DANGLING 0) is raw in `window.txt`. Branch two of the sentence (reproduced and named) is met, both directions seen. |
| 2 | SC2: a 0.1.0 backup restored by this release gives the same pages, same login, and the migration level the record states | VERIFIED | `amd64-t25-old/compat.txt` ends `COMPAT OK`: tag commit 4d238b9 equals `git rev-parse v0.1.0^{commit}`; `version_is_tag` PASS; backup RC 0 and the MANIFEST names v0.1.0; `RESTORE-RC: 0`; pages 8=8, users 2=2, media 12 lines equal, 47 tables equal; both logins `HTTP 303 /admin/`; `MEDIA-SERVED-EQUAL: 4 of 4`; LEVEL-BEFORE 60, LEVEL-AFTER 60; `CHECK: 17 of 17`; the tamper control proves the comparison can fail. I confirmed independently with git: 60 migrations at v0.1.0 and at e86d490, no diff under `internal/db`, and `backup.sh`, `restore.sh`, both units byte-identical between the tag and e86d490. F-7 honestly states that no migration was crossed. |
| 3 | SC3: a nearly full disk above `MIN_FREE_MB` produces a recorded outcome | VERIFIED | `capfull.txt` ends `CAPFULL OK`: floor 1024 read from the installed script, fs 1064 MB, data 1222 MB, free at guard 1064 (guard passed), rsync code 11 `No space left on device (28)`, RC 11, directory 1098915817 bytes / 4221 files, no MANIFEST. Next run: 5 MB free, `ERROR: only 5 MB free ... need 1024 MB`, RC 1. Shipped unit: `UNIT-ACTIVE-STATE: failed`, `UNIT-RESULT: exit-code`, `UNIT-EXECMAINSTATUS: 11`, 11 journal lines. `restore.sh` on the left directory: exit 1, `MANIFEST not found`, service still active, `DATA-BEFORE-RESTORE-DIRECTORIES: 0`. Control on a roomy tmpfs: COMPLETE. Sentinel intact; 66 earlier backups before and after. All numbers in the record match. |
| 4 | Bound files are unchanged since the evidence commit | VERIFIED | `git diff --quiet e86d490 HEAD -- <7 bound files>` exits 0. Files changed since e86d490 are only `25-03-SUMMARY.md`, `deploy/BACKUP-CONSISTENCY-RUN.md` and `deploy/test/README.md`. `deploy/test/out/amd64-t25-evidence-commit.txt` holds `e86d490`; `con-chain` pinned the binary to it (`BINARY: holzcloud v0.1.0-46-ge86d490`). |
| 5 | `docs/beta.md` not edited | VERIFIED | `git diff --quiet f1ba5a2 -- docs/beta.md` exits 0. |
| 6 | `nightly.sh` is additions only (removed lines vs b501b1b at most 6) | VERIFIED | `git diff --numstat b501b1b HEAD -- deploy/test/nightly.sh` gives 3664 added, 3 removed; a count of `^-` lines in the diff also gives 3. `backup.sh`, `restore.sh`, `DEPLOY.md`, both units and `run.sh` show no diff against b501b1b. No shipped file was edited (consistent with the all class 3 claim). |
| 7 | The record claims no more than was measured, and the proofs reproduce | VERIFIED | Run by me: `record-check-con` gives `RECORD-CHECK OK 27 blocks`; `record-check` gives `RECORD-CHECK OK 29 blocks`; `selftest` gives `SELFTEST OK (542 cases)`. Run counts, outcomes, restored runs (40 and 32), CON-02 60 to 60, CON-03 exit 11 and next-night refusal, durations and the pass1 start/finish stamps (13:18:54Z to 13:54:28Z) all match the raw files. The ranges, "does not show" and "Not tested" sections are conservative: they say the load is not real traffic, the tmpfs is not a disk, the unit's status-24 result was not played, and no migration was crossed. See the advisories for three small points. |

**Score:** 7/7 truths verified, 0 behavior-unverified.

### Classification of F-3 and F-5 against the shipped text

I read `deploy/backup.sh` (header lines 25-29, 56-62, set -e on line 2, mkdir on line 64, retention on line 98) and `deploy/DEPLOY.md` (lines 621-623, 633-642).

- `DEPLOY.md` does say: the pair is not atomic and a deleted file can leave a row without a file (F-1, F-2); a directory without a MANIFEST is an interrupted backup that `restore.sh` refuses (F-4 refusal); the floor does not grow with the data and "a backup can run out of space halfway" (F-4).
- `DEPLOY.md` does not say: that a vanished file (rsync 24) ends the night (F-3), or that a failed night leaves a directory which then makes the guard refuse the following nights (F-5). The record admits both ("with a documentation gap") and does not claim that the documents already cover them. So the class 3 label is correct for F-1, F-2 and F-4, and for F-3 and F-5 only in the narrower sense that no measurement contradicts a sentence in the documents. The two gaps are named in the hand-over for phase 28 and in two Owner steps; nothing is hidden or overclaimed. Whether F-3 and F-5 are defects is a design call (status 24 as success would turn a refusal into a silent missing file; pruning before the guard changes what retention is for). The record lists these as owner decisions and I find that reasonable for a verification phase. Recorded as an advisory, not a gap.

### Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| CON-01 | SATISFIED | Truth 1. The "docs/beta.md names the window" half is deferred to phase 28 by the CONTEXT decision; the window is named in the record and handed over. |
| CON-02 | SATISFIED | Truth 2. |
| CON-03 | SATISFIED | Truth 3. |

FUT-02 stays a future item: all 220 disagreements have a named window, so the condition for promoting it is not met.

Bookkeeping, not a gap: `REQUIREMENTS.md` still shows CON-01..03 as `Pending` and `ROADMAP.md` still shows the three plans unchecked. The orchestrator should update both.

### Anti-Patterns Found

No `TBD`, `FIXME` or `XXX` in `deploy/BACKUP-CONSISTENCY-RUN.md`. Zero lines starting `BAD` in `consist.txt`, `prep.txt`, `window.txt`, `capfull.txt` or `compat.txt`.

### Human Verification Required

None. The remaining open items (real disk with reserved blocks, production-sized media, a restore that crosses a migration, native timing, the unit result for status 24) are in the record's *Not tested* list and are not claimed. They belong to phases 26 and 28.

### Gaps Summary

No gaps. The three roadmap success criteria are met by evidence I recomputed from the raw files, the bound-file and `docs/beta.md` guards hold, and the three proof commands reproduce. The three advisories above are precision points and do not change the verdict.

---

_Verified: 2026-10-26_
_Verifier: Claude (gsd-verifier)_
