---
phase: 23-repository-tidy-and-beta-criteria
plan: 01
subsystem: docs
tags: [beta-criteria, changelog, release, evidence, i18n-gate]

requires:
  - phase: 20-hardened-unit-under-systemd
    provides: deploy/SECURITY-SCORE.md (evidence commit f1428d2)
  - phase: 21-install-update-rollback-played
    provides: deploy/DEPLOY-RUN.md (build 2a11eeb)
  - phase: 22-backup-and-restore-measured
    provides: deploy/BACKUP-RUN.md (evidence commit bf74496) and the hand-over in deploy/test/README.md
provides:
  - docs/beta.md, the written beta criteria of 0.1.0 (ten cited promises, the non-promises, the evidence binding, the note on the word alpha)
  - CHANGELOG.md heading "## 0.1.0 — 2026-10-14" with its German lead sentence and the absolute link to docs/beta.md
  - main level with origin/main (merge commit of 9cc7bcb, the slim README)
  - the clean i18n, themewords and english baseline taken before the first edit
affects: [23-02 (completes the 0.1.0 entry, README badge, docs/deployment.md), 23-03 (branches), 23-04 (owner steps and veto points)]

actuals:
  tokens: 3500
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "A promise is a table row whose backticked cited words are proven against one record by a grep -F loop"
    - "The release.yml shell snippets are rehearsed locally against the edited CHANGELOG, with no tag made"

key-files:
  created:
    - docs/beta.md
  modified:
    - CHANGELOG.md

key-decisions:
  - "The product's own alpha label (web.IsAlpha, three admin templates, three catalogue keys) and the alpha sentence in deploy/DEPLOY.md stay unchanged in 0.1.0; docs/beta.md says where the word still appears and why (recorded assumption, owner may veto in plan 23-04)"
  - "Beta is defined as safe to run for real with backups and not feature-frozen; schema and migrations are not promised stable"
  - "The CHANGELOG date is the real date 2026-10-14; the owner changes that one line if the tag day differs"

patterns-established:
  - "Evidence-first contract: promises cite a record, limits are named, each claim is bound to its evidence commit"

requirements-completed: [REPO-01]

plan_head_before: de694ac55a8f70fb9995b5147a0145169a5f542e
plan_head_after: 828c4fab1c3f40640fb2f8dc84dcba6446981264
status: complete
---

# Phase 23 Plan 01: Beta criteria and the 0.1.0 heading Summary

**docs/beta.md as a contract in which ten promises each cite one of the three Phase 20 to 22 records by words that `grep -F` finds in it, every Not tested and Not measured limit is named, and the CHANGELOG now opens with `## 0.1.0 — 2026-10-14`, which release.yml's own shell snippets parse into version 0.1.0 with a notes section that links the beta page.**

## Accomplishments

- Baseline before any edit: `go run ./tools/i18n` printed `0 open, 0 orphaned` on de, es, fr and it (2056 strings), `themewords -check` and `tools/english` exited 0. They were still clean after the last edit and `internal/i18n/locales/` is byte-unchanged.
- Repository brought level with origin/main by a merge (ddb8b28), not a rebase. It was clean: upstream's one commit 9cc7bcb changed README.md and docs/publishing.md only. README.md now has upstream's `## Documentation` list.
- Tracer (task 1): `docs/beta.md` skeleton with P-1 and E-1, and the `## 0.1.0` heading, lead sentence and absolute link in CHANGELOG.md. Release rehearsal printed `release-rehearsal-ok`: first version heading 0.1.0, release.yml's awk returns a non-empty notes section that holds the link, no HTML, `v0.1.0` free locally and on the remote.
- Task 2: docs/beta.md completed. Five level-two headings in the required order, P-1 to P-10 and E-1 to E-3 (13 rows, all cited words found in their record), non-promises in four groups carrying all twenty required words, evidence section naming the changed sections of deploy/DEPLOY.md since 2a11eeb, the rerun rule ("made again") and the `git diff --quiet bf74496 HEAD -- ...` command, and the note on the word alpha.
- The six evidence-set files (deploy/DEPLOY.md, backup.sh, restore.sh, the two backup units, deploy/test/run.sh) are byte-identical to bf74496.

## Task Commits

1. **Task 1 (tracer): merge, baseline, beta skeleton, 0.1.0 heading** - `ddb8b28` (merge of origin/main), `efd0cef` (feat)
2. **Task 2: complete docs/beta.md** - `828c4fa` (docs)

The measured count `commits: 4` is `git rev-list --count de694ac..HEAD`: it includes the merge commit and upstream's 9cc7bcb that the merge brought in, besides the two task commits.

## Files Created/Modified

- `/Users/cheidenreich/privat/holzcloud-cms/docs/beta.md` - the beta criteria of 0.1.0
- `/Users/cheidenreich/privat/holzcloud-cms/CHANGELOG.md` - new top section `## 0.1.0 — 2026-10-14`, nothing below it touched

## What the page says about deploy/DEPLOY.md

Since 2a11eeb the file changed in Prerequisites (the `rsync` bullet), Backups (rewritten), two bullets of Updating and Rolling back about what the pre-upgrade snapshot covers, and Restore from a backup (rewritten). No command of Install, Updating or Rolling back changed. The page states this as `git diff -U0 2a11eeb HEAD -- deploy/DEPLOY.md` shows it today.

## Decisions Made

See the frontmatter. The alpha assumption is the one the owner may veto; overriding it would orphan catalogue keys in four languages (label) or force a rerun of the Phase 22 evidence set (deploy/DEPLOY.md sentence).

## Deviations from Plan

None - plan executed exactly as written.

Notes, not deviations:
- The plan mentioned an untracked `23-PATTERNS.md`; it was already committed when this plan ran. `.planning/STATE.md` and `.planning/config.json` carried orchestrator changes and `.planning/milestone.lock` was untracked; none of them was staged by the task commits.
- `requirements-completed` lists only REPO-01. The plan's frontmatter also names REPO-02 and QUAL-01, but this plan delivers their seed and baseline only: the rest of the 0.1.0 entry, the README and the owner's tag (REPO-02) and the final gate run (QUAL-01) belong to plans 23-02 and 23-04.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path, file access or schema change; only two documents changed.

## Verification Run

- i18n gate, themewords and tools/english: clean before and after.
- `go test . ./internal/changelog/ -count=1` and `go test ./internal/admin/ -run Changelog -count=1`: ok.
- Citation loop: `citations-match-records: 13 rows`. Structure check: `beta-structure-ok`. Evidence and alpha check: `evidence-rule-and-alpha-note-ok`. Release rehearsal: `release-rehearsal-ok`.
- `git tag -l v0.1.0` empty, `git ls-remote --tags origin v0.1.0` empty; nothing pushed.

## Self-Check: PASSED

- FOUND: docs/beta.md
- FOUND: CHANGELOG.md with `## 0.1.0 — 2026-10-14` above `## 0.0.25`
- FOUND commits: ddb8b28, efd0cef, 828c4fa
