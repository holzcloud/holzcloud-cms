---
phase: 11-galerie
plan: 05
subsystem: block model, block renderer, album expansion, public render pipeline, bundle format, block editor
tags: [album, gallery, late-binding, marker, website-scoping, gal-03, gal-07]
status: complete

requires:
  - "11-01 — GalleryItems, Set.T, and renderOne's (s Set, at int) signature"
  - "11-02 — album.Store, Album.Slug, Items()/Pictures(), MaxItems"
  - "11-03 — the album screens and main.go's albumStore"
  - "11-04 — Block.Display, DisplayClass(), and the second struct literal in blocks.go"
provides:
  - "block.Block.AlbumSlug — a gallery names an album of this website instead of listing pictures, JSON key album, omitempty"
  - "the album marker — block.AlbumMarker / HasAlbumMarker / AlbumMarkerSlugs / ReplaceAlbumMarkers, declared in internal/block/render.go and nowhere else"
  - "Empty() and Clean taught that a gallery with an album is not empty — the block is no longer deleted by the save that creates it"
  - "internal/album/expand.go — Set, Set.Lookup, UsedSlugs, Expand: the request-time half"
  - "album.Store.LoadFor — one query for the albums a page names, none for a page that names none"
  - "public.Handler.SetAlbumStore, and the expansion in pageContent between snippet.Expand and filterByPlugins"
  - "bundle.Block.Album — the slug in the manifest and in both struct literals, a pass-through until 11-06"
  - "the album select on the gallery block, with the block's own list folded away rather than thrown away"
affects:
  - "11-06 — replaces the bundle pass-through with the name translation that makes GAL-04 survive a rename"
  - "11-07 — owns the four new German strings (33 offen now) and the browser pass"

tech-stack:
  added: []
  patterns:
    - "late binding by marker: what is stored is a reference, what is served is the thing — snippet.Expand's mechanism with LoadImageSets' loading strategy"
    - "the marker's spelling in exactly one file, with the import direction that forced it written down"
    - "a picture list resolved where the query already ran: album.Set.Lookup() gives out a block.Lookup, so no second copy of admin.blockImages"
    - "a second source for a block's content must be taught to Empty(), or Clean deletes the block silently"

key-files:
  created:
    - internal/album/expand.go
    - internal/album/expand_test.go
    - internal/public/album_test.go
    - internal/admin/block_album_test.go
  modified:
    - internal/block/block.go
    - internal/block/form.go
    - internal/block/render.go
    - internal/block/block_test.go
    - internal/album/store.go
    - internal/public/pagedata.go
    - internal/public/handler.go
    - internal/bundle/format.go
    - internal/bundle/blocks.go
    - internal/bundle/bundle_test.go
    - internal/admin/page_blocks.go
    - internal/admin/page.go
    - cmd/holzcloud/templates/admin/block_list.html
    - cmd/holzcloud/main.go

key-decisions:
  - "The marker's spelling lives in internal/block, not internal/album, because internal/album already imports internal/block for block.Item and the other direction is an import cycle. Decided by building it, as the plan asked."
  - "The wrapper div is written at save and only its contents are late: the columns and the display are properties of the block, the pictures are the album's."
  - "The marker carries the block's position, so an inline gallery and an album gallery on one page mint distinct fragment ids."
  - "A block carrying both an album and its own items renders the album. Concatenating the two would mint two runs of ids from one block position."
  - "The block's own picture list is folded away behind a <details> when an album is chosen, not hidden: a collapsed details still posts its fields, so nothing is thrown away and un-choosing the album gives the list back."
  - "LoadFor's join requires the media row's website to match the album's as well as binding the album's website, because the foreign key on media_id proves the file exists and never whose it is."

patterns-established:
  - "Late expansion of a database-backed list into stored page HTML — the row 11-PATTERNS.md marked 'No Analog Found'. The mechanism is snippet.Expand's; the loading is media.LoadImageSets'."
  - "A counting gate over a whole-tree total is replaced by an isolation measurement when the tree is shared: removing this plan's work must not change the count."

requirements-completed: []

