package admin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/csv"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The CSV import, and the shape it is given on purpose because there is none to
// copy.
//
// **There is no multi-screen wizard anywhere else in this tree.** Every other
// admin handler answers one screen. Two half-analogs exist and they disagree,
// so each contributed one half and neither was copied whole:
//
//   - twofactor.go:184-201 is the only POST in this codebase that re-renders a
//     *different* screen after a rejected submit, with the half-finished state
//     re-fetched from the store rather than carried in the form. That is what a
//     rejected mapping does here.
//   - confirm.go:27-73 is the model for one handler serving both verbs of one
//     address, with the POST branch first and the GET render last.
//
// What is deliberately NOT copied is twofactor.go:154's EnsurePendingSecret,
// which keys the half-finished state on the **user id**. Keyed that way, a
// second upload would silently destroy the first: an operator with two browser
// tabs would lose the file they started with, and nothing would say so. The
// state here is keyed on a random token instead, one row per upload.
//
// **The token is a path segment, screen 2 is a GET, screens 3 and 4 are POSTs.**
// A GET so the mapping screen is bookmarkable and so ?row=N can step the
// sample row without a POST; POSTs for the dry run and the write because those
// are actions, and because screen 4 must not be repeatable by a refresh.
//
// **A swept token renders a named expiry screen and not a 404.** The operator
// did nothing wrong — the sweep took their file after a day — and a 404 says
// they did. A token belonging to somebody else is a 404, and the two answers
// are told apart on purpose; csvimport's own head comment carries the argument.
//
// **The routes are /admin/csv-import/{token} and not
// /admin/websites/import-csv/{token}.** The second cannot be registered: it and
// the existing GET /admin/websites/{id}/pages both match
// /admin/websites/import-csv/pages and neither is more specific, so Go 1.22's
// ServeMux panics in newRouter and the binary never serves a request. Screen 1
// stays at POST /admin/websites/import-csv, beside its two siblings at
// main.go:882-883.
//
// **An import may target an EXISTING website.** That is a deliberate departure
// from wordpress.go:15-19, which states that an importer here always creates a
// new one. That rule's own reason is that every collision needs an answer;
// here the answer is given twice before anything is written — once on screen 1,
// where the operator chooses between skipping and updating a row whose address
// already exists, and once by the dry run, which shows what each row would do
// before the write happens at all.

// The two values of csv_imports.modus.
//
// German because migration 00049 pins them in a CHECK constraint. They are
// data and not identifiers: translating either one here compiles cleanly and
// then has SQLite refuse the row at runtime.
const (
	csvModeNew      = "neu"
	csvModeExisting = "bestehend"
)

// The two form-field prefixes the mapping travels under, named once because
// four places have to agree on them: the template that emits them, the parser
// that reads them back, the dry run that re-emits them as hidden inputs, and
// the stepper that has to recognise a request carrying a mapping from one that
// does not.
//
// A column's target is per COLUMN and addressed by its index; a default is per
// TARGET and addressed by Target.String() (IMP-08), which is why the second is
// not "the same thing for defaults" but a different key space.
const (
	csvTargetPrefix  = "target_"
	csvDefaultPrefix = "default_"
)

// csvMaxUpload is what the upload screen admits, as bytes of the whole request
// body.
const csvMaxUpload = 10 << 20

// csvTermChunk bounds how many labels one term.EnsureNames transaction covers.
//
// **The number is measured and not chosen for looking round.** term.EnsureNames
// opens one transaction around every name it is handed (term/store.go:330), and
// the write pool admits one connection (db.go:42), so the length of one call is
// the length of the stall it imposes on every other request on the machine —
// admin pages, the public site, and session writes, which sit on the same pool
// (main.go:175). One 10 MB file measured 796 000 names in a single transaction:
// 12.2 s of import during which a competing write waited 8.1 s.
//
// The bound is set against what the row loop already proved acceptable. Five
// thousand pages written one row at a time took 1.10 s while a competing writer
// never waited more than 3.2 ms, so a transaction of roughly one row's duration
// is a grain of interleaving this machine is known to tolerate. An INSERT of a
// name costs about 10 microseconds here, which puts 500 names at about 5 ms —
// the same order as one row's writes, and two orders below anything a person
// notices. Smaller batches would buy nothing measurable and pay one transaction
// each.
const csvTermChunk = 500

// csvEnsureTerms creates the labels the file mentions, in batches no larger
// than csvTermChunk.
//
// The ensure function is passed in rather than the store, because what is under
// test is the size of the batches and not what a batch does: a fake counting
// them is the only way to assert the bound, and the bound is the whole point
// (TestCSVTermPrePassIsChunked).
//
// **Batching is not atomicity lost.** The INSERT is ON CONFLICT DO NOTHING and
// a label carries nothing of its own, so a run that fails after the third batch
// has created labels that are correct, reusable and — if no row then points at
// them — orphans of the same kind the pre-pass has always been able to leave.
// The alternative is the transaction this fix exists to remove.
func csvEnsureTerms(ctx context.Context, names []string,
	ensure func(context.Context, []string) error) error {

	for len(names) > 0 {
		n := min(len(names), csvTermChunk)
		if err := ensure(ctx, names[:n]); err != nil {
			return err
		}
		names = names[n:]
	}
	return nil
}

// csvSampleBytes bounds one cell of the sample row as SCREEN 2 DRAWS IT, and
// nowhere else.
//
// csv.MaxCellBytes is 100 kB and is reported rather than applied: the reader
// leaves an oversized cell whole so the report can name its column and its
// size instead of showing the value. That is right for the report, which draws
// no cell at all. A screen is the other kind of consumer named in
// csv.MaxCellBytes's own reasoning — it has to DRAW the thing — and a first row
// carrying a 9 MB cell, perfectly legal inside the 10 MB upload cap, made a
// 9 MB HTML response of the mapping screen, again on every step of the sample
// row.
//
// Two hundred bytes is a sample: enough to recognise what is in the column,
// which is the whole job of that table cell. The cut happens HERE and nowhere
// else, so the value the dry run decides on is still the whole cell.
const csvSampleBytes = 200

// CSVColumnView is one row of the mapping table.
//
// The column itself, plus the three things only the server can work out: which
// target the automatic match proposed, what this column holds in the sample row
// currently on screen, and why the column was left unmapped if it was.
type CSVColumnView struct {
	csvimport.Column
	// Selected is the target's form value, so the template can mark exactly one
	// <option> without knowing how a target is spelled.
	Selected string
	// Sample is this column's cell of the sample row, empty when the file has
	// no data rows at all.
	Sample string
	// Note says why the automatic match left this column alone. Its Reason is a
	// code and its sentence is a {{tf}} literal in the template (D-32).
	Note csvimport.Note
}

