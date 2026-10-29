---
phase: 29-sicherung-robust
verified: 2026-10-29T12:00:00Z
status: passed
score: 9/9 must-haves verified
covered_files:
  - ".planning/phases/28-the-promise-in-writing/28-check.sh"
  - ".planning/phases/29-sicherung-robust/29-01-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-01-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-02-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-02-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-03-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-03-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-04-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-04-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-05-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-05-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-06-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-06-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-07-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-07-SUMMARY.md"
  - ".planning/phases/29-sicherung-robust/29-08-PLAN.md"
  - ".planning/phases/29-sicherung-robust/29-08-SUMMARY.md"
  - "CHANGELOG.md"
  - "deploy/BACKUP-ROBUST-RUN.md"
  - "deploy/backup.sh"
  - "deploy/test/backup-robust.sh"
  - "deploy/test/nightly.sh"
  - "docs/beta.md"
covered_digest: "v3:sha256:da0253e017ab12150596e5bb06b0b842fc0cc70547fbb71b6918919969fcceb9"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 29: Sicherung robust Verification Report

**Phase Goal:** `backup.sh` repeats an rsync exit status 24 once and aborts loudly if it recurs; a failed night removes its own directory when it has no MANIFEST (trap), so a half-written directory can no longer block later nights on a nearly full filesystem. Then a new evidence set (con-chain/nightly), new quotations in `docs/beta.md`, and the `frozen` gate of `28-check.sh` re-pinned or replaced.
**Verified:** 2026-10-29
**Status:** passed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP goal and ROB requirements) | Status | Evidence |
|---|-------|--------|----------|
| 1 | ROB-01: an rsync that ends with 24 is repeated once; a recurrence ends the night loudly with status 24; every other status is not repeated | VERIFIED | `deploy/backup.sh` `run_rsync` is the only caller of rsync (lines for media/templates/plugins/sprachen/marke loop and off-site copy both go through it): rc 0 returns, rc != 24 `exit "$rc"` at once, rc 24 prints `WARNING: rsync exited 24 ... repeating it once`, runs again, a second 24 prints `ERROR: rsync exited 24 again while copying ${what}; giving up` and exits with the repeat's status. I re-ran `bash deploy/test/backup-robust.sh` under the bash on PATH and under `/bin/bash`: both `BACKUP-ROBUST OK (94 cases)`. Real-container evidence: `VANISH: ONCE=PASS TWICE=PASS ONCE-UNIT=PASS TWICE-UNIT=PASS UNIT-TWICE-RESULT=exit-code SHIM-REMOVED=yes` in `deploy/test/out/amd64-t29-src/summary.txt`. |
| 2 | ROB-02: a night that fails before its MANIFEST removes its own directory; a directory with a MANIFEST and any earlier backup is never removed; the next night's guard is not blocked | VERIFIED | `cleanup_failed_night` is an EXIT trap set only after this run's own plain `mkdir` succeeded (`OWN_DIR`), tests `-f MANIFEST` (not `$?`), refuses any path not strictly below `BACKUP_ROOT`, never calls `exit` (original status survives). The script refuses to start if the directory of the run already exists. MANIFEST is written as `MANIFEST.partial` and renamed. Test 14 of `backup-robust.sh` plays SIGTERM during a copy; all 94 cases pass. Container evidence: `HAND-ROBUST=PASS AGAIN-NEXT-NIGHT=PASS UNIT-ROBUST=PASS`, `AGAIN-CLASS=FAILED-NO-DIR`, `FREE-AT-GUARD-MB=1064`, exit 11 kept by hand and under the unit (`UNIT-EXECMAINSTATUS=11`), 60 earlier backups unchanged. |
| 3 | ROB-03: a new evidence set (`nightly.sh con-chain` on a clean tree, evidence commit named in a new record) shows both behaviours and keeps CON-01 to CON-03 green; `record-check-con` proves the record byte for byte | VERIFIED | `summary.txt` of the set: `CON-01 PASS`, `CON-02 PASS`, `CON-03 PASS`, `STEPS: 96 played, 0 failed`, `LEAK-CHECK OK`; `amd64-t29-pass1.status` is 0. `deploy/BACKUP-ROBUST-RUN.md` has the single line `Evidence commit: dd077e4`; `dd077e4` is a commit and an ancestor of HEAD. `git diff --quiet dd077e4 HEAD -- <the seven bound files>` exits 0, so no bound file changed after the evidence commit. I ran `R=deploy/BACKUP-ROBUST-RUN.md ... RC_HANDOVER=... nightly.sh record-check-con`: `RECORD-CHECK OK 30 blocks`; the earlier record still gives `RECORD-CHECK OK 27 blocks`. Consist counters in the set show 8 real status-24 nights repeated and completed, 0 directories left. |
| 4 | ROB-04: `docs/beta.md` quotes the new record byte for byte, drops the sentences of the old record that are no longer true, keeps what the record does not show as named limits; the 0.1.1 CHANGELOG entry says the same in German | VERIFIED | `28-check.sh all`: `ok: quotes: 22 promise rows, every cited word found byte for byte`; `ok: does-not-promise: 45 limits, each with a reason, every quote found ...`; `ok: coverage: ... ROB-01, ROB-02 are in the promises`; rows P-16 to P-22 cite `deploy/BACKUP-ROBUST-RUN.md`; the limit on the left directory is rewritten to the SIGKILL/old-script case; the CHANGELOG 0.1.1 entry states in German the repeat, the removal, the renamed MANIFEST, and that the timer chain and the off-site copy were made with the earlier script. `changelog extract`, `topics` and `go` are ok. |
| 5 | ROB-05: the `frozen` gate and the evidence commands of `28-check.sh` are re-pinned to the new evidence commit, other checks pass, each changed rule has a selftest case that fails when broken | VERIFIED | `28-check.sh all` ends with `ok: frozen: ... (dd077e4) ...` and `ok: regression: nightly.sh selftest 646, record-check 29, record-check-con 27 ... and 30, backup-robust.sh 94, native-amd64.sh ...`; `28-check.sh selftest` ends `SELFTEST OK (89 cases)`. `frozen_in` takes rules A to E with fixture variants, `FROZEN_END=e54c111`, base `7d51436`; E-6 evidence commands are bound to `dd077e4`. |
| 6 | Harness (plans 29-02/03): nightly.sh asserts removal and next-night behaviour, counts repeats/left directories, and plays four VANISH scenarios with a stand-in that is removed on every exit path | VERIFIED | `bash deploy/test/nightly.sh selftest` ends `SELFTEST OK (646 cases)` (above the 542 baseline required). `SHIM-REMOVED=yes`, `DEVIATIONS:` line carries `vanish-shim`; the record names the stand-in as a deviation and the unit-under-real-vanish case as Not tested. |
| 7 | Earlier records and shipped files not touched (`restore.sh`, units, `run.sh`, `DEPLOY.md`, earlier `*-RUN.md`) | VERIFIED | `git diff --stat 7d51436 HEAD -- . ':!.planning'` lists only `CHANGELOG.md`, `deploy/BACKUP-ROBUST-RUN.md`, `deploy/backup.sh`, `deploy/test/README.md`, `deploy/test/backup-robust.sh`, `deploy/test/nightly.sh`, `docs/beta.md`; the `frozen` check agrees. |
| 8 | Owner steps OS-20 and OS-21 are in the owner's list, verbatim from the record | VERIFIED | `### OS-20` and `### OS-21` exist in `28-OWNER-STEPS.md`; `28-check.sh`: `ok: owner steps: 21 steps ... verbatim regions equal their records`, `owner steps complete`. |
| 9 | Requirements bookkeeping: ROB-01 to ROB-05 all claimed by a plan and ticked | VERIFIED | See Requirements Coverage. |

