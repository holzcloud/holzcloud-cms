# Backup and restore, measured: results

This document is the permanent record of phase 22. It records what happened when a backup made by the nightly timer's service was restored by `deploy/restore.sh` into a different, empty container: what came back, what the backup and the restore cost, what that cannot say about a real server, and which defects the first pass found and how they were fixed. Nothing was tuned to look better: a defect is fixed in `deploy/backup.sh`, `deploy/restore.sh` or `deploy/DEPLOY.md` and listed here with the output that showed it, a deviation is only ever a thing the container cannot do, and a number is quoted as the harness wrote it.

The record rests on one evidence set: a single `backup-chain` run from fresh containers, made once against the committed final scripts, units and `deploy/DEPLOY.md` of the evidence commit (see *How this was run*). Its raw output is the Appendix, pasted unchanged and proven block by block against the scratch files. The Findings quote the output of the first pass, made before the repair; that output showed the defects and is not pasted again.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| BAK-01 | PASS. A backup of the database and the media, made by hand and again by the timer, is whole: `verify-backup.txt` and `verify-timer.txt` each hold 57 PASS lines and no BAD line. Two readers agree the database is intact (`holzcloud check` on a copy, `sqlite3` `PRAGMA integrity_check` on the immutable file), the whole-directory sha256 comparison finds all 16 files equal (media, templates, plugins, language file, logo and the CSRF key), and 47 row counts are equal. The first pass found the backup incomplete; the repair is in the scripts. | linux/amd64 emulated on an aarch64 host (Docker Desktop), Debian 12 (bookworm), systemd 252 (252.39-1~deb12u2) | 2026-10-14, build v0.0.25-68-gbf74496 |
| BAK-02 | PASS. The directory the timer made was restored by the document's own restore line into a different, empty container: pages and the marker page answer, the media file is served with the same sha256, the login with the old password works (303 to `/admin/`), five plugins run, `check` passes 17 of 17 assertions, the setup form answers 200 before and 404 after, `restore.txt` shows 17 PASS and no BAD, the state comparison 53 PASS and no BAD. Four refusals (damaged database, no database, answer no, too little disk) leave the live data untouched. Restore wall time 8.126 s with a 20 MB backup and 8.296 s with 300 MiB of added media. | linux/amd64 emulated on an aarch64 host (Docker Desktop), Debian 12 (bookworm), systemd 252 (252.39-1~deb12u2) | 2026-10-14, build v0.0.25-68-gbf74496 |
| BAK-03 | PASS. One run through `holzcloud-backup.timer`: the service was started by the timer (`TriggeredBy=holzcloud-backup.timer`), `Result=success`, the new directory belongs to the service user with mode 700, the journal of that one invocation ends in `=== Backup complete ===`, `NEXT` moved a day on and `LAST` was set. The same two-reader and whole-directory proof was applied to that directory, and it is the directory that was then restored. The real 03:15 window, `Persistent=true` and `RandomizedDelaySec` were not exercised. | linux/amd64 emulated on an aarch64 host (Docker Desktop), Debian 12 (bookworm), systemd 252 (252.39-1~deb12u2) | 2026-10-14, build v0.0.25-68-gbf74496 |

Evidence commit: bf74496

The binary under test is the one built from `NEW_REF` (pinned by the chain to `bf74496`) with the Build line of `deploy/DEPLOY.md`: `NEW: v0.0.25-68-gbf74496 bf74496` in `update.txt` of the source, `holzcloud v0.0.25-68-gbf74496 (bf74496)` in the `MANIFEST` of every backup and in `install.txt` of the target. The old binary used to populate the source is v0.0.10, as in phase 21.

## How this was run

Every command comes out of `deploy/DEPLOY.md` itself: `deploy/test/run.sh` reads the fenced bash blocks of a section and refuses a line the document does not contain, so the play cannot drift from the text. The Backups block and the Restore from a backup block are played through that mechanism; what the container cannot do the way a server does is a named deviation, below.

Pass 1, the chain against the scripts and the document exactly as they stood when the phase began (commit 5de2b17), nothing fixed; it exited 1 with FAIL verdicts after writing `summary.txt`, 249 seconds, build `v0.0.25-63-gb687738`:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=b687738 \
  NAME=holzcloud-bak-pass1-src OUT_DIR=deploy/test/out/amd64-bak-pass1-src \
  TGT_NAME=holzcloud-bak-pass1-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-pass1-tgt \
  ./deploy/test/run.sh backup-chain      # BULK_MB left at the chain's default of 300
```

Final set, after the repair, from fresh containers (one source, the seed-point target and the bulk-point target), `BULK_MB` left at the chain's default of 300:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=bf74496 \
  NAME=holzcloud-bak-src OUT_DIR=deploy/test/out/amd64-bak-src \
  TGT_NAME=holzcloud-bak-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-tgt \
  ./deploy/test/run.sh backup-chain      # BULK_MB left at the chain's default of 300
```

The final chain ran for 238 seconds (the log was created at 12:59:29 and last written at 13:03:27 on the host clock; the image and the Go build were cached) and ended `CHAIN_EXIT=0`. The final set was made against the committed final `deploy/DEPLOY.md`, `deploy/backup.sh`, `deploy/restore.sh`, the two backup units and `deploy/test/run.sh` of the evidence commit, from fresh containers, and none of those files has been edited since (`git diff` between the evidence commit and the commit of this record is empty for them). An earlier complete final set, made on commit e4c3eb8, was thrown away: it showed a restore of about seven seconds against the two seconds `DEPLOY.md` still quoted from pass 1, more than a factor of two, so the document was corrected and the whole set made again. Only the set of the evidence commit is pasted here.

The pass-1 output is quoted in *Findings* and is not pasted again. Four questions that pass 1 could not answer from its own files were asked in an investigation run (the chain played again with `BULK_MB=5` and the target kept, questions E1 to E5, `amd64-bak-inv3-investigation.txt`); it is not part of the evidence set and is quoted only where a finding says so.

One fact about the transcript: `read -p` prints no prompt when stdin is not a terminal, so the `[yes/NO]` question does not appear in `restore-run.txt` although `yes` was typed on stdin and taken; the document's wording about the question comes from the script, and case c of the refusals shows the script does ask (it prints `aborted` on `no`).

## Deviations named

Everything here precedes every result: it is what the container does differently from a real server and what the harness did that the document does not contain. The first column is the name written to `deviations.txt` and `scenario.txt` of the run.

| Name | Why | What a real server does instead |
|------|-----|---------------------------------|
| DEVIATION[transfer] | There are two containers and no network between them. The backup crosses as a tar stream through the host (`docker cp SOURCE:dir -` into `docker cp - TARGET:/opt/holzcloud/backups`), not through a host directory. The proof of a lossless transfer is the sha256 of every file on both sides (`TRANSFER: equal`, 18 and 78 files). | The operator copies the backup directory to the rebuilt machine with `rsync -a` or `scp -r`, which the document names and leaves to the operator. |
| DEVIATION[timer] | The real schedule is 03:15 and a test cannot wait for it. A container-only drop-in on the timer resets `OnCalendar` to an empty value, sets it to a time 60 seconds ahead of the container's clock, and sets `RandomizedDelaySec=0` and `AccuracySec=1s`. The shipped timer and service are untouched, the clock is not changed, only `systemctl daemon-reload` was run. | The timer fires at 03:15 plus up to 300 seconds of random delay, and `Persistent=true` runs a missed window after the machine was off. None of that was observed here. |
| DEVIATION[backup-name] | The document's restore line carries a sample timestamp (`/opt/holzcloud/backups/20260802-031500`); the document cannot know the name a backup was given. The last word was replaced by the real directory the timer made (`/opt/holzcloud/backups/20261014-110152`), the rest of the line is as written and the answer `yes` is typed on stdin. | The operator types the name of the backup from the listing the first line of the block prints. |
| DEVIATION[fixture] | The login password file, the AI key file and the test image stand in for what an operator remembers; none is part of any backup, and they cross from container to container by exec-pipe, never through a host file. The plugin archives, the order row and the login user of phase 21 stand in for operator work in the admin. A language file (`data/sprachen/nl.json`), a logo (`data/marke/logo.svg`) and, in the bulk run, 300 MiB of synthetic media (random bytes outside the database) are planted so that the data directory holds every directory a backup must carry and the copy time can be read. | The operator's real content, an administrator who knows the password, a key kept in a password manager. |
| DEVIATION[scp] | The document's two `scp` lines (the binary and the deploy directory) have no ssh and no second machine here; they were played as `docker cp` of the binary and as a tar stream of the working tree's `deploy` directory without `deploy/test/out`. | `scp` from the build machine to the server. |
| DEVIATION[dropin] | The shipped `holzcloud.service` crashes at its first start under the amd64 emulation (`ProcSubset=pid`, see `unit-start-unchanged.txt` of the run). The container-only drop-in `/etc/systemd/system/holzcloud.service.d/container.conf` sets `ProcSubset=all`. | The unit is expected to start as shipped on a native kernel; phase 20 saw it start unchanged on native arm64. |
| DEVIATION[mount] | The unit's `RequiresMountsFor` names `/opt/holzcloud/data`, a plain directory on the container's root filesystem here; no separate mount exists, so that line was not exercised. | The data directory may be its own volume that must be mounted before the service starts, and a restore touches it with the service stopped. |
| amd64 emulation | Not a `DEVIATION[...]` line of its own: `versions.txt` and `backup.txt` of the run show it. The host architecture is `aarch64` (Docker Desktop, kernel `7.0.14-linuxkit`) and the container's `uname` is `x86_64`, so every container ran under amd64 emulation. Every time in this record is an emulation time, with a warm page cache, on one filesystem for source and backup. | A native x86-64 machine, its own disk and CPU. No native amd64 run exists. |

Four steps that the harness performs and the document does not contain (`scenario.txt` of the target):

- SCENARIO[restore-damaged]: `restore.sh` run on a copy of the transferred backup whose database is truncated to half its size, with `yes` on stdin on purpose, to see that it is refused before the live data is touched. The document names the script, not this directory or this damage.
- SCENARIO[restore-no-sqlite]: `restore.sh` run on a directory that holds only a copy of `MANIFEST`, with `yes` on stdin.
- SCENARIO[restore-answer-no]: `restore.sh` run on the intact backup with `no` on stdin, to see that it aborts before the service is stopped.
- SCENARIO[backup-low-disk]: `backup.sh` run as the service user with `MIN_FREE_MB=99999999`, a threshold no disk has, to see that it exits 1 and makes no directory. The document names the script and its default; the value is new.

## Findings

Pass 1 found eleven defects, F-01 to F-11, each listed with the pass-1 output that showed it and the change that fixed it, visible in the current files. `What happened` quotes the scratch output of pass 1 (the investigation run where a row says so) in backticks, verbatim; `The script or document said` quotes the text as it stood at commit 5de2b17, which no longer exists in the repaired files; `Fix` points at the repaired text, which does exist. The set of the evidence commit found no defect that is not in this table. No row was added by it.

