package admin

import (
	"context"
	stdhtml "html"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
)

// Der vierte Modus des Feldbildschirms, dort geprüft, wo die Berechtigung
// wohnt.
//
// `?textbaustein=<id>` ist die einzige Stelle der Phase, an der eine Nummer aus
// der Adresse einen Träger benennt, den `snippets.Get` **ohne** Websitenummer
// heraussucht. Bei einer Bausteinart erledigt das die Abfrage selbst; hier ist
// es Sache des Handlers, und darum steht die Prüfung in `internal/admin` und
// nicht im Speicher. Geprüft wird über die echten Vorlagen von der Platte, denn
// eine Vorlage, die nicht mehr zu ihrer Datenstruktur passt, soll hier
// scheitern und nicht im Browser.

// feldBildschirm ruft GET …/felder mit der übergebenen Abfrage auf.
func feldBildschirm(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/felder"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, h.HandleFieldList, req)
}

// feldAnlegen schickt das Formular des Feldbildschirms ab.
func feldAnlegen(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(websiteID, 10)+"/felder",
		values, map[string]string{"id": strconv.FormatInt(websiteID, 10)}))
}

// zweiteWebsite legt eine zweite Website mit einem eigenen Textbaustein an —
// die Gegenseite jeder Berechtigungsprüfung in dieser Datei.
func zweiteWebsite(t *testing.T, database *db.DB, name, key, snippetName string) (*domain.Website, *snippet.Snippet) {
	t.Helper()
	ctx := context.Background()
	ws, err := domain.NewStore(database).CreateWebsite(ctx, name, "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, key, snippetName, "Inhalt", "<p>Inhalt</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}
	return ws, sn
}

// Der Modus öffnet sich für einen eigenen Textbaustein, trägt seinen Namen und
// legt ein Feld an, das über OfSnippet zurückkommt.
// Eine Gruppe an einem Textbaustein zeichnet ihre Zeilen — der Fehler, den
// erst der Browserdurchgang gezeigt hat.
//
// Der Gruppenbildschirm ist eine Ebene tiefer und trägt „?gruppe=<id>" ohne
// „textbaustein". Wer sein Formular abschickt, legte darum ein Unterfeld mit
// snippet_id NULL an — während seine Gruppe snippet_id trägt. OfSnippet fragt
// „WHERE snippet_id = $2" und gab die Gruppe danach ohne ein einziges
// Unterfeld heraus: eine Gruppe, die auf dem Formular des Textbausteins keine
// Zeile zeichnen kann und im Archiv anders aussieht als auf dem Bildschirm,
// weil der Importweg beides setzt.
//
// Das Unterfeld erbt seinen Träger deshalb aus der gespeicherten Gruppe.
func TestGruppeAmTextbausteinTraegtIhreUnterfelder(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	if rec := feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Öffnungszeiten"},
		"art":          {field.KindGroup},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("Gruppe anlegen: Status %d, wollte 303", rec.Code)
	}

	defs, err := h.fields.OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil || len(defs) != 1 {
		t.Fatalf("OfSnippet: %v (%d)", err, len(defs))
	}
	gruppe := defs[0]

	// Und jetzt das Unterfeld, so wie der Bildschirm es abschickt: mit
	// „gruppe" und ohne „textbaustein", weil eine Ebene tiefer niemand mehr
	// weiss, an wem die Gruppe hängt.
	if rec := feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Tag"},
		"art":          {field.KindText},
		"gruppe":       {strconv.FormatInt(gruppe.ID, 10)},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("Unterfeld anlegen: Status %d, wollte 303", rec.Code)
	}

	defs, err = h.fields.OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil || len(defs) != 1 {
		t.Fatalf("OfSnippet nach dem Unterfeld: %v (%d)", err, len(defs))
	}
	if len(defs[0].Sub) != 1 || defs[0].Sub[0].Key != "tag" {
		t.Fatalf("die Gruppe kommt ohne ihr Unterfeld zurück: %+v", defs[0].Sub)
	}
	if defs[0].Sub[0].SnippetID != sn.ID {
		t.Errorf("das Unterfeld trägt snippet_id %d, wollte %d — es erbt seinen "+
			"Träger aus der gespeicherten Gruppe", defs[0].Sub[0].SnippetID, sn.ID)
	}

	// Die Gegenprobe des gefährlichen Schnitts: das Unterfeld darf dadurch
	// nicht auf dem Seitenbildschirm auftauchen.
	seiten, err := h.fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range seiten {
		if d.Key == "tag" || d.Key == "oeffnungszeiten" {
			t.Errorf("ein Feld des Textbausteins steht in der Seitenliste: %+v", d)
		}
	}
}

