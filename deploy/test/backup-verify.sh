#!/usr/bin/env bash
# Phase 32: BAK-01, BAK-02 and BAK-03 played in one fresh systemd-base container.
#
# A narrow sibling of deploy/test/run.sh and deploy/test/nightly.sh in the shape
# of deploy/test/native-amd64.sh. It edits neither and reaches run.sh through its
# public subcommands only (build up install postinstall seed backup-fixture); what
# this file adds is the scenarios of phase 32, the files they write and the rule
# that reads a verdict from those files, never from a trace (D-03).
#
#   backup-verify.sh selftest   the pure functions below, a table of cases, no Docker
#   backup-verify.sh run        the evidence: one container, the stages of STAGES, the
#                               raw files below deploy/test/out/amd64-t32-src
#   backup-verify.sh down       remove the container holzcloud-t32, and nothing else
#   backup-verify.sh appendix   the appendix blocks of the fixed paste list, from the files below E
#   backup-verify.sh record-check   the proof of deploy/BACKUP-VERIFY-RUN.md (R) against the
#                               scratch root (E): every block byte for byte, every cited file
#                               pasted, one Evidence commit, pinned and unchanged since
#
# R and E default to deploy/BACKUP-VERIFY-RUN.md and deploy/test/out.
#
# Subcommands can be chained in one call and run in order.
#
# The bound files of the evidence set (BOUND_FILES below): deploy/backup.sh,
# deploy/restore.sh, deploy/DEPLOY.md, deploy/holzcloud-backup.service,
# deploy/holzcloud-backup.timer, deploy/test/run.sh, deploy/test/Dockerfile and
# this file, together with the binary built from the evidence commit. `run`
# refuses to start while one of them differs from HEAD or is untracked, and pins
# NEW_REF to the short sha of HEAD, so the binary and the scripts are one commit.
#
# Stages (STAGES, default "bak01 bak02 bak03"; a subset is a rehearsal, and the
# stage left out reads NOT-RUN, so a subset can never read all PASS):
#   bak01   the document's manual backup line; SHA256SUMS against the directory and
#           sha256sum -c in the container; four damaged copies below /root/t32 (a
#           flipped media byte, a flipped byte inside a database page that still
#           passes `holzcloud check`, a deleted listed file, no SHA256SUMS), each
#           refused with the live service and data unchanged; a backup made by
#           the backup.sh of v0.1.4 and the untouched new one, both restored
#   bak02   an original and a variant that rows name are removed before a backup;
#           restore.sh names both before its question and at the end, status 3
#   bak03   the shipped holzcloud-backup.service, no drop-in, meets an old backup
#           it cannot remove: the journal names it, the unit ends with status 40
#
# Environment:
#   PLATFORM    must be linux/amd64 (on this arm64 Mac it runs under emulation, D-08)
#   TARGET      must be systemd-base
#   STAGES      see above
#   BULK_MB     100, megabytes of synthetic media run.sh backup-fixture writes
#   REHEARSAL   1 lets `run` start while a bound file differs from HEAD; summary.txt
#               then says REHEARSAL, and such a set is never evidence
#
# Names fixed for the phase: container holzcloud-t32; evidence directory
# deploy/test/out/amd64-t32-src; run files deploy/test/out/amd64-t32-evidence-commit.txt
# and deploy/test/out/amd64-t32-pass1.{started,finished,status,log}.
#
# Exit codes: 0 ok, 1 failure (a verdict that is not PASS included), 2 usage,
# 3 the platform cannot give the evidence (no Docker daemon).
#
# Written for bash 3.2 (macOS): no arrays, no mapfile, no case conversion. GNU-only
# work runs inside the container through docker exec.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

PLATFORM=${PLATFORM:-}
TARGET=${TARGET:-}
STAGES=${STAGES:-bak01 bak02 bak03}
BULK_MB=${BULK_MB:-100}
REHEARSAL=${REHEARSAL:-}

T32_NAME=holzcloud-t32
BASE_OUT=deploy/test/out
T32_OUT=$BASE_OUT/amd64-t32-src
RUN_PREFIX=$BASE_OUT/amd64-t32-pass1
EVIDENCE_FILE=$BASE_OUT/amd64-t32-evidence-commit.txt
BOUND_FILES="deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/Dockerfile deploy/test/backup-verify.sh"
DEPLOY_DOC=deploy/DEPLOY.md
# The document's manual backup line (DEPLOY.md, Backups), played by exact text.
MANUAL_LINE='sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups'

say() { printf '%s\n' "$*" >&2; }
die() {
  say "backup-verify.sh: $*"
  exit 1
}
usage() {
  awk 'NR > 1 && /^$/ { exit } NR > 1 { print }' "${BASH_SOURCE[0]}" >&2
  exit 2
}

# ---------------------------------------------------------------------------
# Pure functions. No Docker, no clock, no network: arguments or files in, one
# answer out. `selftest` holds a table of cases, the failing ones included.
# ---------------------------------------------------------------------------

# safe_name NAME: success for the one container this file makes, holzcloud-t32.
safe_name() {
  [ "${1:-}" = holzcloud-t32 ]
}

