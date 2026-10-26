# Consistent and compatible backups, measured: results

This document is the permanent record of phase 25. It records three things that `docs/beta.md` lists as not played: what a backup holds when pages are saved and media is uploaded while it is taken (CON-01), whether this release restores a backup made by the previous release, 0.1.0 (CON-02), and what the shipped backup does on a filesystem that is nearly full but still above `MIN_FREE_MB` (CON-03). It was made on the harness of phases 22 and 24, with the shipped `deploy/backup.sh`, `deploy/restore.sh` and `deploy/holzcloud-backup.service` unchanged.

The record rests on one evidence set: a single `nightly.sh con-chain` run from fresh containers, made once against the committed text of every bound file (see *How this was run*). Its raw output is the Appendix, pasted unchanged, and `bash deploy/test/nightly.sh record-check-con` proves every block against its scratch file. Every number in this document is a line of that set and cites its file in backticks as `DIRECTORY/FILE`, with `DIRECTORY` one of the four scratch directories `amd64-t25-src` (the source), `amd64-t25-tgt` (the second host that the restores of CON-01 ran on), `amd64-t25-old` (the previous release) and `amd64-t25-new` (the fresh box this release restored onto).

The record says what was done, how many times, and what was seen. It does not say that backups are consistent, atomic or safe, because the measurements say the opposite of the first two for the pair of database and media, and a container cannot say the third. What the container cannot show is a named deviation or a line of *Not tested*, never a gap in the table.

## Summary

| Requirement | Result | Platform | Date, commit |
|-------------|--------|----------|--------------|
| CON-01 | PASS, by the second branch of the roadmap sentence: the disagreement was reproduced and its window named. The shipped `backup.sh` was run 40 times per stage while an editor's session saved pages, uploaded pictures and (in the second stage) deleted them, and each backup was restored by the shipped `restore.sh` on a second container and compared in both directions. Stage adds: 12 runs agreed, 28 restored with files that have no row (`amd64-t25-src/consist.txt`: `CONSIST-SUMMARY-STAGE-adds`). Stage deletes: 8 agreed, 13 restored with files without a row, 11 restored with rows whose file the backup lacks, 8 backups were refused by `rsync` (status 24) and left no `MANIFEST` (`CONSIST-SUMMARY-STAGE-deletes`). Every one of the 220 named windows overlaps the database snapshot step (`WINDOW-NAME` lines). Three controls on a paused `backup.sh` make each case happen on purpose: `C0` AGREE, `C1` ORPHAN-ONLY, `C2` ROW-WITHOUT-FILE (`amd64-t25-src/window.txt`). FUT-02: not needed | linux/amd64, x86_64 container under emulation on an aarch64 Docker Desktop host (`amd64-t25-src/versions.txt`) | 2026-10-26, e86d490 |
| CON-02 | PASS. A backup made by `v0.1.0` (`4d238b9`, built from the tag, `backup.sh` of the tag, `RC 0`) restored with this release's `restore.sh` onto a fresh box: 8 pages equal, 2 users equal, 12 media lines equal, both accounts log in (`HTTP 303 /admin/`), 4 of 4 media originals served with the source's sha256, migration level 60 before and 60 after, `run.sh check` 17 of 17 (`amd64-t25-old/compat.txt`). `backup.sh`, `restore.sh` and both units are byte-identical at the tag and now (`SCRIPTS-IDENTICAL`). No migration separates the two releases, so none was played | same | 2026-10-26, e86d490 |
| CON-03 | PASS, in the sense that the outcome is recorded, not that it is harmless. On a filesystem of 1064 MB under a floor of 1024 MB the guard passed (`FREE-AT-GUARD-MB: 1064`), `rsync` ran out of space (`No space left on device (28)`, code 11), `backup.sh` ended 11 and left a 1098915817 byte directory without a `MANIFEST` (`HAND-CLASS: FAILED-PARTIAL-DIR-LEFT`); the shipped unit did the same (`UNIT-RESULT: exit-code`, `UNIT-EXECMAINSTATUS: 11`); the next run was refused by the guard with 5 MB free (`AGAIN-CLASS: GUARD-REFUSED`); `restore.sh` refused the left directory with exit 1 before it asked anything (`amd64-t25-src/capfull.txt`) | same, on a tmpfs, not a disk | 2026-10-26, e86d490 |

Evidence commit: e86d490

The binary under test is the one built from `NEW_REF` (pinned by `con-chain` to `e86d490`) with the Build line of `deploy/DEPLOY.md`: `BINARY: holzcloud v0.1.0-46-ge86d490 (e86d490)` in `amd64-t25-src/consist.txt`. The totals of the set are in `amd64-t25-src/summary.txt`: `CON-01 PASS`, `CON-02 PASS`, `CON-03 PASS`, `STEPS: 96 played, 0 failed` and `LEAK-CHECK OK`; the stage statuses are in `amd64-t25-src/chain-statuses.txt`. No stage result file (`prep.txt`, `consist.txt`, `window.txt`, `capfull.txt`, `compat.txt`) holds a line that begins with `BAD`; the only file of the directory that does is `compare-selftest.txt`, the output of the comparator on four deliberately tampered states, which is meant to print them and is not cited. `CLASSIFY: none` does not occur: seven lines of the summary needed a decision, and the decisions are in *Findings*.

## How this was run

`deploy/test/nightly.sh` is a sibling of `deploy/test/run.sh` (which is not edited: `docs/beta.md` binds it to its evidence commit) and reaches the harness through `run.sh`'s public subcommands only. `con-chain` refuses to start unless `TARGET=systemd-base`, unless the four container names and the four evidence directories are distinct and safe, unless none of the four containers exists and none of the four directories holds anything, pins `NEW_REF` to one sha so that every container of this release gets the byte-identical binary, and plays source, second host, `prep`, `consist`, `window`, `capfull`, `compat-source` and `compat` as child processes with their statuses kept. The verdict of each requirement is computed from the files the stages wrote: `CON-01` is `PASS` only when `prep`, `consist` and `window` each ended 0 with their ok lines and no `BAD` line, `MIN-RUNS` in `consist.txt` is 30 or more, each stage had at least that many runs, and each stage had at least `max(10, ceil(MIN_RUNS / 2))` runs that were actually restored and compared after the restore (`RESTORED-RUNS-STAGE-adds`, `RESTORED-RUNS-STAGE-deletes`); a trace can therefore never pass for the set.

The set was made once, from fresh containers (`holzcloud-t25-src`, `holzcloud-t25-tgt`, `holzcloud-t25-old`, `holzcloud-t25-new`), against the committed text of `e86d490`, started 2026-10-26 13:18:54Z and ended 13:54:28Z on the host clock (35 minutes 34 seconds), exit 0:

```text
PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=e86d490 KEEP=1 CONSIST_RUNS=40 CONSIST_MIN_RUNS=30 CONSIST_RESTORE_EVERY=1 bash deploy/test/nightly.sh con-chain
```

The knobs that were not set on that line had their defaults: `BULK_MB=100` (100 MB of synthetic media in the source, so that the copy of the media takes long enough to be met by the load), `CHURN_WORKERS=4` (four uploaders, one page saver, in the second stage one deleter), `CHURN_PAUSE=0.05`, `CAPFULL_FLOOR_MB=1024`, `CAPFULL_MARGIN_MB=40`, `CAPFULL_DATA_MB=1150`, `COMPAT_TAG=v0.1.0` (`amd64-t25-src/consist.txt`, `amd64-t25-src/capfull.txt`, `amd64-t25-old/compat.txt`). The stage durations are from the unpasted log of the run (`deploy/test/out/amd64-t25-pass1.log`, host clock): source 1 m 22 s, second host 26 s, `prep` 5 s, `consist` 29 m 34 s (`WALL-MINUTES: 29` in `amd64-t25-src/consist.txt`), `window` 1 m 06 s, `capfull` 31 s, `compat-source` 56 s, `compat` with the leak check and the summary 1 m 34 s.

No bound file changed after the evidence commit. The bound files for this set are `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer`, `deploy/test/run.sh` and `deploy/test/nightly.sh`; this command exited 0 when the record was written:

```text
git diff --quiet e86d490 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh
```

Against the evidence commit of phase 24 the same list differs in one file: `git diff --stat b501b1b e86d490 -- <the bound files>` shows `deploy/test/nightly.sh | 3667 ++++-`, 3664 insertions and 3 deletions, and nothing else. The three deleted lines are the requirement-heading loop of `record_check`, parametrised by `RC_REQ_PREFIX` and `RC_REQ_IDS` with defaults that reproduce the old behaviour. `backup.sh`, `restore.sh`, `DEPLOY.md` and both units are byte-identical to `b501b1b`, so phase 24's evidence still binds `backup.sh`.

One defect of the harness was found and fixed before the set was made (class 1, commit `e86d490` itself): the summary code of `con-chain` ended the chain silently, under `set -e` and `pipefail`, when one of the four evidence directories held no `deviations.txt` (the fresh box and the second host never do). It was found by playing the summary function on the files of the earlier traces, not by the chain; the function is now covered by selftest with `set -e` on. The traces of plans 01 and 02 (six runs per stage, a first `capfull` and `compat` each) are not part of this set and are not cited.

Selftest ends `SELFTEST OK (542 cases)` and the phase 24 `record-check` still ends `RECORD-CHECK OK 29 blocks`. No clock was set anywhere in this phase.

## Deviations named

Everything here precedes every result: it is what the container does differently from a real server and what the harness did that the document does not contain. The first column is the name written to the `deviations.txt` files; the kinds in the final summary are `churn dropin editor-session emulation fixture fs hold mount scp start-limit transfer` (`amd64-t25-src/summary.txt`). Those four sets of lines are in `amd64-t25-src/deviations.txt`, `amd64-t25-tgt/deviations.txt`, `amd64-t25-old/deviations.txt` and `amd64-t25-new/deviations.txt`.

| Name | Why | What a real server does instead |
|------|-----|---------------------------------|
| DEVIATION[editor-session] | The load is an editor's session. `auth.MustHaveSecondFactor` is true for an administrator, so every admin URL answers an admin session with a redirect to the compulsory two-factor setup and no admin session reaches a page form (`ADMIN-SESSION: HTTP 303 /admin/2fa/einrichten` in `amd64-t25-src/prep.txt`). The account `churn@example.test`, role editor, was made with the baseline login password; no new secret exists | People edit with their own accounts, administrators with a second factor |
| DEVIATION[churn] | The load is `curl` inside the container against `127.0.0.1:8080` with the CSRF token of each form, four uploaders, one page saver and a deleter, pausing `CHURN_PAUSE` (0.05 s) plus a jitter of up to 0.29 s between requests | Browsers and people, at a much lower rate on a small site. The rate here is chosen to meet the window often, not to model a site |
| DEVIATION[hold] | The controls pause the shipped `backup.sh` between its database snapshot and its copy of the media through `HOLZCLOUD_BIN`, the variable the script documents. `/opt/holzcloud/hc-hold-bin` is a root-owned stand-in for the binary: the real binary with the same arguments and status, and after a successful `backup` it creates a flag and waits (at most 90 s) until the driver removes it. `backup.sh` is untouched. The window it opens is the one in `backup.sh`'s own comment; only its length is the container's | No pause; the window is as long as the copy of the media |
| DEVIATION[fs] | The nearly full filesystem is a tmpfs mounted over the backup root: no blocks reserved for root, no fragmentation, no journal, memory-backed, and the data directory is on another filesystem than the backup root. The errno `rsync` gets is the same (`ENOSPC`, `No space left on device`) | The backup root and the data directory usually share one disk, a full disk stops the site's own writes as well, and root-reserved blocks change the arithmetic |
| DEVIATION[emulation] | The container is linux/amd64 emulated on an arm64 host (`host architecture: aarch64`, `container uname: ... x86_64` in `amd64-t25-src/versions.txt`); copy times say nothing about a server | A native server's CPU and disk |
| DEVIATION[start-limit] | The shipped `holzcloud.service` allows 5 starts within 300 s (`START-LIMIT: StartLimitIntervalUSec=5min StartLimitBurst=5` in `amd64-t25-src/consist.txt`) and `restore.sh` stops and starts it, so `systemctl reset-failed holzcloud` is run on the restoring container before every restore. The unit is not changed and no sandbox directive is touched | Restores are rare; Finding F-6 |
| DEVIATION[transfer] | A backup travels from one container to the other as a tar stream through the host (`docker cp` into `docker cp`) | `rsync` or `scp` between two machines, or a copy back from the off-site target |
| DEVIATION[fixture] | The test image, the login accounts, a language file, a logo and, with `BULK_MB` above zero, 100 MB of synthetic media under `data/media/WEBSITE/bulk` stand in for operator work; the comparator excludes `*/bulk/` because those files have no rows by design | The operator's own content |
| DEVIATION[scp], DEVIATION[dropin], DEVIATION[mount] | Inherited from phases 21 and 22: the two `scp` lines of the document played as `docker cp`; the container-only drop-in `ProcSubset=all` on `holzcloud.service` (the shipped backup units have no drop-in, `UNIT-DROPINS: ''` in `amd64-t25-src/capfull.txt`); the unit's `RequiresMountsFor` names a plain directory | `scp`; the unit as shipped; a data directory on its own mount |

## Findings

Each finding has the class of the plan's rule: (1) a bug of the harness, fixed in `nightly.sh`; (2) a limit of the container, a named deviation or a *Not tested* line; (3) a measured property of a shipped file that matches what it and its document say, recorded and not edited; (4) a defect, a shipped file doing less than it or its document says, with a fix in a commit of its own. No finding is class 4 and no shipped file was edited.

How the classes were decided. The test for class 4 was whether the result contradicts something `deploy/backup.sh`, `deploy/restore.sh` or `deploy/DEPLOY.md` says, or leaves the operator worse off than they say. `backup.sh` says in its header: "The pair is not atomic. The database snapshot is taken first and the files are copied afterwards, with the site running. A file added in between is in the backup without a row, which is harmless; a file deleted in between can leave a row whose file the backup lacks." `DEPLOY.md` says the same in its Backups section, adds that "a directory without [a `MANIFEST`] is a backup that was interrupted, and `restore.sh` refuses it", and that "a backup can run out of space halfway". F-1, F-2, F-3 and F-4 are those sentences observed, with numbers. They were not made class 4 because nothing observed contradicts a sentence: the harmful direction was reproduced and is the one the script's comment predicts; the interrupted backups had no `MANIFEST` and `restore.sh` refused the one it was pointed at; the earlier backups were intact in every run (`EARLIER-BACKUPS-AFTER` equals `EARLIER-BACKUPS-BEFORE` in `amd64-t25-src/capfull.txt`). Two things are new to the documents and are recorded as documentation gaps for phase 28, not as defects: that a file vanishing during the media copy ends the night (F-3), and that the left directory makes the following nights fail on the guard (F-5).

A fix was also considered and not made, because every fix I can see changes what a backup does, which is a decision for the owner and not a repair. Treating `rsync` status 24 as success would turn a refused backup into a completed one with a row whose file is missing, which is the harmful direction without a failed unit to show it. Removing the directory of a failed night, or pruning before the copy, would change what a failed night leaves behind and what the retention step is for. A snapshot that is atomic (FUT-02) is a design, not an edit; this record's FUT-02 line says it is not needed to name the window. None of these needs a sandbox directive to change; the choice between them does not belong to the executor of a verification phase.

Every `CLASSIFY` line of `amd64-t25-src/summary.txt` and what was decided:

| Summary line | Finding | Class |
|--------------|---------|-------|
| consist stage adds: 28 runs with CLASS=ORPHAN-ONLY | F-1 | 3 |
| consist stage deletes: 13 runs with CLASS=ORPHAN-ONLY | F-1 | 3 |
| consist stage deletes: 11 runs with CLASS=ROW-WITHOUT-FILE | F-2 | 3 |
| consist stage deletes: 8 runs with CLASS=BACKUP-REFUSED | F-3 | 3 |
| capfull HAND-CLASS is FAILED-PARTIAL-DIR-LEFT | F-4 | 3 |
| capfull UNIT-CLASS is FAILED-PARTIAL-DIR-LEFT | F-4 | 3 |
| capfull AGAIN-CLASS is GUARD-REFUSED | F-5 | 3 |

There was no `BAD` line in any stage file and no `FALSE-COMPLETE` (nothing exited 0 with a `MANIFEST` and less than the source; `CONTROL-CLASS: COMPLETE`).

**F-1 (class 3): the pair is not atomic, in the harmless direction, without any pause.** Under the load, 28 of 40 backups in stage adds and 13 of 40 in stage deletes restored with files that have no row. Raw (`amd64-t25-src/consist-run-1-compare.txt`, the first run of stage adds):

```text
MISSING: 0
ORPHAN: 5
CLASS: ORPHAN-ONLY
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea-medium.jpg
```

The orphans are the original and the variants of the uploads that landed in the window (`ORPHAN: 5` in this run, the files of two uploads; the other lines of that run name the second one). `backup.sh` and `DEPLOY.md` call this harmless: the restored site has a file that nothing refers to, and `holzcloud check`, `PRAGMA integrity_check`, `PRAGMA foreign_key_check` and the page references were clean in every run (no `BAD` line in `amd64-t25-src/consist.txt`). What the owner sees: nothing, except disk space that is not accounted for.

