---
phase: quick-260928-klh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/media/variants.go
  - internal/media/variants_test.go
  - internal/media/model.go
  - internal/media/store.go
  - internal/media/variant_store.go
  - internal/media/backfill.go
  - internal/media/testdata/opaque-lossy.webp
  - internal/media/testdata/opaque-lossless.webp
  - internal/media/testdata/transparent.webp
  - internal/admin/media.go
  - internal/admin/media_crop.go
  - internal/admin/media_crop_test.go
  - internal/admin/media_screen_test.go
  - internal/public/album_test.go
  - cmd/holzcloud/cli.go
  - cmd/holzcloud/cli_thumbnails_test.go
  - docs/deployment.md
  - CHANGELOG.md
autonomous: true
requirements: [QUICK-260928-klh]

estimate:
  tokens: 130000
  raw_tokens: 130000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "An opaque WebP, lossy or lossless, gets JPEG scaled copies; for the 1000px lossy fixture the 800px medium copy exists as <stem>-medium.jpg and is smaller than the original"
    - "An opaque PNG gets JPEG scaled copies; a PNG or WebP with at least one pixel that is not fully opaque keeps PNG copies, and the transparency survives in them"
    - "The format is decided from the decoded pixels (hasTransparency), never from the MIME type; JPEG sources keep producing JPEG copies exactly as before"
    - "The thumbnail rule is unchanged: a thumbnail is written whenever the original is wider than 400px, even when it is not smaller than the original; medium and large are still dropped when not smaller"
    - "The admin grid and the media picker load the thumbnail by the filename stored in media_variants (Media.ThumbFilename via the store projection), and fall back to the original when no thumb row exists — the URL always names a file that is on disk, for old -thumb.png and new -thumb.jpg installs alike"
    - "SaveVariants replaces the whole set of a picture: afterwards the rows are exactly the new variants, and files named by old rows that the new set no longer names are deleted from the website's media directory after the commit — never a path outside that directory, never the original"
    - "`holzcloud thumbnails -force` turns an install's old -thumb.png / -medium.png copies of an opaque image into .jpg copies, leaving no row that points at a missing file and no variant file that no row names"
    - "The public responsive pass (LoadImageSets + MakeResponsive) offers `<stem>-medium.jpg 800w` in the srcset of an opaque WebP"
    - "CHANGELOG.md has `## 0.0.6 — 2026-09-28` above 0.0.5, in the file's German/Swiss style, telling operators to run `holzcloud thumbnails -force` once"
  artifacts:
    - path: "internal/media/variants.go"
      provides: "hasTransparency(image.Image) bool; MakeVariants picks JPEG vs PNG from the decoded pixels"
      contains: "func hasTransparency"
    - path: "internal/media/model.go"
      provides: "Media.ThumbFilename; ThumbURL/HasThumb read it"
      contains: "ThumbFilename"
    - path: "internal/media/store.go"
      provides: "projection(table) = prefixed columns + correlated thumb-name subquery, used by every scan"
      contains: "label = 'thumb'"
    - path: "internal/media/variant_store.go"
      provides: "SaveVariants(ctx, dir, mediaID, width, height, variants) replacing the whole set and removing stale files"
      contains: "func (s *Store) SaveVariants(ctx context.Context, dir string"
    - path: "internal/media/testdata/opaque-lossy.webp"
      provides: "1000x560 photo-like lossy WebP fixture (~82 KB)"
    - path: "internal/media/testdata/opaque-lossless.webp"
      provides: "1000x560 screenshot-like lossless WebP fixture, decodes to an opaque *image.NRGBA"
    - path: "internal/media/testdata/transparent.webp"
      provides: "1000x560 lossless WebP with a transparent background"
    - path: "cmd/holzcloud/cli_thumbnails_test.go"
      provides: "End-to-end test of `thumbnails -force` swapping .png copies for .jpg with no dangling rows or files"
    - path: "CHANGELOG.md"
      provides: "0.0.6 entry"
      contains: "## 0.0.6 — 2026-09-28"
  key_links:
    - from: "internal/media/variants.go MakeVariants"
      to: "variantFilename + encodeVariant"
      via: "transparent := hasTransparency(src) replaces the MIME comparison"
      pattern: "hasTransparency\\(src\\)"
    - from: "internal/media/store.go projection"
      to: "cmd/holzcloud/templates/admin/media_list.html and media_picker.html ({{.ThumbURL}})"
      via: "scan fills Media.ThumbFilename from media_variants; ThumbURL builds /media/<site>/<ThumbFilename>"
      pattern: "ThumbFilename"
    - from: "cmd/holzcloud/cli.go cmdThumbnails, internal/media/backfill.go, internal/admin/media.go makeVariants, internal/admin/media_crop.go rebuildVariants"
      to: "Store.SaveVariants"
      via: "every caller passes the website media directory so stale files are removed"
      pattern: "SaveVariants\\(ctx, dir"
