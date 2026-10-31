---
phase: 27-repository-tidy
verified: 2026-10-26T18:10:00Z
status: passed
score: 4/4 must-haves verified
covered_files:
  - .planning/phases/27-repository-tidy/27-01-PLAN.md
  - .planning/phases/27-repository-tidy/27-01-SUMMARY.md
  - .planning/phases/27-repository-tidy/27-02-PLAN.md
  - .planning/phases/27-repository-tidy/27-02-SUMMARY.md
  - .planning/phases/27-repository-tidy/27-03-PLAN.md
  - .planning/phases/27-repository-tidy/27-03-SUMMARY.md
  - .planning/phases/27-repository-tidy/27-04-PLAN.md
  - .planning/phases/27-repository-tidy/27-04-SUMMARY.md
  - .planning/phases/27-repository-tidy/27-05-PLAN.md
  - .planning/phases/27-repository-tidy/27-05-SUMMARY.md
  - .planning/phases/27-repository-tidy/27-check.sh
  - docs/branches.md
covered_digest: "v3:sha256:807ede242866b1a4163b7144704950f2d3dc0ffc73d1c4432b16289d5b4e6a2b"
behavior_unverified: 0
overrides_applied: 0
re_verification: false
---

# Phase 27: Repository tidy Verification Report

**Phase Goal:** No stray branch, one current position in STATE.md, v0.1.0 archived.
**Verified:** 2026-10-26
**Status:** passed
**Re-verification:** No, initial verification

All evidence below was measured independently of the SUMMARY files: live `git ls-remote`, `git branch`, `git bundle verify`, `shasum`, `state.validate`, `gh release view`, and a re-run of `27-check.sh all`.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every row of `docs/branches.md` has a recorded decision and the remote list matches it | VERIFIED | `git ls-remote --heads origin` prints exactly `main` (ee3c9ed470), `release` (4d238b97d1), `fix/bausteine-befunde` (fa696f6368). The Decisions table has 11 rows: 9 `deleted 2026-10-26`, `release` kept, `fix/bausteine-befunde` in use by a session. The three survivors match the three kept rows, and their tips equal the recorded tips. The nine deleted names are absent from the remote. `27-check.sh all` confirms evidence and decisions list the same branches, and "branches: the page agrees with git". |
| 2 | `state.advance-plan` no longer refuses with `ambiguous_position_phase` | VERIFIED | `STATE.md` has one `## Current Position` heading (line 205). `state.validate` returns `{"valid": true, "warnings": []}`. `27-check.sh` plays `state.advance-plan` on a copy and it answers without refusing, with no S002. I did not run the mutating verb on the real file. |
| 3 | `milestones/v0.1.0-ROADMAP.md`, `-REQUIREMENTS.md` and its audit exist, and ROADMAP.md carries one line for v0.1.0 | VERIFIED | All three files are present in `.planning/milestones/`. `ROADMAP.md` line 13 is the single Milestones line for v0.1.0 and points at the archive. The check script confirms the phase 20 to 23 detail (104 lines) and Progress rows 1 to 23 are byte for byte in the archive, and ROADMAP.md holds no phase 20 to 23 detail. The audit opens "This audit was written after the release, on 2026-10-26". It gives the release date as 2026-10-18, which matches `gh release view v0.1.0` (`publishedAt` 2026-10-18T15:08:54Z). It names all 13 requirements, and 14 `Record:` quotes were found by `grep -F` in the files they name. |
| 4 | The backup branch and the stray branch in the shared checkout are gone | VERIFIED | `git branch` lists only `main` and `fix/bausteine-befunde`. `git worktree list` shows one worktree. The bundle `~/holzcloud-cms-lokal-vor-origin-abgleich-ef4873ad07.bundle` verifies: "okay", ref `refs/heads/backup/lokal-vor-origin-abgleich` at `ef4873ad07`, complete history. Its sha256 `bdcd2fd9ff...5b2924` matches the value recorded in `27-OWNER-STEPS.md`. `fix/bausteine-befunde` is untouched locally and on the remote, as D-04 requires. |

