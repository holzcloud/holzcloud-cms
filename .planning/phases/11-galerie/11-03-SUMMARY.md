---
phase: 11-galerie
plan: 03
subsystem: admin
tags: [album, admin, htmx, routing, authorization, gal-03, gal-05, gal-07]
status: complete

requires:
  - internal/album (Store, Album, Picture, the five named errors)
  - internal/admin (ImageFieldView, web.RenderAdmin, the menu handler's shape)
  - cmd/holzcloud/templates/admin/block_list.html (the block_bild partial)
provides:
  - internal/admin/album.go — nine HandleAlbum… handlers, AlbumListData, AlbumEditData, AlbumItemView
  - Handler.SetAlbumStore — the fifth setter of that shape
  - two admin screens — album_list, album_edit, both inside the base layout
  - nine routes on adminProtectedMux under /admin/websites/{id}/albums
  - the album filed as content in both authorization lists
affects:
  - 11-05 (the public expansion reads routerDeps.albumStore)
  - 11-06 (the bundle round trip creates and renames through the same store)
  - 11-07 (23 new German source strings to translate; the browser pass)

tech-stack:
  added: []
  patterns:
    - "the store arrives through a setter, never as a nineteenth positional argument"
    - "a POST computes one redirect target, then branches on HX-Request once"
    - "the store's named errors become sentences through errors.Is"
    - "the picture chooser is ImageFieldView, reused, so block_bild works unchanged"
    - "a reorder past either end is not an error — the list as it now is"

key-files:
  created:
    - internal/admin/album.go
    - internal/admin/album_test.go
    - cmd/holzcloud/templates/admin/album_list.html
    - cmd/holzcloud/templates/admin/album_edit.html
  modified:
    - internal/admin/handler.go
    - internal/web/render.go
    - cmd/holzcloud/templates/admin/base.html
    - cmd/holzcloud/templates/admin/icons.html
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go

decisions:
  - "The URL segment is `albums`, English, beside pages/media/menus/tags/snippets — nothing is stored, so the GLOSSARY's stored-value rule has nothing to bite on"
  - "Two screens, not snippet_list's one — an album has children that are added, described and moved"
  - "The handler's website check is a second strap: the store puts website_id in every WHERE, so a handler that forgot could not compile"
  - "A move past either end is a success flash saying the order already stands so — block.Apply's rule, not an error"
  - "The routerDeps local in newRouter was NOT added: Go rejects an unused local and a `_ =` discard would read as an oversight. Plan 11-05 adds it when it has a reader"
  - "A test CAN see a page missing from layoutPageNames. The plan said none could; it was wrong, and the test is mutation-proved"

metrics:
  duration: ~65 min
  completed: 2026-09-07
  tasks: 3
  commits: 4

actuals:
  tokens: 14144
  tasks: 3
  commits: 4
---

# Phase 11 Plan 03: The Album Admin Area Summary

The tax every named, website-owned thing with an ordered child list pays in this
codebase, paid in full: a setter rather than a nineteenth constructor argument,
nine handlers that cannot reach another website's row because no signature lets
them, two screens that render inside the base layout, and an album filed as
content in both authorization lists rather than in neither or in the wrong one.

## What was built

**`internal/admin/album.go`** — nine `HandleAlbum…` handlers, `AlbumListData`,
`AlbumEditData` and `AlbumItemView`, plus three small shared pieces:
`albumScope` (the leading `/admin/websites/{id}` and the nil-store 404),
`albumFromPath` (both ids of the address at once) and `albumSaid` (one of the
store's five named errors turned into a sentence, through `errors.Is`).

The file's opening comment names three things deliberately **not** copied from
`internal/admin/menu.go` and says why for each: `isValidLocationKey` (an album
has no location key), `menuOfWebsite`'s role as the *only* guard (the store
scopes every statement, so this is the second strap), and the substring match on
SQLite's wording at `menu.go:172` (which is exactly what plan 11-02's named
errors exist to end).

**`internal/admin/handler.go`** — one field and `SetAlbumStore`, the fifth
setter of that shape. `NewHandler`'s signature is byte-for-byte unchanged.