---

<objective>
Opaque WebP and PNG uploads (screenshots, photos saved as WebP) get JPEG size variants instead of PNG ones, so the public srcset offers the 800px medium copy instead of only the 400px thumbnail and the full original. Transparency is decided from the decoded pixels, not the MIME type (locked decision: stdlib + the already-present golang.org/x/image only, no WebP encoder). The admin grid stops deriving the thumbnail name from the MIME type and reads the stored name. `holzcloud thumbnails -force` replaces an install's old PNG copies cleanly, so it can be run on the live install afterwards.

Purpose: on holzcloud.ch a 2400px WebP currently serves `srcset="/media/1/<hash>-thumb.png 400w, /media/1/<hash>.webp 2400w"` because its PNG medium (159 KB) and large (452 KB) copies are bigger than the 125 KB original and get dropped. JPEG q82 copies measure 39–53 KB at 800px.

Output: pixel-based format choice in internal/media/variants.go, stored-name thumbnails, whole-set replacement in SaveVariants, three WebP fixtures, tests at media/admin/CLI level, CHANGELOG 0.0.6, SUMMARY with the exact live command.

Audit already done by the planner (the executor does not need to redo it, only keep it true):
- golang.org/x/image v0.45.0 is already in go.mod and internal/media/variants.go already registers the WebP decoder with a blank import of golang.org/x/image/webp. No dependency change.
- Variant file names are derived in exactly two places: MakeVariants (writes them; stays) and Media.ThumbURL in internal/media/model.go (derives the thumb name from the MIME type; MUST change — after this change an opaque PNG/WebP has -thumb.jpg while the MIME says png/webp).
- Everything else reads stored names: ImageSet.SrcSet and LoadImageSets use media_variants.filename; ResolveServed takes the served MIME from the stored variant's extension (.png → image/png, else image/jpeg — correct for .jpg); internal/block/render.go writes no srcset; bundle export/import only carries originals (import relies on Backfill); internal/export crawls rendered HTML; internal/ai builds its own maps without thumb URLs; the only template users are cmd/holzcloud/templates/admin/media_list.html and media_picker.html via {{.ThumbURL}}.
- Public page HTML computes the srcset per request (internal/public/pagedata.go responsive) and serveCached uses a content-hash ETag, so a changed srcset is a new ETag.
- SaveVariants today upserts per (media_id, label) and never deletes: a regenerated set leaves the old -thumb.png file on disk, and a label that the new run drops (medium/large not smaller, or a crop that made the picture narrower) keeps its old row. Both are fixed by Task 2.
- internal/media/crop.go ApplyCrop has its own MIME-based transparent flag for re-encoding the SERVED file under its own extension; it is not a variant and MUST stay as it is.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@.planning/STATE.md
@internal/media/variants.go
@internal/media/variant_store.go
@internal/media/model.go
@internal/media/store.go
@internal/media/backfill.go
@internal/media/variants_test.go
@cmd/holzcloud/cli.go
@CHANGELOG.md

