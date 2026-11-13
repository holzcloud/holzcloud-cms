#!/usr/bin/env bash
set -u

# restore-robust.sh: the hermetic proof of what deploy/restore.sh does with a
# backup it is handed (Phase 32, BAK-01 and BAK-02).
#
#   bash deploy/test/restore-robust.sh
#
# It needs neither Docker nor root. It builds a temporary world with fakes for
# holzcloud, systemctl, curl, chown and rsync, makes the backup under test by
# running the real deploy/backup.sh of this tree (so the format under test is
# the one backup.sh writes), and then runs the real deploy/restore.sh against a
# copy of it, answering `yes`. It asserts on the exit status, on what the script
# said, on whether the service was stopped and on the data directory. Nothing is
# installed anywhere and the temporary directory is removed when the script
# ends.
#
# Written for bash 3.2 (macOS /bin/bash) and for the bash on PATH: no arrays, no
# mapfile, no associative arrays, no ${var,,}, no [[ -v ]]. The scripts under
# test run under the same interpreter as this file ($BASH).
#
# The last line is `RESTORE-ROBUST OK (N cases)` (exit 0) or
# `RESTORE-ROBUST FAILED (K of N cases)` (exit 1). N counts assertions.

REPO=$(cd "$(dirname "$0")/../.." && pwd)
T=$(mktemp -d "${TMPDIR:-/tmp}/restore-robust.XXXXXX") || exit 2
trap 'chmod -R u+w "$T" 2>/dev/null; rm -rf "$T"' EXIT

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

line_of() {
    # line_of FILE STRING: the number of the first line of FILE holding STRING, 0 if none
    awk -v s="$2" 'index($0, s) { print NR; found = 1; exit } END { if (!found) print 0 }' "$1"
}

# ---------------------------------------------------------------------------
# The world: a source data directory, the fakes, and the backup they make.

STAMP=20260101-030000
mkdir -p "$T/src/media" "$T/src/templates" "$T/bin" "$T/count" "$T/backups"
echo "picture" > "$T/src/media/a.jpg"
echo "second picture" > "$T/src/media/b.jpg"
# A name with a backslash. GNU sha256sum writes its line escaped (a leading
# backslash, the name with the backslash doubled), Darwin's writes it as it is;
# either way it is listed and verified like any other file.
echo "odd picture" > "$T/src/media/odd\\name.jpg"
echo "template" > "$T/src/templates/t.html"
echo "key" > "$T/src/csrf.key"

# The real sha256sum the counting wrapper hands over to; a shim to
# `shasum -a 256` (same "<hex>  ./path" lines) where the system has none.
REAL_SHA=$(command -v sha256sum 2>/dev/null || true)
if [ -z "$REAL_SHA" ]; then
    printf '#!/bin/bash\nexec shasum -a 256 "$@"\n' > "$T/bin/sha256sum"
    chmod +x "$T/bin/sha256sum"
    REAL_SHA="$T/bin/sha256sum"
fi

# The counting sha256sum: one line per call in FAKE_SHA_LOG, then the real one.
# It sits in its own directory and is put in front of PATH only where a case
# asks whether anything was hashed.
cat > "$T/count/sha256sum" <<FAKE
#!/bin/bash
echo "\$*" >> "\$FAKE_SHA_LOG"
exec "$REAL_SHA" "\$@"
FAKE
chmod +x "$T/count/sha256sum"

# A sha256sum that cannot read one file (a failing disk: EIO, which root meets
# as well) and hashes everything else, in its own directory.
mkdir -p "$T/eio"
cat > "$T/eio/sha256sum" <<FAKE
#!/bin/bash
rc=0
for a in "\$@"; do
    case "\$a" in
        */b.jpg) echo "sha256sum: \$a: Input/output error" >&2; rc=1 ;;
        *) "$REAL_SHA" "\$a" || rc=\$? ;;
    esac
done
exit \$rc
FAKE
chmod +x "$T/eio/sha256sum"

# The fake rsync: copies the source into the destination (the last two
# arguments), for backup.sh and for restore.sh alike.
cat > "$T/bin/rsync" <<'FAKE'
#!/bin/bash
src=""
dst=""
for a in "$@"; do
    src="$dst"
    dst="$a"
