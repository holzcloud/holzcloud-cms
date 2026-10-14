# Phase 22 findings: pass 1 of the backup and restore proof

Pass 1 is the whole chain played against `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md` and the three unit files exactly as they stood when the phase began (commit 5de2b17). Nothing here is a repair; plan 22-04 repairs from this list. The raw evidence is in the gitignored `deploy/test/out/amd64-bak-pass1-*`; every quote below is a verbatim line of it.

## Pass 1 runs

- **Command** (one call, three fresh containers: the source, the seed-point target and the bulk-point target):

  ```text
  PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=b687738 \
    NAME=holzcloud-bak-pass1-src OUT_DIR=deploy/test/out/amd64-bak-pass1-src \
    TGT_NAME=holzcloud-bak-pass1-tgt TGT_OUT_DIR=deploy/test/out/amd64-bak-pass1-tgt \
    ./deploy/test/run.sh backup-chain      # BULK_MB left at the chain's default of 300
  ```

  Output: `amd64-bak-pass1.log`, `-src`, `-tgt`, `-src-bulk`, `-tgt-bulk`. The chain exited 1, as expected with FAIL verdicts, after `summary.txt` was written.
- **Commit it ran on:** `b687738` (`git rev-parse --short HEAD`); the six specimen files equal commit 5de2b17 and HEAD before and after the run.
- **Date:** 2026-10-14 (UTC), 10:31 to 10:36.
- **Build the binary reports:** `holzcloud v0.0.25-63-gb687738 (b687738)` (the `MANIFEST` and `READYZ` lines of both targets).
- **Host and container architecture** (`backup.txt`): `HOST-ARCH: aarch64`, `CONTAINER-ARCH: x86_64`; `versions.txt`: Docker Desktop, kernel `7.0.14-linuxkit`, container systemd 252 on Debian 12 (bookworm). So every time below comes from an amd64 emulation on an aarch64 host.
- **Wall time of the whole chain:** 249 seconds (images and the Go build were cached).
- **Verdicts** (`summary.txt`): `BAK-01 FAIL`, `BAK-02 FAIL`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`, `STEPS: 76 played, 0 failed`, `LEAK-CHECK OK`. Every FAIL is the one class below (F-01 to F-03).
- **Investigation run, not part of pass 1.** Four questions could not be answered from the pass-1 files alone (A2, A3, D-10, stale `-wal`/`-shm`). The chain was played a second time with `BULK_MB=5` and `KEEP=1` under the names `holzcloud-bak-inv-*` (scratch `amd64-bak-inv3-src`, `-tgt`, `-src-bulk`, `-tgt-bulk`, `amd64-bak-inv3.log`), it gave the same verdicts and the same backup size (715976 bytes), and the questions were asked inside the kept target with `docker exec`. The answers are in `deploy/test/out/amd64-bak-inv3-investigation.txt` (E1 to E5, quoted below by number). The three containers were removed with `run.sh down`; the pass-1 files are untouched.

## Findings

One row per distinct defect. `What happened` is quoted from the scratch files; E-numbers are in the investigation file. Rows F-01 to F-03 account for every BAD line of pass 1 (the block at the end). A reading of the scripts that no run has played is marked as such under the table, not in it.

| ID | Where | The script or document said | What happened | Planned fix | Disposition |
|----|-------|-----------------------------|---------------|-------------|-------------|
| F-01 | `deploy/backup.sh` steps 1 to 4; `verify-backup.txt` and `verify-timer.txt` of the seed point, `verify-backup.txt` of the bulk point | The script copies the database, `csrf.key`, `media/` and `templates/` and nothing else (header comment and steps 1 to 4); DEPLOY.md Backups: "containing the database, the CSRF key, media and user templates" | The whole-directory comparison finds the plugin tree, the language directory and the logo directory only in the source, twelve lines in each of the three files: `BAD  tree src-backup: only in A: ./plugins/bestellung/plugin.wasm`, `BAD  tree src-backup: only in A: ./sprachen/nl.json`, `BAD  tree src-backup: only in A: ./marke/logo.svg` (the timer's backup says `src-timer`). The ten plugin files are `plugin.json` and `plugin.wasm` of five plugins | One loop over the five directory names media, templates, plugins, sprachen and marke, each guarded by a directory test and announced with an `echo`; keep the top-level names so an older backup still restores | fix script |
| F-02 | `deploy/restore.sh` media and templates steps; `restore.txt` of both targets | DEPLOY.md Restore from a backup: "restores database, CSRF key, media and templates" | The restore brings back what the backup holds and no more, and also has a list of four: `COMES-BACK: csrf.key holzcloud.sqlite media`. The whole-directory comparison of source against restored target: `BAD  tree src-restored: only in A: ./plugins/bestellung/plugin.wasm`, `BAD  tree src-restored: only in A: ./sprachen/nl.json`, `BAD  tree src-restored: only in A: ./marke/logo.svg` (twelve lines in each target). Repairing only `backup.sh` would leave this | The same loop in `restore.sh` over the same five names, each guarded and announced | fix script |
| F-03 | The restored site: `compare-src-tgt.txt`, `restore.txt`, `check.txt` of both targets; E1 | DEPLOY.md: the script "waits for `/readyz`", and `/readyz` is the sign of a restored site | `/readyz` is green while the five plugins are down: `READYZ: {"status":"ok","version":"v0.0.25-63-gb687738","commit":"b687738","integrity":"ok"}`, then `BAD plugins running a=5 b=0 (5 expected)` and `BAD  check: 1 of 17 assertions passed (exit status 1), see check.txt; the first BAD line is: loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC`. E1 on the restored target: `"enabled":true,"running":false,"last_error":"the module is missing from disk: open /opt/holzcloud/data/plugins/bestellung/plugin.wasm: no such file or directory"` for all five. The plugin rows came back, enabled, with nothing to run | No change of its own: the consequence disappears with F-01 and F-02; the proof is `plugins running 5/5` and 17 of 17 assertions on the restored target. Readiness does not look at plugins and is not changed here | fix script |
| F-04 | `docs/deployment.md` Backups and `docs/configuration.md` The data directory | "Back up the whole data directory; `/opt/holzcloud/deploy/backup.sh` does it WAL-safely." and "Back up this whole directory and you have everything. `deploy/backup.sh` does it WAL-safely" | The script did not do it: F-01. The source data directory of pass 1 holds `./plugins/bestellung/plugin.json` and `./marke/logo.svg` (`tree-src-backup.txt`); the tree in `docs/configuration.md` lists neither `plugins/` nor `marke/`, so a reader would not know the directories exist | After the script is repaired the claim holds except for two things the script leaves out on purpose, the pre-upgrade snapshots and the SQLite working files; say that, and add `plugins/` and `marke/` to the tree | fix document |
| F-05 | `deploy/DEPLOY.md`, Backups, the opening paragraph | It lists the database, the CSRF key, media and user templates and a `MANIFEST` naming the version | True of the script and incomplete for the site: F-01. It says nothing of what a backup does not hold (everything outside the data directory: unit file and drop-ins, environment files, the secret key, the proxy configuration; the restore proof's own `HAND-REBUILT: no environment file is configured on this unit; on a real server HOLZCLOUD_SECRET_KEY and payment or mail credentials live in one, never in the data directory, so no backup holds them`) and nothing of the order in which database and files are taken (see the non-atomic pair below) | List the five directories and the `MANIFEST`; add a paragraph on what a backup does not hold and one on the order of the database snapshot and the file copy | fix document |
| F-06 | `deploy/DEPLOY.md`, Restore from a backup | One command; the paragraph says the script verifies, moves aside, restores and waits for `/readyz` | The document does not mention the question the script asks, which anything but `yes` answers with `aborted` and exit 1 (`INFO case c output: aborted`), nor that a freshly installed box has its setup form open until the restore replaces the database (`SETUP-BEFORE: HTTP 200 -`, then `SETUP-AFTER: HTTP 404 -`: an account created there is replaced and lost), nor what comes back and what the operator rebuilds (`HAND-REBUILT: the unit file /etc/systemd/system/holzcloud.service (written by the install, no backup holds it)`), nor how a backup directory reaches a rebuilt box (`DEVIATION[transfer]`: the harness had to invent a tar stream) | Give the command, the `yes` question, the order of events, how the backup reaches a rebuilt box, that the setup form of the new box is left alone, what comes back and what is rebuilt by hand, and the measured size and time as an order of magnitude | fix document |
| F-07 | `deploy/restore.sh` `read -r -p`, `restore-run.txt` | The script asks `Stop holzcloud and replace /opt/holzcloud/data? [yes/NO]` | The question is absent from the transcript: `1791974064.218289542 Verifying snapshot...` is followed by `1791974064.303775543 Stopping holzcloud...` with nothing between, although `yes` was on stdin and the answer was taken. This is a harness fact, not a defect of the script or the document: `read -p` prints no prompt when stdin is not a terminal, and case c shows the script does ask (`aborted` on `no`) | None | no action |
| F-08 | `deploy/restore.sh` `cp` of the database; `restore.txt` | The restore keeps modes for the key and the media (`cp -p`, `rsync -a`); the database is copied with a plain `cp` | `INFO owner and mode of holzcloud.sqlite: source 'holzcloud 600', restored 'holzcloud 644' (the data directory is 700)`. The directory mode contains it, so nothing is exposed, but the restored file is wider than the original; `csrf.key` (600) and the media (600) are kept (`cp -p`, `rsync -a`) | One line: `chmod 600` on the restored database (no option, no variable) | fix script |
| F-09 | `deploy/restore.sh` move-aside; E4 | DEPLOY.md: "the previous state is still in `/opt/holzcloud/data.before-restore-<timestamp>`" | Each restore leaves one more whole copy of the replaced data, media included, and nothing removes it: `/opt/holzcloud/data.before-restore-20261014-104201: csrf.key holzcloud.sqlite media plugins` next to two more of the same kind. On a server with large media two restores hold three copies. The document names the directory and not that it stays until the operator deletes it | Say in DEPLOY.md that the set-aside directory is never removed, holds the media of the replaced site, and is deleted by hand once the restored site is accepted | fix document |
| F-10 | `deploy/restore.sh` precondition; E3, E3b | The script refuses a snapshot that fails its integrity check or that has no `holzcloud.sqlite`; the `MANIFEST` is read only if it exists (`[ -f MANIFEST ] && cat`) | A backup interrupted during the media copy has the database and the CSRF key, part of the media and no `MANIFEST` (`backup.sh` writes it last, step 5). `restore.sh` takes it: `restore.sh: === Restore complete ===`, exit status 0, `readyz status after: 200`, and `media files in the live data after: 2` where there were 3. Nothing was refused and a file is gone | `restore.sh` refuses a directory without `MANIFEST`, since its presence is the proof that every earlier step finished; no new field, no digest file (D-10, see its bullet) | fix script |
| F-11 | `deploy/test/README.md` (Output files, `deviations.txt`) against `deploy/test/run.sh` | The README: "A name outside the documented set fails the run" | `run.sh` has no such check; the only readers of the names are the summary's list, `DEVIATIONS: backup-name dropin fixture mount scp timer transfer`, and the chain's own distinct-kinds loop. Found in plan 22-02 and carried in `deferred-items.md` | Reword the README sentence to what the harness does, or add the check to `run.sh`; the smaller change is the sentence | fix harness |

**Read from the scripts, not played** (for 22-04 to play or reword; no row, because no output shows it):

- DEPLOY.md says "Do it once on a test directory before you need it". `restore.sh` takes `DATA_DIR` from the environment but runs `systemctl stop` and `systemctl start` on `SERVICE`, which is the production unit, and polls `127.0.0.1:8080/readyz`, which answers for the production data. A restore pointed at a test directory therefore stops the live site and proves nothing about the restored copy. The advice should name a second machine or container, which is what this phase did.
- `MIN_FREE_MB` is a fixed floor (1024) that does not grow with the size of the data, and `df` looks at the backup root only (see the bullet below).

## D-16 candidates looked at

- **Stale `-wal` and `-shm` after the restore: no defect.** `restore.txt` of both targets: `PASS set-aside: exactly one /opt/holzcloud/data.before-restore-20261014-103424`, and the INFO line that the whole directory moves aside, so the `-wal` and `-shm` of the replaced database go with it. E4 lists what the set-asides hold (`csrf.key holzcloud.sqlite media plugins`: no `-wal`, no `-shm`) and that the live directory holds `holzcloud.sqlite-shm` and `holzcloud.sqlite-wal` only because the service made them when it started (`INFO in the restored data directory and not from the backup, made by the service's own start: holzcloud.sqlite-shm holzcloud.sqlite-wal plugins`). The proof that no foreign log reached the restored file is that the restored database passes every row comparison (47 `PASS rows` lines, one per table that does not move with traffic, and `52 pass, 1 bad` over the whole comparison, the bad one being the plugins of F-03) and the integrity verdict (`PASS readyz status ok, integrity ok`).
- **Ownership and mode of the restored tree: ownership right, one mode wider (F-08).** `PASS owner and mode of /opt/holzcloud/data: holzcloud:holzcloud 700`, `PASS owner and mode of /opt/holzcloud/data/csrf.key: holzcloud:holzcloud 600`, the media file `holzcloud:holzcloud 600`, and the database `holzcloud:holzcloud 644` against 600 in the source. The `chown -R` of the script gives the service user the tree; the unit starts (`READYZ`) and logs in (`LOGIN: HTTP 303 /admin/`).
- **`MIN_FREE_MB` 1024 against the container's disk: never trips here, refuses when it should.** `backup.txt`: `FREE-MB: 354374`, and the bulk point `FREE-MB: 354072`, so 1024 is not close on a 354 GB emulated disk, which says nothing about a small server. Case d played `MIN_FREE_MB=99999999`: `ERROR: only 354372 MB free at /opt/holzcloud/backups, need 99999999 MB`, exit 1, `PASS case d: the number of directories under /opt/holzcloud/backups is unchanged (1): no directory was made`. Reasoning, labelled as such: the floor is a constant, so a site whose database and media together exceed the free space by less than 1024 MB passes the check and fails halfway (the truncated file the comment in the script warns about); the repair would need a size-dependent threshold or a new option, which D-16 does not allow here, so the fact goes into DEPLOY.md as a sentence (operator sets `MIN_FREE_MB` to more than the data directory holds) and no script change is made.
- **The non-atomic pair: a fact, reasoned, not played.** `backup.sh` takes the database snapshot first (step 1, `VACUUM INTO` by the binary) and copies the files afterwards (steps 2 to 4), with the site running. A file uploaded in between is in the backup although the snapshot does not know it: harmless. A file deleted in between can leave a row in the snapshot whose file the backup lacks. The first reading is the same in both data points (`PASS rows` in `verify-backup.txt`: all counts equal, `INFO volatile activity_log backup=22 source=21`, `INFO volatile sessions backup=4 source=3`, the tables that move with traffic), so the pair was consistent in a quiet run; the window itself was not provoked. The fact goes into the header comments of both scripts and into DEPLOY.md.
- **D-10: can a restore tell a truncated backup from a whole one without a new MANIFEST field or checksum file? For the database yes, for the rest no.** Yes for a damaged database: case a truncated `holzcloud.sqlite` from 663552 to 331776 bytes, the damage was proven real first (`INFO case a: DAMAGE-PROVEN truncated`), and `restore.sh` refused with `ERROR: the snapshot fails its integrity check — do not restore it`, exit 1, the eight invariants of the live data unchanged. Not for a flipped byte (A2: E2, both readers say `ok`). No for an incomplete directory: E3b, a backup without `MANIFEST` and with one media file missing restores with exit 0 and `=== Restore complete ===` (F-10). The minimal fix needs no new field and no digest file: `backup.sh` already writes `MANIFEST` last (step 5, after the database, the key, media and templates), so `restore.sh` refuses a directory without one, with a message that says the backup is incomplete. That covers an interrupted backup; it does not cover bit rot in a complete one (a flipped byte in a media file or in a database page), which only a checksum would, and that stays out of this phase by D-10.

