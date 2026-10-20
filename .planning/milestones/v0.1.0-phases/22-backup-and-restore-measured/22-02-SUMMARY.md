---
phase: 22-backup-and-restore-measured
plan: 02
subsystem: infra
tags: [bash, docker, systemd, timer, sqlite, backup, restore, test-harness]

requires:
  - phase: 22-backup-and-restore-measured
    provides: "run.sh backup, transfer and restore between a source and a fresh target container, backup_verify, restore_proof, on_target (plan 22-01)"
provides:
  - "run.sh backup-timer: a container-only drop-in brings the never-fired holzcloud-backup.timer into reach, one run of the hardened service is observed and its backup verified"
  - "run.sh restore-negative: four refusals (damaged database, no database, answer no, no free space) with the live data's invariants equal before and after"
  - "run.sh backup-fixture: a planted language file and logo, and BULK_MB of synthetic media"
  - "run.sh backup-chain: the whole proof from one command, OUT_DIR/summary.txt in a fixed vocabulary, on_exit removes all containers"
  - "deploy/test/README.md: every new subcommand, variable, output file, deviation and scenario, and the section Phase 22: backup, timer and restore"
  - "scratch evidence under deploy/test/out/amd64-bak-t3s/t3t, t4s/t4t and amd64-bak-t5-s, -t, -s-bulk, -t-bulk for plan 22-03"
affects: [22-03, 22-04, 22-05, phase-23]

actuals:
  tokens: 13500
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "the timer is brought into reach by a drop-in with an empty OnCalendar= reset first and daemon-reload only; the evidence is TriggeredBy, the journal and the directory the journal names, never the newest directory and never a timer restart"
    - "a negative case is a CASE line, the script's output, and one PASS or BAD per invariant of the live data (inode, whole-tree hash, InvocationID, ActiveEnterTimestamp, set-aside count, is-active, readyz, integrity_check), taken before and after"
    - "a chain runs every stage in a subshell and keeps its status; verdicts are read from files that must exist, be non-empty and hold no BAD line, so a stage that died proves nothing"

key-files:
  created: []
  modified:
    - deploy/test/run.sh
    - deploy/test/README.md

key-decisions:
  - "backup-timer returns 1 when the timer's backup has BAD lines, but BAK-03 is judged from timer.txt alone: the timer behaving and the backup being incomplete are two findings"
  - "The bulk point reuses the same source container in its own OUT_DIR-bulk (baseline.txt copied in) and a second fresh target, so every restore in the record is onto an empty system"
  - "on_exit removes an explicit list (source, TGT_NAME, TGT_NAME-bulk), not 'TGT_NAME plus -bulk' computed at exit, because a die inside on_target leaves NAME pointing at a target"
  - "backup_verify writes DIR-OWNER-MODE only when the file does not hold it yet, so timer.txt carries the KEY line once"

requirements-completed: [BAK-01, BAK-02, BAK-03]

coverage:
  - id: D1
    description: "The never-fired holzcloud-backup.timer fires once under real systemd through a container-only drop-in; the hardened service runs as the service user, exits 0 and leaves a mode-700 directory that the journal names, and that directory is the one transferred and restored"
    requirement: "BAK-03"
    verification:
      - kind: integration
        ref: "task 1 verify (t3s/t3t): TRIGGERED-BY holzcloud-backup.timer, RESULT success, EXIT-STATUS 0, DIR-OWNER-MODE holzcloud 700, NEXT moved, LAST n/a to set, transfer.txt DIR equals timer.txt DIR, LOGIN HTTP 303 /admin/"
        status: pass
      - kind: integration
        ref: "backup-chain (t5): BAK-03 PASS, TIMER: TRIGGERED-BY=holzcloud-backup.timer RESULT=success"
        status: pass
    human_judgment: false
  - id: D2
    description: "The four refusals are played once on the restored target with the live data untouched, the damage proven real first"
    requirement: "BAK-02"
    verification:
      - kind: integration
        ref: "task 2 verify (t4s/t4t): restore-negative.txt cases a to d, four NEGATIVE-RESULT: refused, no BAD, RESTORE-NEGATIVE OK; backup-chain (t5): RESTORE-NEGATIVE PASS"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two data points (the seed site, and the site with synthetic media) and the planted language file and logo, each restored into its own fresh target"
    requirement: "BAK-02"
    verification:
      - kind: integration
        ref: "task 2 verify: bulk.txt BULK-MB 20, FILES 4, BYTES 20975616, DEVIATION[fixture]; backup-chain (t5): POINT seed and POINT bulk lines, tree comparison finds marke/logo.svg and sprachen/nl.json as BAD (the predicted class)"
        status: pass
    human_judgment: false
  - id: D4
    description: "One command repeats the whole proof from fresh containers, writes summary.txt in the fixed vocabulary and removes every container it made"
    requirement: "BAK-01"
    verification:
      - kind: integration
        ref: "NAME=holzcloud-bak-t5s ... BULK_MB=20 run.sh backup-chain: summary.txt with BAK-01 FAIL, BAK-02 FAIL, BAK-03 PASS, RESTORE-NEGATIVE PASS, both POINT lines, TIMER, STEPS 76 played 0 failed, LEAK-CHECK OK; no holzcloud-bak-t5 container remains; the guard with TARGET=unit-test exits 1 before docker is touched"
        status: pass
    human_judgment: false
  - id: D5
    description: "The README documents every new subcommand, variable, output file, deviation and scenario name, and the pitfalls the phase found"
    verification:
      - kind: other
        ref: "task 3 verify: grep of every listed name and the section heading"
        status: pass
    human_judgment: true
    rationale: "The names are grep-checked, whether the pitfalls and the rerun instructions read as correct and sufficient to a stranger is a judgment"

