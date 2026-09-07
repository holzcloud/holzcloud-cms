---
phase: 11-galerie
plan: 06
subsystem: bundle format, export, import, block translation
tags: [album, bundle, manifest, round-trip, rename, slug-derivation, gal-04, gal-07]
status: complete

requires:
  - "11-02 — album.Store: Create with its single page.Slugify call, List, Items, Rename that moves the name and never the slug"
  - "11-05 — block.Block.AlbumSlug on the domain side and bundle.Block.Album on the manifest side, carried as a pass-through"
  - "11-03 — h.albumStore on the admin handler, which is what bundle.Stores gets filled from"
provides:
  - "bundle.Album and bundle.AlbumItem, and Manifest.Albums (json `albums`, omitempty)"
  - "bundle.Stores.Albums *album.Store — one field serving both halves"
  - "exportAlbums: the albums out, and the slug → name map the block translation needs"
  - "importAlbums: the albums in, created through album.Store.Create, after the media and before the pages"
  - "the block translation on both sides — the album reference leaves as a NAME and comes back through page.Slugify"
  - "missingAlbum beside missingMedia — a block naming an album the archive did not bring is reported by name"
  - "Report.Albums, counted as created and surfaced on the import report screen"
  - "TestAlbumRoundTripAfterRename and TestManifestCarriesNoAlbumSlug — GAL-04's proof, renamed before export, both directions asserted"
affects:
  - "11-07 — owns the one new German string this plan added (34 offen now, was 33) and the browser pass"

actuals:
  tokens: 12413
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "a reusable thing crosses the manifest by its NAME and the address is re-derived on import — Page.Terms' rule, now with an album in the term's place"
    - "one derivation of one key: the importer never writes its own INSERT, so album.Store.Create's single page.Slugify call is the only place an album's address is made"
    - "the proving test renames first — an un-renamed thing round-trips correctly even with the translation missing"

key-files:
  created: []
  modified:
    - internal/bundle/format.go
    - internal/bundle/export.go
    - internal/bundle/import.go
    - internal/bundle/blocks.go
    - internal/bundle/bundle_test.go
    - internal/admin/bundle.go
    - cmd/holzcloud/templates/admin/import_report.html

key-decisions:
  - "The manifest carries the album's name and no slug: carrying both would be two sources for one key, and the derived one would win anyway"
  - "importAlbums sits after the media and before the pages — the media because a picture is a file name, the pages because a block naming a missing album must be reportable; explicitly not last like the menu import, whose items point at pages by address"
  - "Manifest.Version is NOT bumped: `albums` is omitempty, so an export from a website without albums is byte-for-byte what it was and an import meeting no albums key behaves exactly as before"
  - "A block naming an album the archive did not declare has its reference DROPPED on both sides rather than passed through, and the drop is reported by name"
  - "The four new warnings in importAlbums/missingAlbum are built with fmt.Sprintf like their thirty siblings and are therefore invisible to tools/i18n — recorded in WINDOWS.md rather than half-fixed with the wrong locale"

patterns-established:
  - "the album marker's sibling rule: what is stored is an address, what travels is a name, and the two derivations are the same one call"
  - "mutation verification as evidence: each of the four pieces was broken, a named test was watched go red, and the failure text is in this summary"

requirements-completed: [GAL-04, GAL-07]

coverage:
  - id: D1
    description: "An album survives the bundle round trip INCLUDING after a rename — the manifest carries the new name and not the old address, and the block's reference resolves on a website that never saw the album"
    requirement: "GAL-04"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestAlbumRoundTripAfterRename"
        status: pass
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestManifestCarriesNoAlbumSlug"
        status: pass
    human_judgment: false
  - id: D2
    description: "The album travels on the manifest under its own name, with its pictures as file names and never as ids, and with no slug"
    requirement: "GAL-04"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestExportWritesAlbumsWithFileNames"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every album the manifest declares is created on import through album.Store.Create, its pictures resolve to this import's own media, and report.Albums counts what was created rather than what was claimed"
    requirement: "GAL-04"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestImportCreatesAlbumsBeforePages"
        status: pass
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestImportReportsAlbumsCreatedNotClaimed"
        status: pass
      - kind: other
        ref: "python3 comment-stripped count of 'INSERT INTO albums' in internal/bundle/import.go == 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "A block naming an album the archive did not bring is reported by name, the way missingMedia reports a file that did not arrive"
    requirement: "GAL-07"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestBlockNamingAnAbsentAlbumIsReported"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every archive already written still imports unchanged: albums is optional and the format version is untouched"
    requirement: "GAL-04"
    verification:
      - kind: integration
        ref: "internal/bundle/bundle_test.go#TestManifestWithoutAlbumsImportsAsBefore"
        status: pass
      - kind: integration
        ref: "tools/mkbundle/pack_test.go#TestTheExampleStillPacks"
        status: pass
      - kind: other
        ref: "git diff -U0 6e80f4e -- internal/bundle/format.go | grep -E 'const Version|Version +int' == 0"
        status: pass
    human_judgment: false
  - id: D6
    description: "The operator sees how many albums an import made, on the import report screen"
    verification: []
    human_judgment: true
    rationale: "The count is rendered by a template line that no test exercises. Whether the number reads sensibly beside the other four is a look at the screen, and 11-07 owns the browser pass."

