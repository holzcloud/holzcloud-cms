#!/usr/bin/env bash
# Phase 24: the backup timer exactly as shipped.
#
# A sibling of deploy/test/run.sh. It leaves run.sh alone (docs/beta.md binds
# run.sh to its evidence commit) and reaches the harness only through run.sh's
# public subcommands. What it adds is the proof that the SHIPPED
# holzcloud-backup.timer and .service, with no drop-in on either of them, do
# what the documents promise:
#
#   nightly.sh selftest     the pure functions below, a table of cases, no Docker
#   nightly.sh source       run.sh build up install baseline update postinstall backup-fixture
#                           (the populated site whose postinstall enables the shipped timer)
#   nightly.sh target       run.sh up install for the second, fresh container
#   nightly.sh fire         BKP-01: the shipped 03:15 entry starts the hardened service by itself
#   nightly.sh delay        BKP-03: the shipped 300 second delay, sampled and judged against the real run
#   nightly.sh catchup      BKP-02: a window missed while the host was down runs after it is up again
#   nightly.sh prune        BKP-05: directories older than RETENTION_DAYS are deleted by one run of the service
#   nightly.sh offsite      BKP-04: with REMOTE_TARGET set the copy arrives on a second host (needs run.sh backup)
#   nightly.sh offsite-restore   BKP-04: the document's restore line, played on the second host from that copy
#   nightly.sh chain        all of the above from fresh containers, one command; ends in OUT_DIR/summary.txt
#   nightly.sh record-check the proof of deploy/BACKUP-TIMER-RUN.md against the scratch files (R and E)
#   nightly.sh down         remove the source and the second host
#
# Subcommands can be chained in one call and run in order. A failing one ends the
# call with its status.
#
# Exit codes (the meaning of run.sh): 0 ok, 1 a BAD line or a failed precondition,
# 2 usage, 3 the platform cannot give the evidence.
#
# No command in this file sets a clock. The container shares the kernel clock of
# the Docker VM, so a clock setter here would move every container on the daemon.
# The time ZONE is moved instead (a fixed-offset zone compiled with zic and
# installed as /etc/localtime), which changes what "03:15" means and nothing else.
#
# Every PASS and BAD line is computed by a pure function from recorded KEY lines;
# `selftest` holds a table of cases for those functions, the failing ones included.
#
# Environment (all optional):
#   PLATFORM       linux/amd64
#   TARGET         systemd-base (the only target `source` and `target` accept)
#   KEEP           0; 1 keeps containers (only `chain` and `down` ever remove one)
#   R, E           deploy/BACKUP-TIMER-RUN.md and deploy/test/out, the record and the scratch root of record-check
#   NAME, OUT_DIR  holzcloud-t24-src, deploy/test/out/amd64-t24-src (the source)
#   TGT_NAME, TGT_OUT_DIR   holzcloud-t24-host2, deploy/test/out/amd64-t24-host2
#   NEW_REF        HEAD, pinned to a sha by `source` and `target`
#   ZONE_LEAD      90 seconds before 03:15:00 local at which `fire` sets the zone (60 to 150)
#   FIRE_WAIT      540 seconds `fire` waits for the run
#   DELAY_SAMPLES  12 re-armings `delay` takes
#   CATCHUP_LEAD   150 seconds before 03:15:00 local at which `catchup` sets the zone
#   CATCHUP_WAIT   480 seconds `catchup` waits for the run after the start
#
# Written for bash 3.2 (macOS): no arrays, no mapfile, no case conversion.
# Anything that needs GNU tools (date -d, stat -c, sha256sum, zic) runs inside
# the container through docker exec.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

PLATFORM=${PLATFORM:-linux/amd64}
TARGET=${TARGET:-systemd-base}
KEEP=${KEEP:-0}
NAME=${NAME:-holzcloud-t24-src}
OUT_DIR=${OUT_DIR:-deploy/test/out/amd64-t24-src}
TGT_NAME=${TGT_NAME:-holzcloud-t24-host2}
TGT_OUT_DIR=${TGT_OUT_DIR:-deploy/test/out/amd64-t24-host2}
NEW_REF=${NEW_REF:-HEAD}
ZONE_LEAD=${ZONE_LEAD:-90}
FIRE_WAIT=${FIRE_WAIT:-540}
DELAY_SAMPLES=${DELAY_SAMPLES:-12}
CATCHUP_LEAD=${CATCHUP_LEAD:-150}
CATCHUP_WAIT=${CATCHUP_WAIT:-480}

# Allowances, named here and written next to the verdict as KEY lines. Both are
# for whole-second timestamps and exec latency; the second is larger because the
# backup service is ordered After=holzcloud.service and the site takes several
# seconds to start under emulation.
SLACK_FIRE=5
SLACK_CATCHUP=30

STAMP=/var/lib/systemd/timers/stamp-holzcloud-backup.timer
ZONE_ORIG_FILE=/root/.nightly-zone-orig

say() { printf '%s\n' "$*" >&2; }
die() { say "nightly.sh: $*"; exit 1; }
usage() {
  sed -n '2,25p' "${BASH_SOURCE[0]}" >&2
  exit 2
}
dx() { docker exec "$NAME" "$@"; }
dxi() { docker exec -i "$NAME" "$@"; }
# dxo: the output of a command in the container, without the carriage returns.
dxo() { docker exec "$NAME" "$@" | tr -d '\r'; }

# ---------------------------------------------------------------------------
# Pure functions. No Docker, no clock, no file: arguments in, a verdict out.
# They print a short reason and return 0 or 1, and `selftest` holds a table.
# ---------------------------------------------------------------------------

# zone_offset_seconds EPOCH LEAD: the signed offset in [-43200, 43200) that makes
# the local time of day of the UTC instant EPOCH equal 03:15:00 minus LEAD seconds.
zone_offset_seconds() {
  local epoch=$1 lead=$2 want tod off
  want=$((3 * 3600 + 15 * 60 - lead))
  tod=$((epoch % 86400))
  off=$((want - tod))
  while [ "$off" -ge 43200 ]; do off=$((off - 86400)); done
  while [ "$off" -lt -43200 ]; do off=$((off + 86400)); done
  printf '%s\n' "$off"
}

# zone_stdoff SECONDS: the offset in the form zic reads, [-]H:MM:SS.
zone_stdoff() {
  local s=$1 sign=""
  if [ "$s" -lt 0 ]; then
    sign=-
    s=$((0 - s))
  fi
  printf '%s%d:%02d:%02d\n' "$sign" $((s / 3600)) $((s % 3600 / 60)) $((s % 60))
}

# zone_day_shift OFFSET: the same time of day on another calendar day. A whole
# day is added to a negative offset and taken from a non-negative one, so the
# result stays inside (-86400, 86400), which zic accepts.
zone_day_shift() {
  if [ "$1" -ge 0 ]; then
    printf '%s\n' $(($1 - 86400))
  else
    printf '%s\n' $(($1 + 86400))
  fi
}

