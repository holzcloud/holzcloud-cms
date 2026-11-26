#!/usr/bin/env bash
# Phase 34: MIG-01, MIG-02, UPD-01 and UPD-02 played in fresh systemd-base containers.
#
# A sibling of deploy/test/run.sh in the shape of deploy/test/backup-verify.sh. It
# edits neither run.sh, nightly.sh nor backup-verify.sh and reaches run.sh through
# its public subcommands only (build up install seed transfer check); what this
# file adds is the scenarios of phase 34, the files they write and the rule that
# reads a verdict from those files, never from a trace.
#
#   upgrade-verify.sh selftest       the pure functions below, a table of cases, no Docker
#   upgrade-verify.sh run            the evidence: the stages of STAGES, each in its own
#                                    fresh containers, the raw files below deploy/test/out
#   upgrade-verify.sh down           remove the four holzcloud-t34-* containers, nothing else
#   upgrade-verify.sh appendix STAGE the appendix blocks of the fixed paste list of the
#                                    record of STAGE (mig, upd or rb), from the files below E
#   upgrade-verify.sh record-check   the proof of the record R against the scratch root E:
#                                    every block byte for byte, every cited file pasted, one
#                                    Evidence commit, pinned and unchanged since
#
# Subcommands can be chained in one call and run in order.
#
# The bound files of the evidence set (BOUND_FILES below): deploy/DEPLOY.md,
# deploy/backup.sh, deploy/restore.sh, deploy/holzcloud.service, deploy/test/run.sh,
# deploy/test/Dockerfile, this file and the five plugin archives, together with the
# binary built from the evidence commit (its source, BINARY_SOURCE: cmd, internal,
# go.mod, go.sum, changelog.go and the CHANGELOG.md it embeds). The old release is
# bound by the commit sha of its tag, written into mig.txt (tags never move). `run`
# refuses to start while a bound file or the binary's source differs from HEAD or
# is untracked, or the binary compiles a file outside BINARY_SOURCE, and pins
# NEW_REF to the short sha of HEAD, so the binary and the scripts are one commit.
#
# Stages (STAGES, default "mig upd rb"; a subset is a rehearsal, and a stage left
# out reads NOT-RUN, so a subset can never read all PASS):
#   mig   v0.1.1 (migration level 60) installed by today's Install text, seeded with
#         v0.1.1's own plugin archives, backed up by v0.1.1's own backup.sh and
#         binary; the backup restored with the document's restore line into a
#         second container that runs this tree (level 69); the levels read from
#         the database files, the journal of the first start, the 17 checks of
#         run.sh check on the target (MIG-01, MIG-02)
#   upd   this tree installed and seeded, then six migrating updates on its data
#         directory, each the document's Updating blocks played line by line to a
#         synthetic build of this tree with one more no-op migration than the last
#         (made in a scratch clone, never committed); after each, the snapshot set
#         against the newest five, every removal logged (UPD-01)
#   rb    this tree (B0) installed and seeded, then an update to a synthetic build F
#         whose first added migration succeeds and whose second fails; the restart
#         loop to its end without reset-failed (one snapshot written, kept by every
#         restart, named by every "restore with:" line), the previous binary refused
#         on the half-migrated database by the schema guard (SCENARIO[previous-on-half]),
#         Rolling back block 1 played as written, the state before equal to the
#         state after (UPD-02)
#
# Every line the stages upd and rb play is read out of the fenced bash blocks of
# deploy/DEPLOY.md (doc_block); a line play_kind cannot translate ends the stage.
#
# Environment:
#   PLATFORM    must be linux/amd64 (on this arm64 Mac it runs under emulation)
#   TARGET      must be systemd-base
#   STAGES      see above
#   REHEARSAL   1 lets `run` start while a bound file differs from HEAD; the summary
#               then says REHEARSAL, and such a set is never evidence
#   R           record-check: the record, one of deploy/MIGRATION-RESTORE-RUN.md,
#               deploy/UPDATE-CHAIN-RUN.md or deploy/ROLLBACK-RUN.md; its paste list
#               follows from its name
#   E           appendix and record-check: the scratch root, default deploy/test/out
#
# Names fixed for the phase: containers holzcloud-t34-mig-src, holzcloud-t34-mig-tgt,
# holzcloud-t34-upd and holzcloud-t34-rb; directories deploy/test/out/amd64-t34-mig-src,
# amd64-t34-mig-tgt, amd64-t34-upd and amd64-t34-rb; run files
# deploy/test/out/amd64-t34-evidence-commit.txt, amd64-t34-summary.txt and
# amd64-t34-pass1.{started,finished,status,log}; records deploy/MIGRATION-RESTORE-RUN.md
# (mig), deploy/UPDATE-CHAIN-RUN.md (upd) and deploy/ROLLBACK-RUN.md (rb).
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
STAGES=${STAGES:-mig upd rb}
REHEARSAL=${REHEARSAL:-}

OLD_TAG=v0.1.1
MIG_SRC=holzcloud-t34-mig-src
MIG_TGT=holzcloud-t34-mig-tgt
UPD_NAME=holzcloud-t34-upd
RB_NAME=holzcloud-t34-rb
BASE_OUT=deploy/test/out
MIG_SRC_OUT=$BASE_OUT/amd64-t34-mig-src
MIG_TGT_OUT=$BASE_OUT/amd64-t34-mig-tgt
UPD_OUT=$BASE_OUT/amd64-t34-upd
RB_OUT=$BASE_OUT/amd64-t34-rb
RUN_PREFIX=$BASE_OUT/amd64-t34-pass1
EVIDENCE_FILE=$BASE_OUT/amd64-t34-evidence-commit.txt
SUMMARY_FILE=$BASE_OUT/amd64-t34-summary.txt
# Everything of this module that ./cmd/holzcloud compiles or embeds: the root
# package is changelog.go, which embeds CHANGELOG.md. bound_check refuses a run
# while it differs from HEAD or go list -deps names a file outside this list
# (binary_uncovered). The files are listed for linux/amd64, the target the
# drivers build, and bound_check also refuses a file the binary compiles or
# embeds that no commit holds, a gitignored one included (binary_untracked).
BINARY_SOURCE="cmd internal go.mod go.sum changelog.go CHANGELOG.md"
BOUND_FILES="deploy/DEPLOY.md deploy/backup.sh deploy/restore.sh deploy/holzcloud.service deploy/test/run.sh deploy/test/Dockerfile deploy/test/upgrade-verify.sh plugins/bestellung/bestellung.zip plugins/jahreszahl/jahreszahl.zip plugins/kontaktformular/kontaktformular.zip plugins/nicht-gefunden/nicht-gefunden.zip plugins/suche/suche.zip"
DEPLOY_DOC=deploy/DEPLOY.md
# The document's manual backup line (DEPLOY.md, Backups), played by exact text.
MANUAL_LINE='sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups'
PLUGIN_IDS="bestellung jahreszahl kontaktformular nicht-gefunden suche"
NEW_REF=""
OLD_SHA=""

say() { printf '%s\n' "$*" >&2; }
die() {
  say "upgrade-verify.sh: $*"
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

# safe_name NAME: success for the four containers this file makes, and no other.
safe_name() {
  case "${1:-}" in
    holzcloud-t34-mig-src | holzcloud-t34-mig-tgt | holzcloud-t34-upd | holzcloud-t34-rb) return 0 ;;
  esac
  return 1
}

# safe_out PATH: success for the four directories this file writes and its run
# files, each named exactly; no other path, no path below them, no parent step.
safe_out() {
  case "${1:-}" in
    deploy/test/out/amd64-t34-mig-src | deploy/test/out/amd64-t34-mig-tgt | deploy/test/out/amd64-t34-upd \
      | deploy/test/out/amd64-t34-rb) return 0 ;;
    deploy/test/out/amd64-t34-evidence-commit.txt | deploy/test/out/amd64-t34-summary.txt) return 0 ;;
    deploy/test/out/amd64-t34-pass1.started | deploy/test/out/amd64-t34-pass1.finished \
      | deploy/test/out/amd64-t34-pass1.status | deploy/test/out/amd64-t34-pass1.log) return 0 ;;
  esac
  return 1
}

# stages_valid_of TEXT: TEXT is not empty and names only mig, upd and rb.
# stages_valid: the same for STAGES.
stages_valid_of() {
  local s n=0
  for s in ${1:-}; do
    case "$s" in
      mig | upd | rb) n=$((n + 1)) ;;
      *) return 1 ;;
    esac
  done
  [ "$n" -gt 0 ]
}
stages_valid() { stages_valid_of "$STAGES"; }

# stage_wanted NAME: NAME is one of the words of STAGES.
stage_wanted() {
  case " $STAGES " in
    *" $1 "*) return 0 ;;
  esac
  return 1
}

# is_rehearsal: REHEARSAL is set, or STAGES leaves out one of the three stages.
is_rehearsal() {
  [ -n "$REHEARSAL" ] && return 0
  stage_wanted mig && stage_wanted upd && stage_wanted rb && return 1
  return 0
}

# level_of_output TEXT: the one decimal number sqlite3 printed (a trailing carriage
# return dropped); fails on empty output, text, two lines or a sign.
level_of_output() {
  local t
  t=$(printf '%s' "${1:-}" | tr -d '\r')
  case "$t" in
    "" | *[!0-9]*) return 1 ;;
  esac
  printf '%s\n' "$t"
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
# kv_num FILE KEY: the key occurs exactly once and is a number.
kv_num() {
  [ "$(kv_count "$1" "$2")" = 1 ] && is_count "$(kv "$1" "$2")"
}
# kv_same FILE KEY1 KEY2: both keys occur once, are numbers and are equal.
kv_same() {
  kv_num "$1" "$2" && kv_num "$1" "$3" && [ "$(kv "$1" "$2")" -eq "$(kv "$1" "$3")" ]
}

# kv_login FILE KEY: the key occurs once and is a login the server accepted: a 303
# to /admin/, or to the compulsory two-factor enrolment an administrator without a
# second factor is sent to from 0.1.5 on.
kv_login() {
  [ "$(kv_count "$1" "$2")" = 1 ] || return 1
  case "$(kv "$1" "$2")" in
    "303 /admin/" | "303 /admin/2fa/einrichten") return 0 ;;
  esac
  return 1
}

# applied_versions FILE: the `version` values of the "migration applied" lines of
# a journal of JSON lines, in their order, on one line separated by spaces; empty
# for none, a failure for no file.
applied_versions() {
  [ -f "${1:-}" ] || return 1
  sed -n 's/^{.*"msg":"migration applied".*"version":\([0-9][0-9]*\)[,}].*/\1/p' "$1" | tr '\n' ' ' | sed 's/ $//'
}

# consecutive FROM TO: the numbers FROM+1 to TO on one line separated by spaces.
consecutive() {
  local i=$(($1 + 1)) out=""
  while [ "$i" -le "$2" ]; do
    out="$out $i"
    i=$((i + 1))
  done
  printf '%s\n' "${out# }"
}

# verdict_mig01 FILE: PASS, FAIL or NOT-RUN (no file) for the key lines of mig.txt:
# the restore succeeded, both levels are the ones the migration files of the two
# commits give, the restored installation answers (setup closed, the old login
# accepted, the page, the same media file, 17 of 17 checks) and the first start
# left exactly one snapshot, at the level the backup had.
verdict_mig01() {
  local f=${1:-}
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if kv_is "$f" RESTORE-RC 0 && kv_is "$f" RESTORE-COMPLETE yes \
    && kv_same "$f" LEVEL-BEFORE LEVEL-EXPECTED-BEFORE && kv_same "$f" LEVEL-AFTER LEVEL-EXPECTED-AFTER \
    && kv_is "$f" CHECK "17 of 17" && kv_is "$f" SETUP-AFTER 404 && kv_login "$f" LOGIN-AFTER \
    && kv_is "$f" PAGE-AFTER 200 && kv_is "$f" MEDIA-SHA-EQUAL yes \
    && kv_is "$f" SNAPSHOTS-AFTER 1 && kv_same "$f" SNAPSHOT-LEVEL LEVEL-BEFORE; then
    echo PASS
  else
    echo FAIL
  fi
}

# verdict_mig02 FILE: PASS when mig.txt names both releases with their commits
# (and the backup's MANIFEST the old one), both levels with the later one above,
# what restore.sh printed (the notice about the missing checksum file, the end
# line), one snapshot line of the first start and one "migration applied" line
# for every level between, in order.
verdict_mig02() {
  local f=${1:-} b a
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if [ "$(kv_count "$f" OLD-RELEASE)" = 1 ] && printf '%s\n' "$(kv "$f" OLD-RELEASE)" | grep -Eqx 'v0\.1\.1 \([0-9a-f]{7,40}\)' \
    && [ "$(kv_count "$f" NEW-RELEASE)" = 1 ] && printf '%s\n' "$(kv "$f" NEW-RELEASE)" | grep -Eqx '[^ ]+ \([0-9a-f]{7,40}\)' \
    && [ "$(kv_count "$f" MANIFEST-VERSION)" = 1 ] && printf '%s\n' "$(kv "$f" MANIFEST-VERSION)" | grep -Eq '(^| )v0\.1\.1( |$)' \
    && kv_num "$f" LEVEL-BEFORE && kv_num "$f" LEVEL-AFTER \
    && kv_is "$f" RESTORE-NOTICE yes && kv_is "$f" RESTORE-COMPLETE yes && kv_is "$f" JOURNAL-SNAPSHOT-LINES 1; then
    b=$(kv "$f" LEVEL-BEFORE)
    a=$(kv "$f" LEVEL-AFTER)
    if [ "$a" -gt "$b" ] && kv_is "$f" JOURNAL-APPLIED-LINES "$((a - b))" \
      && kv_is "$f" JOURNAL-APPLIED-VERSIONS "$(consecutive "$b" "$a")"; then
      echo PASS
      return 0
    fi
  fi
  echo FAIL
}

