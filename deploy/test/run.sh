#!/usr/bin/env bash
# Drives the systemd test container for deploy/holzcloud.service.
#
#   deploy/test/run.sh smoke                  build, up, start, check /healthz and /readyz
#   deploy/test/run.sh build up start canary  and learn whether seccomp is enforced here
#   deploy/test/run.sh build up start         the same steps one by one
#   deploy/test/run.sh build up start seed    plugins: install, enable, a page and an image (UNIT-02)
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

# One EXIT handler for the whole script. smoke and mdwe set CLEANUP_DOWN so the
# container goes away unless KEEP=1.
CLEANUP_DOWN=0
on_exit() {
  local rc=$?
  trap - EXIT
  if [ "$CLEANUP_DOWN" = 1 ] && [ "$KEEP" != 1 ]; then
    cmd_down
  fi
  exit "$rc"
}

usage() {
  say "usage: run.sh SUBCOMMAND [SUBCOMMAND...]"
  say "  build up start wait canary down smoke   steps, may be combined: run.sh build up start"
  say "  seed                             test website, five plugins, a page and an image through /ai"
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
  # The container goes away on exit unless KEEP=1 (on_exit).
  CLEANUP_DOWN=1
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
    build | up | start | wait | canary | down | smoke | seed)
      "cmd_$sub"
      ;;
    *) usage ;;
  esac
done