coverage:
  - id: D1
    description: "Changing an album changes every page that carries it, without those pages being touched — content_html and updated_at byte-for-byte identical, served body different (GAL-03)"
    requirement: "GAL-03"
    verification:
      - kind: integration
        ref: "internal/public/album_test.go#TestChangingAnAlbumChangesEveryPageThatCarriesIt"
        status: pass
      - kind: integration
        ref: "mutation: album expanded at save time -> FAIL 'the served page did not change when the album did'"
        status: pass
    human_judgment: false
  - id: D2
    description: "A gallery block naming an album survives the save that creates it — Empty()/Clean no longer delete it"
    requirement: "GAL-03"
    verification:
      - kind: unit
        ref: "internal/block/block_test.go#TestAlbumBlockSurvivesClean"
        status: pass
      - kind: unit
        ref: "mutation: Empty()'s album line removed -> FAIL 'the block was dropped by the save that created it'"
        status: pass
      - kind: integration
        ref: "internal/admin/block_album_test.go#TestChoosingAnAlbumIsStoredAndComesBackSelected"
        status: pass
    human_judgment: false
  - id: D3
    description: "Only the albums a page names are loaded; a page naming none issues no query at all"
    requirement: "GAL-03"
    verification:
      - kind: unit
        ref: "internal/album/expand_test.go#TestLoadForIssuesNoQueryWhenTheHTMLNamesNoAlbum"
        status: pass
      - kind: integration
        ref: "internal/public/album_test.go#TestPageWithNoAlbumIssuesNoAlbumQuery"
        status: pass
    human_judgment: false
  - id: D4
    description: "The album's pictures are rendered by the same function the inline gallery uses, from the same list type (GAL-07's one mechanism)"
    requirement: "GAL-07"
    verification:
      - kind: unit
        ref: "internal/album/expand_test.go#TestExpandReplacesTheMarker"
        status: pass
      - kind: other
        ref: "gate: GalleryItems( in internal/album/expand.go, comments stripped = 1"
        status: pass
    human_judgment: false
  - id: D5
    description: "An album of another website is not reachable through a marker (GAL-05 on the read path)"
    verification:
      - kind: integration
        ref: "internal/public/album_test.go#TestAlbumFromAnotherWebsiteExpandsToNothing"
        status: pass
      - kind: other
        ref: "gate: every statement touching album_items names website_id = 10/10"
        status: pass
    human_judgment: false
  - id: D6
    description: "The expanded img tags carry a srcset, which is the ordering of the expansion asserted rather than reasoned about"
    verification:
      - kind: integration
        ref: "internal/public/album_test.go#TestExpandedAlbumPicturesGetTheirSrcSet"
        status: pass
      - kind: other
        ref: "gate: snippet.Expand < album.Expand < filterByPlugins < h.responsive in pageContent"
        status: pass
    human_judgment: false
  - id: D7
    description: "The album slug survives the archive round trip"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestRoundTripKeepsBlocks"
        status: pass
    human_judgment: false
  - id: D8
    description: "The editor picks an album from a select on the gallery block, the form says what that does, and the block's own list is not thrown away"
    verification:
      - kind: integration
        ref: "internal/admin/block_album_test.go#TestTheGalleryBlockOffersTheWebsitesAlbums"
        status: pass
      - kind: integration
        ref: "internal/admin/block_album_test.go#TestChoosingAnAlbumIsStoredAndComesBackSelected"
        status: pass
    human_judgment: true
    rationale: "The markup and the round trip are asserted; whether the hint reads as an explanation and whether a folded <details> beside two other selects is legible is a browser judgement. Plan 11-07 owns the browser pass."

duration: 16 min
completed: 2026-09-07

actuals:
  tokens: 104548
  tasks: 3
  commits: 5
---

# Phase 11 Plan 05: The album, bound late — Summary

**A gallery block stores an album's slug and renders to a marker; the pictures
are looked up on every request, so changing an album changes every page that
carries it while those pages' `content_html` and `updated_at` stay byte for byte
what they were.**

## Performance

- **Duration:** 16 min (first commit to last)
- **Started:** 2026-09-07T18:58:15Z
- **Completed:** 2026-09-07T19:14:35Z
- **Tasks:** 3
- **Files created/modified:** 18

## Accomplishments

- **GAL-03 holds, and it is proved rather than claimed.** The property test
  changes the album, touches nothing else, reads `content_html` and `updated_at`
  straight out of the table before and after, finds them identical, and finds the
  served page different. It was watched red against a save-time expansion.
