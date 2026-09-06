---
phase: 09-csv-import
reviewed: 2026-09-06T18:40:00Z
depth: standard
files_reviewed: 28
files_reviewed_list:
  - cmd/holzcloud/assets/admin.css
  - cmd/holzcloud/main.go
  - cmd/holzcloud/main_test.go
  - cmd/holzcloud/templates/admin/csv_dryrun.html
  - cmd/holzcloud/templates/admin/csv_expired.html
  - cmd/holzcloud/templates/admin/csv_mapping.html
  - cmd/holzcloud/templates/admin/csv_reason.html
  - cmd/holzcloud/templates/admin/csv_report.html
  - cmd/holzcloud/templates/admin/website_list.html
  - internal/admin/csvimport.go
  - internal/admin/csvimport_test.go
  - internal/admin/handler.go
  - internal/csv/csv.go
  - internal/csv/csv_test.go
  - internal/csv/example.go
  - internal/csv/example_test.go
  - internal/csvimport/groups.go
  - internal/csvimport/groups_test.go
  - internal/csvimport/mapping.go
  - internal/csvimport/mapping_test.go
  - internal/csvimport/row.go
  - internal/csvimport/row_test.go
  - internal/csvimport/store.go
  - internal/csvimport/store_test.go
  - internal/csvimport/verdict.go
  - internal/csvimport/verdict_test.go
  - internal/db/migrations/00049_csv_imports.sql
  - internal/web/render.go
findings:
  critical: 1
  warning: 8
  info: 7
  total: 16
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-09-06T18:40:00Z
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

The five areas the brief singled out are, with one exception, genuinely closed,
and I checked each of them mechanically rather than by reading the comment that
claims it.

**IMP-10 holds.** `grep -rn BeginTx internal/csv internal/csvimport
internal/admin/csvimport.go` returns nothing, the tree-wide count of files using
`BeginTx` is still 14, and nothing the row loop calls holds a transaction across
two rows: `term.EnsureNames` runs once before the loop
(`csvimport.go:697-706`, proved by `TestCSVTermsEnsuredOnceBeforeTheLoop`), and
`page.UpdatePage`'s own `BeginTx` opens and closes inside one row.

**D-02's compensation is on the create arm only, and the absence is asserted
rather than argued.** `WriteRow` (`row.go:495-527`) reaches `TrashPage` +
`PurgePage` from exactly one place — the create path's `setTerms` failure — and
`TestUpdateArmIsNotRolledBack` (`row_test.go:410-453`) fails the terms step of an
update and then looks for the page. There is no second path into the
compensation.

**The hostile-file checklist is real and tested on both sides of every
boundary.** 10 MB / +1 measured at the place the cap actually lives
(`csvimport_test.go:233`), `MaxRows` / +1, `MaxColumns` / +1, `MaxCellBytes` / +1
plus the runes-versus-bytes case, NUL and BOM-only refused on the raw bytes
before anything is staged, `LazyQuotes` and `FieldsPerRecord = -1` set before the
header is read, row numbers minted in one place. Nothing is staged when a refusal
fires — I checked the ordering in `HandleCSVImport`, and `csv.New` runs before
`Stage`.

**Authorisation is sound and the menu-handler shape does not repeat here.** All
five routes are wrapped in `requireAdmin` and all five are in
`TestRouteAuthorization`'s table in the same commit. `staged()`
(`csvimport.go:349-366`) is the single place either refusal becomes a response,
and the ownership check lives in the store rather than in each handler. The token
is 128 bits from `crypto/rand`, only its SHA-256 is stored, and `ErrForeign`
answers 404 before a byte of the file is read. On `GET /admin/csv-vorlage` the
website id is unvalidated beyond existence — but this codebase has no per-user
website scoping at all (`TestRouteAuthorization` asserts that any editor keeps
`/admin/websites/1/pages`), so an admin already reaches every website and that
route grants nothing new.

**The pinned German values survived.** `'neu'`, `'bestehend'`, `'uebergehen'` and
`'aktualisieren'` are German in Go and in SQL and each carries the sentence
saying why; the field kinds are taken from `field.Kind*`; the stored status
values are `draft`/`published`. Nothing was translated into a string SQLite would
refuse.

**Comments.** I did not find a single comment in the phase's files that says only
what the code says. Two comments are *wrong* rather than thin — WR-03 and IN-05
below — and that is a different defect.

