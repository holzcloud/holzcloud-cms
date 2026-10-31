# The backup timer as shipped, measured: results

This document is the permanent record of phase 24. It records what happened when the nightly backup was played exactly as shipped: `deploy/holzcloud-backup.timer` and `deploy/holzcloud-backup.service` with no drop-in on either, installed by the Backups block of `deploy/DEPLOY.md`. The timer fired from its own `OnCalendar` entry, its 300 second random delay was sampled, a window missed while the host was down ran after it came up again, directories older than the retention were deleted by one run of the service, and with `REMOTE_TARGET` set the copy arrived on a second host and the site was restored from that copy. Nothing was tuned to make a run pass: a failure is a finding with the output that showed it.

The record rests on one evidence set: a single `nightly.sh chain` run from fresh containers, made once against the committed text of every bound file (see *How this was run*). Its raw output is the Appendix, pasted unchanged, and `bash deploy/test/nightly.sh record-check` proves every block against its scratch file. Every number and every PASS in this document is a line of that set and cites its file in backticks as `DIRECTORY/FILE`, with `DIRECTORY` one of the two scratch directories `amd64-t24-src` (the source) and `amd64-t24-host2` (the second host). What the container cannot show is a numbered owner step and a *Not tested* line, never a claim. One defect was found in a shipped file, a document that names no working credential path for `REMOTE_TARGET`; it is listed in *Findings* with the output that showed it, and it was repaired in the text of `deploy/DEPLOY.md` only.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| BKP-01 | PASS. The shipped timer started the shipped service by itself at 03:15:19 local, `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success` (`amd64-t24-src/fire.txt`). The installed units are byte-equal to `deploy/` and neither has a drop-in (`amd64-t24-src/units-shipped.txt`). The backup `20261026-031519` passes `holzcloud check` and `sqlite3 integrity_check`, is `holzcloud 700`, holds `csrf.key` and 3 of 3 media files (`amd64-t24-src/verify-fire.txt`) | linux/amd64, x86_64 container under emulation on an aarch64 Docker Desktop host, systemd 252, Debian 12 (`amd64-t24-src/versions.txt`) | 2026-10-26, b501b1b |
| BKP-02 | PASS. The stamp (`1793011250`) is older than the missed instant (`1793011523`); the container was down from `1793011375` to `1793011563` (188 s); the shipped timer started the service 157 s after its activation, `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, and the backup `20261027-031818` passes both readers (`amd64-t24-src/catchup.txt`, `amd64-t24-src/verify-catchup.txt`). The control timer with the same calendar and `Persistent=false` did not run (`CONTROL-RAN: no`) | same | 2026-10-26, b501b1b |
| BKP-03 | PASS. The installed timer holds `RandomizedDelaySec=300` and the running timer reports 300 s; 12 re-armings gave delays of 31 to 298 s, 12 distinct values; the real run's draw was 18 s after 03:15:00 and it started at 03:15:19, inside [03:15:00, 03:21:05] (`amd64-t24-src/delay.txt`) | same | 2026-10-26, b501b1b |
| BKP-04 | PASS. With the one line its own comment says to uncomment (`UNIT-DIFF-LINES: 1`) the shipped service copied the backup root to the second host, `Result success`, `COPY-DIR: /backups/holzcloud/20261026-105004`, 158 files compared by sha256 and equal, the same 9 directories on both hosts, each `offsite` with mode 700 (`amd64-t24-src/offsite.txt`). The key in the service user's home failed under the shipped sandbox, a key outside `/home` worked (Findings F-2). The document's restore line played on the second host against that copy: setup form 200 before and 404 after, `LOGIN: HTTP 303 /admin/`, media served `HTTP 200` with the source's sha256, marker page, `check` 17 of 17, 10.858 s (`amd64-t24-src/offsite-restore.txt`, `amd64-t24-host2/restore.txt`) | same | 2026-10-26, b501b1b |
| BKP-05 | PASS. Five directories planted at 45 d, 31 d 1 h, 30 d 12 h, 29 d and 1 d; one run of the shipped service (`RETENTION-DAYS-FROM-UNIT: 30`): the 45 d and the 31 d 1 h directory are gone, the 30 d 12 h, 29 d and 1 d directories are present, each as `prune_expect` says; the four earlier directories stayed; the new backup is `holzcloud 700` and 2 s old (`amd64-t24-src/prune.txt`) | same | 2026-10-26, b501b1b |

Evidence commit: b501b1b

The binary under test is the one built from `NEW_REF` (pinned by the chain to `b501b1b`) with the Build line of `deploy/DEPLOY.md`: `MANIFEST version=holzcloud v0.1.0-31-gb501b1b (b501b1b)` in `amd64-t24-src/verify-fire.txt`. The totals of the set are in `amd64-t24-src/summary.txt`: `BKP-01 PASS` to `BKP-05 PASS`, `STEPS: 56 played, 0 failed` and `LEAK-CHECK OK`.

## How this was run

`deploy/test/nightly.sh` is a sibling of `deploy/test/run.sh` (which is not edited: `docs/beta.md` binds it to its evidence commit) and reaches the harness through `run.sh`'s public subcommands only. `chain` refuses to start unless `TARGET=systemd-base` and both containers are absent, pins `NEW_REF` to a sha so that the source and the second host get the byte-identical binary, and plays source, second host, `fire`, `delay`, `catchup`, `prune`, `run.sh backup`, `offsite` and `offsite-restore`, each stage a child process whose status is kept. The verdict of each requirement is computed from the files the stage wrote, never from what the stage said about itself. The Backups block of the document is played by `run.sh` from the document (`install`, `postinstall`), so the units are the shipped ones, and `units-shipped.txt` shows it: byte-equal to `deploy/`, `DropInPaths` empty, no `holzcloud-backup.*.d` directory.

Pass 1, the chain against the files exactly as they stood when the plan began (evidence commit `cd0d610`), nothing fixed, started 2026-10-26 10:06:09Z and ended 10:18:52Z on the host clock (12 minutes 43 seconds; the image and the Go build were cached). It ended with exit 1 after writing its summary:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=cd0d610 bash deploy/test/nightly.sh chain
```

Its summary read `BKP-01 PASS`, `BKP-02 PASS`, `BKP-03 PASS`, `BKP-04 FAIL (the stage ended with status 1)`, `BKP-05 PASS` and `RESTORE: LOGIN=? MEDIA-SERVED=? WALL-S=?`. The failure was a defect of the harness (Findings F-1), and the chain also showed the defect of a shipped document (F-2). Both were repaired in separate commits whose subjects begin `fix(24-03):`; pass 1's output is quoted in *Findings* and is not pasted again.

A second chain, against the repaired files (evidence commit `7baa09d`), was green in all five requirements (10:22:07Z to 10:35:26Z). It was thrown away and made again, not patched, because a code comment in `nightly.sh` named the wrong cause for the lost test image (Findings F-1): the comment was corrected, `catchup` now also records the cause as a KEY line, and the whole chain was run again from fresh containers. A set is made against one text, so that set is not used.