Environment: Go is at ~/.local/go/bin (every command below prefixes PATH). The executor runs in the checkout on branch feat/webp-variants without worktree isolation. python3 with Pillow 11.1.0 and WebP support is installed (used once to generate the fixtures; nothing Python is committed). internal/admin tests are slow on this loaded Pi: always give them -timeout 40m.

Test helpers that exist: newTestStore(t) in internal/media/usage_test.go (returns *Store and the website id, which is 1); writeTestImage in internal/media/variants_test.go; mediaAdmin, mediaUploadRequest in internal/admin/media_screen_test.go; serve and newTestAdmin in internal/admin/page_handler_test.go; TestTheMediaListFiltersAndCounts shows how to GET HandleMediaList; config.Load reads HOLZCLOUD_DATA_DIR and puts the database at <dir>/holzcloud.sqlite, and internal/config/config_test.go already calls config.Load under test.
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): an opaque WebP gets JPEG copies, the public srcset offers its medium, the grid reads the stored thumb name</name>
  <files>internal/media/variants.go, internal/media/model.go, internal/media/store.go, internal/media/variants_test.go, internal/media/testdata/opaque-lossy.webp, internal/media/testdata/opaque-lossless.webp, internal/media/testdata/transparent.webp</files>
  <behavior>
    - hasTransparency: opaque *image.RGBA → false; *image.NRGBA with every alpha 255 (what x/image/webp returns for a lossless WebP) → false; *image.NRGBA with one alpha 0 → true; *image.YCbCr → false; *image.NYCbCrA with one A below 255 → true; *image.Paletted using a palette entry with alpha 0 → true, same palette with that entry unused → false; a test type implementing only ColorModel/Bounds/At (no Opaque method) takes the fallback scan and answers correctly both ways
    - MakeVariants on testdata/opaque-lossy.webp (copied into a temp dir as shot.webp, mime image/webp): thumb is shot-thumb.jpg, medium is shot-medium.jpg, no .png variant, medium SizeBytes smaller than the original, image.DecodeConfig on the medium file reports format "jpeg"
    - MakeVariants on testdata/opaque-lossless.webp: the thumb is a .jpg (this is the *image.NRGBA-but-opaque path)
    - MakeVariants on testdata/transparent.webp: every variant is .png and the decoded thumb has at least one pixel whose alpha is not 0xffff
    - Opaque PNG (image/png, all alpha 255) → .jpg variants; PNG with a genuinely transparent band → .png variants (TestMakeVariantsKeepsTransparency keeps passing only because the fixture now really is transparent)
    - End to end: newTestStore, Create a row shot.webp / image/webp, MakeVariants, SaveVariants, then LoadImageSets + MakeResponsive on a body containing an img tag for /media/1/shot.webp → output contains "shot-thumb.jpg 400w" and "shot-medium.jpg 800w"
    - Store round trip: after SaveVariants with a thumb row named shot-thumb.png, GetByID, GetByFilename, FindByHash and List all return a Media whose ThumbURL is /media/1/shot-thumb.png; after SaveVariants with a thumb row named shot-thumb.jpg it is /media/1/shot-thumb.jpg; a row with Width 2000 but no thumb row answers the original URL; after SaveCrop bumps the version the thumb URL carries ?v=1
    - Unit: Media with ThumbFilename abc-thumb.jpg → /media/1/abc-thumb.jpg; wide Media without ThumbFilename → its own URL; SVG → its own URL
  </behavior>
  <action>
