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
#   native-amd64.sh record-check   the proof of deploy/NATIVE-AMD64-RUN.md (R, E): pending owner without a
#                               claim, recorded only with the matching evidence (plan 26-02)
#   native-amd64.sh status-check   the record held to .planning/REQUIREMENTS.md (R, REQS)
#   native-amd64.sh appendix    the appendix blocks of the fixed paste list, from the files below E
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
#   R, E, REQS      deploy/NATIVE-AMD64-RUN.md, deploy/test/out and .planning/REQUIREMENTS.md: the record, the
#                   scratch root and the requirements file of record-check, status-check and appendix
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
  sed -n '2,61p' "${BASH_SOURCE[0]}" >&2
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

# bundle: the evidence below BASE_OUT copied to EVIDENCE_DIR, minus everything that
# must never be handed over, and refused (exit 1, nothing left behind) when what
# remains looks like a secret. The layout is DIRECTORY/FILE, so that unpacking the
# bundle into deploy/test/out puts every file where the record expects it.
#
# Never kept: step-NN.txt, seed.txt, *.log, cookie jars, id_*, *.pem, *.key. The
# scan for what remains is a floor, not a promise: a private key marker and an
# AI-key-shaped string, the same two things run.sh's leak_check looks for. The
# harness's own LEAK-CHECK OK in the chain summary is the first line of defence
# and is copied into native-summary.txt.
#
# The copy is made in a temporary sibling and renamed into place only when the
# scan has passed, so a refused bundle leaves nothing.
bundle() {
  local ev=${EVIDENCE_DIR%/} tmp d found=0 hit kept
  guard_dir "$ev"
  tmp=$ev.tmp.$$
  rm -rf "${tmp:?}"
  mkdir -p "$tmp"
  for d in $(dir_names); do
    if [ -d "$BASE_OUT/$d" ]; then
      cp -R "$BASE_OUT/$d" "$tmp/$d"
      found=$((found + 1))
    fi
  done
  if [ "$found" -eq 0 ]; then
    rm -rf "${tmp:?}" "${ev:?}"
    say "native-amd64.sh: bundle: none of the evidence directories exists below $BASE_OUT; an empty artifact is not evidence"
    return 1
  fi
  find "$tmp" -type f \( -name 'step-*.txt' -o -name 'seed.txt' -o -name '*.log' -o -iname '*cookie*' \
    -o -name 'id_*' -o -name '*.pem' -o -name '*.key' \) -exec rm -f {} +
  # A link would be followed by the upload; only plain files and directories travel.
  find "$tmp" ! -type f ! -type d -exec rm -f {} +
  hit=$(grep -rlE 'PRIVATE KEY' "$tmp" | head -n 1) || true
  if [ -n "$hit" ]; then
    rm -rf "${tmp:?}" "${ev:?}"
    say "native-amd64.sh: bundle refused: ${hit#"$tmp"/} contains a private key marker; no bundle was left"
    return 1
  fi
  hit=$(grep -rlE 'hc_[A-Za-z0-9_-]{20,}' "$tmp" | head -n 1) || true
  if [ -n "$hit" ]; then
    rm -rf "${tmp:?}" "${ev:?}"
    say "native-amd64.sh: bundle refused: ${hit#"$tmp"/} contains a string shaped like an AI key; no bundle was left"
    return 1
  fi
  kept=$(find "$tmp" -type f | wc -l | tr -d ' ')
  if [ "$kept" -eq 0 ]; then
    rm -rf "${tmp:?}" "${ev:?}"
    say "native-amd64.sh: bundle: nothing is left after the exclusions; an empty artifact is not evidence"
    return 1
  fi
  rm -rf "${ev:?}"
  mv "$tmp" "$ev"
  echo "BUNDLE: $kept files kept in $ev"
}

# write_job_summary: the native summary as a fenced text block on the job summary
# page, when GITHUB_STEP_SUMMARY names a file that can be appended to. Elsewhere it
# does nothing, and a page that cannot be written is a warning, not a failure.
write_job_summary() {
  local f=${GITHUB_STEP_SUMMARY:-} s
  [ -n "$f" ] || return 0
  s=$(d_self)/native-summary.txt
  [ -f "$s" ] || return 0
  {
    printf '%s\n' '```text'
    cat "$s"
    printf '%s\n' '```'
  } 2> /dev/null >> "$f" || say "native-amd64.sh: the job summary page $f could not be written"
  return 0
}

# run_publish: after the summary is written, the job summary page and the evidence
# bundle. A refused bundle is a failure the caller turns into exit 1.
run_publish() {
  local rc=0
  write_job_summary
  bundle || rc=1
  return "$rc"
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

  # run.sh plays the scp lines with a bsdtar flag (--no-mac-metadata) that GNU
  # tar on the runner refuses (rc 64, found by the first run, 41197606780). The
  # harness is not edited (D-02); a tar in front of PATH drops that one flag.
  mkdir -p "$(d_self)/shim"
  REAL_TAR=$(command -v tar) || die "tar is not installed"
  {
    printf '#!/bin/sh\n'
    printf 'for a in "$@"; do shift; [ "$a" = --no-mac-metadata ] || set -- "$@" "$a"; done\n'
    printf 'exec %s "$@"\n' "$REAL_TAR"
  } > "$(d_self)/shim/tar"
  chmod 755 "$(d_self)/shim/tar"

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
    env PATH="$(d_self)/shim:$PATH" PLATFORM="$PLATFORM" TARGET=systemd-base NAME="holzcloud-$NATIVE_PREFIX-bak-src" OUT_DIR="$(d_src)" \
      TGT_NAME="holzcloud-$NATIVE_PREFIX-bak-tgt" TGT_OUT_DIR="$(d_tgt)" BULK_MB="$BULK_MB" NEW_REF="$NEW_REF" \
      DOCKER_RUN_ARGS="$DOCKER_RUN_ARGS" bash "$ROOT/deploy/test/run.sh" backup-chain || rc=$?
    timing_rc=$rc
    say "native-amd64: run.sh backup-chain ended with status $timing_rc"
  fi

  summarize "$mdwe_rc" "$timing_rc"
  sum=$(d_self)/native-summary.txt
  ov=$(overall_of "$sum")
  rc=$(srv01_exit "$ov")
  if ! run_publish; then
    if [ "$rc" -eq 0 ]; then
      rc=1
      say "native-amd64: the verdict is PASS, but the evidence bundle was refused or could not be made: the job is red"
    fi
  fi
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
# The record (plan 26-02): summary_check, srv_record_check, status_check, appendix.
#
# deploy/NATIVE-AMD64-RUN.md has two honest states. "pending owner" claims nothing:
# no result, no sha, no appendix, and no pass word anywhere outside a fenced block.
# "recorded" is accepted only with the matching evidence. These functions never rely
# on `set -e` (they are called from the left of `||`), print one line per problem and
# end on an answer line. The awk programs are POSIX; a fence is a line that begins
# with three backticks.
# ---------------------------------------------------------------------------

# native_paste_list: DIRECTORY/FILE of the appendix, in order. A list, never a glob:
# step-NN.txt, seed.txt, logs and cookie jars are not on it and are never pasted.
native_paste_list() {
  printf '%s\n' \
    amd64-native/native-summary.txt \
    amd64-native/runner.txt \
    amd64-native-mdwe/versions.txt \
    amd64-native-mdwe/canary.txt \
    amd64-native-mdwe/check.txt \
    amd64-native-mdwe/mdwe-control.txt \
    amd64-native-mdwe/verdict.txt \
    amd64-native-bak-src/summary.txt \
    amd64-native-bak-src/backup.txt \
    amd64-native-bak-src/versions.txt \
    amd64-native-bak-tgt/restore.txt \
    amd64-native-bak-src-bulk/backup.txt \
    amd64-native-bak-tgt-bulk/restore.txt
}

# native_markers REQ: the strings a recorded result of REQ has to hold, one per line
# (CONTEXT.md, Specifics: the single source). The owner steps echo them, so that the
# owner's paste contains them. For SRV-01 the first is a line of the summary and the
# second its last line; summary_check proves the rest.
native_markers() {
  case "${1:-}" in
    SRV-01) printf '%s\n' 'SRV-01 PASS' 'NATIVE-AMD64 DONE' ;;
    SRV-02) printf '%s\n' '=== Backup complete: ' '=== Restore complete ===' 'SRV-02 INSTANCE READY' 'SRV-02 TREES EQUAL' ;;
    SRV-03) printf '%s\n' 'SRV-03 UNITS ENABLED' 'SRV-03 BOOT CHANGED' 'SRV-03 UNITS ACTIVE' 'SRV-03 SITE READY' 'SRV-03 TIMER ARMED' ;;
  esac
}

# claimed_word FILE NAME: the first word after "NAME " on the first such line; empty when absent.
claimed_word() {
  local v=""
  v=$(sed -n "s/^$2 //p" "$1" 2> /dev/null | head -n 1) || true
  printf '%s\n' "${v%% *}"
}

# summary_check FILE: FILE is a native summary as `summarize` writes it, and its claimed
# verdicts are the ones its own lines give. One `SUMMARY BAD:` line per problem, then
# `SUMMARY OK: SRV-01 WORD` (return 0) or `SUMMARY FAILED: N problem(s)` (return 1).
summary_check() {
  local f=${1:-} bad=0 key n first last host cont word mst tst mv tv ov canary c
  if [ -z "$f" ] || [ ! -s "$f" ]; then
    echo "SUMMARY BAD: there is no summary text"
    echo "SUMMARY FAILED: 1 problem(s)"
    return 1
  fi
  first=$(head -n 1 "$f")
  if [ "$first" != "---- native amd64 summary ----" ]; then
    echo "SUMMARY BAD: the first line is '$first', not '---- native amd64 summary ----'"
    bad=$((bad + 1))
  fi
  last=$(tail -n 1 "$f")
  if [ "$last" != "NATIVE-AMD64 DONE" ]; then
    echo "SUMMARY BAD: the last line is '$last', not 'NATIVE-AMD64 DONE'"
    bad=$((bad + 1))
  fi
  for key in COMMIT HOST-ARCH CONTAINER-ARCH CANARY MDWE-WORD MDWE-STATUS TIMING-STATUS; do
    n=$(grep -c "^$key: " "$f" || true)
    if [ "$n" -ne 1 ]; then
      echo "SUMMARY BAD: $n '$key: ' lines, exactly one is required"
      bad=$((bad + 1))
    fi
  done
  for key in SRV-01-MDWE SRV-01-TIMING SRV-01; do
    n=$(grep -c "^$key " "$f" || true)
    if [ "$n" -ne 1 ]; then
      echo "SUMMARY BAD: $n '$key ' verdict lines, exactly one is required"
      bad=$((bad + 1))
    fi
  done
  if [ "$bad" -ne 0 ]; then
    echo "SUMMARY FAILED: $bad problem(s)"
    return 1
  fi

  # The verdicts are recomputed from the lines the summary displays, with the functions
  # that wrote them; a claim that differs from the recomputation is a forgery or a typo.
  host=$(file_value "$f" HOST-ARCH)
  cont=$(file_value "$f" CONTAINER-ARCH)
  canary=$(file_value "$f" CANARY)
  word=$(file_value "$f" MDWE-WORD)
  mst=$(file_value "$f" MDWE-STATUS)
  tst=$(file_value "$f" TIMING-STATUS)
  if [ "$mst" = notrun ]; then
    mv=NOT-RUN
  else
    mv=$(srv01_mdwe "$host" "$cont" "$word")
  fi
  if [ "$tst" = notrun ]; then
    tv=NOT-RUN
  else
    tv=$(srv01_timing "$f")
  fi
  mv=${mv%% *}
  tv=${tv%% *}
  ov=$(srv01_overall "$mv" "$tv")
  c=$(claimed_word "$f" SRV-01-MDWE)
  if [ "$c" != "$mv" ]; then
    echo "SUMMARY BAD: SRV-01-MDWE is claimed '$c', the lines of the summary give '$mv'"
    bad=$((bad + 1))
  fi
  c=$(claimed_word "$f" SRV-01-TIMING)
  if [ "$c" != "$tv" ]; then
    echo "SUMMARY BAD: SRV-01-TIMING is claimed '$c', the lines of the summary give '$tv'"
    bad=$((bad + 1))
  fi
  c=$(claimed_word "$f" SRV-01)
  if [ "$c" != "$ov" ]; then
    echo "SUMMARY BAD: SRV-01 is claimed '$c', the lines of the summary give '$ov'"
    bad=$((bad + 1))
  fi
  if [ "$mv" = PASS ] && [ "$canary" != enforced ]; then
    echo "SUMMARY BAD: SRV-01-MDWE is PASS but CANARY is '$canary', only 'enforced' makes the plugin run count"
    bad=$((bad + 1))
  fi
  if [ "$bad" -ne 0 ]; then
    echo "SUMMARY FAILED: $bad problem(s)"
    return 1
  fi
  echo "SUMMARY OK: SRV-01 $ov"
}