# safe_out PATH: success for a path directly below deploy/test/out whose name
# begins with amd64-t32- and has something after it; no slash, no parent step.
safe_out() {
  local p=${1:-} rest
  case "$p" in
    deploy/test/out/amd64-t32-?*) ;;
    *) return 1 ;;
  esac
  rest=${p#deploy/test/out/}
  case "$rest" in
    */* | *..*) return 1 ;;
  esac
  return 0
}

# sums_equal_find SUMS FIND: yes when the path column of the SHA256SUMS file SUMS
# names exactly the files of the list FIND (one ./path per line), in any order; no
# otherwise, and no for a line of SUMS that is not "<64 hex>  <path>", for an
# absent SUMS and for two empty lists.
sums_equal_find() {
  local s=${1:-} f=${2:-} a b
  if [ ! -f "$s" ] || [ ! -f "$f" ]; then
    echo no
    return 0
  fi
  if grep -Evq '^[0-9a-f]{64}  \./' "$s"; then
    echo no
    return 0
  fi
  a=$(sed 's/^[0-9a-f]\{64\}  //' "$s" | LC_ALL=C sort)
  b=$(LC_ALL=C sort "$f")
  if [ -n "$a" ] && [ "$a" = "$b" ]; then
    echo yes
  else
    echo no
  fi
}

# kv FILE KEY: the value of the one line "KEY: value"; empty when absent. kv_count
# FILE KEY: how many such lines there are.
kv() { sed -n "s/^$2: //p" "$1" 2> /dev/null | head -n 1; }
kv_count() { grep -c "^$2: " "$1" 2> /dev/null || true; }

# is_count TEXT: success for a decimal number without a sign.
is_count() {
  case "${1:-}" in
    "" | *[!0-9]*) return 1 ;;
  esac
  return 0
}

# kv_is FILE KEY VALUE: the key occurs exactly once and has exactly that value.
kv_is() {
  [ "$(kv_count "$1" "$2")" = 1 ] && [ "$(kv "$1" "$2")" = "$3" ]
}
# kv_positive FILE KEY: the key occurs exactly once and is a number above zero.
kv_positive() {
  local v
  [ "$(kv_count "$1" "$2")" = 1 ] || return 1
  v=$(kv "$1" "$2")
  is_count "$v" && [ "$v" -gt 0 ]
}

# verdict_bak01 FILE: PASS, FAIL or NOT-RUN (no file) for the key lines of bak01.txt.
verdict_bak01() {
  local f=${1:-}
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if kv_is "$f" MANUAL-RC 0 && kv_positive "$f" SUMS-LINES && kv_is "$f" SUMS-EQUALS-FIND yes \
    && kv_is "$f" SUMS-CHECK-RC 0 && kv_is "$f" SUMS-HAS-SQLITE yes && kv_positive "$f" SUMS-MEDIA-LINES \
    && kv_is "$f" MANIFEST-CHECKSUMS-LINE yes \
    && kv_media "$f" FLIP-MEDIA-FILE && kv_is "$f" FLIP-MEDIA-RC 1 \
    && kv_is "$f" FLIP-MEDIA-NAMED "$(kv "$f" FLIP-MEDIA-FILE)" && kv_is "$f" INVARIANTS-AFTER-FLIP-MEDIA same \
    && kv_is "$f" BITROT-CHECK "integrity: ok" && kv_is "$f" BITROT-RESTORE-RC 1 \
    && kv_is "$f" BITROT-NAMED ./holzcloud.sqlite && kv_is "$f" INVARIANTS-AFTER-BITROT same \
    && kv_media "$f" DELETED-FILE && kv_is "$f" DELETED-RC 1 \
    && kv_is "$f" DELETED-NAMED "$(kv "$f" DELETED-FILE)" && kv_is "$f" INVARIANTS-AFTER-DELETED same \
    && kv_is "$f" NOSUMS-RC 1 && kv_is "$f" NOSUMS-NAMED yes && kv_is "$f" INVARIANTS-AFTER-NOSUMS same \
    && [ "$(kv_count "$f" OLD-BACKUP-SCRIPT)" = 1 ] \
    && printf '%s\n' "$(kv "$f" OLD-BACKUP-SCRIPT)" | grep -Eqx 'v0\.1\.4 sha256 [0-9a-f]{64}' \
    && kv_is "$f" OLD-BACKUP-RC 0 && kv_is "$f" OLD-RESTORE-RC 0 && kv_is "$f" OLD-NOTICE yes \
    && kv_is "$f" NEW-RESTORE-RC 0 \
    && kv_is "$f" NEW-VERIFIED-LINE "Checksums: $(kv "$f" SUMS-LINES) files verified"; then
    echo PASS
  else
    echo FAIL
  fi
}

# kv_media FILE KEY: the key occurs once and names a file below ./media/.
kv_media() {
  [ "$(kv_count "$1" "$2")" = 1 ] || return 1
  case "$(kv "$1" "$2")" in
    ./media/?*) return 0 ;;
  esac
  return 1
}

# verdict_bak02 FILE: PASS when an original and a variant were lost, the backup
# ran, did not list them, and restore.sh named every one of them before its
# question and again at the end, ending with status 3 and a ready service.
verdict_bak02() {
  local f=${1:-} n
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  n=$(grep -c '^LOST: ' "$f" || true)
  if grep -q '^LOST: media row ' "$f" && grep -q '^LOST: media_variants row ' "$f" \
    && kv_is "$f" BACKUP-RC 0 && kv_is "$f" SUMS-LISTS-LOST no && kv_is "$f" RESTORE-RC 3 \
    && kv_is "$f" MISSING-BEFORE-QUESTION "$n" && kv_is "$f" MISSING-AT-END "$n" && kv_is "$f" READYZ ok; then
    echo PASS
  else
    echo FAIL
  fi
}

# verdict_bak03 FILE: PASS when the shipped unit, without a drop-in, failed the
# night with status 40, named the directory, kept the night and pruned the rest.
verdict_bak03() {
  local f=${1:-}
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if kv_is "$f" UNIT-DROPINS "''" && kv_is "$f" RESULT exit-code && kv_is "$f" EXEC-MAIN-STATUS 40 \
    && kv_is "$f" JOURNAL-NAMES-DIR yes && kv_is "$f" LOCKED-STILL-THERE yes && kv_is "$f" REMOVABLE-GONE yes \
    && kv_is "$f" NIGHT-HAS-MANIFEST yes; then
    echo PASS
  else
    echo FAIL
  fi
}

# invariants_same A B: same when both files hold an inode, an invocation, the
# active state and a tree hash, and are byte for byte equal; changed otherwise.
invariants_same() {
  local a=${1:-} b=${2:-}
  if [ -s "$a" ] && [ -s "$b" ] && cmp -s "$a" "$b" \
    && grep -Eq '^inode [0-9]+$' "$a" && grep -Eq '^invocation [0-9a-f]+$' "$a" \
    && grep -Eq '^active [a-z]+$' "$a" && grep -Eq '^tree [0-9a-f]{64}$' "$a"; then
    echo same
  else
    echo changed
  fi
}

# stage_wanted NAME: NAME is one of the words of STAGES.
stage_wanted() {
  case " $STAGES " in
    *" $1 "*) return 0 ;;
  esac
  return 1
}
# stages_valid: STAGES is not empty and names only bak01, bak02 and bak03.
stages_valid() {
  local s n=0
  for s in $STAGES; do
    case "$s" in
      bak01 | bak02 | bak03) n=$((n + 1)) ;;
      *) return 1 ;;
    esac
  done
  [ "$n" -gt 0 ]
}

# ---------------------------------------------------------------------------
# The container. Every docker call names T32_NAME, which safe_name has accepted.
# ---------------------------------------------------------------------------

dx() { docker exec "$T32_NAME" "$@"; }
dxi() { docker exec -i "$T32_NAME" "$@"; }

# run_sh SUBCOMMAND...: run.sh through its public subcommands, on this container
# and this evidence directory only.
run_sh() {
  env PLATFORM="$PLATFORM" TARGET=systemd-base NEW_REF="$NEW_REF" NAME="$T32_NAME" OUT_DIR="$T32_OUT" \
    KEEP=1 BULK_MB="$BULK_MB" bash "$ROOT/deploy/test/run.sh" "$@"
}

# deviation TEXT: one line of deviations.txt (run.sh writes its own there too).
deviation() {
  say "$*"
  printf '%s\n' "$*" >> "$T32_OUT/deviations.txt"
}

# put FILE KEY VALUE: one key line, also on stderr for the log.
put() {
  printf '%s: %s\n' "$2" "$3" >> "$1"
  say "$2: $3"
}

# cwall FILE: seconds between the two container clock readings of FILE (two lines).
cwall() { awk 'NR == 1 { a = $1 } NR == 2 { b = $1 } END { if (a != "" && b != "") printf "%.1f\n", b - a; else print "?" }' "$1"; }

# bound_check: the bound files are tracked and equal to HEAD. Prints one line per
# problem and fails when there is one.
bound_check() {
  local f bad=0
  for f in $BOUND_FILES; do
    if ! git ls-files --error-unmatch -- "$f" > /dev/null 2>&1; then
      say "backup-verify.sh: bound file $f is not tracked"
      bad=1
    elif ! git diff --quiet HEAD -- "$f"; then
      say "backup-verify.sh: bound file $f differs from HEAD"
      bad=1
    fi
  done
  [ "$bad" -eq 0 ]
}

# stage_bak01: the document's manual backup line, SHA256SUMS against the directory.
stage_bak01() {
  local f=$T32_OUT/bak01.txt before after dir rc lines media
  : > "$f"
  grep -Fxq -- "$MANUAL_LINE" "$DEPLOY_DOC" || die "the document and this file have drifted apart: $DEPLOY_DOC has no line '$MANUAL_LINE'"
  before=$(dx bash -c 'ls -1 /opt/holzcloud/backups' | tr -d '\r') || before=""
  dx date +%s.%N > "$T32_OUT/bak01-manual.wall"
  rc=0
  dx bash -c "$MANUAL_LINE" > "$T32_OUT/bak01-manual.out" 2>&1 || rc=$?
  dx date +%s.%N >> "$T32_OUT/bak01-manual.wall"
  put "$f" MANUAL-RC "$rc"
  after=$(dx bash -c 'ls -1 /opt/holzcloud/backups' | tr -d '\r') || after=""
  dir=$(printf '%s\n' "$after" | grep -Fxv -- "$before" | grep -E '^[0-9]{8}-[0-9]{6}$' | tail -n 1) || dir=""
  if [ -z "$before" ]; then
    dir=$(printf '%s\n' "$after" | grep -E '^[0-9]{8}-[0-9]{6}$' | tail -n 1) || dir=""
  fi
  [ -n "$dir" ] || die "bak01: the manual backup left no new directory below /opt/holzcloud/backups"
  BAK01_DIR=/opt/holzcloud/backups/$dir
  printf '%s\n' "$BAK01_DIR" > "$T32_OUT/bak01-dir.lst"
  say "bak01: the manual backup is $BAK01_DIR"

  dx cat "$BAK01_DIR/SHA256SUMS" > "$T32_OUT/bak01-sums.lst" 2> /dev/null || : > "$T32_OUT/bak01-sums.lst"
  dx bash -c 'cd "$1" && find . -type f ! -path ./SHA256SUMS ! -path ./MANIFEST | LC_ALL=C sort' _ "$BAK01_DIR" > "$T32_OUT/bak01-find.lst"
  lines=$(wc -l < "$T32_OUT/bak01-sums.lst" | tr -d ' ')
  put "$f" SUMS-LINES "$lines"
  put "$f" SUMS-EQUALS-FIND "$(sums_equal_find "$T32_OUT/bak01-sums.lst" "$T32_OUT/bak01-find.lst")"
  dx date +%s.%N > "$T32_OUT/bak01-sumcheck.wall"
  rc=0
  dx bash -c 'cd "$1" && sha256sum -c --quiet SHA256SUMS' _ "$BAK01_DIR" > /dev/null 2>&1 || rc=$?
  dx date +%s.%N >> "$T32_OUT/bak01-sumcheck.wall"
  put "$f" SUMS-CHECK-RC "$rc"
  if grep -Eq '^[0-9a-f]{64}  \./holzcloud\.sqlite$' "$T32_OUT/bak01-sums.lst"; then
    put "$f" SUMS-HAS-SQLITE yes
  else
    put "$f" SUMS-HAS-SQLITE no
  fi
  media=$(grep -Ec '^[0-9a-f]{64}  \./media/' "$T32_OUT/bak01-sums.lst" || true)
  put "$f" SUMS-MEDIA-LINES "${media:-0}"
  if dx grep -qx 'checksums=SHA256SUMS' "$BAK01_DIR/MANIFEST"; then
    put "$f" MANIFEST-CHECKSUMS-LINE yes
  else
    put "$f" MANIFEST-CHECKSUMS-LINE no
  fi
  bak01_refusals "$f"
  bak01_old "$f"
  bak01_new "$f"
  printf 'BAK-01 %s\n' "$(verdict_bak01 "$f")" >> "$f"
}

# Run INSIDE the container. STAMP_SH: $1 is the line to play, stdin the answer.
# Every output line is prefixed with the container's clock (run.sh's
# RESTORE_RUN_SH technique), a last line "WALL t0 t1", and the exit status is the
# line's own (PIPESTATUS), not that of the loop.
STAMP_SH='set -o pipefail
t0=$(date +%s.%N)
bash -c "$1" 2>&1 | while IFS= read -r l; do printf "%s %s\n" "$(date +%s.%N)" "$l"; done
rc=${PIPESTATUS[0]}
t1=$(date +%s.%N)
echo "WALL $t0 $t1"
exit "$rc"'

# Run INSIDE the container. FLIP_SH FILE OFFSET: the byte at OFFSET becomes the
# next byte value (mod 256), written in place.
FLIP_SH='f=$1; off=$2
b=$(od -An -tu1 -N1 -j "$off" "$f" | tr -d " ")
[ -n "$b" ] || exit 1
nb=$(( (b + 1) % 256 ))
printf "\\$(printf %03o "$nb")" | dd of="$f" bs=1 seek="$off" count=1 conv=notrunc status=none'

# Run INSIDE the container. INV_SH: the invariants of the live installation that a
# refused restore must not touch (run.sh's restore-negative takes the same):
# the inode of the database, the service's invocation and state, and the tree
# hash of the data directory without the holzcloud.sqlite family.
INV_SH='d=/opt/holzcloud/data
echo "inode $(stat -c %i "$d/holzcloud.sqlite")"
echo "invocation $(systemctl show -p InvocationID --value holzcloud)"
echo "active $(systemctl is-active holzcloud)"
echo "tree $(cd "$d" && find . -type f ! -name "holzcloud.sqlite*" -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | cut -d" " -f1)"'

# restore_line DIR: the document's restore line with DIR in place of its example.
restore_line() {
  local l
  l=$(grep -E '^sudo /opt/holzcloud/deploy/restore\.sh /opt/holzcloud/backups/[0-9]{8}-[0-9]{6}$' "$DEPLOY_DOC" | head -n 1) || l=""
  [ -n "$l" ] || die "the document and this file have drifted apart: $DEPLOY_DOC has no restore line of the shape 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  printf '%s %s\n' "${l% *}" "$1"
}

# play_restore OUT DIR: the restore line for DIR, `yes` on stdin, every line
# stamped into OUT; returns restore.sh's status. Preceded by reset-failed
# (DEVIATION[start-limit]): several restores in a row restart the service more
# often than its start limit allows, which a server never meets.
play_restore() {
  local out=$1 line rc=0
  line=$(restore_line "$2")
  dx systemctl reset-failed holzcloud > /dev/null 2>&1 || true
  say "restore: $line"
  printf 'yes\n' | docker exec -i "$T32_NAME" bash -c "$STAMP_SH" _ "$line" > "$out" 2>&1 || rc=$?
  return "$rc"
}

# unstamped FILE: the lines of a stamped file without the clock.
unstamped() { sed 's/^[0-9][0-9.]* //' "$1"; }

# inv_take FILE: the invariants, now.
inv_take() { dx bash -c "$INV_SH" | tr -d '\r' > "$1"; }

# copy_case NAME: a copy of the manual backup below /root/t32, never inside
# /opt/holzcloud/backups; its path on stdout.
copy_case() {
  dx bash -c 'mkdir -p /root/t32 && rm -rf "/root/t32/$1" && cp -a "$2" "/root/t32/$1"' _ "$1" "$BAK01_DIR" \
    || die "could not copy $BAK01_DIR to /root/t32/$1"
  printf '/root/t32/%s\n' "$1"
}

# refusal FILE CASE OUT COPY RCKEY: restore the damaged copy with the invariants
# taken before and after; RCKEY and INVARIANTS-AFTER-CASE go to FILE.
refusal() {
  local f=$1 key=$2 out=$3 copy=$4 rc=0 b a
  b=$T32_OUT/inv-$key-before.lst
  a=$T32_OUT/inv-$key-after.lst
  inv_take "$b"
  play_restore "$out" "$copy" || rc=$?
  inv_take "$a"
  put "$f" "$5" "$rc"
  put "$f" "INVARIANTS-AFTER-$key" "$(invariants_same "$b" "$a")"
  dx rm -rf "$copy"
}

# named_differs OUT: the path of the first "differs from its checksum" line.
named_differs() { unstamped "$1" | sed -n 's/^ERROR: \(.*\) differs from its checksum in SHA256SUMS .*/\1/p' | head -n 1; }
named_missing() { unstamped "$1" | sed -n 's/^ERROR: \(.*\) is listed in SHA256SUMS and missing .*/\1/p' | head -n 1; }

# bak01_refusals: four damaged copies, each refused before anything is stopped.
bak01_refusals() {
  local f=$1 copy rel size offs off line got
  # FLIP-MEDIA: one byte of the first ./media/ file in SHA256SUMS order.
  rel=$(grep -E '^[0-9a-f]{64}  \./media/' "$T32_OUT/bak01-sums.lst" | head -n 1 | sed 's/^[0-9a-f]\{64\}  //') || rel=""
  [ -n "$rel" ] || die "bak01: SHA256SUMS lists no media file"
  copy=$(copy_case flip-media)
  size=$(dx stat -c %s "$copy/$rel" | tr -d '\r')
  dx bash -c "$FLIP_SH" _ "$copy/$rel" "$((size / 2))" || die "bak01: could not flip a byte of $copy/$rel"
  put "$f" FLIP-MEDIA-FILE "$rel"
  refusal "$f" FLIP-MEDIA "$T32_OUT/bak01-flip-media.out" "$copy" FLIP-MEDIA-RC
  put "$f" FLIP-MEDIA-NAMED "$(named_differs "$T32_OUT/bak01-flip-media.out")"

  # BITROT: one character of a text the seed wrote ("Plugintest page", the first
  # line of the page's markdown), inside a data page, so that the integrity check
  # still passes; the first occurrence whose flip leaves `integrity: ok` is used.
  copy=$(copy_case bitrot)
  offs=$(dx bash -c 'grep -obUa "Plugintest page" "$1" | cut -d: -f1 | head -n 8' _ "$copy/holzcloud.sqlite" | tr -d '\r') || offs=""
  [ -n "$offs" ] || die "bak01: the snapshot holds no 'Plugintest page'"
  got=""
  : > "$T32_OUT/bak01-bitrot-check.out"
  for off in $offs; do
    dx bash -c 'rm -rf /root/t32/bitrot-check && mkdir -p /root/t32/bitrot-check && cp "$1" /root/t32/bitrot-check/holzcloud.sqlite' _ "$copy/holzcloud.sqlite"
    dx bash -c "$FLIP_SH" _ /root/t32/bitrot-check/holzcloud.sqlite "$off" || die "bak01: could not flip a byte of the check copy"
    line=$(dx bash -c 'HOLZCLOUD_DATA_DIR=/root/t32/bitrot-check /opt/holzcloud/holzcloud check 2>&1; echo "status $?"' | tr -d '\r') || line=""
    printf 'offset %s\n%s\n' "$off" "$line" >> "$T32_OUT/bak01-bitrot-check.out"
    got=$(printf '%s\n' "$line" | grep '^integrity: ' | head -n 1) || got=""
    if [ "$got" = "integrity: ok" ]; then
      dx bash -c "$FLIP_SH" _ "$copy/holzcloud.sqlite" "$off" || die "bak01: could not flip a byte of $copy/holzcloud.sqlite"
      put "$f" BITROT-OFFSET "$off"
      break
    fi
  done
  dx rm -rf /root/t32/bitrot-check
  put "$f" BITROT-CHECK "${got:-?}"
  refusal "$f" BITROT "$T32_OUT/bak01-bitrot.out" "$copy" BITROT-RESTORE-RC
  put "$f" BITROT-NAMED "$(named_differs "$T32_OUT/bak01-bitrot.out")"

  # DELETED: the last ./media/ file of SHA256SUMS removed from the copy.
  rel=$(grep -E '^[0-9a-f]{64}  \./media/' "$T32_OUT/bak01-sums.lst" | tail -n 1 | sed 's/^[0-9a-f]\{64\}  //')
  copy=$(copy_case deleted)
  dx rm -f "$copy/$rel" || die "bak01: could not remove $copy/$rel"
  put "$f" DELETED-FILE "$rel"
  refusal "$f" DELETED "$T32_OUT/bak01-deleted.out" "$copy" DELETED-RC
  put "$f" DELETED-NAMED "$(named_missing "$T32_OUT/bak01-deleted.out")"

  # NOSUMS: SHA256SUMS removed, the MANIFEST that names it kept.
  copy=$(copy_case nosums)
  dx rm -f "$copy/SHA256SUMS" || die "bak01: could not remove $copy/SHA256SUMS"
  refusal "$f" NOSUMS "$T32_OUT/bak01-nosums.out" "$copy" NOSUMS-RC
  if unstamped "$T32_OUT/bak01-nosums.out" | grep -q '^ERROR: .*/MANIFEST names SHA256SUMS but the file is missing'; then
    put "$f" NOSUMS-NAMED yes
  else
    put "$f" NOSUMS-NAMED no
  fi
}

# bak01_old: a backup made by the backup.sh of v0.1.4, restored by this release.
bak01_old() {
  local f=$1 rc=0 sha dir
  git show v0.1.4:deploy/backup.sh > "$T32_OUT/v014-backup.sh" || die "git show v0.1.4:deploy/backup.sh failed"
  dx mkdir -p /opt/holzcloud/deploy-v014
  docker cp "$T32_OUT/v014-backup.sh" "$T32_NAME:/opt/holzcloud/deploy-v014/backup.sh" > /dev/null || die "docker cp of the v0.1.4 backup.sh failed"
  dx bash -c 'chown root:root /opt/holzcloud/deploy-v014/backup.sh && chmod 755 /opt/holzcloud/deploy-v014/backup.sh
install -d -o holzcloud -g holzcloud -m 700 /opt/holzcloud/backups-v014' || die "could not prepare the v0.1.4 script"
  sha=$(dx sha256sum /opt/holzcloud/deploy-v014/backup.sh | cut -d' ' -f1 | tr -d '\r')
  put "$f" OLD-BACKUP-SCRIPT "v0.1.4 sha256 $sha"
  deviation "DEVIATION[old-script]: the backup.sh of v0.1.4 ran with this release's binary (as /opt/holzcloud/deploy-v014/backup.sh, with DATA_DIR and HOLZCLOUD_BIN as the unit sets them, into /opt/holzcloud/backups-v014); a server that kept the old script would run it with the binary of its own release"
  dx bash -c 'sudo -u holzcloud env DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud /opt/holzcloud/deploy-v014/backup.sh /opt/holzcloud/backups-v014' \
    > "$T32_OUT/bak01-old-backup.out" 2>&1 || rc=$?
  put "$f" OLD-BACKUP-RC "$rc"
  dir=$(dx bash -c 'ls -1 /opt/holzcloud/backups-v014 | grep -E "^[0-9]{8}-[0-9]{6}$" | tail -n 1' | tr -d '\r') || dir=""
  [ -n "$dir" ] || die "bak01: the v0.1.4 script left no backup directory"
  rc=0
  play_restore "$T32_OUT/bak01-old-restore.out" "/opt/holzcloud/backups-v014/$dir" || rc=$?
  put "$f" OLD-RESTORE-RC "$rc"
  if unstamped "$T32_OUT/bak01-old-restore.out" | grep -q '^NOTICE: .* has no checksum file (made before 0\.1\.5); its files were not verified$'; then
    put "$f" OLD-NOTICE yes
  else
    put "$f" OLD-NOTICE no
  fi
}

# bak01_new: the untouched manual backup restores with every file verified.
bak01_new() {
  local f=$1 rc=0
  play_restore "$T32_OUT/bak01-new-restore.out" "$BAK01_DIR" || rc=$?
  put "$f" NEW-RESTORE-RC "$rc"
  put "$f" NEW-VERIFIED-LINE "$(unstamped "$T32_OUT/bak01-new-restore.out" | grep -E '^Checksums: [0-9]+ files verified$' | head -n 1)"
}

# new_backup_dir ROOT BEFORE: the timestamp directory of ROOT not in the list BEFORE.
new_backup_dir() {
  local now
  now=$(dx bash -c 'ls -1 "$1"' _ "$1" | tr -d '\r' | grep -E '^[0-9]{8}-[0-9]{6}$') || now=""
  if [ -n "$2" ]; then
    printf '%s\n' "$now" | grep -Fxv -- "$2" | tail -n 1 || true
  else
    printf '%s\n' "$now" | tail -n 1
  fi
}

# stage_bak02: an original and a variant that database rows name are gone before
# the backup; restore.sh names both before its question and again at the end.
stage_bak02() {
  local f=$T32_OUT/bak02.txt row vid label vfile mid wid mfile before dir rc=0 nb na code
  : > "$f"
  dx bash -c 'rm -rf /root/t32/q && mkdir -p /root/t32/q && cp /opt/holzcloud/data/holzcloud.sqlite /root/t32/q/ && if [ -e /opt/holzcloud/data/holzcloud.sqlite-wal ]; then cp /opt/holzcloud/data/holzcloud.sqlite-wal /root/t32/q/; fi' \
    || die "bak02: could not copy the live database"
  row=$(dx sqlite3 -readonly /root/t32/q/holzcloud.sqlite \
    "SELECT v.id, v.label, v.filename, m.id, m.website_id, m.filename FROM media_variants v JOIN media m ON m.id = v.media_id ORDER BY m.id, v.id LIMIT 1" | tr -d '\r') || row=""
  dx rm -rf /root/t32/q
  [ -n "$row" ] || die "bak02: the database has no media row with a variant"
  IFS='|' read -r vid label vfile mid wid mfile <<< "$row"
  dx rm -f "/opt/holzcloud/data/media/$wid/$mfile" "/opt/holzcloud/data/media/$wid/$vfile" || die "bak02: could not remove the two files"
  deviation "DEVIATION[lost-file]: the two files are removed from the live data directory before the backup, standing in for a file deleted between the snapshot and the copy (phase 29 measured that as ROW-WITHOUT-FILE; it is not provoked again here)"
  put "$f" LOST "media row $mid website $wid file $mfile"
  put "$f" LOST "media_variants row $vid label $label file $vfile"
  before=$(dx bash -c 'ls -1 /opt/holzcloud/backups' | tr -d '\r') || before=""
  dx bash -c "$MANUAL_LINE" > "$T32_OUT/bak02-backup.out" 2>&1 || rc=$?
  put "$f" BACKUP-RC "$rc"
  dir=$(new_backup_dir /opt/holzcloud/backups "$before")
  [ -n "$dir" ] || die "bak02: the backup left no new directory"
  if dx grep -Eq "  \./media/$wid/($mfile|$vfile)\$" "/opt/holzcloud/backups/$dir/SHA256SUMS"; then
    put "$f" SUMS-LISTS-LOST yes
  else
    put "$f" SUMS-LISTS-LOST no
  fi
  rc=0
  play_restore "$T32_OUT/bak02-restore.out" "/opt/holzcloud/backups/$dir" || rc=$?
  put "$f" RESTORE-RC "$rc"
  nb=$(unstamped "$T32_OUT/bak02-restore.out" | awk '/^Stopping holzcloud\.\.\.$/ { exit } /^MISSING / { n++ } END { print n + 0 }')
  na=$(unstamped "$T32_OUT/bak02-restore.out" | awk 'seen && /^MISSING / { n++ } /^=== Restore complete ===$/ { seen = 1 } END { print n + 0 }')
  put "$f" MISSING-BEFORE-QUESTION "$nb"
  put "$f" MISSING-AT-END "$na"
  code=$(dx curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/readyz | tr -d '\r') || code="?"
  if [ "$code" = 200 ]; then put "$f" READYZ ok; else put "$f" READYZ "$code"; fi
  printf 'BAK-02 %s\n' "$(verdict_bak02 "$f")" >> "$f"
}

# stage_bak03: the shipped unit, with no drop-in, meets an old backup it cannot remove.
stage_bak03() {
  local f=$T32_OUT/bak03.txt b=/opt/holzcloud/backups cur before dir rc=0 v
  : > "$f"
  dx bash -c 'set -e; b=/opt/holzcloud/backups
mkdir -p "$b/20200101-031500/locked" "$b/20200102-031500"
echo planted > "$b/20200101-031500/MANIFEST"
echo planted > "$b/20200101-031500/locked/keep"
echo planted > "$b/20200102-031500/MANIFEST"
chown holzcloud:holzcloud "$b/20200101-031500" "$b/20200101-031500/MANIFEST"
chown -R root:root "$b/20200101-031500/locked"
chmod 755 "$b/20200101-031500/locked"
chown -R holzcloud:holzcloud "$b/20200102-031500"
touch -d "45 days ago" "$b/20200101-031500" "$b/20200102-031500"' || die "bak03: could not plant the old backups"
  put "$f" PLANTED "$b/20200101-031500 (locked: root-owned)"
  put "$f" PLANTED "$b/20200102-031500"
  v=$(dx systemctl show -p DropInPaths --value holzcloud-backup.service | tr -d '\r')
  put "$f" UNIT-DROPINS "'$v'"
  before=$(dx bash -c 'ls -1 /opt/holzcloud/backups' | tr -d '\r') || before=""
  cur=$(dx journalctl -n 0 --show-cursor --no-pager 2> /dev/null | sed -n 's/^-- cursor: //p' | tr -d '\r') || cur=""
  [ -n "$cur" ] || die "bak03: journalctl gave no cursor"
  dx systemctl start holzcloud-backup.service || rc=$?
  put "$f" START-RC "$rc"
  sleep 2
  dx journalctl -u holzcloud-backup.service --after-cursor "$cur" -o cat --no-pager | tr -d '\r' > "$T32_OUT/bak03-journal.txt" || true
  put "$f" RESULT "$(dx systemctl show -p Result --value holzcloud-backup.service | tr -d '\r')"
  put "$f" EXEC-MAIN-STATUS "$(dx systemctl show -p ExecMainStatus --value holzcloud-backup.service | tr -d '\r')"
  if grep -Fq "ERROR: retention could not remove $b/20200101-031500" "$T32_OUT/bak03-journal.txt"; then
    put "$f" JOURNAL-NAMES-DIR yes
  else
    put "$f" JOURNAL-NAMES-DIR no
  fi
  if dx test -d "$b/20200101-031500"; then put "$f" LOCKED-STILL-THERE yes; else put "$f" LOCKED-STILL-THERE no; fi
  if dx test -e "$b/20200102-031500"; then put "$f" REMOVABLE-GONE no; else put "$f" REMOVABLE-GONE yes; fi
  dir=$(new_backup_dir "$b" "$before")
  if [ -n "$dir" ] && dx test -s "$b/$dir/MANIFEST"; then
    put "$f" NIGHT-HAS-MANIFEST yes
  else
    put "$f" NIGHT-HAS-MANIFEST no
  fi
  dx systemctl reset-failed holzcloud-backup.service > /dev/null 2>&1 || true
  printf 'BAK-03 %s\n' "$(verdict_bak03 "$f")" >> "$f"
}

# write_measurements: size and timings of the manual backup and its checks.
write_measurements() {
  local f=$T32_OUT/measurements.txt step
  {
    echo "BULK-MB: $BULK_MB"
    if [ -n "${BAK01_DIR:-}" ]; then
      echo "BACKUP-MB: $(dx du -sm "$BAK01_DIR" | cut -f1 | tr -d '\r')"
      echo "BACKUP-FILES: $(dx bash -c 'find "$1" -type f | wc -l' _ "$BAK01_DIR" | tr -d '\r ')"
      echo "MANUAL-BACKUP-WALL-S: $(cwall "$T32_OUT/bak01-manual.wall")"
      echo "SHA256SUM-C-WALL-S: $(cwall "$T32_OUT/bak01-sumcheck.wall")"
    fi
    if [ -f "$T32_OUT/bak01-new-restore.out" ]; then
      step=$(awk '/^WALL / { t0 = $2 } / Checksums: [0-9]+ files verified$/ { c = $1 } END { if (t0 != "" && c != "") printf "%.1f\n", c - t0; else print "?" }' "$T32_OUT/bak01-new-restore.out")
      echo "RESTORE-CHECKSUM-STEP-S: $step"
    fi
  } > "$f"
  cat "$f" >&2
}

# write_versions: run.sh's versions.txt (written by `up`), then the tools of this set.
write_versions() {
  local f=$T32_OUT/versions.txt
  {
    echo "host uname -m:         $(uname -m)"
    echo "container uname -m:    $(dx uname -m | tr -d '\r')"
    echo "container sha256sum:   $(dx sha256sum --version | head -n 1 | tr -d '\r')"
    echo "container bash:        $(dx bash --version | head -n 1 | tr -d '\r')"
    echo "container systemctl:   $(dx systemctl --version | head -n 1 | tr -d '\r')"
    echo "binary:                $(dx /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r')"
    echo "NEW_REF:               $NEW_REF"
    echo "evidence commit:       $(cat "$EVIDENCE_FILE")"
  } >> "$f"
}

# write_deviations: the deviations this file adds to those run.sh wrote itself.
write_deviations() {
  local arch
  arch=$(docker info --format '{{.Architecture}}')
  case "$arch" in
    x86_64 | amd64) ;;
    *) deviation "DEVIATION[emulation]: the Docker host is $arch; the linux/amd64 container runs under emulation (D-08), so every wall time of this set is an emulation figure, not a server's" ;;
  esac
  deviation "DEVIATION[fixture]: the five plugin archives are copied into /opt/holzcloud/plugin-zips by this driver before run.sh seed, the step run.sh's baseline takes for an old install; they stand in for operator work in the admin and are no step of the document"
}

