#!/usr/bin/env bash
set -u

# backup-robust.sh: the hermetic proof of what deploy/backup.sh does when a
# night goes wrong (Phase 29, ROB-01 and ROB-02).
#
#   bash deploy/test/backup-robust.sh
#
# It needs neither Docker nor root. It builds a temporary directory with a data
# directory, a fake rsync, a fake holzcloud binary and a fake date, runs the
# real deploy/backup.sh against them, and asserts on the exit status, on the
# files left behind and on what the script said on stderr. Nothing is installed
# anywhere and the temporary directory is removed when the script ends.
#
# Written for bash 3.2 (macOS /bin/bash) and for the bash on PATH: no arrays, no
# mapfile, no associative arrays, no ${var,,}, no [[ -v ]]. The script under
# test runs under the same interpreter as this file ($BASH), so
# `/bin/bash deploy/test/backup-robust.sh` plays it all under 3.2.
#
# The last line is `BACKUP-ROBUST OK (N cases)` (exit 0) or
# `BACKUP-ROBUST FAILED (K of N cases)` (exit 1). N counts assertions.

REPO=$(cd "$(dirname "$0")/../.." && pwd)
T=$(mktemp -d "${TMPDIR:-/tmp}/backup-robust.XXXXXX") || exit 2
trap 'rm -rf "$T"' EXIT

N=0
K=0

bt_eq() {
    # bt_eq NAME EXPECTED ACTUAL
    N=$((N + 1))
    if [ "$2" = "$3" ]; then
        echo "PASS $1"
    else
        K=$((K + 1))
        echo "FAIL $1"
        echo "  expected: $2"
        echo "  actual:   $3"
    fi
}

cnt() {
    # cnt FILE STRING: the number of lines of FILE that contain STRING
    grep -cF -- "$2" "$1" || true
}

bt_has() {
    # bt_has NAME FILE STRING
    bt_eq "$1" "1" "$(cnt "$2" "$3" | awk '{ print ($1 > 0) ? 1 : 0 }')"
}

bt_not() {
    # bt_not NAME FILE STRING
    bt_eq "$1" "0" "$(cnt "$2" "$3")"
}

exists() {
    # exists PATH: yes or no
    if [ -e "$1" ]; then echo yes; else echo no; fi
}

sha() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d' ' -f1
    else
        shasum -a 256 "$1" | cut -d' ' -f1
    fi
}

section() {
    echo "== $1"
}

# ---------------------------------------------------------------------------
# The world: a data directory and the fakes.

STAMP=20260101-030000
EARLIER=20251231-030000
NIGHT="$T/root/$STAMP"
EARLIER_DIR="$T/root/$EARLIER"

mkdir -p "$T/data/media" "$T/data/templates" "$T/bin" "$T/root" "$T/fake-mv" "$T/fake-find"
echo "picture" > "$T/data/media/a.jpg"
echo "template" > "$T/data/templates/t.html"
echo "key" > "$T/data/csrf.key"

# The fake rsync. Each call appends its arguments to FAKE_RSYNC_LOG and takes
# the next status from FAKE_RSYNC_PLAN (one per line; success when the plan is
# empty). Success copies the source into the destination; a failure first
# leaves one file in the destination (a half-written copy) and says what rsync
# says. With FAKE_RSYNC_PIDFILE set it writes its pid there and sleeps.
cat > "$T/bin/rsync" <<'FAKE'
#!/bin/bash
echo "$*" >> "$FAKE_RSYNC_LOG"
src=""
dst=""
for a in "$@"; do
    src="$dst"
    dst="$a"
done
if [ -n "${FAKE_RSYNC_PIDFILE:-}" ]; then
    mkdir -p "$dst"
    echo $$ > "$FAKE_RSYNC_PIDFILE"
    exec sleep 20
fi
rc=0
if [ -n "${FAKE_RSYNC_PLAN:-}" ] && [ -s "$FAKE_RSYNC_PLAN" ]; then
    rc=$(head -n 1 "$FAKE_RSYNC_PLAN")
    tail -n +2 "$FAKE_RSYNC_PLAN" > "$FAKE_RSYNC_PLAN.next"
    /bin/mv "$FAKE_RSYNC_PLAN.next" "$FAKE_RSYNC_PLAN"
fi
mkdir -p "$dst"
if [ "$rc" -eq 0 ]; then
    cp -R "$src". "$dst"
    exit 0
