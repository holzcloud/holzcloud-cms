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
#   nightly.sh prep         phase 25: the editor account the churn runs as, and the admin's two-factor wall in front of the page form
#   nightly.sh consist      CON-01: backup.sh under page saves, uploads and deletes, restored by restore.sh and compared in both directions
#   nightly.sh window       CON-01: a forced window (the shipped backup.sh paused between its two copies), three controls
#   nightly.sh compat-source   phase 25: CON-02, the previous release (COMPAT_TAG) built from its tag, installed from the document, populated
#   nightly.sh compat       CON-02: its own backup.sh makes a backup, this release's restore.sh restores it on a fresh box, the data is compared
#   nightly.sh capfull      CON-03: a filesystem nearly full but above MIN_FREE_MB under the backup root; by hand, by the shipped unit, the next night
#   nightly.sh down-con     remove the two containers of compat-source and compat (OLD_NAME and NEW_NAME), and only those
#   nightly.sh con-chain    phase 25, plan 03: the whole evidence set (CON-01, CON-02, CON-03) from nothing but the image, one command; ends in OUT_DIR/summary.txt
#   nightly.sh record-check-con   the proof of deploy/BACKUP-CONSISTENCY-RUN.md against the scratch files (R and E), and its Owner steps section
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
#   Phase 25 (CON-01); prep, consist and window default NAME, OUT_DIR, TGT_NAME and TGT_OUT_DIR to
#   holzcloud-t25-src, deploy/test/out/amd64-t25-src, holzcloud-t25-tgt, deploy/test/out/amd64-t25-tgt
#   when the caller gave none of them (any other call keeps the t24 names above):
#   BULK_MB        megabytes of synthetic media `source` writes through run.sh backup-fixture (0; the plan's runs use 100)
#   CONSIST_RUNS   40 runs per stage (stage adds: page saves and uploads; stage deletes: the same plus deletes)
#   CONSIST_MIN_RUNS   30, the floor under which no stage verdict is ok (a trace prints MIN-RUNS: 1)
#   CONSIST_STAGES     "adds deletes"
#   CONSIST_RESTORE_EVERY   1, every run is restored; N restores runs 1, N+1, ...; the others are compared on the backup itself
#   CHURN_WORKERS  2 uploaders, one page saver, in stage deletes one deleter
#   CHURN_PAUSE    0.05 seconds each worker waits between two requests
#   Phase 25, plan 02 (CON-02 and CON-03), all optional:
#   COMPAT_TAG     v0.1.0, the previous release; compat-source builds it from this tag
#   OLD_NAME, OLD_OUT_DIR   holzcloud-t25-old, deploy/test/out/amd64-t25-old (the previous release, its evidence and compat.txt)
#   NEW_NAME, NEW_OUT_DIR   holzcloud-t25-new, deploy/test/out/amd64-t25-new (the fresh box with this release)
#   CAPFULL_FLOOR_MB   1024, the shipped MIN_FREE_MB (never passed to backup.sh unless it is lowered, and then DEVIATION[floor])
#   CAPFULL_MARGIN_MB  40, free megabytes of the nearly full filesystem above the floor
#   CAPFULL_DATA_MB    1150, the bulk media backup-fixture writes so that the data directory is larger than that free space
#   capfull defaults NAME and OUT_DIR to the t25 source names like prep, consist and window when the caller gave none
#   Phase 25, plan 03: con-chain and record-check-con take the t25 names of all four containers and evidence directories
#   RC_REQ_PREFIX, RC_REQ_IDS   BKP-0 and "1 2 3 4 5", the requirement headings record_check asks for (record-check-con sets CON-0 and "1 2 3")
#
# Written for bash 3.2 (macOS): no arrays, no mapfile, no case conversion.
# Anything that needs GNU tools (date -d, stat -c, sha256sum, zic) runs inside
# the container through docker exec.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

# Phase 25: whether the caller gave NAME, OUT_DIR, TGT_NAME and TGT_OUT_DIR is
# recorded here, before any default below is applied. A call that names a phase 25
# subcommand (prep, consist, window) and gave none of them gets the t25 containers
# and evidence directories; every other call keeps the t24 defaults, unchanged.
GIVEN_NAME=no
GIVEN_OUT_DIR=no
GIVEN_TGT_NAME=no
GIVEN_TGT_OUT_DIR=no
[ -z "${NAME:-}" ] || GIVEN_NAME=yes
[ -z "${OUT_DIR:-}" ] || GIVEN_OUT_DIR=yes
[ -z "${TGT_NAME:-}" ] || GIVEN_TGT_NAME=yes
[ -z "${TGT_OUT_DIR:-}" ] || GIVEN_TGT_OUT_DIR=yes
P25_CALL=no
for p25_arg in "$@"; do
  case "$p25_arg" in prep | consist | window) P25_CALL=yes ;; esac
done
if [ "$P25_CALL" = yes ]; then
  [ "$GIVEN_NAME" = yes ] || NAME=holzcloud-t25-src
  [ "$GIVEN_OUT_DIR" = yes ] || OUT_DIR=deploy/test/out/amd64-t25-src
  [ "$GIVEN_TGT_NAME" = yes ] || TGT_NAME=holzcloud-t25-tgt
  [ "$GIVEN_TGT_OUT_DIR" = yes ] || TGT_OUT_DIR=deploy/test/out/amd64-t25-tgt
fi
# Plan 02: capfull works on the source container alone and takes the same t25 names.
for p25_arg in "$@"; do
  if [ "$p25_arg" = capfull ]; then
    [ "$GIVEN_NAME" = yes ] || NAME=holzcloud-t25-src
    [ "$GIVEN_OUT_DIR" = yes ] || OUT_DIR=deploy/test/out/amd64-t25-src
  fi
done
# Plan 03: con-chain and record-check-con work on all four t25 containers and directories.
for p25_arg in "$@"; do
  if [ "$p25_arg" = con-chain ] || [ "$p25_arg" = record-check-con ]; then
    [ "$GIVEN_NAME" = yes ] || NAME=holzcloud-t25-src
    [ "$GIVEN_OUT_DIR" = yes ] || OUT_DIR=deploy/test/out/amd64-t25-src
    [ "$GIVEN_TGT_NAME" = yes ] || TGT_NAME=holzcloud-t25-tgt
    [ "$GIVEN_TGT_OUT_DIR" = yes ] || TGT_OUT_DIR=deploy/test/out/amd64-t25-tgt
  fi
done

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

# Phase 25 (CON-01), all optional; see the header.
CONSIST_RUNS=${CONSIST_RUNS:-40}
CONSIST_MIN_RUNS=${CONSIST_MIN_RUNS:-30}
CONSIST_STAGES=${CONSIST_STAGES:-adds deletes}
CONSIST_RESTORE_EVERY=${CONSIST_RESTORE_EVERY:-1}
CHURN_WORKERS=${CHURN_WORKERS:-4}
CHURN_PAUSE=${CHURN_PAUSE:-0.05}

# Phase 25, plan 02 (CON-02, CON-03), all optional; see the header.
COMPAT_TAG=${COMPAT_TAG:-v0.1.0}
OLD_NAME=${OLD_NAME:-holzcloud-t25-old}
OLD_OUT_DIR=${OLD_OUT_DIR:-deploy/test/out/amd64-t25-old}
NEW_NAME=${NEW_NAME:-holzcloud-t25-new}
NEW_OUT_DIR=${NEW_OUT_DIR:-deploy/test/out/amd64-t25-new}
CAPFULL_FLOOR_MB=${CAPFULL_FLOOR_MB:-1024}
CAPFULL_MARGIN_MB=${CAPFULL_MARGIN_MB:-40}
CAPFULL_DATA_MB=${CAPFULL_DATA_MB:-1150}

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
  for i in ${RC_REQ_IDS:-1 2 3 4 5}; do
    if ! grep -q "^## ${RC_REQ_PREFIX:-BKP-0}$i" "$r"; then
      echo "RECORD-CHECK BAD: no heading beginning '## ${RC_REQ_PREFIX:-BKP-0}$i'"
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

# ROB-02: capfull_robust_verdict CLASS DIR_MADE REMOVAL_LINE SENTINEL_INTACT and
# capfull_next_night_verdict GUARD CLASS. One case for the pass, one for each way a
# property can break, one for each kind of word that is refused.
selftest_capfull_robust() {
  local d=20261027-031500
  st_eq "robust: a clean removal passes" PASS "$(capfull_robust_verdict FAILED-NO-DIR none yes yes)"
  st_eq "robust: a directory left is named" "FAIL the night left a directory" "$(capfull_robust_verdict FAILED-PARTIAL-DIR-LEFT "$d" no yes)"
  st_eq "robust: a directory left, even with the removal line" "FAIL the night left a directory" "$(capfull_robust_verdict FAILED-PARTIAL-DIR-LEFT "$d" yes yes)"
  st_eq "robust: a night that completed" "FAIL the night completed where it should have failed" "$(capfull_robust_verdict COMPLETE "$d" no yes)"
  st_eq "robust: a night that completed falsely" "FAIL the night completed where it should have failed" "$(capfull_robust_verdict FALSE-COMPLETE "$d" no yes)"
  st_eq "robust: a guard that refused" "FAIL the guard refused, so the night under test never ran" "$(capfull_robust_verdict GUARD-REFUSED none no yes)"
  st_eq "robust: no class but a directory still named" "FAIL the night made the directory $d and it is still there" "$(capfull_robust_verdict FAILED-NO-DIR "$d" yes yes)"
  st_eq "robust: no removal line" "FAIL the transcript has no line naming the removal of the directory" "$(capfull_robust_verdict FAILED-NO-DIR none no yes)"
  st_eq "robust: the sentinel was touched" "FAIL the backup of the earlier night was touched" "$(capfull_robust_verdict FAILED-NO-DIR none yes no)"
  st_eq "robust: no sentinel was planted" "FAIL no backup of an earlier night was planted, so nothing says it was left alone" "$(capfull_robust_verdict FAILED-NO-DIR none yes none-planted)"
  st_eq "robust: the first broken property wins, the directory before the line" "FAIL the night made the directory $d and it is still there" "$(capfull_robust_verdict FAILED-NO-DIR "$d" no no)"
  st_eq "robust: the removal line before the sentinel" "FAIL the transcript has no line naming the removal of the directory" "$(capfull_robust_verdict FAILED-NO-DIR none no no)"
  st_fail "robust: an unknown class" capfull_robust_verdict UNCLASSIFIED none yes yes
  st_fail "robust: an empty class" capfull_robust_verdict "" none yes yes
  st_fail "robust: an empty directory name" capfull_robust_verdict FAILED-NO-DIR "" yes yes
  st_fail "robust: a directory name with a space" capfull_robust_verdict FAILED-NO-DIR "a b" yes yes
  st_fail "robust: a removal answer that is not yes or no" capfull_robust_verdict FAILED-NO-DIR none maybe yes
  st_fail "robust: a sentinel answer that is not a word" capfull_robust_verdict FAILED-NO-DIR none yes maybe
  st_fail "robust: too few arguments" capfull_robust_verdict FAILED-NO-DIR none yes
  st_fail "robust: too many arguments" capfull_robust_verdict FAILED-NO-DIR none yes yes extra
  st_eq "robust: a refusal prints nothing" "" "$(capfull_robust_verdict UNCLASSIFIED none yes yes || true)"
  st_eq "next night: passed and failed without a directory" PASS "$(capfull_next_night_verdict passed FAILED-NO-DIR)"
  st_eq "next night: the guard refused" "FAIL the guard refused the next night: something still holds the space" "$(capfull_next_night_verdict refused GUARD-REFUSED)"
  st_eq "next night: refused with any class is still a refusal" "FAIL the guard refused the next night: something still holds the space" "$(capfull_next_night_verdict refused FAILED-NO-DIR)"
  st_eq "next night: passed and left a directory" "FAIL the next night passed the guard and left a directory" "$(capfull_next_night_verdict passed FAILED-PARTIAL-DIR-LEFT)"
  st_eq "next night: passed and completed" "FAIL the next night completed where it should have failed" "$(capfull_next_night_verdict passed COMPLETE)"
  st_eq "next night: passed and completed falsely" "FAIL the next night completed where it should have failed" "$(capfull_next_night_verdict passed FALSE-COMPLETE)"
  st_eq "next night: passed but classed as refused" "FAIL the next night was classed as refused although the guard passed" "$(capfull_next_night_verdict passed GUARD-REFUSED)"
  st_fail "next night: an unknown guard" capfull_next_night_verdict maybe FAILED-NO-DIR
  st_fail "next night: an unknown class" capfull_next_night_verdict passed UNCLASSIFIED
  st_fail "next night: too few arguments" capfull_next_night_verdict passed
  st_eq "next night: a refusal prints nothing" "" "$(capfull_next_night_verdict passed UNCLASSIFIED || true)"
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
  selftest_consist
  selftest_outcome
  selftest_hold
  selftest_compat
  selftest_con
  selftest_capfull_robust
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


# st_lists DIR ROWS FILES: DIR/rows and DIR/files from two "\n" separated texts.
st_lists() {
  printf '%s\n' "$2" > "$1/rows"
  printf '%s\n' "$3" > "$1/files"
}

selftest_consist() {
  local t nl=$'\n' r l_rows l_files
  t=$(mktemp -d)
  l_rows=$'1/aa.jpg\n1/aa-thumb.jpg\n1/bb.jpg\n2/cc.jpg'

  # media_agree: "ROWS FILES MISSING ORPHAN CLASS".
  st_lists "$t" "$l_rows" "$l_rows"
  st_eq "media_agree identical lists" "4 4 0 0 AGREE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_lists "$t" "$l_rows" "$l_rows${nl}1/extra.jpg"
  st_eq "media_agree one extra file" "4 5 0 1 ORPHAN-ONLY" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_lists "$t" "$l_rows" $'1/aa.jpg\n1/aa-thumb.jpg\n2/cc.jpg'
  st_eq "media_agree one missing file" "4 3 1 0 ROW-WITHOUT-FILE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_lists "$t" "$l_rows" $'1/aa.jpg\n1/aa-thumb.jpg\n2/cc.jpg\n1/extra.jpg'
  st_eq "media_agree both at once: the harmful direction dominates" "4 4 1 1 ROW-WITHOUT-FILE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_lists "$t" "$l_rows" "$l_rows${nl}1/bulk/bulk-1.bin${nl}1/bulk/bulk-2.bin"
  st_eq "media_agree a file below bulk/ is excluded" "4 4 0 0 AGREE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_eq "media_agree the same without the exclusion" "4 6 0 2 ORPHAN-ONLY" "$(media_agree "$t/rows" "$t/files" '')"
  st_lists "$t" "$l_rows${nl}1/bulk/x.bin" "$l_rows"
  st_eq "media_agree a row below bulk/ is excluded too" "4 4 0 0 AGREE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_lists "$t" "$l_rows" "$l_rows${nl}1/mybulk/x.bin"
  st_eq "media_agree bulk/ is a directory name, not a suffix" "4 5 0 1 ORPHAN-ONLY" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  printf '%s\n%s\n%s\n' "1/bb.jpg" "1/aa.jpg" "1/bb.jpg" > "$t/rows"
  printf '%s\n%s\n' "1/aa.jpg" "1/bb.jpg" > "$t/files"
  st_eq "media_agree order and duplicates do not matter" "2 2 0 0 AGREE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  : > "$t/rows"
  : > "$t/files"
  st_eq "media_agree two empty lists" "0 0 0 0 AGREE" "$(media_agree "$t/rows" "$t/files" '*/bulk/')"
  st_fail "media_agree a missing file" media_agree "$t/none" "$t/files" ''
  st_lists "$t" "$l_rows" "$l_rows${nl}1/extra.jpg"
  media_agree "$t/rows" "$t/files" '*/bulk/' "$t/m.out" "$t/o.out" > /dev/null
  st_eq "media_agree writes the orphan list" "1/extra.jpg" "$(cat "$t/o.out")"
  st_eq "media_agree writes an empty missing list" "0" "$(grep -c . "$t/m.out" || true)"
  st_lists "$t" "$l_rows" $'1/aa.jpg\n1/aa-thumb.jpg\n2/cc.jpg'
  media_agree "$t/rows" "$t/files" '*/bulk/' "$t/m.out" "$t/o.out" > /dev/null
  st_eq "media_agree writes the missing list" "1/bb.jpg" "$(cat "$t/m.out")"

  # run_class BACKUP_RC MANIFEST RESTORE_RC AGREE_CLASS
  st_eq "run_class agree" "AGREE" "$(run_class 0 yes 0 AGREE)"
  st_eq "run_class orphan-only" "ORPHAN-ONLY" "$(run_class 0 yes 0 ORPHAN-ONLY)"
  st_eq "run_class row-without-file" "ROW-WITHOUT-FILE" "$(run_class 0 yes 0 ROW-WITHOUT-FILE)"
  st_eq "run_class backup exit 24, no MANIFEST, lists say agree" "BACKUP-REFUSED" "$(run_class 24 no - AGREE)"
  st_eq "run_class backup exit 24, no MANIFEST, no class at all" "BACKUP-REFUSED" "$(run_class 24 no - -)"
  st_eq "run_class backup exit 1, no MANIFEST, lists say row-without-file" "BACKUP-REFUSED" "$(run_class 1 no 0 ROW-WITHOUT-FILE)"
  st_eq "run_class restore exit 1" "RESTORE-FAILED" "$(run_class 0 yes 1 AGREE)"
  st_eq "run_class restore exit 255 beats the lists" "RESTORE-FAILED" "$(run_class 0 yes 255 ROW-WITHOUT-FILE)"
  st_eq "run_class not restored (-)" "ORPHAN-ONLY" "$(run_class 0 yes - ORPHAN-ONLY)"
  st_eq "run_class a late failure with a MANIFEST keeps the lists' class" "AGREE" "$(run_class 1 yes 0 AGREE)"
  st_fail "run_class exit 0 without a MANIFEST is not a backup" run_class 0 no 0 AGREE
  st_fail "run_class an unknown class" run_class 0 yes 0 MAYBE
  st_fail "run_class a non-number backup status" run_class x yes 0 AGREE
  st_fail "run_class a non-number restore status" run_class 0 yes x AGREE
  st_fail "run_class MANIFEST answer not yes or no" run_class 0 maybe 0 AGREE
  st_eq "run_class prints nothing when it refuses" "" "$(run_class 0 yes 0 MAYBE || true)"

  # window_hits DB_END MEDIA_END, events "START END" on stdin: "BEFORE WINDOW AFTER".
  st_eq "window_hits no events" "0 0 0" "$(printf '' | window_hits 1000 2000)"
  st_eq "window_hits an event that ended exactly at the end of the database step is before" "1 0 0" "$(printf '900 1000\n' | window_hits 1000 2000)"
  st_eq "window_hits an event one millisecond after the database step is in the window" "0 1 0" "$(printf '900 1001\n' | window_hits 1000 2000)"
  st_eq "window_hits an event one millisecond before the end of the media step is in" "0 1 0" "$(printf '1500 1999\n' | window_hits 1000 2000)"
  st_eq "window_hits an event that ended exactly at the end of the media step is after" "0 0 1" "$(printf '1500 2000\n' | window_hits 1000 2000)"
  st_eq "window_hits several" "2 3 1" "$(printf '1 10\n5 999\n1100 1200\n1200 1300\n1900 1950\n1990 2500\n' | window_hits 1000 2000)"
  st_eq "window_hits an event that began before the snapshot and ended in the window counts as ended in it" "0 1 0" "$(printf '800 1500\n' | window_hits 1000 2000)"

  # step_span TRANSCRIPT TEXT: the timestamp of a line and of the next step line (the
  # binary's own lines in between do not end the database step).
  printf '%s\n' '1000 === holzcloud-CMS backup: 20261026-100000 ===' '1010 Backing up database...' '1100 2026/10/26 11:47:01 INFO sqlite opened journal_mode=wal path=/x' '1200 backup written and verified: /x' '1450 Backing up CSRF key...' '1460 Backing up media...' '2100 Backing up templates...' '2200 === Backup complete: /x ===' 'WALL 990 2300' 'RC 0' > "$t/tr"
  st_eq "step_span database ends at the next step line, not at the binary's output" "1010 1450" "$(step_span "$t/tr" 'Backing up database...')"
  st_eq "step_span media" "1460 2100" "$(step_span "$t/tr" 'Backing up media...')"
  st_eq "step_span the last step ends at the complete line" "2100 2200" "$(step_span "$t/tr" 'Backing up templates...')"
  st_fail "step_span a step the transcript lacks" step_span "$t/tr" 'Backing up marke...'
  st_fail "step_span the last line has no next step line" step_span "$t/tr" 'RC 0'
  st_fail "step_span the complete line is followed by no step line" step_span "$t/tr" '=== Backup complete: /x ==='
  r=$(step_span "$t/tr" 'Backing up media...')
  st_eq "step_span feeds window_hits" "0 1 1" "$(printf '1500 1900\n1900 2200\n' | window_hits "${r% *}" "${r#* }")"
  rm -rf "${t:?}"
}
# ---------------------------------------------------------------------------
# Phase 25, plan 01: CON-01. The pair of a backup, the database snapshot and the
# media copy, is not atomic. backup.sh and DEPLOY.md say so and it was never
# provoked: an editor saves pages and uploads and deletes media while the shipped
# backup.sh runs, each backup is restored on a second container by the shipped
# restore.sh, and database and media are compared in both directions.
# ---------------------------------------------------------------------------

# usage again: the header of this file is longer than it was in phase 24, and the
# first usage() above prints a fixed range of lines. A later definition wins.
usage() {
  awk 'NR == 1 { next } /^#/ { print; next } { exit }' "${BASH_SOURCE[0]}" >&2
  exit 2
}

# ---- pure functions (no Docker, no clock; tables in selftest_consist) ----------

# media_filter FILE EXCLUDE_PATTERNS: the sorted, unique, non-empty lines of FILE
# that do not begin with an excluded glob (EXCLUDE_PATTERNS is a space separated
# list of globs, each matched against the whole line as a prefix: `*/bulk/` drops
# every file below a directory called bulk).
media_filter() {
  local p pat keep
  [ -f "$1" ] || return 1
  set -f
  LC_ALL=C sort -u "$1" | while IFS= read -r p; do
    [ -n "$p" ] || continue
    keep=yes
    for pat in ${2:-}; do
      case "$p" in $pat*) keep=no ;; esac
    done
    if [ "$keep" = yes ]; then printf '%s\n' "$p"; fi
  done
  set +f
}

# media_agree ROWS_FILE FILES_FILE EXCLUDE_PATTERNS [MISSING_OUT [ORPHAN_OUT]]:
# ROWS_FILE lists WEBSITE/FILENAME of every media row and every variant row,
# FILES_FILE the files below data/media, relative to it. Prints
# "ROWS FILES MISSING ORPHAN CLASS": the numbers after the exclusion, MISSING the
# rows without a file, ORPHAN the files without a row, and the class AGREE,
# ORPHAN-ONLY (the direction backup.sh calls harmless) or ROW-WITHOUT-FILE (the
# direction it says is possible). The harmful direction dominates when both occur.
# The two lists of names are written to MISSING_OUT and ORPHAN_OUT when given.
media_agree() {
  local rows=$1 files=$2 excl=${3:-} mout=${4:-} oout=${5:-} t nr nf nm no cls
  [ -f "$rows" ] && [ -f "$files" ] || return 1
  t=$(mktemp -d) || return 1
  media_filter "$rows" "$excl" > "$t/r" || true
  media_filter "$files" "$excl" > "$t/f" || true
  LC_ALL=C comm -23 "$t/r" "$t/f" > "$t/m"
  LC_ALL=C comm -13 "$t/r" "$t/f" > "$t/o"
  nr=$(grep -c . "$t/r" || true)
  nf=$(grep -c . "$t/f" || true)
  nm=$(grep -c . "$t/m" || true)
  no=$(grep -c . "$t/o" || true)
  if [ "$nm" -gt 0 ]; then
    cls=ROW-WITHOUT-FILE
  elif [ "$no" -gt 0 ]; then
    cls=ORPHAN-ONLY
  else
    cls=AGREE
  fi
  if [ -n "$mout" ]; then cp "$t/m" "$mout"; fi
  if [ -n "$oout" ]; then cp "$t/o" "$oout"; fi
  rm -rf "${t:?}"
  printf '%s %s %s %s %s\n' "$nr" "$nf" "$nm" "$no" "$cls"
}

# run_class BACKUP_RC MANIFEST_PRESENT RESTORE_RC AGREE_CLASS: the class of one run.
# BACKUP-REFUSED: backup.sh ended non-zero and wrote no MANIFEST (whatever the
# lists say: there is no backup to compare). RESTORE-FAILED: restore.sh ended
# non-zero (RESTORE_RC "-" means the run was not restored). Otherwise the class
# the lists gave. Anything else (a non-number, an unknown class, a zero exit
# without a MANIFEST) is refused, status 1, nothing printed.
run_class() {
  local brc=$1 man=$2 rrc=$3 cls=$4
  case "$brc" in "" | *[!0-9]*) return 1 ;; esac
  case "$man" in yes | no) ;; *) return 1 ;; esac
  if [ "$brc" -ne 0 ] && [ "$man" = no ]; then
    echo BACKUP-REFUSED
    return 0
  fi
  if [ "$brc" -eq 0 ] && [ "$man" = no ]; then return 1; fi
  case "$rrc" in
    - | "") ;;
    *[!0-9]*) return 1 ;;
    *)
      if [ "$rrc" -ne 0 ]; then
        echo RESTORE-FAILED
        return 0
      fi
      ;;
  esac
  case "$cls" in
    AGREE | ORPHAN-ONLY | ROW-WITHOUT-FILE) echo "$cls" ;;
    *) return 1 ;;
  esac
}

# window_hits DB_END MEDIA_END: reads lines "START END" (milliseconds) on stdin and
# prints "BEFORE WINDOW AFTER", the number of events by the interval they ended in:
# BEFORE at or before the end of the database step, WINDOW strictly after it and
# strictly before the end of the media step, AFTER at or after the end of the media
# step. Only the end counts here (the start is for window_name).
window_hits() {
  awk -v de="$1" -v me="$2" '
    NF >= 2 {
      e = $2 + 0
      if (e <= de + 0) b++
      else if (e < me + 0) w++
      else a++
    }
    END { printf "%d %d %d\n", b, w, a }'
}

# step_span TRANSCRIPT TEXT: the transcript of a timestamped script ("MS TEXT" per
# line). Prints "T0 T1": the timestamp of the first line whose text is TEXT and the
# timestamp of the NEXT STEP LINE after it: the next line that begins "Backing up ",
# "Pruning backups" or "=== ". The binary prints lines of its own in the database
# step (sqlite opened, backup verified), which are not the end of it; the plan's
# wording "the next line" would have ended the step 0.1 s early. Status 1 and
# nothing printed when either line is missing.
step_span() {
  local out
  out=$(awk -v want="$2" '
    {
      ts = $1
      $1 = ""
      sub(/^ /, "")
      if (hit) {
        if ($0 ~ /^(Backing up |Pruning backups|=== )/) { print t0, ts; found = 1; exit }
        next
      }
      if ($0 == want) { t0 = ts; hit = 1 }
    }
    END { if (!found) exit 1 }' "$1") || return 1
  printf '%s\n' "$out"
}

# ---- the document's lines --------------------------------------------------------

# doc_backup_line / doc_restore_line: the two commands of the document, read from
# deploy/DEPLOY.md. Printed as "LINENUMBER:TEXT". The restore line has the sample
# timestamp of the document in its last word; the caller replaces it.
doc_backup_line() {
  grep -n -m1 -x -F -- 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups' deploy/DEPLOY.md
}
doc_restore_line() {
  grep -n -m1 '^sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/[0-9-]*$' deploy/DEPLOY.md
}

# ---- scripts shipped to the containers ----------------------------------------

# The editor's session, headless. The password is read from a root-only file in the
# container and neither it nor a cookie nor a token is ever printed; the only
# output is one line "HTTP CODE REDIRECT-PATH". The token comes from the form page
# of the route the request is about to post to. The unit sets HOLZCLOUD_SECURE=true,
# so a post over plain http needs a Referer that carries the https origin.
read -r -d '' P25_SESS_SH <<'SESS_EOF' || true
#!/bin/bash
# hc-sess.sh MODE ARGS: login EMAIL PASSFILE JAR | get PATH JAR | page WID JAR SLUG TITLE MEDIAPATH
#                       | upload WID JAR FILE ORIGINALNAME | delete WID JAR MEDIAID
set -u
umask 077
base=http://127.0.0.1:8080
origin=https://127.0.0.1:8080
mode=${1:-}
shift || true
tmp=$(mktemp /root/.hc-sess.XXXXXX)
trap 'rm -f "$tmp"' EXIT

token_of() {
  grep -o 'name="gorilla.csrf.Token" value="[^"]*"' "$1" | head -n 1 \
    | sed 's/.*value="//; s/"$//; s/&#43;/+/g; s/&#61;/=/g; s/&#47;/\//g'
}
result() {
  local code=${1%% *} loc=${1#* }
  loc=${loc#$base}
  echo "HTTP $code ${loc:--}"
}
fetch_token() {
  curl -s --max-time 60 -o "$tmp" -c "$2" -b "$2" "$base$1" || true
  token_of "$tmp"
}
notoken() { echo "HTTP none no-csrf-field"; exit 0; }
fmt='%{http_code} %{redirect_url}'

case "$mode" in
  login)
    email=$1; passfile=$2; jar=$3
    tok=$(fetch_token /admin/login "$jar")
    [ -n "$tok" ] || notoken
    pass=$(cat "$passfile")
    res=$(curl -s -o /dev/null --max-time 120 -w "$fmt" -c "$jar" -b "$jar" \
      -H "Referer: $origin/admin/login" \
      --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "email=$email" \
      --data-urlencode "password=$pass" "$base/admin/login")
    result "$res"
    ;;
  get)
    res=$(curl -s -o /dev/null --max-time 60 -w "$fmt" -c "$2" -b "$2" "$base$1")
    result "$res"
    ;;
  page)
    wid=$1; jar=$2; slug=$3; title=$4; ref=$5
    path=/admin/websites/$wid/pages/new
    tok=$(fetch_token "$path" "$jar")
    [ -n "$tok" ] || notoken
    res=$(curl -s -o /dev/null --max-time 120 -w "$fmt" -c "$jar" -b "$jar" \
      -H "Referer: $origin$path" -H "X-CSRF-Token: $tok" \
      --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "title=$title" \
      --data-urlencode "slug=$slug" --data-urlencode "status=draft" \
      --data-urlencode "content_markdown=Page $slug ![churn]($ref)" "$base$path")
    result "$res"
    ;;
  upload)
    wid=$1; jar=$2; file=$3; name=$4
    path=/admin/websites/$wid/media
    tok=$(fetch_token "$path" "$jar")
    [ -n "$tok" ] || notoken
    res=$(curl -s -o /dev/null --max-time 120 -w "$fmt" -c "$jar" -b "$jar" \
      -H "Referer: $origin$path" -H "X-CSRF-Token: $tok" \
      -F "media=@$file;filename=$name;type=image/jpeg" "$base$path/upload")
    if [ "${res%% *}" = 403 ]; then
      res=$(curl -s -o /dev/null --max-time 120 -w "$fmt" -c "$jar" -b "$jar" \
        -H "Referer: $origin$path" -F "gorilla.csrf.Token=$tok" \
        -F "media=@$file;filename=$name;type=image/jpeg" "$base$path/upload")
    fi
    result "$res"
    ;;
  delete)
    wid=$1; jar=$2; id=$3
    path=/admin/websites/$wid/media
    tok=$(fetch_token "$path" "$jar")
    [ -n "$tok" ] || notoken
    res=$(curl -s -o /dev/null --max-time 120 -w "$fmt" -c "$jar" -b "$jar" \
      -H "Referer: $origin$path" -H "X-CSRF-Token: $tok" \
      --data-urlencode "gorilla.csrf.Token=$tok" -X POST "$base$path/$id/delete")
    result "$res"
    ;;
  *)
    echo "HTTP none bad-mode"
    ;;
esac
SESS_EOF

