---
phase: 11-galerie
reviewed: 2026-09-08T01:30:00Z
depth: deep
files_reviewed: 24
files_reviewed_list:
  - cmd/holzcloud/assets/bausteine.css
  - cmd/holzcloud/main.go
  - cmd/holzcloud/main_test.go
  - cmd/holzcloud/templates/admin/album_edit.html
  - cmd/holzcloud/templates/admin/album_list.html
  - cmd/holzcloud/templates/admin/base.html
  - cmd/holzcloud/templates/admin/block_list.html
  - cmd/holzcloud/templates/admin/icons.html
  - cmd/holzcloud/templates/admin/import_report.html
  - internal/admin/album.go
  - internal/admin/bundle.go
  - internal/admin/handler.go
  - internal/admin/page.go
  - internal/admin/page_blocks.go
  - internal/album/album.go
  - internal/album/expand.go
  - internal/album/store.go
  - internal/block/block.go
  - internal/block/form.go
  - internal/block/render.go
  - internal/bundle/blocks.go
  - internal/bundle/export.go
  - internal/bundle/format.go
  - internal/bundle/import.go
  - internal/db/migrations/00050_albums.sql
  - internal/public/handler.go
  - internal/public/pagedata.go
  - internal/tmplspec/TEMPLATE-SPEC.md
  - internal/tmplspec/spec_test.go
  - internal/web/render.go
findings:
  critical: 4
  warning: 7
  info: 6
  total: 17
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-09-08T01:30:00Z
**Depth:** deep (cross-file: import graph, call chains, request pipeline, bundle round trip)
**Files Reviewed:** 24 source files + 8 test files across `b5ce7bd`..`4a4c5f4` (commits carrying `(11-01)`…`(11-06)`; the interleaved audit-workflow commits were excluded)
**Status:** issues_found

## Summary

The website-isolation work is genuinely good and is the strongest part of the
phase. `internal/album/store.go` puts `website_id` in the WHERE clause of all
sixteen statements, four item statements carry their own subquery on `albums`,
seven mutating methods turn `RowsAffected() == 0` into a named error, and every
signature takes `websiteID` immediately after the context — the compiler
enumerates the callers, which is exactly the KB's prescription. All nine admin
routes go through `albumFromPath`, which resolves both ids together. The
A-scope family is closed at the store, and I could not construct a
cross-website read or write through any of the nine routes. `LoadFor` binds
`a.website_id` *and* `m.website_id = a.website_id`, and
`TestAlbumFromAnotherWebsiteExpandsToNothing` gives both websites an album of
the same slug, so it proves something.

The lightbox is `:target` and nothing else — no `<dialog>`, no `showModal`, no
script on any path; the large view is a `<figure>` sibling in the flow, hidden
with `display: none` and never with `visibility`/`position: fixed`, so the
`weide` scroll-width finding is not re-committed. The slideshow is
scroll-snap on a container with `overflow-x: auto`, and the one place it could
widen a page — `inline-size: min(90vw, 48rem)` on the revealed panel — is
inside that scroll container, so it scrolls the track and not the document.
`Empty()` was fixed without becoming a no-op:
`TestGalleryWithNeitherItemsNorAlbumIsStillDropped` stayed green under the
mutation, and I confirmed by reading `block.go:296-312` that the new arm is
gated on `TypeGallery && AlbumSlug != ""`.

The two recorded plan defects were resolved correctly. 11-05's `Album:` grep
gate is genuinely unsatisfiable (`Album:` cannot match inside `AlbumSlug:`) and
the substance — the field in both struct literals — is present at
`blocks.go:43` and `blocks.go:96`. 11-06's ordering gates were worked around by
splitting the prose between the call site and the doc comment, and the actual
call order in `Import` is `importMedia` (105) < `importAlbums` (137) <
`importPages` (138), which is what the requirement asked for.

