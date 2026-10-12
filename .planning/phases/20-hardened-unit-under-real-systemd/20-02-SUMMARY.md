---
phase: 20-hardened-unit-under-real-systemd
plan: 02
subsystem: deploy
tags: [systemd, wazero, plugins, mdwe, seccomp, mcp, docker]
requires: ["20-01"]
provides:
  - "run.sh subcommands seed, check, mdwe-control, verdict, mdwe (exit 3 = platform cannot give the evidence)"
  - "UNIT-02 evidence: five plugins loaded, running and invoked under the hardened unit on linux/arm64 (enforced) and linux/amd64 (Rosetta, not enforced)"
  - "MDWE verdict SETTLED on native arm64, VOID on amd64/Rosetta"
affects: [20-03, 21, 22]
tech-stack:
  added: []
  patterns:
    - "Seeding through the MCP endpoint /ai with an admin key that lives only in a root-only file inside the container"
    - "Verdict gated on the canary: a plugin-load pass on a platform without seccomp is reported VOID, never SETTLED"
    - "One EXIT handler restores the mdwe-control drop-in and removes the container"
key-files:
  created: []
  modified:
    - deploy/test/run.sh
    - deploy/holzcloud.service
key-decisions:
  - "MemoryDenyWriteExecute=true stays. Evidence from an enforcing platform: all five plugins load and run on wazero's interpreter fallback; the unit gains comment lines only"
  - "The arm64 run is printed as DEVIATION[D-01] (user decision 2026-10-12) and is the only MDWE evidence; amd64 under Rosetta is VOID (Seccomp: 0)"
  - "\"plugin loaded\" is read from the service invocation (_SYSTEMD_INVOCATION_ID), not only --since: --since has a one-second resolution and let in a line of the previous invocation"
requirements-completed: [UNIT-02]
status: complete
duration: 25min
completed: 2026-10-12
actuals:
  tokens: 5600
  tasks: 3
  commits: 3
plan_head_before: 7b293329886f59324c806321c61b8b5597332af3
plan_head_after: dd38a10da5e09f3825e204cd3e7dfb9abd78c7c9
---

# Phase 20 Plan 02: Plugins under the hardened unit and the MDWE verdict Summary

**All five shipped wasm plugins load, run and answer HTTP under the unchanged hardened unit; on native arm64 (seccomp enforced) MemoryDenyWriteExecute=true makes wazero fall back silently to its interpreter and the verdict is SETTLED, while on amd64 under Rosetta the same run reports VOID and exits 3.**

## What was built

- `seed`: admin key created inside the container (`holzcloud ai key create`, run as the service user), kept in `/root/.hc-key` under umask 077, never on the host. Through `/ai` it creates website `Test` with domain `test.local`, installs, binds and enables the five plugins (archives piped as base64 through `jq -Rs`), publishes page `plugintest` with the shortcodes, uploads `velo-01.jpg` over a one-time link and asserts a `-thumb` variant exists. Prints `SEED OK`.
- `check` / `run_checks`: restart, wait, then 17 `PASS` assertions (5 loaded, no load failures, 5 running with empty `last_error`, 5 invoked over HTTP with `Host: test.local`, stable: `NRestarts=0`, `Result=success`, no `code=killed`/`status=31`/`SIGSYS`), followed by the `engine:` line from `/proc/PID/maps`, four `LATENCY` medians and the `Seccomp` lines. Prints `CHECK OK`.
- `mdwe-control`: drop-in `MemoryDenyWriteExecute=no`, same checks into `mdwe-control.txt`, then restore, assert `yes`, print `RESTORED`. Restoration also runs from the single EXIT handler.
- `verdict`: SETTLED, VOID (exit 3), FAILS, INCONSISTENT; reads `canary.verdict` first. `mdwe` chains the narrow run.
- `deploy/holzcloud.service`: seven comment lines above `MemoryDenyWriteExecute=true` (wazero probe, silent interpreter fallback, tripwire `NewRuntimeConfigCompiler`). The directive is untouched.

## Evidence (gitignored scratch in `deploy/test/out/{amd64,arm64}/`)

