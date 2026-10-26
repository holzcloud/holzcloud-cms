---
phase: quick-261026-ctw
plan: 01
subsystem: docs
tags: [beta, wording, audit-warnings]
status: complete
key-files:
  modified: [docs/beta.md]
---

# Quick 261026-ctw: beta.md wording — reading vs fact, Debian 13, nearly full filesystem

Three wording defects of the phase 28 verifier and the v0.1.1 milestone audit (W4, W1, W2) fixed in `docs/beta.md`; no promise row, no section, no `Why:` and no checker was touched.

## Changes

- **W4** (limit "A file that vanishes during the media copy"): the directory of a status-24 night staying without a `MANIFEST` is now stated as a reading of the script and of the transcript, with no listing taken, the wording of `deploy/BACKUP-CONSISTENCY-RUN.md` F-3. That `rsync` ends with status 24 and writes no `MANIFEST` stays a fact (the record's raw transcript shows it).
- **W1** (line 9): the record `deploy/SECURITY-SCORE.md` (lines 14 and 40) shows the systemd 257 half of P-3 ran in a Debian 13 (trixie) container; line 9 now says so as an exception to "Debian 12". Lines 65 ("the install ran on Debian 12 only", DEPLOY-RUN.md is Debian 12 only) and 81 ("Debian 12 on real hardware was not measured") are true as written and unchanged.
- **W2** (limit "A failed night leaves its directory" and the DEPLOY.md-gaps limit): the guard refusal is now said to happen on a nearly full filesystem, the only case observed.

## Verification

`28-check.sh` slices beta, owner-steps, changelog, frozen: exit 0; selftest: `SELFTEST OK (59 cases)`.

## Deviations from Plan

None. The worktree HEAD (ee3c9ed, three PR-47 commits not in main) was reset to main HEAD d8381d2 as instructed, the tree being clean.

## Self-Check: PASSED
