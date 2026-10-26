#!/usr/bin/env bash
# Phase 26: SRV-01, the five plugins and the backup timings on native amd64.
#
# A sibling of deploy/test/run.sh and deploy/test/nightly.sh. It edits neither
# (docs/beta.md binds both to their evidence commits) and reaches run.sh through
# its public subcommands only: `mdwe` for the five plugins under the shipped
# unit, with the canary deciding whether MemoryDenyWriteExecute is enforced, and
# `backup-chain` with BULK_MB=300 for the timings, the recipe that made the
# emulation figures of deploy/BACKUP-RUN.md. What this file adds is the rule
# that decides the verdict, so that nothing can read green that is not native
# and enforced, and the files a pressed button hands over.
#
# The work is done with the container path of the harness on the runner, not
# with the runner's own systemd. That is deliberate: it is the recipe already
# proven on native arm64 (R2 of deploy/SECURITY-SCORE.md), and its figures
# compare line by line with the emulation figures. The Docker base images and
# the apt packages are fetched at image build time, as the harness always did.
#
#   native-amd64.sh selftest    the pure functions below, a table of cases, no Docker
#   native-amd64.sh run         run.sh mdwe and run.sh backup-chain, then the summary,
#                               the evidence bundle, and the exit code of the verdict
#   native-amd64.sh summarize   native-summary.txt and runner.txt from what the directories hold
#                               (MDWE_RC and TIMING_RC name a stage status; absent means not played)
#   native-amd64.sh down        remove the containers of a run, and only those
#
# Subcommands can be chained in one call and run in order. A failing one ends the
# call with its status; `run` always ends it with the exit code of the verdict.
#
# Verdicts: PASS, VOID, FAIL, NOT-NATIVE, NOT-RUN. SRV-01 is PASS only when the
# Docker host and the container are x86_64, run.sh's verdict word is SETTLED and
# the chain summary has BAK-01, BAK-02, BAK-03 and RESTORE-NEGATIVE PASS,
# LEAK-CHECK OK and numeric POINT seed: and POINT bulk: lines. A canary that says
# seccomp is not enforced gives VOID, a host that is not x86_64 gives NOT-NATIVE
# (checked first, so an arm64 SETTLED never reads as amd64 evidence).
#
# Exit codes: 0 ok (PASS), 1 failure (FAIL, NOT-RUN, a refused bundle), 2 usage,
# 3 the platform cannot give the evidence (VOID or NOT-NATIVE).
#
# No command in this file sets a clock. The container shares the kernel clock of
# the Docker VM, so a clock setter here would move every container on the daemon.
#
# Environment (all optional):
#   PLATFORM        linux/amd64
#   NATIVE_PREFIX   amd64-native, the prefix of every evidence directory and container name
#   BASE_OUT        deploy/test/out
#   EVIDENCE_DIR    deploy/test/out/native-evidence, the bundle the workflow uploads
#   BULK_MB         300, megabytes of synthetic media of the second data point
#   NATIVE_STAGES   "mdwe timing"; a subset is for rehearsals and can never reach PASS,
#                   because the stage not played reads NOT-RUN
#   NEW_REF         HEAD, pinned to its short sha when `run` starts
#   DOCKER_RUN_ARGS extra `docker run` flags, handed to run.sh untouched (it splits them on purpose)
# Derived, below BASE_OUT: NATIVE_PREFIX (this file's own files: runner.txt and native-summary.txt),
#   NATIVE_PREFIX-mdwe (run.sh mdwe), NATIVE_PREFIX-bak-src and NATIVE_PREFIX-bak-tgt (run.sh
#   backup-chain, which adds -bulk to each for the second point); the containers are named
#   holzcloud-NATIVE_PREFIX-mdwe, -bak-src, -bak-tgt and -bak-tgt-bulk.
#
# Written for bash 3.2 (macOS): no arrays, no mapfile, no case conversion. Three
# lessons of earlier phases apply: `grep -c` ends 1 on zero matches, an
# assignment from a failing pipeline ends the script under set -e, and a `case`
# pattern inside $( ) does not parse, so it stands in a function.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

PLATFORM=${PLATFORM:-linux/amd64}
NATIVE_PREFIX=${NATIVE_PREFIX:-amd64-native}
BASE_OUT=${BASE_OUT:-deploy/test/out}
EVIDENCE_DIR=${EVIDENCE_DIR:-$BASE_OUT/native-evidence}
BULK_MB=${BULK_MB:-300}
NATIVE_STAGES=${NATIVE_STAGES:-mdwe timing}
NEW_REF=${NEW_REF:-HEAD}
DOCKER_RUN_ARGS=${DOCKER_RUN_ARGS:-}

say() { printf '%s\n' "$*" >&2; }
die() {
  say "native-amd64.sh: $*"
  exit 1
}
usage() {
  sed -n '2,50p' "${BASH_SOURCE[0]}" >&2
  exit 2
}

# The directories and containers are functions of the environment, read when they
# are used, so that the selftest can point them at a temporary tree.
d_self() { printf '%s\n' "$BASE_OUT/$NATIVE_PREFIX"; }
d_mdwe() { printf '%s\n' "$BASE_OUT/$NATIVE_PREFIX-mdwe"; }
d_src() { printf '%s\n' "$BASE_OUT/$NATIVE_PREFIX-bak-src"; }
d_tgt() { printf '%s\n' "$BASE_OUT/$NATIVE_PREFIX-bak-tgt"; }
# dir_names: the names of the evidence directories below BASE_OUT, one per line.
dir_names() {
  printf '%s\n' "$NATIVE_PREFIX" "$NATIVE_PREFIX-mdwe" "$NATIVE_PREFIX-bak-src" "$NATIVE_PREFIX-bak-tgt" \
    "$NATIVE_PREFIX-bak-src-bulk" "$NATIVE_PREFIX-bak-tgt-bulk"
}
container_names() {
  printf '%s\n' "holzcloud-$NATIVE_PREFIX-mdwe" "holzcloud-$NATIVE_PREFIX-bak-src" \
    "holzcloud-$NATIVE_PREFIX-bak-tgt" "holzcloud-$NATIVE_PREFIX-bak-tgt-bulk"
}

# ---------------------------------------------------------------------------
# Pure functions. No Docker, no clock, no network: arguments or a file in, one
# answer out on stdout. `selftest` holds a table of cases, the failing ones included.
# ---------------------------------------------------------------------------

# arch_is_x86 TEXT: success for exactly x86_64 or amd64, failure for anything else.
arch_is_x86() {
  case "${1:-}" in
    x86_64 | amd64) return 0 ;;
    *) return 1 ;;
  esac
}

# mdwe_word_of FILE: the first word after "MDWE VERDICT: " in the first such line;
# MISSING for an absent or empty file or a file without that line.
mdwe_word_of() {
  local f=${1:-} w=""
  if [ -z "$f" ] || [ ! -s "$f" ]; then
    echo MISSING
    return 0
  fi
  w=$(sed -n 's/^MDWE VERDICT: \([^ ]*\).*/\1/p' "$f" | head -n 1) || true
  printf '%s\n' "${w:-MISSING}"
}