## Assumptions of the research settled

- **A1: confirmed.** The plugin modules, the language files and the logo are not in the backup (F-01, `verify-backup.txt`), and in the restored site the five plugin rows are enabled and not running: `BAD plugins running a=5 b=0 (5 expected)` (`compare-src-tgt.txt`), 1 of 17 `check` assertions passed (`check.txt`), `"enabled":true,"running":false` with the module missing from disk (E1).
- **A2: confirmed.** A one-byte flip of a text value inside a data page is not detected: `integrity_check (immutable URI): ok`, `quick_check (immutable URI): ok` with sqlite3 3.40.1 and `integrity: ok` from the binary's own check, which exited 0, on the flipped copy (E2, E2b). Truncation is detected (case a, `restore-negative.txt`), which is why it is the case played. The choice of corruption was right.
- **A3: confirmed.** The key file `/root/.hc-key` is outside the data directory and so in no backup, the key itself is a row of `ai_tokens`, and after the restore `list_plugins` was answered for that key (`"error":null,"isError":null`, five plugins listed, E1) with `PASS rows ai_tokens` in `compare-src-tgt.txt`. So a working call after the restore proves the key table. In pass 1 alone the claim could not be read, because `state-tgt.txt` shows `plugins: 0` both for an unauthorised call and for five plugins that are not running; after the repair `plugins running 5/5` removes the ambiguity.
- **A5: unchanged, and reasoning.** The ordering argument (a deletion between the snapshot and the file copy can leave a row without its file) follows from the order of the steps of `backup.sh`; no run provoked it, and the admin's own delete order was not read. It goes into DEPLOY.md as a fact about the order, worded so that it does not depend on which comes first in the admin.
- **A6: confirmed.** 300 MiB of media written directly onto the disk do not disturb the restored service: `PASS tree backup-restored: 64 files equal`, `PASS readyz status ok, integrity ok` and `START-READY-S: 1.162` against `1.150` for the seed point, media served with the same sha256.

