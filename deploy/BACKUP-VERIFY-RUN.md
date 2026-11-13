# A backup that carries its checksums, a restore that names what is missing, a retention step that speaks up, measured: results

This document is the permanent record of phase 32. It records what the changed `deploy/backup.sh` and `deploy/restore.sh` do, played in a container with both scripts exactly as committed: that a backup now carries a `SHA256SUMS` file and that `restore.sh` refuses a damaged backup before it touches anything, naming the first file that differs from its checksum or is missing, while a backup made before the checksum file existed still restores, with a notice (BAK-01); that a restore reports every file a database row names and the backup lacks, before its question and again at the end, and ends with status 3 (BAK-02); and that a retention step which cannot remove an old directory writes the directory into the journal and ends the night with status 40 under the shipped unit (BAK-03).

The record rests on one evidence set: a single `backup-verify.sh run` from one fresh container, made once against the committed text of every bound file (see *How this was run*). Its raw output is the Appendix, pasted unchanged, and `bash deploy/test/backup-verify.sh record-check` proves every block against its scratch file byte for byte. Every number in this document is a line of that set and cites its file in backticks as `DIRECTORY/FILE`, with `DIRECTORY` the one scratch directory `amd64-t32-src`; the run files beside it are named by their path below `deploy/test/out`. A file that is not pasted is named as `FILE in DIRECTORY`.

The record says what was done, how many times, and what was seen. It does not say that a backup is safe from tampering, because a checksum is no signature, and it does not say what a real disk does, because a container cannot. What the container cannot show is a named deviation or a line of *Not tested*, never a gap in the table.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| BAK-01 | PASS: `BAK-01 PASS` stands in `amd64-t32-src/summary.txt`. The document's manual backup line ended 0 and wrote a `SHA256SUMS` of 37 lines that names exactly the files of the directory, holds the database snapshot and 23 media lines, passed `sha256sum -c` and is named by the `MANIFEST` (`MANUAL-RC: 0`, `SUMS-LINES: 37`, `SUMS-EQUALS-FIND: yes`, `SUMS-CHECK-RC: 0`, `SUMS-HAS-SQLITE: yes`, `SUMS-MEDIA-LINES: 23`, `MANIFEST-CHECKSUMS-LINE: yes`). Four damaged copies were each refused with status 1 and the live service unchanged, naming the file: a flipped media byte, a flipped byte inside a database page that still read `BITROT-CHECK: integrity: ok`, a deleted listed file, and no `SHA256SUMS` (`FLIP-MEDIA-RC`, `BITROT-RESTORE-RC`, `DELETED-RC`, `NOSUMS-RC`, the four `INVARIANTS-AFTER-*: same`). A backup made by the `backup.sh` of v0.1.4 restored with `OLD-RESTORE-RC: 0` and `OLD-NOTICE: yes`, and the untouched new backup restored with `NEW-RESTORE-RC: 0` and `NEW-VERIFIED-LINE: Checksums: 37 files verified`, all in `amd64-t32-src/bak01.txt`. | Debian 12 container, systemd PID 1, linux/amd64 under emulation on an arm64 host (`amd64-t32-src/versions.txt`) | 2026-11-13, 0ce6d68 |
| BAK-02 | PASS: `BAK-02 PASS` stands in `amd64-t32-src/summary.txt`. An original and its `thumb` variant that rows name were removed before a backup (`LOST:` lines); the backup ended `BACKUP-RC: 0` and its checksum file does not list them (`SUMS-LISTS-LOST: no`); `restore.sh` named both before its question and again at the end (`MISSING-BEFORE-QUESTION: 2`, `MISSING-AT-END: 2`), restored the site (`READYZ: ok`) and ended `RESTORE-RC: 3`, all in `amd64-t32-src/bak02.txt`. | same | 2026-11-13, 0ce6d68 |
| BAK-03 | PASS: `BAK-03 PASS` stands in `amd64-t32-src/summary.txt`. The shipped `holzcloud-backup.service`, with no drop-in (`UNIT-DROPINS: ''`), met an old backup directory it could not remove; the journal names the directory (`JOURNAL-NAMES-DIR: yes`), the unit ended `RESULT: exit-code` and `EXEC-MAIN-STATUS: 40`, the locked directory is still there, the removable one is gone and the night's backup has its `MANIFEST` (`LOCKED-STILL-THERE: yes`, `REMOVABLE-GONE: yes`, `NIGHT-HAS-MANIFEST: yes`), all in `amd64-t32-src/bak03.txt`. | same | 2026-11-13, 0ce6d68 |

Evidence commit: 0ce6d68

The binary under test is the one built from `NEW_REF` (pinned by `run` to the short sha of HEAD, `0ce6d68`, read from `deploy/test/out/amd64-t32-evidence-commit.txt`) with the Build line of `deploy/DEPLOY.md`: `BINARY: holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)` in `amd64-t32-src/summary.txt`, and the same in the lines `binary:`, `NEW_REF:` and `evidence commit:` of `amd64-t32-src/versions.txt`. The summary carries `STAGES: bak01 bak02 bak03` and no `REHEARSAL` line; every restore of the set prints `version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)` from the binary it ran with.