done
mkdir -p "$dst"
cp -R "$src". "$dst"
FAKE
chmod +x "$T/bin/rsync"

# The fake binary. `backup FILE` writes a small file, `version` prints a line.
# `check` records its arguments in FAKE_HC_LOG and answers by FAKE_CHECK_MODE:
#   new      integrity ok, the files marker with nothing missing, exit 0
#   missing  integrity ok, two rows without a file (one a variant), exit 3
#   old      integrity ok only (an older binary ignores -files), exit 0
#   corrupt  an integrity failure, exit 1
#   newer    the newer-schema refusal of plan 32-01 on stderr, exit 1
cat > "$T/bin/holzcloud" <<'FAKE'
#!/bin/bash
case "${1:-}" in
    backup)
        echo "database" > "$2"
        ;;
    version)
        echo "holzcloud vTEST"
        ;;
    check)
        echo "check $*" >> "${FAKE_HC_LOG:-/dev/null}"
        case "${FAKE_CHECK_MODE:-new}" in
            new)
                echo "integrity: ok"
                echo "files: 3 rows checked, 0 missing"
                exit 0
                ;;
            missing)
                echo "integrity: ok"
                echo "files: 5 rows checked, 2 missing"
                echo 'MISSING media/1/gone.jpg (media row 4, website 1, "Sommerfest.jpg")'
                echo 'MISSING media/1/gone-thumb.webp (media_variants row 9, label thumb, of media row 4)'
                echo "a missing variant can be rebuilt with: holzcloud thumbnails"
                exit 3
                ;;
            old)
                echo "integrity: ok"
                exit 0
                ;;
            corrupt)
                echo "integrity: *** in database main ***"
                exit 1
                ;;
            newer)
                echo "error: the database is at migration level 62, this binary knows migrations up to 61" >&2
                exit 1
                ;;
        esac
        ;;
esac
exit 0
FAKE
chmod +x "$T/bin/holzcloud"

# The fake date: a fixed name for the backup directory, the real date otherwise.
cat > "$T/bin/date" <<'FAKE'
#!/bin/bash
if [ "${1:-}" = "+%Y%m%d-%H%M%S" ]; then
    echo "20260101-030000"
    exit 0
fi
exec /bin/date "$@"
FAKE
chmod +x "$T/bin/date"

# systemctl and chown record their arguments; curl answers /readyz.
cat > "$T/bin/systemctl" <<'FAKE'
#!/bin/bash
echo "$*" >> "$FAKE_SYSTEMCTL_LOG"
exit 0
FAKE
chmod +x "$T/bin/systemctl"
cat > "$T/bin/chown" <<'FAKE'
#!/bin/bash
if [ "${FAKE_CHOWN_FAIL:-0}" = "1" ]; then
    echo "chown: changing ownership: Operation not permitted" >&2
    exit 1
fi
exit 0
FAKE
chmod +x "$T/bin/chown"
# mountpoint answers yes for exactly the path in FAKE_MOUNT (a data directory
# that is a mounted disk), no for everything else.
cat > "$T/bin/mountpoint" <<'FAKE'
#!/bin/bash
last=""
for a in "$@"; do
    last="$a"
done
[ -n "${FAKE_MOUNT:-}" ] && [ "$last" = "$FAKE_MOUNT" ]
FAKE
chmod +x "$T/bin/mountpoint"
cat > "$T/bin/curl" <<'FAKE'
#!/bin/bash
echo '{"status":"ready"}'
exit 0
FAKE
chmod +x "$T/bin/curl"

# The fixture: one night of the real backup.sh.
(
    cd "$REPO" || exit 99
    env PATH="$T/bin:$PATH" DATA_DIR="$T/src" HOLZCLOUD_BIN="$T/bin/holzcloud" \
        MIN_FREE_MB=1 RETENTION_DAYS=0 REMOTE_TARGET= \
        "$BASH" "$REPO/deploy/backup.sh" "$T/backups"
) > "$T/fixture.out" 2>&1
FIXTURE="$T/backups/$STAMP"
if [ ! -f "$FIXTURE/MANIFEST" ]; then
    echo "the fixture backup was not made:"
    cat "$T/fixture.out"
    echo "RESTORE-ROBUST FAILED (1 of 1 cases)"
    exit 1
