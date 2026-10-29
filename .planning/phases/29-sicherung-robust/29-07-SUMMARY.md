---
phase: 29-sicherung-robust
plan: 07
subsystem: planning
tags: [beta-page, promises, limits, evidence, checker, selftest, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: record deploy/BACKUP-ROBUST-RUN.md (evidence commit dd077e4), OS-20 and OS-21 and ROB_REC in 28-check.sh (plan 29-06)
provides:
  - docs/beta.md promises P-16 to P-22 from the new record, byte for byte, with whole sentences
  - five limits rewritten and two added, each with Why, one Record, quoted sentences and an owner step
  - evidence row E-6 and seven commands that exit 0, the added and removed lines of backup.sh against e86d490
  - 28-check.sh with E29, the proof that E29 equals the record's Evidence commit line, ROB-01 and ROB-02 in coverage
affects: [29-08]
tech-stack:
  added: []
  patterns: [a literal commit in the checker proven equal to the record's own line; a break-and-restore run per new selftest rule]
key-files:
  created: []
  modified:
    - docs/beta.md
    - .planning/phases/28-the-promise-in-writing/28-check.sh
key-decisions:
  - "Title sentences of F-1 to F-4 and F-8 cannot be quoted: they stand inside bold markers (the sentence boundary of sentence_ok wants a full stop, '- ' or '** ' before and a space or the line end after), so the rows quote the sentences after 'What it shows.', the bullets of Not tested and the body sentences instead."
  - "CHECK_E29 (default the literal dd077e4) lets the selftest show the E29-equals-record proof fail; any other value can only make the check fail."
  - "ROB-04 and ROB-05 are not ticked: plan 29-08 owns 'Complete' for ROB-01 to ROB-05 after the gate is green."
requirements-completed: []
status: complete
commits: 3
plan_head_before: 35c4abc5c38e9d5ea198b6a029d11cc86c2e44bd
plan_head_after: d84c0be2f70a1f303cefd9c216d4296fbf48d907
duration: about 75 min
completed: 2026-10-29
actuals:
  tokens: 6700
  tasks: 3
  commits: 3
---

# Phase 29 Plan 07: The page and the checker know the new record Summary

**`docs/beta.md` promises the changed nightly backup only with sentences of `deploy/BACKUP-ROBUST-RUN.md`, binds them to evidence commit dd077e4 with seven commands that exit 0, and `28-check.sh` holds all of it: `beta` passes in all eight sub-checks and the selftest ends `SELFTEST OK (72 cases)`.**

## What was built

- **Task 1 (tracer), 1be5b75.** `RECORDS` gains `$ROB_REC`. P-19 (ROB-01) leads with the requirement, cites three whole sentences (the one after *What it shows.* of ROB-01, the consist-stage sentence, the Not-tested bullet about the stand-in) and the token `VANISH-ONCE-UNIT-RESULT: success`, and names the stand-in caveat. The ledger row for the atomic snapshot gained one sentence and `P-16, P-19`. The five checks of the verify block (`quotes`, `ledger`, `coverage`, `baseline`, `structure`) exited 0. `evidence` failed (three commands) and `nonpromise` passed; the `evidence` failures are the ones Task 3 removes.
- **Task 2, a3bdb4d.** P-16, P-17 and P-18 cite the new record; P-18 says the failed backup left no directory and `restore.sh` had nothing to refuse (the phrase 'left a directory without a `MANIFEST`' is gone: `grep -c` prints 0). P-20, P-21 and P-22 added. The paragraph under *What 0.1.1 promises* says P-11 to P-15 were made with the earlier `backup.sh`. Limits and ledger below.
- **Task 3, d84c0be.** E-6, E-4 and E-5, the two paragraphs and the code block of seven commands, the sentence about backup.sh, and the checker.

### Promise rows (by first words)

P-16 "CON-01: the pair of database and media is still not atomic" (new numbers: 24 of 40 and 14 of 40 files without a row, 16 of 40 rows without a file, 238 of 269 windows `overlaps-snapshot-step`; no sums); P-17 "CON-02: a backup that the release 0.1.0 made with its own `backup.sh`" (now says that script is not the one that ships); P-18 "CON-03: with 1064 MB free" (rewritten); P-19 "ROB-01: a night whose first copy"; P-20 "ROB-01: a status 24 that recurred"; P-21 "ROB-02: a night that fails for lack of space"; P-22 "ROB-02: the next night on the same filesystem".

### Limits rewritten (by first words), all citing deploy/BACKUP-ROBUST-RUN.md only

- "The backup is not atomic" (numbers of the new record)
- "A file that vanishes during the media copy a second time ends the night" (was "...ends the night: `rsync` ends with status 24 and no `MANIFEST`..."); contains `status 24`, Owner step OS-21
- "A directory without a `MANIFEST` can still be left" (was "A failed night leaves its directory"); merges the signals that cannot be trapped and the hermetic test with the leftovers of the old script; contains `MANIFEST`, Owner step OS-12 and Owner step OS-20
- "Three gaps of `deploy/DEPLOY.md`" (now bound by dd077e4)
- "Whether the retention step should run before the guard and the copy was not decided" (was "What a fix of these findings would change was not decided")

### Limits added

- "The timer, the random delay, the catch-up, the retention and the copy to a second machine were made with the `backup.sh` of the earlier evidence" (Owner step OS-21)
- "A status 24 under the shipped unit was delivered by a stand-in for `rsync`" (Owner step OS-21)

### Ledger rows (the table *What changed since 0.1.0*)

The first cells are unchanged. The row for the atomic snapshot now reads, in its last two cells:

`... A night whose copy of the media ended with status 24 is now repeated once, and the repeat completes the night with the row of the vanished file still in the snapshot. A status 24 that recurs still costs the night, which ends with status 24 and no directory. | P-16, P-19, P-20 |`

The nearly-full-disk row names `P-18, P-21, P-22`; the older-binary row says the 0.1.0 backup was made with 0.1.0's own backup script.

### Limits that still cite deploy/BACKUP-CONSISTENCY-RUN.md, and why

All six are limits whose quoted sentence is a statement about what was not tested or about unchanged code, and is still true; the plan did not list them for rewriting.

| Limit (first words) | Why it is still true |
|---------------------|----------------------|
| "A media tree of production size was not used" | not part of either set |
| "The real traffic of a site at the hour the timer fires" | not part of either set |
| "A restore that crosses a migration was not played" | both releases are at level 60 in both sets |
| "A real disk that is nearly full was not played" | the root-reserved blocks and a different `MIN_FREE_MB` are not part of either set |
| "A restore does not report a row whose file the backup lacks" | `restore.sh` is unchanged, and F-6 of the new record observes it again (the same sentence stands in the new record, so a later plan may re-point it) |
| "The sixth restore within five minutes..." | `restore.sh` and the unit are unchanged |


### Evidence section

- E-6: `Evidence commit: dd077e4` and the `BINARY:` line of the record; E-5 rewritten (the earlier set was made with the `backup.sh` of e86d490; P-16 to P-18 no longer rest on it); E-4 explanation says P-11 to P-15 were made with the `backup.sh` of b501b1b (cited words untouched).
- Seven commands: the first three lose `deploy/backup.sh` (and the third `deploy/test/nightly.sh`); the two program-identity commands pinned to 85fa486 stay; new: `git diff --quiet dd077e4 HEAD -- <seven bound files>` and `git diff --quiet 7d51436 dd077e4 -- . ':!.planning' ':!docs' ':!deploy' ':!.github' ':!CHANGELOG.md'`. All seven exit 0 when the checker runs them.
- Sentence: `deploy/backup.sh` has 89 added lines and 5 removed lines against e86d490 (from `git diff --numstat e86d490 dd077e4`), plus the sentences that restore.sh, DEPLOY.md, both units and run.sh did not change since e86d490, that the earlier sets E-4 and E-5 were made with the earlier `backup.sh`, and that the timer chain was not made again.

### Checker

`E29` (via `CHECK_E29`, default the literal `dd077e4`, with a comment), `EVIDENCE_CMDS` of seven, `check_beta_evidence` (adds `7d51436` and `$E29` to the named shas, the record-equals-E29 proof, E-6 cites its commit and names the record, the numstat sentence with its failure text), `check_beta_coverage` (ROB-01, ROB-02), `fx_page` (ROB tokens, E-6 row, numstat sentence), header comments.

## Selftest

Count: `SELFTEST OK (72 cases)`. Baseline at the start was `SELFTEST FAILED (1 of 63 cases)` (the evidence case on the good fixture, by design); it is green now because the evidence commands were re-made. The one existing case `a requirement missing from the promises` had its transform changed from `'CON-03: '` to `'CON-03 '` because the fixture row now reads `CON-03 ROB-01 ROB-02:`; the rule it tests is unchanged.

Nine new cases, each shown to fail for its own reason by copying the checker aside, breaking the one rule, running the selftest, and restoring the copy (the sha256 of the file after the last restore equals the one taken before: `0ff44809...94ac`; nothing of the break was committed):

| New case | Break (rule replaced by `true \|\|`) | Result |
|----------|--------------------------------------|--------|
| `a command of the new set missing: the bound files of the new set` | the check that every command is on the page | fails (exit 0, wanted 1); the existing `an evidence command missing` fails with it |
| `a command of the new set missing: the program identity of the new set` | same | fails, with the message `the evidence section does not name 7d51436` instead of `does not show the command` |
| `E-6 loses its commit` | the check that E-6 cites `Evidence commit: $E29` | fails, with the row-token message instead of `E-6 does not cite ...` |
| `E-6 loses the record name` | the check that E-6 names `$ROB_REC` | fails, with the row-token message instead of `E-6 does not name ...` |
| `the lines of backup.sh stated with other numbers` | the grep for `A added lines and R removed lines` | fails (exit 0, wanted 1) |
| `the lines of backup.sh not stated` | same | fails (exit 0, wanted 1) |
| `the sha of the checker differs from the record` | `[ "$rs" = "$E29" ]` | fails, with the command message instead of the record message |
| `a promise section without ROB-01` | `ROB-01 ROB-02` removed from the coverage loop | fails (exit 0, wanted 1) |
| `a promise section without ROB-02` | same | fails (exit 0, wanted 1) |

Each break run ended `SELFTEST FAILED` with only the cases of its row (and the one existing case it also protects) failing.

## Verification run

- `28-check.sh beta`: all eight `ok:` lines, exit 0 (`evidence: E-1 to E-6, 7 commands shown and exit 0 ...`).
- `28-check.sh selftest`: `SELFTEST OK (72 cases)`.
- `28-check.sh owner-steps`: exit 0 (21 steps; complete). `changelog` (with GO=0): exit 0.
- `git diff 35c4abc HEAD --stat` lists `docs/beta.md` and `28-check.sh` only (plus this SUMMARY). `git diff` shows no change to P-1 to P-15, E-1 to E-3, *What beta means here* or *Where the word alpha still appears*.
- Expected red until plan 29-08: `frozen` (three FAIL lines: a bound file or an earlier record changed after 05fc076; something outside `.planning`, `docs` and `CHANGELOG.md` changed after 85fa486; a page of docs other than beta.md changed after 85fa486) and `regression` (not run here).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug in the plan's fixture assumption] An existing selftest case no longer matched the fixture**
- **Found during:** Task 3 (first selftest run, `SELFTEST FAILED (1 of 72)`)
- **Issue:** `neg 'a requirement missing from the promises'` removed `CON-03: ` from the fixture row; the row now reads `CON-03 ROB-01 ROB-02:`, so the transform changed nothing and the case passed the page.
- **Fix:** the transform is `'CON-03 '`; the rule and the message are unchanged.
- **Files modified:** `.planning/phases/28-the-promise-in-writing/28-check.sh`
- **Commit:** d84c0be

**2. [Plan wording] Title sentences could not be quoted**
- **Found during:** Task 1
- **Issue:** the record's hand-over names "the title sentences of F-1 and F-2" as quotable, but `sentence_ok` demands a full stop, `- ` or `** ` before a quote and a space or the line end after it; a title stands after `**F-1 (class 3): ` and ends in `.**`.
- **Fix:** the rows quote the sentences after *What it shows.*, the bullets of *Not tested* and body sentences. No checker rule was loosened.
- **Commit:** 1be5b75, a3bdb4d

**3. [Plan wording] Seven commands, three of them with the old record's bound files minus two**
- The plan said the first three commands drop `deploy/backup.sh` (all three) and `deploy/test/nightly.sh` (the third) and "keep the other files it bound"; done exactly so. The first command now names four files (restore.sh, both units, run.sh).

## Known Stubs

None.

## Threat Flags

None. T-29-21: `beta quotes` and `beta nonpromise` ran after each group of edits and find every quoted sentence byte for byte. T-29-22: the timer chain on the changed script, the stand-in under the unit and the real server are limits with Why, Record and a quoted sentence. T-29-23: the commands name fixed commits and file lists, the one that names `HEAD` is run by the checker, and each new rule has a case shown to fail when the rule is broken.

## Deferred / noted

- Re-pointing the limit "A restore does not report a row whose file the backup lacks" to the new record (the same sentence stands in F-6) was left alone; the plan did not list it.
- `ROB-04` (the page and the CHANGELOG) and `ROB-05` (the `frozen` gate and the evidence commands re-pinned) are not ticked in `REQUIREMENTS.md`; plan 29-08 sets ROB-01 to ROB-05 to Complete after the gate is green.

## Self-Check: PASSED

- docs/beta.md and 28-check.sh exist and were changed in 1be5b75, a3bdb4d, d84c0be (all ancestors of HEAD).
- `git rev-list --count 35c4abc..HEAD` before this SUMMARY is 3, equal to the `commits:` above.
- Not committed by design: `.planning/config.json`, `.planning/milestone.lock`.