## How this was run

`deploy/test/backup-verify.sh` is a sibling of `deploy/test/run.sh` and `deploy/test/nightly.sh` (neither is edited) and reaches the harness through `run.sh`'s public subcommands only (`build`, `up`, `install`, `postinstall`, `seed`, `backup-fixture`). `run` refuses to start unless `PLATFORM=linux/amd64` and `TARGET=systemd-base`, refuses an existing container `holzcloud-t32`, refuses while one of the bound files differs from HEAD or is untracked, pins `NEW_REF` to the short sha of HEAD so that the binary and the scripts are one commit, and plays the stages `bak01`, `bak02` and `bak03` in one container. The verdict of each requirement is computed from the `KEY: value` files the stages wrote, each key exactly once, and a stage that did not run reads NOT-RUN, so a trace can never pass for the set.

The set was made once, from a fresh container (`holzcloud-t32`), on the clean committed tree of `0ce6d68` without `REHEARSAL`, started 2026-11-13T08:06:51Z and ended 08:08:40Z on the host clock (1 minute 49 seconds), exit status 0 (`deploy/test/out/amd64-t32-pass1.started`, `deploy/test/out/amd64-t32-pass1.finished` and `deploy/test/out/amd64-t32-pass1.status`). Before it, the worktree held no earlier output of the driver, and after it `bash deploy/test/backup-verify.sh down` removed the container. No second set was made, and no other set is cited. The command was:

```text
PLATFORM=linux/amd64 TARGET=systemd-base bash deploy/test/backup-verify.sh run
```

The knobs not set on that line had their defaults: `STAGES=bak01 bak02 bak03` and `BULK_MB=100` (100 MB of synthetic media in the data directory, `BULK-MB: 100` in `amd64-t32-src/measurements.txt`). The log of the run is `deploy/test/out/amd64-t32-pass1.log`, which is not pasted; its first line reads `backup-verify: platform linux/amd64, NEW_REF pinned to 0ce6d68, stages: bak01 bak02 bak03, BULK_MB=100`.

No bound file changed after the evidence commit. The bound files for this set are `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh`, `deploy/test/Dockerfile` and `deploy/test/backup-verify.sh`, together with the binary built from `cmd`, `internal`, `go.mod` and `go.sum`; this command exited 0 when the record was written:

```text
git diff --quiet 0ce6d68 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/Dockerfile deploy/test/backup-verify.sh cmd internal go.mod go.sum
```

`deploy/test/backup-robust.sh` and `deploy/test/restore-robust.sh`, the hermetic tests of the two changed scripts (they need no Docker and build a fake binary, a fake `rsync` and a fake `date`), are not part of the set, are not bound files and their output is not pasted. When this record was written they ended `BACKUP-ROBUST OK (124 cases)` and `RESTORE-ROBUST OK (66 cases)`, and `bash deploy/test/backup-verify.sh selftest` ended `SELFTEST OK (117 cases)`.

## Deviations named

Everything here precedes every result: it is what the container does differently from a real server and what the harness did that the document does not contain. The first column is the name written to `amd64-t32-src/deviations.txt`; the kinds in the summary are `dropin emulation fixture lost-file mount old-script scp start-limit` (the `DEVIATIONS:` line of `amd64-t32-src/summary.txt`).

| Name | Why | What a real server does instead |
|------|-----|---------------------------------|
| DEVIATION[emulation] | The Docker host is aarch64 and the linux/amd64 container runs under emulation (`host architecture: aarch64`, `container uname -m: x86_64` in `amd64-t32-src/versions.txt`), so every wall time of this set is an emulation figure | A native amd64 CPU and disk |
| DEVIATION[start-limit] | `systemctl reset-failed holzcloud` runs before each restore, because several restores in a row restart the service more often than its start limit allows. The unit is not changed | One restore, by hand, which never meets the limit |
| DEVIATION[dropin] | `/etc/systemd/system/holzcloud.service.d/container.conf` sets `ProcSubset=all` on `holzcloud.service`, because the document's unchanged start crashed in the container (`unit-start-unchanged.txt` in `amd64-t32-src`). The backup units have no drop-in (`UNIT-DROPINS: ''` in `amd64-t32-src/bak03.txt`) | The unit as shipped |
| DEVIATION[scp] | The two `scp` lines of the document were played as `docker cp` of the binary and as a tar stream of the committed `deploy` directory into the container | `scp` from the build machine |
| DEVIATION[fixture] | Two lines: run.sh's `backup-fixture` wrote a language file, a logo and 100 MB of synthetic media under `data/media/WEBSITE/bulk`; the driver copied the five plugin archives into `/opt/holzcloud/plugin-zips` before `run.sh seed`. They stand in for operator work and are no step of the document | The operator's own content and plugins |
| DEVIATION[old-script] | The `backup.sh` of v0.1.4 ran with this release's binary, as `/opt/holzcloud/deploy-v014/backup.sh` with `DATA_DIR` and `HOLZCLOUD_BIN` as the unit sets them, into `/opt/holzcloud/backups-v014`; its sha256 is `OLD-BACKUP-SCRIPT` in `amd64-t32-src/bak01.txt` | A server that kept the old script runs it with the binary of its own release |
| DEVIATION[lost-file] | The two files of BAK-02 were removed from the live data directory before the backup, standing in for a file deleted between the snapshot and the copy. Phase 29 measured that window as ROW-WITHOUT-FILE (`deploy/BACKUP-ROBUST-RUN.md`); it is not provoked again here | A picture deleted in the admin while the media copy runs |
| DEVIATION[mount] | Inherited from phases 21 and 22: the unit's `RequiresMountsFor` names `/opt/holzcloud/data`, a plain directory on the container's root filesystem, so that line was not exercised | A data directory on its own mount |