func TestTextbausteinModusOeffnetSichFuerDeneigenen(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	// Der leere Fall zuerst: derselbe Bildschirm, eine leere Liste und der
	// Satz, der das sagt.
	rec := feldBildschirm(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	leer := rec.Body.String()
	if !strings.Contains(leer, "Kontaktblock") {
		t.Error("der Bildschirm nennt den Textbaustein nicht beim Namen")
	}
	if !strings.Contains(leer, "noch keine Felder") {
		t.Error("der leere Fall zeigt seinen Satz nicht")
	}
	// Ein Textbaustein ist nicht „einfach": „Gilt für" hat an ihm keine
	// Bedeutung. „Pflicht" dagegen schon, und das Kästchen muss stehen.
	if !strings.Contains(leer, `name="pflicht"`) {
		t.Error("das Pflicht-Kästchen fehlt — an einem Textbaustein darf ein Feld verlangt werden")
	}
	if strings.Contains(leer, `name="gilt_fuer"`) {
		t.Error(`"gilt für" steht auf dem Textbaustein-Bildschirm, wo es nichts bedeutet`)
	}
	// Die Auswahl der Feldart ist die volle: eine Gruppe gehört dazu, anders
	// als bei einer Bausteinart.
	if !strings.Contains(leer, `value="`+field.KindGroup+`"`) {
		t.Error("die Feldartenliste kennt keine Gruppe — sie ist nicht field.Kinds")
	}

	// Und jetzt eines anlegen.
	feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Telefonnummer"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	})

	defs, err := field.NewStore(database).OfSnippet(ctx, ws.ID, sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("%d Felder am Textbaustein, wollte 1", len(defs))
	}
	if defs[0].SnippetID != sn.ID {
		t.Errorf("SnippetID = %d, wollte %d", defs[0].SnippetID, sn.ID)
	}

	rec = feldBildschirm(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if !strings.Contains(rec.Body.String(), "Telefonnummer") {
		t.Error("das angelegte Feld steht nicht in der Liste seines Textbausteins")
	}
}

// Ein Textbaustein einer anderen Website öffnet den Modus nicht — und nichts
// von jener Website erscheint. Die Abwesenheit ist die Zusicherung: ein still
// geöffneter Modus gäbe ebenfalls 200 zurück.
func TestTextbausteinFremderWebsiteOeffnetDenModusNicht(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	// Ein eigenes Seitenfeld, damit der Rückfall auf den Seitenbildschirm
	// etwas Nachweisbares zeigt.
	feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Seitenpreis"},
		"art":          {field.KindNumber},
		"gilt_fuer":    {"beides"},
	})

	_, fremd := zweiteWebsite(t, database, "Zweite Seite", "fremd", "Fremder Kontaktblock")

	rec := feldBildschirm(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(fremd.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Fremder Kontaktblock") {
		t.Error("der Name eines Textbausteins einer anderen Website steht auf dem Bildschirm")
	}
	if !strings.Contains(body, "Seitenpreis") {
		t.Error("der Rückfall auf die eigenen Seitenfelder fand nicht statt")
	}
}

// Derselbe Versuch als POST. Der Status allein beweist nichts: ein Handler, der
// erst anlegt und dann 404 sagt, käme damit durch. Deshalb wird über den
// Speicher nachgesehen.
func TestTextbausteinFremderWebsiteWirdBeimSpeichernAbgewiesen(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fremdeWS, fremd := zweiteWebsite(t, database, "Zweite Seite", "fremd", "Fremder Kontaktblock")

	rec := feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Eingeschmuggelt"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(fremd.ID, 10)},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Status %d, wollte 404", rec.Code)
	}

	fields := field.NewStore(database)
	// Unter beiden Websitenummern nachsehen: angelegt worden wäre die
	// Definition mit der Nummer aus der Adresse, gefunden werden soll sie
	// unter keiner von beiden.
	for _, id := range []int64{ws.ID, fremdeWS.ID} {
		defs, err := fields.OfSnippet(ctx, id, fremd.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(defs) != 0 {
			t.Errorf("Website %d trägt %d Definitionen am fremden Textbaustein, wollte 0", id, len(defs))
		}
	}
}

