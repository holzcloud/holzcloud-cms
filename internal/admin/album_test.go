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
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// An album screen is reached with two ids that arrive from two different parts
// of one address — the website in "/admin/websites/{id}" and the album in
// "{albumID}" behind it — and this file is where the two are made to agree.
//
// auth.RequireWebsiteAccess reads only the leading one and says so itself, so
// nothing before the handler has an opinion about the second. internal/album's
// store puts website_id in the WHERE clause of every statement, which means the
// handler is the second strap and not the only one; these tests hold both.
//
// The other half of the file is the answer shape every POST owes: a flash, then
// HX-Redirect for an htmx request and a 303 for a browser with scripting
// switched off. That branch is the whole of "htmx is enhancement only", and it
// is invisible until somebody turns scripting off — so it is asserted here.

// albumFixture is two websites that must not be able to reach each other, each
// with an album and a file of its own.
type albumFixture struct {
	h     *Handler
	sm    *scs.SessionManager
	db    *db.DB
	store *album.Store

	siteA, siteB   *domain.Website
	albumA, albumB *album.Album
	mediaA, mediaB int64
}

func newAlbumFixture(t *testing.T) *albumFixture {
	t.Helper()
	ctx := context.Background()

	h, sm, database, siteA := newTestAdmin(t)
	store := album.NewStore(database)
	h.SetAlbumStore(store)

	siteB, err := domain.NewStore(database).CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}

	f := &albumFixture{h: h, sm: sm, db: database, store: store, siteA: siteA, siteB: siteB}
	f.albumA = f.mustCreate(t, siteA.ID, "Werkstatt")
	f.albumB = f.mustCreate(t, siteB.ID, "Geheime Referenzen")
	f.mediaA = f.seedMedia(t, siteA.ID, "a.jpg")
	f.mediaB = f.seedMedia(t, siteB.ID, "b.jpg")
	return f
}

func (f *albumFixture) mustCreate(t *testing.T, websiteID int64, name string) *album.Album {
	t.Helper()
	a, err := f.store.Create(context.Background(), websiteID, name)
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return a
}

