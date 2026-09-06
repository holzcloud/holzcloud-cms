package admin

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/csv"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
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
// A GET so the mapping screen is bookmarkable and so ?zeile=N can step the
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

// csvMaxUpload is what the upload screen admits, as bytes of the whole request
// body.
const csvMaxUpload = 10 << 20

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
	// PrevRow and NextRow are the stepper's two destinations, or 0 where the
	// control is ABSENT. Absent and not disabled: a disabled link is still a
	// control a keyboard user lands on.
	PrevRow int
	NextRow int
}

// CSVExpiredData is the expiry screen.
type CSVExpiredData struct {
	web.LayoutData
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
		data := CSVExpiredData{LayoutData: web.NewLayoutData(r, h.sm, "Der Upload ist abgelaufen")}
		data.ActiveNav = "websites"
		return nil, false, web.RenderAdmin(w, h.templates, r, "csv_expired", data)
	case err != nil:
		return nil, false, err
	}
	return upload, true, nil
}

// HandleCSVMapping shows the columns of the staged file and where they go —
// screen 2.
//
// GET /admin/csv-import/{token}. A GET so the screen has an address that can be
// bookmarked and so ?zeile=N steps the sample row without a POST.
//
// Everything is re-read here and nothing is trusted from screen 1 (D-29): the
// target website may have been deleted in between, and a field definition may
// have been added, renamed or removed. The definitions are read again at the dry
// run and again at the write for the same reason.
func (h *Handler) HandleCSVMapping(w http.ResponseWriter, r *http.Request) error {
	upload, ok, err := h.staged(w, r)
	if err != nil || !ok {
		return err
	}

	data := CSVMappingData{
		LayoutData:  web.NewLayoutData(r, h.sm, "Spalten zuordnen"),
		FormState:   web.NewFormState(),
		Token:       r.PathValue("token"),
		WebsiteName: upload.WebsiteName,
		Filename:    upload.Filename,
		Collision:   upload.Collision,
	}
	data.ActiveNav = "websites"

	var defs []field.Def
	if upload.Mode == csvModeExisting {
		ws, err := h.domains.GetWebsite(r.Context(), upload.WebsiteID)
		if err != nil {
			return err
		}
		if ws == nil {
			// 00049 sets website_id to NULL rather than cascading, precisely so
			// the wizard can end with a message instead of the file vanishing
			// under the operator's hands. This is that message.
			if err := h.csvImports.Delete(r.Context(), upload.ID); err != nil {
				return err
			}
			web.SetFlashError(h.sm, r.Context(),
				"Die Website dieses Imports gibt es nicht mehr. Der Import wurde abgebrochen; geschrieben wurde nichts.")
			return h.redirect(w, r, "/admin/websites")
		}
		data.Website = ws
		data.WebsiteName = ws.Name
		if defs, err = h.fields.List(r.Context(), ws.ID); err != nil {
			return err
		}
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
		// Unreachable for a file screen 1 staged, which parsed these exact bytes
		// before writing them, and the row is never updated. Answered rather
		// than asserted, because a 500 on a screen the operator reached by a
		// bookmark explains nothing.
		web.SetFlashError(h.sm, r.Context(), "Die abgelegte Datei lässt sich nicht mehr als Tabelle lesen. Bitte noch einmal hochladen.")
		return h.redirect(w, r, "/admin/websites")
	}

	columns := csvimport.Columns(reader.Header())
	mapping := csvimport.AutoMap(reader.Header(), defs)

	// ?zeile= is a 1-based DATA-row index and it is clamped rather than refused:
	// it is a number somebody typed into the address bar, not an attack
	// (IMP-08 adjacency). Clamping also means no arithmetic below can reach a
	// slice out of range.
	want, _ := strconv.Atoi(r.URL.Query().Get("zeile"))

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
			view.Sample = sample.Cells[i]
		}
		data.Columns[i] = view
	}

	return web.RenderAdmin(w, h.templates, r, "csv_mapping", data)
}
