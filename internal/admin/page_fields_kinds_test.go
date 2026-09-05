package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// Die drei kleinen Arten im Seiteneditor: zeit, bereich und code.
//
// Geprüft wird das ausgelieferte HTML und nicht das Ansichtsmodell. Was ein
// FieldView trägt, nützt nichts, wenn der Zweig in field_input.html fehlt —
// und ein fehlender Zweig fällt nicht auf: die Kette endet in einem
// gewöhnlichen Textfeld, das eine Uhrzeit ebenso klaglos entgegennimmt.
func TestFeldartenImSeiteneditor(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	anlegen := func(d field.Def) {
		t.Helper()
		d.WebsiteID = ws.ID
		if _, err := fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
	}

	anlegen(field.Def{Key: "abfahrt", Label: "Abfahrt", Kind: field.KindTime})
	// Mit Grenzen — und mit einem abhängigen Feld daran, damit .Switch "text"
	// ist und der Platzhalterzweig überhaupt gezogen wird. Ohne ihn kann ein
	// Bereichsfeld seine Abhängigen nie ein- und ausblenden.
	anlegen(field.Def{Key: "menge", Label: "Menge", Kind: field.KindRange,
		RangeMin: "1", RangeMax: "9"})
	anlegen(field.Def{Key: "hinweis", Label: "Hinweis", Kind: field.KindText,
		Condition: "menge"})
	// Und eines ohne Grenzen: nach oben und unten offen ist eine gültige
	// Angabe, und dann darf kein leeres min="" im Formular stehen.
	anlegen(field.Def{Key: "offen", Label: "Offen", Kind: field.KindRange})
	anlegen(field.Def{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode})

	p := seedPage(t, database, ws.ID, "Fahrplan", "fahrplan", "text", "draft")
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("das Formular gab %d zurück", rec.Code)
	}
	body := rec.Body.String()

	// --- zeit ---------------------------------------------------------------
	zeit := umFeld(t, body, "feld_abfahrt")
	if !strings.Contains(zeit, `type="time"`) {
		t.Errorf("die Uhrzeit ist kein <input type=\"time\">:\n%s", zeit)
	}

	// --- bereich ------------------------------------------------------------
	bereich := umFeld(t, body, "feld_menge")
	for _, will := range []string{`type="number"`, `min="1"`, `max="9"`, `step="any"`} {
		if !strings.Contains(bereich, will) {
			t.Errorf("dem Bereichsfeld fehlt %s:\n%s", will, bereich)
		}
	}
	// Der Platzhalter ist keine Zier: .feld-schalter--text blendet ein
	// abhängiges Feld über :placeholder-shown aus, und ohne Platzhalter greift
	// die Regel nie.
	if !strings.Contains(bereich, `placeholder=" "`) {
		t.Errorf("dem Bereichsfeld fehlt der Platzhalter, an dem seine Abhängigen hängen:\n%s", bereich)
	}
	if !strings.Contains(bereich, "feld-schalter--text") {
		t.Errorf("das Bereichsfeld trägt keinen Schalter — hängt das abhängige Feld daran?\n%s", bereich)
	}
	if strings.Contains(bereich, `type="range"`) {
		t.Errorf("das Bereichsfeld ist ein Schieber geworden:\n%s", bereich)
	}

	// Ohne Grenzen kein Attribut — nicht min="" und nicht max="". Gemessen am
	// Element selbst und nicht an einem Fenster darum herum: das min="1" des
	// Nachbarfeldes darf hier nicht mitzählen.
	offen := imTag(t, body, "feld_offen")
	for _, darfNicht := range []string{`min=`, `max=`} {
		if strings.Contains(offen, darfNicht) {
			t.Errorf("ein unbegrenztes Bereichsfeld trägt %s:\n%s", darfNicht, offen)
		}
	}
	// Die Gegenprobe auf demselben Weg: das begrenzte Feld trägt beide.
	begrenzt := imTag(t, body, "feld_menge")
	if !strings.Contains(begrenzt, `min="1"`) || !strings.Contains(begrenzt, `max="9"`) {
		t.Errorf("die Grenzen stehen nicht am Element selbst:\n%s", begrenzt)
	}

	// --- code ---------------------------------------------------------------
	code := umFeld(t, body, "feld_schnipsel")
	if !strings.Contains(code, "<textarea") || !strings.Contains(code, "form-code") {
		t.Errorf("das Codefeld ist kein <textarea class=\"… form-code\">:\n%s", code)
	}
	if !strings.Contains(code, `spellcheck="false"`) {
		t.Errorf("dem Codefeld fehlt spellcheck=\"false\":\n%s", code)
	}
}

// Die drei neuen Bedienelemente kommen ohne eine Zeile JavaScript aus.
//
// Gemessen wird der Ausschnitt um die drei Felder herum und nicht die ganze
// Seite: die Verwaltungshülle lädt htmx, ein <script> dort wäre also kein
// Befund, sondern die Bauart des Programms.
func TestFeldartenTragenKeinJavaScript(t *testing.T) {
	roh, err := os.ReadFile("../../cmd/holzcloud/templates/admin/field_input.html")
	if err != nil {
		t.Fatalf("die Vorlage lesen: %v", err)
	}
	for _, verboten := range []string{"<script", "onclick", "oninput", "onchange", "javascript:"} {
		if strings.Contains(string(roh), verboten) {
			t.Errorf("field_input.html enthält %q", verboten)
		}
	}

	// Und dasselbe am ausgelieferten HTML, im Fenster um jedes der drei
	// Felder. Die ganze Seite zu messen wäre kein Befund: die
	// Verwaltungshülle lädt htmx, und das ist die Bauart des Programms.
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
		fenster := umFeld(t, body, name)
		for _, verboten := range []string{"<script", "onclick", "javascript:"} {
			if strings.Contains(fenster, verboten) {
				t.Errorf("um %s herum steht %q:\n%s", name, verboten, fenster)
			}
		}
	}
}

// umFeld schneidet das Formularfeld mit dem gegebenen Namen aus dem Dokument.
//
// Ein Fenster und nicht die ganze Seite: ein min="1" irgendwo sonst im
// Dokument wäre kein Beweis für das Feld, um das es geht.
func umFeld(t *testing.T, body, name string) string {
	t.Helper()
	at := strings.Index(body, `name="`+name+`"`)
	if at < 0 {
		t.Fatalf("das Feld %q steht nicht im Formular", name)
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

// imTag schneidet genau das Element aus, dessen name-Attribut gesucht wurde —
// von seinem "<" bis zu seinem ">".
func imTag(t *testing.T, body, name string) string {
	t.Helper()
	at := strings.Index(body, `name="`+name+`"`)
	if at < 0 {
		t.Fatalf("das Feld %q steht nicht im Formular", name)
	}
	von := strings.LastIndex(body[:at], "<")
	bis := strings.Index(body[at:], ">")
	if von < 0 || bis < 0 {
		t.Fatalf("das Element um %q ist nicht geschlossen", name)
	}
	return body[von : at+bis+1]
}
