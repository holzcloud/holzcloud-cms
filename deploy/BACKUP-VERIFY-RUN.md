# A backup that carries its checksums, a restore that names what is missing, a retention step that speaks up, measured: results

This document is the permanent record of phase 32. It records what the changed `deploy/backup.sh` and `deploy/restore.sh` do, played in a container with both scripts exactly as committed: that a backup now carries a `SHA256SUMS` file and that `restore.sh` refuses a damaged backup before it touches anything, naming the first file that differs from its checksum or is missing, while a backup made before the checksum file existed still restores, with a notice (BAK-01); that a restore reports every file a database row names and the backup lacks, before its question and again at the end, and ends with status 3 (BAK-02); and that a retention step which cannot remove an old directory writes the directory into the journal and ends the night with status 40 under the shipped unit (BAK-03).

The record rests on one evidence set: a single `backup-verify.sh run` from one fresh container, made once against the committed text of every bound file (see *How this was run*). Its raw output is the Appendix, pasted unchanged, and `bash deploy/test/backup-verify.sh record-check` proves every block against its scratch file byte for byte. Every number in this document is a line of that set and cites its file in backticks as `DIRECTORY/FILE`, with `DIRECTORY` the one scratch directory `amd64-t32-src`; the run files beside it are named by their path below `deploy/test/out`. A file that is not pasted is named as `FILE in DIRECTORY`.

The record says what was done, how many times, and what was seen. It does not say that a backup is safe from tampering, because a checksum is no signature, and it does not say what a real disk does, because a container cannot. What the container cannot show is a named deviation or a line of *Not tested*, never a gap in the table.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| BAK-01 | PASS: `BAK-01 PASS` stands in `amd64-t32-src/summary.txt`. The document's manual backup line ended 0 and wrote a `SHA256SUMS` of 37 lines that names exactly the files of the directory, holds the database snapshot and 23 media lines, passed `sha256sum -c` and is named by the `MANIFEST` (`MANUAL-RC: 0`, `SUMS-LINES: 37`, `SUMS-EQUALS-FIND: yes`, `SUMS-CHECK-RC: 0`, `SUMS-HAS-SQLITE: yes`, `SUMS-MEDIA-LINES: 23`, `MANIFEST-CHECKSUMS-LINE: yes`). Four damaged copies were each refused with status 1 and the live service unchanged, naming the file: a flipped media byte, a flipped byte inside a database page that still read `BITROT-CHECK: integrity: ok`, a deleted listed file, and no `SHA256SUMS` (`FLIP-MEDIA-RC`, `BITROT-RESTORE-RC`, `DELETED-RC`, `NOSUMS-RC`, the four `INVARIANTS-AFTER-*: same`). A backup made by the `backup.sh` of v0.1.4 restored with `OLD-RESTORE-RC: 0` and `OLD-NOTICE: yes`, and the untouched new backup restored with `NEW-RESTORE-RC: 0` and `NEW-VERIFIED-LINE: Checksums: 37 files verified`, all in `amd64-t32-src/bak01.txt`. | Debian 12 container, systemd PID 1, linux/amd64 under emulation on an arm64 host (`amd64-t32-src/versions.txt`) | 2026-11-13, 4563954 |
| BAK-02 | PASS: `BAK-02 PASS` stands in `amd64-t32-src/summary.txt`. An original and its `thumb` variant that rows name were removed before a backup (`LOST:` lines); the backup ended `BACKUP-RC: 0` and its checksum file does not list them (`SUMS-LISTS-LOST: no`); `restore.sh` named both before its question and again at the end (`MISSING-BEFORE-QUESTION: 2`, `MISSING-AT-END: 2`), restored the site (`READYZ: ok`) and ended `RESTORE-RC: 3`, all in `amd64-t32-src/bak02.txt`. | same | 2026-11-13, 4563954 |
| BAK-03 | PASS: `BAK-03 PASS` stands in `amd64-t32-src/summary.txt`. The shipped `holzcloud-backup.service`, with no drop-in (`UNIT-DROPINS: ''`), met an old backup directory it could not remove; the journal names the directory (`JOURNAL-NAMES-DIR: yes`), the unit ended `RESULT: exit-code` and `EXEC-MAIN-STATUS: 40`, the locked directory is still there, the removable one is gone and the night's backup has its `MANIFEST` (`LOCKED-STILL-THERE: yes`, `REMOVABLE-GONE: yes`, `NIGHT-HAS-MANIFEST: yes`), all in `amd64-t32-src/bak03.txt`. | same | 2026-11-13, 4563954 |

Evidence commit: 4563954

The binary under test is the one built from `NEW_REF` (pinned by `run` to the short sha of HEAD, `4563954`, read from `deploy/test/out/amd64-t32-evidence-commit.txt`) with the Build line of `deploy/DEPLOY.md`: `BINARY: holzcloud v0.1.4-65-g4563954 (4563954)` in `amd64-t32-src/summary.txt`, and the same in the lines `binary:`, `NEW_REF:` and `evidence commit:` of `amd64-t32-src/versions.txt`. The summary carries `STAGES: bak01 bak02 bak03` and no `REHEARSAL` line; every restore of the set prints `version=holzcloud v0.1.4-65-g4563954 (4563954)` from the binary it ran with.

## How this was run

`deploy/test/backup-verify.sh` is a sibling of `deploy/test/run.sh` and `deploy/test/nightly.sh` (it edits neither) and reaches the harness through `run.sh`'s public subcommands only (`build`, `up`, `install`, `postinstall`, `seed`, `backup-fixture`). `run` refuses to start unless `PLATFORM=linux/amd64` and `TARGET=systemd-base`, refuses an existing container `holzcloud-t32`, refuses while one of the bound files differs from HEAD or is untracked, pins `NEW_REF` to the short sha of HEAD so that the binary and the scripts are one commit, and plays the stages `bak01`, `bak02` and `bak03` in one container. The verdict of each requirement is computed from the `KEY: value` files the stages wrote, each key exactly once, and a stage that did not run reads NOT-RUN, so a trace can never pass for the set.

