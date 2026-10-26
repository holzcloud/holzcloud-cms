---
phase: 24-the-backup-timer-as-shipped
plan: 03
subsystem: deploy/test (backup timer chain, record, owner steps)
tags: [backup, timer, systemd, rsync, ssh, documentation, record, docker, verification]
status: complete

requires:
  - phase: 24-01
    provides: nightly.sh fire, delay, catchup
  - phase: 24-02
    provides: nightly.sh prune, offsite, offsite-restore and the credential findings
provides:
  - nightly.sh chain, down, record-check, verdict_of, req_verdict and the summary vocabulary
  - deploy/BACKUP-TIMER-RUN.md, the permanent record of phase 24 (Evidence commit b501b1b, 29 appendix blocks)
  - the credentials paragraph in the Backups section of deploy/DEPLOY.md (a documentation defect repaired)
  - the Phase 24 section of deploy/test/README.md
affects: [26, 28]

tech-stack:
  added: []
  patterns:
    - "chain stages run as child processes so set -e stays in force and each status is kept"
    - "a record is proven against its scratch files: byte-for-byte blocks, a ledger of cited files, headings, one known evidence commit"
    - "a set is made again, never patched: a wrong code comment in a bound file cost one more chain"

key-files:
  created: [deploy/BACKUP-TIMER-RUN.md]
  modified: [deploy/test/nightly.sh, deploy/test/README.md, deploy/DEPLOY.md]

key-decisions:
  - "The missing credential path is a class 4 documentation defect; repaired in the text of DEPLOY.md only, with the layout experiment B measured (key and known_hosts under /opt/holzcloud/offsite-ssh, a Host block in /etc/ssh/ssh_config.d); no sandbox directive was touched"
  - "The prune boundary (from 31 days of age), the rsync --delete mirror, rsync only suggesting an ssh client and the silent removal in backup.sh are recorded as class 3, not edited"
  - "Chain stages are child processes of nightly.sh, not subshells in a ( ) || list, because set -e is off inside the latter"

requirements-completed: [BKP-01, BKP-02, BKP-03, BKP-04, BKP-05]

metrics:
  duration: "about 1 hour 45 minutes wall clock; three chains of 13 minutes each"
  completed: 2026-10-26

actuals:
  tokens: 38800
  tasks: 3
  commits: 5
plan_head_before: 47c046480fdb847a203533ef75f688eedf212a14
plan_head_after: f05ae09d398015f59470aac3b663479ca0499c32
commits: 5
---

# Phase 24 Plan 03: chain, evidence set and the record Summary

**`nightly.sh chain` plays the whole phase 24 proof from fresh containers in about 13 minutes and writes `summary.txt` with BKP-01 to BKP-05 all PASS, `STEPS: 56 played, 0 failed` and `LEAK-CHECK OK`; `deploy/BACKUP-TIMER-RUN.md` records it with the owner steps, and `record-check` proves its 29 appendix blocks byte for byte. Two harness problems and one documentation defect were found by the first chain and repaired.**

## Result

| Requirement | Result (final set, `deploy/test/out/amd64-t24-src/summary.txt`, evidence commit b501b1b) |
|---|---|
| BKP-01 | PASS. `TRIGGERED-BY=holzcloud-backup.timer RESULT=success START-LOCAL=03:15:19` |
| BKP-02 | PASS. `TRIGGERED-BY=holzcloud-backup.timer AFTER-BOOT-S=157 CONTROL-RAN=no` |
| BKP-03 | PASS. `USED-S=18 SAMPLES=12 MIN-S=31 MAX-S=298` |
| BKP-04 | PASS. `WORKING-CREDENTIALS=offsite-ssh FILES-COMPARED=158 UNIT-DIFF-LINES=1`; restore `LOGIN=HTTP 303 /admin/`, media served, 10.858 s |
| BKP-05 | PASS. `PLANTED=5 DELETED=2 KEPT=3` |

The final chain ran 2026-10-26 10:37:17Z to 10:50:49Z, exit 0. The record's `Evidence commit: b501b1b`; `git diff --quiet b501b1b HEAD -- <bound files>` exits 0. `bash deploy/test/nightly.sh record-check` prints `RECORD-CHECK OK 29 blocks`. `docs/beta.md` is unchanged (`git diff --quiet f1ba5a2 -- docs/beta.md` exits 0).

## What was built

- `deploy/test/nightly.sh` (bash 3.2): `verdict_of`, `file_clean`, `req_verdict`, `kvq`, `steps_of`, `record_check` with selftest tables (220 cases now; failing cases included: altered scratch file, cited file without a block, missing heading, malformed, unknown, absent and doubled Evidence commit, a private key marker, a heading without a scratch file or without a block), `cmd_down`, `cmd_chain`, `cmd_record_check`, dispatch and usage.
- `deploy/BACKUP-TIMER-RUN.md`, `deploy/test/README.md` Phase 24 section, and the credentials paragraph in `deploy/DEPLOY.md`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug in the harness, finding F-1] The staged test image is gone after the catch-up restart**
- **Found during:** task 2, pass 1. `offsite-restore` ended `BAD  /tmp/hc-upload.jpg could not be carried from holzcloud-t24-src to holzcloud-t24-host2`, `BKP-04 FAIL (the stage ended with status 1)`.
- **Cause:** `catchup` stops and starts the source container and `/tmp` is emptied at boot by the tmpfiles rule `D /tmp 1777 root root -` (Debian 12, no mount of its own).
- **Fix:** `catchup` keeps the file under `/root` for the stop and puts it back, with KEY lines `TEST-IMAGE-*` and `TMP-AT-BOOT`. **Commits:** 6a27bea, b501b1b.
- **Note:** the message and comment of 6a27bea named a tmpfs, which was wrong; b501b1b corrects the comment and puts the cause into the evidence. Because nightly.sh is a bound file, the chain run on 7baa09d (green) was thrown away and made again on b501b1b; the record says so.

