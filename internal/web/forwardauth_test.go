package web

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testProxySecret = "correct-horse-battery-staple"

// probe is the handler behind the middleware. Every assertion about stripping
// is made from here — from the far side — because a grep for the word "strip"
// proves nothing about stripping.
type forwardAuthProbe struct {
	calls    int
	header   http.Header
	identity *Identity
	found    bool
}

func (p *forwardAuthProbe) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.calls++
		p.header = r.Header.Clone()
		p.identity, p.found = IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

// wireSpellings are set through Header.Set, which canonicalises exactly as
// net/http does when it reads them off the wire. Adding a thirteenth spelling
// is one line.
var wireSpellings = []string{
	"X-authentik-username",
	"X-authentik-email",
	"X-authentik-name",
	"X-authentik-uid",
	"X-authentik-groups",
	"X-authentik-entitlements",
	"X-authentik-jwt",
	"X-authentik-meta-provider",
	"X_authentik_email",
	"X_authentik_groups",
	"x-authentik-email",
	"X-AUTHENTIK-EMAIL",
}

// rawSpellings are planted straight into the map, bypassing canonicalisation.
// The wire cannot produce them; they are here so the strip cannot quietly come
// to depend on canonical form.
var rawSpellings = []string{
	"x-authentik-email",
	"X-AUTHENTIK-EMAIL",
	"x_authentik_username",
}

func requestCarryingEverySpelling(remoteAddr string) *http.Request {
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = remoteAddr
	for _, name := range wireSpellings {
		req.Header.Set(name, "stranger@example.com")
	}
	for _, name := range rawSpellings {
		req.Header[name] = []string{"stranger@example.com"}
	}
	return req
}

// assertNoIdentityHeaderSurvives walks the header map the handler was given and
// fails on any key whose normalised form carries the identity prefix or names
// the installation's own shared secret.
func assertNoIdentityHeaderSurvives(t *testing.T, h http.Header) {
	t.Helper()
	for name := range h {
		normalised := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		if strings.HasPrefix(normalised, "x-authentik-") {
			t.Errorf("the handler was given %q; every identity header must be gone before it runs", name)
		}
		if normalised == strings.ToLower(ProxySecretHeader) {
			t.Errorf("the handler was given %q; a handler must not be able to read the secret protecting it", name)
		}
	}
}

func serveForwardAuth(t *testing.T, req *http.Request, opts ForwardAuthOptions) *forwardAuthProbe {
	t.Helper()
	p := &forwardAuthProbe{}
	rec := httptest.NewRecorder()
	ForwardAuth(loopbackResolver(t), opts)(p.handler()).ServeHTTP(rec, req)
	if p.calls != 1 {
		t.Fatalf("the next handler ran %d times; the middleware must call it exactly once on every path", p.calls)
	}
	return p
}

func TestForwardAuthBelievesATrustedPeerThatProvesItself(t *testing.T) {
	req := requestCarryingEverySpelling("127.0.0.1:41234")
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set("X-authentik-email", "ada@example.com")
	req.Header.Set("X-authentik-name", "Ada Lovelace")
	req.Header.Set("X-authentik-groups", "holzcloud-admins|redaktion-a")
	req.Header.Set(ProxySecretHeader, testProxySecret)

	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

	if !p.found || p.identity == nil {
		t.Fatal("a trusted peer with the right secret must yield an identity")
	}
	if p.identity.Username != "ada" {
		t.Errorf("Username = %q; want ada", p.identity.Username)
	}
	if p.identity.Email != "ada@example.com" {
		t.Errorf("Email = %q; want ada@example.com", p.identity.Email)
	}
	if p.identity.Name != "Ada Lovelace" {
		t.Errorf("Name = %q; want Ada Lovelace", p.identity.Name)
	}
	if !p.identity.HasGroup("holzcloud-admins") || !p.identity.HasGroup("redaktion-a") {
		t.Errorf("Groups = %v; want both groups", p.identity.Groups)
	}
	assertNoIdentityHeaderSurvives(t, p.header)
}

