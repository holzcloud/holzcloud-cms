---
phase: quick-261026-sxy
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/media/orient.go
  - internal/media/strip.go
  - internal/media/variants.go
  - internal/media/crop.go
  - internal/media/orient_keep_test.go
  - internal/admin/media_scale_test.go
  - docs/publishing.md
autonomous: true
requirements: [QUICK-261026-sxy]

estimate:
  tokens: 75000
  raw_tokens: 75000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "A JPEG, PNG or WebP whose EXIF orientation is 2 to 8 and which is stored WITHOUT scaling (answer No, under the limit, AI upload, upload link, plugin, contact-form attachment: every path ends in Intake.Keep, PrepareUpload, StripMetadata) keeps exactly one piece of EXIF: a 26-byte TIFF block whose only entry is the Orientation tag. A browser therefore shows a portrait phone photo upright"
    - "Everything else in the EXIF block is still removed: GPS, camera make and model, timestamps, the thumbnail (IFD1), maker notes, the Exif and GPS sub-IFDs. The stub is built from the integer 2..8 alone and never copies a byte of the source EXIF, so nothing can ride along; a test with a rich multi-IFD EXIF proves it. XMP, IPTC and comments are removed as before"
    - "Orientation 1, no orientation tag, and anything unreadable give the bytes the strip gave before this change (pinned by tests that pass against the unmodified code before it is touched)"
    - "The scaled copy (answer Yes) is turned in the pixels by Intake.Scaled and carries NO stub: the stored scaled copy has orientation 1 and no Exif. The turn is applied once, never twice. A non-scaled copy of the same upload keeps tag N"
    - "Dimensions is the size the picture has when it is looked at (width and height swapped for orientation 5-8), so the recorded width and height, the srcset descriptors and the crop preview agree with what the browser draws for the original"
    - "The smaller copies (thumb, medium, large) and the cropped picture are made from the upright picture and carry no tag, so the grid thumbnail, the srcset candidates and the original all stand the same way"
    - "The public media route is unchanged: it serves the stored bytes verbatim, tag included, with the DB content type and the immutable cache header"
    - "The three strippers never panic on truncated or overwritten bytes of a tagged file"
    - "go.mod and go.sum are untouched; go run ./tools/i18n ends in 0 open, 0 orphaned; no operator-facing text is added"
  artifacts:
    - path: internal/media/orient.go
      provides: "exifHeader, orientationTIFF, orientationAPP1, orientationChunkPNG, orientationChunkWebP, look, uprightImage (beside the readers that already live here)"
    - path: internal/media/strip.go
      provides: "stripJPEG, stripPNG, stripWebP replace the first EXIF-bearing segment or chunk by the orientation stub when the reader says 2-8"
    - path: internal/media/variants.go
      provides: "Dimensions answers the upright size; MakeVariants scales the stored bitmap, turns the small result, records the upright size"
    - path: internal/media/crop.go
      provides: "ApplyCrop turns the decoded picture upright before the editor's rotation and crop"
    - path: internal/media/orient_keep_test.go
      provides: "Stub bytes, rich-EXIF removal proof, byte identity for orientation 1/absent, Scaled-through-Keep carries no tag, upright dimensions/variants/crop, hostile input for all three formats"
    - path: internal/admin/media_scale_test.go
      provides: "End to end through HandleMediaUpload and HandleMediaServe: a small tagged photo, and a large one answered Yes and No"
    - path: docs/publishing.md
      provides: "The Media section says that one number stays, that the scaled copy has none, and which limit remains"
  key_links:
    - from: internal/media/strip.go
      to: "internal/media/orient.go tiffOrientation (the reader) and orientationTIFF (the writer)"
      via: "the strip asks the same reader that Scaled uses, on the very segment or chunk it is about to drop"
    - from: internal/media/variants.go (MakeVariants, Dimensions)
      to: "internal/media/orient.go look and orientRGBA"
      via: "one read of the file gives header and orientation; the small bitmap is turned, not the full one"
    - from: internal/media/crop.go (ApplyCrop)
      to: "internal/media/orient.go uprightImage"
      via: "orientation first, then the editor's rotation, then the crop rectangle on the upright picture"
    - from: internal/media/scale.go (Scaled)
      to: "internal/media/strip.go via Intake.Keep"
      via: "no code change: the encoders write no EXIF, so the strip finds nothing to turn into a stub"
---

<objective>
Stop the strip from stranding portrait phone photos on their side. StripMetadata currently deletes the whole EXIF block, orientation tag included, from every upload that is not scaled down; a portrait photo under 2560 px is stored as a landscape bitmap with no note, and every browser shows it sideways. 261026-rrh fixed this for the "Yes, scale down" path only and wrote the remaining limit into docs/publishing.md. This task closes the limit.