The set was made once, from a fresh container (`holzcloud-t32`), on the clean committed tree of `4563954` without `REHEARSAL`, started 2026-11-13T14:04:55Z and ended 14:06:43Z on the host clock (1 minute 48 seconds), exit status 0 (`deploy/test/out/amd64-t32-pass1.started`, `deploy/test/out/amd64-t32-pass1.finished` and `deploy/test/out/amd64-t32-pass1.status`). Before it, the worktree held no earlier output of the driver, and after it `bash deploy/test/backup-verify.sh down` removed the container. No second set was made against `4563954`, and no other set is cited. An earlier set, made against `0ce6d68` before the review of phase 32, is superseded: the fixes of that review changed `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/test/run.sh` and the source of the binary after it, so this record was written again from this set and nothing of the earlier one is quoted. The command was:

```text
PLATFORM=linux/amd64 TARGET=systemd-base bash deploy/test/backup-verify.sh run
```

The knobs not set on that line had their defaults: `STAGES=bak01 bak02 bak03` and `BULK_MB=100` (100 MB of synthetic media in the data directory, `BULK-MB: 100` in `amd64-t32-src/measurements.txt`). The log of the run is `deploy/test/out/amd64-t32-pass1.log`, which is not pasted; its first line reads `backup-verify: platform linux/amd64, NEW_REF pinned to 4563954, stages: bak01 bak02 bak03, BULK_MB=100`.

No bound file changed after the evidence commit. The bound files for this set are `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh`, `deploy/test/Dockerfile` and `deploy/test/backup-verify.sh`, together with the binary built from `cmd`, `internal`, `go.mod` and `go.sum`; this command exited 0 when the record was written:

```text
git diff --quiet 4563954 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/Dockerfile deploy/test/backup-verify.sh cmd internal go.mod go.sum
```

`deploy/test/backup-robust.sh` and `deploy/test/restore-robust.sh`, the hermetic tests of the two changed scripts (they need no Docker and build a fake binary, a fake `rsync` and a fake `date`), are not part of the set, are not bound files and their output is not pasted. When this record was written they ended `BACKUP-ROBUST OK (178 cases)` and `RESTORE-ROBUST OK (155 cases)`, and `bash deploy/test/backup-verify.sh selftest` ended `SELFTEST OK (117 cases)`.

## Deviations named

Everything here precedes every result: it is what the container does differently from a real server and what the harness did that the document does not contain. The first column is the name written to `amd64-t32-src/deviations.txt`; the kinds in the summary are `dropin emulation fixture lost-file mount old-script scp start-limit` (the `DEVIATIONS:` line of `amd64-t32-src/summary.txt`).

| Name | Why | What a real server does instead |
|------|-----|---------------------------------|
| DEVIATION[emulation] | The Docker host is aarch64 and the linux/amd64 container runs under emulation (`host architecture: aarch64`, `container uname -m: x86_64` in `amd64-t32-src/versions.txt`), so every wall time of this set is an emulation figure | A native amd64 CPU and disk |
| DEVIATION[start-limit] | `systemctl reset-failed holzcloud` runs before each restore, because several restores in a row restart the service more often than its start limit allows. The unit is not changed | One restore, by hand, which never meets the limit |
| DEVIATION[dropin] | `/etc/systemd/system/holzcloud.service.d/container.conf` sets `ProcSubset=all` on `holzcloud.service`, because the document's unchanged start crashed in the container (`unit-start-unchanged.txt` in `amd64-t32-src`). The backup units have no drop-in (`UNIT-DROPINS: ''` in `amd64-t32-src/bak03.txt`) | The unit as shipped |
| DEVIATION[scp] | The two `scp` lines of the document, which copy into the operator's home directory (`scp holzcloud user@your-server:` and `scp -r deploy/ user@your-server:`), were played as `docker cp` of the binary to `/root/holzcloud` and as a tar stream of the committed `deploy` directory into `/root` of the container | `scp` from the build machine into the operator's home directory |
| DEVIATION[fixture] | Two lines: run.sh's `backup-fixture` wrote a language file, a logo and 100 MB of synthetic media under `data/media/WEBSITE/bulk`; the driver copied the five plugin archives into `/opt/holzcloud/plugin-zips` before `run.sh seed`. They stand in for operator work and are no step of the document | The operator's own content and plugins |
| DEVIATION[old-script] | The `backup.sh` of v0.1.4 ran with this release's binary, as `/opt/holzcloud/deploy-v014/backup.sh` with `DATA_DIR` and `HOLZCLOUD_BIN` as the unit sets them, into `/opt/holzcloud/backups-v014`; its sha256 is `OLD-BACKUP-SCRIPT` in `amd64-t32-src/bak01.txt` | A server that kept the old script runs it with the binary of its own release |
| DEVIATION[lost-file] | The two files of BAK-02 were removed from the live data directory before the backup, standing in for a file deleted between the snapshot and the copy. Phase 29 measured that window as ROW-WITHOUT-FILE (`deploy/BACKUP-ROBUST-RUN.md`); it is not provoked again here | A picture deleted in the admin while the media copy runs |
| DEVIATION[mount] | Inherited from phases 21 and 22: the unit's `RequiresMountsFor` names `/opt/holzcloud/data`, a plain directory on the container's root filesystem, so that line was not exercised. For the same reason every restore of the set renamed the data directory (`Moving current data to /opt/holzcloud/data.before-restore-20261113-140610` in `amd64-t32-src/bak01-new-restore.out`), and the branch of `restore.sh` that moves the contents of a mount point aside inside it was not taken | A data directory on its own mount, whose contents a restore moves into `.before-restore-` and a timestamp inside it |

The obstacle of BAK-03 is not a deviation kind of the file and is named here all the same: the old directory that retention cannot remove is made so by a subdirectory owned by root (`PLANTED: /opt/holzcloud/backups/20200101-031500 (locked: root-owned)` in `amd64-t32-src/bak03.txt`). On a real server the cause would be a disk error, a read-only filesystem or a permission an operator changed (*Container vs. server*).

## Findings

