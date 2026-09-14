package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// What a SHARED cache is allowed to keep.
//
// internal/public/access.go says it, in the comment above serveGate: a proxy
// holding this would hand "the page to someone who has not" unlocked. serveGate
// answers `no-store, private` and is right. The page BEHIND the gate went out
// through serveCached, which answers `public, max-age=300` and sets
// Vary: HX-Request — with Set, so it drops the Vary: Cookie that the session
// middleware added. Cacheable, shared, and keyed on a name that does not
// include the cookie the access depends on.
func TestAnUnlockedPageIsNotHandedToAProxy(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Demo")
	seedPage(t, database, ws.ID, "Preise", "preise", "# Preisliste\n\nNetto 12.—", "published")

	store := page.NewStore(database)
	pg := pageBySlug(t, store, ws.ID, "preise")
	if err := store.SetAccess(context.Background(), pg.ID,
		page.AccessUpdate{Protected: true, Password: "geheim", Hint: "Bitte fragen"},
		auth.DefaultParams); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}
	h.SetShareSigners(sharelink.New([]byte("share-secret-share-secret-share!")),
		sharelink.New([]byte("unlock-secret-unlock-secret-unl!")))

	pg = pageBySlug(t, store, ws.ID, "preise")
	if !pg.Protected() {
		t.Fatal("the page is not protected; the test proves nothing")
	}

	// --- locked: the gate, and it already gets this right ---
	rec := protectedGet(t, h, ws, "preise", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked status = %d, want 401", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("the gate is cacheable: Cache-Control = %q", cc)
	}

	// --- unlocked: the page itself ---
	cookie := &http.Cookie{
		Name:  unlockCookieName(pg.ID),
		Value: h.unlock.Token(pg.ID, time.Now().Add(time.Hour)),
	}
	rec = protectedGet(t, h, ws, "preise", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("unlocked status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Preisliste") {
		t.Fatal("the unlocked page did not render its content; the test proves nothing")
	}

	cc := rec.Header().Get("Cache-Control")
	if strings.Contains(cc, "public") || !strings.Contains(cc, "no-store") {
		t.Errorf("a protected page is offered to shared caches: Cache-Control = %q", cc)
	}
	if vary := strings.Join(rec.Header().Values("Vary"), ", "); !strings.Contains(vary, "Cookie") {
		t.Errorf("the answer depends on a cookie and does not say so: Vary = %q", vary)
	}
}

// The same question for the shop. With both price modes on offer, which prices
// a visitor sees depends on a cookie — and the page went out as
// `public, max-age=300` with the Vary overwritten, so a shared cache could hand
// business prices to a private customer for five minutes.
//
// With ONE price mode the figures are the same for everybody and the page stays
// shareable. That half is asserted too: the fix must not turn the whole
// catalogue into an answer nobody may cache.
func TestShopPricesThatDependOnACookieAreNotShared(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Demo")

	for _, c := range []struct {
		display    string
		wantShared bool
	}{
		{shop.DisplayPrivate, true},
		{shop.DisplayBusiness, true},
		{shop.DisplayBoth, false},
	} {
		t.Run(c.display, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/shop", nil)
			req.Host = "demo.test"
			req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
			// What the session middleware writes on every request, and what
			// the old Set threw away.
			rec.Header().Add("Vary", "Cookie")

			h.servePricedFor(rec, req, shop.Settings{Display: c.display}, []byte("<html>preis</html>"))

			cc := rec.Header().Get("Cache-Control")
			vary := strings.Join(rec.Header().Values("Vary"), ", ")
			if c.wantShared {
				if !strings.Contains(cc, "public") {
					t.Errorf("one price mode is the same for everybody; Cache-Control = %q", cc)
				}
				return
			}
			if strings.Contains(cc, "public") {
				t.Errorf("prices that follow a cookie are offered to shared caches: Cache-Control = %q", cc)
			}
			if !strings.Contains(vary, "Cookie") {
				t.Errorf("the answer follows a cookie and does not say so: Vary = %q", vary)
			}
		})
	}
}

func pageBySlug(t *testing.T, s *page.Store, websiteID int64, slug string) *page.Page {
	t.Helper()
	pg, err := s.GetPageBySlug(context.Background(), websiteID, slug)
	if err != nil {
		t.Fatalf("GetBySlug %q: %v", slug, err)
	}
	return pg
}

func protectedGet(t *testing.T, h *Handler, ws *domain.Website, slug string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	req.SetPathValue("slug", slug)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	if err := h.HandlePage(rec, req); err != nil {
		t.Fatalf("HandlePage: %v", err)
	}
	return rec
}
