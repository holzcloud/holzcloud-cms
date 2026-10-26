---
phase: 28-the-promise-in-writing
plan: 02
subsystem: docs
tags: [owner-steps, changelog, release-notes, rehearsal]
requires:
  - phase: 28-the-promise-in-writing
    provides: 28-check.sh, OS-6 and the 0.1.1 heading of plan 28-01
provides:
  - 28-OWNER-STEPS.md with 19 steps (OS-1 to OS-19), 14 verbatim regions, an index equal to the headings and a Veto points section
  - CHANGELOG.md with the complete German 0.1.1 entry (eight bold-led paragraphs, 7715 bytes of release notes)
  - 28-check.sh selftest fixture for the changelog that survives a real 0.1.1 entry
affects: [28-03]
tech-stack:
  added: []
  patterns: [verbatim region spliced from the record by a program and compared with cmp, new owner step carries a Rehearsal line of what was played and what was only read]
key-files:
  created:
    - .planning/phases/28-the-promise-in-writing/28-02-SUMMARY.md
  modified:
    - .planning/phases/28-the-promise-in-writing/28-OWNER-STEPS.md
    - CHANGELOG.md
    - .planning/phases/28-the-promise-in-writing/28-check.sh
key-decisions:
  - "The nine record steps of Task 2 and the four archive steps of Task 1 are spliced by a program from 28-check.sh extract; no word of a record is retyped."
  - "The PR 47 merge, the heading date, the reworded native amd64 line, the unedited DEPLOY.md and the two small product features are written down as veto points in the owner steps."
requirements-completed: [DOC-02]
status: complete
commits: 4
plan_head_before: 697555898ddb3d2ee6a5c85885b4f4bcb8c9695a
plan_head_after: b7e381e9b975f44fdada2507c4a0fea0c1a7f7c2
duration: 50min
completed: 2026-10-26
actuals:
  tokens: 15000
  tasks: 3
  commits: 4
---

# Phase 28 Plan 02: The owner's steps and the 0.1.1 entry Summary