Owner brief (numbered so tasks can cite them; nothing here is open for renegotiation):
- O-1 DECISION: keep ONLY the orientation tag. When the source had orientation 2 to 8, write a minimal EXIF stub: JPEG APP1 with just the Orientation tag; the equivalent PNG eXIf chunk and WebP EXIF chunk where the stub is valid. Orientation 1 or absent: no stub, bytes exactly as today.
- O-2 All other metadata (GPS, camera, thumbnail, maker notes) stays removed. A test must prove no GPS or other tags survive.
- O-3 Reuse the orientation reader from internal/media/orient.go (merged in 261026-rrh).
- O-4 The scale-down path (Scaled) already turns the pixels and must still write NO stub (no double rotation). Add a test that a stored scaled copy carries no tag and a non-scaled copy keeps tag N.
- O-5 The thumbnail and derived-copy pipeline and the media serve path must not break. Check every caller of StripMetadata (upload, AI/ops, upload links) and every consumer of the stored file's pixels (derived smaller copies, recorded dimensions, crop): do they honour or drop the orientation? Decide the minimal consistent behaviour and document it.
- O-6 A hostile-input test (truncated or overwritten bytes never panic).
- O-7 gofmt, go build, go vet, go test ./internal/media ./internal/admin, go run ./tools/i18n at 0 open and 0 orphaned, no go.mod or go.sum change.
- O-8 Update the "remaining limit" passage in docs/publishing.md that 261026-rrh wrote about this bug. Do NOT edit docs/beta.md, CHANGELOG.md, deploy/*, or .planning/phases/28-the-promise-in-writing/28-check.sh. Its `frozen` gate is known red and accepted by the owner: do not run it to chase green, do not edit it, do not hide it.
- O-9 No push, no tag.

Decisions this plan makes where the brief says "decide" or is silent (Claude's discretion; the executor implements them as written):
- C-1 Where the stub goes: in the place of the FIRST Exif-bearing segment or chunk, the one the reader reads (JPEG: first APP1 that starts with "Exif" and two zero bytes; PNG: first eXIf chunk; WebP: first EXIF chunk). Every other EXIF segment or chunk is dropped as today. The segment chain keeps the order the source had, so JFIF, ICC and Adobe segments are untouched in position. The first Exif-bearing segment is "claimed" even when its orientation is 1, matching what jpegOrientation and the other readers do (a later segment is never consulted).
- C-2 What the stub is: a TIFF block of 26 bytes, little-endian ("II", magic 42, IFD0 at offset 8, one entry, tag 0x0112 of type SHORT with count 1 and the orientation in the low two bytes of the value field, two zero bytes, next-IFD offset 0). Built from the validated integer; no byte is copied from the source. JPEG: APP1 segment of 36 bytes (marker FF E1, length 34, "Exif" and two zeros, the block). PNG: eXIf chunk of 26 bytes with a fresh CRC-32 over type and data. WebP: EXIF chunk of 26 bytes (even, no pad), bare TIFF block without the "Exif" prefix, as the container specification has it.
- C-3 WebP validity: an EXIF chunk is legal only in the extended container, whose first chunk is VP8X with the EXIF flag (bit 3, 0x08) set in its first payload byte. A stub is written only when VP8X is the first chunk; the flag is then set. A WebP with an EXIF chunk but no VP8X is stored as today (no stub): writing one would make the file invalid. Verified at planning time: x/image/webp decodes VP8X + VP8L + EXIF and the reader finds the orientation in it.
- C-4 Derived copies HONOUR the orientation (O-5 decision). Facts found by reading the code: MakeVariants and ApplyCrop decode the stored bitmap with image.Decode, which ignores EXIF, and write fresh files with no EXIF; the recorded width and height come from Dimensions, a header read of the stored bitmap; the srcset lists the recorded original width next to the variants' widths; the crop preview height is computed from the recorded size; the crop tool's "rotation is measured on the upright picture, the one the editor is looking at". Once the original carries a tag the browser shows it upright, so a derived copy that drops the orientation would be sideways beside an upright original, and recorded sizes would be swapped. The minimal consistent rule is therefore: every derived artefact is made from the upright picture and carries no tag. That is three small changes (Dimensions, MakeVariants, ApplyCrop), the same technique Scaled already uses (scale the stored bitmap, then turn the small result with orientRGBA). Untagged pictures take exactly the path they take today.
- C-5 Dimensions changes meaning, deliberately: it returns the upright size and reads the whole file (the EXIF of a PNG or a WebP can sit after the pixel data, so a header-only read cannot know). All of its callers (admin upload, backfill, the regenerate CLI, ApplyCrop, RestoreOriginal) want the upright size for recording; the megapixel budget is unaffected because width times height does not change under a quarter turn. None of them runs per request. One existing assertion in internal/admin/media_scale_test.go reads the stored bitmap's size through Dimensions for a tag-6 original and is adjusted in Task 1 (stated there).
- C-6 Pictures already in a library were stored without EXIF and cannot be repaired; there is no migration. SVG, GIF, PDF and MP4 are not touched.

Verified facts this plan relies on (read at planning time):
- StripMetadata has exactly one production caller, PrepareUpload (upload.go), whose only caller is Intake.Keep (intake.go). Keep is reached from the admin upload (admin/media.go keepChecked, also the staged Yes/No answers and the AI upload), the plugin runtime (plugin/runtime.go) and the contact-form attachment (public/attachment.go). One change point covers all of them.
- Intake.Scaled builds its bytes with the standard encoders (no EXIF, no tag), then Keep runs PrepareUpload on them; with no EXIF in them the strip has nothing to turn into a stub. Intake.Scaled is called only on staged, not yet stored, original bytes (admin/media_scale.go); a stored, tagged file is never scaled a second time.
- Consumers of a stored file's pixel geometry: MakeVariants (variants.go), ApplyCrop and RestoreOriginal (crop.go), Dimensions callers admin/media.go makeVariants, media/backfill.go, cmd/holzcloud/cli.go. Staging.Put and Oversize already use the upright size (uprightSize). The serve route (admin/media.go HandleMediaServe) calls http.ServeFile on the stored path and sets the type from the database: unchanged.
- orient.go has orientationOf, tiffOrientation, walkJPEG, orientRGBA, uprightOf, swapsAxes, Intake.uprightSize. orient_test.go already has helpers tiffBlock, withAPP1, withOrientation, pngWithChunk, webpContainer, quadrants and colA..colD, near, jpegBytes, pngBytes, gradient, intakeOf, dimensionsOf (scale_test.go) that the new tests in the same package reuse. strip_test.go has jpegWithSegments, segment, pngChunk, riffChunk.
- In the admin tests: mediaAdmin, mediaUploadRequest, serve, serveUser, scaleGet, scalePost, tokenOf, libraryOf, sizeOnDisk, portraitPhoto (a real orientation block IN FRONT OF the text-only block that carries "Bauernhof") exist.
- The Bash tool of this environment is zsh: quote globs (--include='*.go'), run any script under bash.
- Baselines at planning time (HEAD db1c3f4): `go test ./internal/media -count=1` green in about 22 s; `go test ./internal/admin -count=1` green in about 405 s (run it in the background, never in the foreground); go vet on both packages clean; gofmt clean; `go run ./tools/i18n` 0 open 0 orphaned; `git log -1 --format=%h -- go.mod go.sum` is 4465375; non-comment references to tiffOrientation, orientRGBA, uprightImage or look( in strip.go, variants.go and crop.go: 0 each; docs/publishing.md has one line with "removes the orientation tag" and one with "Two limits remain".

Out of scope, stated so nobody adds it:
- The PrepareUpload fallback that stores the raw bytes when the strip errors on a malformed file is pre-existing and untouched.
- The VP8X E and X flags that stripWebP leaves set after dropping the chunks they announce are pre-existing; this task sets the EXIF bit only when it writes the stub.
- docs/beta.md, CHANGELOG.md, deploy/* and 28-check.sh (see O-8).

Purpose: a portrait phone photo uploaded under 2560 px stands upright, without publishing where it was taken.
Output: stub writers beside the readers in orient.go, three strippers that use them, derived copies and crop that honour the tag, tests, one docs passage.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@.planning/quick/261026-rrh-exif-ausrichtung-und-icc-profil-beim-ver/261026-rrh-SUMMARY.md
@internal/media/orient.go
@internal/media/strip.go
@internal/media/variants.go
@internal/media/crop.go
@internal/media/intake.go
@internal/media/orient_test.go
@internal/media/strip_test.go
@internal/admin/media_scale_test.go
@docs/publishing.md
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): a JPEG with orientation 6 goes upload - strip - store - recorded size - thumbnail - serve, and keeps only its orientation</name>
  <files>internal/media/orient.go, internal/media/strip.go, internal/media/variants.go, internal/media/orient_keep_test.go, internal/admin/media_scale_test.go</files>
  <read_first>internal/media/orient.go (the readers, orientRGBA, uprightOf, swapsAxes), internal/media/strip.go (stripJPEG, keepJPEGSegment), internal/media/variants.go (Dimensions, MakeVariants), internal/media/orient_test.go (helpers to reuse; do not copy them), internal/media/strip_test.go (jpegWithSegments, segment), internal/media/scale.go (the scale-then-turn technique in Scaled and its comment), internal/admin/media_scale_test.go (portraitPhoto, TestScaleQuestionUsesTheUprightPicture, sizeOnDisk), internal/admin/media_screen_test.go (TestTheServeRouteKeepsItsFourRules for the serve request shape), internal/media/variant_store.go (VariantsFor)</read_first>
  <behavior>
    Write internal/media/orient_keep_test.go and the admin tests FIRST and run them against the unmodified code. The characterization tests (marked G) must PASS at HEAD and stay green; the others must FAIL. Record both lists in the SUMMARY.
    - Helper richEXIF(order, o): a structurally valid multi-IFD TIFF block. IFD0 holds Make (0x010F, ASCII "ACME-Kamera"), Orientation (0x0112, SHORT o), Software (0x0131), the Exif IFD pointer (0x8769) and the GPS IFD pointer (0x8825); the Exif IFD holds DateTimeOriginal (0x9003) and MakerNote (0x927C, UNDEFINED "MAKERNOTE-GEHEIM"); the GPS IFD holds GPSLatitudeRef/GPSLatitude/GPSLongitudeRef/GPSLongitude (0x0001-0x0004, RATIONAL x3); IFD1 holds Compression (0x0103), JPEGInterchangeFormat (0x0201) and its length (0x0202) pointing at the bytes "THUMBNAIL-BYTES". Offsets are computed, not typed. A test asserts the fixture really contains each marker string and that tiffOrientation(richEXIF(order,o)) == o, so the fixture cannot pass vacuously.
    - Helper ifdTags(tiff): lists the tags of IFD0 and the next-IFD offset, bounds-checked, used only by tests.
    - TestOrientationStubBytes: orientationTIFF(6) equals the 26 literal bytes 49 49 2A 00 08 00 00 00 | 01 00 | 12 01 03 00 01 00 00 00 06 00 00 00 | 00 00 00 00; orientationAPP1(6) is FF E1 00 22 45 78 69 66 00 00 followed by those 26 bytes (36 bytes); tiffOrientation(orientationTIFF(o)) == o for o = 2..8; ifdTags gives exactly [0x0112] and next offset 0.
    - TestStripKeepsOnlyTheOrientationJPEG: for o = 2..8 and source byte orders "II" and "MM": source = the 8x8 test JPEG from jpegWithSegments with, in this order directly after SOI, an APP1 Exif segment holding richEXIF(order, o), an XMP APP1, an APP13 and a COM carrying marker strings. Output must: equal the clean JPEG (jpegWithSegments with no segments) with exactly orientationAPP1(o) inserted right after SOI (bytes.Equal, so the pixel data is proven bit-identical and nothing else survived); contain none of the marker strings; decode with jpeg.Decode; give orientationOf("image/jpeg", out) == o; contain exactly one APP1 and ifdTags of its block == [0x0112], next offset 0, block length 26.
    - TestStripLeavesNoStubForOneAbsentOrUnreadable (G): sources whose Exif segment is: orientation 1; a valid TIFF with GPS and Make but no orientation tag; orientation 0, 9 and 65535; orientation stored as the wrong type; the text-only block "Exif" + "GPSLatitude 48.137"; a truncated TIFF; no EXIF at all. Each output is bytes.Equal to the clean JPEG, i.e. what the strip gave before this change.
    - TestStripStubTakesTheExifSegmentsPlace: source order SOI, JFIF APP0, Exif APP1 (orientation 6, rich), XMP APP1, ICC APP2, COM, then the scan gives SOI, JFIF, stub, ICC, scan (built by hand in the test). A second case with two Exif APP1 segments, the first orientation 6 and the second 3, gives one stub of 6. A third case, first Exif segment orientation 1 and second 6, gives NO stub (the first is claimed, as the reader does).
    - TestScaledCopyGetsNoStubOnKeep (O-4, media level): a 3000x1500 quadrant JPEG with orientation 6 through intakeOf and Scaled(0); then PrepareUpload on the scaled content and on the original content, read back with io.ReadAll. The scaled copy's stored bytes: orientationOf == 1 and no "Exif". The original's stored bytes: orientationOf == 6. Also: the scaled copy, decoded, has the upright quadrant arrangement of orientation 6 (reuse the existing colour sampling approach), proving the turn happened once.
    - TestDimensionsAreUpright: a quadrant JPEG written to t.TempDir with orientation 6 and 300x200 stored: Dimensions is 200x300; untagged: 300x200; orientation 1: 300x200; a file that is not an image still errors with the texts "open image" / "read image header" unchanged; a missing file errors.
    - TestVariantsStandUpright: a 2000x1000 quadrant JPEG with orientation 6 on disk, MakeVariants with the default budget: the thumb is 400x800 (Variant.Width 400, Height 800), its file decodes to exactly that, the quadrants in the thumb are in the order orientation 6 gives (sample inside each quadrant, tolerance via near), the file has no Exif and orientationOf == 1; every returned variant is portrait. G: the same pixels with an explicit orientation 1 block and with no block give byte-identical variant files.
    - TestStripJPEGHostileInput: for the tagged rich-EXIF JPEG (kept small): every truncation length from 0 to len, and every offset in the first 200 bytes and the last 40 overwritten with 0x00, 0xFF and 0x7F, through StripMetadata under recover. No panic. When err is nil the output is no longer than the input plus 8 bytes. Also a source with 40 stacked Exif APP1 segments and one with a segment length of 0xFFFF past the end.
    - Admin TestAPhotoUnderTheLimitKeepsOnlyItsOrientation: portraitPhoto(t, 1200, 800, 6) through HandleMediaUpload. The library has one file. Its bytes on disk contain the 36-byte orientationAPP1(6) literal (spelled out in the test as bytes, not by calling an unexported function) and not "Bauernhof" and not "GPS". media.Dimensions of the stored file is 800x1200, the media row's Width and Height are 800 and 1200, and VariantsFor gives a portrait thumb (Height > Width) whose file has no "Exif". HandleMediaServe for the stored filename answers 200 with Content-Type image/jpeg, the immutable Cache-Control header and a body equal to the stored bytes.
    - Admin TestScaleAnswersAndTheOrientationTag: portraitPhoto(t, 3000, 1500, 6) answered Yes (decision "scale"): the stored file is 1280x2560 with no "Exif" (no tag, turned once). The same photo answered No ("keep"): the stored file is the 3000x1500 bitmap WITH the stub of orientation 6 and without "Bauernhof".
    - Existing tests stay green, with exactly one edit: in TestScaleQuestionUsesTheUprightPicture the No branch asserts the stored bitmap size through sizeOnDisk (media.Dimensions), which now answers 1500x3000 for a tag-6 file. Change only that assertion to read the stored bitmap's own header (image.DecodeConfig on the stored file) and expect 3000x1500, and add the upright size 1500x3000 through sizeOnDisk beside it. The sentence in its comment that the original is stored as it came stays true.
  </behavior>
  <action>
Implements O-1 (JPEG), O-2, O-3, O-4, O-5 and O-6 for JPEG, decisions C-1 to C-5.

1. Tests first, as described under behavior. Run them, record the red and green lists, then implement.

2. internal/media/orient.go, next to the readers. Add a package variable exifHeader holding "Exif" followed by two zero bytes and use it in jpegOrientation (replacing its inline literal) so reader and writer share one definition. Add orientationTIFF(o int) []byte returning the 26-byte block of C-2, built with encoding/binary little-endian helpers and the field names in comments (header, IFD0 offset, entry count, tag, type, count, value, next IFD), not as an opaque byte soup. Add orientationAPP1(o int) []byte returning marker, length and exifHeader and the block. Both take an int that the caller has already restricted to 2..8; they do not clamp. Add look(path string) (width, height, o int, err error): os.ReadFile the file, image.DecodeConfig over bytes.NewReader, and orientationOf("image/"+format, data) for the orientation; wrap errors with exactly the texts Dimensions uses today ("open image: %w", "read image header: %w"). Comment on why the whole file is read: the EXIF of a PNG or WebP can follow the pixels.

3. internal/media/strip.go, stripJPEG. Keep keepJPEGSegment as it is (APP1 is still dropped by it). In the walk, before the keep decision, if marker is APP1, the payload starts with exifHeader and no Exif segment has been claimed yet: mark it claimed; read o with tiffOrientation(payload[len(exifHeader):]) (the reader from orient.go, per O-3); when o is 2..8 write orientationAPP1(o) into the output at that position. The segment itself is dropped either way. Update the StripMetadata doc comment: it keeps exactly one thing out of the EXIF block, the orientation number, written fresh; the scaled copy never reaches this because its encoders write no EXIF. Do not copy any byte of the source EXIF into the stub.

4. internal/media/variants.go. Dimensions becomes look plus uprightOf; rewrite its comment (C-5): the size the picture has when it is looked at, the reason the whole file is read, the callers. MakeVariants: replace the Dimensions call by look; keep the width*height budget check on the stored size (symmetric); compute uw, uh with uprightOf; the loop skips a spec when spec.Width >= uw and takes targetHeight = uh*spec.Width/uw (minimum 1); the scale destination is spec.Width by targetHeight, with the two swapped back when swapsAxes(o), the bitmap is scaled in the stored geometry as today, then turned with orientRGBA (which returns its argument unchanged for 1, so untagged pictures produce the bytes they produced) before encodeVariant; Variant.Width and Variant.Height record the upright pair. Comment the technique with a pointer to Scaled. Variants carry no tag because the encoders write none.

5. internal/admin/media_scale_test.go: add the two admin tests and make the one assertion edit under behavior. Use the existing helpers.

Commit atomically after the task passes (feat(quick-261026-sxy): ...).
  </action>
  <verify>
    <automated>go test ./internal/media -count=1 && go test ./internal/admin -run 'TestAPhotoUnderTheLimitKeepsOnlyItsOrientation|TestScaleAnswersAndTheOrientationTag|TestScaleQuestionUsesTheUprightPicture|TestTheServeRouteKeepsItsFourRules|TestYesStoresAScaledCopyWithoutCameraData|TestNoStoresTheOriginal|TestTheAIUploadIsTheScreensUpload|Crop' -count=1 && go vet ./internal/media ./internal/admin && test -z "$(gofmt -l internal)"</automated>
  </verify>
  <done>A tagged JPEG stored without scaling holds one APP1 with the 26-byte orientation block and nothing else from its EXIF (proved on a rich multi-IFD fixture); orientation 1, absent and unreadable give the old bytes; the scaled copy has no tag and is turned once; the recorded size, the thumbnail and the srcset basis are upright; the serve route returns the stored bytes verbatim; hostile input never panics; the red and green lists are in the SUMMARY.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: the same stub for PNG (eXIf) and WebP (EXIF), with the hostile-input proof for all three formats</name>
  <files>internal/media/orient.go, internal/media/strip.go, internal/media/orient_keep_test.go</files>
  <read_first>internal/media/strip.go (stripPNG, stripWebP after Task 1), internal/media/orient.go (pngOrientation, webpOrientation, orientationTIFF), internal/media/orient_test.go (pngWithChunk, webpContainer, TestOrientationFromWebP for the testdata WebP round trip), internal/media/testdata/opaque-lossless.webp (a simple-format VP8L file; its VP8L chunk starts at offset 12)</read_first>
  <behavior>
    Tests first. G marks characterization tests that pass at HEAD.
    - TestOrientationChunkBytes: orientationChunkPNG(6) is 00 00 00 1A "eXIf" + the 26-byte block + a CRC-32 (IEEE) that the test recomputes with hash/crc32 over type and data; orientationChunkWebP(6) is "EXIF" 1A 00 00 00 + the 26-byte block (no pad byte, no "Exif" prefix).
    - TestStripKeepsOnlyTheOrientationPNG: a PNG from pngBytes with a rich eXIf chunk (richEXIF, o = 2..8) after IHDR, plus tEXt, iTXt and tIME chunks with markers. Output: equals the clean PNG with exactly orientationChunkPNG(o) at the position the eXIf chunk had (bytes.Equal against a hand-built expectation); no marker survives; png.Decode succeeds and gives the same bounds and pixels as the source; orientationOf("image/png", out) == o; the chunk's CRC verifies; the IDAT chunks are byte-identical. A second case puts the eXIf chunk after the last IDAT and expects the stub there. A third case has two eXIf chunks, the first with orientation 6 and the second with 3: one stub of 6.
    - TestStripKeepsOnlyTheOrientationWebP: an extended container VP8X + VP8L + EXIF built from the testdata (the VP8L payload is taken from testdata/opaque-lossless.webp; the test fails loudly if the chunk at offset 12 is not VP8L; the VP8X payload is a flags byte, three zero bytes, width-1 and height-1 as 24-bit little-endian values from image.DecodeConfig), the EXIF chunk holding richEXIF(order, o) for o = 2..8, once with and once without the "Exif" prefix, the VP8X flags byte once 0x00 and once 0x08 in the source, plus an XMP chunk with a marker. Output: VP8X flags byte has bit 0x08 set; exactly one EXIF chunk of 26 bytes; the RIFF size equals len(out)-8; no marker survives; the image chunk is byte-identical; image.DecodeConfig of the output equals that of the source and image.Decode succeeds (planning probe: x/image/webp decodes this container); orientationOf("image/webp", out) == o.
    - G TestStripLeavesNoStubPNGWebP: orientation 1, no orientation entry, text-only payloads, unreadable TIFF, for both formats: output bytes.Equal to the strip's old result, i.e. the source with the eXIf or EXIF and XMP chunks removed (and for WebP the RIFF size recomputed), flags untouched. A WebP with an EXIF chunk carrying orientation 6 but NO VP8X (simple container) gives no stub and the old bytes (C-3).
    - TestVariantsStandUprightPNG: a 1000x500 quadrant PNG (pngBytes of quadrants) with an eXIf chunk of orientation 8 on disk: MakeVariants gives a portrait thumb in the order orientation 8 gives ("BD" over "AC" in the table of TestScaledAppliesEveryOrientation), no eXIf chunk in the file, and Dimensions is 500x1000.
    - TestStripHostileInputPNGWebP: the harness of Task 1 (truncations, overwrites in the head and tail, stacked EXIF chunks, a chunk length past the end, a RIFF size that lies) for both formats under recover: no panic; when err is nil the output is at most the input plus 8 bytes. A shared helper may serve all three formats; Task 1's test is adjusted to use it, not duplicated.
  </behavior>
  <action>
Implements O-1 (PNG, WebP), O-2, O-3, O-6, decisions C-1 to C-3.

1. Tests first, record red and green, then implement.

2. internal/media/orient.go: add orientationChunkPNG(o int) []byte (length, type eXIf, the block from orientationTIFF, CRC-32 IEEE over type and data via hash/crc32) and orientationChunkWebP(o int) []byte (type EXIF, little-endian size, the block). Same contract as Task 1: o is already 2..8.

3. internal/media/strip.go, stripPNG: the first eXIf chunk is claimed; read o with tiffOrientation over its payload; when 2..8 write orientationChunkPNG(o) in its place; the chunk itself is dropped as before; later eXIf chunks are dropped. Position is the source's own position (C-1); do not move it. stripWebP: remember, when the first chunk is VP8X with a payload of at least 10 bytes, the offset of its first payload byte inside the output body; on the first EXIF chunk take the payload with the optional exifHeader prefix removed (the same rule webpOrientation applies), read o with tiffOrientation, and only when o is 2..8 AND the VP8X offset is known write orientationChunkWebP(o) and remember to set bit 0x08 of that flags byte after the loop (C-3). Without VP8X: drop the chunk as today, write nothing. Orientation 1 or absent never touches the flags. The RIFF size is computed from the body as already done. Update the stripWebP and stripPNG comments to say what survives.

4. Replace the Task 1 hostile test by the shared helper so one function holds the truncate-and-overwrite loop for the three formats.

Commit atomically (feat(quick-261026-sxy): ...).
  </action>
  <verify>
    <automated>go test ./internal/media -count=1 && go vet ./internal/media && test -z "$(gofmt -l internal)" && test "$(grep -v '^\s*//' internal/media/strip.go | grep -c 'tiffOrientation')" -ge 3</automated>
  </verify>
  <done>PNG and WebP with orientation 2 to 8 keep a valid 26-byte orientation block and nothing else of their EXIF; the PNG CRC and the WebP RIFF size and VP8X flag are right and the files decode; orientation 1, absent and a simple-container WebP give the old bytes; derived copies of a tagged PNG stand upright; hostile input never panics in any of the three strippers.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: the crop tool honours the tag, and docs/publishing.md says what is true now</name>
  <files>internal/media/orient.go, internal/media/crop.go, internal/media/orient_keep_test.go, docs/publishing.md</files>
  <read_first>internal/media/crop.go (ApplyCrop, RestoreOriginal, rotate), internal/media/crop_test.go (testImage and the existing crop tests, whose behaviour for untagged files must not change), internal/admin/media_crop_test.go (Dimensions of the .original copy), docs/publishing.md lines 125-175 (the Media section)</read_first>
  <behavior>
    - TestCropStandsUpright: a 1000x500 quadrant JPEG with orientation 6 as the served file in t.TempDir. ApplyCrop with the zero Crop returns 500x1000, and the served file afterwards decodes to 500x1000 with the quadrants in the order orientation 6 gives, carries no Exif and orientationOf == 1. ApplyCrop with Rotation 90 on the same file returns 1000x500 with the rows D C over B A (the editor's turn is applied on top of the upright picture). A square crop (Ratio "1-1") returns 500x500.
    - TestOriginalKeepsTheTagAndRestores: after the crop, HasOriginal is true and the .original copy still carries orientation 6; RestoreOriginal returns 500x1000 and the restored served file is byte-identical to the upload.
    - G: every existing crop test passes unchanged (untagged files take the old path; uprightImage returns its argument for orientation 1).
    - TestUprightImage: orientation 1 and values outside 1..8 return the same image value; for 2..8 the result has the upright bounds and the pixel table of TestOrientTransformTable for a YCbCr source (a JPEG decode), an NRGBA source (a PNG decode) and an RGBA source.
  </behavior>
  <action>
Implements O-5 (crop) and O-8, decisions C-4 and C-5.

1. Tests first, record red and green, then implement.

2. internal/media/orient.go: add uprightImage(src image.Image, o int) image.Image. For o outside 2..8 return src itself. Otherwise draw src into an image.RGBA of its own bounds with golang.org/x/image/draw (already a dependency of this package) and return orientRGBA of that. Comment the memory: the tagged case holds the decoded picture, one RGBA copy and the turned result, bounded by the megapixel budget that ApplyCrop checks before decoding; the turn of a 24 MP picture is a deliberate, rare, interactive cost, and untagged pictures cost what they cost today.

3. internal/media/crop.go, ApplyCrop: replace the Dimensions call by look on the source copy; keep the budget check on width*height; after image.Decode apply uprightImage(src, o) before the existing rotate call, so the editor's rotation, the focus point and the crop rectangle are all measured on the upright picture, which is the one the editor is looking at (the file's own comment says so). RestoreOriginal needs no edit: its Dimensions calls now answer the upright size. Update the ApplyCrop comment with one sentence on the tag.