# hms_s HH:MM:SS: seconds since midnight.
hms_s() {
  local h m s
  IFS=: read -r h m s <<< "$1"
  printf '%s\n' $((10#$h * 3600 + 10#$m * 60 + 10#${s:-0}))
}

# tod_in_window T LO HI: the time of day T (HH:MM:SS) lies in the closed interval.
tod_in_window() {
  local t lo hi
  t=$(hms_s "$1")
  lo=$(hms_s "$2")
  hi=$(hms_s "$3")
  [ "$t" -ge "$lo" ] && [ "$t" -le "$hi" ]
}

# in_range V LO HI: whole numbers, closed interval.
in_range() { [ "$1" -ge "$2" ] && [ "$1" -le "$3" ]; }

# verdict_eq GOT WANT: equal strings.
verdict_eq() { [ "$1" = "$2" ]; }

# s_hms SECONDS: HH:MM:SS of a number of seconds since midnight.
s_hms() {
  printf '%02d:%02d:%02d\n' $(($1 / 3600)) $(($1 % 3600 / 60)) $(($1 % 60))
}

# fire_window_ok START ARMED BASE RANDOM ACCURACY [SLACK [DRAWN]]: whole epoch seconds.
# The timer drew a delay and so armed itself for ARMED, which must lie inside
# [BASE, BASE + RANDOM] (BASE is 03:15:00 local, computed independently of
# systemd), and the real run started at START, inside [ARMED, ARMED + ACCURACY +
# SLACK]. SLACK is the named allowance for whole-second timestamps and exec
# latency, 5 seconds by default. DRAWN is the second at which that delay was drawn
# (0 when unknown): a draw made after its own elapse time had passed makes the
# timer elapse at once, so the run may then start up to ACCURACY + SLACK after the
# draw instead. Prints the reason on failure, "ok" otherwise.
fire_window_ok() {
  local start=$1 armed=$2 base=$3 rnd=$4 acc=$5 slack=${6:-5} drawn=${7:-0} from
  if [ "$armed" -lt "$base" ]; then
    echo "the armed time $armed lies before the base $base (03:15:00 local)"
    return 1
  fi
  if [ "$armed" -gt $((base + rnd)) ]; then
    echo "the armed time $armed lies $((armed - base)) s after the base $base, more than the random delay of $rnd s"
    return 1
  fi
  if [ "$start" -lt "$armed" ]; then
    echo "the run started at $start, before the armed time $armed"
    return 1
  fi
  from=$armed
  [ "$drawn" -le "$from" ] || from=$drawn
  if [ "$start" -gt $((from + acc + slack)) ]; then
    echo "the run started at $start, $((start - from)) s after $from (the armed time $armed, drawn at $drawn); allowed: accuracy $acc s plus $slack s"
    return 1
  fi
  echo "ok"
}

# sample_stats_ok "D1 D2 ..." RANDOM: the delays of the samples, in seconds, and
# the random delay the timer reports. Prints "MIN MAX DISTINCT VERDICT", where
# the verdict is ok or BAD: and a reason. A delay of 0 in every sample means the
# measurement cannot see the delay, which is a failure to report and not a
# conclusion about the delay.
sample_stats_ok() {
  local list=$1 rnd=$2 sorted min max distinct n
  sorted=$(printf '%s\n' $list | sort -n)
  n=$(printf '%s\n' $sorted | grep -c . || true)
  if [ "$n" -eq 0 ]; then
    echo "0 0 0 BAD: no samples"
    return 1
  fi
  min=$(printf '%s\n' "$sorted" | head -n 1)
  max=$(printf '%s\n' "$sorted" | tail -n 1)
  distinct=$(printf '%s\n' "$sorted" | sort -n -u | grep -c . || true)
  if [ "$min" -lt 0 ]; then
    echo "$min $max $distinct BAD: a sample is negative ($min)"
    return 1
  fi
  if [ "$max" -gt "$rnd" ]; then
    echo "$min $max $distinct BAD: a sample ($max) exceeds the random delay of $rnd s"
    return 1
  fi
  if [ "$max" -eq 0 ]; then
    echo "$min $max $distinct BAD: every sample is 0, the delay looks like 0 or the measurement cannot see it"
    return 1
  fi
  if [ "$distinct" -lt 3 ]; then
    echo "$min $max $distinct BAD: fewer than three distinct values ($distinct)"
    return 1
  fi
  echo "$min $max $distinct ok"
}


# catchup_ok M D U A S R C L T, whole epoch seconds. M the missed instant, D when
# the container went down, U when it was up again, A when the shipped timer was
# activated after that boot, S when the run started, R the random delay, C the
# accuracy, L the named allowance, T the mtime of the stamp before the stop.
# The run must be a catch-up of a window that really was missed, and it must start
# no earlier than the activation and no later than max(M, A) + R + C + L.
catchup_ok() {
  local m=$1 d=$2 u=$3 a=$4 s=$5 r=$6 c=$7 l=$8 t=$9 hi
  if [ "$t" -ge "$m" ]; then
    echo "the stamp ($t) is not older than the missed instant ($m)"
    return 1
  fi
  if [ "$d" -ge "$m" ]; then
    echo "the container was still up at the missed instant (down at $d, missed $m)"
    return 1
  fi
  if [ "$m" -ge "$u" ]; then
    echo "nothing was missed: the container was up again at $u, the instant was $m"
    return 1
  fi
  if [ "$s" -lt "$a" ]; then
    echo "the run started at $s, before the timer's activation at $a"
    return 1
  fi
  if [ "$a" -gt "$m" ]; then hi=$a; else hi=$m; fi
  hi=$((hi + r + c + l))
  if [ "$s" -gt "$hi" ]; then
    echo "the run started at $s, $((s - hi)) s after the latest allowed $hi (max(missed, activation) + $r + $c + $l)"
    return 1
  fi
  echo "ok"
}

# prune_expect AGE_SECONDS RETENTION_DAYS: what backup.sh step 5 does to a
# directory of that age. `find -mtime +N` counts the age in whole 24 hour units,
# rounded down, and matches when that count is greater than N; so the directory is
# deleted once floor(age / 86400) > N, and a retention of 0 switches pruning off
# (the script skips the step). Prints DELETE or KEEP.
prune_expect() {
  local age=$1 ret=$2
  if [ "$ret" -le 0 ]; then
    echo KEEP
  elif [ $((age / 86400)) -gt "$ret" ]; then
    echo DELETE
  else
    echo KEEP
  fi
}

# list_equal LISTA LISTB: two sha256 listings (lines of "HASH  PATH") are equal,
# line for line and in the same order, and not empty. Two empty listings are
# equal and prove nothing, so they fail.
list_equal() {
  [ "$(printf '%s\n' "$1" | grep -c . || true)" -gt 0 ] || return 1
  [ "$1" = "$2" ]
}

# unit_diff_lines TEXTA TEXTB: the number of changed lines between two unit
# files, read from diff (normal format): the larger of the '<' and the '>' lines,
# so a replaced line counts once, an added or a removed line counts once.
unit_diff_lines() {
  local d lt gt
  d=$(diff <(printf '%s\n' "$1") <(printf '%s\n' "$2") || true)
  lt=$(printf '%s\n' "$d" | grep -c '^<' || true)
  gt=$(printf '%s\n' "$d" | grep -c '^>' || true)
  if [ "$lt" -ge "$gt" ]; then echo "$lt"; else echo "$gt"; fi
}

# restore_lines_ok TEXT: the text of a restore.txt written by run.sh restore
# describes a restore that worked: the old password logs in (303 to /admin/), the
# setup form was open before (200) and closed after (404), the media file is
# served (200 and a hash), the marker page and check 17 of 17 passed, and no line
# begins with BAD. The text is the only input.
restore_lines_ok() {
  grep -q '^LOGIN: HTTP 303 /admin/$' <<< "$1" || return 1
  grep -q '^SETUP-BEFORE: HTTP 200 -$' <<< "$1" || return 1
  grep -q '^SETUP-AFTER: HTTP 404 -$' <<< "$1" || return 1
  grep -q '^MEDIA-SERVED: HTTP 200 ' <<< "$1" || return 1
  grep -q '^PASS marker page' <<< "$1" || return 1
  grep -q '^PASS check: 17 of 17' <<< "$1" || return 1
  if grep -q '^BAD' <<< "$1"; then return 1; fi
  return 0
}

# file_clean FILE: the file exists, is not empty and holds no line beginning with
# BAD. A file that was never written (its stage died) proves nothing, so it fails.
file_clean() {
  [ -s "$1" ] || return 1
  if grep -q '^BAD' "$1"; then return 1; fi
  return 0
}

# verdict_of FILE OKLINE: prints PASS and returns 0 when the file exists, its last
# line equals OKLINE and no line begins with BAD; prints FAIL and returns 1 for a
# missing or empty file, a wrong last line, or any BAD line. The only input is the
# file: nothing the stage said about itself counts.
verdict_of() {
  local f=$1 ok=$2 last
  if [ ! -s "$f" ]; then
    echo FAIL
    return 1
  fi
  last=$(tail -n 1 "$f")
  if [ "$last" != "$ok" ]; then
    echo FAIL
    return 1
  fi
  if grep -q '^BAD' "$f"; then
    echo FAIL
    return 1
  fi
  echo PASS
}

# kvq FILE KEY: the value of the last "KEY: value" line, "?" when there is none.
kvq() {
  local v
  v=$(sed -n "s/^$2: //p" "$1" 2> /dev/null | tail -n 1)
  printf '%s\n' "${v:-?}"
}

# steps_of DIR: "played failed" from DIR/steps.txt, counted the way run.sh counts
# everywhere: a systemctl status line never counts as failed.
steps_of() {
  local f="$1/steps.txt" p fl
  if [ -f "$f" ]; then
    p=$(grep -c '^STEP ' "$f" || true)
    fl=$(awk '$1 == "STEP" && $3 != "rc=0" && index($0, "systemctl status") == 0' "$f" | grep -c . || true)
  fi
  printf '%s %s\n' "${p:-0}" "${fl:-0}"
}

# req_verdict STATUS FILE OKLINE [CLEANFILE]: PASS, or FAIL with the reason, for
# one stage. Status 3 is the platform not giving the evidence: that is never a
# PASS, and the reason says so.
req_verdict() {
  local sc=$1 f=$2 ok=$3 clean=${4:-}
  if [ "$sc" -eq 3 ]; then
    echo "FAIL (the platform could not give the evidence: the stage ended with status 3)"
    return 0
  fi
  if [ "$sc" -ne 0 ]; then
    echo "FAIL (the stage ended with status $sc)"
    return 0
  fi
  if [ "$(verdict_of "$f" "$ok" || true)" != PASS ]; then
    echo "FAIL ($f does not end with '$ok', is missing, or holds a BAD line)"
    return 0
  fi
  if [ -n "$clean" ] && ! file_clean "$clean"; then
    echo "FAIL ($clean is missing, empty or holds a BAD line)"
    return 0
  fi
  echo PASS
}

# RC_HEADINGS: the headings a record must carry as lines of their own. The five
# requirement sections are checked by prefix because their titles are free.
RC_HEADINGS='## Summary
## How this was run
## Deviations named
## Findings
## Not tested
## Owner steps
## Hand-over to phase 28
## Appendix: raw output'

# record_check R E LEDGER_DIRS: the proof of a record. It never relies on `set -e`
# (it is called in a context where that is off) and prints one line per problem,
# then RECORD-CHECK OK N blocks, or RECORD-CHECK FAILED and returns 1.
#   (a) every `#### DIRECTORY/FILE` block after `## Appendix: raw output` equals
#       $E/DIRECTORY/FILE byte for byte (a missing final newline is normalised);
#   (b) every token DIR/FILE of a ledger directory outside the appendix has a block;
#   (c) the required headings; (d) exactly one valid, known `Evidence commit: SHA`;
#   (e) no private key marker.
record_check() {
  local r=$1 e=$2 ledger=$3 t n name bad=0 h i sha ld tok nblocks nheads

  if [ ! -f "$r" ]; then
    echo "RECORD-CHECK BAD: the record $r does not exist"
    echo "RECORD-CHECK FAILED: 1 problem"
    return 1
  fi
  t=$(mktemp -d) || return 1
  : > "$t/index"
  : > "$t/heads"

  # (a) The appendix, extracted the way deploy/test/README.md describes it.
  awk -v d="$t" '/^## Appendix: raw output$/{app=1;next} !app{next} /^#### /{name=$2;want=1;next} want && /^```/{ if(!inb){inb=1;n++;f=d "/" n;print n " " name >> (d "/index");printf "" > f;next} else {inb=0;want=0;close(f);next} } inb{print >> f}' "$r"
  awk '/^## Appendix: raw output$/{a=1;next} a && /^#### /{print $2}' "$r" > "$t/heads"
  nheads=$(grep -c . "$t/heads" || true)
  nblocks=$(grep -c . "$t/index" || true)
  if [ "$nheads" -ne "$nblocks" ]; then
    echo "RECORD-CHECK BAD: the appendix has $nheads '#### ' headings and $nblocks fenced blocks: a heading has no block, or a block is not closed"
    bad=$((bad + 1))
  fi
  while read -r n name; do
    [ -n "$n" ] || continue
    if [ ! -f "$e/$name" ]; then
      echo "RECORD-CHECK BAD: no scratch file for the appendix block $name (looked for $e/$name)"
      bad=$((bad + 1))
      continue
    fi
    awk 1 "$t/$n" > "$t/$n.a"
    awk 1 "$e/$name" > "$t/$n.b"
    if ! cmp -s "$t/$n.a" "$t/$n.b"; then
      echo "RECORD-CHECK BAD: the block differs from the scratch file: $name"
      bad=$((bad + 1))
    fi
  done < "$t/index"

  # The body: everything before the appendix.
  awk '/^## Appendix: raw output$/{exit} {print}' "$r" > "$t/body"

  # (b) The ledger: no sentence cites output that is not pasted.
  for ld in $ledger; do
    for tok in $( (grep -oE "$ld/[A-Za-z0-9._-]+" "$t/body" || true) | sed 's/[.]*$//' | sort -u); do
      if ! grep -qxF "$tok" "$t/heads"; then
        echo "RECORD-CHECK BAD: the record cites $tok outside the appendix, and the appendix has no block for it"
        bad=$((bad + 1))
      fi
    done
  done

  # (c) The headings.
  while IFS= read -r h; do
    if ! grep -qxF -- "$h" "$r"; then
      echo "RECORD-CHECK BAD: the heading '$h' is missing"
      bad=$((bad + 1))
    fi
  done <<< "$RC_HEADINGS"
  for i in 1 2 3 4 5; do
    if ! grep -q "^## BKP-0$i" "$r"; then
      echo "RECORD-CHECK BAD: no heading beginning '## BKP-0$i'"
      bad=$((bad + 1))
    fi
  done

  # (d) The evidence commit.
  n=$(grep -c '^Evidence commit: ' "$t/body" || true)
  if [ "$n" -ne 1 ]; then
    echo "RECORD-CHECK BAD: the record has $n 'Evidence commit: ' lines outside the appendix, exactly one is required"
    bad=$((bad + 1))
  else
    sha=$(sed -n 's/^Evidence commit: //p' "$t/body")
    if ! printf '%s\n' "$sha" | grep -qE '^[0-9a-f]{7,40}$'; then
      echo "RECORD-CHECK BAD: the Evidence commit line is malformed: '$sha' is not 7 to 40 hex characters"
      bad=$((bad + 1))
    elif ! git cat-file -e "${sha}^{commit}" 2> /dev/null; then
      echo "RECORD-CHECK BAD: the Evidence commit $sha is unknown to this repository"
      bad=$((bad + 1))
    fi
  fi

  # (e) No private key.
  if grep -q 'PRIVATE KEY' "$r"; then
    echo "RECORD-CHECK BAD: a private key marker occurs in the record"
    bad=$((bad + 1))
  fi

  rm -rf "${t:?}"
  if [ "$bad" -ne 0 ]; then
    echo "RECORD-CHECK FAILED: $bad problem(s)"
    return 1
  fi
  echo "RECORD-CHECK OK $nblocks blocks"
}

# ---------------------------------------------------------------------------
# selftest
# ---------------------------------------------------------------------------

ST_N=0
ST_BAD=0
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

selftest_zone() {
  local mid=1728000000 e off
  # 1728000000 is a UTC midnight; 03:15:00 minus 90 s is 03:13:30, 11610 s.
  st_eq "midnight" 11610 "$(zone_offset_seconds "$mid" 90)"
  st_eq "03:14:59" -89 "$(zone_offset_seconds $((mid + 11699)) 90)"
  st_eq "03:15:00" -90 "$(zone_offset_seconds $((mid + 11700)) 90)"
  st_eq "23:59:59" 11611 "$(zone_offset_seconds $((mid + 86399)) 90)"
  # Raw offset exactly minus half a day stays; one second more wraps.
  st_eq "half a day" -43200 "$(zone_offset_seconds $((mid + 54810)) 90)"
  st_eq "half a day plus one second" 43199 "$(zone_offset_seconds $((mid + 54811)) 90)"
  st_eq "lead 150 at midnight" 11550 "$(zone_offset_seconds "$mid" 150)"
  # The defining property, for any epoch: the local time of day is 03:12:30.
  for e in 0 1 59 86399 86400 1728000001 1728043200 1793071153 1799999999; do
    off=$(zone_offset_seconds "$e" 150)
    st_eq "local time of day, lead 150, epoch $e" 11550 "$(((e + off) % 86400))"
    st_ok "offset range, epoch $e" in_range "$off" -43200 43199
  done
  # A day apart: the same time of day, another calendar day, inside what zic reads.
  st_eq "day shift of 11610" -74790 "$(zone_day_shift 11610)"
  st_eq "day shift of -19825" 66575 "$(zone_day_shift -19825)"
  st_eq "day shift of 0" -86400 "$(zone_day_shift 0)"
  st_eq "day shift of -1" 86399 "$(zone_day_shift -1)"
  for e in 1728000000 1728012345 1793071153; do
    for off in 11610 -19825 0 -43200 43199; do
      sh=$(zone_day_shift "$off")
      st_eq "same time of day after the day shift, epoch $e offset $off" "$(((e + off) % 86400))" "$(((e + sh) % 86400))"
      st_eq "another date after the day shift, epoch $e offset $off" 1 "$((((e + off) / 86400 - (e + sh) / 86400) * (off >= 0 ? 1 : -1)))"
    done
  done
  st_eq "stdoff of a shifted offset" "18:01:08" "$(zone_stdoff 64868)"
  st_eq "stdoff zero" "0:00:00" "$(zone_stdoff 0)"
  st_eq "stdoff 3661" "1:01:01" "$(zone_stdoff 3661)"
  st_eq "stdoff -19800" "-5:30:00" "$(zone_stdoff -19800)"
  st_eq "stdoff 11610" "3:13:30" "$(zone_stdoff 11610)"
  st_eq "stdoff -43200" "-12:00:00" "$(zone_stdoff -43200)"
}

selftest_tod() {
  st_ok "tod at the lower edge" tod_in_window 03:15:00 03:15:00 03:25:00
  st_ok "tod at the upper edge" tod_in_window 03:25:00 03:15:00 03:25:00
  st_fail "tod one second before" tod_in_window 03:14:59 03:15:00 03:25:00
  st_fail "tod one second after" tod_in_window 03:25:01 03:15:00 03:25:00
  st_eq "hms_s" 11700 "$(hms_s 03:15:00)"
  st_eq "hms_s 08:09:07" 29347 "$(hms_s 08:09:07)"
}

selftest_window() {
  # fire_window_ok START ARMED BASE RANDOM ACCURACY [SLACK 5]
  st_ok "start equal to armed" fire_window_ok 1000 1000 900 300 60
  st_ok "start at armed + accuracy + allowance" fire_window_ok 1065 1000 900 300 60
  st_fail "one second later" fire_window_ok 1066 1000 900 300 60
  st_fail "start before armed" fire_window_ok 999 1000 900 300 60
  st_fail "armed before the base" fire_window_ok 900 899 900 300 60
  st_ok "armed at the base" fire_window_ok 900 900 900 300 60
  st_ok "armed at base + delay" fire_window_ok 1200 1200 900 300 60
  st_fail "armed more than the delay after the base" fire_window_ok 1201 1201 900 300 60
  st_ok "a named allowance of 30" fire_window_ok 1090 1000 900 300 60 30
  st_fail "a named allowance of 30, one second later" fire_window_ok 1091 1000 900 300 60 30
  st_eq "ok is the whole answer" "ok" "$(fire_window_ok 1000 1000 900 300 60)"
  # A delay drawn after its own elapse time had passed: the timer elapses at once.
  st_ok "drawn late, start at the draw" fire_window_ok 1040 1000 900 300 60 5 1040
  st_ok "drawn late, start at the latest second" fire_window_ok 1105 1000 900 300 60 5 1040
  st_fail "drawn late, one second later" fire_window_ok 1106 1000 900 300 60 5 1040
  st_fail "drawn late, but start before the armed time" fire_window_ok 999 1000 900 300 60 5 1040
  st_ok "drawn before the armed time changes nothing" fire_window_ok 1065 1000 900 300 60 5 950
  st_fail "drawn before the armed time, one second later" fire_window_ok 1066 1000 900 300 60 5 950
}

selftest_samples() {
  st_ok "3 120 299" sample_stats_ok "3 120 299" 300
  st_eq "3 120 299 stats" "3 299 3 ok" "$(sample_stats_ok "3 120 299" 300)"
  st_fail "all zeros" sample_stats_ok "0 0 0 0 0 0 0 0 0 0 0 0" 300
  st_fail "a sample of 301" sample_stats_ok "3 120 301" 300
  st_ok "a sample of 300" sample_stats_ok "3 120 300" 300
  st_fail "a negative sample" sample_stats_ok "-1 120 299" 300
  st_fail "two distinct values" sample_stats_ok "5 5 5 7 7" 300
  st_fail "one value" sample_stats_ok "42 42 42" 300
  st_fail "no samples" sample_stats_ok "" 300
  st_ok "twelve samples" sample_stats_ok "17 203 88 4 299 150 61 77 12 254 130 99" 300
  st_eq "twelve distinct" "4 299 12 ok" "$(sample_stats_ok "17 203 88 4 299 150 61 77 12 254 130 99" 300)"
  # The window of the real run, recomputed from the recorded properties.
  st_eq "s_hms 03:15:00" "03:15:00" "$(s_hms 11700)"
  st_eq "s_hms 03:21:05" "03:21:05" "$(s_hms $((11700 + 300 + 60 + 5)))"
  st_ok "start inside the recomputed window" tod_in_window 03:19:48 03:15:00 "$(s_hms $((11700 + 300 + 60)))"
  st_ok "start at the recomputed upper edge" tod_in_window 03:21:00 03:15:00 "$(s_hms $((11700 + 300 + 60)))"
  st_fail "start one second past the recomputed upper edge" tod_in_window 03:21:01 03:15:00 "$(s_hms $((11700 + 300 + 60)))"
  st_fail "start before 03:15:00" tod_in_window 03:14:59 03:15:00 "$(s_hms $((11700 + 300 + 60)))"
}


selftest_catchup() {
  # catchup_ok M D U A S R C L T, base case: stamp 500, down 900, missed 1000,
  # up 1100, timer active 1105, run 1110, delay 300, accuracy 60, allowance 30.
  st_ok "a clean catch-up" catchup_ok 1000 900 1100 1105 1110 300 60 30 500
  st_ok "run at the activation" catchup_ok 1000 900 1100 1105 1105 300 60 30 500
  st_ok "run at the latest allowed second" catchup_ok 1000 900 1100 1105 1495 300 60 30 500
  st_fail "run one second past the allowance" catchup_ok 1000 900 1100 1105 1496 300 60 30 500
  st_fail "run before the activation" catchup_ok 1000 900 1100 1105 1104 300 60 30 500
  st_fail "stamp not older than the missed instant" catchup_ok 1000 900 1100 1105 1110 300 60 30 1000
  st_fail "stamp newer than the missed instant" catchup_ok 1000 900 1100 1105 1110 300 60 30 1001
  st_ok "stamp one second older" catchup_ok 1000 900 1100 1105 1110 300 60 30 999
  st_fail "container still up at the missed instant" catchup_ok 1000 1000 1100 1105 1110 300 60 30 500
  st_fail "container went down after the missed instant" catchup_ok 1000 1001 1100 1105 1110 300 60 30 500
  st_fail "nothing was missed" catchup_ok 1000 900 1000 1105 1110 300 60 30 500
  st_fail "up before the missed instant" catchup_ok 1000 900 999 1105 1110 300 60 30 500
  # max(missed, activation): with an activation before the missed instant the
  # bound counts from the missed instant.
  st_ok "activation before the missed instant, latest second" catchup_ok 1000 900 1100 950 1390 300 60 30 500
  st_fail "activation before the missed instant, one second later" catchup_ok 1000 900 1100 950 1391 300 60 30 500
  # The zone for the missed window: a lead of 150 puts the time of day at 03:12:30.
  st_eq "lead 150 time of day" "03:12:30" "$(s_hms "$(((1728012345 + $(zone_offset_seconds 1728012345 150)) % 86400))")"
}

selftest_prune() {
  local d=86400 h=3600
  # prune_expect AGE_SECONDS RETENTION_DAYS: find -mtime +N deletes when the age in
  # whole 24 hour units is greater than N.
  st_eq "45 days, retention 30" DELETE "$(prune_expect $((45 * d)) 30)"
  st_eq "31 days 1 hour, retention 30" DELETE "$(prune_expect $((31 * d + h)) 30)"
  st_eq "exactly 31 days, retention 30" DELETE "$(prune_expect $((31 * d)) 30)"
  st_eq "31 days less a second, retention 30" KEEP "$(prune_expect $((31 * d - 1)) 30)"
  st_eq "30 days 12 hours, retention 30" KEEP "$(prune_expect $((30 * d + 12 * h)) 30)"
  st_eq "exactly 30 days, retention 30" KEEP "$(prune_expect $((30 * d)) 30)"
  st_eq "29 days, retention 30" KEEP "$(prune_expect $((29 * d)) 30)"
  st_eq "1 day, retention 30" KEEP "$(prune_expect "$d" 30)"
  st_eq "no age, retention 30" KEEP "$(prune_expect 0 30)"
  st_eq "any age, retention 0" KEEP "$(prune_expect $((400 * d)) 0)"
  st_eq "2 days, retention 1" DELETE "$(prune_expect $((2 * d)) 1)"
  st_eq "1 day 1 hour, retention 1" KEEP "$(prune_expect $((d + h)) 1)"
  st_eq "1 day, retention 1" KEEP "$(prune_expect "$d" 1)"
}

selftest_offsite() {
  local a b unit_repo unit_one unit_two unit_add
  # list_equal LISTA LISTB: two sha256 listings, equal and not empty.
  a=$'aaaa  ./20261026-1/holzcloud.sqlite\nbbbb  ./20261026-1/MANIFEST\ncccc  ./20261026-2/csrf.key'
  st_ok "equal listings" list_equal "$a" "$a"
  b=$'aaaa  ./20261026-1/holzcloud.sqlite\nbbbX  ./20261026-1/MANIFEST\ncccc  ./20261026-2/csrf.key'
  st_fail "one altered hash" list_equal "$a" "$b"
  b=$'aaaa  ./20261026-1/holzcloud.sqlite\nbbbb  ./20261026-1/MANIFEST\ncccc  ./20261026-2/csrf.key\ndddd  ./20261026-2/MANIFEST'
  st_fail "one extra line" list_equal "$a" "$b"
  st_fail "one extra line, other side" list_equal "$b" "$a"
  b=$'aaaa  ./20261026-1/holzcloud.sqlite\ncccc  ./20261026-2/csrf.key'
  st_fail "one missing line" list_equal "$a" "$b"
  b=$'bbbb  ./20261026-1/MANIFEST\naaaa  ./20261026-1/holzcloud.sqlite\ncccc  ./20261026-2/csrf.key'
  st_fail "same lines, other order" list_equal "$a" "$b"
  st_fail "two empty listings" list_equal "" ""
  st_fail "two blank listings" list_equal $'\n\n' $'\n\n'
  st_fail "empty against full" list_equal "" "$a"
  st_fail "full against empty" list_equal "$a" ""
  # unit_diff_lines REPOTEXT INSTALLEDTEXT: the number of changed lines, read from diff.
  unit_repo=$'[Service]\nEnvironment=RETENTION_DAYS=30\n# Uncomment to copy off the box\n# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud\nNoNewPrivileges=true'
  st_eq "identical units" 0 "$(unit_diff_lines "$unit_repo" "$unit_repo")"
  unit_one=$'[Service]\nEnvironment=RETENTION_DAYS=30\n# Uncomment to copy off the box\nEnvironment=REMOTE_TARGET=offsite@nas:/backups/holzcloud\nNoNewPrivileges=true'
  st_eq "only the REMOTE_TARGET line uncommented" 1 "$(unit_diff_lines "$unit_repo" "$unit_one")"
  unit_two=$'[Service]\nEnvironment=RETENTION_DAYS=7\n# Uncomment to copy off the box\nEnvironment=REMOTE_TARGET=offsite@nas:/backups/holzcloud\nNoNewPrivileges=true'
  st_eq "two lines changed" 2 "$(unit_diff_lines "$unit_repo" "$unit_two")"
  unit_add=$'[Service]\nEnvironment=RETENTION_DAYS=30\n# Uncomment to copy off the box\n# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud\nNoNewPrivileges=true\nProtectHome=false'
  st_eq "one line added" 1 "$(unit_diff_lines "$unit_repo" "$unit_add")"
  st_eq "one line removed" 1 "$(unit_diff_lines "$unit_add" "$unit_repo")"
  st_eq "the uncommenting plus an added line" 2 "$(unit_diff_lines "$unit_repo" "$unit_one"$'\nProtectHome=false')"
}

selftest_restore() {
  local nl=$'\n' dir setb login seta media marker chk base
  dir='DIR: /backups/holzcloud/20261026-095025'
  setb='SETUP-BEFORE: HTTP 200 -'
  login='LOGIN: HTTP 303 /admin/'
  seta='SETUP-AFTER: HTTP 404 -'
  media='MEDIA-SERVED: HTTP 200 3f2a0c'
  marker='PASS marker page: GET /after-update answers 200 with its sentence'
  chk='PASS check: 17 of 17 assertions passed (check.txt)'
  base="$dir$nl$setb$nl$login$nl$seta$nl$media$nl$marker$nl$chk"
  st_ok "a restore that worked" restore_lines_ok "$base"
  st_ok "a restore that worked, with PASS lines around" restore_lines_ok "PASS setup form open before the restore$nl$base${nl}INFO nothing"
  st_fail "no login line" restore_lines_ok "$dir$nl$setb$nl$seta$nl$media$nl$marker$nl$chk"
  st_fail "a login line with another status" restore_lines_ok "$dir$nl$setb${nl}LOGIN: HTTP 200 -$nl$seta$nl$media$nl$marker$nl$chk"
  st_fail "a login that answers 303 elsewhere" restore_lines_ok "$dir$nl$setb${nl}LOGIN: HTTP 303 /admin/setup$nl$seta$nl$media$nl$marker$nl$chk"
  st_fail "a BAD line at the end" restore_lines_ok "$base${nl}BAD  tree: one file differs"
  st_fail "a BAD line in the middle" restore_lines_ok "$dir$nl$setb${nl}BAD  readyz: not ok$nl$login$nl$seta$nl$media$nl$marker$nl$chk"
  st_fail "a setup form that stayed open" restore_lines_ok "$dir$nl$setb$nl$login${nl}SETUP-AFTER: HTTP 200 -$nl$media$nl$marker$nl$chk"
  st_fail "a setup form that was not open before" restore_lines_ok "$dir${nl}SETUP-BEFORE: HTTP 404 -$nl$login$nl$seta$nl$media$nl$marker$nl$chk"
  st_fail "media not served" restore_lines_ok "$dir$nl$setb$nl$login$nl$seta${nl}MEDIA-SERVED: HTTP 404 -$nl$marker$nl$chk"
  st_fail "no marker page line" restore_lines_ok "$dir$nl$setb$nl$login$nl$seta$nl$media$nl$chk"
  st_fail "check 16 of 17" restore_lines_ok "$dir$nl$setb$nl$login$nl$seta$nl$media$nl$marker${nl}BAD  check: 16 of 17 assertions passed"
  st_fail "check line missing" restore_lines_ok "$dir$nl$setb$nl$login$nl$seta$nl$media$nl$marker"
  st_fail "empty text" restore_lines_ok ""
}

selftest_verdict() {
  local t f
  t=$(mktemp -d)
  f=$t/stage.txt
  printf 'KEY: 1\nPASS a\nFIRE OK\n' > "$f"
  st_eq "verdict ok" PASS "$(verdict_of "$f" "FIRE OK" || true)"
  st_ok "verdict ok returns 0" verdict_of "$f" "FIRE OK"
  st_eq "verdict wrong last line" FAIL "$(verdict_of "$f" "DELAY OK" || true)"
  st_fail "verdict wrong last line returns 1" verdict_of "$f" "DELAY OK"
  printf 'KEY: 1\nBAD  x\nFIRE OK\n' > "$f"
  st_eq "verdict BAD line before the ok line" FAIL "$(verdict_of "$f" "FIRE OK" || true)"
  printf 'BAD  x\nFIRE FAILED\n' > "$f"
  st_eq "verdict failed stage" FAIL "$(verdict_of "$f" "FIRE OK" || true)"
  printf 'FIRE OK\nINFO later\n' > "$f"
  st_eq "verdict ok line not last" FAIL "$(verdict_of "$f" "FIRE OK" || true)"
  printf ' BAD indented\nFIRE OK\n' > "$f"
  st_eq "verdict a BAD that does not begin the line" PASS "$(verdict_of "$f" "FIRE OK" || true)"
  : > "$f"
  st_eq "verdict empty file" FAIL "$(verdict_of "$f" "FIRE OK" || true)"
  st_eq "verdict missing file" FAIL "$(verdict_of "$t/none.txt" "FIRE OK" || true)"
  printf 'FIRE OK' > "$f"
  st_eq "verdict last line without newline" PASS "$(verdict_of "$f" "FIRE OK" || true)"

  printf 'PASS a\nINFO b\n' > "$f"
  st_ok "file_clean no BAD" file_clean "$f"
  printf 'PASS a\nBAD  b\n' > "$f"
  st_fail "file_clean BAD" file_clean "$f"
  : > "$f"
  st_fail "file_clean empty" file_clean "$f"
  st_fail "file_clean missing" file_clean "$t/none.txt"

  printf 'FIRE OK\n' > "$f"
  printf 'PASS a\n' > "$t/clean.txt"
  printf 'BAD  a\n' > "$t/dirty.txt"
  st_eq "req_verdict all green" PASS "$(req_verdict 0 "$f" "FIRE OK" "$t/clean.txt")"
  st_eq "req_verdict green without clean file" PASS "$(req_verdict 0 "$f" "FIRE OK")"
  st_eq "req_verdict status 3" "FAIL (the platform could not give the evidence: the stage ended with status 3)" "$(req_verdict 3 "$f" "FIRE OK" "$t/clean.txt")"
  st_eq "req_verdict status 1" "FAIL (the stage ended with status 1)" "$(req_verdict 1 "$f" "FIRE OK" "$t/clean.txt")"
  st_eq "req_verdict dirty verify file" "FAIL ($t/dirty.txt is missing, empty or holds a BAD line)" "$(req_verdict 0 "$f" "FIRE OK" "$t/dirty.txt")"
  st_eq "req_verdict missing verify file" "FAIL ($t/none.txt is missing, empty or holds a BAD line)" "$(req_verdict 0 "$f" "FIRE OK" "$t/none.txt")"
  st_eq "req_verdict wrong ok line" "FAIL ($f does not end with 'DELAY OK', is missing, or holds a BAD line)" "$(req_verdict 0 "$f" "DELAY OK")"

  printf 'A: 1\nB: two\nA: 3\n' > "$f"
  st_eq "kvq last value" 3 "$(kvq "$f" A)"
  st_eq "kvq other key" two "$(kvq "$f" B)"
  st_eq "kvq absent key" "?" "$(kvq "$f" C)"
  st_eq "kvq missing file" "?" "$(kvq "$t/none.txt" A)"

  mkdir -p "$t/d"
  st_eq "steps_of no file" "0 0" "$(steps_of "$t/d")"
  printf 'STEP 01 rc=0 a\nSTEP 02 rc=1 b\nSTEP 03 rc=1 systemctl status holzcloud\nSTEP 04 rc=0 c\n' > "$t/d/steps.txt"
  st_eq "steps_of counts" "4 1" "$(steps_of "$t/d")"
  rm -rf "${t:?}"
}

# st_record_make DIR SHA: DIR/rec.md is a record that record_check accepts, with
# one appendix block, and DIR/out/amd64-t24-src/a.txt is its scratch file.
st_record_make() {
  local d=$1 sha=$2 f='```'
  mkdir -p "$d/out/amd64-t24-src"
  printf 'line one\nline two\n' > "$d/out/amd64-t24-src/a.txt"
  {
    printf '# Record\n\n## Summary\n\nEvidence commit: %s\n\nThe figure is in `amd64-t24-src/a.txt`.\n\n' "$sha"
    printf '## How this was run\n\n## Deviations named\n\n## Findings\n\n'
    printf '## BKP-01: one\n\n## BKP-02: two\n\n## BKP-03: three\n\n## BKP-04: four\n\n## BKP-05: five\n\n'
    printf '## Not tested\n\n## Owner steps\n\n## Hand-over to phase 28\n\n## Appendix: raw output\n\n### R1 source\n\n'
    printf '#### amd64-t24-src/a.txt\n\n%s\nline one\nline two\n%s\n' "$f" "$f"
  } > "$d/rec.md"
}

# st_rc_case NAME WANT R E: record_check must fail and its output must contain WANT.
st_rc_case() {
  local out rc=0
  out=$(record_check "$3" "$4" "amd64-t24-src" 2>&1) || rc=$?
  st_eq "record-check $1 exit" 1 "$rc"
  if ! grep -qF -- "$2" <<< "$out"; then
    ST_N=$((ST_N + 1))
    ST_BAD=$((ST_BAD + 1))
    say "SELFTEST BAD record-check $1: the output does not name '$2': $out"
  else
    ST_N=$((ST_N + 1))
  fi
}

selftest_record() {
  local t sha out rc=0
  t=$(mktemp -d)
  sha=$(git rev-parse --short HEAD)
  st_record_make "$t" "$sha"
  out=$(record_check "$t/rec.md" "$t/out" "amd64-t24-src" 2>&1) || rc=$?
  st_eq "record-check good record exit" 0 "$rc"
  st_eq "record-check good record output" "RECORD-CHECK OK 1 blocks" "$out"

  # An altered scratch file.
  cp -R "$t/out" "$t/out-altered"
  echo "changed" >> "$t/out-altered/amd64-t24-src/a.txt"
  st_rc_case "altered scratch file" "the block differs from the scratch file: amd64-t24-src/a.txt" "$t/rec.md" "$t/out-altered"

  # A missing final newline in the scratch file is normalised, not a difference.
  cp -R "$t/out" "$t/out-nonl"
  printf 'line one\nline two' > "$t/out-nonl/amd64-t24-src/a.txt"
  out=$(record_check "$t/rec.md" "$t/out-nonl" "amd64-t24-src" 2>&1) || rc=$?
  st_eq "record-check missing final newline" "RECORD-CHECK OK 1 blocks" "$out"

  # A cited file without a block.
  awk '{print} /^## Summary$/{print "See amd64-t24-src/ghost.txt for it."}' "$t/rec.md" > "$t/cite.md"
  st_rc_case "cited file without a block" "cites amd64-t24-src/ghost.txt" "$t/cite.md" "$t/out"

  # A citation at the end of a sentence still counts.
  awk '{print} /^## Summary$/{print "It is in amd64-t24-src/ghost.txt."}' "$t/rec.md" > "$t/cite2.md"
  st_rc_case "cited file at a full stop" "cites amd64-t24-src/ghost.txt " "$t/cite2.md" "$t/out"

  # A required heading missing.
  grep -vx '## Not tested' "$t/rec.md" > "$t/nohead.md"
  st_rc_case "missing heading" "the heading '## Not tested' is missing" "$t/nohead.md" "$t/out"
  grep -v '^## BKP-03' "$t/rec.md" > "$t/nobkp.md"
  st_rc_case "missing requirement heading" "no heading beginning '## BKP-03'" "$t/nobkp.md" "$t/out"

  # The evidence commit: malformed, unknown, absent, doubled.
  sed 's/^Evidence commit: .*/Evidence commit: xyz/' "$t/rec.md" > "$t/malformed.md"
  st_rc_case "malformed Evidence commit" "the Evidence commit line is malformed" "$t/malformed.md" "$t/out"
  sed 's/^Evidence commit: .*/Evidence commit: 0123456789abcdef/' "$t/rec.md" > "$t/unknown.md"
  st_rc_case "unknown Evidence commit" "is unknown to this repository" "$t/unknown.md" "$t/out"
  grep -v '^Evidence commit: ' "$t/rec.md" > "$t/nocommit.md"
  st_rc_case "no Evidence commit" "has 0 'Evidence commit: ' lines" "$t/nocommit.md" "$t/out"
  awk '{print} /^## Summary$/{print "Evidence commit: '"$sha"'"}' "$t/rec.md" > "$t/twice.md"
  st_rc_case "two Evidence commit lines" "has 2 'Evidence commit: ' lines" "$t/twice.md" "$t/out"

  # A private key marker.
  awk '{print} /^## Summary$/{print "-----BEGIN OPENSSH PRIVATE KEY-----"}' "$t/rec.md" > "$t/key.md"
  st_rc_case "private key marker" "a private key marker occurs in the record" "$t/key.md" "$t/out"

  # A heading without a scratch file.
  sed 's|^#### amd64-t24-src/a.txt|#### amd64-t24-src/b.txt|' "$t/rec.md" > "$t/nofile.md"
  st_rc_case "heading without a scratch file" "no scratch file for the appendix block amd64-t24-src/b.txt" "$t/nofile.md" "$t/out"

  # A heading without a block.
  { cat "$t/rec.md"; printf '\n#### amd64-t24-src/a.txt\n'; } > "$t/noblock.md"
  st_rc_case "heading without a block" "has 2 '#### ' headings and 1 fenced blocks" "$t/noblock.md" "$t/out"

  # A record that does not exist.
  st_rc_case "missing record" "does not exist" "$t/none.md" "$t/out"
  rm -rf "${t:?}"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  selftest_zone
  selftest_tod
  selftest_window
  selftest_samples
  selftest_catchup
  selftest_prune
  selftest_offsite
  selftest_restore
  selftest_verdict
  selftest_record
  if [ "$ST_BAD" -ne 0 ]; then
    say "SELFTEST FAILED: $ST_BAD of $ST_N cases"
    exit 1
  fi
  say "SELFTEST OK ($ST_N cases)"
  echo "SELFTEST OK"
}

# ---------------------------------------------------------------------------
# The result vocabulary of run.sh: PASS, BAD, INFO and KEY lines, a BAD counter.
# ---------------------------------------------------------------------------

CHECK_OUT=/dev/null
BADS=0

res_begin() {
  CHECK_OUT=$1
  : > "$CHECK_OUT"
}
check_pass() { printf 'PASS %s\n' "$*" | tee -a "$CHECK_OUT"; }
check_bad() {
  printf 'BAD  %s\n' "$*" | tee -a "$CHECK_OUT"
  BADS=$((BADS + 1))
}
check_info() { printf 'INFO %s\n' "$*" | tee -a "$CHECK_OUT"; }
key_line() { printf '%s\n' "$*" | tee -a "$CHECK_OUT"; }

# kv FILE KEY: the value of the last "KEY: value" line of a result file.
kv() { sed -n "s/^$2: //p" "$1" 2> /dev/null | tail -n 1; }

deviation() {
  mkdir -p "$OUT_DIR"
  say "$*"
  printf '%s\n' "$*" >> "$OUT_DIR/deviations.txt"
}

# ---------------------------------------------------------------------------
# Container helpers
# ---------------------------------------------------------------------------

need_container() {
  dx true > /dev/null 2>&1 \
    || die "container $NAME does not answer; run 'bash deploy/test/nightly.sh source' first (NAME=$NAME)"
}

cont_epoch() { dxo date -u +%s; }

# unit_path UNIT: the D-Bus object path of a unit.
unit_path() {
  printf '/org/freedesktop/systemd1/unit/%s' "$(printf '%s' "$1" | sed -e 's/-/_2d/g' -e 's/\./_2e/g')"
}

# bus_us UNIT INTERFACE PROPERTY: a "t" property of the unit, microseconds, as
# systemd itself holds it. The numbers come from the bus, not from parsing the
# pretty-printed text of systemctl show.
bus_us() {
  local out
  out=$(dxo busctl get-property org.freedesktop.systemd1 "$(unit_path "$1")" "org.freedesktop.systemd1.$2" "$3") || return 1
  out=${out#t }
  case "$out" in "" | *[!0-9]*) return 1 ;; esac
  printf '%s\n' "$out"
}

# bus_s: the same in whole seconds (truncated, the way systemd prints them).
bus_s() {
  local v
  v=$(bus_us "$@") || return 1
  printf '%s\n' $((v / 1000000))
}

# show_v UNIT PROPERTY: systemctl show -p PROPERTY --value UNIT.
show_v() { dxo systemctl show "$1" -p "$2" --value; }

# base_epoch: the first 03:15:00 local instant after now, by date arithmetic in
# the container. The zone is fixed-offset, so adding 86400 is safe. Never taken
# from systemd.
base_epoch() {
  dxo bash -c 'now=$(date +%s); b=$(date -d "03:15:00" +%s); [ "$b" -gt "$now" ] || b=$((b + 86400)); echo "$b"'
}

# clock_skew: container epoch minus host epoch, in seconds.
clock_skew() {
  local c h
  h=$(date -u +%s)
  c=$(cont_epoch)
  printf '%s\n' $((c - h))
}

# ---------------------------------------------------------------------------
# leak_check: no secret of this container and nothing shaped like an AI key in
# any file under OUT_DIR or TGT_OUT_DIR. The driver writes no secret itself.
# ---------------------------------------------------------------------------

leak_check() {
  local f secret hit=0 d c
  for c in "$NAME" "$TGT_NAME"; do
    docker container inspect "$c" > /dev/null 2>&1 || continue
    for f in /root/.hc-admin-pass /root/.hc-login-pass /root/.hc-key; do
      secret=$(docker exec "$c" cat "$f" 2> /dev/null) || continue
      [ "${#secret}" -ge 8 ] || continue
      for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
        [ -d "$d" ] || continue
        if grep -rqF -- "$secret" "$d"; then
          say "LEAK: the contents of $f of $c occur in a file under $d"
          hit=1
        fi
      done
    done
  done
  for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
    [ -d "$d" ] || continue
    if grep -rEq 'hc_[A-Za-z0-9_-]{20,}' "$d"; then
      say "LEAK: a string shaped like an AI key occurs in a file under $d"
      hit=1
    fi
    if grep -rq 'PRIVATE KEY' "$d"; then
      say "LEAK: a PRIVATE KEY marker occurs in a file under $d"
      hit=1
    fi
  done
  [ "$hit" -eq 0 ] || exit 1
  say "LEAK-CHECK OK"
}

# ---------------------------------------------------------------------------
# source and target: the containers, played through run.sh's public subcommands
# ---------------------------------------------------------------------------

pin_ref() {
  git rev-parse --short "${NEW_REF}^{commit}" || die "NEW_REF '$NEW_REF' is not a commit of this repository"
}

# source: the populated, updated site with the shipped timer enabled by the
# document's own Backups block, plus the fixture directories.
cmd_source() {
  local ref
  [ "$TARGET" = systemd-base ] \
    || die "source needs TARGET=systemd-base (got '$TARGET'): the install is played out of the document into a bare systemd image"
  ref=$(pin_ref)
  say "source: NEW_REF pinned to $ref, container $NAME, evidence in $OUT_DIR"
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR INSTALL_FROM=old NEW_REF=$ref \
    bash deploy/test/run.sh build up install baseline update postinstall backup-fixture
}

# target: a second, fresh container (plan 02 restores onto it).
cmd_target() {
  local ref
  [ "$TARGET" = systemd-base ] \
    || die "target needs TARGET=systemd-base (got '$TARGET')"
  [ "$TGT_NAME" != "$NAME" ] || die "TGT_NAME equals NAME ($NAME)"
  ref=$(pin_ref)
  say "target: NEW_REF pinned to $ref, container $TGT_NAME, evidence in $TGT_OUT_DIR"
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR INSTALL_FROM=new NEW_REF=$ref \
    bash deploy/test/run.sh up install
}

# ---------------------------------------------------------------------------
# The zone. Moved by files, never by the clock.
# ---------------------------------------------------------------------------

# move_zone LEAD LABEL: compile a fixed-offset zone whose local time of day reads
# 03:15:00 minus LEAD now, and install it as /etc/localtime (a regular-file copy).
# The original link target is kept in ZONE_ORIG_FILE the first time only, so that
# a second move can still put the original back. Prints the readback time.
move_zone() {
  local lead=$1 label=$2 dayshift=${3:-0} now off std zname got want diff
  now=$(cont_epoch)
  off=$(zone_offset_seconds "$now" "$lead")
  [ "$dayshift" = 0 ] || off=$(zone_day_shift "$off")
  std=$(zone_stdoff "$off")
  zname="Nightly/$label"
  printf 'Zone %s %s - P24\n' "$zname" "$std" \
    | dxi bash -c 'cat > /root/nightly-zone.zic && zic -d /usr/share/zoneinfo /root/nightly-zone.zic' \
    || die "zic could not compile the zone $zname ($std)"
  dx bash -c 'f=$1
[ -e "$f" ] || { if [ -e /etc/localtime ]; then readlink -f /etc/localtime; else echo none; fi > "$f"; }' _ "$ZONE_ORIG_FILE" \
    || die "could not remember the original zone"
  dx bash -c 'cp "/usr/share/zoneinfo/$1" /etc/localtime.nightly && mv -f /etc/localtime.nightly /etc/localtime' _ "$zname" \
    || die "could not install the zone $zname as /etc/localtime"
  got=$(dxo date +%H:%M:%S)
  want=$((3 * 3600 + 15 * 60 - lead))
  diff=$(($(hms_s "$got") - want))
  [ "$diff" -ge 0 ] || diff=$((0 - diff))
  [ "$diff" -le 20 ] \
    || die "the zone $zname reads $got, not within 20 s of the wanted time of day (offset $off s)"
  ZONE_NAME=$zname
  ZONE_STD=$std
  ZONE_OFF=$off
  ZONE_READ=$got
}

# rearm_wait LEAD BEFORE_US: poll up to 30 s for NextElapseUSecRealtime of the
# shipped timer to lie in [now, now + lead + 300 + accuracy + 10] and to differ
# from what it was before the zone moved. Returns 1 when it does not follow.
rearm_wait() {
  local lead=$1 before=$2 acc end n now
  acc=$(bus_s holzcloud-backup.timer Timer AccuracyUSec) || return 1
  end=$((SECONDS + 30))
  while [ "$SECONDS" -lt "$end" ]; do
    n=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || n=0
    now=$(cont_epoch)
    if [ "$n" -ne "$before" ] && [ "$n" -ge "$now" ] && [ "$n" -le $((now + lead + 300 + acc + 10)) ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# follow_zone LEAD BEFORE: make sure systemd followed the zone change. If not,
# daemon-reload, then daemon-reexec (PID 1 calls tzset again), then a restart of
# the timer: each recorded as DEVIATION[zone-rearm]. A clock setter never is.
follow_zone() {
  local lead=$1 before=$2 step
  rearm_wait "$lead" "$before" && return 0
  for step in daemon-reload daemon-reexec "restart holzcloud-backup.timer"; do
    deviation "DEVIATION[zone-rearm]: systemd did not follow the zone change on its own within 30 s; 'systemctl $step' was run (never a clock setter)"
    dx systemctl $step || true
    rearm_wait "$lead" "$before" && return 0
  done
  return 1
}

# The draw that fired. The Docker VM steps the realtime clock every 20 to 30
# seconds. Every step makes systemd recalculate each calendar timer and draw a NEW
# random delay, and a draw that is already in the past makes the timer elapse at
# once. Reading NextElapseUSecRealtime once, or even five times a second, therefore
# misses the draw the timer actually fired on. PID 1 says what it drew ("Time has
# been changed", "Adding Ns random time", "Realtime timer elapses at ..."), but at
# debug level only, so the level is raised for the wait and put back afterwards.
# A log level is not a clock and not a unit.
armed_log_start() { dx systemd-analyze log-level debug || die "could not raise the log level of PID 1 in $NAME"; }
armed_log_stop() { dx systemd-analyze log-level info > /dev/null 2>&1 || true; }

# armed_log_collect START_US: writes OUT_DIR/fire-armed-log.txt, the raw lines of
# PID 1 about the shipped timer, and prints "ARMED DRAWN": the whole-second epoch
# the last logged draw elapses at, and the whole-second epoch at which it was
# drawn, among the draws logged at or before START_US (microseconds).
armed_log_collect() {
  dxo bash -c 'journalctl _PID=1 -o short-precise --no-pager 2>/dev/null | grep -E "holzcloud-backup.timer: (Time change|Adding|Realtime timer elapses)|Time has been changed"' > "$OUT_DIR/fire-armed-log.txt" || true
  dxi bash -s -- "$1" << 'INNER' | tr -d '\r'
start_us=$1
journalctl _PID=1 -o json --no-pager 2>/dev/null \
  | jq -r 'select((.MESSAGE | type) == "string") | select(.MESSAGE | test("^holzcloud-backup.timer: Realtime timer elapses at ")) | "\(.__REALTIME_TIMESTAMP) \(.MESSAGE)"' \
  | while read -r rt msg; do
      dt=$(printf '%s' "$msg" | sed -n 's/.* \([0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9:]*\) .*/\1/p')
      [ -n "$dt" ] || continue
      e=$(date -d "$dt" +%s) || continue
      echo "$rt $e"
    done \
  | awk -v s="$start_us" '$1 + 0 <= s + 0 { rt = $1; e = $2 } END { if (rt != "") printf "%d %d\n", e, rt / 1000000 }'
INNER
}

# ---------------------------------------------------------------------------
# verify_dir DIR LABEL: what is asserted about a backup directory. Written to
# OUT_DIR/verify-LABEL.txt; BAD lines count in BADS. The two integrity readers,
# the MANIFEST, owner and mode, csrf.key and the media count.
# ---------------------------------------------------------------------------

verify_dir() {
  local dir=$1 label=$2 keep_out=$CHECK_OUT out crc=0 got k v om mb ms manifest
  res_begin "$OUT_DIR/verify-$label.txt"
  if dx test -f "$dir/holzcloud.sqlite"; then
    check_pass "file $dir/holzcloud.sqlite exists"
  else
    check_bad "file $dir/holzcloud.sqlite is missing"
  fi

  # Reader one: the binary's own check, on a COPY in a directory of its own (it
  # opens the database read-write and would leave -wal and -shm files behind).
  out=$(dx bash -c 't=$(mktemp -d) || exit 97
cp "$1/holzcloud.sqlite" "$t/holzcloud.sqlite" || { rm -rf "$t"; exit 98; }
rc=0
HOLZCLOUD_DATA_DIR="$t" /opt/holzcloud/holzcloud check 2>&1 || rc=$?
rm -rf "$t"
exit "$rc"' _ "$dir" | tr -d '\r') && crc=0 || crc=$?
  if [ "$crc" -eq 0 ] && grep -q 'integrity: ok' <<< "$out"; then
    check_pass "integrity holzcloud-check: ok"
  else
    check_bad "integrity holzcloud-check: exit $crc, output '$(printf '%s' "$out" | tr '\n' ' ')'"
  fi

  # Reader two, independent of the binary: the C sqlite3 CLI, a stronger pragma,
  # and an immutable URI (the file is WAL-mode; nothing is created next to it).
  got=$(dxo sqlite3 "file:$dir/holzcloud.sqlite?immutable=1" "PRAGMA integrity_check;" 2>&1) || got="exit $?: $got"
  if [ "$got" = ok ]; then
    check_pass "integrity sqlite3 integrity_check: ok"
  else
    check_bad "integrity sqlite3 integrity_check: '$(printf '%s' "$got" | tr '\n' ' ')'"
  fi

  manifest=$(dxo cat "$dir/MANIFEST" 2> /dev/null) || manifest=""
  for k in timestamp version data_dir host; do
    v=$(printf '%s\n' "$manifest" | sed -n "s/^$k=//p" | head -n 1)
    if [ -n "$v" ]; then
      check_pass "MANIFEST $k=$v"
    else
      check_bad "MANIFEST has no $k= line"
    fi
  done

  om=$(dxo stat -c '%U %a' "$dir") || om="?"
  if [ "$om" = "holzcloud 700" ]; then
    check_pass "owner and mode of the backup directory: $om"
  else
    check_bad "owner and mode of the backup directory: '$om', expected 'holzcloud 700' (it holds password hashes and the CSRF key)"
  fi

  if dx test -f "$dir/csrf.key"; then
    check_pass "csrf.key is in the backup"
  else
    check_bad "csrf.key is missing from $dir"
  fi

  mb=$(dxo bash -c 'find "$1" -type f | wc -l' _ "$dir/media") || mb="?"
  ms=$(dxo bash -c 'find /opt/holzcloud/data/media -type f | wc -l') || ms="?"
  key_line "MEDIA-FILES-BACKUP: $mb"
  key_line "MEDIA-FILES-DATA: $ms"
  if [ "$mb" = "$ms" ] && [ "$ms" != "?" ] && [ "$ms" -gt 0 ]; then
    check_pass "the backup holds as many media files as the data directory ($mb)"
  else
    check_bad "media files: backup '$mb', data directory '$ms' (they must be equal, and above zero)"
  fi
  CHECK_OUT=$keep_out
}

# ---------------------------------------------------------------------------
# fire: BKP-01
# ---------------------------------------------------------------------------

cmd_fire() {
  local enabled active started f u hs cs dropins lsd cal pers
  local skew0 skew1 now off before_s acc rnd base armed i end act st
  local start start_us endx trig res es id dir om stamp0 stamp1 sl last_armed drawn armed_pair draws

  need_container
  mkdir -p "$OUT_DIR"
  enabled=$(dxo systemctl is-enabled holzcloud-backup.timer 2> /dev/null) || enabled=""
  active=$(dxo systemctl is-active holzcloud-backup.timer 2> /dev/null) || active=""
  [ "$enabled" = enabled ] \
    || die "holzcloud-backup.timer is '${enabled:-not installed}' in $NAME; run 'bash deploy/test/nightly.sh source' (the document's Backups block enables it)"
  [ "$active" = active ] \
    || die "holzcloud-backup.timer is '${active:-unknown}' in $NAME, not active; start it with: docker exec $NAME systemctl start holzcloud-backup.timer"
  started=$(show_v holzcloud-backup.service ExecMainStartTimestamp) || started="?"
  [ -z "$started" ] \
    || die "holzcloud-backup.service has run in $NAME already (ExecMainStartTimestamp '$started'): only the timer's own first run counts; use a fresh container: bash deploy/test/nightly.sh source"
  dx bash -c 'command -v zic > /dev/null' \
    || { say "nightly.sh: zic is not in $NAME: the zone cannot be built on this image"; exit 3; }

  BADS=0

  # Step 1: shipped-ness.
  res_begin "$OUT_DIR/units-shipped.txt"
  for u in timer service; do
    f=holzcloud-backup.$u
    hs=$(shasum -a 256 "deploy/$f" | cut -d' ' -f1)
    cs=$(dxo sha256sum "/etc/systemd/system/$f" | cut -d' ' -f1) || cs="?"
    key_line "SHA256-REPO-$u: $hs"
    key_line "SHA256-INSTALLED-$u: $cs"
    if verdict_eq "$cs" "$hs"; then
      check_pass "the installed $f is byte-equal to deploy/$f"
    else
      check_bad "the installed $f differs from deploy/$f"
    fi
    dropins=$(show_v "$f" DropInPaths) || dropins="?"
    key_line "DROPINPATHS-$u: ${dropins:-empty}"
    if [ -z "$dropins" ]; then
      check_pass "$f has no drop-in (DropInPaths is empty)"
    else
      check_bad "$f has drop-ins: $dropins"
    fi
  done
  lsd=$(dxo bash -c 'ls -d /etc/systemd/system/holzcloud-backup.*.d /usr/lib/systemd/system/holzcloud-backup.*.d /run/systemd/system/holzcloud-backup.*.d 2>/dev/null || true')
  key_line "DROPIN-DIRS: ${lsd:-none}"
  if [ -z "$lsd" ]; then
    check_pass "no holzcloud-backup.*.d directory exists"
  else
    check_bad "drop-in directories exist: $lsd"
  fi
  cal=$(show_v holzcloud-backup.timer TimersCalendar) || cal="?"
  key_line "TIMERS-CALENDAR: $cal"
  case "$cal" in
    *"OnCalendar=*-*-* 03:15:00 "*) check_pass "TimersCalendar reads OnCalendar=*-*-* 03:15:00" ;;
    *) check_bad "TimersCalendar is '$cal', expected OnCalendar=*-*-* 03:15:00" ;;
  esac
  pers=$(show_v holzcloud-backup.timer Persistent) || pers="?"
  key_line "PERSISTENT: $pers"
  {
    echo "---- systemctl show (timer) ----"
    dx systemctl show holzcloud-backup.timer -p TimersCalendar -p Persistent -p RandomizedDelayUSec -p AccuracyUSec -p FixedRandomDelay -p DropInPaths -p FragmentPath
    echo "---- systemctl show (service) ----"
    dx systemctl show holzcloud-backup.service -p Environment -p DropInPaths -p FragmentPath -p User -p ProtectSystem -p NoNewPrivileges
    echo "---- systemctl cat holzcloud-backup.timer ----"
    dx systemctl cat holzcloud-backup.timer
    echo "---- systemctl cat holzcloud-backup.service ----"
    dx systemctl cat holzcloud-backup.service
  } >> "$CHECK_OUT" 2>&1 || true

  # Step 2: the zone. DEVIATION[zone].
  res_begin "$OUT_DIR/fire.txt"
  key_line "EXEC-START-BEFORE: empty"
  skew0=$(clock_skew)
  key_line "CLOCK-SKEW-S-BEFORE: $skew0"
  before_s=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || die "could not read NextElapseUSecRealtime of the timer"
  move_zone "$ZONE_LEAD" Phase24
  deviation "DEVIATION[zone]: the container cannot wait for 03:15, so the time ZONE is moved and never the clock: the fixed-offset zone $ZONE_NAME (STDOFF $ZONE_STD, abbreviation P24, offset $ZONE_OFF s from UTC) was compiled with zic and installed as /etc/localtime, which made the local time read $ZONE_READ; the shipped timer re-armed in it and its own 03:15:00 arrived $ZONE_LEAD seconds later. No command in nightly.sh sets a clock. What it cannot show: a real day passing, daylight saving, the real host's zone, and a timer that has been armed for hours"
  key_line "ZONE: $ZONE_NAME STDOFF $ZONE_STD OFFSET-S $ZONE_OFF READ $ZONE_READ LEAD-S $ZONE_LEAD"
  follow_zone "$ZONE_LEAD" "$before_s" || {
    dx systemctl list-timers holzcloud-backup.timer --no-pager >&2 || true
    check_bad "systemd did not follow the zone change: NextElapseUSecRealtime of the shipped timer is not within the expected window"
    key_line "FIRE FAILED"
    exit 1
  }

  # Step 3: the armed state.
  dx systemctl list-timers holzcloud-backup.timer --no-pager > "$OUT_DIR/fire-before.txt" 2>&1 || true
  acc=$(bus_s holzcloud-backup.timer Timer AccuracyUSec) || acc=-1
  rnd=$(bus_s holzcloud-backup.timer Timer RandomizedDelayUSec) || rnd=-1
  base=$(base_epoch)
  armed=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || armed=0
  now=$(cont_epoch)
  stamp0=$(dxo stat -c '%Y %y' "$STAMP" 2> /dev/null) || stamp0="missing"
  key_line "BASE-EPOCH: $base"
  key_line "NEXT-ARMED-EPOCH: $armed"
  key_line "RANDOM-DELAY-S: $rnd"
  key_line "ACCURACY-S: $acc"
  key_line "SLACK-S: $SLACK_FIRE"
  key_line "STAMP-BEFORE: $stamp0"
  if in_range "$armed" $((now - 1)) $((now + 172800)); then
    check_pass "NextElapseUSecRealtime ($armed) is within two days of now ($now)"
  else
    check_bad "NextElapseUSecRealtime ($armed) is not within two days of now ($now)"
  fi

  armed_log_start

  # Step 4: wait for the run, a bounded poll once a second.
  i=0
  end=$((SECONDS + FIRE_WAIT))
  st=""
  while [ "$SECONDS" -lt "$end" ]; do
    st=$(bus_us holzcloud-backup.service Service ExecMainStartTimestamp) || st=0
    act=$(show_v holzcloud-backup.service ActiveState) || act=""
    if [ "${st:-0}" -gt 0 ]; then
      case "$act" in activating | active | reloading) ;; *) break ;; esac
    fi
    i=$((i + 1))
    [ $((i % 60)) -ne 0 ] || say "fire: waiting, $((end - SECONDS)) s left, the shipped timer was first armed for $armed (now $(cont_epoch))"
    sleep 1
    st=0
  done
  if [ "${st:-0}" -le 0 ]; then
    armed_log_stop
    say "nightly.sh: holzcloud-backup.service did not run within ${FIRE_WAIT}s; the timer and the unit journal:"
    dx systemctl list-timers holzcloud-backup.timer --no-pager >&2 || true
    dx journalctl -u holzcloud-backup.service --no-pager -n 40 >&2 || true
    check_bad "the shipped timer did not start the service within ${FIRE_WAIT} s (armed for $armed)"
    key_line "FIRE FAILED"
    exit 1
  fi

  # Step 5: collect.
  armed_log_stop
  dx systemctl list-timers holzcloud-backup.timer --no-pager > "$OUT_DIR/fire-after.txt" 2>&1 || true
  id=$(show_v holzcloud-backup.service InvocationID) || id=""
  dx journalctl -u holzcloud-backup.service -o short-precise --no-pager "_SYSTEMD_INVOCATION_ID=$id" > "$OUT_DIR/fire-journal.txt" 2>&1 || true
  # The directory is read from this invocation's journal and from nowhere else: a
  # wider journal would hand over the completion line of an earlier run.
  start_us=$(bus_us holzcloud-backup.service Service ExecMainStartTimestamp) || start_us=0
  start=$((start_us / 1000000))
  endx=$(bus_s holzcloud-backup.service Service ExecMainExitTimestamp) || endx=0
  sl=$(dxo date -d "@$start" +%H:%M:%S) || sl="?"
  armed_pair=$(armed_log_collect "$start_us") || armed_pair=""
  last_armed=${armed_pair%% *}
  drawn=${armed_pair#* }
  case "$last_armed" in "" | *[!0-9]*) last_armed=0 drawn=0 ;; esac
  trig=$(show_v holzcloud-backup.service TriggeredBy) || trig=""
  res=$(show_v holzcloud-backup.service Result) || res=""
  es=$(show_v holzcloud-backup.service ExecMainStatus) || es=""
  dir=$(sed -n 's/.*=== Backup complete: \(.*\) ===$/\1/p' "$OUT_DIR/fire-journal.txt" | tail -n 1 | tr -d '\r')
  om="?"
  if [ -n "$dir" ]; then om=$(dxo stat -c '%U %a' "$dir" 2> /dev/null) || om="?"; fi
  stamp1=$(dxo stat -c '%Y %y' "$STAMP" 2> /dev/null) || stamp1="missing"
  skew1=$(clock_skew)
  key_line "START-EPOCH: $start"
  key_line "START-LOCAL: $sl"
  key_line "LAST-ARMED-EPOCH: ${last_armed:-0}"
  key_line "LAST-DRAWN-EPOCH: ${drawn:-0}"
  draws=$(grep -c 'holzcloud-backup.timer: Adding' "$OUT_DIR/fire-armed-log.txt" 2> /dev/null || true)
  key_line "DRAWS-LOGGED: ${draws:-0}"
  key_line "TIME-CHANGED-LOGGED: $(grep -c 'Time has been changed' "$OUT_DIR/fire-armed-log.txt" 2> /dev/null || true)"
  key_line "END-EPOCH: $endx"
  key_line "TRIGGERED-BY: $trig"
  key_line "RESULT: $res"
  key_line "EXIT-STATUS: $es"
  key_line "INVOCATION-ID: $id"
  key_line "DIR: ${dir:-none}"
  key_line "DIR-OWNER-MODE: $om"
  key_line "STAMP-AFTER: $stamp1"
  key_line "CLOCK-SKEW-S-AFTER: $skew1"
  key_line "HOST-ARCH: $(docker info --format '{{.Architecture}}')"
  key_line "CONTAINER-ARCH: $(dxo uname -m)"

  # Step 6: judgement, from the KEY lines above.
  if verdict_eq "$trig" holzcloud-backup.timer; then
    check_pass "the service was started by holzcloud-backup.timer (TriggeredBy), not by hand"
  else
    check_bad "TriggeredBy is '$trig', expected holzcloud-backup.timer"
  fi
  if verdict_eq "$res $es" "success 0"; then
    check_pass "Result success and ExecMainStatus 0"
  else
    check_bad "Result '$res' and ExecMainStatus '$es', expected success and 0"
  fi
  if [ -n "$dir" ] && grep -q '=== Backup complete' "$OUT_DIR/fire-journal.txt"; then
    check_pass "the journal of the invocation holds the script's '=== Backup complete: $dir ===' line"
  else
    check_bad "the journal holds no '=== Backup complete: DIR ===' line (fire-journal.txt)"
  fi
  if [ -n "$dir" ] && dx test -d "$dir"; then
    check_pass "the directory the journal names exists: $dir"
  else
    check_bad "the directory the journal names ('${dir:-none}') does not exist"
  fi
  if verdict_eq "$om" "holzcloud 700"; then
    check_pass "owner and mode of the backup directory: $om (the hardened unit runs as the service user)"
  else
    check_bad "owner and mode of the backup directory: '$om', expected 'holzcloud 700'"
  fi
  if tod_in_window "$sl" 03:15:00 03:25:00; then
    check_pass "the run started at $sl local, at or after 03:15:00 and before 03:25:00"
  else
    check_bad "the run started at $sl local, outside [03:15:00, 03:25:00]"
  fi
  if in_range "$skew0" -120 120 && in_range "$skew1" -120 120; then
    check_pass "the container clock stayed within 120 s of the host's (before $skew0 s, after $skew1 s)"
  else
    check_bad "the container clock is off the host's by $skew0 s before and $skew1 s after (limit 120 s): the clock was moved"
  fi
  if [ "$last_armed" -gt 0 ]; then
    check_pass "PID 1 logged the draw the timer fired on: elapse $last_armed, drawn at $drawn (fire-armed-log.txt)"
  else
    check_bad "PID 1 logged no draw of the shipped timer before the start (fire-armed-log.txt): the armed time cannot be judged"
  fi
  if [ "$stamp1" != missing ]; then
    check_pass "the stamp $STAMP exists after the run ($stamp1)"
  else
    check_bad "the stamp $STAMP does not exist after the run"
  fi

  # Step 7: the backup.
  if [ -n "$dir" ] && dx test -f "$dir/holzcloud.sqlite"; then
    verify_dir "$dir" fire
  else
    res_begin "$OUT_DIR/verify-fire.txt"
    check_bad "the directory '${dir:-none}' holds no holzcloud.sqlite to verify"
  fi
  CHECK_OUT=$OUT_DIR/fire.txt
  if grep -q '^BAD' "$OUT_DIR/units-shipped.txt" "$OUT_DIR/verify-fire.txt"; then
    key_line "UNITS-AND-BACKUP: BAD lines in units-shipped.txt or verify-fire.txt"
  else
    key_line "UNITS-AND-BACKUP: no BAD line in units-shipped.txt and verify-fire.txt"
  fi
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "FIRE FAILED"
    say "nightly.sh: FIRE FAILED, $BADS BAD line(s); see $OUT_DIR/fire.txt, units-shipped.txt, verify-fire.txt"
    exit 1
  fi
  key_line "FIRE OK"
}

# ---------------------------------------------------------------------------
# delay: BKP-03. Needs the container of `fire`, after fire.
# The judgement is split from the capture on purpose: the window assertions read
# only the KEY lines of fire.txt, so they can be re-evaluated offline from the
# recorded file. fire.txt is read and never modified.
# ---------------------------------------------------------------------------

cmd_delay() {
  local f=$OUT_DIR/fire.txt d=$OUT_DIR/delay.txt
  local rnd acc fixed pers inst i base next dlt list stats rc smin smax sdist
  local s0 s1 start armed drawn fbase facc frnd hi_s sl prev rerolls end n logged

  need_container
  [ "$(tail -n 1 "$f" 2> /dev/null)" = "FIRE OK" ] \
    || die "$f does not end FIRE OK; run 'bash deploy/test/nightly.sh fire' first"
  BADS=0
  res_begin "$d"

  # The properties, as systemd reports them. AccuracyUSec is read, never typed.
  rnd=$(bus_s holzcloud-backup.timer Timer RandomizedDelayUSec) || rnd=-1
  acc=$(bus_s holzcloud-backup.timer Timer AccuracyUSec) || acc=-1
  fixed=$(show_v holzcloud-backup.timer FixedRandomDelay) || fixed="?"
  pers=$(show_v holzcloud-backup.timer Persistent) || pers="?"
  key_line "RANDOM-DELAY-S: $rnd"
  key_line "ACCURACY-S: $acc"
  key_line "FIXED-RANDOM-DELAY: $fixed"
  key_line "PERSISTENT: $pers"
  if dx grep -qx 'RandomizedDelaySec=300' /etc/systemd/system/holzcloud-backup.timer; then
    inst="RandomizedDelaySec=300"
    check_pass "the installed holzcloud-backup.timer holds the line RandomizedDelaySec=300"
  else
    inst="absent"
    check_bad "the installed holzcloud-backup.timer holds no RandomizedDelaySec=300 line"
  fi
  key_line "INSTALLED-LINE: $inst"
  if verdict_eq "$rnd" 300; then
    check_pass "the running timer reports RandomizedDelayUSec of 300 s"
  else
    check_bad "RandomizedDelayUSec is $rnd s, expected 300 s"
  fi
  if verdict_eq "$pers" yes; then
    check_pass "Persistent is yes"
  else
    check_bad "Persistent is '$pers', expected yes"
  fi

  # Sampling: restart the shipped timer, read when systemd armed it, subtract the
  # independently computed 03:15:00. A restart must not look like a run: the
  # stamp must not move.
  s0=$(dxo stat -c '%Y' "$STAMP" 2> /dev/null) || s0="missing"
  list=""
  i=1
  while [ "$i" -le "$DELAY_SAMPLES" ]; do
    # The start rate limit (five starts in ten seconds) refuses a sixth restart: pace
    # the samples, and clear the counter once if it was reached all the same.
    if ! dx systemctl restart holzcloud-backup.timer; then
      dx systemctl reset-failed holzcloud-backup.timer || true
      sleep 11
      dx systemctl restart holzcloud-backup.timer || die "could not restart holzcloud-backup.timer for sample $i"
    fi
    base=$(base_epoch)
    next=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || next=0
    dlt=$((next - base))
    key_line "SAMPLE $i $base $next $dlt"
    list="$list $dlt"
    i=$((i + 1))
    sleep 3
  done
  s1=$(dxo stat -c '%Y' "$STAMP" 2> /dev/null) || s1="missing"
  list=${list# }
  stats=$(sample_stats_ok "$list" "$rnd") && rc=0 || rc=$?
  smin=$(printf '%s' "$stats" | cut -d' ' -f1)
  smax=$(printf '%s' "$stats" | cut -d' ' -f2)
  sdist=$(printf '%s' "$stats" | cut -d' ' -f3)
  key_line "D-MIN: $smin"
  key_line "D-MAX: $smax"
  key_line "D-DISTINCT: $sdist"
  key_line "DELAY-SAMPLES: $DELAY_SAMPLES"
  if [ "$rc" -eq 0 ]; then
    check_pass "$DELAY_SAMPLES re-armings gave delays between $smin and $smax s, $sdist distinct values, all inside [0, $rnd] s"
  else
    check_bad "the $DELAY_SAMPLES samples ($list): ${stats#* * * }"
  fi
  key_line "STAMP-MTIME-BEFORE-SAMPLES: $s0"
  key_line "STAMP-MTIME-AFTER-SAMPLES: $s1"
  if verdict_eq "$s1" "$s0"; then
    check_pass "the stamp's mtime did not move during the restarts ($s0)"
  else
    check_bad "the stamp's mtime moved during the restarts ($s0 to $s1): a restart looked like a run"
  fi

  # The raw state next to the samples, so that a measurement that cannot see the
  # delay can say what was observed.
  {
    echo "---- systemctl show holzcloud-backup.timer ----"
    dx systemctl show holzcloud-backup.timer
    echo "---- systemctl list-timers holzcloud-backup.timer ----"
    dx systemctl list-timers holzcloud-backup.timer --no-pager
  } >> "$d" 2>&1 || true

  # The Docker VM steps the realtime clock every 20 to 30 seconds; each step makes
  # systemd recalculate the calendar timers and draw a new random delay. Watch the
  # shipped timer for OBSERVE_S seconds without touching it, so that the record
  # can say how often the armed time moved by itself. PID 1 logs at debug level
  # only for this window, and the level is put back.
  OBSERVE_S=${OBSERVE_S:-75}
  dx systemd-analyze log-level debug > /dev/null 2>&1 || true
  prev=""
  rerolls=0
  end=$((SECONDS + OBSERVE_S))
  while [ "$SECONDS" -lt "$end" ]; do
    n=$(bus_us holzcloud-backup.timer Timer NextElapseUSecRealtime) || n=""
    if [ -n "$prev" ] && [ "$n" != "$prev" ]; then rerolls=$((rerolls + 1)); fi
    prev=$n
    sleep 2
  done
  dx systemd-analyze log-level info > /dev/null 2>&1 || true
  logged=$(dxo bash -c 'journalctl _PID=1 --since "-$1s" --no-pager -o short-precise 2>/dev/null | grep -cE "Time has been changed"' _ $((OBSERVE_S + 10))) || logged=0
  key_line "OBSERVE-S: $OBSERVE_S"
  key_line "REROLLS-WITHOUT-RESTART: $rerolls"
  key_line "TIME-CHANGED-LOGGED: ${logged:-0}"
  dxo bash -c 'journalctl _PID=1 --since "-$1s" --no-pager -o short-precise 2>/dev/null | grep -E "Time has been changed|holzcloud-backup.timer: (Time change|Adding|Realtime timer elapses)" | sed "s/^/PID1 /"' _ $((OBSERVE_S + 10)) >> "$d" 2>&1 || true
  check_info "the shipped timer re-armed itself $rerolls time(s) in $OBSERVE_S s without a restart; systemd logged 'Time has been changed' ${logged:-0} time(s): the Docker VM steps the realtime clock and every step draws a new random delay (the armed time in fire.txt is the draw PID 1 logged last before the run, see fire-armed-log.txt)"

  # The real run of `fire`, judged from its KEY lines only.
  start=$(kv "$f" START-EPOCH)
  armed=$(kv "$f" LAST-ARMED-EPOCH)
  drawn=$(kv "$f" LAST-DRAWN-EPOCH)
  fbase=$(kv "$f" BASE-EPOCH)
  facc=$(kv "$f" ACCURACY-S)
  frnd=$(kv "$f" RANDOM-DELAY-S)
  sl=$(kv "$f" START-LOCAL)
  key_line "SLACK-S: $SLACK_FIRE"
  key_line "DELAY-USED-S: $((armed - fbase))"
  key_line "START-AFTER-ARMED-S: $((start - armed))"
  key_line "DRAWN-AFTER-ARMED-S: $((drawn - armed))"
  stats=$(fire_window_ok "$start" "$armed" "$fbase" "$frnd" "$facc" "$SLACK_FIRE" "${drawn:-0}") && rc=0 || rc=$?
  if [ "$rc" -eq 0 ]; then
    check_pass "the real run: armed $((armed - fbase)) s after 03:15:00 (inside [0, $frnd]), started $((start - armed)) s after the armed time, the draw made $((drawn - armed)) s after it (inside [0, accuracy $facc + slack $SLACK_FIRE] of whichever is later)"
  else
    check_bad "the real run: $stats"
  fi
  hi_s=$((11700 + frnd + facc + SLACK_FIRE))
  if tod_in_window "$sl" 03:15:00 "$(s_hms "$hi_s")"; then
    check_pass "the real run started at $sl local, inside [03:15:00, $(s_hms "$hi_s")] (03:15:00 + $frnd s delay + $facc s accuracy + $SLACK_FIRE s allowance)"
  else
    check_bad "the real run started at $sl local, outside [03:15:00, $(s_hms "$hi_s")]"
  fi

  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "DELAY FAILED"
    say "nightly.sh: DELAY FAILED, $BADS BAD line(s); see $d"
    exit 1
  fi
  key_line "DELAY OK"
}

# ---------------------------------------------------------------------------
# catchup: BKP-02. Needs the container of `fire`.
# A window missed while the host is down is run after it comes up, by the shipped
# Persistent=true. The zone is moved so that 03:15 falls CATCHUP_LEAD seconds
# ahead, the container is stopped before that instant and started after it, and a
# control timer with the same calendar and Persistent=false must NOT run.
# ---------------------------------------------------------------------------

CATCHUP_ARMED=0
CATCHUP_RESTORED=0

# wait_running: poll up to 120 s for the container's systemd to be running or degraded.
wait_running() {
  local end=$((SECONDS + 120)) state=""
  while [ "$SECONDS" -lt "$end" ]; do
    state=$(dx systemctl is-system-running 2> /dev/null | tr -d '\r' || true)
    case "$state" in running | degraded) return 0 ;; esac
    sleep 1
  done
  return 1
}

# catchup_restore: remove the control units and put the zone back. Idempotent; it
# runs on the normal path and from the EXIT trap, so the control timer and the
# moved zone never outlive the subcommand (T-24-05).
catchup_restore() {
  local orig end base n rnd step restored=no
  [ "$CATCHUP_ARMED" = 1 ] && [ "$CATCHUP_RESTORED" = 0 ] || return 0
  CATCHUP_RESTORED=1
  if [ "$(docker inspect -f '{{.State.Running}}' "$NAME" 2> /dev/null)" != true ]; then
    docker start "$NAME" > /dev/null 2>&1 || true
    wait_running || true
  fi
  dx systemctl disable --now p24-control.timer > /dev/null 2>&1 || true
  dx rm -f /etc/systemd/system/p24-control.timer /etc/systemd/system/p24-control.service \
    /etc/systemd/system/timers.target.wants/p24-control.timer > /dev/null 2>&1 || true
  dx systemctl daemon-reload > /dev/null 2>&1 || true
  dx systemctl reset-failed p24-control.service p24-control.timer > /dev/null 2>&1 || true
  orig=$(dxo cat "$ZONE_ORIG_FILE" 2> /dev/null) || orig=""
  if [ -n "$orig" ]; then
    if [ "$orig" = none ]; then
      dx rm -f /etc/localtime || true
    else
      dx ln -sfn "$orig" /etc/localtime || true
    fi
    rnd=$(bus_s holzcloud-backup.timer Timer RandomizedDelayUSec) || rnd=300
    base=$(base_epoch) || base=0
    for step in wait daemon-reload daemon-reexec; do
      [ "$step" = wait ] || dx systemctl "$step" > /dev/null 2>&1 || true
      end=$((SECONDS + 30))
      while [ "$SECONDS" -lt "$end" ]; do
        n=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || n=0
        if [ "$n" -ge "$base" ] && [ "$n" -le $((base + rnd)) ]; then
          restored=yes
          break
        fi
        sleep 1
      done
      [ "$restored" = yes ] && break
    done
  fi
  printf 'ZONE-RESTORED: %s (/etc/localtime -> %s, now %s)\n' "$restored" "${orig:-unknown}" "$(dxo date +%Z 2> /dev/null || echo '?')" | tee -a "$OUT_DIR/catchup.txt" > /dev/null
  say "catchup: zone restored: $restored; control units removed"
}

catchup_trap() {
  local rc=$?
  trap - EXIT
  catchup_restore
  exit "$rc"
}

cmd_catchup() {
  local c=$OUT_DIR/catchup.txt enabled active i end st act
  local stamp_t skew0 skew1 acc rnd before_s m now c0 t_stop wait_s fin_raw up_raw fin_e up_e cont_now
  local timer_active start id dir trig res es stamp_after cran cpers cstart cnext verdict rc after_boot

  need_container
  [ "$(tail -n 1 "$OUT_DIR/fire.txt" 2> /dev/null)" = "FIRE OK" ] \
    || die "$OUT_DIR/fire.txt does not end FIRE OK; run 'bash deploy/test/nightly.sh fire' first"
  enabled=$(dxo systemctl is-enabled holzcloud-backup.timer 2> /dev/null) || enabled=""
  active=$(dxo systemctl is-active holzcloud-backup.timer 2> /dev/null) || active=""
  [ "$enabled" = enabled ] && [ "$active" = active ] \
    || die "holzcloud-backup.timer is '$enabled' and '$active' in $NAME, expected enabled and active"
  dx test -f "$STAMP" || die "the stamp $STAMP does not exist in $NAME; run 'bash deploy/test/nightly.sh fire' first"

  BADS=0
  res_begin "$c"
  trap catchup_trap EXIT
  CATCHUP_ARMED=1

  # (1) The evidence before the stop.
  skew0=$(clock_skew)
  acc=$(bus_s holzcloud-backup.timer Timer AccuracyUSec) || die "could not read AccuracyUSec"
  rnd=$(bus_s holzcloud-backup.timer Timer RandomizedDelayUSec) || die "could not read RandomizedDelayUSec"
  stamp_t=$(dxo stat -c '%Y' "$STAMP") || die "could not stat the stamp"
  {
    echo "---- list-timers before the zone moves ----"
    dx systemctl list-timers holzcloud-backup.timer --no-pager
    echo "---- the stamp ----"
    dx stat -c '%n %Y %y' "$STAMP"
    echo "---- systemctl show holzcloud-backup.timer ----"
    dx systemctl show holzcloud-backup.timer -p Persistent -p RandomizedDelayUSec -p AccuracyUSec -p TimersCalendar -p ActiveEnterTimestamp -p LastTriggerUSec -p DropInPaths
  } > "$OUT_DIR/catchup-before.txt" 2>&1 || true
  key_line "CLOCK-SKEW-S-BEFORE: $skew0"
  key_line "RANDOM-DELAY-S: $rnd"
  key_line "ACCURACY-S: $acc"
  key_line "SLACK-S: $SLACK_CATCHUP"

  # (2) The control, in new files: the same calendar and delay, Persistent=false,
  # its own service. Never a drop-in on a shipped unit.
  printf '[Unit]\nDescription=phase 24 control: the shipped calendar without Persistent=\n\n[Service]\nType=oneshot\nExecStart=/bin/echo p24-control ran\n' \
    | dxi bash -c 'cat > /etc/systemd/system/p24-control.service' || die "could not write p24-control.service"
  printf '[Unit]\nDescription=phase 24 control timer: OnCalendar and RandomizedDelaySec of the shipped timer, Persistent=false\n\n[Timer]\nOnCalendar=*-*-* 03:15:00\nRandomizedDelaySec=300\nPersistent=false\n\n[Install]\nWantedBy=timers.target\n' \
    | dxi bash -c 'cat > /etc/systemd/system/p24-control.timer' || die "could not write p24-control.timer"
  dx systemctl daemon-reload || die "daemon-reload failed"
  dx systemctl enable --now p24-control.timer > /dev/null 2>&1 || die "could not enable the control timer"
  deviation "DEVIATION[control-timer]: a container-only control, p24-control.timer with the calendar and delay of the shipped timer (OnCalendar=*-*-* 03:15:00, RandomizedDelaySec=300) and Persistent=false, aimed at its own p24-control.service (/bin/echo p24-control ran). New files, not a drop-in on a shipped unit; removed at the end. It is the evidence that the catch-up belongs to the shipped Persistent=true"
  deviation "DEVIATION[restart]: a container stop and start replaces a power cycle. PID 1 and every unit restart, and the writable layer, where the stamp lives, survives; no kernel boots, no firmware runs, no network comes up from cold, and the machine is not really off. The catch-up logic that is observed is systemd's own, reading the stamp at the timer's start; whether a real boot orders it the same way is an owner step, not a claim"

  # (3) The zone: 03:15 falls CATCHUP_LEAD seconds ahead.
  before_s=$(bus_s holzcloud-backup.timer Timer NextElapseUSecRealtime) || before_s=0
  # A day apart from the zone of `fire`: backup.sh names a backup after the local
  # date and time to the second, and a first attempt landed on the very second of
  # the fire run's directory ("refusing to overwrite existing file", exit 1) because
  # both zones read 03:16:16 on the same local date. A real catch-up is on a later
  # day; the shifted offset keeps the time of day and changes the date.
  move_zone "$CATCHUP_LEAD" Catchup 1
  deviation "DEVIATION[zone]: for the missed window the zone was moved a second time, to $ZONE_NAME (STDOFF $ZONE_STD, offset $ZONE_OFF s, a whole day away from the zone of the first run so that the local date differs and backup.sh cannot name two backups alike), by files only; the local time read $ZONE_READ. No command sets a clock"
  follow_zone "$CATCHUP_LEAD" "$before_s" || {
    check_bad "systemd did not follow the zone change: the shipped timer is not armed for the new 03:15:00"
    key_line "CATCHUP FAILED"
    exit 1
  }
  m=$(base_epoch)
  now=$(cont_epoch)
  cnext=$(bus_s p24-control.timer Timer NextElapseUSecRealtime) || cnext=0
  key_line "ZONE: $ZONE_NAME STDOFF $ZONE_STD OFFSET-S $ZONE_OFF READ $ZONE_READ LEAD-S $CATCHUP_LEAD"
  key_line "STAMP-BEFORE-STOP: $stamp_t"
  key_line "MISSED-EPOCH: $m"
  key_line "MISSED-LOCAL: $(dxo date -d "@$m" +%H:%M:%S)"
  key_line "CONTROL-NEXT-EPOCH: $cnext"
  if [ $((m - now)) -lt 90 ]; then
    check_bad "only $((m - now)) s were left before the missed instant after the zone had followed (needed 90)"
    key_line "CATCHUP FAILED"
    exit 1
  fi
  if [ "$cnext" -ge "$m" ] && [ "$cnext" -le $((m + rnd + acc)) ]; then
    check_pass "the control timer is armed for the same missed instant (next $cnext, missed $m)"
  else
    check_bad "the control timer is not armed for the missed instant (next $cnext, missed $m): its silence would prove nothing"
  fi
  {
    echo "---- list-timers after the zone moved, before the stop ----"
    dx systemctl list-timers holzcloud-backup.timer p24-control.timer --no-pager
  } >> "$OUT_DIR/catchup-before.txt" 2>&1 || true

  # (4) The stop: a clean systemd shutdown. Everything needed from before it was
  # saved above and in fire-journal.txt; /run is a tmpfs here, so the unit states
  # (and with them ExecMainStartTimestamp) start again from nothing. /tmp is
  # emptied at every boot too: not by a tmpfs (it has no mount of its own) but by
  # the tmpfiles.d rule 'D /tmp 1777 root root -' of systemd, which is written
  # down below as TMP-AT-BOOT. The staged test image of run.sh baseline would be
  # gone after the start; later stages (the restore on the second host, run.sh
  # backup's state capture) read it. It is kept under /root for the stop and put
  # back at the start, and the facts are written down.
  if dx test -f /tmp/hc-upload.jpg; then
    dx install -m 600 /tmp/hc-upload.jpg /root/.nightly-hc-upload.jpg || die "could not keep /tmp/hc-upload.jpg under /root for the stop"
    key_line "TEST-IMAGE-KEPT-FOR-THE-STOP: $(dxo sha256sum /tmp/hc-upload.jpg | cut -d' ' -f1)"
  else
    key_line "TEST-IMAGE-KEPT-FOR-THE-STOP: none (/tmp/hc-upload.jpg is absent before the stop)"
  fi
  c0=$(cont_epoch)
  t_stop=$SECONDS
  docker stop -t 60 "$NAME" > /dev/null || die "docker stop failed"
  fin_raw=$(docker inspect -f '{{.State.FinishedAt}}' "$NAME")

  # (5) Down until the missed instant has passed with margin: a duration on the
  # host's SECONDS counter, not a reading of the host date.
  wait_s=$((m - c0 + 40))
  say "catchup: container down; waiting $wait_s s (counted from the stop) so that $m has passed"
  while [ $((SECONDS - t_stop)) -lt "$wait_s" ]; do sleep 1; done

  # (6) The start.
  docker start "$NAME" > /dev/null || die "docker start failed"
  up_raw=$(docker inspect -f '{{.State.StartedAt}}' "$NAME")
  wait_running || {
    check_bad "systemd was not running or degraded within 120 s of the start"
    key_line "CATCHUP FAILED"
    exit 1
  }
  if dx test -f /root/.nightly-hc-upload.jpg; then
    if dx test -f /tmp/hc-upload.jpg; then
      key_line "TEST-IMAGE-AFTER-THE-START: still in /tmp"
    else
      dx install -m 644 /root/.nightly-hc-upload.jpg /tmp/hc-upload.jpg || die "could not put /tmp/hc-upload.jpg back"
      key_line "TEST-IMAGE-AFTER-THE-START: gone from /tmp with the restart, put back from /root: $(dxo sha256sum /tmp/hc-upload.jpg | cut -d' ' -f1)"
      key_line "TMP-AT-BOOT: tmpfiles.d says '$(dxo grep -h '^D /tmp ' /usr/lib/tmpfiles.d/tmp.conf | head -n 1)'; /tmp has a mount of its own: $(if dx findmnt /tmp > /dev/null 2>&1; then echo yes; else echo no; fi)"
    fi
    dx rm -f /root/.nightly-hc-upload.jpg || true
  fi
  cont_now=$(cont_epoch)
  fin_e=$(dxo date -u -d "$fin_raw" +%s)
  up_e=$(dxo date -u -d "$up_raw" +%s)
  {
    echo "container clock before the stop: $c0"
    echo "docker FinishedAt: $fin_raw ($fin_e)"
    echo "docker StartedAt:  $up_raw ($up_e)"
    echo "container clock after the start: $cont_now"
    echo "missed instant: $m"
  } > "$OUT_DIR/catchup-docker.txt"
  key_line "CLOCK-BEFORE-STOP: $c0"
  key_line "FINISHED-AT-RAW: $fin_raw"
  key_line "STARTED-AT-RAW: $up_raw"
  key_line "DOWN-FROM: $fin_e"
  key_line "UP-AT: $up_e"
  key_line "CLOCK-AFTER-START: $cont_now"

  # (7) Wait for the run, bounded; then 30 s more, so that a control that was
  # going to run would have run.
  end=$((SECONDS + CATCHUP_WAIT))
  st=0
  while [ "$SECONDS" -lt "$end" ]; do
    st=$(bus_us holzcloud-backup.service Service ExecMainStartTimestamp) || st=0
    act=$(show_v holzcloud-backup.service ActiveState) || act=""
    if [ "${st:-0}" -gt 0 ]; then
      case "$act" in activating | active | reloading) ;; *) break ;; esac
    fi
    sleep 1
    st=0
  done
  if [ "${st:-0}" -le 0 ]; then
    say "nightly.sh: holzcloud-backup.service did not run within ${CATCHUP_WAIT}s of the start; the timer and the unit journal:"
    dx systemctl list-timers holzcloud-backup.timer --no-pager >&2 || true
    dx journalctl -u holzcloud-backup.service --no-pager -n 40 >&2 || true
    check_bad "the shipped timer did not start the service within ${CATCHUP_WAIT} s of the start (the missed window was not run)"
    key_line "CATCHUP FAILED"
    exit 1
  fi
  sleep 30

  # (8) Collect.
  dx systemctl list-timers holzcloud-backup.timer p24-control.timer --no-pager > "$OUT_DIR/catchup-after.txt" 2>&1 || true
  id=$(show_v holzcloud-backup.service InvocationID) || id=""
  # The journal of this invocation only. The journal here survives the restart
  # (/var/log/journal exists) and the kernel's boot id is the Docker VM's, so it
  # is the same for both starts: -b would not separate the runs, the invocation
  # does. The directory is read from nowhere else.
  dx journalctl -u holzcloud-backup.service -o short-precise --no-pager "_SYSTEMD_INVOCATION_ID=$id" > "$OUT_DIR/catchup-journal.txt" 2>&1 || true
  timer_active=$(bus_s holzcloud-backup.timer Unit ActiveEnterTimestamp) || timer_active=0
  start=$(bus_s holzcloud-backup.service Service ExecMainStartTimestamp) || start=0
  trig=$(show_v holzcloud-backup.service TriggeredBy) || trig=""
  res=$(show_v holzcloud-backup.service Result) || res=""
  es=$(show_v holzcloud-backup.service ExecMainStatus) || es=""
  dir=$(sed -n 's/.*=== Backup complete: \(.*\) ===$/\1/p' "$OUT_DIR/catchup-journal.txt" | tail -n 1 | tr -d '\r')
  stamp_after=$(dxo stat -c '%Y %y' "$STAMP" 2> /dev/null) || stamp_after="missing"
  cstart=$(bus_s p24-control.service Service ExecMainStartTimestamp) || cstart=0
  cpers=$(show_v p24-control.timer Persistent) || cpers="?"
  if [ "${cstart:-0}" -gt 0 ] || dx journalctl -u p24-control.service --no-pager 2> /dev/null | grep -q 'p24-control ran'; then
    cran=yes
  else
    cran=no
  fi
  skew1=$(clock_skew)
  key_line "TIMER-ACTIVE-EPOCH: $timer_active"
  key_line "START-EPOCH: $start"
  key_line "START-LOCAL: $(dxo date -d "@$start" +%H:%M:%S)"
  key_line "TRIGGERED-BY: $trig"
  key_line "RESULT: $res"
  key_line "EXIT-STATUS: $es"
  key_line "INVOCATION-ID: $id"
  key_line "DIR: ${dir:-none}"
  key_line "DELAY-AFTER-BOOT-S: $((start - timer_active))"
  key_line "STAMP-AFTER: $stamp_after"
  key_line "CONTROL-RAN: $cran"
  key_line "CONTROL-PERSISTENT: $cpers"
  key_line "CLOCK-SKEW-S-AFTER: $skew1"

  # (9) Judgement.
  verdict=$(catchup_ok "$m" "$fin_e" "$up_e" "$timer_active" "$start" "$rnd" "$acc" "$SLACK_CATCHUP" "$stamp_t") && rc=0 || rc=$?
  if [ "$rc" -eq 0 ]; then
    after_boot=$((start - timer_active))
    check_pass "a missed window was run after the boot: stamp $stamp_t < missed $m, down at $fin_e, up at $up_e, timer active at $timer_active, run at $start ($after_boot s after the activation; limit max(missed, activation) + $rnd + $acc + $SLACK_CATCHUP)"
  else
    check_bad "catch-up: $verdict"
  fi
  if verdict_eq "$trig" holzcloud-backup.timer; then
    check_pass "the service was started by holzcloud-backup.timer (TriggeredBy), not by hand"
  else
    check_bad "TriggeredBy is '$trig', expected holzcloud-backup.timer"
  fi
  if verdict_eq "$res $es" "success 0"; then
    check_pass "Result success and ExecMainStatus 0"
  else
    check_bad "Result '$res' and ExecMainStatus '$es', expected success and 0"
  fi
  if [ -n "$dir" ] && dx test -d "$dir"; then
    check_pass "the directory the journal names exists: $dir"
  else
    check_bad "the directory the journal names ('${dir:-none}') does not exist"
  fi
  if verdict_eq "$cran$cpers" "nono"; then
    check_pass "the control timer (same calendar, Persistent=$cpers) did not run after the same stop and start: the catch-up belongs to the shipped Persistent=true"
  else
    check_bad "control: ran '$cran', Persistent '$cpers' (expected: not run, Persistent no)"
  fi
  if in_range "$skew0" -120 120 && in_range "$skew1" -120 120; then
    check_pass "the container clock stayed within 120 s of the host's (before $skew0 s, after $skew1 s)"
  else
    check_bad "the container clock is off the host's by $skew0 s before and $skew1 s after (limit 120 s): the clock was moved"
  fi
  if [ -n "$dir" ] && dx test -f "$dir/holzcloud.sqlite"; then
    verify_dir "$dir" catchup
  else
    res_begin "$OUT_DIR/verify-catchup.txt"
    check_bad "the directory '${dir:-none}' holds no holzcloud.sqlite to verify"
  fi
  CHECK_OUT=$c

  # (10) Clean up: control units out, original zone back.
  catchup_restore
  CHECK_OUT=$c
  trap - EXIT
  if grep -q '^ZONE-RESTORED: yes' "$c"; then
    check_pass "the zone is restored and the shipped timer followed"
  else
    check_bad "the original zone was not restored or the timer did not follow (ZONE-RESTORED)"
  fi
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "CATCHUP FAILED"
    say "nightly.sh: CATCHUP FAILED, $BADS BAD line(s); see $c and verify-catchup.txt"
    exit 1
  fi
  key_line "CATCHUP OK"
}

