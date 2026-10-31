---
phase: 24-the-backup-timer-as-shipped
plan: 02
subsystem: deploy/test (backup pruning, off-site copy, restore from the copy)
tags: [backup, retention, rsync, ssh, REMOTE_TARGET, sandbox, docker, verification]
status: complete

requires:
  - phase: 24-01
    provides: deploy/test/nightly.sh (selftest, source, target, fire, delay, catchup), the invocation-ID journal technique
provides:
  - nightly.sh prune, offsite, offsite-restore and the pure functions prune_expect, list_equal, unit_diff_lines, restore_lines_ok
  - raw evidence under deploy/test/out/amd64-t24-p2final (one chained run from fresh containers) and amd64-t24-p2 (task by task)
affects: [24-03]

tech-stack:
  added: []
  patterns:
    - "the expectation of a planted directory comes from a pure function, not from the author"
    - "a failed attempt is a FINDING with raw evidence, then the next configuration is tried"
    - "the unit under test is changed on the installed copy only, and the diff against the repository file is counted"

key-files:
  created: []
  modified: [deploy/test/nightly.sh]

key-decisions:
  - "The prune boundary is recorded as measured, not judged: a directory goes once floor(age / 24 h) exceeds RETENTION_DAYS"
  - "Credentials are measured: the key where an operator first puts it (the service user's home) fails under the shipped sandbox, a key outside /home with a system ssh config works with no change to the unit"
  - "The restore is played on the second host against /backups/holzcloud/TS, the directory the unit's rsync made"

requirements-completed: [BKP-04, BKP-05]

metrics:
  duration: "about 1 hour 15 minutes wall clock (emulated amd64 on an aarch64 host)"
  completed: 2026-10-26

actuals:
  tokens: 12600
  tasks: 3
  commits: 3
plan_head_before: ea0dac60c659d42ff324a9f643b74e22d7668175
plan_head_after: 474e4f5533322de010d66293e98fc8aa7a8bc209
commits: 3
---

# Phase 24 Plan 02: pruning, off-site copy and restore from it Summary

**`deploy/test/nightly.sh` now plays the shipped `holzcloud-backup.service` through retention pruning (BKP-05) and through an rsync copy to a second host (BKP-04), and restores the site from that copy with the document's own restore line. All three green, reproduced in a second run from fresh containers. One finding for plan 03: the document names no credential path, and the obvious one (a key in the service user's home) does not work under the shipped sandbox.**

## Result

| Requirement | Result | Raw evidence (final set `deploy/test/out/amd64-t24-p2final/`, host 2 in `...-host2/`) |
|---|---|---|
| BKP-05 | `PRUNE OK`. Five directories planted as the service user (mode 700, a file and a subdirectory with a file, mtime set last) at 45 d, 31 d 1 h, 30 d 12 h, 29 d and 1 d. One start of the shipped service (RETENTION_DAYS=30 read from the unit, REMOTE_TARGET unset, installed units byte-equal to `deploy/`): 45 d and 31 d 1 h gone, 30 d 12 h, 29 d and 1 d present, each equal to `prune_expect`; the three earlier backups still there; the new backup `holzcloud 700`, 1 s old. The survivors were removed by a recorded cleanup so they did not travel to host 2 | `prune.txt`, `prune-before.txt`, `prune-after.txt`, `prune-journal.txt`, `prune-final.txt` |
| BKP-04 copy | `OFFSITE OK`. With the one uncommented line (`UNIT-DIFF-LINES: 1`, diff in `offsite-unit-diff.txt`) the shipped service copied the whole backup root to `offsite@nas:/backups/holzcloud`. 122 files compared by sha256, equal; all 7 directories `offsite 700`; no drop-in. Working credentials: `offsite-ssh` (see finding) | `offsite.txt`, `offsite-natural.txt`, `offsite-journal.txt`, `offsite-journal-b.txt`, `offsite-sha-source.txt`, host 2: `offsite-host2.txt`, `offsite-sha-host2.txt`, `offsite-modes.txt` |
| BKP-04 restore | `OFFSITE-RESTORE OK`. Host 2's `/opt/holzcloud/backups` was empty before and after; the document's restore line was played against `/backups/holzcloud/20261026-095552` (restore-run.txt names it): setup form 200 before and 404 after, login with the old password 303 to `/admin/`, media served 200 with the source's sha256 `925d26e1...6577`, marker page, `check` 17 of 17, state comparison 53 pass 0 bad, wall 10.85 s | `offsite-restore.txt`, host 2: `restore.txt`, `restore-run.txt`, `transfer.txt` |

