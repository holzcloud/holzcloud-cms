package media

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*Store, int64) {
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
	return NewStore(database), id
}

func seedPage(t *testing.T, s *Store, websiteID int64, title, slug string) int64 {
	t.Helper()
	res, err := s.DB.Write.Exec(
		`INSERT INTO pages (website_id, title, slug, content_markdown, content_html, status)
		 VALUES ($1, $2, $3, '', '', 'published')`, websiteID, title, slug)
	if err != nil {
		t.Fatalf("insert page: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// A parser, not a regexp: attributes can be single-quoted, unquoted or spread
// over lines, and srcset holds several URLs with descriptors.
func TestExtractRefsFindsEveryReferenceForm(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg", "d.pdf", "e.jpg"} {
		if _, err := s.Create(ctx, ws, name, name, "image/jpeg", 10, "hash-"+name); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	html := `
<p><img src='/media/1/a.jpg' alt="einfach"></p>
<picture>
  <source srcset="/media/1/b.jpg 800w, /media/1/c.jpg 1600w">
  <img
     src=/media/1/a.jpg>
</picture>
<p><a href="/media/1/d.pdf?v=2">Preisliste</a></p>
<p><img src="https://fremd.example/media/1/e.jpg"></p>
`
	ids, err := ExtractRefs(ctx, s, ws, html)
	if err != nil {
		t.Fatalf("ExtractRefs: %v", err)
	}
	if len(ids) != 4 {
		t.Fatalf("found %d references, want 4 (a, b, c, d — not the foreign e): %v", len(ids), ids)
	}
}

// Deleting a file used to break every page showing it, silently.
func TestDeleteRefusesAFileStillInUse(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()

	m, err := s.Create(ctx, ws, "foto.jpg", "foto.jpg", "image/jpeg", 10, "hash")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pageID := seedPage(t, s, ws, "Leistungen", "leistungen")
	if err := s.ReplaceUsage(ctx, pageID, []int64{m.ID}); err != nil {
		t.Fatalf("ReplaceUsage: %v", err)
	}

	err = s.Delete(ctx, m.ID, dir, false)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("got %v, want InUseError", err)
	}
	if len(inUse.Pages) != 1 || inUse.Pages[0] != "Leistungen" {
		t.Errorf("the error does not name the page: %+v", inUse.Pages)
	}

	// The explicit override still works.
	if err := s.Delete(ctx, m.ID, dir, true); err != nil {
		t.Fatalf("forced delete: %v", err)
	}
	if got, _ := s.GetByID(ctx, m.ID); got != nil {
		t.Error("the forced delete did not remove the row")
	}
}

// Trashing the page that used a file must release it, or the file can never be
// deleted again.
func TestUsageIgnoresTrashedPages(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	m, _ := s.Create(ctx, ws, "foto.jpg", "foto.jpg", "image/jpeg", 10, "hash")
	pageID := seedPage(t, s, ws, "Alt", "alt")
	if err := s.ReplaceUsage(ctx, pageID, []int64{m.ID}); err != nil {
		t.Fatalf("ReplaceUsage: %v", err)
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = $1`, pageID); err != nil {
		t.Fatalf("trash page: %v", err)
	}

	pages, err := s.UsedOnPages(ctx, m.ID)
	if err != nil {
		t.Fatalf("UsedOnPages: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("a trashed page still counts as a use: %v", pages)
	}
}

// The "unused" filter is what actually reclaims space on the card.
func TestUnusedFilterFindsOrphans(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	used, _ := s.Create(ctx, ws, "used.jpg", "used.jpg", "image/jpeg", 10, "h1")
	s.Create(ctx, ws, "orphan.jpg", "orphan.jpg", "image/jpeg", 10, "h2")
	pageID := seedPage(t, s, ws, "Seite", "seite")
	s.ReplaceUsage(ctx, pageID, []int64{used.ID})

	items, total, err := s.List(ctx, ws, Filter{Unused: true}, 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Filename != "orphan.jpg" {
		t.Errorf("unused filter returned %+v, want only orphan.jpg", items)
	}
}

func TestFilterMatchesNameAndAltText(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	a, _ := s.Create(ctx, ws, "x1.jpg", "werkstatt.jpg", "image/jpeg", 10, "h1")
	b, _ := s.Create(ctx, ws, "x2.jpg", "IMG_4711.jpg", "image/jpeg", 10, "h2")
	if err := s.UpdateMeta(ctx, b.ID, "Treppe aus Eiche", ""); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}
	_ = a

	byName, _, _ := s.List(ctx, ws, Filter{Query: "werkstatt"}, 1, 20)
	if len(byName) != 1 || byName[0].OriginalName != "werkstatt.jpg" {
		t.Errorf("name search returned %+v", byName)
	}

	// Searching the description is the point of storing it: nobody remembers
	// that the picture of the staircase is called IMG_4711.
	byAlt, _, _ := s.List(ctx, ws, Filter{Query: "eiche"}, 1, 20)
	if len(byAlt) != 1 || byAlt[0].Filename != "x2.jpg" {
		t.Errorf("alt-text search returned %+v", byAlt)
	}
}

func TestFindByHashSpotsADuplicate(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, ws, "a.jpg", "foto.jpg", "image/jpeg", 10, "abc123"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dup, err := s.FindByHash(ctx, ws, "abc123")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if dup == nil || dup.OriginalName != "foto.jpg" {
		t.Errorf("got %+v, want the existing file", dup)
	}

	// A hash from another website must not match.
	if other, _ := s.FindByHash(ctx, ws+999, "abc123"); other != nil {
		t.Error("a hash matched across websites")
	}
}

func TestCountMissingAltTextCountsImagesOnly(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()

	s.Create(ctx, ws, "a.jpg", "a.jpg", "image/jpeg", 10, "h1")
	s.Create(ctx, ws, "b.pdf", "b.pdf", "application/pdf", 10, "h2")
	described, _ := s.Create(ctx, ws, "c.png", "c.png", "image/png", 10, "h3")
	s.UpdateMeta(ctx, described.ID, "Beschreibung", "")

	n, err := s.CountMissingAltText(ctx, ws)
	if err != nil {
		t.Fatalf("CountMissingAltText: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1 — the PDF needs no alt text and c.png has one", n)
	}
}
