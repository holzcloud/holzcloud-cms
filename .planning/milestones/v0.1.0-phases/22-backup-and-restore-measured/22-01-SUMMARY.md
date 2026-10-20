---
phase: 22-backup-and-restore-measured
plan: 01
subsystem: infra
tags: [bash, docker, systemd, sqlite, backup, restore, test-harness]

requires:
  - phase: 21-deploy-update-rollback
    provides: the doc-driven harness (doc_cmd, capture_state, compare_states, leak_check) and the populated-site chain build up install baseline update postinstall
provides:
  - "run.sh subcommands backup, transfer and restore between a source and a fresh target container (TGT_NAME, TGT_OUT_DIR, on_target)"
  - "doc_cmd kinds: restore (DEVIATION[backup-name], answer on stdin, per-line container timestamps) and the DOC_TIMED wrapper"
  - "backup_verify with two integrity readers, whole-data-directory sha256 comparison, row counts, MANIFEST, owner and mode, sizes, architecture, media served"
  - "restore proof: time split, set-aside directory, ownership, tree comparison, media over HTTP, marker page, state comparison with selftest, check, COMES-BACK and HAND-REBUILT"
  - "scratch evidence under deploy/test/out/amd64-bak-t2s and amd64-bak-t2t for plans 22-02 and 22-03"
affects: [22-02, 22-03, 22-04, 22-05, phase-23]

actuals:
  tokens: 10900
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "a failing assertion is a BAD line, never an early exit: every subcommand writes all its files, then returns 1; the dispatcher carries the chain backup transfer restore on and ends with 1"
    - "secrets cross containers only by exec-pipe between two docker exec calls; the backup crosses as a tar stream through the host"
    - "every comparison has a selftest that alters a copy and demands a BAD (selftest-tree.txt, compare-selftest.txt)"

key-files:
  created: []
  modified:
    - deploy/test/run.sh

key-decisions:
  - "The restore transcript is stamped inside the container with GNU date; the exit status comes from PIPESTATUS, the script text travels as a bash -c argument so restore.sh's read gets only the answer"
  - "The whole data directory is hashed (exclusions: holzcloud.sqlite*, pre-upgrade-*.sqlite*, MANIFEST, each with its reason as an INFO line), so plugins/ is caught although backup.sh does not copy it"
  - "backup, transfer and restore are dispatched with || PHASE22_RC=1 so one run of the three shows every finding"

requirements-completed: [BAK-01, BAK-02]

coverage:
  - id: D1
    description: "A backup made by the document's own manual line is restored by the document's own restore line, unchanged and answered yes, in a different fresh container, over a tar stream, and the restored site accepts the old login"
    requirement: "BAK-02"
    verification:
      - kind: integration
        ref: "env PLATFORM=linux/amd64 TARGET=systemd-base NEW_REF=2a11eeb run.sh backup transfer restore (task 1 verify, t1s/t1t): RESTORE OK, TRANSFER: equal, SETUP-BEFORE 200, LOGIN 303, SETUP-AFTER 404"
        status: pass
    human_judgment: false
  - id: D2
    description: "BAK-01 integrity by two readers (holzcloud check on a copy, sqlite3 integrity_check on an immutable URI) and content by comparison (whole-directory sha256, row counts), every comparison shown able to fail"
    requirement: "BAK-01"
    verification:
      - kind: integration
        ref: "task 2 verify (t2s/t2t): verify-backup.txt PASS integrity holzcloud-check and sqlite3, SELFTEST OK in selftest-tree.txt and compare-selftest.txt"
        status: pass
    human_judgment: false
  - id: D3
    description: "The measurements (wall seconds, split, sizes, counts, architecture) and the statement of what comes back and what is rebuilt by hand, as KEY lines"
    requirement: "BAK-02"
    verification:
      - kind: integration
        ref: "task 2 verify: backup.txt and restore.txt carry every pinned KEY line"
        status: pass
    human_judgment: false

duration: 22min
completed: 2026-10-14
status: complete
plan_head_before: 8dfa1b478fcc29644d9281e485416f89e6aacf80
plan_head_after: 76cac56dcd06c87ccd2069bf2ee0bd6a7ec15364
commits: 2
---

# Phase 22 Plan 01: Backup and restore across two containers Summary

**The Phase 21 harness now backs up a populated source container with the document's own line, carries the directory to a fresh target as a tar stream, restores it there with the unchanged restore.sh answered `yes`, and proves the result by integrity, a whole-directory hash comparison, row counts, pages, media, login, permissions, time and size. The unchanged scripts fail one thing, exactly as the research predicted: `plugins/` is not in the backup.**

## Performance

- **Duration:** about 22 min (the container runs are short, 2 to 4 minutes per chain)
- **Completed:** 2026-10-14T10:14Z
- **Tasks:** 2 of 2
- **Files modified:** 1 (deploy/test/run.sh, +810 lines)

