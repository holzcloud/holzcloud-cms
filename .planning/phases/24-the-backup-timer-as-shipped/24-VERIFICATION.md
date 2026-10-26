---
phase: 24-the-backup-timer-as-shipped
verified: 2026-10-26T13:00:00Z
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
  - deploy/test/README.md
  - deploy/test/nightly.sh
covered_digest: "v3:sha256:88f3c3a541a21a9fc26463720ee6e72f1f0eaab5bd2996444906d342933a494c"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "Real-server behaviour (wall-clock 03:15, DST, real power cycle, real network/NAS, pull-back to a rebuilt server, retention on elapsed time) is not exercised; handed over as owner steps O-1..O-5 and a Not tested list"
    category: other
    reason: "Phase 24 goal is 'as shipped, not as a container drop-in'; the container limits are named, not claimed. Owner steps O-2/O-4 are taken by Phase 26 (SRV-03), the rest by Phase 28 (DOC-02)"
    evidence_status: "named in record, not a gap"
---

# Phase 24: The backup timer as shipped - Verification Report

**Phase Goal:** The timer, the delay, the catch-up, the off-site copy and the pruning are each seen working exactly as shipped, not as a container drop-in.
**Verified:** 2026-10-26
**Status:** passed
**Re-verification:** No - initial verification

## Goal Achievement

Method: SUMMARY claims were not used. I re-ran `bash deploy/test/nightly.sh selftest` (SELFTEST OK, 220 cases) and, after copying the gitignored evidence (`amd64-t24-src`, `amd64-t24-host2`) into `deploy/test/out/`, `bash deploy/test/nightly.sh record-check` (RECORD-CHECK OK, 29 blocks, exit 0). I then read the scratch files themselves (fire, delay, catchup, prune, offsite, offsite-restore, units-shipped, host2 restore/transfer) rather than the record's quotations of them.

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Unit starts from the shipped `OnCalendar` without a drop-in; the backup it made passes the integrity check | VERIFIED | `units-shipped.txt`: installed timer and service byte-equal to `deploy/`, `DROPINPATHS` empty for both, `DROPIN-DIRS: none`, `TimersCalendar` reads `OnCalendar=*-*-* 03:15:00`. `fire.txt`: `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `EXEC-START-BEFORE: empty`, started 03:15:19 local, `CLOCK-SKEW-S-BEFORE/AFTER: 0` (time zone moved, never the clock). `verify-fire.txt`: `holzcloud check` and `sqlite3 integrity_check` ok, csrf.key present, 3/3 media. |
| 2 | A run missed while the host is down happens after it comes up; a run with the shipped 300 s delay starts inside that window | VERIFIED | `catchup.txt`: stamp 1793011250 < missed 1793011523, down 1793011375, up 1793011563, run started 157 s after timer activation by the timer, `RESULT: success`; control timer with same calendar and `Persistent=false`: `CONTROL-RAN: no` (the catch-up is attributable to `Persistent=true`). `delay.txt`: installed `RandomizedDelaySec=300`, running timer reports 300 s, 12 re-armings 31..298 s with 12 distinct values, real run drew 18 s and started 03:15:19 inside [03:15:00, 03:21:05]. |
| 3 | With `REMOTE_TARGET` set the copy arrives on a second host, and a restore from that copy gives back pages, media and login | VERIFIED | `offsite.txt`: installed unit differs from repo in exactly one line (the commented `Environment=REMOTE_TARGET=` line, `UNIT-DIFF-LINES: 1`), shipped service `Result success`, 158 files sha256-equal, same 9 dirs on both hosts. Restore on the second host with its own backup root empty before and after: `run.sh restore` exit 0, setup form 200 -> 404, `LOGIN: HTTP 303 /admin/`, media served with the source's sha256, marker page 200, `check` 17/17. Limit named: restore ran on the second host itself; the `rsync -a` pull-back to a rebuilt server is "Not tested" / O-4. |
| 4 | A directory older than the retention is deleted and a younger one stays, both in the same run | VERIFIED | `prune.txt`: 5 planted dirs (45 d, 31 d 1 h -> gone; 30 d 12 h, 29 d, 1 d -> present) in ONE invocation of the shipped service (`RETENTION-DAYS-FROM-UNIT: 30`, `Pruning backups older than 30 days...` in journal), 4 earlier dirs intact, expectation from `prune_expect` table-tested in selftest. Boundary quirk (`find -mtime +30` => deleted from 31 d) recorded as F-3, not hidden. |

**Score:** 4/4 truths verified, 0 behavior-unverified (every behavior-dependent truth - catch-up, delay window, prune - has an executed run recorded, not presence alone).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/test/nightly.sh` | fire/delay/catchup/prune/offsite/offsite-restore/chain/record-check/selftest | VERIFIED | Runs; selftest 220 cases OK; `cmd_record_check` and `chain` present (lines ~2676-2817). |
| `deploy/BACKUP-TIMER-RUN.md` | Record: summary, deviations, findings, per-requirement sections, owner steps, appendix | VERIFIED | All sections present (1400 lines); `Evidence commit: b501b1b`; record-check proves 29 appendix blocks byte for byte; no private key marker. |
| `deploy/test/README.md` | Phase 24 section | VERIFIED | Section at line 309 with rerun command and bound-file rule. |
| `deploy/DEPLOY.md` | Only a proven-defect fix | VERIFIED | `git diff --stat bf74496`: 39 insertions, 0 deletions, one hunk (credentials paragraph). Units, `backup.sh`, `restore.sh`, `run.sh` byte-identical to bf74496; no sandbox directive removed or widened. `docs/beta.md` untouched since phase 23. |