Fixtures first. Generate three files into internal/media/testdata/ with a one-off python3 Pillow command (do not commit any script; record the exact command in the SUMMARY). Parameters, measured by the planner with the real Go pipeline:
(a) opaque-lossy.webp — mode RGB, 1000x560, random.seed(1), per pixel r = int(128 + 100*sin(x/97)*cos(y/53)), g = int(110 + 90*sin((x+y)/140)), b = int(90 + 60*cos(x/211)), then add one random.randint(-10, 10) to all three channels and clamp to 0..255; save as WEBP quality=80 method=6. Expect about 82 KB; Go's JPEG q82 at 800px is about 44 KB (kept), Go's PNG at 800px about 528 KB (what the old rule dropped). If the executor's numbers differ so much that the medium JPEG is not at least 20 percent smaller than the file, raise the noise amplitude until it is, and note it.
(b) opaque-lossless.webp — mode RGB, 1000x560, background (245,245,245), a (30,60,120) bar over the top 48 px, and every 28 px a dark grey horizontal bar 12 px high starting at x=40 whose width is 200 + (row_index*137 mod 700); save WEBP lossless=True. Expect a few hundred bytes; it decodes to *image.NRGBA with Opaque() true.
(c) transparent.webp — mode RGBA, 1000x560, fully transparent background, an opaque (200,40,40) ellipse inside [100,60,900,500]; save WEBP lossless=True.
Add a short comment above the test helper that loads them saying how they were made (in prose, parameters named) and why the lossy one exists (the medium PNG copy of it is larger than the original, the JPEG copy is not).

internal/media/variants.go (implements the locked decision: decide from decoded pixels, stdlib only):
- Add hasTransparency(img image.Image) bool. If img has an Opaque() bool method (every image type the stdlib and x/image decoders return has one: YCbCr, NYCbCrA, RGBA, NRGBA, Gray, Paletted, 16-bit variants, CMYK), return its negation — Opaque stops at the first non-opaque pixel, so this is cheap. Otherwise scan Bounds() with At(x,y).RGBA() and return true at the first alpha below 0xffff.
- In MakeVariants replace the MIME comparison with transparent := hasTransparency(src), computed once right after image.Decode. Everything downstream (variantFilename extension, encodeVariant PNG vs JPEG q82) stays as it is.
- Keep the mimeType parameter (all callers pass it) and give it its remaining job: at the top of MakeVariants return an error when CanMakeVariants(mimeType) is false, so an animated GIF can never be flattened by a caller that forgot to ask.
- Rewrite the MakeVariants doc comment: JPEG unless the decoded picture actually has a pixel that is not fully opaque; say why the MIME type stopped deciding (a WebP or PNG screenshot is opaque, its PNG copies were larger than the original — 2400px WebP 125 KB, 800px PNG 159 KB, 1600px PNG 452 KB — so the size rule dropped them and the srcset offered only the thumbnail and the original). Keep the sentence that a transparent picture stays PNG because JPEG turns transparent parts black.
- Keep the thumbnail exemption from the "not smaller than the original, drop it" rule exactly as it is. Rewrite only its comment: the grid and the picker show the thumbnail for every card, so it must exist whenever the original is wider than it; they now read its stored name from media_variants (no longer derive it), which is what keeps -thumb.png installs and new -thumb.jpg ones both working.
- No file:line citations in any comment (tools/cites -check refuses the form).

internal/media/model.go:
- Add field ThumbFilename string to Media, with a comment: the stored name of the 400px copy from media_variants, empty when there is none; filled by every store read.
- ThumbURL: if ThumbFilename is empty return m.URL(); otherwise "/media/" + website id + "/" + ThumbFilename + m.versionQuery(). Rewrite its comment: the name is looked up, not derived, because the format now depends on the pixels; the lookup rides in the same query as the row, so forty cards still cost one query; a picture without a thumb row (SVG, small logo, never measured, or one whose variants failed while backfill still recorded its size — the old derivation pointed that last case at a file that did not exist) falls back to the original.
- HasThumb returns ThumbFilename != "".

