package album

import (
	"context"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// The stamp, held where it is written.
//
// albums.updated_at is not bookkeeping: it is the Last-Modified every public
// page carrying the album is served with (contentModTime,
// internal/public/pagedata.go). A page that names an album is not touched when
// the album changes — that is GAL-03 — so this column is the only thing a
// browser holding a conditional request can be told the truth by.
//
// Every test below drives the stamp back into the past first and then makes one
// change through the store. Back into the past rather than sleeping a second,
// because the column has second resolution: a test that made a change in the
// same second it created the album would prove nothing either way, and one that
// slept would cost a second per case for the privilege.

// ageAlbum drives an album's updated_at to a fixed moment well in the past, so
// that "did the change move it" has an answer that does not depend on how fast
// the machine is.
func ageAlbum(t *testing.T, s *Store, albumID int64) time.Time {
	t.Helper()
	old := "2020-01-01T00:00:00Z"
	if _, err := s.DB.Write.Exec(
		`UPDATE albums SET updated_at = $1 WHERE id = $2`, old, albumID); err != nil {
		t.Fatalf("age album %d: %v", albumID, err)
	}
	at, err := time.Parse(timeLayout, old)
	if err != nil {
		t.Fatalf("parse %q: %v", old, err)
	}
	return at
}

func stampOf(t *testing.T, s *Store, websiteID, albumID int64) time.Time {
	t.Helper()
	a, err := s.Get(context.Background(), websiteID, albumID)
	if err != nil || a == nil {
		t.Fatalf("Get album %d: %v, %v", albumID, a, err)
	}
	return a.UpdatedAt
}

// TestCreateStampsTheAlbum: 00051 could not give updated_at a default, so the
// INSERT has to name it. An album whose stamp is the empty string is an album
// that says "1 January year one" to every cache that asks.
func TestCreateStampsTheAlbum(t *testing.T) {
	s, _, owner, _ := newTestStore(t)
	a := mustCreate(t, s, owner, "Werkstatt")

	if a.UpdatedAt.IsZero() {
		t.Fatal("a new album carries no updated_at — the INSERT does not name the column 00051 could not give a default")
	}
	if a.UpdatedAt.Before(a.CreatedAt) {
		t.Errorf("updated_at %v is older than created_at %v", a.UpdatedAt, a.CreatedAt)
	}
}

// TestEveryChangeToAnAlbumMovesItsStamp is the whole property in one table.
//
// Five ways to change what an album shows, and each of them has to move the
// stamp. The removal is the case that decides where the stamp lives at all: the
// deleted row takes its own updated_at with it, so nothing derived from
// album_items can report it.
func TestEveryChangeToAnAlbumMovesItsStamp(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		do   func(t *testing.T, s *Store, database *db.DB, owner, albumID int64)
	}{
		{"rename", func(t *testing.T, s *Store, _ *db.DB, owner, albumID int64) {
			if err := s.Rename(ctx, owner, albumID, "Werkstatt 2026"); err != nil {
				t.Fatalf("Rename: %v", err)
			}
		}},
		{"a picture added", func(t *testing.T, s *Store, d *db.DB, owner, albumID int64) {
			mustAdd(t, s, owner, albumID, seedMedia(t, d, owner, "new.jpg"), "New")
		}},
		{"a caption corrected", func(t *testing.T, s *Store, d *db.DB, owner, albumID int64) {
			pics, err := s.Pictures(ctx, owner, albumID)
			if err != nil || len(pics) == 0 {
				t.Fatalf("Pictures: %v, %v", pics, err)
			}
			if err := s.UpdateItem(ctx, owner, albumID, pics[0].ID,
				pics[0].Item.MediaID, "corrected", "corrected"); err != nil {
				t.Fatalf("UpdateItem: %v", err)
			}
		}},
		{"a picture removed", func(t *testing.T, s *Store, d *db.DB, owner, albumID int64) {
			pics, err := s.Pictures(ctx, owner, albumID)
			if err != nil || len(pics) == 0 {
				t.Fatalf("Pictures: %v, %v", pics, err)
			}
			if err := s.DeleteItem(ctx, owner, albumID, pics[0].ID); err != nil {
				t.Fatalf("DeleteItem: %v", err)
			}
		}},
		{"two pictures reordered", func(t *testing.T, s *Store, d *db.DB, owner, albumID int64) {
			pics, err := s.Pictures(ctx, owner, albumID)
			if err != nil || len(pics) < 2 {
				t.Fatalf("Pictures: %v, %v", pics, err)
			}
			if err := s.SwapSortOrder(ctx, owner, albumID, pics[0].ID, pics[1].ID); err != nil {
				t.Fatalf("SwapSortOrder: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, database, owner, _ := newTestStore(t)
			a := mustCreate(t, s, owner, "Werkstatt")
			mustAdd(t, s, owner, a.ID, seedMedia(t, database, owner, "one.jpg"), "One")
			mustAdd(t, s, owner, a.ID, seedMedia(t, database, owner, "two.jpg"), "Two")

			was := ageAlbum(t, s, a.ID)
			tc.do(t, s, database, owner, a.ID)

			now := stampOf(t, s, owner, a.ID)
			if !now.After(was) {
				t.Errorf("%s left updated_at at %v — every page carrying this album is still served the old validator, and a warm browser keeps the old gallery",
					tc.name, now)
			}
		})
	}
}

// TestLoadForReportsTheNewestAlbumItNames is the read half: the value the
// public page's validator is built from.
//
// It is read from the albums and not from their pictures, and the last case is
// why: an album with no pictures at all still has to answer when it last
// changed, and a query hanging off album_items answers nothing for it.
func TestLoadForReportsTheNewestAlbumItNames(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()

	old := mustCreate(t, s, owner, "Alt")
	mustAdd(t, s, owner, old.ID, seedMedia(t, database, owner, "alt.jpg"), "Alt")
	ageAlbum(t, s, old.ID)

	empty := mustCreate(t, s, owner, "Leer")

	// A page naming only the old album is served the old album's stamp.
	set, err := s.LoadFor(ctx, owner, pageWith(old.Slug), nil)
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}
	if got := set.Latest().UTC().Format(timeLayout); got != "2020-01-01T00:00:00Z" {
		t.Errorf("Latest = %s; want the aged album's own stamp", got)
	}

	// A page naming an album with no pictures still learns when it changed.
	set, err = s.LoadFor(ctx, owner, pageWith(empty.Slug), nil)
	if err != nil {
		t.Fatalf("LoadFor (empty album): %v", err)
	}
	if set.Latest().IsZero() {
		t.Error("an album with no pictures reported no timestamp — a page carrying it can never be told its gallery emptied")
	}

	// A page naming none pays for nothing.
	set, err = s.LoadFor(ctx, owner, "<p>nothing here</p>", nil)
	if err != nil {
		t.Fatalf("LoadFor (no marker): %v", err)
	}
	if !set.Latest().IsZero() {
		t.Errorf("a page naming no album reported %v", set.Latest())
	}
}

// pageWith is what a saved page carrying one album gallery looks like: the
// wrapper written at save, and the marker where the pictures go.
func pageWith(slug string) string {
	return `<div class="hc-block hc-galerie">` + block.AlbumMarker(slug, 0) + `</div>`
}
