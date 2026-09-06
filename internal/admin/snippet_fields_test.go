package admin

import (
	"context"
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