# stage_fixtures: the plugin archives run.sh seed installs (run.sh stages them only
# inside `baseline`, which needs the old binary).
stage_fixtures() {
  local id
  dx mkdir -p /opt/holzcloud/plugin-zips || die "could not create /opt/holzcloud/plugin-zips"
  for id in bestellung jahreszahl kontaktformular nicht-gefunden suche; do
    [ -f "$ROOT/plugins/$id/$id.zip" ] || die "plugins/$id/$id.zip is missing in the working tree"
    docker cp "$ROOT/plugins/$id/$id.zip" "$T32_NAME:/opt/holzcloud/plugin-zips/$id.zip" > /dev/null \
      || die "docker cp of plugins/$id/$id.zip failed"
  done
}

# write_summary: the verdicts from the files the stages wrote.
write_summary() {
  local f=$T32_OUT/summary.txt kinds bin
  kinds=$(sed -n 's/^DEVIATION\[\([a-z0-9-]*\)\].*/\1/p' "$T32_OUT/deviations.txt" 2> /dev/null | sort -u | tr '\n' ' ' | sed 's/ $//')
  bin=$(dx /opt/holzcloud/holzcloud version 2> /dev/null | head -n 1 | tr -d '\r') || bin="?"
  {
    echo "---- summary ($PLATFORM) ----"
    echo "BAK-01 $(verdict_bak01 "$T32_OUT/bak01.txt")"
    echo "BAK-02 $(verdict_bak02 "$T32_OUT/bak02.txt")"
    echo "BAK-03 $(verdict_bak03 "$T32_OUT/bak03.txt")"
    echo "STAGES: $STAGES"
    echo "DEVIATIONS: ${kinds:-none}"
    echo "BINARY: $bin"
    if [ -n "$REHEARSAL" ]; then
      echo "REHEARSAL: yes (bound files were allowed to differ from HEAD; this set is not evidence)"
    fi
  } > "$f"
  cat "$f" >&2
}

