---
phase: 20-hardened-unit-under-real-systemd
plan: 03
subsystem: deploy
tags: [systemd, systemd-analyze, security-score, mdwe, wazero, docs, handover]
requires: ["20-02"]
provides:
  - "run.sh subcommands score and all; shared assert_http used by smoke and all"
  - "deploy/SECURITY-SCORE.md: UNIT-01 to UNIT-03 record with raw systemd-analyze security output, container-vs-server table, D-01 deviation"
  - "deploy/test/README.md: hand-over of the recipe to phases 21 and 22"
affects: [21, 22]
tech-stack:
  added: []
  patterns:
    - "score runs before start so the measurement is of the unit as shipped, without the emulation drop-in"
    - "Document appendix assembled from the scratch files by script, then proven line by line with grep -F"
key-files:
  created:
    - deploy/SECURITY-SCORE.md
    - deploy/test/README.md
  modified:
    - deploy/test/run.sh
key-decisions:
  - "No target value imposed on the score: 1.3 online / 1.4 offline on systemd 252, 1.3 / 1.3 on 257, recorded raw (D-03a)"
  - "MDWE verdict stays: SETTLED on native arm64, VOID on amd64 under emulation; the document states that no native amd64 run was made"
  - "Container-vs-server rows labelled measured, inferred or assumed; seccomp claims checked against the systemd.exec manual, which names seccomp/system call filter only for SystemCallFilter, MemoryDenyWriteExecute and SystemCallArchitectures"
requirements-completed: [UNIT-01, UNIT-02, UNIT-03]
status: complete
duration: 45min
completed: 2026-10-12
actuals:
  tokens: 26800
  tasks: 3
  commits: 3
plan_head_before: 65cc0eb002969eccbf4d869a8536adbadad70708
plan_head_after: 783027b4c2330506afa07b5a5d1df6694bffe59c
---

# Phase 20 Plan 03: Security score, container-versus-server record and hand-over Summary

**The unit as shipped scores 1.3 online and 1.4 offline under systemd 252 (1.3 and 1.3 under 257), measured and committed raw in deploy/SECURITY-SCORE.md with every container-versus-server difference labelled measured, inferred or assumed, and deploy/test/README.md hands the test recipe to phases 21 and 22.**

## What was built

- `deploy/test/run.sh`: `score` writes `systemd-analyze security` online (`score-online.txt`) and offline (`score-offline.txt`) plus `systemctl cat` (`score-unit.txt`) raw, and exits 1 when either score file has no `Overall exposure level for` line. `all` runs build, up, score, start, the UNIT-01 assertions, canary, seed, check, mdwe-control, verdict and a summary; exit 3 from canary and verdict is tolerated, anything else fails. `assert_http` is now shared by `smoke` and `all`.
- `deploy/SECURITY-SCORE.md`: nine sections as planned, appendix with 15 raw files pasted from the scratch output.
- `deploy/test/README.md`: twelve sections; 16 subcommands and 11 variables, each with a table row.

## Evidence (one fresh run set, 2026-10-12, repository at f1428d2)

| Run | Result |
|-----|--------|
| R1 `all`, linux/amd64 (emulated, systemd 252) | exit 0; unchanged unit crashed with the `mmap_min_addr` signature (status 5/TRAP), container-only drop-in `ProcSubset=all`, then UNIT-01 PASS; canary `not-enforced` (`Seccomp: 0`; mprotect RW->RX, mmap RWX and AF_NETLINK ALLOWED); 17/17 checks on the compiler engine; verdict VOID; score 1.3 online, 1.4 offline |
| R2 `mdwe`, linux/arm64 native | exit 0; unchanged unit started with no drop-in; canary `enforced` (`Seccomp: 2`); shipped unit on the interpreter engine, control on the compiler, 17/17 and 17/17; verdict SETTLED |
| R3 `build up score`, amd64, trixie (systemd 257) | exit 0; 1.3 online, 1.3 offline; the online table is identical to the 252 online table |

