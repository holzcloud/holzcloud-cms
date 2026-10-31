---
phase: quick-261026-bon
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/block/block.go
  - internal/block/render.go
  - internal/block/imageposition_test.go
  - internal/bundle/blocks_position_test.go
  - internal/ai/tools_pages.go
  - internal/tmplspec/TEMPLATE-SPEC.md
  - cmd/holzcloud/templates/admin/block_list.html
  - cmd/holzcloud/assets/bausteine.css
  - cmd/holzcloud/templates/public/holzcloud/style.css
  - cmd/holzcloud/templates/public/rudel/style.css
  - cmd/holzcloud/templates/public/weide/style.css
  - internal/media/scale.go
  - internal/media/scale_test.go
  - internal/media/staging.go
  - internal/media/staging_test.go
  - internal/admin/handler.go
  - internal/admin/media.go
  - internal/admin/media_scale.go
  - internal/admin/media_scale_test.go
  - internal/admin/page_blocks_test.go
  - cmd/holzcloud/templates/admin/media_scale.html
  - cmd/holzcloud/main.go
  - cmd/holzcloud/main_test.go
  - internal/i18n/locales/de.json
  - internal/i18n/locales/de-CH.json
  - internal/i18n/locales/fr.json
  - internal/i18n/locales/it.json
  - internal/i18n/locales/es.json
  - docs/content-model.md
  - docs/publishing.md
  - docs/configuration.md
autonomous: true
requirements: [QUICK-261026-bon]

estimate:
  tokens: 150000
  raw_tokens: 150000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "An existing Image-and-text block (stored Variant empty or rechts) renders byte-identical markup to before this change and needs no migration"
    - "The editor offers Image position: left, right, above, below; the choice is saved, survives re-editing, and reaches the visitor as a class modifier (hc-bildtext--rechts, --oben, --unten)"
    - "All 8 shipped themes lay the block out stacked for above/below at desktop width; the three themes with their own bildtext layout (holzcloud, rudel, weide) do not stretch a stacked block into the wide two-column slot"
    - "Uploading a JPEG/PNG/WebP/single-frame-GIF whose longest side is over 2560 px stores nothing yet and shows a plain-form page: scale down to fit within 2560 px, Yes or No, working without JavaScript"
    - "Yes stores the image with its longest side exactly 2560 px, same aspect ratio, no camera metadata; No stores the original unchanged"
    - "SVG, PDF, MP4, animated GIFs, images at or under 2560 px and images over the megapixel budget are stored at once, exactly as before"
    - "A multi-file upload puts every oversize file on ONE page with ONE decision; the other files of that upload are stored at once"
    - "A staged upload can be reached only through its own website and its own user; resuming it runs media.Check again; abandoned ones are pruned"
    - "The AI-key upload path (OpUploadMedia) never asks and stores a large image at full size, as before"
  artifacts:
    - path: internal/block/imageposition_test.go
      provides: "Regression tests for the four image positions (render, Clean, form, CSS contract)"
    - path: internal/media/scale.go
      provides: "MaxSide constant, ScaledSize, Intake.Oversize, Intake.Scaled"
    - path: internal/media/staging.go
      provides: "Disk staging scoped per website and user, with Take and Prune"
    - path: internal/admin/media_scale.go
      provides: "The question page (GET) and the decision (POST)"
    - path: cmd/holzcloud/templates/admin/media_scale.html
      provides: "Plain-form Yes/No page"
    - path: internal/admin/media_scale_test.go
      provides: "Handler tests: ask, yes, no, mixed batch, isolation, expiry, poisoned staging, AI path"
  key_links:
    - from: cmd/holzcloud/templates/admin/block_list.html
      to: "internal/block/render.go via Block.Variant (select name b.variante)"
      via: "form.go setBlockField, block.Clean, Block.ImagePositionClass"
    - from: internal/admin/media.go
      to: internal/media/staging.go
      via: "HandleMediaUpload stages each Intake that Oversize reports, redirects to GET /admin/websites/{id}/media/scale"
    - from: internal/admin/media_scale.go
      to: "keepUpload's tail (checkUpload then keepChecked)"
      via: "Take, media.Check again, optional Intake.Scaled, keepChecked"
    - from: cmd/holzcloud/main.go
      to: "internal/media/staging.go Prune"
      via: "media-staging-prune job"
---

<objective>
Two features the owner has decided; nothing here is open for renegotiation.

(1) The block "Image and text" (bildtext) gets the position of its image: left, right, above, below. It is an option of the existing block, not a new block type. The stored value is the block's existing Variant (editor field name `variante`), so empty means left and every stored block renders exactly as before; no migration.

(2) After a media upload, an image whose longest side is over 2560 px is held back and the operator is asked on a plain-form page, without JavaScript, whether to scale it down to fit within 2560 px. Yes stores the scaled image, No stores the original. The upload is staged on disk between the two requests, scoped per website and user.

Purpose: stop heavy originals from reaching the public pages by accident, and let a start page put a picture over or under its text.
Output: block renderer + editor + CSS for 8 themes; media scale + staging packages; admin question page, handlers, routes, prune job; tests; catalogue entries in 4 languages plus the regenerated Swiss one; docs.

