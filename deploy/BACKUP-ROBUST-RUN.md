# A backup that repeats a vanished file and removes a failed night, measured: results

This document is the permanent record of phase 29. It records what the changed `deploy/backup.sh` does: the repeat of a media copy that ended with `rsync` status 24, the removal of the directory of a night that failed, and, as in `deploy/BACKUP-CONSISTENCY-RUN.md`, the consistency of the pair of database and media, the restore of a backup made by 0.1.0 and the nearly full disk. This version of the record is the first draft, made to prove the pipeline of the appendix; every sentence in it is true, and it is not yet complete.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| CON-01 | PASS: `amd64-t29-src/summary.txt` carries the line `CON-01 PASS`. | linux/amd64 container on an aarch64 host (`amd64-t29-src/versions.txt`) | 2026-10-27, dd077e4 |
| CON-02 | PASS: `amd64-t29-src/summary.txt` carries the line `CON-02 PASS`. | same | 2026-10-27, dd077e4 |
| CON-03 | PASS: `amd64-t29-src/summary.txt` carries the line `CON-03 PASS`. | same | 2026-10-27, dd077e4 |
| ROB-01 | The line `VANISH: ONCE=PASS TWICE=PASS ONCE-UNIT=PASS TWICE-UNIT=PASS UNIT-TWICE-RESULT=exit-code SHIM-REMOVED=yes` is in `amd64-t29-src/summary.txt`. | same | 2026-10-27, dd077e4 |
| ROB-02 | The line beginning `CAPFULL:` of `amd64-t29-src/summary.txt` carries `HAND-CLASS=FAILED-NO-DIR`. | same | 2026-10-27, dd077e4 |

Evidence commit: dd077e4

## How this was run

The set was made once, from fresh containers, against the committed text of `dd077e4`.

## Deviations named

The kinds of deviation of the set are listed in the line `DEVIATIONS:` of `amd64-t29-src/summary.txt`.

## Findings

No finding is written in this draft.

## CON-01: a backup taken while pages are saved and media is uploaded

The result of the stage is the line `CON-01 PASS` of `amd64-t29-src/summary.txt`.

## CON-02: a backup made by 0.1.0 restores with this release

The result of the stage is the line `CON-02 PASS` of `amd64-t29-src/summary.txt`.

## CON-03: a disk nearly full but above `MIN_FREE_MB`

The result of the stage is the line `CON-03 PASS` of `amd64-t29-src/summary.txt`.

## ROB-01: a vanished file is repeated once

The result of the four vanish scenarios is the line beginning `VANISH:` of `amd64-t29-src/summary.txt`.

## ROB-02: a failed night removes its own directory

The result of the nearly full disk is the line beginning `CAPFULL:` of `amd64-t29-src/summary.txt`.

## Measurements

The stage durations are in `deploy/test/out/amd64-t29-pass1.log`, which is not pasted.

## Container vs. server

The set ran in a linux/amd64 container on an aarch64 host (`amd64-t29-src/versions.txt`).

## Not tested

- A real server was not part of this set.

## Owner steps

1. Run `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups; echo "exit $?"` on the server and expect the last two lines to be `=== Backup complete: ... ===` and a line `exit 0`; any other last line means the night did not finish.

## Hand-over to docs/beta.md

Nothing in this record is a promise until `docs/beta.md` cites it byte for byte.

## Appendix: raw output

Each block is the scratch file of the evidence set, pasted unchanged. `bash deploy/test/nightly.sh record-check-con` compares every block with its scratch file byte for byte. The files are in `deploy/test/out` (gitignored); `step-NN.txt`, `seed.txt`, the logs, the churn logs, the cookie jars and the key files are not pasted. Of the 80 runs of `consist` the appendix holds the three per-run files (backup, restore, compare) of run 1 of stage adds and of run 5 of stage deletes (the first night of the set that was repeated after a status 24), and the compare file of run 2 of stage adds; every other run is in `amd64-t29-src/consist-runs.txt`, one line each, and in the `WINDOW-NAME` lines of `amd64-t29-src/consist.txt`. The per-run files of the other runs exist in `deploy/test/out` and are not pasted: this is the one place where the appendix holds less than the set, and a statement about those runs rests on the one-line records.

### R1 source: summary and stages

#### amd64-t29-src/summary.txt

```text
---- summary (linux/amd64) ----
CON-01 PASS
CON-02 PASS
CON-03 PASS
CONSIST: stage adds RUNS=40 RESTORED=40 AGREE=16 ORPHAN-ONLY=24 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=36 UPLOADS-IN-WINDOW=65 OUTCOME=ORPHAN-ONLY-NAMED
CONSIST: stage adds window writes: uploads 65, page saves 24, deletes 0; backup status histogram 0=40
CONSIST: stage adds retries: repeated 0, completed after the repeat 0, ended with status 24 0, directories without a MANIFEST left 0
CONSIST: stage deletes RUNS=40 RESTORED=40 AGREE=10 ORPHAN-ONLY=14 ROW-WITHOUT-FILE=16 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=38 UPLOADS-IN-WINDOW=91 OUTCOME=ROW-WITHOUT-FILE-NAMED
CONSIST: stage deletes window writes: uploads 91, page saves 29, deletes 27; backup status histogram 0=40
CONSIST: stage deletes retries: repeated 8, completed after the repeat 8, ended with status 24 0, directories without a MANIFEST left 0
CONSIST: MIN-RUNS 30, restored-run floor 15 per stage (at least half of MIN-RUNS and never fewer than 10, never more than the stage has runs)
WINDOW: C0 (nothing written) AGREE, C1 (upload in the window) ORPHAN-ONLY, C2 (delete in the window) ROW-WITHOUT-FILE
COMPAT: TAG=v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712) LEVEL-BEFORE=60 LEVEL-AFTER=60 LOGIN-ADMIN=HTTP 303 /admin/ LOGIN-EDITOR=HTTP 303 /admin/ PAGES-SOURCE=8 PAGES-RESTORED=8 MEDIA-SERVED=4 of 4 CHECK=17 of 17
CAPFULL: FLOOR-MB=1024 FREE-AT-GUARD-MB=1064 DATA-MB=1225 FS-SIZE-MB=1064 CONTROL-CLASS=COMPLETE HAND-CLASS=FAILED-NO-DIR AGAIN-CLASS=FAILED-NO-DIR UNIT-CLASS=FAILED-NO-DIR UNIT-RESULT=exit-code UNIT-EXECMAINSTATUS=11 RESTORE-PARTIAL-EXIT=not-run (the run by hand left no directory) HAND-ROBUST=PASS AGAIN-NEXT-NIGHT=PASS UNIT-ROBUST=PASS
VANISH: ONCE=PASS TWICE=PASS ONCE-UNIT=PASS TWICE-UNIT=PASS UNIT-TWICE-RESULT=exit-code SHIM-REMOVED=yes
FUT-02: not needed (adds ORPHAN-ONLY-NAMED, deletes ROW-WITHOUT-FILE-NAMED: every disagreement that occurred was reproduced and its window named)
CLASSIFY: consist stage adds: 24 run(s) with CLASS=ORPHAN-ONLY (runs 2 4 6 7 8 9 10 12 13 14 15 16 18 21 22 24 25 29 30 32 36 37 38 39)
CLASSIFY: consist stage deletes: 14 run(s) with CLASS=ORPHAN-ONLY (runs 1 3 8 9 10 12 13 17 20 21 28 29 30 35)
CLASSIFY: consist stage deletes: 16 run(s) with CLASS=ROW-WITHOUT-FILE (runs 2 5 6 11 14 16 19 22 23 24 25 27 33 34 39 40)
CLASSIFY: capfull HAND-CLASS is FAILED-NO-DIR (see capfull.txt)
CLASSIFY: capfull AGAIN-CLASS is FAILED-NO-DIR (see capfull.txt)
CLASSIFY: capfull UNIT-CLASS is FAILED-NO-DIR (see capfull.txt)
STEPS: 96 played, 0 failed
DEVIATIONS: churn dropin editor-session emulation fixture fs hold mount scp start-limit transfer vanish-shim
LEAK-CHECK OK
```

#### amd64-t29-src/chain-statuses.txt

```text
prep=0 consist=0 window=0 capfull=0 compat-source=0 compat=0
```

#### amd64-t29-src/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           6.12.76-linuxkit
container uname:       Linux 6.12.76-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-t29-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.0.10 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-dd077e4 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[editor-session]: the load is an EDITOR's session, not the administrator's. auth.MustHaveSecondFactor(role, viaSSO) is true for an admin, so every admin URL answers an admin session with a redirect to the compulsory two-factor setup (ADMIN-SESSION above) and no admin session reaches a page form. The account churn@example.test, role editor, was created for it with the new binary and the baseline login password (no new secret exists)
DEVIATION[churn]: the load is curl run inside the container against 127.0.0.1:8080 with the CSRF token of each form, not browsers; the pace is set by CHURN_PAUSE and CHURN_WORKERS, not by people
DEVIATION[start-limit]: the shipped holzcloud.service allows 5 starts within 300 s and restore.sh stops and starts it, so a sixth restore within five minutes on one host ends in 'start-limit-hit' (measured, amd64-t25-trace-startlimit). Runs follow each other faster than that, so 'systemctl reset-failed holzcloud' is run on the second container before every restore; the unit is not changed and no sandbox directive is touched
DEVIATION[hold]: the pause is the container's addition. /opt/holzcloud/hc-hold-bin is a root-owned 755 stand-in for the binary: the real binary with the same arguments and status, and after a successful 'backup' it creates /tmp/hc-hold.flag and waits (at most 90 s) until the driver removes it. backup.sh is untouched and is given the stand-in through the HOLZCLOUD_BIN variable it documents. The window it opens, between the snapshot and the copy of the media, is the one in backup.sh's own comment; only its length is the container's
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[fs]: the nearly full filesystem is a tmpfs, not a disk. It has no blocks reserved for root, no fragmentation and no journal, it lives in memory, and the data directory is on another filesystem than the backup root, whereas on a real server the two usually share one disk and a full disk stops the site's own writes as well. The error delivered to rsync is the same errno (ENOSPC, No space left on device)
DEVIATION[emulation]: the container is linux/amd64 emulated on an arm64 host; copy times say nothing about a server
DEVIATION[vanish-shim]: a file cannot be made to vanish at the instant rsync reads the tree, so the status 24 is delivered by a stand-in at /usr/local/bin/rsync (ahead of /usr/bin on the PATH of sudo and of systemd). It runs the real rsync first and then replaces its status by 24 for the first FAILS calls that copy the media directory. What is played is the script's handling of the status 24 (the repeat, the recurrence, the exit status, the removal of the directory) and the unit's, not rsync's detection of a vanished file. The stand-in exists only in this throwaway container and is removed by the harness on every exit path
```

#### amd64-t29-tgt/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-dd077e4 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
```

#### amd64-t29-old/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.1.0 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[editor-session]: the load is an EDITOR's session, not the administrator's. auth.MustHaveSecondFactor(role, viaSSO) is true for an admin, so every admin URL answers an admin session with a redirect to the compulsory two-factor setup (ADMIN-SESSION above) and no admin session reaches a page form. The account churn@example.test, role editor, was created for it with the new binary and the baseline login password (no new secret exists)
DEVIATION[churn]: the load is curl run inside the container against 127.0.0.1:8080 with the CSRF token of each form, not browsers; the pace is set by CHURN_PAUSE and CHURN_WORKERS, not by people
DEVIATION[start-limit]: the shipped holzcloud.service allows 5 starts within 300 s; the fresh box was started by its install and is started again by the restore and by run.sh check, so 'systemctl reset-failed holzcloud' is run on it before each. The unit is not changed and no sandbox directive is touched
```

#### amd64-t29-new/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-dd077e4 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[transfer]: the backup travels as a tar stream through the host (docker cp SOURCE:dir - into docker cp - TARGET:/opt/holzcloud/backups); on two real machines it would be rsync or scp, which the document leaves to the operator. Proof of a lossless transfer is the sha256 of every file on both sides
DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they cross from container to container by exec-pipe, never through a host file
```

#### amd64-t29-src/prep.txt

```text
WEBSITE: 1
PASS the website called Test has id 1
SETUP-PATH: /admin/2fa/einrichten (internal/auth/twofactor.go, SetupPath)
ADMIN-LOGIN: HTTP 303 /admin/
ADMIN-SESSION: HTTP 303 /admin/2fa/einrichten
PASS the admin session on the page form is answered with a redirect to /admin/2fa/einrichten: auth.MustHaveSecondFactor makes the two-factor setup compulsory for an admin
EDITOR-CREATE: created user 2 (churn@example.test, editor)
PASS churn@example.test is an account with the role editor
EDITOR-LOGIN: HTTP 303 /admin/
PASS the editor logs in: HTTP 303 /admin/
PAGE-SAVE: HTTP 303 /admin/websites/1/pages
UPLOAD-1: HTTP 303 /admin/websites/1/media
UPLOAD-2: HTTP 303 /admin/websites/1/media
PASS the page row churn-prep-1793087668 exists (a draft: what an editor can make)
UPLOAD-ROWS: 2
PASS two uploads that differ in their trailing bytes gave two media rows (the strip step keeps the difference)
PASS the file of the first upload is below data/media: 1/30478e8ddea69834b2d0508ed775a25b.jpg
PASS the page references an uploaded media file: media_usage has its row
PREP OK
```

### R2 CON-01 consist

#### amd64-t29-src/consist.txt