**The two screens.** `album_list.html` opens with a comment saying which of the
two shapes was chosen and why the cheaper `snippet_list.html` one-screen form
was passed over: an album has children that are added, described and moved, and
a list of albums each with its own expanded picture list is a screen nobody can
read. `album_edit.html` calls `{{template "block_bild" .Image}}` on every row —
the block editor's own chooser, reached because `block_list.html` is already a
shared partial. Each row's `ImageFieldView` carries `Prefix: "bild"` (so the
posted names are constant and the partial works unchanged) and a per-row `ID`
(so two labels on one page do not point at the same element).

**`internal/web/render.go`** — `album_list` and `album_edit` in
`layoutPageNames`, 48 entries to 50. **And a test for it** — see the finding
below.

**`cmd/holzcloud/main.go`** — the store, the setter, the `routerDeps` field, and
nine routes in the two-level `{id}` / `{albumID}` shape, eight distinct paths.

## Task-by-task

| Task | What | Commit |
|---|---|---|
| 1 RED | Eight failing tests; `go build ./...` stayed green throughout | `2aa521c` |
| 1 GREEN | `album.go`, the field and the setter — all nine gates green | `ad99438` |
| 2 | Two templates, the nav item, the icon, `layoutPageNames`, and the layout test | `38a4baa` |
| 3 | The wiring, nine routes, both authorization lists | `b5ce7bd` |

`git show --stat` on each: `2aa521c` one file, `ad99438` two, `38a4baa` six,
`b5ce7bd` two. Nothing of plan 11-04's (`internal/block/`, `block_list.html`,
`bausteine.css`), nothing of the developer's, no `.planning/STATE.md`, no
`internal/i18n/locales/*.json`.

## The counting table, measured against the post-change tree

Every baseline was re-measured on this tree before any change and matched the
plan exactly, all eight.

| What | Baseline | Plan predicted | Measured after | Verdict |
|---|---|---|---|---|
| `adminProtectedMux.Handle` | 150 | 159 | **159** | as predicted |
| `adminOnly` rows | 19 | 19 | **19** | unchanged, as required |
| editor-open rows | 5 | 6 | **6** | as predicted |
| distinct album route paths | — | 8 | **8** | as predicted |
| admin templates | 66 | 68 | **68** | as predicted |
| `layoutPageNames` entries | 48 | 50 | **50** | as predicted |
| `{{define "icon-` blocks | 26 | 27 | **27** | as predicted |
| `nav-item` anchors | 26 | 27 | **27** | as predicted |
| packages under `internal/` | 41 | 41 | **41** | unchanged, as required |
| files using `BeginTx` | 15 | 15 | **15** | unchanged, as required |
| strings in source | 1280 | measured, not predicted | **1306** | **+26 tree-wide, +23 mine** |

### The shared-tree counters, measured in isolation

Plan 11-04 ran in this same working tree at the same time, so a whole-tree
number is not a statement about this plan. Every row above was therefore also
measured on a copy of `HEAD` with **only this plan's contributions removed** —
the two new templates and `album.go` deleted, `base.html` and `icons.html`
restored to `7ab032c`:

```
=== at HEAD (shared tree) ===        === isolation copy, this plan removed ===
admin templates: 68                  admin templates: 66
packages:        41                  packages:        41
BeginTx files:   15                  BeginTx files:   15
icons:           27                  icons:           26
nav-item:        27                  nav-item:        26
i18n:            1306                i18n:            1283
```

Removing this plan restores every baseline to the digit. That is the honest
form of a count in a shared tree: *removing this work must not change it.*

### The i18n row, and it is deliberately not a prediction

```
$ go run ./tools/i18n
1306 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1277 übersetzt, 29 offen, 0 verwaist
es.json      1277 übersetzt, 29 offen, 0 verwaist
fr-CH.json    4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1277 übersetzt, 29 offen, 0 verwaist
it-CH.json    9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1277 übersetzt, 29 offen, 0 verwaist
```

The count went **up**, which is what the gate asks: 1280 → 1306. Of the 26,
**23 are this plan's** (isolation measurement above) — two screens' worth of
labels plus the handler's nine flashes — and 3 are plan 11-04's, arriving in the
same tree. `offen` went 3 → 29 for the same reason and **that is expected, not a
regression**: the three were plan 11-01's lightbox labels, and plan 11-07 owns
the catalogues. Nothing under `internal/i18n/locales/` was touched here.

