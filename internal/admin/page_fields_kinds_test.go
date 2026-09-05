package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
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

// Das Schlagwortfeld im Seiteneditor: eine Auswahl aus dem, was diese Website
// schon trägt.
//
// Die letzte Behauptung ist die, auf die es ankommt: das Schlagwort einer
// anderen Website steht nirgends im Formular. Jede Sache in diesem Programm
// gehört genau einer Website, und ein Auswahlfeld ist die Stelle, an der eine
// fremde am leichtesten hereinkäme — der Wert wäre gespeichert, das Feld sähe
// gefüllt aus, und die Seite druckte trotzdem nichts.
func TestSchlagwortfeldImSeiteneditor(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)
	terms := term.NewStore(database)

	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "thema", Label: "Thema", Kind: field.KindTerm,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	// Zwei eigene Schlagwörter dieser Website ...
	p := seedPage(t, database, ws.ID, "Eichentisch", "eichentisch", "text", "draft")
	if err := terms.SetForPage(ctx, ws.ID, p.ID, []string{"Möbelbau", "Eiche"}); err != nil {
		t.Fatalf("Schlagwörter anlegen: %v", err)
	}
	// ... und eines einer fremden.
	fremd, err := domain.NewStore(database).CreateWebsite(ctx, "Fremde Website", "")
	if err != nil {
		t.Fatalf("zweite Website: %v", err)
	}
	fremdeSeite := seedPage(t, database, fremd.ID, "Anderswo", "anderswo", "text", "draft")
	if err := terms.SetForPage(ctx, fremd.ID, fremdeSeite.ID, []string{"Zementbau"}); err != nil {
		t.Fatalf("fremdes Schlagwort: %v", err)
	}

	// Der gespeicherte Wert ist das Kürzel, nicht der Name.
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
		t.Fatalf("das Formular gab %d zurück", rec.Code)
	}
	body := rec.Body.String()
	fenster := umFeld(t, body, "feld_thema")

	if !strings.Contains(fenster, "<select") {
		t.Errorf("das Schlagwortfeld ist kein <select>:\n%s", fenster)
	}
	// Beide eigenen stehen darin — der Name als Text, das Kürzel als Wert.
	for kuerzel, name := range map[string]string{"moebelbau": "Möbelbau", "eiche": "Eiche"} {
		if !strings.Contains(fenster, `value="`+kuerzel+`"`) {
			t.Errorf("dem Feld fehlt das Kürzel %q:\n%s", kuerzel, fenster)
		}
		if !strings.Contains(fenster, ">"+name+"<") {
			t.Errorf("dem Feld fehlt der Name %q:\n%s", name, fenster)
		}
	}
	// Das gespeicherte ist vorausgewählt.
	if !strings.Contains(fenster, `value="moebelbau" selected`) {
		t.Errorf("das gespeicherte Schlagwort ist nicht ausgewählt:\n%s", fenster)
	}
	// Und das fremde steht nirgends — nicht im Fenster und nicht im ganzen
	// Dokument.
	if strings.Contains(body, "Zementbau") || strings.Contains(body, "zementbau") {
		t.Error("das Schlagwort einer fremden Website steht im Formular")
	}
}

// siteTerms gibt eine leere Liste zurück und niemals einen Fehler: ein
// Auswahlfeld ohne Auswahl ist ein leeres Auswahlfeld, eine gescheiterte
// Anfrage wäre ein Formular, das sich gar nicht mehr öffnen lässt.
func TestSchlagwortauswahlScheitertLeise(t *testing.T) {
	ctx := context.Background()

	// Ohne Ablage.
	if got := (&Handler{}).siteTerms(ctx, 1); got != nil {
		t.Errorf("ohne Ablage: %#v, wollte nil", got)
	}

	// Und mit einer, die nicht mehr antwortet.
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
