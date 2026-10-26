# Phase 28: the owner's steps

Claude ran none of the commands in this file. A step marked as taken from a record carries that record's text unchanged, between the two comment lines of its verbatim region: the region is copied by a program (`28-check.sh extract`) and compared with `cmp`, so a step cannot drift from its record. The words outside the two comment lines are this file's own. A step whose Source is `new` has a Rehearsal line that says what was played in a throwaway repository, what was read from GitHub with a plain GET call and what was only read in a workflow file and not seen.

The order in which the steps are taken:

- **OS-1 to OS-4 first.** The button of OS-5 and the release of OS-16 need `main` on GitHub. The records under `deploy/`, `docs/beta.md` and the workflow `native-amd64` exist there only after OS-3, and the release body links to them.
- **OS-5** after OS-4: it is the button of the workflow `native-amd64` and runs on GitHub, so it needs the workflow to exist there.
- **OS-6 to OS-14** whenever the server runs the units that this release ships. They need no GitHub step, and none of them needs another in a particular order except where a Needs line says so.
- **OS-15 to OS-18 only after the milestone has been completed.** The number, the date and the tag belong to that step; the number 0.1.1 and the date 2026-10-26 that stand in `CHANGELOG.md` are drafts. The release is not made by the author of this file, and no step of it has been taken: there is no tag `v0.1.1`.
- **OS-19** at any time.

The real-server steps assume a server installed by `deploy/DEPLOY.md`: the program and the units under `/opt/holzcloud`, the units enabled, a service user. They give what the records give, and nothing more is promised for another layout.

| Step | Where | Needs | Source |
|------|-------|-------|--------|
| OS-1 | development machine | nothing | new |
| OS-2 | development machine | OS-1 | new |
| OS-3 | development machine | OS-2 | new |
| OS-4 | GitHub UI or development machine with gh | OS-3 | new |
| OS-6 | real server | nothing | deploy/BACKUP-TIMER-RUN.md O-1 |
| OS-15 | development machine | OS-3 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 |
| OS-16 | development machine | OS-3, OS-4, OS-15 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 |
| OS-17 | development machine | OS-16 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 |
| OS-18 | development machine | OS-17 | .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 |
| OS-19 | development machine | nothing | new |

### OS-1: Look at GitHub from here

Source: new
Where: development machine
Needs: nothing
Expect begins at: "The first prints nothing or `From` lines"
A different output is explained at: "A second number of 0 means"
Rehearsal: The first three commands were played in a throwaway pair of repositories made with `mktemp -d` (a bare repository standing in for GitHub, a clone four commits ahead of it, and four commits that exist only in the bare repository, one of them a merge); they printed a `From` line, the two numbers 4 and 4, and a list of four commits. The fourth command and the ruleset behind it were read from GitHub on 2026-10-26 with plain GET calls; nothing was written to GitHub, and `git ls-remote --heads origin` printed the same three heads (`main` ee3c9ed, `release` 4d238b9, `fix/bausteine-befunde` fa696f6) before and after.

```bash
git fetch origin
git rev-list --left-right --count main...origin/main
git log --oneline main..origin/main
gh api repos/holzcloud/holzcloud-cms/rules/branches/main --jq '.[].type'
```

The first prints nothing or `From` lines. The second prints two numbers on one line: the commits that are only here, then the commits that are only on GitHub. Read on 2026-10-26 it printed `71` and `3`; the first number grows with every commit made here, the second was 3. The third lists the commits that are only on GitHub; on 2026-10-26 they were `ee3c9ed` (the merge of pull request 47), `fa696f6` and `78efbce`. The fourth prints the types of the rules that apply to `main`, one per line; on 2026-10-26 they were `deletion`, `non_fast_forward`, `pull_request` and `required_status_checks`.

A second number of 0 means that there is nothing to bring in, and OS-2 is skipped. Any other commit than the three means that somebody pushed since, so read it before OS-2. No `pull_request` line means that the ruleset changed and OS-3 needs no bypass. An authentication error from `gh` means `gh auth login` first.

`main` carries a ruleset ("main: kein Force-Push, kein Loeschen, PR mit gruener Pruefung") whose only bypass is the Admin role, as read on 2026-10-26 with a plain GET call on the ruleset. It refuses a push that is not a fast-forward, it refuses deleting `main`, and it asks for a pull request with the green check `Build and test`; an Admin may bypass it. This is why OS-2 comes before OS-3.

This step does not answer a line of `docs/beta.md`; it tells what OS-2 and OS-3 start from.

### OS-2: Bring the commits that are only on GitHub into main