// Der Seitenbildschirm bleibt sauber: mit einem Textbausteinfeld in der
// Datenbank listet GET …/felder ohne Abfrageparameter genau die eigenen Felder
// der Seite. Das ist ROADMAP-Kriterium 3 als Prüfung, die bei jedem Commit
// läuft.
func TestTextbausteinfeldErscheintNichtAufDemSeitenbildschirm(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Seitenpreis"},
		"art":          {field.KindNumber},
		"gilt_fuer":    {"beides"},
	})
	feldAnlegen(t, h, sm, ws.ID, url.Values{
		"beschriftung": {"Telefonnummer"},
		"art":          {field.KindText},
		"textbaustein": {strconv.FormatInt(sn.ID, 10)},
	})

	rec := feldBildschirm(t, h, sm, ws.ID, "")
	body := rec.Body.String()
	if !strings.Contains(body, "Seitenpreis") {
		t.Error("das eigene Feld der Seite fehlt auf dem Seitenbildschirm")
	}
	if strings.Contains(body, "Telefonnummer") {
		t.Error("ein Textbausteinfeld steht auf dem Seitenbildschirm — der gefährliche Schnitt dieser Phase")
	}
}

// Die Wertehälfte: was die Redaktion in die Felder eines Textbausteins tippt.
//
// Die vier Fälle hier fahren über die echten Vorlagen von der Platte und über
// den echten Speicherweg, weil beide Enden zusammen die Zusage tragen: ein
// Formular, das seine Namen selbst prägt, und ein Parser, der andere erwartet,
// sind einzeln grün und zusammen still kaputt. Deshalb baut jeder Fall seine
// Formularschlüssel mit field.Def.FieldName — derselben Funktion, aus der die
// Vorlage sie bezieht — und nie von Hand.
//
// Wie ein übersehener Ort aussähe, wenn nur die zählenden Tore liefen: jedes
// grep -c der Pläne 08-01 bis 08-04 meldet grün, das Textbausteinfeld
// erscheint zusätzlich im Seiteneditor unter einem Namen, den niemand gewählt
// hat, jemand füllt es aus, und der Wert landet in der fields-Spalte der Seite,
// wo kein Theme ihn liest. Nichts protokolliert, nichts schlägt fehl, und die
// erste Meldung ist ein Bildschirmfoto. Der vierte Fall unten ist genau dagegen
// geschrieben.

// textbausteinSpeichern schickt das Wertformular eines Textbausteins ab.
func textbausteinSpeichern(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, h, sm, h.HandleSnippetList, postForm(
		"/admin/websites/"+strconv.FormatInt(websiteID, 10)+"/snippets",
		values, map[string]string{"id": strconv.FormatInt(websiteID, 10)}))
}