## Accomplishments

- Two-container support: `on_target` switches NAME, OUT_DIR, INSTALL_FROM and WITH_OLD to `TGT_NAME` and `TGT_OUT_DIR`, restores them afterwards, and dies when `TGT_NAME` equals `NAME` (T-22-04). Nothing in `up` or `install` changed.
- `doc_cmd` gained a `restore` kind (the last word of the document's line is replaced by `RESTORE_DIR`, named `DEVIATION[backup-name]`, `yes` on stdin, every output line stamped with the container's `date +%s.%N`, a `WALL t0 t1` line, exit status from PIPESTATUS) and a `DOC_TIMED=1` wrapper for the plain run kind.
- `backup`: plays the one line of the Backups block that names `/backup.sh`, times it, reads the directory from `=== Backup complete: DIR ===`, and verifies it. `backup_verify` runs `holzcloud check` on a copy and `sqlite3 'file:...?immutable=1' 'PRAGMA integrity_check;'`, checks the MANIFEST keys, `holzcloud 700`, compares the sha256 of the whole source data directory with the backup, compares every table's row count with the state captured just before, and writes sizes, counts, `HOST-ARCH`, `CONTAINER-ARCH` and `MEDIA-SERVED`.
- `transfer`: `docker cp SOURCE:dir - | docker cp - TARGET:/opt/holzcloud/backups`, the three fixtures by exec-pipe, the `ORDER:` line, and a sha256 of every file on both sides (`TRANSFER: equal`).
- `restore`: setup form 200 before, the document's restore line, readyz, login with the old password (303), setup form 404 after; then the time split, set-aside directory, owner and mode, tree comparison against the source and the backup, media over HTTP, marker page, `capture_state tgt` compared with `state-src.txt` (plus its selftest), `check` last, and the derived `COMES-BACK:` and `HAND-REBUILT:` lines.
- Every comparison has a selftest. `selftest-tree.txt` shows an altered list (one line dropped, one hash changed), an empty list and a media-less list each reported; `compare-selftest.txt` shows the same for the state comparison.

## Task Commits

1. **Task 1 (tracer): backup, transfer and restore between two containers** - `25bf2c8` (feat)
2. **Task 2: whole-directory comparison, row counts, cross-container state, measurements** - `76cac56` (feat)

**Plan metadata:** committed with this SUMMARY (docs).

## Files Created/Modified

- `deploy/test/run.sh` - on_target, the restore and timed kinds of doc_cmd, backup, transfer, restore, backup_verify, tree_hash, tree_compare, tree_selftest, rows_compare, dir_sizes, media_served, restore_proof. `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md` and the unit files are untouched (`git diff --quiet HEAD` clean).

## Pass 1 on the unchanged scripts: every BAD line of the final flow (t2s/t2t)

All 39 BAD lines (10 + 12 + 1 + 16 across verify-backup.txt, restore.txt, compare-src-tgt.txt, check.txt) belong to the one class the research predicted (Pitfall 1): `backup.sh` copies only `holzcloud.sqlite`, `csrf.key`, `media/` and `templates/`, so `plugins/` is lost.

- verify-backup.txt, 10 lines: `tree src-backup: only in A: ./plugins/{bestellung,jahreszahl,kontaktformular,nicht-gefunden,suche}/plugin.{json,wasm}`.
- restore.txt, 12 lines: the same ten as `tree src-restored: only in A: ./plugins/...`, plus `state comparison source against target: 52 pass, 1 bad ...; the BAD lines are: plugins running a=5 b=0 (5 expected);` and `check: 1 of 17 assertions passed (exit status 1) ...; the first BAD line is: loaded bestellung: no 'plugin loaded' line ...`.
- compare-src-tgt.txt, 1 line: `BAD plugins running a=5 b=0 (5 expected)`.
- check.txt, 16 lines: `loaded` x5, `no-load-failures` (the journal says `the module is missing from disk`), `running` x5, `invoke` x5 (jahreszahl, kontaktformular 404, bestellung 404, suche, nicht-gefunden).

`tree backup-restored: 4 files equal` passes, so the restore copies exactly what the backup holds. Nothing in `sprachen/` or `marke/` exists in the seeded site, so those two gaps are not exercised (plan 22-02 or 22-04 plants a file in each, per the research).

## Measured (amd64 emulation on an aarch64 host, seed site, 716 KB backup)

backup `WALL-S` 0.502; restore `WALL-S` 1.713 = `VERIFY-S` 0.106 + `STOP-MOVE-COPY-S` 0.217 + `START-READY-S` 1.146 (about one second of resolution, the loop polls once a second); `BYTES` 715976, `DB-BYTES` 663552, `MEDIA-BYTES` 39983, `FILES` 6, `MEDIA-FILES` 3; `HOST-ARCH` aarch64, `CONTAINER-ARCH` x86_64. These are scratch numbers; plan 22-02 and the record take the final ones.

