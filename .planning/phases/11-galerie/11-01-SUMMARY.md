---
phase: 11-galerie
plan: 01
subsystem: block renderer, block stylesheet, template specification
tags: [lightbox, target, gallery, i18n, tmplspec, tracer]
status: complete

requires: []
provides:
  - "block.GalleryItems — the one gallery renderer, exported, called by the
    gallery arm today and by plan 11-05's album expansion tomorrow"
  - "block.Set.T — a translator injected on the Set, the render-time half of
    the i18n work, next to Set.Date"
  - "Render/renderOne/renderOwn carrying the block's position, which is what
    makes a fragment id unique per block as well as per item"
  - "the :target rules in cmd/holzcloud/assets/bausteine.css"
  - "/assets/bausteine.css named in TEMPLATE-SPEC.md and guarded by two tests"
affects:
  - "internal/album (11-05) — calls block.GalleryItems rather than writing a
    second renderer"
  - "internal/block (11-04) — the slideshow's accessible name goes through the
    same Set.T that exists now"
  - "internal/i18n/locales/*.json — three new source strings are open"

tech-stack:
  added: []
  patterns:
    - ":target as the whole lightbox mechanism; no script on any path"
    - "a language-aware hook injected on block.Set as a function, not a locale
      — the shape Set.Date already had"
    - "a guard test over an instruction in TEMPLATE-SPEC.md, which reflection
      cannot hold"

key-files:
  created: []
  modified:
    - internal/block/render.go
    - internal/block/block.go
    - internal/block/block_test.go
    - internal/admin/page_blocks.go
    - internal/bundle/import.go
    - cmd/holzcloud/assets/bausteine.css
    - internal/tmplspec/TEMPLATE-SPEC.md
    - internal/tmplspec/spec_test.go

decisions:
  - "renderOne gains `s Set` as well as `at int`. The plan and 11-PATTERNS §A.2
    both wrote the new signature as `renderOne(b, at, blk, look, md)` — but
    renderOne never took a Set, and the three control names have to be
    translated through one. Passing `s` is the same shape renderOwn has always
    had and it is what the injected translator requires."
  - "The step links point at the neighbouring *resolved* picture, not at
    number±1. The ids stay keyed to the item's own loop index, so a media id
    that no longer resolves does not renumber the pictures after it — but a
    link to a number that was skipped would be a link to an id that does not
    exist."
  - "The close target is the constant `hc-zu`, under the same hc- prefix as
    every id this file mints."
  - "The three German control names are `Vorheriges Bild`, `Nächstes Bild` and
    `Grossansicht schliessen` — new literals, not the existing `Weiter` and
    `Zurück`, which en.json translates as `Continue` and `Back`."
  - "The catalogues are NOT filled here: internal/i18n/locales/*.json is not in
    this plan's files_modified and a second executor was writing in the same
    tree. The number is recorded below for 11-07."

metrics:
  duration: "~35 min"
  completed: 2026-09-07

actuals:
  tokens: 9128
  tasks: 3
  commits: 5
---

# Phase 11 Plan 01: The lightbox tracer — Summary

A gallery picture opens large, steps to its neighbours and closes again through
`:target` and five ordinary anchors, with no script anywhere; the enlargement is
a `<figure>` sibling in the flow, so a page with the stylesheet blocked is still
a readable list of pictures with working anchors.

## What was built

**Task 1 — the renderer learns where it is** (`c432bba`, red at `9a0633e`).
`Render` iterates `for i, blk := range blocks` and hands the position to both
arms. `GalleryItems(at, items, look, t)` is exported and is the only place a
gallery is rendered: tiles first, then large views, ids `hc-b{at+1}-p{j+1}`.
A doc comment states that those ids are minted at save into
`pages.content_html` and can drift after a block reorder. The large view calls
`imgTag(img, alt, "100vw", false)` — no variant named, no path built, no
`srcset`, no inline `object-position`. Previous and next are absent at the two
ends rather than disabled; close points at `#hc-zu`, which names nothing.
`Set.T` is the injected translator, nil-safe through `Set.text`, assigned in
exactly the two places `Set.Date` is assigned.