func (f *albumFixture) seedMedia(t *testing.T, websiteID int64, filename string) int64 {
	t.Helper()
	res, err := f.db.Write.Exec(
		`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes)
		 VALUES ($1, $2, $2, 'image/jpeg', 2048)`, websiteID, filename)
	if err != nil {
		t.Fatalf("insert media %q: %v", filename, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func (f *albumFixture) mustAdd(t *testing.T, albumID, mediaID int64, alt string) int64 {
	t.Helper()
	id, err := f.store.AddItem(context.Background(), f.siteA.ID, albumID, mediaID, alt, "")
	if err != nil {
		t.Fatalf("AddItem(%q): %v", alt, err)
	}
	return id
}

func (f *albumFixture) pictures(t *testing.T, albumID int64) []album.Picture {
	t.Helper()
	pics, err := f.store.Pictures(context.Background(), f.siteA.ID, albumID)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	return pics
}

// albumFlash runs one handler with a live session and hands back both flashes.
//
// The flash is read inside the same request rather than by a second one because
// a refusal here is a flash plus a redirect: the status alone cannot tell a
// refusal from a success, and the sentence is what an operator acts on.
func albumFlash(t *testing.T, h *Handler, sm *scs.SessionManager,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) (rec *httptest.ResponseRecorder, bad, good string) {
	t.Helper()
	rec = httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerErr = fn(w, r)
		bad = sm.GetString(r.Context(), "flash_error")
		good = sm.GetString(r.Context(), "flash_success")
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, bad, good
}

// albumPost builds the POST the browser sends, with the route values the mux
// would have set. htmx decides whether the request carries the header that
// makes the answer a header instead of a redirect.
func albumPost(target string, values url.Values, pathValues map[string]string, htmx bool) *http.Request {
	req := postForm(target, values, pathValues)
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	return req
}

func id64(v int64) string { return strconv.FormatInt(v, 10) }

// TestAlbumEditFromAnotherWebsiteIs404 is the address with two ids that
// disagree: website A in front, website B's album behind it.
func TestAlbumEditFromAnotherWebsiteIs404(t *testing.T) {
	f := newAlbumFixture(t)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/websites/"+id64(f.siteA.ID)+"/albums/"+id64(f.albumB.ID), nil)
	req.SetPathValue("id", id64(f.siteA.ID))
	req.SetPathValue("albumID", id64(f.albumB.ID))
	rec, _, _ := albumFlash(t, f.h, f.sm, f.h.HandleAlbumEdit, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
	// The status alone is not the property. A 404 that still rendered the
	// album's name would have leaked the one thing GAL-05 is about.
	if body := rec.Body.String(); strings.Contains(body, f.albumB.Name) ||
		strings.Contains(body, f.albumB.Slug) {
		t.Errorf("the body names the other website's album: %q", body)
	}
}

// TestAlbumCreateWithADuplicateNameSaysSo: the store's named error, turned into
// a sentence, and no second row.
func TestAlbumCreateWithADuplicateNameSaysSo(t *testing.T) {
	f := newAlbumFixture(t)

	req := albumPost("/admin/websites/"+id64(f.siteA.ID)+"/albums",
		url.Values{"name": {"Werkstatt"}}, map[string]string{"id": id64(f.siteA.ID)}, false)
	_, bad, good := albumFlash(t, f.h, f.sm, f.h.HandleAlbumCreate, req)

	if bad == "" {
		t.Errorf("a duplicate name produced no error flash (success flash %q)", good)
	}
	albums, err := f.store.List(context.Background(), f.siteA.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(albums) != 1 {
		t.Errorf("albums = %d; want 1 — the duplicate was created", len(albums))
	}
}

// TestAlbumCreateWithABlankNameSaysSo: a name of only spaces is not a name.
func TestAlbumCreateWithABlankNameSaysSo(t *testing.T) {
	f := newAlbumFixture(t)

	req := albumPost("/admin/websites/"+id64(f.siteA.ID)+"/albums",
		url.Values{"name": {"   "}}, map[string]string{"id": id64(f.siteA.ID)}, false)
	_, bad, _ := albumFlash(t, f.h, f.sm, f.h.HandleAlbumCreate, req)

	if bad == "" {
		t.Error("a blank name produced no error flash")
	}
	albums, err := f.store.List(context.Background(), f.siteA.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(albums) != 1 {
		t.Errorf("albums = %d; want 1 — the blank name became a row", len(albums))
	}
}

// TestAlbumPostAnswersHXRedirectForHtmxAnd303Otherwise holds both halves of
// CLAUDE.md's htmx rule in one test, because they are one decision: the same
// button has to work whether or not the script that enhances it ever loaded.
func TestAlbumPostAnswersHXRedirectForHtmxAnd303Otherwise(t *testing.T) {
	f := newAlbumFixture(t)
	target := "/admin/websites/" + id64(f.siteA.ID) + "/albums"
	pathValues := map[string]string{"id": id64(f.siteA.ID)}

	plain, _, _ := albumFlash(t, f.h, f.sm, f.h.HandleAlbumCreate,
		albumPost(target, url.Values{"name": {"Ohne Skript"}}, pathValues, false))
	if plain.Code != http.StatusSeeOther {
		t.Errorf("without HX-Request: status = %d; want 303", plain.Code)
	}
	if got := plain.Header().Get("Location"); got != target {
		t.Errorf("without HX-Request: Location = %q; want %q", got, target)
	}
	if got := plain.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("without HX-Request: HX-Redirect = %q; want none", got)
	}

	htmx, _, _ := albumFlash(t, f.h, f.sm, f.h.HandleAlbumCreate,
		albumPost(target, url.Values{"name": {"Mit Skript"}}, pathValues, true))
	if got := htmx.Header().Get("HX-Redirect"); got != target {
		t.Errorf("with HX-Request: HX-Redirect = %q; want %q", got, target)
	}
	if htmx.Body.Len() != 0 {
		t.Errorf("with HX-Request: body = %q; want none", htmx.Body.String())
	}
}

// TestAlbumItemCreateRefusesAnotherWebsitesMedia: the media id comes out of a
// form, so a number is all it takes to name another library's file.
func TestAlbumItemCreateRefusesAnotherWebsitesMedia(t *testing.T) {
	f := newAlbumFixture(t)

	req := albumPost(
		"/admin/websites/"+id64(f.siteA.ID)+"/albums/"+id64(f.albumA.ID)+"/pictures",
		url.Values{"bild.medium": {id64(f.mediaB)}, "bild.alt": {"Fremd"}},
		map[string]string{"id": id64(f.siteA.ID), "albumID": id64(f.albumA.ID)}, false)
	_, bad, good := albumFlash(t, f.h, f.sm, f.h.HandleAlbumItemCreate, req)

	if bad == "" {
		t.Errorf("a foreign media id produced no error flash (success flash %q)", good)
	}
	if pics := f.pictures(t, f.albumA.ID); len(pics) != 0 {
		t.Errorf("pictures = %d; want 0 — the foreign file was added", len(pics))
	}
}

// TestReorderAtTheEndIsNotAnError: the button may have been drawn against a
// list that has since changed, and the honest answer is the list as it now is —
// block.Apply's stated rule, applied to the album.
func TestReorderAtTheEndIsNotAnError(t *testing.T) {
	f := newAlbumFixture(t)
	first := f.mustAdd(t, f.albumA.ID, f.mediaA, "erstes")
	second := f.mustAdd(t, f.albumA.ID, f.mediaA, "zweites")

	req := albumPost(
		"/admin/websites/"+id64(f.siteA.ID)+"/albums/"+id64(f.albumA.ID)+
			"/pictures/"+id64(first)+"/reorder?direction=up",
		url.Values{}, map[string]string{
			"id": id64(f.siteA.ID), "albumID": id64(f.albumA.ID), "itemID": id64(first),
		}, false)
	rec, bad, _ := albumFlash(t, f.h, f.sm, f.h.HandleAlbumItemReorder, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want 303", rec.Code)
	}
	if bad != "" {
		t.Errorf("moving the first picture up reported an error: %q", bad)
	}
	pics := f.pictures(t, f.albumA.ID)
	if len(pics) != 2 || pics[0].ID != first || pics[1].ID != second {
		t.Errorf("order changed: %+v; want %d then %d", pics, first, second)
	}
}

// TestAlbumDeleteRemovesItsPictures: the cascade, seen from the handler.
func TestAlbumDeleteRemovesItsPictures(t *testing.T) {
	f := newAlbumFixture(t)
	f.mustAdd(t, f.albumA.ID, f.mediaA, "erstes")
	f.mustAdd(t, f.albumA.ID, f.mediaA, "zweites")

	req := albumPost(
		"/admin/websites/"+id64(f.siteA.ID)+"/albums/"+id64(f.albumA.ID)+"/delete",
		url.Values{}, map[string]string{
			"id": id64(f.siteA.ID), "albumID": id64(f.albumA.ID),
		}, false)
	rec, bad, good := albumFlash(t, f.h, f.sm, f.h.HandleAlbumDelete, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d (error flash %q); want 303", rec.Code, bad)
	}
	if good == "" {
		t.Error("the delete left no success flash")
	}
	gone, err := f.store.Get(context.Background(), f.siteA.ID, f.albumA.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gone != nil {
		t.Error("the album is still there")
	}
	var rows int
	if err := f.db.Read.QueryRow(
		`SELECT COUNT(*) FROM album_items WHERE album_id = $1`, f.albumA.ID).Scan(&rows); err != nil {
		t.Fatalf("count album_items: %v", err)
	}
	if rows != 0 {
		t.Errorf("album_items = %d; want 0 — the pictures outlived the album", rows)
	}
}

// TestAlbumScreensAre404WithoutAStore: a build that never wired the store has
// no album screens, the same tolerance h.plugins and h.products already have.
// A panic here would be a 500 on a route nobody configured.
func TestAlbumScreensAre404WithoutAStore(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/"+id64(ws.ID)+"/albums", nil)
	req.SetPathValue("id", id64(ws.ID))
	rec, _, _ := albumFlash(t, h, sm, h.HandleAlbumList, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
}

// TestAlbumScreensRenderInsideTheBaseLayout is the instrument the plan says
// does not exist, and it turns out it does.
//
// A page missing from layoutPageNames (internal/web/render.go) renders through
// RenderAdmin as a bare fragment: no navigation, no flash area, and — the part
// that is a security finding rather than a cosmetic one — no hx-headers
// attribute on the body, so every htmx POST from that screen fails the CSRF
// check. The claim in 11-PATTERNS.md §E.7 is that nothing in go test can see
// this. It can: RenderAdmin writes the whole document, and the document either
// carries <body hx-headers=…> or it does not.
//
// It still does not replace the browser pass. This proves the layout wraps the
// screen; it does not prove the screen is readable.
func TestAlbumScreensRenderInsideTheBaseLayout(t *testing.T) {
	f := newAlbumFixture(t)
	f.mustAdd(t, f.albumA.ID, f.mediaA, "erstes")

	for _, c := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) error
		path string
		vals map[string]string
	}{
		{"album_list", f.h.HandleAlbumList,
			"/admin/websites/" + id64(f.siteA.ID) + "/albums",
			map[string]string{"id": id64(f.siteA.ID)}},
		{"album_edit", f.h.HandleAlbumEdit,
			"/admin/websites/" + id64(f.siteA.ID) + "/albums/" + id64(f.albumA.ID),
			map[string]string{"id": id64(f.siteA.ID), "albumID": id64(f.albumA.ID)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.path, nil)
			for k, v := range c.vals {
				req.SetPathValue(k, v)
			}
			rec, _, _ := albumFlash(t, f.h, f.sm, c.fn, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d; want 200", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "hx-headers") {
				t.Error("no hx-headers on the body: every htmx POST from this screen would fail CSRF")
			}
			if !strings.Contains(body, `class="nav-item`) {
				t.Error("no navigation: the screen rendered outside the base layout")
			}
			if !strings.Contains(body, f.albumA.Name) {
				t.Errorf("the album's name is not on the screen")
			}
		})
	}
}
