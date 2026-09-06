# Cross-website write in the menu item handlers

**Date:** 2026-09-06
**Status:** proven, fixed, verified
**Debug session:** `.planning/debug/menu-item-cross-website-write.md`

The claim was correct, and worse than reported in one respect: there are
**four** vulnerable handlers, not three. `HandleMenuItemReorder` carries the
identical defect and writes (it swaps `sort_order`), so a fix covering only the
three named handlers would have closed three quarters of the hole.

---

## 1. The proof — literal failing output before the fix

Test file: `internal/admin/menu_scope_test.go`, committed on its own in
`de4a1ce` so the repository records the proof before the remedy.

The test provisions websites A and B, gives each a menu with two entries,
creates an editor with a `user_websites` row for **A only**, and drives the real
chain — session -> `auth.RequireWebsiteAccess` -> routed mux — exactly as
`cmd/holzcloud/main.go` wires it. Nothing is stubbed.

```
=== RUN   TestMenuItemCreateRefusesAForeignWebsitesMenu
    menu_scope_test.go:174: create: status 303, want 404 or 403 — the write reached a foreign website
    menu_scope_test.go:178: website B's menu grew from [Erster Eintrag Zweiter Eintrag] to [Erster Eintrag Zweiter Eintrag Eingeschleust] — a foreign entry was created
--- FAIL: TestMenuItemCreateRefusesAForeignWebsitesMenu (0.53s)
=== RUN   TestMenuItemUpdateRefusesAForeignWebsitesMenu
    menu_scope_test.go:188: update: status 303, want 404 or 403 — the write reached a foreign website
    menu_scope_test.go:195: website B's entry was retitled from "Erster Eintrag" to "Eingeschleust"
    menu_scope_test.go:198: website B's entry was repointed from "https://example.org/1" to "https://attacker.example/"
--- FAIL: TestMenuItemUpdateRefusesAForeignWebsitesMenu (0.49s)
=== RUN   TestMenuItemDeleteRefusesAForeignWebsitesMenu
    menu_scope_test.go:209: delete: status 303, want 404 or 403 — the write reached a foreign website
    menu_scope_test.go:216: website B's entry was deleted through website A's address
--- FAIL: TestMenuItemDeleteRefusesAForeignWebsitesMenu (0.49s)
=== RUN   TestMenuItemReorderRefusesAForeignWebsitesMenu
    menu_scope_test.go:229: reorder: status 303, want 404 or 403 — the write reached a foreign website
    menu_scope_test.go:233: website B's menu was reordered from [Erster Eintrag Zweiter Eintrag] to [Zweiter Eintrag Erster Eintrag]
--- FAIL: TestMenuItemReorderRefusesAForeignWebsitesMenu (0.48s)
=== RUN   TestMenuItemHandlersStillServeTheirOwnWebsite
--- PASS: TestMenuItemHandlersStillServeTheirOwnWebsite (0.50s)
=== RUN   TestMenuItemHandlersRefuseAMenuIdFromAnotherWebsiteEvenForAnAdmin
    menu_scope_test.go:309: delete with a mismatched menu id: status 303, want 404
    menu_scope_test.go:317: website B's entry was deleted through website A's address
--- FAIL: TestMenuItemHandlersRefuseAMenuIdFromAnotherWebsiteEvenForAnAdmi (0.48s)
FAIL
FAIL	github.com/holzcloud/holzcloud-cms/internal/admin	3.299s
```

Every write returned **303 See Other** — the success redirect. Nothing refused,
nothing logged, nothing to notice afterwards.

The sharpest line is the update case: **website B's navigation entry was
repointed at `https://attacker.example/`**. That is not tampering with another
tenant's data in the abstract; it is redirecting their visitors to an address
the attacker picks, from a link that still reads as the victim's own navigation.

`TestMenuItemHandlersStillServeTheirOwnWebsite` **passed before the fix and
still passes after it.** That is deliberate: it is the guard against an
over-broad fix. Without it, a change that simply refused every request would
have turned all four failures green and looked like success.

### Why the middleware could not save this

`internal/auth/middleware.go:99-115` reads only the leading path segment, and
its own comment says so:

> It therefore reads the address rather than the route: everything under
> `/admin/websites/<number>` belongs to that website, whatever the rest of the
> path turns out to be.

Sound design for what it does — one guard over sixty-odd routes rather than
sixty checks that can each be forgotten. But it makes the middleware
structurally incapable of noticing that a *tail* id belongs elsewhere. The
fixture asserts the premise directly: `NewWebsiteAccessLookup` returns true for
the editor on A and false on B, so the request is admitted on A's address and
the handler does the rest.

---

## 2. The fix, and why it went where it did

Commit `5e453a9`. The check now lives in two helpers in
`internal/admin/menu.go`, and **all seven** menu handlers route through them:

- `menuOfWebsite(ctx, websiteID, menuID)` — the menu, or nil if it belongs elsewhere
- `itemOfWebsite(ctx, websiteID, menuID, itemID)` — the item, only if it hangs in
  that menu *and* that menu belongs to that website