```text
MIN-RUNS: 30
CONSIST-RUNS: 40
CONSIST-STAGES: adds deletes
CONSIST-RESTORE-EVERY: 1
RESTORED-RUNS-FLOOR: 15 per stage (at least half of MIN-RUNS and never fewer than 10, never more than the stage has runs)
CHURN-WORKERS: 4 uploaders, one page saver, in stage deletes one deleter
CHURN-PAUSE-S: 0.05
BACKUP-LINE: deploy/DEPLOY.md:657: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
RESTORE-LINE: deploy/DEPLOY.md:971: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500 (the sample timestamp is replaced by the directory of the run; yes is typed on stdin)
WEBSITE: 1
BASE-MEDIA: /media/1/53edf2234b0d700713f475d07008347f.jpg
BINARY: holzcloud v0.1.0-123-gdd077e4 (dd077e4)
BULK: BULK-MB: 100 FILES: 20 BYTES: 104861696 MEDIA-BYTES: 104909871 
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
START-LIMIT: StartLimitIntervalUSec=5min StartLimitBurst=5 (shipped unit, read from holzcloud-t29-tgt)
INFO COMPARATOR-SELFTEST genuine lists: 9 9 0 0 AGREE
PASS comparator selftest: a row removed is detected as a file without a row (8 9 0 1 ORPHAN-ONLY)
PASS comparator selftest: a file added is detected as a file without a row (9 10 0 1 ORPHAN-ONLY)
PASS comparator selftest: a row added is detected as a row without a file (10 9 1 0 ROW-WITHOUT-FILE)
PASS comparator selftest: a file removed is detected as a row without a file (9 8 1 0 ROW-WITHOUT-FILE)
RUN-1-SECONDS: 26
RUN-1-RESTORE-SECONDS: 13
PROJECTED-MINUTES: 34 at CONSIST_RESTORE_EVERY=1 (2 stage(s) of 40 runs)
WALL-MINUTES: 32
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w4-3.jpg 1793087703666 1793087704230) file 1/edc1250662121ff1fb219c651b66c003-thumb.jpg
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w4-3.jpg 1793087703666 1793087704230) file 1/edc1250662121ff1fb219c651b66c003.jpg
WINDOW-NAME run adds 4: overlaps-snapshot-step (upload churn-adds4-w4-3.jpg 1793087756065 1793087756589) file 1/058c84a071c929ee1455acc87d012df5-medium.jpg
WINDOW-NAME run adds 4: overlaps-snapshot-step (upload churn-adds4-w4-3.jpg 1793087756065 1793087756589) file 1/058c84a071c929ee1455acc87d012df5-thumb.jpg
WINDOW-NAME run adds 4: overlaps-snapshot-step (upload churn-adds4-w4-3.jpg 1793087756065 1793087756589) file 1/058c84a071c929ee1455acc87d012df5.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w3-4.jpg 1793087807500 1793087808046) file 1/651e193d33e3de15e7d22394f4da4f3b-medium.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w3-4.jpg 1793087807500 1793087808046) file 1/651e193d33e3de15e7d22394f4da4f3b-thumb.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w3-4.jpg 1793087807500 1793087808046) file 1/651e193d33e3de15e7d22394f4da4f3b.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w4-4.jpg 1793087807473 1793087808000) file 1/c3f165345d0f1ea34d6bef8ec68d7f80-medium.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w4-4.jpg 1793087807473 1793087808000) file 1/c3f165345d0f1ea34d6bef8ec68d7f80-thumb.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w4-4.jpg 1793087807473 1793087808000) file 1/c3f165345d0f1ea34d6bef8ec68d7f80.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w1-4.jpg 1793087807397 1793087807950) file 1/ec30dff53a98229d92e2e1cca295a703-medium.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w1-4.jpg 1793087807397 1793087807950) file 1/ec30dff53a98229d92e2e1cca295a703-thumb.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w1-4.jpg 1793087807397 1793087807950) file 1/ec30dff53a98229d92e2e1cca295a703.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w4-3.jpg 1793087833001 1793087833702) file 1/15c4f598131bc5cb37d5cfc4f72270dc-thumb.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w4-3.jpg 1793087833001 1793087833702) file 1/15c4f598131bc5cb37d5cfc4f72270dc.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w3-3.jpg 1793087832994 1793087833597) file 1/772b6961ea81a2c2f2c52d5c2e5fd4c7-medium.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w3-3.jpg 1793087832994 1793087833597) file 1/772b6961ea81a2c2f2c52d5c2e5fd4c7-thumb.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w3-3.jpg 1793087832994 1793087833597) file 1/772b6961ea81a2c2f2c52d5c2e5fd4c7.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w2-3.jpg 1793087832983 1793087833618) file 1/ba173278e869e6b1a1914a315e0d283f-medium.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w2-3.jpg 1793087832983 1793087833618) file 1/ba173278e869e6b1a1914a315e0d283f-thumb.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w2-3.jpg 1793087832983 1793087833618) file 1/ba173278e869e6b1a1914a315e0d283f.jpg
WINDOW-NAME run adds 8: overlaps-snapshot-step (upload churn-adds8-w4-4.jpg 1793087858099 1793087858568) file 1/24cabc311b56830ca7fdbc608b7492a8.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w2-4.jpg 1793087885389 1793087885934) file 1/9f5a94d25ab0dbb7c66ba2f93af3fd3d-medium.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w2-4.jpg 1793087885389 1793087885934) file 1/9f5a94d25ab0dbb7c66ba2f93af3fd3d-thumb.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w2-4.jpg 1793087885389 1793087885934) file 1/9f5a94d25ab0dbb7c66ba2f93af3fd3d.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w3-3.jpg 1793087909868 1793087910445) file 1/9cca9bef983e9ad08463b19336861440-medium.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w3-3.jpg 1793087909868 1793087910445) file 1/9cca9bef983e9ad08463b19336861440-thumb.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w3-3.jpg 1793087909868 1793087910445) file 1/9cca9bef983e9ad08463b19336861440.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w2-3.jpg 1793087910023 1793087910647) file 1/ff5381a507a196fe759b18332ace74c7-thumb.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w2-3.jpg 1793087910023 1793087910647) file 1/ff5381a507a196fe759b18332ace74c7.jpg
WINDOW-NAME run adds 12: overlaps-snapshot-step (upload churn-adds12-w2-3.jpg 1793087961276 1793087961746) file 1/51c0446af40b4752dcabe09684ace8bb.jpg
WINDOW-NAME run adds 13: overlaps-snapshot-step (upload churn-adds13-w3-3.jpg 1793087985190 1793087985647) file 1/eeadf2b2b8c6d2de02855b84ab3916e7-thumb.jpg
WINDOW-NAME run adds 13: overlaps-snapshot-step (upload churn-adds13-w3-3.jpg 1793087985190 1793087985647) file 1/eeadf2b2b8c6d2de02855b84ab3916e7.jpg
WINDOW-NAME run adds 14: overlaps-snapshot-step (upload churn-adds14-w3-4.jpg 1793088010449 1793088010887) file 1/f55ef89492c55a49fe7f095a7ae0c445-thumb.jpg
WINDOW-NAME run adds 14: overlaps-snapshot-step (upload churn-adds14-w3-4.jpg 1793088010449 1793088010887) file 1/f55ef89492c55a49fe7f095a7ae0c445.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w2-3.jpg 1793088036509 1793088037490) file 1/da85e93beb0a36dbbbd964e837302256-medium.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w2-3.jpg 1793088036509 1793088037490) file 1/da85e93beb0a36dbbbd964e837302256-thumb.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w2-3.jpg 1793088036509 1793088037490) file 1/da85e93beb0a36dbbbd964e837302256.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w4-3.jpg 1793088036535 1793088037477) file 1/e411a140763c0930c625fec9cbbaf844-medium.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w4-3.jpg 1793088036535 1793088037477) file 1/e411a140763c0930c625fec9cbbaf844-thumb.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w4-3.jpg 1793088036535 1793088037477) file 1/e411a140763c0930c625fec9cbbaf844.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w1-3.jpg 1793088063858 1793088064502) file 1/20270e0c1ea23ad134e4f14ad8fe1a34-medium.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w1-3.jpg 1793088063858 1793088064502) file 1/20270e0c1ea23ad134e4f14ad8fe1a34-thumb.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w1-3.jpg 1793088063858 1793088064502) file 1/20270e0c1ea23ad134e4f14ad8fe1a34.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w3-3.jpg 1793088063840 1793088064466) file 1/3e4367b236c0a9170ce9310c89b104be-medium.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w3-3.jpg 1793088063840 1793088064466) file 1/3e4367b236c0a9170ce9310c89b104be-thumb.jpg
WINDOW-NAME run adds 16: overlaps-snapshot-step (upload churn-adds16-w3-3.jpg 1793088063840 1793088064466) file 1/3e4367b236c0a9170ce9310c89b104be.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-2.jpg 1793088110829 1793088111225) file 1/33e04ed4b4dad5cc28ff3f0484f160fc-medium.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-2.jpg 1793088110829 1793088111225) file 1/33e04ed4b4dad5cc28ff3f0484f160fc-thumb.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-2.jpg 1793088110829 1793088111225) file 1/33e04ed4b4dad5cc28ff3f0484f160fc.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w1-3.jpg 1793088181266 1793088181641) file 1/6ddda01d7caef56d1e9c1acc76ac4ec6-medium.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w1-3.jpg 1793088181266 1793088181641) file 1/6ddda01d7caef56d1e9c1acc76ac4ec6-thumb.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w1-3.jpg 1793088181266 1793088181641) file 1/6ddda01d7caef56d1e9c1acc76ac4ec6.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w2-4.jpg 1793088205012 1793088205388) file 1/4fe1b0432212ead4d0f2a97fc9a96a24-medium.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w2-4.jpg 1793088205012 1793088205388) file 1/4fe1b0432212ead4d0f2a97fc9a96a24-thumb.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w2-4.jpg 1793088205012 1793088205388) file 1/4fe1b0432212ead4d0f2a97fc9a96a24.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w1-4.jpg 1793088253260 1793088254065) file 1/fff94ed0501e2734aaf184e5f9b56399-thumb.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w1-4.jpg 1793088253260 1793088254065) file 1/fff94ed0501e2734aaf184e5f9b56399.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-4.jpg 1793088279547 1793088279966) file 1/0501a690b1c9385c5bc2450a5fe3591c-medium.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-4.jpg 1793088279547 1793088279966) file 1/0501a690b1c9385c5bc2450a5fe3591c-thumb.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-4.jpg 1793088279547 1793088279966) file 1/0501a690b1c9385c5bc2450a5fe3591c.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w2-3.jpg 1793088373924 1793088374293) file 1/7d2d8ac6cd8dfba192f49a26f312cc44-medium.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w2-3.jpg 1793088373924 1793088374293) file 1/7d2d8ac6cd8dfba192f49a26f312cc44-thumb.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w2-3.jpg 1793088373924 1793088374293) file 1/7d2d8ac6cd8dfba192f49a26f312cc44.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w3-3.jpg 1793088373976 1793088374334) file 1/d9e045f35cabad755e82643b356d7922-medium.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w3-3.jpg 1793088373976 1793088374334) file 1/d9e045f35cabad755e82643b356d7922-thumb.jpg
WINDOW-NAME run adds 29: overlaps-snapshot-step (upload churn-adds29-w3-3.jpg 1793088373976 1793088374334) file 1/d9e045f35cabad755e82643b356d7922.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w1-3.jpg 1793088397139 1793088397501) file 1/e0f5977f36d2cc0ca2e34a9883204844-medium.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w1-3.jpg 1793088397139 1793088397501) file 1/e0f5977f36d2cc0ca2e34a9883204844-thumb.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w1-3.jpg 1793088397139 1793088397501) file 1/e0f5977f36d2cc0ca2e34a9883204844.jpg
WINDOW-NAME run adds 32: overlaps-snapshot-step (upload churn-adds32-w1-4.jpg 1793088444658 1793088445029) file 1/54c280443d598f679c2e883515eadee1.jpg
WINDOW-NAME run adds 32: overlaps-snapshot-step (upload churn-adds32-w2-4.jpg 1793088444608 1793088444957) file 1/87a546b5a95e0a6ab9763f7979f9cf2e-thumb.jpg
WINDOW-NAME run adds 32: overlaps-snapshot-step (upload churn-adds32-w2-4.jpg 1793088444608 1793088444957) file 1/87a546b5a95e0a6ab9763f7979f9cf2e.jpg
WINDOW-NAME run adds 36: overlaps-snapshot-step (upload churn-adds36-w3-4.jpg 1793088537159 1793088537528) file 1/50ea9072ebb7e1a887b28c83f4422143.jpg
WINDOW-NAME run adds 37: overlaps-snapshot-step (upload churn-adds37-w3-4.jpg 1793088560918 1793088561265) file 1/767c773e8b499c013db3dbcfca9b583a-thumb.jpg
WINDOW-NAME run adds 37: overlaps-snapshot-step (upload churn-adds37-w3-4.jpg 1793088560918 1793088561265) file 1/767c773e8b499c013db3dbcfca9b583a.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w1-3.jpg 1793088584239 1793088584594) file 1/b6acfc3648587803890f777a711236b2-medium.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w1-3.jpg 1793088584239 1793088584594) file 1/b6acfc3648587803890f777a711236b2-thumb.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w1-3.jpg 1793088584239 1793088584594) file 1/b6acfc3648587803890f777a711236b2.jpg
WINDOW-NAME run adds 39: overlaps-snapshot-step (upload churn-adds39-w3-4.jpg 1793088608373 1793088608728) file 1/1e965d71c2cd68e79ff5d12c817fffa6.jpg
WINDOW-NAME run adds 39: overlaps-snapshot-step (upload churn-adds39-w4-4.jpg 1793088608377 1793088608735) file 1/2221dffbd8a88753a53a9ddd775def99.jpg
WINDOW-NAME run deletes 1: overlaps-snapshot-step (upload churn-deletes1-w1-3.jpg 1793088655367 1793088655749) file 1/e757ed47884c22976e9b2c63a3033ad7.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w3-4.jpg 1793088679529 1793088679961) file 1/5679e08e9d8101e3f0d0ec38116b1607.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (delete 9:1/687008f22392230e4d51660fe21f583a 1793088679487 1793088679773) file 1/687008f22392230e4d51660fe21f583a-medium.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (delete 9:1/687008f22392230e4d51660fe21f583a 1793088679487 1793088679773) file 1/687008f22392230e4d51660fe21f583a-thumb.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (delete 9:1/687008f22392230e4d51660fe21f583a 1793088679487 1793088679773) file 1/687008f22392230e4d51660fe21f583a.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w4-4.jpg 1793088679488 1793088679875) file 1/899137ff4539708e4e0871313e244c62-thumb.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w4-4.jpg 1793088679488 1793088679875) file 1/899137ff4539708e4e0871313e244c62.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w1-4.jpg 1793088679519 1793088679883) file 1/afc1fb5461dd30fe847203842b3948eb-thumb.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w1-4.jpg 1793088679519 1793088679883) file 1/afc1fb5461dd30fe847203842b3948eb.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (upload churn-deletes3-w1-4.jpg 1793088703902 1793088704271) file 1/165fa16d16fcc471d5f69bbd89c2de94-medium.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (upload churn-deletes3-w1-4.jpg 1793088703902 1793088704271) file 1/165fa16d16fcc471d5f69bbd89c2de94-thumb.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (upload churn-deletes3-w1-4.jpg 1793088703902 1793088704271) file 1/165fa16d16fcc471d5f69bbd89c2de94.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769.jpg
WINDOW-NAME run deletes 5: straddles-media-end (upload churn-deletes5-w1-5.jpg 1793088752644 1793088753053) file 1/9555e5461d1ce1d4b1599bc5657605bf.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d.jpg
WINDOW-NAME run deletes 6: overlaps-snapshot-step (delete 31:1/0f0888ef542473d76c16bf3c694f86e0 1793088777119 1793088777447) file 1/0f0888ef542473d76c16bf3c694f86e0-medium.jpg
WINDOW-NAME run deletes 6: overlaps-snapshot-step (delete 31:1/0f0888ef542473d76c16bf3c694f86e0 1793088777119 1793088777447) file 1/0f0888ef542473d76c16bf3c694f86e0-thumb.jpg
WINDOW-NAME run deletes 6: overlaps-snapshot-step (delete 31:1/0f0888ef542473d76c16bf3c694f86e0 1793088777119 1793088777447) file 1/0f0888ef542473d76c16bf3c694f86e0.jpg
WINDOW-NAME run deletes 6: overlaps-snapshot-step (upload churn-deletes6-w4-4.jpg 1793088777184 1793088777580) file 1/46077271c9ec33c047886c9f4eb11107.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (upload churn-deletes8-w1-4.jpg 1793088826214 1793088826585) file 1/1858a2fbadaa95935bdc5b1bce6d21cc-medium.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (upload churn-deletes8-w1-4.jpg 1793088826214 1793088826585) file 1/1858a2fbadaa95935bdc5b1bce6d21cc-thumb.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (upload churn-deletes8-w1-4.jpg 1793088826214 1793088826585) file 1/1858a2fbadaa95935bdc5b1bce6d21cc.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w3-3.jpg 1793088850389 1793088850759) file 1/5f4a0bad67167419bd0a5c258d3d4ffd-thumb.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w3-3.jpg 1793088850389 1793088850759) file 1/5f4a0bad67167419bd0a5c258d3d4ffd.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w1-3.jpg 1793088850363 1793088850751) file 1/7548c55597a14566fd2d6ab372433640-medium.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w1-3.jpg 1793088850363 1793088850751) file 1/7548c55597a14566fd2d6ab372433640-thumb.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w1-3.jpg 1793088850363 1793088850751) file 1/7548c55597a14566fd2d6ab372433640.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w4-3.jpg 1793088850312 1793088850693) file 1/fe80e545c4c5bd28cb59ae353e3684db-medium.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w4-3.jpg 1793088850312 1793088850693) file 1/fe80e545c4c5bd28cb59ae353e3684db-thumb.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (upload churn-deletes9-w4-3.jpg 1793088850312 1793088850693) file 1/fe80e545c4c5bd28cb59ae353e3684db.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w2-3.jpg 1793088874465 1793088874863) file 1/55d9aaafa3c0d5c84f6df9de9f490397-medium.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w2-3.jpg 1793088874465 1793088874863) file 1/55d9aaafa3c0d5c84f6df9de9f490397-thumb.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w2-3.jpg 1793088874465 1793088874863) file 1/55d9aaafa3c0d5c84f6df9de9f490397.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w1-3.jpg 1793088874447 1793088874836) file 1/5f2a7e278862a77370e979bdbcab265e-medium.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w1-3.jpg 1793088874447 1793088874836) file 1/5f2a7e278862a77370e979bdbcab265e-thumb.jpg
WINDOW-NAME run deletes 10: overlaps-snapshot-step (upload churn-deletes10-w1-3.jpg 1793088874447 1793088874836) file 1/5f2a7e278862a77370e979bdbcab265e.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w4-3.jpg 1793088898692 1793088899164) file 1/1e2624b1f8c32a616f6c0613f58d0e71-medium.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w4-3.jpg 1793088898692 1793088899164) file 1/1e2624b1f8c32a616f6c0613f58d0e71-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w4-3.jpg 1793088898692 1793088899164) file 1/1e2624b1f8c32a616f6c0613f58d0e71.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w2-3.jpg 1793088898665 1793088899091) file 1/32e80cdc808ac607a884cb85c366fef8-medium.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w2-3.jpg 1793088898665 1793088899091) file 1/32e80cdc808ac607a884cb85c366fef8-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w2-3.jpg 1793088898665 1793088899091) file 1/32e80cdc808ac607a884cb85c366fef8.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w1-3.jpg 1793088898681 1793088899128) file 1/68f1298997d45ef3db5a5a28d8b4e655-medium.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w1-3.jpg 1793088898681 1793088899128) file 1/68f1298997d45ef3db5a5a28d8b4e655-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w1-3.jpg 1793088898681 1793088899128) file 1/68f1298997d45ef3db5a5a28d8b4e655.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 58:1/894f963c859c5434da6f51c13a5f1cd1 1793088898880 1793088899215) file 1/894f963c859c5434da6f51c13a5f1cd1-medium.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 58:1/894f963c859c5434da6f51c13a5f1cd1 1793088898880 1793088899215) file 1/894f963c859c5434da6f51c13a5f1cd1-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 58:1/894f963c859c5434da6f51c13a5f1cd1 1793088898880 1793088899215) file 1/894f963c859c5434da6f51c13a5f1cd1.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (upload churn-deletes12-w4-3.jpg 1793088923431 1793088923877) file 1/6d1369ded258b4caae5f4455d077ec87-thumb.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (upload churn-deletes12-w4-3.jpg 1793088923431 1793088923877) file 1/6d1369ded258b4caae5f4455d077ec87.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w3-3.jpg 1793088947824 1793088948233) file 1/7d0120e4b93ed8e6fdc720182679d143-thumb.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w3-3.jpg 1793088947824 1793088948233) file 1/7d0120e4b93ed8e6fdc720182679d143.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w4-3.jpg 1793088947779 1793088948205) file 1/96234208a12e9bb22bc9d0feaa37da3d-medium.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w4-3.jpg 1793088947779 1793088948205) file 1/96234208a12e9bb22bc9d0feaa37da3d-thumb.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w4-3.jpg 1793088947779 1793088948205) file 1/96234208a12e9bb22bc9d0feaa37da3d.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 74:1/f26f17a71fbc168c482f1eb0e664d818 1793088972626 1793088972959) file 1/f26f17a71fbc168c482f1eb0e664d818-medium.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 74:1/f26f17a71fbc168c482f1eb0e664d818 1793088972626 1793088972959) file 1/f26f17a71fbc168c482f1eb0e664d818-thumb.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 74:1/f26f17a71fbc168c482f1eb0e664d818 1793088972626 1793088972959) file 1/f26f17a71fbc168c482f1eb0e664d818.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (upload churn-deletes14-w1-4.jpg 1793088972666 1793088973049) file 1/f5a74ac487e7da1c4b46d32162021d9e.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w1-4.jpg 1793089021118 1793089021489) file 1/54d2911d60b27d2b5a006baa98a7fe63-medium.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w1-4.jpg 1793089021118 1793089021489) file 1/54d2911d60b27d2b5a006baa98a7fe63-thumb.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w1-4.jpg 1793089021118 1793089021489) file 1/54d2911d60b27d2b5a006baa98a7fe63.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w3-4.jpg 1793089021043 1793089021405) file 1/74c5ac373e27f6b85abf632c4a93c201-medium.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w3-4.jpg 1793089021043 1793089021405) file 1/74c5ac373e27f6b85abf632c4a93c201-thumb.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w3-4.jpg 1793089021043 1793089021405) file 1/74c5ac373e27f6b85abf632c4a93c201.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 83:1/85edc115e18cdfd999b1f97ce0d88c7c 1793089020729 1793089021023) file 1/85edc115e18cdfd999b1f97ce0d88c7c-medium.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 83:1/85edc115e18cdfd999b1f97ce0d88c7c 1793089020729 1793089021023) file 1/85edc115e18cdfd999b1f97ce0d88c7c-thumb.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 83:1/85edc115e18cdfd999b1f97ce0d88c7c 1793089020729 1793089021023) file 1/85edc115e18cdfd999b1f97ce0d88c7c.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w4-4.jpg 1793089020948 1793089021309) file 1/99adf8a3005683bb9104a0cde97ec3ae-medium.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w4-4.jpg 1793089020948 1793089021309) file 1/99adf8a3005683bb9104a0cde97ec3ae-thumb.jpg
WINDOW-NAME run deletes 16: after-snapshot (upload churn-deletes16-w4-4.jpg 1793089020948 1793089021309) file 1/99adf8a3005683bb9104a0cde97ec3ae.jpg
WINDOW-NAME run deletes 16: straddles-media-end (upload churn-deletes16-w2-4.jpg 1793089021204 1793089021571) file 1/c3a25a1b4c94bc6a800ff75dbf6508ec.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w2-3.jpg 1793089045535 1793089045908) file 1/08184efda2b64bf985f21f9d1ba6f402-thumb.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w2-3.jpg 1793089045535 1793089045908) file 1/08184efda2b64bf985f21f9d1ba6f402.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (delete 99:1/2b00326de2ae024e145211d4be4338a0 1793089095245 1793089095520) file 1/2b00326de2ae024e145211d4be4338a0-medium.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (delete 99:1/2b00326de2ae024e145211d4be4338a0 1793089095245 1793089095520) file 1/2b00326de2ae024e145211d4be4338a0-thumb.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (delete 99:1/2b00326de2ae024e145211d4be4338a0 1793089095245 1793089095520) file 1/2b00326de2ae024e145211d4be4338a0.jpg
WINDOW-NAME run deletes 19: after-snapshot (upload churn-deletes19-w4-4.jpg 1793089095589 1793089095943) file 1/5ccc021a7c979cafe3ccc04497a008c3-medium.jpg
WINDOW-NAME run deletes 19: after-snapshot (upload churn-deletes19-w4-4.jpg 1793089095589 1793089095943) file 1/5ccc021a7c979cafe3ccc04497a008c3-thumb.jpg
WINDOW-NAME run deletes 19: after-snapshot (upload churn-deletes19-w4-4.jpg 1793089095589 1793089095943) file 1/5ccc021a7c979cafe3ccc04497a008c3.jpg
WINDOW-NAME run deletes 19: straddles-media-end (upload churn-deletes19-w3-4.jpg 1793089095611 1793089096044) file 1/93ace2da58dea2571f7263b74781b36b.jpg
WINDOW-NAME run deletes 19: after-snapshot (upload churn-deletes19-w2-4.jpg 1793089095602 1793089095964) file 1/cf060ec1066e6429edcf103b7021a903-thumb.jpg
WINDOW-NAME run deletes 19: after-snapshot (upload churn-deletes19-w2-4.jpg 1793089095602 1793089095964) file 1/cf060ec1066e6429edcf103b7021a903.jpg
WINDOW-NAME run deletes 20: overlaps-snapshot-step (upload churn-deletes20-w1-4.jpg 1793089120348 1793089120701) file 1/7ea8dee92069c65b3befb4f420d60971-thumb.jpg
WINDOW-NAME run deletes 20: overlaps-snapshot-step (upload churn-deletes20-w1-4.jpg 1793089120348 1793089120701) file 1/7ea8dee92069c65b3befb4f420d60971.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (upload churn-deletes21-w1-4.jpg 1793089145374 1793089145753) file 1/9b0f6ad8faefd7fe0c2792a8731979be-medium.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (upload churn-deletes21-w1-4.jpg 1793089145374 1793089145753) file 1/9b0f6ad8faefd7fe0c2792a8731979be-thumb.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (upload churn-deletes21-w1-4.jpg 1793089145374 1793089145753) file 1/9b0f6ad8faefd7fe0c2792a8731979be.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 116:1/c11b93bb281a522c0010063485a34733 1793089170702 1793089170982) file 1/c11b93bb281a522c0010063485a34733-medium.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 116:1/c11b93bb281a522c0010063485a34733 1793089170702 1793089170982) file 1/c11b93bb281a522c0010063485a34733-thumb.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 116:1/c11b93bb281a522c0010063485a34733 1793089170702 1793089170982) file 1/c11b93bb281a522c0010063485a34733.jpg
WINDOW-NAME run deletes 23: after-snapshot (upload churn-deletes23-w4-4.jpg 1793089195236 1793089195592) file 1/12a0cbaf14b60fa891c07cc0ddfe5799-thumb.jpg
WINDOW-NAME run deletes 23: after-snapshot (upload churn-deletes23-w4-4.jpg 1793089195236 1793089195592) file 1/12a0cbaf14b60fa891c07cc0ddfe5799.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (delete 122:1/1f4a55630030d2bddc3aa6c6a972ff54 1793089194851 1793089195141) file 1/1f4a55630030d2bddc3aa6c6a972ff54-medium.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (delete 122:1/1f4a55630030d2bddc3aa6c6a972ff54 1793089194851 1793089195141) file 1/1f4a55630030d2bddc3aa6c6a972ff54-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (delete 122:1/1f4a55630030d2bddc3aa6c6a972ff54 1793089194851 1793089195141) file 1/1f4a55630030d2bddc3aa6c6a972ff54.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w3-4.jpg 1793089194899 1793089195279) file 1/4f239f87c02577819f83535c7110c92b-medium.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w3-4.jpg 1793089194899 1793089195279) file 1/4f239f87c02577819f83535c7110c92b-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w3-4.jpg 1793089194899 1793089195279) file 1/4f239f87c02577819f83535c7110c92b.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w2-4.jpg 1793089194948 1793089195323) file 1/8b7ad4b7ac63b85ab13a593c8bef8d57-medium.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w2-4.jpg 1793089194948 1793089195323) file 1/8b7ad4b7ac63b85ab13a593c8bef8d57-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w2-4.jpg 1793089194948 1793089195323) file 1/8b7ad4b7ac63b85ab13a593c8bef8d57.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w1-4.jpg 1793089194807 1793089195184) file 1/eb23ebea0fe71f773bd2dafd44780a3a-medium.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w1-4.jpg 1793089194807 1793089195184) file 1/eb23ebea0fe71f773bd2dafd44780a3a-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w1-4.jpg 1793089194807 1793089195184) file 1/eb23ebea0fe71f773bd2dafd44780a3a.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 128:1/6c801fcfa83454711f2ba98bffef9bde 1793089219331 1793089219628) file 1/6c801fcfa83454711f2ba98bffef9bde-medium.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 128:1/6c801fcfa83454711f2ba98bffef9bde 1793089219331 1793089219628) file 1/6c801fcfa83454711f2ba98bffef9bde-thumb.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 128:1/6c801fcfa83454711f2ba98bffef9bde 1793089219331 1793089219628) file 1/6c801fcfa83454711f2ba98bffef9bde.jpg
WINDOW-NAME run deletes 25: overlaps-snapshot-step (delete 134:1/aebe445b7eebcce9fd8d1385f603745c 1793089244380 1793089244654) file 1/aebe445b7eebcce9fd8d1385f603745c-medium.jpg
WINDOW-NAME run deletes 25: overlaps-snapshot-step (delete 134:1/aebe445b7eebcce9fd8d1385f603745c 1793089244380 1793089244654) file 1/aebe445b7eebcce9fd8d1385f603745c-thumb.jpg
WINDOW-NAME run deletes 25: overlaps-snapshot-step (delete 134:1/aebe445b7eebcce9fd8d1385f603745c 1793089244380 1793089244654) file 1/aebe445b7eebcce9fd8d1385f603745c.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w4-4.jpg 1793089294946 1793089295380) file 1/291fb35e43cfeb00416f0aac0dd0cc40-medium.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w4-4.jpg 1793089294946 1793089295380) file 1/291fb35e43cfeb00416f0aac0dd0cc40-thumb.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w4-4.jpg 1793089294946 1793089295380) file 1/291fb35e43cfeb00416f0aac0dd0cc40.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w2-4.jpg 1793089294956 1793089295397) file 1/c1bf1c6cc2f2cb9d29cc7cd5db8548f5-medium.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w2-4.jpg 1793089294956 1793089295397) file 1/c1bf1c6cc2f2cb9d29cc7cd5db8548f5-thumb.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (upload churn-deletes27-w2-4.jpg 1793089294956 1793089295397) file 1/c1bf1c6cc2f2cb9d29cc7cd5db8548f5.jpg
WINDOW-NAME run deletes 27: after-snapshot (upload churn-deletes27-w3-4.jpg 1793089295324 1793089295705) file 1/eb9262bbb43c2e21e9c0382efd5a1d28-medium.jpg
WINDOW-NAME run deletes 27: after-snapshot (upload churn-deletes27-w3-4.jpg 1793089295324 1793089295705) file 1/eb9262bbb43c2e21e9c0382efd5a1d28-thumb.jpg
WINDOW-NAME run deletes 27: after-snapshot (upload churn-deletes27-w3-4.jpg 1793089295324 1793089295705) file 1/eb9262bbb43c2e21e9c0382efd5a1d28.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 145:1/f03c0ad8894475d60b6f3fdb3f0cec47 1793089294970 1793089295294) file 1/f03c0ad8894475d60b6f3fdb3f0cec47-medium.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 145:1/f03c0ad8894475d60b6f3fdb3f0cec47 1793089294970 1793089295294) file 1/f03c0ad8894475d60b6f3fdb3f0cec47-thumb.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 145:1/f03c0ad8894475d60b6f3fdb3f0cec47 1793089294970 1793089295294) file 1/f03c0ad8894475d60b6f3fdb3f0cec47.jpg
WINDOW-NAME run deletes 28: overlaps-snapshot-step (upload churn-deletes28-w1-3.jpg 1793089320227 1793089320678) file 1/89e3dc072d2fcab9e37186f2578dc43b-thumb.jpg
WINDOW-NAME run deletes 28: overlaps-snapshot-step (upload churn-deletes28-w1-3.jpg 1793089320227 1793089320678) file 1/89e3dc072d2fcab9e37186f2578dc43b.jpg
WINDOW-NAME run deletes 29: overlaps-snapshot-step (upload churn-deletes29-w2-3.jpg 1793089345535 1793089345999) file 1/527126c05fbb5d622b7eebd42aa66b17-thumb.jpg
WINDOW-NAME run deletes 29: overlaps-snapshot-step (upload churn-deletes29-w2-3.jpg 1793089345535 1793089345999) file 1/527126c05fbb5d622b7eebd42aa66b17.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w1-4.jpg 1793089371418 1793089371807) file 1/0b552d8df0bf88fa57dc27a3322d5ae7.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (upload churn-deletes33-w1-4.jpg 1793089447201 1793089447602) file 1/13a8e53181cd5e782a9dd204f06b9d1c-medium.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (upload churn-deletes33-w1-4.jpg 1793089447201 1793089447602) file 1/13a8e53181cd5e782a9dd204f06b9d1c-thumb.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (upload churn-deletes33-w1-4.jpg 1793089447201 1793089447602) file 1/13a8e53181cd5e782a9dd204f06b9d1c.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (delete 178:1/55ff3f0be9dbf2add92d8998b557a155 1793089447260 1793089447559) file 1/55ff3f0be9dbf2add92d8998b557a155-medium.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (delete 178:1/55ff3f0be9dbf2add92d8998b557a155 1793089447260 1793089447559) file 1/55ff3f0be9dbf2add92d8998b557a155-thumb.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (delete 178:1/55ff3f0be9dbf2add92d8998b557a155 1793089447260 1793089447559) file 1/55ff3f0be9dbf2add92d8998b557a155.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (upload churn-deletes33-w3-4.jpg 1793089447324 1793089447698) file 1/92b1c2b2dad6eab2d71353c07e06400e-thumb.jpg
WINDOW-NAME run deletes 33: overlaps-snapshot-step (upload churn-deletes33-w3-4.jpg 1793089447324 1793089447698) file 1/92b1c2b2dad6eab2d71353c07e06400e.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (upload churn-deletes34-w4-3.jpg 1793089471550 1793089471910) file 1/294e2ce880034329d4891c3507e91ce8-medium.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (upload churn-deletes34-w4-3.jpg 1793089471550 1793089471910) file 1/294e2ce880034329d4891c3507e91ce8-thumb.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (upload churn-deletes34-w4-3.jpg 1793089471550 1793089471910) file 1/294e2ce880034329d4891c3507e91ce8.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (delete 183:1/f857e10c617f9fb5730d2c4b2bd4a394 1793089471501 1793089471791) file 1/f857e10c617f9fb5730d2c4b2bd4a394-medium.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (delete 183:1/f857e10c617f9fb5730d2c4b2bd4a394 1793089471501 1793089471791) file 1/f857e10c617f9fb5730d2c4b2bd4a394-thumb.jpg
WINDOW-NAME run deletes 34: overlaps-snapshot-step (delete 183:1/f857e10c617f9fb5730d2c4b2bd4a394 1793089471501 1793089471791) file 1/f857e10c617f9fb5730d2c4b2bd4a394.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (upload churn-deletes35-w1-4.jpg 1793089497502 1793089497939) file 1/220ea116a71375d0a2f9d947696a66ca-thumb.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (upload churn-deletes35-w1-4.jpg 1793089497502 1793089497939) file 1/220ea116a71375d0a2f9d947696a66ca.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793089599668 1793089600059) file 1/04385844eb3ee45fbc6ef1d97b0c1c79-medium.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793089599668 1793089600059) file 1/04385844eb3ee45fbc6ef1d97b0c1c79-thumb.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793089599668 1793089600059) file 1/04385844eb3ee45fbc6ef1d97b0c1c79.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w3-4.jpg 1793089599800 1793089600177) file 1/44ef3565e034cb4e52f31b43a1754e99-medium.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w3-4.jpg 1793089599800 1793089600177) file 1/44ef3565e034cb4e52f31b43a1754e99-thumb.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w3-4.jpg 1793089599800 1793089600177) file 1/44ef3565e034cb4e52f31b43a1754e99.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w4-4.jpg 1793089600051 1793089600414) file 1/5b1ef0d27f06652f2913b34bb288243f-medium.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w4-4.jpg 1793089600051 1793089600414) file 1/5b1ef0d27f06652f2913b34bb288243f-thumb.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w4-4.jpg 1793089600051 1793089600414) file 1/5b1ef0d27f06652f2913b34bb288243f.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (delete 210:1/9cca9bef983e9ad08463b19336861440 1793089599649 1793089599954) file 1/9cca9bef983e9ad08463b19336861440-medium.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (delete 210:1/9cca9bef983e9ad08463b19336861440 1793089599649 1793089599954) file 1/9cca9bef983e9ad08463b19336861440-thumb.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (delete 210:1/9cca9bef983e9ad08463b19336861440 1793089599649 1793089599954) file 1/9cca9bef983e9ad08463b19336861440.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w1-4.jpg 1793089600056 1793089600428) file 1/b6c18bb25dedec008e9cb357e5a19fec-medium.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w1-4.jpg 1793089600056 1793089600428) file 1/b6c18bb25dedec008e9cb357e5a19fec-thumb.jpg
WINDOW-NAME run deletes 39: after-snapshot (upload churn-deletes39-w1-4.jpg 1793089600056 1793089600428) file 1/b6c18bb25dedec008e9cb357e5a19fec.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w1-4.jpg 1793089625647 1793089626023) file 1/1821abf40edcc812b6b6daa3c056b300-medium.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w1-4.jpg 1793089625647 1793089626023) file 1/1821abf40edcc812b6b6daa3c056b300-thumb.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w1-4.jpg 1793089625647 1793089626023) file 1/1821abf40edcc812b6b6daa3c056b300.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w3-4.jpg 1793089625871 1793089626244) file 1/4bfc80f5a313dbccb32d30484a504619-medium.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w3-4.jpg 1793089625871 1793089626244) file 1/4bfc80f5a313dbccb32d30484a504619-thumb.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w3-4.jpg 1793089625871 1793089626244) file 1/4bfc80f5a313dbccb32d30484a504619.jpg
WINDOW-NAME run deletes 40: after-snapshot (upload churn-deletes40-w2-4.jpg 1793089625985 1793089626370) file 1/8a131a21863b59796eb9a20722e92bda-medium.jpg
WINDOW-NAME run deletes 40: after-snapshot (upload churn-deletes40-w2-4.jpg 1793089625985 1793089626370) file 1/8a131a21863b59796eb9a20722e92bda-thumb.jpg
WINDOW-NAME run deletes 40: after-snapshot (upload churn-deletes40-w2-4.jpg 1793089625985 1793089626370) file 1/8a131a21863b59796eb9a20722e92bda.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w4-4.jpg 1793089625810 1793089626192) file 1/8e151848d071efc2500f4d626e0517c6-medium.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w4-4.jpg 1793089625810 1793089626192) file 1/8e151848d071efc2500f4d626e0517c6-thumb.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (upload churn-deletes40-w4-4.jpg 1793089625810 1793089626192) file 1/8e151848d071efc2500f4d626e0517c6.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 216:1/b318ceb9120462aa0b5fa2aa0ebe2b27 1793089625814 1793089626114) file 1/b318ceb9120462aa0b5fa2aa0ebe2b27-medium.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 216:1/b318ceb9120462aa0b5fa2aa0ebe2b27 1793089625814 1793089626114) file 1/b318ceb9120462aa0b5fa2aa0ebe2b27-thumb.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 216:1/b318ceb9120462aa0b5fa2aa0ebe2b27 1793089625814 1793089626114) file 1/b318ceb9120462aa0b5fa2aa0ebe2b27.jpg
CLASS-COUNTS-STAGE-adds: AGREE=16 ORPHAN-ONLY=24 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0
CONSIST-RUNS-STAGE-adds: 40
RESTORED-RUNS-STAGE-adds: 40
RESTORED-RUN-NUMBERS-STAGE-adds: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
CLASS-AGREE-STAGE-adds: 16
CLASS-ORPHAN-ONLY-STAGE-adds: 24
CLASS-ROW-WITHOUT-FILE-STAGE-adds: 0
CLASS-BACKUP-REFUSED-STAGE-adds: 0
CLASS-RESTORE-FAILED-STAGE-adds: 0
EXPOSED-RUNS-STAGE-adds: 36
UPLOADS-IN-WINDOW-STAGE-adds: 65
PAGES-IN-WINDOW-STAGE-adds: 24
DELETES-IN-WINDOW-STAGE-adds: 0
BACKUP-RC-HISTOGRAM-STAGE-adds: 0=40
RETRIED-RUNS-STAGE-adds: 0
RETRY-COMPLETED-RUNS-STAGE-adds: 0
ABORTED-24-RUNS-STAGE-adds: 0
MANIFEST-LESS-DIRS-STAGE-adds: 0
OUTCOME-STAGE-adds: ORPHAN-ONLY-NAMED
CONSIST-SUMMARY-STAGE-adds: RUNS=40 RESTORED=40 AGREE=16 ORPHAN-ONLY=24 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=36 UPLOADS-IN-WINDOW=65 OUTCOME=ORPHAN-ONLY-NAMED
CLASS-COUNTS-STAGE-deletes: AGREE=10 ORPHAN-ONLY=14 ROW-WITHOUT-FILE=16 BACKUP-REFUSED=0
CONSIST-RUNS-STAGE-deletes: 40
RESTORED-RUNS-STAGE-deletes: 40
RESTORED-RUN-NUMBERS-STAGE-deletes: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
CLASS-AGREE-STAGE-deletes: 10
CLASS-ORPHAN-ONLY-STAGE-deletes: 14
CLASS-ROW-WITHOUT-FILE-STAGE-deletes: 16
CLASS-BACKUP-REFUSED-STAGE-deletes: 0
CLASS-RESTORE-FAILED-STAGE-deletes: 0
EXPOSED-RUNS-STAGE-deletes: 38
UPLOADS-IN-WINDOW-STAGE-deletes: 91
PAGES-IN-WINDOW-STAGE-deletes: 29
DELETES-IN-WINDOW-STAGE-deletes: 27
BACKUP-RC-HISTOGRAM-STAGE-deletes: 0=40
RETRIED-RUNS-STAGE-deletes: 8
RETRY-COMPLETED-RUNS-STAGE-deletes: 8
ABORTED-24-RUNS-STAGE-deletes: 0
MANIFEST-LESS-DIRS-STAGE-deletes: 0
OUTCOME-STAGE-deletes: ROW-WITHOUT-FILE-NAMED
CONSIST-SUMMARY-STAGE-deletes: RUNS=40 RESTORED=40 AGREE=10 ORPHAN-ONLY=14 ROW-WITHOUT-FILE=16 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=38 UPLOADS-IN-WINDOW=91 OUTCOME=ROW-WITHOUT-FILE-NAMED
CONSIST OK
```