# srv01_mdwe HOST_ARCH CONTAINER_ARCH WORD: PASS, VOID, FAIL or NOT-NATIVE.
# NOT-NATIVE is decided first, so an arm64 SETTLED is never amd64 evidence.
srv01_mdwe() {
  if ! arch_is_x86 "${1:-}" || ! arch_is_x86 "${2:-}"; then
    echo NOT-NATIVE
    return 0
  fi
  case "${3:-}" in
    SETTLED) echo PASS ;;
    VOID) echo VOID ;;
    *) echo FAIL ;;
  esac
}

# timing_point_ok FILE LABEL: the POINT LABEL: line carries BACKUP-WALL-S= and
# RESTORE-WALL-S= with a number.
timing_point_ok() {
  grep -E "^POINT $2: .*[ ]BACKUP-WALL-S=[0-9]+([.][0-9]+)?([ ]|\$)" "$1" 2> /dev/null \
    | grep -Eq "[ ]RESTORE-WALL-S=[0-9]+([.][0-9]+)?([ ]|\$)"
}

# srv01_timing FILE: PASS, or FAIL (reason) naming the first thing missing. FILE is
# a chain summary or a native summary: the lines have the same shape.
srv01_timing() {
  local f=${1:-} line key
  if [ -z "$f" ] || [ ! -s "$f" ]; then
    echo "FAIL (no chain summary)"
    return 0
  fi
  for line in "BAK-01 PASS" "BAK-02 PASS" "BAK-03 PASS" "RESTORE-NEGATIVE PASS" "LEAK-CHECK OK"; do
    if ! grep -qxF -- "$line" "$f"; then
      echo "FAIL (missing line: $line)"
      return 0
    fi
  done
  for key in seed bulk; do
    if ! timing_point_ok "$f" "$key"; then
      echo "FAIL (POINT $key: has no numeric BACKUP-WALL-S= and RESTORE-WALL-S=)"
      return 0
    fi
  done
  echo PASS
}

# srv01_overall MDWE TIMING: the precedence NOT-NATIVE, FAIL, NOT-RUN, VOID, PASS;
# the first that either input carries wins, PASS only when both are PASS. An input
# may carry a reason after its word; only the word counts.
srv01_overall() {
  local a=${1:-} b=${2:-} w
  a=${a%% *}
  b=${b%% *}
  for w in NOT-NATIVE FAIL NOT-RUN VOID; do
    if [ "$a" = "$w" ] || [ "$b" = "$w" ]; then
      echo "$w"
      return 0
    fi
  done
  if [ "$a" = PASS ] && [ "$b" = PASS ]; then
    echo PASS
  else
    echo FAIL
  fi
}

# srv01_exit WORD: the exit code of a verdict.
srv01_exit() {
  case "${1:-}" in
    PASS) echo 0 ;;
    VOID | NOT-NATIVE) echo 3 ;;
    *) echo 1 ;; # FAIL, NOT-RUN and anything unknown
  esac
}

# ---------------------------------------------------------------------------
# summarize: the files of the stages in, runner.txt and native-summary.txt out.
# Every read is guarded: a missing file gives "?", "MISSING" or a FAIL with its
# reason, and never ends the script under set -e.
# ---------------------------------------------------------------------------

# file_value FILE KEY: the text after "KEY:" of the first such line, trimmed; ? when absent.
file_value() {
  local v=""
  if [ -f "$1" ]; then
    v=$(sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n 1 | sed 's/[[:space:]]*$//') || true
  fi
  printf '%s\n' "${v:-?}"
}

# last_word TEXT: the last word, ? when there is none.
last_word() {
  local w=""
  w=$(printf '%s\n' "${1:-}" | awk '{print $NF}') || true
  printf '%s\n' "${w:-?}"
}

# canary_of FILE: the word run.sh canary wrote; missing when there is none.
canary_of() {
  local v=""
  if [ -s "$1" ]; then
    v=$(head -n 1 "$1" | tr -d '[:space:]') || true
  fi
  printf '%s\n' "${v:-missing}"
}