# rc_section FILE HEADING: the lines below the heading line that begins with HEADING (a
# "## ..." text) up to the next "## " line outside a fence; the heading is not printed.
rc_section() {
  awk -v p="$2" '
    { isf = (substr($0, 1, 3) == "```") }
    !fence && !isf && substr($0, 1, 3) == "## " {
      if (insec) exit
      if (index($0, p) == 1) { insec = 1; next }
    }
    insec { print }
    isf { fence = !fence }
  ' "$1"
}

# rc_prose: stdin without its fenced blocks and without the fence lines.
rc_prose() {
  awk '{ if (substr($0, 1, 3) == "```") { fence = !fence; next } if (!fence) print }'
}

# rc_result_block: stdin is a requirement section; prints the inside of the first fenced
# block after its "Result (owner): " line (fences before that line are skipped).
# Status 0 when such a block was found and closed, 1 otherwise.
rc_result_block() {
  awk '
    { isf = (substr($0, 1, 3) == "```") }
    state == 0 { if (isf) { fence = !fence } else if (!fence && index($0, "Result (owner): ") == 1) { state = 1 } next }
    state == 1 { if (isf) { state = 2 } next }
    state == 2 { if (isf) { closed = 1; exit } print }
    END { exit (closed ? 0 : 1) }
  '
}

# rc_step N: stdin is the Owner steps section; prints step N, from its "**O-N. " line to
# the next step line or the "**Recording a result" paragraph.
rc_step() {
  awk -v n="$1" '
    index($0, "**O-" n ". ") == 1 { cur = 1; print; next }
    /^\*\*O-[0-9]+\. / || /^\*\*Recording a result/ { cur = 0 }
    cur { print }
  '
}

# rc_status_of RECORD ID: "pending owner" or "recorded" when the section has exactly one
# valid Status line, nothing otherwise.
rc_status_of() {
  local prose n v=""
  prose=$(rc_section "$1" "## $2" | rc_prose)
  n=$(printf '%s\n' "$prose" | grep -c '^Status: ' || true)
  if [ "$n" -eq 1 ]; then
    v=$(printf '%s\n' "$prose" | sed -n 's/^Status: //p' | sed 's/[[:space:]]*$//')
    case "$v" in
      'pending owner' | recorded) ;;
      *) v="" ;;
    esac
  fi
  printf '%s\n' "$v"
}

RC_BAD=0
rc_bad() {
  echo "RECORD-CHECK BAD: $*"
  RC_BAD=$((RC_BAD + 1))
}

# rc_same A B: the two files are equal once a missing final newline is normalised.
rc_same() {
  local a b
  a=$(mktemp) || return 1
  b=$(mktemp) || return 1
  awk 1 "$1" > "$a"
  awk 1 "$2" > "$b"
  if cmp -s "$a" "$b"; then
    rm -f "$a" "$b"
    return 0
  fi
  rm -f "$a" "$b"
  return 1
}

# srv_record_check R E: the proof of the record. Ends `RECORD-CHECK OK N blocks; recorded:
# LIST; pending owner: LIST` or `RECORD-CHECK FAILED: N problem(s)`.
srv_record_check() {
  local r=${1:-deploy/NATIVE-AMD64-RUN.md} e=${2:-deploy/test/out}
  local t id sec prose ns nr st rv want have n h m name sha bcommit out bads ln os stext
  local sv1="" nblocks=0 nheads=0 recorded="" pending="" sumprose rows row k
  RC_BAD=0
  if [ ! -f "$r" ]; then
    echo "RECORD-CHECK BAD: the record $r does not exist"
    echo "RECORD-CHECK FAILED: 1 problem(s)"
    return 1
  fi
  t=$(mktemp -d) || return 1
  awk '/^## Appendix: raw output$/{exit} {print}' "$r" > "$t/body"

  # The headings, each alone on its line; the requirement sections by prefix.
  while IFS= read -r h; do
    if ! grep -qxF -- "$h" "$r"; then
      rc_bad "the heading '$h' is missing"
    fi
  done << 'EOF'
## Summary
## How this was prepared
## Container vs. server
## Not tested
## Owner steps
## Hand-over to phase 28
## Appendix: raw output
EOF
  for id in SRV-01 SRV-02 SRV-03; do
    if ! grep -q "^## $id" "$r"; then
      rc_bad "no heading beginning '## $id'"
    fi
  done

  if grep -q 'PRIVATE KEY' "$r"; then
    rc_bad "a private key marker occurs in the record"
  fi

  sumprose=$(rc_section "$r" "## Summary" | rc_prose)
  rc_prose < "$r" > "$t/prose"

  # One requirement at a time: its status, its result slot, its paste block, its Summary row.
  for id in SRV-01 SRV-02 SRV-03; do
    sec=$(rc_section "$r" "## $id")
    prose=$(printf '%s\n' "$sec" | rc_prose)
    ns=$(printf '%s\n' "$prose" | grep -c '^Status: ' || true)
    nr=$(printf '%s\n' "$prose" | grep -c '^Result (owner): ' || true)
    st=""
    rv=""
    if [ "$ns" -ne 1 ]; then
      rc_bad "$id has $ns 'Status: ' lines, exactly one is required"
    else
      st=$(printf '%s\n' "$prose" | sed -n 's/^Status: //p' | sed 's/[[:space:]]*$//')
      case "$st" in
        'pending owner' | recorded) ;;
        *)
          rc_bad "$id has the unknown status '$st' (the values are 'pending owner' and 'recorded')"
          st=""
          ;;
      esac
    fi
    if [ "$nr" -ne 1 ]; then
      rc_bad "$id has $nr 'Result (owner): ' lines, exactly one is required"
    else
      rv=$(printf '%s\n' "$prose" | sed -n 's/^Result (owner): //p' | sed 's/[[:space:]]*$//')
      case "$rv" in
        pending | recorded) ;;
        *)
          rc_bad "$id has the unknown 'Result (owner): $rv' (the values are 'pending' and 'recorded')"
          rv=""
          ;;
      esac
    fi
    if [ -n "$st" ] && [ -n "$rv" ]; then
      want=pending
      if [ "$st" = recorded ]; then want=recorded; fi
      if [ "$rv" != "$want" ]; then
        rc_bad "$id: 'Result (owner): $rv' does not match 'Status: $st'"
      fi
    fi
    if [ "$id" = SRV-01 ]; then sv1=$st; fi

    have=1
    if ! printf '%s\n' "$sec" | rc_result_block > "$t/paste-$id"; then
      have=0
      : > "$t/paste-$id"
      rc_bad "$id has no closed fenced paste block after its 'Result (owner): ' line"
    fi

    # The Summary row.
    rows=$(printf '%s\n' "$sumprose" | grep "^| $id |" || true)
    n=$(printf '%s\n' "$rows" | grep -c . || true)
    if [ "$n" -ne 1 ]; then
      rc_bad "the Summary table has $n rows beginning '| $id |', exactly one is required"
      row=""
    else
      row=$rows
    fi

    if [ "$st" = "pending owner" ]; then
      pending="$pending $id"
      if [ "$have" -eq 1 ]; then
        n=$(awk 'NF && $0 != "(nothing pasted yet)" { n++ } END { print n + 0 }' "$t/paste-$id")
        if [ "$n" -ne 0 ]; then
          rc_bad "$id is pending owner but its paste block holds text; a result is pasted together with Status: recorded"
        fi
      fi
      if [ -n "$row" ]; then
        if ! printf '%s\n' "$row" | grep -q 'PENDING OWNER'; then
          rc_bad "the Summary row of $id is pending owner but does not say PENDING OWNER"
        fi
        if printf '%s\n' "$row" | grep -Eiwq 'pass|passed|passes'; then
          rc_bad "the Summary row of $id is pending owner and holds a pass word"
        fi
      fi
      if grep -Eq "^[[:space:]>*-]*$id PASS[[:space:]]*\$" "$t/prose"; then
        rc_bad "a line '$id PASS' stands outside a fenced block while $id is pending owner"
      fi
    elif [ "$st" = recorded ]; then
      recorded="$recorded $id"
      if [ -n "$row" ] && printf '%s\n' "$row" | grep -q 'PENDING OWNER'; then
        rc_bad "the Summary row of $id is recorded but still says PENDING OWNER"
      fi
      if [ "$id" = SRV-01 ]; then
        out=$(summary_check "$t/paste-$id" 2>&1) || true
        if ! printf '%s\n' "$out" | grep -q '^SUMMARY OK: SRV-01 PASS$'; then
          bads=$(printf '%s\n' "$out" | grep '^SUMMARY BAD:' || true)
          if [ -n "$bads" ]; then
            while IFS= read -r ln; do
              rc_bad "SRV-01 recorded: ${ln#SUMMARY BAD: }"
            done << EOF
$bads
EOF
          else
            ln=$(printf '%s\n' "$out" | sed -n 's/^SUMMARY OK: SRV-01 //p')
            rc_bad "SRV-01 recorded: the verdict of the summary is ${ln:-unreadable}; only PASS can be recorded (a run that did not satisfy the requirement is a finding and leaves the status pending)"
          fi
        fi
      elif [ "$have" -eq 1 ]; then
        while IFS= read -r m; do
          if ! awk -v m="$m" 'index($0, m) == 1 { f = 1 } END { exit (f ? 0 : 1) }' "$t/paste-$id"; then
            rc_bad "$id recorded: the result block has no line that begins '$m'"
          fi
        done << EOF