**F-2 (class 3): the harmful direction, a row whose file the backup lacks, restores without a word.** 11 of 40 backups in stage deletes restored with a media row and no file (`MISSING=3` in each: the original and the two variants of one deleted picture; runs 3, 7, 8, 11, 12, 21, 24, 30, 32, 35 and 38 in `amd64-t25-src/consist-runs.txt`). Raw (`amd64-t25-src/consist-deletes-run-3-compare.txt`):

```text
CLASS: ROW-WITHOUT-FILE
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55-medium.jpg
```

What an operator sees after such a restore was read from control `C2`, where the same case is made on purpose (`amd64-t25-src/window.txt`, `C2-CONSEQUENCE` lines): the media route for the missing file answers `HTTP 404`, body 19 bytes; the file the backup has answers 200; `restore.sh` ended 0 and printed `"status":"ok"`; `holzcloud check` on a copy of the restored database ended 0 with `integrity: ok`; the editor logs in; the media list of the admin answers 200; `PAGE-REFS-DANGLING: 0`. The only symptom is the 404. No tool of the release reports the row. The released text names the case ("can leave a row whose file the backup lacks") but not this: that the restore itself will not tell. That sentence is for phase 28. Not played: a page that references the deleted picture (the churn deletes pictures that no page uses, `PAGE-REFS: 1 media files named by page HTML` is the one base picture), so the symptom of a dangling reference inside a page is not seen here.

**F-3 (class 3, with a documentation gap): a file that vanishes during the media copy ends the night.** 8 of 40 backups in stage deletes were refused by `backup.sh` itself: `rsync` ended with status 24 ("some files vanished before they could be transferred"), `set -e` ended the script, no `MANIFEST` was written. Raw (`amd64-t25-src/consist-deletes-run-9-backup.txt`):

```text
Backing up media...
file has vanished: "/opt/holzcloud/data/media/1/ebb6b45536ff96db4b0d2eaf27c5f771-medium.jpg"
rsync warning: some files vanished before they could be transferred (code 24) at main.c(1347) [sender=3.2.7]
RC 24
```

The runs are 9, 14, 16, 18, 22, 27, 36 and 40; none occurred in stage adds (`BACKUP-RC-HISTOGRAM-STAGE-deletes: 0=32 24=8`, `BACKUP-RC-HISTOGRAM-STAGE-adds: 0=40`). Nothing was restored from them: `backup.sh` makes the timestamp directory before it copies anything (`deploy/backup.sh:64`), and the transcript of such a run ends at the media step with status 24 and no later line, so each such night leaves a directory without a `MANIFEST` (a reading of the script and of that transcript; no listing of the left directories was taken), which is exactly what `DEPLOY.md` calls an interrupted backup and what `restore.sh` refuses (F-4 shows the refusal). The earlier backups were not touched. Under the shipped unit this would be a failed night (`Result exit-code`, the same mechanism as the status 11 of F-4; the unit was not played with a status 24). The documents do not say that a vanished file is such an interruption.

**F-4 (class 3): a full filesystem above the floor leaves a partial directory and a failed night.** The guard passed with 1064 MB free against a floor of 1024; the data to write was 1222 MB (`DATA-MB`). By hand and by the shipped unit the lines are the same (`amd64-t25-src/capfull-hand.txt`, `amd64-t25-src/capfull-unit.txt`):

```text
Backing up media...
rsync: [receiver] write failed on "/opt/holzcloud/backups/20261026-135143/media/1/bulk/bulk-71.bin": No space left on device (28)
rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
RC 11
```

`HAND-CLASS: FAILED-PARTIAL-DIR-LEFT`, `HAND-DIRECTORY-BYTES: 1098915817`, `HAND-DIRECTORY-FILES: 4221`, `HAND-MANIFEST: no`; the database snapshot in it is whole (`HAND-DATABASE-SNAPSHOT: present 0 ok`: present, `holzcloud check` status 0, `integrity_check` ok) and the media tree is cut off (1096963369 of 1279292860 bytes, 4219 of 4262 files). The unit ended `UNIT-ACTIVE-STATE: failed`, `UNIT-RESULT: exit-code`, `UNIT-EXECMAINSTATUS: 11`, with the same lines in the journal of that invocation (11 lines). `restore.sh` pointed at the left directory with `no` on stdin answered `ERROR: .../MANIFEST not found — the backup is incomplete, do not restore it`, exit 1, before the question, left the service `active` and moved no data (`amd64-t25-src/capfull-restore-partial.txt`). `DEPLOY.md` says "a backup can run out of space halfway", says a directory without a `MANIFEST` is refused by `restore.sh`, and `backup.sh` says that a truncated file "looks like a snapshot": here the only thing that looks like a snapshot is the database, and it is a good one, but the directory is not offered for restore.

**F-5 (class 3, with a documentation gap): the following nights are refused until space is freed.** The next run on the same filesystem (`amd64-t25-src/capfull-again.txt`):

```text
ERROR: only 5 MB free at /opt/holzcloud/backups, need 1024 MB
RC 1
```

`AGAIN-CLASS: GUARD-REFUSED`: the partial directory holds the space. Nothing in the script removes it: step 5 (retention, `deploy/backup.sh:98`) comes after the copies and `set -e` has ended the script before it. The guard (`deploy/backup.sh:59`) also exits before the retention step, so on any filesystem with less than `MIN_FREE_MB` free the script never prunes, whatever left the space short (a reading of the order of the script; the files show nothing was pruned, and the earlier backups were inside the 30 days anyway). On a real server the second night fails differently from the first (a guard message and status 1, not `rsync` and status 11), and the first night's directory has to be removed by hand. `DEPLOY.md` does not say so; it says the floor does not grow with the data. This is the documented guard working as documented, with a consequence nobody wrote down.

