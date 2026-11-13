package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// Every public route that hands out page text, asked whether a value reaches
// it. The page itself is plan 01's; the routes here are the ones beside it — a
// footer a theme prints on every view, the feed, a plugin, the search — each of
// which builds its text on its own path and so has to be told on its own.

// valueRoutesFS is a theme whose layout prints the snippet "fuss" in a footer
// on every view, which is the realistic place for a version number: one line,
// at the bottom of every page, the 404 and the maintenance page included.
func valueRoutesFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><head><title>{{.Page.Title}}</title>` +
				`<meta name="description" content="{{.Meta.Description}}">` +
				`<meta property="og:description" content="{{.Meta.Description}}">` +
				`{{if .Meta.StructuredData}}<script type="application/ld+json">{{.Meta.StructuredData}}</script>{{end}}` +
				`</head>` +
				`<body>{{template "content" .}}` +
				`<footer class="fuss">{{index .Site.Snippets "fuss"}}</footer></body></html>`)},
		"home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<main class="home">{{.Page.ContentHTML}}</main>{{end}}`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article><p class="kurz">{{.Page.Excerpt}}</p>{{.Page.ContentHTML}}</article>{{end}}`)},
		"list.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="liste">{{range .Archive.Entries}}` +
				`<p class="auszug">{{.Excerpt}}</p>{{end}}</section>{{end}}`)},
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">nichts gefunden</p>{{end}}`)},
		"maintenance.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="wartung">gleich zurück</p>{{end}}`)},
		"search.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<section class="suche">{{range .Search.Results}}` +
				`<article><a href="{{.URL}}">{{.Title}}</a><p class="treffer">{{.Snippet}}</p></article>` +
				`{{end}}</section>{{end}}`)},
	}
}

// valueRoutesFixture is one website with the value v = "0.1.5", a snippet
// "fuss" that names it, and one published page that names it in a sentence, in
// a link address and through the snippet.
type valueRoutesFixture struct {
	h        *Handler
	database *db.DB
	ws       *domain.Website
	values   *value.Store
	page     *page.Page
}

const valueRoutesPage = "Alpha [[wert:v]]: neu\n\n" +
	"[Release](https://x.ch/v[[wert:v]])\n\n" +
	"[[snippet:fuss]]\n"

func newValueRoutesFixture(t *testing.T, body string) *valueRoutesFixture {
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
	fsys := valueRoutesFS()
	loader := tmpl.NewLoader(dir, fsys, nil, nil)
	h := NewHandler(page.NewStore(database), menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), loader, nil, dir, fsys, false)
	values := value.NewStore(database)
	h.SetValueStore(values)

	ws := seedWebsite(t, database, "Werte")
	html, err := page.RenderMarkdown("Stand [[wert:v]]")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(html, "[[wert:v]]") {
		t.Fatalf("the snippet does not carry the marker, so nothing here is proved: %q", html)
	}
	if _, err := snippet.NewStore(database).Create(ctx, ws.ID, "fuss", "Fuss", "Stand [[wert:v]]", html); err != nil {
		t.Fatalf("create the snippet: %v", err)
	}
	seedPage(t, database, ws.ID, "Alpha", "alpha", body, "published")
	pg, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "alpha")
	if err != nil || pg == nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	setValue(t, values, ws.ID, "v", "0.1.5")
	return &valueRoutesFixture{h: h, database: database, ws: ws, values: values, page: pg}
}

// moveValues sets every value's stamp of the fixture's website to one fixed
// moment in the future, so an assertion is about which date was chosen and not
// about how fast the machine is.
func (f *valueRoutesFixture) moveValues(t *testing.T, moved string) {
	t.Helper()
	if _, err := f.database.Write.Exec(
		`UPDATE site_values SET updated_at = $1 WHERE website_id = $2`, moved, f.ws.ID); err != nil {
		t.Fatalf("move the values' stamp: %v", err)
	}
}

// mustShowTheValue is the leak check: the value's text stands where the marker
// stood, and the marker is gone in either spelling — the literal one and the
// one a URL encoder makes of it.
func mustShowTheValue(t *testing.T, body string, wants ...string) {
	t.Helper()
	if strings.Contains(body, "[[wert:") || strings.Contains(body, "%5B%5Bwert:") ||
		strings.Contains(body, "%5b%5bwert:") {
		t.Errorf("a value marker reached a reader; this program's internal syntax is not text:\n%.1200s", body)
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%q is missing where the marker stood:\n%.1200s", want, body)
		}
	}
}