Each finding has the class of the earlier records' rule: (1) a bug of the harness, fixed in the driver; (2) a limit of the container, a named deviation or a *Not tested* line; (3) a measured property of a shipped file that matches what it and its document say, recorded and not edited; (4) a defect, a shipped file doing less than it or its document says, with a fix in a commit of its own. No finding of this set is class 1 or class 4, and no shipped file was edited after the evidence commit. The defects the review of phase 32 found in `deploy/backup.sh`, `deploy/restore.sh` and `deploy/DEPLOY.md` were fixed in commits of their own before it, and this set was made on the fixed text.

The test for class 4 was whether a result contradicts something `deploy/backup.sh`, `deploy/restore.sh` or `deploy/DEPLOY.md` says as committed at `4563954`, or leaves the operator worse off than they say. The document says that `SHA256SUMS` holds the checksum of every other regular file of the backup, that `restore.sh` checks it both ways before it touches anything and refuses the backup naming the first file that differs, is missing or is not listed, that a backup made before 0.1.5 is restored with a notice, that files a row names and the backup lacks are listed before the question and again after the restore and the script ends with status 3, and that a retention failure shows `ERROR: retention could not remove` in the journal and ends the night with status 40. F-1 to F-5 are those sentences observed; F-6 is what the text says and this set did not play, and F-7 is what the checksum does not answer.

**F-1 (class 3): A flipped byte inside a database page that still passed the integrity check was refused by the checksum, naming `./holzcloud.sqlite`, before anything was stopped.** The byte at offset 667395 of a copy of the snapshot was flipped, and a separate copy with the same flip read `integrity: ok` with status 0 (`amd64-t32-src/bak01-bitrot-check.out`, `BITROT-OFFSET: 667395` and `BITROT-CHECK: integrity: ok` in `amd64-t32-src/bak01.txt`). `restore.sh` refused it with status 1 (`BITROT-RESTORE-RC: 1`, `BITROT-NAMED: ./holzcloud.sqlite`, `INVARIANTS-AFTER-BITROT: same`). The line (`amd64-t32-src/bak01-bitrot.out`):

```text
1794578750.378664926 ERROR: ./holzcloud.sqlite differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
```

**F-2 (class 3): A flipped media byte, a deleted listed file and a missing SHA256SUMS were each refused with status 1, naming the file, with the inode of the live database, the invocation of the service, its state and the hash of the other files of the data directory unchanged.** The media byte was flipped in `./media/1/65a035c0e3dbaa4364137d4c9572f204-medium.jpg` and the restore named that file; the deleted file was `./media/1/bulk/bulk-9.bin` and the restore named it as listed and missing; the copy without `SHA256SUMS` was refused because its `MANIFEST` names the file (`FLIP-MEDIA-FILE`, `FLIP-MEDIA-NAMED`, `DELETED-FILE`, `DELETED-NAMED`, `NOSUMS-NAMED: yes` and the `INVARIANTS-AFTER-*` lines of `amd64-t32-src/bak01.txt`; the invariants before and after are `inv-*-before.lst` and `inv-*-after.lst` in `amd64-t32-src`). The lines (`amd64-t32-src/bak01-flip-media.out`, `amd64-t32-src/bak01-deleted.out`, `amd64-t32-src/bak01-nosums.out`):

```text
1794578747.103546800 ERROR: ./media/1/65a035c0e3dbaa4364137d4c9572f204-medium.jpg differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
1794578753.000229844 ERROR: ./media/1/bulk/bulk-9.bin is listed in SHA256SUMS and missing — the backup is damaged, do not restore it
1794578755.074116303 ERROR: /root/t32/nosums/MANIFEST names SHA256SUMS but the file is missing — do not restore it
```

**F-3 (class 3): A backup made by the backup.sh of v0.1.4 restored with status 0 and the notice that its files were not verified.** The old script wrote no `Writing checksums...` line (`amd64-t32-src/bak01-old-backup.out`), and the restore printed the notice in place of the verification and went on to the snapshot check (`OLD-RESTORE-RC: 0`, `OLD-NOTICE: yes` in `amd64-t32-src/bak01.txt`). The line (`amd64-t32-src/bak01-old-restore.out`):

```text
1794578757.800114680 NOTICE: /opt/holzcloud/backups-v014/20261113-140556 has no checksum file (made before 0.1.5); its files were not verified
```

**F-4 (class 3): Two files that rows name and the backup lacked were listed before the question and again at the end, the site was restored, and restore.sh ended with status 3.** The lines (`amd64-t32-src/bak02-restore.out`):

```text
1794578786.368176304 files: 3 rows checked, 2 missing
1794578786.386418221 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204.jpg (media row 1, website 1, original name "velo-01.jpg")
1794578786.404203304 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
1794578798.068427629 WARNING: restored, but 2 files that the database names are missing from the backup:
```

**F-5 (class 3): Under the shipped unit with no drop-in, a retention step that could not remove an old directory wrote an ERROR line naming it into the journal, the night's backup was complete, and the unit ended with Result exit-code and ExecMainStatus 40.** The removal error of `rm` is no longer discarded, the script names the directory and says what the status means (`amd64-t32-src/bak03-journal.txt`; `RESULT: exit-code`, `EXEC-MAIN-STATUS: 40`, `NIGHT-HAS-MANIFEST: yes` in `amd64-t32-src/bak03.txt`):

```text
rm: cannot remove '/opt/holzcloud/backups/20200101-031500/locked/keep': Permission denied
ERROR: retention could not remove /opt/holzcloud/backups/20200101-031500; it may be partly removed, remove it by hand
=== Backup complete: /opt/holzcloud/backups/20261113-140638 ===
ERROR: the backup is complete, but retention failed (see above); ending with status 40
holzcloud-backup.service: Main process exited, code=exited, status=40/n/a
```