Three things are not right, and one of them writes to a live site.

The one that matters: **a mapped `Zustand` column whose cell is empty demotes an
existing published page to a draft on the update arm**, with no line in the
report, no revision recorded and no test. `parseStatus` turns an empty cell into
`"draft"` on the argument that a file saying nothing about publication must not
publish two hundred pages; `update()` then applies that substituted value because
the *column* is mapped, not because the *cell* said anything. On the update arm
the safety argument inverts into its opposite.

Beside it: **the D-28 hole is only three-sevenths closed** — `foldHeader`
composes a combining diaeresis back onto a, o and u, and drops every other mark,
so `Café` written NFC folds to `caf` and the same word written NFD folds to
`cafe`. Two keys for one word, which is the exact failure D-28 exists to prevent;
the doc comment names the disagreement and then argues it away. And **the D-32
discipline leaks at its argument slots**: `csv.go:240` builds
`"row has %d cells, the header has %d"` with `fmt.Sprintf`, and that English
fragment is substituted into a German `{{tf}}` sentence on the report screen,
where `tools/i18n` cannot see it.

Structural findings were not supplied for this review; the sections below are
narrative only.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: a mapped-but-empty `Zustand` cell silently unpublishes an existing page

**File:** `internal/csvimport/row.go:63-71`, `internal/csvimport/row.go:304-308`,
`internal/csvimport/row.go:569-571`

**Issue:** three individually reasonable rules compose into a live-site change
nobody is told about.

1. `cellFor` returns `""` for a mapped column whose cell is blank and whose
   target carries no default (`row.go:216-223`).
2. `parseStatus("")` returns `("draft", true)` — a *substitution*, not a
   pass-through. Its comment gives the create-path reason: *"an empty cell is a
   draft, so a file that says nothing about publication does not publish two
   hundred pages on a live website."*
3. `update()` decides what to overwrite from `mapped()`, which asks whether a
   **column** is pointed at the target — not whether the **cell** said anything:

```go
mapped := func(t Target) bool {
    return m.ColumnFor(t.Kind, t.Key) >= 0 || strings.TrimSpace(m.Defaults[t.String()]) != ""
}
...
if mapped(Target{Kind: TargetStatus}) {
    u.Status = create.Status
}
```

So: an operator imports a 300-row file into an existing website with
*Aktualisieren* chosen, the file has a `Zustand` column, and forty of the rows
left that cell blank. Those forty live pages go to `draft` and leave the public
site. Nothing reports it — the verdict is a clean `OutcomeUpdate` with an empty
`Reason`, so the row does not even form a group (`groups.go:118-124`). And
`page.UpdatePage` records a revision only when title, slug, markdown or blocks
changed (`page/store.go:574-576`), so a status-only flip leaves no history entry
either. The operator's evidence that it happened is the public site.

The same shape reaches the body: a mapped but empty `Text` column sets
`u.Markdown`/`u.HTML` to `""` and wipes the page's content. That one *is*
recoverable — the content change does record a revision — and is arguably the
documented "the file says this is empty now" semantics, which is why it is named
here rather than as its own finding. Status is different because the empty cell
is not carried through as empty; it is replaced by a positive value the file
never contained.

No test covers it. `TestUnmappedColumnsAreLeftAlone` (`row_test.go:321`) proves
the *unmapped* status survives; `TestExistingPageIsUpdatedOrSkipped`
(`row_test.go:277`) maps only `Titel` and `Text`.

**Fix:** on the update arm, only write a status the file actually stated. The
cheapest correct version keeps `parseStatus`'s create-path default intact and
distinguishes "the cell was empty" from "the cell said draft":

```go
// row.go — carry the raw cell alongside the parsed status
rawStatus := cellFor(row, m, Target{Kind: TargetStatus})
...
create.StatusGiven = rawStatus != ""   // new field on PageCreate is not needed;
                                       // pass it through the Verdict/create pair
```

and in `update()`:

```go
-	if mapped(Target{Kind: TargetStatus}) {
-		u.Status = create.Status
-	}
+	// A mapped column with an empty cell says nothing about publication, and on
+	// the update arm "nothing" must not mean "draft": that would take a live
+	// page off the public site over a blank cell, and the report would not say
+	// so. The create arm keeps parseStatus's default, where drafting is the
+	// safe direction.
+	if statusGiven {
+		u.Status = create.Status
+	}
```

