# Phase 11: Galerie — Plan Check

**Checked:** 2026-09-07, before execution
**Plans:** `11-01-PLAN.md` … `11-07-PLAN.md`
**Verdict:** **ISSUES FOUND** — 5 blockers, 6 warnings. The phase's hardest
criterion (GAL-03) is the best-gated thing in it. What is not gated is the
translation half of the lightbox: the correction made to `11-01` and `11-07`
landed in the `must_haves` of both and in the *action* of neither, and no gate,
test or browser step anywhere proves a control name is translated **at render
time** — only that it reached the catalogue.

**Verdict per plan**

| Plan | Wave | Verdict | Blockers | Warnings |
|---|---|---|---|---|
| 11-01 | 1 | **BLOCKED** — the corrected claim did not reach the action, and render-time translation has no gate | 2 | 1 |
| 11-02 | 1 | **PASS with a warning** — every baseline correct, the `BeginTx` increase gated behaviourally | 0 | 1 |
| 11-03 | 2 | **BLOCKED** — one counting gate cannot pass as written | 1 | 0 |
| 11-04 | 2 | **BLOCKED** — a gate asserts a number a same-wave plan produces | 1 | 1 |
| 11-05 | 3 | **PASS with warnings** — GAL-03 is proved as a property, the `Clean` trap is found and gated | 0 | 2 |
| 11-06 | 4 | **BLOCKED** — GAL-04's decisive assertion is hand-read, not machine-decided | 1 | 0 |
| 11-07 | 5 | **BLOCKED** — the render-time half of its own `must_have` has no owning step | 2 | 1 |

---

## 1. Baselines: every number the seven plans assert, re-measured

Measured against the working tree on 2026-09-07, before writing this file.
**Every one is correct.** No plan rests on a guessed number — the Phase 9
failure shape does not recur here.