func TestForwardAuthDisbelievesAnUntrustedPeer(t *testing.T) {
	for _, tc := range []struct{ name, remote string }{
		{"over IPv4", "203.0.113.7:54321"},
		{"over IPv6", "[2001:db8::1]:54321"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := requestCarryingEverySpelling(tc.remote)
			req.Header.Set("X-authentik-username", "stranger")
			req.Header.Set(ProxySecretHeader, testProxySecret)

			p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

			if p.found || p.identity != nil {
				t.Errorf("an untrusted peer was believed: %+v", p.identity)
			}
			assertNoIdentityHeaderSurvives(t, p.header)
		})
	}
}

func TestForwardAuthLetsAnUntrustedPeerThroughUnharmed(t *testing.T) {
	// The way back in must not die with the proxy: no identity header, an
	// untrusted peer, and the request is served exactly as it always was.
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	p := &forwardAuthProbe{}
	rec := httptest.NewRecorder()
	ForwardAuth(loopbackResolver(t), ForwardAuthOptions{Enabled: true, Secret: testProxySecret})(p.handler()).ServeHTTP(rec, req)

	if p.calls != 1 {
		t.Fatalf("next ran %d times; want exactly 1", p.calls)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status %d; the middleware must never write a status of its own", rec.Code)
	}
	if p.found {
		t.Error("an anonymous request must carry no identity")
	}
}

func TestForwardAuthRefusesTheWrongSecret(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret string
		set    bool
	}{
		{"a trusted peer with the wrong secret", "not-the-secret", true},
		{"a trusted peer with no secret header at all", "", false},
		{"a trusted peer with an empty secret header", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := requestCarryingEverySpelling("127.0.0.1:41234")
			req.Header.Set("X-authentik-username", "ada")
			if tc.set {
				req.Header.Set(ProxySecretHeader, tc.secret)
			}

			p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

			if p.found || p.identity != nil {
				t.Errorf("the secret did not have to match: %+v", p.identity)
			}
			assertNoIdentityHeaderSurvives(t, p.header)
		})
	}
}

func TestForwardAuthStripsWhileSwitchedOff(t *testing.T) {
	// The strip is unconditional and the trust is not.
	req := requestCarryingEverySpelling("127.0.0.1:41234")
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set(ProxySecretHeader, testProxySecret)

	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: false, Secret: testProxySecret})

	if p.found || p.identity != nil {
		t.Errorf("single sign-on is off and an identity was produced anyway: %+v", p.identity)
	}
	assertNoIdentityHeaderSurvives(t, p.header)
}

func TestForwardAuthNeverMatchesAnEmptyConfiguredSecret(t *testing.T) {
	for _, tc := range []struct {
		name string
		fill func(*http.Request)
	}{
		{"no secret header", func(r *http.Request) {}},
		{"an empty secret header", func(r *http.Request) { r.Header.Set(ProxySecretHeader, "") }},
		{"any secret header", func(r *http.Request) { r.Header.Set(ProxySecretHeader, "anything") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := requestCarryingEverySpelling("127.0.0.1:41234")
			req.Header.Set("X-authentik-username", "ada")
			tc.fill(req)

			p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: ""})

			if p.found || p.identity != nil {
				t.Errorf("an empty configured secret was matchable: %+v", p.identity)
			}
			assertNoIdentityHeaderSurvives(t, p.header)
		})
	}
}

func TestForwardAuthNeedsAUsername(t *testing.T) {
	// The one field everything downstream keys on must be present; an identity
	// with an empty username is not an identity.
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set("X-authentik-email", "ada@example.com")
	req.Header.Set(ProxySecretHeader, testProxySecret)

	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

	if p.found || p.identity != nil {
		t.Errorf("an identity with no username was accepted: %+v", p.identity)
	}
}

func TestForwardAuthStripsTheProxySecretItself(t *testing.T) {
	req := requestCarryingEverySpelling("127.0.0.1:41234")
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set(ProxySecretHeader, testProxySecret)
	req.Header["x-holzcloud-proxy-secret"] = []string{testProxySecret}

	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

	assertNoIdentityHeaderSurvives(t, p.header)
	if got := p.header.Get(ProxySecretHeader); got != "" {
		t.Errorf("the handler can read the shared secret: %q", got)
	}
}

