package public

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// bausteinFS ist ein Theme, das beide Hälften eines Textbausteins druckt: den
// Rumpf über .Site.Snippets und ein einzelnes Feld über .Site.Bausteinfelder.
//
// Und es druckt die Zahl der Einträge in .Page.Feldliste dazu. Das ist die im
// Browser sichtbare Hälfte von D-03: die Seite hat keine eigenen Felder, und
// wenn dort etwas steht, ist das Feld des Textbausteins in den Seitenweg
// gelaufen.
func bausteinFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><body>{{template "content" .}}</body></html>`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article>` +
				`<p class="telefon">{{index .Site.Bausteinfelder "kontakt" "telefon"}}</p>` +
				`<div class="rumpf">{{index .Site.Snippets "kontakt"}}</div>` +
				`<p class="eigene">{{len .Page.Feldliste}}</p>` +
				`</article>{{end}}`)},
		"home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<main>{{.Page.Title}}</main>{{end}}`)},
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">nichts gefunden</p>{{end}}`)},
	}
}

// TestBausteinfelderErreichenDasTheme führt ein Feld eines Textbausteins den
// ganzen Weg: Definition in der Tabelle, Wert am Textbaustein, Auflösung auf
// dem Weg nach draussen, Ausgabe durch eine Vorlage auf einer echten
// öffentlichen Adresse.
//
// Vier Zusagen auf einmal, und jede fällt einzeln auf:
//   - der Feldwert erscheint,
//   - der Markdown-Rumpf des Textbausteins erscheint weiterhin,
//   - .Site.Snippets trägt weiterhin template.HTML,
//   - .Page.Feldliste bleibt leer — die Seite hat keine eigenen Felder, und das
//     Feld des Textbausteins darf dort nicht auftauchen.
func TestBausteinfelderErreichenDasTheme(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	loader := tmpl.NewLoader(dir, bausteinFS(), nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), loader, nil, dir, bausteinFS(), false)
	felder := field.NewStore(database)
	h.SetFieldStore(felder)

	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Kontakt", "kontakt-seite", "# Kontakt", "published")

	// Der Textbaustein mit einem Rumpf aus Markdown, durch dieselbe Kette wie
	// jeder Seiteninhalt: goldmark, dann bluemonday. Eine zweite Kette gibt es
	// nicht, und darum trägt der Guss nach template.HTML dieselbe Zusage.
	html, err := page.RenderMarkdown("Wir sind **da**.")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	bausteine := snippet.NewStore(database)
	sn, err := bausteine.Create(ctx, ws.ID, "kontakt", "Kontakt", "Wir sind **da**.", html)
	if err != nil {
		t.Fatalf("Textbaustein anlegen: %v", err)
	}

	if _, err := felder.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "telefon", Label: "Telefon", Kind: field.KindText,
		SnippetID: sn.ID}); err != nil {
		t.Fatalf("Textbausteinfeld anlegen: %v", err)
	}

	roh, err := field.Encode(field.Data{Values: field.Values{"telefon": "07721 123456"}})
	if err != nil {
		t.Fatalf("field.Encode: %v", err)
	}
	if err := bausteine.SetFields(ctx, sn.ID, roh); err != nil {
		t.Fatalf("SetFields: %v", err)
	}

	rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
		r.SetPathValue("slug", "kontakt-seite")
		return h.HandlePage(w, r)
	}, ws, "GET", "/kontakt-seite")
	if err != nil {
		t.Fatalf("HandlePage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
		t.Errorf("der Feldwert steht nicht in der Seite:\n%s", body)
	}
	if !strings.Contains(body, "<strong>da</strong>") {
		t.Errorf("der Markdown-Rumpf des Textbausteins fehlt:\n%s", body)
	}
	if !strings.Contains(body, `<p class="eigene">0</p>`) {
		t.Errorf(".Page.Feldliste ist nicht leer — ein Feld des Textbausteins ist "+
			"in den Seitenweg gelaufen, und genau das sähe man sonst erst im Browser:\n%s", body)
	}
}

// TestSnippetsBleibtTemplateHTML hält SNIP-05 fest: .Site.Snippets ändert
// seinen Typ nicht, und ein Textbaustein ohne eine einzige Felddefinition
// bekommt trotzdem seinen Eintrag in .Site.Bausteinfelder — eine leere Karte
// und keinen fehlenden Schlüssel.
func TestSnippetsBleibtTemplateHTML(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	loader := tmpl.NewLoader(dir, bausteinFS(), nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), loader, nil, dir, bausteinFS(), false)
	h.SetFieldStore(field.NewStore(database))

	ws := seedWebsite(t, database, "Test Site")
	html, err := page.RenderMarkdown("Nur Text.")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if _, err := snippet.NewStore(database).Create(ctx, ws.ID, "kontakt", "Kontakt", "Nur Text.", html); err != nil {
		t.Fatalf("Textbaustein anlegen: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))

	var site tmpl.SiteData
	h.fillSnippets(req, &site, ws.ID, h.loadSnippets(req, ws.ID))

	var _ map[string]template.HTML = site.Snippets

	if _, ok := site.Snippets["kontakt"]; !ok {
		t.Error(".Site.Snippets hat den Textbaustein verloren")
	}
	werte, ok := site.Bausteinfelder["kontakt"]
	if !ok {
		t.Fatal(".Site.Bausteinfelder hat keinen Eintrag für einen Textbaustein ohne Felder — " +
			"ein Theme, das durch ihn hindurchgreift, scheitert dann auf der Anfrage eines Besuchers")
	}
	if werte == nil {
		t.Error(".Site.Bausteinfelder trägt eine nil-Karte statt einer leeren")
	}
	if len(werte) != 0 {
		t.Errorf(".Site.Bausteinfelder trägt %d Werte, erwartet keine", len(werte))
	}
	if _, ok := site.Bausteinliste["kontakt"]; ok {
		t.Error(".Site.Bausteinliste trägt einen Eintrag ohne einen einzigen gefüllten Wert — " +
			"field.List gibt keinen leeren heraus")
	}
}
