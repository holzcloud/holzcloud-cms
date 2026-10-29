---
phase: 29-sicherung-robust
plan: 05
subsystem: deploy
tags: [backup, record, evidence, appendix, record-check-con, owner-steps, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: plan 29-04 evidence set t29 against evidence commit dd077e4 (scratch directories amd64-t29-src, -tgt, -old, -new)
provides:
  - deploy/BACKUP-ROBUST-RUN.md, the permanent record of the set, with Evidence commit dd077e4 and an appendix of 30 blocks proved byte for byte
  - findings F-1 to F-9, whole-sentence Not tested bullets and two one-line owner steps for plans 29-06 and 29-07
affects: [29-06, 29-07, 29-08]
tech-stack:
  added: []
  patterns: [appendix assembled by a throwaway program from the scratch files, outside the repository, which refuses what the extractor of record_check would mis-read; claims written as whole sentences after a title, a bullet marker or a full stop]
key-files:
  created:
    - deploy/BACKUP-ROBUST-RUN.md
  modified: []
key-decisions:
  - "Tasks 2 and 3 were written and committed together, as one commit, because they edit the same file and the body was written in one pass after the tracer."
  - "ROB-03 is ticked: the record and its proof are this plan's output and no later plan produces evidence for it."
requirements-completed: [ROB-03]
status: complete
commits: 2
plan_head_before: 110f54e55f15fb069c556c705c3e97d85bf02a9b
plan_head_after: ce290b32df61bce73a6522a55650d3d26c056507
duration: about 1 h
completed: 2026-10-29
actuals:
  tokens: 52000
  tasks: 3
  commits: 2
---

# Phase 29 Plan 05: The record of the evidence set Summary

**`deploy/BACKUP-ROBUST-RUN.md` records the set t29 against evidence commit `dd077e4`, and `nightly.sh record-check-con` with the four t29 names ends `RECORD-CHECK OK 30 blocks`.**

## What was built

- **Evidence commit:** `dd077e4`, one line `Evidence commit: dd077e4` in the record, equal to `deploy/test/out/amd64-t29-evidence-commit.txt`. `git diff --quiet dd077e4 HEAD -- <the seven bound files>` exited 0 when the record was written and again before its commit.
- **Block count:** 30 (final). The t25 defaults still end `RECORD-CHECK OK 27 blocks` and the phase 24 `record-check` ends `RECORD-CHECK OK 29 blocks`; none of the earlier records was touched (`git diff --stat HEAD` over the six named records is empty).
- **Record:** headings Summary, How this was run, Deviations named, Findings, CON-01, CON-02, CON-03, ROB-01, ROB-02, Measurements, Container vs. server, Not tested (13 whole-sentence bullets), Owner steps (2 numbered items, one line each), Hand-over to docs/beta.md, Appendix: raw output.
- **Assembler:** a throwaway Python program in a directory made with `mktemp` under the session scratchpad, not committed. It cuts the record at the appendix heading, takes the `amd64-t29-(src|tgt|old|new)/FILE` tokens of the body plus a fixed list of 30 files, and refuses (exit 1, file and line) a line beginning with three backticks, one beginning `#### `, one equal to the appendix heading, and a path outside `amd64-t29-*`. All three refusals were exercised on a copy before use. No glob is used.

## Findings of the record (classes)

| Finding | Class | Subject |
|---------|-------|---------|
| F-1 | 3 | A night whose first copy ended with status 24 and whose repeat succeeded completed with a MANIFEST, by hand and under the unit (`Result=success`) |
| F-2 | 3 | A recurring status 24 ended the night with status 24, left no directory, sentinel unchanged; unit `Result=exit-code`, `ExecMainStatus=24` |
| F-3 | 3 | On a 1064 MB filesystem the night failed with rsync status 11 and left no directory, by hand and under the unit |
| F-4 | 3 | The next night passed the guard and failed the same way without a directory; `GUARD-REFUSED` no longer occurs |
| F-5 | 3 | 8 of 40 nights of stage deletes met a real status 24, repeated and completed; 0 ended with status 24; 0 directories without a MANIFEST; all 8 restored with 3 rows without a file |
| F-6 | 3 | Class counts of both stages (16/24/0 and 10/14/16); a restore still does not report a row without a file |
| F-7 | 3 | The CON-02 backup was made by the tag's `backup.sh`, which is not the shipped one |
| F-8 | 3 | Window names: 238 `overlaps-snapshot-step`, 28 `after-snapshot`, 3 `straddles-media-end`, none unresolved |
| F-9 | 2 | The status 24 of the four VANISH scenarios is delivered by a stand-in for rsync |

No finding of class 1 or 4; no shipped file edited.

## Where the set showed something other than the plan expected

1. **F-7, new:** the plan and the earlier record assume `backup.sh`, `restore.sh` and both units are byte-identical at the tag and now. This set shows `SCRIPTS-IDENTICAL: deploy/backup.sh no`; `restore.sh` and both units are identical. The harness installs the tag's own `backup.sh` before the CON-02 backup (`BACKUP-SH: replaced`). The record says so in the CON-02 summary row, F-7 and the CON-02 "What it shows" sentence, and it answers the promise row P-17 in the hand-over.
2. **F-3 wording:** the plan's sentence reads "on a filesystem of the recorded size under the floor of 1024 MB". The set shows 1064 MB, which is above the floor and below the 1225 MB a backup writes. The record states that, not "under the floor" (the Summary row keeps the earlier record's phrase "under a floor of 1024 MB", meaning the floor setting).
3. **F-5, natural status 24 did occur:** 8 of 40 nights of stage deletes met a real one and all repeated and completed. The plan's alternative sentence (no natural status 24 in 80 runs) was therefore not needed. All 8 repeated nights restored as ROW-WITHOUT-FILE, which the record states.
4. **F-8, new:** two window names that the earlier record did not list occur (28 `after-snapshot`, 3 `straddles-media-end`). The page row P-16 names only `overlaps-snapshot-step` and the hand-over says so.
5. **Counts:** `RESTORED-RUNS-STAGE-deletes` is 40 here (no backup was refused), against 32 in the earlier set; `WALL-MINUTES` is 32.
6. **Tasks 2 and 3 in one commit** (see Deviations).