#### amd64-t29-src/consist-runs.txt

```text
RUN 1 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=25 PAGES=9 DELETES=0 BACKUP-S=2.1 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075437
RUN 2 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=7 DELETES=0 BACKUP-S=2.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075503
RUN 3 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=0 BACKUP-S=2.0 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075530
RUN 4 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=7 DELETES=0 BACKUP-S=2.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075555
RUN 5 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=2.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075620
RUN 6 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=9 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=2.1 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075647
RUN 7 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=8 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=2.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075712
RUN 8 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=2.0 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075737
RUN 9 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=24 PAGES=7 DELETES=0 BACKUP-S=2.0 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075805
RUN 10 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=2.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075829
RUN 11 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=7 DELETES=0 BACKUP-S=2.2 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075855
RUN 12 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=2.0 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075921
RUN 13 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.8 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-075945
RUN 14 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=24 PAGES=7 DELETES=0 BACKUP-S=1.9 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080010
RUN 15 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=2.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080036
RUN 16 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=0 BACKUP-S=2.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080103
RUN 17 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080128
RUN 18 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=15 PAGES=5 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080150
RUN 19 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080215
RUN 20 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080238
RUN 21 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=16 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080301
RUN 22 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080324
RUN 23 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080348
RUN 24 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=2.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080413
RUN 25 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080439
RUN 26 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080503
RUN 27 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080526
RUN 28 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080550
RUN 29 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080613
RUN 30 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080637
RUN 31 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=7 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080701
RUN 32 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080724
RUN 33 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=7 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080747
RUN 34 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080811
RUN 35 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080834
RUN 36 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080857
RUN 37 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080920
RUN 38 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-080944
RUN 39 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081008
RUN 40 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081031
RUN 1 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=4 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081055
RUN 2 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=5 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081119
RUN 3 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=24 PAGES=7 DELETES=6 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081143
RUN 4 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081207
RUN 5 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=13 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=22 PAGES=7 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step,straddles-media-end RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081232
RUN 6 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=1 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=23 PAGES=6 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081257
RUN 7 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081321
RUN 8 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=22 PAGES=7 DELETES=6 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081346
RUN 9 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=8 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=19 PAGES=5 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081410
RUN 10 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081434
RUN 11 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=9 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=19 PAGES=5 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081458
RUN 12 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081523
RUN 13 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081547
RUN 14 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=1 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=22 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081612
RUN 15 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=4 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081635
RUN 16 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=10 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=21 PAGES=7 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=after-snapshot,overlaps-snapshot-step,straddles-media-end RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081700
RUN 17 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081725
RUN 18 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=21 PAGES=7 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081750
RUN 19 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step,after-snapshot,straddles-media-end RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081815
RUN 20 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=21 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081840
RUN 21 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081905
RUN 22 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=0 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=7 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081930
RUN 23 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=11 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=22 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=after-snapshot,overlaps-snapshot-step RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-081954
RUN 24 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=0 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082019
RUN 25 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=19 PAGES=5 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082044
RUN 26 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082109
RUN 27 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=9 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=22 PAGES=7 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step,after-snapshot RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082134
RUN 28 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=19 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082200
RUN 29 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082225
RUN 30 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=23 PAGES=7 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082251
RUN 31 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=24 PAGES=7 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082317
RUN 32 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=21 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082341
RUN 33 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=5 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=7 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082407
RUN 34 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082431
RUN 35 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=24 PAGES=7 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082457
RUN 36 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082522
RUN 37 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082548
RUN 38 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=21 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082613
RUN 39 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=12 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=2 UPLOADS=22 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step,after-snapshot RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082639
RUN 40 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=12 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=23 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step,after-snapshot RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/opt/holzcloud/backups/20261027-082705
```

