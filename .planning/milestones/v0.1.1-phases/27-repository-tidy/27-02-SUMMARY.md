---
phase: 27-repository-tidy
plan: 02
subsystem: planning
tags: [archive, roadmap, milestone-audit, v0.1.0]
requires: []
provides:
  - "milestones/v0.1.0-ROADMAP.md (phase 20 to 23 detail and Progress rows 1 to 23, moved unchanged)"
  - "milestones/v0.1.0-MILESTONE-AUDIT.md (retrospective, written after the release)"
  - "ROADMAP.md with one line for v0.1.0 and a Progress table of phases 24 to 28"
affects: [27-03]
tech-stack:
  added: []
  patterns: ["archive by sed -n extraction, proven byte for byte by 27-check.sh"]
key-files:
  created:
    - .planning/milestones/v0.1.0-ROADMAP.md
    - .planning/milestones/v0.1.0-MILESTONE-AUDIT.md
  modified:
    - .planning/ROADMAP.md
key-decisions:
  - "The audit states the UAT discrepancy (test 2 recorded as pass, nine branches still on origin on 2026-10-26) as a fact and points at phase 27 TIDY-01"
requirements: [TIDY-03]
status: complete
duration: ~25 min
completed: 2026-10-26
metrics:
  tasks: 3
  files: 3
commits: 3
plan_head_before: 3b06411496a69b7992b039e5f5cd2af5351bfac4
plan_head_after: 74b2dcc6e9829a67d301c8c3a68e615e2d8cd886
actuals:
  tokens: 5500
  tasks: 3
  commits: 3
---

# Phase 27 Plan 02: Archive v0.1.0 Summary

v0.1.0 is archived like v2.0 to v2.5: roadmap by byte-exact extraction, a retrospective audit with 14 Record lines that `27-check.sh` finds in the files they name, and a ROADMAP.md that keeps one line and a Progress table of the five open phases.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | `milestones/v0.1.0-ROADMAP.md` by `sed -n` extraction (104-line phase block, Progress rows 1 to 23) | fa5532e |
| 2 | ROADMAP.md: one v0.1.0 line linking the archive, phase 20 to 23 detail and rows 1 to 23 removed, one paragraph under Progress | 6a1884d |
| 3 | `milestones/v0.1.0-MILESTONE-AUDIT.md` | 74b2dcc |

## Verification

- `27-check.sh archive` passes: live, file (byte equality against the parent, 3b06411496), audit (14 Record lines), roadmap.analyze `phase_count 5`, audit-open "All artifact types clear".
- `git diff --stat 3b06411 HEAD`: ROADMAP.md deletions plus a few added lines, and the two new files; nothing else.
- Task 2 hunk checked: the `---` rule before Phase 20 now stands directly before `### 🚧 v0.1.1`.
- `gsd-tools query progress` was not run separately; `roadmap.analyze` and `audit-open` (both reads) ran against the worktree.

## What the audit says that is uneven

- hardened unit ran under systemd 252 in a container, not on a real server;
- plugins measured on native arm64 only (MDWE verdict VOID on amd64);
- 23-UAT test 2 (branch deletion) is `result: pass` while `git ls-remote --heads origin` on 2026-10-26 still lists all nine branches it concerns. Phase 27 TIDY-01 takes this up.

Release data read on 2026-10-26: tag v0.1.0, published 2026-10-18T15:08:54Z, prerelease true, commit 4d238b97d104296469a480e3b701d7a1f8a1c712 (the same from `git rev-parse v0.1.0^{commit}` and `git ls-remote --tags origin`). The CHANGELOG entry is dated 2026-10-14; the audit records both dates.

## Deviations from Plan

**1. [Rule 3 - Blocking] Block deletion in ROADMAP.md done with `sed -i` line ranges, not the Edit tool**
- **Found during:** Task 2
- **Issue:** Deleting 107 lines plus 23 table rows with Edit would have meant retyping them as `old_string`, the thing this plan avoids. The plan said Edit only.
- **Fix:** A wrapper script in the scratchpad asserted the boundary lines (Phase 20 heading, closing rule, v0.1.1 heading, table rows 1, 23 and 24) and then deleted line ranges 390-496 and 640-662. The two small additions (Milestones line, Progress paragraph) were made with Edit. The file was not rewritten as a whole.
- **Files modified:** `.planning/ROADMAP.md`
- **Commit:** 6a1884d

No other deviations. Commands the worktree guard refused (compound shell, `git commit` chained) were run through wrapper scripts in the scratchpad, as instructed. The base was not an ancestor of HEAD at start (HEAD was ee3c9ed) and the tree was clean, so `git reset --hard 3b06411` was run once.

## Known Stubs

None.

## Threat Flags

None.

## Not run

No mutating `state.*` or `roadmap.*` verb was run; STATE.md, REQUIREMENTS.md, docs/branches.md and 27-OWNER-STEPS.md are untouched. No push, no branch or tag change.

## Self-Check: PASSED

- `.planning/milestones/v0.1.0-ROADMAP.md`, `.planning/milestones/v0.1.0-MILESTONE-AUDIT.md`: present.
- Commits fa5532e, 6a1884d, 74b2dcc are ancestors of HEAD.