# ---------------------------------------------------------------------------
# The second host (plan 02). TGT_NAME is the container that receives the copy of
# `offsite` and is restored onto by `offsite-restore`.
# ---------------------------------------------------------------------------

dxt() { docker exec "$TGT_NAME" "$@"; }
dxti() { docker exec -i "$TGT_NAME" "$@"; }
dxto() { docker exec "$TGT_NAME" "$@" | tr -d '\r'; }

# age_dh SECONDS: "31 d 1 h" for a number of seconds.
age_dh() { printf '%d d %d h\n' $(($1 / 86400)) $(($1 % 86400 / 3600)); }

# units_shipped: 0 when both installed units are byte-equal to deploy/, else 1.
units_shipped() {
  local f hs cs rc=0
  for f in holzcloud-backup.service holzcloud-backup.timer; do
    hs=$(shasum -a 256 "deploy/$f" | cut -d' ' -f1)
    cs=$(dxo sha256sum "/etc/systemd/system/$f" | cut -d' ' -f1) || cs="?"
    [ "$hs" = "$cs" ] || rc=1
  done
  return "$rc"
}

# ---------------------------------------------------------------------------
# prune: BKP-05. Directories of a known age are planted next to the real backups
# and ONE run of the shipped service shows which of them the script removed.
# ---------------------------------------------------------------------------

