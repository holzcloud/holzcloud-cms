package term

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func newTestStore(t *testing.T) (*Store, *page.Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Test', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	id, _ := res.LastInsertId()
	return NewStore(database), page.NewStore(database), id
}

func seedPage(t *testing.T, s *page.Store, websiteID int64, title, slug, status string) *page.Page {
	t.Helper()
	p, err := s.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug,
		Markdown: title, HTML: "<p>" + title + "</p>", Status: status,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return p
}

func TestParseFoldsSpellingsOfTheSameLabel(t *testing.T) {
	cases := map[string][]string{
		"Möbel, Eiche":   {"Möbel", "Eiche"},
		"Möbel, möbel":   {"Möbel"},
		"Möbel ,, Eiche": {"Möbel", "Eiche"},
		"  Möbel  bau  ": {"Möbel bau"},
		"":               nil,
		",  ,":           nil,
	}
	for raw, want := range cases {
		if got := Parse(raw); !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestParseBoundsTheNumberOfLabels(t *testing.T) {
	raw := ""
	for i := 0; i < 30; i++ {
		raw += string(rune('a'+i%26)) + string(rune('a'+i/26)) + ", "
	}
	if got := Parse(raw); len(got) != MaxPerPage {
		t.Errorf("Parse kept %d labels, want the cap of %d", len(got), MaxPerPage)
	}
}

func TestSetForPageReplacesRatherThanAccumulates(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()
	p := seedPage(t, pages, ws, "Werkbank", "werkbank", "published")

	if err := store.SetForPage(ctx, ws, p.ID, []string{"Möbel", "Eiche"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	if err := store.SetForPage(ctx, ws, p.ID, []string{"Eiche", "Werkstatt"}); err != nil {
		t.Fatalf("second SetForPage: %v", err)
	}

	terms, err := store.ForPage(ctx, p.ID)
	if err != nil {
		t.Fatalf("ForPage: %v", err)
	}
	if got := Format(terms); got != "Eiche, Werkstatt" {
		t.Errorf("labels = %q, want \"Eiche, Werkstatt\"", got)
	}
}

func TestSetForPageKeepsTheFirstSpellingOfALabel(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()
	first := seedPage(t, pages, ws, "Eins", "eins", "published")
	second := seedPage(t, pages, ws, "Zwei", "zwei", "published")

	if err := store.SetForPage(ctx, ws, first.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	// A second editor typing it in lower case must not rename the archive that
	// the first one created and that links already point at.
	if err := store.SetForPage(ctx, ws, second.ID, []string{"möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}

	terms, _ := store.ForPage(ctx, second.ID)
	if len(terms) != 1 || terms[0].Name != "Möbel" {
		t.Errorf("labels = %v, want the original spelling", terms)
	}
	all, _ := store.ListAll(ctx, ws)
	if len(all) != 1 {
		t.Errorf("two spellings produced %d labels, want one", len(all))
	}
}

func TestListWithCountsHidesLabelsOnlyDraftsCarry(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()
	live := seedPage(t, pages, ws, "Sichtbar", "sichtbar", "published")
	draft := seedPage(t, pages, ws, "Entwurf", "entwurf", "draft")

	if err := store.SetForPage(ctx, ws, live.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	if err := store.SetForPage(ctx, ws, draft.ID, []string{"Geheim"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}

	public, err := store.ListWithCounts(ctx, ws)
	if err != nil {
		t.Fatalf("ListWithCounts: %v", err)
	}
	if len(public) != 1 || public[0].Name != "Möbel" {
		// A label leading to an empty archive also tells a visitor that an
		// unpublished page exists under that name.
		t.Errorf("public labels = %v, want only Möbel", public)
	}
	if public[0].Count != 1 {
		t.Errorf("count = %d, want 1", public[0].Count)
	}

	// The admin sees everything, including what only drafts carry.
	all, err := store.ListAll(ctx, ws)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("admin labels = %v, want both", all)
	}
}

func TestListTaggedShowsPagesAndPostsButNotDrafts(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()

	live := seedPage(t, pages, ws, "Sichtbar", "sichtbar", "published")
	draft := seedPage(t, pages, ws, "Entwurf", "entwurf", "draft")
	post, err := pages.CreatePage(ctx, page.PageCreate{
		WebsiteID: ws, Title: "Beitrag", Slug: "beitrag",
		Markdown: "x", HTML: "<p>x</p>", Status: "published", Kind: page.KindPost,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	for _, id := range []int64{live.ID, draft.ID, post.ID} {
		if err := store.SetForPage(ctx, ws, id, []string{"Möbel"}); err != nil {
			t.Fatalf("SetForPage: %v", err)
		}
	}

	label, err := store.GetBySlug(ctx, ws, "moebel")
	if err != nil || label == nil {
		t.Fatalf("GetBySlug: %v %v", label, err)
	}

	items, total, err := store.ListTagged(ctx, ws, label.ID, 1, 10)
	if err != nil {
		t.Fatalf("ListTagged: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want the page and the post but not the draft", total)
	}
	for _, item := range items {
		if item.Slug == "entwurf" {
			t.Error("a draft appeared in a public label archive")
		}
	}
}

func TestRenameKeepsTheAddress(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()
	p := seedPage(t, pages, ws, "Seite", "seite", "published")
	if err := store.SetForPage(ctx, ws, p.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}

	label, _ := store.GetBySlug(ctx, ws, "moebel")
	if err := store.Rename(ctx, ws, label.ID, "Möbelbau"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Every link that already exists points at the slug; changing it would
	// break them all for the sake of a display name.
	renamed, err := store.GetBySlug(ctx, ws, "moebel")
	if err != nil || renamed == nil {
		t.Fatalf("the address changed with the name: %v %v", renamed, err)
	}
	if renamed.Name != "Möbelbau" {
		t.Errorf("name = %q, want Möbelbau", renamed.Name)
	}
}

func TestDeleteRemovesTheLabelButNotTheContent(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()
	p := seedPage(t, pages, ws, "Seite", "seite", "published")
	if err := store.SetForPage(ctx, ws, p.ID, []string{"Möbel"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}

	label, _ := store.GetBySlug(ctx, ws, "moebel")
	if err := store.Delete(ctx, ws, label.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if terms, _ := store.ForPage(ctx, p.ID); len(terms) != 0 {
		t.Errorf("the page still carries %v", terms)
	}
	if kept, _ := pages.GetPage(ctx, p.ID); kept == nil {
		t.Error("deleting a label deleted the page")
	}
}

func TestForPagesLoadsAWholeListingAtOnce(t *testing.T) {
	store, pages, ws := newTestStore(t)
	ctx := context.Background()

	a := seedPage(t, pages, ws, "A", "a", "published")
	b := seedPage(t, pages, ws, "B", "b", "published")
	c := seedPage(t, pages, ws, "C", "c", "published")

	if err := store.SetForPage(ctx, ws, a.ID, []string{"Möbel", "Eiche"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}
	if err := store.SetForPage(ctx, ws, b.ID, []string{"Eiche"}); err != nil {
		t.Fatalf("SetForPage: %v", err)
	}

	byPage, err := store.ForPages(ctx, []int64{a.ID, b.ID, c.ID})
	if err != nil {
		t.Fatalf("ForPages: %v", err)
	}
	if len(byPage[a.ID]) != 2 || len(byPage[b.ID]) != 1 {
		t.Errorf("labels per page: a=%v b=%v", byPage[a.ID], byPage[b.ID])
	}
	if _, ok := byPage[c.ID]; ok {
		t.Error("a page with no labels got an entry, which a template would render as an empty list")
	}
}