The obstacle of BAK-03 is not a deviation kind of the file and is named here all the same: the old directory that retention cannot remove is made so by a subdirectory owned by root (`PLANTED: /opt/holzcloud/backups/20200101-031500 (locked: root-owned)` in `amd64-t32-src/bak03.txt`). On a real server the cause would be a disk error, a read-only filesystem or a permission an operator changed (*Container vs. server*).

## Findings

Each finding has the class of the earlier records' rule: (1) a bug of the harness, fixed in the driver; (2) a limit of the container, a named deviation or a *Not tested* line; (3) a measured property of a shipped file that matches what it and its document say, recorded and not edited; (4) a defect, a shipped file doing less than it or its document says, with a fix in a commit of its own. No finding is class 1 or class 4, and no shipped file was edited after the evidence commit.

The test for class 4 was whether a result contradicts something `deploy/backup.sh`, `deploy/restore.sh` or `deploy/DEPLOY.md` says as committed at `0ce6d68`, or leaves the operator worse off than they say. The document says that `SHA256SUMS` holds the checksum of every other regular file of the backup, that `restore.sh` checks it before it touches anything and refuses the backup naming the first file that differs or is missing, that a backup made before 0.1.5 is restored with a notice, that files a row names and the backup lacks are listed before the question and again after the restore and the script ends with status 3, and that a retention failure shows `ERROR: retention could not remove` in the journal and ends the night with status 40. F-1 to F-5 are those sentences observed; F-6 and F-7 are what they do not cover.

**F-1 (class 3): A flipped byte inside a database page that still passed the integrity check was refused by the checksum, naming `./holzcloud.sqlite`, before anything was stopped.** The byte at offset 667395 of a copy of the snapshot was flipped, and a separate copy with the same flip read `integrity: ok` with status 0 (`amd64-t32-src/bak01-bitrot-check.out`, `BITROT-OFFSET: 667395` and `BITROT-CHECK: integrity: ok` in `amd64-t32-src/bak01.txt`). `restore.sh` refused it with status 1 (`BITROT-RESTORE-RC: 1`, `BITROT-NAMED: ./holzcloud.sqlite`, `INVARIANTS-AFTER-BITROT: same`). The line (`amd64-t32-src/bak01-bitrot.out`):

```text
1794557269.413649929 ERROR: ./holzcloud.sqlite differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
```

**F-2 (class 3): A flipped media byte, a deleted listed file and a missing SHA256SUMS were each refused with status 1, naming the file, with the inode of the live database, the invocation of the service, its state and the hash of the other files of the data directory unchanged.** The media byte was flipped in `./media/1/75a724e332c96f45816373287fbb025d-medium.jpg` and the restore named that file; the deleted file was `./media/1/bulk/bulk-9.bin` and the restore named it as listed and missing; the copy without `SHA256SUMS` was refused because its `MANIFEST` names the file (`FLIP-MEDIA-FILE`, `FLIP-MEDIA-NAMED`, `DELETED-FILE`, `DELETED-NAMED`, `NOSUMS-NAMED: yes` and the `INVARIANTS-AFTER-*` lines of `amd64-t32-src/bak01.txt`; the invariants before and after are `inv-*-before.lst` and `inv-*-after.lst` in `amd64-t32-src`). The lines (`amd64-t32-src/bak01-flip-media.out`, `amd64-t32-src/bak01-deleted.out`, `amd64-t32-src/bak01-nosums.out`):

```text
1794557266.193852553 ERROR: ./media/1/75a724e332c96f45816373287fbb025d-medium.jpg differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
1794557271.939971014 ERROR: ./media/1/bulk/bulk-9.bin is listed in SHA256SUMS and missing — the backup is damaged, do not restore it
1794557273.985282459 ERROR: /root/t32/nosums/MANIFEST names SHA256SUMS but the file is missing — do not restore it
```

**F-3 (class 3): A backup made by the backup.sh of v0.1.4 restored with status 0 and the notice that its files were not verified.** The old script wrote no `Writing checksums...` line (`amd64-t32-src/bak01-old-backup.out`), and the restore printed the notice in place of the verification and went on to the snapshot check (`OLD-RESTORE-RC: 0`, `OLD-NOTICE: yes` in `amd64-t32-src/bak01.txt`). The line (`amd64-t32-src/bak01-old-restore.out`):

