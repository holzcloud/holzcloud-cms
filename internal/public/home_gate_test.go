package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
)

func protectedHome(t *testing.T) (*Handler, *domain.Website, *page.Page) {
	t.Helper()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Demo")
	seedPage(t, database, ws.ID, "Start", "home", "# Geheimer Inhalt\n\nNur fuer Kunden", "published")

	store := page.NewStore(database)
	pg := pageBySlug(t, store, ws.ID, "home")
	if err := store.SetAccess(context.Background(), pg.ID,
		page.AccessUpdate{Protected: true, Password: "geheim"}, auth.DefaultParams); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}
	h.SetShareSigners(sharelink.New([]byte("share-secret-share-secret-share!")),
		sharelink.New([]byte("unlock-secret-unlock-secret-unl!")))
	return h, ws, pageBySlug(t, store, ws.ID, "home")
}

func getHome(t *testing.T, h *Handler, ws *domain.Website, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	if err := h.HandleHome(rec, req); err != nil {
		t.Fatalf("HandleHome: %v", err)
	}
	return rec
}

// A protected home page is a protected page: no content without the password.
func TestProtectedHomePageAsksForThePassword(t *testing.T) {
	h, ws, pg := protectedHome(t)
	if !pg.Protected() {
		t.Fatal("the page is not protected; the test proves nothing")
	}

	rec := getHome(t, h, ws, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked status = %d, want 401", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Geheimer Inhalt") {
		t.Error("the locked home page leaked its content")
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("the gate is cacheable: %q", cc)
	}

	cookie := &http.Cookie{Name: unlockCookieName(pg.ID),
		Value: h.unlock.Token(pg.ID, time.Now().Add(time.Hour))}
	rec = getHome(t, h, ws, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Geheimer Inhalt") {
		t.Fatalf("unlocked: status %d, content missing", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "public") {
		t.Errorf("the unlocked home page is offered to shared caches: %q", cc)
	}
}

// The password typed into the gate must lead back to the page: the cookie has
// to be sent to "/", not only to "/home".
func TestUnlockingTheHomePageSticks(t *testing.T) {
	h, ws, _ := protectedHome(t)

	form := url.Values{"seite": {"/home"}, "passwort": {"geheim"}}
	req := httptest.NewRequest(http.MethodPost, "/freischalten", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	if err := h.HandleUnlock(rec, req); err != nil {
		t.Fatalf("HandleUnlock: %v", err)
	}
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("unlock answered %d to %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/" {
		t.Fatalf("cookies = %+v, want one scoped to /", cookies)
	}

	if rec := getHome(t, h, ws, cookies[0]); rec.Code != http.StatusOK {
		t.Errorf("home after unlock = %d, want 200", rec.Code)
	}
}
