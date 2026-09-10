---
phase: 09-csv-import
plan: 04
subsystem: csv-import
status: complete
tags: [csv, wizard, admin-screen, download, routes, authorization, i18n]

requires:
  - internal/csv (plan 09-01) — CheckBytes, New, Reader, Row, RowNumber, MaxColumns, MaxRows, Example
  - internal/csvimport (plan 09-02) — Store, Upload, ErrExpired, ErrForeign
  - internal/csvimport (plan 09-03) — Columns, Column, Mappable, AutoMap, Mapping, Note, Reason, CollisionSkip, CollisionUpdate
  - internal/field — Def, List, Create, Kind constants
  - internal/domain — GetWebsite
  - internal/page — Slugify
  - internal/web — LayoutData, FormState, RenderAdmin, SetFlashError, Titlef
provides:
  - admin.HandleCSVImport, admin.staged, admin.HandleCSVMapping, admin.HandleCSVExample
  - admin.CSVMappingData, admin.CSVColumnView, admin.CSVExpiredData
  - admin.csvExampleColumns, admin.csvExampleCell, admin.csvExampleFilename
  - admin.csvModeNew, admin.csvModeExisting, admin.csvMaxUpload
  - templates csv_mapping.html and csv_expired.html, both in layoutPageNames
  - the third <details> panel on website_list.html, with the upload form and the example GET form
  - routes POST /admin/websites/import-csv, GET /admin/csv-import/{token}, GET /admin/csv-vorlage
affects:
  - plan 09-05 adds POST /admin/csv-import/{token}/probe and .../start, both through `staged`
  - plan 09-05 raises layoutPageNames from 46 to 48 and adminProtectedMux.Handle from 148 to 150
  - plan 09-06 owns the catalogues: this wave left 62 collectable strings untranslated on purpose

tech-stack:
  added: []
  patterns:
    - one `staged` helper is the single place a staging token becomes a response
    - a swept token renders a named screen at 200; a foreign one is a 404 before any byte is read
    - a hand-typed index in a query string is clamped, never refused
    - a download's filename slug comes from page.Slugify when the name's provenance is a website

key-files:
  created:
    - internal/admin/csvimport.go
    - internal/admin/csvimport_test.go
    - cmd/holzcloud/templates/admin/csv_mapping.html
    - cmd/holzcloud/templates/admin/csv_expired.html
  modified:
    - cmd/holzcloud/templates/admin/website_list.html
    - internal/web/render.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go

decisions:
  - the 10 MB cap is measured on the REQUEST BODY, where http.MaxBytesReader puts it, so a file of exactly 10 MB does not fit — the multipart envelope is part of what the cap counts
  - the too-many-columns refusal names the cap and not the count found, because internal/csv does not expose the count and widening it for one message was the worse trade
  - .alert / .alert--warning replace the plan's .callout, which has no rule anywhere in cmd/holzcloud/assets/
  - the form field names are `target` and `collision`; their VALUES stay German because migration 00049 pins them
  - test names are English, as waves 1–3 are, not the German names the plan's step 10 lists

metrics:
  duration: ~55m
  completed: 2026-09-06

actuals:
  tokens: 17300
  tasks: 3
  commits: 2
---

# Phase 9 Plan 04: The wizard's first two screens and the example CSV — Summary

The importer now hangs on a screen an admin already visits: a third `<details>`
panel on the website list uploads a `.csv`, stages it, and lands on a mapping
screen that lists the file's columns in the file's own order beside one real row
that can be stepped — with no file input anywhere after the first screen, and an
example CSV downloadable **before** anything is uploaded.

## What shipped

**`internal/admin/csvimport.go`** — a head comment block that fixes the wizard's
shape before any code (both half-analogs named, `twofactor.go:154`'s keying on
the user id explicitly dropped, the GET/POST split, the expiry-not-404 rule, the
`ServeMux` conflict that moved the routes, and the departure from
`wordpress.go:15-19`), then `CSVColumnView`, `CSVMappingData`, `CSVExpiredData`,
`HandleCSVImport`, `staged`, `HandleCSVMapping`, `HandleCSVExample` and the three
example helpers.

