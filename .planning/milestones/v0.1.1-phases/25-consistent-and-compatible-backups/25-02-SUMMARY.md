---
phase: 25-consistent-and-compatible-backups
plan: 02
subsystem: deploy/test (CON-02 harness: a 0.1.0 backup restored by this release; CON-03 harness: backup.sh on a filesystem nearly full but above MIN_FREE_MB)
tags: [backup, restore, compatibility, migration, disk-full, rsync, systemd, tmpfs, docker, verification]
status: complete

requires:
  - phase: 25-01
    provides: nightly.sh prep, the session helper, P25_STAMP_SH, P25_COMPARE_SH, split_sections, leak_check, the document's backup and restore lines
provides:
  - nightly.sh compat-source, compat, capfull, down-con (additive, 0 lines of f513c3d and of b501b1b removed)
  - pure functions with selftest tables: version_is_tag, migration_ok, migration_sentence, compat_volatile, compat_tables_filtered, compat_fixture_ok, capfull_size_mb, capfull_data_ok, capfull_complete_p, capfull_classify
  - a measured trace of CON-02 (pass, no migration crossed) and CON-03 (FAILED-PARTIAL-DIR-LEFT, by hand and by the shipped unit)
affects: [25-03]

tech-stack:
  added: []
  patterns:
    - "the previous release is the 'source' and the fresh box the 'second host' of the shared helpers (compat_names, inside a subshell), so prep, the session helper and leak_check are reused unchanged"
    - "the comparison is shown to fail on a tampered copy before it is trusted (page hash and user role changed, both detected)"
    - "a filesystem is mounted over the backup root inside the container and removed by a trap on every exit path"

key-files:
  created: []
  modified: [deploy/test/nightly.sh]

key-decisions:
  - "NEW_REF is pinned to f513c3d (the merged state of plan 01): HEAD moves with every commit and a new binary would be built each time; the Go code is the one of b501b1b"
  - "CON-03 uses the shipped floor (1024) and never passes MIN_FREE_MB; a different CAPFULL_FLOOR_MB is refused (DEVIATION[floor] is not implemented because the Docker VM held the faithful case)"
  - "DATA-MB is what a backup has to write (the five trees, the database file, csrf.key), not the data directory: the pre-upgrade snapshots are in the data directory and not in a backup"

requirements-completed: []

metrics:
  duration: "not measured (the start time was not recorded); the task commits are timed 13:49, 13:55 and 13:58 local"
  completed: 2026-10-26

actuals:
  tokens: 17800
  tasks: 3
  commits: 3
plan_head_before: f513c3d48d30cae2ec70fcc37fbb229d858abb51
plan_head_after: ae9e0b112efa3b6df7061a889eccbfbb2cfdc434
commits: 3
---

# Phase 25 Plan 02: CON-02 and CON-03 Summary

**`nightly.sh compat-source | compat | capfull` builds v0.1.0 from its tag, lets its own `backup.sh` make a backup, restores it with this release on a fresh box and compares pages, users, media, counts and logins (COMPAT OK, 31 PASS, 0 BAD, run.sh `check` 17 of 17, no migration crossed); and it fills a tmpfs over the backup root to just above `MIN_FREE_MB` and records what `backup.sh`, the shipped unit, the next night and `restore.sh` make of it (the guard passes, rsync fails with ENOSPC, status 11, a partial directory without MANIFEST is left, the next run is refused by the guard).**

Both are traces. CON-02 was played three times from fresh containers (`trace1` the tracer, `trace2` the full comparison, `final` from the committed file) and CON-03 three times (`trace1` showed the unbound variable below, `trace2` and `final` are clean; `final` is from the committed file); the numbers did not change between the runs. The verdicts are plan 03's; `requirements-completed` is empty.

## What was built (all in `deploy/test/nightly.sh`, 1192 lines added, 0 removed against f513c3d and against b501b1b)

