---
phase: 24-the-backup-timer-as-shipped
reviewed: 2026-10-31T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - deploy/test/nightly.sh
  - deploy/test/README.md
findings:
  critical: 0
  warning: 4
  info: 8
  total: 12
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-10-31
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

Scope was the phase 24 part of `deploy/test/nightly.sh` (pure functions, `selftest`, `source`, `target`, `fire`, `delay`, `catchup`, `prune`, `offsite`, `offsite-restore`, `chain`, `down`, `record_check`, `verdict_of`, `req_verdict`) and the Phase 24 section of `deploy/test/README.md`. Later-phase code was read only where it touches phase 24 behaviour. `bash deploy/test/nightly.sh selftest` runs here and ends `SELFTEST OK (646 cases)`.

The harness is carefully built and no BLOCKER turned up: no injection path (all container commands use argv or fixed strings), no secret leaves the containers, and the zone/clock rule is respected. The weaknesses are all in what "every PASS is computed by a pure function" promises:

- Two of the pure verdict functions fail open on non-numeric input. I ran them to confirm.
- `chain` can exit 0 although a stage it ran failed.
- `fire`'s central piece of evidence only exists on a platform whose clock is stepped.
- The documented range of `ZONE_LEAD` is not enforced.

## Warnings

### WR-01: `fire_window_ok` and `catchup_ok` return "ok" for non-numeric input (verdicts fail open)

**File:** `deploy/test/nightly.sh:257-278`, `322-347` (reachable from `1781-1792`)
**Issue:** Every comparison is `[ "$x" -lt "$y" ]` inside an `if`. With an empty or non-numeric argument `[` returns 2, so the `if` is false, the guard is skipped, and the function falls through to `echo "ok"`. I verified it:

```
fire_window_ok "" "" "" "" "" 5 0   -> prints "ok", rc=0
catchup_ok "" "" "" "" "" "" "" "" "" -> prints "ok", rc=0
```

In `cmd_delay` the inputs come from `kv "$f" START-EPOCH` and friends, and `kv` returns the empty string, not `?`, when a key is absent. A `fire.txt` that ends in `FIRE OK` but lacks `LAST-ARMED-EPOCH`, `LAST-DRAWN-EPOCH` or `BASE-EPOCH` (an older harness version, a hand-edited file) turns the "real run" check into a `PASS`. `prune_expect` has the same hole: `prune_expect abc 30` prints `KEEP`. The selftest table has no non-numeric case for any of them, so the suite cannot catch this. This contradicts the file's own rule ("Every PASS and BAD line is computed by a pure function").
**Fix:** Reject anything that is not a whole number at the top of each pure function, and add selftest cases.

```bash
is_whole() { case "$1" in "" | *[!0-9-]* | -) return 1 ;; esac; }
fire_window_ok() {
  local a
  for a in "$1" "$2" "$3" "$4" "$5" "${6:-5}" "${7:-0}"; do
    is_whole "$a" || { echo "not a whole number: '$a'"; return 1; }
  done
  ...
}
# selftest:
st_fail "fire_window_ok empty start" fire_window_ok "" 1000 900 300 60
st_fail "catchup_ok empty stamp"     catchup_ok 1000 900 1100 1105 1110 300 60 30 ""
```

### WR-02: `chain` ignores the status of `run.sh backup` and the failed-step count, so it can exit 0 after a failed stage

**File:** `deploy/test/nightly.sh:6960`, `7005-7008`
**Issue:** `sc_backup` is recorded and printed in the `say` line, but no `req_verdict`, summary line or `fails` increment uses it. `run.sh backup` writes `tree-src-backup.txt`, `tree-backup.txt`, `backup.txt` and `state-src.txt` before it ends with `BACKUP FAILED` (README, `backup` row: "exit 1 after every file is written"). `offsite` and `offsite-restore` only test that those four files exist, so a failed `run.sh backup` leaves BKP-04 `PASS` and the chain exits 0. In the same way `sf` (failed steps from `steps_of`) is printed as `STEPS: N played, F failed` but never counted in `fails`. The README promises that "one failure hides nothing"; these two do.
**Fix:**

```bash
[ "$sc_backup" -eq 0 ] || fails=$((fails + 1))
[ "$sf" -eq 0 ]        || fails=$((fails + 1))
# and print the backup stage in the summary, for example:
echo "BACKUP-STAGE: status $sc_backup"
```

### WR-03: `fire` can only judge the run on a platform that steps the realtime clock

