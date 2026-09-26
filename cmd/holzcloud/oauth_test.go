package main

import (
	"context"
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

	"github.com/holzcloud/holzcloud-cms/internal/auth"
)

// The sign-in a hosted assistant makes, through the real route table: the
// authorisation address, the consent page with its guards and headers, the
// button, and the exchange at the token endpoint.

const oauthRedirect = "https://claude.ai/api/mcp/auth_callback"

func oauthRequest(handler http.Handler, method, target string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Host = "admin.test"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func registerClient(t *testing.T, handler http.Handler) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/oauth/register",
		strings.NewReader(`{"client_name":"Claude","redirect_uris":["`+oauthRedirect+`"],"token_endpoint_auth_method":"none"}`))
	req.Host = "admin.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out["client_id"].(string)
}

func elevate(t *testing.T, sm *scs.SessionManager, cookie *http.Cookie) {
	t.Helper()
	ctx, err := sm.Load(context.Background(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	sm.Put(ctx, auth.SessionKeyElevated, time.Now().Unix())
	if _, _, err := sm.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOAuthConsentThroughTheRouter(t *testing.T) {
	handler, sm, database := testRouterWith(t, routerTweaks{oauth: true})
	cookie := seedUser(t, handler, sm, database, "admin@example.org", "admin")
	// An administrator without a second factor is sent to set one up first;
	// that guard is not what this test is about.
	if _, err := database.Write.Exec(`UPDATE users SET totp_confirmed_at = '2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	clientID := registerClient(t, handler)

	verifier := strings.Repeat("k", 60)
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {oauthRedirect},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"},
		"state": {"s1"},
	}

	// The metadata names this host.
	rec := oauthRequest(handler, "GET", "/.well-known/oauth-authorization-server", nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"authorization_endpoint":"http://admin.test/oauth/authorize"`) {
		t.Fatalf("metadata: %d %s", rec.Code, rec.Body)
	}

	// The authorisation address hands over to the consent page, and keeps the
	// window that opened it connected.
	rec = oauthRequest(handler, "GET", "/oauth/authorize?"+q.Encode(), nil, nil)
	consent := rec.Header().Get("Location")
	if rec.Code != http.StatusFound || !strings.HasPrefix(consent, "/admin/ai/verbinden?") {
		t.Fatalf("authorize: %d %q", rec.Code, consent)
	}
	if got := rec.Header().Get("Cross-Origin-Opener-Policy"); got != "unsafe-none" {
		t.Errorf("authorize COOP = %q", got)
	}

	// An editor is not asked.
	editor := seedUser(t, handler, sm, database, "editor@example.org", "editor")
	if rec := oauthRequest(handler, "GET", consent, nil, editor); rec.Code != http.StatusForbidden {
		t.Errorf("editor on the consent page: %d", rec.Code)
	}

	// The password first, and back to this very address afterwards.
	rec = oauthRequest(handler, "GET", consent, nil, cookie)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), auth.ConfirmPath+"?weiter=") {
		t.Fatalf("unconfirmed: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	back, _ := url.Parse(rec.Header().Get("Location"))
	if auth.SafeReturn(back.Query().Get("weiter")) != consent {
		t.Fatalf("the password prompt returns to %q, not the consent page", back.Query().Get("weiter"))
	}

	elevate(t, sm, cookie)
	rec = oauthRequest(handler, "GET", consent, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("consent page: %d %s", rec.Code, rec.Body)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' https://claude.ai;") {
		t.Errorf("the consent form cannot hand over to the client: %q", csp)
	}
	if got := rec.Header().Get("Cross-Origin-Opener-Policy"); got != "unsafe-none" {
		t.Errorf("consent COOP = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "claude.ai") {
		t.Error("the page does not name where the browser returns to")
	}
	// Everywhere else the policy is as strict as it was.
	if rec := oauthRequest(handler, "GET", "/admin/ai", nil, cookie); rec.Header().Get("Cross-Origin-Opener-Policy") != "same-origin" ||
		strings.Contains(rec.Header().Get("Content-Security-Policy"), "claude.ai") {
		t.Errorf("the key screen lost its headers: %v", rec.Header())
	}

	form := q
	form.Set("website", "0")
	form.Set("rechte", "schreiben")
	rec = oauthRequest(handler, "POST", "/admin/ai/verbinden", form, cookie)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(loc, oauthRedirect+"?") {
		t.Fatalf("connect: %d %q", rec.Code, loc)
	}
	u, _ := url.Parse(loc)
	if u.Query().Get("state") != "s1" || u.Query().Get("code") == "" {
		t.Fatalf("callback %q", loc)
	}

	rec = oauthRequest(handler, "POST", "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {u.Query().Get("code")},
		"client_id": {clientID}, "redirect_uri": {oauthRedirect}, "code_verifier": {verifier},
	}, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"access_token":"hc_`) {
		t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}
}

// A consent page reached while signed out comes back after signing in, with
// every parameter the assistant put into it.
func TestSignInReturnsToTheConsentPage(t *testing.T) {
	handler, sm, database := testRouterWith(t, routerTweaks{oauth: true})
	seedUser(t, handler, sm, database, "admin@example.org", "admin")
	target := "/admin/ai/verbinden?client_id=hcc_x&state=abc"

	rec := oauthRequest(handler, "GET", target, nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("signed out: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sm.Cookie.Name {
			session = c
		}
	}
	if session == nil {
		t.Fatal("the address was not remembered in a session")
	}
	if rec := oauthRequest(handler, "GET", "/admin/login", nil, session); rec.Header().Get("Cross-Origin-Opener-Policy") != "unsafe-none" {
		t.Errorf("the sign-in inside an OAuth window cuts it off from its opener")
	}

	rec = oauthRequest(handler, "POST", "/admin/login",
		url.Values{"email": {"admin@example.org"}, "password": {"ein sicheres passwort"}}, session)
	if got := rec.Header().Get("Location"); got != target {
		t.Fatalf("after signing in: %d %q, want %q", rec.Code, got, target)
	}
}