**`cmd/holzcloud/templates/admin/csv_mapping.html`** — the column list in file
order with the automatic match selected, the sample row and its `<a href>`
stepper, the per-target defaults, the pipe and Ja/Nein help text, the required-
group warning, and the example link. **`csv_expired.html`** — the named expiry
screen, which does not say the operator did anything wrong.

**The third panel** on `website_list.html`: the upload form with the
existing-vs-new choice and the update-or-skip choice, plus a second small GET
form for the example.

**`internal/web/render.go`** — `csv_mapping` and `csv_expired` in
`layoutPageNames`, 44 → 46.

**`cmd/holzcloud/main.go` / `main_test.go`** — three routes behind
`requireAdmin`, three rows in `adminOnly`, each route and its row in the same
commit.

Tests: 20 new tests in `internal/admin`, all passing; whole suite 43 `ok`, no
`FAIL`.

## Commits

| Task | Commit | Subject |
|---|---|---|
| 1 | `400f9ce` | feat(09-04): a file uploaded from a panel becomes a screen with its columns on it |
| 2 | `c9586ca` | feat(09-04): the example CSV is a GET, a snapshot, and every cell defused |
| 3 | — | no code; this summary is its artifact |

`git show --stat` confirms commit 1 touched exactly its eight named files and
commit 2 exactly its six. Nothing under `README.md`, `docs/`, `.github/`,
`CONTRIBUTING.md` or `SECURITY.md` was staged; `.planning/quick/…` stayed
untracked throughout. No `git add -A`, no `git commit -a`.

## Verification gate output, literal

### Task 1

```
=== gate 1: build/vet/gofmt ===        exit=0   (gofmt printed nothing)
=== gate 2: go test ./internal/admin/ -run CSV -v ===
    14 PASS, 0 FAIL, no "no tests to run"
=== gate 3: TestRouteAuthorization|TestAdminRoutesRequireASession ===
    --- PASS: TestRouteAuthorization (0.39s)
    --- PASS: TestAdminRoutesRequireASession (0.36s)
    ok  ... 1.241s     (no panic — no route pattern conflicts)
=== gate 4: layoutPageNames entries ===          46
=== gate 5: csv_mapping|csv_expired in render.go ===   1
=== gate 6: admin templates ===                  63
=== gate 7: <details panels ===                   3
=== gate 8: adminProtectedMux.Handle ===        147
=== gate 9: adminOnly rows ===                   16
=== gate 10: route names in the table ===         2
=== gate 11: script/javascript:/on*= ===
    cmd/holzcloud/templates/admin/csv_expired.html:0
    cmd/holzcloud/templates/admin/csv_mapping.html:0
=== gate 12: gorilla.csrf.Token in csv_mapping ===     1
=== gate 13: import-summary|import-warnings ===        0
=== gate 14: go build ./... && go test ./... ===  exit=0, 43 ok, 0 FAIL
```

The fourteen Task-1 tests:

```
TestCSVUploadStagesAndRedirects              PASS
TestCSVUploadIntoExistingWebsiteNamesIt      PASS
TestCSVTenMegabyteLimit                      PASS
TestCSVDegenerateFilesAreRefused             PASS
TestCSVEmptyHeadingIsColumnN                 PASS
TestCSVForeignTokenIsNotFound                PASS
TestCSVSweptTokenShowsExpired                PASS
TestCSVMappingIsInColumnOrder                PASS
TestCSVSampleRowSteppingIsClamped            PASS
TestCSVHeaderOnlySaysSoInsteadOfAnEmptyTable PASS
TestCSVMappingIsNotEmptyWithoutOwnFields     PASS
TestCSVMappingWarnsAboutARequiredGroup       PASS
TestCSVMappingFormLeadsToTheDryRun           PASS
TestCSVTemplatesCarryNoScript                PASS
```

### Task 2

```
=== gate 1: build/vet/gofmt ===                  exit=0
=== gate 2: go test ./internal/admin/ -run Example -v ===
    6 PASS, 0 FAIL, no "no tests to run"
=== gate 3: GET /admin/csv-vorlage in main.go ===      1
=== gate 4: /beispiel in main.go ===                   0
=== gate 5: X-Content-Type-Options (comments filtered) === 1
=== gate 6: safeDownloadName (comments filtered) ===   0
=== gate 7: internal/csv foreign deps ===              0
=== gate 8: adminProtectedMux.Handle ===             148
=== gate 9: adminOnly rows ===                        17
=== gate 10: all three routes named in the table ===   3
=== gate 11: TestRouteAuthorization ===   --- PASS (0.40s), no panic
=== gate 12: go test ./... ===            exit=0, 43 ok, 0 FAIL
```

