package album

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// What this file is the gate on.
//
// An album that another website can read or write is GAL-05 broken, and the
// store — not a handler — is where that is decided. The reason this is a store
// test and not a handler test has a date on it: on 2026-09-06 four menu item
// handlers reached another website's navigation, because internal/menu/store.go
// takes bare primary keys and left the check to seven callers, of which three
// remembered. de4a1ce proves it, 5e453a9 fixes it. A test that exercised the
// handlers would have found four bugs; a test that exercises the store finds
// the one that made four possible.
//
// So every cross-website case below calls the store directly with a correct
// primary key and the wrong website id. That pair is what a URL and a form make
// easy to produce, and refusing it is meant to be a fact of the SQL rather than
// a rule a handler is asked to remember.

// newTestStore opens a migrated database and returns the store beside two
// websites: the one that owns things, and the stranger.
func newTestStore(t *testing.T) (*Store, *db.DB, int64, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return NewStore(database), database, seedWebsite(t, database, "Owner"), seedWebsite(t, database, "Stranger")
}

func seedWebsite(t *testing.T, database *db.DB, name string) int64 {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("insert website %q: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func seedMedia(t *testing.T, database *db.DB, websiteID int64, filename string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes)
		 VALUES ($1, $2, $2, 'image/jpeg', 2048)`, websiteID, filename)
	if err != nil {
		t.Fatalf("insert media %q: %v", filename, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func mustCreate(t *testing.T, s *Store, websiteID int64, name string) *Album {
	t.Helper()
	a, err := s.Create(context.Background(), websiteID, name)
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return a
}

func mustAdd(t *testing.T, s *Store, websiteID, albumID, mediaID int64, alt string) int64 {
	t.Helper()
	id, err := s.AddItem(context.Background(), websiteID, albumID, mediaID, alt, "")
	if err != nil {
		t.Fatalf("AddItem(%q): %v", alt, err)
	}
	return id
}

func TestGetFromAnotherWebsiteFindsNothing(t *testing.T) {
	s, _, owner, stranger := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")

	got, err := s.Get(ctx, stranger, a.ID)
	if err != nil {
		t.Fatalf("Get from the stranger returned an error: %v — not found is not an error, it is a 404", err)
	}
	if got != nil {
		t.Errorf("Get from the stranger returned album %d (%q) — GAL-05 says it is invisible", got.ID, got.Name)
	}

	got, err = s.BySlug(ctx, stranger, a.Slug)
	if err != nil {
		t.Fatalf("BySlug from the stranger returned an error: %v", err)
	}
	if got != nil {
		t.Errorf("BySlug from the stranger returned album %d — the slug is not a global address", got.ID)
	}

	if own, err := s.Get(ctx, owner, a.ID); err != nil || own == nil {
		t.Fatalf("the owner cannot read its own album: %v, %v", own, err)
	}
}

func TestRenameFromAnotherWebsiteIsRefused(t *testing.T) {
	s, _, owner, stranger := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")

	err := s.Rename(ctx, stranger, a.ID, "Taken over")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Rename from the stranger returned %v, want ErrNotFound — zero affected rows is a failure, not a success", err)
	}

	after, err := s.Get(ctx, owner, a.ID)
	if err != nil || after == nil {
		t.Fatalf("read back: %v, %v", after, err)
	}
	if after.Name != "Werkstatt 2024" {
		t.Errorf("the album is now called %q — the stranger's rename went through", after.Name)
	}
}

func TestDeleteFromAnotherWebsiteIsRefused(t *testing.T) {
	s, _, owner, stranger := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")

	err := s.Delete(ctx, stranger, a.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete from the stranger returned %v, want ErrNotFound", err)
	}

	after, err := s.Get(ctx, owner, a.ID)
	if err != nil || after == nil {
		t.Fatalf("the album is gone after a refused delete: %v, %v", after, err)
	}
}

// TestItemMethodsRefuseAnotherWebsitesAlbum is the shape of the 2026-09-06
// defect, one row per method: the item id is correct, the album id is correct,
// and only the website is wrong — which is exactly what a copied URL supplies.
func TestItemMethodsRefuseAnotherWebsitesAlbum(t *testing.T) {
	s, database, owner, stranger := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")
	strangerMedia := seedMedia(t, database, stranger, "other.jpg")
	first := mustAdd(t, s, owner, a.ID, m, "A bench")
	second := mustAdd(t, s, owner, a.ID, m, "The same bench again")

	cases := map[string]func() error{
		"AddItem": func() error {
			_, err := s.AddItem(ctx, stranger, a.ID, strangerMedia, "smuggled", "")
			return err
		},
		"UpdateItem": func() error {
			return s.UpdateItem(ctx, stranger, a.ID, first, strangerMedia, "smuggled", "")
		},
		"DeleteItem": func() error {
			return s.DeleteItem(ctx, stranger, a.ID, first)
		},
		"SwapSortOrder": func() error {
			return s.SwapSortOrder(ctx, stranger, a.ID, first, second)
		},
	}
	for name, call := range cases {
		if err := call(); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s from the stranger returned %v, want ErrNotFound", name, err)
		}
	}

	pictures, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	if len(pictures) != 2 {
		t.Fatalf("the album holds %d pictures, expected the 2 it started with", len(pictures))
	}
	for _, p := range pictures {
		if p.Item.Alt == "smuggled" {
			t.Errorf("picture %d carries the stranger's text", p.ID)
		}
		if p.Item.MediaID == strangerMedia {
			t.Errorf("picture %d points at the stranger's media row", p.ID)
		}
	}
}

// TestAddItemRefusesAnotherWebsitesMedia holds T-11-07. The foreign key on
// media_id proves the file exists, never whose it is, and the id comes out of a
// form — so the store checks the picture's own website before inserting, the
// way internal/admin/page_blocks.go:105-115 already does for a block picture.
func TestAddItemRefusesAnotherWebsitesMedia(t *testing.T) {
	s, database, owner, stranger := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	foreign := seedMedia(t, database, stranger, "not-yours.jpg")

	if _, err := s.AddItem(ctx, owner, a.ID, foreign, "", ""); err == nil {
		t.Fatal("AddItem accepted a media row from another website's library")
	}

	mine := seedMedia(t, database, owner, "bench.jpg")
	item := mustAdd(t, s, owner, a.ID, mine, "A bench")
	if err := s.UpdateItem(ctx, owner, a.ID, item, foreign, "", ""); err == nil {
		t.Fatal("UpdateItem repointed a picture at another website's library")
	}
}

func TestRenameMovesTheNameAndNotTheSlug(t *testing.T) {
	s, _, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")

	if err := s.Rename(ctx, owner, a.ID, "Werkstatt 2025"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	after, err := s.Get(ctx, owner, a.ID)
	if err != nil || after == nil {
		t.Fatalf("read back: %v, %v", after, err)
	}
	if after.Name != "Werkstatt 2025" {
		t.Errorf("Name is %q, want the new one", after.Name)
	}
	if after.Slug != a.Slug {
		t.Errorf("the slug moved from %q to %q — every page naming this album has just lost it", a.Slug, after.Slug)
	}
	if found, err := s.BySlug(ctx, owner, a.Slug); err != nil || found == nil {
		t.Errorf("the old slug no longer finds the album: %v, %v", found, err)
	}
}

func TestDuplicateNameIsANamedError(t *testing.T) {
	s, _, owner, stranger := newTestStore(t)
	ctx := context.Background()
	mustCreate(t, s, owner, "Werkstatt 2024")

	_, err := s.Create(ctx, owner, "Werkstatt  2024")
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("the second album returned %v, want ErrDuplicateName — a handler must not have to match on the SQL text", err)
	}

	// The constraint is per website, not global: two workshops may both have a
	// "Referenzen".
	if _, err := s.Create(ctx, stranger, "Werkstatt 2024"); err != nil {
		t.Errorf("the stranger cannot use a name the owner took: %v", err)
	}
}

func TestNameOfOnlySpacesIsRefused(t *testing.T) {
	s, _, owner, _ := newTestStore(t)
	for _, raw := range []string{"", "   ", "\t\n "} {
		if _, err := s.Create(context.Background(), owner, raw); !errors.Is(err, ErrNoName) {
			t.Errorf("Create(%q) returned %v, want ErrNoName", raw, err)
		}
	}
	list, err := s.List(context.Background(), owner)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("%d albums were created from names that are not names", len(list))
	}
}

func TestItemsComeBackInSortOrder(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")

	want := []string{"first", "second", "third"}
	for _, alt := range want {
		mustAdd(t, s, owner, a.ID, m, alt)
	}

	items, err := s.Items(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != len(want) {
		t.Fatalf("Items returned %d pictures, want %d", len(items), len(want))
	}
	for i, alt := range want {
		if items[i].Alt != alt {
			t.Errorf("picture %d is %q, want %q — the pictures came back out of order", i, items[i].Alt, alt)
		}
		if items[i].MediaID != m {
			t.Errorf("picture %d names media %d, want %d", i, items[i].MediaID, m)
		}
	}

	pictures, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	for i := 1; i < len(pictures); i++ {
		if pictures[i].SortOrder <= pictures[i-1].SortOrder {
			t.Errorf("sort_order %d does not follow %d — AddItem must put a new picture last",
				pictures[i].SortOrder, pictures[i-1].SortOrder)
		}
	}
}

func TestSwapExchangesExactlyTwoRows(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")
	for _, alt := range []string{"first", "second", "third"} {
		mustAdd(t, s, owner, a.ID, m, alt)
	}

	before, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	if err := s.SwapSortOrder(ctx, owner, a.ID, before[0].ID, before[2].ID); err != nil {
		t.Fatalf("SwapSortOrder: %v", err)
	}

	after, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures after the swap: %v", err)
	}
	got := []string{after[0].Item.Alt, after[1].Item.Alt, after[2].Item.Alt}
	want := []string{"third", "second", "first"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after the swap the order is %v, want %v", got, want)
		}
	}
	// The untouched row keeps the number it had, not merely its position: a
	// swap that rewrote the whole list would also pass the check above.
	for _, p := range after {
		if p.Item.Alt == "second" && p.SortOrder != before[1].SortOrder {
			t.Errorf("the untouched picture moved from sort_order %d to %d — this is a rewrite, not a swap",
				before[1].SortOrder, p.SortOrder)
		}
	}
}

func TestAddItemPastTheCapIsRefused(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")

	for i := 0; i < MaxItems; i++ {
		if _, err := s.AddItem(ctx, owner, a.ID, m, fmt.Sprintf("picture %d", i), ""); err != nil {
			t.Fatalf("AddItem %d of %d: %v", i, MaxItems, err)
		}
	}
	if _, err := s.AddItem(ctx, owner, a.ID, m, "one too many", ""); !errors.Is(err, ErrTooManyItems) {
		t.Fatalf("the %dth picture returned %v, want ErrTooManyItems — the cap is reported, never silently applied",
			MaxItems+1, err)
	}
	pictures, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	if len(pictures) != MaxItems {
		t.Errorf("the album holds %d pictures, want the cap of %d", len(pictures), MaxItems)
	}
}

func TestDeletingAnAlbumTakesItsPicturesWithIt(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")
	mustAdd(t, s, owner, a.ID, m, "A bench")
	mustAdd(t, s, owner, a.ID, m, "The same bench again")

	if err := s.Delete(ctx, owner, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var rows int
	if err := database.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM album_items WHERE album_id = $1`, a.ID).Scan(&rows); err != nil {
		t.Fatalf("count picture rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("%d picture rows outlived their album — the cascade did not fire", rows)
	}
}

func TestDeletingAMediaRowRemovesItFromEveryAlbum(t *testing.T) {
	s, database, owner, _ := newTestStore(t)
	ctx := context.Background()
	first := mustCreate(t, s, owner, "Werkstatt 2024")
	second := mustCreate(t, s, owner, "Referenzen")
	m := seedMedia(t, database, owner, "bench.jpg")
	other := seedMedia(t, database, owner, "table.jpg")
	mustAdd(t, s, owner, first.ID, m, "A bench")
	mustAdd(t, s, owner, second.ID, m, "The same bench")
	mustAdd(t, s, owner, second.ID, other, "A table")

	if _, err := database.Write.ExecContext(ctx, `DELETE FROM media WHERE id = $1`, m); err != nil {
		t.Fatalf("delete media row: %v", err)
	}

	if items, err := s.Items(ctx, owner, first.ID); err != nil || len(items) != 0 {
		t.Errorf("the first album still holds %d pictures (%v) of a file that is gone", len(items), err)
	}
	items, err := s.Items(ctx, owner, second.ID)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != 1 || items[0].MediaID != other {
		t.Errorf("the second album holds %d pictures, want only the table", len(items))
	}
}

// TestSwapDoesNotHoldTheWriteConnection measures the property T-11-09 is about
// rather than a proxy for it.
//
// The write pool admits exactly one connection, so a transaction that is
// neither committed nor rolled back never returns it: the very next write on
// the installation blocks, and blocks for busy_timeout — five seconds — before
// it says so. This test runs under a deadline of two seconds, well below that,
// and every ordinary write between the swaps has to get through it.
//
// It is written deliberately in this shape. Phase 9 shipped a gate that counted
// BeginTx occurrences and read zero while a ten-second write-connection stall
// sat one package away, because a count proves that a transaction is not there
// and says nothing about whether the one that is there lets go. The error path
// is included for the same reason: a defer tx.Rollback() that was forgotten
// leaks the connection only when a swap fails, which is the case nobody runs.
func TestSwapDoesNotHoldTheWriteConnection(t *testing.T) {
	s, database, owner, stranger := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	a := mustCreate(t, s, owner, "Werkstatt 2024")
	m := seedMedia(t, database, owner, "bench.jpg")
	for i := 0; i < 4; i++ {
		if _, err := s.AddItem(ctx, owner, a.ID, m, fmt.Sprintf("picture %d", i), ""); err != nil {
			t.Fatalf("AddItem %d: %v", i, err)
		}
	}
	pictures, err := s.Pictures(ctx, owner, a.ID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}

	for i := 0; i < 40; i++ {
		if err := s.SwapSortOrder(ctx, owner, a.ID, pictures[0].ID, pictures[3].ID); err != nil {
			t.Fatalf("swap %d of 40: %v — a swap that does not return the write connection stalls here", i, err)
		}
		// The failed swap is the case that matters: it leaves the transaction
		// open unless the rollback is deferred.
		if err := s.SwapSortOrder(ctx, stranger, a.ID, pictures[0].ID, pictures[3].ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("the stranger's swap %d returned %v, want ErrNotFound", i, err)
		}
		// An ordinary write behind both of them. If either held the single
		// write connection, this is where the deadline arrives.
		if err := s.Rename(ctx, owner, a.ID, fmt.Sprintf("Werkstatt %d", i)); err != nil {
			t.Fatalf("the write after swap %d did not get the connection: %v", i, err)
		}
	}

	if ctx.Err() != nil {
		t.Fatalf("the deadline arrived during the run: %v", ctx.Err())
	}
}
