---
phase: 11-galerie
plan: 04
subsystem: block model, block renderer, block stylesheet, bundle format, block editor
tags: [gallery, slideshow, scroll-snap, display-mode, bundle-round-trip, a11y]
status: complete

requires:
  - "11-01 — GalleryItems, Set.T, and renderOne's (s Set, at int) signature"
provides:
  - "block.Block.Display — a second display axis on a block, json key
    darstellung, omitempty"
  - "block.DisplaySlideshow = \"diashow\" — the one named non-default"
  - "block.Block.DisplayClass() — the minted modifier class, beside Columns()"
  - "bundle.Block.Display — the field in the manifest and in BOTH struct
    literals of blocks.go, so the value survives the archive round trip"
  - "the .hc-galerie--diashow section of cmd/holzcloud/assets/bausteine.css"
affects:
  - "11-05 (album) — an album rendering through GalleryItems inherits the same
    markup; giving an album a display mode is a field on the album, not here"
  - "11-07 — three new German source strings are open, and the browser pass
    owns the scroll-width measurement this plan cannot make"

tech-stack:
  added: []
  patterns:
    - "a second display axis as its own field with one named constant and an
      empty default — field.Def.Display / DisplayButtons, one package over"
    - "the modifier class minted from the constant in a method beside the
      constant, never concatenated out of the stored string"
    - "one markup, two stylesheets: the two modes differ in the wrapper's
      attributes and in nothing else"

key-files:
  created: []
  modified:
    - internal/block/block.go
    - internal/block/form.go
    - internal/block/render.go
    - internal/block/block_test.go
    - internal/bundle/format.go
    - internal/bundle/blocks.go
    - internal/bundle/bundle_test.go
    - cmd/holzcloud/templates/admin/block_list.html
    - cmd/holzcloud/assets/bausteine.css

decisions:
  - "The choice is Block.Display and not a fourth value of Variant. Measured:
    Variant is already the gallery's column count (block.go Columns(), the
    editor's select), so Variant = \"diashow\" would silently render a
    three-column grid with no error anywhere."
  - "DisplaySlideshow is an English identifier holding a German value,
    \"diashow\", per .planning/GLOSSARY.md's closing rule — the string stands in
    the blocks column of every page that uses it and in every archive ever
    exported."
  - "The modifier class is hc-galerie--diashow, in the file's own BEM register
    (hc-karte--link), and it is minted by DisplayClass() from the constant, so
    nothing a hand-edited archive carries reaches a class attribute (T-11-18)."
  - "The slideshow wrapper carries tabindex=\"0\", role=\"region\" and an
    aria-label. role=\"region\" is what exposes the name on a plain div; without
    it a tab stop leads into a region a screen reader cannot announce."
  - "The accessible name reuses the existing catalogue key \"Galerie\" —
    measured to add 0 source strings — rather than minting a fifth."
  - "The revealed large view inside a slideshow overrides the lightbox
    section's grid-column: 1 / -1 to grid-column: auto, so it is a column of the
    track rather than the whole row. This is a second :target selector, which
    the plan's summary table predicted as zero — see Divergences."

metrics:
  duration: "~25 min"
  completed: 2026-09-07

actuals:
  tokens: 53799
  tasks: 3
  commits: 4
---

# Phase 11 Plan 04: Grid or slideshow — Summary

A gallery block carries a display mode of its own, the same markup lays itself
out as a wrapping grid or as a horizontal track that snaps to each picture, a
keyboard can enter the track, and nothing about it is scripted.

## What was built

**Task 1 — one new field, six landing places** (red at `3519b71`, green at
`6779fad`).

`block.Block.Display` sits beside `Variant`, `json:"darstellung,omitempty"`,
with a doc comment saying in two sentences why it is not another meaning of
`Variant`. `DisplaySlideshow = "diashow"` is the one named non-default, on
`field.DisplayButtons`' model, with the comment that says why the value is
German and the identifier English. `DisplayClass()` sits beside `Columns()` and
maps the constant to `hc-galerie--diashow` — a value it does not know maps to
the empty string, so no fragment of a stored value ever reaches the markup.

`setBlockField` gained `case "darstellung":`, which stores the empty string and
the one constant and **ignores anything else**. `block_list.html` gained a
second select beside the columns one, in the `{{if .HasItems}}` branch and
guarded to `{{if eq .Block.Type "galerie"}}` — a card row has items too and a
snapping track of cards is not what this phase built.

`bundle.Block.Display` landed in the manifest struct **and in both struct
literals of `blocks.go`** — export at `:32` and import at `:76`. That pair is
the whole reason this was one commit: one without the other is a value that
leaves in the archive and never comes back, with nothing anywhere to report it.