**F-6 (class 3, read from the text and not played): The checksum is checked both ways, so a file added to the backup directory after the night is refused by name as not listed, and a symbolic link or a special file in the backup is refused before any file is read; it detects rot and accidental damage, not tampering.** `backup.sh` writes one line per regular file of the night except `SHA256SUMS` and the `MANIFEST`, and fails a night that copied a symbolic link or a special file. `restore.sh` finds the files of the backup itself, never opening a path the list names, and refuses the backup when a listed file differs or is missing, when a file of the backup is not listed (`is in the backup but not listed in SHA256SUMS`), when a line is listed twice, when `SHA256SUMS` is empty or does not list `./holzcloud.sqlite`, and when the backup holds a link or a special file (`is a symbolic link or not a regular file`). A name with a backslash or a newline, which GNU `sha256sum` writes as an escaped line, is checked and verified like any other line. A checksum detects rot and accidental damage, not tampering: whoever can write the backup can rewrite a file and its line alike, as `deploy/DEPLOY.md` says. None of these cases was played in this set; the hermetic test `deploy/test/restore-robust.sh` plays the refusals and the escaped line (*Not tested*).

**F-7 (class 3): A file that was never in the backup is not a checksum failure; the checksum passed and the gap was found by the row check.** The backup of BAK-02 ended 0, its `SHA256SUMS` does not list the two lost files (`BACKUP-RC: 0`, `SUMS-LISTS-LOST: no` in `amd64-t32-src/bak02.txt`; its output is `bak02-backup.out` in `amd64-t32-src`), and the restore verified the 35 files it listed before `holzcloud check -files` named the two (`1794578786.125273721 Checksums: 35 files verified` in `amd64-t32-src/bak02-restore.out`). The two checks answer different questions, and BAK-01 and BAK-02 together are the answer to both.

## BAK-01: a backup carries its checksums and a damaged one is refused

**What was done.** The document's manual backup line (`sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`) was run by its exact text; its output ends with `Writing checksums...`, `Pruning backups older than 30 days...` and `=== Backup complete: /opt/holzcloud/backups/20261113-140542 ===` (`amd64-t32-src/bak01-manual.out`). The path column of the new `SHA256SUMS` was compared with a `find` of the directory and `sha256sum -c` was run on it in the container (`SUMS-LINES: 37`, `SUMS-EQUALS-FIND: yes`, `SUMS-CHECK-RC: 0`, `SUMS-HAS-SQLITE: yes`, `SUMS-MEDIA-LINES: 23`, `MANIFEST-CHECKSUMS-LINE: yes` in `amd64-t32-src/bak01.txt`; the lists are `bak01-sums.lst`, `bak01-find.lst` and `bak01-dir.lst` in `amd64-t32-src`). Four copies of that backup below `/root/t32` were damaged, one way each, and handed to the shipped `restore.sh` with the invariants of the live site taken before and after: a flipped media byte (F-2), a flipped byte inside a database page that still passed `holzcloud check` (F-1), a deleted listed file (F-2) and a removed `SHA256SUMS` (F-2). Then the `backup.sh` of v0.1.4 made a backup with this release's binary (`OLD-BACKUP-RC: 0`, `DEVIATION[old-script]`), which was restored (F-3), and the untouched new backup was restored last:

```text
1794578770.656405088 Checksums: 37 files verified
```

(`amd64-t32-src/bak01-new-restore.out`; `NEW-RESTORE-RC: 0` and `NEW-VERIFIED-LINE: Checksums: 37 files verified` in `amd64-t32-src/bak01.txt`).

*What it shows.* A backup made by the document's manual line carried a `SHA256SUMS` that listed each of its 37 files and passed `sha256sum -c`, and `restore.sh` refused each of four damaged copies of it (a flipped media byte, a flipped database byte that still passed the integrity check, a deleted file and a missing checksum file) with status 1, naming the first file at fault and leaving the live site unchanged, while a backup made by the `backup.sh` of v0.1.4 restored with status 0 and the notice that its files were not verified.

**What it does not show.** Real bit rot on a disk, which flips what it likes and not a byte chosen to keep the integrity check green. A tampered backup, which a checksum cannot catch (F-6). A file added to the backup after the night, a symbolic link or a special file in it, and a file with a backslash or a newline in its name, which the hermetic test plays and this set does not (F-6).

## BAK-02: a restore names the files a row expects and the backup lacks

**What was done.** An original picture and its `thumb` variant, both named by rows, were removed from the live data directory (`DEVIATION[lost-file]`), the document's backup line was run, and the backup was restored with `restore.sh`, answering `yes` at its question, with every output line stamped by the container clock. The two files (`amd64-t32-src/bak02.txt`):

```text
LOST: media row 1 website 1 file 65a035c0e3dbaa4364137d4c9572f204.jpg
LOST: media_variants row 1 label thumb file 65a035c0e3dbaa4364137d4c9572f204-thumb.jpg
```

The restore verified 35 checksums, read `integrity: ok`, printed `files: 3 rows checked, 2 missing` and the two `MISSING` lines with the row, the website and the original name, stopped the service, moved the data aside, restored, waited for readiness, printed the `/readyz` answer with `"status":"ok"`, and then repeated both `MISSING` lines under the `WARNING:` line (F-4, `amd64-t32-src/bak02-restore.out`). The verdict lines are `MISSING-BEFORE-QUESTION: 2`, `MISSING-AT-END: 2`, `READYZ: ok` and `RESTORE-RC: 3` (`amd64-t32-src/bak02.txt`).

*What it shows.* When a media row and a variant row named files that the backup lacked, the backup still ended with status 0, and `restore.sh` restored the site, listed both files before its question and again at the end, and ended with status 3.

**What it does not show.** The window of a deletion between the snapshot and the copy, which phase 29 measured and which is stood in for here (`DEVIATION[lost-file]`). A restore with an installed `holzcloud` older than 0.1.5, which cannot check rows and says so in a `NOTICE` line; the hermetic test plays it and this set does not. Rebuilding the variant with `holzcloud thumbnails`, which the restore names and this set did not run.

## BAK-03: a retention step that cannot remove a directory says so