fi
echo "half" > "$dst/half-written.partial"
if [ "$rc" -eq 24 ]; then
    echo "rsync warning: some files vanished before they could be transferred (code 24) at main.c(1338) [sender=3.2.7]" >&2
else
    echo "rsync error: some error (code $rc) at main.c(1338) [sender=3.2.7]" >&2
fi
exit "$rc"
FAKE
chmod +x "$T/bin/rsync"

# The fake binary: `backup FILE` writes a small file, `version` prints a line.
# With FAKE_HC_FAIL=1 `backup` leaves a file and ends 1.
cat > "$T/bin/holzcloud" <<'FAKE'
#!/bin/bash
case "${1:-}" in
    backup)
        echo "database" > "$2"
        if [ "${FAKE_HC_FAIL:-0}" = "1" ]; then
            exit 1
        fi
        ;;
    version)
        echo "holzcloud vTEST"
        ;;
esac
exit 0
FAKE
chmod +x "$T/bin/holzcloud"

# The fake date: a fixed name for the directory, the real date otherwise.
cat > "$T/bin/date" <<'FAKE'
#!/bin/bash
if [ "${1:-}" = "+%Y%m%d-%H%M%S" ]; then
    echo "20260101-030000"
    exit 0
fi
exec /bin/date "$@"
FAKE
chmod +x "$T/bin/date"

# A fake mv that refuses the rename of the MANIFEST (Test 13), and a fake find
# that fails (Test 16). They sit in their own directories and are put in front
# of PATH only for those runs.
cat > "$T/fake-mv/mv" <<'FAKE'
#!/bin/bash
echo "$*" >> "$FAKE_MV_LOG"
case "${1:-}" in
    *MANIFEST.partial) exit 1 ;;
esac
exec /bin/mv "$@"
FAKE
chmod +x "$T/fake-mv/mv"

cat > "$T/fake-find/find" <<'FAKE'
#!/bin/bash
exit 1
FAKE
chmod +x "$T/fake-find/find"

# ---------------------------------------------------------------------------
# Running the script under test.

RC=0
EARLIER_SHA=""

write_plan() {
    : > "$T/plan"
    for s in "$@"; do
        echo "$s" >> "$T/plan"
    done
}

reset_case() {
    # A clean backup root holding one earlier night, an empty rsync log and plan.
    rm -rf "$T/root" "$T/remote"
    mkdir -p "$EARLIER_DIR"
    echo "earlier database" > "$EARLIER_DIR/holzcloud.sqlite"
    printf 'timestamp=%s\nversion=old\ndata_dir=/x\nhost=h\n' "$EARLIER" > "$EARLIER_DIR/MANIFEST"
    EARLIER_SHA=$(sha "$EARLIER_DIR/holzcloud.sqlite")
    : > "$T/rsync.log"
    : > "$T/mv.log"
    : > "$T/plan"
    rm -f "$T/rsync.pid"
}

run_backup() {
    # run_backup [VAR=value ...]: stdout in $T/out, stderr in $T/err, status in RC.
    # PATH_PREFIX, when set, is put in front of PATH.
    (
        cd "$REPO" || exit 99
        env PATH="${PATH_PREFIX:-}$T/bin:$PATH" \
            DATA_DIR="$T/data" HOLZCLOUD_BIN="$T/bin/holzcloud" \
            MIN_FREE_MB=1 RETENTION_DAYS=30 REMOTE_TARGET= \
            FAKE_RSYNC_LOG="$T/rsync.log" FAKE_RSYNC_PLAN="$T/plan" \
            FAKE_MV_LOG="$T/mv.log" \
            "$@" "$BASH" "$REPO/deploy/backup.sh" "$T/root"
    ) > "$T/out" 2> "$T/err"
    RC=$?
}

calls_to() {
    # calls_to SUFFIX: the number of fake rsync calls whose destination ends in SUFFIX
    awk -v s="$1" '{ n = length($NF); l = length(s); if (n >= l && substr($NF, n - l + 1) == s) c++ } END { print c + 0 }' "$T/rsync.log"
}

earlier_intact() {
    # yes when the earlier night still has its MANIFEST and its file is unchanged
    if [ -f "$EARLIER_DIR/MANIFEST" ] && [ "$(sha "$EARLIER_DIR/holzcloud.sqlite")" = "$EARLIER_SHA" ]; then
        echo yes
    else
        echo no
    fi
}

