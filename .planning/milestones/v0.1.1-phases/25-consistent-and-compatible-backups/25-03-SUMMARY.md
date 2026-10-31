---
phase: 25-consistent-and-compatible-backups
plan: 03
subsystem: deploy/test (the evidence set of CON-01, CON-02 and CON-03 in one command, and the record that proves it)
tags: [backup, restore, consistency, compatibility, disk-full, rsync, systemd, docker, verification, record]
status: complete

requires:
  - phase: 25-01
    provides: prep, consist, window and the verdict helpers for CON-01
  - phase: 25-02
    provides: compat-source, compat, capfull, down-con
provides:
  - nightly.sh con-chain (the whole set from fresh containers, stage statuses kept, verdicts computed from the files), fut02_verdict, owner_steps_ok, record-check-con, a parametrised record_check
  - deploy/BACKUP-CONSISTENCY-RUN.md (the record, 27 appendix blocks, proved by record-check-con)
  - the Phase 25 section of deploy/test/README.md
  - one evidence set at e86d490 (CON-01 PASS, CON-02 PASS, CON-03 PASS, 96 steps played and 0 failed, LEAK-CHECK OK)
affects: [28]

tech-stack:
  added: []
  patterns:
    - "a trace cannot pass for the set: CON-01 needs MIN-RUNS 30, runs per stage at least that, and restored runs per stage at least max(10, ceil(MIN_RUNS / 2))"
    - "the summary is a function (con_summarize) that selftest plays under set -e on a fixture"
    - "the appendix is assembled by a script over a fixed list of files and refuses a file that would break it"

key-files:
  created: [deploy/BACKUP-CONSISTENCY-RUN.md]
  modified: [deploy/test/nightly.sh, deploy/test/README.md]

key-decisions:
  - "All seven CLASSIFY lines are class 3 (a measured property that matches what backup.sh and DEPLOY.md already say, or a gap in them); no class 1 or class 4 finding; no shipped file was edited, so phase 24's evidence commit b501b1b still binds backup.sh"
  - "A backup refused by rsync status 24 is left as it is: tolerating 24 would turn a loud refusal into a silent row without a file, a design decision for the owner and not a repair"
  - "A partial directory left by a full disk, and the following nights refused by the guard, are recorded with the owner-facing consequence and a documentation gap for phase 28, not fixed: every fix (cleanup trap, retention before the copy) changes what a backup does"
  - "fut02_verdict counts REFUSED-NAMED as named (the plan listed three outcomes; a refused backup is named by its rsync status)"

requirements-completed: [CON-01, CON-02, CON-03]

metrics:
  duration: "not measured as one span (the session was compacted once); the evidence run took 35 min 34 s, 2026-10-26 13:18:54Z to 13:54:28Z"
  completed: 2026-10-26

actuals:
  tokens: 25400
  tasks: 3
  commits: 3
plan_head_before: 6542d537000be355dc5ae4e311701c4a964df981
plan_head_after: c4fb8cda4daaefb92171a4027a6bad9b1ef3fd60
---

# Phase 25 Plan 03: The evidence set and the record Summary

One `nightly.sh con-chain` run from fresh containers (pinned to `e86d490`) gave CON-01 PASS, CON-02 PASS and CON-03 PASS, and `deploy/BACKUP-CONSISTENCY-RUN.md` records them without calling backups consistent: under load the database and the media disagree in both directions, and a full disk leaves a partial directory.

## What was done

**Task 1 (`9bc4fbc`, `e86d490`): the chain and the proof.** `con-chain` refuses unless `TARGET=systemd-base`, the four container names and four directories are distinct and safe, no container exists and the directories are empty; it pins `NEW_REF`, plays `source`, `target`, `prep`, `consist`, `window`, `capfull`, `compat-source` and `compat` as child processes, writes `chain-statuses.txt`, runs the leak check over both pairs and writes the verdicts. `fut02_verdict`, `owner_steps_ok` and `record-check-con` were added, and `record_check` takes `RC_REQ_PREFIX` and `RC_REQ_IDS` (3 lines changed, defaults reproduce the old behaviour). Selftest: 542 cases (442 before this plan), a mutation check of three deliberate breakages made it fail 6 cases. The second commit moved the summary into `con_summarize` after playing it on the real trace files found a silent exit (see Deviations).

