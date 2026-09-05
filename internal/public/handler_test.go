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
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// testFS is a minimal public template with the structure every shipped theme
// has: layout.html is the document, each view supplies "content".
func testFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><head><title>{{.Page.Title}}</title></head>` +
				`<body>{{menu .Menus "main"}}{{template "content" .}}</body></html>`)},
		"home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<main class="home">{{.Page.Title}}{{.Page.ContentHTML}}</main>{{end}}`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article>{{.Page.Title}}{{.Page.ContentHTML}}</article>{{end}}`)},
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">nichts gefunden</p>{{end}}`)},
		"search.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="suche">{{range .Search.Results}}` +
				`<article><a href="{{.URL}}">{{.Title}}</a>{{.Snippet}}</article>{{end}}</section>{{end}}`)},
		"style.css": &fstest.MapFile{Data: []byte(`body{color:red}`)},
	}
}

// newTestHandler builds a handler over a real migrated SQLite database and
// returns it together with the stores, so tests can seed content directly.
func newTestHandler(t *testing.T) (*Handler, *db.DB) {
	t.Helper()

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)

	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	fsys := testFS()
	loader := tmpl.NewLoader(dir, fsys, nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database), snippet.NewStore(database), loader, nil, dir, fsys, false)
	return h, database
}

// seedWebsite inserts a website and returns it, ready to place in a context.
func seedWebsite(t *testing.T, database *db.DB, name string) *domain.Website {
	t.Helper()
	ws, err := domain.NewStore(database).CreateWebsite(context.Background(), name, "beschreibung")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	return ws
}

// seedPage inserts a page with the given status.
func seedPage(t *testing.T, database *db.DB, websiteID int64, title, slug, body, status string) {
	t.Helper()
	html, err := page.RenderMarkdown(body)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     title,
		Slug:      slug,
		Markdown:  body,
		HTML:      html,
		Status:    status,
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
}

// request runs one handler with a website already resolved into the context.
func request(h func(http.ResponseWriter, *http.Request) error, ws *domain.Website, method, target string) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "demo.test"
	if ws != nil {
		req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	}
	rec := httptest.NewRecorder()
	err := h(rec, req)
	return rec, err
}

func TestHandleHomeRendersPublishedPage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Startseite", "home", "# Willkommen", "published")

	rec, err := request(h.HandleHome, ws, "GET", "/")
	if err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	body := rec.Body.String()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if !strings.Contains(body, "<html>") || !strings.Contains(body, `class="home"`) {
		t.Errorf("home was not rendered through layout + home view:\n%s", body)
	}
	if !strings.Contains(body, "Willkommen") {
		t.Errorf("page content missing:\n%s", body)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("no ETag was set")
	}
}

// The single most important guarantee of the public side.
func TestDraftPagesNeverReachThePublic(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Geheim", "geheim", "streng geheim", "draft")

	rec, err := request(h.HandlePage, ws, "GET", "/geheim")
	if err != nil {
		t.Fatalf("HandlePage: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("draft page returned status %d; want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "streng geheim") {
		t.Errorf("draft content leaked:\n%s", rec.Body.String())
	}
}

// With no page called "home", the oldest published page stands in.
func TestHandleHomeFallsBackToFirstPublishedPage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "x", "draft")
	seedPage(t, database, ws.ID, "Erste", "erste", "inhalt", "published")

	rec, err := request(h.HandleHome, ws, "GET", "/")
	if err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "Erste") {
		t.Errorf("expected the first published page as home:\n%s", rec.Body.String())
	}
}

func TestHandleHomeWithoutPagesRenders404View(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")

	rec, err := request(h.HandleHome, ws, "GET", "/")
	if err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nichts gefunden") {
		t.Errorf("expected the styled 404 view:\n%s", rec.Body.String())
	}
}

// Without a resolved website the handler must not fall through to some other
// site's content.
func TestHandlersRefuseUnresolvedHost(t *testing.T) {
	h, _ := newTestHandler(t)

	for name, fn := range map[string]func(http.ResponseWriter, *http.Request) error{
		"home":    h.HandleHome,
		"page":    h.HandlePage,
		"sitemap": h.HandleSitemap,
		"robots":  h.HandleRobots,
	} {
		rec, err := request(fn, nil, "GET", "/")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d; want 404", name, rec.Code)
		}
	}
}