#### amd64-t29-src/consist-run-1-backup.txt

```text
1793087677987 === holzcloud-CMS backup: 20261027-075437 ===
1793087678161 Backing up database...
1793087678422 2026/10/27 07:54:38 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793087678472 2026/10/27 07:54:38 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-075437/holzcloud.sqlite
1793087678492 2026/10/27 07:54:38 INFO backup verified path=/opt/holzcloud/backups/20261027-075437/holzcloud.sqlite tables=57
1793087678511 backup written and verified: /opt/holzcloud/backups/20261027-075437/holzcloud.sqlite
1793087678533 Backing up CSRF key...
1793087678555 Backing up media...
1793087679027 Backing up plugins...
1793087679231 Backing up sprachen...
1793087679350 Backing up marke...
1793087679725 Pruning backups older than 30 days...
1793087679784 === Backup complete: /opt/holzcloud/backups/20261027-075437 ===
1793087679810 121M	/opt/holzcloud/backups/20261027-075437
WALL 1793087677708 1793087679843
RC 0
```

#### amd64-t29-src/consist-run-1-restore.txt

```text
1793087685283 === Restoring from /opt/holzcloud/backups/20261027-075437 ===
1793087685302 timestamp=20261027-075437
1793087685320 version=holzcloud v0.1.0-123-gdd077e4 (dd077e4)
1793087685339 data_dir=/opt/holzcloud/data
1793087685360 host=10cd02c67352
1793087685377 Verifying snapshot...
1793087685471 2026/10/27 07:54:45 INFO sqlite opened journal_mode=wal path=/tmp/tmp.ZFB8AqKmvM/holzcloud.sqlite
1793087685502 integrity: ok
1793087685519 Stopping holzcloud...
1793087685765 Moving current data to /opt/holzcloud/data.before-restore-20261027-075445
1793087685797 Restoring database...
1793087685831 Restoring CSRF key...
1793087685851 Restoring media...
1793087686081 Restoring plugins...
1793087686201 Restoring sprachen...
1793087686312 Restoring marke...
1793087686451 Starting holzcloud...
1793087686519 Waiting for readiness...
1793087698385 === Restore complete ===
1793087698446 {"status":"ok","version":"v0.1.0-123-gdd077e4","commit":"dd077e4","uptime_s":0,"integrity":"ok","disk_free_bytes":1814335135744,"database":{"freelist_count":0,"page_count":163,"size_bytes":667648,"wal_bytes":0}}
1793087698464 
WALL 1793087685129 1793087698490
RC 0
```

