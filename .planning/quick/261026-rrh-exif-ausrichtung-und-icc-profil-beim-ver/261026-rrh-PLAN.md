---
phase: quick-261026-rrh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/media/orient.go
  - internal/media/orient_test.go
  - internal/media/profile.go
  - internal/media/profile_test.go
  - internal/media/scale.go
  - internal/media/staging.go
  - internal/admin/media_scale_test.go
  - docs/publishing.md
autonomous: true
requirements: [QUICK-261026-rrh]

estimate:
  tokens: 110000
  raw_tokens: 110000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "A picture answered Yes on the scale-down question stands upright: EXIF orientation 1-8 is applied to the pixels (JPEG APP1, PNG eXIf, WebP EXIF), so a portrait phone photo stored as a landscape bitmap with orientation 6 comes out portrait"
    - "The stored scaled copy carries no orientation tag and no EXIF at all, so a browser cannot turn it a second time"
    - "The 2560 px longest side is computed on the upright geometry: for every orientation 1-8 the size ScaledSize gives for the upright picture is exactly the size of the stored file"
    - "Oversize and the megapixel budget give the same verdict for the eight orientations (the longest side and the pixel count do not change under a quarter turn); only the order of the two numbers changes"
    - "The question page shows the upright size: a bitmap of 3000 x 1500 with orientation 6 is announced as 1500 x 3000 px, becomes 1280 x 2560 px - never the stored 3000 x 1500"
    - "A valid RGB ICC colour profile is carried into the scaled copy: JPEG to JPEG as APP2 ICC_PROFILE chunks, PNG to PNG as iCCP, WebP ICCP to the JPEG or PNG the WebP becomes; it survives Keep (StripMetadata keeps both)"
    - "A profile that is not RGB (CMYK, greyscale), not well formed, too large or not reassemblable is not carried and is not an error; the result is exactly what the program wrote before"
    - "A picture with no EXIF orientation (absent, 1, unreadable) and no carriable profile yields byte-identical scaled bytes to the pre-change pipeline (pinned by a test that carries the old three steps as its reference)"
    - "Answer No, uploads that are never asked, the AI upload path, scaledFormat (GIF to PNG, WebP to JPEG or PNG by transparency) and the smaller derived copies are untouched"
    - "No new dependency: go.mod and go.sum are as they were; only the Go standard library and golang.org/x/image are used"
  artifacts:
    - path: internal/media/orient.go
      provides: "orientationOf, tiffOrientation, orientRGBA, uprightOf, Intake.uprightSize"
    - path: internal/media/profile.go
      provides: "profileOf, validRGBProfile, withProfile (JPEG APP2 and PNG iCCP insertion)"
    - path: internal/media/scale.go
      provides: "Scaled applies orientation and carries the profile; Oversize returns the upright size"
    - path: internal/media/staging.go
      provides: "Staged manifest holds the upright size"
    - path: internal/media/orient_test.go
      provides: "Table of the eight transforms, EXIF reader in both byte orders, hostile input, upright sizes, byte-identity regression"
    - path: internal/media/profile_test.go
      provides: "Profile carried per format, survives Keep, refused profiles leave the output untouched"
    - path: internal/admin/media_scale_test.go
      provides: "End to end: orientation-6 upload, question page shows the upright size, Yes stores it upright"
    - path: docs/publishing.md
      provides: "The known-limits passage of the Media section, true again"
  key_links:
    - from: internal/media/scale.go (Scaled)
      to: "internal/media/orient.go and internal/media/profile.go"
      via: "orientationOf and orientRGBA after the scale, profileOf and withProfile after the encode, both before the byte-limit check"
    - from: internal/media/staging.go (Put)
      to: "Intake.uprightSize"
      via: "the manifest holds the size the picture has when it is looked at, which is what the question page prints"
    - from: internal/admin/media_scale.go (HandleMediaScaleForm)
      to: "media.ScaledSize applied to the staged (upright) size"
      via: "no code change - the page reads the manifest; the end-to-end test pins that the printed 'after' equals the stored size"
---

<objective>
Make "Yes, scale down" lose nothing the picture needs. 261026-bon built the question and the scaling; its Known limits say the scaled copy ignores the EXIF orientation and drops the colour profile. This quick task closes both, for the Yes path only.

