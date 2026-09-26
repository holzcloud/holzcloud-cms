package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Signing an assistant in with OAuth.
//
// A key pasted into a config file is how a program on the operator's own
// machine connects. The hosted assistants — Claude in the browser and the app,
// ChatGPT — do not take a pasted key: they connect to an MCP server only
// through OAuth 2.1, as the MCP specification describes it. They find this
// server's endpoints under /.well-known, register themselves, send the operator
// to a page in the administration to agree, and exchange what comes back for a
// key.
//
// The key they get is a row in ai_tokens like any other. It has the scope the
// operator chose on the consent page, it appears on the key screen, and
// revoking it there ends the connection. OAuth adds a way to hand a key over;
// it adds no second kind of key and no second place that decides.
//
// What is implemented is the part those clients use and nothing more: dynamic
// registration (RFC 7591), the authorization code grant with PKCE (S256 only),
// refresh tokens that are replaced on every use, and the two metadata
// documents (RFC 8414, RFC 9728). No implicit grant, no password grant, no
// client credentials — each of those is a way to get a key without an
// administrator saying yes on a screen.

// The lifetimes of what OAuth hands out.
const (
	// AccessLifetime is how long a key from OAuth is valid before the client
	// must renew it. Short, because renewing costs the client nothing and a
	// copied key should be worth little.
	AccessLifetime = time.Hour
	// RefreshLifetime is how long a connection may lie unused and still be
	// renewed. Every renewal starts it again, so a connection in use does not
	// run out; one forgotten for three months does.
	RefreshLifetime = 90 * 24 * time.Hour
	// CodeLifetime is how long the code from the consent page may take to
	// arrive at the token endpoint. The client exchanges it within seconds.
	CodeLifetime = 10 * time.Minute
)

// ConsentPath is the page in the administration where the operator agrees.
const ConsentPath = "/admin/ai/verbinden"

// Prefixes that make these secrets recognisable, like TokenPrefix.
const (
	clientPrefix  = "hcc_"
	secretPrefix  = "hcs_"
	codePrefix    = "hca_"
	refreshPrefix = "hcr_"
)

// pendingClientLimit bounds how many registered clients without a key may
// exist at once. Registration is open to anybody — that is what dynamic
// registration means — so something has to stop it from filling the database.
const pendingClientLimit = 200

// Client is an assistant that registered itself.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
	Confidential bool
	secretHash   string
}

// AllowsRedirect reports whether a code may be sent to this address. Whole
// strings only: a prefix match is how an attacker's path on the same host
// becomes a place a code can be sent.
func (c *Client) AllowsRedirect(uri string) bool {
	for _, u := range c.RedirectURIs {
		if u == uri {
			return true
		}
	}
	return false
}

// OAuth answers the OAuth endpoints and hands out keys through the Store.
type OAuth struct {
	Store *Store
	// Secure is the deployment's HOLZCLOUD_SECURE. It decides the scheme of
	// the addresses published in the metadata, for the same reason the sitemap
	// uses it: a header a client can set must not decide what this server says
	// its own address is.
	Secure bool
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

func (o *OAuth) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// Base is this server's address as the request reached it.
func (o *OAuth) Base(r *http.Request) string {
	scheme := "http"
	if o.Secure || r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ResourceMetadataURL is where a client learns who signs it in. It goes into
// the WWW-Authenticate header of every refused MCP request, which is how a
// client that knows only the address /ai finds everything else.
func (o *OAuth) ResourceMetadataURL(r *http.Request) string {
	return o.Base(r) + "/.well-known/oauth-protected-resource"
}

// HandleResourceMetadata answers RFC 9728: this resource, and who issues keys
// for it. Both are this server.
func (o *OAuth) HandleResourceMetadata(w http.ResponseWriter, r *http.Request) {
	base := o.Base(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 base + "/ai",
		"authorization_servers":    []string{base},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Holzcloud CMS",
	})
}

// HandleServerMetadata answers RFC 8414.
func (o *OAuth) HandleServerMetadata(w http.ResponseWriter, r *http.Request) {
	base := o.Base(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic", "client_secret_post"},
	})
}

