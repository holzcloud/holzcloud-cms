#!/usr/bin/env bash
set -euo pipefail

# holzcloud-CMS backup
#
# The database snapshot is taken by the binary itself (VACUUM INTO through the
# pure-Go driver) and verified with an integrity check before this script counts
# it as a backup. The previous version shelled out to the sqlite3 CLI — the very
# dependency modernc.org/sqlite exists to avoid — and verified nothing, so a
# corrupted database would be faithfully copied over the last good snapshot
# until every retained copy was broken.
#
# What a backup holds: the database snapshot, csrf.key and five directories of
# the data directory: media, templates, plugins (the installed plugin modules;
# the database keeps only the manifest and the hash of a plugin, the module
# files are here), sprachen (the operator's language files) and marke (the
# logo), plus a MANIFEST that is written last.
#
# What it deliberately does not hold: the pre-upgrade snapshots the service
# keeps in the data directory (old-schema copies, they would not open under the
# running binary), the service's readiness probe file, and everything outside
# the data directory: the unit file and its drop-ins, environment files, the
# secret key (HOLZCLOUD_SECRET_KEY) and the proxy configuration.
#
# The pair is not atomic. The database snapshot is taken first and the files
# are copied afterwards, with the site running. A file added in between is in
# the backup without a row, which is harmless; a file deleted in between can
# leave a row whose file the backup lacks. Back up when the site is quiet if
# that matters.
#
# Another directory joining the data directory has to be added to the list in
# step 3, here and in restore.sh.
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
# Empty until this run's own mkdir of BACKUP_DIR has succeeded; the cleanup acts
# on this path and on no other.
OWN_DIR=""

# Runs on every way out of the script. A night that ends before its MANIFEST
# exists leaves a directory restore.sh refuses and that holds disk space, so
# the next night's guard would refuse too. The rule is the absence of MANIFEST,
# never $?: inside an EXIT trap that runs because of SIGTERM bash reports $? as
# 0. The function does not call exit, so bash keeps the status that brought it
# here, and nothing in it may end early under set -e.
cleanup_failed_night() {
    if [ -z "${OWN_DIR}" ]; then
        return 0
    fi
    if [ -f "${OWN_DIR}/MANIFEST" ]; then
        return 0
    fi
    case "${OWN_DIR}" in
        "${BACKUP_ROOT}"/?*) ;;
        *) return 0 ;;
    esac
    if rm -rf -- "${OWN_DIR}"; then
        echo "ERROR: backup did not finish; removed ${OWN_DIR}, which had no MANIFEST" >&2 || true
    else
        echo "ERROR: backup did not finish and ${OWN_DIR} could not be removed; it has no MANIFEST and restore.sh refuses it, remove it by hand" >&2 || true
    fi
    return 0
}

# run_rsync WHAT RSYNC_ARGS...: the only place that starts rsync. A file that
# vanishes during the copy ends rsync with status 24; that is repeated once.
# A second 24 ends the night with status 24, any other status ends it with that
# status and is never repeated. WHAT names the copy in the messages.
run_rsync() {
    local what="$1"
    local rc=0
    shift
    rsync "$@" || rc=$?
    if [ "${rc}" -eq 0 ]; then
        return 0
    fi
    if [ "${rc}" -ne 24 ]; then
        exit "${rc}"
    fi
    echo "WARNING: rsync exited 24 (files vanished) while copying ${what}; repeating it once" >&2
    rc=0
    rsync "$@" || rc=$?
    if [ "${rc}" -eq 0 ]; then
        return 0
    fi
    if [ "${rc}" -eq 24 ]; then
        echo "ERROR: rsync exited 24 again while copying ${what}; giving up" >&2
    fi
    exit "${rc}"
}

echo "=== holzcloud-CMS backup: ${TIMESTAMP} ==="

mkdir -p "${BACKUP_ROOT}"

# A backup that runs out of space halfway through is worse than none: it leaves
# a truncated file that looks like a snapshot.
free_mb=$(df -Pm "${BACKUP_ROOT}" | awk 'NR==2 {print $4}')
if [ "${free_mb}" -lt "${MIN_FREE_MB}" ]; then
    echo "ERROR: only ${free_mb} MB free at ${BACKUP_ROOT}, need ${MIN_FREE_MB} MB" >&2
    exit 1
fi

# A directory under the name of this run was not made by this run. Nothing of
# it is touched, now or by the cleanup.
if [ -e "${BACKUP_DIR}" ]; then
    echo "ERROR: ${BACKUP_DIR} exists already; refusing to start so that no earlier backup is touched" >&2
    exit 1
fi

# A plain mkdir: it fails rather than reuse a directory that appeared since the
# test above.
mkdir "${BACKUP_DIR}"
OWN_DIR="${BACKUP_DIR}"
trap cleanup_failed_night EXIT
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

# 3. The directories of the data directory that matter. A name that is not a
#    directory is skipped silently.
for name in media templates plugins sprachen marke; do
    if [ -d "${DATA_DIR}/${name}" ]; then
        echo "Backing up ${name}..."
        run_rsync "${name}" -a "${DATA_DIR}/${name}/" "${BACKUP_DIR}/${name}/"
    fi
done

# 4. A manifest, so a restore can tell what it is looking at. It is written
#    last: a restore takes its presence as the proof that every step before it
#    finished. It is written under another name and renamed, so it is never
#    there half written.
{
    echo "timestamp=${TIMESTAMP}"
    echo "version=$(HOLZCLOUD_DATA_DIR=${DATA_DIR} ${HOLZCLOUD_BIN} version 2>/dev/null || echo unknown)"
    echo "data_dir=${DATA_DIR}"
    echo "host=$(hostname)"
} > "${BACKUP_DIR}/MANIFEST.partial"
mv "${BACKUP_DIR}/MANIFEST.partial" "${BACKUP_DIR}/MANIFEST"

# 5. Retention. Without this the disk fills up and takes the site with it.
if [ "${RETENTION_DAYS}" -gt 0 ]; then
    echo "Pruning backups older than ${RETENTION_DAYS} days..."
    find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -mtime "+${RETENTION_DAYS}" \
        -exec rm -rf {} + 2>/dev/null || true
fi

# 6. Off-box copy. A backup on the same disk does not survive the failure it is
#    meant to protect against — which DEPLOY.md itself calls likely.
if [ -n "${REMOTE_TARGET:-}" ]; then
    echo "Copying to ${REMOTE_TARGET}..."
    run_rsync "the off-site copy" -a --delete "${BACKUP_ROOT}/" "${REMOTE_TARGET}/"
fi

echo "=== Backup complete: ${BACKUP_DIR} ==="
du -sh "${BACKUP_DIR}"