```text
1794557276.813849793 NOTICE: /opt/holzcloud/backups-v014/20261113-080755 has no checksum file (made before 0.1.5); its files were not verified
```

**F-4 (class 3): Two files that rows name and the backup lacked were listed before the question and again at the end, the site was restored, and restore.sh ended with status 3.** The lines (`amd64-t32-src/bak02-restore.out`):

```text
1794557304.007063917 files: 3 rows checked, 2 missing
1794557304.024540792 MISSING media/1/75a724e332c96f45816373287fbb025d.jpg (media row 1, website 1, original name "velo-01.jpg")
1794557304.044279167 MISSING media/1/75a724e332c96f45816373287fbb025d-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
1794557314.628997839 WARNING: restored, but 2 files that the database names are missing from the backup:
```

**F-5 (class 3): Under the shipped unit with no drop-in, a retention step that could not remove an old directory wrote an ERROR line naming it into the journal, the night's backup was complete, and the unit ended with Result exit-code and ExecMainStatus 40.** The removal error of `rm` is no longer discarded, the script names the directory and says what the status means (`amd64-t32-src/bak03-journal.txt`; `RESULT: exit-code`, `EXEC-MAIN-STATUS: 40`, `NIGHT-HAS-MANIFEST: yes` in `amd64-t32-src/bak03.txt`):

```text
rm: cannot remove '/opt/holzcloud/backups/20200101-031500/locked/keep': Permission denied
ERROR: retention could not remove /opt/holzcloud/backups/20200101-031500; it may be partly removed, remove it by hand
=== Backup complete: /opt/holzcloud/backups/20261113-080835 ===
ERROR: the backup is complete, but retention failed (see above); ending with status 40
holzcloud-backup.service: Main process exited, code=exited, status=40/n/a
```

**F-6 (class 3, read from the text and not played): The checksum covers only the regular files listed when the backup was made, so a file added to the backup directory afterwards is restored without being verified.** `backup.sh` writes one line per regular file of the night except `SHA256SUMS` and the `MANIFEST`, and `restore.sh` hashes the listed paths only; a file that is in the directory and not in the list is neither refused nor named. Escaped names (a backslash or a newline in the name) are skipped with a `NOTICE` line. A checksum detects rot and accidental damage, not tampering: whoever can write the backup can rewrite a file and its line alike, as `deploy/DEPLOY.md` says. Neither case was played in this set (*Not tested*).

**F-7 (class 3): A file that was never in the backup is not a checksum failure; the checksum passed and the gap was found by the row check.** The backup of BAK-02 ended 0, its `SHA256SUMS` does not list the two lost files (`BACKUP-RC: 0`, `SUMS-LISTS-LOST: no` in `amd64-t32-src/bak02.txt`; its output is `bak02-backup.out` in `amd64-t32-src`), and the restore verified the 35 files it listed before `holzcloud check -files` named the two (`1794557303.772344959 Checksums: 35 files verified` in `amd64-t32-src/bak02-restore.out`). The two checks answer different questions, and BAK-01 and BAK-02 together are the answer to both.

## BAK-01: a backup carries its checksums and a damaged one is refused

**What was done.** The document's manual backup line (`sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`) was run by its exact text; its output ends with `Writing checksums...`, `Pruning backups older than 30 days...` and `=== Backup complete: /opt/holzcloud/backups/20261113-080741 ===` (`amd64-t32-src/bak01-manual.out`). The path column of the new `SHA256SUMS` was compared with a `find` of the directory and `sha256sum -c` was run on it in the container (`SUMS-LINES: 37`, `SUMS-EQUALS-FIND: yes`, `SUMS-CHECK-RC: 0`, `SUMS-HAS-SQLITE: yes`, `SUMS-MEDIA-LINES: 23`, `MANIFEST-CHECKSUMS-LINE: yes` in `amd64-t32-src/bak01.txt`; the lists are `bak01-sums.lst`, `bak01-find.lst` and `bak01-dir.lst` in `amd64-t32-src`). Four copies of that backup below `/root/t32` were damaged, one way each, and handed to the shipped `restore.sh` with the invariants of the live site taken before and after: a flipped media byte (F-2), a flipped byte inside a database page that still passed `holzcloud check` (F-1), a deleted listed file (F-2) and a removed `SHA256SUMS` (F-2). Then the `backup.sh` of v0.1.4 made a backup with this release's binary (`OLD-BACKUP-RC: 0`, `DEVIATION[old-script]`), which was restored (F-3), and the untouched new backup was restored last:

```text
1794557288.477449966 Checksums: 37 files verified
```

(`amd64-t32-src/bak01-new-restore.out`; `NEW-RESTORE-RC: 0` and `NEW-VERIFIED-LINE: Checksums: 37 files verified` in `amd64-t32-src/bak01.txt`).

