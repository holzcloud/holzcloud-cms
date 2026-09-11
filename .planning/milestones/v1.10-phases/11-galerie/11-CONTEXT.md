# Phase 11: Galerie - Context

**Gathered:** 2026-09-06
**Status:** Ready for planning

> **The developer asked for the whole milestone to be carried out
> autonomously**, and on 2026-09-06 also decided that this open-source
> project's code is **English** — identifiers, comments, test names, commit
> messages. **Everything this phase writes is English from the first line.**
> `.planning/GLOSSARY.md` is binding, including its rule that a German word
> *stored* in the database is a value and not an identifier.
>
> Every decision below is Claude's, taken against the tree, and each names the
> evidence it rests on.

<domain>
## Phase Boundary

Everything that happens **after** the picture grid. The grid exists and is not
being rebuilt.

Requirements: GAL-01 … GAL-07.

**Not in this phase:** richer layouts — masonry columns and justified rows were
offered and declined on 2026-09-05. The grid stays as it is.

</domain>

<decisions>
## Implementation Decisions

### The finding that shapes the whole phase

- **D-01: a `:target` lightbox without its stylesheet is not ugly, it is
  broken — and the specification never tells a theme author to load that
  stylesheet.** Measured: all eight shipped themes link
  `/assets/bausteine.css` from their `layout.html`, and
  `internal/tmplspec/TEMPLATE-SPEC.md` **mentions it nowhere**. A theme written
  by following the specification therefore renders a gallery unstyled today —
  which is merely ugly, because a grid without `grid-template-columns` is still
  a list of pictures.

  A lightbox is different in kind. `:target` **is** the mechanism: the large
  view is hidden by CSS and revealed by the fragment. Without the stylesheet it
  is either always visible — every picture rendered twice, at full size, on
  every page — or the anchor does nothing at all. **The feature does not
  degrade, it fails.**

  Two answers, and this phase ships both:
  1. **TEMPLATE-SPEC documents `/assets/bausteine.css` as required** for any
     theme that renders block content, with the reason. This closes a gap that
     predates this phase.
  2. **The markup is written so that the unstyled case is harmless**: the large
     view must read as "the same picture again, with its caption" and not as a
     broken overlay. That is a constraint on the markup, not a stylesheet
     detail, and it belongs in the plan as an acceptance criterion — verified by
     loading a gallery page with the stylesheet blocked.

### The lightbox

- **D-02: `:target`, never `<dialog>`.** `dialog.showModal()` is JavaScript;
  CLAUDE.md permits htmx only, as enhancement, and
  `internal/tmplmgr/script.go` rejects the scripted version in an uploaded
  theme. `:target` costs one anchor per picture and one rule, and the browser's
  back button works for free because the state is in the URL.
- **D-03: the fragment id must be unique per block *and* per item.** A page may
  carry two galleries; `#bild-1` in both is one id twice, and the browser jumps
  to whichever comes first. The id is minted from the block's position in the
  page plus the item's index. `internal/block/render.go` knows both.
- **D-04: next and previous are ordinary `<a href="#…">` links to sibling ids**
  (GAL-02), not buttons. Rendered server-side in the same pass, so the last
  picture's "next" points at the first — or is absent. **Absent**: a wrap-around
  is a surprise in a list of four holiday photos, and an absent control is the
  same answer the row-stepper gives in Phase 9 (IMP-08 adjacency).
- **D-05: the close control is a link to a fragment that matches nothing**,
  which returns `:target` to no match. It is not `href="#"` — that scrolls to
  the top and adds a history entry the back button then has to walk back
  through.

### The album

- **D-06: an album is a table, and its model is `menus`, not a field.**
  `internal/db/migrations/00005_templates_menus_media.sql:22-34` is the shape: a
  parent row scoped to `website_id … ON DELETE CASCADE`, and an ordered child
  table. An album is exactly that with pictures instead of links. **Migration
  `00050`** — 49 exist, `00049_csv_imports.sql` is Phase 9's and is released.
- **D-07: an album belongs to exactly one website** (GAL-05), enforced by the
  same cascade every other resource uses, and every read is scoped by
  `website_id`. There is no cross-website album, the same way there is no
  cross-website menu.
- **D-08: GAL-07's "one mechanism" means the gallery block and the album, not a
  third rewrite of the shop.** Measured: this tree already holds **three**
  picture lists — `block.Item` (`block.go:220`, a JSON array inside the page's
  `blocks` column), the shop's product gallery (`shop/product.go:331` `SetGallery`,
  a join table of media ids), and now an album. The requirement asks that the
  block and the album agree; it does not ask for the shop to be rebuilt, and
  rebuilding it would be scope this phase did not buy. **Say so in the plan**, or
  a later reader reads GAL-07 as broken.
- **D-09: an album survives the bundle round trip and the proving test renames
  it first** (GAL-04). This is the criterion that is easy to declare and easy to
  get wrong: `internal/bundle` carries a reusable thing **by its name** and
  re-derives the address on import (`format.go:163-170`, `import.go:289-329`),
  and Phase 7's Term field is where this first went wrong. An un-renamed album
  round-trips correctly **even when the translation is missing**, so a test
  without the rename proves nothing.

### The slideshow

- **D-10: CSS `scroll-snap`, one block of rules, no markup change** (GAL-06).
  The choice between grid and slideshow is a display mode on the block — the
  same shape Phase 7 gave `auswahl` with `darstellung`. Keyboard and touch come
  from the browser's own scrolling; nothing is scripted.