// CSVMappingData is the mapping screen.
type CSVMappingData struct {
	web.LayoutData
	web.FormState
	// Token is what every control on this screen carries forward. There is no
	// file input anywhere on it: the file is server-side (D-01).
	Token string
	// Website is the target when one already exists, nil for an import that
	// will create it. Re-read on every render (D-29): the website may have been
	// deleted since screen 1.
	Website *domain.Website
	// WebsiteName is what to print either way — the existing website's name, or
	// the name the new one will get.
	WebsiteName string
	// Filename is what the operator uploaded, so screen 2 names the file they
	// are talking about.
	Filename string
	// Collision is "uebergehen" or "aktualisieren", carried from screen 1 so the
	// screen can restate the answer the operator already gave.
	Collision string
	// Columns are the file's columns in the FILE'S ORDER, always — the order the
	// operator sees in their spreadsheet, which is what makes the sample row
	// beside them readable.
	Columns []CSVColumnView
	// Fields are the website's own definitions a column may be pointed at.
	Fields []field.Def
	// UnmappableFields are the definitions no column can ever fill, listed so
	// their absence from the select reads as a decision rather than an omission.
	UnmappableFields []field.Def
	// RequiredGroups are the labels of required group fields. A group is rows
	// and a CSV row is flat, so no cell can fill one — and field.CheckAll then
	// refuses EVERY row of the file. Named here because the refusal is otherwise
	// completely invisible until the dry run reports every single row skipped.
	RequiredGroups []string
	// NoOwnFields is true when the website defines none of its own, so the
	// screen says so instead of looking broken.
	NoOwnFields bool
	// NoRows is true for a file with a header and no data rows: the screen says
	// so in a sentence instead of drawing an empty table that looks like a bug.
	NoRows bool
	// Truncated says the file held more rows than csv.MaxRows.
	Truncated bool
	// TotalRows is how many data rows the file has.
	TotalRows int
	// SampleIndex is the 1-based data row on screen, already clamped.
	SampleIndex int
	// SampleNumber is that row's number as the operator's spreadsheet counts it,
	// minted by csv.RowNumber — the same helper the report uses, so "Zeile 2"
	// means the same row on both screens.
	SampleNumber int
	// LastRowNumber is the spreadsheet's number for the LAST data row, and it is
	// the second number of the sample line.
	//
	// TotalRows is a count of data rows and never counts the header, so putting
	// it beside SampleNumber — which does count the header — made the last row
	// of a twelve-row file read "Zeile 13 von 12". Two numbers in one sentence
	// have to be counted the same way; D-26 says which way that is. Found in a
	// browser and not by the suite, because no test read the two numbers of the
	// sentence together.
	LastRowNumber int
	// PrevRow and NextRow are the stepper's two destinations, or 0 where the
	// control is ABSENT. Absent and not disabled: a disabled link is still a
	// control a keyboard user lands on.
	PrevRow int
	NextRow int
	// Defaults are the per-target defaults, keyed by Target.String(), so a
	// rejected submit comes back with what the operator typed still in the
	// boxes.
	Defaults map[string]string
	// NoTitleTarget is why this screen is being shown again at 422. A bool and
	// not a sentence: the sentence is a {{t}} literal in the template, where
	// tools/i18n can see it (D-32).
	NoTitleTarget bool
}

// CSVExpiredData is the expiry screen.
type CSVExpiredData struct {
	web.LayoutData
}

// CSVHiddenInput is one name/value pair the dry run re-emits.
//
// The mapping is form state and lives nowhere else, so screen 3 has to carry it
// forward for screen 4 — and carrying it as the same names screen 2 posted is
// what makes the commit run the mapping the dry run described.
type CSVHiddenInput struct {
	Name  string
	Value string
}

// CSVDryRunData is screen 3: what would happen, with nothing written.
type CSVDryRunData struct {
	web.LayoutData
	// Token is what the commit button posts to.
	Token string
	// Filename and WebsiteName name the file and the target, so the screen says
	// what it is talking about.
	Filename    string
	WebsiteName string
	// NewWebsite is true when the website does not exist yet — it is made by
	// screen 4 and by nothing before it.
	NewWebsite bool
	// Collision is "uebergehen" or "aktualisieren", so the screen can restate
	// the answer screen 1 took.
	Collision string
	// Report is the same shape screen 4 renders, built by the same function
	// from verdicts the same CheckRow produced.
	Report csvimport.Report
	// Terms is how many labels the import can create, counted and not created.
	// The dry run used not to look at them at all.
	Terms int
	// Repeated is how many rows want an address a row above them takes. Named
	// on the screen when it is not zero, because "1 anlegen / 1 aktualisieren"
	// on a website that does not exist yet is otherwise unexplainable.
	Repeated int
	// Inputs are the mapping as hidden inputs.
	Inputs []CSVHiddenInput
}

// CSVReportData is screen 4: what happened.
type CSVReportData struct {
	web.LayoutData
	// WebsiteID is what the link beside the success count points at. "272
	// Seiten angelegt" is only useful next to the way to go and look at them
	// (D-25).
	WebsiteID   int64
	WebsiteName string
	Filename    string
	Report      csvimport.Report
}