duration: 13min
completed: 2026-10-14
status: complete
plan_head_before: 7c0e9034da0349fcdbaa35c96395d2ea36b4975a
plan_head_after: 2d8160f001d8a38473723faa6aba37a5d2b22211
commits: 3
---

# Phase 22 Plan 02: The timer, the refusals and the chain Summary

**The never-fired holzcloud-backup.timer has fired once under real systemd through a container-only drop-in and its hardened service made a verified mode-700 backup that a different fresh container restored; the four refusals are played with the live data untouched; `backup-chain` repeats the whole proof, including a second data point with synthetic media, from one command. BAK-03 and RESTORE-NEGATIVE pass; BAK-01 and BAK-02 fail only on the one predicted class: plugins/, sprachen/ and marke/ are not in the backup.**

## Performance

- **Duration:** about 13 min of working time (the container runs themselves were 3 to 6 minutes each)
- **Started:** 2026-10-14T10:16:50Z
- **Completed:** 2026-10-14T10:30Z
- **Tasks:** 3 of 3
- **Files modified:** 2 (deploy/test/run.sh +624 lines, deploy/test/README.md +79/-21)

## Accomplishments

- **BAK-03, observed.** A drop-in `/etc/systemd/system/holzcloud-backup.timer.d/container.conf` (an empty `OnCalendar=`, then `*-*-* HH:MM:SS` 60 seconds ahead of the container clock, `RandomizedDelaySec=0`, `AccuracySec=1s`) and `systemctl daemon-reload`, no timer restart. The service ran as the timer's own invocation: `TRIGGERED-BY: holzcloud-backup.timer`, `RESULT: success`, `EXIT-STATUS: 0`, started 10:27:16, ended 10:27:17; `LAST` went from `n/a` to the fire time, `NEXT` from the drop-in time to the same time the next day. The directory is the one the journal's `=== Backup complete: ... ===` line names, owner and mode `holzcloud 700`, and it is not the manual run's. The hardened unit (`ProtectSystem=strict`, `ReadWritePaths`, `PrivateTmp`) ran for real; seccomp is not enforced under the emulation, and the backup unit sets no `SystemCallFilter` anyway.
- **The timer's backup is the one that is restored.** `backup_verify DIR timer` gave it the same two integrity readers, whole-directory comparison and row counts (`verify-timer.txt`, `tree-*-timer.txt`, `rows-timer.txt`); `transfer` took `timer.txt`'s `DIR:` and the target restored it: `LOGIN: HTTP 303 /admin/`, setup form 200 before and 404 after.
- **Four refusals, live data untouched.** In `restore-negative.txt` the damaged case first proves the damage real (`holzcloud check` on a copy of the half-size file: exit 1, `verify WAL: database disk image is malformed (11)`), then `restore.sh` with `yes` on stdin on purpose exits 1 with `the snapshot fails its integrity check`; the directory with only a MANIFEST exits 1 with `holzcloud.sqlite not found`; the answer `no` exits 1 with `aborted`; `backup.sh` with `MIN_FREE_MB=99999999` exits 1 with `only 355304 MB free ... need 99999999 MB` and makes no directory. After every case inode, whole-tree hash, InvocationID, ActiveEnterTimestamp, the one set-aside directory, `active`, `/readyz` 200 and `integrity_check ok` are equal. Four `NEGATIVE-RESULT: refused`, no BAD.
- **Two data points.** The seed site, and the same source container with `BULK_MB` of 5 MiB `/dev/urandom` files written by the service user under `media/WEBSITE/bulk` (outside the database), each restored into its own fresh target. A language file `sprachen/nl.json` and a logo `marke/logo.svg` are planted in both so the data directory holds every directory a backup must carry.
- **One command.** `backup-chain` pins `NEW_REF` to a sha, plays source, first target, `backup`, `backup-timer`, `transfer`, `restore`, `restore-negative`, then the bulk point with a second fresh target, keeps every stage's status, writes `summary.txt` and removes all of its containers unless `KEEP=1`.