**Task 2: the evidence set.** One pass, no bound file changed after the evidence commit `e86d490`. Raw numbers (`deploy/test/out/amd64-t25-src/summary.txt`, `consist.txt`):

- Stage adds: 40 runs, 40 restored, AGREE 12, ORPHAN-ONLY 28, ROW-WITHOUT-FILE 0, BACKUP-REFUSED 0, exposed 35, uploads inside the window 84, page saves 24, backup status histogram `0=40`, outcome ORPHAN-ONLY-NAMED.
- Stage deletes: 40 runs, 32 restored, AGREE 8, ORPHAN-ONLY 13, ROW-WITHOUT-FILE 11, BACKUP-REFUSED 8 (rsync 24), exposed 39, uploads 96, page saves 27, deletes 30, histogram `0=32 24=8`, outcome ROW-WITHOUT-FILE-NAMED.
- 220 `WINDOW-NAME` lines, all `overlaps-snapshot-step`, none unresolved. Controls: C0 AGREE, C1 ORPHAN-ONLY, C2 ROW-WITHOUT-FILE (restore exit 0, `holzcloud check` clean, the file answers 404).
- FUT-02: not needed.
- CON-02: `v0.1.0` backup restored by `v0.1.0-46-ge86d490`: 8 pages, 2 users, 12 media lines, logins and 4 of 4 media served equal, 17 of 17 checks; scripts and units byte-identical at the tag; migration level 60 to 60, none pending.
- CON-03: filesystem 1064 MB, floor 1024, data 1222 MB: guard passed, exit 11 (by hand and by the shipped unit), 1098915817 byte directory without `MANIFEST`, next run refused by the guard (5 MB free), `restore.sh` refused the left directory; control on a roomy tmpfs completed.

**Task 3 (`c4fb8cd`): the record and the README.** `deploy/BACKUP-CONSISTENCY-RUN.md` follows the phase 24 record's headings (Summary with one `Evidence commit: e86d490` line, How this was run, Deviations named, Findings F-1 to F-7, CON-01 to CON-03, Measurements, Container vs. server, Not tested, Owner steps, Hand-over to phase 28, Appendix with 27 blocks). `deploy/test/README.md` has the Phase 25 section and a pointer from the Phase 24 bullet. Proofs: `record-check-con` OK 27 blocks; phase 24 `record-check` OK 29 blocks (run with `E=` the main checkout's `deploy/test/out`, the worktree having no scratch directories); selftest OK (542 cases); `git diff --quiet e86d490 HEAD -- <bound files>` exits 0; `git diff --quiet f1ba5a2 -- docs/beta.md` exits 0; `git diff -U0 b501b1b -- deploy/test/nightly.sh | grep -c '^-[^-]'` is 3.

## Classification of every CLASSIFY line

All seven are class 3. Reasoning, as written in the record: `backup.sh` (header comment) and `DEPLOY.md` already say the pair is not atomic in both directions (a file added in between is harmless, a file deleted in between can leave a row whose file the backup lacks), that a directory without a `MANIFEST` is an interrupted backup which `restore.sh` refuses, and that a backup can run out of space halfway. Every observed result matches a sentence; nothing contradicts one, and the earlier backups were intact in every run.

| CLASSIFY line | Finding | Class | Note |
|---|---|---|---|
| adds ORPHAN-ONLY (28 runs) | F-1 | 3 | the harmless direction |
| deletes ORPHAN-ONLY (13 runs) | F-1 | 3 | same |
| deletes ROW-WITHOUT-FILE (11 runs) | F-2 | 3 | harmful direction; restores silently, only symptom is a 404 on the file; documentation gap for phase 28 |
| deletes BACKUP-REFUSED (8 runs) | F-3 | 3 | fails closed; tolerating rsync 24 is a design decision, left to the owner |
| capfull HAND-CLASS FAILED-PARTIAL-DIR-LEFT | F-4 | 3 | exit 11, partial directory, `restore.sh` refuses it |
| capfull UNIT-CLASS FAILED-PARTIAL-DIR-LEFT | F-4 | 3 | same by the shipped unit |
| capfull AGAIN-CLASS GUARD-REFUSED | F-5 | 3 | the left directory holds the space; the following nights fail on the guard until it is removed; undocumented, handed to phase 28 |