// HandleCSVImport takes the file, checks it and stages it — screen 1.
//
// POST /admin/websites/import-csv, from the third <details> panel on the
// website list. It ends in a 303 to screen 2 and never renders anything itself,
// so a refresh of the mapping screen is an ordinary GET.
func (h *Handler) HandleCSVImport(w http.ResponseWriter, r *http.Request) error {
	// Eine CSV-Datei ist Text; zehn Megabyte sind mehr als jede Tabelle, die
	// jemand von Hand pflegt, und wenig genug für einen kleinen Server.
	r.Body = http.MaxBytesReader(w, r.Body, csvMaxUpload)

	file, header, err := r.FormFile("csv")
	if err != nil {
		// A file that is missing or over the cap is not a form-validation
		// error: there is nothing the operator typed to hand back, so this is a
		// flash and a redirect rather than a 422 — wordpress.go:28-32.
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/websites")
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden.")
		return h.redirect(w, r, "/admin/websites")
	}

	// The bytes first, while the operator is still holding the file: a refusal
	// here names the file they just picked, which a refusal three screens later
	// cannot. Three distinguishable errors, three distinguishable messages, each
	// set through web.SetFlashError by name — tools/i18n reads the first string
	// argument of eight named functions and nothing else, so a message built any
	// other way would be invisible to the translation gate (D-32).
	if err := csv.CheckBytes(raw); err != nil {
		switch {
		case errors.Is(err, csv.ErrEmpty):
			web.SetFlashError(h.sm, r.Context(), "Die Datei ist leer. Es wurde nichts abgelegt.")
		case errors.Is(err, csv.ErrOnlyBOM):
			web.SetFlashError(h.sm, r.Context(), "Die Datei enthält nur eine Byte-Reihenfolge-Marke und sonst nichts – so sieht ein leeres Tabellenblatt auf der Festplatte aus.")
		case errors.Is(err, csv.ErrNULByte):
			web.SetFlashError(h.sm, r.Context(), "Die Datei enthält ein Nullbyte und ist keine CSV-Datei.")
		default:
			web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden.")
		}
		return h.redirect(w, r, "/admin/websites")
	}

	// The header, which is also where the column cap lives (D-38): csv.New
	// refuses more than csv.MaxColumns rather than handing forward a mapping
	// screen with five thousand rows on it. A header CELL that is empty is not
	// a refusal — that column is offered as "Spalte N" on screen 2 and stays
	// pointable by hand.
	if _, err := csv.New(bytes.NewReader(raw)); err != nil {
		switch {
		case errors.Is(err, csv.ErrNoHeader):
			web.SetFlashError(h.sm, r.Context(), "Die Datei hat keine Kopfzeile. Die erste Zeile muss die Spaltenüberschriften enthalten.")
		case errors.Is(err, csv.ErrTooManyColumns):
			web.SetFlashError(h.sm, r.Context(), web.Titlef(r,
				"Die Datei hat mehr als %d Spalten. Es wurde nichts abgelegt.", csv.MaxColumns))
		default:
			web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht als Tabelle gelesen werden.")
		}
		return h.redirect(w, r, "/admin/websites")
	}

	upload := csvimport.Upload{
		UserID:   h.sm.GetInt64(r.Context(), auth.SessionKeyUserID),
		Filename: header.Filename,
		Data:     raw,
	}

	// The target, as the panel asked it (IMP-04). Anything that is not the one
	// known alternative falls back to the safe answer, in both cases: a new
	// website rather than an existing one, and skipping rather than updating,
	// because the safe direction is the one that writes less.
	upload.Mode = csvModeNew
	if r.FormValue("target") == csvModeExisting {
		upload.Mode = csvModeExisting
	}
	upload.Collision = csvimport.CollisionSkip
	if r.FormValue("collision") == csvimport.CollisionUpdate {
		upload.Collision = csvimport.CollisionUpdate
	}

	if upload.Mode == csvModeExisting {
		id, _ := strconv.ParseInt(r.FormValue("website"), 10, 64)
		ws, err := h.domains.GetWebsite(r.Context(), id)
		if err != nil {
			return err
		}
		if ws == nil {
			web.SetFlashError(h.sm, r.Context(), "Diese Website gibt es nicht.")
			return h.redirect(w, r, "/admin/websites")
		}
		upload.WebsiteID = ws.ID
		upload.WebsiteName = ws.Name
	} else {
		// The same cascade wordpress.go:40-46 uses: what was typed, then what
		// the file was called, then a name that is at least honest about where
		// the website came from.
		upload.WebsiteName = strings.TrimSpace(r.FormValue("name"))
		if upload.WebsiteName == "" {
			upload.WebsiteName = strings.TrimSuffix(header.Filename, path.Ext(header.Filename))
		}
		if upload.WebsiteName == "" {
			upload.WebsiteName = "Aus einer Tabelle"
		}
	}

	// The website is NOT created here, and that is the difference between this
	// importer and wordpress.go:48, which creates one before it has read a
	// single item. It is created by screen 4, after the dry run, so an operator
	// who looks at the mapping and walks away does not leave an empty website
	// behind for somebody else to find and wonder about.
	token, err := h.csvImports.Stage(r.Context(), upload)
	if err != nil {
		return err
	}
	return h.redirect(w, r, "/admin/csv-import/"+token)
}

// staged resolves the token in the path to the upload it stands for, and
// answers the two refusals itself.
//
// Every token-bearing screen goes through here and none of them re-implements
// the rule, so the two further screens plan 09-05 adds cannot forget it. The
// second return value is false when this function has already written the
// response; the caller returns immediately in that case.
//
// The ordering discipline, in this phase's own terms: an upload the sweep took
// and an upload belonging to somebody else get DIFFERENT answers, honestly,
// because only the second one is a refusal. The first is the sweep's doing and
// a 404 would tell the operator they had done something wrong. The store draws
// that distinction (csvimport.ErrExpired against csvimport.ErrForeign) and this
// is the one place either is turned into a response.
func (h *Handler) staged(w http.ResponseWriter, r *http.Request) (*csvimport.Upload, bool, error) {
	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	upload, err := h.csvImports.Get(r.Context(), r.PathValue("token"), userID)
	switch {
	case errors.Is(err, csvimport.ErrForeign):
		// Refused before a single byte of the file has been read, so no cell of
		// it can reach the response.
		http.NotFound(w, r)
		return nil, false, nil
	case errors.Is(err, csvimport.ErrExpired):
		return nil, false, h.csvExpired(w, r)
	case err != nil:
		return nil, false, err
	}
	return upload, true, nil
}

// csvExpired is the screen for a token that answers to no staged row.
//
// Two callers and one answer. The sweep took the upload after a day, or another
// request claimed it a moment ago and is importing it right now. Neither is
// something the operator did wrong, so neither is a 404, and both mean the same
// thing to them: this file is not here to be read in, start again.
func (h *Handler) csvExpired(w http.ResponseWriter, r *http.Request) error {
	data := CSVExpiredData{LayoutData: web.NewLayoutData(r, h.sm, "Der Upload ist abgelaufen")}
	data.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "csv_expired", data)
}