cmd_prune() {
  local p=$OUT_DIR/prune.txt b=/opt/holzcloud/backups
  local envs ret now before_names planted age e nm expect obs
  local id0 id res es rc dir om dmt dage gone_min kept_max after_names m
  local n_gone n_kept cleaned st31 st30 age31=$((31 * 86400 + 3600)) age30=$((30 * 86400 + 12 * 3600))

  need_container
  mkdir -p "$OUT_DIR"
  units_shipped \
    || die "the installed holzcloud-backup units differ from deploy/ in $NAME: prune is played against the shipped units, before 'offsite' changes the REMOTE_TARGET line; put them back with: docker exec $NAME cp /opt/holzcloud/deploy/holzcloud-backup.service /opt/holzcloud/deploy/holzcloud-backup.timer /etc/systemd/system/"
  envs=$(show_v holzcloud-backup.service Environment) || envs=""
  ret=$(printf '%s\n' "$envs" | tr ' ' '\n' | sed -n 's/^RETENTION_DAYS=//p' | tail -n 1)
  case "$ret" in "" | *[!0-9]*) die "the service of $NAME names no numeric RETENTION_DAYS (Environment: $envs)" ;; esac
  case " $envs " in
    *" REMOTE_TARGET="*) die "the service of $NAME already sets REMOTE_TARGET: prune is played with it unset; put the unit back (see above)" ;;
  esac
  before_names=$(dxo bash -c 'find "$1" -mindepth 1 -maxdepth 1 -type d -printf "%f\n" | sort' _ "$b") || before_names=""
  [ -n "$before_names" ] \
    || die "$b of $NAME holds no directory: run 'bash deploy/test/run.sh backup' (or 'source') first"

  BADS=0
  res_begin "$p"
  key_line "RETENTION-DAYS-FROM-UNIT: $ret"
  key_line "ENVIRONMENT-OF-UNIT: $envs"
  check_pass "the installed units are byte-equal to deploy/, RETENTION_DAYS=$ret is read from the unit and REMOTE_TARGET is not set"

  # Step 1: the record before.
  now=$(cont_epoch)
  {
    echo "---- container clock (epoch): $now"
    echo "---- ls -l --time-style=full-iso $b"
    dx ls -l --time-style=full-iso "$b"
    echo "---- stat -c '%n %Y' $b"
    dx stat -c '%n %Y' "$b"
    echo "---- directories that exist before the planting (the must-stay set)"
    printf '%s\n' "$before_names"
  } > "$OUT_DIR/prune-before.txt" 2>&1 || true
  key_line "MUST-STAY: $(printf '%s' "$before_names" | tr '\n' ' ')"

  # Step 2: plant five directories as the service user, the mtime set last.
  planted=$(dxi bash -s -- "$((45 * 86400))" "$age31" "$age30" "$((29 * 86400))" "$((1 * 86400))" << 'INNER' | tr -d '\r'
now=$(date +%s)
for age in "$@"; do
  e=$((now - age))
  nm=$(date -d "@$e" +%Y%m%d-%H%M%S)
  runuser -u holzcloud -- bash -c 'set -e; d=/opt/holzcloud/backups/$1
    mkdir -m 700 "$d"; echo planted > "$d/holzcloud.sqlite"
    mkdir -m 700 "$d/media"; echo planted > "$d/media/file.txt"
    touch -d "@$2" "$d/media" "$d"' _ "$nm" "$e" || exit 1
  echo "$nm $age $e"
done
INNER
  ) || die "the planting in $NAME failed"
  [ "$(printf '%s\n' "$planted" | grep -c .)" -eq 5 ] || die "expected five planted directories, got: $planted"
  while read -r nm age e; do
    expect=$(prune_expect "$age" "$ret")
    key_line "PLANTED $nm $age $expect"
    key_line "PLANTED-DETAIL $nm $(age_dh "$age") old, owner and mode $(dxo stat -c '%U %a' "$b/$nm"), holds $(dxo bash -c 'find "$1" -type f | wc -l' _ "$b/$nm") files, one of them in a subdirectory"
  done <<< "$planted"

  # Step 3: one run of the shipped service, by hand (the timer was proven in plan 01).
  key_line "START-BY: systemctl start holzcloud-backup.service by hand, the oneshot returns when the script is done; the shipped timer was proven in plan 01"
  id0=$(show_v holzcloud-backup.service InvocationID) || id0=""
  rc=0
  dx systemctl start holzcloud-backup.service > "$OUT_DIR/prune-start.txt" 2>&1 || rc=$?
  id=$(show_v holzcloud-backup.service InvocationID) || id=""
  res=$(show_v holzcloud-backup.service Result) || res=""
  es=$(show_v holzcloud-backup.service ExecMainStatus) || es=""
  dx journalctl -u holzcloud-backup.service -o short-precise --no-pager "_SYSTEMD_INVOCATION_ID=$id" > "$OUT_DIR/prune-journal.txt" 2>&1 || true
  dir=$(sed -n 's/.*=== Backup complete: \(.*\) ===$/\1/p' "$OUT_DIR/prune-journal.txt" | tail -n 1 | tr -d '\r')
  key_line "START-RC: $rc"
  key_line "RESULT: $res"
  key_line "EXIT-STATUS: $es"
  key_line "INVOCATION-ID: $id"
  key_line "DIR: ${dir:-none}"
  if [ -n "$id" ] && [ "$id" != "$id0" ]; then
    check_pass "the run is a new invocation of the service ($id)"
  else
    check_bad "the invocation did not change ($id0 before, $id after)"
  fi
  if [ "$rc" -eq 0 ] && [ "$res $es" = "success 0" ]; then
    check_pass "Result success and ExecMainStatus 0"
  else
    check_bad "start rc $rc, Result '$res', ExecMainStatus '$es', expected 0, success and 0 (prune-start.txt, prune-journal.txt)"
  fi
  if grep -q "Pruning backups older than $ret days" "$OUT_DIR/prune-journal.txt"; then
    check_pass "the journal of the invocation holds the script's 'Pruning backups older than $ret days...' line"
  else
    check_bad "the journal of the invocation holds no 'Pruning backups older than $ret days' line (prune-journal.txt)"
  fi
  if [ -n "$dir" ]; then
    check_pass "the journal holds the script's '=== Backup complete: $dir ===' line"
  else
    check_bad "the journal holds no '=== Backup complete: DIR ===' line (prune-journal.txt)"
  fi

  # Step 4: the record after, and every planted directory against its expectation.
  {
    echo "---- container clock (epoch): $(cont_epoch)"
    echo "---- ls -l --time-style=full-iso $b"
    dx ls -l --time-style=full-iso "$b"
    echo "---- stat -c '%n %Y' $b"
    dx stat -c '%n %Y' "$b"
  } > "$OUT_DIR/prune-after.txt" 2>&1 || true
  gone_min=0
  kept_max=0
  n_gone=0
  n_kept=0
  st31=?
  st30=?
  while read -r nm age e; do
    expect=$(prune_expect "$age" "$ret")
    if dx test -d "$b/$nm"; then obs=present; else obs=gone; fi
    key_line "OBSERVED $nm $obs"
    [ "$age" -ne "$age31" ] || st31=$obs
    [ "$age" -ne "$age30" ] || st30=$obs
    if [ "$obs" = gone ]; then
      n_gone=$((n_gone + 1))
      if [ "$gone_min" -eq 0 ] || [ "$age" -lt "$gone_min" ]; then gone_min=$age; fi
    else
      n_kept=$((n_kept + 1))
      if [ "$age" -gt "$kept_max" ]; then kept_max=$age; fi
    fi
    if { [ "$expect" = DELETE ] && [ "$obs" = gone ]; } || { [ "$expect" = KEEP ] && [ "$obs" = present ]; }; then
      check_pass "planted $nm ($(age_dh "$age") old, retention $ret d): expected $expect, observed $obs"
    else
      check_bad "planted $nm ($(age_dh "$age") old, retention $ret d): expected $expect, observed $obs (prune-after.txt)"
    fi
  done <<< "$planted"
  after_names=$(dxo bash -c 'find "$1" -mindepth 1 -maxdepth 1 -type d -printf "%f\n" | sort' _ "$b") || after_names=""
  m=0
  while read -r nm; do
    [ -n "$nm" ] || continue
    if printf '%s\n' "$after_names" | grep -qx "$nm"; then
      m=$((m + 1))
    else
      check_bad "the earlier backup $nm is gone after the run"
    fi
  done <<< "$before_names"
  if [ "$m" -eq "$(printf '%s\n' "$before_names" | grep -c .)" ]; then
    check_pass "every directory that existed before the planting is still there ($m)"
  fi
  if [ -n "$dir" ] && dx test -d "$dir"; then
    om=$(dxo stat -c '%U %a' "$dir") || om="?"
    dmt=$(dxo stat -c '%Y' "$dir") || dmt=0
    dage=$(($(cont_epoch) - dmt))
    key_line "DIR-OWNER-MODE: $om"
    key_line "DIR-AGE-S: $dage"
    if [ "$om" = "holzcloud 700" ] && [ "$dage" -ge 0 ] && [ "$dage" -lt 3600 ]; then
      check_pass "the new backup $dir exists, is $om and $dage s old"
    else
      check_bad "the new backup $dir: '$om', age $dage s (expected 'holzcloud 700' and under one hour)"
    fi
  else
    check_bad "the new backup directory '${dir:-none}' does not exist"
  fi

  # Step 5: the boundary, stated only when it was observed.
  if [ "$n_gone" -gt 0 ] && [ "$n_kept" -gt 0 ]; then
    key_line "OBSERVED-BOUNDARY: the youngest planted directory that was deleted was $(age_dh "$gone_min") old, the oldest that was kept was $(age_dh "$kept_max") old (retention $ret days)"
  fi
  if [ "$st31" = gone ] && [ "$st30" = present ]; then
    check_info "OBSERVED RULE: a directory is deleted once floor(age / 24 h) exceeds RETENTION_DAYS, which is from $((ret + 1)) days of age; the 30 day 12 hour directory, older than 30 days by less than a day, was kept; by the same rule it becomes eligible 12 hours later (a later run was not played). This is a measured fact; whether it is a defect is plan 03's rule"
  fi

  # Step 6: the recorded cleanup of what survived.
  cleaned=0
  while read -r nm age e; do
    if dx test -d "$b/$nm"; then
      dx rm -rf "$b/$nm"
      echo "CLEANUP removed $nm" | tee -a "$p"
      cleaned=$((cleaned + 1))
    fi
  done <<< "$planted"
  key_line "CLEANUP-COUNT: $cleaned"
  dx ls -l --time-style=full-iso "$b" > "$OUT_DIR/prune-final.txt" 2>&1 || true

  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "PRUNE FAILED"
    say "nightly.sh: PRUNE FAILED, $BADS BAD line(s); see $p, prune-before.txt, prune-after.txt, prune-journal.txt"
    exit 1
  fi
  key_line "PRUNE OK"
}

