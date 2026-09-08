package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The eight album handlers that take a second id, driven as an editor who may
// enter website A against website B's album.
//
// This file exists because of what .planning/debug/knowledge-base.md calls the
// recurring defect shape of this codebase: an admin route takes a website id
// and a second resource id, auth.RequireWebsiteAccess authorises only the
// first — it says so in its own doc comment — and the second is written without
// being scoped. It has shipped six times: the menu items, the products, the
// order retry mail, the page translation group, the snippets and product_media.
//
// internal/album is genuinely well scoped and the review that asked for this
// file could construct no cross-website access through any of the nine routes:
// website_id stands in the WHERE clause of all sixteen statements, the four
// item statements reach it through a subquery on albums, and every signature
// takes websiteID immediately after the context. So this is a guard, not a
// report of a live defect.
//
// It is written all the same, and the knowledge base says why: internal/menu's
// store "looked reviewed" until four handlers were found reaching another
// website's navigation, and the three existing *_scope_test.go files exist
// because signatures alone were not believed. The next resource with a second
// id will be written by somebody reading these tests.
//
// The shape is the one the knowledge base prescribes, in full: two websites, a
// resource on B, an editor whose user_websites grants only A, the request driven
// through the REAL middleware chain, and the EFFECT asserted as well as the
// status — because a refusal that still moved a picture is no refusal — with
// the negative controls, or a fix that refuses everything would pass.

// albumScopeFixture is the album fixture plus the person under test.
type albumScopeFixture struct {
	*albumFixture

	// The two pictures of website B's album, which is what the item handlers
	// are aimed at.
	itemB1, itemB2 int64
	editorOnlyOnA  int64
}