## Numbers from pass 1

Emulation: an amd64 container on an aarch64 host under Docker Desktop (`HOST-ARCH: aarch64`, `CONTAINER-ARCH: x86_64`), a warm page cache, one filesystem for source and backup, no network between the machines (the backup crosses as a tar stream). A real server's disk and CPU differ.

```text
POINT seed: BACKUP-BYTES=715976 BACKUP-WALL-S=0.509 RESTORE-WALL-S=1.651 VERIFY-S=0.085 STOP-MOVE-COPY-S=0.193 START-READY-S=1.150
POINT bulk: BACKUP-BYTES=315292872 BACKUP-WALL-S=0.722 RESTORE-WALL-S=1.902 VERIFY-S=0.093 STOP-MOVE-COPY-S=0.444 START-READY-S=1.162
```

| | seed | bulk (`BULK_MB: 300`) |
|---|---|---|
| Backup size | 715976 bytes (716K) | 315292872 bytes (301M) |
| Database bytes | 663552 | 663552 |
| Media bytes | 39983 | 314612783 |
| Files in the backup, of which media | 6, 3 | 66, 63 |
| Backup wall, seconds | 0.509 | 0.722 |
| Restore wall, seconds | 1.651 | 1.902 |
| Restore, verify | 0.085 | 0.093 |
| Restore, stop, move aside and copy | 0.193 | 0.444 |
| Restore, start to ready | 1.150 | 1.162 |
| Host and container architecture | aarch64, x86_64 | aarch64, x86_64 |