The final set, from fresh containers, against the committed text of `b501b1b`, started 2026-10-26 10:37:17Z and ended 10:50:49Z on the host clock (13 minutes 32 seconds), exit 0:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=b501b1b bash deploy/test/nightly.sh chain
```

No bound file changed after the evidence commit. The bound files are `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh` and `deploy/test/nightly.sh`; this command exited 0 when the record was written:

```text
git diff --quiet b501b1b HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh
```

No clock was set anywhere: `CLOCK-SKEW-S-BEFORE: 0` and `CLOCK-SKEW-S-AFTER: 0` in `amd64-t24-src/fire.txt` and `amd64-t24-src/catchup.txt`. What moves is the time zone (a fixed-offset zone compiled with `zic` and copied to `/etc/localtime`), which changes what 03:15 means and nothing else.

## Deviations named

Everything here precedes every result: it is what the container does differently from a real server and what the harness did that the document does not contain. The first column is the name written to `amd64-t24-src/deviations.txt`; the kinds in the final summary are `backup-name control-timer credentials dropin fixture hosts mount offsite-host remote-target restart scp ssh-packages zone` (`amd64-t24-src/summary.txt`). `zone-rearm` and `network` did not occur.

| Name | Why | What a real server does instead |
|------|-----|---------------------------------|
| DEVIATION[zone] | A test cannot wait for 03:15. The time zone is moved, never the clock: `Nightly/Phase24` (`STDOFF -7:25:31`) made the local time read 03:13:30 and the shipped timer re-armed in it; for the missed window a second zone `Nightly/Catchup` a whole day away (`STDOFF 16:29:37`) so that `backup.sh` cannot name two backups alike | The real zone, the real clock, a real day passing, daylight saving |
| DEVIATION[control-timer] | A container-only `p24-control.timer` with the shipped calendar and delay and `Persistent=false`, aimed at its own service. New files, not a drop-in on a shipped unit; it is the evidence that the catch-up belongs to `Persistent=true` | Nothing; it has no counterpart |
| DEVIATION[restart] | A container stop and start replaces a power cycle: PID 1 and every unit restart and the writable layer, where the stamp lives, survives; no kernel boots, no firmware runs, no network comes up from cold | The host is powered off and on; the boot order of the timer, the clock and the network is the machine's |
| DEVIATION[ssh-packages] | The Prerequisites install `rsync`, whose package only suggests an ssh client and server (`SSH-CLIENT-ON-SOURCE-BEFORE: absent`, `SSH-SERVER-ON-SECOND-HOST-BEFORE: absent`); `openssh-client` and `openssh-server` were installed with `--no-install-recommends` | A server already has an ssh client, and the remote machine already runs sshd |
| DEVIATION[offsite-host] | The second host is a second fresh container on the same bridge (`holzcloud-t24-host2` at 172.17.0.3) | Another machine on another network, a NAS, a firewall, perhaps another distribution |
| DEVIATION[hosts] | The second host is reached as `nas` by an `/etc/hosts` line in the source | DNS or an address in `REMOTE_TARGET` |
| DEVIATION[remote-target] | The only change to the shipped service is the line its own comment says to uncomment, on the installed copy: `Environment=REMOTE_TARGET=offsite@nas:/backups/holzcloud` (`UNIT-DIFF-LINES: 1` in `amd64-t24-src/offsite.txt`); the repository file is untouched | The operator edits the same line |
| DEVIATION[credentials] | The document and the unit name no credential. The key lives only in the source, only the public key crossed to the second host by exec-pipe, and the host key was taken by `ssh-keyscan` on first use | The operator places the key and compares the host key fingerprint by hand |
| DEVIATION[fixture] | The login password file, the AI key file and the test image stand in for what an operator remembers; none is part of any backup, and they crossed from container to container by exec-pipe, never through a host file. The plugin archives, language file, logo and the order row are fixtures of phases 21 and 22 | The operator's own accounts, plugins and content |
| DEVIATION[backup-name] | The restore line of the document carries a sample timestamp (`20260802-031500`); it was replaced by the directory the unit's copy made, `/backups/holzcloud/20261026-105004` (`amd64-t24-host2/restore-run.txt`), and `yes` is typed on stdin | The operator types the name of the backup directory |
| DEVIATION[dropin] | The one inherited drop-in is on `holzcloud.service` only: `/etc/systemd/system/holzcloud.service.d/container.conf` sets `ProcSubset=all` because the unchanged unit crashes under the amd64 emulation. Neither backup unit has a drop-in | No drop-in |
| DEVIATION[scp] | The two `scp` lines of the document were played as `docker cp` (no ssh, no second machine) | `scp` or `rsync` |
| DEVIATION[mount] | `RequiresMountsFor` names `/opt/holzcloud/data`, a plain directory here | The data directory may be its own volume |
| amd64 emulation | Not a `DEVIATION[...]` line: `amd64-t24-src/versions.txt` shows an `aarch64` host and an `x86_64` container, so every container ran under emulation | A native amd64 server |

## Findings

Each finding has the class of the plan's rule: (1) a bug of the harness, fixed in `nightly.sh`; (2) a limit of the container, a named deviation or a *Not tested* line; (3) a measured property of a shipped file that matches what it and its document say, recorded and not edited; (4) a defect, a shipped file doing less than it or its document says, shown by a failing assertion whose cause a control isolated.

**F-1 (class 1): the test image is gone after the catch-up's container restart.** Pass 1's `offsite-restore` ended (quoted from `amd64-t24-pass1-src/offsite-restore.txt` and its summary, which are not pasted again):

```text
BAD  /tmp/hc-upload.jpg could not be carried from holzcloud-t24-src to holzcloud-t24-host2
BAD  run.sh restore was not played: the independence, the fixtures or the copy comparison failed above
OFFSITE-RESTORE FAILED
BKP-04 FAIL (the stage ended with status 1)
```

The file is the test image that `run.sh baseline` stages in `/tmp`; the restore on the second host reads it. `catchup` stops and starts the source container, and `/tmp` is emptied at every boot. A first reading blamed a tmpfs; that was wrong. `/tmp` has no mount of its own and the tmpfiles rule `D /tmp 1777 root root -` of systemd empties it at every boot, while a copy kept under `/root` survives the same stop and start. The cause is now part of the evidence set (`amd64-t24-src/catchup.txt`):

```text
TEST-IMAGE-KEPT-FOR-THE-STOP: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
TEST-IMAGE-AFTER-THE-START: gone from /tmp with the restart, put back from /root: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
TMP-AT-BOOT: tmpfiles.d says 'D /tmp 1777 root root -'; /tmp has a mount of its own: no
```

Done: `catchup` keeps the file under `/root` for the stop and puts it back after the start (commits `6a27bea` and `b501b1b`, both `fix(24-03):`); no shipped file is involved. The cause is a property of this image's `/tmp` and of the staged fixture, not of any shipped file.

**F-2 (class 4): the document names no working credential path for `REMOTE_TARGET`.** `deploy/DEPLOY.md` said only that `REMOTE_TARGET` is to be set in the unit; neither it nor the unit's comment names a key, a `known_hosts` file or a place for them. The place an operator reaches first, a key and `known_hosts` in the service user's home, does not work in the shipped unit. The service user is `holzcloud:x:997:997::/home/holzcloud:/usr/sbin/nologin` and `/home/holzcloud` does not exist (`SERVICE-USER-HOME: /home/holzcloud (exists by default: no)`). In `amd64-t24-src/offsite.txt` (the same lines were in pass 1):

```text
A-OUTSIDE-EXIT: 0
A-OUTSIDE-DIR: 20261026-104954 (on the second host: yes)
A-SANDBOX-RESULT: start-rc 1 Result exit-code ExecMainStatus 255 directory 20261026-104957 on the second host: no
B-RESULT: start-rc 0 Result success ExecMainStatus 0 directory 20261026-105004 on the second host: yes
WORKING-CREDENTIALS: offsite-ssh
```

The control that isolated the cause is in `amd64-t24-src/offsite-natural.txt` (A5): a transient unit run as `holzcloud` with the unit's own `ProtectHome=true`, `ProtectSystem=strict`, `NoNewPrivileges=true`, `PrivateTmp=true` and `ReadWritePaths` gives `ls: cannot access '/home/holzcloud/.ssh': Permission denied` and, for ssh, `hostkeys_find_by_key_hostfile: hostkeys_foreach failed for /home/holzcloud/.ssh/known_hosts: Permission denied` and `Host key verification failed.` (exit 255); the same unit without `ProtectHome` lists the key directory and ssh exits 0; the same `backup.sh` as `holzcloud` outside the sandbox ended 0 and the directory arrived (A3). So `ProtectHome=true` hides the home from the service, the unit's journal shows `rsync error: unexplained error (code 255)` (`amd64-t24-src/offsite-journal.txt`), and a document that says "set `REMOTE_TARGET`" does not lead to a working copy.

Experiment B (same file) kept the key and `known_hosts` under `/opt/holzcloud/offsite-ssh` and named them in `/etc/ssh/ssh_config.d/holzcloud-backup.conf` (`IdentityFile`, `UserKnownHostsFile`, `IdentitiesOnly yes`, `StrictHostKeyChecking yes`, `BatchMode yes`). The shipped service, unchanged apart from the one `REMOTE_TARGET` line, ended `Result success` and the copy arrived. `/opt` and `/etc` are readable under `ProtectSystem=strict`, so no sandbox directive had to move. Done: a paragraph in the Backups section of `deploy/DEPLOY.md` after the retention paragraph (commit `7baa09d`, `fix(24-03):`) that gives that location, the failure signature, the ssh client that `rsync` only suggests and the `--delete` mirror. Before: one sentence. After: 39 added lines, no line removed (`git diff --stat bf74496 -- deploy/DEPLOY.md`, below). Backups block 1, the only block `run.sh` plays, is unchanged; `deploy/holzcloud-backup.service` and `deploy/holzcloud-backup.timer` are byte-identical to `bf74496`; no `ProtectHome`, `ProtectSystem`, `NoNewPrivileges`, `PrivateTmp` or `ReadWritePaths` was removed or widened. The paragraph's example is the layout of experiment B; the fingerprint comparison it asks for is the operator's, not played here.

**F-3 (class 3): the retention boundary.** `find -mtime +30` counts whole 24 hour units, rounded down, so a directory is deleted once `floor(age / 24 h)` exceeds `RETENTION_DAYS`, which is from 31 days of age. In `amd64-t24-src/prune.txt`: `OBSERVED-BOUNDARY: the youngest planted directory that was deleted was 31 d 1 h old, the oldest that was kept was 30 d 12 h old (retention 30 days)`. The script says "Pruning backups older than 30 days" and the document says retention defaults to 30 days; a directory 30 days and 12 hours old is older than 30 days and was kept, and by the same rule it becomes eligible 12 hours later (a later run was not played). The effect is up to one extra day of retention and never a deletion that is too early, so it is a recorded fact for the owner and not an edit. The sandbox did not block the removal: a tree with a subdirectory went under `ProtectSystem=strict` with `ReadWritePaths`.

**F-4 (class 3): the off-site copy is a mirror.** `rsync -a --delete` makes the second host's copy equal to the backup root: a stray directory planted on the second host before the final run was gone after it (`INFO MIRROR` in `amd64-t24-src/offsite.txt`). What the box prunes or loses, the copy loses at the next run. The document now says so (F-2).

**F-5 (class 3): `rsync` only suggests an ssh client.** `SSH-CLIENT-ON-SOURCE-BEFORE: absent` and `SSH-SERVER-ON-SECOND-HOST-BEFORE: absent` (`amd64-t24-src/offsite.txt`); the Prerequisites install `rsync` and nothing else, and that left no ssh client on the source. A real server and a real NAS have one, so this is a deviation and a sentence in the new paragraph, not a defect of the unit.

**F-6 (class 3): a failed copy leaves the new backup on the box.** The copy runs after the backup and before the `=== Backup complete` line. In the failed run (`amd64-t24-src/offsite-journal.txt`) the journal ends at `rsync error: unexplained error (code 255)` with no complete line, `Result exit-code`, while the new directory `20261026-104957` was already on the source; the next successful run mirrored it. A failed copy is therefore visible as a failed unit, not as a missing backup.

**F-7 (class 2): the Docker VM steps the realtime clock and every step draws a new random delay.** `TIME-CHANGED-LOGGED: 3` and `REROLLS-WITHOUT-RESTART: 3` in 75 seconds (`amd64-t24-src/delay.txt`), `TIME-CHANGED-LOGGED: 4` before the run in `amd64-t24-src/fire.txt`. The armed time read before a run is therefore not the one the run fired on (`fire-before.txt` showed `NEXT 03:17:11`, the run started at 03:15:19); the verdict uses the last draw PID 1 logged before the start (`LAST-ARMED-EPOCH`, `LAST-DRAWN-EPOCH`). `DRAWN-AFTER-ARMED-S: -8` means the timer had drawn a delay that was already in the past and elapsed at once. A real host with a synchronised clock steps it rarely; this is a named limit, not a defect.

**F-8 (class 3, read from the text and not exercised): a failed removal in the retention step is silent.** `backup.sh` prunes with `find ... -exec rm -rf {} + 2>/dev/null || true`. Here the removal worked, so the silence did not matter; if a real server's directory could not be removed, nothing in the journal would say so. This is why owner step O-5 counts directories instead of trusting the log.

## BKP-01: the shipped timer starts the hardened service at its own 03:15 entry

**What was done.** `fire` moved the time zone so that the local time read 03:13:30 and the shipped timer's own `OnCalendar=*-*-* 03:15:00` arrived 90 seconds later (`ZONE: Nightly/Phase24 STDOFF -7:25:31 OFFSET-S -26731 READ 03:13:30 LEAD-S 90`), then waited for the run without starting anything. The installed units were checked first: `SHA256-REPO-timer` equals `SHA256-INSTALLED-timer` (`87ccb6d9...ff9e88b0` for the timer, `befdd74b...e93bf7` for the service, in full in the file), `SHA256-REPO-service` equals `SHA256-INSTALLED-service`, `DROPINPATHS-timer: empty`, `DROPINPATHS-service: empty`, `DROPIN-DIRS: none`, `TIMERS-CALENDAR: { OnCalendar=*-*-* 03:15:00 ...}`, `Persistent=yes`, `RandomizedDelayUSec=5min`, `AccuracyUSec=1min` (`amd64-t24-src/units-shipped.txt`).

**Observed** (`amd64-t24-src/fire.txt`): `EXEC-START-BEFORE: empty` (the service had never run), `START-LOCAL: 03:15:19`, `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `EXIT-STATUS: 0`, `DIR: /opt/holzcloud/backups/20261026-031519`, `DIR-OWNER-MODE: holzcloud 700`, and the lines `PASS the service was started by holzcloud-backup.timer (TriggeredBy), not by hand` and `PASS the run started at 03:15:19 local, at or after 03:15:00 and before 03:25:00`. `amd64-t24-src/fire-before.txt` lists the timer waiting, `amd64-t24-src/fire-after.txt` shows `LAST Mon 2026-10-26 03:15:19 P24` and the next run `Tue 2026-10-27 03:16:55 P24`; `amd64-t24-src/fire-journal.txt` is the journal of that one invocation, taken by invocation ID. The backup it made passes the independent readers: `PASS integrity holzcloud-check: ok`, `PASS integrity sqlite3 integrity_check: ok`, the `MANIFEST` keys, `PASS csrf.key is in the backup`, `PASS the backup holds as many media files as the data directory (3)` (`amd64-t24-src/verify-fire.txt`).

**What it shows.** The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.

**What it does not show.** The real wall-clock 03:15 on a server's own zone (the zone was moved, DEVIATION[zone]), daylight saving, a timer that has been armed for hours, and any behaviour of the real host's clock synchronisation. Owner step O-1.

## BKP-03: the 300 second delay is real and the run starts inside the window

**What was done.** `delay` restarted the shipped timer 12 times, paced by three seconds, read the delay each re-arming drew from PID 1, and judged the real run of `fire` from its own KEY lines. The stamp is the timer's memory of its last run; the restarts must not move it.

**Observed** (`amd64-t24-src/delay.txt`): `PASS the installed holzcloud-backup.timer holds the line RandomizedDelaySec=300`, `PASS the running timer reports RandomizedDelayUSec of 300 s`, `PASS Persistent is yes`, twelve `SAMPLE` lines from 31 to 298 s (`D-MIN: 31`, `D-MAX: 298`, `D-DISTINCT: 12`, `DELAY-SAMPLES: 12`), `PASS the stamp's mtime did not move during the restarts (1793011250)`. The real run: `DELAY-USED-S: 18`, `START-AFTER-ARMED-S: 1`, `PASS the real run started at 03:15:19 local, inside [03:15:00, 03:21:05] (03:15:00 + 300 s delay + 60 s accuracy + 5 s allowance)`.

**What it shows.** The shipped timer delays by a random time drawn from [0, 300] seconds, the draws differ, and the run lands inside 03:15:00 plus delay plus accuracy.

**What it does not show.** That the distribution is uniform (twelve draws and one run), or the delay over many real nights. `AccuracyUSec` is systemd's default of 1 minute, inside the window.

## BKP-02: a window missed while the host was down runs after it is up again

**What was done.** `catchup` moved the zone a second time, a whole calendar day away, so that 03:15:00 was about 150 seconds ahead; armed a control timer with the same calendar and delay and `Persistent=false`; stopped the container cleanly before the instant, kept it down until the instant had passed with margin, and started it again. The test image was kept across the restart (F-1).

**Observed** (`amd64-t24-src/catchup.txt`): `STAMP-BEFORE-STOP: 1793011250`, `MISSED-EPOCH: 1793011523`, `MISSED-LOCAL: 03:15:00`, `DOWN-FROM: 1793011375`, `UP-AT: 1793011563`, `TIMER-ACTIVE-EPOCH: 1793011564`, `START-EPOCH: 1793011721`, `START-LOCAL: 03:18:18`, `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `DELAY-AFTER-BOOT-S: 157`, `CONTROL-RAN: no`, `CONTROL-PERSISTENT: no`; and `PASS a missed window was run after the boot: stamp 1793011250 < missed 1793011523, down at 1793011375, up at 1793011563, timer active at 1793011564, run at 1793011721 (157 s after the activation; limit max(missed, activation) + 300 + 60 + 30)`. The control `PASS` line says the catch-up belongs to the shipped `Persistent=true`. `amd64-t24-src/catchup-before.txt` and `amd64-t24-src/catchup-after.txt` list the timers, `amd64-t24-src/catchup-journal.txt` is the run's journal, `amd64-t24-src/verify-catchup.txt` the backup's readers (`integrity holzcloud-check: ok`, `sqlite3 integrity_check: ok`, 3 of 3 media files). The zone was put back (`ZONE-RESTORED: yes`).

**What it shows.** After a stop that spans the calendar instant, systemd's `Persistent=true` reads the stamp, finds it older than the missed instant, and the shipped timer starts the shipped service within about three minutes of its own activation; the same calendar without `Persistent` stays silent.

**What it does not show.** A real power cycle: the kernel does not boot, no firmware runs, the network does not come up from cold, and the boot order of timers, clock and network is the machine's (DEVIATION[restart]). Owner step O-2. The 157 seconds are an emulation figure and depend on the random delay.

## BKP-05: directories older than the retention are deleted, younger ones stay

**What was done.** `prune` planted five directories as the service user (mode 700, a file, and a subdirectory with a file; the mtime set last) at 45 d, 31 d 1 h, 30 d 12 h, 29 d and 1 d, read `RETENTION_DAYS=30` from the installed unit with `REMOTE_TARGET` unset, started the shipped service once, and compared the directories with what the pure function `prune_expect` says for each age. The expectation comes from a function with a table in `selftest`, not from the author.

**Observed** (`amd64-t24-src/prune.txt`): `PLANTED 20260911-104917 3888000 DELETE`, `PLANTED 20260925-094917 2682000 DELETE`, `PLANTED 20260925-224917 2635200 KEEP`, `PLANTED 20260927-104917 2505600 KEEP`, `PLANTED 20261025-104917 86400 KEEP`; `OBSERVED ... gone` for the first two and `OBSERVED ... present` for the last three; `RESULT: success`, `EXIT-STATUS: 0`; `PASS every directory that existed before the planting is still there (4)`; `DIR-OWNER-MODE: holzcloud 700`, `DIR-AGE-S: 2`. `amd64-t24-src/prune-before.txt`, `amd64-t24-src/prune-after.txt` and `amd64-t24-src/prune-journal.txt` are the listing before, after, and the run's journal (`Pruning backups older than 30 days...`). The three survivors were removed by a recorded cleanup so that they did not travel to the second host.

**What it shows.** In one run of the shipped service, an old directory and a younger one: the old ones are deleted and the young ones stay, the earlier backups stay, and the sandbox allowed the removal (F-3 says where the boundary lies).

**What it does not show.** Retention on elapsed real time: the ages are planted mtimes. A run a day later on the 30 d 12 h directory was not played. Owner step O-5.

## BKP-04: the copy arrives on a second host and the site comes back from it

**What was done.** `offsite` prepared a second container (ssh server, an `offsite` account, `/backups/holzcloud` mode 700), mapped it as `nas`, changed the one line of the installed unit that its own comment names, and tried two credential layouts with the shipped sandbox (F-2). `offsite-restore` then played `run.sh restore` unchanged on the second host: the document's restore line against the directory the unit's copy made, with the second host's own backup root empty before and after.

**Observed.** Copy (`amd64-t24-src/offsite.txt`): `PASS holzcloud-backup.service has no drop-in`, `B-RESULT: start-rc 0 Result success ExecMainStatus 0 directory 20261026-105004 on the second host: yes`, `FILES-COMPARED: 158`, `PASS the sha256 of every file under /opt/holzcloud/backups of the source (158 files) equals that of /backups/holzcloud on the second host`, `PASS the same 9 backup directories on both hosts`, `PASS every copied directory on the second host is owned by offsite with mode 700`; the run's journal is `amd64-t24-src/offsite-journal.txt` (the failing experiment) and the second host's own record `amd64-t24-host2/offsite-host2.txt`. Restore (`amd64-t24-src/offsite-restore.txt`): `BACKUPS-OF-HOST2-BEFORE: empty`, `RUN-SH-RESTORE-EXIT: 0`, `PASS restore-run.txt holds '=== Restoring from /backups/holzcloud/20261026-105004 ===': the document's restore line was played against the copy of the unit`, `SETUP-BEFORE: HTTP 200 -`, `LOGIN: HTTP 303 /admin/`, `SETUP-AFTER: HTTP 404 -`, `MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577` (the sha256 the source serves), `MARKER-PAGE`, `CHECK: check: 17 of 17 assertions passed`, `RESTORE-WALL: 10.858`, `PASS /opt/holzcloud/backups of holzcloud-t24-host2 is still empty after the restore`. The detail is in `amd64-t24-host2/restore.txt` (`PASS state comparison source against target: 53 pass, 0 bad`, `PASS tree backup-restored: 16 files equal`), `amd64-t24-host2/restore-run.txt` (the script's transcript, `=== Restore complete ===`), `amd64-t24-host2/check.txt` and `amd64-t24-host2/transfer.txt` (`TRANSFER: equal`, `VIA: rsync by the shipped unit, no docker cp of the backup`).