- **The trap the planning documents never named is closed and gated.**
  `Empty()` called a gallery with no items empty and `Clean` dropped it, so an
  album-backed block was deleted by the very save that created it, silently.
  `TestAlbumBlockSurvivesClean` is the regression test and it was watched red.
- **Only what the page names is loaded.** `LoadFor` reads the slugs out of the
  HTML first; a page naming no album issues no query at all, asserted with a
  store that has no database behind it.
- **One renderer, two sources.** The album's pictures go through
  `block.GalleryItems` with the block position the marker carries — the same
  function, the same `[]block.Item`. Nothing in `internal/album` renders a tile
  or a large view.
- **The editor can reach it**, and four rendering tests prove the select is
  drawn, guarded to the gallery, and comes back carrying the choice.

## Task Commits

1. **Task 1 RED** — `d710bd6` (test) — nine failing tests in `internal/block`,
   plus `TestRoundTripKeepsBlocks` carrying the album slug
2. **Task 1 GREEN** — `db66951` (feat) — `AlbumSlug`, the `Empty()`/`Clean` fix,
   the `album` case, the marker, both bundle literals
3. **Task 2 RED** — `439843d` (test) — five public tests and seven unit tests
4. **Task 2 GREEN** — `68ac13b` (feat) — `expand.go`, `LoadFor`,
   `SetAlbumStore`, the call in `pageContent`, `main.go`
5. **Task 3** — `7d0d443` (feat) — the select, the folded own-list, four
   rendering tests

No REFACTOR commit: nothing needed cleaning up.

**Plan metadata:** see the final `docs(11-05)` commit.

## The three picture lists, and why the shop is not being rebuilt

This tree holds **three** lists of pictures, and a reader who finds them without
this paragraph reads GAL-07 as unmet.

1. **`block.Item`** — a JSON array inside the page's `blocks` column
   (`internal/block/block.go`), edited in the block editor.
2. **The album** — `album_items`, from plan 11-02, a website-scoped ordered
   child table.
3. **The shop's product gallery** — `internal/shop/product.go:331` `SetGallery`,
   a join table of media ids, rendered by all eight themes as
   `.product__gallery`.

**GAL-07 asks that the gallery *block* and the *album* agree, and they now do**,
through one list type and one renderer: `album.Store.Items` returns
`[]block.Item` — the very type a gallery block holds — and `album.Expand` hands
it to `block.GalleryItems`, the very function `renderOne`'s gallery arm calls.
There is one rendering of a tile and one of a large view in this repository, and
both sources go through it. That is what "one mechanism" means here.

**It does not ask for the shop to be rebuilt, and rebuilding it would be scope
this phase did not buy** (D-08, `11-CONTEXT.md`). The shop's gallery is a
different contract — a product's own pictures, ordered by a join table, styled by
every theme under a class name this phase does not touch. Nothing in GAL-01…07
asks for it. Measured rather than asserted: `git status --porcelain
internal/shop/` prints **0** lines, and the commits of this plan touch no file
under `internal/shop`.

## The counting gates, measured against the post-change tree

| What | Command | Baseline | Plan predicted | **Measured** | Verdict |
|---|---|---|---|---|---|
| fields on `block.Block` | `python3` over the struct | 15 after 11-04 | 16 | **16** | as predicted |
| fields on `bundle.Block` | `python3` over the struct | 15 after 11-04 | 16 | **16** | as predicted |
| `Album:` in `blocks.go` | comment-stripped count | 0 | 2 | **1** | **diverged — the gate is unsatisfiable, see Deviation 1** |
| `\bAlbum:` / `AlbumSlug:` in `blocks.go` | the same gate, corrected | 0 / 0 | 1 / 1 | **1 1** | the intent, measured |
| files with the marker literal | glob over non-test `.go` | 0 | 1 | **1** (`internal/block/render.go`) | as predicted |
| `case "album":` / `page.Slugify(` in `form.go` | comment-stripped | 0 / 0 | 1 / 1 | **1 1** | as predicted |
| `GalleryItems(` in `expand.go` | comment-stripped | — | > 0 | **1** | as predicted |
| statements touching `album_items` / naming `website_id` | comment-stripped | 9 / 9 after 11-02 | equal, > 0 | **10 10** | as predicted |
| `blockImages\|mediaStore.GetByID(` in `pagedata.go` | comment-stripped | **1** | 0 | **1** | **diverged — the baseline was never 0, see Deviation 2** |
| `SetAlbumStore` in `main.go` | `grep -c` | 1 after 11-03 | 2 | **2** | as predicted |
| `album` in `block_list.html` | `grep -c` | 0 | > 0 | **2** | as predicted |
| files using `BeginTx` | `grep -rln … \| grep -v _test \| wc -l` | 15 after 11-02 | 15 | **15** | as predicted |
| migrations | `ls …/*.sql \| wc -l` | 50 after 11-02 | 50 | **50** | as predicted |
| `internal/shop` files changed | `git status --porcelain \| wc -l` | 0 | 0 | **0** | as predicted |
| strings in source | `go run ./tools/i18n` | 1306 / 29 offen after 11-04 | measured, not predicted | **1310 / 33 offen; isolated +4** | measured |

