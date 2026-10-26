# Remote branches

This page lists every branch on the remote besides `main` and says, for each, whether it can go. The verdicts rest on evidence in `main` and on nothing else: not on a branch's age, and not on its name. Most of these branches are orphans: their work was merged long ago, usually as a squash merge, and nothing has needed them since.

Deleting a branch needs the owner's own credentials and say-so at the moment it happens. The 403 that the development environment's proxy answers to the deletion of a ref belongs to that proxy; it is no reason to think this Mac is refused. This page turns "delete the orphan branches" into a checklist and a record. Nothing has been deleted, pushed or tagged in making it. The commands, with the output each one prints, are in `.planning/phases/27-repository-tidy/27-OWNER-STEPS.md`.

## The table

Generated 2026-10-26 against origin/main ee3c9ed470.

The verdicts hold for that commit. If `main` has moved since, regenerate the table with the commands below before deleting anything. Pull requests were read with `gh`, which was authenticated when the table was made, so the PRs column is complete as of that date; no pull request is open.

| Branch | Last commit | Tip | Only in main / only in branch | Cherry | PRs | Verdict | Reason |
|---|---|---|---|---|---|---|---|
| `backfill-releases` | 2026-09-15 | `4b14672344` | 270 / 1 | 1 + | none | **Review first** | One commit of its own that is not in main: "One-shot, second push: finish v2.1 and v2.2". It adds a push-triggered workflow with write access; the facts are under "Branches to look at first". |
| `claude/holzcloud-projects-update-0t9epb` | 2026-10-13 | `9cc7bcb7bd` | 112 / 0 | none ahead | #13 merged | Safe to delete | An ancestor of main: it has no commit that main lacks, and pull request #13 is merged. A session may still be working on it, so check before deleting. |
| `claude/project-open-tasks-lwuj02` | 2026-10-11 | `58b0963a49` | 118 / 4 | 4 - | 21 merged (#7 to #42), #12 closed | Safe to delete | All four commits of its own are patch-equivalent to commits in main (a squash merge); the last two pull requests from it, #41 and #42, are merged. The name was reused for 22 pull requests, so a session may well be working on it: check before deleting. |
| `claude/task-phases-milestones-open-mt1qz0` | 2026-09-14 | `044d51c1d9` | 277 / 0 | none ahead | none | Safe to delete | An ancestor of main: it has no commit that main lacks. A session may still be working on it, so check before deleting. |
| `feat/holzwave-farben` | 2026-10-10 | `88e9450a9d` | 123 / 1 | 1 - | #40 merged | Safe to delete | Its one commit is patch-equivalent to a commit in main; pull request #40 is merged. |
| `fix/bausteine-befunde` | 2026-10-20 | `fa696f6368` | 13 / 0 | none ahead | #47 merged | In use by a session | The branch of another running session. Git shows no commit that main lacks and pull request #47 from it is merged, but the rule does not decide it: it is left alone until that session is finished. |
| `fix/tables-mobile` | 2026-10-09 | `40c9422fd9` | 124 / 1 | 1 - | #39 merged | Safe to delete | Its one commit is patch-equivalent to a commit in main; pull request #39 is merged. |
| `holzbar-colours` | 2026-10-02 | `dce6ed2029` | 152 / 0 | none ahead | #33 merged | Safe to delete | An ancestor of main; pull request #33 is merged. |
| `redirects-keep-language` | 2026-10-02 | `42c4c20da6` | 150 / 0 | none ahead | #34 merged | Safe to delete | An ancestor of main; pull request #34 is merged. |
| `release` | 2026-10-15 | `4d238b97d1` | 15 / 0 | none ahead | none | Keep | The button of route 2 in `.github/workflows/release.yml`: a push to this branch starts a release. It holds no work of its own and is an ancestor of main by design, so it is not an orphan. Do not delete it. |
| `tagfix` | 2026-09-26 | `98791fbc4b` | 219 / 1 | 1 + | none | **Review first** | One commit of its own that is not in main: "ci: one-off tagfix to point tags at the rewritten history". It adds a push-triggered workflow with write access; the facts are under "Branches to look at first". |

The cherry check was repeated against the local `main`. The local `main` is ahead of origin/main by 50 planning commits that have not been pushed, and behind it by 3 (`git rev-list --left-right --count origin/main...main` prints `3 50`). The result was the same for every branch except `fix/bausteine-befunde`: the local `main` does not yet hold the merge of pull request #47, so its two commits still read `+` there. That branch carries no verdict.

## Decisions

| Branch | Verdict | Decision | Tip | Subject of the tip | Result |
|---|---|---|---|---|---|
| `backfill-releases` | Review first | Delete | `4b14672344539906609fa9030c19d9b4f2ef9bb6` | One-shot, second push: finish v2.1 and v2.2 | pending |
| `claude/holzcloud-projects-update-0t9epb` | Safe to delete | Delete | `9cc7bcb7bddfaac551c74fb39306632e79170eac` | docs: slim the README, put four screenshots in it, move the admin tour to publishing.md | pending |
| `claude/project-open-tasks-lwuj02` | Safe to delete | Delete | `58b0963a49c7f7a91791c8b277d556a99f082436` | docs(quick-261011-o5v): Menüeinträge bearbeiten, Handy-Layout | pending |
| `claude/task-phases-milestones-open-mt1qz0` | Safe to delete | Delete | `044d51c1d9d92dc933155cbf3113631e742043f2` | Open milestone v2.3: every word on a page belongs to whoever reads it | pending |
| `feat/holzwave-farben` | Safe to delete | Delete | `88e9450a9da4a2093490c866f7ea39e0d53d1032` | Theme holzcloud: Farben für die Programmseite holzWave | pending |
| `fix/bausteine-befunde` | In use by a session | In use by a session | `fa696f6368dc64911c16ebae3bed396802e6cdf2` | refactor(weide): die Aufruf-Regel steht jetzt im Kern | in use by a session |
| `fix/tables-mobile` | Safe to delete | Delete | `40c9422fd9480f95867d3a1fbc4c10685432963a` | Theme holzcloud: Tabellen auf dem Handy scrollen im eigenen Kasten | pending |
| `holzbar-colours` | Safe to delete | Delete | `dce6ed2029263d627398bf237fd40ec6b080fc06` | holzcloud theme: the holzbar page keeps the ice colours (0.0.18) | pending |
| `redirects-keep-language` | Safe to delete | Delete | `42c4c20da6364584fd5863ff45b1255a3c942610` | Redirects keep the visitor's language (0.0.19) | pending |
| `release` | Keep | Keep | `4d238b97d104296469a480e3b701d7a1f8a1c712` | docs(phase-23): add security threat verification | kept |
| `tagfix` | Review first | Delete | `98791fbc4b6e8eef72773f0dfdfb8bcce56d7081` | ci: one-off tagfix to point tags at the rewritten history | pending |

The Decision column holds one of three values: `Delete`, `Keep`, and `In use by a session`. The Result column holds one of four: `pending` (decided, not yet done), `kept`, `in use by a session`, and `deleted` followed by a date. A date in Result is the day the deletion was recorded here, not necessarily the day it happened. The Tip column is the full commit the branch pointed at when the decision was written, which is what makes a restore possible; the subject is the first line of that commit.

The two Review first branches are decided: the owner chose to delete `backfill-releases` and `tagfix`, both one-off repairs that are done. `fix/bausteine-befunde` is not touched until the session that uses it is finished.

## How the table was made

Every number comes from git and from the pull request list at the time of generation, never from memory. To regenerate it, run these commands in a clone of the repository.

```sh
git fetch origin --prune
git rev-parse --short=10 origin/main
git for-each-ref --format='%(refname:short)' refs/remotes/origin
git log -1 --format=%cs origin/BRANCH
git rev-parse --short=10 origin/BRANCH
git rev-parse origin/BRANCH
git rev-list --left-right --count origin/main...origin/BRANCH
git cherry origin/main origin/BRANCH
git log -1 --format=%s COMMIT
gh pr list --state all --limit 100 --json number,headRefName,state,title
gh pr list --state open
```

The first command brings the remote's branches up to date and forgets the ones that are gone. The second prints the commit the verdicts are measured against. The third lists the branches; leave out `origin`, `origin/HEAD` and `origin/main`. The fourth and fifth print a branch's last commit date and its tip. The sixth prints the full 40-character tip, which is what the Decisions table records. In the output of `git rev-list --left-right --count`, the left number is the commits only in `main` and the right number is the commits only in the branch. `git cherry` prints one line per commit of the branch: a `-` means a patch-equivalent commit exists in `main` (which is how a squash merge is seen through), a `+` means the commit is not in `main`, and no output means the branch has nothing ahead. For each `+` line, `git log -1 --format=%s` with that commit's SHA prints the subject. The last two commands list the pull requests per head branch and any that are still open.

## The verdicts

There are three verdicts, given by this rule and by nothing else, and a fourth case that the rule does not decide.

**Keep** is `release` alone. It is the button of route 2 in `release.yml`, it holds no work of its own, and it is an ancestor of `main` by design.

**Review first** goes to every branch with at least one `+` commit in `git cherry origin/main origin/BRANCH`, a commit that is in no form in `main`. The Reason names the subject of each such commit and judges nothing about it: the work is unmerged, and deciding on unmerged work is not Claude's to do.

**Safe to delete** goes to every other branch, and only because the evidence says so: either the branch has no commit of its own (it is an ancestor of `main`), or every commit it has is patch-equivalent to one in `main`. The Reason names which. The word safe means that the commits are in `main` and nothing is lost with the branch. It says nothing about whether anyone still wants the branch name.

**In use by a session** is not a verdict. It marks a branch that another running session is working on. The rule is not applied to it, whatever git shows, and the branch is not deleted or changed until that session is finished.

Every `claude/` branch is judged by the rule like any other, and its Reason adds that a session may still be working on it, because a session that is working on a branch loses it when the branch is deleted under it. Whether one is working there is not something git can show, so the owner checks.

## Deleting a branch in the GitHub UI

These steps are for the owner, in a browser, and the page names the buttons as GitHub words them at the time of writing.

1. Open the repository on GitHub.
2. Open the branch dropdown above the file list and choose View all branches. This is the Branches page, which is also reached under the Code tab.
3. Find the branch in the list.
4. Click the trash icon at the end of its row, and confirm if GitHub asks.

There is a second route for a branch whose pull request is closed or merged: open the repository's Pull requests, click Closed, open the pull request, and click Delete branch near the bottom of it. The button is not shown while an open pull request exists for the same branch, and deleting the head branch of an open pull request closes that pull request. When the table was regenerated on 2026-10-26, `gh pr list --state open` answered with nothing, so no open pull request exists; run it again before deleting.

Four cautions apply.

- `release` is Keep and is not deleted: it is the button of the release workflow, and a push to it starts a release.
- Each `claude/` branch is deleted only after the owner has checked that no session is working on it.
- `fix/bausteine-befunde` is in use by a session and is not deleted.
- The two Review first branches, `backfill-releases` and `tagfix`, are decided: both are deleted (see the Decisions table for their tips and subjects). The facts under "Branches to look at first" stay as the reason.

The GitHub UI does not check the tip. Before clicking, compare the branch's latest commit there with the Tip column of the Decisions table.

## Restoring a deleted branch

The Tip column of the Decisions table is what makes a deletion recoverable: it is the full commit the branch pointed at. Keep a copy of the table before deleting. There are two routes, and they differ in how well they are known.

**Route A, for a branch that had a pull request.** GitHub's article "Deleting and restoring branches in a pull request" (https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-branches-in-your-repository/deleting-and-restoring-branches-in-a-pull-request) was read on 2026-10-14. It says that you can restore the head branch of a closed pull request: on GitHub, open the repository's main page, click Pull requests, click Closed, click the pull request associated with the branch, and near the bottom of it click Restore branch. The article names closed pull requests. Whether a merged pull request offers the same button is not stated in the part that was read, so do not rely on it. The PRs column shows which branches had a pull request: all but `backfill-releases`, `claude/task-phases-milestones-open-mt1qz0`, `release` and `tagfix` (read again with `gh` on 2026-10-26).

**Route B, for every branch, including the four that never had a pull request and so have no Restore branch button.** Recreate the branch from the Tip SHA, from any clone that still holds the commit:

```sh
git push origin SHA:refs/heads/NAME
```

This route is unverified. How long GitHub keeps a commit that no branch reaches any more is not documented, so the Tip SHA restores the branch only as long as the commit is still available. A local clone that has fetched the branch keeps it regardless, which is the sure way to keep one.

## Branches to look at first

Two branches carry commits that are not in `main`, and each carries a workflow that starts on a push to it. The facts below were read from git (`git show origin/BRANCH:.github/workflows/FILE`); they are what the files say, not a judgement. The commits are not in `main`, and Claude does not decide on unmerged work.

Both workflow files are absent from `main`, whose `.github/workflows` holds `ci.yml`, `image.yml`, `release.yml` and `security.yml`. A workflow runs on the next push to the branch that holds it. Deleting the branch removes the trigger together with the unmerged commit.

### `backfill-releases`

- **File:** `.github/workflows/backfill.yml`, the only difference from `main` (one commit, "One-shot, second push: finish v2.1 and v2.2").
- **Trigger:** a push to the branch `backfill-releases`.
- **Write scope:** the file sets `permissions: contents: read` for the whole file, and the one job `backfill` raises it to `contents: write`.
- **What it does:** for v2.0, v2.1 and v2.2, in that order, it skips a tag that already exists, checks out the commit that closed that milestone, runs the plugin module check (not for v2.1, by a decision written in the file) and `go test ./...`, builds the linux/amd64 binary, and runs `gh release create` with `--target` at that commit, `--generate-notes`, and the binary and its checksum attached.
- **Runs seen:** `gh run list` shows runs on 2026-09-15 (one cancelled, one failed, one successful) and a successful one on 2026-09-26.
- **Why it matters now:** the preamble of `CHANGELOG.md` says that the releases and tags of the earlier count, 1.4 to 2.9.1, were deleted when the count started again. A run on a new push would find no v2.0, v2.1 or v2.2 tag and would publish those three releases again. The file's own header says it is meant to be deleted together with its branch.

### `tagfix`

- **File:** `.github/workflows/tagfix.yml`, with `.tagfix/list.txt` and seven `.tagfix/v1.N.msg` files (one commit, "ci: one-off tagfix to point tags at the rewritten history").
- **Trigger:** a push to the branch `tagfix`.
- **Write scope:** `permissions: contents: write`, set for the whole file.
- **What it does:** for each of the 17 lines of `.tagfix/list.txt` (seven annotated tags, v1.4 to v1.10, with their messages, and ten lightweight tags, v2.0 to v2.8.2) it recreates the tag at the listed commit with `git tag -f`, force-pushes all of them with `+refs/tags/NAME:refs/tags/NAME`, lists the remote's tags, and finally runs `git push origin --delete tagfix`, which deletes its own branch. All 17 listed commits exist in the repository today.
- **Runs seen:** one run on 2026-09-26, which `gh run list` reports as failed; the branch is still there.
- **Why it matters now:** as for `backfill-releases`, the preamble of `CHANGELOG.md` says the tags of the earlier count were deleted. A run on a new push would force-create 17 tags of the old count, and the remote holds no such tag today.

Both branches are decided for deletion (D-02), and the facts above stay as the reason. This page was written without deleting, pushing or tagging anything. The next step is the owner's: the deletions are the last blocks of Part B in `.planning/phases/27-repository-tidy/27-OWNER-STEPS.md`, and the table is regenerated with the commands under "How the table was made" if `main` has moved by then.
