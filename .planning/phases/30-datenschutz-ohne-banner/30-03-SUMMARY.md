---
phase: 30-datenschutz-ohne-banner
plan: 03
subsystem: media
tags: [exif, privacy, regression-test, source-guard]
requires: []
provides:
  - "internal/media/intake_exif_test.go: checkedUpload, keptBytes, regression tests of D-10"
  - "internal/media/upload_guard_test.go: storingCalls, source guard"
affects: [30-06]
tech-stack:
  added: []
  patterns: ["go/parser source guard with a self-check of the detector"]
key-files:
  created:
    - internal/media/intake_exif_test.go
    - internal/media/upload_guard_test.go
  modified: []
key-decisions:
  - "Test 5 keeps two separate uploads of identical bytes, not one Intake twice: a second Keep of the same Intake reuses its filename, and the duplicate branch removes that path, which is the first copy."
requirements-completed: [DSV-05]
status: complete
commits: 3
plan_head_before: 9f69e084923f5eac8d22438bc7cfef7a6ba774f7
plan_head_after: bfbae64
actuals:
  tasks: 2
  commits: 3
---

# Phase 30 Plan 03: EXIF removal secured by test Summary

A photograph with GPS and camera data, sent through the real `Check` and `Intake.Keep`, is shown to reach the disk without them and with its orientation intact (JPEG both byte orders, PNG, WebP, scaled copy), and a source guard fails when anything but `Keep` stores an upload.

## Tasks

| Task | Commit | Files |
|------|--------|-------|
| 1. Real Check and Keep with a GPS/camera photograph | e6dfb56 (comment fix dcb686e) | internal/media/intake_exif_test.go |
| 2. Keep stays the only way an upload is stored | bfbae64 | internal/media/upload_guard_test.go |

No production file changed.

## Red runs

- Strip removed from `Keep` (source stored as it came in): `TestKeepStoresNoCameraDataAndKeepsTheTurn` failed on JPEG little-endian, JPEG big-endian, PNG and WebP (40 "still contains" messages), and `TestKeepRecordsTheBytesOnDisk` failed (nothing stripped). `TestKeepOfAScaledCopyCarriesNoCameraData` stayed green, as it must: the scaled copy is re-encoded from pixels and never carried the metadata, so that test holds the property rather than the strip. The edit was reverted and not committed.
- Guard with a foreign call: a scratch file `internal/admin/zz_scratch.go` calling `media.StoreFile` made `TestOnlyIntakeKeepStoresAnUpload` fail with "internal/admin/zz_scratch.go calls [StoreFile]: an uploaded file must be stored through Intake.Keep ...". File removed.
- The guard walks about 292 non-test Go files (threshold 150). Outside `internal/media` the three functions have 0 callers.

## Verification

- `go test ./internal/media/ -count=1`: ok
- `go vet ./internal/media/`, `gofmt -l internal/media`: clean
- `go run ./tools/english`: 0 lines naming internal, cmd, tools, plugins or sdk
- `go.mod`, `go.sum` and all production files in `internal/media` unchanged against 9f69e08

## Deviations from Plan

**1. [Rule 3 - Blocking] Worktree base was stale.** The worktree branch started at b0995b9, 133 commits behind `main` (no `orient.go`, `scale.go`, `staging.go`, no `.planning`). It had no commits of its own, so it was reset to `main` (9f69e08) before any work. Acceptance diffs were therefore taken against 9f69e08, not e2513b1 (the plan's base; only planning docs differ).

**2. [Rule 1 - Bug] Guard skips `.claude`.** The plan's skip list lacks `.claude`. In the main checkout `.claude/worktrees/*` holds whole copies of the tree, which would make the guard flag `intake.go` copies. Added to the skip list.

**3. Field name.** The plan says the record's `Size`; the `Media` field is `SizeBytes`.

## Known Stubs

None.

## Self-Check: PASSED

Both test files exist; commits e6dfb56, dcb686e, bfbae64 are ancestors of HEAD.