4. docs/publishing.md, Media section, English, in the file's own voice. (a) In the paragraph on camera data being stripped: say that one number is kept, the orientation, when the photo says it has to be turned (2 to 8): a minimal note that holds only this number stays in the file and no coordinates, device, time or thumbnail, and say why (without it browsers show the photo on its side). Keep the sentence that the pixels stay bit-for-bit the same. (b) Replace the passage that ends the "What the copy keeps" paragraph, the one that begins by counting the remaining limits and says the strip takes the orientation note away together with the rest of the block: state that a picture stored without scaling (answer No, or not over the limit, or uploaded by AI key or upload link) keeps that one orientation note and nothing else of its EXIF block; that the scaled copy carries none because it is turned in the pixels; that the smaller copies for different screen widths, the cropped picture and the recorded width and height are made from the upright picture and carry no note; that one limit remains, the smaller copies are made without a colour profile; and that pictures uploaded before this change were stored without the note and stay as they are. Do not quote the old wording. Do not touch any other page under docs/.

5. Final gates, run and recorded in the SUMMARY: the verification section below, including the full ./internal/admin run in the background.

Commit atomically (docs and code may be two commits: feat(quick-261026-sxy): ..., docs(quick-261026-sxy): ...).
  </action>
  <verify>
    <automated>go test ./internal/media -count=1 && go test ./internal/admin -run 'Crop|TestScale|TestAPhoto|TestTheServeRoute' -count=1 && go vet ./internal/media ./internal/admin && test -z "$(gofmt -l internal)" && test "$(grep -c 'removes the orientation tag' docs/publishing.md)" = 0 && test "$(grep -c 'Two limits remain' docs/publishing.md)" = 0 && test -z "$(git status --short -- docs/beta.md CHANGELOG.md deploy .planning/phases/28-the-promise-in-writing/28-check.sh)"</automated>
  </verify>
  <done>A cropped tagged photo is upright and untagged-file crops are unchanged; the upload copy keeps its tag and restores byte for byte; docs/publishing.md states the orientation note, the scaled copy's lack of one, the upright derived copies and the one remaining limit, and no other doc changed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| uploaded bytes to the three strippers | The sender chooses every byte. Check has verified only kind and size. stripJPEG, stripPNG, stripWebP and the readers run on them before anything is trusted, and now decide whether to write a stub |
