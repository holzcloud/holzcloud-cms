package page

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// newTestStore opens a real migrated SQLite database. The interesting parts of
// the store are transactions and constraints, which a fake would not have.
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

	res, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ('Test', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	websiteID, _ := res.LastInsertId()

	return NewStore(database), websiteID
}

func seed(t *testing.T, s *Store, websiteID int64, title, slug, body, status string) *Page {
	t.Helper()
	p, err := s.CreatePage(context.Background(), PageCreate{
		WebsiteID: websiteID,
		Title:     title,
		Slug:      slug,
		Markdown:  body,
		HTML:      "<p>" + body + "</p>",
		Status:    status,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return p
}

// Two editors open the same page. The second save must be refused rather than
// silently dropping the first editor's paragraphs.
func TestUpdatePageRefusesAStaleVersion(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "erster text", "draft")

	stale := p.Version

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "editor A", HTML: "<p>A</p>",
		Status: "draft", ExpectedVersion: stale,
	}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "editor B", HTML: "<p>B</p>",
		Status: "draft", ExpectedVersion: stale,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("second save with a stale version: got %v, want ErrConflict", err)
	}

	after, err := s.GetPage(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.ContentMarkdown != "editor A" {
		t.Errorf("first editor's text was overwritten: %q", after.ContentMarkdown)
	}
}

func TestUpdatePageRecordsTheReplacedTextAsARevision(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "fassung eins", "draft")

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "fassung zwei", HTML: "<p>2</p>",
		Status: "draft", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	revs, err := s.ListRevisions(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	if revs[0].ContentMarkdown != "fassung eins" {
		t.Errorf("revision holds %q, want the replaced text", revs[0].ContentMarkdown)
	}
}

// Publishing changes no content, so it must not push a real edit out of the
// twenty-entry history.
func TestPublishingDoesNotCreateARevision(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "text", "draft")

	if err := s.UpdatePage(ctx, p.ID, PageUpdate{
		Title: "Titel", Slug: "titel", Markdown: "text", HTML: "<p>text</p>",
		Status: "published", ExpectedVersion: p.Version,
	}); err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	revs, err := s.ListRevisions(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("got %d revisions for a status-only change, want 0", len(revs))
	}
}

func TestRevisionsArePrunedToTheLimit(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "start", "draft")

	version := p.Version
	for i := 0; i < maxRevisions+5; i++ {
		body := "fassung " + time.Duration(i).String()
		if err := s.UpdatePage(ctx, p.ID, PageUpdate{
			Title: "Titel", Slug: "titel", Markdown: body, HTML: "<p>x</p>",
			Status: "draft", ExpectedVersion: version,
		}); err != nil {
			t.Fatalf("UpdatePage %d: %v", i, err)
		}
		version++
	}

	revs, err := s.ListRevisions(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revs) != maxRevisions {
		t.Errorf("got %d revisions, want the cap of %d", len(revs), maxRevisions)
	}
}

