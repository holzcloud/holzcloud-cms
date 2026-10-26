---
phase: quick-261026-rrh
plan: 01
subsystem: media
tags: [media, scale-down, exif-orientation, icc-profile, jpeg, png, webp]
requires:
  - phase: quick-261026-bon
    provides: the scale-down question and Intake.Scaled
provides:
  - "Intake.Scaled applies EXIF orientation 1-8 (JPEG, PNG, WebP source) and carries a valid RGB ICC profile"
  - "Oversize and Staging.Put report the upright size, so the question prints what the visitor sees"
affects: [media, admin media upload]
tech-stack:
  added: []
  patterns:
    - "readers that return 1 / nil for anything unreadable, never an error, every offset bounded by the slice"
    - "pre-change pipeline carried in a test as the byte-identity reference"
key-files:
  created:
    - internal/media/orient.go
    - internal/media/orient_test.go
    - internal/media/profile.go
    - internal/media/profile_test.go
  modified:
    - internal/media/scale.go
    - internal/media/staging.go
    - internal/admin/media_scale_test.go
    - docs/publishing.md
key-decisions:
  - "Scale in the stored geometry first, then turn the scaled bitmap (a permutation of at most 2560 x 2560); ScaledSize is symmetric, a test pins it"
  - "Staging.Put stores the upright size, so the admin page needed no change and 'after' equals the stored file's size exactly"
  - "Only an RGB, well-formed, at most 4 MiB profile is carried; never an error"
  - "A WebP source that carries an orientation or a profile gets them honoured like any other source (owner decision b); scaledFormat unchanged"
  - "iCCP found after IDAT is not accepted as a profile (the PNG specification puts it before the pixels)"
status: complete
metrics:
  duration: "about 1h10m"
  completed: 2026-10-26
actuals:
  tokens: 15200
  tasks: 3
  commits: 4
plan_head_before: 319ae8b9c2544453eaf942cb23b13b10bded1814
plan_head_after: b167f21
---

# Phase quick-261026-rrh Plan 01: EXIF orientation and ICC profile on "Yes, scale down" Summary

**Answering Yes now stands a phone photo upright (EXIF orientation 1-8 from JPEG, PNG and WebP applied to the pixels, the long side counted on the upright picture, the question printing the upright sizes) and carries a valid RGB ICC profile into the copy, using only the standard library and golang.org/x/image.**

## What was built

- `internal/media/orient.go`: `orientationOf` (JPEG APP1 Exif, PNG eXIf, WebP EXIF), shared `tiffOrientation` (both byte orders, SHORT/count-1 only), `walkJPEG`, `orientRGBA` (the eight transforms, orientation 1 returns the same image), `Intake.uprightSize`.
- `internal/media/profile.go`: `profileOf` (JPEG APP2 chunks reassembled 1..n, PNG iCCP inflated through a 4 MiB cap, WebP ICCP), `validRGBProfile` (size equals length, `acsp`, `RGB `, at most 4 MiB), `withProfile` (APP2 segments after SOI, iCCP chunk "ICC profile" after IHDR).
- `Scaled`: scale in the stored geometry, `orientRGBA`, encode, `withProfile`, then the byte-limit check on the final bytes. `Oversize` and `Staging.Put` use the upright size. Admin handler and templates unchanged; no new text, the i18n gate is untouched.
- `docs/publishing.md`: the scale-down paragraph states what is carried and the two remaining limits (smaller copies without a profile; unscaled uploads lose the orientation tag with the rest of the EXIF block).

## Verification