# run_body: everything `run` does after its guards; its output goes to the log.
run_body() {
  say "backup-verify: platform $PLATFORM, NEW_REF pinned to $NEW_REF, stages: $STAGES, BULK_MB=$BULK_MB"
  run_sh build up install postinstall
  stage_fixtures
  run_sh seed backup-fixture
  write_versions
  write_deviations
  if stage_wanted bak01 || stage_wanted bak02; then
    deviation "DEVIATION[start-limit]: systemctl reset-failed holzcloud runs before each restore; several restores in a row restart the service more often than its start limit allows, which one restore on a server never meets"
  fi
  if stage_wanted bak01; then stage_bak01; fi
  if stage_wanted bak02; then stage_bak02; fi
  if stage_wanted bak03; then stage_bak03; fi
  write_measurements
  write_summary
  grep -Eq '^BAK-0[0-9] (FAIL|NOT-RUN)$' "$T32_OUT/summary.txt" && return 1
  return 0
}

cmd_run() {
  local rc
  stages_valid || {
    say "backup-verify.sh: STAGES '$STAGES' must name bak01, bak02 or bak03"
    exit 2
  }
  [ "$PLATFORM" = linux/amd64 ] || {
    say "backup-verify.sh: PLATFORM must be linux/amd64 (got '$PLATFORM')"
    exit 2
  }
  [ "$TARGET" = systemd-base ] || {
    say "backup-verify.sh: TARGET must be systemd-base (got '$TARGET')"
    exit 2
  }
  safe_name "$T32_NAME" || die "refusing container name $T32_NAME"
  for p in "$T32_OUT" "$RUN_PREFIX.log" "$EVIDENCE_FILE"; do
    safe_out "$p" || die "refusing path $p"
  done
  if ! bound_check; then
    if [ -n "$REHEARSAL" ]; then
      say "backup-verify.sh: REHEARSAL=1: going on although a bound file differs from HEAD; this set is not evidence"
    else
      die "a bound file differs from HEAD or is untracked; commit first (the evidence is bound to one commit)"
    fi
  fi
  command -v docker > /dev/null 2>&1 || {
    say "backup-verify.sh: docker is not installed"
    exit 3
  }
  docker info > /dev/null 2>&1 || {
    say "backup-verify.sh: the Docker daemon does not answer"
    exit 3
  }
  if docker container inspect "$T32_NAME" > /dev/null 2>&1; then
    die "container $T32_NAME exists; run 'backup-verify.sh down' first (a stale one is never reused)"
  fi
  NEW_REF=$(git rev-parse --short HEAD) || die "HEAD is not a commit"
  mkdir -p "$BASE_OUT"
  rm -f "$RUN_PREFIX.started" "$RUN_PREFIX.finished" "$RUN_PREFIX.status" "$RUN_PREFIX.log"
  printf '%s\n' "$NEW_REF" > "$EVIDENCE_FILE"
  date -u +%Y-%m-%dT%H:%M:%SZ > "$RUN_PREFIX.started"
  set +e
  (set -e; run_body) 2>&1 | tee "$RUN_PREFIX.log"
  rc=${PIPESTATUS[0]}
  set -e
  date -u +%Y-%m-%dT%H:%M:%SZ > "$RUN_PREFIX.finished"
  printf '%s\n' "$rc" > "$RUN_PREFIX.status"
  say "backup-verify: run ended with status $rc"
  exit "$rc"
}