*What it shows.* A backup made by the document's manual line carried a `SHA256SUMS` that listed each of its 37 files and passed `sha256sum -c`, and `restore.sh` refused each of four damaged copies of it (a flipped media byte, a flipped database byte that still passed the integrity check, a deleted file and a missing checksum file) with status 1, naming the first file at fault and leaving the live site unchanged, while a backup made by the `backup.sh` of v0.1.4 restored with status 0 and the notice that its files were not verified.

**What it does not show.** Real bit rot on a disk, which flips what it likes and not a byte chosen to keep the integrity check green. A tampered backup, which a checksum cannot catch (F-6). A file with a backslash or a newline in its name, which the hermetic test plays and this set does not.

## BAK-02: a restore names the files a row expects and the backup lacks

**What was done.** An original picture and its `thumb` variant, both named by rows, were removed from the live data directory (`DEVIATION[lost-file]`), the document's backup line was run, and the backup was restored with `restore.sh`, answering `yes` at its question, with every output line stamped by the container clock. The two files (`amd64-t32-src/bak02.txt`):

```text
LOST: media row 1 website 1 file 75a724e332c96f45816373287fbb025d.jpg
LOST: media_variants row 1 label thumb file 75a724e332c96f45816373287fbb025d-thumb.jpg
```

The restore verified 35 checksums, read `integrity: ok`, printed `files: 3 rows checked, 2 missing` and the two `MISSING` lines with the row, the website and the original name, stopped the service, moved the data aside, restored, waited for readiness, printed the `/readyz` answer with `"status":"ok"`, and then repeated both `MISSING` lines under the `WARNING:` line (F-4, `amd64-t32-src/bak02-restore.out`). The verdict lines are `MISSING-BEFORE-QUESTION: 2`, `MISSING-AT-END: 2`, `READYZ: ok` and `RESTORE-RC: 3` (`amd64-t32-src/bak02.txt`).

*What it shows.* When a media row and a variant row named files that the backup lacked, the backup still ended with status 0, and `restore.sh` restored the site, listed both files before its question and again at the end, and ended with status 3.

**What it does not show.** The window of a deletion between the snapshot and the copy, which phase 29 measured and which is stood in for here (`DEVIATION[lost-file]`). A restore with an installed `holzcloud` older than 0.1.5, which cannot check rows and says so in a `NOTICE` line; the hermetic test plays it and this set does not. Rebuilding the variant with `holzcloud thumbnails`, which the restore names and this set did not run.

## BAK-03: a retention step that cannot remove a directory says so

**What was done.** Two old backup directories dated 2020 were planted below `/opt/holzcloud/backups`, one of them holding a subdirectory owned by root that the user `holzcloud` cannot empty (`PLANTED:` lines of `amd64-t32-src/bak03.txt`). The shipped `holzcloud-backup.service`, with no drop-in (`UNIT-DROPINS: ''`), was started with `systemctl start`, which returned 1 (`START-RC: 1`); then `systemctl show -p Result -p ExecMainStatus` was read and the journal of that invocation kept (`amd64-t32-src/bak03-journal.txt`). The journal holds the whole night: the database, `csrf.key`, media, plugins, language files and logo, `Writing checksums...`, `Pruning backups older than 30 days...`, the `rm` error, the `ERROR: retention could not remove` line with the directory, `=== Backup complete: /opt/holzcloud/backups/20261113-080835 ===`, `120M` and the closing `ERROR:` line with status 40 (F-5).

*What it shows.* Under the shipped `holzcloud-backup.service` with no drop-in, an old backup directory that could not be removed put the line `ERROR: retention could not remove` with that directory into the journal, while the night's backup was complete with its `MANIFEST` and the removable old directory was gone, and the unit ended with `Result=exit-code` and `ExecMainStatus=40`.

**What it does not show.** The timer firing at 03:15 with the changed script, the catch-up and the random delay. A removal that fails on a real disk or a read-only filesystem. `ERROR: retention could not list`, the other line the document names, which was not provoked. The off-site copy after a retention failure, which `DEPLOY.md` says still runs and which this set does not configure.

## Measurements

The figures the harness wrote. Every one is an emulation figure on an aarch64 host (`amd64-t32-src/versions.txt`); none is a promise of speed.