## Verification — literal output

```
$ go build ./... && go vet ./internal/admin/ && gofmt -l internal/admin/
   exit 0, gofmt printed nothing

$ go test ./internal/admin/ -run Album -v
   --- PASS: TestAlbumEditFromAnotherWebsiteIs404 (0.53s)
   --- PASS: TestAlbumCreateWithADuplicateNameSaysSo (0.50s)
   --- PASS: TestAlbumCreateWithABlankNameSaysSo (0.52s)
   --- PASS: TestAlbumPostAnswersHXRedirectForHtmxAnd303Otherwise (0.51s)
   --- PASS: TestAlbumItemCreateRefusesAnotherWebsitesMedia (0.52s)
   --- PASS: TestAlbumDeleteRemovesItsPictures (0.52s)
   --- PASS: TestAlbumScreensAre404WithoutAStore (0.50s)
   --- PASS: TestAlbumScreensRenderInsideTheBaseLayout (0.56s)
   PASS   ok  github.com/holzcloud/holzcloud-cms/internal/admin

$ go test ./internal/admin/ -run 'Album|Reorder' -v
   --- PASS: TestReorderAtTheEndIsNotAnError (0.52s)      (see divergence 1)

$ grep -c 'func (h \*Handler) HandleAlbum' internal/admin/album.go        9
$ python3 … HX-Request / HX-Redirect (comments stripped)               7 7
$ python3 … UNIQUE constraint / errors.Is( (comments stripped)         0 5
$ python3 … isValidLocationKey (comments stripped)                       0
$ grep -c 'func (h \*Handler) SetAlbumStore' internal/admin/handler.go    1
$ git diff -U0 -- internal/admin/handler.go | grep 'func NewHandler' | wc -l   0
$ grep -c 'ImageFieldView' internal/admin/album.go                       7

$ python3 … layoutPageNames parsed                            50 True True
$ ls cmd/holzcloud/templates/admin/*.html | wc -l                       68
$ grep -c 'define "icon-' …/icons.html                                  27
$ grep -c 'class="nav-item' …/base.html                                 27
$ grep -c 'block_bild' …/album_edit.html                                 3
$ python3 … submit buttons vs hx-disabled-elt
    album_list 2 2
    album_edit 6 6
$ go build ./... && go test ./internal/web/ ./internal/admin/ ./cmd/holzcloud/
    ok internal/web 4.532s   ok internal/admin 65.836s   ok cmd/holzcloud 3.342s

$ grep -c 'adminProtectedMux.Handle' cmd/holzcloud/main.go             159
$ sed -n '/adminOnly := \[\]struct/,/^\t}$/p' … | grep -c '{"'          19
$ sed -n '/Content routes stay open to editors/,/^\t} {/p' … | grep -c '{"'   6
$ grep -c '/admin/websites/1/albums' cmd/holzcloud/main_test.go          2
$ grep -oE '/admin/websites/\{id\}/albums[^"]*' … | sort -u | wc -l      8
$ go test ./cmd/holzcloud/ -run 'TestRouteAuthorization|TestAdminRoutesRequireASession' -v
   --- PASS: TestRouteAuthorization (0.48s)
   --- PASS: TestAdminRoutesRequireASession (0.46s)

$ go build ./... && go vet ./... && gofmt -l . && go test ./...
   build OK, vet OK, gofmt printed nothing, no FAIL line, exit 0
```

Every gate in the plan was run. Two diverged; both are below.

## Divergences

**1. `go test ./internal/admin/ -run Album` does not select
`TestReorderAtTheEndIsNotAnError`.**

- **Direction:** the gate under-selects — 7 of the 8 tests run, not 8.
- **Cause:** `-run` is a regex over the test name and
  `TestReorderAtTheEndIsNotAnError` contains no "Album". The plan names both the
  gate and that test, and the two do not meet.
- **What was done:** the test keeps the plan's exact name and the gate was run
  exactly as written (it passes — tests do run, so its `fails_when` never
  fires). A second run with `-run 'Album|Reorder'` covers the eighth, and
  `go test ./...` covers everything. **Nothing was adjusted to make the gate
  look tighter than it is.** A later plan that wants this gate to mean "all of
  them" should rename the test `TestAlbumReorderAtTheEndIsNotAnError`.

