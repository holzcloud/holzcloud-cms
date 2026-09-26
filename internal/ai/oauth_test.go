package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// oauthRig is one server with /ai, the OAuth endpoints and a signed-in
// administrator's id, the way cmd/holzcloud wires them.
type oauthRig struct {
	ts     *httptest.Server
	o      *OAuth
	db     *db.DB
	userID int64
	siteID int64
}

func newOAuthRig(t *testing.T) *oauthRig {
	t.Helper()
	database := newTestDB(t)
	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(context.Background(), "Testhof", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	res, err := database.Write.Exec(`INSERT INTO users (name, email, password, role) VALUES ('A', 'a@example.org', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()

	tokens := NewStore(database)
	o := &OAuth{Store: tokens}
	srv := NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database),
	}))
	srv.SetOAuth(o)

	mux := http.NewServeMux()
	mux.Handle("/ai", srv)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", o.HandleResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", o.HandleServerMetadata)
	mux.HandleFunc("POST /oauth/register", o.HandleRegister)
	mux.HandleFunc("POST /oauth/token", o.HandleToken)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &oauthRig{ts: ts, o: o, db: database, userID: userID, siteID: ws.ID}
}

func (g *oauthRig) postJSON(t *testing.T, path string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	res, err := http.Post(g.ts.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (g *oauthRig) token(t *testing.T, form url.Values) (int, map[string]any) {
	t.Helper()
	res, err := http.PostForm(g.ts.URL+"/oauth/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// mcpStatus calls tools/list with a key and reports the status.
func (g *oauthRig) mcpStatus(t *testing.T, key string) (int, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, g.ts.URL+"/ai",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode, res.Header
}

const testRedirect = "https://claude.ai/api/mcp/auth_callback"

func pkce() (verifier, challenge string) {
	verifier = strings.Repeat("v", 50)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorize registers a public client and walks it through consent, returning
// the client id and the code the browser would carry back.
func (g *oauthRig) authorize(t *testing.T, challenge string, canWrite bool) (string, string) {
	t.Helper()
	status, reg := g.postJSON(t, "/oauth/register", map[string]any{
		"client_name": "Claude", "redirect_uris": []string{testRedirect}, "token_endpoint_auth_method": "none",
	})
	if status != http.StatusCreated {
		t.Fatalf("register: %d %v", status, reg)
	}
	if _, has := reg["client_secret"]; has {
		t.Fatal("a public client got a secret")
	}
	clientID := reg["client_id"].(string)

	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {testRedirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"},
	}
	req, err := g.o.ParseAuthRequest(context.Background(), q)
	if err != nil {
		t.Fatalf("ParseAuthRequest: %v", err)
	}
	target, err := g.o.Grant(context.Background(), req, g.siteID, canWrite, g.userID)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	u, _ := url.Parse(target)
	if !strings.HasPrefix(target, testRedirect+"?") || u.Query().Get("state") != "xyz" {
		t.Fatalf("redirect %q", target)
	}
	return clientID, u.Query().Get("code")
}

// The whole round a hosted assistant makes: refused, discovers, registers, is
// let in, uses the key, renews it — and the old secrets stop working.
func TestOAuthRoundTrip(t *testing.T) {
	g := newOAuthRig(t)

	status, h := g.mcpStatus(t, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("without a key: %d", status)
	}
	if !strings.Contains(h.Get("WWW-Authenticate"), `resource_metadata="http://`) {
		t.Fatalf("the refusal does not say where to sign in: %q", h.Get("WWW-Authenticate"))
	}

	res, _ := http.Get(g.ts.URL + "/.well-known/oauth-authorization-server")
	var meta map[string]any
	_ = json.NewDecoder(res.Body).Decode(&meta)
	res.Body.Close()
	if meta["issuer"] != g.ts.URL || meta["token_endpoint"] != g.ts.URL+"/oauth/token" {
		t.Fatalf("metadata: %v", meta)
	}

	verifier, challenge := pkce()
	clientID, code := g.authorize(t, challenge, true)

	status, tok := g.token(t, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {testRedirect}, "code_verifier": {verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token: %d %v", status, tok)
	}
	access, refresh := tok["access_token"].(string), tok["refresh_token"].(string)
	if status, _ := g.mcpStatus(t, access); status != http.StatusOK {
		t.Fatalf("the issued key does not work: %d", status)
	}

	// The key is an ordinary key: one website, write access, and marked as
	// signed in by its client.
	list, _ := g.o.Store.List(context.Background())
	if len(list) != 1 || !list[0].ViaOAuth() || list[0].WebsiteID != g.siteID || !list[0].CanWrite || list[0].Admin {
		t.Fatalf("stored key: %+v", list)
	}

	// A code works once.
	if status, _ := g.token(t, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID}, "code_verifier": {verifier},
	}); status != http.StatusBadRequest {
		t.Fatalf("a code was accepted twice: %d", status)
	}

	status, tok = g.token(t, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}})
	if status != http.StatusOK {
		t.Fatalf("refresh: %d %v", status, tok)
	}
	if status, _ := g.mcpStatus(t, tok["access_token"].(string)); status != http.StatusOK {
		t.Fatalf("the renewed key does not work: %d", status)
	}
	if status, _ := g.mcpStatus(t, access); status != http.StatusUnauthorized {
		t.Fatalf("the replaced key still works: %d", status)
	}
	if status, _ := g.token(t, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}); status != http.StatusBadRequest {
		t.Fatalf("a refresh secret was accepted twice: %d", status)
	}

	// Revoking on the key screen ends the connection, renewal included.
	if err := g.o.Store.Revoke(context.Background(), list[0].ID); err != nil {
		t.Fatal(err)
	}
	if status, _ := g.token(t, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok["refresh_token"].(string)}, "client_id": {clientID}}); status != http.StatusBadRequest {
		t.Fatalf("a revoked connection renewed itself: %d", status)
	}
}

