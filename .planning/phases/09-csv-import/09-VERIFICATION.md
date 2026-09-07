---
phase: 09-csv-import
verified: 2026-09-06T19:48:02Z
status: passed
score: 6/6 criteria verified — die zwei benannten Lücken sind am 2026-09-08 geschlossen, siehe Nachtrag am Ende
score_note: >-
  4/6 beim Abschluss des Verifizierers. Beide Lücken sind seither behoben und
  gegen HEAD grün geprüft; das ursprüngliche Urteil steht unverändert darüber.
behavior_unverified: 0
overrides_applied: 0
requirements: [IMP-01, IMP-02, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08, IMP-09, IMP-10]
decision_coverage: 38/38
gaps:
  - truth: "Criterion 5 / IMP-10 — no transaction ever spans more than one row"
    status: partial
    reason: >-
      The importer opens no transaction of its own (grep BeginTx over
      internal/csv, internal/csvimport and internal/admin/csvimport.go returns
      0, and the internal/ file count is unchanged at 14). But the write path
      calls term.EnsureNames ONCE for the whole file, before the row loop
      (internal/admin/csvimport.go:819), and that store method opens a single
      BeginTx (internal/term/store.go:330) covering every distinct term name in
      the file. That is one transaction on the one-connection write pool whose
      write set is derived from every row and whose duration grows with the
      file — the failure mode criterion 5 names. The amendment stamp of
      2026-09-06 enumerated three transactions (CreatePage, SetForPage,
      EnsureNames-per-row) and did not name this file-wide fourth shape. The
      phase's own mechanical gate is worded "no call THE ROW FUNCTION makes may
      hold a transaction across two rows" (09-EDGES-resolutions.json,
      IMP-10/concurrency), and EnsureNames is not called by the row function —
      so the gate is structurally unable to see the one case that matters.
    artifacts:
      - path: internal/admin/csvimport.go
        issue: "line 819 — EnsureNames for the whole file, before the row loop"
      - path: internal/term/store.go
        issue: "line 330 — one BeginTx around every INSERT of that call"
    missing:
      - "Either: batch EnsureNames per row (or per bounded chunk), so no single transaction covers names harvested from more than one row."
      - "Or: an amendment stamp on IMP-10 and on criterion 5 naming this fourth transaction explicitly, the way D-02 named the other three — the project's own rule is that a weaker guarantee is never shipped silently."
      - "Either way, a gate that measures what its name claims: BeginTx reachable from the whole write path, not only from the row function."
  - truth: "Criterion 3 — the admin sees per row what would be created, updated or skipped before committing"
    status: partial
    reason: >-
      The dry run genuinely writes nothing and genuinely reports per row with
      the spreadsheet's row number and a reason. But its per-row verdict is
      provably wrong for a file that carries two rows at one new address: the
      dry run cannot know about pages the same run is about to create, so it
      predicts create/create and the write does create/update. The executor
      measured exactly this (5 anlegen / 2 aktualisieren against 4 angelegt /
      3 aktualisiert) and recorded it in deferred-items.md §4 — nothing on
      either screen tells the operator the two may differ, and an operator who
      compares them cannot tell it from a bug. The one test named as the guard
      against the two screens drifting apart,
      TestCSVProbeAndStartAgreeOnEveryVerdict, uses a fixture with no duplicate
      address in it, so it cannot see this case.
    artifacts:
      - path: cmd/holzcloud/templates/admin/csv_dryrun.html
        issue: "no sentence saying a row whose address a row above it takes is counted differently at write time"
      - path: internal/admin/csvimport_test.go
        issue: "TestCSVProbeAndStartAgreeOnEveryVerdict:1082 — fixture carries no duplicate address, so the one divergence the phase knows about is outside the test that claims to guard it"
    missing:
      - "One sentence on the dry run or on the report naming the divergence."
      - "A test case with two rows at one new address, asserting the divergence deliberately rather than leaving it unasserted."
human_verification:
  - test: >-
      Import a CSV of ~5000 rows in which a Schlagwörter column (or a schlagwort
      field column) carries several thousand DISTINCT term names, and while the
      commit is running, load any other admin page and any public page from a
      second browser context.
    expected: >-
      Neither request is blocked for a perceptible time. If both stall until the
      import finishes, the EnsureNames transaction is holding the single write
      connection for a duration proportional to the file, which is the exact
      anti-feature criterion 5 exists to forbid.
    why_human: >-
      No test measures wall-clock contention on the write pool, and the size of
      the window cannot be read off the code — it depends on the number of
      distinct term names in a real file. The executor's browser pass drove two
      concurrent commits, but never a large term vocabulary.
coincidental_reliance_items:
  - truth: "Criterion 4 — reports every slug that was renamed on collision"
    reason: fixture-only
    harden: >-
      Both proofs construct a precondition the production path never produces.
      TestRenamedSlugIsReported (row_test.go:243) calls WriteRow with existing =
      nil, bypassing the handler's per-row live lookup; TestCSVReportNamesEveryRename
      (csvimport_test.go:1164) fabricates the Verdict and renders the template
      directly. No test drives a rename through csvRun, because csvRun's live
      per-row GetPageBySlug turns every deterministic collision into an update
      or a skip first. The clause is still TRUE (every rename that occurs is
      reported) — it is the reachability that is untested, and the executor
      reported it as "not driven" honestly rather than claiming it.