**What the phase got wrong is everything downstream of the marker.** GAL-03's
late binding was proved on exactly one code path — an unconditional GET of
`HandlePage` — and it is false on three others: the conditional-request path
(a warm browser gets 304 and keeps the old gallery *indefinitely*), the Atom
feed (which serves the raw `[[album:...]]` syntax to subscribers), and the
admin preview. Separately, the bundle round trip silently merges two albums
that share a name — which is the state `album.Store.Rename` explicitly
documents as producing — losing one album entirely and re-pointing its
galleries at the other. And the block editor destroys an album-backed gallery
block on the next save whenever the album select is not drawn.

All four of the Critical findings below were reproduced with a failing test
against HEAD; the literal output is quoted. The scratch tests were written in a
copy of the tree and deleted; **no source file in this repository was
modified.**

---

## Critical Issues

### CR-01: A warm browser cache defeats GAL-03 — the visitor keeps the old gallery forever

**File:** `internal/public/pagedata.go:375-385`, `internal/public/handler.go:213`, `:299`, `:443-450`

**Issue:** `contentModTime` is the `Last-Modified` validator, and its own doc
comment states the exact rule it now breaks:

> "It has to account for the snippets, not just the page: editing the opening
> hours changes what the page renders without touching `pages.updated_at`, and
> a browser holding an `If-Modified-Since` would keep the old text
> indefinitely."

Phase 11 added a second source that changes a page's body without touching
`pages.updated_at` — the album — and did not extend `contentModTime`. Worse,
`serveCached` checks `If-None-Match` first and, when it does *not* match, falls
through to the `If-Modified-Since` check instead of serving 200. RFC 7232 §3.3
says `If-Modified-Since` MUST be ignored when `If-None-Match` is present; this
code does the opposite.

Concrete failing scenario, reproduced:

1. Visitor loads `/galerie`. Response carries `ETag: "bee061d2…"` and
   `Last-Modified: Mon, 07 Sep 2026 23:24:20 GMT`.
2. The editor adds `bank.jpg` to album `moebel`. `pages.content_html` and
   `pages.updated_at` are byte-for-byte unchanged — by design.
3. The visitor reloads. The browser sends both `If-None-Match: "bee061d2…"`
   and `If-Modified-Since: Mon, 07 Sep 2026 23:24:20 GMT`.
4. The new ETag differs, so the first check falls through. `modTime` is still
   `pg.UpdatedAt`, which is not after the header's date → **304 Not Modified**.

```
zz_review_test.go:20: first: 200 etag="bee061d2acf776459e6ddf79520ec644" last-modified=Mon, 07 Sep 2026 23:24:20 GMT
zz_review_test.go:36: second: 304 bodylen=0
zz_review_test.go:38: 304: the visitor keeps the old gallery although the album changed — GAL-03 is false on the cached path
--- FAIL: TestReviewConditionalRequestKeepsTheOldAlbum (0.08s)
```

There is no recovery: every later request sends the same `If-Modified-Since`
and gets the same 304. "Changing the album changes every page that carries it"
is false for every visitor who has been on the site before.

**Why the existing tests do not catch it:**
`TestChangingAnAlbumChangesEveryPageThatCarriesIt`
(`internal/public/album_test.go:144`) is a good property test but issues two
*unconditional* GETs through `fetch` (`album_test.go:117-129`), which sets no
`If-None-Match` and no `If-Modified-Since`. The property it proves —
"`h.pageContent` produces different bytes" — is one layer below the property
the requirement is about, which is "what the browser displays".

**Fix:** two independent changes, both needed.

```go
// internal/public/pagedata.go — the validator has to see the albums.
func contentModTime(pg *page.Page, snippets snippet.Rendered, albums time.Time) time.Time {
	newest := pg.UpdatedAt
	if snippets.LatestUpdate.After(newest) {
		newest = snippets.LatestUpdate
	}
	// The newest updated_at among the albums this page names. Requires an
	// updated_at column on albums/album_items (00050 has only created_at) —
	// add it in a follow-up migration, or fall back to MAX(album_items.id)
	// per named album, which moves on every add and delete.
	if albums.After(newest) {
		newest = albums
	}
	return newest
}
```

