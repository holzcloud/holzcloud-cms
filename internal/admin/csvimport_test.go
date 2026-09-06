package admin

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// What this file is the gate on.
//
// Two properties, and neither of them is visible from the mapping screen once
// it renders correctly. The first is the ownership check on the staging token:
// an upload belongs to the person who staged it, and a second admin holding the
// address must not see a single cell of somebody else's file — not in the body
// of a 404 either, which is why the assertions here read the body and not only
// the status. The second is that NOTHING IS WRITTEN before the dry run: screen
// 1 stages bytes and screen 2 reads them, and no page, no website and no field
// comes into being on either. Both are asserted against a real migrated
// database and the real templates from disk, so a template naming a field its
// data struct lacks fails here rather than in a browser.
//
// The counterparty of every authorisation assertion is a second admin account,
// seeded beside the first, because a check that is never given somebody else to
// refuse is not a check that has been tested.

// seedAdmin puts an account in the database and returns its id.
//
// Inserted with SQL rather than through a store: csv_imports.user_id carries a
// REFERENCES users(id), foreign keys are on, and a staged row therefore needs a
// user row that really exists.
func seedAdmin(t *testing.T, database *db.DB, email string) int64 {
	t.Helper()
	res, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', $1, 'x', 'admin')`, email)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// serveAs runs one handler with a live session that already belongs to a user,
// and hands back the flash the handler left behind.
//
// The flash is read here rather than by a second request because every refusal
// on screen 1 is a flash plus a redirect, and the message is the thing under
// test: three degenerate files must produce three different sentences.
func serveAs(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request) (*httptest.ResponseRecorder, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	var handlerErr error
	var flash string
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		handlerErr = fn(w, r)
		flash = sm.GetString(r.Context(), "flash_error")
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, flash
}

// uploadRequest builds the multipart POST the third panel sends.
func uploadRequest(t *testing.T, fields map[string]string, filename string, content []byte) *http.Request {
	t.Helper()
	body, contentType := multipartBody(t, fields, filename, content)
	req := httptest.NewRequest(http.MethodPost, "/admin/websites/import-csv", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	return req
}

func multipartBody(t *testing.T, fields map[string]string, filename string, content []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// The boundary is random per writer but its LENGTH is fixed, so the size of
	// the envelope is deterministic — which is what makes the byte cap testable
	// at the boundary and one step either side.
	mw.SetBoundary("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	fw, err := mw.CreateFormFile("csv", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}

// mappingRequest builds the GET of screen 2, with the path value the mux sets.
func mappingRequest(token, query string) *http.Request {
	target := "/admin/csv-import/" + token
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("token", token)
	return req
}

// stagedCount is how many uploads are lying in the staging table.
func stagedCount(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM csv_imports`).Scan(&n); err != nil {
		t.Fatalf("count staged: %v", err)
	}
	return n
}

// stage puts a file away directly, for the screens that begin with one.
func stage(t *testing.T, h *Handler, userID, websiteID int64, content string) string {
	t.Helper()
	return stageWith(t, h, userID, websiteID, content, csvimport.CollisionSkip)
}

// stageWith is stage with the collision answer screen 1 would have taken. The
// two arms are two different guarantees — skipping is idempotent by leaving the
// page alone, updating by rewriting the same values — so both need a fixture.
func stageWith(t *testing.T, h *Handler, userID, websiteID int64, content, collision string) string {
	t.Helper()
	upload := csvimport.Upload{
		UserID:      userID,
		WebsiteID:   websiteID,
		WebsiteName: "Testseite",
		Mode:        csvModeNew,
		Collision:   collision,
		Filename:    "tabelle.csv",
		Data:        []byte(content),
	}
	if websiteID != 0 {
		upload.Mode = csvModeExisting
	}
	token, err := h.csvImports.Stage(context.Background(), upload)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	return token
}

var tokenPattern = regexp.MustCompile(`^/admin/csv-import/[0-9a-f]{32}$`)

func TestCSVUploadStagesAndRedirects(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	req := uploadRequest(t, map[string]string{"target": "neu", "name": "Aus der Tabelle"},
		"seiten.csv", []byte("Titel,Text\nErste,Ein Satz\n"))
	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVImport, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d (flash %q); want 303", rec.Code, flash)
	}
	if loc := rec.Header().Get("Location"); !tokenPattern.MatchString(loc) {
		t.Errorf("Location = %q; want /admin/csv-import/ plus 32 hex characters", loc)
	}
	if n := stagedCount(t, database); n != 1 {
		t.Errorf("staged rows = %d; want 1", n)
	}

	// And no website came into being: screen 4 creates it, after the dry run,
	// so an abandoned wizard leaves nothing behind.
	websites, err := h.domains.ListWebsites(context.Background())
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(websites) != 1 {
		t.Errorf("websites = %d; want 1, the one the fixture made", len(websites))
	}
}

func TestCSVUploadIntoExistingWebsiteNamesIt(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	req := uploadRequest(t, map[string]string{
		"target":    "bestehend",
		"website":   fmt.Sprintf("%d", ws.ID),
		"collision": "aktualisieren",
	}, "seiten.csv", []byte("Titel,Text\nErste,Ein Satz\n"))
	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVImport, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d (flash %q); want 303", rec.Code, flash)
	}

	token := strings.TrimPrefix(rec.Header().Get("Location"), "/admin/csv-import/")
	screen, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	if screen.Code != http.StatusOK {
		t.Fatalf("mapping status = %d; want 200", screen.Code)
	}
	if !strings.Contains(screen.Body.String(), ws.Name) {
		t.Errorf("mapping screen does not name the target website %q", ws.Name)
	}
}

// The byte cap at the boundary and one step either side.
//
// Measured on the REQUEST BODY and not on the file inside it, because that is
// what http.MaxBytesReader bounds — the same place wordpress.go:25 puts it. A
// file of exactly ten megabytes therefore does not fit: the multipart envelope
// around it is part of what the cap counts.
func TestCSVTenMegabyteLimit(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	fields := map[string]string{"target": "neu"}
	envelope, _ := multipartBody(t, fields, "gross.csv", nil)
	overhead := len(envelope)

	fill := func(n int) []byte {
		body := make([]byte, 0, n)
		body = append(body, "Titel\n"...)
		return append(body, bytes.Repeat([]byte("a"), n-len(body))...)
	}

	atCap := uploadRequest(t, fields, "gross.csv", fill(csvMaxUpload-overhead))
	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVImport, atCap)
	if rec.Code != http.StatusSeeOther || flash != "" {
		t.Fatalf("exactly the cap: status %d, flash %q; want 303 and no error", rec.Code, flash)
	}
	if n := stagedCount(t, database); n != 1 {
		t.Fatalf("exactly the cap: staged rows = %d; want 1", n)
	}

	over := uploadRequest(t, fields, "gross.csv", fill(csvMaxUpload-overhead+1))
	rec, flash = serveAs(t, h, sm, admin, h.HandleCSVImport, over)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("one over the cap: status = %d; want 303", rec.Code)
	}
	if flash == "" {
		t.Error("one over the cap: no flash; the operator is told nothing")
	}
	if n := stagedCount(t, database); n != 1 {
		t.Errorf("one over the cap: staged rows = %d; want 1, the accepted one only", n)
	}
}

func TestCSVDegenerateFilesAreRefused(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	cases := []struct {
		name    string
		content []byte
	}{
		{"zero bytes", []byte{}},
		{"nothing but the byte-order mark", []byte{0xEF, 0xBB, 0xBF}},
		{"no header row", []byte("\n")},
	}

	seen := map[string]string{}
	for _, c := range cases {
		req := uploadRequest(t, map[string]string{"target": "neu"}, "x.csv", c.content)
		rec, flash := serveAs(t, h, sm, admin, h.HandleCSVImport, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status = %d; want 303", c.name, rec.Code)
		}
		if flash == "" {
			t.Errorf("%s: no message", c.name)
		}
		if other, clash := seen[flash]; clash {
			t.Errorf("%s and %s share one message %q; three refusals must be told apart", c.name, other, flash)
		}
		seen[flash] = c.name
		if n := stagedCount(t, database); n != 0 {
			t.Fatalf("%s: staged rows = %d; want 0", c.name, n)
		}
	}
}

// A header CELL that is empty is not a refusal — that column is offered by its
// position and stays pointable by hand (IMP-01 empty).
func TestCSVEmptyHeadingIsColumnN(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	token := stage(t, h, admin, 0, "Titel,Text,,Zustand\nErste,Ein Satz,Rest,entwurf\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Spalte 3") {
		t.Error("the column without a heading is not offered as „Spalte 3“")
	}
	if !strings.Contains(rec.Body.String(), `name="target_2"`) {
		t.Error("the column without a heading carries no select and cannot be pointed anywhere")
	}
}

func TestCSVForeignTokenIsNotFound(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	owner := seedAdmin(t, database, "eins@test")
	stranger := seedAdmin(t, database, "zwei@test")

	const secret = "Geheimzelle"
	token := stage(t, h, owner, 0, "Titel\n"+secret+"\n")

	rec, _ := serveAs(t, h, sm, stranger, h.HandleCSVMapping, mappingRequest(token, ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Error("the 404 body carries a cell of the other admin's file")
	}
}

// A token the sweep has already taken renders a named screen and NOT a 404: the
// operator did nothing wrong and a 404 would say they did (D-33).
func TestCSVSweptTokenShowsExpired(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest("gibtesnicht", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 — an expiry is not a refusal", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "abgelaufen") {
		t.Error("the expiry screen does not say the upload has expired")
	}
	if !strings.Contains(rec.Body.String(), "/admin/websites") {
		t.Error("the expiry screen offers no way back")
	}
}

func TestCSVMappingIsInColumnOrder(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	// Two definitions in an order that is not the file's, so a mapping list
	// built from the definitions rather than from the file would show.
	for i, label := range []string{"Sorte", "Preis"} {
		if _, err := h.fields.Create(context.Background(), field.Def{
			WebsiteID: ws.ID, Key: strings.ToLower(label), Label: label,
			Kind: field.KindText, AppliesTo: field.ForBoth, Position: i,
		}); err != nil {
			t.Fatalf("Create %s: %v", label, err)
		}
	}

	token := stage(t, h, admin, ws.ID, "Preis,Titel,Sorte\n9,Erste,Apfel\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := rec.Body.String()

	preis := strings.Index(body, `name="target_0"`)
	titel := strings.Index(body, `name="target_1"`)
	sorte := strings.Index(body, `name="target_2"`)
	if preis < 0 || titel < 0 || sorte < 0 {
		t.Fatalf("not every column is on the screen: %d %d %d", preis, titel, sorte)
	}
	if !(preis < titel && titel < sorte) {
		t.Errorf("the columns are not in the file's order: %d %d %d", preis, titel, sorte)
	}
}

// ?row= is a hand-typed number and is clamped, never refused (IMP-08).
func TestCSVSampleRowSteppingIsClamped(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	token := stage(t, h, admin, 0, "Titel\neins\nzwei\ndrei\nvier\n")

	cases := []struct {
		query    string
		cell     string
		wantPrev bool
		wantNext bool
	}{
		{"row=1", "eins", false, true},
		{"row=4", "vier", true, false},
		{"row=99", "vier", true, false},
		{"row=-4", "eins", false, true},
	}
	for _, c := range cases {
		rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, c.query))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d; want 200 — an out-of-range row is clamped, not refused", c.query, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, c.cell) {
			t.Errorf("%s: the sample row is not %q", c.query, c.cell)
		}
		// Absent and not disabled: a disabled control is still something a
		// keyboard user lands on. The control is a submit button of the
		// mapping form and no longer an anchor (WR-07), so what is looked for
		// is the control's own wording and not an href.
		if got := strings.Contains(body, "vorherige"); got != c.wantPrev {
			t.Errorf("%s: previous control present = %v; want %v", c.query, got, c.wantPrev)
		}
		if got := strings.Contains(body, "nächste"); got != c.wantNext {
			t.Errorf("%s: next control present = %v; want %v", c.query, got, c.wantNext)
		}
	}
}

// The two numbers of the sample line are both the spreadsheet's, and the test
// is here because a browser found the sentence reading "Zeile 13 von 12".
//
// SampleNumber is minted by csv.RowNumber, so it counts the header: the first
// data row is 2. The second number used to be TotalRows, which counts data rows
// only and never the header. On the last row of a file the two met and the
// screen said "row 13 of 12" — a sentence that is simply false about the file
// in front of the operator, and one the whole suite passed over because no test
// read the two numbers of that sentence together. D-26 puts every row number a
// person reads on the spreadsheet's own counting; this is the second number
// joining the first.
func TestCSVSampleLineCountsBothNumbersTheSameWay(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	// A header and four data rows: the spreadsheet's last row is row 5.
	token := stage(t, h, admin, 0, "Titel\neins\nzwei\ndrei\nvier\n")

	for _, c := range []struct {
		query string
		want  string
	}{
		{"row=1", "Beispiel: Zeile 2 von 5"},
		{"row=3", "Beispiel: Zeile 4 von 5"},
		{"row=4", "Beispiel: Zeile 5 von 5"},
	} {
		rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, c.query))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d; want 200", c.query, rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, c.want) {
			t.Errorf("%s: the sample line does not read %q — the row number and the total are counted differently", c.query, c.want)
		}
	}
	// And the sentence that started this: a row number above its own total.
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, "row=4"))
	if strings.Contains(rec.Body.String(), "Zeile 5 von 4") {
		t.Error(`the last row still reads "Zeile 5 von 4" — the total counts data rows while the row number counts the header`)
	}
}

func TestCSVHeaderOnlySaysSoInsteadOfAnEmptyTable(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	token := stage(t, h, admin, 0, "Titel,Text\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if !strings.Contains(body, "keine einzige Datenzeile") {
		t.Error("a file with no data rows does not say so")
	}
	if strings.Contains(body, "vorherige") || strings.Contains(body, "nächste") {
		t.Error("a file with no data rows still offers a row stepper")
	}
	if !strings.Contains(body, `name="target_0"`) {
		t.Error("the mapping is not settable on a header-only file")
	}
}

// A website with no field definitions of its own still has a usable mapping
// screen, and says why it is short (IMP-04 empty).
func TestCSVMappingIsNotEmptyWithoutOwnFields(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	token := stage(t, h, admin, ws.ID, "Titel,Text\nErste,Ein Satz\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := rec.Body.String()

	for _, want := range []string{`value="title"`, `value="slug"`, `value="body"`, `value="status"`, `value="terms"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the fixed target %s is missing from the select", want)
		}
	}
	if !strings.Contains(body, "keine eigenen Felder") {
		t.Error("a website without its own fields does not say so and the screen looks broken")
	}
}

// A required group refuses every row through field.CheckAll, correctly and
// invisibly. The screen has to say it before the dry run does.
func TestCSVMappingWarnsAboutARequiredGroup(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "zeiten", Label: "Öffnungszeiten",
		Kind: field.KindGroup, Required: true, AppliesTo: field.ForBoth,
	}); err != nil {
		t.Fatalf("Create group: %v", err)
	}

	token := stage(t, h, admin, ws.ID, "Titel\nErste\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := rec.Body.String()
	if !strings.Contains(body, "Öffnungszeiten") {
		t.Error("the required group is not named on the mapping screen")
	}
	if !strings.Contains(body, "jede einzelne Zeile dieser Datei abgewiesen") {
		t.Error("the mapping screen does not say that a required group refuses every row")
	}
}

// The form leads to the dry run, which is plan 09-05's route. Asserted on the
// rendered action rather than by a round trip, because that route does not
// exist yet.
func TestCSVMappingFormLeadsToTheDryRun(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	token := stage(t, h, admin, 0, "Titel\nErste\n")
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	want := `action="/admin/csv-import/` + token + `/probe"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("the mapping form does not post to %s", want)
	}
	if strings.Contains(rec.Body.String(), `type="file"`) {
		t.Error("screen 2 carries a file input; after screen 1 there is none anywhere (D-01)")
	}
}

// htmx is enhancement only. Every control on both new screens is a plain form
// or a plain link, so the wizard works with scripting switched off — and the
// same shapes are what internal/tmplmgr/script.go refuses in an uploaded theme.
func TestCSVTemplatesCarryNoScript(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)<script|javascript:|\son[a-z]+\s*=`)
	// All five files of the wizard, and the list grows with the wizard: a guard
	// naming two templates while the feature ships four is a guard over the two
	// screens that were easy. The two POST screens carry the CSRF token because
	// the middleware is a pass-through in main_test.go — no test would catch
	// its absence, so this grep stands in for one.
	needsToken := map[string]bool{"csv_mapping.html": true, "csv_dryrun.html": true}
	for _, name := range []string{
		"csv_mapping.html", "csv_expired.html", "csv_dryrun.html", "csv_report.html", "csv_reason.html",
	} {
		raw, err := os.ReadFile("../../cmd/holzcloud/templates/admin/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if m := forbidden.FindString(string(raw)); m != "" {
			t.Errorf("%s carries %q", name, m)
		}
		if needsToken[name] && !bytes.Contains(raw, []byte("gorilla.csrf.Token")) {
			t.Errorf("%s has a state-changing form without the CSRF token", name)
		}
	}
}

