#!/usr/bin/env bash
set -euo pipefail

# holzcloud-CMS restore
#
# The counterpart that was missing: DEPLOY.md had a backup section and no
# restore section, so the backups had never been proven to be restorable.
#
# The live data directory is moved aside rather than deleted (its contents,
# when it is a mount point). If the restore turns out to be the wrong snapshot,
# the previous state is still on disk. The directory is never removed by this
# script; it holds the media of the replaced site and is deleted by hand once
# the restored site is accepted.
#
# What a restore brings back: the database, csrf.key and the five directories
# media, templates, plugins, sprachen and marke, each only if the backup has it
# (a backup made before plugins, sprachen and marke were carried restores as it
# always did, without them). A directory without a MANIFEST is refused: backup.sh
# writes it last, so a backup without one was interrupted.
#
# What it does not bring back: the pre-upgrade snapshots in the replaced data
# directory (they stay in the set-aside copy), the service's readiness probe
# file, and everything outside the data directory: the unit file and its
# drop-ins, environment files, the secret key (HOLZCLOUD_SECRET_KEY) and the
# proxy configuration. Those are rebuilt by hand.
#
# Order, which a refused backup depends on: refuse links and special files,
# verify the checksums of SHA256SUMS, verify a copy of the snapshot, ask, stop
# the service, move the data aside, copy, chown, chmod, start, wait for
# /readyz. Nothing is stopped or moved before every check has passed.
#
# Another directory joining the data directory has to be added to the list
# below, here and in backup.sh.
#
# Usage:
#   sudo ./restore.sh /opt/holzcloud/backups/20260802-031500
#
# Exit status:
#   0  restored (checksums verified, or a backup made before 0.1.5, with a notice)
#   1  refused or failed: nothing stopped when the backup was refused; when a
#      step failed after the stop, the service stays stopped and the last line
#      names where the previous data is and the commands that put it back;
#      when the site did not report ready after the restore, see the journal
#   3  restored, but files that database rows name are missing from the backup;
#      they are listed before the question and again at the end
#
# Environment:
#   DATA_DIR       default /opt/holzcloud/data
#   SERVICE        default holzcloud
#   OWNER          default holzcloud:holzcloud
#   HOLZCLOUD_BIN  default /opt/holzcloud/holzcloud

BACKUP_DIR="${1:-}"
DATA_DIR="${DATA_DIR:-/opt/holzcloud/data}"
SERVICE="${SERVICE:-holzcloud}"
OWNER="${OWNER:-holzcloud:holzcloud}"
HOLZCLOUD_BIN="${HOLZCLOUD_BIN:-/opt/holzcloud/holzcloud}"

if [ -z "${BACKUP_DIR}" ] || [ ! -d "${BACKUP_DIR}" ]; then
    echo "usage: $0 <backup-directory>" >&2
    echo "available:" >&2
    ls -1 "$(dirname "${DATA_DIR}")/backups" 2>/dev/null >&2 || true
    exit 1
fi
# A symbolic link or a special file has no place in a backup: backup.sh makes
# neither. A link would be put back into the live data directory by rsync
# without its target ever being checked, and a FIFO or a device would hang the
# checks below forever, as root, before anything is stopped. So nothing of the
# backup is read before this search, which follows no link.
if ! odd=$(cd "${BACKUP_DIR}" && find . \( -type l -o \( ! -type f ! -type d \) \) -print -quit); then
    echo "ERROR: could not search ${BACKUP_DIR} for links and special files (see above); nothing was stopped" >&2
    exit 1
fi
if [ -n "${odd}" ]; then
    echo "ERROR: ${odd} in ${BACKUP_DIR} is a symbolic link or not a regular file; backup.sh makes neither — do not restore it" >&2
    exit 1
fi
if [ ! -f "${BACKUP_DIR}/holzcloud.sqlite" ]; then
    echo "ERROR: ${BACKUP_DIR}/holzcloud.sqlite not found" >&2
    exit 1
fi
# backup.sh writes the MANIFEST last, after the database and every directory,
# so a directory without one is a backup that was interrupted.
if [ ! -f "${BACKUP_DIR}/MANIFEST" ]; then
    echo "ERROR: ${BACKUP_DIR}/MANIFEST not found — the backup is incomplete, do not restore it" >&2
    exit 1
fi

echo "=== Restoring from ${BACKUP_DIR} ==="
cat "${BACKUP_DIR}/MANIFEST"

# Temporary files of the checks below, removed on every way out.
VERIFY_DIR=""
ACTUAL_SUMS=""
remove_temporaries() {
    if [ -n "${VERIFY_DIR}" ]; then
        rm -rf "${VERIFY_DIR}"
    fi
    if [ -n "${ACTUAL_SUMS}" ]; then
        rm -f "${ACTUAL_SUMS}"
    fi
}
trap remove_temporaries EXIT