# ---------------------------------------------------------------------------
# offsite: BKP-04, the copy. The shipped service, changed only by the one line
# its own comment says to uncomment, copies the backup root to a second host.
# Which way the credentials reach it under the shipped sandbox is MEASURED:
# the document and the unit name none.
# ---------------------------------------------------------------------------

NAT_FILE=/dev/null
NAT_RC=0
NAT_OUT=""
OFFA_RC=0
OFFA_RES=""
OFFA_ES=""
OFFA_DIR=""
OFFA_ON=no
OFFA_STRAY_BEFORE=no
OFFA_STRAY_AFTER=no

nat_note() { printf '%s\n' "$*" >> "$NAT_FILE"; }

# nat_run TITLE CMD...: run a command, keep its raw output and status in
# offsite-natural.txt (the record of the credential experiments) and in NAT_*.
nat_run() {
  local t=$1 rc=0 out
  shift
  out=$("$@" 2>&1) || rc=$?
  {
    printf '==== %s\n$ %s\n' "$t" "$*"
    [ -z "$out" ] || printf '%s\n' "$out"
    printf '[exit %s]\n\n' "$rc"
  } >> "$NAT_FILE"
  NAT_RC=$rc
  NAT_OUT=$out
}

# apt_in CONTAINER PACKAGE OUTFILE: install one package; a missing network is
# DEVIATION[network] and exit 3, as run.sh does.
apt_in() {
  local c=$1 pkg=$2 out=$3 rc=0
  {
    echo "---- apt-get update and install $pkg in $c"
    docker exec -e DEBIAN_FRONTEND=noninteractive "$c" bash -c "apt-get update -qq && apt-get install -y --no-install-recommends $pkg"
  } >> "$out" 2>&1 || rc=$?
  if [ "$rc" -ne 0 ] && grep -Eqi 'could not resolve|temporary failure resolving|network is unreachable|failed to connect|connection timed out|name or service not known|unable to locate package' "$out"; then
    deviation "DEVIATION[network]: apt-get install $pkg in $c failed with no route to the package archive (see $out)"
    exit 3
  fi
  [ "$rc" -eq 0 ] || die "apt-get install $pkg in $c failed (see $out)"
}