**Task 2 — the reveal that costs no scroll width** (`a3b49c9`).
73 appended lines, nothing above them touched. `.hc-galerie__gross` is
`display: none`; `.hc-galerie__gross:target` returns it to the flow across
`grid-column: 1 / -1`. The comment names `.planning/STATE.md:209` and what the
other choice cost the `weide` theme. No `position: fixed`, no
`position: absolute`, no `visibility` anywhere in the file.

**Task 3 — the specification requires the stylesheet** (`be00a7d`, red at
`97cffbf`). `/assets/bausteine.css` now appears in §4's asset table with its
reason and the same-origin note, in §3's layout example and in §11's complete
minimal template — above `/t/style.css` in both, with the sentence saying why
the order is what lets a theme win. Two guards:
`TestSpecRequiresTheBlockStylesheet` and
`TestEveryShippedThemeLinksTheBlockStylesheet`, the second of which fails
rather than passes when it finds no theme at all.

## Gate output, literal

| Gate | Expected | Measured |
|---|---|---|
| `go build ./... && go vet … && gofmt -l …` | clean | clean, `gofmt -l` printed nothing |
| `go test ./internal/block/ -run Gallery -v` | no FAIL, tests ran | `7 --- PASS`, `ok … 0.298s` |
| `go test ./...` | exit 0 | exit 0, 44 `ok`, no `FAIL` |
| `grep -c 'func GalleryItems(' render.go` | 1 | `1` |
| non-comment `GalleryItems` count | ≥ 2 | `2` |
| `grep -c 'set.T = '` ×2 | 1 each | `page_blocks.go:1`, `import.go:1` |
| `i18n.N(` in render.go, comments stripped | 3 | `3` |
| `i18n.N("Weiter"\|"Zurück")` | 0 | `0` |
| `TestLightboxControlsGoThroughTheInjectedTranslator` `--- PASS` | 1 | `1` |
| `srcset` in render.go, comments stripped | 0 | `0` |
| `position: fixed\|absolute` in bausteine.css | 0 | `0` |
| `visibility:` in bausteine.css | 0 | `0` |
| `:target` / `display: none` in bausteine.css | both > 0 | `1 1` |
| deleted lines in bausteine.css | 0 | `0` |
| `go test ./internal/template/ ./internal/tmplmgr/` | pass | both `ok` |
| `/assets/bausteine.css` in TEMPLATE-SPEC.md | ≥ 3 | `4` |
| themes linking it | 8 | `8` |
| `go test ./internal/tmplspec/ -v` | no FAIL | `0 --- FAIL`, 8 `--- PASS` |
| two spec guards `--- PASS` | 2 | `2` |

## Divergences, with direction and cause

**1. `renderOne` gained `s Set` as well as `at int` — the plan was wrong about
the tree here.** The plan's action and 11-PATTERNS §A.2 both spell the new
signature `renderOne(b *strings.Builder, at int, blk Block, look Lookup, md
Markdown)`. But `renderOne` never received a `Set` at all — only `renderOwn`
did — and the three control names cannot be translated without one. Adding `s`
is the smallest change that makes the injected translator reachable from the
gallery arm, and it makes the two arms symmetrical. No acceptance criterion is
weakened: `at` is present, passed to both arms, and used only by the gallery.

**2. `-run Gallery` runs 7 of the 10 new tests, not 10.** Direction: three
fewer than a reader of the test list would expect. Cause: `-run` is an
unanchored regexp over the function name, and
`TestTwoGalleriesOnOnePageMintDistinctIds` spells it "Galleries",
`TestSetWithoutTranslatorKeepsTheGermanSource` and
`TestLightboxControlsGoThroughTheInjectedTranslator` do not carry the word at
all. All three run and pass under `go test ./internal/block/` and under
`go test ./...`; the gate's own condition — tests ran, none failed — is met.
The plan's test list is the honest count, not the `-run` count.

**3. `/assets/bausteine.css` appears 4 times in TEMPLATE-SPEC.md, not 3.**
Direction: one above the gate's floor of 3. Cause: §4 carries the path twice on
purpose — once as the table row and once in the sentence that says it is
required for any theme rendering block content. §3's example and §11's minimal
template are the other two. The gate is `< 3 fails`, so this is green.

