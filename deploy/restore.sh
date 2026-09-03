#!/usr/bin/env bash
set -euo pipefail

# Holzcloud CMS restore
#
# The counterpart that was missing: DEPLOY.md had a backup section and no
# restore section, so the backups had never been proven to be restorable.
#
# The live data directory is moved aside rather than deleted. If the restore
# turns out to be the wrong snapshot, the previous state is still on disk.
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

echo "=== Restoring from ${BACKUP_DIR} ==="
[ -f "${BACKUP_DIR}/MANIFEST" ] && cat "${BACKUP_DIR}/MANIFEST"

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

if [ -f "${BACKUP_DIR}/csrf.key" ]; then
    echo "Restoring CSRF key..."
    cp -p "${BACKUP_DIR}/csrf.key" "${DATA_DIR}/csrf.key"
fi
if [ -d "${BACKUP_DIR}/media" ]; then
    echo "Restoring media..."
    rsync -a "${BACKUP_DIR}/media/" "${DATA_DIR}/media/"
fi
if [ -d "${BACKUP_DIR}/templates" ]; then
    echo "Restoring templates..."
    rsync -a "${BACKUP_DIR}/templates/" "${DATA_DIR}/templates/"
fi

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