| ID | Step | The script or document said | What happened | Fix |
|----|------|-----------------------------|---------------|-----|
| F-01 | `deploy/backup.sh`, steps 1 to 4 (database, key, media, templates) | The script copied those four and nothing else. DEPLOY.md, Backups: `containing the database, the CSRF` and `key, media and user templates, plus a` manifest. | The whole-directory comparison finds the plugin tree, the language directory and the logo only in the source, twelve lines in each of `verify-backup.txt` and `verify-timer.txt` of the seed point and of `verify-backup.txt` of the bulk point: `BAD  tree src-backup: only in A: ./plugins/bestellung/plugin.wasm`, `BAD  tree src-backup: only in A: ./sprachen/nl.json`, `BAD  tree src-backup: only in A: ./marke/logo.svg`. | One loop over the five names in `deploy/backup.sh`: `for name in media templates plugins sprachen marke; do`, each guarded by a directory test and announced. The final set: `PASS tree src-backup: 16 files equal`. |
| F-02 | `deploy/restore.sh`, the media and templates steps | DEPLOY.md, Restore from a backup: `database, CSRF key, media and templates, and waits for` the readiness. | The restore brings back what the backup holds and no more: `COMES-BACK: csrf.key holzcloud.sqlite media (the top-level entries of the backup directory that are in the restored data directory)`, and the comparison of source against restored target shows the same three: `BAD  tree src-restored: only in A: ./sprachen/nl.json` (twelve lines in each target). | The same loop in `deploy/restore.sh`: `for name in media templates plugins sprachen marke; do`. The final set: `COMES-BACK: csrf.key holzcloud.sqlite marke media plugins sprachen (the top-level entries of the backup directory that are in the restored data directory)` and `PASS tree src-restored: 16 files equal`. |
| F-03 | The restored site: readiness, `compare-src-tgt.txt`, `check.txt` | DEPLOY.md, Restore from a backup: `database, CSRF key, media and templates, and waits for` the readiness, which was taken as the sign of a restored site. | `/readyz` is green while the plugins are down: `READYZ: {"status":"ok","version":"v0.0.25-63-gb687738","commit":"b687738","integrity":"ok"}`, then `BAD plugins running a=5 b=0 (5 expected)` and `BAD  check: 1 of 17 assertions passed (exit status 1), see check.txt; the first BAD line is: loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC`. The plugin rows came back, enabled, with nothing to run: `"enabled":true,"running":false,"last_error":"the module is missing from disk: open /opt/holzcloud/data/plugins/bestellung/plugin.wasm: no such file or directory"` (investigation run, E1). | No change of its own; it disappears with F-01 and F-02. The final set shows `PASS plugins running 5/5` in `compare-src-tgt.txt` and `PASS check: 17 of 17 assertions passed (check.txt)` in `restore.txt`. Readiness does not look at plugins and was not changed. |
| F-04 | `docs/deployment.md`, Backups, and `docs/configuration.md`, the data directory | `Back up the whole data directory;` and `Back up this whole directory and you have everything.` | The script did not do it (F-01). The source data directory of pass 1 holds `./plugins/bestellung/plugin.json` and a logo directory, and the tree in `docs/configuration.md` listed neither `plugins/` nor `marke/`, so a reader would not know the directories exist. | Both documents now say what the script carries (the database, `csrf.key` and the five directories), what it leaves out (the pre-upgrade snapshots and everything outside the data directory), and the tree lists `plugins/` and `marke/`. |
| F-05 | `deploy/DEPLOY.md`, Backups, opening paragraph | `key, media and user templates, plus a` manifest, and nothing on what a backup does not hold or on the order of the two copies. | True of the script and incomplete for the site (F-01). Pass 1 itself printed the list of what no backup holds: `HAND-REBUILT: no environment file is configured on this unit; on a real server HOLZCLOUD_SECRET_KEY and payment or mail credentials live in one, never in the data directory, so no backup holds them`. | DEPLOY.md Backups lists the five directories and the `MANIFEST` and carries `**What a backup does not hold.**` and `**The pair is not atomic.**`, the second saying that the database snapshot is taken first and the files afterwards. |
| F-06 | `deploy/DEPLOY.md`, Restore from a backup | One command, and the paragraph `database, CSRF key, media and templates, and waits for` the readiness. | The document did not mention the question the script asks, which anything but `yes` answers with `aborted` and exit 1 (`INFO case c output: aborted`), nor that a freshly installed box has its setup form open until the restore replaces the database (`SETUP-BEFORE: HTTP 200 -`, then `SETUP-AFTER: HTTP 404 -`), nor what comes back and what is rebuilt by hand, nor how a backup reaches a rebuilt box (`DEVIATION[transfer]`). | Restore from a backup now gives the command, the `yes` question, the five steps in order, how the backup reaches a rebuilt box, the warning that `restore replaces the database anyway` so no account is to be made in the setup form, what comes back, what is rebuilt by hand, and the measured time as an order of magnitude. |
| F-07 | `deploy/restore.sh`, the `read -r -p` question, `restore-run.txt` | The script asks `Stop holzcloud and replace /opt/holzcloud/data? [yes/NO]`. | The question is absent from the transcript: `1791974064.218289542 Verifying snapshot...` is followed by `1791974064.303775543 Stopping holzcloud...` with nothing between, although `yes` was on stdin and the answer was taken. A harness fact, not a defect of the script or the document: `read -p` prints no prompt when stdin is not a terminal, and case c shows the script does ask. | None. Disposition: no action. The harness README says so under the lessons of phase 22. |
| F-08 | `deploy/restore.sh`, the copy of the database | The restore keeps modes for the key and the media (`cp -p`, `rsync -a`); the database was copied with a plain `cp`. | `INFO owner and mode of holzcloud.sqlite: source 'holzcloud 600', restored 'holzcloud 644' (the data directory is 700)`. The directory mode contains it, so nothing was exposed, but the restored file was wider than the original. | One line in `deploy/restore.sh` after the copy: `chmod 600 "${DATA_DIR}/holzcloud.sqlite"`. The final set: `PASS owner and mode of /opt/holzcloud/data/holzcloud.sqlite: holzcloud:holzcloud 600`. |
| F-09 | `deploy/restore.sh`, the move-aside | DEPLOY.md: the previous state is still in `/opt/holzcloud/data.before-restore-<timestamp>`. | Each restore leaves one more whole copy of the replaced data, media included, and nothing removes it (investigation run, E4): `/opt/holzcloud/data.before-restore-20261014-104201: csrf.key holzcloud.sqlite media plugins` next to two more of the same kind. The document named the directory and not that it stays until the operator deletes it. | DEPLOY.md says `The set-aside directory is never removed.`, that it holds the replaced site with its media, and that it is deleted by hand once the restored site is accepted; the header of `deploy/restore.sh` says the same. |
| F-10 | `deploy/restore.sh`, the precondition on the backup directory | The script refused a snapshot that failed its integrity check or had no database, and read the manifest only if there was one: `[ -f "${BACKUP_DIR}/MANIFEST" ] && cat "${BACKUP_DIR}/MANIFEST"`. | A backup interrupted during the media copy has the database, the CSRF key, part of the media and no `MANIFEST` (`backup.sh` writes it last). `restore.sh` took it (investigation run, E3b): `restore.sh: === Restore complete ===`, exit status 0, and `media files in the live data after: 2` where there were 3. Nothing was refused and a file was gone. | `deploy/restore.sh` refuses a directory without a `MANIFEST`: `ERROR: ${BACKUP_DIR}/MANIFEST not found — the backup is incomplete, do not restore it`. No new field and no digest file. The four refusals of the final set pass with it in place. |
| F-11 | `deploy/test/README.md`, the output files, against `deploy/test/run.sh` | The README said: `A name outside the documented set fails the run` | The harness has no such check; the only readers of the names are the summary's `DEVIATIONS:` list and the chain's own distinct-kinds loop. Found in plan 22-02 and carried as a deferred item. | The README sentence now says what the harness does: it does not check the names, the chain's summary lists the distinct kinds and whoever reads the evidence compares that list with the documented set. |

Read from the scripts and not played, so not a row: DEPLOY.md at the start of the phase said to run a restore once on a test directory (`Do it once on a test directory`). `restore.sh` takes `DATA_DIR` from the environment but runs `systemctl stop` and `systemctl start` on the production unit and polls the production `/readyz`, so a restore pointed at a test directory stops the live site and proves nothing about the restored copy. DEPLOY.md now says to run it once on a spare machine, which is what this phase did.

Also looked at and found to be no defect: the `-wal` and `-shm` of the replaced database (the whole data directory moves aside, the set-aside holds none and the restored file passes every row comparison and the integrity verdict); the ownership of the restored tree (right, the one mode was F-08); the `MIN_FREE_MB` floor against the container's disk (it never trips on a 354 GB emulated disk and refuses an impossible threshold, case d, but it is a fixed floor that does not grow with the data, which DEPLOY.md now says); the non-atomic pair (a fact of the order of the steps, quiet in these runs, not provoked, now in the script headers and in DEPLOY.md). A flipped byte inside a data page of a complete backup is not detected by either integrity reader (investigation run, E2); only truncation is. That limit is outside this phase by decision and DEPLOY.md does not claim otherwise.

## BAK-01: a verified backup of the database and the media

What was asserted: the database of a backup is intact by two independent readers, and the content of the backup equals the content of the source. Both for the backup made by hand (`verify-backup.txt`) and for the one the timer made (`verify-timer.txt`), and for the 300 MiB point (`verify-backup.txt` of the bulk directory).

How:

- **Integrity, two readers.** `PASS integrity holzcloud-check: ok` is `holzcloud check` run on a copy of the snapshot (the way `restore.sh` runs it, because the binary opens the database read-write); it runs `quick_check`. `PASS integrity sqlite3 integrity_check: ok` is `sqlite3` with `PRAGMA integrity_check` on the file opened as `file:PATH?immutable=1`, a separate implementation and the stronger pragma, which also never creates `-wal` and `-shm` files next to a backup.
- **Content, by comparison.** The sha256 of every file below the data directory is listed for the source, the backup and the restored directory, except the SQLite family (a snapshot is a different file from the live one by design), `pre-upgrade-*.sqlite*` (deliberately not part of a backup) and `MANIFEST`; the lists are compared as a whole. The source list is asserted non-empty and to hold a file below `./media/`.
- **The `MANIFEST`** names the version (`PASS MANIFEST version=holzcloud v0.0.25-68-gbf74496 (bf74496)`), the data directory, the host and the timestamp, and the backup directory has `holzcloud 700` as owner and mode.
- **Row counts.** One count per table of the backup database against the source: 47 `PASS rows` lines. Seven tables move with traffic and are reported as `INFO volatile` rather than compared (`activity_log`, `goose_db_version`, `mail_outbox`, `oauth_codes`, `outbox`, `plugin_store`, `sessions`); the two that differ differ by one row: `INFO volatile activity_log backup=22 source=21` and `INFO volatile sessions backup=4 source=3`. All 54 tables are counted (`rows-backup.txt`).

Result in numbers: seed point `verify-backup.txt` 57 PASS, 0 BAD; timer-made backup `verify-timer.txt` 57 PASS, 0 BAD; bulk point `verify-backup.txt` 57 PASS, 0 BAD (`PASS tree src-backup: 76 files equal`). Key lines: `PASS tree src-backup: 16 files equal`, `PASS tree src-timer: 16 files equal`, `INFO MEDIA-SERVED equals the staged source image (/tmp/hc-upload.jpg)`.

How the comparison was shown able to fail (`selftest-tree.txt` and `compare-selftest.txt`, pasted below): the whole-directory comparison is run against deliberately altered copies of the list and must say BAD for each: an altered hash (`BAD  tree selftest-altered: differs: ./sprachen/nl.json`), a missing file (`BAD  tree selftest-altered: only in A: ./csrf.key`), an empty reference list (`BAD  tree selftest-empty: the reference list .tree-empty.txt is empty, which proves nothing`) and a list with no media file (`BAD  tree selftest-nomedia: the reference list .tree-nomedia.txt has no file below ./media/, so an empty media directory would pass`); the file ends in `SELFTEST OK`. The state comparison is shown the same way: a state file altered by one page row and a changed page hash gives `BAD rows pages a=5 b=6` and `BAD page sha256` (`compare-selftest.txt` of both machines). Those BAD lines are expected and are not findings; the real comparisons carry none.