**Score:** 4/4 truths verified.

### STATE-HISTORY verbatim check

The old `STATE.md` at commit `58b2546786` has 650 lines. A sorted unique-line comparison against the current `STATE.md` plus `milestones/STATE-HISTORY.md` leaves exactly one line from the old file unaccounted for (`completed_phases: 26`). That is the frontmatter counter, which the plan said it would change to 3 of 5. The check script's own line-by-line test passes. The current `STATE.md` is 237 lines and `STATE-HISTORY.md` is 500 lines. `STATE-HISTORY.md` names the base commit and carries a block-by-block table of where each part went.

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| TIDY-01 | Each remote branch decided and recorded | SATISFIED | Truth 1. `REQUIREMENTS.md` reads `[x]` and Complete. The check script confirms no decision is pending. |
| TIDY-02 | One current position in STATE.md | SATISFIED in the codebase | Truth 2. `REQUIREMENTS.md` still reads `[ ]` and Pending (see Bookkeeping). |
| TIDY-03 | v0.1.0 archived | SATISFIED in the codebase | Truth 3. `REQUIREMENTS.md` still reads `[ ]` and Pending (see Bookkeeping). |
| TIDY-04 | Local backup and stray branches retired | SATISFIED | Truth 4. `REQUIREMENTS.md` reads `[x]` and Complete. |

No orphaned requirements. The `REQUIREMENTS.md` checkboxes, status cells and `docs/branches.md` agree (`27-check.sh requirements` passes).

### Behavioral Spot-Checks and Probes

| Check | Result | Status |
|-------|--------|--------|
| `bash 27-check.sh all` | `27-CHECK all: PASS`, exit 0, about 50 `ok:` lines and no failure | PASS |
| `git bundle verify` | okay | PASS |
| `state.validate` | valid, no warnings | PASS |
| `roadmap.analyze` | phase_count 5 | PASS |

### Push state

`git status -sb` reports `main` ahead of `origin/main` by 66 and behind by 3. The 3 are the merge of PR #47 and its two commits, which local `main` does not yet hold. This matches what `docs/branches.md` and `STATE.md` say: they report push state as measured at a past date and say that pushing `main` is the owner's step. Nothing in the docs claims `main` was pushed. The remote `main` is ee3c9ed470, the commit the page was generated against. **Not a gap.**

### Anti-Patterns

No code was changed in this phase, so the anti-pattern scan does not apply. `docs/branches.md` and the planning files carry no TBD, FIXME or XXX markers that I found in the files I read.

## Bookkeeping (non-blocking; for the orchestrator's phase-close step)

These are stale status markers, not failures of the phase goal:

1. `REQUIREMENTS.md` still marks TIDY-02 and TIDY-03 as `[ ]` and Pending, although the work is present and verified. They should flip to Complete when the phase closes.
2. `STATE.md` Current Position still reads "Plan: 3 of 5 ... Last activity: plan 27-03", although plans 27-04 and 27-05 are done and have SUMMARY files.
3. The ROADMAP.md Phase 27 plan checkboxes are all `[ ]`.
4. `REQUIREMENTS.md` line 92 still says TIDY-01 and TIDY-04 "read `Pending owner`". The same paragraph then states that both became Complete on 2026-10-26. The first sentence is now historical and could be reworded.
5. The intro of `docs/branches.md` still says "Nothing has been deleted, pushed or tagged in making it". This is true of the page's making and is not contradicted by the Result column, but a reader may stumble over it next to the nine `deleted` rows.

## Gaps Summary

No gaps. All four ROADMAP success criteria hold against live git state and the files themselves. The branch deletions, the local retirements and the verified bundle are real, and the remote matches the recorded decisions exactly. The only open items are bookkeeping markers (above) and the owner's push of `main`, which is outside this phase.

---

_Verified: 2026-10-26_
_Verifier: Claude (gsd-verifier)_
