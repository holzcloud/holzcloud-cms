---
phase: 22-backup-and-restore-measured
verified: 2026-10-14T13:00:00Z
status: passed
score: 3/3 roadmap success criteria verified (plus 5/5 plan-level truth groups)
covered_files:
  - ".planning/phases/22-backup-and-restore-measured/22-01-PLAN.md"
  - ".planning/phases/22-backup-and-restore-measured/22-01-SUMMARY.md"
  - ".planning/phases/22-backup-and-restore-measured/22-02-PLAN.md"
  - ".planning/phases/22-backup-and-restore-measured/22-02-SUMMARY.md"
  - ".planning/phases/22-backup-and-restore-measured/22-03-PLAN.md"
  - ".planning/phases/22-backup-and-restore-measured/22-03-SUMMARY.md"
  - ".planning/phases/22-backup-and-restore-measured/22-04-PLAN.md"
  - ".planning/phases/22-backup-and-restore-measured/22-04-SUMMARY.md"
  - ".planning/phases/22-backup-and-restore-measured/22-05-PLAN.md"
  - ".planning/phases/22-backup-and-restore-measured/22-05-SUMMARY.md"
  - "deploy/BACKUP-RUN.md"
  - "deploy/DEPLOY.md"
  - "deploy/backup.sh"
  - "deploy/holzcloud-backup.service"
  - "deploy/holzcloud-backup.timer"
  - "deploy/restore.sh"
  - "deploy/test/README.md"
  - "deploy/test/run.sh"
  - "docs/configuration.md"
  - "docs/deployment.md"
covered_digest: "v3:sha256:843506268da47e0e6422c648e80b2f62ce812c54cb31a97271dee0a6789a32ab"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 22: Backup and restore, measured - Verification Report

**Phase Goal:** A backup that has been restored is the only backup; prove it.
**Verified:** 2026-10-14
**Status:** passed
**Re-verification:** No - initial verification

Method note: SUMMARY.md claims were not taken as evidence. The verification read the production scripts (`deploy/backup.sh`, `deploy/restore.sh`), the raw scratch output of the final evidence set (`deploy/test/out/amd64-bak-*`, gitignored but present), the record `deploy/BACKUP-RUN.md`, and re-ran a byte comparison of the record's appendix against the scratch files. Nothing was re-run in containers (that requires Docker and about ten minutes of emulated amd64; the evidence set is the artefact under test and its integrity was checked instead).

## Goal Achievement