cmd_down() {
  safe_name "$T32_NAME" || die "refusing container name $T32_NAME"
  command -v docker > /dev/null 2>&1 || die "docker is not installed"
  docker rm -f "$T32_NAME" > /dev/null 2>&1 || true
  say "backup-verify: container $T32_NAME removed"
}

# ---------------------------------------------------------------------------
# The record (plan 32-06 runs these; they are here so that the driver bound by
# the evidence is the one that checks it). They never rely on `set -e`, print
# one line per problem and end on an answer line.
# ---------------------------------------------------------------------------

# t32_paste_list: DIRECTORY/FILE (or FILE, for the run files) of the appendix, in
# order. A list, never a glob: step-NN.txt, seed.txt, the .lst and .wall scratch
# files, bak02-backup.out, v014-backup.sh and the log are not on it.
t32_paste_list() {
  printf '%s\n' \
    amd64-t32-src/summary.txt \
    amd64-t32-src/versions.txt \
    amd64-t32-src/deviations.txt \
    amd64-t32-src/bak01.txt \
    amd64-t32-src/bak01-manual.out \
    amd64-t32-src/bak01-flip-media.out \
    amd64-t32-src/bak01-bitrot-check.out \
    amd64-t32-src/bak01-bitrot.out \
    amd64-t32-src/bak01-deleted.out \
    amd64-t32-src/bak01-nosums.out \
    amd64-t32-src/bak01-old-backup.out \
    amd64-t32-src/bak01-old-restore.out \
    amd64-t32-src/bak01-new-restore.out \
    amd64-t32-src/bak02.txt \
    amd64-t32-src/bak02-restore.out \
    amd64-t32-src/bak03.txt \
    amd64-t32-src/bak03-journal.txt \
    amd64-t32-src/measurements.txt \
    amd64-t32-evidence-commit.txt \
    amd64-t32-pass1.status
}