$(native_markers "$id")
EOF
      fi
    fi
  done

  # The evidence commit: one line, none while SRV-01 is pending, a sha git knows while it is recorded.
  n=$(grep -c '^Evidence commit: ' "$t/body" || true)
  if [ "$n" -ne 1 ]; then
    rc_bad "the record has $n 'Evidence commit: ' lines outside the appendix, exactly one is required"
  else
    sha=$(sed -n 's/^Evidence commit: //p' "$t/body" | sed 's/[[:space:]]*$//')
    if [ "$sha" = "none (pending owner)" ]; then
      if [ "$sv1" = recorded ]; then
        rc_bad "the Evidence commit line says 'none (pending owner)' while SRV-01 is recorded"
      fi
    elif ! printf '%s\n' "$sha" | grep -qE '^[0-9a-f]{7,40}$'; then
      rc_bad "the Evidence commit line is malformed: '$sha' is neither 'none (pending owner)' nor 7 to 40 hex characters"
    elif [ "$sv1" = "pending owner" ]; then
      rc_bad "the Evidence commit line holds the sha $sha while SRV-01 is pending owner"
    elif [ "$sv1" = recorded ]; then
      if ! git cat-file -e "${sha}^{commit}" 2> /dev/null; then
        rc_bad "the Evidence commit $sha is unknown to this repository"
      else
        bcommit=$(file_value "$t/paste-SRV-01" COMMIT)
        case "$bcommit" in
          "$sha"*) ;;
          *) rc_bad "the Evidence commit $sha is not a prefix of the COMMIT of the pasted summary ($bcommit)" ;;
        esac
      fi
    fi
  fi

  # The appendix, extracted the way deploy/test/README.md describes it.
  awk -v d="$t" '/^## Appendix: raw output$/{app=1;next} !app{next} /^#### /{name=$2;want=1;next} want && /^```/{ if(!inb){inb=1;n++;f=d "/b" n;print n " " name >> (d "/index");printf "" > f;next} else {inb=0;want=0;close(f);next} } inb{print >> f}' "$r"
  : >> "$t/index"
  awk '/^## Appendix: raw output$/{a=1;next} a && /^#### /{print $2}' "$r" > "$t/heads"
  nheads=$(grep -c . "$t/heads" || true)
  nblocks=$(grep -c . "$t/index" || true)
  n=$(awk '/^## Appendix: raw output$/{a=1;next} a && (/^#### / || /^```/){c++} END{print c+0}' "$r")
  if [ "$sv1" = "pending owner" ]; then
    if [ "$n" -ne 0 ]; then
      rc_bad "the appendix has a '#### ' heading or a fenced block while SRV-01 is pending owner; it stays empty until SRV-01 is recorded"
    fi
  elif [ "$sv1" = recorded ]; then
    if [ "$nheads" -ne "$nblocks" ]; then
      rc_bad "the appendix has $nheads '#### ' headings and $nblocks fenced blocks: a heading has no block, or a block is not closed"
    fi
    if [ "$nblocks" -eq 0 ]; then
      rc_bad "SRV-01 is recorded and the appendix is empty; the files of the pasted list are the evidence"
    fi
    while read -r k name; do
      [ -n "$k" ] || continue
      if ! native_paste_list | grep -qxF -- "$name"; then
        rc_bad "the appendix block $name is not on the fixed paste list (bash deploy/test/native-amd64.sh appendix prints the list)"
        continue
      fi
      if [ ! -f "$e/$name" ]; then
        rc_bad "no scratch file for the appendix block $name (looked for $e/$name)"
        continue
      fi
      if ! rc_same "$t/b$k" "$e/$name"; then
        rc_bad "the block differs from the scratch file: $name"
      fi
      if [ "$name" = "amd64-native/native-summary.txt" ] && ! rc_same "$t/b$k" "$t/paste-SRV-01"; then
        rc_bad "the native-summary.txt block differs from the Result (owner) block of SRV-01"
      fi
    done < "$t/index"
    if ! grep -qxF "amd64-native/native-summary.txt" "$t/heads"; then
      rc_bad "the appendix has no block for amd64-native/native-summary.txt"
    fi
    n=$(sort "$t/heads" | uniq -d | head -n 1)
    if [ -n "$n" ]; then
      rc_bad "the appendix has two blocks for $n"
    fi
  fi

  # The Owner steps: O-1 to O-3, each complete, each holding the markers it has to produce.
  os=$(rc_section "$r" "## Owner steps")
  for k in 1 2 3; do
    stext=$(printf '%s\n' "$os" | rc_step "$k")
    if [ -z "$stext" ]; then
      rc_bad "the Owner steps have no step beginning '**O-$k. '"
      continue
    fi
    if ! printf '%s\n' "$stext" | grep -q '^```'; then
      rc_bad "step O-$k has no fenced command block"
    fi
    if ! printf '%s\n' "$stext" | grep -q 'Expect'; then
      rc_bad "step O-$k has no 'Expect' paragraph"
    fi
    if ! printf '%s\n' "$stext" | grep -q 'different output'; then
      rc_bad "step O-$k does not say what a different output means (the words 'different output')"
    fi
    if ! printf '%s\n' "$stext" | grep -q 'Paste'; then
      rc_bad "step O-$k does not say where the result is pasted (the word 'Paste')"
    fi
    while IFS= read -r m; do
      if ! printf '%s\n' "$stext" | grep -qF -- "$m"; then
        rc_bad "step O-$k does not hold the marker '$m' that a recorded SRV-0$k result has to contain"
      fi
    done << EOF
$(native_markers "SRV-0$k")
EOF
  done

  rm -rf "${t:?}"
  if [ "$RC_BAD" -ne 0 ]; then
    echo "RECORD-CHECK FAILED: $RC_BAD problem(s)"
    return 1
  fi
  pending=${pending# }
  recorded=${recorded# }
  echo "RECORD-CHECK OK $nblocks blocks; recorded: ${recorded:-none}; pending owner: ${pending:-none}"
}

# status_check R REQS: REQUIREMENTS.md says what the record says.
#   pending owner: the line "- [ ] **SRV-0N**" and a traceability cell "Pending owner"
#   recorded:      the line "- [x] **SRV-0N**" and the cell "Complete"
status_check() {
  local r=${1:-deploy/NATIVE-AMD64-RUN.md} q=${2:-.planning/REQUIREMENTS.md} bad=0 id st want_box want_cell line box row cell
  if [ ! -f "$r" ]; then
    echo "STATUS-CHECK BAD: the record $r does not exist"
    echo "STATUS-CHECK FAILED: 1 problem(s)"
    return 1
  fi
  if [ ! -f "$q" ]; then
    echo "STATUS-CHECK BAD: the requirements file $q does not exist"
    echo "STATUS-CHECK FAILED: 1 problem(s)"
    return 1
  fi
  for id in SRV-01 SRV-02 SRV-03; do
    st=$(rc_status_of "$r" "$id")
    case "$st" in
      "pending owner")
        want_box=' '
        want_cell="Pending owner"
        ;;
      recorded)
        want_box=x
        want_cell=Complete
        ;;
      *)
        echo "STATUS-CHECK BAD: $id has no readable Status in the record (exactly one 'Status: pending owner' or 'Status: recorded' line)"
        bad=$((bad + 1))
        continue
        ;;
    esac
    line=$(grep "^- \[.\] \*\*$id\*\*" "$q" | head -n 1 || true)
    if [ -z "$line" ]; then
      echo "STATUS-CHECK BAD: $id is $st in the record and REQUIREMENTS has no line '- [ ] **$id**'"
      bad=$((bad + 1))
    else
      box=$(printf '%s\n' "$line" | sed 's/^- \[\(.\)\].*/\1/')
      if [ "$box" != "$want_box" ]; then
        echo "STATUS-CHECK BAD: $id is $st in the record and the box in REQUIREMENTS is [$box], expected [$want_box]"
        bad=$((bad + 1))
      fi
    fi
    row=$(grep "^| $id |" "$q" | head -n 1 || true)
    if [ -z "$row" ]; then
      echo "STATUS-CHECK BAD: $id is $st in the record and REQUIREMENTS has no traceability row '| $id |'"
      bad=$((bad + 1))
    else
      cell=$(printf '%s\n' "$row" | awk -F'|' '{ c = $4; gsub(/^[ \t]+|[ \t]+$/, "", c); print c }')
      if [ "$cell" != "$want_cell" ]; then
        echo "STATUS-CHECK BAD: $id is $st in the record and its traceability cell reads '$cell', expected '$want_cell'"
        bad=$((bad + 1))
      fi
    fi
  done
  if [ "$bad" -ne 0 ]; then
    echo "STATUS-CHECK FAILED: $bad problem(s)"
    return 1
  fi
  echo "STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record"
}

# appendix_print E: for each path of the fixed list, a heading and a text block holding the
# file unchanged. Exit status 1, with nothing printed to stdout, when a file is missing or
# holds a line that would be misread inside the record.
appendix_print() {
  local e=${1:-deploy/test/out} p bad=0
  for p in $(native_paste_list); do
    if [ ! -f "$e/$p" ]; then
      say "native-amd64.sh: appendix: no scratch file $e/$p"
      bad=$((bad + 1))
    elif grep -Eq '^(```|#### |## )' "$e/$p"; then
      say "native-amd64.sh: appendix: $e/$p has a line that begins with three backticks, with '#### ' or with '## '; it cannot be pasted unchanged"
      bad=$((bad + 1))
    fi
  done
  if [ "$bad" -ne 0 ]; then
    return 1
  fi
  for p in $(native_paste_list); do
    printf '#### %s\n\n' "$p"
    printf '%s\n' '```text'
    awk 1 "$e/$p"
    printf '%s\n\n' '```'
  done
}

