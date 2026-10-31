---
phase: 25-consistent-and-compatible-backups
plan: 01
subsystem: deploy/test (CON-01 harness: backup.sh under load, restored by restore.sh, compared in both directions)
tags: [backup, restore, consistency, rsync, sqlite, htmx, editor-session, docker, verification]
status: complete

requires:
  - phase: 24
    provides: nightly.sh (source, target, selftest, record-check, leak_check), the cached binaries at b501b1b, the shipped backup.sh and restore.sh
provides:
  - nightly.sh prep, consist, window (additive, zero lines of b501b1b removed)
  - pure functions with selftest tables: media_agree, run_class, window_hits, step_span, window_name, exposure_ok, rule_of_three, con01_outcome, restore_required, restore_floor_ok, restore_every_ok, restore_every_max, consist_pick_every, consist_stage_counts, hold_expect
  - a measured trace of CON-01: the pair is not atomic, in both directions, reproduced by the shipped scripts without any pause (6 runs per stage) and on purpose with three controls
affects: [25-02, 25-03]

tech-stack:
  added: []
  patterns:
    - "load is an editor session over loopback curl with the CSRF token of each form; the admin is walled off by the compulsory two-factor setup"
    - "the verdict is computed from consist-runs.txt, so it can be re-judged offline; consist.txt carries the inputs as KEY lines before the verdicts"
    - "a forced window through HOLZCLOUD_BIN (a root-owned stand-in) leaves the shipped script byte-identical"

key-files:
  created: []
  modified: [deploy/test/nightly.sh]

key-decisions:
  - "The load runs as an editor (churn@example.test); an admin session never reaches a page form (DEVIATION[editor-session], measured as ADMIN-SESSION)"
  - "A stage is a verdict only with a floor of restored and compared runs: max(10, ceil(MIN_RUNS/2)) capped at the stage's runs; a trace (MIN_RUNS below 10) must restore every run it can (a refused backup has nothing to restore)"
  - "The window of a disagreement is named from the churn log; a delete that overlaps the database step is named overlaps-snapshot-step, not assigned to before or after the snapshot"

requirements-completed: []

metrics:
  duration: "not measured (the start time was lost in a context reset); the three task commits are timed 13:01, 13:19 and 13:28 local, the plan commit 12:27"
  completed: 2026-10-26

actuals:
  tokens: 25600
  tasks: 3
  commits: 3
plan_head_before: 19eeb79b8eede6c2deb592a14b0de055e6639dea
plan_head_after: 309815b6ad44c325cf2558104949c6d7462d6263
commits: 3
---

# Phase 25 Plan 01: CON-01 tracer, two stages and the forced window Summary

**`nightly.sh prep | consist | window` runs the shipped `backup.sh` under an editor's page saves, uploads and deletes, restores each directory with the shipped `restore.sh` on a second container and compares media rows against media files in both directions; a six-run trace per stage already reproduces files without a row, a row without a file and a refused backup (rsync 24), and three controls make each case happen on purpose.**

CON-01 itself is not decided here. This plan built the harness and ran it as a trace (`MIN-RUNS: 1`); the evidence set (`MIN-RUNS: 30`, N=40) is plan 03. `requirements-completed` is therefore empty.

## What was built (all in `deploy/test/nightly.sh`, 1892 lines added, 0 removed against b501b1b)

| Task | Commit | Content |
|---|---|---|
| 1 tracer | 21a692a | `prep` (editor account, admin two-factor wall recorded as `ADMIN-SESSION`, both write paths proven), session helper, churn generator, `consist` end to end: the document's manual backup line with a millisecond stamp on every line, tar-stream ship, the document's restore line with `yes`, comparator in both directions, `holzcloud check`, `integrity_check`, `foreign_key_check`, page references, sha256 list of the restored tree against the backup; the comparator is shown to fail on four tampered lists before the first run |
| 2 consist | 76fdcbe | two stages (adds, deletes), `window_name`, `exposure_ok`, `rule_of_three`, `con01_outcome`, restored-run floor, `WINDOW-NAME` lines, inputs as KEY lines before the `OUTCOME-STAGE-*` verdicts, 90-minute projection |
| 3 window | 309815b | `hold_expect`, `cmd_window`: shipped `backup.sh` paused through `HOLZCLOUD_BIN`, controls C0/C1/C2, C2 consequences as raw lines, mechanism INFO lines with file and line, trap that removes the stand-in |

