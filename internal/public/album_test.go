package public

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// albumFixture builds what every test in this file shares: a handler with an
// album store, a website, an album of two pictures, and a published page whose
// stored HTML names that album.
//
// The page is written the way a real save writes it — block.Render over the
// block list, which is exactly what internal/admin/page_blocks.go's
// renderBlocks calls — so what lands in pages.content_html is the marker and
// nothing else about the pictures.
func albumFixture(t *testing.T) (*Handler, *db.DB, *domain.Website, *album.Store, *album.Album) {
	t.Helper()
	ctx := context.Background()

	h, database := newTestHandler(t)
	albums := album.NewStore(database)
	h.SetAlbumStore(albums)

	ws := seedWebsite(t, database, "Test Site")
	a, err := albums.Create(ctx, ws.ID, "Möbel")
	if err != nil {
		t.Fatalf("Create album: %v", err)
	}
	if a.Slug != "moebel" {
		t.Fatalf("album slug = %q, want moebel", a.Slug)
	}
	for _, name := range []string{"schrank.jpg", "tisch.jpg"} {
		if _, err := albums.AddItem(ctx, ws.ID, a.ID,
			seedPicture(t, database, ws.ID, name), "", ""); err != nil {
			t.Fatalf("AddItem %s: %v", name, err)
		}
	}

	seedAlbumPage(t, database, ws.ID, "galerie", block.Block{
		Type: block.TypeGallery, AlbumSlug: a.Slug,
	})
	return h, database, ws, albums, a
}

// seedPicture inserts a media row wide enough for the responsive pass to see
// it, plus one scaled copy, and returns its id.
func seedPicture(t *testing.T, database *db.DB, websiteID int64, filename string) int64 {
	t.Helper()
	ctx := context.Background()
	store := media.NewStore(database)
	m, err := store.Create(ctx, websiteID, filename, filename, "image/jpeg", 4096, filename+"-hash")
	if err != nil {
		t.Fatalf("media.Create %s: %v", filename, err)
	}
	if err := store.SaveVariants(ctx, m.ID, 1600, 1200, []media.Variant{
		{Label: "medium", Filename: strings.TrimSuffix(filename, ".jpg") + "-medium.jpg",
			Width: 800, Height: 600, SizeBytes: 2048},
	}); err != nil {
		t.Fatalf("SaveVariants %s: %v", filename, err)
	}
	return m.ID
}

// seedAlbumPage stores a page whose content_html is what the block renderer
// writes for the given blocks — the ordinary save path, not a hand-built string.
func seedAlbumPage(t *testing.T, database *db.DB, websiteID int64, slug string, blocks ...block.Block) {
	t.Helper()
	encoded, err := block.Encode(blocks, block.Builtin)
	if err != nil {
		t.Fatalf("block.Encode: %v", err)
	}
	if encoded == "" {
		t.Fatal("the blocks were dropped before they were ever stored")
	}
	decoded, err := block.Decode(encoded, block.Builtin)
	if err != nil {
		t.Fatalf("block.Decode: %v", err)
	}
	look := func(int64) (block.Image, bool) { return block.Image{}, false }
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     "Galerie",
		Slug:      slug,
		Markdown:  block.PlainText(decoded, block.Builtin),
		Blocks:    encoded,
		HTML:      block.Render(decoded, block.Builtin, look, page.RenderMarkdown),
		Status:    "published",
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
}

// storedPage reads content_html and updated_at straight out of the table,
// bypassing every layer that might normalise one of them.
func storedPage(t *testing.T, database *db.DB, slug string) (string, string) {
	t.Helper()
	var html, updated string
	err := database.Read.QueryRowContext(context.Background(),
		`SELECT content_html, updated_at FROM pages WHERE slug = $1`, slug).Scan(&html, &updated)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("read stored page: %v", err)
	}
	return html, updated
}