# list_backup_names: the directory names directly below the backup root, sorted.
list_backup_names() { dxo bash -c 'find /opt/holzcloud/backups -mindepth 1 -maxdepth 1 -type d -printf "%f\n" | sort'; }

# sha_list_in CONTAINER ROOT: the sha256 of every file below ROOT, sorted, as root.
sha_list_in() {
  docker exec "$1" bash -c 'cd "$1" && find . -type f -print0 | sort -z | xargs -0 --no-run-if-empty sha256sum' _ "$2" | tr -d '\r'
}

plant_stray() {
  dxt bash -c 'rm -rf /backups/holzcloud/stray-not-in-source
runuser -u offsite -- bash -c "mkdir -m 700 /backups/holzcloud/stray-not-in-source && echo stray > /backups/holzcloud/stray-not-in-source/file.txt"'
}

# authorize_pub PUBFILE: only the public key crosses to the second host, by exec-pipe.
authorize_pub() {
  dx cat "$1" | dxti bash -c 'rm -f /home/offsite/.ssh/authorized_keys; install -d -o offsite -g offsite -m 700 /home/offsite/.ssh
cat > /home/offsite/.ssh/authorized_keys
chown offsite:offsite /home/offsite/.ssh/authorized_keys; chmod 600 /home/offsite/.ssh/authorized_keys'
}

