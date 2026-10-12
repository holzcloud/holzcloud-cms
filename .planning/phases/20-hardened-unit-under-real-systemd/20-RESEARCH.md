# Phase 20: Hardened unit under real systemd - Research

**Researched:** 2026-10-12
**Domain:** systemd sandboxing (`deploy/holzcloud.service`) tested in a Docker systemd container; wazero (wasm plugin runtime) vs `MemoryDenyWriteExecute=`
**Confidence:** HIGH for what was measured on this Mac (Docker Desktop 4.94, Rosetta amd64 and native arm64 runs, all executed in this session); MEDIUM for transfer to a real amd64 server (see Assumptions Log)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

### Container architecture
- **D-01:** Measure under **linux/amd64 via emulation** only (target arch of the project). No arm64 counter-run. The emulation caveat (JIT/mprotect/seccomp behaviour may differ from real hardware) MUST be named in the findings, together with other container-vs-server differences.

### Container recipe & hand-over
- **D-02:** Recipe lives in **`deploy/test/`** as a `Dockerfile` plus a `run.sh`. Systemd-capable base image (Debian-style), binary and unit copied in, run with `--privileged --cgroupns=host` (or the minimum that works). `run.sh` builds, starts, waits for systemd, runs the checks. Phases 21 and 22 reuse it with different steps, so keep build/start/exec separable.

### MDWE vs wazero
- **D-03:** **Measure both variants, then decide.** First run the unit unchanged (`MemoryDenyWriteExecute=true`), record the plugin-load result; on failure measure with `MemoryDenyWriteExecute=false`. Adjust the unit only if the evidence forces it. Switching wazero to the interpreter engine is a code change and only a last resort. No pre-emptive removal. Evidence = every shipped plugin (`plugins/bestellung`, `jahreszahl`, `kontaktformular`, `nicht-gefunden`, `suche`) loads and is invoked under the unit.

### Security score
- **D-03a:** Commit the **raw `systemd-analyze security` output** (full table + overall value) to the repo in `deploy/SECURITY-SCORE.md` (or `docs/`), with a section "Container vs. server" naming directives the container does not enforce. **No target value** is imposed; the score is measured, not tuned.

### Claude's Discretion
- Base image choice, exact Docker flags, how readiness is polled, how plugin loading is exercised, file naming of the score document.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UNIT-01 | The unit `deploy/holzcloud.service` runs under real systemd, the service starts and answers HTTP. | Recipe works: Debian systemd container, `--privileged`, poll `/healthz` inside the container. Under Rosetta amd64 the **unchanged unit crashes at start** (`ProcSubset=pid`); a named container-only drop-in is required (Pitfall 1). Native arm64 runs the unit unchanged. |
| UNIT-02 | All plugins (wazero) load under the hardened unit. The `MemoryDenyWriteExecute=true` suspicion is decided with evidence, unit adjusted if needed. | All five plugins load **and are invoked** under enforced MDWE (arm64 native). wazero falls back silently to its interpreter; the unit needs **no change**. But the amd64/Rosetta run does **not enforce MDWE at all**, so D-01 alone cannot produce this evidence (Summary, Open Question 1). |
| UNIT-03 | The value of `systemd-analyze security` is measured and recorded in the repo. | Measured `1.3 OK` (online, loaded unit) on systemd 252 and 257; `1.4` offline on 252. Exact table captured below; commands and the online/offline difference documented. |
</phase_requirements>

## Summary

The recipe is feasible and was prototyped end to end in this session (build in 48 s, boot ~8 s, plugin load 1-1.6 s per plugin under emulation). Three findings change how the phase must be planned, and one of them collides with a locked decision.