// HandleAuthorize sends the browser on to the consent page.
//
// The page lives in the administration, because agreeing needs a signed-in
// administrator and the administration is where signing in, the second factor
// and the password prompt already are. This address exists so the metadata can
// name an endpoint that is not an admin path.
func (o *OAuth) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	target := ConsentPath
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// registration is what a client sends to register (RFC 7591), as far as it
// matters here.
type registration struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// HandleRegister lets a client introduce itself.
func (o *OAuth) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req registration
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration cannot be read")
		return
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 5 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "between one and five redirect_uris are needed")
		return
	}
	for _, u := range req.RedirectURIs {
		if err := checkRedirectURI(u); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "MCP client"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	method := req.TokenEndpointAuthMethod
	switch method {
	case "none", "client_secret_basic", "client_secret_post":
	case "":
		// RFC 7591 names client_secret_basic as the default.
		method = "client_secret_basic"
	default:
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method must be none, client_secret_basic or client_secret_post")
		return
	}

	client, secret, err := o.Store.RegisterClient(r.Context(), name, req.RedirectURIs, method != "none")
	if errors.Is(err, errTooManyClients) {
		oauthError(w, http.StatusTooManyRequests, "temporarily_unavailable", err.Error())
		return
	}
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "the client could not be stored")
		return
	}

	resp := map[string]any{
		"client_id":                  client.ID,
		"client_id_issued_at":        o.now().Unix(),
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": method,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	if secret != "" {
		resp["client_secret"] = secret
		resp["client_secret_expires_at"] = 0
	}
	writeJSON(w, http.StatusCreated, resp)
}

// checkRedirectURI accepts https addresses, and http only on the loopback
// interface where a desktop client listens for its own callback.
func checkRedirectURI(raw string) error {
	if len(raw) > 2000 {
		return errors.New("a redirect_uri is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%q is not an absolute address", raw)
	}
	if u.Fragment != "" {
		return fmt.Errorf("%q carries a fragment", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return fmt.Errorf("%q must use https", raw)
}

// AuthRequest is a validated request from the consent page's address.
type AuthRequest struct {
	Client      *Client
	RedirectURI string
	State       string
	Challenge   string
	Scope       string
	Resource    string
}

// ErrAuthRequest marks a request the consent page refuses to show. It is never
// answered with a redirect: an unknown client or an unregistered address is
// exactly the case in which sending the browser anywhere would be wrong.
var ErrAuthRequest = errors.New("oauth: not a valid authorisation request")

// ParseAuthRequest checks the parameters the client put into the address.
func (o *OAuth) ParseAuthRequest(ctx context.Context, q url.Values) (*AuthRequest, error) {
	client, err := o.Store.Client(ctx, q.Get("client_id"))
	if err != nil {
		return nil, fmt.Errorf("%w: unknown client", ErrAuthRequest)
	}
	redirect := q.Get("redirect_uri")
	if redirect == "" && len(client.RedirectURIs) == 1 {
		redirect = client.RedirectURIs[0]
	}
	if !client.AllowsRedirect(redirect) {
		return nil, fmt.Errorf("%w: the redirect address is not registered", ErrAuthRequest)
	}
	if q.Get("response_type") != "code" {
		return nil, fmt.Errorf("%w: only response_type=code", ErrAuthRequest)
	}
	challenge := q.Get("code_challenge")
	if challenge == "" || q.Get("code_challenge_method") != "S256" {
		return nil, fmt.Errorf("%w: PKCE with S256 is required", ErrAuthRequest)
	}
	return &AuthRequest{
		Client:      client,
		RedirectURI: redirect,
		State:       q.Get("state"),
		Challenge:   challenge,
		Scope:       q.Get("scope"),
		Resource:    q.Get("resource"),
	}, nil
}

// Values are the request's parameters again, for the consent form to carry
// from the page to the button.
func (a *AuthRequest) Values() url.Values {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", a.Client.ID)
	v.Set("redirect_uri", a.RedirectURI)
	v.Set("code_challenge", a.Challenge)
	v.Set("code_challenge_method", "S256")
	for k, s := range map[string]string{"state": a.State, "scope": a.Scope, "resource": a.Resource} {
		if s != "" {
			v.Set(k, s)
		}
	}
	return v
}

// RedirectOrigin is the scheme and host the browser is sent to afterwards —
// what the consent page names, and what its form may be answered with.
func (a *AuthRequest) RedirectOrigin() string {
	u, err := url.Parse(a.RedirectURI)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// RedirectHost is the host alone, for the sentence on the consent page.
func (a *AuthRequest) RedirectHost() string {
	u, err := url.Parse(a.RedirectURI)
	if err != nil {
		return ""
	}
	return u.Host
}

// DenyURL is where the browser goes when the operator says no.
func (a *AuthRequest) DenyURL() string {
	v := url.Values{}
	v.Set("error", "access_denied")
	if a.State != "" {
		v.Set("state", a.State)
	}
	return withQuery(a.RedirectURI, v)
}

// Grant records the operator's yes and returns where to send the browser.
func (o *OAuth) Grant(ctx context.Context, a *AuthRequest, websiteID int64, canWrite bool, userID int64) (string, error) {
	code := codePrefix + randomString(32)
	var site any
	if websiteID > 0 {
		site = websiteID
	}
	_, err := o.Store.DB.Write.ExecContext(ctx,
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, challenge, website_id, can_write, user_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		hash(code), a.Client.ID, a.RedirectURI, a.Challenge, site, boolToInt(canWrite), userID,
		o.now().Add(CodeLifetime).Format(timeLayout))
	if err != nil {
		return "", fmt.Errorf("store code: %w", err)
	}
	v := url.Values{}
	v.Set("code", code)
	if a.State != "" {
		v.Set("state", a.State)
	}
	return withQuery(a.RedirectURI, v), nil
}

// HandleToken exchanges a code or a refresh secret for a key.
func (o *OAuth) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the request cannot be read")
		return
	}
	client, ok := o.authenticateClient(w, r)
	if !ok {
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		o.exchangeCode(w, r, client)
	case "refresh_token":
		o.refresh(w, r, client)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "authorization_code or refresh_token")
	}
}