// TestCSVEveryReasonHasASentence (D-32, T-09-38): every Reason declared in Go
// has exactly one arm in the shared block.
//
// Counted as OCCURRENCES and not as lines: two {{else if}} arms sharing one
// line would make a line count agree while a reason fell through to the
// {{else}} — and the {{else}} is a neutral sentence, so the screen would look
// answered rather than broken.
func TestCSVEveryReasonHasASentence(t *testing.T) {
	block, err := os.ReadFile("../../cmd/holzcloud/templates/admin/csv_reason.html")
	if err != nil {
		t.Fatalf("read csv_reason.html: %v", err)
	}
	verdicts, err := os.ReadFile("../csvimport/verdict.go")
	if err != nil {
		t.Fatalf("read verdict.go: %v", err)
	}

	arms := regexp.MustCompile(`eq \.Reason "([a-z_]+)"`).FindAllStringSubmatch(string(block), -1)
	codes := regexp.MustCompile(`Reason = "([a-z_]+)"`).FindAllStringSubmatch(string(verdicts), -1)

	have := map[string]int{}
	for _, m := range arms {
		have[m[1]]++
	}
	for _, m := range codes {
		switch have[m[1]] {
		case 1:
		case 0:
			t.Errorf("the reason %q has no arm: it would render as an empty cell, which reads as „no reason“", m[1])
		default:
			t.Errorf("the reason %q has %d arms; the second is unreachable", m[1], have[m[1]])
		}
		delete(have, m[1])
	}
	for code := range have {
		t.Errorf("the block has an arm for %q, which nothing in Go produces", code)
	}
	if len(codes) == 0 {
		t.Fatal("no reason codes were found at all — the pattern no longer matches verdict.go")
	}
}