If the alternative reading is preferred — a blank cell genuinely means "unpublish
this" — then it must at minimum become a reported verdict: a new
`ReasonUnpublished` with the address as its argument, an arm in
`csv_reason.html`, and a warning on the dry-run screen. Either way, add a test in
the shape of `TestExistingPageIsUpdatedOrSkipped` with a `Zustand` column, an
empty cell and an existing `published` page.

## Warnings

### WR-01: `foldHeader` closes D-28 for ä/ö/ü only; every other accent still folds to two different keys

**File:** `internal/csvimport/mapping.go:69-107`

**Issue:** `settleMarks` composes a combining diaeresis back onto `a`, `o`, `u`
and their capitals, and drops every other combining mark while keeping its base
letter. `field.SlugifyKey` (`field/field.go:876-911`) has cases for `ä ö ü ß`
only and **drops** every other non-ASCII letter outright. The two rules disagree
for every accent that is not one of those three:

| heading | normalisation | `settleMarks` | `SlugifyKey` |
|---|---|---|---|
| `Café` | NFC (`é` = U+00E9) | `Café` | `caf` |
| `Café` | NFD (`e` + U+0301) | `Cafe` | `cafe` |

Two keys for one word — verbatim the failure the D-28 correction was written to
prevent, moved from `ö` to `é`. A field whose label is `Café` gets the key `caf`
(`SlugifyKey` on the label at definition time), so a spreadsheet exported by a
macOS tool, or by anything that normalises to NFD, produces a heading that does
not match its own field and lands unmapped with no note against it — `AutoMap`'s
note loop only speaks when another column took the key (`mapping.go:370-384`).
The same holds for `ñ`, `ç`, `å`, `š` and the rest of `page.transliterations`.

The doc comment sees the disagreement and disposes of it: *"that disagreement
lives in SlugifyKey, is older than this phase and is not this phase's to
change."* That is true of `SlugifyKey`, but the divergence between the two
spellings of one heading is new — before this phase there was no NFD handling at
all, and both spellings failed the same way. `foldCell` (`row.go:37-39`) does not
have the problem, because `page.Transliterate` maps `é` to `e`; only the header
fold does.

**Fix:** make the two spellings agree, which is the requirement — the *value* of
the key does not matter, only that one word yields one key. Mirror `SlugifyKey`'s
answer for the marks it has no case for by dropping the base letter with the
mark:

```go
 		if i+1 < len(runes) && runes[i+1] == combiningDiaeresis {
 			if composed, ok := precomposed[r]; ok {
 				b.WriteRune(composed)
 				i++
 				continue
 			}
 		}
+		// A mark on any other base letter: drop the base with it, because
+		// SlugifyKey drops the precomposed spelling of the same letter whole.
+		// Keeping the base would give "cafe" for the decomposed Café and "caf"
+		// for the composed one — two keys for one word, which is exactly the
+		// failure this function exists to prevent. Prettier is not the goal;
+		// agreeing with the other spelling is.
+		if i+1 < len(runes) && unicode.Is(unicode.Mn, runes[i+1]) {
+			i++
+			continue
+		}
 		b.WriteRune(r)
```

Extend `TestFoldHeaderComposesCombiningUmlauts` with an `é` case asserting that
the NFC and NFD spellings fold to the *same* key, whatever that key is.

### WR-02: a user-visible sentence is built with `fmt.Sprintf` in Go and rendered on the report

**File:** `internal/csv/csv.go:239-241`, `internal/csvimport/row.go:275-277`,
`cmd/holzcloud/templates/admin/csv_reason.html:19`

**Issue:** `csv.Reader.Next` builds

```go
row.Error = fmt.Sprintf("row has %d cells, the header has %d", len(rec), len(r.header))
```

`CheckRow` carries that string forward as `ReasonRowUnreadable`'s only argument,
and `csv_reason.html:19` substitutes it into a German sentence:

> Diese Zeile liess sich nicht lesen: row has 5 cells, the header has 3

