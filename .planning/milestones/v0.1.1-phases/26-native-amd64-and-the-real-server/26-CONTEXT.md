# Phase 26: Native amd64 and the real server - Context

**Gathered:** 2026-10-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Hand-over phase. Requirements SRV-01 to SRV-03. What this phase can deliver from an arm64 macOS machine with no amd64 host and no server in reach: (SRV-01) a way to play the five plugins under `MemoryDenyWriteExecute=true` and to time `backup.sh` and `restore.sh` on **native amd64**, which REQUIREMENTS accepts as a GitHub-hosted runner, packaged as a button the owner presses; (SRV-02) the owner steps for a restore on a real server from a backup made there to a different path; (SRV-03) the owner steps for a reboot with the service and timer units enabled. Each step carries its command, the output to expect, what a different output means, and where the result is pasted. The record `deploy/NATIVE-AMD64-RUN.md` holds the steps and a `Result (owner)` slot per requirement.

What this phase cannot deliver, and does not claim: any native amd64 figure, any result from a real server. Pushing, remote branch operations and every outward-facing action need the owner (the proxy answers 403 from here). The requirements stay **not verified** until the owner's results are recorded (D-06).

</domain>

<decisions>
## Implementation Decisions

### Locked (from the owner's brief for this phase)

- **D-01 (SRV-01 is a button):** SRV-01 is delivered as a new workflow `.github/workflows/native-amd64.yml`: `workflow_dispatch` is its only trigger, `runs-on: ubuntu-latest`, `permissions: contents: read`, no secret of any kind, a `timeout-minutes`, one input at most (extra `docker run` flags, passed through an environment variable and never interpolated into a script). Its `uses:` lines are exactly the three official actions the repository already pins in `ci.yml` (checkout, setup-go, upload-artifact), at the same commit hashes. Nothing is loaded from a third party beyond that; the Docker base images and apt packages the harness fetches at image build time are what it has always fetched (README, Safety).
- **D-02 (the harness, not a second one):** The work on the runner is the existing harness reached through `deploy/test/run.sh`'s public subcommands: `mdwe` (build, up, start, canary, seed, check, mdwe-control, verdict: the five plugins under the shipped unit, with the canary deciding whether the directive is enforced) and `backup-chain` with `BULK_MB=300` (the recipe that produced the emulation figures in `deploy/BACKUP-RUN.md`, so native and emulation numbers are comparable line by line). The new logic lives in a **new file**, `deploy/test/native-amd64.sh` (pure verdict functions with a selftest table, the summary writer, the evidence bundle, `record-check`, `status-check`, `appendix`). `run.sh` and `nightly.sh` are not edited.
- **D-03 (verdicts):** The words are PASS, VOID, FAIL, NOT-NATIVE and NOT-RUN. VOID is the harness's own rule: the canary says seccomp is not enforced (`Seccomp: 0`), so the plugin run proves nothing about the directive. NOT-NATIVE is new: the Docker host or the container is not x86_64, so an arm64 SETTLED can never read as amd64 evidence. VOID and NOT-NATIVE end with exit 3 and a red job; FAIL and NOT-RUN end with exit 1; only PASS is green. The result file `native-summary.txt` has a fixed shape (KEY lines, the verdict lines, last line `NATIVE-AMD64 DONE`) that `record-check` recomputes and proves.
- **D-04 (SRV-02 and SRV-03 are owner steps only):** No workflow and no harness run stands in for them. SRV-02: on the real server make a backup with `backup.sh`, restore it with the shipped `restore.sh` to a different path (a throwaway second unit with its own `DATA_DIR` and port, `DATA_DIR` and `SERVICE` given to `restore.sh`), compare, clean up. SRV-03: with `holzcloud.service` and `holzcloud-backup.timer` enabled, reboot the server and show the site, both units and the armed timer back. Command, expected output, meaning of a different output, where to paste, for each.
- **D-05 (the record):** `deploy/NATIVE-AMD64-RUN.md` is in the style of the earlier records (Summary, How this was prepared, one section per requirement, Container vs. server, Not tested, Owner steps, Hand-over to phase 28, Appendix) with a `Status: pending owner | recorded` line and a `Result (owner)` paste block per requirement, and one `Evidence commit:` line that reads `none (pending owner)` until SRV-01 is recorded. `native-amd64.sh record-check` accepts the pending form without claiming a pass and refuses `recorded` without the matching evidence.
- **D-06 (status stays honest):** SRV-01 to SRV-03 are not verified until the owner's results are recorded. Phase 26 is closed on the hand-over: **complete, owner-pending**. ROADMAP.md carries an owner-pending note; REQUIREMENTS.md keeps the three checkboxes unchecked and the traceability cell reads `Pending owner`, a value `requirements.mark-complete` and `phase.complete` both refuse to flip (they flip only `Pending`, `In Progress` and `Gaps Found`), so no automatic step can mark them Complete. The cell becomes `Complete` and the box `[x]` only in the commit that records the result and sets the record's `Status: recorded`; `native-amd64.sh status-check` holds the two files to each other. The phase's verification judges the hand-over (artifacts, gates, honesty), not the owner's runs.
- **D-07 (frozen files):** Not edited by this phase: `deploy/backup.sh`, `deploy/restore.sh`, `deploy/DEPLOY.md`, `deploy/holzcloud.service`, both backup units, `deploy/test/run.sh`, `deploy/test/nightly.sh`, `docs/beta.md`, and the earlier records `deploy/SECURITY-SCORE.md`, `deploy/DEPLOY-RUN.md`, `deploy/BACKUP-RUN.md`, `deploy/BACKUP-TIMER-RUN.md`, `deploy/BACKUP-CONSISTENCY-RUN.md` (`docs/beta.md` quotes them byte for byte, so an edit there breaks phase 28's citations). `deploy/test/README.md` grows by insertion only. The selftest of `nightly.sh` stays at 542 cases, `record-check` at 29 blocks, `record-check-con` at 27, and its removed lines against `b501b1b` at 3 (tripwire 6), because the file is not touched.

### Claude's Discretion

- **D-08 (rehearsal):** The owner steps for SRV-02 and SRV-03 are rehearsed in the existing container harness (amd64 under emulation, the `fresh` install path without Caddy) so that the expected output is observed, not imagined. The rehearsal is quoted in the record as "container rehearsal, not a result", with its deviations named. If Docker is unavailable the expected output is derived from the scripts and labelled as such.
- **D-09 (the old sentence):** "No native amd64 run was made and none is planned (user decision)" in `deploy/SECURITY-SCORE.md` and the matching lines of `docs/beta.md` are superseded by SRV-01. They are not edited here; the record says so and hands the `docs/beta.md` lines to phase 28.
- File names, the marker strings (table under Specifics), the evidence directory names, and the choice of the container path on the runner over the runner's own systemd are decided here. The container path was chosen because it is the recipe already proven on native arm64 (R2 of `SECURITY-SCORE.md`), its figures compare directly with the emulation figures, and a host-level run cannot be tried from here.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deploy/test/run.sh`: `mdwe` (exit 3 = VOID, `verdict.txt` carries `MDWE VERDICT: SETTLED|VOID|FAILS|INCONSISTENT`), `backup-chain` (`summary.txt` with `BAK-01..03`, `RESTORE-NEGATIVE`, `POINT seed:`, `POINT bulk:`, `STEPS:`, `LEAK-CHECK OK`), `up` writes `versions.txt` (`host architecture:`, `container uname:`), `DOCKER_RUN_ARGS` is the hook for a host that needs `--cgroupns=host`.
- `deploy/test/nightly.sh`: `record_check` (appendix byte comparison, `Evidence commit:` rule), `owner_steps_ok`, `leak_check`, the selftest style (`st_eq`, `st_ok`, `st_fail`), the lessons in the README (bash 3.2: a pipeline assignment under `set -e`, `case` patterns inside `$( )`).
- `deploy/test/canary/`: the probe; `deploy/SECURITY-SCORE.md` appendix R1 (amd64 emulated, VOID) and R2 (arm64 native, SETTLED) are real fixtures for the verdict functions; `deploy/BACKUP-RUN.md` appendix holds a real chain `summary.txt`.
- `.github/workflows/ci.yml`: the pinned `actions/checkout@3d3c42e5...`, `actions/setup-go@b7ad1dad...`, `actions/upload-artifact@043fb46d...` and the comment style that explains each pin.

### Established Patterns
- Evidence in gitignored `deploy/test/out/`, pasted unchanged as `#### DIRECTORY/FILE` blocks, proven byte for byte; a claim without recorded output is not allowed.
- Owner steps: numbered, command, expected output, what a different output means (`deploy/BACKUP-TIMER-RUN.md`, "Owner steps"). Phase 24's record already says "Phase 26 (SRV-03) takes O-2 and O-4 into its own owner steps".
- `restore.sh` stops and starts `SERVICE`, replaces `DATA_DIR`, and polls the **production** `http://127.0.0.1:8080/readyz` whatever `DATA_DIR` says (`DEPLOY.md`: a restore pointed at a test directory proves nothing); the owner step for a different path therefore has to check the copy on its own port itself.

### Integration Points
- Phase 28 (DOC-01, DOC-02) cites this record: it may move a `docs/beta.md` line only when the matching `Status` is `recorded`, and lists O-1 to O-3 in OWNER-STEPS.

</code_context>

<specifics>
## Specific Ideas

Evidence directory names (below `deploy/test/out`, prefix `amd64-native`): `amd64-native` (this phase's `native-summary.txt` and `runner.txt`), `amd64-native-mdwe` (`run.sh mdwe`), `amd64-native-bak-src`, `amd64-native-bak-tgt`, `amd64-native-bak-src-bulk`, `amd64-native-bak-tgt-bulk` (`run.sh backup-chain`).

Marker strings a recorded result must contain (they are echoed by the owner steps, so a reader and the checker can find them):

| Requirement | Markers |
|-------------|---------|
| SRV-01 | the pasted block is a valid native summary: last line `NATIVE-AMD64 DONE`, line `SRV-01 PASS`, claimed verdicts equal the recomputed ones |
| SRV-02 | `=== Backup complete: `, `=== Restore complete ===`, `SRV-02 INSTANCE READY`, `SRV-02 TREES EQUAL` |
| SRV-03 | `SRV-03 UNITS ENABLED`, `SRV-03 BOOT CHANGED`, `SRV-03 UNITS ACTIVE`, `SRV-03 SITE READY`, `SRV-03 TIMER ARMED` |

The emulation figures the native ones are set against (`deploy/BACKUP-RUN.md`, `POINT` lines): seed point `BACKUP-WALL-S=0.692`, `RESTORE-WALL-S=8.126`, `START-READY-S=7.397`; bulk point `BACKUP-WALL-S=1.015`, `RESTORE-WALL-S=8.296`, `START-READY-S=7.379`.

How the owner hands a result back: paste it into the `Result (owner)` block of the requirement in `deploy/NATIVE-AMD64-RUN.md`, or give it to a session ("record the native amd64 result"); the recording commit sets `Status: recorded`, the `Evidence commit:` line (SRV-01), the REQUIREMENTS row and box, and `record-check` and `status-check` must both end OK.

</specifics>

<deferred>
## Deferred Ideas

- Running the workflow, the restore on the real server and the reboot: the owner's, after this phase.
- Editing `docs/beta.md` and writing OWNER-STEPS: phase 28 (DOC-01, DOC-02).
- A run on the runner's own systemd (Ubuntu 24.04, systemd 255) outside a container: FUT-05, systemd versions other than 252 and 257.
- A scheduled or push-triggered native run, a self-hosted runner, a matrix of runner images.
- Certificate issuance, DNS and the proxy on a real domain (FUT-03), single sign-on paths (FUT-04).
- Changing the unit if the native run shows the plugins fail under `MemoryDenyWriteExecute=true`: that would be a finding for the owner (the unit's own tripwire comment names the change), not a fix made here.

</deferred>