# The churn generator. Runs in the container in the background; each worker loops
# until /root/hc-churn.stop exists and appends one line per finished request to
# /root/hc-churn.log: END START KIND CODE ID, the times in milliseconds of the
# container's clock. KIND is upload, page or delete. ID is the original name of an
# upload, the slug of a page, and for a delete MEDIAID:WEBSITE/STEM (the stem is the
# stored name up to the first dot, which the original and its variants share). The
# deleter takes the oldest churn upload (original name churn-*) that is left,
# never a baseline file.
read -r -d '' P25_CHURN_SH <<'CHURN_EOF' || true
#!/bin/bash
# hc-churn.sh MODE WORKERS TAG WID BASEREF PAUSE   (MODE adds | deletes)
mode=$1; workers=$2; tag=$3; wid=$4; baseref=$5; pause=${6:-0.3}
LOG=/root/hc-churn.log
STOP=/root/hc-churn.stop
SESS=/root/hc-sess.sh
IMG=/root/hc-base.jpg
DB=/opt/holzcloud/data/holzcloud.sqlite
umask 077
ms() { date +%s%3N; }
logline() { printf '%s\n' "$*" >> "$LOG"; }
# The pause plus up to 0.29 s of jitter: workers that start together and wait the same
# time stay together, and then every backup meets the same phase of the load (measured:
# the same count of uploads inside the window in run after run).
nap() { sleep "$pause"; sleep "0.$((RANDOM % 3))$((RANDOM % 10))"; }
code_of() { local r=$1; set -- $r; echo "${2:-none}"; }
ensure_login() {
  local r
  r=$(bash "$SESS" get /admin/ "$1")
  [ "$r" = "HTTP 200 -" ] || bash "$SESS" login churn@example.test /root/.hc-login-pass "$1" > /dev/null
}
uploader() {
  local w=$1 jar=/root/.hc-jar-churn-u$1 n=0 f name s e res
  ensure_login "$jar"
  while [ ! -e "$STOP" ]; do
    n=$((n + 1))
    name="churn-$tag-w$w-$n.jpg"
    f=/root/.hc-churn-up-$w.jpg
    cat "$IMG" > "$f"
    printf 'trailer %s %s %s %s' "$tag" "$w" "$n" "$(date +%s%N)" >> "$f"
    s=$(ms); res=$(bash "$SESS" upload "$wid" "$jar" "$f" "$name"); e=$(ms)
    logline "$e $s upload $(code_of "$res") $name"
    rm -f "$f"
    nap
  done
}
pager() {
  local jar=/root/.hc-jar-churn-p n=0 slug s e res
  ensure_login "$jar"
  while [ ! -e "$STOP" ]; do
    n=$((n + 1))
    slug="churn-$tag-p$n"
    s=$(ms); res=$(bash "$SESS" page "$wid" "$jar" "$slug" "Churn $tag $n" "$baseref"); e=$(ms)
    logline "$e $s page $(code_of "$res") $slug"
    nap
  done
}
deleter() {
  local jar=/root/.hc-jar-churn-d last=0 row id fn stem s e res
  ensure_login "$jar"
  while [ ! -e "$STOP" ]; do
    row=$(runuser -u holzcloud -- sqlite3 -readonly "$DB" "select id || '|' || filename from media where original_name like 'churn-%' and id > $last order by id limit 1" 2> /dev/null)
    if [ -z "$row" ]; then
      sleep 0.2
      continue
    fi
    id=${row%%|*}; fn=${row#*|}; stem=${fn%%.*}
    s=$(ms); res=$(bash "$SESS" delete "$wid" "$jar" "$id"); e=$(ms)
    logline "$e $s delete $(code_of "$res") $id:$wid/$stem"
    last=$id
    nap
  done
}
w=1
while [ "$w" -le "$workers" ]; do
  uploader "$w" &
  w=$((w + 1))
done
pager &
if [ "$mode" = deletes ]; then deleter & fi
wait
CHURN_EOF

# Run in the container: the command of $1 with a millisecond timestamp in front of
# every line of its output, its own exit status in the RC line. stdin reaches the
# command (the restore's answer). The status is the command's (PIPESTATUS), not the
# loop's that stamps the lines.
read -r -d '' P25_STAMP_SH <<'STAMP_EOF' || true
set -o pipefail
t0=$(date +%s%3N)
bash -c "$1" 2>&1 | while IFS= read -r l; do printf '%s %s\n' "$(date +%s%3N)" "$l"; done
rc=${PIPESTATUS[0]}
echo "WALL $t0 $(date +%s%3N)"
echo "RC $rc"
exit 0
STAMP_EOF

# The comparator, run in the container that holds the backup (mode backup) or the
# restored site (mode restored; live is the same without the tree). Arguments:
# MODE DATABASE MEDIADIR BACKUPDIR DATADIR. It works on a consistent COPY of the
# database and prints sections, "@@NAME" lines between them; the judgement is made
# by the driver. No file is created next to the backup.
read -r -d '' P25_COMPARE_SH <<'CMP_EOF' || true
set -u
mode=$1; db=$2; media=$3; bk=$4; data=$5
t=$(mktemp -d /root/.hc-cmp.XXXXXX) || exit 97
trap 'rm -rf "$t"' EXIT
if [ "$mode" = backup ]; then
  cp "$db" "$t/holzcloud.sqlite"
else
  sqlite3 -readonly "$db" ".backup '$t/holzcloud.sqlite'"
fi
q="select website_id || '/' || filename from media union select m.website_id || '/' || v.filename from media_variants v join media m on m.id = v.media_id order by 1"
echo '@@ROWS'
sqlite3 "$t/holzcloud.sqlite" "$q"
echo '@@FILES'
( cd "$media" && find . -type f | sed 's|^\./||' | LC_ALL=C sort )
echo '@@REFS'
sqlite3 "$t/holzcloud.sqlite" "select content_html from pages" | grep -o '/media/[0-9]*/[A-Za-z0-9._-]*' | sed 's|^/media/||' | LC_ALL=C sort -u
echo '@@CHECK'
out=$(HOLZCLOUD_DATA_DIR="$t" /opt/holzcloud/holzcloud check 2>&1); rc=$?
echo "CHECK-RC: $rc"
echo "CHECK-OUT: $(printf '%s' "$out" | tr '\n' ' ')"
echo "INTEGRITY: $(sqlite3 "$t/holzcloud.sqlite" 'PRAGMA integrity_check' | tr '\n' ' ')"
echo "FKCHECK: $(sqlite3 "$t/holzcloud.sqlite" 'PRAGMA foreign_key_check' | wc -l | tr -d ' ')"
echo "ROWS-MEDIA: $(sqlite3 "$t/holzcloud.sqlite" 'select count(*) from media')"
echo "ROWS-VARIANTS: $(sqlite3 "$t/holzcloud.sqlite" 'select count(*) from media_variants')"
echo "ROWS-PAGES: $(sqlite3 "$t/holzcloud.sqlite" 'select count(*) from pages')"
if [ "$mode" = restored ]; then
  tree() { ( cd "$1" && find . -type f ! -name 'holzcloud.sqlite*' ! -name 'pre-upgrade-*.sqlite*' ! -name MANIFEST -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum ); }
  echo '@@TREE-RESTORED'
  tree "$data"
  echo '@@TREE-BACKUP'
  tree "$bk"
fi
echo '@@END'
CMP_EOF

# ---- helpers ---------------------------------------------------------------

# p25_ship: the three shell texts and the base image into the source container.
p25_ship() {
  printf '%s\n' "$P25_SESS_SH" | dxi bash -c 'umask 077; cat > /root/hc-sess.sh && chmod 700 /root/hc-sess.sh' \
    || die "could not ship the session helper into $NAME"
  printf '%s\n' "$P25_CHURN_SH" | dxi bash -c 'umask 077; cat > /root/hc-churn.sh && chmod 700 /root/hc-churn.sh' \
    || die "could not ship the churn generator into $NAME"
  dx bash -c 'umask 077; [ -s /root/hc-base.jpg ] || cp /tmp/hc-upload.jpg /root/hc-base.jpg; test -s /root/hc-base.jpg' \
    || die "there is no /root/hc-base.jpg and no /tmp/hc-upload.jpg in $NAME to make it from; run 'bash deploy/test/nightly.sh source' first"
}

# sess ARGS...: the session helper in the source container; prints "HTTP CODE REDIRECT".
sess() { dxo bash /root/hc-sess.sh "$@"; }

# p25_wid / p25_baseref: the website called Test and the media path of its first file.
p25_wid() {
  dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id from websites where name = 'Test'" | head -n 1
}
p25_baseref() {
  dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select '/media/' || website_id || '/' || filename from media order by id limit 1" | head -n 1
}

# run_prefix STAGE N: the stem of the raw files of a run. Stage adds keeps the plain
# name of the plan; stage deletes puts its name in front so the two never collide.
run_prefix() {
  if [ "$1" = adds ]; then
    printf 'consist-run-%s\n' "$2"
  elif [ "$1" = C0 ] || [ "$1" = C1 ] || [ "$1" = C2 ]; then
    printf 'window-%s\n' "$1"
  else
    printf 'consist-%s-run-%s\n' "$1" "$2"
  fi
}

# split_sections FILE DIR: the "@@NAME" sections of the comparator output as DIR/NAME.
split_sections() {
  awk -v d="$2" '/^@@/ { f = d "/" substr($0, 3); printf "" > f; next } f != "" { print >> f }' "$1"
}

# ---- prep ---------------------------------------------------------------------

cmd_prep() {
  local o=$OUT_DIR/prep.txt wid setup adm wall ed created n m i rc=0 slug epoch pg mrows fn
  need_container
  mkdir -p "$OUT_DIR"
  BADS=0
  res_begin "$o"
  p25_ship
  wid=$(p25_wid) || wid=""
  key_line "WEBSITE: ${wid:-?}"
  if [ -n "$wid" ]; then
    check_pass "the website called Test has id $wid"
  else
    check_bad "no website called Test in $NAME"
  fi

  # (b) The baseline admin cannot be the editor: the compulsory two-factor setup
  # stands in front of every admin URL. Recorded, not assumed.
  setup=$(sed -n 's/^const SetupPath = "\(.*\)"$/\1/p' internal/auth/twofactor.go)
  key_line "SETUP-PATH: ${setup:-?} (internal/auth/twofactor.go, SetupPath)"
  adm=$(sess login login@example.test /root/.hc-login-pass /root/.hc-jar-prep-admin) || adm="HTTP none error"
  key_line "ADMIN-LOGIN: $adm"
  wall=$(sess get "/admin/websites/${wid:-1}/pages/new" /root/.hc-jar-prep-admin) || wall="HTTP none error"
  key_line "ADMIN-SESSION: $wall"
  if [ "$adm" != "HTTP 303 /admin/" ]; then
    check_bad "the baseline admin could not log in: '$adm'"
  elif [ "$wall" = "HTTP 303 $setup" ]; then
    check_pass "the admin session on the page form is answered with a redirect to $setup: auth.MustHaveSecondFactor makes the two-factor setup compulsory for an admin"
  else
    check_info "the admin session on the page form got '$wall' (not a redirect to $setup); the editor session below is used all the same"
  fi

  # (c) The editor, created by the new binary as the service user; the password is
  # the baseline login password, read from its root-only file on stdin, so that no
  # new secret exists and leak_check already covers it.
  ed=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select role from users where email = 'churn@example.test'") || ed=""
  if [ -n "$ed" ]; then
    check_info "churn@example.test exists already (role $ed); it is not created again"
  else
    created=$(dx bash -c 'runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud user create -email churn@example.test -role editor < /root/.hc-login-pass 2>&1' | tr -d '\r') || rc=$?
    key_line "EDITOR-CREATE: $(printf '%s' "$created" | tail -n 1)"
    if [ "$rc" -ne 0 ]; then check_bad "holzcloud user create -role editor ended with status $rc"; fi
    ed=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select role from users where email = 'churn@example.test'") || ed=""
  fi
  if [ "$ed" = editor ]; then
    check_pass "churn@example.test is an account with the role editor"
  else
    check_bad "churn@example.test has the role '${ed:-none}', expected editor"
  fi

  # (d) The editor logs in.
  adm=$(sess login churn@example.test /root/.hc-login-pass /root/.hc-jar-prep-editor) || adm="HTTP none error"
  key_line "EDITOR-LOGIN: $adm"
  if [ "$adm" = "HTTP 303 /admin/" ]; then
    check_pass "the editor logs in: $adm"
  else
    check_bad "the editor login answered '$adm', expected 'HTTP 303 /admin/'"
  fi

  # (e) Both write paths once, and the proof that two uploads give two rows.
  epoch=$(date +%s)
  slug=churn-prep-$epoch
  pg=$(sess page "${wid:-1}" /root/.hc-jar-prep-editor "$slug" "Churn prep $epoch" "$(p25_baseref)") || pg="HTTP none error"
  key_line "PAGE-SAVE: $pg"
  for i in 1 2; do
    dx bash -c 'umask 077; cat /root/hc-base.jpg > "$1"; printf "trailer prep %s %s" "$2" "$(date +%s%N)" >> "$1"' _ "/root/.hc-prep-up-$i.jpg" "$i" || check_bad "could not make the upload file $i"
    m=$(sess upload "${wid:-1}" /root/.hc-jar-prep-editor "/root/.hc-prep-up-$i.jpg" "churn-prep-$epoch-$i.jpg") || m="HTTP none error"
    key_line "UPLOAD-$i: $m"
    dx rm -f "/root/.hc-prep-up-$i.jpg" || true
  done
  sleep 1
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from pages where slug = '$slug' and status = 'draft'") || n="?"
  if [ "$n" = 1 ]; then
    check_pass "the page row $slug exists (a draft: what an editor can make)"
  else
    check_bad "the page row $slug: $n rows, expected 1"
  fi
  mrows=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from media where original_name like 'churn-prep-$epoch-%'") || mrows="?"
  key_line "UPLOAD-ROWS: $mrows"
  if [ "$mrows" = 2 ]; then
    check_pass "two uploads that differ in their trailing bytes gave two media rows (the strip step keeps the difference)"
  else
    check_bad "two uploads gave $mrows media rows, expected 2 (a hash already exists answer, or a refusal)"
  fi
  fn=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select website_id || '/' || filename from media where original_name = 'churn-prep-$epoch-1.jpg'") || fn=""
  if [ -n "$fn" ] && dx test -f "/opt/holzcloud/data/media/$fn"; then
    check_pass "the file of the first upload is below data/media: $fn"
  else
    check_bad "the file of the first upload is not below data/media ('$fn')"
  fi
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from media_usage u join pages p on p.id = u.page_id where p.slug = '$slug'") || n="?"
  if [ "$n" = 1 ]; then
    check_pass "the page references an uploaded media file: media_usage has its row"
  else
    check_bad "media_usage has $n rows for the page $slug, expected 1"
  fi
  dx rm -f /root/.hc-jar-prep-admin /root/.hc-jar-prep-editor || true

  # (f) What this load is and is not.
  deviation "DEVIATION[editor-session]: the load is an EDITOR's session, not the administrator's. auth.MustHaveSecondFactor(role, viaSSO) is true for an admin, so every admin URL answers an admin session with a redirect to the compulsory two-factor setup (ADMIN-SESSION above) and no admin session reaches a page form. The account churn@example.test, role editor, was created for it with the new binary and the baseline login password (no new secret exists)"
  deviation "DEVIATION[churn]: the load is curl run inside the container against 127.0.0.1:8080 with the CSRF token of each form, not browsers; the pace is set by CHURN_PAUSE and CHURN_WORKERS, not by people"

  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "PREP FAILED"
    say "nightly.sh: PREP FAILED, $BADS BAD line(s); see $o"
    exit 1
  fi
  key_line "PREP OK"
}

# ---- consist --------------------------------------------------------------------

# churn_wait_ready: until the log shows two finished requests, at most 90 seconds.
churn_wait_ready() {
  local i=0 n
  while [ "$i" -lt 90 ]; do
    n=$(dxo bash -c 'grep -c . /root/hc-churn.log 2>/dev/null || true') || n=0
    if [ "${n:-0}" -ge 2 ]; then return 0; fi
    sleep 1
    i=$((i + 1))
  done
  say "churn: the log shows no two finished requests after 90 s; its contents:"
  dx bash -c 'cat /root/hc-churn.log 2>/dev/null; pgrep -af hc-churn.sh' >&2 || true
  return 1
}

# churn_stop: write the stop file, wait up to 30 s for the generator to end and the
# log to stop growing; kill what is left and say so.
churn_stop() {
  local i=0 a b
  dx touch /root/hc-churn.stop || true
  while [ "$i" -lt 30 ]; do
    if ! dx pgrep -f 'hc-churn.sh' > /dev/null 2>&1; then
      a=$(dxo bash -c 'grep -c . /root/hc-churn.log 2>/dev/null || true') || a=0
      sleep 1
      b=$(dxo bash -c 'grep -c . /root/hc-churn.log 2>/dev/null || true') || b=0
      if [ "$a" = "$b" ]; then return 0; fi
    fi
    sleep 1
    i=$((i + 1))
  done
  say "churn: the generator is still running 30 s after the stop file; it is killed"
  dx pkill -f 'hc-churn.sh' > /dev/null 2>&1 || true
  return 1
}

# churn_count FILE KIND: the successful (HTTP 303) lines of that kind.
churn_count() {
  awk -v k="$2" '$3 == k && $4 == 303 { n++ } END { print n + 0 }' "$1"
}

# consist_run STAGE N RESTORE: one run. RESTORE is yes (restored by restore.sh on the
# second container and compared there) or no (compared on the backup itself).
# Results in the globals R_*; the run's line is appended to consist-runs.txt by the
# caller. Returns 1 when the harness itself failed (a BAD line says why).
consist_run() {
  local stage=$1 n=$2 restore=$3 pfx w tag brc man dir bdir ts rrc=- rsync_st line
  local db0 db1 me0 me1 span tdir agree keep_out nbad0 rows files miss orph
  local ref_count dang tr_a tr_b res wr
  pfx=$(run_prefix "$stage" "$n")
  w=$OUT_DIR/.work-$pfx
  rm -rf "$w"
  mkdir -p "$w"
  tag=$stage$n
  R_CLASS=? R_BRC=? R_RRC=- R_MISSING=? R_ORPHAN=? R_WU=? R_WP=? R_WD=? R_U=? R_P=? R_D=? R_BS=? R_DIR=? R_RESTORED=$restore R_RSYNC=- R_BADRUN=no R_NAMED=- R_WNAME=- R_RS=0 R_RETRIES=0 R_LEFT=0
  keep_out=$CHECK_OUT

  if [ -n "${P25_HOLD:-}" ]; then
    # A control of cmd_window: no load; the stand-in pauses backup.sh and one write is made.
    : > "$w/churn.txt"
    hold_backup "$w" "$P25_HOLD" || return 1
  else
  # (1) The load, and a wait until it is running.
  dx rm -f /root/hc-churn.stop /root/hc-churn.log || true
  dx bash -c ': > /root/hc-churn.log' || true
  if ! docker exec -d "$NAME" bash /root/hc-churn.sh "$stage" "$CHURN_WORKERS" "$tag" "$WID" "$BASEREF" "$CHURN_PAUSE"; then
    check_bad "run $stage $n: the churn generator could not be started"
    return 1
  fi
  if ! churn_wait_ready; then
    churn_stop || true
    check_bad "run $stage $n: the load did not start (no two finished requests within 90 s)"
    return 1
  fi

  # (2) The document's manual line, with a timestamp on every output line. It starts
  # a random fraction of a second after the load is running, so that the runs do not
  # all meet the same phase of it.
  sleep "0.$(printf '%03d' $((RANDOM % 1000)))"
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$BACKUP_LINE" > "$w/backup.txt" 2>&1 || true
  churn_stop || check_bad "run $stage $n: the churn generator did not stop in time"
  dx cat /root/hc-churn.log > "$w/churn.txt" 2> /dev/null || true
  fi
  brc=$(sed -n 's/^RC //p' "$w/backup.txt" | tail -n 1)
  if [ -z "$brc" ]; then
    check_bad "run $stage $n: backup.txt holds no RC line; the transcript ends: $(tail -n 3 "$w/backup.txt" | tr '\n' '|')"
    cp "$w/backup.txt" "$OUT_DIR/$pfx-backup.txt" 2> /dev/null || true
    return 1
  fi
  R_BRC=$brc
  dir=$(sed -n 's/^[0-9]* === Backup complete: \(.*\) ===$/\1/p' "$w/backup.txt" | tail -n 1)
  ts=$(sed -n 's/^[0-9]* === holzcloud-CMS backup: \(.*\) ===$/\1/p' "$w/backup.txt" | head -n 1)
  bdir=$dir
  [ -n "$bdir" ] || bdir=/opt/holzcloud/backups/$ts
  man=no
  if [ -n "$ts" ] && dx test -f "$bdir/MANIFEST"; then man=yes; fi
  R_DIR=$bdir
  R_BS=$(awk '$1 == "WALL" { printf "%.1f", ($3 - $2) / 1000 }' "$w/backup.txt")
  rsync_st=$(sed -n -E 's/.*rsync (error|warning): .*\(code ([0-9]+)\).*/\2/p' "$w/backup.txt" | head -n 1)
  R_RSYNC=${rsync_st:--}
  # How often this night repeated an rsync that ended with status 24 (the warning line
  # of backup.sh, contract C-2 of plan 29-01), and how many directories below the backup
  # root have no MANIFEST now that the night is over. Both are numbers.
  R_RETRIES=$(grep -c 'WARNING: rsync exited 24' "$w/backup.txt" 2> /dev/null || true)
  case "$R_RETRIES" in '' | *[!0-9]*) R_RETRIES=0 ;; esac
  if R_LEFT=$(dxo bash -c 'find "$1" -mindepth 1 -maxdepth 1 -type d ! -exec test -f {}/MANIFEST \; -print | wc -l' _ /opt/holzcloud/backups); then
    R_LEFT=$(printf '%s' "$R_LEFT" | tr -d ' \r\n')
  else
    R_LEFT=""
  fi
  case "$R_LEFT" in
    '' | *[!0-9]*)
      R_LEFT=0
      check_bad "run $stage $n: the directories without a MANIFEST below /opt/holzcloud/backups could not be counted"
      ;;
  esac
  if [ "$R_LEFT" -gt 0 ]; then
    check_bad "run $stage $n: $R_LEFT director(ies) without a MANIFEST below /opt/holzcloud/backups after the night (backup.sh removes the directory of a night that fails before its MANIFEST)"
  fi

  # (3) The window: events of the churn log held against the backup's own lines.
  span=$(step_span "$w/backup.txt" "Backing up database...") || span=""
  db0=${span% *}; db1=${span#* }
  span=$(step_span "$w/backup.txt" "Backing up media...") || span=""
  me0=${span% *}; me1=${span#* }
  if [ -z "$span" ] && [ "$brc" -ne 0 ]; then
    # backup.sh died inside the media step (set -e): the step ended with the script.
    me0=$(awk '{ ts = $1; $1 = ""; sub(/^ /, ""); if ($0 == "Backing up media...") print ts }' "$w/backup.txt" | head -n 1)
    me1=$(awk '$1 == "WALL" { print $3 }' "$w/backup.txt")
    [ -n "$me0" ] || me1=""
  fi
  R_U=$(churn_count "$w/churn.txt" upload)
  R_P=$(churn_count "$w/churn.txt" page)
  R_D=$(churn_count "$w/churn.txt" delete)
  if [ -n "$db1" ] && [ -n "$me1" ]; then
    R_WU=$(awk '$3 == "upload" && $4 == 303 { print $2, $1 }' "$w/churn.txt" | window_hits "$db1" "$me1" | awk '{ print $2 }')
    R_WP=$(awk '$3 == "page" && $4 == 303 { print $2, $1 }' "$w/churn.txt" | window_hits "$db1" "$me1" | awk '{ print $2 }')
    R_WD=$(awk '$3 == "delete" && $4 == 303 { print $2, $1 }' "$w/churn.txt" | window_hits "$db1" "$me1" | awk '{ print $2 }')
    { echo "DB-STEP-MS: $db0 $db1"; echo "MEDIA-STEP-MS: $me0 $me1"; } > "$w/steps.txt"
  fi

  # (4) Refused: backup.sh ended non-zero with no MANIFEST. There is nothing to compare.
  if [ "$brc" -ne 0 ] && [ "$man" = no ]; then
    R_CLASS=$(run_class "$brc" "$man" - -) || R_CLASS=?
    R_RESTORED=no
    if [ -z "${P25_HOLD:-}" ]; then consist_name_run "$stage" "$n" "$w" "$db0" "$db1" "$me1"; fi
    consist_finish "$stage" "$n" "$w" "$pfx"
    return 0
  fi
  if [ "$brc" -ne 0 ]; then
    check_bad "run $stage $n: backup.sh ended with status $brc although the MANIFEST exists ($bdir)"
  fi
  if [ -z "$dir" ] || [ "$man" != yes ]; then
    check_bad "run $stage $n: backup.sh ended 0 and named no '=== Backup complete: DIR ===' line or wrote no MANIFEST ($bdir)"
    cp "$w/backup.txt" "$OUT_DIR/$pfx-backup.txt" 2> /dev/null || true
    return 1
  fi

  # (5) Ship the directory to the second container and restore it there, or compare in place.
  if [ "$restore" = yes ]; then
    dxt bash -c 'rm -rf /opt/holzcloud/data.before-restore-* /opt/holzcloud/backups/*' || true
    # The shipped unit has StartLimitBurst=5 within StartLimitIntervalSec=300 and
    # restore.sh stops and starts it, so the sixth restore within five minutes ends
    # in "start-limit-hit" (measured: amd64-t25-trace-startlimit). That is the
    # unit doing what it says, not a finding about the pair; the counter is cleared
    # between runs (DEVIATION[start-limit]) and nothing is changed in the unit.
    dxt systemctl reset-failed holzcloud || true
    if ! docker cp "$NAME:$dir" - | docker cp - "$TGT_NAME:/opt/holzcloud/backups"; then
      check_bad "run $stage $n: the tar stream of $dir to $TGT_NAME failed"
      return 1
    fi
    tdir=/opt/holzcloud/backups/$(basename "$dir")
    wr=$(printf '%s' "$RESTORE_LINE_SAMPLE" | sed "s|/opt/holzcloud/backups/[0-9-]*\$|$tdir|")
    printf 'yes\n' | docker exec -i "$TGT_NAME" bash -c "$P25_STAMP_SH" _ "$wr" > "$w/restore.txt" 2>&1 || true
    rrc=$(sed -n 's/^RC //p' "$w/restore.txt" | tail -n 1)
    if [ -z "$rrc" ]; then
      check_bad "run $stage $n: restore.txt holds no RC line; the transcript ends: $(tail -n 3 "$w/restore.txt" | tr '\n' '|')"
      cp "$w/restore.txt" "$OUT_DIR/$pfx-restore.txt" 2> /dev/null || true
      return 1
    fi
    R_RRC=$rrc
    R_RS=$(awk '$1 == "WALL" { printf "%d", ($3 - $2) / 1000 }' "$w/restore.txt")
    if [ "$rrc" -ne 0 ]; then
      R_CLASS=$(run_class "$brc" "$man" "$rrc" -) || R_CLASS=?
      R_RESTORED=failed
      check_bad "run $stage $n: restore.sh ended with status $rrc on a complete backup; the last lines: $(tail -n 4 "$w/restore.txt" | tr '\n' '|')"
      consist_finish "$stage" "$n" "$w" "$pfx"
      return 0
    fi
    printf '%s\n' "$P25_COMPARE_SH" | docker exec -i "$TGT_NAME" bash -s -- restored /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "$tdir" /opt/holzcloud/data > "$w/cmp.out" 2> "$w/cmp.err" || true
  else
    printf '%s\n' "$P25_COMPARE_SH" | dxi bash -s -- backup "$dir/holzcloud.sqlite" "$dir/media" "$dir" "" > "$w/cmp.out" 2> "$w/cmp.err" || true
  fi
  split_sections "$w/cmp.out" "$w"
  if [ ! -s "$w/ROWS" ] || ! grep -q . "$w/FILES" 2> /dev/null; then
    check_bad "run $stage $n: the comparator returned no rows or no files (cmp.err: $(head -c 300 "$w/cmp.err" | tr '\n' '|'))"
    cp "$w/cmp.out" "$OUT_DIR/$pfx-compare-raw.txt" 2> /dev/null || true
    return 1
  fi

  # (6) The comparison, both directions, and everything else asserted of the site.
  res=$(media_agree "$w/ROWS" "$w/FILES" '*/bulk/' "$w/MISSING" "$w/ORPHAN") || res=""
  if [ -z "$res" ]; then
    check_bad "run $stage $n: media_agree failed on the lists of the comparator"
    return 1
  fi
  set -- $res
  rows=$1; files=$2; miss=$3; orph=$4; agree=$5
  R_MISSING=$miss
  R_ORPHAN=$orph
  R_CLASS=$(run_class "$brc" "$man" "$rrc" "$agree") || R_CLASS=?
  if [ "$R_CLASS" != AGREE ] && [ -z "${P25_HOLD:-}" ]; then consist_name_run "$stage" "$n" "$w" "$db0" "$db1" "$me1"; fi
  {
    echo "RUN: $stage $n restored=$restore"
    echo "BACKUP-DIR: $bdir"
    echo "ROWS: $rows"
    echo "FILES: $files"
    echo "MISSING: $miss"
    echo "ORPHAN: $orph"
    echo "EXCLUDED: */bulk/ (the fixture directory of run.sh backup-fixture has files by design and no rows)"
    echo "CLASS: $R_CLASS"
    cat "$w/names.txt" 2> /dev/null || true
  } > "$w/compare.txt"
  CHECK_OUT=$w/compare.txt
  nbad0=$BADS
  if [ "$R_CLASS" = AGREE ]; then
    check_pass "media and database agree in both directions ($rows rows, $files files)"
  else
    check_info "disagreement: $miss rows without a file, $orph files without a row (class $R_CLASS)"
    if [ "$miss" -gt 0 ]; then
      sed 's/^/KEY MISSING-FILE: /' "$w/MISSING" | head -n 20 | tee -a "$CHECK_OUT" > /dev/null
    fi
    if [ "$orph" -gt 0 ]; then
      sed 's/^/KEY ORPHAN-FILE: /' "$w/ORPHAN" | head -n 20 | tee -a "$CHECK_OUT" > /dev/null
    fi
  fi
  if [ "$(sed -n 's/^CHECK-RC: //p' "$w/CHECK")" = 0 ]; then
    check_pass "holzcloud check on a copy: $(sed -n 's/^CHECK-OUT: //p' "$w/CHECK" | cut -c1-120)"
  else
    check_bad "holzcloud check on a copy ended non-zero: $(sed -n 's/^CHECK-OUT: //p' "$w/CHECK" | cut -c1-200)"
  fi
  if [ "$(sed -n 's/^INTEGRITY: //p' "$w/CHECK" | tr -d ' ')" = ok ]; then
    check_pass "PRAGMA integrity_check: ok"
  else
    check_bad "PRAGMA integrity_check: $(sed -n 's/^INTEGRITY: //p' "$w/CHECK" | cut -c1-200)"
  fi
  if [ "$(sed -n 's/^FKCHECK: //p' "$w/CHECK")" = 0 ]; then
    check_pass "PRAGMA foreign_key_check: no violation"
  else
    check_bad "PRAGMA foreign_key_check: $(sed -n 's/^FKCHECK: //p' "$w/CHECK") violation(s)"
  fi
  : > "$w/ROWS.all"
  LC_ALL=C sort -u "$w/ROWS" > "$w/ROWS.all"
  LC_ALL=C sort -u "$w/FILES" > "$w/FILES.all"
  ref_count=$(grep -c . "$w/REFS" || true)
  dang=$({ LC_ALL=C comm -23 "$w/REFS" "$w/ROWS.all"; LC_ALL=C comm -23 "$w/REFS" "$w/FILES.all"; } | LC_ALL=C sort -u | grep -c . || true)
  check_info "PAGE-REFS: $ref_count media files named by page HTML"
  if [ "${dang:-0}" -eq 0 ]; then
    check_pass "PAGE-REFS-DANGLING: 0"
  else
    check_bad "PAGE-REFS-DANGLING: $dang media file(s) named by a page have no row or no file"
  fi
  if [ "$restore" = yes ]; then
    tr_a=$(grep -c . "$w/TREE-RESTORED" || true)
    tr_b=$(grep -c . "$w/TREE-BACKUP" || true)
    if [ "$tr_a" -gt 0 ] && cmp -s "$w/TREE-RESTORED" "$w/TREE-BACKUP"; then
      check_pass "the sha256 list of the restored tree ($tr_a files) equals that of the backup directory it came from"
    else
      check_bad "the sha256 list of the restored tree ($tr_a files) differs from that of the backup directory ($tr_b files)"
      diff "$w/TREE-RESTORED" "$w/TREE-BACKUP" | head -n 10 | sed 's/^/KEY TREE-DIFF: /' | tee -a "$CHECK_OUT" > /dev/null || true
    fi
  fi
  CHECK_OUT=$keep_out
  if [ "$BADS" -gt "$nbad0" ]; then
    R_BADRUN=yes
    grep '^BAD' "$w/compare.txt" | while IFS= read -r line; do printf 'BAD  run %s %s: %s\n' "$stage" "$n" "${line#BAD  }" | tee -a "$CHECK_OUT"; done
  fi
  consist_finish "$stage" "$n" "$w" "$pfx"
  return 0
}

# consist_finish STAGE N W PFX: the line of consist-runs.txt, the raw files of a run
# that is kept (run 1 and every run that is not AGREE; every run with
# CONSIST_KEEP_RAW=1), and the cleanup of the backup directories of the others.
consist_finish() {
  local stage=$1 n=$2 w=$3 pfx=$4 keep=no f
  printf 'RUN %s %s CLASS=%s BACKUP-RC=%s RESTORE-RC=%s MISSING=%s ORPHAN=%s WINDOW-UPLOADS=%s WINDOW-PAGES=%s WINDOW-DELETES=%s UPLOADS=%s PAGES=%s DELETES=%s BACKUP-S=%s RESTORED=%s NAMED=%s WINDOW-NAME=%s RSYNC-STATUS=%s RETRIES=%s LEFT=%s DIR=%s\n' \
    "$n" "$stage" "$R_CLASS" "$R_BRC" "$R_RRC" "$R_MISSING" "$R_ORPHAN" "$R_WU" "$R_WP" "$R_WD" "$R_U" "$R_P" "$R_D" "$R_BS" "$R_RESTORED" "$R_NAMED" "$R_WNAME" "$R_RSYNC" "$R_RETRIES" "$R_LEFT" "$R_DIR" >> "${P25_RUNS_FILE:-$OUT_DIR/consist-runs.txt}"
  if [ -s "$w/names.txt" ]; then cat "$w/names.txt" >> "$OUT_DIR/consist-names.txt"; fi
  if [ "$n" = 1 ] || [ "$R_CLASS" != AGREE ] || [ "$R_BADRUN" = yes ]; then keep=yes; fi
  if [ -n "${P25_HOLD:-}" ]; then keep=yes; fi
  if [ "$keep" = yes ] || [ "${CONSIST_KEEP_RAW:-0}" = 1 ]; then
    for f in backup restore compare churn steps names; do
      if [ -f "$w/$f.txt" ]; then cp "$w/$f.txt" "$OUT_DIR/$pfx-$f.txt"; fi
    done
  fi
  if [ "$keep" != yes ]; then
    if [ -n "$R_DIR" ] && [ "$R_DIR" != "?" ]; then dx rm -rf "$R_DIR" || true; fi
  fi
  rm -rf "${w:?}"
}

# comparator_selftest ROWS FILES: the comparison can fail. A copy of the genuine
# lists is changed four ways and each must be detected, in the right direction: a
# row removed and a file added are files without a row (ORPHAN-ONLY), a row added
# and a file removed are rows without a file (ROW-WITHOUT-FILE).
comparator_selftest() {
  local rows=$1 files=$2 t r
  t=$(mktemp -d) || return 1
  r=$(media_agree "$rows" "$files" '*/bulk/')
  check_info "COMPARATOR-SELFTEST genuine lists: $r"
  set -- $r
  if [ "$5" != AGREE ]; then
    check_bad "the comparator's own lists do not agree before they are tampered with ($r)"
    rm -rf "${t:?}"
    return 1
  fi
  # a row removed
  media_filter "$rows" '*/bulk/' | sed '1d' > "$t/rows1"
  r=$(media_agree "$t/rows1" "$files" '*/bulk/')
  if [ "${r##* }" = ORPHAN-ONLY ]; then check_pass "comparator selftest: a row removed is detected as a file without a row ($r)"; else check_bad "comparator selftest: a row removed gave '$r', expected ORPHAN-ONLY"; fi
  # a file added
  { cat "$files"; echo "1/tampered-extra-file.jpg"; } > "$t/files1"
  r=$(media_agree "$rows" "$t/files1" '*/bulk/')
  if [ "${r##* }" = ORPHAN-ONLY ]; then check_pass "comparator selftest: a file added is detected as a file without a row ($r)"; else check_bad "comparator selftest: a file added gave '$r', expected ORPHAN-ONLY"; fi
  # a row added
  { cat "$rows"; echo "1/tampered-extra-row.jpg"; } > "$t/rows2"
  r=$(media_agree "$t/rows2" "$files" '*/bulk/')
  if [ "${r##* }" = ROW-WITHOUT-FILE ]; then check_pass "comparator selftest: a row added is detected as a row without a file ($r)"; else check_bad "comparator selftest: a row added gave '$r', expected ROW-WITHOUT-FILE"; fi
  # a file removed
  media_filter "$files" '*/bulk/' | sed '1d' > "$t/files2"
  r=$(media_agree "$rows" "$t/files2" '*/bulk/')
  if [ "${r##* }" = ROW-WITHOUT-FILE ]; then check_pass "comparator selftest: a file removed is detected as a row without a file ($r)"; else check_bad "comparator selftest: a file removed gave '$r', expected ROW-WITHOUT-FILE"; fi
  rm -rf "${t:?}"
}

cmd_consist() {
  local o=$OUT_DIR/consist.txt ro=$OUT_DIR/consist-runs.txt stage n restore line ver t0 t1 first_s tl
  local nclass_a nclass_o nclass_m nclass_r bl rl last nst why pick pe_every pe_status pe_secs
  need_container
  dxt true > /dev/null 2>&1 \
    || die "container $TGT_NAME does not answer; bring the second host up with: bash deploy/test/nightly.sh target (TGT_NAME=$TGT_NAME)"
  [ "$(tail -n 1 "$OUT_DIR/prep.txt" 2> /dev/null)" = "PREP OK" ] \
    || die "$OUT_DIR/prep.txt does not end with PREP OK; run 'bash deploy/test/nightly.sh prep' first"
  case "$CONSIST_RUNS" in "" | *[!0-9]* | 0) die "CONSIST_RUNS '$CONSIST_RUNS' is not a positive number" ;; esac
  case "$CONSIST_MIN_RUNS" in "" | *[!0-9]* | 0) die "CONSIST_MIN_RUNS '$CONSIST_MIN_RUNS' is not a positive number" ;; esac
  case "$CONSIST_RESTORE_EVERY" in "" | *[!0-9]* | 0) die "CONSIST_RESTORE_EVERY '$CONSIST_RESTORE_EVERY' is not a positive number" ;; esac
  nst=0
  for stage in $CONSIST_STAGES; do
    case "$stage" in adds | deletes) nst=$((nst + 1)) ;; *) die "CONSIST_STAGES holds '$stage', expected adds or deletes" ;; esac
  done
  [ "$nst" -gt 0 ] || die "CONSIST_STAGES is empty"
  why=$(restore_every_ok "$CONSIST_RUNS" "$CONSIST_MIN_RUNS" "$CONSIST_RESTORE_EVERY") \
    || die "CONSIST_RESTORE_EVERY=$CONSIST_RESTORE_EVERY: $why. A verdict rests on restored runs: use CONSIST_RESTORE_EVERY=$(restore_every_max "$CONSIST_RUNS" "$CONSIST_MIN_RUNS") or less"
  mkdir -p "$OUT_DIR"
  BADS=0
  res_begin "$o"
  : > "$ro"
  : > "$OUT_DIR/consist-names.txt"
  p25_ship
  WID=$(p25_wid)
  BASEREF=$(p25_baseref)
  [ -n "$WID" ] && [ -n "$BASEREF" ] || die "no website called Test or no media file in $NAME"
  bl=$(doc_backup_line) || die "deploy/DEPLOY.md has no line 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups' (the document and the script have drifted apart)"
  rl=$(doc_restore_line) || die "deploy/DEPLOY.md has no restore line 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  BACKUP_LINE=${bl#*:}
  RESTORE_LINE_SAMPLE=${rl#*:}
  key_line "MIN-RUNS: $CONSIST_MIN_RUNS"
  key_line "CONSIST-RUNS: $CONSIST_RUNS"
  key_line "CONSIST-STAGES: $CONSIST_STAGES"
  key_line "CONSIST-RESTORE-EVERY: $CONSIST_RESTORE_EVERY"
  key_line "RESTORED-RUNS-FLOOR: $(restore_required "$CONSIST_RUNS" "$CONSIST_MIN_RUNS") per stage (at least half of MIN-RUNS and never fewer than 10, never more than the stage has runs)"
  key_line "CHURN-WORKERS: $CHURN_WORKERS uploaders, one page saver, in stage deletes one deleter"
  key_line "CHURN-PAUSE-S: $CHURN_PAUSE"
  key_line "BACKUP-LINE: deploy/DEPLOY.md:${bl%%:*}: $BACKUP_LINE"
  key_line "RESTORE-LINE: deploy/DEPLOY.md:${rl%%:*}: $RESTORE_LINE_SAMPLE (the sample timestamp is replaced by the directory of the run; yes is typed on stdin)"
  key_line "WEBSITE: $WID"
  key_line "BASE-MEDIA: $BASEREF"
  ver=$(dxo /opt/holzcloud/holzcloud version) || ver="?"
  key_line "BINARY: $ver"
  if [ -f "$OUT_DIR/bulk.txt" ]; then
    key_line "BULK: $(tr '\n' ' ' < "$OUT_DIR/bulk.txt")"
  else
    key_line "BULK: none (no $OUT_DIR/bulk.txt: BULK_MB was not given to source)"
  fi
  key_line "HOST-ARCH: $(docker info --format '{{.Architecture}}')"
  key_line "CONTAINER-ARCH: $(dxo uname -m)"
  key_line "START-LIMIT: $(dxto systemctl show holzcloud -p StartLimitIntervalUSec -p StartLimitBurst | tr '\n' ' ')(shipped unit, read from $TGT_NAME)"
  deviation "DEVIATION[start-limit]: the shipped holzcloud.service allows 5 starts within 300 s and restore.sh stops and starts it, so a sixth restore within five minutes on one host ends in 'start-limit-hit' (measured, amd64-t25-trace-startlimit). Runs follow each other faster than that, so 'systemctl reset-failed holzcloud' is run on the second container before every restore; the unit is not changed and no sandbox directive is touched"

  # The comparison can fail: proven on the genuine lists of the quiet source first.
  printf '%s\n' "$P25_COMPARE_SH" | dxi bash -s -- live /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "" "" > "$OUT_DIR/.cmp-live.out" 2> /dev/null || true
  mkdir -p "$OUT_DIR/.cmp-live"
  split_sections "$OUT_DIR/.cmp-live.out" "$OUT_DIR/.cmp-live"
  comparator_selftest "$OUT_DIR/.cmp-live/ROWS" "$OUT_DIR/.cmp-live/FILES" || true
  rm -rf "$OUT_DIR/.cmp-live" "$OUT_DIR/.cmp-live.out"

  t0=$SECONDS
  for stage in $CONSIST_STAGES; do
    case "$stage" in adds | deletes) ;; *) die "CONSIST_STAGES holds '$stage', expected adds or deletes" ;; esac
    n=1
    while [ "$n" -le "$CONSIST_RUNS" ]; do
      restore=no
      if [ $(((n - 1) % CONSIST_RESTORE_EVERY)) -eq 0 ]; then restore=yes; fi
      t1=$SECONDS
      if consist_run "$stage" "$n" "$restore"; then
        say "consist: run $stage $n: $(tail -n 1 "$ro" | cut -d' ' -f4-8) in $((SECONDS - t1)) s"
      else
        say "consist: run $stage $n: the harness failed (BAD line above)"
        printf 'RUN %s %s CLASS=HARNESS-FAILED BACKUP-RC=%s RESTORE-RC=%s\n' "$n" "$stage" "${R_BRC:-?}" "${R_RRC:--}" >> "$ro"
      fi
      if [ "$stage" = "${CONSIST_STAGES%% *}" ] && [ "$n" = 1 ]; then
        first_s=$((SECONDS - t1))
        key_line "RUN-1-SECONDS: $first_s"
        key_line "RUN-1-RESTORE-SECONDS: ${R_RS:-0}"
        # 90 minutes is the budget for all runs; a longer projection raises
        # CONSIST_RESTORE_EVERY (never lowers N), within what the restore floor allows.
        pick=$(consist_pick_every "$CONSIST_RUNS" "$CONSIST_MIN_RUNS" "$nst" "$first_s" "${R_RS:-0}" "$CONSIST_RESTORE_EVERY") || pick="$CONSIST_RESTORE_EVERY ok 0"
        read -r pe_every pe_status pe_secs <<< "$pick"
        key_line "PROJECTED-MINUTES: $((pe_secs / 60)) at CONSIST_RESTORE_EVERY=$pe_every ($nst stage(s) of $CONSIST_RUNS runs)"
        if [ "$pe_every" != "$CONSIST_RESTORE_EVERY" ]; then
          deviation "DEVIATION[restore-every]: the first run took $first_s s, so $nst stage(s) of $CONSIST_RUNS runs would exceed 90 minutes; CONSIST_RESTORE_EVERY is raised from $CONSIST_RESTORE_EVERY to $pe_every (the number of runs is not lowered, the floor of restored runs still holds)"
          key_line "RESTORE-EVERY-RAISED: $CONSIST_RESTORE_EVERY -> $pe_every"
          CONSIST_RESTORE_EVERY=$pe_every
        fi
        if [ "$pe_status" = over ]; then
          key_line "PROJECTION-OVER-90-MIN: even with CONSIST_RESTORE_EVERY=$pe_every the projection is $((pe_secs / 60)) minutes; the runs are not reduced"
        fi
      fi
      n=$((n + 1))
    done
  done
  key_line "WALL-MINUTES: $(((SECONDS - t0) / 60))"
  dx bash -c 'rm -f /root/.hc-jar-churn-*' || true
  while IFS= read -r line; do key_line "$line"; done < "$OUT_DIR/consist-names.txt"
  for stage in $CONSIST_STAGES; do
    nclass_a=$(grep -c "^RUN [0-9]* $stage CLASS=AGREE " "$ro" || true)
    nclass_o=$(grep -c "^RUN [0-9]* $stage CLASS=ORPHAN-ONLY " "$ro" || true)
    nclass_m=$(grep -c "^RUN [0-9]* $stage CLASS=ROW-WITHOUT-FILE " "$ro" || true)
    nclass_r=$(grep -c "^RUN [0-9]* $stage CLASS=BACKUP-REFUSED " "$ro" || true)
    key_line "CLASS-COUNTS-STAGE-$stage: AGREE=$nclass_a ORPHAN-ONLY=$nclass_o ROW-WITHOUT-FILE=$nclass_m BACKUP-REFUSED=$nclass_r"
    consist_stage_report "$stage" "$ro" "$CONSIST_MIN_RUNS"
  done
  leak_check
  last=CONSIST-OK
  if [ "$BADS" -gt 0 ]; then last=CONSIST-FAILED; fi
  if [ "$last" = CONSIST-FAILED ]; then
    key_line "CONSIST FAILED"
    say "nightly.sh: CONSIST FAILED, $BADS BAD line(s); see $o"
    exit 1
  fi
  key_line "CONSIST OK"
}