The task by task set in `amd64-t24-p2/` is also green (prune, offsite, offsite-restore) and ran in the same order from the same fresh containers. It differs from the final set only in that the `OBSERVED RULE` sentence was reworded afterwards (see below); the final set was run with the committed code.

Guards at the end: `git diff --quiet bf74496 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` exits 0 and `git diff --quiet f1ba5a2 -- docs/beta.md` exits 0. No shipped file, `run.sh` or `docs/beta.md` was edited. The only change to a unit was made on the installed copy in the container and is one line.

## Measured facts handed to plan 03

These are recorded, not classified. Plan 03's rule decides which of them is a defect.

1. **Prune boundary.** `find -mtime +30` on the directory's mtime deleted the 31 d 1 h directory and kept the one that is 30 d 12 h old. Observed rule, from `prune.txt`: a directory is deleted once floor(age / 24 h) exceeds `RETENTION_DAYS`, which is from 31 days of age. The document says "Retention defaults to 30 days". A 30 d 12 h old backup is older than 30 days and is kept; by the same rule it becomes eligible 12 hours later (a later run was not played). The sandbox did not block the removal: a non-empty tree with a subdirectory was removed under `ProtectSystem=strict` with `ReadWritePaths`.
2. **Credentials, finding.** `DEPLOY.md` says only "Set `REMOTE_TARGET` in `holzcloud-backup.service`"; the unit comment says "Uncomment to copy off the box". Neither names a credential.
   - The Prerequisites install `rsync`; neither the source nor a plain second host had an ssh client or server (`SSH-CLIENT-ON-SOURCE-BEFORE: absent`, `SSH-SERVER-ON-SECOND-HOST-BEFORE: absent`). `openssh-client` / `openssh-server` were installed in the containers (DEVIATION[ssh-packages]).
   - The service user is `holzcloud:x:997:997::/home/holzcloud:/usr/sbin/nologin` and `/home/holzcloud` does not exist. The operator's first try, `ssh-keygen -t ed25519 -N ''` as that user, stops at the file prompt and exits 1 (`offsite-natural.txt` A1).
   - **Experiment A** (key and known_hosts in `/home/holzcloud/.ssh`, created by the operator): the same `backup.sh` as `holzcloud` outside the sandbox with `REMOTE_TARGET` set ended 0 and the directory arrived (A3). In the shipped service it ended `Result exit-code`, `ExecMainStatus 255`, journal `hostkeys_find_by_key_hostfile: hostkeys_foreach failed for /home/holzcloud/.ssh/known_hosts: Permission denied`, `Host key verification failed`, `rsync error: unexplained error (code 255)` (`offsite-journal.txt`); the directory did not arrive. Cause isolated with transient units run with `systemd-run --wait --pipe -p User=holzcloud`: with the unit's `ProtectHome=true`, `ProtectSystem=strict`, `NoNewPrivileges=true`, `PrivateTmp=true` and its `ReadWritePaths`, `ls -la /home/holzcloud/.ssh` gives `Permission denied` and `ssh -o BatchMode=yes offsite@nas true` gives the same host key error, exit 255; the same unit without `ProtectHome` lists the key directory and ssh exits 0 (A5). So `ProtectHome=true` hides the home from the service.
   - **Experiment B** (key and known_hosts in `/opt/holzcloud/offsite-ssh`, a `Host nas` block in `/etc/ssh/ssh_config.d/holzcloud-backup.conf`, `IdentitiesOnly`, `StrictHostKeyChecking yes`, `BatchMode yes`): the shipped service, unchanged apart from the one `REMOTE_TARGET` line, ended `success` / 0 and the copy arrived. This needs no change to the unit and does not loosen the sandbox (`/opt` and `/etc` are readable under `ProtectSystem=strict`). No STOP condition arose.
3. **Mirror.** A stray directory planted on host 2 before the final run was gone after it: `rsync -a --delete` makes the off-site copy a mirror of the backup root, so what the source prunes or loses is lost off-site too (T-24-10, accepted, nothing built).
4. **Failed copy leaves the backup.** The failed A4 run still left its complete new backup directory on the source (the copy runs after the backup and before the final `=== Backup complete` line), so a failed copy shows only as `Result exit-code` and a missing complete line; the next successful run mirrored it.

## What was built

`deploy/test/nightly.sh`, bash 3.2, subcommands `prune`, `offsite`, `offsite-restore` added to the dispatch and the header. Pure functions with selftest tables (163 cases now, 43 new, failing cases included): `prune_expect` (13), `list_equal` and `unit_diff_lines` (16), `restore_lines_ok` (14). Each table was added and seen failing (command not found) before the function was written. `leak_check` runs at the end of all three, and `offsite` adds `leak_ssh`, which looks for a private key body line and for a host key blob as fixed strings in every output file; neither occurred.