# ---------------------------------------------------------------------------
section "Test 1: the success path is unchanged"
reset_case
run_backup
bt_eq "1 exit status 0" 0 "$RC"
EXPECTED_OUT=$(printf '%s\n' \
    "=== holzcloud-CMS backup: $STAMP ===" \
    "Backing up database..." \
    "Backing up CSRF key..." \
    "Backing up media..." \
    "Backing up templates..." \
    "Pruning backups older than 30 days..." \
    "=== Backup complete: $NIGHT ===")
bt_eq "1 stdout lines (without the du line)" "$EXPECTED_OUT" "$(sed '$d' "$T/out")"
bt_eq "1 stderr is empty" "0" "$(wc -c < "$T/err" | tr -d ' ')"
bt_eq "1 MANIFEST keys in order" "timestamp,version,data_dir,host," "$(cut -d= -f1 "$NIGHT/MANIFEST" | tr '\n' ',')"
bt_eq "1 no MANIFEST.partial is left" no "$(exists "$NIGHT/MANIFEST.partial")"
bt_eq "1 the directory mode is 700" "drwx------" "$(ls -ld "$NIGHT" | cut -c1-10)"
bt_eq "1 the copy holds the media file" yes "$(exists "$NIGHT/media/a.jpg")"
bt_eq "1 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 2: rsync 11 removes the night's own directory and keeps its status"
reset_case
write_plan 11
run_backup
bt_eq "2 exit status is rsync's own, 11" 11 "$RC"
bt_eq "2 the directory of this run does not exist" no "$(exists "$NIGHT")"
bt_eq "2 the earlier night is intact" yes "$(earlier_intact)"
bt_eq "2 the backup root still has the earlier night" yes "$(exists "$EARLIER_DIR")"
bt_has "2 stderr names the removed directory" "$T/err" "ERROR: backup did not finish; removed $NIGHT, which had no MANIFEST"

# ---------------------------------------------------------------------------
REPEAT_MEDIA="WARNING: rsync exited 24 (files vanished) while copying media; repeating it once"
REPEAT_OFFSITE="WARNING: rsync exited 24 (files vanished) while copying the off-site copy; repeating it once"
CLEANUP_LINE="ERROR: backup did not finish; removed $NIGHT, which had no MANIFEST"

section "Test 3: 24 once in the media step, then success"
reset_case
write_plan 24
run_backup
bt_eq "3 exit status 0" 0 "$RC"
bt_eq "3 the repeat line is on stderr once" 1 "$(cnt "$T/err" "$REPEAT_MEDIA")"
bt_not "3 no recurrence line" "$T/err" "again while copying"
bt_eq "3 two calls for media/" 2 "$(calls_to /media/)"
bt_eq "3 one call for templates/" 1 "$(calls_to /templates/)"
bt_eq "3 the night has its MANIFEST" yes "$(exists "$NIGHT/MANIFEST")"
bt_not "3 no cleanup line" "$T/err" "backup did not finish"

# ---------------------------------------------------------------------------
section "Test 4: 24 twice in the media step ends the night with 24"
reset_case
write_plan 24 24
run_backup
bt_eq "4 exit status 24" 24 "$RC"
bt_eq "4 the repeat line is on stderr once" 1 "$(cnt "$T/err" "$REPEAT_MEDIA")"
bt_has "4 the recurrence line names media" "$T/err" "ERROR: rsync exited 24 again while copying media; giving up"
bt_eq "4 exactly two calls for media/" 2 "$(calls_to /media/)"
bt_eq "4 no call for templates/" 0 "$(calls_to /templates/)"
bt_eq "4 the directory is removed" no "$(exists "$NIGHT")"
bt_eq "4 the earlier night is intact" yes "$(earlier_intact)"
bt_has "4 the cleanup line is on stderr" "$T/err" "$CLEANUP_LINE"

# ---------------------------------------------------------------------------
section "Test 5: status 23 and status 11 are never repeated"
reset_case
write_plan 23
run_backup
bt_eq "5 exit status 23" 23 "$RC"
bt_eq "5 one call only (23)" 1 "$(wc -l < "$T/rsync.log" | tr -d ' ')"
bt_not "5 no repeat line (23)" "$T/err" "rsync exited 24"
bt_eq "5 the directory is removed (23)" no "$(exists "$NIGHT")"
reset_case
write_plan 11
run_backup
bt_eq "5 exit status 11" 11 "$RC"
bt_eq "5 one call only (11)" 1 "$(wc -l < "$T/rsync.log" | tr -d ' ')"
bt_not "5 no repeat line (11)" "$T/err" "rsync exited 24"
bt_eq "5 the directory is removed (11)" no "$(exists "$NIGHT")"