**Finding 1 - the amd64 emulation on this host cannot decide the MDWE question.** The host runs Docker Desktop with an arm64 Linux VM; the amd64 container runs under Rosetta (the crash text `VMAllocationTracker.cpp` is Rosetta's; see A6). Inside that container `/proc/<pid>/status` of the service shows `Seccomp: 0`, and a probe program under `-p MemoryDenyWriteExecute=yes` was allowed `mmap RWX` and `mprotect RW->RX`. Every seccomp-based directive of the unit (`MemoryDenyWriteExecute`, `SystemCallFilter`, `RestrictAddressFamilies`, `LockPersonality`) is **silently not enforced**, while mount-namespace directives (`ProtectSystem=strict`, `PrivateDevices`) are enforced. A "plugins load under MDWE" result from this run would be a false pass. D-01 says "no arm64 counter-run" and "name the emulation caveat"; the caveat turns out to be total for this question. The same recipe on a native arm64 container (same host, `PLATFORM=linux/arm64`) enforces everything (`Seccomp: 2`, 24 filters). **Recommendation: keep D-01 for UNIT-01/UNIT-03, and add one native-arm64 run restricted to the MDWE/seccomp evidence; ask the user to confirm this narrow deviation** (Open Question 1). If the user refuses, UNIT-02's MDWE half cannot honestly be called settled; the doc must then say "not enforced in the test environment".

**Finding 2 - MDWE does not break wazero; it silently downgrades it.** The project builds its runtime with `wazero.NewRuntimeConfig()` (auto engine). wazero v1.12.0 probes the compiler by mmap(RW) + mprotect(RX) and, if that fails, selects the interpreter. Under enforced MDWE the probe fails, so all five plugins load on the **interpreter**: measured `plugin loaded` x5, `list_plugins` reports `running: true`, `last_error: ""`, every plugin invoked successfully, no anonymous `r-xp` mappings in the process. With `MemoryDenyWriteExecute=no` the compiler is used (anonymous `r-xp` mappings present). Cost of staying with MDWE=true: hook calls ~1-3x slower (median 1.3-11 ms, far below `CallTimeout = 2 * time.Second`), plugin load ~6x faster. **The unit stays unchanged**; add a comment to it and record the evidence (D-03: unit unchanged is the "first variant", it passes, so the `=false` variant is measured only as a control).

**Finding 3 - the unchanged unit does not start under Rosetta because of `ProcSubset=pid`.** Rosetta opens `/proc/sys/vm/mmap_min_addr` at process start; `ProcSubset=pid` hides it; the process dies with SIGTRAP (`status=5/TRAP`). This is an emulation artefact (native arm64 runs the unit unchanged with `ProcSubset=pid`). The recipe applies a container-only drop-in `ProcSubset=all` for emulated runs and prints it as a named deviation. Do **not** weaken the shipped unit for this.

**Primary recommendation:** Build `deploy/test/{Dockerfile,Dockerfile.dockerignore,run.sh,canary/}` with `PLATFORM` (default `linux/amd64`), start with `--privileged`, always run an **enforcement canary** first and print what the container does not enforce, seed the site and plugins through the MCP endpoint `/ai` with an admin key, and gate the MDWE verdict on the canary reporting "enforced".

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Build amd64 binary | Docker BuildKit native builder (cross-compile) | - | Pure Go, `CGO_ENABLED=0`; `--platform=$BUILDPLATFORM` stage avoids emulated compile (48 s total measured) |
| Run systemd as PID 1 | Container (Debian + systemd) | Docker host kernel | Sandbox directives are executed by the *host kernel* via systemd; the container only supplies the manager |
| Sandbox enforcement (seccomp, mount ns) | Host kernel / emulation layer | systemd | Decides what the test can prove; differs between Rosetta and native (core finding) |
| Plugin runtime (wasm) | Service process (wazero) | - | Engine selected at `NewRuntime` by a runtime probe |
| Seeding site + plugins | Service HTTP API (`/ai` MCP) | `holzcloud ai key create` CLI | Only headless route; admin UI needs TOTP + fresh login |
| Score measurement | `systemd-analyze` in the container | - | Static analysis of unit properties; independent of kernel, dependent on systemd version |
| Evidence archive | Repo (`deploy/SECURITY-SCORE.md`, `deploy/test/README.md`) | - | D-03a, hand-over to phases 21/22 |

## Standard Stack

### Core
| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| Docker Desktop / Engine | 29.8.2 (client+server), cgroup v2 | run systemd container | already on host [VERIFIED: `docker version`, `docker info` this session] |
| Base image `debian:bookworm-slim` | systemd 252.39-1~deb12u2 | systemd PID 1 | worked, DEPLOY.md targets Debian/Ubuntu [VERIFIED: ran] |
| Alt base `debian:trixie-slim` | systemd 257.13-1~deb13u1 | re-measure score on newer systemd | worked; score identical (1.3) [VERIFIED: ran] |
| `golang:1.26.8` (build stage) | 1.26.8 | cross-compile holzcloud | same as root `Dockerfile` `FROM golang:1.26.8 AS build` [VERIFIED: Dockerfile read] |
| systemd, systemd-sysv, dbus | distro | PID 1 | apt, Debian archive |
| curl, jq, procps | distro | HTTP checks, JSON-RPC bodies, `/proc` inspection | apt |
| sqlite3 (optional) | distro | read `plugin_store` / `plugins` rows as evidence | apt |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `deploy/test/canary/` (Go, stdlib only, own `go.mod`) | go 1.24 like `plugins/*/go.mod` | one static probe: mmap RW, mprotect RW->RX, mmap RWX, AF_NETLINK socket, stat `/proc/sys`, write `/etc` | first step of every run, under the unit's properties via `systemd-run` |
| `systemd-analyze security` | in container | UNIT-03 | `holzcloud.service` online + `--offline=true` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Multi-stage Dockerfile with root context | Host `go build` then tiny context | Host build needs Go on the host; multi-stage is self-contained and pins 1.26.8. The root `.dockerignore` excludes `deploy` and `*.md`, so use `deploy/test/Dockerfile.dockerignore` (verified to take precedence, below). |
| `--privileged` | `--cap-add SYS_ADMIN --security-opt seccomp=unconfined --security-opt apparmor=unconfined --cgroupns=private -v /sys/fs/cgroup:/sys/fs/cgroup:rw` | Both started systemd and the service on this host [VERIFIED: ran]. Use `--privileged --tmpfs /run --tmpfs /run/lock` (simplest, verified on cgroup v2 with private cgroupns and no bind mount); mention the minimal variant in the README. |
| Python probe | Go canary | Go gives one static binary per arch, no python in image |
| Rosetta amd64 | arm64 native | See Finding 1; amd64 enforcement only on a native amd64 kernel (Open Question 2) |

**Installation (inside the image build):**
```bash
apt-get update && apt-get install -y --no-install-recommends systemd systemd-sysv dbus curl ca-certificates jq procps sqlite3
```

**Version verification:** no language-ecosystem packages are added to the repo; the image installs Debian archive packages only. The Go version is the one already pinned by `go.mod` (`go 1.26.8`) and the root `Dockerfile`.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none from npm/PyPI/crates) | - | - | - | - | n/a | The phase adds no Go module dependency (canary uses stdlib `syscall`); apt packages come from the Debian archive of the base image |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
The `gsd-tools package-legitimacy` seam targets npm/pypi/crates and was not applicable.