# appendix_print E: the blocks of the paste list from the files below E, each
# "#### NAME", a blank line and a fenced text block. Refuses (prints nothing) when
# a file is missing or holds a line that would end a fence or open a heading.
appendix_print() {
  local e=${1:-deploy/test/out} p bad=0
  for p in $(t32_paste_list); do
    if [ ! -f "$e/$p" ]; then
      say "backup-verify.sh: appendix: no scratch file $e/$p"
      bad=$((bad + 1))
    elif grep -Eq '^(```|#### |## )' "$e/$p"; then
      say "backup-verify.sh: appendix: $e/$p has a line that begins with three backticks, with '#### ' or with '## '; it cannot be pasted unchanged"
      bad=$((bad + 1))
    fi
  done
  if [ "$bad" -ne 0 ]; then
    return 1
  fi
  for p in $(t32_paste_list); do
    printf '#### %s\n\n' "$p"
    printf '%s\n' '```text'
    awk 1 "$e/$p"
    printf '%s\n\n' '```'
  done
}

# t32_record_check R E: the record R against the scratch root E.
#  - every "#### NAME" block under "## Appendix: raw output" equals E/NAME byte for byte,
#    and every file of the paste list has a block;
#  - every `amd64-t32-.../FILE` token in backticks outside the appendix has a block
#    (a file that is not pasted is named as "FILE in DIRECTORY");
#  - exactly one line "Evidence commit: SHA"; SHA resolves, is the pinned commit of
#    E/amd64-t32-evidence-commit.txt, and no bound file nor cmd, internal, go.mod or
#    go.sum differs between SHA and HEAD;
#  - the pasted summary is not a rehearsal's; no line holds a private-key marker.
t32_record_check() {
  local r=${1:-} e=${2:-} n=0 k=0 tmp i name sha pinned p tok bf
  rc_bad() {
    k=$((k + 1))
    printf 'RECORD-CHECK BAD %s\n' "$*"
  }
  if [ ! -f "$r" ]; then
    printf 'RECORD-CHECK BAD no record %s\n' "$r"
    echo "RECORD-CHECK FAILED (1 of 1)"
    return 1
  fi
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/t32-record.XXXXXX") || return 1
  # Split the appendix into numbered block files and a list "INDEX NAME".
  awk -v d="$tmp" '
    $0 == "## Appendix: raw output" { app = 1; next }
    app && /^## / { app = 0 }
    !app { next }
    infence && $0 == "```" { infence = 0; close(f); next }
    infence { print > f; next }
    /^#### / { i++; name = substr($0, 6); print i " " name > (d "/names"); want = 1; next }
    want && $0 == "```text" { infence = 1; want = 0; f = d "/b" i; printf "" > f; next }
  ' "$r"
  : >> "$tmp/names"
  while read -r i name; do
    n=$((n + 1))
    case "$name" in
      /* | *..*) rc_bad "block $name: not a name below the scratch root" ;;
      *)
        if [ ! -f "$tmp/b$i" ]; then
          rc_bad "block $name: no fenced text block after its heading"
        elif [ ! -f "$e/$name" ]; then
          rc_bad "block $name: no scratch file $e/$name"
        elif ! cmp -s "$tmp/b$i" "$e/$name"; then
          rc_bad "block $name: differs from $e/$name"
        fi
        ;;
    esac
  done < "$tmp/names"
  for p in $(t32_paste_list); do
    n=$((n + 1))
    awk -v p="$p" '$2 == p { f = 1 } END { exit !f }' "$tmp/names" || rc_bad "block $p: the paste list has it, the appendix does not"
  done
  # Tokens outside the appendix.
  awk '$0 == "## Appendix: raw output" { exit } { print }' "$r" \
    | grep -o '`amd64-t32-[A-Za-z0-9._-]*/[A-Za-z0-9._/-]*`' | tr -d '`' | sort -u > "$tmp/tokens" || true
  while read -r tok; do
    n=$((n + 1))
    awk -v p="$tok" '$2 == p { f = 1 } END { exit !f }' "$tmp/names" || rc_bad "cited file $tok has no block in the appendix"
  done < "$tmp/tokens"
  # The evidence commit.
  n=$((n + 1))
  if [ "$(grep -c '^Evidence commit: ' "$r" || true)" != 1 ]; then
    rc_bad "the record must hold exactly one line 'Evidence commit: SHA'"
  else
    sha=$(sed -n 's/^Evidence commit: `\{0,1\}\([0-9a-f]\{7,40\}\)`\{0,1\} *$/\1/p' "$r")
    pinned=$(cat "$e/amd64-t32-evidence-commit.txt" 2> /dev/null || true)
    if [ -z "$sha" ] || ! git cat-file -e "$sha^{commit}" 2> /dev/null; then
      rc_bad "Evidence commit '${sha:-?}' does not resolve to a commit"
    elif [ -z "$pinned" ] || [ "$(git rev-parse "$sha^{commit}")" != "$(git rev-parse "$pinned^{commit}" 2> /dev/null || true)" ]; then
      rc_bad "Evidence commit $sha is not the pinned commit '$pinned' of $e/amd64-t32-evidence-commit.txt"
    else
      for bf in $BOUND_FILES; do
        n=$((n + 1))
        git diff --quiet "$sha" HEAD -- "$bf" || rc_bad "bound file $bf differs between $sha and HEAD"
      done
      n=$((n + 1))
      git diff --quiet "$sha" HEAD -- cmd internal go.mod go.sum || rc_bad "cmd, internal, go.mod or go.sum differ between $sha and HEAD (the binary is not the one of the evidence)"
    fi
  fi
  # The summary of a rehearsal is never evidence.
  n=$((n + 1))
  i=$(awk '$2 == "amd64-t32-src/summary.txt" { print $1; exit }' "$tmp/names")
  if [ -n "$i" ] && [ -f "$tmp/b$i" ] && grep -q '^REHEARSAL:' "$tmp/b$i"; then
    rc_bad "the pasted summary is a rehearsal's (REHEARSAL line)"
  fi
  # No secret.
  n=$((n + 1))
  if grep -q 'PRIVATE KEY-----' "$r"; then
    rc_bad "a line holds a private-key marker"
  fi
  rm -rf "${tmp:?}"
  if [ "$k" -ne 0 ]; then
    echo "RECORD-CHECK FAILED ($k of $n)"
    return 1
  fi
  echo "RECORD-CHECK OK ($n checks)"
}

cmd_appendix() {
  local rc=0
  appendix_print "${E:-deploy/test/out}" || rc=$?
  return "$rc"
}
cmd_record_check() {
  local rc=0
  t32_record_check "${R:-deploy/BACKUP-VERIFY-RUN.md}" "${E:-deploy/test/out}" || rc=$?
  return "$rc"
}

# ---------------------------------------------------------------------------
# selftest
# ---------------------------------------------------------------------------

ST_N=0
ST_BAD=0
ST_TMP=""
st_eq() { # NAME EXPECTED ACTUAL
  ST_N=$((ST_N + 1))
  if [ "$2" != "$3" ]; then
    ST_BAD=$((ST_BAD + 1))
    say "SELFTEST BAD $1: expected '$2', got '$3'"
  fi
}
st_ok() { # NAME cmd args...: the command must return 0
  local n=$1
  shift
  ST_N=$((ST_N + 1))
  if ! "$@" > /dev/null 2>&1; then
    ST_BAD=$((ST_BAD + 1))
    say "SELFTEST BAD $n: expected success from $*"
  fi
}
st_fail() { # NAME cmd args...: the command must return non-zero
  local n=$1
  shift
  ST_N=$((ST_N + 1))
  if "$@" > /dev/null 2>&1; then
    ST_BAD=$((ST_BAD + 1))
    say "SELFTEST BAD $n: expected failure from $*"
  fi
}
# st_ans NAME EXPECTED cmd args...: the command's stdout (an error reads ERROR).
st_ans() {
  local n=$1 want=$2 got
  shift 2
  got=$("$@" 2> /dev/null) || got=ERROR
  st_eq "$n" "$want" "$got"
}

H64=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

selftest_names() {
  st_ok "safe_name: the one container" safe_name holzcloud-t32
  st_fail "safe_name: another container" safe_name holzcloud-t29-src
  st_fail "safe_name: a prefix of it" safe_name holzcloud-t3
  st_fail "safe_name: a longer name" safe_name holzcloud-t32x
  st_fail "safe_name: empty" safe_name ""
  st_ok "safe_out: the evidence directory" safe_out deploy/test/out/amd64-t32-src
  st_ok "safe_out: a run file" safe_out deploy/test/out/amd64-t32-pass1.log
  st_fail "safe_out: another phase" safe_out deploy/test/out/amd64-t29-src
  st_fail "safe_out: the scratch root" safe_out deploy/test/out
  st_fail "safe_out: the bare prefix" safe_out deploy/test/out/amd64-t32-
  st_fail "safe_out: a parent step" safe_out deploy/test/out/amd64-t32-../x
  st_fail "safe_out: below the directory" safe_out deploy/test/out/amd64-t32-src/x
  st_fail "safe_out: absolute" safe_out /deploy/test/out/amd64-t32-src
  st_fail "safe_out: the tree" safe_out .
  st_fail "safe_out: empty" safe_out ""
}

