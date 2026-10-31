---
phase: 25-consistent-and-compatible-backups
reviewed: 2026-10-31T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - deploy/BACKUP-CONSISTENCY-RUN.md
  - deploy/test/README.md
  - deploy/test/nightly.sh
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-10-31
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

`nightly.sh` is now 7042 lines. It was reviewed in its current state, including the phase 29 changes (`vanish_*`, `capfull_robust_verdict`, `R_RETRIES`/`R_LEFT`, `cf_run_unit`). Tools were limited: `shellcheck` is not installed. These checks were done instead:

- `bash -n` passes.
- `nightly.sh selftest` ends `SELFTEST OK (646 cases)`.
- `nightly.sh record-check-con` ends `RECORD-CHECK OK 27 blocks`.
- The record's headline numbers were recomputed from `deploy/test/out/amd64-t25-src/consist-runs.txt`. All of these agree with the record: run counts per class, the BACKUP-S minimum, maximum and mean, the run numbers of the 11 ROW-WITHOUT-FILE and 8 BACKUP-REFUSED runs, 220 `WINDOW-NAME` lines, and the uploads, page saves and deletes in the window. The record's `deploy/backup.sh:59/:64/:83` line references match `git show e86d490:deploy/backup.sh`.

No secret is hardcoded. The leak check covers the password and key files of both container pairs.

Nothing found is a blocker. The harness's verdict functions are conservative: the defects below make it fail too often or lose evidence, and do not turn a failure into a PASS. The real problems are:

- One assertion added in phase 29 loses the evidence of the failure it exists to catch (WR-01).
- A function called from an `if` condition runs with `set -e` off, which the same README warns about (WR-02).
- Some pieces of the harness read the working tree and not the pinned ref (WR-04).
- The record and the README still state bound-file facts that phase 29 made false (WR-05).

## Warnings

### WR-01: A leftover MANIFEST-less directory in an otherwise AGREE run is flagged BAD, but its evidence is thrown away

**File:** `deploy/test/nightly.sh:3606-3618` (check), `:3725` (`nbad0=$BADS`), `:3773-3776` (`R_BADRUN`), `:3790` (`keep`), `:3794-3796` (cleanup)
**Issue:** Phase 29 added a count of directories without a `MANIFEST` after each night (`R_LEFT`), with a `check_bad` when it is above 0. That `check_bad` runs before `nbad0=$BADS` (line 3725). `R_BADRUN=yes` is only set when `BADS > nbad0` at line 3773, so the `R_LEFT` BAD never marks the run as bad.

`consist_finish` keeps the raw files of a run only if `n = 1`, `CLASS != AGREE`, or `R_BADRUN = yes`. A night that backs up, restores and compares as AGREE but leaves another directory behind therefore loses `backup.txt`, `restore.txt` and `compare.txt`. Only `R_DIR`, the run's own directory, is removed. The offending leftover directory stays, so the check fires again on every following run. It adds one BAD per run (up to ~79 BADs), none of them with raw files, and the first cause is lost.
**Fix:** Set `R_BADRUN=yes` where the `R_LEFT` BAD is raised, and keep the raw files for it:
```bash
if [ "$R_LEFT" -gt 0 ]; then
  R_BADRUN=yes
  check_bad "run $stage $n: $R_LEFT director(ies) without a MANIFEST ..."
fi
```
Also list the offending names in the BAD line (`find ... -print`, not `| wc -l`) so the evidence survives even when the run is not kept.

### WR-02: `consist_run` is called as an `if` condition, so `set -e` is off for its entire 250-line body

**File:** `deploy/test/nightly.sh:3905` (`if consist_run ...; then`), body `:3540-3780`
**Issue:** `deploy/test/README.md:359` and `:404` and the comment above `chain_stage` (`nightly.sh:6930`) record the rule: a function on the left of `||` or in an `if` condition runs with `set -e` off. `consist_run` is called exactly that way. Every unguarded command in it is then silently ignored. Examples are `cp "$w/backup.txt" "$OUT_DIR/..." 2>/dev/null || true`-style copies, `mkdir -p`, and the `dx` calls whose failure is not tested. A failing command in the middle of a run continues into the comparison with stale or empty data.

Where the code does check explicitly (`|| return 1`) it is fine. Unchecked lines, for example the `docker cp` pipeline's neighbours and the `churn_stop || check_bad` pattern, depend on every author remembering the rule. The `capfull_*` and `window` stages use the same shape.
**Fix:** Either run each run as a child process, the way `chain_stage` does, or make the contract explicit. Add `|| return 1` to each command whose failure matters, and document at the top of `consist_run` that `set -e` is off.

### WR-03: `cf_run_unit` and the inline unit run in `capfull_run` never check that a new invocation started