## Results of the trace (not the evidence set)

Final six-run trace, `deploy/test/out/amd64-t25-trace/consist.txt` (copy in `phase25-out`), last line `CONSIST OK`, `MIN-RUNS: 1`, `BULK: BULK-MB: 100 FILES: 20 BYTES: 104861696`, 4 uploaders, 1 page saver, 1 deleter (stage deletes), `CHURN-PAUSE-S: 0.05` plus up to 0.29 s jitter:

```
OUTCOME-STAGE-adds: ORPHAN-ONLY-NAMED
CONSIST-SUMMARY-STAGE-adds: RUNS=6 RESTORED=6 AGREE=2 ORPHAN-ONLY=4 ROW-WITHOUT-FILE=0 BACKUP-REFUSED=0 RESTORE-FAILED=0 EXPOSED=4 UPLOADS-IN-WINDOW=11 OUTCOME=ORPHAN-ONLY-NAMED
OUTCOME-STAGE-deletes: ROW-WITHOUT-FILE-NAMED
CONSIST-SUMMARY-STAGE-deletes: RUNS=6 RESTORED=5 AGREE=1 ORPHAN-ONLY=3 ROW-WITHOUT-FILE=1 BACKUP-REFUSED=1 RESTORE-FAILED=0 EXPOSED=5 UPLOADS-IN-WINDOW=12 OUTCOME=ROW-WITHOUT-FILE-NAMED
BACKUP-RC-HISTOGRAM-STAGE-deletes: 0=5 24=1
```

`RESTORED-RUNS-STAGE-adds: 6` (runs 1 2 3 4 5 6), `RESTORED-RUNS-STAGE-deletes: 5` (the run that backup.sh refused has no MANIFEST and nothing to restore). Every non-AGREE run carries `WINDOW-NAME` lines, e.g. `overlaps-snapshot-step (upload churn-adds2-w1-4.jpg ...)` for an orphan file and `overlaps-snapshot-step (delete 122:1/a031e791... 1793017907868 1793017908151)` for a refused backup.

A refused backup, raw (`consist-deletes-run-4-backup.txt`):

```
1793017908023 Backing up media...
1793017908110 file has vanished: "/opt/holzcloud/data/media/1/a031e791c7b5df3f82eaae9cdf9b274b-medium.jpg"
1793017908129 file has vanished: "/opt/holzcloud/data/media/1/a031e791c7b5df3f82eaae9cdf9b274b-thumb.jpg"
1793017908153 file has vanished: "/opt/holzcloud/data/media/1/a031e791c7b5df3f82eaae9cdf9b274b.jpg"
1793017908446 rsync warning: some files vanished before they could be transferred (code 24) at main.c(1347) [sender=3.2.7]
WALL 1793017907414 1793017908468
RC 24
```

Forced window, `window.txt` ends `WINDOW OK`:

```
CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE
CONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY
CONTROL C2 ACTION=delete HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=3 ORPHAN=0 CLASS=ROW-WITHOUT-FILE EXPECTED=ROW-WITHOUT-FILE
```

What an operator sees after C2 was restored (raw, nothing asserted in advance):