# ---------------------------------------------------------------------------
section "Test 6: 24 twice in the second directory of the loop"
reset_case
write_plan 0 24 24
run_backup
bt_eq "6 exit status 24" 24 "$RC"
bt_eq "6 media/ was copied once" 1 "$(calls_to /media/)"
bt_eq "6 templates/ was tried twice" 2 "$(calls_to /templates/)"
bt_has "6 the recurrence line names templates" "$T/err" "ERROR: rsync exited 24 again while copying templates; giving up"
bt_eq "6 the directory, which held media/, is removed" no "$(exists "$NIGHT")"
bt_eq "6 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 7: the off-site copy, 24 once"
reset_case
write_plan 0 0 24
run_backup REMOTE_TARGET="$T/remote"
bt_eq "7 exit status 0" 0 "$RC"
bt_eq "7 the repeat line names the off-site copy" 1 "$(cnt "$T/err" "$REPEAT_OFFSITE")"
bt_eq "7 two off-site calls" 2 "$(calls_to /remote/)"
bt_eq "7 the night keeps its MANIFEST" yes "$(exists "$NIGHT/MANIFEST")"
bt_not "7 no cleanup line" "$T/err" "backup did not finish"

# ---------------------------------------------------------------------------
section "Test 8: the off-site copy, 24 twice"
reset_case
write_plan 0 0 24 24
run_backup REMOTE_TARGET="$T/remote"
bt_eq "8 exit status 24" 24 "$RC"
bt_has "8 the recurrence line names the off-site copy" "$T/err" "ERROR: rsync exited 24 again while copying the off-site copy; giving up"
bt_eq "8 two off-site calls" 2 "$(calls_to /remote/)"
bt_eq "8 the night keeps its MANIFEST" yes "$(exists "$NIGHT/MANIFEST")"
bt_not "8 no cleanup line, the backup itself is whole" "$T/err" "backup did not finish"

# ---------------------------------------------------------------------------
section "Test 9: the off-site copy, status 23"
reset_case
write_plan 0 0 23
run_backup REMOTE_TARGET="$T/remote"
bt_eq "9 exit status 23" 23 "$RC"
bt_eq "9 one off-site call" 1 "$(calls_to /remote/)"
bt_not "9 no repeat line" "$T/err" "rsync exited 24"
bt_eq "9 the night keeps its MANIFEST" yes "$(exists "$NIGHT/MANIFEST")"
bt_not "9 no cleanup line" "$T/err" "backup did not finish"

