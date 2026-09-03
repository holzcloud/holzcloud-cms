package template

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// testTemplateFS mirrors the shape every shipped template has: layout.html is
// the document, and each view supplies a "content" block of the same name.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><head><title>{{.Page.Title}}</title></head><body>{{template "content" .}}</body></html>`)},
		"home.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<main class="home">{{.Site.Name}}</main>{{end}}`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article>{{.Page.Title}}</article>{{end}}`)},
		"404.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="notfound">not found</p>{{end}}`)},
		"maintenance.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="maintenance">{{.Meta.Message}}</p>{{end}}`)},
		"search.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="search">{{.Search.Query}}</p>{{end}}`)},
		"list.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<ol class="archive">{{range .Archive.Entries}}<li>{{.Title}}</li>{{end}}</ol>{{end}}`)},
		"gate.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<form class="gate">{{.Gate.Hint}}</form>{{end}}`)},
		"shop.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<ol class="shop">{{range .Catalogue.Products}}<li>{{.Title}}</li>{{end}}</ol>{{end}}`)},
		"product.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article class="product">{{.Product.Title}}</article>{{end}}`)},
		"cart.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<ol class="cart">{{range .Cart.Lines}}<li>{{.Title}}</li>{{end}}</ol>{{end}}`)},
		"checkout.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<form action="{{.Checkout.Action}}"></form>{{end}}`)},
		"order.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<p class="order">{{.Order.Number}}</p>{{end}}`)},
	}
}

func testData() PageData {
	return PageData{
		Site: SiteData{Name: "Example Site"},
		Page: PageContent{Title: "My Page", ContentHTML: "<p>body</p>"},
	}
}

// Every view file defines "content". Parsing them into one set makes the last
// one win, and executing the view file itself renders nothing at all — both
// failure modes produce a blank public site.
func TestRenderPageRendersEachViewDistinctly(t *testing.T) {
	loader := NewLoader(t.TempDir(), testTemplateFS(), nil, nil)
	ctx := context.Background()

	cases := map[string]string{
		"home.html": `<main class="home">Example Site</main>`,
		"page.html": `<article>My Page</article>`,
		"404.html":  `<p class="notfound">not found</p>`,
	}

	for view, want := range cases {
		out, err := loader.RenderPage(ctx, 1, view, testData())
		if err != nil {
			t.Fatalf("RenderPage(%s): %v", view, err)
		}
		got := string(out)

		if len(strings.TrimSpace(got)) == 0 {
			t.Errorf("RenderPage(%s) produced an empty document", view)
			continue
		}
		if !strings.Contains(got, "<html>") {
			t.Errorf("RenderPage(%s) did not render through layout.html:\n%s", view, got)
		}
		if !strings.Contains(got, want) {
			t.Errorf("RenderPage(%s) missing its own content block %q:\n%s", view, want, got)
		}
		// The other views' content must not bleed in.
		for otherView, otherWant := range cases {
			if otherView != view && strings.Contains(got, otherWant) {
				t.Errorf("RenderPage(%s) rendered %s's content block:\n%s", view, otherView, got)
			}
		}
	}
}

func TestRenderPageRejectsUnknownView(t *testing.T) {
	loader := NewLoader(t.TempDir(), testTemplateFS(), nil, nil)

	if _, err := loader.RenderPage(context.Background(), 1, "layout.html", testData()); err == nil {
		t.Error("expected layout.html to be rejected as a view")
	}
	if _, err := loader.RenderPage(context.Background(), 1, "../secret.html", testData()); err == nil {
		t.Error("expected an unknown view name to be rejected")
	}
}

func TestRender404UsesTheNotFoundView(t *testing.T) {
	loader := NewLoader(t.TempDir(), testTemplateFS(), nil, nil)

	out, err := loader.Render404(context.Background(), 1, SiteData{Name: "Example Site"})
	if err != nil {
		t.Fatalf("Render404: %v", err)
	}
	if !strings.Contains(string(out), `<p class="notfound">not found</p>`) {
		t.Errorf("Render404 did not render the 404 view:\n%s", out)
	}
}

// Switching a website's template must not keep serving the old one from cache.
func TestInvalidateTemplateCacheClearsEveryView(t *testing.T) {
	loader := NewLoader(t.TempDir(), testTemplateFS(), nil, nil)
	ctx := context.Background()

	for _, view := range viewFiles {
		if _, err := loader.RenderPage(ctx, 1, view, testData()); err != nil {
			t.Fatalf("warm cache for %s: %v", view, err)
		}
	}

	loader.InvalidateTemplateCache(1)

	remaining := 0
	loader.cache.Range(func(k, _ any) bool {
		if key, ok := k.(cacheKey); ok && key.websiteID == 1 {
			remaining++
		}
		return true
	})
	if remaining != 0 {
		t.Errorf("%d cached views survived invalidation", remaining)
	}
}

