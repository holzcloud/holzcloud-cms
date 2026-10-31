---
phase: 23-repository-tidy-and-beta-criteria
plan: 02
subsystem: docs
tags: [changelog, release-notes, readme, beta, upgrade-note]

requires:
  - phase: 23-repository-tidy-and-beta-criteria
    provides: docs/beta.md and the 0.1.0 heading with its lead sentence (plan 23-01)
provides:
  - the complete German 0.1.0 entry of CHANGELOG.md (the release page body), with the upgrade note for operators on 0.0.25
  - README.md with the status-beta badge, a notice that says no more than docs/beta.md, and the Beta criteria bullet
  - the Versioning section of docs/deployment.md saying beta
affects: [23-03, 23-04 (owner edits the one heading date if the tag day differs)]

actuals:
  tokens: 4500
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Release notes are the CHANGELOG section itself; the upgrade note lives inside it with absolute URLs"

key-files:
  created: []
  modified:
    - CHANGELOG.md
    - README.md
    - docs/deployment.md

key-decisions:
  - "The upgrade note is a CHANGELOG paragraph only (no page under docs/), because only the CHANGELOG section reaches the release page"
  - "The README notice mentions in one clause that the admin screen still labels every 0.x build alpha until 1.0, as docs/beta.md does"
  - "The entry keeps the heading date 2026-10-14 written by plan 23-01"

requirements-completed: [REPO-02]

plan_head_before: 5f6c4b73952caf6cc5963a3b2400b384665ca8e2
plan_head_after: 0aac0e5b2cd4c609843179b6c370dd4624df5883
status: complete
---

# Phase 23 Plan 02: The 0.1.0 entry, README and Versioning Summary

**The 0.1.0 CHANGELOG section now holds seven bold lead-in paragraphs in German, including the note that operators on 0.0.25 copy the whole `deploy` directory to `/opt/holzcloud/deploy`, make the two scripts executable and install `rsync`; the README and the Versioning section say beta in the words of docs/beta.md.**

## Accomplishments

- Task 1: six paragraphs added after the tracer's lead-in: program unchanged, upgrade note, unit under real systemd, install/update/rollback played, backup and restore measured with the scripts repaired, what was not shown. Every fact was checked against the repository first: nothing outside `.planning`, `deploy`, `docs`, `CHANGELOG.md`, `README.md` and `.gitignore` differs from v0.0.25; last migration is `00060_order_token.sql`; `deploy/holzcloud.service` differs in seven comment lines only; `backup.sh` and `restore.sh` carry the `media templates plugins sprachen marke` loop; `restore.sh` refuses a missing `MANIFEST` and runs `chmod 600`; v0.0.25's DEPLOY.md copied only `backup.sh` to `/opt/holzcloud/backup.sh` while the backup unit calls `/opt/holzcloud/deploy/backup.sh`; DEPLOY-RUN.md says the first pass found ten defects.
- Release rehearsal with release.yml's own snippets: first heading 0.1.0, notes section non-empty with no inner level-two heading, no HTML, no relative link, no line removed since v0.0.25 (`changelog-entry-ok`).
- Task 2: README badge `status-beta`, alt `Status: beta`; warning block rewritten (`not feature-frozen`, back up, links to CHANGELOG.md and docs/beta.md); bullet `[Beta criteria](docs/beta.md)` after Deploying. First paragraph of `## Versioning` in docs/deployment.md rewritten, pre-release sentence kept on one line, link `(beta.md)`.

## Task Commits

1. Task 1: complete the 0.1.0 changelog entry - `c5a84f8`
2. Task 2: README badge, notice and bullet; Versioning says beta - `0aac0e5`

## Files Created/Modified

- `/Users/cheidenreich/privat/holzcloud-cms/CHANGELOG.md` - rest of the 0.1.0 section
- `/Users/cheidenreich/privat/holzcloud-cms/README.md` - badge, notice, Documentation bullet
- `/Users/cheidenreich/privat/holzcloud-cms/docs/deployment.md` - Versioning paragraph

## Deviations from Plan

None - plan executed exactly as written.

Note: the first draft of the Versioning paragraph wrapped the sentence `Every 0.x release is published as a pre-release.` across two lines, which the plan's `grep -F` verify would not match; it was reflowed before the commit.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. Only Markdown documents changed.

## Verification Run

- `changelog-facts-ok` components, `changelog-entry-ok`, `readme-notice-ok`, `deployment-doc-ok`, `gates-clean` all pass.
- `go test ./...` passes; `go test . ./internal/changelog/` and the admin Changelog tests pass.
- i18n gate `0 open, 0 orphaned` on four languages, themewords `-check` and `tools/english` clean; `internal/i18n/locales` unchanged.
- The six evidence-set files equal bf74496; `.github`, Go files, go.mod and go.sum equal 9db196b.

## Self-Check: PASSED

- FOUND: CHANGELOG.md, README.md, docs/deployment.md (modified)
- FOUND commits: c5a84f8, 0aac0e5