**File:** `deploy/test/nightly.sh:5983-6010` (`cf_run_unit`), `:6233-6258` (inline copy in `capfull_run` step 5)
**Issue:** Both wait up to 30 s for `InvocationID` to change. After the loop neither checks `[ "$inv" != "$inv0" ]`. If `systemctl start --no-block` did not produce a new invocation (start limit, a dependency job still queued, a stale `reset-failed`), the code carries on. It polls `ActiveState`, which is already `inactive` or `failed` from the previous run, and reads `Result` and `ExecMainStatus` of the previous invocation. It then attributes them to the scenario, and the journal is queried with the old id.

Today this only produces false FAILs. The adjacent runs differ in outcome (exit 11, then 0, then 24), but the harness cannot tell a mis-recorded run from a real one. When `inv0` is empty and the loop times out, `journalctl "_SYSTEMD_INVOCATION_ID="` is run with an empty id.
**Fix:** After the first loop add:
```bash
[ "$inv" != "$inv0" ] && [ -n "$inv" ] || { check_bad "$label: no new invocation of holzcloud-backup.service within 30 s"; ...; return; }
```
Do the same in the inline copy, or better, make `capfull_run` call `cf_run_unit` (see IN-04).

### WR-04: Mechanism readings and document lines come from the working tree, not from the pinned `NEW_REF`

**File:** `deploy/test/nightly.sh:4616-4652` (`window_mechanisms`: `src_line internal/media/intake.go ...`, `deploy/backup.sh`), `doc_backup_line`/`doc_restore_line` (reads `deploy/DEPLOY.md`), `:1211` `pin_ref`
**Issue:** `con-chain` pins `NEW_REF` to one sha precisely so that the binaries do not depend on what lands mid-run. The binary comes from a clone at that ref. But `window_mechanisms` greps `internal/media/*.go`, `internal/db/maintain.go`, `cmd/holzcloud/cli.go` and `deploy/backup.sh` in the checkout, and prints `file:LINE` into `window.txt`. The installed `backup.sh` and the document line also come from the checkout.

If `NEW_REF` is not `HEAD`, or the tree is dirty or has moved on, the INFO lines cite line numbers of code that is not what ran. The record then claims they "were checked against the tree at the evidence commit". Phase 29 already moved `backup.sh` line numbers. The check is also not tied to `evidence-commit.txt`.
**Fix:** Read from the pinned ref:
```bash
src_line() { git show "${NEW_REF}:$1" | grep -n -F -m 1 -- "$2" | cut -d: -f1; }
```
Apply the same to `doc_backup_line`/`doc_restore_line`, or refuse to run when `git rev-parse HEAD` differs from `NEW_REF` or `git status --porcelain -- deploy internal cmd` is not empty.

### WR-05: The record and the README state bound-file facts that phase 29 made false, with no pointer to the successor record

