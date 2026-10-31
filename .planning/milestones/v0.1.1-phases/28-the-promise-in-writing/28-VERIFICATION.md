---
phase: 28-the-promise-in-writing
verified: 2026-10-26T18:00:00Z
status: passed
score: 3/3 must-haves verified
covered_files:
  - .planning/phases/28-the-promise-in-writing/28-01-PLAN.md
  - .planning/phases/28-the-promise-in-writing/28-01-SUMMARY.md
  - .planning/phases/28-the-promise-in-writing/28-02-PLAN.md
  - .planning/phases/28-the-promise-in-writing/28-02-SUMMARY.md
  - .planning/phases/28-the-promise-in-writing/28-03-PLAN.md
  - .planning/phases/28-the-promise-in-writing/28-03-SUMMARY.md
  - CHANGELOG.md
  - docs/beta.md
covered_digest: "v3:sha256:cd180744fe9d1c4c510128fca25ebbba811cd26b1bc892c3df7b16c7c0ea7bfc"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 28: The promise in writing - Verification Report

**Phase Goal:** `docs/beta.md` says exactly what 0.1.1 has proven, and the owner knows exactly what is still theirs to do.
**Requirements:** DOC-01, DOC-02
**Verified:** 2026-10-26
**Status:** passed (4 non-blocking warnings, no gap)
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every line moved to *promises* cites a sentence of a record, checked byte for byte; every line left under *does not promise* still names its reason | VERIFIED | `28-check.sh beta` (all 8 sub-checks) ok: 18 promise rows, every cited word found by `grep -F`, every moved row (P-11 to P-18) cites a whole sentence. I counted the limits myself: 43 bullets under *does not promise*, 43 contain `Why:`. 9 ledger rows each quote a baseline limit. Spot-check by hand below. |
| 2 | `OWNER-STEPS` lists each real-server and GitHub-UI step with command, expected output and the meaning of a different output | VERIFIED | 19 steps (OS-1 to OS-19). I read the whole file: each has Source, Where, Needs, a command block, an `Expect begins at` phrase and an `A different output is explained at` phrase, and the phrases exist in the step. Verbatim regions are the three records' own text (checker `cmp`s them). `owner-steps complete` ok. |
| 3 | CHANGELOG has a 0.1.1 entry that release.yml's own extraction reads | VERIFIED | I ran release.yml's own version line (`grep -m1 -oE '^## [0-9]+\.[0-9]+(\.[0-9]+)?' CHANGELOG.md`) and its own `awk` (copied from `.github/workflows/release.yml` lines 222 to 225): version `0.1.1`, 7715 bytes of notes, byte-identical to the checker's extraction. |

**Score:** 3/3 truths verified. No behavior-dependent truth (documentation phase).

### Checks run by me

| Check | Result |
|-------|--------|
| `bash 28-check.sh all` (beta structure/baseline/ledger/quotes/coverage/nonpromise/srv/evidence, owner-steps steps/complete, changelog extract/topics/go, frozen, regression) | every line `ok`, exit 0, about 61 s |
| regression line (earlier checkers) | nightly.sh selftest 542, record-check 29, record-check-con 27, native-amd64.sh status-check and record-check, selftest 862: ok |
| `28-check.sh frozen` run separately | ok |
| The five evidence commands of *What the evidence is bound to*, run by hand under bash | all five exit 0; the 0.1.0 command (bf74496 plus `deploy/DEPLOY.md`) exits 1, and `git diff --numstat bf74496 HEAD -- deploy/DEPLOY.md` gives `39 0`, as the page says |
| Files changed since phase start (`47367a8~1..HEAD`, outside `.planning/phases/28`) | `.planning/ROADMAP.md`, `CHANGELOG.md`, `docs/beta.md` only. `deploy/`, `.github/`, all Go code untouched. `git diff 85fa486 HEAD` outside `.planning` is those two files |
| `git diff v0.1.0 HEAD` on `backup.sh`, `restore.sh`, both backup units, `holzcloud.service` | empty, so the CHANGELOG sentence "Byte für Byte die von 0.1.0" is true |
| Tag `v0.1.1` | none, locally or on origin (`git ls-remote`) |
| Working tree | clean |

### Hand spot-check of quoted record sentences (5 plus 3)

Each sentence searched with `grep -F` in the record, and its context read:

