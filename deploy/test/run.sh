#!/usr/bin/env bash
# Drives the systemd test container for deploy/holzcloud.service.
#
#   deploy/test/run.sh smoke                  build, up, start, check /healthz and /readyz
#   deploy/test/run.sh build up start canary  and learn whether seccomp is enforced here
#   deploy/test/run.sh build up start         the same steps one by one
#   deploy/test/run.sh build up start seed check    plugins: install, load, invoke (UNIT-02)
#   deploy/test/run.sh mdwe                   the narrow MemoryDenyWriteExecute run
#   deploy/test/run.sh all                    the whole recipe, in the order that keeps the score clean
#   deploy/test/run.sh build up score         systemd-analyze security for the unit as shipped
#   deploy/test/run.sh exec -- systemctl status holzcloud
#   deploy/test/run.sh down
#
# Phase 21 plays deploy/DEPLOY.md literally in a container started from
# TARGET=systemd-base (the commands are read out of the document itself):
#   deploy/test/run.sh build up install        TARGET=systemd-base, then DEPLOY.md's install blocks
#   deploy/test/run.sh firstrun caddy postinstall   on the container `install` left behind
#   INSTALL_FROM=old deploy/test/run.sh build up install baseline update   the old release installed and seeded,
#                                              then the Updating section played (OUT_DIR/update.txt), state s1 compared
#                                              with s0 (compare-s0-s1.txt) and with the service's own pre-upgrade
#                                              snapshot (attach-s1.txt), and a marker page written (state s1m)
#   deploy/test/run.sh negative rollback        after update: the old binary against the migrated database (a step the
#                                              document says not to take: SCENARIO lines in scenario.txt, result in
#                                              negative.txt), then the Rolling back section played (rollback.txt,
#                                              compare-s0-s2.txt), the old version and the pre-update data back
#   deploy/test/run.sh compare state-s0.txt state-s1.txt   the comparison alone, on two state files
#   TARGET=systemd-base deploy/test/run.sh fresh        the whole of DEP-01 in one command, OUT_DIR/summary.txt
#   TARGET=systemd-base [OLD_TAG=v0.0.21] deploy/test/run.sh lifecycle   DEP-02 and DEP-03 in one command, OUT_DIR/summary.txt
#
# The subcommand names, the environment variables below, the exit codes and the
# file names under OUT_DIR are an interface: later plans and phases 21 and 22
# script against them. Extend, do not rename.
#
# Environment (all optional):
#   PLATFORM         linux/amd64 (default) or linux/arm64
#   BASE_IMAGE       debian:bookworm-slim
#   TARGET           unit-test (default) or systemd-base
#   IMAGE            holzcloud-unit-test:ARCH-BASETAG[-TARGET]
#   NAME             holzcloud-unit-test-ARCH
#   OUT_DIR          deploy/test/out/ARCH (gitignored scratch, reset by `up`)
#   KEEP             1 keeps the container after `smoke`
#   READY_TIMEOUT    seconds `wait` polls /healthz, default 120
#   DOCKER_RUN_ARGS  extra `docker run` flags, split at spaces on purpose
#   VERSION, COMMIT  passed to the build when set
#   DEPLOY_DOC       the document the install steps are read from, default deploy/DEPLOY.md
#   OLD_TAG          the older release, default v0.0.10 (built by `bins` with WITH_OLD=1)
#   NEW_REF          the ref installed by default, default HEAD
#   BIN_DIR          built binaries, default deploy/test/out/bin (outside OUT_DIR on purpose)
#   INSTALL_FROM     which binary `install` stages: new (default) or old
#   WITH_OLD=1       `bins` builds the old binary too; REBUILD=1 rebuilds an existing one
#
# Exit codes: 0 ok, 1 failure, 2 usage, 3 "ran, but this platform cannot give
# the evidence".
#
# Written for bash 3.2 (macOS) and bash 5: no arrays, no mapfile, no ${v,,}.
# Anything that needs GNU tools runs inside the container through `docker exec`.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

PLATFORM=${PLATFORM:-linux/amd64}
BASE_IMAGE=${BASE_IMAGE:-debian:bookworm-slim}
TARGET=${TARGET:-unit-test}
KEEP=${KEEP:-0}
READY_TIMEOUT=${READY_TIMEOUT:-120}
DOCKER_RUN_ARGS=${DOCKER_RUN_ARGS:-}