// TestCSVScreensUseOnlyClassesThatExist: the failure .import-summary and
// .import-warnings are standing examples of — a hook in a template with no rule
// anywhere behind it, which looks like styling and is not.
func TestCSVScreensUseOnlyClassesThatExist(t *testing.T) {
	css, err := os.ReadFile("../../cmd/holzcloud/assets/admin.css")
	if err != nil {
		t.Fatalf("read admin.css: %v", err)
	}
	literal := regexp.MustCompile(`class="([^"{}]*)"`)

	for _, name := range []string{"csv_dryrun.html", "csv_report.html", "csv_reason.html"} {
		raw, err := os.ReadFile("../../cmd/holzcloud/templates/admin/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range literal.FindAllStringSubmatch(string(raw), -1) {
			for _, class := range strings.Fields(m[1]) {
				if !regexp.MustCompile(`\.` + regexp.QuoteMeta(class) + `[^a-zA-Z0-9_-]`).Match(css) {
					t.Errorf("%s uses .%s, which has no rule in admin.css", name, class)
				}
			}
		}
	}
}

// exampleRequest builds the GET the small second form in the panel sends.
func exampleRequest(websiteID string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/admin/csv-vorlage?website="+websiteID, nil)
}

func TestCSVExampleHasHeaderAndBOM(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "sorte", Label: "Sorte",
		Kind: field.KindMulti, Choices: []string{"rot", "blau"},
		AppliesTo: field.ForBoth,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "vorraetig", Label: "Vorrätig",
		Kind: field.KindBool, AppliesTo: field.ForBoth, Position: 1,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(fmt.Sprintf("%d", ws.ID)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q; a CSV served without it is one a browser may try to render", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q; the example is a snapshot and a cached copy is a snapshot of another moment", got)
	}

	body := rec.Body.Bytes()
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("the body does not begin with the byte-order mark Excel needs")
	}
	text := string(body)
	for _, want := range []string{"Titel", "Adresse", "Text", "Zustand", "Schlagwörter", "Sorte", "Vorrätig"} {
		if !strings.Contains(text, want) {
			t.Errorf("the header row has no %q column", want)
		}
	}
	// D-21's pipe and D-19's vocabulary, in the file where an operator meets
	// them.
	if !strings.Contains(text, "rot|blau") {
		t.Error("the mehrfachauswahl sample does not show the pipe")
	}
	if !strings.Contains(text, "ja") {
		t.Error("the janein sample does not show a value of the closed vocabulary")
	}
}