warnings:
  - >-
    Two row counts sit on one screen counted two different ways. The mapping
    screen reads "5 Spalten, 12 Zeilen" three lines above "Beispiel: Zeile 13
    von 13"; the report reads "11 Zeilen gelesen" above a group naming "Eine
    Zeile: 12". Both numbers are correct under their own definition (data rows
    vs the spreadsheet's row number, D-26), and the browser pass fixed the worse
    half of this ("Zeile 13 von 12"). Confirmed by reading 04c-mapping-last-row.png
    and 06b-report-grouped.png directly. It is the same confusion one step back.
  - >-
    Per-target defaults are rendered for Zustand and for each custom field only
    (csv_mapping.html:135-142). csvMappingFromForm accepts default_<any target>,
    and TestBlankCellSaysNothingOnTheUpdateArm asserts the body default applies —
    a capability with no control on the screen. Defensible against the wording
    ("each FIELD can carry a default"), but the CR-01 test proves one third of
    itself through a door the operator cannot open.
  - >-
    Imported pages carry locale = "" because CheckRow's page.PageCreate never
    sets Locale, while the admin form sets locale.Pick(...) (internal/admin/page.go:171,181).
    On a monolingual website both are "" and the pages really are
    indistinguishable; on a multilingual website they are not, and
    GetPageBySlug matches across locales while UNIQUE is (website_id, locale, slug)
    (00045_pages_locale_unique.sql:71). Locale is not one of the three properties
    criterion 4 names, so this is recorded rather than counted against it.
---

# Phase 9: CSV Import — Verification

**Goal:** content can arrive as a table — uploaded, mapped, previewed, then written through the ordinary creation path with an honest per-row account.
**Verified:** 2026-09-06 · goal-backward, from the six ROADMAP success criteria including every dated stamp.
**Mode:** initial verification. No previous VERIFICATION.md.

Tree state confirmed by command, not by SUMMARY: `go build ./...`, `go vet ./...`, `gofmt -l .` all silent. `go test ./internal/csv/... ./internal/csvimport/... ./internal/admin/... ./cmd/...` — four packages `ok`, zero failures. HEAD is `39d4260`, a docs-only commit; the last commit touching anything a person can see is `d4e56f7`, which is inside the browser pass.

**Which stamps were read as binding.** Three amendments govern this phase and all three were taken over the original wording:

1. **Criterion 5 — "Amended 2026-09-06, vor der Planung"** (ROADMAP.md:292). "One transaction per row" is withdrawn; **"no transaction ever spans more than one row"** stands, absolute. `REQUIREMENTS.md` IMP-10 carries the matching stamp. This is the sentence criterion 5 is verified against below, and it is where the phase's one real gap is.
2. **Criterion 1 — "Note added at planning, 2026-09-06"** (ROADMAP.md:340). D-06's route table panics `newRouter` at startup. The token-bearing screens move to `/admin/csv-import/{token}`. Verified as built that way.
3. **The file-carrying planning note — "Overridden 2026-09-06, vor der Planung"** (ROADMAP.md:356). "Re-submit the file with the mapping" is unbuildable; the file is staged server-side as raw bytes. Verified as built that way, and it is what makes criterion 3's IMP-05 half true at all.

---

## Criterion 1 — four screens, and both target paths reach the same mapping screen

**Verdict: VERIFIED.**

Five routes, registered under the shape the D-36 note forced, and every one of them in the safety net:

| route | handler | in `TestRouteAuthorization` |
|---|---|---|
| `POST /admin/websites/import-csv` | `HandleCSVImport` | `main_test.go:173` |
| `GET /admin/csv-import/{token}` | `HandleCSVMapping` | `:174` |
| `POST /admin/csv-import/{token}/probe` | `HandleCSVDryRun` | `:175` |
| `POST /admin/csv-import/{token}/start` | `HandleCSVStart` | `:176` |
| `GET /admin/csv-vorlage` | `HandleCSVExample` | `:177` |

`cmd/holzcloud/main.go:889-896`, all five wrapped in `requireAdmin`. The `adminOnly` table is 14 → 19, as the plan predicted. The binary builds and the suite's router tests pass, so the panic the note describes is not present.

- **The third panel exists** — `grep -c '<details' website_list.html` = 3, the CSV panel at `:55`, carrying `target=neu` / `target=bestehend` radios (`:65`, `:74`) and the collision choice.
- **Both paths reach the mapping screen.** `TestCSVUploadStagesAndRedirects` (existing-website fixture, `target=neu`) asserts the 303 to a 32-hex token and that **no website was created** — screen 4 creates it (`HandleCSVStart`, the `csvModeNew` arm), so an abandoned wizard leaves nothing behind. `TestCSVUploadIntoExistingWebsiteNamesIt` follows the redirect into `HandleCSVMapping` and asserts 200 with the website named on the screen. `csv_mapping.html:52` guards the one website-dependent block with `{{if .Website}}`, so the new-website path renders the same screen without a nil dereference.
- **The fourth screen is a rendered response and not a fifth route**, which is what makes a refresh land on the expiry screen instead of importing twice — `TestCSVReloadDoesNotImportTwice`, and the claim-before-the-loop at `csvimport.go:829` that WR-05 put there.
- **Seen, not only asserted:** `01-panel-first-ever-import.png`, `01b-panel-with-a-website.png`, `04a`…`04d`, `06a`, `06b`, `07a`, `07b`. I opened `04c` and `06b` myself; both render with the full layout, one `<h1>`, sidebar, topbar and flash area.