| Figure | Value | Where |
|--------|-------|-------|
| Synthetic media in the data directory | 100 MB | `BULK-MB` in `amd64-t32-src/measurements.txt` |
| Size of a backup | 120 MB, 39 files (37 listed in `SHA256SUMS`, plus `SHA256SUMS` and the `MANIFEST`) | `BACKUP-MB`, `BACKUP-FILES` in `amd64-t32-src/measurements.txt`; `SUMS-LINES` in `amd64-t32-src/bak01.txt` |
| The manual backup, checksums included | 1.8 s wall | `MANUAL-BACKUP-WALL-S` in `amd64-t32-src/measurements.txt` |
| `sha256sum -c` over the backup | 0.6 s wall | `SHA256SUM-C-WALL-S` in `amd64-t32-src/measurements.txt` |
| The checksum step of `restore.sh` | 0.8 s | `RESTORE-CHECKSUM-STEP-S` in `amd64-t32-src/measurements.txt` |
| A refused restore, from its first line to the `ERROR:` line | 1794557265.540066552 to 1794557266.193852553 (the media flip) | `amd64-t32-src/bak01-flip-media.out` |
| The whole run, from its started file to its finished file | 08:06:51Z to 08:08:40Z | `deploy/test/out/amd64-t32-pass1.started`, `deploy/test/out/amd64-t32-pass1.finished` |

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
- A file added to a backup directory after the night, which the checksum does not list and the restore does not verify, was not played (F-6).
- The timer firing at 03:15 with the changed script was not played; the night of BAK-03 was started with `systemctl start`.
- The window of a deletion between the snapshot and the copy was not provoked; the two files of BAK-02 were removed before the backup (`DEVIATION[lost-file]`).
- A file name with a backslash or a newline was played by the hermetic test `deploy/test/restore-robust.sh` only.
- An older binary with the new `restore.sh`, which cannot check rows and says so, was played by the hermetic test only.
- The refusal of a database whose schema is newer than the binary (STB-03) is proven by the tests of this phase and is recorded by phase 34; it was not played here.
- `ERROR: retention could not list` and the off-site copy after a retention failure were not played.
- Native amd64 timing was not measured.
- A restore across a migration was not played; both the backup and the restore of this set are at the schema of `0ce6d68`, and phase 34 records the crossing.

## Owner steps

None. Every step of this record ran in a container on the Mac, by the owner's decision (D-08); nothing here has to be done on the real server before the record counts.

## Hand-over to docs/beta.md

The sentences of this record that the page may quote, named by place and not copied here, so that each stays in exactly one place: BAK-01, the title sentences of F-1, F-2 and F-3 and the sentence after *What it shows.* in *BAK-01*; BAK-02, the title sentences of F-4 and F-7 and the sentence after *What it shows.* in *BAK-02*; BAK-03, the title sentence of F-5 and the sentence after *What it shows.* in *BAK-03*; the limits, the title sentence of F-6 and the bullets of *Not tested*.

The three limits of the page this record answers, named by their first words: "Bit rot in a complete backup is not caught" (a flipped byte inside a database page that passed the integrity check was refused by the checksum, F-1); "A restore does not report a row whose file the backup lacks" (it now names each such file twice and ends with status 3, F-4); "A failed removal in the retention step is silent" (the journal now names the directory and the unit ends with status 40, F-5).

The earlier evidence sets rest on a `backup.sh` and a `restore.sh` that no longer ship after the evidence commit `0ce6d68`. Measured now with `git diff --numstat <set> 0ce6d68 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md`:

| Set | Promise rows | `deploy/backup.sh` | `deploy/restore.sh` | `deploy/DEPLOY.md` |
|-----|--------------|--------------------|---------------------|--------------------|
| bf74496 | P-6 to P-10 | 175 insertions, 11 deletions | 124 insertions, 6 deletions | 141 insertions, 26 deletions |
| b501b1b | P-11 to P-15 | 175 insertions, 11 deletions | 124 insertions, 6 deletions | 102 insertions, 26 deletions |
| dd077e4 | P-16 to P-22 | 88 insertions, 8 deletions | 124 insertions, 6 deletions | 102 insertions, 26 deletions |

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
BINARY: holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
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
binary:                holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
NEW_REF:               0ce6d68
evidence commit:       0ce6d68
```

#### amd64-t32-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-0ce6d68 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
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
FLIP-MEDIA-FILE: ./media/1/75a724e332c96f45816373287fbb025d-medium.jpg
FLIP-MEDIA-RC: 1
INVARIANTS-AFTER-FLIP-MEDIA: same
FLIP-MEDIA-NAMED: ./media/1/75a724e332c96f45816373287fbb025d-medium.jpg
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
=== holzcloud-CMS backup: 20261113-080741 ===
Backing up database...
2026/11/13 08:07:41 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 08:07:41 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261113-080741/holzcloud.sqlite
2026/11/13 08:07:41 INFO backup verified path=/opt/holzcloud/backups/20261113-080741/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups/20261113-080741/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Writing checksums...
Pruning backups older than 30 days...
=== Backup complete: /opt/holzcloud/backups/20261113-080741 ===
120M	/opt/holzcloud/backups/20261113-080741
```

#### amd64-t32-src/bak01-flip-media.out

```text
1794557265.540066552 === Restoring from /root/t32/flip-media ===
1794557265.558950261 timestamp=20261113-080741
1794557265.579735677 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557265.598011844 data_dir=/opt/holzcloud/data
1794557265.617468969 host=96bf35f92900
1794557265.636927094 checksums=SHA256SUMS
1794557265.653303802 Verifying checksums...
1794557266.193852553 ERROR: ./media/1/75a724e332c96f45816373287fbb025d-medium.jpg differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
WALL 1794557265.392870094 1794557266.220208803
```

#### amd64-t32-src/bak01-bitrot-check.out

```text
offset 667395
2026/11/13 08:07:47 INFO sqlite opened journal_mode=wal path=/root/t32/bitrot-check/holzcloud.sqlite
integrity: ok
status 0
```

