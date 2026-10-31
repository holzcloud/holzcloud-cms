---
phase: 28-the-promise-in-writing
plan: 03
subsystem: docs
tags: [beta-page, promises, limits, ledger, evidence]
requires:
  - phase: 28-the-promise-in-writing
    provides: P-11, the tracer ledger row, 28-OWNER-STEPS.md (OS-5 to OS-14), the 0.1.1 CHANGELOG entry
provides:
  - docs/beta.md as the complete 0.1.1 page: promises P-1 to P-18, 43 limits each with a reason, a ledger of nine rows, evidence E-1 to E-5 with five commands that exit 0
affects: []
tech-stack:
  added: []
  patterns: [one whole sentence of a record per promise row, found by grep -F; a moved limit is the first cell of a ledger row; a limit that stays keeps its baseline sentence and gains Why]
key-files:
  created:
    - .planning/phases/28-the-promise-in-writing/28-03-SUMMARY.md
  modified:
    - docs/beta.md
key-decisions:
  - "SRV-01, SRV-02 and SRV-03 stay under does-not-promise, five lines in all, each reading Pending owner with Record deploy/NATIVE-AMD64-RUN.md and OS-5, OS-13 or OS-14; no promise row cites the native record."
  - "The 0.1.0 line 'No native amd64 run was made and none is planned (a user decision)' moves to the ledger as none (reworded) (D-13); a new limit says no run was made yet and one is planned as a button."
  - "The three gaps of deploy/DEPLOY.md are stated as one limit of this page (D-04); the document is not edited."
requirements-completed: [DOC-01]
status: complete
commits: 3
plan_head_before: 497b576456b8297ec635984def6d6de7b18ef16d
plan_head_after: 0f6decc8d7b7b587ee83ecae80a0ad4130c713ea
duration: 45min
completed: 2026-10-26
actuals:
  tokens: 14000
  tasks: 3
  commits: 3
---

# Phase 28 Plan 03: The page that keeps the promise Summary

**docs/beta.md now promises P-1 to P-18, each new row quoting a whole sentence of its record that 28-check.sh finds byte for byte, states 43 limits each with its reason, keeps SRV-01 to SRV-03 pending the owner, and shows five git commands that exit 0.**

## Base

The worktree HEAD was ee3c9ed with a clean tree; `main` read 497b576. The tree was clean, so it was reset to 497b576 with `git reset --hard` as instructed. Remote heads before and after (`git ls-remote --heads origin`): `main` ee3c9ed, `release` 4d238b9, `fix/bausteine-befunde` fa696f6. Nothing was pushed, tagged, run on GitHub or set as a version.

## Baselines (recorded before each task)

- **Task 1:** after plan 28-01, `beta structure`, `beta baseline`, `beta ledger` and `beta quotes` exited 0 (11 promise rows, 1 ledger row) and `beta coverage` exited 1 (BKP-02 to CON-03 and the words window and `overlaps-snapshot-step` missing), as predicted.
- **Task 2:** `beta evidence` exited 1 (fifteen FAIL lines: no E-4, no E-5, no commands, no PR 47, no `bausteine.css`, no 39 added lines). The five commands of `EVIDENCE_CMDS` run by hand exited 0, 0, 0, 0, 0. The 0.1.0 command (bf74496, with `deploy/DEPLOY.md`) exited 1, and `git diff --numstat bf74496 HEAD -- deploy/DEPLOY.md` printed `39	0`.
- **Task 3:** `beta nonpromise` failed (23 limits without Why; the words status 24, MANIFEST and 404 missing) and `beta srv` failed (no `Pending owner` line for SRV-01, SRV-02 or SRV-03).

## What was done

- **Task 1 (ffe899a).** P-12 to P-18 (BKP-02 to BKP-05, CON-01 to CON-03), each Promise cell opening with the requirement name, each Cited words cell holding whole sentences in straight double quotes, selected from the records by a program (`sent.py` printed each sentence from its first words to its full stop and whether it begins at a sentence start). Seven baseline limits left the bullets and became first cells of ledger rows P-12 to P-18 (ledger now 8 rows, none says none). In their place 10 narrower single-line limits: real power cycle (OS-7), uniformity of the delay (no owner step), real network (OS-8), pull of the copy back (OS-9), retention on elapsed time (OS-10), the backup is not atomic, a media tree of production size, real traffic (none of the three has an owner step), a restore across a migration (two quoted sentences, no owner step), a real nearly full disk (OS-11). P-16 uses the words window and `overlaps-snapshot-step`, says the pair is still not atomic and promises the naming of the window and not the agreement of the pair. P-1 to P-11 were not touched.
- **Task 2 (839bef7).** E-4 (P-11 to P-15, b501b1b, `deploy/BACKUP-TIMER-RUN.md`) and E-5 (P-16 to P-18, e86d490, `deploy/BACKUP-CONSISTENCY-RUN.md`); one sentence added to the P-4/P-5 paragraph on the 39 added lines of `deploy/DEPLOY.md` (phase 24, F-2); the rule paragraph rewritten (the rule is kept in its words, the 0.1.0 command is named as no longer exiting 0, the sentence "exits 0 for 0.1.0" is gone); a fenced text block of exactly the five commands of `EVIDENCE_CMDS`; sentences on what the first three and the last two show and on PR 47 (`bausteine.css`, weide `style.css`, not measured by any record, not read by a backup). E-1 to E-3 and the introductory sentence are byte for byte as before.
- **Task 3 (0f6decc).** The 22 kept limits carry their baseline sentence verbatim plus ` Why: ...`; the five tied to the native record read `Pending owner: SRV-01` (native timing, both seccomp lines, OS-5), `SRV-02` (restore on a real server, OS-13) and `SRV-03` (reboot, OS-14) with `Record: deploy/NATIVE-AMD64-RUN.md "Status: pending owner"`. The 0.1.0 "none is planned" line is the ninth ledger row (`none (reworded)`); the new limit says no run was made yet and one is planned as the workflow `native-amd64`. Part C limits: a restore does not report a missing-file row (404), a vanished file ends the night (`rsync` status 24, no `MANIFEST`, unit result not played), a failed night leaves its directory and the guard refuses the following nights (OS-12; the skipped retention is marked as a reading of the script), the three gaps of `deploy/DEPLOY.md` stated here and not edited there, what a fix would change left to the owner, a key in the service user's home does not work (OS-8), the off-site copy is a mirror and a failed copy leaves the new backup, a failed removal in retention is silent (OS-10), the sixth restore within five minutes meets the start limit (in the unit group). The introduction of the does-not-promise section says every limit carries its reason and that `Pending owner` lines wait for the owner.