// csvTarget re-reads the website this import points at and its field
// definitions.
//
// Called by screen 2, screen 3 and screen 4, and never once at the start. The
// wizard spans four requests and an unknown amount of wall time, so between any
// two of them the target website can be deleted and a field definition can be
// added, renamed or removed (D-29, IMP-04 concurrency). A definition that went
// is reported per row by CheckRow and its column falls back to unmapped; a
// website that went ends the wizard here.
//
// The second return value is false when the response has already been written,
// which is staged's convention and not a second one.
func (h *Handler) csvTarget(w http.ResponseWriter, r *http.Request, upload *csvimport.Upload) (*domain.Website, []field.Def, bool, error) {
	if upload.Mode != csvModeExisting {
		// There is no website yet. Screen 4 makes it, and until then there is
		// nothing to re-read and no definition to name.
		return nil, nil, true, nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), upload.WebsiteID)
	if err != nil {
		return nil, nil, false, err
	}
	if ws == nil {
		// 00049 sets website_id to NULL rather than cascading, precisely so the
		// wizard can end with a message instead of the file vanishing under the
		// operator's hands. This is that message — and then the row is cleared
		// up, because ten megabytes should not wait a day for the sweep on
		// behalf of a wizard that can no longer be finished.
		if err := h.csvImports.Delete(r.Context(), upload.ID); err != nil {
			return nil, nil, false, err
		}
		web.SetFlashError(h.sm, r.Context(),
			"Die Website dieses Imports gibt es nicht mehr. Der Import wurde abgebrochen; geschrieben wurde nichts.")
		return nil, nil, false, h.redirect(w, r, "/admin/websites")
	}

	defs, err := h.fields.List(r.Context(), ws.ID)
	if err != nil {
		return nil, nil, false, err
	}
	return ws, defs, true, nil
}

// csvHeader reads the header row back out of the staged bytes.
//
// Unreachable as an error for a file screen 1 staged, which parsed these exact
// bytes before writing them, and the row is never updated. Answered rather than
// asserted all the same, because a 500 on a screen the operator reached from a
// bookmark explains nothing.
func csvHeader(upload *csvimport.Upload) ([]string, error) {
	reader, err := csv.New(bytes.NewReader(upload.Data))
	if err != nil {
		return nil, err
	}
	return reader.Header(), nil
}

// csvUnreadable is the one answer to a staged file that no longer parses.
func (h *Handler) csvUnreadable(w http.ResponseWriter, r *http.Request) error {
	web.SetFlashError(h.sm, r.Context(),
		"Die abgelegte Datei lässt sich nicht mehr als Tabelle lesen. Bitte noch einmal hochladen.")
	return h.redirect(w, r, "/admin/websites")
}

// csvMappingData builds screen 2, with a mapping already decided.
//
// Two callers and two mappings. The GET passes nil and gets the automatic
// match. The dry run passes back what the operator submitted when the mapping
// cannot be run — a mapping with no title column — so the screen comes back
// with their choices intact rather than thrown away, which is what separates a
// form error from a flash and a redirect (twofactor.go:184-201).
func (h *Handler) csvMappingData(r *http.Request, upload *csvimport.Upload,
	ws *domain.Website, defs []field.Def, chosen *csvimport.Mapping) (CSVMappingData, error) {

	data := CSVMappingData{
		LayoutData:  web.NewLayoutData(r, h.sm, "Spalten zuordnen"),
		FormState:   web.NewFormState(),
		Token:       r.PathValue("token"),
		Website:     ws,
		WebsiteName: upload.WebsiteName,
		Filename:    upload.Filename,
		Collision:   upload.Collision,
	}
	data.ActiveNav = "websites"
	if ws != nil {
		data.WebsiteName = ws.Name
	}

	for _, d := range defs {
		if csvimport.Mappable(d.Kind) {
			data.Fields = append(data.Fields, d)
			continue
		}
		data.UnmappableFields = append(data.UnmappableFields, d)
		if d.Kind == field.KindGroup && d.Required {
			data.RequiredGroups = append(data.RequiredGroups, d.Label)
		}
	}
	data.NoOwnFields = len(defs) == 0

	reader, err := csv.New(bytes.NewReader(upload.Data))
	if err != nil {
		return data, err
	}

	columns := csvimport.Columns(reader.Header())
	mapping := csvimport.AutoMap(reader.Header(), defs)
	if chosen != nil {
		// The operator's own choices win. The automatic match's NOTES stay,
		// because they explain the columns nobody has pointed anywhere yet, and
		// a column that is unmapped because its heading was taken twice is
		// still unmapped for that reason after a rejected submit.
		for i := range mapping.Targets {
			if i < len(chosen.Targets) {
				mapping.Targets[i] = chosen.Targets[i]
			}
		}
		mapping.Defaults = chosen.Defaults
	}
	data.Defaults = mapping.Defaults

	// ?row= is a 1-based DATA-row index and it is clamped rather than refused:
	// it is a number somebody typed into the address bar, not an attack
	// (IMP-08 adjacency). Clamping also means no arithmetic below can reach a
	// slice out of range.
	want, _ := strconv.Atoi(r.URL.Query().Get("row"))

	var first, last, at csv.Row
	found := false
	for {
		row, more := reader.Next()
		if !more {
			break
		}
		data.TotalRows++
		if data.TotalRows == 1 {
			first = row
		}
		last = row
		if data.TotalRows == want {
			at, found = row, true
		}
	}
	data.Truncated = reader.Truncated()

	var sample csv.Row
	switch {
	case data.TotalRows == 0:
		data.NoRows = true
	case want > data.TotalRows:
		data.SampleIndex, sample = data.TotalRows, last
	case found:
		data.SampleIndex, sample = want, at
	default:
		data.SampleIndex, sample = 1, first
	}
	if !data.NoRows {
		// The one place this number is minted for this screen, and it is
		// csv.RowNumber — the same helper the report calls, so "Zeile 2" is the
		// same row on both (D-26).
		data.SampleNumber = csv.RowNumber(data.SampleIndex - 1)
		// The same helper for the other end of the sentence, so both numbers on
		// screen count the header or neither does.
		data.LastRowNumber = csv.RowNumber(data.TotalRows - 1)
		if data.SampleIndex > 1 {
			data.PrevRow = data.SampleIndex - 1
		}
		if data.SampleIndex < data.TotalRows {
			data.NextRow = data.SampleIndex + 1
		}
	}

	data.Columns = make([]CSVColumnView, len(columns))
	for i, c := range columns {
		view := CSVColumnView{Column: c, Selected: mapping.Targets[i].String(), Note: mapping.Notes[i]}
		if !data.NoRows && i < len(sample.Cells) {
			view.Sample = csvSample(sample.Cells[i])
		}
		data.Columns[i] = view
	}
	return data, nil
}

// csvSample cuts one cell down to what a table cell on screen 2 can show.
//
// On a RUNE boundary, because the cut is measured in bytes: cutting through a
// multi-byte character would put a replacement glyph on the screen where the
// operator is trying to recognise their own data. The ellipsis says the value
// goes on, so a cell that was cut does not read as a cell that is short.
func csvSample(cell string) string {
	if len(cell) <= csvSampleBytes {
		return cell
	}
	cut := 0
	for i := range cell {
		if i > csvSampleBytes {
			break
		}
		cut = i
	}
	return cell[:cut] + "…"
}