func TestConditionalRequestsReturn304(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Startseite", "home", "inhalt", "published")

	first, err := request(h.HandleHome, ws, "GET", "/")
	if err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	etag := first.Header().Get("ETag")

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	if err := h.HandleHome(rec, req); err != nil {
		t.Fatalf("HandleHome (conditional): %v", err)
	}

	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d; want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 response carried a body: %q", rec.Body.String())
	}
}

func TestHandleTemplateAssetServesAndRejectsTraversal(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")

	rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
		r.SetPathValue("path", "style.css")
		return h.HandleTemplateAsset(w, r)
	}, ws, "GET", "/t/style.css")
	if err != nil {
		t.Fatalf("HandleTemplateAsset: %v", err)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "color:red") {
		t.Errorf("asset not served: status %d body %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Errorf("Content-Type = %q; want text/css", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("assets must be served with nosniff")
	}

	// ServeMux hands percent-encoded traversal through already decoded.
	for _, p := range []string{"../../etc/passwd", "../test.sqlite"} {
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			r.SetPathValue("path", p)
			return h.HandleTemplateAsset(w, r)
		}, ws, "GET", "/t/x")
		if err != nil {
			t.Fatalf("HandleTemplateAsset(%q): %v", p, err)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("traversal %q returned status %d; want 404", p, rec.Code)
		}
	}
}

// Eine geänderte Vorlage muss ankommen.
//
// Bis Fassung 1.8 ging jedes Vorlagen-Asset mit `max-age=31536000, immutable`
// hinaus, auf einer Adresse, die sich nie ändert. `immutable` heisst: frag nicht
// nach, auch nicht beim Neuladen. Ein korrigiertes Stylesheet erreichte damit
// niemanden, der die Seite schon einmal geöffnet hatte — ein Jahr lang. Der Test
// hält beide Hälften fest: die kurze Frist ohne Version und das lange
// Versprechen nur dort, wo die Adresse es einlöst.
func TestVorlagenAssetsBleibenErreichbar(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")

	hole := func(url string, inm string) *httptest.ResponseRecorder {
		t.Helper()
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			r.SetPathValue("path", "style.css")
			if inm != "" {
				r.Header.Set("If-None-Match", inm)
			}
			return h.HandleTemplateAsset(w, r)
		}, ws, "GET", url)
		if err != nil {
			t.Fatalf("HandleTemplateAsset(%s): %v", url, err)
		}
		return rec
	}

	// Ohne Version: kurz und nachfragbar.
	rec := hole("/t/style.css", "")
	if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q; auf einer festen Adresse darf nichts immutable sein", cc)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ohne ETag ist die Rückfrage so teuer wie ein neuer Abruf")
	}

	// Die Rückfrage kostet dann nichts mehr.
	rec = hole("/t/style.css", etag)
	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d; want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 mit Körper: %q", rec.Body.String())
	}

	// Mit Version im Verweis hält der Aufrufer das Versprechen, also gilt es.
	rec = hole("/t/style.css?v=1.8", "")
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q; eine versionierte Adresse darf lange liegen", cc)
	}
}

func TestSitemapListsOnlyPublishedPages(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Startseite", "home", "x", "published")
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "x", "published")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "x", "draft")

	rec, err := request(h.HandleSitemap, ws, "GET", "/sitemap.xml")
	if err != nil {
		t.Fatalf("HandleSitemap: %v", err)
	}
	body := rec.Body.String()

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("Content-Type = %q; want application/xml", ct)
	}
	for _, want := range []string{
		"<loc>http://demo.test/</loc>",
		"<loc>http://demo.test/ueber-uns</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %s:\n%s", want, body)
		}
	}
	// Die Startseite steht als "/" schon oben. Ihre zweite Adresse gehört
	// nicht dazu — siehe TestTheStartPageHasOneAddress.
	if strings.Contains(body, "/home</loc>") {
		t.Errorf("die Startseite steht zweimal im Sitemap:\n%s", body)
	}
	if strings.Contains(body, "entwurf") {
		t.Errorf("draft page listed in the sitemap:\n%s", body)
	}
	if !strings.Contains(body, "<lastmod>") {
		t.Errorf("no lastmod in the sitemap:\n%s", body)
	}
}