Exposed rows with weights: RemoveIPC 0.1, RootDirectory 0.1, RestrictAddressFamilies=~AF_UNIX 0.1, ~AF_(INET|INET6) 0.3, PrivateNetwork 0.5, PrivateUsers 0.2, DeviceAllow 0.1, IPAddressDeny 0.2, and (offline on 252 only) PrivateMounts 0.2. The offline-versus-online difference on 252 is that one row.

## Task commits

1. Task 1 (score, all): `f1428d2`
2. Task 2 (SECURITY-SCORE.md): `5bb0d6c`
3. Task 3 (README.md): `783027b`

Verification: the `grep -F` loop over the 15 evidence files printed `raw-blocks-match`; nine headings in order; four `Overall exposure level for holzcloud.service` lines; no key-shaped string; `go run ./tools/i18n` (0 open, 0 orphaned on every catalogue), `gofmt -l .`, `tools/english`, `tools/cites -check`, `tools/assembled -check` and `go vet ./...` clean. `git diff --quiet daf651f -- internal plugins` exits 0; the unit differs from the planning base only by added `#` comment lines. No test container is left (`docker ps -a`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's README facts about backup.sh were stale**
- **Found during:** Task 3, reading `deploy/backup.sh` as the plan asked
- **Issue:** The plan said `backup.sh` needs `sqlite3`. The current script takes its snapshot with `holzcloud backup` and copies media and templates with `rsync`; only DEPLOY.md's prerequisites still name the `sqlite3` CLI. The image installs `sqlite3` but not `rsync`, and contains neither backup script nor backup units. The unit's `ExecStart` path (`/opt/holzcloud/deploy/backup.sh`) also differs from where DEPLOY.md's install steps copy the script (`/opt/holzcloud/backup.sh`).
- **Fix:** The README states what the files say and hands the `rsync` question, the missing scripts, the `sqlite3` documentation mismatch and the path mismatch to phase 22. Nothing was changed in DEPLOY.md or the Dockerfile (out of scope for this plan).
- **Files modified:** deploy/test/README.md
- **Commit:** 783027b

**2. [Plan wording] SECURITY-SCORE.md was composed from a template through a script, not typed with a single Write**
- **Why:** The plan demands every raw block be byte-identical to its scratch file, including trailing whitespace of the tool's tables. The prose was written with the Write tool; the raw blocks were substituted from the scratch files by a script kept outside the repository (scratchpad), and the `grep -F` loop proved the match.
- **Commit:** 5bb0d6c

### Findings recorded, not fixed

- The systemd.exec manual (read 2026-10-12) names seccomp or a system call filter only for `SystemCallFilter=`, `MemoryDenyWriteExecute=` and `SystemCallArchitectures=`. `RestrictAddressFamilies=`, `RestrictNamespaces=`, `RestrictRealtime=`, `RestrictSUIDSGID=` and `LockPersonality=` are listed as seccomp-based in the plan; the document marks the manual-unconfirmed ones assumed or inferred instead of repeating the plan's claim.
- That the emulation layer is Rosetta is inferred from the crash text (`VMAllocationTracker.cpp`), not confirmed from Docker settings; the table says so.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-20-09: `seed.txt` and every journal beyond `check.txt` stayed out of the documents, no key-shaped string and no host path in either file. T-20-10: every container-vs-server row has a status; the Not measured section lists native amd64, systemd 255, real hardware and QEMU. T-20-11: all evidence is from one fresh run set made after the last code change.

## Issues for the coordinator

- `state.advance-plan` was not run: it refuses with `ambiguous_position_phase` (several `Phase:` entries in STATE.md). STATE.md structure was not edited by hand.
- `.planning/config.json` and `.planning/milestone.lock` were left unstaged as instructed.

## Self-Check: PASSED

- `deploy/SECURITY-SCORE.md`, `deploy/test/README.md`, `deploy/test/run.sh` present; commits `f1428d2`, `5bb0d6c`, `783027b` are ancestors of HEAD; `cmd_score` and `cmd_all` defined.
