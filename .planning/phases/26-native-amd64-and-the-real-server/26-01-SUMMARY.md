---
phase: 26-native-amd64-and-the-real-server
plan: 01
subsystem: deploy/test harness, CI
tags: [native-amd64, workflow_dispatch, verdict-functions, evidence-bundle, srv-01]
status: complete
requires:
  - phase: 20-25
    provides: deploy/test/run.sh (mdwe, backup-chain), the R1/R2 and chain-summary records used as fixtures
provides:
  - deploy/test/native-amd64.sh (selftest, pure verdict functions, summarize, bundle, run, down)
  - .github/workflows/native-amd64.yml (manual-dispatch button for SRV-01)
affects: [26-02 (record-check, status-check, appendix extend native-amd64.sh), 26-03, 28]
tech-stack:
  added: []
  patterns: [pure verdict functions with a real-fixture selftest table, evidence bundle with refuse-and-leave-nothing scan]
key-files:
  created:
    - deploy/test/native-amd64.sh
    - .github/workflows/native-amd64.yml
  modified: []
key-decisions:
  - "The verdict is a function of the lines native-summary.txt displays (HOST-ARCH, CONTAINER-ARCH, MDWE-WORD and the chain lines), so plan 02's record-check can recompute it; the stage status numbers are shown but do not change a verdict"
  - "NATIVE_PREFIX, container names and refusals cover four containers (the -bak-tgt-bulk target too), because run.sh up refuses a stale one"
requirements-completed: []
duration: not measured (no start time was recorded)
completed: 2026-10-26
commits: 2
plan_head_before: 8f684c9bfcdd27af9a44193df1f767550ff8d123
plan_head_after: 72b3868d90f63598b30d95b8d79d53ef1f5cc08b
estimate:
  tokens: 95000
  tasks: 2
actuals:
  tokens: 15500
  tasks: 2
  commits: 2
---

# Phase 26 Plan 01: Native amd64 button and verdict instrument Summary

SRV-01's instrument exists and has been shown on the real path from this arm64 machine: a manual-dispatch workflow plays `run.sh mdwe` and `run.sh backup-chain` on a GitHub-hosted amd64 runner, and `deploy/test/native-amd64.sh` decides the verdict by rules an emulated or arm64 host cannot satisfy. SRV-01 itself is **not claimed**: no native amd64 run exists, `requirements-completed` is empty, and the requirement stays pending owner (D-06).

## What was built

**`deploy/test/native-amd64.sh`** (1373 lines, bash 3.2, a sibling of `run.sh` and `nightly.sh` that edits neither and reaches `run.sh` through `mdwe` and `backup-chain` only):

- Pure functions: `arch_is_x86`, `mdwe_word_of`, `srv01_mdwe` (NOT-NATIVE decided first), `srv01_timing` (names the first missing line), `srv01_overall` (precedence NOT-NATIVE, FAIL, NOT-RUN, VOID, PASS), `srv01_exit` (PASS 0, FAIL/NOT-RUN 1, VOID/NOT-NATIVE 3).
- `summarize`: reads the `mdwe` and `bak-src` directories with every read guarded, writes `runner.txt` and `native-summary.txt` (the KEY lines, the three `SRV-01*` verdict lines, last line `NATIVE-AMD64 DONE`).
- `run`: refuses without Docker or when any of the four containers exists, resets its own and the evidence directory with `run.sh up`'s guards, pins `NEW_REF`, plays the stages in `NATIVE_STAGES` as child processes with statuses kept, summarizes, publishes, exits with the verdict's code. `down` removes the containers.
- `bundle`: copies the six evidence directories as `DIRECTORY/FILE`, drops `step-*.txt`, `seed.txt`, `*.log`, `*cookie*` (case-insensitive), `id_*`, `*.pem`, `*.key` and any link, scans what is left for `PRIVATE KEY` and `hc_` plus 20 or more key characters, builds in a temporary sibling and renames only after the scan, so a refusal leaves nothing. An empty result (no directory, or only excluded files) is refused. `write_job_summary` appends the summary as a fenced `text` block to `GITHUB_STEP_SUMMARY` when it is set; a refused bundle turns an otherwise green run into exit 1.
- `selftest`: 274 cases, no Docker, ends `SELFTEST OK (274 cases)` on stdout. Fixtures are the real R1 (amd64 emulated, VOID) and R2 (arm64 native, SETTLED) `versions.txt`/`canary`/`check.txt`/`mdwe-control.txt`/`verdict.txt` texts of `deploy/SECURITY-SCORE.md` and the real chain `summary.txt` of `deploy/BACKUP-RUN.md`. The one synthetic set (R2 with `aarch64` turned into `x86_64`, to exercise PASS) is labelled so in the code and exists only in the selftest's temporary directory. `summarize` is exercised under `set -e` with missing directories, both stages missing, neither stage played, and one stage played.

