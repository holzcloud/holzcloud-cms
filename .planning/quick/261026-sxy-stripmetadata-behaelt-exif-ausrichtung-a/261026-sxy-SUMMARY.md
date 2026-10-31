---
phase: quick-261026-sxy
plan: 01
subsystem: media
tags: [exif, orientation, strip, jpeg, png, webp, crop, variants]
requires:
  - phase: quick-261026-rrh
    provides: orientation readers in internal/media/orient.go, orientRGBA, Intake.Scaled
provides:
  - "orientation stub (26-byte TIFF block, one entry) for JPEG APP1, PNG eXIf, WebP EXIF"
  - "Dimensions, MakeVariants and ApplyCrop work on the upright picture"
affects: [media upload, AI/plugin/upload-link/contact-form paths via Intake.Keep, crop tool, srcset]
tech-stack:
  added: []
  patterns: ["write the stub from the validated integer, never copy source bytes", "scale in stored geometry, then turn the small result"]
key-files:
  created: [internal/media/orient_keep_test.go]
  modified: [internal/media/orient.go, internal/media/strip.go, internal/media/variants.go, internal/media/crop.go, internal/admin/media_scale_test.go, docs/publishing.md]
key-decisions:
  - "Only the orientation number is kept (owner decision O-1), written fresh"
  - "Derived copies, recorded sizes and crop are made from the upright picture and carry no tag"
duration: single session
completed: 2026-10-26
status: complete
actuals:
  tokens: 14800
  tasks: 3
  commits: 4
plan_head_before: 26dae2a0b11efa5e906d8251dbb37fcf4f166add
plan_head_after: 6d66171cd91fde71dafc81784090a970fa4d9a81
---

# Phase quick-261026-sxy Plan 01: StripMetadata keeps the EXIF orientation Summary

**A portrait phone photo stored without scaling keeps exactly one piece of EXIF, a fresh 26-byte orientation block (JPEG, PNG, WebP), so browsers show it upright; GPS, device, time, thumbnail and maker notes stay removed, and derived copies and the crop are made from the upright picture.**

## Commits

- 4db212e feat: the JPEG strip keeps the orientation number and nothing else (Task 1, tracer)
- e6c8361 feat: the same orientation stub for PNG (eXIf) and WebP (EXIF) (Task 2)
- 9935bff feat: the crop tool works on the upright picture (Task 3, code)
- 6d66171 docs: publishing.md says the orientation note is kept (Task 3, docs)

## Security proof (T-sxy-01)

`TestStripKeepsOnlyTheOrientationJPEG/PNG/WebP` feed a rich multi-IFD EXIF (Make, Software, DateTimeOriginal, MakerNote, GPS IFD with a position and a text tag, IFD1 with a thumbnail; both byte orders; orientation 2..8; the WebP once with and once without the "Exif" prefix) plus XMP, IPTC, COM, tIME, tEXt, iTXt. Output is `bytes.Equal` to the clean file with exactly the independently spelled stub, no marker string survives, the stub's IFD0 holds only tag 0x0112 with next-IFD 0. `TestRichEXIFFixtureIsReal` proves the fixture contains every marker and reads the orientation, so the tests cannot pass vacuously.

```
--- PASS: TestRichEXIFFixtureIsReal
--- PASS: TestStripKeepsOnlyTheOrientationJPEG
--- PASS: TestStripLeavesNoStubForOneAbsentOrUnreadable
--- PASS: TestScaledCopyGetsNoStubOnKeep
--- PASS: TestStripJPEGHostileInput
--- PASS: TestStripKeepsOnlyTheOrientationPNG
--- PASS: TestStripKeepsOnlyTheOrientationWebP
--- PASS: TestStripLeavesNoStubPNGWebP
--- PASS: TestStripHostileInputPNGWebP
ok  github.com/holzcloud/holzcloud-cms/internal/media
```

## Red and green lists

Before implementation (helpers `orientationTIFF/APP1/Chunk*`, `exifHeader`, `look`, `uprightImage` existed as pure writers so the test files compile; `stripJPEG/PNG/WebP`, `Dimensions`, `MakeVariants`, `ApplyCrop` were unmodified):