Shape: two independent vertical features, each a complete path (editor to visitor; upload to library), so no horizontal layer task exists. Task 1 is the first feature end to end, Task 2 the second, Task 3 the docs, the whole-tree gates and the browser check. Every task leaves `go run ./tools/i18n` at 0 open, 0 orphaned.

Verified facts this plan relies on (grep/read at planning time):
- bildtext already has a left/right switch: block_list.html select named `variante`, Variant "rechts" renders `hc-bildtext--rechts`. Above/below extend that same field. The label key "Image sits" is replaced by "Image position".
- All 8 shipped themes (default, holzcloud, journal, magazine, midnight, rudel, schlicht, weide) link /assets/bausteine.css; only holzcloud, rudel and weide have their own hc-bildtext rules. bausteine.css keeps source order image then text "so a screen reader's order does not depend on a layout setting"; above/below keep that rule and use CSS order.
- Block markup is frozen into pages.content_html at save; old pages keep their old markup, so the new modifiers only appear on pages saved afterwards.
- The template data contract (loader.go, SampleData/MinimalData in sample.go) has no block fields: blocks arrive as rendered HTML in .Page.Content. Nothing to add there. internal/bundle copies Variant as a free string both ways, so oben/unten travel unchanged; a test pins it.
- keepUpload (internal/admin/media.go) is shared with the AI path (OpUploadMedia in ops_media.go). media.Check is the only thing that creates an *media.Intake, and Intake.Keep is the only way to store one.
- The route table in cmd/holzcloud/main_test.go must list every new route. Go's mux rejects a pattern like media/scale/{token} next to media/{mediaID}/delete; the new routes are therefore literal two-segment ones with the token in a query/form field.
- `go run ./tools/i18n` at HEAD: 2056 strings, four catalogues at "0 open, 0 orphaned". `go run ./tools/themewords -check` is green at HEAD.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@internal/block/block.go
@internal/block/render.go
@internal/block/form.go
@internal/admin/media.go
@internal/admin/ops_media.go
@internal/media/intake.go
@internal/media/variants.go
@internal/csvimport/store.go
@cmd/holzcloud/templates/admin/media_crop.html
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Image position (left, right, above, below) on the Image-and-text block, all 8 themes</name>
  <files>internal/block/block.go, internal/block/render.go, internal/block/imageposition_test.go, internal/bundle/blocks_position_test.go, internal/ai/tools_pages.go, internal/tmplspec/TEMPLATE-SPEC.md, cmd/holzcloud/templates/admin/block_list.html, cmd/holzcloud/assets/bausteine.css, cmd/holzcloud/templates/public/holzcloud/style.css, cmd/holzcloud/templates/public/rudel/style.css, cmd/holzcloud/templates/public/weide/style.css, internal/admin/page_blocks_test.go, internal/i18n/locales/de.json, internal/i18n/locales/de-CH.json, internal/i18n/locales/fr.json, internal/i18n/locales/it.json, internal/i18n/locales/es.json</files>
  <read_first>internal/block/block.go (Variant, DisplaySlideshow, DisplayClass, Clean), internal/block/render.go (the TypeImageText arm), internal/block/display_vocab_test.go (the vocabulary-test pattern to copy), cmd/holzcloud/templates/admin/block_list.html (the bildtext section), cmd/holzcloud/assets/bausteine.css (Bild und Text section), the hc-bildtext rules and the wide-slot rule of holzcloud, rudel and weide style.css, internal/admin/page_blocks_test.go (TestBlocksSurviveBeingEditedAgain for the full-path pattern), internal/tmplspec/TEMPLATE-SPEC.md section 4</read_first>
  <behavior>
    - Variant "" and "rechts" on a bildtext block render exactly `<div class="hc-block hc-bildtext">` and `<div class="hc-block hc-bildtext hc-bildtext--rechts">`, with inner markup unchanged (the old markup is the contract for every stored page)
    - Variant "oben" adds the modifier hc-bildtext--oben, "unten" adds hc-bildtext--unten; in all four cases the image block precedes the text block in the source
    - Clean keeps rechts, oben, unten on a bildtext block and turns "", "links" and any other string into "" (left is the empty value); other block types keep their Variant untouched (bild "voll", galerie "4"); the encoded JSON never carries an unknown value
    - A hand-edited Variant such as a quote followed by an attribute adds no class at all (the class is minted from constants, never concatenated)
    - FromForm turns b0.variante=unten into Variant "unten"
    - Full path through the admin: posting a page with a bildtext block and variante=oben stores and renders hc-bildtext--oben; the edit form shows four options with "above" selected
    - bausteine.css and each of holzcloud, rudel, weide style.css name both new modifiers (contract test); the other five themes are covered by bausteine.css plus the existing test that every theme links it
    - An export then import of a bildtext block keeps each of the four positions
  </behavior>
  <action>
Per the owner's decision: the position is the existing Variant of the bildtext block, no new JSON key, no new block type, no migration, left as the default.