```go
// internal/public/handler.go:437-450 — a present-and-mismatching If-None-Match
// must end the conditional negotiation, per RFC 7232 §3.3.
if match := r.Header.Get("If-None-Match"); match != "" {
	if match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
} else if since := r.Header.Get("If-Modified-Since"); since != "" {
	if t, err := http.ParseTime(since); err == nil && !modTime.Truncate(time.Second).After(t) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
}
```

The second change alone fixes the browser case for every browser that sends an
ETag; the first is what makes the date validator honest for the caches and
proxies that do not.

---

### CR-02: Two albums with one name collapse into one on the bundle round trip — an album and its pictures are lost, and its galleries silently bind to a different album

**File:** `internal/bundle/format.go:303-306`, `internal/bundle/blocks.go:129-144`, `internal/bundle/import.go:373-399`, `internal/album/store.go:233-236`

**Issue:** The manifest carries an album by name and nothing else, and the
importer re-derives the address with `page.Slugify(name)`. That is only sound
if a name identifies an album — and this store guarantees it does not.
`album.Store.Rename` says so in its own doc comment:

> "Two albums may therefore end up with the same visible name and different
> addresses, which is a thing an operator can see and undo."

A rename is precisely the operation the phase was built around (GAL-04), so
this is not an exotic state. When it happens:

- `exportAlbums` (`export.go:436-449`) writes two manifest entries with the
  same `"name"` and nothing to tell them apart.
- `exportBlocks` (`blocks.go:43`) maps each gallery's slug to that same name.
- `importAlbums` (`import.go:374-380`) creates the first, and the second hits
  `UNIQUE (website_id, slug)` → `ErrDuplicateName` → `continue` with a warning.
- `declaredAlbums` (`blocks.go:138-144`) is built from the **manifest**, not
  from what was created, so `importAlbumSlug` still resolves the second name →
  every gallery that pointed at the lost album now points at the surviving one.
- `missingAlbum` (`blocks.go:151-162`) reports nothing, because the name *was*
  declared.

Reproduced. Website `Werkstatt` has album A (`werkstatt-2024`, one picture) and
album B (`werkstatt-2025`, empty); the operator corrects A's name to
`Werkstatt 2025`. A page carries two galleries, one per album:

```
manifest "albums": [ {"name":"Werkstatt 2025","items":[…hobel.jpg…]},
                     {"name":"Werkstatt 2025"} ]
report.Albums=1  warnings=[Album 2 "Werkstatt 2025" konnte nicht angelegt werden: an album with this name already exists: "Werkstatt 2025"]
imported album slug="werkstatt-2025" name="Werkstatt 2025" pictures=1
block 0 type=galerie album="werkstatt-2025"
block 1 type=galerie album="werkstatt-2025"
BOTH galleries now point at the same album "werkstatt-2025"
--- FAIL: TestReviewTwoAlbumsOneNameCollide (0.11s)
```

The imported site shows album A's pictures where album B's belonged. The
warning names an album number, not a page or a block, so nothing connects the
loss to the galleries it silently rewrote.

**Why the existing tests do not catch it:** `TestAlbumRoundTripAfterRename`
(`bundle_test.go:1032`) uses exactly one album. It proves the *translation*
survives a rename; it never creates the collision the rename makes possible.
`TestManifestCarriesNoAlbumSlug` asserts the old slug is absent — which is
still true here. `TestImportCreatesAlbumsBeforePages` uses two albums with
distinct names.

**Fix:** the manifest needs a per-album disambiguator, or the importer needs to
resolve references against what it actually created. The cheaper and more
honest of the two is the second — it also fixes the sibling case where `Create`
fails for any other reason:

```go
// internal/bundle/import.go — importAlbums returns the names it actually made.
func importAlbums(…) map[string]bool {
	created := map[string]bool{}
	for i, a := range m.Albums {
		row, err := s.Albums.Create(ctx, websiteID, a.Name)
		if err != nil { …warn…; continue }
		created[a.Name] = true
		…
	}
	report.Albums = len(created)
	return created
}
// importPages then passes `created` where it passes declaredAlbums(m) today,
// so a block naming a name that did NOT become an album is reported by
// missingAlbum instead of being bound to somebody else's row.
```