`HandleMenuItemCreate` had no lookup at all, so it gained an explicit one.

### Handler level, not store level — the reason

The brief framed this as handler-vs-store with blast radius as the tiebreaker.
Measuring both changed the picture, in both directions:

**The blast-radius argument for the handler is weaker than it looks.** Nearly
every caller of the unscoped menu store methods is `internal/admin/menu.go`
itself — `GetMenu` x3, `GetItem` x3, `UpdateItem`, `DeleteItem`, `SwapSortOrder`
x1 each — plus `starter.go` (`CreateMenu`/`CreateItem`). "Touches every caller"
would have meant one file and a seeder, not a sweep.

**The decisive argument is schema, not blast radius.** `terms` has a
`website_id` column (`00015_terms.sql:14`), which is why
`internal/term/store.go:284-311` can put `website_id` in every `WHERE`.
**`menu_items` has no such column** (`00045_pages_locale_unique.sql:151-161`) —
it reaches the website only indirectly, through `menus`. So the term shape does
not transfer literally: every item statement would need a JOIN or a correlated
subquery, in five methods, inside a security patch. Worth making, but not while
closing an active hole, and not with correctness that is obvious on review.

So: handler level, but **not** four copies of an `if`. Four copies is precisely
how the defect arose — the three menu handlers had the check, the four item
handlers did not, and nothing pointed it out. One helper means the rule has one
home, which is the same argument the middleware makes for itself.

The store now says so out loud, as required — `internal/menu/store.go`, on the
`Store` type:

> **This store does not enforce website scoping, and a caller must.** [...]
> Whoever adds a handler under `/admin/websites/{id}/menus` must therefore check
> that the menu belongs to the website in the address, via the admin package's
> `menuOfWebsite` / `itemOfWebsite` helpers. [...] The term store solves the same
> problem the safer way [...] and that remains the better shape. It is not copied
> here yet because `menu_items` has no `website_id` column of its own.

**Recommended follow-up (not done here):** give `internal/menu/store.go` the
term shape via a JOIN through `menus`, and delete the warning comment when it is
no longer true. That converts the guarantee from "the next author reads a
comment" into "the next author cannot express the bug".

---

## 3. Neighbour audit — every admin route with a website id plus a second id

39 routes matched `/admin/websites/{id}/.../{secondID}` via the direct
`ErrHandler` form, plus 12 more registered as `.Handle(...requireAdmin(...))`,
which the first pass missed — the content-kind, block-kind and field screens.
Both forms are included.

**Result: the four menu item handlers were the only dirty ones. Everything else
is clean.** Named individually, since a clean list is the point.

### Dirty — fixed in this session

| Handler | Location | What was missing |
|---|---|---|
| `HandleMenuItemCreate` | `internal/admin/menu.go:285` | no menu lookup at all |
| `HandleMenuItemUpdate` | `internal/admin/menu.go:364` | `item.MenuID != menuID` only |
| `HandleMenuItemDelete` | `internal/admin/menu.go:426` | `item.MenuID != menuID` only |
| `HandleMenuItemReorder` | `internal/admin/menu.go:467` | `item.MenuID != menuID` only — **not in the original report** |

### Clean — verified, with the mechanism that makes each safe

**Pages** — all compare `p.WebsiteID != websiteID` after `GetPage`:
`HandlePageEdit` (`page.go:498`), `HandlePageDelete` (`page.go:679`),
`HandlePageStatusToggle` (`page.go:712`), `HandlePageInlineEditTitle`
(`page.go:782`), `HandlePageInlineEditSave` (`page.go:809`),
`HandlePageDuplicate` (`page_bulk.go:115`), `HandlePageReview`
(`page_bulk.go:162`), `HandlePageShare` (`sharelink.go:29`),
`HandlePageTranslate` (`page_locale.go:139`), `HandleTrashRestore`
(`page_history.go:174`).

**Page revisions** — the same two-link chain the menu items were missing, done
correctly. Both check `rev.PageID != pageID` **and** `p.WebsiteID != websiteID`:
`HandlePageRevisionRestore` (`page_history.go:64`), `HandlePageRevisionLabel`
(`page_compare.go:164`), `HandlePageRevisions` (`page_history.go:23`),
`HandlePageRevisionCompare` (`page_compare.go:56`).
`HandlePageRevisionLabel` even carries a comment describing this exact attack —
the project knew the shape here and simply missed it in menus.

**Media** — `lookupMedia` (`media.go:302-323`) checks `m.WebsiteID != websiteID`
centrally, the same one-helper shape adopted for menus: `HandleMediaMeta`
(`media.go:251`), `HandleMediaDelete` (`media.go:272`), `HandleMediaCrop`
(`media_crop.go:60`), `HandleMediaCropSave` (`media_crop.go:89`).

**Media into a page** — `HandlePageInsertMedia` (`media_picker.go:68`) checks
*both* ids, including the form-supplied `media_id`: `p.WebsiteID != websiteID`
and `m.WebsiteID != websiteID`.

**Snippets** — `HandleSnippetDelete` (`snippet.go:342`): `sn.WebsiteID != websiteID`.

