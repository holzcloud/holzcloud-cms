---
phase: 22-backup-and-restore-measured
reviewed: 2026-10-14T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - deploy/test/run.sh
  - deploy/backup.sh
  - deploy/restore.sh
  - deploy/DEPLOY.md
  - deploy/BACKUP-RUN.md
  - deploy/test/README.md
  - docs/deployment.md
  - docs/configuration.md
findings:
  critical: 0
  warning: 9
  info: 6
  total: 15
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-10-14
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

The focus was `backup.sh` and `restore.sh` (the production scripts changed in this phase), then the phase 22 half of `run.sh` (lines 2565-3902), then the documents. `bash -n` passes on all three scripts. `shellcheck` is not installed here, so no linter output backs these findings. I traced the scripts against the Go side (`cmd/holzcloud/cli.go`, `internal/db/maintain.go`, `internal/config/config.go`) to confirm what `backup`, `check` and `version` actually do.

The loop over the five directories matches the directories the Go code writes under the data directory: `media`, `templates`, `plugins`, `sprachen` (`i18n.DirName`) and `marke` (`branding.DirName`), plus `csrf.key`. The MANIFEST-last ordering and the snapshot verification before anything is stopped are sound.

The weaknesses are in the failure paths, which the harness did not exercise. These are:
- a backup that can look green while holding nothing;
- a restore that can strand the site stopped;
- two behaviours that the comments and `DEPLOY.md` describe differently from what the code does.

No BLOCKER was proven. Warning 1, 2 and 5 are the closest to one.

`docs/configuration.md` and `docs/deployment.md` are accurate against the code.

## Warnings

### WR-01: A file that vanishes during the rsync kills the backup, contradicting what the script and DEPLOY.md say

**File:** `deploy/backup.sh:80-85` (comment at 25-29; `deploy/DEPLOY.md`, "The pair is not atomic")
**Issue:**
- The header comment and DEPLOY.md say that "a file deleted in between can leave a row whose file the backup lacks". That holds only for a file deleted before rsync lists the directory.
- A file removed between rsync's scan and its transfer makes rsync exit 24 ("some files vanished"). Under `set -euo pipefail` that ends the script at line 83.
- The result is a half-written `${BACKUP_DIR}` with no MANIFEST. The service unit fails, and retention (step 5) and the off-box copy (step 6) are skipped.
- The incomplete directory stays in the backups folder, where `restore.sh`'s "available" listing shows it next to good ones. It is removed only by a later successful run, after `RETENTION_DAYS`.
- The nightly job runs against a live site whose media can be deleted at any time, so this will happen on a busy site.
- Refusing the MANIFEST-less directory is the safe outcome. But the site then has no backup for that night, and the documentation describes a different failure.

**Fix:** Tolerate exit 24 explicitly and let the MANIFEST be written:
```bash
rsync_dir() {
    local rc=0
    rsync -a "$1/" "$2/" || rc=$?
    if [ "${rc}" -eq 24 ]; then
        echo "WARNING: files vanished while copying $1; the backup holds the rest" >&2
    elif [ "${rc}" -ne 0 ]; then
        return "${rc}"
    fi
}
```
Also remove the incomplete `${BACKUP_DIR}` on any other failure, with a `trap ... ERR` or `EXIT` that removes it unless the MANIFEST exists. Correct the wording in DEPLOY.md to match whichever behaviour is chosen.

### WR-02: backup.sh never checks that the live database exists, so an empty data directory yields a green, "verified" backup of nothing

**File:** `deploy/backup.sh:69` (with `cmd/holzcloud/cli.go:101-114`, `internal/db/maintain.go:68-97`)
**Issue:**
- `holzcloud backup` goes through `openForCLI`, which runs `db.Open` and then `RunMigrations`. Neither refuses a missing file. SQLite creates `holzcloud.sqlite` and goose builds the full schema in it.
- `VACUUM INTO` then snapshots that empty database. The integrity check passes and the table count is greater than zero.
- So if `DATA_DIR` points at an existing directory that holds no database, the script exits 0 with a verified, MANIFEST-complete backup of an empty site, and it creates a stray database in the wrong directory.
- Three triggers are realistic:
  - a typo in `DATA_DIR`;
  - a manual run with `DATA_DIR=/opt/holzcloud`;
  - the scenario DEPLOY.md's restore section is written for: the data volume is lost or remounted empty while the timer keeps firing.
- The timer then shows success every night. `RETENTION_DAYS` pruning keeps the empty backups and discards the good ones after 30 days. This is the failure mode the script's own header describes ("a corrupted database ... copied over the last good snapshot").
- The harness cannot catch it. `restore-negative` case d tests only the free-space refusal.