1. internal/block/block.go: add three exported constants with English identifiers and German values, in the manner of DisplaySlideshow and with the same justification in the comment (a German word standing in the database is a value, not an identifier; see .planning/GLOSSARY.md closing rule): ImageRight = "rechts", ImageAbove = "oben", ImageBelow = "unten". Update the Variant field comment to say that for an image-text block it is the image position and that empty means left. Add a method Block.ImagePositionClass next to DisplayClass that returns the full modifier class (hc-bildtext--rechts, hc-bildtext--oben, hc-bildtext--unten) by equality against the constants and the empty string for everything else; the class is minted from the constant, never built from the stored string. In Set.Clean add one step beside the Display check: for TypeImageText keep Variant only when it is one of the three constants, otherwise set it to empty; leave every other type's Variant alone. Comment it as the same one-check-where-every-writer-meets rule as Display, covering the form, the AI tool and the archive.

2. internal/block/render.go, the TypeImageText arm: after the base class string, append a space and blk.ImagePositionClass() when it is non-empty, replacing the inline comparison against "rechts". Nothing else in the arm changes; the source order stays image, then text. Do not touch the frozen-markup contract of Variant "" and "rechts".

3. cmd/holzcloud/templates/admin/block_list.html, bildtext section: relabel the select with the key "Image position" (replacing "Image sits") and offer four options in this order: value "" labelled left (selected when Variant is none of rechts, oben, unten), "rechts" labelled right, "oben" labelled above, "unten" labelled below. Replace the hint with the key "On a narrow screen an image on the left or right sits above the text." (the old hint becomes untrue for below). Keep the select name `{{.Prefix}}.variante`. Every word goes through {{t}}.

4. internal/ai/tools_pages.go: change the builtinUses text of block.TypeImageText so the assistant is told: variant links (the default), rechts, oben or unten, naming the position of the image relative to the text. The wire and the Clean step need no further change.

5. CSS, per the owner's decision that all 8 themes support it. bausteine.css (Bild und Text section): inside the existing min-width 45em block make oben and unten a single column (grid-template-columns one column, align-items start); outside any media query add one rule that gives the image of hc-bildtext--unten order 2, so below holds on a phone as well as on a desktop. Extend the existing comment that explains the source-order rule so it covers all four positions. For the three themes with their own layout, keep the wide slot (text-start to breit-ende) for the side-by-side layouts only, because a stacked block is read as a column of text and a 1178 px line of prose is unreadable: in rudel and weide exclude the stacked modifiers from the hc-bildtext entry of the wide-slot :is() list using :where(:not(...)) so the rule's specificity is unchanged; in holzcloud do the equivalent for `.page-content > .hc-bildtext` so oben/unten get the theme's normal text column, and add oben/unten to the theme's min-width 45em column rules as a single column placed after the rechts rule. The holzcloud heading-spanning layout (the long :has() selector family keyed on image-then-heading-text) must not apply to oben or unten: add the same :where(:not(...)) exclusion to every selector of that family. Keep every comment in the stylesheet's own German register and say why in each place. Use plain CSS inside the existing structure, no new files.

6. internal/tmplspec/TEMPLATE-SPEC.md section 4: after the paragraph that says every rule in bausteine.css is one class deep, add a short passage for theme authors that lists the modifiers of the image-text block (hc-bildtext--rechts, hc-bildtext--oben, hc-bildtext--unten), says the markup order is always image then text, and that the core already stacks oben and unten. Write it in the document's English. No data-contract field changes, so SampleData, MinimalData and loader.go stay as they are; the spec tests must stay green.

