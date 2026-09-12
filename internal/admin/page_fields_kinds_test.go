package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// The three small kinds in the page editor: zeit, bereich and code.
//
// What is checked is the delivered HTML and not the view model. What a
// FieldView carries is of no use if the branch in field_input.html is missing —
// and a missing branch is not noticed: the chain ends in an ordinary text field
// that takes a time of day just as uncomplainingly.
func TestFieldKindsInThePageEditor(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	create := func(d field.Def) {
		t.Helper()
		d.WebsiteID = ws.ID
		if _, err := fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
	}

	create(field.Def{Key: "abfahrt", Label: "Abfahrt", Kind: field.KindTime})
	// With bounds — and with a dependent field on it, so that .Switch is "text"
	// and the placeholder branch is taken at all. Without it a range field can
	// never show and hide its dependants.
	create(field.Def{Key: "menge", Label: "Menge", Kind: field.KindRange,
		RangeMin: "1", RangeMax: "9"})
	create(field.Def{Key: "hinweis", Label: "Hinweis", Kind: field.KindText,
		Condition: "menge"})
	// And one without bounds: open at the top and at the bottom is a valid
	// statement, and then no empty min="" may stand in the form.
	create(field.Def{Key: "offen", Label: "Offen", Kind: field.KindRange})
	create(field.Def{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode})

	p := seedPage(t, database, ws.ID, "Fahrplan", "fahrplan", "text", "draft")
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	body := rec.Body.String()

	// --- zeit ---------------------------------------------------------------
	zeit := aroundField(t, body, "feld_abfahrt")
	if !strings.Contains(zeit, `type="time"`) {
		t.Errorf("the time of day is not an <input type=\"time\">:\n%s", zeit)
	}

	// --- bereich ------------------------------------------------------------
	bereich := aroundField(t, body, "feld_menge")
	for _, will := range []string{`type="number"`, `min="1"`, `max="9"`, `step="any"`} {
		if !strings.Contains(bereich, will) {
			t.Errorf("dem Bereichsfeld fehlt %s:\n%s", will, bereich)
		}
	}
	// The placeholder is no ornament: .feld-schalter--text hides a dependent
	// field through :placeholder-shown, and without a placeholder the rule never
	// takes hold.
	if !strings.Contains(bereich, `placeholder=" "`) {
		t.Errorf("the range field is missing the placeholder its dependants hang off:\n%s", bereich)
	}
	if !strings.Contains(bereich, "feld-schalter--text") {
		t.Errorf("the range field carries no switch — does the dependent field hang off it?\n%s", bereich)
	}
	if strings.Contains(bereich, `type="range"`) {
		t.Errorf("the range field has become a slider:\n%s", bereich)
	}

	// Without bounds no attribute — not min="" and not max="". Measured on the
	// element itself and not on a window around it: the min="1" of the
	// neighbouring field must not count here.
	offen := inTag(t, body, "feld_offen")
	for _, darfNicht := range []string{`min=`, `max=`} {
		if strings.Contains(offen, darfNicht) {
			t.Errorf("an unbounded range field carries %s:\n%s", darfNicht, offen)
		}
	}
	// The counter-check on the same path: the bounded field carries both.
	begrenzt := inTag(t, body, "feld_menge")
	if !strings.Contains(begrenzt, `min="1"`) || !strings.Contains(begrenzt, `max="9"`) {
		t.Errorf("the bounds are not on the element itself:\n%s", begrenzt)
	}

	// --- code ---------------------------------------------------------------
	code := aroundField(t, body, "feld_schnipsel")
	if !strings.Contains(code, "<textarea") || !strings.Contains(code, "form-code") {
		t.Errorf("the code field is not a <textarea class=\"… form-code\">:\n%s", code)
	}
	if !strings.Contains(code, `spellcheck="false"`) {
		t.Errorf("dem Codefeld fehlt spellcheck=\"false\":\n%s", code)
	}
}

// The three new controls get by without a line of JavaScript.
//
// What is measured is the section around the three fields and not the whole
// page: the admin shell loads htmx, so a <script> there would be no finding but
// the build of the program.
func TestFieldKindsCarryNoJavaScript(t *testing.T) {
	raw, err := os.ReadFile("../../cmd/holzcloud/templates/admin/field_input.html")
	if err != nil {
		t.Fatalf("die Vorlage lesen: %v", err)
	}
	for _, verboten := range []string{"<script", "onclick", "oninput", "onchange", "javascript:"} {
		if strings.Contains(string(raw), verboten) {
			t.Errorf("field_input.html contains %q", verboten)
		}
	}

	// And the same on the delivered HTML, in the window around each of the
	// three fields. Measuring the whole page would be no finding: the admin
	// shell loads htmx, and that is the build of the program.
	h, sm, database, ws := newTestAdmin(t)
	fields := field.NewStore(database)
	for _, d := range []field.Def{
		{Key: "abfahrt", Label: "Abfahrt", Kind: field.KindTime},
		{Key: "menge", Label: "Menge", Kind: field.KindRange, RangeMin: "1", RangeMax: "9"},
		{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode},
	} {
		d.WebsiteID = ws.ID
		if _, err := fields.Create(context.Background(), d); err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
	}
	p := seedPage(t, database, ws.ID, "Fahrplan", "fahrplan", "text", "draft")
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	body := serve(t, h, sm, h.HandlePageEdit, req).Body.String()
	for _, name := range []string{"feld_abfahrt", "feld_menge", "feld_schnipsel"} {
		fenster := aroundField(t, body, name)
		for _, verboten := range []string{"<script", "onclick", "javascript:"} {
			if strings.Contains(fenster, verboten) {
				t.Errorf("um %s herum steht %q:\n%s", name, verboten, fenster)
			}
		}
	}
}

