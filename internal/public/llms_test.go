package public

import (
	"context"
	"encoding/xml"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func sitemapLocs(t *testing.T, body string) []string {
	t.Helper()
	var doc struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("sitemap is not well-formed: %v\n%s", err, body)
	}
	var out []string
	for _, u := range doc.URLs {
		out = append(out, u.Loc)
	}
	sort.Strings(out)
	return out
}

var llmsLink = regexp.MustCompile(`\]\((http[^)]*)\)`)

func llmsLocs(body string) []string {
	var out []string
	for _, m := range llmsLink.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestLLMSAndSitemapShareOneSelection(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	other := seedWebsite(t, database, "Andere")
	seedPage(t, database, ws.ID, "Offen", "offen", "x", "published")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "x", "draft")
	seedPage(t, database, ws.ID, "Geplant", "geplant", "x", "published")
	seedPage(t, database, ws.ID, "Versteckt", "versteckt", "x", "published")
	seedPage(t, database, ws.ID, "Geschuetzt", "geschuetzt", "x", "published")
	seedPage(t, database, other.ID, "Fremd", "fremd", "x", "published")
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := database.Write.ExecContext(context.Background(), q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE pages SET publish_at = '2999-01-01T00:00:00Z' WHERE slug = 'geplant'`)
	exec(`UPDATE pages SET noindex = 1 WHERE slug = 'versteckt'`)
	exec(`UPDATE pages SET access = 'password' WHERE slug = 'geschuetzt'`)

	sm, err := request(h.HandleSitemap, ws, "GET", "/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	ll, err := request(h.HandleLLMS, ws, "GET", "/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	if ct := ll.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := ll.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
	for name, body := range map[string]string{"sitemap": sm.Body.String(), "llms": ll.Body.String()} {
		if !strings.Contains(body, "/offen") {
			t.Errorf("%s: published page missing:\n%s", name, body)
		}
		for _, bad := range []string{"entwurf", "geplant", "versteckt", "geschuetzt", "fremd"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s lists %q:\n%s", name, bad, body)
			}
		}
	}
	a, b := sitemapLocs(t, sm.Body.String()), llmsLocs(ll.Body.String())
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Errorf("URL sets differ:\nsitemap %v\nllms    %v", a, b)
	}
	if !strings.HasPrefix(ll.Body.String(), "# Test Site\n") {
		t.Errorf("heading missing:\n%s", ll.Body.String())
	}
}

func TestLLMSGroupsByLanguage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	ws.Locale, ws.ExtraLocales = "de", "fr"
	seedPage(t, database, ws.ID, "Kontakt", "kontakt", "x", "published")
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: ws.ID, Title: "Contact", Slug: "contact", Markdown: "x", HTML: "<p>x</p>",
		Status: "published", Locale: "fr",
	}); err != nil {
		t.Fatal(err)
	}
	rec, err := request(h.HandleLLMS, ws, "GET", "/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if n := strings.Count(body, "\n## "); n != 2 {
		t.Fatalf("want two language sections, got %d:\n%s", n, body)
	}
	if strings.Index(body, "## Deutsch") > strings.Index(body, "## Français") || !strings.Contains(body, "## Deutsch") {
		t.Errorf("main language must come first:\n%s", body)
	}
	if !strings.Contains(body, "(http://demo.test/fr/contact)") {
		t.Errorf("fr URL not prefixed:\n%s", body)
	}
}

func TestLLMSEscapesHostileTitle(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "A] [B](x)\nC", "hostile", "x", "published")
	rec, err := request(h.HandleLLMS, ws, "GET", "/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(rec.Body.String(), "\n") {
		if strings.Contains(l, "/hostile)") {
			lines = append(lines, l)
		}
	}
	want := `- [A\] \[B\]\(x\) C](http://demo.test/hostile)`
	if len(lines) != 1 || !strings.HasPrefix(lines[0], want) {
		t.Errorf("want one line %q, got %q", want, lines)
	}
}