// D-18: four kinds a cell can never fill get no column at all, because a
// heading the dry run then refuses is a promise broken one screen later.
func TestCSVExampleSkipsUnmappableKinds(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	for i, d := range []field.Def{
		{Key: "bild", Label: "Titelbild", Kind: field.KindImage},
		{Key: "verweis", Label: "Verwandte Seite", Kind: field.KindRef},
		{Key: "zeiten", Label: "Öffnungszeiten", Kind: field.KindGroup},
		{Key: "trenner", Label: "Zwischenüberschrift", Kind: field.KindSection},
		{Key: "preis", Label: "Preis", Kind: field.KindNumber},
	} {
		d.WebsiteID, d.AppliesTo, d.Position = ws.ID, field.ForBoth, i
		if _, err := h.fields.Create(context.Background(), d); err != nil {
			t.Fatalf("Create %s: %v", d.Label, err)
		}
	}

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(fmt.Sprintf("%d", ws.ID)))
	text := rec.Body.String()
	for _, unwanted := range []string{"Titelbild", "Verwandte Seite", "Öffnungszeiten", "Zwischenüberschrift"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("the example offers a column for %q, which no cell can fill", unwanted)
		}
	}
	if !strings.Contains(text, "Preis") {
		t.Error("the example is missing the one mappable definition")
	}
}

// D-16: a label beginning with = is a formula the moment the file opens.
func TestCSVExampleDefusesFormulas(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "preis", Label: "=Preis",
		Kind: field.KindText, AppliesTo: field.ForBoth,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(fmt.Sprintf("%d", ws.ID)))
	text := rec.Body.String()
	if !strings.Contains(text, "'=Preis") {
		t.Errorf("the label =Preis is not defused; body was %q", text)
	}
}

func TestCSVExampleFilenameComesFromTheWebsite(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	ws, err := h.domains.CreateWebsite(context.Background(), "Velowerkstatt Bärn", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(fmt.Sprintf("%d", ws.ID)))
	got := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(got, "attachment; ") {
		t.Errorf("Content-Disposition = %q; want an attachment, not inline", got)
	}
	if !strings.Contains(got, "-vorlage.csv") || !strings.Contains(got, "velowerkstatt") {
		t.Errorf("Content-Disposition = %q; the name does not come from the website", got)
	}
}

// Not a token: this route is not token-bearing (D-37). The refusal is the
// ordinary website one — for a website that was NAMED and is not there. A
// website not named at all is a different question and is answered by
// TestCSVExampleWithoutAWebsiteIsTheFixedColumns below.
func TestCSVExampleUnknownWebsiteIsNotFound(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest("9999"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("website=9999: status = %d; want 404", rec.Code)
	}
}