## Architecture Patterns

### System Architecture Diagram

```
 operator laptop (arm64 Mac, Docker Desktop, Rosetta)
        |
        |  run.sh build  --PLATFORM=linux/amd64-->  docker build  (BuildKit native builder)
        |                                             |-- stage build: golang:1.26.8, GOARCH=amd64 cross-compile -> /holzcloud
        |                                             '-- stage final: debian + systemd, COPY binary, unit, plugin zips, canary
        v
 run.sh up   --docker run --privileged-->  container (PID 1 = systemd)
        |                                    |
        |   [emulated only] drop-in ProcSubset=all  (named deviation, container only)
        |                                    |
        |   systemctl start holzcloud  ---->  holzcloud.service (hardened unit, unchanged)
        |                                       |-- Go server :8080 (127.0.0.1 only; no port publish!)
        |                                       |-- SQLite /opt/holzcloud/data
        |                                       '-- wazero engine: compiler OR interpreter (probe)
        v
 run.sh wait  poll `systemctl is-system-running`, then `curl /healthz` inside container
        |
 run.sh canary   systemd-run -p <unit sandbox props> hc-canary   -> "ENFORCED / NOT ENFORCED" table
        |
 run.sh seed     holzcloud ai key create (as holzcloud) -> /ai JSON-RPC: create_website, add_domain,
        |        install_plugin x5, set_plugin_websites, enable_plugin, create_page, publish_page
        |
 run.sh check    restart unit -> journal `plugin loaded` x5 -> list_plugins running:true ->
        |        invoke each plugin over HTTP (Host: test.local) -> /proc/PID/maps engine probe ->
        |        NRestarts=0, no SIGSYS
        |
 run.sh score    systemd-analyze security holzcloud.service  (+ --offline=true)  -> deploy/SECURITY-SCORE.md
        |
 run.sh exec -- <cmd>   /  run.sh down      (phases 21, 22 plug in here)
```

### Recommended Project Structure
```
deploy/
  holzcloud.service            # unchanged except a comment about wazero fallback (optional)
  SECURITY-SCORE.md            # D-03a: raw output, versions, container-vs-server table, MDWE evidence
  test/
    Dockerfile                 # multi-stage, ARG GO_VERSION, BASE_IMAGE
    Dockerfile.dockerignore    # overrides root .dockerignore (which excludes deploy and *.md)
    run.sh                     # subcommands: build up wait canary seed check score exec down all
    README.md                  # hand-over for phases 21/22: flags, env vars, deviations, pitfalls
    canary/{main.go,go.mod}    # own module: root `go vet ./...` / `go mod tidy` are unaffected
```
`canary/` as its own module mirrors `plugins/*/go.mod`; `gofmt -l .` in CI still covers it, so format it. [VERIFIED: ci.yml read; plugins/jahreszahl/go.mod read]

### Pattern 1: Dockerfile (prototype built and run in this session)
```dockerfile
# Source: prototype, built with `docker build --platform linux/amd64 -f ... .` from repo root (48 s)
ARG GO_VERSION=1.26.8
ARG BASE_IMAGE=debian:bookworm-slim
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /holzcloud ./cmd/holzcloud

FROM ${BASE_IMAGE}
ENV container=docker DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus curl ca-certificates jq procps sqlite3 \
 && rm -rf /var/lib/apt/lists/* \
 && systemctl mask systemd-modules-load.service   # trixie: fails in a container -> "degraded"
RUN useradd -r -s /usr/sbin/nologin holzcloud \
 && mkdir -p /opt/holzcloud/data && chown holzcloud:holzcloud /opt/holzcloud/data
COPY --from=build /holzcloud /opt/holzcloud/holzcloud
COPY deploy/holzcloud.service /etc/systemd/system/holzcloud.service
COPY plugins/ /opt/holzcloud/plugins-src/
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
```
Notes: `useradd` line is exactly DEPLOY.md's (`sudo useradd -r -s /usr/sbin/nologin holzcloud`, DEPLOY.md:78). The service is deliberately **not** enabled in the image; `run.sh` starts it so the first journal lines are captured. Copy only `plugins/*/*.zip` and the canary if image size matters (plugins/ is 24 MB).

`Dockerfile.dockerignore` (precedence over the root `.dockerignore` is documented: "A Dockerfile-specific ignore-file takes precedence over the `.dockerignore` file at the root of the build context if both exist." [CITED: docs.docker.com/build/concepts/context/]; verified by build):
```
.git
.planning
.claude
data
sites
docs
*.md
!README.md
!CHANGELOG.md
```
`*.md` plus `!CHANGELOG.md` is kept because `changelog.go` embeds `CHANGELOG.md` (root `.dockerignore` comment, read).

### Pattern 2: Run flags and platform handling
```bash
docker build --platform "$PLATFORM" -f deploy/test/Dockerfile -t "$IMAGE" .
docker run -d --name "$NAME" --platform "$PLATFORM" --privileged \
  --tmpfs /run --tmpfs /run/lock "$IMAGE"
```
`--platform` must be given to **both** build and run. A local image built from `FROM --platform=linux/amd64 ...` and started with `docker run --platform linux/amd64` failed with `pull access denied`; building with the CLI `--platform` flag and running with the same flag worked [VERIFIED: ran]. Running without `--platform` on an amd64 image only warns.