| Task | Commit | Content |
|---|---|---|
| 1 tracer | 49abb43 | `compat-source` (run.sh `build up install baseline postinstall backup-fixture`, `INSTALL_FROM=old OLD_TAG=v0.1.0`, no update step, `version_is_tag` on the running binary), `compat` (prep, the tag's `backup.sh` blob compared by sha256 with the installed file and with NEW_REF's, the document's manual backup line, run.sh `transfer`, the document's restore line with yes on stdin, state of source, backup and restored database, logins, migration level read twice), `down-con`, `version_is_tag`, `migration_ok`, `migration_sentence` |
| 2 | 295d2a4 | the fixture (three pages with non-ASCII Markdown, a link, a picture, one more upload, through the 0.1.0 editor forms), the table counts with the named volatile list, goose rows, the tampered-copy control, the fresh-box control, tree list against the backup directory, `integrity_check`, `foreign_key_check`, every media original served over HTTP, `run.sh check`, `COMPAT-DIFF` and `SCRIPTS-IDENTICAL` lines |
| 3 | ae9e0b1 | `cmd_capfull` and `capfull_size_mb`, `capfull_data_ok`, `capfull_complete_p`, `capfull_classify` (see below); the plan 02 block now sits next to plan 01's blocks |

## CON-02: what was measured (`amd64-t25-compat-old/compat.txt`, last run, ends `COMPAT OK`)

```
OLD-BINARY: holzcloud v0.1.0 (4d238b9)                      (built from the tag by bins_build, sha256 83d5ba08...; migrations: 60)
NEW-BINARY: holzcloud v0.1.0-40-gf513c3d (f513c3d)          (migrations: 60)
BACKUP-SH: identical            (sha256 d9f807d8... at v0.1.0, at f513c3d and installed)
MANIFEST: timestamp=20261026-130039 version=holzcloud v0.1.0 (4d238b9) ...
SETUP-BEFORE: HTTP 200 -        SETUP-AFTER: HTTP 404 -
RESTORE-RC: 0   RESTORE-WALL-S: 10.8
STATE-SOURCE:   LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3
STATE-RESTORED: LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2 MEDIA-ROWS: 4 UPLOADED-MEDIA: 3
LEVEL-BEFORE: 60   LEVEL-AFTER: 60   MIGRATIONS-HIGHEST-AT-TAG: 60   MIGRATIONS-HIGHEST-AT-NEW-REF: 60
MIGRATION-SENTENCE: No migration separates the two releases: the backup is at level 60, the restored database is at level 60, the restore ran none.
PRE-UPGRADE-SNAPSHOT-AFTER: 0
LOGIN-ADMIN: HTTP 303 /admin/   LOGIN-EDITOR: HTTP 303 /admin/
MEDIA-SERVED-EQUAL: 4 of 4      CHECK: 17 of 17      CHECK-BAD-LINES: 0
TAMPER-CONTROL: page hash changed -> 1 BAD line(s) for PAGES, user role changed -> 1 BAD line(s) for USERS
SET-ASIDE: /opt/holzcloud/data.before-restore-20261026-130112 users 0
```

Both comparisons (source against restored, backup against restored) PASS for pages (8), users (2), media (12 lines with file hashes), the row counts of 47 non-volatile tables and the goose rows (61). The restored media rows and files agree in both directions, the sha256 list of the restored data directory (25 files) equals that of the backup directory, `holzcloud check`, `integrity_check` and `foreign_key_check` are clean. The seven volatile tables are named with their reason (`INFO COMPARE ... TABLES volatile`): their counts happened to be equal too (activity_log 26 and 26, sessions 4 and 4).

The raw difference between the releases is in the file as `COMPAT-DIFF` lines (16 files, 992 insertions, 61 deletions in `go.mod`, `go.sum`, `internal/media` and `cmd`; nothing in `internal/db`); `SCRIPTS-IDENTICAL: yes` for `backup.sh`, `restore.sh`, the unit and the timer.

What this pair does not show: a pending migration. Both releases have 60 migrations, so `migration_ok` was judged on 60, 60, 60. How a restore handles a pending migration (the pre-upgrade snapshot, the level going from 58 to 60) is covered only by `migration_ok`'s table and belongs on the record's Not tested list. The editor published a page (`FIXTURE-STATUS-compat-liste: published`): in this release an editor may publish.

## CON-03: what was measured (`amd64-t25-capfull/capfull.txt`, last run, ends `CAPFULL OK`)

Construction: `SHIPPED-MIN-FREE-MB: 1024` read from the installed script, no `MIN_FREE_MB` in the environment, `UNIT-DROPINS: ''`; `DATA-MB: 1170` (1225702594 bytes in 245 files in the five trees, plus database and csrf.key; the data directory holds 1227403962 bytes), `FS-SIZE-MB: 1064` (`capfull_data_ok` passed: at least the floor, smaller than the data).

