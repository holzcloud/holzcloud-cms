package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// A gallery bound to an album, deleted by a save that never touched it.
//
// The album <select> is the ONLY carrier of AlbumSlug in the block editor, and
// block.FromForm rebuilds every block purely from the posted fields. An absent
// or empty bN.album therefore means AlbumSlug == "", Empty() reports the
// gallery as empty and Clean drops the block — with the flash saying "Seite
// gespeichert". This is the same silent deletion 11-05 fixed, arriving through
// a different door.
//
// Two doors, and neither of them needs anything to have gone wrong:
//
//	(a) the named album was deleted. Other albums remain, so the select IS
//	    drawn — but no <option> matches, so none carries selected and the
//	    browser submits the first, value="".
//	(b) the website has no albums left, so {{if .Albums}} is false and the
//	    select is not drawn at all.
//
// TestAlbumBlockSurvivesClean covers only the case where the block still HAS a
// slug; TestNoAlbumsMeansNoSelect asserts the cause of (b) as correct behaviour
// and never follows it to the save.

// blockEditorHTML draws the block editor for one gallery block through an
// editor action, which is the redraw every one of these cases goes through.
func blockEditorHTML(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, form url.Values) string {
	t.Helper()
	form["bausteinaktion"] = []string{"neu:trenner"}
	req := postForm("/admin/websites/1/pages/new", blockForm(websiteID, form),
		map[string]string{"id": strconv.FormatInt(websiteID, 10)})
	return serve(t, h, sm, h.HandlePageCreate, req).Body.String()
}

// TestAGalleryKeepsItsAlbumWhenTheAlbumIsGone is door (a).
func TestAGalleryKeepsItsAlbumWhenTheAlbumIsGone(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	albums := album.NewStore(database)
	h.SetAlbumStore(albums)
	gone, err := albums.Create(ctx, ws.ID, "Sommer 2025")
	if err != nil {
		t.Fatalf("Create album: %v", err)
	}
	if _, err := albums.Create(ctx, ws.ID, "Möbel"); err != nil {
		t.Fatalf("Create second album: %v", err)
	}
	if err := albums.Delete(ctx, ws.ID, gone.ID); err != nil {
		t.Fatalf("Delete album: %v", err)
	}

	// The redraw: the select is there, and the option carrying the block's own
	// slug is the selected one.
	body := blockEditorHTML(t, h, sm, ws.ID, url.Values{
		"b0.typ":   {"galerie"},
		"b0.album": {gone.Slug},
	})
	if !strings.Contains(body, `name="b0.album"`) {
		t.Fatalf("no album select at all:\n%s", body)
	}
	if !strings.Contains(body, `<option value="`+gone.Slug+`" selected`) {
		t.Fatalf("the block's own album has no selected option, so the browser will submit the first one (value=\"\") and the next save deletes the gallery:\n%s", body)
	}
	if !strings.Contains(body, "gibt es nicht mehr") {
		t.Errorf("the option does not say that this album is gone, so the editor cannot tell it from a real one:\n%s", body)
	}

	// And the save that follows keeps the block. This is the assertion the
	// whole finding is about: the effect, not the markup.
	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":   {"galerie"},
		"b0.album": {gone.Slug},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}
	blocks, err := block.Decode(p.Blocks, block.Builtin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("%d blocks instead of 1 — the gallery block was deleted by a save that never touched it: %+v", len(blocks), blocks)
	}
	if blocks[0].AlbumSlug != gone.Slug {
		t.Errorf("the block's album is %q, want %q — the reference was posted away", blocks[0].AlbumSlug, gone.Slug)
	}
}