The three timed parts of the restore add to 1.428 s and 1.699 s; the rest of `RESTORE-WALL-S` is the script's own start, the `MANIFEST` print and the last `/readyz` print. The start-to-ready part has a resolution of about one second, because the script polls once a second.

- **Seed point, in words:** a site of under a megabyte is backed up in about half a second and restored in under two seconds, of which roughly one second is the service starting; on this emulation, so the order of magnitude is seconds, not minutes.
- **Bulk point, in words:** with 300 MiB of media the backup still takes under a second and the restore under two seconds, so on this emulation the cost of a backup or a restore is dominated by the second the service needs to become ready, not by the copy, up to a few hundred megabytes.
- **Per-megabyte rate, an estimate:** the difference of the two data points (314576896 bytes, 300 MiB, more) is 0.213 s of backup wall and 0.251 s of restore wall (the same 0.251 s in stop, move and copy), about 0.7 ms and 0.8 ms per MiB, so roughly one second per GiB of media. This is an estimate and rests on two single readings, on the assumption that time is linear in size and bound by copying; the spread of the seed point alone over three runs (0.419 s in plan 22-02, 0.509 s here, 0.469 s in the investigation run) is about half the difference, so the rate is good to a factor of two at best, and a cold cache or a slow disk on a real server will make it larger.