### The i18n row, measured by isolation rather than by a whole-tree total

`11-CONTEXT.md`'s insights say a whole-tree count must not be asserted by a plan
that shares its tree, and this tree was shared: a separate audit workflow was
committing to `main` throughout. So the honest form was used — *removing this
plan's work must not change the count.*

| Tree | Strings | Offen |
|---|---|---|
| this plan's `block_list.html` reverted to `HEAD`'s | 1306 | 29 |
| this plan's work present | **1310** | **33** |

**Exactly +4, all four in `block_list.html`**, and the pre-plan number matches
11-04's recorded end state (1306 / 29) to the digit:

- `Bilder aus` — the select's label
- `– der eigenen Liste unten –` — the empty option
- the hint sentence, `Aus einem Album kommen die Bilder des Albums, …`
- the `<details>` summary, `Eigene Bilderliste — bleibt gespeichert, …`

`page_blocks.go` adds **zero**: its only new string goes to `slog.Error`, which
`tools/i18n` does not collect, and the `AlbumChoice` values are album names an
operator typed.

**The catalogues were not filled.** `internal/i18n/locales/*.json` is not in this
plan's `files_modified` and plan 11-07 owns them. The standing `33 offen` is
expected, not a regression; **4 of those 33 are this plan's**, on top of 11-04's
3 and 11-03's 26.

## Mutation evidence

A gate that was never seen red is a claim, not coverage. Both of the plan's
named silent failures were induced and observed.

### 1. The `Empty()`/`Clean` fix — the block deleted by the save that creates it

Mutation: the album arm removed from `Empty()`'s gallery case. Literal output:

```
=== RUN   TestAlbumBlockSurvivesClean
    block_test.go:1152: the block was dropped by the save that created it
--- FAIL: TestAlbumBlockSurvivesClean (0.00s)
=== RUN   TestGalleryWithNeitherItemsNorAlbumIsStillDropped
--- PASS: TestGalleryWithNeitherItemsNorAlbumIsStillDropped (0.00s)
```

The second line matters as much as the first: the other half stayed green, so
the fix did not turn `Empty()` into a no-op. Restored, re-run, `--- PASS`.

Worth recording: **`TestRoundTripKeepsBlocks` stayed green under this mutation**
(`ok internal/bundle 0.501s`), because its gallery block also carries items.
Only the dedicated regression test catches this.

### 2. GAL-03 — the album expanded at save time

Mutation: the fixture renders the page the way a save-time expansion would, the
album's pictures baked straight into `content_html`, no marker. Stage 1, the
precondition guard:

```
    album_test.go:158: the stored page does not carry the marker — the pictures were
    baked in at save and GAL-03 cannot hold:
        <div class="hc-block hc-galerie hc-spalten-3"><figure class="hc-galerie__bild">
        <a class="hc-galerie__oeffnen" href="#hc-b1-p1"><img src="/media/1/schrank.jpg" …
--- FAIL: TestChangingAnAlbumChangesEveryPageThatCarriesIt (0.07s)
```

That only proves the precondition fires, so the mutation was taken one step
further: the two early guards were silenced, leaving nothing but the property
assertions at steps 4 and 5.

```
    album_test.go:180: the served page did not change when the album did — the
    expansion is not happening at request time, and GAL-03 is false
--- FAIL: TestChangingAnAlbumChangesEveryPageThatCarriesIt (0.08s)
```