## Deviations from Plan

### Auto-fixed Issues

None needed. The three subcommands ran green on their first run against the fresh containers.

### Recorded deviations of the test setup (written to `deviations.txt`)

- **DEVIATION[ssh-packages]**: the document's Prerequisites install rsync, which only suggests an ssh client and server; `openssh-client` (source) and `openssh-server` (host 2) were installed with `--no-install-recommends`.
- **DEVIATION[offsite-host]**: host 2 is a second fresh container on the same bridge network (172.17.0.x), not another machine on another network; it cannot show a real network path, a NAS, a firewall or another distribution.
- **DEVIATION[hosts]**: host 2 is reached as `nas` by an appended `/etc/hosts` line in the source (Docker bind-mounts that file).
- **DEVIATION[remote-target]**: the only change to the shipped service is the line its comment says to uncomment, on the installed copy: `# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud` became `Environment=REMOTE_TARGET=offsite@nas:/backups/holzcloud`.
- **DEVIATION[credentials]**: the private key lives only in the source; only the public key crossed to host 2 by exec-pipe; the host key was taken by `ssh-keyscan` on first use (T-24-08: a real operator compares the fingerprint by hand).
- **DEVIATION[fixture]**: the login password file, AI key file and test image crossed from the source to host 2 by exec-pipe, as `run.sh transfer` does it, plus the ORDER line of `baseline.txt`.
- The prune run and the off-site attempts start the service by hand (`systemctl start`), stated in a KEY line; the timer was proven in plan 01.
- Reuse of the cached binary: `NEW_REF=8fc579c` was pinned so that plan 01's built binaries in `deploy/test/out/bin` were used.

### Plan wording

- The plan's automated verify for task 1 re-uses plan 01's trace container; it no longer existed, so fresh containers were created with `source` and `target` (names `holzcloud-t24p2`, `holzcloud-t24p2f`, evidence `amd64-t24-p2*`), and `run.sh backup` was run before `prune`, as the plan's preconditions say.
- The `OBSERVED RULE` INFO line of the first task by task run said the kept directory "is deleted by a later nightly run". That is the rule applied, not an observation, so the sentence was reworded (eligible 12 hours later, a later run was not played) before commit; the final set carries the committed wording.

## Not played, handed to the owner

- The pull by `rsync -a` that the document describes for a rebuilt server. The restore ran on host 2 itself, from the copy path.
- A real remote target with a real network, a hardware NAS, a firewall, host key comparison by fingerprint.
- A prune run a day later on the 30 d 12 h directory (the rule predicts it goes; not played).

## Known Stubs

None.

## Threat Flags

None beyond the register. T-24-07 (key disclosure) held: only the public key crossed, `leak_check` and `leak_ssh` passed in all three subcommands. T-24-09 (prune removes a backup that matters): the must-stay set of 3 earlier backups was asserted present. T-24-11: every copied directory is `offsite 700`. T-24-12: host 2's own backup root was empty before and after the restore, and `restore-run.txt` names the copy path.

## Evidence locations

- Worktree (gitignored): `deploy/test/out/amd64-t24-p2final/` and `...-host2/` (chained run), `deploy/test/out/amd64-t24-p2/` and `...-host2/` (task by task), logs `amd64-t24-p2*.log`, `amd64-t24-p2final-*.log`.
- Copy merged into `/private/tmp/claude-501/-Users-holz-Projects-holzcloud-cms/295eb7a6-636f-4c10-8442-a5a0771f92c4/scratchpad/phase24-out/` (plan 01's content kept).
- All four containers (`holzcloud-t24p2`, `holzcloud-t24p2-host2`, `holzcloud-t24p2f`, `holzcloud-t24p2f-host2`) were removed.

## Self-Check: PASSED

- `deploy/test/nightly.sh` exists; `bash -n` and `bash deploy/test/nightly.sh selftest` pass (163 cases).
- Commits 3d467be, 53af806 and 474e4f5 are on HEAD (`git rev-list --count ea0dac6..HEAD` = 3).
- `prune.txt` ends `PRUNE OK`, `offsite.txt` ends `OFFSITE OK`, `offsite-restore.txt` ends `OFFSITE-RESTORE OK` in `amd64-t24-p2final`; no BAD line in them or in host 2's `restore.txt`; no `PRIVATE KEY` marker in any evidence file.
- Guards on the bound files and on `docs/beta.md` exit 0.