### Pattern 3: Enforcement canary (the guard against false passes)
Run first in every run, under the unit's own sandbox properties, as the `holzcloud` user. `PrivateTmp=true` hides `/tmp` from the transient unit, so install the binary at `/usr/local/bin/hc-canary` (exec failure `status=203` otherwise, observed).
```bash
systemd-run --wait --pipe -q -p MemoryDenyWriteExecute=yes -p User=holzcloud \
  -p NoNewPrivileges=true -p ProtectSystem=strict -p ReadWritePaths=/opt/holzcloud/data \
  -p PrivateTmp=true -p PrivateDevices=true -p ProtectProc=invisible -p ProcSubset=pid \
  -p RestrictNamespaces=true -p 'RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX' \
  -p LockPersonality=true -p CapabilityBoundingSet= \
  -p SystemCallFilter=@system-service -p 'SystemCallFilter=~@privileged @resources' \
  /usr/local/bin/hc-canary
```
Measured output (native arm64, enforced):
```
mmap RW                ALLOWED
mprotect RW->RX        denied (operation not permitted)
mmap RWX               denied (operation not permitted)
socket AF_NETLINK      denied (address family not supported by protocol)
socket AF_INET         ALLOWED
stat /proc/sys         denied (no such file or directory)
stat /dev/kmsg         denied (no such file or directory)
write /etc             denied (read-only file system)
```
Measured under Rosetta amd64 (with `ProcSubset=all`, because `pid` crashes it):
```
mmap RW ALLOWED / mprotect RW->RX ALLOWED / mmap RWX ALLOWED / socket AF_NETLINK ALLOWED
stat /proc/sys ALLOWED / stat /dev/kmsg denied / write /etc denied (read-only file system)
```
Interpretation rule for `run.sh`: if `mprotect RW->RX` or `mmap RWX` is ALLOWED while MDWE=yes, print `SECCOMP NOT ENFORCED - MDWE verdict is void on this platform` and exit non-zero for the MDWE step (not for UNIT-01).

### Pattern 4: Seeding without a browser (verified)
Admin UI needs TOTP enrolment and a fresh login; the MCP endpoint takes a bearer key.
```bash
# as the service user, same env as the unit (UMask 0077, files owned by holzcloud)
runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data \
  /opt/holzcloud/holzcloud ai key create -name t -level admin   # key is the line starting hc_
```
Then JSON-RPC `tools/call` against `POST http://127.0.0.1:8080/ai` with `Authorization: Bearer <key>`: `create_website {"name":"Test"}`, `add_domain {"website":1,"domain":"test.local","primary":true}`, `install_plugin {"archive_base64":...}`, `set_plugin_websites {"id":...,"websites":[1]}` (after install, else FOREIGN KEY error), `enable_plugin {"id":...}`, `create_page {website,title,slug,markdown}`, `publish_page {"id":N,"status":"published"}`. Tool names and the `/ai` mount were exercised against the running unit [VERIFIED: ran]; `holzcloud ai key create` flags `-name`, `-level` read from `cmd/holzcloud/cli_ai.go:39-45`.
```bash
# zips are ~1 MB each: never pass them through `jq --arg` (ARG_MAX: "Argument list too long")
base64 -w0 /tmp/suche.zip | jq -Rs '{jsonrpc:"2.0",id:1,method:"tools/call",
  params:{name:"install_plugin",arguments:{archive_base64:.}}}' \
  | curl -s -X POST http://127.0.0.1:8080/ai -H "Authorization: Bearer $KEY" \
      -H 'Content-Type: application/json' --data-binary @-
```
Media (exercise of the image/thumbnail path under the syscall filter): `create_upload_link` returns an `https://` URL even over plain HTTP; rewrite to `http://` and `curl --upload-file x.png`. A 1600x1000 PNG produced `-thumb.jpg` and `-medium.jpg` under the hardened unit, `NRestarts=0` [VERIFIED: ran].

### Pattern 5: Evidence that a plugin loaded **and** ran, and which engine
| Plugin | Invocation (Host: test.local) | Observed under enforced MDWE (arm64) |
|--------|-------------------------------|--------------------------------------|
| jahreszahl | page with `[[jahr]]` | rendered `<p>Jahr: 2026</p>`; `plugin_store` row `jahreszahl\|1\|ersetzungen\|1` |
| kontaktformular | page with `[[formular]]` | `<form class="contact-form" method="POST" action="/formular">`; GET `/formular` -> 303 |
| bestellung | page with `[[bestellung]]` | `<div class="bestellung"><p>Im Moment gibt es nichts zu bestellen.</p></div>`; GET `/bestellung` -> 303 |
| suche | `/suche?q=Plugintest` | result link `href="/plugintest" class="search-result__title"` |
| nicht-gefunden | any 404 | `plugin_store` rows `nicht-gefunden\|1\|pfad:/gibts-nicht\|{"pfad":"/gibts-nicht","anzahl":21,...}` |

Server-side proof: journal line `"msg":"plugin loaded","plugin":"<id>"` for all five (manager.go:126 `m.log.Info("plugin loaded", "plugin", p.ID, "version", p.Version)`; failure is manager.go:142 `m.log.Error("plugin not loaded", "plugin", id, "err", err)`), and MCP `list_plugins` -> `"running": true`, `"last_error": ""`.

Engine discriminator (no code change needed):
```bash
P=$(systemctl show -p MainPID --value holzcloud)
awk '$2 ~ /x/ && $6 == ""' /proc/$P/maps     # anonymous executable mappings
```
Empty under MDWE=yes (interpreter); several `r-xp` anonymous regions under MDWE=no (compiler). The Go binary's own text is file-backed, so the filter is clean. [VERIFIED: ran, both modes]

