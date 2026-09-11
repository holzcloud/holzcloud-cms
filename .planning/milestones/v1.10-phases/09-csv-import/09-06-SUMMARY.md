---
phase: 09-csv-import
plan: 06
subsystem: i18n, admin-ui
tags: [i18n, csv-import, browser-pass, uat, playwright, accessibility, css, plural]
status: complete

requires:
  - "09-05: the four screens and the report, finished"
  - "09-REVIEW-FIX: all nine review findings fixed and committed (65897e9 … 3e54f87)"
provides:
  - "every German string phase 9 added, translated into en, es, fr, it and de-CH"
  - "the whole visible surface of phase 9 driven once through a running server"
  - "four defects the suite passed over, found in a browser and fixed"
affects:
  - "internal/i18n/locales/ — five catalogues"
  - "the four CSV screens and the shared reason partial"
  - "cmd/holzcloud/assets/admin.css — one scoped rule"

tech-stack:
  added: []
  patterns:
    - "the singular is a string of its own, not a rule in code — the shape six other admin screens already have"
    - "two numbers in one sentence are counted the same way, or one of them is wrong"
    - "the layout renders the page heading; a template does not render a second one"

key-files:
  created:
    - .planning/phases/09-csv-import/deferred-items.md
  modified:
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - internal/i18n/locales/de-CH.json
    - internal/admin/csvimport.go
    - internal/admin/csvimport_test.go
    - cmd/holzcloud/templates/admin/csv_mapping.html
    - cmd/holzcloud/templates/admin/csv_dryrun.html
    - cmd/holzcloud/templates/admin/csv_report.html
    - cmd/holzcloud/templates/admin/csv_expired.html
    - cmd/holzcloud/templates/admin/csv_reason.html
    - cmd/holzcloud/assets/admin.css

decisions:
  - "The status placeholder stays `draft` in all four foreign catalogues, because statusVocabulary is closed and `borrador` / `brouillon` / `bozza` are not in it — a translated placeholder would be a rejected row."
  - "The singular rule is applied where the project applies it: one count beside its noun. The file-header sentences carry two counts at once and are left alone, because a singular for those is a plural mechanism this project has deliberately not built."
  - "`ReasonRenamed` is reported as NOT DRIVEN rather than claimed: three separate properties of the design close every route to it from a browser."

metrics:
  duration: "~2h50m"
  completed: 2026-09-06

actuals:
  tokens: 59000
  tasks: 2
  commits: 4
---

# Phase 9 Plan 6: The standing gate, and the half no test can hold — Summary

113 German strings translated into four languages and de-CH rebuilt by rule, and
the whole visible surface of phase 9 driven through a running browser — which
found four defects the entire suite had passed over.

---

## The order this ran in, and why it matters

The precondition was checked first and was met: `git log` showed the nine
review-fix commits `65897e9 … 3e54f87`, and `git status --porcelain` was empty.
The pass ran **after** the fix round, as the plan required.

It then had to be run **three times**, because each browser pass found something
that changed what a person sees, and a signature on a screen that has since
changed is worth nothing. The final numbers below are all from the third pass,
against the tree as it stands, on a database created from scratch by that same
binary.

---

## Task 1 — the catalogues

`go run ./tools/i18n` before: **1271 strings in the source**, 113 open on each of
en, es, fr, it. That is the count the plan asked to see move — it rose from the
1158 baseline by exactly the number of `{{t}}`/`{{th}}`/`{{tf}}` literals the
four new templates and the panel added, which is the proof D-32 was honoured:
the sentences live where `tools/i18n` can see them and not in `fmt.Sprintf`.

All 113 translated by hand, then `-schweiz`, then the gate:

```
1271 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1271 übersetzt, 0 offen, 0 verwaist
es.json      1271 übersetzt, 0 offen, 0 verwaist
fr-CH.json    4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1271 übersetzt, 0 offen, 0 verwaist
it-CH.json    9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1271 übersetzt, 0 offen, 0 verwaist
```

`de-CH` went 55 → 73: eighteen new deviations, every one by rule (`„…“` → `«…»`,
`heißt` → `heisst`). `fr-CH.json` and `it-CH.json` came out byte-identical —
`git diff --exit-code` on the two returned 0.