**Terms** — store-scoped, the term shape: `HandleTermRename` (`term.go:48`) and
`HandleTermDelete` (`term.go:76`) call `terms.Rename/Delete(ctx, websiteID, id)`,
which put `website_id` in the `WHERE` and, for `Rename`, error on
`RowsAffected() == 0` (`term/store.go:284-311`).

**Redirects** — `HandleRedirectDelete` (`redirect.go:161`) calls
`pages.DeleteRedirect(ctx, websiteID, id)` — scoped in SQL.

**Saved views** — `HandleSavedViewDelete` (`listview.go:275`) is the strictest of
all: `DELETE ... WHERE id = $1 AND user_id = $2 AND website_id = $3`, scoped per
person as well as per website.

**Products** — `HandleProductForm` (`product.go:141`) and `HandleProductDelete`
(`product.go:298`) both check `p.WebsiteID != ws.ID`.

**Orders** — `HandleOrderDetail` (`order.go:179`) and `HandleOrderDocument`
(`orderdoc.go:93`) look up by `orders.ByNumber(ctx, ws.ID, number)`; the status
write is `SetStatus(ctx, ws.ID, order.ID, ...)`. Scoped at the lookup.

**Preview** — `HandlePreviewPage` (`preview.go:65`) resolves via
`GetPageBySlug(ctx, ws.ID, slug)`. Read-only and scoped.

**Content kinds, block kinds, fields** (the `requireAdmin`-wrapped routes the
first pass missed) — all store-scoped with `WHERE id = $1 AND website_id = $2`,
verified in the SQL, not just in the signature:
`HandleKindDelete` (`kind.go:164`), `HandleKindMove` (`kind.go:202`) ->
`kind/store.go:147,171`;
`HandleBlockTypeDelete` (`blocktype.go:123`), `HandleBlockTypeMove`
(`blocktype.go:144`) -> `block/store.go:188,198`;
`HandleFieldDelete` (`field.go:255`), `HandleFieldMove` (`field.go:281`) ->
`field/store.go:598,611`.

### What the audit says about the codebase

The dominant pattern here is the **store-scoped** one — `kind`, `block`, `field`
and `term` all put `website_id` in the `WHERE`. `menu` is the outlier that takes
bare primary keys, and it is the one that had the hole. That is an argument for
the follow-up above, and the reason the warning comment sits on the `Store` type
rather than on the individual methods.

---

## 4. Gates

| Gate | Result |
|---|---|
| `go build ./...` | clean, exit 0 |
| `go vet ./...` | clean, exit 0 |
| `gofmt -l .` | empty |
| `go test ./...` | **exit 0** |

```
$ go run ./tools/i18n
1269 Zeichenketten im Quelltext
de-CH.json   55 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1158 übersetzt, 111 offen, 0 verwaist
es.json      1158 übersetzt, 111 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1158 übersetzt, 111 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1158 übersetzt, 111 offen, 0 verwaist
```

**0 verwaist** in every catalogue and nothing lost: the change adds no
user-facing strings, only Go comments and test failure messages, neither of
which is a translation key.

One note on the suite: an intermediate full run failed on
`TestCSVSecondImportRenamesAndReportsIt`. That test did not exist in `HEAD` and
had already disappeared from the working tree by the time it was investigated —
it belongs to the concurrent `internal/admin/csvimport.go` work, which was
mid-edit. It was not touched here, and the final `go test ./...` is exit 0.

---

## 5. Commits

| Hash | Files | Purpose |
|---|---|---|
| `de4a1ce` | `internal/admin/menu_scope_test.go` (+319) | the failing test — the proof |
| `5e453a9` | `internal/admin/menu.go` (+75/-12), `internal/menu/store.go` (+20) | the fix |

Both verified with `git show --stat`; only the named files are in each. Nothing
belonging to the developer (`README.md`, `docs/`, `.github/`, `CONTRIBUTING.md`,
`SECURITY.md`) or to the concurrent agent (`internal/admin/csvimport.go`,
`cmd/holzcloud/templates/admin/csv_*.html`, `internal/csvimport/`) was staged,
modified or read into either commit.

This report is deliberately left uncommitted.

---

## 6. Prevention

**Why no gate caught it.** No test existed for cross-website access on any
route. The rule was carried entirely by convention — each handler author
remembering — and by a middleware whose documented contract explicitly does not
cover it. Review is the only gate that could have caught it, and review compared
the item handlers against nothing.

**Recurrence guards now in place:**

1. `internal/admin/menu_scope_test.go` — six tests, including the
   over-broad-fix guard and an admin-path case, since an admin passes the
   middleware everywhere and so exercises the handler check in isolation.
2. The warning on the `menu.Store` type, aimed at the next person to add a
   handler under this route.
3. One helper per rule instead of one `if` per handler, so the next menu handler
   inherits the check by calling the only lookup available.

**Still open:** nothing generic prevents this class on a *new* resource. The
durable fix is the store shape — a handler that cannot name a row without naming
its website cannot leak one. `kind`, `block`, `field` and `term` already work
that way; `menu` should follow.