**File:** `deploy/BACKUP-CONSISTENCY-RUN.md:33-37, 41-43, 265, 267`; `deploy/test/README.md:365, 369, 409-411, 447`
**Issue:** The record says "No bound file changed after the evidence commit" and shows `git diff --quiet e86d490 HEAD -- deploy/backup.sh ... deploy/test/nightly.sh` as having exited 0. At HEAD that command exits 1 (verified). Phase 29 changed `backup.sh` (+94 lines, the repeat of rsync status 24 and the removal of a failed night's directory) and `nightly.sh` (+593 lines).

F-3, F-4 and F-5, and the "Left to the owner" paragraph, describe the shipped `backup.sh` as it was at `e86d490`. At HEAD that behaviour is deliberately different: a first status 24 is repeated, and a failed night's directory is removed. The record has no "superseded by `deploy/BACKUP-ROBUST-RUN.md`" note. `docs/beta.md` already cites the newer record (P-19, P-20). The README is stale in the same way:
- Line 411 says `SELFTEST OK (542 cases)`; the current count is 646.
- Line 409 says the diff exits 0 while the evidence holds.
- Line 369 says "`nightly.sh` grew by additions".
- The tripwire `git diff -U0 b501b1b -- nightly.sh | grep -c '^-[^-]'` printed 3 today, so it still holds. The "542" claim and the line 409 claim do not.
- Line 447 repeats "still ends SELFTEST OK (542 cases)" for phase 26.

A reader who follows the record's own verification command gets a failure and cannot tell whether the evidence or HEAD is wrong.
**Fix:** Add a short note at the top of the record. It should say the record binds the files at `e86d490` only, that `backup.sh` and `nightly.sh` were changed in phase 29, and that F-3 to F-5 are historical, with a pointer to `deploy/BACKUP-ROBUST-RUN.md`. Change the command in "How this was run" to `git diff --quiet e86d490 <commit> -- ...` with the phase 24/25 boundary named, or mark it historical. In the README, state the selftest count as "at least 542" or update it per phase.

### WR-06: Destructive `rm -rf` on the "target" container with no `NAME != TGT_NAME` guard in `consist`, `window` and `compat`

**File:** `deploy/test/nightly.sh:3661` (`dxt bash -c 'rm -rf /opt/holzcloud/data.before-restore-* /opt/holzcloud/backups/*'`); guards exist only at `:1232`, `:2500`, `:2755`; `con_distinct` at `:6442`, `con_dir_safe` at `:6462`
**Issue:** `cmd_consist` wipes `/opt/holzcloud/backups/*` on `$TGT_NAME` before every restored run and then restores onto it. Unlike `target`, `offsite` and `offsite-restore`, `consist`, `window` and `compat` do not refuse `TGT_NAME == NAME`. A mis-set environment points the wipe and the restore at the source, destroying the source's backups and replacing its data directory. The container is throwaway, but a long run is lost.

Separately, `con_distinct` and `con_dir_safe` compare strings only. `deploy/test/out/x` and `deploy/test/out/x/` or `./deploy/test/out/x` count as different directories. `con_dir_safe` accepts `deploy`, `deploy/test` or `$ROOT/..`. Today `con_dir_empty` happens to stop most of these.
**Fix:** Add `[ "$NAME" != "$TGT_NAME" ] || die ...` to `cmd_consist`, `cmd_window` and `compat_run`. Canonicalise before comparing, with `cd "$d" && pwd -P` or a trailing-slash and `./` strip.

## Info

### IN-01: The first `usage()` is dead code, and the live one breaks under a relative invocation

**File:** `deploy/test/nightly.sh:174-178` (dead), `:2974-2977` (live), `:88-89`
**Issue:** The first `usage()` prints `sed -n '2,25p'`, which cuts the subcommand list off, and is overridden by the second definition at line 2974. Both read `"${BASH_SOURCE[0]}"` after `cd "$ROOT"`. Running `bash nightly.sh` from `deploy/test/` prints `awk: can't open file nightly.sh` instead of the usage text (verified). `ROOT` was computed before the `cd` but `BASH_SOURCE[0]` was not made absolute.
**Fix:** Delete the first definition and set `SELF=$ROOT/deploy/test/nightly.sh` after `ROOT` is computed. Use it in `usage` and in other self-reads.

### IN-02: The header comment still documents `CHURN_WORKERS` as 2

**File:** `deploy/test/nightly.sh:71` versus `:150` (`CHURN_WORKERS=${CHURN_WORKERS:-4}`)
**Issue:** The code default is 4 and `consist.txt` prints it. The README (`:393`) admits "the header comment of `nightly.sh` still says 2" and leaves it, while `usage` prints that header to the user.
**Fix:** Correct the header to 4 uploaders and drop the apology sentence from the README.

### IN-03: Step (4) of `capfull` can no longer run, which lost its coverage of `restore.sh` on a partial directory

**File:** `deploy/test/nightly.sh:6213-6229`, README `:388`
**Issue:** Since phase 29, a passing hand run removes its own directory, so `CF_HAND_DIR` is `none` and step (4) always records `RESTORE-PARTIAL-EXIT: not-run`. The README still describes `capfull` as playing "`restore.sh` on what is left". The refusal of a MANIFEST-less directory by `restore.sh` is therefore no longer exercised in `con-chain`. Only the phase 22 `restore-no-sqlite` scenario covers it, and that scenario uses a different directory shape.
**Fix:** Either plant a MANIFEST-less directory on purpose (SIGKILL case, as the README's "named limit" describes) and point `restore.sh` at it, or update the README and the summary line to say the step is not run.

### IN-04: The unit-start sequence exists twice

**File:** `deploy/test/nightly.sh:5983-6010` (`cf_run_unit`), `:6233-6258` (inline in `capfull_run`)
**Issue:** The comment on `cf_run_unit` says the logic is "copied here, stage (5) keeps its own inline code". Two copies of the same polling and bounds drift (both lack the check in WR-03).
**Fix:** Make `capfull_run` step (5) call `cf_run_unit UNIT "$OUT_DIR/capfull-unit.txt"`.

### IN-05: `consist_finish` runs `rm -rf` on a path taken from the transcript, and early returns leave `.work-*` directories behind

**File:** `deploy/test/nightly.sh:3596-3597` (`bdir=/opt/holzcloud/backups/$ts`), `:3794-3796`, early `return 1` at `:3568-3705`
**Issue:**
1. When `ts` is empty, `R_DIR` becomes `/opt/holzcloud/backups/` with a trailing slash. `consist_finish` guards only against empty and `?`, so `dx rm -rf "$R_DIR"` would delete the whole backup root. It is not reachable today, because only AGREE runs reach that branch and AGREE needs a directory.
2. Every early `return 1` before `consist_finish` skips `rm -rf "$w"`. Failed runs leave `.work-<pfx>` directories (with `backup.txt` and churn logs) in the evidence directory.
**Fix:** In `consist_finish`, delete only when `case "$R_DIR" in /opt/holzcloud/backups/?*) ... esac`. Put `trap`-free cleanup of `$w` in the caller of `consist_run`.

---

_Reviewed: 2026-10-31_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