func TestNoPublicRouteLeaksAValueMarker(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	footer := `<footer class="fuss"><p>Stand 0.1.5</p>`

	t.Run("the page", func(t *testing.T) {
		body := servedBody(t, f.h, f.ws, "alpha")
		mustShowTheValue(t, body, "<p>Alpha 0.1.5: neu</p>", `href="https://x.ch/v0.1.5"`, footer)
	})

	t.Run("the home page", func(t *testing.T) {
		rec, err := request(f.h.HandleHome, f.ws, "GET", "/")
		if err != nil {
			t.Fatalf("HandleHome: %v", err)
		}
		mustShowTheValue(t, rec.Body.String(), "<p>Alpha 0.1.5: neu</p>", footer)
	})

	t.Run("the feed", func(t *testing.T) {
		rec, err := request(f.h.HandleFeed, f.ws, "GET", "/feed.xml")
		if err != nil {
			t.Fatalf("HandleFeed: %v", err)
		}
		mustShowTheValue(t, rec.Body.String(), "Alpha 0.1.5: neu", "Stand 0.1.5", "https://x.ch/v0.1.5")
	})

	pagesGet := func(t *testing.T, ctx context.Context) string {
		t.Helper()
		got, err := f.h.PagesForPlugin(ctx, f.ws.ID, plugin.PagesQuery{
			Op: plugin.OpPagesGet, Slug: "alpha", Limit: 10,
		})
		if err != nil {
			t.Fatalf("pages.get: %v", err)
		}
		if len(got.Pages) != 1 {
			t.Fatalf("pages.get returned %d pages, want 1", len(got.Pages))
		}
		return got.Pages[0].HTML
	}

	t.Run("a plugin, with a request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://demo.test/alpha", nil)
		req = req.WithContext(domain.WebsiteToContext(req.Context(), f.ws))
		mustShowTheValue(t, pagesGet(t, withRequest(req.Context(), req)),
			"<p>Alpha 0.1.5: neu</p>", "<p>Stand 0.1.5</p>", "https://x.ch/v0.1.5")
	})

	t.Run("a plugin, without a request", func(t *testing.T) {
		mustShowTheValue(t, pagesGet(t, context.Background()),
			"<p>Alpha 0.1.5: neu</p>", "<p>Stand 0.1.5</p>", "https://x.ch/v0.1.5")
	})

	t.Run("the 404 page", func(t *testing.T) {
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			return f.h.renderNotFound(w, r, f.ws)
		}, f.ws, "GET", "/gibtesnicht")
		if err != nil {
			t.Fatalf("renderNotFound: %v", err)
		}
		if !strings.Contains(rec.Body.String(), "nichts gefunden") {
			t.Fatalf("the theme's 404 view was not drawn:\n%s", rec.Body.String())
		}
		mustShowTheValue(t, rec.Body.String(), footer)
	})

	t.Run("the maintenance page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "demo.test"
		req = req.WithContext(domain.WebsiteToContext(req.Context(), f.ws))
		rec := httptest.NewRecorder()
		f.h.HandleMaintenance(rec, req)
		if !strings.Contains(rec.Body.String(), "gleich zurück") {
			t.Fatalf("the theme's maintenance view was not drawn:\n%s", rec.Body.String())
		}
		mustShowTheValue(t, rec.Body.String(), footer)
	})

	t.Run("the share-link preview", func(t *testing.T) {
		share := sharelink.New([]byte("share-secret-share-secret-share!"))
		f.h.SetShareSigners(share, sharelink.New([]byte("unlock-secret-unlock-secret-unl!")))
		token := share.Token(f.page.ID, time.Now().Add(time.Hour))
		rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
			r.SetPathValue("token", token)
			return f.h.HandleShareLink(w, r)
		}, f.ws, "GET", sharelink.Path(token))
		if err != nil {
			t.Fatalf("HandleShareLink: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("share link status = %d, want 200:\n%s", rec.Code, rec.Body.String())
		}
		mustShowTheValue(t, rec.Body.String(), "<p>Alpha 0.1.5: neu</p>", footer)
	})

	t.Run("the search page", func(t *testing.T) {
		rec, err := request(f.h.HandleSearch, f.ws, "GET", "/suche?q=neu")
		if err != nil {
			t.Fatalf("HandleSearch: %v", err)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `class="treffer"`) {
			t.Fatalf("the search found nothing, so nothing here is proved:\n%s", body)
		}
		mustShowTheValue(t, body, "Alpha 0.1.5: <mark>neu</mark>", footer)
	})

	t.Run("a plugin's search", func(t *testing.T) {
		got, err := f.h.PagesForPlugin(context.Background(), f.ws.ID, plugin.PagesQuery{
			Op: plugin.OpPagesSearch, Query: "neu", Limit: 10,
		})
		if err != nil {
			t.Fatalf("pages.search: %v", err)
		}
		if len(got.Pages) != 1 {
			t.Fatalf("pages.search returned %d pages, want 1", len(got.Pages))
		}
		mustShowTheValue(t, got.Pages[0].Snippet, "Alpha 0.1.5: <mark>neu</mark>")
	})
}