#### amd64-t32-src/bak01-bitrot.out

```text
1794557268.770306387 === Restoring from /root/t32/bitrot ===
1794557268.789475096 timestamp=20261113-080741
1794557268.807060304 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557268.828214804 data_dir=/opt/holzcloud/data
1794557268.847491429 host=96bf35f92900
1794557268.867808554 checksums=SHA256SUMS
1794557268.886599637 Verifying checksums...
1794557269.413649929 ERROR: ./holzcloud.sqlite differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it
WALL 1794557268.630460971 1794557269.438090221
```

#### amd64-t32-src/bak01-deleted.out

```text
1794557271.306306222 === Restoring from /root/t32/deleted ===
1794557271.326896097 timestamp=20261113-080741
1794557271.346087055 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557271.369316347 data_dir=/opt/holzcloud/data
1794557271.388405305 host=96bf35f92900
1794557271.407942722 checksums=SHA256SUMS
1794557271.428232805 Verifying checksums...
1794557271.939971014 ERROR: ./media/1/bulk/bulk-9.bin is listed in SHA256SUMS and missing — the backup is damaged, do not restore it
WALL 1794557271.164730430 1794557271.962938055
```

#### amd64-t32-src/bak01-nosums.out

```text
1794557273.885420250 === Restoring from /root/t32/nosums ===
1794557273.904307042 timestamp=20261113-080741
1794557273.920381875 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557273.936268709 data_dir=/opt/holzcloud/data
1794557273.952646042 host=96bf35f92900
1794557273.968410334 checksums=SHA256SUMS
1794557273.985282459 ERROR: /root/t32/nosums/MANIFEST names SHA256SUMS but the file is missing — do not restore it
WALL 1794557273.735179917 1794557274.006301167
```

#### amd64-t32-src/bak01-old-backup.out

```text
=== holzcloud-CMS backup: 20261113-080755 ===
Backing up database...
2026/11/13 08:07:55 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 08:07:55 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups-v014/20261113-080755/holzcloud.sqlite
2026/11/13 08:07:55 INFO backup verified path=/opt/holzcloud/backups-v014/20261113-080755/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups-v014/20261113-080755/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Pruning backups older than 30 days...
=== Backup complete: /opt/holzcloud/backups-v014/20261113-080755 ===
120M	/opt/holzcloud/backups-v014/20261113-080755
```

#### amd64-t32-src/bak01-old-restore.out

```text
1794557276.709369877 === Restoring from /opt/holzcloud/backups-v014/20261113-080755 ===
1794557276.731108002 timestamp=20261113-080755
1794557276.750921877 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557276.773310460 data_dir=/opt/holzcloud/data
1794557276.794850293 host=96bf35f92900
1794557276.813849793 NOTICE: /opt/holzcloud/backups-v014/20261113-080755 has no checksum file (made before 0.1.5); its files were not verified
1794557276.831620502 Verifying snapshot...
1794557276.961851419 2026/11/13 08:07:56 INFO sqlite opened journal_mode=wal path=/tmp/tmp.jwofh7t6zk/holzcloud.sqlite
1794557276.980202669 integrity: ok
1794557276.999463252 files: 3 rows checked, 0 missing
1794557277.018006835 Stopping holzcloud...
1794557277.065985127 Moving current data to /opt/holzcloud/data.before-restore-20261113-080757
1794557277.099011419 Restoring database...
1794557277.130265919 Restoring CSRF key...
1794557277.151672794 Restoring media...
1794557277.353367585 Restoring plugins...
1794557277.486919044 Restoring sprachen...
1794557277.598120502 Restoring marke...
1794557277.738937336 Starting holzcloud...
1794557277.779849502 Waiting for readiness...
1794557287.428302799 === Restore complete ===
1794557287.478133174 {"status":"ok","version":"v0.1.4-47-g0ce6d68","commit":"0ce6d68","uptime_s":0,"integrity":"ok","disk_free_bytes":1589193801728,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794557287.493981090 
WALL 1794557276.560750460 1794557287.515839674
```

#### amd64-t32-src/bak01-new-restore.out

```text
1794557287.825621924 === Restoring from /opt/holzcloud/backups/20261113-080741 ===
1794557287.848375965 timestamp=20261113-080741
1794557287.867205132 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557287.884321507 data_dir=/opt/holzcloud/data
1794557287.901387674 host=96bf35f92900
1794557287.925961882 checksums=SHA256SUMS
1794557287.948528924 Verifying checksums...
1794557288.477449966 Checksums: 37 files verified
1794557288.498434549 Verifying snapshot...
1794557288.672148966 2026/11/13 08:08:08 INFO sqlite opened journal_mode=wal path=/tmp/tmp.8TdIUXjf7k/holzcloud.sqlite
1794557288.688307841 integrity: ok
1794557288.709440466 files: 3 rows checked, 0 missing
1794557288.727572799 Stopping holzcloud...
1794557288.781601049 Moving current data to /opt/holzcloud/data.before-restore-20261113-080808
1794557288.814061466 Restoring database...
1794557288.847124383 Restoring CSRF key...
1794557288.866077174 Restoring media...
1794557289.068551174 Restoring plugins...
1794557289.199008049 Restoring sprachen...
1794557289.305570466 Restoring marke...
1794557289.454272049 Starting holzcloud...
1794557289.494878466 Waiting for readiness...
1794557300.220389138 === Restore complete ===
1794557300.268737263 {"status":"ok","version":"v0.1.4-47-g0ce6d68","commit":"0ce6d68","uptime_s":1,"integrity":"ok","disk_free_bytes":1589068316672,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794557300.289065888 
WALL 1794557287.676203132 1794557300.317611930
```