**Fix:** Add a guard before step 1:
```bash
if [ ! -s "${DATA_DIR}/holzcloud.sqlite" ]; then
    echo "ERROR: ${DATA_DIR}/holzcloud.sqlite does not exist; refusing to snapshot an empty database" >&2
    exit 1
fi
```
Add a matching negative case to `restore-negative`, or a new `backup-negative`.

### WR-03: Retention runs `rm -rf` on every directory in the backup root and hides every error

**File:** `deploy/backup.sh:98-102`
**Issue:**
- `find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -mtime +N -exec rm -rf {} +` deletes any subdirectory older than N days. It does not check that the name looks like a backup (`YYYYMMDD-HHMMSS`).
- `BACKUP_ROOT` is `$1`. The script is documented as `./backup.sh [backup_dir]`, so `backup.sh /opt/holzcloud` or `backup.sh /srv` is a plausible mistake. That deletes `data.before-restore-*` (the set-aside copy `restore.sh` promises never to delete), `deploy`, and anything else old.
- `2>/dev/null || true` hides permission errors and failures. A disk that fills because pruning silently failed is exactly what step 5's comment warns about, and nothing reports it.
- Directory mtime is also a weak proxy for age. `scp -r` copies of backups into the root get a fresh mtime and are never pruned; unpacked backups are pruned on their own mtime.

**Fix:** Restrict the match and stop swallowing errors:
```bash
find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d \
     -name '[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9]' \
     -mtime "+${RETENTION_DAYS}" -exec rm -rf {} + \
  || echo "WARNING: pruning old backups failed" >&2
```

### WR-04: The off-box copy is a `--delete` mirror, and the shipped unit has no way to run ssh

**File:** `deploy/backup.sh:106-109`; `deploy/holzcloud-backup.service` (`User=holzcloud`, `ProtectHome=true`); `deploy/DEPLOY.md:96-97` (`useradd -r -s nologin`, no home)
**Issue:**
- `rsync -a --delete BACKUP_ROOT/ REMOTE_TARGET/` mirrors local state to the remote. Anything that empties or damages the local root propagates to the only off-box copy on the next run: a missing mount, a wrong `BACKUP_DIR` argument, or the pruning bugs above. The remote then holds no history of its own, only what the local box holds.
- The unit that runs it uses a system user created with no home directory, under `ProtectHome=true` and `ProtectSystem=strict`. The documented remote form is `user@nas:/backups/...`, i.e. ssh. There is no place for the key or `known_hosts`, and the script cannot pass `-e "ssh -i ..."`.
- DEPLOY.md says "a backup that has never left the machine is not yet a backup" and tells the operator to set `REMOTE_TARGET`, but gives no working recipe. The record admits this path never ran: `BACKUP-RUN.md` lines 152 and 228, `README.md` line 299.
- The result is a documented safety feature that is likely not to work as shipped, and an operator will find out only when the unit fails.

**Fix:**
- Drop `--delete`, or guard it so it only runs when the local root holds at least one complete backup.
- Add an `RSYNC_RSH` (or `RSYNC_SSH_OPTS`) environment variable, and document a concrete setup for the key under `/opt/holzcloud/.ssh` or `/var/lib/...`. Add that path to `ReadWritePaths`, or run the remote copy as a separate unit.
- Until it is proven, say so in DEPLOY.md instead of presenting it as available.

### WR-05: restore.sh has no recovery path after the service is stopped and the data directory is moved

**File:** `deploy/restore.sh:82-109`
**Issue:**
- After `systemctl stop` (line 83) and `mv` (line 89), every step runs under `set -e` with no trap and no message.
- Any failure leaves the site down with a half-built `${DATA_DIR}`, and the script exits non-zero with no explanation of the state. The failures include: `cp` or `rsync` hitting a full disk, a bad `OWNER` making `chown` fail, or `mv` failing.
- `mv "${DATA_DIR}" "${ASIDE}"` fails with "Device or resource busy" if `${DATA_DIR}` is a mount point. DEPLOY.md line 38 recommends keeping the data directory on a filesystem of its own. `BACKUP-RUN.md` lists "a separate data volume" as not tested.
- The failure happens after the service has already been stopped.
- A relative or in-tree `BACKUP_DIR` inside `${DATA_DIR}` vanishes with the `mv`.
- Nothing refuses a same-second `ASIDE` name collision. A second run in the same second would move the new directory inside the first.
- The aside copy is never lost, which is good, but the operator is not told where it is or how to go back.