## Counts

Promise rows: 18 (P-1 to P-18, no gap). Limits: 43 (22 kept baseline limits, 21 new limits). Ledger rows: 9 (eight with P-11 to P-18 in order, one `none (reworded)`). Kept 22 + moved 9 = 31 baseline limits, none dropped, none both (`beta baseline`). Evidence rows: 5.

## Verification

Run in this worktree after Task 3 (commit 0f6decc, tree clean):

- `28-check.sh beta` (structure, baseline, ledger, quotes, coverage, nonpromise, srv, evidence): exit 0, every line ok.
- `28-check.sh owner-steps`: exit 0 (19 steps). `28-check.sh changelog` (extract 7715 bytes, topics, go): exit 0. `28-check.sh frozen`: exit 0. `28-check.sh selftest`: `SELFTEST OK (59 cases)`.
- The regression components, run by hand because `28-check.sh regression` cannot pass in a worktree (see the deviation below), with `E=/Users/holz/Projects/holzcloud-cms/deploy/test/out`: `nightly.sh selftest` `SELFTEST OK (542 cases)` (read through `2>&1`), `nightly.sh record-check` `RECORD-CHECK OK 29 blocks`, `nightly.sh record-check-con` `RECORD-CHECK OK 27 blocks`, `native-amd64.sh status-check` `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`, `native-amd64.sh record-check` `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03`, `native-amd64.sh selftest` `SELFTEST OK (862 cases)`.
- `28-check.sh all` as one command was NOT run here. The orchestrator re-runs it in the main checkout, where `deploy/test/out` exists.
- The CHANGELOG paragraph "Was die Dokumente noch nicht sagen", which says the three DEPLOY.md gaps are stated on `docs/beta.md`, is now true: the limit "Three gaps of `deploy/DEPLOY.md` are stated on this page and not in the document" names all three, and `beta nonpromise` requires the words atomic, status 24, MANIFEST, 404 and DEPLOY.md.
- Remote heads after: identical to before. `git diff 497b576 HEAD` touches `docs/beta.md` only (plus this summary).

## Deviations from Plan

### Environment, reported, not fixed

**1. `28-check.sh regression` fails in an agent worktree.** Same as plan 28-02: the gitignored `deploy/test/out` exists only in the main checkout. The components were run by hand with `E` set (above) and all pass; no checker, program or record is wrong. `28-check.sh all` is for the orchestrator to run in the main checkout.

### Auto-fixed Issues

None. The plan was executed as written. Two wording choices outside the plan's text: the Why of the Debian-12-on-real-hardware limit says every record of this milestone ran in a container (an earlier draft said "or on a hosted runner", removed because SRV-01 has no result yet); the ledger and limit wording for "Whether the random delay is drawn uniformly" names no owner step because none answers it.

## Auth gates

None. Only read-only `git ls-remote` was run against the remote.

## Known Stubs

None. A scan of `docs/beta.md` for `TODO`, `FIXME`, `coming soon` and `placeholder` finds nothing (`beta structure` checks `TODO`).

## Threat Flags

None. The change is prose in `docs/beta.md`; no endpoint, auth path, file access or schema. The threat register was applied: T-28-11 by `beta baseline` and `beta ledger`, T-28-12 by `beta quotes` and `beta coverage`, T-28-13 by `beta evidence` running the five commands, T-28-14 by `beta srv`, T-28-15 by `beta nonpromise`.

## Points for the owner

- A quote can only prove that a sentence was said; the Promise cells of P-12 to P-18 were written to stay inside their sentence. Two places go slightly beyond the first sentence and rest on the record elsewhere: P-14 says the other host was a second container (record deviation `offsite-host`), P-18 gives 1064 MB and 1024 MB (record F-4 and the same line of CON-03).
- The limit "A failed night leaves its directory ... the retention step does not run on such nights" says in its Why that retention being skipped is a reading of the order of the script; no run showed it.
- `deploy/DEPLOY.md` is unedited (D-04). Editing it to say the three gaps would force the evidence of b501b1b and e86d490 to be made again; that is the owner's decision.

## Self-Check: PASSED

- `docs/beta.md` and this summary exist; commits ffe899a, 839bef7 and 0f6decc are ancestors of HEAD.