The final count is 1271 and not 1272 because two later fixes cancelled: one added
`Eine Zeile`, the other retired `Dieser Upload ist abgelaufen` when the duplicate
heading went.

**One translation decision worth stating.** The default-status placeholder
`entwurf` is a value the operator *types into a status cell*, and
`statusVocabulary` (`row.go:53`) is closed. `draft` is in it; `borrador`,
`brouillon` and `bozza` are not. So the placeholder reads `draft` in all four —
translating it would have printed an example that the importer then rejects.

Commit `7025582`, touching nothing outside `internal/i18n/locales/`.

---

## The counting table, measured against the finished tree

| What | Predicted | Measured | |
|---|---|---|---|
| `layoutPageNames` entries / of them `csv_` | 48 / 4 | **48 / 4** | ✓ |
| `adminOnly` rows | 19 | **19** | ✓ |
| `adminProtectedMux.Handle` | 150 | **150** | ✓ |
| admin templates | 65 | **66** | **✗ — see below** |
| `<details` in `website_list.html` | 3 | **3** | ✓ |
| `jobs.Job{` | 12 | **12** | ✓ |
| migrations | 49 | **49** | ✓ |
| packages under `internal/` | 40 | **40** | ✓ |
| admin CSS files | 2 | **2** | ✓ |
| `BeginTx` in the importer | 0 | **0** | ✓ |
| `BeginTx` under `internal/`, non-test | 14 | **14** | ✓ |
| `BeginTx` tree-wide, non-test | — | **15** | (14 + `cmd/holzcloud/cli.go`) |

**The one divergence is the plan's arithmetic, not the tree's.** The phase added
**five** template files, not four: `csv_mapping`, `csv_dryrun`, `csv_report` and
`csv_expired` are screens, and `csv_reason.html` is the shared partial that holds
the reason and outcome blocks. 61 + 5 = 66. `layoutPageNames` is correctly 48
with exactly four `csv_` names, because a partial must **not** be in it — so the
two numbers disagreeing is the two things being different, which is right.

The `BeginTx` tree-wide 15 is the correction 09-REVIEW-FIX already recorded; the
plan's gate is scoped to `internal/`, and that measures 14.

---

## The browser pass

Real Chrome, driven with the Playwright already present in the npx cache
(required by path — no download, no `npx --yes`), against the real binary built
from this tree, on a free port with `HOLZCLOUD_DATA_DIR` pointed at the session
scratch path. Two admin accounts created with `holzcloud user create` (password
on stdin). Two-factor is compulsory for admins and was **enrolled and verified
with a self-computed TOTP** — HMAC-SHA1, six digits, thirty seconds — on both
accounts. No bypass, no code changed to get in.

50 screenshots in `…/scratchpad/shots/`. Every step below was driven; where a
step could not be reached it says so and says why.

### 1 — The third `<details>` panel · `01-panel-first-ever-import.png`, `01b-panel-with-a-website.png`

Three panels in the content area, in this order: *Website aus einer Sicherung
einspielen*, *Von WordPress umziehen*, *Seiten aus einer Tabelle einlesen
(.csv)*. The CSV panel is the third and reads as their sibling.

Both choices are on it and both are on-screen: `target=neu` (checked) /
`target=bestehend`, and `collision=uebergehen` (checked) / `collision=aktualisieren`.

With **no website yet**, the panel carries two forms — `POST
/admin/websites/import-csv` and `GET /admin/csv-vorlage` — and the example form
has *no* `<select>`, only the button *Beispieldatei für eine neue Website*. That
is exactly WR-08's new path, seen at the moment it is about: the very first import.

Once a website exists, the panel carries **three** forms and two `website`
selects: the example form for this website appears with its own `<select>`, and
WR-08's fieldless second form stays beside it.

*(Note for the count gate: the rendered page has four `<details>`, because
`base.html` contributes the topbar user switcher. `grep -c '<details'` on
`website_list.html` is 3, and both numbers are right.)*

### 2 — The example CSV, before anything was uploaded · `02-example-before-upload.png`

Downloaded from the panel with **nothing uploaded and no website in existence**:

```
website-vorlage.csv          BOM: yes
Titel,Adresse,Text,Zustand,Schlagwörter
Beispielseite,beispielseite,Ein Satz über die Seite.,entwurf,Beispiel|Muster
```

The five fixed columns, and the pipe visible in the multi-value sample cell.
The filename is `website-vorlage.csv` and **not** `untitled-vorlage.csv` — the
half of WR-08 the review had got wrong, seen working.

And for a website that does exist, from the panel's own select:

```
werkstatt-vorlage.csv        BOM: yes
Titel,Adresse,Text,Zustand,Schlagwörter,Grösse,Im Angebot
Beispielseite,beispielseite,Ein Satz über die Seite.,entwurf,Beispiel|Muster,Beispiel,ja
```

The website's own two fields are in the header, spelled composed.

### 3 — Screen 1 refusing each hostile file · five screenshots `03-hostile-*.png`

Five files, five different sentences, no website created by any of them, and each
time back on the list with the message in the flash area:

| file | what the screen said |
|---|---|
| zero bytes | „Die Datei ist leer. Es wurde nichts abgelegt." |
| a BOM and nothing else | „Die Datei enthält nur eine Byte-Reihenfolge-Marke und sonst nichts – so sieht ein leeres Tabellenblatt auf der Festplatte aus." |
| no header row | „Die Datei hat keine Kopfzeile. Die erste Zeile muss die Spaltenüberschriften enthalten." |
| 101 columns | „Die Datei hat mehr als 100 Spalten. Es wurde nichts abgelegt." |
| a NUL byte in a cell | „Die Datei enthält ein Nullbyte und ist keine CSV-Datei." |

Website count after each: **0**.

### 4 — The mapping screen · `04a`, `04b`, `04c`, `04d`

**The layout is there** (D-31, the half no test can see): sidebar, topbar,
navigation, `#flash-area`, and the CSRF `hx-headers` attribute on `<body>`.
Confirmed by eye on the screenshot and by DOM probe on all four new screens.

*Automatic matches.* All five columns of `gut.csv` matched in the file's order —
Titel→title, Adresse→slug, Text→body, Zustand→status, Schlagwörter→terms.
On the existing website with fields, seven of eight matched, including
**`Grösse` written decomposed** (`G r o U+0308 s s e` in the file, verified in
the hex) matching `field:groesse` — D-28, seen rather than asserted.

*The second `Titel` column.* Unmapped, `– nichts –`, **with a sentence saying
why**: „Dieselbe Überschrift steht schon über Spalte 1. Diese hier bleibt ohne
Ziel, bis du eines wählst."

*The required group field.* „Das Pflichtfeld „Bilder" ist eine Gruppe. …
Solange das Feld Pflicht ist, wird jede einzelne Zeile dieser Datei abgewiesen."
Driven to prove the sentence tells the truth: with the group still required, the
dry run reported **0 anlegen / 0 aktualisieren / 11 übergehen**.

*Per-field defaults.* `default_status`, `default_field:groesse`,
`default_field:im_angebot` — one box per field, plus Zustand.

*The sample row and the stepper — WR-07, checked hardest.* Two column targets
overridden to `– nichts –` and a default typed into the box, then *nächste
Zeile* clicked. Before and after the step:

```
before  ["target_0=title","target_1=slug","target_2=body","target_3=none","target_4=none"]
        default_status="veroeffentlicht"
after   ["target_0=title","target_1=slug","target_2=body","target_3=none","target_4=none"]
        default_status="veroeffentlicht"
```

Identical. The operator's work survives the step. The control is a submit button
with `formmethod="GET"`, and the URL it produces carries the whole mapping.
*vorherige Zeile* is **absent** on the first row and *nächste Zeile* absent on
the last — absent, not disabled.

### 5 — The dry run · `05a`, `05b`, `05c`

„Es wurde nichts geschrieben. Was hier steht, würde beim Einlesen geschehen –
noch ist keine einzige Seite entstanden und keine verändert worden."

**IMP-05 seen and not asserted:** a second tab opened on `/admin/websites`
during the dry run showed **0 websites**. Nothing had been written — not the
pages, not even the website.

Rows grouped by reason, with the spreadsheet's row numbers beside each group:

```
übergehen  „vielleicht" ist weder ein Entwurf noch veröffentlicht.       Eine Zeile: 4
übergehen  „Im Angebot": „womoeglich" ist kein Ja und kein Nein.         Eine Zeile: 5
übergehen  Diese Zeile hat keinen Titel. …                               Eine Zeile: 8
übergehen  Diese Zeile hat 9 Zellen, die Kopfzeile hat 8 Spalten. …      Eine Zeile: 12
```

### 6 — The write, the report, and the row numbers meeting · `06a`, `06b`

The report renders with the layout, and the counters read
**4 angelegt / 3 aktualisiert / 4 übergangen**, grouped by reason with the row
numbers inside each group, and a link straight to the page list.

**The cross-check the plan asked for.** On the mapping screen the row with the
nonsense status reads „Beispiel: **Zeile 4** von 12". In the dry run it is
„Eine **Zeile: 4**". In the report it is „Eine **Zeile: 4**". Same file, same
row, three screens, one number — D-26 confirmed by eye and not by one screen
alone.

The twelve pages of the clean file are really there, with the statuses the file
gave them (Preise, Team, Datenschutz, Stellen as *Entwurf*, the rest
*Veröffentlicht*), and the Markdown really rendered — read back from the
database: `# Willkommen` → `<h1>`, `**Startseite**` → `<strong>`, and a GFM
table → `<table>`.