### Key Links

| From | To | Status | Details |
|------|----|--------|---------|
| `chain` | `summary.txt` | WIRED | `summary.txt`: BKP-01..05 PASS, `STEPS: 56 played, 0 failed`, `LEAK-CHECK OK`, deviations listed. |
| Record appendix | `deploy/test/out/amd64-t24-*` | WIRED | `record-check` OK 29 blocks (re-run by me). |
| Evidence commit | bound files | WIRED | `git diff --quiet b501b1b HEAD -- <7 bound files>` exits 0; `verify.commits` all valid. |

### Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| BKP-01 | SATISFIED | `fire.txt`, `units-shipped.txt`, `verify-fire.txt` |
| BKP-02 | SATISFIED | `catchup.txt` incl. Persistent=false control |
| BKP-03 | SATISFIED | `delay.txt`, `fire.txt` |
| BKP-04 | SATISFIED (second-host restore; pull-back to rebuilt server named Not tested) | `offsite.txt`, `offsite-restore.txt`, host2 `restore.txt`/`transfer.txt` |
| BKP-05 | SATISFIED | `prune.txt` |

No orphaned requirements (REQUIREMENTS.md maps only BKP-01..05 to Phase 24).

### Container limits - named, not claimed

Record carries a 13-row Deviations table (zone, control-timer, restart, ssh-packages, offsite-host, hosts, remote-target, credentials, fixture, backup-name, dropin, scp, mount) plus amd64-under-aarch64 emulation (`versions.txt`: aarch64 host / x86_64 container), a Container-vs-server table, a 9-item Not tested list, and owner steps O-1..O-5 each with command, expected output and meaning of a different output. The one inherited drop-in is on `holzcloud.service` only, never on a backup unit. Honest findings: F-2 (class 4 defect: no working credential path for `REMOTE_TARGET` under `ProtectHome=true`; reproduced with a control, fixed in docs only), F-3 prune boundary, F-4 rsync `--delete` mirror, F-6 failed copy leaves backup on box, F-8 silent failed removal.

### Probe Execution

Probes are this phase's own `nightly.sh selftest` (OK, 220 cases) and `record-check` (OK, 29 blocks). A fresh `chain` (13 min, Docker, emulated amd64) was not re-run; the evidence set was verified on its own files and against the committed text.

### Anti-Patterns Found

None. No TBD/FIXME/XXX in `nightly.sh`, the record or the README. Working tree clean apart from the gitignored `deploy/test/out/` copy I made.

### Human Verification Required

None for the phase goal. The real-server items (O-1..O-5) are explicit owner hand-overs by phase design (Phase 26 SRV-03, Phase 28 DOC-02), not unmet success criteria.

### Gaps Summary

No gaps. Housekeeping outside this verification: `REQUIREMENTS.md` still shows BKP-01..05 as `[ ]`/Pending and the ROADMAP plan checkboxes for phase 24 are unticked; the orchestrator should update them when closing the phase.

---

_Verified: 2026-10-26_
_Verifier: Claude (gsd-verifier)_
