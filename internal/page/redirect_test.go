package page

import (
	"context"
	"testing"
)

func redirectTarget(t *testing.T, s *Store, ws int64, from string) string {
	t.Helper()
	r, err := s.LookupRedirect(context.Background(), ws, from)
	if err != nil {
		t.Fatalf("LookupRedirect: %v", err)
	}
	if r == nil {
		return ""
	}
	return r.ToPath
}

// Renaming a page must not break the links pointing at it.
func TestRenamingAPageLeavesARedirect(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Kontakt", "kontakt", "text", "published")

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Kontakt und Anfahrt", Slug: "kontakt-und-anfahrt",
		Markdown: "text", HTML: "<p>text</p>", Status: "published",
		ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	if got := redirectTarget(t, s, ws, "/kontakt"); got != "/kontakt-und-anfahrt" {
		t.Errorf("redirect target = %q, want /kontakt-und-anfahrt", got)
	}
}

// The inline title editor derives the slug from the title, so this is the path
// where a live URL changes by accident.
func TestInlineRenameAlsoLeavesARedirect(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Kontakt", "kontakt", "text", "published")

	if err := s.UpdatePageTitle(ctx, p.ID, "Anfahrt", "anfahrt", nil); err != nil {
		t.Fatalf("UpdatePageTitle: %v", err)
	}
	if got := redirectTarget(t, s, ws, "/kontakt"); got != "/anfahrt" {
		t.Errorf("redirect target = %q, want /anfahrt", got)
	}
}

// Renaming back must not leave a redirect pointing at the page's own address,
// which would be an infinite loop.
func TestRenamingBackRemovesTheLoop(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Kontakt", "kontakt", "text", "published")

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Anfahrt", Slug: "anfahrt", Markdown: "text", HTML: "<p>text</p>",
		Status: "published", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("first rename: %v", err)
	}
	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Kontakt", Slug: "kontakt", Markdown: "text", HTML: "<p>text</p>",
		Status: "published", ExpectedVersion: p.Version + 1,
	}); err != nil {
		t.Fatalf("rename back: %v", err)
	}

	if got := redirectTarget(t, s, ws, "/kontakt"); got != "" {
		t.Errorf("a redirect from the page's own address survived: %q", got)
	}
	if got := redirectTarget(t, s, ws, "/anfahrt"); got != "/kontakt" {
		t.Errorf("redirect from the intermediate address = %q, want /kontakt", got)
	}
}

// Two renames in a row must collapse to one hop rather than needing a runtime
// hop limit.
func TestRedirectChainsCollapse(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "A", "a", "text", "published")

	for i, slug := range []string{"b", "c"} {
		if err := s.UpdatePage(ctx, p.ID, PageUpdate{
			Title: slug, Slug: slug, Markdown: "text", HTML: "<p>text</p>",
			Status: "published", ExpectedVersion: p.Version + int64(i),
		}); err != nil {
			t.Fatalf("rename to %s: %v", slug, err)
		}
	}

	if got := redirectTarget(t, s, ws, "/a"); got != "/c" {
		t.Errorf("the first address points at %q, want /c in one hop", got)
	}
	if got := redirectTarget(t, s, ws, "/b"); got != "/c" {
		t.Errorf("the second address points at %q, want /c", got)
	}
}

func TestRedirectCountsHits(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if err := s.AddRedirect(ctx, ws, "/alte-seite", "/neue-seite", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := s.LookupRedirect(ctx, ws, "/alte-seite"); err != nil {
			t.Fatalf("LookupRedirect: %v", err)
		}
	}

	list, err := s.ListRedirects(ctx, ws)
	if err != nil {
		t.Fatalf("ListRedirects: %v", err)
	}
	if len(list) != 1 || list[0].Hits != 3 {
		t.Errorf("hit count is %+v, want 3 — it is what shows which old URL still brings traffic", list)
	}
}

// A redirect belongs to one website; another site's path must not resolve.
func TestRedirectsAreScopedToTheirWebsite(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if err := s.AddRedirect(ctx, ws, "/alt", "/neu", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	if got := redirectTarget(t, s, ws+999, "/alt"); got != "" {
		t.Errorf("another website resolved the redirect to %q", got)
	}
}
