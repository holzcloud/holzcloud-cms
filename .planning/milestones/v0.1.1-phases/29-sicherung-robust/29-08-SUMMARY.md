---
phase: 29-sicherung-robust
plan: 08
subsystem: planning
tags: [checker, frozen, regression, selftest, requirements, closing-note, tracer]
requires:
  - phase: 29-sicherung-robust
    provides: the page and the checker know the new record (plan 29-07), the record deploy/BACKUP-ROBUST-RUN.md with evidence commit dd077e4 (plan 29-05)
provides:
  - frozen_in, the six rules of the frozen gate taking a repository and revisions, pinned to 05fc076, 7d20710, 7d51436, FROZEN_END (e54c111) and E29 (dd077e4)
  - FROZEN_PHASE29_FILES, the exact list of five files the phase changed outside .planning, docs and CHANGELOG.md
  - reg_pin and the regression gate pinned to the numbers of the phase (646, 29, 27, 30, 94, 862)
  - 17 selftest cases for the frozen rules and reg_pin, each rule shown to fail on a temporary break
  - ROB-04 and ROB-05 Complete (ROB-01 to ROB-03 were already), after the checker was green
affects: []
tech-stack:
  added: []
  patterns: [a gate that takes its repository and revisions as arguments, so that it is played on a throwaway repository; a gate pinned to commits and a bound set held against HEAD; one selftest case per rule that fails for its own rule label]
key-files:
  created: []
  modified:
    - .planning/phases/28-the-promise-in-writing/28-check.sh
    - .planning/REQUIREMENTS.md
key-decisions:
  - "FROZEN_END is the literal e54c111, the head of main when this plan began, which is after the last commit of the phase that touched anything outside .planning, docs and CHANGELOG.md (docs(29-07)); nothing after it counts outside those three paths."
  - "Rule D (the seven bound files against the evidence commit) names HEAD and is not pinned to an end: a change to one of them means the evidence set is made again."
  - "The nightly.sh selftest pin compares its last two lines, because its last line is the bare word SELFTEST OK and the count stands one line above."
requirements-completed: [ROB-04, ROB-05]
status: complete
commits: 3
plan_head_before: e54c111b8c94acef9e5f6f5d9134b6c8648f9d43
plan_head_after: a8a38c9d954f5298020f2ee3e80fd687899d768b
duration: about 1 h
completed: 2026-10-29
actuals:
  tokens: 4400
  tasks: 3
  commits: 3
---

# Phase 29 Plan 08: The frozen gate, the regression pins and the close of the phase Summary

**`28-check.sh all` exits 0 on the tree of phase 29 with `frozen` pinned to commits (six rules, each played on a throwaway repository and shown to fail on a temporary break), `regression` pinned to the numbers the phase produced, the selftest at `SELFTEST OK (89 cases)`, and ROB-01 to ROB-05 Complete.**

## Baseline of `frozen` at the start

`bash 28-check.sh frozen` printed three FAIL lines on the head of main (`e54c111`):

1. `FAIL: a bound file or an earlier record changed after 05fc076`
2. `FAIL: something outside .planning, docs and CHANGELOG.md changed after 85fa486`
3. `FAIL: a page of docs other than beta.md changed after 85fa486`

## What was built