### Anti-Patterns to Avoid
- **Publishing port 8080 to test the site.** The unit sets no `HOLZCLOUD_LISTEN`, so the service binds `127.0.0.1` (journal: `"addr":"127.0.0.1:8080"`); a published port reaches nothing. Use `docker exec ... curl http://127.0.0.1:8080/...`. Do not "fix" it by editing the unit; the root `Dockerfile` sets `HOLZCLOUD_LISTEN=0.0.0.0` for a different deployment shape.
- **`sleep N` for readiness.** Under Rosetta the service needs ~8-10 s (plugin compile ~1-1.6 s each, and `/healthz` answers only after plugins loaded, because `server starting` is logged after them). Poll `/healthz` with a timeout of 120 s.
- **Reading `systemctl is-system-running` as pass/fail.** Trixie reports `degraded` because `systemd-modules-load.service` fails in a container; mask it in the image or accept `running|degraded`.
- **Weakening the shipped unit to fit the emulator.** Put `ProcSubset=all` in `/etc/systemd/system/holzcloud.service.d/container.conf` inside the container only, and print it as DEVIATION.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Install/enable plugins headlessly | a browser/CSRF/TOTP login script, or hand-written `INSERT INTO plugins` | MCP tools `install_plugin`, `enable_plugin` over `/ai` | Same checks as the admin upload; SQL seeding bypasses `Manager.Install` (files on disk + manifest + migrations) |
| "Did wazero use the JIT?" | patching `internal/plugin/runtime.go` to log the engine | `/proc/PID/maps` anonymous `r-xp` check | No code change in a no-feature phase; also i18n/English gates stay untouched |
| Score tabulation | parsing/prettifying `systemd-analyze` | commit the raw output verbatim (D-03a) | Raw output is the evidence |
| Readiness | custom wait loop with sleeps | `curl --retry`-style loop on `/healthz` (documented in DEPLOY.md: "constant 200 while the process answers") | endpoint exists |
| systemd in Docker | own init shim | Debian `systemd` package, `STOPSIGNAL SIGRTMIN+3`, `--tmpfs /run` | standard, verified |

**Key insight:** every pitfall found here was a *silent* one (a sandbox directive not applied, an engine silently downgraded, a bind address that makes a published port useless). The recipe's value is in the explicit self-checks (canary, engine probe, `NRestarts`), not in the happy path.

## Common Pitfalls

### Pitfall 1: Unit crashes at start under Rosetta (`ProcSubset=pid`)
**What goes wrong:** `ExecStart` dies with `status=5/TRAP`; journal: `assertion failed [!result.is_error]: Unable to open /proc/sys/vm/mmap_min_addr (VMAllocationTracker.cpp:318 init)`. `Restart=on-failure` loops 5 times in 300 s (`StartLimitBurst=5`), then the unit is `failed`.
**Why:** Rosetta reads `/proc/sys/...`; `ProcSubset=pid` hides all non-process files [CITED: systemd.exec(5) ProcSubset= "all files and directories not directly associated with process management and introspection are made invisible"]. Not a defect of the unit: native arm64 runs it unchanged.
**How to avoid:** emulated runs only: drop-in `[Service]\nProcSubset=all`, then `systemctl daemon-reload && systemctl reset-failed holzcloud && systemctl restart holzcloud`. Record it as a deviation. Always try the unchanged unit first so the crash text is captured as evidence.
**Warning signs:** `Active: activating (auto-restart) (Result: signal)`, `code=killed, signal=TRAP`.

### Pitfall 2: A passing plugin test that proves nothing
**What goes wrong:** Under Rosetta all plugins load with MDWE=true because the filter is not there (compiler used, `r-xp` mappings present).
**How to avoid:** canary first; MDWE verdict requires `mprotect RW->RX denied` under MDWE=yes; additionally record the engine via maps. Without enforcement, write "not enforced" not "settled".

### Pitfall 3: Misreading wazero's fallback as failure (or as success of the compiler)
Under enforced MDWE the compiler probe `executableMmapSupported()` fails and wazero picks the interpreter without a log line. "All plugins loaded" is therefore true **and** says nothing about speed. Record the engine and the latency (arm64 native, median of 10, seconds): `/` 0.0024 (compiler) vs 0.0040 (interpreter); `/suche?q=` 0.0035 vs 0.0113; `/formular` 0.0004 vs 0.0014; `/bestellung` 0.0003 vs 0.0013; plugin load ~1 s each (compiler) vs ~0.15-0.2 s (interpreter). `CallTimeout` is `2 * time.Second` (runtime.go), so there is a wide margin. A future switch to `wazero.NewRuntimeConfigCompiler()` would make the engine mandatory and plugins **would** fail under MDWE; that is the tripwire to document in the unit comment.

### Pitfall 4: `/tmp` is private for the transient test unit
`PrivateTmp=true` makes a binary in `/tmp` invisible to `systemd-run` units (`status=203/EXEC`). Install helpers under `/usr/local/bin`.

### Pitfall 5: ARG_MAX with base64 plugin archives
`jq --arg b "$(base64 ...)"` fails with `Argument list too long` for ~1 MB zips and the MCP call then reports `the file is empty`. Pipe via stdin (`jq -Rs`).