## BAD lines of pass 1

Every distinct line that begins with BAD in any non-selftest `.txt` file of the four scratch directories, unchanged, 62 lines. They are F-01 (`tree src-backup`, `tree src-timer`), F-02 (`tree src-restored`) and F-03 (`state comparison`, `plugins running`, `check`, `loaded`, `no-load-failures`, `running`, `invoke`).

```text
BAD  check: 1 of 17 assertions passed (exit status 1), see check.txt; the first BAD line is: loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  check: 1 of 17 assertions passed (exit status 1), see check.txt; the first BAD line is: loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  invoke bestellung: class="bestellung" on the page and GET /bestellung 303 expected, got 404
BAD  invoke jahreszahl: 'Jahr: 2026' missing from /plugintest or the shortcode is still there
BAD  invoke kontaktformular: contact-form on the page and GET /formular 303 expected, got 404
BAD  invoke nicht-gefunden: GET /gibts-nicht gave 404 (404 expected), plugin_store rows: 0
BAD  invoke suche: no result link to /plugintest
BAD  loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  loaded bestellung: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  loaded jahreszahl: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  loaded jahreszahl: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  loaded kontaktformular: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  loaded kontaktformular: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  loaded nicht-gefunden: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  loaded nicht-gefunden: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  loaded suche: no 'plugin loaded' line for it since Wed 2026-10-14 10:34:31 UTC
BAD  loaded suche: no 'plugin loaded' line for it since Wed 2026-10-14 10:35:36 UTC
BAD  no-load-failures: {"time":"2026-10-14T10:34:32.476085338Z","level":"ERROR","msg":"plugin not loaded","plugin":"kontaktformular","err":"the module is missing from disk: open /opt/holzcloud/data/plugins/kontaktformular/plugin.wasm: no such file or directory"}
BAD  no-load-failures: {"time":"2026-10-14T10:35:37.648246007Z","level":"ERROR","msg":"plugin not loaded","plugin":"kontaktformular","err":"the module is missing from disk: open /opt/holzcloud/data/plugins/kontaktformular/plugin.wasm: no such file or directory"}
BAD  running bestellung: list_plugins does not show it running with an empty last_error
BAD  running jahreszahl: list_plugins does not show it running with an empty last_error
BAD  running kontaktformular: list_plugins does not show it running with an empty last_error
BAD  running nicht-gefunden: list_plugins does not show it running with an empty last_error
BAD  running suche: list_plugins does not show it running with an empty last_error
BAD  state comparison source against target: 52 pass, 1 bad, see compare-src-tgt.txt; the BAD lines are: plugins running a=5 b=0 (5 expected);
BAD  tree src-backup: only in A: ./marke/logo.svg
BAD  tree src-backup: only in A: ./plugins/bestellung/plugin.json
BAD  tree src-backup: only in A: ./plugins/bestellung/plugin.wasm
BAD  tree src-backup: only in A: ./plugins/jahreszahl/plugin.json
BAD  tree src-backup: only in A: ./plugins/jahreszahl/plugin.wasm
BAD  tree src-backup: only in A: ./plugins/kontaktformular/plugin.json
BAD  tree src-backup: only in A: ./plugins/kontaktformular/plugin.wasm
BAD  tree src-backup: only in A: ./plugins/nicht-gefunden/plugin.json
BAD  tree src-backup: only in A: ./plugins/nicht-gefunden/plugin.wasm
BAD  tree src-backup: only in A: ./plugins/suche/plugin.json
BAD  tree src-backup: only in A: ./plugins/suche/plugin.wasm
BAD  tree src-backup: only in A: ./sprachen/nl.json
BAD  tree src-restored: only in A: ./marke/logo.svg
BAD  tree src-restored: only in A: ./plugins/bestellung/plugin.json
BAD  tree src-restored: only in A: ./plugins/bestellung/plugin.wasm
BAD  tree src-restored: only in A: ./plugins/jahreszahl/plugin.json
BAD  tree src-restored: only in A: ./plugins/jahreszahl/plugin.wasm
BAD  tree src-restored: only in A: ./plugins/kontaktformular/plugin.json
BAD  tree src-restored: only in A: ./plugins/kontaktformular/plugin.wasm
BAD  tree src-restored: only in A: ./plugins/nicht-gefunden/plugin.json
BAD  tree src-restored: only in A: ./plugins/nicht-gefunden/plugin.wasm
BAD  tree src-restored: only in A: ./plugins/suche/plugin.json
BAD  tree src-restored: only in A: ./plugins/suche/plugin.wasm
BAD  tree src-restored: only in A: ./sprachen/nl.json
BAD  tree src-timer: only in A: ./marke/logo.svg
BAD  tree src-timer: only in A: ./plugins/bestellung/plugin.json
BAD  tree src-timer: only in A: ./plugins/bestellung/plugin.wasm
BAD  tree src-timer: only in A: ./plugins/jahreszahl/plugin.json
BAD  tree src-timer: only in A: ./plugins/jahreszahl/plugin.wasm
BAD  tree src-timer: only in A: ./plugins/kontaktformular/plugin.json
BAD  tree src-timer: only in A: ./plugins/kontaktformular/plugin.wasm
BAD  tree src-timer: only in A: ./plugins/nicht-gefunden/plugin.json
BAD  tree src-timer: only in A: ./plugins/nicht-gefunden/plugin.wasm
BAD  tree src-timer: only in A: ./plugins/suche/plugin.json
BAD  tree src-timer: only in A: ./plugins/suche/plugin.wasm
BAD  tree src-timer: only in A: ./sprachen/nl.json
BAD plugins running a=5 b=0 (5 expected)
```

## Harness repairs

None.