**F-6 (class 3 and 2): the sixth restore within five minutes on one host fails to start the service.** The shipped `holzcloud.service` carries `StartLimitIntervalSec=300` and `StartLimitBurst=5`, and `restore.sh` stops and starts the service. It was met in the earlier traces of this phase, which are not part of this set and are not cited; in this set it is not reproduced, because the harness resets the counter before every restore, and the set shows only the values (`START-LIMIT`) and the deviation. An operator who rehearses a restore repeatedly meets it; the cure is `systemctl reset-failed holzcloud`. The unit is shipped as it is. Class 3 for the unit (a property of its limit and the script's stop and start), class 2 for the harness (`DEVIATION[start-limit]`).

**F-7 (class 3): v0.1.0 to this release is a pair with nothing pending.** `backup.sh`, `restore.sh`, `holzcloud-backup.service` and `holzcloud-backup.timer` are byte-identical at `v0.1.0` and at `e86d490` (`SCRIPTS-IDENTICAL: ... yes` four times in `amd64-t25-old/compat.txt`), both releases carry 60 migrations with the highest numbered 60, the backup was at level 60 and the restored database at level 60, and the restore ran no migration (`MIGRATION-SENTENCE`). So the compatibility shown is that of two releases that differ in code (16 files, `COMPAT-DIFF`) and not in schema. A restore that crosses a migration was not played.

## CON-01: a backup taken while pages are saved and media is uploaded

**What was done.** `prep` made the editor account and recorded that an administrator session never reaches a page form (`ADMIN-SESSION`, `DEVIATION[editor-session]`): `PREP OK`, both write paths proven (`amd64-t25-src/prep.txt`). `consist` then played two stages of 40 runs. In each run a churn generator (four uploaders, one page saver, and in stage deletes one deleter that removes uploaded pictures) was started on the source, and a random fraction of a second later the document's manual backup line (`deploy/DEPLOY.md:657`, `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups`) ran with a millisecond stamp on every line of its output. The backup directory was shipped to the second container and restored there with the document's restore line (`deploy/DEPLOY.md:971`, the sample timestamp replaced, `yes` on stdin), and the restored site was compared with the backup: media rows against media files in both directions (the comparator was first shown to fail on four tampered lists: `COMPARATOR-SELFTEST`), `holzcloud check` on a copy, `PRAGMA integrity_check`, `PRAGMA foreign_key_check`, the pages' media references and the sha256 list of the restored tree against that of the backup directory. The window of every disagreement was named from the churn log and the backup's own stamps (`WINDOW-NAME` lines). `window` then paused the shipped `backup.sh` between its two copies and made one write in the pause, three times.

**Observed** (`amd64-t25-src/consist.txt`): `MIN-RUNS: 30`, `CONSIST-RUNS: 40`, `CONSIST-RESTORE-EVERY: 1`, `RESTORED-RUNS-FLOOR: 15 per stage`; `RESTORED-RUNS-STAGE-adds: 40`, `RESTORED-RUNS-STAGE-deletes: 32` (the eight refused backups have nothing to restore), `CLASS-COUNTS-STAGE-adds: AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0`, `CLASS-COUNTS-STAGE-deletes: AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8`, `RESTORE-FAILED=0` in both, and

```text
CONSIST-SUMMARY-STAGE-adds: RUNS=40 RESTORED=40 AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=35 UPLOADS-IN-WINDOW=84 OUTCOME=ORPHAN-ONLY-NAMED
CONSIST-SUMMARY-STAGE-deletes: RUNS=40 RESTORED=32 AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8 RESTORE-FAILED=0 EXPOSED=39 UPLOADS-IN-WINDOW=96 OUTCOME=ROW-WITHOUT-FILE-NAMED
```

The writes counted inside the window (from the end of the database step to the end of the media step, in the runs that were measured): stage adds 84 uploads and 24 page saves; stage deletes 96 uploads, 27 page saves and 30 deletes (`UPLOADS-IN-WINDOW-STAGE-*`, `PAGES-IN-WINDOW-STAGE-*`, `DELETES-IN-WINDOW-STAGE-*`); 35 and 39 of the 40 runs had at least one upload inside it (`EXPOSED-RUNS-STAGE-*`), so the exposure guard (half of the runs) held. The 220 `WINDOW-NAME` lines all read `overlaps-snapshot-step`: 95 for uploads in stage adds, 68 for uploads and 57 for deletes in stage deletes, none `UNRESOLVED`; the eight refused runs carry `rsync-24` as their name (`WINDOW-NAME=overlaps-snapshot-step,rsync-24` in `amd64-t25-src/consist-runs.txt`). The outcomes: `OUTCOME-STAGE-adds: ORPHAN-ONLY-NAMED`, `OUTCOME-STAGE-deletes: ROW-WITHOUT-FILE-NAMED`, and the file ends `CONSIST OK`.

The three controls (`amd64-t25-src/window.txt`, ends `WINDOW OK`):

```text
CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE
CONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY
CONTROL C2 ACTION=delete HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=3 ORPHAN=0 CLASS=ROW-WITHOUT-FILE EXPECTED=ROW-WITHOUT-FILE
```

The consequence of the harmful direction after a restore is quoted in F-2 (`C2-CONSEQUENCE` lines in the same file).

**The mechanism, from reading `backup.sh` and the media code (a reading, not a measurement; the `INFO MECHANISM` lines of `amd64-t25-src/window.txt` carry the file and line, and they were checked against the tree at the evidence commit):** the database snapshot is `VACUUM INTO` (`internal/db/maintain.go:81`) run by the command line process `holzcloud backup` (`cmd/holzcloud/cli.go:282`), not by the service, and it is a consistent database file at one instant, taken before the media are copied. `backup.sh` then copies the media with `rsync -a` (`deploy/backup.sh:83`), so the copy takes whatever the site has written by then. An upload writes the file (`internal/media/intake.go:163`) before it inserts the row (`:177`), and creates the file straight under its final name (`internal/media/upload.go:107`), with no temporary name and no rename. A delete removes the variant files (`internal/media/usage.go:191`), then the row (`:193`), then the original (`:198`). So an upload whose file `rsync` finds and whose row is newer than the snapshot gives a file without a row (harmless), a delete whose row is in the snapshot and whose files are gone before `rsync` reads them gives a row without a file (harmful), and a file that disappears while `rsync` is reading the directory gives status 24, which `set -e` turns into the end of the script before the `MANIFEST` (`deploy/backup.sh:2`, `:91`), and `restore.sh` refuses a directory without one. That all 220 names are `overlaps-snapshot-step` fits this reading (an event that is entirely after the snapshot was not in the file list `rsync` built), but it is a fit, not a test.

**The reason the session is an editor's.** `ADMIN-SESSION: HTTP 303 /admin/2fa/einrichten` (`amd64-t25-src/prep.txt`): the compulsory two-factor setup stands in front of every admin page for an administrator, and no page form can be reached with an admin session. The editor can make pages (drafts, and in CON-02 a published one), upload and delete pictures: the same write paths.

**Ranges.** Every stage had disagreements, so no rule-of-three bound was computed (`NO-DISAGREEMENT-STAGE-*` and `RULE-OF-THREE-STAGE-*` do not occur in `amd64-t25-src/consist.txt`). N is 40 runs per stage; the backup itself took 1.3 to 5.0 s (mean 1.56 s) in stage adds and 1.0 to 1.7 s (mean 1.46 s) in stage deletes (`BACKUP-S=` in `amd64-t25-src/consist-runs.txt`).

**What it shows.** Under a load of four uploaders, a page saver and a deleter, the shipped `backup.sh` followed by the shipped `restore.sh` gives a site whose database and media do not always agree: files without a row in 41 of 80 runs, rows without a file in 11 of 40 runs of the second stage, and a refused backup in 8 of 40. Each disagreement was named (`overlaps-snapshot-step`) and each case was reproduced on purpose by a pause between the two copies (`C1`, `C2`), with the clean case as the control (`C0`). The window is the one in `backup.sh`'s own comment. The restore, the integrity checks and the page references were clean in all 72 restored runs.

**What it does not show.** How often it happens on a real site: the load is a `curl` generator at a rate chosen to meet the window, not the traffic of a site at 03:15, and the rates here (84 and 96 uploads inside a window of a second or two in 40 runs) are an upper bound on exposure, not an estimate of it. Nothing about a media tree of production size, where the copy takes longer and the window is wider. Nothing about the symptom of a dangling reference inside a page (the deleted pictures were not used by a page). Nothing about an atomic snapshot, because none exists to compare. The unit's result for a refused backup (status 24) was not played.

**FUT-02** (`amd64-t25-src/summary.txt`): `FUT-02: not needed (adds ORPHAN-ONLY-NAMED, deletes ROW-WITHOUT-FILE-NAMED: every disagreement that occurred was reproduced and its window named)`. It rests on the `WINDOW-NAME` lines (220, none unresolved) and on the controls, and it says only that FUT-02 is not made necessary by an unnameable window; it does not say that the window is harmless, and F-2 and F-3 show it is not.

## CON-02: a backup made by 0.1.0 restores with this release

**What was done.** `compat-source` built the previous release from the tag `v0.1.0` with the Build line of `deploy/DEPLOY.md` (`COMPAT-TAG: v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712)`, `describe: v0.1.0`, `commit: 4d238b9`, `sha256: 83d5ba08ad2bde11b65c02271a13a867082c0071c81cd7b69bdf213f384ae2d3` in `amd64-t25-old/compat-binary.txt`), installed it into a fresh container out of the document (no update step: an updated source would run the new binary), and `version_is_tag` confirmed the running binary is exactly the tag (`PASS version_is_tag: the source container runs exactly the tag v0.1.0` in `amd64-t25-old/compat.txt`). A site was staged through the previous release's own admin forms: three pages with non-ASCII Markdown, a link and a picture, one more upload, an editor, so that the source held 8 pages, 2 users and 4 media rows. `compat` then ran the tag's own `backup.sh` (`BACKUP-RC: 0`, `MANIFEST: ... version=holzcloud v0.1.0 (4d238b9)`), carried the directory to a fresh box that runs this release (`holzcloud v0.1.0-46-ge86d490`), played the document's restore line there with `yes` on stdin (`RESTORE-RC: 0`, `RESTORE-WALL-S: 10.8`), and compared.

**Observed** (`amd64-t25-old/compat.txt`, ends `COMPAT OK`):

```text
STATE-SOURCE: LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3
STATE-RESTORED: LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3
LOGIN-ADMIN: HTTP 303 /admin/
LOGIN-EDITOR: HTTP 303 /admin/
MEDIA-SERVED-EQUAL: 4 of 4
LEVEL-BEFORE: 60
LEVEL-AFTER: 60
MIGRATION-SENTENCE: No migration separates the two releases: the backup is at level 60, the restored database is at level 60, the restore ran none.
CHECK: 17 of 17
```

Pages (8 lines), users (2), media (12 lines with file hashes), the row counts of 47 tables outside a named volatile list of seven (with their reasons) and the goose rows (61) are equal source against restored and backup against restored; the comparison was shown to fail on a changed page hash and a changed user role (`TAMPER-CONTROL`); the restored media rows and files agree in both directions; the restored data directory (25 files) equals the backup directory by sha256; `holzcloud check`, `integrity_check` and `foreign_key_check` are clean; the setup form answered 200 before the restore on the fresh box and 404 after; the box was fresh (`SET-ASIDE ... users 0`); `PRE-UPGRADE-SNAPSHOT-AFTER: 0`. The raw difference between the releases, as the harness wrote it: 16 files changed, 992 insertions, 61 deletions in `go.mod`, `go.sum`, `internal/media` and `cmd` (`COMPAT-DIFF` lines) and nothing in `internal/db`. What `go.mod` changes in that listing is a goose bump (`github.com/pressly/goose/v3` v3.28.0 to v3.29.0), `golang.org/x/image` v0.46.0 to v0.47.0 and other module bumps (`git diff v0.1.0 e86d490 -- go.mod`); `internal/media` gained `scale.go` and `staging.go`, the upload scale-down question, which no upload of this staged site triggered a difference for: the restored originals were served with the source's sha256 (`MEDIA-SERVED-EQUAL: 4 of 4`).

**What it shows.** A backup that the previous release's `backup.sh` made restores with this release's `restore.sh` into a fresh box with the same pages, the same users, the same media and the same logins, at the same migration level, and the site passes the 17 assertions of `run.sh check`. Because the scripts and units are byte-identical between the tag and this release, the "older binary" in this pair differs in the program and not in the script that made the backup.

**What it does not show.** A restore that crosses a migration: both releases are at level 60, and how a restore handles a pending migration (the pre-upgrade snapshot, the level going from 58 to 60) is covered only by the pure function `migration_ok` and its table in selftest, and by nothing that was played. A tag older than 0.1.0. A backup made by an older release with a different script. A real upgrade of a production site.

## CON-03: a disk nearly full but above `MIN_FREE_MB`

**What was done.** `capfull` read the shipped floor from the installed script (`SHIPPED-MIN-FREE-MB: 1024`, no `MIN_FREE_MB` in the environment or the unit, `UNIT-DROPINS: ''`), wrote 1150 MB of synthetic media so that a backup has to write more than the filesystem will hold (`DATA-MB: 1222`: the five trees, the database file and `csrf.key`, not the pre-upgrade snapshots), and mounted a tmpfs over `/opt/holzcloud/backups` of the floor plus a margin of 40 MB (`FS-SIZE-MB: 1064`). `capfull_data_ok` held: the filesystem is at least the floor and smaller than the data. Four runs followed: a control on a roomy tmpfs (`CONTROL-FS-SIZE-MB: 2646`), `backup.sh` by hand on the 1064 MB one, the next run on the same filesystem, and the shipped `holzcloud-backup.service` on a fresh 1064 MB one (`systemctl start`, so `UNIT-TRIGGEREDBY` shows the timer that can trigger the unit, not a timer that fired); `restore.sh` was also pointed at the directory the hand run left, with `no` on stdin.

**Observed** (`amd64-t25-src/capfull.txt`, ends `CAPFULL OK`):

| Run | Free at the guard | Guard | Exit | Directory left | `MANIFEST` | Class |
|-----|-------------------|-------|------|----------------|------------|-------|
| control, roomy tmpfs | 2646 MB | passed | 0 | 1281246788 bytes, 4265 files, all of the source | yes | COMPLETE |
| by hand, 1064 MB | 1064 MB | passed | 11 | `20261026-135143`, 1098915817 bytes, 4221 files | no | FAILED-PARTIAL-DIR-LEFT |
| the next run, same filesystem | 5 MB | refused | 1 | none | no | GUARD-REFUSED |
| shipped unit, fresh 1064 MB | 1064 MB | passed | 11 | 1098915817 bytes, 4221 files | no | FAILED-PARTIAL-DIR-LEFT |

The raw lines are in F-4 and F-5. The sentinel directory planted as an earlier night is intact in every run (`SENTINEL-INTACT: yes`, sha256 unchanged); the earlier backups beneath the mount (66 directories) are back after the unmount (`EARLIER-BACKUPS-AFTER: 66`, the same names). The unit: `UNIT-ACTIVE-STATE: failed`, `UNIT-RESULT: exit-code`, `UNIT-EXECMAINSTATUS: 11`, 11 journal lines of that invocation (`amd64-t25-src/capfull-unit.txt`). `restore.sh` on the left directory: `RESTORE-PARTIAL-EXIT: 1`, `SERVICE-ACTIVE-AFTER-RESTORE-PARTIAL: active`, `DATA-BEFORE-RESTORE-DIRECTORIES: 0` (`amd64-t25-src/capfull-restore-partial.txt`). The free space of every step is in `amd64-t25-src/capfull-df.txt`. The class is from `capfull_classify`: passed, non-zero, no `MANIFEST`, a directory left.

**What it shows.** At the floor the guard cannot see the data: a backup larger than the free space starts, fails when the space is gone, exits non-zero (11, by hand and by the unit), and leaves a directory with a whole database snapshot, a partial media tree and no `MANIFEST`. `restore.sh` does not accept that directory. The next night is refused by the guard until the space is freed, because the left directory holds it. Nothing exited 0 on a truncated copy (`FALSE-COMPLETE` did not occur).

**What it does not show.** A real disk: root-reserved blocks (a real filesystem's free space for the service user is less than `df` shows), fragmentation, a backup root that shares a filesystem with the data directory (so that a full disk also stops the site's own writes), and a tmpfs that is memory-backed (a tmpfs is not a disk; the errno is the same). A different `MIN_FREE_MB`. The off-site copy of a partial directory (`REMOTE_TARGET` unset here). What the next night does after the operator removed the left directory by hand (not played).

## Measurements

The figures the harness wrote. Every one is an emulation figure on an aarch64 host (`amd64-t25-src/versions.txt`); none is a promise of speed.

| Figure | Value | Where |
|--------|-------|-------|
| Runs per stage; restored runs | 40 and 40 (adds), 40 and 32 (deletes) | `CONSIST-RUNS-STAGE-*`, `RESTORED-RUNS-STAGE-*` in `amd64-t25-src/consist.txt` |
| One run, backup plus transfer plus restore plus comparison | 22 s of which the restore 12 s | `RUN-1-SECONDS`, `RUN-1-RESTORE-SECONDS` in `amd64-t25-src/consist.txt` |
| The consist stage, 80 runs | 29 minutes | `WALL-MINUTES` in `amd64-t25-src/consist.txt` |
| A backup of the source with 100 MB of bulk under load | 1.0 to 5.0 s, means 1.56 s and 1.46 s | `BACKUP-S` in `amd64-t25-src/consist-runs.txt` |
| Churn per stage | writes counted inside the window: 84 uploads, 24 page saves (adds); 96, 27 and 30 deletes (deletes) | `UPLOADS-IN-WINDOW-STAGE-*`, `PAGES-IN-WINDOW-STAGE-*`, `DELETES-IN-WINDOW-STAGE-*` in `amd64-t25-src/consist.txt` |
| Restore of the 0.1.0 backup on a fresh box | 10.8 s wall | `RESTORE-WALL-S` in `amd64-t25-old/compat.txt` |
| Files carried to the fresh box | 27 files, sha256 equal on both sides | `TRANSFER`, `FILES` in `amd64-t25-new/transfer.txt` |
| Data a backup has to write in CON-03 | 1222 MB; the filesystem 1064 MB; the control 2646 MB | `DATA-MB`, `FS-SIZE-MB`, `CONTROL-FS-SIZE-MB` in `amd64-t25-src/capfull.txt` |
| The partial directory | 1098915817 bytes, 4221 files | `HAND-DIRECTORY-BYTES`, `HAND-DIRECTORY-FILES` in `amd64-t25-src/capfull.txt` |

## Container vs. server

| Aspect | In the container | On a real server | Measured or assumed |
|--------|------------------|------------------|---------------------|
| The load | `curl` as an editor, four uploaders, a page saver, a deleter, a pause of 0.05 s plus jitter | People in browsers, much fewer writes in a given second | Measured here; the real pattern is assumed to be lighter |
| The account | An editor, because an administrator is walled off by the two-factor setup | Administrators with a second factor, editors | Measured (`ADMIN-SESSION`) |
| The window | The copy of 100 MB of bulk plus the site's media, a second or two | As long as the copy of the real media tree | Measured here; the real width is assumed to grow with the tree |
| The pause of the controls | A stand-in binary behind `HOLZCLOUD_BIN`, up to 90 s | None | Measured; the shipped script is untouched |
| The nearly full filesystem | A tmpfs over the backup root | A shared disk with root-reserved blocks, the data directory on the same disk | Measured for the errno and the script's reaction; the arithmetic of a real disk is assumed |
| Speed and timing | amd64 emulated on aarch64 | A native server | Not measured natively |
| The restore | `restore.sh` on a second container, the counter of the start limit reset between runs | One restore, by hand | Measured here; the start limit is F-6 |
| The previous release | Built from the tag by the document's Build line, installed out of the document | The operator's own 0.1.0 install | Measured |

## Not tested

- An atomic snapshot of database and media (no such snapshot exists), and the real traffic pattern of a site at the hour the timer fires.
- A media tree of production size, where the copy takes longer and the window is wider.
- A page that references a picture deleted in the window, and what such a page shows after a restore.
- A restore that crosses a migration (both releases of CON-02 are at level 60); a tag older than 0.1.0; a restore on a different CPU architecture.
- A real disk that is nearly full: root-reserved blocks, a backup root on the data directory's own filesystem, a full disk that also stops the site; a different `MIN_FREE_MB`; the off-site copy of a partial directory.
- The shipped unit's result for a backup refused with `rsync` status 24, and the night after one.
- What the night after a failed night does once the left directory was removed by hand.
- Native amd64 timing.

## Owner steps

Each step is for the real server, one command per line, with the output to expect and what a different output means. Both exist because the measurements above depend on numbers only the owner's server has.

1. Compare the data a backup has to write with the space the guard looks at. Run `sudo du -sm /opt/holzcloud/data` and `df -Pm /opt/holzcloud/backups | awk 'NR==2 {print $4}'`. The first number is an upper bound of what one backup writes (it includes the pre-upgrade snapshots, which a backup does not carry), the second is what `backup.sh` reads as free space before it starts. If the second is not clearly larger than the first, a backup can pass the guard and still run out of space halfway (CON-03, F-4): set `MIN_FREE_MB` in the unit above the first number, as `deploy/DEPLOY.md` says, or make room.
2. Look for a directory without a `MANIFEST`, the trace of a night that failed: `for d in /opt/holzcloud/backups/*/; do [ -f "$d/MANIFEST" ] || echo "no MANIFEST: $d"; done`. No output means every directory is a finished backup. A line means that night was interrupted (a full disk, F-4, or a file that vanished during the copy, F-3): `restore.sh` will refuse that directory, and if the filesystem is nearly full it is the directory that makes the next night's guard refuse (F-5). Check `systemctl status holzcloud-backup.service` for the failed unit, free the space or remove that one directory, and run `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups` once by hand.

## Hand-over to phase 28

The bound files of the evidence commit are byte-identical to `b501b1b` and to the commit phase 24's record names, except `deploy/test/nightly.sh` (this phase's additions and the three lines of the `record_check` heading loop). No shipped file was edited: `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, both units and `deploy/test/run.sh` are what `docs/beta.md` binds, so what `docs/beta.md` says about those files rests on unchanged text, and phase 24's evidence commit `b501b1b` still binds `backup.sh`. `docs/beta.md` was not edited by this phase (`git diff --quiet f1ba5a2 -- docs/beta.md` exits 0).

The three lines of `docs/beta.md` (the group *The nightly backup and a restore* under *What 0.1.0 does not promise*) that this record bears on, quoted as they stand, with what the record now supports and what stays open:

1. *"An atomic database-plus-media snapshot is not promised: the pair is taken one after the other with the site running, and the window in which a file is added or deleted in between was not provoked."* The window was provoked, by load (40 runs per stage) and on purpose (three controls), and the two directions of the disagreement were seen: files without a row in 41 of 80 runs and rows without a file in 11 of 40 runs of the second stage, and a third outcome, a backup refused by `rsync` status 24 in 8 of 40 (F-1 to F-3). Every disagreement was named (`overlaps-snapshot-step`); FUT-02 is not made necessary. What the record supports saying: the pair is not atomic, the harmful direction restores without any message and shows as a 404 on the file, and a file that vanishes during the copy ends the night. What stays open: how often this happens on a real site, a production-sized media tree, and a page that references a deleted picture.
2. *"A backup made by an older binary than the one that restores it was not played in the final set."* It was played for one pair: `v0.1.0` made the backup with its own, byte-identical `backup.sh`, this release restored it, with the same pages, users, media and logins and at level 60 on both sides (`amd64-t25-old/compat.txt`, F-7). What stays open: a pair that crosses a migration, an older tag, a different CPU architecture.
3. *"A disk that is nearly full but above `MIN_FREE_MB` was not provoked; the floor was only shown to refuse an impossible value."* It was provoked on a tmpfs of 1064 MB under a floor of 1024: the guard passed, the backup ran out of space, `backup.sh` and the shipped unit ended 11 and left a partial directory without a `MANIFEST`, `restore.sh` refused it, the next night was refused by the guard (F-4, F-5). What stays open: a real disk with reserved blocks and a shared data directory, and a different `MIN_FREE_MB`. Two sentences the documents lack, found here (F-3, F-5): that a file vanishing during the copy fails the night, and that a failed night leaves a directory that makes the next night fail on the guard until space is freed.

FUT-02: `FUT-02: not needed (adds ORPHAN-ONLY-NAMED, deletes ROW-WITHOUT-FILE-NAMED: every disagreement that occurred was reproduced and its window named)` (`amd64-t25-src/summary.txt`).

Left to the owner and not decided here, because each is a change of what a backup does and not a repair: whether `rsync` status 24 should end the night (now) or complete it with a possibly missing file; whether a failed night should remove the directory it made; whether the retention step should run before the guard and the copy. This record found no measurement that says one of them is wrong.

Nothing in this record is a promise until phase 28 cites it byte for byte. A change to a bound file after the evidence commit means the set is made again before the record is cited for the changed text.

## Appendix: raw output

Each block is the scratch file of the evidence set, pasted unchanged. `bash deploy/test/nightly.sh record-check-con` compares every block with its file byte for byte. The files are in `deploy/test/out` (gitignored); `step-NN.txt`, `seed.txt`, the logs, the churn logs, cookie jars and key files are not pasted. Of the 80 runs of `consist` the appendix holds the three per-run files (backup, restore, compare) of run 1 of stage adds and of run 3 of stage deletes (the first with a row without a file), and the backup transcript of run 9 of stage deletes (the first refused backup; there is nothing to restore or compare); every other run is in `amd64-t25-src/consist-runs.txt`, one line each, and in the 220 `WINDOW-NAME` lines of `amd64-t25-src/consist.txt`. The per-run files of the other 77 runs exist in `deploy/test/out` and are not pasted: this is the one place where the appendix holds less than the set, and a statement about those runs rests on the one-line records.


### R1 source: summary and stages

#### amd64-t25-src/summary.txt

```text
---- summary (linux/amd64) ----
CON-01 PASS
CON-02 PASS
CON-03 PASS
CONSIST: stage adds RUNS=40 RESTORED=40 AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=35 UPLOADS-IN-WINDOW=84 OUTCOME=ORPHAN-ONLY-NAMED
CONSIST: stage adds window writes: uploads 84, page saves 24, deletes 0; backup status histogram 0=40
CONSIST: stage deletes RUNS=40 RESTORED=32 AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8 RESTORE-FAILED=0 EXPOSED=39 UPLOADS-IN-WINDOW=96 OUTCOME=ROW-WITHOUT-FILE-NAMED
CONSIST: stage deletes window writes: uploads 96, page saves 27, deletes 30; backup status histogram 0=32 24=8
CONSIST: MIN-RUNS 30, restored-run floor 15 per stage (at least half of MIN-RUNS and never fewer than 10, never more than the stage has runs)
WINDOW: C0 (nothing written) AGREE, C1 (upload in the window) ORPHAN-ONLY, C2 (delete in the window) ROW-WITHOUT-FILE
COMPAT: TAG=v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712) LEVEL-BEFORE=60 LEVEL-AFTER=60 LOGIN-ADMIN=HTTP 303 /admin/ LOGIN-EDITOR=HTTP 303 /admin/ PAGES-SOURCE=8 PAGES-RESTORED=8 MEDIA-SERVED=4 of 4 CHECK=17 of 17
CAPFULL: FLOOR-MB=1024 FREE-AT-GUARD-MB=1064 DATA-MB=1222 FS-SIZE-MB=1064 CONTROL-CLASS=COMPLETE HAND-CLASS=FAILED-PARTIAL-DIR-LEFT AGAIN-CLASS=GUARD-REFUSED UNIT-CLASS=FAILED-PARTIAL-DIR-LEFT UNIT-RESULT=exit-code UNIT-EXECMAINSTATUS=11 RESTORE-PARTIAL-EXIT=1
FUT-02: not needed (adds ORPHAN-ONLY-NAMED, deletes ROW-WITHOUT-FILE-NAMED: every disagreement that occurred was reproduced and its window named)
CLASSIFY: consist stage adds: 28 run(s) with CLASS=ORPHAN-ONLY (runs 1 2 6 7 8 9 10 12 13 14 15 17 18 20 21 22 24 25 26 27 28 30 31 34 36 38 39 40)
CLASSIFY: consist stage deletes: 13 run(s) with CLASS=ORPHAN-ONLY (runs 2 4 5 13 15 17 19 20 23 25 29 31 39)
CLASSIFY: consist stage deletes: 11 run(s) with CLASS=ROW-WITHOUT-FILE (runs 3 7 8 11 12 21 24 30 32 35 38)
CLASSIFY: consist stage deletes: 8 run(s) with CLASS=BACKUP-REFUSED (runs 9 14 16 18 22 27 36 40)
CLASSIFY: capfull HAND-CLASS is FAILED-PARTIAL-DIR-LEFT (see capfull.txt)
CLASSIFY: capfull AGAIN-CLASS is GUARD-REFUSED (see capfull.txt)
CLASSIFY: capfull UNIT-CLASS is FAILED-PARTIAL-DIR-LEFT (see capfull.txt)
STEPS: 96 played, 0 failed
DEVIATIONS: churn dropin editor-session emulation fixture fs hold mount scp start-limit transfer
LEAK-CHECK OK
```

#### amd64-t25-src/chain-statuses.txt

```text
prep=0 consist=0 window=0 capfull=0 compat-source=0 compat=0
```

#### amd64-t25-src/versions.txt

```text
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           6.12.76-linuxkit
container uname:       Linux 6.12.76-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
```

#### amd64-t25-src/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-old-v0.0.10 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-e86d490 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[editor-session]: the load is an EDITOR's session, not the administrator's. auth.MustHaveSecondFactor(role, viaSSO) is true for an admin, so every admin URL answers an admin session with a redirect to the compulsory two-factor setup (ADMIN-SESSION above) and no admin session reaches a page form. The account churn@example.test, role editor, was created for it with the new binary and the baseline login password (no new secret exists)
DEVIATION[churn]: the load is curl run inside the container against 127.0.0.1:8080 with the CSRF token of each form, not browsers; the pace is set by CHURN_PAUSE and CHURN_WORKERS, not by people
DEVIATION[start-limit]: the shipped holzcloud.service allows 5 starts within 300 s and restore.sh stops and starts it, so a sixth restore within five minutes on one host ends in 'start-limit-hit' (measured, amd64-t25-trace-startlimit). Runs follow each other faster than that, so 'systemctl reset-failed holzcloud' is run on the second container before every restore; the unit is not changed and no sandbox directive is touched
DEVIATION[hold]: the pause is the container's addition. /opt/holzcloud/hc-hold-bin is a root-owned 755 stand-in for the binary: the real binary with the same arguments and status, and after a successful 'backup' it creates /tmp/hc-hold.flag and waits (at most 90 s) until the driver removes it. backup.sh is untouched and is given the stand-in through the HOLZCLOUD_BIN variable it documents. The window it opens, between the snapshot and the copy of the media, is the one in backup.sh's own comment; only its length is the container's
DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read
DEVIATION[fs]: the nearly full filesystem is a tmpfs, not a disk. It has no blocks reserved for root, no fragmentation and no journal, it lives in memory, and the data directory is on another filesystem than the backup root, whereas on a real server the two usually share one disk and a full disk stops the site's own writes as well. The error delivered to rsync is the same errno (ENOSPC, No space left on device)
DEVIATION[emulation]: the container is linux/amd64 emulated on an arm64 host; copy times say nothing about a server
```