**The property itself catches it**, not merely the marker check. Restored,
re-run, `--- PASS`.

## Gate output, literal

```
$ go build ./... && go vet ./internal/block/ ./internal/bundle/ && gofmt -l …    exit=0, no paths
$ go test ./internal/block/ -run 'Album|Clean|Gallery' -v            17 --- PASS, 0 --- FAIL
$ go test ./internal/block/ -run TestAlbumBlockSurvivesClean -v | grep -c PASS        1
$ python3 … Album: / Display: in blocks.go                                         1 2
$ python3 … \bAlbum: / AlbumSlug: in blocks.go                                     1 1
$ python3 … case "album": / page.Slugify( in form.go                               1 1
$ python3 … marker literal over internal/**/*.go       1  ['internal/block/render.go']
$ go test ./internal/bundle/ -run TestRoundTripKeepsBlocks -v      --- PASS (0.09s)

$ go build ./... && go vet ./internal/album/ ./internal/public/ && gofmt -l …    exit=0
$ go test ./internal/album/ ./internal/public/                       ok, ok — no FAIL
$ go test ./internal/public/ -run TestChangingAnAlbum… -v | grep -c PASS              1
$ python3 … pageContent order    snippet.Expand 159 < album.Expand 975
                                 < filterByPlugins 1213 < h.responsive 1385
$ python3 … GalleryItems( in expand.go                                               1
$ python3 … album_items statements / naming website_id                            10 10
$ python3 … blockImages|mediaStore.GetByID( in pagedata.go                            1
$ grep -c 'SetAlbumStore' cmd/holzcloud/main.go                                      2

$ python3 … block.Block fields                                                      16
$ git status --porcelain internal/shop/ | wc -l                                      0
$ grep -c 'album' cmd/holzcloud/templates/admin/block_list.html                       2
$ grep -rln BeginTx internal/ --include='*.go' | grep -v _test | wc -l               15
$ ls internal/db/migrations/*.sql | wc -l                                            50
$ go run ./tools/i18n | head -1                            1310 Zeichenketten (isolated +4)

$ go build ./... && go vet ./... && gofmt -l . && go test ./...
    build OK, vet OK, gofmt printed nothing, test exit 0, 44 packages ok, 0 FAIL
```

## Decisions Made

**Which package owns the marker, decided by building it.** `internal/album`
imports `internal/block` for `block.Item` (`album.go:6`, `store.go:56`), so
`internal/block` importing `internal/album` is an import cycle — confirmed, not
assumed. The plan's fallback applies: **the prefix, the pattern, the writer and
the three readers live in `internal/block/render.go`**, and
`internal/album/expand.go` calls them. One file either way, which was the point.
`internal/block` gained an import of `internal/page` for `Slugify`; `go list
-deps internal/page` shows `auth` and `db` only, so no cycle there either.

**The wrapper at save, the contents late.** The columns class and 11-04's
display modifier are written into `content_html` at save, because they are
properties of the block and known then; only the pictures belong to the album.
A marker carrying them would be a second encoding of what the class attribute
already says.

**Both sources at once: the album wins.** Not a state the editor can produce,
but a hand-edited archive can. Concatenating the two would mint two runs of
fragment ids from one block position.

**The block's own list is folded, not hidden.** The plan asked for one of two
answers and for the choice to be stated. Hiding the rows would have made the
save silently empty the block's own list, so instead they go inside a
`<details>` whenever an album is chosen: a collapsed `<details>` still posts its
fields, so nothing is lost, the form never shows two lists side by side, and an
editor who un-chooses the album gets their list back. No script, in a stack that
permits none. The hint says which of the two lists is shown.

**`LoadFor`'s second website condition is not redundant.** The join binds
`a.website_id = $1` *and* requires `m.website_id = a.website_id`. The first is
GAL-05. The second matters because an `album_items` row need not have been
written through `AddItem`: the foreign key on `media_id` proves the file exists
and never whose it is. The join makes the picture's website a fact of this query
rather than a property of its history.

## Deviations from Plan

### 1. [plan defect — a gate that cannot be satisfied] `Album:` in `blocks.go` counts 1, not 2