**What was done.** Two old backup directories dated 2020 were planted below `/opt/holzcloud/backups`, one of them holding a subdirectory owned by root that the user `holzcloud` cannot empty (`PLANTED:` lines of `amd64-t32-src/bak03.txt`). The shipped `holzcloud-backup.service`, with no drop-in (`UNIT-DROPINS: ''`), was started with `systemctl start`, which returned 1 (`START-RC: 1`); then `systemctl show -p Result -p ExecMainStatus` was read and the journal of that invocation kept (`amd64-t32-src/bak03-journal.txt`). The journal holds the whole night: the database, `csrf.key`, media, plugins, language files and logo, `Writing checksums...`, `Pruning backups older than 30 days...`, the `rm` error, the `ERROR: retention could not remove` line with the directory, `=== Backup complete: /opt/holzcloud/backups/20261113-140638 ===`, `120M` and the closing `ERROR:` line with status 40 (F-5).

*What it shows.* Under the shipped `holzcloud-backup.service` with no drop-in, an old backup directory that could not be removed put the line `ERROR: retention could not remove` with that directory into the journal, while the night's backup was complete with its `MANIFEST` and the removable old directory was gone, and the unit ended with `Result=exit-code` and `ExecMainStatus=40`.

**What it does not show.** The timer firing at 03:15 with the changed script, the catch-up and the random delay. A removal that fails on a real disk or a read-only filesystem. `ERROR: retention could not list`, the other line the document names, which was not provoked. The off-site copy, which now runs before retention, and a failed copy, which does not skip retention; this set does not configure `REMOTE_TARGET`, and the hermetic test `deploy/test/backup-robust.sh` plays both. A `lost+found` or another name not shaped like a backup in the backup root, which retention now leaves alone; the hermetic test plays it.

## Measurements

The figures the harness wrote. Every one is an emulation figure on an aarch64 host (`amd64-t32-src/versions.txt`); none is a promise of speed.

| Figure | Value | Where |
|--------|-------|-------|
| Synthetic media in the data directory | 100 MB | `BULK-MB` in `amd64-t32-src/measurements.txt` |
| Size of a backup | 120 MB, 39 files (37 listed in `SHA256SUMS`, plus `SHA256SUMS` and the `MANIFEST`) | `BACKUP-MB`, `BACKUP-FILES` in `amd64-t32-src/measurements.txt`; `SUMS-LINES` in `amd64-t32-src/bak01.txt` |
| The manual backup, checksums included | 1.9 s wall | `MANUAL-BACKUP-WALL-S` in `amd64-t32-src/measurements.txt` |
| `sha256sum -c` over the backup | 0.6 s wall | `SHA256SUM-C-WALL-S` in `amd64-t32-src/measurements.txt` |
| The checksum step of `restore.sh` | 0.9 s | `RESTORE-CHECKSUM-STEP-S` in `amd64-t32-src/measurements.txt` |
| A refused restore, from its first line to the `ERROR:` line | 1794578746.428014674 to 1794578747.103546800 (the media flip) | `amd64-t32-src/bak01-flip-media.out` |
| The whole run, from its started file to its finished file | 14:04:55Z to 14:06:43Z | `deploy/test/out/amd64-t32-pass1.started`, `deploy/test/out/amd64-t32-pass1.finished` |

## Container vs. server

| Aspect | In the container | On a real server | Measured or assumed |
|--------|------------------|------------------|---------------------|
| Speed and timing | amd64 emulated on aarch64 | A native server | Not measured natively |
| The damage of BAK-01 | One byte flipped or one file deleted on purpose, in a copy below `/root/t32` | Bit rot, a failing disk, a truncated transfer, anywhere in the tree | Measured for the refusal; the kind of damage is assumed |
| The cause of a failed removal in BAK-03 | A subdirectory owned by root inside an old backup | A disk error, a read-only filesystem, a permission an operator changed | Measured for the message, the journal and the status; the cause is assumed |
| The journal | The container's own `journald` under systemd PID 1, read right after the invocation | The server's journal, read the morning after, perhaps rotated | Measured here |
| The lost file of BAK-02 | Removed from the live data directory before the backup | Deleted in the admin while the media copy runs | Assumed equivalent for the restore; the window was measured in phase 29 |
| The old backup of BAK-01 | The `backup.sh` of v0.1.4 run with this release's binary | The old script with the old binary | Measured for the restore; the old binary is assumed to write the same tree |
| The restores | Several in a row on one container, the start limit reset before each | One restore, by hand | Measured here |
| `holzcloud.service` | With the container-only drop-in `ProcSubset=all` | As shipped | Named deviation; the backup units ran as shipped |

## Not tested

- Real bit rot on a disk was not played: the damage of BAK-01 was a chosen byte in a chosen copy.
- Tampering was not played, and a checksum is no signature: whoever can write the backup can rewrite a file and its line alike (F-6).
- A file added to a backup directory after the night, which the restore refuses by name as not listed, and a symbolic link or a special file in a backup, which the restore refuses before it reads a file, were played by the hermetic test `deploy/test/restore-robust.sh` only (F-6).
- The timer firing at 03:15 with the changed script was not played; the night of BAK-03 was started with `systemctl start`.
- The window of a deletion between the snapshot and the copy was not provoked; the two files of BAK-02 were removed before the backup (`DEVIATION[lost-file]`).
- A file name with a backslash or a newline, whose escaped line is verified like any other, was played by the hermetic test `deploy/test/restore-robust.sh` only.
- An older binary with the new `restore.sh`, which cannot check rows and says so, was played by the hermetic test only.
- A restore that fails after the service was stopped, and the way back it then prints as its last line, was played by the hermetic test `deploy/test/restore-robust.sh` only, which runs the printed commands as printed; every restore of this set went through.
- The refusal of a database whose schema is newer than the binary (STB-03) is proven by the tests of this phase and is recorded by phase 34; it was not played here.
- `ERROR: retention could not list`, the off-site copy, which now runs before retention, and a retention step that leaves a `lost+found` alone were not played; the hermetic test `deploy/test/backup-robust.sh` plays them.
- Native amd64 timing was not measured.
- A restore across a migration was not played; both the backup and the restore of this set are at the schema of `4563954`, and phase 34 records the crossing.