Brief items (the owner's words, numbered so tasks can cite them; nothing here is open for renegotiation):
- B-1 Apply EXIF orientation 1-8 when the answer is Yes, so a portrait photo stands right after scaling. Do not carry or duplicate the orientation tag.
- B-2 The 2560 longest side is computed on the rotated (upright) geometry. The Oversize check and the pixel budget stay consistent with that. The sizes the question prints before and after are decided and tested.
- B-3 Carry the ICC colour profile over where the format allows (JPEG APP2 ICC_PROFILE chunks, PNG iCCP), using only the Go standard library and golang.org/x/image that go.mod already has. No new dependency.
- B-4 Pictures without EXIF orientation or ICC profile behave byte for byte as before. GIF and WebP conversions (which format a GIF or a WebP becomes) are as before.
- B-5 Tests with generated fixtures: orientation 1-8, profile kept, none unchanged.
- B-6 `go run ./tools/i18n` stays at 0 open, 0 orphaned (if any text changes; none is planned). `go build`, `go vet`, `go test ./...` green.
- B-7 Update the known-limits passage in docs/publishing.md (the 261026-bon work documented the scale-down and "the colour profile is not carried over" there; nothing about it is in content-model.md). Do NOT edit docs/beta.md, CHANGELOG.md, deploy/*, deploy/test/nightly.sh or .planning/phases/28-the-promise-in-writing/28-check.sh.
- B-8 `bash .planning/phases/28-the-promise-in-writing/28-check.sh all` must stay green. SEE THE CONFLICT BELOW - this item cannot be met as worded.
- B-9 No push, no tag.

Decisions this plan makes where the brief says "decide" or is silent (each is a Claude's-discretion call; the executor implements them as written):
- Which formats: orientation is read from JPEG (APP1 "Exif"), PNG (eXIf chunk) and WebP (EXIF chunk), because each of them can be a source of a Yes. GIF has no EXIF. "GIF/WebP conversions as before" is read as: the choice of target format (scaledFormat) does not change. A WebP that carries an orientation or a profile gets them honoured like any other source. If the owner meant WebP to be left alone entirely, that is one switch case each in orientationOf and profileOf to remove.
- Order of work: scale first in the stored geometry, then turn the scaled bitmap (a pure pixel permutation of at most 2560 x 2560, so the resampling quality is exactly what it was and the memory stays bounded). The final size is ScaledSize of the upright size; ScaledSize is symmetric under swapping the two sides, so it equals the stored-geometry size swapped back - a test pins that symmetry.
- The question shows what the visitor will see: "before" is the upright size, "after" is ScaledSize of it, and "after" equals the stored file's size exactly. Staging.Put therefore stores the upright size in the manifest and the admin page needs no change.
- Oversize and the pixel budget: max(width, height) and width x height do not change under a quarter turn, so the verdict is the same for orientation 1-8 by construction. Oversize returns the upright pair (the numbers swap for 5-8), and a test asserts the verdict does not vary with the orientation.
- Profile rule: carry a profile only when its header says the colour space is "RGB " and it is well formed (declared size equals its length, 'acsp' signature, at most 4 MiB). The scaled copy is always an RGB bitmap; a CMYK or greyscale profile attached to RGB pixels would colour them wrongly, so it is left off - the same result as before this change. Never an error.
- Name written into an iCCP chunk: the constant "ICC profile".

CONFLICT the executor must not paper over (B-8 versus B-1..B-7):
`28-check.sh all` runs `frozen` (check_frozen). It compares committed HEAD with 85fa486 and fails if anything outside .planning, docs and CHANGELOG.md changed ("something outside .planning, docs and CHANGELOG.md changed after 85fa486") or if any page of docs other than beta.md changed ("a page of docs other than beta.md changed after 85fa486"). At planning time both comparisons are quiet and `all` exits 0 (checked). This task must commit changes under internal/ (B-1..B-5) and to docs/publishing.md (B-7) and must not touch 28-check.sh, so `frozen` WILL fail after the first code commit - there is no way to make `all` fully green under these rules. Phase 28 is complete; `frozen` is that phase's own freeze gate and is expected to go red once the program moves on.
What the plan does instead of pretending: Task 3 runs `all` and requires that the ONLY failures are those two named lines of `frozen` (every other sub-check - beta, owner-steps, changelog, regression, and the two deploy-binding diffs of frozen itself - stays green). That proves nothing else the phase guards was disturbed. The executor reports the red `frozen` to the owner in the SUMMARY under Deviations and does not edit 28-check.sh, beta.md, CHANGELOG.md or anything under deploy/ to hide it. The owner decides afterwards between accepting the red gate and re-pinning it in a separate, explicitly authorised change.

Purpose: a phone photo uploaded at full size and scaled by the question must not come out lying on its side, and a wide-gamut photograph must not come out dull.
Output: orient.go, profile.go (new, stdlib and x/image only), changes to scale.go and staging.go, tests, one docs passage.

Verified facts this plan relies on (read at planning time):
- `Intake.content` at Scaled time holds the ORIGINAL uploaded bytes, EXIF and all; Check does not strip, Keep does (PrepareUpload -> StripMetadata). So the orientation and the profile are readable in Scaled. `Scaled` returns a new Intake whose bytes come from the standard encoders (no EXIF, no profile); `Keep` then runs StripMetadata on that result, which keeps APP2 "ICC_PROFILE" segments and PNG iCCP chunks (strip.go keepJPEGSegment and pngDropChunks), so a carried profile survives storage.
- `Staging.Put` stores the intake's original bytes plus a manifest with the size from image.DecodeConfig (the stored geometry) and the admin page prints `media.ScaledSize` of exactly that. For orientation 5-8 that is wrong in both numbers; this is why Put must change.
- `Intake.Oversize` is used by uploadOne only as a boolean (internal/admin/media.go); its width and height results are read by tests.
- image.DecodeConfig for webp works because internal/media/variants.go blank-imports golang.org/x/image/webp. internal/media/testdata holds opaque-lossless.webp, opaque-lossy.webp and transparent.webp.
- The admin test helper `photo` (internal/admin/ops_media_test.go) writes an APP1 whose payload is "Exif" plus a text marker and no TIFF structure at all. orientationOf must read that as "no orientation" without a panic or an error, or existing tests break.
- go.mod/go.sum were last changed in commit 4465375; the dependency guard compares against that.
- No EXIF orientation handling exists anywhere in the Go sources (grep). StripMetadata drops the whole EXIF block, orientation included, for every upload that is not scaled - a pre-existing limit this task documents and does not change (see Out of scope).
- Baseline at planning time: `28-check.sh all` (GO=0 NATIVE_SELFTEST=0) exits 0 with no FAIL line.

Out of scope, flagged for the owner, NOT planned here:
- StripMetadata removes the EXIF block, orientation tag included, from every upload that is stored without scaling (answer No, small pictures, AI and upload-link paths). A portrait phone photo under 2560 px is therefore stored with its landscape bitmap and no tag, and browsers show it on its side. Fixing that means deciding what an orientation-only EXIF stub looks like next to the "bit-for-bit" promise of the strip; it is a separate quick task. This plan only says so in the docs (Task 3).
- Smaller derived copies (responsive sizes) and the crop tool: untouched, still made without a profile and without orientation.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@.planning/quick/261026-bon-block-bild-und-text-bildlage-links-recht/261026-bon-SUMMARY.md
@internal/media/scale.go
@internal/media/staging.go
@internal/media/strip.go
@internal/media/scale_test.go
@internal/media/variants.go
@internal/admin/media_scale.go
@internal/admin/media_scale_test.go
@internal/admin/ops_media_test.go
@docs/publishing.md
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): a JPEG with orientation 6 goes upload - question - Yes - stored upright, with the question printing the upright sizes</name>
  <files>internal/media/orient.go, internal/media/orient_test.go, internal/media/scale.go, internal/media/staging.go, internal/admin/media_scale_test.go</files>
  <read_first>internal/media/scale.go (ScaledSize, Oversize, Scaled), internal/media/staging.go (Put, StagedFile), internal/media/strip.go (the JPEG segment walk to imitate, not to modify), internal/media/scale_test.go (gradient, jpegBytes, intakeOf, dimensionsOf helpers - reuse them, same package), internal/admin/media_scale_test.go (serveUser, mediaUploadRequest, tokenOf, libraryOf, sizeOnDisk, TestALargeUploadIsHeldBackAndAsked for the page assertions), internal/admin/ops_media_test.go (photo helper)</read_first>
  <behavior>
    - FIRST, before touching scale.go: write TestPlainPictureBytesDoNotChange and run it against the unmodified code. It carries the pre-change pipeline as its own reference (image.Decode, ScaledSize of the decoded config, image.NewRGBA, draw.CatmullRom.Scale with draw.Over, jpeg.Encode at quality 90 or png.Encode) and asserts bytes.Equal with Intake.Scaled for a JPEG without EXIF, a JPEG whose EXIF is the text-only block of the `photo` helper, a JPEG with a real TIFF EXIF whose orientation is 1, a PNG and a still GIF (PNG output). It must pass at HEAD (that is the proof the reference equals the old behaviour) and stay green afterwards.
    - Orientation table (TestOrientTransformTable, pixel exact, no JPEG involved): a 3 x 2 RGBA with six distinct colours laid out A B C over D E F; orientRGBA with 1..8 gives, as rows of the upright image: 1 ABC/DEF; 2 CBA/FED; 3 FED/CBA; 4 DEF/ABC; 5 AD/BE/CF; 6 DA/EB/FC; 7 FC/EB/DA; 8 CF/BE/AD. Write the expected layouts as literal letters in the test, not as a second copy of the coordinate formulas. orientRGBA(img, 1) returns the same image, not a copy.
    - TestTiffOrientationParse: a hand-built TIFF block (byte order "II" and "MM", magic 42, IFD0 at offset 8, one 12-byte SHORT entry, tag 0x0112) gives 1..8 for the values 1..8; values 0, 9 and 65535, a wrong type (not SHORT), a count other than 1, an IFD offset past the end, an entry count that runs past the end, an empty block, a block that is only the header, and the payload "Exif" followed by text all give 1. A helper builds the block so the table stays readable.
    - TestOrientationFromJPEG: APP1 with prefix "Exif" and two zero bytes, then the TIFF block, placed AFTER a JFIF APP0 and before the scan, is found; an APP1 that starts with the XMP namespace header is skipped and a later Exif APP1 is still found; a JPEG with the orientation only after SOS is not searched (1).
    - TestScaledAppliesJPEGOrientation: a 3000 x 1500 JPEG whose four quadrants are flat distinct colours (top-left, top-right, bottom-left, bottom-right) with EXIF orientation 6, through intakeOf and Scaled(0): the output decodes as 1280 x 2560 (upright is 1500 x 3000), the colour at the centre of each upright quadrant is the one the table gives within a small tolerance (JPEG is lossy; sample well inside each quadrant), the output contains no APP1 segment and no "Exif" string, orientationOf(output) is 1, and the source intake's bytes are unchanged.
    - TestUprightSizeAndOversizeAgree: for each orientation 1..8 build a 3000 x 1500 JPEG with that tag. Oversize(0) returns ok=true in all eight with the upright pair (3000 x 1500 for 1-4, 1500 x 3000 for 5-8); Oversize(2) (a budget of 2 MP, below 4.5 MP) is false in all eight; a 2560 x 1000 picture with orientation 6 (upright 1000 x 2560, longest side exactly 2560) is not oversize. Staging.Put on a NewStaging(t.TempDir()) stores the same upright pair in Files(). ScaledSize of that pair equals the dimensions of Scaled's output for all eight.
    - TestScaledSizeIsSymmetric: for a spread of pairs including equal sides, a side of 1, 2559, 2560, 2561, 3000, 4000, 10000: ScaledSize(h, w) is the swap of ScaledSize(w, h).
    - Admin end to end (TestScaleQuestionUsesTheUprightPicture in internal/admin/media_scale_test.go): upload a 3000 x 1500 JPEG with EXIF orientation 6 through HandleMediaUpload; read the token; GET the question: the line for the file reads 1500 x 3000 px and becomes 1280 x 2560 px (check how the template prints the multiplication sign in the real output and assert on that), and 3000 x 1500 does not appear as the file's size; POST Yes: the library has one file, sizeOnDisk is 1280 x 2560, the bytes hold no "Exif"; a second upload of the same picture answered No stores the original at 3000 x 1500 (unchanged path).
    - The existing tests in media_scale_test.go and scale_test.go still pass unchanged.
  </behavior>
  <action>
Create internal/media/orient.go with these pieces (names are fixed so later tasks and tests agree). B-1, B-2.

orientationOf(mimeType string, data []byte) int returns 1-8 and returns 1 for everything it cannot read - no error, no panic, never a value outside 1-8. In this task it handles "image/jpeg" only (walk the marker chain the way stripJPEG does, stop at SOS or EOI, look at APP1 segments whose payload begins with Exif followed by two zero bytes, hand the remainder to tiffOrientation, keep looking after an APP1 that is not Exif such as XMP); every other MIME type returns 1 for now. Check every length and offset against the slice before using it, so a hostile segment length cannot slice out of range.

tiffOrientation(tiff []byte) int parses a TIFF block: bytes 0-1 are II or MM, bytes 2-3 the number 42 in that byte order, bytes 4-7 the offset of IFD0 from the start of the block. IFD0 is a two-byte entry count followed by 12-byte entries (tag, type, count, four value bytes). Return the SHORT value of tag 0x0112 only when its type is 3 and its count is 1 and the value is 1-8; anything else is 1. Cap the number of entries examined by what fits in the slice. Keep it shared: the PNG and WebP readers of Task 2 hand their blocks to it.

swapsAxes(o int) bool is true for 5-8. uprightOf(w, h, o int) (int, int) swaps the pair when swapsAxes. orientRGBA(src *image.RGBA, o int) *image.RGBA returns src itself for 1 (no copy: that is what keeps the unchanged case byte-identical) and otherwise a new RGBA, copying four Pix bytes per pixel by index arithmetic rather than SetRGBA per pixel. The EXIF values mean: 2 mirror left-right, 3 turn 180, 4 mirror top-bottom, 5 transpose, 6 turn 90 degrees clockwise to reach upright, 7 transverse, 8 turn 90 degrees counter-clockwise to reach upright; 5-8 swap width and height. The literal table in the behaviour list is the arbiter if a formula and the table disagree.

Add the method (in *Intake) uprightSize() (w, h int, err error): image.DecodeConfig on in.content for the stored pair, then uprightOf with orientationOf(in.MimeType, in.content). Header only, never a pixel decode.

Change internal/media/scale.go. In Oversize, take the pair from uprightSize instead of the bare DecodeConfig and return the upright pair; the longest-side test and the megapixel test stay exactly where they are and read the upright pair (they are quarter-turn invariant; do not add any orientation-dependent branch). In Scaled, keep the early DecodeConfig and megapixel refusal (stored pair), decode as before, then: o := orientationOf(in.MimeType, in.content); the upright pair is uprightOf(cfg.Width, cfg.Height, o); the final size is ScaledSize of the upright pair; the bitmap to scale into has the final size swapped back when swapsAxes(o) (the stored geometry), is filled with the same draw.CatmullRom.Scale call, and then goes through orientRGBA(dst, o) before encoding. Everything after - scaledFormat, hasTransparency of src, the encoder and its quality, the byte-limit check, the Intake built - is untouched. Update the doc comments of Oversize and Scaled: the size is the upright size, orientation is applied, the result carries no orientation tag (the encoders write none) and no EXIF; leave the sentence about the profile for Task 3, and say plainly meanwhile that the profile is not yet carried so the comment stays true at this commit.

Change internal/media/staging.go: in Put replace the image.DecodeConfig call by in.uprightSize() (same failure handling), and say in the StagedFile comment that Width and Height are the size the picture has when it is looked at. Remove the image import only if nothing else in the file uses it.

Write the tests named in the behaviour list. In orient_test.go add small helpers: a function that builds the TIFF block for an order and a value; one that inserts an APP1 Exif segment into JPEG bytes right after SOI (the existing scale_test builds one by hand - copy the approach, do not move scale_test code); one that makes the four-quadrant picture. Reuse gradient, jpegBytes, intakeOf, dimensionsOf from scale_test.go; do not redefine them. Add TestScaleQuestionUsesTheUprightPicture to internal/admin/media_scale_test.go using the existing helpers; that file needs a helper that returns a JPEG with a real orientation tag - build it from the `photo` bytes plus a TIFF block, do not change `photo`. New strings: none. Per B-4 nothing about format selection or the No path changes.
  </action>
  <verify>
    <automated>go test ./internal/media -run 'TestPlainPictureBytesDoNotChange|TestOrient|TestTiffOrientationParse|TestScaledAppliesJPEGOrientation|TestUprightSizeAndOversizeAgree|TestScaledSizeIsSymmetric|TestScaled|TestOversize|TestStaging' -count=1 && go test ./internal/admin -run 'TestScaleQuestionUsesTheUprightPicture|TestALargeUploadIsHeldBackAndAsked|TestYesStoresAScaledCopyWithoutCameraData|TestNoStoresTheOriginal' -count=1 && go build ./... && go vet ./internal/media ./internal/admin</automated>
  </verify>
  <done>An orientation-6 JPEG answered Yes is stored 1280 x 2560 and upright with no EXIF; the question page printed 1500 x 3000 px, becomes 1280 x 2560 px; the byte-identity test passed before the change and after it; the existing scale and staging tests are green; the change is committed (one commit, `feat(quick-261026-rrh): scale-down applies the EXIF orientation`).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: orientation from PNG eXIf and WebP EXIF, all eight values through Scaled, hostile input</name>
  <files>internal/media/orient.go, internal/media/orient_test.go</files>
  <read_first>internal/media/orient.go (as Task 1 left it), internal/media/strip.go (stripPNG and stripWebP show how both chunk chains are walked), internal/media/variants.go (blank import of the webp decoder), internal/media/testdata (the three WebP files)</read_first>
  <behavior>
    - TestOrientationFromPNG: a PNG with an eXIf chunk placed after IHDR holding the bare TIFF block (no Exif prefix) is read; the same chunk placed after IDAT is read too (the PNG specification allows it anywhere between IHDR and IEND, outside the IDAT run); a CRC that is wrong does not matter to the reader; a chunk length running past the end gives 1; the signature missing gives 1.
    - TestOrientationFromWebP: a RIFF/WEBP container with an EXIF chunk holding the TIFF block is read; the same with the six-byte "Exif" prefix some writers put in front (tolerated); an odd-sized chunk is padded to even before the next one; a chunk size or RIFF size that lies gives 1. Build these as plain byte containers for the reader; if the x/image webp decoder also accepts the container with an added EXIF chunk after a testdata image, add one Scaled round trip, otherwise the reader-level test stands alone and the plan says nothing more about it.
    - TestScaledAppliesEveryOrientation: PNG source (lossless, so the colours are exact), 3000 x 1500 stored, four flat quadrants, eXIf orientation 1..8 in turn, Scaled(0): for each the output dimensions equal ScaledSize of the upright pair (2560 x 1280 for 1-4, 1280 x 2560 for 5-8), and the colour at the centre of each upright quadrant equals the table exactly: with the stored quadrants as A B over C D the upright layouts are 1 AB/CD; 2 BA/DC; 3 DC/BA; 4 CD/AB; 5 AC/BD; 6 CA/DB; 7 DB/CA; 8 BD/AC. The PNG output holds no eXIf chunk and orientationOf(output) is 1.
    - TestOrientationNeverPanics: every prefix of a valid Exif APP1 JPEG, of a valid eXIf PNG and of a valid EXIF WebP, and the same inputs with each length byte and each IFD offset byte overwritten with 0x00 and 0xFF, run through orientationOf and give a value in 1..8 without panicking. Also nil and empty slices.
    - The Task 1 tests stay green.
  </behavior>
  <action>
Extend orientationOf in internal/media/orient.go (B-1). Add the "image/png" case: check the 8-byte signature, then walk the chunks (4-byte big-endian length, 4-byte type, payload, 4-byte CRC) over the whole stream as stripPNG does, stopping at IEND or on a length that does not fit, and pass the payload of the first eXIf chunk to tiffOrientation. Do not verify the CRC (a reader that refuses a chunk over its CRC would silently skip a tag every browser reads; the Go decoder has already accepted the file or the upload was refused earlier). Add the "image/webp" case: check RIFF and WEBP, walk the chunks (four-character type, four-byte little-endian size, payload padded to even) and pass the payload of the first EXIF chunk to tiffOrientation after removing a leading Exif-and-two-zero-bytes prefix if present. Every offset checked against the slice before use; a malformed container returns 1. "image/gif" and everything else stay 1 (GIF carries no EXIF).

Scaled needs no further change: Task 1 already calls orientationOf for every MIME type. Add the tests from the behaviour list. For the PNG fixtures write a helper that inserts a chunk with a correct CRC (hash/crc32, IEEE table) after IHDR or after the last IDAT; for WebP, a helper that assembles RIFF, WEBP and chunks with correct padding. Keep the table of expected upright layouts as literal letters in the test.

Do not change the profile logic (Task 3) and do not touch strip.go.
  </action>
  <verify>
    <automated>go test ./internal/media -run 'TestOrient|TestTiffOrientationParse|TestScaledAppliesEveryOrientation|TestScaledAppliesJPEGOrientation|TestUprightSizeAndOversizeAgree|TestPlainPictureBytesDoNotChange' -count=1 && go vet ./internal/media</automated>
  </verify>
  <done>All eight orientations are applied exactly for a PNG source and the stored size equals the displayed "after" for each; PNG and WebP readers exist and survive every truncated or corrupted input; committed (`feat(quick-261026-rrh): orientation from PNG and WebP, all eight values tested`).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: carry the ICC profile into the scaled copy, update the docs passage, run every gate</name>
  <files>internal/media/profile.go, internal/media/profile_test.go, internal/media/scale.go, docs/publishing.md</files>
  <read_first>internal/media/strip.go (keepJPEGSegment keeps APP2 beginning ICC_PROFILE and a NUL; pngDropChunks leaves iCCP alone - the profile must survive Keep), internal/media/scale.go (as Tasks 1-2 left it), docs/publishing.md lines 127-160 (the Media section and the scale-down paragraph), the 28-check.sh header comment and check_frozen (to read the conflict in the objective once more)</read_first>
  <behavior>
    - Helper in the test: testProfile(size int, space string) builds a profile of exactly size bytes (at least 132): the first four bytes the size big-endian, bytes 16-19 the colour space, bytes 20-23 "XYZ ", bytes 36-39 "acsp", a tag count of zero, the rest filled with a recognisable pattern.
    - TestProfileCarriedJPEG: a 3000 x 1500 JPEG carrying a 300 kB RGB profile split into several APP2 chunks (ICC_PROFILE, NUL, sequence number, chunk count, then up to 65519 bytes of data) goes through Scaled; profileOf(output) equals the input profile byte for byte; every APP2 segment of the output has a segment length of at most 65535; the sequence numbers run 1..n with the right count; the output decodes. A 200-byte profile (one chunk) works too.
    - TestProfileCarriedPNG: a PNG with an iCCP chunk (name, NUL, method 0, zlib stream) goes through Scaled; the output's iCCP decompresses to the same bytes; the chunk sits after IHDR and before the first IDAT; the file decodes with image/png (which also proves the CRCs are valid).
    - TestProfileFromWebP: profileOf on a RIFF/WEBP container with an ICCP chunk returns the bytes; when the decoder accepts such a container, WebP to JPEG (opaque) and to PNG (transparent) both carry it, otherwise the reader-level test stands alone.
    - TestProfileSurvivesKeep: StripMetadata on the Scaled output (JPEG and PNG) still has the profile; the output still has no EXIF.
    - TestProfileNotCarriedWhenUnfit: each of these leaves the Scaled output byte-identical to the Task 1 reference pipeline - colour space "CMYK" and "GRAY", signature not "acsp", declared size different from the real length, profile larger than 4 MiB, a JPEG whose chunk sequence has a gap, a duplicate or a count that disagrees, an iCCP whose compression method is not 0, an iCCP whose zlib stream is corrupt, an iCCP that inflates beyond the cap (a small compressed bomb), and a GIF source. None returns an error.
    - TestScaledOrientationAndProfileTogether: an orientation-6 JPEG with a valid profile comes out upright and with the profile; the output has no APP1.
    - TestPlainPictureBytesDoNotChange (from Task 1) is green.
    - docs/publishing.md says what is true.
  </behavior>
  <action>
Create internal/media/profile.go (B-3), standard library only (bytes, compress/zlib, encoding/binary, hash/crc32, io) - no new module, go.mod and go.sum stay as they are.

const maxProfile = 4 << 20, with a comment saying real profiles run from a few hundred bytes to a few hundred kilobytes and that this is the cap on what is carried and on what an iCCP stream may inflate to.

profileOf(mimeType string, data []byte) []byte returns the source's ICC profile when validRGBProfile accepts it, else nil. image/jpeg: walk the segments (as the orientation reader does) collecting APP2 segments whose payload starts with ICC_PROFILE and a NUL, then a sequence number and a chunk count; reassemble in sequence order; return nil unless the numbers run 1..n exactly once each, every chunk states the same count, and the total is at most maxProfile. image/png: find the iCCP chunk (the first one), read the name up to its NUL, require compression method 0, inflate the rest through io.LimitReader(maxProfile+1) and return nil when the cap is exceeded or the stream is corrupt. image/webp: the ICCP chunk's payload. Everything else, nil.

validRGBProfile(p []byte) bool: at least 132 bytes, at most maxProfile, the big-endian size in bytes 0-3 equal to len(p), bytes 36-39 equal "acsp", bytes 16-19 equal "RGB " (note the trailing space). Document in the comment why a non-RGB profile is refused: the scaled copy is an RGB bitmap whatever the source was, so a CMYK or greyscale profile on it would colour it wrongly; leaving it off is what the program did before.

withProfile(mimeType string, encoded, profile []byte) []byte returns encoded unchanged when profile is nil or the encoded bytes do not have the expected shape. image/jpeg: insert APP2 segments immediately after the two SOI bytes; each segment is FF E2, a two-byte length, the twelve bytes ICC_PROFILE and NUL, the sequence number starting at 1, the total count, and up to 65519 bytes of the profile; at most 255 segments. image/png: insert one iCCP chunk immediately after IHDR (the first 33 bytes of the encoded file): length, type, the name "ICC profile" (a constant), a NUL, compression method 0, the zlib-compressed profile, and the CRC-32 (IEEE) over type and data. No other MIME type is ever written with a profile (a WebP is never an output; scaledFormat is untouched).

In internal/media/scale.go, after the encode and before the byte-limit check, read the profile from the source with profileOf(in.MimeType, in.content) and, when it is not nil, replace the encoded bytes with withProfile(mimeType, encoded, profile) where mimeType is the OUTPUT format. The limit check then measures the final bytes. Update the doc comment of Scaled so it states what is carried (an RGB profile, and the orientation is applied) and what is not (any other metadata; the result has no EXIF, XMP, IPTC or comment).

Update docs/publishing.md, the paragraph that begins "Very large images are offered for scaling down" (about lines 143-151). Replace the clause "the colour profile is not carried over and the original is not kept" by prose saying: a photo that carries an EXIF orientation is turned upright and the copy carries no orientation tag, and the sizes the question shows are those of the upright picture; an RGB colour profile (ICC) is carried into the copy, a profile for another colour model such as CMYK or greyscale is not, because the copy is always RGB; everything else in the metadata stays removed; the original is not kept. Add, as a stated limit and in the same register as the surrounding text, that the smaller derived copies are still made without a profile, and that a picture stored without being scaled (answer No, or under the limit) goes through the metadata strip described just above, which removes the orientation tag together with the rest of the EXIF block, so such a photo shows in the orientation of its bitmap. Do not change any other page of docs/ and do not touch docs/beta.md.

Then run every gate (all of them, once, at the end; fix and re-run until each one holds): gofmt -l internal cmd tools prints nothing; go build ./...; go vet ./...; CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dev/null ./cmd/holzcloud; go run ./tools/i18n reports 0 open, 0 orphaned for the four catalogues (no text changed, so no catalogue edit is expected; if it reports anything, the cause is a string added by this work and the fix is to remove or translate it); go run ./tools/themewords -check; go run ./tools/english; go test ./... -timeout 30m (the admin package alone takes about six minutes); the dependency guard below; the 28-check run below. Then write the SUMMARY with the Deviations section naming the red `frozen` sub-check.
  </action>
  <verify>
    <automated>go test ./internal/media -count=1 && go build ./... && go vet ./... && test "$(git log -1 --format=%h -- go.mod go.sum)" = "4465375" && test -z "$(gofmt -l internal cmd tools)"</automated>
  </verify>
  <done>A valid RGB profile from a JPEG, PNG or WebP source reaches the stored JPEG or PNG copy and survives Keep; every unfit profile leaves the output exactly as before; docs/publishing.md describes orientation, profile and the remaining limits truthfully; go.mod and go.sum untouched; all gates in the action pass; 28-check.sh all fails only on the two named frozen lines (see verification); committed (`feat(quick-261026-rrh): scale-down carries the RGB colour profile`, plus `docs(quick-261026-rrh): ...` for the publishing.md change if committed separately).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| uploaded file to EXIF/ICC parsers | The bytes are chosen by the sender; Check has verified only kind and size. orientationOf, tiffOrientation, profileOf run on them before anything is trusted |
| staged file on disk to the decision | A staging file is re-read at POST and goes through media.Check again; the manifest size is used only for display |
| source profile to output file | A profile read from the upload is written into a new file that is then served publicly for a year |

## STRIDE Threat Register

Threat IDs are unique within this quick task (the earlier T-bon-NN belong to 261026-bon).

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-rrh-01 | Denial of service | orientationOf, tiffOrientation (JPEG segment lengths, PNG chunk lengths, WebP sizes, IFD offsets and counts) | high | mitigate | Every length and offset is checked against the slice before use; any doubt returns orientation 1; entry loop bounded by the slice; TestOrientationNeverPanics feeds every prefix and every 0x00/0xFF overwrite of length and offset bytes plus nil and empty input |
| T-rrh-02 | Denial of service | iCCP inflate (a few bytes of zlib that expand to gigabytes) | high | mitigate | Inflate only through io.LimitReader(maxProfile+1); exceeding the cap drops the profile without error; the unfit-profile test includes a compressed bomb |
| T-rrh-03 | Denial of service | memory of the orientation turn and the extra bitmap | medium | mitigate | The turn is applied to the already scaled bitmap (at most 2560 x 2560), after the megapixel refusal that is quarter-turn invariant, under the existing resizeSlots throttle; no new decode of the full-size picture |
| T-rrh-04 | Tampering | writing a malformed or foreign-colour-model profile into the served copy | medium | mitigate | validRGBProfile (size equals length, acsp signature, RGB space, at most 4 MiB) before anything is written; JPEG chunk sequence must be exactly 1..n; withProfile returns the bytes untouched when it cannot insert; PNG output is decoded by image/png in the test to prove CRC and chunk order |
| T-rrh-05 | Information disclosure | camera data (GPS, device, time) leaking through the new code | high | mitigate | Only the profile and the orientation value are taken from the source; the encoders write no EXIF; Keep still runs StripMetadata; tests assert the marker string and every APP1 are absent from the output and that orientationOf(output) is 1 |
| T-rrh-06 | Information disclosure | text inside a carried ICC profile (description, copyright) | low | accept | Already published today for every unscaled JPEG and PNG (strip.go keeps the profile on purpose); carrying it into the scaled copy changes nothing about what is exposed |
| T-rrh-07 | Tampering | trusting the manifest's size at POST | low | accept | The size in the manifest only feeds the text on the question page; the POST re-reads the bytes, runs media.Check and recomputes everything in Scaled |
| T-rrh-SC | Tampering | npm/pip/cargo installs | high | mitigate | No package-manager install in this plan; the dependency guard asserts go.mod and go.sum are unchanged (last touched 4465375); any new module would need the package-legitimacy checkpoint and is out of scope |
</threat_model>

<verification>
Run once all three tasks are committed. Record each baseline; a gate that already holds at HEAD is an invariant guard, not proof of the work.

1. Unit and handler tests: `go test ./internal/media -count=1` (baseline: green before; the new tests fail before the code) and `go test ./internal/admin -run 'TestScaleQuestionUsesTheUprightPicture|TestALargeUploadIsHeldBackAndAsked|TestYesStoresAScaledCopyWithoutCameraData|TestNoStoresTheOriginal|TestOneDecisionCoversAWholeUpload|TestTheAIUploadNeverAsks' -count=1`.
2. Whole tree: `go build ./...`, `go vet ./...`, `go test ./... -timeout 30m` (exit 0), `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dev/null ./cmd/holzcloud`.
3. `test -z "$(gofmt -l internal cmd tools)"`.
4. Catalogue gates: `go run ./tools/i18n` ends in 0 open, 0 orphaned (baseline from 261026-bon: green); `go run ./tools/themewords -check`; `go run ./tools/english`.
5. Dependency guard: `test "$(git log -1 --format=%h -- go.mod go.sum)" = "4465375"` (baseline: true now; must stay true).
6. Byte identity: TestPlainPictureBytesDoNotChange passed against the unmodified scale.go in Task 1 before the change was made (executor records that run in the SUMMARY) and passes now.
7. The 28-check with the conflict stated in the objective. After the commits, run: `GO=0 NATIVE_SELFTEST=0 bash .planning/phases/28-the-promise-in-writing/28-check.sh all 2>&1 | grep '^FAIL' | sort`. Required output, exactly these two lines and nothing else:
   FAIL: a page of docs other than beta.md changed after 85fa486
   FAIL: something outside .planning, docs and CHANGELOG.md changed after 85fa486
   (baseline before this task: no FAIL line, exit 0.) Anything else in that list - a deploy-binding diff, beta, owner-steps, changelog, regression - means the work disturbed something it must not touch: stop and fix. Also run once without GO=0 so `changelog go` (go test of the changelog screens) runs: no extra FAIL line.
8. `git status --short` shows nothing for docs/beta.md, CHANGELOG.md, deploy/, deploy/test/nightly.sh, .planning/phases/28-the-promise-in-writing/28-check.sh; no tag created, nothing pushed.
</verification>

<success_criteria>
- Yes on a portrait phone photo (JPEG, PNG or WebP with orientation 2-8) stores an upright copy at the size the question announced, with no orientation tag.
- Any picture without a readable orientation or a carriable profile is scaled to exactly the bytes it was before this change.
- An RGB profile on the source is on the stored copy; no other metadata is.
- The docs say what is carried, what is not, and that unscaled uploads still lose the orientation tag.
- Every gate in the verification holds, with the one named exception of the phase-28 `frozen` sub-check, which is reported, not hidden.
</success_criteria>

<output>
Create `.planning/quick/261026-rrh-exif-ausrichtung-und-icc-profil-beim-ver/261026-rrh-SUMMARY.md` when done, with a Deviations section that names the red `frozen` sub-check and the owner decision it needs.
</output>