Task 1, green (characterization or fixture, pass against unmodified behaviour): TestRichEXIFFixtureIsReal, TestOrientationStubBytes, TestStripLeavesNoStubForOneAbsentOrUnreadable, TestStripJPEGHostileInput.
Task 1, red: TestStripKeepsOnlyTheOrientationJPEG, TestStripStubTakesTheExifSegmentsPlace, TestScaledCopyGetsNoStubOnKeep, TestDimensionsAreUpright, TestVariantsStandUpright; admin: TestAPhotoUnderTheLimitKeepsOnlyItsOrientation, TestScaleAnswersAndTheOrientationTag (No branch), TestScaleQuestionUsesTheUprightPicture (edited assertion).

Task 2, green before the code: TestOrientationChunkBytes, TestStripLeavesNoStubPNGWebP, TestStripHostileInputPNGWebP, and TestVariantsStandUprightPNG (already satisfied by Task 1's MakeVariants/Dimensions change, kept as a PNG regression guard).
Task 2, red: TestStripKeepsOnlyTheOrientationPNG, TestStripKeepsOnlyTheOrientationWebP.

Task 3, red: TestCropStandsUpright. Green before the code: TestOriginalKeepsTheTagAndRestores (Dimensions was already upright after Task 1), TestUprightImage (pure function added first).

## Decision record as built

- C-1 stub in the place of the first Exif-bearing segment/chunk; claimed even at orientation 1 (a later one is never consulted). Built and tested (`TestStripStubTakesTheExifSegmentsPlace`, PNG "two chunks" case).
- C-2 26-byte little-endian TIFF block built from the integer; JPEG APP1 36 bytes, PNG eXIf chunk 38 bytes with fresh CRC-32, WebP EXIF chunk 34 bytes without the "Exif" prefix.
- C-3 WebP stub only when VP8X is the first chunk with at least 10 payload bytes; its EXIF flag (0x08) is set only then. Simple container: chunk dropped as before.
- C-4 derived copies honour the tag: MakeVariants scales the stored bitmap, turns the small result with orientRGBA, records the upright pair; ApplyCrop turns the decoded picture upright before the editor's rotation and crop. No tag on derived files.
- C-5 Dimensions returns the upright size and reads the whole file (`look`).
- C-6 no migration; documented.

## Deviations from Plan

### Auto-fixed Issues

None beyond the planned edits.

### Plan-stated deviations

**1. [Planned] One edited assertion in TestScaleQuestionUsesTheUprightPicture** - the No branch read the stored bitmap's size through `sizeOnDisk` (media.Dimensions), which now answers the upright size. It now reads the bitmap's own header through `image.DecodeConfig` (3000x1500) and additionally asserts the upright 1500x3000 through `sizeOnDisk`.

### Differences from the plan text

- The plan lists Task 1 as writing `look` and the writers; `orientationChunkPNG/WebP` and `uprightImage` were added to `orient.go` ahead of their strippers/callers in Tasks 2 and 3 so the test files compile for the red run. No behaviour is changed by them alone.
- The shared hostile-input helper `hostileStrip` was written once in Task 1 and used by Task 2 directly (no later replacement needed).
- The working tree started at ee3c9ed and was reset (clean tree) to main 26dae2a as instructed.
- The per-plan commit ledger file under the git dir could not be written (path refused by the worktree guard); `plan_head_before` is the known base 26dae2a, and `commits` was measured with `git rev-list --count 26dae2a..HEAD` (4, before this summary commit).

## Verification gates (all run in the worktree)

- `gofmt -l internal cmd tools`: empty
- `go build ./...`, `go vet ./...`, `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dev/null ./cmd/holzcloud`: clean
- `go test ./internal/media -count=1`: ok (about 23-36 s)
- `go test ./internal/admin -count=1 -timeout 30m`: ok (419.6 s)
- `go test` of every other package (`./...` minus admin): ok, exit 0
- `go run ./tools/i18n`: every catalogue "0 open, 0 orphaned"; no operator-facing text added
- go.mod / go.sum untouched (last commit 4465375); docs changed: docs/publishing.md only; docs/beta.md, CHANGELOG.md, deploy/*, 28-check.sh untouched and not run (the accepted red `frozen` gate is not chased)
- wiring: `tiffOrientation` in strip.go (non-comment) 3; `look(`/`orientRGBA`/`uprightImage` in variants.go 3, crop.go 2
- No push, no tag

## Known Stubs

None.

## Threat Flags

None. The only new output surface is the fixed 26-byte block built from an integer 2..8.

## Self-Check: PASSED

Files exist (orient_keep_test.go, orient.go, strip.go, variants.go, crop.go, media_scale_test.go, publishing.md) and commits 4db212e, e6c8361, 9935bff, 6d66171 are ancestors of HEAD.
