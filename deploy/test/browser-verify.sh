#!/usr/bin/env bash
# Phase 34: UAT-01, UAT-02 and UAT-03 played in a real browser against a dev server
# with its own data directory.
#
# A sibling of deploy/test/backup-verify.sh in its shape (bound files, an evidence
# commit pinned to HEAD, verdicts read from KEY: value files, never from a trace),
# but without a container: the three flows are a browser's, and they run on the Mac.
# It builds this tree and tools/totpcode into UAT_DIR, starts the binary there on a
# data directory of its own (loopback, mail queued to an unreachable port, never
# sent), runs a Playwright script against it, and computes the verdicts from the
# KEY: value lines the script prints.
#
#   browser-verify.sh selftest      the pure functions below, a table of cases, no browser
#   browser-verify.sh run           the evidence: build, dev server, the script, the raw
#                                   files below deploy/test/out/mac-t34-uat
#   browser-verify.sh appendix      the appendix blocks of the fixed paste list, from the files below E
#   browser-verify.sh record-check  the proof of deploy/BROWSER-RUN.md (R) against the scratch
#                                   root (E): every block byte for byte, the script text
#                                   included, every cited mac-t34- file pasted or written
#                                   as "`FILE` (not pasted)", one Evidence commit, pinned
#                                   and unchanged since, nothing secret outside the script
#
# R and E default to deploy/BROWSER-RUN.md and deploy/test/out.
#
# Why the script is not in the tree: the stack allows no JavaScript beyond
# cmd/holzcloud/assets/htmx.min.js (CLAUDE.md), and a Playwright script is
# JavaScript. It lives outside the repository (UAT_JS); `run` copies it byte for
# byte to mac-t34-uat/script.txt, and the record pastes that copy, so its text and
# its output reach the repository only as the record's appendix. `run` and
# `record-check` refuse while the tracked JavaScript is anything but the htmx file.
#
# The flows (the script prints the keys; this file judges them):
#   UAT-01  a second administrator, created on /admin/users/new behind the password
#           question, is forced through two-step enrolment at the first sign-in,
#           sees ten recovery codes and the factor on /admin/konto, and is asked for
#           a code at the next sign-in; a recovery code works once
#   UAT-02  a visitor orders with invoice and, in a second order, with prepayment;
#           both orders are listed in the admin with their payment method (Payrexx
#           is never configured)
#   UAT-03  a contact message sent after a wait of at least four seconds arrives;
#           a submission that fills the honeypot gets the same answer and lands in
#           quarantine with its reason; every visitor request goes to one origin
#
# Nothing secret leaves UAT_DIR: the script makes its passwords at run time and
# prints counts, shapes and names, never a TOTP secret, a code, a recovery code, a
# password, a cookie value or an order token. leak_scan reads uat.txt and
# server-excerpt.txt in `run` and the whole record in `record-check`.
#
# The bound files of the evidence set (BOUND_FILES below): this file,
# tools/totpcode/main.go and plugins/kontaktformular/kontaktformular.zip (the
# archive the script uploads), together with the binary built from BINARY_SOURCE
# of the evidence commit (cmd, internal, go.mod, go.sum, changelog.go and the
# CHANGELOG.md it embeds). `run` refuses to start while one of them differs from
# HEAD or is untracked, or the binary compiles a file outside BINARY_SOURCE or a
# file that is untracked or gitignored, and pins NEW_REF to the short sha of HEAD.
#
# Environment:
#   UAT_DIR     absolute, outside the repository (and outside the main checkout of a
#               worktree); the driver creates it. Refused when it exists and holds
#               anything but what an earlier run of this driver left there.
#   UAT_JS      absolute path of the Playwright script, outside the repository; must exist
#   PW          the Playwright module directory (holds package.json); nothing is installed
#   PORT        18793, the dev server's port on 127.0.0.1; refused when something answers there
#   REHEARSAL   1 lets `run` start while a bound file differs from HEAD; summary.txt
#               then says REHEARSAL, and such a set is never evidence
#
# Names fixed for the phase: evidence directory deploy/test/out/mac-t34-uat (summary.txt,
# versions.txt, uat.txt, script.txt, server-excerpt.txt); run files
# deploy/test/out/mac-t34-evidence-commit.txt and deploy/test/out/mac-t34-pass1.{started,finished,status,log}.
# The server log, the script's stderr and its screenshots stay in UAT_DIR.
#
# The server: started by this file in the background, its pid kept; that pid and no
# other is stopped (SIGTERM, then a wait) at the end and on every exit path.
#
# Exit codes: 0 ok, 1 failure (a verdict that is not PASS included), 2 usage or a
# refused guard, 3 the platform cannot give the evidence (no go, node or curl).
#
# Written for bash 3.2 (macOS): no arrays, no mapfile, no case conversion. Run it
# with bash explicitly.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"
# The main checkout when this tree is a git worktree (the same directory otherwise).
MAIN_ROOT=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)

UAT_DIR=${UAT_DIR:-}
UAT_JS=${UAT_JS:-}
PW=${PW:-}
PORT=${PORT:-18793}
REHEARSAL=${REHEARSAL:-}

BASE_OUT=deploy/test/out
T34_OUT=$BASE_OUT/mac-t34-uat
RUN_PREFIX=$BASE_OUT/mac-t34-pass1
EVIDENCE_FILE=$BASE_OUT/mac-t34-evidence-commit.txt
BOUND_FILES="deploy/test/browser-verify.sh tools/totpcode/main.go plugins/kontaktformular/kontaktformular.zip"
# Everything of this module that ./cmd/holzcloud compiles or embeds: the root
# package is changelog.go, which embeds CHANGELOG.md. bound_check refuses a run
# while go list -deps names a file outside this list (binary_uncovered). The
# files are listed for linux/amd64, the target the drivers build, and
# bound_check also refuses a file the binary compiles or embeds that no commit
# holds, a gitignored one included (binary_untracked).
BINARY_SOURCE="cmd internal go.mod go.sum changelog.go CHANGELOG.md"
HTMX_JS=cmd/holzcloud/assets/htmx.min.js
# What a run leaves in UAT_DIR, and the mark that says a run of this file made it.
UAT_MARK=.browser-verify-t34
UAT_ENTRIES="$UAT_MARK holzcloud totpcode data server.log node.out node.err browser.txt shots"

say() { printf '%s\n' "$*" >&2; }
die() {
  say "browser-verify.sh: $*"
  exit 1
}
refuse() {
  say "browser-verify.sh: $*"
  exit 2
}
usage() {
  awk 'NR > 1 && /^$/ { exit } NR > 1 { print }' "${BASH_SOURCE[0]}" >&2
  exit 2
}

# ---------------------------------------------------------------------------
# Pure functions. No browser, no server, no network: arguments or files in, one
# answer out. `selftest` holds a table of cases, the failing ones included.
# ---------------------------------------------------------------------------