fi

# ---------------------------------------------------------------------------
# Running the script under test.

B="$T/case/$STAMP"
LIVE="$T/live"
RC=0
SENTINEL_SHA=""

reset_case() {
    # A fresh copy of the fixture in $B, a live data directory holding a
    # sentinel, empty logs.
    chmod -R u+w "$T/case" 2>/dev/null
    rm -rf "$T/case" "$LIVE" "$LIVE".before-restore-*
    mkdir -p "$T/case"
    cp -R "$FIXTURE" "$B"
    mkdir -p "$LIVE"
    echo "the live site" > "$LIVE/sentinel.txt"
    SENTINEL_SHA=$(sha "$LIVE/sentinel.txt")
    : > "$T/systemctl.log"
    : > "$T/hc.log"
    : > "$T/sha.log"
}

run_restore() {
    # run_restore [VAR=value ...]: stdout in $T/out, stderr in $T/err, status in
    # RC. PATH_PREFIX, when set, is put in front of PATH.
    (
        cd "$REPO" || exit 99
        echo yes | env PATH="${PATH_PREFIX:-}$T/bin:$PATH" \
            DATA_DIR="$LIVE" HOLZCLOUD_BIN="$T/bin/holzcloud" OWNER=nobody:nobody \
            SERVICE=holzcloud \
            FAKE_SYSTEMCTL_LOG="$T/systemctl.log" FAKE_HC_LOG="$T/hc.log" \
            FAKE_SHA_LOG="$T/sha.log" \
            "$@" "$BASH" "$REPO/deploy/restore.sh" "$B"
    ) > "$T/out" 2> "$T/err"
    RC=$?
}

live_untouched() {
    # yes when the live directory is where it was, with its sentinel unchanged
    if [ -f "$LIVE/sentinel.txt" ] && [ "$(sha "$LIVE/sentinel.txt")" = "$SENTINEL_SHA" ]; then
        echo yes
    else
        echo no
    fi
}

restored() {
    # yes when the live directory holds the backup's database and media
    if [ -f "$LIVE/holzcloud.sqlite" ] && [ -f "$LIVE/media/a.jpg" ] && [ ! -e "$LIVE/sentinel.txt" ]; then
        echo yes
    else
        echo no
    fi
}

flip() {
    # flip FILE: change the content of FILE, keep its size
    if [ "$(head -c 1 "$1")" = "X" ]; then
        printf 'Y' | dd of="$1" bs=1 count=1 conv=notrunc 2>/dev/null
    else
        printf 'X' | dd of="$1" bs=1 count=1 conv=notrunc 2>/dev/null
    fi
}

DAMAGED="the backup is damaged, do not restore it"

# ---------------------------------------------------------------------------
section "Case verified: an untouched backup restores"
reset_case
PATH_PREFIX="$T/count:" run_restore
SUMS_LINES=$(wc -l < "$B/SHA256SUMS" | tr -d ' ')
bt_eq "verified: exit status 0" 0 "$RC"
bt_has "verified: the verifying line" "$T/out" "Verifying checksums..."
bt_has "verified: the count line names every line of SHA256SUMS" "$T/out" "Checksums: $SUMS_LINES files verified"
bt_eq "verified: the counting sha256sum was used" yes "$(if [ -s "$T/sha.log" ]; then echo yes; else echo no; fi)"
bt_eq "verified: the service was stopped" 1 "$(grep -cx 'stop holzcloud' "$T/systemctl.log")"
bt_eq "verified: the service was started" 1 "$(grep -cx 'start holzcloud' "$T/systemctl.log")"
bt_eq "verified: the data directory is the backup's" yes "$(restored)"
bt_not "verified: no NOTICE" "$T/out" "NOTICE"