internal/media/store.go:
- Keep the columns constant. Add projection(table string) string that returns prefixed(table) followed by the correlated subquery COALESCE((SELECT v.filename FROM media_variants v WHERE v.media_id = <table>.id AND v.label = 'thumb'), ''). The table qualifier is mandatory: unqualified id inside the subquery would bind to media_variants.id. UNIQUE (media_id, label) from migration 00013 makes it a point lookup.
- FindByHash, GetByID, GetByFilename select projection("media") from media; List selects projection("m") from media m. scan reads one more value into m.ThumbFilename. Update the columns comment so it names projection as the single source of what scan reads.

internal/media/variants_test.go:
- writeTestImage: when transparent is true it must write a PNG with real transparency (use image.NRGBA and set alpha 0 over, say, the top quarter); add a small helper for an opaque PNG. TestMakeVariantsRefusesOversizedImage keeps working with either.
- Add the tests listed in behavior (hasTransparency table, lossy/lossless/transparent WebP, opaque PNG, the end-to-end srcset test, the store round trip), and rewrite TestThumbURLFallsBackToOriginal to set ThumbFilename. Update the comment in TestMakeVariantsProducesSmallerCopies that says the admin grid addresses the thumbnail by name.
- Write the failing tests first (RED), then the implementation (GREEN).
  </action>
  <verify>
    <automated>export PATH=$HOME/.local/go/bin:$PATH; go build ./... && go vet ./internal/media/ && go test ./internal/media/ -count=1 && go test ./internal/media/ -count=1 -run 'WebP|Transparen|ThumbURL|Opaque' -v 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)' | tail -40</automated>
  </verify>
  <done>go test ./internal/media passes; an opaque WebP (lossy and lossless) and an opaque PNG get .jpg copies, transparent PNG/WebP keep .png copies with alpha intact; MakeResponsive offers shot-medium.jpg 800w for the lossy WebP fixture; Media.ThumbURL comes from the stored thumb row through every store read; the three fixtures are committed; the whole repo still builds.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: regeneration replaces the whole set — `thumbnails -force` swaps .png copies for .jpg with nothing dangling</name>
  <files>internal/media/variant_store.go, internal/media/backfill.go, internal/media/variants_test.go, internal/admin/media.go, internal/admin/media_crop.go, internal/admin/media_crop_test.go, internal/public/album_test.go, cmd/holzcloud/cli.go, cmd/holzcloud/cli_thumbnails_test.go</files>
  <behavior>
    - Swap (media package): shot.webp from the lossy fixture with a pre-change state seeded through SaveVariants — rows thumb → shot-thumb.png and large → shot-large.png, both files on disk. After MakeVariants + SaveVariants(ctx, dir, ...): rows are exactly thumb → shot-thumb.jpg and medium → shot-medium.jpg; shot-thumb.png and shot-large.png are gone from disk; the original is still there; the directory holds exactly the original plus the files the rows name. A second identical run leaves the same rows and files (same-name files are overwritten, never deleted).
    - Safety: a stale row whose filename is "../escape.png" does not delete the file one directory up; a stale row that names the original's own filename does not delete the original.
    - Existing SaveVariants tests keep their meaning with the new signature (TestSaveAndLoadImageSets still ends with 2 rows after a re-run, TestDeleteRemovesVariantFiles still removes the copy).
    - CLI end to end (cmd/holzcloud/cli_thumbnails_test.go, test name starting TestThumbnails so the verify filter finds it): t.Setenv HOLZCLOUD_DATA_DIR to a temp dir, open that dir's holzcloud.sqlite with db.Open + db.RunMigrations, create a website through domain.NewStore(...).CreateWebsite, write foto.png (opaque, noisy enough that its medium JPEG is kept — seeded math/rand) and logo.png (transparent band), seed the pre-change state for both (rows and placeholder files foto-thumb.png, foto-medium.png, logo-thumb.png, width/height recorded), then cmdThumbnails([]string{"-force"}) returns nil. Afterwards: foto has a thumb row foto-thumb.jpg and no .png row; foto-thumb.png and foto-medium.png are gone; logo keeps logo-thumb.png on disk and in its row; every row names a file that exists; every file in the website directory is an original or named by a row. Running -force a second time keeps all of that true.
  </behavior>
  <action>