// TestCSVExampleWithoutAWebsiteIsTheFixedColumns (WR-08): the file the panel
// promises in words for a website that does not exist yet.
//
// D-37 states the requirement and the answer, and the panel prints it: "Für
// eine Website, die es noch nicht gibt, gibt es keine Felder zu lesen: dort
// sind es nur die festen Spalten." There was no control that produced it and
// the handler accepted no request without an existing website — ParseInt("")
// yields 0, GetWebsite(0) yields nil, and the answer was http.NotFound. On a
// fresh installation, which is the single most likely moment for a first CSV
// import, the select was empty, the button submitted website= and the operator
// got a bare 404 from the download they had just been told to use.
func TestCSVExampleWithoutAWebsiteIsTheFixedColumns(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	// A field on the existing website, so a file that reached the definitions
	// would show it and this test would notice.
	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "sorte", Label: "Sorte", Kind: field.KindText,
	}); err != nil {
		t.Fatalf("create field: %v", err)
	}

	// Absent, empty and an explicit zero all mean the same thing: no website
	// was named. The panel's second form sends the first of the three.
	for _, id := range []string{"", "0", "nichts"} {
		rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(id))
		if rec.Code != http.StatusOK {
			t.Fatalf("website=%q: status = %d; want 200 — this is the new-website case, not a refusal", id, rec.Code)
		}
		body := rec.Body.String()
		for _, column := range []string{"Titel", "Adresse", "Text", "Zustand"} {
			if !strings.Contains(body, column) {
				t.Errorf("website=%q: the fixed column %q is missing:\n%s", id, column, body)
			}
		}
		if strings.Contains(body, "Sorte") {
			t.Errorf("website=%q: the file carries a field of an existing website", id)
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="website-vorlage.csv"`) {
			t.Errorf("website=%q: Content-Disposition = %q, want website-vorlage.csv", id, got)
		}
	}
}

// TestWebsiteListOffersTheNewWebsiteExample (WR-08): the control exists, and it
// exists on an installation with no websites at all.
func TestWebsiteListOffersTheNewWebsiteExample(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	rec, _ := serveAs(t, h, sm, admin, h.HandleWebsiteList,
		httptest.NewRequest(http.MethodGet, "/admin/websites", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if !strings.Contains(body, "Beispieldatei für eine neue Website") {
		t.Errorf("the panel offers no way to get the fixed-column file:\n%s", body)
	}
	// Its own form, carrying no website field: a second button named "website"
	// inside the first form would lose to the select, which is serialised
	// earlier and therefore read first.
	if n := strings.Count(body, `action="/admin/csv-vorlage"`); n != 2 {
		t.Errorf("%d forms point at the example route, want 2", n)
	}
}

// IMP-07 concurrency: the example is a snapshot and is never authoritative. A
// field added after it was downloaded simply arrives unmapped, and the mapping
// screen is what shows that.
func TestCSVExampleIsASnapshotAndALaterFieldArrivesUnmapped(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(fmt.Sprintf("%d", ws.ID)))
	downloaded := rec.Body.String()
	if strings.Contains(downloaded, "Sorte") {
		t.Fatal("the example already knows a field that does not exist yet")
	}

	if _, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "sorte", Label: "Sorte",
		Kind: field.KindText, AppliesTo: field.ForBoth,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	token := stage(t, h, admin, ws.ID, "Titel,Adresse,Text,Zustand,Schlagwörter\nErste,erste,Ein Satz,entwurf,Apfel\n")
	screen, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := screen.Body.String()
	if !strings.Contains(body, `value="field:sorte"`) {
		t.Error("the field added after the download is not offered on the mapping screen")
	}
	if !strings.Contains(body, "Sorte") {
		t.Error("the mapping screen does not name the new field")
	}
}

// ── The dry run and the write ─────────────────────────────────────────────
//
// What the tests below guard, and why each one exists rather than being an
// argument in a comment:
//
// The dry run writes nothing. Asserted by counting the pages table, not by
// trusting the flag that says so.
//
// The dry run and the write reach the same verdict for every row of one file.
// Two functions deciding the same thing drift, and the drifted one is always
// the one the operator was not shown — this is the only test that would catch
// it (D-22).
//
// The compensation in WriteRow runs on the CREATE path only. On the update arm
// a later failure is reported and NOTHING is undone, because a page that was
// already there when the import started is not this import's to delete. A
// missing branch is invisible to a reviewer, so it gets a test of its own in
// internal/csvimport/row_test.go.

// csvPost builds one of the two POSTs of screens 3 and 4, with the path value
// the mux would set.
func csvPost(token, screen string, values url.Values) *http.Request {
	return postForm("/admin/csv-import/"+token+"/"+screen, values, map[string]string{"token": token})
}

// csvTargets is the mapping a screen-2 form would post: one target per column,
// left to right.
func csvTargets(targets ...string) url.Values {
	v := url.Values{}
	for i, target := range targets {
		v.Set("target_"+strconv.Itoa(i), target)
	}
	return v
}

// pageCount is the whole pages table, which is what IMP-05 is about: the dry
// run may not add a row anywhere.
func pageCount(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM pages`).Scan(&n); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	return n
}

// slugsInOrder is every live page of a website, oldest row first — which is the
// order they were written in.
func slugsInOrder(t *testing.T, database *db.DB, websiteID int64) []string {
	t.Helper()
	rows, err := database.Read.QueryContext(context.Background(),
		`SELECT slug FROM pages WHERE website_id = $1 AND deleted_at IS NULL ORDER BY id`, websiteID)
	if err != nil {
		t.Fatalf("list slugs: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatalf("scan slug: %v", err)
		}
		out = append(out, slug)
	}
	return out
}

// TestCSVProbeWritesNothing (IMP-05): the whole file goes through validation and
// the pages table is untouched afterwards.
func TestCSVProbeWritesNothing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel,Text\nErste,Ein Satz\nZweite,Noch einer\n,Ohne Titel\n")

	before := pageCount(t, database)
	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVDryRun,
		csvPost(token, "probe", csvTargets("title", "body")))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (flash %q); want 200", rec.Code, flash)
	}
	if after := pageCount(t, database); after != before {
		t.Errorf("pages went from %d to %d; the dry run must write nothing", before, after)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Es wurde nichts geschrieben") {
		t.Error("the dry run does not say that nothing has been written")
	}
	if !strings.Contains(body, "keinen Titel") {
		t.Error("the row without a title is not reported by its reason")
	}
	// And the staging row survives: the operator has not committed yet.
	if n := stagedCount(t, database); n != 1 {
		t.Errorf("staged rows = %d after the dry run; want 1", n)
	}
}

// TestCSVProbeAndStartAgreeOnEveryVerdict (D-22): the one test that would catch
// the dry run and the write drifting apart. Both are run over one file and the
// verdicts are compared row by row.
func TestCSVProbeAndStartAgreeOnEveryVerdict(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	ctx := context.Background()

	file := "Titel,Text,Zustand\n" +
		"Erste,Ein Satz,entwurf\n" +
		",Ohne Titel,entwurf\n" +
		"Dritte,Noch einer,violett\n" +
		"Vierte,Und noch einer,veröffentlicht\n"
	token := stage(t, h, admin, ws.ID, file)

	upload, err := h.csvImports.Get(ctx, token, admin)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defs, err := h.fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatalf("List fields: %v", err)
	}
	m := csvMappingFromForm(csvPost(token, "probe", csvTargets("title", "body", "status")), 3)

	dry, _, err := h.csvRun(ctx, upload, m, defs, ws.ID, false, nil)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if n := pageCount(t, database); n != 0 {
		t.Fatalf("the dry run wrote %d pages", n)
	}

	written, _, err := h.csvRun(ctx, upload, m, defs, ws.ID, true, &admin)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if len(dry) != len(written) {
		t.Fatalf("the dry run decided %d rows and the write %d", len(dry), len(written))
	}
	for i := range dry {
		if dry[i].Row != written[i].Row || dry[i].Outcome != written[i].Outcome || dry[i].Reason != written[i].Reason {
			t.Errorf("row %d: dry run said %+v, the write said %+v", dry[i].Row, dry[i], written[i])
		}
	}
	// And the file really did contain all three cases, so the comparison above
	// is not four identical successes agreeing with each other.
	if dry[1].Reason != csvimport.ReasonNoTitle || dry[2].Reason != csvimport.ReasonStatusUnknown {
		t.Fatalf("the fixture no longer carries the refusals it is meant to: %+v", dry)
	}
}