# inside P D: success when P is D or lies below it (textually).
inside() {
  case "$1" in
    "$2" | "$2"/*) return 0 ;;
  esac
  return 1
}

# phys_path P: P with its longest existing ancestor resolved to a physical path
# (symlinks followed), the rest appended unchanged. /tmp becomes /private/tmp.
phys_path() {
  local p=$1 rest="" d
  while [ ! -d "$p" ] && [ "$p" != / ] && [ -n "$p" ]; do
    rest="/$(basename "$p")$rest"
    p=$(dirname "$p")
  done
  d=$(cd -P "$p" 2> /dev/null && pwd) || d=$p
  [ "$d" = / ] && d=""
  printf '%s%s\n' "$d" "$rest"
}

# outside_repo P: success when P, logical and physical, is neither inside the
# repository nor inside the main checkout, and is no parent of either.
outside_repo() {
  local p=$1 q r
  q=$(phys_path "$p")
  for r in "$ROOT" "$MAIN_ROOT" "$(phys_path "$ROOT")" "$(phys_path "$MAIN_ROOT")"; do
    if inside "$p" "$r" || inside "$q" "$r" || inside "$r" "$p" || inside "$r" "$q"; then
      return 1
    fi
  done
  return 0
}

# safe_uat_dir P: an absolute path, no parent step, not the root, not inside the
# repository or the main checkout and no parent of them.
safe_uat_dir() {
  local p=${1:-}
  case "$p" in
    /*) ;;
    *) return 1 ;;
  esac
  case "$p" in
    / | */.. | */../* | */. | */./*) return 1 ;;
  esac
  [ "$p" != "${HOME:-/nonexistent}" ] || return 1
  outside_repo "$p"
}

# safe_script P: an absolute path of an existing file outside the repository.
safe_script() {
  local p=${1:-}
  case "$p" in
    /*) ;;
    *) return 1 ;;
  esac
  [ -f "$p" ] || return 1
  outside_repo "$p"
}

# uat_dir_left P: success when P is absent, empty, or holds the mark of this file
# and nothing but the entries a run of it leaves (UAT_ENTRIES).
uat_dir_left() {
  local p=$1 e
  [ -e "$p" ] || return 0
  [ -d "$p" ] || return 1
  [ -n "$(ls -A "$p")" ] || return 0
  [ -f "$p/$UAT_MARK" ] || return 1
  for e in $(ls -A "$p"); do
    case " $UAT_ENTRIES " in
      *" $e "*) ;;
      *) return 1 ;;
    esac
  done
  return 0
}

# kv_lines: from stdin, only the lines "KEY: value" (an upper-case key of letters,
# digits and dashes, a value that is not empty), unchanged and in order.
kv_lines() {
  grep -E '^[A-Z][A-Z0-9-]*: .' || true
}

# leak_scan FILE: success when no line of FILE holds what must never be pasted: a
# base32 run of 16 or more (a TOTP secret), a run of exactly 32 hex digits (an
# order token; a sha256 or a commit sha is longer), a basket or session cookie with
# its value, or a recovery code (five and five of its alphabet). Prints the number
# and kind of every such line, never the line.
leak_scan() {
  local f=${1:-} bad=0 n
  [ -f "$f" ] || {
    say "leak_scan: no file $f"
    return 1
  }
  for n in $(grep -nE '[A-Z2-7]{16,}' "$f" | cut -d: -f1); do
    say "leak_scan: $f line $n: a base32 run of 16 or more"
    bad=1
  done
  for n in $(grep -nE '(^|[^0-9a-f])[0-9a-f]{32}([^0-9a-f]|$)' "$f" | cut -d: -f1); do
    say "leak_scan: $f line $n: a run of 32 hex digits"
    bad=1
  done
  for n in $(grep -nE '(hc_warenkorb|holzcloud_session)=[^;[:space:]]' "$f" | cut -d: -f1); do
    say "leak_scan: $f line $n: a cookie with its value"
    bad=1
  done
  for n in $(grep -nE '(^|[^0-9a-z])[2-9b-df-hj-km-np-tv-xz]{5}-[2-9b-df-hj-km-np-tv-xz]{5}([^0-9a-z]|$)' "$f" | cut -d: -f1); do
    say "leak_scan: $f line $n: a recovery code"
    bad=1
  done
  [ "$bad" -eq 0 ]
}

# kv FILE KEY: the value of the one line "KEY: value"; empty when absent. kv_count
# FILE KEY: how many such lines there are.
kv() { sed -n "s/^$2: //p" "$1" 2> /dev/null | head -n 1; }
kv_count() { grep -c "^$2: " "$1" 2> /dev/null || true; }

# kv_is FILE KEY VALUE: the key occurs exactly once and has exactly that value.
kv_is() {
  [ "$(kv_count "$1" "$2")" = 1 ] && [ "$(kv "$1" "$2")" = "$3" ]
}
# kv_starts FILE KEY PREFIX: the key occurs exactly once and its value begins with PREFIX.
kv_starts() {
  [ "$(kv_count "$1" "$2")" = 1 ] || return 1
  case "$(kv "$1" "$2")" in
    "$3"*) return 0 ;;
  esac
  return 1
}

# verdict_uat01 FILE: PASS, FAIL or NOT-RUN (no file) for the UAT-01 keys of uat.txt.
verdict_uat01() {
  local f=${1:-}
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if kv_is "$f" ADMIN1-ENROLLED yes &&
    kv_is "$f" ADMIN2-CREATED yes &&
    kv_is "$f" PASSWORD-QUESTION yes &&
    kv_is "$f" FORCED-REDIRECT /admin/2fa/einrichten &&
    kv_is "$f" SECRET-SHOWN yes &&
    kv_is "$f" RECOVERY-CODES 10 &&
    kv_is "$f" KONTO-FACTOR on &&
    kv_is "$f" RELOGIN-ASKS-CODE yes &&
    kv_is "$f" RELOGIN-OK yes &&
    kv_starts "$f" RECOVERY-CODE-ONCE "yes "; then
    echo PASS
  else
    echo FAIL
  fi
}

# kv_names FILE KEY WORD...: the key occurs exactly once and its value, a list
# separated by commas, names every WORD as an item of its own.
kv_names() {
  local f=$1 k=$2 v w
  shift 2
  [ "$(kv_count "$f" "$k")" = 1 ] || return 1
  v=", $(kv "$f" "$k" | sed 's/ *, */, /g'),"
  for w in "$@"; do
    case "$v" in
      *", $w,"*) ;;
      *) return 1 ;;
    esac
  done
  return 0
}

# verdict_uat02 FILE: PASS, FAIL or NOT-RUN (no file) for the UAT-02 keys of uat.txt.
verdict_uat02() {
  local f=${1:-} n
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if ! kv_is "$f" PRODUCT-PAGE 200 ||
    ! kv_is "$f" COOKIE-BEFORE-ACTION none ||
    ! kv_is "$f" BASKET-COOKIE hc_warenkorb ||
    ! kv_names "$f" PAYMENT-OFFERED invoice prepay ||
    ! kv_is "$f" PAYREXX-OFFERED no ||
    ! kv_is "$f" ORDER-1-ADMIN-PAYMENT Rechnung ||
    ! kv_is "$f" ORDER-2-ADMIN-PAYMENT Vorauskasse; then
    echo FAIL
    return 0
  fi
  for n in 1 2; do
    if ! kv_starts "$f" "ORDER-$n-URL-SHAPE" "matches " || ! kv_is "$f" "ORDER-$n-ADMIN-LISTED" yes; then
      echo FAIL
      return 0
    fi
  done
  echo PASS
}

# verdict_uat03 FILE: PASS, FAIL or NOT-RUN (no file) for the UAT-03 keys of uat.txt.
verdict_uat03() {
  local f=${1:-} ms
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  ms=$(kv "$f" FORM-WAIT-MS)
  case "$ms" in
    "" | *[!0-9]*) ms=0 ;;
  esac
  if kv_is "$f" FORM-PAGE 200 &&
    [ "$(kv_count "$f" FORM-WAIT-MS)" = 1 ] && [ "$ms" -ge 4000 ] &&
    kv_is "$f" MESSAGE-IN-ADMIN yes &&
    kv_is "$f" SPAM-FIELD website &&
    kv_is "$f" SPAM-ANSWER-SAME yes &&
    kv_is "$f" SPAM-IN-QUARANTINE yes &&
    [ "$(kv_count "$f" QUARANTINE-REASON)" = 1 ] && [ -n "$(kv "$f" QUARANTINE-REASON)" ] &&
    kv_is "$f" SPAM-NOT-IN-MESSAGES yes &&
    kv_starts "$f" SAME-ORIGIN "yes"; then
    echo PASS
  else
    echo FAIL
  fi
}

# js_tracked_ok: the tracked JavaScript of the tree is exactly the htmx file.
js_tracked_ok() {
  [ "$(git ls-files '*.js' '*.mjs' '*.cjs')" = "$HTMX_JS" ]
}

