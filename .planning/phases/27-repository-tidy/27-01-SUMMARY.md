---
phase: 27-repository-tidy
plan: 01
subsystem: repository
tags: [branches, docs, owner-steps, tidy]
requires: []
provides:
  - "docs/branches.md regenerated against origin/main ee3c9ed470 with a Decisions table (full tips and subjects)"
  - "27-OWNER-STEPS.md: lease-guarded deletes for nine branches, GitHub-UI path, bundle and local retirements"
  - "TIDY-01 and TIDY-04 held at Pending owner"
affects: [27-04, 27-05]
key-files:
  created:
    - .planning/phases/27-repository-tidy/27-OWNER-STEPS.md
  modified:
    - docs/branches.md
    - .planning/REQUIREMENTS.md
decisions:
  - "Nothing deleted, pushed or tagged: every Delete row reads pending"
status: complete
metrics:
  completed: 2026-10-26
actuals:
  tokens: 8300
  tasks: 3
  commits: 3
plan_head_before: 3b06411496a69b7992b039e5f5cd2af5351bfac4
plan_head_after: 51eb170a1f09d3968c01b21cab8d7f2c49b4cfe9
---

# Phase 27 Plan 01: Branch decisions and owner steps Summary

Evidence for all eleven remote branches recomputed against origin/main `ee3c9ed470`, one decision per branch with its full 40-character tip and subject recorded before any deletion exists, and a copy-pasteable, lease-guarded step list for the owner. The remote is exactly as it was.

## What was done

1. **Tracer, `holzbar-colours` end to end** (`0f501a1`): fresh evidence row (152 / 0, no cherry line, #33 merged), first Decisions row, and block B-1 of the owner steps. `27-check.sh branches --only holzbar-colours` and the two greps passed before expanding.
2. **All eleven branches** (`5747757`): evidence table regenerated, Decisions table with nine `Delete` / `pending`, `release` `Keep` / `kept`, `fix/bausteine-befunde` `In use by a session`. Prose brought in line (second paragraph, vocabulary paragraph, "How the table was made" with `git rev-parse origin/BRANCH`, fourth case in "The verdicts", UI-section cautions, restore section, closing paragraph).
3. **Owner steps and requirements** (`51eb170`): Parts A to E in `27-OWNER-STEPS.md` (37 `Expected` blocks), TIDY-01 and TIDY-04 `Pending owner` with open boxes and the explanatory paragraph.

## Measured against the plan's facts

The eleven branch names and every verdict matched the plan (seven Safe to delete, two Review first, `release` Keep, `fix/bausteine-befunde` in use). Differences found by measuring again:

- `fix/bausteine-befunde` now reads 13 / 0 with no cherry line and its pull request #47 is merged, so git shows it holds nothing main lacks. Per D-04 it still carries no verdict and is untouched; the Reason says so plainly.
- Local `main` is 50 commits ahead of and 3 behind origin/main (`3 50`). The 3 are the merge of #47 and its two commits, which is why `git cherry main origin/fix/bausteine-befunde` still shows two `+`. For every other branch the local-main cherry equals the origin/main one; the page's sentence now says this.
- Other values moved with main (only-in-main counts, `release` tip `4d238b97d1`, date 2026-10-15). No branch appeared or vanished; no `Undecided` row.
- `docs/refresh-2026-10-20` (`cfdeb69`) is an ancestor of both main and origin/main; `git worktree list` shows its worktree as prunable.

## Rehearsals (no push anywhere)

- Bundle of `backup/lokal-vor-origin-abgleich`: 26349797 bytes, `git bundle verify` printed `The bundle records a complete history.`, sha256 `bdcd2fd9ff164b1d1cd59e6ed68a36f7509e5dba00155ea07ead512cfa5b2924`; restore via `git fetch BUNDLE refs/heads/...` into a fresh repository printed `* [new branch]` and the tip `ef4873ad07...`. The bundle file was deleted again.
- `git branch -D` of the backup branch and `git worktree prune` plus `git branch -d docs/refresh-2026-10-20` were run only in a `mktemp -d` clone (`Deleted branch ... (was ef4873a).`, `... (was cfdeb69).`; without prune git refuses because a worktree entry exists). The real checkout was verified unchanged afterwards.
- The delete-command outputs in the owner steps are the planner's rehearsal on a throwaway bare repository, as the plan instructed; I did not run any `git push`.

## Deviations from Plan

**1. [Rule 3 - Blocking] Worktree reset.** HEAD was `ee3c9ed` (not a descendant of `3b06411`) with a clean tree, so I ran `git reset --hard 3b06411` as the orchestrator instructed.

**2. [Scope] No STATE.md, ROADMAP.md or requirements mark-complete.** The orchestrator restricted me to the three plan files because plan 27-02 edits ROADMAP.md and milestones in parallel. I therefore did not run `state.advance-plan`, `roadmap.update-plan-progress` or `requirements.mark-complete` (the last would also have wrongly flipped TIDY-01/04 away from `Pending owner`). The orchestrator must advance STATE.md and the ROADMAP progress row after merging.

**3. [Rule 1 - Wording] Extra caution and expected-output notes.** In the UI section I added a fourth caution for `fix/bausteine-befunde` (D-04) next to the three the plan implied; in Part A the `gh auth status` expectation deliberately omits the token line (it prints a token prefix, which the plan's secret scan forbids in the file).

**4. [Wording] "Expected" for `gh pr list --state open`.** It printed nothing when captured non-interactively; in a terminal `gh` may add a "no open pull requests" line. The file says both.

`27-check.sh` was not edited. Wrapper scripts for commands the worktree guard refuses live in the scratchpad, outside the repository.

## Verification

`27-check.sh branches`, `owner-steps` and `requirements` pass; the two `Pending owner` greps pass; no token pattern in the two files; `git ls-remote --heads origin` equals the before-snapshot (`REMOTE_UNCHANGED`); `git status --short` listed only the plan's files.

## Known Stubs

None.

## Threat Flags

None. The only remote-facing commands are `git fetch origin --prune`, `git ls-remote` and read-only `gh`.

## Self-Check: PASSED

Files `docs/branches.md`, `27-OWNER-STEPS.md`, `REQUIREMENTS.md` exist; commits `0f501a1`, `5747757`, `51eb170` are ancestors of HEAD.
