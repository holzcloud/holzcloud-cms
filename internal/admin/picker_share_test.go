package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
)

// Two small screens that exist because of what an editor would otherwise do:
// the picker, because inserting an image meant navigating away and losing
// everything typed; the preview link, because the alternative people reach for
// is publishing a draft "just for a minute" so a customer can look at it, which
// is how a half-finished price list ends up in a search index for a year.
//
// Driven red by ten mutations, all ten caught. Two had to be chased and both
// are the recurring shape:
//
//   - The foreign-page case paired a foreign page with a foreign PICTURE, so
//     the picture's guard caught it and the page's could be deleted with
//     nothing noticing. It uses a picture of this website now.
//   - Asking for ten thousand days and checking only the status would pass with
//     the lifetime bound deleted. The token is now handed to the signer at a
//     moment beyond the bound, which is the thing the bound exists for.

func TestThePickerOffersThisWebsitesFilesAndFilters(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
	if _, err := media.NewStore(database).Create(ctx, ws.ID, "abc.pdf", "preise.pdf",
		"application/pdf", 10, "hash-pdf"); err != nil {
		t.Fatal(err)
	}
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := media.NewStore(database).Create(ctx, other.ID, "fremd.png", "fremd.png",
		"image/png", 10, "hash-fremd"); err != nil {
		t.Fatal(err)
	}

	body := func(query string) string {
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/medienwahl"+query, nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		rec := serve(t, h, sm, h.HandleMediaPicker, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", query, rec.Code)
		}
		return rec.Body.String()
	}

	all := body("")
	if !strings.Contains(all, "werkstatt.png") || !strings.Contains(all, "preise.pdf") {
		t.Error("the picker does not offer this website's files")
	}
	if strings.Contains(all, "fremd.png") {
		t.Error("the picker offers another website's file")
	}

	// The same filters the media list has, because the picker is that list in a
	// fragment.
	images := body("?kind=image")
	if !strings.Contains(images, "werkstatt.png") || strings.Contains(images, "preise.pdf") {
		t.Error("the picture filter did not narrow the picker")
	}

	// A fragment and not a page: it is swapped into the editor, so a whole
	// document would put a second <html> inside the one already open.
	if strings.Contains(all, "<html") || strings.Contains(all, "<!DOCTYPE") {
		t.Error("the picker answers with a whole page rather than a fragment")
	}
}

// Inserting APPENDS, which is a real limitation and a deliberate one: placing
// text at the caret needs script, and the only way to wire that from a template
// would be an inline handler — which means loosening script-src away from
// 'self'. What must hold is that nothing already typed is lost.
func TestInsertingAPictureAppendsAndKeepsWhatWasThere(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "werkstatt.png", onePixelPNG))
	items, _, _ := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	if len(items) != 1 {
		t.Fatalf("%d files", len(items))
	}
	m := items[0]

	const typed = "Ein langer Absatz, den niemand zweimal schreiben möchte."
	p := seedPage(t, database, ws.ID, "Über uns", "ueber-uns", typed, "draft")

	route := websiteRoute(ws.ID, "pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageInsertMedia, postForm("/admin/websites/1/pages/1/bild",
		url.Values{"media_id": {strconv.FormatInt(m.ID, 10)}}, route))
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	after, err := h.pages.GetPage(ctx, p.ID)
	if err != nil || after == nil {
		t.Fatal(err)
	}
	if !strings.Contains(after.ContentMarkdown, typed) {
		t.Fatalf("what was typed is gone: %q", after.ContentMarkdown)
	}
	if !strings.Contains(after.ContentMarkdown, m.Filename) {
		t.Errorf("the picture was not inserted: %q", after.ContentMarkdown)
	}
	// Saved through the normal path, so the version moved and the HTML was
	// re-rendered rather than patched.
	if after.Version <= p.Version {
		t.Errorf("the version did not move: %d then %d", p.Version, after.Version)
	}
	if !strings.Contains(after.ContentHTML, "<img") {
		t.Errorf("the HTML was not re-rendered: %q", after.ContentHTML)
	}
}