// textbausteinBildschirm ruft GET …/snippets mit der übergebenen Abfrage auf.
func textbausteinBildschirm(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/snippets"
	if query != "" {
		target += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", strconv.FormatInt(websiteID, 10))
	return serve(t, h, sm, h.HandleSnippetList, req)
}

// textbausteinFeld legt eine Felddefinition an einem Textbaustein an und gibt
// sie zurück, damit der Aufrufer seinen Formularschlüssel aus FieldName holen
// kann statt ihn zu tippen.
func textbausteinFeld(t *testing.T, database *db.DB, websiteID, snippetID int64, def field.Def) field.Def {
	t.Helper()
	def.WebsiteID = websiteID
	def.SnippetID = snippetID
	angelegt, err := field.NewStore(database).Create(context.Background(), def)
	if err != nil {
		t.Fatalf("Textbausteinfeld %q anlegen: %v", def.Key, err)
	}
	return *angelegt
}

// bausteinMitFeldern legt einen Textbaustein an und gibt ihn zurück.
func bausteinMitFeldern(t *testing.T, database *db.DB, websiteID int64, key, name string) *snippet.Snippet {
	t.Helper()
	sn, err := snippet.NewStore(database).Create(context.Background(), websiteID, key, name,
		"Wir sind **da**.", "<p>Wir sind <strong>da</strong>.</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}
	return sn
}

// gespeicherteFelder liest die fields-Spalte eines Textbausteins zurück.
func gespeicherteFelder(t *testing.T, database *db.DB, websiteID, id int64) field.Data {
	t.Helper()
	sn, err := snippet.NewStore(database).Get(context.Background(), websiteID, id)
	if err != nil || sn == nil {
		t.Fatalf("den Textbaustein zurücklesen: %v", err)
	}
	return field.Decode(sn.Fields)
}

// Der Rundlauf: getippt, gespeichert, wieder geöffnet, und dieselben Werte
// stehen da — im Formular und in der Spalte.
func TestSnippetFeldRundlauf(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := bausteinMitFeldern(t, database, ws.ID, "kontakt", "Kontaktblock")
	kurz := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefon", Label: "Telefon", Kind: field.KindText})
	lang := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	rec := textbausteinSpeichern(t, h, sm, ws.ID, url.Values{
		"id":               {strconv.FormatInt(sn.ID, 10)},
		"key":              {"kontakt"},
		"name":             {"Kontaktblock"},
		"content_markdown": {"Wir sind **da**."},
		kurz.FieldName():   {"07721 123456"},
		lang.FieldName():   {"Nur vormittags erreichbar."},
	})
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("Status %d, wollte eine Umleitung nach dem Speichern", rec.Code)
	}

	daten := gespeicherteFelder(t, database, ws.ID, sn.ID)
	if got := daten.Values["telefon"]; got != "07721 123456" {
		t.Errorf("telefon = %q, wollte %q", got, "07721 123456")
	}
	if got := daten.Values["hinweis"]; got != "Nur vormittags erreichbar." {
		t.Errorf("hinweis = %q, wollte %q", got, "Nur vormittags erreichbar.")
	}

	// Und dasselbe im Formular, unter denselben Namen.
	body := textbausteinBildschirm(t, h, sm, ws.ID, "edit="+strconv.FormatInt(sn.ID, 10)).Body.String()
	for _, wollte := range []string{
		`name="` + kurz.FieldName() + `"`,
		`name="` + lang.FieldName() + `"`,
		"07721 123456",
		"Nur vormittags erreichbar.",
	} {
		if !strings.Contains(body, wollte) {
			t.Errorf("das wiedergeöffnete Formular trägt %q nicht", wollte)
		}
	}

	// Der Rumpf und die Kennung haben den Durchgang überstanden.
	sn2, err := snippet.NewStore(database).Get(context.Background(), ws.ID, sn.ID)
	if err != nil || sn2 == nil {
		t.Fatalf("zurücklesen: %v", err)
	}
	if sn2.Key != "kontakt" || sn2.ContentMarkdown != "Wir sind **da**." {
		t.Errorf("Kennung oder Rumpf haben sich geändert: %q / %q", sn2.Key, sn2.ContentMarkdown)
	}
}

// Ein abgewiesenes Speichern schreibt nichts — auch nicht halb.
//
// Der Nachweis steht im Speicher und nicht nur im Rumpf der Antwort: ein
// Handler, der erst schreibt und dann ablehnt, käme sonst durch.
func TestSnippetFeldPflichtWirdAbgewiesen(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := bausteinMitFeldern(t, database, ws.ID, "kontakt", "Kontaktblock")
	pflicht := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefon", Label: "Telefon", Kind: field.KindText, Required: true})
	frei := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	gemeinsam := func(telefon, hinweis string) url.Values {
		return url.Values{
			"id":                {strconv.FormatInt(sn.ID, 10)},
			"key":               {"kontakt"},
			"name":              {"Kontaktblock"},
			"content_markdown":  {"Wir sind **da**."},
			pflicht.FieldName(): {telefon},
			frei.FieldName():    {hinweis},
		}
	}

	textbausteinSpeichern(t, h, sm, ws.ID, gemeinsam("07721 123456", "Erster Hinweis"))
	if got := gespeicherteFelder(t, database, ws.ID, sn.ID).Values["telefon"]; got != "07721 123456" {
		t.Fatalf("der erste Durchgang hat nichts abgelegt: telefon = %q", got)
	}

	rec := textbausteinSpeichern(t, h, sm, ws.ID, gemeinsam("", "Zweiter Hinweis"))
	if rec.Code == http.StatusSeeOther || rec.Code == http.StatusFound {
		t.Fatalf("Status %d — das leere Pflichtfeld wurde angenommen", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Zweiter Hinweis") {
		t.Error("das abgewiesene Formular hat den getippten Wert des anderen Feldes verloren")
	}
	if !strings.Contains(body, "muss ausgefüllt werden") {
		t.Error("neben dem Feld steht kein Grund")
	}

	daten := gespeicherteFelder(t, database, ws.ID, sn.ID)
	if got := daten.Values["telefon"]; got != "07721 123456" {
		t.Errorf("telefon = %q — die Ablehnung hat den vorherigen Wert angetastet", got)
	}
	if got := daten.Values["hinweis"]; got != "Erster Hinweis" {
		t.Errorf("hinweis = %q — die Ablehnung hat halb geschrieben", got)
	}
}

// TestSnippetFeldSanierung: derselbe Schutz auf beiden Trägern, und der Beweis
// ist, dass die beiden übereinstimmen.
//
// Was einen langtext-Feldwert schützt, ist **nicht** goldmark und nicht
// bluemonday. field.Resolve hat für KindLong keinen eigenen Arm; der Wert fällt
// in den default:-Arm und kommt als schlichte Go-Zeichenkette heraus, und
// html/template maskiert sie kontextabhängig dort, wo das Theme sie druckt.
// Genau dasselbe geschieht mit demselben Wert auf einer Seite — deshalb ist
// „die beiden stimmen überein" eine Aussage über einen geteilten Mechanismus
// und kein Zufall.
//
// Die Kette goldmark → bluemonday gehört dem **Rumpf** des Textbausteins. Das
// ist ein anderer Wert auf einem anderen Weg, und er wird hier nur genannt, um
// die Linie zu ziehen.
//
// Was dieser Fall nicht ist: er prüft nicht den Maskierer von html/template
// nach, der eigene Prüfungen hat, und er prüft goldmark nicht nach. Er weist
// nach, dass der Feldweg des Textbausteins dieselbe Maskierung erreicht wie der
// Feldweg der Seite — „eine Kette, keine zweite" ist eine Eigenschaft des
// Aufrufgraphen, und so wird sie von aussen behauptet.
func TestSnippetFeldSanierung(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	const boshaft = `<script>alert(1)</script>`

	sn := bausteinMitFeldern(t, database, ws.ID, "kontakt", "Kontaktblock")
	amBaustein := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})

	// Dieselbe Kennung, dieselbe Feldart, der andere Träger.
	anDerSeite, err := field.NewStore(database).Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "hinweis", Label: "Hinweis", Kind: field.KindLong})
	if err != nil {
		t.Fatalf("Seitenfeld anlegen: %v", err)
	}

	textbausteinSpeichern(t, h, sm, ws.ID, url.Values{
		"id":                   {strconv.FormatInt(sn.ID, 10)},
		"key":                  {"kontakt"},
		"name":                 {"Kontaktblock"},
		"content_markdown":     {"Wir sind **da**."},
		amBaustein.FieldName(): {boshaft},
	})
	serve(t, h, sm, h.HandlePageCreate, postForm("/admin/websites/1/pages/new", url.Values{
		"title":                {"Startseite"},
		"slug":                 {"start"},
		"status":               {"published"},
		"kind":                 {"page"},
		anDerSeite.FieldName(): {boshaft},
	}, map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("die Seite wurde nicht angelegt: %v", err)
	}

	// Beide Träger durch denselben Auflöser, mit ihren eigenen Definitionen.
	amBausteinAufgeloest := field.Resolve([]field.Def{amBaustein},
		gespeicherteFelder(t, database, ws.ID, sn.ID), field.Links{})
	anDerSeiteAufgeloest := field.Resolve([]field.Def{*anDerSeite},
		field.Decode(p.Fields), field.Links{})

	if amBausteinAufgeloest["hinweis"] != anDerSeiteAufgeloest["hinweis"] {
		t.Fatalf("Textbaustein und Seite lösen denselben Wert verschieden auf:\n  Baustein: %#v\n  Seite:    %#v",
			amBausteinAufgeloest["hinweis"], anDerSeiteAufgeloest["hinweis"])
	}

	// Und so, wie ein Theme sie druckt: dieselbe Vorlage über beide.
	wieEinTheme := template.Must(template.New("theme").Parse(`<p class="hinweis">{{.}}</p>`))
	druck := func(wert any) string {
		var aus strings.Builder
		if err := wieEinTheme.Execute(&aus, wert); err != nil {
			t.Fatalf("drucken: %v", err)
		}
		return aus.String()
	}
	ausBaustein := druck(amBausteinAufgeloest["hinweis"])
	ausSeite := druck(anDerSeiteAufgeloest["hinweis"])

	for name, aus := range map[string]string{"Textbaustein": ausBaustein, "Seite": ausSeite} {
		if strings.Contains(aus, "<script") {
			t.Errorf("%s: eine lebende Marke hat überlebt: %s", name, aus)
		}
	}
	if ausBaustein != ausSeite {
		t.Errorf("die beiden Träger drucken denselben Wert verschieden:\n  Baustein: %s\n  Seite:    %s",
			ausBaustein, ausSeite)
	}

	// Der Rumpf behält seine eigene, andere Kette — unberührt von dieser Phase.
	sn2, err := snippet.NewStore(database).Get(ctx, ws.ID, sn.ID)
	if err != nil || sn2 == nil {
		t.Fatalf("zurücklesen: %v", err)
	}
	if !strings.Contains(sn2.ContentHTML, "<strong>da</strong>") {
		t.Errorf("der Markdown-Rumpf ist nicht mehr das, was die Kette aus ihm gemacht hat: %q", sn2.ContentHTML)
	}
}