### Observable Truths (ROADMAP success criteria, the contract)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `backup.sh` output passes its integrity check and contains database and media | VERIFIED | `amd64-bak-src/verify-backup.txt` (manual) and `verify-timer.txt` (timer): `PASS integrity holzcloud-check: ok`, `PASS integrity sqlite3 integrity_check: ok` (immutable URI, a second, stronger reader), `PASS file .../holzcloud.sqlite exists`, MANIFEST present, `PASS tree src-backup: 16 files equal` (whole-directory sha256 of source against backup, media included: `MEDIA-FILES: 3`, `MEDIA-BYTES: 39983`), 47 `PASS rows` lines, no BAD line outside the deliberate selftest. The script itself runs `holzcloud backup` (VACUUM INTO plus verify, non-zero on failure) and rsyncs `media templates plugins sprachen marke` (`backup.sh` lines 69, 80-85). The comparison was shown able to fail (`selftest-tree.txt`, `compare-selftest.txt` carry the expected BAD lines). |
| 2 | `restore.sh` onto an empty system yields working pages, media and login; time and size recorded | VERIFIED | Target is a second, fresh `systemd-base` container (not the source). `amd64-bak-tgt/restore.txt`: `SETUP-BEFORE: HTTP 200` then `SETUP-AFTER: HTTP 404`; `LOGIN: HTTP 303 /admin/` with the old password; `PASS marker page ... 200`; `PASS media served over HTTP: same status and sha256` (`925d26e1...`); `PASS tree src-restored: 16 files equal`; `PASS state comparison source against target: 53 pass, 0 bad`; `PASS check: 17 of 17 assertions` (five plugins running). Time and size: `WALL-S: 8.126`, `VERIFY-S 0.083`, `STOP-MOVE-COPY-S 0.455`, `START-READY-S 7.397`, `BACKUP-BYTES 20548955`, `DB-BYTES 663552`, `MEDIA-BYTES 39983`; second point with 300 MiB bulk: `BACKUP-BYTES=335125851`, `RESTORE-WALL-S=8.296` (`summary.txt`). Four refusals played with eight live-data invariants unchanged (`restore-negative.txt`, `RESTORE-NEGATIVE OK`). Emulation (aarch64 host, x86_64 container) is named next to the numbers. |
| 3 | One run through `holzcloud-backup.timer` is observed | VERIFIED | `amd64-bak-src/timer.txt`: `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `EXIT-STATUS: 0`, `LAST-BEFORE: n/a` then `LAST-AFTER` set, `NEXT` moved a day on, new directory `holzcloud 700` that is the one the journal names. `timer-journal.txt` shows the hardened service (`holzcloud-backup.service`) walking database, key, media, plugins, sprachen, marke and `=== Backup complete ===`. The backup the timer made is the one that was transferred and restored in criterion 2 (`DIR: /opt/holzcloud/backups/20261014-110152` in `restore.txt`). The container-only drop-in is named DEVIATION[timer]; shipped unit files are unchanged. |

**Score:** 3/3 roadmap truths verified; 0 present-but-behavior-unverified.

### Plan-level must-haves (merged; none reduce the roadmap contract)

| Plan | Truth | Status | Evidence |
|------|-------|--------|----------|
| 22-01 | Two containers, tar-stream transfer, `restore.sh` played as the document says, no `--yes` flag | VERIFIED | `restore-run.txt` and `transfer.txt` exist for both points; `restore.sh` still reads `read -r -p ... [yes/NO]` (line 79); no `--yes` in either script. |
| 22-02 | Timer, negative cases, fixture and `backup-chain` with fixed summary vocabulary | VERIFIED | `summary.txt` holds `BAK-01/02/03 PASS`, `RESTORE-NEGATIVE PASS`, two POINT lines, `TIMER:`, `STEPS: 78 played, 0 failed`, `DEVIATIONS: backup-name dropin fixture mount scp timer transfer`, `LEAK-CHECK OK`. |
| 22-03 | Pass 1 on unchanged scripts with every finding decided | VERIFIED | `22-FINDINGS.md` plus `deploy/BACKUP-RUN.md` Findings: F-01 to F-11, each with step, quote, fix. Pass-1 scratch directories (`amd64-bak-pass1-*`) exist. |
| 22-04 | Repairs from findings, both scripts carry the five directories, docs aligned, final set from the committed texts | VERIFIED | Both scripts loop over `media templates plugins sprachen marke`; restore refuses a backup without MANIFEST (line 60) and keeps verify-then-ask-then-stop order; `amd64-bak-evidence-commit.txt` = `bf74496`; `git diff bf74496 HEAD` shows no change to `backup.sh`, `restore.sh`, `DEPLOY.md`, the unit files or `run.sh` since the evidence set. No Go files changed in the phase. |
| 22-05 | `BACKUP-RUN.md` complete, appendix byte-proven, evidence commit named, README hand-over | VERIFIED | All required sections present in order (Summary, How this was run, Deviations named, Findings, BAK-01..03, Measurements, What comes back, Container vs. server, Not tested, Appendix). `Evidence commit: bf74496` on line 15. Independent check performed here: 40 appendix blocks compared against `deploy/test/out/<name>`, 40 equal, 0 different. `deploy/test/README.md` has `## Hand-over to phase 23`. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/backup.sh` | Carries whole directory set, limits stated | VERIFIED | 113 lines, `sprachen`/`marke`/`plugins` loop, header states what is and is not held and non-atomicity. `bash -n` ok. |
| `deploy/restore.sh` | Restores same set, verify-first | VERIFIED | 127 lines, same loop, MANIFEST gate, snapshot check on a copy before `systemctl stop`. `bash -n` ok. |
| `deploy/DEPLOY.md` | Backups and Restore sections that match what happened | VERIFIED | Sections exist and are played by the harness (`doc_play`); no angle-bracket placeholder in the played block per the 78 played steps. |
| `deploy/test/run.sh` | backup, transfer, restore, backup-timer, restore-negative, backup-fixture, backup-chain | VERIFIED | +1436 lines vs phase start; `bash -n` ok; produced the evidence set. |
| `deploy/BACKUP-RUN.md` | Permanent record | VERIFIED | 1497 lines, structure and appendix proven above. |
| `deploy/test/README.md` | Pointer and hand-over | VERIFIED | Hand-over section present; the README sentence about a DEVIATION name check that F-11 flagged no longer exists. |
| `docs/deployment.md`, `docs/configuration.md` | No false "whole directory" claim; tree lists plugins/ and marke/ | VERIFIED | Both changed in the phase; the code review judged them accurate against the code. |
| `deploy/test/out/amd64-bak-evidence-commit.txt` | Commit of the final set | VERIFIED | `bf74496`, equals the line in `BACKUP-RUN.md`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `backup.sh` | `restore.sh` | Same five directory names | WIRED | Identical `for name in media templates plugins sprachen marke` in both. |
| `DEPLOY.md` Restore block | `run.sh cmd_restore` | `doc_play` reads the block, restore line translated under DEVIATION[backup-name] | WIRED | `restore-run.txt` transcript and `scenario.txt` present; no host-side substitution beyond the named deviation. |
| `cmd_transfer` | timer-made backup | default source is `timer.txt` `DIR:` | WIRED | `transfer.txt` and `restore.txt` both name `20261014-110152`, the timer's directory. |
| `BACKUP-RUN.md` Appendix | scratch files | byte-equal fenced blocks | WIRED | 40/40 verified by this verifier. |
| `README.md` | `BACKUP-RUN.md` | pointer | WIRED | Present. |

