package admin

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/field"
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
	upload := csvimport.Upload{
		UserID:      userID,
		WebsiteID:   websiteID,
		WebsiteName: "Testseite",
		Mode:        csvModeNew,
		Collision:   csvimport.CollisionSkip,
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
	if !strings.Contains(rec.Body.String(), `name="ziel_2"`) {
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

	preis := strings.Index(body, `name="ziel_0"`)
	titel := strings.Index(body, `name="ziel_1"`)
	sorte := strings.Index(body, `name="ziel_2"`)
	if preis < 0 || titel < 0 || sorte < 0 {
		t.Fatalf("not every column is on the screen: %d %d %d", preis, titel, sorte)
	}
	if !(preis < titel && titel < sorte) {
		t.Errorf("the columns are not in the file's order: %d %d %d", preis, titel, sorte)
	}
}

// ?zeile= is a hand-typed number and is clamped, never refused (IMP-08).
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
		{"zeile=1", "eins", false, true},
		{"zeile=4", "vier", true, false},
		{"zeile=99", "vier", true, false},
		{"zeile=-4", "eins", false, true},
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
		// Absent and not disabled: a disabled link is still something a
		// keyboard user lands on.
		if got := strings.Contains(body, `href="?zeile=`) && strings.Contains(body, "vorherige"); got != c.wantPrev {
			t.Errorf("%s: previous control present = %v; want %v", c.query, got, c.wantPrev)
		}
		if got := strings.Contains(body, "nächste"); got != c.wantNext {
			t.Errorf("%s: next control present = %v; want %v", c.query, got, c.wantNext)
		}
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
	if !strings.Contains(body, `name="ziel_0"`) {
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
	for _, name := range []string{"csv_mapping.html", "csv_expired.html"} {
		raw, err := os.ReadFile("../../cmd/holzcloud/templates/admin/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if m := forbidden.FindString(string(raw)); m != "" {
			t.Errorf("%s carries %q", name, m)
		}
		if !bytes.Contains(raw, []byte("gorilla.csrf.Token")) && name == "csv_mapping.html" {
			t.Errorf("%s has a state-changing form without the CSRF token", name)
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
// ordinary website one.
func TestCSVExampleUnknownWebsiteIsNotFound(t *testing.T) {
	h, sm, database, _ := newTestAdmin(t)
	admin := seedAdmin(t, database, "eins@test")

	for _, id := range []string{"9999", "", "nichts"} {
		rec, _ := serveAs(t, h, sm, admin, h.HandleCSVExample, exampleRequest(id))
		if rec.Code != http.StatusNotFound {
			t.Errorf("website=%q: status = %d; want 404", id, rec.Code)
		}
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