And, separately, refuse the collision at its source: give `Rename` the same
duplicate-name check `Create` has, or emit a warning at export time when
`exportAlbums` sees two albums resolving to one `nameBySlug` value — silently
writing an ambiguous manifest is the part that cannot be recovered from.

---

### CR-03: The Atom feed serves the raw `[[album:…]]` marker to subscribers

**File:** `internal/public/pagedata.go:369-373`, `internal/public/feed.go:101`

**Issue:** `expandForFeed` expands snippet markers and nothing else. Its own doc
comment states the rule Phase 11 broke:

> "expandForFeed expands snippet markers in feed content, so a subscriber sees
> the same text as a visitor rather than the raw marker."

`internal/album/expand.go:97-99` states the same rule from the other side: "a
visitor must not see the internal syntax on a live page." The feed is a live
public route and it now does. Reproduced:

```xml
<entry>
  <title>Galerie</title>
  <content type="html">&lt;div class=&#34;hc-block hc-galerie hc-spalten-3&#34;&gt;[[album:moebel:0]]&lt;/div&gt;</content>
</entry>
```

Every post carrying an album-backed gallery ships `[[album:<slug>:<n>]]` into
every subscriber's reader, and it also leaks the album's internal address.

**Why the existing tests do not catch it:** there is no feed test in the phase's
test set at all. `album.Expand` is called from exactly one place —
`internal/public/pagedata.go:49` — and the phase's ordering gate
(`snippet.Expand < album.Expand < filterByPlugins < h.responsive`) measures
positions *inside* `pageContent`, so it can never notice a second renderer that
does not call it.

**Fix:** the feed has the website id and the request; give it the same
expansion the page gets.

```go
// internal/public/feed.go — inside HandleFeed, once per feed, not per entry.
albumSet, haveAlbums := album.Set{}, false
if h.albumStore != nil {
	// The feed body is the concatenation of the entries, so load for all of
	// them at once rather than per entry.
	…
}
Content: atomContent{Type: "html", Body: h.expandForFeed(r, website.ID, p.ContentHTML, snippets)},
```

Better still: make one exported helper that owns the whole "stored HTML →
servable HTML" chain (snippets, albums, plugins, responsive) and have
`pageContent`, `expandForFeed` and the admin preview all call it. Three callers
open-coding the pipeline is why only one of them was updated.

---

### CR-04: A gallery block bound to an album is deleted by the next save whenever the album select is not drawn

**File:** `cmd/holzcloud/templates/admin/block_list.html:186`, `internal/admin/page_blocks.go:144-158`, `internal/block/form.go:192-208`, `internal/block/block.go:296-312`, `:400-441`

**Issue:** The album `<select>` is the only carrier of `AlbumSlug` in the block
editor form, and it is guarded by `{{if .Albums}}`. `block.FromForm`
(`form.go:46-120`) rebuilds every block purely from the posted fields — an
absent `bN.album` means `AlbumSlug == ""`. `Empty()` then reports the gallery
as empty and `Clean` drops the block entirely.

Two reachable routes, neither of which requires an error:

**(a) The named album was deleted.** The website still has other albums, so the
select is drawn — but with no `<option>` matching the block's slug, so no
option carries `selected` and the browser submits the first one, `value=""`.
The next save of that page for any unrelated reason deletes the gallery block.

**(b) The website has no albums left, or `siteAlbums` hit a read error.**
`page_blocks.go:152-155` logs the error with `slog.Error` and returns `nil`, so
`{{if .Albums}}` is false and the select is not rendered at all. A transient
database hiccup while opening the page editor destroys every album-backed
gallery on that page at the next save.

Reproduced at the exact boundary the editor crosses:

```
zz_review_test.go:21: parsed AlbumSlug="" Empty=true
zz_review_test.go:23: after Clean: 0 blocks
zz_review_test.go:25: the gallery block was deleted by a save that never touched it
--- FAIL: TestReviewGalleryLosesItsAlbumWhenTheSelectIsNotDrawn (0.00s)
```

This is the same silent deletion 11-05 fixed, arriving through a different
door. The block, its display mode, its column count and its own folded picture
list all go with it, and the flash says "Seite gespeichert".