#### amd64-t29-src/consist-run-1-compare.txt

```text
RUN: adds 1 restored=yes
BACKUP-DIR: /opt/holzcloud/backups/20261027-075437
ROWS: 57
FILES: 57
MISSING: 0
ORPHAN: 0
EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)
CLASS: AGREE
PASS media and database agree in both directions (57 rows, 57 files)
PASS holzcloud check on a copy: 2026/10/27 07:54:59 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.VPHPjx/holzcloud.sqlite integrity: ok
PASS PRAGMA integrity_check: ok
PASS PRAGMA foreign_key_check: no violation
INFO PAGE-REFS: 1 media files named by page HTML
PASS PAGE-REFS-DANGLING: 0
PASS the sha256 list of the restored tree (90 files) equals that of the backup directory it came from
```

#### amd64-t29-src/consist-run-2-compare.txt

```text
RUN: adds 2 restored=yes
BACKUP-DIR: /opt/holzcloud/backups/20261027-075503
ROWS: 114
FILES: 116
MISSING: 0
ORPHAN: 2
EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)
CLASS: ORPHAN-ONLY
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w4-3.jpg 1793087703666 1793087704230) file 1/edc1250662121ff1fb219c651b66c003-thumb.jpg
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w4-3.jpg 1793087703666 1793087704230) file 1/edc1250662121ff1fb219c651b66c003.jpg
INFO disagreement: 0 rows without a file, 2 files without a row (class ORPHAN-ONLY)
KEY ORPHAN-FILE: 1/edc1250662121ff1fb219c651b66c003-thumb.jpg
KEY ORPHAN-FILE: 1/edc1250662121ff1fb219c651b66c003.jpg
PASS holzcloud check on a copy: 2026/10/27 07:55:26 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.RVECtG/holzcloud.sqlite integrity: ok
PASS PRAGMA integrity_check: ok
PASS PRAGMA foreign_key_check: no violation
INFO PAGE-REFS: 1 media files named by page HTML
PASS PAGE-REFS-DANGLING: 0
PASS the sha256 list of the restored tree (149 files) equals that of the backup directory it came from
```

#### amd64-t29-src/consist-deletes-run-5-backup.txt

```text
1793088752100 === holzcloud-CMS backup: 20261027-081232 ===
1793088752181 Backing up database...
1793088752322 2026/10/27 08:12:32 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793088752376 2026/10/27 08:12:32 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-081232/holzcloud.sqlite
1793088752404 2026/10/27 08:12:32 INFO backup verified path=/opt/holzcloud/backups/20261027-081232/holzcloud.sqlite tables=57
1793088752429 backup written and verified: /opt/holzcloud/backups/20261027-081232/holzcloud.sqlite
1793088752451 Backing up CSRF key...
1793088752488 Backing up media...
1793088752622 file has vanished: "/opt/holzcloud/data/media/1/fadef39d412922d7a866e3f970bf571d-medium.jpg"
1793088752640 file has vanished: "/opt/holzcloud/data/media/1/fadef39d412922d7a866e3f970bf571d-thumb.jpg"
1793088752657 file has vanished: "/opt/holzcloud/data/media/1/fadef39d412922d7a866e3f970bf571d.jpg"
1793088752878 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1347) [sender=3.2.7]
1793088752913 WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
1793088753011 Backing up plugins...
1793088753140 Backing up sprachen...
1793088753244 Backing up marke...
1793088753467 Pruning backups older than 30 days...
1793088753491 === Backup complete: /opt/holzcloud/backups/20261027-081232 ===
1793088753518 163M	/opt/holzcloud/backups/20261027-081232
WALL 1793088751927 1793088753543
RC 0
```

#### amd64-t29-src/consist-deletes-run-5-restore.txt

```text
1793088759408 === Restoring from /opt/holzcloud/backups/20261027-081232 ===
1793088759427 timestamp=20261027-081232
1793088759445 version=holzcloud v0.1.0-123-gdd077e4 (dd077e4)
1793088759465 data_dir=/opt/holzcloud/data
1793088759483 host=10cd02c67352
1793088759500 Verifying snapshot...
1793088759601 2026/10/27 08:12:39 INFO sqlite opened journal_mode=wal path=/tmp/tmp.P2qS3M0urT/holzcloud.sqlite
1793088759632 integrity: ok
1793088759649 Stopping holzcloud...
1793088759712 Moving current data to /opt/holzcloud/data.before-restore-20261027-081239
1793088759744 Restoring database...
1793088759776 Restoring CSRF key...
1793088759794 Restoring media...
1793088760141 Restoring plugins...
1793088760272 Restoring sprachen...
1793088760386 Restoring marke...
1793088760544 Starting holzcloud...
1793088760597 Waiting for readiness...
1793088771315 === Restore complete ===
1793088771362 {"status":"ok","version":"v0.1.0-123-gdd077e4","commit":"dd077e4","uptime_s":0,"integrity":"ok","disk_free_bytes":1809877233664,"database":{"freelist_count":0,"page_count":350,"size_bytes":1433600,"wal_bytes":0}}
1793088771383 
WALL 1793088759257 1793088771406
RC 0
```

#### amd64-t29-src/consist-deletes-run-5-compare.txt

```text
RUN: deletes 5 restored=yes
BACKUP-DIR: /opt/holzcloud/backups/20261027-081232
ROWS: 2652
FILES: 2662
MISSING: 3
ORPHAN: 13
EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)
CLASS: ROW-WITHOUT-FILE
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w2-4.jpg 1793088752367 1793088752778) file 1/29f26b968d35e0603484bd1a693f83ff.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w3-4.jpg 1793088752304 1793088752692) file 1/578f39302fe560a23cd55237d2bdc769.jpg
WINDOW-NAME run deletes 5: straddles-media-end (upload churn-deletes5-w1-5.jpg 1793088752644 1793088753053) file 1/9555e5461d1ce1d4b1599bc5657605bf.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793088752098 1793088752462) file 1/a3a9253b0be942dcc89f36c24ac0efca.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w4-4.jpg 1793088752353 1793088752753) file 1/a83d4995dfc51d7679fc429ca612958e.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d-medium.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (delete 25:1/fadef39d412922d7a866e3f970bf571d 1793088752310 1793088752606) file 1/fadef39d412922d7a866e3f970bf571d.jpg
INFO disagreement: 3 rows without a file, 13 files without a row (class ROW-WITHOUT-FILE)
KEY MISSING-FILE: 1/fadef39d412922d7a866e3f970bf571d-medium.jpg
KEY MISSING-FILE: 1/fadef39d412922d7a866e3f970bf571d-thumb.jpg
KEY MISSING-FILE: 1/fadef39d412922d7a866e3f970bf571d.jpg
KEY ORPHAN-FILE: 1/29f26b968d35e0603484bd1a693f83ff-medium.jpg
KEY ORPHAN-FILE: 1/29f26b968d35e0603484bd1a693f83ff-thumb.jpg
KEY ORPHAN-FILE: 1/29f26b968d35e0603484bd1a693f83ff.jpg
KEY ORPHAN-FILE: 1/578f39302fe560a23cd55237d2bdc769-medium.jpg
KEY ORPHAN-FILE: 1/578f39302fe560a23cd55237d2bdc769-thumb.jpg
KEY ORPHAN-FILE: 1/578f39302fe560a23cd55237d2bdc769.jpg
KEY ORPHAN-FILE: 1/9555e5461d1ce1d4b1599bc5657605bf.jpg
KEY ORPHAN-FILE: 1/a3a9253b0be942dcc89f36c24ac0efca-medium.jpg
KEY ORPHAN-FILE: 1/a3a9253b0be942dcc89f36c24ac0efca-thumb.jpg
KEY ORPHAN-FILE: 1/a3a9253b0be942dcc89f36c24ac0efca.jpg
KEY ORPHAN-FILE: 1/a83d4995dfc51d7679fc429ca612958e-medium.jpg
KEY ORPHAN-FILE: 1/a83d4995dfc51d7679fc429ca612958e-thumb.jpg
KEY ORPHAN-FILE: 1/a83d4995dfc51d7679fc429ca612958e.jpg
PASS holzcloud check on a copy: 2026/10/27 08:12:51 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.Ne50pF/holzcloud.sqlite integrity: ok
PASS PRAGMA integrity_check: ok
PASS PRAGMA foreign_key_check: no violation
INFO PAGE-REFS: 1 media files named by page HTML
PASS PAGE-REFS-DANGLING: 0
PASS the sha256 list of the restored tree (2695 files) equals that of the backup directory it came from
```

### R3 CON-01 window

#### amd64-t29-src/window.txt

```text
BACKUP-LINE: deploy/DEPLOY.md:657: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups (HOLZCLOUD_BIN=/opt/holzcloud/hc-hold-bin is put in front of the script through env, as backup.sh documents)
RESTORE-LINE: deploy/DEPLOY.md:971: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500 (yes on stdin)
WEBSITE: 1
HOLD-BOUND-S: 90 (a bounded wait in the stand-in; the driver removes the flag as soon as its write is done)
INFO MECHANISM (read from the code, not measured): an upload writes the file (internal/media/intake.go:163) before it inserts the row (internal/media/intake.go:177)
INFO MECHANISM (read from the code, not measured): the file is created straight under its final name (internal/media/upload.go:107), no temporary name and no rename, so a copy that meets it half written holds a partial file for which there is no row yet
INFO MECHANISM (read from the code, not measured): a delete removes the variant files (internal/media/usage.go:191), then the row (internal/media/usage.go:193), then the original (internal/media/usage.go:198); a snapshot taken after the first step has the row, a copy taken after the last step has no file
INFO MECHANISM (read from the code, not measured): the snapshot is VACUUM INTO (internal/db/maintain.go:81) run by the command line process 'holzcloud backup' (cmd/holzcloud/cli.go:282), not by the service; it is a consistent database file at one instant, taken before the media are copied
INFO MECHANISM (read from the code, not measured): backup.sh runs with set -e (deploy/backup.sh:2) and copies the media through run_rsync (deploy/backup.sh:165, defined at deploy/backup.sh:100), which repeats an rsync that ended with status 24, a file vanished during the copy, once; a second status 24 ends the script before the MANIFEST is written (deploy/backup.sh:174), the cleanup then removes the directory of the night, and restore.sh refuses a directory without a MANIFEST
PASS the editor logs in for the controls: HTTP 303 /admin/
SEED-UPLOAD: not needed, 1417 churn uploads exist for control C2 to delete
CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE
CONTROL-C0-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C0-WRITE: nothing written
PASS control C0 (none): class AGREE, as expected
CONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY
CONTROL-C1-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C1-WRITE: uploaded window-c1-1793089674.jpg: HTTP 303 /admin/websites/1/media, row and file 1/0de6a1aaf8ab2e4d1352e53dc0b8381a.jpg in the live site
ORPHAN-FILE-C1: 1/0de6a1aaf8ab2e4d1352e53dc0b8381a-medium.jpg
ORPHAN-FILE-C1: 1/0de6a1aaf8ab2e4d1352e53dc0b8381a-thumb.jpg
ORPHAN-FILE-C1: 1/0de6a1aaf8ab2e4d1352e53dc0b8381a.jpg
PASS control C1 (upload): class ORPHAN-ONLY, as expected
CONTROL C2 ACTION=delete HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=3 ORPHAN=0 CLASS=ROW-WITHOUT-FILE EXPECTED=ROW-WITHOUT-FILE
CONTROL-C2-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C2-WRITE: deleted media 219 (1/096f5e878213e22e7f8082f5b778bd65.jpg): HTTP 303 /admin/websites/1/media, row gone, 0 files of it left in the live site
MISSING-FILE-C2: 1/096f5e878213e22e7f8082f5b778bd65-medium.jpg
MISSING-FILE-C2: 1/096f5e878213e22e7f8082f5b778bd65-thumb.jpg
MISSING-FILE-C2: 1/096f5e878213e22e7f8082f5b778bd65.jpg
PASS control C2 (delete): class ROW-WITHOUT-FILE, as expected
C2-CONSEQUENCE media route, file the backup lacks (GET /media/1/096f5e878213e22e7f8082f5b778bd65.jpg, Host test.local): HTTP status and body length: 404 19
C2-CONSEQUENCE media route, file the backup has (GET /media/1/53edf2234b0d700713f475d07008347f.jpg, Host test.local): HTTP status and body length: 200 21611
C2-CONSEQUENCE holzcloud check on a copy of the restored database: exit status 0 and output: 2026/10/27 08:28:39 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.33EfO0/holzcloud.sqlite integrity: ok
C2-CONSEQUENCE restore.sh exit status 0, its last line (empty lines skipped): {"status":"ok","version":"v0.1.0-123-gdd077e4","commit":"dd077e4","uptime_s":0,"integrity":"ok","disk_free_bytes":1804436672512,"database":{"freelist_count":0,"page_count":476,"size_bytes":1949696,"wal_bytes":0}}
C2-CONSEQUENCE editor login on the restored site: HTTP 303 /admin/
C2-CONSEQUENCE media list of the admin (editor session, GET /admin/websites/1/media): HTTP 200 -
C2-CONSEQUENCE restored site: INFO PAGE-REFS: 1 media files named by page HTML
C2-CONSEQUENCE restored site: PASS PAGE-REFS-DANGLING: 0
PASS the stand-in, the flag and the bound marker are gone from holzcloud-t29-src
PASS no process of the stand-in is left in holzcloud-t29-src
WINDOW OK
```

