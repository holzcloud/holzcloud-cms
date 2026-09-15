package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// plantWords writes a stored body straight into the column, because that is
// where block.Render puts it: a page built from blocks is rendered once on save
// and the markers survive into content_html.
func plantWords(t *testing.T, database *db.DB, slug, body string) {
	t.Helper()
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE pages SET content_html = $1 WHERE slug = $2`, body, slug); err != nil {
		t.Fatalf("plant the markers in %q: %v", slug, err)
	}
}

// storedGallery is what a saved gallery block looks like from v2.3 on: the
// words this program mints stand as markers, in text and in an attribute alike.
func storedGallery() string {
	return `<div class="hc-block hc-galerie hc-spalten-3 hc-galerie--diashow" role="region" aria-label="` +
		block.Word("gallery") + `">` +
		`<nav><a href="#hc-b0-p2">` + block.Word("next") + `</a>` +
		`<a href="#hc-close">` + block.Word("close") + `</a></nav></div>`
}

// WORD-01 and WORD-02 in one assertion: a page answers in ITS OWN language.
//
// This is window 34 and it is the third time this project has had to answer the
// same question. v2.0 asked the operator's language on a public route and
// answered German visitors in English (PUB-01). v2.1 fixed that for a plugin's
// own text. v2.2 made an album gallery's two halves agree with each other — and
// both of them answered in the WEBSITE's language, which on a French page is the
// wrong one.
//
// The website's main language is German here and the page is French. If the
// words came from the website, every assertion below would still find "Galerie"
// — which is why the control asserted on is one whose German and French differ.
func TestAPageAnswersInItsOwnLanguageAndNotTheWebsites(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")

	de := seedPageIn(t, database, ws.ID, "Galerie", "galerie", "Bilder", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Galerie", "galerie-fr", "Images", "published", "fr", de.ID)
	plantWords(t, database, "galerie", storedGallery())
	plantWords(t, database, "galerie-fr", storedGallery())

	for _, tc := range []struct {
		name, target, slug, want, wrong string
	}{
		{"the main language", "/galerie", "galerie", "Nächstes Bild", "Image suivante"},
		{"a second language", "/fr/galerie-fr", "galerie-fr", "Image suivante", "Nächstes Bild"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := throughMiddleware(h.HandlePage, ws, tc.target, tc.slug)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("%q is missing; the page did not answer in its own language:\n%.600s", tc.want, body)
			}
			if strings.Contains(body, tc.wrong) {
				t.Errorf("%q reached a page published in another language — the words "+
					"are coming from somewhere that is not the page:\n%.600s", tc.wrong, body)
			}
		})
	}
}

// WORD-03: a page written before this milestone renders exactly as it did.
//
// Its body carries the words themselves, frozen at save, and no marker. The
// guard skips it, nothing is rewritten, and what a visitor sees is what was
// stored — wrong language and all, until somebody saves the page again. That is
// window 8's rule and the reason the album marker's new fields were optional:
// no migration, no broken page, no re-save required to render.
func TestAPageStoredBeforeTheMarkersIsServedUnchanged(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")

	de := seedPageIn(t, database, ws.ID, "Alt", "alt", "Bilder", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Alt", "alt-fr", "Images", "published", "fr", de.ID)
	// Exactly what a v2.2 save wrote: the German word, in the markup.
	old := `<div class="hc-block hc-galerie" role="region" aria-label="Galerie">` +
		`<nav><a href="#hc-b0-p2">Nächstes Bild</a></nav></div>`
	plantWords(t, database, "alt-fr", old)

	rec := throughMiddleware(h.HandlePage, ws, "/fr/alt-fr", "alt-fr")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Nächstes Bild") {
		t.Errorf("an old page lost its frozen word; the resolution must leave a "+
			"marker-free body alone:\n%.600s", body)
	}
}

// Nothing this program mints may reach a reader as syntax, on any public route.
//
// The same rule internal/album/expand.go states — "a visitor must not see the
// internal syntax on a live page" — applied to the third marker family. The
// feed is on the list because it was the route the album marker shipped
// verbatim into for a whole phase, and a plugin is on it because a plugin is a
// reader too.
func TestNoPublicRouteLeaksAWordMarker(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Testseite")
	seedPage(t, database, ws.ID, "Galerie", "galerie", "Bilder", "published")
	plantWords(t, database, "galerie", storedGallery())

	t.Run("the page", func(t *testing.T) {
		rec := throughMiddleware(h.HandlePage, ws, "/galerie", "galerie")
		mustNotLeak(t, rec.Body.String())
	})

	t.Run("the feed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "http://demo.test/feed.xml", nil)
		req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
		if err := h.HandleFeed(rec, req); err != nil {
			t.Fatalf("HandleFeed: %v", err)
		}
		mustNotLeak(t, rec.Body.String())
	})

	t.Run("a plugin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://demo.test/galerie", nil)
		req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
		got, err := h.PagesForPlugin(withRequest(req.Context(), req), ws.ID, plugin.PagesQuery{
			Op: plugin.OpPagesGet, Slug: "galerie", Limit: 10,
		})
		if err != nil {
			t.Fatalf("pages.get: %v", err)
		}
		if len(got.Pages) != 1 {
			t.Fatalf("pages.get returned %d pages, want 1", len(got.Pages))
		}
		mustNotLeak(t, got.Pages[0].HTML)
	})
}

func mustNotLeak(t *testing.T, body string) {
	t.Helper()
	if block.HasWordMarker(body) {
		t.Errorf("a word marker reached a reader; this program's internal syntax "+
			"is not text:\n%.600s", body)
	}
}