# Verify the checksums BEFORE touching the live data. backup.sh writes
# SHA256SUMS and the MANIFEST line checksums=SHA256SUMS since 0.1.5. A backup
# with the line and without the file is damaged. A backup that has the file is
# verified whatever its MANIFEST says, so a MANIFEST that lost its line cannot
# wave a damaged backup through. Only a backup with neither, which an older
# release made, is restored unverified, with a notice.
#
# The check goes both ways. Every line of SHA256SUMS must name a file of the
# backup with that checksum, and every regular file of the backup except
# SHA256SUMS and the MANIFEST must be listed, ./holzcloud.sqlite included. An
# empty or truncated SHA256SUMS is refused, never counted as "0 files verified".
#
# SHA256SUMS is input from the backup to a script that runs as root. Every line
# is checked before any file is read: "<64 hex>  ./path" with no .. segment. A
# line beginning with a backslash, which GNU sha256sum writes for a name holding
# a backslash or a newline, is the same line after that backslash and is checked
# the same way. No path of SHA256SUMS is ever opened: the files are found with
# find, hashed the way backup.sh hashed them, and the two lists are compared by
# name, in the order of SHA256SUMS, so the first file that is missing or differs
# is named, then the first file that is not listed. The output of sha256sum -c
# is not used: GNU and Darwin word and order it differently, and a missing file
# goes to stderr.
if grep -qx 'checksums=SHA256SUMS' "${BACKUP_DIR}/MANIFEST" || [ -f "${BACKUP_DIR}/SHA256SUMS" ]; then
    if [ ! -f "${BACKUP_DIR}/SHA256SUMS" ]; then
        echo "ERROR: ${BACKUP_DIR}/MANIFEST names SHA256SUMS but the file is missing — do not restore it" >&2
        exit 1
    fi
    echo "Verifying checksums..."
    # o is where the hex starts: 2 after the backslash of an escaped line.
    bad_line=$(LC_ALL=C awk '
        { o = (substr($0, 1, 1) == "\\") ? 2 : 1 }
        length($0) < 68 + o || substr($0, o, 64) !~ /^[0-9a-f]+$/ || substr($0, o + 64, 4) != "  ./" { print NR; exit }
        substr($0, o + 66) ~ /(^|\/)\.\.(\/|$)/ { print NR; exit }' "${BACKUP_DIR}/SHA256SUMS")
    if [ -n "${bad_line}" ]; then
        echo "ERROR: line ${bad_line} of SHA256SUMS is not in the format backup.sh writes or names a path outside the backup — do not restore it" >&2
        exit 1
    fi
    listed=$(LC_ALL=C awk 'END { print NR }' "${BACKUP_DIR}/SHA256SUMS")
    if [ "${listed}" -eq 0 ]; then
        echo "ERROR: ${BACKUP_DIR}/SHA256SUMS is empty — the backup is damaged, do not restore it" >&2
        exit 1
    fi
    if ! LC_ALL=C grep -q '^[0-9a-f]\{64\}  \./holzcloud\.sqlite$' "${BACKUP_DIR}/SHA256SUMS"; then
        echo "ERROR: SHA256SUMS does not list ./holzcloud.sqlite — the backup is damaged, do not restore it" >&2
        exit 1
    fi
    ACTUAL_SUMS=$(mktemp)
    (
        cd "${BACKUP_DIR}"
        find . -type f ! -path ./SHA256SUMS ! -path ./MANIFEST -print0 \
            | LC_ALL=C sort -z | xargs -0 -r sha256sum
    ) > "${ACTUAL_SUMS}" 2>/dev/null || true
    # name(l) is the path as the line writes it; key(l) marks an escaped one,
    # so it never meets a plain name of the same spelling; hash(l) the hex.
    # SUMS_FILE goes through the environment: awk -v would expand backslashes.
    first=$(SUMS_FILE="${BACKUP_DIR}/SHA256SUMS" LC_ALL=C awk '
        function esc(l) { return substr(l, 1, 1) == "\\" }
        function name(l) { return esc(l) ? substr(l, 68) : substr(l, 67) }
        function key(l) { return (esc(l) ? "\\" : "/") name(l) }
        function hash(l) { return esc(l) ? substr(l, 2, 64) : substr(l, 1, 64) }
        FILENAME != ENVIRON["SUMS_FILE"] { A[key($0)] = hash($0); next }
        {
            k = key($0)
            if (k in L) { print "twice " name($0); exit }
            L[k] = 1
            if (!(k in A)) { print "missing " name($0); exit }
            if (A[k] != hash($0)) { print "differs " name($0); exit }
        }' "${ACTUAL_SUMS}" "${BACKUP_DIR}/SHA256SUMS")
    case "${first}" in
        "twice "*)
            echo "ERROR: ${first#twice } is listed twice in SHA256SUMS — the backup is damaged, do not restore it" >&2
            exit 1
            ;;
        "missing "*)
            echo "ERROR: ${first#missing } is listed in SHA256SUMS and missing — the backup is damaged, do not restore it" >&2
            exit 1
            ;;
        "differs "*)
            echo "ERROR: ${first#differs } differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it" >&2
            exit 1
            ;;
    esac
    unlisted=$(SUMS_FILE="${BACKUP_DIR}/SHA256SUMS" LC_ALL=C awk '
        function esc(l) { return substr(l, 1, 1) == "\\" }
        function name(l) { return esc(l) ? substr(l, 68) : substr(l, 67) }
        function key(l) { return (esc(l) ? "\\" : "/") name(l) }
        FILENAME == ENVIRON["SUMS_FILE"] { L[key($0)] = 1; next }
        !(key($0) in L) { print name($0); exit }' "${BACKUP_DIR}/SHA256SUMS" "${ACTUAL_SUMS}")
    if [ -n "${unlisted}" ]; then
        echo "ERROR: ${unlisted} is in the backup but not listed in SHA256SUMS — the backup is damaged, do not restore it" >&2
        exit 1
    fi
    echo "Checksums: ${listed} files verified"
else
    echo "NOTICE: ${BACKUP_DIR} has no checksum file (made before 0.1.5); its files were not verified"
fi

# Verify the snapshot BEFORE touching the live data. Restoring a corrupt file
# over a working one turns a recoverable problem into an unrecoverable one.
echo "Verifying snapshot..."
VERIFY_DIR=$(mktemp -d)
cp "${BACKUP_DIR}/holzcloud.sqlite" "${VERIFY_DIR}/holzcloud.sqlite"
# `check -files DIR` checks the snapshot and then names every media row whose
# file DIR lacks: status 0 none missing, 3 missing (integrity ok), 1 any
# refusal. A missing file is reported, never refused: the database is restored
# and the operator decides about the files (status 3 at the end). A binary
# older than 0.1.5 ignores the flag and prints no files line, so the line is
# required before "nothing missing" is believed.
MISSING_COUNT=0
MISSING_LINES=""
if check_out=$(HOLZCLOUD_DATA_DIR="${VERIFY_DIR}" "${HOLZCLOUD_BIN}" check -files "${BACKUP_DIR}" 2>&1); then
    check_rc=0
else
    check_rc=$?
fi
if [ -n "${check_out}" ]; then
    printf '%s\n' "${check_out}"
fi
files_line=$(printf '%s\n' "${check_out}" | grep -E '^files: [0-9]+ rows checked, [0-9]+ missing$' || true)
if [ "${check_rc}" -eq 0 ] && [ -n "${files_line}" ]; then
    :
elif [ "${check_rc}" -eq 0 ]; then
    echo "NOTICE: this holzcloud does not check media rows (older than 0.1.5); a missing media file is not reported"
elif [ "${check_rc}" -eq 3 ] && [ -n "${files_line}" ]; then
    MISSING_COUNT=$(printf '%s\n' "${files_line}" | sed -n 's/^files: [0-9]* rows checked, \([0-9]*\) missing$/\1/p')
    MISSING_LINES=$(printf '%s\n' "${check_out}" | grep '^MISSING ' || true)
else
    if printf '%s\n' "${check_out}" | grep '^integrity: ' | grep -qvx 'integrity: ok'; then
        echo "ERROR: the snapshot fails its integrity check — do not restore it" >&2
    else
        echo "ERROR: the installed holzcloud refused the snapshot (see above) — do not restore it" >&2
    fi
    exit 1
fi

read -r -p "Stop ${SERVICE} and replace ${DATA_DIR}? [yes/NO] " confirm
[ "${confirm}" = "yes" ] || { echo "aborted"; exit 1; }

# Where the current data goes. A data directory that is a mount point (a data
# disk; holzcloud.service names it in RequiresMountsFor) cannot be renamed:
# mv fails with "Device or resource busy". Its contents are moved instead,
# into a directory on the same disk, so the move is a rename and needs no
# space. Earlier set-aside directories stay where they are.
is_mount_point() {
    if command -v mountpoint > /dev/null 2>&1; then
        mountpoint -q -- "$1"
    else
        [ "$(df -P -- "$1" 2> /dev/null | awk 'NR == 2 { print $6 }')" = "$1" ]
    fi
}
ASIDE_STAMP=$(date +%Y%m%d-%H%M%S)
DATA_IS_MOUNT=""
if [ -d "${DATA_DIR}" ] && is_mount_point "${DATA_DIR}"; then
    DATA_IS_MOUNT=yes
    ASIDE="${DATA_DIR}/.before-restore-${ASIDE_STAMP}"
else
    ASIDE="${DATA_DIR}.before-restore-${ASIDE_STAMP}"
fi

# From the stop until the start, a failure leaves the site down and the data
# directory half written; set -e would end the script without a word about
# the way back. STAGE says how far the restore got, and the trap prints the
# commands that put the previous data back and start the service.
STAGE=""
way_back() {
    local d a s mv_back
    d=$(printf '%q' "${DATA_DIR}")
    a=$(printf '%q' "${ASIDE}")
    s=$(printf '%q' "${SERVICE}")
    mv_back="find ${a} -mindepth 1 -maxdepth 1 -exec sh -c 'mv -- \"\$@\" \"\$0\"' ${d} {} + && rmdir ${a}"
    case "${STAGE}" in
        stopped)
            echo "ERROR: the restore failed after ${SERVICE} was stopped, and ${SERVICE} is not running; ${DATA_DIR} was not changed. To go back: systemctl start ${s}" >&2
            ;;
        moving)
            echo "ERROR: the restore failed after ${SERVICE} was stopped, and ${SERVICE} is not running; some of the previous data is in ${ASIDE}, the rest still in ${DATA_DIR}. As root, to go back: ${mv_back} && systemctl start ${s}" >&2
            ;;
        moved)
            if [ -n "${DATA_IS_MOUNT}" ]; then
                echo "ERROR: the restore failed after ${SERVICE} was stopped, and ${SERVICE} is not running; ${DATA_DIR} is half written and the previous data is in ${ASIDE}. As root, to go back: find ${d} -mindepth 1 -maxdepth 1 ! -name '.before-restore-*' -exec rm -rf -- {} + && ${mv_back} && systemctl start ${s}" >&2
            else
                echo "ERROR: the restore failed after ${SERVICE} was stopped, and ${SERVICE} is not running; ${DATA_DIR} is half written and the previous data is in ${ASIDE}. As root, to go back: rm -rf -- ${d} && mv -- ${a} ${d} && systemctl start ${s}" >&2
            fi
            ;;
        fresh)
            echo "ERROR: the restore failed after ${SERVICE} was stopped, and ${SERVICE} is not running; there was no ${DATA_DIR} before, so there is nothing to go back to. Fix the cause (see above) and run the restore again" >&2
            ;;
    esac
    return 0
}
on_exit() {
    remove_temporaries
    way_back
}
trap on_exit EXIT