// Neither the page nor the picture may come from another website, and a
// mismatch is a 404 that changes nothing.
func TestInsertingRefusesWhatIsNotThisWebsites(t *testing.T) {
	h, sm, database, ws := mediaAdmin(t)
	ctx := context.Background()
	other, err := domain.NewStore(database).CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, other.ID, "fremd.png", onePixelPNG))
	fremdMedia, _, _ := media.NewStore(database).List(ctx, other.ID, media.Filter{}, 1, 10)
	if len(fremdMedia) != 1 {
		t.Fatalf("%d files on the other website", len(fremdMedia))
	}
	mine := seedPage(t, database, ws.ID, "Meins", "meins", "unberührt", "draft")
	fremdPage := seedPage(t, database, other.ID, "Fremd", "fremd", "auch unberührt", "draft")

	// A picture of THIS website, so the foreign-page case is refused by the
	// page's guard and not by the picture's. The first draft paired a foreign
	// page with a foreign picture, and the picture's guard caught it — so the
	// page's could be deleted with nothing noticing.
	serve(t, h, sm, h.HandleMediaUpload, mediaUploadRequest(t, ws.ID, "meins.png", onePixelPNG))
	meins, _, _ := media.NewStore(database).List(ctx, ws.ID, media.Filter{}, 1, 10)
	if len(meins) != 1 {
		t.Fatalf("%d files on this website", len(meins))
	}

	for _, c := range []struct {
		what            string
		pageID, mediaID int64
	}{
		{"another website's picture", mine.ID, fremdMedia[0].ID},
		{"another website's page, with a picture of this one", fremdPage.ID, meins[0].ID},
		{"a picture that does not exist", mine.ID, 999},
	} {
		rec := serve(t, h, sm, h.HandlePageInsertMedia,
			postForm("/admin/websites/1/pages/1/bild",
				url.Values{"media_id": {strconv.FormatInt(c.mediaID, 10)}},
				websiteRoute(ws.ID, "pageID", strconv.FormatInt(c.pageID, 10))))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", c.what, rec.Code)
		}
	}

	for _, p := range []struct {
		id   int64
		want string
	}{{mine.ID, "unberührt"}, {fremdPage.ID, "auch unberührt"}} {
		after, _ := h.pages.GetPage(ctx, p.id)
		if after == nil || after.ContentMarkdown != p.want {
			t.Errorf("page %d was changed: %q", p.id, after.ContentMarkdown)
		}
	}
}

// A preview link is absolute, because it is pasted into an email, and it
// expires — bounded above whatever the form asks for.
func TestAPreviewLinkIsAbsoluteAndExpires(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	p := seedPage(t, database, ws.ID, "Preisliste", "preisliste", "Noch nicht fertig.", "draft")
	route := websiteRoute(ws.ID, "pageID", strconv.FormatInt(p.ID, 10))

	rec := serve(t, h, sm, h.HandlePageShare,
		postForm("/admin/websites/1/pages/1/teilen", url.Values{}, route))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "http://") && !strings.Contains(body, "https://") {
		t.Error("the link is not absolute, so pasting it into an email gives nothing")
	}
	if !strings.Contains(body, sharelink.Path("")) {
		t.Errorf("the screen does not show a preview link:\n%.300s", body)
	}

	// A lifetime longer than the bound is CUT to it rather than refused, and
	// the cut is what the test is about: the point of the bound is that a link
	// somebody forgets about stops working. Asking for ten thousand days and
	// checking only the status would pass with the bound deleted.
	for _, days := range []string{"1", "9999", "0", "keine-zahl", ""} {
		rec := serve(t, h, sm, h.HandlePageShare, postForm("/admin/websites/1/pages/1/teilen",
			url.Values{"tage": {days}}, route))
		if rec.Code != http.StatusOK {
			t.Errorf("tage=%q: status %d", days, rec.Code)
			continue
		}
		asked := tokenFrom(t, rec.Body.String())
		beyond := time.Now().UTC().Add(sharelink.MaxLifetime + 48*time.Hour)
		if _, err := h.share.Check(asked, beyond); err == nil {
			t.Errorf("tage=%q produced a link that still works beyond the longest "+
				"lifetime this screen offers", days)
		}
	}

	// The token the screen hands out is one the signer accepts, and it stops
	// working after its moment — which is the whole reason this screen exists
	// instead of publishing a draft "just for a minute".
	token := tokenFrom(t, body)
	if _, err := h.share.Check(token, time.Now().UTC()); err != nil {
		t.Errorf("the link the screen shows is not one the signer accepts: %v", err)
	}
	if _, err := h.share.Check(token, time.Now().UTC().Add(sharelink.MaxLifetime+48*time.Hour)); err == nil {
		t.Error("the link still works long after the longest lifetime this screen offers")
	}

	// Another website's page is a 404.
	other, err := domain.NewStore(database).CreateWebsite(context.Background(), "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	fremd := seedPage(t, database, other.ID, "Fremd", "fremd", "x", "draft")
	rec = serve(t, h, sm, h.HandlePageShare, postForm("/admin/websites/1/pages/1/teilen",
		url.Values{}, websiteRoute(ws.ID, "pageID", strconv.FormatInt(fremd.ID, 10))))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another website's page: status %d, want 404", rec.Code)
	}
}

// tokenFrom pulls the preview token out of the rendered screen, the way
// somebody copies the link out of it.
func tokenFrom(t *testing.T, body string) string {
	t.Helper()
	prefix := sharelink.Path("")
	i := strings.Index(body, prefix)
	if i < 0 {
		t.Fatalf("no preview link in the screen")
	}
	rest := body[i+len(prefix):]
	end := strings.IndexAny(rest, `"'< `)
	if end < 0 {
		t.Fatal("the link does not end")
	}
	return rest[:end]
}
