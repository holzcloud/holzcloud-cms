---
phase: 26-native-amd64-and-the-real-server
plan: 02
subsystem: deploy/test harness, owner hand-over record
tags: [native-amd64, record-check, status-check, owner-steps, srv-01, srv-02, srv-03, hand-over]
status: complete
requires:
  - phase: 26-01
    provides: deploy/test/native-amd64.sh (verdict functions, summarize, bundle), the native-amd64.yml workflow
provides:
  - deploy/test/native-amd64.sh record-check, status-check, appendix (summary_check, srv_record_check, status_check, native_paste_list, native_markers)
  - deploy/NATIVE-AMD64-RUN.md (the record in the pending-owner form, owner steps O-1 to O-3)
affects: [26-03 (README section, REQUIREMENTS cells Pending owner, ROADMAP note), 28 (docs/beta.md lines, OWNER-STEPS)]
tech-stack:
  added: []
  patterns: [a checked pending state between "empty" and "recorded", markers at line start so a command echo is not an output, a verdict recomputed from the displayed lines]
key-files:
  created:
    - deploy/NATIVE-AMD64-RUN.md
  modified:
    - deploy/test/native-amd64.sh
key-decisions:
  - "A recorded SRV-02/SRV-03 marker must BEGIN a line of the pasted block, not merely occur in it: a pasted command line (`echo \"SRV-02 TREES EQUAL\"`) is not the output of one"
  - "The appendix of a recorded SRV-01 may hold a subset of the thirteen paths of the fixed list, as long as it includes amd64-native/native-summary.txt, every block equals its scratch file and the summary block equals the Result (owner) block"
  - "summary_check also requires CANARY: enforced for a PASS and the MDWE-STATUS/TIMING-STATUS lines, and honours `notrun` there, because those are the lines that summarize writes and the verdict depends on"
requirements-completed: []
duration: "about 40 minutes of work, 15:57 to 16:35 CET on the host clock (no start time was recorded; taken from the first and last logged commands)"
completed: 2026-10-26
commits: 3
plan_head_before: 8359caa33db96db694b7bf3cba2071d80e4c133b
plan_head_after: 60d02b1506b7a4a26c2082d8bd29ea2b7e098b38
estimate:
  tokens: 120000
  tasks: 3
actuals:
  tokens: 25900
  tasks: 3
  commits: 3
---

# Phase 26 Plan 02: Record checker and the owner hand-over Summary

The hand-over of SRV-01, SRV-02 and SRV-03 is in place and claims nothing: `deploy/NATIVE-AMD64-RUN.md` holds the three owner steps and the three requirements in the `pending owner` form, and `bash deploy/test/native-amd64.sh record-check` ends `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03`. **No requirement is verified.** `requirements-completed` is empty; no native amd64 run, no restore on a real server and no reboot of a real server exists.

## What was built

