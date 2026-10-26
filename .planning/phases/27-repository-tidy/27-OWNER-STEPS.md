# Phase 27: the owner's steps

Nothing in this file has been run against the real remote. It was written without a single `git push`, and the expected output of each delete command was seen only on a throwaway repository. Every command here needs your own credentials and your say-so at the moment you run it. The tips and names come from the Decisions table of `docs/branches.md`; if a tip there no longer matches the remote, the lease in the command makes git refuse and nothing is deleted.

`URL` in the expected outputs below is what `git remote get-url origin` prints, here `https://github.com/holzcloud/holzcloud-cms.git`. GitHub's own wording on the `To` line carries the repository URL, and lines beginning `remote:` are GitHub's: they are harmless unless they say denied or protected.

## Part B: deleting the remote branches

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

The lease makes git refuse the delete if the branch moved since the tip was recorded.

#### A different output means

- **Stale info.** `To URL`, then ` ! [rejected]        (delete) -> holzbar-colours (stale info)`, then `error: failed to push some refs to 'URL'`, exit 1, nothing deleted. The branch moved after its tip was recorded. Stop, tell the session; the Tip in `docs/branches.md` no longer holds.
- **Already gone.** `error: unable to delete 'holzbar-colours': remote ref does not exist`, then the same `failed to push` line, exit 1. Nothing to do; tell the session so it records the result.