# binary_covered P: P (relative to the repository) is an entry of BINARY_SOURCE
# or lies below one.
binary_covered() {
  local s
  for s in $BINARY_SOURCE; do
    case "${1:-}" in
      "" | /*) return 1 ;;
      "$s" | "$s"/*) return 0 ;;
    esac
  done
  return 1
}

# binary_files: the files of this module that ./cmd/holzcloud compiles or embeds
# for the target the drivers build (GOOS=linux GOARCH=amd64 CGO_ENABLED=0, so a
# file behind //go:build linux counts and one behind darwin does not, whatever
# machine this runs on), as paths relative to the repository, one per line. The
# one place that runs go list. Fails when go list fails.
binary_files() {
  local p here phys out
  here=$(pwd)
  phys=$(pwd -P)
  out=$(GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go list -deps -f '{{if .Module}}{{if .Module.Main}}{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}' ./cmd/holzcloud) || return 1
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    p=${p#"$here"/}
    p=${p#"$phys"/}
    printf '%s\n' "$p"
  done <<< "$out"
}

# binary_uncovered: the files binary_files lists that BINARY_SOURCE does not
# cover, one per line; empty when the list binds the whole binary. Fails when
# go list fails.
binary_uncovered() {
  local p out
  out=$(binary_files) || return 1
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    binary_covered "$p" || printf '%s\n' "$p"
  done <<< "$out"
}

# binary_untracked: the files binary_files lists that git does not track, one
# per line: untracked and gitignored files alike, because a binary built from a
# file no commit holds is not the binary of any commit (IN-03 of the second
# review of phase 34). Only files the binary really compiles or embeds count; a
# Finder .DS_Store is neither. Fails when go list fails.
binary_untracked() {
  local p out
  out=$(binary_files) || return 1
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    git ls-files --error-unmatch -- "$p" > /dev/null 2>&1 || printf '%s\n' "$p"
  done <<< "$out"
}

# safe_out PATH: a path directly below deploy/test/out whose name begins with mac-t34-.
safe_out() {
  local p=${1:-} rest
  case "$p" in
    deploy/test/out/mac-t34-?*) ;;
    *) return 1 ;;
  esac
  rest=${p#deploy/test/out/}
  case "$rest" in
    */* | *..*) return 1 ;;
  esac
  return 0
}

# ---------------------------------------------------------------------------
# The run
# ---------------------------------------------------------------------------

# bound_check: the bound files are tracked and equal to HEAD. Prints one line per
# problem and fails when there is one.
bound_check() {
  local f bad=0 unc unt
  for f in $BOUND_FILES; do
    if ! git ls-files --error-unmatch -- "$f" > /dev/null 2>&1; then
      say "browser-verify.sh: bound file $f is not tracked"
      bad=1
    elif ! git diff --quiet HEAD -- "$f"; then
      say "browser-verify.sh: bound file $f differs from HEAD"
      bad=1
    fi
  done
  if ! git diff --quiet HEAD -- $BINARY_SOURCE || [ -n "$(git ls-files --others --exclude-standard -- $BINARY_SOURCE)" ]; then
    say "browser-verify.sh: the binary's source ($BINARY_SOURCE) differs from HEAD"
    bad=1
  fi
  if ! unc=$(binary_uncovered); then
    say "browser-verify.sh: go list -deps ./cmd/holzcloud failed, so the binary's source cannot be bound"
    bad=1
  else
    if [ -n "$unc" ]; then
      say "browser-verify.sh: the binary compiles or embeds files BINARY_SOURCE does not bind: $(printf '%s\n' "$unc" | tr '\n' ' ')"
      bad=1
    fi
    if ! unt=$(binary_untracked); then
      say "browser-verify.sh: go list -deps ./cmd/holzcloud failed, so the binary's source cannot be bound"
      bad=1
    elif [ -n "$unt" ]; then
      say "browser-verify.sh: the binary compiles or embeds files that no commit holds (untracked or gitignored): $(printf '%s\n' "$unt" | tr '\n' ' ')"
      bad=1
    fi
  fi
  [ "$bad" -eq 0 ]
}

# port_answers: success when something answers HTTP on 127.0.0.1:PORT.
port_answers() {
  local rc=0
  curl -s -o /dev/null --max-time 2 "http://127.0.0.1:$PORT/" || rc=$?
  [ "$rc" -ne 7 ] && [ "$rc" -ne 28 ]
}

SRV_PID=""
# start_server: the binary of UAT_DIR on UAT_DIR/data, loopback, a clean environment.
start_server() {
  : > "$UAT_DIR/server.log"
  env -i PATH="$PATH" HOME="$HOME" TMPDIR="${TMPDIR:-/tmp}" \
    HOLZCLOUD_DATA_DIR="$UAT_DIR/data" \
    HOLZCLOUD_PORT="$PORT" \
    HOLZCLOUD_LISTEN=127.0.0.1 \
    HOLZCLOUD_SMTP_HOST=127.0.0.1 \
    HOLZCLOUD_SMTP_PORT=2 \
    HOLZCLOUD_SMTP_FROM=cms@example.test \
    "$UAT_DIR/holzcloud" < /dev/null >> "$UAT_DIR/server.log" 2>&1 &
  SRV_PID=$!
  say "browser-verify: server started, pid $SRV_PID, port $PORT, data $UAT_DIR/data"
}

# stop_server: the pid start_server kept, and no other. SIGTERM, a wait of up to
# twenty seconds, then SIGKILL to the same pid.
stop_server() {
  local i=0
  [ -n "$SRV_PID" ] || return 0
  if kill -0 "$SRV_PID" 2> /dev/null; then
    kill -TERM "$SRV_PID" 2> /dev/null || true
    while kill -0 "$SRV_PID" 2> /dev/null && [ "$i" -lt 40 ]; do
      sleep 0.5
      i=$((i + 1))
    done
    if kill -0 "$SRV_PID" 2> /dev/null; then
      say "browser-verify: server pid $SRV_PID did not stop on SIGTERM; SIGKILL"
      kill -KILL "$SRV_PID" 2> /dev/null || true
    fi
  fi
  wait "$SRV_PID" 2> /dev/null || true
  say "browser-verify: server pid $SRV_PID stopped"
  SRV_PID=""
}

# wait_ready: /admin/login answers below 500 within thirty seconds.
wait_ready() {
  local i=0 code
  while [ "$i" -lt 150 ]; do
    kill -0 "$SRV_PID" 2> /dev/null || die "the server exited before it answered (see $UAT_DIR/server.log)"
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "http://127.0.0.1:$PORT/admin/login" || true)
    case "$code" in
      2?? | 3?? | 4??) return 0 ;;
    esac
    sleep 0.2
    i=$((i + 1))
  done
  die "the server did not answer on port $PORT within thirty seconds"
}

# excerpt_of LOG DIR: the server log lines a record cites, from the JSON log LOG:
# the start, the listening address, one line counting the applied migrations with
# their lowest and highest version, the pre-upgrade snapshot, every plugin line,
# every second factor switched on, every warning and error, the shutdown; DIR is
# written as $UAT_DIR and each line is cut to 300 characters. Last, the line count.
excerpt_of() {
  local log=$1 dir=$2
  awk -v dir="$dir" '
    function out(s,   i, r) {
      r = ""
      while (dir != "" && (i = index(s, dir)) > 0) { r = r substr(s, 1, i - 1) "$UAT_DIR"; s = substr(s, i + length(dir)) }
      print substr(r s, 1, 300)
    }
    /"msg":"migration applied"/ {
      n++
      v = $0; sub(/.*"version":/, "", v); sub(/[^0-9].*/, "", v); v += 0
      if (lo == "" || v < lo) lo = v
      if (v > hi) hi = v
      if (!said) { said = 1; mark = NR }
      next
    }
    /"msg":"(starting|server starting|pre-upgrade snapshot written|shutting down|shutdown complete|two factor enabled)"/ ||
    /"msg":"[^"]*plugin/ || /"level":"(WARN|ERROR)"/ { out($0) }
    END {
      if (n) printf "migrations applied at this start: %d (versions %d to %d)\n", n, lo, hi
      else print "migrations applied at this start: 0"
      printf "server log lines: %d\n", NR
    }
  ' "$log"
}

# write_excerpt: excerpt_of over the run's server log.
write_excerpt() {
  excerpt_of "$UAT_DIR/server.log" "$UAT_DIR" > "$T34_OUT/server-excerpt.txt"
}

