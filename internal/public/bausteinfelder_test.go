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
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// bausteinFS ist ein Theme, das beide Hälften eines Textbausteins druckt: den
// Rumpf über .Site.Snippets und ein einzelnes Feld über .Site.SnippetFields.
//
// Und es druckt die Zahl der Einträge in .Page.FieldList dazu. Das ist die im
// Browser sichtbare Hälfte von D-03: die Seite hat keine eigenen Felder, und
// wenn dort etwas steht, ist das Feld des Textbausteins in den Seitenweg
// gelaufen.
func bausteinFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><body>{{template "content" .}}</body></html>`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article>` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>` +
				`<div class="rumpf">{{index .Site.Snippets "kontakt"}}</div>` +
				`<p class="eigene">{{len .Page.FieldList}}</p>` +
				`</article>{{end}}`)},
		"home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<main>{{.Page.Title}}</main>{{end}}`)},
		// Die 404- und die Wartungsansicht drucken dieselbe eine Zeile: es sind
		// die Seiten, auf denen ein Besucher am ehesten den Kontakt sucht, und
		// gerade dort war die Fläche leer.
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">nichts gefunden</p>` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>{{end}}`)},
		"maintenance.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="wartung">gleich zurück</p>` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>{{end}}`)},
		// Drei Ansichten von drei verschiedenen Zuschnitten, alle mit derselben
		// einen Zeile: das Schlagwortarchiv und das Beitragsarchiv teilen sich
		// list.html, die Suche und der Katalog haben je eine eigene.
		"list.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="liste">` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>` +
				`</section>{{end}}`)},
		"search.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="suche">` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>` +
				`</section>{{end}}`)},
		"shop.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="katalog">` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>` +
				`</section>{{end}}`)},
	}
}

// bausteinVorrichtung baut, was jede Prüfung hier teilt: eine gewanderte
// Datenbank, einen Handler über bausteinFS, eine Website, einen Textbaustein
// „kontakt" mit einem Markdown-Rumpf, eine Felddefinition „telefon" daran und
// deren Wert.
func bausteinVorrichtung(t *testing.T) (*Handler, *db.DB, *domain.Website) {
	t.Helper()
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
	if err := bausteine.SetFields(ctx, ws.ID, sn.ID, roh); err != nil {
		t.Fatalf("SetFields: %v", err)
	}
	return h, database, ws
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
//   - .Page.FieldList bleibt leer — die Seite hat keine eigenen Felder, und das
//     Feld des Textbausteins darf dort nicht auftauchen.
func TestBausteinfelderErreichenDasTheme(t *testing.T) {
	h, database, ws := bausteinVorrichtung(t)
	seedPage(t, database, ws.ID, "Kontakt", "kontakt-seite", "# Kontakt", "published")

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
		t.Errorf("the field value is not in the page:\n%s", body)
	}
	if !strings.Contains(body, "<strong>da</strong>") {
		t.Errorf("the snippet's Markdown body is missing:\n%s", body)
	}
	if !strings.Contains(body, `<p class="eigene">0</p>`) {
		t.Errorf(".Page.FieldList ist nicht leer — ein Feld des Textbausteins ist "+
			"in den Seitenweg gelaufen, und genau das sähe man sonst erst im Browser:\n%s", body)
	}
}

// TestSnippetsBleibtTemplateHTML hält SNIP-05 fest: .Site.Snippets ändert
// seinen Typ nicht, und ein Textbaustein ohne eine einzige Felddefinition
// bekommt trotzdem seinen Eintrag in .Site.SnippetFields — eine leere Karte
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
	werte, ok := site.SnippetFields["kontakt"]
	if !ok {
		t.Fatal(".Site.SnippetFields hat keinen Eintrag für einen Textbaustein ohne Felder — " +
			"ein Theme, das durch ihn hindurchgreift, scheitert dann auf der Anfrage eines Besuchers")
	}
	if werte == nil {
		t.Error(".Site.SnippetFields carries a nil map instead of an empty one")
	}
	if len(werte) != 0 {
		t.Errorf(".Site.SnippetFields carries %d values, expected none", len(werte))
	}
	if _, ok := site.SnippetList["kontakt"]; ok {
		t.Error(".Site.SnippetList trägt einen Eintrag ohne einen einzigen gefüllten Wert — " +
			"field.List gibt keinen leeren heraus")
	}
}