// Unpublishing and republishing must not make an old page look newly written,
// which is what a plain "set published_at on publish" would do.
func TestPublishedAtSurvivesAnUnpublishCycle(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Titel", "titel", "text", "published")

	if p.PublishedAt == nil {
		t.Fatal("a page created as published has no published_at")
	}
	first := *p.PublishedAt

	if err := s.SetPageStatus(ctx, p.ID, "draft", nil); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if err := s.SetPageStatus(ctx, p.ID, "published", nil); err != nil {
		t.Fatalf("republish: %v", err)
	}

	after, err := s.GetPage(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.PublishedAt == nil || !after.PublishedAt.Equal(first) {
		t.Errorf("published_at is %v, want it unchanged at %v", after.PublishedAt, first)
	}
}

// A trashed page must disappear from the public site immediately, and must not
// keep blocking its own address.
func TestTrashHidesThePageAndFreesItsAddress(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Impressum", "impressum", "text", "published")

	if err := s.TrashPage(ctx, p.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}

	got, err := s.GetPublishedPage(ctx, ws, "impressum")
	if err != nil {
		t.Fatalf("GetPublishedPage: %v", err)
	}
	if got != nil {
		t.Error("a trashed page is still served publicly")
	}

	// The address has to be usable again straight away.
	replacement := seed(t, s, ws, "Impressum", "impressum", "neu", "published")
	if replacement.Slug != "impressum" {
		t.Errorf("new page got slug %q, want impressum — the trashed row still holds it", replacement.Slug)
	}
}

func TestRestoreBringsBackTheOriginalAddress(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Kontakt", "kontakt", "text", "published")

	if err := s.TrashPage(ctx, p.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}
	if err := s.RestorePage(ctx, p.ID); err != nil {
		t.Fatalf("RestorePage: %v", err)
	}

	after, err := s.GetPage(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.Slug != "kontakt" {
		t.Errorf("restored slug is %q, want kontakt", after.Slug)
	}
	if after.InTrash() {
		t.Error("restored page is still marked as deleted")
	}
}

// Restoring into an address that was taken over in the meantime must not fail —
// the content matters more than the address.
func TestRestoreUniquifiesATakenAddress(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Kontakt", "kontakt", "alt", "published")

	if err := s.TrashPage(ctx, p.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}
	seed(t, s, ws, "Kontakt", "kontakt", "neu", "published")

	if err := s.RestorePage(ctx, p.ID); err != nil {
		t.Fatalf("RestorePage: %v", err)
	}
	after, err := s.GetPage(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if after.Slug == "kontakt" || after.InTrash() {
		t.Errorf("restored page has slug %q, trashed=%v", after.Slug, after.InTrash())
	}
}

func TestListPagesExcludesTheTrash(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	keep := seed(t, s, ws, "Bleibt", "bleibt", "text", "draft")
	gone := seed(t, s, ws, "Weg", "weg", "text", "draft")

	if err := s.TrashPage(ctx, gone.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}

	pages, total, err := s.ListPages(ctx, ws, ListFilter{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if total != 1 || len(pages) != 1 || pages[0].ID != keep.ID {
		t.Errorf("list returned %d of %d pages, want only the live one", len(pages), total)
	}

	trash, err := s.ListTrash(ctx, ws)
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(trash) != 1 || trash[0].Slug != "weg" {
		t.Errorf("trash listing is %+v, want the original address", trash)
	}
}

func TestPurgeExpiredTrashKeepsRecentDeletions(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	p := seed(t, s, ws, "Frisch", "frisch", "text", "draft")
	if err := s.TrashPage(ctx, p.ID); err != nil {
		t.Fatalf("TrashPage: %v", err)
	}

	n, err := s.PurgeExpiredTrash(ctx, TrashRetention)
	if err != nil {
		t.Fatalf("PurgeExpiredTrash: %v", err)
	}
	if n != 0 {
		t.Errorf("purged %d pages, want 0 — the deletion is minutes old", n)
	}

	// Backdate the deletion past the retention window, which is what the clock
	// does in production.
	old := time.Now().UTC().Add(-TrashRetention - time.Hour).Format(timeLayout)
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET deleted_at = $1 WHERE id = $2`, old, p.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	if n, err = s.PurgeExpiredTrash(ctx, TrashRetention); err != nil {
		t.Fatalf("PurgeExpiredTrash: %v", err)
	}
	if n != 1 {
		t.Errorf("purged %d pages, want 1", n)
	}
}

// A rename onto an occupied address is an explicit choice, so it is reported
// rather than quietly turned into "kontakt-2".
func TestUpdatePageReportsATakenAddress(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	seed(t, s, ws, "Kontakt", "kontakt", "text", "draft")
	other := seed(t, s, ws, "Impressum", "impressum", "text", "draft")

	err := s.UpdatePage(ctx, other.ID, PageUpdate{
		Title: "Impressum", Slug: "kontakt", Markdown: "text", HTML: "<p>text</p>",
		Status: "draft", ExpectedVersion: other.Version,
	})
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("got %v, want ErrSlugTaken", err)
	}
}
