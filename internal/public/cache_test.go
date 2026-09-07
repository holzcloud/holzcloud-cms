package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// What a warm browser is told, which is a different question from what the
// renderer produces.
//
// internal/public/album_test.go proves GAL-03 one layer down: change the album,
// and h.pageContent produces different bytes. That is true and it is not the
// requirement. The requirement is about what a visitor SEES, and a visitor who
// has been on the site before does not ask an unconditional question — the
// browser sends back the ETag and the date it was given and asks "still this?".
// Every test in this file asks it that way.

// conditionalGet is the request a browser that has the page already sends: both
// validators, exactly as it was given them.
func conditionalGet(t *testing.T, h *Handler, ws *domain.Website, slug, etag, since string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	req.SetPathValue("slug", slug)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if since != "" {
		req.Header.Set("If-Modified-Since", since)
	}
	rec := httptest.NewRecorder()
	if err := h.HandlePage(rec, req); err != nil {
		t.Fatalf("HandlePage (conditional): %v", err)
	}
	return rec
}

func plainGet(t *testing.T, h *Handler, ws *domain.Website, slug string) *httptest.ResponseRecorder {
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
	return rec
}

// TestAWarmBrowserIsNotKeptOnAnOldAlbum is the requirement, asked the way a
// browser asks it.
//
// The visitor has the page. The editor corrects a caption in the album. The
// page's own row is not touched — that is what GAL-03 buys and it is asserted
// here too, because without it this test could pass for a page that was
// re-saved. The visitor reloads, and the browser sends back both validators.
//
// Before the fix this answered 304: the new ETag did not match, the code fell
// through to the date, pages.updated_at had not moved, and the visitor kept the
// old gallery — with no request they could have made that would recover.
func TestAWarmBrowserIsNotKeptOnAnOldAlbum(t *testing.T) {
	h, database, ws, albums, a := albumFixture(t)
	ctx := context.Background()

	first := plainGet(t, h, ws, "galerie")
	etag := first.Header().Get("ETag")
	since := first.Header().Get("Last-Modified")
	if etag == "" || since == "" {
		t.Fatalf("the first response carried etag=%q last-modified=%q", etag, since)
	}
	if !strings.Contains(first.Body.String(), "hc-galerie__bild") {
		t.Fatal("fixture: the first response is not a gallery")
	}

	htmlBefore, updatedBefore := storedPage(t, database, "galerie")

	pics, err := albums.Pictures(ctx, ws.ID, a.ID)
	if err != nil || len(pics) == 0 {
		t.Fatalf("Pictures: %v, %v", pics, err)
	}
	if err := albums.UpdateItem(ctx, ws.ID, a.ID, pics[0].ID,
		pics[0].Item.MediaID, pics[0].Item.Alt, "Frisch beschriftet"); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	htmlAfter, updatedAfter := storedPage(t, database, "galerie")
	if htmlAfter != htmlBefore || updatedAfter != updatedBefore {
		t.Fatalf("the page's own row moved — this test would then prove nothing:\n before %q %q\n after  %q %q",
			htmlBefore, updatedBefore, htmlAfter, updatedAfter)
	}

	second := conditionalGet(t, h, ws, "galerie", etag, since)
	if second.Code == http.StatusNotModified {
		t.Fatalf("304: the visitor keeps the old gallery although the album changed — GAL-03 is false on the cached path (etag %s, if-modified-since %s)",
			etag, since)
	}
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", second.Code)
	}
	if !strings.Contains(second.Body.String(), "Frisch beschriftet") {
		t.Error("the served page does not carry the new caption")
	}
}

// TestAMismatchingIfNoneMatchEndsTheNegotiation is the RFC 7232 §3.3 half on
// its own, without an album anywhere near it.
//
// "A recipient MUST ignore If-Modified-Since if the request contains an
// If-None-Match header field." The code used to check the ETag, and on a
// mismatch carry on to the date — so the weaker validator could overrule the
// stronger one and answer 304 for a body it had just been told was different.
//
// The date here is deliberately in the future, which is the state every page
// whose content moved without its own row moving is in.
func TestAMismatchingIfNoneMatchEndsTheNegotiation(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	seedPage(t, database, ws.ID, "Über uns", "ueber-uns", "inhalt", "published")

	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)

	rec := conditionalGet(t, h, ws, "ueber-uns", `"nicht-mehr-aktuell"`, future)
	if rec.Code == http.StatusNotModified {
		t.Fatal("304 for a request whose If-None-Match did not match — If-Modified-Since decided a negotiation RFC 7232 §3.3 says it may not take part in")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	// The negative control: the matching ETag must still answer 304, or a fix
	// that simply stopped answering 304 at all would pass the assertion above.
	fresh := plainGet(t, h, ws, "ueber-uns")
	again := conditionalGet(t, h, ws, "ueber-uns", fresh.Header().Get("ETag"), future)
	if again.Code != http.StatusNotModified {
		t.Errorf("a matching ETag answered %d; conditional requests have stopped working altogether", again.Code)
	}
}

// TestChangingAnAlbumMovesThePagesLastModified is the other half of the fix:
// the date validator itself, made honest.
//
// Every browser sends an ETag, so the check above covers them. This one covers
// what does not — a proxy, a CDN, curl -z — by asserting that the album's own
// updated_at reaches the header at all. The album's stamp is driven to a fixed
// moment in the future so that the assertion is about which value was chosen
// and not about how fast the machine is.
func TestChangingAnAlbumMovesThePagesLastModified(t *testing.T) {
	h, database, ws, _, a := albumFixture(t)

	before := plainGet(t, h, ws, "galerie").Header().Get("Last-Modified")

	moved := "2031-04-05T06:07:08Z"
	if _, err := database.Write.Exec(
		`UPDATE albums SET updated_at = $1 WHERE id = $2`, moved, a.ID); err != nil {
		t.Fatalf("move the album's stamp: %v", err)
	}

	after := plainGet(t, h, ws, "galerie").Header().Get("Last-Modified")
	if after == before {
		t.Fatalf("Last-Modified stayed at %q although the album changed — the validator does not see the albums, and a cache that sends only a date keeps the old gallery",
			before)
	}
	want, err := time.Parse("2006-01-02T15:04:05Z", moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if after != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the album's own stamp %q", after, want.UTC().Format(http.TimeFormat))
	}
}
