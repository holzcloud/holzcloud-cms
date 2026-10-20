---
phase: 23-repository-tidy-and-beta-criteria
plan: 03
subsystem: docs
tags: [branches, repository-tidy, github-ui, restore, workflows]

requires:
  - phase: 23-repository-tidy-and-beta-criteria
    provides: beta criteria and 0.1.0 entry (plans 23-01, 23-02)
provides:
  - docs/branches.md, the generated table of the ten remote branches besides main with an evidence-based verdict each
  - the GitHub-UI deletion steps, the restore routes with their verification status, and the facts of the two push-triggered workflows on unmerged branches
affects: [23-04 (owner steps may point at docs/branches.md)]

actuals:
  tokens: 6500
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Verdicts by git cherry patch-equivalence against origin/main, recomputed by the verify command; never by age or name"

key-files:
  created:
    - docs/branches.md
  modified: []

key-decisions:
  - "release is Keep (button of release.yml route 2); backfill-releases and tagfix are Review first (a + commit each, each with a push-triggered workflow holding contents: write); the other seven are Safe to delete"
  - "claude/holzcloud-projects-update-0t9epb is Safe to delete because its tip is the very commit origin/main points at (0 / 0); it had moved since the research"
  - "The Restore branch route cites GitHub's article read 2026-10-14 and claims it only for closed pull requests; the recreate-from-SHA route is marked unverified"
  - "The GitHub article on creating and deleting branches could not be fetched (404), so the View all branches and trash icon steps are written as GitHub words them at the time of writing, without a citation"

requirements-completed: [REPO-03]

plan_head_before: fdf7b43f07acd9bd9c2c9aeab02d13d6caebee8e
plan_head_after: 7611a91087b17639689cf1b5eff9f62e9c123f31
status: complete
---

# Phase 23 Plan 03: Remote branches page Summary

**docs/branches.md lists the ten remote branches besides main with a verdict each that the verify command recomputes from git (tip SHA, branch-only count, cherry evidence), the GitHub-UI deletion steps, two restore routes with their verification status, and the workflow facts of tagfix and backfill-releases; nothing was deleted, pushed or tagged.**

## Accomplishments

- Task 1: `git fetch origin --prune`, then for every branch the tip, the left/right count, `git cherry` and the pull requests per head branch from `gh`. Generated 2026-10-14 against origin/main 9cc7bcb7bd. Result: `release` Keep; `backfill-releases` and `tagfix` Review first (one `+` commit each, subjects named); `claude/holzcloud-projects-update-0t9epb`, `claude/project-open-tasks-lwuj02`, `claude/task-phases-milestones-open-mt1qz0`, `feat/holzwave-farben`, `fix/tables-mobile`, `holzbar-colours`, `redirects-keep-language` Safe to delete, each with its evidence (ancestor, patch-equivalent, merged pull request numbers). The cherry check against local main (ahead of origin/main by unpushed planning commits) gave the same result. No open pull request exists.
- Task 2: deletion steps (branches page and closed-pull-request page), the three cautions, the open-pull-request check; restore Route A cited to GitHub's article read 2026-10-14 (claimed for closed pull requests only), Route B marked unverified; one subsection each for `backfill-releases` and `tagfix` with file, trigger, write scope, effect and the runs `gh run list` shows (backfill: 2026-09-15 and 2026-09-26; tagfix: one failed run on 2026-09-26, branch still present). The page notes that a run on a new push would recreate releases and tags that the CHANGELOG preamble says were deleted, and that the owner decides.

## Task Commits

1. Task 1: audit and evidence table - `01bb769`
2. Task 2: deletion steps, restore routes, Review first facts - `7611a91`

## Files Created/Modified

- `/Users/cheidenreich/privat/holzcloud-cms/docs/branches.md` - new, seven level-two sections as specified

## Deviations from Plan

None - plan executed exactly as written.

Notes: the plan's example claimed `claude/holzcloud-projects-update-0t9epb` had moved; it now equals origin/main exactly, so its row reads `0 / 0`, `none ahead`. The sentinel-based incremental write was used while drafting and removed before the first commit.

## Issues Encountered

GitHub's article "Creating and deleting branches within your repository" answered 404, so the Branches-page steps carry no citation (the pull-request route is cited from the article that was read).

## Known Stubs

None.

## Threat Flags

None. Only a Markdown document was added. T-23-09 to T-23-11 mitigated as planned.

## Verification Run

- `branch-table-matches-git` (10 rows), `branches-structure-ok`, `branches-steps-ok`, `review-first-facts-ok`, `gates-clean` all printed.
- i18n gate `0 open, 0 orphaned`; Go files, go.mod, go.sum, `.github` equal 9db196b; evidence-set files equal bf74496.
- No remote ref was changed: no push, no delete, no tag.

## Self-Check: PASSED

- FOUND: docs/branches.md
- FOUND commits: 01bb769, 7611a91