selftest_outcome() {
  local t
  t=$(mktemp -d) || return 1
  # window_name KIND START END DB_START DB_END MEDIA_END; database step 1000..1300, media step ends 2000.
  st_eq "window_name before the database step" before "$(window_name upload 100 900 1000 1300 2000)"
  st_eq "window_name ended exactly at the start of the database step" before "$(window_name upload 100 1000 1000 1300 2000)"
  st_eq "window_name ended one millisecond into the database step" overlaps-snapshot-step "$(window_name upload 100 1001 1000 1300 2000)"
  st_eq "window_name inside the database step" overlaps-snapshot-step "$(window_name delete 1100 1200 1000 1300 2000)"
  st_eq "window_name began before and ended after the whole database step" overlaps-snapshot-step "$(window_name delete 900 1400 1000 1300 2000)"
  st_eq "window_name began inside the database step and ended in the window" overlaps-snapshot-step "$(window_name upload 1299 1500 1000 1300 2000)"
  st_eq "window_name began exactly at the end of the database step" after-snapshot "$(window_name upload 1300 1500 1000 1300 2000)"
  st_eq "window_name ended one millisecond before the end of the media step" after-snapshot "$(window_name upload 1500 1999 1000 1300 2000)"
  st_eq "window_name ended exactly at the end of the media step" straddles-media-end "$(window_name delete 1500 2000 1000 1300 2000)"
  st_eq "window_name ended after the media step" straddles-media-end "$(window_name page 1900 2500 1000 1300 2000)"
  st_eq "window_name began exactly at the end of the media step" after "$(window_name upload 2000 2100 1000 1300 2000)"
  st_eq "window_name long after" after "$(window_name upload 5000 5100 1000 1300 2000)"
  st_fail "window_name unknown kind" window_name copy 1500 1600 1000 1300 2000
  st_fail "window_name a non-number" window_name upload x 1600 1000 1300 2000
  st_fail "window_name an event that ends before it starts" window_name upload 1600 1500 1000 1300 2000
  st_fail "window_name steps out of order" window_name upload 1500 1600 1300 1000 2000
  st_fail "window_name media step ends before the database step" window_name upload 1500 1600 1000 1300 1200
  st_eq "window_name prints nothing when it refuses" "" "$(window_name copy 1500 1600 1000 1300 2000 || true)"

  # exposure_ok N MIN_RUNS EXPOSED
  st_ok "exposure 40 runs, 20 exposed" exposure_ok 40 30 20
  st_fail "exposure 40 runs, 19 exposed" exposure_ok 40 30 19
  st_ok "exposure 40 runs, 40 exposed" exposure_ok 40 30 40
  st_fail "exposure 29 runs with a floor of 30, all exposed" exposure_ok 29 30 29
  st_ok "exposure 30 runs with a floor of 30, 15 exposed" exposure_ok 30 30 15
  st_fail "exposure 30 runs, 14 exposed" exposure_ok 30 30 14
  st_ok "exposure a trace of one run with a floor of one" exposure_ok 1 1 1
  st_fail "exposure one run against the default floor of 30" exposure_ok 1 30 1
  st_fail "exposure one run, none exposed" exposure_ok 1 1 0
  st_ok "exposure 3 runs, 2 exposed (half rounds up)" exposure_ok 3 1 2
  st_fail "exposure 3 runs, 1 exposed" exposure_ok 3 1 1
  st_fail "exposure no runs" exposure_ok 0 0 0
  st_fail "exposure a non-number" exposure_ok x 30 5
  st_eq "exposure_ok says ok" ok "$(exposure_ok 40 30 20)"

  # rule_of_three N: 3/N as a percent, one decimal
  st_eq "rule_of_three 40" "7.5" "$(rule_of_three 40)"
  st_eq "rule_of_three 30" "10.0" "$(rule_of_three 30)"
  st_eq "rule_of_three 1" "300.0" "$(rule_of_three 1)"
  st_eq "rule_of_three 7 (rounds half up)" "42.9" "$(rule_of_three 7)"
  st_eq "rule_of_three 80" "3.8" "$(rule_of_three 80)"
  st_eq "rule_of_three 300" "1.0" "$(rule_of_three 300)"
  st_fail "rule_of_three 0" rule_of_three 0
  st_fail "rule_of_three x" rule_of_three x

  # con01_outcome AGREE ORPHAN-ONLY ROW-WITHOUT-FILE REFUSED RESTORE-FAILED N MIN_RUNS EXPOSED NAMED
  st_eq "outcome all agree, exposure ok" AGREE-ALL "$(con01_outcome 40 0 0 0 0 40 30 40 0)"
  st_eq "outcome all agree, 19 of 40 exposed" INCONCLUSIVE "$(con01_outcome 40 0 0 0 0 40 30 19 0)"
  st_eq "outcome all agree, 29 runs against a floor of 30" INCONCLUSIVE "$(con01_outcome 29 0 0 0 0 29 30 29 0)"
  st_eq "outcome orphans only, all named" ORPHAN-ONLY-NAMED "$(con01_outcome 0 40 0 0 0 40 30 40 40)"
  st_eq "outcome some orphans, the rest agree" ORPHAN-ONLY-NAMED "$(con01_outcome 30 10 0 0 0 40 30 40 10)"
  st_eq "outcome a row without a file, every window named" ROW-WITHOUT-FILE-NAMED "$(con01_outcome 20 10 10 0 0 40 30 40 20)"
  st_eq "outcome a row without a file, one window not named" UNNAMED "$(con01_outcome 20 10 10 0 0 40 30 40 19)"
  st_eq "outcome an orphan whose window is not named" UNNAMED "$(con01_outcome 39 1 0 0 0 40 30 40 0)"
  st_eq "outcome unnamed beats inconclusive" UNNAMED "$(con01_outcome 30 10 0 0 0 40 30 5 9)"
  st_eq "outcome a refused backup named by its rsync status" REFUSED-NAMED "$(con01_outcome 39 0 0 1 0 40 30 40 1)"
  st_eq "outcome a refused backup without a name" UNNAMED "$(con01_outcome 39 0 0 1 0 40 30 40 0)"
  st_eq "outcome refused beside orphans reports the orphans" ORPHAN-ONLY-NAMED "$(con01_outcome 30 8 0 2 0 40 30 40 10)"
  st_eq "outcome a failed restore" RESTORE-FAILED "$(con01_outcome 39 0 0 0 1 40 30 40 0)"
  st_eq "outcome counts that do not add up" INCOMPLETE "$(con01_outcome 35 0 0 0 0 40 30 40 0)"
  st_eq "outcome a trace of one run, exposed" AGREE-ALL "$(con01_outcome 1 0 0 0 0 1 1 1 0)"
  st_eq "outcome a trace of one run, not exposed" INCONCLUSIVE "$(con01_outcome 1 0 0 0 0 1 1 0 0)"
  st_eq "outcome a trace can never be the evidence set" INCONCLUSIVE "$(con01_outcome 1 0 0 0 0 1 30 1 0)"
  st_fail "outcome a non-number" con01_outcome x 0 0 0 0 1 1 1 0

  # restore_required N MIN_RUNS and the floor of restored runs
  st_eq "restore_required 40 runs, floor 30" 15 "$(restore_required 40 30)"
  st_eq "restore_required 40 runs, floor 10" 10 "$(restore_required 40 10)"
  st_eq "restore_required never fewer than 10" 10 "$(restore_required 40 1)"
  st_eq "restore_required a trace of 6 runs: every run" 6 "$(restore_required 6 1)"
  st_eq "restore_required 100 runs, floor 100" 50 "$(restore_required 100 100)"
  st_eq "restore_required a floor of 31 rounds up" 16 "$(restore_required 40 31)"
  st_ok "restore floor 40 of 40" restore_floor_ok 40 40 30
  st_ok "restore floor exactly 15" restore_floor_ok 40 15 30
  st_fail "restore floor 14 of 40" restore_floor_ok 40 14 30
  st_fail "restore floor 9 of 40 with a floor of 1" restore_floor_ok 40 9 1
  st_ok "restore floor 6 of 6" restore_floor_ok 6 6 1
  st_fail "restore floor 5 of 6" restore_floor_ok 6 5 1
  st_ok "restore floor 5 of 6 when one was refused (a trace)" restore_floor_ok 6 5 1 1
  st_fail "restore floor 4 of 6 when one was refused (a trace)" restore_floor_ok 6 4 1 1
  st_fail "restore floor none restored, all refused (a trace)" restore_floor_ok 6 0 1 6
  st_fail "restore floor 14 of 40 with 5 refused (an evidence set keeps its floor)" restore_floor_ok 40 14 30 5
  st_ok "restore floor 15 of 40 with 5 refused" restore_floor_ok 40 15 30 5
  st_ok "restore_every 1" restore_every_ok 40 30 1
  st_ok "restore_every 2 restores 20" restore_every_ok 40 30 2
  st_ok "restore_every 2 with an odd count restores 21" restore_every_ok 41 30 2
  st_fail "restore_every 3 restores 14" restore_every_ok 40 30 3
  st_fail "restore_every 4 restores 10" restore_every_ok 40 30 4
  st_eq "restore_every_ok says why" "only 14 of 40 runs would be restored and compared after the restore, the floor is 15" "$(restore_every_ok 40 30 3 || true)"
  st_eq "restore_every_max 40 runs floor 30" 2 "$(restore_every_max 40 30)"
  st_eq "restore_every_max a trace" 1 "$(restore_every_max 6 1)"
  st_eq "restore_every_max 80 runs floor 30" 5 "$(restore_every_max 80 30)"

  # consist_stage_counts: N AGREE ORPHAN ROWMISS REFUSED RESTFAIL EXPOSED NAMED RESTORED UPIN PGIN DELIN
  printf '%s\n' \
    'RUN 1 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=- DIR=/x' \
    'RUN 2 adds CLASS=ORPHAN-ONLY BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=3 WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=yes RSYNC-STATUS=- DIR=/x' \
    'RUN 3 adds CLASS=ROW-WITHOUT-FILE BACKUP-RC=0 RESTORE-RC=- MISSING=1 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=1 WINDOW-DELETES=2 UPLOADS=9 PAGES=3 DELETES=5 BACKUP-S=1.3 RESTORED=no NAMED=yes RSYNC-STATUS=- DIR=/x' \
    'RUN 4 adds CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=? WINDOW-PAGES=? WINDOW-DELETES=? UPLOADS=9 PAGES=3 DELETES=5 BACKUP-S=1.0 RESTORED=yes NAMED=yes RSYNC-STATUS=24 DIR=/x' \
    'RUN 5 adds CLASS=RESTORE-FAILED BACKUP-RC=0 RESTORE-RC=1 MISSING=? ORPHAN=? WINDOW-UPLOADS=1 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=- DIR=/x' \
    'RUN 1 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=7 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=- DIR=/x' \
    > "$t/runs.txt"
  st_eq "counts of stage adds" "5 1 1 1 1 1 3 3 2 4 2 2" "$(consist_stage_counts "$t/runs.txt" adds)"
  st_eq "counts of stage deletes" "1 1 0 0 0 0 1 0 1 7 0 0" "$(consist_stage_counts "$t/runs.txt" deletes)"
  st_eq "counts of a stage that has no runs" "0 0 0 0 0 0 0 0 0 0 0 0" "$(consist_stage_counts "$t/runs.txt" nothing)"
  printf '%s\n' \
    'RUN 1 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=2 WINDOW-PAGES=1 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=- RETRIES=0 LEFT=0 DIR=/x' \
    'RUN 2 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=2.1 RESTORED=no NAMED=- RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/x' \
    'RUN 3 adds CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=? WINDOW-PAGES=? WINDOW-DELETES=? UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=2.4 RESTORED=no NAMED=yes RSYNC-STATUS=24 RETRIES=1 LEFT=0 DIR=/x' \
    'RUN 4 adds CLASS=BACKUP-REFUSED BACKUP-RC=1 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=? WINDOW-PAGES=? WINDOW-DELETES=? UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.0 RESTORED=no NAMED=- RSYNC-STATUS=- RETRIES=0 LEFT=2 DIR=/x' \
    'RUN 5 adds CLASS=BACKUP-REFUSED BACKUP-RC=24 RESTORE-RC=- MISSING=? ORPHAN=? WINDOW-UPLOADS=? WINDOW-PAGES=? WINDOW-DELETES=? UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.0 RESTORED=no NAMED=yes RSYNC-STATUS=24 DIR=/x' \
    'RUN 6 adds CLASS=HARNESS-FAILED BACKUP-RC=? RESTORE-RC=-' \
    'RUN 7 adds CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=no NAMED=- RSYNC-STATUS=- RETRIES=x LEFT=y DIR=/x' \
    'RUN 1 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=0 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=24 RETRIES=1 LEFT=1 DIR=/x' \
    > "$t/retry.txt"
  st_eq "retry counts of stage adds: two repeated, one completed, two ended with 24, two directories left" "2 1 2 2" "$(consist_retry_counts "$t/retry.txt" adds)"
  st_eq "retry counts of stage deletes" "1 1 0 1" "$(consist_retry_counts "$t/retry.txt" deletes)"
  st_eq "retry counts of a stage that has no runs" "0 0 0 0" "$(consist_retry_counts "$t/retry.txt" nothing)"
  st_eq "retry counts of an older file without the tokens" "0 0 1 0" "$(consist_retry_counts "$t/runs.txt" adds)"
  st_eq "retry counts of a file without a night that repeated" "0 0 0 0" "$(consist_retry_counts "$t/runs.txt" deletes)"
  st_eq "retry counts keep the stages apart" "1 1 0 0" "$(printf '%s\n' 'RUN 1 x CLASS=AGREE BACKUP-RC=0 RETRIES=2 LEFT=0 DIR=/x' 'RUN 2 y CLASS=AGREE BACKUP-RC=0 RETRIES=1 LEFT=0 DIR=/x' > "$t/two.txt"; consist_retry_counts "$t/two.txt" x)"
  st_eq "the new RUN line leaves the twelve numbers of consist_stage_counts as they were" "$(consist_stage_counts "$t/runs.txt" deletes)" "$(printf '%s\n' 'RUN 1 deletes CLASS=AGREE BACKUP-RC=0 RESTORE-RC=0 MISSING=0 ORPHAN=0 WINDOW-UPLOADS=7 WINDOW-PAGES=0 WINDOW-DELETES=0 UPLOADS=9 PAGES=3 DELETES=0 BACKUP-S=1.3 RESTORED=yes NAMED=- RSYNC-STATUS=- RETRIES=1 LEFT=3 DIR=/x' > "$t/new.txt"; consist_stage_counts "$t/new.txt" deletes)"
  st_eq "the new RUN lines through consist_stage_counts: stage adds" "7 3 0 0 3 0 1 2 1 2 1 0" "$(consist_stage_counts "$t/retry.txt" adds)"
  st_eq "restored run numbers of stage adds" "1 2" "$(consist_restored_numbers "$t/runs.txt" adds)"
  st_eq "backup status histogram of stage adds" "0=4 24=1" "$(consist_rc_histogram "$t/runs.txt" adds)"

  # consist_pick_every N MIN_RUNS STAGES FIRST_S RESTORE_S CURRENT
  st_eq "pick_every 40 runs fit as they are" "1 ok 1680" "$(consist_pick_every 40 30 2 21 10 1)"
  st_eq "pick_every 200 runs raise the interval to 4" "4 ok 5400" "$(consist_pick_every 200 30 2 21 10 1)"
  st_eq "pick_every 400 runs do not fit at any interval" "26 over 9120" "$(consist_pick_every 400 30 2 21 10 1)"
  st_eq "pick_every never lowers a given interval" "3 ok 1160" "$(consist_pick_every 40 30 2 21 10 3)"
  st_fail "pick_every a non-number" consist_pick_every x 30 2 21 10 1
  rm -rf "${t:?}"
}
# ---------------------------------------------------------------------------
# Phase 25, plan 01, task 2: N runs in two stages, the exposure guard, the window
# named, the verdict with its rule of three. All of it is computed from the file
# consist-runs.txt, so it can be judged again offline.
# ---------------------------------------------------------------------------

# is_num V...: every argument is a non-empty string of digits.
is_num() {
  local v
  for v in "$@"; do
    case "$v" in "" | *[!0-9]*) return 1 ;; esac
  done
  return 0
}

# window_name KIND START END DB_START DB_END MEDIA_END: where an event stands against
# the two copies of one backup, all in milliseconds. One of
#   before                 it ended at or before the start of the database step
#   overlaps-snapshot-step it overlapped the database step (the snapshot instant lies
#                          somewhere inside it: the CLI process opens the database,
#                          checks migrations, runs VACUUM INTO and reopens the file,
#                          so an event that overlaps cannot be ordered against it)
#   after-snapshot         it began at or after the end of the database step and
#                          ended before the end of the media step
#   straddles-media-end    it began in the window and ended at or after the end of
#                          the media step
#   after                  it began at or after the end of the media step
# Refuses (status 1, nothing printed) a kind other than upload, page, delete, a
# non-number, an event that ends before it starts, or steps out of order.
window_name() {
  local kind=$1 s=$2 e=$3 ds=$4 de=$5 me=$6
  case "$kind" in upload | page | delete) ;; *) return 1 ;; esac
  is_num "$s" "$e" "$ds" "$de" "$me" || return 1
  [ "$s" -le "$e" ] && [ "$ds" -le "$de" ] && [ "$de" -le "$me" ] || return 1
  if [ "$e" -le "$ds" ]; then
    echo before
  elif [ "$s" -lt "$de" ]; then
    echo overlaps-snapshot-step
  elif [ "$s" -ge "$me" ]; then
    echo after
  elif [ "$e" -lt "$me" ]; then
    echo after-snapshot
  else
    echo straddles-media-end
  fi
}

# exposure_ok N MIN_RUNS EXPOSED: a stage proves something about the window only if
# it had at least MIN_RUNS runs and at least half of them (rounded up) had an upload
# in the window. Prints ok, or the reason; status 1 for the reason.
exposure_ok() {
  local n=$1 min=$2 ex=$3
  is_num "$n" "$min" "$ex" || { echo "not numbers"; return 1; }
  if [ "$n" -lt 1 ]; then
    echo "no runs"
    return 1
  fi
  if [ "$n" -lt "$min" ]; then
    echo "$n runs are fewer than the floor of $min"
    return 1
  fi
  if [ $((ex * 2)) -lt "$n" ]; then
    echo "only $ex of $n runs had an upload in the window, fewer than half"
    return 1
  fi
  echo ok
}

# rule_of_three N: the upper 95 percent bound of a per-run rate after N runs with no
# event, 3/N as a percent with one decimal (half rounds up).
rule_of_three() {
  is_num "$1" || return 1
  [ "$1" -ge 1 ] || return 1
  local t=$(((6000 + $1) / (2 * $1)))
  printf '%d.%d\n' $((t / 10)) $((t % 10))
}

# con01_outcome AGREE ORPHAN-ONLY ROW-WITHOUT-FILE BACKUP-REFUSED RESTORE-FAILED N MIN_RUNS EXPOSED NAMED:
# the class counts of one stage, its number of runs, the floor, the runs with an
# upload in the window and the number of disagreeing runs (orphan, row without
# file, refused) whose window was named (a refused backup is named by its rsync
# status). One of INCOMPLETE (the counts do not add up to N: a run failed in the
# harness), UNNAMED (a disagreement whose window cannot be named: FUT-02 candidate),
# RESTORE-FAILED, INCONCLUSIVE (exposure_ok fails), ROW-WITHOUT-FILE-NAMED,
# ORPHAN-ONLY-NAMED, REFUSED-NAMED, AGREE-ALL. The harmful direction dominates.
con01_outcome() {
  local a=$1 o=$2 m=$3 r=$4 x=$5 n=$6 min=$7 ex=$8 nm=$9 dis
  is_num "$a" "$o" "$m" "$r" "$x" "$n" "$min" "$ex" "$nm" || return 1
  if [ $((a + o + m + r + x)) -ne "$n" ]; then
    echo INCOMPLETE
    return 0
  fi
  dis=$((o + m + r))
  if [ "$nm" -lt "$dis" ]; then
    echo UNNAMED
  elif [ "$x" -gt 0 ]; then
    echo RESTORE-FAILED
  elif ! exposure_ok "$n" "$min" "$ex" > /dev/null; then
    echo INCONCLUSIVE
  elif [ "$m" -gt 0 ]; then
    echo ROW-WITHOUT-FILE-NAMED
  elif [ "$o" -gt 0 ]; then
    echo ORPHAN-ONLY-NAMED
  elif [ "$r" -gt 0 ]; then
    echo REFUSED-NAMED
  else
    echo AGREE-ALL
  fi
}

# restore_required N MIN_RUNS: how many runs of a stage must have been restored by
# restore.sh and compared after the restore for the stage to be a verdict: at least
# half of MIN_RUNS (rounded up) and never fewer than 10, and never more than the
# stage has runs (a trace of fewer than 10 runs must restore every run).
restore_required() {
  local n=$1 min=$2 req
  req=$(((min + 1) / 2))
  if [ "$req" -lt 10 ]; then req=10; fi
  if [ "$req" -gt "$n" ]; then req=$n; fi
  echo "$req"
}

# restore_floor_ok N RESTORED MIN_RUNS [REFUSED]: ok, or the reason (status 1). A
# backup that backup.sh refused has no MANIFEST and nothing to restore; in a trace
# (MIN_RUNS below 10, where the floor is every run) those runs are not asked for, but
# at least one run must have been restored. An evidence set (MIN_RUNS of 10 or more)
# keeps its floor whatever was refused.
restore_floor_ok() {
  local req ref=${4:-0}
  req=$(restore_required "$1" "$3")
  if [ "$3" -lt 10 ] && [ $(($1 - ref)) -lt "$req" ]; then
    req=$(($1 - ref))
    if [ "$req" -lt 1 ]; then req=1; fi
  fi
  if [ "$2" -ge "$req" ]; then
    echo ok
    return 0
  fi
  echo "only $2 of $1 runs were restored and compared after the restore, the floor is $req"
  return 1
}

# restore_every_ok N MIN_RUNS EVERY: what CONSIST_RESTORE_EVERY=EVERY gives: runs 1,
# 1+EVERY, ... are restored, which is ceil(N / EVERY) of them.
restore_every_ok() {
  local req cnt=$((($1 + $3 - 1) / $3))
  req=$(restore_required "$1" "$2")
  if [ "$cnt" -ge "$req" ]; then
    echo ok
    return 0
  fi
  echo "only $cnt of $1 runs would be restored and compared after the restore, the floor is $req"
  return 1
}

# restore_every_max N MIN_RUNS: the largest EVERY that still restores the required number.
restore_every_max() {
  local req
  req=$(restore_required "$1" "$2")
  echo $(($1 / req))
}