func TestForwardAuthLeavesEveryOtherHeaderAlone(t *testing.T) {
	req := requestCarryingEverySpelling("127.0.0.1:41234")
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set(ProxySecretHeader, testProxySecret)
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req.Header.Set("X-Request-ID", "abc123")
	req.Header.Set("Authorization", "Bearer something")

	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

	for _, name := range []string{"X-Forwarded-For", "X-Request-ID", "Authorization"} {
		if p.header.Get(name) == "" {
			t.Errorf("%s was deleted; the scan must only take identity headers", name)
		}
	}
}

func TestSplitGroups(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{"three groups", "a|b|c", []string{"a", "b", "c"}},
		{"an empty element is dropped", "a||b", []string{"a", "b"}},
		{"an empty header is no groups, not one empty group", "", nil},
		{"only separators is no groups", "||", nil},
		{"spaces are trimmed", " a | b ", []string{"a", "b"}},
		{"one group", "holzcloud-admins", []string{"holzcloud-admins"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := splitGroups(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("splitGroups(%q) = %#v; want %#v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("splitGroups(%q) = %#v; want %#v", tc.raw, got, tc.want)
				}
			}
		})
	}
}

func TestHasGroupComparesWholeElements(t *testing.T) {
	ident := Identity{Groups: splitGroups("not-holzcloud-admins|redaktion-a")}
	if ident.HasGroup("holzcloud-admins") {
		t.Error("not-holzcloud-admins was accepted as holzcloud-admins")
	}
	if !ident.HasGroup("redaktion-a") {
		t.Error("redaktion-a is a member and was not found")
	}
	if (Identity{}).HasGroup("") {
		t.Error("an identity with no groups is in no group, not in the empty one")
	}
}

func TestIdentityFromContextOnAContextThatNeverSawTheMiddleware(t *testing.T) {
	ident, ok := IdentityFromContext(context.Background())
	if ok || ident != nil {
		t.Errorf("IdentityFromContext(background) = %v, %v; want nil, false", ident, ok)
	}
}

// TestForwardAuthChecksThePeerBeforeItReadsAHeader is the ordering gate. It
// reads the source rather than the behaviour on purpose: no observable answer
// distinguishes "the peer was checked first" from "the peer was checked after
// the header was read", because both refuse. The property is a source-order
// property, so the test asserts source order — and it fails if the check moves
// to the right of a header read.
func TestForwardAuthChecksThePeerBeforeItReadsAHeader(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "forwardauth.go", nil, 0)
	if err != nil {
		t.Fatalf("parse forwardauth.go: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "ForwardAuth" && d.Recv == nil {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("no func ForwardAuth in forwardauth.go")
	}

	// The guard must be a single short-circuiting && chain.
	var guard *ast.IfStmt
	ast.Inspect(fn, func(n ast.Node) bool {
		if guard != nil {
			return false
		}
		if stmt, ok := n.(*ast.IfStmt); ok {
			if _, isAnd := stmt.Cond.(*ast.BinaryExpr); isAnd {
				guard = stmt
			}
		}
		return true
	})
	if guard == nil {
		t.Fatal("ForwardAuth has no if statement whose condition is one expression; the ordering cannot be short-circuiting")
	}

	operands := flattenAnd(guard.Cond)
	trust, secret := -1, -1
	for i, op := range operands {
		if exprMentions(op, "IsTrustedPeer") {
			trust = i
		}
		if exprMentions(op, "secretMatches") {
			secret = i
		}
	}
	if trust < 0 {
		t.Fatal("the guard does not call IsTrustedPeer")
	}
	if secret < 0 {
		t.Fatal("the guard does not call secretMatches")
	}
	if secret <= trust {
		t.Errorf("the secret is compared at operand %d and the peer at %d; the peer must be checked first, "+
			"because && short-circuits left to right and a secret comparison reads a header", secret, trust)
	}

	// No header may be read anywhere in ForwardAuth before the peer check.
	trustPos := operands[trust].Pos()
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Header" {
			return true
		}
		if sel.Pos() < trustPos {
			t.Errorf("a header is read at %s, before the peer is checked at %s",
				fset.Position(sel.Pos()), fset.Position(trustPos))
		}
		return true
	})
}

func flattenAnd(e ast.Expr) []ast.Expr {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.LAND {
		return []ast.Expr{e}
	}
	return append(flattenAnd(bin.X), flattenAnd(bin.Y)...)
}

func exprMentions(e ast.Expr, name string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}