internal/media/variant_store.go — SaveVariants becomes a replacement of the whole set, with the directory as a required argument so no caller can forget the files: new signature SaveVariants(ctx context.Context, dir string, mediaID int64, width, height int, variants []Variant) error, where dir is the website's media directory (media.WebsiteDir(dataDir, websiteID)). Inside the one existing transaction: read the media row's own filename and the filenames of all its current media_variants rows; DELETE all media_variants rows of mediaID; INSERT the new ones (plain INSERT, the delete removed any conflict); UPDATE width/height as now; commit. Only after a successful commit remove the stale files: every old filename that is not among the new filenames, skipping any name that is empty, not equal to filepath.Base of itself (so nothing outside dir), equal to the original's filename, or equal to SourceName(original); ignore a file that is already gone. Order matters and goes in the comment: rows first, files second, so a crash in between leaves an orphaned file (a leak), never a row pointing at nothing (a 404 in a srcset). Replace the old "a re-run replaces what is there" comment with this reasoning, and mention that it also clears copies a crop made obsolete (a picture cropped narrower than 800px used to keep its old medium row and file). Keep ResolveServed, LoadImageSets and removeVariantFiles unchanged.

Callers — pass the directory they already have:
- internal/admin/media.go makeVariants: destDir.
- internal/admin/media_crop.go rebuildVariants: dir.
- internal/media/backfill.go Backfill: dir.
- cmd/holzcloud/cli.go cmdThumbnails: dir. No other change to the loop; its existing behaviour (skip and continue on a MakeVariants or Dimensions error, so a failed file keeps its old, still valid set) is exactly right. Update the flag help of -force to say it also replaces copies made in an older format, and extend the command's doc comment accordingly.
- Tests: internal/media/variants_test.go (four calls: temp dir or the website dir already built in the test), internal/admin/media_crop_test.go seedImage (its dir variable), internal/public/album_test.go seedPicture (t.TempDir()).