#### amd64-t25-tgt/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-e86d490 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
```

#### amd64-t25-old/deviations.txt

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

#### amd64-t25-new/deviations.txt

```text
DEVIATION[scp]: 'scp holzcloud user@your-server:/tmp/holzcloud' was played as docker cp of deploy/test/out/bin/holzcloud-new-e86d490 to /tmp/holzcloud in the container (there is no ssh and no second machine here)
DEVIATION[scp]: 'scp -r deploy/ user@your-server:/tmp/deploy/' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into /tmp in the container, by docker cp -
DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)
DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised
DEVIATION[transfer]: the backup travels as a tar stream through the host (docker cp SOURCE:dir - into docker cp - TARGET:/opt/holzcloud/backups); on two real machines it would be rsync or scp, which the document leaves to the operator. Proof of a lossless transfer is the sha256 of every file on both sides
DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they cross from container to container by exec-pipe, never through a host file
```

#### amd64-t25-src/prep.txt

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
PASS the page row churn-prep-1793020844 exists (a draft: what an editor can make)
UPLOAD-ROWS: 2
PASS two uploads that differ in their trailing bytes gave two media rows (the strip step keeps the difference)
PASS the file of the first upload is below data/media: 1/8cf8356f8430d2f0a3107cc3613ab38a.jpg
PASS the page references an uploaded media file: media_usage has its row
PREP OK
```

### R2 CON-01 consist