// TestCSVStartCreatesInFileOrder (IMP-04 ordering).
func TestCSVStartCreatesInFileOrder(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel\nGamma\nAlpha\nBeta\n")

	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(token, "start", csvTargets("title")))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (flash %q); want 200", rec.Code, flash)
	}

	got := slugsInOrder(t, database, ws.ID)
	want := []string{"gamma", "alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pages were written as %v; want the file's own order %v", got, want)
	}
	if !strings.Contains(rec.Body.String(), "Einlesen abgeschlossen") {
		t.Error("the report screen did not render")
	}
}

// TestCSVReportNamesEveryRename (D-23, criterion 4): a rename is a success the
// operator still has to be told about, and the report names both addresses.
//
// Driven through the template rather than through the handler, and the reason
// is a measurement rather than convenience: WriteRow renames only when it is
// handed existing == nil while the address is in fact taken, and csvRun looks
// that page up per row, so at the handler the second import of one file SKIPS
// or UPDATES — it does not rename. The rename is the answer to a genuine race
// between the lookup and the INSERT, which internal/csvimport/row_test.go's
// TestRenamedSlugIsReported drives at the writer. What is left to guard here is
// that the report has a sentence for it, and that is this.
func TestCSVReportNamesEveryRename(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	report := csvimport.Summarize([]csvimport.Verdict{
		{Row: 2, Outcome: csvimport.OutcomeCreate},
		{Row: 3, Outcome: csvimport.OutcomeCreate, Reason: csvimport.ReasonRenamed,
			Args: []string{"alpha", "alpha-2"}},
	})

	rec, _ := serveAs(t, h, sm, admin, func(w http.ResponseWriter, r *http.Request) error {
		return web.RenderAdmin(w, h.templates, r, "csv_report", CSVReportData{
			LayoutData:  web.NewLayoutData(r, h.sm, "Einlesen abgeschlossen"),
			WebsiteID:   ws.ID,
			WebsiteName: ws.Name,
			Filename:    "tabelle.csv",
			Report:      report,
		})
	}, httptest.NewRequest(http.MethodGet, "/admin/csv-import/abc/start", nil))

	body := rec.Body.String()
	for _, want := range []string{"alpha", "alpha-2", "war schon vergeben", "1 Adressen"} {
		if !strings.Contains(body, want) {
			t.Errorf("the report does not carry %q:\n%s", want, body)
		}
	}
}

// TestCSVSecondImportWithUpdateIsIdempotent (IMP-02 idempotency): the same file
// twice with "aktualisieren" rewrites the same values and creates nothing.
func TestCSVSecondImportWithUpdateIsIdempotent(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	file := "Titel,Text\nAlpha,Ein Satz\nBeta,Noch einer\n"

	first := stageWith(t, h, admin, ws.ID, file, csvimport.CollisionUpdate)
	serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(first, "start", csvTargets("title", "body")))
	if n := pageCount(t, database); n != 2 {
		t.Fatalf("first run created %d pages; want 2", n)
	}

	second := stageWith(t, h, admin, ws.ID, file, csvimport.CollisionUpdate)
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(second, "start", csvTargets("title", "body")))

	if n := pageCount(t, database); n != 2 {
		t.Errorf("second run left %d pages; want 2 — an update is idempotent", n)
	}
	if body := rec.Body.String(); !strings.Contains(body, "2 aktualisiert") {
		t.Errorf("the second run's report does not say both rows were updated:\n%s", body)
	}
}

// TestCSVSecondImportWithSkipLeavesThePageAlone (IMP-02 idempotency, the other
// arm): the second run reports every row as skipped and touches nothing.
func TestCSVSecondImportWithSkipLeavesThePageAlone(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	file := "Titel,Text\nAlpha,Ein Satz\n"

	first := stage(t, h, admin, ws.ID, file)
	serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(first, "start", csvTargets("title", "body")))

	second := stage(t, h, admin, ws.ID, file)
	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(second, "start", csvTargets("title", "body")))

	if n := pageCount(t, database); n != 1 {
		t.Errorf("pages = %d; want 1", n)
	}
	if body := rec.Body.String(); !strings.Contains(body, "gibt es die Seite schon") {
		t.Errorf("the skipped row is not reported by its reason:\n%s", body)
	}
}

// TestCSVDeletedTargetWebsiteEndsTheWizard (IMP-04 concurrency): a website
// deleted between two screens ends the wizard with a message, not a panic.
func TestCSVDeletedTargetWebsiteEndsTheWizard(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel\nAlpha\n")

	if err := h.domains.DeleteWebsite(context.Background(), ws.ID); err != nil {
		t.Fatalf("DeleteWebsite: %v", err)
	}

	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVDryRun,
		csvPost(token, "probe", csvTargets("title")))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want 303", rec.Code)
	}
	if !strings.Contains(flash, "gibt es nicht mehr") {
		t.Errorf("flash = %q; the operator is not told what happened", flash)
	}
	if n := stagedCount(t, database); n != 0 {
		t.Errorf("staged rows = %d; the abandoned upload was not cleared up", n)
	}
}

// TestCSVDeletedFieldDefinitionIsReported (IMP-04 concurrency, D-29): the column
// falls back to unmapped, the row says so, and the rest of the row still
// imports.
func TestCSVDeletedFieldDefinitionIsReported(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	def, err := h.fields.Create(context.Background(), field.Def{
		WebsiteID: ws.ID, Key: "sorte", Label: "Sorte",
		Kind: field.KindText, AppliesTo: field.ForBoth,
	})
	if err != nil {
		t.Fatalf("Create field: %v", err)
	}
	token := stage(t, h, admin, ws.ID, "Titel,Sorte\nAlpha,Boskoop\n")

	if err := h.fields.Delete(context.Background(), ws.ID, def.ID); err != nil {
		t.Fatalf("Delete field: %v", err)
	}

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVStart,
		csvPost(token, "start", csvTargets("title", "field:sorte")))

	if body := rec.Body.String(); !strings.Contains(body, "gibt es nicht mehr") {
		t.Errorf("the deleted definition is not reported:\n%s", body)
	}
	if got := slugsInOrder(t, database, ws.ID); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Errorf("pages = %v; the rest of the row must still import", got)
	}
}

