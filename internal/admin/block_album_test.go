package admin

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The gate the plan names for this control is `grep -c 'album' block_list.html`,
// and a grep cannot tell a rendered select from a comment mentioning one. Plan
// 11-04 recorded the same gap for its own display select and could not close it
// there, because the editor form is this package's. It is closed here for both
// halves of what the control has to do: appear at all, and come back carrying
// the choice that was made.

// The select is drawn on a gallery, with the website's albums as its options
// and the empty value meaning the block's own list.
func TestTheGalleryBlockOffersTheWebsitesAlbums(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	albums := album.NewStore(database)
	h.SetAlbumStore(albums)
	if _, err := albums.Create(ctx, ws.ID, "Möbel"); err != nil {
		t.Fatalf("Create album: %v", err)
	}

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"galerie"},
		"bausteinaktion": {"neu:trenner"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if !strings.Contains(body, `name="b0.album"`) {
		t.Errorf("the album select is not on the gallery block — the case in "+
			"setBlockField has no control anybody can reach:\n%s", body)
	}
	if !strings.Contains(body, `<option value="moebel"`) {
		t.Errorf("the website's album is not among the options:\n%s", body)
	}
	if !strings.Contains(body, `<option value="" selected`) {
		t.Errorf("the block's own list is not the selected default:\n%s", body)
	}
}

// The select is guarded to the gallery, exactly as the display one is: a card
// row has items too, and block.Set.Clean drops an album from one anyway.
func TestOnlyAGalleryOffersAnAlbum(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)

	albums := album.NewStore(database)
	h.SetAlbumStore(albums)
	if _, err := albums.Create(context.Background(), ws.ID, "Möbel"); err != nil {
		t.Fatalf("Create album: %v", err)
	}

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"karten"},
		"bausteinaktion": {"neu:trenner"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if strings.Contains(body, `name="b0.album"`) {
		t.Errorf("a card row was offered an album:\n%s", body)
	}
}

// A website with no album gets no select at all, rather than a choice with
// nothing in it. And a build with no album store behaves the way it did before
// albums existed.
func TestNoAlbumsMeansNoSelect(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"galerie"},
		"bausteinaktion": {"neu:trenner"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	body := serve(t, h, sm, h.HandlePageCreate, req).Body.String()

	if strings.Contains(body, `name="b0.album"`) {
		t.Errorf("a select with no options was drawn:\n%s", body)
	}
}

// The whole way through the editor: the choice is posted, stored as a slug,
// rendered as a marker into content_html, and comes back selected in the form.
//
// The last of those four is what nothing else asserts. A select that loses the
// current choice on every redraw is a control that looks right until somebody
// presses another button in the editor.
func TestChoosingAnAlbumIsStoredAndComesBackSelected(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	albums := album.NewStore(database)
	h.SetAlbumStore(albums)
	if _, err := albums.Create(ctx, ws.ID, "Möbel"); err != nil {
		t.Fatalf("Create album: %v", err)
	}

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":   {"galerie"},
		"b0.album": {"moebel"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}
	blocks, err := block.Decode(p.Blocks, block.Builtin)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("blocks = %+v, %v — a gallery naming an album was dropped by the "+
			"save that created it", blocks, err)
	}
	if blocks[0].AlbumSlug != "moebel" {
		t.Errorf("the album was not stored: %+v", blocks[0])
	}
	if !strings.Contains(p.ContentHTML, "[[album:moebel:0]]") {
		t.Errorf("content_html does not carry the marker:\n%s", p.ContentHTML)
	}

	// And back into the form, through an editor action rather than a save.
	back := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"galerie"},
		"b0.album":       {"moebel"},
		"bausteinaktion": {"neu:trenner"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	body := serve(t, h, sm, h.HandlePageCreate, back).Body.String()

	if !strings.Contains(body, `<option value="moebel" selected`) {
		t.Errorf("the chosen album did not come back selected:\n%s", body)
	}
	// The block's own list is folded away rather than thrown away: a collapsed
	// details still posts its fields, so an editor who changes their mind gets
	// the list back.
	if !strings.Contains(body, `<details class="block-own-list">`) {
		t.Errorf("the block's own list is not folded away while an album is chosen:\n%s", body)
	}
}