# ---------------------------------------------------------------------------
section "Case media-flip: one changed byte in a media file is refused by name"
reset_case
flip "$B/media/a.jpg"
run_restore
bt_eq "media-flip: exit status 1" 1 "$RC"
bt_has "media-flip: the file is named" "$T/err" "ERROR: ./media/a.jpg differs from its checksum in SHA256SUMS — $DAMAGED"
bt_eq "media-flip: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "media-flip: the live data is untouched" yes "$(live_untouched)"
bt_eq "media-flip: the binary was not asked" 0 "$(wc -l < "$T/hc.log" | tr -d ' ')"

# ---------------------------------------------------------------------------
section "Case first-of-two: of two changed files the first in SHA256SUMS is named"
reset_case
flip "$B/templates/t.html"
flip "$B/csrf.key"
run_restore
bt_eq "first-of-two: exit status 1" 1 "$RC"
bt_has "first-of-two: csrf.key, first in SHA256SUMS, is named" "$T/err" "ERROR: ./csrf.key differs from its checksum in SHA256SUMS — $DAMAGED"
bt_not "first-of-two: the second file is not named" "$T/err" "./templates/t.html"
bt_eq "first-of-two: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case pre-015: a backup made before 0.1.5 restores with a notice"
reset_case
grep -vx 'checksums=SHA256SUMS' "$B/MANIFEST" > "$T/manifest.old"
cp "$T/manifest.old" "$B/MANIFEST"
rm -f "$B/SHA256SUMS"
run_restore
bt_eq "pre-015: exit status 0" 0 "$RC"
bt_has "pre-015: the notice" "$T/out" "NOTICE: $B has no checksum file (made before 0.1.5); its files were not verified"
bt_not "pre-015: nothing claims to be verified" "$T/out" "files verified"
bt_eq "pre-015: the data directory is the backup's" yes "$(restored)"

# ---------------------------------------------------------------------------
section "Case manifest-names-missing-file: the MANIFEST names SHA256SUMS, the file is gone"
reset_case
rm -f "$B/SHA256SUMS"
run_restore
bt_eq "manifest-names-missing-file: exit status 1" 1 "$RC"
bt_has "manifest-names-missing-file: the message" "$T/err" "ERROR: $B/MANIFEST names SHA256SUMS but the file is missing — do not restore it"
bt_eq "manifest-names-missing-file: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "manifest-names-missing-file: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
HEX=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