| What | Plans say | Measured | |
|---|---|---|---|
| migrations, highest file | 49, `00049_csv_imports.sql` | 49, same file | ✓ |
| packages under `internal/` | 40 | 40 | ✓ |
| files using `BeginTx` (non-test) | 14 | 14 | ✓ |
| admin templates | 66 | 66 | ✓ |
| `layoutPageNames` entries | 48 | 48 (parsed) | ✓ |
| `adminProtectedMux.Handle` | 150 | 150 | ✓ |
| `adminOnly` rows | 19 | 19 (via the plan's own `sed`) | ✓ |
| editor-open rows | 5 | 5 (via the plan's own `sed`) | ✓ |
| `{{define "icon-` blocks | 26 | 26 | ✓ |
| `class="nav-item` anchors | 26 | 26 | ✓ |
| fields on `block.Block` | 14 | 14 (parsed) | ✓ |
| fields on `bundle.Block` | 14 | 14 (parsed) | ✓ |
| fields on `bundle.Manifest` | 12 | 12 (parsed) | ✓ |
| fields on `bundle.Stores` | 10 | 10 (parsed) | ✓ |
| `bundle.Stores{` sites | 1 | 1 (`internal/admin/bundle.go:25`) | ✓ |
| `page.Slugify(` in `blocks.go` | 0 | 0 | ✓ |
| `bausteine.css`: fixed/absolute, visibility, `:target`, `display:none` | 0 0 0 0 | 0 0 0 0 | ✓ |
| `srcset` in `block/render.go` | 2 (comments only) | 2, both inside `:41-46` | ✓ |
| `/assets/bausteine.css` in TEMPLATE-SPEC | 0 | 0 | ✓ |
| themes linking it | 8 | 8 | ✓ |
| strings in source | 1277 | 1277, `0 offen, 0 verwaist` ×4 | ✓ |
| `Weiter` / `Zurück` in `en.json` | `Continue` / `Back` | exactly that | ✓ |
| `de4a1ce` / `5e453a9` | the menu cross-website write and its fix | both commits exist, both say so | ✓ |
| `internal/menu/store.go:14-33` | records the defect | it does, in those words | ✓ |
| `STATE.md` weide finding | positioned element counts toward scroll width under `visibility: hidden` | recorded, measured at 1280/1100/1024 | ✓ |

Two numbers **no plan measures**, measured here for the first time — see B1 and W3:

| What | Measured now |
|---|---|
| distinct `/admin/websites/{id}/menus…` paths for its nine routes | **8** |
| `en.json` keys whose value equals the German source (len > 3) | **27** |

---

## 2. Coverage

### GAL-01 … GAL-07 — all seven owned, none by prose

| Req | Owning plan / task |
|---|---|
| GAL-01 | 11-01 T1 (the `:target` large view, ids from block position + item index), 11-01 T2 (the reveal), 11-07 step 4 |
| GAL-02 | 11-01 T1 (`<a href="#…">` siblings, absent at both ends — `TestGalleryFirstHasNoPreviousAndLastHasNoNext`) |
| GAL-03 | 11-05 T2 `TestChangingAnAlbumChangesEveryPageThatCarriesIt`, 11-05 T1 (the marker), 11-07 step 6 |
| GAL-04 | 11-06 T2 `TestAlbumRoundTripAfterRename` (see B5) |
| GAL-05 | 11-02 T2 (the SQL statement scan), 11-03 T3 (both authorisation lists), 11-07 step 10 |
| GAL-06 | 11-04 T1 (`Display`, six landing places), 11-04 T2 (`scroll-snap`), 11-07 step 7 |
| GAL-07 | 11-01 T1 (`GalleryItems`, exported, one renderer), 11-02 T2 (`[]block.Item` shared), 11-05 T2 (the expansion calls it) |
| QUAL-01/02 | 11-07 T1 and T2 |

The union of the seven `requirements:` fields covers GAL-01 … GAL-07 with no
gap, and each has a task that delivers it rather than mentions it.

### The six ROADMAP success criteria

Criteria 1, 2, 3, 4 and 5 each have an owning task with a runnable gate.
Criterion 6 is 11-07 and is where two of the five blockers sit.

### D-01 … D-11 — all eleven implemented, none contradicted, nothing deferred crept in

Checked one by one against the plans' actions. The four Deferred Ideas — masonry
and justified layouts, an album as a field kind, zoom/pan, renaming `"galerie"` —
appear in **no** task. D-08's "the shop is not touched" is carried as a
*measurement* (`git status --porcelain internal/shop/` = 0) in 11-05 and 11-07,
which is the right shape for a negative decision.

---

## 3. The specific checks requested

### GAL-03, the criterion that can be false silently — **held, and held properly**

The finding is correct and the plans act on it. `internal/admin/page_blocks.go:74-81`
does freeze block HTML into `pages.content_html`; `internal/public/pagedata.go:32-40`
only rewrites it. 11-05 makes the album late-bound the way `snippet.Expand` is
(`internal/snippet/store.go:243-264`, whose early return and
"expands to nothing" rule are both copied), and places the call between
`snippet.Expand` and `filterByPlugins` with a gate that parses `pageContent` and
asserts the four call positions are in order.

And the proving test is the property, not the function:
`TestChangingAnAlbumChangesEveryPageThatCarriesIt` reads `content_html` **and**
`updated_at` straight out of the database, changes the album, asserts **both are
byte-for-byte identical**, and asserts the served body changed. Step 5 of its own
six steps is called out in the plan as the reason the test is worth writing. The
unit tests in `internal/album/expand_test.go` are *in addition to* it, not
instead of it. **No blocker. This is the strongest part of the phase.**

### The save that deletes the feature — **found, fixed and gated**

`block.Block.Empty()`'s gallery arm (`block.go:~248-255`) returns true when no
item carries a media id, a title or markdown, and `Set.Clean` drops every block
`Empty()` reports before encoding. A gallery naming an album carries no items.
11-05 names this as a defect **in the planning documents themselves**
(`<flagged_assumptions>`: "§A.3 gets as far as … and stops one function short"),
fixes `Empty()` in task 1, and gates it with
`TestAlbumBlockSurvivesClean` counted by `--- PASS`. The other half —
`TestGalleryWithNeitherItemsNorAlbumIsStillDropped` — stops the fix from turning
`Empty()` into a no-op. **No blocker.**

### The counting gates — one is unreachable, one races its own wave

Every baseline is right (§1). Two gates still fail, for two different reasons —
B1 and B2 below. Every comment-stripping gate was checked for the "matches the
prose that explains the prohibition" trap and **all of them strip first**:
`parent_id|location_key` in the migration, `isValidLocationKey` in the handler,
`srcset` in `render.go` (which would read 2 on the unchanged tree without the
strip — the plan says so explicitly), `UNIQUE constraint` in `album.go`. The
`page.Slugify(` gates all carry the parenthesis, as `11-CONTEXT.md` demands.

### `BeginTx` 14 → 15 — **the real gate is behavioural**

11-02 T2 requires `TestSwapDoesNotHoldTheWriteConnection`: a sequence of swaps
against the single write connection under a context deadline **below** the
5000 ms `busy_timeout`, failing on the deadline. It is run as its own gate with
`-timeout 30s`. The count gate is present in five plans and is explicitly
described in 11-02 T3 as existing "so that a *second* transaction appearing
anywhere in this phase is visible" — a change detector, not the proof. That is
exactly the correction Phase 9 earned. **No blocker.**

### `layoutPageNames` — **the arithmetic is right**

The phase adds two templates and only two: `album_list.html` and
`album_edit.html` (11-03 T2). The lightbox and the slideshow add **no** template
— the first is emitted by `internal/block/render.go`, the second is CSS on a
modifier class. 48 + 2 = 50, and the gate **parses the slice** rather than
grepping (the two names also appear in `main.go` and in the templates). The
independent admin-template count agrees: 66 + 2 = 68. 11-07 re-asserts `50 2`
against the finished tree and steps 1 and 2 of the browser pass confirm both
screens by eye, which is the only instrument that exists. **No defect here.**

### The authorisation table and the store's scoping — **both correct**

11-03 T3 files the album in the **editor-open** list at `main_test.go:186-190`
and gates **both directions**: `adminOnly` still 19, editor-open now 6. Either
mistake is invisible until somebody is locked out or let in, and both numbers are
measured.

The store follows `internal/term/store.go:284-311` and not `internal/menu/store.go`:
`websiteID` first after the context on every method, `website_id` in every WHERE,
`RowsAffected() == 0` as a named error, item statements scoped through a subquery
on `albums` because `album_items` has no column of its own. The gate is a
**statement scan** — it extracts every backticked SQL literal from the file and
prints the ones without `website_id` — which is stronger than a grep and names
the offender. 11-05 extends the same scan to the loading query. The 2026-09-06
history is cited accurately: `de4a1ce` proves it, `5e453a9` fixes it,
`menu/store.go:14-33` records it. **No blocker.**

### The unstyled case — **correctly constrained**

11-01 T1 requires a `<figure class="hc-galerie__gross">` sibling **inside** the
gallery `<div>`, after all tiles, and forbids a positioned wrapper, a backdrop or
`aria-modal`. T2 hides it with `display: none` and gates
`position: fixed|absolute` = 0 **and** `visibility:` = 0 with CSS comments
stripped — both baselines measured at 0. 11-04 re-asserts both for the scroll
container, which is where the finding reproduces most easily, and 11-07 asserts
all four numbers a last time. Browser step 8 loads a gallery page with
`/assets/bausteine.css` blocked. **No blocker.**

### A new `Block` field in both halves of `bundle/blocks.go` — **both gated, twice**

11-04: `Display:` comment-stripped count = 2. 11-05: `Album:` **and** `Display:`
both = 2 in one gate, so the second field cannot be added while the first
regresses. 11-07 closes with the three struct-field counts `16 16 13` in one
command, which is §A.4's rule made mechanical. **No blocker.**

### Wave ordering — no file is written twice in one wave

| Wave | Plans | Overlap |
|---|---|---|
| 1 | 11-01, 11-02 | none — `internal/block` + spec vs. `internal/album` + migrations |
| 2 | 11-03, 11-04 | none — admin/router/templates vs. `internal/block` + `internal/bundle` |
| 3, 4, 5 | 11-05, 11-06, 11-07 | alone in their waves |

Dependencies are acyclic with no forward references, and each plan's `wave` equals
`max(deps)+1`. `11-07` carries the review-fix halt as a real `<precondition>`
element on task 2 — not as prose — naming Phases 7, 8 **and** 9. **The Phase 7/8/9
failure is closed.** The one wave-2 defect is not a file overlap but a *count*
dependency: B2.

---

## 4. BLOCKERS (must fix before execution)

### B1 — `11-03`, Task 3 verify: the distinct-route-path gate expects 6 and will measure 8

```
grep -oE '/admin/websites/\{id\}/albums[^"]*' cmd/holzcloud/main.go | sort -u | wc -l
fails_when: the printed count is not 6
```

The `fails_when` prose enumerates "the collection, the album, its update, its
delete, its pictures, and the two-and-a-half under a picture" — "two-and-a-half"
is the arithmetic giving up. The nine routes the same task registers use **eight**
distinct paths:

```
/admin/websites/{id}/albums                                   (GET + POST — one path, two routes)
/admin/websites/{id}/albums/{albumID}
/admin/websites/{id}/albums/{albumID}/update
/admin/websites/{id}/albums/{albumID}/delete
/admin/websites/{id}/albums/{albumID}/pictures
/admin/websites/{id}/albums/{albumID}/pictures/{itemID}/update
/admin/websites/{id}/albums/{albumID}/pictures/{itemID}/delete
/admin/websites/{id}/albums/{albumID}/pictures/{itemID}/reorder
```

Measured confirmation against the analog the plan copies: the same nine menu
routes produce **8** distinct paths under the identical command.

**Fix:** expect **8**, and replace the enumeration in `fails_when` with the eight
paths listed. The gate's stated purpose — "the paths were enumerated rather than
assumed" — is exactly what it currently fails to do.

### B2 — `11-04`, Task 3: a gate asserts a number that a same-wave plan produces

```
ls internal/db/migrations/*.sql | wc -l && ls cmd/holzcloud/templates/admin/*.html | wc -l
fails_when: the two printed counts are not 50 and 68
```

and the table row `admin templates | 68 after 11-03 | 0 | 68`.

11-04 is wave 2 and declares `depends_on: ["11-01"]`. The 68 comes from
`album_list.html` and `album_edit.html`, which **11-03 creates — the other wave-2
plan**. The two run in parallel by design; there is no declared edge and no file
overlap for the wave guard to catch. Whichever ordering the executor picks, 11-04
can reach its own gate while the count is still 66 and fail deterministically for
a reason that has nothing to do with 11-04.

The migration row (50, from wave-1 11-02) is fine. The `BeginTx` = 15 row is fine
for the same reason.

**Fix:** either drop the admin-template assertion from 11-04 (it is 11-03's number
and 11-07 re-asserts it against the finished tree anyway), or state the count as a
range, or add `"11-03"` to `depends_on` and move 11-04 to wave 3 — the cheapest is
the first, because the number is already gated twice elsewhere.

### B3 — `11-01` Task 1's action still carries the claim that was corrected away

`11-01`'s `must_haves` truth is correct: `internal/block` calls `i18n.N` nine
times and `N` **is** one of the collector's eight functions
(`tools/i18n/main.go:53-65`, verified: `SetFlashError`, `SetFlashSuccess`,
`SetFlashWarning`, `Add`, `NewLayoutData`, `Titlef`, `T`, `N`; and
`internal/block/block.go:74-82` calls `N` nine times). Measured, both halves
confirmed.

But `11-01`'s **action** still reads:

> `tools/i18n/main.go:53-65` reads the first string argument of exactly eight
> named Go functions; **`internal/block` calls none of them**, so a literal
> written here is invisible to `go run ./tools/i18n` and stays German for ever.

That is false, and it is the sentence the executor reads while writing the code.
`11-07` Task 1 carries the same stale claim inside a `fails_when`:

> `internal/block` calls none of the collector's eight functions, so their
> presence here is the only proof the marking worked.

**Fix:** in `11-01`'s action, replace the premise with the true one — the package
is already collectable, so `i18n.N` is the right marker for *collection*, and the
real gap is that `render.go` carries **no locale**, which is why `Set.T` is
injected. In `11-07`, rewrite the `fails_when` rationale to match its own
`must_haves` truth. Both documents currently disagree with themselves.

### B4 — nothing anywhere proves the three control strings are **translated at render time**

This is the direct consequence of B3 and it is the finding that will read green.

`11-07`'s own `must_haves` states the requirement exactly:

> Their presence in the catalogue proves only half of it … The half that matters
> is that they are *translated when rendered*: `render.go` carries no locale of
> its own, so the browser step must read a gallery on a website whose language is
> not German and see the control in that language.

There is no such browser step. The eleven numbered steps of `11-07` Task 2 are:
album screens, edit-screen layout, duplicate name, lightbox, two galleries, album
on two pages, slideshow, stylesheet blocked, scripting off, second website, round
trip. **None of them names a language, a locale or a non-German website.**

And no automated gate closes it either:

- `11-01` gates `i18n.N(` = 3 (marking) and `set.T = ` = 1 per file (injection).
  Neither proves the gallery arm *applies* `T` to those three literals.
- `11-01`'s only translator test is `TestSetWithoutTranslatorKeepsTheGermanSource`
  — the **nil** case.
- `11-04` has `TestSlideshowNameGoesThroughTheTranslator` (a `Set` whose `T`
  uppercases) — for the slideshow's accessible name, not for the three controls.
- `11-07` asserts the three literals exist as keys in `en.json`. A string that is
  marked, collected, translated into four languages and then printed in German
  satisfies every one of these.

That is precisely the failure `11-01` describes one paragraph above the gate that
does not catch it: "the string is collected, translated in four catalogues, and
still printed in German — which is worse … because the gate would read green."

**Fix, both halves:**
1. `11-01` Task 1: add `TestGalleryControlsGoThroughTheTranslator`, in the shape
   `11-04` already uses — a `Set` whose `T` uppercases (or prefixes), asserting
   all three rendered control names came back transformed. Add it to the named
   test list and to the acceptance criteria.
2. `11-07` Task 2: add a numbered browser step that sets the website's language to
   one of `en`, `es`, `fr` or `it`, saves a page with a gallery, opens the public
   page and reads the three control names in that language — with its own
   screenshot and its own acceptance-criteria line, like steps 4 and 8.

### B5 — `11-06`, Task 2: GAL-04's decisive assertion is read by hand, not by the gate

```
print('Rename' in f, f.count('Contains'), f.count('!strings.Contains') + f.count('strings.Contains'))
fails_when: the first printed value is not True — … Read the other two numbers
            by hand and confirm both directions are asserted …
```

The plan is right that only the **negative** assertion — the manifest does *not*
contain the old slug — fails when the translation is missing;
`bundle_test.go:1124-1137` spends thirteen lines saying so, and the plan quotes
them. The gate then leaves that assertion to a human reader, in a phase whose
`11-CONTEXT.md` opens by recording that the developer asked for the whole
milestone to be carried out **autonomously**. In an unattended run nobody reads it.

The consequence is exact: `TestAlbumRoundTripAfterRename` could be written with
the rename and with only the positive assertion. It would pass. The `--- PASS`
gate would read 1. GAL-04 would be declared and unproved — which is what happened
to Phase 7's Term field, as this plan's own objective says.

**Fix:** make the negative direction machine-decided. Extract the test body and
assert `f.count('!strings.Contains') >= 1` (or `assert.NotContains`, whichever the
file uses), with `fails_when: the printed count is 0 — the manifest-does-not-carry-
the-old-slug assertion is the only one that fails when the translation is missing`.
Keep `'Rename' in f` as the second half.

---

## 5. WARNINGS (worth fixing)

**W1 — `11-04`'s `<verification>` contradicts `11-04`'s own gate.** It reads
"`block.Block` has 15 fields and `bundle.Block` has **13**". The gate in Task 3
expects **15** for `bundle.Block`, `11-07`'s closing gate expects 16 after both
fields, and the measured baseline is **14**. 13 is wrong on every reading. Change
it to 15.

**W2 — `11-05` Task 1's marker-uniqueness gate cannot see the drift it names.**
It scans non-test `.go` files for the literal `[[album:` and expects exactly 1,
"because two files means two spellings of one string in two packages". But a Go
regexp source literal is written `\[\[album:` and does **not** match that pattern.
So a tree with the *writer* in `internal/block/render.go` and the *pattern* in
`internal/album/expand.go` — the exact split the gate exists to forbid — still
measures 1 and passes. A gate that proves a proxy. Add a second count over
`\\\[\\\[album:` (the escaped form), or assert both spellings live in the same
file by name.

**W3 — `11-07` Task 1's "German value left in the catalogue" gate has no
baseline.** Its `fails_when` is "the printed count rose from what plan 11-06's
summary recorded" — and `11-06`'s summary table has no such row, so the number
will not exist when the gate runs. This is Phase 9's second shape: a condition
needing two numbers from a command that prints one. **Measured now: 27** for
`en.json` (es 9, fr 15, it 14). Pin the expected value to 27, or add the row to
`11-06`'s table.

**W4 — four plans count `--- PASS` lines to prove a test exists.** `11-01` T3
(= 2), `11-02` T1 (= 1), `11-05` T1 and T2 (= 1), `11-06` T2 (= 1). A test that
uses `t.Run` prints one `--- PASS` per subtest and the gate over-counts —
and `11-01`'s `TestEveryShippedThemeLinksTheBlockStylesheet` is a natural
subtest-per-theme, while `11-02`'s `TestItemMethodsRefuseAnotherWebsitesAlbum` is
described in the plan as "a table over four methods". Use
`grep -c -- '--- PASS: TestName'` anchored on the function name, or `-run
'^TestName$'` with `grep -c '^--- PASS'`.

**W5 — `11-02` Task 1's rollback-order gate is brittle on a literal.** It does
`d.index('DROP TABLE IF EXISTS albums;')`, which raises `ValueError` if the
executor writes `DROP TABLE albums;`. The house style (`00049`'s tail) does use
`IF EXISTS`, so this will most likely pass — and the failure is loud rather than
silent — but the action text never says `IF EXISTS`. Say it, or match on
`DROP TABLE(?: IF EXISTS)? albums;`.

**W6 — `11-05` Task 3's `grep -c 'album' block_list.html` is a keyword gate.**
It is meant to prove the select's `name` attribute exists, but the same task adds
a `form-hint` sentence about albums, which satisfies it on its own. Match the
attribute: `grep -c 'name="[^"]*album'`.

---

## 6. Accepted, not findings

- **No `11-UI-SPEC.md`, which the roadmap's research flag calls "worth a
  UI-SPEC".** `11-07`'s `<flagged_assumptions>` states the substitution
  explicitly and lists what carries its content instead (the focusable large
  view, anchors rather than buttons, absent controls at the ends, the close
  target that matches nothing, plus browser step 4). Declared rather than
  skipped; that is the right handling.
- **`11-04` and `11-06` deliberately leave the i18n count unpredicted** and
  measure it instead. That is the correct answer to Phase 9's guessed number, not
  an omission.
- **`bundle_test.go` and `block_test.go` become mixed-language.** Every plan that
  touches them says so and asks for a comment saying so. Deliberate.

---

## The single most dangerous thing found

**B4** — nothing proves the lightbox's three control names are translated at
render time.

Not because it is the hardest to fix — it is two additions, one test and one
browser step — but because of how it fails. Every gate in the phase reads green:
the three literals are marked (`i18n.N(` = 3), they reach four catalogues,
`go run ./tools/i18n` reports `0 offen, 0 verwaist` four times, `Weiter` and
`Zurück` are provably not among them. `11-07` signs the standing gate off. And a
Spanish visitor clicking a picture on a Spanish website reads **Weiter**,
**Zurück** and **Schliessen** — for ever, on every gallery of every site the
product runs, because `render.go` has no locale and nothing checks that `Set.T`
was ever applied to those three strings.

The phase's own documents already know this. `11-01`'s `must_haves` calls it
"worse than the `internal/field` hole because the gate would read green", and
`11-07`'s `must_haves` names the browser step that would catch it. The correction
that produced both sentences reached the frontmatter of two plans and the
executable body of neither — which is the same shape as Phase 9's B1, where a
hand edit landed in prose and not in three gates. The fix is small; finding it
after the browser pass has been signed off is not.