```
TestCSVExampleHasHeaderAndBOM                       PASS
TestCSVExampleSkipsUnmappableKinds                  PASS
TestCSVExampleDefusesFormulas                       PASS
TestCSVExampleFilenameComesFromTheWebsite           PASS
TestCSVExampleUnknownWebsiteIsNotFound              PASS
TestCSVExampleIsASnapshotAndALaterFieldArrivesUnmapped PASS
```

### Task 3 — every number this wave moves, predicted against measured

| What | Baseline | This plan adds | Predicted | **Measured** | Divergence |
|---|---|---|---|---|---|
| `layoutPageNames` entries | 44 | 2 | 46 | **46** | none |
| admin templates | 61 | 2 | 63 | **63** | none |
| `adminProtectedMux.Handle` | 145 | 3 | 148 | **148** | none |
| `adminOnly` rows | 14 | 3 | 17 | **17** | none |
| `<details` panels | 2 | 1 | 3 | **3** | none |
| files using `BeginTx` in `internal/` | 14 | 0 | 14 | **14** | none |
| `BeginTx` over `csv/` + `csvimport/` + `admin/csvimport.go` | 0 | 0 | 0 | **0** | none |
| admin CSS files | 2 | 0 | 2 | **2** | none |
| migrations | 49 | 0 | 49 | **49** | none |

Two further gates, carried forward from earlier waves:

```
go.mod / go.sum diff against HEAD                  exit=0 (untouched)
field.SlugifyKey( outside internal/csvimport/mapping.go   0
```

The `SlugifyKey` gate is run in the **parenthesised** form wave 3's summary
prescribed (`grep 'field\.SlugifyKey('`), not the bare word. It measures 0, which
is the property: the header fold has exactly one implementation.

**Three numbers that must not move did not move.** `BeginTx` is still 14 files
across `internal/`, and 0 across the importer's three paths — `csvimport.go`
opens no transaction anywhere. The CSS file count is still 2: no new stylesheet,
and plan 09-05 appends an `@layer components` block to `admin.css` rather than
creating one.

## Counting divergences

### 1. `tools/i18n` moved from 0 offen to 62 offen — upward, and it is the D-32 proof

**Direction:** upward, from wave 3's measured 0.

**Cause:** this wave adds 62 new user-visible German strings — two templates, the
panel, the flashes. Every one of them was written through `{{t}}`, `{{th}}`,
`{{tf}}`, `web.SetFlashError` or `web.Titlef` **by name**, which is why the tool
can see them at all. A `fmt.Sprintf` sentence would have left the counter at 0
while the screens were German-only, which is exactly the hole D-32 exists to
close; the rise from 0 to 62 is the evidence that no such sentence was written.

**Nothing was translated.** The prompt and the phase plan both put the catalogue
work in plan 09-06, and adding keys here would have moved that work out of the
plan that owns it. `0 verwaist` throughout — no orphaned key was introduced.

### 2. The plan's step 10 lists German test names; English were written

**Direction:** the plan's own `-run CSV` gate would still have passed either way,
but the phase's English mandate and waves 1–3's practice settle it.

**Cause:** the same leftover wave 3 recorded as its divergence 3 — a list written
before the plan was turned to English. All fourteen Task-1 tests are named
`TestCSV…` so the plan's `-run CSV` selector matches every one of them, and all
six Task-2 tests are named `TestCSVExample…` so both `-run CSV` and `-run Example`
match. Same tests, same properties, only the names turned.

### 3. `internal/admin` now has 20 CSV tests where the plan lists 11

**Direction:** upward, away from any failure threshold.

**Cause:** three properties the plan states in prose but does not put in its test
list got tests of their own — the existing-website target naming its website
(`TestCSVUploadIntoExistingWebsiteNamesIt`), the required-group warning the wave-3
summary flagged forward (`TestCSVMappingWarnsAboutARequiredGroup`), and the dry-run
form's `action` plus the absence of any file input on screen 2
(`TestCSVMappingFormLeadsToTheDryRun`). A stated guarantee with no gate is not a
guarantee — wave 2's words, applied again.

