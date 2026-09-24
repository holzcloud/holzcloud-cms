package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/export"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The export asks the router main() serves, so this test does too: a website
// with a domain, two published pages and a draft, exported into a directory.
func TestExportWritesTheWebsiteAsFiles(t *testing.T) {
	handler, _, database := testRouter(t)
	ctx := context.Background()

	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(ctx, "Demo", "beschreibung")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := domains.AddDomain(ctx, ws.ID, "demo.test", true); err != nil {
		t.Fatal(err)
	}
	pages := page.NewStore(database)
	for _, p := range []struct{ slug, body, status string }{
		{"ueber-uns", "Wir sind da. [Kontakt](/kontakt) und [weg](https://example.org/).", "published"},
		{"kontakt", "Schreiben Sie uns.", "published"},
		{"geheim", "Noch nicht fertig.", "draft"},
	} {
		html, err := page.RenderMarkdown(p.body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pages.CreatePage(ctx, page.PageCreate{
			WebsiteID: ws.ID, Title: p.slug, Slug: p.slug, Markdown: p.body, HTML: html, Status: p.status,
		}); err != nil {
			t.Fatal(err)
		}
	}

	dir := filepath.Join(t.TempDir(), "out")
	report, err := export.Run(ctx, export.Options{Handler: handler, Host: "demo.test", Dir: dir})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	for _, want := range []string{"index.html", "ueber-uns/index.html", "kontakt/index.html", "sitemap.xml", "robots.txt", "404.html"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s missing from the export (files: %v)", want, report.Files)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "geheim")); err == nil {
		t.Error("a draft was exported")
	}
	for _, f := range report.Files {
		if strings.HasPrefix(f, "admin/") {
			t.Errorf("an admin address was exported: %s", f)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "ueber-uns/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Wir sind da.") {
		t.Errorf("the exported page is not the page:\n%s", body)
	}

	// A second export into the same directory would keep what has been
	// deleted since, so it is refused.
	if _, err := export.Run(ctx, export.Options{Handler: handler, Host: "demo.test", Dir: dir}); err == nil {
		t.Error("an export into a non-empty directory was accepted")
	}
}