// authenticateClient finds the client and, for a confidential one, checks its
// secret. A public client proves itself with PKCE and the refresh secret.
func (o *OAuth) authenticateClient(w http.ResponseWriter, r *http.Request) (*Client, bool) {
	id, secret, basic := r.BasicAuth()
	if basic {
		// RFC 6749 §2.3.1: both halves are form-encoded before they are joined.
		id, _ = url.QueryUnescape(id)
		secret, _ = url.QueryUnescape(secret)
	} else {
		id = r.PostForm.Get("client_id")
		secret = r.PostForm.Get("client_secret")
	}
	client, err := o.Store.Client(r.Context(), id)
	if err != nil {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
		return nil, false
	}
	if client.Confidential {
		if subtle.ConstantTimeCompare([]byte(hash(secret)), []byte(client.secretHash)) != 1 {
			oauthError(w, http.StatusUnauthorized, "invalid_client", "the client secret is wrong")
			return nil, false
		}
	}
	return client, true
}

func (o *OAuth) exchangeCode(w http.ResponseWriter, r *http.Request, client *Client) {
	code := r.PostForm.Get("code")
	var (
		clientID, redirect, challenge, expires string
		websiteID                              *int64
		canWrite                               int
	)
	// Deleted as it is read: a code is good for exactly one exchange, and two
	// requests racing with the same code cannot both find it.
	err := o.Store.DB.Write.QueryRowContext(r.Context(),
		`DELETE FROM oauth_codes WHERE code_hash = $1
		 RETURNING client_id, redirect_uri, challenge, website_id, can_write, expires_at`,
		hash(code)).Scan(&clientID, &redirect, &challenge, &websiteID, &canWrite, &expires)
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code is unknown or already used")
		return
	}
	if clientID != client.ID {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code belongs to another client")
		return
	}
	if uri := r.PostForm.Get("redirect_uri"); uri != "" && uri != redirect {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match")
		return
	}
	if t, perr := time.Parse(timeLayout, expires); perr != nil || o.now().After(t) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code has expired")
		return
	}
	if !pkceMatches(r.PostForm.Get("code_verifier"), challenge) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "code_verifier does not match")
		return
	}

	access := TokenPrefix + randomString(32)
	refresh := refreshPrefix + randomString(32)
	var site any
	if websiteID != nil {
		site = *websiteID
	}
	now := o.now()
	_, err = o.Store.DB.Write.ExecContext(r.Context(),
		`INSERT INTO ai_tokens (name, token_hash, website_id, can_write, is_admin, expires_at, client_id, refresh_hash, refresh_expires_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8)`,
		client.Name, hash(access), site, canWrite, now.Add(AccessLifetime).Format(timeLayout),
		client.ID, hash(refresh), now.Add(RefreshLifetime).Format(timeLayout))
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "the key could not be stored")
		return
	}
	writeTokens(w, access, refresh)
}