# seccomp_of FILE: the number after "Seccomp:" (not Seccomp_filters:); ? when absent.
seccomp_of() {
  local v=""
  if [ -f "$1" ]; then
    v=$(sed -n 's/^Seccomp:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$1" | head -n 1) || true
  fi
  printf '%s\n' "${v:-?}"
}

# engine_of FILE: the word after "engine: "; ? when absent.
engine_of() {
  local v=""
  if [ -f "$1" ]; then
    v=$(sed -n 's/^engine: \([a-z]*\).*/\1/p' "$1" | head -n 1) || true
  fi
  printf '%s\n' "${v:-?}"
}

# pass_count FILE: the number of lines that begin PASS; ? when the file is absent.
pass_count() {
  local v=""
  if [ -f "$1" ]; then
    v=$(grep -c '^PASS' "$1" || true)
  fi
  printf '%s\n' "${v:-?}"
}

# env_value NAME: the value of the environment variable NAME, "unset" when absent.
env_value() {
  eval "printf '%s\\n' \"\${$1:-unset}\""
}

# cpu_model: the model name of the first CPU in /proc/cpuinfo, ? when there is none.
cpu_model() {
  local v=""
  if [ -r /proc/cpuinfo ]; then
    v=$(sed -n 's/^model name[[:space:]]*:[[:space:]]*//p' /proc/cpuinfo | head -n 1) || true
  fi
  printf '%s\n' "${v:-?}"
}

# docker_line: "SERVER-VERSION ARCHITECTURE" when Docker answers, ? otherwise.
docker_line() {
  local v=""
  if command -v docker > /dev/null 2>&1; then
    v=$(docker info --format '{{.ServerVersion}} {{.Architecture}}' 2> /dev/null) || v=""
  fi
  printf '%s\n' "${v:-?}"
}

write_runner_txt() {
  local f=$1 v
  {
    printf 'UNAME: %s\n' "$(uname -srm 2> /dev/null || echo '?')"
    printf 'CPU: %s\n' "$(cpu_model)"
    printf 'DOCKER: %s\n' "$(docker_line)"
    for v in RUNNER_ENVIRONMENT RUNNER_OS RUNNER_ARCH ImageOS ImageVersion GITHUB_REPOSITORY GITHUB_SHA \
      GITHUB_RUN_ID GITHUB_RUN_ATTEMPT GITHUB_WORKFLOW; do
      printf '%s: %s\n' "$v" "$(env_value "$v")"
    done
  } > "$f"
}

# overall_of FILE: the word of the SRV-01 line of a native summary.
overall_of() {
  local v=""
  if [ -f "$1" ]; then
    v=$(sed -n 's/^SRV-01 //p' "$1" | head -n 1) || true
  fi
  printf '%s\n' "${v:-FAIL}"
}

# summarize MDWE_STATUS TIMING_STATUS: a stage status is a number, or notrun.
summarize() {
  local mst=${1:-notrun} tst=${2:-notrun}
  local dn dm ds vers host cont canary word commit run_line mv tv ov chain
  dn=$(d_self)
  dm=$(d_mdwe)
  ds=$(d_src)
  mkdir -p "$dn"
  write_runner_txt "$dn/runner.txt"

  vers=$dm/versions.txt
  host=$(file_value "$vers" "host architecture")
  cont=$(last_word "$(file_value "$vers" "container uname")")
  canary=$(canary_of "$dm/canary.verdict")
  word=$(mdwe_word_of "$dm/verdict.txt")

  if [ "$mst" = notrun ]; then
    mv=NOT-RUN
  else
    mv=$(srv01_mdwe "$host" "$cont" "$word")
  fi
  chain=$ds/summary.txt
  if [ "$tst" = notrun ]; then
    tv=NOT-RUN
  else
    tv=$(srv01_timing "$chain")
  fi
  ov=$(srv01_overall "$mv" "$tv")

  if [ -n "${GITHUB_SHA:-}" ]; then
    commit=$GITHUB_SHA
  else
    commit=$(git rev-parse HEAD 2> /dev/null) || commit=unknown
  fi
  if [ -n "${GITHUB_RUN_ID:-}" ]; then
    run_line="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-?}/actions/runs/$GITHUB_RUN_ID attempt ${GITHUB_RUN_ATTEMPT:-?}"
  else
    run_line=local
  fi

  {
    echo "---- native amd64 summary ----"
    echo "COMMIT: $commit"
    echo "RUN: $run_line"
    echo "RUNNER-ENVIRONMENT: $(env_value RUNNER_ENVIRONMENT)"
    echo "RUNNER-KERNEL: $(uname -sr 2> /dev/null || echo '?')"
    echo "HOST-ARCH: $host"
    echo "CONTAINER-ARCH: $cont"
    echo "CANARY: $canary"
    echo "SECCOMP-SHIPPED: $(seccomp_of "$dm/check.txt")"
    echo "ENGINE-SHIPPED: $(engine_of "$dm/check.txt")"
    echo "ENGINE-CONTROL: $(engine_of "$dm/mdwe-control.txt")"
    echo "ASSERTIONS-SHIPPED: $(pass_count "$dm/check.txt")"
    echo "ASSERTIONS-CONTROL: $(pass_count "$dm/mdwe-control.txt")"
    echo "MDWE-WORD: $word"
    echo "MDWE-STATUS: $mst"
    echo "TIMING-STATUS: $tst"
    if [ "$tst" != notrun ] && [ -f "$chain" ]; then
      grep -E '^(BAK-0|RESTORE-NEGATIVE |POINT |TIMER:|STEPS:|DEVIATIONS:|LEAK-CHECK )' "$chain" || true
    fi
    echo "SRV-01-MDWE $mv"
    echo "SRV-01-TIMING $tv"
    echo "SRV-01 $ov"
    echo "NATIVE-AMD64 DONE"
  } > "$dn/native-summary.txt"
  cat "$dn/native-summary.txt"
}

cmd_summarize() {
  summarize "${MDWE_RC:-notrun}" "${TIMING_RC:-notrun}"
}

# ---------------------------------------------------------------------------
# run and down
# ---------------------------------------------------------------------------

# guard_dir DIR: the same refusals as run.sh up, for a directory that is removed.
guard_dir() {
  case "${1:-}" in
    "" | / | . | .. | ./ | "$ROOT" | "$ROOT"/ | "$BASE_OUT" | "$BASE_OUT"/)
      die "refusing directory '${1:-}'"
      ;;
  esac
}

# stage_wanted NAME: NAME is one of the words of NATIVE_STAGES.
stage_wanted() {
  case " $NATIVE_STAGES " in
    *" $1 "*) return 0 ;;
  esac
  return 1
}

# stages_valid: every word of NATIVE_STAGES is mdwe or timing, and there is one.
stages_valid() {
  local s n=0
  for s in $NATIVE_STAGES; do
    case "$s" in
      mdwe | timing) n=$((n + 1)) ;;
      *) return 1 ;;
    esac
  done
  [ "$n" -gt 0 ]
}

# run_publish: the call point after the summary is written. The evidence bundle
# and the job summary page are added here.
run_publish() {
  return 0
}