## Deviations from plan

### 1. [Rule 1 — Bug] The 10 MB cap is on the request body, so a file of exactly 10 MB does not fit

**Found during:** Task 1, writing `TestCSVTenMegabyteLimit`.

**Issue:** the plan's `<behavior>` says "Posting a file of exactly 10 MB is
accepted; 10 MB + 1 is refused", and its acceptance criterion mandates
`http.MaxBytesReader(w, r.Body, 10<<20)` as the handler's first statement. Those
two cannot both hold. `MaxBytesReader` bounds the **whole request body**, and a
multipart envelope around a 10 485 760-byte file adds a boundary, two part
headers and the terminator — so a file of exactly 10 MB arrives as a body of
rather more than 10 MB and is refused. The same is true of `wordpress.go:25`,
which the plan copies.

**Fix:** the acceptance criterion is honoured literally — `MaxBytesReader` is the
first statement with the byte count unchanged — and the boundary is proved where
the cap actually lives. `TestCSVTenMegabyteLimit` measures the envelope's
overhead with a fixed multipart boundary, sizes the file so the **body** is
exactly `10<<20`, asserts that it is accepted and staged, then adds one byte and
asserts a flash, a 303 and no second staged row. That is the cap at the boundary
and one step either side, at the handler, with nothing bent to make it true.

**Files:** `internal/admin/csvimport_test.go`. **Commit:** `400f9ce`.

### 2. [Rule 3 — Blocking] The too-many-columns message names the cap, not the count found

**Issue:** the plan says the D-38 refusal should name "the count it found and the
cap". `internal/csv` refuses the file inside `csv.New` and returns
`fmt.Errorf("%w: %d", ErrTooManyColumns, len(header))`; the count is in the
error's **text** and nowhere in its API. Recovering it would mean parsing an
error string, and exposing it would mean widening `internal/csv`'s surface for
one sentence.

**Fix:** the flash reads „Die Datei hat mehr als %d Spalten. Es wurde nichts
abgelegt." through `web.Titlef`, which is one of the eight functions `tools/i18n`
collects, so the sentence stays translatable. The operator is told the cap and
that nothing was staged, which is the actionable half. Recorded here rather than
adjusted in either direction.

**Files:** `internal/admin/csvimport.go`. **Commit:** `400f9ce`.

### 3. [Rule 2 — dead CSS hook] `.callout` has no rule anywhere; `.alert` was used

**Issue:** the plan's step 6 prescribes `.callout` for both new templates, and its
own acceptance criterion forbids using a class with no rule in
`cmd/holzcloud/assets/admin.css`. Measured: `grep -c callout` is **0** in both
`admin.css` and `bausteine.css`. `.callout` is a third dead hook of exactly the
kind the plan warns about with `.import-summary` and `.import-warnings` —
`import_report.html` uses it today and it styles nothing.

**Fix:** `.alert` and `.alert--warning`, which are real rules in `admin.css`, for
the truncation notice, the required-group warning and the expiry screen. Every
other class used on the two templates was checked against the same list:
`.card`, `.table`, `.form-group`, `.form-label`, `.form-input`, `.form-select`,
`.form-check`, `.form-check-label`, `.form-hint`, `.badge`, `.text-muted`,
`.btn`, `.btn--primary`. No new class was minted.

**Files:** both new templates and `website_list.html`. **Commits:** `400f9ce`,
`c9586ca`.

### 4. [Rule 3 — Blocking] The plan names the target form field twice, once `target` and once `ziel`

**Issue:** Task 1's `<action>` says "`target` is `neu` or `bestehend`" and its
`<behavior>` says "Posting with `ziel=bestehend`".

**Fix:** the form field names are `target` and `collision` — English, as the
wave's mandate requires for identifiers. Their **values** stay `neu`,
`bestehend`, `uebergehen` and `aktualisieren`, because migration 00049 pins those
four in CHECK constraints: they are data, and translating one compiles cleanly
and then has SQLite refuse the row at runtime. The query parameter `?zeile=`
stays German because it is an address, and this tree's admin URLs are German
throughout (`/admin/protokoll`, `…/uebersetzen`, `…/vergleich`).