### What must not change

- `internal/block/render.go:157-175`'s existing gallery markup, its
  `srcset`/`sizes`, its focus-point cropping and its own-aspect-ratio rendering
  from version 1.8. **Everything this phase adds happens after the grid.**
- `cmd/holzcloud/assets/bausteine.css:106-144`'s grid rules.
- The shop's product gallery (D-08).
- Any released migration.

### Language

- **D-11: every identifier, comment and test name this phase writes is
  English**, per the developer's decision of 2026-09-06 and
  `.planning/GLOSSARY.md`. **But the stored values stay German**: the block type
  is `"galerie"` because that string is in every existing page's `blocks`
  column and in every bundle ever exported, and the CSS class is `hc-galerie`
  because it is in all eight themes. Turning those is LANG-08's question in
  Phase 12, not this phase's. New glossary entries this phase needs — `Album`,
  `Diashow` → `slideshow`, `Grossansicht` → `large view`, `Lichtkasten` →
  `lightbox` — go into the glossary in the commit that first uses them.

</decisions>

<references>
## Canonical References

- `.planning/ROADMAP.md` — Phase 11: goal, six success criteria, planning notes.
- `.planning/REQUIREMENTS.md` — GAL-01 … GAL-07.
- `.planning/GLOSSARY.md` — **binding**, including the stored-value rule.
- `internal/block/block.go:45-46` `TypeGallery`, `:181-217` `Block`, `:220`
  `Item`.
- `internal/block/render.go:157-175` — the gallery renderer as it stands.
- `cmd/holzcloud/assets/bausteine.css:106-144` — the grid, and the comment
  explaining why the aspect ratio was given back.
- `internal/db/migrations/00005_templates_menus_media.sql:22-34` — the
  parent/ordered-child shape an album copies.
- `internal/bundle/format.go:163-170`, `import.go:289-329` and `:573-583` — how
  a reusable thing crosses the bundle by name.
- `internal/tmplspec/TEMPLATE-SPEC.md` — the data contract, and the place where
  `bausteine.css` is **not** mentioned (D-01).
- `internal/tmplmgr/script.go` — why the lightbox cannot be scripted.

</references>

<insights>
## Existing Code Insights

- **The media package is already strong and is not the work here.**
  `internal/media/` has `crop.go`, `responsive.go`, `strip.go`, `usage.go`,
  `variant_store.go`, `mp4.go`. The large view **serves an existing large
  variant** rather than minting a new size.
- **An album pays the same tax menus and terms pay**: a table, a migration, an
  admin area, website scoping, the bundle round trip, `TEMPLATE-SPEC.md` +
  `SampleData` + `MinimalData` if a theme can reach it, and `tools/i18n -write`
  followed by `-schweiz`. Budget it once.
- **A whole-tree count must not be asserted by a plan that shares its wave.**
  Found twice in this phase and it is one rule, not two: the plan checker caught
  11-04 gating `admin templates == 68`, a number **11-03** produces; and 11-02's
  execution hit `1280` where its plan said `1277`, because **11-01's** three
  lightbox labels landed in the same tree. 11-02 did the right thing — it
  *measured* the attribution by taking its own package out and re-running, rather
  than arguing it — but the threshold was unsatisfiable as written.

  **The honest form of such a gate is an isolation measurement**: *removing this
  plan's work must not change the count.* A whole-tree total belongs in the last
  plan of the phase, once, where every contributor has landed.

- **Counting gates are measured line-by-line against the post-change tree**, and
  a gate must measure what its name claims. Phase 9 produced three of the same
  failure, one per wave: a guessed number; a condition needing two numbers from
  a command that prints one; and a command measuring something adjacent to its
  name (`grep 'SlugifyKey'` counts the word — `grep 'field\.SlugifyKey('`
  counts the calls). Put the parenthesis in.
- **The browser pass runs after the code-review fix round**, not before. Phase 7
  and Phase 8 both had user-visible changes land after a signed-off pass.

</insights>

<discretion>
## Claude's Discretion

| # | Question | Settled | Evidence |
|---|---|---|---|
| 1 | Does the shop's product gallery inherit the large view? | **No, and the plan says so** (D-08) | The roadmap asks for the decision either way. Three picture lists exist; GAL-07 asks the block and the album to agree, not the shop to be rebuilt |
| 2 | What happens without `bausteine.css`? | **The spec starts requiring it, and the markup degrades harmlessly** (D-01) | All eight themes link it; TEMPLATE-SPEC mentions it nowhere. A `:target` lightbox does not degrade, it fails |
| 3 | Does the large view wrap around? | **No — the last picture has no "next"** (D-04) | An absent control is the answer Phase 9's row stepper already gives |
| 4 | Is the block type renamed to `gallery`? | **No** (D-11) | `"galerie"` is in every page's `blocks` column and every exported bundle; `hc-galerie` is in all eight themes. That is LANG-08's question in Phase 12 |

</discretion>

<deferred>
## Deferred Ideas

- **Masonry and justified layouts** — declined 2026-09-05.
- **An album as an addressable field kind.** If it ever becomes one it inherits
  `SplitValues`/`JoinValues` (FIELD-07) rather than inventing a second spelling.
  Not in scope: nothing in GAL-01…07 asks for it.
- **Zoom or pan inside the large view.** That needs script.
- **Renaming `"galerie"` and `hc-galerie` to English** — LANG-08, Phase 12.

</deferred>