**Why the existing tests do not catch it:**
`TestAlbumBlockSurvivesClean` (`block_test.go:1152`) hands `Clean` a block that
*has* a slug — the one case the fix covers.
`TestChoosingAnAlbumIsStoredAndComesBackSelected` (`block_album_test.go:97`)
always runs against a website whose album still exists.
`TestNoAlbumsMeansNoSelect` (`block_album_test.go:77`) asserts the select is
absent on a website with no albums — it asserts the *cause* of this bug as
correct behaviour and never follows it to the save.

**Fix:** carry the value even when no control can show it, and never let the
form be the only place a stored value lives.

```html
{{/* cmd/holzcloud/templates/admin/block_list.html */}}
{{if .Albums}}
  …the select as it is today, plus an option for a slug the list no longer
  contains so it stays selected and visibly named as missing…
{{else if .Block.AlbumSlug}}
  {{/* No list to choose from, but the block already names an album: keep it
       rather than posting it away. */}}
  <input type="hidden" name="{{.Prefix}}.album" value="{{.Block.AlbumSlug}}">
  <p class="form-hint text-muted">{{t "Dieses Album ist nicht mehr in der Liste; die Zuordnung bleibt erhalten."}}</p>
{{end}}
```

and make `siteAlbums` distinguish "no albums" from "could not ask": a read
error should propagate to the handler (a 500 the operator sees) rather than
becoming an empty list that silently changes what the form posts.

---

## Warnings

### WR-01: Six operator-facing sentences in the album admin are structurally invisible to `tools/i18n`

**File:** `internal/admin/album.go:149-163`, `:373-385`

**Issue:** The collector reads the *arguments* of `SetFlashError`/`SetFlashSuccess`.
`albumSaid` and `requireOwnPicture` return their German sentences from a helper,
and the call sites pass the helper's result:

```go
web.SetFlashError(h.sm, r.Context(), albumSaid(err))   // album.go:234, 316, 343, 407, 446, 480
web.SetFlashError(h.sm, r.Context(), refused)          // album.go:400, 440
```

Measured: `go run ./tools/i18n -write` on a copy of the tree adds 34 keys, and
none of these six is among them.

```
"Ein Album mit diesem Namen gibt es schon"              0
"Bitte einen Namen für das Album angeben"               0
"Dieses Album ist voll…"                                0
"Dieses Bild gehört nicht zur Mediathek dieser Website" 0
"Das Album oder das Bild gibt es nicht mehr"            0
"Bitte ein Bild auswählen"                              0
```

This is the C-i18n family exactly: the gate will read `0 offen, 0 verwaist`
after 11-07 and these six will still print in German to a French or Italian
operator, with nothing anywhere saying so. It is a regression against the
sibling file — `internal/admin/menu.go:160,167,174,181` passes its literals
directly and they are all in the catalogue.

**Fix:** mark the literals where they are written, so the collector sees them,
and translate at the call site:

```go
func albumSaid(err error) string {
	switch {
	case errors.Is(err, album.ErrNoName):
		return i18n.N("Bitte einen Namen für das Album angeben")
	…
}
// and at each call site
web.SetFlashError(h.sm, r.Context(), web.T(r, albumSaid(err)))
```

(Confirm `i18n.N` is one of `tools/i18n`'s `goFuncs` — `internal/block/render.go:400-410`
uses it for exactly this purpose and those three keys *are* collected.)

### WR-02: A store error or an unwired store serves the raw marker instead of nothing

**File:** `internal/public/pagedata.go:47-51`, `:84-95`

