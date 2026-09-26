package admin

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/oidc"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

var oidcTestKey, _ = rsa.GenerateKey(rand.Reader, 2048)

const (
	oidcTestIssuer = "https://auth.example.org/application/o/holzcloud/"
	oidcTestClient = "holzcloud"
)

func newOIDCAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	h, sm, database := newForwardAuthAdmin(t, false)
	h.cfg.OIDCEnabled = true
	h.cfg.OIDCName = "Authentik"
	h.cfg.OIDCIssuer = oidcTestIssuer
	h.cfg.OIDCAuthorizeURL = "https://auth.example.org/application/o/authorize/"
	h.cfg.OIDCClientID = oidcTestClient
	h.cfg.OIDCRedirectURL = "https://cms.example.org/admin/oidc/callback"
	h.cfg.OIDCScopes = "openid email profile"
	h.cfg.OIDCUsernameClaim = "preferred_username"
	h.cfg.OIDCGroupsClaim = "groups"
	h.cfg.OIDCKeys = []oidc.Key{{ID: "k1", Public: &oidcTestKey.PublicKey}}
	return h, sm, database
}

// idToken signs a token as the provider would, with claims overriding the
// defaults (nil deletes one).
func idToken(t *testing.T, nonce string, over map[string]any) string {
	t.Helper()
	c := map[string]any{
		"iss": oidcTestIssuer, "aud": oidcTestClient, "sub": "hashed-subject",
		"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(),
		"nonce": nonce, "preferred_username": "ada", "email": "ada@example.com",
		"name": "Ada Lovelace", "groups": []string{"staff"},
	}
	for k, v := range over {
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
	}
	hj, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	cj, _ := json.Marshal(c)
	signed := base64.RawURLEncoding.EncodeToString(hj) + "." + base64.RawURLEncoding.EncodeToString(cj)
	digest := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, oidcTestKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// oidcStart runs the start handler and returns the redirect, the state cookie
// and the state and nonce it sent.
func oidcStart(t *testing.T, h *Handler, sm *scs.SessionManager) (*url.URL, *http.Cookie, string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.HandleOIDCStart(w, r); err != nil {
			t.Fatal(err)
		}
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/admin/oidc/start", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("start answered %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no state cookie")
	}
	return loc, cookie, loc.Query().Get("state"), loc.Query().Get("nonce")
}

type callbackResult struct {
	rec     *httptest.ResponseRecorder
	userID  int64
	role    string
	viaOIDC bool
	viaSSO  bool
}

func oidcCallback(t *testing.T, h *Handler, sm *scs.SessionManager, cookie *http.Cookie, form url.Values) callbackResult {
	t.Helper()
	req := httptest.NewRequest("POST", "/admin/oidc/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "203.0.113.9:5555"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	var res callbackResult
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.HandleOIDCCallback(w, r); err != nil {
			t.Fatal(err)
		}
		res.userID = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		res.role = sm.GetString(r.Context(), auth.SessionKeyUserRole)
		res.viaOIDC = sm.GetBool(r.Context(), auth.SessionKeyViaOIDC)
		res.viaSSO = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
	})).ServeHTTP(rec, req)
	res.rec = rec
	return res
}

