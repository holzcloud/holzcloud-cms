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
if [ "$K" -eq 0 ]; then
    echo "BACKUP-ROBUST OK ($N cases)"
    exit 0
fi
echo "BACKUP-ROBUST FAILED ($K of $N cases)"
exit 1
