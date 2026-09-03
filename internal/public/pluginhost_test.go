package public

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// The rule the whole plugin system rests on: a module is written by someone
// else, so "published only" has to hold in the host. A check inside a plugin
// is a promise; this is the guarantee.
func TestPluginsNeverSeeADraft(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")
	seedPage(t, database, ws.ID, "Fertig", "fertig", "Sichtbar.", "published")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "Geheim.", "draft")

	ctx := context.Background()

	list, _, err := listPages(t, h, ctx, ws.ID)
	if err != nil {
		t.Fatalf("pages.list: %v", err)
	}
	for _, p := range list {
		if p.Slug == "entwurf" {
			t.Fatal("der Entwurf steht in der Liste")
		}
	}

	got, err := h.PagesForPlugin(ctx, ws.ID, plugin.PagesQuery{
		Op: plugin.OpPagesGet, Slug: "entwurf", Limit: 10,
	})
	if err != nil {
		t.Fatalf("pages.get: %v", err)
	}
	if len(got.Pages) != 0 {
		t.Errorf("pages.get lieferte den Entwurf: %+v", got.Pages)
	}

	found, err := h.PagesForPlugin(ctx, ws.ID, plugin.PagesQuery{
		Op: plugin.OpPagesSearch, Query: "Geheim", Limit: 10,
	})
	if err != nil {
		t.Fatalf("pages.search: %v", err)
	}
	for _, p := range found.Pages {
		if p.Slug == "entwurf" {
			t.Error("die Suche lieferte den Entwurf")
		}
	}
}

// A plugin gets one site's pages, never another's — websites on one server are
// separate publications, not folders.
func TestPagesAreScopedToOneWebsite(t *testing.T) {
	h, database := newTestHandler(t)
	a := seedWebsite(t, database, "Erste")
	b := seedWebsite(t, database, "Zweite")
	seedPage(t, database, a.ID, "Nur bei A", "nur-a", "Text.", "published")
	seedPage(t, database, b.ID, "Nur bei B", "nur-b", "Text.", "published")

	list, _, err := listPages(t, h, context.Background(), a.ID)
	if err != nil {
		t.Fatalf("pages.list: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "nur-a" {
		t.Errorf("got %+v, want nur die Seite von A", list)
	}
}

// The body is left out of a listing on purpose: a hundred pages with their HTML
// would be the whole site in one payload, copied into a sandbox that has
// sixteen megabytes in total.
func TestListingHasNoBodies(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")
	seedPage(t, database, ws.ID, "Eine Seite", "eine-seite", "Ein langer Text.", "published")

	list, _, err := listPages(t, h, context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("pages.list: %v", err)
	}
	if len(list) != 1 || list[0].HTML != "" {
		t.Errorf("die Liste trägt einen Rumpf mit: %+v", list)
	}

	one, err := h.PagesForPlugin(context.Background(), ws.ID, plugin.PagesQuery{
		Op: plugin.OpPagesGet, Slug: "eine-seite", Limit: 1,
	})
	if err != nil {
		t.Fatalf("pages.get: %v", err)
	}
	if len(one.Pages) != 1 || !strings.Contains(one.Pages[0].HTML, "langer Text") {
		t.Errorf("pages.get liefert den Rumpf nicht: %+v", one.Pages)
	}
}

// A plugin's markup is read by visitors in this site's origin, so it goes
// through the same sanitiser as an editor's. There is no laxer policy for
// anyone.
func TestRenderStripsWhatWouldExecute(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	out, err := renderFor(t, h, ws, plugin.RenderArg{
		Title: "Von einem Plugin",
		HTML:  `<p>Harmlos</p><script>alert(1)</script><img src=x onerror="alert(2)">`,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<script") || strings.Contains(out, "onerror") {
		t.Errorf("das Skript hat es auf die Seite geschafft:\n%s", out)
	}
	if !strings.Contains(out, "Harmlos") {
		t.Error("der harmlose Teil fehlt")
	}
}

// The point of the render operation: the plugin supplies the middle, the theme
// supplies everything around it.
func TestRenderUsesTheTheme(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	out, err := renderFor(t, h, ws, plugin.RenderArg{Title: "Suche", HTML: "<p>Treffer</p>"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "<html>") || !strings.Contains(out, "<title>Suche</title>") {
		t.Errorf("die Ausgabe ist keine ganze Seite des Themes:\n%s", out)
	}
}

// A plugin naming a file rather than a view would be a way to ask the theme
// loader for something that is not a view at all.
func TestRenderRefusesAnUnknownView(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	if _, err := renderFor(t, h, ws, plugin.RenderArg{
		Title: "Egal", View: "../../etc/passwd",
	}); err == nil {
		t.Error("eine erfundene Ansicht wurde angenommen")
	}
}

// Only <mark> survives in a result row. It is the one place a plugin hands over
// a fragment of somebody else's text, and the only markup it needs there is the
// highlight.
func TestSnippetKeepsOnlyTheHighlight(t *testing.T) {
	got := string(markOnly(`<mark>Schaf</mark> & <b>fett</b> <script>x</script>`))
	if !strings.Contains(got, "<mark>Schaf</mark>") {
		t.Errorf("die Hervorhebung fehlt: %q", got)
	}
	for _, bad := range []string{"<b>", "<script>"} {
		if strings.Contains(got, bad) {
			t.Errorf("%s hat überlebt: %q", bad, got)
		}
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("das kaufmännische Und wurde nicht maskiert: %q", got)
	}
}

// Rendering needs a visitor. From an admin screen there is no site to draw, and
// saying so beats returning a page nobody asked for.
func TestRenderOutsideARequestFails(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")

	if _, err := h.RenderForPlugin(context.Background(), ws.ID,
		plugin.RenderArg{Title: "Egal"}); err == nil {
		t.Error("das Ausgeben ohne Anfrage wurde angenommen")
	}
}

// --- helpers ----------------------------------------------------------------

func listPages(t *testing.T, h *Handler, ctx context.Context, websiteID int64) ([]plugin.PageInfo, int, error) {
	t.Helper()
	res, err := h.PagesForPlugin(ctx, websiteID, plugin.PagesQuery{
		Op: plugin.OpPagesList, Limit: 50,
	})
	return res.Pages, res.Total, err
}

// renderFor calls the render operation the way a plugin reaches it: inside a
// request whose context carries the website.
func renderFor(t *testing.T, h *Handler, ws *domain.Website, a plugin.RenderArg) (string, error) {
	t.Helper()
	r := httptest.NewRequest("GET", "http://example.test/suche?q=schaf", nil)
	r = r.WithContext(domain.WebsiteToContext(r.Context(), ws))
	return h.RenderForPlugin(withRequest(r.Context(), r), ws.ID, a)
}