**All 19 real-server and GitHub-UI steps of the milestone are in `28-OWNER-STEPS.md` (14 of them carrying a record's own text, copied by a program), and `CHANGELOG.md` has a complete German 0.1.1 entry that release.yml's own lines read as 7715 bytes of notes.**

## Base

The worktree HEAD was ee3c9ed with a clean tree; `main` was 6975558. The tree was clean, so it was reset to 6975558 with `git reset --hard` as instructed. Remote heads before and after the plan (`git ls-remote --heads origin`): `main` ee3c9ed, `release` 4d238b9, `fix/bausteine-befunde` fa696f6. Nothing was pushed, tagged or run on GitHub.

## What was done

- **Orchestrator-authorised fix, first (fbae710, `fix(28-02): 28-check.sh selftest fixture ...`).** `fx_changelog` inserted a small 0.1.1 entry above the first `## ` heading of the real `CHANGELOG.md`; the real file now starts with `## 0.1.1`, so the fixture had two 0.1.1 headings and `changelog extract: the good fixture` failed (the 1 of 59 reported by plan 28-01). The fixture now keeps the preamble (everything above the first `## `), puts its own 0.1.1 entry there and continues from the first `## 0.1.0` heading, so any 0.1.1 entry the real file carries is left out. Nine lines of the checker changed, all inside `fx_changelog`; no check was loosened and no `st_fails` reason was touched. Proof: `selftest` prints `SELFTEST OK (59 cases)` with the real `CHANGELOG.md` (before any CHANGELOG work, and again at the end) and with the 0.1.0-only baseline (`CHANGELOG_FILE=` pointing at `git show 47367a8:CHANGELOG.md`). The negative cases (`st_fails` with their reason strings) are among the 59 and all pass, which they could not do if their reasons had changed.
- **Task 1 (52c8cbb).** Introduction, index, OS-1 to OS-4 and OS-19 (new, each with a Rehearsal line), OS-15 to OS-18 (the four release steps of the archived 0.1.0 owner steps, 0.1.0 read as 0.1.1, with the commands, notes and the Veto points section added). `owner-steps steps` passed with 10 steps, `frozen` 0, three remote heads.
- **Task 2 (de99da8).** OS-5 and OS-7 to OS-14, each the record's own text between the two comment lines, spliced by a short script that calls `28-check.sh extract` (not retyped). `git diff` of the commit has no removed line, so OS-1 to OS-4, OS-6 and OS-15 to OS-19 are byte for byte as in Task 1. The index table now lists OS-1 to OS-19. Baseline before the task: `owner-steps complete` exited 1 (nine record steps and `gh workflow run native-amd64.yml` missing, fewer than 19 steps), `owner-steps steps` exited 0 for ten steps. After: both exit 0, 19 steps, 14 verbatim regions.
- **Task 3 (b7e381e).** The 0.1.1 entry, under the unchanged heading `## 0.1.1 — 2026-10-26` directly above `## 0.1.0 — 2026-10-14`. Baseline before the task: `changelog topics` exited 1 listing 11 missing words and the bold-lead count (1 of 6), `changelog extract` exited 0. After: `extract` (7715 bytes of notes, 8 bold paragraphs), `topics`, `GO=1 changelog go` and `frozen` exit 0; the entry holds no `## ` line and no raw HTML; only `CHANGELOG.md` changed in the commit.

All 18 Expect and different-output phrases of Task 2 and Task 1 were found verbatim in their steps; no phrase had to be replaced by a nearer sentence start.

## Rehearsal and read-only transcripts

Rehearsals ran in `mktemp -d` directories (a bare repository standing in for GitHub, a clone of it, a second clone standing in for PR 47), with `GIT_CONFIG_GLOBAL=/dev/null`, and were removed afterwards. Git version 2.54.0 (Apple Git-157).

Throwaway pair (a clone four commits ahead, and four commits only in the bare repository, one a merge):

```text
git push origin main                          (before fetching)
 ! [rejected]        main -> main (fetch first)
exit 1
git fetch origin
From /private/tmp/claude-501/rehearsal.QvscEB/github
   7ef2356..e3a3990  main       -> origin/main
git rev-list --left-right --count main...origin/main
4	4
git log --oneline main..origin/main           (four commits, newest "Merge branch 'side'")
git push origin main                          (after the fetch, before the merge)
 ! [rejected]        main -> main (non-fast-forward)
git merge-tree --write-tree main origin/main
4812c0eedcb33311e480f0631736b8d6b776445c      exit 0
git merge --no-edit origin/main
Merge made by the 'ort' strategy.   side.txt | 1 + ; style.css | 2 ++ ; 2 files changed, 3 insertions(+)
git diff --stat HEAD^1 HEAD
 side.txt  | 1 +     style.css | 2 ++     2 files changed, 3 insertions(+)
git push origin main
   e3a3990..32d1277  main -> main         exit 0
git ls-remote origin refs/heads/main  and  git rev-parse main    (the same id, 32d127759ddcd939...)
git fetch; git rev-list --left-right --count main...origin/main  ->  0	0
conflict case (both sides change one line of app.txt):
git merge-tree --write-tree main origin/main
8a48773f60c6124ac2fa729694b306037b16bd85
100644 df967b96... 1	app.txt   (and stages 2 and 3)
Auto-merging app.txt
CONFLICT (content): Merge conflict in app.txt          exit 1
```

Read-only calls against the real remote and repository (2026-10-26; the values the steps quote):

```text
git fetch origin                                   (no output, exit 0)
git rev-list --left-right --count main...origin/main     71	3
git log --oneline main..origin/main
ee3c9ed Merge pull request #47 from holzcloud/fix/bausteine-befunde
fa696f6 refactor(weide): die Aufruf-Regel steht jetzt im Kern
78efbce fix(bausteine): vier Befunde im Kern statt je Vorlage abfangen
gh api repos/holzcloud/holzcloud-cms/rules/branches/main --jq '.[].type'
deletion / non_fast_forward / pull_request / required_status_checks
gh api repos/holzcloud/holzcloud-cms/rulesets/22933928  (GET): enforcement active, bypass_actors [{actor_id 5, RepositoryRole, always}], the same four rule types
gh workflow list: Backfill the missed releases, CI, image, Release, Security Audit, tagfix, Dependabot Updates   (no native-amd64)
gh run list --workflow CI --branch main --limit 3
completed success ... #47 merge ... 13m37s / completed success ... 13m31s / completed success ... 13m38s
gh run list --workflow Release --limit 3             14m12s, 14m7s, 13m50s, all completed success
git ls-remote --tags origin                         ends ... refs/tags/v0.1.0 (no v0.1.1)
git merge-tree --write-tree main origin/main        8619e6408849e43b2b0ef8077db569171299d1bf, exit 0 (real repository: the three commits merge cleanly)
git merge-base --is-ancestor origin/release main    exit 0 (release is an ancestor, so main:release is a fast-forward)
git diff --stat <merge base> origin/main            bausteine.css | 25 ; weide/style.css | 36 ; 2 files changed, 22 insertions(+), 39 deletions(-)
bundle: ls -l 26349797 bytes; sha256 bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924; git bundle verify "is okay", the one ref ef4873ad07e30ac27b02e5829ff0f1a20307ca9f refs/heads/backup/lokal-vor-origin-abgleich, "complete history", sha1
```

Note on OS-1: `71 3` is read from this worktree's `main` and grows with every commit; the step says so. Not played because they need GitHub, and said so in the steps: the `remote: Bypassed rule violations` lines, the `GH013` line, the pull request route of OS-3 (documented), the `in_progress` word of `gh run list`, and what `gh workflow list` shows after the push.

## Sources of the CHANGELOG paragraphs (D-10, T-28-10)

1. What 0.1.1 shows: `deploy/BACKUP-TIMER-RUN.md` BKP-01 (03:15:19, shifted zone), BKP-02 (stamp, 188 s stop, control `Persistent=false`), BKP-03 (delay 300 s), BKP-05 (retention), BKP-04 (copy to a second host, restore from it); `deploy/BACKUP-CONSISTENCY-RUN.md` CON-02 (backup of 0.1.0). `Persistent=true` and `RandomizedDelaySec=300` also read in `deploy/holzcloud-backup.timer`.
2. What the runs teach: timer record F-2 (home hidden by `ProtectHome=true`, `Host key verification failed`, `/opt/holzcloud/offsite-ssh`, the DEPLOY.md paragraph of commit 7baa09d), F-4 (mirror, `rsync -a --delete`), F-6 (a failed copy leaves the new backup), F-3 (`find -mtime +30`, 30 d 12 h kept, 31 d 1 h removed).
3. Not atomic: consistency record Summary and "What it shows" (41 of 80, 11 of 40, four uploaders, a page saver and a deleter, the window in `backup.sh`'s own comment, controls C0 to C2, 404 as the only symptom, F-2).
4. Full disk and vanished file: CON-03 (status 11, no `MANIFEST`, `restore.sh` refusal, guard refusal of the next run), F-3 (status 24, 8 of 40, the unit's result not played, Not tested), F-5 (the left directory holds the space; retention comes after the guard, "a reading of the order of the script"), owner-step items 1 and 2 of the consistency record.
5. Gaps of the documents: `28-CONTEXT.md` D-04 and plan 28-03's list of limits; the consistency record's hand-over lines.
6. Owner's part: `deploy/NATIVE-AMD64-RUN.md` (SRV-01 to SRV-03 pending), `.github/workflows/native-amd64.yml`, Not tested lists of both records.
7. Program changes: commits 8846285 (`Bildlage`; `de.json` "Image position" = "Bildlage", "left/right/above/below" = links/rechts/oben/unten), ec99f50 (`de.json` "Scale down to fit within %d px?", "Yes, scale down", "No, keep the originals", "The choice applies to every image on this page ..."; 2560 px from `.planning/phases/28-the-promise-in-writing/28-CONTEXT.md`), 78efbce and fa696f6 (PR 47, the four findings), `git diff v0.1.0 HEAD -- go.mod` (goose v3.29.0, x/image v0.47.0, x/crypto v0.58.0, x/net v0.61.0), the highest migration `00060_order_token.sql`, and `git diff --stat v0.1.0 HEAD -- deploy/backup.sh deploy/restore.sh deploy/holzcloud-backup.service deploy/holzcloud-backup.timer` printing nothing.
8. What the evidence no longer covers: `git merge-base --is-ancestor` shows 8846285 and ec99f50 are in both evidence commits b501b1b and e86d490; the two CSS files are those PR 47 changes after the evidence builds (`28-CONTEXT.md`, `28-check.sh frozen`).

## Verification

- `owner-steps` (steps and complete): exit 0, 19 steps. `changelog` (extract, topics, go): exit 0, 7715 bytes, 8 bold paragraphs. `selftest`: `SELFTEST OK (59 cases)`. `frozen`: exit 0. Remote heads after: identical to before.
- `NATIVE_SELFTEST=0 28-check.sh regression` cannot pass in this worktree and it did not (see the deviation below). Its five components, run by hand with `E=/Users/holz/Projects/holzcloud-cms/deploy/test/out`: `nightly.sh selftest` `SELFTEST OK (542 cases)` (read through `2>&1`), `record-check` `RECORD-CHECK OK 29 blocks`, `record-check-con` `RECORD-CHECK OK 27 blocks`, `native-amd64.sh status-check` `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`, `native-amd64.sh record-check` `RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03`. The 862-case native selftest was not run (plan 28-03).
- `git status --short` is empty at the end; no go test left files behind.

## Deviations from Plan

### Authorised change of the checker

**1. [Checker defect, fixed by authorisation] `fx_changelog` broke once the real `CHANGELOG.md` carried a 0.1.1 entry.** Reported by plan 28-01; fixed in fbae710 as described above, the only edit of `28-check.sh`. Files modified: `28-check.sh`. Commit: fbae710.

### Verification item that cannot pass as written, in a worktree

**2. [Environment, reported, not fixed] `28-check.sh regression` fails in an agent worktree.** It derives `ROOT` from `git rev-parse --show-toplevel` and passes `E="$ROOT/deploy/test/out"` to `record-check-con`, and `record-check` reads the evidence from the same directory; the gitignored `deploy/test/out` exists only in the main checkout, not in a worktree, so both fail with 29 and 27 problems (the first run printed both FAIL lines, the second, with `E` exported, only the second because the checker overrides `E` for `record-check-con`). The five components pass when run by hand with `E` set (above). Nothing in the program, the records or the checker is wrong. In the main checkout `regression` should exit 0; the orchestrator can confirm there.

### Auto-fixed Issues

None. The plan was executed as written; the one wording change worth naming is that the intro of the owner steps was written first (the plan asked for it in Task 1) and that the OS-3 pull request route uses `git push origin main:merge-main-0.1.1` (a branch with another name, as the plan asked) and no force.

## Auth gates

None. `gh` was already signed in and used for plain reads only.

## Known Stubs

None. A scan for `TODO`, `FIXME`, `coming soon` and `placeholder` finds one word `placeholder` in `28-OWNER-STEPS.md`, inside the verbatim region of `deploy/NATIVE-AMD64-RUN.md O-1` (the record's own sentence about a result shape); it is not a stub of this plan.

## Threat Flags

None. No new network endpoint, auth path, file access or schema. The owner steps file holds no token, key block, credential URL, force push, deletion or tag command (`owner-steps steps` checks all of these); the remote was only read.

## Veto points for the owner (also in the file itself)

- OS-2: merging the three commits of PR 47 into `main` makes the shipped program differ from the evidence builds in `cmd/holzcloud/assets/bausteine.css` and the weide `style.css`. This is the owner's decision; the step and the CHANGELOG say so.
- The date `2026-10-26` in the entry heading is a draft (OS-15).
- The reworded native amd64 line (D-13) is plan 28-03's.
- `deploy/DEPLOY.md` is not edited; the three gaps go on `docs/beta.md`.
- The CHANGELOG states two small product features (Bildlage, the 2560 px question) although new CMS functions are out of scope for the milestone; they predate the evidence builds.

## Self-Check: PASSED

- `28-OWNER-STEPS.md`, `CHANGELOG.md` and `28-check.sh` exist and are changed; commits fbae710, 52c8cbb, de99da8 and b7e381e are ancestors of HEAD.