### R4 CON-03, ROB-01 and ROB-02 capfull

#### amd64-t29-src/capfull.txt

```text
SHIPPED-MIN-FREE-MB: 1024 (read from /opt/holzcloud/deploy/backup.sh)
MIN-FREE-MB-IN-ENVIRONMENT: '' (empty: backup.sh runs with its default); unit Environment: DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30
UNIT-DROPINS: '' (empty: the shipped unit as it is)
BACKUP-LINE: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
BINARY: holzcloud v0.1.0-123-gdd077e4 (dd077e4)
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
EARLIER-BACKUPS-BEFORE: 60 (20261027-075325 20261027-075355 20261027-075437 20261027-075503 20261027-075555 20261027-075647 20261027-075712 20261027-075737 20261027-075805 20261027-075829 20261027-075921 20261027-075945 20261027-080010 20261027-080036 20261027-080103 20261027-080150 20261027-080301 20261027-080324 20261027-080413 20261027-080439 20261027-080613 20261027-080637 20261027-080724 20261027-080857 20261027-080920 20261027-080944 20261027-081008 20261027-081055 20261027-081119 20261027-081143 20261027-081232 20261027-081257 20261027-081346 20261027-081410 20261027-081434 20261027-081458 20261027-081523 20261027-081547 20261027-081612 20261027-081700 20261027-081725 20261027-081815 20261027-081840 20261027-081905 20261027-081930 20261027-081954 20261027-082019 20261027-082044 20261027-082134 20261027-082200 20261027-082225 20261027-082251 20261027-082407 20261027-082431 20261027-082457 20261027-082639 20261027-082705 20261027-082733 20261027-082754 20261027-082815)
BULK: BULK-MB: 1150 FILES: 230 BYTES: 1205874688 MEDIA-BYTES: 1262942094 
FLOOR-MB: 1024
DATA-DIR-BYTES: 1289701305 (everything below /opt/holzcloud/data, the pre-upgrade snapshots included, which a backup does not carry)
SOURCE-TREES: bytes 1282414625 files 4496 (media, templates, plugins, sprachen, marke)
DATA-MB: 1225 (what a backup has to write: those trees, the database file and csrf.key)
FS-SIZE-MB: 1064
PASS capfull_data_ok: the filesystem of 1064 MB is at least the floor (1024 MB) and smaller than the 1225 MB a backup has to write
CONTROL-FS-SIZE-MB: 2649
CONTROL-FREE-AT-GUARD-MB: 2649
CONTROL-PARTIAL-DIR-TREES: bytes 1282414625 files 4496 in the five directories, the source holds 1282414625 bytes in 4496 files
CONTROL-GUARD: passed
CONTROL-BACKUP-RC: 0
CONTROL-RSYNC-STATUS: -
CONTROL-NO-SPACE-LINES: 0
CONTROL-DIRECTORY-MADE: 20261027-082851
CONTROL-DIRECTORY-BYTES: 1284455154
CONTROL-DIRECTORY-FILES: 4499
CONTROL-MANIFEST: yes
CONTROL-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
CONTROL-SENTINEL-INTACT: none-planted
CONTROL-REMOVAL-LINE: no
CONTROL-CLASS: COMPLETE
CONTROL-LAST-LINES: 1793089731847 Backing up CSRF key...
CONTROL-LAST-LINES: 1793089731866 Backing up media...
CONTROL-LAST-LINES: 1793089733476 Backing up plugins...
CONTROL-LAST-LINES: 1793089733609 Backing up sprachen...
CONTROL-LAST-LINES: 1793089733715 Backing up marke...
CONTROL-LAST-LINES: 1793089733942 Pruning backups older than 30 days...
CONTROL-LAST-LINES: 1793089733967 === Backup complete: /opt/holzcloud/backups/20261027-082851 ===
CONTROL-LAST-LINES: 1793089733987 1.3G	/opt/holzcloud/backups/20261027-082851
CONTROL-LAST-LINES: WALL 1793089731380 1793089734009
CONTROL-LAST-LINES: RC 0
PASS the control completes on a roomy filesystem: exit 0, '=== Backup complete ===', all of the source
CONTROL-CLASS: COMPLETE
SENTINEL: /opt/holzcloud/backups/20261026-082855 sha256 35a58afed09e880dc008e4cc491655f98e4d90550365ba955ab9903df58ff4ed
FREE-AT-GUARD-MB: 1064 (df -Pm at the backup root just before the run, the way backup.sh reads it; the floor is 1024)
HAND-GUARD: passed
HAND-BACKUP-RC: 11
HAND-RSYNC-STATUS: 11
HAND-NO-SPACE-LINES: 1
HAND-DIRECTORY-MADE: none
HAND-DIRECTORY-BYTES: 0
HAND-DIRECTORY-FILES: 0
HAND-MANIFEST: no
HAND-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
HAND-SENTINEL-INTACT: yes
HAND-REMOVAL-LINE: yes
HAND-CLASS: FAILED-NO-DIR
HAND-LAST-LINES: 1793089736799 2026/10/27 08:28:56 INFO backup verified path=/opt/holzcloud/backups/20261027-082856/holzcloud.sqlite tables=57
HAND-LAST-LINES: 1793089736818 backup written and verified: /opt/holzcloud/backups/20261027-082856/holzcloud.sqlite
HAND-LAST-LINES: 1793089736835 Backing up CSRF key...
HAND-LAST-LINES: 1793089736852 Backing up media...
HAND-LAST-LINES: 1793089738178 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082856/media/1/bulk/bulk-71.bin": No space left on device (28)
HAND-LAST-LINES: 1793089738194 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
HAND-LAST-LINES: 1793089738289 rsync: [sender] write error: Broken pipe (32)
HAND-LAST-LINES: 1793089738355 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082856, which had no MANIFEST
HAND-LAST-LINES: WALL 1793089736372 1793089738376
HAND-LAST-LINES: RC 11
GUARD: passed
BACKUP-RC: 11
PARTIAL-DIR: none
PARTIAL-DIR-BYTES: 0
MANIFEST-IN-PARTIAL: no
SENTINEL-INTACT: yes
HAND-CLASS: FAILED-NO-DIR
INFO backup.sh on the nearly full filesystem: FAILED-NO-DIR
HAND-ROBUST: PASS
PASS HAND: the failed night (FAILED-NO-DIR) left no directory, named the removal on stderr and left the sentinel of the earlier night intact (yes)
AGAIN-FREE-AT-GUARD-MB: 1064
AGAIN-GUARD: passed
AGAIN-BACKUP-RC: 11
AGAIN-RSYNC-STATUS: 11
AGAIN-NO-SPACE-LINES: 1
AGAIN-DIRECTORY-MADE: none
AGAIN-DIRECTORY-BYTES: 0
AGAIN-DIRECTORY-FILES: 0
AGAIN-MANIFEST: no
AGAIN-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
AGAIN-SENTINEL-INTACT: yes
AGAIN-REMOVAL-LINE: yes
AGAIN-CLASS: FAILED-NO-DIR
AGAIN-LAST-LINES: 1793089739923 2026/10/27 08:28:59 INFO backup verified path=/opt/holzcloud/backups/20261027-082859/holzcloud.sqlite tables=57
AGAIN-LAST-LINES: 1793089739943 backup written and verified: /opt/holzcloud/backups/20261027-082859/holzcloud.sqlite
AGAIN-LAST-LINES: 1793089739962 Backing up CSRF key...
AGAIN-LAST-LINES: 1793089739979 Backing up media...
AGAIN-LAST-LINES: 1793089741328 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082859/media/1/bulk/bulk-71.bin": No space left on device (28)
AGAIN-LAST-LINES: 1793089741344 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
AGAIN-LAST-LINES: 1793089741448 rsync: [sender] write error: Broken pipe (32)
AGAIN-LAST-LINES: 1793089741520 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082859, which had no MANIFEST
AGAIN-LAST-LINES: WALL 1793089739494 1793089741541
AGAIN-LAST-LINES: RC 11
AGAIN-CLASS: FAILED-NO-DIR
AGAIN-NEXT-NIGHT: PASS
PASS the next night was not refused by the guard and failed the same way without leaving a directory
AGAIN-ROBUST: PASS
PASS AGAIN: the failed night (FAILED-NO-DIR) left no directory, named the removal on stderr and left the sentinel of the earlier night intact (yes)
RESTORE-PARTIAL-EXIT: not-run (the run by hand left no directory)
UNIT-FREE-AT-GUARD-MB: 1064
UNIT-INVOCATION: c8e1eda3c43c4d5db19006fa0c8fc2bd (started by systemctl start, previous invocation 'none')
UNIT-ACTIVE-STATE: failed
UNIT-RESULT: exit-code
UNIT-EXECMAINSTATUS: 11
UNIT-TRIGGEREDBY: 'holzcloud-backup.timer'
UNIT-JOURNAL-LINES: 12
UNIT-GUARD: passed
UNIT-BACKUP-RC: 11
UNIT-RSYNC-STATUS: 11
UNIT-NO-SPACE-LINES: 1
UNIT-DIRECTORY-MADE: none
UNIT-DIRECTORY-BYTES: 0
UNIT-DIRECTORY-FILES: 0
UNIT-MANIFEST: no
UNIT-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
UNIT-SENTINEL-INTACT: yes
UNIT-REMOVAL-LINE: yes
UNIT-CLASS: FAILED-NO-DIR
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082903/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO backup verified path=/opt/holzcloud/backups/20261027-082903/holzcloud.sqlite tables=57
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: backup written and verified: /opt/holzcloud/backups/20261027-082903/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: Backing up CSRF key...
UNIT-LAST-LINES: 2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: Backing up media...
UNIT-LAST-LINES: 2026-10-27T08:29:04+0000 10cd02c67352 backup.sh[61005]: rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082903/media/1/bulk/bulk-71.bin": No space left on device (28)
UNIT-LAST-LINES: 2026-10-27T08:29:04+0000 10cd02c67352 backup.sh[61005]: rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
UNIT-LAST-LINES: 2026-10-27T08:29:05+0000 10cd02c67352 backup.sh[61004]: rsync: [sender] write error: Broken pipe (32)
UNIT-LAST-LINES: 2026-10-27T08:29:05+0000 10cd02c67352 backup.sh[60984]: ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082903, which had no MANIFEST
UNIT-CLASS: FAILED-NO-DIR
UNIT-ROBUST: PASS
PASS UNIT: the failed night (FAILED-NO-DIR) left no directory, named the removal on stderr and left the sentinel of the earlier night intact (yes)
VANISH-ONCE-SENTINEL: /opt/holzcloud/backups/20261026-082910 sha256 48e92645b583c5fe24a177d18299ead7369846c98836355e68c596072df72903
VANISH-ONCE-STAND-IN: /usr/local/bin/rsync, the first 1 call(s) that copy the media directory end with 24 (mode hand)
VANISH-ONCE-PARTIAL-DIR-TREES: bytes 1282414625 files 4496 in the five directories, the source holds 1282414625 bytes in 4496 files
VANISH-ONCE-GUARD: passed
VANISH-ONCE-BACKUP-RC: 0
VANISH-ONCE-RSYNC-STATUS: 24
VANISH-ONCE-NO-SPACE-LINES: 0
VANISH-ONCE-DIRECTORY-MADE: 20261027-082911
VANISH-ONCE-DIRECTORY-BYTES: 1284455154
VANISH-ONCE-DIRECTORY-FILES: 4499
VANISH-ONCE-MANIFEST: yes
VANISH-ONCE-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
VANISH-ONCE-SENTINEL-INTACT: yes
VANISH-ONCE-REMOVAL-LINE: no
VANISH-ONCE-CLASS: COMPLETE
VANISH-ONCE-LAST-LINES: 1793089753703 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-ONCE-LAST-LINES: 1793089753724 WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
VANISH-ONCE-LAST-LINES: 1793089753885 Backing up plugins...
VANISH-ONCE-LAST-LINES: 1793089754045 Backing up sprachen...
VANISH-ONCE-LAST-LINES: 1793089754179 Backing up marke...
VANISH-ONCE-LAST-LINES: 1793089754438 Pruning backups older than 30 days...
VANISH-ONCE-LAST-LINES: 1793089754464 === Backup complete: /opt/holzcloud/backups/20261027-082911 ===
VANISH-ONCE-LAST-LINES: 1793089754481 1.3G	/opt/holzcloud/backups/20261027-082911
VANISH-ONCE-LAST-LINES: WALL 1793089751686 1793089754502
VANISH-ONCE-LAST-LINES: RC 0
VANISH-ONCE-REPEAT-LINES: 1
VANISH-ONCE-GIVEUP-LINES: 0
VANISH-ONCE-VERDICT: PASS
PASS VANISH-ONCE: the shipped script met a status 24 (once) and ended COMPLETE with status 0, 1 repeat line(s), 0 giving-up line(s)
PASS VANISH-ONCE: the stand-in, its configuration and its counter are gone from the container
VANISH-TWICE-SENTINEL: /opt/holzcloud/backups/20261026-082916 sha256 3021c18e8a49936abda9a7f5889d42d63e4dfa4988274fd026aa5b3da72e222a
VANISH-TWICE-STAND-IN: /usr/local/bin/rsync, the first 2 call(s) that copy the media directory end with 24 (mode hand)
VANISH-TWICE-GUARD: passed
VANISH-TWICE-BACKUP-RC: 24
VANISH-TWICE-RSYNC-STATUS: 24
VANISH-TWICE-NO-SPACE-LINES: 0
VANISH-TWICE-DIRECTORY-MADE: none
VANISH-TWICE-DIRECTORY-BYTES: 0
VANISH-TWICE-DIRECTORY-FILES: 0
VANISH-TWICE-MANIFEST: no
VANISH-TWICE-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
VANISH-TWICE-SENTINEL-INTACT: yes
VANISH-TWICE-REMOVAL-LINE: yes
VANISH-TWICE-CLASS: FAILED-NO-DIR
VANISH-TWICE-LAST-LINES: 1793089757945 backup written and verified: /opt/holzcloud/backups/20261027-082917/holzcloud.sqlite
VANISH-TWICE-LAST-LINES: 1793089757961 Backing up CSRF key...
VANISH-TWICE-LAST-LINES: 1793089757978 Backing up media...
VANISH-TWICE-LAST-LINES: 1793089759494 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-TWICE-LAST-LINES: 1793089759510 WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
VANISH-TWICE-LAST-LINES: 1793089759675 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-TWICE-LAST-LINES: 1793089759692 ERROR: rsync exited 24 again while copying media; giving up
VANISH-TWICE-LAST-LINES: 1793089759748 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082917, which had no MANIFEST
VANISH-TWICE-LAST-LINES: WALL 1793089757511 1793089759769
VANISH-TWICE-LAST-LINES: RC 24
VANISH-TWICE-REPEAT-LINES: 1
VANISH-TWICE-GIVEUP-LINES: 1
VANISH-TWICE-VERDICT: PASS
PASS VANISH-TWICE: the shipped script met a status 24 (twice) and ended FAILED-NO-DIR with status 24, 1 repeat line(s), 1 giving-up line(s)
VANISH-TWICE-ROBUST: PASS
PASS VANISH-TWICE: the failed night (FAILED-NO-DIR) left no directory, named the removal on stderr and left the sentinel of the earlier night intact (yes)
PASS VANISH-TWICE: the stand-in, its configuration and its counter are gone from the container
VANISH-ONCE-UNIT-SENTINEL: /opt/holzcloud/backups/20261026-082920 sha256 6d17138e8e3f576c3ec1165e76629241eb2bbe1c80f842b0a89b87091359486b
VANISH-ONCE-UNIT-STAND-IN: /usr/local/bin/rsync, the first 1 call(s) that copy the media directory end with 24 (mode unit)
VANISH-ONCE-UNIT-INVOCATION: 9f5bc0dd12734f29963a91cad47977d8 (started by systemctl start, previous invocation 'c8e1eda3c43c4d5db19006fa0c8fc2bd')
VANISH-ONCE-UNIT-ACTIVE-STATE: inactive
VANISH-ONCE-UNIT-RESULT: success
VANISH-ONCE-UNIT-EXECMAINSTATUS: 0
VANISH-ONCE-UNIT-TRIGGEREDBY: 'holzcloud-backup.timer'
VANISH-ONCE-UNIT-JOURNAL-LINES: 16
VANISH-ONCE-UNIT-PARTIAL-DIR-TREES: bytes 1282414625 files 4496 in the five directories, the source holds 1282414625 bytes in 4496 files
VANISH-ONCE-UNIT-GUARD: passed
VANISH-ONCE-UNIT-BACKUP-RC: 0
VANISH-ONCE-UNIT-RSYNC-STATUS: 24
VANISH-ONCE-UNIT-NO-SPACE-LINES: 0
VANISH-ONCE-UNIT-DIRECTORY-MADE: 20261027-082922
VANISH-ONCE-UNIT-DIRECTORY-BYTES: 1284455154
VANISH-ONCE-UNIT-DIRECTORY-FILES: 4499
VANISH-ONCE-UNIT-MANIFEST: yes
VANISH-ONCE-UNIT-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
VANISH-ONCE-UNIT-SENTINEL-INTACT: yes
VANISH-ONCE-UNIT-REMOVAL-LINE: no
VANISH-ONCE-UNIT-CLASS: COMPLETE
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: Backing up CSRF key...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: Backing up media...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:23+0000 10cd02c67352 backup.sh[61719]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:23+0000 10cd02c67352 backup.sh[61699]: WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up plugins...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up sprachen...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up marke...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Pruning backups older than 30 days...
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: === Backup complete: /opt/holzcloud/backups/20261027-082922 ===
VANISH-ONCE-UNIT-LAST-LINES: 2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61772]: 1.3G        /opt/holzcloud/backups/20261027-082922
VANISH-ONCE-UNIT-REPEAT-LINES: 1
VANISH-ONCE-UNIT-GIVEUP-LINES: 0
VANISH-ONCE-UNIT-UNIT-STATE-CHECK: PASS
VANISH-ONCE-UNIT-VERDICT: PASS
PASS VANISH-ONCE-UNIT: the shipped script met a status 24 (once) and ended COMPLETE with status 0, 1 repeat line(s), 0 giving-up line(s)
PASS VANISH-ONCE-UNIT: the stand-in, its configuration and its counter are gone from the container
VANISH-TWICE-UNIT-SENTINEL: /opt/holzcloud/backups/20261026-082930 sha256 4cd4fb4aa9b7587290fdccb53237ed1e80daa5bf4a1f2b2885cde122380e91b7
VANISH-TWICE-UNIT-STAND-IN: /usr/local/bin/rsync, the first 2 call(s) that copy the media directory end with 24 (mode unit)
VANISH-TWICE-UNIT-INVOCATION: 6fa2ec58860247f79aa9b9459f3c5238 (started by systemctl start, previous invocation '9f5bc0dd12734f29963a91cad47977d8')
VANISH-TWICE-UNIT-ACTIVE-STATE: failed
VANISH-TWICE-UNIT-RESULT: exit-code
VANISH-TWICE-UNIT-EXECMAINSTATUS: 24
VANISH-TWICE-UNIT-TRIGGEREDBY: 'holzcloud-backup.timer'
VANISH-TWICE-UNIT-JOURNAL-LINES: 13
VANISH-TWICE-UNIT-GUARD: passed
VANISH-TWICE-UNIT-BACKUP-RC: 24
VANISH-TWICE-UNIT-RSYNC-STATUS: 24
VANISH-TWICE-UNIT-NO-SPACE-LINES: 0
VANISH-TWICE-UNIT-DIRECTORY-MADE: none
VANISH-TWICE-UNIT-DIRECTORY-BYTES: 0
VANISH-TWICE-UNIT-DIRECTORY-FILES: 0
VANISH-TWICE-UNIT-MANIFEST: no
VANISH-TWICE-UNIT-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
VANISH-TWICE-UNIT-SENTINEL-INTACT: yes
VANISH-TWICE-UNIT-REMOVAL-LINE: yes
VANISH-TWICE-UNIT-CLASS: FAILED-NO-DIR
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: 2026/10/27 08:29:32 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082931/holzcloud.sqlite
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: 2026/10/27 08:29:32 INFO backup verified path=/opt/holzcloud/backups/20261027-082931/holzcloud.sqlite tables=57
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: backup written and verified: /opt/holzcloud/backups/20261027-082931/holzcloud.sqlite
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62029]: Backing up CSRF key...
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62029]: Backing up media...
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62049]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62067]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: ERROR: rsync exited 24 again while copying media; giving up
VANISH-TWICE-UNIT-LAST-LINES: 2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082931, which had no MANIFEST
VANISH-TWICE-UNIT-REPEAT-LINES: 1
VANISH-TWICE-UNIT-GIVEUP-LINES: 1
VANISH-TWICE-UNIT-UNIT-STATE-CHECK: PASS
VANISH-TWICE-UNIT-VERDICT: PASS
PASS VANISH-TWICE-UNIT: the shipped script met a status 24 (twice) and ended FAILED-NO-DIR with status 24, 1 repeat line(s), 1 giving-up line(s)
VANISH-TWICE-UNIT-ROBUST: PASS
PASS VANISH-TWICE-UNIT: the failed night (FAILED-NO-DIR) left no directory, named the removal on stderr and left the sentinel of the earlier night intact (yes)
PASS VANISH-TWICE-UNIT: the stand-in, its configuration and its counter are gone from the container
VANISH-SHIM-REMOVED: yes
INFO backup.sh runs set -e and its retention step (find -mtime +RETENTION_DAYS) comes after the copies, so a run that ends in the copy never reaches it; that is a reading of the script. What is on disk is in the lines above: the sentinel of an earlier night, which is inside the retention window, shows nothing about deletion
EARLIER-BACKUPS-AFTER: 60 (20261027-075325 20261027-075355 20261027-075437 20261027-075503 20261027-075555 20261027-075647 20261027-075712 20261027-075737 20261027-075805 20261027-075829 20261027-075921 20261027-075945 20261027-080010 20261027-080036 20261027-080103 20261027-080150 20261027-080301 20261027-080324 20261027-080413 20261027-080439 20261027-080613 20261027-080637 20261027-080724 20261027-080857 20261027-080920 20261027-080944 20261027-081008 20261027-081055 20261027-081119 20261027-081143 20261027-081232 20261027-081257 20261027-081346 20261027-081410 20261027-081434 20261027-081458 20261027-081523 20261027-081547 20261027-081612 20261027-081700 20261027-081725 20261027-081815 20261027-081840 20261027-081905 20261027-081930 20261027-081954 20261027-082019 20261027-082044 20261027-082134 20261027-082200 20261027-082225 20261027-082251 20261027-082407 20261027-082431 20261027-082457 20261027-082639 20261027-082705 20261027-082733 20261027-082754 20261027-082815)
PASS the filesystem is unmounted and the earlier backups are back, the same directories as before
CAPFULL OK
```