# write_versions: host, tools, the binary and the script.
write_versions() {
  local f=$T34_OUT/versions.txt pwv
  pwv=$(sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' "$PW/package.json" | head -n 1)
  {
    echo "host:                  $(uname -srm), macOS $(sw_vers -productVersion 2> /dev/null || echo '?')"
    echo "go:                    $(go version)"
    echo "node:                  $(node --version)"
    echo "playwright:            ${pwv:-?}"
    echo "browser:               $(cat "$UAT_DIR/browser.txt" 2> /dev/null || echo '?')"
    echo "binary:                $("$UAT_DIR/holzcloud" version 2> /dev/null | head -n 1)"
    echo "NEW_REF:               $NEW_REF"
    echo "evidence commit:       $(cat "$EVIDENCE_FILE")"
    echo "script sha256:         $(shasum -a 256 "$UAT_JS" | cut -d' ' -f1)"
  } > "$f"
  cat "$f" >&2
}

# write_summary: the verdicts from uat.txt.
write_summary() {
  local f=$T34_OUT/summary.txt
  {
    echo "---- summary (mac, dev server, Chromium) ----"
    echo "UAT-01 $(verdict_uat01 "$T34_OUT/uat.txt")"
    echo "UAT-02 $(verdict_uat02 "$T34_OUT/uat.txt")"
    echo "UAT-03 $(verdict_uat03 "$T34_OUT/uat.txt")"
    echo "BINARY: $("$UAT_DIR/holzcloud" version 2> /dev/null | head -n 1)"
    if [ -n "$REHEARSAL" ]; then
      echo "REHEARSAL: yes (bound files were allowed to differ from HEAD; this set is not evidence)"
    fi
  } > "$f"
  cat "$f" >&2
}

# run_body: everything `run` does after its guards; its output goes to the log.
run_body() {
  local rc=0 zip
  trap 'stop_server' EXIT
  trap 'stop_server; exit 130' INT TERM
  say "browser-verify: NEW_REF pinned to $NEW_REF, UAT_DIR $UAT_DIR, port $PORT"
  rm -rf "${UAT_DIR:?}"
  mkdir -p "$UAT_DIR"
  : > "$UAT_DIR/$UAT_MARK"
  rm -rf "${T34_OUT:?}"
  mkdir -p "$T34_OUT"
  say "browser-verify: building the binary and tools/totpcode"
  CGO_ENABLED=0 go build -trimpath \
    -ldflags="-X main.Version=$(git describe --tags --always) -X main.Commit=$NEW_REF" \
    -o "$UAT_DIR/holzcloud" ./cmd/holzcloud
  go build -o "$UAT_DIR/totpcode" ./tools/totpcode
  cp "$UAT_JS" "$T34_OUT/script.txt"
  cmp -s "$UAT_JS" "$T34_OUT/script.txt" || die "script.txt is not a byte copy of $UAT_JS"
  port_answers && die "something answers on 127.0.0.1:$PORT already"
  start_server
  wait_ready
  zip="$ROOT/plugins/kontaktformular/kontaktformular.zip"
  say "browser-verify: running the script"
  set +e
  S="$UAT_DIR" PW="$PW" PORT="$PORT" ZIP="$zip" node "$UAT_JS" > "$UAT_DIR/node.out" 2> "$UAT_DIR/node.err"
  rc=$?
  set -e
  say "browser-verify: the script ended with status $rc ($(wc -l < "$UAT_DIR/node.err" | tr -d ' ') lines on stderr, kept in $UAT_DIR/node.err)"
  stop_server
  kv_lines < "$UAT_DIR/node.out" > "$T34_OUT/uat.txt"
  write_excerpt
  if ! leak_scan "$T34_OUT/uat.txt" || ! leak_scan "$T34_OUT/server-excerpt.txt"; then
    mv "$T34_OUT/uat.txt" "$UAT_DIR/uat.txt.refused"
    mv "$T34_OUT/server-excerpt.txt" "$UAT_DIR/server-excerpt.txt.refused"
    say "browser-verify.sh: leak_scan refused the output; it is moved to $UAT_DIR and nothing is pasted"
    write_versions
    write_summary
    return 1
  fi
  grep -E '^([A-Z][A-Z0-9-]*: |note: )' "$UAT_DIR/node.out" >&2 || true
  write_versions
  write_summary
  [ "$rc" -eq 0 ] || return 1
  grep -Eq '^UAT-0[0-9] (FAIL|NOT-RUN)$' "$T34_OUT/summary.txt" && return 1
  return 0
}

cmd_run() {
  local rc p
  [ -n "$UAT_DIR" ] || refuse "UAT_DIR is not set"
  safe_uat_dir "$UAT_DIR" || refuse "UAT_DIR '$UAT_DIR' must be absolute, outside the repository and the main checkout, and no parent of them"
  uat_dir_left "$UAT_DIR" || refuse "UAT_DIR '$UAT_DIR' exists and holds something an earlier run of this driver did not leave there"
  [ -n "$UAT_JS" ] || refuse "UAT_JS is not set"
  safe_script "$UAT_JS" || refuse "UAT_JS '$UAT_JS' must be an existing file outside the repository"
  [ -n "$PW" ] && [ -f "$PW/package.json" ] || refuse "PW '$PW' is not a Playwright module directory"
  case "$PORT" in
    "" | *[!0-9]*) refuse "PORT '$PORT' is not a number" ;;
  esac
  [ "$PORT" -ge 1024 ] && [ "$PORT" -le 65535 ] || refuse "PORT $PORT is out of range"
  for p in "$T34_OUT" "$RUN_PREFIX.log" "$EVIDENCE_FILE"; do
    safe_out "$p" || refuse "refusing path $p"
  done
  js_tracked_ok || die "the tracked JavaScript is not exactly $HTMX_JS: $(git ls-files '*.js' '*.mjs' '*.cjs' | tr '\n' ' ')"
  if ! bound_check; then
    if [ -n "$REHEARSAL" ]; then
      say "browser-verify.sh: REHEARSAL=1: going on although a bound file differs from HEAD; this set is not evidence"
    else
      die "a bound file differs from HEAD or is untracked; commit first (the evidence is bound to one commit)"
    fi
  fi
  for p in go node curl shasum; do
    command -v "$p" > /dev/null 2>&1 || {
      say "browser-verify.sh: $p is not installed"
      exit 3
    }
  done
  port_answers && refuse "something answers on 127.0.0.1:$PORT; choose another PORT"
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
  say "browser-verify: run ended with status $rc"
  exit "$rc"
}

# ---------------------------------------------------------------------------
# The record (plan 34-06 runs these; they are here so that the driver bound by
# the evidence is the one that checks it). They never rely on `set -e`, print
# one line per problem and end on an answer line.
# ---------------------------------------------------------------------------

# t34_paste_list: DIRECTORY/FILE (or FILE, for the run files) of the appendix, in
# order. A list, never a glob: the run log, the started and finished stamps and
# everything in UAT_DIR (server log, stderr, screenshots) are not on it.
t34_paste_list() {
  printf '%s\n' \
    mac-t34-uat/summary.txt \
    mac-t34-uat/versions.txt \
    mac-t34-uat/uat.txt \
    mac-t34-uat/script.txt \
    mac-t34-uat/server-excerpt.txt \
    mac-t34-evidence-commit.txt \
    mac-t34-pass1.status
}

# appendix_print E: the blocks of the paste list from the files below E, each
# "#### NAME", a blank line and a fenced text block. Refuses (prints nothing) when
# a file is missing or holds a line that would end a fence or open a heading.
appendix_print() {
  local e=${1:-deploy/test/out} p bad=0
  for p in $(t34_paste_list); do
    if [ ! -f "$e/$p" ]; then
      say "browser-verify.sh: appendix: no scratch file $e/$p"
      bad=$((bad + 1))
    elif grep -Eq '^(```|#### |## )' "$e/$p"; then
      say "browser-verify.sh: appendix: $e/$p has a line that begins with three backticks, with '#### ' or with '## '; it cannot be pasted unchanged"
      bad=$((bad + 1))
    fi
  done
  if [ "$bad" -ne 0 ]; then
    return 1
  fi
  for p in $(t34_paste_list); do
    printf '#### %s\n\n' "$p"
    printf '%s\n' '```text'
    awk 1 "$e/$p"
    printf '%s\n\n' '```'
  done
}

