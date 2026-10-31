---
phase: 26-native-amd64-and-the-real-server
verified: 2026-10-26T17:00:00Z
status: passed
score: 5/5 hand-over must-haves verified
scope: hand-over only; SRV-01..03 real-world results are owner-pending and are NOT claimed
covered_files:
  - .github/workflows/native-amd64.yml
  - .planning/phases/26-native-amd64-and-the-real-server/26-01-PLAN.md
  - .planning/phases/26-native-amd64-and-the-real-server/26-01-SUMMARY.md
  - .planning/phases/26-native-amd64-and-the-real-server/26-02-PLAN.md
  - .planning/phases/26-native-amd64-and-the-real-server/26-02-SUMMARY.md
  - .planning/phases/26-native-amd64-and-the-real-server/26-03-PLAN.md
  - .planning/phases/26-native-amd64-and-the-real-server/26-03-SUMMARY.md
  - deploy/NATIVE-AMD64-RUN.md
  - deploy/test/native-amd64.sh
covered_digest: "v3:sha256:9acfde8a8adc87b3345f2ebdd3d13cbb5dacdee346fe1d08585c404dca533c19"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 26: Native amd64 and the real server - Verification Report

**Phase Goal (ROADMAP):** The seccomp-dependent directives and the timings are measured on native amd64; restore and reboot on a real server are either recorded or handed over precisely.
**Scope of this verification:** the hand-over only. The owner's real-server runs and the GitHub runner run have not happened and cannot from here; they are not gaps.
**Status:** passed (for the hand-over)
**Re-verification:** No, initial verification

## IMPORTANT: real-world verification is owner-pending

SRV-01, SRV-02 and SRV-03 are NOT verified in the world. No native amd64 run, no real-server restore and no real-server reboot exist. The record says so everywhere (`Status: pending owner`, `Result (owner): pending`, `(nothing pasted yet)`, `Evidence commit: none (pending owner)`). The "measured on native amd64" half of the goal is not achieved and is not claimed; the "handed over precisely" half is what this report judges, and it holds.

## Observable Truths (the five scoped checks)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Workflow is `workflow_dispatch` only, `contents: read`, no secrets, pins equal `ci.yml`, nothing third-party beyond those | VERIFIED | `.github/workflows/native-amd64.yml`: single trigger `workflow_dispatch` (one string input, passed through `env:` `DOCKER_RUN_ARGS`, never interpolated into the `run:` line). `permissions: contents: read`. No `secrets.` reference, no push/pull_request/schedule. Exactly three `uses:` lines, and `diff` of the sorted `uses:` lines against `ci.yml` is empty: checkout `3d3c42e5...`, setup-go `b7ad1dad...`, upload-artifact `043fb46d...` (upload-artifact is the only action ci.yml may not already use; it is by commit pin and first-party `actions/`, and the diff showed ci.yml carries the same pin). `concurrency` without cancel, `timeout-minutes: 90`. |
| 2 | `native-amd64.sh` verdict: PASS only if host and container are x86_64 AND word SETTLED; VOID / NOT-NATIVE exit 3 | VERIFIED | Read the code: `srv01_mdwe` decides NOT-NATIVE first (`arch_is_x86` on both host and container, `?` counts as not x86), then SETTLED->PASS, VOID->VOID, anything else->FAIL. `srv01_overall` precedence NOT-NATIVE, FAIL, NOT-RUN, VOID, PASS (PASS only if both inputs PASS, else FAIL). `srv01_exit`: PASS 0, VOID/NOT-NATIVE 3, everything else 1. `cmd_run` also turns a PASS into exit 1 if the evidence bundle is refused. A stage that was not played is NOT-RUN. Ran `selftest`: `SELFTEST OK (862 cases)` (includes the real R1/R2 texts and an arm64 SETTLED that yields NOT-NATIVE). `record-check`: `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03`. `status-check`: `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`. |
| 3 | `NATIVE-AMD64-RUN.md`: O-1..O-3 each with command and expected output; no claim of a pass; honest Not tested; no secrets; commands correct and safe | VERIFIED | See below. |
| 4 | Frozen files untouched; nightly harness regressions intact | VERIFIED | `git diff --quiet 05fc076 -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud.service deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh .github/workflows/ci.yml` exit 0. `git diff --quiet f1ba5a2 -- docs/beta.md` exit 0. `nightly.sh selftest`: `SELFTEST OK (542 cases)`. With `E=$PWD/deploy/test/out`: `record-check` `RECORD-CHECK OK 29 blocks`, `record-check-con` `RECORD-CHECK OK 27 blocks`. `git status` clean afterwards. |
| 5 | REQUIREMENTS SRV rows read "Pending owner", boxes unchecked | VERIFIED | `.planning/REQUIREMENTS.md` lines 33-35 `- [ ] **SRV-01..03**`; traceability rows 80-82 `Phase 26 \| Pending owner`. `status-check` enforces this pairing against the record. |

### Detail for truth 3 (the owner document)