**File:** `deploy/test/nightly.sh:1518`, `1556-1559`, `1623-1627` (and `1306-1335`, `armed_log_collect`)
**Issue:** The armed time of the real run is taken only from PID 1's debug lines ("Realtime timer elapses at ...") logged at or before the start. `armed_log_start` raises the log level to debug at line 1518, after `move_zone`/`follow_zone` have already caused the timer to re-arm and draw its delay. If the clock is not stepped afterwards, no further draw is logged. The only draw is then at info level, in no journal, so `last_armed` stays 0 and the run ends `BAD PID 1 logged no draw of the shipped timer` and `FIRE FAILED`. The code and the README attribute the draws to "the Docker VM steps the realtime clock every 20 to 30 seconds". The project's target is a native linux/amd64 server (phase 26), where there is no such stepping, so the stage reports a false failure there.
**Fix:** Raise the log level before `move_zone`, so the draw made by the zone change is in the journal and is the one the awk picks:

```bash
armed_log_start            # before: move_zone "$ZONE_LEAD" Phase24
move_zone "$ZONE_LEAD" Phase24
```

and call `armed_log_stop` on every exit path (see IN-04).

### WR-04: Numeric knobs are not validated, and the documented range of `ZONE_LEAD` is not enforced

**File:** `deploy/test/nightly.sh:55`, `138-142`, `1488`; `deploy/test/README.md:341`
**Issue:** The header and the README say `ZONE_LEAD` is "60 to 150", but nothing checks it. These values break the stage in confusing ways:
- `ZONE_LEAD=0` puts 03:15:00 at "now", so the armed time is not after the base.
- `ZONE_LEAD=300` with the default `FIRE_WAIT=540` can need `300 + 300 + 60` seconds, so the stage fails with "did not run within 540 s".
- A non-numeric value makes the arithmetic in `zone_offset_seconds` abort with a bash syntax error mid-stage, after `fire` has already checked the container and begun to change it.

`FIRE_WAIT`, `DELAY_SAMPLES`, `CATCHUP_LEAD` and `CATCHUP_WAIT` have the same problem.
**Fix:** Validate once at start-up, before any container is touched:

```bash
for v in ZONE_LEAD FIRE_WAIT DELAY_SAMPLES CATCHUP_LEAD CATCHUP_WAIT; do
  case "${!v}" in "" | *[!0-9]*) die "$v must be a whole number (got '${!v}')" ;; esac
done
in_range "$ZONE_LEAD" 60 150 || die "ZONE_LEAD must be 60 to 150 (got $ZONE_LEAD)"
[ "$FIRE_WAIT" -ge $((ZONE_LEAD + 300 + 60 + 60)) ] || die "FIRE_WAIT $FIRE_WAIT is too short for ZONE_LEAD $ZONE_LEAD"
```

(`${!v}` works in bash 3.2.)

## Info

### IN-01: README describes `record-check` as proving the wrong record

**File:** `deploy/test/README.md:361`
**Issue:** "reproduces the appendix proof of `deploy/BACKUP-RUN.md`". The default for `R` in `cmd_record_check` (nightly.sh:7017) is `deploy/BACKUP-TIMER-RUN.md`, and the section is about the phase 24 record. `BACKUP-RUN.md` is the phase 22 record.
**Fix:** Name `deploy/BACKUP-TIMER-RUN.md`.

### IN-02: Deviation kinds and one knob the script emits or reads are undocumented