Source: new
Where: development machine
Needs: OS-1
Expect begins at: "The first prints one 40-character tree id"
A different output is explained at: "Exit 1 with file names from the first command means a conflict"
Rehearsal: All three commands were played in the throwaway pair of OS-1: the first printed one tree id and exit 0, the second printed `Merge made by the 'ort' strategy.` and the two files it brought in, the third listed those two files. A conflict was played too, with both sides changing one line of one file: the first command then printed three index lines, `CONFLICT (content): Merge conflict in app.txt` and exit 1. On the real repository only the first command was run on 2026-10-26 (it exits 0, so the three commits merge cleanly) and `git diff --stat` between the merge base and `origin/main`, which printed the two files named below; the merge itself was not made.

```bash
git merge-tree --write-tree main origin/main
git merge --no-edit origin/main
git diff --stat HEAD^1 HEAD
```

The first prints one 40-character tree id and exits 0; it changes nothing in the working copy. The second makes a merge commit and prints `Merge made by the 'ort' strategy.` and the files it brought in. The third lists exactly `cmd/holzcloud/assets/bausteine.css` and `cmd/holzcloud/templates/public/weide/style.css`, the two files of pull request 47; the same difference between the merge base and `origin/main` read `2 files changed, 22 insertions(+), 39 deletions(-)` on 2026-10-26.

Exit 1 with file names from the first command means a conflict: stop, and do not resolve it with a strategy option. Any file other than the two means that something besides pull request 47 came in since OS-1; read it first. Nothing is sent to GitHub by this step.

This is the owner's decision. The ruleset forbids a push that is not a fast-forward and forbids deleting the commits, so they have to come in; but bringing them in makes the shipped program differ from the evidence builds in those two files, which `docs/beta.md` states and the CHANGELOG entry names. Never overwrite pull request 47 on GitHub.

This step does not answer a line of `docs/beta.md`; it is the reason for the program-identity sentence of its evidence section.

### OS-3: Publish main

Source: new
Where: development machine
Needs: OS-2
Expect begins at: "The push output ends with a line that contains `main -> main`"
A different output is explained at: "A line that ends with `(non-fast-forward)` means"
Rehearsal: The push and the two reads were played on the throwaway pair of OS-1. A push before OS-1 was refused with `! [rejected]` and `main -> main (fetch first)`; after the fetch and before OS-2 the refusal ended with `(non-fast-forward)`; after OS-2 the push printed `e3a3990..32d1277  main -> main` and exit 0, and the two reads printed the same commit id. Not played, because they need GitHub: the `remote: Bypassed rule violations` lines, the `GH013` line and the pull request route; those are described as GitHub documents them and were not seen here.

```bash
git push origin main
git ls-remote origin refs/heads/main
git rev-parse main
```

The push output ends with a line that contains `main -> main`, in the form `OLD..NEW  main -> main`. GitHub may add lines that begin `remote: Bypassed rule violations`; they say that the Admin bypass of the ruleset was used, they are not an error, and they are documented by GitHub and were not seen here. The last two commands print the same commit id.

This comes first among the steps that touch GitHub: the records under `deploy/`, `docs/beta.md` and the workflow `native-amd64` exist on GitHub only after it, and the release body links to them.

A line that ends with `(non-fast-forward)` means that OS-1 and OS-2 were not done against the current GitHub: fetch again and repeat OS-2. A line with `GH013: Repository rule violations found` (documented, not seen) means that the signed-in account is not an Admin, so use a pull request instead. Push `main` to a branch with another name:

```bash
git push origin main:merge-main-0.1.1
```

Open the pull request from that branch into `main` on GitHub, wait for the check `Build and test`, and merge it with the option Create a merge commit (never Squash and never Rebase, which would rewrite the commits). Then take the merge back:

```bash
git fetch origin
git merge --ff-only origin/main
```

This step does not answer a line of `docs/beta.md`; every link that the page and the release body hold leads nowhere until it is done.

### OS-4: Read the result of CI on main

Source: new
Where: GitHub UI or development machine with gh
Needs: OS-3
Expect begins at: "The first line begins with `completed` and `success`"
A different output is explained at: "`in_progress` means wait"
Rehearsal: Read on 2026-10-26 with plain `gh` reads, before the push: `gh run list --workflow CI --branch main --limit 3` printed three lines that begin `completed` and `success` and took 13m37s, 13m31s and 13m38s, and `gh workflow list` printed seven workflows (Backfill the missed releases, CI, image, Release, Security Audit, tagfix, Dependabot Updates) and no `native-amd64`. What the commands print after the push was not seen, because the push is the owner's.

```bash
gh run list --workflow CI --branch main --limit 1
gh workflow list
```

The first line begins with `completed` and `success` and names the commit that was pushed. The three runs before took between 13m31s and 13m38s, so wait about that long after OS-3. `gh workflow list` now includes `native-amd64`, which it did not on 2026-10-26 before the push (GitHub lists a workflow with a `workflow_dispatch` trigger once its file is on the default branch; documented, not seen).

