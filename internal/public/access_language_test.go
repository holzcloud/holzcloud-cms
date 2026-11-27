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

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
)

// cheapGateParams keep the gate tests fast; the real ones are tuned for the server.
var cheapGateParams = auth.Argon2Params{
	Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
}

// signers gives a handler the two signers the server's wiring gives it.
func signers(h *Handler) {
	h.SetShareSigners(sharelink.New([]byte("share-secret-share-secret-share!")),
		sharelink.New([]byte("unlock-secret-unlock-secret-unl!")))
}

// protect puts a password in front of a page.
func protect(t *testing.T, database *db.DB, id int64, password string) {
	t.Helper()
	if err := page.NewStore(database).SetAccess(context.Background(), id,
		page.AccessUpdate{Protected: true, Password: password}, cheapGateParams); err != nil {
		t.Fatalf("SetAccess: %v", err)
	}
}

// postUnlock sends the gate's form to target the way a browser does, through
// the language middleware like every other public request.
func postUnlock(h *Handler, ws *domain.Website, target string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = h.HandleUnlock(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

var gateSeite = regexp.MustCompile(`name="seite"\s+value="([^"]*)"`)

// gateSeiteValue is what the form of the gate in the body would post as the page.
func gateSeiteValue(t *testing.T, body string) string {
	t.Helper()
	m := gateSeite.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no hidden seite field in the gate:\n%s", body)
	}
	return m[1]
}

func getWithCookie(h *Handler, ws *domain.Website, target, slug string, c *http.Cookie) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	if c != nil {
		req.AddCookie(c)
	}
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slug == "" {
			_ = h.HandleHome(w, r)
			return
		}
		r.SetPathValue("slug", strings.TrimPrefix(r.URL.Path, "/"))
		_ = h.HandlePage(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

func TestAProtectedPageInASecondLanguageUnlocks(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Geheim", "geheim", "Deutscher Inhalt", "published", "", 0)
	fr := seedPageIn(t, database, ws.ID, "Secret", "secret", "Contenu francais", "published", "fr", de.ID)
	protect(t, database, de.ID, "deutsch1")
	protect(t, database, fr.ID, "francais1")

	rec := getWithCookie(h, ws, "/fr/secret", "secret", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked status = %d, want 401", rec.Code)
	}
	seite := gateSeiteValue(t, rec.Body.String())
	if seite != "fr/secret" {
		t.Fatalf("the gate posts seite = %q, want fr/secret", seite)
	}

	// The German password is the German page's, not this one's.
	rec = postUnlock(h, ws, "/freischalten", url.Values{"seite": {seite}, "passwort": {"deutsch1"}})
	if rec.Code != http.StatusUnauthorized || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("the German password on the French page: %d, cookies %v", rec.Code, rec.Result().Cookies())
	}

	rec = postUnlock(h, ws, "/freischalten", url.Values{"seite": {seite}, "passwort": {"francais1"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/fr/secret" {
		t.Fatalf("unlock answered %d to %q, want 303 to /fr/secret", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/fr/secret" {
		t.Fatalf("cookies = %+v, want one with Path /fr/secret", cookies)
	}

	rec = getWithCookie(h, ws, "/fr/secret", "secret", cookies[0])
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Contenu francais") {
		t.Fatalf("after unlock: %d, content missing", rec.Code)
	}
}

func TestTheHomePageOfASecondLanguageUnlocks(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := multilingualSite(t, database, "fr")
	// Filed under the language right away: the slug "home" is unique per
	// website until the page has been given its language.
	fr := seedPageIn(t, database, ws.ID, "Accueil", "home", "Bienvenue chez nous", "published", "fr", 0)
	protect(t, database, fr.ID, "francais1")

	rec := getWithCookie(h, ws, "/fr", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked status = %d, want 401", rec.Code)
	}
	seite := gateSeiteValue(t, rec.Body.String())
	if seite != "fr/home" {
		t.Fatalf("the gate posts seite = %q, want fr/home", seite)
	}

	rec = postUnlock(h, ws, "/freischalten", url.Values{"seite": {seite}, "passwort": {"francais1"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/fr" {
		t.Fatalf("unlock answered %d to %q, want 303 to /fr", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/fr" {
		t.Fatalf("cookies = %+v, want one with Path /fr", cookies)
	}
	rec = getWithCookie(h, ws, "/fr", "", cookies[0])
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Bienvenue chez nous") {
		t.Fatalf("after unlock: %d, content missing", rec.Code)
	}
}

func TestAnUnlockPostedUnderTheLanguagePrefixFindsThePage(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Geheim", "geheim", "Deutscher Inhalt", "published", "", 0)
	fr := seedPageIn(t, database, ws.ID, "Secret", "secret", "Contenu francais", "published", "fr", de.ID)
	protect(t, database, fr.ID, "francais1")

	rec := postUnlock(h, ws, "/fr/freischalten", url.Values{"seite": {"secret"}, "passwort": {"francais1"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/fr/secret" {
		t.Fatalf("unlock answered %d to %q, want 303 to /fr/secret", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTheMainLanguageUnlockIsUnchanged(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Geheim", "geheim", "Deutscher Inhalt", "published", "", 0)
	protect(t, database, de.ID, "deutsch1")

	rec := getWithCookie(h, ws, "/geheim", "geheim", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked status = %d, want 401", rec.Code)
	}
	seite := gateSeiteValue(t, rec.Body.String())
	if seite != "geheim" {
		t.Fatalf("the gate posts seite = %q, want geheim", seite)
	}
	rec = postUnlock(h, ws, "/freischalten", url.Values{"seite": {seite}, "passwort": {"deutsch1"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/geheim" {
		t.Fatalf("unlock answered %d to %q, want 303 to /geheim", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/geheim" {
		t.Fatalf("cookies = %+v, want one with Path /geheim", cookies)
	}
}

// postUnlockFrom is postUnlock as the client at remote.
func postUnlockFrom(h *Handler, ws *domain.Website, remote, target string, form url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "demo.test"
	req.RemoteAddr = remote
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = h.HandleUnlock(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

// The form posts to /freischalten, which carries no language prefix. Every
// answer that draws the gate again (a wrong password, a full throttle, a busy
// server) must still be the page's own language: its words, its menu. Before
// this, a French visitor who mistyped once met the German gate.
func TestTheGateAnswersAPostInThePagesLanguage(t *testing.T) {
	h, database := newTestHandler(t)
	signers(h)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Geheim", "geheim", "Deutscher Inhalt", "published", "", 0)
	fr := seedPageIn(t, database, ws.ID, "Secret", "secret", "Contenu francais", "published", "fr", de.ID)
	protect(t, database, fr.ID, "francais1")

	menus := menu.NewStore(database)
	for _, m := range []struct{ name, loc, item string }{
		{"Haupt", "", "Hauptmenue-de"},
		{"Principal", "fr", "Menu-principal-fr"},
	} {
		created, err := menus.CreateMenu(context.Background(), ws.ID, m.name, "main", m.loc)
		if err != nil {
			t.Fatalf("CreateMenu %s: %v", m.loc, err)
		}
		if _, err := menus.CreateItem(context.Background(), created.ID, nil, m.item, "url", "/", nil, 0); err != nil {
			t.Fatalf("CreateItem %s: %v", m.loc, err)
		}
	}

	inFrench := func(name string, rec *httptest.ResponseRecorder, status int) {
		t.Helper()
		body := rec.Body.String()
		if rec.Code != status {
			t.Fatalf("%s: status %d, want %d", name, rec.Code, status)
		}
		if !strings.Contains(body, `<p class="gate-lang">fr</p>`) {
			t.Errorf("%s: the gate is not drawn in French:\n%s", name, body)
		}
		if !strings.Contains(body, "Menu-principal-fr") || strings.Contains(body, "Hauptmenue-de") {
			t.Errorf("%s: the gate carries the main language's menu:\n%s", name, body)
		}
		if got := gateSeiteValue(t, body); got != "fr/secret" {
			t.Errorf("%s: the gate posts seite = %q, want fr/secret", name, got)
		}
	}

	wrong := url.Values{"seite": {"fr/secret"}, "passwort": {"falsch-falsch"}}
	const client = "198.51.100.40:4000"
	for i := 0; i < gateClientMax; i++ {
		inFrench("wrong password", postUnlockFrom(h, ws, client, "/freischalten", wrong), http.StatusUnauthorized)
	}
	inFrench("throttled", postUnlockFrom(h, ws, client, "/freischalten", wrong), http.StatusTooManyRequests)

	gate := auth.NewGate(1, time.Millisecond, time.Millisecond)
	release, err := gate.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	previous := auth.SetArgon2Gate(gate)
	t.Cleanup(func() { auth.SetArgon2Gate(previous) })
	inFrench("busy", postUnlockFrom(h, ws, "198.51.100.41:4000", "/freischalten", wrong), http.StatusServiceUnavailable)
	release()

	// The main language's page keeps its own: German, the main menu.
	protect(t, database, de.ID, "deutsch1")
	rec := postUnlockFrom(h, ws, "198.51.100.42:4000", "/freischalten", url.Values{"seite": {"geheim"}, "passwort": {"falsch-falsch"}})
	if body := rec.Body.String(); rec.Code != http.StatusUnauthorized ||
		!strings.Contains(body, `<p class="gate-lang">de</p>`) || !strings.Contains(body, "Hauptmenue-de") {
		t.Errorf("the main language's gate after a wrong password: %d\n%s", rec.Code, body)
	}
}