## Decisions Made

- Keep the pinned vocabulary exactly (subcommands, variables, file names, KEY lines, deviation names).
- The BAD summary lines for the state comparison and for `check` carry the text of the underlying BAD lines, so the plan's class filter (`plugin|loaded|running|invoke|...`) judges the real cause and a non-plugin BAD would not hide behind a summary.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Dispatcher stopped the chain at the first BAD line**
- **Found during:** Task 2 (first full run)
- **Issue:** `backup` returned 1 on its predicted BAD lines, so with `set -e` the dispatcher exited and `transfer` and `restore` never ran, although the plan's verify (and plan 22-03's pass 1) needs all three files of one run.
- **Fix:** `backup | transfer | restore` are dispatched with `|| PHASE22_RC=1`; the script ends with `exit "${PHASE22_RC:-0}"`. A `die` still ends the run at once.
- **Files modified:** deploy/test/run.sh
- **Commit:** 76cac56

**2. [Rule 1 - Bug] Summary BAD lines were outside the predicted class by wording**
- **Found during:** Task 2 (first verify)
- **Issue:** `BAD state comparison ... 1 bad` and `BAD check: 1 of 17 ...` contain none of the class words, so the plan's own filter flagged them.
- **Fix:** both lines now append the text of the underlying BAD lines.
- **Commit:** 76cac56

### Plan-text departures (named, each small)

- **Test image mode.** The plan says the three fixtures are written under umask 077. `/tmp/hc-upload.jpg` is written under umask 022: it is no secret and `capture_state` hashes it as the service user (`runuser -u holzcloud`), which cannot read a 600 root file and would report `source none`. The two secret files are 077.
- **No readiness probe file excluded.** The plan lists "the readiness probe file the service writes into the data directory" among the exclusions. In this build `/readyz` asks statfs and writes nothing there (internal/web/health.go), so nothing is excluded for it; such a file would be a finding. The three exclusions are `holzcloud.sqlite*`, `pre-upgrade-*.sqlite*` (the pattern covers the `-wal`/`-shm` files the harness's own ATTACH reads leave next to a snapshot) and `MANIFEST`.
- **COMES-BACK derivation.** It lists the top-level entries of the backup directory that are present in the restored data directory (csrf.key, holzcloud.sqlite, media), and an INFO line lists what the service's own start created (`plugins`, `-wal`, `-shm`). The plan's literal "entries found in the restored data directory" would have listed the empty `plugins/` the service creates, which is not something that came back.
- **`check` through a subshell.** `( cmd_check )` is used so that cmd_check's `exit` becomes a BAD line; run_checks' globals stay isolated.
- **Task 1's own verify no longer ends in 0 after Task 2.** Task 1's verify required `RESTORE OK` and no BAD in restore.txt; with Task 2 the same flow correctly exits 1 on the unchanged scripts. Task 1's verify passed at commit `25bf2c8` (run t1s/t1t) and Task 2's verify, which supersedes it, passed at `76cac56`.

**Total deviations:** 2 auto-fixed (both Rule 1), 4 plan-text departures. **Impact:** none on scope; the vocabulary is unchanged.

## Findings for later plans

- Confirmed defect (plan 22-04): the backup lacks `plugins/`; the restored site has the five plugin rows and no modules (`the module is missing from disk`).
- New observation (plan 22-04, D-16 "chown/mode of the restored tree"): after the restore `holzcloud.sqlite` is mode 644 (restore.sh copies as root under umask 022) where the live file of the source is 600. The 700 data directory keeps it private; recorded as an INFO line in restore.txt, not asserted.
- `read -p` prints no prompt without a terminal, so the `[yes/NO]` line is absent from restore-run.txt.
- The restore transcript shows the set-aside directory holds the first start's empty database plus a `pre-upgrade-...` snapshot; `users` is 0 there.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the tar stream and the exec-pipes of the plan's own threat model (T-22-01, T-22-04); `leak_check` passed for source and target in both full runs, and no `hc_`-shaped string occurs under either OUT_DIR.

## Next Phase Readiness

Plan 22-02 can set `OUT_DIR/timer.txt` with a `DIR:` line and `transfer` will restore that directory (D-03). Scratch evidence for plans 22-02 and 22-03: `deploy/test/out/amd64-bak-t2s` and `deploy/test/out/amd64-bak-t2t` (the containers were removed; the files remain, gitignored).

## Self-Check: PASSED

- `deploy/test/run.sh` defines on_target, backup_verify, cmd_backup, cmd_transfer, cmd_restore, tree_hash, tree_compare, tree_selftest, rows_compare.
- Commits `25bf2c8` and `76cac56` are ancestors of HEAD.
- `git diff --quiet HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md` exits 0 (checked before each commit).
- The `backup` guard against a missing container exits 1 with a `run.sh:` message and creates no container.