**`deploy/test/native-amd64.sh`** (plan 01's functions keep their names and answers; selftest grew from 274 to 862 cases):

- `summary_check FILE`: a native summary is valid when it starts `---- native amd64 summary ----`, ends `NATIVE-AMD64 DONE`, has exactly one of each KEY line and verdict line, and its three claimed verdicts (first word after `SRV-01-MDWE`, `SRV-01-TIMING`, `SRV-01`) equal the ones `srv01_mdwe`, `srv01_timing` and `srv01_overall` recompute from the lines of the file itself. A stage that `summarize` marked `notrun` reads NOT-RUN. A PASS over a canary that is not `enforced` is refused.
- `srv_record_check` (`record-check`, `R` and `E` from the environment): the pending form is accepted without a claim (no result text, `Evidence commit: none (pending owner)`, an empty appendix, `PENDING OWNER` and no pass word in a pending Summary row, no line `SRV-0N PASS` outside a fence); `recorded` is refused unless the evidence is there (SRV-02 and SRV-03: every marker at the start of a line of the Result block; SRV-01: a valid summary with verdict PASS, an `Evidence commit:` sha that `git cat-file` knows and that is a prefix of the summary's `COMMIT:`, an appendix on the fixed list that includes the native summary, with every block byte-equal to its scratch file and the summary block equal to the Result block). It also checks the headings, the one `Evidence commit:` line, the private-key marker, and the Owner steps (O-1 to O-3 each with a fenced block, `Expect`, `different output`, `Paste`, and every marker the step has to produce).
- `status_check` (`status-check`, `R` and `REQS`): pending means `- [ ] **SRV-0N**` and the cell `Pending owner`, recorded means `- [x]` and `Complete`.
- `appendix_print` (`appendix`, `E`): the thirteen files of `native_paste_list` as `#### DIRECTORY/FILE` and a `text` block each, exit 1 with nothing on stdout when a file is missing or holds a line that begins with three backticks, `#### ` or `## `.

**`deploy/NATIVE-AMD64-RUN.md`**: the record, 371 lines. Summary (three `PENDING OWNER` rows, `Evidence commit: none (pending owner)`), How this was prepared, the three requirement sections (what it will show, what it does not show, `Status: pending owner`, `Result (owner): pending`, `(nothing pasted yet)`), the emulation figures SRV-01 will be set against (the two `POINT` lines quoted byte for byte, an empty native column), Container vs. server, Not tested, Owner steps O-1 to O-3 with a "Recording a result" paragraph, the hand-over of five `docs/beta.md` lines to phase 28 and an empty appendix.

## The container rehearsal (D-08)

Docker was available (Docker Desktop 29.3.1, arm64 host). `PLATFORM=linux/amd64 TARGET=systemd-base NAME=holzcloud-t26-reh ... run.sh build up install firstrun postinstall` (15:57:27 to 15:58:02 CET) gave the stand-in server: production unit and backup timer enabled, one manual backup, an administrator account. Played there and quoted in the record as "Container rehearsal, not a result":

- **O-2** (15:00:21Z): backup, second unit from the installed unit with `sed`, `restore.sh` given `DATA_DIR` and `SERVICE`, copy ready on port 8081 with `"integrity":"ok"`, trees `equal`, clean-up. Observed and recorded in the text: `=== Restore complete ===` was followed by the **production** readiness JSON (`uptime_s` 149), the copy's own answer had `uptime_s` 0; with a terminal (via `script(1)`) the question reads `Stop holzcloud-restore-test and replace /srv/holzcloud-restore-test/data? [yes/NO]`; the lost-variable variant (15:01:52Z) printed `Stop holzcloud and replace /opt/holzcloud/data? [yes/NO]`, answered `no`, ended `aborted`, production stayed `active`, no `data.before-restore-*`.
- **O-3** (first draft 15:01:27Z, final text 15:12:37Z): `docker restart` for the reboot, PID 1's start time changed, both units active, `NRestarts=0`, site ready, timer armed. The first draft kept its output only on the terminal; the final text `tee`s into the owner's home. This was the only command block that needed correcting.
- Deviations named in the record: amd64 under emulation; the `ProcSubset=all` container drop-in on both units (not in the owner text); a restart for a reboot; `script(1)` for the question; fixture files; no website domain (`/` answers 404 on both ports); root's shell.
- The rehearsal container is removed (`run.sh down`), `docker ps -a` shows no `holzcloud-` container. The transcripts are gitignored scratch (`deploy/test/out/amd64-t26-reh/owner-o2.txt`, `owner-o3.txt`).

## Verification

- `bash deploy/test/native-amd64.sh selftest`: `SELFTEST OK (862 cases)` on macOS bash 3.2.57; also `SELFTEST OK (783 cases)` on Debian 12 (bash 5.2.15, mawk 1.3.4, no git, so the cases that need a real commit are skipped by the selftest itself).
- **Mutation checks** (19 rules switched off one at a time in a copy; each must fail the selftest): 18 killed on the first try; `recording-boundary` (the `**Recording a result` terminator of an Owner step) survived because its test put the marker inside the step instead of the paragraph; the test was repaired and the mutant is now killed. Mutants not committed.
- `bash deploy/test/nightly.sh selftest 2>&1` ends `SELFTEST OK (542 cases)`; `record-check` 29 blocks and `record-check-con` 27 blocks, **run with `E` pointing at the main checkout's `deploy/test/out`** because this worktree has none of the phase 24 and 25 scratch files (without it both end `FAILED`, 29 and 27 problems).
- `record-check` ends `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03`; the five `docs/beta.md` lines are quoted byte for byte; the two `POINT` lines are quoted byte for byte; `git diff --quiet` against `05fc076` is clean for `backup.sh`, `restore.sh`, `DEPLOY.md`, the unit, both backup units, `run.sh`, `nightly.sh`, `ci.yml` and the five earlier records, and against `f1ba5a2` for `docs/beta.md`.
- Wrong on purpose and refused (all in the selftest): a forged `SRV-01 PASS` over an aarch64 host, over `MDWE-WORD: VOID`, after `BAK-02 PASS` was removed; a pending row with PASS; `SRV-02 PASS` as a prose line; a recorded status with a missing marker, with the commands instead of their output, with an unknown or non-prefix evidence sha, with an appendix block that differs, is off the list or lacks the native summary; recorded SRV-01 over a VOID, FAIL, NOT-NATIVE, NOT-RUN or not-played-timing summary.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A selftest case that did not test its rule, found by mutation**
- **Found during:** Task 1, mutation check `recording-boundary`
- **Issue:** the case "a marker in the Recording paragraph does not count for O-3" removed the marker from O-3 and inserted it before the Recording line, i.e. still inside the step; with the terminator removed from `rc_step` the selftest stayed green.
- **Fix:** the marker is now inserted after the Recording line, plus a fixture assertion that it is there; the mutant is killed.
- **Files modified:** `deploy/test/native-amd64.sh`
- **Commit:** `a91c911`

### Plan deviations and observations (no rule applied)

- **`status-check` fails in this tree, as the plan expects.** `.planning/REQUIREMENTS.md` still reads `Pending` in the three traceability cells; the pending-owner form needs `Pending owner`. That edit is plan 26-03's (and is outside this plan's rules: REQUIREMENTS.md is not touched here). The selftest holds `status-check` on fixtures; the real file is not checked until 26-03.
- **The plan's Task 1 verify lines for `nightly.sh` cannot pass as written in a worktree**: its `record-check` and `record-check-con` need the scratch files of phases 24 and 25, which exist only in the main checkout (`E=/Users/holz/Projects/holzcloud-cms/deploy/test/out`), and the selftest count is on stderr (`2>&1`), as 26-01 already noted. 26-03's sweep should do the same.
- **Markers must begin a line (stricter than "occur in the block")**, so that an echoed command is not taken for its output. The Owner steps print every marker at the start of a line (verified in the rehearsal transcripts). Stated in the record's O-2 and O-3 Paste lines.
- **The appendix may be a subset of the fixed list** (the plan says "only paths of the fixed list, includes native-summary"); nothing forces all thirteen. `appendix` prints all thirteen, so the normal record holds all thirteen.
- **The `SRV-01 PASS` rule outside a fence is the whole line.** O-1's Expect text has to name `SRV-01 PASS` (the marker must occur in the step), so a substring rule would refuse the record's own owner step; inline prose, a fenced block and the line inside an Owner step all pass, a line consisting of `SRV-0N PASS` (also after a list bullet) does not.
- **The Bash tool's worktree guard refused every command text that was not plain**, so the rehearsal, the mutation checks, the Linux selftest, the commits and the verifies ran through wrapper scripts in the scratchpad, as the brief allowed. The base commit was not an ancestor of HEAD (`ee3c9ed`, tree clean), so `git reset --hard 8359caa` was run first, as instructed.

## Known limits (stated, not hidden)

- Nothing here has run on amd64 hardware, on GitHub or on a real server. The expected output of O-2 and O-3 is from an emulated container on an arm64 host; a restart is not a reboot; the second unit is not a spare machine.
- O-1's expected shape is derived from the script and the arm64 rehearsal, labelled so; the first real `runner.txt` and timing readings come from the owner's run.
- The record-check can prove that the pasted text is internally consistent and equal to the scratch files; it cannot prove that the owner ran the commands. The line-start rule makes an unexecuted `echo` insufficient, nothing more.
- `.planning/WINDOWS.md` was not appended: the three empty `Result (owner)` blocks are the intended pending-owner state, tracked by the record and by `status-check`, not defects of this plan.

## Threat model coverage

T-26-06 (recorded without evidence, pass word without evidence: refused by `record-check`, each refusal in the selftest, a forged summary caught by recomputation), T-26-07 (the commands print no password, key or token; the throwaway unit has its `EnvironmentFile=`, payment, mail and secret-key lines removed, with a `grep` for the owner to read; the record says what is fine to paste; a private-key marker in the record is refused), T-26-08 (`sudo env DATA_DIR=... SERVICE=...`, the question to read, `no` on a lost variable rehearsed, production `active` and no `data.before-restore-*` after), T-26-10 (the path is created mode 700 for the service user, the clean-up removes the copy and the unit and shows the path gone) and T-26-SC (no package, no action added) are mitigated as planned. T-26-09 accepted as planned (the owner chooses the time; the step names the console route). No new security surface beyond the plan's register.

## Known Stubs

None in code. The three `(nothing pasted yet)` blocks, the empty "Native (pending owner)" cells and `Evidence commit: none (pending owner)` in `deploy/NATIVE-AMD64-RUN.md` are the intended pending-owner form (D-05, D-06); the owner's results resolve them.

## Self-Check: PASSED

- FOUND: `deploy/NATIVE-AMD64-RUN.md`, `deploy/test/native-amd64.sh`
- FOUND commits: `a91c911`, `1a791b1`, `60d02b1` (all ancestors of HEAD; three commits counted from `plan_head_before`)
- Frozen files unchanged since `05fc076`, `docs/beta.md` since `f1ba5a2`; working tree clean; no container left
