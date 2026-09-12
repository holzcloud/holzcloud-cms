package term

import (
	"context"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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
		t.Errorf("public terms = %v, want only Möbel", public) //nolint:german — the message quotes the German fixture it is about
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
		t.Errorf("name = %q, want Möbelbau", renamed.Name) //nolint:german — the message quotes the German fixture it is about
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

// Normalize ist die eine Hälfte von Parse, die von einem einzelnen Namen
// handelt. Was sie nicht tut, ist der Grund, warum sie für sich steht: ein
// Archiv ist keine Seite, es kennt weder Kommas als Trenner noch MaxPerPage.
func TestNormalizeKeepsAWholeNameAndDoesNotCount(t *testing.T) {
	if got, want := Normalize("  Möbel,   Bau  "), "Möbel, Bau"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
	if got := Normalize("   "); got != "" {
		t.Errorf("Normalize(whitespace) = %q, want empty", got)
	}
	long := strings.Repeat("a", MaxNameLength+10)
	if got := Normalize(long); len([]rune(got)) != MaxNameLength {
		t.Errorf("Normalize of an over-long name kept %d runes, want %d", len([]rune(got)), MaxNameLength)
	}

	// Parse keeps its own bound: it reads one entry's field, not an archive.
	many := make([]string, 0, MaxPerPage+3)
	for i := 0; i < MaxPerPage+3; i++ {
		many = append(many, "wort"+strconv.Itoa(i))
	}
	if got := Parse(strings.Join(many, ", ")); len(got) != MaxPerPage {
		t.Errorf("Parse gave %d labels, want %d", len(got), MaxPerPage)
	}
}

// insertProduct writes a catalogue row directly. Raw SQL rather than the shop
// store, so the term package's tests do not take a dependency on it just to
// have a row with a website on it.
func insertProduct(t *testing.T, s *Store, websiteID int64, slug string) int64 {
	t.Helper()
	res, err := s.DB.Write.Exec(
		`INSERT INTO products (website_id, slug, title, price_gross, tax_bp,
			status, created_at, updated_at)
		 VALUES ($1, $2, $2, 4900, 810, 'published', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		websiteID, slug)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func productTermCount(t *testing.T, s *Store, productID int64) int {
	t.Helper()
	var n int
	if err := s.DB.Read.QueryRow(
		`SELECT COUNT(*) FROM product_terms WHERE product_id = $1`, productID).Scan(&n); err != nil {
		t.Fatalf("count product terms: %v", err)
	}
	return n
}

// SetForProduct is handed a website and a product that are supposed to belong
// together. When they do not, it used to clear the foreign product's labels and
// attach one of the caller's own — a row in product_terms spanning two
// websites, which no later code fix can undo. Both halves must now refuse.
func TestSetForProductLeavesAForeignProductAlone(t *testing.T) {
	s, _, websiteA := newTestStore(t)
	ctx := context.Background()

	res, err := s.DB.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Zweite', '')`)
	if err != nil {
		t.Fatalf("insert second website: %v", err)
	}
	websiteB, _ := res.LastInsertId()

	foreign := insertProduct(t, s, websiteB, "fremder-tisch")
	if err := s.SetForProduct(ctx, websiteB, foreign, []string{"Tische", "Massivholz"}); err != nil {
		t.Fatalf("SetForProduct on the owning website: %v", err)
	}
	if got := productTermCount(t, s, foreign); got != 2 {
		t.Fatalf("fixture: the foreign product should start with 2 labels, has %d", got)
	}

	// Website A's id with website B's product: the shape the product save
	// handler used to produce.
	if err := s.SetForProduct(ctx, websiteA, foreign, []string{"Schnäppchen"}); err != nil {
		t.Fatalf("SetForProduct across websites: %v", err)
	}

	if got := productTermCount(t, s, foreign); got != 2 {
		t.Errorf("the foreign product went from 2 labels to %d", got)
	}
}

func TestSetForProductStillLabelsItsOwnProduct(t *testing.T) {
	s, _, websiteID := newTestStore(t)
	ctx := context.Background()

	own := insertProduct(t, s, websiteID, "eigener-tisch")
	if err := s.SetForProduct(ctx, websiteID, own, []string{"Tische", "Massivholz"}); err != nil {
		t.Fatalf("SetForProduct: %v", err)
	}
	if got := productTermCount(t, s, own); got != 2 {
		t.Errorf("the own product has %d labels, want 2", got)
	}

	// Replacing is still replacing, not appending.
	if err := s.SetForProduct(ctx, websiteID, own, []string{"Tische"}); err != nil {
		t.Fatalf("SetForProduct again: %v", err)
	}
	if got := productTermCount(t, s, own); got != 1 {
		t.Errorf("after replacing, the own product has %d labels, want 1", got)
	}
}