# consist_stage_counts RUNSFILE STAGE: "N AGREE ORPHAN ROWMISS REFUSED RESTFAIL
# EXPOSED NAMED RESTORED UPIN PGIN DELIN" from the RUN lines of one stage: the runs
# and the classes, the runs with an upload in the window, the runs whose window was
# named, the runs that were restored and compared after the restore (a failed
# restore and a refused backup are not), and the writes that ended in the window.
consist_stage_counts() {
  awk -v st="$2" '
    $1 == "RUN" && $3 == st {
      n++
      split("", v)
      for (i = 4; i <= NF; i++) { k = index($i, "="); if (k > 0) v[substr($i, 1, k - 1)] = substr($i, k + 1) }
      c = v["CLASS"]
      if (c == "AGREE") a++
      else if (c == "ORPHAN-ONLY") o++
      else if (c == "ROW-WITHOUT-FILE") m++
      else if (c == "BACKUP-REFUSED") r++
      else if (c == "RESTORE-FAILED") x++
      if (v["WINDOW-UPLOADS"] ~ /^[0-9]+$/) { up += v["WINDOW-UPLOADS"]; if (v["WINDOW-UPLOADS"] + 0 > 0) e++ }
      if (v["WINDOW-PAGES"] ~ /^[0-9]+$/) pg += v["WINDOW-PAGES"]
      if (v["WINDOW-DELETES"] ~ /^[0-9]+$/) dl += v["WINDOW-DELETES"]
      if (v["NAMED"] == "yes") nm++
      if (v["RESTORED"] == "yes" && (c == "AGREE" || c == "ORPHAN-ONLY" || c == "ROW-WITHOUT-FILE")) rs++
    }
    END { printf "%d %d %d %d %d %d %d %d %d %d %d %d\n", n, a, o, m, r, x, e, nm, rs, up, pg, dl }' "$1"
}

# consist_retry_counts RUNSFILE STAGE: "RETRIED RETRY-COMPLETED ABORTED-24 LEFT" from the
# RUN lines of one stage: the runs whose RETRIES token is above 0, those of them whose
# BACKUP-RC is 0, the runs whose BACKUP-RC is 24 (a night that ended with status 24 after
# the repeat) and the sum of the LEFT tokens (directories without a MANIFEST that a night
# left). A line without the tokens, an older file or a CLASS=HARNESS-FAILED line, counts
# as 0 for RETRIES and for LEFT; a token that is not a number does too.
consist_retry_counts() {
  awk -v st="$2" '
    $1 == "RUN" && $3 == st {
      split("", v)
      for (i = 4; i <= NF; i++) { k = index($i, "="); if (k > 0) v[substr($i, 1, k - 1)] = substr($i, k + 1) }
      r = (v["RETRIES"] ~ /^[0-9]+$/) ? v["RETRIES"] + 0 : 0
      l = (v["LEFT"] ~ /^[0-9]+$/) ? v["LEFT"] + 0 : 0
      left += l
      if (r > 0) { retried++; if (v["BACKUP-RC"] == "0") done++ }
      if (v["BACKUP-RC"] == "24") ab++
    }
    END { printf "%d %d %d %d\n", retried, done, ab, left }' "$1"
}

# consist_restored_numbers RUNSFILE STAGE: the numbers of the runs counted as restored.
consist_restored_numbers() {
  awk -v st="$2" '
    $1 == "RUN" && $3 == st && $0 ~ / RESTORED=yes / && ($4 == "CLASS=AGREE" || $4 == "CLASS=ORPHAN-ONLY" || $4 == "CLASS=ROW-WITHOUT-FILE") { printf "%s%s", sep, $2; sep = " " }
    END { printf "\n" }' "$1"
}

# consist_rc_histogram RUNSFILE STAGE: "RC=COUNT RC=COUNT" of BACKUP-RC, ascending.
consist_rc_histogram() {
  awk -v st="$2" '
    $1 == "RUN" && $3 == st { for (i = 4; i <= NF; i++) if ($i ~ /^BACKUP-RC=/) c[substr($i, 11)]++ }
    END { for (k in c) print k "=" c[k] }' "$1" | sort -n | tr '\n' ' ' | sed 's/ $//'
}

# consist_name_run STAGE N W DB0 DB1 ME1: the window of every file that accounts for a
# run's disagreement. The files are the rows without a file, the files without a row
# and, for a refused backup, the files rsync says vanished. A delete is found in the
# churn log by the stem of the stored name (the part before the first dot or dash),
# an upload by the original name that the live database gives that stem. Sets R_NAMED
# (yes when every file has a name; for a refused backup yes when rsync gave a status)
# and R_WNAME (the distinct names, comma separated), appends one WINDOW-NAME line per
# file to W/names.txt.
consist_name_run() {
  local stage=$1 n=$2 w=$3 db0=$4 db1=$5 me1=$6 f stem line kind s e id orig name nres=0 nun=0 names=""
  : > "$w/names.txt"
  awk '$3 == "delete" && $4 == 303 { split($5, a, ":"); print a[2], $2, $1, $5 }' "$w/churn.txt" > "$w/ev-delete.txt"
  awk '$3 == "upload" && $4 == 303 { print $5, $2, $1 }' "$w/churn.txt" > "$w/ev-upload.txt"
  dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
    "select website_id || '/' || filename || '|' || original_name from media where original_name like 'churn-%'" > "$w/map.txt" 2> /dev/null || true
  { cat "$w/MISSING" "$w/ORPHAN" 2> /dev/null || true
    sed -n 's|.*file has vanished: "/opt/holzcloud/data/media/\(.*\)".*|\1|p' "$w/backup.txt"
  } | LC_ALL=C sort -u | head -n 40 > "$w/names-files.txt"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    stem=$(printf '%s' "$f" | sed 's|^\([0-9]*/[0-9a-f]*\).*|\1|')
    kind=""; s=""; e=""; id=""; name=""
    line=$(awk -v k="$stem" '$1 == k { print $2, $3, $4; exit }' "$w/ev-delete.txt")
    if [ -n "$line" ]; then
      kind=delete
      set -- $line
      s=$1; e=$2; id=$3
    else
      orig=$(awk -F'|' -v k="$stem" 'index($1, k ".") == 1 { print $2; exit }' "$w/map.txt")
      if [ -n "$orig" ]; then
        line=$(awk -v k="$orig" '$1 == k { print $2, $3; exit }' "$w/ev-upload.txt")
        if [ -n "$line" ]; then
          kind=upload
          set -- $line
          s=$1; e=$2; id=$orig
        fi
      fi
    fi
    if [ -n "$kind" ] && is_num "$db0" "$db1" "$me1"; then
      name=$(window_name "$kind" "$s" "$e" "$db0" "$db1" "$me1") || name=""
    fi
    if [ -n "$name" ]; then
      printf 'WINDOW-NAME run %s %s: %s (%s %s %s %s) file %s\n' "$stage" "$n" "$name" "$kind" "$id" "$s" "$e" "$f" >> "$w/names.txt"
      nres=$((nres + 1))
      case ",$names," in *",$name,"*) ;; *) names=${names:+$names,}$name ;; esac
    else
      printf 'WINDOW-NAME run %s %s: UNRESOLVED file %s (no event of the churn log accounts for it)\n' "$stage" "$n" "$f" >> "$w/names.txt"
      nun=$((nun + 1))
    fi
  done < "$w/names-files.txt"
  R_NAMED=no
  if [ "$nres" -gt 0 ] && [ "$nun" -eq 0 ]; then R_NAMED=yes; fi
  if [ "$R_CLASS" = BACKUP-REFUSED ]; then
    if [ "$R_RSYNC" != - ]; then R_NAMED=yes; names=${names:+$names,}rsync-$R_RSYNC; fi
  fi
  R_WNAME=${names:--}
}

# consist_pick_every N MIN_RUNS STAGES FIRST_S RESTORE_S CURRENT: after the first run,
# the smallest CONSIST_RESTORE_EVERY (not below CURRENT) at which STAGES stages of N
# runs fit in 90 minutes, given that a run costs FIRST_S seconds of which RESTORE_S
# are the restore; it never goes past what the restore floor allows. Prints
# "EVERY ok|over PROJECTED_SECONDS"; over means even the largest allowed EVERY does
# not fit (the runs are not reduced for that).
consist_pick_every() {
  local n=$1 min=$2 st=$3 first=$4 rs=$5 cur=$6 b e max t
  is_num "$n" "$min" "$st" "$first" "$rs" "$cur" || return 1
  b=$((first - rs))
  if [ "$b" -lt 0 ]; then b=0; fi
  max=$(restore_every_max "$n" "$min")
  if [ "$cur" -gt "$max" ]; then max=$cur; fi
  e=$cur
  while :; do
    t=$((st * (n * b + ((n + e - 1) / e) * rs)))
    if [ "$t" -le 5400 ]; then
      echo "$e ok $t"
      return 0
    fi
    if [ "$e" -ge "$max" ]; then
      echo "$e over $t"
      return 0
    fi
    e=$((e + 1))
  done
}

# consist_stage_report STAGE RUNSFILE MIN_RUNS: the KEY lines of one stage and its
# outcome, from the RUN lines alone, with a BAD line per cause when the stage is no
# verdict. Returns the outcome in STAGE_OUTCOME.
consist_stage_report() {
  local stage=$1 f=$2 min=$3 n a o m r x e nm rs up pg dl out why rt rt_n rt_c rt_a rt_l
  read -r n a o m r x e nm rs up pg dl <<< "$(consist_stage_counts "$f" "$stage")"
  read -r rt_n rt_c rt_a rt_l <<< "$(consist_retry_counts "$f" "$stage")"
  out=$(con01_outcome "$a" "$o" "$m" "$r" "$x" "$n" "$min" "$e" "$nm") || out=INCOMPLETE
  STAGE_OUTCOME=$out
  key_line "CONSIST-RUNS-STAGE-$stage: $n"
  key_line "RESTORED-RUNS-STAGE-$stage: $rs"
  key_line "RESTORED-RUN-NUMBERS-STAGE-$stage: $(consist_restored_numbers "$f" "$stage")"
  key_line "CLASS-AGREE-STAGE-$stage: $a"
  key_line "CLASS-ORPHAN-ONLY-STAGE-$stage: $o"
  key_line "CLASS-ROW-WITHOUT-FILE-STAGE-$stage: $m"
  key_line "CLASS-BACKUP-REFUSED-STAGE-$stage: $r"
  key_line "CLASS-RESTORE-FAILED-STAGE-$stage: $x"
  key_line "EXPOSED-RUNS-STAGE-$stage: $e"
  key_line "UPLOADS-IN-WINDOW-STAGE-$stage: $up"
  key_line "PAGES-IN-WINDOW-STAGE-$stage: $pg"
  key_line "DELETES-IN-WINDOW-STAGE-$stage: $dl"
  key_line "BACKUP-RC-HISTOGRAM-STAGE-$stage: $(consist_rc_histogram "$f" "$stage")"
  key_line "RETRIED-RUNS-STAGE-$stage: $rt_n"
  key_line "RETRY-COMPLETED-RUNS-STAGE-$stage: $rt_c"
  key_line "ABORTED-24-RUNS-STAGE-$stage: $rt_a"
  key_line "MANIFEST-LESS-DIRS-STAGE-$stage: $rt_l"
  key_line "OUTCOME-STAGE-$stage: $out"
  key_line "CONSIST-SUMMARY-STAGE-$stage: RUNS=$n RESTORED=$rs AGREE=$a ORPHAN-ONLY=$o ROW-WITHOUT-FILE=$m BACKUP-REFUSED=$r RESTORE-FAILED=$x EXPOSED=$e UPLOADS-IN-WINDOW=$up OUTCOME=$out"
  if [ "$out" = AGREE-ALL ]; then
    rt=$(rule_of_three "$n")
    key_line "NO-DISAGREEMENT-STAGE-$stage: 0 disagreements in $n runs, $rs of them restored and compared after the restore, $e with at least one upload inside the window"
    key_line "RULE-OF-THREE-STAGE-$stage: $rt percent per run at 95 percent confidence, under this load, in this container"
  fi
  if [ "$n" -lt "$min" ]; then
    check_bad "stage $stage: $n runs are fewer than the floor of $min (CONSIST_MIN_RUNS)"
  fi
  why=$(restore_floor_ok "$n" "$rs" "$min" "$r") || check_bad "stage $stage: $why"
  if [ "$rt_l" -gt 0 ]; then
    check_bad "stage $stage: $rt_l director(ies) without a MANIFEST were left behind by the nights of the stage; backup.sh removes the directory of a night that fails before its MANIFEST"
  fi
  case "$out" in
    INCONCLUSIVE)
      why=$(exposure_ok "$n" "$min" "$e") || true
      check_bad "stage $stage is INCONCLUSIVE: $why. The load did not reach the window often enough: raise BULK_MB, which widens the media copy, or CHURN_WORKERS, and run again"
      ;;
    UNNAMED)
      check_bad "stage $stage has a disagreement whose window cannot be named (UNNAMED, see the WINDOW-NAME lines of consist-runs): FUT-02, an atomic snapshot, is a candidate requirement"
      ;;
    RESTORE-FAILED)
      check_bad "stage $stage: restore.sh failed on a complete backup in $x run(s)"
      ;;
    INCOMPLETE)
      check_bad "stage $stage: the class counts do not add up to the $n runs; a run failed in the harness"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Phase 25, plan 01, task 3: the window made on purpose. The shipped backup.sh is
# paused, through the HOLZCLOUD_BIN variable it documents, between its database
# snapshot and its copy of the media; one write is made in the pause; the directory
# is restored by the shipped restore.sh and compared. Three controls: nothing written
# (the comparator must say AGREE), a file uploaded (ORPHAN-ONLY), a file deleted
# (ROW-WITHOUT-FILE).
# ---------------------------------------------------------------------------

# hold_expect ACTION: the class a control must produce. Any other action returns
# non-zero and prints nothing.
hold_expect() {
  case "$1" in
    none) echo AGREE ;;
    upload) echo ORPHAN-ONLY ;;
    delete) echo ROW-WITHOUT-FILE ;;
    *) return 1 ;;
  esac
}

selftest_hold() {
  st_eq "hold_expect none" AGREE "$(hold_expect none)"
  st_eq "hold_expect upload" ORPHAN-ONLY "$(hold_expect upload)"
  st_eq "hold_expect delete" ROW-WITHOUT-FILE "$(hold_expect delete)"
  st_fail "hold_expect page" hold_expect page
  st_fail "hold_expect empty" hold_expect ""
  st_fail "hold_expect upper case" hold_expect UPLOAD
  st_eq "hold_expect prints nothing for an unknown action" "" "$(hold_expect page || true)"
  st_eq "hold_expect none is the class run_class gives for agreeing lists" "$(hold_expect none)" "$(run_class 0 yes 0 AGREE)"
  st_eq "hold_expect upload is the class run_class gives for a file without a row" "$(hold_expect upload)" "$(run_class 0 yes 0 ORPHAN-ONLY)"
  st_eq "hold_expect delete is the class run_class gives for a row without a file" "$(hold_expect delete)" "$(run_class 0 yes 0 ROW-WITHOUT-FILE)"
}

HOLD_BIN=/opt/holzcloud/hc-hold-bin
HOLD_FLAG=/tmp/hc-hold.flag
HOLD_BOUND=/tmp/hc-hold.bound

# The stand-in for the binary. It is the real binary with the same arguments and the
# same status; only after a `backup` that succeeded it creates the flag and waits,
# at most 90 seconds, for the driver to remove it.
read -r -d '' P25_HOLD_SH <<'HOLD_EOF' || true
#!/bin/bash
REAL=/opt/holzcloud/holzcloud
FLAG=/tmp/hc-hold.flag
BOUND=/tmp/hc-hold.bound
"$REAL" "$@"
rc=$?
if [ "${1:-}" = backup ] && [ "$rc" -eq 0 ]; then
  : > "$FLAG"
  i=0
  while [ -e "$FLAG" ] && [ "$i" -lt 900 ]; do
    sleep 0.1
    i=$((i + 1))
  done
  if [ -e "$FLAG" ]; then : > "$BOUND"; fi
fi
exit "$rc"
HOLD_EOF

# hold_backup W ACTION: the document's manual line with the stand-in as the binary,
# the write of ACTION made while it waits, and the release. Sets H_REACHED (yes/no),
# H_SECS (the seconds the pause lasted), H_BOUND (yes when the 90 s bound was reached),
# H_EFFECT (what the database and the disk showed after the write) and H_FILE (the
# media path a delete removed). A failure of the harness is a BAD line and status 1.
hold_backup() {
  local w=$1 action=$2 line pid i=0 t_hold name res row id fn stem n f
  H_REACHED=no H_SECS=0 H_BOUND=no H_EFFECT=- H_FILE=-
  case "$BACKUP_LINE" in
    "sudo -u holzcloud /opt/holzcloud/deploy/backup.sh "*) ;;
    *)
      check_bad "the manual backup line of the document is not 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh DIR' ($BACKUP_LINE); the stand-in cannot be put in front of it"
      return 1
      ;;
  esac
  line="sudo -u holzcloud env HOLZCLOUD_BIN=$HOLD_BIN ${BACKUP_LINE#sudo -u holzcloud }"
  if ! printf '%s\n' "$P25_HOLD_SH" | dxi bash -c "cat > $HOLD_BIN && chown root:root $HOLD_BIN && chmod 755 $HOLD_BIN"; then
    check_bad "could not install the stand-in $HOLD_BIN in $NAME"
    return 1
  fi
  dx rm -f "$HOLD_FLAG" "$HOLD_BOUND" || true
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$line" > "$w/backup.txt" 2>&1 &
  pid=$!
  while [ "$i" -lt 300 ]; do
    if dx test -e "$HOLD_FLAG"; then
      H_REACHED=yes
      break
    fi
    if ! kill -0 "$pid" 2> /dev/null; then break; fi
    sleep 0.2
    i=$((i + 1))
  done
  if [ "$H_REACHED" = yes ]; then
    t_hold=$SECONDS
    case "$action" in
      none)
        H_EFFECT="nothing written"
        ;;
      upload)
        name=window-c1-$(date +%s).jpg
        dx bash -c 'umask 077; cat /root/hc-base.jpg > "$1"; printf "trailer window %s" "$(date +%s%N)" >> "$1"' _ /root/.hc-win-up.jpg || true
        res=$(sess upload "$WID" /root/.hc-jar-window /root/.hc-win-up.jpg "$name") || res="HTTP none error"
        dx rm -f /root/.hc-win-up.jpg || true
        row=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select website_id || '/' || filename from media where original_name = '$name'") || row=""
        H_FILE=$row
        if [ "${res%% /*}" = "HTTP 303" ] && [ -n "$row" ] && dx test -f "/opt/holzcloud/data/media/$row"; then
          H_EFFECT="uploaded $name: $res, row and file $row in the live site"
        else
          check_bad "control upload: the write in the pause did not take effect (answer '$res', row '$row')"
        fi
        ;;
      delete)
        row=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id || '|' || website_id || '/' || filename from media where original_name like 'churn-%' order by id limit 1") || row=""
        if [ -z "$row" ]; then
          check_bad "control delete: no earlier churn upload exists to delete"
        else
          id=${row%%|*}; fn=${row#*|}; stem=${fn%%.*}
          res=$(sess delete "$WID" /root/.hc-jar-window "$id") || res="HTTP none error"
          n=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from media where id = $id") || n="?"
          f=$(dxo bash -c "ls /opt/holzcloud/data/media/${stem}* 2> /dev/null | wc -l" | tr -d ' ') || f="?"
          H_FILE=$fn
          if [ "${res%% /*}" = "HTTP 303" ] && [ "$n" = 0 ] && [ "$f" = 0 ]; then
            H_EFFECT="deleted media $id ($fn): $res, row gone, $f files of it left in the live site"
          else
            check_bad "control delete: the write in the pause did not take effect (answer '$res', rows left $n, files left $f)"
          fi
        fi
        ;;
    esac
    dx rm -f "$HOLD_FLAG" || true
    H_SECS=$((SECONDS - t_hold))
  fi
  wait "$pid" || true
  if dx test -e "$HOLD_BOUND"; then H_BOUND=yes; fi
  dx rm -f "$HOLD_FLAG" "$HOLD_BOUND" || true
  if [ "$H_REACHED" != yes ]; then
    check_bad "control $action: the pause was never reached (no $HOLD_FLAG within 60 s); the transcript ends: $(tail -n 3 "$w/backup.txt" | tr '\n' '|')"
  fi
  if [ "$H_BOUND" = yes ]; then
    check_bad "control $action: the 90 s bound of the pause was reached; the driver was too slow"
  fi
  return 0
}

# window_cleanup: nothing of the controls stays in either container.
window_cleanup() {
  dx rm -f "$HOLD_BIN" "$HOLD_FLAG" "$HOLD_BOUND" /root/.hc-win-up.jpg /root/.hc-jar-window || true
  dxt rm -f /root/.hc-jar-window-tgt /root/hc-sess.sh || true
  if [ "${WIN_TGT_PASS:-}" = made ]; then dxt rm -f /root/.hc-login-pass || true; fi
}

# src_line FILE PATTERN: the number of the first line of FILE that holds PATTERN
# (fixed string), or nothing.
src_line() {
  grep -n -F -m 1 -- "$2" "$1" | cut -d: -f1
}

# window_mechanisms: what the code says, as INFO lines with the line they were read from.
window_mechanisms() {
  local a b c d
  a=$(src_line internal/media/intake.go 'StoreFile(source, destPath, in.limit)')
  b=$(src_line internal/media/intake.go 'store.Create(ctx, websiteID, in.Filename')
  if [ -n "$a" ] && [ -n "$b" ] && [ "$a" -lt "$b" ]; then
    check_info "MECHANISM (read from the code, not measured): an upload writes the file (internal/media/intake.go:$a) before it inserts the row (internal/media/intake.go:$b)"
  else
    check_bad "internal/media/intake.go no longer writes the file before the row (lines '$a' '$b'); the reading of the mechanism is out of date"
  fi
  a=$(src_line internal/media/upload.go 'os.Create(destPath)')
  if [ -n "$a" ]; then
    check_info "MECHANISM (read from the code, not measured): the file is created straight under its final name (internal/media/upload.go:$a), no temporary name and no rename, so a copy that meets it half written holds a partial file for which there is no row yet"
  else
    check_bad "internal/media/upload.go no longer creates the file at its final name; the reading of the mechanism is out of date"
  fi
  a=$(src_line internal/media/usage.go 's.removeVariantFiles(ctx, id, websiteID, dataDir)')
  b=$(src_line internal/media/usage.go 'DELETE FROM media WHERE id = $1')
  c=$(src_line internal/media/usage.go 'os.Remove(diskPath)')
  if [ -n "$a" ] && [ -n "$b" ] && [ -n "$c" ] && [ "$a" -lt "$b" ] && [ "$b" -lt "$c" ]; then
    check_info "MECHANISM (read from the code, not measured): a delete removes the variant files (internal/media/usage.go:$a), then the row (internal/media/usage.go:$b), then the original (internal/media/usage.go:$c); a snapshot taken after the first step has the row, a copy taken after the last step has no file"
  else
    check_bad "internal/media/usage.go no longer deletes variants, row and original in that order (lines '$a' '$b' '$c'); the reading of the mechanism is out of date"
  fi
  a=$(src_line internal/db/maintain.go "VACUUM INTO '")
  b=$(src_line cmd/holzcloud/cli.go 'db.Backup(context.Background(), database, fs.Arg(0))')
  if [ -n "$a" ] && [ -n "$b" ]; then
    check_info "MECHANISM (read from the code, not measured): the snapshot is VACUUM INTO (internal/db/maintain.go:$a) run by the command line process 'holzcloud backup' (cmd/holzcloud/cli.go:$b), not by the service; it is a consistent database file at one instant, taken before the media are copied"
  else
    check_bad "the snapshot is no longer taken by 'holzcloud backup' through VACUUM INTO (lines '$a' '$b'); the reading of the mechanism is out of date"
  fi
  a=$(src_line deploy/backup.sh 'set -euo pipefail')
  b=$(src_line deploy/backup.sh 'run_rsync "${name}" -a "${DATA_DIR}/${name}/"')
  d=$(src_line deploy/backup.sh 'run_rsync()')
  c=$(src_line deploy/backup.sh 'echo "timestamp=${TIMESTAMP}"')
  if [ -n "$a" ] && [ -n "$b" ] && [ -n "$c" ] && [ -n "$d" ]; then
    check_info "MECHANISM (read from the code, not measured): backup.sh runs with set -e (deploy/backup.sh:$a) and copies the media through run_rsync (deploy/backup.sh:$b, defined at deploy/backup.sh:$d), which repeats an rsync that ended with status 24, a file vanished during the copy, once; a second status 24 ends the script before the MANIFEST is written (deploy/backup.sh:$c), the cleanup then removes the directory of the night, and restore.sh refuses a directory without a MANIFEST"
  else
    check_bad "deploy/backup.sh no longer has set -e, run_rsync and the MANIFEST as read (lines '$a' '$b' '$d' '$c'); the reading of the mechanism is out of date"
  fi
}

# window_consequences: what an operator sees on the restored site after C2. Raw output,
# no value is asserted.
window_consequences() {
  local cf=$OUT_DIR/window-C2-compare.txt rf=$OUT_DIR/window-C2-restore.txt mf pf st d lr last made=no
  mf=$(sed -n 's/^KEY MISSING-FILE: //p' "$cf" | grep -v -e '-thumb\.' -e '-medium\.' | head -n 1)
  if [ -z "$mf" ]; then mf=$(sed -n 's/^KEY MISSING-FILE: //p' "$cf" | head -n 1); fi
  pf=${BASEREF#/media/}
  if [ -z "$mf" ]; then
    check_bad "C2: the comparison named no file that the backup lacks; the consequences cannot be read"
    return 0
  fi
  st=$(dxto curl -s --max-time 30 -o /dev/null -w '%{http_code} %{size_download}' -H 'Host: test.local' "http://127.0.0.1:8080/media/$mf") || st="curl failed"
  key_line "C2-CONSEQUENCE media route, file the backup lacks (GET /media/$mf, Host test.local): HTTP status and body length: $st"
  st=$(dxto curl -s --max-time 30 -o /dev/null -w '%{http_code} %{size_download}' -H 'Host: test.local' "http://127.0.0.1:8080/media/$pf") || st="curl failed"
  key_line "C2-CONSEQUENCE media route, file the backup has (GET /media/$pf, Host test.local): HTTP status and body length: $st"
  d=$(mktemp -d) || return 0
  printf '%s\n' "$P25_COMPARE_SH" | dxti bash -s -- live /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "" "" > "$d/live.out" 2> /dev/null || true
  split_sections "$d/live.out" "$d"
  key_line "C2-CONSEQUENCE holzcloud check on a copy of the restored database: exit status $(sed -n 's/^CHECK-RC: //p' "$d/CHECK") and output: $(sed -n 's/^CHECK-OUT: //p' "$d/CHECK" | cut -c1-300)"
  rm -rf "${d:?}"
  lr=$(sed -n 's/^RC //p' "$rf" | tail -n 1)
  last=$(grep -v -e '^WALL ' -e '^RC ' "$rf" | sed 's/^[0-9]* //' | grep -v '^$' | tail -n 1 | cut -c1-300)
  key_line "C2-CONSEQUENCE restore.sh exit status $lr, its last line (empty lines skipped): $last"
  if ! dxt test -s /root/.hc-login-pass; then
    if dxo cat /root/.hc-login-pass | dxti bash -c 'umask 077; cat > /root/.hc-login-pass'; then WIN_TGT_PASS=made; made=yes; fi
  fi
  printf '%s\n' "$P25_SESS_SH" | dxti bash -c 'umask 077; cat > /root/hc-sess.sh && chmod 700 /root/hc-sess.sh' || true
  st=$(dxto bash /root/hc-sess.sh login churn@example.test /root/.hc-login-pass /root/.hc-jar-window-tgt) || st="HTTP none error"
  key_line "C2-CONSEQUENCE editor login on the restored site: $st"
  st=$(dxto bash /root/hc-sess.sh get "/admin/websites/$WID/media" /root/.hc-jar-window-tgt) || st="HTTP none error"
  key_line "C2-CONSEQUENCE media list of the admin (editor session, GET /admin/websites/$WID/media): $st"
  grep -h 'PAGE-REFS' "$cf" | sed 's/^/C2-CONSEQUENCE restored site: /' | tee -a "$CHECK_OUT" || true
  if [ "$made" = yes ]; then dxt rm -f /root/.hc-login-pass || true; WIN_TGT_PASS=; fi
}

cmd_window() {
  local o=$OUT_DIR/window.txt c action exp bl rl epoch res nrows k rc=0 miss orph f
  need_container
  dxt true > /dev/null 2>&1 \
    || die "container $TGT_NAME does not answer; bring the second host up with: bash deploy/test/nightly.sh target (TGT_NAME=$TGT_NAME)"
  [ "$(tail -n 1 "$OUT_DIR/prep.txt" 2> /dev/null)" = "PREP OK" ] \
    || die "$OUT_DIR/prep.txt does not end with PREP OK; run 'bash deploy/test/nightly.sh prep' first"
  mkdir -p "$OUT_DIR"
  BADS=0
  res_begin "$o"
  : > "$OUT_DIR/window-runs.txt"
  P25_RUNS_FILE=$OUT_DIR/window-runs.txt
  WIN_TGT_PASS=
  trap window_cleanup EXIT
  p25_ship
  WID=$(p25_wid)
  BASEREF=$(p25_baseref)
  [ -n "$WID" ] && [ -n "$BASEREF" ] || die "no website called Test or no media file in $NAME"
  bl=$(doc_backup_line) || die "deploy/DEPLOY.md has no line 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups'"
  rl=$(doc_restore_line) || die "deploy/DEPLOY.md has no restore line 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  BACKUP_LINE=${bl#*:}
  RESTORE_LINE_SAMPLE=${rl#*:}
  key_line "BACKUP-LINE: deploy/DEPLOY.md:${bl%%:*}: $BACKUP_LINE (HOLZCLOUD_BIN=$HOLD_BIN is put in front of the script through env, as backup.sh documents)"
  key_line "RESTORE-LINE: deploy/DEPLOY.md:${rl%%:*}: $RESTORE_LINE_SAMPLE (yes on stdin)"
  key_line "WEBSITE: $WID"
  key_line "HOLD-BOUND-S: 90 (a bounded wait in the stand-in; the driver removes the flag as soon as its write is done)"
  deviation "DEVIATION[hold]: the pause is the container's addition. $HOLD_BIN is a root-owned 755 stand-in for the binary: the real binary with the same arguments and status, and after a successful 'backup' it creates $HOLD_FLAG and waits (at most 90 s) until the driver removes it. backup.sh is untouched and is given the stand-in through the HOLZCLOUD_BIN variable it documents. The window it opens, between the snapshot and the copy of the media, is the one in backup.sh's own comment; only its length is the container's"
  window_mechanisms

  # The editor's session for the writes in the pauses.
  res=$(sess login churn@example.test /root/.hc-login-pass /root/.hc-jar-window) || res="HTTP none error"
  if [ "$res" = "HTTP 303 /admin/" ]; then
    check_pass "the editor logs in for the controls: $res"
  else
    check_bad "the editor login answered '$res', expected 'HTTP 303 /admin/'"
  fi
  # C2 needs an earlier churn upload to delete; without one a seed is uploaded before the pause.
  nrows=$(dxo runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from media where original_name like 'churn-%'") || nrows=0
  if [ "${nrows:-0}" -lt 1 ]; then
    epoch=$(date +%s)
    dx bash -c 'umask 077; cat /root/hc-base.jpg > /root/.hc-win-up.jpg; printf "trailer seed %s" "$(date +%s%N)" >> /root/.hc-win-up.jpg' || true
    res=$(sess upload "$WID" /root/.hc-jar-window /root/.hc-win-up.jpg "churn-window-seed-$epoch.jpg") || res="HTTP none error"
    dx rm -f /root/.hc-win-up.jpg || true
    key_line "SEED-UPLOAD: $res (no churn upload existed for control C2 to delete)"
  else
    key_line "SEED-UPLOAD: not needed, $nrows churn uploads exist for control C2 to delete"
  fi

  k=0
  for action in none upload delete; do
    c=C$k
    k=$((k + 1))
    exp=$(hold_expect "$action") || { check_bad "no expectation for the action $action"; continue; }
    P25_HOLD=$action
    H_REACHED=no H_SECS=0 H_BOUND=no H_EFFECT=- H_FILE=-
    if consist_run "$c" 1 yes; then
      :
    else
      check_bad "control $c: the harness failed (BAD line above)"
      P25_HOLD=
      continue
    fi
    P25_HOLD=
    key_line "CONTROL $c ACTION=$action HOLD-REACHED=$H_REACHED HOLD-RELEASED-IN-S=$H_SECS MISSING=$R_MISSING ORPHAN=$R_ORPHAN CLASS=$R_CLASS EXPECTED=$exp"
    key_line "CONTROL-$c-BACKUP-RC: $R_BRC RESTORE-RC: $R_RRC HOLD-BOUND-REACHED: $H_BOUND"
    key_line "CONTROL-$c-WRITE: $H_EFFECT"
    sed -n 's/^KEY MISSING-FILE: /MISSING-FILE-'"$c"': /p' "$OUT_DIR/window-$c-compare.txt" 2> /dev/null | tee -a "$CHECK_OUT" || true
    sed -n 's/^KEY ORPHAN-FILE: /ORPHAN-FILE-'"$c"': /p' "$OUT_DIR/window-$c-compare.txt" 2> /dev/null | tee -a "$CHECK_OUT" || true
    if [ "$H_REACHED" != yes ]; then
      check_bad "control $c: the pause was not reached, so the class says nothing about the window"
    elif [ "$R_CLASS" = "$exp" ]; then
      check_pass "control $c ($action): class $R_CLASS, as expected"
    else
      check_bad "control $c ($action): class $R_CLASS, expected $exp"
    fi
    if [ "$c" = C2 ]; then window_consequences; fi
  done
  P25_RUNS_FILE=

  window_cleanup
  trap - EXIT
  if dx test ! -e "$HOLD_BIN" && dx test ! -e "$HOLD_FLAG" && dx test ! -e "$HOLD_BOUND"; then
    check_pass "the stand-in, the flag and the bound marker are gone from $NAME"
  else
    check_bad "the stand-in or its flag is still in $NAME"
  fi
  if dx bash -c "pgrep -f '[h]c-hold-bin' > /dev/null"; then
    check_bad "a process of the stand-in is still running in $NAME"
  else
    check_pass "no process of the stand-in is left in $NAME"
  fi
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "WINDOW FAILED"
    say "nightly.sh: WINDOW FAILED, $BADS BAD line(s); see $o"
    exit 1
  fi
  key_line "WINDOW OK"
}

# ---------------------------------------------------------------------------
# Phase 25, plan 02: CON-02. A backup made by the release before this one, with
# that release's own backup.sh, restored by this release's restore.sh on a fresh
# box. compat-source builds the previous release from its tag and populates it;
# compat plays backup, transfer and restore and compares what is on the far side.
# ---------------------------------------------------------------------------

# version_is_tag LINE TAG: LINE, the output of `holzcloud version`, names exactly the
# tag: "holzcloud TAG (HEX)". A build that is some commits past the tag
# ("holzcloud v0.1.0-35-gf087dca (f087dca)"), a "dev" build, another tag and an empty
# line are not that tag.
version_is_tag() {
  local line=$1 tag=$2 rest
  [ -n "$line" ] && [ -n "$tag" ] || return 1
  case "$line" in "holzcloud $tag ("*")") ;; *) return 1 ;; esac
  rest=${line#"holzcloud $tag ("}
  rest=${rest%")"}
  case "$rest" in "" | *[!0-9a-f]*) return 1 ;; esac
  return 0
}

# migration_ok BEFORE AFTER HIGHEST_AT_NEW: the migration level read from the backup's
# own database (BEFORE) and from the restored one after the service started (AFTER),
# against the highest migration file of the release that restored it. Passes when the
# restored database stands at that highest migration and did not go down. A level
# below the highest means a pending migration was not applied; a level above the
# highest means the restoring release is older than the database; either is no pass.
migration_ok() {
  is_num "$1" "$2" "$3" || return 1
  [ "$2" -eq "$3" ] && [ "$2" -ge "$1" ]
}

# migration_sentence BEFORE AFTER: the sentence the file carries about the migrations,
# computed from the two levels and not typed. Refuses a non-number and a level that
# went down.
migration_sentence() {
  is_num "$1" "$2" || return 1
  [ "$2" -ge "$1" ] || return 1
  if [ "$1" -eq "$2" ]; then
    echo "No migration separates the two releases: the backup is at level $1, the restored database is at level $2, the restore ran none."
  else
    echo "$(($2 - $1)) migration(s) separate the two releases: the backup is at level $1, the restored database is at level $2, they were applied."
  fi
}

# compat_volatile TABLE: prints the reason and returns 0 for a table whose row count moves
# with traffic or with the restore itself; returns 1 for every other table. The list is
# the one of run.sh's is_volatile (deploy/test/run.sh), the only place a difference in a
# count may be excused, and each entry carries its reason. A difference in a table that
# is not listed here is a BAD and an investigation, never a new entry.
compat_volatile() {
  case "$1" in
    sessions) echo "one row per login, deleted at expiry: the comparison itself logs in" ;;
    activity_log) echo "the audit trail, one row per admin action; logging in and saving a page add rows" ;;
    mail_outbox) echo "the mail queue: a background run drains it and the rows stay only for a while" ;;
    outbox) echo "the shop's queue of messages, drained by a background run" ;;
    plugin_store) echo "written by the plugins at run time (the 404 hook stores the missing path)" ;;
    goose_db_version) echo "one row per applied migration; compared as a list of its own, not as a count" ;;
    oauth_codes) echo "an authorisation code lives for minutes and is used once" ;;
    *) return 1 ;;
  esac
}