**2. The `routerDeps` local in `newRouter` was not added.**

- **Direction:** two of the plan's three router-dependency places are there
  (`main.go:407` the literal, `main.go:642` the struct field); the third
  (`main.go:694`, the local) is not.
- **Cause:** Go rejects an unused local variable, and `newRouter` has no reader
  for the album store — the nine routes reach it through `adminHandler`. The
  only way to write the line today is `_ = albumStore`, a discard that reads as
  an oversight to the next person and proves nothing. The struct field carries a
  five-line comment saying exactly this and naming plan 11-05's public expansion
  as what will read it.
- **Nothing in the plan's gates measures this**, so the divergence is reported
  rather than caught.

## What the plan and 11-02-SUMMARY got wrong about the tree

**1. `internal/web/render.go:46` is testable, and the plan says three times that
it is not.** The objective, `must_haves.truths`, `11-PATTERNS.md` §E.7 and the
`fails_when` of the `layoutPageNames` gate all state that a page missing from
that slice is invisible to `go test ./...` and that the browser pass is the only
instrument. That is false. `web.RenderAdmin` writes the whole document, and the
document either carries `<body hx-headers='{"X-CSRF-Token": …}'>` and the
navigation or it does not — a handler test that asserts on the response body
sees the difference immediately.

`TestAlbumScreensRenderInsideTheBaseLayout` does exactly that for both screens,
and it was **mutation-tested**: removing `"album_list", "album_edit"` from the
slice again makes it fail, restoring them makes it pass.

```
--- mutant: names removed from layoutPageNames ---
--- FAIL: TestAlbumScreensRenderInsideTheBaseLayout (0.15s)
--- restored ---
ok  github.com/holzcloud/holzcloud-cms/internal/admin  0.904s
```

This matters beyond bookkeeping. The plan's own threat register calls a missing
entry a **security** finding (T-11-13: no `hx-headers` means every htmx POST from
that screen fails CSRF), and a security control whose only instrument is a human
opening a browser is an uncontrolled one. Phase 8's closing gate was one such
omission; a test of this shape would have caught it. **Recommendation for a
later phase:** one table-driven test over `layoutPageNames` that renders every
layout page against a fixture and asserts `hx-headers` is present. Written down
here rather than done, because it is 48 pages' worth of fixtures and outside
this plan's `files_modified`.

**2. The plan's `read_first` for task 2 points at
`cmd/holzcloud/templates/admin/block_list.html:252-280` for the `block_bild`
partial.** It is at `:250-280` in this tree — off by two lines, same paragraph,
no consequence. The partial's field names (`{{.Prefix}}.medium`,
`{{.Prefix}}.alt`, `{{.Prefix}}.bildunterschrift`, `{{.ID}}-…` for the label
targets) are exactly as the plan describes.

**3. `11-02-SUMMARY.md` was right about everything this plan depended on**, and
the two things it added beyond its own plan were both needed here:
`Pictures()` backs the reorder and delete controls (a `block.Item` carries no
row id, so `Items()` alone could not have drawn a single button), and
`ErrForeignMedia` is one of the five branches of `albumSaid`. Without either,
this plan would have had to write a second query over `album_items` — the second
spelling GAL-07 forbids.

## Deviations from Plan

**1. [Rule 2 — missing critical functionality] `TestAlbumScreensRenderInsideTheBaseLayout`**

- **Found during:** Task 2, after writing the two templates.
- **Issue:** the plan accepts that a template which parses but does not
  *execute* — a misspelled field, a partial fed the wrong type — ships green and
  is first seen by whoever opens the admin. Both screens had no execution
  coverage at all, and the layout membership that T-11-13 calls a security
  control had none either.
- **Fix:** one table-driven test over both screens asserting 200, `hx-headers`,
  a `nav-item`, and the album's own name in the body. Mutation-proved above.