section "Case dotdot: a line naming ../ is refused before anything is hashed"
reset_case
echo "$HEX  ./media/../../etc/hosts" >> "$B/SHA256SUMS"
BADNO=$(wc -l < "$B/SHA256SUMS" | tr -d ' ')
PATH_PREFIX="$T/count:" run_restore
bt_eq "dotdot: exit status 1" 1 "$RC"
bt_has "dotdot: the line is named" "$T/err" "ERROR: line $BADNO of SHA256SUMS is not in the format backup.sh writes or names a path outside the backup — do not restore it"
bt_eq "dotdot: nothing was hashed" 0 "$(wc -l < "$T/sha.log" | tr -d ' ')"
bt_eq "dotdot: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "dotdot: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case not-dot-slash: a line naming an absolute path is refused the same way"
reset_case
# The bad line goes first, so the number is 1.
{ echo "$HEX  /etc/hosts"; cat "$B/SHA256SUMS"; } > "$T/sums.new"
cp "$T/sums.new" "$B/SHA256SUMS"
PATH_PREFIX="$T/count:" run_restore
bt_eq "not-dot-slash: exit status 1" 1 "$RC"
bt_has "not-dot-slash: the line is named" "$T/err" "ERROR: line 1 of SHA256SUMS is not in the format backup.sh writes or names a path outside the backup — do not restore it"
bt_eq "not-dot-slash: nothing was hashed" 0 "$(wc -l < "$T/sha.log" | tr -d ' ')"
bt_eq "not-dot-slash: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case escaped-line: a backslash line is verified like any other, a missing file refused"
reset_case
printf '\\%s  ./media/gone\\\\name.jpg\n' "$HEX" >> "$B/SHA256SUMS"
run_restore
bt_eq "escaped-line: exit status 1" 1 "$RC"
bt_has "escaped-line: the file is named" "$T/err" "ERROR: ./media/gone\\\\name.jpg is listed in SHA256SUMS and missing — $DAMAGED"
bt_not "escaped-line: no notice waves it through" "$T/out" "NOTICE"
bt_eq "escaped-line: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "escaped-line: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case escaped-rot: a first byte rotted into a backslash is a malformed line"
reset_case
{ printf '\\'; head -n 1 "$B/SHA256SUMS" | cut -c2-; tail -n +2 "$B/SHA256SUMS"; } > "$T/sums.new"
cp "$T/sums.new" "$B/SHA256SUMS"
PATH_PREFIX="$T/count:" run_restore
bt_eq "escaped-rot: exit status 1" 1 "$RC"
bt_has "escaped-rot: line 1 is named" "$T/err" "ERROR: line 1 of SHA256SUMS is not in the format backup.sh writes or names a path outside the backup — do not restore it"
bt_eq "escaped-rot: nothing was hashed" 0 "$(wc -l < "$T/sha.log" | tr -d ' ')"
bt_eq "escaped-rot: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case backslash-name: a real file with a backslash in its name is verified"
reset_case
flip "$B/media/odd\\name.jpg"
run_restore
bt_eq "backslash-name: exit status 1" 1 "$RC"
bt_has "backslash-name: the file is named as differing" "$T/err" "name.jpg differs from its checksum in SHA256SUMS — $DAMAGED"
bt_has "backslash-name: it is the odd file" "$T/err" "ERROR: ./media/odd"
bt_eq "backslash-name: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case empty-sums: a zero-length SHA256SUMS is refused, not 0 files verified"
reset_case
: > "$B/SHA256SUMS"
run_restore
bt_eq "empty-sums: exit status 1" 1 "$RC"
bt_has "empty-sums: the message" "$T/err" "ERROR: $B/SHA256SUMS is empty — $DAMAGED"
bt_not "empty-sums: nothing claims to be verified" "$T/out" "files verified"
bt_eq "empty-sums: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "empty-sums: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case no-db-line: SHA256SUMS without ./holzcloud.sqlite is refused"
reset_case
grep -v '  \./holzcloud\.sqlite$' "$B/SHA256SUMS" > "$T/sums.new"
cp "$T/sums.new" "$B/SHA256SUMS"
run_restore
bt_eq "no-db-line: exit status 1" 1 "$RC"
bt_has "no-db-line: the message" "$T/err" "ERROR: SHA256SUMS does not list ./holzcloud.sqlite — $DAMAGED"
bt_not "no-db-line: nothing claims to be verified" "$T/out" "files verified"
bt_eq "no-db-line: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case truncated: a SHA256SUMS cut at a line boundary names the unlisted file"
reset_case
sed '$d' "$B/SHA256SUMS" > "$T/sums.new"
cp "$T/sums.new" "$B/SHA256SUMS"
run_restore
bt_eq "truncated: exit status 1" 1 "$RC"
bt_has "truncated: the dropped file is named" "$T/err" "ERROR: ./templates/t.html is in the backup but not listed in SHA256SUMS — $DAMAGED"
bt_not "truncated: nothing claims to be verified" "$T/out" "files verified"
bt_eq "truncated: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "truncated: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case added-file: a file added after the night is refused by name"
reset_case
echo "added later" > "$B/media/added.jpg"
run_restore
bt_eq "added-file: exit status 1" 1 "$RC"
bt_has "added-file: the file is named" "$T/err" "ERROR: ./media/added.jpg is in the backup but not listed in SHA256SUMS — $DAMAGED"
bt_eq "added-file: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case twice: a line listed twice is refused"
reset_case
head -n 1 "$B/SHA256SUMS" >> "$B/SHA256SUMS"
run_restore
bt_eq "twice: exit status 1" 1 "$RC"
bt_has "twice: the file is named" "$T/err" "ERROR: ./csrf.key is listed twice in SHA256SUMS — $DAMAGED"
bt_eq "twice: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case manifest-lost-line: a SHA256SUMS beside a MANIFEST without the line is still verified"
reset_case
grep -vx 'checksums=SHA256SUMS' "$B/MANIFEST" > "$T/manifest.old"
cp "$T/manifest.old" "$B/MANIFEST"
flip "$B/media/a.jpg"
run_restore
bt_eq "manifest-lost-line: exit status 1" 1 "$RC"
bt_has "manifest-lost-line: the file is named" "$T/err" "ERROR: ./media/a.jpg differs from its checksum in SHA256SUMS — $DAMAGED"
bt_not "manifest-lost-line: no notice of an old backup" "$T/out" "made before 0.1.5"
bt_eq "manifest-lost-line: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
ODD="is a symbolic link or not a regular file; backup.sh makes neither — do not restore it"

