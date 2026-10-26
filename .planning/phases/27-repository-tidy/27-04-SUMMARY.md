---
phase: 27-repository-tidy
plan: 04
subsystem: repository
tags: [branches, deletion, owner-gate, tidy]
requires: ["27-01", "27-03"]
provides:
  - "nine remote branches deleted from origin, each with a lease on its recorded full tip"
  - "docs/branches.md: nine Decisions rows read deleted 2026-10-26"
  - "TIDY-01 Complete"
affects: [27-05]
key-files:
  modified:
    - docs/branches.md
    - .planning/REQUIREMENTS.md
decisions:
  - "Gate answer delete-all-nine (owner, 2026-10-26) carried out in full"
status: complete
metrics:
  completed: 2026-10-26
actuals:
  tokens: 2200
  tasks: 2
  commits: 1
plan_head_before: 38afd1b608f5c3f654d7b36cc00f91df272f84d9
plan_head_after: cf9ac88cb5311ef3ab9ee07450021099eeb8cd1b
---

# Phase 27 Plan 04: Delete the remote branches Summary

All nine remote branches of the Decisions table are gone from origin, deleted one at a time with a lease on the recorded full tip; `docs/branches.md` records each as `deleted 2026-10-26` and TIDY-01 reads Complete.

## The gate (Task 1)

Answer: `delete-all-nine`, given by the owner through AskUserQuestion on 2026-10-26. Choosing it also confirms that no session works on the three `claude/*` branches. `fix/bausteine-befunde`, `release`, `main` and all tags were in no option and were not touched.

## What was done (Task 2)

Base: the worktree was at `ee3c9ed` (clean); main's HEAD (read from the main checkout's `.git/refs/heads/main`, because the worktree guard refuses `git -C`) was `38afd1b608f5c3f654d7b36cc00f91df272f84d9`, so `git reset --hard 38afd1b` was run on the clean tree.

Preflight (all read-only):

- `git fetch origin --prune`: no output, exit 0.
- `bash .planning/phases/27-repository-tidy/27-check.sh branches`: `27-CHECK branches: PASS` (every recorded tip equals the remote tip).
- `gh pr list --state open`: no output, exit 0.

Order and output. For each branch `git ls-remote --heads origin NAME` first printed exactly the recorded tip; the push was `git push origin --force-with-lease=refs/heads/NAME:FULLSHA --delete NAME`, which printed two lines and exit 0; `git ls-remote --heads origin NAME` afterwards printed nothing (exit 0). Every push printed `To https://github.com/holzcloud/holzcloud-cms.git` followed by the line named here, and no `remote:` line. The row was set to `deleted 2026-10-26` before the next push.

| # | Branch | Recorded tip | Push printed |
|---|--------|--------------|--------------|
| 1 (trial) | `holzbar-colours` | `dce6ed2029263d627398bf237fd40ec6b080fc06` | ` - [deleted]         holzbar-colours` |
| 2 | `redirects-keep-language` | `42c4c20da6364584fd5863ff45b1255a3c942610` | ` - [deleted]         redirects-keep-language` |
| 3 | `fix/tables-mobile` | `40c9422fd9480f95867d3a1fbc4c10685432963a` | ` - [deleted]         fix/tables-mobile` |
| 4 | `feat/holzwave-farben` | `88e9450a9da4a2093490c866f7ea39e0d53d1032` | ` - [deleted]         feat/holzwave-farben` |
| 5 | `claude/holzcloud-projects-update-0t9epb` | `9cc7bcb7bddfaac551c74fb39306632e79170eac` | ` - [deleted]         claude/holzcloud-projects-update-0t9epb` |
| 6 | `claude/project-open-tasks-lwuj02` | `58b0963a49c7f7a91791c8b277d556a99f082436` | ` - [deleted]         claude/project-open-tasks-lwuj02` |
| 7 | `claude/task-phases-milestones-open-mt1qz0` | `044d51c1d9d92dc933155cbf3113631e742043f2` | ` - [deleted]         claude/task-phases-milestones-open-mt1qz0` |
| 8 | `backfill-releases` | `4b14672344539906609fa9030c19d9b4f2ef9bb6` | ` - [deleted]         backfill-releases` |
| 9 | `tagfix` | `98791fbc4b6e8eef72773f0dfdfb8bcce56d7081` | ` - [deleted]         tagfix` |

The trial push did not answer 403; no stop rule fired.

Close:

- `git fetch origin --prune` (exit 0), then `git ls-remote --heads origin` printed exactly three lines:
  `fa696f6368dc64911c16ebae3bed396802e6cdf2 refs/heads/fix/bausteine-befunde`, `ee3c9ed470110125e726a60ecf40d91d0bb968e2 refs/heads/main`, `4d238b97d104296469a480e3b701d7a1f8a1c712 refs/heads/release`.
- `27-check.sh branches`: PASS. `27-check.sh requirements`: PASS (`TIDY-01 reads Complete and no decision is pending`).
- The sentence "Rows marked deleted keep the evidence row as it was measured on the day of the table." was added under the Decisions table.
- TIDY-01: box `[x]`, cell `Complete`, by scoped edit of `.planning/REQUIREMENTS.md`; one sentence was added to the explanatory paragraph under the traceability table so that it no longer says TIDY-01 is pending.

Commit: `cf9ac88` (`docs(27-04): record the deletion of nine remote branches, TIDY-01 complete`).

## Restoring

Each tip stays in the Decisions table; route B in `docs/branches.md` (`git push origin SHA:refs/heads/NAME`) recreates a branch for as long as GitHub still holds the commit.

## Still pending

Nothing in this plan. TIDY-04 (local branches, plan 27-05) is unchanged at `Pending owner`. The three Decisions rows that are not Delete (`release` kept, `fix/bausteine-befunde` in use by a session) are as before.

## Pushes that ran

Exactly nine, all of the form `git push origin --force-with-lease=refs/heads/NAME:FULLSHA --delete NAME`. No tag and no other ref was pushed; main was not pushed.

## Deviations from Plan

**1. [Rule 3 - Blocking] Worktree reset.** HEAD was `ee3c9ed` with a clean tree and the plan files did not exist there; reset to main's `38afd1b` as the orchestrator instructed.

**2. [Scope] No STATE.md or ROADMAP.md update, no `requirements.mark-complete`.** The orchestrator restricted the work to this plan's files; TIDY-01 was set by scoped edit as the plan says. The orchestrator must advance STATE.md and the ROADMAP progress row after merging.

**3. [Wording] Extra sentence in REQUIREMENTS.md.** The paragraph under the traceability table named TIDY-01 as `Pending owner`; one sentence now says it is Complete (it would otherwise be false).

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

`docs/branches.md` and `.planning/REQUIREMENTS.md` exist; commit `cf9ac88` is an ancestor of HEAD; `git ls-remote --heads origin` lists `release`, `fix/bausteine-befunde` and `main` only.