func (o *OAuth) refresh(w http.ResponseWriter, r *http.Request, client *Client) {
	old := r.PostForm.Get("refresh_token")
	access := TokenPrefix + randomString(32)
	refresh := refreshPrefix + randomString(32)
	now := o.now()
	// One statement finds and replaces, so a refresh secret used twice — a copy
	// in the wrong hands, or a client retrying — works exactly once.
	var id int64
	err := o.Store.DB.Write.QueryRowContext(r.Context(),
		`UPDATE ai_tokens
		    SET token_hash = $1, expires_at = $2, refresh_hash = $3, refresh_expires_at = $4
		  WHERE refresh_hash = $5 AND client_id = $6 AND refresh_expires_at > $7
		 RETURNING id`,
		hash(access), now.Add(AccessLifetime).Format(timeLayout),
		hash(refresh), now.Add(RefreshLifetime).Format(timeLayout),
		hash(old), client.ID, now.Format(timeLayout)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token is unknown, used or expired")
		return
	}
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "the key could not be renewed")
		return
	}
	writeTokens(w, access, refresh)
}

func writeTokens(w http.ResponseWriter, access, refresh string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(AccessLifetime / time.Second),
		"refresh_token": refresh,
	})
}

// pkceMatches checks S256: the challenge is the unpadded base64url SHA-256 of
// the verifier.
func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

var errTooManyClients = errors.New("too many registrations are waiting for consent; try again tomorrow")

// RegisterClient stores a new client and returns its secret, if it has one.
//
// Registrations that never led to a key are pruned after a day, and while too
// many are waiting no new one is taken. Nothing else limits registration, and
// nothing needs to: a client that nobody agreed to cannot do anything.
func (s *Store) RegisterClient(ctx context.Context, name string, redirectURIs []string, confidential bool) (*Client, string, error) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(timeLayout)
	if _, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM oauth_clients
		  WHERE created_at < $1
		    AND client_id NOT IN (SELECT client_id FROM ai_tokens WHERE client_id IS NOT NULL)`, cutoff); err != nil {
		return nil, "", fmt.Errorf("prune clients: %w", err)
	}
	var waiting int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM oauth_clients
		  WHERE client_id NOT IN (SELECT client_id FROM ai_tokens WHERE client_id IS NOT NULL)`).Scan(&waiting); err != nil {
		return nil, "", fmt.Errorf("count clients: %w", err)
	}
	if waiting >= pendingClientLimit {
		return nil, "", errTooManyClients
	}

	c := &Client{ID: clientPrefix + randomString(16), Name: name, RedirectURIs: redirectURIs, Confidential: confidential}
	var secret string
	var secretHash any
	if confidential {
		secret = secretPrefix + randomString(32)
		c.secretHash = hash(secret)
		secretHash = c.secretHash
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO oauth_clients (client_id, name, redirect_uris, secret_hash) VALUES ($1, $2, $3, $4)`,
		c.ID, c.Name, strings.Join(redirectURIs, "\n"), secretHash); err != nil {
		return nil, "", fmt.Errorf("store client: %w", err)
	}
	return c, secret, nil
}

// Client looks a registered client up.
func (s *Store) Client(ctx context.Context, id string) (*Client, error) {
	if id == "" {
		return nil, sql.ErrNoRows
	}
	var (
		c      Client
		uris   string
		secret *string
	)
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT client_id, name, redirect_uris, secret_hash FROM oauth_clients WHERE client_id = $1`, id).
		Scan(&c.ID, &c.Name, &uris, &secret)
	if err != nil {
		return nil, err
	}
	c.RedirectURIs = strings.Split(uris, "\n")
	if secret != nil {
		c.Confidential = true
		c.secretHash = *secret
	}
	return &c, nil
}

func randomString(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand does not fail on a supported platform; if it ever does,
		// handing out a predictable secret is the one thing worse than stopping.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func withQuery(base string, v url.Values) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + v.Encode()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func oauthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}