// TestAGalleryKeepsItsAlbumWhenThereIsNoListAtAll is door (b): no albums on the
// website, so nothing to choose from — but the block already names one, and a
// form must never be the only place a stored value lives.
func TestAGalleryKeepsItsAlbumWhenThereIsNoListAtAll(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	albums := album.NewStore(database)
	h.SetAlbumStore(albums)
	// No album is created at all: this is the website whose last album was
	// deleted, and it is also what a failed read used to look like.

	body := blockEditorHTML(t, h, sm, ws.ID, url.Values{
		"b0.typ":   {"galerie"},
		"b0.album": {"sommer-2025"},
	})
	if !strings.Contains(body, `<option value="sommer-2025" selected`) {
		t.Fatalf("with no albums on the website the select was not drawn, so nothing posts b0.album and the next save deletes the gallery:\n%s", body)
	}

	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":   {"galerie"},
		"b0.album": {"sommer-2025"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	serve(t, h, sm, h.HandlePageCreate, req)

	p, err := page.NewStore(database).GetPageBySlug(ctx, ws.ID, "start")
	if err != nil || p == nil {
		t.Fatalf("the page was not created: %v", err)
	}
	blocks, err := block.Decode(p.Blocks, block.Builtin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(blocks) != 1 || blocks[0].AlbumSlug != "sommer-2025" {
		t.Fatalf("the gallery did not survive: %+v", blocks)
	}
}

// TestAGalleryWithNoAlbumStillGetsNoSelect is the negative control.
//
// A fix that simply always drew the select would pass both tests above and
// would undo a decision 11-05 took on purpose: a website with no albums must
// not be offered a choice with nothing in it. TestNoAlbumsMeansNoSelect holds
// the same line; this one holds it for the block that HAS an album store wired,
// which is the case the fix could most easily have widened.
func TestAGalleryWithNoAlbumStillGetsNoSelect(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetAlbumStore(album.NewStore(database))

	body := blockEditorHTML(t, h, sm, ws.ID, url.Values{"b0.typ": {"galerie"}})
	if strings.Contains(body, `name="b0.album"`) {
		t.Errorf("a select with no options was drawn for a gallery that names no album:\n%s", body)
	}
}

// TestAlbumChoicesForLeavesAKnownAlbumAlone is the unit half, because the
// browser-level tests above cannot show that the ordinary case is untouched:
// an unnecessary extra option would be a duplicate in the list of every gallery
// that is working correctly.
func TestAlbumChoicesForLeavesAKnownAlbumAlone(t *testing.T) {
	have := []AlbumChoice{{Slug: "moebel", Name: "Möbel"}, {Slug: "werkzeug", Name: "Werkzeug"}}

	if got := albumChoicesFor(have, ""); len(got) != 2 {
		t.Errorf("a gallery with its own list got %d options, want the 2 albums", len(got))
	}
	if got := albumChoicesFor(have, "moebel"); len(got) != 2 {
		t.Errorf("a gallery naming a known album got %d options, want 2 — the list has grown a duplicate", len(got))
	}
	got := albumChoicesFor(have, "sommer-2025")
	if len(got) != 3 {
		t.Fatalf("a gallery naming an unknown album got %d options, want 3", len(got))
	}
	if got[2].Slug != "sommer-2025" || !got[2].Missing {
		t.Errorf("the appended option is %+v; want the block's own slug, marked missing", got[2])
	}
	if got := albumChoicesFor(nil, "sommer-2025"); len(got) != 1 || !got[0].Missing {
		t.Errorf("with no albums at all the carrier was not made: %+v", got)
	}
}

// TestAFailedAlbumReadIsReportedAndNotCalledAnEmptyList is the other half of
// CR-04, and it is a distinction rather than a crash.
//
// siteAlbums used to log a read failure and return nil, so "I could not ask"
// arrived at the template as "this website has no albums". Two harms follow
// from one lie: the editor is told something false about their own site and may
// create a second album beside the fifty they have, and — before the select
// learned to carry a value it cannot show — {{if .Albums}} was then false and
// the next save deleted every album-backed gallery on the page.
//
// CLAUDE.md's rule for this is one line: handlers return error, and a wrapper
// writes the response. A database that cannot answer is not a website without
// albums.
func TestAFailedAlbumReadIsReportedAndNotCalledAnEmptyList(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetAlbumStore(album.NewStore(database))

	// The table is taken away, which is the cheapest honest way to make the
	// store's read fail: every other failure mode of List reaches this code by
	// the same return.
	if _, err := database.Write.Exec(`DROP TABLE albums`); err != nil {
		t.Fatalf("drop albums: %v", err)
	}

	if _, err := h.siteAlbums(context.Background(), ws.ID); err == nil {
		t.Fatal("a failed album read was reported as a website with no albums")
	}

	// And it reaches the handler, so the operator sees a failure rather than a
	// form that has quietly changed what it posts.
	req := postForm("/admin/websites/1/pages/new", blockForm(ws.ID, url.Values{
		"b0.typ":         {"galerie"},
		"b0.album":       {"sommer-2025"},
		"bausteinaktion": {"neu:trenner"},
	}), map[string]string{"id": strconv.FormatInt(ws.ID, 10)})

	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerErr = h.HandlePageCreate(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr == nil {
		t.Errorf("the block editor drew a form although the album list could not be read:\n%s", rec.Body.String())
	}

	// The negative control: with the table there, the same request is fine.
	h2, sm2, database2, ws2 := newTestAdmin(t)
	h2.SetAlbumStore(album.NewStore(database2))
	if _, err := h2.siteAlbums(context.Background(), ws2.ID); err != nil {
		t.Errorf("an ordinary read reported an error: %v", err)
	}
	_ = sm2
}