## Owner steps

None. Every step of this record ran in a container on the Mac, by the owner's decision (D-08); nothing here has to be done on the real server before the record counts.

## Hand-over to docs/beta.md

The sentences of this record that the page may quote, named by place and not copied here, so that each stays in exactly one place: BAK-01, the title sentences of F-1, F-2 and F-3 and the sentence after *What it shows.* in *BAK-01*; BAK-02, the title sentences of F-4 and F-7 and the sentence after *What it shows.* in *BAK-02*; BAK-03, the title sentence of F-5 and the sentence after *What it shows.* in *BAK-03*; the limits, the title sentence of F-6 and the bullets of *Not tested*.

The three limits of the page this record answers, named by their first words: "Bit rot in a complete backup is not caught" (a flipped byte inside a database page that passed the integrity check was refused by the checksum, F-1); "A restore does not report a row whose file the backup lacks" (it now names each such file twice and ends with status 3, F-4); "A failed removal in the retention step is silent" (the journal now names the directory and the unit ends with status 40, F-5).

The earlier evidence sets rest on a `backup.sh` and a `restore.sh` that no longer ship after the evidence commit `4563954`. Measured now with `git diff --numstat <set> 4563954 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md`:

| Set | Promise rows | `deploy/backup.sh` | `deploy/restore.sh` | `deploy/DEPLOY.md` |
|-----|--------------|--------------------|---------------------|--------------------|
| bf74496 | P-6 to P-10 | 241 insertions, 14 deletions | 292 insertions, 14 deletions | 185 insertions, 41 deletions |
| b501b1b | P-11 to P-15 | 241 insertions, 14 deletions | 292 insertions, 14 deletions | 146 insertions, 41 deletions |
| dd077e4 | P-16 to P-22 | 155 insertions, 12 deletions | 292 insertions, 14 deletions | 146 insertions, 41 deletions |

Nothing in this record is a promise until the page cites it byte for byte. A change to a bound file after the evidence commit means the set is made again before the record is cited for the changed text. `docs/beta.md` was not edited by this plan.

## Appendix: raw output

Each block is the scratch file of the evidence set, pasted unchanged by `bash deploy/test/backup-verify.sh appendix`, and `bash deploy/test/backup-verify.sh record-check` compares every block with its scratch file byte for byte. The files are in `deploy/test/out` (gitignored); `step-NN.txt`, `seed.txt`, the `.lst` and `.wall` files, `bak02-backup.out`, `v014-backup.sh`, the unit-start files and the log are not pasted.

#### amd64-t32-src/summary.txt

```text
---- summary (linux/amd64) ----
BAK-01 PASS
BAK-02 PASS
BAK-03 PASS
STAGES: bak01 bak02 bak03
DEVIATIONS: dropin emulation fixture lost-file mount old-script scp start-limit
BINARY: holzcloud v0.1.4-65-g4563954 (4563954)
```

#### amd64-t32-src/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
host uname -m:         arm64
container uname -m:    x86_64
container sha256sum:   sha256sum (GNU coreutils) 9.1
container bash:        GNU bash, version 5.2.15(1)-release (x86_64-pc-linux-gnu)
container systemctl:   systemd 252 (252.39-1~deb12u2)
binary:                holzcloud v0.1.4-65-g4563954 (4563954)
NEW_REF:               4563954
evidence commit:       4563954
```

#### amd64-t32-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:' was played as docker cp of deploy/test/out/bin/holzcloud-new-4563954 to /root/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /root in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[emulation]: the Docker host is aarch64; the linux/amd64 container runs under emulation (D-08), so every wall time of this set is an emulation figure, not a server's
DEVIATION[fixture]: the five plugin archives are copied into /opt/holzcloud/plugin-zips by this driver before run.sh seed, the step run.sh's baseline takes for an old install; they stand in for operator work in the admin and are no step of the document
DEVIATION[start-limit]: systemctl reset-failed holzcloud runs before each restore; several restores in a row restart the service more often than its start limit allows, which one restore on a server never meets
DEVIATION[old-script]: the backup.sh of v0.1.4 ran with this release's binary (as /opt/holzcloud/deploy-v014/backup.sh, with DATA_DIR and HOLZCLOUD_BIN as the unit sets them, into /opt/holzcloud/backups-v014); a server that kept the old script would run it with the binary of its own release
DEVIATION[lost-file]: the two files are removed from the live data directory before the backup, standing in for a file deleted between the snapshot and the copy (phase 29 measured that as ROW-WITHOUT-FILE; it is not provoked again here)
```

#### amd64-t32-src/bak01.txt

```text
MANUAL-RC: 0
SUMS-LINES: 37
SUMS-EQUALS-FIND: yes
SUMS-CHECK-RC: 0
SUMS-HAS-SQLITE: yes
SUMS-MEDIA-LINES: 23
MANIFEST-CHECKSUMS-LINE: yes
FLIP-MEDIA-FILE: ./media/1/65a035c0e3dbaa4364137d4c9572f204-medium.jpg
FLIP-MEDIA-RC: 1
INVARIANTS-AFTER-FLIP-MEDIA: same
FLIP-MEDIA-NAMED: ./media/1/65a035c0e3dbaa4364137d4c9572f204-medium.jpg
BITROT-OFFSET: 667395
BITROT-CHECK: integrity: ok
BITROT-RESTORE-RC: 1
INVARIANTS-AFTER-BITROT: same
BITROT-NAMED: ./holzcloud.sqlite
DELETED-FILE: ./media/1/bulk/bulk-9.bin
DELETED-RC: 1
INVARIANTS-AFTER-DELETED: same
DELETED-NAMED: ./media/1/bulk/bulk-9.bin
NOSUMS-RC: 1
INVARIANTS-AFTER-NOSUMS: same
NOSUMS-NAMED: yes
OLD-BACKUP-SCRIPT: v0.1.4 sha256 ac02fac9096d3ed1665794c87a1624f8c896bb980ae615d80be0d4bab7d8de96
OLD-BACKUP-RC: 0
OLD-RESTORE-RC: 0
OLD-NOTICE: yes
NEW-RESTORE-RC: 0
NEW-VERIFIED-LINE: Checksums: 37 files verified
BAK-01 PASS
```

