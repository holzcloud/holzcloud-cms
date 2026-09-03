#!/usr/bin/env bash
set -euo pipefail

# Holzcloud CMS backup
#
# The database snapshot is taken by the binary itself (VACUUM INTO through the
# pure-Go driver) and verified with an integrity check before this script counts
# it as a backup. The previous version shelled out to the sqlite3 CLI — the very
# dependency modernc.org/sqlite exists to avoid — and verified nothing, so a
# corrupted database would be faithfully copied over the last good snapshot
# until every retained copy was broken.
#
# Usage:
#   ./backup.sh [backup_dir]
#
# Environment:
#   DATA_DIR         default /opt/holzcloud/data
#   HOLZCLOUD_BIN    default /opt/holzcloud/holzcloud
#   RETENTION_DAYS   default 30
#   REMOTE_TARGET    optional rsync target, e.g. user@nas:/backups/holzcloud
#   MIN_FREE_MB      default 1024 — refuse to start below this

DATA_DIR="${DATA_DIR:-/opt/holzcloud/data}"
HOLZCLOUD_BIN="${HOLZCLOUD_BIN:-/opt/holzcloud/holzcloud}"
BACKUP_ROOT="${1:-/opt/holzcloud/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
MIN_FREE_MB="${MIN_FREE_MB:-1024}"
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
BACKUP_DIR="${BACKUP_ROOT}/${TIMESTAMP}"

echo "=== Holzcloud backup: ${TIMESTAMP} ==="

mkdir -p "${BACKUP_ROOT}"

# A backup that runs out of space halfway through is worse than none: it leaves
# a truncated file that looks like a snapshot.
free_mb=$(df -Pm "${BACKUP_ROOT}" | awk 'NR==2 {print $4}')
if [ "${free_mb}" -lt "${MIN_FREE_MB}" ]; then
    echo "ERROR: only ${free_mb} MB free at ${BACKUP_ROOT}, need ${MIN_FREE_MB} MB" >&2
    exit 1
fi

mkdir -p "${BACKUP_DIR}"
chmod 700 "${BACKUP_DIR}"

# 1. Database — snapshot and integrity check in one step, non-zero on failure.
echo "Backing up database..."
HOLZCLOUD_DATA_DIR="${DATA_DIR}" "${HOLZCLOUD_BIN}" backup "${BACKUP_DIR}/holzcloud.sqlite"

# 2. CSRF key. Without it every existing session and form token is invalidated
#    on restore — the old script skipped it entirely.
if [ -f "${DATA_DIR}/csrf.key" ]; then
    echo "Backing up CSRF key..."
    cp -p "${DATA_DIR}/csrf.key" "${BACKUP_DIR}/csrf.key"
fi

# 3. Media files
if [ -d "${DATA_DIR}/media" ]; then
    echo "Backing up media..."
    rsync -a "${DATA_DIR}/media/" "${BACKUP_DIR}/media/"
fi

# 4. User-uploaded templates
if [ -d "${DATA_DIR}/templates" ]; then
    echo "Backing up templates..."
    rsync -a "${DATA_DIR}/templates/" "${BACKUP_DIR}/templates/"
fi

# 5. A manifest, so a restore can tell what it is looking at.
{
    echo "timestamp=${TIMESTAMP}"
    echo "version=$(HOLZCLOUD_DATA_DIR=${DATA_DIR} ${HOLZCLOUD_BIN} version 2>/dev/null || echo unknown)"
    echo "data_dir=${DATA_DIR}"
    echo "host=$(hostname)"
} > "${BACKUP_DIR}/MANIFEST"

# 6. Retention. Without this the disk fills up and takes the site with it.
if [ "${RETENTION_DAYS}" -gt 0 ]; then
    echo "Pruning backups older than ${RETENTION_DAYS} days..."
    find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -mtime "+${RETENTION_DAYS}" \
        -exec rm -rf {} + 2>/dev/null || true
fi

# 7. Off-box copy. A backup on the same disk does not survive the failure it is
#    meant to protect against — which DEPLOY.md itself calls likely.
if [ -n "${REMOTE_TARGET:-}" ]; then
    echo "Copying to ${REMOTE_TARGET}..."
    rsync -a --delete "${BACKUP_ROOT}/" "${REMOTE_TARGET}/"
fi

echo "=== Backup complete: ${BACKUP_DIR} ==="
du -sh "${BACKUP_DIR}"
