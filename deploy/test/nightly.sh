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
#   KEEP           0; 1 keeps containers (nightly.sh never removes one itself)
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
  sed -n '2,19p' "${BASH_SOURCE[0]}" >&2
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

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  selftest_zone
  selftest_tod
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
  local lead=$1 label=$2 now off std zname got want diff
  now=$(cont_epoch)
  off=$(zone_offset_seconds "$now" "$lead")
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
  if ! grep -q 'Backup complete' "$OUT_DIR/fire-journal.txt"; then
    check_info "the journal narrowed to the invocation $id holds no completion line; the unit's whole journal is written instead"
    dx journalctl -u holzcloud-backup.service -o short-precise --no-pager > "$OUT_DIR/fire-journal.txt" 2>&1 || true
  fi
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
# Dispatch
# ---------------------------------------------------------------------------

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    selftest | source | target | fire) "cmd_$sub" ;;
    *) usage ;;
  esac
done