### Pitfall 6: Online vs offline score differ
`systemd-analyze security holzcloud.service` (manager's loaded unit) gave `1.3 OK` on 252; `--offline=true /etc/systemd/system/holzcloud.service` gave `1.4` on 252 (extra finding `PrivateMounts=` 0.2) and `1.3` on 257. A drop-in changes the online value only if it touches scored properties (`ProcSubset=all` did not change the rounded 1.3, but adds a `0.1` line). Record: systemd version, online/offline, with or without drop-in.

### Pitfall 7: Seccomp violations are invisible in a container
Real servers log SIGSYS kills to the audit log; the container has no auditd. The substitutes are `systemctl show holzcloud -p NRestarts -p Result` (expect `0`, `success`/`exit-code` only on stop) and `journalctl -u holzcloud | grep -E 'code=killed|status=31|SIGSYS'`. Also exercise more than `/healthz` (page render, search, media upload + thumbnails were run).

### Pitfall 8: Restart limit hides the second failure
`StartLimitIntervalSec=300`, `StartLimitBurst=5` (unit lines 8-9 of the `[Unit]` block). Between experiments run `systemctl reset-failed holzcloud`, or the next `start` is refused for the wrong reason.

### Pitfall 9: Files created as root
Run `holzcloud ai key create` and other CLI subcommands through `runuser -u holzcloud -- env HOLZCLOUD_DATA_DIR=/opt/holzcloud/data ...`. Without the env var the CLI opens `/tmp/data/holzcloud.sqlite` (observed: `unable to open database file (14)`); as root it would create root-owned WAL files in a directory the unit's `UMask=0077` expects to own.

## Container vs. server (differences to name in `deploy/SECURITY-SCORE.md`)

| Aspect | Measured here | Real server (assumption unless noted) |
|--------|---------------|----------------------------------------|
| Arch/emulation | amd64 via Rosetta on arm64 VM (kernel `7.0.14-linuxkit`, `uname -m` x86_64 in container) [VERIFIED] | native amd64 kernel |
| seccomp-based directives (`MemoryDenyWriteExecute`, `SystemCallFilter`, `RestrictAddressFamilies`, `LockPersonality`, ...) | **not enforced under Rosetta** (`Seccomp: 0`, canary ALLOWED) [VERIFIED]; enforced on native arm64 container (`Seccomp: 2`, `Seccomp_filters: 24`) [VERIFIED] | enforced [ASSUMED] |
| Mount-namespace directives (`ProtectSystem=strict`, `PrivateDevices`, `PrivateTmp`, `ProcSubset`) | enforced in both [VERIFIED] | enforced |
| `ProcSubset=pid` | breaks Rosetta; fine natively on arm64 [VERIFIED]; native amd64 not run [ASSUMED fine] | applies |
| systemd version | 252 (bookworm) and 257 (trixie) measured; server may run 255 (Ubuntu 24.04) or other [ASSUMED] | per distro |
| cgroups | v2 via Docker Desktop, privileged container, `systemd-modules-load` fails | host's own |
| Network | loopback only inside container; `IPAddressDeny`, `PrivateNetwork` score lines unaffected | - |
| Audit log of seccomp kills | none | present (`auditd`/journal) |
| Score | static analysis: identical value on a server with the same systemd version | same |
| CPU/timing | Rosetta: plugin load 1-1.6 s each, startup 8-10 s | faster; numbers here are not benchmarks |
| QEMU instead of Rosetta (Docker Desktop setting) | not tested [ASSUMED different again] | - |

## Measured score (raw, for D-03a; commit the full output, this is the shape)

`systemd-analyze security holzcloud --no-pager`, debian:bookworm-slim, systemd 252.39-1~deb12u2, unit unchanged (no drop-in), native arm64 container:
```
✗ RemoveIPC=                        Service user may leave SysV IPC objects around                 0.1
✗ RootDirectory=/RootImage=         Service runs within the host's root directory                  0.1
✗ RestrictAddressFamilies=~AF_UNIX  Service may allocate local sockets                             0.1
✗ RestrictAddressFamilies=~AF_(INET|INET6)  Service may allocate Internet sockets                0.3
✗ PrivateNetwork=                   Service has access to the host's network                       0.5
✗ PrivateUsers=                     Service has access to other users                              0.2
✗ DeviceAllow=                      Service has a device ACL with some special devices: char-rtc:r 0.1
✗ IPAddressDeny=                    Service does not define an IP address allow list               0.2
→ Overall exposure level for holzcloud.service: 1.3 OK :-)
```
Same eight findings and `1.3 OK` on trixie (257.13). Offline on 252: `1.4` (adds `PrivateMounts=` 0.2); offline on 257: `1.3`. With MDWE: `✓ MemoryDenyWriteExecute= Service cannot create writable executable memory mappings`. The planner must have the task capture the **complete** table (the ✓ lines too) per D-03a, not this excerpt. Unmeasured: Ubuntu 24.04 (systemd 255), Debian 12 on real hardware.

## Code Examples

### Poll readiness (no sleeps)
```bash
wait_ready() {  # $1 = container
  local i
  for i in $(seq 1 120); do
    docker exec "$1" curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1 && return 0
    sleep 1
  done
  docker exec "$1" journalctl -u holzcloud --no-pager -n 40 >&2
  return 1
}
```
`/healthz` returned `{"status":"ok"}` and `/readyz` returned `{"status":"ok","version":"dev","commit":"unknown","uptime_s":9,"integrity":"ok",...}` [VERIFIED: ran]; DEPLOY.md describes both (DEPLOY.md "Health checks").

### Emulation drop-in (named deviation)
```bash
docker exec "$NAME" bash -c 'mkdir -p /etc/systemd/system/holzcloud.service.d &&
  printf "[Service]\nProcSubset=all\n" > /etc/systemd/system/holzcloud.service.d/container.conf &&
  systemctl daemon-reload && systemctl reset-failed holzcloud; systemctl restart holzcloud'
echo "DEVIATION: ProcSubset=all applied (Rosetta needs /proc/sys); unit file itself unchanged" >&2
```

### MDWE control run (D-03 "measure both")
Variant A (primary): unit unchanged. Variant B (control): drop-in `[Service]\nMemoryDenyWriteExecute=no`, restart, record `plugin loaded` x5 and the maps probe, remove the drop-in, `daemon-reload`, restart. Both variants belong in `deploy/SECURITY-SCORE.md` next to the canary output so the reader sees that enforcement was real.

## State of the Art

| Old Approach | Current Approach | Changed | Impact |
|--------------|------------------|---------|--------|
| 9c4 summary: "`MemoryDenyWriteExecute=true` with wazero's default compiler engine (needs mprotect to executable) ... first suspect" | wazero v1.12.0 auto engine probes and falls back to the interpreter | measured 2026-10-12 | the suspicion is refuted as a *failure*, confirmed as a *silent downgrade* |
| systemd MDWE | implemented as a system-call filter; a kernel `PR_SET_MDWE` check is the preferred method where available [CITED: systemd.exec(5)] | - | both block mmap W+X, mprotect to X, `pkey_mprotect`, `shmat` X |

**Deprecated/outdated:** none relevant.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The host's amd64 emulation is Rosetta (inferred from the assertion text `VMAllocationTracker.cpp` and the Docker Desktop version; settings not read) | Summary, Container vs. server | If QEMU, behaviour of seccomp/`/proc` differs; recipe's canary still detects it |
| A2 | A native amd64 kernel enforces MDWE identically to the measured arm64 container, and wazero's probe (`mmap_unix.go` `Mprotect(... PROT_READ\|PROT_EXEC)`, arch independent) behaves the same | Summary | If not, the "unit unchanged" decision rests on arm64 only. Mitigation: run the recipe once on a native amd64 Docker host (e.g. a GitHub Actions `ubuntu-latest` job) |
| A3 | `ProcSubset=pid` is harmless for the Go binary on native amd64 (verified only native arm64) | Container vs. server | Service would fail to start on a real server; the amd64 run under Rosetta cannot show it |
| A4 | Ubuntu 24.04 / systemd 255 yields the same `1.3` | Measured score | Documented score differs from what an operator sees |
| A5 | Real servers log seccomp kills to the audit log | Pitfall 7 | minor |
| A6 | `docker exec` access and `--privileged` are acceptable on the developer machine | Pattern 2 | none expected |
| A7 | Not testing under Docker Desktop with Rosetta disabled (QEMU) is acceptable | Container vs. server | unknown behaviour class remains unnamed |

## Open Questions

1. **D-01 vs a settled MDWE answer (needs the user).**
   - What we know: Rosetta amd64 does not enforce seccomp at all; native arm64 on the same host does. All evidence for "plugins load and run under enforced MDWE" comes from arm64.
   - What's unclear: whether the user accepts a single, narrowly scoped arm64 run (`PLATFORM=linux/arm64 ./run.sh check`) as the MDWE evidence, despite "No arm64 counter-run".
   - Recommendation: yes, accept it and state in the findings that the engine probe is arch independent. If refused: UNIT-02 is documented as "MDWE not enforced in the test environment; wazero fallback demonstrated by source reading and by MDWE=no control", and a native-amd64 run is added to the backlog.
2. **Native amd64 confirmation.** Is a one-off run on a real amd64 Docker host (CI runner) wanted before 0.1.0? The recipe needs nothing but `PLATFORM=linux/amd64` there; phase 23's beta criteria could cite it. Not planned unless the user asks.
3. **Comment in the unit.** Add a 3-line comment above `MemoryDenyWriteExecute=true` ("wazero falls back to its interpreter; moving to the compiler-only engine requires `=false`")? Recommended; it changes no behaviour and `deploy_docs_test.go` does not read the unit [VERIFIED: grep shows it reads `Caddyfile.example` and `DEPLOY.md`].

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker (Desktop, cgroup v2, arm64 VM) | recipe | yes | 29.8.2, Docker Desktop 4.94.0 | none needed |
| amd64 emulation | D-01 | yes (Rosetta) | - | - |
| arm64 native container | MDWE evidence | yes | same engine | - |
| Go on host | optional canary build outside Docker | yes | go1.27.2 darwin/arm64 (`GOTOOLCHAIN=go1.26.8` auto-fetched works) | build canary in Docker build stage |
| Network to Docker Hub / Debian mirrors | image build | yes | - | cached layers |
| jq / sqlite3 on host | not needed | - | - | inside the image |
| systemd-analyze | UNIT-03 | in image | 252 / 257 | - |

**Missing dependencies with no fallback:** none.
**Missing with fallback:** none. Cleanup note: the experiment containers and `hc-exp:*` images created during this research were removed; `git status` of the repo is clean.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json` (read). The phase's checks are `run.sh` subcommands (`canary`, `check`, `score`); each exits non-zero on failure. Suggested spot checks per task: `./deploy/test/run.sh canary`, `./deploy/test/run.sh check`. No Go unit tests are added; existing gates are unaffected: no Go in `internal/`, nothing for `tools/i18n` (reads `cmd/holzcloud/templates/admin`, `internal`, `plugins/`) or `tools/english` (`plugins/`, `sdk/`) to collect (CLAUDE.md). `canary/` has its own `go.mod`, so `go vet ./...` and `go mod tidy` in CI are untouched; keep it gofmt-clean (`gofmt -l .` is a CI step).

## Security Domain

`security_enforcement` is not disabled in config, so included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (test key only) | admin key created in a throwaway container, never committed |
| V3 Session Management | no | - |
| V4 Access Control | yes (sandbox) | the hardened unit itself: `NoNewPrivileges`, empty `CapabilityBoundingSet`, `ProtectSystem=strict`, `SystemCallFilter` |
| V5 Input Validation | no | - |
| V6 Cryptography | no | - |
| V14 Configuration | yes | do not weaken the shipped unit; deviations only in a container-local drop-in; `HOLZCLOUD_SECURE=true` stays |

### Known Threat Patterns for this phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Test-only weakening leaking into the shipped unit | Elevation of privilege | drop-in only inside the container; `git diff deploy/holzcloud.service` limited to a comment |
| `--privileged` container on a developer host | Elevation | throwaway container, no host mounts except nothing (no `-v` of host paths needed), `down` removes it; document |
| Admin API key printed to logs/committed | Information disclosure | key stays in container `/tmp/key`; `run.sh` must not echo it into committed docs |
| False sense of security from unenforced seccomp | Repudiation | enforcement canary, "not enforced" printed loudly |

## Project Constraints (from CLAUDE.md)

- No new runtime fetches; this phase adds none to the product (build-time Docker pulls only affect the test image, not the binary). CSP/`tmplmgr` untouched.
- Hard stack (Go, htmx, plain CSS, SQLite via modernc) unaffected; no npm/Node: do not introduce a Node helper into `deploy/test/`.
- `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud`, `GOOS=linux GOARCH=amd64` build flags are reused in the Dockerfile.
- Anything operator-facing in Go goes through the i18n catalogue; this phase adds no operator-visible text (documents/scripts only). English prose in new docs; CLAUDE.md rule "Go files snake_case".
- GSD workflow enforcement: edits happen through `/gsd-execute-phase`. Commits as holzcloud.ch with the attribution lines given by the session (user memory: local git identity holzcloud.ch noreply).
- Questions to the user go through AskUserQuestion with options (user memory): Open Question 1 should be asked that way.
- `.dockerignore` excludes `deploy` and `*.md`: handled by `Dockerfile.dockerignore`.

## Sources

### Primary (HIGH confidence)
- Executed in this session (Docker Desktop 4.94.0, Engine 29.8.2): systemd containers on debian:bookworm-slim (252.39) and debian:trixie-slim (257.13), amd64/Rosetta and arm64 native, hardened unit with and without MDWE, canary probe, MCP seeding of five plugins, media upload.
- `/Users/cheidenreich/privat/holzcloud-cms/deploy/holzcloud.service` lines 60-97 (Read): `MemoryDenyWriteExecute=true` (72), `ProtectProc=invisible` (80), `ProcSubset=pid` (81), `SystemCallFilter=@system-service` (89), `SystemCallFilter=~@privileged @resources` (90).
- `/Users/cheidenreich/privat/holzcloud-cms/internal/plugin/runtime.go:112-118` (Read): `cfg := wazero.NewRuntimeConfig().` + `WithMemoryLimitPages(MemoryPages)` + `WithCloseOnContextDone(true)`.
- wazero v1.12.0 source, module cache `github.com/tetratelabs/wazero@v1.12.0`: `runtime.go:156-161` (`if configKind == engineKindAuto { if platform.CompilerSupports(config.enabledFeatures) { configKind = engineKindCompiler } else { configKind = engineKindInterpreter } }`), `internal/platform/platform.go:59-71` (`executableMmapSupported` = `MmapCodeSegment(1)` then `MprotectCodeSegment(seg)`), `mmap_unix.go`/`mmap_linux.go` (mmap `PROT_READ|PROT_WRITE`, mprotect `PROT_READ|PROT_EXEC`). go.mod pin `github.com/tetratelabs/wazero v1.12.0`.
- `internal/plugin/manager.go:126,142` (Read): log messages `plugin loaded` / `plugin not loaded`.
- `cmd/holzcloud/cli_ai.go:39-45`, `cmd/holzcloud/main.go:1426` (Read): key CLI flags, MCP under `/ai`.
- Root `Dockerfile`, `.dockerignore`, `.github/workflows/ci.yml`, `deploy/DEPLOY.md`, `.planning/quick/261006-9c4-*/SUMMARY` (read).

### Secondary (MEDIUM confidence)
- [systemd.exec(5) on manpages.debian.org](https://manpages.debian.org/trixie/systemd/systemd.exec.5.en.html): `ProcSubset=` and `MemoryDenyWriteExecute=` semantics (incompatible with JIT engines; implemented as a system call filter).
- [Docker build context docs](https://docs.docker.com/build/concepts/context/): Dockerfile-specific ignore file takes precedence over `.dockerignore`.

### Tertiary (LOW confidence)
- Web searches for "wazero MDWE" and "Rosetta seccomp not applied" returned no direct sources; all claims on those topics rest on the measurements above (hence A1, A2).

## Metadata

**Confidence breakdown:**
- Standard stack / recipe: HIGH - built and run in this session on the actual host.
- MDWE vs wazero behaviour: HIGH on arm64 native (measured both ways, source read); MEDIUM transfer to native amd64 (A2).
- Container-vs-server table: HIGH for the Rosetta/arm64 rows, LOW for QEMU and real-server rows (assumptions).
- Pitfalls: HIGH - each was observed.

**Research date:** 2026-10-12
**Valid until:** 2026-11-12 (Docker Desktop / Rosetta behaviour can change with an update; re-run the canary after upgrades)