`in_progress` means wait (the exact word is documented by `gh`, not seen here). `completed` and `failure` means stop: read `gh run view RUN-ID --log-failed`, and take neither OS-5 nor OS-16. No `native-amd64` in the list means that the push did not reach the default branch.

This step does not answer a line of `docs/beta.md`; it tells whether OS-5 and the release may follow.

### OS-6: The shipped timer at its real 03:15

Source: deploy/BACKUP-TIMER-RUN.md O-1
Where: real server
Needs: nothing
Expect begins at: "Expect `TriggeredBy=holzcloud-backup.timer`"
A different output is explained at: "A start at another hour points at"
<!-- verbatim: deploy/BACKUP-TIMER-RUN.md O-1 -->
**O-1. The shipped timer at its real 03:15.** After the timer has been enabled for a night:

```bash
systemctl list-timers holzcloud-backup.timer
systemctl show holzcloud-backup.service -p TriggeredBy -p Result -p ExecMainStartTimestamp
journalctl -u holzcloud-backup.service --since yesterday -o short-precise | tail -n 20
timedatectl
```

Expect `TriggeredBy=holzcloud-backup.timer`, `Result=success`, an `ExecMainStartTimestamp` between 03:15:00 and about 03:21:00 local time, and a journal that ends with `=== Backup complete: /opt/holzcloud/backups/YYYYMMDD-HHMMSS ===`. A start at another hour points at the host's time zone (`timedatectl`), at a clock that is not synchronised (`System clock synchronized: no`), or at `AccuracySec` (the timer has the default of one minute). `TriggeredBy` empty means the last run was a manual one, not the timer's.
<!-- end verbatim -->

This step answers the limit about a real server's own zone on `docs/beta.md`; a result is not recorded in this repository by this phase.

### OS-15: The date of the heading

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 (adapted)
Where: development machine
Needs: OS-3
Expect begins at: "If the release is made on another day, edit that one line and commit it."
A different output is explained at: "The workflow ignores the date"
Rehearsal: The heading command was run on `CHANGELOG.md` of this repository on 2026-10-26 and printed the line named below; the edit and the commit it may lead to were not played.

```bash
grep -m1 '^## ' CHANGELOG.md
date +%Y-%m-%d
```

The first prints the heading of the entry, now `## 0.1.1 — 2026-10-26`; the second prints today's date. The number 0.1.1 and the date are drafts that the milestone completion step confirms. What to do when they differ on the day of the release is in the text below, taken from the earlier release.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 0 -->
The heading `## 0.1.1 — DATE` of `CHANGELOG.md` carries the day the entry was written. If the release is made on another day, edit that one line and commit it. The workflow ignores the date; the admin screen and the embedded changelog show it.

Read `docs/beta.md` once. It is the contract the release page links to.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`; it settles the one line of the entry that the page's version number is read from.

### OS-16: Make the release

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 (adapted)
Where: development machine
Needs: OS-3, OS-4, OS-15
Expect begins at: "The push starts the Release workflow"
A different output is explained at: "If the tests fail, nothing is tagged and nothing is published."

This is taken only after the milestone has been completed and the number and the date have been confirmed. The other steps of this file whose result is still pending do not hold it back, because the page ships them as limits.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 2 -->
```bash
git push origin main:release
```

This is route 2 of `.github/workflows/release.yml`. The `release` branch holds no work of its own and is only ever fast-forwarded. The push starts the Release workflow, which reads the first version heading of `CHANGELOG.md` (0.1.1), runs the whole test suite, builds linux/amd64, mints the tag `v0.1.1` through the Releases API after the tests have passed, and publishes the release as a pre-release, because every `v0.*` tag is one.

The alternative for a person: GitHub, Actions, Release, Run workflow, with no inputs (route 3), after OS-3. With no inputs it does the same thing at the head of the default branch.

The release body is the `CHANGELOG.md` section for 0.1.1 followed by GitHub's generated notes. There is no separate notes file. If the tests fail, nothing is tagged and nothing is published.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`; the page and the release body are what it publishes.

### OS-17: Check the release

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 (adapted)
Where: development machine
Needs: OS-16
Expect begins at: "The first prints one line."
A different output is explained at: "no line from `git ls-remote --tags` means that no tag exists yet"
Rehearsal: The explanation after the text below was read from `.github/workflows/release.yml` and was not played. On 2026-10-26 `git ls-remote --tags origin` listed `v0.1.0` and no `v0.1.1`, and the last three runs of the Release workflow had taken 14m12s, 14m7s and 13m50s.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 3 -->
```bash
git ls-remote --tags origin v0.1.1
gh release view v0.1.1 --json isPrerelease,body
```