# compat_tables_filtered TEXT: the "table NAME COUNT" lines of TEXT that are not about a
# volatile table, sorted.
compat_tables_filtered() {
  local l n
  printf '%s\n' "$1" | while IFS= read -r l; do
    set -- $l
    n=${2:-}
    [ -n "$n" ] || continue
    compat_volatile "$n" > /dev/null || printf '%s\n' "$l"
  done | LC_ALL=C sort
}

# compat_fixture_ok PAGES USERS UPLOADS: the staged site is not trivial: at least three
# pages, two users and two uploaded media rows. A comparison of two empty sites is equal
# and proves nothing.
compat_fixture_ok() {
  is_num "$1" "$2" "$3" || return 1
  [ "$1" -ge 3 ] && [ "$2" -ge 2 ] && [ "$3" -ge 2 ]
}

selftest_compat() {
  st_ok "version_is_tag the tag itself" version_is_tag 'holzcloud v0.1.0 (4d238b9)' v0.1.0
  st_ok "version_is_tag a full length commit" version_is_tag 'holzcloud v0.1.0 (4d238b97d104296469a480e3b701d7a1f8a1c712)' v0.1.0
  st_fail "version_is_tag some commits past the tag" version_is_tag 'holzcloud v0.1.0-35-gf087dca (f087dca)' v0.1.0
  st_fail "version_is_tag a dev build" version_is_tag 'holzcloud dev (unknown)' v0.1.0
  st_fail "version_is_tag another tag" version_is_tag 'holzcloud v0.0.21 (abcdef1)' v0.1.0
  st_fail "version_is_tag a tag that only starts like the wanted one" version_is_tag 'holzcloud v0.1.01 (abcdef1)' v0.1.0
  st_fail "version_is_tag an empty line" version_is_tag '' v0.1.0
  st_fail "version_is_tag an empty tag" version_is_tag 'holzcloud v0.1.0 (4d238b9)' ''
  st_fail "version_is_tag no commit in the brackets" version_is_tag 'holzcloud v0.1.0 ()' v0.1.0
  st_fail "version_is_tag a commit that is not hex" version_is_tag 'holzcloud v0.1.0 (unknown)' v0.1.0
  st_fail "version_is_tag text after the brackets" version_is_tag 'holzcloud v0.1.0 (4d238b9) extra' v0.1.0
  st_ok "migration_ok no migration crossed" migration_ok 60 60 60
  st_ok "migration_ok a pending migration applied (58 to 60)" migration_ok 58 60 60
  st_fail "migration_ok the level went down" migration_ok 60 59 60
  st_fail "migration_ok a migration was pending and not applied" migration_ok 60 60 61
  st_fail "migration_ok the database is above the restoring release" migration_ok 61 61 60
  st_fail "migration_ok a non-number before" migration_ok x 60 60
  st_fail "migration_ok a non-number after" migration_ok 60 '' 60
  st_fail "migration_ok a non-number highest" migration_ok 60 60 x
  st_eq "migration_sentence no migration" "No migration separates the two releases: the backup is at level 60, the restored database is at level 60, the restore ran none." "$(migration_sentence 60 60)"
  st_eq "migration_sentence two migrations" "2 migration(s) separate the two releases: the backup is at level 58, the restored database is at level 60, they were applied." "$(migration_sentence 58 60)"
  st_fail "migration_sentence the level went down" migration_sentence 60 58
  st_fail "migration_sentence a non-number" migration_sentence 60 x
  st_ok "migration_ok 58, 60, 60 passes" migration_ok 58 60 60
  st_fail "migration_ok 0, 0, 60 fails: nothing applied" migration_ok 0 0 60
  st_fail "migration_ok 60, 60, empty fails" migration_ok 60 60 ""

  # The lists of the comparison. list_equal is the judge; these cases show how the
  # lists are built: a page list that differs in one HTML hash, a user list that
  # differs in one role, a table list that differs only in a volatile table.
  local pa pb ua ub ta tb tc
  pa=$'1|gruesse||draft|Gr|m1|h1\n1|liste||published|Li|m2|h2'
  pb=$'1|gruesse||draft|Gr|m1|h1\n1|liste||published|Li|m2|hX'
  ua=$'churn@example.test|editor\nlogin@example.test|admin'
  ub=$'churn@example.test|editor\nlogin@example.test|editor'
  ta=$'table pages 5\ntable sessions 3\ntable users 2\ntable activity_log 9'
  tb=$'table pages 5\ntable sessions 7\ntable users 2\ntable activity_log 14'
  tc=$'table pages 6\ntable sessions 3\ntable users 2\ntable activity_log 9'
  st_ok "list_equal two identical page lists" list_equal "$pa" "$pa"
  st_fail "list_equal page lists that differ in one HTML hash" list_equal "$pa" "$pb"
  st_ok "list_equal two identical user lists" list_equal "$ua" "$ua"
  st_fail "list_equal user lists that differ in one role" list_equal "$ua" "$ub"
  st_fail "list_equal two empty page lists prove nothing" list_equal "" ""
  st_eq "compat_tables_filtered drops the volatile tables and sorts" $'table pages 5\ntable users 2' "$(compat_tables_filtered "$ta")"
  st_ok "tables that differ only in volatile tables are equal after the exclusions" list_equal "$(compat_tables_filtered "$ta")" "$(compat_tables_filtered "$tb")"
  st_fail "tables that differ in a non-volatile table are not equal" list_equal "$(compat_tables_filtered "$ta")" "$(compat_tables_filtered "$tc")"
  st_ok "compat_volatile sessions" compat_volatile sessions
  st_ok "compat_volatile goose_db_version" compat_volatile goose_db_version
  st_fail "compat_volatile pages" compat_volatile pages
  st_fail "compat_volatile users" compat_volatile users
  st_fail "compat_volatile media" compat_volatile media
  st_ok "compat_fixture_ok three pages, two users, two uploads" compat_fixture_ok 3 2 2
  st_fail "compat_fixture_ok two pages" compat_fixture_ok 2 2 2
  st_fail "compat_fixture_ok one user" compat_fixture_ok 3 1 2
  st_fail "compat_fixture_ok one upload" compat_fixture_ok 3 2 1
  st_fail "compat_fixture_ok a non-number" compat_fixture_ok 3 x 2

  # CON-03: capfull_size_mb FLOOR MARGIN; capfull_data_ok DATA_MB FS_MB FLOOR;
  # capfull_complete_p SRC_BYTES SRC_FILES BK_BYTES BK_FILES; capfull_classify GUARD RC
  # MANIFEST PARTIAL_DIR COMPLETE.
  st_eq "capfull_size_mb 1024 and 40 give 1064" 1064 "$(capfull_size_mb 1024 40)"
  st_eq "capfull_size_mb 1024 and 0 give 1024" 1024 "$(capfull_size_mb 1024 0)"
  st_fail "capfull_size_mb a non-number floor" capfull_size_mb x 40
  st_fail "capfull_size_mb an empty margin" capfull_size_mb 1024 ""
  st_ok "capfull_data_ok 1175, 1064, 1024 is the construction that is asked for" capfull_data_ok 1175 1064 1024
  st_fail "capfull_data_ok 500, 1064, 1024: the copy would fit" capfull_data_ok 500 1064 1024
  st_fail "capfull_data_ok 1064, 1064, 1024: the copy would just fit" capfull_data_ok 1064 1064 1024
  st_fail "capfull_data_ok 1175, 900, 1024: the guard would refuse" capfull_data_ok 1175 900 1024
  st_ok "capfull_data_ok the guard at exactly the floor passes" capfull_data_ok 1175 1024 1024
  st_fail "capfull_data_ok a non-number data size" capfull_data_ok x 1064 1024
  st_fail "capfull_data_ok an empty filesystem size" capfull_data_ok 1175 "" 1024
  st_ok "capfull_complete_p equal bytes and files" capfull_complete_p 1205862400 241 1205862400 241
  st_fail "capfull_complete_p fewer bytes" capfull_complete_p 1205862400 241 1100000000 241
  st_fail "capfull_complete_p fewer files" capfull_complete_p 1205862400 241 1205862400 200
  st_fail "capfull_complete_p a non-number" capfull_complete_p 1205862400 x 1205862400 241
  st_eq "capfull_classify complete" COMPLETE "$(capfull_classify passed 0 yes yes yes)"
  st_eq "capfull_classify the guard refused" GUARD-REFUSED "$(capfull_classify refused 1 no no no)"
  st_eq "capfull_classify failed, a directory left" FAILED-PARTIAL-DIR-LEFT "$(capfull_classify passed 11 no yes no)"
  st_eq "capfull_classify failed, no directory" FAILED-NO-DIR "$(capfull_classify passed 1 no no no)"
  st_eq "capfull_classify exit 0 and a MANIFEST, but smaller than the source" FALSE-COMPLETE "$(capfull_classify passed 0 yes yes no)"
  st_fail "capfull_classify exit 0 without a MANIFEST is no class" capfull_classify passed 0 no yes yes
  st_fail "capfull_classify exit 0 without a directory is no class" capfull_classify passed 0 no no no
  st_fail "capfull_classify a refusal that exited 0" capfull_classify refused 0 no no no
  st_fail "capfull_classify a refusal that left a directory" capfull_classify refused 1 no yes no
  st_fail "capfull_classify a failure with a MANIFEST" capfull_classify passed 11 yes yes no
  st_fail "capfull_classify a failure that calls itself complete" capfull_classify passed 11 no yes yes
  st_fail "capfull_classify an unknown guard" capfull_classify maybe 0 yes yes yes
  st_fail "capfull_classify a non-number status" capfull_classify passed x no yes no
  st_fail "capfull_classify an answer that is not yes or no" capfull_classify passed 0 maybe yes yes
  st_eq "capfull_classify prints nothing when it refuses the numbers" "" "$(capfull_classify passed 0 no yes yes || true)"
}

# compat_names: the previous release is the "source" of the shared helpers (NAME,
# OUT_DIR, dx...), the fresh box with this release their "second host" (TGT_NAME,
# TGT_OUT_DIR, dxt...). Called inside a subshell by the subcommands, so that a later
# subcommand of the same call (capfull) keeps the names it was given.
compat_names() {
  local d
  [ "$OLD_NAME" != "$NEW_NAME" ] || die "OLD_NAME equals NEW_NAME ($OLD_NAME)"
  [ "$OLD_OUT_DIR" != "$NEW_OUT_DIR" ] || die "OLD_OUT_DIR equals NEW_OUT_DIR ($OLD_OUT_DIR)"
  for d in "$OLD_OUT_DIR" "$NEW_OUT_DIR"; do
    case "$d" in
      / | . | .. | ./ | "$ROOT" | "$ROOT"/) die "refusing the evidence directory '$d'" ;;
    esac
  done
  NAME=$OLD_NAME
  OUT_DIR=$OLD_OUT_DIR
  TGT_NAME=$NEW_NAME
  TGT_OUT_DIR=$NEW_OUT_DIR
}

# The state of one database, counts and hashes and nothing that opens a credential:
# the highest applied goose version and its rows, one "table NAME COUNT" line per table
# of sqlite_master, one line per page "website|slug|locale|status|title|sha256 of the
# Markdown|sha256 of the HTML", one per user "email|role" (never a hash, a token or a
# cookie), one per media row and per media_variants row "website/filename|sha256 of the
# file", MISSING for a file that is not there. Arguments: MODE DATABASE MEDIADIR. MODE
# file opens the database on the immutable URI (a backup's file, which has no WAL);
# MODE live reads the running site's database read-only as the service user.
read -r -d '' P25_STATE_SH <<'STATE_EOF' || true
set -u
mode=$1; db=$2; media=$3
if [ "$mode" = file ]; then
  q() { sqlite3 "file:$db?immutable=1" "$1"; }
else
  q() { runuser -u holzcloud -- sqlite3 -readonly "$db" ".timeout 5000" "$1"; }
fi
sha() { sha256sum | cut -d' ' -f1; }
echo '@@KEYS'
echo "LEVEL: $(q "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1")"
echo "GOOSE-ROWS: $(q "select count(*) from goose_db_version")"
echo "PAGES-COUNT: $(q "select count(*) from pages")"
echo "USERS-COUNT: $(q "select count(*) from users")"
echo "MEDIA-ROWS: $(q "select count(*) from media")"
echo "UPLOADED-MEDIA: $(q "select count(*) from media where original_name like 'churn-%'")"
echo '@@GOOSE'
q "select version_id || '|' || is_applied from goose_db_version order by id"
echo '@@TABLES'
q "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name" | while IFS= read -r t; do
  n=$(q "select count(*) from \"$t\"" 2> /dev/null) || n="?"
  echo "table $t $n"
done
echo '@@PAGES'
q "select id from pages order by website_id, locale, slug" | while IFS= read -r id; do
  meta=$(q "select website_id || '|' || slug || '|' || locale || '|' || status || '|' || title from pages where id = $id")
  md=$(q "select content_markdown from pages where id = $id" | sha)
  ht=$(q "select content_html from pages where id = $id" | sha)
  echo "$meta|$md|$ht"
done
echo '@@USERS'
q "select email || '|' || role from users order by email"
echo '@@MEDIA'
q "select website_id || '/' || filename from media union select m.website_id || '/' || v.filename from media_variants v join media m on m.id = v.media_id order by 1" | while IFS= read -r p; do
  if [ -f "$media/$p" ]; then echo "$p|$(sha < "$media/$p")"; else echo "$p|MISSING"; fi
done
echo '@@MEDIAORIG'
q "select website_id || '/' || filename from media order by 1" | while IFS= read -r p; do
  if [ -f "$media/$p" ]; then echo "$p|$(sha < "$media/$p")"; else echo "$p|MISSING"; fi
done
echo '@@END'
STATE_EOF

# compat_state CONTAINER MODE DATABASE MEDIADIR OUTFILE: P25_STATE_SH run in CONTAINER.
# Status 1 when the transcript does not end in its @@END line.
compat_state() {
  local c=$1
  printf '%s\n' "$P25_STATE_SH" | docker exec -i "$c" bash -s -- "$2" "$3" "$4" > "$5" 2> "$5.err" || return 1
  grep -q '^@@END$' "$5" || return 1
  rm -f "$5.err"
}

# compat_compare LABEL STATE_A STATE_B: the pages, the users and every media row with
# its file, as lists, judged by list_equal (a list that is empty on both sides proves
# nothing and fails); the table counts outside the volatile list (each volatile table is
# named as an INFO line with its reason); and, when both databases stand at the same
# migration level, the goose rows. PASS and BAD lines go to CHECK_OUT.
compat_compare() {
  local label=$1 sa=$2 sb=$3 t s n ta tb l tn r va vb la lb
  t=$(mktemp -d) || die "mktemp failed"
  mkdir -p "$t/a" "$t/b"
  split_sections "$sa" "$t/a"
  split_sections "$sb" "$t/b"
  ta=$(grep '^table ' "$t/a/TABLES" 2> /dev/null || true)
  tb=$(grep '^table ' "$t/b/TABLES" 2> /dev/null || true)
  while IFS= read -r l; do
    [ -n "$l" ] || continue
    set -- $l
    tn=${2:-}
    if r=$(compat_volatile "$tn"); then
      va=${3:-?}
      vb=$(printf '%s\n' "$tb" | awk -v n="$tn" '$2 == n { print $3 }')
      check_info "COMPARE $label TABLES volatile $tn: left $va, right ${vb:-absent} ($r)"
    fi
  done <<< "$ta"
  if list_equal "$(compat_tables_filtered "$ta")" "$(compat_tables_filtered "$tb")"; then
    n=$(compat_tables_filtered "$ta" | grep -c . || true)
    check_pass "COMPARE $label TABLES: the row counts of $n tables outside the volatile list are equal"
  else
    check_bad "COMPARE $label TABLES: a table outside the volatile list differs"
    diff <(compat_tables_filtered "$ta") <(compat_tables_filtered "$tb") 2> /dev/null | head -n 10 | sed "s/^/KEY DIFF-$label-TABLES: /" | tee -a "$CHECK_OUT" > /dev/null || true
  fi
  la=$(kv "$sa" LEVEL)
  lb=$(kv "$sb" LEVEL)
  if [ -n "$la" ] && [ "$la" = "$lb" ]; then
    if list_equal "$(cat "$t/a/GOOSE" 2> /dev/null)" "$(cat "$t/b/GOOSE" 2> /dev/null)"; then
      check_pass "COMPARE $label GOOSE: the levels are equal ($la) and so are the goose rows ($(grep -c . "$t/a/GOOSE" || true))"
    else
      check_bad "COMPARE $label GOOSE: the levels are equal ($la) but the goose rows differ"
      diff "$t/a/GOOSE" "$t/b/GOOSE" 2> /dev/null | head -n 6 | sed "s/^/KEY DIFF-$label-GOOSE: /" | tee -a "$CHECK_OUT" > /dev/null || true
    fi
  else
    check_info "COMPARE $label GOOSE: the levels differ (${la:-?} and ${lb:-?}); the goose rows are not compared"
  fi
  for s in PAGES USERS MEDIA; do
    n=$(grep -c . "$t/a/$s" 2> /dev/null || true)
    if list_equal "$(cat "$t/a/$s" 2> /dev/null)" "$(cat "$t/b/$s" 2> /dev/null)"; then
      check_pass "COMPARE $label $s: equal ($n lines)"
    else
      check_bad "COMPARE $label $s: the lists differ or are empty ($n lines on the left, $(grep -c . "$t/b/$s" 2> /dev/null || true) on the right)"
      diff "$t/a/$s" "$t/b/$s" 2> /dev/null | head -n 6 | sed "s/^/KEY DIFF-$label-$s: /" | tee -a "$CHECK_OUT" > /dev/null || true
    fi
  done
  rm -rf "${t:?}"
}

# compat_tamper_control SOURCE_STATE RESTORED_STATE WORKDIR: the comparison can fail. A copy
# of the restored state with one page's HTML hash changed and one user's role changed is
# compared with the source's state, and both differences must be detected. The BAD lines of
# the tampered comparison go to a file of their own and are not counted as findings.
compat_tamper_control() {
  local src=$1 rest=$2 w=$3 keep=$CHECK_OUT nb=$BADS out=$3/tamper.txt hp hu
  awk '
    /^@@/ { sect = substr($0, 3); print; next }
    sect == "PAGES" && !p { print $0 "x"; p = 1; next }
    sect == "USERS" && !u { print $0 "-tampered"; u = 1; next }
    { print }' "$rest" > "$w/state-tampered.txt"
  CHECK_OUT=$out
  : > "$out"
  compat_compare tampered "$src" "$w/state-tampered.txt" > /dev/null
  CHECK_OUT=$keep
  BADS=$nb
  hp=$(grep -c '^BAD  COMPARE tampered PAGES' "$out" || true)
  hu=$(grep -c '^BAD  COMPARE tampered USERS' "$out" || true)
  key_line "TAMPER-CONTROL: page hash changed -> $hp BAD line(s) for PAGES, user role changed -> $hu BAD line(s) for USERS"
  if [ "$hp" = 1 ] && [ "$hu" = 1 ]; then
    check_pass "the comparison fails when altered: a changed page hash and a changed user role are both detected"
  else
    check_bad "the comparison did not detect the altered copy (PAGES $hp, USERS $hu)"
  fi
}

# git_migrations REF: "COUNT HIGHEST", the migration files of REF and the highest number
# among them, from git ls-tree and nothing else.
git_migrations() {
  git ls-tree --name-only "$1" internal/db/migrations/ \
    | sed -n -E 's|^internal/db/migrations/0*([0-9]+)_.*\.sql$|\1|p' | LC_ALL=C sort -n \
    | awk '{ n++; h = $1 } END { print n + 0, h + 0 }'
}