**Issue:** `albumSet` returns `(album.Set{}, false)` on a query failure and
`pageContent` then skips `album.Expand` entirely, so `[[album:moebel:0]]` is
served to the visitor. The same happens when `h.albumStore == nil`. The doc
comment argues this is the safe choice ("A failing album must cost its own
block and never the page") but it is the opposite: it costs the page's
credibility while the block would have cost only itself.

The analog does the other thing. `loadSnippets` (`pagedata.go:349-359`) logs and
returns an **empty map**, and `snippet.Expand` still runs — so every marker is
replaced with `""` and none survives. `album.Expand` already does the right
thing with a zero `Set` (`expand.go:110-113` returns `""` for an unknown slug);
it simply is not called.

**Fix:**

```go
// pagedata.go — always expand, even when the load failed. A zero Set expands
// every marker to nothing, which is what the snippet path already does.
if h.albumStore != nil {
	set, _ := h.albumSet(r, websiteID, body)   // errors are logged inside
	body = album.Expand(body, set)
} else if block.HasAlbumMarker(body) {
	body = album.Expand(body, album.Set{})
}
```

### WR-03: The admin preview shows the raw marker instead of the gallery

**File:** `internal/admin/preview.go:236-247`

**Issue:** `previewPageContent` writes `template.HTML(pg.ContentHTML)` with no
expansion of any kind. An editor who binds a gallery to an album and presses
"Vorschau" sees `[[album:sommer-2025:0]]` — the one screen that exists to show
what will be published shows the internal syntax instead.

(The preview already had this gap for snippet markers, so this is a widening
rather than a new class. It matters more here because the album *is* the block's
only content: the snippet case shows a page with one sentence missing, the album
case shows a page with a bracketed token where the pictures are.)

**Fix:** route the preview through the same pipeline the public page uses — see
CR-03's closing suggestion. At minimum, expand albums and snippets in
`previewPageContent`, which needs the handler's stores rather than a package
function.

### WR-04: `importAlbumSlug` derives the album address a second way, from an unnormalised name

**File:** `internal/bundle/blocks.go:129-134` vs `internal/album/store.go:111-120`, `:145-166`

**Issue:** `internal/album`'s package comment claims the mitigation outright:

> "This store makes exactly ONE call to `page.Slugify`, in `Create`, and
> everything that needs an album's slug goes through it. The bundle importer of
> plan 11-06 calls `Create`; it does not derive its own."

The second half is false. `importAlbumSlug` calls `page.Slugify(name)` on the
**raw** manifest name, while `Create` calls it on `normalizeName(name)`, which
truncates at `MaxNameLength = 60` runes. For any manifest album name longer
than 60 runes the two derivations disagree, so `Create` makes an album at one
address and every gallery reference is written to a different one. The gallery
renders nothing, and `missingAlbum` reports nothing, because the name *was*
declared. This is exactly the "two callers deriving one key two ways" hazard
`internal/term/store.go:318-328` warns about, which the album store's own
comment quotes.

An honest export cannot produce a name that long (the store caps it), but a
manifest is a file anybody can edit and the threat register treats it as
untrusted (T-11-27).

**Fix:** apply the same normalisation on both sides, or better, remove the
second derivation entirely by having `importAlbums` return the slug it made
per manifest name and having `importBlocks` look it up — which is the same
change CR-02 needs.

```go
// blocks.go
func importAlbumSlug(name string, albumSlugs map[string]string) string {
	return albumSlugs[name]   // filled by importAlbums from row.Slug
}
```

### WR-05: No handler-level cross-website scope test for the eight album write handlers

**File:** `internal/admin/album_test.go:142-160`

**Issue:** The KB's recurrence guard for this defect family is explicit: *"Two
websites, a resource on B, an editor whose `user_websites` grants only A, drive
the request through the real middleware chain, and assert the effect (the row
is unchanged) as well as the status."* The phase has exactly one such test,
`TestAlbumEditFromAnotherWebsiteIs404`, and it (a) covers only the **GET**
handler, (b) calls `f.h.HandleAlbumEdit` directly rather than driving the real
middleware chain, and (c) asserts a status and a body, not an effect. Nothing
in `internal/admin` drives `HandleAlbumUpdate`, `HandleAlbumDelete`,
`HandleAlbumItemCreate`, `HandleAlbumItemUpdate`, `HandleAlbumItemDelete` or
`HandleAlbumItemReorder` against another website's album.

The store closes this, and I could not construct an escape — so this is a
gap in the *guard*, not a live defect. But `internal/menu/store.go` also
"looked reviewed" until four handlers were found reaching another website's
navigation, and the three existing `*_scope_test.go` files exist because
signatures alone were not believed.

**Fix:** add `internal/admin/album_scope_test.go` in the shape of
`internal/admin/product_scope_test.go`: for each of the eight write handlers,
website A in the path and website B's album/item behind it, driven through the
real chain, asserting 404 **and** that the row on B is unchanged — with the
negative control (the same action on the own website succeeds), or a fix that
refuses everything passes.

### WR-06: The bundle importer can put a video into an album, where it renders as a broken `<img>`

**File:** `internal/album/store.go:326-361`, `:477-491`, `internal/bundle/import.go:382-393`, `internal/block/render.go:548-600`

**Issue:** `Store.AddItem` calls `requireOwnMedia`, which checks only
`website_id`. The admin handler's `requireOwnPicture`
(`internal/admin/album.go:373-385`) additionally checks `m.IsImage()` — and its
comment says why: "it is also what catches a film chosen where a photo belongs,
which the store, correctly, has no opinion about." But `importAlbums` calls
`AddItem` directly, bypassing the handler, and resolves the picture through
`mediaByName[it.Media]`, which can name an MP4 in the archive's media list.

`GalleryItems` has no `Film` guard — unlike the `TypeImage` arm at
`render.go:99-102`, which does `if !ok || img.Film { return }` — so the item
goes through `imgTag` and the visitor gets `<img src="/media/1/film.mp4">`.

**Fix:** either give `AddItem` the opinion (`AND mime_type LIKE 'image/%'` in
`requireOwnMedia`, with a named `ErrNotAPicture`), or give `GalleryItems` the
guard the sibling arm already has:

```go
for j, it := range items {
	img, ok := look(it.MediaID)
	if !ok || img.Film {   // a film in a picture grid is a broken <img>
		continue
	}
	…
```

The second is cheaper and also covers the inline-gallery path, which has the
same hole through a hand-edited archive.

### WR-07: `AddItem` reads the sort order from the read pool and writes on the write pool

**File:** `internal/album/store.go:335-352`

**Issue:** `COUNT(*)` and `MAX(sort_order)` are read on `s.DB.Read` and the
`INSERT` uses `lastOrder+1` on `s.DB.Write`. Under WAL these are different
snapshots, so two concurrent adds to one album can both read `MAX = 5` and both
insert `sort_order = 6`. `Pictures` then orders by `sort_order, id`, which is
still deterministic — but `SwapSortOrder` between two rows sharing a
`sort_order` writes each the other's value, i.e. writes nothing, and the arrow
button becomes a control that reports "Reihenfolge geändert" and changes
nothing. The same window lets the `MaxItems` cap be exceeded by one.

**Fix:** compute the next order inside the INSERT, so one statement decides:

```sql
INSERT INTO album_items (album_id, media_id, alt, caption, sort_order)
SELECT $1, $2, $3, $4,
       COALESCE((SELECT MAX(sort_order) FROM album_items WHERE album_id = $1), -1) + 1
 WHERE EXISTS (SELECT 1 FROM albums WHERE id = $1 AND website_id = $5)
   AND (SELECT COUNT(*) FROM album_items WHERE album_id = $1) < 120
```

(with the cap as a bound parameter so `MaxItems` stays the single source), or
run the read and the write in one `BeginTx` on the write pool.

---

## Info

### IN-01: `block.HasAlbumMarker` is exported but used only inside its own package

**File:** `internal/block/render.go:478-480`

Used at `render.go:489` and `:508` and in `block_test.go`. Nothing outside
`internal/block` calls it — `internal/album/expand.go` wraps
`AlbumMarkerSlugs` and `ReplaceAlbumMarkers` only. **Fix:** unexport it to
`hasAlbumMarker`, or use it in WR-02's fix, where a public caller finally exists.

### IN-02: `album.Store.BySlug` has no production caller

**File:** `internal/album/store.go:183-187`

Called only from `internal/album/store_test.go:98,235` and
`internal/bundle/bundle_test.go:937,943,1125`. The bundle importer it was
presumably written for goes through `Create` instead. **Fix:** delete it, or
give it the caller CR-02's fix needs (resolving a block's reference against
what actually exists on the target website).