echo "Stopping ${SERVICE}..."
systemctl stop "${SERVICE}"
STAGE=stopped

# Move aside, never delete.
if [ -n "${DATA_IS_MOUNT}" ]; then
    echo "${DATA_DIR} is a mount point; moving its contents to ${ASIDE}"
    mkdir "${ASIDE}"
    STAGE=moving
    find "${DATA_DIR}" -mindepth 1 -maxdepth 1 ! -name '.before-restore-*' \
        -exec sh -c 'mv -- "$@" "$0"' "${ASIDE}" {} +
    STAGE=moved
elif [ -d "${DATA_DIR}" ]; then
    echo "Moving current data to ${ASIDE}"
    mv "${DATA_DIR}" "${ASIDE}"
    STAGE=moved
else
    STAGE=fresh
fi
mkdir -p "${DATA_DIR}"

echo "Restoring database..."
cp "${BACKUP_DIR}/holzcloud.sqlite" "${DATA_DIR}/holzcloud.sqlite"
chmod 600 "${DATA_DIR}/holzcloud.sqlite"

if [ -f "${BACKUP_DIR}/csrf.key" ]; then
    echo "Restoring CSRF key..."
    cp -p "${BACKUP_DIR}/csrf.key" "${DATA_DIR}/csrf.key"
fi
for name in media templates plugins sprachen marke; do
    if [ -d "${BACKUP_DIR}/${name}" ]; then
        echo "Restoring ${name}..."
        rsync -a "${BACKUP_DIR}/${name}/" "${DATA_DIR}/${name}/"
    fi
done

chown -R "${OWNER}" "${DATA_DIR}"
chmod 700 "${DATA_DIR}"

echo "Starting ${SERVICE}..."
systemctl start "${SERVICE}"
STAGE=started

echo "Waiting for readiness..."
for _ in $(seq 1 30); do
    if curl -fsS http://127.0.0.1:8080/readyz >/dev/null 2>&1; then
        echo "=== Restore complete ==="
        curl -fsS http://127.0.0.1:8080/readyz
        echo
        if [ "${MISSING_COUNT}" -gt 0 ]; then
            echo "WARNING: restored, but ${MISSING_COUNT} files that the database names are missing from the backup:" >&2
            printf '%s\n' "${MISSING_LINES}" >&2
            exit 3
        fi
        exit 0
    fi
    sleep 1
done

echo "WARNING: /readyz did not report ready within 30s — check: journalctl -u ${SERVICE} -n 50" >&2
exit 1