# The second session helper: a page saved with Markdown read from a file (the helper of
# plan 01 writes one fixed sentence). Same rules: the password and the cookie are never
# printed, the only output is "HTTP CODE REDIRECT-PATH".
read -r -d '' P25_SESS2_SH <<'SESS2_EOF' || true
#!/bin/bash
# hc-sess2.sh pagemd WID JAR SLUG TITLE STATUS MDFILE
set -u
umask 077
base=http://127.0.0.1:8080
origin=https://127.0.0.1:8080
mode=${1:-}
shift || true
tmp=$(mktemp /root/.hc-sess2.XXXXXX)
trap 'rm -f "$tmp"' EXIT
token_of() {
  grep -o 'name="gorilla.csrf.Token" value="[^"]*"' "$1" | head -n 1 \
    | sed 's/.*value="//; s/"$//; s/&#43;/+/g; s/&#61;/=/g; s/&#47;/\//g'
}
case "$mode" in
  pagemd)
    wid=$1; jar=$2; slug=$3; title=$4; status=$5; md=$6
    path=/admin/websites/$wid/pages/new
    curl -s --max-time 60 -o "$tmp" -c "$jar" -b "$jar" "$base$path" || true
    tok=$(token_of "$tmp")
    if [ -z "$tok" ]; then echo "HTTP none no-csrf-field"; exit 0; fi
    res=$(curl -s -o /dev/null --max-time 120 -w '%{http_code} %{redirect_url}' -c "$jar" -b "$jar" \
      -H "Referer: $origin$path" -H "X-CSRF-Token: $tok" \
      --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "title=$title" \
      --data-urlencode "slug=$slug" --data-urlencode "status=$status" \
      --data-urlencode "content_markdown@$md" "$base$path")
    code=${res%% *}
    loc=${res#* }
    loc=${loc#$base}
    echo "HTTP $code ${loc:--}"
    ;;
  *)
    echo "HTTP none bad-mode"
    ;;
esac
SESS2_EOF

# compat_fixture WID: what a person made on the previous release, through its own admin
# forms as the editor of plan 01's prep and nothing else: three pages with distinct
# Markdown (non-ASCII text, an outbound link, an inner link, a picture that is one of the
# uploads; the third is requested as published, and what the editor may do is read back
# from the database) and one more upload. Every answer is a KEY line.
compat_fixture() {
  local wid=$1 jar=/root/.hc-jar-compat-fx db=/opt/holzcloud/data/holzcloud.sqlite r ref t1 t2 t3 st n
  printf '%s\n' "$P25_SESS2_SH" | dxi bash -c 'umask 077; cat > /root/hc-sess2.sh && chmod 700 /root/hc-sess2.sh' \
    || die "could not ship the second session helper into $NAME"
  r=$(sess login churn@example.test /root/.hc-login-pass "$jar") || r="HTTP none error"
  key_line "FIXTURE-LOGIN: $r"
  ref=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select '/media/' || website_id || '/' || filename from media where original_name like 'churn-prep-%' order by id limit 1" | head -n 1)
  [ -n "$ref" ] || die "no upload of prep in $NAME to put on a page"
  # Octal escapes, so that the bytes are UTF-8 whatever the locale of this shell is.
  t1=$(printf 'Gr\303\274\303\237e aus Z\303\274rich')
  t2=$(printf 'Ein Bild und ein Link')
  t3=$(printf 'Liste \342\200\223 ver\303\266ffentlicht?')
  printf '# Gr\303\274\303\237e aus Z\303\274rich\n\n\303\204pfel, \303\226l, \303\234bung: gr\303\266\303\237er, sch\303\266ner \342\200\223 und [holzcloud](https://holzcloud.ch/).\n' \
    | dxi bash -c 'umask 077; cat > /root/.hc-md-1.md'
  printf 'Ein Bild: ![Beschreibung](%s)\n\nUnd ein [innerer Link](/compat-gruesse) mit Umlaut: K\303\244se.\n' "$ref" \
    | dxi bash -c 'umask 077; cat > /root/.hc-md-2.md'
  printf '## Liste\n\n- eins\n- zwei \303\244\n- drei\n\n`code` und **fett**, Preis: 5 \342\202\254 & mehr.\n' \
    | dxi bash -c 'umask 077; cat > /root/.hc-md-3.md'
  r=$(dxo bash /root/hc-sess2.sh pagemd "$wid" "$jar" compat-gruesse "$t1" draft /root/.hc-md-1.md) || r="HTTP none error"
  key_line "FIXTURE-PAGE-1: compat-gruesse $r"
  r=$(dxo bash /root/hc-sess2.sh pagemd "$wid" "$jar" compat-bild "$t2" draft /root/.hc-md-2.md) || r="HTTP none error"
  key_line "FIXTURE-PAGE-2: compat-bild $r"
  r=$(dxo bash /root/hc-sess2.sh pagemd "$wid" "$jar" compat-liste "$t3" published /root/.hc-md-3.md) || r="HTTP none error"
  key_line "FIXTURE-PAGE-3: compat-liste (published requested) $r"
  dx rm -f /root/.hc-md-1.md /root/.hc-md-2.md /root/.hc-md-3.md || true
  dx bash -c 'umask 077; cat /root/hc-base.jpg > /root/.hc-compat-up.jpg; printf "trailer compat %s" "$(date +%s%N)" >> /root/.hc-compat-up.jpg'
  r=$(sess upload "$wid" "$jar" /root/.hc-compat-up.jpg churn-compat-extra.jpg) || r="HTTP none error"
  dx rm -f /root/.hc-compat-up.jpg "$jar" || true
  key_line "FIXTURE-UPLOAD: churn-compat-extra.jpg $r"
  sleep 1
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select count(*) from pages where slug like 'compat-%'") || n="?"
  key_line "FIXTURE-PAGES-SAVED: $n"
  if [ "$n" = 3 ]; then check_pass "the three fixture pages exist on the previous release"; else check_bad "the fixture pages: $n rows with a slug compat-%, expected 3"; fi
  st=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select status from pages where slug = 'compat-liste'") || st="?"
  key_line "FIXTURE-STATUS-compat-liste: ${st:-none} (requested: published; an editor may or may not publish, this is what the database says)"
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select count(*) from media where original_name = 'churn-compat-extra.jpg'") || n="?"
  if [ "$n" = 1 ]; then check_pass "the extra upload has its media row"; else check_bad "the extra upload: $n media rows, expected 1"; fi
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select count(*) from media_usage u join pages p on p.id = u.page_id where p.slug = 'compat-bild'") || n="?"
  key_line "FIXTURE-MEDIA-USAGE-ROWS-compat-bild: $n"
  n=$(dxo runuser -u holzcloud -- sqlite3 -readonly "$db" "select count(*) from pages where slug like 'compat-%' and content_html like '%holzcloud.ch%'") || n="?"
  key_line "FIXTURE-PAGES-WITH-OUTBOUND-LINK: $n"
}

# compat_ship_sess CONTAINER: the session helper of plan 01 into a container.
compat_ship_sess() {
  printf '%s\n' "$P25_SESS_SH" | docker exec -i "$1" bash -c 'umask 077; cat > /root/hc-sess.sh && chmod 700 /root/hc-sess.sh' \
    || die "could not ship the session helper into $1"
}
# tsess ARGS...: the session helper in the second container; prints "HTTP CODE REDIRECT".
tsess() { docker exec "$TGT_NAME" bash /root/hc-sess.sh "$@" | tr -d '\r'; }

cmd_compat_source() { (
  compat_names
  compat_source_run
); }

compat_source_run() {
  local ref ver info bd=${BIN_DIR:-deploy/test/out/bin}
  [ "$TARGET" = systemd-base ] \
    || die "compat-source needs TARGET=systemd-base (got '$TARGET'): the install is played out of the document into a bare systemd image"
  git rev-parse --verify --quiet "${COMPAT_TAG}^{commit}" > /dev/null \
    || die "COMPAT_TAG '$COMPAT_TAG' is not a tag or commit of this repository"
  ref=$(pin_ref)
  say "compat-source: the previous release $COMPAT_TAG is built from its tag, installed in $NAME and populated; this release is NEW_REF $ref; evidence in $OUT_DIR"
  # No update step: a source that was updated runs the new binary and answers nothing.
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR INSTALL_FROM=old OLD_TAG=$COMPAT_TAG NEW_REF=$ref BULK_MB=0 KEEP=1 \
    bash deploy/test/run.sh build up install baseline postinstall backup-fixture
  ver=$(dxo /opt/holzcloud/holzcloud version) || die "holzcloud version failed in $NAME"
  version_is_tag "$ver" "$COMPAT_TAG" \
    || die "$NAME runs '$ver', which is not exactly the tag $COMPAT_TAG (version_is_tag): the previous release was not built from the tag"
  info=$bd/holzcloud-old-$COMPAT_TAG.info
  [ -f "$info" ] || die "$info does not exist: bins_build writes it next to the binary"
  {
    cat "$info"
    echo "container-version: $ver"
  } > "$OUT_DIR/compat-binary.txt"
  say "compat-source: $ver, binary details in $OUT_DIR/compat-binary.txt"
  say "COMPAT-SOURCE OK"
}

cmd_compat() { (
  compat_names
  compat_run
); }

# compat: CON-02, played on the containers compat-source and the fresh box leave.
compat_run() {
  local o=$OUT_DIR/compat.txt w=$OUT_DIR/.work-compat ref ver bl rl dir man mver brc rrc tdir wr n
  local tag_sha new_sha inst_sha bsh setup_b setup_a lv_b lv_a hi_new hi_tag n_new n_tag adm ed code f ms mp msha ssha mrow
  local mok mtot res aside au pu np nb
  [ "$TARGET" = systemd-base ] || die "compat needs TARGET=systemd-base (got '$TARGET')"
  need_container
  ref=$(pin_ref)
  ver=$(dxo /opt/holzcloud/holzcloud version) || die "holzcloud version failed in $NAME"
  version_is_tag "$ver" "$COMPAT_TAG" \
    || die "$NAME runs '$ver', not exactly the tag $COMPAT_TAG; play compat-source first (T-25-05)"
  if docker container inspect "$TGT_NAME" > /dev/null 2>&1; then
    n=$(dxto runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from users" 2> /dev/null) || n="?"
    [ "$n" = 0 ] || die "$TGT_NAME exists and holds users ('$n'): a restore leaves the box restored, so compat needs a fresh one. Bring one up with: docker rm -f $TGT_NAME; PLATFORM=$PLATFORM TARGET=$TARGET NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR INSTALL_FROM=new NEW_REF=$ref KEEP=1 bash deploy/test/run.sh build up install"
  fi
  bl=$(doc_backup_line) || die "deploy/DEPLOY.md has no line 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups'"
  rl=$(doc_restore_line) || die "deploy/DEPLOY.md has no restore line 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  bl=${bl#*:}
  rl=${rl#*:}

  # (1) The site holds something a person made: the editor of plan 01's prep makes a
  # page and two uploads through the previous release's own admin forms.
  cmd_prep
  [ "$(tail -n 1 "$OUT_DIR/prep.txt" 2> /dev/null)" = "PREP OK" ] || die "$OUT_DIR/prep.txt does not end with PREP OK"
  BADS=0
  res_begin "$o"
  rm -rf "$w"
  mkdir -p "$w"
  key_line "COMPAT-TAG: $COMPAT_TAG ($(git rev-parse "${COMPAT_TAG}^{commit}"))"
  key_line "NEW-REF: $ref"
  key_line "OLD-BINARY: $ver"
  check_pass "version_is_tag: the source container runs exactly the tag $COMPAT_TAG"
  key_line "HOST-ARCH: $(docker info --format '{{.Architecture}}')"
  key_line "CONTAINER-ARCH: $(dxo uname -m)"
  compat_fixture "$(p25_wid)"
  compat_state "$NAME" live /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "$OUT_DIR/compat-state-source.txt" \
    || { check_bad "STATE-SOURCE could not be read from $NAME"; }
  key_line "STATE-SOURCE: $(sed -n '/^@@KEYS$/,/^@@GOOSE$/p' "$OUT_DIR/compat-state-source.txt" | grep -v '^@@' | tr '\n' ' ')"
  if compat_fixture_ok "$(kv "$OUT_DIR/compat-state-source.txt" PAGES-COUNT)" "$(kv "$OUT_DIR/compat-state-source.txt" USERS-COUNT)" "$(kv "$OUT_DIR/compat-state-source.txt" UPLOADED-MEDIA)"; then
    check_pass "the staged site is not trivial: at least three pages, two users and two uploaded media rows"
  else
    check_bad "the staged site is trivial (pages $(kv "$OUT_DIR/compat-state-source.txt" PAGES-COUNT), users $(kv "$OUT_DIR/compat-state-source.txt" USERS-COUNT), uploaded media rows $(kv "$OUT_DIR/compat-state-source.txt" UPLOADED-MEDIA)): a comparison of two nearly empty sites proves nothing"
  fi

  # (2) Whose backup.sh: the blob at the tag against the installed file and this release's.
  tag_sha=$(git show "$COMPAT_TAG:deploy/backup.sh" | shasum -a 256 | awk '{ print $1 }')
  new_sha=$(git show "$ref:deploy/backup.sh" | shasum -a 256 | awk '{ print $1 }')
  inst_sha=$(dxo sha256sum /opt/holzcloud/deploy/backup.sh | awk '{ print $1 }')
  key_line "BACKUP-SH-SHA256-TAG: $tag_sha"
  key_line "BACKUP-SH-SHA256-NEW-REF: $new_sha"
  key_line "BACKUP-SH-SHA256-INSTALLED: $inst_sha"
  if [ "$inst_sha" = "$tag_sha" ]; then
    bsh=identical
  else
    git show "$COMPAT_TAG:deploy/backup.sh" | dxi bash -c 'cat > /opt/holzcloud/deploy/backup.sh && chmod 755 /opt/holzcloud/deploy/backup.sh' \
      || die "could not install the tag's backup.sh into $NAME"
    inst_sha=$(dxo sha256sum /opt/holzcloud/deploy/backup.sh | awk '{ print $1 }')
    [ "$inst_sha" = "$tag_sha" ] || die "the installed backup.sh still differs from the tag's after the replacement"
    bsh=replaced
  fi
  key_line "BACKUP-SH: $bsh"
  if [ "$tag_sha" = "$new_sha" ]; then
    key_line "BACKUP-SH-TAG-EQUALS-NEW-REF: yes"
  else
    key_line "BACKUP-SH-TAG-EQUALS-NEW-REF: no"
  fi

  # (3) The document's manual backup line, as the service user, by the previous release.
  key_line "BACKUP-LINE: $bl"
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$bl" > "$w/backup.txt" 2>&1 || true
  cp "$w/backup.txt" "$OUT_DIR/compat-backup.txt"
  brc=$(sed -n 's/^RC //p' "$w/backup.txt" | tail -n 1)
  dir=$(sed -n 's/^[0-9]* === Backup complete: \(.*\) ===$/\1/p' "$w/backup.txt" | tail -n 1)
  key_line "BACKUP-RC: ${brc:-none}"
  key_line "BACKUP-DIR: ${dir:-none}"
  if [ "$brc" != 0 ] || [ -z "$dir" ] || ! dx test -f "$dir/MANIFEST"; then
    check_bad "the previous release's backup.sh did not complete (status ${brc:-none}, directory '${dir:-none}'); the transcript ends: $(tail -n 4 "$w/backup.txt" | tr '\n' '|')"
    key_line "COMPAT FAILED"
    exit 1
  fi
  man=$(dxo cat "$dir/MANIFEST")
  key_line "MANIFEST: $(printf '%s' "$man" | tr '\n' ' ')"
  mver=$(printf '%s\n' "$man" | sed -n 's/^version=//p')
  if version_is_tag "$mver" "$COMPAT_TAG"; then
    check_pass "the MANIFEST of the backup names the tag: version=$mver"
  else
    check_bad "the MANIFEST of the backup says version=$mver, not the tag $COMPAT_TAG"
  fi
  compat_state "$NAME" file "$dir/holzcloud.sqlite" "$dir/media" "$OUT_DIR/compat-state-backup.txt" \
    || { check_bad "STATE-BACKUP could not be read from $dir"; }

  # (4) The fresh box with this release, if it is not there yet.
  if ! docker container inspect "$TGT_NAME" > /dev/null 2>&1; then
    say "compat: bringing up the fresh box $TGT_NAME with this release ($ref)"
    PLATFORM=$PLATFORM TARGET=$TARGET NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR INSTALL_FROM=new NEW_REF=$ref KEEP=1 \
      bash deploy/test/run.sh build up install > "$OUT_DIR/compat-new-up.log" 2>&1 \
      || die "the fresh box $TGT_NAME could not be brought up; see $OUT_DIR/compat-new-up.log"
  fi
  key_line "NEW-BINARY: $(dxto /opt/holzcloud/holzcloud version)"
  compat_ship_sess "$TGT_NAME"
  setup_b=$(tsess get /admin/setup /root/.hc-jar-compat-probe) || setup_b="HTTP none error"
  dxt rm -f /root/.hc-jar-compat-probe || true
  key_line "SETUP-BEFORE: $setup_b"
  if [ "$setup_b" = "HTTP 200 -" ]; then
    check_pass "the setup form answers 200 on the fresh box before the restore"
  else
    check_bad "the setup form on the fresh box before the restore: '$setup_b', expected 'HTTP 200 -'"
  fi

  # (5) The backup directory to the fresh box: run.sh's public transfer.
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR TGT_NAME=$TGT_NAME TGT_OUT_DIR=$TGT_OUT_DIR BACKUP_DIR_PATH=$dir KEEP=1 \
    bash deploy/test/run.sh transfer > "$OUT_DIR/compat-transfer.log" 2>&1 || true
  if grep -q '^TRANSFER: equal' "$TGT_OUT_DIR/transfer.txt" 2> /dev/null; then
    check_pass "transfer: $(grep '^FILES:' "$TGT_OUT_DIR/transfer.txt" | head -n 1) files, sha256 equal on both sides"
  else
    check_bad "transfer did not end TRANSFER: equal; see $OUT_DIR/compat-transfer.log"
    key_line "COMPAT FAILED"
    exit 1
  fi

  # (6) This release's restore.sh, the document's restore line, yes on stdin.
  tdir=/opt/holzcloud/backups/$(basename "$dir")
  wr=$(printf '%s' "$rl" | sed "s|/opt/holzcloud/backups/[0-9-]*\$|$tdir|")
  key_line "RESTORE-LINE: $wr"
  deviation "DEVIATION[start-limit]: the shipped holzcloud.service allows 5 starts within 300 s; the fresh box was started by its install and is started again by the restore and by run.sh check, so 'systemctl reset-failed holzcloud' is run on it before each. The unit is not changed and no sandbox directive is touched"
  dxt systemctl reset-failed holzcloud || true
  printf 'yes\n' | docker exec -i "$TGT_NAME" bash -c "$P25_STAMP_SH" _ "$wr" > "$w/restore.txt" 2>&1 || true
  cp "$w/restore.txt" "$OUT_DIR/compat-restore.txt"
  rrc=$(sed -n 's/^RC //p' "$w/restore.txt" | tail -n 1)
  key_line "RESTORE-RC: ${rrc:-none}"
  key_line "RESTORE-WALL-S: $(awk '$1 == "WALL" { printf "%.1f", ($3 - $2) / 1000 }' "$w/restore.txt")"
  if [ "$rrc" = 0 ] && grep -q '=== Restore complete ===' "$w/restore.txt"; then
    check_pass "restore.sh of this release ended 0 with '=== Restore complete ===' on the previous release's backup"
  else
    check_bad "restore.sh of this release ended ${rrc:-none} on the previous release's backup; the transcript ends: $(tail -n 4 "$w/restore.txt" | tr '\n' '|')"
    key_line "COMPAT FAILED"
    exit 1
  fi

  # (7) The far side.
  compat_state "$TGT_NAME" live /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "$OUT_DIR/compat-state-restored.txt" \
    || { check_bad "STATE-RESTORED could not be read from $TGT_NAME"; }
  key_line "STATE-RESTORED: $(sed -n '/^@@KEYS$/,/^@@GOOSE$/p' "$OUT_DIR/compat-state-restored.txt" | grep -v '^@@' | tr '\n' ' ')"
  compat_compare source-restored "$OUT_DIR/compat-state-source.txt" "$OUT_DIR/compat-state-restored.txt"
  compat_compare backup-restored "$OUT_DIR/compat-state-backup.txt" "$OUT_DIR/compat-state-restored.txt"
  compat_tamper_control "$OUT_DIR/compat-state-source.txt" "$OUT_DIR/compat-state-restored.txt" "$w"

  # The restored data directory against the backup directory it came from, and the
  # database and media of the restored site in both directions (plan 01's comparator).
  printf '%s\n' "$P25_COMPARE_SH" | docker exec -i "$TGT_NAME" bash -s -- restored /opt/holzcloud/data/holzcloud.sqlite /opt/holzcloud/data/media "$tdir" /opt/holzcloud/data > "$w/cmp.out" 2> "$w/cmp.err" || true
  mkdir -p "$w/cmp"
  split_sections "$w/cmp.out" "$w/cmp"
  cp "$w/cmp.out" "$OUT_DIR/compat-compare-raw.txt"
  if [ -s "$w/cmp/ROWS" ] && grep -q . "$w/cmp/FILES" 2> /dev/null; then
    res=$(media_agree "$w/cmp/ROWS" "$w/cmp/FILES" '*/bulk/') || res="0 0 0 0 ?"
    set -- $res
    key_line "RESTORED-MEDIA: rows $1 files $2 rows-without-file $3 files-without-row $4 class $5"
    if [ "$5" = AGREE ]; then check_pass "media rows and media files of the restored box agree in both directions"; else check_bad "media rows and files of the restored box: class $5"; fi
  else
    check_bad "the comparator returned no rows or no files for the restored box (cmp.err: $(head -c 300 "$w/cmp.err" | tr '\n' '|'))"
  fi
  if [ "$(sed -n 's/^CHECK-RC: //p' "$w/cmp/CHECK")" = 0 ]; then check_pass "holzcloud check on a copy of the restored database: exit 0"; else check_bad "holzcloud check on a copy of the restored database: $(sed -n 's/^CHECK-OUT: //p' "$w/cmp/CHECK" | cut -c1-200)"; fi
  if [ "$(sed -n 's/^INTEGRITY: //p' "$w/cmp/CHECK" | tr -d ' ')" = ok ]; then check_pass "PRAGMA integrity_check on the restored database: ok"; else check_bad "PRAGMA integrity_check on the restored database: $(sed -n 's/^INTEGRITY: //p' "$w/cmp/CHECK" | cut -c1-200)"; fi
  if [ "$(sed -n 's/^FKCHECK: //p' "$w/cmp/CHECK")" = 0 ]; then check_pass "PRAGMA foreign_key_check on the restored database: no violation"; else check_bad "PRAGMA foreign_key_check on the restored database: $(sed -n 's/^FKCHECK: //p' "$w/cmp/CHECK") violation(s)"; fi
  if [ "$(grep -c . "$w/cmp/TREE-RESTORED" || true)" -gt 0 ] && cmp -s "$w/cmp/TREE-RESTORED" "$w/cmp/TREE-BACKUP"; then
    check_pass "the sha256 list of the restored data directory ($(grep -c . "$w/cmp/TREE-RESTORED" || true) files) equals that of the backup directory it came from"
  else
    check_bad "the sha256 list of the restored data directory differs from that of the backup directory ($(grep -c . "$w/cmp/TREE-RESTORED" || true) and $(grep -c . "$w/cmp/TREE-BACKUP" || true) files)"
    diff "$w/cmp/TREE-RESTORED" "$w/cmp/TREE-BACKUP" 2> /dev/null | head -n 10 | sed 's/^/KEY TREE-DIFF: /' | tee -a "$CHECK_OUT" > /dev/null || true
  fi
  # The box was fresh: the set-aside directory is the empty database of the first start.
  aside=$(dxto bash -c 'ls -1d /opt/holzcloud/data.before-restore-* 2>/dev/null | head -n 1') || aside=""
  au=$(dxt sqlite3 "file:$aside/holzcloud.sqlite?immutable=1" "select count(*) from users" 2>&1 | tr -d '\r') || au="unreadable"
  key_line "SET-ASIDE: ${aside:-none} users $au"
  if [ -n "$aside" ] && [ "$au" = 0 ] && [ "$setup_b" = "HTTP 200 -" ]; then
    check_pass "the restored box was fresh: the setup form was 200 and the set-aside database is the empty one of the first start (users 0)"
  else
    check_bad "the restored box was not fresh: set-aside '${aside:-none}' users '$au', setup form before '$setup_b'"
  fi
  pu=$(dxto bash -c 'find /opt/holzcloud/data -maxdepth 1 -name "pre-upgrade-*" | wc -l') || pu="?"
  key_line "PRE-UPGRADE-SNAPSHOT-AFTER: $pu"
  check_info "a pre-upgrade snapshot appears in the data directory when the service finds a migration pending at its start; this pair may have none pending, so the number says nothing about how a restore handles one"
  setup_a=$(tsess get /admin/setup /root/.hc-jar-compat-probe) || setup_a="HTTP none error"
  key_line "SETUP-AFTER: $setup_a"
  if [ "$setup_a" = "HTTP 404 -" ]; then
    check_pass "the setup form answers 404 after the restore (the restored database has users)"
  else
    check_bad "the setup form after the restore: '$setup_a', expected 'HTTP 404 -'"
  fi
  adm=$(tsess login login@example.test /root/.hc-login-pass /root/.hc-jar-compat-a) || adm="HTTP none error"
  ed=$(tsess login churn@example.test /root/.hc-login-pass /root/.hc-jar-compat-e) || ed="HTTP none error"
  dxt rm -f /root/.hc-jar-compat-probe /root/.hc-jar-compat-a /root/.hc-jar-compat-e || true
  key_line "LOGIN-ADMIN: $adm"
  key_line "LOGIN-EDITOR: $ed"
  if [ "$adm" = "HTTP 303 /admin/" ] && [ "$ed" = "HTTP 303 /admin/" ]; then
    check_pass "both accounts of the previous release log in on the restored box: HTTP 303 /admin/"
  else
    check_bad "logins on the restored box: admin '$adm', editor '$ed', expected 'HTTP 303 /admin/' for both"
  fi
  code=$(dxto curl -s -o /dev/null -w '%{http_code}' --max-time 30 -H 'Host: test.local' http://127.0.0.1:8080/plugintest) || code=error
  key_line "PLUGINTEST: HTTP $code"
  if [ "$code" = 200 ]; then check_pass "/plugintest answers 200 for test.local on the restored box"; else check_bad "/plugintest answered $code on the restored box"; fi
  mok=0
  mtot=0
  while IFS= read -r mrow; do
    [ -n "$mrow" ] || continue
    mtot=$((mtot + 1))
    mp=${mrow%%|*}
    ssha=${mrow#*|}
    msha=$(dxt bash -c 'curl -s --max-time 30 -H "Host: test.local" "http://127.0.0.1:8080/media/$1" | sha256sum | cut -d" " -f1' _ "$mp" | tr -d '\r') || msha=error
    key_line "MEDIA-SERVED: $mp source=$ssha served=$msha"
    if [ "$ssha" != MISSING ] && [ "$ssha" = "$msha" ]; then
      mok=$((mok + 1))
    else
      check_bad "the media original $mp: served sha256 '$msha', the source's file '$ssha'"
    fi
  done < <(sed -n '/^@@MEDIAORIG$/,/^@@END$/p' "$OUT_DIR/compat-state-source.txt" | grep -v '^@@')
  key_line "MEDIA-SERVED-EQUAL: $mok of $mtot"
  if [ "$mtot" -ge 1 ] && [ "$mok" = "$mtot" ]; then
    check_pass "every media original is served over HTTP with the sha256 of the source's file ($mok of $mtot)"
  else
    check_bad "media originals served with the source's sha256: $mok of $mtot"
  fi

  # The migration level, read twice.
  lv_b=$(kv "$OUT_DIR/compat-state-backup.txt" LEVEL)
  lv_a=$(kv "$OUT_DIR/compat-state-restored.txt" LEVEL)
  set -- $(git_migrations "$ref")
  n_new=$1
  hi_new=$2
  set -- $(git_migrations "${COMPAT_TAG}^{commit}")
  n_tag=$1
  hi_tag=$2
  key_line "MIGRATIONS-AT-TAG: $n_tag"
  key_line "MIGRATIONS-HIGHEST-AT-TAG: $hi_tag"
  key_line "MIGRATIONS-AT-NEW-REF: $n_new"
  key_line "MIGRATIONS-HIGHEST-AT-NEW-REF: $hi_new"
  key_line "LEVEL-BEFORE: $lv_b"
  key_line "LEVEL-AFTER: $lv_a"
  if migration_ok "$lv_b" "$lv_a" "$hi_new"; then
    check_pass "migration_ok: before $lv_b, after $lv_a, highest at $ref $hi_new"
    key_line "MIGRATION-SENTENCE: $(migration_sentence "$lv_b" "$lv_a")"
  else
    check_bad "migration_ok failed: before '$lv_b', after '$lv_a', highest at $ref '$hi_new'"
  fi
  key_line "GOOSE-ROWS-BEFORE: $(kv "$OUT_DIR/compat-state-backup.txt" GOOSE-ROWS)"
  key_line "GOOSE-ROWS-AFTER: $(kv "$OUT_DIR/compat-state-restored.txt" GOOSE-ROWS)"
  check_info "this pair crosses $((hi_new - lv_b)) migration(s) by the levels read; a pair with no pending migration does not show how a restore handles one, that stays on the Not tested list of the record and migration_ok's table is what keeps the judgement right when a later release does cross one"

  # What changed between the two releases in code that handles data, listed and not summarised.
  key_line "COMPAT-DIFF-COMMAND: git diff --stat=200 $COMPAT_TAG $ref -- go.mod go.sum internal/db internal/media cmd"
  git diff --stat=200 "$COMPAT_TAG" "$ref" -- go.mod go.sum internal/db internal/media cmd | sed 's/^/COMPAT-DIFF /' | tee -a "$CHECK_OUT"
  for f in deploy/backup.sh deploy/restore.sh deploy/holzcloud-backup.service deploy/holzcloud-backup.timer; do
    tag_sha=$(git show "$COMPAT_TAG:$f" | shasum -a 256 | awk '{ print $1 }')
    new_sha=$(git show "$ref:$f" | shasum -a 256 | awk '{ print $1 }')
    if [ "$tag_sha" = "$new_sha" ]; then
      key_line "SCRIPTS-IDENTICAL: $f yes $tag_sha"
    else
      key_line "SCRIPTS-IDENTICAL: $f no tag=$tag_sha new-ref=$new_sha"
    fi
  done

  # The 17 assertions of run.sh check, last, because it restarts the service.
  dxt systemctl reset-failed holzcloud || true
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR KEEP=1 \
    bash deploy/test/run.sh check > "$OUT_DIR/compat-check.log" 2>&1 || true
  cp "$TGT_OUT_DIR/check.txt" "$OUT_DIR/compat-check.txt" 2> /dev/null || true
  np=$(grep -c '^PASS' "$TGT_OUT_DIR/check.txt" 2> /dev/null || true)
  nb=$(grep -c '^BAD' "$TGT_OUT_DIR/check.txt" 2> /dev/null || true)
  key_line "CHECK: $np of 17"
  key_line "CHECK-BAD-LINES: $nb"
  if [ "${np:-0}" = 17 ] && [ "${nb:-0}" = 0 ]; then
    check_pass "run.sh check on the restored box: 17 of 17 assertions, the five plugins loaded, running and invoked"
  else
    check_bad "run.sh check on the restored box: ${np:-0} of 17 passed, ${nb:-0} BAD line(s); see $TGT_OUT_DIR/check.txt"
  fi

  rm -rf "${w:?}"
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "COMPAT FAILED"
    say "nightly.sh: COMPAT FAILED, $BADS BAD line(s); see $o"
    exit 1
  fi
  key_line "COMPAT OK"
}

# down-con: remove the two containers of compat-source and compat. Safe when either is
# absent; it never touches NAME or TGT_NAME (the existing `down` does).
cmd_down_con() {
  local c
  for c in "$OLD_NAME" "$NEW_NAME"; do
    if docker container inspect "$c" > /dev/null 2>&1; then
      docker rm -f "$c" > /dev/null 2>&1 || say "down-con: could not remove $c"
    fi
  done
  say "down-con: $OLD_NAME and $NEW_NAME are gone"
}

# ---------------------------------------------------------------------------
# Phase 25, plan 02: CON-03. backup.sh refuses to start below MIN_FREE_MB (default
# 1024), and the document says that floor does not grow with the data: "or a backup can
# run out of space halfway". Nobody played it. capfull mounts a filesystem over the
# backup root that is above the floor and smaller than the backup needs, and records
# what the script, the shipped unit, the next night and restore.sh make of it.
# ---------------------------------------------------------------------------

# capfull_size_mb FLOOR MARGIN: the size of the nearly full filesystem, the floor plus
# a margin, so that the free space at the guard is above the floor.
capfull_size_mb() {
  is_num "$1" "$2" || return 1
  echo $(($1 + $2))
}

# capfull_data_ok DATA_MB FS_MB FLOOR: the construction that is asked for and nothing
# else: the filesystem is at least the floor (the guard passes) and the data is larger
# than the filesystem (the copy cannot fit).
capfull_data_ok() {
  is_num "$1" "$2" "$3" || return 1
  [ "$2" -ge "$3" ] && [ "$1" -gt "$2" ]
}

# capfull_complete_p SRC_BYTES SRC_FILES BK_BYTES BK_FILES: the copy holds as many files
# and as many bytes as the source it was taken from. Fewer of either is not complete.
capfull_complete_p() {
  is_num "$1" "$2" "$3" "$4" || return 1
  [ "$3" -ge "$1" ] && [ "$4" -ge "$2" ]
}

# capfull_classify GUARD RC MANIFEST PARTIAL_DIR COMPLETE: the outcome class of one run of
# backup.sh from numbers that were recorded. GUARD is passed or refused (the script's own
# message), RC the exit status, MANIFEST whether the directory this run made holds one,
# PARTIAL_DIR whether this run made a directory at all, COMPLETE whether that directory
# holds all of the source. Prints one of
#   COMPLETE                  passed, 0, a MANIFEST, a directory, all of the source
#   GUARD-REFUSED             refused, non-zero, nothing made
#   FAILED-PARTIAL-DIR-LEFT   passed, non-zero, no MANIFEST, a directory left
#   FAILED-NO-DIR             passed, non-zero, no MANIFEST, no directory
#   FALSE-COMPLETE            passed, 0, a MANIFEST, a directory, less than the source
# and refuses (status 1, nothing printed) every other combination, a non-number and an
# answer that is not yes or no.
capfull_classify() {
  local g=$1 rc=$2 m=$3 p=$4 c=$5 x
  case "$g" in passed | refused) ;; *) return 1 ;; esac
  is_num "$rc" || return 1
  for x in "$m" "$p" "$c"; do
    case "$x" in yes | no) ;; *) return 1 ;; esac
  done
  if [ "$g" = refused ]; then
    [ "$rc" -ne 0 ] && [ "$m" = no ] && [ "$p" = no ] && [ "$c" = no ] || return 1
    echo GUARD-REFUSED
    return 0
  fi
  if [ "$rc" -eq 0 ]; then
    [ "$m" = yes ] && [ "$p" = yes ] || return 1
    if [ "$c" = yes ]; then echo COMPLETE; else echo FALSE-COMPLETE; fi
    return 0
  fi
  [ "$m" = no ] && [ "$c" = no ] || return 1
  if [ "$p" = yes ]; then echo FAILED-PARTIAL-DIR-LEFT; else echo FAILED-NO-DIR; fi
}

# capfull_robust_verdict CLASS DIR_MADE REMOVAL_LINE SENTINEL_INTACT: whether one failed
# night kept the promise of backup.sh (ROB-02): it left no directory of its own, said so
# on stderr, and did not touch the backup of an earlier night. The words each argument
# may take:
#   CLASS            COMPLETE, FALSE-COMPLETE, GUARD-REFUSED, FAILED-PARTIAL-DIR-LEFT or
#                    FAILED-NO-DIR (what capfull_classify printed)
#   DIR_MADE         none, or the name of the directory this run made (letters, digits,
#                    dot, dash, underscore)
#   REMOVAL_LINE     yes or no: the transcript holds the line "removed ..., which had no
#                    MANIFEST" (contract C-2 of plan 29-01)
#   SENTINEL_INTACT  yes, no or none-planted (the sentinel of the earlier night)
# Prints PASS, or FAIL followed by the first broken property, and returns 0 for both. It
# refuses (status 1, nothing printed) every argument that is not one of these words.
capfull_robust_verdict() {
  [ "$#" -eq 4 ] || return 1
  local c=$1 d=$2 r=$3 s=$4
  case "$c" in COMPLETE | FALSE-COMPLETE | GUARD-REFUSED | FAILED-PARTIAL-DIR-LEFT | FAILED-NO-DIR) ;; *) return 1 ;; esac
  case "$d" in none) ;; "" | *[!A-Za-z0-9._-]*) return 1 ;; esac
  case "$r" in yes | no) ;; *) return 1 ;; esac
  case "$s" in yes | no | none-planted) ;; *) return 1 ;; esac
  case "$c" in
    FAILED-PARTIAL-DIR-LEFT) echo "FAIL the night left a directory"; return 0 ;;
    COMPLETE | FALSE-COMPLETE) echo "FAIL the night completed where it should have failed"; return 0 ;;
    GUARD-REFUSED) echo "FAIL the guard refused, so the night under test never ran"; return 0 ;;
  esac
  if [ "$d" != none ]; then
    echo "FAIL the night made the directory $d and it is still there"
    return 0
  fi
  if [ "$r" != yes ]; then
    echo "FAIL the transcript has no line naming the removal of the directory"
    return 0
  fi
  case "$s" in
    no) echo "FAIL the backup of the earlier night was touched"; return 0 ;;
    none-planted) echo "FAIL no backup of an earlier night was planted, so nothing says it was left alone"; return 0 ;;
  esac
  echo PASS
}

# capfull_next_night_verdict GUARD CLASS: whether the night after a failed one was let
# through the free-space guard and failed again by itself, which is the case only when
# the failed night gave the space back. GUARD is passed or refused, CLASS a word of
# capfull_robust_verdict. PASS for passed and FAILED-NO-DIR; FAIL with a reason for
# every other combination; status 1 and nothing printed for a word outside the lists.
capfull_next_night_verdict() {
  [ "$#" -eq 2 ] || return 1
  local g=$1 c=$2
  case "$g" in passed | refused) ;; *) return 1 ;; esac
  case "$c" in COMPLETE | FALSE-COMPLETE | GUARD-REFUSED | FAILED-PARTIAL-DIR-LEFT | FAILED-NO-DIR) ;; *) return 1 ;; esac
  if [ "$g" = refused ]; then
    echo "FAIL the guard refused the next night: something still holds the space"
    return 0
  fi
  case "$c" in
    FAILED-NO-DIR) echo PASS ;;
    FAILED-PARTIAL-DIR-LEFT) echo "FAIL the next night passed the guard and left a directory" ;;
    GUARD-REFUSED) echo "FAIL the next night was classed as refused although the guard passed" ;;
    *) echo "FAIL the next night completed where it should have failed" ;;
  esac
}

CF_ROOT=/opt/holzcloud/backups

# cf_free: the free megabytes at the backup root, read the way backup.sh reads them.
cf_free() { dxo bash -c 'df -Pm "$1" | awk "NR==2 {print \$4}"' _ "$CF_ROOT"; }
# cf_ls: the names of the directories directly below the backup root, sorted.
cf_ls() { dxo bash -c 'find "$1" -mindepth 1 -maxdepth 1 -type d -printf "%f\n" | LC_ALL=C sort' _ "$CF_ROOT"; }
# cf_df LABEL: df -Pm and the mount line of the backup root, into capfull-df.txt.
cf_df() {
  {
    echo "== $1 ($(dxo date -u +%Y-%m-%dT%H:%M:%SZ)) =="
    dxo df -Pm "$CF_ROOT"
    dxo bash -c 'grep " /opt/holzcloud/backups " /proc/mounts || echo "(not a mount point)"'
  } >> "$OUT_DIR/capfull-df.txt"
}
# cf_mount SIZE_MB: a fresh tmpfs over the backup root, owned by the service user.
cf_mount() {
  dx mount -t tmpfs -o "size=${1}m,mode=0700,uid=$CF_UID,gid=$CF_GID" tmpfs "$CF_ROOT" \
    || die "could not mount a $1 MB tmpfs over $CF_ROOT in $NAME"
}
# cf_umount: take the filesystem away again (lazily if it is busy); the backups beneath are back.
cf_umount() {
  if dx bash -c 'grep -q " /opt/holzcloud/backups tmpfs " /proc/mounts'; then
    dx umount "$CF_ROOT" || dx umount -l "$CF_ROOT" || true
  fi
}
# capfull_cleanup: on every exit path.
capfull_cleanup() {
  docker exec "$NAME" bash -c 'if grep -q " /opt/holzcloud/backups tmpfs " /proc/mounts; then umount /opt/holzcloud/backups || umount -l /opt/holzcloud/backups; fi; rm -rf /root/cf-db-check' > /dev/null 2>&1 || true
}
# cf_stats DIR: "BYTES FILES" of the five directories a backup carries below DIR.
cf_stats() {
  dxo bash -c 'for n in media templates plugins sprachen marke; do if [ -d "$1/$n" ]; then find "$1/$n" -type f -printf "%s\n"; fi; done | awk "{ b += \$1; n++ } END { print b + 0, n + 0 }"' _ "$1"
}
# cf_sentinel: a directory named as yesterday's backup, with one file of known sha256,
# owned like a backup.
cf_sentinel() {
  local name
  name=$(dxo date -d yesterday +%Y%m%d-%H%M%S)
  CF_SENT=$CF_ROOT/$name
  dx bash -c 'set -e; mkdir "$1"; printf "the backup of an earlier night %s\n" "$2" > "$1/sentinel.txt"; chown -R holzcloud:holzcloud "$1"; chmod 700 "$1"; chmod 600 "$1/sentinel.txt"' _ "$CF_SENT" "$name" \
    || die "could not plant the sentinel directory $CF_SENT"
  CF_SENT_SHA=$(dxo sha256sum "$CF_SENT/sentinel.txt" | awk '{ print $1 }')
}
# cf_sentinel_intact: yes when the sentinel directory still holds the file with its sha256.
cf_sentinel_intact() {
  local now
  if [ -z "${CF_SENT:-}" ]; then
    echo none-planted
    return 0
  fi
  now=$(dxo sha256sum "$CF_SENT/sentinel.txt" 2> /dev/null | awk '{ print $1 }') || now=""
  if [ -n "$now" ] && [ "$now" = "$CF_SENT_SHA" ]; then echo yes; else echo no; fi
}
# cf_dbcheck DIR: the snapshot below DIR, copied away and judged: "present CHECK-RC INTEGRITY".
cf_dbcheck() {
  local out
  if ! dx test -f "$1/holzcloud.sqlite"; then
    echo "absent - -"
    return 0
  fi
  out=$(dxo bash -c 'rm -rf /root/cf-db-check; mkdir -p /root/cf-db-check; cp "$1/holzcloud.sqlite" /root/cf-db-check/holzcloud.sqlite; HOLZCLOUD_DATA_DIR=/root/cf-db-check /opt/holzcloud/holzcloud check > /root/cf-db-check/out 2>&1; echo "CHECK-RC $?"; echo "INTEGRITY $(sqlite3 /root/cf-db-check/holzcloud.sqlite "PRAGMA integrity_check" 2>&1 | tr "\n" " ")"; rm -rf /root/cf-db-check' _ "$1") || out=""
  echo "present $(printf '%s\n' "$out" | sed -n 's/^CHECK-RC //p') $(printf '%s\n' "$out" | sed -n 's/^INTEGRITY //p' | tr -d ' ')"
}