**Both import paths were driven**, as the roadmap's planning note requires: a
new website (`gut.csv` → „Werkstatt", 12 created) and the existing one
(`boese2.csv` → „Werkstatt"), the latter on both collision answers —
*aktualisieren* (4/3/4) and *übergehen*.

**One thing not driven, and the reason.** The **renamed-address group (D-23)**
could not be produced in a browser. Three properties of the design each close a
route to it: the create-vs-update pre-check is live and per row, so a duplicate
address becomes an update or a skip; the trash frees the address (a deleted
page's slug becomes `trash-<id>-<slug>`, verified in the database); and two
concurrent commits serialise on a one-connection write pool — I drove that race
with two browser contexts committing the same address under `Promise.all` and
got `1 angelegt` / `1 übergangen`, not a rename. `ReasonRenamed` is the safety
net for a read-to-INSERT race this architecture makes vanishingly rare. Covered
by the suite; recorded in `deferred-items.md` as **not driven**, with the
evidence, rather than reported as passed.

### 7 — The expiry screen · `07a`, `07b`

Driven twice: by **reloading the report** (the token has been spent) and with a
**token that never existed**. Both give the same screen, both render with the
layout, and reloading did **not** import a second time — the website list was
identical before and after.

It reads as a fact and not as an accusation (D-33): „Eine hochgeladene Tabelle
wird nach einem Tag wieder weggeräumt. Diese hier ist nicht mehr da – geschrieben
wurde nichts, und die Website ist unverändert." — and then a way forward:
„Lade die Datei einfach noch einmal hoch." with a *Noch einmal beginnen* button.

### 8 — With JavaScript switched off · `08a` … `08e`

Genuinely off, and proved rather than assumed: with `javaScriptEnabled: false`
the `<script src=…htmx…>` tag is still in the markup while `window.htmx` is
`undefined` — the page's own script did not execute.

Everything worked:

- signing in, **including the two-factor code** — plain forms all the way
- the `<details>` panel opens (native HTML, no script)
- the file input, both radio groups, the website `<select>`
- **the panel submit** → the mapping screen, with its layout
- **the mapping screen's stepper** → row 3, and the mapping *and* the typed
  per-field default came with it (`"ohne-skript"` still in the box)
- **the dry-run submit** → the dry run, with its layout
- the write, the report, the expiry screen, and the example download
  (a GET form carrying no field at all)

Not one control needed a script.

### Additional — a foreign token · `10-foreign-token.png`

Admin 1 staged `gut.csv` as „Geheime Werkstatt". Admin 2, a second real account
with its own second factor, opened admin 1's URL: **HTTP 404**, the page reads
`404 page not found`, and the body contains none of `gut.csv`, `Geheime
Werkstatt`, `Willkommen`, `Öffnungszeiten` or `Schlagwörter`.

### Additional — all four screens in all five languages · 25 screenshots `09-*`

Every new screen driven end to end in de, en, es, fr and it. All twenty render
with the layout. A sample of what was read:

| | en | fr | it |
|---|---|---|---|
| panel | Import pages from a spreadsheet (.csv) | Importer des pages depuis un tableur (.csv) | Importare pagine da un foglio di calcolo (.csv) |
| mapping | Map the columns | Affecter les colonnes | Assegnare le colonne |
| dry run | Dry run | Essai à blanc | Prova a vuoto |
| report | Import finished | Import terminé | Importazione conclusa |
| expired | The upload has expired | Le téléversement a expiré | Il caricamento è scaduto |
| a reason | „vielleicht" is neither a draft nor published. One row: 4 | « vielleicht » n'est ni un brouillon ni publiée. Une ligne: 4 | «vielleicht» non è né una bozza né pubblicata. Una riga: 4 |

### Console, CSP and the server log

- **Browser console: 0 errors** across the whole final pass, in every language,
  with and without scripting.
- **CSP violations: 0.** The header is present on every admin document:
  `default-src 'self'; script-src 'self'; …; frame-ancestors 'none'; base-uri
  'none'; object-src 'none'`.
- **Server log: 0 `ERROR` lines, 0 CSP entries.** Two `WARN` lines in the whole
  run, both mine: the 404 from the foreign-token check and a `/favicon.ico` 404.

One console error, `Transition was skipped`, appeared during exploratory runs and
is **not** phase 9's: it is Chrome's own unhandled rejection from
`@view-transition { navigation: auto; }` at `admin.css:1807`, and it reproduces
identically by clicking from `/admin/templates` to `/admin/users` — screens this
phase never touched. Written up in `deferred-items.md`.

---

## Deviations from Plan — four defects the browser found and the suite did not

Every one of these passed `go test ./...` before and after it was written. This
is the whole reason the gate exists.

### 1. [Rule 1 — Bug] „Beispiel: Zeile 13 von 12"

- **Found during:** step 4, stepping to the last row of a twelve-row file.
- **Issue:** `SampleNumber` is minted by `csv.RowNumber` and counts the header
  (D-26: the first data row is row 2). The second number was `TotalRows`, which
  counts data rows and never the header. In one sentence the two met, and the
  last row of a twelve-row file read **„Zeile 13 von 12"** — a sentence that is
  simply false about the file in front of the operator.
- **Fix:** `LastRowNumber`, minted by the same helper. The sentence now reads
  „Zeile 2 von 13" … „Zeile 13 von 13", both numbers the spreadsheet's.
- **Test:** `TestCSVSampleLineCountsBothNumbersTheSameWay` — fails before with
  the literal broken sentence, passes after.
- **Files:** `internal/admin/csvimport.go`, `csv_mapping.html`, `csvimport_test.go`
- **Commit:** `7fbc0e3`

### 2. [Rule 1 — Bug] „1 Zeilen: 4" on every group of one row, in five languages

- **Found during:** step 5, reading the grouped dry run.
- **Issue:** the reason groups printed `{{tf "%d Zeilen" .Total}}`
  unconditionally. A group of one row — the commonest kind — read „1 Zeilen: 4",
  and „1 rows", „1 filas", „1 lignes", „1 righe". This administration already
  handles exactly this, six times over (`blocktype_list.html:41` and `:44`,
  `dashboard.html:31`, `media_list.html:43`, `snippet_list.html:94`,
  `website_list.html:152`), with `{{if eq .X 1}}{{t "singular"}}{{else}}…`.
  This phase was the seventh site and did it differently.
- **Fix:** the project's own pattern. And the cell **stood twice, wortgleich**,
  in `csv_dryrun.html` and `csv_report.html` — exactly the two copies
  `csv_reason.html` says in its own header it exists to prevent — so it is now
  the partial `csv-rows` and there is one of it.
- **Test:** `TestCSVOneRowIsSingularOnBothScreens` — fails before on both
  screens with `"1 Zeilen: 2"`, passes after.
- **Files:** `csv_reason.html`, `csv_dryrun.html`, `csv_report.html`, four catalogues
- **Commit:** `66c27dc`

### 3. [Rule 1 — Bug] Two `<h1>` on all four new screens

- **Found during:** looking at a screenshot. Nothing in the markup reads wrong.
- **Issue:** `base.html:198` renders `<h1>{{.Title}}</h1>` in the content header
  for every page. All four new templates rendered a **second** one directly below
  it saying the same thing, so the operator read „Einlesen abgeschlossen" twice,
  stacked, and the document carried two first-level headings. Measured: every
  screen this phase did not touch has exactly one `<h1>`; all four of these had
  two. Fifty-four of the sixty-six admin templates rely on the header alone.
- **Fix:** the four redundant `<h1>`s removed. `Dieser Upload ist abgelaufen`
  became an orphan key and was removed from all four catalogues by hand, since
  `tools/i18n` reports orphans but never deletes them.
- **Test:** `TestCSVScreensCarryOneHeadingEach` — fails before with `2, 2, 2, 2`.
- **Commit:** `d4e56f7`

### 4. [Rule 1 — Bug] The column hint came out shouted

- **Found during:** looking at the same screenshot.
- **Issue:** the „Dieselbe Überschrift steht schon über Spalte 1…" sentence sits
  inside a `<th scope="row">`, and `.table th` (`admin.css:972`) uppercases every
  header cell and spreads its letters. Right for the word „Titel"; wrong for the
  sentence beneath it, which arrived as **„TITEL DIESELBE ÜBERSCHRIFT STEHT SCHON
  ÜBER SPALTE 1. DIESE HIER BLEIBT OHNE ZIEL, BIS DU EINES WÄHLST."** — glued to
  the column name, on one run-on line, in all five languages. IMP-06 exists so an
  unexplained blank does not look like an oversight; an unreadable explanation is
  barely better.
- **Fix:** one rule, `.csv-columns th .form-hint`, taking the casing and the
  letter-spacing back and putting the sentence on its own line. Scoped there and
  **not** to `.form-hint` or `.table th`, which the whole administration uses.
- **Commit:** `d4e56f7`

---

## Where the plan was wrong

1. **"admin templates: 61 + 4 = 65."** The phase added five files. Four are
   screens and belong in `layoutPageNames`; the fifth, `csv_reason.html`, is the
   shared partial and must not be. The tree measures 66, and 48 / 4 for
   `layoutPageNames` is correct alongside it.

2. **The plan's step 6 assumes every renamed address can be named in the
   browser.** It cannot — see above and `deferred-items.md` §3. The plan's own
   instruction ("a step that cannot be reached is a finding, not a step to
   skip") is what this follows.

3. **The plan's `boese.csv` calls for a header written with a decomposed
   `Grösse`.** On a *new* website there are no fields for it to match, so D-28
   would not have been exercised. Driven instead against a website that has a
   `Grösse` field, and additionally with `Schlagwörter` written decomposed on the
   new-website path, where it matches one of the five fixed targets.

---

## Verification

```
go build ./...            clean
go vet ./...              clean
gofmt -l .                (no output)
go test ./...             exit 0
go run ./tools/i18n       0 offen, 0 verwaist × 4
git diff --exit-code -- internal/i18n/locales/fr-CH.json internal/i18n/locales/it-CH.json   exit 0
git status --porcelain -- internal/i18n/locales/                                            (no output)
```

Counting table: eleven of twelve rows as predicted; the twelfth explained above.

Browser: eleven numbered steps driven, 50 screenshots, both import paths, both
collision answers, scripting off, a foreign token, five languages. Console 0,
CSP 0, server log 0 `ERROR` and 0 CSP.

---

## Commits

| | |
|---|---|
| `7025582` | `i18n(09-06)` the hundred and thirteen sentences of the import, in four languages |
| `7fbc0e3` | `fix(09-06)` the sample line counts both its numbers the same way |
| `66c27dc` | `fix(09-06)` a group of one row says „Eine Zeile", and the cell stands once |
| `d4e56f7` | `fix(09-06)` one heading per screen, and the column hint is a sentence again |

Every commit named its files and was checked with `git show --stat`. No
`git add -A`, no `git commit -a`. Nothing outside phase 9 was staged — the
developer's parallel work on `README.md`, `docs/`, `.github/`, `CONTRIBUTING.md`
and `SECURITY.md` was not touched.

## Known Stubs

None.

## Self-Check: PASSED