cmd_record_check() {
  local rc=0
  srv_record_check "${R:-deploy/NATIVE-AMD64-RUN.md}" "${E:-deploy/test/out}" || rc=$?
  return "$rc"
}
cmd_status_check() {
  local rc=0
  status_check "${R:-deploy/NATIVE-AMD64-RUN.md}" "${REQS:-.planning/REQUIREMENTS.md}" || rc=$?
  return "$rc"
}
cmd_appendix() {
  local rc=0
  appendix_print "${E:-deploy/test/out}" || rc=$?
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

# fx_bundle_tree BASE: a source tree of the shape `run` leaves behind, with files
# that must travel and files that must not. SYNTHETIC: R2's mdwe texts stand in for a run.
fx_bundle_tree() {
  local b=$1
  rm -rf "${b:?}"
  mkdir -p "$b/p" "$b/p-bak-src" "$b/p-bak-src-bulk" "$b/p-bak-tgt"
  fx_mdwe_r2 "$b/p-mdwe"
  printf 'UNAME: Linux\n' > "$b/p/runner.txt"
  printf '%s\n' '---- native amd64 summary ----' 'NATIVE-AMD64 DONE' > "$b/p/native-summary.txt"
  fx_chain "$b/p-bak-src/summary.txt"
  printf 'STEP 1 rc=0 true\n' > "$b/p-bak-src/steps.txt"
  printf 'BACKUP-BYTES=1\n' > "$b/p-bak-src-bulk/backup.txt"
  printf 'RESTORE-WALL-S=1\n' > "$b/p-bak-tgt/restore.txt"
  # never handed over
  printf 'step\n' > "$b/p-mdwe/step-01.txt"
  printf 'step\n' > "$b/p-bak-src/step-12.txt"
  printf -- '-----BEGIN OPENSSH PRIVATE KEY-----\n' > "$b/p-mdwe/seed.txt"
  printf 'journal\n' > "$b/p-mdwe/holzcloud.log"
  printf 'jar\n' > "$b/p-bak-src/cookies.txt"
  printf 'jar\n' > "$b/p-bak-src/AdminCookieJar"
  printf 'k\n' > "$b/p-bak-tgt/id_ed25519"
  printf 'k\n' > "$b/p-bak-tgt/server.pem"
  printf 'k\n' > "$b/p-bak-tgt/site.key"
  ln -s "$b/p/runner.txt" "$b/p-bak-tgt/link-to-host"
}

# exists_any PATH: a file, a directory or a link (even a dangling one).
exists_any() { [ -e "$1" ] || [ -L "$1" ]; }

# st_bundle BASE: bundle with BASE as BASE_OUT and BASE/ev as EVIDENCE_DIR; prints
# the exit status on the first line and the output after it.
st_bundle() {
  local rc=0 out
  out=$(
    (
      BASE_OUT=$1
      NATIVE_PREFIX=p
      EVIDENCE_DIR=${2-$1/ev}
      bundle
    ) 2>&1
  ) || rc=$?
  printf '%s\n%s\n' "$rc" "$out"
}

selftest_bundle() {
  local t=$ST_TMP rc out f b=$ST_TMP/bn

  # The happy path: exclusions, layout, count, no leftovers.
  fx_bundle_tree "$b"
  out=$(st_bundle "$b")
  st_eq "bundle: succeeds" 0 "$(printf '%s\n' "$out" | head -n 1)"
  st_has "bundle: prints the number of files kept" "BUNDLE: 11 files kept in $b/ev" "$out"
  st_eq "bundle: file count is what is on disk" 11 "$(find "$b/ev" -type f | wc -l | tr -d ' ')"
  for f in p-mdwe/step-01.txt p-mdwe/seed.txt p-mdwe/holzcloud.log p-bak-src/step-12.txt p-bak-src/cookies.txt \
    p-bak-src/AdminCookieJar p-bak-tgt/id_ed25519 p-bak-tgt/server.pem p-bak-tgt/site.key p-bak-tgt/link-to-host; do
    st_fail "bundle: $f is not kept" exists_any "$b/ev/$f"
  done
  for f in p/runner.txt p/native-summary.txt p-mdwe/verdict.txt p-mdwe/check.txt p-mdwe/versions.txt \
    p-mdwe/canary.verdict p-mdwe/mdwe-control.txt p-bak-src/summary.txt p-bak-src/steps.txt \
    p-bak-src-bulk/backup.txt p-bak-tgt/restore.txt; do
    st_ok "bundle: $f is kept, byte for byte" cmp -s "$b/$f" "$b/ev/$f"
  done
  st_eq "bundle: no temporary sibling is left" 0 "$(find "$b" -maxdepth 1 -name 'ev.tmp.*' | wc -l | tr -d ' ')"
  st_ok "bundle: the fixture's seed file does hold a key marker (the exclusion is what saves it)" \
    grep -q 'PRIVATE KEY' "$b/p-mdwe/seed.txt"

  # A stale bundle directory is replaced, not merged.
  printf 'stale\n' > "$b/ev/stale.txt"
  out=$(st_bundle "$b")
  st_eq "bundle: a second bundle succeeds" 0 "$(printf '%s\n' "$out" | head -n 1)"
  st_fail "bundle: a stale file of an earlier bundle is gone" test -e "$b/ev/stale.txt"

  # Refusals. Each leaves no bundle directory and no temporary sibling, and names the file.
  fx_bundle_tree "$b"
  mkdir -p "$b/ev"
  printf 'old bundle\n' > "$b/ev/old.txt"
  printf -- '-----BEGIN RSA PRIVATE KEY-----\nMIIB\n' > "$b/p-mdwe/extra.txt"
  out=$(st_bundle "$b")
  st_eq "bundle refuses a private key marker: exit 1" 1 "$(printf '%s\n' "$out" | head -n 1)"
  st_has "bundle refuses a private key marker: names the file" "p-mdwe/extra.txt" "$out"
  st_has "bundle refuses a private key marker: names the reason" "private key marker" "$out"
  st_fail "bundle refuses a private key marker: no bundle left" test -e "$b/ev"
  st_eq "bundle refuses a private key marker: no temporary sibling" 0 "$(find "$b" -maxdepth 1 -name 'ev.tmp.*' | wc -l | tr -d ' ')"

  fx_bundle_tree "$b"
  printf 'token hc_%s end\n' "ABCDEFGHIJKLMNOPQRSTUVWX" > "$b/p-bak-src/leak.txt"
  out=$(st_bundle "$b")
  st_eq "bundle refuses a key-shaped string: exit 1" 1 "$(printf '%s\n' "$out" | head -n 1)"
  st_has "bundle refuses a key-shaped string: names the file" "p-bak-src/leak.txt" "$out"
  st_has "bundle refuses a key-shaped string: names the reason" "AI key" "$out"
  st_fail "bundle refuses a key-shaped string: no bundle left" test -e "$b/ev"
  st_eq "bundle refuses a key-shaped string: no temporary sibling" 0 "$(find "$b" -maxdepth 1 -name 'ev.tmp.*' | wc -l | tr -d ' ')"

  # The boundary of the shape: 19 characters after hc_ is not a key, 20 is.
  fx_bundle_tree "$b"
  printf 'hc_%s\n' "ABCDEFGHIJKLMNOPQRS" > "$b/p-bak-src/short.txt"
  out=$(st_bundle "$b")
  st_eq "bundle: hc_ and 19 characters is not a key" 0 "$(printf '%s\n' "$out" | head -n 1)"
  fx_bundle_tree "$b"
  printf 'hc_%s\n' "ABCDEFGHIJKLMNOPQRST" > "$b/p-bak-src/edge.txt"
  out=$(st_bundle "$b")
  st_eq "bundle: hc_ and 20 characters is a key" 1 "$(printf '%s\n' "$out" | head -n 1)"
  st_fail "bundle: hc_ and 20 characters leaves no bundle" test -e "$b/ev"

  # A key-shaped string in a file that is excluded anyway does not refuse the bundle.
  fx_bundle_tree "$b"
  printf 'hc_%s\n' "ABCDEFGHIJKLMNOPQRSTUVWX" > "$b/p-mdwe/step-02.txt"
  out=$(st_bundle "$b")
  st_eq "bundle: a key in step-02.txt is removed before the scan" 0 "$(printf '%s\n' "$out" | head -n 1)"
  st_fail "bundle: step-02.txt is not in the bundle" test -e "$b/ev/p-mdwe/step-02.txt"

  # A tree with none of the directories: failure, nothing left.
  rm -rf "${b:?}"
  mkdir -p "$b/elsewhere"
  printf 'x\n' > "$b/elsewhere/a.txt"
  out=$(st_bundle "$b")
  st_eq "bundle on a tree without the directories: exit 1" 1 "$(printf '%s\n' "$out" | head -n 1)"
  st_has "bundle on a tree without the directories: says why" "an empty artifact is not evidence" "$out"
  st_fail "bundle on a tree without the directories: no bundle left" test -e "$b/ev"
  st_eq "bundle on a tree without the directories: no temporary sibling" 0 "$(find "$b" -maxdepth 1 -name 'ev.tmp.*' | wc -l | tr -d ' ')"

  # A tree with only a step-01.txt: everything is excluded, so nothing is evidence.
  rm -rf "${b:?}"
  mkdir -p "$b/p-mdwe"
  printf 'step\n' > "$b/p-mdwe/step-01.txt"
  out=$(st_bundle "$b")
  st_eq "bundle on a tree with only step-01.txt: exit 1" 1 "$(printf '%s\n' "$out" | head -n 1)"
  st_has "bundle on a tree with only step-01.txt: says why" "nothing is left after the exclusions" "$out"
  st_fail "bundle on a tree with only step-01.txt: no bundle left" test -e "$b/ev"

  # The bundle refuses to remove what is not its own.
  fx_bundle_tree "$b"
  for f in "" / . .. "$b" "$ROOT"; do
    out=$(st_bundle "$b" "$f")
    st_eq "bundle: EVIDENCE_DIR '$f' is refused" 1 "$(printf '%s\n' "$out" | head -n 1)"
  done
  st_ok "bundle: a refused EVIDENCE_DIR left the tree alone" test -f "$b/p-mdwe/verdict.txt"

  # The job summary page.
  rm -f "$t/page.md"
  rc=0
  (
    BASE_OUT=$b NATIVE_PREFIX=p GITHUB_STEP_SUMMARY=$t/page.md
    write_job_summary
  ) > /dev/null 2>&1 || rc=$?
  st_eq "write_job_summary: succeeds" 0 "$rc"
  st_eq "write_job_summary: opens the fence" '```text' "$(head -n 1 "$t/page.md")"
  st_eq "write_job_summary: closes the fence" '```' "$(tail -n 1 "$t/page.md")"
  st_eq "write_job_summary: carries the summary" 1 "$(grep -c '^NATIVE-AMD64 DONE$' "$t/page.md" || true)"
  (
    BASE_OUT=$b NATIVE_PREFIX=p GITHUB_STEP_SUMMARY=$t/page.md
    write_job_summary
  ) > /dev/null 2>&1
  st_eq "write_job_summary: appends, a second call makes two blocks" 2 "$(grep -c '^```text$' "$t/page.md" || true)"
  rc=0
  (
    unset GITHUB_STEP_SUMMARY
    BASE_OUT=$b NATIVE_PREFIX=p
    write_job_summary
  ) > /dev/null 2>&1 || rc=$?
  st_eq "write_job_summary: unset is a no-op, exit 0" 0 "$rc"
  rc=0
  out=$(
    (
      BASE_OUT=$b NATIVE_PREFIX=p GITHUB_STEP_SUMMARY=$t/no-such-dir/page.md
      write_job_summary
    ) 2>&1
  ) || rc=$?
  st_eq "write_job_summary: an unwritable page is a warning, exit 0" 0 "$rc"
  st_has "write_job_summary: an unwritable page is named" "could not be written" "$out"
  rc=0
  (
    BASE_OUT=$t/none NATIVE_PREFIX=p GITHUB_STEP_SUMMARY=$t/page3.md
    write_job_summary
  ) > /dev/null 2>&1 || rc=$?
  st_eq "write_job_summary: no summary file is a no-op, exit 0" 0 "$rc"
  st_fail "write_job_summary: no summary file writes no page" test -e "$t/page3.md"

  # run_publish: the page, then the bundle; a refused bundle is a failure.
  fx_bundle_tree "$b"
  rm -f "$t/page4.md"
  rc=0
  (
    BASE_OUT=$b NATIVE_PREFIX=p EVIDENCE_DIR=$b/ev GITHUB_STEP_SUMMARY=$t/page4.md
    run_publish
  ) > /dev/null 2>&1 || rc=$?
  st_eq "run_publish: ok" 0 "$rc"
  st_ok "run_publish: the page exists" test -s "$t/page4.md"
  st_ok "run_publish: the bundle exists" test -d "$b/ev"
  printf 'hc_%s\n' "ABCDEFGHIJKLMNOPQRSTUVWX" > "$b/p-bak-tgt/leak.txt"
  rc=0
  (
    BASE_OUT=$b NATIVE_PREFIX=p EVIDENCE_DIR=$b/ev
    run_publish
  ) > /dev/null 2>&1 || rc=$?
  st_eq "run_publish: a refused bundle fails" 1 "$rc"
  st_fail "run_publish: a refused bundle leaves nothing" test -e "$b/ev"

  # run refuses before it touches Docker or anything else.
  rc=0
  (NATIVE_STAGES=bogus cmd_run) > /dev/null 2>&1 || rc=$?
  st_eq "run: an unknown stage is a usage error" 2 "$rc"
  rc=0
  (NATIVE_STAGES="" cmd_run) > /dev/null 2>&1 || rc=$?
  st_eq "run: no stage is a usage error" 2 "$rc"
  rc=0
  (NATIVE_PREFIX=a/b cmd_run) > /dev/null 2>&1 || rc=$?
  st_eq "run: a prefix with a slash is refused" 1 "$rc"
  rc=0
  (NATIVE_PREFIX="" cmd_run) > /dev/null 2>&1 || rc=$?
  st_eq "run: an empty prefix is refused" 1 "$rc"
  rc=0
  /bin/bash "${BASH_SOURCE[0]}" bogus > /dev/null 2>&1 || rc=$?
  st_eq "an unknown subcommand is a usage error" 2 "$rc"
  rc=0
  /bin/bash "${BASH_SOURCE[0]}" > /dev/null 2>&1 || rc=$?
  st_eq "no subcommand is a usage error" 2 "$rc"
}

# ---------------------------------------------------------------------------
# selftest of the record (plan 26-02). The summaries are made by `summarize` over
# SYNTHETIC native sets (R2 turned into x86_64, the real chain summary); no native
# amd64 run exists. The records are minimal ones in the shape srv_record_check reads.
# ---------------------------------------------------------------------------

# fx_summary NAME KIND: $ST_TMP/sm-NAME.txt, a native summary of KIND:
# pass (synthetic native, both stages), notiming (timing not played), void, fail,
# notnative (the real R2 on arm64), notrun (mdwe played, timing not played is "notiming";
# this one plays neither). COMMIT is the HEAD of this repository, not a GITHUB_SHA.
fx_summary() {
  local b=$ST_TMP/fxs-$1 mst=0 tst=0
  rm -rf "${b:?}"
  mkdir -p "$b/p-bak-src"
  fx_chain "$b/p-bak-src/summary.txt"
  case "$2" in
    pass) fx_mdwe_synthetic "$b/p-mdwe" ;;
    notiming)
      fx_mdwe_synthetic "$b/p-mdwe"
      tst=notrun
      ;;
    void)
      fx_mdwe_synthetic "$b/p-mdwe"
      echo not-enforced > "$b/p-mdwe/canary.verdict"
      fx_verdict_r1 "$b/p-mdwe/verdict.txt"
      mst=3
      ;;
    fail)
      fx_mdwe_synthetic "$b/p-mdwe"
      fx_verdict_fails "$b/p-mdwe/verdict.txt"
      mst=1
      ;;
    notnative) fx_mdwe_r2 "$b/p-mdwe" ;;
    notrun)
      mst=notrun
      tst=notrun
      ;;
  esac
  (
    unset GITHUB_SHA GITHUB_RUN_ID
    st_summarize "$b" "$mst" "$tst" > /dev/null
  )
  cp "$b/p/native-summary.txt" "$ST_TMP/sm-$1.txt"
}