# cf_judge LABEL TRANSCRIPT RC BEFORE_LIST: what one run left. Reads the guard from the
# script's own message in TRANSCRIPT, the directory this run made from the listing before
# and after, and sets CF_GUARD CF_RC CF_NEW CF_MAN CF_PART CF_COMPLETE CF_DIRBYTES
# CF_DIRFILES CF_DB CF_RSYNC CF_CLASS CF_SENT_NOW CF_REMOVAL; the facts go to the file as KEY lines of LABEL.
cf_judge() {
  local label=$1 f=$2 rc=$3 before=$4 after nd d st sb sf n
  after=$(cf_ls)
  nd=$(comm -13 <(printf '%s\n' "$before" | LC_ALL=C sort) <(printf '%s\n' "$after" | LC_ALL=C sort) | head -n 1)
  if grep -q 'ERROR: only' "$f"; then CF_GUARD=refused; else CF_GUARD=passed; fi
  CF_RC=$rc
  CF_NEW=${nd:-none}
  CF_MAN=no
  CF_PART=no
  CF_COMPLETE=no
  CF_DIRBYTES=0
  CF_DIRFILES=0
  CF_DB="absent - -"
  CF_RSYNC=$(sed -n -E 's/.*rsync (error|warning): .*\(code ([0-9]+)\).*/\2/p' "$f" | head -n 1)
  CF_RSYNC=${CF_RSYNC:--}
  if [ -n "$nd" ]; then
    CF_PART=yes
    d=$CF_ROOT/$nd
    if dx test -f "$d/MANIFEST"; then CF_MAN=yes; fi
    CF_DIRBYTES=$(dxo du -sb "$d" | cut -f1)
    CF_DIRFILES=$(dxo bash -c 'find "$1" -type f | wc -l' _ "$d")
    st=$(cf_stats "$d")
    sb=${st% *}
    sf=${st#* }
    if [ "$CF_MAN" = yes ] && capfull_complete_p "$CF_SRC_BYTES" "$CF_SRC_FILES" "$sb" "$sf"; then CF_COMPLETE=yes; fi
    CF_DB=$(cf_dbcheck "$d")
    key_line "$label-PARTIAL-DIR-TREES: bytes $sb files $sf in the five directories, the source holds $CF_SRC_BYTES bytes in $CF_SRC_FILES files"
  fi
  CF_CLASS=$(capfull_classify "$CF_GUARD" "$rc" "$CF_MAN" "$CF_PART" "$CF_COMPLETE") || CF_CLASS=UNCLASSIFIED
  key_line "$label-GUARD: $CF_GUARD"
  key_line "$label-BACKUP-RC: $rc"
  key_line "$label-RSYNC-STATUS: $CF_RSYNC"
  key_line "$label-NO-SPACE-LINES: $(grep -c 'No space left on device' "$f" || true)"
  key_line "$label-DIRECTORY-MADE: $CF_NEW"
  key_line "$label-DIRECTORY-BYTES: $CF_DIRBYTES"
  key_line "$label-DIRECTORY-FILES: $CF_DIRFILES"
  key_line "$label-MANIFEST: $CF_MAN"
  key_line "$label-DATABASE-SNAPSHOT: $CF_DB (present, holzcloud check status, integrity_check)"
  CF_SENT_NOW=$(cf_sentinel_intact)
  key_line "$label-SENTINEL-INTACT: $CF_SENT_NOW"
  # The line backup.sh prints when it removes the directory of a night that failed before
  # its MANIFEST (contract C-2 of plan 29-01). A number from grep -c, so that a file that
  # cannot be read counts as no line.
  n=$(grep -c 'removed .*, which had no MANIFEST' "$f" 2> /dev/null || true)
  case "$n" in '' | *[!0-9]*) n=0 ;; esac
  if [ "$n" -gt 0 ]; then CF_REMOVAL=yes; else CF_REMOVAL=no; fi
  key_line "$label-REMOVAL-LINE: $CF_REMOVAL"
  key_line "$label-CLASS: $CF_CLASS"
  if [ "$CF_CLASS" = UNCLASSIFIED ]; then
    check_bad "$label: capfull_classify refused the recorded numbers (guard $CF_GUARD, status $rc, MANIFEST $CF_MAN, directory $CF_PART, complete $CF_COMPLETE)"
  fi
  tail -n 10 "$f" | sed "s/^/$label-LAST-LINES: /" | tee -a "$CHECK_OUT" > /dev/null
}

# capfull_report_robust LABEL: the verdict of capfull_robust_verdict on the facts cf_judge
# has just set, as the KEY line LABEL-ROBUST and a PASS or BAD line. A refusal of the
# words (which cf_judge's own UNCLASSIFIED would cause) is a BAD line, never a pass.
capfull_report_robust() {
  local v
  v=$(capfull_robust_verdict "$CF_CLASS" "$CF_NEW" "$CF_REMOVAL" "$CF_SENT_NOW") || v="FAIL the verdict refused the recorded words (class $CF_CLASS, directory $CF_NEW, removal line $CF_REMOVAL, sentinel $CF_SENT_NOW)"
  key_line "$1-ROBUST: $v"
  if [ "$v" = PASS ]; then
    check_pass "$1: the failed night ($CF_CLASS) left no directory, named the removal on stderr and left the sentinel of the earlier night intact ($CF_SENT_NOW)"
  else
    check_bad "$1: ${v#FAIL }"
  fi
}

cmd_capfull() { (
  trap capfull_cleanup EXIT
  capfull_run
); }

capfull_run() {
  local o=$OUT_DIR/capfull.txt shipped ref bl rl data_bytes need_bytes data_mb fs_mb ctl_mb st
  local before w=$OUT_DIR/.work-capfull rrc ans ia aside inv0 inv res em trig i state ls_before ls_after
  [ "$TARGET" = systemd-base ] || die "capfull needs TARGET=systemd-base (got '$TARGET')"
  need_container
  mkdir -p "$OUT_DIR"
  rm -rf "$w"
  mkdir -p "$w"
  : > "$OUT_DIR/capfull-df.txt"
  BADS=0
  res_begin "$o"
  bl=$(doc_backup_line) || die "deploy/DEPLOY.md has no line 'sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups'"
  rl=$(doc_restore_line) || die "deploy/DEPLOY.md has no restore line 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  bl=${bl#*:}
  rl=${rl#*:}
  CF_UID=$(dxo id -u holzcloud)
  CF_GID=$(dxo id -g holzcloud)
  [ -n "$CF_UID" ] && [ -n "$CF_GID" ] || die "no service user holzcloud in $NAME"

  # The floor is the shipped one, read from the installed script, and not passed on.
  shipped=$(dxo sed -n 's/^MIN_FREE_MB="${MIN_FREE_MB:-\([0-9]*\)}"$/\1/p' /opt/holzcloud/deploy/backup.sh)
  key_line "SHIPPED-MIN-FREE-MB: ${shipped:-?} (read from /opt/holzcloud/deploy/backup.sh)"
  [ "$shipped" = "$CAPFULL_FLOOR_MB" ] \
    || die "CAPFULL_FLOOR_MB=$CAPFULL_FLOOR_MB is not the shipped default ($shipped): the faithful case is the one to try first; a lowered floor needs MIN_FREE_MB passed to backup.sh and DEVIATION[floor], which this driver does not do"
  key_line "MIN-FREE-MB-IN-ENVIRONMENT: '$(docker exec "$NAME" printenv MIN_FREE_MB 2> /dev/null | tr -d '\r' || true)' (empty: backup.sh runs with its default); unit Environment: $(dxo systemctl show holzcloud-backup.service -p Environment --value)"
  st=$(show_v holzcloud-backup.service DropInPaths)
  key_line "UNIT-DROPINS: '$st' (empty: the shipped unit as it is)"
  if [ -n "$st" ]; then check_bad "holzcloud-backup.service has a drop-in ($st): capfull judges the shipped unit"; fi
  key_line "BACKUP-LINE: $bl"
  key_line "BINARY: $(dxo /opt/holzcloud/holzcloud version)"
  key_line "HOST-ARCH: $(docker info --format '{{.Architecture}}')"
  key_line "CONTAINER-ARCH: $(dxo uname -m)"
  ls_before=$(cf_ls)
  key_line "EARLIER-BACKUPS-BEFORE: $(printf '%s\n' "$ls_before" | grep -c . || true) ($(printf '%s' "$ls_before" | tr '\n' ' '))"

  # The data: bulk media so that the whole backup is larger than the free space at the guard.
  PLATFORM=$PLATFORM TARGET=$TARGET NAME=$NAME OUT_DIR=$OUT_DIR BULK_MB=$CAPFULL_DATA_MB KEEP=1 \
    bash deploy/test/run.sh backup-fixture > "$OUT_DIR/capfull-fixture.log" 2>&1 \
    || die "run.sh backup-fixture with BULK_MB=$CAPFULL_DATA_MB failed; see $OUT_DIR/capfull-fixture.log"
  key_line "BULK: $(tr '\n' ' ' < "$OUT_DIR/bulk.txt")"
  st=$(cf_stats /opt/holzcloud/data)
  CF_SRC_BYTES=${st% *}
  CF_SRC_FILES=${st#* }
  data_bytes=$(dxo du -sb /opt/holzcloud/data | cut -f1)
  need_bytes=$((CF_SRC_BYTES + $(dxo stat -c %s /opt/holzcloud/data/holzcloud.sqlite) + $(dxo bash -c 'stat -c %s /opt/holzcloud/data/csrf.key 2>/dev/null || echo 0')))
  data_mb=$(((need_bytes + 1048575) / 1048576))
  fs_mb=$(capfull_size_mb "$CAPFULL_FLOOR_MB" "$CAPFULL_MARGIN_MB") || die "capfull_size_mb refused FLOOR=$CAPFULL_FLOOR_MB MARGIN=$CAPFULL_MARGIN_MB"
  key_line "FLOOR-MB: $CAPFULL_FLOOR_MB"
  key_line "DATA-DIR-BYTES: $data_bytes (everything below /opt/holzcloud/data, the pre-upgrade snapshots included, which a backup does not carry)"
  key_line "SOURCE-TREES: bytes $CF_SRC_BYTES files $CF_SRC_FILES (media, templates, plugins, sprachen, marke)"
  key_line "DATA-MB: $data_mb (what a backup has to write: those trees, the database file and csrf.key)"
  key_line "FS-SIZE-MB: $fs_mb"
  capfull_data_ok "$data_mb" "$fs_mb" "$CAPFULL_FLOOR_MB" \
    || die "the construction fails before it starts: DATA-MB $data_mb, FS-SIZE-MB $fs_mb, FLOOR-MB $CAPFULL_FLOOR_MB (the filesystem must be at least the floor and smaller than the data); raise CAPFULL_DATA_MB or lower CAPFULL_MARGIN_MB"
  check_pass "capfull_data_ok: the filesystem of $fs_mb MB is at least the floor ($CAPFULL_FLOOR_MB MB) and smaller than the $data_mb MB a backup has to write"
  deviation "DEVIATION[fs]: the nearly full filesystem is a tmpfs, not a disk. It has no blocks reserved for root, no fragmentation and no journal, it lives in memory, and the data directory is on another filesystem than the backup root, whereas on a real server the two usually share one disk and a full disk stops the site's own writes as well. The error delivered to rsync is the same errno (ENOSPC, No space left on device)"
  deviation "DEVIATION[emulation]: the container is linux/amd64 emulated on an arm64 host; copy times say nothing about a server"

  # (1) The control: a roomy filesystem, the document's manual line.
  ctl_mb=$((data_mb + CAPFULL_FLOOR_MB + 400))
  cf_mount "$ctl_mb"
  cf_df "control before ($ctl_mb MB)"
  key_line "CONTROL-FS-SIZE-MB: $ctl_mb"
  key_line "CONTROL-FREE-AT-GUARD-MB: $(cf_free)"
  before=$(cf_ls)
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$bl" > "$OUT_DIR/capfull-control.txt" 2>&1 || true
  cf_df "control after"
  rrc=$(sed -n 's/^RC //p' "$OUT_DIR/capfull-control.txt" | tail -n 1)
  cf_judge CONTROL "$OUT_DIR/capfull-control.txt" "${rrc:-255}" "$before"
  CF_CONTROL_CLASS=$CF_CLASS
  if [ "$CF_CONTROL_CLASS" = COMPLETE ] && grep -q '=== Backup complete: ' "$OUT_DIR/capfull-control.txt"; then
    check_pass "the control completes on a roomy filesystem: exit 0, '=== Backup complete ===', all of the source"
  else
    check_bad "the control did not complete on a roomy filesystem (class $CF_CONTROL_CLASS); the stage would not be failing for the reason under test"
  fi
  key_line "CONTROL-CLASS: $CF_CONTROL_CLASS"
  cf_umount

  # (2) By hand on the nearly full filesystem.
  cf_mount "$fs_mb"
  cf_sentinel
  key_line "SENTINEL: $CF_SENT sha256 $CF_SENT_SHA"
  cf_df "hand before ($fs_mb MB)"
  CF_FREE=$(cf_free)
  key_line "FREE-AT-GUARD-MB: $CF_FREE (df -Pm at the backup root just before the run, the way backup.sh reads it; the floor is $CAPFULL_FLOOR_MB)"
  before=$(cf_ls)
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$bl" > "$OUT_DIR/capfull-hand.txt" 2>&1 || true
  cf_df "hand after"
  rrc=$(sed -n 's/^RC //p' "$OUT_DIR/capfull-hand.txt" | tail -n 1)
  cf_judge HAND "$OUT_DIR/capfull-hand.txt" "${rrc:-255}" "$before"
  CF_HAND_CLASS=$CF_CLASS
  CF_HAND_DIR=$CF_NEW
  key_line "GUARD: $CF_GUARD"
  key_line "BACKUP-RC: $CF_RC"
  key_line "PARTIAL-DIR: $CF_NEW"
  key_line "PARTIAL-DIR-BYTES: $CF_DIRBYTES"
  key_line "MANIFEST-IN-PARTIAL: $CF_MAN"
  key_line "SENTINEL-INTACT: $(cf_sentinel_intact)"
  key_line "HAND-CLASS: $CF_HAND_CLASS"
  if [ "$CF_GUARD" != passed ]; then
    check_bad "the guard refused on the nearly full filesystem (free $CF_FREE MB against a floor of $CAPFULL_FLOOR_MB): the stage did not get past the guard"
  fi
  case "$CF_HAND_CLASS" in
    COMPLETE) check_bad "the backup completed on the nearly full filesystem: the stage did not provoke the failure it is built for" ;;
    FALSE-COMPLETE) check_info "FALSE-COMPLETE: backup.sh ended 0 with a MANIFEST and less than the source; this is a defect candidate for plan 03 (class 4)" ;;
    FAILED-PARTIAL-DIR-LEFT) check_bad "backup.sh on the nearly full filesystem left the directory $CF_HAND_DIR without a MANIFEST: the property under test (a failed night removes its own directory) is broken" ;;
    *) check_info "backup.sh on the nearly full filesystem: $CF_HAND_CLASS" ;;
  esac
  capfull_report_robust HAND

  # (3) The next night on the same filesystem, unchanged.
  cf_df "again before"
  key_line "AGAIN-FREE-AT-GUARD-MB: $(cf_free)"
  before=$(cf_ls)
  docker exec "$NAME" bash -c "$P25_STAMP_SH" _ "$bl" > "$OUT_DIR/capfull-again.txt" 2>&1 || true
  cf_df "again after"
  rrc=$(sed -n 's/^RC //p' "$OUT_DIR/capfull-again.txt" | tail -n 1)
  cf_judge AGAIN "$OUT_DIR/capfull-again.txt" "${rrc:-255}" "$before"
  key_line "AGAIN-CLASS: $CF_CLASS"
  st=$(capfull_next_night_verdict "$CF_GUARD" "$CF_CLASS") || st="FAIL the verdict refused the recorded words (guard $CF_GUARD, class $CF_CLASS)"
  key_line "AGAIN-NEXT-NIGHT: $st"
  if [ "$st" = PASS ]; then
    check_pass "the next night was not refused by the guard and failed the same way without leaving a directory"
  else
    check_bad "the next night: ${st#FAIL }"
  fi
  capfull_report_robust AGAIN

  # (4) restore.sh pointed at the partial directory, with no on stdin.
  if [ "$CF_HAND_DIR" != none ]; then
    ans=$(printf '%s' "$rl" | sed "s|/opt/holzcloud/backups/[0-9-]*\$|$CF_ROOT/$CF_HAND_DIR|")
    key_line "RESTORE-PARTIAL-LINE: $ans (no is typed on stdin)"
    printf 'no\n' | docker exec -i "$NAME" bash -c "$P25_STAMP_SH" _ "$ans" > "$OUT_DIR/capfull-restore-partial.txt" 2>&1 || true
    rrc=$(sed -n 's/^RC //p' "$OUT_DIR/capfull-restore-partial.txt" | tail -n 1)
    ia=$(dxo systemctl is-active holzcloud || true)
    aside=$(dxo bash -c 'ls -1d /opt/holzcloud/data.before-restore-* 2>/dev/null | wc -l')
    key_line "RESTORE-PARTIAL-EXIT: ${rrc:-none}"
    key_line "RESTORE-PARTIAL-ANSWER: $(grep -v '^WALL \|^RC ' "$OUT_DIR/capfull-restore-partial.txt" | tail -n 3 | sed 's/^[0-9]* //' | tr '\n' '|')"
    key_line "SERVICE-ACTIVE-AFTER-RESTORE-PARTIAL: $ia"
    key_line "DATA-BEFORE-RESTORE-DIRECTORIES: $aside"
    if [ "$ia" = active ] && [ "$aside" = 0 ]; then
      check_pass "restore.sh on the partial directory with no on stdin replaced nothing: the service stayed active and no data.before-restore directory appeared"
    else
      check_bad "restore.sh on the partial directory: service $ia, $aside data.before-restore director(ies)"
    fi
  else
    key_line "RESTORE-PARTIAL-EXIT: not-run (the run by hand left no directory)"
  fi
  cf_umount

  # (5) The shipped unit on a fresh nearly full filesystem of the same size.
  cf_mount "$fs_mb"
  cf_sentinel
  cf_df "unit before ($fs_mb MB)"
  key_line "UNIT-FREE-AT-GUARD-MB: $(cf_free)"
  before=$(cf_ls)
  inv0=$(show_v holzcloud-backup.service InvocationID)
  dx systemctl reset-failed holzcloud-backup.service || true
  dx systemctl start --no-block holzcloud-backup.service || die "systemctl start holzcloud-backup.service failed"
  i=0
  inv=$inv0
  while [ "$i" -lt 30 ] && [ "$inv" = "$inv0" ]; do
    sleep 1
    inv=$(show_v holzcloud-backup.service InvocationID)
    i=$((i + 1))
  done
  i=0
  state=activating
  while [ "$i" -lt 180 ]; do
    state=$(show_v holzcloud-backup.service ActiveState)
    case "$state" in inactive | failed) break ;; esac
    sleep 5
    i=$((i + 1))
  done
  res=$(show_v holzcloud-backup.service Result)
  em=$(show_v holzcloud-backup.service ExecMainStatus)
  trig=$(show_v holzcloud-backup.service TriggeredBy)
  dxo journalctl "_SYSTEMD_INVOCATION_ID=$inv" --no-pager -o short-iso > "$OUT_DIR/capfull-unit.txt" 2>&1 || true
  cf_df "unit after"
  key_line "UNIT-INVOCATION: $inv (started by systemctl start, previous invocation '${inv0:-none}')"
  key_line "UNIT-ACTIVE-STATE: $state"
  key_line "UNIT-RESULT: $res"
  key_line "UNIT-EXECMAINSTATUS: $em"
  key_line "UNIT-TRIGGEREDBY: '${trig}'"
  key_line "UNIT-JOURNAL-LINES: $(grep -c . "$OUT_DIR/capfull-unit.txt" || true)"
  cf_judge UNIT "$OUT_DIR/capfull-unit.txt" "${em:-255}" "$before"
  key_line "UNIT-CLASS: $CF_CLASS"
  capfull_report_robust UNIT
  case "$state" in inactive | failed) ;; *) check_bad "the unit was still '$state' after the bounded poll" ;; esac
  cf_umount

  # (6) What the script did to the order of things, read and not assumed.
  check_info "backup.sh runs set -e and its retention step (find -mtime +RETENTION_DAYS) comes after the copies, so a run that ends in the copy never reaches it; that is a reading of the script. What is on disk is in the lines above: the sentinel of an earlier night, which is inside the retention window, shows nothing about deletion"
  ls_after=$(cf_ls)
  key_line "EARLIER-BACKUPS-AFTER: $(printf '%s\n' "$ls_after" | grep -c . || true) ($(printf '%s' "$ls_after" | tr '\n' ' '))"
  if [ "$ls_before" = "$ls_after" ]; then
    check_pass "the filesystem is unmounted and the earlier backups are back, the same directories as before"
  else
    check_bad "the backup root after the stage differs from before: '$(printf '%s' "$ls_before" | tr '\n' ' ')' and '$(printf '%s' "$ls_after" | tr '\n' ' ')'"
  fi
  if dx bash -c 'grep -q " /opt/holzcloud/backups " /proc/mounts'; then
    check_bad "something is still mounted at /opt/holzcloud/backups"
  fi

  rm -rf "${w:?}"
  leak_check
  if [ "$BADS" -gt 0 ]; then
    key_line "CAPFULL FAILED"
    say "nightly.sh: CAPFULL FAILED, $BADS BAD line(s); see $o"
    exit 1
  fi
  key_line "CAPFULL OK"
}

# ---------------------------------------------------------------------------
# Phase 25, plan 03: the whole evidence set in one command (con-chain), the verdicts
# it computes from the files, and the proof of the record (record-check-con).
# ---------------------------------------------------------------------------

# fut02_verdict ADDS DELETES: what the two stage outcomes of CON-01 say about FUT-02
# (an atomic snapshot). PROMOTE when either stage has a disagreement whose window could
# not be named (UNNAMED); NOT-NEEDED when both are a named outcome (AGREE-ALL,
# ORPHAN-ONLY-NAMED, ROW-WITHOUT-FILE-NAMED, or REFUSED-NAMED, a refused backup named by
# its rsync status); UNDECIDED otherwise (INCONCLUSIVE, RESTORE-FAILED, INCOMPLETE, empty
# or unknown: a stage that did not finish decides nothing about FUT-02).
fut02_verdict() {
  local o
  for o in "${1:-}" "${2:-}"; do
    if [ "$o" = UNNAMED ]; then
      echo PROMOTE
      return 0
    fi
  done
  for o in "${1:-}" "${2:-}"; do
    case "$o" in
      AGREE-ALL | ORPHAN-ONLY-NAMED | ROW-WITHOUT-FILE-NAMED | REFUSED-NAMED) ;;
      *)
        echo UNDECIDED
        return 0
        ;;
    esac
  done
  echo NOT-NEEDED
}

# fut02_line ADDS DELETES: the FUT-02 line of the summary, with the reason in terms of
# the outcomes.
fut02_line() {
  case "$(fut02_verdict "$1" "$2")" in
    PROMOTE) echo "FUT-02: PROMOTE (adds $1, deletes $2: a disagreement was seen whose window could not be named, so an atomic snapshot is a candidate requirement)" ;;
    NOT-NEEDED) echo "FUT-02: not needed (adds $1, deletes $2: every disagreement that occurred was reproduced and its window named)" ;;
    *) echo "FUT-02: undecided (adds ${1:-?}, deletes ${2:-?}: a stage that did not finish with a verdict decides nothing about FUT-02)" ;;
  esac
}

# owner_steps_ok FILE: the section `## Owner steps` of a record holds a numbered line
# (`1. ` or `O-1`) or a line that begins `None` followed by a reason. A section that is
# empty, one that holds only the word None, and a record without the heading fail.
# Prints ok, or the reason (status 1).
owner_steps_ok() {
  local f=$1 sec line rest
  if [ ! -f "$f" ]; then
    echo "owner steps: $f does not exist"
    return 1
  fi
  if ! grep -qx '## Owner steps' "$f"; then
    echo "owner steps: the record has no heading '## Owner steps'"
    return 1
  fi
  sec=$(awk '/^## Owner steps$/ { s = 1; next } s && /^## / { exit } s { print }' "$f")
  while IFS= read -r line; do
    case "$line" in
      [0-9].\ * | [0-9][0-9].\ * | O-[0-9]*)
        echo ok
        return 0
        ;;
      None[!A-Za-z0-9]*)
        rest=$(printf '%s' "${line#None}" | tr -cd '[:alnum:]')
        if [ "${#rest}" -ge 8 ]; then
          echo ok
          return 0
        fi
        ;;
    esac
  done <<< "$sec"
  echo "owner steps: the section holds neither a numbered step nor a line 'None' with a reason"
  return 1
}

# con01_verdict SC_PREP PREP SC_CONSIST CONSIST SC_WINDOW WINDOW [FLOOR]: PASS, or FAIL
# with the reason. PASS needs prep, consist and window to have ended 0 with their ok
# lines and no BAD line; MIN-RUNS in consist.txt at least FLOOR (30); the runs of each
# stage at least MIN-RUNS; and the runs of each stage that were actually restored and
# compared after the restore at least restore_required (RESTORED-RUNS-STAGE-x). A
# trace can therefore never pass for the set.
con01_verdict() {
  local scp=$1 fp=$2 scc=$3 fc=$4 scw=$5 fw=$6 floor=${7:-30} v min st runs rs req
  v=$(req_verdict "$scp" "$fp" "PREP OK")
  if [ "$v" != PASS ]; then
    echo "FAIL (prep: ${v#FAIL (}"
    return 0
  fi
  v=$(req_verdict "$scc" "$fc" "CONSIST OK")
  if [ "$v" != PASS ]; then
    echo "FAIL (consist: ${v#FAIL (}"
    return 0
  fi
  v=$(req_verdict "$scw" "$fw" "WINDOW OK")
  if [ "$v" != PASS ]; then
    echo "FAIL (window: ${v#FAIL (}"
    return 0
  fi
  min=$(kvq "$fc" MIN-RUNS)
  if ! is_num "$min" || [ "$min" -lt "$floor" ]; then
    echo "FAIL (MIN-RUNS in $fc is '$min', the evidence set needs $floor or more: a trace is not the set)"
    return 0
  fi
  for st in adds deletes; do
    runs=$(kvq "$fc" "CONSIST-RUNS-STAGE-$st")
    if ! is_num "$runs" || [ "$runs" -lt "$min" ]; then
      echo "FAIL (stage $st had '$runs' runs, fewer than MIN-RUNS $min)"
      return 0
    fi
    rs=$(kvq "$fc" "RESTORED-RUNS-STAGE-$st")
    req=$(restore_required "$runs" "$min")
    if ! is_num "$rs" || [ "$rs" -lt "$req" ]; then
      echo "FAIL (stage $st: '$rs' runs were restored and compared after the restore, the floor is $req)"
      return 0
    fi
  done
  echo PASS
}

# con_distinct A B C D: the four values are not empty and pairwise different. Prints the
# reason and returns 1 otherwise.
con_distinct() {
  local x
  if [ "$#" -ne 4 ]; then
    echo "con_distinct wants four values"
    return 1
  fi
  for x in "$@"; do
    if [ -z "$x" ]; then
      echo "one of the four is empty"
      return 1
    fi
  done
  if [ "$1" = "$2" ] || [ "$1" = "$3" ] || [ "$1" = "$4" ] || [ "$2" = "$3" ] || [ "$2" = "$4" ] || [ "$3" = "$4" ]; then
    echo "two of '$1' '$2' '$3' '$4' are the same"
    return 1
  fi
}

# con_dir_safe DIR [ROOT]: DIR is something the chain may fill and a person may remove:
# not empty, not / . .., not the repository root.
con_dir_safe() {
  local root=${2:-$ROOT}
  case "$1" in
    "" | / | . | .. | ./ | ../ | "$root" | "$root"/) return 1 ;;
  esac
}

# con_dir_empty DIR: DIR does not exist or holds nothing.
con_dir_empty() {
  [ ! -d "$1" ] || [ -z "$(ls -A "$1")" ]
}

# con_field LINE KEY: the value of "KEY: value" inside a line made of such pairs.
con_field() {
  printf ' %s' "$1" | sed -n "s/.* $2: \([^ ]*\).*/\1/p" | tail -n 1
}

# con_class FILE C: the CLASS= of the control line "CONTROL C ..." of window.txt.
con_class() {
  local v
  v=$(sed -n "s/^CONTROL $2 .* CLASS=\([A-Z-]*\) EXPECTED=.*/\1/p" "$1" 2> /dev/null | tail -n 1)
  printf '%s\n' "${v:-?}"
}

# con_classify RUNSFILE: one CLASSIFY line per stage and class other than AGREE in the
# runs file, with the numbers of those runs.
con_classify() {
  awk '
    $1 == "RUN" && $4 ~ /^CLASS=/ && $4 != "CLASS=AGREE" {
      k = $3 " " substr($4, 7)
      if (!(k in cnt)) order[++no] = k
      cnt[k]++
      nums[k] = nums[k] (nums[k] == "" ? "" : " ") $2
    }
    END { for (i = 1; i <= no; i++) { k = order[i]; split(k, a, " "); printf "CLASSIFY: consist stage %s: %d run(s) with CLASS=%s (runs %s)\n", a[1], cnt[k], a[2], nums[k] } }' "$1" 2> /dev/null
}

# con_classify_capfull FILE: one CLASSIFY line for every capfull class of FILE that is not
# COMPLETE (the control must be COMPLETE; the others are what is to be classified), a
# FALSE-COMPLETE spelled out as a defect candidate. (A function of its own: bash 3.2
# cannot parse a case pattern inside a command substitution.)
con_classify_capfull() {
  local k v
  for k in CONTROL-CLASS HAND-CLASS AGAIN-CLASS UNIT-CLASS; do
    v=$(kvq "$1" "$k")
    case "$v" in
      COMPLETE) ;;
      "?") echo "CLASSIFY: capfull $k is missing from $1" ;;
      FALSE-COMPLETE) echo "CLASSIFY: capfull $k is FALSE-COMPLETE: backup.sh ended 0 with a MANIFEST and less than the source; a defect candidate (class 4), to be read in capfull.txt before anything else" ;;
      *) echo "CLASSIFY: capfull $k is $v (see capfull.txt)" ;;
    esac
  done
}