```
C2-CONSEQUENCE media route, file the backup lacks (GET /media/1/7a76e7a2b5d07603d83e828818c0a77a.jpg, Host test.local): HTTP status and body length: 404 19
C2-CONSEQUENCE media route, file the backup has (GET /media/1/811c26f2092ca7f025ad6d2284a5267a.jpg, Host test.local): HTTP status and body length: 200 21611
C2-CONSEQUENCE holzcloud check on a copy of the restored database: exit status 0 and output: ... INFO sqlite opened journal_mode=wal path=... integrity: ok
C2-CONSEQUENCE restore.sh exit status 0, its last line (empty lines skipped): {"status":"ok","version":"v0.1.0-31-gb501b1b","commit":"b501b1b",...}
C2-CONSEQUENCE editor login on the restored site: HTTP 303 /admin/
C2-CONSEQUENCE media list of the admin (editor session, GET /admin/websites/1/media): HTTP 200 -
C2-CONSEQUENCE restored site: PASS PAGE-REFS-DANGLING: 0
```

So a row without a file restores without any error: `restore.sh` exits 0, `holzcloud check` is silent about it, the media list answers 200, and the only symptom is a 404 on the file.

## Findings (class: plan 03 decides how they enter the record)

1. **Not atomic, in both directions, without any pause.** Under 4 uploaders and 1 page saver, 4 of 6 runs in stage adds and 3 of 6 in stage deletes restored with files that have no row (all named: an upload overlapping the database step). One run of stage deletes restored with 3 files missing for a row (a delete overlapping the database step: variant files removed before the snapshot, the row still in it). One run of stage deletes was refused by `backup.sh` itself (rsync 24, no MANIFEST).
2. **Sixth restore within five minutes on one host fails.** The shipped `holzcloud.service` has `StartLimitIntervalSec=300` and `StartLimitBurst=5`; `restore.sh` stops and starts the service, so the sixth restore in five minutes ends `Job for holzcloud.service failed because start of the service was attempted too often` (journal: `start-limit-hit`). Raw evidence: `amd64-t25-trace-startlimit/consist-deletes-run-1-restore.txt`, `target-unit-and-journal.txt`. Not edited; the harness runs `systemctl reset-failed holzcloud` before each restore (DEVIATION[start-limit]). An operator who restores repeatedly while rehearsing meets this.
3. **A refused backup leaves its directory behind.** `backup.sh` creates the timestamp directory before it copies anything, so after rsync 24 the directory is there without a MANIFEST (the harness checked only that the MANIFEST is missing, not what else remains); `restore.sh` refuses a directory without one by design. Not inspected further here.

## Deviations from Plan

### Process

- **[Base]** `git merge-base --is-ancestor 19eeb79 HEAD` was false and the tree was clean; `git reset --hard 19eeb79` was run as instructed.
- **[Pin]** `NEW_REF=b501b1b` was pinned so that the cached binaries of phase 24 are reused. The Go code of HEAD is identical to b501b1b (only docs and deploy test files differ).

### Auto-fixed / design changes while executing

1. **[Rule 1 - Bug] `step_span` ended a step at the binary's own next output line.** A step now ends at the next line starting `Backing up `, `Pruning backups` or `=== `. The plan says "to the next line"; the window is 0.1 s longer than with that reading, which is the correct one (the CLI process prints `sqlite opened` after the snapshot step began).
2. **[Rule 3 - Blocking] start-limit** (finding 2), harness only.
3. **[Rule 1 - Bug] The rsync status regex only knew `rsync error`.** Code 24 comes as `rsync warning: some files vanished ... (code 24)`; both are read now (`sed -E`; BSD sed has no `\|`).
4. **[Rule 1 - Bug] Phase lock between load and backup.** Without jitter every run met the same phase of the load (0 uploads in the window, run after run, so every stage was INCONCLUSIVE). Workers now sleep `CHURN_PAUSE` plus up to 0.29 s of jitter and the backup starts a random fraction of a second after the load is running. Defaults changed from the first draft: `CHURN_WORKERS=4` (was 2), `CHURN_PAUSE=0.05` (was 0.3).
5. **[Added rule] Restored runs.** `RESTORED-RUNS-STAGE-x: n`, `RESTORED-RUN-NUMBERS-STAGE-x` and `CONSIST-SUMMARY-STAGE-x ... RESTORED=n` are KEY lines. Floor: `max(10, ceil(MIN_RUNS/2))`, capped at the stage's runs; `CONSIST_RESTORE_EVERY` is checked against it before the first run (`CONSIST_RESTORE_EVERY=5` with 40/30 dies with `use CONSIST_RESTORE_EVERY=2 or less`). In a trace (MIN_RUNS below 10) a refused backup is not asked for, at least one run must be restored; an evidence set keeps its full floor whatever was refused.
6. **[Naming]** Stage deletes writes `consist-deletes-run-N-*`, stage adds `consist-run-N-*`, the controls `window-Cn-*`; a mixed call `source target prep consist` uses the `t25` default container names for all.
7. **[Scope]** `usage()` is redefined later in the file (prints all leading `#` header lines) because the original prints a fixed line range; no original line was removed.
8. **[Added]** `BACKUP-REFUSED` is a named result (`REFUSED-NAMED`, named by its rsync status) beside the orphan and row classes; `CONSIST_KEEP_RAW=1` keeps the raw files of every run.