// HandleCSVMapping shows the columns of the staged file and where they go —
// screen 2.
//
// GET /admin/csv-import/{token}. A GET so the screen has an address that can be
// bookmarked and so ?row=N steps the sample row without a POST.
//
// Everything is re-read here and nothing is trusted from screen 1 (D-29). The
// definitions are read again at the dry run and again at the write for the same
// reason.
func (h *Handler) HandleCSVMapping(w http.ResponseWriter, r *http.Request) error {
	upload, ok, err := h.staged(w, r)
	if err != nil || !ok {
		return err
	}
	ws, defs, ok, err := h.csvTarget(w, r, upload)
	if err != nil || !ok {
		return err
	}

	// The sample-row stepper is a submit button of the mapping form carrying
	// formmethod="GET", so stepping arrives here as a query string holding the
	// operator's own choices. They win over the automatic match, exactly as
	// they do when the dry run hands the mapping back at 422. Plain anchors
	// here used to throw the mapping away on every step, silently, and on a
	// file with thirty columns that is the whole of the operator's work.
	chosen, err := h.csvSubmittedMapping(r, upload)
	if err != nil {
		return h.csvUnreadable(w, r)
	}

	data, err := h.csvMappingData(r, upload, ws, defs, chosen)
	if err != nil {
		return h.csvUnreadable(w, r)
	}
	return web.RenderAdmin(w, h.templates, r, "csv_mapping", data)
}

// csvSubmittedMapping is the mapping THIS request carries in its own fields, or
// nil when it carries none.
//
// Nil and not an empty mapping, and the difference is the first visit: a
// mapping with every column pointed at nothing would override the automatic
// match with nothing, so a screen reached from screen 1 would come back blank.
// So the question asked is whether any mapping control is present at all.
func (h *Handler) csvSubmittedMapping(r *http.Request, upload *csvimport.Upload) (*csvimport.Mapping, error) {
	if err := r.ParseForm(); err != nil {
		return nil, nil
	}
	carries := false
	for name := range r.Form {
		if strings.HasPrefix(name, csvTargetPrefix) || strings.HasPrefix(name, csvDefaultPrefix) {
			carries = true
			break
		}
	}
	if !carries {
		return nil, nil
	}

	header, err := csvHeader(upload)
	if err != nil {
		return nil, err
	}
	m := csvMappingFromForm(r, len(header))
	return &m, nil
}

// csvMappingFromForm reads the mapping the operator submitted.
//
// It is form state and not a column on the staging row, which is what keeps
// that row write-once — and the write-once row is what makes the dry run
// trustworthy, because the dry run and the write then read bytes nothing has
// touched in between (D-30).
//
// A field target whose key names no definition is KEPT rather than dropped.
// That is deliberate: CheckRow reports it per row as ReasonFieldMissing and
// falls the column back to unmapped, which is a reported row instead of a
// silently ignored column (D-29). Dropping it here would make a definition
// deleted mid-wizard invisible.
func csvMappingFromForm(r *http.Request, columns int) csvimport.Mapping {
	m := csvimport.Mapping{
		Targets:  make([]csvimport.Target, columns),
		Notes:    make([]csvimport.Note, columns),
		Defaults: map[string]string{},
	}

	for i := range m.Targets {
		m.Targets[i] = csvTargetFromForm(r.FormValue(csvTargetPrefix + strconv.Itoa(i)))
	}

	// The default belongs to the TARGET and not to the column (IMP-08), so the
	// form name is "default_" plus the target's own spelling and the two ends
	// of the round trip are one rule. Screen 3 re-emits exactly these names as
	// hidden inputs, so the commit posts the mapping the dry run described.
	//
	// A default for a target that cannot carry one is dropped here as well as
	// ignored in cellFor. A mapping arriving from a form is not the screen's
	// mapping, and the door the screen does not draw is not one a hand-made
	// POST may open either — csvTargetFromForm answers the same way for the
	// same reason.
	if err := r.ParseForm(); err == nil {
		for name, values := range r.Form {
			key, isDefault := strings.CutPrefix(name, csvDefaultPrefix)
			if !isDefault || key == "" || len(values) == 0 {
				continue
			}
			if !csvimport.TakesDefault(csvTargetFromForm(key)) {
				continue
			}
			if value := strings.TrimSpace(values[0]); value != "" {
				m.Defaults[key] = value
			}
		}
	}
	return m
}

// csvTargetFromForm reads one <select>'s value back into a target.
//
// A value outside the closed set becomes TargetNone: a mapping arriving from a
// form is not the screen's mapping, and the safe direction is the one that
// writes less.
func csvTargetFromForm(value string) csvimport.Target {
	if key, isField := strings.CutPrefix(value, csvimport.TargetField+":"); isField && key != "" {
		return csvimport.Target{Kind: csvimport.TargetField, Key: key}
	}
	switch value {
	case csvimport.TargetTitle, csvimport.TargetSlug, csvimport.TargetBody,
		csvimport.TargetStatus, csvimport.TargetTerms:
		return csvimport.Target{Kind: value}
	}
	return csvimport.Target{Kind: csvimport.TargetNone}
}

// csvMappingInputs is the mapping as hidden inputs, so screen 3's commit button
// posts exactly the mapping screen 3 described.
//
// Built from the parsed mapping rather than copied out of the request, so what
// goes forward is what the dry run actually ran — and sorted, so two runs over
// one file produce the same markup.
func csvMappingInputs(m csvimport.Mapping) []CSVHiddenInput {
	out := make([]CSVHiddenInput, 0, len(m.Targets)+len(m.Defaults))
	for i, t := range m.Targets {
		out = append(out, CSVHiddenInput{Name: csvTargetPrefix + strconv.Itoa(i), Value: t.String()})
	}

	keys := make([]string, 0, len(m.Defaults))
	for key := range m.Defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, CSVHiddenInput{Name: csvDefaultPrefix + key, Value: m.Defaults[key]})
	}
	return out
}

