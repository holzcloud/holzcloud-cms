package page

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The whole feature rests on FTS5 being compiled into the pure-Go driver and on
// remove_diacritics finding "Möbel" when a visitor types "mobel" — decisive for
// a German-language CMS.
func TestSearchFindsPagesIgnoringDiacritics(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	seed(t, s, ws, "Möbel aus Eiche", "moebel", "Wir bauen Schränke und Türen.", "published")
	seed(t, s, ws, "Kontakt", "kontakt", "Rufen Sie an.", "published")

	results, err := s.SearchPages(ctx, ws, "mobel", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 1 || results[0].Page.Slug != "moebel" {
		t.Fatalf("got %d results (%+v), want the Möbel page", len(results), results)
	}
}

func TestSearchHighlightsBodyMatches(t *testing.T) {
	s, ws := newTestStore(t)
	seed(t, s, ws, "Leistungen", "leistungen", "Wir bauen Schränke, Türen und Treppen aus Massivholz.", "published")

	results, err := s.SearchPages(context.Background(), ws, "Treppen", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(string(results[0].Snippet), "<mark>Treppen</mark>") {
		t.Errorf("body match not highlighted: %q", results[0].Snippet)
	}
}

// The snippet is built from page text, so it is user input reaching the admin's
// browser as HTML. Escaping has to happen before the sentinels become tags.
func TestSearchSnippetEscapesPageContent(t *testing.T) {
	s, ws := newTestStore(t)
	seed(t, s, ws, "Test", "test",
		`Ein <script>alert(1)</script> und ein Treppengeländer.`, "published")

	results, err := s.SearchPages(context.Background(), ws, "Treppengeländer", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	snippet := string(results[0].Snippet)
	if strings.Contains(snippet, "<script") {
		t.Errorf("script survived into the snippet: %q", snippet)
	}
	if !strings.Contains(snippet, "&lt;script") {
		t.Errorf("the tag was not escaped: %q", snippet)
	}
}

// A bare MATCH over user text is a syntax-error vector: a stray quote or a
// bare operator makes SQLite reject the statement, so the search 500s instead
// of returning nothing.
func TestSearchSurvivesFTSMetacharacters(t *testing.T) {
	s, ws := newTestStore(t)
	seed(t, s, ws, "Kontakt", "kontakt", "Rufen Sie an.", "published")

	for _, q := range []string{
		`"`, `O"Brien`, `AND`, `NOT OR`, `*`, `^foo`, `a:b`, `(unbalanced`, `-`, `""`,
	} {
		if _, err := s.SearchPages(context.Background(), ws, q, false, 20); err != nil {
			t.Errorf("query %q returned an error: %v", q, err)
		}
	}
}

func TestFTSQueryQuotesEveryTerm(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"   ":        "",
		"moebel":     `"moebel"*`,
		"alte eiche": `"alte" AND "eiche"*`,
		`O"Brien`:    `"O""Brien"*`,
		"AND":        `"AND"*`,
	}
	for in, want := range cases {
		if got := FTSQuery(in); got != want {
			t.Errorf("FTSQuery(%q) = %q; want %q", in, got, want)
		}
	}
}

// The public search must obey exactly the same visibility rules as the page
// handler — a draft that turns up in search results is a draft leak.
func TestSearchHidesDraftsFromThePublic(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	seed(t, s, ws, "Geheim", "geheim", "streng geheimes Treppenprojekt", "draft")

	public, err := s.SearchPages(ctx, ws, "Treppenprojekt", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(public) != 0 {
		t.Errorf("a draft turned up in the public search: %+v", public)
	}

	admin, err := s.SearchPages(ctx, ws, "Treppenprojekt", true, 20)
	if err != nil {
		t.Fatalf("SearchPages (admin): %v", err)
	}
	if len(admin) != 1 {
		t.Errorf("the admin search does not find drafts: %+v", admin)
	}
}

// A trashed page's terms must leave the index, or search resurrects it.
func TestSearchDropsTrashedPages(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Weg", "weg", "Treppenhaus", "published")

	if err := s.TrashPage(ctx, p.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}
	results, err := s.SearchPages(ctx, ws, "Treppenhaus", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("a trashed page is still searchable: %+v", results)
	}
}

// The index is kept in sync by triggers; an edit that did not reach it would
// leave the old text findable and the new text invisible.
func TestSearchIndexFollowsEdits(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "Wintergarten", "published")

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "Dachstuhl", HTML: "<p>Dachstuhl</p>",
		Status: "published", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	if old, _ := s.SearchPages(ctx, ws, "Wintergarten", false, 20); len(old) != 0 {
		t.Errorf("the replaced text is still findable: %+v", old)
	}
	if fresh, _ := s.SearchPages(ctx, ws, "Dachstuhl", false, 20); len(fresh) != 1 {
		t.Errorf("the new text is not findable: %+v", fresh)
	}
}

// Scheduling is a read-time predicate, so a page with a future publish_at must
// be invisible everywhere a visitor can look.
func TestScheduledPageIsInvisibleUntilItsMoment(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(24 * time.Hour)

	p, err := s.CreatePage(ctx, PageCreate{
		WebsiteID: ws, Title: "Aktion", Slug: "aktion",
		Markdown: "Sommeraktion", HTML: "<p>Sommeraktion</p>", Status: "published",
		Schedule: PageSchedule{PublishAt: &future},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	if got, _ := s.GetPublishedPage(ctx, ws, "aktion"); got != nil {
		t.Error("a scheduled page is already served")
	}
	if got, _ := s.GetHomePage(ctx, ws); got != nil {
		t.Error("a scheduled page stands in as the home page")
	}
	entries, _ := s.ListPublishedForSitemap(ctx, ws)
	if len(entries) != 0 {
		t.Errorf("a scheduled page is in the sitemap: %+v", entries)
	}
	if hits, _ := s.SearchPages(ctx, ws, "Sommeraktion", false, 20); len(hits) != 0 {
		t.Error("a scheduled page is findable through public search")
	}

	// It is still the operator's page: the admin list has to show it.
	pages, total, _ := s.ListPages(ctx, ws, ListFilter{Page: 1, PerPage: 20})
	if total != 1 || len(pages) != 1 || !pages[0].Scheduled() {
		t.Errorf("the admin list does not show the scheduled page: %+v", pages)
	}

	// Move the moment into the past and it appears.
	past := time.Now().UTC().Add(-time.Hour)
	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Aktion", Slug: "aktion", Markdown: "Sommeraktion", HTML: "<p>Sommeraktion</p>",
		Status: "published", ExpectedVersion: p.Version,
		Schedule: PageSchedule{PublishAt: &past},
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if got, _ := s.GetPublishedPage(ctx, ws, "aktion"); got == nil {
		t.Error("the page did not appear after its publication moment")
	}
}

func TestExpiredPageDisappears(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Hour)

	if _, err := s.CreatePage(ctx, PageCreate{
		WebsiteID: ws, Title: "Aktion", Slug: "aktion",
		Markdown: "abgelaufen", HTML: "<p>abgelaufen</p>", Status: "published",
		Schedule: PageSchedule{UnpublishAt: &past},
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	if got, _ := s.GetPublishedPage(ctx, ws, "aktion"); got != nil {
		t.Error("an expired page is still served")
	}
}