| Run | free at guard | guard | exit | directory left | MANIFEST | class |
|---|---|---|---|---|---|---|
| control, roomy tmpfs (2594 MB) | 2594 MB | passed | 0 | 1226371958 bytes, 248 files, all of the source | yes | COMPLETE |
| by hand, 1064 MB | 1064 MB | passed | 11 | 20261026-125633, 1112198687 bytes, 217 files | no | FAILED-PARTIAL-DIR-LEFT |
| the next run, same filesystem | 4 MB | refused | 1 | none | no | GUARD-REFUSED |
| shipped `holzcloud-backup.service`, fresh 1064 MB | 1064 MB | passed | 11 | 1112198687 bytes, 217 files | no | FAILED-PARTIAL-DIR-LEFT |

Raw: by hand and by the unit the same lines, `rsync: [receiver] write failed on ".../media/1/bulk/bulk-83.bin": No space left on device (28)`, `rsync error: error in file IO (code 11) at receiver.c(401)`. The unit ends `UNIT-ACTIVE-STATE: failed`, `UNIT-RESULT: exit-code`, `UNIT-EXECMAINSTATUS: 11`, 11 journal lines of that invocation (by invocation id). The partial directory holds a complete database snapshot (`holzcloud check` status 0, `integrity_check` ok) and `media` cut off after 1111530543 of 1225702594 bytes. The sentinel directory of an earlier night is intact in every run (`SENTINEL-INTACT: yes`, sha256 unchanged); the earlier backups beneath the mount are back after the unmount (`EARLIER-BACKUPS-AFTER` equals `BEFORE`).

The next run, raw (`capfull-again.txt`): `ERROR: only 4 MB free at /opt/holzcloud/backups, need 1024 MB`, status 1. The partial directory holds the space, so the guard now refuses every following run. Retention (step 5 of `backup.sh`) comes after the copies and `set -e` ends the script before it (reading of the script; the files show nothing was pruned and the sentinel is inside the window anyway), so nothing in the script removes the partial directory.

`restore.sh` pointed at the partial directory with `no` on stdin (`capfull-restore-partial.txt`): `ERROR: .../MANIFEST not found — the backup is incomplete, do not restore it`, exit 1, before the question is asked; the service stayed `active`, `DATA-BEFORE-RESTORE-DIRECTORIES: 0`.

DEVIATION[fs]: a tmpfs, not a disk: no reserved blocks, no fragmentation, memory-backed, the data directory is on another filesystem than the backup root, whereas on a real server the two usually share one disk and a full disk also stops the site's own writes; the errno is the same. `UNIT-TRIGGEREDBY` reads `holzcloud-backup.timer`: that is the unit relationship (the timer that can trigger the service), not evidence that the timer fired; the run was started with `systemctl start --no-block` (the invocation id was empty before).

## Findings (raw; plan 03 classifies them)

1. **A backup made by 0.1.0 restores with this release**: same pages, users, media and logins, level 60 before and after, 17 of 17 checks. No shipped file was needed to change.
2. **A nearly full disk above the floor leaves a partial directory and a failed night** (class FAILED-PARTIAL-DIR-LEFT, rsync 11, exit 11, by hand and by the unit). `backup.sh` says nothing about the directory it leaves; the exit status is the only signal, and the MANIFEST rule is what keeps `restore.sh` from using it. This is a measured property of the shipped script, not an edit.
3. **The next night is refused by the guard** while the partial directory stays (free 4 MB, status 1) and the script cannot prune it. On a real server the second night fails differently from the first and the first failed night has to be cleaned up by hand. This is a reading of order in the script combined with the measurement; the disk of a real server may behave differently (DEVIATION[fs]).
4. **No FALSE-COMPLETE** occurred (nothing exited 0 with a MANIFEST on the nearly full filesystem).

## Deviations from Plan

### Process

