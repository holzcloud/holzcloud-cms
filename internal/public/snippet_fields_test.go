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

// snippetFS is a theme that prints both halves of a snippet: the body through
// .Site.Snippets and one single field through .Site.SnippetFields.
//
// And it prints the number of entries in .Page.FieldList alongside. That is the
// browser-visible half of D-03: the page has no fields of its own, and if
// something stands there, the snippet's field has run into the page path.
func snippetFS() fstest.MapFS {
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
		// The 404 view and the maintenance view print the same one line: they
		// are the pages on which a visitor is most likely to look for the
		// contact details, and exactly there the area was empty.
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">nichts gefunden</p>` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>{{end}}`)},
		"maintenance.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="wartung">gleich back</p>` +
				`<p class="telefon">{{index .Site.SnippetFields "kontakt" "telefon"}}</p>{{end}}`)},
		// Three views of three different shapes, all with the same one line:
		// the term archive and the post archive share list.html, the search and
		// the catalogue have one each of their own.
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

// snippetFixture builds what every check here shares: a migrated database, a
// handler over snippetFS, a website, a snippet "kontakt" with a markdown body,
// a field definition "telefon" on it and that field's value.
func snippetFixture(t *testing.T) (*Handler, *db.DB, *domain.Website) {
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

	loader := tmpl.NewLoader(dir, snippetFS(), nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), loader, nil, dir, snippetFS(), false)
	felder := field.NewStore(database)
	h.SetFieldStore(felder)

	ws := seedWebsite(t, database, "Test Site")

	// The snippet with a body of markdown, through the same chain as any page
	// content: goldmark, then bluemonday. There is no second chain, and that is
	// why the cast to template.HTML carries the same promise.
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

// TestSnippetFieldsReachTheTheme drives a snippet's field the whole way:
// definition in the table, value on the snippet, resolution on the way out,
// output through a template on a real public address.
//
// Four promises at once, and each falls on its own:
//   - the field value appears,
//   - the snippet's markdown body still appears,
//   - .Site.Snippets still carries template.HTML,
//   - .Page.FieldList stays empty — the page has no fields of its own, and the
//     snippet's field must not show up there.
func TestSnippetFieldsReachTheTheme(t *testing.T) {
	h, database, ws := snippetFixture(t)
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

// TestSnippetsStaysTemplateHTML holds SNIP-05 fast: .Site.Snippets does not
// change its type, and a snippet without a single field definition still gets
// its entry in .Site.SnippetFields — an empty map and not a missing key.
func TestSnippetsStaysTemplateHTML(t *testing.T) {
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

	loader := tmpl.NewLoader(dir, snippetFS(), nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), loader, nil, dir, snippetFS(), false)
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

// TestSnippetFieldsOnSeveralRoutes is the half a grep cannot give — and the
// reason it stands here is the fault phase 7 demonstrated twice.
//
// This plan's counting gate shows that no assignment to .Site.Snippets survived
// outside fillSnippets. It does not show that every route also calls the new
// function: a route that simply forgot the filling entirely would pass the grep
// gate without any trouble. And it would be noticed nowhere. The page appears,
// the status is 200, no error is logged, the theme's
// {{index .Site.SnippetFields …}} hands out nothing — the whole finding is an
// empty spot on one kind of page, and it is seen by a visitor.
//
// Three routes of three different shapes, because the twelve converted places
// had two different forms: one that already holds the Rendered in its hand
// (tag.go), and one that has to load it first (shop.go). The search comes in
// addition, because it draws a view of its own and has to carry the same
// promise.
func TestSnippetFieldsOnSeveralRoutes(t *testing.T) {
	ctx := context.Background()
	h, database, ws := snippetFixture(t)

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
	// The key is derived, not brought along — which is why it is read back here
	// rather than guessed.
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

// The counter-check to the counting gate: a route that does not call
// fillSnippets at all.
//
// The plan's grep gate shows that no assignment survived outside fillSnippets.
// It cannot see the opposite gap — a route that does neither the one nor the
// other passes through without anything being missing anywhere. Three did:
// renderNotFound, serveShareError and HandleMaintenance. All three draw the
// theme's real template, and all three are pages on which a visitor looks for
// the contact details — the one address found is wrong, the website is away
// just now, the preview link has expired. The footer was empty there, with no
// error and no entry in the log.
//
// The .Site.Snippets half was open before that; the two new members inherited
// the gap on the day they were introduced.
func TestBausteinfelderAufDenRoutenOhneSeite(t *testing.T) {
	h, database, ws := snippetFixture(t)
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
		if !strings.Contains(body, "gleich back") {
			t.Fatalf("the theme's maintenance view was not drawn:\n%s", body)
		}
		if !strings.Contains(body, `<p class="telefon">07721 123456</p>`) {
			t.Errorf("the maintenance page does not carry the snippet surface:\n%s", body)
		}
	})
}