**What it shows.** With `REMOTE_TARGET` set in the shipped service, the whole backup root arrives on another host, byte for byte, with the owner and modes the unit's rsync gives; and a site restored from that copy alone has its pages, its media, its login and its 17 `check` assertions.

**What it does not show.** A real network, a real NAS, a firewall, the operator's credentials, a host key compared by fingerprint (DEVIATION[credentials]); the pull of the copy back to a rebuilt server by `rsync -a` that the document describes (the restore ran on the second host itself); a second run after the second host lost a directory. Owner steps O-3 and O-4.

## Measurements

The figures the harness wrote. Every one is an emulation figure on an aarch64 host (`amd64-t24-src/versions.txt`); none is a promise of speed.

| Figure | Value | Where |
|--------|-------|-------|
| Delay of the real run (BKP-01, BKP-03) | 18 s after 03:15:00, started 03:15:19 | `DELAY-USED-S`, `START-LOCAL` in `amd64-t24-src/delay.txt`, `amd64-t24-src/fire.txt` |
| Delays drawn by 12 re-armings | 31 to 298 s, 12 distinct | `D-MIN`, `D-MAX`, `D-DISTINCT` in `amd64-t24-src/delay.txt` |
| Container down | 188 s (`DOWN-FROM: 1793011375`, `UP-AT: 1793011563`) | `amd64-t24-src/catchup.txt` |
| Catch-up start after the timer's activation (BKP-02) | 157 s | `DELAY-AFTER-BOOT-S` in `amd64-t24-src/catchup.txt` |
| One service run with the off-site copy | 10:50:04.56 to 10:50:06.89, 2.3 s, of which the `rsync` step (`Copying to ...` to `=== Backup complete ...`) 1.5 s; the copied backup is `20M` | `amd64-t24-src/offsite-journal-b.txt` |
| Files compared on the second host | 158 over 9 directories | `FILES-COMPARED` in `amd64-t24-src/offsite.txt` |
| Restore on the second host | 10.858 s wall, of which waiting for `/readyz` 9.716 s, stop-move-copy 0.682 s, verify 0.134 s | `WALL-S`, `START-READY-S`, `STOP-MOVE-COPY-S`, `VERIFY-S` in `amd64-t24-host2/restore.txt` |

`deploy/DEPLOY.md` says a restore of this site takes about seven seconds; this chain measured 10.858 s, 1.6 times that, inside the factor of two that would call for the document to be corrected and the set to be made again. The restore time is dominated by the service starting and loading its plugins.

## Container vs. server

| Aspect | In the container | On a real server | Measured or assumed |
|--------|------------------|------------------|---------------------|
| 03:15 | The zone is moved so that 03:15:00 arrives in 90 s | The server's own zone and clock | Measured in the container (`amd64-t24-src/fire.txt`); the real wall clock is assumed |
| Daylight saving | A fixed-offset zone | The zone may shift an hour twice a year | Not tested |
| Realtime clock | The Docker VM steps it about every 30 s; each step re-arms the timer with a new random draw (`TIME-CHANGED-LOGGED`, `REROLLS-WITHOUT-RESTART`) | A synchronised host slews or steps it rarely | Measured here; the real behaviour is assumed |
| Power cycle | `docker stop` and `docker start`; PID 1 and the units restart, the writable layer survives, no kernel boots | The machine is off and on; timers, clock and network order themselves | Measured as a stop and start; boot ordering assumed |
| Persistence of the stamp | `/var/lib/systemd/timers/stamp-holzcloud-backup.timer` is in the writable layer | On the root filesystem | Measured here; the same file on a real host is assumed |
| Journal | `/var/log/journal` exists in the image and survives the stop; runs are found by invocation ID | Depends on `Storage=` in `journald.conf` | Measured here (`amd64-t24-src/catchup-journal.txt`); persistence on a real host is assumed |
| `/tmp` | Emptied at boot by the tmpfiles rule (`TMP-AT-BOOT` in `amd64-t24-src/catchup.txt`) | The same rule on a Debian 12 install | Measured here |
| Second host | A container on the same bridge, `nas` by `/etc/hosts`, ssh packages installed | Another machine, DNS, a firewall, sshd already running | Not tested on a real network |
| Credentials and host key | Key made in the source, public key crossed by exec-pipe, host key taken by `ssh-keyscan` on first use | The operator places the key and compares the fingerprint by hand | Measured for the layout (F-2); the fingerprint comparison is an owner step |
| Retention | Planted directories with set mtimes | Real days passing | Measured for the rule, not for elapsed time |
| Speed | amd64 emulated on aarch64; every figure is an emulation figure | A native server's disk, CPU and network | Not measured natively |

## Not tested