## Acceptance checks

- `grep -c '^Evidence commit: '` prints 1; `grep -c '^## Hand-over to docs/beta.md$'` prints 1; `grep -c 'Hand-over to phase 28'` prints 0; `grep -c 'PRIVATE KEY\|TODO'` prints 0.
- `grep -c '^\*\*F-[0-9]* (class [1-4])'` prints 9; `grep -c '^## ROB-0[12]'` prints 2; `grep -c 'What it shows'` prints 6.
- Owner steps: 2 numbered one-line items. Not tested: 13 bullets, all ending with a full stop, `SIGKILL` occurs twice.
- Three numbers of F-1 to F-6 checked by `grep -F` against the scratch files: `1284455154` (F-1, `amd64-t29-src/capfull.txt`), `RETRIED-RUNS-STAGE-deletes: 8` (F-5, `amd64-t29-src/consist.txt`), `FREE-AT-GUARD-MB: 1064` (F-3, `amd64-t29-src/capfull.txt`). In addition a script compared every backticked `KEY: value` / `KEY=value` item of the body with the corpus of the four directories: all found except two intended negatives (`CLASSIFY: none`, and the knob `TARGET=systemd-base` of the command line); every backticked key was found except the strings stated not to occur (`FALSE-COMPLETE`, `GUARD-REFUSED`, the rule-of-three keys).
- T-29-17: the appendix was read for `password`, `secret` and `token`; the only hits are prose in `DEVIATION[...]` lines (the word "token" for the CSRF token of each form, "password file" for a fixture); no key, secret or private key text.
- At the time of writing the body: `BACKUP-ROBUST OK (94 cases)` and `SELFTEST OK (646 cases)` (re-run, quoted in the record, not pasted).
- Containers: `docker ps -a | grep -c holzcloud-t29` prints 0. They were already removed by plan 29-04, so the `down down-con` step of Task 3 was a no-op and was not run.

## Deviations from Plan

**1. [Judgment] Tasks 2 and 3 committed together.** The body (Task 2) and the limits, owner steps and hand-over (Task 3) were written in one pass and edit one file, so one commit `ce290b3` carries both; the tracer is its own commit `ac26bef`. Both verify blocks pass on the final state.

**2. [Judgment] `git status --porcelain` is not limited to the record and the SUMMARY.** `.planning/config.json` (modified) and `.planning/milestone.lock` (untracked) belong to the orchestrator and were not committed, as instructed.

**3. [Judgment] Containers not removed by this plan.** See above: already gone.

No auto-fixes, no shipped file edited, no earlier record edited.

## Known Stubs

None.

## Threat Flags

None. The record adds no network endpoint, auth path or file access; T-29-16 (blocks differ from files) is mitigated by the assembler and `record-check-con`; T-29-18 (claims beyond evidence) by the key and value comparison above and by the *Not tested* bullets.

## Self-Check: PASSED

- FOUND: deploy/BACKUP-ROBUST-RUN.md
- FOUND commit ac26bef and ce290b3 (ancestors of HEAD)
- `RECORD-CHECK OK 30 blocks` (t29), `RECORD-CHECK OK 27 blocks` (t25 defaults), `RECORD-CHECK OK 29 blocks` (phase 24)