- **O-1** (SRV-01): `gh workflow run native-amd64.yml --ref main`, `gh run list`, `gh run watch`, then `gh run download RUN-ID --name native-amd64-evidence --dir deploy/test/out`. Expected output is given as a placeholder-marked shape, explicitly "not a result ... not observed on amd64", with a table of what each non-PASS word means (VOID, NOT-NATIVE, FAIL, systemd boot flag, refused bundle). Matches the workflow's artifact name and path.
- **O-2** (SRV-02): checked against `deploy/restore.sh`, `deploy/holzcloud.service` and `deploy/DEPLOY.md`. The restore target is a second, throwaway unit `holzcloud-restore-test` with `DATA_DIR=$T/data` (`/srv/holzcloud-restore-test`) and `SERVICE=holzcloud-restore-test`, passed via `sudo env` (restore.sh honours both variables; it only stops `$SERVICE` and only moves `$DATA_DIR` if it exists, which it does not for the fresh path). Production is not stopped or replaced. The doc correctly warns that restore.sh's closing readiness probe hits production port 8080, and checks the copy on its own port 8081. Safety guards present: a pre-check that the path is free, the instruction to read the confirmation question and answer `no` if it names `holzcloud` / `/opt/holzcloud/data`, the unit derived with `EnvironmentFile`, `PAYREXX`, `SMTP`, `SECRET` lines stripped so the copy cannot send mail or take payments, the copy binds loopback only (`defaultListen = 127.0.0.1` in `internal/config/config.go`), cleanup uses `${T:?}`, and the post-check looks for a `data.before-restore-*` directory. Backup command `sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups` and the `=== Backup complete: ... ===` marker match `backup.sh` and `DEPLOY.md` (line 657/731); restore path matches `DEPLOY.md` line 971.
- **O-3** (SRV-03): before/after blocks tee into `~/` so the output survives the reboot; compares PID 1 start time, `NRestarts`, `Result`, readyz integrity, timer arming. The doc warns to keep a console in reach.
- **No claim of a pass:** Summary rows read `PENDING OWNER ... not run`; every result block reads `(nothing pasted yet)`; container-rehearsal outputs are labelled "Container rehearsal, not a result" with deviations listed; SRV-01 shape explicitly says placeholders.
- **Not tested** section is substantial (spare machine/real network, power loss and 03:15 catch-up, host-level Ubuntu 24.04 / systemd 255, proxy/certs, cold cache/own mount, secrets-bearing backup into a secretless copy).
- **Secrets:** a scan of the document for private-key markers, `sk-`, `AKIA`, `ghp_` and `password=` forms found nothing; the doc states that nothing in the steps prints a secret.
- Hand-over to phase 28 lists the five `docs/beta.md` lines, unedited, with the rule for when each may move.

## Anti-Patterns / Findings

| Severity | Item | Impact |
|----------|------|--------|
| Info (advisory, not a gap) | O-2 first block prints `STOP: $T exists` but does not halt; a reader who ignores it and continues would `install -d -o holzcloud -m 700 "$T"` onto, and later `rm -rf`, an existing directory. The doc's expected output says "`STOP` appears nowhere" and the variable `T` defaults to a fresh path under `/srv`, so this only bites a person who overrides it and ignores the message. | Low; consider a hard `exit`/`return` in a later revision. Not a defect of the hand-over. |
| Info | O-2's unit derivation `sed` assumes the installed unit has the shipped `HOLZCLOUD_PORT=8080` and `/opt/holzcloud/data` lines. A customised unit that differs fails safe: the copy cannot bind or the owner sees it in the `grep` output the doc asks them to read; production is untouched. | None |
| Info | ROADMAP progress table row for phase 26 still reads `3/3 In Progress`; closing the phase is the orchestrator's step. | None for verification |

No debt markers (TBD/FIXME/XXX) in the new files were found by the checks run; no stubs: the script is 2613 lines with a 862-case selftest that passes.

## Requirements Coverage

| Requirement | Hand-over state | Real-world state |
|-------------|-----------------|------------------|
| SRV-01 | Instrument complete (workflow, verdict rules, evidence bundle, checker, O-1) | **Owner-pending**: no native amd64 run, no native figure |
| SRV-02 | Owner step O-2 complete and checked against restore.sh/DEPLOY.md | **Owner-pending**: no restore on a real server |
| SRV-03 | Owner step O-3 complete | **Owner-pending**: no real reboot |

All three correctly read `Pending owner`, boxes unchecked, with `status-check` holding REQUIREMENTS.md to the record.

## Human Verification Required (owner actions, not gaps)

1. **O-1:** press the `native-amd64` workflow button; paste the `---- native amd64 summary ----` block; only `SRV-01 PASS` can be recorded.
2. **O-2:** run the restore-to-a-different-path steps on the real server; paste output.
3. **O-3:** reboot the real server with units enabled; paste before/after output.
Each is recorded by a separate commit and accepted only when `record-check` and `status-check` end OK.

## Gaps Summary

None in the hand-over. The instrument is sound, the verdict logic cannot produce a green on a non-native or non-enforcing platform (proved by the selftest and by reading the code), the frozen files are byte-identical to their evidence commits, and the record is honest about what has not been done. The phase goal's measurement half remains owner-pending by design.

---

_Verified: 2026-10-26_
_Verifier: Claude (gsd-verifier)_