- **Found during:** Task 1, running the gate.
- **Issue:** the gate is
  `print(len(re.findall(r'Album:',s)), len(re.findall(r'Display:',s)))` with
  `<fails_when>` "either printed number is not 2". `Display:` counts 2 because
  `block.Block.Display` and `bundle.Block.Display` share a name, so the field
  appears as `Display: b.Display` in both literals. The album's two names differ
  by the plan's own requirement: `block.Block.AlbumSlug` (acceptance criterion 1)
  and `bundle.Block.Album` (acceptance criterion 9). The literals therefore read
  `Album: b.AlbumSlug` on the way out and `AlbumSlug: b.Album` on the way in, and
  `Album:` cannot match inside `AlbumSlug:` — the character after `Album` is `S`.
  **No implementation satisfying both acceptance criteria can make this gate
  print 2.**
- **Fix:** the substance was implemented — the field is in the manifest struct
  and in **both** struct literals, which is what §A.4's rule asks and what stops
  the value being lost silently. The gate was replaced with one that measures
  the same intent on this pair of names:
  `len(re.findall(r'\bAlbum:',s))` and `len(re.findall(r'AlbumSlug:',s))`,
  which must both be 1 — one literal on the way out, one on the way in.
- **Measured:** `1 1`. Proved end to end by `TestRoundTripKeepsBlocks`, which
  now asserts `angekommen[3].AlbumSlug == "moebel"` after a real export/import.
- **Files:** `internal/bundle/format.go`, `internal/bundle/blocks.go`,
  `internal/bundle/bundle_test.go`
- **Commit:** `db66951`, `d710bd6`

### 2. [plan defect — a baseline that was never 0] `blockImages|mediaStore.GetByID(` in `pagedata.go`

- **Found during:** Task 2, running the gate.
- **Issue:** the gate requires 0 and the tree reads 1. The hit is
  `internal/public/pagedata.go:242`, inside **`fieldImages`** — the resolver for
  a page's own *picture fields*, returning a `field.Image` through a
  `field.Lookup`. It has nothing to do with blocks and it predates this phase.
- **Evidence, as an isolation measurement rather than an argument:**

  | Tree | Count |
  |---|---|
  | pre-plan (`git show HEAD~2:internal/public/pagedata.go`, untouched by 11-05) | **1** |
  | post-plan | **1** |

  **This plan adds zero.** The gate's intent — the album's picture lookup comes
  from `album.Set.Lookup()`, built where the query already ran, and not from a
  second copy of `admin.blockImages` — holds exactly: `internal/public` declares
  no picture-lookup function for the album, and `Set.Lookup()` closes over a map
  the website-scoped query filled.
- **Fix:** none needed in code. Recorded rather than adjusted. This is the same
  class of defect `11-CONTEXT.md` names twice for this phase and 11-02 and 11-04
  each recorded once: a whole-tree total asserted by a plan that does not own the
  whole tree. The honest form is the isolation measurement above.
- **Files:** none
- **Commit:** —

### 3. [Rule 2 — missing critical functionality] Four rendering tests for the album select

- **Found during:** Task 3.
- **Issue:** the only gate the plan gives the new control is
  `grep -c 'album' block_list.html != 0`, and a grep cannot tell a rendered
  `<select>` from a comment that mentions one. Plan 11-04 recorded the identical
  gap for its own display select and could not close it, because the editor form
  belongs to `internal/admin`, which was another plan's territory in that wave.
  A `case "album":` in `setBlockField` with no reachable control is a feature
  nobody can use, and nothing would have said so.
- **Fix:** `internal/admin/block_album_test.go`, four tests through the real
  handler and the real on-disk templates: the select is drawn on a gallery with
  the website's albums as options; it is **not** drawn on a card row; a website
  with no album gets no select rather than an empty one; and the whole way
  through — posted, slugified, stored, rendered as a marker into `content_html`,
  and **back into the form still selected**. The last is what nothing else
  asserts, and a select that loses its choice on every redraw looks right until
  somebody presses another button in the editor.
- **Files:** `internal/admin/block_album_test.go` (new; not in the plan's
  `files_modified`)
- **Verification:** all four `--- PASS`.
- **Commit:** `7d0d443`

### 4. [documented choice] Three tests beyond the eleven the plan names