# t34_record_check R E: the record R against the scratch root E.
#  - every "#### NAME" block under "## Appendix: raw output" equals E/NAME byte for
#    byte (the script text in mac-t34-uat/script.txt included), and every file of
#    the paste list has a block;
#  - every `mac-t34-...` token in backticks outside the appendix is the directory
#    mac-t34-uat, has a block, or stands on its line as "`TOKEN` (not pasted)";
#  - exactly one line "Evidence commit: SHA"; SHA resolves, is the pinned commit of
#    E/mac-t34-evidence-commit.txt, and no bound file nor anything of
#    BINARY_SOURCE differs between SHA and HEAD;
#  - the tracked JavaScript is exactly the htmx file;
#  - the pasted summary is not a rehearsal's and reads UAT-01, UAT-02 and UAT-03
#    PASS, and the pasted mac-t34-pass1.status is exactly 0;
#  - no line holds a private-key marker; leak_scan passes over the whole record
#    outside the script block.
t34_record_check() {
  local r=${1:-} e=${2:-} n=0 k=0 tmp i name sha pinned p tok bf v
  rc_bad() {
    k=$((k + 1))
    printf 'RECORD-CHECK BAD %s\n' "$*"
  }
  if [ ! -f "$r" ]; then
    printf 'RECORD-CHECK BAD no record %s\n' "$r"
    echo "RECORD-CHECK FAILED (1 of 1)"
    return 1
  fi
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/t34-record.XXXXXX") || return 1
  # Split the appendix into numbered block files and a list "INDEX NAME".
  awk -v d="$tmp" '
    $0 == "## Appendix: raw output" { app = 1; next }
    app && !infence && /^## / { app = 0 }
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
  for p in $(t34_paste_list); do
    n=$((n + 1))
    awk -v p="$p" '$2 == p { f = 1 } END { exit !f }' "$tmp/names" || rc_bad "block $p: the paste list has it, the appendix does not"
  done
  # Tokens outside the appendix.
  awk '$0 == "## Appendix: raw output" { exit } { print }' "$r" > "$tmp/prose"
  grep -o '`mac-t34-[A-Za-z0-9._/-]*`' "$tmp/prose" | tr -d '`' | sort -u > "$tmp/tokens" || true
  while read -r tok; do
    n=$((n + 1))
    [ "$tok" = mac-t34-uat ] && continue
    awk -v p="$tok" '$2 == p { f = 1 } END { exit !f }' "$tmp/names" && continue
    grep -qF "\`$tok\` (not pasted)" "$tmp/prose" && continue
    rc_bad "cited file $tok has no block in the appendix and is not named as not pasted"
  done < "$tmp/tokens"
  # The evidence commit.
  n=$((n + 1))
  if [ "$(grep -c '^Evidence commit: ' "$r" || true)" != 1 ]; then
    rc_bad "the record must hold exactly one line 'Evidence commit: SHA'"
  else
    sha=$(sed -n 's/^Evidence commit: `\{0,1\}\([0-9a-f]\{7,40\}\)`\{0,1\} *$/\1/p' "$r")
    pinned=$(cat "$e/mac-t34-evidence-commit.txt" 2> /dev/null || true)
    if [ -z "$sha" ] || ! git cat-file -e "$sha^{commit}" 2> /dev/null; then
      rc_bad "Evidence commit '${sha:-?}' does not resolve to a commit"
    elif [ -z "$pinned" ] || [ "$(git rev-parse "$sha^{commit}")" != "$(git rev-parse "$pinned^{commit}" 2> /dev/null || true)" ]; then
      rc_bad "Evidence commit $sha is not the pinned commit '$pinned' of $e/mac-t34-evidence-commit.txt"
    else
      for bf in $BOUND_FILES; do
        n=$((n + 1))
        git diff --quiet "$sha" HEAD -- "$bf" || rc_bad "bound file $bf differs between $sha and HEAD"
      done
      n=$((n + 1))
      git diff --quiet "$sha" HEAD -- $BINARY_SOURCE || rc_bad "the binary's source ($BINARY_SOURCE) differs between $sha and HEAD (the binary is not the one of the evidence)"
    fi
  fi
  # No JavaScript beyond htmx entered the tree.
  n=$((n + 1))
  js_tracked_ok || rc_bad "the tracked JavaScript is not exactly $HTMX_JS"
  # The summary of a rehearsal is never evidence.
  n=$((n + 1))
  i=$(awk '$2 == "mac-t34-uat/summary.txt" { print $1; exit }' "$tmp/names")
  if [ -n "$i" ] && [ -f "$tmp/b$i" ] && grep -q '^REHEARSAL:' "$tmp/b$i"; then
    rc_bad "the pasted summary is a rehearsal's (REHEARSAL line)"
  fi
  # Nor is a failed set: UAT-01 to UAT-03 read PASS, once each, and the pasted
  # status of the run is 0. Prose claiming PASS over an appendix that shows
  # FAIL is what this refuses.
  if [ -n "$i" ] && [ -f "$tmp/b$i" ]; then
    for v in UAT-01 UAT-02 UAT-03; do
      n=$((n + 1))
      if [ "$(grep -c "^$v " "$tmp/b$i" || true)" != 1 ] || ! grep -qx "$v PASS" "$tmp/b$i"; then
        rc_bad "the pasted summary does not read '$v PASS' exactly once"
      fi
    done
  else
    n=$((n + 1))
    rc_bad "no pasted mac-t34-uat/summary.txt to read the verdicts from"
  fi
  n=$((n + 1))
  i=$(awk '$2 == "mac-t34-pass1.status" { print $1; exit }' "$tmp/names")
  printf '0\n' > "$tmp/zero"
  if [ -z "$i" ] || [ ! -f "$tmp/b$i" ] || ! cmp -s "$tmp/zero" "$tmp/b$i"; then
    rc_bad "the pasted mac-t34-pass1.status is not exactly the one line 0: the run did not end well"
  fi
  # No secret.
  n=$((n + 1))
  if grep -q 'PRIVATE KEY-----' "$r"; then
    rc_bad "a line holds a private-key marker"
  fi
  n=$((n + 1))
  awk '
    $0 == "#### mac-t34-uat/script.txt" { skip = 1; next }
    skip == 1 && $0 == "```text" { skip = 2; next }
    skip == 2 && $0 == "```" { skip = 0; next }
    skip == 2 { next }
    { print }
  ' "$r" > "$tmp/scan"
  leak_scan "$tmp/scan" 2> "$tmp/leaks" || rc_bad "leak_scan: $(wc -l < "$tmp/leaks" | tr -d ' ') line(s) outside the script block hold a secret's shape ($(sed 's/^leak_scan: [^ ]* //' "$tmp/leaks" | tr '\n' ';' | sed 's/;$//'))"
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
  t34_record_check "${R:-deploy/BROWSER-RUN.md}" "${E:-deploy/test/out}" || rc=$?
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

selftest_guards() {
  local f=$ST_TMP/uat.js d=$ST_TMP/left
  printf 'x\n' > "$f"
  st_fail "safe_uat_dir: a relative path" safe_uat_dir scratch/t34-uat
  st_fail "safe_uat_dir: empty" safe_uat_dir ""
  st_fail "safe_uat_dir: the repository itself" safe_uat_dir "$ROOT"
  st_fail "safe_uat_dir: a path inside the repository" safe_uat_dir "$ROOT/deploy/test/out/uat"
  st_fail "safe_uat_dir: the main checkout" safe_uat_dir "$MAIN_ROOT"
  st_fail "safe_uat_dir: a path inside the main checkout" safe_uat_dir "$MAIN_ROOT/data/uat"
  st_fail "safe_uat_dir: a parent of the repository" safe_uat_dir "$(dirname "$MAIN_ROOT")"
  st_fail "safe_uat_dir: the root" safe_uat_dir /
  st_fail "safe_uat_dir: a parent step" safe_uat_dir "$ST_TMP/a/../b"
  st_ok "safe_uat_dir: an absolute path elsewhere" safe_uat_dir "$ST_TMP/run"
  st_ok "safe_script: an existing file elsewhere" safe_script "$f"
  st_fail "safe_script: a missing file" safe_script "$ST_TMP/absent.js"
  st_fail "safe_script: a relative path" safe_script uat.js
  st_fail "safe_script: a file inside the repository" safe_script "$ROOT/deploy/test/backup-verify.sh"
  st_ok "uat_dir_left: absent" uat_dir_left "$d"
  mkdir -p "$d"
  st_ok "uat_dir_left: empty" uat_dir_left "$d"
  : > "$d/server.log"
  st_fail "uat_dir_left: a run's file without the mark" uat_dir_left "$d"
  : > "$d/$UAT_MARK"
  mkdir -p "$d/data"
  st_ok "uat_dir_left: the mark and a run's entries" uat_dir_left "$d"
  : > "$d/notes.txt"
  st_fail "uat_dir_left: something a run does not leave" uat_dir_left "$d"
  st_ok "js_tracked_ok: the tree holds only the htmx file" js_tracked_ok
}

selftest_excerpt() {
  local log=$ST_TMP/server.log got
  printf '%s\n' \
    '{"time":"t1","level":"INFO","msg":"starting","version":"v9","commit":"abc1234"}' \
    '{"time":"t2","level":"INFO","msg":"sqlite opened","path":"/x+y/data/holzcloud.sqlite"}' \
    '{"time":"t3","level":"INFO","msg":"migration applied","version":1,"type":"sql"}' \
    '{"time":"t4","level":"INFO","msg":"migration applied","version":69,"type":"sql"}' \
    '{"time":"t5","level":"INFO","msg":"pre-upgrade snapshot written","path":"/x+y/data/pre-upgrade.sqlite"}' \
    '{"time":"t6","level":"INFO","msg":"plugin installed","id":"kontaktformular"}' \
    '{"time":"t7","level":"WARN","msg":"mail delivery failed","attempt":1}' \
    '{"time":"t8","level":"INFO","msg":"shutdown complete"}' > "$log"
  got=$(excerpt_of "$log" /x+y)
  st_eq "excerpt_of: the cited lines, the directory named, the migrations counted" '{"time":"t1","level":"INFO","msg":"starting","version":"v9","commit":"abc1234"}
{"time":"t5","level":"INFO","msg":"pre-upgrade snapshot written","path":"$UAT_DIR/data/pre-upgrade.sqlite"}
{"time":"t6","level":"INFO","msg":"plugin installed","id":"kontaktformular"}
{"time":"t7","level":"WARN","msg":"mail delivery failed","attempt":1}
{"time":"t8","level":"INFO","msg":"shutdown complete"}
migrations applied at this start: 2 (versions 1 to 69)
server log lines: 8' "$got"
}

selftest_kv_lines() {
  local got
  got=$(printf '%s\n' 'note: setting up' 'ADMIN2-CREATED: yes' 'lower: no' 'RECOVERY-CODES: 10' \
    '  UAT-01: indented' 'ORDER-1-NUMBER: 2026-0001' 'Mixed-Case: no' 'NOVALUE:' 'ERROR: something' '=== STEP' | kv_lines)
  st_eq "kv_lines: only KEY: value lines" "ADMIN2-CREATED: yes
RECOVERY-CODES: 10
ORDER-1-NUMBER: 2026-0001
ERROR: something" "$got"
  st_eq "kv_lines: nothing in, nothing out" "" "$(printf 'note: x\n' | kv_lines)"
}

selftest_leak_scan() {
  local f=$ST_TMP/leak.txt
  printf '%s\n' 'SECRET-SHOWN: yes' 'RECOVERY-CODES: 10' 'RECOVERY-CODE-SHAPE: 5-5 lower-case alphanumeric' \
    'ORDER-1-URL-SHAPE: matches /bestellung/[0-9a-f]{32}' 'BASKET-COOKIE: hc_warenkorb' 'SAME-ORIGIN: yes (41 requests, 1 origin)' \
    'script sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' 'NEW_REF: cfce101' > "$f"
  st_ok "leak_scan: the documented key lines pass" leak_scan "$f"
  printf 'SECRET: JBSWY3DPEHPK3PXPJBSWY3DP\n' > "$f"
  st_fail "leak_scan: a base32 run of 16 or more" leak_scan "$f"
  printf 'x JBSW Y3DP EHPK 3PXP\n' > "$f"
  st_ok "leak_scan: four base32 groups of four are not a run" leak_scan "$f"
  printf 'ORDER: /bestellung/0123456789abcdef0123456789abcdef done\n' > "$f"
  st_fail "leak_scan: a 32-hex run" leak_scan "$f"
  printf 'COOKIE: hc_warenkorb=abc123\n' > "$f"
  st_fail "leak_scan: hc_warenkorb= with a value" leak_scan "$f"
  printf 'COOKIE: holzcloud_session=abc123\n' > "$f"
  st_fail "leak_scan: holzcloud_session= with a value" leak_scan "$f"
  printf 'CODE: 2b3c4-d5f6g\n' > "$f"
  st_fail "leak_scan: a recovery code" leak_scan "$f"
  st_fail "leak_scan: no file" leak_scan "$ST_TMP/absent.txt"
}

# fx_uat01 FILE [KEY VALUE]: the key lines of a passing UAT-01, one value replaced
# (the value "-" removes the key).
fx_uat01() {
  local f=$1 k=${2:-} v=${3:-}
  printf '%s\n' \
    'ADMIN1-ENROLLED: yes' \
    'ADMIN2-CREATED: yes' \
    'PASSWORD-QUESTION: yes' \
    'FORCED-REDIRECT: /admin/2fa/einrichten' \
    'SECRET-SHOWN: yes' \
    'RECOVERY-CODES: 10' \
    'RECOVERY-CODE-SHAPE: 5-5 lower-case alphanumeric' \
    'KONTO-FACTOR: on' \
    'RELOGIN-ASKS-CODE: yes' \
    'RELOGIN-OK: yes' \
    'RECOVERY-CODE-ONCE: yes (accepted once, refused the second time)' > "$f"
  if [ -n "$k" ]; then
    awk -v k="$k" -v v="$v" 'index($0, k ": ") == 1 { if (v != "-") print k ": " v; next } { print }' "$f" > "$f.x" && mv "$f.x" "$f"
  fi
}

selftest_verdict_uat01() {
  local f=$ST_TMP/uat.txt kv
  fx_uat01 "$f"
  st_eq "verdict_uat01: every key right" PASS "$(verdict_uat01 "$f")"
  st_eq "verdict_uat01: no file" NOT-RUN "$(verdict_uat01 "$ST_TMP/absent.txt")"
  for kv in 'ADMIN2-CREATED|no' 'FORCED-REDIRECT|/admin/' 'SECRET-SHOWN|no' 'RECOVERY-CODES|9' \
    'KONTO-FACTOR|off' 'RELOGIN-ASKS-CODE|no' 'RELOGIN-OK|no' 'PASSWORD-QUESTION|no' 'ADMIN1-ENROLLED|no' \
    'RECOVERY-CODE-ONCE|no (accepted twice)' 'RECOVERY-CODES|-' 'FORCED-REDIRECT|-'; do
    fx_uat01 "$f" "${kv%%|*}" "${kv#*|}"
    st_eq "verdict_uat01: ${kv%%|*} '${kv#*|}'" FAIL "$(verdict_uat01 "$f")"
  done
  fx_uat01 "$f"
  printf 'RECOVERY-CODES: 10\n' >> "$f"
  st_eq "verdict_uat01: a key twice" FAIL "$(verdict_uat01 "$f")"
}

# fx_keys FILE LINES [KEY VALUE]: LINES (one "KEY: value" per line) into FILE, one
# value replaced (the value "-" removes the key).
fx_keys() {
  local f=$1 k=${3:-} v=${4:-}
  printf '%s\n' "$2" > "$f"
  if [ -n "$k" ]; then
    awk -v k="$k" -v v="$v" 'index($0, k ": ") == 1 { if (v != "-") print k ": " v; next } { print }' "$f" > "$f.x" && mv "$f.x" "$f"
  fi
}

FX_UAT02='FIXTURE-SQL: none
PRODUCT-PAGE: 200
COOKIE-BEFORE-ACTION: none
BASKET-COOKIE: hc_warenkorb
PAYMENT-OFFERED: invoice, prepay
PAYREXX-OFFERED: no
ORDER-1-URL-SHAPE: matches /bestellung/[0-9a-f]{32}
ORDER-1-NUMBER: 2026-0001
ORDER-1-ADMIN-LISTED: yes
ORDER-1-ADMIN-PAYMENT: Rechnung
ORDER-1-ADMIN-STATUS: Neu
ORDER-2-URL-SHAPE: matches /bestellung/[0-9a-f]{32}
ORDER-2-NUMBER: 2026-0002
ORDER-2-ADMIN-LISTED: yes
ORDER-2-ADMIN-PAYMENT: Vorauskasse
ORDER-2-ADMIN-STATUS: Neu'

FX_UAT03='FORM-PAGE: 200
FORM-WAIT-MS: 4512
MESSAGE-ANSWER: formular=gesendet, Danke
MESSAGE-IN-ADMIN: yes
SPAM-FIELD: website
SPAM-ANSWER-SAME: yes
SPAM-IN-QUARANTINE: yes
QUARANTINE-REASON: Das versteckte Feld wurde ausgefüllt
SPAM-NOT-IN-MESSAGES: yes
SAME-ORIGIN: yes (57 requests, 1 origin)
JS: on'

selftest_verdict_uat02() {
  local f=$ST_TMP/uat02.txt kv
  fx_keys "$f" "$FX_UAT02"
  st_eq "verdict_uat02: every key right" PASS "$(verdict_uat02 "$f")"
  st_eq "verdict_uat02: no file" NOT-RUN "$(verdict_uat02 "$ST_TMP/absent.txt")"
  st_eq "verdict_uat02: the keys of UAT-01 only" FAIL "$(fx_uat01 "$f" && verdict_uat02 "$f")"
  for kv in 'PRODUCT-PAGE|404' 'COOKIE-BEFORE-ACTION|hc_warenkorb' 'BASKET-COOKIE|none' 'BASKET-COOKIE|hc_warenkorb2' \
    'PAYMENT-OFFERED|invoice' 'PAYMENT-OFFERED|prepay' 'PAYMENT-OFFERED|invoices, prepayment' 'PAYREXX-OFFERED|yes' \
    'ORDER-1-URL-SHAPE|does not match (/kasse)' 'ORDER-2-URL-SHAPE|does not match (/kasse)' \
    'ORDER-1-ADMIN-LISTED|no' 'ORDER-2-ADMIN-LISTED|no' 'ORDER-1-ADMIN-PAYMENT|Vorauskasse' 'ORDER-2-ADMIN-PAYMENT|Rechnung' \
    'ORDER-1-ADMIN-PAYMENT|-' 'ORDER-2-URL-SHAPE|-' 'PAYREXX-OFFERED|-'; do
    fx_keys "$f" "$FX_UAT02" "${kv%%|*}" "${kv#*|}"
    st_eq "verdict_uat02: ${kv%%|*} '${kv#*|}'" FAIL "$(verdict_uat02 "$f")"
  done
}

selftest_verdict_uat03() {
  local f=$ST_TMP/uat03.txt kv
  fx_keys "$f" "$FX_UAT03"
  st_eq "verdict_uat03: every key right" PASS "$(verdict_uat03 "$f")"
  fx_keys "$f" "$FX_UAT03" FORM-WAIT-MS 4000
  st_eq "verdict_uat03: a wait of exactly 4000 ms" PASS "$(verdict_uat03 "$f")"
  st_eq "verdict_uat03: no file" NOT-RUN "$(verdict_uat03 "$ST_TMP/absent.txt")"
  for kv in 'FORM-PAGE|404' 'FORM-WAIT-MS|3999' 'FORM-WAIT-MS|soon' 'FORM-WAIT-MS|' 'MESSAGE-IN-ADMIN|no' \
    'SPAM-FIELD|email' 'SPAM-ANSWER-SAME|no' 'SPAM-IN-QUARANTINE|no' 'QUARANTINE-REASON|-' \
    'SPAM-NOT-IN-MESSAGES|no' 'SAME-ORIGIN|no (57 requests, 2 origins)' 'SAME-ORIGIN|-' 'MESSAGE-IN-ADMIN|-'; do
    fx_keys "$f" "$FX_UAT03" "${kv%%|*}" "${kv#*|}"
    st_eq "verdict_uat03: ${kv%%|*} '${kv#*|}'" FAIL "$(verdict_uat03 "$f")"
  done
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

# rc_of cmd args...: the status of the command, its output dropped.
rc_of() {
  local rc=0
  "$@" > /dev/null 2>&1 || rc=$?
  echo "$rc"
}

# fx_scratch E: a scratch root holding every file of the paste list, small texts.
fx_scratch() {
  local e=$1 p
  rm -rf "${e:?}"
  mkdir -p "$e/mac-t34-uat"
  for p in $(t34_paste_list); do
    printf 'line one of %s\nKEY: value\n' "$p" > "$e/$p"
  done
  printf 'UAT-01 PASS\nUAT-02 PASS\nUAT-03 PASS\n' > "$e/mac-t34-uat/summary.txt"
  printf '%s\n' "'use strict';" "const { chromium } = require(process.env.PW);" \
    "console.log('ORDER-1-URL-SHAPE: ' + (/^\/bestellung\/[0-9a-f]{32}\$/.test(u) ? 'matches' : 'no'));" \
    "const t = \`two \${x} words\`;" > "$e/mac-t34-uat/script.txt"
  printf '0\n' > "$e/mac-t34-pass1.status"
  git rev-parse --short HEAD > "$e/mac-t34-evidence-commit.txt"
}

# fx_record E R [EVIDENCE-LINE]: a record whose appendix is `appendix` over E.
fx_record() {
  local e=$1 r=$2 ev=${3:-}
  [ -n "$ev" ] || ev="Evidence commit: $(cat "$e/mac-t34-evidence-commit.txt")"
  {
    echo "# Browser run"
    echo
    echo "$ev"
    echo
    echo 'The verdicts are `mac-t34-uat/summary.txt`; the directory `mac-t34-uat` holds the rest,'
    echo 'the run log `mac-t34-pass1.log` (not pasted) the trace.'
    echo
    echo "## Appendix: raw output"
    echo
    appendix_print "$e"
  } > "$r"
}

# fx_edit R AWK-PROGRAM: rewrite the record R through awk.
fx_edit() {
  awk "$2" "$1" > "$1.x" && mv "$1.x" "$1"
}

selftest_record() {
  local e=$ST_TMP/e r=$ST_TMP/RUN.md out marker v
  fx_scratch "$e"
  fx_record "$e" "$r"
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a record equal to its files" 0 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the OK line" "RECORD-CHECK OK (" "$out"

  # One byte of one block differs from its file.
  fx_record "$e" "$r"
  fx_edit "$r" '{ if ($0 == "line one of mac-t34-uat/uat.txt") print "line one of mac-t34-uat/uat.txT"; else print }'
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a block differs by one byte" 1 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the differing block is named" "mac-t34-uat/uat.txt" "$out"

  # One byte of the script block differs from script.txt.
  fx_record "$e" "$r"
  fx_edit "$r" '{ if ($0 == "'"'"'use strict'"'"';") print "'"'"'use strict'"'"'"; else print }'
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: the script block differs by one byte" 1 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the script block is named" "mac-t34-uat/script.txt" "$out"

  # A DIRECTORY/FILE token outside the appendix without a block.
  fx_record "$e" "$r"
  fx_edit "$r" '{ print } /^Evidence commit: / { print ""; print "See `mac-t34-uat/notpasted.txt`." }'
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a cited file without a block" 1 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the cited file is named" "mac-t34-uat/notpasted.txt" "$out"

  # A run file cited without a block and without saying it is not pasted.
  fx_record "$e" "$r"
  fx_edit "$r" '{ print } /^Evidence commit: / { print ""; print "The run began at `mac-t34-pass1.started`." }'
  st_eq "record-check: a run file cited, neither pasted nor named as not pasted" 1 "$(rc_of t34_record_check "$r" "$e")"

  # Two Evidence commit lines.
  fx_record "$e" "$r"
  fx_edit "$r" '{ print } /^Evidence commit: / { print $0 }'
  st_eq "record-check: two Evidence commit lines" 1 "$(rc_of t34_record_check "$r" "$e")"

  # An Evidence commit that does not resolve.
  printf '0000000\n' > "$e/mac-t34-evidence-commit.txt"
  fx_record "$e" "$r"
  st_eq "record-check: an Evidence commit that does not resolve" 1 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"

  # An Evidence commit that resolves but is not the pinned one.
  fx_record "$e" "$r" "Evidence commit: $(git rev-parse --short 'HEAD~1')"
  st_eq "record-check: an Evidence commit other than the pinned one" 1 "$(rc_of t34_record_check "$r" "$e")"

  # A private-key marker anywhere (built from two halves, so this file holds none).
  marker="-----BEGIN OPENSSH PRIV""ATE KEY-----"
  fx_record "$e" "$r"
  printf '%s\n' "$marker" >> "$r"
  st_eq "record-check: a private-key marker" 1 "$(rc_of t34_record_check "$r" "$e")"

  # A line leak_scan rejects, in the prose and in a pasted block.
  fx_record "$e" "$r"
  fx_edit "$r" '{ print } /^Evidence commit: / { print ""; print "Token: 0123456789abcdef0123456789abcdef" }'
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: a 32-hex run in the prose" 1 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the leak is named" "leak_scan" "$out"
  printf 'SECRET: JBSWY3DPEHPK3PXPJBSWY3DP\n' >> "$e/mac-t34-uat/uat.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a base32 secret in the uat.txt block" 1 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"

  # The script block is outside leak_scan: its text is the script, byte for byte.
  printf '%s\n' "const sample = '0123456789abcdef0123456789abcdef';" >> "$e/mac-t34-uat/script.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a 32-hex run inside the script block only" 0 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"

  # A rehearsal's summary is never evidence.
  printf 'UAT-01 PASS\nUAT-02 PASS\nUAT-03 PASS\nREHEARSAL: yes\n' > "$e/mac-t34-uat/summary.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a rehearsal's summary" 1 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"

  # A failed set is never evidence either: each of the three verdicts, and the
  # run's status (WR-02 of the phase 34 review).
  for v in UAT-01 UAT-02 UAT-03; do
    printf -- '---- summary ----\nUAT-01 PASS\nUAT-02 PASS\nUAT-03 PASS\n' | sed "s/^$v PASS\$/$v FAIL/" > "$e/mac-t34-uat/summary.txt"
    fx_record "$e" "$r"
    out=$(t34_record_check "$r" "$e" 2>&1) || true
    st_eq "record-check: a FAIL summary ($v)" 1 "$(rc_of t34_record_check "$r" "$e")"
    st_has "record-check: the failed verdict $v is named" "$v" "$out"
  done
  printf 'UAT-01 PASS\nUAT-02 NOT-RUN\nUAT-03 PASS\n' > "$e/mac-t34-uat/summary.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a NOT-RUN verdict" 1 "$(rc_of t34_record_check "$r" "$e")"
  printf 'UAT-01 PASS\nUAT-03 PASS\n' > "$e/mac-t34-uat/summary.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a summary without one verdict line" 1 "$(rc_of t34_record_check "$r" "$e")"
  printf 'UAT-01 PASS\nUAT-02 PASS\nUAT-02 FAIL\nUAT-03 PASS\n' > "$e/mac-t34-uat/summary.txt"
  fx_record "$e" "$r"
  st_eq "record-check: a summary with one verdict line twice" 1 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"
  printf '1\n' > "$e/mac-t34-pass1.status"
  fx_record "$e" "$r"
  out=$(t34_record_check "$r" "$e" 2>&1) || true
  st_eq "record-check: status 1" 1 "$(rc_of t34_record_check "$r" "$e")"
  st_has "record-check: the status is named" "mac-t34-pass1.status" "$out"
  printf '0\n0\n' > "$e/mac-t34-pass1.status"
  fx_record "$e" "$r"
  st_eq "record-check: a status of two lines" 1 "$(rc_of t34_record_check "$r" "$e")"
  : > "$e/mac-t34-pass1.status"
  fx_record "$e" "$r"
  st_eq "record-check: an empty status" 1 "$(rc_of t34_record_check "$r" "$e")"
  fx_scratch "$e"

  # A block missing for a file of the paste list.
  fx_record "$e" "$r"
  fx_edit "$r" '/^#### mac-t34-uat\/server-excerpt.txt$/ { skip = 1 } skip && /^```$/ { skip = 0; next } !skip { print }'
  st_eq "record-check: a file of the paste list without a block" 1 "$(rc_of t34_record_check "$r" "$e")"

  # No record at all.
  st_eq "record-check: no record" 1 "$(rc_of t34_record_check "$ST_TMP/absent.md" "$e")"
}

selftest_appendix() {
  local e=$ST_TMP/e
  fx_scratch "$e"
  st_eq "appendix: every file pasteable" 0 "$(rc_of appendix_print "$e")"
  st_has "appendix: the first block" '#### mac-t34-uat/summary.txt' "$(appendix_print "$e")"
  st_has "appendix: the script block" '#### mac-t34-uat/script.txt' "$(appendix_print "$e")"
  printf 'x\n```\n' >> "$e/mac-t34-uat/uat.txt"
  st_eq "appendix: a line beginning with three backticks" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  printf '#### x\n' >> "$e/mac-t34-uat/script.txt"
  st_eq "appendix: a line beginning with '#### '" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  printf '## x\n' >> "$e/mac-t34-uat/server-excerpt.txt"
  st_eq "appendix: a line beginning with '## '" 1 "$(rc_of appendix_print "$e")"
  fx_scratch "$e"
  rm -f "$e/mac-t34-uat/versions.txt"
  st_eq "appendix: a file of the list is missing" 1 "$(rc_of appendix_print "$e")"
}

# The binary's source is everything of this module that ./cmd/holzcloud compiles
# or embeds: cmd, internal, go.mod, go.sum and the root package (changelog.go,
# which embeds CHANGELOG.md). IN-01 of the phase 34 review.
selftest_binary_source() {
  local p
  for p in cmd/holzcloud/main.go internal/db/db.go go.mod go.sum changelog.go CHANGELOG.md; do
    st_ok "binary_covered: $p" binary_covered "$p"
  done
  for p in changelog.go.orig cmdx/main.go sdk/sdk.go plugins/suche/main.go tools/totpcode/main.go README.md ""; do
    st_fail "binary_covered: '$p'" binary_covered "$p"
  done
  if command -v go > /dev/null 2>&1; then
    st_eq "binary_uncovered: this tree's binary is bound whole" "" "$(binary_uncovered 2>&1)"
    st_eq "binary_uncovered: the list before the review misses the root package" "changelog.go CHANGELOG.md" \
      "$(BINARY_SOURCE='cmd internal go.mod go.sum' binary_uncovered 2>&1 | tr '\n' ' ' | sed 's/ $//')"
  fi
}

# The binary's files are listed for linux/amd64 and every one of them must be
# tracked (IN-03 of the second review of phase 34), shown on a scratch module in
# the selftest's temporary directory, never on the real tree: cmd/holzcloud has a
# file behind //go:build linux that imports a package nothing else names, and a
# gitignored file in package main.
selftest_scratch_bound() { # DIR: bound_check in the scratch module, nothing bound but its source
  ( cd "$1" && export GOWORK=off GOFLAGS= && BOUND_FILES="" BINARY_SOURCE='cmd lnx' bound_check )
}
selftest_binary_files() {
  local d gover
  command -v go > /dev/null 2>&1 || return 0
  d=$ST_TMP/binmod
  gover=$(sed -n 's/^go //p' go.mod | head -n 1)
  mkdir -p "$d/cmd/holzcloud" "$d/lnx"
  printf 'module example.com/st\n\ngo %s\n' "$gover" > "$d/go.mod"
  printf 'package main\n\nfunc main() {}\n' > "$d/cmd/holzcloud/main.go"
  printf '//go:build linux\n\npackage main\n\nimport _ "example.com/st/lnx"\n' > "$d/cmd/holzcloud/main_linux.go"
  printf 'package lnx\n' > "$d/lnx/lnx.go"
  printf 'cmd/holzcloud/extra.go\n' > "$d/.gitignore"
  git -C "$d" init -q 2> /dev/null
  git -C "$d" add -A
  git -C "$d" -c user.name=st -c user.email=st@invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m scratch
  st_eq "binary_uncovered: the list is made for linux/amd64, so lnx/lnx.go shows" "lnx/lnx.go" \
    "$(cd "$d" && export GOWORK=off GOFLAGS= && BINARY_SOURCE='cmd' binary_uncovered 2>&1)"
  st_eq "binary_untracked: control, every file the binary compiles is tracked" "" \
    "$(cd "$d" && export GOWORK=off GOFLAGS= && BINARY_SOURCE='cmd lnx' binary_untracked 2>&1)"
  st_ok "bound_check: control, a scratch module whose binary is all tracked" selftest_scratch_bound "$d"
  printf 'package main\n' > "$d/cmd/holzcloud/extra.go"
  st_eq "binary_untracked: a gitignored file in package main is named" "cmd/holzcloud/extra.go" \
    "$(cd "$d" && export GOWORK=off GOFLAGS= && BINARY_SOURCE='cmd lnx' binary_untracked 2>&1)"
  st_fail "bound_check: refuses a gitignored file the binary compiles" selftest_scratch_bound "$d"
  st_has "bound_check: and says which file and why" "files that no commit holds (untracked or gitignored): cmd/holzcloud/extra.go" "$(selftest_scratch_bound "$d" 2>&1)"
}

cmd_selftest() {
  local t=${TMPDIR:-/tmp}
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${t%/}/browser-verify-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_guards
  selftest_excerpt
  selftest_kv_lines
  selftest_leak_scan
  selftest_verdict_uat01
  selftest_verdict_uat02
  selftest_verdict_uat03
  selftest_record
  selftest_appendix
  selftest_binary_source
  selftest_binary_files
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
    appendix) cmd_appendix ;;
    record-check) cmd_record_check ;;
    *) usage ;;
  esac
done