section "Case link-dir: a directory replaced by a link out of the backup is refused unread"
reset_case
mkdir -p "$T/outside/1"
echo "outside" > "$T/outside/1/f"
mkdir -p "$B/media"
ln -s "$T/outside/1" "$B/media/1"
echo "$(sha "$T/outside/1/f")  ./media/1/f" >> "$B/SHA256SUMS"
PATH_PREFIX="$T/count:" run_restore
bt_eq "link-dir: exit status 1" 1 "$RC"
bt_has "link-dir: the link is named" "$T/err" "ERROR: ./media/1 in $B $ODD"
bt_not "link-dir: nothing claims to be verified" "$T/out" "files verified"
bt_eq "link-dir: nothing was hashed" 0 "$(wc -l < "$T/sha.log" | tr -d ' ')"
bt_eq "link-dir: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "link-dir: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case link-file: a link to a device is refused unread"
reset_case
ln -s /dev/zero "$B/media/zero.jpg"
PATH_PREFIX="$T/count:" run_restore
bt_eq "link-file: exit status 1" 1 "$RC"
bt_has "link-file: the link is named" "$T/err" "ERROR: ./media/zero.jpg in $B $ODD"
bt_eq "link-file: nothing was hashed" 0 "$(wc -l < "$T/sha.log" | tr -d ' ')"
bt_eq "link-file: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case fifo: a FIFO is refused instead of hanging the check"
reset_case
mkfifo "$B/media/pipe.jpg"
PATH_PREFIX="$T/count:" run_restore
bt_eq "fifo: exit status 1" 1 "$RC"
bt_has "fifo: the FIFO is named" "$T/err" "ERROR: ./media/pipe.jpg in $B $ODD"
bt_eq "fifo: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "fifo: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case link-old: a backup made before 0.1.5 with a link is refused too"
reset_case
grep -vx 'checksums=SHA256SUMS' "$B/MANIFEST" > "$T/manifest.old"
cp "$T/manifest.old" "$B/MANIFEST"
rm -f "$B/SHA256SUMS"
mkdir -p "$T/outside"
ln -s "$T/outside" "$B/templates/elsewhere"
run_restore
bt_eq "link-old: exit status 1" 1 "$RC"
bt_has "link-old: the link is named" "$T/err" "ERROR: ./templates/elsewhere in $B $ODD"
bt_eq "link-old: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case listed-missing: a listed file that is gone is refused by name"
reset_case
rm -f "$B/media/a.jpg"
run_restore
bt_eq "listed-missing: exit status 1" 1 "$RC"
bt_has "listed-missing: the file is named" "$T/err" "ERROR: ./media/a.jpg is listed in SHA256SUMS and missing — $DAMAGED"
bt_eq "listed-missing: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "listed-missing: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case unreadable: a file that cannot be read is not called missing"
reset_case
PATH_PREFIX="$T/eio:" run_restore
bt_eq "unreadable: exit status 1" 1 "$RC"
bt_has "unreadable: it says the file cannot be read" "$T/err" "ERROR: a file of $B cannot be read, so the backup was not verified: sha256sum: ./media/b.jpg: Input/output error"
bt_not "unreadable: it does not call the file missing" "$T/err" "and missing"
bt_not "unreadable: it does not call the backup damaged" "$T/err" "$DAMAGED"
bt_not "unreadable: nothing claims to be verified" "$T/out" "files verified"
bt_eq "unreadable: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "unreadable: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
MISS1='MISSING media/1/gone.jpg (media row 4, website 1, "Sommerfest.jpg")'
MISS2='MISSING media/1/gone-thumb.webp (media_variants row 9, label thumb, of media row 4)'