**Fix:**
- Before stopping anything, refuse when `mountpoint -q "${DATA_DIR}"` is true, or when `BACKUP_DIR` is under `${DATA_DIR}`.
- After the stop, install an `ERR` or `EXIT` trap that prints the aside path and the exact undo commands:
```bash
on_fail() {
    echo "RESTORE FAILED: ${SERVICE} is stopped. Previous data is in ${ASIDE:-<none>}." >&2
    echo "  To go back: rm -rf '${DATA_DIR}' && mv '${ASIDE}' '${DATA_DIR}' && systemctl start ${SERVICE}" >&2
}
trap on_fail ERR
```
- Add `-$$` or a nanosecond suffix to `ASIDE`.

### WR-06: restore.sh does not stop the backup timer, which can fire against the half-restored directory

**File:** `deploy/restore.sh:82-112` (with `deploy/holzcloud-backup.timer`, `deploy/holzcloud-backup.service`)
**Issue:**
- The window between `mv` (line 89) and the final `systemctl start` is several seconds. The unit file has `After=holzcloud.service` only as an ordering hint; it does not stop the backup service from running while holzcloud is down.
- If the timer fires in that window, `backup.sh` runs `holzcloud backup` on the not-yet-populated `${DATA_DIR}`. Per WR-02 that creates and snapshots an empty database, and the run ends with a valid MANIFEST.
- It can also create `holzcloud.sqlite` before the `cp` at line 94, racing the restore.

**Fix:** Before stopping the service, run `systemctl stop holzcloud-backup.timer holzcloud-backup.service`, and start the timer again only after `/readyz` answers. Or take a `flock` shared by `backup.sh` and `restore.sh`. WR-02's guard also closes the empty-snapshot half.

### WR-07: restore.sh hard-codes port 8080 for `/readyz`, so a successful restore exits 1 on any other port

**File:** `deploy/restore.sh:115-126`
**Issue:**
- The service port is configurable (`HOLZCLOUD_PORT`, `internal/config/config.go:308`). The script polls `http://127.0.0.1:8080/readyz` unconditionally.
- A server running on another port gets a completed restore followed by "WARNING: /readyz did not report ready within 30s" and exit 1. The operator will think the restore failed.
- DEPLOY.md admits the URL is not tied to `DATA_DIR` but not that it ignores `HOLZCLOUD_PORT`.

**Fix:**
```bash
PORT="${HOLZCLOUD_PORT:-8080}"
...
curl -fsS "http://127.0.0.1:${PORT}/readyz"
```
Document `HOLZCLOUD_PORT` in the script's environment list.

### WR-08: Every phase 22 stage runs with `set -e` switched off

**File:** `deploy/test/run.sh:2583` (`on_target`), `3783-3787, 3801-3805` (`backup-chain`), `3884-3896` (dispatcher)
**Issue:**
- Each stage is called as `"$fn" || rc=$?`, `( cmd_x ) || sc=$?`, or `"cmd_$sub" || PHASE22_RC=1`. Bash ignores `errexit` inside a command that is part of an `||` list, including inside functions and subshells it calls.
- The stage bodies were written as if `set -euo pipefail` (line 69) held. Examples are `cmd_up`'s bare `docker run -d ...` (line 545) and `capture_state src`.
- Concrete effect: when `on_target cmd_up` is run and `docker run` fails (name taken, platform pull error, daemon down), the script does not stop. It polls `systemctl is-system-running` for 90 seconds in a container that does not exist, then dies with the unrelated message "systemd did not reach running or degraded". The first failure is hidden.
- The same applies to `cmd_install` and the other stages run through `on_target`.
- This undermines "a step that fails is recorded, never hidden".

**Fix:** Add `|| die` to the commands that matter, starting with `docker run`:
```bash
docker run -d --name "$NAME" ... >/dev/null || die "docker run of $NAME failed"
```
Or run each stage as `( set -e; "$fn" "$@" )` and capture the status separately:
```bash
rc=0
( set -e; "$fn" "$@" ) || rc=$?
```
That subshell form does not help for the `||` itself, so it needs `if` or `set +e` handling. The explicit `|| die` is the simpler, reliable fix.

### WR-09: The BAK-01, BAK-02 and BAK-03 verdicts in `backup-chain` ignore the stage exit statuses

**File:** `deploy/test/run.sh:3811-3824`
**Issue:**
- `bak01` is `PASS` when three `verify-*.txt` files exist, are non-empty and hold no BAD line. The captured `sc_backup`, `sc_timer` and `sc_bbackup` are not consulted.
- `cmd_backup` writes `verify-backup.txt` and then calls `leak_check`, which calls `exit 1` on a leak. `cmd_backup_timer` also runs `leak_check`. A stage that dies after writing a clean file therefore still counts.
- `bak03` is `file_clean timer.txt` plus a TRIGGERED-BY line, not `sc_timer == 0`.
- The summary prints stage statuses on a separate line but does not gate on them.
- This matters because the summary is the artifact later plans and phase 23 grep.