// csvRunResult is what one pass over the staged file produced.
//
// A struct and not four return values, because two of the four were added to
// close a gap and a fifth is plausible: the dry run has to be able to say
// everything the write will do, and every time it could not, the screen the
// operator is asked to believe was quietly narrower than the run behind it.
type csvRunResult struct {
	// Verdicts is one per row, in file order.
	Verdicts []csvimport.Verdict
	// Truncated: the file held more rows than csv.MaxRows, and a cap is
	// reported rather than silently applied.
	Truncated bool
	// Terms is how many distinct labels the rows of this file can store —
	// capped per row by csvimport.TermNames, which is the number the write arm
	// hands to term.EnsureNames. Labels that already exist are counted too:
	// knowing which do would cost a query per name, and "up to this many" is
	// the honest thing a run that reads nothing can say.
	Terms int
	// Repeated is how many rows of the DRY RUN wanted an address a row above
	// them takes. Zero on the write arm, which needs no such count because its
	// per-row lookup is live.
	Repeated int
}

// csvRun walks the staged bytes once and produces one verdict per row.
//
// **`write` is the only difference between the dry run and the write**, and the
// write is suppressed by not being reached rather than by a second code path.
// Two functions deciding the same thing drift, and the drifted one is always
// the one the operator did not see (D-22).
//
// **Both callers build their reader from upload.Data — the same staged bytes.**
// That is what makes IMP-05 true, and it is the property "re-submit the file
// with the mapping" could not have had: under that design the operator could
// have picked a DIFFERENT file at the commit step and the dry run would have
// been describing the wrong one, silently (D-30).
//
// **No transaction is opened here and none spans two rows** (IMP-10). Every
// write the loop performs is a store call that opens and closes its own inside
// one row — CreatePage a single autocommitted INSERT, SetForPage its own at
// term/store.go:120. The write pool admits one connection (db.go:28,
// _txlock=immediate), so other requests, admin and public, interleave BETWEEN
// rows. A transaction held across a file would block every request on the
// machine, which is the anti-feature IMP-10 exists to forbid.
//
// **The term pre-pass below stands outside the loop, and it is the one place
// that sentence was ever false.** term.EnsureNames opens a transaction of its
// own, and it used to be handed every name the whole file mentioned — one
// transaction on the single write connection whose duration grew with the file
// (measured: 12.2 s, a competing write waiting 8.1 s). It is bounded twice now:
// TermNames harvests only what a row can store, and csvEnsureTerms hands the
// result over in batches of csvTermChunk. Neither bound alone would be enough
// to write this sentence down; both together are.
//
// The result carries what the file produced besides its verdicts, because the
// dry run has to be able to say all of it.
func (h *Handler) csvRun(ctx context.Context, upload *csvimport.Upload, m csvimport.Mapping,
	defs []field.Def, websiteID int64, write bool, userID *int64) (csvRunResult, error) {

	var result csvRunResult

	reader, err := csv.New(bytes.NewReader(upload.Data))
	if err != nil {
		return result, err
	}

	// The term names, harvested on BOTH arms.
	//
	// The write arm needs them because they have to exist before the first page
	// that points at one — that is internal/bundle/import.go:289-329's order and
	// not an optimisation: a term field's stored value is a slug, so the term it
	// names has to exist before the value referring to it is written, or the
	// page would carry an address resolving to nothing.
	//
	// The DRY RUN needs them because it used not to have them at all (the
	// harvest stood behind `if write`), so the screen whose whole purpose is to
	// show what would happen said nothing whatever about the labels the import
	// was about to create — and there can be thousands. It creates none of them,
	// it counts them.
	var rows []csv.Row
	for {
		row, more := reader.Next()
		if !more {
			break
		}
		rows = append(rows, row)
	}
	names := csvimport.TermNames(defs, m, rows)
	result.Terms = len(names)

	if write && len(names) > 0 {
		// In batches: EnsureNames opens ONE transaction around whatever it is
		// handed, and the write pool admits one connection, so a single call
		// covering a whole file holds the machine's only write connection for
		// the length of that file. csvTermChunk carries the measurement behind
		// the number.
		if err := csvEnsureTerms(ctx, names, func(ctx context.Context, batch []string) error {
			_, err := h.terms.EnsureNames(ctx, websiteID, batch)
			return err
		}); err != nil {
			return result, err
		}
	}

	// Two passes over bytes that are already in memory. The reader is re-opened
	// rather than the rows re-used, so the deciding pass reads the file exactly
	// the way a single-pass run would have.
	if reader, err = csv.New(bytes.NewReader(upload.Data)); err != nil {
		return result, err
	}

	// The addresses rows above the current one will take.
	//
	// The write arm needs no such list: its GetPageBySlug below is live and per
	// row, so row 40 genuinely finds the page row 4 has just created. The dry
	// run writes nothing, so the only way it can answer the same question is to
	// remember what it has already decided to create — and without this it
	// predicted create for BOTH rows of a file naming one address twice while
	// the write created once and then updated. IMP-05 and D-22 exist so that
	// the dry run cannot say something the write does not do, and a dry run
	// that is wrong about a row is worse than no dry run, because it is
	// believed. Measured as 5 anlegen / 2 aktualisieren against 4 angelegt /
	// 3 aktualisiert before this existed.
	planned := map[string]bool{}

	writer := csvimport.Writer{Pages: h.pages, Terms: h.terms}
	var verdicts []csvimport.Verdict
	for {
		// In FILE ORDER (IMP-04 ordering), so a file whose row 40 duplicates
		// row 4 gets the rename or the update in that order and the report
		// reads the way the file does. One pass, appended to once and by
		// nothing else (IMP-03 concurrency).
		row, more := reader.Next()
		if !more {
			break
		}

		// Per row, and NOT from a map built before the loop. A map goes stale
		// during a five-thousand-row write — row 40 creating the address row 4
		// already took would not be in it — and IMP-02's concurrency resolution
		// says the database is the arbiter rather than a pre-check in the
		// importer. The read pool is separate from the write pool, so these
		// lookups do not contend with the writes.
		var existing *page.Page
		slug := csvimport.RowSlug(row, m)
		if slug != "" && websiteID != 0 {
			found, err := h.pages.GetPageBySlug(ctx, websiteID, slug)
			if err != nil {
				return result, err
			}
			existing = found
		}

		if write {
			verdicts = append(verdicts,
				writer.WriteRow(ctx, websiteID, defs, row, m, existing, upload.Collision, userID))
			continue
		}

		if existing == nil && planned[slug] {
			// The page an earlier row of this same file will create. Only Slug
			// is read from it — CheckRow names it in the skipped-address reason
			// (row.go:441) and asks nothing else — and nothing else about a
			// page that does not exist yet could be said honestly. The
			// websiteID == 0 case is deliberately included: a file importing
			// into a website that screen 4 has not created yet is exactly where
			// this divergence was measured.
			existing = &page.Page{Slug: slug}
			result.Repeated++
		}

		v, _, _ := csvimport.CheckRow(defs, row, m, existing, upload.Collision)
		if v.Outcome == csvimport.OutcomeCreate && slug != "" {
			// Only a row that really creates takes an address. A row refused
			// for its title or its status creates nothing, so the row below it
			// is still free to have the address — which is why this is recorded
			// from the VERDICT and not from the mapping.
			planned[slug] = true
		}
		verdicts = append(verdicts, v)
	}
	result.Verdicts, result.Truncated = verdicts, reader.Truncated()
	return result, nil
}