// The scheme must come from configuration, not from a client-supplied header,
// because these URLs are handed to search engines.
func TestSitemapSchemeFollowsConfiguration(t *testing.T) {
	h, database := newTestHandler(t)
	h.secure = true
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Startseite", "home", "x", "published")

	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	req.Host = "demo.test"
	req.Header.Set("X-Forwarded-Proto", "gopher")
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	if err := h.HandleSitemap(rec, req); err != nil {
		t.Fatalf("HandleSitemap: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<loc>https://demo.test/") {
		t.Errorf("expected https from configuration:\n%s", body)
	}
	if strings.Contains(body, "gopher") {
		t.Errorf("a forwarded scheme header influenced the sitemap:\n%s", body)
	}
}

func TestRobotsDisallowsAdminAndPointsAtSitemap(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")

	rec, err := request(h.HandleRobots, ws, "GET", "/robots.txt")
	if err != nil {
		t.Fatalf("HandleRobots: %v", err)
	}
	body := rec.Body.String()

	for _, want := range []string{"User-agent: *", "Disallow: /admin/", "Sitemap: http://demo.test/sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q; want text/plain", ct)
	}
}

// Two websites in one database must not see each other's pages.
func TestPagesAreScopedToTheResolvedWebsite(t *testing.T) {
	h, database := newTestHandler(t)
	a := seedWebsite(t, database, "Site A")
	b := seedWebsite(t, database, "Site B")
	seedPage(t, database, a.ID, "Nur A", "exklusiv", "inhalt von a", "published")

	rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
		r.SetPathValue("slug", "exklusiv")
		return h.HandlePage(w, r)
	}, b, "GET", "/exklusiv")
	if err != nil {
		t.Fatalf("HandlePage: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "inhalt von a") {
		t.Errorf("another website's page leaked:\n%s", rec.Body.String())
	}
}

// A menu link must not be able to carry script into a rendered page.
func TestRenderedMenuIsEscaped(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Startseite", "home", "x", "published")

	menuStore := menu.NewStore(database)
	m, err := menuStore.CreateMenu(context.Background(), ws.ID, "Haupt", "main", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if _, err := menuStore.CreateItem(context.Background(), m.ID, nil,
		`<script>alert(1)</script>`, "url", "javascript:alert(2)", nil, 0); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	rec, err := request(h.HandleHome, ws, "GET", "/")
	if err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	body := rec.Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("menu title was not escaped:\n%s", body)
	}
	if strings.Contains(body, `href="javascript:`) {
		t.Errorf("javascript: URL was rendered as a link:\n%s", body)
	}
}

// TestTheStartPageHasOneAddress covers the redirect and the sitemap entry that
// go with it: the start page lives at the root of its language, and /home is
// the address it used to be reachable under a second time — with its own
// canonical link and its own line in the sitemap, which is how one page turns
// into two for a search engine.
func TestTheStartPageHasOneAddress(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	if err := domain.NewStore(database).UpdateSettings(context.Background(), ws.ID, domain.Settings{
		Locale: "de", ExtraLocales: "fr", TimeZone: "Europe/Zurich", OfflineMode: "notfound",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	ws, err := domain.NewStore(database).GetWebsite(context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	seedPage(t, database, ws.ID, "Startseite", page.HomeSlug, "# Willkommen", "published")
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: ws.ID, Title: "Accueil", Slug: page.HomeSlug, Status: "published",
		Markdown: "Bienvenue.", Locale: "fr",
	}); err != nil {
		t.Fatalf("die französische Startseite: %v", err)
	}

	for _, c := range []struct{ locale, want string }{{"", "/"}, {"fr", "/fr"}} {
		req := httptest.NewRequest("GET", "/home", nil)
		req.Host = "demo.test"
		ctx := domain.WebsiteToContext(req.Context(), ws)
		if c.locale != "" {
			ctx = context.WithValue(ctx, localeKey{}, c.locale)
		}
		req = req.WithContext(ctx)
		req.SetPathValue("slug", page.HomeSlug)

		rec := httptest.NewRecorder()
		if err := h.HandlePage(rec, req); err != nil {
			t.Fatalf("HandlePage %q: %v", c.locale, err)
		}
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("/home in der Sprache %q antwortet mit %d statt 301", c.locale, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Errorf("/home in der Sprache %q leitet nach %q statt nach %q", c.locale, got, c.want)
		}
	}

	rec, err := request(h.HandleSitemap, ws, "GET", "/sitemap.xml")
	if err != nil {
		t.Fatalf("HandleSitemap: %v", err)
	}
	if strings.Contains(rec.Body.String(), "/home") {
		t.Errorf("die Startseite steht ein zweites Mal im Sitemap:\n%s", rec.Body.String())
	}
}