# doc_block FILE SECTION N: the logical lines of the Nth fenced bash block of the
# section whose heading line is exactly "## SECTION" (a ### subsection belongs to
# it; the next # or ## heading ends it). Backslash-continued lines are joined,
# blank lines and lines that begin with # are dropped, as run.sh's doc_awk does.
# Fails when the file, the section or the block is missing, or N is no number
# above zero. The stages read every line they play through this function: a
# command of the Updating or Rolling back blocks is never typed into this file.
doc_block() {
  local f=${1:-} s=${2:-} n=${3:-} out rc=0
  [ -f "$f" ] || return 1
  is_count "$n" || return 1
  [ "$n" -gt 0 ] || return 1
  out=$(S="$s" N="$n" awk '
    BEGIN { s = ENVIRON["S"]; want = ENVIRON["N"] + 0; insec = 0; found = 0; nb = 0; inf = 0; take = 0; hascont = 0; cont = "" }
    {
      line = $0
      sub(/\r$/, "", line)
      if (inf) {
        if (line ~ /^[ \t]*```[ \t]*$/) {
          if (hascont) { print cont; hascont = 0; cont = "" }
          inf = 0
          next
        }
        if (!take) next
        t = line
        sub(/^[ \t]+/, "", t)
        if (hascont) {
          t = cont " " t; hascont = 0; cont = ""
        } else if (t == "" || t ~ /^#/) {
          next
        }
        if (t ~ /\\$/) { sub(/[ \t]*\\$/, "", t); cont = t; hascont = 1; next }
        sub(/[ \t]+$/, "", t)
        if (t == "") next
        print t
        next
      }
      if (line ~ /^[ \t]*```/) {
        inf = 1
        lang = line
        sub(/^[ \t]*`+[ \t]*/, "", lang)
        sub(/[ \t].*$/, "", lang)
        take = 0
        if (insec && lang == "bash") { nb++; take = (nb == want) }
        next
      }
      if (line ~ /^# / || line ~ /^## /) {
        insec = 0
        if (!found && line == "## " s) { insec = 1; found = 1 }
        next
      }
    }
    END { if (!found) exit 3; if (nb < want) exit 4 }' "$f") || rc=$?
  [ "$rc" -eq 0 ] || return "$rc"
  [ -n "$out" ] || return 4
  printf '%s\n' "$out"
}

# play_kind LINE: how a document line is played in the container, mirroring
# run.sh's doc_cmd: scp-binary (`scp holzcloud user@host:[dir]`, played as docker
# cp of the binary under test), scp-files (`scp deploy/a deploy/b user@host:[dir]`,
# single files directly in deploy/, played as docker cp of the working tree's
# files), build (a `go build` line, skipped: the binary was built by that same
# line before), run (every other line, as root in the container). Fails for an
# scp line of any other shape and for a line with an angle-bracket placeholder:
# a line this file cannot translate ends the stage.
play_kind() {
  local l=${1:-} last
  [ -n "$l" ] || return 1
  if printf '%s\n' "$l" | grep -Eq '<[A-Za-z][A-Za-z0-9_.-]*>'; then
    return 1
  fi
  case "$l" in
    "scp "*)
      set -f
      # shellcheck disable=SC2086
      set -- $l
      set +f
      for last in "$@"; do :; done
      if [ "$#" -eq 3 ] && [ "$2" = holzcloud ] && [ "${3#*:}" != "$3" ]; then
        echo scp-binary
        return 0
      fi
      if [ "$#" -ge 3 ] && [ "${last#*:}" != "$last" ]; then
        shift
        while [ "$#" -gt 1 ]; do
          case "$1" in
            deploy/*/* | deploy/) return 1 ;;
            deploy/?*) ;;
            *) return 1 ;;
          esac
          shift
        done
        echo scp-files
        return 0
      fi
      return 1
      ;;
    *"go build"*) echo build ;;
    *) echo run ;;
  esac
}

# restore_with_paths FILE: the distinct snapshot paths that the "restore with:"
# lines of a journal of JSON lines name, in the order they first appear.
restore_with_paths() {
  [ -f "${1:-}" ] || return 1
  sed -n 's/^{.*"msg":"restore with: [^"]* cp \([^ "]*\) [^"]*".*/\1/p' "$1" | awk '!seen[$0]++'
}

# json_num LINE KEY: the number of "KEY":N in one JSON line; empty when absent or
# not a number. json_str LINE KEY: the string "KEY":"..." (no escaped quote inside).
# json_err LINE: the value of "err", the last key of the server's error lines,
# with its escapes as they stand.
json_num() { printf '%s\n' "${1:-}" | sed -n "s/.*\"$2\":\([0-9][0-9]*\)[,}].*/\1/p" | head -n 1; }
json_str() { printf '%s\n' "${1:-}" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p" | head -n 1; }
json_err() { printf '%s\n' "${1:-}" | sed -n 's/.*"err":"\(.*\)"}$/\1/p' | head -n 1; }

# json_escape TEXT: TEXT as it stands inside a JSON string (backslash, quote).
json_escape() { printf '%s\n' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

# guard_sentence GOFILE DB BIN: the sentence NewerSchemaError.Error returns, taken
# from the string pieces of its fmt.Sprintf in GOFILE (internal/db/schema_guard.go),
# with the two %d replaced by DB and BIN. Never a typed copy of the sentence.
guard_sentence() {
  local g=${1:-} s
  [ -f "$g" ] || return 1
  s=$(awk '
    /return fmt\.Sprintf\(/ { on = 1 }
    on {
      line = $0
      while (match(line, /"([^"\\]|\\.)*"/)) {
        out = out substr(line, RSTART + 1, RLENGTH - 2)
        line = substr(line, RSTART + RLENGTH)
      }
    }
    on && /e\.DBLevel/ { print out; done = 1; exit }
    END { if (!done) exit 1 }' "$g") || return 1
  [ -n "$s" ] || return 1
  s=$(printf '%s\n' "$s" | sed 's/\\"/"/g')
  printf '%s\n' "$s" | sed "s/%d/$2/; s/%d/$3/"
}

# compare_state S0 S2: the two state captures line by line; one line "BAD s0 ... |
# s2 ..." per difference (a line missing on one side included), then "bad=N".
# Two empty captures are no comparison and count as one difference.
compare_state() {
  awk '
    NR == FNR { a[FNR] = $0; na = FNR; next }
    { b[FNR] = $0; nb = FNR }
    END {
      n = (na > nb) ? na : nb
      bad = 0
      if (n == 0) { print "BAD no state on either side"; bad = 1 }
      for (i = 1; i <= n; i++) {
        if (!(i in a) || !(i in b) || a[i] != b[i]) {
          printf "BAD s0 %s | s2 %s\n", ((i in a) ? a[i] : "(none)"), ((i in b) ? b[i] : "(none)")
          bad++
        }
      }
      print "bad=" bad
    }' "$1" "$2"
}

# kv_level FILE KEY WANT: the key occurs once, is a number and equals WANT.
kv_level() {
  kv_num "$1" "$2" && [ "$(kv "$1" "$2")" -eq "$3" ]
}

# verdict_upd02 FILE: PASS, FAIL or NOT-RUN (no file) for the key lines of rb.txt.
# The levels come from the migration counts the stage wrote (LEVEL-EXPECTED-B0 of
# this tree, LEVEL-EXPECTED-F, two more), never typed: the update stopped with a
# message naming F's last migration, after it had applied the one before; one
# snapshot was written and every later start of the loop kept it (KEPT = STARTS
# - 1), every start named that one snapshot in its "restore with:" line; the
# database stood one level above B0; the previous binary refused it with the
# guard's sentence and both levels; Rolling back block 1 ran without a failing
# line, the old version answers ok with integrity ok, the last line found nothing
# to migrate, the database is at B0's level without F's table, the set-aside copy
# at the half level, no marker left, and the state equals the one before. The
# loop ran to its end (ACTIVE-STATE failed, systemd's start-limit line) without
# /healthz ever answering, the snapshots after it hold the written one at B0's
# level and none at the half level, and the previous binary's starts on the
# half-migrated database ended failed without /healthz answering either.
verdict_upd02() {
  local f=${1:-} b0 fl st rcs r rz bv
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  if kv_num "$f" LEVEL-EXPECTED-B0 && kv_num "$f" LEVEL-EXPECTED-F && kv_num "$f" STARTS \
    && [ "$(kv_count "$f" B0-VERSION)" = 1 ] && [ -n "$(kv "$f" B0-VERSION)" ]; then
    b0=$(kv "$f" LEVEL-EXPECTED-B0)
    fl=$(kv "$f" LEVEL-EXPECTED-F)
    st=$(kv "$f" STARTS)
    bv=$(kv "$f" B0-VERSION)
    rcs=$(kv "$f" ROLLBACK-RC)
    rz=$(kv "$f" ROLLBACK-READYZ)
    if [ "$fl" -eq "$((b0 + 2))" ] && [ "$st" -ge 2 ] \
      && kv_num "$f" UPDATE-CHECK-RC && [ "$(kv "$f" UPDATE-CHECK-RC)" -ne 0 ] \
      && kv_is "$f" UPDATE-CHECK-SNAPSHOT-LINE yes \
      && kv_is "$f" JOURNAL-SNAPSHOT-WRITTEN 1 && kv_level "$f" JOURNAL-SNAPSHOT-KEPT "$((st - 1))" \
      && kv_level "$f" JOURNAL-MIGRATIONS-FAILED "$st" && kv_level "$f" JOURNAL-RESTORE-WITH "$st" \
      && kv_is "$f" RESTORE-WITH-DISTINCT-PATHS 1 && kv_is "$f" RESTORE-WITH-NAMES-WRITTEN yes \
      && [ "$(kv_count "$f" STOP-MESSAGE)" = 1 ] \
      && case "$(kv "$f" STOP-MESSAGE)" in *"partial migration error ("*"version:$fl)"*) true ;; *) false ;; esac \
      && kv_is "$f" APPLIED-BEFORE-FAILURE "$((b0 + 1))" \
      && kv_level "$f" LEVEL-HALF "$((b0 + 1))" \
      && kv_is "$f" GUARD-LINE yes && kv_level "$f" GUARD-DB-LEVEL "$((b0 + 1))" \
      && kv_level "$f" GUARD-BINARY-LEVEL "$b0" && kv_is "$f" GUARD-SENTENCE yes \
      && [ "$(kv_count "$f" ROLLBACK-RC)" = 1 ] && [ -n "$rcs" ] \
      && [ "$(kv_count "$f" ROLLBACK-READYZ)" = 1 ] \
      && case "$rz" in *'"status":"ok"'*) true ;; *) false ;; esac \
      && case "$rz" in *"\"version\":\"$bv\""*) true ;; *) false ;; esac \
      && case "$rz" in *'"integrity":"ok"'*) true ;; *) false ;; esac \
      && kv_is "$f" ROLLBACK-CHECK-LINE "no migration, no snapshot" \
      && kv_level "$f" LEVEL-AFTER-ROLLBACK "$b0" && kv_is "$f" T34-OK-TABLE absent \
      && kv_level "$f" AFTER-UPDATE-LEVEL "$((b0 + 1))" && kv_is "$f" MARKER-AFTER absent \
      && kv_is "$f" COMPARE-S0-S2 "bad=0" \
      && kv_is "$f" ACTIVE-STATE failed && kv_is "$f" START-LIMIT-LINE yes \
      && kv_is "$f" HEALTHZ-DURING-LOOP never \
      && kv_is "$f" GUARD-HEALTHZ never && kv_is "$f" GUARD-ACTIVE-STATE failed \
      && kv_level "$f" WRITTEN-LEVEL "$b0" && loop_snapshots_ok "$f" "$b0"; then
      for r in $rcs; do
        [ "$r" = 0 ] || {
          echo FAIL
          return 0
        }
      done
      echo PASS
      return 0
    fi
  fi
  echo FAIL
}

# loop_snapshots_ok FILE B0: SNAPSHOTS-AFTER-LOOP (NAME:LEVEL words) holds
# exactly one snapshot at B0's level, the one WRITTEN-PATH names, and none at the
# half level B0 + 1: the restart loop wrote no snapshot of the half-migrated
# database. Levels are compared as whole numbers, so 9 never matches 69.
loop_snapshots_ok() {
  local f=$1 b0=$2 w wp at0=0 at1=0 named=0
  [ "$(kv_count "$f" SNAPSHOTS-AFTER-LOOP)" = 1 ] && [ "$(kv_count "$f" WRITTEN-PATH)" = 1 ] || return 1
  wp=$(kv "$f" WRITTEN-PATH)
  [ -n "$wp" ] || return 1
  for w in $(kv "$f" SNAPSHOTS-AFTER-LOOP); do
    if [ "${w##*:}" = "$b0" ]; then
      at0=$((at0 + 1))
      [ "${w%:*}" != "${wp##*/}" ] || named=1
    elif [ "${w##*:}" = "$((b0 + 1))" ]; then
      at1=$((at1 + 1))
    fi
  done
  [ "$at0" -eq 1 ] && [ "$named" -eq 1 ] && [ "$at1" -eq 0 ]
}

# snap_stamp NAME: the UTC stamp at the end of a snapshot's name
# (pre-upgrade-VERSION-YYYYMMDD-HHMMSS.sqlite); 00000000-000000 for a name without
# one, which sorts it as the oldest, as internal/db PruneSnapshots does.
snap_stamp() {
  case "${1:-}" in
    *-[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9].sqlite)
      printf '%s\n' "$1" | sed 's/.*-\([0-9]\{8\}-[0-9]\{6\}\)\.sqlite$/\1/'
      ;;
    *) echo 00000000-000000 ;;
  esac
}

# by_stamp NAMES: the names oldest first by their stamp, then by name (the order
# PruneSnapshots sorts in: by time, never by the version string), one line,
# separated by spaces.
by_stamp() {
  local s
  for s in ${1:-}; do
    printf '%s %s\n' "$(snap_stamp "$s")" "$s"
  done | LC_ALL=C sort -k1,1 -k2,2 | awk '{ printf "%s%s", (NR > 1 ? " " : ""), $2 } END { print "" }'
}

# prune_expect INITIAL WRITTEN K: the snapshots the data directory holds after the
# Kth update, when it held INITIAL before the first and the updates wrote WRITTEN
# (in order): the newest five of INITIAL and the first K of WRITTEN by stamp,
# oldest first, on one line. Fails when WRITTEN names fewer than K, or K is no
# number above zero.
prune_expect() {
  local init=${1:-} written=${2:-} k=${3:-} all w i=0
  is_count "$k" || return 1
  [ "$k" -gt 0 ] || return 1
  all=$init
  for w in $written; do
    [ "$i" -lt "$k" ] || break
    i=$((i + 1))
    all="$all $w"
  done
  [ "$i" -eq "$k" ] || return 1
  by_stamp "$all" | tr ' ' '\n' | grep . | tail -n 5 | tr '\n' ' ' | sed 's/ $//'
  echo
}

# removed_between BEFORE AFTER: the names of BEFORE that AFTER no longer holds, in
# the order of BEFORE, on one line.
removed_between() {
  local s out=""
  for s in ${1:-}; do
    case " ${2:-} " in
      *" $s "*) ;;
      *) out="$out $s" ;;
    esac
  done
  printf '%s\n' "${out# }"
}

# verdict_upd01 FILE: PASS, FAIL or NOT-RUN (no file) for the key lines of upd.txt:
# six updates, each with a written snapshot, each served by its own build
# (READYZ-k is the version of BUILD-k), each at the level the migration count
# gives (LEVEL-EXPECTED-B0 plus k), each leaving exactly the newest five
# (MATCH-k yes); at least one snapshot removed, and every removal logged.
verdict_upd01() {
  local f=${1:-} b0 k
  if [ ! -f "$f" ]; then
    echo NOT-RUN
    return 0
  fi
  kv_num "$f" LEVEL-EXPECTED-B0 || {
    echo FAIL
    return 0
  }
  b0=$(kv "$f" LEVEL-EXPECTED-B0)
  for k in 1 2 3 4 5 6; do
    if ! kv_is "$f" "MATCH-$k" yes || [ "$(kv_count "$f" "BUILD-$k")" != 1 ] || [ "$(kv_count "$f" "READYZ-$k")" != 1 ] \
      || [ -z "$(kv "$f" "READYZ-$k")" ] || [ "$(kv "$f" "READYZ-$k")" != "$(kv "$f" "BUILD-$k" | awk '{ print $1 }')" ] \
      || [ "$(kv_count "$f" "WRITTEN-$k")" != 1 ] \
      || ! case "$(kv "$f" "WRITTEN-$k")" in pre-upgrade-*.sqlite) true ;; *) false ;; esac \
      || ! kv_level "$f" "LEVEL-AFTER-$k" "$((b0 + k))"; then
      echo FAIL
      return 0
    fi
  done
  if kv_num "$f" REMOVALS-TOTAL && [ "$(kv "$f" REMOVALS-TOTAL)" -ge 1 ] && kv_same "$f" REMOVALS-LOGGED REMOVALS-TOTAL; then
    echo PASS
  else
    echo FAIL
  fi
}

# ---------------------------------------------------------------------------
# The containers. Every docker call names a container that safe_name has
# accepted, and every run.sh call a directory that safe_out has accepted.
# ---------------------------------------------------------------------------

# dxc NAME CMD...: docker exec in one of the four containers.
dxc() {
  local n=$1
  shift
  safe_name "$n" || die "refusing container name $n"
  docker exec "$n" "$@"
}

# run_sh NAME OUT SUBCOMMAND...: run.sh through its public subcommands, on that
# container and that directory only, the new binary of NEW_REF. run_sh_old: the
# same with INSTALL_FROM=old and OLD_TAG (run.sh builds the tag from a clone with
# today's Build line and plays today's Install text).
run_sh() {
  local n=$1 o=$2
  shift 2
  safe_name "$n" || die "refusing container name $n"
  safe_out "$o" || die "refusing path $o"
  env PLATFORM="$PLATFORM" TARGET=systemd-base NEW_REF="$NEW_REF" NAME="$n" OUT_DIR="$o" KEEP=1 \
    INSTALL_FROM=new bash "$ROOT/deploy/test/run.sh" "$@"
}
run_sh_old() {
  local n=$1 o=$2
  shift 2
  safe_name "$n" || die "refusing container name $n"
  safe_out "$o" || die "refusing path $o"
  env PLATFORM="$PLATFORM" TARGET=systemd-base NEW_REF="$NEW_REF" NAME="$n" OUT_DIR="$o" KEEP=1 \
    INSTALL_FROM=old OLD_TAG="$OLD_TAG" bash "$ROOT/deploy/test/run.sh" "$@"
}

# run_sh_transfer SRC SRC_OUT TGT TGT_OUT DIR: run.sh transfer of the backup
# directory DIR of SRC into TGT (with the three fixtures run.sh carries along).
run_sh_transfer() {
  local n
  for n in "$1" "$3"; do safe_name "$n" || die "refusing container name $n"; done
  for n in "$2" "$4"; do safe_out "$n" || die "refusing path $n"; done
  env PLATFORM="$PLATFORM" TARGET=systemd-base NEW_REF="$NEW_REF" NAME="$1" OUT_DIR="$2" \
    TGT_NAME="$3" TGT_OUT_DIR="$4" BACKUP_DIR_PATH="$5" KEEP=1 bash "$ROOT/deploy/test/run.sh" transfer
}

# DEV_FILE: where deviation() writes; set by each stage to its own directory.
DEV_FILE=""
# deviation TEXT: one line of the driver's deviations of the current stage.
deviation() {
  say "$*"
  [ -n "$DEV_FILE" ] || die "deviation before a stage directory exists: $*"
  printf '%s\n' "$*" >> "$DEV_FILE"
}

# put FILE KEY VALUE: one key line, also on stderr for the log.
put() {
  printf '%s: %s\n' "$2" "$3" >> "$1"
  say "$2: $3"
}

# bound_check: the bound files are tracked and equal to HEAD. Prints one line per
# problem and fails when there is one.
bound_check() {
  local f bad=0 unc unt
  for f in $BOUND_FILES; do
    if ! git ls-files --error-unmatch -- "$f" > /dev/null 2>&1; then
      say "upgrade-verify.sh: bound file $f is not tracked"
      bad=1
    elif ! git diff --quiet HEAD -- "$f"; then
      say "upgrade-verify.sh: bound file $f differs from HEAD"
      bad=1
    fi
  done
  if ! git diff --quiet HEAD -- $BINARY_SOURCE || [ -n "$(git ls-files --others --exclude-standard -- $BINARY_SOURCE)" ]; then
    say "upgrade-verify.sh: the binary's source ($BINARY_SOURCE) differs from HEAD"
    bad=1
  fi
  if ! unc=$(binary_uncovered); then
    say "upgrade-verify.sh: go list -deps ./cmd/holzcloud failed, so the binary's source cannot be bound"
    bad=1
  else
    if [ -n "$unc" ]; then
      say "upgrade-verify.sh: the binary compiles or embeds files BINARY_SOURCE does not bind: $(printf '%s\n' "$unc" | tr '\n' ' ')"
      bad=1
    fi
    if ! unt=$(binary_untracked); then
      say "upgrade-verify.sh: go list -deps ./cmd/holzcloud failed, so the binary's source cannot be bound"
      bad=1
    elif [ -n "$unt" ]; then
      say "upgrade-verify.sh: the binary compiles or embeds files that no commit holds (untracked or gitignored): $(printf '%s\n' "$unt" | tr '\n' ' ')"
      bad=1
    fi
  fi
  [ "$bad" -eq 0 ]
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

# unstamped FILE: the lines of a stamped file without the clock.
unstamped() { sed 's/^[0-9][0-9.]* //' "$1"; }

# restore_line DIR: the document's restore line with DIR in place of its example
# (DEVIATION[backup-name]).
restore_line() {
  local l
  l=$(grep -E '^sudo /opt/holzcloud/deploy/restore\.sh /opt/holzcloud/backups/[0-9]{8}-[0-9]{6}$' "$DEPLOY_DOC" | head -n 1) || l=""
  [ -n "$l" ] || die "the document and this file have drifted apart: $DEPLOY_DOC has no restore line of the shape 'sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/TIMESTAMP'"
  printf '%s %s\n' "${l% *}" "$1"
}

# LEVEL_SQL: the highest applied goose version, the one definition of a level.
LEVEL_SQL="select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1"

# level_file NAME PATH: the level of a database file that no process writes (a
# backup, a snapshot), read through the immutable URI as the service user, so
# that no -wal or -shm file appears beside it. Never a holzcloud subcommand: every
# one of them migrates, and from 0.1.5 snapshots, before it answers. "?" when the
# answer is not one number.
level_file() {
  local out
  out=$(dxc "$1" runuser -u holzcloud -- sqlite3 "file:$2?immutable=1" "$LEVEL_SQL" 2> /dev/null) || out=""
  level_of_output "$out" || echo "?"
}

# level_live NAME: the level of the live database of the running service. The
# immutable URI would ignore the -wal file, which holds what the service wrote
# since its last checkpoint (the migrations of the first start among it), so the
# live file is read through a read-only connection that sees the log; still
# sqlite3, never a holzcloud subcommand.
level_live() {
  local out
  out=$(dxc "$1" runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite ".timeout 5000" "$LEVEL_SQL" 2> /dev/null) || out=""
  level_of_output "$out" || echo "?"
}

# Runs INSIDE the container (docker exec -i NAME bash -s -- ARGS...); the shell
# text is on stdin. run.sh's First Run technique, this file's own copy (run.sh
# keeps its FORM_SH to itself):
#   get   PATH JAR                  GET with the cookie jar
#   login PATH EMAIL PASSFILE JAR   GET the form into JAR, take the hidden CSRF
#                                   field, POST email and password
# The jar is created under umask 077. The password is read from PASSFILE, a
# root-only file in the container, and neither it nor the token nor a cookie is
# ever printed: the only output is one line "HTTP STATUS REDIRECT-PATH". The unit
# sets HOLZCLOUD_SECURE=true, so a form over plain http is only accepted with a
# Referer that carries the https origin (DEVIATION[browser]).
read -r -d '' FORM_SH << 'FORM_EOF' || true
set -eu
umask 077
base=http://127.0.0.1:8080
mode=$1
path=$2
case "$mode" in
  get)
    jar=$3
    res=$(curl -s -o /dev/null --max-time 60 -w '%{http_code} %{redirect_url}' -c "$jar" -b "$jar" "$base$path")
    ;;
  login)
    email=$3
    passfile=$4
    jar=$5
    page=$(curl -s --max-time 60 -c "$jar" -b "$jar" "$base$path")
    tok=$(printf '%s' "$page" | grep -o 'name="gorilla.csrf.Token" value="[^"]*"' | head -n 1 \
      | sed 's/.*value="//; s/"$//; s/&#43;/+/g; s/&#61;/=/g; s/&#47;/\//g')
    if [ -z "$tok" ]; then
      echo "HTTP none no-csrf-field"
      exit 0
    fi
    pass=$(cat "$passfile")
    res=$(curl -s -o /dev/null --max-time 120 -w '%{http_code} %{redirect_url}' -c "$jar" -b "$jar" \
      -H "Referer: https://127.0.0.1:8080$path" \
      --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "email=$email" \
      --data-urlencode "password=$pass" "$base$path")
    ;;
  *)
    echo "HTTP none bad-mode"
    exit 0
    ;;
esac
code=${res%% *}
loc=${res#* }
loc=${loc#$base}
echo "HTTP $code ${loc:--}"
FORM_EOF

# form_in NAME ARGS...: FORM_SH in that container; prints "HTTP STATUS REDIRECT-PATH".
form_in() {
  local n=$1
  shift
  safe_name "$n" || die "refusing container name $n"
  printf '%s\n' "$FORM_SH" | docker exec -i "$n" bash -s -- "$@"
}

# http_status NAME PATH: the status of PATH for test.local in that container.
http_status() {
  dxc "$1" curl -s -o /dev/null -w '%{http_code}' -H 'Host: test.local' "http://127.0.0.1:8080$2" | tr -d '\r'
}

# media_row NAME: website/file of the first media row (the upload run.sh seed
# made), through a read-only connection to the live database.
media_row() {
  dxc "$1" runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite ".timeout 5000" \
    "select website_id || '/' || filename from media order by id limit 1" | tr -d '\r'
}

# media_get NAME ROW: "HTTP STATUS SHA256" of /media/ROW for test.local, the body
# hashed inside the container.
media_get() {
  dxc "$1" bash -c 'f=$(mktemp); c=$(curl -s -o "$f" -w "%{http_code}" -H "Host: test.local" "http://127.0.0.1:8080/media/$1"); echo "HTTP $c $(sha256sum < "$f" | cut -d" " -f1)"; rm -f "$f"' _ "$2" | tr -d '\r'
}

# migrations_at REF: the number of migration files at REF (git ls-tree), the level
# a binary of REF migrates to. Never a typed number.
migrations_at() {
  git ls-tree --name-only "$1" internal/db/migrations/ | grep -c '\.sql$' || true
}

# ---------------------------------------------------------------------------
# Stage mig: a v0.1.1 backup, made by its own script and binary, restored into
# this tree across the migrations above its level.
# ---------------------------------------------------------------------------

# mig_source: v0.1.1 installed by today's Install text, seeded with its own plugin
# archives, a login user made by its own CLI.
mig_source() {
  local f=$1 id zip sha ver
  # `up` resets the directory, so the stage's own files begin after it.
  run_sh_old "$MIG_SRC" "$MIG_SRC_OUT" build up install
  : > "$f"
  DEV_FILE=$MIG_SRC_OUT/deviations-driver.lst
  : > "$DEV_FILE"
  ver=$(dxc "$MIG_SRC" /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r') || ver=""
  case "$ver" in
    *"$OLD_TAG"*) ;;
    *) die "mig: the source runs '$ver', not $OLD_TAG" ;;
  esac
  put "$f" OLD-RELEASE "$OLD_TAG ($(git rev-parse --short "$OLD_SHA"))"
  put "$f" OLD-COMMIT "$OLD_SHA"
  put "$f" OLD-VERSION-LINE "$ver"
  put "$f" NEW-RELEASE "$(git describe --tags --always "$NEW_REF") ($NEW_REF)"

  # v0.1.1's own plugin archives (F-4): its manifest check knows no `privacy`
  # hook, which today's kontaktformular and bestellung declare.
  dxc "$MIG_SRC" mkdir -p /opt/holzcloud/plugin-zips || die "mig: could not create /opt/holzcloud/plugin-zips"
  for id in $PLUGIN_IDS; do
    zip=$MIG_SRC_OUT/v011-$id.zip
    git show "$OLD_SHA:plugins/$id/$id.zip" > "$zip" || die "mig: git show $OLD_TAG:plugins/$id/$id.zip failed"
    [ -s "$zip" ] || die "mig: $OLD_TAG has an empty plugins/$id/$id.zip"
    sha=$(shasum -a 256 "$zip" | awk '{ print $1 }')
    docker cp "$zip" "$MIG_SRC:/opt/holzcloud/plugin-zips/$id.zip" > /dev/null || die "mig: docker cp of $zip failed"
    put "$f" OLD-PLUGIN "$id sha256 $sha"
  done
  deviation "DEVIATION[old-plugins]: the v0.1.1 source is seeded with the plugin archives of v0.1.1 itself (git show v0.1.1:plugins/ID/ID.zip), not with this tree's: v0.1.1 refuses today's kontaktformular and bestellung archives, whose manifests declare the hook 'privacy' that its manifest check does not know (finding F-4). After the restore the host of this tree runs those v0.1.1 modules"
  deviation "DEVIATION[fixture]: the five plugin archives are copied into /opt/holzcloud/plugin-zips before run.sh seed, and the login user is made with the CLI of v0.1.1 (password in a root-only file, given on stdin); with the website, page, AI key and test image run.sh seed makes, they stand in for operator work in the admin and are no step of the document. No order row is written (ORDER: none)"
  local rc=0
  run_sh "$MIG_SRC" "$MIG_SRC_OUT" seed || rc=$?
  put "$f" SEED-RC "$rc"
  [ "$rc" -eq 0 ] || die "mig: run.sh seed failed on the $OLD_TAG source"

  dxc "$MIG_SRC" bash -c 'umask 077; openssl rand -hex 12 > /root/.hc-login-pass; test -s /root/.hc-login-pass' \
    || die "mig: could not create the login password file"
  dxc "$MIG_SRC" bash -c 'runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud user create -email login@example.test -role admin < /root/.hc-login-pass' \
    > /dev/null || die "mig: holzcloud user create failed on the $OLD_TAG binary"
  {
    echo "OLD_TAG: $OLD_TAG"
    echo "ORDER: none (the stage writes no order row; run.sh transfer reads this line)"
  } > "$MIG_SRC_OUT/baseline.txt"

  # What the restored installation has to answer the same way afterwards.
  deviation "DEVIATION[browser]: the login before the backup and after the restore is played with curl, not in a browser; the shipped unit sets HOLZCLOUD_SECURE=true, so the post over plain http carries a Referer with the https origin, and the hidden CSRF field is read out of the page"
  put "$f" LOGIN-BEFORE "$(form_in "$MIG_SRC" login /admin/login login@example.test /root/.hc-login-pass /root/.hc-jar-t34 | sed 's/^HTTP //')"
  dxc "$MIG_SRC" rm -f /root/.hc-jar-t34 || true
  put "$f" PAGE-BEFORE "$(http_status "$MIG_SRC" /plugintest)"
  MEDIA_ROW=$(media_row "$MIG_SRC") || MEDIA_ROW=""
  [ -n "$MEDIA_ROW" ] || die "mig: the $OLD_TAG source has no media row after run.sh seed"
  put "$f" MEDIA-PATH "/media/$MEDIA_ROW"
  MEDIA_BEFORE=$(media_get "$MIG_SRC" "$MEDIA_ROW") || MEDIA_BEFORE=""
  put "$f" MEDIA-BEFORE "$MEDIA_BEFORE"
}

# mig_backup: v0.1.1's backup.sh in place of today's, the document's manual line
# played with v0.1.1's binary; MIG_DIR is the directory it names.
mig_backup() {
  local f=$1 sha rc=0
  git show "$OLD_SHA:deploy/backup.sh" > "$MIG_SRC_OUT/v011-backup.sh" || die "mig: git show $OLD_TAG:deploy/backup.sh failed"
  docker cp "$MIG_SRC_OUT/v011-backup.sh" "$MIG_SRC:/opt/holzcloud/deploy/backup.sh" > /dev/null \
    || die "mig: docker cp of the $OLD_TAG backup.sh failed"
  dxc "$MIG_SRC" bash -c 'chown root:root /opt/holzcloud/deploy/backup.sh && chmod 755 /opt/holzcloud/deploy/backup.sh' \
    || die "mig: could not set the owner of the $OLD_TAG backup.sh"
  sha=$(dxc "$MIG_SRC" sha256sum /opt/holzcloud/deploy/backup.sh | cut -d' ' -f1 | tr -d '\r')
  put "$f" BACKUP-SCRIPT "$OLD_TAG sha256 $sha"
  deviation "DEVIATION[old-script]: /opt/holzcloud/deploy/backup.sh is replaced by the backup.sh of v0.1.1 (git show v0.1.1:deploy/backup.sh, root 755) before the manual line is played, because today's Install text copied this tree's; a server still on v0.1.1 has that script there from its own install"
  grep -Fxq -- "$MANUAL_LINE" "$DEPLOY_DOC" || die "the document and this file have drifted apart: $DEPLOY_DOC has no line '$MANUAL_LINE'"
  say "mig: $MANUAL_LINE"
  docker exec "$MIG_SRC" bash -c "$STAMP_SH" _ "$MANUAL_LINE" < /dev/null > "$MIG_SRC_OUT/mig-backup.out" 2>&1 || rc=$?
  put "$f" BACKUP-RC "$rc"
  MIG_DIR=$(unstamped "$MIG_SRC_OUT/mig-backup.out" | sed -n 's/^=== Backup complete: \(.*\) ===$/\1/p' | tail -n 1)
  case "$MIG_DIR" in
    /opt/holzcloud/backups/[0-9]*-[0-9]*) ;;
    *) die "mig: the manual backup named no directory ('$MIG_DIR')" ;;
  esac
  put "$f" BACKUP-DIR "$MIG_DIR"
  put "$f" MANIFEST-VERSION "$(dxc "$MIG_SRC" sed -n 's/^version=//p' "$MIG_DIR/MANIFEST" | head -n 1 | tr -d '\r')"
  if dxc "$MIG_SRC" test -e "$MIG_DIR/SHA256SUMS"; then
    put "$f" SUMS-IN-BACKUP yes
  else
    put "$f" SUMS-IN-BACKUP no
  fi
  put "$f" LEVEL-BEFORE "$(level_file "$MIG_SRC" "$MIG_DIR/holzcloud.sqlite")"
  put "$f" LEVEL-EXPECTED-BEFORE "$(migrations_at "$OLD_SHA")"
}

# mig_restore: a fresh target with this tree, the backup carried over, the
# document's restore line played on it.
mig_restore() {
  local f=$1 rc=0 line base
  run_sh "$MIG_TGT" "$MIG_TGT_OUT" build up install
  run_sh_transfer "$MIG_SRC" "$MIG_SRC_OUT" "$MIG_TGT" "$MIG_TGT_OUT" "$MIG_DIR" || rc=$?
  put "$f" TRANSFER-RC "$rc"
  [ "$rc" -eq 0 ] || die "mig: run.sh transfer failed"
  base=$(basename "$MIG_DIR")
  line=$(restore_line "/opt/holzcloud/backups/$base")
  deviation "DEVIATION[backup-name]: the sample timestamp of the document's restore line is replaced by the transferred directory /opt/holzcloud/backups/$base; the answer 'yes' is typed on stdin"
  deviation "DEVIATION[start-limit]: systemctl reset-failed holzcloud runs before the restore and before run.sh check; the install start, the restore's start and check's restart come closer together than on a server"
  dxc "$MIG_TGT" systemctl reset-failed holzcloud > /dev/null 2>&1 || true
  say "mig: $line"
  rc=0
  printf 'yes\n' | docker exec -i "$MIG_TGT" bash -c "$STAMP_SH" _ "$line" > "$MIG_SRC_OUT/mig-restore.out" 2>&1 || rc=$?
  put "$f" RESTORE-RC "$rc"
  if unstamped "$MIG_SRC_OUT/mig-restore.out" | grep -q '^NOTICE: .* has no checksum file (made before 0\.1\.5); its files were not verified$'; then
    put "$f" RESTORE-NOTICE yes
  else
    put "$f" RESTORE-NOTICE no
  fi
  if unstamped "$MIG_SRC_OUT/mig-restore.out" | grep -qx '=== Restore complete ==='; then
    put "$f" RESTORE-COMPLETE yes
  else
    put "$f" RESTORE-COMPLETE no
  fi
  put "$f" READY-SECONDS "$(sed -n 's/^READY_SECONDS="\${READY_SECONDS:-\([0-9]*\)}"$/\1/p' deploy/restore.sh | head -n 1)"
  put "$f" FIRST-START-S "$(awk '/ Starting holzcloud\.\.\.$/ { s = $1 } / === Restore complete ===$/ { c = $1 } END { if (s != "" && c != "") printf "%.1f\n", c - s; else print "?" }' "$MIG_SRC_OUT/mig-restore.out")"
}

# mig_after: what the first start after the restore wrote and how the restored
# installation answers, on the target; then run.sh check.
mig_after() {
  local f=$1 t0 snaps s n lg rc=0 pass bad
  # The journal since the restore began (its first stamp, the container's clock):
  # the install's own first start, with its 69 migrations, lies before it.
  t0=$(awk '/^WALL / { print $2 }' "$MIG_SRC_OUT/mig-restore.out" | head -n 1)
  t0=${t0%%.*}
  is_count "$t0" || die "mig: mig-restore.out has no WALL line"
  dxc "$MIG_TGT" journalctl -u holzcloud --since "@$t0" -o cat --no-pager | tr -d '\r' \
    | grep -E '"msg":"(pre-upgrade snapshot written|migration applied)"' > "$MIG_SRC_OUT/mig-journal.txt" || true
  snaps=$(dxc "$MIG_TGT" bash -c 'cd /opt/holzcloud/data && ls -1 | grep -E "^pre-upgrade-.*\.sqlite$" || true' | tr -d '\r')
  n=$(printf '%s\n' "$snaps" | grep -c . || true)
  put "$f" SNAPSHOTS-AFTER "$n"
  for s in $snaps; do
    put "$f" SNAPSHOT-FILE "/opt/holzcloud/data/$s"
    put "$f" SNAPSHOT-LEVEL "$(level_file "$MIG_TGT" "/opt/holzcloud/data/$s")"
  done
  put "$f" JOURNAL-SNAPSHOT-LINES "$(grep -c '"msg":"pre-upgrade snapshot written"' "$MIG_SRC_OUT/mig-journal.txt" || true)"
  put "$f" JOURNAL-APPLIED-LINES "$(grep -c '"msg":"migration applied"' "$MIG_SRC_OUT/mig-journal.txt" || true)"
  put "$f" JOURNAL-APPLIED-VERSIONS "$(applied_versions "$MIG_SRC_OUT/mig-journal.txt")"
  put "$f" LEVEL-AFTER "$(level_live "$MIG_TGT")"
  put "$f" LEVEL-EXPECTED-AFTER "$(migrations_at "$NEW_REF")"

  # A restored installation has an account, so the setup form is closed.
  put "$f" SETUP-AFTER "$(form_in "$MIG_TGT" get /admin/setup /root/.hc-jar-t34-probe | sed 's/^HTTP \([^ ]*\) .*/\1/')"
  lg=$(form_in "$MIG_TGT" login /admin/login login@example.test /root/.hc-login-pass /root/.hc-jar-t34) || lg="HTTP error"
  put "$f" LOGIN-AFTER "${lg#HTTP }"
  put "$f" LOGIN-NEXT-AFTER "$(form_in "$MIG_TGT" get /admin/ /root/.hc-jar-t34 | sed 's/^HTTP //')"
  dxc "$MIG_TGT" bash -c 'rm -f /root/.hc-jar-t34 /root/.hc-jar-t34-probe' || true
  put "$f" PAGE-AFTER "$(http_status "$MIG_TGT" /plugintest)"
  s=$(media_get "$MIG_TGT" "$MEDIA_ROW") || s=""
  put "$f" MEDIA-AFTER "$s"
  case "$MEDIA_BEFORE" in
    "HTTP 200 "[0-9a-f]*)
      if [ "$s" = "$MEDIA_BEFORE" ]; then put "$f" MEDIA-SHA-EQUAL yes; else put "$f" MEDIA-SHA-EQUAL no; fi
      ;;
    *) put "$f" MEDIA-SHA-EQUAL no ;;
  esac

  # run.sh check on the target: it restarts the unit and makes 17 assertions.
  dxc "$MIG_TGT" systemctl reset-failed holzcloud > /dev/null 2>&1 || true
  run_sh "$MIG_TGT" "$MIG_TGT_OUT" check || rc=$?
  put "$f" CHECK-RC "$rc"
  if [ -f "$MIG_TGT_OUT/check.txt" ]; then
    cp "$MIG_TGT_OUT/check.txt" "$MIG_SRC_OUT/mig-check.txt"
  else
    : > "$MIG_SRC_OUT/mig-check.txt"
  fi
  pass=$(grep -c '^PASS ' "$MIG_SRC_OUT/mig-check.txt" || true)
  bad=$(grep -c '^BAD ' "$MIG_SRC_OUT/mig-check.txt" || true)
  put "$f" CHECK "$pass of $((pass + bad))"
  put "$f" ORDER "none (the stage writes no order row; transfer only carries the line)"
}

stage_mig() {
  local f=$MIG_SRC_OUT/mig.txt
  MIG_DIR=""
  mig_source "$f"
  mig_backup "$f"
  mig_restore "$f"
  mig_after "$f"
  printf 'MIG-01 %s\n' "$(verdict_mig01 "$f")" >> "$f"
  printf 'MIG-02 %s\n' "$(verdict_mig02 "$f")" >> "$f"
}

# mig_deviations: deviations.txt of the source directory, the driver's own lines
# first, then those run.sh wrote for the source and for the target; a line that
# occurs twice (the same install played twice) is kept once.
mig_deviations() {
  local d=$MIG_SRC_OUT arch
  arch=$(docker info --format '{{.Architecture}}')
  case "$arch" in
    x86_64 | amd64) ;;
    *) deviation "DEVIATION[emulation]: the Docker host is $arch; the linux/amd64 containers run under emulation, so every wall time of this set (the first start after the restore among them) is an emulation figure, not a server's" ;;
  esac
  [ ! -f "$d/deviations.txt" ] || mv "$d/deviations.txt" "$d/deviations-runsh-src.lst"
  cat "$d/deviations-driver.lst" "$d/deviations-runsh-src.lst" "$MIG_TGT_OUT/deviations.txt" 2> /dev/null \
    | awk '/^DEVIATION\[/ && !seen[$0]++' > "$d/deviations.txt"
}

# mig_versions: run.sh's versions.txt of the source (written by `up`), then both
# releases and the tools of this set.
mig_versions() {
  local f=$MIG_SRC_OUT/versions.txt
  {
    echo "host uname -m:         $(uname -m)"
    echo "container uname -m:    $(dxc "$MIG_SRC" uname -m | tr -d '\r')"
    echo "container systemctl:   $(dxc "$MIG_TGT" systemctl --version | head -n 1 | tr -d '\r')"
    echo "source binary:         $(dxc "$MIG_SRC" /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r')"
    echo "target binary:         $(dxc "$MIG_TGT" /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r')"
    echo "$OLD_TAG commit:         $OLD_SHA"
    echo "NEW_REF:               $NEW_REF"
    echo "evidence commit:       $(cat "$EVIDENCE_FILE")"
  } >> "$f"
}

# ---------------------------------------------------------------------------
# The document played line by line (stages upd and rb). Every line comes from
# doc_block; play_kind says how it is played; nothing of the Updating or Rolling
# back blocks is typed here.
# ---------------------------------------------------------------------------

# Run INSIDE the container. PLAY_SH: $1 is one document line, run as root in its
# own bash; every output line is stamped with the container's clock, then a line
# "WALL t0 t1", and the status is the line's own. The line's bash has pipefail,
# so the status of a pipeline is that of its last failing command: the check
# lines pipe curl into jq, and jq alone exits 0 on an empty input. What runs is
# the line as written.
PLAY_SH='set -o pipefail
t0=$(date +%s.%N)
bash -o pipefail -c "$1" 2>&1 | while IFS= read -r l; do printf "%s %s\n" "$(date +%s.%N)" "$l"; done
rc=${PIPESTATUS[0]}
t1=$(date +%s.%N)
echo "WALL $t0 $t1"
exit "$rc"'

# Set by play_block: the status of each line it played, in order, and the
# scratch file (.lst, never pasted) holding the output of each, one per line.
PLAY_RCS=""
PLAY_FILES=""

# play_block NAME OUT TAG SECTION N BIN: the Nth bash block of SECTION of the
# document, line by line in the container NAME, each line through play_kind:
# scp-binary as docker cp of BIN (DEVIATION[scp]), scp-files as docker cp of the
# working tree's files (DEVIATION[scp]), build skipped (the binary was made by
# that same line in the scratch clone, DEVIATION[synthetic-build]), run in its
# own shell (PLAY_SH). OUT gets "$ LINE", the stamped output and "RC N" per line.
# A block that is not in the document, or a line play_kind refuses, ends the stage.
play_block() {
  local n=$1 out=$2 tag=$3 sec=$4 b=$5 bin=${6:-} lines line kind rc dest f lf i=0 files
  safe_name "$n" || die "refusing container name $n"
  lines=$(doc_block "$DEPLOY_DOC" "$sec" "$b") \
    || die "the document and this file have drifted apart: $DEPLOY_DOC has no bash block $b in the section '$sec'"
  PLAY_RCS=""
  PLAY_FILES=""
  while IFS= read -r line; do
    i=$((i + 1))
    kind=$(play_kind "$line") || die "no translation for this line of '$sec', block $b: $line"
    lf=$(dirname "$out")/$tag-$b-$i.lst
    printf '$ %s\n' "$line" >> "$out"
    say ">>> $tag $sec $b.$i ($kind): $line"
    rc=0
    case "$kind" in
      scp-binary)
        [ -n "$bin" ] && [ -f "$bin" ] || die "$sec block $b sends a binary, and there is none to send ('$bin')"
        dest=${line##*:}
        [ -n "$dest" ] || dest=/root/holzcloud
        deviation "DEVIATION[scp]: '$line' was played as docker cp of the synthetic build of this update to $dest in the container (there is no ssh and no second machine here)"
        docker cp "$bin" "$n:$dest" < /dev/null > "$lf" 2>&1 || rc=$?
        ;;
      scp-files)
        dest=${line##*:}
        [ -n "$dest" ] || dest=/root
        files=$(printf '%s\n' "$line" | awk '{ for (i = 2; i < NF; i++) printf "%s ", $i }' | sed 's/ $//')
        deviation "DEVIATION[scp]: '$line' was played as docker cp of the working tree's $files to $dest/ in the container (there is no ssh and no second machine here)"
        : > "$lf"
        for f in $files; do
          [ -f "$ROOT/$f" ] || die "$sec block $b: $f is not a file of the working tree"
          docker cp "$ROOT/$f" "$n:$dest/" < /dev/null >> "$lf" 2>&1 || rc=$?
        done
        ;;
      build)
        printf 'SKIP[synthetic-build] the binary of this update was built by this line in a scratch clone before the update\n' > "$lf"
        ;;
      run)
        docker exec "$n" bash -c "$PLAY_SH" _ "$line" < /dev/null > "$lf" 2>&1 || rc=$?
        ;;
    esac
    cat "$lf" >> "$out"
    printf 'RC %s\n' "$rc" >> "$out"
    PLAY_RCS="${PLAY_RCS:+$PLAY_RCS }$rc"
    PLAY_FILES="${PLAY_FILES:+$PLAY_FILES }$lf"
  done <<< "$lines"
}

# play_text FILE: the output of one played line without the clock and without the
# WALL line.
play_text() { unstamped "$1" | grep -v '^WALL ' || true; }

# nth_word N WORDS...: the Nth of the words.
nth_word() {
  local k=$1
  shift
  [ "$k" -ge 1 ] && [ "$k" -le "$#" ] || return 1
  shift $((k - 1))
  printf '%s\n' "$1"
}

# readyz_of FILE: the jq output of a played /readyz line, joined to one line
# without blanks (the values carry none).
readyz_of() {
  play_text "$1" | awk '/^\{/ { on = 1 } on { print } on && /^\}/ { exit }' | tr -d ' \n'
}

# stage_zips NAME: the plugin archives of this tree, where run.sh seed reads them.
stage_zips() {
  local id
  dxc "$1" mkdir -p /opt/holzcloud/plugin-zips || die "could not create /opt/holzcloud/plugin-zips"
  for id in $PLUGIN_IDS; do
    [ -f "$ROOT/plugins/$id/$id.zip" ] || die "plugins/$id/$id.zip is missing in the working tree"
    docker cp "$ROOT/plugins/$id/$id.zip" "$1:/opt/holzcloud/plugin-zips/$id.zip" > /dev/null \
      || die "docker cp of plugins/$id/$id.zip failed"
  done
  deviation "DEVIATION[fixture]: the five plugin archives of this tree are copied into /opt/holzcloud/plugin-zips before run.sh seed; with the website, page, AI key and test image run.sh seed makes, they stand in for operator work in the admin and are no step of the document"
}

# ---------------------------------------------------------------------------
# Synthetic builds (DEVIATION[synthetic-build]): this tree with added migrations,
# in a scratch clone (git clone --no-checkout and a checkout of NEW_REF, as
# run.sh's bins_build does; never git worktree), one throwaway local commit per
# build so that git describe names each build apart, the Build line of the
# document played in the clone. The binaries and their .info are kept in the
# stage directory; the clone is deleted by a trap, also on failure. Nothing is
# committed or pushed in this repository.
# ---------------------------------------------------------------------------

# syn_clone DIR: the clone in DIR/src at NEW_REF; dies unless the highest migration
# number equals the number of migration files (the next file is that plus one).
syn_clone() {
  local top
  git clone -q --no-checkout "$ROOT" "$1/src" || die "synthetic build: git clone failed"
  git -C "$1/src" checkout -q -f --detach "$NEW_REF" || die "synthetic build: git checkout of $NEW_REF failed"
  top=$(ls "$1/src/internal/db/migrations" | grep -E '^[0-9]+_.*\.sql$' | sort | tail -n 1 | sed 's/_.*//; s/^0*//')
  [ "${top:-0}" -eq "$(migrations_at "$NEW_REF")" ] \
    || die "synthetic build: the highest migration ($top) is not the number of migration files at $NEW_REF"
}

# syn_migration DIR VERSION NAME UP...: a migration file VERSION_NAME.sql in the
# clone, the Up statements given, a Down that drops the table of the first.
syn_migration() {
  local d=$1 v=$2 name=$3 t
  shift 3
  t=$(printf '%s\n' "$1" | sed -n 's/^CREATE TABLE \([a-z0-9_]*\) .*/\1/p')
  {
    echo "-- +goose Up"
    echo "-- Phase 34: a throwaway migration of a synthetic build (deploy/test/upgrade-verify.sh); never committed."
    for s in "$@"; do echo "$s"; done
    echo
    echo "-- +goose Down"
    echo "DROP TABLE $t;"
  } > "$d/src/internal/db/migrations/$(printf '%05d' "$v")_$name.sql"
}

# syn_commit_build DIR BIN MESSAGE: the added files committed in the clone (its
# own identity, no hook, no signature), the Build line of the document played
# there, the binary moved to BIN and BIN.info written.
syn_commit_build() {
  local src=$1/src bin=$2 line
  git -C "$src" add internal/db/migrations || die "synthetic build: git add failed"
  git -C "$src" -c user.name=t34 -c user.email=t34@invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null \
    commit -q -m "$3" || die "synthetic build: git commit in the scratch clone failed"
  line=$(doc_block "$DEPLOY_DOC" Build 1) || die "the document has no Build line"
  [ "$(printf '%s\n' "$line" | wc -l | tr -d ' ')" = 1 ] || die "the Build block of $DEPLOY_DOC is not one line"
  say "synthetic build ($3): $line"
  (cd "$src" && bash -c "$line") || die "synthetic build: the Build line failed"
  [ -f "$src/holzcloud" ] || die "synthetic build: the Build line produced no holzcloud file"
  mv "$src/holzcloud" "$bin" || die "synthetic build: moving the binary to $bin failed"
  [ -f "$bin" ] || die "synthetic build: $bin is not the built binary"
  {
    echo "version: $(git -C "$src" describe --tags --always)"
    echo "commit: $(git -C "$src" rev-parse --short HEAD)"
    echo "sha256: $(shasum -a 256 "$bin" | awk '{ print $1 }')"
    echo "migrations: $(ls "$src/internal/db/migrations" | grep -c '\.sql$')"
  } > "$bin.info" || die "synthetic build: writing $bin.info failed"
}

# info_of BIN KEY: one value of BIN.info.
info_of() { sed -n "s/^$2: //p" "$1.info" | head -n 1; }

# syn_build_rb DIR: F, this tree plus a migration that succeeds and one that fails
# after it has created a table (two files, because goose runs each file in its
# own transaction: one failing file alone is rolled back and the database stays
# at the old level). The binary is DIR/syn/holzcloud-f.
syn_build_rb() {
  local d=$1 b0
  b0=$(migrations_at "$NEW_REF")
  mkdir -p "$d/syn" || die "rb: cannot create $d/syn"
  # Every step is checked by hand: bash ignores `set -e` in a subshell that is
  # the left side of `||`, and a step that failed silently (an empty tmp from
  # mktemp, above all) would clone outside any scratch directory.
  (
    tmp=$(syn_scratch rb) || exit 1
    trap 'rm -rf "${tmp:?}"' EXIT
    syn_clone "$tmp" || exit 1
    syn_migration "$tmp" "$((b0 + 1))" t34_ok "CREATE TABLE t34_ok (id INTEGER PRIMARY KEY) STRICT;" || exit 1
    syn_migration "$tmp" "$((b0 + 2))" t34_fail "CREATE TABLE t34_half (id INTEGER PRIMARY KEY) STRICT;" \
      "INSERT INTO t34_no_such_table VALUES (1);" || exit 1
    syn_commit_build "$tmp" "$d/syn/holzcloud-f" "t34: F, a migration that succeeds and one that fails part-way" || exit 1
  ) || die "rb: the synthetic build F failed"
}

# syn_scratch KIND: a new scratch directory $TMPDIR/t34-syn-KIND.XXXXXX for a
# synthetic build's clone, its path printed; fails, printing nothing, unless
# mktemp made a directory.
syn_scratch() {
  local t
  t=$(mktemp -d "${TMPDIR:-/tmp}/t34-syn-$1.XXXXXX") || return 1
  [ -n "$t" ] && [ -d "$t" ] || return 1
  printf '%s\n' "$t"
}

# ---------------------------------------------------------------------------
# Stage rb: an update to a build whose migration fails part-way, the restart loop,
# the previous binary on the half-migrated database, Rolling back block 1.
# ---------------------------------------------------------------------------

# Runs INSIDE the container as the service user (runuser -u holzcloud -- bash -s):
# the schema level, the table list, the row counts of five tables and two content
# hashes, through a read-only connection to the live database (the service may be
# running, and the immutable URI would ignore its -wal file). Never a holzcloud
# subcommand, never a row of a credential table.
read -r -d '' RB_STATE_SH << 'RB_STATE_EOF' || true
set -eu
db=/opt/holzcloud/data/holzcloud.sqlite
q() { sqlite3 -readonly "$db" ".timeout 5000" "$1"; }
echo "schema: $(q "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1")"
echo "tables: $(q "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name" | tr '\n' ' ' | sed 's/ $//')"
for t in pages users media orders plugins; do
  echo "rows $t: $(q "select count(*) from $t")"
done
if [ "$(q "select count(*) from pages where slug = 'plugintest'")" = 1 ]; then
  echo "page: $(q "select content_markdown from pages where slug = 'plugintest'" | sha256sum | cut -d' ' -f1)"
else
  echo "page: none"
fi
m=$(q "select website_id || '/' || filename from media order by id limit 1")
if [ -n "$m" ] && [ -f "/opt/holzcloud/data/media/$m" ]; then
  echo "media: $(sha256sum < "/opt/holzcloud/data/media/$m" | cut -d' ' -f1)"
else
  echo "media: none"
fi
RB_STATE_EOF

# state_capture NAME FILE: RB_STATE_SH in that container into FILE.
state_capture() {
  safe_name "$1" || die "refusing container name $1"
  printf '%s\n' "$RB_STATE_SH" | docker exec -i "$1" runuser -u holzcloud -- bash -s | tr -d '\r' > "$2" \
    || die "the state capture failed in $1"
  grep -q '^schema: [0-9]' "$2" || die "the state capture in $1 has no schema line"
}

# snapshot_names NAME: the pre-upgrade snapshot files of the data directory, in
# name order, one per line.
snapshot_names() {
  dxc "$1" bash -c 'cd /opt/holzcloud/data && ls -1 | grep -E "^pre-upgrade-.*\.sqlite$" || true' | tr -d '\r'
}

# snapshot_levels NAME NAMES: "NAME:LEVEL ..." for the names, each level read
# through the immutable URI.
snapshot_levels() {
  local n=$1 s out=""
  for s in $2; do
    out="$out $s:$(level_file "$n" "/opt/holzcloud/data/$s")"
  done
  printf '%s\n' "${out# }"
}

# unit_prop NAME PROP: one property of holzcloud.service.
unit_prop() { dxc "$1" systemctl show -p "$2" --value holzcloud | tr -d '\r'; }

# wait_start_window NAME: wait until the unit's last start is older than the
# unit's StartLimitIntervalSec, so the starts of the install do not count against
# the burst of the update that follows (on a server an update comes long after the
# last start). A wait, not a reset: the unit is not touched. WAITED_S is set.
WAITED_S=0
wait_start_window() {
  local n=$1 iv t now left
  iv=$(sed -n 's/^StartLimitIntervalSec=\([0-9][0-9]*\)$/\1/p' deploy/holzcloud.service | head -n 1)
  is_count "$iv" || die "deploy/holzcloud.service has no StartLimitIntervalSec in seconds"
  t=$(dxc "$n" bash -c 'date -d "$(systemctl show -p ExecMainStartTimestamp --value holzcloud)" +%s' | tr -d '\r') || t=""
  is_count "$t" || die "could not read the time of the unit's last start"
  now=$(dxc "$n" date +%s | tr -d '\r')
  left=$((t + iv + 2 - now))
  WAITED_S=0
  if [ "$left" -gt 0 ]; then
    say "waiting ${left}s until the last start of the unit is older than StartLimitIntervalSec=$iv"
    sleep "$left"
    WAITED_S=$left
  fi
}

# wait_failed NAME MAX: poll every two seconds, at most MAX seconds, until the unit
# is failed and NRestarts has not changed for two polls; /healthz is probed on
# each poll. Sets WF_STATE, WF_NRESTARTS, WF_RESULT, WF_HEALTHZ (never|answered)
# and WF_WAITED.
wait_failed() {
  local n=$1 max=$2 i=0 st="" nr="" prev=x stable=0
  WF_HEALTHZ=never
  while [ "$i" -lt "$max" ]; do
    if dxc "$n" curl -fsS --max-time 2 http://127.0.0.1:8080/healthz > /dev/null 2>&1; then
      WF_HEALTHZ=answered
    fi
    st=$(unit_prop "$n" ActiveState)
    nr=$(unit_prop "$n" NRestarts)
    if [ "$st" = failed ] && [ "$nr" = "$prev" ]; then
      stable=$((stable + 1))
    else
      stable=0
    fi
    prev=$nr
    [ "$stable" -lt 2 ] || break
    sleep 2
    i=$((i + 2))
  done
  WF_STATE=$st
  WF_NRESTARTS=$nr
  WF_RESULT=$(unit_prop "$n" Result)
  WF_WAITED=$i
}

# rb_update: B0 installed and seeded, s0, F built, the Updating blocks played to F.
rb_update() {
  local f=$1 d=$RB_OUT b0 fv b
  run_sh "$RB_NAME" "$RB_OUT" build up install postinstall
  : > "$f"
  DEV_FILE=$RB_OUT/deviations-driver.lst
  : > "$DEV_FILE"
  stage_zips "$RB_NAME"
  run_sh "$RB_NAME" "$RB_OUT" seed || die "rb: run.sh seed failed"
  b0=$(migrations_at "$NEW_REF")
  put "$f" B0-VERSION "$(dxc "$RB_NAME" bash -c 'curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | jq -r .version' | tr -d '\r')"
  put "$f" LEVEL-EXPECTED-B0 "$b0"
  put "$f" LEVEL-EXPECTED-F "$((b0 + 2))"
  state_capture "$RB_NAME" "$d/state-s0.lst"
  grep -qx 'page: none' "$d/state-s0.lst" && die "rb: the seeded page plugintest is missing in s0"
  grep -qx 'media: none' "$d/state-s0.lst" && die "rb: the uploaded image is missing in s0"

  syn_build_rb "$d"
  F_BIN=$d/syn/holzcloud-f
  fv=$(info_of "$F_BIN" version)
  put "$f" F-BUILD "$fv ($(info_of "$F_BIN" commit)) migrations $(info_of "$F_BIN" migrations) sha256 $(info_of "$F_BIN" sha256)"
  [ "$(info_of "$F_BIN" migrations)" -eq "$((b0 + 2))" ] || die "rb: F does not carry two migrations more than this tree"
  deviation "DEVIATION[synthetic-build]: the binary of this update is F ($fv), a throwaway build of this tree with two added migrations, $(printf '%05d' $((b0 + 1)))_t34_ok.sql (creates t34_ok) and $(printf '%05d' $((b0 + 2)))_t34_fail.sql (creates t34_half, then inserts into a table that does not exist), made in a scratch clone with one local commit and the Build line of the document, the clone deleted afterwards; nothing of it is committed or pushed. The Build line of Updating block 2 is skipped for it. Released binaries give no update whose migration fails part-way while the previous binary carries the schema guard"

  wait_start_window "$RB_NAME"
  put "$f" START-WINDOW-WAIT-S "$WAITED_S"

  # No reset-failed before or during the update: the restart loop is the evidence.
  RB_T0=$(dxc "$RB_NAME" date +%s | tr -d '\r')
  : > "$d/rb-update.out"
  for b in 1 2 3; do
    play_block "$RB_NAME" "$d/rb-update.out" rb-update Updating "$b" "$F_BIN"
  done
  put "$f" UPDATE-CHECK-RC "$(nth_word 1 $PLAY_RCS)"
  # The journal line of the check block, as DEPLOY.md *Updating* step 3 has it:
  # after a failed update it must name the snapshot (written by the first start,
  # kept by the restarts), never print "no migration, no snapshot".
  local ck
  ck=$(play_text "$(nth_word 2 $PLAY_FILES)")
  if printf '%s\n' "$ck" | grep -qE 'pre-upgrade snapshot (written|kept)' \
    && ! printf '%s\n' "$ck" | grep -qF 'no migration, no snapshot'; then
    put "$f" UPDATE-CHECK-SNAPSHOT-LINE yes
  else
    put "$f" UPDATE-CHECK-SNAPSHOT-LINE no
  fi
}

# rb_loop: the restart loop to its end, the journal of the update window, the
# half-migrated database and the snapshots.
rb_loop() {
  local f=$1 d=$RB_OUT j wl paths names fv
  wait_failed "$RB_NAME" 240
  put "$f" NRESTARTS "$WF_NRESTARTS"
  put "$f" RESULT "$WF_RESULT"
  put "$f" ACTIVE-STATE "$WF_STATE"
  put "$f" HEALTHZ-DURING-LOOP "$WF_HEALTHZ"
  j=$d/rb-journal.txt
  dxc "$RB_NAME" journalctl -u holzcloud --since "@$RB_T0" -o cat --no-pager | tr -d '\r' > "$j" || true
  fv=$(info_of "$F_BIN" version)
  put "$f" STARTS "$(grep -c "\"msg\":\"starting\",\"version\":\"$fv\"" "$j" || true)"
  put "$f" JOURNAL-SNAPSHOT-WRITTEN "$(grep -c '"msg":"pre-upgrade snapshot written"' "$j" || true)"
  put "$f" JOURNAL-SNAPSHOT-KEPT "$(grep -c '"msg":"pre-upgrade snapshot kept"' "$j" || true)"
  put "$f" JOURNAL-MIGRATIONS-FAILED "$(grep -c '"msg":"migrations failed"' "$j" || true)"
  put "$f" JOURNAL-RESTORE-WITH "$(grep -c '"msg":"restore with: ' "$j" || true)"
  paths=$(restore_with_paths "$j")
  put "$f" RESTORE-WITH-DISTINCT-PATHS "$(printf '%s\n' "$paths" | grep -c . || true)"
  wl=$(grep '"msg":"pre-upgrade snapshot written"' "$j" | head -n 1) || wl=""
  if [ -n "$paths" ] && [ "$paths" = "$(json_str "$wl" path)" ]; then
    put "$f" RESTORE-WITH-NAMES-WRITTEN yes
  else
    put "$f" RESTORE-WITH-NAMES-WRITTEN no
  fi
  put "$f" WRITTEN-PATH "$(json_str "$wl" path)"
  put "$f" WRITTEN-LEVEL "$(json_num "$wl" level)"
  put "$f" STOP-MESSAGE "$(json_err "$(grep '"msg":"migrations failed"' "$j" | head -n 1)")"
  put "$f" APPLIED-BEFORE-FAILURE "$(applied_versions "$j")"
  if grep -q 'Start request repeated too quickly' "$j"; then
    put "$f" START-LIMIT-LINE yes
  else
    put "$f" START-LIMIT-LINE no
  fi
  put "$f" LEVEL-HALF "$(level_live "$RB_NAME")"
  put "$f" MARKER-DURING "$(dxc "$RB_NAME" bash -c 'cat /opt/holzcloud/data/pre-upgrade.pending 2>/dev/null || echo absent' | tr -d '\r')"
  names=$(snapshot_names "$RB_NAME")
  put "$f" SNAPSHOTS-AFTER-LOOP "$(snapshot_levels "$RB_NAME" "$names")"
  {
    echo "the pre-upgrade snapshots after the restart loop, in name order, each with its level (immutable read):"
    for s in $names; do
      echo "/opt/holzcloud/data/$s level $(level_file "$RB_NAME" "/opt/holzcloud/data/$s")"
    done
  } > "$d/rb-snapshots.txt"
}

# rb_guard: SCENARIO[previous-on-half], the previous binary started on the
# half-migrated database, which the document says not to do.
rb_guard() {
  local f=$1 d=$RB_OUT t1 g gl b0 want
  scenario "SCENARIO[previous-on-half]: after the restart loop ended, systemctl reset-failed holzcloud, cp -p /opt/holzcloud/holzcloud.prev /opt/holzcloud/holzcloud and systemctl start holzcloud: the previous binary is started on the half-migrated database, which Rolling back says not to do, so that the record shows the schema guard refusing it"
  deviation "DEVIATION[start-limit]: systemctl reset-failed holzcloud runs once outside the document, before the previous binary is started on the half-migrated database (SCENARIO[previous-on-half]); the restart loop of the failing update ran to its end without one, and the reset-failed of Rolling back block 1 is the document's own line"
  t1=$(dxc "$RB_NAME" date +%s | tr -d '\r')
  dxc "$RB_NAME" systemctl reset-failed holzcloud || die "rb: reset-failed failed"
  dxc "$RB_NAME" cp -p /opt/holzcloud/holzcloud.prev /opt/holzcloud/holzcloud || die "rb: putting the previous binary in place failed"
  dxc "$RB_NAME" systemctl start holzcloud || true
  wait_failed "$RB_NAME" 240
  put "$f" GUARD-HEALTHZ "$WF_HEALTHZ"
  put "$f" GUARD-ACTIVE-STATE "$WF_STATE"
  g=$d/rb-guard.txt
  dxc "$RB_NAME" journalctl -u holzcloud --since "@$t1" -o cat --no-pager | tr -d '\r' > "$g" || true
  gl=$(grep '"msg":"refusing to start: the database schema is newer than this binary"' "$g" | head -n 1) || gl=""
  put "$f" GUARD-STARTS "$(grep -c '"msg":"refusing to start: the database schema is newer than this binary"' "$g" || true)"
  if [ -n "$gl" ]; then put "$f" GUARD-LINE yes; else put "$f" GUARD-LINE no; fi
  put "$f" GUARD-DB-LEVEL "$(json_num "$gl" db_level)"
  put "$f" GUARD-BINARY-LEVEL "$(json_num "$gl" binary_level)"
  b0=$(migrations_at "$NEW_REF")
  want=$(guard_sentence internal/db/schema_guard.go "$((b0 + 1))" "$b0") || want=""
  if [ -n "$want" ] && grep -Fq "\"err\":\"$(json_escape "$want")\"}" "$g"; then
    put "$f" GUARD-SENTENCE yes
  else
    put "$f" GUARD-SENTENCE no
  fi
}

# rb_rollback: Rolling back block 1 as written, then the state after it.
rb_rollback() {
  local f=$1 d=$RB_OUT nf
  : > "$d/rb-rollback.out"
  play_block "$RB_NAME" "$d/rb-rollback.out" rb-rollback "Rolling back" 1 ""
  put "$f" ROLLBACK-RC "$PLAY_RCS"
  set -- $PLAY_FILES
  nf=$#
  [ "$nf" -ge 2 ] || die "rb: Rolling back block 1 has fewer than two lines"
  put "$f" ROLLBACK-READYZ "$(readyz_of "$(nth_word $((nf - 1)) $PLAY_FILES)")"
  put "$f" ROLLBACK-CHECK-LINE "$(play_text "$(nth_word "$nf" $PLAY_FILES)" | tail -n 1)"

  state_capture "$RB_NAME" "$d/state-s2.lst"
  put "$f" LEVEL-AFTER-ROLLBACK "$(level_live "$RB_NAME")"
  if [ "$(dxc "$RB_NAME" runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite ".timeout 5000" \
    "select count(*) from sqlite_master where type = 'table' and name = 't34_ok'" | tr -d '\r')" = 0 ]; then
    put "$f" T34-OK-TABLE absent
  else
    put "$f" T34-OK-TABLE present
  fi
  put "$f" AFTER-UPDATE-LEVEL "$(level_file "$RB_NAME" /opt/holzcloud/data/holzcloud.sqlite.after-update)"
  if dxc "$RB_NAME" test -e /opt/holzcloud/data/pre-upgrade.pending; then
    put "$f" MARKER-AFTER present
  else
    put "$f" MARKER-AFTER absent
  fi
  {
    echo "s0, before the update (B0 running, seeded):"
    sed 's/^/s0 /' "$d/state-s0.lst"
    echo "s2, after Rolling back block 1:"
    sed 's/^/s2 /' "$d/state-s2.lst"
    echo "comparison line by line:"
    compare_state "$d/state-s0.lst" "$d/state-s2.lst"
  } > "$d/rb-state.txt"
  put "$f" COMPARE-S0-S2 "$(compare_state "$d/state-s0.lst" "$d/state-s2.lst" | tail -n 1)"
}

stage_rb() {
  local f=$RB_OUT/rb.txt
  rb_update "$f"
  rb_loop "$f"
  rb_guard "$f"
  rb_rollback "$f"
  printf 'UPD-02 %s\n' "$(verdict_upd02 "$f")" >> "$f"
}

# ---------------------------------------------------------------------------
# Stage upd: six migrating updates on one data directory, the pruning of the
# pre-upgrade snapshots to the newest five.
# ---------------------------------------------------------------------------

# syn_build_upd DIR: six builds in one scratch clone, build k carrying the no-op
# migrations 1 to k (each after its own commit, so each build's version differs);
# the binaries are DIR/syn/holzcloud-syn-K, listed in DIR/upd-builds.txt.
syn_build_upd() {
  local d=$1 b0 k
  b0=$(migrations_at "$NEW_REF")
  mkdir -p "$d/syn" || die "upd: cannot create $d/syn"
  # Checked by hand, as in syn_build_rb.
  (
    tmp=$(syn_scratch upd) || exit 1
    trap 'rm -rf "${tmp:?}"' EXIT
    syn_clone "$tmp" || exit 1
    for k in 1 2 3 4 5 6; do
      syn_migration "$tmp" "$((b0 + k))" "t34_noop_$k" "CREATE TABLE t34_noop_$k (id INTEGER PRIMARY KEY) STRICT;" || exit 1
      syn_commit_build "$tmp" "$d/syn/holzcloud-syn-$k" "t34: synthetic build $k, the no-op migrations 1 to $k" || exit 1
    done
  ) || die "upd: the synthetic builds failed"
  {
    echo "six synthetic builds of $NEW_REF ($(git describe --tags --always "$NEW_REF")), made in a scratch clone and deleted with it; build k adds the no-op migrations $(printf '%05d' $((b0 + 1))) to $(printf '%05d' $((b0 + 6))) up to the kth:"
    for k in 1 2 3 4 5 6; do
      echo "build $k: version $(info_of "$d/syn/holzcloud-syn-$k" version) commit $(info_of "$d/syn/holzcloud-syn-$k" commit) migrations $(info_of "$d/syn/holzcloud-syn-$k" migrations) sha256 $(info_of "$d/syn/holzcloud-syn-$k" sha256)"
    done
  } > "$d/upd-builds.txt"
}

# upd_snapshot_block TITLE NAMES: a block of upd-snapshots.txt, each name (oldest
# first) with its level.
upd_snapshot_block() {
  local s
  {
    echo "$1"
    for s in $2; do
      echo "  $s level $(level_file "$UPD_NAME" "/opt/holzcloud/data/$s")"
    done
  } >> "$UPD_OUT/upd-snapshots.txt"
}

stage_upd() {
  local f=$UPD_OUT/upd.txt d=$UPD_OUT b0 k b bin init now prev written="" w exp j left rj s total=0 logged=0
  run_sh "$UPD_NAME" "$UPD_OUT" build up install postinstall
  : > "$f"
  DEV_FILE=$UPD_OUT/deviations-driver.lst
  : > "$DEV_FILE"
  stage_zips "$UPD_NAME"
  run_sh "$UPD_NAME" "$UPD_OUT" seed || die "upd: run.sh seed failed"
  b0=$(migrations_at "$NEW_REF")
  put "$f" LEVEL-EXPECTED-B0 "$b0"
  init=$(by_stamp "$(snapshot_names "$UPD_NAME" | tr '\n' ' ')")
  put "$f" SNAPSHOTS-AFTER-0 "$init"
  : > "$d/upd-snapshots.txt"
  upd_snapshot_block "after the install and run.sh seed ($(dxc "$UPD_NAME" /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r')), oldest first:" "$init"

  syn_build_upd "$d"
  deviation "DEVIATION[synthetic-build]: the six binaries of the updates are throwaway builds of this tree, build k with the no-op migrations $(printf '%05d' $((b0 + 1)))_t34_noop_1.sql to $(printf '%05d' $((b0 + 6)))_t34_noop_6.sql up to the kth (each creates one empty table), made in one scratch clone with one local commit per build (so that each build's version differs) and the Build line of the document, the clone deleted afterwards; nothing of it is committed or pushed. The Build line of Updating block 2 is skipped for them. Released binaries alone never prune: of the tags only 0.1.2 to 0.1.3 migrates, so they give no five updates that each write a snapshot"
  deviation "DEVIATION[start-limit]: systemctl reset-failed holzcloud runs before each of the six updates: six updates within minutes, each with a stop and a start, meet StartLimitBurst (five starts in 300 seconds), which updates days apart never do"

  prev=$init
  for k in 1 2 3 4 5 6; do
    bin=$d/syn/holzcloud-syn-$k
    put "$f" "BUILD-$k" "$(info_of "$bin" version) ($(info_of "$bin" commit)) migrations $(info_of "$bin" migrations)"
    dxc "$UPD_NAME" systemctl reset-failed holzcloud > /dev/null 2>&1 || true
    t0=$(dxc "$UPD_NAME" date +%s | tr -d '\r')
    : > "$d/upd-$k.out"
    for b in 1 2 3; do
      play_block "$UPD_NAME" "$d/upd-$k.out" "upd-$k" Updating "$b" "$bin"
    done
    put "$f" "READYZ-$k" "$(dxc "$UPD_NAME" bash -c 'curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | jq -r .version' | tr -d '\r')"
    j=$d/upd-$k-journal.lst
    dxc "$UPD_NAME" journalctl -u holzcloud --since "@$t0" -o cat --no-pager | tr -d '\r' > "$j" || true
    w=$(json_str "$(grep '"msg":"pre-upgrade snapshot written"' "$j" | head -n 1)" path)
    w=${w##*/}
    put "$f" "WRITTEN-$k" "$w"
    rj=$({ grep '"msg":"removed old pre-upgrade snapshot"' "$j" || true; } | while IFS= read -r s; do
      s=$(json_str "$s" path)
      printf '%s ' "${s##*/}"
    done | sed 's/ $//')
    put "$f" "REMOVED-$k" "${rj:-none}"
    now=$(by_stamp "$(snapshot_names "$UPD_NAME" | tr '\n' ' ')")
    put "$f" "SNAPSHOTS-AFTER-$k" "$now"
    written="${written:+$written }$w"
    exp=$(prune_expect "$init" "$written" "$k") || exp="?"
    put "$f" "EXPECTED-$k" "$exp"
    if [ -n "$w" ] && [ "$now" = "$exp" ]; then put "$f" "MATCH-$k" yes; else put "$f" "MATCH-$k" no; fi
    left=$(removed_between "$prev" "$now")
    put "$f" "LEFT-$k" "${left:-none}"
    for s in $left; do
      total=$((total + 1))
      case " $rj " in
        *" $s "*) logged=$((logged + 1)) ;;
      esac
    done
    put "$f" "LEVEL-AFTER-$k" "$(level_live "$UPD_NAME")"
    upd_snapshot_block "after update $k ($(info_of "$bin" version)), oldest first:" "$now"
    prev=$now
  done
  put "$f" REMOVALS-TOTAL "$total"
  put "$f" REMOVALS-LOGGED "$logged"
  printf 'UPD-01 %s\n' "$(verdict_upd01 "$f")" >> "$f"
}

# scenario TEXT: a step the driver takes that the document must not contain, in
# the stage's deviations (run.sh's vocabulary).
scenario() {
  say "$*"
  [ -n "$DEV_FILE" ] || die "scenario before a stage directory exists: $*"
  printf '%s\n' "$*" >> "$DEV_FILE"
}

# stage_deviations DIR: deviations.txt of a stage directory, the driver's own lines
# first (DEVIATION and SCENARIO), then those run.sh wrote; a line that occurs twice
# is kept once.
stage_deviations() {
  local d=$1 arch
  arch=$(docker info --format '{{.Architecture}}')
  case "$arch" in
    x86_64 | amd64) ;;
    *) deviation "DEVIATION[emulation]: the Docker host is $arch; the linux/amd64 container runs under emulation, so every wall time of this set (the restarts of the unit among them) is an emulation figure, not a server's" ;;
  esac
  [ ! -f "$d/deviations.txt" ] || mv "$d/deviations.txt" "$d/deviations-runsh.lst"
  cat "$d/deviations-driver.lst" "$d/deviations-runsh.lst" 2> /dev/null \
    | awk '/^(DEVIATION|SCENARIO)\[/ && !seen[$0]++' > "$d/deviations.txt"
}

# stage_versions DIR NAME BINS...: run.sh's versions.txt of DIR (written by `up`),
# then the host, the container, the binaries of the stage and the commits.
stage_versions() {
  local d=$1 n=$2 b
  shift 2
  {
    echo "host uname -m:         $(uname -m)"
    echo "container uname -m:    $(dxc "$n" uname -m | tr -d '\r')"
    echo "container systemctl:   $(dxc "$n" systemctl --version | head -n 1 | tr -d '\r')"
    echo "running binary:        $(dxc "$n" /opt/holzcloud/holzcloud version | head -n 1 | tr -d '\r')"
    for b in "$@"; do
      echo "$(basename "$b"):  $(info_of "$b" version) ($(info_of "$b" commit)) migrations $(info_of "$b" migrations) sha256 $(info_of "$b" sha256)"
    done
    echo "NEW_REF:               $NEW_REF"
    echo "evidence commit:       $(cat "$EVIDENCE_FILE")"
  } >> "$d/versions.txt"
}

# head_binary: the version line of the HEAD build, from a container that runs it
# (the update chain ends on a synthetic build, so its container is not asked),
# else from the .info run.sh's bins wrote for it.
head_binary() {
  local n v i
  for n in "$MIG_TGT" "$RB_NAME"; do
    if docker container inspect "$n" > /dev/null 2>&1; then
      v=$(dxc "$n" /opt/holzcloud/holzcloud version 2> /dev/null | head -n 1 | tr -d '\r') || v=""
      if [ -n "$v" ]; then
        printf '%s\n' "$v"
        return 0
      fi
    fi
  done
  i=deploy/test/out/bin/holzcloud-new-$NEW_REF.info
  if [ -f "$i" ]; then
    printf 'holzcloud %s (%s)\n' "$(sed -n 's/^describe: *//p' "$i" | head -n 1)" "$(sed -n 's/^commit: *//p' "$i" | head -n 1)"
    return 0
  fi
  echo "?"
}

# write_summary: the verdicts from the files the stages wrote; a stage that did
# not run reads NOT-RUN whatever lies in its directory.
write_summary() {
  local f=$SUMMARY_FILE kinds m1=NOT-RUN m2=NOT-RUN u1=NOT-RUN u2=NOT-RUN d
  if stage_wanted mig; then
    m1=$(verdict_mig01 "$MIG_SRC_OUT/mig.txt")
    m2=$(verdict_mig02 "$MIG_SRC_OUT/mig.txt")
  fi
  if stage_wanted upd; then
    u1=$(verdict_upd01 "$UPD_OUT/upd.txt")
  fi
  if stage_wanted rb; then
    u2=$(verdict_upd02 "$RB_OUT/rb.txt")
  fi
  kinds=$(for d in "$MIG_SRC_OUT" "$UPD_OUT" "$RB_OUT"; do
    [ ! -f "$d/deviations.txt" ] || cat "$d/deviations.txt"
  done | sed -n 's/^DEVIATION\[\([a-z0-9-]*\)\].*/\1/p' | sort -u | tr '\n' ' ' | sed 's/ $//')
  {
    echo "MIG-01 $m1"
    echo "MIG-02 $m2"
    echo "UPD-01 $u1"
    echo "UPD-02 $u2"
    echo "STAGES: $STAGES"
    echo "DEVIATIONS: ${kinds:-none}"
    echo "BINARY: $(head_binary)"
    if is_rehearsal; then
      echo "REHEARSAL: yes"
    fi
  } > "$f"
  cat "$f" >&2
}

# run_body: everything `run` does after its guards; its output goes to the log.
run_body() {
  say "upgrade-verify: platform $PLATFORM, NEW_REF pinned to $NEW_REF, $OLD_TAG is $OLD_SHA, stages: $STAGES"
  if stage_wanted mig; then
    stage_mig
    mig_versions
    mig_deviations
  fi
  if stage_wanted upd; then
    stage_upd
    stage_versions "$UPD_OUT" "$UPD_NAME" "$UPD_OUT/syn/holzcloud-syn-1" "$UPD_OUT/syn/holzcloud-syn-2" \
      "$UPD_OUT/syn/holzcloud-syn-3" "$UPD_OUT/syn/holzcloud-syn-4" "$UPD_OUT/syn/holzcloud-syn-5" "$UPD_OUT/syn/holzcloud-syn-6"
    stage_deviations "$UPD_OUT"
  fi
  if stage_wanted rb; then
    stage_rb
    stage_versions "$RB_OUT" "$RB_NAME" "$RB_OUT/syn/holzcloud-f"
    stage_deviations "$RB_OUT"
  fi
  write_summary
  grep -Eq '^(MIG|UPD)-0[0-9] (FAIL|NOT-RUN)$' "$SUMMARY_FILE" && return 1
  return 0
}

cmd_run() {
  local rc p n
  stages_valid || {
    say "upgrade-verify.sh: STAGES '$STAGES' must name mig, upd or rb"
    exit 2
  }
  [ "$PLATFORM" = linux/amd64 ] || {
    say "upgrade-verify.sh: PLATFORM must be linux/amd64 (got '$PLATFORM')"
    exit 2
  }
  [ "$TARGET" = systemd-base ] || {
    say "upgrade-verify.sh: TARGET must be systemd-base (got '$TARGET')"
    exit 2
  }
  for p in "$MIG_SRC_OUT" "$MIG_TGT_OUT" "$UPD_OUT" "$RB_OUT" "$RUN_PREFIX.started" "$RUN_PREFIX.finished" \
    "$RUN_PREFIX.status" "$RUN_PREFIX.log" "$EVIDENCE_FILE" "$SUMMARY_FILE"; do
    safe_out "$p" || die "refusing path $p"
  done
  if ! bound_check; then
    if [ -n "$REHEARSAL" ]; then
      say "upgrade-verify.sh: REHEARSAL=1: going on although a bound file differs from HEAD; this set is not evidence"
    else
      die "a bound file differs from HEAD or is untracked; commit first (the evidence is bound to one commit)"
    fi
  fi
  command -v docker > /dev/null 2>&1 || {
    say "upgrade-verify.sh: docker is not installed"
    exit 3
  }
  docker info > /dev/null 2>&1 || {
    say "upgrade-verify.sh: the Docker daemon does not answer"
    exit 3
  }
  for n in "$MIG_SRC" "$MIG_TGT" "$UPD_NAME" "$RB_NAME"; do
    safe_name "$n" || die "refusing container name $n"
    if docker container inspect "$n" > /dev/null 2>&1; then
      die "container $n exists; run 'upgrade-verify.sh down' first (a stale one is never reused)"
    fi
  done
  NEW_REF=$(git rev-parse --short HEAD) || die "HEAD is not a commit"
  OLD_SHA=$(git rev-parse --verify --quiet "$OLD_TAG^{commit}") || die "the tag $OLD_TAG is not in this repository"
  mkdir -p "$BASE_OUT"
  # The directories of this file only: a stale stage directory never lends a verdict.
  for p in "$MIG_SRC_OUT" "$MIG_TGT_OUT" "$UPD_OUT" "$RB_OUT"; do
    rm -rf "${p:?}"
  done
  rm -f "$RUN_PREFIX.started" "$RUN_PREFIX.finished" "$RUN_PREFIX.status" "$RUN_PREFIX.log" "$SUMMARY_FILE"
  printf '%s\n' "$NEW_REF" > "$EVIDENCE_FILE"
  date -u +%Y-%m-%dT%H:%M:%SZ > "$RUN_PREFIX.started"
  set +e
  (set -e; run_body) 2>&1 | tee "$RUN_PREFIX.log"
  rc=${PIPESTATUS[0]}
  set -e
  date -u +%Y-%m-%dT%H:%M:%SZ > "$RUN_PREFIX.finished"
  printf '%s\n' "$rc" > "$RUN_PREFIX.status"
  say "upgrade-verify: run ended with status $rc"
  exit "$rc"
}

cmd_down() {
  local n
  command -v docker > /dev/null 2>&1 || die "docker is not installed"
  for n in "$MIG_SRC" "$MIG_TGT" "$UPD_NAME" "$RB_NAME"; do
    safe_name "$n" || die "refusing container name $n"
    docker rm -f "$n" > /dev/null 2>&1 || true
    say "upgrade-verify: container $n removed"
  done
}

# ---------------------------------------------------------------------------
# The records (plan 34-05 and 34-07 run these; they are here so that the driver
# bound by the evidence is the one that checks it). They never rely on `set -e`,
# print one line per problem and end on an answer line.
# ---------------------------------------------------------------------------

# paste_list STAGE: DIRECTORY/FILE (or FILE, for the run files) of the appendix of
# the record of STAGE, in order. A list, never a glob: the logs, the .lst scratch
# files, baseline.txt, seed.txt, steps.txt, the plugin archives, the old scripts
# and every key, password or cookie file are not on it.
paste_list() {
  case "${1:-}" in
    mig)
      printf '%s\n' \
        amd64-t34-summary.txt \
        amd64-t34-mig-src/versions.txt \
        amd64-t34-mig-src/deviations.txt \
        amd64-t34-mig-src/mig.txt \
        amd64-t34-mig-src/mig-backup.out \
        amd64-t34-mig-src/mig-restore.out \
        amd64-t34-mig-src/mig-journal.txt \
        amd64-t34-mig-src/mig-check.txt \
        amd64-t34-evidence-commit.txt \
        amd64-t34-pass1.status
      ;;
    upd)
      printf '%s\n' \
        amd64-t34-summary.txt \
        amd64-t34-upd/versions.txt \
        amd64-t34-upd/deviations.txt \
        amd64-t34-upd/upd.txt \
        amd64-t34-upd/upd-builds.txt \
        amd64-t34-upd/upd-1.out \
        amd64-t34-upd/upd-2.out \
        amd64-t34-upd/upd-3.out \
        amd64-t34-upd/upd-4.out \
        amd64-t34-upd/upd-5.out \
        amd64-t34-upd/upd-6.out \
        amd64-t34-upd/upd-snapshots.txt \
        amd64-t34-evidence-commit.txt \
        amd64-t34-pass1.status
      ;;
    rb)
      printf '%s\n' \
        amd64-t34-summary.txt \
        amd64-t34-rb/versions.txt \
        amd64-t34-rb/deviations.txt \
        amd64-t34-rb/rb.txt \
        amd64-t34-rb/rb-update.out \
        amd64-t34-rb/rb-journal.txt \
        amd64-t34-rb/rb-snapshots.txt \
        amd64-t34-rb/rb-guard.txt \
        amd64-t34-rb/rb-rollback.out \
        amd64-t34-rb/rb-state.txt \
        amd64-t34-evidence-commit.txt \
        amd64-t34-pass1.status
      ;;
    *) return 1 ;;
  esac
}

# record_stage R: the stage whose record R is (mig, upd or rb), from its file name
# under deploy/; fails for any other file.
record_stage() {
  case "${1:-}" in
    deploy/MIGRATION-RESTORE-RUN.md | */deploy/MIGRATION-RESTORE-RUN.md) echo mig ;;
    deploy/UPDATE-CHAIN-RUN.md | */deploy/UPDATE-CHAIN-RUN.md) echo upd ;;
    deploy/ROLLBACK-RUN.md | */deploy/ROLLBACK-RUN.md) echo rb ;;
    *) return 1 ;;
  esac
}

# appendix_print STAGE E: the blocks of the paste list of STAGE from the files
# below E, each "#### NAME", a blank line and a fenced text block. Refuses (prints
# nothing) when a file is missing or holds a line that would end a fence or open
# a heading.
appendix_print() {
  local st=${1:-} e=${2:-deploy/test/out} p bad=0 list
  list=$(paste_list "$st") || {
    say "upgrade-verify.sh: appendix: the stage must be mig, upd or rb (got '$st')"
    return 2
  }
  for p in $list; do
    if [ ! -f "$e/$p" ]; then
      say "upgrade-verify.sh: appendix: no scratch file $e/$p"
      bad=$((bad + 1))
    elif grep -Eq '^(```|#### |## )' "$e/$p"; then
      say "upgrade-verify.sh: appendix: $e/$p has a line that begins with three backticks, with '#### ' or with '## '; it cannot be pasted unchanged"
      bad=$((bad + 1))
    fi
  done
  if [ "$bad" -ne 0 ]; then
    return 1
  fi
  for p in $list; do
    printf '#### %s\n\n' "$p"
    printf '%s\n' '```text'
    awk 1 "$e/$p"
    printf '%s\n\n' '```'
  done
}

# t34_record_check R E STAGE: the record R against the scratch root E, with the
# paste list of STAGE.
#  - every "#### NAME" block under "## Appendix: raw output" equals E/NAME byte for
#    byte, and every file of the paste list has a block;
#  - every `amd64-t34-.../FILE` token in backticks outside the appendix has a block
#    (a file that is not pasted is named as "FILE in DIRECTORY");
#  - exactly one line "Evidence commit: SHA"; SHA resolves, is the pinned commit of
#    E/amd64-t34-evidence-commit.txt, and no bound file nor anything of
#    BINARY_SOURCE differs between SHA and HEAD;
#  - the pasted summary is not a rehearsal's, reads PASS for the record's own
#    verdicts (record_verdicts) and for every other verdict of the run, and the
#    pasted amd64-t34-pass1.status is exactly 0;
#  - no line holds a private-key marker.
t34_record_check() {
  local r=${1:-} e=${2:-} st=${3:-} n=0 k=0 tmp i name sha pinned p tok bf list v
  rc_bad() {
    k=$((k + 1))
    printf 'RECORD-CHECK BAD %s\n' "$*"
  }
  if ! list=$(paste_list "$st"); then
    printf 'RECORD-CHECK BAD no paste list for the stage %s\n' "$st"
    echo "RECORD-CHECK FAILED (1 of 1)"
    return 1
  fi
  if [ ! -f "$r" ]; then
    printf 'RECORD-CHECK BAD no record %s\n' "$r"
    echo "RECORD-CHECK FAILED (1 of 1)"
    return 1
  fi
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/t34-record.XXXXXX") || return 1
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
  for p in $list; do
    n=$((n + 1))
    awk -v p="$p" '$2 == p { f = 1 } END { exit !f }' "$tmp/names" || rc_bad "block $p: the paste list has it, the appendix does not"
  done
  # Tokens outside the appendix.
  awk '$0 == "## Appendix: raw output" { exit } { print }' "$r" \
    | grep -o '`amd64-t34-[A-Za-z0-9._-]*/[A-Za-z0-9._/-]*`' | tr -d '`' | sort -u > "$tmp/tokens" || true
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
    pinned=$(cat "$e/amd64-t34-evidence-commit.txt" 2> /dev/null || true)
    if [ -z "$sha" ] || ! git cat-file -e "$sha^{commit}" 2> /dev/null; then
      rc_bad "Evidence commit '${sha:-?}' does not resolve to a commit"
    elif [ -z "$pinned" ] || [ "$(git rev-parse "$sha^{commit}")" != "$(git rev-parse "$pinned^{commit}" 2> /dev/null || true)" ]; then
      rc_bad "Evidence commit $sha is not the pinned commit '$pinned' of $e/amd64-t34-evidence-commit.txt"
    else
      for bf in $BOUND_FILES; do
        n=$((n + 1))
        git diff --quiet "$sha" HEAD -- "$bf" || rc_bad "bound file $bf differs between $sha and HEAD"
      done
      n=$((n + 1))
      git diff --quiet "$sha" HEAD -- $BINARY_SOURCE || rc_bad "the binary's source ($BINARY_SOURCE) differs between $sha and HEAD (the binary is not the one of the evidence)"
    fi
  fi
  # The summary of a rehearsal is never evidence.
  n=$((n + 1))
  i=$(awk '$2 == "amd64-t34-summary.txt" { print $1; exit }' "$tmp/names")
  if [ -n "$i" ] && [ -f "$tmp/b$i" ] && grep -q '^REHEARSAL:' "$tmp/b$i"; then
    rc_bad "the pasted summary is a rehearsal's (REHEARSAL line)"
  fi
  # Nor is a failed set: the record's own verdicts read PASS, once each; no
  # verdict of the run reads anything else; the pasted status of the run is 0.
  # Prose claiming PASS over an appendix that shows FAIL is what this refuses.
  if [ -n "$i" ] && [ -f "$tmp/b$i" ]; then
    for v in $(record_verdicts "$st"); do
      n=$((n + 1))
      if [ "$(grep -c "^$v " "$tmp/b$i" || true)" != 1 ] || ! grep -qx "$v PASS" "$tmp/b$i"; then
        rc_bad "the pasted summary does not read '$v PASS' exactly once"
      fi
    done
    n=$((n + 1))
    if awk '/^(MIG|UPD)-0[0-9] / && !/^(MIG|UPD)-0[0-9] PASS$/ { f = 1 } END { exit !f }' "$tmp/b$i"; then
      rc_bad "the pasted summary holds a verdict that is not PASS: $(awk '/^(MIG|UPD)-0[0-9] / && !/^(MIG|UPD)-0[0-9] PASS$/' "$tmp/b$i" | tr '\n' ';' | sed 's/;$//')"
    fi
  else
    n=$((n + 1))
    rc_bad "no pasted amd64-t34-summary.txt to read the verdicts from"
  fi
  n=$((n + 1))
  i=$(awk '$2 == "amd64-t34-pass1.status" { print $1; exit }' "$tmp/names")
  printf '0\n' > "$tmp/zero"
  if [ -z "$i" ] || [ ! -f "$tmp/b$i" ] || ! cmp -s "$tmp/zero" "$tmp/b$i"; then
    rc_bad "the pasted amd64-t34-pass1.status is not exactly the one line 0: the run did not end well"
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

# record_verdicts STAGE: the verdict lines the record of STAGE stands on.
record_verdicts() {
  case "${1:-}" in
    mig) echo "MIG-01 MIG-02" ;;
    upd) echo "UPD-01" ;;
    rb) echo "UPD-02" ;;
    *) return 1 ;;
  esac
}

cmd_appendix() {
  local st=${1:-} rc=0
  paste_list "$st" > /dev/null || {
    say "upgrade-verify.sh: appendix STAGE: the stage must be mig, upd or rb (got '$st')"
    return 2
  }
  appendix_print "$st" "${E:-deploy/test/out}" || rc=$?
  return "$rc"
}
cmd_record_check() {
  local st rc=0
  st=$(record_stage "${R:-}") || {
    say "upgrade-verify.sh: record-check: R must name deploy/MIGRATION-RESTORE-RUN.md, deploy/UPDATE-CHAIN-RUN.md or deploy/ROLLBACK-RUN.md (got '${R:-}')"
    return 2
  }
  t34_record_check "$R" "${E:-deploy/test/out}" "$st" || rc=$?
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

selftest_names() {
  local n
  for n in holzcloud-t34-mig-src holzcloud-t34-mig-tgt holzcloud-t34-upd holzcloud-t34-rb; do
    st_ok "safe_name: $n" safe_name "$n"
  done
  for n in holzcloud-t32 holzcloud-t34 holzcloud-t34-mig holzcloud-t34-mig-srcx holzcloud-t34-rb2 \
    holzcloud-unit-test-amd64 holzcloud-t34-upd/x ""; do
    st_fail "safe_name: '$n'" safe_name "$n"
  done
  for n in amd64-t34-mig-src amd64-t34-mig-tgt amd64-t34-upd amd64-t34-rb; do
    st_ok "safe_out: $n" safe_out "deploy/test/out/$n"
  done
  for n in amd64-t34-evidence-commit.txt amd64-t34-summary.txt amd64-t34-pass1.started amd64-t34-pass1.finished \
    amd64-t34-pass1.status amd64-t34-pass1.log; do
    st_ok "safe_out: the run file $n" safe_out "deploy/test/out/$n"
  done
  for n in deploy/test/out/amd64-t32-src deploy/test/out/amd64-t34- deploy/test/out/amd64-t34-mig \
    deploy/test/out/amd64-t34-mig-src/x deploy/test/out/amd64-t34-../x deploy/test/out/amd64-t34-other \
    deploy/test/out /deploy/test/out/amd64-t34-upd ./deploy/test/out/amd64-t34-rb . ""; do
    st_fail "safe_out: '$n'" safe_out "$n"
  done
}

selftest_stages() {
  local s
  for s in "mig upd rb" "mig" "upd" "rb" "mig rb" "rb mig"; do
    st_ok "stages_valid: '$s'" stages_valid_of "$s"
  done
  for s in "" "mig bak01" "MIG" "migration" "mig upd rb all"; do
    st_fail "stages_valid: '$s'" stages_valid_of "$s"
  done
}

selftest_level() {
  st_ans "level_of_output: 60" 60 level_of_output 60
  st_ans "level_of_output: 69 with a carriage return" 69 level_of_output "$(printf '69\r')"
  st_ans "level_of_output: empty" ERROR level_of_output ""
  st_ans "level_of_output: text" ERROR level_of_output "Error: unable to open database"
  st_ans "level_of_output: two lines" ERROR level_of_output "$(printf '60\n61')"
  st_ans "level_of_output: a sign" ERROR level_of_output "-1"
  st_ans "level_of_output: a space inside" ERROR level_of_output "6 0"
}

# fx_set FILE KEY VALUE: the line "KEY: ..." of FILE replaced by "KEY: VALUE"; with
# VALUE "-" the line is removed.
fx_set() {
  awk -v k="$2" -v v="$3" 'index($0, k ": ") == 1 { if (v != "-") print k ": " v; next } { print }' "$1" > "$1.x"
  mv "$1.x" "$1"
}

# fx_mig FILE [KEY VALUE]: every key of mig.txt, all good; with KEY and VALUE, that
# one line replaced (or removed for "-").
fx_mig() {
  local f=$1
  {
    echo "OLD-RELEASE: v0.1.1 (5bcae9b)"
    echo "OLD-VERSION-LINE: holzcloud v0.1.1 (5bcae9b)"
    echo "NEW-RELEASE: v0.1.4-233-gcfce101 (cfce101)"
    echo "SEED-RC: 0"
    echo "BACKUP-RC: 0"
    echo "MANIFEST-VERSION: holzcloud v0.1.1 (5bcae9b)"
    echo "SUMS-IN-BACKUP: no"
    echo "LEVEL-BEFORE: 60"
    echo "LEVEL-EXPECTED-BEFORE: 60"
    echo "TRANSFER-RC: 0"
    echo "RESTORE-RC: 0"
    echo "RESTORE-NOTICE: yes"
    echo "RESTORE-COMPLETE: yes"
    echo "SNAPSHOTS-AFTER: 1"
    echo "SNAPSHOT-LEVEL: 60"
    echo "JOURNAL-SNAPSHOT-LINES: 1"
    echo "JOURNAL-APPLIED-LINES: 9"
    echo "JOURNAL-APPLIED-VERSIONS: 61 62 63 64 65 66 67 68 69"
    echo "LEVEL-AFTER: 69"
    echo "LEVEL-EXPECTED-AFTER: 69"
    echo "SETUP-AFTER: 404"
    echo "LOGIN-AFTER: 303 /admin/"
    echo "PAGE-AFTER: 200"
    echo "MEDIA-SHA-EQUAL: yes"
    echo "CHECK: 17 of 17"
    echo "ORDER: none (no order row)"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_mig01() {
  local f=$ST_TMP/mig.txt kv
  st_ans "verdict_mig01: no file" NOT-RUN verdict_mig01 "$ST_TMP/absent.txt"
  fx_mig "$f"
  st_ans "verdict_mig01: every key good" PASS verdict_mig01 "$f"
  fx_mig "$f" LOGIN-AFTER "303 /admin/2fa/einrichten"
  st_ans "verdict_mig01: a login sent on to the compulsory enrolment" PASS verdict_mig01 "$f"
  # One wrong (or absent, "-") value per case; each must give FAIL.
  for kv in "RESTORE-RC|1" "RESTORE-RC|3" "RESTORE-RC|-" "RESTORE-COMPLETE|no" "RESTORE-COMPLETE|-" \
    "LEVEL-BEFORE|61" "LEVEL-BEFORE|?" "LEVEL-BEFORE|-" "LEVEL-EXPECTED-BEFORE|-" \
    "LEVEL-AFTER|60" "LEVEL-AFTER|68" "LEVEL-AFTER|-" "LEVEL-EXPECTED-AFTER|70" "LEVEL-EXPECTED-AFTER|-" \
    "CHECK|16 of 17" "CHECK|17 of 18" "CHECK|" "CHECK|-" "SETUP-AFTER|200" "SETUP-AFTER|-" \
    "LOGIN-AFTER|200 -" "LOGIN-AFTER|303 /admin/login" "LOGIN-AFTER|403 -" "LOGIN-AFTER|-" \
    "PAGE-AFTER|404" "PAGE-AFTER|-" "MEDIA-SHA-EQUAL|no" "MEDIA-SHA-EQUAL|-" \
    "SNAPSHOTS-AFTER|0" "SNAPSHOTS-AFTER|2" "SNAPSHOTS-AFTER|-" "SNAPSHOT-LEVEL|69" "SNAPSHOT-LEVEL|?" \
    "SNAPSHOT-LEVEL|-"; do
    fx_mig "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_mig01: ${kv%%|*} '${kv#*|}'" FAIL verdict_mig01 "$f"
  done
  fx_mig "$f"
  echo "RESTORE-RC: 0" >> "$f"
  st_ans "verdict_mig01: a key twice" FAIL verdict_mig01 "$f"
  fx_mig "$f" LEVEL-BEFORE ""
  fx_set "$f" LEVEL-EXPECTED-BEFORE ""
  st_ans "verdict_mig01: two empty levels are no level" FAIL verdict_mig01 "$f"
}

selftest_verdict_mig02() {
  local f=$ST_TMP/mig.txt kv
  st_ans "verdict_mig02: no file" NOT-RUN verdict_mig02 "$ST_TMP/absent.txt"
  fx_mig "$f"
  st_ans "verdict_mig02: every key good" PASS verdict_mig02 "$f"
  for kv in "OLD-RELEASE|v0.1.1" "OLD-RELEASE|v0.1.2 (5bcae9b)" "OLD-RELEASE|v0.1.1 (zzz)" "OLD-RELEASE|-" \
    "NEW-RELEASE|cfce101" "NEW-RELEASE|v0.1.4-233-gcfce101 ()" "NEW-RELEASE|-" \
    "MANIFEST-VERSION|holzcloud v0.1.4 (abc1234)" "MANIFEST-VERSION|-" \
    "LEVEL-BEFORE|-" "LEVEL-BEFORE|x" "LEVEL-AFTER|-" "RESTORE-NOTICE|no" "RESTORE-NOTICE|-" \
    "RESTORE-COMPLETE|no" "RESTORE-COMPLETE|-" "JOURNAL-SNAPSHOT-LINES|0" "JOURNAL-SNAPSHOT-LINES|2" \
    "JOURNAL-SNAPSHOT-LINES|-" "JOURNAL-APPLIED-LINES|8" "JOURNAL-APPLIED-LINES|69" "JOURNAL-APPLIED-LINES|-" \
    "JOURNAL-APPLIED-VERSIONS|61 62 63 64 65 66 67 68" "JOURNAL-APPLIED-VERSIONS|62 63 64 65 66 67 68 69 70" \
    "JOURNAL-APPLIED-VERSIONS|61 62 63 65 64 66 67 68 69" "JOURNAL-APPLIED-VERSIONS|" "JOURNAL-APPLIED-VERSIONS|-"; do
    fx_mig "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_mig02: ${kv%%|*} '${kv#*|}'" FAIL verdict_mig02 "$f"
  done
  # No migration at all is no restore across one.
  fx_mig "$f" LEVEL-AFTER 60
  fx_set "$f" JOURNAL-APPLIED-LINES 0
  fx_set "$f" JOURNAL-APPLIED-VERSIONS ""
  st_ans "verdict_mig02: the same level before and after" FAIL verdict_mig02 "$f"
  # Another pair of levels with its own consecutive list passes.
  fx_mig "$f" LEVEL-BEFORE 67
  fx_set "$f" JOURNAL-APPLIED-LINES 2
  fx_set "$f" JOURNAL-APPLIED-VERSIONS "68 69"
  st_ans "verdict_mig02: levels 67 to 69" PASS verdict_mig02 "$f"
}

selftest_applied() {
  local j=$ST_TMP/journal.txt
  {
    echo '{"time":"2026-11-21T10:31:48.25Z","level":"INFO","msg":"pre-upgrade snapshot written","path":"/opt/holzcloud/data/pre-upgrade-v0.1.5-20261121-103148.sqlite"}'
    echo '{"time":"2026-11-21T10:31:48.33Z","level":"INFO","msg":"migration applied","version":61,"type":"sql","duration_ms":46}'
    echo '{"time":"2026-11-21T10:31:48.34Z","level":"INFO","msg":"migration applied","version":62,"type":"sql","duration_ms":0}'
    echo '{"time":"2026-11-21T10:31:48.35Z","level":"INFO","msg":"plugin loaded","plugin":"suche","version":"1.0.0"}'
    echo '{"time":"2026-11-21T10:31:48.36Z","level":"INFO","msg":"migration applied","version":63,"type":"go","duration_ms":4}'
    echo '2026/11/21 10:31:46 INFO migration applied version=64 type=sql duration_ms=7'
  } > "$j"
  st_ans "applied_versions: three JSON lines in order" "61 62 63" applied_versions "$j"
  : > "$j"
  st_ans "applied_versions: an empty journal" "" applied_versions "$j"
  st_ans "applied_versions: no file" ERROR applied_versions "$ST_TMP/absent.txt"
  echo '{"level":"INFO","msg":"migration applied","version":70,"type":"sql"}' > "$j"
  echo '{"level":"ERROR","msg":"migrations failed","err":"goose up: version 71"}' >> "$j"
  st_ans "applied_versions: a failure line is no applied migration" "70" applied_versions "$j"
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

# fx_scratch E STAGE: a scratch root holding every file of the paste list of
# STAGE, small texts.
fx_scratch() {
  local e=$1 st=$2 p
  rm -rf "${e:?}"
  mkdir -p "$e/amd64-t34-mig-src" "$e/amd64-t34-upd" "$e/amd64-t34-rb"
  for p in $(paste_list "$st"); do
    printf 'line one of %s\nKEY: value\n' "$p" > "$e/$p"
  done
  printf 'MIG-01 PASS\nMIG-02 PASS\nUPD-01 PASS\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  printf '0\n' > "$e/amd64-t34-pass1.status"
  git rev-parse --short HEAD > "$e/amd64-t34-evidence-commit.txt"
}

# fx_record E STAGE R [EVIDENCE-LINE]: a record whose appendix is `appendix` over E.
fx_record() {
  local e=$1 st=$2 r=$3 ev=${4:-}
  [ -n "$ev" ] || ev="Evidence commit: $(cat "$e/amd64-t34-evidence-commit.txt")"
  {
    echo "# A run"
    echo
    echo "$ev"
    echo
    echo "The summary is \`amd64-t34-summary.txt\`; the versions are in \`$(paste_list "$st" | sed -n 2p)\`."
    echo
    echo "## Appendix: raw output"
    echo
    appendix_print "$st" "$e"
  } > "$r"
}

selftest_record() {
  local e=$ST_TMP/e r=$ST_TMP/RUN.md out marker st
  fx_scratch "$e" mig
  fx_record "$e" mig "$r"
  out=$(t34_record_check "$r" "$e" mig 2>&1) || true
  st_eq "record-check: a record equal to its files" 0 "$(rc_of t34_record_check "$r" "$e" mig)"
  st_has "record-check: the OK line" "RECORD-CHECK OK (" "$out"

  # One byte of one block differs from its file.
  fx_record "$e" mig "$r"
  awk '{ if ($0 == "line one of amd64-t34-mig-src/mig-journal.txt") print "line one of amd64-t34-mig-src/mig-journal.txT"; else print }' "$r" > "$r.x" && mv "$r.x" "$r"
  out=$(t34_record_check "$r" "$e" mig 2>&1) || true
  st_eq "record-check: a block differs by one byte" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  st_has "record-check: the differing block is named" "amd64-t34-mig-src/mig-journal.txt" "$out"

  # A DIRECTORY/FILE token outside the appendix without a block.
  fx_record "$e" mig "$r"
  awk '{ print } /^Evidence commit: / { print ""; print "See `amd64-t34-mig-src/notpasted.txt`." }' "$r" > "$r.x" && mv "$r.x" "$r"
  out=$(t34_record_check "$r" "$e" mig 2>&1) || true
  st_eq "record-check: a cited file without a block" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  st_has "record-check: the cited file is named" "amd64-t34-mig-src/notpasted.txt" "$out"

  # Two Evidence commit lines.
  fx_record "$e" mig "$r"
  awk '{ print } /^Evidence commit: / { print $0 }' "$r" > "$r.x" && mv "$r.x" "$r"
  st_eq "record-check: two Evidence commit lines" 1 "$(rc_of t34_record_check "$r" "$e" mig)"

  # No Evidence commit line.
  fx_record "$e" mig "$r"
  grep -v '^Evidence commit: ' "$r" > "$r.x" && mv "$r.x" "$r"
  st_eq "record-check: no Evidence commit line" 1 "$(rc_of t34_record_check "$r" "$e" mig)"

  # An Evidence commit that does not resolve.
  printf '0000000\n' > "$e/amd64-t34-evidence-commit.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: an Evidence commit that does not resolve" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  fx_scratch "$e" mig

  # An Evidence commit that resolves but is not the pinned one.
  fx_record "$e" mig "$r" "Evidence commit: $(git rev-parse --short 'HEAD~1')"
  st_eq "record-check: an Evidence commit other than the pinned one" 1 "$(rc_of t34_record_check "$r" "$e" mig)"

  # A private-key marker anywhere (built from two halves, so this file holds none).
  marker="-----BEGIN OPENSSH PRIV""ATE KEY-----"
  fx_record "$e" mig "$r"
  printf '%s\n' "$marker" >> "$r"
  st_eq "record-check: a private-key marker" 1 "$(rc_of t34_record_check "$r" "$e" mig)"

  # A rehearsal's summary is never evidence.
  printf 'MIG-01 PASS\nMIG-02 PASS\nREHEARSAL: yes\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a rehearsal's summary" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  fx_scratch "$e" mig

  # A failed set is never evidence either: the record's own verdicts, any other
  # verdict of the run, and the run's status (WR-02 of the phase 34 review).
  printf 'MIG-01 FAIL\nMIG-02 PASS\nUPD-01 PASS\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  out=$(t34_record_check "$r" "$e" mig 2>&1) || true
  st_eq "record-check: a FAIL summary (MIG-01 of a mig record)" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  st_has "record-check: the failed verdict is named" "MIG-01" "$out"
  printf 'MIG-01 PASS\nMIG-02 FAIL\nUPD-01 PASS\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a FAIL summary (MIG-02 of a mig record)" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  printf 'MIG-01 PASS\nMIG-02 PASS\nUPD-01 PASS\nUPD-02 FAIL\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a FAIL summary (UPD-02 of the same run, under a mig record)" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  printf 'MIG-01 PASS\nMIG-02 PASS\nUPD-01 NOT-RUN\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a NOT-RUN verdict in the summary" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  printf 'MIG-01 PASS\nUPD-01 PASS\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a summary without the record's verdict line" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  printf 'MIG-01 PASS\nMIG-01 PASS\nMIG-02 PASS\nUPD-01 PASS\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" mig "$r"
  st_eq "record-check: a summary with the record's verdict line twice" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  fx_scratch "$e" mig
  printf '1\n' > "$e/amd64-t34-pass1.status"
  fx_record "$e" mig "$r"
  out=$(t34_record_check "$r" "$e" mig 2>&1) || true
  st_eq "record-check: status 1" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  st_has "record-check: the status is named" "amd64-t34-pass1.status" "$out"
  printf '0\n0\n' > "$e/amd64-t34-pass1.status"
  fx_record "$e" mig "$r"
  st_eq "record-check: a status of two lines" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  : > "$e/amd64-t34-pass1.status"
  fx_record "$e" mig "$r"
  st_eq "record-check: an empty status" 1 "$(rc_of t34_record_check "$r" "$e" mig)"
  fx_scratch "$e" upd
  printf 'MIG-01 PASS\nMIG-02 PASS\nUPD-01 FAIL\nUPD-02 PASS\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" upd "$r"
  st_eq "record-check: a FAIL summary (UPD-01 of an upd record)" 1 "$(rc_of t34_record_check "$r" "$e" upd)"
  fx_scratch "$e" rb
  printf 'MIG-01 PASS\nMIG-02 PASS\nUPD-01 PASS\nUPD-02 FAIL\nSTAGES: mig upd rb\n' > "$e/amd64-t34-summary.txt"
  fx_record "$e" rb "$r"
  st_eq "record-check: a FAIL summary (UPD-02 of an rb record)" 1 "$(rc_of t34_record_check "$r" "$e" rb)"
  fx_scratch "$e" rb
  printf '1\n' > "$e/amd64-t34-pass1.status"
  fx_record "$e" rb "$r"
  st_eq "record-check: status 1 under an rb record" 1 "$(rc_of t34_record_check "$r" "$e" rb)"
  fx_scratch "$e" mig

  # A block missing for a file of the paste list.
  fx_record "$e" mig "$r"
  awk '/^#### amd64-t34-mig-src\/mig-check.txt$/ { skip = 1 } skip && /^```$/ { skip = 0; next } !skip { print }' "$r" > "$r.x" && mv "$r.x" "$r"
  st_eq "record-check: a file of the paste list without a block" 1 "$(rc_of t34_record_check "$r" "$e" mig)"

  # The record of one stage checked against the paste list of another.
  fx_record "$e" mig "$r"
  st_eq "record-check: a mig record against the upd paste list" 1 "$(rc_of t34_record_check "$r" "$e" upd)"

  # No record at all; no stage.
  st_eq "record-check: no record" 1 "$(rc_of t34_record_check "$ST_TMP/absent.md" "$e" mig)"
  st_eq "record-check: no stage" 1 "$(rc_of t34_record_check "$r" "$e" "")"

  # The upd and rb records pass on their own paste lists.
  for st in upd rb; do
    fx_scratch "$e" "$st"
    fx_record "$e" "$st" "$r"
    st_eq "record-check: a $st record equal to its files" 0 "$(rc_of t34_record_check "$r" "$e" "$st")"
  done

  # R names the record; the paste list follows from its name; any other file refuses.
  st_ans "record_stage: MIGRATION-RESTORE-RUN.md" mig record_stage deploy/MIGRATION-RESTORE-RUN.md
  st_ans "record_stage: UPDATE-CHAIN-RUN.md" upd record_stage deploy/UPDATE-CHAIN-RUN.md
  st_ans "record_stage: ROLLBACK-RUN.md in another checkout" rb record_stage /x/holzcloud-cms/deploy/ROLLBACK-RUN.md
  for st in deploy/BACKUP-VERIFY-RUN.md deploy/MIGRATION-RESTORE-RUN.mdx MIGRATION-RESTORE-RUN.md docs/ROLLBACK-RUN.md ""; do
    st_ans "record_stage: '$st'" ERROR record_stage "$st"
  done
  st_eq "record-check: R outside the three records" 2 "$(R=deploy/BACKUP-VERIFY-RUN.md rc_of cmd_record_check)"
  st_eq "record-check: no R" 2 "$(R='' rc_of cmd_record_check)"
}

selftest_appendix() {
  local e=$ST_TMP/e st
  for st in mig upd rb; do
    fx_scratch "$e" "$st"
    st_eq "appendix $st: every file pasteable" 0 "$(rc_of appendix_print "$st" "$e")"
  done
  fx_scratch "$e" mig
  st_has "appendix: the first block" '#### amd64-t34-summary.txt' "$(appendix_print mig "$e")"
  st_eq "appendix: the mig list has ten files" 10 "$(paste_list mig | wc -l | tr -d ' ')"
  st_eq "appendix: the upd list has fourteen files" 14 "$(paste_list upd | wc -l | tr -d ' ')"
  st_eq "appendix: the rb list has twelve files" 12 "$(paste_list rb | wc -l | tr -d ' ')"
  printf 'x\n```\n' >> "$e/amd64-t34-mig-src/mig.txt"
  st_eq "appendix: a line beginning with three backticks" 1 "$(rc_of appendix_print mig "$e")"
  fx_scratch "$e" mig
  printf '#### x\n' >> "$e/amd64-t34-mig-src/mig-restore.out"
  st_eq "appendix: a line beginning with '#### '" 1 "$(rc_of appendix_print mig "$e")"
  fx_scratch "$e" mig
  printf '## x\n' >> "$e/amd64-t34-mig-src/mig-journal.txt"
  st_eq "appendix: a line beginning with '## '" 1 "$(rc_of appendix_print mig "$e")"
  fx_scratch "$e" mig
  rm -f "$e/amd64-t34-mig-src/mig-check.txt"
  st_eq "appendix: a file of the list is missing" 1 "$(rc_of appendix_print mig "$e")"
  st_eq "appendix: an unknown stage" 2 "$(rc_of cmd_appendix bak01)"
  st_eq "appendix: no stage" 2 "$(rc_of cmd_appendix "")"
}

# fx_doc FILE: a document with two ## sections, bash and text blocks, a ###
# subsection, a comment and a continued line.
fx_doc() {
  {
    echo '# Title'
    echo
    echo '## Updating'
    echo
    echo 'Text.'
    echo
    echo '```bash'
    echo 'sudo cp a b'
    echo '# a comment'
    echo
    echo '/opt/x version \'
    echo '  --flag'
    echo '```'
    echo
    echo '```bash'
    echo 'scp holzcloud user@your-server:'
    echo '```'
    echo
    echo '```text'
    echo 'not bash'
    echo '```'
    echo
    echo '### Sub'
    echo
    echo '```bash'
    echo 'third'
    echo '```'
    echo
    echo '## Rolling back'
    echo
    echo '```bash'
    echo 'only one'
    echo '```'
    echo
    echo '## Updating later'
    echo
    echo '```bash'
    echo 'not this one'
    echo '```'
  } > "$1"
}

selftest_doc_block() {
  local d=$ST_TMP/doc.md
  fx_doc "$d"
  st_ans "doc_block: Updating block 1, comment dropped, continuation joined" "$(printf 'sudo cp a b\n/opt/x version --flag')" doc_block "$d" Updating 1
  st_ans "doc_block: Updating block 2" "scp holzcloud user@your-server:" doc_block "$d" Updating 2
  st_ans "doc_block: a text block is no bash block, a ### subsection belongs to the section" "third" doc_block "$d" Updating 3
  st_ans "doc_block: Updating has no block 4" ERROR doc_block "$d" Updating 4
  st_ans "doc_block: Rolling back block 1" "only one" doc_block "$d" "Rolling back" 1
  st_ans "doc_block: Rolling back has no block 2" ERROR doc_block "$d" "Rolling back" 2
  st_ans "doc_block: no such section" ERROR doc_block "$d" Recovery 1
  st_ans "doc_block: a heading that only begins with the name" ERROR doc_block "$d" Updat 1
  st_ans "doc_block: block 0" ERROR doc_block "$d" Updating 0
  st_ans "doc_block: block x" ERROR doc_block "$d" Updating x
  st_ans "doc_block: no document" ERROR doc_block "$ST_TMP/absent.md" Updating 1
  # The real document: the blocks the stages play exist and have the expected shape.
  st_eq "doc_block: DEPLOY.md Build block 1 is one go build line" "1 yes" \
    "$(doc_block "$DEPLOY_DOC" Build 1 | wc -l | tr -d ' ') $(doc_block "$DEPLOY_DOC" Build 1 | grep -q 'go build' && echo yes)"
  st_ok "doc_block: DEPLOY.md Updating block 3" doc_block "$DEPLOY_DOC" Updating 3
  st_has "doc_block: DEPLOY.md Rolling back block 1 resets the start limit" "sudo systemctl reset-failed holzcloud" "$(doc_block "$DEPLOY_DOC" "Rolling back" 1)"
}

selftest_play_kind() {
  local l
  st_ans "play_kind: the binary to the home directory" scp-binary play_kind "scp holzcloud user@your-server:"
  st_ans "play_kind: the binary to a directory" scp-binary play_kind "scp holzcloud user@your-server:/home/u/"
  st_ans "play_kind: two files of deploy/" scp-files play_kind "scp deploy/backup.sh deploy/restore.sh user@your-server:"
  st_ans "play_kind: the Build line" build play_kind 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud'
  st_ans "play_kind: a server line" run play_kind "sudo systemctl stop holzcloud"
  st_ans "play_kind: a pipeline" run play_kind "curl -fsS http://127.0.0.1:8080/readyz | jq '{status}'"
  for l in "scp -r deploy/ user@your-server:/opt/holzcloud/" "scp deploy/test/run.sh user@x:" "scp holzcloud" \
    "scp holzcloud.prev user@x:" "scp /etc/passwd user@x:" "scp deploy/ user@x:" "scp holzcloud user@x" \
    "sudo cp <snapshot> /opt/holzcloud/data/holzcloud.sqlite" ""; do
    st_ans "play_kind: refuses '$l'" ERROR play_kind "$l"
  done
}

selftest_restore_with() {
  local j=$ST_TMP/rw.txt
  {
    echo '{"time":"t","level":"INFO","msg":"pre-upgrade snapshot written","path":"/d/pre-upgrade-f-20261121-120000.sqlite","level":69}'
    echo '{"time":"t","level":"ERROR","msg":"restore with: systemctl stop holzcloud && cp /d/pre-upgrade-f-20261121-120000.sqlite /d/holzcloud.sqlite && systemctl start holzcloud"}'
    echo '{"time":"t","level":"WARN","msg":"pre-upgrade snapshot kept","path":"/d/pre-upgrade-f-20261121-120000.sqlite","level":69,"db_level":70}'
    echo '{"time":"t","level":"ERROR","msg":"restore with: systemctl stop holzcloud && cp /d/pre-upgrade-f-20261121-120000.sqlite /d/holzcloud.sqlite && systemctl start holzcloud"}'
    echo '{"time":"t","level":"ERROR","msg":"restore with: systemctl stop holzcloud && cp /d/pre-upgrade-f-20261121-120007.sqlite /d/holzcloud.sqlite && systemctl start holzcloud"}'
    echo 'restore with: systemctl stop holzcloud && cp /d/not-json.sqlite /d/holzcloud.sqlite'
  } > "$j"
  st_ans "restore_with_paths: distinct paths in order, an escaped && too, a line that is no JSON ignored" \
    "$(printf '/d/pre-upgrade-f-20261121-120000.sqlite\n/d/pre-upgrade-f-20261121-120007.sqlite')" restore_with_paths "$j"
  : > "$j"
  st_ans "restore_with_paths: none" "" restore_with_paths "$j"
  st_ans "restore_with_paths: no file" ERROR restore_with_paths "$ST_TMP/absent.txt"
}

selftest_json() {
  local l='{"time":"t","level":"ERROR","msg":"refusing to start: the database schema is newer than this binary","db_level":70,"binary_level":69,"err":"the database is at level 70: see \"Rolling back\""}'
  st_ans "json_num: db_level" 70 json_num "$l" db_level
  st_ans "json_num: binary_level" 69 json_num "$l" binary_level
  st_ans "json_num: level is a text here" "" json_num "$l" level
  st_ans "json_err: the err of the line, escapes kept" 'the database is at level 70: see \"Rolling back\"' json_err "$l"
  st_ans "json_err: no err" "" json_err '{"msg":"x"}'
  st_ans "json_str: msg" "refusing to start: the database schema is newer than this binary" json_str "$l" msg
  st_ans "json_str: a path" "/d/x.sqlite" json_str '{"msg":"pre-upgrade snapshot written","path":"/d/x.sqlite","level":69}' path
}

selftest_guard_sentence() {
  local g=$ST_TMP/guard.go s
  {
    echo 'func (e *NewerSchemaError) Error() string {'
    echo '	return fmt.Sprintf("level %d, up to %d: "+'
    echo '		"see \"Rolling back\" "+'
    echo '		"and stop",'
    echo '		e.DBLevel, e.BinaryLevel)'
    echo '}'
  } > "$g"
  st_ans "guard_sentence: the pieces joined, the levels put in" 'level 70, up to 69: see "Rolling back" and stop' guard_sentence "$g" 70 69
  st_ans "guard_sentence: no file" ERROR guard_sentence "$ST_TMP/absent.go" 70 69
  printf 'package db\n' > "$g"
  st_ans "guard_sentence: no Sprintf" ERROR guard_sentence "$g" 70 69
  s=$(guard_sentence internal/db/schema_guard.go 70 69) || s=""
  st_has "guard_sentence: schema_guard.go, its start" "the database is at migration level 70, and this binary knows migrations up to 69: a newer holzcloud has migrated it" "$s"
  st_has "guard_sentence: schema_guard.go, its end" 'as deploy/DEPLOY.md describes under "Rolling back"' "$s"
  st_ans "json_escape: quotes and backslashes" 'a \"b\" c\\d' json_escape 'a "b" c\d'
}

selftest_compare_state() {
  local a=$ST_TMP/s0 b=$ST_TMP/s2 out
  printf 'schema: 69\ntables: a b\nrows pages: 1\n' > "$a"
  cp "$a" "$b"
  st_eq "compare_state: equal" "bad=0" "$(compare_state "$a" "$b" | tail -n 1)"
  printf 'schema: 69\ntables: a b t34_ok\nrows pages: 1\n' > "$b"
  out=$(compare_state "$a" "$b")
  st_eq "compare_state: one line differs" "bad=1" "$(printf '%s\n' "$out" | tail -n 1)"
  st_has "compare_state: the differing line is named" "tables: a b t34_ok" "$out"
  printf 'schema: 69\ntables: a b\n' > "$b"
  st_eq "compare_state: a line missing" "bad=1" "$(compare_state "$a" "$b" | tail -n 1)"
  printf 'schema: 69\ntables: a b\nrows pages: 1\nrows users: 2\n' > "$b"
  st_eq "compare_state: a line more" "bad=1" "$(compare_state "$a" "$b" | tail -n 1)"
  : > "$a"
  : > "$b"
  st_eq "compare_state: two empty states are no state" "bad=1" "$(compare_state "$a" "$b" | tail -n 1)"
}

# fx_rb FILE [KEY VALUE]: every key of rb.txt, all good; with KEY and VALUE, that
# one line replaced (or removed for "-").
fx_rb() {
  local f=$1
  {
    echo "B0-VERSION: v0.1.4-245-ga44208a"
    echo "LEVEL-EXPECTED-B0: 69"
    echo "LEVEL-EXPECTED-F: 71"
    echo "F-BUILD: v0.1.4-246-g1234567 (1234567) migrations 71 sha256 abc"
    echo "START-WINDOW-WAIT-S: 120"
    echo "UPDATE-CHECK-RC: 7"
    echo "UPDATE-CHECK-SNAPSHOT-LINE: yes"
    echo "STARTS: 5"
    echo "NRESTARTS: 4"
    echo "RESULT: start-limit-hit"
    echo "ACTIVE-STATE: failed"
    echo "JOURNAL-SNAPSHOT-WRITTEN: 1"
    echo "JOURNAL-SNAPSHOT-KEPT: 4"
    echo "JOURNAL-MIGRATIONS-FAILED: 5"
    echo "JOURNAL-RESTORE-WITH: 5"
    echo "RESTORE-WITH-DISTINCT-PATHS: 1"
    echo "RESTORE-WITH-NAMES-WRITTEN: yes"
    echo "STOP-MESSAGE: goose up: partial migration error (type:sql,version:71): SQL logic error: no such table: t34_no_such_table (1)"
    echo "APPLIED-BEFORE-FAILURE: 70"
    echo "START-LIMIT-LINE: yes"
    echo "LEVEL-HALF: 70"
    echo "HEALTHZ-DURING-LOOP: never"
    echo "WRITTEN-PATH: /opt/holzcloud/data/pre-upgrade-b-20261121-120000.sqlite"
    echo "WRITTEN-LEVEL: 69"
    echo "SNAPSHOTS-AFTER-LOOP: pre-upgrade-a-20261121-110000.sqlite:0 pre-upgrade-b-20261121-120000.sqlite:69"
    echo "GUARD-HEALTHZ: never"
    echo "GUARD-ACTIVE-STATE: failed"
    echo "GUARD-LINE: yes"
    echo "GUARD-DB-LEVEL: 70"
    echo "GUARD-BINARY-LEVEL: 69"
    echo "GUARD-SENTENCE: yes"
    echo "ROLLBACK-RC: 0 0 0 0 0 0 0 0 0 0 0"
    echo 'ROLLBACK-READYZ: {"status":"ok","version":"v0.1.4-245-ga44208a","commit":"a44208a","integrity":"ok"}'
    echo "ROLLBACK-CHECK-LINE: no migration, no snapshot"
    echo "LEVEL-AFTER-ROLLBACK: 69"
    echo "T34-OK-TABLE: absent"
    echo "AFTER-UPDATE-LEVEL: 70"
    echo "MARKER-AFTER: absent"
    echo "COMPARE-S0-S2: bad=0"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_upd02() {
  local f=$ST_TMP/rb.txt kv
  st_ans "verdict_upd02: no file" NOT-RUN verdict_upd02 "$ST_TMP/absent.txt"
  fx_rb "$f"
  st_ans "verdict_upd02: every key good" PASS verdict_upd02 "$f"
  for kv in "B0-VERSION|" "B0-VERSION|-" "LEVEL-EXPECTED-B0|-" "LEVEL-EXPECTED-B0|x" "LEVEL-EXPECTED-F|72" "LEVEL-EXPECTED-F|-" \
    "UPDATE-CHECK-RC|0" "UPDATE-CHECK-RC|-" "UPDATE-CHECK-SNAPSHOT-LINE|no" "UPDATE-CHECK-SNAPSHOT-LINE|-" "STARTS|1" "STARTS|4" "STARTS|-" \
    "JOURNAL-SNAPSHOT-WRITTEN|0" "JOURNAL-SNAPSHOT-WRITTEN|2" "JOURNAL-SNAPSHOT-WRITTEN|-" \
    "JOURNAL-SNAPSHOT-KEPT|5" "JOURNAL-SNAPSHOT-KEPT|3" "JOURNAL-SNAPSHOT-KEPT|-" \
    "JOURNAL-MIGRATIONS-FAILED|4" "JOURNAL-MIGRATIONS-FAILED|-" "JOURNAL-RESTORE-WITH|4" "JOURNAL-RESTORE-WITH|-" \
    "RESTORE-WITH-DISTINCT-PATHS|0" "RESTORE-WITH-DISTINCT-PATHS|2" "RESTORE-WITH-DISTINCT-PATHS|-" \
    "RESTORE-WITH-NAMES-WRITTEN|no" "RESTORE-WITH-NAMES-WRITTEN|-" \
    "STOP-MESSAGE|goose up: partial migration error (type:sql,version:70): x" "STOP-MESSAGE|goose up: version:71) failed" "STOP-MESSAGE|-" \
    "APPLIED-BEFORE-FAILURE|" "APPLIED-BEFORE-FAILURE|70 71" "APPLIED-BEFORE-FAILURE|-" \
    "LEVEL-HALF|69" "LEVEL-HALF|71" "LEVEL-HALF|?" "LEVEL-HALF|-" \
    "GUARD-LINE|no" "GUARD-LINE|-" "GUARD-DB-LEVEL|71" "GUARD-DB-LEVEL|-" "GUARD-BINARY-LEVEL|70" "GUARD-BINARY-LEVEL|-" \
    "GUARD-SENTENCE|no" "GUARD-SENTENCE|-" \
    "ROLLBACK-RC|0 0 1 0" "ROLLBACK-RC|" "ROLLBACK-RC|0 x" "ROLLBACK-RC|-" \
    'ROLLBACK-READYZ|{"status":"ok","version":"v0.1.4-246-g1234567","commit":"1234567","integrity":"ok"}' \
    'ROLLBACK-READYZ|{"status":"degraded","version":"v0.1.4-245-ga44208a","commit":"a44208a","integrity":"ok"}' \
    'ROLLBACK-READYZ|{"status":"ok","version":"v0.1.4-245-ga44208a","commit":"a44208a","integrity":"failed"}' \
    'ROLLBACK-READYZ|{"status":"ok","version":"v0.1.4-245-ga44208a-dirty","commit":"a44208a","integrity":"ok"}' \
    "ROLLBACK-READYZ|" "ROLLBACK-READYZ|-" \
    "ROLLBACK-CHECK-LINE|pre-upgrade snapshot written" "ROLLBACK-CHECK-LINE|-" \
    "LEVEL-AFTER-ROLLBACK|70" "LEVEL-AFTER-ROLLBACK|-" "T34-OK-TABLE|present" "T34-OK-TABLE|-" \
    "AFTER-UPDATE-LEVEL|69" "AFTER-UPDATE-LEVEL|71" "AFTER-UPDATE-LEVEL|-" "MARKER-AFTER|present" "MARKER-AFTER|-" \
    "COMPARE-S0-S2|bad=1" "COMPARE-S0-S2|bad=" "COMPARE-S0-S2|-" \
    "ACTIVE-STATE|activating" "ACTIVE-STATE|active" "ACTIVE-STATE|" "ACTIVE-STATE|-" \
    "START-LIMIT-LINE|no" "START-LIMIT-LINE|" "START-LIMIT-LINE|-" \
    "HEALTHZ-DURING-LOOP|answered" "HEALTHZ-DURING-LOOP|" "HEALTHZ-DURING-LOOP|-" \
    "GUARD-HEALTHZ|answered" "GUARD-HEALTHZ|" "GUARD-HEALTHZ|-" \
    "GUARD-ACTIVE-STATE|activating" "GUARD-ACTIVE-STATE|" "GUARD-ACTIVE-STATE|-" \
    "WRITTEN-LEVEL|70" "WRITTEN-LEVEL|" "WRITTEN-LEVEL|-" \
    "WRITTEN-PATH|/opt/holzcloud/data/pre-upgrade-c-20261121-130000.sqlite" "WRITTEN-PATH|" "WRITTEN-PATH|-" \
    "SNAPSHOTS-AFTER-LOOP|pre-upgrade-a-20261121-110000.sqlite:0 pre-upgrade-b-20261121-120000.sqlite:69 pre-upgrade-c-20261121-130000.sqlite:70" \
    "SNAPSHOTS-AFTER-LOOP|pre-upgrade-a-20261121-110000.sqlite:0 pre-upgrade-b-20261121-120000.sqlite:69 pre-upgrade-c-20261121-130000.sqlite:69" \
    "SNAPSHOTS-AFTER-LOOP|pre-upgrade-a-20261121-110000.sqlite:0 pre-upgrade-c-20261121-130000.sqlite:69" \
    "SNAPSHOTS-AFTER-LOOP|pre-upgrade-a-20261121-110000.sqlite:0" "SNAPSHOTS-AFTER-LOOP|" "SNAPSHOTS-AFTER-LOOP|-"; do
    fx_rb "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_upd02: ${kv%%|*} '${kv#*|}'" FAIL verdict_upd02 "$f"
  done
  fx_rb "$f"
  echo "GUARD-LINE: yes" >> "$f"
  st_ans "verdict_upd02: a key twice" FAIL verdict_upd02 "$f"
  # A shorter loop with counts that agree passes: the verdict reads the counts, not a typed five.
  fx_rb "$f" STARTS 3
  fx_set "$f" JOURNAL-SNAPSHOT-KEPT 2
  fx_set "$f" JOURNAL-MIGRATIONS-FAILED 3
  fx_set "$f" JOURNAL-RESTORE-WITH 3
  st_ans "verdict_upd02: three starts that agree" PASS verdict_upd02 "$f"
  # Other levels, all from the counts: B0 at 67, F at 69.
  fx_rb "$f" LEVEL-EXPECTED-B0 67
  fx_set "$f" LEVEL-EXPECTED-F 69
  fx_set "$f" STOP-MESSAGE "goose up: partial migration error (type:sql,version:69): x"
  fx_set "$f" APPLIED-BEFORE-FAILURE 68
  fx_set "$f" LEVEL-HALF 68
  fx_set "$f" GUARD-DB-LEVEL 68
  fx_set "$f" GUARD-BINARY-LEVEL 67
  fx_set "$f" LEVEL-AFTER-ROLLBACK 67
  fx_set "$f" AFTER-UPDATE-LEVEL 68
  fx_set "$f" WRITTEN-LEVEL 67
  fx_set "$f" SNAPSHOTS-AFTER-LOOP "pre-upgrade-a-20261121-110000.sqlite:0 pre-upgrade-b-20261121-120000.sqlite:67"
  st_ans "verdict_upd02: levels 67 and 69" PASS verdict_upd02 "$f"
  fx_set "$f" GUARD-BINARY-LEVEL 69
  st_ans "verdict_upd02: levels 67 and 69, a typed 69 for the binary" FAIL verdict_upd02 "$f"
}

selftest_prune_expect() {
  local e0=pre-upgrade-v0.1.4-245-ga44208a-20261121-120000.sqlite w k exp
  local a1=pre-upgrade-v0.1.4-246-g1111111-20261121-120100.sqlite
  local a2=pre-upgrade-v0.1.4-247-g2222222-20261121-120200.sqlite
  local a3=pre-upgrade-v0.1.4-248-g3333333-20261121-120300.sqlite
  local a4=pre-upgrade-v0.1.4-249-g4444444-20261121-120400.sqlite
  local a5=pre-upgrade-v0.1.4-250-g5555555-20261121-120500.sqlite
  local a6=pre-upgrade-v0.1.4-251-g6666666-20261121-120600.sqlite
  w="$a1 $a2 $a3 $a4 $a5 $a6"
  # One initial file (the first start of the install), the table of 34-RESEARCH.md Pattern 3.
  st_ans "prune_expect: one initial, K=1" "$e0 $a1" prune_expect "$e0" "$w" 1
  st_ans "prune_expect: one initial, K=2" "$e0 $a1 $a2" prune_expect "$e0" "$w" 2
  st_ans "prune_expect: one initial, K=3" "$e0 $a1 $a2 $a3" prune_expect "$e0" "$w" 3
  st_ans "prune_expect: one initial, K=4" "$e0 $a1 $a2 $a3 $a4" prune_expect "$e0" "$w" 4
  st_ans "prune_expect: one initial, K=5, the initial one removed" "$a1 $a2 $a3 $a4 $a5" prune_expect "$e0" "$w" 5
  st_ans "prune_expect: one initial, K=6, the first update's removed" "$a2 $a3 $a4 $a5 $a6" prune_expect "$e0" "$w" 6
  # No initial file.
  st_ans "prune_expect: none initial, K=1" "$a1" prune_expect "" "$w" 1
  st_ans "prune_expect: none initial, K=5" "$a1 $a2 $a3 $a4 $a5" prune_expect "" "$w" 5
  st_ans "prune_expect: none initial, K=6" "$a2 $a3 $a4 $a5 $a6" prune_expect "" "$w" 6
  # Two initial files, given in name order, not in time order.
  local e1=pre-upgrade-dev-20261121-115900.sqlite
  st_ans "prune_expect: two initial, K=1, by time" "$e1 $e0 $a1" prune_expect "$e0 $e1" "$w" 1
  st_ans "prune_expect: two initial, K=3" "$e1 $e0 $a1 $a2 $a3" prune_expect "$e1 $e0" "$w" 3
  st_ans "prune_expect: two initial, K=4, the older initial removed" "$e0 $a1 $a2 $a3 $a4" prune_expect "$e1 $e0" "$w" 4
  st_ans "prune_expect: two initial, K=5" "$a1 $a2 $a3 $a4 $a5" prune_expect "$e1 $e0" "$w" 5
  st_ans "prune_expect: two initial, K=6" "$a2 $a3 $a4 $a5 $a6" prune_expect "$e1 $e0" "$w" 6
  # Version strings that sort against their time: v0.1.10 is written after v0.1.9.
  local v9=pre-upgrade-v0.1.9-20261121-130000.sqlite v10=pre-upgrade-v0.1.10-20261121-130100.sqlite
  st_ans "prune_expect: v0.1.10 after v0.1.9 by time, not before by name" "$a3 $a4 $a5 $v9 $v10" \
    prune_expect "$a1 $a2 $a3 $a4 $a5" "$v9 $v10" 2
  st_ans "by_stamp: v0.1.10 written after v0.1.9" "$v9 $v10" by_stamp "$v10 $v9"
  st_ans "by_stamp: a name without a stamp is the oldest" "pre-upgrade-x.sqlite $a1" by_stamp "$a1 pre-upgrade-x.sqlite"
  st_ans "by_stamp: none" "" by_stamp ""
  st_ans "snap_stamp: a name" 20261121-120100 snap_stamp "$a1"
  st_ans "snap_stamp: no stamp" 00000000-000000 snap_stamp pre-upgrade-x.sqlite
  st_ans "prune_expect: fewer written than K" ERROR prune_expect "$e0" "$a1 $a2" 3
  st_ans "prune_expect: K=0" ERROR prune_expect "$e0" "$w" 0
  st_ans "prune_expect: K=x" ERROR prune_expect "$e0" "$w" x
  st_ans "removed_between: one left" "$e0" removed_between "$e0 $a1 $a2 $a3 $a4" "$a1 $a2 $a3 $a4 $a5"
  st_ans "removed_between: none left" "" removed_between "$e0 $a1" "$e0 $a1 $a2"
  st_ans "removed_between: two left, in the order before" "$e1 $e0" removed_between "$e1 $e0 $a1" "$a1 $a2"
  st_ans "removed_between: a name that is a prefix of another is not found in it" "$a1" removed_between "$a1" "$a1.x"
}

# fx_upd FILE [KEY VALUE]: every key of upd.txt, all good; with KEY and VALUE, that
# one line replaced (or removed for "-").
fx_upd() {
  local f=$1 k
  {
    echo "LEVEL-EXPECTED-B0: 69"
    echo "SNAPSHOTS-AFTER-0: e0"
    for k in 1 2 3 4 5 6; do
      echo "BUILD-$k: v0.1.4-24$k-g$k$k$k$k$k$k$k ($k$k$k$k$k$k$k) migrations $((69 + k))"
      echo "READYZ-$k: v0.1.4-24$k-g$k$k$k$k$k$k$k"
      echo "WRITTEN-$k: pre-upgrade-v0.1.4-24$k-g$k$k$k$k$k$k$k-20261121-12000$k.sqlite"
      echo "REMOVED-$k: "
      echo "SNAPSHOTS-AFTER-$k: x"
      echo "EXPECTED-$k: x"
      echo "MATCH-$k: yes"
      echo "LEVEL-AFTER-$k: $((69 + k))"
    done
    echo "REMOVALS-TOTAL: 2"
    echo "REMOVALS-LOGGED: 2"
  } > "$f"
  if [ -n "${2:-}" ]; then fx_set "$f" "$2" "${3:-}"; fi
}

selftest_verdict_upd01() {
  local f=$ST_TMP/upd.txt kv
  st_ans "verdict_upd01: no file" NOT-RUN verdict_upd01 "$ST_TMP/absent.txt"
  fx_upd "$f"
  st_ans "verdict_upd01: every key good" PASS verdict_upd01 "$f"
  for kv in "LEVEL-EXPECTED-B0|-" "LEVEL-EXPECTED-B0|x" "LEVEL-EXPECTED-B0|68" \
    "MATCH-1|no" "MATCH-3|-" "MATCH-6|no" "MATCH-6|" \
    "READYZ-2|v0.1.4-241-g1111111" "READYZ-2|" "READYZ-4|-" "BUILD-5|-" \
    "WRITTEN-1|" "WRITTEN-1|-" "WRITTEN-6|holzcloud.sqlite" \
    "LEVEL-AFTER-6|74" "LEVEL-AFTER-6|76" "LEVEL-AFTER-6|-" "LEVEL-AFTER-1|69" \
    "REMOVALS-TOTAL|0" "REMOVALS-TOTAL|-" "REMOVALS-LOGGED|1" "REMOVALS-LOGGED|-"; do
    fx_upd "$f" "${kv%%|*}" "${kv#*|}"
    st_ans "verdict_upd01: ${kv%%|*} '${kv#*|}'" FAIL verdict_upd01 "$f"
  done
  fx_upd "$f"
  echo "MATCH-2: yes" >> "$f"
  st_ans "verdict_upd01: a key twice" FAIL verdict_upd01 "$f"
  fx_upd "$f" REMOVALS-TOTAL 1
  fx_set "$f" REMOVALS-LOGGED 1
  st_ans "verdict_upd01: one removal, logged" PASS verdict_upd01 "$f"
}

# The binary's source is everything of this module that ./cmd/holzcloud compiles
# or embeds: cmd, internal, go.mod, go.sum and the root package (changelog.go,
# which embeds CHANGELOG.md). IN-01 of the phase 34 review.
selftest_binary_source() {
  local p
  for p in cmd/holzcloud/main.go internal/db/db.go go.mod go.sum changelog.go CHANGELOG.md; do
    st_ok "binary_covered: $p" binary_covered "$p"
  done
  for p in changelog.go.orig cmdx/main.go sdk/sdk.go plugins/suche/main.go tools/i18n/main.go README.md deploy/DEPLOY.md ""; do
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

# The synthetic builds fail closed (IN-02 of the phase 34 review): bash ignores
# `set -e` in a subshell that is the left side of `||`, so each step is checked
# by hand. syn_clone is stubbed for the mktemp cases, so a build that went on
# would only print where it would have cloned, never clone.
selftest_syn_fail_closed() {
  local d=$ST_TMP/syn out rc b
  rm -rf "${d:?}"
  mkdir -p "$d"
  for b in syn_build_rb syn_build_upd; do
    out=$( (
      NEW_REF=$(git rev-parse --short HEAD)
      TMPDIR=$d/no-such-dir
      export TMPDIR
      syn_clone() { echo "CLONE:$1"; }
      syn_migration() { :; }
      syn_commit_build() { :; }
      "$b" "$d/$b"
    ) 2> /dev/null) && rc=0 || rc=$?
    st_eq "$b: a failing mktemp ends the build" 1 "$rc"
    case "$out" in
      *CLONE:*) st_eq "$b: a failing mktemp clones nothing" "" "$out" ;;
      *) st_eq "$b: a failing mktemp clones nothing" "" "" ;;
    esac
  done
  # A mv of the built binary that fails ends the build, although the .info file
  # beside it could still be written.
  mkdir -p "$d/m/src/internal/db/migrations" "$d/bin/holzcloud-x/holzcloud/keep"
  git -C "$d/m/src" init -q
  echo "-- x" > "$d/m/src/internal/db/migrations/00001_x.sql"
  out=$( (
    doc_block() { echo 'touch holzcloud'; }
    syn_commit_build "$d/m" "$d/bin/holzcloud-x" "t34: selftest"
  ) 2>&1) && rc=0 || rc=$?
  st_eq "syn_commit_build: a failing mv ends the build" 1 "$rc"
}

cmd_selftest() {
  ST_N=0
  ST_BAD=0
  ST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/upgrade-verify-selftest.XXXXXX")
  trap 'rm -rf "${ST_TMP:?}"' EXIT
  selftest_names
  selftest_stages
  selftest_level
  selftest_verdict_mig01
  selftest_verdict_mig02
  selftest_applied
  selftest_record
  selftest_appendix
  selftest_doc_block
  selftest_play_kind
  selftest_restore_with
  selftest_json
  selftest_guard_sentence
  selftest_compare_state
  selftest_verdict_upd02
  selftest_prune_expect
  selftest_verdict_upd01
  selftest_binary_source
  selftest_binary_files
  selftest_syn_fail_closed
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
    appendix)
      st=${1:-}
      [ "$#" -eq 0 ] || shift
      rc=0
      cmd_appendix "$st" || rc=$?
      [ "$rc" -eq 0 ] || exit "$rc"
      ;;
    record-check)
      rc=0
      cmd_record_check || rc=$?
      [ "$rc" -eq 0 ] || exit "$rc"
      ;;
    *) usage ;;
  esac
done