# offsite_attempt LABEL JOURNALFILE: one start of the shipped service, judged by
# its own result and by whether its directory is on the second host.
offsite_attempt() {
  local jf=$2 before after id nm
  plant_stray
  if dxt test -d /backups/holzcloud/stray-not-in-source; then OFFA_STRAY_BEFORE=yes; else OFFA_STRAY_BEFORE=no; fi
  before=$(list_backup_names)
  OFFA_RC=0
  dx systemctl start holzcloud-backup.service > "$OUT_DIR/offsite-start-$1.txt" 2>&1 || OFFA_RC=$?
  id=$(show_v holzcloud-backup.service InvocationID) || id=""
  OFFA_RES=$(show_v holzcloud-backup.service Result) || OFFA_RES=""
  OFFA_ES=$(show_v holzcloud-backup.service ExecMainStatus) || OFFA_ES=""
  dx journalctl -u holzcloud-backup.service -o short-precise --no-pager "_SYSTEMD_INVOCATION_ID=$id" > "$jf" 2>&1 || true
  after=$(list_backup_names)
  # The script prints its complete line only after the copy, so a failed copy names
  # no directory: the new one is the one present after and not before.
  nm=$(comm -13 <(printf '%s\n' "$before") <(printf '%s\n' "$after") | tail -n 1)
  OFFA_DIR=$nm
  if [ -n "$nm" ] && dxt test -d "/backups/holzcloud/$nm"; then OFFA_ON=yes; else OFFA_ON=no; fi
  if dxt test -d /backups/holzcloud/stray-not-in-source; then OFFA_STRAY_AFTER=yes; else OFFA_STRAY_AFTER=no; fi
}

# leak_ssh: the private keys and the known_hosts content of the experiments are
# not in any output file. A key body line and the key blob of a host are looked
# up as fixed strings.
leak_ssh() {
  local f line d hit=0 ssh_home=$1
  for f in "$ssh_home/.ssh/id_ed25519" /opt/holzcloud/offsite-ssh/id_ed25519; do
    dx test -f "$f" || continue
    line=$(dxo sed -n 2p "$f")
    [ "${#line}" -ge 16 ] || continue
    for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
      if grep -rqF -- "$line" "$d"; then
        say "LEAK: a line of the private key $f occurs in a file under $d"
        hit=1
      fi
    done
  done
  for f in "$ssh_home/.ssh/known_hosts" /opt/holzcloud/offsite-ssh/known_hosts; do
    dx test -f "$f" || continue
    line=$(dxo awk '!/^#/ && NF >= 3 { print $3; exit }' "$f")
    [ "${#line}" -ge 16 ] || continue
    for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
      if grep -rqF -- "$line" "$d"; then
        say "LEAK: the host key in $f occurs in a file under $d"
        hit=1
      fi
    done
  done
  [ "$hit" -eq 0 ] || exit 1
  say "SSH-LEAK-CHECK OK"
}

cmd_offsite() {
  local o=$OUT_DIR/offsite.txt h2=$TGT_OUT_DIR/offsite-host2.txt nat=$OUT_DIR/offsite-natural.txt
  local want=offsite@nas:/backups/holzcloud unitf=/etc/systemd/system/holzcloud-backup.service
  local repo_unit=/opt/holzcloud/deploy/holzcloud-backup.service f ip home_dir home_existed hs cs ulines tenv drop
  local ssh_src_before ssh_h2_before before_a3 after_a3 a3_dir a3_on a3_rc
  local a4_rc a4_res a4_es a4_dir a4_on b_rc b_res b_es b_dir b_on
  local working=none fin_dir fin_rc fin_res fin_es fin_stray_before fin_stray_after
  local sl tl nfiles dirs_src dirs_h2 bad_modes line

  need_container
  dxt true > /dev/null 2>&1 \
    || die "the second host $TGT_NAME does not answer; bring it up with: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/nightly.sh target"
  [ "$NAME" != "$TGT_NAME" ] || die "TGT_NAME equals NAME ($NAME)"
  for f in tree-src-backup.txt tree-backup.txt backup.txt state-src.txt; do
    [ -f "$OUT_DIR/$f" ] \
      || die "$OUT_DIR/$f is missing; run: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/run.sh backup"
  done
  mkdir -p "$OUT_DIR" "$TGT_OUT_DIR"
  ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$TGT_NAME")
  [ -n "$ip" ] || die "the second host $TGT_NAME has no IP address"

  BADS=0
  res_begin "$o"
  : > "$h2"
  : > "$nat"
  NAT_FILE=$nat
  key_line "HOST2: $TGT_NAME at $ip"

  # The state of the two hosts before anything is installed.
  if dx bash -c 'command -v ssh > /dev/null'; then ssh_src_before=present; else ssh_src_before=absent; fi
  if dxt bash -c 'command -v sshd > /dev/null'; then ssh_h2_before=present; else ssh_h2_before=absent; fi
  key_line "SSH-CLIENT-ON-SOURCE-BEFORE: $ssh_src_before"
  key_line "SSH-SERVER-ON-SECOND-HOST-BEFORE: $ssh_h2_before"

  # Preparation. DEVIATION[ssh-packages], DEVIATION[hosts], DEVIATION[offsite-host].
  deviation "DEVIATION[ssh-packages]: the document's Prerequisites install rsync, whose package only suggests an ssh client and an ssh server (ssh on the source: $ssh_src_before before, sshd on the second host: $ssh_h2_before before); openssh-client was installed in the source and openssh-server in the second host with apt-get install --no-install-recommends. A real off-site target already runs sshd, and an operator's machine has an ssh client; the document does not say so"
  deviation "DEVIATION[offsite-host]: the second host is a second fresh container on the same bridge network ($TGT_NAME at $ip), not another machine on another network. What it cannot show: a real network path, a real NAS, a firewall, a different distribution on the receiving side"
  deviation "DEVIATION[hosts]: the second host is reached as nas by an /etc/hosts line in the source ($ip nas), appended with an append redirection because Docker bind-mounts that file; there is no DNS for it"
  apt_in "$NAME" openssh-client "$OUT_DIR/offsite-apt.txt"
  apt_in "$TGT_NAME" openssh-server "$h2"
  dxt bash -c 'command -v rsync' > /dev/null || die "rsync is not installed in $TGT_NAME: the document's Prerequisites install it, and a receiving rsync is needed"
  dxt bash -c 'id offsite > /dev/null 2>&1 || useradd -m -s /bin/bash offsite
usermod -p "*" offsite
mkdir -p /backups/holzcloud; chown offsite:offsite /backups/holzcloud; chmod 700 /backups/holzcloud
systemctl enable --now ssh' >> "$h2" 2>&1 || die "could not prepare the account and the ssh service in $TGT_NAME (see $h2)"
  f=0
  while [ "$f" -lt 30 ]; do
    [ "$(dxto systemctl is-active ssh 2> /dev/null || true)" = active ] && break
    f=$((f + 1))
    sleep 1
  done
  [ "$(dxto systemctl is-active ssh 2> /dev/null || true)" = active ] || die "sshd is not active in $TGT_NAME after 30 s (see $h2)"
  {
    echo "---- the second host after the preparation"
    dxt ls -ld /backups /backups/holzcloud
    dxt getent passwd offsite
    dxt rsync --version | head -n 1
    dxt dpkg-query -W openssh-server
  } >> "$h2" 2>&1 || true
  dx bash -c 'grep -qE "^$1 +nas\$" /etc/hosts || echo "$1 nas" >> /etc/hosts' _ "$ip" || die "could not map nas in /etc/hosts of $NAME"
  [ "$(dxo getent hosts nas | awk '{print $1}')" = "$ip" ] || die "nas does not resolve to $ip in $NAME"
  key_line "RSYNC-SOURCE: $(dxo rsync --version | head -n 1)"
  key_line "RSYNC-HOST2: $(dxto rsync --version | head -n 1)"
  key_line "SSH-CLIENT: $(dx ssh -V 2>&1 | tr -d '\r')"
  key_line "SSH-SERVER: $(dxto dpkg-query -W openssh-server)"

  # The one changed line. DEVIATION[remote-target].
  cs=$(dxo sha256sum "$repo_unit" | cut -d' ' -f1)
  hs=$(shasum -a 256 deploy/holzcloud-backup.service | cut -d' ' -f1)
  [ "$cs" = "$hs" ] || die "$repo_unit of $NAME differs from deploy/holzcloud-backup.service"
  if ! dx cmp -s "$repo_unit" "$unitf"; then
    nat_note "the installed unit differed from the repository copy before the line was set; it was put back first (cp $repo_unit $unitf)"
    dx cp "$repo_unit" "$unitf" || die "could not put the unit back"
  fi
  dx grep -Fxq -- '# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud' "$unitf" \
    || die "the exact comment line '# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud' is not in $unitf of $NAME"
  dx sed -i "s|^# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud\$|Environment=REMOTE_TARGET=$want|" "$unitf" || die "could not set the REMOTE_TARGET line"
  dx systemctl daemon-reload || die "daemon-reload failed"
  deviation "DEVIATION[remote-target]: the only change to the shipped service is the line its own comment says to uncomment, on the installed copy $unitf: '# Environment=REMOTE_TARGET=user@nas:/backups/holzcloud' became 'Environment=REMOTE_TARGET=$want'; the repository file is untouched"
  ulines=$(unit_diff_lines "$(dxo cat "$repo_unit")" "$(dxo cat "$unitf")")
  diff <(dxo cat "$repo_unit") <(dxo cat "$unitf") > "$OUT_DIR/offsite-unit-diff.txt" || true
  key_line "UNIT-DIFF-LINES: $ulines"
  while IFS= read -r line; do key_line "UNIT-DIFF $line"; done < "$OUT_DIR/offsite-unit-diff.txt"
  tenv=$(show_v holzcloud-backup.service Environment) || tenv=""
  drop=$(show_v holzcloud-backup.service DropInPaths) || drop="?"
  key_line "REMOTE-TARGET: $(printf '%s\n' "$tenv" | tr ' ' '\n' | sed -n 's/^REMOTE_TARGET=//p')"
  if [ "$ulines" = 1 ]; then
    check_pass "the installed service differs from the repository file in exactly one line"
  else
    check_bad "the installed service differs from the repository file in $ulines lines, expected 1 (offsite-unit-diff.txt)"
  fi
  if [ -z "$drop" ]; then
    check_pass "holzcloud-backup.service has no drop-in"
  else
    check_bad "holzcloud-backup.service has drop-ins: $drop"
  fi

  # Experiment A, the key where an operator would first put it.
  home_dir=$(dxo getent passwd holzcloud | cut -d: -f6)
  nat_note "Experiment A: credentials where an operator first puts them, in the service user's home"
  nat_run "A1 getent passwd holzcloud" dx getent passwd holzcloud
  if dx test -d "$home_dir"; then home_existed=yes; else home_existed=no; fi
  nat_note "the home directory field is $home_dir, it exists before the experiment: $home_existed"
  key_line "SERVICE-USER-HOME: $home_dir (exists by default: $home_existed)"
  nat_run "A1 ssh-keygen -t ed25519 -N '' as holzcloud, stdin closed (the operator's first try)" dx runuser -u holzcloud -- ssh-keygen -t ed25519 -N ''
  dx rm -rf "$home_dir/.ssh"
  if [ "$home_existed" = no ]; then
    nat_note "A2: the operator creates the home directory $home_dir (holzcloud, 750) and $home_dir/.ssh (700)"
    dx mkdir -p "$home_dir" && dx chown holzcloud:holzcloud "$home_dir" && dx chmod 750 "$home_dir" || die "could not create $home_dir"
  fi
  dx install -d -o holzcloud -g holzcloud -m 700 "$home_dir/.ssh" || die "could not create $home_dir/.ssh"
  nat_run "A2 ssh-keygen -f $home_dir/.ssh/id_ed25519 as holzcloud" dx runuser -u holzcloud -- ssh-keygen -q -t ed25519 -N '' -f "$home_dir/.ssh/id_ed25519" -C holzcloud-backup
  [ "$NAT_RC" -eq 0 ] || die "the key could not be generated (see $nat)"
  nat_run "A2 ssh-keyscan -t ed25519 nas into known_hosts (first use, DEVIATION[credentials])" dx runuser -u holzcloud -- bash -c 'ssh-keyscan -t ed25519 nas > "$1"; test -s "$1"' _ "$home_dir/.ssh/known_hosts"
  [ "$NAT_RC" -eq 0 ] || die "ssh-keyscan of nas gave nothing (see $nat)"
  authorize_pub "$home_dir/.ssh/id_ed25519.pub" || die "could not authorize the public key on $TGT_NAME"
  deviation "DEVIATION[credentials]: the document and the unit name no credential. The private key lives only in the source ($home_dir/.ssh/id_ed25519 in experiment A, /opt/holzcloud/offsite-ssh/id_ed25519 in experiment B), only the public key crossed to $TGT_NAME by exec-pipe, and the host key was taken by ssh-keyscan on first use (T-24-08: a real operator compares the fingerprint by hand)"

  # A3, the control outside the sandbox.
  before_a3=$(list_backup_names)
  nat_run "A3 the same backup.sh once as holzcloud OUTSIDE the sandbox, REMOTE_TARGET=$want" dx runuser -u holzcloud -- env DATA_DIR=/opt/holzcloud/data HOLZCLOUD_BIN=/opt/holzcloud/holzcloud RETENTION_DAYS=30 "REMOTE_TARGET=$want" /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
  a3_rc=$NAT_RC
  after_a3=$(list_backup_names)
  a3_dir=$(comm -13 <(printf '%s\n' "$before_a3") <(printf '%s\n' "$after_a3") | tail -n 1)
  if [ -n "$a3_dir" ] && dxt test -d "/backups/holzcloud/$a3_dir"; then a3_on=yes; else a3_on=no; fi
  key_line "A-OUTSIDE-EXIT: $a3_rc"
  key_line "A-OUTSIDE-DIR: ${a3_dir:-none} (on the second host: $a3_on)"

  # A4, inside the shipped sandbox.
  nat_note "A4: the shipped service, ProtectHome=true, ProtectSystem=strict, started by hand (the timer was proven in plan 01)"
  offsite_attempt a "$OUT_DIR/offsite-journal.txt"
  a4_rc=$OFFA_RC
  a4_res=$OFFA_RES
  a4_es=$OFFA_ES
  a4_dir=$OFFA_DIR
  a4_on=$OFFA_ON
  key_line "A-SANDBOX-RESULT: start-rc $a4_rc Result $a4_res ExecMainStatus $a4_es directory ${a4_dir:-none} on the second host: $a4_on"
  if [ "$a4_rc" -eq 0 ] && [ "$a4_res $a4_es" = "success 0" ] && [ "$a4_on" = yes ]; then
    working=home
    fin_dir=$a4_dir
    fin_rc=$a4_rc
    fin_res=$a4_res
    fin_es=$a4_es
    fin_stray_before=$OFFA_STRAY_BEFORE
    fin_stray_after=$OFFA_STRAY_AFTER
    nat_note "A4 succeeded: the key in the service user's home works under the shipped sandbox; experiment B is skipped"
  else
    key_line "FINDING: the key in $home_dir/.ssh does not work under the shipped sandbox: start-rc $a4_rc, Result '$a4_res', ExecMainStatus '$a4_es', directory on the second host: $a4_on (offsite-journal.txt); outside the sandbox the same script ended with status $a3_rc and the directory arrived: $a3_on (offsite-natural.txt, A3)"
    nat_note "A4 failed. Isolating the cause with two transient units, as the service user, with the unit's own properties:"
    nat_run "A5 transient unit WITH ProtectHome=true, ProtectSystem=strict, NoNewPrivileges, PrivateTmp, ReadWritePaths: ls -la $home_dir/.ssh" \
      dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectHome=true -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p "ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups" ls -la "$home_dir/.ssh"
    nat_run "A5 transient unit WITHOUT ProtectHome (the rest as above): ls -la $home_dir/.ssh" \
      dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p "ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups" ls -la "$home_dir/.ssh"
    nat_run "A5 transient unit WITH the shipped sandbox: ssh -o BatchMode=yes offsite@nas true" \
      dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectHome=true -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p "ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups" ssh -o BatchMode=yes offsite@nas true
    nat_run "A5 transient unit WITHOUT ProtectHome: ssh -o BatchMode=yes offsite@nas true" \
      dx systemd-run --wait --pipe --quiet -p User=holzcloud -p Group=holzcloud -p ProtectSystem=strict -p NoNewPrivileges=true -p PrivateTmp=true -p "ReadWritePaths=/opt/holzcloud/data /opt/holzcloud/backups" ssh -o BatchMode=yes offsite@nas true

    # Experiment B, key and known_hosts outside /home, named by a system ssh config.
    nat_note "Experiment B: key and known_hosts under /opt/holzcloud/offsite-ssh, named in /etc/ssh/ssh_config.d/holzcloud-backup.conf; the unit keeps its one changed line"
    dx rm -rf /opt/holzcloud/offsite-ssh
    dx install -d -o holzcloud -g holzcloud -m 700 /opt/holzcloud/offsite-ssh || die "could not create /opt/holzcloud/offsite-ssh"
    nat_run "B1 ssh-keygen -f /opt/holzcloud/offsite-ssh/id_ed25519 as holzcloud" dx runuser -u holzcloud -- ssh-keygen -q -t ed25519 -N '' -f /opt/holzcloud/offsite-ssh/id_ed25519 -C holzcloud-backup
    [ "$NAT_RC" -eq 0 ] || die "the key of experiment B could not be generated (see $nat)"
    nat_run "B1 ssh-keyscan -t ed25519 nas into /opt/holzcloud/offsite-ssh/known_hosts" dx runuser -u holzcloud -- bash -c 'ssh-keyscan -t ed25519 nas > "$1"; test -s "$1"' _ /opt/holzcloud/offsite-ssh/known_hosts
    [ "$NAT_RC" -eq 0 ] || die "ssh-keyscan of nas gave nothing (see $nat)"
    authorize_pub /opt/holzcloud/offsite-ssh/id_ed25519.pub || die "could not authorize the key of experiment B on $TGT_NAME"
    printf 'Host nas\n    User offsite\n    IdentityFile /opt/holzcloud/offsite-ssh/id_ed25519\n    UserKnownHostsFile /opt/holzcloud/offsite-ssh/known_hosts\n    IdentitiesOnly yes\n    StrictHostKeyChecking yes\n    BatchMode yes\n' \
      | dxi bash -c 'mkdir -p /etc/ssh/ssh_config.d; cat > /etc/ssh/ssh_config.d/holzcloud-backup.conf; chmod 644 /etc/ssh/ssh_config.d/holzcloud-backup.conf' || die "could not write the ssh config"
    nat_run "B2 /etc/ssh/ssh_config.d/holzcloud-backup.conf and the directory" dx bash -c 'cat /etc/ssh/ssh_config.d/holzcloud-backup.conf; ls -la /opt/holzcloud/offsite-ssh'
    offsite_attempt b "$OUT_DIR/offsite-journal-b.txt"
    b_rc=$OFFA_RC
    b_res=$OFFA_RES
    b_es=$OFFA_ES
    b_dir=$OFFA_DIR
    b_on=$OFFA_ON
    key_line "B-RESULT: start-rc $b_rc Result $b_res ExecMainStatus $b_es directory ${b_dir:-none} on the second host: $b_on"
    if [ "$b_rc" -eq 0 ] && [ "$b_res $b_es" = "success 0" ] && [ "$b_on" = yes ]; then
      working=offsite-ssh
      fin_dir=$b_dir
      fin_rc=$b_rc
      fin_res=$b_res
      fin_es=$b_es
      fin_stray_before=$OFFA_STRAY_BEFORE
      fin_stray_after=$OFFA_STRAY_AFTER
    else
      key_line "FINDING: experiment B (key and known_hosts under /opt/holzcloud/offsite-ssh, a system ssh config) did not give a copy either: start-rc $b_rc, Result '$b_res', ExecMainStatus '$b_es' (offsite-journal-b.txt)"
      fin_dir=$b_dir
      fin_rc=$b_rc
      fin_res=$b_res
      fin_es=$b_es
      fin_stray_before=$OFFA_STRAY_BEFORE
      fin_stray_after=$OFFA_STRAY_AFTER
    fi
  fi

  # Judgement.
  key_line "WORKING-CREDENTIALS: $working"
  key_line "RESULT: ${fin_res:-none}"
  key_line "EXIT-STATUS: ${fin_es:-none}"
  key_line "COPY-DIR: ${fin_dir:+/backups/holzcloud/$fin_dir}"
  if [ "$working" = none ]; then
    check_bad "no configuration produced a copy on the second host (see offsite-natural.txt and the journals)"
  else
    check_pass "credentials '$working': the shipped service copied $fin_dir to $TGT_NAME (Result $fin_res, ExecMainStatus $fin_es)"
  fi
  if [ "$fin_rc" -eq 0 ] && [ "$fin_res $fin_es" = "success 0" ]; then
    check_pass "the final run of the service: Result success, ExecMainStatus 0"
  else
    check_bad "the final run of the service: start-rc $fin_rc, Result '$fin_res', ExecMainStatus '$fin_es'"
  fi

  sl=$(sha_list_in "$NAME" /opt/holzcloud/backups) || sl=""
  tl=$(sha_list_in "$TGT_NAME" /backups/holzcloud) || tl=""
  printf '%s\n' "$sl" > "$OUT_DIR/offsite-sha-source.txt"
  printf '%s\n' "$tl" > "$TGT_OUT_DIR/offsite-sha-host2.txt"
  nfiles=$(printf '%s\n' "$sl" | grep -c . || true)
  key_line "FILES-COMPARED: $nfiles"
  if list_equal "$sl" "$tl"; then
    check_pass "the sha256 of every file under /opt/holzcloud/backups of the source ($nfiles files) equals that of /backups/holzcloud on the second host"
  else
    check_bad "the sha256 lists of /opt/holzcloud/backups (source, $nfiles files) and /backups/holzcloud (second host, $(printf '%s\n' "$tl" | grep -c . || true) files) differ or are empty (offsite-sha-source.txt, offsite-sha-host2.txt)"
  fi
  dirs_src=$(list_backup_names)
  dirs_h2=$(dxto find /backups/holzcloud -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort)
  if [ -n "$dirs_src" ] && [ "$dirs_src" = "$dirs_h2" ]; then
    check_pass "the same $(printf '%s\n' "$dirs_src" | grep -c .) backup directories on both hosts"
  else
    check_bad "the backup directories differ: source '$(printf '%s' "$dirs_src" | tr '\n' ' ')', second host '$(printf '%s' "$dirs_h2" | tr '\n' ' ')'"
  fi
  bad_modes=$(dxto find /backups/holzcloud -mindepth 1 -maxdepth 1 -type d -exec stat -c '%n %U %a' {} + | grep -v ' offsite 700$' || true)
  dxto find /backups/holzcloud -mindepth 1 -maxdepth 1 -type d -exec stat -c '%n %U %a' {} + > "$TGT_OUT_DIR/offsite-modes.txt" || true
  if [ -z "$bad_modes" ] && [ -s "$TGT_OUT_DIR/offsite-modes.txt" ]; then
    check_pass "every copied directory on the second host is owned by offsite with mode 700 (offsite-modes.txt)"
  else
    check_bad "copied directories that are not 'offsite 700' (they hold password hashes and the CSRF key): ${bad_modes:-none listed}"
  fi
  if [ "$fin_stray_before" = yes ] && [ "$fin_stray_after" = no ]; then
    check_info "MIRROR: a stray directory planted on the second host before the final run is gone after it: rsync --delete makes the off-site copy a mirror of the backup root, so what the source prunes or loses is lost off-site too"
  else
    check_info "MIRROR: the stray directory on the second host was there before the final run: $fin_stray_before, and after it: $fin_stray_after"
  fi
  check_info "NOT PLAYED: the pull by rsync -a that the document describes for a rebuilt server; offsite-restore restores on the second host itself"

  leak_ssh "$home_dir"
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "OFFSITE FAILED"
    say "nightly.sh: OFFSITE FAILED, $BADS BAD line(s); see $o, $nat"
    exit 1
  fi
  key_line "OFFSITE OK"
}