- **Task 1 (tracer), 3973fae.** `frozen_in DIR R1 R2 BASE END EVC FILES` with rules A to F, `check_frozen` calling it with `"$ROOT" 05fc076 7d20710 7d51436 "$FROZEN_END" "$E29" "$FROZEN_PHASE29_FILES"`; the variables `FROZEN_END=e54c111` and `FROZEN_PHASE29_FILES` in the variable block. `frozen` printed one `ok:` line and exited 0. The verify command on `FROZEN_END` exits 0 (`git diff --quiet e54c111 HEAD -- . ':!.planning' ':!docs' ':!CHANGELOG.md'`). `grep -c FROZEN_END` 5, `FROZEN_PHASE29_FILES` 3, `^frozen_in()` 1, `85fa486` 6 (baseline 8: the two rules that named it are gone).
- **Task 2, 2576904.** `fx_frozen` builds a throwaway repository (`GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, hooks off, `user.name`/`user.email` on every commit); `fx_frozen_run`, `fz_pass`, `fz_only`, `fz_has`, `fz_msg` run `frozen_in` in a subshell and count a case; 17 new cases; `reg_pin LABEL EXPECTED ACTUAL` and a re-pinned `check_regression` that uses it for every pinned line.
- **Task 3, a8a38c9.** The whole checker run, the proofs that the program is as it was, and the requirements ticked after that.

### The files of the phase (FROZEN_PHASE29_FILES)

`git diff --name-only 7d51436 HEAD -- . ':!.planning' ':!docs' ':!CHANGELOG.md'` printed exactly:

```
deploy/BACKUP-ROBUST-RUN.md
deploy/backup.sh
deploy/test/README.md
deploy/test/backup-robust.sh
deploy/test/nightly.sh
```

The union of the `files_modified` entries of plans 29-01 to 29-07 outside those three paths is the same five files. There is no stray change; nothing is reported as a finding.

### The six rules

| Rule | Claim | Pair or revision |
|------|-------|------------------|
| A | restore.sh, DEPLOY.md, holzcloud.service, both backup units, run.sh and the earlier records are as they were | `05fc076` against HEAD |
| B | the phase 26 instrument and release.yml are as they were | `7d20710` against HEAD |
| C | backup.sh and nightly.sh were the same at the base of the phase as the earlier evidence bound them | `05fc076` against `7d51436` |
| D | the seven files the new evidence binds are unchanged; a change means the set is made again (the E-6 command of docs/beta.md, same list) | `dd077e4` against HEAD |
| E | the files changed outside .planning, docs and CHANGELOG.md are exactly the list of the phase | `7d51436` against `e54c111` |
| F | no page of docs but beta.md changed | `7d51436` against `e54c111` |

### Regression pins

| Command | Printed now | Earlier pin |
|---------|-------------|-------------|
| `nightly.sh selftest` | `SELFTEST OK (646 cases)` (and the bare line `SELFTEST OK` after it) | 542 |
| `nightly.sh record-check` | `RECORD-CHECK OK 29 blocks` | same |
| `nightly.sh record-check-con` (earlier set) | `RECORD-CHECK OK 27 blocks` | same |
| `nightly.sh record-check-con` (set of `BACKUP-ROBUST-RUN.md`) | `RECORD-CHECK OK 30 blocks` | new |
| `backup-robust.sh` | `BACKUP-ROBUST OK (94 cases)` | new |
| `native-amd64.sh` status-check, record-check, selftest | unchanged (selftest 862) | same |

The 646 agrees with the SUMMARY of plan 29-03 (542 at the start of the phase, 594 after plan 29-02, 646 after plan 29-03); the 94 agrees with the SUMMARY of plan 29-01; the 30 with the SUMMARY of plan 29-05. No SUMMARY was wrong.

## Selftest: the cases and the temporary breaks

Count: 72 at the end of plan 29-07, 89 now (17 new): `frozen` passes on the repository as the phase leaves it; passes after a later commit that changes a program file, CHANGELOG.md and docs/beta.md (the pin lets the program move on); one failing case for each of A (a unit; restore.sh), B, C, D, E (three: a program file in the evidence commit, a name listed that was not changed, a changed file the list lacks), F; an unknown revision fails closed; the messages of D ("the set is made again") and E (the difference with `diff`); and three for `reg_pin` (equal passes, different fails with label, pin and actual, empty actual fails). Every failing case asserts the exit status 1 and the exact set of rule labels in the output: a case that trips another rule as well does not pass `fz_only`.

For each rule the line that begins `fail "Rule X:` was turned into `: "Rule X:` in a copy of the checker (`28-check-brk.sh` in the phase directory, removed after the run, never committed), the selftest run on the copy:

| Break | Cases that failed |
|-------|-------------------|
| Rule A | a unit changed after the earlier evidence; restore.sh changed after the earlier evidence; an earlier revision that does not exist fails closed (3 of 89) |
| Rule B | release.yml changed after the phase 26 instrument (1 of 89) |
| Rule C | nightly.sh differs between the earlier evidence and the phase base (1 of 89) |
| Rule D | backup.sh changed after the evidence commit; rule D says that the set is made again (2 of 89) |
| Rule E | the evidence commit also changes a program file; a name listed that was not changed; a changed file the list lacks; rule E prints the difference (4 of 89) |
| Rule F | the end commit also changes a page of docs other than beta.md (1 of 89) |
| `reg_pin` (`true \|\|` for the comparison) | another string fails with the label, the pin and the actual line; an empty actual line fails (2 of 89) |

In every run only the cases of that rule failed. The break copy was deleted after the last run; `git status` shows only the committed file.

## Verification run

Lines printed by `bash 28-check.sh all` (exit 0, 84 s):

```
ok: beta structure: title 0.1.1, six headings, four groups, the beta and alpha paragraphs
ok: baseline f1ba5a2: 13 rows unchanged, 31 limits each kept (as the start of a bullet) or moved (quoted in the ledger)
ok: ledger: 9 rows, each quotes a baseline limit and names real promise rows
ok: quotes: 22 promise rows, every cited word found byte for byte, every moved row cites a whole sentence
ok: coverage: BKP-01 to BKP-05, CON-01 to CON-03 and ROB-01, ROB-02 are in the promises, the window is named, every moved row is in the ledger
ok: does-not-promise: 45 limits, each with a reason, every quote found, every owner step exists, every new limit traced to its record
ok: srv: SRV-01 to SRV-03 are pending owner, stay under does-not-promise with their reason and an owner step, and no promise cites the record
ok: evidence: E-1 to E-6, 7 commands shown and exit 0, the DEPLOY.md difference stated, the added and removed lines of deploy/backup.sh stated, E-6 bound to dd077e4
ok: owner steps: 21 steps, each with Source, Where, Needs, a command block, the place of the expected output and of the meaning of a different one; verbatim regions equal their records
ok: owner steps complete: every step of the four records, the push of main, the button, the release and the bundle are here, and the index equals the headings
ok: changelog extract: release.yml's own lines read 0.1.1 and write 10650 bytes of notes (8 bold paragraphs)
ok: changelog topics: the findings, the gaps, the two features and the owner's part are in the entry
ok: changelog go: the parser and the screens read the new entry
ok: frozen: the files that bind the new evidence (dd077e4), the earlier records, restore.sh, the units, run.sh, release.yml and the phase 26 instrument are as they were, and between 7d51436 and e54c111 exactly the listed files changed and no page of docs but beta.md
ok: regression: nightly.sh selftest 646, record-check 29, record-check-con 27 (earlier set) and 30 (the robust record), backup-robust.sh 94, native-amd64.sh status-check and record-check, selftest 862
```

Other lines:

- `28-check.sh selftest`: `SELFTEST OK (89 cases)`.
- `bash deploy/test/backup-robust.sh | tail -n 1`: `BACKUP-ROBUST OK (94 cases)`; `bash -n deploy/backup.sh`: exit 0.
- `bash deploy/test/native-amd64.sh status-check`: `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`.
- `bash .planning/phases/27-repository-tidy/27-check.sh requirements`: `27-CHECK requirements: PASS`, before and after the ticks.
- The program is as it was: `git diff --quiet 7d51436 HEAD -- . ':!.planning' ':!docs' ':!deploy' ':!.github' ':!CHANGELOG.md'` exits 0 (no Go file, template, asset or migration changed); `git diff --quiet e54c111 HEAD -- . ':!.planning' ':!docs' ':!CHANGELOG.md'` exits 0.
- `git tag --points-at HEAD` prints nothing; `git log --oneline origin/main..HEAD` was only read (126 commits ahead of the remote); nothing was pushed, tagged, released, and `gh` was not used.

## The close of the phase: what the five requirements are and the evidence

- **ROB-01** (a status 24 is repeated once; a recurring one ends the night loudly). Record `deploy/BACKUP-ROBUST-RUN.md` F-1 (first copy ended with status 24, the repeat succeeded, a `MANIFEST` was written, by hand and under the unit with `Result=success`), F-2 (a recurring status 24 ends the night with status 24, no directory, the sentinel unchanged, unit `Result=exit-code`, `ExecMainStatus=24`) and F-5 (8 of 40 nights of stage deletes met a real status 24; all 8 repeated and completed; none ended with status 24). The page: P-19, P-20. The hermetic test: `BACKUP-ROBUST OK (94 cases)`. Caveat the page keeps: the status 24 of the four VANISH scenarios is delivered by a stand-in for `rsync` (F-9, class 2).
- **ROB-02** (a failed night removes its own directory). F-3 (a night on a 1064 MB filesystem failed with rsync status 11 and left no directory, by hand and under the unit) and F-4 (the next night passed the guard and failed the same way; `GUARD-REFUSED` no longer occurs). The page: P-21, P-22.
- **ROB-03** (a new evidence set, proved byte for byte). The record against evidence commit `dd077e4`; `nightly.sh record-check-con` with the four t29 names ends `RECORD-CHECK OK 30 blocks`; CON-01 to CON-03 stay green (F-5 to F-8: 40 nights restored per stage, 238 windows `overlaps-snapshot-step`, 28 `after-snapshot`, 3 `straddles-media-end`, none unresolved; the CON-02 backup was made by the tag's own `backup.sh`, F-7).
- **ROB-04** (the page and the notes). `docs/beta.md` P-16 to P-22 quote the record byte for byte with whole sentences; five limits rewritten and two added, each with one record and an owner step; evidence row E-6 with seven commands that exit 0; the 0.1.1 entry of `CHANGELOG.md` in German (`changelog extract`: 10650 bytes of notes, 8 bold paragraphs). Checked by `28-check.sh beta` (eight `ok:` lines) and `changelog` (three).
- **ROB-05** (the gate). This plan: `frozen` pinned and green, `regression` pinned and green, each changed rule with a case that fails when it is broken (table above).

### Figures of the new set

The set was made once by `nightly.sh con-chain` from fresh containers: wall time 32 minutes (`WALL-MINUTES` in the record), two stages of 40 nights each (adds, deletes), 30 appended blocks, 9 findings (F-1 to F-9, eight of class 3 and one of class 2), 13 whole-sentence bullets under *Not tested*. Counters: 1064 MB free at the guard in the capfull scenario; 8 repeated nights of 40 in stage deletes; 0 nights ending with status 24 by chance; 0 directories without a `MANIFEST` at the end.

### What stays open for the owner

- The owner steps **OS-20** and **OS-21** (items 1 and 2 of the record): the first night after the upgrade on a real server, and the timer chain on the changed script.
- The timer, the random delay, the catch-up, the retention and the copy to a second machine were made with the `backup.sh` of the earlier evidence; with the changed script the timer chain and the off-site copy were not made again (a limit of the page, owner step OS-21).
- The order of retention and the guard: whether the retention step should run before the guard and the copy was not decided (a limit of the page).
- A real server with real traffic at the hour the timer fires, a media tree of production size, a real nearly full disk with root-reserved blocks, and a restore that crosses a migration: none was played (limits that remain from the earlier record).
- A status 24 under the shipped unit was delivered by a stand-in for `rsync`, not by a real vanishing file (F-9).
- Releasing 0.1.1: the push of main, the button and the release are the owner's steps in `28-OWNER-STEPS.md`; the milestone is deliberately open.

Nothing was pushed, tagged or released by this phase. The pin of this plan (`FROZEN_END`) states where the program is held fixed; everything the evidence binds is held against `HEAD` for good.

## Deviations from Plan

### Auto-fixed Issues

None in the program. Judgments about the plan's wording:

**1. [Judgment] The RED run of Task 2 is the break run, not a run before the rules existed.** The plan puts the rules in Task 1 (the tracer) and the cases in Task 2 "before the rules are final"; with the rules committed, the cases could not be shown failing "where the rule is not yet there". Each case was instead shown to fail on a temporary break of its rule (table above), which is the claim the plan asks the SUMMARY to carry. *Commit:* 2576904.

**2. [Judgment] "R1 given as the evidence commit fails with Rule C" cannot be built.** With R1 as the evidence commit rule D fails (backup.sh differs between R1 and HEAD) and rule C does not look at the evidence commit. The case for rule C is the claim the rule actually makes: nightly.sh changes between R1 and BASE (variant `c`), failing with Rule C and no other rule.

**3. [Judgment] The `nightly.sh selftest` pin reads two lines.** The plan says the pin is "the line that `tail -n 1` prints"; the last line of that command is the bare `SELFTEST OK`, the count stands one line above. The old pin used `tail -n 3` and `grep -F`; the new one compares the last two lines exactly with `SELFTEST OK (646 cases)` and `SELFTEST OK`.

**4. [Judgment] Names of the helpers.** The runner that builds a fixture and plays `frozen_in` on it is `fx_frozen_run` (so that `grep -c fx_frozen` counts it: 9). `FROZEN_END` is `e54c111`, the head when the plan began, which is also the commit of the last change outside .planning, docs and CHANGELOG.md in the history of the phase, so the pin lets nothing of the phase fall outside rule E.

**5. [Judgment] ROB-01 to ROB-03 were already ticked** by plans 29-01 (ROB-01, ROB-02) and 29-05 (ROB-03) (the traceability rows said Complete); this plan ticked ROB-04 and ROB-05 and the footer, with scoped edits, only after `28-check.sh all` was green. `grep -c '^- \[x\] \*\*ROB-0[1-5]\*\*'` prints 5, the Complete rows 5, the Pending rows 0.

## Known Stubs

None.

## Threat Flags

None. The plan changes the checker and the requirements file only; T-29-24 is mitigated by six rules with a failing case each, and by Rule D against HEAD; T-29-25 is accepted as designed and stated above.

## Self-Check: PASSED

- `.planning/phases/28-the-promise-in-writing/28-check.sh`: present, `bash -n` ok, selftest ends `SELFTEST OK (89 cases)`.
- `.planning/REQUIREMENTS.md`: ROB-01 to ROB-05 `[x]` and Complete.
- Commits 3973fae, 2576904, a8a38c9: ancestors of HEAD.