**Fix:** Require the statuses too:
```bash
if [ "$sc_backup" -eq 0 ] && [ "$sc_bbackup" -eq 0 ] && file_clean ... ; then bak01=PASS; fi
if [ "$sc_timer" -eq 0 ] && file_clean "$seed_out/timer.txt" ...; then bak03=PASS; fi
```
`sc_transfer`, `sc_btransfer` and `sc_neg` also need to gate their dependent verdicts.

## Info

### IN-01: Unquoted binary path and data dir in the MANIFEST `version=` substitution

**File:** `deploy/backup.sh:92`
**Issue:**
- `$(HOLZCLOUD_DATA_DIR=${DATA_DIR} ${HOLZCLOUD_BIN} version ...)` leaves `${HOLZCLOUD_BIN}` unquoted. A path with a space splits into two words and the command fails. The `|| echo unknown` swallows that, so the MANIFEST silently says `version=unknown`.
- `version` does not read the data directory at all, so that prefix is dead.

**Fix:** `version=$("${HOLZCLOUD_BIN}" version 2>/dev/null || echo unknown)`. Quote it the same way at the other call sites; line 69 already does.

### IN-02: A closed stdin makes restore.sh exit 1 without a word

**File:** `deploy/restore.sh:79`
**Issue:**
- `read -r -p ...` returns non-zero at EOF. Under `set -e` the script ends with status 1 and no "aborted" line, and `-p` prints no prompt when stdin is not a terminal.
- DEPLOY.md promises that anything but `yes` "prints `aborted` and exits 1".
- Run from cron, a pipe, or `</dev/null`, the operator sees nothing.

**Fix:** `read -r -p "..." confirm || confirm=""`, so the `aborted` branch runs.

### IN-03: RETENTION_DAYS and MIN_FREE_MB are not validated until after the backup is done

**File:** `deploy/backup.sh:59, 98`
**Issue:**
- A non-numeric `MIN_FREE_MB` makes `[ ... -lt ... ]` fail with "integer expression expected" and ends the script via `set -e`. An empty `free_mb` (from a failing `df`) does the same, with no message naming the cause.
- A non-numeric `RETENTION_DAYS` fails at line 98, after the database was snapshotted and the MANIFEST written. The backup is complete but the unit reports failure and the off-box copy at line 108 is skipped.

**Fix:** Validate both at the top with `case "$x" in ''|*[!0-9]*) echo "ERROR: ..." >&2; exit 2;; esac`.

### IN-04: The restore gate is `quick_check`; the documentation says "integrity check"

**File:** `deploy/restore.sh:74` (`cmdCheck` in `cmd/holzcloud/cli.go`, `db.QuickCheck`); `deploy/DEPLOY.md` Backups and Restore sections
**Issue:**
- Both `backup.sh` and `restore.sh` use `PRAGMA quick_check` through the binary. DEPLOY.md calls it "an integrity check".
- `BACKUP-RUN.md` and `README.md` already admit that a flipped byte in a data page passes both readers.
- The harness proves damage is caught only by truncation and a 4 KB overwrite (case a). The MANIFEST holds no checksum, so media corruption is not detected at all.

**Fix:** Say "quick check" in DEPLOY.md, or run `integrity_check` in restore (where time is cheap). Consider writing a `sha256sum` list into the MANIFEST and verifying it in `restore.sh`.

### IN-05: tree_selftest has a 1-in-65536 spurious failure

**File:** `deploy/test/run.sh:2672`
**Issue:** The "changed hash" case overwrites the first four characters of the last line's hash with `0000`. If that hash already starts with `0000`, nothing changes. The following check then finds no `differs` line and the script `die`s with "the comparison cannot fail". It is rare but not impossible.

**Fix:** Flip the first character instead, for example `substr($0,1,1) == "0" ? "1" : "0"`.

### IN-06: DEPLOY.md quotes a restore time that the evidence record contradicts

**File:** `deploy/DEPLOY.md:962-966` against `deploy/BACKUP-RUN.md:165-172`, `deploy/test/README.md:263`
**Issue:**
- DEPLOY.md says a restore took "about seven seconds". The final set measures `RESTORE-WALL-S` 8.126 and 8.296 seconds; seven is only the start-to-ready part (7.397 and 7.379).
- `README.md` and `BACKUP-RUN.md` both say "about eight".
- Phase 22's own rule is that a figure wrong by more than a factor of two is a fix. This one is within it, but the three documents should still agree.
- "backed up in under a second" is true for the 20 MB point (0.692) and not for the 320 MB point (1.015).

**Fix:** Change DEPLOY.md to "about eight seconds, of which about seven are the service starting".

---

_Reviewed: 2026-10-14_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
