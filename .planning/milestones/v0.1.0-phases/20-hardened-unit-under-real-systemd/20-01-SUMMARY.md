---
phase: 20-hardened-unit-under-real-systemd
plan: 01
subsystem: deploy
tags: [systemd, docker, seccomp, rosetta, canary, hardening]
requires: []
provides:
  - "deploy/test/run.sh recipe (build, up, start, wait, canary, exec, logs, down, smoke) for plans 20-02/20-03 and phases 21/22"
  - "UNIT-01 evidence on linux/amd64 (Rosetta) and native arm64"
  - "Enforcement canary with a MemoryDenyWriteExecute=no control; verdict file and exit code 3 convention"
affects: [20-02, 20-03, 21, 22]
tech-stack:
  added: []
  patterns:
    - "Systemd as PID 1 of a privileged Debian container, unit copied in unchanged and started by the script"
    - "Known-signature fallback: crash text saved as evidence first, then a container-only drop-in"
key-files:
  created:
    - deploy/test/Dockerfile
    - deploy/test/Dockerfile.dockerignore
    - deploy/test/run.sh
    - deploy/test/canary/main.go
    - deploy/test/canary/go.mod
  modified:
    - .gitignore
key-decisions:
  - "ARG TARGETOS/TARGETARCH carry no default in the Dockerfile; the fallback sits at the use site (${TARGETARCH:-amd64})"
  - "Canary uses the literal afNetlink = 16 instead of syscall.AF_NETLINK so go vet ./... compiles on macOS"
requirements-completed: [UNIT-01]
status: complete
duration: 55min
completed: 2026-10-12
actuals:
  tokens: 5300
  tasks: 2
  commits: 2
plan_head_before: ed799e8557dee34ec365088de86e4a816ec9d698
plan_head_after: f46ec35bf513104b5932600eaf56950417d8d593
---

# Phase 20 Plan 01: Test recipe and enforcement canary Summary

**The unchanged deploy/holzcloud.service starts under systemd as PID 1 of a Debian container on linux/amd64 (Rosetta, with a recorded crash and a container-only ProcSubset=all drop-in) and on native arm64 (no drop-in), and a canary run under the unit's own sandbox properties proves seccomp is enforced on arm64 and not on Rosetta.**

## What was built

- `deploy/test/Dockerfile`: stages `build` (cross-compile on the native builder), `canary`, `systemd-base`, `unit-test` (last, default target). The unit is copied to `/etc/systemd/system/` and not enabled; only the five plugin zips travel, not the plugin tree.
- `deploy/test/Dockerfile.dockerignore`: overrides the root `.dockerignore` (which excludes `deploy`) and keeps `.git`, `.planning`, `data` and similar out of the layers.
- `deploy/test/run.sh`: bash 3.2 compatible, no arrays. Subcommands can be chained (`build up start canary`); `exec` and `logs` take the rest of the arguments. Exit codes 0, 1, 2 and 3 as specified.
- `deploy/test/canary/`: stdlib-only probe in its own module `hc-canary`.

## Evidence (gitignored scratch in `deploy/test/out/{amd64,arm64}/`)

| Platform | `smoke` | Unchanged unit | Seccomp of service | `canary` |
|----------|---------|----------------|--------------------|----------|
| linux/amd64 (Rosetta, kernel 7.0.14-linuxkit, systemd 252) | `UNIT-01 PASS` | crashed, `mmap_min_addr`, status 5/TRAP; saved as `unit-start-unchanged.txt`; drop-in `ProcSubset=all` then started it | `Seccomp: 0` | exit 3, `not-enforced`: mprotect RW->RX and mmap RWX ALLOWED even under `MemoryDenyWriteExecute=yes` |
| linux/arm64 native | `UNIT-01 PASS` | started, no drop-in, no `unit-start-unchanged.txt` | `Seccomp: 2`, 24 filters | exit 0, `enforced`: both denied under `yes`, both ALLOWED in the `no` control |

- `/healthz` returned `{"status":"ok"}` and `/readyz` reported status ok and integrity ok on both.
- On amd64 the mount-namespace directives are still enforced (`/dev/kmsg` missing, `/etc` read-only); only the seccomp-based ones are not. This is the statement for the "Container vs. server" section in plan 20-03.
- `git diff --quiet daf651f -- deploy/holzcloud.service` exits 0: the shipped unit is byte-identical.
- Gates `gofmt -l`, `go vet` (host and GOOS=linux), linux amd64/arm64 builds, `tools/english`, `tools/cites -check`, `tools/assembled -check` all pass.

## Task commits

1. Task 1 (tracer): `60f7905`
2. Task 2 (canary): `f46ec35`

The tracer gate ran in auto mode: `<verify>` was re-run end to end on both architectures before Task 2 and passed (the Task 1 verify on amd64 and arm64 was run against the tracer build; the canary additions were then verified separately).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ARG defaults in the Dockerfile override --platform**
- **Found during:** Task 1, first native arm64 smoke
- **Issue:** The plan said to keep `ARG TARGETARCH=amd64` / `ARG TARGETOS=linux`. With the default, `--platform linux/arm64` still built an amd64 binary and the service died with "rosetta error: unable to mmap ExecutableHeap".
- **Fix:** `ARG TARGETOS` / `ARG TARGETARCH` without defaults, `GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}` at the use site, in both `build` and `canary` stages. The ELF machine type of the arm64 binary was checked afterwards.
- **Files modified:** deploy/test/Dockerfile
- **Commit:** 60f7905 (and the canary stage in f46ec35)

**2. [Rule 1 - Bug] syscall.AF_NETLINK does not exist on darwin**
- **Found during:** Task 2
- **Issue:** The plan's host `go vet ./...` would not compile the canary on macOS. A `//go:build linux` tag instead made the vet exit 1 ("no packages to vet").
- **Fix:** `const afNetlink = 16` with a comment; the probe only ever runs in the Linux container.
- **Files modified:** deploy/test/canary/main.go
- **Commit:** f46ec35

### Process notes

- The pre-commit guard refused to commit on `main` (protected branch) at first. The first report went back to the coordinator, the user allowed commits on main, and the coordinator set `git.allow_default_branch_commits=true` in `.planning/config.json` (committed by the coordinator, not by this plan). The tasks were committed afterwards.
- Because both tasks were finished in the working tree before the guard was resolved, Task 1's commit was built from the tracer-state content of `Dockerfile` and `run.sh` (canary parts removed) through the index, and Task 2 added the rest. The committed tree equals the verified working tree.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model (T-20-01 to T-20-04 mitigations are in place: no `-v`, no `-p`, stale container refused, guarded OUT_DIR reset, context filter, unit untouched, canary with control and exit 3).

## Self-Check: PASSED