### 5. [Rule 2] A deleted target website ends the wizard with a message and deletes the staged row

**Issue:** D-29 and the plan require the mapping screen to re-check the target and
"end the wizard with a message rather than a nil dereference", without saying
what becomes of the staged bytes.

**Fix:** the row is deleted and the operator is flashed
„Die Website dieses Imports gibt es nicht mehr. Der Import wurde abgebrochen;
geschrieben wurde nichts.", then redirected to the website list. Leaving the row
standing would leave up to ten megabytes waiting a day for a sweep to take, on
behalf of a wizard that can no longer be finished. `00049`'s
`ON DELETE SET NULL` comment says the row stays so the screen **can** explain
what happened; it does explain, and then it clears up.

### 6. [Rule 2] `HandleCSVMapping` answers an unparseable staged file rather than 500-ing

**Issue:** re-reading the staged bytes with `csv.New` can in principle fail. It
cannot in practice — screen 1 parsed those exact bytes before staging them and the
row is never updated — but a returned error becomes a bare 500 on a screen an
operator may have reached from a bookmark.

**Fix:** a flash and a redirect, with the comment saying the branch is
unreachable and why it is answered anyway.

## Notes for plan 09-05

- **`staged` is the only place `ErrForeign` and `ErrExpired` become responses.**
  Both new screens must call it and neither may re-implement the rule. Its
  signature is `(*csvimport.Upload, bool, error)`; a `false` means the response
  has already been written.
- **`layoutPageNames` is at 46** and must reach 48. `adminProtectedMux.Handle` is
  at 148 and must reach 150; `adminOnly` is at 17 and must reach 19.
- **The mapping form already posts to `/admin/csv-import/{token}/probe`** and
  sends `ziel_<index>` per column plus `default_status` and
  `default_field:<key>` per target. `TestCSVMappingFormLeadsToTheDryRun` asserts
  the action; until 09-05 registers that route the button 404s, which is expected.
- **The report screen needs its own template**, not `import_report.html`: that
  file's `.import-summary` and `.import-warnings` have no rule anywhere, and so
  does its `.callout`. Three dead hooks, not two.
- **`?zeile=` steps with plain `<a href>` links**, so a stepped row loses unsaved
  select changes. D-38 anticipates a `formmethod="GET"` submit that carries the
  mapping in the query string; that is a 09-05 refinement and the column cap of
  100 is what makes it affordable.

## Known Stubs

None. Every symbol this plan declares is reached by a test in `internal/admin`,
and both new templates are rendered from disk by those tests rather than only
parsed.

The one thing no test in this repository can see remains `layoutPageNames`: a
name missing from it renders a bare fragment with no navigation, no flash area
and no CSRF body attribute. The count gate reads 46 and both names are in it; the
browser pass in plan 09-06 is what actually looks.

## Threat Flags

None beyond the register the plan already carries. Three new routes were added
and all three are in `TestRouteAuthorization`'s table in the same commit as their
registration (T-09-22). The staging token's ownership check is asserted with a
real second admin account and the assertion reads the response **body**, not only
the status, so a 404 issued after a render would fail (T-09-23). No `template.HTML`
cast exists on this road; no transaction is opened (`BeginTx` measured 0 over all
three paths); `go.mod` and `go.sum` are untouched, so T-09-SC does not fire.

## Self-Check: PASSED

- `internal/admin/csvimport.go` — FOUND
- `internal/admin/csvimport_test.go` — FOUND
- `cmd/holzcloud/templates/admin/csv_mapping.html` — FOUND
- `cmd/holzcloud/templates/admin/csv_expired.html` — FOUND
- `cmd/holzcloud/templates/admin/website_list.html` — FOUND (3 `<details>` panels)
- `internal/web/render.go` — FOUND (46 entries in `layoutPageNames`)
- `cmd/holzcloud/main.go` — FOUND (148 `adminProtectedMux.Handle`)
- `cmd/holzcloud/main_test.go` — FOUND (17 `adminOnly` rows)
- commit `400f9ce` — FOUND
- commit `c9586ca` — FOUND