`internal/block`: `TestCleanDropsAnAlbumSlugFromABlockThatIsNotAGallery` (the
acceptance criterion "Clean does not carry an album slug on a block type that is
not a gallery" had no test named for it), `TestTheAlbumMarkerIsPlainTextInsideAnElement`
(the plan asks for the `MakeResponsive` survival to be "a property to state and
to test rather than to assume") and `TestAlbumMarkerReaderFindsWhatTheWriterWrote`
(the writer and the readers now live in one package, so the round trip between
them is testable without a database). `internal/album`:
`TestExpandUsesTheInjectedTranslator`. All eleven named tests exist, in English,
one property each.

### 5. [not a deviation — recorded because it shaped the measurements] A parallel workflow committed to this tree throughout

Thirteen commits landed on `main` between this plan's first and last, and eight
of them are not this plan's — an audit workflow closing website-isolation and
field defects (`ff96569`, `df8eafb`, `754a123`, `ed56959`, `d937e2d`,
`24d4c7b`, `68043c4`, `4759907`). Two of its files (`internal/csvimport/row.go`,
`internal/field/field.go`) sat modified in the working tree while Task 1 was
committed and were deliberately **not** staged; every commit of this plan was
staged file by file and `git show --stat` confirms each touches only this plan's
files. This is why every whole-tree number above is reported as an isolation
measurement.

---

**Total deviations:** 2 plan defects recorded with evidence and not worked
around, 1 auto-added under Rule 2, 2 documented choices.
**Impact on plan:** no scope creep. The two plan defects are gates that could
not be satisfied as written; in both cases the *substance* the gate exists to
protect was implemented and measured, and the corrected measurement is printed
beside the original.

## What the plan got wrong about the tree

Beyond the two gates above:

1. **`internal/album/expand.go` cannot own the marker.** The plan's
   `must_haves.artifacts` and its `key_links` both place the marker's pattern
   and writer there, and its own action text anticipates the cycle and gives the
   fallback. The fallback is what applies: `internal/album` already imports
   `internal/block`. The `key_links` row "from `internal/block/render.go` to
   `internal/album/expand.go` via the marker" therefore runs the other way; the
   row "from `expand.go` to `render.go` via `GalleryItems`" holds exactly.
2. **`internal/public/pagedata.go:287-297` `h.responsive`** is at `:287` as the
   plan says, but it is **called from inside the `tmpl.PageContent` literal** at
   the `ContentHTML` line, not as a statement of its own. The ordering gate still
   reads correctly, but a naive reading of `pageContent` looking for four
   statements finds three.
3. **A comment naming `h.responsive` inside `pageContent` breaks the ordering
   gate**, because the gate does a substring `find` over the whole function
   including its comments. The first draft of the placement comment said "before
   `h.responsive`" and the gate printed `h.responsive 447`, i.e. *before*
   `filterByPlugins`. The wording was changed to "before the responsive rewrite
   at the end of this function". Worth knowing: a gate that measures source
   positions measures prose too.
4. **`internal/media`'s `columns` and `scan` are unexported**, so `LoadFor`
   cannot reuse the media projection and selects the ten columns it needs by
   hand. The six values a `block.Image` is built from still come from
   `media.Media`'s own methods (`URL`, `FocusCSS`, `IsVideo`), so no media rule
   is computed twice — that is what `imageOf` is for.

## Issues Encountered

None. Every gate in the plan was run and its literal output is above.

## Known Stubs

None. No `TODO`, `FIXME`, placeholder value, `t.Skip` or unrun `<verify>` was
added by any of the five commits. The `placeholder=` attributes in
`block_list.html` are HTML input placeholders and predate this plan.

## Threat Flags

None new. The register's dispositions are satisfied:

- **T-11-21** (markup in the album value) — `setBlockField` stores
  `page.Slugify(value)`, gated at `1 1` and asserted by
  `TestAlbumValueIsSlugifiedOnTheWayIn`, which posts
  `Möbel "2025" <script>` and gets `moebel-2025-script`.
- **T-11-22** (another website's album) — `LoadFor` binds `a.website_id` and
  requires `m.website_id = a.website_id`; gated at `10 10` and asserted by
  `TestAlbumFromAnotherWebsiteExpandsToNothing`, which gives both websites an
  album of the *same slug* so the test proves something.
- **T-11-23** (an album expanded on every request) — `MaxItems = 120` bounds the
  album, `LoadFor` issues one query for all named albums, and a page naming none
  issues zero, asserted twice.
- **T-11-24** (a marker on a visitor's page) — a missing album expands to the
  empty string;
  `TestAlbumMarkerForADeletedAlbumLeavesNoSyntaxOnThePage` and
  `TestExpandDropsAnUnknownSlug`.
- **T-11-25** (a failing album taking the page) — the error is logged with
  `slog.Error` and the body returned unexpanded.
- **T-11-26** (the expansion after the responsive rewrite) — gated by the
  position check and asserted by `TestExpandedAlbumPicturesGetTheirSrcSet`.
- **T-11-SC** — `go.mod` untouched, no package-manager install ran. Every new
  import is inside this repository.

## Not verified here

**No browser pass.** Plan 11-07 owns it. Three things this plan does not claim:

1. **That the folded `<details>` reads well** beside the columns and display
   selects. It carries `form-hint text-muted` on its summary and no rule of its
   own in `admin.css` — `admin.css` is not in this plan's `files_modified`, and
   a bare `<details>` is legible without one. Whether it *looks* right is the
   browser pass's question.
2. **That the four new German strings read well.** They are not translated;
   `internal/i18n/locales/*.json` belongs to plan 11-07.
3. **The render cost of a 120-picture album on a busy page.** The query is one
   and the cap is enforced, but no measurement was taken.

## Next Phase Readiness

- **Plan 11-06** has what it needs: `bundle.Block.Album` carries the slug through
  both halves of `blocks.go` today, and the field's own comment names 11-06 as
  the plan that replaces the pass-through with the name translation. `Rename`
  exists on the store and does not move the slug, which is what
  `TestAlbumRoundTripAfterRename` will rest on.
- **Plan 11-07** inherits **33 offen**, of which 4 are this plan's, and the
  browser pass listed above.
- **No blockers.**

---
*Phase: 11-galerie*
*Completed: 2026-09-07*

## Self-Check: PASSED

```
FOUND: internal/album/expand.go
FOUND: internal/album/expand_test.go
FOUND: internal/public/album_test.go
FOUND: internal/admin/block_album_test.go
FOUND: internal/block/block.go   form.go   render.go   block_test.go
FOUND: internal/album/store.go
FOUND: internal/public/pagedata.go   handler.go
FOUND: internal/bundle/format.go   blocks.go   bundle_test.go
FOUND: internal/admin/page_blocks.go   page.go
FOUND: cmd/holzcloud/templates/admin/block_list.html
FOUND: cmd/holzcloud/main.go
FOUND commit: d710bd6  test(11-05): failing tests for a gallery block …
FOUND commit: db66951  feat(11-05): a gallery block names an album …
FOUND commit: 439843d  test(11-05): failing tests for the request-time expansion
FOUND commit: 68ac13b  feat(11-05): expand a page's albums at request time …
FOUND commit: 7d0d443  feat(11-05): the editor picks an album …
```

Each commit touches only this plan's files, confirmed by `git show --stat`:
`d710bd6` two files, `db66951` five, `439843d` two, `68ac13b` five, `7d0d443`
four. Nothing of the parallel audit workflow's, nothing of the developer's.

## TDD Gate Compliance

Tasks 1 and 2 carried `tdd="true"` and both gate sequences are in the log in
order: `test(11-05)` at `d710bd6` (RED — the suite failed to build against a
`Block` with no `AlbumSlug` and no marker functions, while `go build ./...`
stayed green so the parallel workflow's tree was not broken), then
`feat(11-05)` at `db66951` (GREEN); `test(11-05)` at `439843d` (RED — no `Set`,
no `SetAlbumStore`), then `feat(11-05)` at `68ac13b` (GREEN). No REFACTOR commit
in either: nothing needed cleaning up.

**Fail-fast note.** No test passed unexpectedly in either RED phase — both RED
commits failed to compile, which is the strongest form of red available when the
missing thing is a field and a function.

## REQUIREMENTS.md

**Not marked.** `requirements.ready-ids` reports `0/2 requirement(s) ready`:
GAL-03 is also declared by 11-07 and GAL-07 by 11-06, and neither has a SUMMARY
yet. The shared-ID gate holds them until the last plan declaring them finishes,
which is correct — this plan proves GAL-03 for the block and the album, and
11-07's browser pass is the other half of what the requirement asks.