// TestCSVReloadDoesNotImportTwice (D-33): the staging row goes after the loop
// and before the render, so a refreshed POST lands on the expiry screen.
func TestCSVReloadDoesNotImportTwice(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel\nAlpha\nBeta\n")

	serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(token, "start", csvTargets("title")))
	if n := pageCount(t, database); n != 2 {
		t.Fatalf("first run created %d pages; want 2", n)
	}
	if n := stagedCount(t, database); n != 0 {
		t.Fatalf("staged rows = %d after the write; want 0", n)
	}

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(token, "start", csvTargets("title")))
	if n := pageCount(t, database); n != 2 {
		t.Errorf("the refresh imported the file again: %d pages", n)
	}
	if body := rec.Body.String(); !strings.Contains(body, "abgelaufen") {
		t.Errorf("the refresh did not land on the expiry screen:\n%s", body)
	}
}

// TestCSVNewWebsiteAppearsOnlyAtStart: an abandoned wizard leaves no empty
// website behind, and the dry run is still part of the abandoning.
func TestCSVNewWebsiteAppearsOnlyAtStart(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	ctx := context.Background()

	token := stage(t, h, admin, 0, "Titel\nAlpha\n")

	before, err := h.domains.ListWebsites(ctx)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	serveAs(t, h, sm, admin, h.HandleCSVDryRun, csvPost(token, "probe", csvTargets("title")))
	during, err := h.domains.ListWebsites(ctx)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(during) != len(before) {
		t.Errorf("the dry run created a website: %d, was %d", len(during), len(before))
	}

	serveAs(t, h, sm, admin, h.HandleCSVStart, csvPost(token, "start", csvTargets("title")))
	after, err := h.domains.ListWebsites(ctx)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("websites = %d; want %d — the write creates it", len(after), len(before)+1)
	}
}