selftest_sums() {
  local s=$ST_TMP/sums f=$ST_TMP/find
  printf '%s  ./holzcloud.sqlite\n%s  ./media/1/a.jpg\n%s  ./media/1/b.jpg\n' "$H64" "$H64" "$H64" > "$s"
  printf './holzcloud.sqlite\n./media/1/a.jpg\n./media/1/b.jpg\n' > "$f"
  st_ans "sums_equal_find: the same list" yes sums_equal_find "$s" "$f"
  printf './media/1/b.jpg\n./holzcloud.sqlite\n./media/1/a.jpg\n' > "$f"
  st_ans "sums_equal_find: another order" yes sums_equal_find "$s" "$f"
  printf './holzcloud.sqlite\n./media/1/a.jpg\n./media/1/b.jpg\n./media/1/c.jpg\n' > "$f"
  st_ans "sums_equal_find: a file SHA256SUMS lacks" no sums_equal_find "$s" "$f"
  printf './holzcloud.sqlite\n./media/1/a.jpg\n' > "$f"
  st_ans "sums_equal_find: a listed file the directory lacks" no sums_equal_find "$s" "$f"
  printf './holzcloud.sqlite\n./media/1/a.jpg\n./media/1/x.jpg\n' > "$f"
  st_ans "sums_equal_find: one name differs" no sums_equal_find "$s" "$f"
  : > "$f"
  st_ans "sums_equal_find: an empty directory" no sums_equal_find "$s" "$f"
  st_ans "sums_equal_find: no SHA256SUMS" no sums_equal_find "$ST_TMP/absent" "$f"
  printf 'not a line\n' > "$s"
  printf 'not a line\n' > "$f"
  st_ans "sums_equal_find: a line that is no checksum line" no sums_equal_find "$s" "$f"
}

# fx_set FILE KEY VALUE: the line "KEY: ..." of FILE replaced by "KEY: VALUE"; with
# VALUE "-" the line is removed.
fx_set() {
  awk -v k="$2" -v v="$3" 'index($0, k ": ") == 1 { if (v != "-") print k ": " v; next } { print }' "$1" > "$1.x"
  mv "$1.x" "$1"
}