**`.github/workflows/native-amd64.yml`**: `workflow_dispatch` only, one string input `docker_run_args` reaching the script as `env: DOCKER_RUN_ARGS`, `permissions: contents: read`, fixed concurrency group without cancel, `ubuntu-latest`, `timeout-minutes: 90`, checkout (`fetch-depth: 0`), setup-go, `bash deploy/test/native-amd64.sh run`, and an `if: always()` upload of `deploy/test/out/native-evidence` as `native-amd64-evidence` (`if-no-files-found: error`, 90 days). The `uses:` set equals `ci.yml`'s three pins. No secret, no other action.

## Verification

- `bash deploy/test/native-amd64.sh selftest`: `SELFTEST OK (274 cases)`.
- The Task 1 workflow lint set passed: one trigger, read-only contents as the only permission, `ubuntu-latest` and a timeout, no `secrets.` in any string value, `uses:` set equal to `ci.yml`'s, every `run:` block parses under bash 3.2, the env mapping of `DOCKER_RUN_ARGS`.
- Mutation checks (done by hand, mutants not committed): changing the exit code of VOID, widening `arch_is_x86`, putting FAIL before NOT-NATIVE, dropping either scan, widening the key shape to 19 characters, dropping the `seed.txt`, `step-*` or cookie exclusion (the `*.log`, `id_*`, `*.pem` and `*.key` exclusions were not mutated), not removing links, leaving an old bundle behind and ignoring a refused bundle each make the selftest fail. One weak case was found this way and fixed (see Deviations).
- `nightly.sh selftest` still ends `SELFTEST OK (542 cases)`; `run.sh`, `nightly.sh`, `backup.sh`, `restore.sh`, `DEPLOY.md`, both units, `docs/beta.md` and the earlier records are byte-identical to `05fc076`.

### Rehearsal on the real path (this machine, arm64, Docker Desktop 29.3.1, aarch64)

Command (host date `Mon Oct 26 15:52:33 CET 2026` to `15:53:27 CET 2026`, images were cached): `PLATFORM=linux/arm64 NATIVE_PREFIX=rehearsal-arm64-native NATIVE_STAGES=mdwe EVIDENCE_DIR=deploy/test/out/rehearsal-arm64-evidence bash deploy/test/native-amd64.sh run`.