cmd_run() {
  local n mdwe_rc=notrun timing_rc=notrun rc ov sum
  stages_valid || {
    say "native-amd64.sh: NATIVE_STAGES '$NATIVE_STAGES' must name mdwe, timing or both"
    exit 2
  }
  case "$NATIVE_PREFIX" in
    "" | */*) die "NATIVE_PREFIX '$NATIVE_PREFIX' must be a non-empty name without a slash" ;;
  esac
  command -v docker > /dev/null 2>&1 || die "docker is not installed or not on PATH"
  docker info > /dev/null 2>&1 || die "the Docker daemon does not answer"
  for n in $(container_names); do
    if docker container inspect "$n" > /dev/null 2>&1; then
      die "container $n exists; run 'native-amd64.sh down' first (a stale one is never reused)"
    fi
  done
  guard_dir "$(d_self)"
  guard_dir "$EVIDENCE_DIR"
  rm -rf "${EVIDENCE_DIR:?}" "$(d_self)"
  mkdir -p "$EVIDENCE_DIR" "$(d_self)"

  # The ref both containers are built from, pinned so a commit landing mid-run
  # changes nothing.
  NEW_REF=$(git rev-parse --short "${NEW_REF}^{commit}") || die "NEW_REF '$NEW_REF' is not a commit of this repository"
  say "native-amd64: platform $PLATFORM, NEW_REF pinned to $NEW_REF, stages: $NATIVE_STAGES, BULK_MB=$BULK_MB"

  # Each stage is a child process of run.sh with its status kept; a stage that
  # fails never stops the other.
  if stage_wanted mdwe; then
    rc=0
    env PLATFORM="$PLATFORM" TARGET=unit-test NAME="holzcloud-$NATIVE_PREFIX-mdwe" OUT_DIR="$(d_mdwe)" \
      DOCKER_RUN_ARGS="$DOCKER_RUN_ARGS" bash "$ROOT/deploy/test/run.sh" mdwe || rc=$?
    mdwe_rc=$rc
    say "native-amd64: run.sh mdwe ended with status $mdwe_rc"
  fi
  if stage_wanted timing; then
    rc=0
    env PLATFORM="$PLATFORM" TARGET=systemd-base NAME="holzcloud-$NATIVE_PREFIX-bak-src" OUT_DIR="$(d_src)" \
      TGT_NAME="holzcloud-$NATIVE_PREFIX-bak-tgt" TGT_OUT_DIR="$(d_tgt)" BULK_MB="$BULK_MB" NEW_REF="$NEW_REF" \
      DOCKER_RUN_ARGS="$DOCKER_RUN_ARGS" bash "$ROOT/deploy/test/run.sh" backup-chain || rc=$?
    timing_rc=$rc
    say "native-amd64: run.sh backup-chain ended with status $timing_rc"
  fi

  summarize "$mdwe_rc" "$timing_rc"
  sum=$(d_self)/native-summary.txt
  ov=$(overall_of "$sum")
  rc=$(srv01_exit "$ov")
  run_publish || {
    [ "$rc" -ne 0 ] || rc=1
  }
  say "native-amd64: SRV-01 $ov, exit $rc"
  exit "$rc"
}

# down: the containers of a run and the -bulk target, safe when absent.
cmd_down() {
  local n
  command -v docker > /dev/null 2>&1 || die "docker is not installed or not on PATH"
  for n in $(container_names); do
    docker rm -f "$n" > /dev/null 2>&1 || true
  done
  say "native-amd64: containers of $NATIVE_PREFIX removed"
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
st_has() { # NAME NEEDLE HAYSTACK: the text must contain the needle
  ST_N=$((ST_N + 1))
  case "$3" in
    *"$2"*) ;;
    *)
      ST_BAD=$((ST_BAD + 1))
      say "SELFTEST BAD $1: expected '$3' to contain '$2'"
      ;;
  esac
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

# --- Fixtures. The texts are those of the real runs, retyped verbatim: R1 (amd64
# under emulation, VOID) and R2 (arm64 native, SETTLED) of deploy/SECURITY-SCORE.md
# and the chain summary of deploy/BACKUP-RUN.md. -----------------------------

fx_pass17() {
  printf '%s\n' \
    'PASS loaded bestellung' 'PASS loaded jahreszahl' 'PASS loaded kontaktformular' \
    'PASS loaded nicht-gefunden' 'PASS loaded suche' 'PASS no-load-failures' \
    'PASS running bestellung' 'PASS running jahreszahl' 'PASS running kontaktformular' \
    'PASS running nicht-gefunden' 'PASS running suche' 'PASS invoke jahreszahl' \
    'PASS invoke kontaktformular' 'PASS invoke bestellung' 'PASS invoke suche' \
    'PASS invoke nicht-gefunden' 'PASS stable'
}

# fx_check_r1 FILE: R1 check.txt (the shipped unit, amd64 under emulation).
fx_check_r1() {
  {
    fx_pass17
    printf '%s\n' \
      'engine: compiler (9 anonymous executable mappings)' \
      '# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests' \
      'LATENCY /plugintest median_s=0.009147 n=10' \
      'LATENCY /suche?q=Plugintest median_s=0.008848 n=10' \
      'LATENCY /formular median_s=0.005883 n=10' \
      'LATENCY /bestellung median_s=0.005986 n=10'
    printf 'Seccomp:\t0\nSeccomp_filters:\t0\n'
    printf '%s\n' 'NRestarts=0' 'Result=success' \
      '{"time":"2026-10-12T10:41:01.789655629Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}' \
      '{"time":"2026-10-12T10:41:03.058772463Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:04.107872422Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:05.023770297Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:05.947229964Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}'
  } > "$1"
}

# fx_control_r1 FILE: R1 mdwe-control.txt (MemoryDenyWriteExecute=no).
fx_control_r1() {
  {
    fx_pass17
    printf '%s\n' \
      'engine: compiler (10 anonymous executable mappings)' \
      '# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests' \
      'LATENCY /plugintest median_s=0.009656 n=10' \
      'LATENCY /suche?q=Plugintest median_s=0.008233 n=10' \
      'LATENCY /formular median_s=0.005748 n=10' \
      'LATENCY /bestellung median_s=0.005695 n=10'
    printf 'Seccomp:\t0\nSeccomp_filters:\t0\n'
    printf '%s\n' 'NRestarts=0' 'Result=success' \
      '{"time":"2026-10-12T10:41:13.005876676Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}' \
      '{"time":"2026-10-12T10:41:14.305524468Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:15.326119344Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:16.252086094Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:17.153804678Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}'
  } > "$1"
}

# fx_check_r2 FILE: R2 check.txt (the shipped unit, arm64 native).
fx_check_r2() {
  {
    fx_pass17
    printf '%s\n' \
      'engine: interpreter (0 anonymous executable mappings)' \
      '# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests' \
      'LATENCY /plugintest median_s=0.011671 n=10' \
      'LATENCY /suche?q=Plugintest median_s=0.007672 n=10' \
      'LATENCY /formular median_s=0.000907 n=10' \
      'LATENCY /bestellung median_s=0.000804 n=10'
    printf 'Seccomp:\t2\nSeccomp_filters:\t24\n'
    printf '%s\n' 'NRestarts=0' 'Result=success' \
      '{"time":"2026-10-12T10:41:47.559763511Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}' \
      '{"time":"2026-10-12T10:41:47.637560803Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:47.694508553Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:47.745387636Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:47.794530595Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}'
  } > "$1"
}

# fx_control_r2 FILE: R2 mdwe-control.txt.
fx_control_r2() {
  {
    fx_pass17
    printf '%s\n' \
      'engine: compiler (8 anonymous executable mappings)' \
      '# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests' \
      'LATENCY /plugintest median_s=0.002179 n=10' \
      'LATENCY /suche?q=Plugintest median_s=0.002084 n=10' \
      'LATENCY /formular median_s=0.000260 n=10' \
      'LATENCY /bestellung median_s=0.000209 n=10'
    printf 'Seccomp:\t2\nSeccomp_filters:\t22\n'
    printf '%s\n' 'NRestarts=0' 'Result=success' \
      '{"time":"2026-10-12T10:41:50.982459638Z","level":"INFO","msg":"plugin loaded","plugin":"kontaktformular","version":"1.5.0"}' \
      '{"time":"2026-10-12T10:41:51.75384518Z","level":"INFO","msg":"plugin loaded","plugin":"bestellung","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:52.292437764Z","level":"INFO","msg":"plugin loaded","plugin":"nicht-gefunden","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:52.81347993Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}' \
      '{"time":"2026-10-12T10:41:53.309832875Z","level":"INFO","msg":"plugin loaded","plugin":"jahreszahl","version":"1.0.0"}'
  } > "$1"
}

fx_verdict_r1() {
  printf '%s\n' 'MDWE VERDICT: VOID - seccomp is not enforced on this platform (linux/amd64), so the plugin results prove nothing about MemoryDenyWriteExecute; 17/17 assertions passed, engine under the shipped unit: compiler' > "$1"
}
fx_verdict_r2() {
  printf '%s\n' "MDWE VERDICT: SETTLED - MemoryDenyWriteExecute=true is enforced here (canary: enforced), all five plugins loaded and ran on wazero's interpreter fallback (17/17 assertions), the control with the directive off ran on the compiler engine (17/17), and the unit stays unchanged" > "$1"
}
fx_verdict_fails() {
  printf '%s\n' 'MDWE VERDICT: FAILS - seccomp is enforced and only 12/17 assertions passed under MemoryDenyWriteExecute=true: the plugins do not load under the shipped unit' > "$1"
}

fx_versions_r1() {
  cat > "$1" << 'EOF'
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit x86_64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
EOF
}
fx_versions_r2() {
  cat > "$1" << 'EOF'
host architecture:     aarch64
host operating system: Docker Desktop
host kernel:           7.0.14-linuxkit
container uname:       Linux 7.0.14-linuxkit aarch64
container systemd:     systemd 252 (252.39-1~deb12u2)
container os:          PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
EOF
}

# fx_chain FILE: the chain summary of deploy/BACKUP-RUN.md (amd64 under emulation).
fx_chain() {
  cat > "$1" << 'EOF'
---- summary (linux/amd64) ----
BAK-01 PASS
BAK-02 PASS
BAK-03 PASS
RESTORE-NEGATIVE PASS
POINT seed: BACKUP-BYTES=20548955 BACKUP-WALL-S=0.692 RESTORE-WALL-S=8.126 VERIFY-S=0.083 STOP-MOVE-COPY-S=0.455 START-READY-S=7.397
POINT bulk: BACKUP-BYTES=335125851 BACKUP-WALL-S=1.015 RESTORE-WALL-S=8.296 VERIFY-S=0.088 STOP-MOVE-COPY-S=0.631 START-READY-S=7.379
TIMER: TRIGGERED-BY=holzcloud-backup.timer RESULT=success
STEPS: 78 played, 0 failed
DEVIATIONS: backup-name dropin fixture mount scp timer transfer
LEAK-CHECK OK
EOF
}

# fx_mdwe_r1 DIR, fx_mdwe_r2 DIR: a whole run.sh mdwe directory.
fx_mdwe_r1() {
  mkdir -p "$1"
  fx_versions_r1 "$1/versions.txt"
  echo not-enforced > "$1/canary.verdict"
  fx_check_r1 "$1/check.txt"
  fx_control_r1 "$1/mdwe-control.txt"
  fx_verdict_r1 "$1/verdict.txt"
}
fx_mdwe_r2() {
  mkdir -p "$1"
  fx_versions_r2 "$1/versions.txt"
  echo enforced > "$1/canary.verdict"
  fx_check_r2 "$1/check.txt"
  fx_control_r2 "$1/mdwe-control.txt"
  fx_verdict_r2 "$1/verdict.txt"
}
# fx_mdwe_synthetic DIR: R2 with every aarch64 turned into x86_64. SYNTHETIC: no
# native amd64 run exists, this only exercises the verdict rules, and it is never
# written outside the temporary directory of the selftest.
fx_mdwe_synthetic() {
  fx_mdwe_r2 "$1"
  sed 's/aarch64/x86_64/' "$1/versions.txt" > "$1/versions.new"
  mv "$1/versions.new" "$1/versions.txt"
}

# sum_val FILE KEY: the value of a "KEY: value" line; sum_word FILE NAME: of a "NAME word" line.
sum_val() { sed -n "s/^$2: //p" "$1" 2> /dev/null | head -n 1; }
sum_word() { sed -n "s/^$2 //p" "$1" 2> /dev/null | head -n 1; }

# st_summarize BASE MDWE_STATUS TIMING_STATUS: summarize under set -e with the
# environment pointed at BASE and prefix p; prints the exit status. A command
# substitution does not inherit set -e, so the subshell asks for it itself.
st_summarize() {
  local rc=0
  (
    set -e
    BASE_OUT=$1
    NATIVE_PREFIX=p
    docker_line() { echo '?'; }
    summarize "$2" "$3"
  ) > "$1/summarize.out" 2>&1 || rc=$?
  printf '%s\n' "$rc"
}

selftest_pure() {
  local t=$ST_TMP row h c w e

  st_ok "arch x86_64" arch_is_x86 x86_64
  st_ok "arch amd64" arch_is_x86 amd64
  st_fail "arch aarch64" arch_is_x86 aarch64
  st_fail "arch arm64" arch_is_x86 arm64
  st_fail "arch x86" arch_is_x86 x86
  st_fail "arch ?" arch_is_x86 '?'
  st_fail "arch empty" arch_is_x86 ""

  mkdir -p "$t/w"
  fx_verdict_r1 "$t/w/r1"
  fx_verdict_r2 "$t/w/r2"
  fx_verdict_fails "$t/w/fails"
  : > "$t/w/empty"
  printf 'nothing to see here\n' > "$t/w/noline"
  st_eq "mdwe_word_of R1" VOID "$(mdwe_word_of "$t/w/r1")"
  st_eq "mdwe_word_of R2" SETTLED "$(mdwe_word_of "$t/w/r2")"
  st_eq "mdwe_word_of FAILS" FAILS "$(mdwe_word_of "$t/w/fails")"
  st_eq "mdwe_word_of missing file" MISSING "$(mdwe_word_of "$t/w/none")"
  st_eq "mdwe_word_of empty file" MISSING "$(mdwe_word_of "$t/w/empty")"
  st_eq "mdwe_word_of no verdict line" MISSING "$(mdwe_word_of "$t/w/noline")"
  st_eq "mdwe_word_of no argument" MISSING "$(mdwe_word_of "")"

  while read -r h c w e; do
    st_eq "srv01_mdwe $h $c $w" "$e" "$(srv01_mdwe "$h" "$c" "$w")"
  done << 'EOF'
x86_64 x86_64 SETTLED PASS
x86_64 x86_64 VOID VOID
x86_64 x86_64 FAILS FAIL
x86_64 x86_64 INCONSISTENT FAIL
x86_64 x86_64 MISSING FAIL
x86_64 x86_64 SOMETHING-ELSE FAIL
amd64 amd64 SETTLED PASS
aarch64 aarch64 SETTLED NOT-NATIVE
aarch64 x86_64 VOID NOT-NATIVE
x86_64 aarch64 SETTLED NOT-NATIVE
x86_64 aarch64 VOID NOT-NATIVE
? ? SETTLED NOT-NATIVE
x86_64 ? SETTLED NOT-NATIVE
? x86_64 SETTLED NOT-NATIVE
aarch64 aarch64 FAILS NOT-NATIVE
EOF

  # srv01_timing on the real chain summary, then with each required line removed.
  fx_chain "$t/w/chain"
  st_eq "srv01_timing real chain summary" PASS "$(srv01_timing "$t/w/chain")"
  for row in "BAK-01 PASS" "BAK-02 PASS" "BAK-03 PASS" "RESTORE-NEGATIVE PASS" "LEAK-CHECK OK"; do
    grep -vxF -- "$row" "$t/w/chain" > "$t/w/chain-minus" || true
    st_has "srv01_timing without '$row'" "FAIL (missing line: $row)" "$(srv01_timing "$t/w/chain-minus")"
    sed "s/^$row\$/${row% *} FAIL/" "$t/w/chain" > "$t/w/chain-bad"
    case "$row" in
      "LEAK-CHECK OK") sed 's/^LEAK-CHECK OK$/LEAK-CHECK BAD/' "$t/w/chain" > "$t/w/chain-bad" ;;
    esac
    st_has "srv01_timing '$row' replaced" "FAIL (missing line: $row)" "$(srv01_timing "$t/w/chain-bad")"
  done
  grep -v '^POINT seed:' "$t/w/chain" > "$t/w/chain-minus" || true
  st_has "srv01_timing without POINT seed" "POINT seed" "$(srv01_timing "$t/w/chain-minus")"
  grep -v '^POINT bulk:' "$t/w/chain" > "$t/w/chain-minus" || true
  st_has "srv01_timing without POINT bulk" "POINT bulk" "$(srv01_timing "$t/w/chain-minus")"
  sed 's/RESTORE-WALL-S=8.126/RESTORE-WALL-S=?/' "$t/w/chain" > "$t/w/chain-bad"
  st_has "srv01_timing non-numeric restore wall time (seed)" "POINT seed" "$(srv01_timing "$t/w/chain-bad")"
  sed 's/BACKUP-WALL-S=1.015/BACKUP-WALL-S=/' "$t/w/chain" > "$t/w/chain-bad"
  st_has "srv01_timing empty backup wall time (bulk)" "POINT bulk" "$(srv01_timing "$t/w/chain-bad")"
  sed 's/BACKUP-WALL-S=0.692/BACKUP-WALL-S=0.69x/' "$t/w/chain" > "$t/w/chain-bad"
  st_has "srv01_timing garbage wall time (seed)" "POINT seed" "$(srv01_timing "$t/w/chain-bad")"
  sed 's/ BACKUP-WALL-S=0.692//' "$t/w/chain" > "$t/w/chain-bad"
  st_has "srv01_timing wall time key absent (seed)" "POINT seed" "$(srv01_timing "$t/w/chain-bad")"
  sed 's/BACKUP-WALL-S=0.692/BACKUP-WALL-S=7/' "$t/w/chain" > "$t/w/chain-ok"
  st_eq "srv01_timing whole-number wall time" PASS "$(srv01_timing "$t/w/chain-ok")"
  st_eq "srv01_timing empty file" "FAIL (no chain summary)" "$(srv01_timing "$t/w/empty")"
  st_eq "srv01_timing missing file" "FAIL (no chain summary)" "$(srv01_timing "$t/w/none")"
  st_eq "srv01_timing no argument" "FAIL (no chain summary)" "$(srv01_timing "")"

  while read -r h c e; do
    st_eq "srv01_overall $h $c" "$e" "$(srv01_overall "$h" "$c")"
  done << 'EOF'
PASS PASS PASS
PASS VOID VOID
VOID PASS VOID
VOID VOID VOID
PASS NOT-RUN NOT-RUN
NOT-RUN PASS NOT-RUN
NOT-RUN VOID NOT-RUN
VOID NOT-RUN NOT-RUN
PASS FAIL FAIL
FAIL PASS FAIL
FAIL VOID FAIL
VOID FAIL FAIL
FAIL NOT-RUN FAIL
NOT-RUN FAIL FAIL
NOT-NATIVE NOT-RUN NOT-NATIVE
NOT-RUN NOT-NATIVE NOT-NATIVE
NOT-NATIVE FAIL NOT-NATIVE
FAIL NOT-NATIVE NOT-NATIVE
NOT-NATIVE VOID NOT-NATIVE
VOID NOT-NATIVE NOT-NATIVE
NOT-NATIVE PASS NOT-NATIVE
PASS NOT-NATIVE NOT-NATIVE
NOT-NATIVE NOT-NATIVE NOT-NATIVE
FAIL FAIL FAIL
NOT-RUN NOT-RUN NOT-RUN
EOF
  st_eq "srv01_overall carries a reason" FAIL "$(srv01_overall PASS 'FAIL (no chain summary)')"
  st_eq "srv01_overall reason after NOT-NATIVE" NOT-NATIVE "$(srv01_overall 'FAIL (x)' NOT-NATIVE)"
  st_eq "srv01_overall unknown word" FAIL "$(srv01_overall PASS BOGUS)"
  st_eq "srv01_overall empty" FAIL "$(srv01_overall "" "")"

  st_eq "srv01_exit PASS" 0 "$(srv01_exit PASS)"
  st_eq "srv01_exit FAIL" 1 "$(srv01_exit FAIL)"
  st_eq "srv01_exit NOT-RUN" 1 "$(srv01_exit NOT-RUN)"
  st_eq "srv01_exit VOID" 3 "$(srv01_exit VOID)"
  st_eq "srv01_exit NOT-NATIVE" 3 "$(srv01_exit NOT-NATIVE)"
  st_eq "srv01_exit unknown" 1 "$(srv01_exit BOGUS)"
  st_eq "srv01_exit empty" 1 "$(srv01_exit "")"
}

selftest_summarize() {
  local t=$ST_TMP rc f e

  # (a) the R2 directories (arm64, native, SETTLED), timing not played.
  mkdir -p "$t/a"
  fx_mdwe_r2 "$t/a/p-mdwe"
  rc=$(st_summarize "$t/a" 0 notrun)
  f=$t/a/p/native-summary.txt
  st_eq "summarize a: does not abort" 0 "$rc"
  st_eq "summarize a: first line" "---- native amd64 summary ----" "$(head -n 1 "$f")"
  st_eq "summarize a: last line" "NATIVE-AMD64 DONE" "$(tail -n 1 "$f")"
  st_eq "summarize a: HOST-ARCH" aarch64 "$(sum_val "$f" HOST-ARCH)"
  st_eq "summarize a: CONTAINER-ARCH" aarch64 "$(sum_val "$f" CONTAINER-ARCH)"
  st_eq "summarize a: CANARY" enforced "$(sum_val "$f" CANARY)"
  st_eq "summarize a: SECCOMP-SHIPPED" 2 "$(sum_val "$f" SECCOMP-SHIPPED)"
  st_eq "summarize a: ENGINE-SHIPPED" interpreter "$(sum_val "$f" ENGINE-SHIPPED)"
  st_eq "summarize a: ENGINE-CONTROL" compiler "$(sum_val "$f" ENGINE-CONTROL)"
  st_eq "summarize a: ASSERTIONS-SHIPPED" 17 "$(sum_val "$f" ASSERTIONS-SHIPPED)"
  st_eq "summarize a: ASSERTIONS-CONTROL" 17 "$(sum_val "$f" ASSERTIONS-CONTROL)"
  st_eq "summarize a: MDWE-WORD" SETTLED "$(sum_val "$f" MDWE-WORD)"
  st_eq "summarize a: MDWE-STATUS" 0 "$(sum_val "$f" MDWE-STATUS)"
  st_eq "summarize a: TIMING-STATUS" notrun "$(sum_val "$f" TIMING-STATUS)"
  st_eq "summarize a: SRV-01-MDWE" NOT-NATIVE "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize a: SRV-01-TIMING" NOT-RUN "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize a: SRV-01" NOT-NATIVE "$(sum_word "$f" SRV-01)"
  st_eq "summarize a: no chain line copied" 0 "$(grep -c '^\(BAK-0\|POINT \|LEAK-CHECK \)' "$f" || true)"
  st_eq "summarize a: exit code of the verdict" 3 "$(srv01_exit "$(overall_of "$f")")"
  st_eq "summarize a: the order of the verdict lines" "SRV-01-MDWE SRV-01-TIMING SRV-01 NATIVE-AMD64" \
    "$(grep -E '^(SRV-01|NATIVE-AMD64)' "$f" | sed 's/ .*//' | tr '\n' ' ' | sed 's/ $//')"
  for e in UNAME CPU DOCKER RUNNER_ENVIRONMENT RUNNER_OS RUNNER_ARCH ImageOS ImageVersion GITHUB_REPOSITORY \
    GITHUB_SHA GITHUB_RUN_ID GITHUB_RUN_ATTEMPT GITHUB_WORKFLOW; do
    st_eq "summarize a: runner.txt has $e" 1 "$(grep -c "^$e: " "$t/a/p/runner.txt" || true)"
  done

  # (a2) the same directories with GITHUB_* absent and then present.
  rc=$(
    unset RUNNER_ENVIRONMENT GITHUB_RUN_ID
    st_summarize "$t/a" 0 notrun > /dev/null
    sum_val "$t/a/p/runner.txt" RUNNER_ENVIRONMENT
  )
  st_eq "summarize a2: an absent variable reads unset" unset "$rc"
  rc=$(
    export GITHUB_RUN_ID=4242 GITHUB_RUN_ATTEMPT=2 GITHUB_REPOSITORY=owner/repo GITHUB_SHA=0123456789abcdef \
      RUNNER_ENVIRONMENT=github-hosted GITHUB_SERVER_URL=https://github.example
    st_summarize "$t/a" 0 notrun > /dev/null
    sum_val "$t/a/p/native-summary.txt" RUN
  )
  st_eq "summarize a2: RUN names the run" "https://github.example/owner/repo/actions/runs/4242 attempt 2" "$rc"
  rc=$(
    unset GITHUB_RUN_ID
    export GITHUB_SHA=0123456789abcdef
    st_summarize "$t/a" 0 notrun > /dev/null
    sum_val "$t/a/p/native-summary.txt" COMMIT
  )
  st_eq "summarize a2: COMMIT is GITHUB_SHA when set" 0123456789abcdef "$rc"
  rc=$(
    unset GITHUB_RUN_ID GITHUB_SHA
    st_summarize "$t/a" 0 notrun > /dev/null
    sum_val "$t/a/p/native-summary.txt" RUN
  )
  st_eq "summarize a2: RUN is local without a run id" local "$rc"

  # (b) the R1 directories (amd64 under emulation on an arm64 host): NOT-NATIVE too.
  mkdir -p "$t/b"
  fx_mdwe_r1 "$t/b/p-mdwe"
  rc=$(st_summarize "$t/b" 3 notrun)
  f=$t/b/p/native-summary.txt
  st_eq "summarize b: does not abort" 0 "$rc"
  st_eq "summarize b: HOST-ARCH" aarch64 "$(sum_val "$f" HOST-ARCH)"
  st_eq "summarize b: CONTAINER-ARCH" x86_64 "$(sum_val "$f" CONTAINER-ARCH)"
  st_eq "summarize b: CANARY" not-enforced "$(sum_val "$f" CANARY)"
  st_eq "summarize b: SECCOMP-SHIPPED" 0 "$(sum_val "$f" SECCOMP-SHIPPED)"
  st_eq "summarize b: ENGINE-SHIPPED" compiler "$(sum_val "$f" ENGINE-SHIPPED)"
  st_eq "summarize b: MDWE-WORD" VOID "$(sum_val "$f" MDWE-WORD)"
  st_eq "summarize b: SRV-01-MDWE" NOT-NATIVE "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize b: SRV-01" NOT-NATIVE "$(sum_word "$f" SRV-01)"
  st_eq "summarize b: last line" "NATIVE-AMD64 DONE" "$(tail -n 1 "$f")"

  # (c) a SYNTHETIC native set: R2 turned into x86_64 hosts, with the real chain summary.
  mkdir -p "$t/c"
  fx_mdwe_synthetic "$t/c/p-mdwe"
  mkdir -p "$t/c/p-bak-src"
  fx_chain "$t/c/p-bak-src/summary.txt"
  rc=$(st_summarize "$t/c" 0 0)
  f=$t/c/p/native-summary.txt
  st_eq "summarize c: does not abort" 0 "$rc"
  st_eq "summarize c: HOST-ARCH" x86_64 "$(sum_val "$f" HOST-ARCH)"
  st_eq "summarize c: CONTAINER-ARCH" x86_64 "$(sum_val "$f" CONTAINER-ARCH)"
  st_eq "summarize c: SRV-01-MDWE" PASS "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize c: SRV-01-TIMING" PASS "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize c: SRV-01" PASS "$(sum_word "$f" SRV-01)"
  st_eq "summarize c: last line" "NATIVE-AMD64 DONE" "$(tail -n 1 "$f")"
  st_eq "summarize c: exit code of the verdict" 0 "$(srv01_exit "$(overall_of "$f")")"
  st_eq "summarize c: BAK-01 copied verbatim" "BAK-01 PASS" "$(grep -x 'BAK-01 PASS' "$f")"
  st_eq "summarize c: POINT seed copied verbatim" \
    "$(grep '^POINT seed:' "$t/c/p-bak-src/summary.txt")" "$(grep '^POINT seed:' "$f")"
  st_eq "summarize c: POINT bulk copied verbatim" \
    "$(grep '^POINT bulk:' "$t/c/p-bak-src/summary.txt")" "$(grep '^POINT bulk:' "$f")"
  st_eq "summarize c: STEPS copied" "STEPS: 78 played, 0 failed" "$(grep '^STEPS:' "$f")"
  st_eq "summarize c: LEAK-CHECK copied" "LEAK-CHECK OK" "$(grep '^LEAK-CHECK ' "$f")"
  st_eq "summarize c: the header of the chain summary is not copied" 0 "$(grep -c '^---- summary' "$f" || true)"
  # the native summary is itself a valid timing input
  st_eq "summarize c: srv01_timing accepts the native summary" PASS "$(srv01_timing "$f")"

  # (d) the synthetic set whose canary says not-enforced, with the R1 verdict line.
  mkdir -p "$t/d"
  fx_mdwe_synthetic "$t/d/p-mdwe"
  echo not-enforced > "$t/d/p-mdwe/canary.verdict"
  fx_verdict_r1 "$t/d/p-mdwe/verdict.txt"
  mkdir -p "$t/d/p-bak-src"
  fx_chain "$t/d/p-bak-src/summary.txt"
  rc=$(st_summarize "$t/d" 3 0)
  f=$t/d/p/native-summary.txt
  st_eq "summarize d: does not abort" 0 "$rc"
  st_eq "summarize d: CANARY" not-enforced "$(sum_val "$f" CANARY)"
  st_eq "summarize d: SRV-01-MDWE" VOID "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize d: SRV-01-TIMING" PASS "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize d: SRV-01" VOID "$(sum_word "$f" SRV-01)"
  st_eq "summarize d: exit code of the verdict" 3 "$(srv01_exit "$(overall_of "$f")")"

  # (e) the synthetic set with a FAILS verdict line.
  mkdir -p "$t/e"
  fx_mdwe_synthetic "$t/e/p-mdwe"
  fx_verdict_fails "$t/e/p-mdwe/verdict.txt"
  mkdir -p "$t/e/p-bak-src"
  fx_chain "$t/e/p-bak-src/summary.txt"
  rc=$(st_summarize "$t/e" 1 0)
  f=$t/e/p/native-summary.txt
  st_eq "summarize e: does not abort" 0 "$rc"
  st_eq "summarize e: MDWE-WORD" FAILS "$(sum_val "$f" MDWE-WORD)"
  st_eq "summarize e: SRV-01-MDWE" FAIL "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize e: SRV-01" FAIL "$(sum_word "$f" SRV-01)"
  st_eq "summarize e: exit code of the verdict" 1 "$(srv01_exit "$(overall_of "$f")")"

  # (f) the synthetic set with the chain summary missing.
  mkdir -p "$t/f"
  fx_mdwe_synthetic "$t/f/p-mdwe"
  rc=$(st_summarize "$t/f" 0 1)
  f=$t/f/p/native-summary.txt
  st_eq "summarize f: does not abort" 0 "$rc"
  st_eq "summarize f: SRV-01-MDWE" PASS "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize f: SRV-01-TIMING" "FAIL (no chain summary)" "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize f: SRV-01" FAIL "$(sum_word "$f" SRV-01)"
  st_eq "summarize f: last line" "NATIVE-AMD64 DONE" "$(tail -n 1 "$f")"

  # (g) both directories missing, both stages reported as having ended in failure.
  mkdir -p "$t/g"
  rc=$(st_summarize "$t/g" 1 1)
  f=$t/g/p/native-summary.txt
  st_eq "summarize g: does not abort" 0 "$rc"
  st_eq "summarize g: HOST-ARCH" "?" "$(sum_val "$f" HOST-ARCH)"
  st_eq "summarize g: CANARY" missing "$(sum_val "$f" CANARY)"
  st_eq "summarize g: SECCOMP-SHIPPED" "?" "$(sum_val "$f" SECCOMP-SHIPPED)"
  st_eq "summarize g: ASSERTIONS-SHIPPED" "?" "$(sum_val "$f" ASSERTIONS-SHIPPED)"
  st_eq "summarize g: MDWE-WORD" MISSING "$(sum_val "$f" MDWE-WORD)"
  st_eq "summarize g: SRV-01-MDWE" NOT-NATIVE "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize g: SRV-01-TIMING" "FAIL (no chain summary)" "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize g: SRV-01 (NOT-NATIVE outranks FAIL)" NOT-NATIVE "$(sum_word "$f" SRV-01)"
  st_eq "summarize g: last line" "NATIVE-AMD64 DONE" "$(tail -n 1 "$f")"

  # (h) neither stage played.
  mkdir -p "$t/h"
  rc=$(st_summarize "$t/h" notrun notrun)
  f=$t/h/p/native-summary.txt
  st_eq "summarize h: does not abort" 0 "$rc"
  st_eq "summarize h: SRV-01-MDWE" NOT-RUN "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize h: SRV-01-TIMING" NOT-RUN "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize h: SRV-01" NOT-RUN "$(sum_word "$f" SRV-01)"
  st_eq "summarize h: exit code of the verdict" 1 "$(srv01_exit "$(overall_of "$f")")"

  # (i) the mdwe stage not played, the timing stage played on a native set: NOT-RUN, never PASS.
  mkdir -p "$t/i"
  mkdir -p "$t/i/p-bak-src"
  fx_chain "$t/i/p-bak-src/summary.txt"
  rc=$(st_summarize "$t/i" notrun 0)
  f=$t/i/p/native-summary.txt
  st_eq "summarize i: SRV-01-MDWE" NOT-RUN "$(sum_word "$f" SRV-01-MDWE)"
  st_eq "summarize i: SRV-01-TIMING" PASS "$(sum_word "$f" SRV-01-TIMING)"
  st_eq "summarize i: SRV-01" NOT-RUN "$(sum_word "$f" SRV-01)"

  # (j) the summarize subcommand reads the stage statuses from MDWE_RC and TIMING_RC.
  rc=$(
    BASE_OUT=$t/c NATIVE_PREFIX=p MDWE_RC=0 TIMING_RC=0
    docker_line() { echo '?'; }
    cmd_summarize > /dev/null
    sum_word "$t/c/p/native-summary.txt" SRV-01
  )
  st_eq "cmd_summarize with both statuses" PASS "$rc"
  rc=$(
    BASE_OUT=$t/c NATIVE_PREFIX=p
    unset MDWE_RC TIMING_RC
    docker_line() { echo '?'; }
    cmd_summarize > /dev/null
    sum_word "$t/c/p/native-summary.txt" SRV-01
  )
  st_eq "cmd_summarize with no status reads not played" NOT-RUN "$rc"
}

selftest_stages() {
  local s
  for s in "mdwe" "timing" "mdwe timing" "timing mdwe"; do
    NATIVE_STAGES=$s
    st_ok "stages_valid '$s'" stages_valid
  done
  for s in "" "bogus" "mdwe bogus" "all"; do
    NATIVE_STAGES=$s
    st_fail "stages_valid '$s'" stages_valid
  done
  NATIVE_STAGES="mdwe timing"
  st_ok "stage_wanted mdwe" stage_wanted mdwe
  st_ok "stage_wanted timing" stage_wanted timing
  NATIVE_STAGES=mdwe
  st_ok "stage_wanted mdwe alone" stage_wanted mdwe
  st_fail "stage_wanted timing absent" stage_wanted timing
  NATIVE_STAGES="mdwe timing"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/native-amd64-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_pure
  selftest_summarize
  selftest_stages
  if [ "$ST_BAD" -ne 0 ]; then
    say "SELFTEST FAILED: $ST_BAD of $ST_N cases"
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
    summarize) cmd_summarize ;;
    run) cmd_run ;;
    down) cmd_down ;;
    *) usage ;;
  esac
done