ARCH=${PLATFORM#*/}
BASE_TAG=$(printf '%s' "$BASE_IMAGE" | tr ':/' '--')
NAME=${NAME:-holzcloud-unit-test-$ARCH}
if [ -z "${IMAGE:-}" ]; then
  IMAGE=holzcloud-unit-test:$ARCH-$BASE_TAG
  [ "$TARGET" = unit-test ] || IMAGE=$IMAGE-$TARGET
fi
OUT_DIR=${OUT_DIR:-deploy/test/out/$ARCH}
OLD_TAG=${OLD_TAG:-v0.0.10}
NEW_REF=${NEW_REF:-HEAD}
BIN_DIR=${BIN_DIR:-deploy/test/out/bin}
INSTALL_FROM=${INSTALL_FROM:-new}
DEPLOY_DOC=${DEPLOY_DOC:-deploy/DEPLOY.md}
WITH_OLD=${WITH_OLD:-0}
REBUILD=${REBUILD:-0}
STAGE_BIN=${STAGE_BIN:-}
# Phase 22: two containers. NAME and OUT_DIR keep meaning the SOURCE, the
# container that is backed up; the TARGET is a second, fresh container that the
# restore is played onto (on_target switches the globals). RESTORE_DIR is the
# backup directory inside the target that the document's restore line is given,
# RESTORE_ANSWER what is typed at its question, BACKUP_DIR_PATH an explicit
# source directory for `transfer`.
TGT_NAME=${TGT_NAME:-$NAME-tgt}
TGT_OUT_DIR=${TGT_OUT_DIR:-$OUT_DIR-tgt}
RESTORE_DIR=${RESTORE_DIR:-}
RESTORE_ANSWER=${RESTORE_ANSWER:-yes}
BACKUP_DIR_PATH=${BACKUP_DIR_PATH:-}
# Phase 22, the timer and the bulk point: TIMER_OFFSET seconds ahead of the
# container's clock is when the container-only drop-in makes the timer fire,
# TIMER_WAIT seconds is how long `backup-timer` polls for the run, BULK_MB the
# megabytes of synthetic media `backup-fixture` writes (0: none; the chain uses
# 300 for its second data point).
TIMER_OFFSET=${TIMER_OFFSET:-60}
TIMER_WAIT=${TIMER_WAIT:-$((TIMER_OFFSET + 180))}
BULK_MB=${BULK_MB:-}
SRC_NAME=""
SRC_OUT_DIR=""
# A caller sets DOC_TIMED=1 around one doc_cmd call to have the line timed.
DOC_TIMED=0

say() { printf '%s\n' "$*" >&2; }
die() { say "run.sh: $*"; exit 1; }
dx() { docker exec "$NAME" "$@"; }
dxi() { docker exec -i "$NAME" "$@"; }

# ---------------------------------------------------------------------------
# The document is the only source of knowledge (D-01, D-12). Every command the
# install steps run is READ out of DEPLOY_DOC, and doc_cmd refuses a line that
# is not in it, so this script can mirror the document and never drift from it.
# ---------------------------------------------------------------------------

# doc_awk HEADING N: the logical command lines of the fenced bash blocks of the
# section whose ## or ### heading line is exactly HEADING (a plain string
# comparison: Caddy (Reverse Proxy + HTTPS) holds pattern characters). N is the
# Nth bash block, 0 all of them, -1 prints the number of bash blocks instead.
# An empty HEADING means the whole document. Backslash-continued lines are
# joined; blank lines and lines that start with # are dropped. The section ends
# at the next heading of the same or a higher level. Exit 3: no such heading,
# 4: fewer bash blocks than asked for. POSIX awk only.
doc_awk() {
  H=$1 N=$2 awk '
    function ltrim(s) { sub(/^[ \t]+/, "", s); return s }
    BEGIN {
      h = ENVIRON["H"]; want = ENVIRON["N"] + 0
      all = (h == ""); insec = all; found = all; seclvl = 0
      nb = 0; inf = 0; isbash = 0; take = 0; cont = ""; hascont = 0
    }
    {
      line = $0
      sub(/\r$/, "", line)
      t = ltrim(line)
      if (inf) {
        if (t ~ /^```[ \t]*$/) {
          if (hascont) { print cont; hascont = 0; cont = "" }
          inf = 0
          next
        }
        if (!(isbash && insec && take)) next
        s = t
        if (hascont) {
          s = cont " " s; hascont = 0; cont = ""
        } else if (s == "" || s ~ /^#/) {
          next
        }
        if (s ~ /\\$/) {
          sub(/[ \t]*\\$/, "", s)
          cont = s; hascont = 1
          next
        }
        sub(/[ \t]+$/, "", s)
        if (s == "" || s ~ /^#/) next
        print s
        next
      }
      if (t ~ /^```/) {
        inf = 1
        lang = t
        sub(/^`+[ \t]*/, "", lang)
        sub(/[ \t].*$/, "", lang)
        isbash = (lang == "bash")
        take = 0
        if (isbash && insec) { nb++; take = (want == 0 || nb == want) }
        next
      }
      if (match(line, /^#+ /)) {
        lvl = RLENGTH - 1
        txt = substr(line, RLENGTH + 1)
        sub(/[ \t]+$/, "", txt)
        if (!all) {
          if (insec && lvl <= seclvl) insec = 0
          if (!found && (txt "") == (h "") && (lvl == 2 || lvl == 3)) {
            insec = 1; found = 1; seclvl = lvl
          }
        }
        next
      }
    }
    END {
      if (!found) exit 3
      if (want < 0) { print nb; exit 0 }
      if (want > 0 && nb < want) exit 4
      if (want == 0 && !all && nb == 0) exit 4
    }' "$DEPLOY_DOC"
}

# doc_lines HEADING [N]: see doc_awk. Dies, naming DEPLOY_DOC, when the heading
# or the block does not exist. Call it as `x=$(doc_lines ...) || exit 1`: a die
# inside a command substitution only ends the substitution.
doc_lines() {
  local out rc=0
  [ -f "$DEPLOY_DOC" ] || die "the document $DEPLOY_DOC does not exist"
  out=$(doc_awk "$1" "${2:-0}") || rc=$?
  case "$rc" in
    0) ;;
    3) die "$DEPLOY_DOC has no section '$1' (a ## or ### heading with exactly that text)" ;;
    4) die "$DEPLOY_DOC: the section '$1' has fewer than ${2:-1} bash blocks" ;;
    *) die "reading $DEPLOY_DOC failed (awk exit $rc)" ;;
  esac
  [ -z "$out" ] || printf '%s\n' "$out"
}

# doc_blocks HEADING: how many bash blocks the section has.
doc_blocks() {
  local out rc=0
  [ -f "$DEPLOY_DOC" ] || die "the document $DEPLOY_DOC does not exist"
  out=$(doc_awk "$1" -1) || rc=$?
  [ "$rc" -eq 0 ] || die "$DEPLOY_DOC has no section '$1' (a ## or ### heading with exactly that text)"
  printf '%s\n' "$out"
}

# Every section `install` plays must exist before docker is touched.
doc_preflight() {
  doc_lines "Build" 1 >/dev/null
  doc_lines "Transfer to the server" 1 >/dev/null
  doc_lines "Install" 1 >/dev/null
  doc_lines "systemd Service" 1 >/dev/null
}

steps_file() { printf '%s/steps.txt' "$OUT_DIR"; }
steps_count() {
  local n
  n=$(grep -c '^STEP ' "$(steps_file)" 2>/dev/null || true)
  printf '%s\n' "${n:-0}"
}

# steps_failed_since N: the counted steps after number N that ended non-zero.
# `systemctl status` reports the unit state through its exit code, so it never
# counts; neither does a SKIP.
steps_failed_since() {
  [ -f "$(steps_file)" ] || return 0
  awk -v n="$1" '$1 == "STEP" && ($2 + 0) > n && $3 != "rc=0" && index($0, "systemctl status") == 0' "$(steps_file)"
}

# deviation TEXT: a named deviation goes to the screen and to deviations.txt.
deviation() {
  mkdir -p "$OUT_DIR"
  say "$*"
  printf '%s\n' "$*" >> "$OUT_DIR/deviations.txt"
}

# Run INSIDE the container. TIMED_RUN_SH: the line is the script on stdin; two
# readings of the container's clock around it, and its own exit status.
TIMED_RUN_SH='t0=$(date +%s.%N)
rc=0
bash -s || rc=$?
t1=$(date +%s.%N)
echo "WALL $t0 $t1"
exit "$rc"'

# RESTORE_RUN_SH: $1 is the restore line, stdin the answer. Every output line of
# the script is prefixed with the container's clock, so the restore can be split
# into its steps from the transcript; the exit status is the script's own
# (PIPESTATUS), not that of the loop that stamps the lines.
RESTORE_RUN_SH='set -o pipefail
t0=$(date +%s.%N)
bash -c "$1" 2>&1 | while IFS= read -r l; do printf "%s %s\n" "$(date +%s.%N)" "$l"; done
rc=${PIPESTATUS[0]}
t1=$(date +%s.%N)
echo "WALL $t0 $t1"
exit "$rc"'

# doc_cmd LINE: the single place where a command is played. LINE must be a
# logical line of a bash block of the document, and must be pasteable as it
# stands. A step that fails is recorded, never hidden; its status is returned.
doc_cmd() {
  local line=$1 all n nn rc=0 outf kind=run
  [ -n "$line" ] || die "doc_cmd: an empty command line"
  [ -f "$DEPLOY_DOC" ] || die "the document $DEPLOY_DOC does not exist"
  all=$(doc_awk "" 0) || die "reading $DEPLOY_DOC failed"
  grep -Fxq -- "$line" <<< "$all" \
    || die "the document and the script have drifted apart: $DEPLOY_DOC has no command line '$line'"
  if grep -Eq '<[A-Za-z][A-Za-z0-9_.-]*>' <<< "$line"; then
    die "refusing '$line': it holds an angle-bracket placeholder, and a played block must be pasteable as it stands (D-01)"
  fi

  mkdir -p "$OUT_DIR"
  n=$(steps_count)
  n=$((n + 1))
  nn=$(printf '%02d' "$n")
  outf="$OUT_DIR/step-$nn.txt"
  printf '$ %s\n' "$line" > "$outf"

  case "$line" in
    scp\ *) kind=scp ;;
    *"go build"*) kind=skip ;;
    *"systemctl reload caddy"*) kind=caddy ;;
    *"apt install"*)
      # No -y: the operator answers apt's question. stdin is the script, which
      # has ended, so the answer has to come from somewhere else.
      case "$line" in *" -y"*) kind=run ;; *) kind=prompt ;; esac
      ;;
    *"/restore.sh "*) kind=restore ;;
    *) kind=run ;;
  esac

  say ">>> step $nn: $line"
  case "$kind" in
    scp)
      local dest parent
      set -f
      # shellcheck disable=SC2086
      set -- $line
      set +f
      if [ "$#" -eq 3 ] && [ "$2" = holzcloud ] && [ "${3#*:}" != "$3" ]; then
        [ -f "$STAGE_BIN" ] || die "STAGE_BIN '$STAGE_BIN' is not a file: nothing to put where the scp line sends the binary"
        dest=${3#*:}
        deviation "DEVIATION[scp]: '$line' was played as docker cp of $STAGE_BIN to $dest in the container (there is no ssh and no second machine here)"
        docker cp "$STAGE_BIN" "$NAME:$dest" > "$outf.tmp" 2>&1 || rc=$?
      elif [ "$#" -eq 4 ] && [ "$2" = -r ] && [ "$3" = deploy/ ] && [ "${4#*:}" != "$4" ]; then
        dest=${4#*:}
        dest=${dest%/}
        parent=$(dirname "$dest")
        [ "$(basename "$dest")" = deploy ] \
          || die "no translation for this scp line: the target directory must be named deploy: $line"
        deviation "DEVIATION[scp]: '$line' was played as a tar stream of the working tree's deploy directory (without deploy/test/out) into $parent in the container, by docker cp -"
        # macOS bsdtar: COPYFILE_DISABLE and --no-mac-metadata keep ._ files out, and
        # --no-xattrs the com.apple.provenance attribute, which the daemon cannot set
        # in the container ("lsetxattr ... operation not supported").
        COPYFILE_DISABLE=1 tar -C "$ROOT" --no-xattrs --no-acls --no-mac-metadata \
          --exclude deploy/test/out --exclude .DS_Store -cf - deploy \
          | docker cp - "$NAME:$parent" > "$outf.tmp" 2>&1 || rc=$?
      else
        die "no translation for this scp line: $line"
      fi
      cat "$outf.tmp" >> "$outf"
      cat "$outf.tmp" >&2
      rm -f "$outf.tmp"
      ;;
    skip)
      printf '%s\n' "STEP $nn rc=0 SKIP[dev-machine] $line" | tee -a "$(steps_file)"
      say "SKIP[dev-machine]: the Build line runs on the development machine; bins already played it from this same line"
      return 0
      ;;
    run)
      # The line is the whole script on stdin, run as root: sudo exists in the
      # image, so a sudo line runs as it is written. dxi has its own stdin, so
      # a loop that feeds doc_cmd is not read from.
      if [ "${DOC_TIMED:-0}" = 1 ]; then
        # A caller that wants the wall time of this one line (the backup) sets
        # DOC_TIMED=1 around the call. The readings are the container's own GNU
        # date, never `time` of a pipeline and never the host's date (D-14).
        : > "$OUT_DIR/backup-run.txt"
        printf '%s\n' "$line" | dxi bash -c "$TIMED_RUN_SH" 2>&1 | tee -a "$outf" "$OUT_DIR/backup-run.txt" || rc=$?
      else
        printf '%s\n' "$line" | dxi bash -s 2>&1 | tee -a "$outf" || rc=$?
      fi
      ;;
    restore)
      # The document's restore line carries a sample timestamp, because the
      # document cannot know the name of the directory a backup made. The last
      # word of the line is replaced by RESTORE_DIR; everything else is the
      # line as written (D-05). The answer comes from stdin; the script text is
      # an argument of bash -c, so it does not share that stdin and restore.sh's
      # read is not fed the script. Note that `read -p` prints no prompt when
      # stdin is not a terminal, so the [yes/NO] question never appears in the
      # transcript.
      local rlast rline
      [ -n "$RESTORE_DIR" ] || die "doc_cmd: '$line' runs restore.sh with a backup directory; set RESTORE_DIR to that directory inside the container"
      rlast=${line##* }
      rline="${line%"$rlast"}$RESTORE_DIR"
      deviation "DEVIATION[backup-name]: the document's sample timestamp ($rlast) was replaced by the directory $RESTORE_DIR, because the document cannot know the name a backup was given; the rest of the line is as written and the answer '$RESTORE_ANSWER' is typed on stdin"
      printf '%s\n' "$RESTORE_ANSWER" | docker exec -i "$NAME" bash -c "$RESTORE_RUN_SH" _ "$rline" 2>&1 | tee "$outf.tmp" || rc=$?
      cat "$outf.tmp" >> "$outf"
      cp "$outf.tmp" "$OUT_DIR/restore-run.txt"
      rm -f "$outf.tmp"
      ;;
    caddy)
      deviation "DEVIATION[caddy]: '$line' was replaced by 'caddy validate --config /etc/caddy/Caddyfile': the container has no public domain, no DNS and no ACME, and a real reload would start certificate issuance for example.com"
      printf '%s\n' "caddy validate --config /etc/caddy/Caddyfile" | dxi bash -s 2>&1 | tee -a "$outf" || rc=$?
      ;;
    prompt)
      deviation "DEVIATION[prompt]: '$line' asks apt for a confirmation and nobody types one here; the answer y is supplied on stdin"
      { yes || true; } | dxi bash -c "$line" 2>&1 | tee -a "$outf" || rc=$?
      ;;
  esac
  if [ "$rc" -ne 0 ] && grep -Eqi 'could not resolve|temporary failure resolving|network is unreachable|failed to connect|connection timed out|name or service not known' "$outf"; then
    deviation "DEVIATION[network]: step $nn failed with no route to a package repository or download host: $line"
  fi
  printf '%s\n' "STEP $nn rc=$rc $line" | tee -a "$(steps_file)"
  return "$rc"
}

# doc_play HEADING [N]: every logical line of the section (or its Nth bash
# block) through doc_cmd. It goes on after a failing step, so one run shows
# every finding, and returns non-zero if any counted step failed.
doc_play() {
  local lines line n0
  lines=$(doc_lines "$@") || exit 1
  n0=$(steps_count)
  while IFS= read -r line; do
    doc_cmd "$line" || true
  done <<< "$lines"
  [ -z "$(steps_failed_since "$n0")" ]
}

# One EXIT handler for the whole script. smoke and mdwe set CLEANUP_DOWN so the
# container goes away unless KEEP=1; mdwe-control sets MDWE_DROPIN while its
# weakening drop-in exists, so a failure or an interrupt still restores the unit
# (T-20-06).
CLEANUP_DOWN=0
MDWE_DROPIN=0
BINS_TMP=""
PHASE22_RC=0
on_exit() {
  local rc=$?
  trap - EXIT
  if [ -n "$BINS_TMP" ]; then
    rm -rf "${BINS_TMP:?}"
  fi
  if [ "$MDWE_DROPIN" = 1 ]; then
    mdwe_restore || rc=1
  fi
  if [ "$CLEANUP_DOWN" = 1 ] && [ "$KEEP" != 1 ]; then
    cmd_down
  fi
  exit "$rc"
}

usage() {
  say "usage: run.sh SUBCOMMAND [SUBCOMMAND...]"
  say "  build up start wait canary down smoke   steps, may be combined: run.sh build up start"
  say "  seed                             test website, five plugins, a page and an image through /ai"
  say "  check                            restart the unit, 17 assertions on the plugins, engine, latency"
  say "  mdwe-control                     the same checks with MemoryDenyWriteExecute=no, then restore"
  say "  verdict                          MDWE verdict from canary.verdict, check.txt, mdwe-control.txt"
  say "  mdwe                             build up start canary seed check mdwe-control verdict"
  say "  score                            systemd-analyze security, online and offline, raw (run before start)"
  say "  all                              build up score start canary seed check mdwe-control verdict, then a summary"
  say "                                   exits 0 when nothing really failed, even if canary or verdict said 3"
  say "  exit 3 means: ran, but this platform cannot give the evidence (canary or verdict VOID)"
  say "  bins                             build the binaries of NEW_REF (and OLD_TAG with WITH_OLD=1) by the Build line of the document"
  say "  install                          play the document's Build, Transfer, Install and systemd Service blocks from TARGET=systemd-base"
  say "  firstrun                         the document's First Run over curl: setup, login, compulsory two-factor redirect"
  say "  caddy                            the document's Caddy block; reload replaced by caddy validate; exit 3 without a network"
  say "  postinstall                      the document's Backups block, then every deploy path it names must exist"
  say "  baseline                         INSTALL_FROM=old: seed the old install, a login user and an order row, capture state s0"
  say "  update                           play the document's Updating section as it stands, capture s1, compare with s0 and the pre-upgrade snapshot, write the marker page"
  say "  negative                         after update: the OLD binary against the migrated database, once, recorded in negative.txt; the new binary comes back"
  say "  rollback                         after update: play the document's Rolling back section, capture s2, compare with s0 (or s1m), rollback.txt"
  say "  fresh                            DEP-01 from a bare image: build up install firstrun postinstall caddy, then summary.txt (needs TARGET=systemd-base)"
  say "  lifecycle                        DEP-02 and DEP-03: install OLD_TAG, baseline, update, negative case (when a migration lies between), rollback, summary.txt (needs TARGET=systemd-base)"
  say "  compare A B [OUTFILE]            compare two state files (state-LABEL.txt) without a container; exit 1 on a BAD line"
  say "  backup                           the document's manual backup line in the source container, timed, then two integrity readers on the backup (OUT_DIR/backup.txt, verify-backup.txt)"
  say "  transfer                         the backup as a tar stream into the target container TGT_NAME, plus the fixture files by exec-pipe (TGT_OUT_DIR/transfer.txt)"
  say "  restore                          in the target: the document's restore line answered yes, then setup form, readyz and login (TGT_OUT_DIR/restore.txt)"
  say "  backup-timer                     in the source: a container-only drop-in brings holzcloud-backup.timer TIMER_OFFSET seconds ahead, one run of the hardened service is observed (OUT_DIR/timer.txt)"
  say "  restore-negative                 in the target, after restore: a damaged backup, one without a database, the answer no and a backup with no free space are refused, the live data unchanged (TGT_OUT_DIR/restore-negative.txt)"
  say "  backup-fixture                   in the source: a language file and a logo are planted, and BULK_MB of synthetic media are written (OUT_DIR/bulk.txt)"
  say "  exec [--] COMMAND...             run a command in the container (takes the rest)"
  say "  logs [ARGS...]                   journalctl -u holzcloud (takes the rest)"
  exit 2
}

# Polls /healthz inside the container once a second. Never a fixed sleep: under
# Rosetta the service needs 8 to 10 seconds, the plugin compile included.
wait_ready() {
  local i=1
  while [ "$i" -le "$READY_TIMEOUT" ]; do
    if dx curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  say "run.sh: /healthz did not answer within ${READY_TIMEOUT}s"
  dx journalctl -u holzcloud --no-pager -n 40 >&2 || true
  return 1
}

cmd_build() {
  local extra=""
  [ -z "${VERSION:-}" ] || extra="$extra --build-arg VERSION=$VERSION"
  [ -z "${COMMIT:-}" ] || extra="$extra --build-arg COMMIT=$COMMIT"
  say "building $IMAGE (target $TARGET, $PLATFORM)"
  # --platform goes to build AND run: a locally built image started without it
  # failed with a pull error.
  # shellcheck disable=SC2086
  docker build --platform "$PLATFORM" --target "$TARGET" \
    -f deploy/test/Dockerfile --build-arg "BASE_IMAGE=$BASE_IMAGE" $extra \
    -t "$IMAGE" .
}

cmd_up() {
  if docker container inspect "$NAME" >/dev/null 2>&1; then
    die "container $NAME already exists; run 'run.sh down' first (a stale one is never reused)"
  fi

  # OUT_DIR is removed and recreated; refuse anything that could be the tree.
  case "${OUT_DIR:?}" in
    / | . | .. | ./ | "$ROOT" | "$ROOT"/) die "refusing OUT_DIR '$OUT_DIR'" ;;
  esac
  rm -rf "${OUT_DIR:?}"
  mkdir -p "$OUT_DIR"

  local host_arch
  host_arch=$(docker info --format '{{.Architecture}}')
  say "platform:   $PLATFORM"
  say "image:      $IMAGE"
  say "target:     $TARGET"
  say "out dir:    $OUT_DIR"
  say "host arch:  $host_arch"
  if [ "$PLATFORM" != linux/amd64 ]; then
    say "DEVIATION[D-01]: $PLATFORM is the one narrow native run the user allowed on 2026-10-12 as MDWE evidence."
    say "  UNIT-01 and UNIT-03 are measured on linux/amd64. Reason: under Rosetta the amd64"
    say "  container installs no seccomp filter (Seccomp: 0), so it cannot decide MemoryDenyWriteExecute."
  fi

  # No -p and no -v: nothing is published and no host path is mounted (T-20-01).
  # Private cgroupns worked on Docker Desktop with cgroup version 2, so D-02's
  # --cgroupns=host is not needed here. A host that needs it passes it, plus the
  # cgroup mount, through DOCKER_RUN_ARGS.
  # shellcheck disable=SC2086
  docker run -d --name "$NAME" --platform "$PLATFORM" --privileged \
    --tmpfs /run --tmpfs /run/lock $DOCKER_RUN_ARGS "$IMAGE" >/dev/null

  local i=1 state=""
  while [ "$i" -le 90 ]; do
    # Non-zero while booting, so tolerate it and read the text.
    state=$(dx systemctl is-system-running 2>/dev/null || true)
    case "$state" in running | degraded) break ;; esac
    sleep 1
    i=$((i + 1))
  done
  case "$state" in
    running | degraded) say "systemd is $state after ${i}s" ;;
    *) die "systemd did not reach running or degraded within 90s (last state: '${state}')" ;;
  esac

  {
    echo "host architecture:     $(docker info --format '{{.Architecture}}')"
    echo "host operating system: $(docker info --format '{{.OperatingSystem}}')"
    echo "host kernel:           $(docker info --format '{{.KernelVersion}}')"
    echo "container uname:       $(dx uname -srm)"
    echo "container systemd:     $(dx systemctl --version | head -n 1)"
    echo "container os:          $(dx grep '^PRETTY_NAME' /etc/os-release)"
  } > "$OUT_DIR/versions.txt"
}

# The unit under test, unchanged first. Only the known Rosetta crash signature
# earns the container-only drop-in; any other failure ends the run.
cmd_start() {
  local i=1 healthy=0 active restarts
  dx systemctl start holzcloud || true
  while [ "$i" -le 45 ]; do
    if dx curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
      healthy=1
      break
    fi
    active=$(dx systemctl show -p ActiveState --value holzcloud 2>/dev/null || true)
    restarts=$(dx systemctl show -p NRestarts --value holzcloud 2>/dev/null || echo 0)
    [ "$active" != failed ] || break
    [ "${restarts:-0}" -le 0 ] || break
    sleep 1
    i=$((i + 1))
  done

  if [ "$healthy" -eq 0 ]; then
    {
      dx systemctl status holzcloud --no-pager -l || true
      echo
      dx journalctl -u holzcloud --no-pager || true
    } > "$OUT_DIR/unit-start-unchanged.txt"
    if grep -q 'mmap_min_addr' "$OUT_DIR/unit-start-unchanged.txt"; then
      say "the unchanged unit crashed with the mmap_min_addr signature (Rosetta opens"
      say "/proc/sys/vm/mmap_min_addr at start and ProcSubset=pid hides it; status 5/TRAP)"
      dx sh -c 'mkdir -p /etc/systemd/system/holzcloud.service.d && printf "[Service]\nProcSubset=all\n" > /etc/systemd/system/holzcloud.service.d/container.conf'
      dx systemctl daemon-reload
      # StartLimitBurst=5 would otherwise refuse the next start for the wrong reason.
      dx systemctl reset-failed holzcloud || true
      dx systemctl restart holzcloud
      say "DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all."
      say "  Container only; deploy/holzcloud.service is untouched. Evidence: unit-start-unchanged.txt"
      wait_ready || exit 1
    else
      say "run.sh: the unit failed to start and the crash is not the known mmap_min_addr one"
      dx journalctl -u holzcloud --no-pager -n 40 >&2 || true
      exit 1
    fi
  fi

  local pid
  pid=$(dx systemctl show -p MainPID --value holzcloud)
  {
    echo "== systemctl status =="
    dx systemctl status holzcloud --no-pager -l || true
    echo
    echo "== systemctl show =="
    dx systemctl show -p MainPID -p NRestarts -p Result -p ActiveState holzcloud
    echo
    echo "== Seccomp (/proc/$pid/status, read as root) =="
    dx grep '^Seccomp' "/proc/$pid/status" || true
  } > "$OUT_DIR/unit-start.txt"
  say "unit started; evidence in $OUT_DIR/unit-start.txt"
}

# Runs the enforcement probe under the unit's own sandbox properties, once with
# MemoryDenyWriteExecute=yes and once with =no as a control, and says whether
# this platform enforces seccomp. This is how the emulation caveat of D-01 is
# measured rather than asserted. The probe sits in /usr/local/bin because
# PrivateTmp=true hides /tmp from the transient unit.
canary_run() {
  # $1 is the MemoryDenyWriteExecute value; $PROC_SUBSET is set by cmd_canary.
  dx systemd-run --wait --pipe --quiet \
    -p User=holzcloud \
    -p NoNewPrivileges=true \
    -p ProtectSystem=strict \
    -p ReadWritePaths=/opt/holzcloud/data \
    -p PrivateTmp=true \
    -p PrivateDevices=true \
    -p ProtectProc=invisible \
    -p "ProcSubset=$PROC_SUBSET" \
    -p RestrictNamespaces=true \
    -p "RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX" \
    -p LockPersonality=true \
    -p CapabilityBoundingSet= \
    -p SystemCallFilter=@system-service \
    -p "SystemCallFilter=~@privileged @resources" \
    -p "MemoryDenyWriteExecute=$1" \
    /usr/local/bin/hc-canary
}

cmd_canary() {
  dx systemctl is-active --quiet holzcloud \
    || die "holzcloud is not active; run 'run.sh start' first (start decides whether the ProcSubset drop-in exists)"

  # The same ProcSubset the unit actually runs with: the drop-in exists only
  # after the known Rosetta crash.
  PROC_SUBSET=pid
  if dx test -e /etc/systemd/system/holzcloud.service.d/container.conf; then
    PROC_SUBSET=all
  fi

  local yes no
  yes=$(canary_run yes) || die "systemd-run failed for the MemoryDenyWriteExecute=yes probe"
  no=$(canary_run no) || die "systemd-run failed for the MemoryDenyWriteExecute=no control"
  {
    echo "== MemoryDenyWriteExecute=yes =="
    echo "$yes"
    echo
    echo "== MemoryDenyWriteExecute=no (control) =="
    echo "$no"
  } > "$OUT_DIR/canary.txt"
  cat "$OUT_DIR/canary.txt"

  # (1) The control must allow both operations, or the probe is invalid: it
  # would be denied for some reason other than the seccomp filter.
  if ! { printf '%s\n' "$no" | grep -Eq '^mprotect RW->RX +ALLOWED$' \
      && printf '%s\n' "$no" | grep -Eq '^mmap RWX +ALLOWED$'; }; then
    say "CANARY INVALID - the MemoryDenyWriteExecute=no control did not allow mprotect RW->RX and mmap RWX"
    exit 1
  fi

  # Mount-namespace directives work without seccomp; report them either way.
  say "mount-namespace probes denied under the unit's properties:"
  printf '%s\n' "$yes" | grep -E '^(stat /proc/sys|stat /dev/kmsg|write /etc) +denied' >&2 \
    || say "  none"

  # (2) Under yes, either operation still ALLOWED means no filter is installed.
  if printf '%s\n' "$yes" | grep -Eq '^(mprotect RW->RX|mmap RWX) +ALLOWED$'; then
    echo not-enforced > "$OUT_DIR/canary.verdict"
    say "SECCOMP NOT ENFORCED - MDWE verdict is void on this platform"
    exit 3
  fi

  # (3)
  echo enforced > "$OUT_DIR/canary.verdict"
  say "SECCOMP ENFORCED"
}

cmd_wait() { wait_ready; }

cmd_exec() {
  [ "${1:-}" != -- ] || shift
  [ "$#" -gt 0 ] || usage
  docker exec -i "$NAME" "$@"
}

cmd_logs() { dx journalctl -u holzcloud --no-pager "$@"; }

cmd_down() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }

# The UNIT-01 assertions, shared by smoke and all: /healthz and /readyz answer
# and say what they should. Writes $OUT_DIR/http.txt, prints nothing.
assert_http() {
  local health ready
  health=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/healthz) || die "/healthz failed"
  ready=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz) || die "/readyz failed"
  {
    echo "== /healthz =="
    echo "$health"
    echo "== /readyz =="
    echo "$ready"
  } > "$OUT_DIR/http.txt"
  [ "$health" = '{"status":"ok"}' ] || die "/healthz body was '$health'"
  printf '%s' "$ready" | dxi jq -e '.status=="ok" and .integrity=="ok"' >/dev/null \
    || die "/readyz is not status ok and integrity ok: $ready"
}

cmd_smoke() {
  # The container goes away on exit unless KEEP=1 (on_exit).
  CLEANUP_DOWN=1
  cmd_build
  cmd_up
  cmd_start
  assert_http
  say "UNIT-01 PASS"
}

# ---------------------------------------------------------------------------
# UNIT-02: plugins under the hardened unit, and MemoryDenyWriteExecute.
# ---------------------------------------------------------------------------

# Runs INSIDE the container (docker exec -i bash -c "$MCP_SH"): stdin is one
# JSON-RPC request body. The bearer key is read from the root-only key file
# there and never leaves the container (T-20-05). A failing tool answers HTTP
# 200 with result.isError (toolError in internal/ai/mcp.go), a protocol failure
# with .error; both end in a non-zero exit and the text on stderr. Success
# prints result.content[0].text, which is a JSON document.
MCP_SH='set -eu
resp=$(curl -sS --max-time 120 -X POST http://127.0.0.1:8080/ai -H "Authorization: Bearer $(cat /root/.hc-key)" -H "Content-Type: application/json" --data-binary @-)
if printf "%s" "$resp" | jq -e "(.error == null) and ((.result.isError // false) | not)" >/dev/null 2>&1; then
  printf "%s\n" "$resp" | jq -r ".result.content[0].text"
else
  printf "%s\n" "$resp" | jq -r ".error.message // .result.content[0].text" >&2 || printf "%s\n" "$resp" >&2
  exit 1
fi'

# mcp TOOL ARGUMENTS_JSON: one tools/call. The body is built with jq, never by
# concatenating JSON.
mcp() {
  dx jq -n --arg n "$1" --argjson a "$2" \
    '{jsonrpc:"2.0",id:1,method:"tools/call",params:{name:$n,arguments:$a}}' \
    | dxi bash -c "$MCP_SH"
}

# mcp_install PLUGIN_ID: the archive is about 1 MB. It goes through a pipe, never
# through jq --arg or a command line (Argument list too long, and the tool then
# reports an empty file).
mcp_install() {
  dx bash -c 'base64 -w0 "/opt/holzcloud/plugin-zips/$1.zip" | jq -Rs "{jsonrpc:\"2.0\",id:1,method:\"tools/call\",params:{name:\"install_plugin\",arguments:{archive_base64:.}}}"' _ "$1" \
    | dxi bash -c "$MCP_SH"
}

seed_log() { # $1 tool, $2 answer text
  {
    echo "== $1 =="
    echo "$2"
    echo
  } >> "$OUT_DIR/seed.txt"
}

cmd_seed() {
  dx systemctl is-active --quiet holzcloud \
    || die "holzcloud is not active; run 'run.sh start' first"
  if dx test -e /root/.hc-key; then
    die "this container is already seeded (key file exists); run 'run.sh down' and 'run.sh up' for a fresh one"
  fi
  mkdir -p "$OUT_DIR"
  : > "$OUT_DIR/seed.txt"

  # The key is created and kept inside the container only. The CLI prints the
  # secret on a line of its own that starts with ai.TokenPrefix ("hc_"); only
  # that line is selected, inside the container, and written to a file created
  # under umask 077. Nothing of it reaches this shell. Run as the service user
  # with the unit's data directory, so the database files stay owned by
  # holzcloud.
  dx bash -c 'set +x; umask 077
    out=$(runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud ai key create -name hc-unit-test -level admin) || { echo "ai key create failed" >&2; exit 1; }
    printf "%s\n" "$out" | grep -m1 "^hc_" > /root/.hc-key
    test -s /root/.hc-key' || die "could not create the admin key"

  local ans wid pid id
  ans=$(mcp create_website '{"name":"Test"}') || die "create_website failed: $ans"
  seed_log create_website "$ans"
  wid=$(printf '%s' "$ans" | dxi jq -e '.id') || die "create_website gave no id"

  ans=$(mcp add_domain "{\"website\":$wid,\"domain\":\"test.local\",\"primary\":true}") || die "add_domain failed: $ans"
  seed_log add_domain "$ans"

  # Per plugin, in this order: the foreign key of set_plugin_websites needs the
  # plugin to exist, and enable_plugin loads it for those websites.
  for id in bestellung jahreszahl kontaktformular nicht-gefunden suche; do
    ans=$(mcp_install "$id") || die "install_plugin $id failed: $ans"
    seed_log "install_plugin $id" "$ans"
    ans=$(mcp set_plugin_websites "{\"id\":\"$id\",\"websites\":[$wid]}") || die "set_plugin_websites $id failed: $ans"
    seed_log "set_plugin_websites $id" "$ans"
    ans=$(mcp enable_plugin "{\"id\":\"$id\"}") || die "enable_plugin $id failed: $ans"
    seed_log "enable_plugin $id" "$ans"
  done

  local md args
  md=$(printf 'Plugintest page\n\nJahr: [[jahr]]\n\n[[formular]]\n\n[[bestellung]]\n')
  args=$(dx jq -n --argjson w "$wid" --arg md "$md" \
    '{website:$w,title:"Plugintest",slug:"plugintest",markdown:$md}') || die "could not build the create_page arguments"
  ans=$(mcp create_page "$args") || die "create_page failed: $ans"
  seed_log create_page "$ans"
  pid=$(printf '%s' "$ans" | dxi jq -e '.id') || die "create_page gave no id"
  ans=$(mcp publish_page "{\"id\":$pid,\"status\":\"published\"}") || die "publish_page failed: $ans"
  seed_log publish_page "$ans"

  # One image, so that more than /healthz runs under the syscall filter
  # (Pitfall 7). The build context excludes sites/, so it comes in by docker cp.
  docker cp "$ROOT/sites/beispiel/media/velo-01.jpg" "$NAME:/tmp/hc-upload.jpg" || die "docker cp of the test image failed"
  ans=$(mcp create_upload_link "{\"website\":$wid,\"file_name\":\"velo-01.jpg\",\"alt_text\":\"Testbild fuer den Unit-Test\"}") \
    || die "create_upload_link failed: $ans"
  # The link is a one-time secret; keep it out of seed.txt.
  seed_log create_upload_link "$(printf '%s' "$ans" | dxi jq 'del(.upload_url, .example)')"
  local url
  url=$(printf '%s' "$ans" | dxi jq -er '.upload_url') || die "create_upload_link gave no upload_url"
  # HOLZCLOUD_SECURE=true makes the link https even over plain HTTP.
  url="http:${url#https:}"
  ans=$(dx curl --fail -sS --max-time 120 --upload-file /tmp/hc-upload.jpg "$url") || die "uploading the image failed: $ans"
  seed_log upload "$ans"

  # The thumbnail variant (width 400) is always kept, internal/media/variants.go.
  local i=1 thumbs=""
  while [ "$i" -le 20 ]; do
    thumbs=$(dx find /opt/holzcloud/data -name '*-thumb*' 2>/dev/null || true)
    [ -z "$thumbs" ] || break
    sleep 1
    i=$((i + 1))
  done
  [ -n "$thumbs" ] || die "no thumbnail variant appeared under /opt/holzcloud/data"
  seed_log "thumbnail" "$thumbs"
  say "SEED OK website=$wid page=$pid"
}

# Reads the unit's journal since the current start, so the first, crashed start
# of a Rosetta run stays out of the stability check.
journal_since() { dx journalctl -u holzcloud --since "$1" -o cat --no-pager; }

# The same, narrowed to this one invocation of the service. --since has a
# resolution of one second and lets in a line of the previous invocation when
# the restart happened inside that second; for "plugin loaded" that would be a
# false pass.
journal_invocation() {
  dx journalctl "_SYSTEMD_INVOCATION_ID=$2" --since "$1" -o cat --no-pager
}

check_pass() { printf 'PASS %s\n' "$*" | tee -a "$CHECK_OUT"; }
check_bad() { printf 'BAD  %s\n' "$*" | tee -a "$CHECK_OUT"; CHECK_BADS=$((CHECK_BADS + 1)); }

# http_code PATH [QUERY]: status code for test.local.
http_code() { dx curl -s -o /dev/null -w '%{http_code}' -H 'Host: test.local' "http://127.0.0.1:8080$1"; }
http_body() { dx curl -s -H 'Host: test.local' "http://127.0.0.1:8080$1"; }

# run_checks OUTFILE: the unit has just been (re)started and is ready. Prints
# every result to the screen and to OUTFILE; fails after printing all of them.
run_checks() {
  CHECK_OUT=$1
  CHECK_BADS=0
  : > "$CHECK_OUT"

  local since jlog id plugins year page search code
  since=$(dx systemctl show -p ActiveEnterTimestamp --value holzcloud)
  [ -n "$since" ] || die "no ActiveEnterTimestamp: the unit is not active"
  jlog=$(journal_invocation "$since" "$(dx systemctl show -p InvocationID --value holzcloud)")
  local jload=$jlog

  # 1-5: loaded, 6: nothing failed to load.
  for id in bestellung jahreszahl kontaktformular nicht-gefunden suche; do
    if grep 'plugin loaded' <<< "$jlog" | grep -q "\"plugin\":\"$id\""; then
      check_pass "loaded $id"
    else
      check_bad "loaded $id: no 'plugin loaded' line for it since $since"
    fi
  done
  if grep -q 'plugin not loaded' <<< "$jlog"; then
    check_bad "no-load-failures: $(grep 'plugin not loaded' <<< "$jlog" | head -n 3)"
  else
    check_pass "no-load-failures"
  fi

  # 7-11: running, no error.
  plugins=$(mcp list_plugins '{}') || plugins=""
  for id in bestellung jahreszahl kontaktformular nicht-gefunden suche; do
    if printf '%s' "$plugins" | dxi jq -e --arg id "$id" \
        '.plugins[] | select(.id == $id) | (.running == true) and (.last_error == "")' >/dev/null 2>&1; then
      check_pass "running $id"
    else
      check_bad "running $id: list_plugins does not show it running with an empty last_error"
    fi
  done

  # 12-15: invoked over HTTP.
  year=$(dx date +%Y)
  page=$(http_body /plugintest)
  if grep -q "Jahr: $year" <<< "$page" && ! grep -q '\[\[jahr\]\]' <<< "$page"; then
    check_pass "invoke jahreszahl"
  else
    check_bad "invoke jahreszahl: 'Jahr: $year' missing from /plugintest or the shortcode is still there"
  fi
  code=$(http_code /formular)
  if grep -q 'contact-form' <<< "$page" && [ "$code" = 303 ]; then
    check_pass "invoke kontaktformular"
  else
    check_bad "invoke kontaktformular: contact-form on the page and GET /formular 303 expected, got $code"
  fi
  code=$(http_code /bestellung)
  if grep -q 'class="bestellung"' <<< "$page" && [ "$code" = 303 ]; then
    check_pass "invoke bestellung"
  else
    check_bad "invoke bestellung: class=\"bestellung\" on the page and GET /bestellung 303 expected, got $code"
  fi
  search=$(http_body '/suche?q=Plugintest')
  if grep -q 'href="/plugintest"' <<< "$search"; then
    check_pass "invoke suche"
  else
    check_bad "invoke suche: no result link to /plugintest"
  fi

  # 16: the 404 hook stores a row.
  code=$(http_code /gibts-nicht)
  local i=1 rows=0
  if [ "$code" = 404 ]; then
    while [ "$i" -le 10 ]; do
      rows=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
        "select count(*) from plugin_store where plugin_id='nicht-gefunden' and key like 'pfad:/gibts-nicht%'" 2>/dev/null || echo 0)
      [ "${rows:-0}" -le 0 ] || break
      sleep 1
      i=$((i + 1))
    done
  fi
  if [ "$code" = 404 ] && [ "${rows:-0}" -gt 0 ]; then
    check_pass "invoke nicht-gefunden"
  else
    check_bad "invoke nicht-gefunden: GET /gibts-nicht gave $code (404 expected), plugin_store rows: ${rows:-0}"
  fi

  # 17: nothing was restarted or killed. A container has no auditd, so these
  # are the substitute for the seccomp kill a server would log (Pitfall 7).
  local restarts result
  restarts=$(dx systemctl show -p NRestarts --value holzcloud)
  result=$(dx systemctl show -p Result --value holzcloud)
  jlog=$(journal_since "$since")
  # (the unit's journal since the start, systemd's own lines about it included)
  if [ "$restarts" = 0 ] && [ "$result" = success ] \
      && ! grep -Eq 'code=killed|status=31|SIGSYS' <<< "$jlog"; then
    check_pass "stable"
  else
    check_bad "stable: NRestarts=$restarts Result=$result, or a kill signature in the journal"
  fi

  # Informational lines: they never start with PASS and never fail the run.
  local mainpid n
  mainpid=$(dx systemctl show -p MainPID --value holzcloud)
  # Anonymous executable mappings: field 2 has x, field 6 is empty or a name the
  # Go runtime gave its own mapping ([anon:...] on newer kernels). wazero's
  # compiler leaves such mappings, its interpreter none. No code change needed.
  n=$(dx awk '$2 ~ /x/ && ($6 == "" || $6 ~ /^\[anon/) { c++ } END { print c + 0 }' "/proc/$mainpid/maps")
  {
    if [ "$n" -eq 0 ]; then
      echo "engine: interpreter (0 anonymous executable mappings)"
    else
      echo "engine: compiler ($n anonymous executable mappings)"
    fi
    # The hook budget is CallTimeout = 2 * time.Second in internal/plugin/runtime.go.
    echo "# hook budget CallTimeout is 2 s (internal/plugin/runtime.go); the medians are of ten requests"
    local p med
    for p in /plugintest '/suche?q=Plugintest' /formular /bestellung; do
      med=$(dx bash -c 'for i in 1 2 3 4 5 6 7 8 9 10; do curl -s -o /dev/null -w "%{time_total}\n" -H "Host: test.local" "http://127.0.0.1:8080$1"; done | sort -n | sed -n 5p' _ "$p")
      echo "LATENCY $p median_s=$med n=10"
    done
    dx grep '^Seccomp' "/proc/$mainpid/status" || true
    echo "NRestarts=$restarts"
    echo "Result=$result"
    grep 'plugin loaded' <<< "$jload" || true
  } | tee -a "$CHECK_OUT"

  if [ "$CHECK_BADS" -gt 0 ]; then
    say "run.sh: $CHECK_BADS assertion(s) failed, see BAD lines"
    return 1
  fi
  return 0
}

cmd_check() {
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh start' first"
  dx systemctl restart holzcloud
  wait_ready || exit 1
  run_checks "$OUT_DIR/check.txt" || exit 1
  say "CHECK OK"
}

# The drop-in is the only thing this command changes, and only inside the
# container. mdwe_restore is what on_exit calls too, so a failed control run
# cannot leave the unit weakened (T-20-06).
MDWE_DROPIN_FILE=/etc/systemd/system/holzcloud.service.d/mdwe-control.conf

mdwe_restore() {
  dx rm -f "$MDWE_DROPIN_FILE" || return 1
  dx systemctl daemon-reload || return 1
  dx systemctl reset-failed holzcloud || true
  dx systemctl restart holzcloud || return 1
  wait_ready || return 1
  local v
  v=$(dx systemctl show -p MemoryDenyWriteExecute --value holzcloud)
  if [ "$v" != yes ]; then
    say "run.sh: MemoryDenyWriteExecute is '$v' after the restore, expected yes"
    return 1
  fi
  MDWE_DROPIN=0
  say "RESTORED"
}

cmd_mdwe_control() {
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh start' first"
  dx test -e /root/.hc-key || die "the container is not seeded; run 'run.sh seed' first"

  MDWE_DROPIN=1
  local rc=0
  dx sh -c "mkdir -p /etc/systemd/system/holzcloud.service.d && printf '[Service]\nMemoryDenyWriteExecute=no\n' > $MDWE_DROPIN_FILE" || rc=1
  if [ "$rc" -eq 0 ]; then
    dx systemctl daemon-reload || rc=1
    dx systemctl reset-failed holzcloud || true
    dx systemctl restart holzcloud || rc=1
    wait_ready || rc=1
  fi
  if [ "$rc" -eq 0 ]; then
    say "control: MemoryDenyWriteExecute=no is $(dx systemctl show -p MemoryDenyWriteExecute --value holzcloud) in the manager"
    run_checks "$OUT_DIR/mdwe-control.txt" || rc=1
    say "MDWE-CONTROL: engine=$(sed -n 's/^engine: \([a-z]*\).*/\1/p' "$OUT_DIR/mdwe-control.txt" 2>/dev/null)"
  fi

  # Restore on the normal path; on_exit does the same if this shell dies first.
  mdwe_restore || exit 1
  [ "$rc" -eq 0 ] || exit 1
}

count_pass() { grep -c '^PASS' "$1" 2>/dev/null || true; }
engine_of() { sed -n 's/^engine: \([a-z]*\).*/\1/p' "$1" 2>/dev/null | head -n 1; }

# The MDWE verdict. It is SETTLED only where the canary showed that seccomp is
# enforced; anywhere else it is VOID and the exit code is 3, so a Rosetta run can
# never be quoted as MDWE evidence (T-20-07).
cmd_verdict() {
  [ -f "$OUT_DIR/canary.verdict" ] || die "no $OUT_DIR/canary.verdict; run 'run.sh canary' first"
  local canary yes_n ctl_n yes_engine ctl_engine msg rc
  canary=$(cat "$OUT_DIR/canary.verdict")
  yes_n=$(count_pass "$OUT_DIR/check.txt")
  ctl_n=$(count_pass "$OUT_DIR/mdwe-control.txt")
  yes_engine=$(engine_of "$OUT_DIR/check.txt")
  ctl_engine=$(engine_of "$OUT_DIR/mdwe-control.txt")
  yes_n=${yes_n:-0}
  ctl_n=${ctl_n:-0}

  if [ "$canary" != enforced ]; then
    msg="MDWE VERDICT: VOID - seccomp is not enforced on this platform ($PLATFORM), so the plugin results prove nothing about MemoryDenyWriteExecute; ${yes_n}/17 assertions passed, engine under the shipped unit: ${yes_engine:-unknown}"
    rc=3
  elif [ "$yes_n" -lt 17 ]; then
    msg="MDWE VERDICT: FAILS - seccomp is enforced and only ${yes_n}/17 assertions passed under MemoryDenyWriteExecute=true: the plugins do not load under the shipped unit"
    rc=1
  elif [ "$yes_engine" != interpreter ] || [ "$ctl_n" -lt 17 ]; then
    msg="MDWE VERDICT: INCONSISTENT - seccomp is enforced, but the engine under MemoryDenyWriteExecute=true is '${yes_engine:-unknown}' (interpreter expected) and the control passed ${ctl_n}/17: the probe or the harness is unreliable"
    rc=1
  else
    msg="MDWE VERDICT: SETTLED - MemoryDenyWriteExecute=true is enforced here (canary: enforced), all five plugins loaded and ran on wazero's interpreter fallback (${yes_n}/17 assertions), the control with the directive off ran on the ${ctl_engine:-unknown} engine (${ctl_n}/17), and the unit stays unchanged"
    rc=0
  fi
  echo "$msg" > "$OUT_DIR/verdict.txt"
  echo "$msg"
  exit "$rc"
}

# The narrow run. The canary may end in 3 (not enforced) without stopping it:
# the verdict then says VOID and carries the exit code 3.
cmd_mdwe() {
  CLEANUP_DOWN=1
  local crc=0
  cmd_build
  cmd_up
  cmd_start
  ( cmd_canary ) || crc=$?
  case "$crc" in 0 | 3) ;; *) die "the canary failed (exit $crc)" ;; esac
  cmd_seed
  cmd_check
  cmd_mdwe_control
  ( cmd_verdict ) || exit $?
}

# UNIT-03: the score of the unit exactly as shipped. Run it before `start`: a
# failed first start under Rosetta adds a container-only drop-in, and the
# measurement must not include it. The files hold the tool's output and nothing
# else; raw is the evidence. No pager and no terminal, so no colour codes.
cmd_score() {
  dx test -e /etc/systemd/system/holzcloud.service \
    || die "no unit in the container; run 'run.sh build up' first"
  mkdir -p "$OUT_DIR"
  # A non-zero exit of the tool must not hide the output; the check below
  # decides whether anything was measured.
  dx systemd-analyze security holzcloud --no-pager > "$OUT_DIR/score-online.txt" || true
  dx systemd-analyze security --offline=true --no-pager /etc/systemd/system/holzcloud.service \
    > "$OUT_DIR/score-offline.txt" || true
  dx systemctl cat holzcloud > "$OUT_DIR/score-unit.txt" || true

  local f
  for f in score-online.txt score-offline.txt; do
    grep -q 'Overall exposure level for' "$OUT_DIR/$f" \
      || die "$OUT_DIR/$f has no 'Overall exposure level for' line: nothing was measured"
  done
  say "online:  $(grep 'Overall exposure level for' "$OUT_DIR/score-online.txt")"
  say "offline: $(grep 'Overall exposure level for' "$OUT_DIR/score-offline.txt")"
}

# The whole recipe. The order matters: score before start, so the measurement is
# of the unit as shipped. A canary or verdict that says 3 (VOID) does not fail
# the run; any other failure does.
cmd_all() {
  CLEANUP_DOWN=1
  local crc=0 vrc=0
  cmd_build
  cmd_up
  cmd_score
  cmd_start
  assert_http
  ( cmd_canary ) || crc=$?
  case "$crc" in 0 | 3) ;; *) die "the canary failed (exit $crc)" ;; esac
  cmd_seed
  cmd_check
  cmd_mdwe_control
  ( cmd_verdict ) || vrc=$?
  case "$vrc" in 0 | 3) ;; *) die "the MDWE verdict failed (exit $vrc)" ;; esac

  say "---- summary ($PLATFORM) ----"
  say "UNIT-01 PASS"
  say "CANARY: $(cat "$OUT_DIR/canary.verdict")"
  say "$(cat "$OUT_DIR/verdict.txt")"
  say "UNIT-03 online:  $(grep 'Overall exposure level for' "$OUT_DIR/score-online.txt")"
  say "UNIT-03 offline: $(grep 'Overall exposure level for' "$OUT_DIR/score-offline.txt")"
}

# ---------------------------------------------------------------------------
# Phase 21: the document played literally (DEP-01).
# ---------------------------------------------------------------------------

# bin_path old|new: the file for a ref. Named by ref, so a changed OLD_TAG or a
# new HEAD never reuses a stale file.
bin_path() {
  local sha
  case "$1" in
    old) printf '%s/holzcloud-old-%s\n' "$BIN_DIR" "$OLD_TAG" ;;
    new)
      sha=$(git rev-parse --short "${NEW_REF}^{commit}") || die "NEW_REF '$NEW_REF' is not a commit of this repository"
      printf '%s/holzcloud-new-%s\n' "$BIN_DIR" "$sha"
      ;;
    *) die "bin_path: old or new, not '$1'" ;;
  esac
}

# bins_build WHICH REF: a clone at REF (a clone only reads this repository; a
# git worktree would write into .git), and then the Build line of the document,
# played from the clone's directory the way an operator plays it on a dev
# machine.
bins_build() {
  local which=$1 ref=$2 dest sha line info
  dest=$(bin_path "$which") || exit 1
  mkdir -p "$BIN_DIR"
  if [ -f "$dest" ] && [ "$REBUILD" != 1 ]; then
    say "reusing $dest (REBUILD=1 builds it again)"
    [ ! -f "$dest.info" ] || cat "$dest.info" >&2
    return 0
  fi
  sha=$(git rev-parse --verify --quiet "${ref}^{commit}") || die "bins: '$ref' is not a commit of this repository"
  line=$(doc_lines "Build" 1) || exit 1

  BINS_TMP=$(mktemp -d "${TMPDIR:-/tmp}/holzcloud-bins.XXXXXX") || die "bins: no temporary directory"
  git clone -q --no-checkout "$ROOT" "$BINS_TMP/src" || die "bins: git clone failed"
  git -C "$BINS_TMP/src" checkout -q -f --detach "$sha" || die "bins: git checkout of $sha failed"
  say "building the $which binary ($ref) with the Build line of $DEPLOY_DOC: $line"
  ( cd "$BINS_TMP/src" && bash -c "$line" ) || die "bins: the Build line failed for $ref"
  [ -f "$BINS_TMP/src/holzcloud" ] || die "bins: the Build line produced no holzcloud file for $ref"
  mv "$BINS_TMP/src/holzcloud" "$dest"

  info="$dest.info"
  {
    echo "== $which: $ref =="
    echo "file:       $dest"
    echo "describe:   $(git -C "$BINS_TMP/src" describe --tags --always)"
    echo "commit:     $(git -C "$BINS_TMP/src" rev-parse --short HEAD)"
    echo "sha256:     $(shasum -a 256 "$dest" | awk '{ print $1 }')"
    echo "migrations: $(find "$BINS_TMP/src/internal/db/migrations" -name '*.sql' | wc -l | tr -d ' ')"
  } > "$info"
  cat "$info" >> "$BIN_DIR/bins.txt"
  cat "$info" >&2
  rm -rf "${BINS_TMP:?}"
  BINS_TMP=""
}

cmd_bins() {
  case "$INSTALL_FROM" in new | old) ;; *) die "INSTALL_FROM is '$INSTALL_FROM', expected new or old" ;; esac
  doc_lines "Build" 1 >/dev/null
  bins_build new "$NEW_REF"
  if [ "$INSTALL_FROM" = old ] || [ "$WITH_OLD" = 1 ]; then
    bins_build old "$OLD_TAG"
  fi
}

# install: the document's own blocks, from a container that has no user, no
# directory, no binary and no unit (TARGET=systemd-base).
cmd_install() {
  doc_preflight
  dx true >/dev/null 2>&1 || die "container $NAME does not answer; run 'run.sh build up' with TARGET=systemd-base first"
  if dx test -e /opt/holzcloud/holzcloud; then
    die "/opt/holzcloud/holzcloud exists: this is not a fresh target; TARGET=systemd-base is the starting point"
  fi
  cmd_bins
  STAGE_BIN=$(bin_path "$INSTALL_FROM") || exit 1

  mkdir -p "$OUT_DIR"
  local n0 failed unit ready bin
  n0=$(steps_count)
  # The image already holds sudo, curl, ca-certificates, jq, procps, sqlite3 and
  # openssl. The Prerequisites block is played first, as an operator would; a
  # tool of the image that the document does not name is the one thing that makes
  # a later step work here without the document saying why (DEVIATION[preinstalled]).
  local nbp plines pre_missing="" t
  nbp=$(doc_blocks "Prerequisites") || exit 1
  if [ "$nbp" -gt 0 ]; then
    doc_play "Prerequisites" || true
    plines=$(doc_lines "Prerequisites")
  else
    plines=""
  fi
  for t in ca-certificates curl jq sqlite3 openssl; do
    grep -qw -- "$t" <<< "$plines" || pre_missing="$pre_missing $t"
  done
  if [ -n "$pre_missing" ]; then
    deviation "DEVIATION[preinstalled]: the systemd-base image already holds$pre_missing, which the document's Prerequisites do not name, so a step that needs one of them works here without the document saying so"
  fi

  doc_play "Build" || true
  doc_play "Transfer to the server" || true
  doc_play "Install" || true
  # The block holds the literal systemctl start. Under amd64 emulation it ends
  # in the known crash (Type=simple returns before the process dies), and
  # cmd_start then re-issues it, recognises the signature and applies the
  # container-only drop-in (D-03, D-13).
  doc_play "systemd Service" 1 || true
  failed=$(steps_failed_since "$n0")
  if [ -n "$failed" ]; then
    say "run.sh: counted steps that failed so far:"
    printf '%s\n' "$failed" >&2
  fi

  cmd_start
  if dx test -e /etc/systemd/system/holzcloud.service.d/container.conf; then
    printf '%s\n' "DEVIATION[dropin]: /etc/systemd/system/holzcloud.service.d/container.conf sets ProcSubset=all (container only; the document's unchanged start crashed, see unit-start-unchanged.txt)" >> "$OUT_DIR/deviations.txt"
  fi
  unit=$(dx systemctl is-active holzcloud || true)
  [ "$unit" = active ] || die "the unit is '$unit', expected active"
  assert_http

  ready=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -c '{status,version,commit,integrity}') \
    || die "/readyz did not answer for install.txt"
  bin=$(dx /opt/holzcloud/holzcloud version) || die "holzcloud version failed"
  {
    echo "readyz: $ready"
    echo "binary: $bin"
    echo "unit: $unit"
  } > "$OUT_DIR/install.txt"
  cat "$OUT_DIR/install.txt"

  deviation "DEVIATION[mount]: the unit's RequiresMountsFor names /opt/holzcloud/data, a plain directory on the container's root filesystem; no separate mount exists here, so that line was not exercised"

  failed=$(steps_failed_since "$n0")
  if [ -n "$failed" ]; then
    say "run.sh: INSTALL FAILED, counted steps with a non-zero status:"
    printf '%s\n' "$failed" >&2
    exit 1
  fi
  say "INSTALL OK"
}

# ---------------------------------------------------------------------------
# First Run, Caddy and the Backups block, played on the container `install`
# left behind (D-03, D-04). Nothing here installs anything of its own.
# ---------------------------------------------------------------------------

# Runs INSIDE the container (docker exec -i NAME bash -s -- ARGS...); the shell
# text is on stdin. It is the headless form of the browser steps of First Run:
#   get  PATH JAR                           GET with the cookie jar
#   post PATH EMAIL PASSFILE JAR [confirm]  GET the form into JAR, take the hidden
#                                           CSRF field, POST email and password
#                                           (and password_confirm with a sixth word)
# The jar is created under umask 077. The password is read from PASSFILE, a
# root-only file in the container, and neither it nor the token nor a cookie is
# ever printed: the only output is one line "HTTP STATUS REDIRECT-PATH" (T-21-01).
# The unit sets HOLZCLOUD_SECURE=true, so a form over plain http is only
# accepted with a Referer that carries the https origin. html/template writes
# plus, equals and slash of the token as entities; they are undone before the post.
read -r -d '' FORM_SH <<'FORM_EOF' || true
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
  post)
    email=$3
    passfile=$4
    jar=$5
    confirm=${6:-}
    page=$(curl -s --max-time 60 -c "$jar" -b "$jar" "$base$path")
    tok=$(printf '%s' "$page" | grep -o 'name="gorilla.csrf.Token" value="[^"]*"' | head -n 1 \
      | sed 's/.*value="//; s/"$//; s/&#43;/+/g; s/&#61;/=/g; s/&#47;/\//g')
    if [ -z "$tok" ]; then
      echo "HTTP none no-csrf-field"
      exit 0
    fi
    pass=$(cat "$passfile")
    if [ -n "$confirm" ]; then
      res=$(curl -s -o /dev/null --max-time 120 -w '%{http_code} %{redirect_url}' -c "$jar" -b "$jar" \
        -H "Referer: https://127.0.0.1:8080$path" \
        --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "email=$email" \
        --data-urlencode "password=$pass" --data-urlencode "password_confirm=$pass" "$base$path")
    else
      res=$(curl -s -o /dev/null --max-time 120 -w '%{http_code} %{redirect_url}' -c "$jar" -b "$jar" \
        -H "Referer: https://127.0.0.1:8080$path" \
        --data-urlencode "gorilla.csrf.Token=$tok" --data-urlencode "email=$email" \
        --data-urlencode "password=$pass" "$base$path")
    fi
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

# form ARGS...: FORM_SH in the container; prints "HTTP STATUS REDIRECT-PATH".
form() { printf '%s\n' "$FORM_SH" | dxi bash -s -- "$@"; }

# leak_check: no secret of this container, and nothing shaped like an AI key,
# may occur in a file under OUT_DIR. Called at the end of every subcommand that
# touches a credential (T-21-01).
leak_check() {
  local f secret hit=0
  mkdir -p "$OUT_DIR"
  for f in /root/.hc-admin-pass /root/.hc-login-pass /root/.hc-key; do
    secret=$(dx cat "$f" 2>/dev/null) || continue
    # A value this short would match unrelated text and prove nothing.
    [ "${#secret}" -ge 8 ] || continue
    if grep -rqF -- "$secret" "$OUT_DIR"; then
      say "LEAK: the contents of $f occur in a file under $OUT_DIR"
      hit=1
    fi
  done
  if grep -rEq 'hc_[A-Za-z0-9_-]{20,}' "$OUT_DIR"; then
    say "LEAK: a string shaped like an AI key occurs in a file under $OUT_DIR"
    hit=1
  fi
  [ "$hit" -eq 0 ] || exit 1
  say "LEAK-CHECK OK"
}

# firstrun: the First Run section of the document. Its browser steps are played
# with curl (DEVIATION[browser]); what counts as success is D-04's list.
cmd_firstrun() {
  local nb out
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh install' first"
  if dx test -e /root/.hc-admin-pass; then
    die "this container has been through First Run already (/root/.hc-admin-pass exists); run 'run.sh down' and start again"
  fi
  mkdir -p "$OUT_DIR"
  nb=$(doc_blocks "First Run") || exit 1
  if [ "$nb" -gt 0 ]; then
    doc_play "First Run" || true
  fi
  deviation "DEVIATION[browser]: the document's First Run is played with curl and not in a browser. The shipped unit sets HOLZCLOUD_SECURE=true, so the form is only accepted as if it came through the TLS proxy: a post over plain http needs a Referer header that carries the https origin, and the hidden CSRF field is read out of the page"

  # The password is made inside the container and read only there (T-21-01).
  dx bash -c 'umask 077; openssl rand -hex 12 > /root/.hc-admin-pass; test -s /root/.hc-admin-pass' \
    || die "could not create the administrator password file"

  CHECK_OUT="$OUT_DIR/firstrun.txt"
  CHECK_BADS=0
  : > "$CHECK_OUT"

  # (1) Until an account exists, whoever reaches the form first becomes the
  # administrator (T-21-02).
  out=$(form get /admin/setup /root/.hc-jar-probe) || out="HTTP error"
  if [ "$out" = "HTTP 200 -" ]; then
    check_pass "setup form open before any account: GET /admin/setup answers 200"
  else
    check_bad "setup form before any account: GET /admin/setup expected 'HTTP 200 -', got '$out'"
  fi

  # (2)
  out=$(form post /admin/setup admin@example.test /root/.hc-admin-pass /root/.hc-jar-setup confirm) || out="HTTP error"
  if [ "$out" = "HTTP 303 /admin/" ]; then
    check_pass "setup POST creates the account: 303 to /admin/"
  else
    check_bad "setup POST expected 'HTTP 303 /admin/', got '$out'"
  fi

  # (3) The form is closed once an account exists.
  out=$(form get /admin/setup /root/.hc-jar-probe2) || out="HTTP error"
  if [ "$out" = "HTTP 404 -" ]; then
    check_pass "setup form closed afterwards: GET /admin/setup answers 404"
  else
    check_bad "setup form afterwards: GET /admin/setup expected 'HTTP 404 -', got '$out'"
  fi

  # (4) One login, with a fresh jar. Ten failures from one address would block
  # it for fifteen minutes.
  out=$(form post /admin/login admin@example.test /root/.hc-admin-pass /root/.hc-jar-login) || out="HTTP error"
  if [ "$out" = "HTTP 303 /admin/" ]; then
    check_pass "login POST: 303 to /admin/"
  else
    check_bad "login POST expected 'HTTP 303 /admin/', got '$out'"
  fi

  # (5) Two-factor enrolment is compulsory for administrators.
  out=$(form get /admin/ /root/.hc-jar-login) || out="HTTP error"
  if [ "$out" = "HTTP 303 /admin/2fa/einrichten" ]; then
    check_pass "first request after the login: 303 to /admin/2fa/einrichten (compulsory two-factor enrolment)"
  else
    check_bad "GET /admin/ with the session expected 'HTTP 303 /admin/2fa/einrichten', got '$out'"
  fi

  # The jars hold session and CSRF cookies; the password file stays for later steps.
  dx bash -c 'rm -f /root/.hc-jar-*' || true
  leak_check
  if [ "$CHECK_BADS" -gt 0 ]; then
    say "run.sh: FIRSTRUN FAILED, $CHECK_BADS assertion(s), see BAD lines"
    exit 1
  fi
  say "FIRSTRUN OK"
}

# caddy: the Caddy block of the document. Exit 0 when the configuration was
# validated, 3 when only the network or a missing prerequisite stopped that, 1
# when Caddy was there and rejected the file.
cmd_caddy() {
  local n0 last i f reason
  dx true >/dev/null 2>&1 || die "container $NAME does not answer; run 'run.sh build up install' first"
  mkdir -p "$OUT_DIR"
  n0=$(steps_count)
  doc_play "Caddy (Reverse Proxy + HTTPS)" 1 || true
  last=$(steps_count)

  : > "$OUT_DIR/caddy.txt"
  i=$((n0 + 1))
  while [ "$i" -le "$last" ]; do
    f=$(printf '%s/step-%02d.txt' "$OUT_DIR" "$i")
    {
      echo "== step $i =="
      cat "$f"
      grep "^STEP $(printf '%02d' "$i") " "$(steps_file)" || true
      echo
    } >> "$OUT_DIR/caddy.txt"
    i=$((i + 1))
  done

  if [ -n "$(awk -v n="$n0" '$1 == "STEP" && ($2 + 0) > n && index($0, "systemctl reload caddy") > 0 && $3 == "rc=0"' "$(steps_file)")" ]; then
    echo "CADDY: validated" | tee -a "$OUT_DIR/caddy.txt"
    return 0
  fi
  if grep -q 'DEVIATION\[network\]' "$OUT_DIR/deviations.txt" 2>/dev/null; then
    reason="no route to the package repository"
  elif ! dx bash -c 'command -v caddy' >/dev/null 2>&1; then
    reason="caddy is not installed after the document's own steps; see the failed steps above"
  else
    echo "CADDY: failed (caddy is installed and did not validate the Caddyfile; see the step output above)" | tee -a "$OUT_DIR/caddy.txt"
    exit 1
  fi
  echo "CADDY: not run ($reason)" | tee -a "$OUT_DIR/caddy.txt"
  exit 3
}

# postinstall: the Backups block, then the audit of the document against the
# installed system (paths.txt): every deploy path the document names must exist.
cmd_postinstall() {
  local n0 failed p tool
  dx test -e /opt/holzcloud/holzcloud || die "nothing is installed in container $NAME; run 'run.sh build up install' first"
  mkdir -p "$OUT_DIR"
  n0=$(steps_count)
  doc_play "Backups" 1 || true
  failed=$(steps_failed_since "$n0")

  CHECK_OUT="$OUT_DIR/paths.txt"
  CHECK_BADS=0
  : > "$CHECK_OUT"
  local all paths
  all=$(doc_awk "" 0) || die "reading $DEPLOY_DOC failed"
  # Paths of the form /opt/holzcloud/deploy/NAME in any bash block; a value the
  # reader fills in, or a pattern, is no path.
  paths=$(grep -Eo '/opt/holzcloud/deploy/[^][:space:]"'"'"'|;&)(]*' <<< "$all" | grep -v '[<*$]' | sort -u || true)
  # A name in braces (holzcloud-backup.{service,timer}) is two files; expand it
  # the way the shell that plays the line does, but only for the plain shape.
  local expanded="" q
  for q in $paths; do
    if printf '%s' "$q" | grep -Eq '^[A-Za-z0-9/._-]*\{[A-Za-z0-9._,-]+\}[A-Za-z0-9/._-]*$'; then
      expanded="$expanded $(bash -c "printf '%s ' $q")"
    else
      expanded="$expanded $q"
    fi
  done
  paths=$expanded
  for p in /opt/holzcloud/holzcloud /opt/holzcloud/data /opt/holzcloud/backups $paths; do
    if dx test -e "$p"; then
      printf 'PASS exists %s\n' "$p" | tee -a "$CHECK_OUT"
      case "$p" in
        *.sh)
          if ! dx test -x "$p"; then
            printf 'BAD not executable %s\n' "$p" | tee -a "$CHECK_OUT"
            CHECK_BADS=$((CHECK_BADS + 1))
          fi
          ;;
      esac
    else
      printf 'BAD missing %s\n' "$p" | tee -a "$CHECK_OUT"
      CHECK_BADS=$((CHECK_BADS + 1))
    fi
  done
  # Informational: which tools the install had at hand. Never fails the run.
  {
    for tool in rsync gpg fuser curl jq sqlite3 openssl; do
      if dx bash -c 'command -v "$1" >/dev/null' _ "$tool"; then
        echo "TOOL $tool present"
      else
        echo "TOOL $tool absent"
      fi
    done
  } | tee -a "$CHECK_OUT"

  if [ -n "$failed" ]; then
    say "run.sh: counted steps of the Backups block that failed:"
    printf '%s\n' "$failed" >&2
  fi
  if [ "$CHECK_BADS" -gt 0 ] || [ -n "$failed" ]; then
    say "run.sh: POSTINSTALL FAILED, $CHECK_BADS path problem(s) and $(printf '%s\n' "$failed" | grep -c . || true) failed step(s)"
    exit 1
  fi
  say "POSTINSTALL OK"
}

# ---------------------------------------------------------------------------
# Phase 21: an installation of the old version, then the Updating section
# played exactly as it stands (DEP-02). `baseline` and `update` are separable
# (D-12); the chain is: run.sh build up install baseline update, with
# INSTALL_FROM=old so the old install is itself played out of the document.
# ---------------------------------------------------------------------------

# stage_fixtures: seed reads the plugin archives from /opt/holzcloud/plugin-zips,
# a directory the systemd-base image does not have. They, the test image, the
# login user and the order row stand in for operator work in the admin and are
# no steps of the document.
stage_fixtures() {
  local id
  deviation "DEVIATION[fixture]: the plugin archives, the test image, the login user and the order row are test fixtures that stand in for operator work in the admin, not steps of the document"
  dx mkdir -p /opt/holzcloud/plugin-zips || die "could not create /opt/holzcloud/plugin-zips in the container"
  for id in bestellung jahreszahl kontaktformular nicht-gefunden suche; do
    [ -f "$ROOT/plugins/$id/$id.zip" ] || die "plugins/$id/$id.zip is missing in the working tree"
    docker cp "$ROOT/plugins/$id/$id.zip" "$NAME:/opt/holzcloud/plugin-zips/$id.zip" \
      || die "docker cp of plugins/$id/$id.zip failed"
  done
}

# insert_old_order NUMBER: one order row, written straight into the database.
# The statement names the columns of the INSERT in internal/shop/order.go at
# v0.0.10 (the shop flow itself would need a browser) and, like that code, does
# not name the token column, which does not exist at schema 58; at a tag whose
# schema already has the column the omission leaves it empty. Amounts are in the
# smallest unit of the currency (CHF 100.00 with 8.1 percent tax inside).
# old_order_run NUMBER leaves the outcome to the caller (the negative case wants the
# error text); insert_old_order NUMBER dies when the INSERT fails.
old_order_run() {
  local number=$1 wid
  case "$number" in *[!0-9A-Za-z-]* | "") die "insert_old_order: odd order number '$number'" ;; esac
  wid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
    "select id from websites order by id limit 1") || die "insert_old_order: could not read the website id"
  [ -n "$wid" ] || die "insert_old_order: there is no website to attach the order to"
  printf '%s\n' \
    ".timeout 5000" \
    "INSERT INTO orders (website_id, number, audience, currency, email, name, company, vat_number, phone, street, postal_code, city, country, note, items_net, items_tax, items_gross, shipping_net, shipping_tax, shipping_gross, total_net, total_tax, total_gross, vat_exempt, status, payment_method, payment_status, return_policy, created_at, updated_at) VALUES ($wid, '$number', 'private', 'CHF', 'kunde@example.test', 'Test Kunde', '', '', '', 'Teststrasse 1', '8000', 'Zuerich', 'CH', '', 9250, 750, 10000, 0, 0, 0, 9250, 750, 10000, 0, 'new', 'invoice', 'open', '', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));" \
    | dxi runuser -u holzcloud -- sqlite3 /opt/holzcloud/data/holzcloud.sqlite
}

insert_old_order() {
  old_order_run "$1" || die "insert_old_order: the INSERT failed"
}

# Runs INSIDE the container as the service user (runuser -u holzcloud -- bash -s -- ORDER SOURCE):
# counts and content hashes only, and only through a read-only connection. No row
# of any table is selected, the users, sessions and key tables included (T-21-07,
# T-21-09), and no holzcloud subcommand is started, because every one of them
# migrates silently (T-21-08). The only values read out of a content table are
# the page text and the stored media file name, and they leave this script as
# sha256 digests and a length.
read -r -d '' STATE_SH <<'STATE_EOF' || true
set -eu
db=/opt/holzcloud/data/holzcloud.sqlite
order=${1:-}
src=${2:-/tmp/hc-upload.jpg}
case "$order" in *[!0-9A-Za-z-]*) order="" ;; esac
q() { sqlite3 -readonly "$db" ".timeout 5000" "$1"; }
sha() { sha256sum | cut -d' ' -f1; }
echo "schema: $(q "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1")"
q "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name" | while IFS= read -r t; do
  n=$(q "select count(*) from \"$t\"" 2>/dev/null) || n="?"
  echo "table $t $n"
done
# The page: the stored Markdown and the stored HTML of the slug plugintest.
if [ "$(q "select count(*) from pages where slug = 'plugintest'")" = 1 ]; then
  pmd=$(q "select content_markdown from pages where slug = 'plugintest'" | sha)
  phtml=$(q "select content_html from pages where slug = 'plugintest'" | sha)
  echo "page: $pmd $phtml"
else
  echo "page: none"
fi
# The media file: the original that the upload stored, next to the file that
# seed staged. The scaled variants are not hashed.
mrow=$(q "select website_id || '/' || filename from media order by id limit 1")
mh=none
if [ -n "$mrow" ] && [ -f "/opt/holzcloud/data/media/$mrow" ]; then
  mh=$(sha < "/opt/holzcloud/data/media/$mrow")
fi
sh=""
if [ -r "$src" ]; then sh=$(sha < "$src"); fi
echo "media: $mh source ${sh:-none}"
# The order: whether the schema has the token column, and the stored length.
if [ "$(q "select count(*) from pragma_table_info('orders') where name = 'token'")" = 1 ]; then
  len=$(q "select length(token) from orders where number = '$order'")
  echo "order: ${order:-none} ${len:-missing}"
else
  echo "order: ${order:-none} no-token-column"
fi
echo "owner: $(stat -c '%U %a' "$db")"
STATE_EOF

# order_number: the order the baseline inserted, from the variable or baseline.txt.
ORDER_NO=${ORDER_NO:-}
order_number() {
  if [ -n "$ORDER_NO" ]; then
    printf '%s\n' "$ORDER_NO"
  else
    sed -n 's/^ORDER: \([0-9A-Za-z-]*\).*/\1/p' "$OUT_DIR/baseline.txt" 2>/dev/null | head -n 1
  fi
}

# capture_state LABEL: OUT_DIR/state-LABEL.txt as KEY: value lines. identity is
# the /readyz answer reduced to status, version, commit and integrity; binary the
# `holzcloud version` line (it opens no database); schema the highest applied
# goose version; one "table NAME N" line per table of sqlite_master, nothing
# hand-listed; page, media, order and owner from STATE_SH; public the status of
# /plugintest and / for test.local and whether the page title is in the body;
# plugins the number of plugins that list_plugins shows running with an empty
# last_error; login the result of one login for login@example.test, status and
# redirect path only. Counts and content hashes, never a row of a credential
# table (T-21-09).
capture_state() {
  local label=$1 f ident ver c1 c2 title body pl np lg
  f="$OUT_DIR/state-$label.txt"
  mkdir -p "$OUT_DIR"
  ident=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -c '{status,version,commit,integrity}') \
    || die "capture_state $label: /readyz did not answer"
  ver=$(dx /opt/holzcloud/holzcloud version) || die "capture_state $label: holzcloud version failed"
  {
    echo "identity: $ident"
    echo "binary: $ver"
    printf '%s\n' "$STATE_SH" | dxi runuser -u holzcloud -- bash -s -- "$(order_number)" /tmp/hc-upload.jpg
  } > "$f" || die "capture_state $label: reading the database failed"
  grep -q '^table ' "$f" || die "capture_state $label: no table line in $f"

  c1=$(http_code /plugintest) || c1=error
  c2=$(http_code /) || c2=error
  # The body goes into a variable first: grep -q ends early and under pipefail
  # the curl in front of it would then count as a failure.
  body=$(http_body /plugintest) || body=""
  title=no
  if grep -q '<title>Plugintest' <<< "$body"; then title=yes; fi
  pl=$(mcp list_plugins '{}') || pl=""
  np=$(printf '%s' "$pl" | dxi jq '[.plugins[] | select(.running == true and .last_error == "")] | length' 2>/dev/null) || np=0
  lg=$(form post /admin/login login@example.test /root/.hc-login-pass /root/.hc-jar-state) || lg="HTTP error"
  dx rm -f /root/.hc-jar-state || true
  {
    echo "public: plugintest=$c1 root=$c2 title=$title"
    echo "plugins: ${np:-0}"
    echo "login: $lg"
  } >> "$f"
  say "state $label captured in $f ($(grep -c '^table ' "$f") tables)"
}

# ---------------------------------------------------------------------------
# The comparison (D-06). A comparison that cannot fail proves nothing, so
# compare_selftest alters a copy of a state file and requires a BAD.
# ---------------------------------------------------------------------------

# Tables whose rows move with traffic or with the update itself. Each entry has
# its reason beside it; a difference in a table that is not listed here is a BAD
# and an investigation, never a new entry.
is_volatile() {
  case "$1" in
    sessions) return 0 ;;          # one row per login, deleted at expiry: the capture itself logs in
    activity_log) return 0 ;;      # the audit trail, one row per admin action; logging in and the marker page add rows
    mail_outbox) return 0 ;;       # the mail queue: a background run drains it and the rows stay only for a while
    outbox) return 0 ;;            # the shop's queue of messages, drained by a background run
    plugin_store) return 0 ;;      # written by the plugins at run time (the 404 hook stores the missing path)
    goose_db_version) return 0 ;;  # one row per applied migration, so it grows by exactly the migrations of the update
    oauth_codes) return 0 ;;       # an authorisation code lives for minutes and is used once
  esac
  return 1
}

# st_get FILE KEY: the text after "KEY: " on the first such line.
st_get() { sed -n "s/^$2: //p" "$1" | head -n 1; }
# st_count FILE TABLE: the count on the "table TABLE N" line, empty when there is none.
st_count() { awk -v t="$2" '$1 == "table" && $2 == t { print $3; exit }' "$1"; }

CMP_OUT=""
CMP_BADS=0
cmp_pass() { printf 'PASS %s\n' "$*" | tee -a "$CMP_OUT" >&2; }
cmp_info() { printf 'INFO %s\n' "$*" | tee -a "$CMP_OUT" >&2; }
cmp_bad() { printf 'BAD %s\n' "$*" | tee -a "$CMP_OUT" >&2; CMP_BADS=$((CMP_BADS + 1)); }

# compare_states A B OUTFILE: A and B are state files. PASS and BAD lines go to
# OUTFILE and to the screen; the exit status is 1 when any BAD.
compare_states() {
  local a=$1 b=$2 t na nb va vb ma mb mha msrc la lb pa pb ta tb oa ob tla tlb bina binb
  [ -f "$a" ] && [ -f "$b" ] || die "compare_states: $a and $b must both be state files"
  CMP_OUT=$3
  CMP_BADS=0
  mkdir -p "$(dirname "$CMP_OUT")"
  : > "$CMP_OUT"
  printf '%s\n' "# compare-states A=$a B=$b" >> "$CMP_OUT"

  while IFS=' ' read -r _ t na; do
    [ -n "$t" ] || continue
    nb=$(st_count "$b" "$t")
    if [ -z "$nb" ]; then
      cmp_info "a-only $t a=$na"
    elif is_volatile "$t"; then
      cmp_info "volatile $t a=$na b=$nb"
    elif [ "$na" = "?" ] || [ "$nb" = "?" ]; then
      cmp_bad "rows $t a=$na b=$nb (a count could not be read)"
    elif [ "$na" = "$nb" ]; then
      cmp_pass "rows $t"
    else
      cmp_bad "rows $t a=$na b=$nb"
    fi
  done <<< "$(grep '^table ' "$a" || true)"
  while IFS=' ' read -r _ t nb; do
    [ -n "$t" ] || continue
    [ -n "$(st_count "$a" "$t")" ] || cmp_info "new-only $t b=$nb"
  done <<< "$(grep '^table ' "$b" || true)"

  pa=$(st_get "$a" page)
  pb=$(st_get "$b" page)
  if [ -n "$pa" ] && [ "$pa" != none ] && [ "$pa" = "$pb" ]; then
    cmp_pass "page sha256"
  else
    cmp_bad "page sha256 a='$pa' b='$pb'"
  fi

  ma=$(st_get "$a" media)
  mb=$(st_get "$b" media)
  mha=${ma%% *}
  msrc=${ma##* }
  if [ -n "$ma" ] && [ "$mha" != none ] && [ "$ma" = "$mb" ] && [ "$mha" = "$msrc" ]; then
    cmp_pass "media sha256"
  else
    cmp_bad "media sha256 a='$ma' b='$mb' (the stored file must equal the staged source)"
  fi

  la=$(st_get "$a" login)
  lb=$(st_get "$b" login)
  if [ "$la" = "HTTP 303 /admin/" ] && [ "$lb" = "HTTP 303 /admin/" ]; then
    cmp_pass "login"
  else
    cmp_bad "login a='$la' b='$lb' (expected 'HTTP 303 /admin/' twice)"
  fi

  va=$(st_get "$a" public)
  vb=$(st_get "$b" public)
  if [ "$va" = "plugintest=200 root=200 title=yes" ] && [ "$vb" = "$va" ]; then
    cmp_pass "public page"
  else
    cmp_bad "public page a='$va' b='$vb'"
  fi

  ta=$(st_get "$a" plugins)
  tb=$(st_get "$b" plugins)
  if [ "$ta" = 5 ] && [ "$tb" = 5 ]; then
    cmp_pass "plugins running 5/5"
  else
    cmp_bad "plugins running a=${ta:-?} b=${tb:-?} (5 expected)"
  fi

  oa=$(st_get "$a" order)
  ob=$(st_get "$b" order)
  tla=${oa##* }
  tlb=${ob##* }
  if [ -z "$oa" ] || [ -z "$ob" ]; then
    cmp_bad "order token: no order line (a='$oa' b='$ob')"
  elif [ "$tla" = no-token-column ] && [ "$tlb" = no-token-column ]; then
    cmp_pass "order token (no token column in either schema, nothing migrated)"
  elif [ "$tla" = no-token-column ] && [ "$tlb" = 32 ]; then
    cmp_pass "order token (the migration backfilled a 32-character token)"
  elif [ "$tla" != no-token-column ] && [ "$tla" = "$tlb" ]; then
    cmp_pass "order token (length $tla in both, as it was)"
  else
    cmp_bad "order token a='$oa' b='$ob'"
  fi

  bina=$(st_get "$a" binary)
  binb=$(st_get "$b" binary)
  cmp_info "binary a='$bina' b='$binb'"
  cmp_info "schema a=$(st_get "$a" schema) b=$(st_get "$b" schema)"

  [ "$CMP_BADS" -eq 0 ]
}

# compare_selftest A B: alters a copy of B (one table count, the page hash) and
# demands that compare_states says BAD for both, so a clean result means something.
compare_selftest() {
  local a=$1 b=$2 tmp out
  tmp="$OUT_DIR/.state-tampered.txt"
  out="$OUT_DIR/compare-selftest.txt"
  awk '$1 == "table" && $2 == "pages" { $3 = $3 + 1 } $1 == "page:" { $2 = "0000" $2 } { print }' "$b" > "$tmp"
  if compare_states "$a" "$tmp" "$out" 2>/dev/null; then
    rm -f "$tmp"
    die "compare_selftest: an altered state file was NOT reported; the comparison cannot fail"
  fi
  rm -f "$tmp"
  grep -q '^BAD rows pages' "$out" && grep -q '^BAD page sha256' "$out" \
    || die "compare_selftest: the altered copy did not produce both expected BAD lines, see $out"
  printf '%s\n' "# SELFTEST OK: the BAD lines above come from a deliberately altered copy of $(basename "$b"); the real comparison is in the other file" >> "$out"
  say "compare self-test: an altered state file is reported ($CMP_BADS BAD)"
}

# Runs INSIDE the container as the service user, on the live database with the
# pre-upgrade snapshot attached, read only: the goose version of both, the file
# mode and owner of the snapshot, and the row counts of every table that both
# have. Counts only (T-21-09).
read -r -d '' ATTACH_SH <<'ATTACH_EOF' || true
set -eu
live=/opt/holzcloud/data/holzcloud.sqlite
snap=$1
tables=$(sqlite3 -readonly "$live" ".timeout 5000" "ATTACH '$snap' AS old; select m.name from main.sqlite_master m join old.sqlite_master o on o.name = m.name and o.type = 'table' where m.type = 'table' and m.name not like 'sqlite_%' order by m.name")
{
  echo ".timeout 5000"
  echo "ATTACH '$snap' AS old;"
  echo "select 'snapschema ' || coalesce(max(version_id), 0) from old.goose_db_version where is_applied = 1;"
  echo "select 'liveschema ' || coalesce(max(version_id), 0) from main.goose_db_version where is_applied = 1;"
  for t in $tables; do
    echo "select 'snaprow $t ' || (select count(*) from old.\"$t\") || ' ' || (select count(*) from main.\"$t\");"
  done
} | sqlite3 -readonly "$live"
echo "snapfile $(stat -c '%U %a' "$snap")"
ATTACH_EOF

# attach_check SNAPSHOT: the independent proof. The snapshot is the service's own
# verified copy of the old state; its counts must equal S0 and the live counts
# of every non-volatile table both have.
attach_check() {
  local snap=$1 out raw line t sn ln n0 bk bkm sschema lschema s0schema s1schema owner
  out="$OUT_DIR/attach-s1.txt"
  : > "$out"
  CMP_OUT=$out
  CMP_BADS=0
  if [ -z "$snap" ] || [ "$snap" = none ]; then
    cmp_bad "snapshot: the journal names no 'pre-upgrade snapshot written' path"
    return 1
  fi
  raw=$(printf '%s\n' "$ATTACH_SH" | dxi runuser -u holzcloud -- bash -s -- "$snap") \
    || { cmp_bad "snapshot: the ATTACH session failed for $snap"; return 1; }
  s0schema=$(st_get "$OUT_DIR/state-s0.txt" schema)
  s1schema=$(st_get "$OUT_DIR/state-s1.txt" schema)
  sschema=$(printf '%s\n' "$raw" | sed -n 's/^snapschema //p')
  lschema=$(printf '%s\n' "$raw" | sed -n 's/^liveschema //p')
  if [ -n "$sschema" ] && [ "$sschema" = "$s0schema" ]; then
    cmp_pass "snapshot schema $sschema (the old schema, as captured in S0)"
  else
    cmp_bad "snapshot schema ${sschema:-?}, S0 had ${s0schema:-?}"
  fi
  if [ -n "$lschema" ] && [ "$lschema" = "$s1schema" ] && [ "$lschema" -gt "${sschema:-0}" ]; then
    cmp_pass "live schema $lschema (the new schema, ahead of the snapshot)"
  else
    cmp_bad "live schema ${lschema:-?}, S1 had ${s1schema:-?}, snapshot ${sschema:-?}"
  fi
  owner=$(printf '%s\n' "$raw" | sed -n 's/^snapfile //p')
  if [ "$owner" = "holzcloud 600" ]; then
    cmp_pass "snapshot mode $owner (user and octal mode, T-21-10)"
  else
    cmp_bad "snapshot mode '$owner', expected 'holzcloud 600'"
  fi
  while IFS=' ' read -r line t sn ln; do
    [ "$line" = snaprow ] || continue
    if is_volatile "$t"; then
      cmp_info "snapshot volatile $t snapshot=$sn live=$ln"
      continue
    fi
    n0=$(st_count "$OUT_DIR/state-s0.txt" "$t")
    if [ -n "$n0" ] && [ "$sn" = "$n0" ] && [ "$ln" = "$sn" ]; then
      cmp_pass "snapshot rows $t"
    else
      cmp_bad "snapshot rows $t snapshot=$sn s0=${n0:-none} live=$ln"
    fi
  done <<< "$raw"

  bk=$(dx bash -c 'ls -1 /opt/holzcloud/backups 2>/dev/null | sort | tail -n 1' || true)
  if [ -z "$bk" ]; then
    cmp_info "no backup: /opt/holzcloud/backups holds none"
  else
    bkm=$(dx stat -c '%U %a' "/opt/holzcloud/backups/$bk") || bkm="?"
    if [ "$bkm" = "holzcloud 700" ]; then
      cmp_pass "backup mode $bkm ($bk)"
    else
      cmp_bad "backup mode '$bkm' for $bk, expected 'holzcloud 700'"
    fi
  fi
  [ "$CMP_BADS" -eq 0 ]
}

# snapshot_count: the pre-upgrade snapshot files in the data directory.
snapshot_count() {
  dx bash -c 'ls -1 /opt/holzcloud/data/pre-upgrade-*.sqlite 2>/dev/null | wc -l' | tr -d ' \r'
}

# nomigration_check PRE_N SNAP APPLIED SA SB: the update of two builds with the same
# schema. Nothing migrates, so nothing may be written: no snapshot path in the
# journal, no new snapshot file, no 'migration applied' line, the schema unchanged.
# The result goes to attach-s1.txt, where the ATTACH comparison stands when a
# snapshot exists.
nomigration_check() {
  local pre_n=$1 snap=$2 applied=$3 sa=$4 sb=$5 post_n out
  out="$OUT_DIR/attach-s1.txt"
  : > "$out"
  CMP_OUT=$out
  CMP_BADS=0
  post_n=$(snapshot_count)
  if [ -z "$snap" ] || [ "$snap" = none ]; then
    cmp_pass "no snapshot (the journal has no 'pre-upgrade snapshot written' line since the start of the update)"
  else
    cmp_bad "no snapshot: the journal names $snap although no migration was pending"
  fi
  if [ "$post_n" = "$pre_n" ]; then
    cmp_pass "no new pre-upgrade file in the data directory ($pre_n before, $post_n after)"
  else
    cmp_bad "pre-upgrade files: $pre_n before the update, $post_n after"
  fi
  if [ "${applied:-0}" = 0 ]; then
    cmp_pass "no migration applied in the journal"
  else
    cmp_bad "$applied 'migration applied' lines although no migration was pending"
  fi
  if [ -n "$sa" ] && [ "$sa" = "$sb" ]; then
    cmp_pass "schema unchanged ($sa -> $sb)"
  else
    cmp_bad "schema ${sa:-?} -> ${sb:-?}, expected no change"
  fi
  [ "$CMP_BADS" -eq 0 ]
}

# update_marker: a page written after the update, through the new binary. The
# rollback of plan 21-04 must make it disappear again.
update_marker() {
  local wid args ans pid code
  wid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id from websites where name = 'Test'") || return 1
  [ -n "$wid" ] || return 1
  args=$(dx jq -n --argjson w "$wid" '{website:$w,title:"After update",slug:"after-update",markdown:"Written after the update. A rollback to the old version must take this page away again."}') || return 1
  ans=$(mcp create_page "$args") || return 1
  pid=$(printf '%s' "$ans" | dxi jq -e '.id') || return 1
  mcp publish_page "{\"id\":$pid,\"status\":\"published\"}" >/dev/null || return 1
  code=$(http_code /after-update)
  [ "$code" = 200 ] || { say "GET /after-update answered $code, expected 200"; return 1; }
  return 0
}

# compare A B [OUTFILE]: the comparison on two state files, for a rerun without
# the container (exit 1 when any BAD).
cmd_compare() {
  [ "$#" -ge 2 ] || usage
  compare_states "$1" "$2" "${3:-$OUT_DIR/compare.txt}"
}

# baseline: state S0 on the OLD binary, under the unit the repaired document
# installed (D-06).
cmd_baseline() {
  local ver wid pid order
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh install' with INSTALL_FROM=old first"
  ver=$(dx /opt/holzcloud/holzcloud version) || die "holzcloud version failed"
  case "$ver" in
    *"$OLD_TAG"*) ;;
    *) die "baseline needs the OLD binary installed (INSTALL_FROM=old, OLD_TAG=$OLD_TAG); 'holzcloud version' says '$ver'" ;;
  esac
  mkdir -p "$OUT_DIR"
  stage_fixtures
  cmd_seed

  # The login user: the password is made inside the container, kept in a
  # root-only file and handed to the command on stdin, so it is in no argument
  # list and in no output file (T-21-07). The old binary runs the command, which
  # is the only CLI call besides `ai key create` that baseline makes.
  dx bash -c 'umask 077; openssl rand -hex 12 > /root/.hc-login-pass; test -s /root/.hc-login-pass' \
    || die "could not create the login password file"
  dx bash -c 'runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data /opt/holzcloud/holzcloud user create -email login@example.test -role admin < /root/.hc-login-pass' \
    || die "holzcloud user create failed on the old binary"

  order="$(dx date +%Y)-9001"
  ORDER_NO=$order
  insert_old_order "$order"
  say "the order $order is a direct INSERT into the database (the shop flow would need a browser)"

  wid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id from websites where name = 'Test'") || wid=""
  pid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id from pages where slug = 'plugintest'") || pid=""
  capture_state s0
  {
    echo "OLD_TAG: $OLD_TAG"
    echo "VERSION: $ver"
    echo "WEBSITE: ${wid:-?}"
    echo "DOMAIN: test.local"
    echo "PAGE: ${pid:-?} plugintest (published)"
    echo "PLUGINS: bestellung jahreszahl kontaktformular nicht-gefunden suche"
    echo "USER: login@example.test (admin, created with holzcloud user create on the old binary)"
    echo "ORDER: $order (a direct INSERT in the statement shape of the old code, not the shop flow)"
  } > "$OUT_DIR/baseline.txt"
  leak_check
  say "BASELINE OK"
}

# update: the Updating section of the document, played exactly as it stands
# (D-07). Both binaries exist; the new one is what the scp line carries. Then
# the proof that nothing was lost (D-06): state s1 against s0, the comparison
# against the service's own pre-upgrade snapshot, and a marker page that the
# rollback of plan 21-04 has to remove again.
cmd_update() {
  local o_ver o_com n_ver n_com start n0 failed mo mn pend snap sa sb prev bk applied jl crc=0 arc=0 mrc=0 marker cpass cbad apass abad pre_n
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh install baseline' first"
  [ -f "$OUT_DIR/state-s0.txt" ] || die "no $OUT_DIR/state-s0.txt; run 'run.sh baseline' first"
  WITH_OLD=1
  cmd_bins
  STAGE_BIN=$(bin_path new) || exit 1
  [ -f "$STAGE_BIN" ] || die "the new binary $STAGE_BIN does not exist"
  mo=$(sed -n 's/^migrations: *//p' "$(bin_path old).info" | head -n 1)
  mn=$(sed -n 's/^migrations: *//p' "$STAGE_BIN.info" | head -n 1)
  [ -n "$mo" ] && [ -n "$mn" ] || die "no migration file counts in $(bin_path old).info and $STAGE_BIN.info"
  pend=$((mn - mo))
  say "migration files: old=$mo new=$mn pending=$pend"

  o_ver=$(sed -n 's/^identity: //p' "$OUT_DIR/state-s0.txt" | dxi jq -r '.version')
  o_com=$(sed -n 's/^identity: //p' "$OUT_DIR/state-s0.txt" | dxi jq -r '.commit')
  sa=$(sed -n 's/^schema: //p' "$OUT_DIR/state-s0.txt")

  # The container's clock, in the zone journalctl reads --since in.
  start=$(dx date '+%Y-%m-%d %H:%M:%S')
  pre_n=$(snapshot_count)
  n0=$(steps_count)
  doc_play "Updating" || true
  failed=$(steps_failed_since "$n0")
  wait_ready || { printf 'READY: no\n' > "$OUT_DIR/update.txt"; exit 1; }

  n_ver=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -r '.version')
  n_com=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -r '.commit')
  jl=$(dx bash -c 'journalctl -u holzcloud --since "$1" -o cat --no-pager | grep -E "pre-upgrade snapshot written|migration applied" || true' _ "$start")
  snap=$(printf '%s\n' "$jl" | grep 'pre-upgrade snapshot written' | head -n 1 | dxi jq -r '.path // empty' 2>/dev/null || true)
  applied=$(printf '%s\n' "$jl" | grep -c 'migration applied' || true)
  prev=absent
  if dx test -e /opt/holzcloud/holzcloud.prev; then prev=present; fi
  bk=$(dx bash -c 'ls -1 /opt/holzcloud/backups 2>/dev/null | sort | tail -n 1' || true)

  capture_state s1
  sb=$(sed -n 's/^schema: //p' "$OUT_DIR/state-s1.txt")

  compare_states "$OUT_DIR/state-s0.txt" "$OUT_DIR/state-s1.txt" "$OUT_DIR/compare-s0-s1.txt" || crc=1
  cpass=$(grep -c '^PASS' "$OUT_DIR/compare-s0-s1.txt" || true)
  cbad=$(grep -c '^BAD' "$OUT_DIR/compare-s0-s1.txt" || true)
  compare_selftest "$OUT_DIR/state-s0.txt" "$OUT_DIR/state-s1.txt"
  if [ "$pend" -eq 0 ]; then
    nomigration_check "$pre_n" "$snap" "$applied" "$sa" "$sb" || arc=1
  else
    attach_check "$snap" || arc=1
  fi
  apass=$(grep -c '^PASS' "$OUT_DIR/attach-s1.txt" || true)
  abad=$(grep -c '^BAD' "$OUT_DIR/attach-s1.txt" || true)

  # The marker: written through the new binary after the comparison, so the
  # comparison is of the update alone.
  marker=failed
  if update_marker; then marker=ok; else mrc=1; fi
  capture_state s1m

  {
    echo "OLD: $o_ver $o_com"
    echo "NEW: $n_ver $n_com"
    echo "MIGRATIONS: old=$mo new=$mn pending=$pend"
    echo "APPLIED: $applied"
    echo "SNAPSHOT: ${snap:-none}"
    echo "SCHEMA: ${sa:-?} -> ${sb:-?}"
    echo "PREV: $prev"
    echo "BACKUP: ${bk:-none}"
    echo "COMPARE: pass=${cpass:-0} bad=${cbad:-0}"
    echo "ATTACH: pass=${apass:-0} bad=${abad:-0}"
    printf '%s\n' "$jl" | while IFS= read -r l; do
      [ -z "$l" ] || echo "JOURNAL: $l"
    done
    echo "MARKER: $marker"
  } > "$OUT_DIR/update.txt"
  cat "$OUT_DIR/update.txt"

  leak_check
  if [ -n "$failed" ]; then
    say "run.sh: UPDATE FAILED, counted steps with a non-zero status:"
    printf '%s\n' "$failed" >&2
    exit 1
  fi
  [ "$o_ver" != "$n_ver" ] || { say "run.sh: UPDATE FAILED, /readyz reports the same version ($n_ver) before and after"; exit 1; }
  if [ "$pend" -eq 0 ]; then
    if [ "${sb:-x}" != "${sa:-y}" ]; then
      say "run.sh: UPDATE FAILED, no migration lies between these builds and the schema moved (${sa:-?} -> ${sb:-?})"
      exit 1
    fi
  elif ! [ "${sb:-0}" -gt "${sa:-0}" ] 2>/dev/null; then
    say "run.sh: UPDATE FAILED, the schema did not move forward (${sa:-?} -> ${sb:-?})"
    exit 1
  fi
  if [ "$crc" -ne 0 ] || [ "$arc" -ne 0 ] || [ "$mrc" -ne 0 ]; then
    say "run.sh: UPDATE FAILED: comparison s0/s1 bad=${cbad:-0}, attach bad=${abad:-0}, marker $marker; see compare-s0-s1.txt and attach-s1.txt"
    exit 1
  fi
  say "UPDATE OK"
}

# ---------------------------------------------------------------------------
# Phase 21: going back (DEP-03). `rollback` plays the document's Rolling back
# section after the update; `negative` plays, once, what the document says not to
# do, so that the document can state what happens instead of assuming it (D-09).
# ---------------------------------------------------------------------------

# scenario TEXT: a step the harness performs that the document must not contain
# (the vocabulary next to DEVIATION: a deviation replaces a document line the
# container cannot play; a scenario is a line that is in no document on purpose).
scenario() {
  mkdir -p "$OUT_DIR"
  say "$*"
  printf '%s\n' "$*" >> "$OUT_DIR/scenario.txt"
}

# readyz_line: the /readyz answer reduced to status, version, commit, integrity.
readyz_line() {
  dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -c '{status,version,commit,integrity}'
}

# aside_schema: the highest applied goose version of the set-aside copy, read
# immutable so that no -wal or -shm file appears next to it.
aside_schema() {
  dx runuser -u holzcloud -- sqlite3 'file:/opt/holzcloud/data/holzcloud.sqlite.after-update?immutable=1' \
    "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1"
}

# rollback: the Rolling back section of the document, played as it stands, on the
# container `update` left behind. The update's journal decides which of its two
# blocks is played: a snapshot path (block 1) or none (block 2). The harness adds no
# wait to the play: the document's own verification line waits for the start.
cmd_rollback() {
  local snap mode nb n0 failed old_ver s2_ver sch_want sch_now code owner aside tok
  local cmpfile since inv jl crc=0 f tk1
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh update' first"
  [ -f "$OUT_DIR/update.txt" ] && grep -q '^MARKER: ok' "$OUT_DIR/update.txt" \
    || die "no $OUT_DIR/update.txt with 'MARKER: ok'; run 'run.sh update' first"
  for f in state-s0.txt state-s1.txt state-s1m.txt; do
    [ -f "$OUT_DIR/$f" ] || die "no $OUT_DIR/$f; run 'run.sh baseline update' first"
  done
  dx test -e /opt/holzcloud/holzcloud.prev || die "/opt/holzcloud/holzcloud.prev does not exist: there is no old binary to put back"
  nb=$(doc_blocks "Rolling back") || exit 1
  [ "$nb" -ge 2 ] || die "$DEPLOY_DOC: the section 'Rolling back' has $nb bash blocks, two are needed (snapshot written, none written)"

  snap=$(sed -n 's/^SNAPSHOT: //p' "$OUT_DIR/update.txt" | head -n 1)
  case "$snap" in
    none | "") mode=binary ;;
    /*) mode=snapshot ;;
    *) die "update.txt names a SNAPSHOT that is neither a path nor 'none': '$snap'" ;;
  esac
  say "rollback mode: $mode"
  mkdir -p "$OUT_DIR"

  n0=$(steps_count)
  if [ "$mode" = snapshot ]; then
    doc_play "Rolling back" 1 || true
  else
    doc_play "Rolling back" 2 || true
  fi
  failed=$(steps_failed_since "$n0")
  wait_ready || { printf 'MODE: %s\nREADY: no\n' "$mode" > "$OUT_DIR/rollback.txt"; exit 1; }

  capture_state s2
  if [ "$mode" = snapshot ]; then
    cmpfile="$OUT_DIR/compare-s0-s2.txt"
    compare_states "$OUT_DIR/state-s0.txt" "$OUT_DIR/state-s2.txt" "$cmpfile" || crc=1
  else
    cmpfile="$OUT_DIR/compare-s1m-s2.txt"
    compare_states "$OUT_DIR/state-s1m.txt" "$OUT_DIR/state-s2.txt" "$cmpfile" || crc=1
  fi

  CHECK_OUT="$OUT_DIR/rollback.txt"
  CHECK_BADS=0
  printf 'MODE: %s\n' "$mode" > "$CHECK_OUT"

  # The old version runs again.
  old_ver=$(sed -n 's/^identity: //p' "$OUT_DIR/state-s0.txt" | dxi jq -r '.version')
  s2_ver=$(sed -n 's/^identity: //p' "$OUT_DIR/state-s2.txt" | dxi jq -r '.version')
  if [ -n "$old_ver" ] && [ "$old_ver" = "$s2_ver" ]; then
    check_pass "version old $s2_ver"
  else
    check_bad "version old: /readyz says '$s2_ver', the baseline had '$old_ver'"
  fi

  # The schema is the old one (snapshot) or still the new one (binary only).
  if [ "$mode" = snapshot ]; then sch_want=$(st_get "$OUT_DIR/state-s0.txt" schema); else sch_want=$(st_get "$OUT_DIR/state-s1.txt" schema); fi
  sch_now=$(st_get "$OUT_DIR/state-s2.txt" schema)
  if [ -n "$sch_want" ] && [ "$sch_want" = "$sch_now" ]; then
    check_pass "schema $sch_now"
  else
    check_bad "schema: now '$sch_now', expected '$sch_want'"
  fi

  # The marker page of the update: gone with the snapshot, kept with the binary alone.
  code=$(http_code /after-update) || code=error
  if [ "$mode" = snapshot ]; then
    if [ "$code" = 404 ]; then check_pass "marker gone (GET /after-update 404)"; else check_bad "marker gone: GET /after-update answered $code, expected 404"; fi
    tok=$(st_get "$OUT_DIR/state-s2.txt" order)
    if [ "${tok##* }" = no-token-column ]; then
      check_pass "no token column (the order of the old schema, as it was)"
    else
      check_bad "no token column: the order line of state-s2 is '$tok'"
    fi
  else
    if [ "$code" = 200 ]; then check_pass "marker kept (GET /after-update 200)"; else check_bad "marker kept: GET /after-update answered $code, expected 200"; fi
    # The order token is whatever the update left: 32 characters when the
    # migration backfilled it, 0 when the old build already ran at the new schema
    # and wrote the order itself. Putting the binary back must not change it.
    tok=$(st_get "$OUT_DIR/state-s2.txt" order)
    tk1=$(st_get "$OUT_DIR/state-s1m.txt" order)
    if [ -n "$tok" ] && [ "${tok##* }" = "${tk1##* }" ] && [ "${tok##* }" != no-token-column ]; then
      check_pass "token kept (${tok##* } characters, as after the update)"
    else
      check_bad "token kept: the order line of state-s2 is '$tok', after the update it was '$tk1'"
    fi
  fi
  code=$(http_code /plugintest) || code=error
  if [ "$code" = 200 ]; then check_pass "pre-update page (GET /plugintest 200)"; else check_bad "pre-update page: GET /plugintest answered $code"; fi

  # Ownership and mode of the live database (T-21-16).
  owner=$(st_get "$OUT_DIR/state-s2.txt" owner)
  if [ "$owner" = "holzcloud 600" ]; then
    check_pass "owner and mode $owner"
  else
    check_bad "owner and mode: '$owner', expected 'holzcloud 600'"
  fi

  # The migrated database stays on disk, so the rollback can be undone (T-21-13).
  if [ "$mode" = snapshot ]; then
    aside=$(aside_schema) || aside=""
    if [ -n "$aside" ] && [ "$aside" = "$(st_get "$OUT_DIR/state-s1.txt" schema)" ]; then
      check_pass "set-aside copy holzcloud.sqlite.after-update at schema $aside"
    else
      check_bad "set-aside copy: schema '${aside:-unreadable}', expected the new one ($(st_get "$OUT_DIR/state-s1.txt" schema))"
    fi
  fi

  # The restart migrated nothing and took no snapshot.
  since=$(dx systemctl show -p ActiveEnterTimestamp --value holzcloud)
  inv=$(dx systemctl show -p InvocationID --value holzcloud)
  jl=$(journal_invocation "$since" "$inv" | grep -E 'migration applied|pre-upgrade snapshot written' || true)
  if [ -z "$jl" ]; then
    check_pass "no migration on restart"
  else
    check_bad "no migration on restart: $(printf '%s' "$jl" | head -n 2)"
  fi

  # The service reports the database sound.
  if sed -n 's/^identity: //p' "$OUT_DIR/state-s2.txt" | dxi jq -e '.status == "ok" and .integrity == "ok"' >/dev/null 2>&1; then
    check_pass "integrity ok"
  else
    check_bad "integrity: /readyz is not status ok and integrity ok"
  fi

  # Informational: what a snapshot brings back (T-21-15). Counts only.
  {
    echo "INFO credentials users=$(st_count "$OUT_DIR/state-s2.txt" users) sessions=$(st_count "$OUT_DIR/state-s2.txt" sessions) ai_tokens=$(st_count "$OUT_DIR/state-s2.txt" ai_tokens) user_tokens=$(st_count "$OUT_DIR/state-s2.txt" user_tokens) (counts in the restored state; state-s0 had users=$(st_count "$OUT_DIR/state-s0.txt" users) ai_tokens=$(st_count "$OUT_DIR/state-s0.txt" ai_tokens))"
    echo "INFO compare $cmpfile pass=$(grep -c '^PASS' "$cmpfile" || true) bad=$(grep -c '^BAD' "$cmpfile" || true)"
  } | tee -a "$CHECK_OUT"

  leak_check
  if [ -n "$failed" ]; then
    say "run.sh: ROLLBACK FAILED, counted steps with a non-zero status:"
    printf '%s\n' "$failed" >&2
    exit 1
  fi
  if [ "$CHECK_BADS" -gt 0 ] || [ "$crc" -ne 0 ]; then
    say "run.sh: ROLLBACK FAILED, $CHECK_BADS check(s) and comparison status $crc; see rollback.txt and $(basename "$cmpfile")"
    exit 1
  fi
  say "ROLLBACK OK"
}

# negative: the OLD binary against the migrated database, on the container
# `update` left behind. The document says not to do this, so nothing is read from
# it: the case is a scenario. The result, whichever it is, is the evidence (D-09);
# the case ends with the new binary running again.
cmd_negative() {
  local snap start i healthy=0 active restarts result out new_ver new_bin nv
  local ready schema jl ins1 ins2 yr n_after
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active; run 'run.sh update' first"
  [ -f "$OUT_DIR/update.txt" ] && grep -q '^MARKER: ok' "$OUT_DIR/update.txt" \
    || die "no $OUT_DIR/update.txt with 'MARKER: ok'; run 'run.sh update' first"
  snap=$(sed -n 's/^SNAPSHOT: //p' "$OUT_DIR/update.txt" | head -n 1)
  case "$snap" in
    none | "") die "the update wrote no snapshot: no migration lies between these builds, and the negative case needs a migrated schema" ;;
  esac
  dx test -e /opt/holzcloud/holzcloud.prev || die "/opt/holzcloud/holzcloud.prev does not exist: there is no old binary to start"
  new_bin=$(bin_path new) || exit 1
  [ -f "$new_bin" ] || die "the new binary $new_bin does not exist; run 'run.sh update' first"
  new_ver=$(sed -n 's/^NEW: \([^ ]*\) .*/\1/p' "$OUT_DIR/update.txt" | head -n 1)
  [ -n "$new_ver" ] || die "update.txt has no NEW line"
  yr=$(dx date +%Y)

  scenario "SCENARIO[negative]: the OLD binary (holzcloud.prev) is started against the database the NEW binary migrated, with no snapshot restored. The document tells nobody to do this; it is played once to find out what happens"
  dx systemctl stop holzcloud
  dx cp -p /opt/holzcloud/holzcloud.prev /opt/holzcloud/holzcloud
  start=$(dx date '+%Y-%m-%d %H:%M:%S')
  dx systemctl start holzcloud || true

  i=1
  while [ "$i" -le "$READY_TIMEOUT" ]; do
    if dx curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
      healthy=1
      break
    fi
    active=$(dx systemctl show -p ActiveState --value holzcloud 2>/dev/null || true)
    restarts=$(dx systemctl show -p NRestarts --value holzcloud 2>/dev/null || echo 0)
    [ "$active" != failed ] || break
    [ "${restarts:-0}" -le 0 ] || break
    sleep 1
    i=$((i + 1))
  done

  mkdir -p "$OUT_DIR"
  : > "$OUT_DIR/negative.txt"
  if [ "$healthy" -eq 1 ]; then
    result=starts
    echo "NEGATIVE-RESULT: starts" >> "$OUT_DIR/negative.txt"
    ready=$(readyz_line) || ready="unavailable"
    echo "readyz: $ready" >> "$OUT_DIR/negative.txt"
  else
    result=refuses
    echo "NEGATIVE-RESULT: refuses" >> "$OUT_DIR/negative.txt"
    echo "readyz: unavailable" >> "$OUT_DIR/negative.txt"
  fi
  schema=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
    "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1" 2>&1 | tr '\n' ' ') || schema="unreadable"
  echo "schema: $schema" >> "$OUT_DIR/negative.txt"
  jl=$(dx bash -c 'journalctl -u holzcloud --since "$1" -o cat --no-pager | grep -i "migrat" || true' _ "$start")
  if [ -z "$jl" ]; then
    echo "journal: none" >> "$OUT_DIR/negative.txt"
  else
    printf '%s\n' "$jl" | while IFS= read -r l; do echo "journal: $l"; done >> "$OUT_DIR/negative.txt"
  fi

  if [ "$result" = starts ]; then
    # The old code's statement shape: it does not name orders.token, which
    # migration 00060 made NOT NULL DEFAULT '' with a unique index.
    ins1=ok
    out=$(old_order_run "$yr-9002" 2>&1) || ins1=$(printf '%s' "$out" | tr '\n' ' ')
    ins2=ok
    out=$(old_order_run "$yr-9003" 2>&1) || ins2=$(printf '%s' "$out" | tr '\n' ' ')
    echo "insert-1: $ins1" >> "$OUT_DIR/negative.txt"
    echo "insert-2: $ins2" >> "$OUT_DIR/negative.txt"
    n_after=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select count(*) from orders" 2>&1 | tr '\n' ' ') || n_after="unreadable"
    echo "orders: $n_after" >> "$OUT_DIR/negative.txt"
  else
    dx bash -c 'journalctl -u holzcloud --since "$1" -o cat --no-pager | grep "^{" | tail -n 4 || true' _ "$start" \
      | while IFS= read -r l; do echo "refusal: $l"; done >> "$OUT_DIR/negative.txt"
    echo "insert-1: skipped" >> "$OUT_DIR/negative.txt"
    echo "insert-2: skipped" >> "$OUT_DIR/negative.txt"
  fi

  # Back to the new binary, so the chain goes on from the real post-update state.
  scenario "SCENARIO[restore-new]: the NEW binary is put back (docker cp to /tmp/holzcloud, stop, copy, start), so the update state is as the update left it, plus the order rows the old code wrote"
  dx systemctl stop holzcloud
  dx systemctl reset-failed holzcloud || true
  docker cp "$new_bin" "$NAME:/tmp/holzcloud" || die "docker cp of the new binary failed"
  dx cp /tmp/holzcloud /opt/holzcloud/holzcloud
  dx systemctl start holzcloud || true
  wait_ready || exit 1
  nv=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -r '.version')
  if [ "$nv" != "$new_ver" ]; then
    echo "RESTORE: bad (/readyz says '$nv', expected '$new_ver')" >> "$OUT_DIR/negative.txt"
    leak_check
    die "the new binary did not come back: /readyz says '$nv', expected '$new_ver'"
  fi
  echo "RESTORE: ok" >> "$OUT_DIR/negative.txt"
  cat "$OUT_DIR/negative.txt"
  leak_check
  say "NEGATIVE OK (the old binary $result)"
}

# ---------------------------------------------------------------------------
# Phase 21: the two chains (D-12). Each replays a whole proof from one command and
# writes OUT_DIR/summary.txt. The words of the summary (DEP-01 PASS, DEP-02 PASS,
# DEP-02 not applicable, DEP-03 PASS, LEAK-CHECK OK) are what phase 22 and the beta
# criteria grep: extend, do not rename.
# ---------------------------------------------------------------------------

# chain_need_base NAME: the proof starts from a bare systemd image, nothing else.
chain_need_base() {
  [ "$TARGET" = systemd-base ] \
    || die "$1: the proof starts from a bare systemd image; run it with TARGET=systemd-base (the document is played onto a machine with nothing but systemd)"
}

# steps_summary: "STEPS: N played, F failed", counted from steps.txt the way the
# harness counts everywhere (systemctl status never counts, a SKIP is rc=0).
steps_summary() {
  local played failed
  played=$(grep -c '^STEP ' "$(steps_file)" 2>/dev/null || true)
  failed=$(steps_failed_since 0 | grep -c '^STEP ' || true)
  printf 'STEPS: %s played, %s failed\n' "${played:-0}" "${failed:-0}"
}

# deviation_kinds: the distinct DEVIATION kinds of deviations.txt, on one line.
deviation_kinds() {
  local k
  k=$(sed -n 's/^DEVIATION\[\([a-z]*\)\].*/\1/p' "$OUT_DIR/deviations.txt" 2>/dev/null | sort -u | tr '\n' ' ' | sed 's/ $//')
  printf '%s\n' "${k:-none}"
}

# fresh: DEP-01. Install, First Run, the audit of the document against the
# installed system, and Caddy. Caddy may only be unreachable (exit 3), never
# rejected.
cmd_fresh() {
  chain_need_base fresh
  CLEANUP_DOWN=1
  local crc=0 steps
  cmd_build
  cmd_up
  cmd_install
  cmd_firstrun
  cmd_postinstall
  ( cmd_caddy ) || crc=$?
  case "$crc" in 0 | 3) ;; *) die "the Caddy block failed (exit $crc)" ;; esac

  leak_check
  steps=$(steps_summary)
  {
    echo "---- summary ($PLATFORM) ----"
    echo "DEP-01 PASS"
    grep '^CADDY: ' "$OUT_DIR/caddy.txt" | tail -n 1
    echo "$steps"
    echo "DEVIATIONS: $(deviation_kinds)"
    echo "LEAK-CHECK OK"
  } > "$OUT_DIR/summary.txt"
  cat "$OUT_DIR/summary.txt"
}

# lifecycle: DEP-02 and DEP-03. The old release installed and seeded, the update,
# the negative case when a migration lies between the builds, the rollback.
cmd_lifecycle() {
  chain_need_base lifecycle
  CLEANUP_DOWN=1
  INSTALL_FROM=old
  local pend dep2=fail dep3=fail cmpfile mode
  cmd_build
  cmd_up
  cmd_install
  cmd_baseline
  cmd_update
  pend=$(sed -n 's/^MIGRATIONS:.*pending=\([0-9]*\).*/\1/p' "$OUT_DIR/update.txt" | head -n 1)
  [ -n "$pend" ] || die "update.txt has no 'MIGRATIONS: ... pending=N' line"
  if [ "$pend" -ge 1 ]; then
    cmd_negative
  else
    scenario "SCENARIO[negative]: skipped, no migration lies between these builds (pending=0), so there is no migrated schema for the old binary to meet"
  fi
  cmd_rollback

  mode=$(sed -n 's/^MODE: //p' "$OUT_DIR/rollback.txt" | head -n 1)
  if [ "$mode" = snapshot ]; then cmpfile="$OUT_DIR/compare-s0-s2.txt"; else cmpfile="$OUT_DIR/compare-s1m-s2.txt"; fi
  if [ "$pend" -ge 1 ] && grep -q '^MARKER: ok' "$OUT_DIR/update.txt" \
    && ! grep -q '^SNAPSHOT: none' "$OUT_DIR/update.txt" \
    && ! grep -q '^BAD' "$OUT_DIR/compare-s0-s1.txt" "$OUT_DIR/attach-s1.txt"; then
    dep2=pass
  fi
  if [ -n "$mode" ] && [ -f "$cmpfile" ] && ! grep -q '^BAD' "$OUT_DIR/rollback.txt" "$cmpfile"; then
    dep3=pass
  fi

  leak_check
  {
    echo "---- summary ($PLATFORM) ----"
    sed -n '/^OLD: /p;/^NEW: /p;/^MIGRATIONS: /p;/^SNAPSHOT: /p' "$OUT_DIR/update.txt"
    echo "MODE: $mode"
    if [ -f "$OUT_DIR/negative.txt" ]; then
      grep '^NEGATIVE-RESULT: ' "$OUT_DIR/negative.txt" | head -n 1
    fi
    if [ "$pend" -eq 0 ]; then
      echo "DEP-02 not applicable (no migration between these builds)"
    elif [ "$dep2" = pass ]; then
      echo "DEP-02 PASS"
    else
      echo "DEP-02 FAIL"
    fi
    if [ "$dep3" = pass ]; then echo "DEP-03 PASS"; else echo "DEP-03 FAIL"; fi
    echo "LEAK-CHECK OK"
  } > "$OUT_DIR/summary.txt"
  cat "$OUT_DIR/summary.txt"
  if [ "$dep3" != pass ] || { [ "$pend" -ge 1 ] && [ "$dep2" != pass ]; }; then
    die "lifecycle: a requirement did not pass, see summary.txt"
  fi
}

# ---------------------------------------------------------------------------
# Phase 22: a backup, carried to a second container and restored there (BAK-01,
# BAK-02). The source is the populated site that
#   INSTALL_FROM=old run.sh build up install baseline update postinstall
# leaves; the target is a fresh TARGET=systemd-base container brought up with
# `build up install` under its own NAME and OUT_DIR (TGT_NAME, TGT_OUT_DIR).
#   run.sh backup transfer restore     NAME=source TGT_NAME=target
# The names of the subcommands, of the variables above and of the files and KEY
# lines below are an interface that later plans and phase 23 script against.
# A failing assertion is a BAD line, never an early exit: each subcommand writes
# every file it owns and only then exits 1.
# ---------------------------------------------------------------------------

# on_target FUNCTION [ARGS]: run FUNCTION with NAME, OUT_DIR, INSTALL_FROM and
# WITH_OLD switched to the target container, and put them back afterwards, even
# when FUNCTION returns non-zero. SRC_NAME and SRC_OUT_DIR name the source while
# it runs. A restore must never be pointed at the container that was backed up
# (T-22-04).
on_target() {
  local fn=$1 rc=0 keep_from keep_old
  shift
  [ "$TGT_NAME" != "$NAME" ] \
    || die "TGT_NAME equals NAME ($NAME): a restore must never be pointed at the container that was backed up; give the target its own TGT_NAME"
  SRC_NAME=$NAME
  SRC_OUT_DIR=$OUT_DIR
  keep_from=$INSTALL_FROM
  keep_old=$WITH_OLD
  NAME=$TGT_NAME
  OUT_DIR=$TGT_OUT_DIR
  INSTALL_FROM=new
  WITH_OLD=0
  "$fn" "$@" || rc=$?
  NAME=$SRC_NAME
  OUT_DIR=$SRC_OUT_DIR
  INSTALL_FROM=$keep_from
  WITH_OLD=$keep_old
  return "$rc"
}

# key_line TEXT: a KEY: value line of a result file, to CHECK_OUT and the screen.
key_line() { printf '%s\n' "$*" | tee -a "$CHECK_OUT"; }

check_info() { printf 'INFO %s\n' "$*" | tee -a "$CHECK_OUT"; }

# What the whole-directory comparison leaves out, in ONE place: PATTERN|REASON.
# An exclusion is a decision with a reason, never a convenience; every entry is
# written to the verify file as an INFO line. Anything else that is in the data
# directory and not in the backup is a finding (a BAD line), which is the point of
# comparing the whole directory and not only the directories the script copies:
# a comparison limited to those would pass while the restore is broken (D-07).
# (No readiness probe file is excluded: /readyz asks statfs and writes nothing
# into the data directory, so such a file would be a finding too.)
TREE_EXCLUDES='holzcloud.sqlite*|the SQLite family: the database is compared by integrity and by row counts, because a snapshot is a different file from the live one by design, and the -wal and -shm files are working files of the live one
pre-upgrade-*.sqlite*|old-schema copies the service writes before a migration, deliberately not part of a backup
MANIFEST|only a backup has one; it names the version and the host that made it'

# tree_excluded_info: the exclusions as INFO lines of CHECK_OUT.
tree_excluded_info() {
  local pat reason
  while IFS='|' read -r pat reason; do
    if [ -n "$pat" ]; then check_info "EXCLUDED $pat: $reason"; fi
  done <<< "$TREE_EXCLUDES"
}

# tree_hash DIR: one line per regular file below DIR, "sha256  ./relative/path",
# sorted by path, as root in the current container (GNU find, sort -z and xargs
# -0); the output is for a host file. The exclusions are TREE_EXCLUDES.
tree_hash() {
  local dir=$1 args="" pat
  while IFS='|' read -r pat _; do
    if [ -n "$pat" ]; then args="$args ! -name '$pat'"; fi
  done <<< "$TREE_EXCLUDES"
  dx bash -c "cd \"\$1\" && find . -type f$args -print0 | sort -z | xargs -0 -r sha256sum" _ "$dir"
}

# tree_compare LISTA LISTB LABEL: two tree_hash files on the host, compared with
# POSIX awk and sort only. PASS "tree LABEL: N files equal" or one BAD per path.
# The first list is the reference: it must not be empty and must hold a path below
# ./media/, because an empty media directory would pass a listing (D-07, T-22-03).
tree_compare() {
  local a=$1 b=$2 label=$3 res n d line
  if [ ! -s "$a" ]; then
    check_bad "tree $label: the reference list $(basename "$a") is empty, which proves nothing"
  elif ! grep -q ' \./media/' "$a"; then
    check_bad "tree $label: the reference list $(basename "$a") has no file below ./media/, so an empty media directory would pass"
  fi
  res=$(awk -v fa="$a" '
    FILENAME == fa { A[substr($0, 67)] = substr($0, 1, 64); next }
    { B[substr($0, 67)] = substr($0, 1, 64) }
    END {
      for (p in A) {
        if (!(p in B)) print "D only in A: " p
        else if (A[p] != B[p]) print "D differs: " p
        else n++
      }
      for (p in B) if (!(p in A)) print "D only in B: " p
      print "N " n + 0
    }' "$a" "$b")
  n=$(printf '%s\n' "$res" | sed -n 's/^N //p')
  d=$(printf '%s\n' "$res" | sed -n 's/^D //p' | sort)
  if [ -n "$d" ]; then
    while IFS= read -r line; do
      check_bad "tree $label: $line"
    done <<< "$d"
  elif [ -s "$a" ]; then
    check_pass "tree $label: ${n:-0} files equal"
  fi
}

# tree_selftest LIST: a comparison that cannot fail proves nothing (T-22-03). A
# copy of the list loses a line and has one hash changed, and an empty list is
# compared too, and so is a list with nothing below ./media/; each must produce
# its BAD line. The expected BAD lines go to
# OUT_DIR/selftest-tree.txt, never to the verify file, and the file ends with
# SELFTEST OK. Dies when a tampered input was accepted.
tree_selftest() {
  local list=$1 st="$OUT_DIR/selftest-tree.txt" tampered="$OUT_DIR/.tree-tampered.txt" empty="$OUT_DIR/.tree-empty.txt" nomedia="$OUT_DIR/.tree-nomedia.txt"
  local keep_out=$CHECK_OUT keep_bads=$CHECK_BADS n b0
  n=$(awk 'END { print NR }' "$list")
  [ "${n:-0}" -ge 2 ] || die "tree_selftest: $list has ${n:-0} line(s); at least two are needed to alter one"
  awk -v n="$n" 'NR == 1 { next } NR == n { $0 = "0000" substr($0, 5) } { print }' "$list" > "$tampered"
  : > "$empty"
  printf '%s\n' "# The BAD lines below come from deliberately altered copies of $(basename "$list"); they are expected and are no findings" > "$st"
  CHECK_OUT=$st
  b0=$CHECK_BADS
  tree_compare "$list" "$tampered" selftest-altered > /dev/null
  if [ "$CHECK_BADS" -le "$b0" ] || ! grep -q '^BAD .*only in A' "$st" || ! grep -q '^BAD .*differs' "$st"; then
    CHECK_OUT=$keep_out
    die "tree_selftest: a list with a missing line and a changed hash was NOT reported as only-in-A and differs; the comparison cannot fail, see $st"
  fi
  tree_compare "$empty" "$list" selftest-empty > /dev/null
  if ! grep -q 'selftest-empty: the reference list .* is empty' "$st"; then
    CHECK_OUT=$keep_out
    die "tree_selftest: an empty reference list was NOT reported, see $st"
  fi
  # The reference list of an empty media directory: no path below ./media/.
  grep -v ' \./media/' "$list" > "$nomedia" || true
  tree_compare "$nomedia" "$list" selftest-nomedia > /dev/null
  if ! grep -q 'selftest-nomedia: the reference list .* has no file below ./media/' "$st"; then
    CHECK_OUT=$keep_out
    die "tree_selftest: a reference list without any file below ./media/ was NOT reported, see $st"
  fi
  echo "SELFTEST OK" >> "$st"
  rm -f "$tampered" "$empty" "$nomedia"
  CHECK_OUT=$keep_out
  CHECK_BADS=$keep_bads
  say "tree self-test: an altered list and an empty list are reported (selftest-tree.txt)"
}

# Runs INSIDE the container as root, one argument: a database file. One line per
# table of sqlite_master, "table NAME N": counts only, never a row of any table,
# the users, sessions and key tables included. The file is opened through the URI
# with immutable=1: the VACUUM INTO file is WAL-mode and must not grow -wal and
# -shm files next to it.
read -r -d '' ROWS_SH <<'ROWS_EOF' || true
set -u
u="file:$1?immutable=1"
sqlite3 "$u" "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name" | while IFS= read -r t; do
  n=$(sqlite3 "$u" "select count(*) from \"$t\"" 2>/dev/null) || n="?"
  echo "table $t $n"
done
ROWS_EOF

# rows_compare DB STATEFILE LABEL: the row counts of the database DB (in the
# current container) against the "table NAME N" lines of a state file. PASS "rows
# T", INFO for the volatile tables of is_volatile, BAD for a difference and for a
# table that exists on one side only. The raw counts go to OUT_DIR/rows-LABEL.txt.
rows_compare() {
  local db=$1 sf=$2 label=$3 rf="$OUT_DIR/rows-$3.txt" t n sn
  printf '%s\n' "$ROWS_SH" | dxi bash -s -- "$db" > "$rf" || check_bad "rows: counting the tables of $db failed"
  if ! grep -q '^table ' "$rf"; then
    check_bad "rows: no table line for $db, see $rf"
    return 0
  fi
  while IFS=' ' read -r _ t n; do
    [ -n "$t" ] || continue
    sn=$(st_count "$sf" "$t")
    if [ -z "$sn" ]; then
      check_bad "rows $t: the table exists in $label ($n rows) and not in $(basename "$sf")"
    elif is_volatile "$t"; then
      check_info "volatile $t $label=$n source=$sn"
    elif [ "$n" = "?" ] || [ "$sn" = "?" ]; then
      check_bad "rows $t: a count could not be read ($label=$n source=$sn)"
    elif [ "$n" = "$sn" ]; then
      check_pass "rows $t"
    else
      check_bad "rows $t: $label=$n source=$sn"
    fi
  done < "$rf"
  while IFS=' ' read -r _ t sn; do
    [ -n "$t" ] || continue
    if ! grep -q "^table $t " "$rf"; then
      check_bad "rows $t: the table exists in $(basename "$sf") ($sn rows) and not in $label"
    fi
  done <<< "$(grep '^table ' "$sf" || true)"
}

# Runs INSIDE the container as root, one argument: a directory. Raw numbers for
# dir_sizes. du -sb counts apparent bytes and each directory as 4096; the media
# figure is the sum of the sizes of the regular files below media/, so that a
# directory entry does not count as data.
read -r -d '' SIZES_SH <<'SIZES_EOF' || true
d=$1
echo "total $(du -sb "$d" | cut -f1)"
echo "human $(du -sh "$d" | cut -f1)"
echo "db $(stat -c %s "$d/holzcloud.sqlite" 2>/dev/null || echo 0)"
if [ -d "$d/media" ]; then
  echo "media $(find "$d/media" -type f -printf '%s\n' | awk '{ s += $1 } END { print s + 0 }')"
  echo "mediafiles $(find "$d/media" -type f | wc -l)"
else
  echo "media 0"
  echo "mediafiles 0"
fi
echo "files $(find "$d" -type f | wc -l)"
SIZES_EOF

# dir_sizes DIR TOTALKEY SIZEKEY: KEY: value lines on stdout for a directory of the
# current container: TOTALKEY (du -sb), SIZEKEY (du -sh, skipped when empty),
# DB-BYTES, MEDIA-BYTES, FILES and MEDIA-FILES; then HOST-ARCH (docker info) and
# CONTAINER-ARCH (uname -m), so that a record can say these are emulation numbers
# (D-14, D-15).
dir_sizes() {
  local d=$1 tk=$2 sk=$3 raw
  raw=$(printf '%s\n' "$SIZES_SH" | dxi bash -s -- "$d") || raw=""
  printf '%s: %s\n' "$tk" "$(sed -n 's/^total //p' <<< "$raw")"
  if [ -n "$sk" ]; then printf '%s: %s\n' "$sk" "$(sed -n 's/^human //p' <<< "$raw")"; fi
  printf 'DB-BYTES: %s\n' "$(sed -n 's/^db //p' <<< "$raw")"
  printf 'MEDIA-BYTES: %s\n' "$(sed -n 's/^media //p' <<< "$raw")"
  printf 'FILES: %s\n' "$(sed -n 's/^files //p' <<< "$raw")"
  printf 'MEDIA-FILES: %s\n' "$(sed -n 's/^mediafiles //p' <<< "$raw")"
  printf 'HOST-ARCH: %s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'CONTAINER-ARCH: %s\n' "$(dx uname -m | tr -d '\r')"
}

# media_served: the status and the sha256 of the media file the first row of the
# media table names (the row STATE_SH reads), fetched over HTTP as test.local. For
# comparison before and after a restore, not an assertion by itself.
media_served() {
  local row
  row=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
    "select website_id || '/' || filename from media order by id limit 1") || return 1
  if [ -z "$row" ]; then
    echo "none"
    return 0
  fi
  dx bash -c 'f=$(mktemp); c=$(curl -s -o "$f" -w "%{http_code}" -H "Host: test.local" "http://127.0.0.1:8080/media/$1"); echo "HTTP $c $(sha256sum < "$f" | cut -d" " -f1)"; rm -f "$f"' _ "$row"
}

# backup_verify DIR LABEL: what is asserted about a backup directory, in the
# current container. Results go to OUT_DIR/verify-LABEL.txt as PASS, INFO and BAD
# lines; the function returns 1 when any BAD line was written, after all were.
backup_verify() {
  local dir=$1 label=$2 out crc=0 got
  CHECK_OUT="$OUT_DIR/verify-$label.txt"
  CHECK_BADS=0
  : > "$CHECK_OUT"

  if dx test -f "$dir/holzcloud.sqlite"; then
    check_pass "file $dir/holzcloud.sqlite exists"
  else
    check_bad "file $dir/holzcloud.sqlite is missing"
  fi
  if dx test -f "$dir/MANIFEST"; then
    check_pass "file $dir/MANIFEST exists"
  else
    check_bad "file $dir/MANIFEST is missing"
  fi

  # Reader one: the binary's own check, on a COPY in a directory of its own,
  # exactly the way restore.sh does it. Never against the backup directory: the
  # binary opens the database read-write and would leave -wal and -shm files in
  # the backup.
  out=$(dx bash -c 't=$(mktemp -d) || exit 97
cp "$1/holzcloud.sqlite" "$t/holzcloud.sqlite" || { rm -rf "$t"; exit 98; }
rc=0
HOLZCLOUD_DATA_DIR="$t" /opt/holzcloud/holzcloud check 2>&1 || rc=$?
rm -rf "$t"
exit "$rc"' _ "$dir") && crc=0 || crc=$?
  if [ "$crc" -eq 0 ] && grep -q 'integrity: ok' <<< "$out"; then
    check_pass "integrity holzcloud-check: ok"
  else
    check_bad "integrity holzcloud-check: exit $crc, output '$(printf '%s' "$out" | tr '\n' ' ')'"
  fi

  # Reader two, independent of the binary: the C sqlite3 CLI and a stronger
  # pragma (the binary's check is quick_check). The VACUUM INTO file is itself
  # WAL-mode, and a plain read-only open of it failed on a newer sqlite, so the
  # file is opened through the URI with immutable=1, which also never creates a
  # -wal or -shm file next to the backup.
  got=$(dx sqlite3 "file:$dir/holzcloud.sqlite?immutable=1" "PRAGMA integrity_check;" 2>&1) || got="exit $?: $got"
  if [ "$got" = ok ]; then
    check_pass "integrity sqlite3 integrity_check: ok"
  else
    check_bad "integrity sqlite3 integrity_check: '$(printf '%s' "$got" | tr '\n' ' ')'"
  fi

  # The MANIFEST names what the backup came from.
  local manifest k v kf="$OUT_DIR/$label.txt" om ms
  manifest=$(dx cat "$dir/MANIFEST" 2>/dev/null || true)
  for k in timestamp version data_dir host; do
    v=$(printf '%s\n' "$manifest" | sed -n "s/^$k=//p" | head -n 1)
    if [ -n "$v" ]; then
      check_pass "MANIFEST $k=$v"
    else
      check_bad "MANIFEST has no $k= line"
    fi
  done

  # The directory holds password hashes and the CSRF key.
  om=$(dx stat -c '%U %a' "$dir" | tr -d '\r') || om="?"
  # backup-timer has written the KEY line already; it is not written twice.
  grep -q '^DIR-OWNER-MODE: ' "$kf" 2>/dev/null || echo "DIR-OWNER-MODE: $om" >> "$kf"
  if [ "$om" = "holzcloud 700" ]; then
    check_pass "owner and mode of the backup directory: $om"
  else
    check_bad "owner and mode of the backup directory: '$om', expected 'holzcloud 700' (it holds password hashes and the CSRF key)"
  fi

  # Content, by comparison and not by listing: the sha256 of every file of the
  # source data directory against the backup directory.
  tree_excluded_info
  tree_hash /opt/holzcloud/data > "$OUT_DIR/tree-src-$label.txt" || check_bad "tree: hashing /opt/holzcloud/data failed"
  tree_hash "$dir" > "$OUT_DIR/tree-$label.txt" || check_bad "tree: hashing $dir failed"
  tree_compare "$OUT_DIR/tree-src-$label.txt" "$OUT_DIR/tree-$label.txt" "src-$label"

  # Rows: every table of the backup database against the source state captured
  # just before the backup.
  if [ -f "$OUT_DIR/state-src.txt" ]; then
    rows_compare "$dir/holzcloud.sqlite" "$OUT_DIR/state-src.txt" "$label"
  else
    check_bad "rows: no $OUT_DIR/state-src.txt to compare the backup database with"
  fi

  tree_selftest "$OUT_DIR/tree-src-$label.txt"

  # Sizes, counts and the architecture, for the record.
  dir_sizes "$dir" BYTES SIZE >> "$kf"
  ms=$(media_served) || ms="error"
  echo "MEDIA-SERVED: $ms" >> "$kf"
  if [ "${ms#HTTP 200 }" = "$ms" ]; then
    check_info "MEDIA-SERVED is '$ms': the source does not serve its media file"
  elif [ "${ms#HTTP 200 }" = "$(dx sha256sum /tmp/hc-upload.jpg 2>/dev/null | cut -d' ' -f1)" ]; then
    check_info "MEDIA-SERVED equals the staged source image (/tmp/hc-upload.jpg)"
  else
    check_info "MEDIA-SERVED differs from the staged source image (/tmp/hc-upload.jpg)"
  fi

  [ "$CHECK_BADS" -eq 0 ]
}

# backup: BAK-01. The document's own manual-run line of the Backups section,
# timed, then the verification of the directory it made.
cmd_backup() {
  local lines line nl n nn outf dir wall free rc=0 vrc=0
  dx true >/dev/null 2>&1 \
    || die "container $NAME does not answer; run 'run.sh build up install baseline update postinstall' with INSTALL_FROM=old first"
  dx test -e /etc/systemd/system/holzcloud-backup.timer \
    || die "no holzcloud-backup.timer in $NAME; run 'run.sh postinstall' first (the document's Backups block installs the units)"
  grep -q '^ORDER:' "$OUT_DIR/baseline.txt" 2>/dev/null \
    || die "no ORDER: line in $OUT_DIR/baseline.txt; run 'run.sh baseline' first"
  mkdir -p "$OUT_DIR"

  # The source as it is before the backup (compared with the target later).
  capture_state src

  lines=$(doc_lines "Backups" 1) || exit 1
  line=$(grep -F -- '/backup.sh' <<< "$lines" || true)
  nl=$(printf '%s\n' "$line" | grep -c . || true)
  [ "$nl" -eq 1 ] \
    || die "$DEPLOY_DOC: bash block 1 of the section 'Backups' has $nl lines naming /backup.sh, exactly one (the manual run) is expected"
  free=$(dx df -Pm /opt/holzcloud/backups | awk 'NR == 2 { print $4 }') || free="?"

  DOC_TIMED=1
  doc_cmd "$line" || rc=$?
  DOC_TIMED=0
  n=$(steps_count)
  nn=$(printf '%02d' "$n")
  outf="$OUT_DIR/step-$nn.txt"
  dir=$(sed -n 's/^=== Backup complete: \(.*\) ===$/\1/p' "$outf" | tail -n 1)
  wall=$(awk '$1 == "WALL" { printf "%.3f", $3 - $2 }' "$outf")
  {
    echo "DIR: ${dir:-none}"
    echo "WALL-S: ${wall:-?}"
    # The script's own threshold is MIN_FREE_MB 1024; this is informational.
    echo "FREE-MB: ${free:-?}"
  } > "$OUT_DIR/backup.txt"

  if [ "$rc" -ne 0 ] || [ -z "$dir" ]; then
    CHECK_OUT="$OUT_DIR/verify-backup.txt"
    CHECK_BADS=0
    : > "$CHECK_OUT"
    check_bad "backup: the document's line ended with status $rc and named no '=== Backup complete: DIR ===' line (step $nn)"
    vrc=1
  else
    backup_verify "$dir" backup || vrc=1
  fi

  leak_check
  if [ "$vrc" -ne 0 ]; then
    say "run.sh: BACKUP FAILED, see the BAD lines of verify-backup.txt"
    return 1
  fi
  say "BACKUP OK"
}

# transfer_into SRC_DIR: runs in the target's context (on_target).
transfer_into() {
  local src_dir=$1 base sl tl nfiles f tf rc=0
  base=$(basename "$src_dir")
  mkdir -p "$OUT_DIR"
  tf="$OUT_DIR/transfer.txt"
  : > "$tf"
  deviation "DEVIATION[transfer]: the backup travels as a tar stream through the host (docker cp SOURCE:dir - into docker cp - TARGET:/opt/holzcloud/backups); on two real machines it would be rsync or scp, which the document leaves to the operator. Proof of a lossless transfer is the sha256 of every file on both sides"
  deviation "DEVIATION[fixture]: the login password file, the AI key file and the test image are fixtures that stand in for what an operator remembers; none of them is part of any backup, and they cross from container to container by exec-pipe, never through a host file"

  docker exec "$SRC_NAME" test -d "$src_dir" || die "the source $SRC_NAME has no directory $src_dir"
  docker exec "$SRC_NAME" test -f "$src_dir/holzcloud.sqlite" || die "$src_dir of $SRC_NAME holds no holzcloud.sqlite"
  dx mkdir -p /opt/holzcloud/backups || die "could not create /opt/holzcloud/backups in $NAME"

  if ! docker cp "$SRC_NAME:$src_dir" - | docker cp - "$NAME:/opt/holzcloud/backups"; then
    echo "BAD transfer: the tar stream from $SRC_NAME:$src_dir to $NAME:/opt/holzcloud/backups failed" | tee -a "$tf"
    return 1
  fi

  # The fixtures. The two secrets are written under umask 077; the test image is
  # no secret and the service user has to read it (capture_state hashes it as
  # that user), so it keeps the mode a docker cp gave it in the source.
  for f in /root/.hc-login-pass /root/.hc-key; do
    docker exec "$SRC_NAME" cat "$f" \
      | docker exec -i "$NAME" bash -c 'umask 077; cat > "$1"; test -s "$1"' _ "$f" \
      || { echo "BAD transfer: $f could not be carried from $SRC_NAME to $NAME" | tee -a "$tf"; rc=1; }
  done
  docker exec "$SRC_NAME" cat /tmp/hc-upload.jpg \
    | docker exec -i "$NAME" bash -c 'umask 022; cat > "$1"; test -s "$1"' _ /tmp/hc-upload.jpg \
    || { echo "BAD transfer: /tmp/hc-upload.jpg could not be carried from $SRC_NAME to $NAME" | tee -a "$tf"; rc=1; }

  # The order number (an order number, no secret) so capture_state finds it here.
  grep -m1 '^ORDER:' "$SRC_OUT_DIR/baseline.txt" > "$OUT_DIR/baseline.txt" \
    || { echo "BAD transfer: $SRC_OUT_DIR/baseline.txt has no ORDER: line" | tee -a "$tf"; rc=1; }

  sl=$(docker exec "$SRC_NAME" bash -c 'cd "$1" && find . -type f -print0 | sort -z | xargs -0 sha256sum' _ "$src_dir") || sl=""
  tl=$(dx bash -c 'cd "/opt/holzcloud/backups/$1" && find . -type f -print0 | sort -z | xargs -0 sha256sum' _ "$base") || tl=""
  nfiles=$(printf '%s\n' "$sl" | grep -c . || true)
  {
    echo "DIR: /opt/holzcloud/backups/$base"
    echo "SOURCE: $SRC_NAME:$src_dir"
    echo "FILES: $nfiles"
  } >> "$tf"
  if [ "$nfiles" -gt 0 ] && [ "$sl" = "$tl" ]; then
    echo "TRANSFER: equal" | tee -a "$tf"
  else
    echo "BAD transfer: the sha256 lists of $src_dir in $SRC_NAME and of /opt/holzcloud/backups/$base in $NAME differ or are empty ($nfiles files in the source)" | tee -a "$tf"
    rc=1
  fi
  [ "$rc" -eq 0 ]
}

# transfer: D-04. The source directory is BACKUP_DIR_PATH when set; else the
# backup the timer made (OUT_DIR/timer.txt, plan 22-02: that is the one that gets
# restored); else the manual one (OUT_DIR/backup.txt).
cmd_transfer() {
  local src_dir=$BACKUP_DIR_PATH
  if [ -z "$src_dir" ] && [ -f "$OUT_DIR/timer.txt" ]; then
    src_dir=$(sed -n 's/^DIR: //p' "$OUT_DIR/timer.txt" | head -n 1)
  fi
  if [ -z "$src_dir" ] && [ -f "$OUT_DIR/backup.txt" ]; then
    src_dir=$(sed -n 's/^DIR: //p' "$OUT_DIR/backup.txt" | head -n 1)
  fi
  case "$src_dir" in
    "" | none) die "no backup directory known: run 'run.sh backup' first, or set BACKUP_DIR_PATH" ;;
  esac
  dx true >/dev/null 2>&1 || die "the source container $NAME does not answer"
  docker exec "$TGT_NAME" true >/dev/null 2>&1 \
    || die "the target container $TGT_NAME does not answer; bring it up with 'run.sh build up install' under NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR"
  on_target transfer_into "$src_dir" || { say "run.sh: TRANSFER FAILED, see $TGT_OUT_DIR/transfer.txt"; return 1; }
  say "TRANSFER OK"
}

# restore_into: runs in the target's context (on_target). The document's restore
# line, played with the answer on stdin, then the first assertions: the setup form
# open before (the empty database of the first start), the restored database live
# afterwards (login with the old password, the setup form closed).
restore_into() {
  local dir out f ready lg before after n0 failed rrc=0
  dx test -x /opt/holzcloud/holzcloud \
    || die "no /opt/holzcloud/holzcloud in $NAME; bring the target up with 'run.sh build up install' first"
  [ -f "$OUT_DIR/transfer.txt" ] && grep -q '^TRANSFER: equal' "$OUT_DIR/transfer.txt" \
    || die "no $OUT_DIR/transfer.txt with 'TRANSFER: equal'; run 'run.sh transfer' first"
  dir=$(sed -n 's/^DIR: //p' "$OUT_DIR/transfer.txt" | head -n 1)
  dx test -f "$dir/holzcloud.sqlite" || die "the target $NAME has no $dir/holzcloud.sqlite; run 'run.sh transfer' first"
  for f in /root/.hc-login-pass /root/.hc-key /tmp/hc-upload.jpg; do
    dx test -s "$f" || die "the target $NAME has no $f; run 'run.sh transfer' first (it carries the fixtures)"
  done

  CHECK_OUT="$OUT_DIR/restore.txt"
  CHECK_BADS=0
  : > "$CHECK_OUT"
  key_line "DIR: $dir"

  before=$(form get /admin/setup /root/.hc-jar-probe) || before="HTTP error"
  dx rm -f /root/.hc-jar-probe || true
  key_line "SETUP-BEFORE: $before"
  if [ "$before" = "HTTP 200 -" ]; then
    check_pass "setup form open before the restore (the empty database of the first start)"
  else
    check_bad "setup form before the restore: expected 'HTTP 200 -', got '$before'"
  fi

  RESTORE_DIR=$dir
  n0=$(steps_count)
  doc_play "Restore from a backup" 1 || rrc=$?
  RESTORE_DIR=""
  failed=$(steps_failed_since "$n0")
  if [ -n "$failed" ]; then
    check_bad "restore: a counted step of the document's restore block ended non-zero: $(printf '%s' "$failed" | tr '\n' ' ')"
  else
    check_pass "restore: the document's restore block ran without a failing step"
  fi

  if ! wait_ready; then
    check_bad "restore: /healthz did not answer within ${READY_TIMEOUT}s after the restore"
  fi
  ready=$(readyz_line) || ready="unavailable"
  key_line "READYZ: $ready"
  if printf '%s' "$ready" | dxi jq -e '.status == "ok" and .integrity == "ok"' >/dev/null 2>&1; then
    check_pass "readyz status ok, integrity ok"
  else
    check_bad "readyz: not status ok and integrity ok: $ready"
  fi

  lg=$(form post /admin/login login@example.test /root/.hc-login-pass /root/.hc-jar-restore) || lg="HTTP error"
  dx rm -f /root/.hc-jar-restore || true
  key_line "LOGIN: $lg"
  if [ "$lg" = "HTTP 303 /admin/" ]; then
    check_pass "login with the password the user had before: 303 to /admin/"
  else
    check_bad "login: expected 'HTTP 303 /admin/', got '$lg'"
  fi

  after=$(form get /admin/setup /root/.hc-jar-probe) || after="HTTP error"
  dx rm -f /root/.hc-jar-probe || true
  key_line "SETUP-AFTER: $after"
  if [ "$after" = "HTTP 404 -" ]; then
    check_pass "setup form closed after the restore (the restored database has users)"
  else
    check_bad "setup form after the restore: expected 'HTTP 404 -', got '$after'"
  fi

  restore_proof "$dir"

  leak_check
  if [ "$CHECK_BADS" -gt 0 ]; then
    say "run.sh: RESTORE FAILED, $CHECK_BADS assertion(s), see restore.txt"
    return 1
  fi
  say "RESTORE OK"
}

# perm_check PATH WANT: the owner:group and, when WANT has a second word, the octal
# mode of PATH in the current container, as a PASS or a BAD line of CHECK_OUT.
perm_check() {
  local path=$1 want=$2 got cmp
  got=$(dx stat -c '%U:%G %a' "$path" 2>/dev/null | tr -d '\r') || got="?"
  cmp=$got
  case "$want" in *" "*) ;; *) cmp=${got%% *} ;; esac
  if [ "$cmp" = "$want" ]; then
    check_pass "owner and mode of $path: $got"
  else
    check_bad "owner and mode of $path: '$got', expected '$want'"
  fi
}

# restore_proof BACKUP_DIR: everything after the restore that makes it a proof and
# not a path, in the target's context (on_target): the measurements, the set-aside
# directory, ownership, the whole-directory comparison, the media file over HTTP,
# the marker page, the state of the source against the state of the target, and
# last of all `check`, which restarts the unit. Every failure is a BAD line.
restore_proof() {
  local dir=$1 tl kv asides na ac media_row ms want mcode mbody np crc=0 p c b
  local back made hr f unitf dropins envf mf

  # --- time: the split from the per-line timestamps of restore-run.txt, the
  # container's own clock (D-14, D-15).
  tl=$(awk '
    $2 == "Verifying" && $3 == "snapshot..." { v = $1 }
    $2 == "Stopping" { s = $1 }
    $2 == "Starting" { t = $1 }
    $2 == "===" && $3 == "Restore" && $4 == "complete" { c = $1 }
    $1 == "WALL" { w0 = $2; w1 = $3 }
    END {
      if (v != "" && s != "") printf "VERIFY-S: %.3f\n", s - v; else print "VERIFY-S: ?"
      if (s != "" && t != "") printf "STOP-MOVE-COPY-S: %.3f\n", t - s; else print "STOP-MOVE-COPY-S: ?"
      if (t != "" && c != "") printf "START-READY-S: %.3f\n", c - t; else print "START-READY-S: ?"
      if (w0 != "" && w1 != "") printf "WALL-S: %.3f\n", w1 - w0; else print "WALL-S: ?"
    }' "$OUT_DIR/restore-run.txt" 2>/dev/null) || tl=""
  while IFS= read -r kv; do
    [ -z "$kv" ] || key_line "$kv"
  done <<< "$tl"
  check_info "the readiness loop of restore.sh polls once a second, so START-READY-S has a resolution of about one second; the times come from the amd64 emulation named in HOST-ARCH and CONTAINER-ARCH, not from a real server"

  # --- size: the restored data directory and the backup it came from.
  while IFS= read -r kv; do
    [ -z "$kv" ] || key_line "$kv"
  done <<< "$(dir_sizes /opt/holzcloud/data DATA-BYTES "")"
  key_line "BACKUP-BYTES: $(dx du -sb "$dir" | cut -f1)"

  # --- the set-aside directory: the empty database of the first start.
  asides=$(dx bash -c 'ls -1d /opt/holzcloud/data.before-restore-* 2>/dev/null' | tr -d '\r' || true)
  na=$(printf '%s\n' "$asides" | grep -c . || true)
  if [ "$na" -eq 1 ]; then
    check_pass "set-aside: exactly one $asides"
    ac=$(dx sqlite3 "file:$asides/holzcloud.sqlite?immutable=1" "select count(*) from users" 2>&1) || ac="unreadable"
    if [ "$ac" = 0 ]; then
      check_pass "set-aside database is the empty one of the first start (users: 0)"
    else
      check_bad "set-aside database: users count '$ac', the empty first-start database has 0"
    fi
  else
    check_bad "set-aside: $na directories match /opt/holzcloud/data.before-restore-*, exactly one is expected"
  fi
  check_info "restore.sh moves the whole data directory aside, so the -wal and -shm files of the replaced database went with it; none can reach the restored file, and the proof is that the restored database passes the row comparison and the integrity verdict"

  # --- owner and mode of what was restored.
  perm_check /opt/holzcloud/data "holzcloud:holzcloud 700"
  perm_check /opt/holzcloud/data/holzcloud.sqlite "holzcloud:holzcloud"
  perm_check /opt/holzcloud/data/csrf.key "holzcloud:holzcloud 600"
  mf=$(dx bash -c 'find /opt/holzcloud/data/media -type f | head -n 1' | tr -d '\r') || mf=""
  if [ -n "$mf" ]; then
    perm_check "$mf" "holzcloud:holzcloud"
  else
    check_bad "no media file below /opt/holzcloud/data/media after the restore"
  fi

  # --- content: the whole restored data directory against the source's and the
  # backup's lists (made in the source container by `backup`).
  tree_excluded_info
  tree_hash /opt/holzcloud/data > "$OUT_DIR/tree-restored.txt" || check_bad "tree: hashing the restored /opt/holzcloud/data failed"
  if [ -f "$SRC_OUT_DIR/tree-src-backup.txt" ]; then
    tree_compare "$SRC_OUT_DIR/tree-src-backup.txt" "$OUT_DIR/tree-restored.txt" src-restored
  else
    check_bad "tree: $SRC_OUT_DIR/tree-src-backup.txt is missing; run 'run.sh backup' in the source first"
  fi
  if [ -f "$SRC_OUT_DIR/tree-backup.txt" ]; then
    tree_compare "$SRC_OUT_DIR/tree-backup.txt" "$OUT_DIR/tree-restored.txt" backup-restored
  else
    check_bad "tree: $SRC_OUT_DIR/tree-backup.txt is missing; run 'run.sh backup' in the source first"
  fi

  # --- the media file over HTTP has the hash it had before the backup.
  ms=$(media_served) || ms="error"
  key_line "MEDIA-SERVED: $ms"
  want=$(sed -n 's/^MEDIA-SERVED: //p' "$SRC_OUT_DIR/backup.txt" 2>/dev/null | head -n 1)
  if [ -n "$want" ] && [ "$ms" = "$want" ] && [ "${ms#HTTP 200 }" != "$ms" ]; then
    check_pass "media served over HTTP: same status and sha256 as in the source ($ms)"
  else
    check_bad "media served over HTTP: '$ms' in the target, '$want' in the source"
  fi

  # --- the marker page, written after the update and carried by the backup.
  mcode=$(http_code /after-update) || mcode=error
  mbody=$(http_body /after-update) || mbody=""
  if [ "$mcode" = 200 ] && grep -q 'Written after the update' <<< "$mbody"; then
    check_pass "marker page: GET /after-update answers 200 with its sentence"
  else
    check_bad "marker page: GET /after-update answered $mcode (200 with the sentence 'Written after the update' expected)"
  fi

  # --- state of the source against state of the target, and the proof that the
  # comparison fails when it is altered.
  capture_state tgt
  if [ -f "$SRC_OUT_DIR/state-src.txt" ]; then
    compare_states "$SRC_OUT_DIR/state-src.txt" "$OUT_DIR/state-tgt.txt" "$OUT_DIR/compare-src-tgt.txt" || crc=1
    p=$(grep -c '^PASS' "$OUT_DIR/compare-src-tgt.txt" || true)
    b=$(grep -c '^BAD' "$OUT_DIR/compare-src-tgt.txt" || true)
    if [ "$crc" -eq 0 ]; then
      check_pass "state comparison source against target: $p pass, $b bad (compare-src-tgt.txt)"
    else
      check_bad "state comparison source against target: $p pass, $b bad, see compare-src-tgt.txt; the BAD lines are: $(grep '^BAD' "$OUT_DIR/compare-src-tgt.txt" | sed 's/^BAD *//' | tr '\n' ';' | cut -c1-400)"
    fi
    compare_selftest "$SRC_OUT_DIR/state-src.txt" "$OUT_DIR/state-tgt.txt"
    # restore.sh copies the database as root under its own umask, so the mode may
    # differ from the 600 the service gave the live file; the 700 data directory
    # is what keeps it private. Recorded, not asserted (a finding for the script).
    check_info "owner and mode of holzcloud.sqlite: source '$(st_get "$SRC_OUT_DIR/state-src.txt" owner)', restored '$(st_get "$OUT_DIR/state-tgt.txt" owner)' (the data directory is $(dx stat -c '%a' /opt/holzcloud/data | tr -d '\r'))"
  else
    check_bad "state: $SRC_OUT_DIR/state-src.txt is missing; run 'run.sh backup' in the source first"
  fi

  # --- what comes back from the backup and what the box got from elsewhere,
  # derived from the machine, not typed.
  back=$(dx bash -c 'for e in $(ls -1A "$1"); do if [ -e "$2/$e" ]; then echo "$e"; fi; done' _ "$dir" /opt/holzcloud/data | tr '\r\n' '  ' | sed 's/ *$//')
  made=$(dx bash -c 'for e in $(ls -1A "$2"); do if [ ! -e "$1/$e" ]; then echo "$e"; fi; done' _ "$dir" /opt/holzcloud/data | tr '\r\n' '  ' | sed 's/ *$//')
  key_line "COMES-BACK: $back (the top-level entries of the backup directory that are in the restored data directory)"
  check_info "in the restored data directory and not from the backup, made by the service's own start: ${made:-nothing}"
  unitf=$(dx systemctl show -p FragmentPath --value holzcloud | tr -d '\r') || unitf=""
  dropins=$(dx systemctl show -p DropInPaths --value holzcloud | tr -d '\r') || dropins=""
  envf=$(dx systemctl show -p EnvironmentFiles --value holzcloud | tr -d '\r') || envf=""
  key_line "HAND-REBUILT: the unit file ${unitf:-?} (written by the install, no backup holds it)"
  if [ -n "$dropins" ]; then
    key_line "HAND-REBUILT: the drop-ins $dropins (written by the install or the harness, no backup holds them)"
  else
    key_line "HAND-REBUILT: no drop-ins on the unit"
  fi
  if [ -n "$envf" ]; then
    key_line "HAND-REBUILT: the environment files $envf (no backup holds them; HOLZCLOUD_SECRET_KEY and payment or mail credentials live there, never in the data directory)"
  else
    key_line "HAND-REBUILT: no environment file is configured on this unit; on a real server HOLZCLOUD_SECRET_KEY and payment or mail credentials live in one, never in the data directory, so no backup holds them"
  fi
  for f in /root/.hc-login-pass /root/.hc-key /tmp/hc-upload.jpg; do
    if dx test -e "$f"; then
      key_line "HAND-REBUILT: $f (a test fixture carried from the source by exec-pipe; it stands in for what the operator remembers, no backup holds it)"
    fi
  done

  # --- last: `check` through cmd_check in a subshell, so that its exit becomes
  # a BAD line and not the end of this subcommand. It restarts the unit, which is
  # why it runs after everything above.
  ( cmd_check ) || ccrc=$?
  np=0
  if [ -f "$OUT_DIR/check.txt" ]; then np=$(grep -c '^PASS' "$OUT_DIR/check.txt" || true); fi
  if [ "$ccrc" -eq 0 ] && [ "${np:-0}" -eq 17 ]; then
    check_pass "check: 17 of 17 assertions passed (check.txt)"
  else
    check_bad "check: ${np:-0} of 17 assertions passed (exit status $ccrc), see check.txt; the first BAD line is: $(grep -m1 '^BAD' "$OUT_DIR/check.txt" 2>/dev/null | sed 's/^BAD *//' | cut -c1-200)"
  fi
}

# restore: BAK-02, on the target, through on_target only.
cmd_restore() {
  docker exec "$TGT_NAME" true >/dev/null 2>&1 \
    || die "the target container $TGT_NAME does not answer; bring it up with 'run.sh build up install' under NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR"
  on_target restore_into
}

# backup-timer: BAK-03. The timer has never fired in any run, because its real
# schedule is 03:15. A drop-in that exists in this container only (D-11) puts the
# next run TIMER_OFFSET seconds ahead of the container's clock; the shipped units
# in deploy/ and in /etc/systemd/system are untouched and the clock is not
# changed (it would also move the journal, the -mtime retention and the stamp of
# Persistent=true). Only daemon-reload is run: restarting the timer would stamp
# LAST with the restart time and make "LAST moved" mean nothing. "Observed" is
# three raw files (timer-before.txt, timer-after.txt, timer-journal.txt) and the
# proof of who started the service (TriggeredBy, Result, ExecMainStatus). The
# directory is the one the journal names, never "the newest": the backups
# directory already holds the postinstall and the Updating backups.
cmd_backup_timer() {
  local dropin_dir=/etc/systemd/system/holzcloud-backup.timer.d
  local dropin=/etc/systemd/system/holzcloud-backup.timer.d/container.conf
  local enabled started both clock when now2 target left i act st tf dtxt jl dir om
  local trig res es t0 t1 nb na lb la bads=0 vrc=0 timer_bads=0
  case "$TIMER_OFFSET" in "" | *[!0-9]*) die "TIMER_OFFSET '$TIMER_OFFSET' is not a number of seconds" ;; esac
  case "$TIMER_WAIT" in "" | *[!0-9]*) die "TIMER_WAIT '$TIMER_WAIT' is not a number of seconds" ;; esac
  [ "$TIMER_OFFSET" -ge 10 ] || die "TIMER_OFFSET $TIMER_OFFSET is too short: daemon-reload and the first reading take several seconds under emulation, use 10 or more"
  dx true >/dev/null 2>&1 \
    || die "container $NAME does not answer; run 'run.sh build up install baseline update postinstall' with INSTALL_FROM=old first"
  enabled=$(dx systemctl is-enabled holzcloud-backup.timer 2>/dev/null | tr -d '\r') || enabled=""
  [ "$enabled" = enabled ] \
    || die "holzcloud-backup.timer is '${enabled:-not installed}' in $NAME, not enabled; run 'run.sh postinstall' first (the document's Backups block enables it)"
  started=$(dx systemctl show holzcloud-backup.service -p ExecMainStartTimestamp --value | tr -d '\r') || started="?"
  [ -z "$started" ] \
    || die "holzcloud-backup.service has run in $NAME already (ExecMainStartTimestamp '$started'): only the timer's own first run counts, and a manual backup does not start the unit; use a fresh container"
  grep -q '^DIR: ' "$OUT_DIR/backup.txt" 2>/dev/null \
    || die "no DIR: line in $OUT_DIR/backup.txt; run 'run.sh backup' first, so that the manual run is known and the timer's directory can be told apart from it"
  mkdir -p "$OUT_DIR"

  # The drop-in (D-11): the empty OnCalendar= first, because OnCalendar is a
  # list and only the empty assignment resets it (else 03:15 stays active).
  both=$(dx bash -c 'now=$(date -u +%s); echo "$(date -u -d "@$now" "+%Y-%m-%d %H:%M:%S") UTC $(date -u -d "@$((now + $1))" +%H:%M:%S) $((now + $1))"' _ "$TIMER_OFFSET" | tr -d '\r') \
    || die "could not read the clock of $NAME"
  clock=$(printf '%s' "$both" | cut -d' ' -f1-3)
  when=$(printf '%s' "$both" | cut -d' ' -f4)
  target=$(printf '%s' "$both" | cut -d' ' -f5)
  dtxt=$(printf '[Timer]\nOnCalendar=\nOnCalendar=*-*-* %s\nRandomizedDelaySec=0\nAccuracySec=1s\n' "$when")
  printf '%s\n' "$dtxt" > "$OUT_DIR/timer-dropin.txt"
  printf '%s\n' "$dtxt" | dxi bash -c 'mkdir -p "$1" && cat > "$2"' _ "$dropin_dir" "$dropin" \
    || die "could not write $dropin in $NAME"
  deviation "DEVIATION[timer]: the container-only drop-in $dropin sets OnCalendar to an empty value (which resets the list) and then to *-*-* $when (TIMER_OFFSET=$TIMER_OFFSET seconds after the container's clock read $clock), RandomizedDelaySec=0 and AccuracySec=1s; the shipped holzcloud-backup.timer (OnCalendar=*-*-* 03:15:00, Persistent=true, RandomizedDelaySec=300) and its service are untouched, the clock is not changed, and only systemctl daemon-reload was run, no restart of the timer. It exists because the real schedule is 03:15 and a test cannot wait for it. What it cannot show, and what is therefore NOT TESTED: the real 03:15 window, Persistent=true after a missed run, and RandomizedDelaySec"
  dx systemctl daemon-reload || die "systemctl daemon-reload failed in $NAME"

  now2=$(dx date -u +%s | tr -d '\r') || now2=0
  left=$((target - now2))
  if [ "$left" -lt 3 ]; then
    die "only $left second(s) were left before the drop-in's time $when after daemon-reload: the next run is tomorrow; run again with a larger TIMER_OFFSET (now $TIMER_OFFSET)"
  fi

  # Before the fire.
  dx systemctl list-timers holzcloud-backup.timer --no-pager > "$OUT_DIR/timer-before.txt" 2>&1 || true
  nb=$(dx systemctl show holzcloud-backup.timer -p NextElapseUSecRealtime --value | tr -d '\r') || nb=""
  lb=$(dx systemctl show holzcloud-backup.timer -p LastTriggerUSec --value | tr -d '\r') || lb=""

  # Poll once a second, never one long sleep: the oneshot is finished when its
  # start timestamp is set and its state is no longer activating or active.
  i=0
  st=""
  while [ "$i" -le "$TIMER_WAIT" ]; do
    st=$(dx systemctl show holzcloud-backup.service -p ExecMainStartTimestamp --value | tr -d '\r') || st=""
    act=$(dx systemctl show holzcloud-backup.service -p ActiveState --value | tr -d '\r') || act=""
    if [ -n "$st" ]; then
      case "$act" in activating | active | reloading) ;; *) break ;; esac
    fi
    sleep 1
    i=$((i + 1))
  done
  if [ -z "$st" ] || [ "$i" -gt "$TIMER_WAIT" ]; then
    say "run.sh: holzcloud-backup.service did not run within ${TIMER_WAIT}s of the drop-in (time $when); its journal and the timer:"
    dx journalctl -u holzcloud-backup.service --no-pager -n 40 >&2 || true
    dx systemctl list-timers holzcloud-backup.timer --no-pager >&2 || true
    die "the timer did not fire; the drop-in stays in $NAME for a look (systemctl cat holzcloud-backup.timer)"
  fi

  # After the fire.
  dx systemctl list-timers holzcloud-backup.timer --no-pager > "$OUT_DIR/timer-after.txt" 2>&1 || true
  dx journalctl -u holzcloud-backup.service --no-pager -o short-precise > "$OUT_DIR/timer-journal.txt" 2>&1 || true
  na=$(dx systemctl show holzcloud-backup.timer -p NextElapseUSecRealtime --value | tr -d '\r') || na=""
  la=$(dx systemctl show holzcloud-backup.timer -p LastTriggerUSec --value | tr -d '\r') || la=""
  trig=$(dx systemctl show holzcloud-backup.service -p TriggeredBy --value | tr -d '\r') || trig=""
  res=$(dx systemctl show holzcloud-backup.service -p Result --value | tr -d '\r') || res=""
  es=$(dx systemctl show holzcloud-backup.service -p ExecMainStatus --value | tr -d '\r') || es=""
  t0=$st
  t1=$(dx systemctl show holzcloud-backup.service -p ExecMainExitTimestamp --value | tr -d '\r') || t1=""
  dir=$(sed -n 's/.*=== Backup complete: \(.*\) ===$/\1/p' "$OUT_DIR/timer-journal.txt" | tail -n 1)
  om="?"
  if [ -n "$dir" ]; then om=$(dx stat -c '%U %a' "$dir" 2>/dev/null | tr -d '\r') || om="?"; fi

  tf="$OUT_DIR/timer.txt"
  CHECK_OUT=$tf
  CHECK_BADS=0
  : > "$tf"
  key_line "TRIGGERED-BY: $trig"
  key_line "RESULT: $res"
  key_line "EXIT-STATUS: $es"
  key_line "START: $t0"
  key_line "END: $t1"
  key_line "DROPIN-TIME: $when"
  key_line "CLOCK: $clock"
  key_line "NEXT-BEFORE: ${nb:-n/a}"
  key_line "NEXT-AFTER: ${na:-n/a}"
  key_line "LAST-BEFORE: ${lb:-n/a}"
  key_line "LAST-AFTER: ${la:-n/a}"
  key_line "DIR: ${dir:-none}"
  key_line "DIR-OWNER-MODE: $om"
  key_line "HOST-ARCH: $(docker info --format '{{.Architecture}}')"
  key_line "CONTAINER-ARCH: $(dx uname -m | tr -d '\r')"

  # The assertions.
  if [ "$trig" = holzcloud-backup.timer ]; then
    check_pass "the service was started by holzcloud-backup.timer (TriggeredBy), not by hand"
  else
    check_bad "TriggeredBy is '$trig', expected holzcloud-backup.timer"
  fi
  if [ "$res" = success ] && [ "$es" = 0 ]; then
    check_pass "Result success and ExecMainStatus 0"
  else
    check_bad "Result '$res' and ExecMainStatus '$es', expected success and 0"
  fi
  if [ -n "$dir" ] && grep -q '=== Backup complete' "$OUT_DIR/timer-journal.txt"; then
    check_pass "the journal of the unit holds the script's '=== Backup complete: $dir ===' line"
  else
    check_bad "the journal of the unit holds no '=== Backup complete: DIR ===' line (timer-journal.txt)"
  fi
  if [ -n "$dir" ] && dx test -d "$dir"; then
    check_pass "the directory the journal names exists: $dir"
  else
    check_bad "the directory the journal names ('${dir:-none}') does not exist"
  fi
  if [ "$om" = "holzcloud 700" ]; then
    check_pass "owner and mode of the timer's backup directory: $om (the hardened unit runs as the service user)"
  else
    check_bad "owner and mode of the timer's backup directory: '$om', expected 'holzcloud 700' (it holds password hashes and the CSRF key)"
  fi
  case "${lb:-n/a}" in
    n/a | "" | "0") check_pass "LAST before the run was empty (n/a): the timer had never fired" ;;
    *) check_bad "LAST before the run was '$lb', the timer had fired before (or was restarted)" ;;
  esac
  case "${la:-n/a}" in
    n/a | "" | "0") check_bad "LAST after the run is empty: the timer's LAST did not move" ;;
    *) check_pass "LAST after the run is set: $la" ;;
  esac
  if [ -n "$na" ] && [ "$na" != "$nb" ]; then
    check_pass "NEXT moved from '$nb' to '$na'"
  else
    check_bad "NEXT did not move: before '$nb', after '$na'"
  fi
  if [ -n "$dir" ] && [ "$dir" != "$(sed -n 's/^DIR: //p' "$OUT_DIR/backup.txt" | head -n 1)" ]; then
    check_pass "the timer made its own directory, not the manual run's ($(sed -n 's/^DIR: //p' "$OUT_DIR/backup.txt" | head -n 1))"
  else
    check_bad "the timer's directory '${dir:-none}' equals the manual run's"
  fi
  timer_bads=$CHECK_BADS

  # The backup that is going to be restored gets the same proof as the manual
  # one: two integrity readers, the whole-directory comparison, row counts.
  if [ -n "$dir" ] && dx test -f "$dir/holzcloud.sqlite"; then
    backup_verify "$dir" timer || vrc=1
  else
    CHECK_OUT="$OUT_DIR/verify-timer.txt"
    CHECK_BADS=0
    : > "$CHECK_OUT"
    check_bad "backup-timer: the timer's directory '${dir:-none}' holds no holzcloud.sqlite to verify"
    vrc=1
  fi

  leak_check
  bads=$timer_bads
  if [ "$bads" -gt 0 ]; then
    say "run.sh: BACKUP-TIMER FAILED, $bads assertion(s) of timer.txt are BAD"
    return 1
  fi
  if [ "$vrc" -ne 0 ]; then
    say "run.sh: BACKUP-TIMER FAILED: the timer behaved, but its backup has BAD lines, see verify-timer.txt"
    return 1
  fi
  say "BACKUP-TIMER OK"
}

# ---------------------------------------------------------------------------
# Phase 22, plan 02: the refusals of restore.sh and backup.sh, played once with
# the live data shown untouched (D-09), and the fixture of the second data point.
# ---------------------------------------------------------------------------

# neg_inv: the invariants of the LIVE data in the current container, one "KEY
# value" line each. The tree is the whole data directory by the same exclusions as
# every other comparison (the SQLite family is out: the running service writes to
# it, so a byte hash of it is no invariant); the database is asked for its
# integrity through the immutable URI instead.
neg_inv() {
  local th
  th=$(tree_hash /opt/holzcloud/data | dxi sha256sum | cut -d' ' -f1) || th="?"
  dx bash -c 'd=/opt/holzcloud/data
echo "inode $(stat -c %i "$d")"
echo "invocation $(systemctl show holzcloud -p InvocationID --value)"
echo "activeenter $(systemctl show holzcloud -p ActiveEnterTimestamp --value)"
echo "asides $(ls -1d /opt/holzcloud/data.before-restore-* 2>/dev/null | wc -l)"
echo "active $(systemctl is-active holzcloud)"
echo "readyz $(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/readyz)"
echo "integrity $(sqlite3 "file:$d/holzcloud.sqlite?immutable=1" "PRAGMA integrity_check;" 2>&1 | tr "\n" " ")"' | tr -d '\r'
  printf 'tree %s\n' "$th"
}

# neg_finish LETTER NAME BEFORE OUTPUT EXIT PATTERN DESCRIPTION: the assertions of
# one case. The exit status must be 1, the output must match PATTERN, and every
# invariant of the live data must be equal to what it was before the case. A
# case that holds on all counts is `NEGATIVE-RESULT: refused`: it was refused and
# the live data was not touched.
neg_finish() {
  local l=$1 name=$2 before=$3 out=$4 ec=$5 pat=$6 desc=$7 after k b a b0 line n=0
  after=$(neg_inv)
  key_line "CASE $l $name: exit=$ec"
  b0=$CHECK_BADS
  while IFS= read -r line; do
    n=$((n + 1))
    [ "$n" -le 25 ] || break
    check_info "case $l output: $line"
  done <<< "$out"
  if [ "$ec" = 1 ]; then
    check_pass "case $l: exit status 1"
  else
    check_bad "case $l: exit status '$ec', expected 1"
  fi
  if printf '%s\n' "$out" | grep -Eq -- "$pat"; then
    check_pass "case $l: the output $desc"
  else
    check_bad "case $l: the output does not $desc"
  fi
  for k in inode tree invocation activeenter asides active readyz integrity; do
    b=$(printf '%s\n' "$before" | sed -n "s/^$k //p" | head -n 1)
    a=$(printf '%s\n' "$after" | sed -n "s/^$k //p" | head -n 1)
    if [ -n "$b" ] && [ "$a" = "$b" ]; then
      check_pass "case $l: invariant $k unchanged ($b)"
    else
      check_bad "case $l: invariant $k changed or unreadable: before '$b', after '$a'"
    fi
  done
  if [ "$CHECK_BADS" -eq "$b0" ]; then
    key_line "NEGATIVE-RESULT: refused"
  fi
}

# neg_restore DIR ANSWER: restore.sh on DIR with ANSWER on stdin, as root like the
# document's `sudo`; prints its output and, last, "EXIT n". The answer comes from
# stdin on purpose, so that a restore that passed its check would go on.
neg_restore() {
  printf '%s\n' "$2" | dxi bash -c 'cd /; /opt/holzcloud/deploy/restore.sh "$1" 2>&1; echo "EXIT $?"' _ "$1" | tr -d '\r'
}

# restore_negative_into: runs in the target's context (on_target), after a
# successful restore: the live data are the restored ones.
restore_negative_into() {
  local dir out ec before bdir ndir0 ndir1 proof asz
  dx test -x /opt/holzcloud/deploy/restore.sh || die "no /opt/holzcloud/deploy/restore.sh in $NAME; bring the target up with 'run.sh build up install' first"
  [ -f "$OUT_DIR/restore.txt" ] && grep -q '^DIR: ' "$OUT_DIR/restore.txt" && grep -q '^SETUP-AFTER: HTTP 404 -$' "$OUT_DIR/restore.txt" \
    || die "no $OUT_DIR/restore.txt with a DIR: line and 'SETUP-AFTER: HTTP 404 -': the restore did not complete; run 'run.sh restore' first"
  dir=$(sed -n 's/^DIR: //p' "$OUT_DIR/restore.txt" | head -n 1)
  dx test -f "$dir/holzcloud.sqlite" || die "the target $NAME has no $dir/holzcloud.sqlite"
  dx systemctl is-active --quiet holzcloud || die "holzcloud is not active in $NAME: the invariants are taken from the running restored site"

  CHECK_OUT="$OUT_DIR/restore-negative.txt"
  CHECK_BADS=0
  : > "$CHECK_OUT"
  key_line "DIR: $dir"
  check_info "the live data are the restored site; before and after every case the inode of /opt/holzcloud/data, the sha256 of every file below it except the SQLite family and MANIFEST, the service's InvocationID and ActiveEnterTimestamp, the number of /opt/holzcloud/data.before-restore-* directories, systemctl is-active, the /readyz status and PRAGMA integrity_check of the live database (immutable URI: the running service writes to the file, so a byte hash of it is no invariant) are taken and must be equal"

  # (a) a backup whose database is damaged.
  scenario "SCENARIO[restore-damaged]: restore.sh is run on a copy of the transferred backup whose holzcloud.sqlite is truncated to half its size (or, when the check accepts that, has 4 KB overwritten in the middle), with 'yes' on stdin on purpose, to see that it is refused before the live data is touched. The document names the script, not this directory or this damage"
  bdir=/opt/holzcloud/backups/negcase-damaged
  dx rm -rf "$bdir"
  dx cp -a "$dir" "$bdir" || die "could not copy $dir to $bdir in $NAME"
  proof=$(dx bash -c 'set -u
b=$1; orig=$2
f=$b/holzcloud.sqlite
size=$(stat -c %s "$orig/holzcloud.sqlite")
try() {
  t=$(mktemp -d)
  cp "$f" "$t/holzcloud.sqlite"
  rc=0
  HOLZCLOUD_DATA_DIR="$t" /opt/holzcloud/holzcloud check > "$t/out.txt" 2>&1 || rc=$?
  echo "check exit status $rc: $(tr "\n" " " < "$t/out.txt" | cut -c1-200)"
  rm -rf "$t"
  return "$rc"
}
cp -p "$orig/holzcloud.sqlite" "$f"
truncate -s $((size / 2)) "$f"
echo "damage: holzcloud.sqlite truncated from $size to $((size / 2)) bytes"
if ! try; then echo "DAMAGE-PROVEN truncated"; exit 0; fi
cp -p "$orig/holzcloud.sqlite" "$f"
dd if=/dev/urandom of="$f" bs=4096 count=1 seek=$((size / 2 / 4096)) conv=notrunc 2>/dev/null
echo "damage: 4 KB overwritten at offset $((size / 2 / 4096 * 4096)) of $size bytes"
if ! try; then echo "DAMAGE-PROVEN overwritten"; exit 0; fi
echo "DAMAGE-NOT-PROVEN"' _ "$bdir" "$dir" | tr -d '\r') || proof="DAMAGE-NOT-PROVEN"
  while IFS= read -r asz; do
    [ -z "$asz" ] || check_info "case a: $asz"
  done <<< "$proof"
  if printf '%s\n' "$proof" | grep -q '^DAMAGE-PROVEN '; then
    check_pass "case a: the damage is real: the binary's check, run the way restore.sh runs it on a copy, refused the damaged file ($(printf '%s\n' "$proof" | sed -n 's/^DAMAGE-PROVEN //p'))"
  else
    check_bad "case a: the damage was NOT proven real: the binary's check accepted the damaged file, so the refusal below would prove nothing"
  fi
  before=$(neg_inv)
  out=$(neg_restore "$bdir" yes)
  ec=$(printf '%s\n' "$out" | sed -n 's/^EXIT //p' | tail -n 1)
  neg_finish a restore-damaged "$before" "$out" "$ec" 'fails its integrity check' "names the integrity refusal ('the snapshot fails its integrity check')"

  # (b) a backup directory without a database.
  scenario "SCENARIO[restore-no-sqlite]: restore.sh is run on a directory that holds only a copy of MANIFEST, with 'yes' on stdin, to see that it is refused"
  ndir0=/opt/holzcloud/backups/negcase-empty
  dx rm -rf "$ndir0"
  dx mkdir "$ndir0" || die "could not create $ndir0 in $NAME"
  dx cp -p "$dir/MANIFEST" "$ndir0/MANIFEST" || die "no MANIFEST to copy from $dir"
  before=$(neg_inv)
  out=$(neg_restore "$ndir0" yes)
  ec=$(printf '%s\n' "$out" | sed -n 's/^EXIT //p' | tail -n 1)
  neg_finish b restore-no-sqlite "$before" "$out" "$ec" 'holzcloud\.sqlite not found' "names the missing holzcloud.sqlite"

  # (c) the intact backup, the answer is no.
  scenario "SCENARIO[restore-answer-no]: restore.sh is run on the intact transferred backup with 'no' on stdin, to see that it aborts before the service is stopped"
  before=$(neg_inv)
  out=$(neg_restore "$dir" no)
  ec=$(printf '%s\n' "$out" | sed -n 's/^EXIT //p' | tail -n 1)
  neg_finish c restore-answer-no "$before" "$out" "$ec" '^aborted' "says 'aborted'"

  dx rm -rf "$bdir" "$ndir0"

  # (d) backup.sh with a free-space threshold no disk has: refused before it
  # makes a directory.
  scenario "SCENARIO[backup-low-disk]: backup.sh is run as the service user with MIN_FREE_MB=99999999, a threshold no disk has, to see that it exits 1 and makes no directory. The document names the script and its default; the value is new"
  ndir1=$(dx bash -c 'ls -1d /opt/holzcloud/backups/*/ 2>/dev/null | wc -l' | tr -d '\r')
  before=$(neg_inv)
  out=$(rc=0; dx runuser -u holzcloud -- env MIN_FREE_MB=99999999 /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups 2>&1 || rc=$?; echo "EXIT $rc")
  out=$(printf '%s\n' "$out" | tr -d '\r')
  ec=$(printf '%s\n' "$out" | sed -n 's/^EXIT //p' | tail -n 1)
  neg_finish d backup-low-disk "$before" "$out" "$ec" 'only [0-9]+ MB free' "names the free space ('only N MB free ... need 99999999 MB')"
  asz=$(dx bash -c 'ls -1d /opt/holzcloud/backups/*/ 2>/dev/null | wc -l' | tr -d '\r')
  if [ "$asz" = "$ndir1" ]; then
    check_pass "case d: the number of directories under /opt/holzcloud/backups is unchanged ($asz): no directory was made"
  else
    check_bad "case d: the number of directories under /opt/holzcloud/backups changed from $ndir1 to $asz"
  fi

  leak_check
  if [ "$CHECK_BADS" -gt 0 ]; then
    key_line "RESTORE-NEGATIVE FAILED: $CHECK_BADS assertion(s) are BAD"
    return 1
  fi
  key_line "RESTORE-NEGATIVE OK"
}

# restore-negative: D-09, on the target, after `restore`, through on_target only.
cmd_restore_negative() {
  docker exec "$TGT_NAME" true >/dev/null 2>&1 \
    || die "the target container $TGT_NAME does not answer; bring it up with 'run.sh build up install' under NAME=$TGT_NAME OUT_DIR=$TGT_OUT_DIR, then 'run.sh restore'"
  on_target restore_negative_into
}

# backup-fixture: D-14 and D-02, in the source container as the service user. The
# seed site has no operator language file and no logo, so a backup of it could not
# show whether those directories are carried: one small valid file is planted in
# each, always. With BULK_MB above zero the second data point's load is written
# too: synthetic media, incompressible, outside the database (no row refers to
# it), so the time of a copy is not dominated by the start of the service.
cmd_backup_fixture() {
  local mb=${BULK_MB:-0} wid n free need bulk bytes mbytes
  case "$mb" in "" | *[!0-9]*) die "BULK_MB '$mb' is not a number of megabytes" ;; esac
  dx true >/dev/null 2>&1 \
    || die "container $NAME does not answer; run 'run.sh build up install baseline update postinstall' with INSTALL_FROM=old first"
  dx test -e /opt/holzcloud/data/holzcloud.sqlite \
    || die "no /opt/holzcloud/data/holzcloud.sqlite in $NAME; run 'run.sh install' first"
  mkdir -p "$OUT_DIR"
  deviation "DEVIATION[fixture]: a language file (data/sprachen/nl.json), a logo (data/marke/logo.svg) and, with BULK_MB above zero, synthetic media (data/media/WEBSITE/bulk, random bytes outside the database) stand in for operator work; none of them is a step of the document. They make the data directory hold every directory a backup must carry, and a load whose copy time can be read"

  # The flat JSON object of text pairs that internal/i18n Parse accepts; the file
  # name is the language tag. Written by the service user, as the admin would.
  printf '%s\n' '{"Speichern": "Opslaan", "Abbrechen": "Annuleren"}' \
    | dxi runuser -u holzcloud -- bash -c 'umask 077; mkdir -p /opt/holzcloud/data/sprachen && cat > /opt/holzcloud/data/sprachen/nl.json' \
    || die "could not plant /opt/holzcloud/data/sprachen/nl.json"
  # internal/branding LogoPath looks for logo.svg, logo.png, logo.webp in the folder.
  printf '%s\n' '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" fill="#325737"/></svg>' \
    | dxi runuser -u holzcloud -- bash -c 'umask 077; mkdir -p /opt/holzcloud/data/marke && cat > /opt/holzcloud/data/marke/logo.svg' \
    || die "could not plant /opt/holzcloud/data/marke/logo.svg"
  say "planted data/sprachen/nl.json and data/marke/logo.svg"

  if [ "$mb" -gt 0 ]; then
    n=$((mb / 5))
    [ "$n" -ge 1 ] || die "BULK_MB $mb is less than one 5 MiB file"
    free=$(dx df -Pm /opt/holzcloud/data | awk 'NR == 2 { print $4 }' | tr -d '\r') || free=0
    need=$((mb * 4 + 1024))
    if [ "${free:-0}" -lt "$need" ]; then
      die "BULK_MB=$mb needs $need MB free (the files, their copy in a backup and in a restore, and the 1024 MB MIN_FREE_MB that backup.sh refuses to start below), $NAME has $free MB"
    fi
    wid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite "select id from websites order by id limit 1" | tr -d '\r') \
      || die "backup-fixture: could not read the website id"
    [ -n "$wid" ] || die "backup-fixture: there is no website to attach media to"
    bulk=/opt/holzcloud/data/media/$wid/bulk
    dx runuser -u holzcloud -- bash -c 'umask 077; d=$1; n=$2
rm -rf "$d"; mkdir -p "$d"
i=1
while [ "$i" -le "$n" ]; do
  head -c 5242880 /dev/urandom > "$d/bulk-$i.bin"
  i=$((i + 1))
done' _ "$bulk" "$n" || die "could not write the bulk media below $bulk"
    bytes=$(dx du -sb "$bulk" | cut -f1 | tr -d '\r')
    mbytes=$(dx du -sb /opt/holzcloud/data/media | cut -f1 | tr -d '\r')
    {
      echo "BULK-MB: $mb"
      echo "FILES: $(dx bash -c 'find "$1" -type f | wc -l' _ "$bulk" | tr -d '\r')"
      echo "BYTES: $bytes"
      echo "MEDIA-BYTES: $mbytes"
    } > "$OUT_DIR/bulk.txt"
    cat "$OUT_DIR/bulk.txt"
  fi
  say "BACKUP-FIXTURE OK"
}

trap on_exit EXIT

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    exec | logs | compare)
      "cmd_$sub" "$@"
      exit $?
      ;;
    build | up | start | wait | canary | down | smoke | seed | check | verdict | mdwe | score | all | bins | install | firstrun | caddy | postinstall | baseline | update | negative | rollback | fresh | lifecycle)
      "cmd_$sub"
      ;;
    mdwe-control)
      cmd_mdwe_control
      ;;
    backup | transfer | restore)
      # A BAD line makes the subcommand return 1 after it has written every file
      # it owns. The chain `backup transfer restore` goes on, so that one run
      # shows every finding of all three, and ends with 1 (die still ends it at
      # once: a missing precondition is no finding).
      "cmd_$sub" || PHASE22_RC=1
      ;;
    backup-timer)
      cmd_backup_timer || PHASE22_RC=1
      ;;
    restore-negative)
      cmd_restore_negative || PHASE22_RC=1
      ;;
    backup-fixture)
      cmd_backup_fixture || PHASE22_RC=1
      ;;
    *) usage ;;
  esac
done
exit "${PHASE22_RC:-0}"