// TestBausteinfelderAufMehrerenRouten ist die Hälfte, die ein grep nicht geben
// kann — und der Grund, warum sie hier steht, ist der Fehler, den Phase 7
// zweimal vorgeführt hat.
//
// Das Zählgatter dieses Plans weist nach, dass ausserhalb von fillSnippets
// keine Zuweisung an .Site.Snippets überlebt hat. Es weist nicht nach, dass
// jede Route die neue Funktion auch aufruft: eine Route, die das Füllen
// schlicht ganz vergässe, käme durch das grep-Gatter ohne Weiteres hindurch.
// Und sie fiele nirgends auf. Die Seite erscheint, der Status ist 200, kein
// Fehler wird protokolliert, {{index .Site.SnippetFields …}} des Themes gibt
// nichts heraus — der ganze Befund ist eine leere Stelle auf einer Art von
// Seite, und gesehen wird sie von einem Besucher.
//
// Drei Routen von drei verschiedenen Zuschnitten, weil die zwölf umgestellten
// Stellen zwei verschiedene Formen hatten: eine, die den Rendered bereits in
// der Hand hält (tag.go), und eine, die ihn erst laden muss (shop.go). Die
// Suche kommt dazu, weil sie eine eigene Ansicht zeichnet und dieselbe Zusage
// tragen muss.
func TestBausteinfelderAufMehrerenRouten(t *testing.T) {
	ctx := context.Background()
	h, database, ws := bausteinVorrichtung(t)

	schlagworte := term.NewStore(database)
	h.SetTermStore(schlagworte)

	beitragHTML, err := page.RenderMarkdown("Ein Beitrag über Möbel.")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	beitrag, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Beitrag", Slug: "beitrag",
		Markdown: "Ein Beitrag über Möbel.", HTML: beitragHTML,
		Status: "published", Kind: "post",
	})
	if err != nil {
		t.Fatalf("Beitrag anlegen: %v", err)
	}
	if err := schlagworte.SetForPage(ctx, ws.ID, beitrag.ID, []string{"Moebel"}); err != nil {
		t.Fatalf("Schlagwort setzen: %v", err)
	}
	// Die Kennung wird abgeleitet, nicht mitgebracht — deshalb hier
	// nachgelesen statt geraten.
	var schlagwortSlug string
	if err := database.Read.QueryRowContext(ctx,
		`SELECT slug FROM terms WHERE website_id = $1 LIMIT 1`, ws.ID).Scan(&schlagwortSlug); err != nil {
		t.Fatalf("Schlagwortkennung lesen: %v", err)
	}

	shopSite(t, h, database, ws)
	if _, err := shop.NewStore(database).Create(ctx, &shop.Product{
		WebsiteID: ws.ID, Slug: "tisch", Title: "Tisch",
		PriceGross: 4900, TaxRate: money.RateStandard, Status: "published",
	}); err != nil {
		t.Fatalf("Produkt anlegen: %v", err)
	}

	routen := []struct {
		name   string
		datei  string
		ziel   string
		fahren func(http.ResponseWriter, *http.Request) error
	}{
		{
			name:  "Schlagwortarchiv",
			datei: "tag.go",
			ziel:  "/tag/" + schlagwortSlug,
			fahren: func(w http.ResponseWriter, r *http.Request) error {
				r.SetPathValue("slug", schlagwortSlug)
				return h.HandleTag(w, r)
			},
		},
		{
			name:   "Suche",
			datei:  "search.go",
			ziel:   "/suche?q=Beitrag",
			fahren: h.HandleSearch,
		},
		{
			name:  "Katalog",
			datei: "shop.go",
			ziel:  "/shop",
			fahren: func(w http.ResponseWriter, r *http.Request) error {
				return h.HandleShop(w, r, ws, "")
			},
		},
	}

	for _, route := range routen {
		t.Run(route.name, func(t *testing.T) {
			rec, err := request(route.fahren, ws, "GET", route.ziel)
			if err != nil {
				t.Fatalf("%s: %v", route.name, err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: Status = %d, erwartet 200\n%s", route.name, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
				t.Errorf("%s (%s): der Feldwert des Textbausteins fehlt — diese Route "+
					"füllt .Site.SnippetFields nicht, und im Betrieb wäre der ganze "+
					"Befund eine leere Stelle auf genau dieser Art von Seite:\n%s",
					route.name, route.datei, body)
			}
		})
	}
}

// Die Gegenprobe zum Zählgatter: eine Route, die fillSnippets gar nicht ruft.
//
// Das grep-Gatter des Plans weist nach, dass ausserhalb von fillSnippets keine
// Zuweisung überlebt hat. Es kann die entgegengesetzte Lücke nicht sehen — eine
// Route, die weder das eine noch das andere tut, kommt hindurch, ohne dass
// irgendwo etwas fehlt. Drei taten es: renderNotFound, serveShareError und
// HandleMaintenance. Alle drei zeichnen die echte Vorlage des Themes, und alle
// drei sind Seiten, auf denen ein Besucher den Kontakt sucht — die eine
// gefundene Adresse ist falsch, die Website ist gerade weg, der Vorschaulink
// ist abgelaufen. Der Fussteil war dort leer, ohne Fehler und ohne Eintrag im
// Protokoll.
//
// Die .Site.Snippets-Hälfte war schon vorher offen; die beiden neuen Mitglieder
// erbten die Lücke am Tag ihrer Einführung.
func TestBausteinfelderAufDenRoutenOhneSeite(t *testing.T) {
	h, database, ws := bausteinVorrichtung(t)
	_ = database

	t.Run("404", func(t *testing.T) {
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			return h.renderNotFound(w, r, ws)
		}, ws, "GET", "/gibtesnicht")
		if err != nil {
			t.Fatalf("renderNotFound: %v", err)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "nichts gefunden") {
			t.Fatalf("the theme's 404 view was not drawn:\n%s", body)
		}
		if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
			t.Errorf("the 404 page does not carry the snippet surface:\n%s", body)
		}
	})

	t.Run("abgelaufener Vorschaulink", func(t *testing.T) {
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			return h.serveShareError(w, r, ws, sharelink.ErrExpired)
		}, ws, "GET", "/s/abgelaufen")
		if err != nil {
			t.Fatalf("serveShareError: %v", err)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "nichts gefunden") {
			t.Fatalf("the theme's 404 view was not drawn:\n%s", body)
		}
		if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
			t.Errorf("die Seite zum abgelaufenen Vorschaulink trägt die "+
				"Textbausteinfläche nicht:\n%s", body)
		}
	})

	t.Run("Wartung", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "demo.test"
		req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
		rec := httptest.NewRecorder()
		h.HandleMaintenance(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("Status = %d, erwartet 503", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "gleich zurück") {
			t.Fatalf("the theme's maintenance view was not drawn:\n%s", body)
		}
		if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
			t.Errorf("the maintenance page does not carry the snippet surface:\n%s", body)
		}
	})
}