// csvPrepare is the preamble both POST screens share: the staged file, the
// re-read target, the header and the submitted mapping.
//
// The false return means the response has already been written — the expiry
// screen, a 404, a flash and a redirect, or the mapping screen re-rendered at
// 422 because the mapping cannot be run.
func (h *Handler) csvPrepare(w http.ResponseWriter, r *http.Request) (*csvimport.Upload,
	*domain.Website, []field.Def, csvimport.Mapping, bool, error) {

	var none csvimport.Mapping

	upload, ok, err := h.staged(w, r)
	if err != nil || !ok {
		return nil, nil, nil, none, false, err
	}
	ws, defs, ok, err := h.csvTarget(w, r, upload)
	if err != nil || !ok {
		return nil, nil, nil, none, false, err
	}

	header, err := csvHeader(upload)
	if err != nil {
		return nil, nil, nil, none, false, h.csvUnreadable(w, r)
	}
	m := csvMappingFromForm(r, len(header))

	if m.ColumnFor(csvimport.TargetTitle, "") < 0 {
		// A mapping with no title column refuses every single row, so it is a
		// mapping error and not a file error. Re-rendered as a FORM error at
		// 422 with the operator's choices intact — a flash and a redirect here
		// would throw the whole mapping away, which is why twofactor.go:184-201
		// re-renders instead of redirecting and why page_handler_test.go:118-128
		// exists.
		data, err := h.csvMappingData(r, upload, ws, defs, &m)
		if err != nil {
			return nil, nil, nil, none, false, h.csvUnreadable(w, r)
		}
		data.NoTitleTarget = true
		return nil, nil, nil, none, false,
			web.RenderFormError(w, h.templates, r, "csv_mapping", data)
	}
	return upload, ws, defs, m, true, nil
}

// HandleCSVDryRun runs the whole file through the decision and writes nothing —
// screen 3.
//
// POST /admin/csv-import/{token}/probe. Every row is decided by the very
// function the write calls, so what the operator reads here is what the write
// will do (D-22), and SELECT COUNT(*) FROM pages is unchanged afterwards
// (IMP-05).
func (h *Handler) HandleCSVDryRun(w http.ResponseWriter, r *http.Request) error {
	upload, ws, defs, m, ok, err := h.csvPrepare(w, r)
	if err != nil || !ok {
		return err
	}

	run, err := h.csvRun(r.Context(), upload, m, defs, upload.WebsiteID, false, nil)
	if err != nil {
		return err
	}
	report := csvimport.Summarize(run.Verdicts)
	report.Truncated = run.Truncated

	data := CSVDryRunData{
		LayoutData:  web.NewLayoutData(r, h.sm, "Probelauf"),
		Token:       r.PathValue("token"),
		Filename:    upload.Filename,
		WebsiteName: upload.WebsiteName,
		NewWebsite:  upload.Mode == csvModeNew,
		Collision:   upload.Collision,
		Report:      report,
		Terms:       run.Terms,
		Repeated:    run.Repeated,
		Inputs:      csvMappingInputs(m),
	}
	data.ActiveNav = "websites"
	if ws != nil {
		data.WebsiteName = ws.Name
	}
	return web.RenderAdmin(w, h.templates, r, "csv_dryrun", data)
}

// HandleCSVStart writes the rows and reports what happened — screen 4.
//
// POST /admin/csv-import/{token}/start.
func (h *Handler) HandleCSVStart(w http.ResponseWriter, r *http.Request) error {
	upload, ws, defs, m, ok, err := h.csvPrepare(w, r)
	if err != nil || !ok {
		return err
	}

	// BEFORE anything is written, and before the website is created: the row
	// is claimed, and only the request that gets it imports. Two overlapping
	// commits on one token both passed staged(), both reached the loop and on
	// the "new website" path both called CreateWebsite — two websites, each
	// carrying the whole file, from one double-click. The loser lands on the
	// expiry screen, which is the honest answer: somebody else is already
	// reading this file in. Store.Claim carries the trade this makes.
	mine, err := h.csvImports.Claim(r.Context(), upload.ID)
	if err != nil {
		return err
	}
	if !mine {
		return h.csvExpired(w, r)
	}

	websiteID, websiteName := upload.WebsiteID, upload.WebsiteName
	if ws != nil {
		websiteName = ws.Name
	}
	if upload.Mode == csvModeNew {
		// HERE, and not on screen 1: an operator who looks at the mapping or at
		// the dry run and walks away leaves no empty website behind for
		// somebody else to find and wonder about. wordpress.go:48 creates one
		// before it has read a single item; this is the departure from it.
		created, err := h.domains.CreateWebsite(r.Context(), upload.WebsiteName, "")
		if err != nil {
			return err
		}
		// A new website changes what the resolver would answer for a host, so
		// the cache goes, exactly as wordpress.go:91 does it.
		h.resolver.InvalidateCache()
		websiteID, websiteName = created.ID, created.Name
		if defs, err = h.fields.List(r.Context(), websiteID); err != nil {
			return err
		}
	}

	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	run, err := h.csvRun(r.Context(), upload, m, defs, websiteID, true, &userID)
	if err != nil {
		return err
	}

	// Nothing to clear up here: the row was taken above, before the first
	// write. A refresh of this POST therefore finds no token and lands on the
	// expiry screen (D-33), the same as it did when the delete stood here —
	// and now a second request that arrives DURING the loop lands there too.
	report := csvimport.Summarize(run.Verdicts)
	report.Truncated = run.Truncated

	data := CSVReportData{
		LayoutData:  web.NewLayoutData(r, h.sm, "Einlesen abgeschlossen"),
		WebsiteID:   websiteID,
		WebsiteName: websiteName,
		Filename:    upload.Filename,
		Report:      report,
	}
	data.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "csv_report", data)
}