**2. [Rule 2 - Class 4 documentation defect, finding F-2] No working credential path for `REMOTE_TARGET` in DEPLOY.md**
- **Issue:** the document and the unit comment name no credential; the key in the service user's home fails under `ProtectHome=true` (`ExecMainStatus 255`, `Host key verification failed`).
- **Fix:** a paragraph in the Backups section of `deploy/DEPLOY.md` after the retention paragraph (39 added lines, no removal): the measured layout (`/opt/holzcloud/offsite-ssh`, `/etc/ssh/ssh_config.d/holzcloud-backup.conf`), the failure signature, the ssh client that `rsync` only suggests, the `--delete` mirror. Backups block 1, which `run.sh` plays, is unchanged; no sandbox directive was removed or widened (`git diff bf74496 -- deploy/holzcloud-backup.service deploy/holzcloud-backup.timer` is empty). **Commit:** 7baa09d. Pass 1's failing output was kept as `amd64-t24-pass1-*` and quoted in the record.
- **Files differing from bf74496 now:** `deploy/DEPLOY.md` only (`git diff --stat bf74496 -- <bound files>`: 1 file, 39 insertions).

**3. [Plan wording] Stages are child processes, not `( cmd_x ) || sc_x=$?` subshells**
- A subshell on the left of `||` runs with `set -e` off for its whole body, so a stage could go on after a failed command. `chain` runs `bash nightly.sh STAGE` for each stage, which keeps `set -e` and the status. The semantics asked for (status kept, nothing hidden) are the same.

**4. [Plan wording] `leak_check` runs in a subshell in `chain`**
- A hit gives `LEAK-CHECK FAILED` in the summary and exit 1 after the summary, instead of dying before it.

**5. [Plan wording] The appendix has one more block than the plan's list**
- `amd64-t24-src/offsite-journal-b.txt` was added so the off-site service run time in *Measurements* cites pasted output.

### Auth gates

None.

## Chains run

| Chain | Evidence commit | Started to ended (host clock) | Result |
|---|---|---|---|
| Pass 1 | cd0d610 | 10:06:09Z to 10:18:52Z | `BKP-04 FAIL`; kept as `amd64-t24-pass1-*`, quoted in the record |
| Superseded | 7baa09d | 10:22:07Z to 10:35:26Z | all PASS; thrown away (wrong code comment in a bound file); kept as `amd64-t24-superseded-*`, not pasted |
| Final | b501b1b | 10:37:17Z to 10:50:49Z | all PASS, exit 0; the set the record pastes |

## Known Stubs

None.

## Threat Flags

None beyond the register. T-24-13 held: `record-check` proves every block, the ledger, headings and the evidence commit. T-24-14 held: no sandbox directive was touched. T-24-15 held: `leak_check` over both directories reports `LEAK-CHECK OK`, `record-check` found no key marker, and `step-NN.txt`, `seed.txt`, logs and key files are not pasted. T-24-16 held: one evidence commit, the diff over the bound files exits 0. T-24-17 held: `docs/beta.md` unchanged.

## Evidence locations

- Worktree (gitignored): `deploy/test/out/amd64-t24-src/` and `amd64-t24-host2/` (final), `amd64-t24-pass1-*`, `amd64-t24-superseded-*`, logs `amd64-t24-chain.log`, `amd64-t24-chain1.log`, `amd64-t24-superseded-chain.log`.
- Merged into `/private/tmp/claude-501/-Users-holz-Projects-holzcloud-cms/295eb7a6-636f-4c10-8442-a5a0771f92c4/scratchpad/phase24-out/` (earlier content kept).
- All `holzcloud-t24*` containers were removed by the chain.

## Self-Check: PASSED

- `deploy/BACKUP-TIMER-RUN.md`, `deploy/test/nightly.sh`, `deploy/test/README.md` exist; `bash -n` and `selftest` pass (220 cases).
- Commits cd0d610, 6a27bea, 7baa09d, b501b1b and f05ae09 are on HEAD (`git rev-list --count 47c0464..f05ae09` = 5).
- `summary.txt` holds `BKP-01 PASS` to `BKP-05 PASS`, `STEPS: 56 played, 0 failed`, `LEAK-CHECK OK`; `record-check` prints `RECORD-CHECK OK 29 blocks`.
- `git diff --quiet b501b1b HEAD -- <bound files>` and `git diff --quiet f1ba5a2 -- docs/beta.md` exit 0.