## Task Commits

1. **Task 1 (tracer): the timer fires the hardened backup service once, observed and verified** - `a956685` (feat)
2. **Task 2: the four refusals with invariants, planted directories, synthetic bulk media** - `8627a20` (feat)
3. **Task 3: backup-chain and the README** - `2d8160f` (feat)

**Plan metadata:** committed with this SUMMARY (docs).

Tracer gate (auto mode): Task 1's verify is the end-to-end run itself (source chain, target, `backup backup-timer transfer restore`); it passed, so expansion went ahead.

## Files Created/Modified

- `deploy/test/run.sh` - `cmd_backup_timer`, `cmd_restore_negative` (with `neg_inv`, `neg_finish`, `neg_restore`, `restore_negative_into`), `cmd_backup_fixture`, `cmd_backup_chain` (with `file_clean`, `kv`, `steps_of`, `point_line`), the `CLEANUP_TARGETS` handling of `on_exit`, header comment, usage and dispatch. `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud-backup.service`, `deploy/holzcloud-backup.timer` and `deploy/holzcloud.service` are untouched (`git diff --quiet HEAD` clean before every commit and after the last run).
- `deploy/test/README.md` - subcommand rows, environment rows, output-file rows, the four directories of a chain, deviation and scenario names, the section `Phase 22: backup, timer and restore` replacing `Hand-over to phase 22`.

## Pass 1 on the unchanged scripts: every BAD line of the chain run (t5)

All of it is the class the research predicted (Pitfall 1): `backup.sh` copies only `holzcloud.sqlite`, `csrf.key`, `media/` and `templates/`.

- `verify-backup.txt` and `verify-timer.txt` of the seed point and `verify-backup.txt` of the bulk point, 12 lines each: `tree src-backup` (or `src-timer`): `only in A: ./plugins/{bestellung,jahreszahl,kontaktformular,nicht-gefunden,suche}/plugin.{json,wasm}` (10), `./marke/logo.svg` (1), `./sprachen/nl.json` (1). The two planted files are the new ones against plan 22-01: the backup loses them too.
- `restore.txt` of each target, 14 lines: the same 12 as `tree src-restored: only in A`, plus `state comparison source against target: 52 pass, 1 bad ...; the BAD lines are: plugins running a=5 b=0 (5 expected);` and `check: 1 of 17 assertions passed (exit status 1) ...; the first BAD line is: loaded bestellung: no 'plugin loaded' line ...`.
- `compare-src-tgt.txt` of each target, 1 line: `BAD plugins running a=5 b=0 (5 expected)`.
- `check.txt` of each target, 16 lines: `loaded` x5, `no-load-failures` (`plugin not loaded`), `running` x5, `invoke` x5 (jahreszahl, kontaktformular 404, bestellung 404, suche, nicht-gefunden).
- Bulk point: its 12 verify lines are the same set and no `media/.../bulk` line, so the 20 MB of synthetic media are in the backup and in the restore byte for byte.

Verdicts: `BAK-01 FAIL`, `BAK-02 FAIL`, `BAK-03 PASS`, `RESTORE-NEGATIVE PASS`. Plan 22-04 fixes the scripts.

## Measured (amd64 emulation on an aarch64 host; scratch numbers, plan 22-03 takes the final ones)

| Point | Backup bytes | Backup wall s | Restore wall s | verify s | stop+move+copy s | start to ready s |
|---|---|---|---|---|---|---|
| seed (716 KB) | 715976 | 0.419 | 1.652 | 0.092 | 0.198 | 1.139 |
| bulk (BULK_MB=20, 21.7 MB) | 21691592 | 0.491 | 1.640 | 0.095 | 0.209 | 1.126 |

At 20 MB the copy is below the resolution of the readings (stop+move+copy 0.209 against 0.198 s), so no per-megabyte rate can be read from it; the 300 MB default of the chain is what plan 22-03 needs. The `START-READY-S` resolution is about one second (the script polls once a second).

## Decisions Made

