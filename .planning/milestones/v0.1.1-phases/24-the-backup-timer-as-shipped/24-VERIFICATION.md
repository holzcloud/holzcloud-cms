---
phase: 24-the-backup-timer-as-shipped
verified: 2026-10-31T12:00:00Z
status: passed
score: 4/4 roadmap success criteria verified (BKP-01..BKP-05 all satisfied within named container limits)
covered_files:
  - .planning/phases/24-the-backup-timer-as-shipped/24-01-PLAN.md
  - .planning/phases/24-the-backup-timer-as-shipped/24-01-SUMMARY.md
  - .planning/phases/24-the-backup-timer-as-shipped/24-02-PLAN.md
  - .planning/phases/24-the-backup-timer-as-shipped/24-02-SUMMARY.md
  - .planning/phases/24-the-backup-timer-as-shipped/24-03-PLAN.md
  - .planning/phases/24-the-backup-timer-as-shipped/24-03-SUMMARY.md
  - deploy/BACKUP-TIMER-RUN.md
  - deploy/DEPLOY.md
  - deploy/backup.sh
  - deploy/holzcloud-backup.service
  - deploy/holzcloud-backup.timer
  - deploy/restore.sh
  - deploy/test/README.md
  - deploy/test/nightly.sh
covered_digest: "v4:sha256:7b47c06e00254caa4f578fdaf1cfe791737e4cf1f69c0e514dcb5ae74f88202f"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  gaps_closed: []
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "WR-02 (24-REVIEW): nightly.sh chain does not count the status of run.sh backup nor the failed-step count (sf) in its exit code, so a failed run.sh backup would not turn BKP-04 FAIL. Latent harness hardening gap. In the recorded evidence set the backup did succeed (steps 34/35 rc=0, STEPS 56 played 0 failed, backup.txt complete) and the BKP-04 evidence does not depend on it"
    category: other
    reason: "Fix is to count sc_backup and sf in `fails` in cmd_chain (nightly.sh ~7005-7008); would need a fresh chain only if the record's evidence is re-made"
    evidence_status: "reproduced by reading the code; actual evidence inspected and clean"
  - finding: "WR-01 (24-REVIEW): pure verdict functions fail open on empty/non-numeric input (confirmed: `prune_expect abc 30` prints KEEP, rc 0). Affects hand-edited or truncated KEY files, not the recorded set, whose prune.txt/fire.txt/catchup.txt hold real numeric values and observed gone/present lines"
    category: other
    reason: "Harness hardening; add a whole-number guard and selftest cases"
    evidence_status: "reproduced"
  - finding: "deploy/backup.sh changed after the phase 24 evidence commit b501b1b (phase 29: rsync status-24 repeat via run_rsync, cleanup of a failed night, MANIFEST rename). Phase 24 measured the earlier text. Step 5 (find -mtime prune) is byte-unchanged; the REMOTE_TARGET rsync keeps its arguments and only gains a repeat on status 24. BACKUP-TIMER-RUN.md line 282 ('backup.sh byte-identical to bf74496') is true of the evidence commit, not of HEAD. The record itself says a bound-file change means the set is made again before the record is cited for the changed text"
    category: other
    reason: "Phase 28 (DOC-02) must cite the record only for text that still matches, or re-run `nightly.sh chain`; the phase 24 goal was achieved against the text shipped at the time"
    evidence_status: "git diff b501b1b HEAD -- deploy/backup.sh read in full; not re-measured in a container"
  - finding: "Real-server behaviour (wall-clock 03:15, DST, real power cycle, real network/NAS, pull-back to a rebuilt server, retention on elapsed time) is not exercised; handed over as owner steps O-1..O-5 and a Not tested list"
    category: other
    reason: "Phase 24 goal is 'as shipped, not as a container drop-in'; container limits are named, not claimed. O-2/O-4 go to phase 26 (SRV-03), the rest to phase 28 (DOC-02)"
    evidence_status: "named in record, not a gap"
---

# Phase 24: The backup timer as shipped - Verification Report

**Phase Goal:** The timer, the delay, the catch-up, the off-site copy and the pruning are each seen working exactly as shipped, not as a container drop-in.
**Verified:** 2026-10-31
**Status:** passed
**Re-verification:** Yes - regeneration of a stale report (the earlier report was `passed`; it went stale because `deploy/test/README.md` and `deploy/test/nightly.sh` changed in phase 29, and `deploy/backup.sh` did too). All truths were re-checked against the evidence files and the current code, not against the earlier report or the SUMMARYs.