Tests: add the media-package swap and safety tests to internal/media/variants_test.go, and create cmd/holzcloud/cli_thumbnails_test.go with the CLI test from behavior (package main, imports only what exists: internal/db, internal/domain, internal/media, stdlib image encoders). Write a small helper in each test that asserts the two invariants (every row's file exists; every variant-looking file in the directory is named by a row). Tests first, then the implementation.
  </action>
  <verify>
    <automated>export PATH=$HOME/.local/go/bin:$PATH; go build ./... && go vet ./... && go test ./internal/media/ ./internal/public/ ./cmd/holzcloud/ -count=1 && go test ./cmd/holzcloud/ -count=1 -run 'Thumbnail' -v 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' && go test ./internal/admin/ -count=1 -timeout 40m -run 'Zuschnitt|Crop|Upload|Media'</automated>
  </verify>
  <done>SaveVariants(ctx, dir, ...) replaces rows and removes only stale files inside dir after commit; all four production callers and three test callers pass the directory; `thumbnails -force` swaps an opaque PNG's .png copies for .jpg and keeps a transparent PNG's .png copies, with both invariants holding after one and after two runs; media, public, cmd/holzcloud and the admin crop/upload/media tests pass.</done>
</task>

<task type="auto">
  <name>Task 3: the admin grid shows a thumbnail that exists, operator notes, full gate</name>
  <files>internal/admin/media_screen_test.go, CHANGELOG.md, cmd/holzcloud/cli.go, docs/deployment.md</files>
  <action>
internal/admin/media_screen_test.go — add one test (for example TestTheGridShowsTheThumbnailThatExists): mediaAdmin; encode in the test an opaque 640x360 PNG (image.RGBA gradient) and a 640x360 PNG with a fully transparent band (image.NRGBA); upload both through HandleMediaUpload with mediaUploadRequest (expect 303 each); read the rows with media.NewStore(database).List; assert the opaque upload's ThumbFilename ends in -thumb.jpg and the transparent one's in -thumb.png; for each, assert the file exists under media.WebsiteDir(h.cfg.DataDir, ws.ID) and that the HTML of HandleMediaList (GET as in TestTheMediaListFiltersAndCounts, without a filter) contains the src value built from ThumbURL. This is the proof that the grid addresses the thumbnail it really has. Add the image, image/color and image/png imports the file needs.

CHANGELOG.md — insert a new section directly above "## 0.0.5 — 2026-09-28", headed exactly "## 0.0.6 — 2026-09-28", in the file's style: German with Swiss spelling (ss, never the sharp s), whole sentences, each paragraph led by one bold sentence, the why included. Three short paragraphs:
(1) Screenshots and pictures stored as WebP or PNG get small copies again: until now every WebP and PNG got PNG copies; for a screenshot or a photo those are larger than the original (2400px WebP 125 KB, 800px PNG copy 159 KB, 1600px 452 KB), so the rule that drops a copy which saves nothing dropped them, and a phone downloaded the full original. Now the picture itself decides: without a single transparent pixel the copies are JPEG (the 800px copy of that WebP is about 40 to 50 KB); a picture with real transparency keeps PNG copies as before.
(2) The media library shows the thumbnail that exists, by its stored name, rather than guessing the name from the file type.
(3) After the update run `holzcloud thumbnails -force` once (in a container: kubectl exec into the pod, then `/holzcloud thumbnails -force`). It replaces the old PNG copies of opaque pictures with JPEG copies and deletes the old files, and also clears copies a crop had made obsolete. Without it existing pictures keep their old copies, which still work and are only heavier; new uploads get the new copies at once. A page a browser cached in the last five minutes may briefly ask for a deleted old copy; reloading fixes it.

cmd/holzcloud/cli.go usage text and docs/deployment.md command list — change the thumbnails line in both, identically and keeping the column alignment, to name the flag: "holzcloud thumbnails [-force]" followed by a description saying it generates the scaled copies of older images and, with -force, rebuilds them for every image.

Then run the full gate from the verify line and fix anything it finds. In the SUMMARY, record: the fixture generation command and measured sizes (original, medium JPEG, what a PNG medium would have been); the exact live command, kubectl exec -n NAMESPACE deploy/DEPLOYMENT -- /holzcloud thumbnails -force (only after the new image is rolled out — the old binary would write PNG copies again), its expected output line "N images processed, M failed", and a check afterwards (the page HTML's srcset now lists a -medium.jpg 800w candidate for the WebP from the problem statement); the five-minute HTML cache window; and, as an observation left unchanged, that ApplyCrop re-encodes a cropped WebP as PNG bytes under the .webp name (pre-existing, out of scope).
  </action>
  <verify>
    <automated>export PATH=$HOME/.local/go/bin:$PATH; test -z "$(gofmt -l .)" && go mod tidy && git diff --exit-code -- go.mod go.sum && go vet ./... && go run ./tools/english && go run ./tools/cites -check && go run ./tools/i18n && head -40 CHANGELOG.md | grep -q '^## 0.0.6 — 2026-09-28' && grep -q 'thumbnails \[-force\]' cmd/holzcloud/cli.go && grep -q 'thumbnails \[-force\]' docs/deployment.md && go test $(go list ./... | grep -v '/internal/admin$') -count=1 && go test ./internal/admin/ -count=1 -timeout 40m</automated>
  </verify>
  <done>The grid test proves the admin list's img src names an existing -thumb.jpg for an opaque PNG and an existing -thumb.png for a transparent one; CHANGELOG 0.0.6 sits above 0.0.5; the usage line and docs name -force; gofmt, go vet, tools/english, tools/cites -check, tools/i18n (0 open, 0 orphaned) and go test ./... (admin with -timeout 40m) are all green; go.mod/go.sum unchanged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| upload → decoder | Operator- or AI-uploaded image bytes are decoded to decide the format; the decode is already bounded by the megapixel budget and the two-slot throttle |
| database row → filesystem | SaveVariants now deletes files whose names come from media_variants rows |
| CLI in the live pod → running server | `thumbnails -force` rewrites rows and files while the server serves them |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-q260928klh-01 | Tampering | SaveVariants stale-file removal | high | mitigate | Only names equal to their own filepath.Base are removed, only inside the passed website directory, never the original or its crop source; covered by the "../escape.png" and original-name tests in Task 2 |
| T-q260928klh-02 | Denial of Service | hasTransparency on large decodes | low | accept | Runs after the existing DecodeConfig megapixel check (24 MP default) and inside the existing throttle; Opaque() stops at the first non-opaque pixel and is linear in bytes already allocated by the decode |
| T-q260928klh-03 | Information Disclosure | thumb-name subquery in the media projection | low | mitigate | Correlated on the row's own id, so it can only return that picture's own thumbnail; ResolveServed's website join is unchanged, so a thumbnail stays unreachable through another website's route |
| T-q260928klh-04 | Tampering (integrity of references) | row/file ordering during regeneration | medium | mitigate | Rows are committed before stale files are removed, so a crash leaves an orphaned file, never a row pointing at a missing file; invariants asserted after one and two -force runs in Task 2 |
| T-q260928klh-05 | Denial of Service (broken image) | browser-cached HTML after -force | low | accept | Public HTML has max-age=300 and a content ETag; for at most five minutes a cached page may request a deleted old copy. Documented in CHANGELOG and SUMMARY |
| T-q260928klh-06 | Tampering (integrity of references) | MakeVariants partial-failure cleanup during regeneration | low | accept | Pre-existing: if encoding fails mid-run (disk full), MakeVariants removes the copies it wrote in this run, which may share a name with a still-stored row. Needs a write failure on the card; out of scope, noted in SUMMARY |
| T-q260928klh-SC | Tampering | Go module installs | low | accept | No new dependency: image/png, image/jpeg and the already-required golang.org/x/image v0.45.0. The go mod tidy + git diff check in Task 3 proves go.mod/go.sum unchanged |
</threat_model>

<verification>
- go test ./internal/media: pixel-based format choice for lossy/lossless/transparent WebP and opaque/transparent PNG; medium copy of the lossy WebP kept and smaller; srcset offers shot-medium.jpg 800w; ThumbURL from stored rows; whole-set replacement with safety cases.
- go test ./cmd/holzcloud -run Thumbnail: `thumbnails -force` swaps .png for .jpg with no dangling rows or files, idempotent.
- go test ./internal/admin -timeout 40m: the grid's img src names an existing thumbnail for both formats; crop and upload tests still pass with the new SaveVariants signature.
- Gate: gofmt -l empty, go vet ./..., go run ./tools/english, go run ./tools/cites -check, go run ./tools/i18n (0 open, 0 orphaned), go test ./..., go.mod/go.sum unchanged.
</verification>

<success_criteria>
- An opaque WebP or PNG upload produces JPEG thumb/medium(/large) copies; a transparent one produces PNG copies; JPEG uploads behave as before.
- The public srcset of an opaque WebP includes its 800px medium copy.
- No admin grid, picker or srcset URL points at a file that does not exist, before or after `holzcloud thumbnails -force`.
- After `-force`, no media_variants row names a missing file and no stale variant file remains.
- CHANGELOG 0.0.6 written; SUMMARY documents the exact live command and the post-run check.
</success_criteria>

<output>
Create `.planning/quick/260928-klh-opaque-webp-uploads-get-jpeg-size-varian/260928-klh-SUMMARY.md` when done
</output>