What this does not show: the integrity readers do not find a flipped byte inside a data page of an otherwise well-formed database (see *Findings*, last paragraph), so BAK-01 is a statement about truncation, missing files and differing content, not about bit rot.

## BAK-02: a restore onto an empty system

What was asserted: a backup restores into a different machine that holds nothing of the site, and the site that comes up is the one that was backed up. The target `holzcloud-bak-tgt` is a second fresh container installed with the document's own steps 1 to 19 (`steps.txt` of the target; `install.txt`: `readyz: {"status":"ok",...}`, `unit: active`), so its first start has made an empty database with an open setup form. The directory the timer made reached it by the tar stream (`TRANSFER: equal`), and the restore line of the document was played with `yes` on stdin.

Result in numbers, seed point (`restore.txt`): 17 PASS, 0 BAD. Key lines:

- Before and after: `SETUP-BEFORE: HTTP 200 -` and `PASS setup form open before the restore (the empty database of the first start)`, then `SETUP-AFTER: HTTP 404 -` and `PASS setup form closed after the restore (the restored database has users)`.
- Pages and media: `PASS marker page: GET /after-update answers 200 with its sentence`, `PASS media served over HTTP: same status and sha256 as in the source (HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577)`; `public: plugintest=200 root=200 title=yes` in `state-tgt.txt`.
- Login: `LOGIN: HTTP 303 /admin/` and `PASS login with the password the user had before: 303 to /admin/`.
- Plugins: `PASS plugins running 5/5` in `compare-src-tgt.txt` and `PASS check: 17 of 17 assertions passed (check.txt)`; `check.txt` holds 17 PASS lines and the five `plugin loaded` journal lines.
- Integrity: `PASS readyz status ok, integrity ok`.
- Owner and mode: `PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700`, the database `holzcloud:holzcloud 600`, `csrf.key` `holzcloud:holzcloud 600`, a media file `holzcloud:holzcloud 600`.
- The set-aside: `PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261014-110158` and `PASS set-aside database is the empty one of the first start (users: 0)`: the first start's empty database went aside whole, with its `-wal` and `-shm`.
- Per-file comparison: `PASS tree src-restored: 16 files equal` and `PASS tree backup-restored: 16 files equal` (the source, the backup and the restored directory are the same 16 files with the same sha256).
- State comparison (`compare-src-tgt.txt`): 53 PASS, 0 BAD: 47 row counts, the page hash, the media hash, login, the public page, five plugins running, the order token. The tables that move with traffic differ, as expected: `INFO volatile activity_log a=21 b=23`, `INFO volatile sessions a=3 b=5`.

The same for the 300 MiB point (`restore.txt` of the bulk target): 17 PASS, 0 BAD, `PASS tree src-restored: 76 files equal`, `PASS media served over HTTP: same status and sha256 as in the source (HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577)`, 53 PASS and 0 BAD in the state comparison.

The four refusals (`restore-negative.txt`, 42 PASS, 0 BAD), each played on the restored site as live data. Before and after every case eight invariants of the live data are taken and must be equal: the inode of `/opt/holzcloud/data`, the sha256 of every file below it (except the SQLite family and `MANIFEST`), the service's `InvocationID` and `ActiveEnterTimestamp`, the number of set-aside directories, `systemctl is-active`, the `/readyz` status and `PRAGMA integrity_check` of the live database. All eight held in every case (`PASS case a: invariant tree unchanged (6e171cc9b70e4cd8043cb758e17bdeee3d62e768835f9dc48653d322b49b7150)` is the same value in the four).

| Case | Played | Result |
|------|--------|--------|
| a, restore-damaged | The database of a copy truncated from 663552 to 331776 bytes; the damage was first proven real (`INFO case a: DAMAGE-PROVEN truncated`) | exit 1, `ERROR: the snapshot fails its integrity check — do not restore it`; eight invariants unchanged |
| b, restore-no-sqlite | A directory with only a `MANIFEST` | exit 1, `ERROR: /opt/holzcloud/backups/negcase-empty/holzcloud.sqlite not found`; eight invariants unchanged |
| c, restore-answer-no | The intact backup, `no` on stdin | exit 1, `aborted` after `integrity: ok` and before the service is stopped; eight invariants unchanged |
| d, backup-low-disk | `backup.sh` with `MIN_FREE_MB=99999999` | exit 1, `ERROR: only 354286 MB free at /opt/holzcloud/backups, need 99999999 MB`; `PASS case d: the number of directories under /opt/holzcloud/backups is unchanged (1): no directory was made` |

How the state comparison was shown able to fail: `compare-selftest.txt` of the target, an altered copy of the target's state, gives `BAD rows pages a=5 b=6` and `BAD page sha256`, and ends `# SELFTEST OK`.

What this does not show: a restore onto a real, rebuilt server (see *Not tested*), a restore of a backup made by an older binary in the final set, and recovery from bit rot in a complete backup.

## BAK-03: one run through the timer

What was asserted: the nightly backup runs when the timer fires, as the hardened service, and the directory it makes is as good as the one made by hand and is restorable. The timer was changed only by a container-only drop-in (DEVIATION[timer]); the service unit is the shipped one.

Result in numbers (`timer.txt`): 9 PASS, 0 BAD. The evidence, each from the raw files:

- **Before and after.** `timer-before.txt`: `Wed 2026-10-14 11:01:52 UTC 59s left -    -      holzcloud-backup.timer holzcloud-backup.service` (NEXT set, LAST empty). `timer-after.txt`: `Thu 2026-10-15 11:01:52 UTC 23h left Wed 2026-10-14 11:01:52 UTC 921ms ago holzcloud-backup.timer holzcloud-backup.service` (NEXT moved a day on, LAST set). `PASS NEXT moved from 'Wed 2026-10-14 11:01:52 UTC' to 'Thu 2026-10-15 11:01:52 UTC'`.
- **The journal of that one invocation** (`timer-journal.txt`, the unit `holzcloud-backup.service`): it starts with `Starting holzcloud-backup.service - holzcloud-CMS backup...`, walks the script's steps (database, CSRF key, media, plugins, sprachen, marke, pruning) and ends with `=== Backup complete: /opt/holzcloud/backups/20261014-110152 ===` and `Finished holzcloud-backup.service - holzcloud-CMS backup.`
- **Started by the timer.** `TRIGGERED-BY: holzcloud-backup.timer`, and `PASS the service was started by holzcloud-backup.timer (TriggeredBy), not by hand`; `RESULT: success`, `EXIT-STATUS: 0`, `PASS Result success and ExecMainStatus 0`.
- **The directory and its owner.** `PASS owner and mode of the timer's backup directory: holzcloud 700 (the hardened unit runs as the service user)`; `PASS the timer made its own directory, not the manual run's (/opt/holzcloud/backups/20261014-110048)`. The directory is the one the journal names, not the newest one: the backups directory already held the manual backup of the chain and the backups of the Updating play.
- **The hardening, as far as it was exercised.** The service ran as `User=holzcloud` under its shipped `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `ReadWritePaths` of the data and backups directories and `PrivateTmp`, read the data directory, wrote the backups directory, and finished with `Result=success`. That the directives were enforced and not only not in the way was not probed separately (no write outside the allowed paths was attempted), and the seccomp filter of the main unit is not enforced under this emulation at all (`Seccomp:	0` in `check.txt`).
- **The same proof applied to the directory the timer made.** `verify-timer.txt`: 57 PASS, 0 BAD, two integrity readers `ok`, `PASS tree src-timer: 16 files equal`, 47 row counts equal. That directory was the one transferred and restored in BAK-02 (`DIR: /opt/holzcloud/backups/20261014-110152` in `transfer.txt` and `restore.txt`).

What this does not show, in *Not tested*: the real 03:15 window, `Persistent=true` after a missed run, `RandomizedDelaySec`, the off-site `REMOTE_TARGET` copy, retention pruning.

## Measurements

Two data points from the same chain: the seed site as it is (a small site with five plugins, one image and a few pages) and the same site with 300 MiB of synthetic media added before the backup (`bulk.txt`: `BULK-MB: 300`, 60 files, `BYTES: 314576896`). Every value is quoted as the harness wrote it, from the `POINT` lines of `summary.txt` and the KEY lines of `backup.txt` and `restore.txt`; none is rounded.

| | Seed point | Bulk point (300 MiB added) |
|---|---|---|
| Backup size, `du -sb` bytes (`BACKUP-BYTES`) | 20548955 | 335125851 |
| Backup size, human (`SIZE`) | 20M | 320M |
| Database bytes (`DB-BYTES`) | 663552 | 663552 |
| Media bytes (`MEDIA-BYTES`) | 39983 | 314612783 |
| Files in the backup, of which media (`FILES`, `MEDIA-FILES`) | 18, 3 | 78, 63 |
| Backup wall seconds (`BACKUP-WALL-S`) | 0.692 | 1.015 |
| Restore wall seconds (`RESTORE-WALL-S`) | 8.126 | 8.296 |
| Restore, verify (`VERIFY-S`) | 0.083 | 0.088 |
| Restore, stop and move aside and copy (`STOP-MOVE-COPY-S`) | 0.455 | 0.631 |
| Restore, start to ready (`START-READY-S`) | 7.397 | 7.379 |
| Host and container architecture | aarch64, x86_64 | aarch64, x86_64 |

The seed backup is 20 MB, not under one, because the installed plugin modules are in it: the five `plugin.wasm` files are nearly all of it, and the media of the seed site is 39983 bytes. The three timed parts of the restore add to 0.083 + 0.455 + 7.397 = 7.935 s and 0.088 + 0.631 + 7.379 = 8.098 s; the rest of `RESTORE-WALL-S` is the script's own start, the `MANIFEST` print and the last `/readyz` print. The start-to-ready part has a resolution of about one second, because the script polls once a second.

In words, and only for this setup: these are emulation numbers, from an amd64 container on an aarch64 host under Docker Desktop with a warm page cache, one filesystem for source and backup and no network between the machines, so the order of magnitude is seconds, not minutes. A backup of the 300 MiB point takes about one second. A restore takes about eight seconds at both points, and nearly all of it is the service starting: start-to-ready is 7.397 and 7.379 seconds, and the `check.txt` journal shows the five plugins loading about a second apart under emulation. The copy itself is the small part: 0.455 and 0.631 seconds for stop, move and copy.

Extrapolation, an estimate and not a measurement. The difference of the two data points is 314576896 bytes (300 MiB) of media more, which cost 0.323 s more backup wall (1.015 minus 0.692) and 0.176 s more stop, move and copy (0.631 minus 0.455), while start-to-ready did not change (7.397 against 7.379). That is about 1.1 ms per MiB for the backup and about 0.6 ms per MiB for the restore's copy, so roughly 1.1 seconds per GiB of media to back up and 0.6 seconds per GiB to restore, on top of a fixed restore cost of about seven to eight seconds for a site with five plugins. The assumption is that the cost is bound by copying and linear in size, taken from two single readings: no repeat of the final set was made, so no spread of the final texts was measured, and the rate is good to a factor of two at best. On that assumption a site with 10 GiB of media would take on the order of 11 seconds to back up and on the order of 14 seconds to restore, of which six are the copy. A cold cache, a slow or network disk, thousands of small files instead of 63 large ones, or an off-site copy will make the copy part larger; a native CPU is expected to start the plugins faster. None of this was measured on a real server.

## What comes back and what does not

What comes back is what the restore printed about itself. From `restore.txt` of the target:

```text
COMES-BACK: csrf.key holzcloud.sqlite marke media plugins sprachen (the top-level entries of the backup directory that are in the restored data directory)
```

and the things that did not come back and were rebuilt by hand, the six `HAND-REBUILT:` lines of the same file:

```text
HAND-REBUILT: the unit file /etc/systemd/system/holzcloud.service (written by the install, no backup holds it)
HAND-REBUILT: the drop-ins /etc/systemd/system/holzcloud.service.d/container.conf (written by the install or the harness, no backup holds them)
HAND-REBUILT: no environment file is configured on this unit; on a real server HOLZCLOUD_SECRET_KEY and payment or mail credentials live in one, never in the data directory, so no backup holds them
HAND-REBUILT: /root/.hc-login-pass (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
HAND-REBUILT: /root/.hc-key (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
HAND-REBUILT: /tmp/hc-upload.jpg (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)
```

What lives outside every backup, by the scripts and by `deploy/DEPLOY.md`: the unit file and its drop-ins in `/etc/systemd/system`, environment files, `HOLZCLOUD_SECRET_KEY` and the payment and mail credentials that live in an environment file, the proxy configuration and the certificate state, and the pre-upgrade snapshots in the data directory. The three test fixtures (the login password file, the AI key file, the test image) stand in for what an operator remembers; they are outside the backup by design. The AI key itself is a row of the database and came back with it: a working `list_plugins` call with that key after the restore, and `PASS rows ai_tokens`, show it. The sha256 of `csrf.key` in the hash lists of the Appendix belongs to a throwaway container and protects nothing.

## Container vs. server

Everything above was played in privileged Debian 12 containers with systemd as PID 1, on an aarch64 Docker Desktop VM, under amd64 emulation. This table names each difference that matters to a backup or a restore, with what the runs showed and what a real server does. The last column says how the claim that the result carries over to a real server is known: **measured** (the thing itself was observed in these runs and the server does not differ in kind), **inferred** (read from the code, the unit or an earlier phase, or reasoned, not observed in these runs) or **assumed** (nothing in these runs speaks for it; the place to look first if a server behaves differently).

| Aspect | Measured here | Real server | Status |
|--------|---------------|-------------|--------|
| Architecture and the emulation layer | `HOST-ARCH: aarch64`, `CONTAINER-ARCH: x86_64`; the shipped unit needs the container-only `ProcSubset=all` drop-in (DEVIATION[dropin]); every time is an emulation time | A native x86-64 kernel, the unit expected to start as shipped; no native amd64 run exists | inferred |
| Seccomp and the unit's syscall filter | `Seccomp:` and `Seccomp_filters:` read 0 in `check.txt`: the filter is not enforced under this emulation, so a green run says nothing about it | The kernel enforces the filter; phase 20 measured that on native arm64 for `MemoryDenyWriteExecute` only | inferred |
| The privileged shared-kernel container | A privileged container on the kernel of the Docker Desktop VM (`7.0.14-linuxkit`), systemd as PID 1; the cgroup and memory figures are that VM's | A server runs the units under its own systemd, kernel and cgroup tree; the resource directives were not tested for effect | assumed |
| The data directory without a separate mount | A plain directory on the container's root filesystem (DEVIATION[mount]); `RequiresMountsFor` was not exercised and a restore onto a separate volume was not played | The data directory may be its own volume that must be mounted before the service starts | assumed |
| The container disk against MIN_FREE_MB | `FREE-MB: 354363`, so the floor of 1024 is never near; case d refused an impossible floor (`only 354286 MB free ... need 99999999 MB`), exit 1, no directory made | On a small server the free space can be below 1024 MB and `backup.sh` refuses; a disk that is nearly full but above the floor was not provoked, and the floor does not grow with the data | inferred |
| The container clock and zone against the real 03:15 | The clock reads UTC; the drop-in fires 60 seconds after `CLOCK: 2026-10-14 11:00:52 UTC` | The timer fires at 03:15 in the server's time zone, plus up to 300 seconds of random delay; that window was not observed | assumed |
| The timer starting the hardened service | `TriggeredBy=holzcloud-backup.timer`, `Result=success`, directory owned by `holzcloud` with mode 700 (`timer.txt`) | The same mechanism with the shipped schedule | measured |
| Persistent after a missed run | Not exercised: the timer had never fired before the one run | `Persistent=true` is meant to run a missed window after the machine was off, which was not observed | assumed |
| The tar stream against rsync or scp | The backup crossed as a tar stream and every file hash matched on both sides (`TRANSFER: equal`, 18 and 78 files) | `rsync -a` or `scp -r` between two machines, over a network, with `rsync -a` keeping the directory mode 700; not played | inferred |
| Disk speed against restore time | A warm cache, source and backup on one filesystem, 0.455 and 0.631 seconds to stop, move and copy | A real disk, a cold cache and possibly a network volume make the copy larger | assumed |
| Incompressible synthetic media against real media | 63 random-byte files of about 5 MiB (`bulk.txt`), outside the database, copied with `rsync -a` (no compression, so the content does not matter for copy time) | Real media is many more, smaller files plus thumbnails; per-file overhead was not measured | assumed |
| Plugin start time inside the restore | `START-READY-S` 7.397 and 7.379 with five plugins loading about a second apart under emulation | A native CPU is expected to start them faster; a site with more plugins starts more slowly | inferred |
| Stored secrets outside every backup | The unit has no environment file, `HAND-REBUILT:` lists the unit file, drop-ins and fixtures, and none of the hash lists holds a key file other than `csrf.key` | `HOLZCLOUD_SECRET_KEY` and the payment and mail credentials sit in an environment file outside the data directory, so no backup holds them | measured |
| Mail-account passwords without HOLZCLOUD_SECRET_KEY | Not played: no website in the seed data has a mail account (`table website_mail 0` in `state-tgt.txt`) | `deploy/DEPLOY.md` says they cannot be opened after a restore without the key | assumed |
| The database and the files taken one after the other | `PASS rows` equal in both data points; the pair was consistent in a quiet run; the window itself was not provoked | A busy site can have a file added or deleted between the snapshot and the copy | inferred |

## Not tested

What no run of this phase observed, so that nobody reads the record as saying more:

- The real 03:15 window: the timer was moved to a time a minute ahead by a container-only drop-in, so the shipped calendar entry never fired.
- `Persistent=true` after a missed run: the timer had never fired before the one observed run, and nothing was powered off.
- `RandomizedDelaySec`: set to 0 by the drop-in, so the shipped 300 seconds of random delay were never in play.
- The off-site `REMOTE_TARGET` copy: not set, so the `rsync --delete` of the backup root to another machine did not run.
- Retention pruning: the script printed `Pruning backups older than 30 days...`, but no directory was older than 30 days, so nothing was deleted and the delete was not exercised.
- An atomic database-plus-media snapshot: the pair is not atomic by construction (the snapshot first, the files afterwards, the site running), and the window in which a file is added or deleted in between was not provoked.
- A backup older than the binary: a backup made by an older binary and restored by a newer one was not played in the final set. A layout without `plugins`, `sprachen` and `marke` was restored once during the repair (plan 22-04, outside the evidence set) with exit 0 and a green `/readyz`; that is not the same claim.
- A restore on a real server: every restore went into a throwaway container, with the set-aside directory in the same filesystem, and no separate mount.
- Native amd64 timing: every number is an emulation number on an aarch64 host; native amd64 timing was not measured, and the extrapolation in *Measurements* is an estimate.
- Bit rot in a complete backup: a flipped byte inside a data page passes both integrity readers, and no checksum file exists (outside this phase by decision).
- A disk nearly full but above `MIN_FREE_MB`: the floor was only shown to refuse an impossible value.
- The consequence of losing `HOLZCLOUD_SECRET_KEY`: the document states it, no run played it.
- A restore through a browser: the login and the pages were checked with `curl` and judged by status codes, hashes and counts; what the pages look like was not seen.

## Appendix: raw output

The files of the evidence set, pasted unchanged: every line is the line of the scratch file, with nothing retyped, trimmed or re-wrapped, appended by a script that copies each file rather than by hand. Under each heading stands the scratch directory below `deploy/test/out`; each file is a fenced block under a heading that is its own path, `DIRECTORY/FILE`. The scratch directories are gitignored and are made again by the `backup-chain` command in *How this was run*. Not pasted, on purpose: any `step-NN.txt`, `seed.txt`, any `.log` file, the cookie jars and the password files. The generated passwords and the key exist only inside the throwaway containers, and `LEAK-CHECK OK` in `summary.txt` says that none of them is in any output file. The pasted text was searched for key-shaped strings, 24-character hexadecimal words, cookie headers and password-hash prefixes before it was committed. The proof that each block is its scratch file is a block-by-block byte comparison, the command is in `deploy/test/README.md` under *Phase 22: backup, timer and restore*. The BAD lines in `selftest-tree.txt` and `compare-selftest.txt` are deliberate: they show the comparisons fail when they should.

### R1 source

Scratch directory `deploy/test/out/amd64-bak-src`, the source container `holzcloud-bak-src`, written by the `backup-chain` command of *How this was run*.

#### amd64-bak-src/summary.txt

```text
---- summary (linux/amd64) ----
BAK-01 PASS
BAK-02 PASS
BAK-03 PASS
RESTORE-NEGATIVE PASS
POINT seed: BACKUP-BYTES=20548955 BACKUP-WALL-S=0.692 RESTORE-WALL-S=8.126 VERIFY-S=0.083 STOP-MOVE-COPY-S=0.455 START-READY-S=7.397
POINT bulk: BACKUP-BYTES=335125851 BACKUP-WALL-S=1.015 RESTORE-WALL-S=8.296 VERIFY-S=0.088 STOP-MOVE-COPY-S=0.631 START-READY-S=7.379
TIMER: TRIGGERED-BY=holzcloud-backup.timer RESULT=success
STEPS: 78 played, 0 failed
DEVIATIONS: backup-name dropin fixture mount scp timer transfer
LEAK-CHECK OK
```

#### amd64-bak-src/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-bak-src/steps.txt

```text
STEP 01 rc=0 sudo apt update
STEP 02 rc=0 sudo apt install -y ca-certificates curl jq sqlite3 rsync psmisc gnupg openssl
STEP 03 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 04 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 05 rc=0 scp -r deploy/ user@your-server:/tmp/deploy/
STEP 06 rc=0 sudo useradd -r -s /usr/sbin/nologin holzcloud
STEP 07 rc=0 sudo mkdir -p /opt/holzcloud/data
STEP 08 rc=0 sudo mkdir -p /opt/holzcloud/backups
STEP 09 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 10 rc=0 sudo chmod 755 /opt/holzcloud/holzcloud
STEP 11 rc=0 sudo cp -r /tmp/deploy /opt/holzcloud/deploy
STEP 12 rc=0 sudo chmod 755 /opt/holzcloud/deploy/backup.sh /opt/holzcloud/deploy/restore.sh
STEP 13 rc=0 sudo chown -R holzcloud:holzcloud /opt/holzcloud/data /opt/holzcloud/backups
STEP 14 rc=0 sudo chmod 700 /opt/holzcloud/backups
STEP 15 rc=0 sudo cp /tmp/deploy/holzcloud.service /etc/systemd/system/holzcloud.service
STEP 16 rc=0 sudo systemctl daemon-reload
STEP 17 rc=0 sudo systemctl enable holzcloud
STEP 18 rc=0 sudo systemctl start holzcloud
STEP 19 rc=3 sudo systemctl status holzcloud
STEP 20 rc=0 sudo cp -p /opt/holzcloud/holzcloud /opt/holzcloud/holzcloud.prev
STEP 21 rc=0 /opt/holzcloud/holzcloud version
STEP 22 rc=0 sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
STEP 23 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 24 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 25 rc=0 sudo systemctl stop holzcloud
STEP 26 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 27 rc=0 sudo systemctl start holzcloud
STEP 28 rc=0 curl -fsS --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/readyz | jq '{status, version, commit, integrity}'
STEP 29 rc=0 sudo journalctl -u holzcloud --since "$(systemctl show -p ActiveEnterTimestamp --value holzcloud)" -o cat | grep -E 'pre-upgrade snapshot written|migration applied' || echo 'no migration, no snapshot'
STEP 30 rc=0 sudo cp /opt/holzcloud/deploy/holzcloud-backup.{service,timer} /etc/systemd/system/
STEP 31 rc=0 sudo systemctl daemon-reload
STEP 32 rc=0 sudo systemctl enable --now holzcloud-backup.timer
STEP 33 rc=0 systemctl list-timers holzcloud-backup.timer
STEP 34 rc=0 sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
STEP 35 rc=0 sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
```

#### amd64-bak-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.0.10 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-bf74496 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[timer]: the container-only drop-in /etc/systemd/system/holzcloud-backup.timer.d/container.conf sets OnCalendar to an empty value (which resets the list) and then to *-*-* 11:01:52 (TIMER_OFFSET=60 seconds after the container's clock read 2026-10-14 11:00:52 UTC), RandomizedDelaySec=0 and AccuracySec=1s; the shipped holzcloud-backup.timer (OnCalendar=*-*-* 03:15:00, Persistent=true, RandomizedDelaySec=300) and its service are untouched, the clock is not changed, and only systemctl daemon-reload was run, no restart of the timer. It exists because the real schedule is 03:15 and a test cannot wait for it. What it cannot show, and what is therefore NOT TESTED: the real 03:15 window, Persistent=true after a missed run, and RandomizedDelaySec
```

#### amd64-bak-src/baseline.txt

```text
OLD_TAG: v0.0.10
VERSION: holzcloud v0.0.10 (ec365d6)
WEBSITE: 1
DOMAIN: test.local
PAGE: 4 plugintest (published)
PLUGINS: bestellung jahreszahl kontaktformular nicht-gefunden suche
USER: login@example.test (admin, created with holzcloud user create on the old binary)
ORDER: 2026-9001 (a direct INSERT in the statement shape of the old code, not the shop flow)
```

#### amd64-bak-src/update.txt

```text
OLD: v0.0.10 ec365d6
NEW: v0.0.25-68-gbf74496 bf74496
MIGRATIONS: old=58 new=60 pending=2
APPLIED: 2
SNAPSHOT: /opt/holzcloud/data/pre-upgrade-v0.0.25-68-gbf74496-20261014-110008.sqlite
SCHEMA: 58 -> 60
PREV: present
BACKUP: 20261014-110007
COMPARE: pass=52 bad=0
ATTACH: pass=50 bad=0
JOURNAL: {"time":"2026-10-14T11:00:08.806778924Z","level":"INFO","msg":"pre-upgrade snapshot written","path":"/opt/holzcloud/data/pre-upgrade-v0.0.25-68-gbf74496-20261014-110008.sqlite"}
JOURNAL: {"time":"2026-10-14T11:00:08.857710133Z","level":"INFO","msg":"migration applied","version":59,"type":"sql","duration_ms":3}
JOURNAL: {"time":"2026-10-14T11:00:08.858530883Z","level":"INFO","msg":"migration applied","version":60,"type":"sql","duration_ms":3}
MARKER: ok
```

#### amd64-bak-src/paths.txt

```text
PASS exists /opt/holzcloud/holzcloud
PASS exists /opt/holzcloud/data
PASS exists /opt/holzcloud/backups
PASS exists /opt/holzcloud/deploy/backup.sh
PASS exists /opt/holzcloud/deploy/holzcloud-backup.service
PASS exists /opt/holzcloud/deploy/holzcloud-backup.timer
PASS exists /opt/holzcloud/deploy/restore.sh
TOOL rsync present
TOOL gpg present
TOOL fuser present
TOOL curl present
TOOL jq present
TOOL sqlite3 present
TOOL openssl present
```

#### amd64-bak-src/backup.txt

```text
DIR: /opt/holzcloud/backups/20261014-110048
WALL-S: 0.692
FREE-MB: 354363
DIR-OWNER-MODE: holzcloud 700
BYTES: 20548955
SIZE: 20M
DB-BYTES: 663552
MEDIA-BYTES: 39983
FILES: 18
MEDIA-FILES: 3
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
```

#### amd64-bak-src/backup-run.txt

```text
=== holzcloud-CMS backup: 20261014-110048 ===
Backing up database...
2026/10/14 11:00:48 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026/10/14 11:00:48 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261014-110048/holzcloud.sqlite
2026/10/14 11:00:48 INFO backup verified path=/opt/holzcloud/backups/20261014-110048/holzcloud.sqlite tables=57
backup written and verified: /opt/holzcloud/backups/20261014-110048/holzcloud.sqlite
Backing up CSRF key...
Backing up media...
Backing up plugins...
Backing up sprachen...
Backing up marke...
Pruning backups older than 30 days...
=== Backup complete: /opt/holzcloud/backups/20261014-110048 ===
20M	/opt/holzcloud/backups/20261014-110048
WALL 1791975648.033441804 1791975648.724971554
```

#### amd64-bak-src/verify-backup.txt

```text
PASS file /opt/holzcloud/backups/20261014-110048/holzcloud.sqlite exists
PASS file /opt/holzcloud/backups/20261014-110048/MANIFEST exists
PASS integrity holzcloud-check: ok
PASS integrity sqlite3 integrity_check: ok
PASS MANIFEST timestamp=20261014-110048
PASS MANIFEST version=holzcloud v0.0.25-68-gbf74496 (bf74496)
PASS MANIFEST data_dir=/opt/holzcloud/data
PASS MANIFEST host=770a112e67c6
PASS owner and mode of the backup directory: holzcloud 700
INFO EXCLUDED holzcloud.sqlite*: the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
INFO EXCLUDED pre-upgrade-*.sqlite*: old-schema copies the service writes before a migration, deliberately not part of a backup
INFO EXCLUDED MANIFEST: only a backup has one; it names the version and the host that made it
PASS tree src-backup: 16 files equal
INFO volatile activity_log backup=22 source=21
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version backup=61 source=61
INFO volatile mail_outbox backup=0 source=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes backup=0 source=0
PASS rows order_items
PASS rows orders
INFO volatile outbox backup=0 source=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store backup=2 source=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions backup=4 source=3
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
INFO MEDIA-SERVED equals the staged source image (/tmp/hc-upload.jpg)
```

#### amd64-bak-src/selftest-tree.txt

```text
# The BAD lines below come from deliberately altered copies of tree-src-timer.txt; they are expected and are no findings
BAD  tree selftest-altered: differs: ./sprachen/nl.json
BAD  tree selftest-altered: only in A: ./csrf.key
BAD  tree selftest-empty: the reference list .tree-empty.txt is empty, which proves nothing
BAD  tree selftest-empty: only in B: ./csrf.key
BAD  tree selftest-empty: only in B: ./marke/logo.svg
BAD  tree selftest-empty: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121-medium.jpg
BAD  tree selftest-empty: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg
BAD  tree selftest-empty: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121.jpg
BAD  tree selftest-empty: only in B: ./plugins/bestellung/plugin.json
BAD  tree selftest-empty: only in B: ./plugins/bestellung/plugin.wasm
BAD  tree selftest-empty: only in B: ./plugins/jahreszahl/plugin.json
BAD  tree selftest-empty: only in B: ./plugins/jahreszahl/plugin.wasm
BAD  tree selftest-empty: only in B: ./plugins/kontaktformular/plugin.json
BAD  tree selftest-empty: only in B: ./plugins/kontaktformular/plugin.wasm
BAD  tree selftest-empty: only in B: ./plugins/nicht-gefunden/plugin.json
BAD  tree selftest-empty: only in B: ./plugins/nicht-gefunden/plugin.wasm
BAD  tree selftest-empty: only in B: ./plugins/suche/plugin.json
BAD  tree selftest-empty: only in B: ./plugins/suche/plugin.wasm
BAD  tree selftest-empty: only in B: ./sprachen/nl.json
BAD  tree selftest-nomedia: the reference list .tree-nomedia.txt has no file below ./media/, so an empty media directory would pass
BAD  tree selftest-nomedia: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121-medium.jpg
BAD  tree selftest-nomedia: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg
BAD  tree selftest-nomedia: only in B: ./media/1/b5063aa67e89ee95fad3ecb76932c121.jpg
SELFTEST OK
```

#### amd64-bak-src/tree-src-backup.txt

```text
c4962705b717f1045c1b1bf007337dae1a854ea01ea646af9bc5912c42a206eb  ./csrf.key
edb5e152af751a6fc4b9fba2aec0df921b0c0557bc16540f89162ccc5aa7dffd  ./marke/logo.svg
f7d32faf44f628fa15ca7e661f1bc44f0cc8acc9293b2a9afd793e44eda278c9  ./media/1/b5063aa67e89ee95fad3ecb76932c121-medium.jpg
30bbd092f76ba6c3e4aa8f4bf8a3ac754caf417206194a3eea77d6d150b1a3bd  ./media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg
925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577  ./media/1/b5063aa67e89ee95fad3ecb76932c121.jpg
78accaa6a3b84e6208d26327769365ddfb438476d2146af46f09cc5059803ae5  ./plugins/bestellung/plugin.json
6c3f442a5bef8b18cd346270bfc2d22d68bee30e6d4f9833515cdca3d9633584  ./plugins/bestellung/plugin.wasm
4ce4a778687ff7ce891e18d3783fc9991504a046fabb58fda6846bb3b61592d7  ./plugins/jahreszahl/plugin.json
28baaefb9815d80e51e3ba1215c1c361a388e6d4a289befd08c63ee6be626e06  ./plugins/jahreszahl/plugin.wasm
c5ae347388a54100f2bb0bbcac0dc0c6637327ff284966d8b1f3b99a3114084e  ./plugins/kontaktformular/plugin.json
853ec9cbd068baf90c90d9b28519ff0e1f6d7741092c35411497426f8ab4c526  ./plugins/kontaktformular/plugin.wasm
9b221689d2026240a136a85290565440767908793994a63cc5a15e93854a4d24  ./plugins/nicht-gefunden/plugin.json
a1a99b370740874593caa72fc59d6cf21feaabcce1db83a9bfef5f5433058c18  ./plugins/nicht-gefunden/plugin.wasm
98a5680b31113a7753784df43a4ca1a19cdaf8702388c5f1df9e467a4356c536  ./plugins/suche/plugin.json
14fbbd142bceee774ae7963b31827b05f22f9b20237bf45bf862b8027c89d13d  ./plugins/suche/plugin.wasm
4a36bcffe85430857ab8843ad6b8e32fb7968db383b1fce4f365e8304522d37c  ./sprachen/nl.json
```

#### amd64-bak-src/timer.txt

```text
TRIGGERED-BY: holzcloud-backup.timer
RESULT: success
EXIT-STATUS: 0
START: Wed 2026-10-14 11:01:52 UTC
END: Wed 2026-10-14 11:01:52 UTC
DROPIN-TIME: 11:01:52
CLOCK: 2026-10-14 11:00:52 UTC
NEXT-BEFORE: Wed 2026-10-14 11:01:52 UTC
NEXT-AFTER: Thu 2026-10-15 11:01:52 UTC
LAST-BEFORE: n/a
LAST-AFTER: Wed 2026-10-14 11:01:52 UTC
DIR: /opt/holzcloud/backups/20261014-110152
DIR-OWNER-MODE: holzcloud 700
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
PASS the service was started by holzcloud-backup.timer (TriggeredBy), not by hand
PASS Result success and ExecMainStatus 0
PASS the journal of the unit holds the script's '=== Backup complete: /opt/holzcloud/backups/20261014-110152 ===' line
PASS the directory the journal names exists: /opt/holzcloud/backups/20261014-110152
PASS owner and mode of the timer's backup directory: holzcloud 700 (the hardened unit runs as the service user)
PASS LAST before the run was empty (n/a): the timer had never fired
PASS LAST after the run is set: Wed 2026-10-14 11:01:52 UTC
PASS NEXT moved from 'Wed 2026-10-14 11:01:52 UTC' to 'Thu 2026-10-15 11:01:52 UTC'
PASS the timer made its own directory, not the manual run's (/opt/holzcloud/backups/20261014-110048)
BYTES: 20548955
SIZE: 20M
DB-BYTES: 663552
MEDIA-BYTES: 39983
FILES: 18
MEDIA-FILES: 3
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
```

#### amd64-bak-src/timer-before.txt

```text
NEXT                        LEFT     LAST PASSED UNIT                   ACTIVATES
Wed 2026-10-14 11:01:52 UTC 59s left -    -      holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-bak-src/timer-after.txt

```text
NEXT                        LEFT     LAST                        PASSED    UNIT                   ACTIVATES
Thu 2026-10-15 11:01:52 UTC 23h left Wed 2026-10-14 11:01:52 UTC 921ms ago holzcloud-backup.timer holzcloud-backup.service

1 timers listed.
Pass --all to see loaded but inactive timers, too.
```

#### amd64-bak-src/timer-journal.txt

```text
Oct 14 11:01:52.155805 770a112e67c6 systemd[1]: Starting holzcloud-backup.service - holzcloud-CMS backup...
Oct 14 11:01:52.227155 770a112e67c6 backup.sh[3917]: === holzcloud-CMS backup: 20261014-110152 ===
Oct 14 11:01:52.275067 770a112e67c6 backup.sh[3917]: Backing up database...
Oct 14 11:01:52.345982 770a112e67c6 backup.sh[3925]: 2026/10/14 11:01:52 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
Oct 14 11:01:52.371355 770a112e67c6 backup.sh[3925]: 2026/10/14 11:01:52 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261014-110152/holzcloud.sqlite
Oct 14 11:01:52.379522 770a112e67c6 backup.sh[3925]: 2026/10/14 11:01:52 INFO backup verified path=/opt/holzcloud/backups/20261014-110152/holzcloud.sqlite tables=57
Oct 14 11:01:52.380697 770a112e67c6 backup.sh[3925]: backup written and verified: /opt/holzcloud/backups/20261014-110152/holzcloud.sqlite
Oct 14 11:01:52.382008 770a112e67c6 backup.sh[3917]: Backing up CSRF key...
Oct 14 11:01:52.393417 770a112e67c6 backup.sh[3917]: Backing up media...
Oct 14 11:01:52.476827 770a112e67c6 backup.sh[3917]: Backing up plugins...
Oct 14 11:01:52.578478 770a112e67c6 backup.sh[3917]: Backing up sprachen...
Oct 14 11:01:52.665734 770a112e67c6 backup.sh[3917]: Backing up marke...
Oct 14 11:01:52.822761 770a112e67c6 backup.sh[3917]: Pruning backups older than 30 days...
Oct 14 11:01:52.836383 770a112e67c6 backup.sh[3917]: === Backup complete: /opt/holzcloud/backups/20261014-110152 ===
Oct 14 11:01:52.846182 770a112e67c6 backup.sh[3963]: 20M        /opt/holzcloud/backups/20261014-110152
Oct 14 11:01:52.847768 770a112e67c6 systemd[1]: holzcloud-backup.service: Deactivated successfully.
Oct 14 11:01:52.847851 770a112e67c6 systemd[1]: Finished holzcloud-backup.service - holzcloud-CMS backup.
```

#### amd64-bak-src/timer-dropin.txt

```text
[Timer]
OnCalendar=
OnCalendar=*-*-* 11:01:52
RandomizedDelaySec=0
AccuracySec=1s
```

#### amd64-bak-src/verify-timer.txt

```text
PASS file /opt/holzcloud/backups/20261014-110152/holzcloud.sqlite exists
PASS file /opt/holzcloud/backups/20261014-110152/MANIFEST exists
PASS integrity holzcloud-check: ok
PASS integrity sqlite3 integrity_check: ok
PASS MANIFEST timestamp=20261014-110152
PASS MANIFEST version=holzcloud v0.0.25-68-gbf74496 (bf74496)
PASS MANIFEST data_dir=/opt/holzcloud/data
PASS MANIFEST host=770a112e67c6
PASS owner and mode of the backup directory: holzcloud 700
INFO EXCLUDED holzcloud.sqlite*: the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
INFO EXCLUDED pre-upgrade-*.sqlite*: old-schema copies the service writes before a migration, deliberately not part of a backup
INFO EXCLUDED MANIFEST: only a backup has one; it names the version and the host that made it
PASS tree src-timer: 16 files equal
INFO volatile activity_log timer=22 source=21
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version timer=61 source=61
INFO volatile mail_outbox timer=0 source=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes timer=0 source=0
PASS rows order_items
PASS rows orders
INFO volatile outbox timer=0 source=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store timer=2 source=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions timer=4 source=3
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
INFO MEDIA-SERVED equals the staged source image (/tmp/hc-upload.jpg)
```

#### amd64-bak-src/tree-src-timer.txt

```text
c4962705b717f1045c1b1bf007337dae1a854ea01ea646af9bc5912c42a206eb  ./csrf.key
edb5e152af751a6fc4b9fba2aec0df921b0c0557bc16540f89162ccc5aa7dffd  ./marke/logo.svg
f7d32faf44f628fa15ca7e661f1bc44f0cc8acc9293b2a9afd793e44eda278c9  ./media/1/b5063aa67e89ee95fad3ecb76932c121-medium.jpg
30bbd092f76ba6c3e4aa8f4bf8a3ac754caf417206194a3eea77d6d150b1a3bd  ./media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg
925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577  ./media/1/b5063aa67e89ee95fad3ecb76932c121.jpg
78accaa6a3b84e6208d26327769365ddfb438476d2146af46f09cc5059803ae5  ./plugins/bestellung/plugin.json
6c3f442a5bef8b18cd346270bfc2d22d68bee30e6d4f9833515cdca3d9633584  ./plugins/bestellung/plugin.wasm
4ce4a778687ff7ce891e18d3783fc9991504a046fabb58fda6846bb3b61592d7  ./plugins/jahreszahl/plugin.json
28baaefb9815d80e51e3ba1215c1c361a388e6d4a289befd08c63ee6be626e06  ./plugins/jahreszahl/plugin.wasm
c5ae347388a54100f2bb0bbcac0dc0c6637327ff284966d8b1f3b99a3114084e  ./plugins/kontaktformular/plugin.json
853ec9cbd068baf90c90d9b28519ff0e1f6d7741092c35411497426f8ab4c526  ./plugins/kontaktformular/plugin.wasm
9b221689d2026240a136a85290565440767908793994a63cc5a15e93854a4d24  ./plugins/nicht-gefunden/plugin.json
a1a99b370740874593caa72fc59d6cf21feaabcce1db83a9bfef5f5433058c18  ./plugins/nicht-gefunden/plugin.wasm
98a5680b31113a7753784df43a4ca1a19cdaf8702388c5f1df9e467a4356c536  ./plugins/suche/plugin.json
14fbbd142bceee774ae7963b31827b05f22f9b20237bf45bf862b8027c89d13d  ./plugins/suche/plugin.wasm
4a36bcffe85430857ab8843ad6b8e32fb7968db383b1fce4f365e8304522d37c  ./sprachen/nl.json
```

### R2 target

Scratch directory `deploy/test/out/amd64-bak-tgt`, the first target container `holzcloud-bak-tgt`: the empty system the timer-made backup was restored into.

#### amd64-bak-tgt/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-bak-tgt/steps.txt

```text
STEP 01 rc=0 sudo apt update
STEP 02 rc=0 sudo apt install -y ca-certificates curl jq sqlite3 rsync psmisc gnupg openssl
STEP 03 rc=0 SKIP[dev-machine] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)" ./cmd/holzcloud
STEP 04 rc=0 scp holzcloud user@your-server:/tmp/holzcloud
STEP 05 rc=0 scp -r deploy/ user@your-server:/tmp/deploy/
STEP 06 rc=0 sudo useradd -r -s /usr/sbin/nologin holzcloud
STEP 07 rc=0 sudo mkdir -p /opt/holzcloud/data
STEP 08 rc=0 sudo mkdir -p /opt/holzcloud/backups
STEP 09 rc=0 sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
STEP 10 rc=0 sudo chmod 755 /opt/holzcloud/holzcloud
STEP 11 rc=0 sudo cp -r /tmp/deploy /opt/holzcloud/deploy
STEP 12 rc=0 sudo chmod 755 /opt/holzcloud/deploy/backup.sh /opt/holzcloud/deploy/restore.sh
STEP 13 rc=0 sudo chown -R holzcloud:holzcloud /opt/holzcloud/data /opt/holzcloud/backups
STEP 14 rc=0 sudo chmod 700 /opt/holzcloud/backups
STEP 15 rc=0 sudo cp /tmp/deploy/holzcloud.service /etc/systemd/system/holzcloud.service
STEP 16 rc=0 sudo systemctl daemon-reload
STEP 17 rc=0 sudo systemctl enable holzcloud
STEP 18 rc=0 sudo systemctl start holzcloud
STEP 19 rc=3 sudo systemctl status holzcloud
STEP 20 rc=0 sudo ls -1 /opt/holzcloud/backups
STEP 21 rc=0 sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500
```

#### amd64-bak-tgt/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-bf74496 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[transfer]: the backup travels as a tar stream through the host (docker cp SOURCE:dir - into docker cp - TARGET:/opt/holzcloud/backups); on two real machines it would be rsync or scp, which the document leaves to the operator. Proof of a lossless transfer is the sha256 of every file on both sides
DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they cross from container to container by exec-pipe, never through a host file
DEVIATION[backup-name]: the document's sample timestamp (/opt/holzcloud/backups/20260802-031500) was replaced by the directory /opt/holzcloud/backups/20261014-110152, because the document cannot know the name a backup was given; the rest of the line is as written and the answer 'yes' is typed on stdin
```

#### amd64-bak-tgt/scenario.txt

```text
SCENARIO[restore-damaged]: restore.sh is run on a copy of the transferred backup whose holzcloud.sqlite is truncated to half its size (or, when the check accepts that, has 4 KB overwritten in the middle), with 'yes' on stdin on purpose, to see that it is refused before the live data is touched. The document names the script, not this directory or this damage
SCENARIO[restore-no-sqlite]: restore.sh is run on a directory that holds only a copy of MANIFEST, with 'yes' on stdin, to see that it is refused
SCENARIO[restore-answer-no]: restore.sh is run on the intact transferred backup with 'no' on stdin, to see that it aborts before the service is stopped
SCENARIO[backup-low-disk]: backup.sh is run as the service user with MIN_FREE_MB=99999999, a threshold no disk has, to see that it exits 1 and makes no directory. The document names the script and its default; the value is new
```

#### amd64-bak-tgt/install.txt

```text
readyz: {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","integrity":"ok"}
binary: holzcloud v0.0.25-68-gbf74496 (bf74496)
unit: active
```

#### amd64-bak-tgt/transfer.txt

```text
DIR: /opt/holzcloud/backups/20261014-110152
SOURCE: holzcloud-bak-src:/opt/holzcloud/backups/20261014-110152
FILES: 18
TRANSFER: equal
```

#### amd64-bak-tgt/restore.txt

```text
DIR: /opt/holzcloud/backups/20261014-110152
SETUP-BEFORE: HTTP 200 -
PASS setup form open before the restore (the empty database of the first start)
PASS restore: the document's restore block ran without a failing step
READYZ: {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","integrity":"ok"}
PASS readyz status ok, integrity ok
LOGIN: HTTP 303 /admin/
PASS login with the password the user had before: 303 to /admin/
SETUP-AFTER: HTTP 404 -
PASS setup form closed after the restore (the restored database has users)
VERIFY-S: 0.083
STOP-MOVE-COPY-S: 0.455
START-READY-S: 7.397
WALL-S: 8.126
INFO the readiness loop of restore.sh polls once a second, so START-READY-S has a resolution of about one second; the times come from the amd64 emulation named in HOST-ARCH and CONTAINER-ARCH, not from a real server
DATA-BYTES: 20618714
DB-BYTES: 663552
MEDIA-BYTES: 39983
FILES: 19
MEDIA-FILES: 3
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
BACKUP-BYTES: 20548955
PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261014-110158
PASS set-aside database is the empty one of the first start (users: 0)
INFO restore.sh moves the whole data directory aside, so the -wal and -shm files of the replaced database went with it; none can reach the restored file, and the proof is that the restored database passes the row comparison and the integrity verdict
PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700
PASS owner and mode of /opt/holzcloud/data/holzcloud.sqlite: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/csrf.key: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg: holzcloud:holzcloud 600
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

#### amd64-bak-tgt/restore-run.txt

```text
1791975718.142510878 === Restoring from /opt/holzcloud/backups/20261014-110152 ===
1791975718.153433044 timestamp=20261014-110152
1791975718.165374253 version=holzcloud v0.0.25-68-gbf74496 (bf74496)
1791975718.177569169 data_dir=/opt/holzcloud/data
1791975718.190503919 host=770a112e67c6
1791975718.199968378 Verifying snapshot...
1791975718.250321419 2026/10/14 11:01:58 INFO sqlite opened journal_mode=wal path=/tmp/tmp.t0Bl2AZ252/holzcloud.sqlite
1791975718.269662628 integrity: ok
1791975718.282649003 Stopping holzcloud...
1791975718.315978294 Moving current data to /opt/holzcloud/data.before-restore-20261014-110158
1791975718.336264003 Restoring database...
1791975718.358231169 Restoring CSRF key...
1791975718.369550503 Restoring media...
1791975718.456745461 Restoring plugins...
1791975718.553178211 Restoring sprachen...
1791975718.633593711 Restoring marke...
1791975718.737822169 Starting holzcloud...
1791975718.765296378 Waiting for readiness...
1791975726.134982131 === Restore complete ===
1791975726.163620048 {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","uptime_s":1,"integrity":"ok","disk_free_bytes":371494752256,"database":{"freelist_count":0,"page_count":162,"size_bytes":663552,"wal_bytes":0}}
1791975726.173822548 
WALL 1791975718.059980961 1791975726.185571006
```

#### amd64-bak-tgt/state-tgt.txt

```text
identity: {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","integrity":"ok"}
binary: holzcloud v0.0.25-68-gbf74496 (bf74496)
schema: 60
table activity_log 23
table ai_tokens 1
table album_items 0
table albums 0
table app_settings 0
table block_types 0
table cart_items 0
table carts 0
table content_types 0
table csv_imports 0
table form_messages 0
table goose_db_version 61
table mail_outbox 0
table media 1
table media_usage 0
table media_variants 2
table menu_items 3
table menus 2
table oauth_clients 0
table oauth_codes 0
table order_items 0
table orders 1
table outbox 0
table page_field_defs 0
table page_revisions 0
table page_terms 0
table pages 5
table pages_fts 5
table pages_fts_config 1
table pages_fts_data 9
table pages_fts_docsize 5
table pages_fts_idx 7
table plugin_migrations 1
table plugin_store 2
table plugin_websites 5
table plugins 5
table product_media 0
table product_terms 0
table products 0
table redirects 0
table saved_views 0
table sessions 5
table snippets 0
table templates 8
table terms 0
table theme_wording 0
table user_recovery_codes 0
table user_tokens 0
table user_websites 0
table users 1
table website_domains 1
table website_mail 0
table website_templates 1
table websites 1
page: 722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804
media: 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 source 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
order: 2026-9001 32
owner: holzcloud 600
public: plugintest=200 root=200 title=yes
plugins: 5
login: HTTP 303 /admin/
```

#### amd64-bak-tgt/compare-src-tgt.txt

```text
# compare-states A=deploy/test/out/amd64-bak-src/state-src.txt B=deploy/test/out/amd64-bak-tgt/state-tgt.txt
INFO volatile activity_log a=21 b=23
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=61 b=61
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=2 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=3 b=5
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
PASS page sha256
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (length 32 in both, as it was)
INFO binary a='holzcloud v0.0.25-68-gbf74496 (bf74496)' b='holzcloud v0.0.25-68-gbf74496 (bf74496)'
INFO schema a=60 b=60
```

#### amd64-bak-tgt/compare-selftest.txt

```text
# compare-states A=deploy/test/out/amd64-bak-src/state-src.txt B=deploy/test/out/amd64-bak-tgt/.state-tampered.txt
INFO volatile activity_log a=21 b=23
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=61 b=61
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
BAD rows pages a=5 b=6
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=2 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=3 b=5
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
BAD page sha256 a='722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804' b='0000722cb2f1d7d8ab8fb9c875251cf546d261f0225b006bb68d898d0791b1d27092 ae73077bf91d27d2a9d8dd51f505a309f5b99e12047552819fef5adeab1e2804'
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (length 32 in both, as it was)
INFO binary a='holzcloud v0.0.25-68-gbf74496 (bf74496)' b='holzcloud v0.0.25-68-gbf74496 (bf74496)'
INFO schema a=60 b=60
# SELFTEST OK: the BAD lines above come from a deliberately altered copy of state-tgt.txt; the real comparison is in the other file
```

#### amd64-bak-tgt/check.txt

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
LATENCY /plugintest median_s=0.008818 n=10
LATENCY /suche?q=Plugintest median_s=0.007850 n=10
LATENCY /formular median_s=0.005605 n=10
LATENCY /bestellung median_s=0.005594 n=10
Seccomp:	0
Seccomp_filters:	0
NRestarts=0
Result=success
{"time":"2026-10-14T11:02:14.18974976Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}
{"time":"2026-10-14T11:02:15.440649386Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}
{"time":"2026-10-14T11:02:16.383886511Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}
{"time":"2026-10-14T11:02:17.28846572Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}
{"time":"2026-10-14T11:02:18.16954922Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}
```

#### amd64-bak-tgt/restore-negative.txt

```text
DIR: /opt/holzcloud/backups/20261014-110152
INFO the live data are the restored site; before and after every case the inode of /opt/holzcloud/data, the sha256 of every file below it except the SQLite family and MANIFEST, the service's InvocationID and ActiveEnterTimestamp, the number of /opt/holzcloud/data.before-restore-* directories, systemctl is-active, the /readyz status and PRAGMA integrity_check of the live database (immutable URI: the running service writes to the file, so a byte hash of it is no invariant) are taken and must be equal
INFO case a: damage: holzcloud.sqlite truncated from 663552 to 331776 bytes
INFO case a: check exit status 1: error: open database at /tmp/tmp.7RZyYCIZYs/holzcloud.sqlite: verify WAL: database disk image is malformed (11) 
INFO case a: DAMAGE-PROVEN truncated
PASS case a: the damage is real: the binary's check, run the way restore.sh runs it on a copy, refused the damaged file (truncated)
CASE a restore-damaged: exit=1
INFO case a output: === Restoring from /opt/holzcloud/backups/negcase-damaged ===
INFO case a output: timestamp=20261014-110152
INFO case a output: version=holzcloud v0.0.25-68-gbf74496 (bf74496)
INFO case a output: data_dir=/opt/holzcloud/data
INFO case a output: host=770a112e67c6
INFO case a output: Verifying snapshot...
INFO case a output: error: open database at /tmp/tmp.YQNAG26hmG/holzcloud.sqlite: verify WAL: database disk image is malformed (11)
INFO case a output: ERROR: the snapshot fails its integrity check — do not restore it
INFO case a output: EXIT 1
PASS case a: exit status 1
PASS case a: the output names the integrity refusal ('the snapshot fails its integrity check')
PASS case a: invariant inode unchanged (1562614)
PASS case a: invariant tree unchanged (6e171cc9b70e4cd8043cb758e17bdeee3d62e768835f9dc48653d322b49b7150)
PASS case a: invariant invocation unchanged (4f2300d0ed694741919e840285b4de53)
PASS case a: invariant activeenter unchanged (Wed 2026-10-14 11:02:11 UTC)
PASS case a: invariant asides unchanged (1)
PASS case a: invariant active unchanged (active)
PASS case a: invariant readyz unchanged (200)
PASS case a: invariant integrity unchanged (ok )
NEGATIVE-RESULT: refused
CASE b restore-no-sqlite: exit=1
INFO case b output: ERROR: /opt/holzcloud/backups/negcase-empty/holzcloud.sqlite not found
INFO case b output: EXIT 1
PASS case b: exit status 1
PASS case b: the output names the missing holzcloud.sqlite
PASS case b: invariant inode unchanged (1562614)
PASS case b: invariant tree unchanged (6e171cc9b70e4cd8043cb758e17bdeee3d62e768835f9dc48653d322b49b7150)
PASS case b: invariant invocation unchanged (4f2300d0ed694741919e840285b4de53)
PASS case b: invariant activeenter unchanged (Wed 2026-10-14 11:02:11 UTC)
PASS case b: invariant asides unchanged (1)
PASS case b: invariant active unchanged (active)
PASS case b: invariant readyz unchanged (200)
PASS case b: invariant integrity unchanged (ok )
NEGATIVE-RESULT: refused
CASE c restore-answer-no: exit=1
INFO case c output: === Restoring from /opt/holzcloud/backups/20261014-110152 ===
INFO case c output: timestamp=20261014-110152
INFO case c output: version=holzcloud v0.0.25-68-gbf74496 (bf74496)
INFO case c output: data_dir=/opt/holzcloud/data
INFO case c output: host=770a112e67c6
INFO case c output: Verifying snapshot...
INFO case c output: 2026/10/14 11:02:24 INFO sqlite opened journal_mode=wal path=/tmp/tmp.kbQ3cJqdg8/holzcloud.sqlite
INFO case c output: integrity: ok
INFO case c output: aborted
INFO case c output: EXIT 1
PASS case c: exit status 1
PASS case c: the output says 'aborted'
PASS case c: invariant inode unchanged (1562614)
PASS case c: invariant tree unchanged (6e171cc9b70e4cd8043cb758e17bdeee3d62e768835f9dc48653d322b49b7150)
PASS case c: invariant invocation unchanged (4f2300d0ed694741919e840285b4de53)
PASS case c: invariant activeenter unchanged (Wed 2026-10-14 11:02:11 UTC)
PASS case c: invariant asides unchanged (1)
PASS case c: invariant active unchanged (active)
PASS case c: invariant readyz unchanged (200)
PASS case c: invariant integrity unchanged (ok )
NEGATIVE-RESULT: refused
CASE d backup-low-disk: exit=1
INFO case d output: === holzcloud-CMS backup: 20261014-110225 ===
INFO case d output: ERROR: only 354286 MB free at /opt/holzcloud/backups, need 99999999 MB
INFO case d output: EXIT 1
PASS case d: exit status 1
PASS case d: the output names the free space ('only N MB free ... need 99999999 MB')
PASS case d: invariant inode unchanged (1562614)
PASS case d: invariant tree unchanged (6e171cc9b70e4cd8043cb758e17bdeee3d62e768835f9dc48653d322b49b7150)
PASS case d: invariant invocation unchanged (4f2300d0ed694741919e840285b4de53)
PASS case d: invariant activeenter unchanged (Wed 2026-10-14 11:02:11 UTC)
PASS case d: invariant asides unchanged (1)
PASS case d: invariant active unchanged (active)
PASS case d: invariant readyz unchanged (200)
PASS case d: invariant integrity unchanged (ok )
NEGATIVE-RESULT: refused
PASS case d: the number of directories under /opt/holzcloud/backups is unchanged (1): no directory was made
RESTORE-NEGATIVE OK
```

#### amd64-bak-tgt/tree-restored.txt

```text
c4962705b717f1045c1b1bf007337dae1a854ea01ea646af9bc5912c42a206eb  ./csrf.key
edb5e152af751a6fc4b9fba2aec0df921b0c0557bc16540f89162ccc5aa7dffd  ./marke/logo.svg
f7d32faf44f628fa15ca7e661f1bc44f0cc8acc9293b2a9afd793e44eda278c9  ./media/1/b5063aa67e89ee95fad3ecb76932c121-medium.jpg
30bbd092f76ba6c3e4aa8f4bf8a3ac754caf417206194a3eea77d6d150b1a3bd  ./media/1/b5063aa67e89ee95fad3ecb76932c121-thumb.jpg
925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577  ./media/1/b5063aa67e89ee95fad3ecb76932c121.jpg
78accaa6a3b84e6208d26327769365ddfb438476d2146af46f09cc5059803ae5  ./plugins/bestellung/plugin.json
6c3f442a5bef8b18cd346270bfc2d22d68bee30e6d4f9833515cdca3d9633584  ./plugins/bestellung/plugin.wasm
4ce4a778687ff7ce891e18d3783fc9991504a046fabb58fda6846bb3b61592d7  ./plugins/jahreszahl/plugin.json
28baaefb9815d80e51e3ba1215c1c361a388e6d4a289befd08c63ee6be626e06  ./plugins/jahreszahl/plugin.wasm
c5ae347388a54100f2bb0bbcac0dc0c6637327ff284966d8b1f3b99a3114084e  ./plugins/kontaktformular/plugin.json
853ec9cbd068baf90c90d9b28519ff0e1f6d7741092c35411497426f8ab4c526  ./plugins/kontaktformular/plugin.wasm
9b221689d2026240a136a85290565440767908793994a63cc5a15e93854a4d24  ./plugins/nicht-gefunden/plugin.json
a1a99b370740874593caa72fc59d6cf21feaabcce1db83a9bfef5f5433058c18  ./plugins/nicht-gefunden/plugin.wasm
98a5680b31113a7753784df43a4ca1a19cdaf8702388c5f1df9e467a4356c536  ./plugins/suche/plugin.json
14fbbd142bceee774ae7963b31827b05f22f9b20237bf45bf862b8027c89d13d  ./plugins/suche/plugin.wasm
4a36bcffe85430857ab8843ad6b8e32fb7968db383b1fce4f365e8304522d37c  ./sprachen/nl.json
```

### R3 bulk source

Scratch directory `deploy/test/out/amd64-bak-src-bulk`, the same source with 300 MiB of synthetic media added.

#### amd64-bak-src-bulk/backup.txt

```text
DIR: /opt/holzcloud/backups/20261014-110230
WALL-S: 1.015
FREE-MB: 353986
DIR-OWNER-MODE: holzcloud 700
BYTES: 335125851
SIZE: 320M
DB-BYTES: 663552
MEDIA-BYTES: 314612783
FILES: 78
MEDIA-FILES: 63
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
MEDIA-SERVED: HTTP 200 925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
```

#### amd64-bak-src-bulk/bulk.txt

```text
BULK-MB: 300
FILES: 60
BYTES: 314576896
MEDIA-BYTES: 314625071
```

#### amd64-bak-src-bulk/verify-backup.txt

```text
PASS file /opt/holzcloud/backups/20261014-110230/holzcloud.sqlite exists
PASS file /opt/holzcloud/backups/20261014-110230/MANIFEST exists
PASS integrity holzcloud-check: ok
PASS integrity sqlite3 integrity_check: ok
PASS MANIFEST timestamp=20261014-110230
PASS MANIFEST version=holzcloud v0.0.25-68-gbf74496 (bf74496)
PASS MANIFEST data_dir=/opt/holzcloud/data
PASS MANIFEST host=770a112e67c6
PASS owner and mode of the backup directory: holzcloud 700
INFO EXCLUDED holzcloud.sqlite*: the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
INFO EXCLUDED pre-upgrade-*.sqlite*: old-schema copies the service writes before a migration, deliberately not part of a backup
INFO EXCLUDED MANIFEST: only a backup has one; it names the version and the host that made it
PASS tree src-backup: 76 files equal
INFO volatile activity_log backup=23 source=22
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version backup=61 source=61
INFO volatile mail_outbox backup=0 source=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes backup=0 source=0
PASS rows order_items
PASS rows orders
INFO volatile outbox backup=0 source=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store backup=2 source=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions backup=5 source=4
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
INFO MEDIA-SERVED equals the staged source image (/tmp/hc-upload.jpg)
```

### R4 bulk target

Scratch directory `deploy/test/out/amd64-bak-tgt-bulk`, the second target, the empty system the 300 MiB backup was restored into.

#### amd64-bak-tgt-bulk/transfer.txt

```text
DIR: /opt/holzcloud/backups/20261014-110230
SOURCE: holzcloud-bak-src:/opt/holzcloud/backups/20261014-110230
FILES: 78
TRANSFER: equal
```

#### amd64-bak-tgt-bulk/restore.txt

```text
DIR: /opt/holzcloud/backups/20261014-110230
SETUP-BEFORE: HTTP 200 -
PASS setup form open before the restore (the empty database of the first start)
PASS restore: the document's restore block ran without a failing step
READYZ: {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","integrity":"ok"}
PASS readyz status ok, integrity ok
LOGIN: HTTP 303 /admin/
PASS login with the password the user had before: 303 to /admin/
SETUP-AFTER: HTTP 404 -
PASS setup form closed after the restore (the restored database has users)
VERIFY-S: 0.088
STOP-MOVE-COPY-S: 0.631
START-READY-S: 7.379
WALL-S: 8.296
INFO the readiness loop of restore.sh polls once a second, so START-READY-S has a resolution of about one second; the times come from the amd64 emulation named in HOST-ARCH and CONTAINER-ARCH, not from a real server
DATA-BYTES: 335195610
DB-BYTES: 663552
MEDIA-BYTES: 314612783
FILES: 79
MEDIA-FILES: 63
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
BACKUP-BYTES: 335125851
PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261014-110301
PASS set-aside database is the empty one of the first start (users: 0)
INFO restore.sh moves the whole data directory aside, so the -wal and -shm files of the replaced database went with it; none can reach the restored file, and the proof is that the restored database passes the row comparison and the integrity verdict
PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700
PASS owner and mode of /opt/holzcloud/data/holzcloud.sqlite: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/csrf.key: holzcloud:holzcloud 600
PASS owner and mode of /opt/holzcloud/data/media/1/bulk/bulk-36.bin: holzcloud:holzcloud 600
INFO EXCLUDED holzcloud.sqlite*: the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
INFO EXCLUDED pre-upgrade-*.sqlite*: old-schema copies the service writes before a migration, deliberately not part of a backup
INFO EXCLUDED MANIFEST: only a backup has one; it names the version and the host that made it
PASS tree src-restored: 76 files equal
PASS tree backup-restored: 76 files equal
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

#### amd64-bak-tgt-bulk/restore-run.txt

```text
1791975781.291720962 === Restoring from /opt/holzcloud/backups/20261014-110230 ===
1791975781.302978629 timestamp=20261014-110230
1791975781.319667462 version=holzcloud v0.0.25-68-gbf74496 (bf74496)
1791975781.331444921 data_dir=/opt/holzcloud/data
1791975781.346251087 host=770a112e67c6
1791975781.355762421 Verifying snapshot...
1791975781.415508921 2026/10/14 11:03:01 INFO sqlite opened journal_mode=wal path=/tmp/tmp.ThHBuBiNi1/holzcloud.sqlite
1791975781.431907546 integrity: ok
1791975781.443283587 Stopping holzcloud...
1791975781.481673462 Moving current data to /opt/holzcloud/data.before-restore-20261014-110301
1791975781.501753171 Restoring database...
1791975781.523700296 Restoring CSRF key...
1791975781.535884129 Restoring media...
1791975781.790678754 Restoring plugins...
1791975781.891575588 Restoring sprachen...
1791975781.974427129 Restoring marke...
1791975782.074510713 Starting holzcloud...
1791975782.104121713 Waiting for readiness...
1791975789.453268341 === Restore complete ===
1791975789.480838841 {"status":"ok","version":"v0.0.25-68-gbf74496","commit":"bf74496","uptime_s":0,"integrity":"ok","disk_free_bytes":370079416320,"database":{"freelist_count":0,"page_count":162,"size_bytes":663552,"wal_bytes":0}}
1791975789.490381425 
WALL 1791975781.206582712 1791975789.502442966
```

#### amd64-bak-tgt-bulk/compare-src-tgt.txt

```text
# compare-states A=deploy/test/out/amd64-bak-src-bulk/state-src.txt B=deploy/test/out/amd64-bak-tgt-bulk/state-tgt.txt
INFO volatile activity_log a=22 b=24
PASS rows ai_tokens
PASS rows album_items
PASS rows albums
PASS rows app_settings
PASS rows block_types
PASS rows cart_items
PASS rows carts
PASS rows content_types
PASS rows csv_imports
PASS rows form_messages
INFO volatile goose_db_version a=61 b=61
INFO volatile mail_outbox a=0 b=0
PASS rows media
PASS rows media_usage
PASS rows media_variants
PASS rows menu_items
PASS rows menus
PASS rows oauth_clients
INFO volatile oauth_codes a=0 b=0
PASS rows order_items
PASS rows orders
INFO volatile outbox a=0 b=0
PASS rows page_field_defs
PASS rows page_revisions
PASS rows page_terms
PASS rows pages
PASS rows pages_fts
PASS rows pages_fts_config
PASS rows pages_fts_data
PASS rows pages_fts_docsize
PASS rows pages_fts_idx
PASS rows plugin_migrations
INFO volatile plugin_store a=2 b=2
PASS rows plugin_websites
PASS rows plugins
PASS rows product_media
PASS rows product_terms
PASS rows products
PASS rows redirects
PASS rows saved_views
INFO volatile sessions a=4 b=6
PASS rows snippets
PASS rows templates
PASS rows terms
PASS rows theme_wording
PASS rows user_recovery_codes
PASS rows user_tokens
PASS rows user_websites
PASS rows users
PASS rows website_domains
PASS rows website_mail
PASS rows website_templates
PASS rows websites
PASS page sha256
PASS media sha256
PASS login
PASS public page
PASS plugins running 5/5
PASS order token (length 32 in both, as it was)
INFO binary a='holzcloud v0.0.25-68-gbf74496 (bf74496)' b='holzcloud v0.0.25-68-gbf74496 (bf74496)'
INFO schema a=60 b=60
```