## Method

- `bash deploy/test/nightly.sh selftest` re-run: `SELFTEST OK (646 cases)` (was 220 at phase 24; later phases added cases, none of the phase 24 ones fail).
- `bash deploy/test/nightly.sh record-check` re-run against the gitignored evidence `deploy/test/out/amd64-t24-src` and `amd64-t24-host2`: `RECORD-CHECK OK 29 blocks`, exit 0 (the record's appendix is byte-equal to the scratch files).
- Read the scratch evidence directly (`fire.txt`, `delay.txt`, `catchup.txt`, `prune.txt`, `offsite.txt`, `offsite-restore.txt`, `backup.txt`, `summary.txt`, host2 `restore.txt`/`transfer.txt`) rather than the record's quotations.
- Diffed the shipped files against both `bf74496` (phase 23 baseline) and the evidence commit `b501b1b`.
- A fresh `chain` (13 min, Docker, emulated amd64) was not re-run.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The unit starts from the shipped `OnCalendar` without a drop-in, and the backup it made passes the integrity check | VERIFIED | `fire.txt`: `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `EXIT-STATUS: 0`, `EXEC-START-BEFORE: empty`, started `03:15:19` local in a moved zone (`CLOCK-SKEW-S-BEFORE/AFTER: 0`, so zone moved, never the clock), journal names the directory, `DIR-OWNER-MODE: holzcloud 700`, `FIRE OK`. `units-shipped.txt`/`verify-fire.txt`: installed units byte-equal to `deploy/`, no drop-in, `holzcloud check` and `sqlite3 integrity_check` ok (summary: `UNITS-AND-BACKUP: no BAD line`). |
| 2 | A run missed while the host is down happens after it comes up, and a run with the shipped 300 s delay starts inside that window | VERIFIED | `catchup.txt`: stamp < missed instant, container stopped across it, run started by the timer 157 s after activation, `Result success`; control timer with the same calendar and `Persistent=false`: `CONTROL-RAN: no` (catch-up attributable to `Persistent=true`). `delay.txt`: `RandomizedDelaySec=300` installed and reported by the running timer, 12 re-armings 31..298 s with 12 distinct values, real run drew 18 s, started `03:15:19` inside the window. |
| 3 | With `REMOTE_TARGET` set the copy arrives on a second host, and a restore from that copy gives back pages, media and login | VERIFIED | `offsite.txt`: installed service differs from the repository file in exactly one line (the commented `Environment=REMOTE_TARGET=` line), shipped service `Result success`, 158 files sha256-equal, same 9 directories on both hosts, modes 700 owned by the unprivileged account. Credential finding measured with a control (key under `/home` fails under `ProtectHome=true`, same script succeeds outside the sandbox; key under `/opt/holzcloud/offsite-ssh` works with no sandbox change). `offsite-restore.txt` + host2 `restore.txt`/`transfer.txt`: restore played from `/backups/holzcloud/20261026-105004` (the unit's copy, `TRANSFER: equal`, `VIA: rsync by the shipped unit, no docker cp`), setup form open then 404, login 303 to `/admin/`, media served with the source's sha256, marker page 200, `check` 17 of 17, `OFFSITE-RESTORE OK`. Limit named: restore ran on the second host itself; the `rsync -a` pull-back to a rebuilt server is "Not tested" / O-4. |
| 4 | A directory older than the retention is deleted and a younger one stays, both in the same run | VERIFIED | `prune.txt`: 5 planted directories (45 d and 31 d 1 h gone; 30 d 12 h, 29 d, 1 d present) in ONE run of the shipped service, `PLANTED=5 DELETED=2 KEPT=3`, expectation from `prune_expect`, earlier backups intact. The `find -mtime +30` boundary (deleted from 31 d) is recorded as a measured property (F-3), not hidden. `backup.sh` step 5 is still byte-identical to the measured text at HEAD. |

**Score:** 4/4 truths verified, 0 behavior-unverified. Every behavior-dependent truth (catch-up, delay window, prune, off-site copy and restore) rests on an executed run recorded in the scratch files, not on symbol presence.

### BKP-04 and review finding WR-02 (the point asked for)

WR-02 is real: in `cmd_chain` (`deploy/test/nightly.sh` ~6960 and ~7005-7008) `sc_backup` is stored and printed but never enters `fails`, and the failed-step count `sf` is only printed. `offsite` and `offsite-restore` check that the files the `run.sh backup` stage writes exist, not that the stage succeeded. So the chain's verdict function could print `BKP-04 PASS` after a failed `run.sh backup`.

It does not make BKP-04 unproven here, for three reasons I checked in the evidence rather than assumed:

1. The evidence set this verdict rests on has a good `run.sh backup`: `steps.txt` shows STEP 34 and 35 (`backup.sh` as the service user) `rc=0`, `backup-run.txt` ends `=== Backup complete: /opt/holzcloud/backups/20261026-104928 ===`, `backup.txt` holds a complete record (18 files, 3 media files, media served HTTP 200), and `summary.txt` says `STEPS: 56 played, 0 failed`.
2. The BKP-04 proof is not that stage. It is the shipped `holzcloud-backup.service` run with `REMOTE_TARGET` set (`Result success`, `ExecMainStatus 0`), a sha256 comparison of 158 files on two hosts, and a restore from the copy on the second host with 17 of 17 `check` assertions, login 303 and media hash equal. A failed `run.sh backup` would have shown up as a failed restore or a hash mismatch there.
3. No BAD line exists in any phase 24 stage file of either evidence directory (the only `BAD` hits are inside the harness's own selftest outputs, which are expected).

Judgement: the WR-02 gap is a latent harness weakness (a future run could be reported green after a failed stage), recorded as an advisory with its fix, not a failure of the BKP-04 requirement.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/test/nightly.sh` | fire/delay/catchup/prune/offsite/offsite-restore/chain/record-check/selftest | VERIFIED | Present, selftest OK (646), `cmd_chain`, `cmd_record_check` wired; grew in later phases (25, 26, 29) without breaking phase 24 stages. |
| `deploy/BACKUP-TIMER-RUN.md` | Record with summary, deviations, findings, per-requirement sections, owner steps, appendix | VERIFIED | 1400 lines, `Evidence commit: b501b1b`, `record-check` OK 29 blocks. |
| `deploy/test/README.md` | Phase 24 section | VERIFIED | `## Phase 24: the nightly run as shipped` at line 310, rerun command and bound-file rule. Known doc slip IN-01 (names `BACKUP-RUN.md` instead of `BACKUP-TIMER-RUN.md` for `record-check`), cosmetic. |
| `deploy/DEPLOY.md` | Only a proven-defect fix | VERIFIED | `git diff --stat bf74496`: 39 insertions, 0 deletions; unchanged since `b501b1b`. |
| Shipped units, `restore.sh`, `run.sh` | Not edited | VERIFIED | Both units, `restore.sh`, `run.sh` byte-identical to `bf74496`; units and `restore.sh` also unchanged since `b501b1b`. `docs/beta.md` was only added to by later phases, not by phase 24. |
| `deploy/backup.sh` | Not edited by phase 24 | VERIFIED for phase 24; DRIFTED afterwards | Identical to `bf74496` at the evidence commit. Changed by phase 29 (see advisory 3). Prune block untouched. |

### Key Link Verification

| From | To | Status | Details |
|------|----|--------|---------|
| `chain` | `summary.txt` | WIRED | `summary.txt`: BKP-01..05 PASS, TIMER/DELAY/CATCHUP/PRUNE/OFFSITE/RESTORE lines, `STEPS: 56 played, 0 failed`, `LEAK-CHECK OK`. Wiring gap WR-02 noted above. |
| Record appendix | `deploy/test/out/amd64-t24-*` | WIRED | `record-check` OK 29 blocks, re-run by me. |
| `cmd_offsite_restore` | `run.sh restore` | WIRED | `transfer.txt: TRANSFER: equal`, restore from the unit's copy path. |
| `cmd_prune` | `backup.sh` step 5 | WIRED | `prune_expect` verdicts match observed gone/present. |

### Data-Flow Trace (Level 4)

Not applicable (no rendered dynamic data). The analogous question, whether recorded verdicts come from observed state or from asserted state, was checked: `prune.txt` lists observed presence from the filesystem, `offsite.txt` compares sha256 lists from both hosts, `catchup.txt` records container clock and `docker inspect` times. All FLOWING from real observations.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Pure verdict functions and table cases | `bash deploy/test/nightly.sh selftest` | `SELFTEST OK (646 cases)` | PASS |
| Record equals evidence | `bash deploy/test/nightly.sh record-check` | `RECORD-CHECK OK 29 blocks` | PASS |
| `prune_expect` on non-numeric input (WR-01) | sourced function, `prune_expect abc 30` | prints `KEEP`, rc 0 | CONFIRMED fail-open (advisory) |

### Probe Execution

The phase's probes are its own `selftest` and `record-check` (both PASS above). No `scripts/*/tests/probe-*.sh` is declared for this phase.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| BKP-01 | 24-01, 24-03 | Timer fires from shipped 03:15 entry, no drop-in; record shows the unit starting and its backup | SATISFIED | `fire.txt`, `units-shipped.txt`, `verify-fire.txt` |
| BKP-02 | 24-01, 24-03 | Missed run caught up via `Persistent=true` | SATISFIED | `catchup.txt` incl. `Persistent=false` control |
| BKP-03 | 24-01, 24-03 | `RandomizedDelaySec` shipped 300 s, delay inside window | SATISFIED | `delay.txt`, `fire.txt` |
| BKP-04 | 24-02, 24-03 | `REMOTE_TARGET` copy runs and a restore from the copy brings the site back | SATISFIED (second-host restore; pull-back to rebuilt server named Not tested, O-4) | `offsite.txt`, `offsite-restore.txt`, host2 `restore.txt`, `transfer.txt` |
| BKP-05 | 24-02, 24-03 | Retention deletes older, keeps younger, both in a record | SATISFIED | `prune.txt` |

All five IDs appear in PLAN frontmatter (`24-01`: BKP-01..03, `24-02`: BKP-04..05, `24-03`: BKP-01..05) and in REQUIREMENTS.md, where all five are ticked and mapped to Phase 24 as Complete. No orphaned requirements (REQUIREMENTS.md maps only BKP-01..05 to Phase 24).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `deploy/test/nightly.sh` | 6960, 7005-7008 | chain ignores `sc_backup` and `sf` (WR-02) | Warning | Latent false green in a future run; this evidence set clean |
| `deploy/test/nightly.sh` | 257-278, 322-347 | verdict functions fail open on non-numeric input (WR-01) | Warning | Hand-edited/truncated KEY files only |
| `deploy/test/nightly.sh` | 1518, 1556 | `fire` judges the run only where the clock is stepped (WR-03) | Warning | False FAIL (not false PASS) on a native host; belongs to phase 26 |
| `deploy/test/nightly.sh` | 55, 138-142 | knobs not validated (WR-04) | Warning | Confusing aborts, not wrong verdicts |

No `TBD`/`FIXME`/`XXX` in `nightly.sh`, `BACKUP-TIMER-RUN.md`, `README.md` or `DEPLOY.md`. The review's eight info items (README names the wrong record, undocumented deviation kinds and `OBSERVE_S`, trap/cleanup hygiene) are hygiene, none changes a verdict.

### Human Verification Required

None for the phase goal. The real-server items (O-1..O-5) are explicit owner hand-overs by phase design (Phase 26 SRV-03, Phase 28 DOC-02), not unmet success criteria.

### Gaps Summary

No gaps. Phase goal achieved: each of the timer, the delay, the catch-up, the off-site copy and the pruning was seen working through the shipped units with no drop-in, with the container limits named instead of claimed.

Items to carry forward (none blocks):
1. Harden `cmd_chain` (WR-02) and the pure functions (WR-01) before the next evidence set is made.
2. Because `deploy/backup.sh` was changed by phase 29 after evidence commit `b501b1b`, phase 28 must either re-run `nightly.sh chain` or cite the phase 24 record only for text that still matches. The prune and `REMOTE_TARGET` code paths phase 24 measured are behaviorally unchanged by diff (retention block untouched; the copy only gains a single repeat on rsync status 24), but I did not re-measure them in a container.

---

_Verified: 2026-10-31_
_Verifier: Claude (gsd-verifier)_