- **Files:** `internal/admin/album_test.go` (in the plan's `files_modified`,
  though task 2's own `<files>` list does not name it).
- **Commit:** `38a4baa`

**2. [Rule 2 — missing critical functionality] `TestAlbumScreensAre404WithoutAStore`**

- **Found during:** Task 1. "A nil `albumStore` answers 404 rather than
  panicking" is an acceptance criterion with no test named for it, and a
  criterion nothing measures is a wish.
- **Fix:** an eighth test, using the bare `newTestAdmin` handler.
- **Commit:** `ad99438` (test at `2aa521c`)

**3. [Rule 2 — missing critical functionality] `SetAlbumStore` in `testRouter`**

- **Found during:** Task 3. `cmd/holzcloud/main_test.go`'s router builds its own
  admin handler, so without this the album row in the editor-open list would
  pass on a 404 produced by the nil-store guard — a green assertion about
  nothing. The table's contract is "not 403", and a route that never reaches the
  handler satisfies it vacuously.
- **Fix:** `adminHandler.SetAlbumStore(album.NewStore(database))` in
  `testRouter`, with a comment saying why.
- **Commit:** `b5ce7bd`

**4. [documented choice] Every refusal takes the same htmx branch as every
success.**

`internal/admin/menu.go` answers a validation refusal with a bare
`http.Redirect` and only takes the `HX-Request` branch on the success path
(`:154-160` vs `:167-176`). Copied verbatim that would mean an htmx refusal
swaps the redirect's body into the page instead of navigating. Each POST handler
here therefore computes one redirect target, sets a flash on every path, and
falls through to a single `HX-Request` branch at the end — which is also what
makes the gate's `7 7` a real pair per handler rather than an accident.

**5. [documented choice] `HandleAlbumList` runs one `Pictures` query per album.**

The list shows how many pictures are in each album and the store has no count
method. The alternative was a second statement in `internal/album`, which is
outside this plan's `files_modified` and would be a second query over
`album_items` beside `Pictures`. The loop carries a comment naming itself as the
first place to look if a website ever has hundreds of albums.

## Known Stubs

None. No placeholder, no `TODO`, no `FIXME`, no `t.Skip`, no unrun `<verify>`.
Every gate in the plan was run and its literal output is above.

## Threat Flags

None beyond the register. The four boundaries it names are all closed:

- **T-11-11** (two ids, nine handlers): every handler passes `websiteID` into
  every store call — `albumFromPath` is the single place both ids meet, and the
  store's WHERE clause is the strap under it. `TestAlbumEditFromAnotherWebsiteIs404`
  asserts on the body as well as the status.
- **T-11-12** (the wrong authorization table): `adminOnly` still 19, editor-open
  now 6, both measured.
- **T-11-13** (CSRF through `hx-headers`): now covered by a test, not only by
  the count gate — see the finding above.
- **T-11-14** (`AddItem` with another website's media id): `requireOwnPicture`
  in the handler *and* `requireOwnMedia` in the store, with
  `TestAlbumItemCreateRefusesAnotherWebsitesMedia` over the pair. The handler's
  copy also catches a film chosen where a photo belongs, which the store
  correctly has no opinion about.
- **T-11-15** (a name or caption reaching the screen): nothing in either
  template casts to `template.HTML`; `html/template` escapes by context.
- **T-11-16** (double submit): every submit button in both templates carries
  `hx-disabled-elt="this"` — 2/2 and 6/6, gated.
- **T-11-SC:** `go.mod` untouched, no package-manager install, no new dependency.

## Self-Check: PASSED

```
FOUND: internal/admin/album.go
FOUND: internal/admin/album_test.go
FOUND: cmd/holzcloud/templates/admin/album_list.html
FOUND: cmd/holzcloud/templates/admin/album_edit.html
FOUND commit: 2aa521c  test(11-03): failing tests for the album admin handlers
FOUND commit: ad99438  feat(11-03): the nine album handlers …
FOUND commit: 38a4baa  feat(11-03): the two album screens …
FOUND commit: b5ce7bd  feat(11-03): nine album routes, authorised as content
```

## TDD Gate Compliance

Task 1 carried `tdd="true"` and the gate sequence is in the log in order:
`test(11-03)` at `2aa521c` (RED — the test binary failed to build against a
`Handler` with no `SetAlbumStore` and no `HandleAlbum…`, while `go build ./...`
stayed green so the parallel executor's tree was never broken), then
`feat(11-03)` at `ad99438` (GREEN — eight tests passing, all nine gates green on
the first run). No REFACTOR commit: nothing needed cleaning up.