# ---------------------------------------------------------------------------
# offsite-restore: BKP-04, the restore from the copy the unit made. The document's
# own restore line is played on the second host against /backups/holzcloud/TS, the
# directory rsync put there, and not against a docker cp copy.
# ---------------------------------------------------------------------------

cmd_offsite_restore() {
  local o=$OUT_DIR/offsite-restore.txt r=$TGT_OUT_DIR/restore.txt rr=$TGT_OUT_DIR/restore-run.txt tf=$TGT_OUT_DIR/transfer.txt
  local copy copy_name f sl tl nfiles rc=0 pre post fx_ok=yes restored mline cline

  need_container
  dxt true > /dev/null 2>&1 \
    || die "the second host $TGT_NAME does not answer; bring it up with: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/nightly.sh target"
  [ "$NAME" != "$TGT_NAME" ] || die "TGT_NAME equals NAME ($NAME)"
  [ "$(tail -n 1 "$OUT_DIR/offsite.txt" 2> /dev/null)" = "OFFSITE OK" ] \
    || die "$OUT_DIR/offsite.txt does not end OFFSITE OK; run: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/nightly.sh offsite"
  copy=$(kv "$OUT_DIR/offsite.txt" COPY-DIR)
  case "$copy" in
    /backups/holzcloud/?*) ;;
    *) die "$OUT_DIR/offsite.txt names no COPY-DIR below /backups/holzcloud (got '$copy')" ;;
  esac
  for f in tree-src-backup.txt tree-backup.txt backup.txt state-src.txt baseline.txt; do
    [ -f "$OUT_DIR/$f" ] \
      || die "$OUT_DIR/$f is missing; run: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/run.sh backup"
  done
  dxt test -x /opt/holzcloud/holzcloud \
    || die "no runnable /opt/holzcloud/holzcloud in $TGT_NAME; bring the second host up with: PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR bash deploy/test/nightly.sh target"
  dxt test -f "$copy/holzcloud.sqlite" \
    || die "$TGT_NAME has no $copy/holzcloud.sqlite; run 'nightly.sh offsite' again"
  mkdir -p "$OUT_DIR" "$TGT_OUT_DIR"
  copy_name=${copy##*/}

  BADS=0
  res_begin "$o"
  key_line "RESTORED-FROM: $copy"

  # (1) Independence: nothing in the second host's own backup root.
  pre=$(dxto ls -A /opt/holzcloud/backups) || pre="?"
  key_line "BACKUPS-OF-HOST2-BEFORE: ${pre:-empty}"
  if [ -z "$pre" ]; then
    check_pass "/opt/holzcloud/backups of $TGT_NAME is empty before the restore: the restore can only come from $copy"
  else
    check_bad "/opt/holzcloud/backups of $TGT_NAME is not empty before the restore ($pre): the copy of the unit would not be the only source"
  fi

  # (2) The fixtures, by exec-pipe, as run.sh transfer_into does it.
  deviation "DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they crossed from container to container by exec-pipe, never through a host file, exactly as run.sh transfer does it; the ORDER line of the source's baseline.txt went to $TGT_OUT_DIR/baseline.txt"
  for f in /root/.hc-login-pass /root/.hc-key; do
    if ! dx cat "$f" | dxti bash -c 'umask 077; cat > "$1"; test -s "$1"' _ "$f"; then
      check_bad "$f could not be carried from $NAME to $TGT_NAME"
      fx_ok=no
    fi
  done
  if ! dx cat /tmp/hc-upload.jpg | dxti bash -c 'umask 022; cat > "$1"; test -s "$1"' _ /tmp/hc-upload.jpg; then
    check_bad "/tmp/hc-upload.jpg could not be carried from $NAME to $TGT_NAME"
    fx_ok=no
  fi
  if ! grep -m1 '^ORDER:' "$OUT_DIR/baseline.txt" > "$TGT_OUT_DIR/baseline.txt"; then
    check_bad "$OUT_DIR/baseline.txt has no ORDER: line"
    fx_ok=no
  fi

  # (3) The copy against the source, and the one file run.sh restore reads.
  sl=$(sha_list_in "$NAME" "/opt/holzcloud/backups/$copy_name") || sl=""
  tl=$(sha_list_in "$TGT_NAME" "$copy") || tl=""
  nfiles=$(printf '%s\n' "$sl" | grep -c . || true)
  {
    echo "DIR: $copy"
    echo "SOURCE: copy made by holzcloud-backup.service through REMOTE_TARGET"
    echo "FILES: $nfiles"
    echo "VIA: rsync by the shipped unit, no docker cp of the backup"
  } > "$tf"
  if list_equal "$sl" "$tl"; then
    echo "TRANSFER: equal" >> "$tf"
    check_pass "the sha256 of every file of $copy on $TGT_NAME ($nfiles files) equals that of /opt/holzcloud/backups/$copy_name on the source"
  else
    check_bad "the sha256 lists of $copy ($TGT_NAME) and /opt/holzcloud/backups/$copy_name ($NAME) differ or are empty"
  fi

  # (4) run.sh restore, unchanged.
  if [ "$BADS" -eq 0 ] && [ "$fx_ok" = yes ]; then
    PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR KEEP=1 \
      bash deploy/test/run.sh restore > "$OUT_DIR/offsite-restore-run.log" 2>&1 || rc=$?
    key_line "RUN-SH-RESTORE-EXIT: $rc"
    cp "$rr" "$OUT_DIR/offsite-restore-run.txt" 2> /dev/null || true

    # (5) The judgement, from restore.txt of the second host.
    if [ "$rc" -eq 0 ]; then
      check_pass "run.sh restore ended with status 0"
    else
      check_bad "run.sh restore ended with status $rc (offsite-restore-run.log, $r)"
    fi
    if [ -f "$r" ] && restore_lines_ok "$(cat "$r")"; then
      check_pass "restore.txt: login 303 to /admin/, setup form 200 before and 404 after, media served 200, marker page, check 17 of 17, and no BAD line"
    else
      check_bad "restore.txt does not hold the lines of a restore that worked (see $r)"
    fi
    if [ -f "$rr" ] && grep -qF "=== Restoring from $copy ===" "$rr"; then
      check_pass "restore-run.txt holds '=== Restoring from $copy ===': the document's restore line was played against the copy of the unit"
    else
      check_bad "restore-run.txt holds no '=== Restoring from $copy ===' line"
    fi
    restored=$(kv "$r" DIR)
    if [ "$restored" = "$copy" ]; then
      check_pass "the DIR line of restore.txt equals the copy path ($restored)"
    else
      check_bad "the DIR line of restore.txt is '$restored', expected '$copy'"
    fi
    post=$(dxto ls -A /opt/holzcloud/backups) || post="?"
    if [ -z "$post" ]; then
      check_pass "/opt/holzcloud/backups of $TGT_NAME is still empty after the restore"
    else
      check_bad "/opt/holzcloud/backups of $TGT_NAME holds '$post' after the restore"
    fi

    # (6) The key lines.
    mline=$(grep -m1 '^PASS marker page' "$r" 2> /dev/null || true)
    cline=$(grep -m1 '^PASS check: 17 of 17' "$r" 2> /dev/null || true)
    key_line "SETUP-BEFORE: $(kv "$r" SETUP-BEFORE)"
    key_line "LOGIN: $(kv "$r" LOGIN)"
    key_line "SETUP-AFTER: $(kv "$r" SETUP-AFTER)"
    key_line "MEDIA-SERVED: $(kv "$r" MEDIA-SERVED)"
    key_line "MARKER-PAGE: ${mline#PASS }"
    key_line "CHECK: ${cline#PASS }"
    key_line "RESTORE-WALL: $(kv "$r" WALL-S)"
    { grep -E '^(PASS|BAD)' "$r" 2> /dev/null || true; } | sed 's/^/RESTORE-TXT /' | tee -a "$o" > /dev/null
  else
    check_bad "run.sh restore was not played: the independence, the fixtures or the copy comparison failed above"
  fi
  check_info "NOT PLAYED: the pull by rsync -a that the document describes for a rebuilt server; the restore ran on the second host itself, from the copy path"

  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "OFFSITE-RESTORE FAILED"
    say "nightly.sh: OFFSITE-RESTORE FAILED, $BADS BAD line(s); see $o, $r"
    exit 1
  fi
  key_line "OFFSITE-RESTORE OK"
}

# ---------------------------------------------------------------------------
# down, chain and record-check
# ---------------------------------------------------------------------------

# down: remove the source and the second host. Safe when either is absent.
cmd_down() {
  local c
  for c in "$NAME" "$TGT_NAME"; do
    if docker container inspect "$c" > /dev/null 2>&1; then
      docker rm -f "$c" > /dev/null 2>&1 || say "down: could not remove $c"
    fi
  done
  say "down: $NAME and $TGT_NAME are gone"
}

CHAIN_KEEP=0
chain_cleanup() {
  [ "$CHAIN_KEEP" != 1 ] || return 0
  cmd_down || true
}

# chain_stage SUB: one stage as a child process of this script, so that `set -e`
# stays in force inside it (a function called from the left of `||` runs with `-e`
# off, and a stage that goes on after a failed command would hide the failure).
# The child inherits the environment that `chain` exported.
chain_stage() {
  say "chain: ---- stage $1 started $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  bash "$ROOT/deploy/test/nightly.sh" "$1"
}

# chain: the whole proof of phase 24 from nothing but the image, in one command.
# Every stage's status is kept, so one failure hides nothing; the verdict of each
# requirement comes from the files the stages wrote.
cmd_chain() {
  local sc_fire=0 sc_delay=0 sc_catchup=0 sc_prune=0 sc_backup=0 sc_offsite=0 sc_restore=0
  local ref d v1 v2 v3 v4 v4b v5 sp=0 sf=0 bp kinds leak s fails=0 sum=$OUT_DIR/summary.txt

  [ "$TARGET" = systemd-base ] \
    || die "chain needs TARGET=systemd-base (got '$TARGET'): the proof starts from a bare systemd image, nothing else"
  for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
    case "$d" in
      / | . | .. | ./ | "$ROOT" | "$ROOT"/) die "chain: refusing the evidence directory '$d'" ;;
    esac
  done
  [ "$NAME" != "$TGT_NAME" ] || die "chain: TGT_NAME equals NAME ($NAME)"
  [ "$OUT_DIR" != "$TGT_OUT_DIR" ] || die "chain: TGT_OUT_DIR equals OUT_DIR ($OUT_DIR)"
  for d in "$NAME" "$TGT_NAME"; do
    if docker container inspect "$d" > /dev/null 2>&1; then
      die "chain: the container $d exists already; the chain starts from fresh containers, remove it with: NAME=$NAME TGT_NAME=$TGT_NAME bash deploy/test/nightly.sh down"
    fi
  done
  # The binary of both containers is the same, even if a commit lands mid-run.
  ref=$(pin_ref)
  NEW_REF=$ref
  CHAIN_KEEP=$KEEP
  trap chain_cleanup EXIT
  # The children never remove a container; the trap above does, unless KEEP=1 was given.
  export PLATFORM TARGET NAME OUT_DIR TGT_NAME TGT_OUT_DIR NEW_REF ZONE_LEAD FIRE_WAIT DELAY_SAMPLES CATCHUP_LEAD CATCHUP_WAIT
  export KEEP=1
  say "chain: NEW_REF pinned to $ref; source $NAME ($OUT_DIR), second host $TGT_NAME ($TGT_OUT_DIR), started $(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # Without these two there is nothing to measure.
  chain_stage source || die "chain: the source could not be built (status $?)"
  chain_stage target || die "chain: the second host could not be built (status $?)"

  chain_stage fire || sc_fire=$?
  chain_stage delay || sc_delay=$?
  chain_stage catchup || sc_catchup=$?
  chain_stage prune || sc_prune=$?
  say "chain: ---- stage run.sh backup started $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  bash "$ROOT/deploy/test/run.sh" backup || sc_backup=$?
  chain_stage offsite || sc_offsite=$?
  chain_stage offsite-restore || sc_restore=$?

  # The verdicts, from the files the stages wrote.
  v1=$(req_verdict "$sc_fire" "$OUT_DIR/fire.txt" "FIRE OK" "$OUT_DIR/verify-fire.txt")
  v2=$(req_verdict "$sc_catchup" "$OUT_DIR/catchup.txt" "CATCHUP OK" "$OUT_DIR/verify-catchup.txt")
  v3=$(req_verdict "$sc_delay" "$OUT_DIR/delay.txt" "DELAY OK")
  v4=$(req_verdict "$sc_offsite" "$OUT_DIR/offsite.txt" "OFFSITE OK")
  v4b=$(req_verdict "$sc_restore" "$OUT_DIR/offsite-restore.txt" "OFFSITE-RESTORE OK")
  [ "$v4" != PASS ] || v4=$v4b
  v5=$(req_verdict "$sc_prune" "$OUT_DIR/prune.txt" "PRUNE OK")

  # No secret of either container in either directory.
  if ( leak_check ); then leak=OK; else leak=FAILED; fi

  for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
    bp=$(steps_of "$d")
    sp=$((sp + ${bp% *}))
    sf=$((sf + ${bp#* }))
  done
  kinds=$(for d in "$OUT_DIR" "$TGT_OUT_DIR"; do
    sed -n 's/^DEVIATION\[\([a-z-]*\)\].*/\1/p' "$d/deviations.txt" 2> /dev/null
  done | sort -u | tr '\n' ' ' | sed 's/ $//')

  {
    echo "---- summary ($PLATFORM) ----"
    echo "BKP-01 $v1"
    echo "BKP-02 $v2"
    echo "BKP-03 $v3"
    echo "BKP-04 $v4"
    echo "BKP-05 $v5"
    echo "TIMER: TRIGGERED-BY=$(kvq "$OUT_DIR/fire.txt" TRIGGERED-BY) RESULT=$(kvq "$OUT_DIR/fire.txt" RESULT) START-LOCAL=$(kvq "$OUT_DIR/fire.txt" START-LOCAL)"
    echo "DELAY: USED-S=$(kvq "$OUT_DIR/delay.txt" DELAY-USED-S) SAMPLES=$(kvq "$OUT_DIR/delay.txt" DELAY-SAMPLES) MIN-S=$(kvq "$OUT_DIR/delay.txt" D-MIN) MAX-S=$(kvq "$OUT_DIR/delay.txt" D-MAX)"
    echo "CATCHUP: TRIGGERED-BY=$(kvq "$OUT_DIR/catchup.txt" TRIGGERED-BY) AFTER-BOOT-S=$(kvq "$OUT_DIR/catchup.txt" DELAY-AFTER-BOOT-S) CONTROL-RAN=$(kvq "$OUT_DIR/catchup.txt" CONTROL-RAN)"
    echo "PRUNE: PLANTED=$(grep -c '^PLANTED ' "$OUT_DIR/prune.txt" 2> /dev/null || true) DELETED=$(grep -c '^OBSERVED .* gone$' "$OUT_DIR/prune.txt" 2> /dev/null || true) KEPT=$(grep -c '^OBSERVED .* present$' "$OUT_DIR/prune.txt" 2> /dev/null || true)"
    echo "OFFSITE: WORKING-CREDENTIALS=$(kvq "$OUT_DIR/offsite.txt" WORKING-CREDENTIALS) FILES-COMPARED=$(kvq "$OUT_DIR/offsite.txt" FILES-COMPARED) UNIT-DIFF-LINES=$(kvq "$OUT_DIR/offsite.txt" UNIT-DIFF-LINES)"
    echo "RESTORE: LOGIN=$(kvq "$OUT_DIR/offsite-restore.txt" LOGIN) MEDIA-SERVED=$(kvq "$OUT_DIR/offsite-restore.txt" MEDIA-SERVED) WALL-S=$(kvq "$OUT_DIR/offsite-restore.txt" RESTORE-WALL)"
    echo "STEPS: $sp played, $sf failed"
    echo "DEVIATIONS: ${kinds:-none}"
    if [ "$leak" = OK ]; then echo "LEAK-CHECK OK"; else echo "LEAK-CHECK FAILED"; fi
  } > "$sum"
  cat "$sum"
  say "chain: stage statuses (1 is a BAD line or a refusal, 3 is a platform that could not give the evidence): fire=$sc_fire delay=$sc_delay catchup=$sc_catchup prune=$sc_prune run.sh-backup=$sc_backup offsite=$sc_offsite offsite-restore=$sc_restore"
  say "chain: finished $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  for s in "$v1" "$v2" "$v3" "$v4" "$v5"; do
    [ "$s" = PASS ] || fails=$((fails + 1))
  done
  [ "$leak" = OK ] || fails=$((fails + 1))
  if [ "$fails" -ne 0 ]; then
    say "chain: $fails requirement(s) or checks did not pass, see $sum"
    exit 1
  fi
}

# record-check: the proof of deploy/BACKUP-TIMER-RUN.md against the scratch files.
cmd_record_check() {
  local r=${R:-deploy/BACKUP-TIMER-RUN.md} e=${E:-deploy/test/out}
  record_check "$r" "$e" "$(basename "$OUT_DIR") $(basename "$TGT_OUT_DIR")" || exit 1
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    selftest | source | target | fire | delay | catchup | prune | offsite | chain | down) "cmd_$sub" ;;
    offsite-restore) cmd_offsite_restore ;;
    record-check) cmd_record_check ;;
    *) usage ;;
  esac
done