| source EXIF to stored file | The stored file is served publicly for a year with an immutable cache header; whatever the stub carries is published |
| stored file to derived copies | MakeVariants and ApplyCrop decode a file that now carries a tag and must not turn it twice or leave it unturned |

## STRIDE Threat Register

Threat IDs are unique within this quick task (earlier T-rrh-NN and T-bon-NN belong to other tasks).

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-sxy-01 | Information disclosure | The stub leaking GPS, device, time, thumbnail or maker notes from the source EXIF | critical | mitigate | The stub is built from the validated integer 2..8 alone by orientationTIFF; no source byte is copied. TestStripKeepsOnlyTheOrientationJPEG, ...PNG and ...WebP use a rich multi-IFD fixture and assert bytes.Equal against clean-plus-stub, the absence of every marker string, and that the stub's IFD0 has exactly the tag 0x0112 and no next IFD |
| T-sxy-02 | Denial of service | Panic or out-of-range slice in the stub code on hostile segment lengths, chunk lengths, RIFF sizes | high | mitigate | The new code reuses lengths the existing strippers have already bounds-checked, calls tiffOrientation which bounds every offset, and writes only fixed-size stubs; TestStripJPEGHostileInput and TestStripHostileInputPNGWebP truncate at every length and overwrite head and tail offsets with 0x00, 0xFF, 0x7F under recover, plus stacked EXIF segments and lying lengths |
| T-sxy-03 | Tampering | A malformed stub making a stored file undecodable or invalid (bad PNG CRC, wrong RIFF size, missing VP8X flag) | high | mitigate | PNG CRC recomputed by the test; RIFF size equals len-8; the VP8X EXIF bit is set and a stub is written only when VP8X is the first chunk; image/jpeg, image/png and the x/image webp decoder must accept the output and report the source's bounds |
| T-sxy-04 | Tampering | Double rotation: a scaled copy that carries a tag after it was already turned | high | mitigate | Scaled's encoders write no EXIF, so the strip has nothing to turn into a stub; TestScaledCopyGetsNoStubOnKeep and the admin Yes test assert orientation 1 and no Exif on the stored scaled copy while the non-scaled copy keeps tag N |
| T-sxy-05 | Tampering | Derived copies or recorded sizes inconsistent with the original (sideways thumbnails, swapped width and height in srcset and preview) | medium | mitigate | Dimensions, MakeVariants and ApplyCrop work on the upright picture (C-4, C-5); tests cover the upright size, thumbnail pixel order, variant records, the crop result and restore; untagged files are pinned to identical bytes |
| T-sxy-06 | Denial of service | Memory: Dimensions now reads the whole file; the crop of a tagged picture holds an extra RGBA copy | medium | accept | Dimensions is called once per upload, backfill item or crop, never per request, on a file already bounded by the upload limit. The crop turn is bounded by the megapixel budget checked before decoding and applies only to a tagged file; the extra copy is about the size of the decoded bitmap |
| T-sxy-07 | Information disclosure | The orientation number as a fingerprint of how the camera was held | low | accept | Owner decision O-1: this is the one thing kept, on purpose; it reveals nothing about place, device or time |
| T-sxy-08 | Information disclosure | Malformed files for which the strip errors and PrepareUpload stores the raw bytes | medium | accept | Pre-existing behaviour, untouched and out of scope (see the objective); this task adds no new error path that stores raw bytes |
| T-sxy-SC | Tampering | npm/pip/cargo installs | high | mitigate | No package-manager install in this plan; the dependency guard asserts go.mod and go.sum are unchanged (last touched 4465375); hash/crc32, encoding/binary and golang.org/x/image/draw are already available |
</threat_model>