#### amd64-t32-src/bak01-manual.out

```text
=== holzcloud-CMS backup: 20261113-140542 ===
Backing up database...
2026/11/13 14:05:42 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 14:05:42 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261113-140542/holzcloud.sqlite
2026/11/13 14:05:42 INFO backup verified path=/opt/holzcloud/backups/20261113-140542/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups/20261113-140542/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Writing checksums...
Pruning backups older than 30 days...
=== Backup complete: /opt/holzcloud/backups/20261113-140542 ===
120M	/opt/holzcloud/backups/20261113-140542
```

#### amd64-t32-src/bak01-flip-media.out

```text
1794578746.428014674 === Restoring from /root/t32/flip-media ===
1794578746.446625258 timestamp=20261113-140542
1794578746.464521424 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578746.488142383 data_dir=/opt/holzcloud/data
1794578746.508804716 host=f4a5303cc111
1794578746.527856174 checksums=SHA256SUMS
1794578746.544740799 Verifying checksums...
1794578747.103546800 ERROR: ./media/1/65a035c0e3dbaa4364137d4c9572f204-medium.jpg differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
WALL 1794578746.257438466 1794578747.140926716
```

#### amd64-t32-src/bak01-bitrot-check.out

```text
offset 667395
2026/11/13 14:05:48 INFO sqlite opened journal_mode=wal path=/root/t32/bitrot-check/holzcloud.sqlite
integrity: ok
status 0
```

#### amd64-t32-src/bak01-bitrot.out

```text
1794578749.705776509 === Restoring from /root/t32/bitrot ===
1794578749.729648843 timestamp=20261113-140542
1794578749.748781468 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578749.769588843 data_dir=/opt/holzcloud/data
1794578749.790208426 host=f4a5303cc111
1794578749.806238551 checksums=SHA256SUMS
1794578749.826196968 Verifying checksums...
1794578750.378664926 ERROR: ./holzcloud.sqlite differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
WALL 1794578749.538374217 1794578750.415699926
```

#### amd64-t32-src/bak01-deleted.out

```text
1794578752.333494719 === Restoring from /root/t32/deleted ===
1794578752.353545177 timestamp=20261113-140542
1794578752.375869427 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578752.392586010 data_dir=/opt/holzcloud/data
1794578752.416037927 host=f4a5303cc111
1794578752.433763469 checksums=SHA256SUMS
1794578752.453577052 Verifying checksums...
1794578753.000229844 ERROR: ./media/1/bulk/bulk-9.bin is listed in SHA256SUMS and missing — the backup is damaged, do not restore it
WALL 1794578752.166705219 1794578753.040038677
```

#### amd64-t32-src/bak01-nosums.out

```text
1794578754.971619678 === Restoring from /root/t32/nosums ===
1794578754.991620470 timestamp=20261113-140542
1794578755.008952720 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578755.025799970 data_dir=/opt/holzcloud/data
1794578755.042191762 host=f4a5303cc111
1794578755.058206720 checksums=SHA256SUMS
1794578755.074116303 ERROR: /root/t32/nosums/MANIFEST names SHA256SUMS but the file is missing — do not restore it
WALL 1794578754.802980387 1794578755.095715803
```

#### amd64-t32-src/bak01-old-backup.out

```text
=== holzcloud-CMS backup: 20261113-140556 ===
Backing up database...
2026/11/13 14:05:56 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 14:05:56 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups-v014/20261113-140556/holzcloud.sqlite
2026/11/13 14:05:56 INFO backup verified path=/opt/holzcloud/backups-v014/20261113-140556/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups-v014/20261113-140556/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Pruning backups older than 30 days...
=== Backup complete: /opt/holzcloud/backups-v014/20261113-140556 ===
120M	/opt/holzcloud/backups-v014/20261113-140556
```

#### amd64-t32-src/bak01-old-restore.out

```text
1794578757.703445513 === Restoring from /opt/holzcloud/backups-v014/20261113-140556 ===
1794578757.725654888 timestamp=20261113-140556
1794578757.743995721 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578757.763477221 data_dir=/opt/holzcloud/data
1794578757.781209721 host=f4a5303cc111
1794578757.800114680 NOTICE: /opt/holzcloud/backups-v014/20261113-140556 has no checksum file (made before 0.1.5); its files were not verified
1794578757.817560471 Verifying snapshot...
1794578757.948940721 2026/11/13 14:05:57 INFO sqlite opened journal_mode=wal path=/tmp/tmp.7BsM8JUlBR/holzcloud.sqlite
1794578757.972757013 integrity: ok
1794578757.991484180 files: 3 rows checked, 0 missing
1794578758.031506805 Stopping holzcloud...
1794578758.085511388 Moving current data to /opt/holzcloud/data.before-restore-20261113-140557
1794578758.118381138 Restoring database...
1794578758.155209347 Restoring CSRF key...
1794578758.175644888 Restoring media...
1794578758.375124972 Restoring plugins...
1794578758.507344347 Restoring sprachen...
1794578758.618895763 Restoring marke...
1794578758.757587430 Starting holzcloud...
1794578758.797319430 Waiting for readiness...
1794578769.550601796 === Restore complete ===
1794578769.597858629 {"status":"ok","version":"v0.1.4-65-g4563954","commit":"4563954","uptime_s":1,"integrity":"ok","disk_free_bytes":1589193687040,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794578769.615355879 
WALL 1794578757.533803513 1794578769.636767796
```

#### amd64-t32-src/bak01-new-restore.out