selftest_con() {
  local t f c out rc
  t=$(mktemp -d)

  # fut02_verdict
  st_eq "fut02 both agree" NOT-NEEDED "$(fut02_verdict AGREE-ALL AGREE-ALL)"
  st_eq "fut02 orphan and row" NOT-NEEDED "$(fut02_verdict ORPHAN-ONLY-NAMED ROW-WITHOUT-FILE-NAMED)"
  st_eq "fut02 row and agree" NOT-NEEDED "$(fut02_verdict ROW-WITHOUT-FILE-NAMED AGREE-ALL)"
  st_eq "fut02 refused is named" NOT-NEEDED "$(fut02_verdict REFUSED-NAMED AGREE-ALL)"
  st_eq "fut02 adds unnamed" PROMOTE "$(fut02_verdict UNNAMED AGREE-ALL)"
  st_eq "fut02 deletes unnamed" PROMOTE "$(fut02_verdict ORPHAN-ONLY-NAMED UNNAMED)"
  st_eq "fut02 unnamed beats inconclusive" PROMOTE "$(fut02_verdict INCONCLUSIVE UNNAMED)"
  st_eq "fut02 inconclusive" UNDECIDED "$(fut02_verdict INCONCLUSIVE AGREE-ALL)"
  st_eq "fut02 restore failed" UNDECIDED "$(fut02_verdict AGREE-ALL RESTORE-FAILED)"
  st_eq "fut02 incomplete" UNDECIDED "$(fut02_verdict INCOMPLETE INCOMPLETE)"
  st_eq "fut02 empty" UNDECIDED "$(fut02_verdict "" AGREE-ALL)"
  st_eq "fut02 unknown word" UNDECIDED "$(fut02_verdict AGREE-ALL "?")"
  st_eq "fut02 none given" UNDECIDED "$(fut02_verdict)"
  st_eq "fut02 line promote" "FUT-02: PROMOTE (adds UNNAMED, deletes AGREE-ALL: a disagreement was seen whose window could not be named, so an atomic snapshot is a candidate requirement)" "$(fut02_line UNNAMED AGREE-ALL)"
  st_eq "fut02 line begins" "FUT-02: not needed" "$(fut02_line AGREE-ALL AGREE-ALL | cut -c1-18)"
  st_eq "fut02 line undecided begins" "FUT-02: undecided" "$(fut02_line "?" "?" | cut -c1-17)"

  # owner_steps_ok
  f=$t/o.md
  printf '# R\n\n## Owner steps\n\n1. Do the thing on a real server.\n\n## Next\n' > "$f"
  st_ok "owner numbered" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nO-1 Do it.\n' > "$f"
  st_ok "owner O-1" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nNone, every measurement ran on the container.\n\n## Next\n' > "$f"
  st_ok "owner None with a reason" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nNone: nothing here needs a server.\n' > "$f"
  st_ok "owner None colon reason" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nNone\n\n## Next\n' > "$f"
  st_fail "owner only None" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nNone.\n\n## Next\n' > "$f"
  st_fail "owner None with a full stop" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\n## Next\n\n1. A numbered line in the next section.\n' > "$f"
  st_fail "owner empty section" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n' > "$f"
  st_fail "owner empty section at the end" owner_steps_ok "$f"
  printf '# R\n\n## Not tested\n\n1. Something.\n' > "$f"
  st_fail "owner no heading" owner_steps_ok "$f"
  printf '# R\n\n## Owner steps\n\nNonetheless, there is more to say about it.\n' > "$f"
  st_fail "owner Nonetheless is not None" owner_steps_ok "$f"
  st_fail "owner missing file" owner_steps_ok "$t/none.md"

  # con01_verdict
  printf 'PREP OK\n' > "$t/prep.txt"
  printf 'MIN-RUNS: 30\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 38\nCONSIST OK\n' > "$t/consist.txt"
  printf 'WINDOW OK\n' > "$t/window.txt"
  st_eq "con01 all green" PASS "$(con01_verdict 0 "$t/prep.txt" 0 "$t/consist.txt" 0 "$t/window.txt")"
  st_eq "con01 prep failed" "FAIL (prep: the stage ended with status 1)" "$(con01_verdict 1 "$t/prep.txt" 3 "$t/consist.txt" 3 "$t/window.txt")"
  st_eq "con01 consist status" "FAIL (consist: the stage ended with status 1)" "$(con01_verdict 0 "$t/prep.txt" 1 "$t/consist.txt" 0 "$t/window.txt")"
  st_eq "con01 window status" "FAIL (window: the stage ended with status 1)" "$(con01_verdict 0 "$t/prep.txt" 0 "$t/consist.txt" 1 "$t/window.txt")"
  st_eq "con01 window not played" "FAIL (window: the platform could not give the evidence: the stage ended with status 3)" "$(con01_verdict 0 "$t/prep.txt" 0 "$t/consist.txt" 3 "$t/window.txt")"
  printf 'MIN-RUNS: 29\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 40\nCONSIST OK\n' > "$t/c2.txt"
  st_eq "con01 MIN-RUNS 29" "FAIL (MIN-RUNS in $t/c2.txt is '29', the evidence set needs 30 or more: a trace is not the set)" "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c2.txt" 0 "$t/window.txt")"
  printf 'MIN-RUNS: 1\nCONSIST-RUNS-STAGE-adds: 6\nRESTORED-RUNS-STAGE-adds: 6\nCONSIST-RUNS-STAGE-deletes: 6\nRESTORED-RUNS-STAGE-deletes: 6\nCONSIST OK\n' > "$t/c3.txt"
  st_fail "con01 a trace is not the set" test "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c3.txt" 0 "$t/window.txt")" = PASS
  printf 'CONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 40\nCONSIST OK\n' > "$t/c4.txt"
  st_fail "con01 no MIN-RUNS line" test "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c4.txt" 0 "$t/window.txt")" = PASS
  printf 'MIN-RUNS: 30\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 29\nRESTORED-RUNS-STAGE-deletes: 29\nCONSIST OK\n' > "$t/c5.txt"
  st_eq "con01 a stage under MIN-RUNS" "FAIL (stage deletes had '29' runs, fewer than MIN-RUNS 30)" "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c5.txt" 0 "$t/window.txt")"
  printf 'MIN-RUNS: 30\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 14\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 40\nCONSIST OK\n' > "$t/c6.txt"
  st_eq "con01 restored below the floor" "FAIL (stage adds: '14' runs were restored and compared after the restore, the floor is 15)" "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c6.txt" 0 "$t/window.txt")"
  printf 'MIN-RUNS: 30\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 15\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 15\nCONSIST OK\n' > "$t/c7.txt"
  st_eq "con01 restored at the floor" PASS "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c7.txt" 0 "$t/window.txt")"
  printf 'MIN-RUNS: 30\nCONSIST-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 40\nCONSIST OK\n' > "$t/c8.txt"
  st_fail "con01 no restored line" test "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c8.txt" 0 "$t/window.txt")" = PASS
  printf 'MIN-RUNS: 30\nBAD  x\nCONSIST-RUNS-STAGE-adds: 40\nRESTORED-RUNS-STAGE-adds: 40\nCONSIST-RUNS-STAGE-deletes: 40\nRESTORED-RUNS-STAGE-deletes: 40\nCONSIST OK\n' > "$t/c9.txt"
  st_fail "con01 a BAD line" test "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c9.txt" 0 "$t/window.txt")" = PASS
  printf 'WINDOW FAILED\n' > "$t/w2.txt"
  st_fail "con01 window not ok" test "$(con01_verdict 0 "$t/prep.txt" 0 "$t/consist.txt" 0 "$t/w2.txt")" = PASS
  st_fail "con01 missing prep file" test "$(con01_verdict 0 "$t/none.txt" 0 "$t/consist.txt" 0 "$t/window.txt")" = PASS
  st_eq "con01 floor argument" PASS "$(con01_verdict 0 "$t/prep.txt" 0 "$t/c3.txt" 0 "$t/window.txt" 1)"

  # the refusals of con-chain
  st_ok "con_distinct four names" con_distinct a b c d
  st_fail "con_distinct first two" con_distinct a a c d
  st_fail "con_distinct last two" con_distinct a b c c
  st_fail "con_distinct first and last" con_distinct a b c a
  st_fail "con_distinct empty" con_distinct a b "" d
  st_fail "con_distinct three values" con_distinct a b c
  st_ok "con_dir_safe a directory" con_dir_safe deploy/test/out/amd64-t25-src /repo
  st_fail "con_dir_safe slash" con_dir_safe / /repo
  st_fail "con_dir_safe dot" con_dir_safe . /repo
  st_fail "con_dir_safe dot dot" con_dir_safe .. /repo
  st_fail "con_dir_safe empty" con_dir_safe "" /repo
  st_fail "con_dir_safe root" con_dir_safe /repo /repo
  st_fail "con_dir_safe root with slash" con_dir_safe /repo/ /repo
  mkdir -p "$t/empty" "$t/full"
  : > "$t/full/x"
  st_ok "con_dir_empty absent" con_dir_empty "$t/absent"
  st_ok "con_dir_empty empty" con_dir_empty "$t/empty"
  st_fail "con_dir_empty holds a file" con_dir_empty "$t/full"

  # con_field, con_class, con_classify
  st_eq "con_field" 8 "$(con_field 'LEVEL: 60 GOOSE-ROWS: 61 PAGES-COUNT: 8 USERS-COUNT: 2' PAGES-COUNT)"
  st_eq "con_field first" 60 "$(con_field 'LEVEL: 60 GOOSE-ROWS: 61' LEVEL)"
  st_eq "con_field absent" "" "$(con_field 'LEVEL: 60' USERS-COUNT)"
  printf 'CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE\nCONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY\n' > "$t/w.txt"
  st_eq "con_class C0" AGREE "$(con_class "$t/w.txt" C0)"
  st_eq "con_class C1" ORPHAN-ONLY "$(con_class "$t/w.txt" C1)"
  st_eq "con_class C2 absent" "?" "$(con_class "$t/w.txt" C2)"
  printf 'RUN 1 adds CLASS=AGREE X=1\nRUN 2 adds CLASS=ORPHAN-ONLY X=1\nRUN 3 adds CLASS=ORPHAN-ONLY X=1\nRUN 1 deletes CLASS=ROW-WITHOUT-FILE X=1\nRUN 2 deletes CLASS=BACKUP-REFUSED X=1\n' > "$t/runs.txt"
  out=$(con_classify "$t/runs.txt")
  st_eq "con_classify lines" 3 "$(printf '%s\n' "$out" | grep -c '^CLASSIFY: ')"
  st_eq "con_classify groups the runs" "CLASSIFY: consist stage adds: 2 run(s) with CLASS=ORPHAN-ONLY (runs 2 3)" "$(printf '%s\n' "$out" | sed -n 1p)"
  st_eq "con_classify one row" "CLASSIFY: consist stage deletes: 1 run(s) with CLASS=ROW-WITHOUT-FILE (runs 1)" "$(printf '%s\n' "$out" | sed -n 2p)"
  printf 'RUN 1 adds CLASS=AGREE X=1\n' > "$t/runs2.txt"
  st_eq "con_classify all agree" "" "$(con_classify "$t/runs2.txt")"
  printf 'CONTROL-CLASS: COMPLETE\nHAND-CLASS: FAILED-PARTIAL-DIR-LEFT\nAGAIN-CLASS: GUARD-REFUSED\nUNIT-CLASS: FALSE-COMPLETE\n' > "$t/cf.txt"
  out=$(con_classify_capfull "$t/cf.txt")
  st_eq "con_classify_capfull lines" 3 "$(printf '%s\n' "$out" | grep -c '^CLASSIFY: ')"
  st_eq "con_classify_capfull hand" "CLASSIFY: capfull HAND-CLASS is FAILED-PARTIAL-DIR-LEFT (see capfull.txt)" "$(printf '%s\n' "$out" | sed -n 1p)"
  st_eq "con_classify_capfull false complete is a defect candidate" 1 "$(printf '%s\n' "$out" | grep -c 'UNIT-CLASS is FALSE-COMPLETE.*defect candidate (class 4)')"
  st_eq "con_classify_capfull a missing file names every class" 4 "$(con_classify_capfull "$t/none.txt" | grep -c 'is missing from')"

  # con_summarize on a fixture that has the shape of the real files
  mkdir -p "$t/s/src" "$t/s/tgt" "$t/s/old" "$t/s/new"
  printf 'PREP OK\n' > "$t/s/src/prep.txt"
  {
    printf 'MIN-RUNS: 30\nRESTORED-RUNS-FLOOR: 15 per stage\n'
    for c in adds deletes; do
      printf 'CONSIST-RUNS-STAGE-%s: 40\nRESTORED-RUNS-STAGE-%s: 40\nUPLOADS-IN-WINDOW-STAGE-%s: 90\nPAGES-IN-WINDOW-STAGE-%s: 20\nDELETES-IN-WINDOW-STAGE-%s: 3\n' "$c" "$c" "$c" "$c" "$c"
      printf 'BACKUP-RC-HISTOGRAM-STAGE-%s: 0=39 24=1\nOUTCOME-STAGE-%s: ORPHAN-ONLY-NAMED\n' "$c" "$c"
      printf 'CONSIST-SUMMARY-STAGE-%s: RUNS=40 RESTORED=40 AGREE=10 ORPHAN-ONLY=29 OUTCOME=ORPHAN-ONLY-NAMED\n' "$c"
    done
    printf 'CONSIST OK\n'
  } > "$t/s/src/consist.txt"
  printf 'RUN 1 adds CLASS=AGREE X=1\nRUN 2 adds CLASS=ORPHAN-ONLY X=1\n' > "$t/s/src/consist-runs.txt"
  printf 'CONTROL C0 ACTION=none HOLD-REACHED=yes HOLD-RELEASED-IN-S=0 MISSING=0 ORPHAN=0 CLASS=AGREE EXPECTED=AGREE\nCONTROL C1 ACTION=upload HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=0 ORPHAN=3 CLASS=ORPHAN-ONLY EXPECTED=ORPHAN-ONLY\nCONTROL C2 ACTION=delete HOLD-REACHED=yes HOLD-RELEASED-IN-S=1 MISSING=3 ORPHAN=0 CLASS=ROW-WITHOUT-FILE EXPECTED=ROW-WITHOUT-FILE\nWINDOW OK\n' > "$t/s/src/window.txt"
  printf 'FLOOR-MB: 1024\nDATA-MB: 1170 (what a backup has to write)\nFS-SIZE-MB: 1064\nFREE-AT-GUARD-MB: 1064 (df -Pm)\nCONTROL-CLASS: COMPLETE\nHAND-CLASS: FAILED-PARTIAL-DIR-LEFT\nAGAIN-CLASS: GUARD-REFUSED\nUNIT-CLASS: FAILED-PARTIAL-DIR-LEFT\nUNIT-RESULT: exit-code\nUNIT-EXECMAINSTATUS: 11\nRESTORE-PARTIAL-EXIT: 1\nCAPFULL OK\n' > "$t/s/src/capfull.txt"
  printf 'COMPAT-TAG: v0.1.0 (4d238b9)\nSTATE-SOURCE: LEVEL: 60 PAGES-COUNT: 8 USERS-COUNT: 2 \nSTATE-RESTORED: LEVEL: 60 PAGES-COUNT: 8 USERS-COUNT: 2 \nLEVEL-BEFORE: 60\nLEVEL-AFTER: 60\nLOGIN-ADMIN: HTTP 303 /admin/\nLOGIN-EDITOR: HTTP 303 /admin/\nMEDIA-SERVED-EQUAL: 4 of 4\nCHECK: 17 of 17\nCOMPAT OK\n' > "$t/s/old/compat.txt"
  printf 'details\n' > "$t/s/old/compat-binary.txt"
  printf 'STEP 01 rc=0 a\nSTEP 02 rc=1 b\n' > "$t/s/src/steps.txt"
  printf 'STEP 01 rc=0 a\n' > "$t/s/old/steps.txt"
  printf 'DEVIATION[churn]: x\nDEVIATION[fs]: y\n' > "$t/s/src/deviations.txt"
  printf 'DEVIATION[churn]: x\nDEVIATION[hold]: z\n' > "$t/s/tgt/deviations.txt"
  out=$(set -e; OUT_DIR=$t/s/src TGT_OUT_DIR=$t/s/tgt OLD_OUT_DIR=$t/s/old NEW_OUT_DIR=$t/s/new PLATFORM=linux/amd64 con_summarize 0 0 0 0 0 0 OK 2>&1; echo "FAILS=$CON_FAILS")
  st_eq "con_summarize CON-01" 1 "$(printf '%s\n' "$out" | grep -c '^CON-01 PASS$')"
  st_eq "con_summarize CON-02" 1 "$(printf '%s\n' "$out" | grep -c '^CON-02 PASS$')"
  st_eq "con_summarize CON-03" 1 "$(printf '%s\n' "$out" | grep -c '^CON-03 PASS$')"
  st_eq "con_summarize leak line" 1 "$(printf '%s\n' "$out" | grep -c '^LEAK-CHECK OK$')"
  st_eq "con_summarize FUT-02 line" 1 "$(printf '%s\n' "$out" | grep -c '^FUT-02: not needed (adds ORPHAN-ONLY-NAMED, deletes ORPHAN-ONLY-NAMED')"
  st_eq "con_summarize STEPS" 1 "$(printf '%s\n' "$out" | grep -c '^STEPS: 3 played, 1 failed$')"
  st_eq "con_summarize DEVIATIONS" 1 "$(printf '%s\n' "$out" | grep -c '^DEVIATIONS: churn fs hold$')"
  st_eq "con_summarize WINDOW" 1 "$(printf '%s\n' "$out" | grep -c '^WINDOW: C0 (nothing written) AGREE, C1 (upload in the window) ORPHAN-ONLY, C2 (delete in the window) ROW-WITHOUT-FILE$')"
  st_eq "con_summarize COMPAT" 1 "$(printf '%s\n' "$out" | grep -c '^COMPAT: TAG=v0.1.0 (4d238b9) LEVEL-BEFORE=60 LEVEL-AFTER=60 LOGIN-ADMIN=HTTP 303 /admin/ LOGIN-EDITOR=HTTP 303 /admin/ PAGES-SOURCE=8 PAGES-RESTORED=8 MEDIA-SERVED=4 of 4 CHECK=17 of 17$')"
  st_eq "con_summarize CAPFULL" 1 "$(printf '%s\n' "$out" | grep -c '^CAPFULL: FLOOR-MB=1024 FREE-AT-GUARD-MB=1064 DATA-MB=1170 FS-SIZE-MB=1064 CONTROL-CLASS=COMPLETE HAND-CLASS=FAILED-PARTIAL-DIR-LEFT AGAIN-CLASS=GUARD-REFUSED UNIT-CLASS=FAILED-PARTIAL-DIR-LEFT UNIT-RESULT=exit-code UNIT-EXECMAINSTATUS=11 RESTORE-PARTIAL-EXIT=1$')"
  st_eq "con_summarize CLASSIFY lines" 1 "$(printf '%s\n' "$out" | grep -c '^CLASSIFY: consist stage adds: 1 run(s) with CLASS=ORPHAN-ONLY (runs 2)$')"
  st_eq "con_summarize CLASSIFY capfull" 1 "$(printf '%s\n' "$out" | grep -c '^CLASSIFY: capfull HAND-CLASS is FAILED-PARTIAL-DIR-LEFT')"
  st_eq "con_summarize no failures" "FAILS=0" "$(printf '%s\n' "$out" | tail -n 1)"
  st_eq "con_summarize writes summary.txt" 1 "$(grep -c '^CON-03 PASS$' "$t/s/src/summary.txt")"
  out=$(set -e; OUT_DIR=$t/s/src TGT_OUT_DIR=$t/s/tgt OLD_OUT_DIR=$t/s/old NEW_OUT_DIR=$t/s/new PLATFORM=linux/amd64 con_summarize 0 3 3 0 0 0 FAILED 2>&1; echo "FAILS=$CON_FAILS")
  st_eq "con_summarize a failed leak check and a broken CON-01" "FAILS=2" "$(printf '%s\n' "$out" | tail -n 1)"
  st_eq "con_summarize says LEAK-CHECK FAILED" 1 "$(printf '%s\n' "$out" | grep -c '^LEAK-CHECK FAILED$')"
  out=$(set -e; OUT_DIR=$t/s/src TGT_OUT_DIR=$t/s/tgt OLD_OUT_DIR=$t/s/old NEW_OUT_DIR=$t/s/new PLATFORM=linux/amd64 con_summarize 0 0 0 0 1 3 OK 2>&1; echo "FAILS=$CON_FAILS")
  st_eq "con_summarize compat-source failed is CON-02 FAIL" 1 "$(printf '%s\n' "$out" | grep -c '^CON-02 FAIL (compat-source ended with status 1')"
  rm -f "$t/s/old/compat-binary.txt"
  out=$(set -e; OUT_DIR=$t/s/src TGT_OUT_DIR=$t/s/tgt OLD_OUT_DIR=$t/s/old NEW_OUT_DIR=$t/s/new PLATFORM=linux/amd64 con_summarize 0 0 0 0 0 0 OK 2>&1; echo "FAILS=$CON_FAILS")
  st_eq "con_summarize without compat-binary.txt is CON-02 FAIL" 1 "$(printf '%s\n' "$out" | grep -c '^CON-02 FAIL (.*compat-binary.txt is missing')"

  # record_check with the CON headings, and with none set it keeps its answers
  st_record_make "$t/r" "$(git rev-parse --short HEAD)"
  sed 's/^## BKP-0\([1-5]\)/## CON-0\1/' "$t/r/rec.md" > "$t/r/con.md"
  grep -v '^## CON-02' "$t/r/con.md" > "$t/r/con-no2.md"
  out=$(RC_REQ_PREFIX=CON-0 RC_REQ_IDS="1 2 3" record_check "$t/r/con.md" "$t/r/out" "amd64-t24-src" 2>&1) || true
  st_eq "record-check CON record with CON headings" "RECORD-CHECK OK 1 blocks" "$out"
  out=$(RC_REQ_PREFIX=CON-0 RC_REQ_IDS="1 2 3" record_check "$t/r/con-no2.md" "$t/r/out" "amd64-t24-src" 2>&1) && rc=0 || rc=$?
  st_eq "record-check CON record without CON-02 exit" 1 "$rc"
  st_eq "record-check CON record without CON-02 names it" 1 "$(printf '%s\n' "$out" | grep -c "no heading beginning '## CON-02'")"
  out=$(record_check "$t/r/con.md" "$t/r/out" "amd64-t24-src" 2>&1) && rc=0 || rc=$?
  st_eq "record-check CON record, no variables, exit" 1 "$rc"
  st_eq "record-check CON record, no variables, fails on BKP-01" 1 "$(printf '%s\n' "$out" | grep -c "no heading beginning '## BKP-01'")"
  out=$(RC_REQ_PREFIX=CON-0 RC_REQ_IDS="1 2 3" record_check "$t/r/rec.md" "$t/r/out" "amd64-t24-src" 2>&1) && rc=0 || rc=$?
  st_eq "record-check BKP record with CON headings asked, exit" 1 "$rc"
  st_eq "record-check BKP record with CON headings asked, names CON-01" 1 "$(printf '%s\n' "$out" | grep -c "no heading beginning '## CON-01'")"
  out=$(record_check "$t/r/rec.md" "$t/r/out" "amd64-t24-src" 2>&1) || true
  st_eq "record-check BKP record, no variables, unchanged" "RECORD-CHECK OK 1 blocks" "$out"
  out=$(RC_REQ_PREFIX=CON-0 RC_REQ_IDS="1 2 3 4 5" record_check "$t/r/con.md" "$t/r/out" "amd64-t24-src" 2>&1) || true
  st_eq "record-check five CON ids on a record with five" "RECORD-CHECK OK 1 blocks" "$out"
  rm -rf "${t:?}"
}

CON_KEEP=0
con_chain_cleanup() {
  [ "$CON_KEEP" != 1 ] || return 0
  cmd_down || true
  cmd_down_con || true
}

# con_summarize SC_PREP SC_CONSIST SC_WINDOW SC_CAPFULL SC_CSRC SC_COMPAT LEAK: the
# verdicts of the three requirements, from the stage statuses and the files the stages
# wrote, and the summary block (to OUT_DIR/summary.txt, printed). It sets CON_FAILS to
# the number of requirements and checks that did not pass. It looks at files only.
CON_FAILS=0
con_summarize() {
  local sc_prep=$1 sc_consist=$2 sc_window=$3 sc_capfull=$4 sc_csrc=$5 sc_compat=$6 leak=$7
  local v1 v2 v3 v fut fut_a fut_d kinds bp sp=0 sf=0 st d f cl s sum=$OUT_DIR/summary.txt
  CON_FAILS=0
  v1=$(con01_verdict "$sc_prep" "$OUT_DIR/prep.txt" "$sc_consist" "$OUT_DIR/consist.txt" "$sc_window" "$OUT_DIR/window.txt" 30)
  if [ "$sc_csrc" -ne 0 ]; then
    v2="FAIL (compat-source ended with status $sc_csrc: the previous release was not brought up and compat was not played)"
  elif [ ! -s "$OLD_OUT_DIR/compat-binary.txt" ]; then
    v2="FAIL ($OLD_OUT_DIR/compat-binary.txt is missing: nothing says which binary the previous release was)"
  else
    v2=$(req_verdict "$sc_compat" "$OLD_OUT_DIR/compat.txt" "COMPAT OK")
  fi
  v3=$(req_verdict "$sc_capfull" "$OUT_DIR/capfull.txt" "CAPFULL OK")
  fut_a=$(kvq "$OUT_DIR/consist.txt" OUTCOME-STAGE-adds)
  fut_d=$(kvq "$OUT_DIR/consist.txt" OUTCOME-STAGE-deletes)
  fut=$(fut02_line "$fut_a" "$fut_d")

  for d in "$OUT_DIR" "$TGT_OUT_DIR" "$OLD_OUT_DIR" "$NEW_OUT_DIR"; do
    bp=$(steps_of "$d")
    sp=$((sp + ${bp% *}))
    sf=$((sf + ${bp#* }))
  done
  # a directory without deviations.txt (the fresh box, the second host) is no failure:
  # the pipeline runs under pipefail, and a failed assignment would end the chain here
  kinds=$(for d in "$OUT_DIR" "$TGT_OUT_DIR" "$OLD_OUT_DIR" "$NEW_OUT_DIR"; do
    sed -n 's/^DEVIATION\[\([a-z-]*\)\].*/\1/p' "$d/deviations.txt" 2> /dev/null || true
  done | sort -u | tr '\n' ' ' | sed 's/ $//') || kinds=""

  # What the person running the chain has to classify (1 harness bug, 2 container limit,
  # 3 measured property of the shipped scripts, 4 defect in a shipped file).
  cl=$(
    con_classify "$OUT_DIR/consist-runs.txt"
    for f in "$OUT_DIR/prep.txt" "$OUT_DIR/consist.txt" "$OUT_DIR/window.txt" "$OUT_DIR/capfull.txt" "$OLD_OUT_DIR/compat.txt"; do
      grep '^BAD' "$f" 2> /dev/null | cut -c1-300 | sed "s|^|CLASSIFY: BAD line in $f: |" || true
    done
    con_classify_capfull "$OUT_DIR/capfull.txt"
  ) || cl=""

  {
    echo "---- summary ($PLATFORM) ----"
    echo "CON-01 $v1"
    echo "CON-02 $v2"
    echo "CON-03 $v3"
    for st in adds deletes; do
      echo "CONSIST: stage $st $(kvq "$OUT_DIR/consist.txt" "CONSIST-SUMMARY-STAGE-$st")"
      echo "CONSIST: stage $st window writes: uploads $(kvq "$OUT_DIR/consist.txt" "UPLOADS-IN-WINDOW-STAGE-$st"), page saves $(kvq "$OUT_DIR/consist.txt" "PAGES-IN-WINDOW-STAGE-$st"), deletes $(kvq "$OUT_DIR/consist.txt" "DELETES-IN-WINDOW-STAGE-$st"); backup status histogram $(kvq "$OUT_DIR/consist.txt" "BACKUP-RC-HISTOGRAM-STAGE-$st")"
    done
    echo "CONSIST: MIN-RUNS $(kvq "$OUT_DIR/consist.txt" MIN-RUNS), restored-run floor $(kvq "$OUT_DIR/consist.txt" RESTORED-RUNS-FLOOR)"
    echo "WINDOW: C0 (nothing written) $(con_class "$OUT_DIR/window.txt" C0), C1 (upload in the window) $(con_class "$OUT_DIR/window.txt" C1), C2 (delete in the window) $(con_class "$OUT_DIR/window.txt" C2)"
    echo "COMPAT: TAG=$(kvq "$OLD_OUT_DIR/compat.txt" COMPAT-TAG) LEVEL-BEFORE=$(kvq "$OLD_OUT_DIR/compat.txt" LEVEL-BEFORE) LEVEL-AFTER=$(kvq "$OLD_OUT_DIR/compat.txt" LEVEL-AFTER) LOGIN-ADMIN=$(kvq "$OLD_OUT_DIR/compat.txt" LOGIN-ADMIN) LOGIN-EDITOR=$(kvq "$OLD_OUT_DIR/compat.txt" LOGIN-EDITOR) PAGES-SOURCE=$(con_field "$(kvq "$OLD_OUT_DIR/compat.txt" STATE-SOURCE)" PAGES-COUNT) PAGES-RESTORED=$(con_field "$(kvq "$OLD_OUT_DIR/compat.txt" STATE-RESTORED)" PAGES-COUNT) MEDIA-SERVED=$(kvq "$OLD_OUT_DIR/compat.txt" MEDIA-SERVED-EQUAL) CHECK=$(kvq "$OLD_OUT_DIR/compat.txt" CHECK)"
    v=$(kvq "$OUT_DIR/capfull.txt" FREE-AT-GUARD-MB)
    echo "CAPFULL: FLOOR-MB=$(kvq "$OUT_DIR/capfull.txt" FLOOR-MB) FREE-AT-GUARD-MB=${v%% *} DATA-MB=$(kvq "$OUT_DIR/capfull.txt" DATA-MB | cut -d' ' -f1) FS-SIZE-MB=$(kvq "$OUT_DIR/capfull.txt" FS-SIZE-MB) CONTROL-CLASS=$(kvq "$OUT_DIR/capfull.txt" CONTROL-CLASS) HAND-CLASS=$(kvq "$OUT_DIR/capfull.txt" HAND-CLASS) AGAIN-CLASS=$(kvq "$OUT_DIR/capfull.txt" AGAIN-CLASS) UNIT-CLASS=$(kvq "$OUT_DIR/capfull.txt" UNIT-CLASS) UNIT-RESULT=$(kvq "$OUT_DIR/capfull.txt" UNIT-RESULT) UNIT-EXECMAINSTATUS=$(kvq "$OUT_DIR/capfull.txt" UNIT-EXECMAINSTATUS) RESTORE-PARTIAL-EXIT=$(kvq "$OUT_DIR/capfull.txt" RESTORE-PARTIAL-EXIT)"
    echo "$fut"
    if [ -n "$cl" ]; then printf '%s\n' "$cl"; else echo "CLASSIFY: none"; fi
    echo "STEPS: $sp played, $sf failed"
    echo "DEVIATIONS: ${kinds:-none}"
    if [ "$leak" = OK ]; then echo "LEAK-CHECK OK"; else echo "LEAK-CHECK FAILED"; fi
  } > "$sum"
  cat "$sum"
  for s in "$v1" "$v2" "$v3"; do
    [ "$s" = PASS ] || CON_FAILS=$((CON_FAILS + 1))
  done
  [ "$leak" = OK ] || CON_FAILS=$((CON_FAILS + 1))
}

# con-chain: the whole proof of phase 25 from nothing but the image, in one command.
# Source and second host, the editor (prep), CON-01 (consist, window), CON-03 (capfull)
# and then the previous release's pair (compat-source, compat) for CON-02. Every stage is
# a child process of this script with its status kept (a function on the left of `||`
# runs with set -e off), and every verdict comes from the files the stages wrote.
cmd_con_chain() {
  local ref d c why s v1 v2 v3 fut fut_a fut_d leak kinds bp sp=0 sf=0 fails=0 st f k v
  local sc_prep=0 sc_consist=0 sc_window=0 sc_capfull=0 sc_csrc=0 sc_compat=0
  local sum=$OUT_DIR/summary.txt cl

  [ "$TARGET" = systemd-base ] \
    || die "con-chain needs TARGET=systemd-base (got '$TARGET'): the proof starts from a bare systemd image, nothing else; run it as: TARGET=systemd-base bash deploy/test/nightly.sh con-chain"
  why=$(con_distinct "$NAME" "$TGT_NAME" "$OLD_NAME" "$NEW_NAME") \
    || die "con-chain: the four container names must be different ($why); set NAME, TGT_NAME, OLD_NAME and NEW_NAME apart"
  why=$(con_distinct "$OUT_DIR" "$TGT_OUT_DIR" "$OLD_OUT_DIR" "$NEW_OUT_DIR") \
    || die "con-chain: the four evidence directories must be different ($why); set OUT_DIR, TGT_OUT_DIR, OLD_OUT_DIR and NEW_OUT_DIR apart"
  for d in "$OUT_DIR" "$TGT_OUT_DIR" "$OLD_OUT_DIR" "$NEW_OUT_DIR"; do
    con_dir_safe "$d" || die "con-chain: refusing the evidence directory '$d'"
  done
  for c in "$NAME" "$TGT_NAME" "$OLD_NAME" "$NEW_NAME"; do
    if docker container inspect "$c" > /dev/null 2>&1; then
      die "con-chain: the container $c exists already; the chain starts from fresh containers, remove them with: NAME=$NAME TGT_NAME=$TGT_NAME OLD_NAME=$OLD_NAME NEW_NAME=$NEW_NAME bash deploy/test/nightly.sh down down-con"
    fi
  done
  for d in "$OUT_DIR" "$TGT_OUT_DIR" "$OLD_OUT_DIR" "$NEW_OUT_DIR"; do
    con_dir_empty "$d" \
      || die "con-chain: $d holds the files of an earlier pass (a stage overwrites only what it writes, and an old file would be pasted into the record as if it belonged to this pass); remove it with: rm -rf $d"
  done

  # One binary for every container of this release, even if a commit lands mid-run.
  ref=$(pin_ref)
  NEW_REF=$ref
  BULK_MB=${BULK_MB:-100}
  CON_KEEP=$KEEP
  trap con_chain_cleanup EXIT
  export PLATFORM TARGET NAME OUT_DIR TGT_NAME TGT_OUT_DIR OLD_NAME OLD_OUT_DIR NEW_NAME NEW_OUT_DIR NEW_REF BULK_MB
  export CONSIST_RUNS CONSIST_MIN_RUNS CONSIST_STAGES CONSIST_RESTORE_EVERY CHURN_WORKERS CHURN_PAUSE
  export CAPFULL_FLOOR_MB CAPFULL_MARGIN_MB CAPFULL_DATA_MB COMPAT_TAG
  export KEEP=1
  say "con-chain: NEW_REF pinned to $ref; source $NAME ($OUT_DIR), second host $TGT_NAME ($TGT_OUT_DIR), previous release $OLD_NAME ($OLD_OUT_DIR), fresh box $NEW_NAME ($NEW_OUT_DIR); CONSIST_RUNS=$CONSIST_RUNS CONSIST_MIN_RUNS=$CONSIST_MIN_RUNS CONSIST_RESTORE_EVERY=$CONSIST_RESTORE_EVERY BULK_MB=$BULK_MB; started $(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # Without these two nothing can be measured.
  chain_stage source || die "con-chain: the source could not be built (status $?)"
  chain_stage target || die "con-chain: the second host could not be built (status $?)"

  chain_stage prep || sc_prep=$?
  if [ "$sc_prep" -eq 0 ]; then
    chain_stage consist || sc_consist=$?
    chain_stage window || sc_window=$?
  else
    sc_consist=3
    sc_window=3
    say "con-chain: prep ended with status $sc_prep: consist and window are not played (nothing was measured), their status is recorded as 3"
  fi
  # capfull replaces the bulk directory of the source and mounts over its backup root,
  # so it comes after the stages that read that data; the previous release's pair is
  # brought up last, so that the Docker VM's memory is not shared with it while a 1.1 GB
  # filesystem is filled.
  chain_stage capfull || sc_capfull=$?
  chain_stage compat-source || sc_csrc=$?
  if [ "$sc_csrc" -eq 0 ]; then
    chain_stage compat || sc_compat=$?
  else
    sc_compat=3
    say "con-chain: compat-source ended with status $sc_csrc: compat is not played, its status is recorded as 3"
  fi

  # The verdicts, from the files the stages wrote.
  # The statuses are kept as a file of their own: and the record pastes it.
  printf 'prep=%s consist=%s window=%s capfull=%s compat-source=%s compat=%s\n' "$sc_prep" "$sc_consist" "$sc_window" "$sc_capfull" "$sc_csrc" "$sc_compat" > "$OUT_DIR/chain-statuses.txt"

  # No secret of any container in any of the four directories.
  leak=OK
  ( leak_check ) || leak=FAILED
  ( NAME=$OLD_NAME OUT_DIR=$OLD_OUT_DIR TGT_NAME=$NEW_NAME TGT_OUT_DIR=$NEW_OUT_DIR leak_check ) || leak=FAILED

  con_summarize "$sc_prep" "$sc_consist" "$sc_window" "$sc_capfull" "$sc_csrc" "$sc_compat" "$leak"
  say "con-chain: stage statuses (1 is a BAD line or a refusal, 3 is not played or the platform could not give the evidence): prep=$sc_prep consist=$sc_consist window=$sc_window capfull=$sc_capfull compat-source=$sc_csrc compat=$sc_compat"
  say "con-chain: finished $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [ "$CON_FAILS" -ne 0 ]; then
    say "con-chain: $CON_FAILS requirement(s) or checks did not pass, see $OUT_DIR/summary.txt"
    exit 1
  fi
}

# record-check-con: the proof of deploy/BACKUP-CONSISTENCY-RUN.md against the scratch
# files: record_check with the CON headings and the ledger of the four evidence
# directories, and then the Owner steps section. The last line is the answer.
cmd_record_check_con() {
  local r=${R:-deploy/BACKUP-CONSISTENCY-RUN.md} e=${E:-deploy/test/out} out rc=0 why orc=0
  out=$(RC_REQ_PREFIX=CON-0 RC_REQ_IDS="1 2 3" record_check "$r" "$e" "$(basename "$OUT_DIR") $(basename "$TGT_OUT_DIR") $(basename "$OLD_OUT_DIR") $(basename "$NEW_OUT_DIR")" 2>&1) || rc=$?
  why=$(owner_steps_ok "$r") || orc=1
  if [ "$orc" -eq 0 ]; then
    printf '%s\n' "$out"
    [ "$rc" -eq 0 ] || exit 1
    return 0
  fi
  if [ "$rc" -eq 0 ]; then out=$(printf '%s\n' "$out" | sed '$d'); fi
  [ -z "$out" ] || printf '%s\n' "$out"
  echo "RECORD-CHECK BAD: $why"
  echo "RECORD-CHECK FAILED: the Owner steps section"
  exit 1
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
    prep | consist | window) "cmd_$sub" ;;
    compat-source) cmd_compat_source ;;
    compat) cmd_compat ;;
    capfull) cmd_capfull ;;
    down-con) cmd_down_con ;;
    con-chain) cmd_con_chain ;;
    record-check-con) cmd_record_check_con ;;
    *) usage ;;
  esac
done