#### amd64-t32-src/bak02.txt

```text
LOST: media row 1 website 1 file 75a724e332c96f45816373287fbb025d.jpg
LOST: media_variants row 1 label thumb file 75a724e332c96f45816373287fbb025d-thumb.jpg
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
1794557303.108143417 === Restoring from /opt/holzcloud/backups/20261113-080821 ===
1794557303.132273125 timestamp=20261113-080821
1794557303.151240208 version=holzcloud v0.1.4-47-g0ce6d68 (0ce6d68)
1794557303.169718000 data_dir=/opt/holzcloud/data
1794557303.187492250 host=96bf35f92900
1794557303.207349208 checksums=SHA256SUMS
1794557303.226243875 Verifying checksums...
1794557303.772344959 Checksums: 35 files verified
1794557303.795007375 Verifying snapshot...
1794557303.970161917 2026/11/13 08:08:23 INFO sqlite opened journal_mode=wal path=/tmp/tmp.E4WTF4WnGk/holzcloud.sqlite
1794557303.988608042 integrity: ok
1794557304.007063917 files: 3 rows checked, 2 missing
1794557304.024540792 MISSING media/1/75a724e332c96f45816373287fbb025d.jpg (media row 1, website 1, original name "velo-01.jpg")
1794557304.044279167 MISSING media/1/75a724e332c96f45816373287fbb025d-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
1794557304.062312417 a missing variant can be rebuilt with: holzcloud thumbnails
1794557304.079416125 Stopping holzcloud...
1794557304.144141084 Moving current data to /opt/holzcloud/data.before-restore-20261113-080824
1794557304.175671084 Restoring database...
1794557304.207578209 Restoring CSRF key...
1794557304.231784334 Restoring media...
1794557304.445523167 Restoring plugins...
1794557304.580657959 Restoring sprachen...
1794557304.690855959 Restoring marke...
1794557304.835752751 Starting holzcloud...
1794557304.874708418 Waiting for readiness...
1794557314.540259422 === Restore complete ===
1794557314.588171672 {"status":"ok","version":"v0.1.4-47-g0ce6d68","commit":"0ce6d68","uptime_s":0,"integrity":"ok","disk_free_bytes":1588817580032,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1794557314.610901381 
1794557314.628997839 WARNING: restored, but 2 files that the database names are missing from the backup:
1794557314.644925797 MISSING media/1/75a724e332c96f45816373287fbb025d.jpg (media row 1, website 1, original name "velo-01.jpg")
1794557314.661902089 MISSING media/1/75a724e332c96f45816373287fbb025d-thumb.jpg (media_variants row 1, label "thumb", of media row 1)
WALL 1794557302.960374458 1794557314.682800089
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
=== holzcloud-CMS backup: 20261113-080835 ===
Backing up database...
2026/11/13 08:08:35 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/11/13 08:08:35 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261113-080835/holzcloud.sqlite
2026/11/13 08:08:35 INFO backup verified path=/opt/holzcloud/backups/20261113-080835/holzcloud.sqlite tables=55
backup written and verified: /opt/holzcloud/backups/20261113-080835/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Writing checksums...
Pruning backups older than 30 days...
rm: cannot remove '/opt/holzcloud/backups/20200101-031500/locked/keep': Permission denied
ERROR: retention could not remove /opt/holzcloud/backups/20200101-031500; it may be partly removed, remove it by hand
=== Backup complete: /opt/holzcloud/backups/20261113-080835 ===
120M	/opt/holzcloud/backups/20261113-080835
ERROR: the backup is complete, but retention failed (see above); ending with status 40
holzcloud-backup.service: Main process exited, code=exited, status=40/n/a
holzcloud-backup.service: Failed with result 'exit-code'.
Failed to start holzcloud-backup.service - holzcloud-CMS backup.
holzcloud-backup.service: Consumed 1.673s CPU time.
```

#### amd64-t32-src/measurements.txt

```text
BULK-MB: 100
BACKUP-MB: 120
BACKUP-FILES: 39
MANUAL-BACKUP-WALL-S: 1.8
SHA256SUM-C-WALL-S: 0.6
RESTORE-CHECKSUM-STEP-S: 0.8
```

#### amd64-t32-evidence-commit.txt

```text
0ce6d68
```

#### amd64-t32-pass1.status

```text
0
```