### IN-03: `TestSpecRequiresTheBlockStylesheet` counts a substring, not a placement

**File:** `internal/tmplspec/spec_test.go:216-231`

`strings.Count(spec, "/assets/bausteine.css") >= 3` is satisfied by three
mentions anywhere in the document, including three in one paragraph. The test's
own comment names three specific sections (§4's table, §3's layout example,
§11's minimal template) and asserts none of them. It currently reads 4, so the
threshold is already one away from meaning what it says. The sibling
`TestEveryShippedThemeLinksTheBlockStylesheet` is the honest gate and does fail
on an empty glob, which is the right shape. **Fix:** assert the link inside the
two `<head>` examples specifically, e.g. by counting occurrences that are part
of a `<link rel="stylesheet" href="…">` tag.

### IN-04: The album POST handlers set `HX-Redirect` without `Vary: HX-Request`

**File:** `internal/admin/album.go:242-247`, `:321-326`, `:348-353`, `:413-418`, `:453-458`, `:485-490`, `:551-556`

CLAUDE.md requires `Vary: HX-Request` on any handler that branches on the
header. `web.RenderAdmin` sets it (`internal/web/render.go:264`) but these
paths return before rendering. `internal/admin/menu.go` has the same omission,
so this is a copied pattern rather than a new one, and a 303/200-with-header
is not usefully cacheable — hence Info. **Fix:** set the header in the shared
`ErrHandler` wrapper so no handler has to remember.

### IN-05: An out-of-range block position in a marker silently removes the gallery

**File:** `internal/block/render.go:511-518`

`strconv.Atoi(parts[2])` on `[[album:x:99999999999999999999]]` returns an error
and `ReplaceAlbumMarkers` returns `""` — the block vanishes with no log line.
Only reachable from hand-edited `content_html`, and vanishing is the safe
outcome, but it is worth a `slog.Warn` so the one operator it ever happens to
can find out why.

### IN-06: Nothing asserts that one named album out of many loads only one

**File:** `internal/album/store.go:527-577`, `internal/album/expand_test.go:128`

The prompt's requirement — "a website with fifty albums must not load fifty to
render one page" — is held by the `a.slug IN (…)` clause, which I verified by
reading the SQL. The tests cover only the zero case
(`TestLoadForIssuesNoQueryWhenTheHTMLNamesNoAlbum`,
`TestPageWithNoAlbumIssuesNoAlbumQuery`). A test that seeds three albums, names
one, and asserts the returned `Set` holds exactly that one would make the
property observable rather than inferred.

---

## Notes on the two recorded plan defects

Both were resolved correctly and neither needs re-opening.

**11-05's `Album:` grep gate** is genuinely unsatisfiable as written — `Album:`
cannot match inside `AlbumSlug:` because the next character is `S`, and the two
names differ by the plan's own acceptance criteria. The substance the gate
protects is present: `blocks.go:43` carries `Album: albumNameBySlug[b.AlbumSlug]`
on the way out and `blocks.go:96` carries `AlbumSlug: importAlbumSlug(…)` on the
way in. The replacement gate (`\bAlbum:` and `AlbumSlug:` at 1 each) measures
the same intent.

**11-06's ordering gates** were worked around by keeping the identifiers
`importMenus`/`exportPages` out of the call-site comments and putting the full
argument on the function doc comments instead. That is the better placement, and
the constraint itself holds: `Import` calls `importMedia` (`import.go:105`) →
`importAlbums` (`:137`) → `importPages` (`:138`) in that order, and
`buildManifest` calls `exportAlbums` (`export.go:156`) before `exportPages`
(`:170`).

I did **not** re-report `internal/bundle`'s four new `fmt.Sprintf` warnings —
they are on the ledger as `.planning/WINDOWS.md` entry 6, they match thirty
siblings, and closing them needs the operator's locale threaded through
`bundle.Import`, which is outside this phase.

---

_Reviewed: 2026-09-08T01:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
