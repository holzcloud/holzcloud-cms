#!/usr/bin/env bash
# Drives the systemd test container for deploy/holzcloud.service.
#
#   deploy/test/run.sh smoke                  build, up, start, check /healthz and /readyz
#   deploy/test/run.sh build up start canary  and learn whether seccomp is enforced here
#   deploy/test/run.sh build up start         the same steps one by one
#   deploy/test/run.sh exec -- systemctl status holzcloud
#   deploy/test/run.sh down
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

say() { printf '%s\n' "$*" >&2; }
die() { say "run.sh: $*"; exit 1; }
dx() { docker exec "$NAME" "$@"; }
dxi() { docker exec -i "$NAME" "$@"; }

usage() {
  say "usage: run.sh SUBCOMMAND [SUBCOMMAND...]"
  say "  build up start wait canary down smoke   steps, may be combined: run.sh build up start"
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

cmd_smoke() {
  # The container goes away on exit unless KEEP=1.
  trap 'rc=$?; [ "$KEEP" = 1 ] || cmd_down; exit $rc' EXIT
  cmd_build
  cmd_up
  cmd_start

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
  say "UNIT-01 PASS"
}

[ "$#" -gt 0 ] || usage
while [ "$#" -gt 0 ]; do
  sub=$1
  shift
  case "$sub" in
    exec | logs)
      "cmd_$sub" "$@"
      exit $?
      ;;
    build | up | start | wait | canary | down | smoke)
      "cmd_$sub"
      ;;
    *) usage ;;
  esac
done