7. Tests (write them first, red, per tdd): internal/block/imageposition_test.go with the render, Clean, FromForm and class-minting cases from the behavior list, plus a CSS contract test that reads ../../cmd/holzcloud/assets/bausteine.css and the three theme stylesheets and requires both modifier names in each (also assert the five other shipped themes' layout files link bausteine.css, reusing the idea of the existing tmplspec test rather than duplicating its failure message). internal/bundle/blocks_position_test.go: exportBlocks then importBlocks for each of the four positions keeps Variant. internal/admin/page_blocks_test.go: one full-path test in the style of the existing ones (post a page with a bildtext block, read the stored content_html and the edit form).

8. Catalogue: run `go run ./tools/i18n -write`, delete the two retired keys ("Image sits", "On a narrow screen the image always sits above the text.") from de, fr, it, es, fill the four new keys in those four files, then `go run ./tools/i18n -schweiz` to regenerate de-CH. Keep the register of the neighbouring entries. Suggested values: "Image position": Bildlage / Position de l'image / Posizione dell'immagine / Posición de la imagen; "above": oben / en haut / in alto / arriba; "below": unten / en bas / in basso / abajo; "On a narrow screen an image on the left or right sits above the text.": Auf einem schmalen Bildschirm steht ein Bild links oder rechts über dem Text. / Sur un écran étroit, une image à gauche ou à droite se place au-dessus du texte. / Su uno schermo stretto un'immagine a sinistra o a destra sta sopra il testo. / En una pantalla estrecha, una imagen a la izquierda o a la derecha queda encima del texto.
  </action>
  <verify>
    <automated>go test ./internal/block ./internal/bundle ./internal/ai ./internal/tmplspec ./internal/template ./internal/i18n -timeout 20m &amp;&amp; go test ./internal/admin -run 'ImagePosition|Block' -timeout 20m &amp;&amp; OUT=$(go run ./tools/i18n) &amp;&amp; printf '%s\n' "$OUT" | grep -q '0 open, 0 orphaned' &amp;&amp; ! printf '%s\n' "$OUT" | grep -Eq '[1-9][0-9]* (open|orphaned)' &amp;&amp; go run ./tools/themewords -check</automated>
  </verify>
  <done>The four positions are selectable in the editor, saved, normalised by Clean, rendered with the right class and styled in all 8 themes (stacked, column width in holzcloud/rudel/weide); Variant "" and "rechts" markup is byte-identical to before; the i18n gate still reports 4 catalogues at 0 open, 0 orphaned (baseline at HEAD is also 4, so this is a regression guard, not the proof of the feature; the proof is the new tests); the spec passage and the AI hint are in.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Scale-down question after upload (staging, plain-form page, Yes/No)</name>
  <files>internal/media/scale.go, internal/media/scale_test.go, internal/media/staging.go, internal/media/staging_test.go, internal/admin/handler.go, internal/admin/media.go, internal/admin/media_scale.go, internal/admin/media_scale_test.go, cmd/holzcloud/templates/admin/media_scale.html, cmd/holzcloud/main.go, cmd/holzcloud/main_test.go, internal/i18n/locales/de.json, internal/i18n/locales/de-CH.json, internal/i18n/locales/fr.json, internal/i18n/locales/it.json, internal/i18n/locales/es.json</files>
  <read_first>internal/admin/media.go (HandleMediaUpload, uploadOne, keepUpload, makeVariants), internal/admin/ops_media.go (OpUploadMedia, memFile), internal/media/intake.go (Check, Intake, Keep, Limits), internal/media/variants.go (resizeSlots, hasTransparency, encodeVariant, Dimensions, DefaultMaxMegapixels), internal/media/strip.go (what is removed and why), internal/csvimport/store.go (token, per-user check, Claim, Prune, the ErrExpired/ErrForeign split), internal/admin/csvimport.go (staged(), csvExpired), cmd/holzcloud/templates/admin/media_crop.html (plain-form page shape), cmd/holzcloud/main.go around the csv-import prune job and the media routes, internal/admin/media_screen_test.go and internal/admin/ops_media_test.go (mediaAdmin, mediaUploadRequest, photo helper), internal/web/nojs_test.go</read_first>
  <behavior>
    - media.ScaledSize: 3000x1500 gives 2560x1280; 1500x3000 gives 1280x2560; exactly 2560 on the long side is unchanged; the short side never drops below 1
    - Intake.Oversize is true for a JPEG/PNG/WebP/single-frame-GIF whose longest side is over 2560 and whose pixel count is within the megapixel budget; false for 2560 exactly, SVG, MP4, PDF, an animated GIF, an animated or undecodable WebP, and an image over the budget
    - Intake.Scaled returns a new Intake: JPEG stays JPEG, PNG stays PNG, WebP becomes JPEG when opaque and PNG when it has transparency, a still GIF becomes PNG; the long side is exactly 2560; the aspect ratio is kept within one pixel; no EXIF/XMP/comment survives; the name's extension follows the new format
    - Staging: Put then Get then Take then Discard round-trips bytes and names; Get with another website's id gives the expired error; Get with another user gives the foreign error; a token that is not 32 lower-case hex characters (empty, with ../, upper case, 31 characters) gives the expired error and touches nothing outside the staging root; a second Take of the same token gives the expired error; Prune removes staging older than the cutoff and keeps fresh ones
    - Upload of one JPEG over 2560: 303 to the scale page with a token, nothing in the library, one staging entry; the page lists the file, its size and the size it would become, has a plain POST form whose action is /admin/websites/{id}/media/scale, a hidden token, and two submit buttons named decision with values scale and keep
    - Decision scale: library holds one image with longest side 2560, same ratio, EXIF block of the source absent, staging gone, flash says File uploaded plus the scaled note; decision keep: library holds the original dimensions; both redirect to the media list (HX-Redirect when the request carries HX-Request: true)
    - Exactly 2560 px, SVG, an animated GIF and an over-budget image are stored at once with no question
    - A batch of two oversize files and one small file: the small one is stored at once, the two oversize ones share one page and one decision
    - A token of website A used on website B's route, or by another user, never reveals or stores anything (404 for foreign user, expired flash for foreign website); an unknown or malformed token gives the expired flash and a redirect to the media list, never a 500
    - Poisoned staging: if the staged bytes are replaced by something media.Check refuses, the decision stores nothing and reports the refusal
    - The AI path: OpUploadMedia with a 3000 px image stores it at 3000 px, asks nothing, stages nothing
    - A missing or unknown decision value changes nothing and shows the question again
  </behavior>
  <action>
Per the owner's decisions: the threshold is a named constant in the media package; the question page is a normal form POST with htmx only as an enhancement (any hx-post must equal the form action, internal/web/nojs_test.go); the upload is staged, scoped per website, with expiry; one decision for all files on the page; keepUpload stays the AI/ops door and does not ask; media.Check and the SVG scanner cannot be bypassed.

A. internal/media/scale.go (new). Export const MaxSide = 2560 with a doc comment saying why (longest side, pixels). Export func ScaledSize(width, height int) (int, int) as described in the behavior list (round to nearest, minimum 1). Export ErrNotScalable. Add method Intake.Oversize(maxMegapixels int) (width, height int, ok bool): only image/jpeg, image/png, image/webp, image/gif; read the size with image.DecodeConfig on the held bytes (never a full decode); false when the long side is at most MaxSide; false when width times height exceeds the budget (maxMegapixels 0 or less means DefaultMaxMegapixels), because Scaled could not decode it anyway and the existing too-large-for-scaled-copies warning already covers that file; for image/gif decode with image/gif DecodeAll after the budget check and answer false when there is more than one frame, because scaling would flatten the animation (the same reason CanMakeVariants leaves GIF out). Add method Intake.Scaled(maxMegapixels int) (*Intake, error): take a slot of resizeSlots for the duration, decode, scale with draw.CatmullRom into an RGBA of ScaledSize, choose the output with a small pure function scaledFormat(mimeType string, transparent bool) returning the new MIME type and extension (jpeg to jpeg; png to png; webp to jpeg or, when hasTransparency, png; gif to png), encode (JPEG at a named quality constant, 90; PNG default), and return a new Intake with the new MimeType, OriginalName whose extension is swapped only when the format changed, a fresh Filename from GenerateFilename, the new Size and bytes, and the same limit. Return ErrNotScalable (wrapped with the cause) when decoding fails, the format is not one of the four, or the scaled bytes exceed the limit. A Go encoder writes no EXIF, XMP, comment or ICC profile, which satisfies the metadata rule; Keep still runs PrepareUpload on the result. Say in the doc comment that the colour profile is not carried over, as with the existing scaled copies. Import image/gif; golang.org/x/image/webp and draw are already in go.mod.

B. internal/media/staging.go (new). Staging lives on disk, not in SQLite (decision of this plan: up to 20 images at the image limit each would be blobs in the single-writer file that also carries sessions; csvimport's per-user token, expired/foreign split and Prune are copied, its table is not). Export type Staging with NewStaging(dataDir), root = dataDir/media-staging (a sibling of media/, so the public media route can never serve it). Layout: root/{websiteID}/{token}/ with manifest.json (website id, user id, created time, per file: display name, width, height) and the held bytes as files named by index; directories 0700, files 0600. Put(websiteID, userID, intakes) writes into a temporary name and renames, so a half-written staging is never visible, records dimensions with image.DecodeConfig, and returns a 128-bit crypto/rand hex token (32 lower-case hex characters). Get(websiteID, token, userID) returns a *Staged; reject any token not matching exactly 32 lower-case hex characters BEFORE building a path (this is the path-traversal guard), answering the expired error; a missing directory is expired; a manifest user that differs from userID is the foreign error. Take does what Get does and then renames the directory to a .taken sibling so that exactly one caller gets it (a double submit gets expired). Staged has Content(i) and Discard(). Prune(olderThan) removes staging directories (including .tmp and .taken leftovers) whose modification time is older than the cutoff and returns how many went. Export the errors (ErrStagingExpired, ErrStagingForeign) and a StagingRetention constant of one hour.

C. internal/admin/media.go. Split keepUpload without changing its signature or behaviour: checkUpload(file, header) runs media.Check with the installation's limits; keepChecked(ctx, websiteID, in *media.Intake) does Keep, the plugin event, the variants and the re-read; keepUpload becomes checkUpload followed by keepChecked, so OpUploadMedia and the AI tools are untouched and never ask. Extract the tail of uploadOne that turns (mediaUpload, error) into an uploadResult into uploadResultOf(r, up, err, note string), where a non-empty note is one more part of the existing dash-joined message. Change uploadOne to return (uploadResult, *media.Intake): it opens the file, calls checkUpload (refusals map exactly as before), and when intake.Oversize(h.cfg.MaxMegapixels) says yes returns that intake for staging instead of storing it; otherwise keepChecked as before. Extract the flash aggregation of HandleMediaUpload (single result answers with its own sentence; several are summed up and every refusal named; the ohne_beschreibung redirect) into reportUploads(r, redirect, names, results) returning the redirect target, keeping every existing sentence. HandleMediaUpload then: processes all headers, stores the non-asked ones, reports them with reportUploads, and when any intake was asked calls h.mediaStaging.Put with the website id and the session user id (auth.SessionKeyUserID via h.sm.GetInt64) and redirects to /admin/websites/{id}/media/scale?token=TOKEN; a Put failure is reported as an Upload-failed flash and the redirect goes to the media list. Add the field mediaStaging *media.Staging to Handler in handler.go, built in NewHandler from cfg.DataDir.

D. internal/admin/media_scale.go (new). HandleMediaScaleForm (GET): website from the path (404 when absent), token from the query, Get with the session user; expired gives an error flash (new key: This upload has expired – please upload the images again.) and a redirect to the media list via h.redirect; foreign gives 404; otherwise render template media_scale with LayoutData titled web.Titlef(r, "Scale down to fit within %d px?", media.MaxSide), ActiveNav media, CurrentWebsite, the files (name, size, scaled size from media.ScaledSize), MaxSide and the token. HandleMediaScale (POST): same website and token handling; read decision; anything other than scale or keep redirects back to the GET page without touching the staging; Take; defer Discard; for each file build a multipart header and a memFile (ops_media.go), run checkUpload again (so media.Check and the SVG scanner run on the staged bytes, which is also what defends against a tampered staging file), on scale call Intake.Scaled and on success continue with the scaled intake plus the note web.Titlef(r, "scaled down to fit within %d px", media.MaxSide), on ErrNotScalable log a slog.Warn and keep the original with the note web.T(r, "could not be scaled down, kept as uploaded"); then keepChecked and uploadResultOf; finally reportUploads and h.redirect to the media list.

E. cmd/holzcloud/templates/admin/media_scale.html (new). Follow media_crop.html: define "actions" (back link, reuse the key Back to the media library), "content" and "media_scale-content". A card with the question as heading ({{tf "Scale down to fit within %d px?" .MaxSide}}), an intro sentence (tf with MaxSide), a list with the file name (escaped by the template engine) and the line from {{tf "%d × %d px, becomes %d × %d px" ...}}, then ONE plain form: method POST, action /admin/websites/{{.WebsiteID}}/media/scale, hidden gorilla.csrf.Token, hidden token, and two submit buttons name decision with values scale (btn btn--primary, Yes, scale down) and keep (btn, No, keep the originals), both with hx-disabled-elt="this" per CLAUDE.md. Do not put hx-post on the form: the page then has one path with and without the script, and nojs_test.go holds. A hint sentence says the choice applies to every image on the page and that a scaled image is saved anew and the original is not kept. Reuse existing admin classes; add rules to the admin stylesheet (kebab-case, inside @layer components) only if the list needs them.

F. cmd/holzcloud/main.go: register GET and POST /admin/websites/{id}/media/scale beside the other media routes (no requireAdmin, like upload), and add a job named media-staging-prune, every 15 minutes, RunAtStart true (staging lives on disk and survives a restart, so a startup sweep of stale directories is safe, unlike the csv job), calling media.NewStaging(cfg.DataDir).Prune with media.StagingRetention and logging the count when non-zero. cmd/holzcloud/main_test.go: add both patterns to the route table beside the media entries.

G. Tests, written first. internal/media/scale_test.go and staging_test.go cover the behavior list for those packages (build images with the standard encoders; WebP has no encoder here, so test scaledFormat as a table and use the files in internal/media/testdata only for decode-side checks). internal/admin/media_scale_test.go covers every admin line of the behavior list with mediaAdmin, mediaUploadRequest, serve and the photo helper, driving the real handlers (HandleMediaUpload, HandleMediaScaleForm, HandleMediaScale); assert that the GET page contains no hx-post that differs from the action. Check library content with media.NewStore(database).List and dimensions with media.Dimensions on the stored path (media.Path).

H. Catalogue: `go run ./tools/i18n -write`, fill de, fr, it, es, then `go run ./tools/i18n -schweiz`. Keys and suggested values (de uses the sharp s, -schweiz converts it): "Scale down to fit within %d px?": Passend verkleinern auf höchstens %d px? / Réduire pour tenir dans %d px ? / Ridimensionare per stare entro %d px? / ¿Reducir para que quepa en %d px?; "These images are larger than %d px on their longest side. Scaled down to fit, they load faster and take up less space; the proportions stay the same.": Diese Bilder sind auf der längsten Seite größer als %d px. Passend verkleinert laden sie schneller und brauchen weniger Platz; das Seitenverhältnis bleibt gleich. / Ces images mesurent plus de %d px sur leur plus grand côté. Réduites à la bonne taille, elles se chargent plus vite et prennent moins de place ; les proportions restent les mêmes. / Queste immagini superano %d px sul lato più lungo. Ridimensionate, si caricano più in fretta e occupano meno spazio; le proporzioni restano uguali. / Estas imágenes superan los %d px en su lado más largo. Reducidas, cargan más rápido y ocupan menos espacio; las proporciones no cambian.; "%d × %d px, becomes %d × %d px": %d × %d px, wird zu %d × %d px / %d × %d px, devient %d × %d px / %d × %d px, diventa %d × %d px / %d × %d px, pasa a %d × %d px; "Yes, scale down": Ja, verkleinern / Oui, réduire / Sì, ridimensiona / Sí, reducir; "No, keep the originals": Nein, Originale behalten / Non, garder les originaux / No, tieni gli originali / No, conservar los originales; "The choice applies to every image on this page. A scaled image is saved anew; the original is not kept.": Die Wahl gilt für alle Bilder auf dieser Seite. Ein verkleinertes Bild wird neu gespeichert; das Original bleibt nicht erhalten. / Le choix vaut pour toutes les images de cette page. Une image réduite est enregistrée à nouveau ; l'original n'est pas conservé. / La scelta vale per tutte le immagini di questa pagina. Un'immagine ridimensionata viene salvata di nuovo; l'originale non viene conservato. / La elección vale para todas las imágenes de esta página. Una imagen reducida se guarda de nuevo; el original no se conserva.; "This upload has expired – please upload the images again.": Dieser Upload ist abgelaufen – bitte lade die Bilder noch einmal hoch. / Ce téléversement a expiré – veuillez envoyer à nouveau les images. / Questo caricamento è scaduto – carica di nuovo le immagini. / Esta subida ha caducado – vuelve a subir las imágenes.; "scaled down to fit within %d px": auf höchstens %d px verkleinert / réduite pour tenir dans %d px / ridimensionata entro %d px / reducida para caber en %d px; "could not be scaled down, kept as uploaded": ließ sich nicht verkleinern, so gespeichert wie hochgeladen / n'a pas pu être réduite, enregistrée telle quelle / non è stato possibile ridimensionarla, salvata così com'è / no se pudo reducir, guardada tal como se subió. Adjust each to the register of its neighbours. Keep the sentence assembled by Go limited to the dash-joined catalogue parts, never a Sprintf of German or English prose (CLAUDE.md, Translation).
  </action>
  <verify>
    <automated>go vet ./internal/media ./internal/admin ./cmd/holzcloud &amp;&amp; go test ./internal/media ./internal/web ./internal/i18n -timeout 20m &amp;&amp; go test ./internal/admin -run 'Media|Upload|Scale' -timeout 20m &amp;&amp; go test ./cmd/holzcloud -timeout 20m &amp;&amp; OUT=$(go run ./tools/i18n) &amp;&amp; printf '%s\n' "$OUT" | grep -q '0 open, 0 orphaned' &amp;&amp; ! printf '%s\n' "$OUT" | grep -Eq '[1-9][0-9]* (open|orphaned)'</automated>
  </verify>
  <done>An oversize raster upload is staged and answered with the Yes/No page; Yes stores a 2560 px copy without camera data, No stores the original; every exempt kind and the AI path store at once; staging is reachable only by its own website and user, re-checked on resume, and pruned by a job; both new routes are in the route table; the i18n gate is at 0 open, 0 orphaned for the four catalogues and the Swiss one is regenerated.</done>
</task>

<task type="auto">
  <name>Task 3: Documentation, whole-tree gates and a browser check of both features</name>
  <files>docs/content-model.md, docs/publishing.md, docs/configuration.md</files>
  <read_first>docs/content-model.md (Block kinds section), docs/publishing.md (Media section), docs/configuration.md (data directory listing and the HOLZCLOUD_MAX_MEGAPIXELS row)</read_first>
  <action>
1. docs/content-model.md, Block kinds: after the sentence that lists the nine kinds, add one sentence saying that Image and text can place the image to the left or right of the text (left by default), or above or below it, and that on a narrow screen left and right become above. docs/publishing.md, Media: add a short paragraph after the camera-data one: when the longest side of a JPEG, PNG, WebP or still GIF is over 2560 px, the upload stops at a page asking whether to scale it down to fit within 2560 px; Yes stores a re-encoded copy (JPEG stays JPEG, PNG stays PNG, WebP becomes JPEG or PNG, a GIF becomes PNG; the colour profile is not carried over and the original is not kept), No stores the original; one decision covers all large files of one upload; SVG, PDF, MP4, animated GIFs, images beyond the megapixel limit and uploads by AI key or upload link are never asked. docs/configuration.md: add media-staging/{website_id}/ to the data directory listing as images waiting for that question, cleared after an hour and not part of the backup, and extend the HOLZCLOUD_MAX_MEGAPIXELS row to say the scale-down question is only offered for images within that limit.

2. No CHANGELOG entry and no version bump: the release of milestone v0.1.1 owns both (project memory: milestone and release carry one number). Say so in the SUMMARY so the owner can add an entry if a release is cut earlier.

3. Whole-tree gates, in this order, fixing what they report: gofmt over internal, cmd and tools prints nothing; go vet ./...; go run ./tools/i18n shows 0 open, 0 orphaned for de, fr, it, es; go run ./tools/themewords -check; go run ./tools/english, which is red at HEAD for one unrelated file (build/devseed/main.go, 2 German lines) and must name no other file after this change; then the full test run once.

4. Browser check with the Playwright MCP, never handed back to the owner. Build with CGO_ENABLED=0 into the scratchpad directory, run the binary with HOLZCLOUD_DATA_DIR set to a fresh scratch directory and HOLZCLOUD_PORT set to a free port, complete the setup screen, create a website. (a) Upload a generated 3000 by 1800 JPEG plus a small PNG in one request: the PNG appears in the library at once, the page asks about the JPEG, click No on one run and Yes on another and read the stored dimensions in the library. Repeat with JavaScript disabled in the browser context to prove the plain-form path. (b) In the page editor add an Image-and-text block with each position, save, and open the public page at 1280 px and 390 px width with the default theme and with holzcloud, rudel and weide (activate each shipped theme from the website design screen): above and below stack, left and right sit beside the text at desktop width, the image comes first on a phone except for below, nothing overlaps, no horizontal scroll. Save screenshots in the scratchpad directory only, describe what was seen in the SUMMARY, and if the browser run proves infeasible say so there and rely on the tests.
  </action>
  <verify>
    <automated>[ -z "$(gofmt -l internal cmd tools)" ] &amp;&amp; go vet ./... &amp;&amp; OUT=$(go run ./tools/i18n) &amp;&amp; printf '%s\n' "$OUT" | grep -q '0 open, 0 orphaned' &amp;&amp; ! printf '%s\n' "$OUT" | grep -Eq '[1-9][0-9]* (open|orphaned)' &amp;&amp; go run ./tools/themewords -check &amp;&amp; { EN=$(go run ./tools/english 2>&1 || true); ! printf '%s\n' "$EN" | grep -E '^ +[0-9]+ ' | grep -v 'build/devseed/main.go'; } &amp;&amp; go test ./... -timeout 30m</automated>
  </verify>
  <done>Docs describe both features; gofmt, vet, i18n (4 catalogues at 0 open, 0 orphaned), themewords, english (only the pre-existing build/devseed finding) and the full test suite pass; the browser check shows the question page working with and without JavaScript, the Yes and No outcomes, and the four image positions in the default theme and in holzcloud, rudel and weide at 1280 and 390 px.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser to /admin/websites/{id}/media/upload and /media/scale | Authenticated operator input: multipart files, a token, a decision value; the token is attacker-controllable text that becomes a filesystem lookup |
| staged bytes on disk to media.Check and the encoders | Bytes written by an earlier request are read back; they are re-validated, not trusted |
| stored block Variant to public HTML class attribute | A string from a form, the AI wire or an archive reaches an HTML attribute on every visitor's page |
| image decoder to server memory | An attacker-chosen image header can claim a huge pixel area |

## STRIDE Threat Register

Threat IDs are unique within this quick task; there are no earlier PLAN files in this directory.

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-bon-01 | Tampering | media.Staging token to path | high | mitigate | Token must match exactly 32 lower-case hex characters before any path is built; website id comes from the parsed route integer; staging tests assert that ../ and malformed tokens touch nothing outside the root |
| T-bon-02 | Information disclosure | Staged upload read by another website or user | high | mitigate | Directory is root/{websiteID}/{token}, so another website's route cannot find it (expired); manifest user id must equal the session user (foreign gives 404); isolation tests for both; staging root is a sibling of media/ and never served |
| T-bon-03 | Tampering | Bypass of media.Check or the SVG scanner via the staging path | high | mitigate | Resume runs checkUpload (media.Check) on the staged bytes again; keepChecked accepts only an *media.Intake, which only Check or Intake.Scaled can produce; poisoned-staging test; keepUpload stays Check plus keepChecked for the AI path |
| T-bon-04 | Denial of service | Decode bomb in Oversize/Scaled | high | mitigate | Size is read with DecodeConfig first; images over the megapixel budget are never offered and never decoded; decoding goes through the existing resizeSlots throttle; a failed decode falls back to storing the original |
| T-bon-05 | Denial of service | Disk filled by abandoned stagings | medium | mitigate | At most 20 files per request, each already bounded by media.Check limits; prune job every 15 minutes with one hour retention plus RunAtStart; directories 0700 and files 0600 |
| T-bon-06 | Tampering | Variant string into the class attribute (stored XSS) | high | mitigate | Class minted from constants by equality (ImagePositionClass), Clean normalises the vocabulary for bildtext on every writer path; test with a quote-and-attribute payload |
| T-bon-07 | Tampering | CSRF on the new POST route | medium | mitigate | Route is on adminProtectedMux behind the existing CSRF middleware; the form carries gorilla.csrf.Token; the route table test lists it |
| T-bon-08 | Tampering | Double submit processes one staging twice | low | mitigate | Take renames the directory atomically so only one request gets it; Keep also refuses duplicate bytes by hash |
| T-bon-09 | Information disclosure | Camera metadata kept in a scaled copy | medium | mitigate | Re-encoding with the standard encoders writes no EXIF, XMP or comments; Keep still runs PrepareUpload; test asserts the source's EXIF marker string is absent |
| T-bon-SC | Tampering | npm/pip/cargo installs | high | accept | No package is installed by this plan: image/gif is standard library, golang.org/x/image is already in go.mod; no legitimacy checkpoint needed |
</threat_model>

<verification>
- Task 1 and Task 2 automated commands pass independently and each leaves the i18n gate at 4 catalogues of 0 open, 0 orphaned.
- Task 3's whole-tree command passes: gofmt, vet, i18n, themewords -check, english (no file besides the pre-existing build/devseed one), go test ./...
- Existing pages: a stored Variant "" or "rechts" bildtext block produces byte-identical markup (asserted in internal/block/imageposition_test.go).
- Browser check recorded in the SUMMARY: question page with and without JavaScript, Yes and No outcomes, four positions in four themes at two widths.
</verification>

<success_criteria>
- The editor offers Image position with left, right, above, below; old blocks unchanged; all 8 shipped themes handle all four; spec, AI hint, bundle and catalogue updated.
- An upload over 2560 px on its longest side asks once, on one page for the whole upload, without JavaScript; Yes gives an exactly 2560 px copy without metadata, No keeps the original; exempt kinds and the AI path never ask.
- media.Check and the SVG scanner run on every path; staging is per website and user, token-guarded, taken atomically and pruned.
- `go run ./tools/i18n` reports 0 open, 0 orphaned; `go run ./tools/themewords -check` is green; tests added in internal/block, internal/bundle, internal/media, internal/admin and cmd/holzcloud.
</success_criteria>

<output>
Create `.planning/quick/261026-bon-block-bild-und-text-bildlage-links-recht/261026-bon-SUMMARY.md` when done
</output>