The frame is a `{{tf}}` literal the tool can see; the half a person actually
needs is an English string `tools/i18n` cannot. This is D-32's hole exactly,
narrowed from the whole sentence to its argument — and `csv.go`'s package comment
asserts the opposite: *"Every message this package produces is a short technical
reason for a developer or an error value."* It is not; it reaches the operator's
report screen.

The reachable path is ordinary, not exotic: any row with more cells than the
header. (The sibling message at `csv.go:247` is unreachable on screen —
`CheckRow`'s own cell loop returns `ReasonCellTooLong` first — but it sits in the
same function and will be reached the day that loop moves.) Three further
arguments carry raw Go error text into the same screen: `ReasonBodyUnreadable`
(`row.go:316`), `ReasonNotWritten` (`row.go:408`, `:507`, `:524`) and
`ReasonNotRolledBack` (`row.go:500`, `:519`, `:522`).

**Fix:** give the too-wide row its own code, the way the too-long cell already
has one — the numbers are arguments, not prose:

```go
// verdict.go
// ReasonRowTooWide: the row has more cells than the header.
// Arguments: the row's cell count, the header's.
ReasonRowTooWide Reason = "row_too_wide"
```

```go
// csv.go — a code and its numbers, not a sentence
row.Wide = len(rec)   // or a small typed value beside Error
```

```html
{{- else if eq .Reason "row_too_wide"}}{{tf "Diese Zeile hat %s Zellen, die Kopfzeile hat %s." (index .Args 0) (index .Args 1)}}
```

`TestCSVEveryReasonHasASentence` will then hold the new arm in place. Keep
`ReasonRowUnreadable` for `encoding/csv`'s own `ParseError`, which is stdlib
prose and not this phase's to translate — but say so in the comment, so the
exemption reads as a decision.

### WR-03: `verdict.go` says the screen does not show the store's error; the template shows it

**File:** `internal/csvimport/verdict.go:114-118`,
`cmd/holzcloud/templates/admin/csv_reason.html:30-31`

**Issue:**

```go
// ReasonNotWritten: the store refused the row, or the row was taken back
// cleanly after a later step failed. Arguments: the page's title, what the
// store said. The second argument is a Go error meant for the log; the
// screen shows the title and the code's own sentence.
```

The screen does not:

```html
{{- else if eq .Reason "not_written"}}{{tf "„%s“ wurde nicht geschrieben: %s" (index .Args 0) (index .Args 1)}}
```

So `page.Store`'s wrapped error — which on a constraint failure carries the SQL
statement's own text — is printed on the report. The audience is an admin, so
this is not an escalation; the defect is that the comment states a guarantee the
code does not keep, and the next person to add a reason will believe it. The same
sentence covers `ReasonNotRolledBack` at `:104-113`.

**Fix:** pick one and make the other agree. Either correct the comment —

```go
// Arguments: the page's title, and what the store said. The second argument is
// raw Go error text and the report prints it: an operator who cannot see it
// cannot tell a duplicate address from a disk that is full, and this screen is
// behind an admin session.
```

— or drop `(index .Args 1)` from both arms and log the error with `slog.Error`
instead, which is what the comment currently promises.

### WR-04: `update()` swallows an encode failure and reports the row as a clean update

**File:** `internal/csvimport/row.go:591-593`

**Issue:**

```go
if encoded, err := field.Encode(field.Clean(defs, merged)); err == nil {
    u.Fields = encoded
}
```

If `Encode` fails, `u.Fields` keeps `existing.Fields` — every mapped field value
of the row is silently discarded, `UpdatePage` succeeds, and `WriteRow` returns
the verdict `CheckRow` minted: outcome *aktualisieren*, no reason. The report
tells the operator the row went through with the values they mapped. It did not.

The create path handles the identical call correctly: `row.go:406-409` returns
`ReasonNotWritten` when `field.Encode` fails. The two arms disagree about whether
the same failure matters.

`field.Encode` is `json.Marshal` over `map[string]string`, so this is close to
unreachable today — but it is a swallowed error on the write path, on the one arm
that touches pages the operator already had, and the cost of fixing it is three
lines.

**Fix:** hand the error out of `update()` and report it:

```go
-func (w Writer) update(existing *page.Page, create page.PageCreate, data field.Data,
-	defs []field.Def, m Mapping, userID *int64) page.PageUpdate {
+func (w Writer) update(existing *page.Page, create page.PageCreate, data field.Data,
+	defs []field.Def, m Mapping, userID *int64) (page.PageUpdate, error) {
...
-	if encoded, err := field.Encode(field.Clean(defs, merged)); err == nil {
-		u.Fields = encoded
-	}
-	return u
+	encoded, err := field.Encode(field.Clean(defs, merged))
+	if err != nil {
+		// The create arm returns ReasonNotWritten for this exact failure. An
+		// update that silently keeps the old values and reports success is the
+		// one answer the operator cannot check.
+		return u, err
+	}
+	u.Fields = encoded
+	return u, nil
```

and in `WriteRow`'s update arm, turn a non-nil error into
`ReasonNotWritten` before `UpdatePage` is called.

### WR-05: two concurrent posts to `/start` on one token both import the file

**File:** `internal/admin/csvimport.go:838-880`,
`internal/csvimport/store.go:178-184`

**Issue:** the staging row is deleted **after** the loop (`csvimport.go:878`),
and the comment above it defends that placement against a *refresh*
(`TestCSVReloadDoesNotImportTwice`, `csvimport_test.go:1084`) — which it does
close, because the row is gone by the time the report renders. It does not close
two requests that overlap. Both pass `staged()`, both reach the loop, and in
`csvModeNew` both call `CreateWebsite` (`:853`): two websites, each carrying the
whole file. In `csvModeExisting` with *Aktualisieren* the second run rewrites
every page the first just wrote; with *Übergehen* it mostly self-heals.

Reachable by a double-click. `hx-disabled-elt="this"` on the commit button
(`csv_dryrun.html:76`) does not help: `base.html:12` carries `hx-headers` only,
there is no `hx-boost`, and the form has no `hx-post` — so htmx never processes
the submit and never disables the button. That is a tree-wide convention
(`CLAUDE.md` requires the attribute) and not this phase's invention, but this is
the one handler in the tree where losing the race creates a whole website.

**Fix:** claim the row before the loop rather than after it. `Store.Delete`
already discards the count it needs:

```go
// Claim takes the staged row and says whether this caller got it. The write
// pool admits one connection, so exactly one of two overlapping commits sees
// a 1 — and the loser lands on the expiry screen, which is the honest answer:
// somebody else is already reading this file in.
func (s *Store) Claim(ctx context.Context, id int64) (bool, error) {
	res, err := s.DB.Write.ExecContext(ctx, `DELETE FROM csv_imports WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("claim staged csv upload: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
```

`upload.Data` is already in memory, so the run needs nothing from the row after
the claim. This trades away the "a process that dies mid-write leaves the row
intact and the operator can retry" argument at `store.go:174-177` — worth
trading, because a crash mid-write also leaves half the pages written, and
re-uploading a file costs one file picker. If the trade is refused, say so in the
comment and name the concurrent-commit window, so it is a decision rather than an
oversight.

### WR-06: the mapping screen renders a cell the reader has already refused, at full size

**File:** `internal/admin/csvimport.go:538-544`,
`cmd/holzcloud/templates/admin/csv_mapping.html:97`

**Issue:** `MaxCellBytes` is reported and never applied — `csv.Reader.Next` sets
`row.Error` and leaves `Cells` holding the whole string. That is right for the
report, which names the column and the size and not the value
(`ReasonCellTooLong`). The mapping screen does the opposite:

```go
if !data.NoRows && i < len(sample.Cells) {
    view.Sample = sample.Cells[i]
}
```

with no bound, and the template prints `{{$c.Sample}}` raw. A file whose first
row carries a 9 MB cell — legal inside the 10 MB body cap — produces a 9 MB HTML
response on screen 2, and the stepper will show it again on every visit. The
response is bounded by the upload cap, so this is not an amplification; it is
`MaxCellBytes`'s own stated purpose (`csv.go:54-60`: *"it would reach every
consumer of a row at that size"*) not being honoured at one of the two consumers.
This is the project's recurring signature — correct at every site but one.

**Fix:** truncate at the view boundary, where the number already lives:

```go
if !data.NoRows && i < len(sample.Cells) {
	// The reader reports an oversized cell rather than cutting it, so the
	// report can name its size. A screen is the other kind of consumer: it
	// has to draw the thing. Cut here and nowhere else, so the value the
	// dry run decides on is still the whole cell.
	view.Sample = sample.Cells[i]
	if len(view.Sample) > csvSampleBytes {
		view.Sample = view.Sample[:csvSampleBytes] + "…"
	}
}
```

with `csvSampleBytes` a named constant (200 is plenty for a sample) and a cut on
a rune boundary. Add a case to `TestCSVMappingIsInColumnOrder`'s neighbourhood
asserting the rendered length.

### WR-07: stepping the sample row throws away the operator's mapping

**File:** `cmd/holzcloud/templates/admin/csv_mapping.html:73-74`

**Issue:**

```html
{{if .PrevRow}}<a href="?row={{.PrevRow}}">{{t "vorherige Zeile"}}</a>{{end}}
{{if .NextRow}}<a href="?row={{.NextRow}}">{{t "nächste Zeile"}}</a>{{end}}
```

Plain anchors, inside the form but not part of it. Following one is a fresh GET,
`HandleCSVMapping` passes `chosen = nil` to `csvMappingData`
(`csvimport.go:571`), and the screen comes back with the automatic match — every
select the operator changed and every default they typed is gone, with nothing on
screen saying so. IMP-08's promise is a sample row *navigable to the next*, and
the navigation costs the operator their work on a file with any real number of
columns.

09-04's summary records this and defers it: *"D-38 anticipates a
`formmethod="GET"` submit that carries the mapping in the query string; that is a
09-05 refinement and the column cap of 100 is what makes it affordable."* 09-05
did not do it, does not mention it, and there is no `deferred-items.md` in the
phase directory — so the one thing the 100-column cap was half-justified by
(D-38's second reason: *"the mapping now rides in a query string"*) is now not
true of the shipped code either.

**Fix:** make the stepper a submit of the same form, which is what D-38 costed:

```html
<button type="submit" class="btn btn--sm" formmethod="GET"
        formaction="/admin/csv-import/{{.Token}}" name="row" value="{{.PrevRow}}"
        hx-disabled-elt="this">{{t "vorherige Zeile"}}</button>
```

`HandleCSVMapping` then reads the submitted mapping the way `csvPrepare` does and
passes it as `chosen`, which `csvMappingData` already supports. If it is deferred
again, put it in a `deferred-items.md` and correct D-38's second reason in the
same commit.

### WR-08: the example CSV has no path for a new website, and 404s on an installation with none

**File:** `cmd/holzcloud/templates/admin/website_list.html:104-119`,
`internal/admin/csvimport.go:915-924`

**Issue:** D-37 states the requirement and the answer: *"IMP-07 exists to help the
operator write the file… The 'new website' path gets the four fixed columns and
nothing else."* The panel explains that in prose (`website_list.html:107-109`) and
then offers only a `<select>` over `.Websites`. There is no control that produces
the fixed-column template, and `HandleCSVExample` accepts no request without an
existing website: `ParseInt("")` yields 0, `GetWebsite(0)` yields nil, and the
handler answers `http.NotFound`.

On a fresh installation — no websites yet, which is the single most likely moment
for a first CSV import — the `<select>` is empty, the button submits `website=`
and the operator gets a bare 404 page from the download form they were just told
to use. `TestCSVExampleUnknownWebsiteIsNotFound` (`csvimport_test.go:728`) pins
that 404 as correct behaviour, so the gap is asserted rather than caught.

**Fix:** accept the missing website as the fixed-column case, which is what the
panel already promises in words:

```go
	id, _ := strconv.ParseInt(r.FormValue("website"), 10, 64)
	var defs []field.Def
	name := ""
	if id != 0 {
		ws, err := h.domains.GetWebsite(r.Context(), id)
		if err != nil {
			return err
		}
		if ws == nil {
			http.NotFound(w, r)
			return nil
		}
		// A website that does not exist yet has no field definitions to build
		// columns from, so the file is the five fixed columns and nothing else.
		// That is the case IMP-07 exists for: the operator is writing the file
		// before the website is there.
		name, defs = ws.Name, mustList(...)
	}
```

with `csvExampleFilename("")` already yielding `website-vorlage.csv` (see IN-01),
and a second submit button in the panel — `name="website" value="0"` — labelled
for the new-website case. Keep `TestCSVExampleUnknownWebsiteIsNotFound` for a
*nonexistent* id and add one for `website=0`.

## Info

### IN-01: `csvExampleFilename`'s empty-slug branch is unreachable

**File:** `internal/admin/csvimport.go:962-968`

**Issue:** `page.Slugify` returns `"untitled"` when the result would be empty
(`page/slug.go:118-120`), so `if slug == ""` never fires and a website named
`«»` downloads as `untitled-vorlage.csv`. Harmless, but it is a guard that reads
as coverage and is not.

**Fix:** drop the branch, or keep it and say it is defence against a future
`Slugify` — the comment currently says neither. It becomes live if WR-08 is taken,
where the fallback is the new-website case.

### IN-02: the group-row fallback passes a key where the template's `%s` is a label

**File:** `internal/csvimport/row.go:384-397`,
`cmd/holzcloud/templates/admin/csv_reason.html:25`

**Issue:** `ReasonFieldRejected`'s documented arguments are *the field's label,
Check's own reason*, and the first loop supplies `d.Label`. The fallback for a key
belonging to a group row supplies `keys[0]` — a raw key such as `oeffnung.0.von`
— into the same `{{tf "„%s“: %s"}}`. The operator reads an internal key in the
slot where every other row shows a label.

**Fix:** resolve the key back to a label where one exists, or give the group case
its own reason code so the sentence can say what it is looking at.

### IN-03: `Upload.CreatedAt` is read by nothing but a test

**File:** `internal/csvimport/store.go:91`, `:164-166`,
`internal/csvimport/store_test.go:120-122`

**Issue:** `Prune` compares `erstellt_am` in SQL and never touches the Go field;
no handler and no template reads `CreatedAt`. The test that keeps it alive says
*"CreatedAt is empty — the sweep hangs on that column"*, which is true of the
column and not of the field it asserts.

**Fix:** either use it — the mapping screen could say how long the upload has
left before the sweep, which is a real thing to tell somebody — or drop the field
and the parse, and let the test assert against the column.

### IN-04: the class gate covers three of the five new templates

**File:** `internal/admin/csvimport_test.go:579-600`

**Issue:** `TestCSVScreensUseOnlyClassesThatExist` walks `csv_dryrun.html`,
`csv_report.html` and `csv_reason.html`. `csv_mapping.html` and
`csv_expired.html` are not in the list, and `csv_mapping.html` is the screen with
the most markup of the five. I checked its classes by hand and all of them have
rules; the gate simply does not.

**Fix:** add the two names to the slice.

### IN-05: `csvRun`'s comment inventories the loop's writes and omits `UpdatePage`

**File:** `internal/admin/csvimport.go:668-673`

**Issue:** *"Every write the loop performs is a store call that opens and closes
its own inside one row — CreatePage a single autocommitted INSERT, SetForPage its
own at term/store.go:120."* The update arm's `page.UpdatePage` is also a write the
loop performs and it opens a `BeginTx` (`page/store.go:547`). IMP-10 still holds
— the transaction opens and closes inside one row — but the list is the thing a
later reader will check the guarantee against, and it is missing the one entry
that actually opens a transaction. `row.go:455-459` gets this right and names all
four.

**Fix:** add `UpdatePage` to the list, and say that it opens one *inside* a row,
which is precisely the distinction IMP-10 draws.

### IN-06: the four translation catalogues are red — 111 offen

**File:** `tools/i18n` output, `internal/i18n/*.json`

**Issue:** the baseline was `1158 übersetzt, 0 offen, 0 verwaist` in en/es/fr/it;
the tree now reports `1269 Zeichenketten im Quelltext` and `111 offen` in all
four. That is the D-32 mechanism working exactly as intended — the strings are
visible to the tool, which is the whole point — but the CSV feature is
German-only in four languages until they are filled in. 09-05's summary assigns
this to plan 09-06, which exists and has not run.

**Fix:** none in this code. Recorded so the phase is not signed off while the
gate reads red; it closes with 09-06.

### IN-07: `AutoMap` rebuilds the whole column slice for one number

**File:** `internal/csvimport/mapping.go:378-383`

**Issue:** `Columns(header)[first].Number` allocates a slice of every column to
read one field that is `first + 1` by construction (`mapping.go:190`).

**Fix:** `strconv.Itoa(first + 1)`, or hoist the `Columns(header)` call above the
loop if the indirection is wanted for the invariant it documents.

---

_Reviewed: 2026-09-06T18:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