**File:** `deploy/test/README.md:346-347`, `341-342`; `deploy/test/nightly.sh:1299`, `2409`, `1761`
**Issue:** The README's "Deviation names" list omits `zone-rearm` (written by `follow_zone`) and `network` (written by `apt_in`, which then exits 3). `OBSERVE_S` (default 75, read by `delay`) is neither in the README variable list nor in the script header. The result-file list also omits `offsite-apt.txt`, `offsite-start-a.txt`, `offsite-start-b.txt`, `prune-start.txt`, `prune-final.txt` and `offsite-restore-run.log`. A reader sees them in `out/` and cannot tell whether they belong in the record.
**Fix:** Add them to the README (and `OBSERVE_S` to the header's environment list).

### IN-03: Required headings and requirement headings are checked against the whole record, appendix included

**File:** `deploy/test/nightly.sh:545`, `551`
**Issue:** `(b)` and `(d)` read `$t/body` (everything before the appendix), but `(c)` greps `"$r"`. A heading that exists only as a line inside pasted raw output satisfies check (c). The README says such lines cannot be pasted unchanged, which makes this unlikely, but the fix costs nothing.
**Fix:** `grep -qxF -- "$h" "$t/body"` and `grep -q "^## ...$i" "$t/body"`. Also anchor the requirement heading (`"^## ${prefix}$i\\b"` or `[^0-9]`) so `## BKP-010` does not satisfy `BKP-01`.

### IN-04: PID 1 is left at debug log level when `fire` or `delay` aborts

**File:** `deploy/test/nightly.sh:1314-1315`, `1518`, `1762`, `1772`
**Issue:** `armed_log_stop` runs only on the two explicit paths in `fire`. Any `set -e` abort or `die` between `armed_log_start` and `armed_log_stop`, and the same in `delay` between lines 1762 and 1772, leaves the container's systemd at debug. `catchup` has an EXIT trap for its zone and control units; these two have none.
**Fix:** `trap armed_log_stop EXIT` after `armed_log_start` (and the same pattern around the `delay` observation loop).

### IN-05: Restoring the zone creates a symlink loop if `/etc/localtime` was a regular file

**File:** `deploy/test/nightly.sh:1258`, `1854-1857`
**Issue:** `move_zone` records `readlink -f /etc/localtime`. For a regular file that prints the path itself, `/etc/localtime`. `catchup_restore` then runs `ln -sfn /etc/localtime /etc/localtime`, which writes a self-referencing link and leaves the container with no valid zone. With the Debian image used here `/etc/localtime` is a symlink, so it works today.
**Fix:** In `move_zone`, store `none` or copy the original to `/root/.nightly-localtime.orig` when `[ ! -L /etc/localtime ]`, and restore from that.

### IN-06: `prune` leaves planted directories behind when it aborts after planting

**File:** `deploy/test/nightly.sh:2219-2232`, `2346-2352`
**Issue:** Cleanup (step 6) is on the happy path only. A `die` at 2231/2232 (a partial plant) or any abort before step 6 leaves 45/31/30/29/1-day directories holding a fake `holzcloud.sqlite` in `/opt/holzcloud/backups`. A rerun then lists them in `MUST-STAY` and reports `the earlier backup ... is gone` as a BAD when the run prunes them. The `offsite` sha comparison would also carry them.
**Fix:** Register an EXIT trap right after planting that removes the five names.

### IN-07: Dead and shadowed code, and small hygiene items

**File:** `deploy/test/nightly.sh:174-177`, `643`, `6899-6902`, `6929-6935`
**Issue:**
- The first `usage()` (`sed -n '2,25p'`) is overwritten by the definition at line 2975 and never runs. Its range stops at the `compat-source` line, so it would not even print `down`.
- `selftest_zone` assigns an undeclared `sh` (line 643), which leaks into the global namespace.
- `cmd_down` prints `down: X and Y are gone` even when the `could not remove` message was just printed.
- The `chain` evidence-directory guard is an exact-match blacklist (`run.sh cmd_up` has the same one). It does not refuse `TGT_OUT_DIR` being a parent of `OUT_DIR` (the `target` stage's `rm -rf` would then delete the finished source evidence) or `OUT_DIR=deploy/test/out`, which deletes every other phase's evidence.
**Fix:** Delete the first `usage`; add `sh` to the `local` list; count failed removals and say `could not remove` in the final line; in `chain` reject any `OUT_DIR` where `"$a/" = "$b/"*` for the pair, and reject a path outside `$OUT_ROOT/`.

### IN-08: `offsite-apt.txt` is appended to and never reset, so an old network failure can mislabel a new one

**File:** `deploy/test/nightly.sh:2402-2413`, `2510-2512`, `2526`
**Issue:** `offsite` truncates `offsite.txt`, `offsite-host2.txt` and `offsite-natural.txt` at the start but not `offsite-apt.txt`. `apt_in` greps the whole accumulated file for "could not resolve" / "unable to locate package" when `apt-get` fails. After one run that failed for want of a network, a later non-network `apt-get` failure is reported as `DEVIATION[network]` with exit 3 ("platform cannot give the evidence") instead of a real failure. `deviations.txt` also grows duplicate lines on repeated stages. (`chain` is not affected, because `source` wipes `OUT_DIR`.)
**Fix:** `: > "$OUT_DIR/offsite-apt.txt"` next to the other truncations, or have `apt_in` judge only the output of the call it just made.

---

_Reviewed: 2026-10-31_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