**Score:** 9/9 truths verified (0 present, behavior-unverified). The two behavior-dependent truths (1, 2: repeat and cleanup/ordering invariants) are backed by a behavioral test that passes (`backup-robust.sh`, including the SIGTERM case) and by the container set, not by symbol presence.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/backup.sh` | `run_rsync`, `cleanup_failed_night`, `OWN_DIR`, MANIFEST rename, start refusal | VERIFIED | All present, wired (trap installed after mkdir; both rsync call sites use `run_rsync`); `MIN_FREE_MB=` line and echo lines that the harness reads are intact. |
| `deploy/test/backup-robust.sh` | hermetic test ending `BACKUP-ROBUST OK (N cases)` | VERIFIED | Ran: 94 cases, both bash versions. |
| `deploy/test/nightly.sh` | verdict functions, RC_HANDOVER, VANISH scenarios | VERIFIED | selftest 646 cases passes. |
| `deploy/BACKUP-ROBUST-RUN.md` | record with `Evidence commit:` and appendix | VERIFIED | `record-check-con` 30 blocks OK. |
| `docs/beta.md`, `CHANGELOG.md` | P-16..P-22, E-6, limits; German 0.1.1 entry | VERIFIED | `28-check.sh all` OK on every sub-check. |
| `28-OWNER-STEPS.md`, `28-check.sh` | OS-20/21, frozen_in, regression pins | VERIFIED | selftest 89 cases OK. |

### Key Link Verification

| From | To | Status | Details |
|------|----|--------|---------|
| backup.sh message strings | nightly.sh transcript greps | WIRED | Verdicts `HAND-ROBUST`, `UNIT-ROBUST`, `VANISH-*` PASS in the set made on the committed script. |
| Record `Evidence commit` | git | WIRED | `dd077e4` exists, ancestor of HEAD, bound files unchanged since. |
| Record appendix | scratch files | WIRED | Byte-for-byte compare, 30 blocks. |
| Record owner steps | OS-20/OS-21 | WIRED | `cmp` of verbatim regions inside `28-check.sh`. |
| beta.md quotes | record | WIRED | every cited word found by `grep -F`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Hermetic proof of ROB-01/02 | `bash deploy/test/backup-robust.sh`; `/bin/bash deploy/test/backup-robust.sh` | `BACKUP-ROBUST OK (94 cases)` twice | PASS |
| Harness selftest | `bash deploy/test/nightly.sh selftest` | `SELFTEST OK (646 cases)` | PASS |
| Checker selftest | `bash .planning/phases/28-the-promise-in-writing/28-check.sh selftest` | `SELFTEST OK (89 cases)` | PASS |
| Checker all | `bash .planning/phases/28-the-promise-in-writing/28-check.sh all` | every line `ok:` incl. frozen, regression, beta, changelog | PASS |
| Record proof | `nightly.sh record-check-con` (new record / earlier record) | `RECORD-CHECK OK 30 blocks` / `27 blocks` | PASS |
| Bound files unchanged since evidence commit | `git diff --quiet dd077e4 HEAD -- <7 files>` | exit 0 | PASS |

No Docker rehearsal was run; the evidence set under `deploy/test/out` (gitignored scratch, present on disk) was read and is recorded in `deploy/BACKUP-ROBUST-RUN.md`.

### Probe Execution

`deploy/test/backup-robust.sh` is the phase's declared probe; it passed (above). No other `scripts/*/tests/probe-*.sh` exist.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| ROB-01 | 29-01, 29-03 | SATISFIED | Truth 1 |
| ROB-02 | 29-01, 29-02 | SATISFIED | Truth 2 |
| ROB-03 | 29-02, 29-03, 29-04, 29-05 | SATISFIED | Truth 3 |
| ROB-04 | 29-06, 29-07 | SATISFIED | Truth 4 |
| ROB-05 | 29-07, 29-08 | SATISFIED | Truth 5 |

All five IDs appear in plan frontmatter, in REQUIREMENTS.md (ticked `[x]`, traceability rows `Complete`) and in the ROADMAP phase. No ORPHANED requirement maps to Phase 29.

### Anti-Patterns Found

None. No unreferenced `TBD`, `FIXME` or `XXX` marker was found in `deploy/backup.sh`. No stub or hardcoded-empty path applies to shell/doc artifacts.

### Human Verification Required

None for the phase's own promises. Open owner items, which the plans declare as not part of the phase and which are not gaps:

- OS-20 (first night of the changed `backup.sh` on a real server) and OS-21 (the morning after the first timer night).
- The timer chain and the off-site copy on the changed script, a status 24 from a really vanishing file under the unit, a real server and a real nearly full disk. The record's *Not tested* list, the page limits and the CHANGELOG all name these as limits.

### Observations (not gaps)

- The status 24 in the four VANISH scenarios is delivered by a stand-in for rsync (named deviation `vanish-shim`); the real-vanish case is only seen in 8 of 40 consist nights. This is stated in the record and the page, so the claim is not overstated.
- `.planning/config.json` (modified) and `.planning/milestone.lock` (untracked) were not touched.

### Gaps Summary

No gaps. The shipped script does what the goal says, the hermetic test and the committed evidence set prove it, the page and the release notes quote the record, and the checker's frozen gate is pinned and exercised in its selftest.

---

_Verified: 2026-10-29_
_Verifier: Claude (gsd-verifier)_