**Task 2 — the track that snaps** (red at `9350775`, green at `50d4cab`).

The gallery arm of `renderOne` appends the modifier and, in slideshow mode
only, `tabindex="0" role="region" aria-label="…"`. **Everything inside the
wrapper is byte for byte identical in both modes** — the tiles, the large
views, the fragment ids, the controls — which is what makes GAL-06 one block of
CSS rather than a second renderer, and what keeps the lightbox working in both
modes for nothing. `TestSlideshowAddsOnlyTheModifierAndTheAccessibleName`
proves it by diffing the two renderings rather than by asserting it.

`bausteine.css` gained 78 appended lines and deleted none. The grid rules at
`:103-144` are untouched, so a gallery without the modifier renders today's
markup through today's rules. The new section turns the same grid into one row
of columns (`grid-template-columns: none`, `grid-auto-flow: column`,
`grid-auto-columns: min(80%, 22rem)`), scrolls it (`overflow-x: auto`), snaps
it (`scroll-snap-type: inline mandatory` on the track,
`scroll-snap-align: start` on every direct child) and keeps the end of the
track from handing the gesture to the page behind
(`overscroll-behavior-inline: contain`).

**Task 3 — the arithmetic** (no code; the table below).

## The counting gates, measured against the post-change tree

| What | Baseline | This plan adds | Predicted | **Measured** |
|---|---|---|---|---|
| fields on `block.Block` | 14 | 1 (`Display`) | 15 | **15** |
| fields on `bundle.Block` | 14 | 1 (`Display`) | 15 | **15** |
| `Display:` in `blocks.go`, comments stripped | 0 | 2 | 2 | **2** |
| `case "darstellung":` in `form.go` | 0 | 1 | 1 | **1** |
| `darstellung` in `block_list.html` | 0 | 2 | ≥ 1 | **2** |
| `DisplaySlideshow` in `block.go`, comments stripped | 0 | 2 | ≥ 2 | **2** |
| `scroll-snap-type` / `-align`, comments stripped | 0 / 0 | 1 / 1 | both > 0 | **1 1** |
| `position: fixed\|absolute` / `visibility:` | 0 / 0 | 0 / 0 | both 0 | **0 0** |
| deleted lines in `bausteine.css` | — | 0 | 0 | **0** |
| removed lines mentioning `Columns` in `block.go` | — | 0 | 0 | **0** |
| `:target` rules in `bausteine.css` | 1 | 0 | unchanged | **2 — diverged** |
| files using `BeginTx` | 15 | 0 | 15 | **15** |
| migrations | 50 | 0 | 50 | **50** |
| admin templates | 66 at plan start | 0 | *not asserted* | 68 — **11-03's, not mine** |
| **source strings, isolated** | see below | +3 | measured, not predicted | **+3** |

### The i18n row, measured by isolation rather than by a whole-tree total

A whole-tree total is unsatisfiable in this wave: 11-03 was landing strings in
the same tree, and the tree count moved from **1280 / 3 offen** at plan start to
**1306 / 29 offen** at plan end. Neither number is this plan's to assert.

So the honest form was used — *removing this plan's work must not change the
count* — measured in a scratch copy of commit `50d4cab`, replacing only this
plan's two string-bearing files with their pre-plan versions and re-running
`go run ./tools/i18n -root <copy>`:

| Scratch tree | Strings |
|---|---|
| this plan's work present | **1292** |
| `block_list.html` reverted to pre-plan | 1289 |
| `render.go` reverted to pre-plan (11-01's three lightbox strings go with it) | 1289 |
| both reverted | 1286 |

**This plan adds exactly +3 source strings**, all in `block_list.html`:
`Raster`, `Diashow`, and the hint sentence
`Eine Diashow legt die Bilder nebeneinander in eine Spur, die man seitwärts
schiebt und die an jedem Bild einrastet.`

**And the accessible name added zero.** Reverting `render.go` alone drops the
count by exactly 3 — which is 11-01's `Vorheriges Bild`, `Nächstes Bild`,
`Grossansicht schliessen` and **nothing of mine**. `textGallery = i18n.N("Galerie")`
reuses the key the block kind at `block.go:77` already carries, translated in
`en`, `es`, `fr` and `it` since long before this phase. `Darstellung` was also
already a key, which is why four new literals cost three new strings.

The catalogues were **not** filled: `internal/i18n/locales/*.json` is not in
this plan's `files_modified` and plan 11-07 owns them. The standing `29 offen`
is expected, not a regression; **3 of those 29 are this plan's.**

## Gate output, literal

| Gate | Expected | Measured |
|---|---|---|
| `go build ./... && go vet ./internal/block/ ./internal/bundle/ && gofmt -l …` | clean | `exit=0`, `gofmt -l` printed nothing |
| `Display:` in `blocks.go`, comments stripped | 2 | `2` |
| `case "darstellung":` in `form.go`, comments stripped | 1 | `1` |
| `grep -c 'darstellung' block_list.html` | > 0 | `2` |
| `DisplaySlideshow` in `block.go`, comments stripped | ≥ 2 | `2` |
| `go test ./internal/block/ -run TestKodierenUndLesenIstVerlustfrei -v` | no FAIL, ran | `--- PASS: TestKodierenUndLesenIstVerlustfrei (0.00s)`, `ok … 0.269s` |
| `go test ./internal/bundle/ -run TestRoundTripKeepsBlocks -v` | no FAIL, ran | `--- PASS: TestRoundTripKeepsBlocks (0.09s)`, `ok … 0.391s` |
| removed lines mentioning `Columns` in `block.go` | 0 | `0` |
| `go test ./internal/block/ -run 'Slideshow\|Gallery' -v` | no FAIL, ran | `10 --- PASS`, `0 --- FAIL` |
| snap type / snap align, comments stripped | both > 0 | `1 1` |
| `position: fixed\|absolute` / `visibility:`, comments stripped | `0 0` | `0 0` |
| deleted lines in `bausteine.css` | 0 | `0` |
| `block.Block` fields, struct parsed | 15 | `15` |
| `bundle.Block` fields, struct parsed | 15 | `15` |
| `grep -rln BeginTx internal/ --include='*.go' \| grep -v _test \| wc -l` | 15 | `15` |
| `ls internal/db/migrations/*.sql \| wc -l` | 50 | `50` |
| `go run ./tools/i18n` | count up, name adds no key | `1306`, `29 offen`; **isolated +3, name +0** |
| `go build ./... && go vet ./... && gofmt -l . && go test ./...` | clean, no FAIL | `build ok`, `vet ok`, `gofmt` printed nothing, no `FAIL` |

## Divergences, with direction and cause

**1. `:target` rules went from 1 to 2; the plan's table predicted 0 added.**
Direction: one more than the table row. Cause: the plan's table and the plan's
own action text contradict each other, and the action is right. The action
requires that "the revealed large view inside a track must be a member of the
track and not something placed over it." The lightbox section gives
`.hc-galerie__gross:target` a `grid-column: 1 / -1`, which is correct for a
wrapping grid and means *the whole row* in a single-row track. Making it a
member of the track can only be written by overriding that declaration, and the
override must match the same `:target` state — hence
`.hc-galerie.hc-galerie--diashow .hc-galerie__gross:target { grid-column: auto;
grid-row: 1; inline-size: min(90vw, 48rem); }`. The row was a summary-table
entry and not a `<verify>` gate, so nothing failed; it is recorded rather than
adjusted. The hiding technique is unchanged: still `display: none`, still no
placement out of flow, both gated at `0`.

**2. The plan's `<verification>` block says "`bundle.Block` has 13"; its own
task-3 gate says 15, and 15 is right.** Direction: the prose is two low. Cause:
a stale number in the closing checklist — the struct was measured at **14**
fields on the pre-change tree (`Type`, `Markdown`, `Media`, `Poster`, `Alt`,
`Caption`, `Variant`, `Title`, `Text`, `Source`, `LinkText`, `LinkURL`,
`Items`, `Fields`) and the gate's own `<fails_when>` states 14 as the baseline.
The executable gate was followed.

**3. `go test ./...` failed transiently mid-run with 10 `--- FAIL` lines in
`internal/admin`, `internal/web` and `cmd/holzcloud`, and was NOT touched.**
Direction: failures in packages outside this plan's `files_modified`. Cause: the
parallel 11-03 executor's mid-edit state in the shared tree — `git status` at
that moment showed `internal/web/render.go`, `base.html`, `icons.html` and two
new album templates in flight. A `gofmt -l .` in the same second reported
`size of internal/web/render.go changed during reading`. Both cleared on their
own once 11-03 committed `ad99438`/`38a4baa`; the final chain runs green. This
is the same class of event 11-01 recorded as its divergence 5, and the same
answer was given: leave it alone.

**4. `TestGalleryWithoutDisplayRendersTodaysMarkup` was green in the RED
commit.** Direction: one of task 2's three named tests could not fail before the
implementation. Cause: it is a *pinning* test by construction — it asserts the
grid renders what it renders today, which is true both before and after. It is
recorded here rather than silently skipped past, per the TDD fail-fast rule. The
other two of the three did fail at `9350775`, with the exact messages `the
slideshow renders the same wrapper as the grid` and `the name did not go through
Set.T`.

## What the plan and 11-01-SUMMARY got wrong about the tree

- **`setBlockField`'s signature is `(b *Block, name string, values []string)`,
  not `(…, value string)`.** The plan's action writes `case "darstellung":` as
  if the case saw a bare string; it sees `value`, a local derived from `values`
  further up the function. No behaviour changed, but a test calling
  `setBlockField(&b, "darstellung", "x", Builtin)` — which is what the plan's
  shape implies — does not compile.
- **The plan's `<verification>` "bundle.Block has 13" contradicts its own gate**
  (divergence 2).
- **The plan's `:target` table row contradicts its own task-2 action**
  (divergence 1).
- **The admin-template row is correctly absent from the gate but still stands in
  task 3's table as "68 after 11-03".** Measured: 66 at this plan's start, 68 at
  its end, and both of those two are 11-03's `album_list.html` and
  `album_edit.html`. This plan adds no template. The number is reported here and
  not asserted, exactly as instructed.
- **11-01-SUMMARY was right and the plan was wrong about `renderOne`.** The
  signature in this tree is `renderOne(b *strings.Builder, at int, blk Block,
  s Set, look Lookup, md Markdown)`. Without the `Set` the gallery arm cannot
  reach `s.text`, and the accessible name would have had to be a hard-coded
  German literal. Everything 11-01-SUMMARY says about the tree held.
- **`i18n.N("Galerie")` already existed at `block.go:77`,** inside the block-kind
  table, so `render.go`'s `textGallery` costs nothing. `Darstellung` was already
  a key too. The plan assumed one reuse and got two.

## Known Stubs

None. No `TODO`, `FIXME`, placeholder value or skipped test was added by any of
the four commits. The `placeholder=` attributes in `block_list.html` are HTML
input placeholders and predate this plan.

## Threat Flags

None new. The register's dispositions are satisfied:

- **T-11-17** — `setBlockField` accepts `""` and `DisplaySlideshow` and stores
  nothing else, proved by
  `TestSetBlockFieldIgnoresADisplayOutsideTheVocabulary`.
- **T-11-18** — `DisplayClass()` mints the class from the constant; the stored
  string is never concatenated into the markup, proved by
  `TestDisplayClassIsMintedAndNotConcatenated`, which feeds it
  `x" onload="alert(1)` and gets the empty string. The accessible name goes
  through `html.EscapeString`.
- **T-11-19** — no placement out of flow anywhere in the file, gated with
  comments stripped at `0 0`. **The scroll-width property itself is not
  verified here** — see below.
- **T-11-20** — `Display:` appears in both struct literals of `blocks.go`, gated
  at 2, and `TestRoundTripKeepsBlocks` asserts the value on the imported
  gallery.
- **T-11-SC** — `go.mod` is untouched; no import was added to any package. No
  package-manager install ran.

## Not verified here

**No browser pass.** Plan 11-07 owns it. Two things this plan cannot prove and
does not claim:

1. **The scroll width of a page carrying a slideshow.** No unit test can see it.
   It must be measured at three widths against the real files, the way
   `.planning/STATE.md:209` was measured before it was believed — and a
   horizontal scroll container is the easiest place in this phase to reproduce
   that finding, because it is a scroll container on purpose. The stylesheet
   comment says so in the file.
2. **That the select renders with the current choice selected.** The markup is
   `{{if ne .Block.Display "diashow"}}` on the grid option and
   `{{if eq .Block.Display "diashow"}}` on the slideshow one, so an unknown
   stored value falls back to the grid being selected. The editor form is
   `internal/admin`'s, which is 11-03's territory in this wave, so no rendering
   test of it was added here.

Also not done here: the three new German strings are **not** translated.
`internal/i18n/locales/*.json` belongs to plan 11-07.

## Self-Check: PASSED

- `internal/block/block.go` — FOUND
- `internal/block/form.go` — FOUND
- `internal/block/render.go` — FOUND
- `internal/block/block_test.go` — FOUND
- `internal/bundle/format.go` — FOUND
- `internal/bundle/blocks.go` — FOUND
- `internal/bundle/bundle_test.go` — FOUND
- `cmd/holzcloud/templates/admin/block_list.html` — FOUND
- `cmd/holzcloud/assets/bausteine.css` — FOUND
- commits `3519b71`, `6779fad`, `9350775`, `50d4cab` — all FOUND in `git log`
