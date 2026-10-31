package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
)

// setCookies returns every Set-Cookie header value of a response.
func setCookies(rec *httptest.ResponseRecorder) []string {
	return rec.Result().Header.Values("Set-Cookie")
}

// The price-view cookie is the one cookie the shop sets for a choice the
// visitor makes. Driven through the real router with the installation
// configured as served over TLS but the request arriving as plain HTTP, which
// is what a TLS-ending proxy passes on: the flag must follow the installation,
// not the request.
func TestPriceViewCookieEndToEnd(t *testing.T) {
	handler, _, database := testRouterWith(t, routerTweaks{shop: true, secure: true})
	ctx := context.Background()

	store := domain.NewStore(database)
	ws, err := store.CreateWebsite(ctx, "Laden", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if _, err := store.AddDomain(ctx, ws.ID, "shop.test", true); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE websites SET shop_base = 'shop', currency = 'CHF',
		 shipping_gross = 0, shipping_tax_bp = 810, price_display = 'both'
		 WHERE id = ?`, ws.ID); err != nil {
		t.Fatalf("switch the shop on: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Willkommen", Slug: "home", Markdown: "Hallo", HTML: "<p>Hallo</p>",
		Status: "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	get := httptest.NewRequest(http.MethodGet, "/", nil)
	get.Host = "shop.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: status %d, want 200", rec.Code)
	}
	if got := setCookies(rec); len(got) != 0 {
		t.Errorf("GET / set a cookie without the visitor doing anything: %q", got)
	}

	form := url.Values{"ansicht": {"business"}, "zurueck": {"/shop"}}
	post := httptest.NewRequest(http.MethodPost, "/preise", strings.NewReader(form.Encode()))
	post.Host = "shop.test"
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, post)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /preise: status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/shop" {
		t.Errorf("POST /preise: Location = %q, want /shop", loc)
	}
	got := setCookies(rec)
	if len(got) != 1 {
		t.Fatalf("POST /preise: want exactly one Set-Cookie, got %q", got)
	}
	c := got[0]
	if !strings.HasPrefix(c, shop.AudienceCookieName+"=business") {
		t.Errorf("cookie = %q, want %s=business", c, shop.AudienceCookieName)
	}
	for _, want := range []string{"Max-Age=2592000", "HttpOnly", "Secure", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q lacks %q", c, want)
		}
	}
}

// Fixed words of the fixture. The tests below look for them in the answers, so
// a page that is served but is not the page asked for does not pass.
const (
	cookieTestPagePassword = "geheimwort"
	cookieTestTagName      = "Werkstatt"
	cookieTestTagSlug      = "werkstatt"
	cookieTestSearchWord   = "Gartenzwerg"
	cookieTestProductSlug  = "tisch"
	cookieTestProductTitle = "Eichentisch"
	cookieTestHomeTitle    = "Startseite"
	cookieTestPostCount    = 12
)

// cookieTestSiteName is the name of the website seeded for one theme.
func cookieTestSiteName(slug string) string { return "Testsite " + slug }

// seedThemeSites creates one website per theme the program ships, each with
// the theme activated and a representative body of content: a start page, a
// content page, a protected page, twelve posts (so the archive has a second
// page at the default page size) of which three carry a tag, and a shop with
// two products that shows both price modes.
//
// Everything is fixed text; nothing depends on the clock.
func seedThemeSites(t *testing.T, database *db.DB, dataDir string) {
	t.Helper()
	ctx := context.Background()

	domains := domain.NewStore(database)
	templates := tmplmgr.NewStore(database, dataDir)
	pages := page.NewStore(database)
	terms := term.NewStore(database)
	products := shop.NewStore(database)
	// As small as seedUser makes them: the test needs a hash, not a work factor.
	params := auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	for _, bt := range tmpl.BuiltinTemplates {
		ws, err := domains.CreateWebsite(ctx, cookieTestSiteName(bt.Slug), "Beschreibung")
		if err != nil {
			t.Fatalf("%s: CreateWebsite: %v", bt.Slug, err)
		}
		if _, err := domains.AddDomain(ctx, ws.ID, bt.Slug+".test", true); err != nil {
			t.Fatalf("%s: AddDomain: %v", bt.Slug, err)
		}
		tp, err := templates.CreateBuiltin(ctx, bt.Name, bt.Slug)
		if err != nil {
			t.Fatalf("%s: CreateBuiltin: %v", bt.Slug, err)
		}
		if err := templates.ActivateForWebsite(ctx, ws.ID, tp.ID); err != nil {
			t.Fatalf("%s: ActivateForWebsite: %v", bt.Slug, err)
		}

		create := func(title, slug, kind string) *page.Page {
			p, err := pages.CreatePage(ctx, page.PageCreate{
				WebsiteID: ws.ID, Title: title, Slug: slug, Kind: kind, Status: "published",
				Markdown: "Text von " + title, HTML: "<p>Text von " + title + "</p>",
			})
			if err != nil {
				t.Fatalf("%s: CreatePage %s: %v", bt.Slug, slug, err)
			}
			return p
		}

		create(cookieTestHomeTitle, "home", "page")
		create(cookieTestSearchWord+" Geschichte", "ueber-uns", "page")
		secret := create("Geheime Seite", "geheim", "page")
		if err := pages.SetAccess(ctx, secret.ID, page.AccessUpdate{
			Protected: true, Password: cookieTestPagePassword, Hint: "Ein Hinweis",
		}, params); err != nil {
			t.Fatalf("%s: SetAccess: %v", bt.Slug, err)
		}

		for i := 1; i <= cookieTestPostCount; i++ {
			p := create(fmt.Sprintf("Beitrag %02d", i), fmt.Sprintf("beitrag-%02d", i), "post")
			if i <= 3 {
				if err := terms.SetForPage(ctx, ws.ID, p.ID, []string{cookieTestTagName}); err != nil {
					t.Fatalf("%s: SetForPage: %v", bt.Slug, err)
				}
			}
		}

		if _, err := database.Write.ExecContext(ctx,
			`UPDATE websites SET shop_base = 'shop', currency = 'CHF',
			 shipping_gross = 0, shipping_tax_bp = 810, price_display = 'both'
			 WHERE id = ?`, ws.ID); err != nil {
			t.Fatalf("%s: switch the shop on: %v", bt.Slug, err)
		}
		for _, p := range []struct{ slug, title string }{
			{cookieTestProductSlug, cookieTestProductTitle},
			{"stuhl", "Eichenstuhl"},
		} {
			if _, err := products.Create(ctx, &shop.Product{
				WebsiteID: ws.ID, Slug: p.slug, Title: p.title,
				PriceGross: 4900, TaxRate: money.RateStandard, Status: "published",
			}); err != nil {
				t.Fatalf("%s: create product %s: %v", bt.Slug, p.slug, err)
			}
		}
	}
}

// cookieFindings is the detector: what a response sets as a cookie. Empty
// means the response asked the browser to remember nothing.
//
// The matrix and the self-check both use it, so a weakened detector weakens
// both tests visibly.
func cookieFindings(rec *httptest.ResponseRecorder) []string {
	return setCookies(rec)
}

// publicGet is one address of the matrix.
type publicGet struct {
	path string
	// status is what the address answers. Several values only where the
	// program's own intent allows more than one (see the entry).
	status []int
	// body is a word the answer must contain, or empty.
	body string
}

// publicGetMatrix lists the public GET addresses a visitor reaches without
// doing anything. A later address is one added line.
//
// Statuses are exact. An address that answers 404 where content is expected
// would make the whole test vacuous, so there is no "anything".
var publicGetMatrix = []publicGet{
	{"/", []int{200}, cookieTestHomeTitle},
	{"/ueber-uns", []int{200}, cookieTestSearchWord},
	// The gate answers 401, not the page: a protected page is not served.
	{"/geheim", []int{401}, ""},
	{"/aktuelles", []int{200}, "Beitrag"},
	{"/aktuelles?seite=2", []int{200}, "Beitrag"},
	{"/tag/" + cookieTestTagSlug, []int{200}, "Beitrag 01"},
	// Without the plugin /suche would be a page slug nobody wrote and answer
	// 404, so the 200 shows the plugin runs. "Geschichte" occurs only in the
	// hit's title, not in the echoed query.
	{"/suche?q=" + cookieTestSearchWord, []int{200}, "Geschichte"},
	{"/gibt-es-nicht", []int{404}, ""},
	{"/shop", []int{200}, cookieTestProductTitle},
	{"/shop/" + cookieTestProductSlug, []int{200}, cookieTestProductTitle},
	{"/warenkorb", []int{200}, ""},
	// The fixture's basket is empty, and the checkout sends a visitor with
	// nothing to pay for back to the basket. Not a hole: /warenkorb above
	// is the page that shows the empty basket.
	{"/kasse", []int{303}, ""},
	{"/feed.xml", []int{200}, "Beitrag"},
	{"/sitemap.xml", []int{200}, "ueber-uns"},
	{"/robots.txt", []int{200}, ""},
	{"/llms.txt", []int{200}, ""},
	{"/t/style.css", []int{200}, ""},
	{"/bestellung/unbekannt", []int{404}, ""},
	{"/zahlung/zurueck/unbekannt", []int{404}, ""},
}

// lastMatrixRun holds the theme slugs the matrix ran, for the coverage test.
var lastMatrixRun []string

// runPublicGetMatrix visits every address of the matrix on one website per
// shipped theme and returns the theme slugs it ran.
func runPublicGetMatrix(t *testing.T) []string {
	t.Helper()
	handler, _, _ := testRouterWith(t, routerTweaks{
		shop: true, secure: true, searchPlugin: true, seed: seedThemeSites,
	})

	var ran []string
	for _, bt := range tmpl.BuiltinTemplates {
		ran = append(ran, bt.Slug)
		t.Run(bt.Slug, func(t *testing.T) {
			for _, g := range publicGetMatrix {
				req := httptest.NewRequest(http.MethodGet, g.path, nil)
				req.Host = bt.Slug + ".test"
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				t.Logf("GET %s -> %d", g.path, rec.Code)
				if !slices.Contains(g.status, rec.Code) {
					t.Errorf("%s GET %s: status %d, want one of %v", bt.Slug, g.path, rec.Code, g.status)
				}
				if g.body != "" && !strings.Contains(rec.Body.String(), g.body) {
					t.Errorf("%s GET %s: body lacks %q", bt.Slug, g.path, g.body)
				}
				if got := cookieFindings(rec); len(got) != 0 {
					t.Errorf("%s GET %s sets a cookie without the visitor doing anything: %q", bt.Slug, g.path, got)
				}
			}

			// A 200 from an error page would pass the status check above, so
			// the start page must show the website it belongs to.
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = bt.Slug + ".test"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			body := rec.Body.String()
			if !strings.Contains(body, cookieTestSiteName(bt.Slug)) && !strings.Contains(body, cookieTestHomeTitle) {
				t.Errorf("%s GET /: neither the website's name nor the start page's title is in the answer", bt.Slug)
			}
		})
	}
	return ran
}

// No public GET sets a cookie. Running through the real router with the real
// session middleware over every theme the program ships: a new template
// function, middleware or asset route that adds a Set-Cookie to a plain GET
// fails here, and not in a complaint.
func TestNoPublicGetSetsACookie(t *testing.T) {
	lastMatrixRun = runPublicGetMatrix(t)
}

// A ninth theme directory that is not listed cannot escape the cookie test.
func TestEveryShippedThemeIsInTheMatrix(t *testing.T) {
	entries, err := fs.ReadDir(staticFS, "templates/public")
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	var listed []string
	for _, bt := range tmpl.BuiltinTemplates {
		listed = append(listed, bt.Slug)
	}
	slices.Sort(dirs)
	slices.Sort(listed)
	if !slices.Equal(dirs, listed) {
		t.Fatalf("themes on disk %v differ from tmpl.BuiltinTemplates %v", dirs, listed)
	}

	ran := lastMatrixRun
	if ran == nil {
		// Run on its own (-run), so the matrix has not run before this.
		ran = runPublicGetMatrix(t)
	}
	slices.Sort(ran)
	if !slices.Equal(ran, listed) {
		t.Errorf("the matrix ran %v, the program ships %v", ran, listed)
	}
}

// postForm sends a form to the router the way a visitor's browser would.
func postForm(handler http.Handler, host, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// cookieNames lists the names in a set of Set-Cookie header values.
func cookieNames(headers []string) []string {
	var names []string
	for _, h := range headers {
		name, _, _ := strings.Cut(h, "=")
		names = append(names, name)
	}
	return names
}

// The matrix above is green because there is nothing to see, not because the
// detector cannot see: the three cookies a visitor can trigger show up in the
// same detector, on the same router and the same websites.
func TestVisitorTriggeredCookiesAreVisibleToTheDetector(t *testing.T) {
	handler, _, _ := testRouterWith(t, routerTweaks{
		shop: true, secure: true, searchPlugin: true, seed: seedThemeSites,
	})
	const host = "default.test"

	t.Run("price view", func(t *testing.T) {
		rec := postForm(handler, host, "/preise", url.Values{"ansicht": {"business"}, "zurueck": {"/shop"}})
		got := cookieFindings(rec)
		if rec.Code != http.StatusSeeOther || len(got) != 1 || cookieNames(got)[0] != shop.AudienceCookieName {
			t.Errorf("POST /preise: status %d, cookies %q, want 303 and exactly %s", rec.Code, got, shop.AudienceCookieName)
		}
	})

	t.Run("basket", func(t *testing.T) {
		rec := postForm(handler, host, "/warenkorb/hinzufuegen",
			url.Values{"artikel": {cookieTestProductSlug}, "menge": {"1"}})
		got := cookieFindings(rec)
		if rec.Code != http.StatusSeeOther || len(got) != 1 || cookieNames(got)[0] != shop.CartCookieName {
			t.Errorf("POST /warenkorb/hinzufuegen: status %d, cookies %q, want 303 and exactly %s",
				rec.Code, got, shop.CartCookieName)
		}
	})

	t.Run("unlock", func(t *testing.T) {
		rec := postForm(handler, host, "/freischalten",
			url.Values{"seite": {"geheim"}, "passwort": {cookieTestPagePassword}})
		got := cookieFindings(rec)
		if rec.Code != http.StatusSeeOther || len(got) != 1 || !strings.HasPrefix(cookieNames(got)[0], "hc_frei_") {
			t.Errorf("POST /freischalten: status %d, cookies %q, want 303 and exactly one hc_frei_ cookie", rec.Code, got)
		}
	})
}

// The detector itself: it flags a handler that sets a cookie on a GET, and
// does not flag one that sets none.
func TestCookieDetectorFlagsASetCookieOnAGet(t *testing.T) {
	run := func(h http.HandlerFunc) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec
	}

	setting := run(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "tracker", Value: "1"})
	})
	if got := cookieFindings(setting); len(got) == 0 {
		t.Error("the detector did not flag a GET that sets a cookie")
	}

	quiet := run(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if got := cookieFindings(quiet); len(got) != 0 {
		t.Errorf("the detector flagged a GET that sets nothing: %q", got)
	}
}
