# Phase 11: Galerie - Pattern Map

**Mapped:** 2026-09-06
**Files analyzed:** 19 (9 new, 10 modified)
**Analogs found:** 16 / 19
**Language:** English identifiers, comments and test names (D-11). Stored values
stay German — `"galerie"`, `hc-galerie`, and any new stored vocabulary this
phase mints (see §A.4).

**Two findings decide the plan, and neither is a style question.**

1. **`internal/block/render.go` has no index and no block identity of any
   kind** — and worse, the block HTML is **frozen into `pages.content_html` at
   save time**. Both halves of D-03's fragment id are unavailable today, and the
   second half is what an album collides with. §A.
2. **An unstyled `:target` overlay renders as "every picture a second time, at
   full size, in document order"** — not as a broken box — *provided* the markup
   is a `<figure>` and not a positioned overlay container. That is a
   constrainable outcome, and §B says exactly what constrains it.

---

## A. (a) How does the renderer know a block's position?

### A.1 It does not. Measured.

`internal/block/render.go:66-75` is the whole loop:

```go
func Render(blocks []Block, s Set, look Lookup, md Markdown) string {
	var b strings.Builder
	for _, blk := range blocks {
		if own, ok := s.OwnOf(blk.Type); ok {
			renderOwn(&b, blk, own, s, look, md)
			continue
		}
		renderOne(&b, blk, look, md)
	}
	return b.String()
}
```

`for _, blk := range blocks` — the index is discarded at the one place it
exists. `renderOne` (`render.go:78`) and `renderOwn` (`render.go:264`) take
`(b *strings.Builder, blk Block, …)` and no position. Inside the gallery arm
(`render.go:157-175`) the item loop is `for _, it := range blk.Items` — the item
index is discarded too.

`block.Block` (`block.go:181-217`) carries **no `ID` field**: `Type`, `Markdown`,
`MediaID`, `Alt`, `Caption`, `Variant`, `Title`, `Text`, `Source`, `LinkText`,
`LinkURL`, `PosterID`, `Items`, `Fields`. `block.Item` (`block.go:220-227`) has
no id either. A block's only identity is its position in the JSON array, and
`block.Apply` (`block.go`/`form.go`) reorders that array freely
(`form.go:213-230`, actions `up`/`down`/`delete`).

### A.2 What has to change, and how invasive it is

**Minimally invasive — two `range` statements and two signatures.** Concretely:

- `render.go:68` → `for i, blk := range blocks`
- `render.go:70` and `:73` → pass `i` to `renderOwn` / `renderOne`
- `render.go:78` → `func renderOne(b *strings.Builder, at int, blk Block, look Lookup, md Markdown)`
- `render.go:158` → `for j, it := range blk.Items`
- id minted as e.g. `fmt.Sprintf("b%d-p%d", at+1, j+1)` inside the gallery arm only.

`renderOwn` does not need the index today, but giving it the same parameter
keeps the two arms symmetrical; a reviewer reading one and not the other should
not have to wonder. That is a judgement call for the plan, not a fact — **state
it either way**.

**Callers:** exactly two, both unaffected because they call `Render`, not the
unexported arms:
- `internal/admin/page_blocks.go:80` — `block.Render(blocks, set, h.blockImages(ctx, websiteID), page.RenderMarkdown)`
- `internal/bundle/import.go:694` — same call, on import re-render.

**No analog** exists for "the renderer knows where it is": no other function in
`internal/block` takes a position. The closest *idiom* in this tree for a
per-instance id is `internal/bundle/import.go:1131`, which passes the loop index
`i` straight into `CreateItem(..., i)` as `sort_order`. Copy the shape (index of
the range loop used directly), not the meaning.

### A.3 The trap that matters more than the index

**Block HTML is rendered once, on save, and stored.**
`internal/admin/page_blocks.go:74-81`:

```go
// renderBlocks turns a page's blocks into the HTML that is stored and served.
// … means a block page is rendered once on save rather than on every visit.
```

The public side then reads `pg.ContentHTML` and only *rewrites* it —
`internal/public/pagedata.go:32` (`snippet.Expand`), `:36`
(`filterByPlugins`), `:40` (`h.responsive(...)` → `media.MakeResponsive`).

Two consequences the plan must carry:

- **Fragment ids are frozen at save.** Reordering blocks on a *later* save
  re-mints them; an old bookmark to `#b2-p3` lands elsewhere. That is
  acceptable and should be **written down** rather than discovered.