```text
1794578769.969117505 === Restoring from /opt/holzcloud/backups/20261113-140542 ===
1794578769.988558255 timestamp=20261113-140542
1794578770.007347171 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578770.025080380 data_dir=/opt/holzcloud/data
1794578770.043890546 host=f4a5303cc111
1794578770.061639338 checksums=SHA256SUMS
1794578770.081016838 Verifying checksums...
1794578770.656405088 Checksums: 37 files verified
1794578770.675669755 Verifying snapshot...
1794578770.854411463 2026/11/13 14:06:10 INFO sqlite opened journal_mode=wal path=/tmp/tmp.Epi4Cqb64T/holzcloud.sqlite
1794578770.871738463 integrity: ok
1794578770.890703630 files: 3 rows checked, 0 missing
1794578770.929687547 Stopping holzcloud...
1794578770.987493630 Moving current data to /opt/holzcloud/data.before-restore-20261113-140610
1794578771.020250838 Restoring database...
1794578771.052342963 Restoring CSRF key...
1794578771.071505588 Restoring media...
1794578771.278945339 Restoring plugins...
1794578771.411338255 Restoring sprachen...
1794578771.518057464 Restoring marke...
1794578771.657371505 Starting holzcloud...
1794578771.698599422 Waiting for readiness...
1794578782.423577052 === Restore complete ===
1794578782.471112594 {"status":"ok","version":"v0.1.4-65-g4563954","commit":"4563954","uptime_s":1,"integrity":"ok","disk_free_bytes":1589068201984,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794578782.490017636 
WALL 1794578769.798585046 1794578782.538881261
```

#### amd64-t32-src/bak02.txt

```text
LOST: media row 1 website 1 file 65a035c0e3dbaa4364137d4c9572f204.jpg
LOST: media_variants row 1 label thumb file 65a035c0e3dbaa4364137d4c9572f204-thumb.jpg
BACKUP-RC: 0
SUMS-LISTS-LOST: no
RESTORE-RC: 3
MISSING-BEFORE-QUESTION: 2
MISSING-AT-END: 2
READYZ: ok
BAK-02 PASS
```

#### amd64-t32-src/bak02-restore.out

```text
1794578785.430958637 === Restoring from /opt/holzcloud/backups/20261113-140623 ===
1794578785.453503845 timestamp=20261113-140623
1794578785.472113929 version=holzcloud v0.1.4-65-g4563954 (4563954)
1794578785.492715179 data_dir=/opt/holzcloud/data
1794578785.513446304 host=f4a5303cc111
1794578785.531213470 checksums=SHA256SUMS
1794578785.550099637 Verifying checksums...
1794578786.125273721 Checksums: 35 files verified
1794578786.145408429 Verifying snapshot...
1794578786.329252554 2026/11/13 14:06:26 INFO sqlite opened journal_mode=wal path=/tmp/tmp.EX57yMuEu7/holzcloud.sqlite
1794578786.347945346 integrity: ok
1794578786.368176304 files: 3 rows checked, 2 missing
1794578786.386418221 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204.jpg (media row 1, website 1, original name "velo-01.jpg")
1794578786.404203304 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
1794578786.422496512 a missing variant can be rebuilt with: holzcloud thumbnails
1794578786.461034304 Stopping holzcloud...
1794578786.516782721 Moving current data to /opt/holzcloud/data.before-restore-20261113-140626
1794578786.552310137 Restoring database...
1794578786.583497887 Restoring CSRF key...
1794578786.602156887 Restoring media...
1794578786.819700429 Restoring plugins...
1794578786.951169929 Restoring sprachen...
1794578787.059869596 Restoring marke...
1794578787.209389638 Starting holzcloud...
1794578787.254864096 Waiting for readiness...
1794578797.979939629 === Restore complete ===
1794578798.025282629 {"status":"ok","version":"v0.1.4-65-g4563954","commit":"4563954","uptime_s":1,"integrity":"ok","disk_free_bytes":1588817338368,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794578798.045879295 
1794578798.068427629 WARNING: restored, but 2 files that the database names are missing from the backup:
1794578798.084653504 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204.jpg (media row 1, website 1, original name "velo-01.jpg")
1794578798.101079795 MISSING media/1/65a035c0e3dbaa4364137d4c9572f204-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
WALL 1794578785.260304804 1794578798.122905629
```

#### amd64-t32-src/bak03.txt

```text
PLANTED: /opt/holzcloud/backups/20200101-031500 (locked: root-owned)
PLANTED: /opt/holzcloud/backups/20200102-031500
UNIT-DROPINS: ''
START-RC: 1
RESULT: exit-code
EXEC-MAIN-STATUS: 40
JOURNAL-NAMES-DIR: yes
LOCKED-STILL-THERE: yes
REMOVABLE-GONE: yes
NIGHT-HAS-MANIFEST: yes
BAK-03 PASS
```

#### amd64-t32-src/bak03-journal.txt

```text
=== holzcloud-CMS backup: 20261113-140638 ===
Backing up database...
2026/11/13 14:06:39 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 14:06:39 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261113-140638/holzcloud.sqlite
2026/11/13 14:06:39 INFO backup verified path=/opt/holzcloud/backups/20261113-140638/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups/20261113-140638/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Writing checksums...
Pruning backups older than 30 days...
rm: cannot remove '/opt/holzcloud/backups/20200101-031500/locked/keep': Permission denied
ERROR: retention could not remove /opt/holzcloud/backups/20200101-031500; it may be partly removed, remove it by hand
=== Backup complete: /opt/holzcloud/backups/20261113-140638 ===
120M	/opt/holzcloud/backups/20261113-140638
ERROR: the backup is complete, but retention failed (see above); ending with status 40
holzcloud-backup.service: Main process exited, code=exited, status=40/n/a
holzcloud-backup.service: Failed with result 'exit-code'.
Failed to start holzcloud-backup.service - holzcloud-CMS backup.
holzcloud-backup.service: Consumed 1.766s CPU time.
```

#### amd64-t32-src/measurements.txt

```text
BULK-MB: 100
BACKUP-MB: 120
BACKUP-FILES: 39
MANUAL-BACKUP-WALL-S: 1.9
SHA256SUM-C-WALL-S: 0.6
RESTORE-CHECKSUM-STEP-S: 0.9
```

#### amd64-t32-evidence-commit.txt

```text
4563954
```

#### amd64-t32-pass1.status

```text
0
```