// Das Seitenformular bleibt sauber, im Browser gesehen.
//
// Die Speicherhälfte dieser Zusage hält 08-01s TestBausteinNamensraum. Hier
// steht die andere: was eine Redaktorin *sieht*, ist eine gezeichnete Vorlage
// und kein Abfrageergebnis, und darum wird sie gezeichnet.
func TestSnippetFeldStehtNichtImSeitenformular(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	sn := bausteinMitFeldern(t, database, ws.ID, "kontakt", "Kontaktblock")
	amBaustein := textbausteinFeld(t, database, ws.ID, sn.ID, field.Def{
		Key: "telefonnummer", Label: "Telefonnummer", Kind: field.KindText})

	textbausteinSpeichern(t, h, sm, ws.ID, url.Values{
		"id":                   {strconv.FormatInt(sn.ID, 10)},
		"key":                  {"kontakt"},
		"name":                 {"Kontaktblock"},
		"content_markdown":     {"Wir sind **da**."},
		amBaustein.FieldName(): {"07721 123456"},
	})

	req := httptest.NewRequest(http.MethodGet,
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/pages/new", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if strings.Contains(body, "Telefonnummer") {
		t.Error("die Beschriftung eines Textbausteinfeldes steht im Seiteneditor")
	}
	// Der einzige von Hand geschriebene Feldpräfix dieser Datei, und er steht
	// in einer Behauptung über eine Abwesenheit.
	if strings.Contains(body, `name="feld_telefonnummer"`) {
		t.Error("der Formularname eines Textbausteinfeldes steht im Seiteneditor")
	}
}