- **An album cannot be expanded at save time.** GAL-03 says "changing the album
  changes every page that carries it, without touching those pages". If the
  album's pictures are baked into `content_html` on save, GAL-03 is false and
  the failure is silent. **The analog is `snippet.Expand`**
  (`internal/snippet/store.go:251-264`), the one existing mechanism in this tree
  for late binding: a marker `[[snippet:key]]` survives into `content_html` and
  is replaced at request time, for exactly this reason — the comment at
  `internal/public/pagedata.go:22-25` says so in these words ("Baking the
  expansion into content_html on save would freeze a copy of the opening hours
  into every page, which is the thing snippets exist to prevent").

  So a gallery block that names an album must render to a **marker** at save
  time and be expanded in `pageContent` before `h.responsive` runs — the
  ordering matters, because the expanded `<img>` tags must still be seen by
  `media.MakeResponsive` (`pagedata.go:40` wraps `body` after `Expand` at `:32`;
  put the album expansion between them, next to `snippet.Expand`).

  **What NOT to copy from `snippet.Expand`:** its map is `map[string]template.HTML`
  loaded per request for the *whole website* (`loadSnippets`, pagedata.go:~300).
  A website with fifty albums must not load fifty albums to render one page.
  Load by the keys actually present, the way `LoadImageSets`
  (`internal/media/variant_store.go:176`) does — its comment states the rule:
  "Only the files actually named in the HTML are looked up".

### A.4 The `Variant` collision — a second stored-vocabulary decision

D-10 wants a display mode on the gallery block. **`Block.Variant` is already
taken for a gallery**: `block.go:268-277` reads `Variant` as the column count
(`"2"`, `""`→3, `"4"`), and the editor's select writes it at
`cmd/holzcloud/templates/admin/block_list.html:157-161`. A slideshow flag cannot
ride on `Variant` without ambiguity.

**Analog for a second display axis:** `internal/field/field.go:271-302` —
`Def.Display`, constant `DisplayButtons = "knopfreihe"`, with the doc-comment
rule "an empty Display is the drop-down that already exists". Copy that shape
exactly: a new `Block.Display string \`json:"darstellung,omitempty"\`` beside
`Variant` at `block.go:203`, empty meaning grid, one named constant for the
slideshow. **The stored value is German** by the GLOSSARY rule
(`.planning/GLOSSARY.md:178-200`) and by the precedent of `knopfreihe`; the Go
identifier is English.

Everything a new `Block` field must touch, all in one commit:
- `internal/block/block.go:203` — the field.
- `internal/block/form.go:180-190` — a `case "darstellung":` in `setBlockField`.
- `cmd/holzcloud/templates/admin/block_list.html:157` — the select, beside the columns one.
- `internal/bundle/blocks.go:28-33` (export) and `:70-76` (import) — the struct literal in **both**; a field added to one and not the other loses the value on the round trip, silently.
- `internal/bundle/format.go` `Block` struct — the JSON field.
- `internal/block/block_test.go:248` `TestKodierenUndLesenIstVerlustfrei` is the encode/decode guard; `internal/bundle/bundle_test.go:653` `TestRoundTripKeepsBlocks` is the bundle guard.

---

## B. (b) The theme stylesheet, and what an unstyled lightbox looks like

### B.1 Where a theme's CSS comes from

`TEMPLATE-SPEC.md:178-190` §4 "Asset URLs" is the whole contract:

| You write | The browser gets |
|---|---|
| `/t/style.css` | `style.css` from your archive |
| `/t/fonts/inter.woff2` | `fonts/inter.woff2` from your archive |

`/t/` maps to the template directory. §3's layout example
(`TEMPLATE-SPEC.md:141`) links **only** `/t/style.css`, and so does §11's
complete minimal template (`TEMPLATE-SPEC.md:873`). The string
`bausteine.css` appears **nowhere** in the specification — verified by grep over
`internal/tmplspec/TEMPLATE-SPEC.md`.

Measured against the shipped themes, all eight link it from `layout.html`:

```
journal:40  midnight:41  default:40  magazine:40
holzcloud:48  schlicht:40  weide:51  rudel:51
```

(`cmd/holzcloud/templates/public/*/layout.html`.) So D-01's finding is exact:
**every shipped theme has it, and no document tells anyone to.**

`/assets/` is same-origin and therefore allowed by `default-src 'self'`
(`internal/web/headers.go`); nothing in `internal/tmplmgr/external.go` objects to
a theme linking it, because it is not external.

### B.2 What an unstyled `:target` overlay actually renders as

There is no CSS anywhere except `bausteine.css` and the theme's own file. With
`bausteine.css` absent, **the new rules simply do not exist**, so the markup
renders with UA defaults only. That means:

- A `<figure id="b1-p2">` containing a second `<img>` of the same picture and a
  `<figcaption>` renders as **a normal figure in the flow** — the picture again,
  at its natural size, with its caption underneath. Ugly (the gallery is
  effectively doubled), harmless, and semantically true.
- `<a href="#b1-p3">Weiter</a>` renders as an ordinary link and **works**: it
  moves the viewport to the sibling figure. Without CSS the lightbox degrades
  into *an anchored list of large pictures*, which is a coherent page.
- The close control (D-05, a link to a fragment that matches nothing) renders as
  a link that does nothing visible. Harmless.

**Therefore the constraint on the markup, as an acceptance criterion:** the
large view is a **`<figure>` sibling inside the gallery `<div>`**, in document
order after the grid tiles, hidden by `display: none` in `bausteine.css` and
revealed by `.hc-galerie__gross:target { display: … }`. It must **not** be a
wrapper element that only makes sense when positioned (`position: fixed`,
a backdrop `<div>`, `aria-modal` markup, an element whose only content is
chrome). Anything whose unstyled rendering is "a stack of grey boxes above the
page" fails the criterion.

**`display: none` and not `opacity`/`visibility`:** `weide`'s
recorded finding (`.planning/STATE.md:209`) is that an absolutely positioned
element counts toward scroll width **even with `visibility: hidden`**, and that
made every page of that theme scroll sideways. The same mistake here costs the
same bug on every gallery page in the product.

**Verification:** load a gallery page with `/assets/bausteine.css` blocked (the
browser pass of the standing gate), and confirm the page is a readable list of
pictures with working anchors.

### B.3 The specification change (D-01 half 1)

Three places in `internal/tmplspec/TEMPLATE-SPEC.md`, all in one commit:
- **§4 table, `:180-184`** — add the `/assets/bausteine.css` row with its reason.
- **§3 layout example, `:141`** — add the `<link>` above `/t/style.css`, so
  order matters visibly (the theme wins).
- **§11 minimal template, `:873`** — same line, or the "complete minimal
  template" is a theme that renders blocks wrong.

`internal/tmplspec/spec_test.go:19` `TestSpecDocumentsEveryFieldOfTheContract`
checks the *data contract* by reflection and will **not** catch a missing asset
row — the spec has no test for §4. **No analog** for a guard over the asset
table. If the plan wants one, the cheapest honest gate is a test in
`internal/tmplspec` that asserts `strings.Contains(Markdown(), "/assets/bausteine.css")`
plus one that asserts every shipped `layout.html` links it — the second is the
one that would have caught the drift.

---

## C. (c) The album's table and store

### C.1 The migration — `menus` verified as the model

`internal/db/migrations/00005_templates_menus_media.sql:22-40`, read whole:

```sql
CREATE TABLE menus (
    id           INTEGER PRIMARY KEY,
    website_id   INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    location_key TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(website_id, location_key)
) STRICT;

CREATE TABLE menu_items (
    id         INTEGER PRIMARY KEY,
    menu_id    INTEGER NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES menu_items(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    …
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (…)
```

**Copy:** the two-table shape, `website_id … ON DELETE CASCADE` on the parent,
`album_id … ON DELETE CASCADE` on the child, `sort_order INTEGER NOT NULL
DEFAULT 0`, `STRICT`.

**Do NOT copy:**
- `parent_id` — an album is a flat ordered list, not a tree. Everything in
  `internal/menu/store.go:280-349` (`buildTree`, `createsCycle`, `materialize`)
  exists only because menus nest, and it exists because a bug was found there
  (the doc comment at `:281-289` records it). An album that copies it inherits a
  hazard it has no use for.
- `id INTEGER PRIMARY KEY` without `AUTOINCREMENT` — `menus` is from 00005 and
  predates the house style. `snippets` (`00010:35`), `terms` (`00015:13`) and
  `csv_imports` (`00049`) all use `INTEGER PRIMARY KEY AUTOINCREMENT`. **The
  two candidates disagree; the newer one wins.**
- `UNIQUE(website_id, location_key)` — an album has no location key. The
  question the plan must settle is whether an album has a **slug** (like
  `terms`, `00015:17-20` `UNIQUE (website_id, slug)`) or only a name. §D says it
  should have one, and why.

**The picture rows:** the child table holds `media_id INTEGER NOT NULL
REFERENCES media(id) ON DELETE CASCADE`, plus `alt` and `caption` — the same
three things `block.Item` carries (`block.go:220-227`). GAL-07's "one mechanism"
is satisfied by both feeding the *same* `[]block.Item` into the *same* gallery
arm, not by a second renderer.

**Migration prose style:** `internal/db/migrations/00049_csv_imports.sql` is the
model, and it is the most recent. Copy: a paragraph above the table saying *why*
it exists, a comment per non-obvious column, and — importantly — the
**`-- +goose Down` paragraph at `00049`'s tail (last 20 lines)**, which explains
why *this* rollback is two statements and how it differs from an index swap.
`00050` creates and changes nothing, so it gets the same short, complete
rollback: `DROP TABLE albums_items; DROP TABLE albums;` in that order. **Do NOT
copy `00049`'s German prose as a language precedent** — D-11 makes this phase's
comments English; the *stored values* stay German if any closed vocabulary is
minted, which for a flat album should be none.

**Number:** `00050`. Verified: the directory ends at `00049_csv_imports.sql`.

### C.2 The store — `internal/menu/store.go`, read whole (349 lines)

**Parent/ordered-child idiom, exactly as it stands:**

| Concern | Where | What it does |
|---|---|---|
| Create parent | `:25-33` | INSERT on `s.DB.Write`, then `LastInsertId` → re-`GetMenu` and return the row. Never construct the struct by hand. |
| Read one | `:36-50` | `s.DB.Read.QueryRowContext`, `sql.ErrNoRows` → `(nil, nil)`, not an error. |
| List | `:53-73` | `WHERE website_id = $1 ORDER BY name`, `defer rows.Close()`, `rows.Err()` at the end. |
| Create child | `:95-106` | takes an explicit `sortOrder int` from the caller. |
| List children | `:158-192` | `ORDER BY mi.parent_id NULLS FIRST, mi.sort_order` |
| **Reorder** | `:196-222` | **`SwapSortOrder(ctx, itemID1, itemID2)`** — one transaction, read both `sort_order`s, write them crossed, `defer tx.Rollback()` + `return tx.Commit()`. |

**How order is changed: by swapping two rows' `sort_order`, in a transaction —
not by rewriting the whole list.** That is the idiom to copy verbatim
(`:196-222`), and the admin side that drives it is
`HandleMenuItemReorder` (`internal/admin/menu.go:467`), routed at
`cmd/holzcloud/main.go:878`.

**How website scoping is enforced — and the honest answer is: it is not, in the
store.** Read the signatures:

```go
func (s *Store) GetMenu(ctx, id int64)                       // no websiteID
func (s *Store) UpdateMenu(ctx, id int64, name, locationKey) // no websiteID
func (s *Store) DeleteMenu(ctx, id int64)                    // no websiteID
func (s *Store) DeleteItem(ctx, id int64)                    // no websiteID
```

`menu.Store` scopes only `ListMenus` (`:55`) and `GetMenuTreeIn` (`:239`,
`WHERE m.website_id = $1`). Every other read and write trusts the handler to
have checked, and `internal/admin/menu.go:141-200` does check — but by
convention, in the handler, one path at a time.

**`internal/term/store.go` does it differently, and it is the newer and better
of the two.** Every mutating method carries the website id **in the WHERE
clause**:

```go
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	`DELETE FROM terms WHERE id = $1 AND website_id = $2`   // :285-287
}
func (s *Store) Rename(ctx context.Context, websiteID, id int64, name string) error {
	`UPDATE terms SET name = $1 WHERE id = $2 AND website_id = $3`  // :303-304
	if n, _ := res.RowsAffected(); n == 0 { return fmt.Errorf("term %d not found", id) }
}
```

`RowsAffected() == 0 → error` is what turns "belongs to another website" from a
silent no-op into a reported failure.

**Verdict where the two analogs disagree: `internal/term/store.go` wins for
scoping, `internal/menu/store.go` wins for ordering.** GAL-05 asks for an album
to be *invisible from every other website*, and a store whose `Delete` takes no
`websiteID` cannot make that a fact of the database. So:

- signatures: `AlbumStore.Get/Update/Delete/Rename(ctx, websiteID, id, …)` — term shape
- SQL: `… WHERE id = $1 AND website_id = $2` on every statement — term shape
- `RowsAffected() == 0` → a named error — term shape (`store.go:307-309`)
- item ordering and the swap transaction — menu shape (`store.go:196-222`), with
  the child scoped through a join or a re-check that the album is this
  website's, because `menu_items` has no `website_id` and neither will
  `album_items`.

**Do NOT copy** `term.Store`'s slug-normalisation coupling wholesale — the long
comment at `:318-328` warns that `EnsureNames` and `SetForPage` must derive the
slug with the *same* call (`page.Slugify`) or an import creates a second row
beside the one it meant to reuse. If an album gets a slug (§D), that warning
applies to it word for word and should be quoted in the album store's own doc
comment.

---

## D. (d) The bundle round trip for a reusable thing

### D.1 What travels by name today, and where

**`format.go:163-170`** states the rule the whole design rests on:

```go
	// Terms are the labels this page carries, spelled as their names are shown
	// — "Laufräder", not "laufraeder". … The name and not the slug because a
	// manifest exists to be read and repaired by hand, and because the reading
	// side has always worked this way …
	Terms []string `json:"terms,omitempty"`
```

**`export.go:518-534`** — the out-translation of a `KindTerm` value:

```go
		if kinds[key] == field.KindTerm {
			name := nameBySlug[val]
			if name == "" {
				continue     // a slug naming no term of this website: dropped
			}
			out[key] = name
			continue
		}
```

**`import.go:573-583`** — the in-translation, the lines the prompt names:

```go
		if kinds[key] == field.KindTerm {
			// Der Wert im Archiv ist ein Name, gespeichert wird ein Kürzel.
			// page.Slugify ist die eine Regel, die auch der Schlagwortspeicher
			// anwendet …
			out[key] = page.Slugify(val)
			continue
		}
```

**`export.go:535-545` / `import.go:585-592`** — `KindImage` travels as a **file
name**, resolved through `mediaByName`; a name that is not in the bundle
resolves to *nothing*, never to a number.

**`import.go:289-329`** `importTerms` creates every declared label before any
page is written, via `s.Terms.EnsureNames`, and reports `report.Terms = n` — the
count of what was *created*, not what the archive claimed ("eine Zahl, die ein
Bericht nennt, soll geglaubt werden können", `:331-332`).

**And the block-level copy of the same rule** is `internal/bundle/blocks.go`,
whose header comment (`:11-18`) is the sentence to quote in the plan:

> "the whole difficulty is one line long: a picture is stored as an id, and an
> id means nothing on the machine the bundle lands on. So it travels as a file
> name."

`exportBlocks` (`blocks.go:25-59`) maps `it.MediaID → mediaByID[...]`;
`importBlocks` (`blocks.go:68-101`) maps back through `mediaByName`;
`missingMedia` (`blocks.go:107-140`) reports the names that did not arrive.

### D.2 The exact shape an album must copy

Four pieces, and **all four are required or the round trip is a silent
data-loss**:

1. **`format.go`** — an `Album` struct beside `Menu` (`:38`) and `Snippet`
   (`:39`), added to `Manifest` as `Albums []Album \`json:"albums,omitempty"\``.
   Its pictures are **file names**, exactly as `BlockItem.Media` is:
   `{Name string; Items []AlbumItem}` with `AlbumItem{Media, Alt, Caption}`.
   Copy `format.go:163-170`'s comment discipline: say *why* the name and not the
   id, in the struct.
2. **`export.go`** — `exportAlbums(ctx, s, ws.ID, m)` called from the export
   chain at `export.go:158-170` (beside `exportMenus` `:158` and
   `exportSnippets` `:161`), using the same `mediaByID` map `exportBlocks`
   already receives at `:260`.
3. **`import.go`** — `importAlbums`, called from `:126-127`'s group
   (`importSnippets`, `importMenus`). **Placement matters:** albums must be
   created **before pages** if a page's gallery block names one, for the same
   reason `importMenus` is deliberately last (`import.go:1086-1087`: "Menus come
   last: their items point at pages by slug, and the pages have to exist"). An
   album points at *media*, not pages, so it goes **after media and before
   pages**. Say which, in the plan, with the reason.
4. **The block's reference to the album** — a gallery block naming album id `7`
   is the same class of value as `KindRef` and `KindImage`: a number that means
   nothing elsewhere. It travels as the album's **name**, and comes back through
   `page.Slugify`-style derivation — which is precisely why §C.1 says the album
   should carry a **slug** column: `terms` does (`00015:17-20`), and the whole
   name→slug re-derivation on import (`import.go:576-582`) depends on the
   stored side having a stable derived key that `Rename` does **not** move
   (`term/store.go:293-296`).

### D.3 The test, and why the rename is the whole point

**The named analog: `internal/bundle/bundle_test.go:1138`
`TestSchlagwortfeldRundreise`, sub-test `"umbenannt und an keiner Seite"`.** Its
comment at `:1124-1137` is the argument in full:

> "Deshalb wird hier vor dem Export umbenannt: ein Schlagwort, dessen Name noch
> zu seinem Kürzel passt, reist auch ohne die Übersetzung heil und bewiese gar
> nichts."

The mechanics to copy, step for step (`:1163-1200`):

1. create the thing → `slug == "moebel"`, assert it
2. `s.Terms.Rename(ctx, ws.ID, id, "Möbelbau")` — name moves, slug does not
3. store the value that references it, which is still the **old** slug
4. `exportTo` → `manifestOf`
5. assert the manifest contains `"Möbelbau"` **and** assert it does **not**
   contain `"moebel"` — both directions, because only the negative assertion
   fails when the translation is missing
6. import into a fresh website, assert the value resolves to the album that is
   actually there

The album test is `TestAlbumRoundTripAfterRename` (English, D-11), living beside
`TestRoundTripKeepsBlocks` (`bundle_test.go:653`). **The store therefore needs a
`Rename` before this test can be written** — that is a dependency between plans,
not a detail.

**What NOT to copy:** the German test name and prose. `bundle_test.go` is
entirely German; D-11 says everything this phase writes is English. That makes
this file **mixed-language**, deliberately. Say so in the plan, or a reviewer
will "fix" it.

---

## E. (e) Adding a new admin area — the full checklist

The most recent comparable areas are **snippets** (a named reusable thing a
website owns) and **menus** (a named thing with an ordered child list). **Menus
wins as the analog** — an album has children and needs reordering, snippets do
not. Where menus is old-fashioned (see §C.2 scoping), take the term/snippet
answer instead.

Every one of these moves in the same commit:

| # | Thing | Exact place | Copy from |
|---|---|---|---|
| 1 | Store construction | `cmd/holzcloud/main.go:210` `menuStore := menu.NewStore(database)` | verbatim shape |
| 2 | Handler dependency | `main.go:261` `admin.NewHandler(…, menuStore, mediaStore, snippetStore, termStore, …)` — **the constructor is positional and already 18 arguments long**; adding a 19th touches every call site incl. tests | note the cost in the plan |
| 3 | Router dependency struct | `main.go:629` field `menuStore *menu.Store`, `main.go:400` assignment, `main.go:694` local in `newRouter` | verbatim |
| 4 | Routes | `main.go:870-878` — nine lines, all `adminProtectedMux.HandleFunc("VERB /admin/websites/{id}/menus…", adminHandler.ErrHandler(adminHandler.HandleMenu…))` | verbatim; note the `{id}` + `{menuID}` two-level path shape |
| 5 | **Route authorization test** | `cmd/holzcloud/main_test.go:158` is the **`adminOnly`** table — an album is content, so it does **not** go there. It goes in the **editor-open** list at `main_test.go:186-190`, beside `{"GET", "/admin/websites/1/menus"}` | `main_test.go:186-190` |
| 6 | Session test | `main_test.go:203-210` `TestAdminRoutesRequireASession` — add the list route | verbatim |
| 7 | **`layoutPageNames`** | `internal/web/render.go:46` — the single long line; `menu_list` and `menu_edit` are both in it. **A page missing here renders without the base layout and nobody notices until the screen is opened.** | the list itself |
| 8 | Handler | `internal/admin/menu.go` whole — `MenuListData`/`MenuEditData` embed `web.LayoutData` (`:17-19`, `:38-44`), `web.NewLayoutData(r, h.sm, web.Titlef(...))` (`:75`), `data.ActiveNav = "menus"` (`:82`, `:195`), `data.CurrentWebsite = ws` (`:83`), `web.RenderAdmin(w, h.templates, r, "menu_list", data)` (`:84`) | verbatim |
| 9 | Flash + redirect + htmx | `menu.go:126-135` — `web.SetFlashSuccess`, then `if r.Header.Get("HX-Request") == "true" { w.Header().Set("HX-Redirect", redirect); return nil }`, else `http.Redirect(…, http.StatusSeeOther)` | verbatim; this is the CLAUDE.md rule made concrete |
| 10 | Templates | `cmd/holzcloud/templates/admin/menu_list.html` + `menu_edit.html`; the one-screen list-with-inline-form pattern is `snippet_list.html:16` (form posts to the list URL) with `?edit={{.ID}}` (`:97`) | pick one and say which |
| 11 | Navigation | `cmd/holzcloud/templates/admin/base.html:100-102` — the `nav-item` anchor with the `is-active`/`aria-current` pair keyed on `.ActiveNav`, plus an `{{template "icon-…"}}` that must exist in `icons.html` (registered as a shared partial at `internal/web/render.go:33`) | `base.html:100-102` |
| 12 | i18n | `go run ./tools/i18n -write` then `-schweiz`; standing gate wants `0 offen, 0 verwaist` | ROADMAP success criterion 6 |

**What NOT to copy from `internal/admin/menu.go`:** `HandleMenuCreate`'s
`isValidLocationKey` branch (`:113-118`) — an album has no location key — and
its reliance on the store not scoping by website (`:141-200` re-fetches the
website and compares by hand). Use the term-shaped store (§C.2) so the handler's
check is a belt over a braces, not the only strap.

---

## F. (f) `SampleData` / `MinimalData` / `TEMPLATE-SPEC.md`

**The three places, and the tests that tie them:**

| # | Place | Path |
|---|---|---|
| 1 | The structs — the contract itself | `internal/template/loader.go` (`PageData`, `PageContent` at `:446`, `SiteData`) |
| 2 | The fixtures | `internal/template/sample.go:29` `SampleData()`, `:320` `MinimalData()` |
| 3 | The document | `internal/tmplspec/TEMPLATE-SPEC.md` §5, `.Site` table at `:200-219` |

**The tests:**

- `internal/tmplspec/spec_test.go:19` **`TestSpecDocumentsEveryFieldOfTheContract`** — walks the contract structs by reflection (`contractPaths()`, `:34-60`) and fails if the Markdown never mentions a dotted path. **This is the test that fires if a field is added to the contract and not to the document.**
- `internal/template/sample_test.go:21` **`TestSampleDataFillsEveryField`** — fails if `SampleData` leaves any contract field at its zero value.
- `internal/template/sample_test.go:87` **`TestMinimalDataLeavesOptionalFieldsEmpty`** — the other half; it names the specific traps (`:91` neighbours, `:94` dates, `:97` menus, `:100` archive entries).
- Consumed by `internal/template/check.go:242-243`: `full := renderProblem(set, SampleData()); empty := renderProblem(set, MinimalData())` — the two renderings every upload survives.

**The decision the plan must make explicitly:** *does a theme reach an album at
all?* If the album surfaces only inside a gallery block, the theme sees it as
`.Page.ContentHTML` and **none of the three places changes**. If the plan adds
`.Site.Albums` (nothing in GAL-01…07 asks for it), then **all three change or
the suite fails**, and the cost is three edits plus two fixtures. **Recommend:
do not add it.** The cheaper answer keeps the contract fixed and the block the
one door.

The `TEMPLATE-SPEC.md` change this phase *does* owe is §B.3's asset row, which
is a different section and a different reason.

---

## G. (g) Which media variant the large view should serve

**The sizes that exist** — `internal/media/variants.go:26-35`:

```go
// variantSpecs are the widths generated for every uploaded photo.
// Three sizes, not five: each one costs disk and seconds of CPU …
var variantSpecs = []VariantSpec{
	{Label: "thumb", Width: 400},
	{Label: "medium", Width: 800},
	{Label: "large", Width: 1600},
}
```

Only smaller copies are made — `variants.go:102`: "upscaling a small logo to
1600 pixels wastes …". So a small original has fewer than three.

**The right answer is: link no variant at all.** `internal/block/render.go:41-46`
already states the house rule in the file the change lands in:

> "Nothing here writes a srcset. The public pipeline already runs every page
> through media.MakeResponsive, which knows which variants exist and adds them —
> and which leaves a hand-written sizes alone. … Two places computing srcsets
> would be two places to get the variant naming wrong."

So the large view emits an ordinary `imgTag(img, it.Alt, "100vw", false)`
(`render.go:363-393`) — the original `src`, a hand-written `sizes` of `100vw`
because the large view *is* the viewport — and `media.MakeResponsive`
(`internal/media/responsive.go:28`) adds the candidate list at request time.
`ImageSet.SrcSet` (`variant_store.go:147-162`) then offers `400w`, `800w`,
`1600w` and the original, and the browser picks `large` (1600w) on a desktop by
itself. **No new size is minted, and none is named.**

Two reasons not to hand-write `/media/1/foo-large.jpg`:
- **Versioning.** `ImageSet.VersionedPath` / `versionQuery`
  (`variant_store.go:129-138`) appends `?v=N` because media is cached immutable
  for a year and cropping broke that promise. A hand-built path carries no
  version and shows the pre-crop picture forever.
- **Absence.** A small original has no `large` variant at all; the hand-built
  path 404s.

`cropped: false` on the large view is deliberate: focus-point `object-position`
only matters where the stylesheet squeezes a picture into a fixed shape
(`render.go:359-361`), and the large view does not.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/block/render.go` (mod) | renderer | transform | its own `:157-175` + `:78-243` | exact (self) |
| `internal/block/block.go` (mod, `Display` field) | model | — | `internal/field/field.go:271-302` | role-match |
| `internal/block/form.go` (mod) | parser | request-response | `form.go:180-190` | exact (self) |
| `internal/block/block_test.go` (mod) | test | — | `:248`, `:302`, `:112` | exact |
| `cmd/holzcloud/assets/bausteine.css` (mod) | style | — | `:106-144` + the `weide` finding in `.planning/STATE.md:209` | exact |
| `internal/db/migrations/00050_albums.sql` (new) | migration | batch DDL | `00005:22-40` (shape) + `00049` (prose, AUTOINCREMENT, Down half) | exact |
| `internal/album/album.go` (new) | model | — | `internal/menu/menu.go` / `internal/term/term.go` | role-match |
| `internal/album/store.go` (new) | store | CRUD | `internal/term/store.go:284-311` (scoping) + `internal/menu/store.go:196-222` (ordering) | **two analogs, see §C.2** |
| `internal/album/store_test.go` (new) | test | — | `internal/menu/*_test.go` | role-match |
| `internal/admin/album.go` (new) | handler | request-response | `internal/admin/menu.go:54-135, :467` | exact |
| `internal/admin/album_test.go` (new) | test | — | `internal/admin/snippet_fields_test.go:1-60` | role-match |
| `cmd/holzcloud/templates/admin/album_list.html` (new) | template | — | `menu_list.html` / `snippet_list.html:16, :97` | exact |
| `cmd/holzcloud/templates/admin/album_edit.html` (new) | template | — | `menu_edit.html` | exact |
| `cmd/holzcloud/templates/admin/base.html` (mod) | template | — | `:100-102` | exact |
| `cmd/holzcloud/templates/admin/block_list.html` (mod) | template | — | `:157-161` | exact |
| `internal/web/render.go:46` (mod) | config | — | the list itself | exact — **easy to forget** |
| `cmd/holzcloud/main.go` (mod) | config | — | `:210, :261, :400, :629, :694, :870-878` | exact |
| `cmd/holzcloud/main_test.go` (mod) | test | — | `:186-190` (**not** `:158`) | exact |
| `internal/bundle/format.go` (mod) | model | — | `:38-40`, `:163-170` | exact |
| `internal/bundle/export.go` (mod) | transform | batch | `:271-302` `exportMenus` + `:518-534` | exact |
| `internal/bundle/import.go` (mod) | transform | batch | `:289-329` `importTerms` + `:573-583` | exact |
| `internal/bundle/blocks.go` (mod) | transform | — | `:25-101` | exact (self) |
| `internal/bundle/bundle_test.go` (mod) | test | — | `:1138` `TestSchlagwortfeldRundreise` | exact |
| `internal/public/pagedata.go` (mod, album expansion) | handler | transform | `:32` `snippet.Expand` + `internal/snippet/store.go:251-264` | role-match — **see §A.3** |
| `internal/tmplspec/TEMPLATE-SPEC.md` (mod) | doc | — | its own `:141, :180-184, :873` | exact (self) |

---

## Shared Patterns

### Website scoping on every read and write
**Source:** `internal/term/store.go:284-311`
**Apply to:** every method of `internal/album/store.go`
Website id in the `WHERE` clause, `RowsAffected() == 0` → a named error. Not the
handler-only convention `internal/menu/store.go:36-92` uses.

### Ordered children
**Source:** `internal/menu/store.go:196-222` `SwapSortOrder`
**Apply to:** album picture reordering, and `internal/admin/album.go`'s reorder
handler (`internal/admin/menu.go:467`, route `main.go:878`).

### A picture crosses the bundle as a file name
**Source:** `internal/bundle/blocks.go:11-18` (the reason), `:25-59` / `:68-101`
(the two halves), `:107-140` (`missingMedia`, so the report says which file)
**Apply to:** the album's items in `export.go` / `import.go`.

### A reusable thing crosses the bundle by its name
**Source:** `internal/bundle/format.go:163-170`, `export.go:518-534`,
`import.go:573-583`
**Apply to:** the album, and to a gallery block's reference to one. Proven by a
test that **renames first** — `bundle_test.go:1124-1137` says why.

### Flash, redirect, and the htmx branch
**Source:** `internal/admin/menu.go:126-135`
**Apply to:** every POST handler in `internal/admin/album.go`.

### A doc comment that says why, and what is deliberately not done
**Source:** `internal/db/migrations/00049_csv_imports.sql` (whole),
`internal/block/render.go:41-46`, `internal/bundle/blocks.go:11-18`
**Apply to:** the migration, the album store, and the lightbox arm of
`render.go` — in English (D-11).

---

## No Analog Found

| File / concern | Role | Data Flow | Reason |
|---|---|---|---|
| The lightbox markup and its `:target` rules | renderer + style | — | Nothing in this tree uses `:target` for anything. Grepped `bausteine.css` and `admin.css`: no `:target` rule exists. The shape is **new** and belongs in the plan as a decision with a reason, in the voice `internal/admin/media_crop.go:14-24` uses for its own invented pattern. The UI-SPEC the roadmap asks for covers the accessible shape. |
| A block knowing its own position | renderer | — | §A.1. No function in `internal/block` takes an index. The change is small; the *precedent* is absent. |
| A guard over the asset table in `TEMPLATE-SPEC.md` | test | — | §B.3. `spec_test.go` checks the data contract by reflection and nothing else. A theme could stop linking `bausteine.css` today and no test would notice. |
| Late expansion of a *database-backed list* into stored page HTML | handler | transform | §A.3. `snippet.Expand` is the only late-binding mechanism, and it expands a per-website map of pre-rendered HTML, not a per-marker lookup. The album's expansion is a near-copy with a different loading strategy — closer to `media.LoadImageSets` (`variant_store.go:176`) in how it decides what to load. |

---

## Explicitly out of scope, and it must be said in the plan

**The shop's product gallery is not touched** (D-08). It is the third picture
list in this tree — `internal/shop/product.go:331` `SetGallery`, a join table of
media ids, rendered by all themes as `.product__gallery`. GAL-07 asks the
gallery *block* and the *album* to agree; a reader who finds three lists and no
sentence will read GAL-07 as broken.

---

## Metadata

**Analog search scope:** `internal/block`, `internal/menu`, `internal/term`,
`internal/snippet`, `internal/bundle`, `internal/media`, `internal/admin`,
`internal/public`, `internal/tmplspec`, `internal/web`, `internal/db/migrations`,
`cmd/holzcloud`, `cmd/holzcloud/templates/{admin,public}`,
`cmd/holzcloud/assets`.
**Files read in full:** `internal/block/render.go`, `internal/menu/store.go`,
`internal/bundle/blocks.go`, `internal/db/migrations/00049_csv_imports.sql`,
`.planning/phases/11-galerie/11-CONTEXT.md`, `.planning/GLOSSARY.md`.
**Tracked-source check:** every analog path above is git-tracked; no
`.gsd/capabilities/` mirror path appears in this document.
**Pattern extraction date:** 2026-09-06