---

## Criterion 2 — the mapping screen: targets, automatic matching, override, sample row, defaults, example CSV

**Verdict: VERIFIED.** One narrower-than-claimed note, carried in `warnings` rather than against the criterion.

- **Targets, including explicitly nothing.** `csv_mapping.html:111-121` — a `<select>` per column with `none`, `title`, `slug`, `body`, `status`, `terms` and one option per mappable field. `TestImageAndRefAreNoTarget` and `TestWithoutFieldsTheFourFixedTargetsRemain` hold both ends of D-18.
- **Automatic matching ignoring case and accents, and D-28's correction is in the tree.** `internal/csvimport/mapping.go:80` `foldHeader` = `field.SlugifyKey(settleHeaderMarks(header))`. The CONTEXT's own correction is the load-bearing part: the original "strip `unicode.Mn`" would have produced `grosse` where the field carries `groesse`. `settleHeaderMarks` composes the diaeresis back onto a/o/u and drops the rest. Proved, not asserted:
  - `TestFoldHeaderComposesCombiningUmlauts` writes both spellings as escapes (`ö` vs `ö`), asserts they are different strings first so the test cannot pass vacuously, then asserts both fold to `groesse` — the key `SlugifyKey` derives from the label.
  - `TestFoldHeaderAgreesOnEveryOtherAccent` (the WR-01 fix) does the same for é, ñ, ç, å and š, and asserts the key is the one `SlugifyKey` derives — `caf`, not `cafe`. This is the hole the original three-line fix left.
  - `TestFoldCellKeepsTheBaseLetter` holds the other consumer: a *cell* folds through `page.Transliterate` and keeps its letters. Two folds, one loop, one flag — asserted so a later edit cannot silently move a status column outside its own vocabulary.
- **Every automatic match is overridable** — the `<select>` carries `Selected` and `csvTargetFromForm` reads `target_N` back. `TestSecondHeadingSaysWhyItStaysEmpty` and `TestTwoEqualHeadingsDoNotCollide` hold D-27: a column is addressed by index, the second `Titel` is left unmapped **with a sentence in place**, not silently dropped.
- **One real row, steppable.** `TestCSVSampleRowSteppingIsClamped` drives `row=1`, `row=4`, `row=99`, `row=-4`: out of range is clamped at 200, never refused; "vorherige" absent on the first row, "nächste" absent on the last — absent, not disabled. `TestCSVSteppingKeepsTheMapping` (WR-07) holds the property the browser checked hardest: the step is a `formmethod="GET"` submit of the mapping form, so the operator's overrides and typed defaults survive it.
- **Defaults per field.** `csv_mapping.html:135-142` — `default_status` plus one `default_field:<key>` per field. Confirmed on `04c-mapping-last-row.png`: the *Vorgaben* card, with `Zustand` in it.
- **Example CSV, before anything is uploaded** — D-37's placement fix. `GET /admin/csv-vorlage?website={id}` and a second, fieldless form for a website that does not exist yet. `TestCSVExampleWithoutAWebsiteIsTheFixedColumns`, `TestCSVExampleHasHeaderAndBOM`, `TestCSVExampleSkipsUnmappableKinds`, `TestCSVExampleDefusesFormulas`, `TestCSVExampleFilenameComesFromTheWebsite`. Driven in the browser at `02-example-before-upload.png` with **no website in existence** — filename `website-vorlage.csv`, BOM present, the pipe visible in the multi-value sample cell.
- **The two-count confusion** (see `warnings`): "12 Zeilen" and "Zeile 13 von 13" stand three lines apart on the same screen. Both correct, both counted differently. The browser pass fixed the worse half of this and left the milder half.

---

## Criterion 3 — the whole file through validation, writing nothing

**Verdict: PARTIAL. The "writes nothing" half is verified beyond doubt. The "see per row what WOULD happen" half has a known, measured case where the prediction is wrong and the screen does not say so.**

**What holds:**

- **Nothing is written.** `TestCSVProbeWritesNothing` counts pages before and after through the real handler and asserts the staging row survives. `TestDryRunWritesNothing` does the same at the unit level. The dry run reaches `csvimport.CheckRow`, which touches no store at all (`row.go:250`), and never `WriteRow`. `csvRun`'s `write` flag is the only difference between the two runs — D-22 as designed, and there is genuinely only one decision function.
- **Seen in the browser and not only asserted:** a second tab opened on `/admin/websites` during the dry run showed **0 websites** (`05b-second-tab-empty.png`). Not the pages, not even the website.
- **Per row, with the row number and the reason.** D-26's helper is the single mint: `csv.RowNumber(index) = index + 2`, `TestRowNumberIsTheSpreadsheetsOwn`. The mapping screen, the dry run and the report all call it, and the browser pass cross-checked one row across all three screens.
- **The dry run and the write read the same staged bytes** — both `csvRun` callers build their reader from `upload.Data` (`csvimport.go:797`, `:826`). That is IMP-05/concurrency and it is exactly what the overridden "re-submit the file" note could not have delivered.
- **The stale-target edge (IMP-04/concurrency, D-29) is closed at both ends.** `TestCSVDeletedTargetWebsiteEndsTheWizard` — the website deleted between screens ends the wizard with a named message and clears the staged row, not a nil dereference. `TestCSVDeletedFieldDefinitionIsReported` — the column falls back to unmapped, the row says so, and the rest of the row still imports.