The pre-measured findings: (a) is F-1 to F-3, (b) is F-4 and F-5, (c) the restore start limit is F-6 (class 3 for the unit, class 2 for the harness, `DEVIATION[start-limit]`), (d) is F-7. No fix was made and none needed a sandbox loosening, so no `checkpoint:decision` was returned; the owner-facing alternatives (status 24 ends the night or completes it, a failed night removes its directory, retention before the guard and the copy) are listed in the record's Hand-over.

## Deviations from Plan

**1. [Rule 1 - Bug] The summary of `con-chain` ended the chain silently.** Found while playing `con_summarize` on the files of the earlier traces: `kinds=$(... sed ... "$d/deviations.txt")` failed under `set -e` and `pipefail` when a directory (the fresh box, the second host) held no `deviations.txt`. Fixed with `|| true` inside and `|| kinds=""` outside, the summary moved into `con_summarize` with 18 more selftest cases run under `$(set -e; ...)`. Committed as `e86d490` before the evidence run, so the set was made against the fixed text.

**2. [Plan wording] `fut02_verdict` also counts REFUSED-NAMED as named.** The plan listed AGREE-ALL, ORPHAN-ONLY-NAMED and ROW-WITHOUT-FILE-NAMED; a refused backup is named by its rsync status, so it is added to the NOT-NEEDED set. The outcome of this set does not depend on it (adds ORPHAN-ONLY-NAMED, deletes ROW-WITHOUT-FILE-NAMED).

**3. [Rule 3 - Blocking] The block of new code was moved so the diff against `b501b1b` stays at 3 removed lines.** Placed between `cmd_capfull` and the `# down, chain and record-check` banner; the first placement made the default diff report ~107 removed lines (an alignment artifact).

**4. [Environment] The branch was behind the base.** `6542d53` was not an ancestor of HEAD and the tree was clean, so `git reset --hard 6542d53` was run as the orchestrator instructed.

**5. [Environment] Phase 24 `record-check` needs `E=<main checkout>/deploy/test/out`** in the worktree, which has no scratch directories of phase 24.

**6. [Appendix] Not every per-run file is pasted.** The appendix holds the three files of run 1 (adds) and run 3 (deletes) and the backup transcript of run 9 (the first refused backup); the other 77 runs are in `consist-runs.txt` (one line each) and the 220 `WINDOW-NAME` lines of `consist.txt`. The record says so in the appendix introduction.

**7. [Housekeeping] A mistyped scratchpad path** left a stray placeholder file outside the repository; it was removed. (A similar stray file from plan 02 still exists outside the repository, harmless.)

Not done: the unit's result for a status 24 refusal and a restore across a migration were not played (both are on the record's Not tested list). Corrections made to the record before the commit, after checking it against the evidence files: the F-1 quote (`ORPHAN: 5`, two uploads), the BAD-line statement (`compare-selftest.txt` holds BAD lines on purpose), the architecture keys, the mkdir reading for status 24, and the start-limit wording.

## Known Stubs

None.

## Threat Flags

None. No shipped file changed; the harness adds no network endpoint or auth path to the product. The held-binary stand-in used by `window` is root-owned, container-only and removed with the container.

## Self-Check: PASSED

- `deploy/BACKUP-CONSISTENCY-RUN.md`, `deploy/test/README.md`, `deploy/test/nightly.sh` exist and are committed.
- Commits `9bc4fbc`, `e86d490`, `c4fb8cd` are ancestors of HEAD; `commits: 3` measured with `git rev-list --count 6542d53..HEAD` before this SUMMARY was committed.
- `record-check-con` OK 27 blocks; `record-check` OK 29 blocks; selftest OK (542 cases); bound files unchanged since `e86d490`; `docs/beta.md` unchanged.
- The four evidence containers were removed (`docker ps -a` shows no `t25` name).