### Data-Flow Trace (Level 4)

Not applicable in the UI sense (no rendered dynamic UI). The equivalent was checked: the restored values (pages, media hash, login, plugins) are read from the target container over HTTP, not from the source's files, and are compared with values captured independently on the source (`state-src.txt` against `state-tgt.txt`). The backup data flows source container, tar stream through host, target container; the secrets cross by exec-pipe and `leak_check` passed (`LEAK-CHECK OK`).

### Behavioral Spot-Checks

| Behavior | Check | Result | Status |
|----------|-------|--------|--------|
| Scripts parse | `bash -n` on `backup.sh`, `restore.sh`, `run.sh` | syntax-ok | PASS |
| Scripts unchanged since evidence | `git diff bf74496 HEAD -- deploy/backup.sh restore.sh DEPLOY.md units run.sh` | empty | PASS |
| Record appendix equals scratch | python byte comparison of 40 blocks | 40 equal, 0 bad | PASS |
| No BAD outside selftests | `grep ^BAD` over four scratch trees | only `compare-selftest.txt` / `selftest-tree.txt` (expected) | PASS |
| Docker re-run of `backup-chain` | not run | skipped: needs Docker and about ten minutes of emulated amd64; the committed evidence set is the artefact | SKIP (not a gap) |

Behavior-dependent truths (restore refuses before touching live data; the timer starts the hardened unit) are covered by behavioral evidence from the harness run itself: the four refusals compare InvocationID, ActiveEnterTimestamp, inode and tree hash before and after each case, and the timer evidence comes from systemd's own `TriggeredBy`/`Result`/journal. None is left at presence only.

### Probe Execution

