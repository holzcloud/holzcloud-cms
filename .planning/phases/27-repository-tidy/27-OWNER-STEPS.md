# Phase 27: the owner's steps

Nothing in this file has been run against the real remote. It was written without a single `git push`, and the expected output of each delete command was seen only on a throwaway repository. Every command here needs your own credentials and your say-so at the moment you run it. The tips and names come from the Decisions table of `docs/branches.md`; if a tip there no longer matches the remote, the lease in the command makes git refuse and nothing is deleted.

`URL` in the expected outputs below is what `git remote get-url origin` prints, here `https://github.com/holzcloud/holzcloud-cms.git`. GitHub's own wording on the `To` line carries the repository URL, and lines beginning `remote:` are GitHub's: they are harmless unless they say denied or protected.

Run everything from the checkout `/Users/holz/Projects/holzcloud-cms`. `fix/bausteine-befunde` (remote and local) and `main` are never touched by any step here: `fix/bausteine-befunde` belongs to a running session and `release` stays (D-03, D-04).

## Part A: before anything

### A-1: is `gh` signed in

```sh
gh auth status
```

Expected: `Logged in to github.com account holzcloud`, `Active account: true`, and a token scope list that contains `repo`. (Do not paste the token line anywhere.)

### A-2: the remote is as recorded

```sh
git fetch origin --prune
git ls-remote --heads origin
```

Expected: the fetch prints nothing. The second command prints twelve lines, each a full SHA, a tab, and `refs/heads/NAME`: the eleven branches besides `main` with the tips of the Decisions table, and `main`. When this file was written (2026-10-26) it printed:

```
4b14672344539906609fa9030c19d9b4f2ef9bb6	refs/heads/backfill-releases
9cc7bcb7bddfaac551c74fb39306632e79170eac	refs/heads/claude/holzcloud-projects-update-0t9epb
58b0963a49c7f7a91791c8b277d556a99f082436	refs/heads/claude/project-open-tasks-lwuj02
044d51c1d9d92dc933155cbf3113631e742043f2	refs/heads/claude/task-phases-milestones-open-mt1qz0
88e9450a9da4a2093490c866f7ea39e0d53d1032	refs/heads/feat/holzwave-farben
fa696f6368dc64911c16ebae3bed396802e6cdf2	refs/heads/fix/bausteine-befunde
40c9422fd9480f95867d3a1fbc4c10685432963a	refs/heads/fix/tables-mobile
dce6ed2029263d627398bf237fd40ec6b080fc06	refs/heads/holzbar-colours
ee3c9ed470110125e726a60ecf40d91d0bb968e2	refs/heads/main
42c4c20da6364584fd5863ff45b1255a3c942610	refs/heads/redirects-keep-language
4d238b97d104296469a480e3b701d7a1f8a1c712	refs/heads/release
98791fbc4b6e8eef72773f0dfdfb8bcce56d7081	refs/heads/tagfix
```

`main` may have moved since; the tip of any branch you are about to delete must not have. If one differs from the Decisions table, stop.

### A-3: no open pull request

```sh
gh pr list --state open
```

Expected: nothing listed (on 2026-10-26 it printed nothing; in a terminal `gh` may add a line saying there are no open pull requests, which is the same answer). Deleting the head branch of an open pull request closes it, so anything but an empty list means stop.

## Part B: deleting the remote branches

Each block has three commands: look at the branch, delete it with a lease that carries the full tip, look again. The lease makes git refuse the delete if the branch moved since the tip was recorded. After a deletion, tell the session so that `docs/branches.md` records `deleted DATE` in the Result column (see Part D).

### B-1: `holzbar-colours` (the trial, D-01)

```sh
git ls-remote --heads origin holzbar-colours
```

Expected: one line, the full tip, a tab, `refs/heads/holzbar-colours`: `dce6ed2029263d627398bf237fd40ec6b080fc06	refs/heads/holzbar-colours`.

```sh
git push origin --force-with-lease=refs/heads/holzbar-colours:dce6ed2029263d627398bf237fd40ec6b080fc06 --delete holzbar-colours
```

Expected: two lines and exit 0.

```
To URL
 - [deleted]         holzbar-colours
```

```sh
git ls-remote --heads origin holzbar-colours
```

Expected: no output.