# fx_scratch E SUMMARY: a scratch root with every file of the fixed paste list; the
# native summary is SUMMARY, the others are one-line texts.
fx_scratch() {
  local e=$1 p
  rm -rf "${e:?}"
  for p in $(native_paste_list); do
    mkdir -p "$e/$(dirname "$p")"
    printf 'content of %s\n' "$p" > "$e/$p"
  done
  cp "$2" "$e/amd64-native/native-summary.txt"
}

fx_steps() {
  local k m
  for k in 1 2 3; do
    printf '**O-%s. Step %s.**\n\n' "$k" "$k"
    printf '%s\n' '```bash'
    for m in $(native_markers "SRV-0$k" | tr ' ' '~'); do
      printf 'echo "%s"\n' "$(printf '%s' "$m" | tr '~' ' ')"
    done
    printf '%s\n\n' '```'
    printf 'Expect text.\n\nA different output means a finding.\n\nPaste the result into the block.\n\n'
  done
  printf '**Recording a result.** Text.\n\n'
}

# fx_rec FILE EVID S1 S2 S3: a record; each S is pending or recorded. The blocks of the
# recorded requirements are $RB/SRV-0N, the appendix of a recorded SRV-01 is $RB/appendix.
fx_rec() {
  local f=$1 ev=$2 id s i
  {
    printf '# Record\n\n## Summary\n\n| Requirement | Result | Platform | Date, commit |\n|---|---|---|---|\n'
    i=0
    for s in "$3" "$4" "$5"; do
      i=$((i + 1))
      if [ "$s" = recorded ]; then
        printf '| SRV-0%s | Recorded from the result of the owner. | server | date |\n' "$i"
      else
        printf '| SRV-0%s | PENDING OWNER. The steps exist. | server | not run |\n' "$i"
      fi
    done
    printf '\nEvidence commit: %s\n\n## How this was prepared\n\nText.\n\n' "$ev"
    i=0
    for s in "$3" "$4" "$5"; do
      i=$((i + 1))
      id=SRV-0$i
      printf '## %s: title\n\nWhat it will show.\n\n' "$id"
      if [ "$s" = recorded ]; then
        printf 'Status: recorded\nResult (owner): recorded\n\n'
        printf '%s\n' '```text'
        cat "$RB/$id"
        printf '%s\n\n' '```'
      else
        printf 'Status: pending owner\nResult (owner): pending\n\n'
        printf '%s\n(nothing pasted yet)\n%s\n\n' '```text' '```'
      fi
    done
    printf '## Container vs. server\n\nText.\n\n## Not tested\n\nText.\n\n## Owner steps\n\n'
    fx_steps
    printf '## Hand-over to phase 28\n\nText.\n\n## Appendix: raw output\n\n'
    if [ "$3" = recorded ]; then cat "$RB/appendix"; fi
  } > "$f"
}

# fx_blocks: the paste blocks of SRV-02 and SRV-03, as the owner steps print them.
fx_blocks() {
  mkdir -p "$RB"
  printf '%s\n' '=== Backup complete: /opt/holzcloud/backups/20261026-100000 ===' '=== Restore complete ===' \
    'SRV-02 INSTANCE READY' 'SRV-02 TREES EQUAL' > "$RB/SRV-02"
  printf '%s\n' 'SRV-03 UNITS ENABLED' 'SRV-03 BOOT CHANGED' 'SRV-03 UNITS ACTIVE' 'SRV-03 SITE READY' \
    'SRV-03 TIMER ARMED' > "$RB/SRV-03"
}

# fx_rb1 SUMMARYFILE: $RB/SRV-01 and the scratch root $RB/E for a recorded SRV-01.
fx_rb1() {
  cp "$1" "$RB/SRV-01"
  fx_scratch "$RB/E" "$1"
  appendix_print "$RB/E" > "$RB/appendix"
}

# st_rc NAME WANT FILE E: srv_record_check fails, names WANT and ends RECORD-CHECK FAILED.
st_rc() {
  local out rc=0
  out=$(srv_record_check "$3" "$4" 2>&1) || rc=$?
  st_eq "record-check $1: exit" 1 "$rc"
  st_has "record-check $1: names the problem" "$2" "$out"
  st_has "record-check $1: ends FAILED" "RECORD-CHECK FAILED: " "$(printf '%s\n' "$out" | tail -n 1)"
}

# st_rok NAME WANT FILE E: srv_record_check ends with the line WANT and exit 0.
st_rok() {
  local out rc=0
  out=$(srv_record_check "$3" "$4" 2>&1) || rc=$?
  st_eq "record-check $1: exit" 0 "$rc"
  st_eq "record-check $1: last line" "$2" "$(printf '%s\n' "$out" | tail -n 1)"
  st_eq "record-check $1: no BAD line" 0 "$(printf '%s\n' "$out" | grep -c 'RECORD-CHECK BAD' || true)"
}

selftest_summary_check() {
  local t=$ST_TMP f out rc key

  fx_summary pass pass
  fx_summary notiming notiming
  fx_summary void void
  fx_summary fail fail
  fx_summary notnative notnative
  fx_summary notrun notrun

  rc=0
  out=$(summary_check "$t/sm-pass.txt" 2>&1) || rc=$?
  st_eq "summary_check: the synthetic native set" 0 "$rc"
  st_eq "summary_check: the synthetic native set, answer" "SUMMARY OK: SRV-01 PASS" "$out"
  rc=0
  out=$(summary_check "$t/sm-notiming.txt" 2>&1) || rc=$?
  st_eq "summary_check: timing stage not played" 0 "$rc"
  st_eq "summary_check: timing stage not played, answer" "SUMMARY OK: SRV-01 NOT-RUN" "$out"
  st_eq "summary_check: timing stage not played has no chain line" 0 "$(grep -c '^\(BAK-0\|POINT \)' "$t/sm-notiming.txt" || true)"
  st_eq "summary_check: timing stage not played says NOT-RUN" "NOT-RUN" "$(sum_word "$t/sm-notiming.txt" SRV-01-TIMING)"
  for key in void fail notnative notrun; do
    rc=0
    out=$(summary_check "$t/sm-$key.txt" 2>&1) || rc=$?
    st_eq "summary_check: a consistent $key summary is valid" 0 "$rc"
  done
  st_eq "summary_check: void" "SUMMARY OK: SRV-01 VOID" "$(summary_check "$t/sm-void.txt")"
  st_eq "summary_check: fail" "SUMMARY OK: SRV-01 FAIL" "$(summary_check "$t/sm-fail.txt")"
  st_eq "summary_check: not native" "SUMMARY OK: SRV-01 NOT-NATIVE" "$(summary_check "$t/sm-notnative.txt")"
  st_eq "summary_check: not run" "SUMMARY OK: SRV-01 NOT-RUN" "$(summary_check "$t/sm-notrun.txt")"

  st_sc() { # NAME WANT FILE: the summary is refused with a SUMMARY BAD line naming WANT
    local o r=0
    o=$(summary_check "$3" 2>&1) || r=$?
    st_eq "summary_check $1: exit" 1 "$r"
    st_has "summary_check $1: names the problem" "SUMMARY BAD: $2" "$o"
    st_has "summary_check $1: ends FAILED" "SUMMARY FAILED: " "$(printf '%s\n' "$o" | tail -n 1)"
  }
  f=$t/sm-bad.txt
  sed '$d' "$t/sm-pass.txt" > "$f"
  st_sc "no last line" "the last line is 'SRV-01 PASS'" "$f"
  { cat "$t/sm-pass.txt"; echo "SRV-01 PASS"; } > "$f"
  st_sc "last line is not DONE" "the last line is 'SRV-01 PASS', not 'NATIVE-AMD64 DONE'" "$f"
  sed '1s/.*/---- something else ----/' "$t/sm-pass.txt" > "$f"
  st_sc "first line" "the first line is '---- something else ----'" "$f"
  for key in COMMIT HOST-ARCH CONTAINER-ARCH MDWE-WORD CANARY MDWE-STATUS TIMING-STATUS; do
    grep -v "^$key: " "$t/sm-pass.txt" > "$f" || true
    st_sc "missing $key" "0 '$key: ' lines" "$f"
    awk -v k="$key: " '{print} index($0, k) == 1 {print}' "$t/sm-pass.txt" > "$f"
    st_sc "duplicated $key" "2 '$key: ' lines" "$f"
  done
  for key in SRV-01-MDWE SRV-01-TIMING SRV-01; do
    grep -v "^$key " "$t/sm-pass.txt" > "$f" || true
    st_sc "missing $key" "0 '$key ' verdict lines" "$f"
  done
  sed 's/^HOST-ARCH: .*/HOST-ARCH: aarch64/' "$t/sm-pass.txt" > "$f"
  st_sc "SRV-01 PASS claimed on an aarch64 host" "SRV-01-MDWE is claimed 'PASS', the lines of the summary give 'NOT-NATIVE'" "$f"
  st_has "summary_check: an aarch64 host forges the overall verdict too" "SRV-01 is claimed 'PASS', the lines of the summary give 'NOT-NATIVE'" "$(summary_check "$f" 2>&1 || true)"
  sed 's/^CONTAINER-ARCH: .*/CONTAINER-ARCH: aarch64/' "$t/sm-pass.txt" > "$f"
  st_sc "SRV-01 PASS claimed on an aarch64 container" "SRV-01-MDWE is claimed 'PASS', the lines of the summary give 'NOT-NATIVE'" "$f"
  grep -vx 'BAK-02 PASS' "$t/sm-pass.txt" > "$f" || true
  st_sc "SRV-01-TIMING PASS claimed without BAK-02 PASS" "SRV-01-TIMING is claimed 'PASS', the lines of the summary give 'FAIL'" "$f"
  sed 's/^LEAK-CHECK OK$/LEAK-CHECK BAD/' "$t/sm-pass.txt" > "$f"
  st_sc "SRV-01-TIMING PASS claimed without LEAK-CHECK OK" "SRV-01-TIMING is claimed 'PASS'" "$f"
  grep -v '^POINT bulk:' "$t/sm-pass.txt" > "$f" || true
  st_sc "SRV-01-TIMING PASS claimed without a bulk point" "SRV-01-TIMING is claimed 'PASS'" "$f"
  sed 's/^MDWE-WORD: .*/MDWE-WORD: VOID/' "$t/sm-pass.txt" > "$f"
  st_sc "SRV-01-MDWE PASS claimed over MDWE-WORD: VOID" "SRV-01-MDWE is claimed 'PASS', the lines of the summary give 'VOID'" "$f"
  sed 's/^MDWE-WORD: .*/MDWE-WORD: FAILS/' "$t/sm-pass.txt" > "$f"
  st_sc "SRV-01-MDWE PASS claimed over MDWE-WORD: FAILS" "SRV-01-MDWE is claimed 'PASS', the lines of the summary give 'FAIL'" "$f"
  sed 's/^CANARY: .*/CANARY: not-enforced/' "$t/sm-pass.txt" > "$f"
  st_sc "PASS over a canary that is not enforced" "SRV-01-MDWE is PASS but CANARY is 'not-enforced'" "$f"
  sed 's/^SRV-01 PASS$/SRV-01 FAIL/' "$t/sm-pass.txt" > "$f"
  st_sc "overall verdict understated" "SRV-01 is claimed 'FAIL', the lines of the summary give 'PASS'" "$f"
  sed 's/^SRV-01-TIMING PASS$/SRV-01-TIMING NOT-RUN/' "$t/sm-pass.txt" > "$f"
  st_sc "timing verdict does not follow TIMING-STATUS" "SRV-01-TIMING is claimed 'NOT-RUN', the lines of the summary give 'PASS'" "$f"
  sed 's/^TIMING-STATUS: 0$/TIMING-STATUS: notrun/' "$t/sm-pass.txt" > "$f"
  st_sc "timing claimed PASS but TIMING-STATUS says notrun" "SRV-01-TIMING is claimed 'PASS', the lines of the summary give 'NOT-RUN'" "$f"
  st_sc "no summary text" "there is no summary text" "$t/none.txt"
  : > "$f"
  st_sc "empty summary" "there is no summary text" "$f"
  st_eq "summary_check: the claimed word may carry a reason" "SUMMARY OK: SRV-01 FAIL" \
    "$(sed 's/^SRV-01-TIMING PASS$/SRV-01-TIMING FAIL (no chain summary)/; s/^SRV-01 PASS$/SRV-01 FAIL/; s/^BAK-01 PASS$/BAK-01 X/' "$t/sm-pass.txt" > "$f"; summary_check "$f")"
}