- **[Base]** `git merge-base --is-ancestor f513c3d HEAD` was false and the tree was clean; `git reset --hard f513c3d` was run as instructed.
- **[Pin]** `NEW_REF=f513c3d` was pinned (see key decisions). Binaries are built into `deploy/test/out/bin` of the worktree (the phase 24 cache was in the main checkout and not reachable).
- **[Names]** The trace containers are `holzcloud-t25-compat-old`, `holzcloud-t25-compat-new` and `holzcloud-t25-capfull` (evidence in `amd64-t25-compat-old`, `amd64-t25-compat-new`, `amd64-t25-capfull`), not the `-trace-` names of the plan's verify lines; all three were removed with `docker rm -f` at the end. The capfull source is `nightly.sh source` with `NAME=holzcloud-t25-capfull`.
- **[record-check]** In the worktree `deploy/test/out` has no phase 24 scratch directories, so `record-check` was run with `E=` pointing at the main checkout's `deploy/test/out`; it reads `RECORD-CHECK OK 29 blocks`.
- **[Commit split]** The pure functions of task 2 (`compat_volatile`, `compat_tables_filtered`, `compat_fixture_ok` and their tables) were already in the file when the tracer of task 1 ran and landed in its commit; their tables were seen failing (5 of 412 cases) before the functions existed. The CON-03 tables were seen failing (10 of 442; the `st_fail` cases pass vacuously while a function is missing) before `capfull_*` existed. The task 3 code was removed from the file for the task 2 commit and restored for the third.
- **[Diff placement]** With the plan 02 block at the end of the file, the default `git diff` against b501b1b showed 104 removed lines (an alignment artifact of a 1200-line insertion: `--patience` and `--minimal` showed 0). The block now sits next to plan 01's blocks; the default diff shows 0 removed lines against f513c3d and against b501b1b.
- **[Final runs]** After the move, `capfull` and `down-con compat-source compat` were played again from the committed file with the same result (CAPFULL OK, COMPAT OK).
- **[git diff --stat]** `COMPAT-DIFF` uses `--stat=200` so that no name is cut; the command is written to the file.
- **[Stray file]** A typo created `/private/tmp/claude-501/-Users-holz/projects-holzcloud-cms/placeholder` (one line of text, outside the repository); the removal of its directory was denied by the permission classifier and was not worked around. It is harmless and can be deleted.

### Auto-fixed

1. **[Rule 1 - Bug] `cf_sentinel_intact` read an unset variable in the control** (no sentinel is planted there): the first capfull run printed `CF_SENT: unbound variable` and `CONTROL-SENTINEL-INTACT: no`. It now answers `none-planted` when no sentinel exists. Found in the first trace, fixed before the committed runs.

## Checks at the end

- `bash deploy/test/nightly.sh selftest`: `SELFTEST OK (442 cases)` (368 of plan 01 plus 74 new).
- `record-check` with `E=` set: `RECORD-CHECK OK 29 blocks`.
- `git diff -U0 f513c3d -- deploy/test/nightly.sh | grep -c '^-[^-]'` is 0; the same against b501b1b is 0; no `mapfile|readarray|declare -A|local -A` in the added lines.
- `git diff --quiet b501b1b HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` and `git diff --quiet f1ba5a2 -- docs/beta.md` exit 0. Nothing shipped was edited, `run.sh` is untouched, STATE.md and ROADMAP.md are untouched.
- `leak_check` ended `LEAK-CHECK OK` in every subcommand; no mount remains in the capfull container (removed) and `mount | grep /opt/holzcloud/backups` on the host shows nothing.

## Evidence kept

`/private/tmp/claude-501/-Users-holz-Projects-holzcloud-cms/295eb7a6-636f-4c10-8442-a5a0771f92c4/scratchpad/phase25-out/` holds `amd64-t25-compat-old/` (compat.txt, compat-state-*.txt, compat-backup.txt, compat-restore.txt, compat-check.txt, compat-binary.txt, prep.txt), `amd64-t25-compat-new/` (transfer.txt, check.txt, install.txt), `amd64-t25-capfull/` (capfull.txt, capfull-control/hand/again/unit/restore-partial/df.txt), the logs of every run (`*-trace1`, `*-trace2`, `*-final`), `bin/*.info` and `t25-nightly-02.diff`. These are trace evidence; the final set is plan 03's.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: no new network surface; the state files carry counts, emails, roles and sha256 of content, never a credential row; the fixtures cross by exec-pipe; the mount lives in the container's own mount namespace.

## Self-Check: PASSED