If B-1 answers 403 or any permission error, do not go on to B-2: use Part C and tell the session (D-01).

### B-2: `redirects-keep-language`

```sh
git ls-remote --heads origin redirects-keep-language
```

Expected: one line: `42c4c20da6364584fd5863ff45b1255a3c942610`, a tab, `refs/heads/redirects-keep-language`.

```sh
git push origin --force-with-lease=refs/heads/redirects-keep-language:42c4c20da6364584fd5863ff45b1255a3c942610 --delete redirects-keep-language
```

Expected: `To URL`, then ` - [deleted]         redirects-keep-language`, exit 0.

```sh
git ls-remote --heads origin redirects-keep-language
```

Expected: no output.

### B-3: `fix/tables-mobile`

```sh
git ls-remote --heads origin fix/tables-mobile
```

Expected: one line: `40c9422fd9480f95867d3a1fbc4c10685432963a`, a tab, `refs/heads/fix/tables-mobile`.

```sh
git push origin --force-with-lease=refs/heads/fix/tables-mobile:40c9422fd9480f95867d3a1fbc4c10685432963a --delete fix/tables-mobile
```

Expected: `To URL`, then ` - [deleted]         fix/tables-mobile`, exit 0.

```sh
git ls-remote --heads origin fix/tables-mobile
```

Expected: no output.

### B-4: `feat/holzwave-farben`

```sh
git ls-remote --heads origin feat/holzwave-farben
```

Expected: one line: `88e9450a9da4a2093490c866f7ea39e0d53d1032`, a tab, `refs/heads/feat/holzwave-farben`.

```sh
git push origin --force-with-lease=refs/heads/feat/holzwave-farben:88e9450a9da4a2093490c866f7ea39e0d53d1032 --delete feat/holzwave-farben
```

Expected: `To URL`, then ` - [deleted]         feat/holzwave-farben`, exit 0.

```sh
git ls-remote --heads origin feat/holzwave-farben
```

Expected: no output.

### The three `claude/*` branches

Before B-5 to B-7, confirm that no session is working on these branches. A session that is working on a branch loses it when the branch is deleted under it, and git cannot show whether one is. `claude/project-open-tasks-lwuj02` in particular was reused for 22 pull requests.

### B-5: `claude/holzcloud-projects-update-0t9epb`

```sh
git ls-remote --heads origin claude/holzcloud-projects-update-0t9epb
```

Expected: one line: `9cc7bcb7bddfaac551c74fb39306632e79170eac`, a tab, `refs/heads/claude/holzcloud-projects-update-0t9epb`.

```sh
git push origin --force-with-lease=refs/heads/claude/holzcloud-projects-update-0t9epb:9cc7bcb7bddfaac551c74fb39306632e79170eac --delete claude/holzcloud-projects-update-0t9epb
```

Expected: `To URL`, then ` - [deleted]         claude/holzcloud-projects-update-0t9epb`, exit 0.

```sh
git ls-remote --heads origin claude/holzcloud-projects-update-0t9epb
```

Expected: no output.

### B-6: `claude/project-open-tasks-lwuj02`

```sh
git ls-remote --heads origin claude/project-open-tasks-lwuj02
```

Expected: one line: `58b0963a49c7f7a91791c8b277d556a99f082436`, a tab, `refs/heads/claude/project-open-tasks-lwuj02`.

```sh
git push origin --force-with-lease=refs/heads/claude/project-open-tasks-lwuj02:58b0963a49c7f7a91791c8b277d556a99f082436 --delete claude/project-open-tasks-lwuj02
```

Expected: `To URL`, then ` - [deleted]         claude/project-open-tasks-lwuj02`, exit 0.

```sh
git ls-remote --heads origin claude/project-open-tasks-lwuj02
```

Expected: no output.

### B-7: `claude/task-phases-milestones-open-mt1qz0`

```sh
git ls-remote --heads origin claude/task-phases-milestones-open-mt1qz0
```

Expected: one line: `044d51c1d9d92dc933155cbf3113631e742043f2`, a tab, `refs/heads/claude/task-phases-milestones-open-mt1qz0`.

```sh
git push origin --force-with-lease=refs/heads/claude/task-phases-milestones-open-mt1qz0:044d51c1d9d92dc933155cbf3113631e742043f2 --delete claude/task-phases-milestones-open-mt1qz0
```