func TestOIDCStartSendsTheBrowserToTheProvider(t *testing.T) {
	h, sm, _ := newOIDCAdmin(t)
	loc, cookie, state, nonce := oidcStart(t, h, sm)

	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != h.cfg.OIDCAuthorizeURL {
		t.Errorf("sent to %s", got)
	}
	q := loc.Query()
	for k, want := range map[string]string{
		"response_type": "id_token", "response_mode": "form_post", "client_id": oidcTestClient,
		"redirect_uri": h.cfg.OIDCRedirectURL, "scope": "openid email profile",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if len(state) < 40 || len(nonce) < 40 || state == nonce {
		t.Errorf("state %q, nonce %q", state, nonce)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteNoneMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Errorf("the cookie is not a __Host- cookie a cross-site post carries: %+v", cookie)
	}
	if !strings.HasPrefix(cookie.Value, state+"."+nonce+".") {
		t.Errorf("the cookie does not carry what was sent")
	}
}

func TestOIDCSignsInALinkedAccount(t *testing.T) {
	h, sm, database := newOIDCAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	_, cookie, state, nonce := oidcStart(t, h, sm)

	res := oidcCallback(t, h, sm, cookie, url.Values{"state": {state}, "id_token": {idToken(t, nonce, nil)}})

	if res.rec.Code != http.StatusSeeOther || res.rec.Header().Get("Location") != "/admin/" {
		t.Fatalf("answered %d to %q", res.rec.Code, res.rec.Header().Get("Location"))
	}
	if res.userID != id || res.role != user.RoleEditor {
		t.Errorf("session: user %d role %q", res.userID, res.role)
	}
	if !res.viaOIDC || res.viaSSO {
		t.Errorf("marks: oidc %v, sso %v", res.viaOIDC, res.viaSSO)
	}
	// The state cookie is spent.
	for _, c := range res.rec.Result().Cookies() {
		if c.Name == oidcCookieName && c.MaxAge >= 0 {
			t.Errorf("the state cookie survives the callback: %+v", c)
		}
	}
	// And the protocol says which way in it was.
	var found bool
	for _, row := range activityRows(t, database) {
		if row.Action == activity.ActionAuthLoginFail {
			t.Errorf("a refusal was recorded: %+v", row)
		}
		found = found || row.Action == activity.ActionAuthLoginSuccess
	}
	if !found {
		t.Error("no sign-in row")
	}
}

func TestOIDCGroupsGrantAdministrationAndProvisioning(t *testing.T) {
	h, sm, database := newOIDCAdmin(t)
	h.users.Params = cheapHashing
	ws, err := h.domains.CreateWebsite(context.Background(), "Provisioned into", "")
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.SSOProvision = true
	h.cfg.SSODefaultWebsite = ws.ID
	h.cfg.SSOAdminGroup = "holzcloud-admins"

	_, cookie, state, nonce := oidcStart(t, h, sm)
	tok := idToken(t, nonce, map[string]any{"groups": []string{"holzcloud-admins"}})
	res := oidcCallback(t, h, sm, cookie, url.Values{"state": {state}, "id_token": {tok}})

	if res.userID == 0 || res.role != user.RoleAdmin {
		t.Fatalf("session: user %d role %q (%d)", res.userID, res.role, res.rec.Code)
	}
	_, _, role, _ := accountByEmail(t, database, "ada@example.com")
	if role != user.RoleAdmin {
		t.Errorf("stored role %q", role)
	}
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM activity_log WHERE action = $1 AND json_extract(metadata, '$.via') = 'oidc'`,
		activity.ActionUserCreate).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("the provisioning row does not say it came through OpenID Connect")
	}
}

func TestOIDCRefusesWhatDoesNotBelongToThisSignIn(t *testing.T) {
	h, sm, database := newOIDCAdmin(t)
	seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	for name, tc := range map[string]func(state, nonce string, cookie *http.Cookie) (*http.Cookie, url.Values){
		"another state": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {"guessed"}, "id_token": {idToken(t, nonce, nil)}}
		},
		"no cookie": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return nil, url.Values{"state": {state}, "id_token": {idToken(t, nonce, nil)}}
		},
		"another nonce": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "id_token": {idToken(t, "a nonce from another sign-in", nil)}}
		},
		"another issuer": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "id_token": {idToken(t, nonce, map[string]any{"iss": "https://evil.example/"})}}
		},
		"no username": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "id_token": {idToken(t, nonce, map[string]any{"preferred_username": nil})}}
		},
		"the provider refused": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "error": {"access_denied"}}
		},
		"an unlinked person": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "id_token": {idToken(t, nonce, map[string]any{
				"preferred_username": "mallory", "email": "mallory@example.com"})}}
		},
		"an unlinked person with a known address": func(state, nonce string, c *http.Cookie) (*http.Cookie, url.Values) {
			return c, url.Values{"state": {state}, "id_token": {idToken(t, nonce, map[string]any{
				"preferred_username": "mallory"})}}
		},
	} {
		_, cookie, state, nonce := oidcStart(t, h, sm)
		c, form := tc(state, nonce, cookie)
		res := oidcCallback(t, h, sm, c, form)
		if res.userID != 0 || res.viaOIDC {
			t.Errorf("%s: signed in as %d", name, res.userID)
		}
		if res.rec.Code != http.StatusSeeOther || res.rec.Header().Get("Location") != "/admin/login" {
			t.Errorf("%s: answered %d to %q", name, res.rec.Code, res.rec.Header().Get("Location"))
		}
	}
}

func TestOIDCIsNotThereWhileSwitchedOff(t *testing.T) {
	h, sm, _ := newOIDCAdmin(t)
	h.cfg.OIDCEnabled = false
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = h.HandleOIDCStart(w, r)
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/admin/oidc/start", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("start: %d", rec.Code)
	}
	res := oidcCallback(t, h, sm, nil, url.Values{})
	if res.rec.Code != http.StatusNotFound {
		t.Errorf("callback: %d", res.rec.Code)
	}
}

// Switching OpenID Connect off ends every session it made: they skipped the
// second factor on the provider's word.
func TestOIDCGuardEndsSessionsOnceSwitchedOff(t *testing.T) {
	h, sm, database := newOIDCAdmin(t)
	seedAccount(t, database, "ada@example.com", user.RoleAdmin)
	_, cookie, state, nonce := oidcStart(t, h, sm)
	res := oidcCallback(t, h, sm, cookie, url.Values{"state": {state}, "id_token": {idToken(t, nonce, nil)}})
	session := sessionCookie(t, sm, res.rec)
	if session == nil || res.userID == 0 {
		t.Fatal("no session to end")
	}

	run := func() int64 {
		var uid int64
		req := httptest.NewRequest("GET", "/admin/", nil)
		req.AddCookie(session)
		sm.LoadAndSave(h.OIDCGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		}))).ServeHTTP(httptest.NewRecorder(), req)
		return uid
	}
	if run() == 0 {
		t.Fatal("the guard ended a session while switched on")
	}
	h.cfg.OIDCEnabled = false
	if uid := run(); uid != 0 {
		t.Errorf("the session outlived the switch as user %d", uid)
	}
}