- The real wall-clock 03:15 on a server's own zone, and daylight saving.
- A real power cycle and the boot ordering of the timer, the clock and the network (the container was stopped and started).
- A real network between two real machines, the owner's credentials and a host key verified by fingerprint.
- The pull of the copy back to a rebuilt server by `rsync -a` into `/opt/holzcloud/backups` that the document describes; the restore ran on the second host from the copy path.
- Retention on elapsed real time rather than planted mtimes, and a run a day later on the 30 d 12 h directory.
- `journald` persistence across the stop on a real host (`Storage=` is the machine's).
- Native amd64 timing.
- A second off-site run after the second host lost a directory, and a copy to a target that is nearly full.
- Whether the distribution of the random delay is uniform (twelve draws and one real run).

## Owner steps

Each step is for the real server, one command per line, with the output to expect and what a different output means. Phase 26 (SRV-03) takes O-2 and O-4 into its own owner steps.

**O-1. The shipped timer at its real 03:15.** After the timer has been enabled for a night:

```bash
systemctl list-timers holzcloud-backup.timer
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStartTimestamp
journalctl -u holzcloud-backup.service --since yesterday -o short-precise | tail -n 20
timedatectl
```

Expect `TriggeredBy=holzcloud-backup.timer`, `Result=success`, an `ExecMainStartTimestamp` between 03:15:00 and about 03:21:00 local time, and a journal that ends with `=== Backup complete: /opt/holzcloud/backups/YYYYMMDD-HHMMSS ===`. A start at another hour points at the host's time zone (`timedatectl`), at a clock that is not synchronised (`System clock synchronized: no`), or at `AccuracySec` (the timer has the default of one minute). `TriggeredBy` empty means the last run was a manual one, not the timer's.

**O-2. The catch-up after a real power cycle.** Note the stamp, power the host off before 03:15, power it on after 03:21:

```bash
sudo stat -c '%y' /var/lib/systemd/timers/stamp-holzcloud-backup.timer
uptime -s
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStartTimestamp
journalctl -u holzcloud-backup.service -b -o short-precise
```

Expect a start within about six minutes of boot (the random delay of up to 300 seconds plus the accuracy; the service waits for `holzcloud.service`), `TriggeredBy=holzcloud-backup.timer`, `Result=success`. No run points at the stamp (it must be older than the missed instant; `systemctl show holzcloud-backup.timer -p Persistent` must say `yes`), at the zone, or at the timer not being enabled (`systemctl is-enabled holzcloud-backup.timer`).

**O-3. The off-site copy to your real second host, with your real key.** Follow the credentials paragraph in the Backups section of `deploy/DEPLOY.md` (the layout measured in F-2: key and `known_hosts` under `/opt/holzcloud/offsite-ssh`, a `Host` block in `/etc/ssh/ssh_config.d/holzcloud-backup.conf`). Compare the host key fingerprint by hand, then:

```bash
sudo -u holzcloud ssh-keygen -lf /opt/holzcloud/offsite-ssh/known_hosts
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub    # run this one ON the remote host and compare
sudo systemctl edit --full holzcloud-backup.service  # uncomment and set Environment=REMOTE_TARGET=user@host:/path
sudo systemctl start holzcloud-backup.service
systemctl show holzcloud-backup.service -p Result -p ExecMainStatus
ssh user@host ls -1 /path
```

Expect `Result=success`, `ExecMainStatus=0`, and the newest backup directory listed on the remote host with the same name. Different outputs name their own cause: `Permission denied (publickey)` is a public key that is not in the remote account's `authorized_keys`, or an `IdentityFile` that points elsewhere; `Host key verification failed` is a `known_hosts` that is missing, unreadable to the service (a file under `/home` is hidden by `ProtectHome=true`) or does not match; `rsync: Failed to exec ssh` or `ssh: command not found` is a server without an ssh client (`openssh-client`); `ExecMainStatus=255` in general is an ssh failure, and the new backup is on the box either way (F-6).

**O-4. The copy back from the second host and the restore on a rebuilt box.** On a server rebuilt by the Install and systemd Service sections, with no account created in the setup form:

```bash
sudo rsync -a user@host:/path/ /opt/holzcloud/backups/
sudo ls -1 /opt/holzcloud/backups
sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/YYYYMMDD-HHMMSS
```

Answer `yes` at the question. Expect the transcript to end with `=== Restore complete ===`, `/admin/setup` to answer 404, and a login with the old password to work; `ls` must show the directories with mode `drwx------` (`rsync -a` keeps it). A refusal before anything is stopped (`MANIFEST` missing, snapshot failing its check) names the directory; a directory without a `MANIFEST` is an interrupted copy, and a different login means the wrong backup directory was chosen.

**O-5. Retention on a real clock.** After more than 31 days of nightly runs:

```bash
ls -1 /opt/holzcloud/backups | wc -l
find /opt/holzcloud/backups -mindepth 1 -maxdepth 1 -type d -mtime +31
systemctl show holzcloud-backup.service -p Environment
```

Expect no directory from the `find` (nothing older than 31 days) and about 31 directories in the first count. A surviving old directory points at `RETENTION_DAYS` (check the third line; `0` switches pruning off), at the sandbox (`ReadWritePaths` must name `/opt/holzcloud/backups`), or at the script discarding the error of the removal (F-8: `2>/dev/null || true` hides a failure from the journal). A directory 30 days and a few hours old is expected to be there; it goes with the next run (F-3).

## Hand-over to phase 28

The bound files of the evidence commit differ from `bf74496`, the commit phase 23's beta criteria were written against, as follows (the output of the command, as it is):

```text
git diff --stat bf74496 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh
 deploy/DEPLOY.md | 39 +++++++++++++++++++++++++++++++++++++++
 1 file changed, 39 insertions(+)
```

Only `deploy/DEPLOY.md` differs, by 39 added lines in one hunk in the Backups section after the retention paragraph (F-2); `deploy/backup.sh`, `deploy/restore.sh`, both units and `deploy/test/run.sh` are byte-identical to `bf74496`. So P-4 to P-10 of `docs/beta.md`, which rest on `deploy/BACKUP-RUN.md` and on those files, rest on unchanged text, except where a row quotes the Backups section of the document: block 1 and the restore section are byte-identical, and the added paragraph comes after block 1. `deploy/test/nightly.sh` is a new file; `docs/beta.md` is not edited by this phase.

The five bullets of the group *The nightly backup and a restore* of `docs/beta.md` under *What 0.1.0 does not promise* that this record bears on, with the sentence that answers each and what stays unproven:

1. *The real 03:15 window was not exercised ... so the shipped calendar entry never fired.* Answered by BKP-01: the shipped `OnCalendar=*-*-* 03:15:00` fired the shipped service with no drop-in (`TIMERS-CALENDAR` in `amd64-t24-src/units-shipped.txt`, `START-LOCAL: 03:15:19` in `amd64-t24-src/fire.txt`). Unproven: the server's own zone and wall clock, and daylight saving (O-1).
2. *`Persistent=true` after a missed run was not exercised.* Answered by BKP-02: the missed window ran after the container was started again, and the control without `Persistent` did not (`amd64-t24-src/catchup.txt`). Unproven: a real power cycle and boot ordering (O-2).
3. *`RandomizedDelaySec` was not exercised.* Answered by BKP-03: the shipped 300 seconds are installed, the running timer reports them, twelve draws lay between 31 and 298 s and the real run started inside the window (`amd64-t24-src/delay.txt`). Unproven: uniformity and the delay over many nights.
4. *The off-site `REMOTE_TARGET` copy did not run.* Answered by BKP-04: the copy ran in the shipped service and the site was restored from it (`amd64-t24-src/offsite.txt`, `amd64-t24-src/offsite-restore.txt`). Unproven: a real network, the owner's credentials and a fingerprint check, the pull back by `rsync -a` (O-3, O-4). The credential layout is a documented, measured one (F-2); the key in the service user's home does not work.
5. *Retention pruning did not delete anything.* Answered by BKP-05: two of five planted directories were deleted and three stayed, in one run (`amd64-t24-src/prune.txt`). Unproven: elapsed real time, the 30 d 12 h survivor going one run later, the silence of a failed removal (O-5, F-3, F-8).

Nothing in this record is a promise until phase 28 cites it byte for byte. A change to a bound file after the evidence commit means the set is made again before the record is cited for the changed text.

## Appendix: raw output

Each block is the scratch file of the evidence set, pasted unchanged. `bash deploy/test/nightly.sh record-check` compares every block with its file byte for byte. The files are in `deploy/test/out` (gitignored); `step-NN.txt`, `seed.txt`, the logs, cookie jars and key files are not pasted.

### R1 source

#### amd64-t24-src/summary.txt

```text
---- summary (linux/amd64) ----
BKP-01 PASS
BKP-02 PASS
BKP-03 PASS
BKP-04 PASS
BKP-05 PASS
TIMER: TRIGGERED-BY=holzcloud-backup.timer RESULT=success START-LOCAL=03:15:19
DELAY: USED-S=18 SAMPLES=12 MIN-S=31 MAX-S=298
CATCHUP: TRIGGERED-BY=holzcloud-backup.timer AFTER-BOOT-S=157 CONTROL-RAN=no
PRUNE: PLANTED=5 DELETED=2 KEPT=3
OFFSITE: WORKING-CREDENTIALS=offsite-ssh FILES-COMPARED=158 UNIT-DIFF-LINES=1
RESTORE: LOGIN=HTTP 303 /admin/ MEDIA-SERVED=HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 WALL-S=10.858
STEPS: 56 played, 0 failed
DEVIATIONS: backup-name control-timer credentials dropin fixture hosts mount offsite-host remote-target restart scp ssh-packages zone
LEAK-CHECK OK
```

#### amd64-t24-src/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           6.12.76-linuxkit
container uname:       Linux 6.12.76-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-t24-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.0.10 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-b501b1b to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[zone]: the container cannot wait for 03:15, so the time ZONE is moved and never the clock: the fixed-offset zone Nightly/Phase24 (STDOFF -7:25:31, abbreviation P24, offset -26731 s from UTC) was compiled with zic and installed as /etc/localtime, which made the local time read 03:13:30; the shipped timer re-armed in it and its own 03:15:00 arrived 90 seconds later. No command in nightly.sh sets a clock. What it cannot show: a real day passing, daylight saving, the real host's zone, and a timer that has been armed for hours
DEVIATION[control-timer]: a container-only control, p24-control.timer with the calendar and delay of the shipped timer (OnCalendar=*-*-* 03:15:00, RandomizedDelaySec=300) and Persistent=false, aimed at its own p24-control.service (/bin/echo p24-control ran). New files, not a drop-in on a shipped unit; removed at the end. It is the evidence that the catch-up belongs to the shipped Persistent=true
DEVIATION[restart]: a container stop and start replaces a power cycle. PID 1 and every unit restart, and the writable layer, where the stamp lives, survives; no kernel boots, no firmware runs, no network comes up from cold, and the machine is not really off. The catch-up logic that is observed is systemd's own, reading the stamp at the timer's start; whether a real boot orders it the same way is an owner step, not a claim
DEVIATION[zone]: for the missed window the zone was moved a second time, to Nightly/Catchup (STDOFF 16:29:37, offset 59377 s, a whole day away from the zone of the first run so that the local date differs and backup.sh cannot name two backups alike), by files only; the local time read 03:12:31. No command sets a clock
DEVIATION[ssh-packages]: the document's Prerequisites install rsync, whose package only suggests an ssh client and an ssh server (ssh on the source: absent before, sshd on the second host: absent before); openssh-client was installed in the source and openssh-server in the second host with apt-get install --no-install-recommends. A real off-site target already runs sshd, and an operator's machine has an ssh client; the document does not say so
DEVIATION[offsite-host]: the second host is a second fresh container on the same bridge network (holzcloud-t24-host2 at 172.17.0.3), not another machine on another network. What it cannot show: a real network path, a real NAS, a firewall, a different distribution on the receiving side
DEVIATION[hosts]: the second host is reached as nas by an /etc/hosts line in the source (172.17.0.3 nas), appended with an append redirection because Docker bind-mounts that file; there is no DNS for it
DEVIATION[remote-target]: the only change to the shipped service is the line its own comment says to uncomment, on the installed copy /etc/systemd/system/holzcloud-backup.service: '# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud' became 'Environment=REMOTE_TARGET=offsite@nas:/backups/holzcloud'; the repository file is untouched
DEVIATION[credentials]: the document and the unit name no credential. The private key lives only in the source (/home/holzcloud/.ssh/id_ed25519 in experiment A, /opt/holzcloud/offsite-ssh/id_ed25519 in experiment B), only the public key crossed to holzcloud-t24-host2 by exec-pipe, and the host key was taken by ssh-keyscan on first use (T-24-08: a real operator compares the fingerprint by hand)
DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they crossed from container to container by exec-pipe, never through a host file, exactly as run.sh transfer does it; the ORDER line of the source's baseline.txt went to deploy/test/out/amd64-t24-host2/baseline.txt
```

#### amd64-t24-src/units-shipped.txt

```text
SHA256-REPO-timer: 87ccb6d9ec499469994b1ef0aa020bb6e3a9df140dd71b19169f4193ff9e88b0
SHA256-INSTALLED-timer: 87ccb6d9ec499469994b1ef0aa020bb6e3a9df140dd71b19169f4193ff9e88b0
PASS the installed holzcloud-backup.timer is byte-equal to deploy/holzcloud-backup.timer
DROPINPATHS-timer: empty
PASS holzcloud-backup.timer has no drop-in (DropInPaths is empty)
SHA256-REPO-service: befdd74bd8233c7922755824bdea6b60aa0bb0beb0c83ba0982562cdc7e93bf7
SHA256-INSTALLED-service: befdd74bd8233c7922755824bdea6b60aa0bb0beb0c83ba0982562cdc7e93bf7
PASS the installed holzcloud-backup.service is byte-equal to deploy/holzcloud-backup.service
DROPINPATHS-service: empty
PASS holzcloud-backup.service has no drop-in (DropInPaths is empty)
DROPIN-DIRS: none
PASS no holzcloud-backup.*.d directory exists
TIMERS-CALENDAR: { OnCalendar=*-*-* 03:15:00 ; next_elapse=Tue 2026-10-27 03:15:00 UTC }
PASS TimersCalendar reads OnCalendar=*-*-* 03:15:00
PERSISTENT: yes
---- systemctl show (timer) ----
TimersCalendar={ OnCalendar=*-*-* 03:15:00 ; next_elapse=Tue 2026-10-27 03:15:00 UTC }
AccuracyUSec=1min
RandomizedDelayUSec=5min
FixedRandomDelay=no
Persistent=yes
FragmentPath=/etc/systemd/system/holzcloud-backup.timer
DropInPaths=
---- systemctl show (service) ----
Environment=DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30
User=holzcloud
ProtectSystem=strict
NoNewPrivileges=yes
FragmentPath=/etc/systemd/system/holzcloud-backup.service
DropInPaths=
---- systemctl cat holzcloud-backup.timer ----
# /etc/systemd/system/holzcloud-backup.timer
[Unit]
Description=Nightly holzcloud-CMS backup
Documentation=file:///opt/holzcloud/deploy/DEPLOY.md

[Timer]
OnCalendar=*-*-* 03:15:00
# Run after a missed window, e.g. because the Pi was off.
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
---- systemctl cat holzcloud-backup.service ----
# /etc/systemd/system/holzcloud-backup.service
[Unit]
Description=holzcloud-CMS backup
Documentation=file:///opt/holzcloud/deploy/DEPLOY.md
After=holzcloud.service

[Service]
Type=oneshot
User=holzcloud
Group=holzcloud
ExecStart=/opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
Environment=DATA_DIR=/opt/holzcloud/data
Environment=HOLZCLOUD_BIN=/opt/holzcloud/holzcloud
Environment=RETENTION_DAYS=30
# Uncomment to copy off the box — a backup on the same disk does not survive
# the disk failure it exists for.
# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups
PrivateTmp=true
```

#### amd64-t24-src/fire.txt

```text
EXEC-START-BEFORE: empty
CLOCK-SKEW-S-BEFORE: 0
ZONE: Nightly/Phase24 STDOFF -7:25:31 OFFSET-S -26731 READ 03:13:30 LEAD-S 90
BASE-EPOCH: 1793011231
NEXT-ARMED-EPOCH: 1793011362
RANDOM-DELAY-S: 300
ACCURACY-S: 60
SLACK-S: 5
STAMP-BEFORE: 1793011112 2026-10-26 03:13:01.563494010 -0725
PASS NextElapseUSecRealtime (1793011362) is within two days of now (1793011142)
START-EPOCH: 1793011250
START-LOCAL: 03:15:19
LAST-ARMED-EPOCH: 1793011249
LAST-DRAWN-EPOCH: 1793011241
DRAWS-LOGGED: 6
TIME-CHANGED-LOGGED: 4
END-EPOCH: 1793011251
TRIGGERED-BY: holzcloud-backup.timer
RESULT: success
EXIT-STATUS: 0
INVOCATION-ID: fbbc5929f994452fa2e6333b5f9efee4
DIR: /opt/holzcloud/backups/20261026-031519
DIR-OWNER-MODE: holzcloud 700
STAMP-AFTER: 1793011250 2026-10-26 03:15:19.228982000 -0725
CLOCK-SKEW-S-AFTER: 0
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
PASS the service was started by holzcloud-backup.timer (TriggeredBy), not by hand
PASS Result success and ExecMainStatus 0
PASS the journal of the invocation holds the script's '=== Backup complete: /opt/holzcloud/backups/20261026-031519 ===' line
PASS the directory the journal names exists: /opt/holzcloud/backups/20261026-031519
PASS owner and mode of the backup directory: holzcloud 700 (the hardened unit runs as the service user)
PASS the run started at 03:15:19 local, at or after 03:15:00 and before 03:25:00
PASS the container clock stayed within 120 s of the host's (before 0 s, after 0 s)
PASS PID 1 logged the draw the timer fired on: elapse 1793011249, drawn at 1793011241 (fire-armed-log.txt)
PASS the stamp /var/lib/systemd/timers/stamp-holzcloud-backup.timer exists after the run (1793011250 2026-10-26 03:15:19.228982000 -0725)
UNITS-AND-BACKUP: no BAD line in units-shipped.txt and verify-fire.txt
FIRE OK
```

#### amd64-t24-src/fire-before.txt

```text
NEXT                        LEFT          LAST PASSED UNIT                   ACTIVATES
Mon 2026-10-26 03:17:11 P24 3min 40s left -    -      holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-t24-src/fire-after.txt

```text
NEXT                        LEFT     LAST                        PASSED UNIT                   ACTIVATES
Tue 2026-10-27 03:16:55 P24 24h left Mon 2026-10-26 03:15:19 P24 1s ago holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-t24-src/fire-journal.txt

```text
Oct 26 03:15:19.241911 0c141912becc (ackup.sh)[3954]: /run/systemd/unit-root/home (rw-implicit) is duplicate.
Oct 26 03:15:19.242039 0c141912becc (ackup.sh)[3954]: /run/systemd/unit-root/root (rw-implicit) is duplicate.
Oct 26 03:15:19.242050 0c141912becc (ackup.sh)[3954]: /run/systemd/unit-root/run/user (rw-implicit) is duplicate.
Oct 26 03:15:19.242729 0c141912becc (ackup.sh)[3954]: Bind-mounting / on /run/systemd/unit-root (MS_BIND|MS_REC "")...
Oct 26 03:15:19.242847 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/
Oct 26 03:15:19.242975 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/dev
Oct 26 03:15:19.243001 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/home
Oct 26 03:15:19.243566 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/inaccessible/dir on /run/systemd/unit-root/home (MS_BIND|MS_REC "")...
Oct 26 03:15:19.243591 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/inaccessible/dir to /run/systemd/unit-root/home
Oct 26 03:15:19.243611 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/opt/holzcloud/backups
Oct 26 03:15:19.243625 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/unit-root/opt/holzcloud/backups on /run/systemd/unit-root/opt/holzcloud/backups (MS_BIND|MS_REC "")...
Oct 26 03:15:19.243637 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/unit-root/opt/holzcloud/backups to /run/systemd/unit-root/opt/holzcloud/backups
Oct 26 03:15:19.243652 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/opt/holzcloud/data
Oct 26 03:15:19.243663 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/unit-root/opt/holzcloud/data on /run/systemd/unit-root/opt/holzcloud/data (MS_BIND|MS_REC "")...
Oct 26 03:15:19.243675 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/unit-root/opt/holzcloud/data to /run/systemd/unit-root/opt/holzcloud/data
Oct 26 03:15:19.243689 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/proc
Oct 26 03:15:19.243703 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/root
Oct 26 03:15:19.243846 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/inaccessible/dir on /run/systemd/unit-root/root (MS_BIND|MS_REC "")...
Oct 26 03:15:19.243865 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/inaccessible/dir to /run/systemd/unit-root/root
Oct 26 03:15:19.243886 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/run/credentials
Oct 26 03:15:19.244010 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/inaccessible/dir on /run/systemd/unit-root/run/credentials (MS_BIND|MS_REC "")...
Oct 26 03:15:19.244027 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/inaccessible/dir to /run/systemd/unit-root/run/credentials
Oct 26 03:15:19.244043 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/run/systemd/incoming
Oct 26 03:15:19.244201 0c141912becc (ackup.sh)[3954]: Followed source symlinks /run/systemd/propagate/holzcloud-backup.service → /run/systemd/propagate/holzcloud-backup.service.
Oct 26 03:15:19.244217 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/propagate/holzcloud-backup.service on /run/systemd/unit-root/run/systemd/incoming (MS_BIND "")...
Oct 26 03:15:19.244230 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/propagate/holzcloud-backup.service to /run/systemd/unit-root/run/systemd/incoming
Oct 26 03:15:19.244244 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/run/user
Oct 26 03:15:19.244393 0c141912becc (ackup.sh)[3954]: Bind-mounting /run/systemd/inaccessible/dir on /run/systemd/unit-root/run/user (MS_BIND|MS_REC "")...
Oct 26 03:15:19.244414 0c141912becc (ackup.sh)[3954]: Successfully mounted /run/systemd/inaccessible/dir to /run/systemd/unit-root/run/user
Oct 26 03:15:19.244427 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/sys
Oct 26 03:15:19.244440 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/tmp
Oct 26 03:15:19.244493 0c141912becc (ackup.sh)[3954]: Bind-mounting /tmp/systemd-private-8ad2d9c707d0429d984d2ea90b782d85-holzcloud-backup.service-FsYiHi/tmp on /run/systemd/unit-root/tmp (MS_BIND|MS_REC "")...
Oct 26 03:15:19.244515 0c141912becc (ackup.sh)[3954]: Successfully mounted /tmp/systemd-private-8ad2d9c707d0429d984d2ea90b782d85-holzcloud-backup.service-FsYiHi/tmp to /run/systemd/unit-root/tmp
Oct 26 03:15:19.244530 0c141912becc (ackup.sh)[3954]: Applying namespace mount on /run/systemd/unit-root/var/tmp
Oct 26 03:15:19.244540 0c141912becc (ackup.sh)[3954]: Bind-mounting /var/tmp/systemd-private-8ad2d9c707d0429d984d2ea90b782d85-holzcloud-backup.service-lvS774/tmp on /run/systemd/unit-root/var/tmp (MS_BIND|MS_REC "")...
Oct 26 03:15:19.244555 0c141912becc (ackup.sh)[3954]: Successfully mounted /var/tmp/systemd-private-8ad2d9c707d0429d984d2ea90b782d85-holzcloud-backup.service-lvS774/tmp to /run/systemd/unit-root/var/tmp
Oct 26 03:15:19.248334 0c141912becc (ackup.sh)[3954]: mount_setattr() failed, falling back to classic remounting: Function not implemented
Oct 26 03:15:19.253720 0c141912becc (ackup.sh)[3954]: holzcloud-backup.service: Executing: /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
Oct 26 03:15:19.331229 0c141912becc backup.sh[3954]: === holzcloud-CMS backup: 20261026-031519 ===
Oct 26 03:15:19.411335 0c141912becc backup.sh[3954]: Backing up database...
Oct 26 03:15:19.529553 0c141912becc backup.sh[3968]: 2026/10/26 03:15:19 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 26 03:15:19.569186 0c141912becc backup.sh[3968]: 2026/10/26 03:15:19 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-031519/holzcloud.sqlite
Oct 26 03:15:19.582306 0c141912becc backup.sh[3968]: 2026/10/26 03:15:19 INFO backup verified path=/opt/holzcloud/backups/20261026-031519/holzcloud.sqlite tables=57
Oct 26 03:15:19.583879 0c141912becc backup.sh[3968]: backup written and verified: /opt/holzcloud/backups/20261026-031519/holzcloud.sqlite
Oct 26 03:15:19.585419 0c141912becc backup.sh[3954]: Backing up CSRF key...
Oct 26 03:15:19.603710 0c141912becc backup.sh[3954]: Backing up media...
Oct 26 03:15:19.705581 0c141912becc backup.sh[3954]: Backing up plugins...
Oct 26 03:15:19.828700 0c141912becc backup.sh[3954]: Backing up sprachen...
Oct 26 03:15:19.930771 0c141912becc backup.sh[3954]: Backing up marke...
Oct 26 03:15:20.122708 0c141912becc backup.sh[3954]: Pruning backups older than 30 days...
Oct 26 03:15:20.147034 0c141912becc backup.sh[3954]: === Backup complete: /opt/holzcloud/backups/20261026-031519 ===
Oct 26 03:15:20.164191 0c141912becc backup.sh[4006]: 20M        /opt/holzcloud/backups/20261026-031519
```

#### amd64-t24-src/verify-fire.txt

```text
PASS file /opt/holzcloud/backups/20261026-031519/holzcloud.sqlite exists
PASS integrity holzcloud-check: ok
PASS integrity sqlite3 integrity_check: ok
PASS MANIFEST timestamp=20261026-031519
PASS MANIFEST version=holzcloud v0.1.0-31-gb501b1b (b501b1b)
PASS MANIFEST data_dir=/opt/holzcloud/data
PASS MANIFEST host=0c141912becc
PASS owner and mode of the backup directory: holzcloud 700
PASS csrf.key is in the backup
MEDIA-FILES-BACKUP: 3
MEDIA-FILES-DATA: 3
PASS the backup holds as many media files as the data directory (3)
```

#### amd64-t24-src/delay.txt

```text
RANDOM-DELAY-S: 300
ACCURACY-S: 60
FIXED-RANDOM-DELAY: no
PERSISTENT: yes
PASS the installed holzcloud-backup.timer holds the line RandomizedDelaySec=300
INSTALLED-LINE: RandomizedDelaySec=300
PASS the running timer reports RandomizedDelayUSec of 300 s
PASS Persistent is yes
SAMPLE 1 1793097631 1793097929 298
SAMPLE 2 1793097631 1793097833 202
SAMPLE 3 1793097631 1793097811 180
SAMPLE 4 1793097631 1793097662 31
SAMPLE 5 1793097631 1793097684 53
SAMPLE 6 1793097631 1793097784 153
SAMPLE 7 1793097631 1793097910 279
SAMPLE 8 1793097631 1793097862 231
SAMPLE 9 1793097631 1793097915 284
SAMPLE 10 1793097631 1793097887 256
SAMPLE 11 1793097631 1793097821 190
SAMPLE 12 1793097631 1793097912 281
D-MIN: 31
D-MAX: 298
D-DISTINCT: 12
DELAY-SAMPLES: 12
PASS 12 re-armings gave delays between 31 and 298 s, 12 distinct values, all inside [0, 300] s
STAMP-MTIME-BEFORE-SAMPLES: 1793011250
STAMP-MTIME-AFTER-SAMPLES: 1793011250
PASS the stamp's mtime did not move during the restarts (1793011250)
---- systemctl show holzcloud-backup.timer ----
Unit=holzcloud-backup.service
TimersCalendar={ OnCalendar=*-*-* 03:15:00 ; next_elapse=Tue 2026-10-27 03:15:00 P24 }
OnClockChange=no
OnTimezoneChange=no
NextElapseUSecRealtime=Tue 2026-10-27 03:19:41 P24
NextElapseUSecMonotonic=0
LastTriggerUSec=Mon 2026-10-26 03:15:19 P24
LastTriggerUSecMonotonic=0
Result=success
AccuracyUSec=1min
RandomizedDelayUSec=5min
FixedRandomDelay=no
Persistent=yes
WakeSystem=no
RemainAfterElapse=yes
Id=holzcloud-backup.timer
Names=holzcloud-backup.timer
Requires=sysinit.target
WantedBy=timers.target
Conflicts=shutdown.target
Before=holzcloud-backup.service timers.target shutdown.target
After=sysinit.target time-sync.target -.mount time-set.target
Triggers=holzcloud-backup.service
RequiresMountsFor=/var/lib/systemd/timers
Documentation=file:///opt/holzcloud/deploy/DEPLOY.md
Description=Nightly holzcloud-CMS backup
LoadState=loaded
ActiveState=active
FreezerState=running
SubState=waiting
FragmentPath=/etc/systemd/system/holzcloud-backup.timer
UnitFileState=enabled
UnitFilePreset=enabled
StateChangeTimestamp=Mon 2026-10-26 03:16:01 P24
StateChangeTimestampMonotonic=2121550815
InactiveExitTimestamp=Mon 2026-10-26 03:16:01 P24
InactiveExitTimestampMonotonic=2121550815
ActiveEnterTimestamp=Mon 2026-10-26 03:16:01 P24
ActiveEnterTimestampMonotonic=2121550815
ActiveExitTimestamp=Mon 2026-10-26 03:16:01 P24
ActiveExitTimestampMonotonic=2121550663
InactiveEnterTimestamp=Mon 2026-10-26 03:16:01 P24
InactiveEnterTimestampMonotonic=2121550663
CanStart=yes
CanStop=yes
CanReload=no
CanIsolate=no
CanClean=state
CanFreeze=no
StopWhenUnneeded=no
RefuseManualStart=no
RefuseManualStop=no
AllowIsolate=no
DefaultDependencies=yes
OnSuccessJobMode=fail
OnFailureJobMode=replace
IgnoreOnIsolate=no
NeedDaemonReload=no
JobTimeoutUSec=infinity
JobRunningTimeoutUSec=infinity
JobTimeoutAction=none
ConditionResult=yes
AssertResult=yes
ConditionTimestamp=Mon 2026-10-26 03:16:01 P24
ConditionTimestampMonotonic=2121550701
AssertTimestamp=Mon 2026-10-26 03:16:01 P24
AssertTimestampMonotonic=2121550704
Transient=no
Perpetual=no
StartLimitIntervalUSec=10s
StartLimitBurst=5
StartLimitAction=none
FailureAction=none
SuccessAction=none
InvocationID=95b876b148514279b06561001c19b122
CollectMode=inactive
---- systemctl list-timers holzcloud-backup.timer ----
NEXT                        LEFT     LAST                        PASSED  UNIT                   ACTIVATES
Tue 2026-10-27 03:19:41 P24 24h left Mon 2026-10-26 03:15:19 P24 45s ago holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
OBSERVE-S: 75
REROLLS-WITHOUT-RESTART: 3
TIME-CHANGED-LOGGED: 3
PID1 Oct 26 03:16:10.415901 0c141912becc systemd[1]: Time has been changed
PID1 Oct 26 03:16:10.416519 0c141912becc systemd[1]: holzcloud-backup.timer: Time change, recalculating next elapse.
PID1 Oct 26 03:16:10.416575 0c141912becc systemd[1]: holzcloud-backup.timer: Adding 4min 38.679971s random time.
PID1 Oct 26 03:16:10.416587 0c141912becc systemd[1]: holzcloud-backup.timer: Realtime timer elapses at Tue 2026-10-27 03:19:38 P24.
PID1 Oct 26 03:16:40.423542 0c141912becc systemd[1]: Time has been changed
PID1 Oct 26 03:16:40.424066 0c141912becc systemd[1]: holzcloud-backup.timer: Time change, recalculating next elapse.
PID1 Oct 26 03:16:40.424115 0c141912becc systemd[1]: holzcloud-backup.timer: Adding 2min 35.104605s random time.
PID1 Oct 26 03:16:40.424126 0c141912becc systemd[1]: holzcloud-backup.timer: Realtime timer elapses at Tue 2026-10-27 03:17:35 P24.
PID1 Oct 26 03:17:10.433956 0c141912becc systemd[1]: Time has been changed
PID1 Oct 26 03:17:10.434483 0c141912becc systemd[1]: holzcloud-backup.timer: Time change, recalculating next elapse.
PID1 Oct 26 03:17:10.434531 0c141912becc systemd[1]: holzcloud-backup.timer: Adding 4min 12.408220s random time.
PID1 Oct 26 03:17:10.434541 0c141912becc systemd[1]: holzcloud-backup.timer: Realtime timer elapses at Tue 2026-10-27 03:19:12 P24.
INFO the shipped timer re-armed itself 3 time(s) in 75 s without a restart; systemd logged 'Time has been changed' 3 time(s): the Docker VM steps the realtime clock and every step draws a new random delay (the armed time in fire.txt is the draw PID 1 logged last before the run, see fire-armed-log.txt)
SLACK-S: 5
DELAY-USED-S: 18
START-AFTER-ARMED-S: 1
DRAWN-AFTER-ARMED-S: -8
PASS the real run: armed 18 s after 03:15:00 (inside [0, 300]), started 1 s after the armed time, the draw made -8 s after it (inside [0, accuracy 60 + slack 5] of whichever is later)
PASS the real run started at 03:15:19 local, inside [03:15:00, 03:21:05] (03:15:00 + 300 s delay + 60 s accuracy + 5 s allowance)
DELAY OK
```

#### amd64-t24-src/catchup.txt

```text
CLOCK-SKEW-S-BEFORE: 0
RANDOM-DELAY-S: 300
ACCURACY-S: 60
SLACK-S: 30
ZONE: Nightly/Catchup STDOFF 16:29:37 OFFSET-S 59377 READ 03:12:31 LEAD-S 150
STAMP-BEFORE-STOP: 1793011250
MISSED-EPOCH: 1793011523
MISSED-LOCAL: 03:15:00
CONTROL-NEXT-EPOCH: 1793011539
PASS the control timer is armed for the same missed instant (next 1793011539, missed 1793011523)
TEST-IMAGE-KEPT-FOR-THE-STOP: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
TEST-IMAGE-AFTER-THE-START: gone from /tmp with the restart, put back from /root: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
TMP-AT-BOOT: tmpfiles.d says 'D /tmp 1777 root root -'; /tmp has a mount of its own: no
CLOCK-BEFORE-STOP: 1793011375
FINISHED-AT-RAW: 2026-10-26T10:42:55.59553159Z
STARTED-AT-RAW: 2026-10-26T10:46:03.780971218Z
DOWN-FROM: 1793011375
UP-AT: 1793011563
CLOCK-AFTER-START: 1793011565
TIMER-ACTIVE-EPOCH: 1793011564
START-EPOCH: 1793011721
START-LOCAL: 03:18:18
TRIGGERED-BY: holzcloud-backup.timer
RESULT: success
EXIT-STATUS: 0
INVOCATION-ID: 26d9877c0e6d4e3ba2e826e5ab26d695
DIR: /opt/holzcloud/backups/20261027-031818
DELAY-AFTER-BOOT-S: 157
STAMP-AFTER: 1793011721 2026-10-27 03:18:18.543081000 +1629
CONTROL-RAN: no
CONTROL-PERSISTENT: no
CLOCK-SKEW-S-AFTER: 0
PASS a missed window was run after the boot: stamp 1793011250 < missed 1793011523, down at 1793011375, up at 1793011563, timer active at 1793011564, run at 1793011721 (157 s after the activation; limit max(missed, activation) + 300 + 60 + 30)
PASS the service was started by holzcloud-backup.timer (TriggeredBy), not by hand
PASS Result success and ExecMainStatus 0
PASS the directory the journal names exists: /opt/holzcloud/backups/20261027-031818
PASS the control timer (same calendar, Persistent=no) did not run after the same stop and start: the catch-up belongs to the shipped Persistent=true
PASS the container clock stayed within 120 s of the host's (before 0 s, after 0 s)
ZONE-RESTORED: yes (/etc/localtime -> /usr/share/zoneinfo/Etc/UTC, now UTC)
PASS the zone is restored and the shipped timer followed
CATCHUP OK
```

#### amd64-t24-src/catchup-before.txt

```text
---- list-timers before the zone moves ----
NEXT                        LEFT     LAST                        PASSED      UNIT                   ACTIVATES
Tue 2026-10-27 03:19:12 P24 24h left Mon 2026-10-26 03:15:19 P24 2min 2s ago holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
---- the stamp ----
/var/lib/systemd/timers/stamp-holzcloud-backup.timer 1793011250 2026-10-26 03:15:19.228982000 -0725
---- systemctl show holzcloud-backup.timer ----
TimersCalendar={ OnCalendar=*-*-* 03:15:00 ; next_elapse=Tue 2026-10-27 03:15:00 P24 }
LastTriggerUSec=Mon 2026-10-26 03:15:19 P24
AccuracyUSec=1min
RandomizedDelayUSec=5min
Persistent=yes
DropInPaths=
ActiveEnterTimestamp=Mon 2026-10-26 03:16:01 P24
---- list-timers after the zone moved, before the stop ----
NEXT                        LEFT          LAST                        PASSED      UNIT                   ACTIVATES
Tue 2026-10-27 03:15:16 P24 2min 44s left -                           -           p24-control.timer      p24-control.service
Tue 2026-10-27 03:18:15 P24 5min left     Tue 2026-10-27 03:10:27 P24 2min 4s ago holzcloud-backup.timer holzcloud-backup.service

2 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-t24-src/catchup-after.txt

```text
NEXT                        LEFT     LAST                        PASSED  UNIT                   ACTIVATES
Wed 2026-10-28 03:18:26 P24 23h left Tue 2026-10-27 03:18:18 P24 31s ago holzcloud-backup.timer holzcloud-backup.service
Wed 2026-10-28 03:18:52 P24 24h left -                           -       p24-control.timer      p24-control.service

2 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-t24-src/catchup-journal.txt

```text
Oct 27 03:18:18.643687 0c141912becc backup.sh[1712]: === holzcloud-CMS backup: 20261027-031818 ===
Oct 27 03:18:18.721765 0c141912becc backup.sh[1712]: Backing up database...
Oct 27 03:18:18.842173 0c141912becc backup.sh[1720]: 2026/10/27 03:18:18 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 27 03:18:18.881748 0c141912becc backup.sh[1720]: 2026/10/27 03:18:18 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-031818/holzcloud.sqlite
Oct 27 03:18:18.894847 0c141912becc backup.sh[1720]: 2026/10/27 03:18:18 INFO backup verified path=/opt/holzcloud/backups/20261027-031818/holzcloud.sqlite tables=57
Oct 27 03:18:18.896373 0c141912becc backup.sh[1720]: backup written and verified: /opt/holzcloud/backups/20261027-031818/holzcloud.sqlite
Oct 27 03:18:18.898154 0c141912becc backup.sh[1712]: Backing up CSRF key...
Oct 27 03:18:18.916326 0c141912becc backup.sh[1712]: Backing up media...
Oct 27 03:18:19.017932 0c141912becc backup.sh[1712]: Backing up plugins...
Oct 27 03:18:19.147091 0c141912becc backup.sh[1712]: Backing up sprachen...
Oct 27 03:18:19.256904 0c141912becc backup.sh[1712]: Backing up marke...
Oct 27 03:18:19.452999 0c141912becc backup.sh[1712]: Pruning backups older than 30 days...
Oct 27 03:18:19.477609 0c141912becc backup.sh[1712]: === Backup complete: /opt/holzcloud/backups/20261027-031818 ===
Oct 27 03:18:19.494902 0c141912becc backup.sh[1770]: 20M        /opt/holzcloud/backups/20261027-031818
```

#### amd64-t24-src/verify-catchup.txt

```text
PASS file /opt/holzcloud/backups/20261027-031818/holzcloud.sqlite exists
PASS integrity holzcloud-check: ok
PASS integrity sqlite3 integrity_check: ok
PASS MANIFEST timestamp=20261027-031818
PASS MANIFEST version=holzcloud v0.1.0-31-gb501b1b (b501b1b)
PASS MANIFEST data_dir=/opt/holzcloud/data
PASS MANIFEST host=0c141912becc
PASS owner and mode of the backup directory: holzcloud 700
PASS csrf.key is in the backup
MEDIA-FILES-BACKUP: 3
MEDIA-FILES-DATA: 3
PASS the backup holds as many media files as the data directory (3)
```

#### amd64-t24-src/prune.txt

```text
RETENTION-DAYS-FROM-UNIT: 30
ENVIRONMENT-OF-UNIT: DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30
PASS the installed units are byte-equal to deploy/, RETENTION_DAYS=30 is read from the unit and REMOTE_TARGET is not set
MUST-STAY: 20261026-031519 20261026-103806 20261026-103832 20261027-031818
PLANTED 20260911-104917 3888000 DELETE
PLANTED-DETAIL 20260911-104917 45 d 0 h old, owner and mode holzcloud 700, holds 2 files, one of them in a subdirectory
PLANTED 20260925-094917 2682000 DELETE
PLANTED-DETAIL 20260925-094917 31 d 1 h old, owner and mode holzcloud 700, holds 2 files, one of them in a subdirectory
PLANTED 20260925-224917 2635200 KEEP
PLANTED-DETAIL 20260925-224917 30 d 12 h old, owner and mode holzcloud 700, holds 2 files, one of them in a subdirectory
PLANTED 20260927-104917 2505600 KEEP
PLANTED-DETAIL 20260927-104917 29 d 0 h old, owner and mode holzcloud 700, holds 2 files, one of them in a subdirectory
PLANTED 20261025-104917 86400 KEEP
PLANTED-DETAIL 20261025-104917 1 d 0 h old, owner and mode holzcloud 700, holds 2 files, one of them in a subdirectory
START-BY: systemctl start holzcloud-backup.service by hand, the oneshot returns when the script is done; the shipped timer was proven in plan 01
START-RC: 0
RESULT: success
EXIT-STATUS: 0
INVOCATION-ID: d1339adae78d45dbb0cef5fede1269a7
DIR: /opt/holzcloud/backups/20261026-104919
PASS the run is a new invocation of the service (d1339adae78d45dbb0cef5fede1269a7)
PASS Result success and ExecMainStatus 0
PASS the journal of the invocation holds the script's 'Pruning backups older than 30 days...' line
PASS the journal holds the script's '=== Backup complete: /opt/holzcloud/backups/20261026-104919 ===' line
OBSERVED 20260911-104917 gone
PASS planted 20260911-104917 (45 d 0 h old, retention 30 d): expected DELETE, observed gone
OBSERVED 20260925-094917 gone
PASS planted 20260925-094917 (31 d 1 h old, retention 30 d): expected DELETE, observed gone
OBSERVED 20260925-224917 present
PASS planted 20260925-224917 (30 d 12 h old, retention 30 d): expected KEEP, observed present
OBSERVED 20260927-104917 present
PASS planted 20260927-104917 (29 d 0 h old, retention 30 d): expected KEEP, observed present
OBSERVED 20261025-104917 present
PASS planted 20261025-104917 (1 d 0 h old, retention 30 d): expected KEEP, observed present
PASS every directory that existed before the planting is still there (4)
DIR-OWNER-MODE: holzcloud 700
DIR-AGE-S: 2
PASS the new backup /opt/holzcloud/backups/20261026-104919 exists, is holzcloud 700 and 2 s old
OBSERVED-BOUNDARY: the youngest planted directory that was deleted was 31 d 1 h old, the oldest that was kept was 30 d 12 h old (retention 30 days)
INFO OBSERVED RULE: a directory is deleted once floor(age / 24 h) exceeds RETENTION_DAYS, which is from 31 days of age; the 30 day 12 hour directory, older than 30 days by less than a day, was kept; by the same rule it becomes eligible 12 hours later (a later run was not played). This is a measured fact; whether it is a defect is plan 03's rule
CLEANUP removed 20260925-224917
CLEANUP removed 20260927-104917
CLEANUP removed 20261025-104917
CLEANUP-COUNT: 3
PRUNE OK
```

#### amd64-t24-src/prune-before.txt

```text
---- container clock (epoch): 1793011757
---- ls -l --time-style=full-iso /opt/holzcloud/backups
total 16
drwx------ 6 holzcloud holzcloud 4096 2026-10-26 10:40:51.031159004 +0000 20261026-031519
drwx------ 4 holzcloud holzcloud 4096 2026-10-26 10:38:07.361848012 +0000 20261026-103806
drwx------ 4 holzcloud holzcloud 4096 2026-10-26 10:38:33.418494010 +0000 20261026-103832
drwx------ 6 holzcloud holzcloud 4096 2026-10-26 10:48:42.359627000 +0000 20261027-031818
---- stat -c '%n %Y' /opt/holzcloud/backups
/opt/holzcloud/backups 1793011721
---- directories that exist before the planting (the must-stay set)
20261026-031519
20261026-103806
20261026-103832
20261027-031818
```

#### amd64-t24-src/prune-after.txt

```text
---- container clock (epoch): 1793011761
---- ls -l --time-style=full-iso /opt/holzcloud/backups
total 32
drwx------ 3 holzcloud holzcloud 4096 2026-09-25 22:49:17.000000000 +0000 20260925-224917
drwx------ 3 holzcloud holzcloud 4096 2026-09-27 10:49:17.000000000 +0000 20260927-104917
drwx------ 3 holzcloud holzcloud 4096 2026-10-25 10:49:17.000000000 +0000 20261025-104917
drwx------ 6 holzcloud holzcloud 4096 2026-10-26 10:40:51.031159004 +0000 20261026-031519
drwx------ 4 holzcloud holzcloud 4096 2026-10-26 10:38:07.361848012 +0000 20261026-103806
drwx------ 4 holzcloud holzcloud 4096 2026-10-26 10:38:33.418494010 +0000 20261026-103832
drwx------ 6 holzcloud holzcloud 4096 2026-10-26 10:49:20.540471004 +0000 20261026-104919
drwx------ 6 holzcloud holzcloud 4096 2026-10-26 10:48:42.359627000 +0000 20261027-031818
---- stat -c '%n %Y' /opt/holzcloud/backups
/opt/holzcloud/backups 1793011760
```

#### amd64-t24-src/prune-journal.txt

```text
Oct 26 10:49:19.829864 0c141912becc backup.sh[2221]: === holzcloud-CMS backup: 20261026-104919 ===
Oct 26 10:49:19.911121 0c141912becc backup.sh[2221]: Backing up database...
Oct 26 10:49:20.031447 0c141912becc backup.sh[2229]: 2026/10/26 10:49:20 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 26 10:49:20.069955 0c141912becc backup.sh[2229]: 2026/10/26 10:49:20 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-104919/holzcloud.sqlite
Oct 26 10:49:20.082798 0c141912becc backup.sh[2229]: 2026/10/26 10:49:20 INFO backup verified path=/opt/holzcloud/backups/20261026-104919/holzcloud.sqlite tables=57
Oct 26 10:49:20.084351 0c141912becc backup.sh[2229]: backup written and verified: /opt/holzcloud/backups/20261026-104919/holzcloud.sqlite
Oct 26 10:49:20.086133 0c141912becc backup.sh[2221]: Backing up CSRF key...
Oct 26 10:49:20.104215 0c141912becc backup.sh[2221]: Backing up media...
Oct 26 10:49:20.207700 0c141912becc backup.sh[2221]: Backing up plugins...
Oct 26 10:49:20.332072 0c141912becc backup.sh[2221]: Backing up sprachen...
Oct 26 10:49:20.436544 0c141912becc backup.sh[2221]: Backing up marke...
Oct 26 10:49:20.636512 0c141912becc backup.sh[2221]: Pruning backups older than 30 days...
Oct 26 10:49:20.679533 0c141912becc backup.sh[2221]: === Backup complete: /opt/holzcloud/backups/20261026-104919 ===
Oct 26 10:49:20.697170 0c141912becc backup.sh[2268]: 20M        /opt/holzcloud/backups/20261026-104919
```

#### amd64-t24-src/offsite.txt

```text
HOST2: holzcloud-t24-host2 at 172.17.0.3
SSH-CLIENT-ON-SOURCE-BEFORE: absent
SSH-SERVER-ON-SECOND-HOST-BEFORE: absent
RSYNC-SOURCE: rsync  version 3.2.7  protocol version 32
RSYNC-HOST2: rsync  version 3.2.7  protocol version 32
SSH-CLIENT: OpenSSH_9.2p1 Debian-2+deb12u10, OpenSSL 3.0.22 25 Aug 2026
SSH-SERVER: openssh-server	1:9.2p1-2+deb12u10
UNIT-DIFF-LINES: 1
UNIT-DIFF 16c16
UNIT-DIFF < # Environment=REMOTE_TARGET=user@nas:/backups/holzcloud
UNIT-DIFF ---
UNIT-DIFF > Environment=REMOTE_TARGET=offsite@nas:/backups/holzcloud
REMOTE-TARGET: offsite@nas:/backups/holzcloud
PASS the installed service differs from the repository file in exactly one line
PASS holzcloud-backup.service has no drop-in
SERVICE-USER-HOME: /home/holzcloud (exists by default: no)
A-OUTSIDE-EXIT: 0
A-OUTSIDE-DIR: 20261026-104954 (on the second host: yes)
A-SANDBOX-RESULT: start-rc 1 Result exit-code ExecMainStatus 255 directory 20261026-104957 on the second host: no
FINDING: the key in /home/holzcloud/.ssh does not work under the shipped sandbox: start-rc 1, Result 'exit-code', ExecMainStatus '255', directory on the second host: no (offsite-journal.txt); outside the sandbox the same script ended with status 0 and the directory arrived: yes (offsite-natural.txt, A3)
B-RESULT: start-rc 0 Result success ExecMainStatus 0 directory 20261026-105004 on the second host: yes
WORKING-CREDENTIALS: offsite-ssh
RESULT: success
EXIT-STATUS: 0
COPY-DIR: /backups/holzcloud/20261026-105004
PASS credentials 'offsite-ssh': the shipped service copied 20261026-105004 to holzcloud-t24-host2 (Result success, ExecMainStatus 0)
PASS the final run of the service: Result success, ExecMainStatus 0
FILES-COMPARED: 158
PASS the sha256 of every file under /opt/holzcloud/backups of the source (158 files) equals that of /backups/holzcloud on the second host
PASS the same 9 backup directories on both hosts
PASS every copied directory on the second host is owned by offsite with mode 700 (offsite-modes.txt)
INFO MIRROR: a stray directory planted on the second host before the final run is gone after it: rsync --delete makes the off-site copy a mirror of the backup root, so what the source prunes or loses is lost off-site too
INFO NOT PLAYED: the pull by rsync -a that the document describes for a rebuilt server; offsite-restore restores on the second host itself
OFFSITE OK
```

#### amd64-t24-src/offsite-natural.txt

```text
Experiment A: credentials where an operator first puts them, in the service user's home
==== A1 getent passwd holzcloud
$ dx getent passwd holzcloud
holzcloud:x:997:997::/home/holzcloud:/usr/sbin/nologin
[exit 0]

the home directory field is /home/holzcloud, it exists before the experiment: no
==== A1 ssh-keygen -t ed25519 -N '' as holzcloud, stdin closed (the operator's first try)
$ dx runuser -u holzcloud -- ssh-keygen -t ed25519 -N 
Generating public/private ed25519 key pair.
Enter file in which to save the key (/home/holzcloud/.ssh/id_ed25519): 
[exit 1]

A2: the operator creates the home directory /home/holzcloud (holzcloud, 750) and /home/holzcloud/.ssh (700)
==== A2 ssh-keygen -f /home/holzcloud/.ssh/id_ed25519 as holzcloud
$ dx runuser -u holzcloud -- ssh-keygen -q -t ed25519 -N  -f /home/holzcloud/.ssh/id_ed25519 -C holzcloud-backup
[exit 0]

==== A2 ssh-keyscan -t ed25519 nas into known_hosts (first use, DEVIATION[credentials])
$ dx runuser -u holzcloud -- bash -c ssh-keyscan -t ed25519 nas > "$1"; test -s "$1" _ /home/holzcloud/.ssh/known_hosts
# nas:22 SSH-2.0-OpenSSH_9.2p1 Debian-2+deb12u10
[exit 0]

==== A3 the same backup.sh once as holzcloud OUTSIDE the sandbox, REMOTE_TARGET=offsite@nas:/backups/holzcloud
$ dx runuser -u holzcloud -- env DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30 REMOTE_TARGET=offsite@nas:/backups/holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
=== holzcloud-CMS backup: 20261026-104954 ===
Backing up database...
2026/10/26 10:49:54 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/10/26 10:49:54 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-104954/holzcloud.sqlite
2026/10/26 10:49:54 INFO backup verified path=/opt/holzcloud/backups/20261026-104954/holzcloud.sqlite tables=57
backup written and verified: /opt/holzcloud/backups/20261026-104954/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Pruning backups older than 30 days...
Copying to offsite@nas:/backups/holzcloud...
=== Backup complete: /opt/holzcloud/backups/20261026-104954 ===
20M	/opt/holzcloud/backups/20261026-104954
[exit 0]

A4: the shipped service, ProtectHome=true, ProtectSystem=strict, started by hand (the timer was proven in plan 01)
A4 failed. Isolating the cause with two transient units, as the service user, with the unit's own properties:
==== A5 transient unit WITH ProtectHome=true, ProtectSystem=strict, NoNewPrivileges, PrivateTmp, ReadWritePaths: ls -la /home/holzcloud/.ssh
$ dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectHome=true -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups ls -la /home/holzcloud/.ssh
/usr/bin/ls: cannot access '/home/holzcloud/.ssh': Permission denied
[exit 2]

==== A5 transient unit WITHOUT ProtectHome (the rest as above): ls -la /home/holzcloud/.ssh
$ dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups ls -la /home/holzcloud/.ssh
total 24
drwx------ 2 holzcloud holzcloud 4096 Oct 26 10:49 .
drwxr-x--- 4 holzcloud holzcloud 4096 Oct 26 10:49 ..
-rw------- 1 holzcloud holzcloud  411 Oct 26 10:49 id_ed25519
-rw-r--r-- 1 holzcloud holzcloud   98 Oct 26 10:49 id_ed25519.pub
-rw------- 1 holzcloud holzcloud  921 Oct 26 10:49 known_hosts
-rw-r--r-- 1 holzcloud holzcloud   85 Oct 26 10:49 known_hosts.old
[exit 0]

==== A5 transient unit WITH the shipped sandbox: ssh -o BatchMode=yes offsite@nas true
$ dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectHome=true -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups ssh -o BatchMode=yes offsite@nas true
hostkeys_find_by_key_hostfile: hostkeys_foreach failed for /home/holzcloud/.ssh/known_hosts: Permission denied
Host key verification failed.
[exit 255]

==== A5 transient unit WITHOUT ProtectHome: ssh -o BatchMode=yes offsite@nas true
$ dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups ssh -o BatchMode=yes offsite@nas true
[exit 0]

Experiment B: key and known_hosts under /opt/holzcloud/offsite-ssh, named in /etc/ssh/ssh_config.d/holzcloud-backup.conf; the unit keeps its one changed line
==== B1 ssh-keygen -f /opt/holzcloud/offsite-ssh/id_ed25519 as holzcloud
$ dx runuser -u holzcloud -- ssh-keygen -q -t ed25519 -N  -f /opt/holzcloud/offsite-ssh/id_ed25519 -C holzcloud-backup
[exit 0]

==== B1 ssh-keyscan -t ed25519 nas into /opt/holzcloud/offsite-ssh/known_hosts
$ dx runuser -u holzcloud -- bash -c ssh-keyscan -t ed25519 nas > "$1"; test -s "$1" _ /opt/holzcloud/offsite-ssh/known_hosts
# nas:22 SSH-2.0-OpenSSH_9.2p1 Debian-2+deb12u10
[exit 0]

==== B2 /etc/ssh/ssh_config.d/holzcloud-backup.conf and the directory
$ dx bash -c cat /etc/ssh/ssh_config.d/holzcloud-backup.conf; ls -la /opt/holzcloud/offsite-ssh
Host nas
    User offsite
    IdentityFile /opt/holzcloud/offsite-ssh/id_ed25519
    UserKnownHostsFile /opt/holzcloud/offsite-ssh/known_hosts
    IdentitiesOnly yes
    StrictHostKeyChecking yes
    BatchMode yes
total 20
drwx------ 2 holzcloud holzcloud 4096 Oct 26 10:50 .
drwxr-xr-x 7 root      root      4096 Oct 26 10:50 ..
-rw------- 1 holzcloud holzcloud  411 Oct 26 10:50 id_ed25519
-rw-r--r-- 1 holzcloud holzcloud   98 Oct 26 10:50 id_ed25519.pub
-rw-r--r-- 1 holzcloud holzcloud   85 Oct 26 10:50 known_hosts
[exit 0]

```

#### amd64-t24-src/offsite-journal.txt

```text
Oct 26 10:49:57.933039 0c141912becc backup.sh[3487]: === holzcloud-CMS backup: 20261026-104957 ===
Oct 26 10:49:58.015236 0c141912becc backup.sh[3487]: Backing up database...
Oct 26 10:49:58.137938 0c141912becc backup.sh[3495]: 2026/10/26 10:49:58 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 26 10:49:58.178274 0c141912becc backup.sh[3495]: 2026/10/26 10:49:58 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-104957/holzcloud.sqlite
Oct 26 10:49:58.192260 0c141912becc backup.sh[3495]: 2026/10/26 10:49:58 INFO backup verified path=/opt/holzcloud/backups/20261026-104957/holzcloud.sqlite tables=57
Oct 26 10:49:58.193921 0c141912becc backup.sh[3495]: backup written and verified: /opt/holzcloud/backups/20261026-104957/holzcloud.sqlite
Oct 26 10:49:58.195721 0c141912becc backup.sh[3487]: Backing up CSRF key...
Oct 26 10:49:58.213794 0c141912becc backup.sh[3487]: Backing up media...
Oct 26 10:49:58.319095 0c141912becc backup.sh[3487]: Backing up plugins...
Oct 26 10:49:58.444706 0c141912becc backup.sh[3487]: Backing up sprachen...
Oct 26 10:49:58.547411 0c141912becc backup.sh[3487]: Backing up marke...
Oct 26 10:49:58.749541 0c141912becc backup.sh[3487]: Pruning backups older than 30 days...
Oct 26 10:49:58.773613 0c141912becc backup.sh[3487]: Copying to offsite@nas:/backups/holzcloud...
Oct 26 10:49:59.534770 0c141912becc backup.sh[3534]: hostkeys_find_by_key_hostfile: hostkeys_foreach failed for /home/holzcloud/.ssh/known_hosts: Permission denied
Oct 26 10:49:59.535330 0c141912becc backup.sh[3534]: Host key verification failed.
Oct 26 10:49:59.537706 0c141912becc backup.sh[3533]: rsync: connection unexpectedly closed (0 bytes received so far) [sender]
Oct 26 10:49:59.538147 0c141912becc backup.sh[3533]: rsync error: unexplained error (code 255) at io.c(232) [sender=3.2.7]
```

#### amd64-t24-src/offsite-journal-b.txt

```text
Oct 26 10:50:04.561237 0c141912becc backup.sh[3666]: === holzcloud-CMS backup: 20261026-105004 ===
Oct 26 10:50:04.644067 0c141912becc backup.sh[3666]: Backing up database...
Oct 26 10:50:04.768605 0c141912becc backup.sh[3674]: 2026/10/26 10:50:04 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 26 10:50:04.808624 0c141912becc backup.sh[3674]: 2026/10/26 10:50:04 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-105004/holzcloud.sqlite
Oct 26 10:50:04.822063 0c141912becc backup.sh[3674]: 2026/10/26 10:50:04 INFO backup verified path=/opt/holzcloud/backups/20261026-105004/holzcloud.sqlite tables=57
Oct 26 10:50:04.823721 0c141912becc backup.sh[3674]: backup written and verified: /opt/holzcloud/backups/20261026-105004/holzcloud.sqlite
Oct 26 10:50:04.825498 0c141912becc backup.sh[3666]: Backing up CSRF key...
Oct 26 10:50:04.844885 0c141912becc backup.sh[3666]: Backing up media...
Oct 26 10:50:04.949660 0c141912becc backup.sh[3666]: Backing up plugins...
Oct 26 10:50:05.073690 0c141912becc backup.sh[3666]: Backing up sprachen...
Oct 26 10:50:05.176630 0c141912becc backup.sh[3666]: Backing up marke...
Oct 26 10:50:05.367762 0c141912becc backup.sh[3666]: Pruning backups older than 30 days...
Oct 26 10:50:05.391649 0c141912becc backup.sh[3666]: Copying to offsite@nas:/backups/holzcloud...
Oct 26 10:50:06.890951 0c141912becc backup.sh[3666]: === Backup complete: /opt/holzcloud/backups/20261026-105004 ===
Oct 26 10:50:06.908584 0c141912becc backup.sh[3714]: 20M        /opt/holzcloud/backups/20261026-105004
```

#### amd64-t24-src/offsite-restore.txt

```text
RESTORED-FROM: /backups/holzcloud/20261026-105004
BACKUPS-OF-HOST2-BEFORE: empty
PASS /opt/holzcloud/backups of holzcloud-t24-host2 is empty before the restore: the restore can only come from /backups/holzcloud/20261026-105004
PASS the sha256 of every file of /backups/holzcloud/20261026-105004 on holzcloud-t24-host2 (18 files) equals that of /opt/holzcloud/backups/20261026-105004 on the source
RUN-SH-RESTORE-EXIT: 0
PASS run.sh restore ended with status 0
PASS restore.txt: login 303 to /admin/, setup form 200 before and 404 after, media served 200, marker page, check 17 of 17, and no BAD line
PASS restore-run.txt holds '=== Restoring from /backups/holzcloud/20261026-105004 ===': the document's restore line was played against the copy of the unit
PASS the DIR line of restore.txt equals the copy path (/backups/holzcloud/20261026-105004)
PASS /opt/holzcloud/backups of holzcloud-t24-host2 is still empty after the restore
SETUP-BEFORE: HTTP 200 -
LOGIN: HTTP 303 /admin/
SETUP-AFTER: HTTP 404 -
MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
MARKER-PAGE: marker page: GET /after-update answers 200 with its sentence
CHECK: check: 17 of 17 assertions passed (check.txt)
RESTORE-WALL: 10.858
RESTORE-TXT PASS setup form open before the restore (the empty database of the first start)
RESTORE-TXT PASS restore: the document's restore block ran without a failing step
RESTORE-TXT PASS readyz status ok, integrity ok
RESTORE-TXT PASS login with the password the user had before: 303 to /admin/
RESTORE-TXT PASS setup form closed after the restore (the restored database has users)
RESTORE-TXT PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261026-105012
RESTORE-TXT PASS set-aside database is the empty one of the first start (users: 0)
RESTORE-TXT PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700
RESTORE-TXT PASS owner and mode of /opt/holzcloud/data/holzcloud.sqlite: holzcloud:holzcloud 600
RESTORE-TXT PASS owner and mode of /opt/holzcloud/data/csrf.key: holzcloud:holzcloud 600
RESTORE-TXT PASS owner and mode of /opt/holzcloud/data/media/1/74d77651d0137b8514146d836d7e165c.jpg: holzcloud:holzcloud 600
RESTORE-TXT PASS tree src-restored: 16 files equal
RESTORE-TXT PASS tree backup-restored: 16 files equal
RESTORE-TXT PASS media served over HTTP: same status and sha256 as in the source (HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577)
RESTORE-TXT PASS marker page: GET /after-update answers 200 with its sentence
RESTORE-TXT PASS state comparison source against target: 53 pass, 0 bad (compare-src-tgt.txt)
RESTORE-TXT PASS check: 17 of 17 assertions passed (check.txt)
INFO NOT PLAYED: the pull by rsync -a that the document describes for a rebuilt server; the restore ran on the second host itself, from the copy path
OFFSITE-RESTORE OK
```

### R2 second host

#### amd64-t24-host2/offsite-host2.txt

```text
---- apt-get update and install openssh-server in holzcloud-t24-host2
Reading package lists...
Building dependency tree...
Reading state information...
The following additional packages will be installed:
  libbsd0 libcbor0.8 libedit2 libfido2-1 libnsl2 libtirpc-common libtirpc3
  libwrap0 openssh-client openssh-sftp-server runit-helper sensible-utils ucf
Suggested packages:
  keychain libpam-ssh monkeysphere ssh-askpass molly-guard ufw
Recommended packages:
  xauth default-logind | logind | libpam-systemd ncurses-term
The following NEW packages will be installed:
  libbsd0 libcbor0.8 libedit2 libfido2-1 libnsl2 libtirpc-common libtirpc3
  libwrap0 openssh-client openssh-server openssh-sftp-server runit-helper
  sensible-utils ucf
0 upgraded, 14 newly installed, 0 to remove and 1 not upgraded.
Need to get 2106 kB of archives.
After this operation, 9763 kB of additional disk space will be used.
Get:1 http://deb.debian.org/debian bookworm/main amd64 libbsd0 amd64 0.11.7-2 [117 kB]
Get:2 http://deb.debian.org/debian bookworm/main amd64 libedit2 amd64 3.1-20221030-2 [93.0 kB]
Get:3 http://deb.debian.org/debian bookworm/main amd64 libcbor0.8 amd64 0.8.0-2+b1 [27.4 kB]
Get:4 http://deb.debian.org/debian bookworm/main amd64 libfido2-1 amd64 1.12.0-2+b1 [77.2 kB]
Get:5 http://deb.debian.org/debian bookworm/main amd64 openssh-client amd64 1:9.2p1-2+deb12u10 [994 kB]
Get:6 http://deb.debian.org/debian bookworm/main amd64 openssh-sftp-server amd64 1:9.2p1-2+deb12u10 [65.9 kB]
Get:7 http://deb.debian.org/debian bookworm/main amd64 sensible-utils all 0.0.17+nmu1 [19.0 kB]
Get:8 http://deb.debian.org/debian bookworm/main amd64 ucf all 3.0043+nmu1+deb12u1 [55.3 kB]
Get:9 http://deb.debian.org/debian bookworm/main amd64 runit-helper all 2.15.2 [6520 B]
Get:10 http://deb.debian.org/debian bookworm/main amd64 libtirpc-common all 1.3.3+ds-1 [14.0 kB]
Get:11 http://deb.debian.org/debian bookworm/main amd64 libtirpc3 amd64 1.3.3+ds-1 [85.2 kB]
Get:12 http://deb.debian.org/debian bookworm/main amd64 libnsl2 amd64 1.3.0-2 [39.5 kB]
Get:13 http://deb.debian.org/debian bookworm/main amd64 libwrap0 amd64 7.6.q-32 [54.9 kB]
Get:14 http://deb.debian.org/debian bookworm/main amd64 openssh-server amd64 1:9.2p1-2+deb12u10 [457 kB]
debconf: delaying package configuration, since apt-utils is not installed
Fetched 2106 kB in 0s (9457 kB/s)
Selecting previously unselected package libbsd0:amd64.
(Reading database ... (Reading database ... 5%(Reading database ... 10%(Reading database ... 15%(Reading database ... 20%(Reading database ... 25%(Reading database ... 30%(Reading database ... 35%(Reading database ... 40%(Reading database ... 45%(Reading database ... 50%(Reading database ... 55%(Reading database ... 60%(Reading database ... 65%(Reading database ... 70%(Reading database ... 75%(Reading database ... 80%(Reading database ... 85%(Reading database ... 90%(Reading database ... 95%(Reading database ... 100%(Reading database ... 8561 files and directories currently installed.)
Preparing to unpack .../00-libbsd0_0.11.7-2_amd64.deb ...
Unpacking libbsd0:amd64 (0.11.7-2) ...
Selecting previously unselected package libedit2:amd64.
Preparing to unpack .../01-libedit2_3.1-20221030-2_amd64.deb ...
Unpacking libedit2:amd64 (3.1-20221030-2) ...
Selecting previously unselected package libcbor0.8:amd64.
Preparing to unpack .../02-libcbor0.8_0.8.0-2+b1_amd64.deb ...
Unpacking libcbor0.8:amd64 (0.8.0-2+b1) ...
Selecting previously unselected package libfido2-1:amd64.
Preparing to unpack .../03-libfido2-1_1.12.0-2+b1_amd64.deb ...
Unpacking libfido2-1:amd64 (1.12.0-2+b1) ...
Selecting previously unselected package openssh-client.
Preparing to unpack .../04-openssh-client_1%3a9.2p1-2+deb12u10_amd64.deb ...
Unpacking openssh-client (1:9.2p1-2+deb12u10) ...
Selecting previously unselected package openssh-sftp-server.
Preparing to unpack .../05-openssh-sftp-server_1%3a9.2p1-2+deb12u10_amd64.deb ...
Unpacking openssh-sftp-server (1:9.2p1-2+deb12u10) ...
Selecting previously unselected package sensible-utils.
Preparing to unpack .../06-sensible-utils_0.0.17+nmu1_all.deb ...
Unpacking sensible-utils (0.0.17+nmu1) ...
Selecting previously unselected package ucf.
Preparing to unpack .../07-ucf_3.0043+nmu1+deb12u1_all.deb ...
Moving old data out of the way
Unpacking ucf (3.0043+nmu1+deb12u1) ...
Selecting previously unselected package runit-helper.
Preparing to unpack .../08-runit-helper_2.15.2_all.deb ...
Unpacking runit-helper (2.15.2) ...
Selecting previously unselected package libtirpc-common.
Preparing to unpack .../09-libtirpc-common_1.3.3+ds-1_all.deb ...
Unpacking libtirpc-common (1.3.3+ds-1) ...
Selecting previously unselected package libtirpc3:amd64.
Preparing to unpack .../10-libtirpc3_1.3.3+ds-1_amd64.deb ...
Unpacking libtirpc3:amd64 (1.3.3+ds-1) ...
Selecting previously unselected package libnsl2:amd64.
Preparing to unpack .../11-libnsl2_1.3.0-2_amd64.deb ...
Unpacking libnsl2:amd64 (1.3.0-2) ...
Selecting previously unselected package libwrap0:amd64.
Preparing to unpack .../12-libwrap0_7.6.q-32_amd64.deb ...
Unpacking libwrap0:amd64 (7.6.q-32) ...
Selecting previously unselected package openssh-server.
Preparing to unpack .../13-openssh-server_1%3a9.2p1-2+deb12u10_amd64.deb ...
Unpacking openssh-server (1:9.2p1-2+deb12u10) ...
Setting up runit-helper (2.15.2) ...
Setting up libtirpc-common (1.3.3+ds-1) ...
Setting up libcbor0.8:amd64 (0.8.0-2+b1) ...
Setting up sensible-utils (0.0.17+nmu1) ...
Setting up libfido2-1:amd64 (1.12.0-2+b1) ...
Setting up libbsd0:amd64 (0.11.7-2) ...
Setting up libtirpc3:amd64 (1.3.3+ds-1) ...
Setting up libedit2:amd64 (3.1-20221030-2) ...
Setting up ucf (3.0043+nmu1+deb12u1) ...
Setting up libnsl2:amd64 (1.3.0-2) ...
Setting up openssh-client (1:9.2p1-2+deb12u10) ...
Setting up libwrap0:amd64 (7.6.q-32) ...
Setting up openssh-sftp-server (1:9.2p1-2+deb12u10) ...
Setting up openssh-server (1:9.2p1-2+deb12u10) ...

Creating config file /etc/ssh/sshd_config with new version
Creating SSH2 RSA key; this may take some time ...
3072 SHA256:FyjPNdg5AOd2+442bLJKBl/dzBhvF9N6Om05pIuva78 root@802c943533be (RSA)
Creating SSH2 ECDSA key; this may take some time ...
256 SHA256:6zBgYNEt72y2gwMq0WsGbLnuipkPO7rhxpy6jX9bU9A root@802c943533be (ECDSA)
Creating SSH2 ED25519 key; this may take some time ...
256 SHA256:Nfkl8/3eiEh1hjOrADvDbcZY/EGOcqOOp8VsN+VsvJo root@802c943533be (ED25519)
invoke-rc.d: policy-rc.d denied execution of start.
Created symlink /etc/systemd/system/sshd.service → /lib/systemd/system/ssh.service.
Created symlink /etc/systemd/system/multi-user.target.wants/ssh.service → /lib/systemd/system/ssh.service.
/usr/sbin/policy-rc.d returned 101, not running 'start rescue-ssh.target ssh.service'
/usr/sbin/policy-rc.d returned 101, not running 'start ssh.socket'
Processing triggers for libc-bin (2.36-9+deb12u14) ...
Synchronizing state of ssh.service with SysV service script with /lib/systemd/systemd-sysv-install.
Executing: /lib/systemd/systemd-sysv-install enable ssh
---- the second host after the preparation
drwxr-xr-x 3 root    root    4096 Oct 26 10:49 /backups
drwx------ 2 offsite offsite 4096 Oct 26 10:49 /backups/holzcloud
offsite:x:1000:1000::/home/offsite:/bin/bash
rsync  version 3.2.7  protocol version 32
openssh-server	1:9.2p1-2+deb12u10
```

#### amd64-t24-host2/transfer.txt

```text
DIR: /backups/holzcloud/20261026-105004
SOURCE: copy made by holzcloud-backup.service through REMOTE_TARGET
FILES: 18
VIA: rsync by the shipped unit, no docker cp of the backup
TRANSFER: equal
```

#### amd64-t24-host2/restore.txt

```text
DIR: /backups/holzcloud/20261026-105004
SETUP-BEFORE: HTTP 200 -
PASS setup form open before the restore (the empty database of the first start)
PASS restore: the document's restore block ran without a failing step
READYZ: {"status":"ok","version":"v0.1.0-31-gb501b1b","commit":"b501b1b","integrity":"ok"}
PASS readyz status ok, integrity ok
LOGIN: HTTP 303 /admin/
PASS login with the password the user had before: 303 to /admin/
SETUP-AFTER: HTTP 404 -
PASS setup form closed after the restore (the restored database has users)
VERIFY-S: 0.134
STOP-MOVE-COPY-S: 0.682
START-READY-S: 9.716
WALL-S: 10.858
INFO the readiness loop of restore.sh polls once a second, so START-READY-S has a resolution of about one second; the times come from the amd64 emulation named in HOST-ARCH and CONTAINER-ARCH, not from a real server
DATA-BYTES: 20618714
DB-BYTES: 663552
MEDIA-BYTES: 39983
FILES: 19
MEDIA-FILES: 3
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
BACKUP-BYTES: 20548954
PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261026-105012
PASS set-aside database is the empty one of the first start (users: 0)
INFO restore.sh moves the whole data directory aside, so the -wal and -shm files of the replaced database went with it; none can reach the restored file, and the proof is that the restored database passes the row comparison and the integrity verdict
PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700
PASS owner and mode of /opt/holzcloud/data/holzcloud.sqlite: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/csrf.key: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/media/1/74d77651d0137b8514146d836d7e165c.jpg: holzcloud:holzcloud 600
INFO EXCLUDED holzcloud.sqlite*: the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
INFO EXCLUDED pre-upgrade-*.sqlite*: old-schema copies the service writes before a migration, deliberately not part of a backup
INFO EXCLUDED MANIFEST: only a backup has one; it names the version and the host that made it
PASS tree src-restored: 16 files equal
PASS tree backup-restored: 16 files equal
MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
PASS media served over HTTP: same status and sha256 as in the source (HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577)
PASS marker page: GET /after-update answers 200 with its sentence
PASS state comparison source against target: 53 pass, 0 bad (compare-src-tgt.txt)
INFO owner and mode of holzcloud.sqlite: source 'holzcloud 600', restored 'holzcloud 600' (the data directory is 700)
COMES-BACK: csrf.key holzcloud.sqlite marke media plugins sprachen (the top-level entries of the backup directory that are in the restored data directory)
INFO in the restored data directory and not from the backup, made by the service's own start: holzcloud.sqlite-shm holzcloud.sqlite-wal
HAND-REBUILT: the unit file /etc/systemd/system/holzcloud.service (written by the install, no backup holds it)
HAND-REBUILT: the drop-ins /etc/systemd/system/holzcloud.service.d/container.conf (written by the install or the harness, no backup holds them)
HAND-REBUILT: no environment file is configured on this unit; on a real server HOLZCLOUD_SECRET_KEY and payment or mail credentials live in one, never in the data directory, so no backup holds them
HAND-REBUILT: /root/.hc-login-pass (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
HAND-REBUILT: /root/.hc-key (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
HAND-REBUILT: /tmp/hc-upload.jpg (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
PASS check: 17 of 17 assertions passed (check.txt)
```

#### amd64-t24-host2/restore-run.txt

```text
1793011812.648777667 === Restoring from /backups/holzcloud/20261026-105004 ===
1793011812.669737417 timestamp=20261026-105004
1793011812.688875833 version=holzcloud v0.1.0-31-gb501b1b (b501b1b)
1793011812.707959458 data_dir=/opt/holzcloud/data
1793011812.731960833 host=0c141912becc
1793011812.751138042 Verifying snapshot...
1793011812.836480500 2026/10/26 10:50:12 INFO sqlite opened journal_mode=wal path=/tmp/tmp.IXVLwzhcys/holzcloud.sqlite
1793011812.867340625 integrity: ok
1793011812.885363875 Stopping holzcloud...
1793011812.992779375 Moving current data to /opt/holzcloud/data.before-restore-20261026-105012
1793011813.025829042 Restoring database...
1793011813.057806667 Restoring CSRF key...
1793011813.077887667 Restoring media...
1793011813.185670750 Restoring plugins...
1793011813.312981375 Restoring sprachen...
1793011813.429562167 Restoring marke...
1793011813.566893834 Starting holzcloud...
1793011813.622800167 Waiting for readiness...
1793011823.282447463 === Restore complete ===
1793011823.327785463 {"status":"ok","version":"v0.1.0-31-gb501b1b","commit":"b501b1b","uptime_s":0,"integrity":"ok","disk_free_bytes":1821895528448,"database":{"freelist_count":0,"page_count":162,"size_bytes":663552,"wal_bytes":0}}
1793011823.345802297 
WALL 1793011812.508839208 1793011823.367203713
```

#### amd64-t24-host2/check.txt

```text
PASS loaded bestellung
PASS loaded jahreszahl
PASS loaded kontaktformular
PASS loaded nicht-gefunden
PASS loaded suche
PASS no-load-failures
PASS running bestellung
PASS running jahreszahl
PASS running kontaktformular
PASS running nicht-gefunden
PASS running suche
PASS invoke jahreszahl
PASS invoke kontaktformular
PASS invoke bestellung
PASS invoke suche
PASS invoke nicht-gefunden
PASS stable
engine: compiler (9 anonymous executable mappings)
# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests
LATENCY /plugintest median_s=0.014939 n=10
LATENCY /suche?q=Plugintest median_s=0.012987 n=10
LATENCY /formular median_s=0.009998 n=10
LATENCY /bestellung median_s=0.009693 n=10
Seccomp:	0
Seccomp_filters:	0
NRestarts=0
Result=success
{"time":"2026-10-26T10:50:36.505967928Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-26T10:50:38.384103387Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-26T10:50:39.874884055Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-26T10:50:41.218084305Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-26T10:50:42.519600083Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```