# fx_bak01 FILE [KEY VALUE]: every key of bak01.txt, all good; with KEY and VALUE,
# that one line replaced (or removed for "-").
fx_bak01() {
  local f=$1
  {
    echo "MANUAL-RC: 0"
    echo "SUMS-LINES: 31"
    echo "SUMS-EQUALS-FIND: yes"
    echo "SUMS-CHECK-RC: 0"
    echo "SUMS-HAS-SQLITE: yes"
    echo "SUMS-MEDIA-LINES: 27"
    echo "MANIFEST-CHECKSUMS-LINE: yes"
    echo "FLIP-MEDIA-FILE: ./media/1/a.jpg"
    echo "FLIP-MEDIA-RC: 1"
    echo "FLIP-MEDIA-NAMED: ./media/1/a.jpg"
    echo "INVARIANTS-AFTER-FLIP-MEDIA: same"
    echo "BITROT-CHECK: integrity: ok"
    echo "BITROT-RESTORE-RC: 1"
    echo "BITROT-NAMED: ./holzcloud.sqlite"
    echo "INVARIANTS-AFTER-BITROT: same"
    echo "DELETED-FILE: ./media/1/z.bin"
    echo "DELETED-RC: 1"
    echo "DELETED-NAMED: ./media/1/z.bin"
    echo "INVARIANTS-AFTER-DELETED: same"
    echo "NOSUMS-RC: 1"
    echo "NOSUMS-NAMED: yes"
    echo "INVARIANTS-AFTER-NOSUMS: same"
    echo "OLD-BACKUP-SCRIPT: v0.1.4 sha256 $H64"
    echo "OLD-BACKUP-RC: 0"
    echo "OLD-RESTORE-RC: 0"
    echo "OLD-NOTICE: yes"
    echo "NEW-RESTORE-RC: 0"
    echo "NEW-VERIFIED-LINE: Checksums: 31 files verified"
    echo "BAK-01 PASS"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_bak01() {
  local f=$ST_TMP/bak01.txt kv
  st_ans "verdict_bak01: no file" NOT-RUN verdict_bak01 "$ST_TMP/absent.txt"
  fx_bak01 "$f"
  st_ans "verdict_bak01: every key good" PASS verdict_bak01 "$f"
  # One wrong (or absent, "-") value per case; each must give FAIL.
  for kv in "MANUAL-RC|1" "SUMS-LINES|0" "SUMS-EQUALS-FIND|no" "SUMS-CHECK-RC|1" "SUMS-HAS-SQLITE|no" \
    "SUMS-MEDIA-LINES|0" "SUMS-MEDIA-LINES|x" "MANIFEST-CHECKSUMS-LINE|no" \
    "FLIP-MEDIA-FILE|./media/1/b.jpg" "FLIP-MEDIA-RC|0" "FLIP-MEDIA-NAMED|./media/1/b.jpg" "FLIP-MEDIA-NAMED|" \
    "INVARIANTS-AFTER-FLIP-MEDIA|changed" "BITROT-CHECK|integrity: database disk image is malformed" \
    "BITROT-RESTORE-RC|0" "BITROT-NAMED|./media/1/a.jpg" "INVARIANTS-AFTER-BITROT|changed" \
    "DELETED-FILE|./holzcloud.sqlite" "DELETED-RC|0" "DELETED-NAMED|./media/1/a.jpg" "INVARIANTS-AFTER-DELETED|changed" \
    "NOSUMS-RC|0" "NOSUMS-NAMED|no" "INVARIANTS-AFTER-NOSUMS|changed" \
    "OLD-BACKUP-SCRIPT|v0.1.3 sha256 $H64" "OLD-BACKUP-SCRIPT|v0.1.4 sha256 x" "OLD-BACKUP-RC|1" \
    "OLD-RESTORE-RC|1" "OLD-NOTICE|no" "NEW-RESTORE-RC|3" \
    "NEW-VERIFIED-LINE|Checksums: 30 files verified" "NEW-VERIFIED-LINE|" \
    "MANUAL-RC|-" "NEW-VERIFIED-LINE|-" "INVARIANTS-AFTER-NOSUMS|-"; do
    fx_bak01 "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_bak01: ${kv%%|*} '${kv#*|}'" FAIL verdict_bak01 "$f"
  done
  fx_bak01 "$f"
  echo "MANUAL-RC: 0" >> "$f"
  st_ans "verdict_bak01: a key twice" FAIL verdict_bak01 "$f"
}

fx_bak02() {
  local f=$1
  {
    echo "LOST: media row 1 website 1 file 1a2b.jpg"
    echo "LOST: media_variants row 3 label thumb file 1a2b-thumb.webp"
    echo "BACKUP-RC: 0"
    echo "SUMS-LISTS-LOST: no"
    echo "RESTORE-RC: 3"
    echo "MISSING-BEFORE-QUESTION: 2"
    echo "MISSING-AT-END: 2"
    echo "READYZ: ok"
    echo "BAK-02 PASS"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_bak02() {
  local f=$ST_TMP/bak02.txt kv
  st_ans "verdict_bak02: no file" NOT-RUN verdict_bak02 "$ST_TMP/absent.txt"
  fx_bak02 "$f"
  st_ans "verdict_bak02: every key good" PASS verdict_bak02 "$f"
  for kv in "BACKUP-RC|1" "SUMS-LISTS-LOST|yes" "RESTORE-RC|0" "RESTORE-RC|1" "MISSING-BEFORE-QUESTION|1" \
    "MISSING-AT-END|3" "MISSING-AT-END|0" "READYZ|503" "READYZ|-" "RESTORE-RC|-"; do
    fx_bak02 "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_bak02: ${kv%%|*} '${kv#*|}'" FAIL verdict_bak02 "$f"
  done
  fx_bak02 "$f"
  grep -v '^LOST: media_variants' "$f" > "$f.x" && mv "$f.x" "$f"
  st_ans "verdict_bak02: no variant lost" FAIL verdict_bak02 "$f"
  fx_bak02 "$f"
  grep -v '^LOST: media row' "$f" > "$f.x" && mv "$f.x" "$f"
  st_ans "verdict_bak02: no original lost" FAIL verdict_bak02 "$f"
  fx_bak02 "$f" MISSING-BEFORE-QUESTION 1
  fx_set "$f" MISSING-AT-END 1
  st_ans "verdict_bak02: one MISSING line for two lost files" FAIL verdict_bak02 "$f"
}

fx_bak03() {
  local f=$1
  {
    echo "PLANTED: /opt/holzcloud/backups/20200101-031500 (locked: root-owned)"
    echo "PLANTED: /opt/holzcloud/backups/20200102-031500"
    echo "UNIT-DROPINS: ''"
    echo "START-RC: 1"
    echo "RESULT: exit-code"
    echo "EXEC-MAIN-STATUS: 40"
    echo "JOURNAL-NAMES-DIR: yes"
    echo "LOCKED-STILL-THERE: yes"
    echo "REMOVABLE-GONE: yes"
    echo "NIGHT-HAS-MANIFEST: yes"
    echo "BAK-03 PASS"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_bak03() {
  local f=$ST_TMP/bak03.txt kv
  st_ans "verdict_bak03: no file" NOT-RUN verdict_bak03 "$ST_TMP/absent.txt"
  fx_bak03 "$f"
  st_ans "verdict_bak03: every key good" PASS verdict_bak03 "$f"
  for kv in "UNIT-DROPINS|'/etc/systemd/system/holzcloud-backup.service.d/x.conf'" "UNIT-DROPINS|" \
    "RESULT|success" "EXEC-MAIN-STATUS|0" "EXEC-MAIN-STATUS|4" "JOURNAL-NAMES-DIR|no" "LOCKED-STILL-THERE|no" \
    "REMOVABLE-GONE|no" "NIGHT-HAS-MANIFEST|no" "RESULT|-" "EXEC-MAIN-STATUS|-"; do
    fx_bak03 "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_bak03: ${kv%%|*} '${kv#*|}'" FAIL verdict_bak03 "$f"
  done
}

selftest_invariants() {
  local a=$ST_TMP/inv-a b=$ST_TMP/inv-b
  printf 'inode 12\ninvocation abc\nactive active\ntree %s\n' "$H64" > "$a"
  cp "$a" "$b"
  st_ans "invariants_same: equal" same invariants_same "$a" "$b"
  printf 'inode 13\ninvocation abc\nactive active\ntree %s\n' "$H64" > "$b"
  st_ans "invariants_same: another inode" changed invariants_same "$a" "$b"
  printf 'inode 12\ninvocation abd\nactive active\ntree %s\n' "$H64" > "$b"
  st_ans "invariants_same: another invocation" changed invariants_same "$a" "$b"
  printf 'inode 12\ninvocation abc\nactive inactive\ntree %s\n' "$H64" > "$b"
  st_ans "invariants_same: no longer active" changed invariants_same "$a" "$b"
  printf 'inode 12\ninvocation abc\nactive active\ntree 0%s\n' "$H64" > "$b"
  st_ans "invariants_same: another tree" changed invariants_same "$a" "$b"
  st_ans "invariants_same: no second file" changed invariants_same "$a" "$ST_TMP/absent"
  : > "$a"
  : > "$b"
  st_ans "invariants_same: two empty files" changed invariants_same "$a" "$b"
  printf 'inode \ninvocation \nactive inactive\ntree %s\n' "$H64" > "$a"
  cp "$a" "$b"
  st_ans "invariants_same: equal, but without an inode" changed invariants_same "$a" "$b"
  printf 'inode 12\ninvocation abc\nactive active\n' > "$a"
  cp "$a" "$b"
  st_ans "invariants_same: equal, but without a tree" changed invariants_same "$a" "$b"
}

st_has() { # NAME NEEDLE HAYSTACK: the text must contain the needle
  ST_N=$((ST_N + 1))
  case "$3" in
    *"$2"*) ;;
    *)
      ST_BAD=$((ST_BAD + 1))
      say "SELFTEST BAD $1: expected the text to contain '$2', got: $3"
      ;;
  esac
}

# fx_scratch E: a scratch root holding every file of the paste list, small texts.
fx_scratch() {
  local e=$1 p
  rm -rf "${e:?}"
  mkdir -p "$e/amd64-t32-src"
  for p in $(t32_paste_list); do
    printf 'line one of %s\nKEY: value\n' "$p" > "$e/$p"
  done
  printf 'BAK-01 PASS\nBAK-02 PASS\nBAK-03 PASS\n' > "$e/amd64-t32-src/summary.txt"
  printf '0\n' > "$e/amd64-t32-pass1.status"
  git rev-parse --short HEAD > "$e/amd64-t32-evidence-commit.txt"
}

# fx_record E R [EVIDENCE-LINE]: a record whose appendix is `appendix` over E.
fx_record() {
  local e=$1 r=$2 ev=${3:-}
  [ -n "$ev" ] || ev="Evidence commit: $(cat "$e/amd64-t32-evidence-commit.txt")"
  {
    echo "# Backup verify run"
    echo
    echo "$ev"
    echo
    echo 'The summary is `amd64-t32-src/summary.txt`; the directory `amd64-t32-src` holds the rest.'
    echo
    echo "## Appendix: raw output"
    echo
    appendix_print "$e"
  } > "$r"
}

# rc_of cmd args...: the status of the command, its output dropped.
rc_of() {
  local rc=0
  "$@" > /dev/null 2>&1 || rc=$?
  echo "$rc"
}

selftest_record() {
  local e=$ST_TMP/e r=$ST_TMP/RUN.md out marker
  fx_scratch "$e"
  fx_record "$e" "$r"
  out=$(t32_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a record equal to its files" 0 "$(rc_of t32_record_check "$r" "$e")"
  st_has "record-check: the OK line" "RECORD-CHECK OK (" "$out"

  # One byte of one block differs from its file.
  fx_record "$e" "$r"
  awk '{ if ($0 == "line one of amd64-t32-src/bak02.txt") print "line one of amd64-t32-src/bak02.txT"; else print }' "$r" > "$r.x" && mv "$r.x" "$r"
  out=$(t32_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a block differs by one byte" 1 "$(rc_of t32_record_check "$r" "$e")"
  st_has "record-check: the differing block is named" "amd64-t32-src/bak02.txt" "$out"

  # A DIRECTORY/FILE token outside the appendix without a block.
  fx_record "$e" "$r"
  awk '{ print } /^Evidence commit: / { print ""; print "See `amd64-t32-src/notpasted.txt`." }' "$r" > "$r.x" && mv "$r.x" "$r"
  out=$(t32_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a cited file without a block" 1 "$(rc_of t32_record_check "$r" "$e")"
  st_has "record-check: the cited file is named" "amd64-t32-src/notpasted.txt" "$out"

  # Two Evidence commit lines.
  fx_record "$e" "$r"
  awk '{ print } /^Evidence commit: / { print $0 }' "$r" > "$r.x" && mv "$r.x" "$r"
  st_eq "record-check: two Evidence commit lines" 1 "$(rc_of t32_record_check "$r" "$e")"

  # An Evidence commit that does not resolve.
  printf '0000000\n' > "$e/amd64-t32-evidence-commit.txt"
  fx_record "$e" "$r"
  st_eq "record-check: an Evidence commit that does not resolve" 1 "$(rc_of t32_record_check "$r" "$e")"
  fx_scratch "$e"

  # An Evidence commit that resolves but is not the pinned one.
  fx_record "$e" "$r" "Evidence commit: $(git rev-parse --short 'HEAD~1')"
  st_eq "record-check: an Evidence commit other than the pinned one" 1 "$(rc_of t32_record_check "$r" "$e")"

  # A private-key marker anywhere (built from two halves, so this file holds none).
  marker="-----BEGIN OPENSSH PRIV""ATE KEY-----"
  fx_record "$e" "$r"
  printf '%s\n' "$marker" >> "$r"
  st_eq "record-check: a private-key marker" 1 "$(rc_of t32_record_check "$r" "$e")"

  # A rehearsal's summary is never evidence.
  printf 'BAK-01 PASS\nBAK-02 PASS\nBAK-03 PASS\nREHEARSAL: yes\n' > "$e/amd64-t32-src/summary.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a rehearsal's summary" 1 "$(rc_of t32_record_check "$r" "$e")"
  fx_scratch "$e"

  # A block missing for a file of the paste list.
  fx_record "$e" "$r"
  awk '/^#### amd64-t32-src\/bak03-journal.txt$/ { skip = 1 } skip && /^```$/ { skip = 0; next } !skip { print }' "$r" > "$r.x" && mv "$r.x" "$r"
  st_eq "record-check: a file of the paste list without a block" 1 "$(rc_of t32_record_check "$r" "$e")"

  # No record at all.
  st_eq "record-check: no record" 1 "$(rc_of t32_record_check "$ST_TMP/absent.md" "$e")"
}

selftest_appendix() {
  local e=$ST_TMP/e
  fx_scratch "$e"
  st_eq "appendix: every file pasteable" 0 "$(rc_of appendix_print "$e")"
  st_has "appendix: the first block" '#### amd64-t32-src/summary.txt' "$(appendix_print "$e")"
  printf 'x\n```\n' >> "$e/amd64-t32-src/bak01.txt"
  st_eq "appendix: a line beginning with three backticks" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  printf '#### x\n' >> "$e/amd64-t32-src/bak02.txt"
  st_eq "appendix: a line beginning with '#### '" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  printf '## x\n' >> "$e/amd64-t32-src/bak03-journal.txt"
  st_eq "appendix: a line beginning with '## '" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  rm -f "$e/amd64-t32-src/measurements.txt"
  st_eq "appendix: a file of the list is missing" 1 "$(rc_of appendix_print "$e")"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/backup-verify-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_names
  selftest_sums
  selftest_verdict_bak01
  selftest_verdict_bak02
  selftest_verdict_bak03
  selftest_invariants
  selftest_record
  selftest_appendix
  if [ "$ST_BAD" -ne 0 ]; then
    echo "SELFTEST FAILED: $ST_BAD of $ST_N cases"
    exit 1
  fi
  echo "SELFTEST OK ($ST_N cases)"
}

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    selftest) cmd_selftest ;;
    run) cmd_run ;;
    down) cmd_down ;;
    appendix) cmd_appendix ;;
    record-check) cmd_record_check ;;
    *) usage ;;
  esac
done