Expected: `To URL`, then ` - [deleted]         claude/task-phases-milestones-open-mt1qz0`, exit 0.

```sh
git ls-remote --heads origin claude/task-phases-milestones-open-mt1qz0
```

Expected: no output.

### The two Review first branches (D-02)

`backfill-releases` and `tagfix` each carry one commit that is not in `main` and a workflow with write access that starts on a push to the branch. `backfill-releases` would publish the releases v2.0, v2.1 and v2.2 again, and `tagfix` would force-create 17 tags of the earlier version count, because the remote holds none of them today. Both were one-off repairs that are done, and deleting the branch removes the trigger together with the commit (the full facts are under "Branches to look at first" in `docs/branches.md`). The commit of each stays reachable only through its tip SHA, so the tip and the subject are in the Decisions table: `4b14672344539906609fa9030c19d9b4f2ef9bb6` ("One-shot, second push: finish v2.1 and v2.2") and `98791fbc4b6e8eef72773f0dfdfb8bcce56d7081` ("ci: one-off tagfix to point tags at the rewritten history"). A local clone that has fetched them keeps them regardless.

### B-8: `backfill-releases`

```sh
git ls-remote --heads origin backfill-releases
```

Expected: one line: `4b14672344539906609fa9030c19d9b4f2ef9bb6`, a tab, `refs/heads/backfill-releases`.

```sh
git push origin --force-with-lease=refs/heads/backfill-releases:4b14672344539906609fa9030c19d9b4f2ef9bb6 --delete backfill-releases
```

Expected: `To URL`, then ` - [deleted]         backfill-releases`, exit 0.

```sh
git ls-remote --heads origin backfill-releases
```

Expected: no output.

### B-9: `tagfix`

```sh
git ls-remote --heads origin tagfix
```

Expected: one line: `98791fbc4b6e8eef72773f0dfdfb8bcce56d7081`, a tab, `refs/heads/tagfix`.

```sh
git push origin --force-with-lease=refs/heads/tagfix:98791fbc4b6e8eef72773f0dfdfb8bcce56d7081 --delete tagfix
```

Expected: `To URL`, then ` - [deleted]         tagfix`, exit 0.

```sh
git ls-remote --heads origin tagfix
```

Expected: no output.

### A different output means

The success form of a delete is the two lines `To URL` and ` - [deleted]         NAME`, exit 0. The delete command was tried on a throwaway bare repository on 2026-10-26 and printed exactly these. Anything else:

- **Stale info.** `To URL`, then ` ! [rejected]        (delete) -> NAME (stale info)`, then `error: failed to push some refs to 'URL'`, exit 1, nothing deleted. The branch moved after its tip was recorded. Stop, tell the session: the Tip in `docs/branches.md` no longer holds.
- **Already gone.** `error: unable to delete 'NAME': remote ref does not exist`, then the same `failed to push` line, exit 1. Nothing to do; tell the session so it records the result.
- **A 403 or any permission error.** Use the GitHub-UI path in Part C and tell the session.

Stop at the first output that is neither the success form nor one of these.

## Part C: the GitHub UI, for a branch that git refuses

The UI does not check the tip. Before clicking, compare the branch's latest commit on GitHub with the Tip column of the Decisions table in `docs/branches.md`; if they differ, stop.

1. Open the repository on GitHub.
2. Open the branch dropdown above the file list and choose View all branches. This is the Branches page, which is also reached under the Code tab.
3. Find the branch in the list.
4. Click the trash icon at the end of its row, and confirm if GitHub asks.

A second route for a branch whose pull request is closed or merged: Pull requests, Closed, open the pull request, Delete branch near the bottom. The button is not shown while an open pull request exists for the same branch.

## Part D: telling the session

After you have deleted branches by hand, the answer to the gate in plan 27-04 is "I did it myself". The session then fetches, checks the remote against the Decisions table, and records `deleted DATE` in the Result column. The page is held to the remote by `bash .planning/phases/27-repository-tidy/27-check.sh branches`.

## Part E: the two local branches (TIDY-04)