// HandleCSVExample hands out an example file built from one website's own field
// definitions — IMP-07.
//
// **A GET**, because all five downloads this tree already has are GETs
// (media.go:377, bundle.go:48, template.go:361, language.go:74 and :88,
// plugin.go:374) and a POST that returns a file would be the only one of its
// kind here (D-34).
//
// **Not behind the staging token** (D-37). IMP-07 exists to help the operator
// *write* the file; hung off the token it could be fetched only once the file it
// was meant to produce already existed. So this handler never calls staged: the
// website comes from a form value, which is ai.go:96's idiom and
// template.go:193's, and is reached from a second small GET form in the same
// panel — the shape page_list.html:11, media_list.html:22 and
// activity_log.html:15 already use.
//
// **It is a snapshot**, taken at the moment it is downloaded, and nothing
// downstream treats it as authoritative: a field added afterwards simply arrives
// unmapped, which the mapping screen shows and says.
func (h *Handler) HandleCSVExample(w http.ResponseWriter, r *http.Request) error {
	// The website id comes from a form VALUE and not from the path, so
	// auth.RequireWebsiteAccess (internal/auth/middleware.go:102) does not see
	// it: that middleware reads the id out of the URL path, and a route taking
	// a website from a form escapes it. Harmless today, and only today —
	// this route is behind requireAdmin, and NewWebsiteAccessLookup
	// (handler.go:167-169) returns true unconditionally for the role admin,
	// because user_websites restricts editors only. The day a route of this
	// shape is opened to editors, that is no longer true and this handler
	// needs its own check. Four other routes share the shape; that is a
	// separate change and not this one.
	name := ""
	var defs []field.Def
	if id, _ := strconv.ParseInt(r.FormValue("website"), 10, 64); id != 0 {
		ws, err := h.domains.GetWebsite(r.Context(), id)
		if err != nil {
			return err
		}
		if ws == nil {
			// The ordinary website-ownership answer, exactly as a page of a
			// website that is not there would answer. A website NAMED and not
			// there is still a 404; a website not named at all is the case
			// below, and the two are different questions.
			http.NotFound(w, r)
			return nil
		}
		if defs, err = h.fields.List(r.Context(), ws.ID); err != nil {
			return err
		}
		name = ws.Name
	}
	// No website named: the fixed columns and nothing else, which is what the
	// panel promises in words and what D-37 states for the "new website" path.
	// (D-37 counts four of them and the file carries five — Schlagwörter is
	// the fifth; the count is the decision's, the columns are
	// csvExampleColumns'.) It is also the single
	// most likely moment for a first import: a fresh installation has no
	// website yet, so there is nothing to read field definitions from, and
	// answering 404 there hands the operator a bare error page from the very
	// download they were just told to use.
	header, sample := csvExampleColumns(defs)
	body, err := csv.Example(header, [][]string{sample})
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	// Ohne nosniff könnte ein Browser den Inhalt anders deuten als die
	// Kopfzeile sagt, und die Angabe oben wäre eine Empfehlung statt einer
	// Schranke.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="`+csvExampleFilename(name)+`"`)
	// The file is a snapshot of the definitions as they are right now
	// (IMP-07 concurrency); a cached copy would be a snapshot of a different
	// moment and would quietly disagree with the mapping screen.
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(body)
	return err
}

// csvExampleFilename names the download after the website it came from.
//
// page.Slugify and not plugin.go:387's safeDownloadName, and the difference is
// provenance rather than taste: safeDownloadName filters a name a PLUGIN
// supplied, where filtering is the only option. This name comes from a website
// in this installation — bundle.go:71-77's case — and Slugify yields [a-z0-9-]
// only, so the result is safe by construction instead of by stripping
// characters afterwards.
//
// The empty name is the new-website case and it is answered FIRST, before
// Slugify: page.Slugify returns "untitled" for a string it can make nothing of
// (page/slug.go:118-120), so the old `if slug == ""` guard below it could never
// fire and a download with no website named would have arrived as
// "untitled-vorlage.csv" — a name that reads like a fault. "website-vorlage.csv"
// says what the file is.
func csvExampleFilename(name string) string {
	if strings.TrimSpace(name) == "" {
		return "website-vorlage.csv"
	}
	return page.Slugify(name) + "-vorlage.csv"
}

// csvExampleColumns builds the example's header row and its one sample row.
//
// Built here, in internal/admin, and deliberately not in internal/csv:
// csv.Example takes plain strings precisely so the pure package never learns
// about field.Def, which is what keeps its checklist against a hostile file
// provable without db.Open.
//
// The headings are the ones AutoMap matches on, so an operator who fills this
// file in gets an automatic match on every column — which is the whole of
// IMP-07. The definitions are walked in field.List's own order (ORDER BY
// position, id), the order they stand in on the field screen, and a definition
// csvimport.Mappable refuses gets no column at all: offering a heading no cell
// could ever fill would be a promise the dry run then breaks.
func csvExampleColumns(defs []field.Def) (header, sample []string) {
	header = []string{"Titel", "Adresse", "Text", "Zustand", "Schlagwörter"}
	sample = []string{"Beispielseite", "beispielseite", "Ein Satz über die Seite.", "entwurf", "Beispiel|Muster"}
	for _, d := range defs {
		if !csvimport.Mappable(d.Kind) {
			continue
		}
		header = append(header, d.Label)
		sample = append(sample, csvExampleCell(d))
	}
	return header, sample
}

// csvExampleCell is one plausible value for a field of this kind.
//
// This is the second place D-21's pipe and D-19's vocabulary become
// user-visible, and the file is where an operator actually meets them: a
// mehrfachauswahl cell shows the pipe, a janein cell shows "ja". Nothing here is
// translated — it is the content of a file, written once and then edited by the
// person who downloaded it, not a sentence on a screen.
func csvExampleCell(d field.Def) string {
	switch d.Kind {
	case field.KindLong:
		return "Ein längerer Text über mehrere Zeilen."
	case field.KindCode:
		return "beispiel"
	case field.KindNumber:
		return "12"
	case field.KindRange:
		if d.RangeMin != "" {
			return d.RangeMin
		}
		return "5"
	case field.KindDate:
		return "2026-09-06"
	case field.KindTime:
		return "09:30"
	case field.KindBool:
		return "ja"
	case field.KindChoice:
		if len(d.Choices) > 0 {
			return d.Choices[0]
		}
		return "rot"
	case field.KindMulti:
		if len(d.Choices) > 1 {
			return d.Choices[0] + "|" + d.Choices[1]
		}
		if len(d.Choices) == 1 {
			return d.Choices[0]
		}
		return "rot|blau"
	case field.KindLink:
		return "/eine-seite"
	case field.KindTerm:
		return "Beispiel"
	default:
		return "Beispiel"
	}
}
