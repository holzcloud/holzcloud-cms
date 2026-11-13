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
# logo), plus SHA256SUMS and a MANIFEST that is written last.
#
# SHA256SUMS holds one line per regular file of the backup directory except
# itself and the MANIFEST at the top, in sha256sum's own format ("<hex>  ./path",
# C order). It is written before the MANIFEST, so "MANIFEST present" still means
# "the night finished, checksums included", and the MANIFEST says so with the
# line checksums=SHA256SUMS. restore.sh verifies every file against it before
# anything is stopped. SHA-256 here detects rot and accidental damage, not
# tampering: whoever can write the backup can rewrite a file and its line alike.
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
# A file that vanishes while it is copied ends rsync with status 24. The copy is
# repeated once; a second 24 ends the night with status 24 and a message that
# names what was being copied. Any other rsync status is never repeated.
#
# Retention removes old backup directories one by one. One it cannot remove,
# or a listing of the backup root that fails, is named in an ERROR line on
# stderr; the night still makes its off-site copy, prints its complete line and
# ends with status 40, so the timer unit shows the failure instead of a full
# disk weeks later. A failed off-site copy ends the night first, with rsync's
# status.
#
# Exit status: 0 complete; 40 complete, but retention could not list or remove
# old backups; any other non-zero status: the night failed (rsync's own status
# for a copy).
#
# A night that fails before its MANIFEST exists removes the directory it made,
# so a half-written directory does not sit on the disk and make the free-space
# guard refuse every later night. An earlier backup is never touched: the
# script refuses to start when the directory of this run exists already, and
# the cleanup removes only the directory this run made itself. A directory that
# holds a MANIFEST is never removed, whatever fails afterwards (retention, the
# off-site copy). The MANIFEST is written under another name and renamed, so it
# is never there half written and "no MANIFEST" means "the night did not
# finish". Limit: SIGKILL (including the kernel's out-of-memory kill) and a
# power loss cannot be trapped, so a directory without MANIFEST can still
# remain after them; restore.sh still refuses it.
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

# 4. Checksums. One SHA256SUMS line per regular file of the night, in the
#    format sha256sum writes ("<hex>  ./path"), sorted in C order, so that
#    restore.sh can tell a damaged file by name before anything is stopped.
#    The four top-level names are excluded by anchored -path, never by -name:
#    a template may well carry its own MANIFEST.json, and that is listed. Every
#    path begins with ./, so no option can be mistaken for a file name. A hash
#    that fails ends the night here, before the MANIFEST, and the trap removes
#    the directory.
echo "Writing checksums..."
(
    cd "${BACKUP_DIR}"
    find . -type f \
        ! -path ./SHA256SUMS ! -path ./SHA256SUMS.partial \
        ! -path ./MANIFEST ! -path ./MANIFEST.partial -print0 \
        | LC_ALL=C sort -z | xargs -0 -r sha256sum
) > "${BACKUP_DIR}/SHA256SUMS.partial"
mv "${BACKUP_DIR}/SHA256SUMS.partial" "${BACKUP_DIR}/SHA256SUMS"

# 5. A manifest, so a restore can tell what it is looking at. It is written
#    last: a restore takes its presence as the proof that every step before it
#    finished, the checksums included. It is written under another name and
#    renamed, so it is never there half written. The checksums line is what
#    tells restore.sh that SHA256SUMS belongs to this backup: a backup without
#    it was made before 0.1.5, one with it and without the file is damaged.
{
    echo "timestamp=${TIMESTAMP}"
    echo "version=$(HOLZCLOUD_DATA_DIR=${DATA_DIR} ${HOLZCLOUD_BIN} version 2>/dev/null || echo unknown)"
    echo "data_dir=${DATA_DIR}"
    echo "host=$(hostname)"
    echo "checksums=SHA256SUMS"
} > "${BACKUP_DIR}/MANIFEST.partial"
mv "${BACKUP_DIR}/MANIFEST.partial" "${BACKUP_DIR}/MANIFEST"

# 6. Retention. Without this the disk fills up and takes the site with it, so
#    a failure here is never silent: the directories are listed first and
#    removed one by one, and each one that cannot be removed is named on stderr
#    (the journal of holzcloud-backup.service), next to rm's own lines. The
#    night still goes on to the off-site copy and ends with status 40 below.
#    Only direct children of the backup root are touched, never the directory
#    of this night.
RETENTION_FAILED=""
if [ "${RETENTION_DAYS}" -gt 0 ]; then
    echo "Pruning backups older than ${RETENTION_DAYS} days..."
    prune_root="${BACKUP_ROOT%/}"
    if old_list=$(mktemp) \
        && find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -mtime "+${RETENTION_DAYS}" -print0 > "${old_list}"; then
        while IFS= read -r -d '' old; do
            case "${old}" in
                "${prune_root}"/*/*) continue ;;
                "${prune_root}"/?*) ;;
                *) continue ;;
            esac
            if [ "${old}" = "${prune_root}/${TIMESTAMP}" ]; then
                continue
            fi
            if ! rm -rf -- "${old}"; then
                echo "ERROR: retention could not remove ${old}; it may be partly removed, remove it by hand" >&2
                RETENTION_FAILED=yes
            fi
        done < "${old_list}"
    else
        echo "ERROR: retention could not list ${BACKUP_ROOT}; old backups were not pruned" >&2
        RETENTION_FAILED=yes
    fi
    if [ -n "${old_list:-}" ]; then
        rm -f "${old_list}"
    fi
fi

# 7. Off-box copy. A backup on the same disk does not survive the failure it is
#    meant to protect against — which DEPLOY.md itself calls likely.
if [ -n "${REMOTE_TARGET:-}" ]; then
    echo "Copying to ${REMOTE_TARGET}..."
    run_rsync "the off-site copy" -a --delete "${BACKUP_ROOT}/" "${REMOTE_TARGET}/"
fi

echo "=== Backup complete: ${BACKUP_DIR} ==="
du -sh "${BACKUP_DIR}"

# A retention failure ends the night with status 40, a status of its own:
# rsync's statuses end at 35, and the script already ends with rsync's own
# status elsewhere (24, 11, 255, ...). The backup itself is whole, its MANIFEST
# exists, so the trap removes nothing. A failed off-site copy ends the night
# before this line with its own status; the retention ERROR line is in the
# journal either way.
if [ -n "${RETENTION_FAILED}" ]; then
    echo "ERROR: the backup is complete, but retention failed (see above); ending with status 40" >&2
    exit 40
fi
