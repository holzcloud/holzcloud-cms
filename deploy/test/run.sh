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
#                                              then the Updating section played as shipped (OUT_DIR/update.txt)
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
      printf '%s\n' "$line" | dxi bash -s 2>&1 | tee -a "$outf" || rc=$?
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
  say "  update                           play the document's Updating section as it stands, then capture state s1 and write update.txt"
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
insert_old_order() {
  local number=$1 wid
  case "$number" in *[!0-9A-Za-z-]* | "") die "insert_old_order: odd order number '$number'" ;; esac
  wid=$(dx runuser -u holzcloud -- sqlite3 -readonly /opt/holzcloud/data/holzcloud.sqlite \
    "select id from websites order by id limit 1") || die "insert_old_order: could not read the website id"
  [ -n "$wid" ] || die "insert_old_order: there is no website to attach the order to"
  printf '%s\n' \
    ".timeout 5000" \
    "INSERT INTO orders (website_id, number, audience, currency, email, name, company, vat_number, phone, street, postal_code, city, country, note, items_net, items_tax, items_gross, shipping_net, shipping_tax, shipping_gross, total_net, total_tax, total_gross, vat_exempt, status, payment_method, payment_status, return_policy, created_at, updated_at) VALUES ($wid, '$number', 'private', 'CHF', 'kunde@example.test', 'Test Kunde', '', '', '', 'Teststrasse 1', '8000', 'Zuerich', 'CH', '', 9250, 750, 10000, 0, 0, 0, 9250, 750, 10000, 0, 'new', 'invoice', 'open', '', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));" \
    | dxi runuser -u holzcloud -- sqlite3 /opt/holzcloud/data/holzcloud.sqlite \
    || die "insert_old_order: the INSERT failed"
}

# Runs INSIDE the container as the service user (runuser -u holzcloud -- bash -s):
# counts only, and only through a read-only connection. No row of any table is
# selected, the users, sessions and key tables included (T-21-07), and no holzcloud
# subcommand is started, because every one of them migrates silently (T-21-08).
read -r -d '' STATE_SH <<'STATE_EOF' || true
set -eu
db=/opt/holzcloud/data/holzcloud.sqlite
q() { sqlite3 -readonly "$db" ".timeout 5000" "$1"; }
echo "schema: $(q "select coalesce(max(version_id), 0) from goose_db_version where is_applied = 1")"
q "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name" | while IFS= read -r t; do
  n=$(q "select count(*) from \"$t\"" 2>/dev/null) || n="?"
  echo "table $t $n"
done
STATE_EOF

# capture_state LABEL: OUT_DIR/state-LABEL.txt as KEY: value lines. identity is
# the /readyz answer reduced to status, version, commit and integrity; schema the
# highest applied goose version; then one "table NAME N" line per table of
# sqlite_master, nothing hand-listed. First version: plan 21-03 completes it.
capture_state() {
  local label=$1 f ident
  f="$OUT_DIR/state-$label.txt"
  mkdir -p "$OUT_DIR"
  ident=$(dx curl -fsS --max-time 5 http://127.0.0.1:8080/readyz | dxi jq -c '{status,version,commit,integrity}') \
    || die "capture_state $label: /readyz did not answer"
  {
    echo "identity: $ident"
    printf '%s\n' "$STATE_SH" | dxi runuser -u holzcloud -- bash -s
  } > "$f" || die "capture_state $label: reading the database failed"
  grep -q '^table ' "$f" || die "capture_state $label: no table line in $f"
  say "state $label captured in $f ($(grep -c '^table ' "$f") tables)"
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
# (D-07). Both binaries exist; the new one is what the scp line carries.
cmd_update() {
  local o_ver o_com n_ver n_com start n0 failed mo mn pend snap sa sb prev bk applied jl
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
  {
    echo "OLD: $o_ver $o_com"
    echo "NEW: $n_ver $n_com"
    echo "MIGRATIONS: old=$mo new=$mn pending=$pend"
    echo "APPLIED: $applied"
    echo "SNAPSHOT: ${snap:-none}"
    echo "SCHEMA: ${sa:-?} -> ${sb:-?}"
    echo "PREV: $prev"
    echo "BACKUP: ${bk:-none}"
    printf '%s\n' "$jl" | while IFS= read -r l; do
      [ -z "$l" ] || echo "JOURNAL: $l"
    done
  } > "$OUT_DIR/update.txt"
  cat "$OUT_DIR/update.txt"

  leak_check
  if [ -n "$failed" ]; then
    say "run.sh: UPDATE FAILED, counted steps with a non-zero status:"
    printf '%s\n' "$failed" >&2
    exit 1
  fi
  [ "$o_ver" != "$n_ver" ] || { say "run.sh: UPDATE FAILED, /readyz reports the same version ($n_ver) before and after"; exit 1; }
  if ! [ "${sb:-0}" -gt "${sa:-0}" ] 2>/dev/null; then
    say "run.sh: UPDATE FAILED, the schema did not move forward (${sa:-?} -> ${sb:-?})"
    exit 1
  fi
  say "UPDATE OK"
}

trap on_exit EXIT

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    exec | logs)
      "cmd_$sub" "$@"
      exit $?
      ;;
    build | up | start | wait | canary | down | smoke | seed | check | verdict | mdwe | score | all | bins | install | firstrun | caddy | postinstall | baseline | update)
      "cmd_$sub"
      ;;
    mdwe-control)
      cmd_mdwe_control
      ;;
    *) usage ;;
  esac
done