selftest_record() {
  local t=$ST_TMP f g e out rc head1 root1 id k

  head1=$(git rev-parse HEAD 2> /dev/null) || head1=""
  root1=$(git rev-list --max-parents=0 HEAD 2> /dev/null | tail -n 1) || root1=""
  RB=$t/rb
  rm -rf "${RB:?}"
  fx_blocks
  fx_summary pass pass
  fx_rb1 "$t/sm-pass.txt"
  e=$RB/E
  f=$t/rec-pending.md
  g=$t/rec-mut.md
  fx_rec "$f" "none (pending owner)" pending pending pending

  st_eq "native_paste_list: thirteen paths" 13 "$(native_paste_list | wc -l | tr -d ' ')"
  st_eq "native_paste_list: first" amd64-native/native-summary.txt "$(native_paste_list | head -n 1)"
  st_eq "native_paste_list: last" amd64-native-bak-tgt-bulk/restore.txt "$(native_paste_list | tail -n 1)"
  st_eq "native_paste_list: no duplicates" 0 "$(native_paste_list | sort | uniq -d | wc -l | tr -d ' ')"
  st_eq "native_markers SRV-01" 2 "$(native_markers SRV-01 | wc -l | tr -d ' ')"
  st_eq "native_markers SRV-02" 4 "$(native_markers SRV-02 | wc -l | tr -d ' ')"
  st_eq "native_markers SRV-03" 5 "$(native_markers SRV-03 | wc -l | tr -d ' ')"
  st_eq "native_markers SRV-02 keeps its trailing space" "=== Backup complete: " "$(native_markers SRV-02 | head -n 1)"
  st_eq "native_markers unknown" 0 "$(native_markers SRV-09 | wc -l | tr -d ' ')"

  # --- the pending form
  st_rok "pending form" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$f" "$e"
  st_rc "no record" "does not exist" "$t/none.md" "$e"

  # --- structure
  for k in "## Summary" "## How this was prepared" "## Container vs. server" "## Not tested" "## Owner steps" \
    "## Hand-over to phase 28" "## Appendix: raw output"; do
    grep -vxF -- "$k" "$f" > "$g" || true
    st_rc "missing heading '$k'" "the heading '$k' is missing" "$g" "$e"
  done
  for id in SRV-01 SRV-02 SRV-03; do
    sed "s/^## $id/## Other $id/" "$f" > "$g"
    st_rc "missing $id heading" "no heading beginning '## $id'" "$g" "$e"
  done

  # --- status and result
  sed '/^## SRV-02/,/^## SRV-03/{/^Status: /d;}' "$f" > "$g"
  st_rc "no Status line" "SRV-02 has 0 'Status: ' lines" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/{/^Status: /p;}' "$f" > "$g"
  st_rc "two Status lines" "SRV-02 has 2 'Status: ' lines" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^Status: pending owner/Status: done/' "$f" > "$g"
  st_rc "unknown Status" "SRV-02 has the unknown status 'done'" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^Status: pending owner/Status: pending/' "$f" > "$g"
  st_rc "Status pending (without owner)" "unknown status 'pending'" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/{/^Result (owner): /d;}' "$f" > "$g"
  st_rc "no Result line" "SRV-02 has 0 'Result (owner): ' lines" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/{/^Result (owner): /p;}' "$f" > "$g"
  st_rc "two Result lines" "SRV-02 has 2 'Result (owner): ' lines" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^Result (owner): pending/Result (owner): recorded/' "$f" > "$g"
  st_rc "Result recorded under Status pending owner" "'Result (owner): recorded' does not match 'Status: pending owner'" "$g" "$e"
  sed '/^## SRV-03/,/^## Container/s/^Status: pending owner/Status: recorded/' "$f" > "$g"
  st_rc "Status recorded over Result pending" "'Result (owner): pending' does not match 'Status: recorded'" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^Result (owner): pending/Result (owner): maybe/' "$f" > "$g"
  st_rc "unknown Result" "unknown 'Result (owner): maybe'" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^(nothing pasted yet)$/hello/' "$f" > "$g"
  st_rc "pending with a pasted line" "SRV-02 is pending owner but its paste block holds text" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^(nothing pasted yet)$/(nothing pasted yet)\
SRV-02 TREES EQUAL/' "$f" > "$g"
  st_rc "pending with a marker pasted" "SRV-02 is pending owner but its paste block holds text" "$g" "$e"
  sed '/^## SRV-02/,/^## SRV-03/s/^(nothing pasted yet)$//' "$f" > "$g"
  st_rok "pending with a blank paste block" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$g" "$e"
  awk '/^## SRV-02/{s=1} s && /^```/{n++; if (n==1) next; if (n==2) {s=0; next}} {print}' "$f" > "$g"
  st_rc "no paste block" "SRV-02 has no closed fenced paste block" "$g" "$e"

  # --- the Summary table
  grep -v '^| SRV-03 |' "$f" > "$g" || true
  st_rc "no Summary row" "0 rows beginning '| SRV-03 |'" "$g" "$e"
  sed 's/^| SRV-02 | PENDING OWNER\./| SRV-02 | Waiting./' "$f" > "$g"
  st_rc "pending row without PENDING OWNER" "the Summary row of SRV-02 is pending owner but does not say PENDING OWNER" "$g" "$e"
  sed 's/^| SRV-02 | PENDING OWNER\./| SRV-02 | PENDING OWNER. This will PASS./' "$f" > "$g"
  st_rc "pending row with PASS" "the Summary row of SRV-02 is pending owner and holds a pass word" "$g" "$e"
  sed 's/^| SRV-02 | PENDING OWNER\./| SRV-02 | PENDING OWNER. It passed./' "$f" > "$g"
  st_rc "pending row with passed" "holds a pass word" "$g" "$e"
  sed 's/^| SRV-02 | PENDING OWNER\./| SRV-02 | PENDING OWNER. A password is needed./' "$f" > "$g"
  st_rok "pending row with the word password is fine" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$g" "$e"

  # --- the evidence commit line
  grep -v '^Evidence commit: ' "$f" > "$g" || true
  st_rc "no Evidence commit line" "the record has 0 'Evidence commit: ' lines" "$g" "$e"
  awk '{print} /^## How this was prepared$/{print ""; print "Evidence commit: none (pending owner)"}' "$f" > "$g"
  st_rc "two Evidence commit lines" "the record has 2 'Evidence commit: ' lines" "$g" "$e"
  sed 's/^Evidence commit: .*/Evidence commit: xyz/' "$f" > "$g"
  st_rc "malformed Evidence commit" "the Evidence commit line is malformed: 'xyz'" "$g" "$e"
  sed 's/^Evidence commit: .*/Evidence commit: None (pending owner)/' "$f" > "$g"
  st_rc "Evidence commit with another capital" "malformed" "$g" "$e"
  if [ -n "$head1" ]; then
    sed "s/^Evidence commit: .*/Evidence commit: $head1/" "$f" > "$g"
    st_rc "a sha while SRV-01 is pending" "holds the sha $head1 while SRV-01 is pending owner" "$g" "$e"
  fi

  # --- the appendix while SRV-01 is pending
  { cat "$f"; printf '#### amd64-native/runner.txt\n\n```text\nx\n```\n'; } > "$g"
  st_rc "an appendix block while SRV-01 is pending" "the appendix has a '#### ' heading or a fenced block while SRV-01 is pending owner" "$g" "$e"
  { cat "$f"; printf '#### amd64-native/runner.txt\n'; } > "$g"
  st_rc "an appendix heading while SRV-01 is pending" "while SRV-01 is pending owner" "$g" "$e"
  { cat "$f"; printf '```text\nx\n```\n'; } > "$g"
  st_rc "an appendix fence while SRV-01 is pending" "while SRV-01 is pending owner" "$g" "$e"
  { cat "$f"; printf 'A paragraph of prose about the appendix.\n'; } > "$g"
  st_rok "appendix prose while SRV-01 is pending" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$g" "$e"

  # --- a pass word outside a fence
  for id in SRV-01 SRV-02 SRV-03; do
    awk -v l="$id PASS" '{print} /^## How this was prepared$/{print ""; print l}' "$f" > "$g"
    st_rc "$id PASS outside a fence while pending" "a line '$id PASS' stands outside a fenced block while $id is pending owner" "$g" "$e"
    awk -v l="$id PASS" '{print} /^## How this was prepared$/{print ""; print "```text"; print l; print "```"}' "$f" > "$g"
    st_rok "$id PASS inside a fence while pending" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$g" "$e"
    awk -v l="$id PASS" '{print} /^## How this was prepared$/{print ""; print "The line " l " is what a run prints."}' "$f" > "$g"
    st_rok "$id PASS inline in prose while pending" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$g" "$e"
  done
  awk '{print} /^## How this was prepared$/{print ""; print "- SRV-02 PASS"}' "$f" > "$g"
  st_rc "a bulleted pass line while pending" "a line 'SRV-02 PASS' stands outside a fenced block" "$g" "$e"
  awk '{print} /^## How this was prepared$/{print ""; print "SRV-01 PASS"}' "$f" > "$g"
  st_has "the pass line is refused for SRV-01 only, not for the others" "SRV-01 is pending owner" "$(srv_record_check "$g" "$e" 2>&1 || true)"

  # --- a private key
  awk '{print} /^## How this was prepared$/{print ""; print "```text"; print "-----BEGIN OPENSSH PRIVATE KEY-----"; print "```"}' "$f" > "$g"
  st_rc "a private key marker" "a private key marker occurs in the record" "$g" "$e"

  # --- the owner steps
  sed 's/^\*\*O-3\. /**Q-3. /' "$f" > "$g"
  st_rc "no O-3" "the Owner steps have no step beginning '**O-3. '" "$g" "$e"
  sed 's/^\*\*O-1\. /**Q-1. /' "$f" > "$g"
  st_rc "no O-1" "the Owner steps have no step beginning '**O-1. '" "$g" "$e"
  sed 's/^\*\*O-2\. /**Q-2. /' "$f" > "$g"
  st_rc "no O-2" "the Owner steps have no step beginning '**O-2. '" "$g" "$e"
  for k in 1 2 3; do
    case $k in
      1) rng='/^\*\*O-1\. /,/^\*\*O-2\. /' ;;
      2) rng='/^\*\*O-2\. /,/^\*\*O-3\. /' ;;
      3) rng='/^\*\*O-3\. /,/^\*\*Recording/' ;;
    esac
    sed "$rng"'{/^```/d;}' "$f" > "$g"
    st_rc "O-$k without a command block" "step O-$k has no fenced command block" "$g" "$e"
    sed "$rng"'s/Expect/Wait for/' "$f" > "$g"
    st_rc "O-$k without Expect" "step O-$k has no 'Expect' paragraph" "$g" "$e"
    sed "$rng"'s/different output/other thing/' "$f" > "$g"
    st_rc "O-$k without different output" "step O-$k does not say what a different output means" "$g" "$e"
    sed "$rng"'s/Paste/Send/' "$f" > "$g"
    st_rc "O-$k without Paste" "step O-$k does not say where the result is pasted" "$g" "$e"
  done
  sed '/^\*\*O-1\. /,/^\*\*O-2\. /s/NATIVE-AMD64 DONE/NATIVE DONE/' "$f" > "$g"
  st_rc "O-1 without its DONE marker" "step O-1 does not hold the marker 'NATIVE-AMD64 DONE'" "$g" "$e"
  sed '/^\*\*O-1\. /,/^\*\*O-2\. /s/SRV-01 PASS/SRV-01 OK/' "$f" > "$g"
  st_rc "O-1 without its PASS marker" "step O-1 does not hold the marker 'SRV-01 PASS'" "$g" "$e"
  for k in '=== Backup complete: ' '=== Restore complete ===' 'SRV-02 INSTANCE READY' 'SRV-02 TREES EQUAL'; do
    sed "/^\*\*O-2\. /,/^\*\*O-3\. /s/$k/XX/" "$f" > "$g"
    st_rc "O-2 without the marker '$k'" "step O-2 does not hold the marker '$k'" "$g" "$e"
  done
  for k in 'SRV-03 UNITS ENABLED' 'SRV-03 BOOT CHANGED' 'SRV-03 UNITS ACTIVE' 'SRV-03 SITE READY' 'SRV-03 TIMER ARMED'; do
    sed "/^\*\*O-3\. /,/^\*\*Recording/s/$k/XX/" "$f" > "$g"
    st_rc "O-3 without the marker '$k'" "step O-3 does not hold the marker '$k'" "$g" "$e"
  done
  # a marker that stands only in another step does not count for this one
  sed '/^\*\*O-3\. /,/^\*\*Recording/s/SRV-03 SITE READY/SRV-02 TREES EQUAL/' "$f" > "$g"
  st_rc "a marker of another step" "step O-3 does not hold the marker 'SRV-03 SITE READY'" "$g" "$e"
  # the Recording paragraph is not part of O-3
  sed '/^\*\*O-3\. /,/^\*\*Recording/s/SRV-03 TIMER ARMED/XX/' "$f" | awk '{print} /^\*\*Recording a result/{print "echo SRV-03 TIMER ARMED"}' > "$g"
  st_has "the fixture of the Recording paragraph test holds the marker after the Recording line" \
    "echo SRV-03 TIMER ARMED" "$(awk '/^\*\*Recording a result/{p=1} p' "$g")"
  st_rc "a marker in the Recording paragraph does not count for O-3" "step O-3 does not hold the marker 'SRV-03 TIMER ARMED'" "$g" "$e"

  # --- SRV-02 and SRV-03 recorded
  fx_rec "$g" "none (pending owner)" pending recorded pending
  st_rok "SRV-02 recorded" "RECORD-CHECK OK 0 blocks; recorded: SRV-02; pending owner: SRV-01 SRV-03" "$g" "$e"
  fx_rec "$g" "none (pending owner)" pending pending recorded
  st_rok "SRV-03 recorded" "RECORD-CHECK OK 0 blocks; recorded: SRV-03; pending owner: SRV-01 SRV-02" "$g" "$e"
  fx_rec "$g" "none (pending owner)" pending recorded recorded
  st_rok "SRV-02 and SRV-03 recorded" "RECORD-CHECK OK 0 blocks; recorded: SRV-02 SRV-03; pending owner: SRV-01" "$g" "$e"

  local gg=$t/rec-r2.md
  fx_rec "$gg" "none (pending owner)" pending recorded pending
  sed 's/^| SRV-02 | Recorded from the result of the owner\./| SRV-02 | PENDING OWNER./' "$gg" > "$g"
  st_rc "recorded row still says PENDING OWNER" "the Summary row of SRV-02 is recorded but still says PENDING OWNER" "$g" "$e"
  for k in '=== Backup complete: ' '=== Restore complete ===' 'SRV-02 INSTANCE READY' 'SRV-02 TREES EQUAL'; do
    grep -vF -- "$k" "$RB/SRV-02" > "$t/rb-block" || true
    cp "$RB/SRV-02" "$t/rb-keep"
    cp "$t/rb-block" "$RB/SRV-02"
    fx_rec "$g" "none (pending owner)" pending recorded pending
    cp "$t/rb-keep" "$RB/SRV-02"
    st_rc "SRV-02 recorded without '$k'" "SRV-02 recorded: the result block has no line that begins '$k'" "$g" "$e"
  done
  cp "$RB/SRV-02" "$t/rb-keep"
  printf '%s\n' '$ echo "=== Backup complete: "' '$ echo "=== Restore complete ==="' '$ echo "SRV-02 INSTANCE READY"' '$ echo "SRV-02 TREES EQUAL"' > "$RB/SRV-02"
  fx_rec "$g" "none (pending owner)" pending recorded pending
  cp "$t/rb-keep" "$RB/SRV-02"
  st_rc "SRV-02 recorded with the commands but not their output" "SRV-02 recorded: the result block has no line that begins '=== Backup complete: '" "$g" "$e"
  : > "$RB/SRV-02"
  fx_rec "$g" "none (pending owner)" pending recorded pending
  cp "$t/rb-keep" "$RB/SRV-02"
  st_rc "SRV-02 recorded with an empty block" "SRV-02 recorded: the result block has no line that begins" "$g" "$e"
  for k in 'SRV-03 UNITS ENABLED' 'SRV-03 BOOT CHANGED' 'SRV-03 UNITS ACTIVE' 'SRV-03 SITE READY' 'SRV-03 TIMER ARMED'; do
    cp "$RB/SRV-03" "$t/rb-keep"
    grep -vF -- "$k" "$t/rb-keep" > "$RB/SRV-03" || true
    fx_rec "$g" "none (pending owner)" pending pending recorded
    cp "$t/rb-keep" "$RB/SRV-03"
    st_rc "SRV-03 recorded without '$k'" "SRV-03 recorded: the result block has no line that begins '$k'" "$g" "$e"
  done

  # --- SRV-01 recorded
  if [ -n "$head1" ] && [ -n "$root1" ]; then
    fx_rec "$g" "$head1" recorded pending pending
    st_rok "SRV-01 recorded" "RECORD-CHECK OK 13 blocks; recorded: SRV-01; pending owner: SRV-02 SRV-03" "$g" "$e"
    fx_rec "$g" "$(printf '%s' "$head1" | cut -c1-12)" recorded pending pending
    st_rok "SRV-01 recorded with a 12 character sha" "RECORD-CHECK OK 13 blocks; recorded: SRV-01; pending owner: SRV-02 SRV-03" "$g" "$e"
    fx_rec "$g" "$(printf '%s' "$head1" | cut -c1-7)" recorded pending pending
    st_rok "SRV-01 recorded with a 7 character sha" "RECORD-CHECK OK 13 blocks; recorded: SRV-01; pending owner: SRV-02 SRV-03" "$g" "$e"
    fx_rec "$g" "$head1" recorded recorded recorded
    st_rok "all three recorded" "RECORD-CHECK OK 13 blocks; recorded: SRV-01 SRV-02 SRV-03; pending owner: none" "$g" "$e"

    fx_rec "$gg" "$head1" recorded pending pending
    sed 's/^| SRV-01 | Recorded from the result of the owner\./| SRV-01 | PENDING OWNER./' "$gg" > "$g"
    st_rc "SRV-01 recorded row still says PENDING OWNER" "the Summary row of SRV-01 is recorded but still says PENDING OWNER" "$g" "$e"
    sed 's/^Evidence commit: .*/Evidence commit: none (pending owner)/' "$gg" > "$g"
    st_rc "none (pending owner) while SRV-01 is recorded" "says 'none (pending owner)' while SRV-01 is recorded" "$g" "$e"
    sed 's/^Evidence commit: .*/Evidence commit: 0123456789abcdef/' "$gg" > "$g"
    st_rc "an unknown Evidence commit" "the Evidence commit 0123456789abcdef is unknown to this repository" "$g" "$e"
    sed "s/^Evidence commit: .*/Evidence commit: $root1/" "$gg" > "$g"
    st_rc "an Evidence commit that is not the COMMIT of the summary" "is not a prefix of the COMMIT of the pasted summary" "$g" "$e"
    sed 's/^Evidence commit: .*/Evidence commit: xyz/' "$gg" > "$g"
    st_rc "a malformed Evidence commit while recorded" "the Evidence commit line is malformed" "$g" "$e"

    # appendix rules
    sed 's/^#### amd64-native\/runner.txt$/#### amd64-native\/step-01.txt/' "$gg" > "$g"
    st_rc "an appendix path off the list" "the appendix block amd64-native/step-01.txt is not on the fixed paste list" "$g" "$e"
    awk '/^#### amd64-native\/native-summary.txt$/{skip=1; next} skip && /^#### /{skip=0} !skip{print}' "$gg" > "$g"
    st_rc "no native-summary block" "the appendix has no block for amd64-native/native-summary.txt" "$g" "$e"
    awk '/^## Appendix: raw output$/{print; a=1; next} !a{print}' "$gg" > "$g"
    st_rc "recorded SRV-01 with an empty appendix" "the appendix is empty" "$g" "$e"
    awk '{print} /^#### amd64-native\/runner.txt$/{r=1} r && /^```$/ && ++c==2 {print "#### amd64-native/runner.txt"; print ""; print "```text"; print "content of amd64-native/runner.txt"; print "```"; r=0}' "$gg" > "$g"
    st_rc "a block twice" "the appendix has two blocks for amd64-native/runner.txt" "$g" "$e"
    awk '/^#### amd64-native\/runner.txt$/{print; getline; print; getline; print; getline; print "changed"; next} {print}' "$gg" > "$g"
    st_rc "a block that differs from its file" "the block differs from the scratch file: amd64-native/runner.txt" "$g" "$e"
    mv "$e/amd64-native/runner.txt" "$e/amd64-native/runner.txt.gone"
    st_rc "a block without its scratch file" "no scratch file for the appendix block amd64-native/runner.txt" "$gg" "$e"
    mv "$e/amd64-native/runner.txt.gone" "$e/amd64-native/runner.txt"
    printf 'content of amd64-native/runner.txt' > "$e/amd64-native/runner.txt"
    st_rok "a scratch file without a final newline is normalised" "RECORD-CHECK OK 13 blocks; recorded: SRV-01; pending owner: SRV-02 SRV-03" "$gg" "$e"
    printf 'content of amd64-native/runner.txt\n' > "$e/amd64-native/runner.txt"
    awk '/^## Appendix: raw output$/{print; a=1; next} a && /^#### amd64-native-bak-tgt-bulk\/restore.txt$/{skip=1} !skip{print}' "$gg" > "$g"
    st_rok "the appendix may hold a subset of the list, as long as the native summary is in it" "RECORD-CHECK OK 12 blocks; recorded: SRV-01; pending owner: SRV-02 SRV-03" "$g" "$e"
    # the Result (owner) block must equal the native-summary block
    sed 's/^RUN: local$/RUN: locally/' "$RB/SRV-01" > "$t/rb-sum"
    cp "$RB/SRV-01" "$t/rb-keep"
    cp "$t/rb-sum" "$RB/SRV-01"
    fx_rec "$g" "$head1" recorded pending pending
    cp "$t/rb-keep" "$RB/SRV-01"
    st_rc "the Result block differs from the native-summary block" "the native-summary.txt block differs from the Result (owner) block of SRV-01" "$g" "$e"

    # summaries that are valid and not PASS can never be recorded
    for k in void fail notnative notrun notiming; do
      fx_summary "r-$k" "$k"
      RB1=$t/rb1-$k
      mkdir -p "$RB1"
      cp "$RB/SRV-02" "$RB1/SRV-02"
      cp "$RB/SRV-03" "$RB1/SRV-03"
      out=$(RB=$RB1 fx_rb1 "$t/sm-r-$k.txt"; RB=$RB1 fx_rec "$RB1/rec.md" "$head1" recorded pending pending; printf '%s\n' "$RB1/rec.md")
      st_rc "SRV-01 recorded over a $k summary" "SRV-01 recorded: the verdict of the summary is" "$out" "$RB1/E"
    done
    # a forged summary: PASS claimed over an aarch64 host
    sed 's/^HOST-ARCH: .*/HOST-ARCH: aarch64/' "$t/sm-pass.txt" > "$t/sm-forged.txt"
    RB1=$t/rb1-forged
    mkdir -p "$RB1"
    cp "$RB/SRV-02" "$RB1/SRV-02"
    cp "$RB/SRV-03" "$RB1/SRV-03"
    out=$(RB=$RB1 fx_rb1 "$t/sm-forged.txt"; RB=$RB1 fx_rec "$RB1/rec.md" "$head1" recorded pending pending; printf '%s\n' "$RB1/rec.md")
    st_rc "SRV-01 recorded over a forged summary" "SRV-01 recorded: SRV-01-MDWE is claimed 'PASS', the lines of the summary give 'NOT-NATIVE'" "$out" "$RB1/E"
    # the block is not a summary at all
    printf 'SRV-01 PASS\nNATIVE-AMD64 DONE\n' > "$t/sm-two.txt"
    RB1=$t/rb1-two
    mkdir -p "$RB1"
    cp "$RB/SRV-02" "$RB1/SRV-02"
    cp "$RB/SRV-03" "$RB1/SRV-03"
    out=$(RB=$RB1 fx_rb1 "$t/sm-two.txt"; RB=$RB1 fx_rec "$RB1/rec.md" "$head1" recorded pending pending; printf '%s\n' "$RB1/rec.md")
    st_rc "SRV-01 recorded over the two marker lines alone" "SRV-01 recorded: the first line is" "$out" "$RB1/E"
  fi

  # --- the command line
  rc=0
  out=$(R=$f E=$e /bin/bash "${BASH_SOURCE[0]}" record-check 2>&1) || rc=$?
  st_eq "record-check on the command line: exit" 0 "$rc"
  st_eq "record-check on the command line: answer" "RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03" "$(printf '%s\n' "$out" | tail -n 1)"
  sed 's/^Status: pending owner/Status: done/' "$f" > "$g"
  rc=0
  out=$(R=$g E=$e /bin/bash "${BASH_SOURCE[0]}" record-check 2>&1) || rc=$?
  st_eq "record-check on the command line: a bad record exits 1" 1 "$rc"
}