// tmplmgr accepts an archive carrying only layout.html and page.html, and the
// README promises the rest falls back to the default theme. Without a per-file
// fallback such a conforming upload made GET / fail with 500 on home.html.
func TestPartialThemeFallsBackPerFile(t *testing.T) {
	// A theme with the two required files and nothing else.
	partial := fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html><body class="eigen">{{template "content" .}}</body></html>`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<article class="eigen-page">{{.Page.Title}}</article>{{end}}`)},
	}

	// Install it on disk the way an upload would, so it is the winning source
	// while the complete default theme stays available as the fallback.
	dir := t.TempDir()
	themeDir := filepath.Join(dir, "templates", "eigen")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, file := range partial {
		if err := os.WriteFile(filepath.Join(themeDir, name), file.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: "eigen"})

	// page.html comes from the uploaded theme.
	out, err := loader.RenderPage(context.Background(), 1, "page.html", testData())
	if err != nil {
		t.Fatalf("page view: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "eigen-page") || !strings.Contains(body, `class="eigen"`) {
		t.Errorf("own layout/page not used:\n%s", body)
	}

	// home.html and 404.html are absent and must fall back rather than 500.
	for _, view := range []string{"home.html", "404.html"} {
		out, err := loader.RenderPage(context.Background(), 1, view, testData())
		if err != nil {
			t.Errorf("%s should fall back rather than fail: %v", view, err)
			continue
		}
		// The layout must still be the theme's own — a theme may never end up
		// with a foreign layout wrapped around its content.
		if !strings.Contains(string(out), `class="eigen"`) {
			t.Errorf("%s lost the theme's own layout:\n%s", view, out)
		}
	}
}

// Every view of every shipped theme must render against a fully populated
// PageData — a broken theme should fail the suite, not a visitor's request.
func TestShippedThemesRenderEveryView(t *testing.T) {
	themes := []string{"default", "schlicht", "magazine", "midnight", "journal", "rudel", "weide", "holzcloud"}
	for _, theme := range themes {
		dir := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "public", theme)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("theme %s missing: %v", theme, err)
		}
		loader := NewLoader(t.TempDir(), os.DirFS(dir), nil, nil)
		for _, view := range viewFiles {
			out, err := loader.RenderPage(context.Background(), 1, view, SampleData())
			if err != nil {
				t.Errorf("%s/%s: %v", theme, view, err)
				continue
			}
			body := string(out)
			if !strings.Contains(body, "<html") {
				t.Errorf("%s/%s did not render through layout.html", theme, view)
			}
			// Assertions on literal markup alone cannot tell a working theme
			// from one whose {{ }} got mangled into { }: everything static
			// still matches. These check that data actually made it in.
			if strings.Contains(body, "{.") || strings.Contains(body, "{if ") {
				t.Errorf("%s/%s contains unexpanded template syntax", theme, view)
			}
			if !strings.Contains(body, "Holzbau Schmidt") {
				t.Errorf("%s/%s never rendered the site name", theme, view)
			}
			if !strings.Contains(body, "https://example.de") {
				t.Errorf("%s/%s never rendered the canonical URL", theme, view)
			}
			if !strings.Contains(body, "Impressum") {
				t.Errorf("%s/%s did not render the footer menu", theme, view)
			}
			if !strings.Contains(body, `lang="de"`) {
				t.Errorf("%s/%s does not declare German content", theme, view)
			}
			// A keyboard user should not have to tab through the whole
			// navigation on every page to reach the content.
			if !strings.Contains(body, `class="skip-link"`) || !strings.Contains(body, `id="main"`) {
				t.Errorf("%s/%s has no skip link pointing at the main landmark", theme, view)
			}
			// The archive is only useful if the entries and the pager reach the
			// page — a list.html that renders its heading and nothing else
			// passes every assertion above.
			if view == "list.html" {
				for _, want := range []string{"Neue Werkbank", "/neue-werkbank", "/aktuelles?seite=3", "/tag/eiche"} {
					if !strings.Contains(body, want) {
						t.Errorf("%s/list.html is missing %q", theme, want)
					}
				}
				// A label archive that never says which label it is showing
				// leaves the visitor guessing why these entries and no others.
				if !strings.Contains(body, "Eichenholz") {
					t.Errorf("%s/list.html does not name the label it is filtered by", theme)
				}
			}
			// The gate is the only thing standing in front of a protected
			// page; a theme that renders no form makes it unreachable.
			if view == "gate.html" {
				for _, want := range []string{"/freischalten", `name="passwort"`, "Anschreiben"} {
					if !strings.Contains(body, want) {
						t.Errorf("%s/gate.html is missing %q", theme, want)
					}
				}
			}
			// A post has neighbours; without them a reader who followed a link
			// to one entry has no way onward.
			if view == "page.html" {
				if !strings.Contains(body, "/naechst") {
					t.Errorf("%s/page.html does not link to the next entry", theme)
				}
				// Labels that never reach the page are labels nobody can
				// follow, which is the whole point of having them.
				if !strings.Contains(body, "/tag/moebel") {
					t.Errorf("%s/page.html does not render the page's labels", theme)
				}
				// A customer who cannot tell a preview from the live site will
				// report the page as published, and nobody will believe them.
				if !strings.Contains(body, "preview-banner") {
					t.Errorf("%s/page.html does not mark a preview as one", theme)
				}
			}
			// Two themes used to emit a bare <ul> for the main menu, which is
			// not a landmark and has no accessible name.
			if strings.Count(body, "<nav ") < 2 {
				t.Errorf("%s/%s does not wrap both menus in a named <nav>", theme, view)
			}
			if !strings.Contains(body, `role="contentinfo"`) {
				t.Errorf("%s/%s has no contentinfo landmark", theme, view)
			}
		}

		// Removing the focus ring leaves a keyboard user with no way to tell
		// where they are.
		css, err := os.ReadFile(filepath.Join(dir, "style.css"))
		if err != nil {
			t.Fatalf("%s/style.css: %v", theme, err)
		}
		if strings.Contains(strings.ReplaceAll(string(css), " ", ""), "outline:none") &&
			!strings.Contains(string(css), "main:focus") {
			t.Errorf("%s suppresses the focus outline", theme)
		}
		if !strings.Contains(string(css), "@media print") {
			t.Errorf("%s has no print stylesheet", theme)
		}
	}
}