func newAlbumScopeFixture(t *testing.T) *albumScopeFixture {
	t.Helper()
	ctx := context.Background()

	f := &albumScopeFixture{albumFixture: newAlbumFixture(t)}

	// Website B's album gets two pictures, so a reorder has something to move
	// and a delete has something to take.
	f.itemB1 = f.addToB(t, f.mediaB, "B eins")
	f.itemB2 = f.addToB(t, f.seedMedia(t, f.siteB.ID, "b2.jpg"), "B zwei")

	users := user.NewStore(f.db, cheapHashing)
	editorID, err := users.Create(ctx, "Editor A", "editor-a@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	// An assignment must exist: no assignment at all means every website, see
	// NewWebsiteAccessLookup.
	if err := users.SetRights(ctx, editorID, user.Rights{MayPublish: true, Websites: []int64{f.siteA.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	f.editorOnlyOnA = editorID

	// The premise of the whole attack. Without this the tests below could be
	// measuring a guard that refuses everybody.
	allowed := NewWebsiteAccessLookup(f.db)
	if !allowed(ctx, editorID, f.siteA.ID) {
		t.Fatal("the editor should be allowed on website A")
	}
	if allowed(ctx, editorID, f.siteB.ID) {
		t.Fatal("the editor should not be allowed on website B")
	}
	return f
}

func (f *albumScopeFixture) addToB(t *testing.T, mediaID int64, alt string) int64 {
	t.Helper()
	id, err := f.store.AddItem(context.Background(), f.siteB.ID, f.albumB.ID, mediaID, alt, "Unterschrift B")
	if err != nil {
		t.Fatalf("AddItem on B (%q): %v", alt, err)
	}
	return id
}

// drive runs one request through the same chain cmd/holzcloud/main.go builds:
// the session, then RequireWebsiteAccess, then the routed mux with the nine
// album routes registered exactly as main.go registers them.
//
// Going through the real middleware is the point. A handler-only test — which
// is what TestAlbumEditFromAnotherWebsiteIs404 is — cannot show that the guard
// lets the request past, and "the guard lets it past" is the premise of the
// whole defect family.
func (f *albumScopeFixture) drive(t *testing.T, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/websites/{id}/albums", f.h.ErrHandler(f.h.HandleAlbumList))
	mux.HandleFunc("POST /admin/websites/{id}/albums", f.h.ErrHandler(f.h.HandleAlbumCreate))
	mux.HandleFunc("GET /admin/websites/{id}/albums/{albumID}", f.h.ErrHandler(f.h.HandleAlbumEdit))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/update", f.h.ErrHandler(f.h.HandleAlbumUpdate))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/delete", f.h.ErrHandler(f.h.HandleAlbumDelete))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures", f.h.ErrHandler(f.h.HandleAlbumItemCreate))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/update", f.h.ErrHandler(f.h.HandleAlbumItemUpdate))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/delete", f.h.ErrHandler(f.h.HandleAlbumItemDelete))
	mux.HandleFunc("POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/reorder", f.h.ErrHandler(f.h.HandleAlbumItemReorder))

	guarded := auth.RequireWebsiteAccess(f.sm, NewWebsiteAccessLookup(f.db))(mux)

	req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	rec := httptest.NewRecorder()
	f.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.sm.Put(r.Context(), auth.SessionKeyUserID, f.editorOnlyOnA)
		guarded.ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

// albumOnB re-reads website B's album straight from the store, on B's own
// website id, which is the only way to see it at all.
func (f *albumScopeFixture) albumOnB(t *testing.T) *album.Album {
	t.Helper()
	a, err := f.store.Get(context.Background(), f.siteB.ID, f.albumB.ID)
	if err != nil {
		t.Fatalf("Get album B: %v", err)
	}
	return a
}

func (f *albumScopeFixture) picturesOnB(t *testing.T) []album.Picture {
	t.Helper()
	pics, err := f.store.Pictures(context.Background(), f.siteB.ID, f.albumB.ID)
	if err != nil {
		t.Fatalf("Pictures on B: %v", err)
	}
	return pics
}

// unchangedB is the effect assertion every refusal below owes. The status is
// half the property; this is the half that would still have caught the six
// shipped defects, because every one of them answered a plausible status while
// writing the foreign row.
func (f *albumScopeFixture) unchangedB(t *testing.T, what string) {
	t.Helper()

	a := f.albumOnB(t)
	if a == nil {
		t.Fatalf("%s: website B's album is gone", what)
	}
	if a.Name != f.albumB.Name || a.Slug != f.albumB.Slug {
		t.Errorf("%s: website B's album became %q/%q, was %q/%q",
			what, a.Name, a.Slug, f.albumB.Name, f.albumB.Slug)
	}

	pics := f.picturesOnB(t)
	if len(pics) != 2 {
		t.Fatalf("%s: website B's album has %d pictures, had 2", what, len(pics))
	}
	if pics[0].ID != f.itemB1 || pics[1].ID != f.itemB2 {
		t.Errorf("%s: website B's pictures are now %d, %d — the order was rewritten",
			what, pics[0].ID, pics[1].ID)
	}
	if pics[0].Item.Alt != "B eins" || pics[1].Item.Alt != "B zwei" {
		t.Errorf("%s: website B's descriptions are now %q, %q",
			what, pics[0].Item.Alt, pics[1].Item.Alt)
	}
	for _, p := range pics {
		if p.Item.Caption != "Unterschrift B" {
			t.Errorf("%s: a caption on website B became %q", what, p.Item.Caption)
		}
	}
}

// nameForm is what the intruder would type into the rename field.
func nameForm() url.Values { return url.Values{"name": {"Übernommen"}} }

// pictureForm is what block_bild posts, aimed at website A's own file — which
// is the interesting direction: the attacker uses a picture they are entitled
// to and an album they are not.
func (f *albumScopeFixture) pictureForm() url.Values {
	return url.Values{
		"bild.medium":           {id64(f.mediaA)},
		"bild.alt":              {"Übernommen"},
		"bild.bildunterschrift": {"Übernommen"},
	}
}

// The seven refusals, one per handler that takes an album id. Each drives
// website A in the address and website B's album behind it.

func TestAlbumEditThroughTheRealChainRefusesAForeignAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodGet,
		fmt.Sprintf("/admin/websites/%d/albums/%d", f.siteA.ID, f.albumB.ID), nil)
	refused(t, rec, "edit")

	// The screen is also a read, and GAL-05 is about what may be seen. Named
	// rather than dumped: the admin layout is four hundred lines and printing it
	// buries the one word that matters.
	for _, secret := range []string{f.albumB.Name, f.albumB.Slug, "B eins", "B zwei"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the refusal put %q on the screen — website B's album is readable through website A's address", secret)
		}
	}
	f.unchangedB(t, "edit")
}

func TestAlbumUpdateRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/update", f.siteA.ID, f.albumB.ID), nameForm())
	refused(t, rec, "update")
	f.unchangedB(t, "update")
}

func TestAlbumDeleteRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/delete", f.siteA.ID, f.albumB.ID), nil)
	refused(t, rec, "delete")
	f.unchangedB(t, "delete")
}

func TestAlbumItemCreateRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures", f.siteA.ID, f.albumB.ID), f.pictureForm())
	refused(t, rec, "item create")
	f.unchangedB(t, "item create")
}

func TestAlbumItemUpdateRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d/update", f.siteA.ID, f.albumB.ID, f.itemB1),
		f.pictureForm())
	refused(t, rec, "item update")
	f.unchangedB(t, "item update")
}

func TestAlbumItemDeleteRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d/delete", f.siteA.ID, f.albumB.ID, f.itemB1), nil)
	refused(t, rec, "item delete")
	f.unchangedB(t, "item delete")
}

func TestAlbumItemReorderRefusesAForeignWebsitesAlbum(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d/reorder?direction=down",
			f.siteA.ID, f.albumB.ID, f.itemB1), nil)
	refused(t, rec, "item reorder")
	f.unchangedB(t, "item reorder")
}

// TestAlbumCreateOnAForeignWebsiteIsRefused is the eighth handler. It takes no
// album id, so the only id it can disagree about is the website's own — which
// is the middleware's job, and this asserts the middleware is actually in the
// chain rather than trusting that it is.
func TestAlbumCreateOnAForeignWebsiteIsRefused(t *testing.T) {
	f := newAlbumScopeFixture(t)

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums", f.siteB.ID), url.Values{"name": {"Eingeschmuggelt"}})
	refused(t, rec, "create on B")

	list, err := f.store.List(context.Background(), f.siteB.ID)
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("website B has %d albums, had 1 — one was created from website A's session", len(list))
	}
}

// The negative controls. Without them a fix that refused every album write
// would pass every test above, and the album area would be dead while the suite
// stayed green.

func TestAlbumWritesStillWorkOnTheOwnWebsite(t *testing.T) {
	f := newAlbumScopeFixture(t)
	ctx := context.Background()

	// Create.
	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums", f.siteA.ID), url.Values{"name": {"Neues Album"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create on the own website: status %d, want 303", rec.Code)
	}
	list, err := f.store.List(ctx, f.siteA.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("website A has %v albums after the create: %v", len(list), err)
	}

	// Rename.
	rec = f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/update", f.siteA.ID, f.albumA.ID), nameForm())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("rename on the own website: status %d, want 303", rec.Code)
	}
	if a, _ := f.store.Get(ctx, f.siteA.ID, f.albumA.ID); a == nil || a.Name != "Übernommen" {
		t.Errorf("the rename on the own website did not take: %+v", a)
	}

	// Add a picture.
	rec = f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures", f.siteA.ID, f.albumA.ID), f.pictureForm())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add a picture on the own website: status %d, want 303", rec.Code)
	}
	pics, err := f.store.Pictures(ctx, f.siteA.ID, f.albumA.ID)
	if err != nil || len(pics) != 1 {
		t.Fatalf("website A's album has %d pictures after the add: %v", len(pics), err)
	}

	// Delete the picture again.
	rec = f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d/delete", f.siteA.ID, f.albumA.ID, pics[0].ID), nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete a picture on the own website: status %d, want 303", rec.Code)
	}
	if pics, _ = f.store.Pictures(ctx, f.siteA.ID, f.albumA.ID); len(pics) != 0 {
		t.Errorf("the delete on the own website did not take: %d pictures left", len(pics))
	}

	// Delete the album.
	rec = f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/delete", f.siteA.ID, f.albumA.ID), nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete on the own website: status %d, want 303", rec.Code)
	}
	if a, _ := f.store.Get(ctx, f.siteA.ID, f.albumA.ID); a != nil {
		t.Errorf("the delete on the own website did not take: %+v", a)
	}
}

// TestAlbumReorderStillWorksOnTheOwnWebsite is its own control because the
// reorder is the one write that changes nothing an operator can name — it
// swaps two numbers — and a refusal there looks exactly like a success.
func TestAlbumReorderStillWorksOnTheOwnWebsite(t *testing.T) {
	f := newAlbumScopeFixture(t)
	ctx := context.Background()

	first := f.mustAdd(t, f.albumA.ID, f.mediaA, "A eins")
	second := f.mustAdd(t, f.albumA.ID, f.seedMedia(t, f.siteA.ID, "a2.jpg"), "A zwei")

	rec := f.drive(t, http.MethodPost,
		fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d/reorder?direction=down",
			f.siteA.ID, f.albumA.ID, first), nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reorder on the own website: status %d, want 303", rec.Code)
	}

	pics, err := f.store.Pictures(ctx, f.siteA.ID, f.albumA.ID)
	if err != nil || len(pics) != 2 {
		t.Fatalf("Pictures: %v, %v", pics, err)
	}
	if pics[0].ID != second || pics[1].ID != first {
		t.Errorf("the reorder on the own website did not take: %d, %d", pics[0].ID, pics[1].ID)
	}
}