// The term field in the page editor: a choice out of what this website already
// carries.
//
// The last assertion is the one that matters: the term of another website
// stands nowhere in the form. Everything in this program belongs to exactly one
// website, and a choice field is the place where a foreign one would get in
// most easily — the value would be stored, the field would look filled, and the
// page would print nothing all the same.
func TestATermFieldInThePageEditor(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)
	terms := term.NewStore(database)

	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "thema", Label: "Thema", Kind: field.KindTerm,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	// Two terms of this website's own ...
	p := seedPage(t, database, ws.ID, "Eichentisch", "eichentisch", "text", "draft")
	if err := terms.SetForPage(ctx, ws.ID, p.ID, []string{"Möbelbau", "Eiche"}); err != nil {
		t.Fatalf("create terms: %v", err)
	}
	// ... and one of a foreign one.
	fremd, err := domain.NewStore(database).CreateWebsite(ctx, "Fremde Website", "")
	if err != nil {
		t.Fatalf("zweite Website: %v", err)
	}
	foreignPage := seedPage(t, database, fremd.ID, "Anderswo", "anderswo", "text", "draft")
	if err := terms.SetForPage(ctx, fremd.ID, foreignPage.ID, []string{"Zementbau"}); err != nil {
		t.Fatalf("fremdes Schlagwort: %v", err)
	}

	// The stored value is the slug, not the name.
	raw, err := field.Encode(field.Data{Values: field.Values{"thema": "moebelbau"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.NewStore(database).SetFields(ctx, p.ID, raw); err != nil {
		t.Fatalf("Wert setzen: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	body := rec.Body.String()
	fenster := aroundField(t, body, "feld_thema")

	if !strings.Contains(fenster, "<select") {
		t.Errorf("the term field is not a <select>:\n%s", fenster)
	}
	// Both of its own stand in it — the name as text, the slug as value.
	for kuerzel, name := range map[string]string{"moebelbau": "Möbelbau", "eiche": "Eiche"} {
		if !strings.Contains(fenster, `value="`+kuerzel+`"`) {
			t.Errorf("the field is missing the slug %q:\n%s", kuerzel, fenster)
		}
		if !strings.Contains(fenster, ">"+name+"<") {
			t.Errorf("the field is missing the name %q:\n%s", name, fenster)
		}
	}
	// The stored one is preselected.
	if !strings.Contains(fenster, `value="moebelbau" selected`) {
		t.Errorf("the stored term is not selected:\n%s", fenster)
	}
	// And the foreign one stands nowhere — not in the window and not in the
	// whole document.
	if strings.Contains(body, "Zementbau") || strings.Contains(body, "zementbau") {
		t.Error("another website's term is in the form")
	}
}

// siteTerms hands back an empty list and never an error: a choice field without
// a choice is an empty choice field, a failed query would be a form that can no
// longer be opened at all.
func TestTheTermChoiceFailsQuietly(t *testing.T) {
	ctx := context.Background()

	// Ohne Ablage.
	if got := (&Handler{}).siteTerms(ctx, 1); got != nil {
		t.Errorf("ohne Ablage: %#v, wollte nil", got)
	}

	// And with one that no longer answers.
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "kaputt.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	h := &Handler{terms: term.NewStore(database)}
	database.Close()
	if got := h.siteTerms(ctx, 1); got != nil {
		t.Errorf("bei einem Fehler: %#v, wollte nil", got)
	}
}

// aroundField cuts the form field with the given name out of the document.
//
// A window and not the whole page: a min="1" somewhere else in the document
// would be no proof about the field in question.
func aroundField(t *testing.T, body, name string) string {
	t.Helper()
	at := strings.Index(body, `name="`+name+`"`)
	if at < 0 {
		t.Fatalf("the field %q is not in the form", name)
	}
	von := at - 600
	if von < 0 {
		von = 0
	}
	bis := at + 600
	if bis > len(body) {
		bis = len(body)
	}
	return body[von:bis]
}

// inTag cuts out exactly the element whose name attribute was looked for — from
// its "<" to its ">".
func inTag(t *testing.T, body, name string) string {
	t.Helper()
	at := strings.Index(body, `name="`+name+`"`)
	if at < 0 {
		t.Fatalf("the field %q is not in the form", name)
	}
	von := strings.LastIndex(body[:at], "<")
	bis := strings.Index(body[at:], ">")
	if von < 0 || bis < 0 {
		t.Fatalf("the element around %q is not closed", name)
	}
	return body[von : at+bis+1]
}

// checkFields checks field.For(defs, pageKind) — the fields that stand on this
// page at all. field.Clean ran alongside it over the unfiltered list. Between
// the two lay a hole: a form built by hand could send along the value of a
// field that applies only to posts, and it was stored without anything ever
// having checked it. Since trimTo truncates nothing any more (D-13), without
// any length limit at all.
func TestAForeignFieldValueIsNotStoredAlong(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "vorspann", Label: "Vorspann", Kind: field.KindLong,
		AppliesTo: field.ForPost,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	// A page, not a post — and the value of the post field all the same, well
	// over the byte limit.
	zuLang := strings.Repeat("a", field.MaxValueBytes+50)
	req := postForm("/admin/websites/1/pages/new", url.Values{
		"title":         {"Startseite"},
		"slug":          {"start"},
		"status":        {"published"},
		"kind":          {"page"},
		"feld_vorspann": {zuLang},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}
	if got := field.Decode(p.Fields).Values["vorspann"]; got != "" {
		t.Errorf("%d Byte eines fremden Feldes wurden abgelegt", len(got))
	}
}