- Byte identity (B-4): `TestPlainPictureBytesDoNotChange` carries the three old steps (decode, `ScaledSize` of the stored header, `CatmullRom` into `image.NewRGBA`, encode) as its reference. It was written and run first against the unmodified `scale.go` and PASSED (5 sub-cases: JPEG without EXIF, JPEG with the text-only EXIF block, JPEG with a real orientation-1 block, PNG, still GIF), and stays green after the change. `TestProfileNotCarriedWhenUnfit` (17 cases: CMYK, GRAY, no `acsp`, wrong declared size, over 4 MiB, JPEG sequence gap / duplicate / disagreeing count / missing chunk / sequence zero, PNG method 1 / corrupt zlib / truncated zlib / 64 MiB bomb / no name / after IDAT, GIF) asserts the same equality.
- Orientation: pixel-exact table of the eight transforms; all eight values through `Scaled` with a lossless PNG source (exact quadrant colours, sizes equal `ScaledSize` of the upright pair); JPEG orientation 6 end to end; `Oversize`/`Staging.Put`/`Scaled` agree for 1..8; `TestOrientationNeverPanics` feeds every prefix and every 0x00/0xFF overwrite of the first 400 bytes of a valid JPEG, PNG and WebP.
- Admin end to end: `TestScaleQuestionUsesTheUprightPicture`: a 3000 x 1500 JPEG with orientation 6 is announced "1500 × 3000 px, becomes 1280 × 2560 px", stored 1280 x 2560 without EXIF; answered No it is stored at 3000 x 1500.
- Gates: gofmt clean; `go build ./...`, `go vet ./...`, static `CGO_ENABLED=0` build OK; `go run ./tools/i18n` 0 open, 0 orphaned for de/es/fr/it; `go run ./tools/themewords -check` OK; `go run ./tools/english` OK; `go.mod`/`go.sum` last touched by 4465375 (unchanged).
- Full test run: `go test ./... -timeout 30m` green, every package `ok` (internal/media 30 s; the admin package included).
- `28-check.sh all` (GO=0 and, separately, with `changelog go` running; NATIVE_SELFTEST=0): see Deviations.

## Deviations from Plan

### Owner-accepted red gate

**1. [Accepted conflict, B-8] `28-check.sh all` fails exactly on the two `frozen` lines**
- Result of `bash .planning/phases/28-the-promise-in-writing/28-check.sh all` after the commits:
  - `FAIL: something outside .planning, docs and CHANGELOG.md changed after 85fa486`
  - `FAIL: a page of docs other than beta.md changed after 85fa486`
  - Every other sub-check is `ok`: beta (structure, baseline, ledger, quotes, coverage, does-not-promise, srv, evidence), owner steps, changelog (extract, topics, and `changelog go` when run without GO=0), regression (nightly selftest 542, record-check 29, record-check-con 27, native-amd64 status-check and record-check), and the deploy-binding diffs of `frozen` itself (05fc076 and 7d20710 produced no FAIL).
- Not hidden: `28-check.sh`, `docs/beta.md`, `CHANGELOG.md` and `deploy/*` are untouched. The owner accepted the red gate (decision a); re-pinning it is a separate authorised change.
- In the worktree the regression slice needs the gitignored `deploy/test/out` of the main checkout; it was copied in (ignored by git, not committed). `NATIVE_SELFTEST=0` as the plan says.

### Auto-fixed Issues

None. Two small things inside the plan's reach: the test helper for inserting JPEG segments is named `insertSegments` (an existing `jpegWithSegments(t, ...)` in `strip_test.go` has that name), and a profile in a PNG iCCP chunk found after IDAT is refused (PNG specification; not in the plan's list, covered by a test, result identical to before).

### Process notes

- The worktree HEAD was `ee3c9ed`, the main base `319ae8b`, the tree clean; reset hard to `319ae8b` as instructed. `ee3c9ed` is not an ancestor of `319ae8b` (merge-base `b654596`).
- The first task commit was created with a wrong message (a stale file of the shared scratchpad was read as the message file); it was amended at once, before anything else, to the intended message. No other history was rewritten.

## Owner decisions applied

- (a) the red `frozen` is reported above, nothing edited to hide it.
- (b) WebP: the plan's choice stands. A WebP source with an EXIF orientation or an ICCP profile gets them honoured (orientation applied, profile carried into the JPEG or PNG the WebP becomes). To leave WebP alone entirely, remove the `image/webp` case in `orientationOf` and in `profileOf`. `scaledFormat` (which format a WebP or GIF becomes) is unchanged.
- (c) The pre-existing `StripMetadata` limit (the orientation tag is dropped from every upload that is stored without scaling) is not touched; it is documented in `docs/publishing.md` as a remaining limit and becomes its own quick task.

## Known Stubs

None.

## Threat Flags

None. The mitigations of the plan's register are in place: bounded readers with a no-panic test (T-rrh-01), capped inflate with a bomb case (T-rrh-02), the turn applied to the already scaled bitmap (T-rrh-03), `validRGBProfile` and exact JPEG sequence rules (T-rrh-04), only orientation and profile are taken from the source and the copy has no EXIF (T-rrh-05), no dependency change (T-rrh-SC).

## Self-Check

PASSED: all created files exist (orient.go, orient_test.go, profile.go, profile_test.go, this SUMMARY); commits b8dc2ff, f4887c7, efbd3aa, b167f21 are in HEAD's history; `git status` clean apart from the ignored `deploy/test/out`; docs/beta.md, CHANGELOG.md, deploy/, 28-check.sh, go.mod, go.sum unchanged; no push, no tag.