No `scripts/*/tests/probe-*.sh` and none declared in the plans: not applicable.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| BAK-01 | 22-01, 22-02, 22-03, 22-04, 22-05 | `backup.sh` erzeugt eine verifizierte Sicherung von SQLite und Medien | SATISFIED | Truth 1; two integrity readers, content comparison with 16/16 files and 47 row counts equal, for the manual and the timer-made backup. |
| BAK-02 | 22-01, 22-02, 22-03, 22-04, 22-05 | `restore.sh` stellt auf einem leeren System her; Seiten, Medien, Login funktionieren; Dauer und Groesse gemessen | SATISFIED | Truth 2; restore into a fresh container, 53/53 state assertions, 17/17 check assertions, two measured data points. |
| BAK-03 | 22-02, 22-03, 22-04, 22-05 | Ein Lauf ueber `holzcloud-backup.timer` ist einmal real durchgespielt | SATISFIED | Truth 3; `timer.txt` 9 PASS, journal of one invocation. |

All three IDs appear in PLAN frontmatter and in REQUIREMENTS.md (marked `[x]`, traceability rows "Phase 22 / Complete"). Orphaned requirements (mapped to Phase 22 in REQUIREMENTS.md but claimed by no plan): none.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| all changed files | - | TBD/FIXME/XXX debt markers | none found | - |
| `deploy/backup.sh` | 80-85, 69, 98-102, 106-109 | Failure-path weaknesses (REVIEW WR-01 to WR-04) | WARNING | Not covered by any must-have; see below |
| `deploy/restore.sh` | 82-126 | Failure-path weaknesses (REVIEW WR-05 to WR-07, IN-02) | WARNING | Not covered by any must-have; see below |
| `deploy/test/run.sh` | 2583, 3811-3824 | `set -e` off inside `||` lists; verdicts ignore stage exit codes (WR-08, WR-09) | WARNING | Harness hardening; the actual run is clean (STEPS 78/0) and every verdict was backed by file content |

### Human Verification Required

None. Every success criterion has direct, machine-readable evidence. Items the record itself names as not tested (the real 03:15 window, `Persistent=true`, `RandomizedDelaySec`, `REMOTE_TARGET` copy, retention pruning, native amd64 timing, a restore on a real rebuilt server) are explicitly outside the three roadmap criteria and are declared as such in `BACKUP-RUN.md` "Not tested", not claimed.

### Gaps Summary

No gaps. All three roadmap success criteria are achieved and proven by an evidence set that was made from the committed final scripts and has been unchanged since; the record's appendix is byte-identical to that evidence.

Advisory (from `22-REVIEW.md`, 0 critical, 9 warnings; none contradicts a must-have, so none fails the phase). These are real risks in the production scripts that the harness did not exercise, and the maintainer should decide where they go (phase 23 is documentation/release; a follow-up quick task or a later milestone fits better):
- WR-02 (highest value): `backup.sh` does not check that `holzcloud.sqlite` exists; an empty or wrong `DATA_DIR` yields a green, "verified" backup of an empty database, and retention would eventually discard the good ones. Interacts with WR-06 (the timer can fire during a restore).
- WR-01: a file vanishing during rsync (exit 24) aborts the backup under `set -e`, contradicting the header comment and DEPLOY.md.
- WR-05: `restore.sh` has no recovery message or trap after the service is stopped and the data directory is moved aside.
- WR-03/WR-04: retention `rm -rf` is not restricted to backup-shaped names and hides errors; `rsync --delete` to the off-box target plus a unit that cannot run ssh (`ProtectHome`, no home). The record already states the off-box path never ran.
- WR-07: `restore.sh` hard-codes port 8080 for `/readyz`.
- IN-06: DEPLOY.md says restore "about seven seconds" where the measured wall time is 8.1-8.3 s (7.4 s is only start-to-ready); within the phase's own factor-of-two rule, but the three documents should agree.

Also noted: `deferred-items.md` records a STATE.md position problem (three `Phase:` entries, progress counters 0) that is out of scope for this verification.

---

_Verified: 2026-10-14_
_Verifier: Claude (gsd-verifier)_
