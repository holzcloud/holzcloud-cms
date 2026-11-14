package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

func shopRequest(ws *domain.Website, path string) *http.Request {
	req := httptest.NewRequest("GET", path, nil)
	req.Host = "demo.test"
	return req.WithContext(domain.WebsiteToContext(req.Context(), ws))
}

// TestAProductDescriptionResolvesItsValues is WR-03 of the phase 33 review.
// A product's description is the body text of the product page, rendered by
// the same Markdown pipeline as a page; a value in it is the value, on the
// product page, in the catalogue's summary and in the description for search
// engines — never the raw [[wert:…]].
func TestAProductDescriptionResolvesItsValues(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	fsys := testFS()
	fsys["layout.html"] = &fstest.MapFile{Data: []byte(
		`<html><head><meta name="description" content="{{.Meta.Description}}"></head>` +
			`<body>{{template "content" .}}</body></html>`)}
	fsys["product.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}<main>{{.Product.DescriptionHTML}}</main>{{end}}`)}
	fsys["shop.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}{{range .Catalogue.Products}}<p class="auszug">{{.Excerpt}}</p>{{end}}{{end}}`)}
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), tmpl.NewLoader(dir, fsys, nil, nil), nil, dir, fsys, false)
	store := value.NewStore(database)
	h.SetValueStore(store)

	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	setValue(t, store, ws.ID, "telefon", "044 123 45 67")

	const markdown = "Fragen? [[wert:telefon]] oder [Anruf](/kontakt/[[wert:telefon]])"
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shop.NewStore(database).Create(context.Background(), &shop.Product{
		WebsiteID: ws.ID, Slug: "tisch", Title: "Tisch", SKU: "T-1",
		DescriptionMarkdown: markdown, DescriptionHTML: html, Excerpt: page.Excerpt(markdown),
		PriceGross: 4950, TaxRate: money.RateStandard, Status: "published",
	}); err != nil {
		t.Fatal(err)
	}

	serve := func(path string, handle func(*httptest.ResponseRecorder) error) string {
		t.Helper()
		rec := httptest.NewRecorder()
		if err := handle(rec); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		return rec.Body.String()
	}
	product := serve("/shop/tisch", func(rec *httptest.ResponseRecorder) error {
		req := shopRequest(ws, "/shop/tisch")
		return h.HandleProduct(rec, req, ws, "tisch")
	})
	for _, want := range []string{
		"<p>Fragen? 044 123 45 67 oder", `href="/kontakt/044%20123%2045%2067"`,
		`<meta name="description" content="Fragen? 044 123 45 67 oder Anruf">`,
	} {
		if !strings.Contains(product, want) {
			t.Errorf("the product page lacks %q:\n%s", want, product)
		}
	}
	catalogue := serve("/shop", func(rec *httptest.ResponseRecorder) error {
		return h.HandleShop(rec, shopRequest(ws, "/shop"), ws, "")
	})
	if !strings.Contains(catalogue, `<p class="auszug">Fragen? 044 123 45 67 oder Anruf</p>`) {
		t.Errorf("the catalogue's summary lacks the value:\n%s", catalogue)
	}
	for name, body := range map[string]string{"product": product, "catalogue": catalogue} {
		if strings.Contains(body, "wert:") {
			t.Errorf("the %s page prints a raw marker:\n%s", name, body)
		}
	}
}
