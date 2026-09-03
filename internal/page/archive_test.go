package page

import (
	"context"
	"testing"
	"time"
)

// seedPost creates a published archive entry with an explicit publication date,
// so the ordering can be asserted without waiting for the clock.
func seedPost(t *testing.T, s *Store, websiteID int64, title, slug string, published time.Time) *Page {
	t.Helper()
	p, err := s.CreatePage(context.Background(), PageCreate{
		WebsiteID: websiteID,
		Title:     title,
		Slug:      slug,
		Markdown:  title,
		HTML:      "<p>" + title + "</p>",
		Status:    "published",
		Kind:      KindPost,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if _, err := s.DB.Write.Exec(`UPDATE pages SET published_at = $1 WHERE id = $2`,
		published.UTC().Format(timeLayout), p.ID); err != nil {
		t.Fatalf("backdate post: %v", err)
	}
	updated, _ := s.GetPage(context.Background(), p.ID)
	return updated
}

func TestListArchiveOrdersByPublicationNotByEdit(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	old := seedPost(t, s, ws, "Alter Beitrag", "alt", base)
	seedPost(t, s, ws, "Neuer Beitrag", "neu", base.Add(48*time.Hour))
	// A page must never appear in the archive, however recently it was touched.
	seed(t, s, ws, "Über uns", "ueber-uns", "text", "published")

	// Editing the old entry must not lift it back to the top: an archive sorted
	// by last edit reorders itself every time a typo is fixed.
	if err := s.UpdatePage(ctx, old.ID, PageUpdate{
		Title: "Alter Beitrag", Slug: "alt", Markdown: "korrigiert",
		HTML: "<p>korrigiert</p>", Status: "published", Kind: KindPost,
		ExpectedVersion: old.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	posts, total, err := s.ListArchive(ctx, ws, 1, 10)
	if err != nil {
		t.Fatalf("ListArchive: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 posts (the page must not be counted)", total)
	}
	if posts[0].Slug != "neu" || posts[1].Slug != "alt" {
		t.Errorf("order = %q, %q; want neu, alt", posts[0].Slug, posts[1].Slug)
	}
}

func TestListArchiveHidesDraftsAndScheduledEntries(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	seedPost(t, s, ws, "Sichtbar", "sichtbar", base)

	draft, err := s.CreatePage(ctx, PageCreate{
		WebsiteID: ws, Title: "Entwurf", Slug: "entwurf",
		Markdown: "x", HTML: "<p>x</p>", Status: "draft", Kind: KindPost,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	future := time.Now().UTC().Add(24 * time.Hour)
	if _, err := s.CreatePage(ctx, PageCreate{
		WebsiteID: ws, Title: "Geplant", Slug: "geplant",
		Markdown: "x", HTML: "<p>x</p>", Status: "published", Kind: KindPost,
		Schedule: PageSchedule{PublishAt: &future},
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	posts, total, err := s.ListArchive(ctx, ws, 1, 10)
	if err != nil {
		t.Fatalf("ListArchive: %v", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Slug != "sichtbar" {
		t.Fatalf("archive shows %d entries, want only the published one: %+v", total, posts)
	}
	_ = draft
}

func TestListArchivePages(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		seedPost(t, s, ws, "Eintrag", "eintrag-"+string(rune('a'+i)), base.Add(time.Duration(i)*time.Hour))
	}

	first, total, err := s.ListArchive(ctx, ws, 1, 2)
	if err != nil {
		t.Fatalf("ListArchive: %v", err)
	}
	if total != 5 || len(first) != 2 {
		t.Fatalf("page 1: %d of %d, want 2 of 5", len(first), total)
	}
	third, _, err := s.ListArchive(ctx, ws, 3, 2)
	if err != nil {
		t.Fatalf("ListArchive page 3: %v", err)
	}
	if len(third) != 1 {
		t.Fatalf("page 3 has %d entries, want the single remainder", len(third))
	}
	// No entry may appear on two pages, and none may be skipped between them.
	if third[0].Slug == first[0].Slug || third[0].Slug == first[1].Slug {
		t.Error("an entry appears on more than one page")
	}
}

func TestAdjacentPostsWalkTheArchiveInOrder(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)

	seedPost(t, s, ws, "Erster", "erster", base)
	middle := seedPost(t, s, ws, "Zweiter", "zweiter", base.Add(time.Hour))
	seedPost(t, s, ws, "Dritter", "dritter", base.Add(2*time.Hour))

	prev, next, err := s.AdjacentPosts(ctx, ws, middle)
	if err != nil {
		t.Fatalf("AdjacentPosts: %v", err)
	}
	if prev == nil || prev.Slug != "erster" {
		t.Errorf("prev = %v, want erster", prev)
	}
	if next == nil || next.Slug != "dritter" {
		t.Errorf("next = %v, want dritter", next)
	}

	// At the ends of the archive there is nothing to link to, and a theme must
	// be able to tell that apart from an error.
	newest, _ := s.GetPageBySlug(ctx, ws, "dritter")
	prev, next, err = s.AdjacentPosts(ctx, ws, newest)
	if err != nil {
		t.Fatalf("AdjacentPosts at the end: %v", err)
	}
	if next != nil {
		t.Errorf("the newest entry has a next: %v", next)
	}
	if prev == nil || prev.Slug != "zweiter" {
		t.Errorf("prev of the newest = %v, want zweiter", prev)
	}
}

func TestAdjacentPostsIgnorePages(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)

	seedPost(t, s, ws, "Erster", "erster", base)
	post := seedPost(t, s, ws, "Zweiter", "zweiter", base.Add(2*time.Hour))
	// A page published between the two entries must not become a neighbour:
	// following "next" would leave the archive without saying so.
	seed(t, s, ws, "Kontakt", "kontakt", "text", "published")

	prev, next, err := s.AdjacentPosts(ctx, ws, post)
	if err != nil {
		t.Fatalf("AdjacentPosts: %v", err)
	}
	if next != nil {
		t.Errorf("a page became the next entry: %v", next)
	}
	if prev == nil || prev.Slug != "erster" {
		t.Errorf("prev = %v, want erster", prev)
	}
}

func TestChangeKindKeepsTheAddress(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Neues aus der Werkstatt", "neues", "text", "published")

	if err := s.ChangeKind(ctx, p.ID, KindPost); err != nil {
		t.Fatalf("ChangeKind: %v", err)
	}
	moved, _ := s.GetPage(ctx, p.ID)
	if !moved.IsPost() {
		t.Fatal("the record did not become a post")
	}
	// The address is what every existing link points at; moving a record
	// between the two kinds must not touch it.
	if moved.Slug != "neues" {
		t.Errorf("slug changed to %q", moved.Slug)
	}
	if moved.Version <= p.Version {
		t.Error("the version was not raised, so an open editor could overwrite the change unnoticed")
	}
}

func TestNormalizeKindRejectsAnythingElse(t *testing.T) {
	for in, want := range map[string]string{
		"post": KindPost, "page": KindPage, "": KindPage,
		"POST": KindPage, "artikel": KindPage,
	} {
		if got := NormalizeKind(in); got != want {
			t.Errorf("NormalizeKind(%q) = %q, want %q", in, got, want)
		}
	}
}