section "Case missing-media: rows without a file are reported, restored anyway, status 3"
reset_case
run_restore FAKE_CHECK_MODE=missing
bt_eq "missing-media: exit status 3" 3 "$RC"
bt_eq "missing-media: the binary was asked with -files and the backup" 1 "$(grep -cxF "check check -files $B" "$T/hc.log")"
# The question is read from a pipe here, so bash does not print it; the first
# line after it is "Stopping holzcloud...".
STOPPING=$(line_of "$T/out" "Stopping holzcloud...")
bt_eq "missing-media: the restore went on" yes "$(if [ "$STOPPING" -gt 0 ]; then echo yes; else echo no; fi)"
bt_eq "missing-media: the first MISSING line comes before the question" yes \
    "$(if [ "$(line_of "$T/out" "$MISS1")" -gt 0 ] && [ "$(line_of "$T/out" "$MISS1")" -lt "$STOPPING" ]; then echo yes; else echo no; fi)"
bt_eq "missing-media: the second MISSING line comes before the question" yes \
    "$(if [ "$(line_of "$T/out" "$MISS2")" -gt 0 ] && [ "$(line_of "$T/out" "$MISS2")" -lt "$STOPPING" ]; then echo yes; else echo no; fi)"
bt_eq "missing-media: the service was stopped and started" "stop holzcloud,start holzcloud," "$(tr '\n' ',' < "$T/systemctl.log")"
bt_eq "missing-media: the data directory is the backup's" yes "$(restored)"
bt_has "missing-media: the closing warning counts two" "$T/err" "WARNING: restored, but 2 files that the database names are missing from the backup:"
WARNAT=$(line_of "$T/err" "WARNING: restored, but")
bt_eq "missing-media: the first MISSING line again after the warning" yes \
    "$(if [ "$(line_of "$T/err" "$MISS1")" -gt "$WARNAT" ]; then echo yes; else echo no; fi)"
bt_eq "missing-media: the second MISSING line again after the warning" yes \
    "$(if [ "$(line_of "$T/err" "$MISS2")" -gt "$WARNAT" ]; then echo yes; else echo no; fi)"

# ---------------------------------------------------------------------------
section "Case old-binary: a binary without the files marker gets a notice"
reset_case
run_restore FAKE_CHECK_MODE=old
bt_eq "old-binary: exit status 0" 0 "$RC"
bt_has "old-binary: the notice" "$T/out" "NOTICE: this holzcloud does not check media rows (older than 0.1.5); a missing media file is not reported"
bt_eq "old-binary: the data directory is the backup's" yes "$(restored)"

# ---------------------------------------------------------------------------
section "Case corrupt: an integrity failure keeps its sentence"
reset_case
run_restore FAKE_CHECK_MODE=corrupt
bt_eq "corrupt: exit status 1" 1 "$RC"
bt_has "corrupt: the integrity sentence" "$T/err" "ERROR: the snapshot fails its integrity check — do not restore it"
bt_not "corrupt: not the neutral sentence" "$T/err" "refused the snapshot"
bt_eq "corrupt: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "corrupt: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
section "Case binary-refuses: any other refusal is shown and named neutrally"
reset_case
run_restore FAKE_CHECK_MODE=newer
bt_eq "binary-refuses: exit status 1" 1 "$RC"
bt_has "binary-refuses: the binary's own line is shown" "$T/out" "error: the database is at migration level 62"
bt_has "binary-refuses: the neutral sentence" "$T/err" "ERROR: the installed holzcloud refused the snapshot (see above) — do not restore it"
bt_not "binary-refuses: not the integrity sentence" "$T/err" "fails its integrity check"
bt_eq "binary-refuses: the service was not stopped" 0 "$(grep -c stop "$T/systemctl.log")"
bt_eq "binary-refuses: the live data is untouched" yes "$(live_untouched)"