Result: `run.sh mdwe` ended with status 0 and the verdict SETTLED (a second native arm64 measurement of the same thing as R2, at commit `c5a3d61`: canary enforced, `Seccomp: 2`, shipped engine interpreter, control engine compiler, 17 and 17 assertions). `native-summary.txt` read `HOST-ARCH: aarch64`, `MDWE-WORD: SETTLED`, `SRV-01-MDWE NOT-NATIVE`, `SRV-01-TIMING NOT-RUN`, `SRV-01 NOT-NATIVE`, last line `NATIVE-AMD64 DONE`; the script exited 3; the bundle held nine files with no `step-*`, `seed.txt` or `*.log`; `down` left no container. As predicted, an arm64 SETTLED did not read as amd64 evidence. The timing stage was not played here (the plan says so: an arm64 container cannot play the document's amd64 Build line without emulation); its verdict function is covered by the real chain summary fixture. The rehearsal directories are gitignored scratch and were not pasted anywhere.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A weak selftest case, found by a mutation check**
- **Found during:** Task 2, after the bundle tests passed on the first run
- **Issue:** the case "link is not kept" used `test -e`, which is false for a dangling link, so it passed even with the link removal deleted. A summarize case of the first draft built on a base directory that does not exist tested the harness, not `summarize`, and was dropped before the first run.
- **Fix:** `exists_any` (file, directory or link), and the fixture link now points at an existing file.
- **Files modified:** `deploy/test/native-amd64.sh`
- **Commit:** `72b3868`

### Plan deviations and observations (no rule applied)

- **The plan's own verify line for `nightly.sh` cannot pass as written.** `bash deploy/test/nightly.sh selftest | tail -n 2 | head -n 1 | grep -qx 'SELFTEST OK (542 cases)'` reads stdout only, and `nightly.sh` prints `SELFTEST OK (542 cases)` on stderr and only `SELFTEST OK` on stdout. Run with `2>&1` the second-to-last line is exactly `SELFTEST OK (542 cases)`, which is what I checked. Plan 02 and 03 should not copy that line unchanged. (`native-amd64.sh` prints its count on stdout, as the Task 1 verify needs.)
- **Six evidence directories, not five.** The plan says "the five derived directories"; CONTEXT lists six (`amd64-native`, `-mdwe`, `-bak-src`, `-bak-tgt`, `-bak-src-bulk`, `-bak-tgt-bulk`). The bundle copies all six that exist.
- **Four container names are checked and removed, not three.** `run.sh backup-chain` also creates `holzcloud-<prefix>-bak-tgt-bulk`, and `run.sh up` refuses a stale container, so `run` refuses and `down` removes it as well.
- **`TARGET=unit-test` is passed to `run.sh mdwe` explicitly** (it is `run.sh`'s default) so an exported `TARGET` of the caller cannot change which image the plugin stage uses.
- **The Bash tool's worktree guard refused every command text that invoked `bash` or was not a plain command**, so all runs (selftest, lint, rehearsal, commits) went through wrapper scripts outside the repository, as the brief allowed.

## Known limits (stated, not hidden)

- **Nothing here has run on amd64 or on GitHub.** The workflow was linted with `yq` only (no `actionlint`, not dispatched; the rules forbid triggering anything on GitHub). Untested until the owner presses the button: whether systemd boots in the privileged containers on `ubuntu-latest` with the default flags (the `docker_run_args` input is the hook if it does not), the actual run time against the 90-minute ceiling (a guess, no native run has been timed), and the real `runner.txt` on a hosted runner.
- **A crash before `versions.txt` exists reads NOT-NATIVE (exit 3).** The specified rule treats an unknown architecture (`?`) as not x86_64, and the verdict has to stay recomputable from the displayed lines. The crash is visible in the same file (`HOST-ARCH: ?`, `MDWE-STATUS: 1`) and the job is red either way, but the exit code 3 can mislead; a hardening would show the stage status in the verdict word. I did not add it because it would make the verdict depend on a value `record-check` may not read.
- **`DOCKER_RUN_ARGS` is trusted input of a repository writer.** `run.sh` splits it at spaces on purpose; the workflow passes it by environment variable only (T-26-01).

## Threat model coverage

T-26-01 (input only through `env:`, asserted by the lint), T-26-02 (exclusions plus both scans plus the chain's own `LEAK-CHECK OK` required for PASS, all in the selftest), T-26-03 (NOT-NATIVE first, exit 3, real R1/R2 fixtures, guard rehearsed on the real path), T-26-05 (`timeout-minutes`, fixed concurrency group, dispatch only), T-26-SC (no new action or version: `uses:` set equals `ci.yml`'s) are mitigated as planned. T-26-04 accepted as planned. No new security surface beyond the plan's register.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: `deploy/test/native-amd64.sh`, `.github/workflows/native-amd64.yml`
- FOUND commits: `c5a3d61`, `72b3868` (both ancestors of HEAD; two commits counted from `plan_head_before`)
- Frozen files unchanged since `05fc076`; working tree clean after the task commits
