#!/usr/bin/env bash
set -euo pipefail

# holzcloud-CMS restore
#
# The counterpart that was missing: DEPLOY.md had a backup section and no
# restore section, so the backups had never been proven to be restorable.
#
# The live data directory is moved aside rather than deleted. If the restore
# turns out to be the wrong snapshot, the previous state is still on disk. The
# directory is never removed by this script; it holds the media of the replaced
# site and is deleted by hand once the restored site is accepted.
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
# Order, which a refused backup depends on: verify the checksums of SHA256SUMS,
# verify a copy of the snapshot, ask, stop the service, move the data aside,
# copy, chown, chmod, start, wait for /readyz. Nothing is stopped or moved
# before both checks have passed.
#
# Another directory joining the data directory has to be added to the list
# below, here and in backup.sh.
#
# Usage:
#   sudo ./restore.sh /opt/holzcloud/backups/20260802-031500
#
# Exit status:
#   0  restored (checksums verified, or a backup made before 0.1.5, with a notice)
#   1  refused or failed: nothing stopped when the backup was refused; when the
#      site did not report ready after the restore, see the journal
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

# Verify the checksums BEFORE touching the live data. The MANIFEST decides,
# never the presence of the file: backup.sh writes the line
# checksums=SHA256SUMS since 0.1.5, so a backup with the line and without the
# file is damaged, and one without the line was made earlier and is restored
# unverified, with a notice.
#
# SHA256SUMS is input from the backup to a script that runs as root. Every line
# is checked before any file is read: "<64 hex>  ./path" with no .. segment, or
# a line beginning with a backslash, which sha256sum writes for a name holding
# a backslash or a newline; such a line is counted and never followed. Then the
# listed paths are hashed from inside the backup directory and compared in the
# order of SHA256SUMS, so the first file that is missing or differs is named.
# The output of sha256sum -c is not used: GNU and Darwin word and order it
# differently, and a missing file goes to stderr.
if grep -qx 'checksums=SHA256SUMS' "${BACKUP_DIR}/MANIFEST"; then
    if [ ! -f "${BACKUP_DIR}/SHA256SUMS" ]; then
        echo "ERROR: ${BACKUP_DIR}/MANIFEST names SHA256SUMS but the file is missing — do not restore it" >&2
        exit 1
    fi
    echo "Verifying checksums..."
    bad_line=$(LC_ALL=C awk '
        substr($0, 1, 1) == "\\" { next }
        length($0) < 69 || substr($0, 1, 64) !~ /^[0-9a-f]+$/ || substr($0, 65, 4) != "  ./" { print NR; exit }
        substr($0, 67) ~ /(^|\/)\.\.(\/|$)/ { print NR; exit }' "${BACKUP_DIR}/SHA256SUMS")
    if [ -n "${bad_line}" ]; then
        echo "ERROR: line ${bad_line} of SHA256SUMS is not in the format backup.sh writes or names a path outside the backup — do not restore it" >&2
        exit 1
    fi
    escaped=$(LC_ALL=C awk 'substr($0, 1, 1) == "\\" { n++ } END { print n + 0 }' "${BACKUP_DIR}/SHA256SUMS")
    listed=$(LC_ALL=C awk 'substr($0, 1, 1) != "\\" { n++ } END { print n + 0 }' "${BACKUP_DIR}/SHA256SUMS")
    ACTUAL_SUMS=$(mktemp)
    # A missing file makes sha256sum end 1; the comparison below decides. A
    # checked line holds no newline in its path, so newline-separated paths
    # are exact here.
    (
        cd "${BACKUP_DIR}"
        LC_ALL=C awk 'substr($0, 1, 1) != "\\" { print substr($0, 67) }' SHA256SUMS \
            | tr '\n' '\0' | xargs -0 -r sha256sum
    ) > "${ACTUAL_SUMS}" 2>/dev/null || true
    first=$(LC_ALL=C awk -v fa="${ACTUAL_SUMS}" '
        FILENAME == fa { A[substr($0, 67)] = substr($0, 1, 64); next }
        substr($0, 1, 1) == "\\" { next }
        {
            p = substr($0, 67)
            if (!(p in A)) { print "missing " p; exit }
            if (A[p] != substr($0, 1, 64)) { print "differs " p; exit }
        }' "${ACTUAL_SUMS}" "${BACKUP_DIR}/SHA256SUMS")
    case "${first}" in
        "missing "*)
            echo "ERROR: ${first#missing } is listed in SHA256SUMS and missing — the backup is damaged, do not restore it" >&2
            exit 1
            ;;
        "differs "*)
            echo "ERROR: ${first#differs } differs from its checksum in SHA256SUMS — the backup is damaged, do not restore it" >&2
            exit 1
            ;;
    esac
    echo "Checksums: ${listed} files verified"
    if [ "${escaped}" -gt 0 ]; then
        echo "NOTICE: ${escaped} lines of SHA256SUMS name a file with a backslash or a newline in its name; those files were not verified"
    fi
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

echo "Stopping ${SERVICE}..."
systemctl stop "${SERVICE}"

# Move aside, never delete.
if [ -d "${DATA_DIR}" ]; then
    ASIDE="${DATA_DIR}.before-restore-$(date +%Y%m%d-%H%M%S)"
    echo "Moving current data to ${ASIDE}"
    mv "${DATA_DIR}" "${ASIDE}"
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