1. P-11: "The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole." is `BACKUP-TIMER-RUN.md:125`, the *What it shows* line of BKP-01.
2. P-13: "The shipped timer delays by a random time drawn from [0, 300] seconds, the draws differ, and the run lands inside 03:15:00 plus delay plus accuracy." is `BACKUP-TIMER-RUN.md:135`. The next line says uniformity is not shown (twelve draws, one run); the row repeats that.
3. P-15: "The effect is up to one extra day of retention and never a deletion that is too early, so it is a recorded fact for the owner and not an edit." is inside F-3, `BACKUP-TIMER-RUN.md:107`.
4. P-18: "Nothing exited 0 on a truncated copy (`FALSE-COMPLETE` did not occur)." and the long first sentence are `BACKUP-CONSISTENCY-RUN.md:212`; 1064 MB against 1024 is F-4 (line 114).
5. P-17: "A backup that the previous release's `backup.sh` made restores with this release's `restore.sh` ..." is `BACKUP-CONSISTENCY-RUN.md:193`; the next sentence there says the scripts are byte-identical between the two releases, so "one pair of releases" on the page is the right hedge.
6. P-12 (`BACKUP-TIMER-RUN.md:145`) and the limit-44 sentence (`BACKUP-CONSISTENCY-RUN.md:167`, 41 of 80, 11 of 40, 8 of 40) also match byte for byte.
7. The numbers of the limits agree with the record: F-1 28+13 = 41 of 80, F-2 11 of 40, F-3 8 of 40 (`rsync` 24, runs 9, 14, 16, 18, 22, 27, 36, 40).
8. P-14's "second container on the same bridge network" is `BACKUP-TIMER-RUN.md:346` (DEVIATION[offsite-host]).

### Skeptical read of `docs/beta.md`

- **Promise rows do not claim more than their sentences, with the exceptions in the warnings.** Each moved row carries its own deviation: P-11 says the zone was moved and a real server's zone is not shown; P-12 says stop and start of a container, not a power cycle; P-13 refuses to say "uniform"; P-14 says second container, not a real machine; P-15 says the ages were planted; P-16 promises "the naming of the window and not the agreement of the pair"; P-17 says one pair of releases at level 60; P-18 stops at "nothing completed on a truncated copy".
- **SRV-01 to SRV-03 stay unpromised and pending owner.** Lines 48, 49, 70 and 77 to 79 each carry `Pending owner: SRV-0n`, `Record: deploy/NATIVE-AMD64-RUN.md "Status: pending owner"` and the owner step (OS-5, OS-13, OS-14). No promise row cites `NATIVE-AMD64-RUN.md`. `deploy/NATIVE-AMD64-RUN.md` still reads `PENDING OWNER` for all three (lines 15 to 17), `.planning/REQUIREMENTS.md` still `Pending owner`. The changed 0.1.0 line "none is planned (a user decision)" is reworded to "no run was made yet; one is planned as a button" and is moved to the ledger as `none (reworded)`, which is true now that the workflow exists.
- **The findings are stated as limits, not as promises.** Non-atomic in both directions (line 44, with 41 of 80, 11 of 40, 8 of 40); restore silent on a row without a file (54); `rsync` 24 ends the night, `MANIFEST` missing, unit result not played (55); leftover directory blocks following nights, retention not run (56); the three `DEPLOY.md` gaps and why they are not fixed (57); the three undecided fixes (58); key in the service user's home (59); mirror and failed copy (60); silent failed removal (61); sixth restore in five minutes (84). All ten limits of CONTEXT D-03 are present.
- **Kept limits keep their reason.** 43 of 43 contain `Why:`. The 0.1.0 baseline limits that no record touches (bit rot, secret-key loss, browser, Ubuntu 24.04, ACME, SSO, five updates, and so on) say "no record of phases 24 to 26 plays ...", which is a reason and true. Nothing of the 31 baseline limits was dropped (22 kept verbatim, 9 in the ledger; the ledger rows each name the residual limit that stays above).
- **Evidence binding is honest.** The page states that the 0.1.0 command no longer exits 0 and why (39 added lines), runs three commands against HEAD and two pinned to 85fa486, and names PR 47 as the one difference in the program (`bausteine.css`, weide `style.css`). The quick-task features (Bildlage 8846285, resize question ec99f50) are ancestors of both b501b1b and e86d490, so the claim that they were in the measured program holds.

### Owner steps: the commands

- No `--force`, `-f`, `--force-with-lease`, `--delete`, colon-prefixed refspec, `--tags` or `git tag` anywhere in the file (grep; the checker also refuses these). The only `git push` lines are `git push origin main` (OS-3), `git push origin main:merge-main-0.1.1` (OS-3 fallback), and `git push origin main:release` (OS-16, OS-18). All are plain pushes.
- `git push origin main:release` is the tag-minting step by design (release.yml route 2: tests, then tag through the Releases API). It is fenced "only after the milestone has been completed", with OS-15 first, and the `release` branch (4d238b9, the v0.1.0 tag) is an ancestor of `main`, so it is a fast-forward. It is the owner's, and the text says the author did not run it. That fits the project rule that milestone completion sets the tag.
- The only deletions: `sudo rm -rf -- "${T:?}"` and `sudo rm` of the throwaway unit in OS-13 (verbatim from `deploy/NATIVE-AMD64-RUN.md`; `T` is checked as free in the first block and the question names it), and OS-12's "remove that one directory" in prose. Both come from earlier verified records, not from this phase.
- main/PR 47 advice agrees with the ruleset facts. I read them again with `gh api repos/holzcloud/holzcloud-cms/rules/branches/main`: `deletion`, `non_fast_forward`, `pull_request`, `required_status_checks`. `release` has no ruleset. `git ls-remote --heads origin` still gives main ee3c9ed, release 4d238b9, fix/bausteine-befunde fa696f6. A plain push of `main` is refused as non-fast-forward until `origin/main` is merged in (OS-2, a merge and never a force); the Admin bypass is the first route and the PR route (`merge-main-0.1.1`, merge commit, then `merge --ff-only`) is the fallback, both with the unseen GitHub lines marked "documented, not seen". "Never overwrite pull request 47" is stated.
- Each step has an expected output and an explanation of a different output; `new` steps carry a `Rehearsal:` line that separates what was played from what was only read. The numbers in OS-1 (71 and 3) are dated and explained as drifting.