selftest_status_check() {
  local t=$ST_TMP f g q out rc

  RB=$t/rbs
  rm -rf "${RB:?}"
  fx_blocks
  q=$t/reqs.md
  g=$t/reqs-mut.md
  printf '%s\n' '# Requirements' '' \
    '- [ ] **SRV-01**: text.' '- [ ] **SRV-02**: text.' '- [ ] **SRV-03**: text.' '' \
    '| Requirement | Phase | Status |' '|-------------|-------|--------|' \
    '| SRV-01 | Phase 26 | Pending owner |' '| SRV-02 | Phase 26 | Pending owner |' '| SRV-03 | Phase 26 | Pending owner |' > "$q"
  f=$t/rec-s.md
  fx_rec "$f" "none (pending owner)" pending pending pending

  st_sk() { # NAME WANT RECORD REQS
    local o r=0
    o=$(status_check "$3" "$4" 2>&1) || r=$?
    st_eq "status-check $1: exit" 1 "$r"
    st_has "status-check $1: names the problem" "$2" "$o"
    st_has "status-check $1: ends FAILED" "STATUS-CHECK FAILED: " "$(printf '%s\n' "$o" | tail -n 1)"
  }

  out=$(status_check "$f" "$q" 2>&1) || true
  st_eq "status-check: all pending" "STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record" "$out"
  rc=0
  out=$(R=$f REQS=$q /bin/bash "${BASH_SOURCE[0]}" status-check 2>&1) || rc=$?
  st_eq "status-check on the command line: exit" 0 "$rc"
  st_eq "status-check on the command line: answer" "STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record" "$out"

  sed 's/^- \[ \] \*\*SRV-02\*\*/- [x] **SRV-02**/' "$q" > "$g"
  st_sk "pending record, box checked" "SRV-02 is pending owner in the record and the box in REQUIREMENTS is [x], expected [ ]" "$f" "$g"
  sed 's/^| SRV-03 | Phase 26 | Pending owner |/| SRV-03 | Phase 26 | Pending |/' "$q" > "$g"
  st_sk "pending record, cell Pending" "SRV-03 is pending owner in the record and its traceability cell reads 'Pending', expected 'Pending owner'" "$f" "$g"
  sed 's/^| SRV-01 | Phase 26 | Pending owner |/| SRV-01 | Phase 26 | Complete |/' "$q" > "$g"
  st_sk "pending record, cell Complete" "SRV-01 is pending owner in the record and its traceability cell reads 'Complete'" "$f" "$g"
  grep -v '^- \[ \] \*\*SRV-03\*\*' "$q" > "$g" || true
  st_sk "missing requirement line" "SRV-03 is pending owner in the record and REQUIREMENTS has no line '- [ ] **SRV-03**'" "$f" "$g"
  grep -v '^| SRV-02 |' "$q" > "$g" || true
  st_sk "missing traceability row" "SRV-02 is pending owner in the record and REQUIREMENTS has no traceability row '| SRV-02 |'" "$f" "$g"
  sed 's/^| SRV-02 | Phase 26 | Pending owner |/| SRV-02 | Phase 26 | pending owner |/' "$q" > "$g"
  st_sk "cell in another case" "its traceability cell reads 'pending owner', expected 'Pending owner'" "$f" "$g"
  st_sk "no record" "the record $t/none.md does not exist" "$t/none.md" "$q"
  st_sk "no requirements file" "the requirements file $t/none.md does not exist" "$f" "$t/none.md"
  sed 's/^Status: pending owner/Status: done/' "$f" > "$g.rec"
  st_sk "record without a readable status" "SRV-01 has no readable Status in the record" "$g.rec" "$q"

  fx_rec "$f" "none (pending owner)" pending recorded pending
  sed 's/^- \[ \] \*\*SRV-02\*\*/- [x] **SRV-02**/; s/^| SRV-02 | Phase 26 | Pending owner |/| SRV-02 | Phase 26 | Complete |/' "$q" > "$g"
  out=$(status_check "$f" "$g" 2>&1) || true
  st_eq "status-check: SRV-02 recorded, both files say so" "STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record" "$out"
  st_sk "recorded record, box open and cell Pending owner" "SRV-02 is recorded in the record and the box in REQUIREMENTS is [ ], expected [x]" "$f" "$q"
  st_has "recorded record, cell Pending owner" "SRV-02 is recorded in the record and its traceability cell reads 'Pending owner', expected 'Complete'" "$(status_check "$f" "$q" 2>&1 || true)"
  sed 's/^| SRV-02 | Phase 26 | Complete |/| SRV-02 | Phase 26 | Pending |/' "$g" > "$g.2"
  st_sk "recorded record, cell Pending" "its traceability cell reads 'Pending', expected 'Complete'" "$f" "$g.2"
  sed 's/^- \[x\] \*\*SRV-02\*\*/- [ ] **SRV-02**/' "$g" > "$g.3"
  st_sk "recorded record, box open and cell Complete" "the box in REQUIREMENTS is [ ], expected [x]" "$f" "$g.3"
}