// A German site must not publish US dates, and an English one must not publish
// German ones. formatDate used to be hard-wired to "January 2, 2006".
func TestDatesFollowTheSiteLocale(t *testing.T) {
	loader := NewLoader(t.TempDir(), fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(
			`<html lang="{{.Site.Locale}}"><body>{{template "content" .}}</body></html>`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}<time datetime="{{formatDateISO .Page.PublishedAt}}">{{formatDate .Page.PublishedAt}}</time>{{end}}`)},
	}, nil, nil)

	published := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	data := PageData{Page: PageContent{Title: "T", PublishedAt: &published}}

	data.Site = SiteData{Locale: "de", TimeZone: "Europe/Berlin"}
	de, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("de: %v", err)
	}
	if !strings.Contains(string(de), "14. März 2026") {
		t.Errorf("German date missing:\n%s", de)
	}

	// Same website id, different locale: the cache must not hand back the
	// German set, which is what a cache key without the locale would do.
	data.Site = SiteData{Locale: "en", TimeZone: "Europe/Berlin"}
	en, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("en: %v", err)
	}
	if !strings.Contains(string(en), "March 14, 2026") {
		t.Errorf("English date missing:\n%s", en)
	}

	// The machine-readable form stays the same in both.
	for _, out := range [][]byte{de, en} {
		if !strings.Contains(string(out), `datetime="2026-03-14"`) {
			t.Errorf("ISO date missing:\n%s", out)
		}
	}
}

// A timestamp just before midnight UTC falls on the next day in Berlin. Without
// the conversion a page published at 01:30 local time is dated a day early.
func TestDatesRenderInTheSiteTimeZone(t *testing.T) {
	loader := NewLoader(t.TempDir(), fstest.MapFS{
		"layout.html": &fstest.MapFile{Data: []byte(`{{template "content" .}}`)},
		"page.html": &fstest.MapFile{Data: []byte(
			`{{define "content"}}{{formatDateISO .Page.PublishedAt}}{{end}}`)},
	}, nil, nil)

	published := time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC) // 01:30 on 1 July in Berlin
	data := PageData{
		Site: SiteData{Locale: "de", TimeZone: "Europe/Berlin"},
		Page: PageContent{Title: "T", PublishedAt: &published},
	}
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if strings.TrimSpace(string(out)) != "2026-07-01" {
		t.Errorf("date rendered as %q, want 2026-07-01 — it was not converted to Europe/Berlin", out)
	}
}