### CHANGELOG 0.1.1 entry: claims against the repo

Checked and supported: goose 3.29.0, x/image 0.47.0, x/crypto 0.58.0, x/net 0.61.0 (`go.mod`); highest migration 00060; `Bildlage` and the 2560 px question with the exact German answers ("Ja, verkleinern", "Nein, Originale behalten") in `internal/i18n/locales/de.json`, `MaxSide = 2560` in `internal/media/scale.go`; PR 47's four findings (commit 78efbce message); `backup.sh`, `restore.sh` and both units unchanged since v0.1.0; `DEPLOY.md` now names `/opt/holzcloud/offsite-ssh` and the `Host key verification failed` failure; the quick-task features predate the evidence builds; all numbers (41 of 80, 11 of 40, 8 of 40, 31/18/12 hours) match the records. The entry marks what was read and not measured ("aus dem Skript gelesen, nicht gemessen"; "das Ergebnis der Unit für diesen Fall wurde nicht gespielt"). No unsupported claim found. The heading date 2026-10-26 and the number are declared drafts that OS-15 settles.

## Non-blocking warnings (wording; none changes the status)

1. **`docs/beta.md` line 9 over-generalises the platform.** "Every row below was shown in a Debian 12 container ... except where a row says otherwise." P-3 reports `1.3 online and 1.3 offline on systemd 257`, and `deploy/SECURITY-SCORE.md:14` says that run was Debian 13 (trixie). P-3 does not say otherwise. The sentence is inherited verbatim from the 0.1.0 baseline (f1ba5a2), so the phase kept an existing imprecision. Suggested fix: add "(Debian 13 for the systemd 257 half of P-3)" to line 9 or to P-3.
2. **Limit on line 56 (and the `DEPLOY.md` gap on line 57) is unconditional where the record is conditional.** "A failed night leaves its directory, and the guard refuses the following nights until that directory is removed by hand." The record (F-5) observed the refusal on a filesystem of 1064 MB where the left directory holds the space; the CHANGELOG states the condition ("Bei einem knappen Laufwerk"), the page's Why says "holds the space" but the headline does not say "on a nearly full filesystem". It errs on the cautious side, but it is not as exact as the CHANGELOG.
3. **P-16 says "both directions were seen: files without a row and rows without a file", but its cited words are the sentence about naming and reproducing the window.** The supporting sentence is the one just before it in the record (`BACKUP-CONSISTENCY-RUN.md:167`), and limit line 44 quotes it. The row's detail is true but is not itself covered by the row's quotes; a third cited sentence would close it.
4. **Line 55 states "the directory of that night stays without [a MANIFEST]" as fact.** The record (F-3) says this is "a reading of the script and of that transcript; no listing of the left directories was taken". The Why covers the unit result, not this reading. Conservative direction again.

Also not on the page, and left as is because the records call them harness effects: timer F-1 (test image), F-5 (no ssh client in the test image) and F-7 (the Docker VM steps the clock and re-draws the delay). P-13's quoted claim and the 12 samples hold regardless.

## Bookkeeping for the orchestrator (not defects of the phase's work)

- `.planning/ROADMAP.md` still shows `0/3 plans complete` and unchecked plan boxes for phase 28; `.planning/REQUIREMENTS.md` has DOC-01 and DOC-02 unchecked and `Pending`. The phase closure step needs to tick these.
- No commit was made by this verification; only this file was written.

## Gaps Summary

None. All three roadmap success criteria hold in the repository, the scripted checker and the earlier checkers pass, the frozen files are untouched, `docs/beta.md` and `CHANGELOG.md` are the only product-facing content changes, no `v0.1.1` tag exists, SRV-01 to SRV-03 remain unpromised and pending the owner, and the owner steps contain no force push, tag command or deletion that the earlier records did not already carry. The four warnings are wording refinements for a later edit of `docs/beta.md`, which as a bound file should be made together with any other change to it.

---

_Verified: 2026-10-26_
_Verifier: Claude (gsd-verifier)_
