package structured

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func decode(t *testing.T, js string) map[string]any {
	t.Helper()
	if js == "" {
		t.Fatal("no structured data was produced")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("the output is not valid JSON: %v\n%s", err, js)
	}
	return doc
}

func nodes(t *testing.T, doc map[string]any) map[string]map[string]any {
	t.Helper()
	graph, ok := doc["@graph"].([]any)
	if !ok {
		t.Fatalf("no @graph: %v", doc)
	}
	byType := map[string]map[string]any{}
	for _, n := range graph {
		node := n.(map[string]any)
		byType[node["@type"].(string)] = node
	}
	return byType
}

func fullBusiness() Business {
	return Business{
		Type:         "HomeAndConstructionBusiness",
		Name:         "Holzbau Schmidt",
		URL:          "https://example.de",
		LogoURL:      "/media/1/logo.png",
		Street:       "Waldweg 3",
		PostalCode:   "75173",
		City:         "Pforzheim",
		Country:      "DE",
		Phone:        "+49 7231 123456",
		Email:        "info@example.de",
		OpeningHours: "Mo-Fr 08:00-17:00\n\nSa 09:00-12:00\n",
	}
}

func TestBusinessBlockCarriesWhatASearchResultShows(t *testing.T) {
	published := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	js := Build(fullBusiness(), Page{
		Title: "Über uns", URL: "https://example.de/ueber-uns",
		Description: "Wir bauen Möbel.", SiteName: "Holzbau Schmidt",
		UpdatedAt: &published,
	}, nil)

	byType := nodes(t, decode(t, string(js)))
	biz, ok := byType["HomeAndConstructionBusiness"]
	if !ok {
		t.Fatalf("no business node: %v", byType)
	}

	if biz["telephone"] != "+49 7231 123456" {
		t.Errorf("telephone = %v", biz["telephone"])
	}
	addr, ok := biz["address"].(map[string]any)
	if !ok {
		t.Fatalf("no address: %v", biz)
	}
	for key, want := range map[string]string{
		"streetAddress": "Waldweg 3", "postalCode": "75173",
		"addressLocality": "Pforzheim", "addressCountry": "DE",
	} {
		if addr[key] != want {
			t.Errorf("address %s = %v, want %q", key, addr[key], want)
		}
	}

	// A relative path in a JSON document is not resolved the way a browser
	// resolves one in an attribute.
	if biz["logo"] != "https://example.de/media/1/logo.png" {
		t.Errorf("logo = %v, want an absolute URL", biz["logo"])
	}

	hours, ok := biz["openingHours"].([]any)
	if !ok || len(hours) != 2 {
		t.Fatalf("openingHours = %v, want two rules with the blank line dropped", biz["openingHours"])
	}
	if hours[0] != "Mo-Fr 08:00-17:00" {
		t.Errorf("first rule = %v", hours[0])
	}
}

func TestNoBusinessTypeMeansNoBusinessBlock(t *testing.T) {
	b := fullBusiness()
	b.Type = ""

	js := Build(b, Page{
		Title: "Über uns", URL: "https://example.de/ueber-uns",
	}, nil)

	// A personal blog must not claim to be a shop just because it has an
	// address in the imprint.
	if strings.Contains(string(js), "Waldweg") {
		t.Errorf("the address leaked without a type:\n%s", js)
	}
	byType := nodes(t, decode(t, string(js)))
	if _, ok := byType["WebPage"]; !ok {
		t.Errorf("the page node disappeared with the business one: %v", byType)
	}
}