# ---------------------------------------------------------------------------
section "Test 10: the free-space guard refuses, nothing is made, nothing is removed"
reset_case
run_backup MIN_FREE_MB=999999999999
bt_eq "10 exit status 1" 1 "$RC"
bt_has "10 stderr holds the guard's message" "$T/err" "ERROR: only "
bt_eq "10 no new directory in the root" no "$(exists "$NIGHT")"
bt_not "10 no cleanup line" "$T/err" "backup did not finish"
bt_eq "10 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 11: a directory under the name of this run makes the script refuse"
reset_case
mkdir -p "$NIGHT"
echo "mine" > "$NIGHT/keep.txt"
KEEP_SHA=$(sha "$NIGHT/keep.txt")
run_backup
bt_eq "11 exit status 1" 1 "$RC"
bt_has "11 the refusal line is on stderr" "$T/err" "ERROR: $NIGHT exists already; refusing to start so that no earlier backup is touched"
bt_eq "11 the directory still exists" yes "$(exists "$NIGHT")"
bt_eq "11 its file is unchanged" "$KEEP_SHA" "$(sha "$NIGHT/keep.txt")"
bt_eq "11 nothing was added to it" 1 "$(ls -A "$NIGHT" | wc -l | tr -d ' ')"
bt_not "11 no cleanup line" "$T/err" "backup did not finish"
bt_eq "11 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 12: step 1 fails"
reset_case
run_backup FAKE_HC_FAIL=1
bt_eq "12 exit status 1" 1 "$RC"
bt_eq "12 the directory is removed" no "$(exists "$NIGHT")"
bt_has "12 the cleanup line is printed" "$T/err" "$CLEANUP_LINE"
bt_eq "12 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 13: the rename of the MANIFEST fails"
reset_case
PATH_PREFIX="$T/fake-mv:" run_backup
bt_eq "13 exit status 1" 1 "$RC"
bt_has "13 the fake mv was asked for MANIFEST.partial" "$T/mv.log" "MANIFEST.partial"
bt_eq "13 the directory is removed although MANIFEST.partial was in it" no "$(exists "$NIGHT")"
bt_has "13 the cleanup line is printed" "$T/err" "$CLEANUP_LINE"
bt_eq "13 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 14: SIGTERM during a copy"
# What both bash 3.2 and bash 5 guarantee, and all that is asserted: the
# directory is gone and the status is above 128. The exact number (143 from the
# script's own signal, or the fake rsync's 143 passed on) depends on which of
# the two is seen first and is not asserted.
reset_case
(
    cd "$REPO" || exit 99
    exec env PATH="$T/bin:$PATH" DATA_DIR="$T/data" HOLZCLOUD_BIN="$T/bin/holzcloud" \
        MIN_FREE_MB=1 RETENTION_DAYS=30 REMOTE_TARGET= \
        FAKE_RSYNC_LOG="$T/rsync.log" FAKE_RSYNC_PLAN="$T/plan" \
        FAKE_RSYNC_PIDFILE="$T/rsync.pid" \
        "$BASH" "$REPO/deploy/backup.sh" "$T/root"
) > "$T/out" 2> "$T/err" &
SPID=$!
i=0
while [ ! -s "$T/rsync.pid" ] && [ "$i" -lt 100 ]; do
    sleep 0.1
    i=$((i + 1))
done
RPID=$(cat "$T/rsync.pid" 2>/dev/null || echo 0)
bt_eq "14 the fake rsync started" yes "$(if [ "$RPID" -gt 0 ]; then echo yes; else echo no; fi)"
# bash 3.2 reports a job killed by a signal on its stderr; this one was.
{
    if [ "$RPID" -gt 0 ]; then
        kill -TERM "$SPID" "$RPID" 2>/dev/null
    else
        kill -TERM "$SPID" 2>/dev/null
    fi
    i=0
    while kill -0 "$SPID" 2>/dev/null && [ "$i" -lt 100 ]; do
        sleep 0.1
        i=$((i + 1))
    done
    ENDED=yes
    if kill -0 "$SPID" 2>/dev/null; then
        ENDED=no
        kill -KILL "$SPID" "$RPID" 2>/dev/null
    fi
    { wait "$SPID"; RC=$?; } 2>/dev/null
} 2>/dev/null
bt_eq "14 the script ended within 10 seconds" yes "$ENDED"
bt_eq "14 the status is above 128" yes "$(if [ "$RC" -gt 128 ]; then echo yes; else echo no; fi)"
bt_eq "14 the directory is gone" no "$(exists "$NIGHT")"
bt_eq "14 the earlier night is intact" yes "$(earlier_intact)"

# ---------------------------------------------------------------------------
section "Test 15: the script text the harness depends on"
bt_eq "15 MIN_FREE_MB default read by sed" 1024 "$(sed -n 's/^MIN_FREE_MB="${MIN_FREE_MB:-\([0-9]*\)}"$/\1/p' "$REPO/deploy/backup.sh")"
for lit in \
    '=== holzcloud-CMS backup: ' \
    'Backing up database...' \
    'Backing up CSRF key...' \
    'Backing up ${name}...' \
    'Pruning backups older than' \
    'Copying to ' \
    '=== Backup complete: '; do
    bt_has "15 backup.sh holds: $lit" "$REPO/deploy/backup.sh" "$lit"
done

# ---------------------------------------------------------------------------
section "Test 16: retention cannot fail the night"
reset_case
PATH_PREFIX="$T/fake-find:" run_backup
bt_eq "16 exit status 0 although find fails" 0 "$RC"
bt_eq "16 the night has its MANIFEST" yes "$(exists "$NIGHT/MANIFEST")"
bt_has "16 the complete line is printed" "$T/out" "=== Backup complete: $NIGHT ==="

# ---------------------------------------------------------------------------
if [ "$K" -eq 0 ]; then
    echo "BACKUP-ROBUST OK ($N cases)"
    exit 0
fi
echo "BACKUP-ROBUST FAILED ($K of $N cases)"
exit 1