**What does not hold, and it is the phase's own finding rather than mine:** for two rows at one new address the dry run says *create / create* and the write does *create / update*. The executor measured `5 anlegen / 2 aktualisieren` against `4 angelegt / 3 aktualisiert` and wrote it into `deferred-items.md` §4 with the correct explanation — and then did not close it, for a defensible reason (a new sentence in five languages on a screen whose wording was settled in 09-05). But criterion 3's promise is that the operator *sees what would happen before committing*, and for that shape of file they do not. `TestCSVProbeAndStartAgreeOnEveryVerdict` is named in the code as "the one test that would catch the dry run and the write drifting apart"; its fixture has no duplicate address, so it is exactly the test whose name sounds right and whose assertion is narrower than the claim. Recorded as a gap.

---

## Criterion 4 — pages indistinguishable from hand-made ones, because the importer calls the same creation path

**Verdict: VERIFIED.** This was checked adversarially — any second path would fail the criterion however well it worked — and there is no second path.

Every step of the row is the ordinary call, and the ordinary call is the *only* one:

| what the criterion names | what `internal/csvimport/row.go` calls | second implementation? |
|---|---|---|
| slug generated | `page.Slugify` (`:247`), `page.ValidateSlug` (`:313`) | none |
| slug unique per website | `page.CreatePage` (`:527`) and its own retry loop (`store.go:469-489`) | none — no pre-dedup, deliberately (D-23) |
| Markdown rendered and sanitised | `page.RenderMarkdown` (`:328`) — goldmark then `sanitizer.Sanitize`, `page/markdown.go:79-85` | none; no `template.HTML` cast anywhere on this road |
| validation | `field.CheckAll` (`:396`), then `field.Clean` (`:415`), then `field.Encode` (`:416`) | none |
| multi-value encoding | `field.JoinValues` (`:369`) — the function Phase 7 hardened for this caller | none; the newline string is never built here |
| terms | `term.Normalize` → `page.Slugify` → `term.EnsureNames` → `term.SetForPage` | none |
| draft unless a status column says otherwise | `parseStatus`, closed vocabulary; empty cell → `draft` on the create arm | — |

Confirmed by grep across the whole tree: `page.RenderMarkdown` has fifteen callers and one definition; the CSV importer is one of the fifteen, beside `wordpress.go:128` and `admin/page.go:367`. `page.CreatePage` is called, not extended, and takes no transaction parameter. `internal/wxr`, `internal/admin/wordpress.go` and `internal/bundle` are untouched.

**Mixed file, good rows in, bad rows named:** `TestCSVMixedFileImportsTheGoodRows` runs a four-row file through the real `HandleCSVStart` and asserts the database holds exactly `[alpha delta]`, the counters read `2 angelegt / 2 übergangen`, and **both** refusal sentences are on the report. `TestCSVEveryReasonHasASentence` closes the class rather than the instance: it parses `verdict.go` for every `Reason` code and `csv_reason.html` for every arm and fails on a code with no arm, an arm with no code, or two arms for one code — so a reason can never render as an empty cell reading "no reason".

**Nothing half-written:** see criterion 5.

**The one clause proved only through a door the production path never opens** — "reports every slug that was renamed on collision". Recorded in `coincidental_reliance_items`. The clause is true; its *reachability* is what is untested, and the executor said so rather than claiming it. Judged below under criterion 6.

---

## Criterion 5 — a hostile file refused, and no transaction over more than one row

**Verdict: PARTIAL. The hostile-file half is verified at every boundary. The compensation half is verified on both arms, including the arm where it must be absent. The transaction sentence — the amended one — is not literally true.**

### The hostile file: verified

Every cap is tested at the boundary and one step either side, which is what the IMP-09/boundary resolution demanded (six cases, not three):

| defence | constant | boundary proof |
|---|---|---|
| bytes | `csvMaxUpload = 10 << 20` (`csvimport.go:95`), `http.MaxBytesReader` at `:264` | `TestCSVTenMegabyteLimit` — body of exactly the cap accepted and staged, `+1` refused with a flash and nothing staged |
| rows | `MaxRows = 5000` | `TestRowLimit` — exactly `MaxRows` not truncated, `MaxRows+1` truncated **and reported**, never silently applied |
| columns | `MaxColumns = 100` (D-38, the gap D-08…D-16 left) | `TestColumnLimit` — 100 accepted, 101 → `ErrTooManyColumns` |
| cells | `MaxCellBytes = 100000` | `TestCellLimitInBytes` — exact/+1, and the emoji case both ways, so the limit provably counts bytes and not runes |
| BOM | stripped once, at the reader | `TestBOMIsStrippedOnce` — and a second BOM mid-file stays as content |
| stray quote | `LazyQuotes = true` | `TestStrayQuoteDoesNotEatTheRest` — one bad row, the rest of the file still read |
| short row | `FieldsPerRecord = -1` + padding | `TestShortRowDoesNotShift` — cell 5 of a 3-cell row is empty **in position**, and `TestCellLimitInBytes` asserts the row *number* on the refusal |
| NUL byte | `CheckBytes` on the raw bytes before parsing | `TestCheckBytesRefuses` — including a NUL behind the BOM |