duration: 21 min
completed: 2026-09-07
---

# Phase 11 Plan 06: The Album Crosses the Manifest Summary

**An album exported, renamed before it went, imported into a fresh website, and every page that carried it still carrying it — the reference travelling as a name and the address re-derived by the one `page.Slugify` call `album.Store.Create` already makes.**

## Performance

- **Duration:** 21 min (commit span `60806b6` 00:57:40 → `fb909bc` 01:05:30 local, plus the reading before it)
- **Started:** 2026-09-07T22:47Z (approximate — the plan's `record_start_time` was not stamped; the first commit is exact)
- **Completed:** 2026-09-07T23:08:06Z
- **Tasks:** 3 (task 3 is arithmetic and produced no code, by its own instruction)
- **Files modified:** 7

## Accomplishments

- **GAL-04 is true and proved.** `TestAlbumRoundTripAfterRename` creates `Werkstatt 2024`, asserts its address is what `page.Slugify` makes of that name, renames it to `Werkstatt 2025` (which slugifies **differently**), asserts the rename did **not** move the address, hangs a gallery block carrying the now-stale address on a page, exports, asserts the manifest contains the new name **and does not contain the old address**, then imports into a website that has never seen the album and asserts the block resolves to it — by address, by name, and by its picture belonging to the new website.
- **All four pieces landed and all four were mutation-verified.** The `Album` struct on the manifest, `exportAlbums`, `importAlbums`, and the block translation were each broken in turn, a named test was watched go red, and the failure text is reproduced below. A claim of coverage that was never seen red is not evidence, so it is here.
- **The negative assertion is machine-decided, not read by hand.** `TestManifestCarriesNoAlbumSlug` is the focused gate: renamed album, `strings.Contains(manifest, oldSlug)` must be false. This is the exact shape in which Phase 7's Term field shipped declared and unproved.
- **A missing album is reported by name.** `missingAlbum` beside `missingMedia`; the operator reads *"Seite „Arbeiten“: das Album „Aussenanlagen“ ist nicht im Archiv, die Galerie bleibt leer"* instead of finding a gallery that is simply not there.
- **Every archive already written still imports unchanged.** `albums` is `omitempty`, `Manifest.Version` is untouched, and the example bundle's pack test was **run** (not reasoned about) and passes.

## Task Commits

1. **Task 1 (RED): failing tests for the album on the manifest** — `60806b6` (test)
2. **Task 1 (GREEN): an album crosses the manifest under its name** — `5f7709e` (feat)
3. **Task 2 (RED): failing tests for the album reference that survives a rename** — `d4b579a` (test)
4. **Task 2 (GREEN): a gallery's album reference travels as a name** — `fb909bc` (feat)

Task 3 carries no commit: its instruction is "No new code. The arithmetic, and one thing to check that is not a count." Its output is the table below.

Every commit was staged file by file. `git show --stat` on all four shows only this plan's files; another agent committed to the same working tree between mine (`52c0937`, `031b888`, `cb7cdc5`, `7e0c834`, `50c2030`, `42d651e`) and none of their files appear in any of my commits.

## Files Created/Modified

- `internal/bundle/format.go` — `Album`, `AlbumItem`, `Manifest.Albums`; and `Block.Album`'s comment rewritten, because it is no longer the pass-through 11-05 documented
- `internal/bundle/export.go` — `Stores.Albums`, `exportAlbums`, the call placed between the media and the pages, the slug → name map threaded into `exportPages`
- `internal/bundle/import.go` — `Report.Albums`, `importAlbums`, its placement and the reason, `declaredAlbums` wired into `importPages`, the `missingAlbum` warning loop
- `internal/bundle/blocks.go` — the translation on both sides, `importAlbumSlug`, `declaredAlbums`, `missingAlbum`
- `internal/bundle/bundle_test.go` — seven new tests and one helper, English in a German file, with the reason written above them
- `internal/admin/bundle.go` — `Albums: h.albumStore`, the one construction site
- `cmd/holzcloud/templates/admin/import_report.html` — the album count reaches a person

## The counting gates of this wave

Baselines measured against the pre-change tree at `6e80f4e` on 2026-09-07, immediately before the first commit. Every row was measured again against the finished tree.

| What | Command | Baseline | Predicted add | Predicted after | **Measured after** | Verdict |
|---|---|---|---|---|---|---|
| fields on `bundle.Manifest` | `python3` over the struct | 12 | 1 (`Albums`) | 13 | **13** | matches |
| top-level structs in `format.go` | `grep -c '^type '` | **14** (measured) | 2 (`Album`, `AlbumItem`) | 16 | **16** | matches |
| `bundle.Stores` fields | `python3` over the struct | 10 | 1 | 11 | **11** | matches |
| `bundle.Stores{` construction sites | `grep -rn … \| wc -l` | 1 | 0 | 1 | **1** | matches |
| `page.Slugify(` in `blocks.go` | comment-stripped count | 0 | 1 | 1 | **1** | matches |
| `INSERT INTO albums` in `import.go` | comment-stripped count | 0 | 0 | 0 | **0** | matches |
| `func missingAlbum` in `blocks.go` | comment-stripped count | 0 | 1 | 1 | **1** | matches |
| `Albums:` in `internal/admin/bundle.go` | `grep -c` | 0 | 1 | 1 | **1** | matches |
| `Manifest.Version` touched | `git diff -U0 6e80f4e … \| wc -l` | 0 | 0 | 0 | **0** | matches |
| files using `BeginTx` | `grep -rln … \| grep -v _test \| wc -l` | 15 after 11-02 | 0 | 15 | **15** | matches |
| migrations | `ls …/*.sql \| wc -l` | 50 after 11-02 | 0 | 50 | **50** | matches |
| admin templates | `ls …/admin/*.html \| wc -l` | 68 after 11-03 | 0 | 68 | **68** | matches |
| strings in source | `go run ./tools/i18n \| head -1` | **1310** (measured after 11-05) | not predicted | — | **1311** | +1, isolated below |
| `offen` in `en.json` | `go run ./tools/i18n` | **33** (measured after 11-05) | not predicted | — | **34** | +1, isolated below |

**No divergence anywhere.** Both rows the plan marked "measured" were measured twice, before and after, and both numbers are above.

**The i18n row as an isolation measurement rather than a subtraction.** Another agent changed `tools/i18n/main.go` while this plan ran (`+71` lines), so a before/after difference across the whole tree would have measured two things at once. Measured on the finished tree instead, with only my one line removed and restored:

| Tree | Strings | `offen` |
|---|---|---|
| finished tree | 1311 | 34 |
| finished tree, minus the `{{tf "%d Alben"}}` line only | 1310 | 33 |
| finished tree, line restored | 1311 | 34 |

**This plan adds exactly one**, and it is routed through the catalogue with `tf`, so plan 11-07 finds it. See the deviation below for the four operator-facing strings this plan added that `tools/i18n` will *not* find, and why they were left in that form.

## The example bundle, checked rather than reasoned about

```
$ go test ./tools/mkbundle/ -v
--- PASS: TestTheExampleStillPacks
--- PASS: TestEveryPictureIsTheTypeItClaims
--- PASS: TestAPageMayCarryALabelSpelledAsItsName
--- PASS: TestALabelSpelledAsASlugSaysWhatToWriteInstead
--- PASS: TestALabelThatIsNeitherNameNorSlugIsStillRefused
--- PASS: TestTheExampleStillCarriesALabelWorthShowing
ok  	github.com/holzcloud/holzcloud-cms/tools/mkbundle	0.287s
```

`sites/beispiel/holzcloud.json` carries no `album` or `albums` key and no gallery block, so nothing in it changed meaning. `readManifest` there refuses a field `internal/bundle` does not know, which is why this test is the one that would have caught a rename or a drop — it did not fire, because the key is new and optional.

**`Manifest.Version` is not bumped, and that is a decision rather than an omission.** `albums` is `omitempty`: an export from a website with no albums is byte-for-byte what it was, and an import that meets no `albums` key behaves exactly as it did. Bumping the version would make every archive already handed to somebody look wrong to the importer, for no gain — `readManifest` refuses a *newer* major version, so the bump would break the old archives rather than protect them.

## Mutation verification

Each of the four pieces was broken, the named test was watched go red, and the source was restored with `git checkout`. The failure output is the evidence.

**1. The `Album` struct on the manifest** — `json:"albums,omitempty"` changed to `json:"-"`:

```
--- FAIL: TestExportWritesAlbumsWithFileNames
    bundle_test.go:877: the manifest does not carry "albums":
    bundle_test.go:877: the manifest does not carry "name": "Referenzen":
--- FAIL: TestImportCreatesAlbumsBeforePages
    bundle_test.go:930: report.Albums = 0, want 2 (warnings: [])
    bundle_test.go:939: album "referenzen" is not on the imported website: <nil>
--- FAIL: TestImportReportsAlbumsCreatedNotClaimed
    bundle_test.go:988: report.Albums = 0, want 1 — three were claimed, one could be made
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1108: a clean round trip warned: [Seite "Arbeiten": das Album "Werkstatt 2025" ist nicht im Archiv, die Galerie bleibt leer]
--- FAIL: TestBlockNamingAnAbsentAlbumIsReported
    bundle_test.go:1219: the album that did arrive is reported as missing: [...]
```

**2. `exportAlbums`** — `m.Albums = append(...)` replaced with a discard:

```
--- FAIL: TestExportWritesAlbumsWithFileNames
    bundle_test.go:877: the manifest does not carry "albums":
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1108: a clean round trip warned: [Seite "Arbeiten": das Album "Werkstatt 2025" ist nicht im Archiv, die Galerie bleibt leer]
    bundle_test.go:1119: 1 blocks instead of 2: [{Type:text ... AlbumSlug: ...}]
```

**3. `importAlbums`** — the creation loop short-circuited:

```
--- FAIL: TestImportCreatesAlbumsBeforePages
    bundle_test.go:930: report.Albums = 0, want 2 (warnings: [])
    bundle_test.go:939: album "referenzen" is not on the imported website: <nil>
--- FAIL: TestImportReportsAlbumsCreatedNotClaimed
    bundle_test.go:988: report.Albums = 0, want 1 — three were claimed, one could be made
    bundle_test.go:1003: 0 albums on the new website, want 1: []
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1127: the gallery points at no album of the imported website: <nil>
```

**4a. The block translation, export half** — restored to 11-05's pass-through, `Album: b.AlbumSlug`:

```
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1097: the manifest still carries the old slug "werkstatt-2024"; the reference
        travels as a slug and will point at nothing on the other machine:
    bundle_test.go:1108: a clean round trip warned: [Seite "Arbeiten": das Album "werkstatt-2024" ist nicht im Archiv, die Galerie bleibt leer]
--- FAIL: TestManifestCarriesNoAlbumSlug
    bundle_test.go:1183: the manifest carries the album's old address "referenzen-aussen"; it must
        carry the name and let the other machine derive the address:
```

**4b. The block translation, import half** — `page.Slugify(name)` replaced with `name`:

```
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1122: the gallery points at "Werkstatt 2025"; the album on this website is "werkstatt-2025"
--- FAIL: TestBlockNamingAnAbsentAlbumIsReported
    bundle_test.go:1241: the block naming an album that arrived points at "Referenzen"
```

**4c. `missingAlbum`** — made to report nothing:

```
--- FAIL: TestBlockNamingAnAbsentAlbumIsReported
    bundle_test.go:1216: the report does not name the album that did not arrive: []
```

**The most valuable thing this exercise produced is not in the list above.** When the Task 2 tests were first run against the Task 1 tree — albums on the manifest, no block translation — `TestAlbumRoundTripAfterRename` failed like this:

```
--- FAIL: TestAlbumRoundTripAfterRename
    bundle_test.go:1087: the manifest still carries the old slug "werkstatt-2024"; ...
    bundle_test.go:1112: the gallery points at "werkstatt-2024"; the album on this website is "werkstatt-2025"
```

The **positive** assertion — the manifest contains `Werkstatt 2025` — passed, because the albums list already carried the name. Only the **negative** one failed. That is the plan's claim ("only the negative assertion fails when the translation is absent") observed live rather than taken on trust, and it is why the test asserts both directions and why `TestManifestCarriesNoAlbumSlug` exists as a gate of its own.

## Decisions Made

**No slug on the manifest's album.** §D.2 left it open. The name alone travels; the importing machine derives the address with `page.Slugify`, the one call `album.Store.Create` makes. Carrying both would be two sources for one key — the shape `internal/term/store.go:318-328` warns about — and the derived one would win anyway, so the carried one would be documentation that could silently disagree.

**`importAlbums` between the media and the pages.** After the media because an album's pictures are file names and `mediaByName` is where those become ids. Before the pages because a gallery block names an album and the report must be able to say *which* album a block did not find. Explicitly **not** last like the menu import, whose items point at pages by address and therefore need the pages first.

**The importer writes no INSERT.** Every album is made through `album.Store.Create`, gated by a comment-stripped count of zero. The point is not tidiness: `Create` holds the single `page.Slugify` call that derives an album's address, and `importBlocks` derives the same address from the same name. A second INSERT would be a second derivation of one key, and the two would agree right up until the day one of them changed.

**A block whose album did not arrive loses its reference and its block.** The reference is dropped rather than passed through, for the reason `export.go:524-531` gives for a term. `set.Clean` then removes the gallery, because a gallery with neither an album nor a list of its own is empty — 11-05's own rule. That is the right outcome: what stays on the page is what can be drawn, and `missingAlbum` is where the operator reads what went missing and why. `TestBlockNamingAnAbsentAlbumIsReported` asserts both halves.

**The new tests are English inside a German file, on purpose, and a comment says so.** Eleven lines above `seedAlbumSite` state the rule (`.planning/GLOSSARY.md`: what is written now is written in English), state that the German around it predates the rule and is not being rewritten by this plan, and ask the next reader not to translate one half to match the other. Without that line the file looks like a mistake in one direction or the other.

## Deviations from Plan

### 1. [plan defect — two gates that cannot be satisfied alongside the comments the same plan asks for]

- **Found during:** Task 1, writing the two placement comments.
- **Issue:** The plan's `<action>` for Task 1 gives the `importAlbums` call-site comment nearly verbatim, and it contains the sentence *"Deliberately **not** last like `importMenus`, whose items point at pages by slug"*. The gate for the same task is:

  ```python
  f = re.search(r'func Import\(.*?\n\treturn report, nil\n\}', s, re.S).group(0)
  print(f.find('importMedia'), f.find('importAlbums'), f.find('importPages'), f.find('importMenus'))
  ```

  with `<fails_when>` "the four numbers are not in increasing order". The comment sits **above** the `importAlbums` call, so writing the literal `importMenus` in it makes `find('importMenus')` return a position **before** `find('importPages')`. **No implementation carrying the comment the plan dictates can make this gate pass.** The identical trap exists on the export side: the plan asks for a comment "in the register of the one at `:143-150`", and that one names `exportPages` literally — writing that identifier above the `exportAlbums` call puts `find('exportPages')` before `find('exportAlbums')`.
- **Evidence:** both gates print increasing numbers only because the identifiers were kept out of the two call-site comments. Measured on the finished tree — export `874 1464 1945`, import `1072 2909 2973 3106`.
- **Fix:** the substance was written and split by audience. The **call-site** comments state the placement and the reason in prose (*"Ausdrücklich nicht zuletzt wie die Menüs, deren Einträge über Adressen auf Seiten zeigen"*, with the file-and-line `import.go:1087` for the contrast); the **doc comments on `exportAlbums` and `importAlbums`** carry the full argument and are free to name identifiers, because neither gate reads outside `func buildManifest` / `func Import`. This is arguably the better placement anyway — the reason belongs on the function, the ordering constraint belongs at the call.
- **Files:** `internal/bundle/export.go`, `internal/bundle/import.go`
- **Commit:** `5f7709e`

### 2. [Rule 3 — blocking] `TestRoundTripKeepsBlocks` asserted the pass-through this plan removes

- **Found during:** Task 2, first green run.
- **Issue:** 11-05 left `TestRoundTripKeepsBlocks` asserting `angekommen[3].AlbumSlug == "moebel"` after a real export/import, with **no album of that address anywhere in the test's website** — correct for a pass-through, and exactly what the translation is required to drop. Failure text:

  ```
  --- FAIL: TestRoundTripKeepsBlocks
      bundle_test.go:753: the gallery's album did not survive the archive: ""
  ```
- **Fix:** the test now creates the album it names — `s.Albums.Create(ctx, ws, "Möbel")`, whose derived address is `moebel` — so the assertion holds again and now proves the round trip *through* the translation rather than around it. The comment that described the value as a pass-through was rewritten to say what it now proves and to point at `TestAlbumRoundTripAfterRename` for the case where the two derivations must not be allowed to agree by accident.
- **Files:** `internal/bundle/bundle_test.go`
- **Commit:** `fb909bc`

### 3. [Rule 2 — missing critical] `TestBlockNamingAnAbsentAlbumIsReported` expected a block that `Clean` correctly deletes

- **Found during:** Task 2, first green run. This is a defect in the test I wrote in the RED step, not in the plan.
- **Issue:** the test expected two blocks back, one with an empty reference. `set.Clean` removes a gallery that has neither an album nor a list of its own — 11-05's rule, and right.
- **Fix:** the test asserts one block, and carries a comment saying why the second is gone and that the report is where the loss is visible. The report assertion (`warned(report, "Aussenanlagen")`) is unchanged and is the part that matters.
- **Files:** `internal/bundle/bundle_test.go`
- **Commit:** `fb909bc`

### 4. [Rule 2 — missing critical] One file outside the plan's `files_modified`

- **Found during:** Task 1.
- **Issue:** the plan's `<action>` says *"surface it wherever the import report is rendered, so the number reaches a person"*, and its acceptance criteria require `report.Albums` to be believable — but `files_modified` lists six files and the import report screen is a seventh, `cmd/holzcloud/templates/admin/import_report.html`. Without the line the count exists and nobody ever reads it.
- **Fix:** one line, `<li>{{tf "%d Alben" .Report.Albums}}</li>`, beside the other four. Routed through `tf` so the catalogue sees it — measured as exactly +1 above, which is what plan 11-07 will find.
- **Files:** `cmd/holzcloud/templates/admin/import_report.html`
- **Commit:** `5f7709e`

### 5. [recorded, not fixed] Four new operator-facing sentences that `tools/i18n` cannot see

- **Found during:** Task 2, after `7e0c834` landed on main mid-flight.
- **Issue:** `CLAUDE.md` now states, in a section added while this plan ran, that *"a sentence built with `fmt.Sprintf` is invisible to \[the collector], wherever it stands. Assemble the sentence in the catalogue, not in Go."* `bundle.Report.Warnings` is built entirely that way — roughly thirty such sentences predate this plan in `import.go` — and this plan added four more (`importAlbums`' three, `missingAlbum`'s one), in the same form as their siblings.
- **Why not fixed here:** the honest fix is the shape `.planning/GLOSSARY.md` already prescribes for `csvimport` — a **code plus arguments**, never a finished sentence (D-32) — which means changing the `Report` type and every producer and its renderer. The cheap fix is worse than nothing: `internal/bundle` already imports `i18n`, but the only locale it holds is `manifest.Site.Locale`, the language of the **imported website**, not of the operator reading the report. Routing the warnings through that would translate the operator's report into the wrong language while making the gate report `0 offen`. Adding the operator's locale to `bundle.Import` is a signature change through the whole import path — Rule 4 territory and not this plan's scope.
- **Fix:** recorded rather than half-done. Entered in `.planning/WINDOWS.md` as entry 6 (`kind: deviation`, `internal/bundle/import.go`), which is the same class as the standing entry 3 for `internal/field/field.go`.
- **Files:** `.planning/WINDOWS.md`
- **Commit:** with this summary

---

**Total deviations:** 5 — 2 plan defects recorded with evidence and worked around rather than argued with, 2 auto-fixes (Rule 3 blocking, Rule 2 missing critical), 1 recorded and deliberately not fixed.

**Impact on plan:** no scope creep. Deviation 1 is the same class 11-05 recorded twice — a gate whose `<fails_when>` contradicts the comment the same plan dictates — and the substance of both comments was written, only split between the call and the function. Deviation 2 is the price of turning a pass-through into a translation and is the correct behaviour showing up in an old test. Deviation 5 widens a pre-existing surface by four sentences and is on the ledger.

## Issues Encountered

**Another agent was committing to the same working tree.** Six of their commits are interleaved with my four in `git log`. Every commit here was staged file by file and `git show --stat` on all four shows only this plan's files; no `git add -A` or `git add .` was used, and no `git clean`, `git stash` or blanket reset was run at any point.

**One consequence of that had to be measured around.** `tools/i18n/main.go` changed by +71 lines mid-flight, so the string count's before/after would have measured two changes at once. The isolation measurement in the table above replaces the subtraction.

## Known Stubs

None. Every file this plan touched was scanned for hardcoded empty values flowing to a render path, placeholder text, `TODO`/`FIXME`, and skipped tests. The only hit is the word "placeholder" inside a test comment (`bundle_test.go:1006`) explaining that an unusable album name is reported and skipped rather than stored under one — which is the assertion, not a stub.

## Threat Flags

None. Every disposition in the plan's threat register is `mitigate` and each is in place:

| Threat | Where it is mitigated |
|---|---|
| T-11-27 (tampering, album name in an uploaded manifest) | `importAlbums` creates through `album.Store.Create` — `normalizeName`, `MaxNameLength`, `page.Slugify` — and writes no INSERT (comment-stripped count `0`) |
| T-11-28 (elevation, a picture not in the archive) | resolved through `mediaByName`, built from the media this import made for this website; a name that is not there resolves to nothing, never a number, and is reported |
| T-11-29 (disclosure, a reference passed through) | dropped on both sides — `exportBlocks` on an unknown slug, `importAlbumSlug` on an undeclared name |
| T-11-30 (DoS, very many albums) | `album.MaxItems` bounds each album, `maxExportPages` bounds the export, and the import is a loop of ordinary `Create`/`AddItem` calls with no transaction held across them |
| T-11-31 (tampering, a value silently lost or mistranslated) | all four pieces landed, both directions asserted, and the proving test renames first |
| T-11-SC (supply chain) | `go.mod` untouched; every new import (`internal/album`, `internal/page`) is inside this repository |

No new network endpoint, auth path, file-access pattern or schema change was introduced — this plan adds one optional JSON key and no migration.

## Website Isolation

The standing rule holds. `exportAlbums` reads only through `s.Albums.List(ctx, websiteID)` and `s.Albums.Items(ctx, websiteID, a.ID)`; `importAlbums` creates only through `s.Albums.Create(ctx, websiteID, …)` and `s.Albums.AddItem(ctx, websiteID, …)`. **Every one of those four signatures takes the website id as a parameter**, which is the lesson in `.planning/debug/knowledge-base.md` — the check lives where the storage is, so the compiler names every call site — and it is why this plan could not have made the defect that has now shipped six times: there is no way to call these without naming a website.

Proved end to end rather than only by signature: `TestImportCreatesAlbumsBeforePages` asserts the imported album's picture belongs to the **new** website, and `TestAlbumRoundTripAfterRename` asserts the copied album does **not** point at the original website's media row.

## User Setup Required

None — no external service configuration.

## Next Phase Readiness

- **Ready for 11-07.** It owns the browser pass and the string catalogue: **34 offen** now (33 from earlier Phase 11 work, +1 from this plan's `%d Alben` on the import report), measured by isolation above.
- **One thing 11-07 should know:** the four warning sentences this plan added to `internal/bundle/import.go` will **not** appear in its `offen` count, because they are `fmt.Sprintf`. They are on the ledger as entry 6. Closing them is not a Phase 11 job — it needs the operator's locale threaded into `bundle.Import`.
- `go build ./...` clean, `go vet ./...` silent, `gofmt -l .` silent, `go test ./...` **44 packages ok, 0 FAIL**.

## Self-Check

- `internal/bundle/format.go` — FOUND
- `internal/bundle/export.go` — FOUND
- `internal/bundle/import.go` — FOUND
- `internal/bundle/blocks.go` — FOUND
- `internal/bundle/bundle_test.go` — FOUND
- `internal/admin/bundle.go` — FOUND
- `cmd/holzcloud/templates/admin/import_report.html` — FOUND
- `60806b6` — FOUND
- `5f7709e` — FOUND
- `d4b579a` — FOUND
- `fb909bc` — FOUND

## Self-Check: PASSED

---
*Phase: 11-galerie*
*Completed: 2026-09-07*