// codeAusdruck holt den ersten <code>…</code> aus einer gerenderten Seite und
// macht die Entitäten wieder zu Zeichen.
//
// Der Bildschirm schreibt die geschweiften Klammern als &#123;, sonst würde die
// Verwaltungsvorlage den Rat selbst auszuführen versuchen. Für die Prüfung muss
// er wieder das sein, was der Betreiber abschreibt.
func codeAusdruck(t *testing.T, koerper string) string {
	t.Helper()
	auf := strings.Index(koerper, "<code>")
	if auf < 0 {
		t.Fatal("kein <code> auf dem Bildschirm")
	}
	rest := koerper[auf+len("<code>"):]
	zu := strings.Index(rest, "</code>")
	if zu < 0 {
		t.Fatal("<code> ohne Ende")
	}
	return stdhtml.UnescapeString(rest[:zu])
}

// Der Rat auf dem Bildschirm muss ein Ausdruck sein, der sich übersetzen lässt.
//
// validKey erlaubt den Bindestrich ausdrücklich (internal/admin/snippet.go),
// und der Musterschlüssel dieses Projekts heisst „footer-kontakt"
// (internal/template/sample.go). Die Feldsuffixform
// {{.Site.SnippetFields.footer-kontakt.telefon}} ist für Go kein Ausdruck,
// sondern ein Übersetzungsfehler — „bad character U+002D". Wer den Rat
// abschreibt, bekommt sein Theme von template.Check abgewiesen, mit einer
// Meldung, die ein Zeichen nennt und keine Ursache.
//
// Geprüft wird deshalb nicht der Wortlaut, sondern die Eigenschaft: was da
// steht, geht durch den Übersetzer. TEMPLATE-SPEC.md benutzt durchweg index;
// der Bildschirm war die eine Stelle, die der Spezifikation widersprach.
func TestRatDesFeldbildschirmsLaesstSichUebersetzen(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	sn, err := snippet.NewStore(database).Create(ctx, ws.ID, "footer-kontakt", "Kontaktblock",
		"Adresse", "<p>Adresse</p>")
	if err != nil {
		t.Fatalf("snippet.Create: %v", err)
	}

	rec := feldBildschirm(t, h, sm, ws.ID, "textbaustein="+strconv.FormatInt(sn.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d, wollte 200", rec.Code)
	}
	rat := codeAusdruck(t, rec.Body.String())
	if !strings.Contains(rat, "footer-kontakt") {
		t.Fatalf("der Rat nennt den Schlüssel nicht: %q", rat)
	}
	if _, err := template.New("rat").Parse(rat); err != nil {
		t.Errorf("der Rat auf dem Bildschirm ist kein übersetzbarer Ausdruck:\n  %s\n  %v",
			rat, err)
	}
}