The first prints one line. In the second, `isPrerelease` is `true` and the body opens with the text of the CHANGELOG entry.

The job `holzcloud-ch` writes the version into the holzcloud.ch pages only when the `CMS_TOKEN` secret exists. Without it the job prints a notice and succeeds without doing so.
<!-- end verbatim -->

Wait about 14 minutes after OS-16 before this. A different output, as the workflow gives it: no line from `git ls-remote --tags` means that no tag exists yet, so the Release run has not finished or its tests failed (`gh run list --workflow Release --limit 3` shows which); if the tests failed, nothing was published. `isPrerelease` of `false` means stop and read `.github/workflows/release.yml`, because every `v0.*` tag is a pre-release there.

This step answers no line of `docs/beta.md`; it checks that the release exists as the page says it does.

### OS-18: If something is wrong afterwards

Source: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 (adapted)
Where: development machine
Needs: OS-17
Expect begins at: "moves no tag"
A different output is explained at: "A published tag is never moved."
Rehearsal: Read from `.github/workflows/release.yml` and the earlier release; not played, because it needs a release that exists.

```bash
git push origin main:release
```

This is used only when the text of the entry must be corrected after the release exists: commit the corrected entry on `main` first, then run the push. The record's own words follow.

<!-- verbatim: .planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md step 4 -->
A push to `release` at a version that is already out moves no tag and rewrites only the release text from `CHANGELOG.md`. That is how a corrected entry reaches the page. A published tag is never moved.
<!-- end verbatim -->

This step answers no line of `docs/beta.md`.

### OS-19: Keep the bundle of phase 27

Source: new
Where: development machine
Needs: nothing
Expect begins at: "The size is 26349797 bytes"
A different output is explained at: "Another size or another checksum means"
Rehearsal: The three read-only commands were run on the real file on 2026-10-26 and printed what is written below. The command that brings the lineage back was played in a throwaway repository in phase 27 (E-5 of its owner steps); it was not run again here.

The bundle is `/Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle`, the only copy of the old lineage now that its branch is gone. Run the commands in any clone of the project; `git bundle verify` needs a repository.

```bash
ls -l /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
shasum -a 256 /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
git bundle verify /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle
```

The size is 26349797 bytes. The checksum is `bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924`. `git bundle verify` prints five lines: the first ends with `is okay`, then `The bundle contains this ref:`, `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f refs/heads/backup/lokal-vor-origin-abgleich`, `The bundle records a complete history.` and `The bundle uses this hash algorithm: sha1`.

Another size or another checksum means that the file changed or was replaced: delete nothing, and compare it with a copy. A `verify` that fails means that the bundle is damaged.

To bring the lineage back, fetch it from the bundle into a branch of the same name (the command of E-5 of the phase 27 owner steps, with the real path):

```bash
git fetch /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle refs/heads/backup/lokal-vor-origin-abgleich:refs/heads/backup/lokal-vor-origin-abgleich
```

The old lineage is never merged into `main`: single features are ported, not the branch.

This step answers no line of `docs/beta.md`; it keeps what the repository no longer holds.

## Veto points

These are the decisions the plans of this phase made for the owner. Each can be overruled; the cost is named.

**The merge of pull request 47 (OS-2).** `main` here is ahead of GitHub and also behind it, so the commits of pull request 47 have to come in before anything can be published. The cost: the shipped program then differs from the programs that the evidence builds measured, in `cmd/holzcloud/assets/bausteine.css` and in the `style.css` of the weide theme. `docs/beta.md` and the CHANGELOG entry say so in words; overruling this means publishing from a branch without those commits, which GitHub's ruleset does not allow as a push to `main`.

**The date of the heading (OS-15).** `2026-10-26` is the day the entry was written, as in 0.1.0. The milestone completion step confirms or changes it, and the workflow ignores it.

**The reworded native amd64 line.** The 0.1.0 limit "none is planned (a user decision)" is no longer true since the button exists; `docs/beta.md` says that no run was made yet and that one is planned. To keep the old wording instead, keep the baseline bullet and add a `Why:` that corrects it; the checker accepts both.

**`deploy/DEPLOY.md` is not edited.** Three gaps are written in `docs/beta.md` and in these steps instead: a vanished file ends the night, a failed night's directory blocks the following nights, and a restore does not report a row whose file is missing. An edit of `deploy/DEPLOY.md` would stale the evidence bound to it and force the runs to be made again; whether to pay that is the owner's.

**Two small product features ride in this release.** The block Bild und Text has a position for the picture, and an upload of a large raster asks whether to shrink it. They predate the evidence builds and were already in the program the records measured, but new CMS functions are out of scope for this milestone; the entry states them as what they are.