- BAK-03 is judged from `timer.txt` alone; `backup-timer` still exits 1 when the timer's backup has BAD lines. The timer behaving and the backup being incomplete are separate findings.
- The bulk point uses the same source container (the same site with media added) with its own evidence directory and a second fresh target, as RESEARCH's open question 2 recommended.
- `on_exit` removes an explicit list of container names (source, target, bulk target), robust against a die while `NAME` points at a target.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `deviation_kinds` could not see hyphenated deviation names**
- **Found during:** Task 3 (the summary's `DEVIATIONS:` line has to list `backup-name`)
- **Issue:** `sed 's/^DEVIATION\[\([a-z]*\)\].*/\1/p'` stops at the hyphen, so `DEVIATION[backup-name]` would have been dropped from every summary.
- **Fix:** the pattern is `[a-z-]*` in `deviation_kinds` and in the chain's own distinct-kinds loop.
- **Files modified:** deploy/test/run.sh
- **Commit:** 2d8160f

**2. [Rule 1 - Bug] The KEY line `DIR-OWNER-MODE:` would have been written twice into timer.txt**
- **Found during:** Task 1 (design: `backup-timer` writes it, `backup_verify` appends it again)
- **Fix:** `backup_verify` writes it only when the file does not hold the line yet; `backup` is unaffected.
- **Files modified:** deploy/test/run.sh
- **Commit:** a956685

### Plan-text departures (named, each small)

- **`restore-negative` and the pass-1 exit statuses.** The plan's Task 2 verify allows `backup` and `restore` to exit 1; `restore-negative` itself exits 0 when its cases hold, and it does (RESTORE-NEGATIVE OK in t4 and t5).
- **errexit inside the chain's subshells.** `( cmd_x ) || sc=$?` runs the stage with `set -e` suspended (the same shape `cmd_fresh` uses for `caddy`), so a command that fails without its own check does not end the stage. The stages check their preconditions and their results explicitly (`die`, BAD lines), so nothing in them relied on errexit.
- **`CLEANUP_TARGETS` and a name list.** The plan names `CLEANUP_TARGETS=1`; the handler also reads `CLEANUP_TARGET_LIST` (source, target, bulk target) instead of computing `TGT_NAME-bulk` at exit.

**Total deviations:** 2 auto-fixed (both Rule 1), 3 plan-text departures. **Impact:** none on scope; the pinned vocabulary is unchanged.

## Findings for later plans

- **For 22-03:** use `BULK_MB=300` (the chain default); at 20 MB nothing about copy time can be read. The scratch evidence of this plan is `amd64-bak-t5-s`, `-t`, `-s-bulk`, `-t-bulk` (containers removed, files remain, gitignored).
- **For 22-04:** the one defect is still the whole class above (plugins/, sprachen/, marke/). `restore.sh` also leaves `holzcloud.sqlite` at mode 644 after the restore (from plan 22-01); still recorded as INFO.
- **README claim not verified:** `deploy/test/README.md` (Output files, `deviations.txt`) says a deviation name outside the documented set fails the run, and the research says the same. `grep` finds no such check in `run.sh`; I did not add one (out of scope) and noted it in `deferred-items.md`.
- `LAST-BEFORE` was `n/a` in every run, so `postinstall`'s `enable --now` does not stamp `LAST` on a fresh unit; the assertion that it is empty held in t3, t5 and the chain's own run.

## Known Stubs

None.

## Threat Flags

None. The new surface is the plan's own threat model: the container-only drop-in (T-22-05, `git diff --quiet HEAD` over the units and scripts is clean), the mode-700 timer directory (T-22-06, asserted), the bulk size check (T-22-07, `backup-fixture` dies when `BULK_MB * 4 + 1024` MB are not free), and the four refusals (T-22-08). `leak_check` passed on all four directories of the chain, and no `hc_`-shaped string occurs under any t3, t4 or t5 directory.

## Next Phase Readiness

Plan 22-03 can run `backup-chain` with `BULK_MB=300` and take the numbers; plan 22-04 can fix `backup.sh` and `restore.sh` against `BAK-01 FAIL` and `BAK-02 FAIL` and watch them turn PASS with the same command.

## Self-Check: PASSED

- `deploy/test/run.sh` defines cmd_backup_timer, cmd_restore_negative, cmd_backup_fixture, cmd_backup_chain and the CLEANUP_TARGETS handling; `deploy/test/README.md` has the `Phase 22: backup, timer and restore` heading and every listed name.
- Commits `a956685`, `8627a20` and `2d8160f` are ancestors of HEAD.
- `git diff --quiet HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer` exits 0.
- No `holzcloud-bak-t5` container remains; the guard with `TARGET=unit-test` exits 1 before docker is touched.