**4. Step links point at the neighbouring resolved picture.** The plan's
behaviour list asks both that "the item loop's own index is what numbers the
picture" and that first/last have no previous/next. With a gallery whose middle
media id no longer resolves, those two are only compatible if the ids stay keyed
to the item index while the links walk the *rendered* list. That is what
`GalleryItems` does, and the reason is in a comment.

**5. A transient `internal/album` build failure was observed and NOT touched.**
Mid-run, `go test ./...` failed with `internal/album/store_test.go: undefined:
ErrDuplicateName` — the parallel 11-02 executor's RED state in the shared tree.
Outside this plan's `files_modified`; left alone. It cleared on its own once
11-02 committed `1ad5d11`, and the final `go test ./...` exits 0 with 44 `ok`
and no `FAIL`.

## Hand-off to 11-07 — a number that plan needs and does not yet have

`go run ./tools/i18n` against the finished tree:

```
1280 Zeichenketten im Quelltext
en.json      1277 übersetzt, 3 offen, 0 verwaist
es.json      1277 übersetzt, 3 offen, 0 verwaist
fr.json      1277 übersetzt, 3 offen, 0 verwaist
it.json      1277 übersetzt, 3 offen, 0 verwaist
```

1277 → 1280 source strings, exactly +3, `0 verwaist` in all four. That is the
collector proving the three literals are visible — the half that marking buys.
The other half is already proved by
`TestLightboxControlsGoThroughTheInjectedTranslator`, which renders the same
gallery twice and fails if a German literal survives a translator that maps it
away.

**The four catalogues are deliberately not filled here.**
`internal/i18n/locales/*.json` is not in this plan's `files_modified`, and a
second executor was writing in the same tree. Whoever runs
`go run ./tools/i18n -write` must translate three keys, not one:
`Vorheriges Bild`, `Nächstes Bild`, `Grossansicht schliessen`. Until then the
standing gate reads `3 offen` in four catalogues and that is the expected
number, not a regression.

## What the plan got wrong about the tree

- `renderOne` has no `Set` parameter (divergence 1). Both the plan and
  11-PATTERNS §A.2 imply it can reach a translator without one; it cannot.
- `internal/block` already imported `internal/i18n` — `block.go` calls
  `i18n.N` nine times. The plan says the package "gains one import"; what
  actually gains it is `render.go`, and the package-level claim in the plan's
  acceptance criteria was already true before this commit.
- 11-PLAN-CHECK's B3 and B4 were both already corrected in the plan file as
  handed to this executor: the action carries the true premise, and
  `TestLightboxControlsGoThroughTheInjectedTranslator` is a gate in `<verify>`.
  Note that the test does **not** appear in the action's named test list — it
  was added from the `<verify>` block. Anyone reading only the action would
  write nine tests and miss the one that matters.
- W4's warning was honoured: neither spec guard uses `t.Run`, so the
  `--- PASS` count is exactly 2 and cannot over-count.

## Known Stubs

None. No `TODO`, `FIXME`, placeholder value or skipped test was added by any of
the five commits.

## Threat Flags

None. `T-11-01` and `T-11-02` are satisfied as planned — the large view escapes
its caption with the same `html.EscapeString` the tile uses and renders the
picture through the same `imgTag`, and every fragment id is `fmt.Sprintf` over
two integers and never over editor input. `T-11-05` is held by
`TestGalleryCloseTargetsNothingOnThePage`. No new network surface, no new file
access, no schema change, no dependency: `go.mod` is untouched.

## Not verified here

No browser pass. Plan 11-07 owns it, after the code-review fix round — the
stylesheet-blocked rendering, the two-galleries page, the keyboard path and the
non-German website all belong to that step.

## Self-Check: PASSED

- `internal/block/render.go` — FOUND
- `internal/block/block.go` — FOUND
- `internal/block/block_test.go` — FOUND
- `internal/admin/page_blocks.go` — FOUND
- `internal/bundle/import.go` — FOUND
- `cmd/holzcloud/assets/bausteine.css` — FOUND
- `internal/tmplspec/TEMPLATE-SPEC.md` — FOUND
- `internal/tmplspec/spec_test.go` — FOUND
- commits `9a0633e`, `c432bba`, `a3b49c9`, `97cffbf`, `be00a7d` — all FOUND in
  `git log`