#### amd64-t29-src/capfull-hand.txt

```text
1793089736534 === holzcloud-CMS backup: 20261027-082856 ===
1793089736607 Backing up database...
1793089736737 2026/10/27 08:28:56 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793089736779 2026/10/27 08:28:56 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082856/holzcloud.sqlite
1793089736799 2026/10/27 08:28:56 INFO backup verified path=/opt/holzcloud/backups/20261027-082856/holzcloud.sqlite tables=57
1793089736818 backup written and verified: /opt/holzcloud/backups/20261027-082856/holzcloud.sqlite
1793089736835 Backing up CSRF key...
1793089736852 Backing up media...
1793089738178 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082856/media/1/bulk/bulk-71.bin": No space left on device (28)
1793089738194 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
1793089738289 rsync: [sender] write error: Broken pipe (32)
1793089738355 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082856, which had no MANIFEST
WALL 1793089736372 1793089738376
RC 11
```

#### amd64-t29-src/capfull-again.txt

```text
1793089739652 === holzcloud-CMS backup: 20261027-082859 ===
1793089739730 Backing up database...
1793089739861 2026/10/27 08:28:59 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793089739902 2026/10/27 08:28:59 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082859/holzcloud.sqlite
1793089739923 2026/10/27 08:28:59 INFO backup verified path=/opt/holzcloud/backups/20261027-082859/holzcloud.sqlite tables=57
1793089739943 backup written and verified: /opt/holzcloud/backups/20261027-082859/holzcloud.sqlite
1793089739962 Backing up CSRF key...
1793089739979 Backing up media...
1793089741328 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082859/media/1/bulk/bulk-71.bin": No space left on device (28)
1793089741344 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
1793089741448 rsync: [sender] write error: Broken pipe (32)
1793089741520 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082859, which had no MANIFEST
WALL 1793089739494 1793089741541
RC 11
```

#### amd64-t29-src/capfull-unit.txt

```text
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: === holzcloud-CMS backup: 20261027-082903 ===
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: Backing up database...
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082903/holzcloud.sqlite
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: 2026/10/27 08:29:03 INFO backup verified path=/opt/holzcloud/backups/20261027-082903/holzcloud.sqlite tables=57
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60992]: backup written and verified: /opt/holzcloud/backups/20261027-082903/holzcloud.sqlite
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: Backing up CSRF key...
2026-10-27T08:29:03+0000 10cd02c67352 backup.sh[60984]: Backing up media...
2026-10-27T08:29:04+0000 10cd02c67352 backup.sh[61005]: rsync: [receiver] write failed on "/opt/holzcloud/backups/20261027-082903/media/1/bulk/bulk-71.bin": No space left on device (28)
2026-10-27T08:29:04+0000 10cd02c67352 backup.sh[61005]: rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
2026-10-27T08:29:05+0000 10cd02c67352 backup.sh[61004]: rsync: [sender] write error: Broken pipe (32)
2026-10-27T08:29:05+0000 10cd02c67352 backup.sh[60984]: ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082903, which had no MANIFEST
```

#### amd64-t29-src/capfull-df.txt

```text
== control before (2649 MB) (2026-10-27T08:28:50Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     0      2649       0% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== control after (2026-10-27T08:28:54Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649  1238      1412      47% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== hand before (1064 MB) (2026-10-27T08:28:55Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== hand after (2026-10-27T08:28:58Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== again before (2026-10-27T08:28:59Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== again after (2026-10-27T08:29:01Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== unit before (1064 MB) (2026-10-27T08:29:02Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== unit after (2026-10-27T08:29:09Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== VANISH-ONCE before (2649 MB) (2026-10-27T08:29:10Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-ONCE after (2026-10-27T08:29:14Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649  1238      1412      47% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-TWICE before (2649 MB) (2026-10-27T08:29:16Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-TWICE after (2026-10-27T08:29:19Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-ONCE-UNIT before (2649 MB) (2026-10-27T08:29:21Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-ONCE-UNIT after (2026-10-27T08:29:28Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649  1238      1412      47% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-TWICE-UNIT before (2649 MB) (2026-10-27T08:29:30Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
== VANISH-TWICE-UNIT after (2026-10-27T08:29:38Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2649     1      2649       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2712576k,mode=700,uid=997,gid=997 0 0
```

#### amd64-t29-src/capfull-vanish-once.txt

```text
1793089751849 === holzcloud-CMS backup: 20261027-082911 ===
1793089751925 Backing up database...
1793089752055 2026/10/27 08:29:12 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793089752098 2026/10/27 08:29:12 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082911/holzcloud.sqlite
1793089752117 2026/10/27 08:29:12 INFO backup verified path=/opt/holzcloud/backups/20261027-082911/holzcloud.sqlite tables=57
1793089752135 backup written and verified: /opt/holzcloud/backups/20261027-082911/holzcloud.sqlite
1793089752151 Backing up CSRF key...
1793089752169 Backing up media...
1793089753703 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
1793089753724 WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
1793089753885 Backing up plugins...
1793089754045 Backing up sprachen...
1793089754179 Backing up marke...
1793089754438 Pruning backups older than 30 days...
1793089754464 === Backup complete: /opt/holzcloud/backups/20261027-082911 ===
1793089754481 1.3G	/opt/holzcloud/backups/20261027-082911
WALL 1793089751686 1793089754502
RC 0
```

#### amd64-t29-src/capfull-vanish-twice.txt

```text
1793089757667 === holzcloud-CMS backup: 20261027-082917 ===
1793089757738 Backing up database...
1793089757866 2026/10/27 08:29:17 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793089757907 2026/10/27 08:29:17 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082917/holzcloud.sqlite
1793089757925 2026/10/27 08:29:17 INFO backup verified path=/opt/holzcloud/backups/20261027-082917/holzcloud.sqlite tables=57
1793089757945 backup written and verified: /opt/holzcloud/backups/20261027-082917/holzcloud.sqlite
1793089757961 Backing up CSRF key...
1793089757978 Backing up media...
1793089759494 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
1793089759510 WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
1793089759675 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
1793089759692 ERROR: rsync exited 24 again while copying media; giving up
1793089759748 ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082917, which had no MANIFEST
WALL 1793089757511 1793089759769
RC 24
```