// TestCSVTermsEnsuredOnceBeforeTheLoop (D-20): a file of 300 rows carrying four
// distinct labels leaves four labels, not twelve hundred and not four per row.
func TestCSVTermsEnsuredOnceBeforeTheLoop(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	var file strings.Builder
	file.WriteString("Titel,Schlagwörter\n")
	names := []string{"Apfel", "Birne", "Kirsche", "Zwetschge"}
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&file, "Seite %d,%s|%s\n", i, names[i%4], names[(i+1)%4])
	}
	token := stage(t, h, admin, ws.ID, file.String())

	rec, flash := serveAs(t, h, sm, admin, h.HandleCSVStart,
		csvPost(token, "start", csvTargets("title", "terms")))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (flash %q); want 200", rec.Code, flash)
	}

	var labels int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM terms WHERE website_id = $1`, ws.ID).Scan(&labels); err != nil {
		t.Fatalf("count terms: %v", err)
	}
	if labels != 4 {
		t.Errorf("terms = %d; want 4 — the names are ensured once for the whole file", labels)
	}
	if n := pageCount(t, database); n != 300 {
		t.Errorf("pages = %d; want 300", n)
	}
}

// TestCSVMixedFileImportsTheGoodRows (criterion 4): the good rows go in, the
// rest are skipped with a named reason, and nothing is half written.
func TestCSVMixedFileImportsTheGoodRows(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID,
		"Titel,Zustand\nAlpha,entwurf\n,entwurf\nGamma,violett\nDelta,veröffentlicht\n")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVStart,
		csvPost(token, "start", csvTargets("title", "status")))

	if got := slugsInOrder(t, database, ws.ID); !reflect.DeepEqual(got, []string{"alpha", "delta"}) {
		t.Errorf("pages = %v; want alpha and delta", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "2 angelegt") || !strings.Contains(body, "2 übergangen") {
		t.Errorf("the counters do not say 2 and 2:\n%s", body)
	}
	if !strings.Contains(body, "keinen Titel") || !strings.Contains(body, "weder ein Entwurf") {
		t.Error("the two refusals are not both named on the report")
	}
}

// TestCSVMappingWithoutATitleIsAFormError: a mapping the operator got wrong
// comes back as the mapping screen at 422 with their choices intact, never as a
// flash and a redirect that throws the whole mapping away.
func TestCSVMappingWithoutATitleIsAFormError(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel,Text\nAlpha,Ein Satz\n")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVDryRun,
		csvPost(token, "probe", csvTargets("none", "body")))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d; want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Keine Spalte zeigt auf den Titel") {
		t.Error("the screen does not say what is wrong with the mapping")
	}
	// Their choices are still there: the second column is still on the body.
	if !strings.Contains(body, `<option value="body" selected>`) {
		t.Errorf("the mapping was thrown away:\n%s", body)
	}
	if n := pageCount(t, database); n != 0 {
		t.Errorf("a refused mapping wrote %d pages", n)
	}
}

// TestCSVReportIsGroupedNotListed (D-25): forty rows failing for one reason are
// one line with forty row numbers on it, not forty lines.
func TestCSVReportIsGroupedNotListed(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	var file strings.Builder
	file.WriteString("Titel,Zustand\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&file, "Seite %d,violett\n", i)
	}
	token := stage(t, h, admin, ws.ID, file.String())

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVDryRun,
		csvPost(token, "probe", csvTargets("title", "status")))

	body := rec.Body.String()
	if n := strings.Count(body, "weder ein Entwurf"); n != 1 {
		t.Errorf("the reason is printed %d times; want once — that is the whole of D-25", n)
	}
	if !strings.Contains(body, "40 Zeilen") {
		t.Error("the group does not carry its own count")
	}
	if !strings.Contains(body, "und 15 weitere") {
		t.Errorf("the row list is not capped at 25 with the rest counted:\n%s", body)
	}
}

// TestCSVStoreErrorReachesTheReport (WR-03): the two reasons that carry raw Go
// error text really do print it, which is what their comments now say.
//
// The defect this pins was a comment claiming a guarantee the code does not
// keep: "the second argument is a Go error meant for the log; the screen shows
// the title and the code's own sentence." The screen shows the error. Either
// answer is defensible — this one is chosen, because an operator who cannot see
// what the store said cannot tell a duplicate address from a full disk — but
// the two have to agree, and the next person adding a reason will believe the
// comment. So the arms are asserted, and a later change that drops the argument
// has to change the comment in the same commit.
func TestCSVStoreErrorReachesTheReport(t *testing.T) {
	block, err := os.ReadFile("../../cmd/holzcloud/templates/admin/csv_reason.html")
	if err != nil {
		t.Fatalf("read csv_reason.html: %v", err)
	}
	for _, reason := range []string{"not_written", "not_rolled_back"} {
		arm := regexp.MustCompile(`eq \.Reason "` + reason + `"}}[^\n]*`).FindString(string(block))
		if arm == "" {
			t.Errorf("no arm for %q at all", reason)
			continue
		}
		if !strings.Contains(arm, "index .Args 1") {
			t.Errorf("the %q arm no longer prints what the store said: %s\n"+
				"if that is deliberate, verdict.go's comment has to stop saying the report prints it", reason, arm)
		}
	}
}

// TestCSVTwoOverlappingCommitsImportOnce (WR-05): a double-click does not
// create two websites carrying the same file.
//
// Reachable by an ordinary double-click. hx-disabled-elt="this" on the commit
// button does not help: base.html carries hx-headers only, there is no
// hx-boost, and the form has no hx-post, so htmx never processes the submit and
// never disables the button. The staging row was deleted after the loop, so
// both requests passed staged(), both reached the loop and both reached
// CreateWebsite.
//
// The write pool admits one connection, so the two claims serialise there and
// exactly one DELETE affects a row.
func TestCSVTwoOverlappingCommitsImportOnce(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	ctx := context.Background()

	before, err := h.domains.ListWebsites(ctx)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}

	token := stage(t, h, admin, 0, "Titel\nAlpha\nBeta\n")

	// Both requests are built before either is served and released together,
	// so the window they used to share is the window under test. serveAs is
	// not used: it calls t.Fatalf, which may only be called from the test's own
	// goroutine, so the handler errors are collected and reported below.
	var start, done sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	start.Add(1)
	for i := 0; i < 2; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			req := csvPost(token, "start", csvTargets("title"))
			start.Wait()
			sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sm.Put(r.Context(), auth.SessionKeyUserID, admin)
				if err := h.HandleCSVStart(w, r); err != nil {
					mu.Lock()
					failures = append(failures, err)
					mu.Unlock()
				}
			})).ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	start.Done()
	done.Wait()
	for _, err := range failures {
		t.Fatalf("handler: %v", err)
	}

	after, err := h.domains.ListWebsites(ctx)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Errorf("websites = %d, want %d — one commit, one website, however many requests arrived",
			len(after), len(before)+1)
	}
	if n := pageCount(t, database); n != 2 {
		t.Errorf("%d pages, want 2 — the file was imported more than once", n)
	}
	if n := stagedCount(t, database); n != 0 {
		t.Errorf("staged rows = %d after the commit, want 0", n)
	}
}

// TestCSVMappingCutsAnOversizedSample (WR-06): the mapping screen does not
// redraw a cell the reader has already refused, at full size.
//
// csv.MaxCellBytes is reported and never applied — the reader leaves the cell
// whole so the report can name its column and its size instead of showing the
// value. Screen 2 did the opposite: it copied the cell into the view with no
// bound and the template printed it. A first row carrying a 9 MB cell, legal
// inside the 10 MB upload cap, made a 9 MB HTML response, and the stepper drew
// it again on every visit. The cut belongs at the view boundary, which is the
// consumer that has to draw the thing.
func TestCSVMappingCutsAnOversizedSample(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	huge := strings.Repeat("a", csvSampleBytes*50)
	token := stage(t, h, admin, ws.ID, "Titel,Text\nAlpha,"+huge+"\n")

	rec, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := rec.Body.String()
	if strings.Contains(body, huge) {
		t.Error("the whole oversized cell was written into the response")
	}
	if len(body) > len(huge) {
		t.Errorf("the response is %d bytes for a %d-byte cell — the sample is not bounded", len(body), len(huge))
	}

	// And the cut lands on a rune boundary: a cell of multi-byte characters
	// must not put a replacement glyph on the screen where the operator is
	// trying to recognise their own data.
	multi := strings.Repeat("ä", csvSampleBytes)
	cut := csvSample(multi)
	if !utf8.ValidString(cut) {
		t.Errorf("csvSample cut through a character: %q", cut)
	}
	if len(cut) > csvSampleBytes+len("…") {
		t.Errorf("csvSample returned %d bytes, want at most %d", len(cut), csvSampleBytes+len("…"))
	}
	if !strings.HasSuffix(cut, "…") {
		t.Error("a cut cell does not say it was cut, so it reads as a short cell")
	}

	// A cell that fits is handed through untouched, ellipsis and all.
	if got := csvSample("Alpha"); got != "Alpha" {
		t.Errorf("csvSample(%q) = %q, want it unchanged", "Alpha", got)
	}
}

// TestCSVSteppingKeepsTheMapping (WR-07): stepping the sample row carries the
// operator's mapping with it.
//
// The stepper was two plain anchors, inside the form but not part of it.
// Following one was a fresh GET, HandleCSVMapping passed chosen = nil, and the
// screen came back with the automatic match: every select the operator had
// changed and every default they had typed was gone, with nothing on screen
// saying so. On a file with any real number of columns the navigation cost them
// their work — and IMP-08's promise is a sample row navigable to the next.
//
// 09-02-PLAN's recorded decision settles the shape: one form, and the stepper
// is a submit button inside it with formmethod="GET", so the button sends the
// same form's fields to screen 2 as a query string.
func TestCSVSteppingKeepsTheMapping(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")
	token := stage(t, h, admin, ws.ID, "Titel,Sorte\nAlpha,Boskoop\nBeta,Gravensteiner\n")

	// The screen as it arrives from screen 1: the stepper is a submit of the
	// mapping form, not a link, or nothing could carry the mapping at all.
	first, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping, mappingRequest(token, ""))
	body := first.Body.String()
	if strings.Contains(body, `href="?row=`) {
		t.Error("the stepper is still a plain anchor: following one is a fresh GET and the mapping is lost")
	}
	if !strings.Contains(body, `formmethod="GET"`) || !strings.Contains(body, `name="row" value="2"`) {
		t.Errorf("no stepping submit for row 2 on the screen:\n%s", body)
	}

	// The operator points column 2 at the title instead and types a default,
	// then steps. The button submits the whole form, so both arrive as query
	// values — which is what the stepping request below is.
	stepped, _ := serveAs(t, h, sm, admin, h.HandleCSVMapping,
		mappingRequest(token, "row=2&target_0=none&target_1=title&default_status=entwurf"))
	body = stepped.Body.String()

	if !strings.Contains(body, `<option value="title" selected>`) {
		t.Error("the operator's own choice did not survive the step")
	}
	if !regexp.MustCompile(`id="target_0"[^>]*>\s*<option value="none" selected`).MatchString(body) {
		t.Error("column 1 came back with the automatic match instead of the operator's „nothing“")
	}
	if !strings.Contains(body, `value="entwurf"`) {
		t.Error("the default the operator typed was thrown away by the step")
	}
	// And the step really did step: the sample row on screen is the second.
	if !strings.Contains(body, "Gravensteiner") {
		t.Error("the sample row did not advance")
	}
}