| Platform | Seccomp | Canary | Engine, shipped unit | Engine, `=no` control | Checks | Verdict | Exit |
|----------|---------|--------|----------------------|-----------------------|--------|---------|------|
| linux/arm64 native | 2 (24 filters; 22 in the control) | enforced | interpreter, 0 anonymous exec mappings | compiler, 7 to 9 | 17/17 and 17/17 | SETTLED | 0 |
| linux/amd64 (Rosetta) | 0 | not-enforced | compiler, 10 mappings | compiler, 9 | 17/17 and 17/17 | VOID | 3 |

Median latency of ten requests (seconds), arm64, interpreter vs compiler: `/plugintest` 0.011 vs 0.0023, `/suche?q=Plugintest` 0.0077 vs 0.0020, `/formular` 0.00087 vs 0.00025, `/bestellung` 0.00080 vs 0.00024. The hook budget `CallTimeout` is 2 s, so the interpreter is slower by roughly 3 to 5 times but about 200 times inside the budget. On Rosetta the form routes cost about 5.8 ms either way (emulation overhead dominates).

- The amd64 result is functional evidence only: it shows the plugins load and run, and says nothing about MDWE because no filter is installed there (a plugin pass on amd64 uses the compiler, which proves the opposite of what an enforced run shows).
- `git diff -U0 daf651f -- deploy/holzcloud.service` holds only added `+#` lines; `git diff --quiet daf651f -- internal plugins` exits 0.
- No API-key-shaped string under `deploy/test/out`; the key never reached the host (the upload link, also a secret, is stripped from `seed.txt`).
- `docker ps -a` shows no leftover test container; the drop-in was restored (`RESTORED`) in both runs.

## Task commits

1. Task 1 (seed): `7587e37`
2. Task 2 (check): `5a2e612`
3. Task 3 (control, verdict, unit comment): `dd38a10`

The three commits were cut from one verified working tree through the index (intermediate `run.sh` versions with only the subcommands of each task), so each commit is self-consistent; the final tree equals the one both `mdwe` runs were executed against.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `--since` let a line of the previous invocation into the "loaded" assertions**
- **Found during:** Task 3, first arm64 `mdwe` run
- **Issue:** `ActiveEnterTimestamp` prints whole seconds, so a `plugin loaded` line from the invocation before the restart (same second) appeared in the output and could turn a missing load into a false PASS.
- **Fix:** the loaded assertions and the verbatim `plugin loaded` lines now read `journalctl _SYSTEMD_INVOCATION_ID=<id> --since <ts>`; the stability check keeps the wider `-u holzcloud --since <ts>` query on purpose, so systemd's own kill messages are still seen.
- **Files modified:** deploy/test/run.sh
- **Commit:** dd38a10

**2. [Rule 3 - Blocking] `cmd_canary` ends the script with `exit 3`**
- **Issue:** `mdwe` must tolerate the canary's exit 3 without aborting.
- **Fix:** `( cmd_canary ) || crc=$?` in a subshell; 0 and 3 continue, anything else is fatal. `cmd_verdict` is run the same way so its exit code is the script's.
- **Commit:** dd38a10

### Process notes

- The plan's smoke-time `trap` in `cmd_smoke` was replaced by one global `on_exit` handler (`CLEANUP_DOWN`, `MDWE_DROPIN`), because the `mdwe-control` restore and the container removal both have to run on the same EXIT and a second `trap` would have replaced the first. The `smoke` behaviour is unchanged.
- `bash -n` only; `shellcheck` is not installed on this host.
- `deploy/SECURITY-SCORE.md`, referenced by the new unit comment, does not exist yet; plan 20-03 creates it.
- `state.advance-plan` was not run by hand-editing: see State updates below.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model: T-20-05 (key stays in the container, grep of output clean), T-20-06 (drop-in removed on a trap-protected path, `RESTORED` printed), T-20-07 (amd64 returns VOID and exit 3), T-20-08 (unit diff is comment-only) are all in place.

## Self-Check: PASSED

- `deploy/test/run.sh`, `deploy/holzcloud.service` present; commits `7587e37`, `5a2e612`, `dd38a10` are ancestors of HEAD.
- `cmd_seed`, `cmd_check`, `run_checks`, `cmd_mdwe_control`, `cmd_verdict`, `cmd_mdwe` defined; four verdict words present.