The branch `backup/lokal-vor-origin-abgleich` is the old local history (489 commits, no common ancestor with `main`). The `archiv` remote answers "Repository not found", so this branch is the only copy of that lineage. The bundle comes first, and you keep it. `docs/refresh-2026-10-20` is the planner's reading of "the stray branch in the shared checkout": its tip is an ancestor of `main`, and the worktree it was checked out in has no directory any more. Confirm that this is the branch you mean before E-4. The branches `fix/bausteine-befunde` (local) and `main` are not touched by this part.

### E-1: note the tip

```sh
git rev-parse backup/lokal-vor-origin-abgleich
```

Expected: `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f`. If it prints anything else, stop: the branch has moved since the plan was written.

### E-2: the bundle, outside the repository

```sh
mkdir -p ~/holzcloud-backup
git bundle create ~/holzcloud-backup/backup-lokal-vor-origin-abgleich.bundle backup/lokal-vor-origin-abgleich
```

Expected: no output and exit 0; the file is about 26 MB (a rehearsal on 2026-10-26 gave 26349797 bytes).

```sh
git bundle verify ~/holzcloud-backup/backup-lokal-vor-origin-abgleich.bundle
```

Expected: five lines. The first is `PATH is okay`, then:

```
The bundle contains this ref:
ef4873ad07e30ac27b02e5829ff0f1a20307ca9f refs/heads/backup/lokal-vor-origin-abgleich
The bundle records a complete history.
The bundle uses this hash algorithm: sha1
```

```sh
shasum -a 256 ~/holzcloud-backup/backup-lokal-vor-origin-abgleich.bundle
```

Expected: one line, 64 hexadecimal characters and the path. The rehearsal bundle gave `bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924`; git does not promise a byte-identical bundle on a second run, so note the value of your own file. Keep the bundle: it is the only copy of the old lineage once the branch is gone.

### E-3: retire the backup branch

```sh
git branch -D backup/lokal-vor-origin-abgleich
```

Expected: `Deleted branch backup/lokal-vor-origin-abgleich (was ef4873a).` and exit 0. (Seen in a throwaway clone of this repository on 2026-10-26; nothing was run in the real checkout.)

### E-4: retire the stray branch

```sh
git worktree prune
git branch -d docs/refresh-2026-10-20
```

Expected: `git worktree prune` prints nothing and exits 0, and it removes only worktree entries whose directory is gone. Then `Deleted branch docs/refresh-2026-10-20 (was cfdeb69).` and exit 0. The lower-case `-d` is the safety: git refuses to delete a branch that is not merged into the current branch, and this one is an ancestor of `main`. Without the prune, git refuses with `error: cannot delete branch 'docs/refresh-2026-10-20' used by worktree at '...'`, exit 1; that was seen in the rehearsal and is not harmful.

### E-5: bringing the old lineage back from the bundle

```sh
git fetch ~/holzcloud-backup/backup-lokal-vor-origin-abgleich.bundle refs/heads/backup/lokal-vor-origin-abgleich:refs/heads/backup/lokal-vor-origin-abgleich
```

Expected: `From PATH`, then ` * [new branch]      backup/lokal-vor-origin-abgleich -> backup/lokal-vor-origin-abgleich`, and the branch points at `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f` again. (Seen on 2026-10-26 in a fresh throwaway repository.) The old lineage is never merged into `main`: single features are ported, not the branch.

## Done in plan 27-05

Date: 2026-10-26. The owner's answer at the gate: `retire-both` (nothing is missing from the backup branch, and `docs/refresh-2026-10-20` is the stray branch; the owner confirmed that reading).

Retired, in this order:

- `docs/refresh-2026-10-20`, tip `cfdeb691992a9fa2ccbfde7552c2fc9c9e2d54a0` (an ancestor of `main`): `git worktree prune`, then `git branch -d` printed `Deleted branch docs/refresh-2026-10-20 (was cfdeb69).`
- `backup/lokal-vor-origin-abgleich`, tip `ef4873ad07e30ac27b02e5829ff0f1a20307ca9f`, 489 commits: bundled, verified (`The bundle records a complete history.`), restored into a throwaway repository (same tip, 489 commits), then `git branch -D` printed `Deleted branch backup/lokal-vor-origin-abgleich (was ef4873a).`

Bundle: /Users/holz/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle sha256 bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924 (489 commits, 26349797 bytes)

Keep the bundle: it is the only copy of the old lineage. E-5 above brings the branch back from it.