// A search passage is cut at a token boundary, and the boundary can fall inside
// a marker. What is left of it must not reach the reader either: neither the
// opening half at the end of the passage nor the closing half at its start.
func TestASearchPassageCutInsideAMarkerShowsNoPieceOfIt(t *testing.T) {
	// The passage is twenty tokens. The marker is four of them — wert, version,
	// holzcloud, cms — so seventeen words after "Gamma" put its first token
	// inside the passage and its last outside, and sixteen words before
	// "Epsilon" do the same from the other end. The precondition below says so
	// rather than trusting the arithmetic.
	f := newValueRoutesFixture(t, "Gamma "+strings.Repeat("Wort ", 17)+
		"[[wert:version-holzcloud-cms]] Delta "+strings.Repeat("Wort ", 16)+"Epsilon")
	setValue(t, f.values, f.ws.ID, "version-holzcloud-cms", "0.1.5")

	for _, tc := range []struct{ name, query, cut string }{
		{"the opening half at the end", "Gamma", "[["},
		{"the closing half at the start", "Epsilon", "]]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := page.NewStore(f.database).SearchPages(context.Background(), f.ws.ID, tc.query, false, 10)
			if err != nil || len(hits) != 1 {
				t.Fatalf("SearchPages(%q) = %d hits, %v; want 1", tc.query, len(hits), err)
			}
			raw := hits[0].Raw
			if !strings.Contains(raw, tc.cut) || strings.Contains(raw, "[[wert:version-holzcloud-cms]]") {
				t.Fatalf("the passage is not cut inside the marker, so nothing here is proved: %q", raw)
			}

			rec, err := request(f.h.HandleSearch, f.ws, "GET", "/suche?q="+tc.query)
			if err != nil {
				t.Fatalf("HandleSearch: %v", err)
			}
			body := rec.Body.String()
			start := strings.Index(body, `class="treffer"`)
			if start < 0 {
				t.Fatalf("the search found nothing:\n%s", body)
			}
			hit := body[start:]
			if end := strings.Index(hit, "</p>"); end >= 0 {
				hit = hit[:end]
			}
			for _, piece := range []string{"[[", "]]", "wert:", "holzcloud-cms"} {
				if strings.Contains(hit, piece) {
					t.Errorf("the hit shows %q, a piece of the marker:\n%s", piece, hit)
				}
			}
		})
	}
}

// TestChangingAValueMovesTheFeed is VAL-04 on the feed's two dates, in the
// shape of TestChangingAnAlbumMovesTheFeedsDates: the entries would carry the
// new text and the ETag would move with them, but a reader trusting <updated>,
// or a cache sending only a date, would keep the old version.
func TestChangingAValueMovesTheFeed(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	setValue(t, f.values, f.ws.ID, "v", "0.1.6")
	moved := "2031-04-05T06:07:08Z"
	f.moveValues(t, moved)

	rec, err := request(f.h.HandleFeed, f.ws, "GET", "/feed.xml")
	if err != nil {
		t.Fatalf("HandleFeed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alpha 0.1.6: neu") {
		t.Errorf("the feed does not carry the changed value:\n%s", body)
	}
	if !strings.Contains(body, "<updated>"+moved+"</updated>") {
		t.Errorf("the feed's <updated> does not carry the value's stamp %s — a reader keeps the old version:\n%s",
			moved, body)
	}
	want, err := time.Parse(time.RFC3339, moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rec.Header().Get("Last-Modified"); got != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the value's own stamp %q", got, want.UTC().Format(http.TimeFormat))
	}
}

// TestAThemePrintedSnippetFollowsTheValue is the case the page's own value pass
// cannot see: the page's text names no value at all, only the footer the theme
// prints does. A value change has to reach that footer and move the page's
// Last-Modified with it.
func TestAThemePrintedSnippetFollowsTheValue(t *testing.T) {
	f := newValueRoutesFixture(t, "Nur Text, kein Wert.")
	before := plainGet(t, f.h, f.ws, "alpha")
	mustShowTheValue(t, before.Body.String(), `<footer class="fuss"><p>Stand 0.1.5</p>`)

	setValue(t, f.values, f.ws.ID, "v", "0.1.6")
	moved := "2031-04-05T06:07:08Z"
	f.moveValues(t, moved)

	rec := conditionalGet(t, f.h, f.ws, "alpha", "", before.Header().Get("Last-Modified"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a browser holding only the date got %d; want 200 — it would keep the old footer", rec.Code)
	}
	mustShowTheValue(t, rec.Body.String(), `<footer class="fuss"><p>Stand 0.1.6</p>`)
	want, err := time.Parse(time.RFC3339, moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rec.Header().Get("Last-Modified"); got != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the value's own stamp %q", got, want.UTC().Format(http.TimeFormat))
	}
}