func TestOAuthRefusesWrongVerifier(t *testing.T) {
	g := newOAuthRig(t)
	_, challenge := pkce()
	clientID, code := g.authorize(t, challenge, false)
	if status, _ := g.token(t, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"code_verifier": {strings.Repeat("w", 50)},
	}); status != http.StatusBadRequest {
		t.Fatalf("a wrong verifier was accepted: %d", status)
	}
}

func TestOAuthExpiredKeyIsRefused(t *testing.T) {
	g := newOAuthRig(t)
	verifier, challenge := pkce()
	clientID, code := g.authorize(t, challenge, false)
	_, tok := g.token(t, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID}, "code_verifier": {verifier},
	})
	g.db.Write.Exec(`UPDATE ai_tokens SET expires_at = $1`, time.Now().UTC().Add(-time.Minute).Format(timeLayout))
	if status, _ := g.mcpStatus(t, tok["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("an expired key worked: %d", status)
	}
}

func TestOAuthAuthRequestChecks(t *testing.T) {
	g := newOAuthRig(t)
	_, reg := g.postJSON(t, "/oauth/register", map[string]any{
		"client_name": "X", "redirect_uris": []string{testRedirect}, "token_endpoint_auth_method": "none",
	})
	id := reg["client_id"].(string)
	base := url.Values{
		"response_type": {"code"}, "client_id": {id}, "redirect_uri": {testRedirect},
		"code_challenge": {"abc"}, "code_challenge_method": {"S256"},
	}
	for name, change := range map[string]func(url.Values){
		"unknown client":  func(v url.Values) { v.Set("client_id", "hcc_nobody") },
		"other redirect":  func(v url.Values) { v.Set("redirect_uri", "https://claude.ai.evil.example/cb") },
		"redirect prefix": func(v url.Values) { v.Set("redirect_uri", testRedirect+"/more") },
		"no PKCE":         func(v url.Values) { v.Del("code_challenge") },
		"plain PKCE":      func(v url.Values) { v.Set("code_challenge_method", "plain") },
		"implicit grant":  func(v url.Values) { v.Set("response_type", "token") },
	} {
		v := url.Values{}
		for k, s := range base {
			v[k] = append([]string(nil), s...)
		}
		change(v)
		if _, err := g.o.ParseAuthRequest(context.Background(), v); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestOAuthRegistrationChecks(t *testing.T) {
	g := newOAuthRig(t)
	for _, uri := range []string{"http://example.org/cb", "javascript:alert(1)", "https://x.example/cb#frag", "/relative"} {
		if status, _ := g.postJSON(t, "/oauth/register", map[string]any{"redirect_uris": []string{uri}}); status != http.StatusBadRequest {
			t.Errorf("%q: %d", uri, status)
		}
	}
	if status, _ := g.postJSON(t, "/oauth/register", map[string]any{"redirect_uris": []string{"http://127.0.0.1:33418/cb"}}); status != http.StatusCreated {
		t.Errorf("loopback refused: %d", status)
	}
}

// A client that registered with a secret must present it.
func TestOAuthConfidentialClient(t *testing.T) {
	g := newOAuthRig(t)
	_, reg := g.postJSON(t, "/oauth/register", map[string]any{"client_name": "C", "redirect_uris": []string{testRedirect}})
	id, secret := reg["client_id"].(string), reg["client_secret"].(string)
	verifier, challenge := pkce()
	req, _ := g.o.ParseAuthRequest(context.Background(), url.Values{
		"response_type": {"code"}, "client_id": {id}, "redirect_uri": {testRedirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
	})
	target, _ := g.o.Grant(context.Background(), req, 0, false, g.userID)
	u, _ := url.Parse(target)
	code := u.Query().Get("code")

	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id}, "code_verifier": {verifier}}
	if status, _ := g.token(t, form); status != http.StatusUnauthorized {
		t.Fatalf("no secret accepted: %d", status)
	}
	form.Set("client_secret", secret)
	if status, out := g.token(t, form); status != http.StatusOK {
		t.Fatalf("with secret: %d %v", status, out)
	}
}