#### amd64-t25-src/consist.txt

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
BASE-MEDIA: /media/1/298394ffb25e21bf962eb03e70c386c8.jpg
BINARY: holzcloud v0.1.0-46-ge86d490 (e86d490)
BULK: BULK-MB: 100 FILES: 20 BYTES: 104861696 MEDIA-BYTES: 104909871 
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
START-LIMIT: StartLimitIntervalUSec=5min StartLimitBurst=5 (shipped unit, read from holzcloud-t25-tgt)
INFO COMPARATOR-SELFTEST genuine lists: 9 9 0 0 AGREE
PASS comparator selftest: a row removed is detected as a file without a row (8 9 0 1 ORPHAN-ONLY)
PASS comparator selftest: a file added is detected as a file without a row (9 10 0 1 ORPHAN-ONLY)
PASS comparator selftest: a row added is detected as a row without a file (10 9 1 0 ROW-WITHOUT-FILE)
PASS comparator selftest: a file removed is detected as a row without a file (9 8 1 0 ROW-WITHOUT-FILE)
RUN-1-SECONDS: 22
RUN-1-RESTORE-SECONDS: 12
PROJECTED-MINUTES: 29 at CONSIST_RESTORE_EVERY=1 (2 stage(s) of 40 runs)
WALL-MINUTES: 29
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea-medium.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea-thumb.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w1-3.jpg 1793020852262 1793020852683) file 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w1-3.jpg 1793020852262 1793020852683) file 1/952490050431668bc1e8d5fcf5571c55.jpg
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w2-4.jpg 1793020875299 1793020875812) file 1/bee2f736b9d91c0c1d95d18e4b10abaa-medium.jpg
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w2-4.jpg 1793020875299 1793020875812) file 1/bee2f736b9d91c0c1d95d18e4b10abaa-thumb.jpg
WINDOW-NAME run adds 2: overlaps-snapshot-step (upload churn-adds2-w2-4.jpg 1793020875299 1793020875812) file 1/bee2f736b9d91c0c1d95d18e4b10abaa.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w1-3.jpg 1793020964101 1793020964503) file 1/688c944286e032271b988205d0594069-thumb.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w1-3.jpg 1793020964101 1793020964503) file 1/688c944286e032271b988205d0594069.jpg
WINDOW-NAME run adds 6: overlaps-snapshot-step (upload churn-adds6-w4-3.jpg 1793020964137 1793020964540) file 1/d5b1ce364db01264af86f2d86776941d.jpg
WINDOW-NAME run adds 7: overlaps-snapshot-step (upload churn-adds7-w1-3.jpg 1793020992136 1793020992833) file 1/d1f7b8748ebb474040d5b9197748c22e.jpg
WINDOW-NAME run adds 8: overlaps-snapshot-step (upload churn-adds8-w2-4.jpg 1793021028624 1793021029038) file 1/fd445accaa2770aa27717468ee72fb31-thumb.jpg
WINDOW-NAME run adds 8: overlaps-snapshot-step (upload churn-adds8-w2-4.jpg 1793021028624 1793021029038) file 1/fd445accaa2770aa27717468ee72fb31.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w2-3.jpg 1793021050924 1793021051293) file 1/5cb39611bfc51566087167df6413176b.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w3-3.jpg 1793021050829 1793021051231) file 1/9a9a052459461e9e7c4c784f1d13fbf0-medium.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w3-3.jpg 1793021050829 1793021051231) file 1/9a9a052459461e9e7c4c784f1d13fbf0-thumb.jpg
WINDOW-NAME run adds 9: overlaps-snapshot-step (upload churn-adds9-w3-3.jpg 1793021050829 1793021051231) file 1/9a9a052459461e9e7c4c784f1d13fbf0.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w1-3.jpg 1793021072016 1793021072436) file 1/72a51a715b1a0318e039e20dbda78471-thumb.jpg
WINDOW-NAME run adds 10: overlaps-snapshot-step (upload churn-adds10-w1-3.jpg 1793021072016 1793021072436) file 1/72a51a715b1a0318e039e20dbda78471.jpg
WINDOW-NAME run adds 12: overlaps-snapshot-step (upload churn-adds12-w1-4.jpg 1793021117484 1793021117873) file 1/922dfd6082166496009af53f9a372faf-medium.jpg
WINDOW-NAME run adds 12: overlaps-snapshot-step (upload churn-adds12-w1-4.jpg 1793021117484 1793021117873) file 1/922dfd6082166496009af53f9a372faf-thumb.jpg
WINDOW-NAME run adds 12: overlaps-snapshot-step (upload churn-adds12-w1-4.jpg 1793021117484 1793021117873) file 1/922dfd6082166496009af53f9a372faf.jpg
WINDOW-NAME run adds 13: overlaps-snapshot-step (upload churn-adds13-w4-4.jpg 1793021140496 1793021140953) file 1/9e06c8910ba88fb023b2aafd8a544f87.jpg
WINDOW-NAME run adds 14: overlaps-snapshot-step (upload churn-adds14-w2-3.jpg 1793021162572 1793021162972) file 1/340421f2a00549d8cd992f758689d22e-thumb.jpg
WINDOW-NAME run adds 14: overlaps-snapshot-step (upload churn-adds14-w2-3.jpg 1793021162572 1793021162972) file 1/340421f2a00549d8cd992f758689d22e.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w2-3.jpg 1793021184963 1793021185372) file 1/0bebf281010af0240543b5067ef0c95d.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w4-3.jpg 1793021184974 1793021185396) file 1/4ba6aa18e07c3c6078b42d6b39defb79.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w3-3.jpg 1793021184906 1793021185306) file 1/b9dcfba296be16c3f1f14262924ade06-medium.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w3-3.jpg 1793021184906 1793021185306) file 1/b9dcfba296be16c3f1f14262924ade06-thumb.jpg
WINDOW-NAME run adds 15: overlaps-snapshot-step (upload churn-adds15-w3-3.jpg 1793021184906 1793021185306) file 1/b9dcfba296be16c3f1f14262924ade06.jpg
WINDOW-NAME run adds 17: overlaps-snapshot-step (upload churn-adds17-w4-3.jpg 1793021229886 1793021230260) file 1/dae972d491631ce2fbc986f6777e2d8b-thumb.jpg
WINDOW-NAME run adds 17: overlaps-snapshot-step (upload churn-adds17-w4-3.jpg 1793021229886 1793021230260) file 1/dae972d491631ce2fbc986f6777e2d8b.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-4.jpg 1793021252694 1793021253101) file 1/f350912d77be235bf93c97738adab4ec-medium.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-4.jpg 1793021252694 1793021253101) file 1/f350912d77be235bf93c97738adab4ec-thumb.jpg
WINDOW-NAME run adds 18: overlaps-snapshot-step (upload churn-adds18-w2-4.jpg 1793021252694 1793021253101) file 1/f350912d77be235bf93c97738adab4ec.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w3-3.jpg 1793021299128 1793021299556) file 1/020fb97df4e46df7acea10c74a5da072-medium.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w3-3.jpg 1793021299128 1793021299556) file 1/020fb97df4e46df7acea10c74a5da072-thumb.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w3-3.jpg 1793021299128 1793021299556) file 1/020fb97df4e46df7acea10c74a5da072.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w2-3.jpg 1793021299265 1793021299674) file 1/39ea4594ce009d1caeaaa0229943e55e.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w4-3.jpg 1793021299221 1793021299619) file 1/b416e5edbb7abb83b36288011192ddd5-thumb.jpg
WINDOW-NAME run adds 20: overlaps-snapshot-step (upload churn-adds20-w4-3.jpg 1793021299221 1793021299619) file 1/b416e5edbb7abb83b36288011192ddd5.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w2-3.jpg 1793021321940 1793021322333) file 1/7c964b8466efa3dea70fc7b23e842d91-thumb.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w2-3.jpg 1793021321940 1793021322333) file 1/7c964b8466efa3dea70fc7b23e842d91.jpg
WINDOW-NAME run adds 21: overlaps-snapshot-step (upload churn-adds21-w3-3.jpg 1793021321981 1793021322388) file 1/d28d2d1840c21ba4968f2079e42bc224.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w2-4.jpg 1793021344265 1793021344642) file 1/2dfdb67930eebc11c6e7606483ee7a2b.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w4-4.jpg 1793021344196 1793021344566) file 1/3ce99fc93bee14d59f0f1fa183c4c015-medium.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w4-4.jpg 1793021344196 1793021344566) file 1/3ce99fc93bee14d59f0f1fa183c4c015-thumb.jpg
WINDOW-NAME run adds 22: overlaps-snapshot-step (upload churn-adds22-w4-4.jpg 1793021344196 1793021344566) file 1/3ce99fc93bee14d59f0f1fa183c4c015.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w1-3.jpg 1793021390017 1793021390519) file 1/761fff5c99e60056a695b7575ed58498-medium.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w1-3.jpg 1793021390017 1793021390519) file 1/761fff5c99e60056a695b7575ed58498-thumb.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w1-3.jpg 1793021390017 1793021390519) file 1/761fff5c99e60056a695b7575ed58498.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w3-3.jpg 1793021390054 1793021390557) file 1/9b0b74f354805ff3cefea6e76776de9f-medium.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w3-3.jpg 1793021390054 1793021390557) file 1/9b0b74f354805ff3cefea6e76776de9f-thumb.jpg
WINDOW-NAME run adds 24: overlaps-snapshot-step (upload churn-adds24-w3-3.jpg 1793021390054 1793021390557) file 1/9b0b74f354805ff3cefea6e76776de9f.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w2-3.jpg 1793021413580 1793021414000) file 1/a750549359a70fa45412c7843279c953-medium.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w2-3.jpg 1793021413580 1793021414000) file 1/a750549359a70fa45412c7843279c953-thumb.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w2-3.jpg 1793021413580 1793021414000) file 1/a750549359a70fa45412c7843279c953.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-3.jpg 1793021413577 1793021414004) file 1/dd141ae0d8ccefe650636a27128ebecd-medium.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-3.jpg 1793021413577 1793021414004) file 1/dd141ae0d8ccefe650636a27128ebecd-thumb.jpg
WINDOW-NAME run adds 25: overlaps-snapshot-step (upload churn-adds25-w4-3.jpg 1793021413577 1793021414004) file 1/dd141ae0d8ccefe650636a27128ebecd.jpg
WINDOW-NAME run adds 26: overlaps-snapshot-step (upload churn-adds26-w3-3.jpg 1793021435748 1793021436174) file 1/30a05881e7e8ad755856969444d5738d-medium.jpg
WINDOW-NAME run adds 26: overlaps-snapshot-step (upload churn-adds26-w3-3.jpg 1793021435748 1793021436174) file 1/30a05881e7e8ad755856969444d5738d-thumb.jpg
WINDOW-NAME run adds 26: overlaps-snapshot-step (upload churn-adds26-w3-3.jpg 1793021435748 1793021436174) file 1/30a05881e7e8ad755856969444d5738d.jpg
WINDOW-NAME run adds 27: overlaps-snapshot-step (upload churn-adds27-w2-3.jpg 1793021467805 1793021468201) file 1/be519452903e748d1d6e9427709c12b6.jpg
WINDOW-NAME run adds 27: overlaps-snapshot-step (upload churn-adds27-w1-3.jpg 1793021467725 1793021468117) file 1/c28321a48226f35ac389f7581d1421c9-medium.jpg
WINDOW-NAME run adds 27: overlaps-snapshot-step (upload churn-adds27-w1-3.jpg 1793021467725 1793021468117) file 1/c28321a48226f35ac389f7581d1421c9-thumb.jpg
WINDOW-NAME run adds 27: overlaps-snapshot-step (upload churn-adds27-w1-3.jpg 1793021467725 1793021468117) file 1/c28321a48226f35ac389f7581d1421c9.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w3-3.jpg 1793021491833 1793021492278) file 1/099c57a477d3d383cd93c38ef893f67f-medium.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w3-3.jpg 1793021491833 1793021492278) file 1/099c57a477d3d383cd93c38ef893f67f-thumb.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w3-3.jpg 1793021491833 1793021492278) file 1/099c57a477d3d383cd93c38ef893f67f.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w1-3.jpg 1793021491828 1793021492225) file 1/0a3c4edd94f4b2355af7b85671e4eeaf-medium.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w1-3.jpg 1793021491828 1793021492225) file 1/0a3c4edd94f4b2355af7b85671e4eeaf-thumb.jpg
WINDOW-NAME run adds 28: overlaps-snapshot-step (upload churn-adds28-w1-3.jpg 1793021491828 1793021492225) file 1/0a3c4edd94f4b2355af7b85671e4eeaf.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w3-3.jpg 1793021538510 1793021538921) file 1/fd618001d743ca598b2a7a09839429dd-medium.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w3-3.jpg 1793021538510 1793021538921) file 1/fd618001d743ca598b2a7a09839429dd-thumb.jpg
WINDOW-NAME run adds 30: overlaps-snapshot-step (upload churn-adds30-w3-3.jpg 1793021538510 1793021538921) file 1/fd618001d743ca598b2a7a09839429dd.jpg
WINDOW-NAME run adds 31: overlaps-snapshot-step (upload churn-adds31-w3-4.jpg 1793021562265 1793021562629) file 1/91c2b77d200a8e259acca6dd180746ca-thumb.jpg
WINDOW-NAME run adds 31: overlaps-snapshot-step (upload churn-adds31-w3-4.jpg 1793021562265 1793021562629) file 1/91c2b77d200a8e259acca6dd180746ca.jpg
WINDOW-NAME run adds 31: overlaps-snapshot-step (upload churn-adds31-w1-4.jpg 1793021562218 1793021562585) file 1/ff6de70410a34c4cf2696199a680abfa-medium.jpg
WINDOW-NAME run adds 31: overlaps-snapshot-step (upload churn-adds31-w1-4.jpg 1793021562218 1793021562585) file 1/ff6de70410a34c4cf2696199a680abfa-thumb.jpg
WINDOW-NAME run adds 31: overlaps-snapshot-step (upload churn-adds31-w1-4.jpg 1793021562218 1793021562585) file 1/ff6de70410a34c4cf2696199a680abfa.jpg
WINDOW-NAME run adds 34: overlaps-snapshot-step (upload churn-adds34-w4-4.jpg 1793021632777 1793021633154) file 1/1610c1426d60fb36c1d2cd6f138605e1.jpg
WINDOW-NAME run adds 36: overlaps-snapshot-step (upload churn-adds36-w3-4.jpg 1793021680079 1793021680472) file 1/b28e7d23d1471c408ceedd55296d7744-thumb.jpg
WINDOW-NAME run adds 36: overlaps-snapshot-step (upload churn-adds36-w3-4.jpg 1793021680079 1793021680472) file 1/b28e7d23d1471c408ceedd55296d7744.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w2-3.jpg 1793021726916 1793021727338) file 1/ddcdb4dfa998c2cdb752efe300d262fc-medium.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w2-3.jpg 1793021726916 1793021727338) file 1/ddcdb4dfa998c2cdb752efe300d262fc-thumb.jpg
WINDOW-NAME run adds 38: overlaps-snapshot-step (upload churn-adds38-w2-3.jpg 1793021726916 1793021727338) file 1/ddcdb4dfa998c2cdb752efe300d262fc.jpg
WINDOW-NAME run adds 39: overlaps-snapshot-step (upload churn-adds39-w3-3.jpg 1793021750596 1793021750987) file 1/6ef4c9ccab59467136033c16dc7c0380-medium.jpg
WINDOW-NAME run adds 39: overlaps-snapshot-step (upload churn-adds39-w3-3.jpg 1793021750596 1793021750987) file 1/6ef4c9ccab59467136033c16dc7c0380-thumb.jpg
WINDOW-NAME run adds 39: overlaps-snapshot-step (upload churn-adds39-w3-3.jpg 1793021750596 1793021750987) file 1/6ef4c9ccab59467136033c16dc7c0380.jpg
WINDOW-NAME run adds 40: overlaps-snapshot-step (upload churn-adds40-w1-3.jpg 1793021774259 1793021774641) file 1/8baf1c5e4b84c861ada1535a9b45bf7b.jpg
WINDOW-NAME run adds 40: overlaps-snapshot-step (upload churn-adds40-w3-3.jpg 1793021774194 1793021774569) file 1/bdd6cc94e13b379b09243bb8587a3f48-medium.jpg
WINDOW-NAME run adds 40: overlaps-snapshot-step (upload churn-adds40-w3-3.jpg 1793021774194 1793021774569) file 1/bdd6cc94e13b379b09243bb8587a3f48-thumb.jpg
WINDOW-NAME run adds 40: overlaps-snapshot-step (upload churn-adds40-w3-3.jpg 1793021774194 1793021774569) file 1/bdd6cc94e13b379b09243bb8587a3f48.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w2-3.jpg 1793021822199 1793021822619) file 1/0378e7186a793b1f231c655200e578d5-medium.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w2-3.jpg 1793021822199 1793021822619) file 1/0378e7186a793b1f231c655200e578d5-thumb.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w2-3.jpg 1793021822199 1793021822619) file 1/0378e7186a793b1f231c655200e578d5.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w4-3.jpg 1793021822241 1793021822657) file 1/1e784e59fedf0b27ffbe8b484113138d-medium.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w4-3.jpg 1793021822241 1793021822657) file 1/1e784e59fedf0b27ffbe8b484113138d-thumb.jpg
WINDOW-NAME run deletes 2: overlaps-snapshot-step (upload churn-deletes2-w4-3.jpg 1793021822241 1793021822657) file 1/1e784e59fedf0b27ffbe8b484113138d.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55-medium.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55.jpg
WINDOW-NAME run deletes 4: overlaps-snapshot-step (upload churn-deletes4-w3-3.jpg 1793021869236 1793021869624) file 1/15d992b8979a0fdbddf64d0b8ec26f7e.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793021893439 1793021893817) file 1/c361cdf59ebe4d13e1b4124a0a44ab38-thumb.jpg
WINDOW-NAME run deletes 5: overlaps-snapshot-step (upload churn-deletes5-w1-4.jpg 1793021893439 1793021893817) file 1/c361cdf59ebe4d13e1b4124a0a44ab38.jpg
WINDOW-NAME run deletes 7: overlaps-snapshot-step (delete 35:1/4787bdd5d5f6329ea69aa03be901df23 1793021942124 1793021942411) file 1/4787bdd5d5f6329ea69aa03be901df23-medium.jpg
WINDOW-NAME run deletes 7: overlaps-snapshot-step (delete 35:1/4787bdd5d5f6329ea69aa03be901df23 1793021942124 1793021942411) file 1/4787bdd5d5f6329ea69aa03be901df23-thumb.jpg
WINDOW-NAME run deletes 7: overlaps-snapshot-step (delete 35:1/4787bdd5d5f6329ea69aa03be901df23 1793021942124 1793021942411) file 1/4787bdd5d5f6329ea69aa03be901df23.jpg
WINDOW-NAME run deletes 7: overlaps-snapshot-step (upload churn-deletes7-w2-4.jpg 1793021942128 1793021942508) file 1/db22652338093cb6329a7b4ad342c22a.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (upload churn-deletes8-w1-4.jpg 1793021966654 1793021967060) file 1/6ea3581ca4c630de48a2091b2d3551d4.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (delete 41:1/a90ba5a86347d817ebcc3ab7c1467062 1793021966620 1793021966972) file 1/a90ba5a86347d817ebcc3ab7c1467062-medium.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (delete 41:1/a90ba5a86347d817ebcc3ab7c1467062 1793021966620 1793021966972) file 1/a90ba5a86347d817ebcc3ab7c1467062-thumb.jpg
WINDOW-NAME run deletes 8: overlaps-snapshot-step (delete 41:1/a90ba5a86347d817ebcc3ab7c1467062 1793021966620 1793021966972) file 1/a90ba5a86347d817ebcc3ab7c1467062.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (delete 46:1/ebb6b45536ff96db4b0d2eaf27c5f771 1793021991140 1793021991494) file 1/ebb6b45536ff96db4b0d2eaf27c5f771-medium.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (delete 46:1/ebb6b45536ff96db4b0d2eaf27c5f771 1793021991140 1793021991494) file 1/ebb6b45536ff96db4b0d2eaf27c5f771-thumb.jpg
WINDOW-NAME run deletes 9: overlaps-snapshot-step (delete 46:1/ebb6b45536ff96db4b0d2eaf27c5f771 1793021991140 1793021991494) file 1/ebb6b45536ff96db4b0d2eaf27c5f771.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w4-3.jpg 1793022020452 1793022020862) file 1/1d521645d72f65ea8d54ad7b3faf2bfb-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (upload churn-deletes11-w4-3.jpg 1793022020452 1793022020862) file 1/1d521645d72f65ea8d54ad7b3faf2bfb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 55:1/dd1993bc7ab3deae4b197f22ec55b330 1793022020451 1793022020781) file 1/dd1993bc7ab3deae4b197f22ec55b330-medium.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 55:1/dd1993bc7ab3deae4b197f22ec55b330 1793022020451 1793022020781) file 1/dd1993bc7ab3deae4b197f22ec55b330-thumb.jpg
WINDOW-NAME run deletes 11: overlaps-snapshot-step (delete 55:1/dd1993bc7ab3deae4b197f22ec55b330 1793022020451 1793022020781) file 1/dd1993bc7ab3deae4b197f22ec55b330.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (upload churn-deletes12-w3-4.jpg 1793022044926 1793022045382) file 1/1bbb828360e96cf5152dc6e649402fc9.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (upload churn-deletes12-w2-4.jpg 1793022044870 1793022045347) file 1/4f00b7347fffc92dba61ea2531cd4784-thumb.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (upload churn-deletes12-w2-4.jpg 1793022044870 1793022045347) file 1/4f00b7347fffc92dba61ea2531cd4784.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (delete 61:1/d67473b8be4722b6acd352e6a5aae169 1793022044896 1793022045273) file 1/d67473b8be4722b6acd352e6a5aae169-medium.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (delete 61:1/d67473b8be4722b6acd352e6a5aae169 1793022044896 1793022045273) file 1/d67473b8be4722b6acd352e6a5aae169-thumb.jpg
WINDOW-NAME run deletes 12: overlaps-snapshot-step (delete 61:1/d67473b8be4722b6acd352e6a5aae169 1793022044896 1793022045273) file 1/d67473b8be4722b6acd352e6a5aae169.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w1-3.jpg 1793022067770 1793022068141) file 1/2a0de18da1e7ff744c5d106aacacb951-thumb.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w1-3.jpg 1793022067770 1793022068141) file 1/2a0de18da1e7ff744c5d106aacacb951.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w3-3.jpg 1793022067741 1793022068122) file 1/9d9908da7dc05787bda8678c971ffe5c-thumb.jpg
WINDOW-NAME run deletes 13: overlaps-snapshot-step (upload churn-deletes13-w3-3.jpg 1793022067741 1793022068122) file 1/9d9908da7dc05787bda8678c971ffe5c.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 70:1/5933e56c7e62ef082805815809b98e58 1793022090655 1793022090992) file 1/5933e56c7e62ef082805815809b98e58-medium.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 70:1/5933e56c7e62ef082805815809b98e58 1793022090655 1793022090992) file 1/5933e56c7e62ef082805815809b98e58-thumb.jpg
WINDOW-NAME run deletes 14: overlaps-snapshot-step (delete 70:1/5933e56c7e62ef082805815809b98e58 1793022090655 1793022090992) file 1/5933e56c7e62ef082805815809b98e58.jpg
WINDOW-NAME run deletes 15: overlaps-snapshot-step (upload churn-deletes15-w3-4.jpg 1793022096746 1793022097129) file 1/6f3b93a5681c2a1638fff1f602a95db0-thumb.jpg
WINDOW-NAME run deletes 15: overlaps-snapshot-step (upload churn-deletes15-w3-4.jpg 1793022096746 1793022097129) file 1/6f3b93a5681c2a1638fff1f602a95db0.jpg
WINDOW-NAME run deletes 15: overlaps-snapshot-step (upload churn-deletes15-w1-4.jpg 1793022096688 1793022097062) file 1/b35873292cd756318d357e3a6c170855-medium.jpg
WINDOW-NAME run deletes 15: overlaps-snapshot-step (upload churn-deletes15-w1-4.jpg 1793022096688 1793022097062) file 1/b35873292cd756318d357e3a6c170855-thumb.jpg
WINDOW-NAME run deletes 15: overlaps-snapshot-step (upload churn-deletes15-w1-4.jpg 1793022096688 1793022097062) file 1/b35873292cd756318d357e3a6c170855.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 81:1/ed417267dba21f09fe8aee32da89e67e 1793022121570 1793022121866) file 1/ed417267dba21f09fe8aee32da89e67e-medium.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 81:1/ed417267dba21f09fe8aee32da89e67e 1793022121570 1793022121866) file 1/ed417267dba21f09fe8aee32da89e67e-thumb.jpg
WINDOW-NAME run deletes 16: overlaps-snapshot-step (delete 81:1/ed417267dba21f09fe8aee32da89e67e 1793022121570 1793022121866) file 1/ed417267dba21f09fe8aee32da89e67e.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w1-3.jpg 1793022126961 1793022127420) file 1/314ed8e74a4a4e56d1d992e49b3524ec-medium.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w1-3.jpg 1793022126961 1793022127420) file 1/314ed8e74a4a4e56d1d992e49b3524ec-thumb.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w1-3.jpg 1793022126961 1793022127420) file 1/314ed8e74a4a4e56d1d992e49b3524ec.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w3-3.jpg 1793022127009 1793022127453) file 1/60da446cb4759e31f5e4cc62fccbb322-thumb.jpg
WINDOW-NAME run deletes 17: overlaps-snapshot-step (upload churn-deletes17-w3-3.jpg 1793022127009 1793022127453) file 1/60da446cb4759e31f5e4cc62fccbb322.jpg
WINDOW-NAME run deletes 18: overlaps-snapshot-step (delete 91:1/ca126b229bf8d873970310bc98a355c6 1793022150832 1793022151175) file 1/ca126b229bf8d873970310bc98a355c6-medium.jpg
WINDOW-NAME run deletes 18: overlaps-snapshot-step (delete 91:1/ca126b229bf8d873970310bc98a355c6 1793022150832 1793022151175) file 1/ca126b229bf8d873970310bc98a355c6-thumb.jpg
WINDOW-NAME run deletes 18: overlaps-snapshot-step (delete 91:1/ca126b229bf8d873970310bc98a355c6 1793022150832 1793022151175) file 1/ca126b229bf8d873970310bc98a355c6.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (upload churn-deletes19-w1-4.jpg 1793022157146 1793022157547) file 1/5f26b688a7dec0010db02345ddbe12e5-thumb.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (upload churn-deletes19-w1-4.jpg 1793022157146 1793022157547) file 1/5f26b688a7dec0010db02345ddbe12e5.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (upload churn-deletes19-w4-4.jpg 1793022157052 1793022157463) file 1/b6d02b6dac8ef2ed6819322bbe02c1d3-medium.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (upload churn-deletes19-w4-4.jpg 1793022157052 1793022157463) file 1/b6d02b6dac8ef2ed6819322bbe02c1d3-thumb.jpg
WINDOW-NAME run deletes 19: overlaps-snapshot-step (upload churn-deletes19-w4-4.jpg 1793022157052 1793022157463) file 1/b6d02b6dac8ef2ed6819322bbe02c1d3.jpg
WINDOW-NAME run deletes 20: overlaps-snapshot-step (upload churn-deletes20-w2-3.jpg 1793022180607 1793022181009) file 1/05eb56bf2ca448fb8a5a5029a3a035d4.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (upload churn-deletes21-w3-4.jpg 1793022204311 1793022204688) file 1/117ac4034999c1c3dd7752b8e8671473-thumb.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (upload churn-deletes21-w3-4.jpg 1793022204311 1793022204688) file 1/117ac4034999c1c3dd7752b8e8671473.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (delete 107:1/6513e449ed876902e7d9120890d490c2 1793022204295 1793022204601) file 1/6513e449ed876902e7d9120890d490c2-medium.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (delete 107:1/6513e449ed876902e7d9120890d490c2 1793022204295 1793022204601) file 1/6513e449ed876902e7d9120890d490c2-thumb.jpg
WINDOW-NAME run deletes 21: overlaps-snapshot-step (delete 107:1/6513e449ed876902e7d9120890d490c2 1793022204295 1793022204601) file 1/6513e449ed876902e7d9120890d490c2.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 112:1/d5b1ce364db01264af86f2d86776941d 1793022227708 1793022228026) file 1/d5b1ce364db01264af86f2d86776941d-medium.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 112:1/d5b1ce364db01264af86f2d86776941d 1793022227708 1793022228026) file 1/d5b1ce364db01264af86f2d86776941d-thumb.jpg
WINDOW-NAME run deletes 22: overlaps-snapshot-step (delete 112:1/d5b1ce364db01264af86f2d86776941d 1793022227708 1793022228026) file 1/d5b1ce364db01264af86f2d86776941d.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w4-4.jpg 1793022233730 1793022234141) file 1/4cdad78e6ad8cc9b14cc47f8ee2bf666-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w4-4.jpg 1793022233730 1793022234141) file 1/4cdad78e6ad8cc9b14cc47f8ee2bf666.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w1-4.jpg 1793022233771 1793022234189) file 1/a57e5c3e74fa35dd7f4d554d88cf836a-thumb.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w1-4.jpg 1793022233771 1793022234189) file 1/a57e5c3e74fa35dd7f4d554d88cf836a.jpg
WINDOW-NAME run deletes 23: overlaps-snapshot-step (upload churn-deletes23-w2-4.jpg 1793022233787 1793022234222) file 1/e0e7eda0c65148d9e03bee2ddf554b58.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (upload churn-deletes24-w4-3.jpg 1793022258254 1793022258642) file 1/9ca2d6091aa7104ed7980c86680be8bb-thumb.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (upload churn-deletes24-w4-3.jpg 1793022258254 1793022258642) file 1/9ca2d6091aa7104ed7980c86680be8bb.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 121:1/b8493ce3ec7559a29fe4a293429c3445 1793022258239 1793022258537) file 1/b8493ce3ec7559a29fe4a293429c3445-medium.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 121:1/b8493ce3ec7559a29fe4a293429c3445 1793022258239 1793022258537) file 1/b8493ce3ec7559a29fe4a293429c3445-thumb.jpg
WINDOW-NAME run deletes 24: overlaps-snapshot-step (delete 121:1/b8493ce3ec7559a29fe4a293429c3445 1793022258239 1793022258537) file 1/b8493ce3ec7559a29fe4a293429c3445.jpg
WINDOW-NAME run deletes 25: overlaps-snapshot-step (upload churn-deletes25-w4-4.jpg 1793022282200 1793022282589) file 1/b542e52f25d6615ac22c28267b7df740.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 138:1/9f0ea34c172c50cdfcd774e37092bd52 1793022330653 1793022331084) file 1/9f0ea34c172c50cdfcd774e37092bd52-medium.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 138:1/9f0ea34c172c50cdfcd774e37092bd52 1793022330653 1793022331084) file 1/9f0ea34c172c50cdfcd774e37092bd52-thumb.jpg
WINDOW-NAME run deletes 27: overlaps-snapshot-step (delete 138:1/9f0ea34c172c50cdfcd774e37092bd52 1793022330653 1793022331084) file 1/9f0ea34c172c50cdfcd774e37092bd52.jpg
WINDOW-NAME run deletes 29: overlaps-snapshot-step (upload churn-deletes29-w1-4.jpg 1793022360591 1793022360983) file 1/18aef09d0cf0aac114fbe1daea4353ab.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w3-3.jpg 1793022384595 1793022384983) file 1/2d70d9477ef70368dba6fbfa2b64218c-medium.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w3-3.jpg 1793022384595 1793022384983) file 1/2d70d9477ef70368dba6fbfa2b64218c-thumb.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w3-3.jpg 1793022384595 1793022384983) file 1/2d70d9477ef70368dba6fbfa2b64218c.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (delete 153:1/4c4efe1797df1b74069bece4804255e4 1793022384558 1793022384865) file 1/4c4efe1797df1b74069bece4804255e4-medium.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (delete 153:1/4c4efe1797df1b74069bece4804255e4 1793022384558 1793022384865) file 1/4c4efe1797df1b74069bece4804255e4-thumb.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (delete 153:1/4c4efe1797df1b74069bece4804255e4 1793022384558 1793022384865) file 1/4c4efe1797df1b74069bece4804255e4.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w1-3.jpg 1793022384574 1793022384969) file 1/b2641b1deef3d8d99129b691885af63c-medium.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w1-3.jpg 1793022384574 1793022384969) file 1/b2641b1deef3d8d99129b691885af63c-thumb.jpg
WINDOW-NAME run deletes 30: overlaps-snapshot-step (upload churn-deletes30-w1-3.jpg 1793022384574 1793022384969) file 1/b2641b1deef3d8d99129b691885af63c.jpg
WINDOW-NAME run deletes 31: overlaps-snapshot-step (upload churn-deletes31-w4-3.jpg 1793022408516 1793022408956) file 1/16ef172bdb7642f2db37054ea9d5eee8.jpg
WINDOW-NAME run deletes 31: overlaps-snapshot-step (upload churn-deletes31-w3-3.jpg 1793022408494 1793022408893) file 1/b9b4e0e10a989f031a0d604e524a6bb6-thumb.jpg
WINDOW-NAME run deletes 31: overlaps-snapshot-step (upload churn-deletes31-w3-3.jpg 1793022408494 1793022408893) file 1/b9b4e0e10a989f031a0d604e524a6bb6.jpg
WINDOW-NAME run deletes 31: overlaps-snapshot-step (upload churn-deletes31-w2-3.jpg 1793022408479 1793022408887) file 1/f8aa82c73df9ae0734fc1947eda9fc69-thumb.jpg
WINDOW-NAME run deletes 31: overlaps-snapshot-step (upload churn-deletes31-w2-3.jpg 1793022408479 1793022408887) file 1/f8aa82c73df9ae0734fc1947eda9fc69.jpg
WINDOW-NAME run deletes 32: overlaps-snapshot-step (delete 163:1/311948e009a4a21e436fb93321f2cd07 1793022433054 1793022433362) file 1/311948e009a4a21e436fb93321f2cd07-medium.jpg
WINDOW-NAME run deletes 32: overlaps-snapshot-step (delete 163:1/311948e009a4a21e436fb93321f2cd07 1793022433054 1793022433362) file 1/311948e009a4a21e436fb93321f2cd07-thumb.jpg
WINDOW-NAME run deletes 32: overlaps-snapshot-step (delete 163:1/311948e009a4a21e436fb93321f2cd07 1793022433054 1793022433362) file 1/311948e009a4a21e436fb93321f2cd07.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (upload churn-deletes35-w4-4.jpg 1793022507942 1793022508339) file 1/33bb2fcf957509b1c4d772631dc28f66.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (delete 179:1/834061d18e5a51a5784359d66fc33bf0 1793022507834 1793022508133) file 1/834061d18e5a51a5784359d66fc33bf0-medium.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (delete 179:1/834061d18e5a51a5784359d66fc33bf0 1793022507834 1793022508133) file 1/834061d18e5a51a5784359d66fc33bf0-thumb.jpg
WINDOW-NAME run deletes 35: overlaps-snapshot-step (delete 179:1/834061d18e5a51a5784359d66fc33bf0 1793022507834 1793022508133) file 1/834061d18e5a51a5784359d66fc33bf0.jpg
WINDOW-NAME run deletes 36: overlaps-snapshot-step (delete 186:1/97fbed166718a465180b0909f11c9257 1793022532592 1793022532974) file 1/97fbed166718a465180b0909f11c9257-medium.jpg
WINDOW-NAME run deletes 36: overlaps-snapshot-step (delete 186:1/97fbed166718a465180b0909f11c9257 1793022532592 1793022532974) file 1/97fbed166718a465180b0909f11c9257-thumb.jpg
WINDOW-NAME run deletes 36: overlaps-snapshot-step (delete 186:1/97fbed166718a465180b0909f11c9257 1793022532592 1793022532974) file 1/97fbed166718a465180b0909f11c9257.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w3-3.jpg 1793022564628 1793022565061) file 1/36de240be446c424fac75da4ebbe51ff-thumb.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w3-3.jpg 1793022564628 1793022565061) file 1/36de240be446c424fac75da4ebbe51ff.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w2-3.jpg 1793022564599 1793022565011) file 1/8e2f58e016d025fff97796c4abee8bbd-medium.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w2-3.jpg 1793022564599 1793022565011) file 1/8e2f58e016d025fff97796c4abee8bbd-thumb.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w2-3.jpg 1793022564599 1793022565011) file 1/8e2f58e016d025fff97796c4abee8bbd.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (delete 195:1/a7b321e339ec9fe782a64005da67715f 1793022564621 1793022564959) file 1/a7b321e339ec9fe782a64005da67715f-medium.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (delete 195:1/a7b321e339ec9fe782a64005da67715f 1793022564621 1793022564959) file 1/a7b321e339ec9fe782a64005da67715f-thumb.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (delete 195:1/a7b321e339ec9fe782a64005da67715f 1793022564621 1793022564959) file 1/a7b321e339ec9fe782a64005da67715f.jpg
WINDOW-NAME run deletes 38: overlaps-snapshot-step (upload churn-deletes38-w1-3.jpg 1793022564650 1793022565098) file 1/e7d4b97ae66205f9c66e42734e96b18b.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793022591366 1793022591776) file 1/243f8a9a49324cf316100cf79f0f1146-medium.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793022591366 1793022591776) file 1/243f8a9a49324cf316100cf79f0f1146-thumb.jpg
WINDOW-NAME run deletes 39: overlaps-snapshot-step (upload churn-deletes39-w2-4.jpg 1793022591366 1793022591776) file 1/243f8a9a49324cf316100cf79f0f1146.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 207:1/60555b7ee6b20b7a4a8f38c44ecdbe7c 1793022616199 1793022616583) file 1/60555b7ee6b20b7a4a8f38c44ecdbe7c-medium.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 207:1/60555b7ee6b20b7a4a8f38c44ecdbe7c 1793022616199 1793022616583) file 1/60555b7ee6b20b7a4a8f38c44ecdbe7c-thumb.jpg
WINDOW-NAME run deletes 40: overlaps-snapshot-step (delete 207:1/60555b7ee6b20b7a4a8f38c44ecdbe7c 1793022616199 1793022616583) file 1/60555b7ee6b20b7a4a8f38c44ecdbe7c.jpg
CLASS-COUNTS-STAGE-adds: AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0
CONSIST-RUNS-STAGE-adds: 40
RESTORED-RUNS-STAGE-adds: 40
RESTORED-RUN-NUMBERS-STAGE-adds: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40
CLASS-AGREE-STAGE-adds: 12
CLASS-ORPHAN-ONLY-STAGE-adds: 28
CLASS-ROW-WITHOUT-FILE-STAGE-adds: 0
CLASS-BACKUP-REFUSED-STAGE-adds: 0
CLASS-RESTORE-FAILED-STAGE-adds: 0
EXPOSED-RUNS-STAGE-adds: 35
UPLOADS-IN-WINDOW-STAGE-adds: 84
PAGES-IN-WINDOW-STAGE-adds: 24
DELETES-IN-WINDOW-STAGE-adds: 0
BACKUP-RC-HISTOGRAM-STAGE-adds: 0=40
OUTCOME-STAGE-adds: ORPHAN-ONLY-NAMED
CONSIST-SUMMARY-STAGE-adds: RUNS=40 RESTORED=40 AGREE=12 ORPHAN-ONLY=28 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=35 UPLOADS-IN-WINDOW=84 OUTCOME=ORPHAN-ONLY-NAMED
CLASS-COUNTS-STAGE-deletes: AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8
CONSIST-RUNS-STAGE-deletes: 40
RESTORED-RUNS-STAGE-deletes: 32
RESTORED-RUN-NUMBERS-STAGE-deletes: 1 2 3 4 5 6 7 8 10 11 12 13 15 17 19 20 21 23 24 25 26 28 29 30 31 32 33 34 35 37 38 39
CLASS-AGREE-STAGE-deletes: 8
CLASS-ORPHAN-ONLY-STAGE-deletes: 13
CLASS-ROW-WITHOUT-FILE-STAGE-deletes: 11
CLASS-BACKUP-REFUSED-STAGE-deletes: 8
CLASS-RESTORE-FAILED-STAGE-deletes: 0
EXPOSED-RUNS-STAGE-deletes: 39
UPLOADS-IN-WINDOW-STAGE-deletes: 96
PAGES-IN-WINDOW-STAGE-deletes: 27
DELETES-IN-WINDOW-STAGE-deletes: 30
BACKUP-RC-HISTOGRAM-STAGE-deletes: 0=32 24=8
OUTCOME-STAGE-deletes: ROW-WITHOUT-FILE-NAMED
CONSIST-SUMMARY-STAGE-deletes: RUNS=40 RESTORED=32 AGREE=8 ORPHAN-ONLY=13 ROW-WITHOUT-FILE=11 BACKUP-REFUSED=8 RESTORE-FAILED=0 EXPOSED=39 UPLOADS-IN-WINDOW=96 OUTCOME=ROW-WITHOUT-FILE-NAMED
CONSIST OK
```

#### amd64-t25-src/consist-runs.txt

```text
RUN 1 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=16 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132052
RUN 2 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=5 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=24 PAGES=7 DELETES=0 BACKUP-S=3.0 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132115
RUN 3 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132138
RUN 4 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132200
RUN 5 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132222
RUN 6 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=17 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132244
RUN 7 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=7 WINDOW-PAGES=2 WINDOW-DELETES=0 UPLOADS=23 PAGES=8 DELETES=0 BACKUP-S=5.0 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132311
RUN 8 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132348
RUN 9 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=4 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=16 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132410
RUN 10 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132431
RUN 11 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=21 PAGES=7 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132454
RUN 12 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132517
RUN 13 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=7 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132540
RUN 14 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132602
RUN 15 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132624
RUN 16 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132647
RUN 17 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=16 PAGES=5 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132709
RUN 18 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=23 PAGES=6 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132732
RUN 19 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=22 PAGES=7 DELETES=0 BACKUP-S=1.7 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132755
RUN 20 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132819
RUN 21 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=16 PAGES=6 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132841
RUN 22 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=4 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132904
RUN 23 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132927
RUN 24 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-132949
RUN 25 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133013
RUN 26 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133035
RUN 27 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=4 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133107
RUN 28 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133131
RUN 29 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=7 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133155
RUN 30 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=5 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133218
RUN 31 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=21 PAGES=7 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133242
RUN 32 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133305
RUN 33 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=5 DELETES=0 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133329
RUN 34 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133352
RUN 35 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=21 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133416
RUN 36 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=4 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133439
RUN 37 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133503
RUN 38 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133526
RUN 39 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133550
RUN 40 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=4 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=16 PAGES=6 DELETES=0 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133614
RUN 1 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=23 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133638
RUN 2 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133702
RUN 3 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=7 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133725
RUN 4 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=17 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133749
RUN 5 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133813
RUN 6 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=7 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133837
RUN 7 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=1 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=22 PAGES=6 DELETES=6 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133901
RUN 8 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=1 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133926
RUN 9 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=18 PAGES=5 DELETES=5 BACKUP-S=1.0 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-133950
RUN 10 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-133956
RUN 11 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=2 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=18 PAGES=5 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134020
RUN 12 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=3 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134044
RUN 13 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=4 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134107
RUN 14 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=16 PAGES=5 DELETES=4 BACKUP-S=1.0 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134130
RUN 15 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=22 PAGES=6 DELETES=6 BACKUP-S=1.4 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134136
RUN 16 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=16 PAGES=5 DELETES=5 BACKUP-S=1.1 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134201
RUN 17 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=16 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134206
RUN 18 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=18 PAGES=5 DELETES=5 BACKUP-S=1.1 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134230
RUN 19 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=23 PAGES=7 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134237
RUN 20 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=1 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134300
RUN 21 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=2 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134324
RUN 22 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=16 PAGES=4 DELETES=4 BACKUP-S=1.0 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134347
RUN 23 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=4 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=22 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134353
RUN 24 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=2 WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134418
RUN 25 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134442
RUN 26 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=21 PAGES=7 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134506
RUN 27 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.1 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134530
RUN 28 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134536
RUN 29 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=1 WINDOW-UPLOADS=2 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134600
RUN 30 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=6 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=18 PAGES=6 DELETES=5 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134624
RUN 31 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=5 WINDOW-UPLOADS=4 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=16 PAGES=5 DELETES=4 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134648
RUN 32 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=22 PAGES=6 DELETES=6 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134713
RUN 33 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=5 BACKUP-S=1.7 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134738
RUN 34 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=19 PAGES=5 DELETES=5 BACKUP-S=1.5 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134802
RUN 35 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=1 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134827
RUN 36 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=6 BACKUP-S=1.3 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-134852
RUN 37 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.7 RESTORED=yes NAMED=- WINDOW-NAME=- RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134858
RUN 38 deletes CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=0 MISSING=3 ORPHAN=6 WINDOW-UPLOADS=4 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=17 PAGES=6 DELETES=5 BACKUP-S=1.7 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134924
RUN 39 deletes CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=3 WINDOW-PAGES=0 WINDOW-DELETES=1 UPLOADS=23 PAGES=6 DELETES=6 BACKUP-S=1.6 RESTORED=yes NAMED=yes WINDOW-NAME=overlaps-snapshot-step RSYNC-STATUS=- DIR=/opt/holzcloud/backups/20261026-134951
RUN 40 deletes CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=3 WINDOW-PAGES=1 WINDOW-DELETES=1 UPLOADS=20 PAGES=6 DELETES=5 BACKUP-S=1.1 RESTORED=no NAMED=yes WINDOW-NAME=overlaps-snapshot-step,rsync-24 RSYNC-STATUS=24 DIR=/opt/holzcloud/backups/20261026-135015
```

#### amd64-t25-src/consist-run-1-backup.txt

```text
1793020852226 === holzcloud-CMS backup: 20261026-132052 ===
1793020852311 Backing up database...
1793020852458 2026/10/26 13:20:52 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793020852504 2026/10/26 13:20:52 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-132052/holzcloud.sqlite
1793020852523 2026/10/26 13:20:52 INFO backup verified path=/opt/holzcloud/backups/20261026-132052/holzcloud.sqlite tables=57
1793020852546 backup written and verified: /opt/holzcloud/backups/20261026-132052/holzcloud.sqlite
1793020852565 Backing up CSRF key...
1793020852586 Backing up media...
1793020852796 Backing up plugins...
1793020852916 Backing up sprachen...
1793020853019 Backing up marke...
1793020853316 Pruning backups older than 30 days...
1793020853342 === Backup complete: /opt/holzcloud/backups/20261026-132052 ===
1793020853367 121M	/opt/holzcloud/backups/20261026-132052
WALL 1793020852052 1793020853388
RC 0
```

#### amd64-t25-src/consist-run-1-restore.txt

```text
1793020858065 === Restoring from /opt/holzcloud/backups/20261026-132052 ===
1793020858083 timestamp=20261026-132052
1793020858105 version=holzcloud v0.1.0-46-ge86d490 (e86d490)
1793020858126 data_dir=/opt/holzcloud/data
1793020858145 host=dbfcb235cb73
1793020858166 Verifying snapshot...
1793020858261 2026/10/26 13:20:58 INFO sqlite opened journal_mode=wal path=/tmp/tmp.VkkAXumNeg/holzcloud.sqlite
1793020858289 integrity: ok
1793020858306 Stopping holzcloud...
1793020858387 Moving current data to /opt/holzcloud/data.before-restore-20261026-132058
1793020858418 Restoring database...
1793020858449 Restoring CSRF key...
1793020858470 Restoring media...
1793020858693 Restoring plugins...
1793020858821 Restoring sprachen...
1793020858929 Restoring marke...
1793020859064 Starting holzcloud...
1793020859118 Waiting for readiness...
1793020869837 === Restore complete ===
1793020869885 {"status":"ok","version":"v0.1.0-46-ge86d490","commit":"e86d490","uptime_s":0,"integrity":"ok","disk_free_bytes":1821754814464,"database":{"freelist_count":0,"page_count":165,"size_bytes":675840,"wal_bytes":20632}}
1793020869901 
WALL 1793020857919 1793020869923
RC 0
```

#### amd64-t25-src/consist-run-1-compare.txt

```text
RUN: adds 1 restored=yes
BACKUP-DIR: /opt/holzcloud/backups/20261026-132052
ROWS: 39
FILES: 44
MISSING: 0
ORPHAN: 5
EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)
CLASS: ORPHAN-ONLY
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea-medium.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea-thumb.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w2-3.jpg 1793020852219 1793020852625) file 1/39bb01c984583b2c810ff666d72a1eea.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w1-3.jpg 1793020852262 1793020852683) file 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
WINDOW-NAME run adds 1: overlaps-snapshot-step (upload churn-adds1-w1-3.jpg 1793020852262 1793020852683) file 1/952490050431668bc1e8d5fcf5571c55.jpg
INFO disagreement: 0 rows without a file, 5 files without a row (class ORPHAN-ONLY)
KEY ORPHAN-FILE: 1/39bb01c984583b2c810ff666d72a1eea-medium.jpg
KEY ORPHAN-FILE: 1/39bb01c984583b2c810ff666d72a1eea-thumb.jpg
KEY ORPHAN-FILE: 1/39bb01c984583b2c810ff666d72a1eea.jpg
KEY ORPHAN-FILE: 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
KEY ORPHAN-FILE: 1/952490050431668bc1e8d5fcf5571c55.jpg
PASS holzcloud check on a copy: 2026/10/26 13:21:10 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.RtK6st/holzcloud.sqlite integrity: ok
PASS PRAGMA integrity_check: ok
PASS PRAGMA foreign_key_check: no violation
INFO PAGE-REFS: 1 media files named by page HTML
PASS PAGE-REFS-DANGLING: 0
PASS the sha256 list of the restored tree (77 files) equals that of the backup directory it came from
```

#### amd64-t25-src/consist-deletes-run-3-backup.txt

```text
1793021845318 === holzcloud-CMS backup: 20261026-133725 ===
1793021845414 Backing up database...
1793021845562 2026/10/26 13:37:25 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793021845608 2026/10/26 13:37:25 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-133725/holzcloud.sqlite
1793021845629 2026/10/26 13:37:25 INFO backup verified path=/opt/holzcloud/backups/20261026-133725/holzcloud.sqlite tables=57
1793021845648 backup written and verified: /opt/holzcloud/backups/20261026-133725/holzcloud.sqlite
1793021845667 Backing up CSRF key...
1793021845694 Backing up media...
1793021846085 Backing up plugins...
1793021846208 Backing up sprachen...
1793021846311 Backing up marke...
1793021846584 Pruning backups older than 30 days...
1793021846607 === Backup complete: /opt/holzcloud/backups/20261026-133725 ===
1793021846640 159M	/opt/holzcloud/backups/20261026-133725
WALL 1793021845128 1793021846672
RC 0
```

#### amd64-t25-src/consist-deletes-run-3-restore.txt

```text
1793021852113 === Restoring from /opt/holzcloud/backups/20261026-133725 ===
1793021852132 timestamp=20261026-133725
1793021852150 version=holzcloud v0.1.0-46-ge86d490 (e86d490)
1793021852169 data_dir=/opt/holzcloud/data
1793021852192 host=dbfcb235cb73
1793021852209 Verifying snapshot...
1793021852310 2026/10/26 13:37:32 INFO sqlite opened journal_mode=wal path=/tmp/tmp.assqMsbnRF/holzcloud.sqlite
1793021852341 integrity: ok
1793021852358 Stopping holzcloud...
1793021852469 Moving current data to /opt/holzcloud/data.before-restore-20261026-133732
1793021852502 Restoring database...
1793021852534 Restoring CSRF key...
1793021852554 Restoring media...
1793021852891 Restoring plugins...
1793021853016 Restoring sprachen...
1793021853123 Restoring marke...
1793021853269 Starting holzcloud...
1793021853316 Waiting for readiness...
1793021864030 === Restore complete ===
1793021864081 {"status":"ok","version":"v0.1.0-46-ge86d490","commit":"e86d490","uptime_s":0,"integrity":"ok","disk_free_bytes":1815519780864,"database":{"freelist_count":0,"page_count":331,"size_bytes":1355776,"wal_bytes":0}}
1793021864099 
WALL 1793021851966 1793021864123
RC 0
```

#### amd64-t25-src/consist-deletes-run-3-compare.txt

```text
RUN: deletes 3 restored=yes
BACKUP-DIR: /opt/holzcloud/backups/20261026-133725
ROWS: 2436
FILES: 2433
MISSING: 3
ORPHAN: 0
EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)
CLASS: ROW-WITHOUT-FILE
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55-medium.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
WINDOW-NAME run deletes 3: overlaps-snapshot-step (delete 15:1/952490050431668bc1e8d5fcf5571c55 1793021845435 1793021845772) file 1/952490050431668bc1e8d5fcf5571c55.jpg
INFO disagreement: 3 rows without a file, 0 files without a row (class ROW-WITHOUT-FILE)
KEY MISSING-FILE: 1/952490050431668bc1e8d5fcf5571c55-medium.jpg
KEY MISSING-FILE: 1/952490050431668bc1e8d5fcf5571c55-thumb.jpg
KEY MISSING-FILE: 1/952490050431668bc1e8d5fcf5571c55.jpg
PASS holzcloud check on a copy: 2026/10/26 13:37:44 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.PMtPrD/holzcloud.sqlite integrity: ok
PASS PRAGMA integrity_check: ok
PASS PRAGMA foreign_key_check: no violation
INFO PAGE-REFS: 1 media files named by page HTML
PASS PAGE-REFS-DANGLING: 0
PASS the sha256 list of the restored tree (2466 files) equals that of the backup directory it came from
```

#### amd64-t25-src/consist-deletes-run-9-backup.txt

```text
1793021990960 === holzcloud-CMS backup: 20261026-133950 ===
1793021991039 Backing up database...
1793021991193 2026/10/26 13:39:51 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793021991263 2026/10/26 13:39:51 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-133950/holzcloud.sqlite
1793021991285 2026/10/26 13:39:51 INFO backup verified path=/opt/holzcloud/backups/20261026-133950/holzcloud.sqlite tables=57
1793021991321 backup written and verified: /opt/holzcloud/backups/20261026-133950/holzcloud.sqlite
1793021991352 Backing up CSRF key...
1793021991386 Backing up media...
1793021991523 file has vanished: "/opt/holzcloud/data/media/1/ebb6b45536ff96db4b0d2eaf27c5f771-medium.jpg"
1793021991552 file has vanished: "/opt/holzcloud/data/media/1/ebb6b45536ff96db4b0d2eaf27c5f771-thumb.jpg"
1793021991571 file has vanished: "/opt/holzcloud/data/media/1/ebb6b45536ff96db4b0d2eaf27c5f771.jpg"
1793021991782 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1347) [sender=3.2.7]
WALL 1793021990756 1793021991805
RC 24
```

### R3 CON-01 window

#### amd64-t25-src/window.txt

```text
BACKUP-LINE: deploy/DEPLOY.md:657: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups (HOLZCLOUD_BIN=/opt/holzcloud/hc-hold-bin is put in front of the script through env, as backup.sh documents)
RESTORE-LINE: deploy/DEPLOY.md:971: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500 (yes on stdin)
WEBSITE: 1
HOLD-BOUND-S: 90 (a bounded wait in the stand-in; the driver removes the flag as soon as its write is done)
INFO MECHANISM (read from the code, not measured): an upload writes the file (internal/media/intake.go:163) before it inserts the row (internal/media/intake.go:177)
INFO MECHANISM (read from the code, not measured): the file is created straight under its final name (internal/media/upload.go:107), no temporary name and no rename, so a copy that meets it half written holds a partial file for which there is no row yet
INFO MECHANISM (read from the code, not measured): a delete removes the variant files (internal/media/usage.go:191), then the row (internal/media/usage.go:193), then the original (internal/media/usage.go:198); a snapshot taken after the first step has the row, a copy taken after the last step has no file
INFO MECHANISM (read from the code, not measured): the snapshot is VACUUM INTO (internal/db/maintain.go:81) run by the command line process 'holzcloud backup' (cmd/holzcloud/cli.go:282), not by the service; it is a consistent database file at one instant, taken before the media are copied
INFO MECHANISM (read from the code, not measured): backup.sh runs with set -e (deploy/backup.sh:2) and copies the media with rsync -a (deploy/backup.sh:83); an rsync status of 24, a file vanished during the copy, ends the script before the MANIFEST is written (deploy/backup.sh:91), and restore.sh refuses a directory without one
PASS the editor logs in for the controls: HTTP 303 /admin/
SEED-UPLOAD: not needed, 1339 churn uploads exist for control C2 to delete
CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE
CONTROL-C0-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C0-WRITE: nothing written
PASS control C0 (none): class AGREE, as expected
CONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY
CONTROL-C1-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C1-WRITE: uploaded window-c1-1793022643.jpg: HTTP 303 /admin/websites/1/media, row and file 1/644d394cd11b01cd8553d431a8c22944.jpg in the live site
ORPHAN-FILE-C1: 1/644d394cd11b01cd8553d431a8c22944-medium.jpg
ORPHAN-FILE-C1: 1/644d394cd11b01cd8553d431a8c22944-thumb.jpg
ORPHAN-FILE-C1: 1/644d394cd11b01cd8553d431a8c22944.jpg
PASS control C1 (upload): class ORPHAN-ONLY, as expected
CONTROL C2 ACTION=delete HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=3 ORPHAN=0 CLASS=ROW-WITHOUT-FILE EXPECTED=ROW-WITHOUT-FILE
CONTROL-C2-BACKUP-RC: 0 RESTORE-RC: 0 HOLD-BOUND-REACHED: no
CONTROL-C2-WRITE: deleted media 209 (1/5b8a46ab93e3f3e265fb6e8c08f79424.jpg): HTTP 303 /admin/websites/1/media, row gone, 0 files of it left in the live site
MISSING-FILE-C2: 1/5b8a46ab93e3f3e265fb6e8c08f79424-medium.jpg
MISSING-FILE-C2: 1/5b8a46ab93e3f3e265fb6e8c08f79424-thumb.jpg
MISSING-FILE-C2: 1/5b8a46ab93e3f3e265fb6e8c08f79424.jpg
PASS control C2 (delete): class ROW-WITHOUT-FILE, as expected
C2-CONSEQUENCE media route, file the backup lacks (GET /media/1/5b8a46ab93e3f3e265fb6e8c08f79424.jpg, Host test.local): HTTP status and body length: 404 19
C2-CONSEQUENCE media route, file the backup has (GET /media/1/298394ffb25e21bf962eb03e70c386c8.jpg, Host test.local): HTTP status and body length: 200 21611
C2-CONSEQUENCE holzcloud check on a copy of the restored database: exit status 0 and output: 2026/10/26 13:51:25 INFO sqlite opened journal_mode=wal path=/root/.hc-cmp.yW8lXQ/holzcloud.sqlite integrity: ok
C2-CONSEQUENCE restore.sh exit status 0, its last line (empty lines skipped): {"status":"ok","version":"v0.1.0-46-ge86d490","commit":"e86d490","uptime_s":0,"integrity":"ok","disk_free_bytes":1809618100224,"database":{"freelist_count":0,"page_count":456,"size_bytes":1867776,"wal_bytes":0}}
C2-CONSEQUENCE editor login on the restored site: HTTP 303 /admin/
C2-CONSEQUENCE media list of the admin (editor session, GET /admin/websites/1/media): HTTP 200 -
C2-CONSEQUENCE restored site: INFO PAGE-REFS: 1 media files named by page HTML
C2-CONSEQUENCE restored site: PASS PAGE-REFS-DANGLING: 0
PASS the stand-in, the flag and the bound marker are gone from holzcloud-t25-src
PASS no process of the stand-in is left in holzcloud-t25-src
WINDOW OK
```

### R4 CON-03 capfull

#### amd64-t25-src/capfull.txt

```text
SHIPPED-MIN-FREE-MB: 1024 (read from /opt/holzcloud/deploy/backup.sh)
MIN-FREE-MB-IN-ENVIRONMENT: '' (empty: backup.sh runs with its default); unit Environment: DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30
UNIT-DROPINS: '' (empty: the shipped unit as it is)
BACKUP-LINE: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
BINARY: holzcloud v0.1.0-46-ge86d490 (e86d490)
HOST-ARCH: aarch64
CONTAINER-ARCH: x86_64
EARLIER-BACKUPS-BEFORE: 66 (20261026-131946 20261026-132013 20261026-132052 20261026-132115 20261026-132244 20261026-132311 20261026-132348 20261026-132410 20261026-132431 20261026-132517 20261026-132540 20261026-132602 20261026-132624 20261026-132709 20261026-132732 20261026-132819 20261026-132841 20261026-132904 20261026-132949 20261026-133013 20261026-133035 20261026-133107 20261026-133131 20261026-133218 20261026-133242 20261026-133352 20261026-133439 20261026-133526 20261026-133550 20261026-133614 20261026-133638 20261026-133702 20261026-133725 20261026-133749 20261026-133813 20261026-133901 20261026-133926 20261026-133950 20261026-134020 20261026-134044 20261026-134107 20261026-134130 20261026-134136 20261026-134201 20261026-134206 20261026-134230 20261026-134237 20261026-134300 20261026-134324 20261026-134347 20261026-134353 20261026-134418 20261026-134442 20261026-134530 20261026-134600 20261026-134624 20261026-134648 20261026-134713 20261026-134827 20261026-134852 20261026-134924 20261026-134951 20261026-135015 20261026-135022 20261026-135043 20261026-135103)
BULK: BULK-MB: 1150 FILES: 230 BYTES: 1205874688 MEDIA-BYTES: 1259808041 
FLOOR-MB: 1024
DATA-DIR-BYTES: 1286551204 (everything below /opt/holzcloud/data, the pre-upgrade snapshots included, which a backup does not carry)
SOURCE-TREES: bytes 1279292860 files 4262 (media, templates, plugins, sprachen, marke)
DATA-MB: 1222 (what a backup has to write: those trees, the database file and csrf.key)
FS-SIZE-MB: 1064
PASS capfull_data_ok: the filesystem of 1064 MB is at least the floor (1024 MB) and smaller than the 1222 MB a backup has to write
CONTROL-FS-SIZE-MB: 2646
CONTROL-FREE-AT-GUARD-MB: 2646
CONTROL-PARTIAL-DIR-TREES: bytes 1279292860 files 4262 in the five directories, the source holds 1279292860 bytes in 4262 files
CONTROL-GUARD: passed
CONTROL-BACKUP-RC: 0
CONTROL-RSYNC-STATUS: -
CONTROL-NO-SPACE-LINES: 0
CONTROL-DIRECTORY-MADE: 20261026-135137
CONTROL-DIRECTORY-BYTES: 1281246788
CONTROL-DIRECTORY-FILES: 4265
CONTROL-MANIFEST: yes
CONTROL-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
CONTROL-SENTINEL-INTACT: none-planted
CONTROL-CLASS: COMPLETE
CONTROL-LAST-LINES: 1793022698166 Backing up CSRF key...
CONTROL-LAST-LINES: 1793022698185 Backing up media...
CONTROL-LAST-LINES: 1793022699760 Backing up plugins...
CONTROL-LAST-LINES: 1793022699890 Backing up sprachen...
CONTROL-LAST-LINES: 1793022699995 Backing up marke...
CONTROL-LAST-LINES: 1793022700209 Pruning backups older than 30 days...
CONTROL-LAST-LINES: 1793022700232 === Backup complete: /opt/holzcloud/backups/20261026-135137 ===
CONTROL-LAST-LINES: 1793022700250 1.3G	/opt/holzcloud/backups/20261026-135137
CONTROL-LAST-LINES: WALL 1793022697690 1793022700272
CONTROL-LAST-LINES: RC 0
PASS the control completes on a roomy filesystem: exit 0, '=== Backup complete ===', all of the source
CONTROL-CLASS: COMPLETE
SENTINEL: /opt/holzcloud/backups/20261025-135141 sha256 563909c09c8d061109e818c9b3c0605158e17477e1941267f19c1df39fafc788
FREE-AT-GUARD-MB: 1064 (df -Pm at the backup root just before the run, the way backup.sh reads it; the floor is 1024)
HAND-PARTIAL-DIR-TREES: bytes 1096963369 files 4219 in the five directories, the source holds 1279292860 bytes in 4262 files
HAND-GUARD: passed
HAND-BACKUP-RC: 11
HAND-RSYNC-STATUS: 11
HAND-NO-SPACE-LINES: 1
HAND-DIRECTORY-MADE: 20261026-135143
HAND-DIRECTORY-BYTES: 1098915817
HAND-DIRECTORY-FILES: 4221
HAND-MANIFEST: no
HAND-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
HAND-SENTINEL-INTACT: yes
HAND-CLASS: FAILED-PARTIAL-DIR-LEFT
HAND-LAST-LINES: 1793022703382 2026/10/26 13:51:43 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-135143/holzcloud.sqlite
HAND-LAST-LINES: 1793022703401 2026/10/26 13:51:43 INFO backup verified path=/opt/holzcloud/backups/20261026-135143/holzcloud.sqlite tables=57
HAND-LAST-LINES: 1793022703422 backup written and verified: /opt/holzcloud/backups/20261026-135143/holzcloud.sqlite
HAND-LAST-LINES: 1793022703439 Backing up CSRF key...
HAND-LAST-LINES: 1793022703456 Backing up media...
HAND-LAST-LINES: 1793022704858 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261026-135143/media/1/bulk/bulk-71.bin": No space left on device (28)
HAND-LAST-LINES: 1793022704875 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
HAND-LAST-LINES: 1793022704973 rsync: [sender] write error: Broken pipe (32)
HAND-LAST-LINES: WALL 1793022702955 1793022704995
HAND-LAST-LINES: RC 11
GUARD: passed
BACKUP-RC: 11
PARTIAL-DIR: 20261026-135143
PARTIAL-DIR-BYTES: 1098915817
MANIFEST-IN-PARTIAL: no
SENTINEL-INTACT: yes
HAND-CLASS: FAILED-PARTIAL-DIR-LEFT
INFO backup.sh on the nearly full filesystem: FAILED-PARTIAL-DIR-LEFT (a measured property of the shipped script)
AGAIN-FREE-AT-GUARD-MB: 5
AGAIN-GUARD: refused
AGAIN-BACKUP-RC: 1
AGAIN-RSYNC-STATUS: -
AGAIN-NO-SPACE-LINES: 0
AGAIN-DIRECTORY-MADE: none
AGAIN-DIRECTORY-BYTES: 0
AGAIN-DIRECTORY-FILES: 0
AGAIN-MANIFEST: no
AGAIN-DATABASE-SNAPSHOT: absent - - (present, holzcloud check status, integrity_check)
AGAIN-SENTINEL-INTACT: yes
AGAIN-CLASS: GUARD-REFUSED
AGAIN-LAST-LINES: 1793022707174 === holzcloud-CMS backup: 20261026-135147 ===
AGAIN-LAST-LINES: 1793022707221 ERROR: only 5 MB free at /opt/holzcloud/backups, need 1024 MB
AGAIN-LAST-LINES: WALL 1793022706993 1793022707245
AGAIN-LAST-LINES: RC 1
AGAIN-CLASS: GUARD-REFUSED
RESTORE-PARTIAL-LINE: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20261026-135143 (no is typed on stdin)
RESTORE-PARTIAL-EXIT: 1
RESTORE-PARTIAL-ANSWER: ERROR: /opt/holzcloud/backups/20261026-135143/MANIFEST not found — the backup is incomplete, do not restore it|
SERVICE-ACTIVE-AFTER-RESTORE-PARTIAL: active
DATA-BEFORE-RESTORE-DIRECTORIES: 0
PASS restore.sh on the partial directory with no on stdin replaced nothing: the service stayed active and no data.before-restore directory appeared
UNIT-FREE-AT-GUARD-MB: 1064
UNIT-INVOCATION: ff75ab04d0364587b211f9b0085bc058 (started by systemctl start, previous invocation 'none')
UNIT-ACTIVE-STATE: failed
UNIT-RESULT: exit-code
UNIT-EXECMAINSTATUS: 11
UNIT-TRIGGEREDBY: 'holzcloud-backup.timer'
UNIT-JOURNAL-LINES: 11
UNIT-PARTIAL-DIR-TREES: bytes 1096963369 files 4219 in the five directories, the source holds 1279292860 bytes in 4262 files
UNIT-GUARD: passed
UNIT-BACKUP-RC: 11
UNIT-RSYNC-STATUS: 11
UNIT-NO-SPACE-LINES: 1
UNIT-DIRECTORY-MADE: 20261026-135149
UNIT-DIRECTORY-BYTES: 1098915817
UNIT-DIRECTORY-FILES: 4221
UNIT-MANIFEST: no
UNIT-DATABASE-SNAPSHOT: present 0 ok (present, holzcloud check status, integrity_check)
UNIT-SENTINEL-INTACT: yes
UNIT-CLASS: FAILED-PARTIAL-DIR-LEFT
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up database...
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-135149/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO backup verified path=/opt/holzcloud/backups/20261026-135149/holzcloud.sqlite tables=57
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: backup written and verified: /opt/holzcloud/backups/20261026-135149/holzcloud.sqlite
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up CSRF key...
UNIT-LAST-LINES: 2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up media...
UNIT-LAST-LINES: 2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55469]: rsync: [receiver] write failed on "/opt/holzcloud/backups/20261026-135149/media/1/bulk/bulk-71.bin": No space left on device (28)
UNIT-LAST-LINES: 2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55469]: rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
UNIT-LAST-LINES: 2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55468]: rsync: [sender] write error: Broken pipe (32)
UNIT-CLASS: FAILED-PARTIAL-DIR-LEFT
INFO backup.sh runs set -e and its retention step (find -mtime +RETENTION_DAYS) comes after the copies, so a run that ends in the copy never reaches it; that is a reading of the script. What is on disk is in the lines above: the sentinel of an earlier night, which is inside the retention window, shows nothing about deletion
EARLIER-BACKUPS-AFTER: 66 (20261026-131946 20261026-132013 20261026-132052 20261026-132115 20261026-132244 20261026-132311 20261026-132348 20261026-132410 20261026-132431 20261026-132517 20261026-132540 20261026-132602 20261026-132624 20261026-132709 20261026-132732 20261026-132819 20261026-132841 20261026-132904 20261026-132949 20261026-133013 20261026-133035 20261026-133107 20261026-133131 20261026-133218 20261026-133242 20261026-133352 20261026-133439 20261026-133526 20261026-133550 20261026-133614 20261026-133638 20261026-133702 20261026-133725 20261026-133749 20261026-133813 20261026-133901 20261026-133926 20261026-133950 20261026-134020 20261026-134044 20261026-134107 20261026-134130 20261026-134136 20261026-134201 20261026-134206 20261026-134230 20261026-134237 20261026-134300 20261026-134324 20261026-134347 20261026-134353 20261026-134418 20261026-134442 20261026-134530 20261026-134600 20261026-134624 20261026-134648 20261026-134713 20261026-134827 20261026-134852 20261026-134924 20261026-134951 20261026-135015 20261026-135022 20261026-135043 20261026-135103)
PASS the filesystem is unmounted and the earlier backups are back, the same directories as before
CAPFULL OK
```

#### amd64-t25-src/capfull-hand.txt

```text
1793022703124 === holzcloud-CMS backup: 20261026-135143 ===
1793022703198 Backing up database...
1793022703335 2026/10/26 13:51:43 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
1793022703382 2026/10/26 13:51:43 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-135143/holzcloud.sqlite
1793022703401 2026/10/26 13:51:43 INFO backup verified path=/opt/holzcloud/backups/20261026-135143/holzcloud.sqlite tables=57
1793022703422 backup written and verified: /opt/holzcloud/backups/20261026-135143/holzcloud.sqlite
1793022703439 Backing up CSRF key...
1793022703456 Backing up media...
1793022704858 rsync: [receiver] write failed on "/opt/holzcloud/backups/20261026-135143/media/1/bulk/bulk-71.bin": No space left on device (28)
1793022704875 rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
1793022704973 rsync: [sender] write error: Broken pipe (32)
WALL 1793022702955 1793022704995
RC 11
```

#### amd64-t25-src/capfull-again.txt

```text
1793022707174 === holzcloud-CMS backup: 20261026-135147 ===
1793022707221 ERROR: only 5 MB free at /opt/holzcloud/backups, need 1024 MB
WALL 1793022706993 1793022707245
RC 1
```

#### amd64-t25-src/capfull-restore-partial.txt

```text
1793022707986 ERROR: /opt/holzcloud/backups/20261026-135143/MANIFEST not found — the backup is incomplete, do not restore it
WALL 1793022707832 1793022708010
RC 1
```

#### amd64-t25-src/capfull-unit.txt

```text
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: === holzcloud-CMS backup: 20261026-135149 ===
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up database...
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/data/holzcloud.sqlite
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO sqlite opened journal_mode=wal path=/opt/holzcloud/backups/20261026-135149/holzcloud.sqlite
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: 2026/10/26 13:51:49 INFO backup verified path=/opt/holzcloud/backups/20261026-135149/holzcloud.sqlite tables=57
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55456]: backup written and verified: /opt/holzcloud/backups/20261026-135149/holzcloud.sqlite
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up CSRF key...
2026-10-26T13:51:49+0000 dbfcb235cb73 backup.sh[55448]: Backing up media...
2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55469]: rsync: [receiver] write failed on "/opt/holzcloud/backups/20261026-135149/media/1/bulk/bulk-71.bin": No space left on device (28)
2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55469]: rsync error: error in file IO (code 11) at receiver.c(401) [receiver=3.2.7]
2026-10-26T13:51:51+0000 dbfcb235cb73 backup.sh[55468]: rsync: [sender] write error: Broken pipe (32)
```

#### amd64-t25-src/capfull-df.txt

```text
== control before (2646 MB) (2026-10-26T13:51:37Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2646     0      2646       0% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2709504k,mode=700,uid=997,gid=997 0 0
== control after (2026-10-26T13:51:40Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    2646  1234      1413      47% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=2709504k,mode=700,uid=997,gid=997 0 0
== hand before (1064 MB) (2026-10-26T13:51:42Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== hand after (2026-10-26T13:51:45Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064  1060         5     100% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== again before (2026-10-26T13:51:46Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064  1060         5     100% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== again after (2026-10-26T13:51:47Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064  1060         5     100% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== unit before (1064 MB) (2026-10-26T13:51:48Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064     1      1064       1% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
== unit after (2026-10-26T13:51:56Z) ==
Filesystem     1048576-blocks  Used Available Capacity Mounted on
tmpfs                    1064  1060         5     100% /opt/holzcloud/backups
tmpfs /opt/holzcloud/backups tmpfs rw,relatime,size=1089536k,mode=700,uid=997,gid=997 0 0
```

### R5 CON-02 compat

#### amd64-t25-old/compat.txt

```text
COMPAT-TAG: v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712)
NEW-REF: e86d490
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
BACKUP-SH-SHA256-NEW-REF: d9f807d806a61ef3a9920890394247b3ecd475a17b92de4929ad517e2c17eba2
BACKUP-SH-SHA256-INSTALLED: d9f807d806a61ef3a9920890394247b3ecd475a17b92de4929ad517e2c17eba2
BACKUP-SH: identical
BACKUP-SH-TAG-EQUALS-NEW-REF: yes
BACKUP-LINE: sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
BACKUP-RC: 0
BACKUP-DIR: /opt/holzcloud/backups/20261026-135313
MANIFEST: timestamp=20261026-135313 version=holzcloud v0.1.0 (4d238b9) data_dir=/opt/holzcloud/data host=9ca9ccbe0c9e
PASS the MANIFEST of the backup names the tag: version=holzcloud v0.1.0 (4d238b9)
NEW-BINARY: holzcloud v0.1.0-46-ge86d490 (e86d490)
SETUP-BEFORE: HTTP 200 -
PASS the setup form answers 200 on the fresh box before the restore
PASS transfer: FILES: 27 files, sha256 equal on both sides
RESTORE-LINE: sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20261026-135313
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
SET-ASIDE: /opt/holzcloud/data.before-restore-20261026-135348 users 0
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
MEDIA-SERVED: 1/78a53bc2da5afe3fde1396c7181d99ba.jpg source=925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577 served=925d26e12cb8ed51e272c6cadba3f6fb5d32f4af8bf06d1d2123ddbf8a176577
MEDIA-SERVED: 1/a59bfc266a99ef83577c33d42bfa2356.jpg source=878ec3c76d60a504b602bd46db8ff9357581e173377dd5e5dedc9ad0bdeb3a40 served=878ec3c76d60a504b602bd46db8ff9357581e173377dd5e5dedc9ad0bdeb3a40
MEDIA-SERVED: 1/c3f7dd0305ab5260444ab59baaf59f5b.jpg source=1b0a15f61f5a34986b43fbb64e88b5e2947ffc257bdc82cac0f004aaa1a8c1cb served=1b0a15f61f5a34986b43fbb64e88b5e2947ffc257bdc82cac0f004aaa1a8c1cb
MEDIA-SERVED: 1/ee00d95ae52bcf14e66c3c2223954ddc.jpg source=2232b559930f6da4367de5d08d2b6ab0b6bfec2c8d094c9a29a5569dcdba94c1 served=2232b559930f6da4367de5d08d2b6ab0b6bfec2c8d094c9a29a5569dcdba94c1
MEDIA-SERVED-EQUAL: 4 of 4
PASS every media original is served over HTTP with the sha256 of the source's file (4 of 4)
MIGRATIONS-AT-TAG: 60
MIGRATIONS-HIGHEST-AT-TAG: 60
MIGRATIONS-AT-NEW-REF: 60
MIGRATIONS-HIGHEST-AT-NEW-REF: 60
LEVEL-BEFORE: 60
LEVEL-AFTER: 60
PASS migration_ok: before 60, after 60, highest at e86d490 60
MIGRATION-SENTENCE: No migration separates the two releases: the backup is at level 60, the restored database is at level 60, the restore ran none.
GOOSE-ROWS-BEFORE: 61
GOOSE-ROWS-AFTER: 61
INFO this pair crosses 0 migration(s) by the levels read; a pair with no pending migration does not show how a restore handles one, that stays on the Not tested list of the record and migration_ok's table is what keeps the judgement right when a later release does cross one
COMPAT-DIFF-COMMAND: git diff --stat=200 v0.1.0 e86d490 -- go.mod go.sum internal/db internal/media cmd
COMPAT-DIFF  cmd/holzcloud/assets/admin.css                       |  18 +++++++++++
COMPAT-DIFF  cmd/holzcloud/assets/bausteine.css                   |  18 +++++++++--
COMPAT-DIFF  cmd/holzcloud/main.go                                |  21 ++++++++++++
COMPAT-DIFF  cmd/holzcloud/main_test.go                           |   2 ++
COMPAT-DIFF  cmd/holzcloud/templates/admin/block_list.html        |   8 +++--
COMPAT-DIFF  cmd/holzcloud/templates/admin/media_scale.html       |  41 +++++++++++++++++++++++
COMPAT-DIFF  cmd/holzcloud/templates/public/holzcloud/layout.html |   2 +-
COMPAT-DIFF  cmd/holzcloud/templates/public/holzcloud/style.css   |  33 ++++++++++++-------
COMPAT-DIFF  cmd/holzcloud/templates/public/rudel/style.css       |   6 +++-
COMPAT-DIFF  cmd/holzcloud/templates/public/weide/style.css       |   5 ++-
COMPAT-DIFF  go.mod                                               |  22 ++++++-------
COMPAT-DIFF  go.sum                                               |  60 +++++++++++++++++-----------------
COMPAT-DIFF  internal/media/scale.go                              | 183 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/scale_test.go                         | 248 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/staging.go                            | 219 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  internal/media/staging_test.go                       | 167 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
COMPAT-DIFF  16 files changed, 992 insertions(+), 61 deletions(-)
SCRIPTS-IDENTICAL: deploy/backup.sh yes d9f807d806a61ef3a9920890394247b3ecd475a17b92de4929ad517e2c17eba2
SCRIPTS-IDENTICAL: deploy/restore.sh yes 9a411d72ca2c3c8dcb081915af91c46f743de7f79373296d8cc4bdf905a59633
SCRIPTS-IDENTICAL: deploy/holzcloud-backup.service yes befdd74bd8233c7922755824bdea6b60aa0bb0beb0c83ba0982562cdc7e93bf7
SCRIPTS-IDENTICAL: deploy/holzcloud-backup.timer yes 87ccb6d9ec499469994b1ef0aa020bb6e3a9df140dd71b19169f4193ff9e88b0
CHECK: 17 of 17
CHECK-BAD-LINES: 0
PASS run.sh check on the restored box: 17 of 17 assertions, the five plugins loaded, running and invoked
COMPAT OK
```

#### amd64-t25-old/compat-binary.txt

```text
== old: v0.1.0 ==
file:       deploy/test/out/bin/holzcloud-old-v0.1.0
describe:   v0.1.0
commit:     4d238b9
sha256:     83d5ba08ad2bde11b65c02271a13a867082c0071c81cd7b69bdf213f384ae2d3
migrations: 60
container-version: holzcloud v0.1.0 (4d238b9)
```

#### amd64-t25-new/transfer.txt

```text
DIR: /opt/holzcloud/backups/20261026-135313
SOURCE: holzcloud-t25-old:/opt/holzcloud/backups/20261026-135313
FILES: 27
TRANSFER: equal
```