**On the byte cap, the D-08 correction is honest and the test matches it.** "A file of exactly 10 MB" cannot be built because `MaxBytesReader` bounds the whole multipart body. The test measures the envelope and proves the boundary **where the cap actually lives**. That is the same guarantee, measured at the place the code enforces it — and `internal/admin/wordpress.go:25` carries the same inherited shape, so the phase named a pre-existing imprecision rather than creating one.

Screen 1's refusals are one message each, and the test asserts they are *distinguishable*: `TestCSVDegenerateFilesAreRefused` fails if two of the three share a sentence. Driven in the browser too — five files, five sentences, website count 0 after each (`03-hostile-*.png`).

### The compensation: verified, including the branch that must not exist

D-02's rule is asymmetric and the asymmetry is the whole point. `WriteRow` (`row.go:498-551`):

- **Create arm** — `SetForPage` fails → `TrashPage` then `PurgePage`, the ordinary two steps, no new store method. If the compensation itself fails the verdict says so and names the page.
- **Update arm** — `SetForPage` fails → the row is reported `OutcomeUpdate` / `ReasonNotRolledBack` and **nothing is undone**. There is no `TrashPage` on this arm. I read the branch; it is absent.

Both are asserted, which matters because a branch that must not run leaves no trace of not running:

- `TestRowIsRolledBack` opens a second, *closed* database for the term store so `SetForPage` fails on its first statement, then asserts the verdict is a skip, that it is not `ReasonNotRolledBack`, and that **zero pages** are left behind.
- `TestUpdateArmIsNotRolledBack` fails only the terms step (a website id no `websites` row carries, so the foreign key bites in `terms` and nowhere else), then asserts the operator's own page is **still there** and carries the update it did get. The failure message says what the test exists for: *"the operator's own page was DELETED by the recovery path"*.

### The transaction sentence: the gap

`grep -rn BeginTx internal/csv internal/csvimport internal/admin/csvimport.go` returns **nothing**. The `internal/`-scoped file count is **14**, unchanged from the baseline. No mutex, no lock, no held connection across the loop; `Store.Claim` is a single autocommitted `DELETE`; the per-row `GetPageBySlug` goes to the separate read pool. The anti-feature the criterion exists to forbid — a transaction wrapping the page writes of a whole file — is genuinely absent, and I looked for it.

**But one transaction on the write path does span the whole file.** `internal/admin/csvimport.go:808-822`, on the `write` arm and before the row loop:

```
if names := csvimport.TermNames(defs, m, rows); len(names) > 0 {
    if _, err := h.terms.EnsureNames(ctx, websiteID, names); err != nil {
```

`TermNames` collects every distinct term name **the whole file mentions** (`row.go:160-200`), de-duplicated but unbounded — up to 5000 rows × `term.MaxPerPage` names. `term.EnsureNames` (`term/store.go:329-352`) opens one `BeginTx` on `DB.Write` and runs one `INSERT … ON CONFLICT DO NOTHING` per name inside it. So there is exactly one transaction, on the pool that admits exactly one connection, whose write set is derived from every row of the file and whose duration grows with the file. That is the sentence "no transaction ever spans more than one row" being false, and it is false in the direction the criterion cares about.

Three things make this a gap worth naming rather than a nitpick:

1. **The amendment enumerated the transactions and missed this shape.** D-02's table names `CreatePage`, `SetForPage` and `EnsureNames` — but `EnsureNames` *as a per-row call*. Hoisting it to once-per-file (D-20, correctly copying `internal/bundle/import.go:289-329`) changed its scope from one row to the file, and no stamp records that.
2. **The phase's own gate cannot see it.** IMP-10/concurrency reads: *"grep the importer for BeginTx and expect zero, and assert that no call **the row function** makes holds a transaction across two rows."* `EnsureNames` is not called by the row function. This is precisely the failure the CONTEXT itself catalogues three times — *"a command measuring something adjacent to its name"* — occurring a fourth time, in the gate for the requirement the phase amended most carefully.
3. **The project's own standard applies.** D-02 says in as many words: *"The plan must not silently ship a weaker guarantee than the requirement's wording."* IMP-10 got a stamp for exactly this reason. This deviation has code comments and a design decision behind it, but no stamp — and the code comment at `csvimport.go:786-790` inventories the loop's transactions and does not mention that one stands outside the loop covering all of it.

The fix is small either way: batch `EnsureNames` per row or per chunk, or stamp the requirement. What must not happen is that it stays unstated.

---

## Criterion 6 — the standing gate (QUAL-01, QUAL-02)

**Verdict: VERIFIED.**

### QUAL-01 — run by me, literal output

```
$ go run ./tools/i18n
1271 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1271 übersetzt, 0 offen, 0 verwaist
es.json      1271 übersetzt, 0 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1271 übersetzt, 0 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1271 übersetzt, 0 offen, 0 verwaist
```