#### amd64-t29-src/capfull-vanish-once-unit.txt

```text
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: === holzcloud-CMS backup: 20261027-082922 ===
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: Backing up database...
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61707]: 2026/10/27 08:29:22 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61707]: 2026/10/27 08:29:22 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082922/holzcloud.sqlite
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61707]: 2026/10/27 08:29:22 INFO backup verified path=/opt/holzcloud/backups/20261027-082922/holzcloud.sqlite tables=57
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61707]: backup written and verified: /opt/holzcloud/backups/20261027-082922/holzcloud.sqlite
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: Backing up CSRF key...
2026-10-27T08:29:22+0000 10cd02c67352 backup.sh[61699]: Backing up media...
2026-10-27T08:29:23+0000 10cd02c67352 backup.sh[61719]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
2026-10-27T08:29:23+0000 10cd02c67352 backup.sh[61699]: WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up plugins...
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up sprachen...
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Backing up marke...
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: Pruning backups older than 30 days...
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61699]: === Backup complete: /opt/holzcloud/backups/20261027-082922 ===
2026-10-27T08:29:24+0000 10cd02c67352 backup.sh[61772]: 1.3G        /opt/holzcloud/backups/20261027-082922
```

#### amd64-t29-src/capfull-vanish-twice-unit.txt

```text
2026-10-27T08:29:31+0000 10cd02c67352 backup.sh[62029]: === holzcloud-CMS backup: 20261027-082931 ===
2026-10-27T08:29:31+0000 10cd02c67352 backup.sh[62029]: Backing up database...
2026-10-27T08:29:31+0000 10cd02c67352 backup.sh[62037]: 2026/10/27 08:29:31 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: 2026/10/27 08:29:32 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261027-082931/holzcloud.sqlite
2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: 2026/10/27 08:29:32 INFO backup verified path=/opt/holzcloud/backups/20261027-082931/holzcloud.sqlite tables=57
2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62037]: backup written and verified: /opt/holzcloud/backups/20261027-082931/holzcloud.sqlite
2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62029]: Backing up CSRF key...
2026-10-27T08:29:32+0000 10cd02c67352 backup.sh[62029]: Backing up media...
2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62049]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: WARNING: rsync exited 24 (files vanished) while copying media; repeating it once
2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62067]: rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]
2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: ERROR: rsync exited 24 again while copying media; giving up
2026-10-27T08:29:33+0000 10cd02c67352 backup.sh[62029]: ERROR: backup did not finish; removed /opt/holzcloud/backups/20261027-082931, which had no MANIFEST
```

### R5 CON-02 compat

#### amd64-t29-old/compat.txt

```text
COMPAT-TAG: v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712)
NEW-REF: dd077e4
OLD-BINARY: holzcloud v0.1.0 (4d238b9)
PASS version_is_tag: the source container runs exactly the tag v0.1.0
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
FIXTURE-LOGIN: HTTP 303 /admin/
FIXTURE-PAGE-1: compat-gruesse HTTP 303 /admin/websites/1/pages
FIXTURE-PAGE-2: compat-bild HTTP 303 /admin/websites/1/pages
FIXTURE-PAGE-3: compat-liste (published requested) HTTP 303 /admin/websites/1/pages
FIXTURE-UPLOAD: churn-compat-extra.jpg HTTP 303 /admin/websites/1/media
FIXTURE-PAGES-SAVED: 3
PASS the three fixture pages exist on the previous release
FIXTURE-STATUS-compat-liste: published (requested: published; an editor may or may not publish, this is what the database says)
PASS the extra upload has its media row
FIXTURE-MEDIA-USAGE-ROWS-compat-bild: 1
FIXTURE-PAGES-WITH-OUTBOUND-LINK: 1
STATE-SOURCE: LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3 
PASS the staged site is not trivial: at least three pages, two users and two uploaded media rows
BACKUP-SH-SHA256-TAG: d9f807d806a61ef3a9920890394247b3ecd475a17b92de4929ad517e2c17eba2
BACKUP-SH-SHA256-NEW-REF: ac02fac9096d3ed1665794c87a1624f8c896bb980ae615d80be0d4bab7d8de96
BACKUP-SH-SHA256-INSTALLED: ac02fac9096d3ed1665794c87a1624f8c896bb980ae615d80be0d4bab7d8de96
BACKUP-SH: replaced
BACKUP-SH-TAG-EQUALS-NEW-REF: no
BACKUP-LINE: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
BACKUP-RC: 0
BACKUP-DIR: /opt/holzcloud/backups/20261027-083052
MANIFEST: timestamp=20261027-083052 version=holzcloud v0.1.0 (4d238b9) data_dir=/opt/holzcloud/data host=4be841e91412
PASS the MANIFEST of the backup names the tag: version=holzcloud v0.1.0 (4d238b9)
NEW-BINARY: holzcloud v0.1.0-123-gdd077e4 (dd077e4)
SETUP-BEFORE: HTTP 200 -
PASS the setup form answers 200 on the fresh box before the restore
PASS transfer: FILES: 27 files, sha256 equal on both sides
RESTORE-LINE: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20261027-083052
RESTORE-RC: 0
RESTORE-WALL-S: 10.8
PASS restore.sh of this release ended 0 with '=== Restore complete ===' on the previous release's backup
STATE-RESTORED: LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3 
INFO COMPARE source-restored TABLES volatile activity_log: left 26, right 26 (the audit trail, one row per admin action; logging in and saving a page add rows)
INFO COMPARE source-restored TABLES volatile goose_db_version: left 61, right 61 (one row per applied migration; compared as a list of its own, not as a count)
INFO COMPARE source-restored TABLES volatile mail_outbox: left 0, right 0 (the mail queue: a background run drains it and the rows stay only for a while)
INFO COMPARE source-restored TABLES volatile oauth_codes: left 0, right 0 (an authorisation code lives for minutes and is used once)
INFO COMPARE source-restored TABLES volatile outbox: left 0, right 0 (the shop's queue of messages, drained by a background run)
INFO COMPARE source-restored TABLES volatile plugin_store: left 2, right 2 (written by the plugins at run time (the 404 hook stores the missing path))
INFO COMPARE source-restored TABLES volatile sessions: left 4, right 4 (one row per login, deleted at expiry: the comparison itself logs in)
PASS COMPARE source-restored TABLES: the row counts of 47 tables outside the volatile list are equal
PASS COMPARE source-restored GOOSE: the levels are equal (60) and so are the goose rows (61)
PASS COMPARE source-restored PAGES: equal (8 lines)
PASS COMPARE source-restored USERS: equal (2 lines)
PASS COMPARE source-restored MEDIA: equal (12 lines)
INFO COMPARE backup-restored TABLES volatile activity_log: left 26, right 26 (the audit trail, one row per admin action; logging in and saving a page add rows)
INFO COMPARE backup-restored TABLES volatile goose_db_version: left 61, right 61 (one row per applied migration; compared as a list of its own, not as a count)
INFO COMPARE backup-restored TABLES volatile mail_outbox: left 0, right 0 (the mail queue: a background run drains it and the rows stay only for a while)
INFO COMPARE backup-restored TABLES volatile oauth_codes: left 0, right 0 (an authorisation code lives for minutes and is used once)
INFO COMPARE backup-restored TABLES volatile outbox: left 0, right 0 (the shop's queue of messages, drained by a background run)
INFO COMPARE backup-restored TABLES volatile plugin_store: left 2, right 2 (written by the plugins at run time (the 404 hook stores the missing path))
INFO COMPARE backup-restored TABLES volatile sessions: left 4, right 4 (one row per login, deleted at expiry: the comparison itself logs in)
PASS COMPARE backup-restored TABLES: the row counts of 47 tables outside the volatile list are equal
PASS COMPARE backup-restored GOOSE: the levels are equal (60) and so are the goose rows (61)
PASS COMPARE backup-restored PAGES: equal (8 lines)
PASS COMPARE backup-restored USERS: equal (2 lines)
PASS COMPARE backup-restored MEDIA: equal (12 lines)
TAMPER-CONTROL: page hash changed -> 1 BAD line(s) for PAGES, user role changed -> 1 BAD line(s) for USERS
PASS the comparison fails when altered: a changed page hash and a changed user role are both detected
RESTORED-MEDIA: rows 12 files 12 rows-without-file 0 files-without-row 0 class AGREE
PASS media rows and media files of the restored box agree in both directions
PASS holzcloud check on a copy of the restored database: exit 0
PASS PRAGMA integrity_check on the restored database: ok
PASS PRAGMA foreign_key_check on the restored database: no violation
PASS the sha256 list of the restored data directory (25 files) equals that of the backup directory it came from
SET-ASIDE: /opt/holzcloud/data.before-restore-20261027-083126 users 0
PASS the restored box was fresh: the setup form was 200 and the set-aside database is the empty one of the first start (users 0)
PRE-UPGRADE-SNAPSHOT-AFTER: 0
INFO a pre-upgrade snapshot appears in the data directory when the service finds a migration pending at its start; this pair may have none pending, so the number says nothing about how a restore handles one
SETUP-AFTER: HTTP 404 -
PASS the setup form answers 404 after the restore (the restored database has users)
LOGIN-ADMIN: HTTP 303 /admin/
LOGIN-EDITOR: HTTP 303 /admin/
PASS both accounts of the previous release log in on the restored box: HTTP 303 /admin/
PLUGINTEST: HTTP 200
PASS /plugintest answers 200 for test.local on the restored box
MEDIA-SERVED: 1/756be0ef804be7d0e566f832b417396e.jpg source=bfb2e4047996ebab6d05022cffbcea34e8da7dd5f153b9690a76a9948d5dc5ee served=bfb2e4047996ebab6d05022cffbcea34e8da7dd5f153b9690a76a9948d5dc5ee
MEDIA-SERVED: 1/9bd665a289832b777b908171a9741e94.jpg source=b3cc58d2e5fc4ae1070730121e52d46557da99b79fad292531a55a797c256004 served=b3cc58d2e5fc4ae1070730121e52d46557da99b79fad292531a55a797c256004
MEDIA-SERVED: 1/ac8312aaffe6ac9f1eb6196afd09696b.jpg source=925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 served=925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
MEDIA-SERVED: 1/ca6d29ea18794165c6dacc11d4c9c55c.jpg source=5d968f7b02e61ceb758d23346f0ec2376f4766edfab43e034386fb43ff8d3fe5 served=5d968f7b02e61ceb758d23346f0ec2376f4766edfab43e034386fb43ff8d3fe5
MEDIA-SERVED-EQUAL: 4 of 4
PASS every media original is served over HTTP with the sha256 of the source's file (4 of 4)
MIGRATIONS-AT-TAG: 60
MIGRATIONS-HIGHEST-AT-TAG: 60
MIGRATIONS-AT-NEW-REF: 60
MIGRATIONS-HIGHEST-AT-NEW-REF: 60
LEVEL-BEFORE: 60
LEVEL-AFTER: 60
PASS migration_ok: before 60, after 60, highest at dd077e4 60
MIGRATION-SENTENCE: No migration separates the two releases: the backup is at level 60, the restored database is at level 60, the restore ran none.
GOOSE-ROWS-BEFORE: 61
GOOSE-ROWS-AFTER: 61
INFO this pair crosses 0 migration(s) by the levels read; a pair with no pending migration does not show how a restore handles one, that stays on the Not tested list of the record and migration_ok's table is what keeps the judgement right when a later release does cross one
COMPAT-DIFF-COMMAND: git diff --stat=200 v0.1.0 dd077e4 -- go.mod go.sum internal/db internal/media cmd
COMPAT-DIFF  cmd/holzcloud/assets/admin.css                       |  18 +++
COMPAT-DIFF  cmd/holzcloud/assets/bausteine.css                   |  18 ++-
COMPAT-DIFF  cmd/holzcloud/main.go                                |  21 +++
COMPAT-DIFF  cmd/holzcloud/main_test.go                           |   2 +
COMPAT-DIFF  cmd/holzcloud/templates/admin/block_list.html        |   8 +-
COMPAT-DIFF  cmd/holzcloud/templates/admin/media_scale.html       |  41 ++++++
COMPAT-DIFF  cmd/holzcloud/templates/public/holzcloud/layout.html |   2 +-
COMPAT-DIFF  cmd/holzcloud/templates/public/holzcloud/style.css   |  33 +++--
COMPAT-DIFF  cmd/holzcloud/templates/public/rudel/style.css       |   6 +-
COMPAT-DIFF  cmd/holzcloud/templates/public/weide/style.css       |   5 +-
COMPAT-DIFF  go.mod                                               |  22 ++--
COMPAT-DIFF  go.sum                                               |  60 ++++-----
COMPAT-DIFF  internal/media/crop.go                               |   9 +-
COMPAT-DIFF  internal/media/orient.go                             | 343 +++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/orient_keep_test.go                   | 983 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/orient_test.go                        | 640 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/profile.go                            | 241 ++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/profile_test.go                       | 301 +++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/scale.go                              | 213 ++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/scale_test.go                         | 248 +++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/staging.go                            | 219 +++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/staging_test.go                       | 167 ++++++++++++++++++++++++
COMPAT-DIFF  internal/media/strip.go                              |  60 ++++++++-
COMPAT-DIFF  internal/media/variants.go                           |  45 ++++---
COMPAT-DIFF  24 files changed, 3626 insertions(+), 79 deletions(-)
SCRIPTS-IDENTICAL: deploy/backup.sh no tag=d9f807d806a61ef3a9920890394247b3ecd475a17b92de4929ad517e2c17eba2 new-ref=ac02fac9096d3ed1665794c87a1624f8c896bb980ae615d80be0d4bab7d8de96
SCRIPTS-IDENTICAL: deploy/restore.sh yes 9a411d72ca2c3c8dcb081915af91c46f743de7f79373296d8cc4bdf905a59633
SCRIPTS-IDENTICAL: deploy/holzcloud-backup.service yes befdd74bd8233c7922755824bdea6b60aa0bb0beb0c83ba0982562cdc7e93bf7
SCRIPTS-IDENTICAL: deploy/holzcloud-backup.timer yes 87ccb6d9ec499469994b1ef0aa020bb6e3a9df140dd71b19169f4193ff9e88b0
CHECK: 17 of 17
CHECK-BAD-LINES: 0
PASS run.sh check on the restored box: 17 of 17 assertions, the five plugins loaded, running and invoked
COMPAT OK
```

#### amd64-t29-old/compat-binary.txt

```text
== old: v0.1.0 ==
file:       deploy/test/out/bin/holzcloud-old-v0.1.0
describe:   v0.1.0
commit:     4d238b9
sha256:     83d5ba08ad2bde11b65c02271a13a867082c0071c81cd7b69bdf213f384ae2d3
migrations: 60
container-version: holzcloud v0.1.0 (4d238b9)
```

#### amd64-t29-new/transfer.txt

```text
DIR: /opt/holzcloud/backups/20261027-083052
SOURCE: holzcloud-t29-old:/opt/holzcloud/backups/20261027-083052
FILES: 27
TRANSFER: equal
```
