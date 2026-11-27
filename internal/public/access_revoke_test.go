package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// unlockedCookie is what the real form hands a visitor who typed the password.
func unlockedCookie(t *testing.T, h *Handler, ws *domain.Website, slug, password string) *http.Cookie {
	t.Helper()
	rec := postUnlock(h, ws, "/freischalten", url.Values{"seite": {slug}, "passwort": {password}})
	cookies := rec.Result().Cookies()
	if rec.Code != http.StatusSeeOther || len(cookies) != 1 {
		t.Fatalf("unlock answered %d with %d cookies, want 303 and one", rec.Code, len(cookies))
	}
	return cookies[0]
}

func revokeSite(t *testing.T) (*Handler, *domain.Website, *page.Page, *page.Store, *db.DB) {
	t.Helper()
	h, database := newTestHandler(t)
	signers(h)
	ws := seedWebsite(t, database, "Demo")
	seedPage(t, database, ws.ID, "Preise", "preise", "# Preisliste\n\nNetto 12", "published")
	store := page.NewStore(database)
	pg := pageBySlug(t, store, ws.ID, "preise")
	protect(t, database, pg.ID, "erstes-passwort")
	return h, ws, pageBySlug(t, store, ws.ID, "preise"), store, database
}

func TestChangingThePagePasswordEndsEveryUnlock(t *testing.T) {
	h, ws, pg, store, _ := revokeSite(t)
	c := unlockedCookie(t, h, ws, "preise", "erstes-passwort")
	if rec := getWithCookie(h, ws, "/preise", "preise", c); rec.Code != http.StatusOK {
		t.Fatalf("before the change: %d, want 200", rec.Code)
	}

	if err := store.SetAccess(context.Background(), pg.ID,
		page.AccessUpdate{Protected: true, Password: "zweites-passwort"}, cheapGateParams); err != nil {
		t.Fatal(err)
	}
	rec := getWithCookie(h, ws, "/preise", "preise", c)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "Preisliste") {
		t.Fatalf("after the change: %d, want the gate", rec.Code)
	}
}

func TestChangingOnlyTheHintKeepsTheUnlock(t *testing.T) {
	h, ws, pg, store, _ := revokeSite(t)
	c := unlockedCookie(t, h, ws, "preise", "erstes-passwort")

	if err := store.SetAccess(context.Background(), pg.ID,
		page.AccessUpdate{Protected: true, Hint: "Neuer Hinweis"}, cheapGateParams); err != nil {
		t.Fatal(err)
	}
	if rec := getWithCookie(h, ws, "/preise", "preise", c); rec.Code != http.StatusOK {
		t.Fatalf("after a hint change: %d, want 200", rec.Code)
	}
}

func TestAnUnlockFromBeforeTheUpgradeNoLongerOpensThePage(t *testing.T) {
	h, ws, pg, _, _ := revokeSite(t)
	// 0.1.5 minted the token with the unlock signer itself.
	old := &http.Cookie{Name: unlockCookieName(pg.ID), Value: h.unlock.Token(pg.ID, time.Now().Add(time.Hour))}
	rec := getWithCookie(h, ws, "/preise", "preise", old)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("an unlock from before the upgrade: %d, want the gate", rec.Code)
	}
	// And the page still opens with the password.
	c := unlockedCookie(t, h, ws, "preise", "erstes-passwort")
	if rec := getWithCookie(h, ws, "/preise", "preise", c); rec.Code != http.StatusOK {
		t.Fatalf("after typing the password: %d, want 200", rec.Code)
	}
}

func TestAnUnlockOpensOnlyItsOwnPage(t *testing.T) {
	h, ws, pg, store, database := revokeSite(t)
	seedPage(t, database, ws.ID, "Intern", "intern", "Nur intern", "published")
	other := pageBySlug(t, store, ws.ID, "intern")
	protect(t, database, other.ID, "erstes-passwort")

	c := unlockedCookie(t, h, ws, "preise", "erstes-passwort")
	moved := &http.Cookie{Name: unlockCookieName(other.ID), Value: c.Value}
	if pg.ID == other.ID {
		t.Fatal("the test needs two pages")
	}
	if rec := getWithCookie(h, ws, "/intern", "intern", moved); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a token of another page opens this one: %d", rec.Code)
	}
}

var tokenShape = regexp.MustCompile(`^\d+-\d+\.[A-Za-z0-9_-]+$`)

func TestShareLinksAreUntouched(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := seedWebsite(t, database, "Demo")
	seedPage(t, database, ws.ID, "Entwurf", "entwurf", "Noch nicht fertig", "draft")
	pg := pageBySlug(t, page.NewStore(database), ws.ID, "entwurf")

	token := h.share.Token(pg.ID, time.Now().Add(time.Hour))
	if !tokenShape.MatchString(token) {
		t.Fatalf("token %q has not the id-expiry.signature shape", token)
	}
	req := httptest.NewRequest(http.MethodGet, "/vorschau/"+token, nil)
	req.Host = "demo.test"
	req.SetPathValue("token", token)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	if err := h.HandleShareLink(rec, req); err != nil {
		t.Fatalf("HandleShareLink: %v", err)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Noch nicht fertig") {
		t.Fatalf("share link: status %d", rec.Code)
	}
}
