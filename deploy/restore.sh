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
# Order, which a refused backup depends on: verify a copy of the snapshot, ask,
# stop the service, move the data aside, copy, chown, chmod, start, wait for
# /readyz. Nothing is stopped or moved before the snapshot has passed.
#
# Another directory joining the data directory has to be added to the list
# below, here and in backup.sh.
#
# Usage:
#   sudo ./restore.sh /opt/holzcloud/backups/20260802-031500
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

# Verify the snapshot BEFORE touching the live data. Restoring a corrupt file
# over a working one turns a recoverable problem into an unrecoverable one.
echo "Verifying snapshot..."
VERIFY_DIR=$(mktemp -d)
trap 'rm -rf "${VERIFY_DIR}"' EXIT
cp "${BACKUP_DIR}/holzcloud.sqlite" "${VERIFY_DIR}/holzcloud.sqlite"
if ! HOLZCLOUD_DATA_DIR="${VERIFY_DIR}" "${HOLZCLOUD_BIN}" check; then
    echo "ERROR: the snapshot fails its integrity check — do not restore it" >&2
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
        exit 0
    fi
    sleep 1
done

echo "WARNING: /readyz did not report ready within 30s — check: journalctl -u ${SERVICE} -n 50" >&2
exit 1