`0 offen, 0 verwaist` on all four. 1158 → 1271 is +113, matching the SUMMARY exactly — and that rise is itself the proof D-32 was honoured: had the report's sentences been built with `fmt.Sprintf`, the count would not have moved and the gate would have reported green over a German-only report screen. The sentences are `{{t}}`/`{{tf}}` literals in `csv_reason.html`, and `TestCSVEveryReasonHasASentence` keeps every reason code tied to one.

### QUAL-02 — the browser half

The claim is 50 screenshots, four browser-found defects, a re-drive from a fresh database after each fix, and one guarantee reported as not driven. Checked, not taken:

- **The 50 screenshots exist and are real.** `scratchpad/shots/` holds exactly 50 PNGs with the names the SUMMARY gives. I opened two. `04c-mapping-last-row.png` shows the mapping screen with the full admin layout, one `<h1>`, "Beispiel: **Zeile 13 von 13**" (defect 1 fixed), "vorherige Zeile" present and "nächste Zeile" absent on the last row (the clamp), and the *Vorgaben* card. `06b-report-grouped.png` shows the report with counters `4 angelegt / 3 aktualisiert / 4 übergangen`, groups by reason with the row numbers beside them, "**Eine Zeile**: 4" (defect 2 fixed), and one `<h1>` (defect 3 fixed).
- **The four defects are real defects with real regression tests.** `TestCSVSampleLineCountsBothNumbersTheSameWay`, `TestCSVOneRowIsSingularOnBothScreens`, `TestCSVScreensCarryOneHeadingEach` all exist and pass, and each carries a comment naming the broken output it was written against. The fourth (the shouted hint) is a CSS rule, scoped to `.csv-columns th .form-hint` and not to `.form-hint` or `.table th`.
- **The pass really did run last.** `git log` over `internal/admin/csvimport.go`, the five CSV templates, `website_list.html`, `admin.css` and `internal/i18n/locales/` shows nothing after `d4e56f7`. HEAD is a docs commit. So no user-visible change landed after the signature — the failure Phase 7 and Phase 8 both had.
- **The counting table's one divergence is honest** — 66 admin templates, not 65, because the phase added five files and the fifth is a partial that must **not** be in `layoutPageNames`. I counted `layoutPageNames` myself: **48 entries, exactly four `csv_`**, and `csv_reason.html` correctly absent. D-31's gate — the one only a browser can see — holds.

### Judging the one "not driven"

**The call was honest.** I did not take the reasoning on trust; I checked all three legs against the tree:

1. **The pre-check is live and per row.** `csvRun` calls `h.pages.GetPageBySlug` *inside* the loop (`csvimport.go:849-855`), deliberately not from a map built before it, so row 40 sees the page row 4 just created. A duplicate address inside one file therefore becomes an update or a skip, never a rename.
2. **The trash frees the address.** `page.Store.TrashPage` (`store.go:801-810`) sets `slug = 'trash-' || id || '-' || slug` in the same `UPDATE`. So `GetPageBySlug`'s `LivePredicate` (`deleted_at IS NULL`) cannot miss a slug that is still held — a trashed page no longer holds it.
3. **Locale does not open a route either.** `UNIQUE(website_id, locale, slug)` is narrower than `GetPageBySlug`'s `(website_id, slug)` lookup, so the pre-check is the *more* inclusive of the two. It cannot return nil where the `INSERT` would collide.

That leaves a genuine read-to-`INSERT` race, which one write connection makes vanishingly rare — and the executor drove even that, with two browser contexts under `Promise.all`, and got `1 angelegt / 1 übergangen`. **Reachable and skipped would have been the wrong call; not reachable is the right one**, and reporting it as not driven with the evidence is better than reporting it as passed. It is recorded above as `fixture-only` reliance rather than as a failure, because the clause it belongs to is true — it is the reachability that no test and no browser can show.

---

## Edge resolutions spot-checked against the tree

Twelve of the 27 `resolved`/`explicit` resolutions, chosen across nine of the ten requirements:

| requirement / category | resolution | found in the tree | |
|---|---|---|---|
| IMP-01 / adjacency | a column is addressed by INDEX; two `Titel` columns both appear | `TestTwoEqualHeadingsDoNotCollide`, `TestSecondHeadingSaysWhyItStaysEmpty` | ✓ |
| IMP-01 / empty | three degenerate files, one message each | `TestCSVDegenerateFilesAreRefused` — asserts the three messages are *distinct*; `TestCSVEmptyHeadingIsColumnN` | ✓ |
| IMP-01 / concurrency | a token belongs to the user who created it | `TestForeignTokenIsRefused`, `TestCSVForeignTokenIsNotFound` (404, and the body carries none of the other admin's data); browser `10-foreign-token.png` | ✓ |
| IMP-02 / idempotency | second run renames on create, rewrites on update, and the report says which | `TestFileReadTwice`, `TestCSVSecondImportWithUpdateIsIdempotent`, `TestCSVSecondImportWithSkipLeavesThePageAlone` | ✓ |
| IMP-03 / boundary | header is row 1, first data row is row 2, one helper mints it | `csv.RowNumber` (`csv.go:128`), `TestRowNumberIsTheSpreadsheetsOwn`; cross-checked across three screens in the browser | ✓ |
| IMP-04 / concurrency | website and field defs re-read at dry run **and** at write | `csvPrepare` on both POSTs; `TestCSVDeletedTargetWebsiteEndsTheWizard`, `TestCSVDeletedFieldDefinitionIsReported` | ✓ |
| IMP-05 / concurrency | dry run and write read the same staged bytes | both `csvRun` callers read `upload.Data`; `05b-second-tab-empty.png` | ✓ |
| IMP-06 / encoding | the NFD edge — a decomposed heading matches its own field | `settleHeaderMarks` + the two fold tests, and the D-28 correction (compose, don't strip) is what is actually built | ✓ |
| IMP-07 / concurrency | the example is a snapshot; a later field arrives unmapped | `TestCSVExampleIsASnapshotAndALaterFieldArrivesUnmapped`, and the sentence saying so on `csv_mapping.html:55` | ✓ |
| IMP-08 / adjacency | stepping clamped at both ends; out-of-range `?row=` clamped, not an error | `TestCSVSampleRowSteppingIsClamped` — four cases including `row=99` and `row=-4` | ✓ |
| IMP-09 / boundary + ordering + precision | every cap at the boundary and one either side; a short row is empty in position; the cell cap is bytes | the five `internal/csv` boundary tests above | ✓ |
| IMP-10 / concurrency | `BeginTx` in the importer = 0, and no call the row function makes holds a transaction across two rows | 0 and 14 measured — **but the gate's wording lets `EnsureNames` through; see criterion 5** | ⚠ |

Eleven clean, one that passes as written and fails what it was written to protect.

---

## Requirements coverage

| req | status | evidence |
|---|---|---|
| IMP-01 | SATISFIED | criterion 2 — targets incl. "nichts", per-index columns |
| IMP-02 | SATISFIED | criterion 4 — one creation path, no second implementation of slug/sanitise/validate |
| IMP-03 | SATISFIED | criterion 4 — mixed file, named reasons, row numbers; `Summarize` groups by reason |
| IMP-04 | SATISFIED | criterion 1 — both paths; `CollisionUpdate` / `CollisionSkip`, no third behaviour |
| IMP-05 | SATISFIED with a note | criterion 3 — writes nothing, verified; the per-row prediction diverges for duplicate addresses |
| IMP-06 | SATISFIED | criterion 2 — fold tests including the WR-01 widening |
| IMP-07 | SATISFIED | criterion 2 — reachable before upload (D-37), BOM, defused, per-website and fixed-column paths |
| IMP-08 | SATISFIED | criterion 2 — stepper, clamps, defaults; mapping survives the step |
| IMP-09 | SATISFIED | criterion 5 — every cap at the boundary and one either side |
| IMP-10 | **PARTIAL** | criterion 5 — importer opens none; `EnsureNames` spans the file |

No orphaned requirements: `REQUIREMENTS.md` maps IMP-01…IMP-10 to Phase 9 and all ten are claimed by the plans.

## Decision coverage

**38/38.** Every `D-01`…`D-38` appears either in the shipped code (29 of them, mostly as the doc comment that names the decision) or in a plan/summary for the nine that are process decisions (`D-03`, `D-04`, `D-05`, `D-06`, `D-07`, `D-08`, `D-24`, `D-31`, `D-35`). No decision vanished during execution. Non-blocking gate; recorded for drift.

## Anti-patterns

None. `TODO`, `FIXME`, `XXX`, `TBD`, `HACK`, `PLACEHOLDER`, "not yet implemented" and "coming soon" all return zero across `internal/csv/`, `internal/csvimport/`, `internal/admin/csvimport.go`, the five CSV templates and `website_list.html`. No skipped or disabled tests in the phase's test files. No circular fixtures: every expected value is written by hand or derived from the file under test, and the one place a test could have compared the system with itself — `TestCSVProbeAndStartAgreeOnEveryVerdict` — asserts a *third* property (that the fixture really carried the refusals) so the comparison cannot be four identical successes agreeing with each other.

---

## What is claimed but not proved

Asked for explicitly, answered plainly. Five things:

1. **"No transaction ever spans more than one row"** rests on a gate that measures the row function only. One transaction — `term.EnsureNames`, once per file — stands outside it. The claim is prose; the gate is real but points elsewhere. **This is the gap.**
2. **"See per row what would be created, updated or skipped"** rests on a test whose fixture excludes the one case the phase knows diverges. The divergence is written down in `deferred-items.md`, invisible on the screen, and unasserted in the suite.
3. **"Reports every slug that was renamed on collision"** rests on two tests that each construct a state the production loop never produces — `existing = nil` in one, a fabricated `Verdict` in the other. The clause is true; nothing proves it is *reachable*, and the executor said as much.
4. **"Each field can carry a default"** is proved for status and for custom fields, and the CR-01 test additionally asserts a **body** default that has no control on the mapping screen. One third of the phase's most important regression test exercises a door the operator cannot open.
5. **"Indistinguishable from hand-made ones"** is proved for the three properties the criterion names. It is not true of `locale`, which the importer leaves at `""` while the page form sets `locale.Pick(...)`. Harmless on a monolingual website, not on a multilingual one, and outside the criterion's own list — recorded, not counted against it.

## Gaps summary

The phase is built, and built unusually well: one creation path with no second implementation anywhere on it, boundary tests on both sides of every cap, a compensation that exists on exactly one of two arms with a test for the arm where it must be absent, and a browser pass that found four real defects a green suite had passed over and left 50 screenshots that survive scrutiny. CR-01's fix is the strongest thing in the phase — `TestBlankCellSaysNothingOnTheUpdateArm` asserts all three slots preserved (status, markdown, field), all three defaults still applying, **and** the create arm's `draft`, against a real database, and the mechanism behind it (`stated := cellFor(...) != ""`) is one rule where there were three.

Two things stop this being a clean pass, and neither is a coding mistake:

- **A file-wide transaction on the one-connection write pool**, reached through `term.EnsureNames` before the row loop. Deliberate, precedented, commented — and outside every stamp and every gate this phase wrote for the requirement it amended most carefully. Stamp it or batch it; do not leave it unstated, because the project's own rule for exactly this situation is written into D-02.
- **A dry run that can be wrong about a row and does not say so.** Found by the executor, explained correctly, recorded — and then left where only a reader of `deferred-items.md` will find it, on the screen whose whole purpose is to be trusted before committing.

---

_Verified: 2026-09-06T19:48:02Z_
_Verifier: Claude (gsd-verifier) — goal-backward from ROADMAP.md Phase 9, stamps binding over original wording_

---

## Nachtrag, 2026-09-08: beide Lücken sind zu

> Nachgetragen und nicht eingearbeitet, aus demselben Grund wie in Phase 6:
> ein Bericht, der sein eigenes Urteil still hochstuft, ist weniger wert als
> einer, der zeigt, was sich geändert hat. Das Urteil oben stand richtig da,
> als es geschrieben wurde.

**Lücke 1 — die dateiweite Transaktion — ist gebündelt, nicht gestempelt.**

`internal/admin/csvimport.go:917` ruft nicht mehr `EnsureNames` für die ganze
Datei, sondern `csvEnsureTerms`, das in Bündeln von `csvTermChunk = 500`
arbeitet. Der Kommentar über der Konstanten trägt die Messung, die den Wert
begründet, statt einer runden Zahl:

> Eine 10-MB-Datei ergab 796 000 Namen in einer einzigen Transaktion: 12,2 s
> Import, während ein konkurrierender Schreibzugriff 8,1 s wartete.

Nach der Bündelung: 3,54 s Import, 3,64 ms längste Wartezeit — dieselbe Datei.
Das Tor dazu ist `TestCSVTermPrePassIsChunked`, und es misst genau das, was der
Bericht oben als fehlend benennt: die **Grösse** der Bündel, über eine
eingereichte Zählfunktion, nicht was ein Bündel tut.

Der Bericht schrieb: „Stempeln oder bündeln; lass es nicht unausgesprochen."
Es wurde gebündelt.

**Lücke 2 — der Probelauf, der sich irren kann — ist geschlossen.**

`internal/admin/csvimport.go:956` führt eine `planned`-Karte: die Adressen, die
Zeilen oberhalb der aktuellen belegen werden. Der Kommentar nennt die Messung,
die den Fehler zeigte, und behält sie:

> Gemessen als 5 anlegen / 2 aktualisieren gegen 4 angelegt / 3 aktualisiert,
> bevor es das gab.

Der Schreibarm braucht die Karte nicht — sein `GetPageBySlugIn` ist live und
pro Zeile, also findet Zeile 40 die Seite, die Zeile 4 gerade angelegt hat. Der
Probelauf schreibt nichts und kann dieselbe Frage nur beantworten, indem er
sich merkt, was er selbst schon zu erzeugen beschlossen hat.

Das Tor ist `TestCSVProbeAndStartAgreeOnEveryVerdict`, und seine Vorrichtung
trägt jetzt die Zeile, die der Bericht als fehlend benennt — dieselbe Adresse
zweimal in einer Datei, gefahren mit beiden Kollisionsregeln.

Beide laufen grün gegen HEAD:

```
--- PASS: TestCSVProbeAndStartAgreeOnEveryVerdict (1.08s)
    --- PASS: .../aktualisieren
    --- PASS: .../uebergehen
--- PASS: TestCSVTermPrePassIsChunked (0.00s)
```

**Was der Bericht nicht gesehen hat und was seither dazugekommen ist.** Am
2026-09-07/08 sind an diesem Pfad vier weitere Befunde behoben worden, jeder mit
rotem Beweis und Mutationsprobe: eine leere Schlagwortzelle löschte die
Schlagwörter einer aktualisierten Seite; die Adresssuche ignorierte die Sprache
und schrieb in eine Übersetzung; eine Vorgabe für ein Feld ohne Spalte wurde
angenommen und nie geschrieben, obwohl der Bildschirm das Gegenteil verspricht;
und der Importer prüfte gegen Felder, die `gilt_fuer` von den Seiten
ausschliesst, die er anlegt — was auf einer Website mit einem
Pflichtfeld für Beiträge **jede** Zeile jedes Imports abwies. Siehe
`.planning/audits/v1.6-ADVERSARIAL-AUDIT.md`, Befunde 4, 5, 19 und 20.