// fetch requests a public page and returns the body.
func fetch(t *testing.T, h *Handler, ws *domain.Website, slug string) string {
	t.Helper()
	rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
		r.SetPathValue("slug", slug)
		return h.HandlePage(w, r)
	}, ws, "GET", "/"+slug)
	if err != nil {
		t.Fatalf("HandlePage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// GAL-03, asserted as a property rather than claimed.
//
// The album changes. The page is not touched — not saved, not re-rendered, not
// migrated. Its content_html and its updated_at are read out of the table
// before and after and must be byte for byte the same. And the page a visitor
// is served must be different.
//
// Without step 3 this test passes for a page that was re-saved, which is
// precisely the thing GAL-03 says must not be necessary. Expanded at save time
// the requirement is false, and nothing anywhere reports it: no test fails, no
// line is logged, and the editor changes the album, reloads, and sees the old
// pictures.
func TestChangingAnAlbumChangesEveryPageThatCarriesIt(t *testing.T) {
	h, database, ws, albums, a := albumFixture(t)

	// 1. what is stored, before.
	htmlBefore, updatedBefore := storedPage(t, database, "galerie")
	if !strings.Contains(htmlBefore, "[[album:moebel:0]]") {
		t.Fatalf("the stored page does not carry the marker — the pictures were "+
			"baked in at save and GAL-03 cannot hold:\n%s", htmlBefore)
	}
	if strings.Contains(htmlBefore, "schrank.jpg") {
		t.Fatalf("a copy of the album's pictures was frozen into content_html:\n%s", htmlBefore)
	}

	// 2. what is served, before.
	before := fetch(t, h, ws, "galerie")
	if !strings.Contains(before, "schrank.jpg") {
		t.Fatalf("the album was not expanded on the way out:\n%s", before)
	}

	// 3. the album changes, and NOTHING else does.
	if _, err := albums.AddItem(context.Background(), ws.ID, a.ID,
		seedPicture(t, database, ws.ID, "bank.jpg"), "", ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// 4. the page is untouched, byte for byte.
	htmlAfter, updatedAfter := storedPage(t, database, "galerie")
	if htmlAfter != htmlBefore {
		t.Errorf("content_html changed although the page was never saved:\nbefore %s\nafter  %s",
			htmlBefore, htmlAfter)
	}
	if updatedAfter != updatedBefore {
		t.Errorf("updated_at moved although the page was never saved: %q -> %q",
			updatedBefore, updatedAfter)
	}

	// 5. and the page a visitor gets is different.
	after := fetch(t, h, ws, "galerie")
	if after == before {
		t.Fatal("the served page did not change when the album did — the expansion " +
			"is not happening at request time, and GAL-03 is false")
	}
	if !strings.Contains(after, "bank.jpg") {
		t.Errorf("the new picture is not on the page:\n%s", after)
	}
}

// A marker whose album was deleted expands to nothing. A visitor must never see
// the internal syntax on a live page, and a deleted album should cost its own
// block and not the article around it.
func TestAlbumMarkerForADeletedAlbumLeavesNoSyntaxOnThePage(t *testing.T) {
	h, database, ws, albums, a := albumFixture(t)

	if err := albums.Delete(context.Background(), ws.ID, a.ID); err != nil {
		t.Fatalf("Delete album: %v", err)
	}

	body := fetch(t, h, ws, "galerie")
	if strings.Contains(body, "[[album:") {
		t.Errorf("the marker is visible to a visitor:\n%s", body)
	}
	if strings.Contains(body, "schrank.jpg") {
		t.Errorf("a deleted album's pictures are still served:\n%s", body)
	}
	if !strings.Contains(body, `<article>`) {
		t.Errorf("the page around the block was damaged:\n%s", body)
	}
	_ = database
}

// GAL-05 on the read path. Another website's album of the same name must not be
// findable through a marker — the check belongs in the query, not in the caller.
func TestAlbumFromAnotherWebsiteExpandsToNothing(t *testing.T) {
	h, database, ws, _, _ := albumFixture(t)
	ctx := context.Background()

	other := seedWebsite(t, database, "Andere Website")
	albums := album.NewStore(database)
	foreign, err := albums.Create(ctx, other.ID, "Möbel")
	if err != nil {
		t.Fatalf("Create foreign album: %v", err)
	}
	if foreign.Slug != "moebel" {
		t.Fatalf("the two albums do not share a slug, so this proves nothing: %q", foreign.Slug)
	}
	if _, err := albums.AddItem(ctx, other.ID, foreign.ID,
		seedPicture(t, database, other.ID, "fremd.jpg"), "", ""); err != nil {
		t.Fatalf("AddItem foreign: %v", err)
	}

	body := fetch(t, h, ws, "galerie")
	if strings.Contains(body, "fremd.jpg") {
		t.Errorf("another website's picture is on this website's page:\n%s", body)
	}
	if !strings.Contains(body, "schrank.jpg") {
		t.Errorf("the own album stopped resolving:\n%s", body)
	}
}

// The ordering, asserted rather than reasoned about.
//
// The expansion runs before h.responsive, so media.MakeResponsive sees the img
// tags it produces and adds their candidate list. An expansion placed after
// that call would produce pictures with no srcset — and nothing anywhere would
// report it.
func TestExpandedAlbumPicturesGetTheirSrcSet(t *testing.T) {
	h, _, ws, _, _ := albumFixture(t)

	body := fetch(t, h, ws, "galerie")

	if !strings.Contains(body, "schrank-medium.jpg") {
		t.Errorf("the expanded picture has no candidate list — the expansion ran "+
			"after the responsive rewrite:\n%s", body)
	}
	if !strings.Contains(body, "srcset=") {
		t.Errorf("no srcset anywhere on the page:\n%s", body)
	}
}

// A page that names no album must issue no album query at all.
//
// Asserted on the store rather than on the output: the handler is given an
// album store with no database, so any query at all is a nil dereference. The
// only reason the request succeeds is the early return in LoadFor on a document
// whose HTML names no album.
func TestPageWithNoAlbumIssuesNoAlbumQuery(t *testing.T) {
	h, database := newTestHandler(t)
	h.SetAlbumStore(&album.Store{})

	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "# Über uns", "published")

	body := fetch(t, h, ws, "ueber-uns")
	if !strings.Contains(body, "Über uns") {
		t.Errorf("the page did not render:\n%s", body)
	}
}