<verification>
Run once all three tasks are committed. Record each baseline; a gate that already holds at HEAD is an invariant guard, not proof of the work.

1. `go test ./internal/media -count=1` (baseline green in about 22 s; the stub, upright-size and variant tests fail before the code, the characterization tests marked G pass before it).
2. `go test ./internal/admin -count=1 -timeout 30m` in the background (baseline green in about 405 s; never run it in the foreground). Then `go build ./...`, `go vet ./...`, `go test ./... -timeout 30m` as the tree-wide net, and `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dev/null ./cmd/holzcloud`.
3. `test -z "$(gofmt -l internal cmd tools)"` (baseline: true).
4. `go run ./tools/i18n` ends in 0 open, 0 orphaned (baseline: green; no operator-facing text is added or changed).
5. Dependency guard: `test "$(git log -1 --format=%h -- go.mod go.sum)" = "4465375"` (baseline: true; must stay true).
6. Wiring guard: `grep -v '^\s*//' internal/media/strip.go | grep -c tiffOrientation` is at least 3 (baseline 0); the same count over variants.go and crop.go for `look(\|orientRGBA\|uprightImage` is at least 1 each (baseline 0).
7. Docs: `grep -c 'removes the orientation tag' docs/publishing.md` and `grep -c 'Two limits remain' docs/publishing.md` are 0 (baseline 1 each); `git diff --name-only db1c3f4..HEAD -- docs` lists docs/publishing.md and nothing else.
8. Scope guard: `git status --short` and `git diff --name-only <plan commit>..HEAD` show nothing for docs/beta.md, CHANGELOG.md, deploy/, .planning/phases/28-the-promise-in-writing/28-check.sh, go.mod, go.sum. The `frozen` gate of 28-check.sh is known red and accepted by the owner (O-8): it is neither run nor edited nor mentioned as a failure to fix.
9. No tag created, nothing pushed (O-9).
</verification>

<success_criteria>
- A portrait phone photo under 2560 px (JPEG, PNG or WebP with orientation 2 to 8) is stored with a 26-byte orientation block and shows upright; no GPS, device, time, thumbnail or maker note is in the stored file.
- Orientation 1, no tag and unreadable tags store the bytes the strip gave before.
- A scaled copy carries no tag and is turned once; the non-scaled copy of the same upload carries tag N.
- Recorded width and height, thumbnails, the srcset candidates and a cropped picture are upright and carry no tag; the serve route is unchanged.
- Hostile input never panics; all gates above hold; the docs state what is true now.
</success_criteria>

<output>
Create `.planning/quick/261026-sxy-stripmetadata-behaelt-exif-ausrichtung-a/261026-sxy-SUMMARY.md` when done, with the red and green test lists of Tasks 1 and 2, the decision record C-1 to C-6 as built, and a Deviations section (the one edited assertion in TestScaleQuestionUsesTheUprightPicture, and anything else that differs from this plan).
</output>
