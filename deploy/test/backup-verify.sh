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
#           sha256sum -c in the container
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
  sed -n '2,44p' "${BASH_SOURCE[0]}" >&2
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
  if kv_is "$f" MANUAL-RC 0 && kv_is "$f" SUMS-EQUALS-FIND yes && kv_is "$f" SUMS-CHECK-RC 0 \
    && kv_is "$f" SUMS-HAS-SQLITE yes && kv_positive "$f" SUMS-MEDIA-LINES \
    && kv_is "$f" MANIFEST-CHECKSUMS-LINE yes; then
    echo PASS
  else
    echo FAIL
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
  printf 'BAK-01 %s\n' "$(verdict_bak01 "$f")" >> "$f"
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
  if stage_wanted bak01; then stage_bak01; fi
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

# fx_bak01_t1 FILE [KEY VALUE]: the keys of the first part of bak01, all good; with
# KEY and VALUE, that one line replaced.
fx_bak01_t1() {
  local f=$1 k=${2:-} v=${3:-}
  {
    echo "MANUAL-RC: 0"
    echo "SUMS-LINES: 31"
    echo "SUMS-EQUALS-FIND: yes"
    echo "SUMS-CHECK-RC: 0"
    echo "SUMS-HAS-SQLITE: yes"
    echo "SUMS-MEDIA-LINES: 27"
    echo "MANIFEST-CHECKSUMS-LINE: yes"
  } > "$f"
  if [ -n "$k" ]; then
    awk -v k="$k" -v v="$v" 'index($0, k ": ") == 1 { print k ": " v; next } { print }' "$f" > "$f.x"
    mv "$f.x" "$f"
  fi
}

selftest_verdict_bak01_t1() {
  local f=$ST_TMP/bak01.txt
  st_ans "verdict_bak01: no file" NOT-RUN verdict_bak01 "$ST_TMP/absent.txt"
  fx_bak01_t1 "$f"
  st_ans "verdict_bak01: every key good" PASS verdict_bak01 "$f"
  fx_bak01_t1 "$f" MANUAL-RC 1
  st_ans "verdict_bak01: the backup line failed" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" SUMS-EQUALS-FIND no
  st_ans "verdict_bak01: SHA256SUMS is not the directory" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" SUMS-CHECK-RC 1
  st_ans "verdict_bak01: sha256sum -c failed" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" SUMS-HAS-SQLITE no
  st_ans "verdict_bak01: the snapshot is not listed" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" SUMS-MEDIA-LINES 0
  st_ans "verdict_bak01: no media line" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" SUMS-MEDIA-LINES x
  st_ans "verdict_bak01: a media count that is no number" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f" MANIFEST-CHECKSUMS-LINE no
  st_ans "verdict_bak01: no checksums line in the MANIFEST" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f"
  grep -v '^MANUAL-RC: ' "$f" > "$f.x" && mv "$f.x" "$f"
  st_ans "verdict_bak01: a key is absent" FAIL verdict_bak01 "$f"
  fx_bak01_t1 "$f"
  echo "MANUAL-RC: 0" >> "$f"
  st_ans "verdict_bak01: a key twice" FAIL verdict_bak01 "$f"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/backup-verify-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_names
  selftest_sums
  selftest_verdict_bak01_t1
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
    *) usage ;;
  esac
done