func TestAPostBecomesAnArticleWithItsDates(t *testing.T) {
	published := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 1, 11, 30, 0, 0, time.UTC)

	js := Build(fullBusiness(), Page{
		Title: "Neue Werkbank", URL: "https://example.de/neue-werkbank",
		Description: "Endlich fertig.", IsPost: true, SiteName: "Holzbau Schmidt",
		PublishedAt: &published, UpdatedAt: &updated,
	}, nil)

	byType := nodes(t, decode(t, string(js)))
	article, ok := byType["Article"]
	if !ok {
		t.Fatalf("a post did not become an Article: %v", byType)
	}
	// headline is what an article is indexed by; name alone is ignored.
	if article["headline"] != "Neue Werkbank" {
		t.Errorf("headline = %v", article["headline"])
	}
	if article["datePublished"] != "2026-03-14T09:00:00Z" {
		t.Errorf("datePublished = %v", article["datePublished"])
	}
	if article["dateModified"] != "2026-04-01T11:30:00Z" {
		t.Errorf("dateModified = %v", article["dateModified"])
	}
	// The publisher is a reference, not a second copy of the address.
	pub, ok := article["publisher"].(map[string]any)
	if !ok || pub["@id"] != "https://example.de#organisation" {
		t.Errorf("publisher = %v, want a reference to the business node", article["publisher"])
	}
}

func TestAPageIsAWebPageNotAnArticle(t *testing.T) {
	js := Build(fullBusiness(), Page{
		Title: "Über uns", URL: "https://example.de/ueber-uns",
	}, nil)
	byType := nodes(t, decode(t, string(js)))
	if _, ok := byType["Article"]; ok {
		t.Error("a plain page was declared an Article")
	}
	if _, ok := byType["WebPage"]; !ok {
		t.Errorf("no WebPage node: %v", byType)
	}
}

func TestBreadcrumbsNeedMoreThanOneStep(t *testing.T) {
	single := Build(fullBusiness(), Page{
		Title: "Über uns", URL: "https://example.de/ueber-uns",
	}, []Crumb{{Name: "Start", URL: "https://example.de/"}})
	if strings.Contains(string(single), "BreadcrumbList") {
		t.Error("a one-step trail was emitted, which says nothing")
	}

	js := Build(fullBusiness(), Page{
		Title: "Neue Werkbank", URL: "https://example.de/neue-werkbank", IsPost: true,
	}, []Crumb{
		{Name: "Start", URL: "https://example.de/"},
		{Name: "Aktuelles", URL: "https://example.de/aktuelles"},
		{Name: "Neue Werkbank", URL: "https://example.de/neue-werkbank"},
	})
	byType := nodes(t, decode(t, string(js)))
	crumbs, ok := byType["BreadcrumbList"]
	if !ok {
		t.Fatalf("no breadcrumbs: %v", byType)
	}
	items := crumbs["itemListElement"].([]any)
	if len(items) != 3 {
		t.Fatalf("%d steps, want 3", len(items))
	}
	// Positions are one-based and in order, or the trail reads backwards.
	for i, item := range items {
		if got := item.(map[string]any)["position"].(float64); int(got) != i+1 {
			t.Errorf("step %d has position %v", i, got)
		}
	}
}

func TestATitleCannotBreakOutOfTheScriptElement(t *testing.T) {
	js := string(Build(fullBusiness(), Page{
		Title: `Preise </script><script>alert(1)</script>`,
		URL:   "https://example.de/preise",
	}, nil))

	// The document is placed inside <script type="application/ld+json">, so a
	// literal "</script>" in a page title would end the element early and turn
	// the rest of the JSON into markup.
	if strings.Contains(js, "</script>") {
		t.Errorf("a closing script tag survived:\n%s", js)
	}
	if !strings.Contains(js, `\u003c/script\u003e`) {
		t.Errorf("the tag was not escaped as \\u003c:\n%s", js)
	}
	// And it must still be valid JSON that decodes back to the original text.
	byType := nodes(t, decode(t, js))
	if byType["WebPage"]["name"] != `Preise </script><script>alert(1)</script>` {
		t.Errorf("the title did not survive the round trip: %v", byType["WebPage"]["name"])
	}
}

func TestNothingWorthSayingProducesNoDocument(t *testing.T) {
	if js := Build(Business{}, Page{}, nil); js != "" {
		t.Errorf("an empty site produced %q; a theme would emit an empty element", js)
	}
}

func TestKnownOrgTypeGuardsTheDropdown(t *testing.T) {
	if !KnownOrgType("LocalBusiness") || !KnownOrgType("") {
		t.Error("an offered type was rejected")
	}
	// An unknown type produces data a search engine discards silently, which is
	// worse than emitting none.
	if KnownOrgType("Weltraumbahnhof") {
		t.Error("an invented type was accepted")
	}
}
