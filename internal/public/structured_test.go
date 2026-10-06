package public

import (
	"context"
	"encoding/json"
	"html"
	"net/http/httptest"
	"path/filepath"
	"regexp"
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
)

const hostileTitle = `Tisch </script><script>alert(1)</script>`

// ldTheme is a theme that prints the structured data the way every shipped one
// does, so the tests look at what a browser would be handed.
func ldTheme() fstest.MapFS {
	fsys := testFS()
	ld := `<script type="application/ld+json">{{.Meta.StructuredData}}</script>`
	fsys["layout.html"] = &fstest.MapFile{Data: []byte(
		`<html><head>` + ld + `</head><body>{{template "content" .}}</body></html>`)}
	fsys["product.html"] = &fstest.MapFile{Data: []byte(
		`{{define "content"}}<main>{{.Product.Title}}</main>{{end}}`)}
	return fsys
}

func ldHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	fsys := ldTheme()
	loader := tmpl.NewLoader(dir, fsys, nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database), snippet.NewStore(database), loader, nil, dir, fsys, false)
	return h, database
}

var ldBlock = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

// graphOf pulls the one ld+json element out of a body and returns its nodes by
// type. It fails unless there is exactly one element, which is also the proof
// that a hostile title did not close it early.
func graphOf(t *testing.T, body string) map[string]map[string]any {
	t.Helper()
	m := ldBlock.FindAllStringSubmatch(body, -1)
	if len(m) != 1 {
		t.Fatalf("want exactly one ld+json element, got %d:\n%s", len(m), body)
	}
	var doc struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(m[0][1])), &doc); err != nil {
		t.Fatalf("ld+json is not valid: %v\n%s\nBODY %s", err, m[0][1], body)
	}
	out := map[string]map[string]any{}
	for _, n := range doc.Graph {
		out[n["@type"].(string)] = n
	}
	return out
}

func TestPostEmitsBlogPostingAndBreadcrumbs(t *testing.T) {
	h, database := ldHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	ws.OrgType = "Organization"
	ws.BlogBase = "aktuelles"
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: ws.ID, Title: hostileTitle, Slug: "beitrag", Markdown: "x", HTML: "<p>x</p>",
		Status: "published", Kind: "post",
	}); err != nil {
		t.Fatal(err)
	}
	rec := throughMiddleware(h.HandlePage, ws, "/beitrag", "beitrag")
	body := rec.Body.String()
	if strings.Count(body, "</script>") != 1 {
		t.Fatalf("hostile title closed the element:\n%s", body)
	}
	g := graphOf(t, body)
	post, ok := g["BlogPosting"]
	if !ok {
		t.Fatalf("no BlogPosting: %v", g)
	}
	if post["headline"] != hostileTitle {
		t.Errorf("headline = %v", post["headline"])
	}
	for _, k := range []string{"datePublished", "dateModified", "author"} {
		if post[k] == nil {
			t.Errorf("BlogPosting lacks %s: %v", k, post)
		}
	}
	crumbs, ok := g["BreadcrumbList"]
	if !ok {
		t.Fatalf("no BreadcrumbList: %v", g)
	}
	if n := len(crumbs["itemListElement"].([]any)); n != 3 {
		t.Errorf("%d crumbs, want 3 (Home, archive, post)", n)
	}
}

func TestProductEmitsProductOfferAndBreadcrumbs(t *testing.T) {
	h, database := ldHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	shopSite(t, h, database, ws)
	ctx := context.Background()
	products := shop.NewStore(database)
	stock := 0
	for _, tc := range []struct {
		slug  string
		stock *int
		want  string
	}{
		{"tisch", nil, "https://schema.org/InStock"},
		{"leer", &stock, "https://schema.org/OutOfStock"},
	} {
		if _, err := products.Create(ctx, &shop.Product{
			WebsiteID: ws.ID, Slug: tc.slug, Title: hostileTitle, SKU: "T-1",
			PriceGross: 4950, TaxRate: money.RateStandard, Status: "published", Stock: tc.stock,
		}); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "/shop/"+tc.slug, nil)
		req.Host = "demo.test"
		req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
		rec := httptest.NewRecorder()
		if err := h.HandleProduct(rec, req, ws, tc.slug); err != nil {
			t.Fatal(err)
		}
		body := rec.Body.String()
		if strings.Count(body, "</script>") != 1 {
			t.Fatalf("hostile title closed the element:\n%s", body)
		}
		g := graphOf(t, body)
		prod, ok := g["Product"]
		if !ok {
			t.Fatalf("no Product: %v", g)
		}
		if prod["name"] != hostileTitle || prod["sku"] != "T-1" {
			t.Errorf("product = %v", prod)
		}
		offer, _ := prod["offers"].(map[string]any)
		if offer["@type"] != "Offer" || offer["price"] != "49.50" || offer["priceCurrency"] != "CHF" || offer["availability"] != tc.want {
			t.Errorf("offer = %v; want availability %s", offer, tc.want)
		}
		crumbs, ok := g["BreadcrumbList"]
		if !ok {
			t.Fatalf("no BreadcrumbList: %v", g)
		}
		if n := len(crumbs["itemListElement"].([]any)); n != 3 {
			t.Errorf("%d crumbs, want 3", n)
		}
	}
}

func TestDecimalAmount(t *testing.T) {
	for in, want := range map[money.Amount]string{0: "0.00", 5: "0.05", 1250: "12.50", 123456: "1234.56", -250: "-2.50"} {
		if got := decimalAmount(in); got != want {
			t.Errorf("decimalAmount(%d) = %q; want %q", in, got, want)
		}
	}
}
