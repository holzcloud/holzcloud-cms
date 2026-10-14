# Remote branches

This page lists every branch on the remote besides `main` and says, for each, whether it can go. The verdicts rest on evidence in `main` and on nothing else: not on a branch's age, and not on its name. Most of these branches are orphans: their work was merged long ago, usually as a squash merge, and nothing has needed them since.

Deleting a branch is the owner's job, by hand, in the GitHub web interface. The development environment's proxy answers 403 to the deletion of a ref, so a session working here cannot do it, and that is why this page exists: it turns "delete the orphan branches" into a checklist. Nothing has been deleted, pushed or tagged in making it.

## The table

Generated 2026-10-14 against origin/main 9cc7bcb7bd.

The verdicts hold for that commit. If `main` has moved since, regenerate the table with the commands below before deleting anything. Pull requests were read with `gh`, which was authenticated when the table was made, so the PRs column is complete as of that date; no pull request is open.

| Branch | Last commit | Tip | Only in main / only in branch | Cherry | PRs | Verdict | Reason |
|---|---|---|---|---|---|---|---|
| `backfill-releases` | 2026-09-15 | `4b14672344` | 158 / 1 | 1 + | none | **Review first** | One commit of its own that is not in main: "One-shot, second push: finish v2.1 and v2.2". It adds a push-triggered workflow with write access; the facts are under "Branches to look at first". |
| `claude/holzcloud-projects-update-0t9epb` | 2026-10-13 | `9cc7bcb7bd` | 0 / 0 | none ahead | #13 merged | Safe to delete | The branch points at the very commit origin/main points at, so it holds nothing main lacks. A session may still be working on it, so check before deleting. |
| `claude/project-open-tasks-lwuj02` | 2026-10-11 | `58b0963a49` | 6 / 4 | 4 - | 21 merged (#7 to #42), #12 closed | Safe to delete | All four commits of its own are patch-equivalent to commits in main (a squash merge); the last two pull requests from it, #41 and #42, are merged. The name was reused for 22 pull requests, so a session may well be working on it: check before deleting. |
| `claude/task-phases-milestones-open-mt1qz0` | 2026-09-14 | `044d51c1d9` | 165 / 0 | none ahead | none | Safe to delete | An ancestor of main: it has no commit that main lacks. A session may still be working on it, so check before deleting. |
| `feat/holzwave-farben` | 2026-10-10 | `88e9450a9d` | 11 / 1 | 1 - | #40 merged | Safe to delete | Its one commit is patch-equivalent to a commit in main; pull request #40 is merged. |
| `fix/tables-mobile` | 2026-10-09 | `40c9422fd9` | 12 / 1 | 1 - | #39 merged | Safe to delete | Its one commit is patch-equivalent to a commit in main; pull request #39 is merged. |
| `holzbar-colours` | 2026-10-02 | `dce6ed2029` | 40 / 0 | none ahead | #33 merged | Safe to delete | An ancestor of main; pull request #33 is merged. |
| `redirects-keep-language` | 2026-10-02 | `42c4c20da6` | 38 / 0 | none ahead | #34 merged | Safe to delete | An ancestor of main; pull request #34 is merged. |
| `release` | 2026-10-11 | `7878f57ff0` | 2 / 0 | none ahead | none | Keep | The button of route 2 in `.github/workflows/release.yml`: a push to this branch starts a release. It holds no work of its own and is an ancestor of main by design, so it is not an orphan. Do not delete it. |
| `tagfix` | 2026-09-26 | `98791fbc4b` | 107 / 1 | 1 + | none | **Review first** | One commit of its own that is not in main: "ci: one-off tagfix to point tags at the rewritten history". It adds a push-triggered workflow with write access; the facts are under "Branches to look at first". |

The cherry check was repeated against the local `main`, which is ahead of origin/main by planning commits that have not been pushed. It gave the same result for every branch.

## How the table was made

Every number comes from git and from the pull request list at the time of generation, never from memory. To regenerate it, run these commands in a clone of the repository.

```sh
git fetch origin --prune
git rev-parse --short=10 origin/main
git for-each-ref --format='%(refname:short)' refs/remotes/origin
git log -1 --format=%cs origin/BRANCH
git rev-parse --short=10 origin/BRANCH
git rev-list --left-right --count origin/main...origin/BRANCH
git cherry origin/main origin/BRANCH
git log -1 --format=%s COMMIT
gh pr list --state all --limit 100 --json number,headRefName,state,title
gh pr list --state open
```

The first command brings the remote's branches up to date and forgets the ones that are gone. The second prints the commit the verdicts are measured against. The third lists the branches; leave out `origin`, `origin/HEAD` and `origin/main`. The fourth and fifth print a branch's last commit date and its tip. In the output of `git rev-list --left-right --count`, the left number is the commits only in `main` and the right number is the commits only in the branch. `git cherry` prints one line per commit of the branch: a `-` means a patch-equivalent commit exists in `main` (which is how a squash merge is seen through), a `+` means the commit is not in `main`, and no output means the branch has nothing ahead. For each `+` line, `git log -1 --format=%s` with that commit's SHA prints the subject. The last two commands list the pull requests per head branch and any that are still open.

## The verdicts

There are three verdicts, given by this rule and by nothing else.

**Keep** is `release` alone. It is the button of route 2 in `release.yml`, it holds no work of its own, and it is an ancestor of `main` by design.

**Review first** goes to every branch with at least one `+` commit in `git cherry origin/main origin/BRANCH`, a commit that is in no form in `main`. The Reason names the subject of each such commit and judges nothing about it: the work is unmerged, and deciding on unmerged work is not Claude's to do.

**Safe to delete** goes to every other branch, and only because the evidence says so: either the branch has no commit of its own (it is an ancestor of `main`), or every commit it has is patch-equivalent to one in `main`. The Reason names which. The word safe means that the commits are in `main` and nothing is lost with the branch. It says nothing about whether anyone still wants the branch name.

Every `claude/` branch is judged by this rule like any other, and its Reason adds that a session may still be working on it, because a session that is working on a branch loses it when the branch is deleted under it. Whether one is working there is not something git can show, so the owner checks.