## Deviation labels written by the harness

`DEVIATION[editor-session]`, `DEVIATION[churn]`, `DEVIATION[start-limit]`, `DEVIATION[hold]` (and, when the first run is slow, `DEVIATION[restore-every]`), in `deviations.txt`.

## Checks at the end

- `bash deploy/test/nightly.sh selftest`: `SELFTEST OK (368 cases)` (220 of phase 24 plus 148 new); the new tables were first seen failing (57 cases) before the functions existed.
- `record-check`: `RECORD-CHECK OK 29 blocks`.
- `git diff -U0 b501b1b -- deploy/test/nightly.sh | grep -c '^-[^-]'` is 0; no `mapfile|readarray|declare -A|local -A` in the added lines.
- `git diff --quiet b501b1b HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh` and `git diff --quiet f1ba5a2 -- docs/beta.md` hold.
- Task 1 verify on fresh containers: `prep.txt` ends `PREP OK` (the editor was created by prep), `RUN 1 adds CLASS=ORPHAN-ONLY`, no `BAD` in `consist-run-1-compare.txt`.
- The stand-in, the flag and the bound marker are gone from the source container after `window`; `leak_check` ends `LEAK-CHECK OK` in every subcommand.

## Parameters for plan 03

- Load: `CHURN_WORKERS=4`, `CHURN_PAUSE=0.05`, `BULK_MB=100` gave 4 to 6 exposed runs of 6 per stage with the jitter; the exposure guard (half of N, N at least MIN_RUNS) is met with room.
- One restored run is about 23 s (12 s of it the restore waiting for `/readyz`), so 2 stages of 40 runs take about 31 minutes; no projection beyond 90 minutes occurs, `CONSIST_RESTORE_EVERY=1` stays.
- Floors: `CONSIST_RUNS=40 CONSIST_MIN_RUNS=30` needs 15 restored runs per stage; `CONSIST_RESTORE_EVERY` may be 2 at most.

## Evidence kept

`/private/tmp/claude-501/-Users-holz-Projects-holzcloud-cms/295eb7a6-636f-4c10-8442-a5a0771f92c4/scratchpad/phase25-out/` holds `amd64-t25-trace/` (prep.txt, consist.txt, consist-runs.txt, window.txt, window-C0/C1/C2-*, raw per-run files), `amd64-t25-trace-tgt/`, `amd64-t25-trace-startlimit/`, the logs, `t25-nightly.diff`. Caution: `amd64-t25-trace/` holds raw per-run files of several trace runs; `consist.txt` and `consist-runs.txt` are from the last one (consist9) and are authoritative, a raw `consist-*-run-N-*` file may be older. This is trace evidence only; the final evidence of phase 25 is made in plan 03. Both trace containers were removed.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: no new network surface; the editor reuses the baseline password file, the helper prints only `HTTP code redirect`, `leak_check` passed in every subcommand.

## Self-Check: PASSED