selftest_appendix() {
  local t=$ST_TMP e out rc p n

  fx_summary pass pass
  e=$t/ap-e
  fx_scratch "$e" "$t/sm-pass.txt"
  rc=0
  out=$(appendix_print "$e" 2> /dev/null) || rc=$?
  st_eq "appendix: all files present" 0 "$rc"
  st_eq "appendix: thirteen headings" 13 "$(printf '%s\n' "$out" | grep -c '^#### ' || true)"
  st_eq "appendix: twenty-six fence lines" 26 "$(printf '%s\n' "$out" | grep -c '^```' || true)"
  st_eq "appendix: the headings are the list, in order" "$(native_paste_list)" "$(printf '%s\n' "$out" | sed -n 's/^#### //p')"
  st_eq "appendix: the first block is the native summary" "---- native amd64 summary ----" \
    "$(printf '%s\n' "$out" | sed -n '4p')"
  st_eq "appendix: a block opens as text" '```text' "$(printf '%s\n' "$out" | sed -n '3p')"
  rc=0
  out=$(R=/nonexistent E=$e /bin/bash "${BASH_SOURCE[0]}" appendix 2> /dev/null) || rc=$?
  st_eq "appendix on the command line: exit" 0 "$rc"
  st_eq "appendix on the command line: thirteen headings" 13 "$(printf '%s\n' "$out" | grep -c '^#### ' || true)"

  # a file without a final newline is printed with one and closed properly
  printf 'no newline at the end' > "$e/amd64-native/runner.txt"
  out=$(appendix_print "$e" 2> /dev/null)
  st_has "appendix: a file without a final newline" 'no newline at the end
```' "$out"
  printf 'content of amd64-native/runner.txt\n' > "$e/amd64-native/runner.txt"

  # a missing file: exit 1, nothing on stdout, the first missing file named
  n=0
  for p in $(native_paste_list); do
    n=$((n + 1))
    mv "$e/$p" "$e/$p.gone"
    rc=0
    out=$(appendix_print "$e" 2> "$t/ap-err") || rc=$?
    st_eq "appendix: missing $p exits 1" 1 "$rc"
    st_eq "appendix: missing $p prints nothing on stdout" "" "$out"
    st_has "appendix: missing $p is named" "no scratch file $e/$p" "$(cat "$t/ap-err")"
    mv "$e/$p.gone" "$e/$p"
  done
  mv "$e/amd64-native/native-summary.txt" "$e/amd64-native/native-summary.txt.gone"
  mv "$e/amd64-native/runner.txt" "$e/amd64-native/runner.txt.gone"
  appendix_print "$e" > /dev/null 2> "$t/ap-err" || true
  st_eq "appendix: two missing files are both named" 2 "$(grep -c 'no scratch file' "$t/ap-err" || true)"
  st_eq "appendix: the first missing file is named first" "native-amd64.sh: appendix: no scratch file $e/amd64-native/native-summary.txt" "$(head -n 1 "$t/ap-err")"
  mv "$e/amd64-native/native-summary.txt.gone" "$e/amd64-native/native-summary.txt"
  mv "$e/amd64-native/runner.txt.gone" "$e/amd64-native/runner.txt"

  # lines that cannot be pasted unchanged
  for p in '```text' '```' '#### heading' '## heading'; do
    { cat "$e/amd64-native/runner.txt"; printf '%s\n' "$p"; } > "$t/ap-bad"
    cp "$e/amd64-native/runner.txt" "$t/ap-keep"
    cp "$t/ap-bad" "$e/amd64-native/runner.txt"
    rc=0
    out=$(appendix_print "$e" 2> "$t/ap-err") || rc=$?
    st_eq "appendix: a line '$p' exits 1" 1 "$rc"
    st_eq "appendix: a line '$p' prints nothing on stdout" "" "$out"
    st_has "appendix: a line '$p' names the file" "$e/amd64-native/runner.txt has a line that begins" "$(cat "$t/ap-err")"
    cp "$t/ap-keep" "$e/amd64-native/runner.txt"
  done
  { cat "$e/amd64-native/runner.txt"; printf 'a ```inline``` fence is fine\ntext #### not at the start is fine\n'; } > "$t/ap-ok"
  cp "$e/amd64-native/runner.txt" "$t/ap-keep"
  cp "$t/ap-ok" "$e/amd64-native/runner.txt"
  rc=0
  appendix_print "$e" > /dev/null 2>&1 || rc=$?
  st_eq "appendix: backticks and hashes inside a line are fine" 0 "$rc"
  cp "$t/ap-keep" "$e/amd64-native/runner.txt"
  rc=0
  E=$t/none /bin/bash "${BASH_SOURCE[0]}" appendix > /dev/null 2>&1 || rc=$?
  st_eq "appendix on the command line without scratch files exits 1" 1 "$rc"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/native-amd64-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_pure
  selftest_summarize
  selftest_stages
  selftest_bundle
  selftest_summary_check
  selftest_record
  selftest_status_check
  selftest_appendix
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
    record-check) cmd_record_check ;;
    status-check) cmd_status_check ;;
    appendix) cmd_appendix ;;
    *) usage ;;
  esac
done
