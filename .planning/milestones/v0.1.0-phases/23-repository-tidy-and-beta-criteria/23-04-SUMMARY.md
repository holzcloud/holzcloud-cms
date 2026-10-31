---
phase: 23-repository-tidy-and-beta-criteria
plan: 04
subsystem: docs
tags: [release, owner-steps, final-gates, evidence-set]

requires:
  - phase: 23-repository-tidy-and-beta-criteria
    provides: beta criteria, 0.1.0 entry, README and Versioning (23-01, 23-02), docs/branches.md (23-03)
provides:
  - 23-OWNER-STEPS.md, the push, release, check and branch-tidy steps only the owner can take, with the veto points and the state of every gate at the end of the phase
affects: []

actuals:
  tokens: 4000
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Gates run last on the end state and their result is recorded in the owner's file, never by loosening a check"

key-files:
  created:
    - .planning/phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md
  modified: []

key-decisions:
  - "git push origin main comes before git push origin main:release: the release body links to files that exist on GitHub only after main is pushed, and the Actions button builds the head of the default branch"
  - "No command in the owner file creates or pushes a tag; the only tag command is the read-only ls-remote listing"
  - "Four veto points are recorded with the cost of overriding each (alpha label in product and DEPLOY.md, heading date, 0.x pre-release, CHANGELOG as release body)"

requirements-completed: [REPO-02, REPO-03, QUAL-01]

plan_head_before: 3cb6ed022a8510763acb286e5ceffec9ee49fad4
plan_head_after: 61709e66f90146d510b4c2886e66747717b70f79
commits: 2
status: complete
---

# Phase 23 Plan 04: Owner steps and final gates Summary

**23-OWNER-STEPS.md gives the owner a two-command release (git push origin main, then git push origin main:release) with the reason for the order, read-only checks, the branch tidy pointer and four veto points; every gate is green on the end state, the Phase 22 evidence set equals bf74496, and no tag exists or was pushed.**

## Accomplishments

- Task 1: wrote the owner file with sections 0 to 5, Veto points, Outside the repository and What this phase did not do. Route 2 and route 3 of release.yml are stated as the file describes them (pre-release for v0.*, notes from the CHANGELOG section plus generated notes, nothing tagged if tests fail, holzcloud-ch job needs CMS_TOKEN).
- Task 2: ran every gate on the final state and appended `## State at the end of the phase`.
  - `go run ./tools/i18n`: 0 open, 0 orphaned for de, es, fr, it (2056 strings each); catalogues byte-unchanged.
  - `themewords -check` and `tools/english` exit 0; build, changelog tests, admin changelog tests and deploy-document tests pass.
  - `go test -timeout 20m ./... -count=1`: all packages pass (55 ok, no FAIL, no panic).
  - Six evidence-set files equal bf74496; no Go file, go.mod, go.sum, catalogue or workflow differs from 9db196b; nothing outside .planning, deploy, docs, CHANGELOG.md, README.md, .gitignore differs from v0.0.25.
  - release.yml's two snippets on the final CHANGELOG.md yield 0.1.0 with a non-empty notes section holding the upgrade note and both links; v0.1.0 exists neither locally nor on the remote.
  - docs/beta.md citations all found in their records; docs/branches.md equals git after `git fetch origin --prune` (10 rows), so no regeneration was needed.

## Task Commits

1. Task 1: owner steps - `580f88b`
2. Task 2: final gates recorded - `61709e6`

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None.

## Notes for the owner

The owner file is the remaining human work: push main, push main:release, run the two checks, delete the Safe to delete branches in the GitHub UI after deciding on `backfill-releases` and `tagfix`. Nothing was pushed, tagged or deleted by this plan.

## Self-Check: PASSED

- FOUND: .planning/phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md
- FOUND: 580f88b, 61709e6 (ancestors of HEAD)