# ---------------------------------------------------------------------------
# The way back a failed restore prints is played as it stands, with the fakes
# on PATH: what follows "to go back: " on the ERROR line.
play_way_back() {
    sed -n 's/^.*[Tt]o go back: //p' "$T/err" | head -n 1 > "$T/way-back.sh"
    if [ ! -s "$T/way-back.sh" ]; then
        echo "no way back"
        return
    fi
    if env PATH="$T/bin:$PATH" FAKE_SYSTEMCTL_LOG="$T/systemctl.log" "$BASH" "$T/way-back.sh" > "$T/way-back.out" 2>&1; then
        echo ok
    else
        echo "failed: $(cat "$T/way-back.out")"
    fi
}
STOPPED_LINE="ERROR: the restore failed after holzcloud was stopped, and holzcloud is not running"

section "Case fails-after-stop: a failure after the stop prints the way back, which works"
reset_case
run_restore FAKE_CHOWN_FAIL=1
bt_eq "fails-after-stop: exit status 1" 1 "$RC"
bt_has "fails-after-stop: the stopped line" "$T/err" "$STOPPED_LINE"
bt_has "fails-after-stop: the set-aside directory is named" "$T/err" "the previous data is in $LIVE.before-restore-"
bt_eq "fails-after-stop: the service was stopped, not started" "stop holzcloud," "$(tr '\n' ',' < "$T/systemctl.log")"
bt_eq "fails-after-stop: the set-aside directory holds the old site" yes \
    "$(if [ -f "$(ls -d "$LIVE".before-restore-* 2>/dev/null | head -n 1)/sentinel.txt" ]; then echo yes; else echo no; fi)"
bt_eq "fails-after-stop: the way back runs" ok "$(play_way_back)"
bt_eq "fails-after-stop: after it the live data is the old site" yes "$(live_untouched)"
bt_eq "fails-after-stop: after it no set-aside directory is left" 0 "$(ls -d "$LIVE".before-restore-* 2>/dev/null | wc -l | tr -d ' ')"
bt_eq "fails-after-stop: the way back starts the service" "stop holzcloud,start holzcloud," "$(tr '\n' ',' < "$T/systemctl.log")"

# ---------------------------------------------------------------------------
section "Case mount: a data directory that is a mount point keeps its place, its contents go aside"
reset_case
run_restore FAKE_MOUNT="$LIVE"
bt_eq "mount: exit status 0" 0 "$RC"
bt_eq "mount: the data directory is the backup's" yes "$(restored)"
bt_eq "mount: the old site is in a directory on the same disk" yes \
    "$(if [ -f "$(ls -d "$LIVE"/.before-restore-* 2>/dev/null | head -n 1)/sentinel.txt" ]; then echo yes; else echo no; fi)"
bt_eq "mount: nothing was set aside beside it" 0 "$(ls -d "$LIVE".before-restore-* 2>/dev/null | wc -l | tr -d ' ')"
bt_has "mount: it says where the old site went" "$T/out" "is a mount point; moving its contents to $LIVE/.before-restore-"

# ---------------------------------------------------------------------------
section "Case mount-fails: a mount point and a failure after the stop: the way back works"
reset_case
run_restore FAKE_MOUNT="$LIVE" FAKE_CHOWN_FAIL=1
bt_eq "mount-fails: exit status 1" 1 "$RC"
bt_has "mount-fails: the stopped line" "$T/err" "$STOPPED_LINE"
bt_has "mount-fails: the set-aside directory is named" "$T/err" "the previous data is in $LIVE/.before-restore-"
bt_eq "mount-fails: the way back runs" ok "$(play_way_back)"
bt_eq "mount-fails: after it the live data is the old site" yes "$(live_untouched)"
bt_eq "mount-fails: after it the restored database is gone" no "$(exists "$LIVE/holzcloud.sqlite")"
bt_eq "mount-fails: after it no set-aside directory is left" 0 "$(ls -d "$LIVE"/.before-restore-* 2>/dev/null | wc -l | tr -d ' ')"

# ---------------------------------------------------------------------------
if [ "$K" -eq 0 ]; then
    echo "RESTORE-ROBUST OK ($N cases)"
    exit 0
fi
echo "RESTORE-ROBUST FAILED ($K of $N cases)"
exit 1
