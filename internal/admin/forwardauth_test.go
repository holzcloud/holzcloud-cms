package admin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The shared secret the tests below hand the proxy layer. It is only ever used
// to get an identity into the request context the way production does; nothing
// in ForwardAuthSignIn reads it.
const fwdSecret = "the-real-secret"

// fwdTrusted trusts loopback and nothing else, which is what the request
// builder below sets RemoteAddr to.
func fwdTrusted() *web.ClientIPResolver {
	return web.NewClientIPResolver([]netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("::1/128"),
	})
}

// newForwardAuthAdmin builds the package's ordinary test handler over a real
// migrated database and gives it the three things forward authentication needs
// and newTestAdmin leaves out: a client-address resolver, a protocol store, and
// single sign-on switched on in its configuration.
func newForwardAuthAdmin(t *testing.T, ssoEnabled bool) (*Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	h, sm, database, _ := newTestAdmin(t)
	h.cfg.SSOEnabled = ssoEnabled
	h.cfg.SSOSecret = fwdSecret
	h.clientIP = fwdTrusted()
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database
}

// probeResult is what the handler on the far side of the middleware saw.
type probeResult struct {
	runs   int
	userID int64
	role   string
	email  string
	viaSSO bool
	status int
}

// serveForwardAuth drives one request through the production shape:
// web.ForwardAuth puts the identity into the context, the session manager loads
// the session, and ForwardAuthSignIn runs between them.
//
// proxyBelieves is separate from the handler's own cfg.SSOEnabled on purpose.
// In production both come from the same setting, so an identity can never be in
// the context while the sign-in middleware is switched off — which is precisely
// why the switch inside ForwardAuthSignIn would be untested if the two were
// tied together here. Setting them independently is the only way to prove that
// the middleware's own first condition does the work.
func serveForwardAuth(t *testing.T, h *Handler, sm *scs.SessionManager, proxyBelieves bool, req *http.Request) (*httptest.ResponseRecorder, *probeResult) {
	t.Helper()
	res := &probeResult{}
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.runs++
		res.userID = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		res.role = sm.GetString(r.Context(), auth.SessionKeyUserRole)
		res.email = sm.GetString(r.Context(), auth.SessionKeyUserEmail)
		res.viaSSO = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
	})
	chain := web.ForwardAuth(fwdTrusted(), web.ForwardAuthOptions{
		Enabled: proxyBelieves, Secret: fwdSecret,
	})(sm.LoadAndSave(h.ForwardAuthSignIn(probe)))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	res.status = rec.Code
	return rec, res
}

// fwdRequest builds a request that arrives from a trusted peer carrying the
// right secret, so web.ForwardAuth believes it. username is the identity;
// email is the account key, and the two are deliberately different things.
func fwdRequest(username, email string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.Host = "admin.test"
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set(web.ProxySecretHeader, fwdSecret)
	if username != "" {
		req.Header.Set("X-authentik-username", username)
	}
	if email != "" {
		req.Header.Set("X-authentik-email", email)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// seedAccount inserts a user and returns its id.
func seedAccount(t *testing.T, database *db.DB, email, role string) int64 {
	t.Helper()
	res, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', $1, 'x', $2)`, email, role)
	if err != nil {
		t.Fatalf("insert user %q: %v", email, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// activityRows returns every protocol row, oldest first.
func activityRows(t *testing.T, database *db.DB) []activity.Entry {
	t.Helper()
	rows, err := database.Read.QueryContext(context.Background(),
		`SELECT COALESCE(user_id, 0), actor_email, action, entity_id FROM activity_log ORDER BY id`)
	if err != nil {
		t.Fatalf("read activity_log: %v", err)
	}
	defer rows.Close()

	var out []activity.Entry
	for rows.Next() {
		var e activity.Entry
		var uid int64
		if err := rows.Scan(&uid, &e.ActorEmail, &e.Action, &e.EntityID); err != nil {
			t.Fatal(err)
		}
		e.UserID = &uid
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func countUsers(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func lastLoginAt(t *testing.T, database *db.DB, id int64) string {
	t.Helper()
	var at sql.NullString
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT last_login_at FROM users WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at.String
}

func sessionCookie(t *testing.T, sm *scs.SessionManager, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sm.Cookie.Name {
			return c
		}
	}
	return nil
}

// establishSession creates a session carrying one marker value and returns its
// cookie, so a later request has a token that can be compared with the one it
// leaves with.
func establishSession(t *testing.T, sm *scs.SessionManager) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "forward_auth_test_marker", "before")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	c := sessionCookie(t, sm, rec)
	if c == nil {
		t.Fatal("no session cookie issued for the pre-existing session")
	}
	return c
}

func TestForwardAuthSignsInAnExistingAccount(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	if res.runs != 1 {
		t.Fatalf("the next handler ran %d times; want exactly 1 on every path", res.runs)
	}
	if res.userID != id {
		t.Errorf("session user_id = %d; want %d", res.userID, id)
	}
	if res.role != user.RoleEditor {
		t.Errorf("session user_role = %q; want %q", res.role, user.RoleEditor)
	}
	if res.email != "ada@example.com" {
		t.Errorf("session user_email = %q; want the address as the database stores it", res.email)
	}
	if !res.viaSSO {
		t.Error("via_sso is false after a forward-auth sign-in; plan 10-06 has no input to read")
	}
}

func TestForwardAuthUsesTheStoredSpellingOfTheAddress(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "  ADA@Example.COM  ", nil))

	if res.userID != id {
		t.Fatalf("ADA@Example.COM did not sign in ada@example.com: user_id = %d", res.userID)
	}
	if res.email != "ada@example.com" {
		t.Errorf("session user_email = %q; the header only claimed an address, the database names the account", res.email)
	}
}

func TestForwardAuthRotatesTheSessionToken(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	seedAccount(t, database, "ada@example.com", user.RoleEditor)

	before := establishSession(t, sm)
	rec, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", before))

	after := sessionCookie(t, sm, rec)
	if after == nil {
		t.Fatal("no session cookie was written across the sign-in")
	}
	if after.Value == before.Value {
		t.Errorf("the session token did not change across the sign-in (%q); "+
			"whoever fixed the id before the sign-in still owns the session after it", before.Value)
	}
	if res.userID == 0 {
		t.Error("the sign-in did not happen at all, so the token comparison proves nothing")
	}
}

func TestForwardAuthWritesTheSameActivityRowAPasswordSignInWrites(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	if before := lastLoginAt(t, database, id); before != "" {
		t.Fatalf("last_login_at was already set to %q before the sign-in", before)
	}

	serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	rows := activityRows(t, database)
	if len(rows) != 1 {
		t.Fatalf("wrote %d protocol rows; want exactly 1", len(rows))
	}
	if rows[0].Action != activity.ActionAuthLoginSuccess {
		t.Errorf("action = %q; want %q", rows[0].Action, activity.ActionAuthLoginSuccess)
	}
	if rows[0].ActorEmail != "ada@example.com" {
		t.Errorf("actor_email = %q; want the stored address", rows[0].ActorEmail)
	}
	if rows[0].UserID == nil || *rows[0].UserID != id {
		t.Errorf("user_id = %v; want %d — a sign-in nobody can attribute is not in the protocol", rows[0].UserID, id)
	}
	if lastLoginAt(t, database, id) == "" {
		t.Error("last_login_at is still empty; a forgotten account stays invisible in the user list")
	}
}

func TestForwardAuthWithoutAnIdentityChangesNothing(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	seedAccount(t, database, "ada@example.com", user.RoleEditor)

	// No username header at all, so web.ForwardAuth builds no identity.
	rec, res := serveForwardAuth(t, h, sm, true, fwdRequest("", "", nil))

	if res.runs != 1 {
		t.Fatalf("the next handler ran %d times; want exactly 1", res.runs)
	}
	if res.userID != 0 {
		t.Errorf("a request carrying no identity was signed in as %d", res.userID)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("the middleware wrote status %d; it writes none of its own on any path", rec.Code)
	}
	if rows := activityRows(t, database); len(rows) != 0 {
		t.Errorf("wrote %d protocol rows for a request with no identity; want 0", len(rows))
	}
}

func TestForwardAuthNeverOverwritesASignedInSession(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	mine := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	theirs := seedAccount(t, database, "grace@example.com", user.RoleAdmin)

	// A session reached by password, established the way completeLogin does.
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.completeLogin(r, mine, user.RoleEditor, "ada@example.com")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	cookie := sessionCookie(t, sm, rec)
	if cookie == nil {
		t.Fatal("no session cookie for the password sign-in")
	}
	before := activityRows(t, database)

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("grace", "grace@example.com", cookie))

	if res.userID != mine {
		t.Errorf("the signed-in session was replaced: user_id = %d, want %d (%d is the identity the proxy asserted)",
			res.userID, mine, theirs)
	}
	if res.email != "ada@example.com" {
		t.Errorf("session user_email = %q; the existing session must be left exactly as it is", res.email)
	}
	if res.viaSSO {
		t.Error("a session reached by password was marked as established through single sign-on")
	}
	if after := activityRows(t, database); len(after) != len(before) {
		t.Errorf("wrote %d further protocol rows for an already signed-in request; "+
			"one row per page view is what makes the protocol useless", len(after)-len(before))
	}
}

func TestForwardAuthRefusesAndFallsThrough(t *testing.T) {
	for _, tc := range []struct {
		name             string
		username, email  string
		seed             string
		wantAttemptEmail string
	}{
		{
			name: "an address no account carries",
			// The fall-through this phase is built around: no account is
			// created here, and no 403 is answered.
			username: "stranger", email: "stranger@example.com",
			wantAttemptEmail: "stranger@example.com",
		},
		{
			name: "an address carrying a byte above 0x7F, even though a row would match it",
			// The account exists and Go's ToLower would find it. SQLite's
			// COLLATE NOCASE would not, so the two disagree about how many
			// accounts this address names — and a guess here is a guess about
			// who an administrator is.
			username: "mueller", email: "Müller@example.com",
			seed:             "müller@example.com",
			wantAttemptEmail: "müller@example.com",
		},
		{
			name:             "an identity with no address at all",
			username:         "ada",
			wantAttemptEmail: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database := newForwardAuthAdmin(t, true)
			if tc.seed != "" {
				seedAccount(t, database, tc.seed, user.RoleAdmin)
			}
			usersBefore := countUsers(t, database)

			rec, res := serveForwardAuth(t, h, sm, true, fwdRequest(tc.username, tc.email, nil))

			if res.runs != 1 {
				t.Fatalf("the next handler ran %d times; want exactly 1 — a refusal is a fall-through", res.runs)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("the middleware answered %d; a refusal is never a status of its own, "+
					"because the way back in must not die with the proxy", rec.Code)
			}
			if res.userID != 0 {
				t.Errorf("a refused identity was signed in as %d", res.userID)
			}
			if got := countUsers(t, database); got != usersBefore {
				t.Errorf("the users table grew from %d to %d; this plan creates no account", usersBefore, got)
			}

			rows := activityRows(t, database)
			if len(rows) != 1 {
				t.Fatalf("wrote %d protocol rows; want exactly 1 refusal", len(rows))
			}
			if rows[0].Action != activity.ActionAuthLoginFail {
				t.Errorf("action = %q; want %q", rows[0].Action, activity.ActionAuthLoginFail)
			}
			if rows[0].ActorEmail != tc.wantAttemptEmail {
				t.Errorf("actor_email = %q; want the attempted address %q", rows[0].ActorEmail, tc.wantAttemptEmail)
			}
		})
	}
}

func TestForwardAuthDoesNothingWhileSwitchedOff(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, false)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	// The proxy layer still believes, so an identity really is in the context.
	// Only cfg.SSOEnabled is false, which is the condition under test.
	rec, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	if res.runs != 1 {
		t.Fatalf("the next handler ran %d times; want exactly 1", res.runs)
	}
	if res.userID != 0 {
		t.Errorf("single sign-on is off and somebody was signed in anyway: user_id = %d", res.userID)
	}
	if res.viaSSO {
		t.Error("single sign-on is off and via_sso was written anyway")
	}
	if rows := activityRows(t, database); len(rows) != 0 {
		t.Errorf("single sign-on is off and %d protocol rows were written; want 0", len(rows))
	}
	if rec.Code != http.StatusOK {
		t.Errorf("the middleware wrote status %d with the switch off", rec.Code)
	}
	if lastLoginAt(t, database, id) != "" {
		t.Error("single sign-on is off and the account was still looked up and stamped")
	}
}

func TestPasswordSignInCarriesNoSSOMark(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	var viaSSO bool
	var userID int64
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.completeLogin(r, id, user.RoleEditor, "ada@example.com")
		viaSSO = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
		userID = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if userID != id {
		t.Fatalf("the password funnel did not sign in: user_id = %d", userID)
	}
	if viaSSO {
		t.Error("a password sign-in carries via_sso; the mark must be written by the forward-auth path alone")
	}
}

func TestIsASCII(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"ada@example.com", true},
		{"", true},
		{"müller@example.com", false},
		{"ada@exämple.com", false},
		{"ada+tag@example.com", true},
		{"@example.com", true},
		{"@example.com", false},
	} {
		if got := isASCII(tc.in); got != tc.want {
			t.Errorf("isASCII(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// The two properties below are not observable from outside the program, so they
// are asserted against the source itself — the same shape, and for the same
// reason, as the ordering gate wave 2 put on internal/web/forwardauth.go.
//
// Session fixation is the case in point. scs's RenewToken preserves the values
// already in the session, so a middleware that rotated the token *after*
// writing user_id would answer every request identically to one that rotated it
// before: same status, same session contents, and a token that changed either
// way. Every behavioural test in this file stays green under that reordering,
// and the property — that a token fixed before the sign-in is not the token
// after it — is gone. A test that reads the source is the only kind that fails.

// forwardAuthSignInBody parses this package and returns the function under
// test together with the file set positions belong to.
func forwardAuthSignInBody(t *testing.T) (*ast.FuncDecl, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "forwardauth.go", nil, 0)
	if err != nil {
		t.Fatalf("parse forwardauth.go: %v", err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "ForwardAuthSignIn" {
			return fn, fset
		}
	}
	t.Fatal("ForwardAuthSignIn not found in forwardauth.go")
	return nil, nil
}

func TestForwardAuthRotatesTheTokenBeforeItWritesAnything(t *testing.T) {
	fn, fset := forwardAuthSignInBody(t)

	var renew []token.Pos
	var writes []struct {
		name string
		pos  token.Pos
	}
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "RenewToken":
			renew = append(renew, sel.Sel.Pos())
		case "Put", "completeLogin":
			writes = append(writes, struct {
				name string
				pos  token.Pos
			}{sel.Sel.Name, sel.Sel.Pos()})
		}
		return true
	})

	if len(renew) != 1 {
		t.Fatalf("RenewToken is called %d times in ForwardAuthSignIn; want exactly 1", len(renew))
	}
	if len(writes) == 0 {
		t.Fatal("nothing is written to the session at all; the sign-in cannot be happening")
	}
	for _, w := range writes {
		if w.pos < renew[0] {
			t.Errorf("%s is called at %s, before RenewToken at %s; "+
				"the token must be rotated before anything goes into the session, or whoever fixed "+
				"the session id before the sign-in still owns it after",
				w.name, fset.Position(w.pos), fset.Position(renew[0]))
		}
	}
}

func TestCompleteLoginHasExactlyFourCallers(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	var callers []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "completeLogin" {
				callers = append(callers, fset.Position(sel.Sel.Pos()).String())
			}
			return true
		})
	}

	// The password form, the second factor, first-run setup, and forward auth.
	// A three means a path put user_id into the session itself and its sign-in
	// will not appear in /admin/protokoll; a five means a fifth funnel was
	// opened, and the two ends of it will drift.
	if len(callers) != 4 {
		t.Errorf("completeLogin has %d call sites; want exactly 4\n  %s",
			len(callers), strings.Join(callers, "\n  "))
	}
}

// TestForwardAuthNeverMatchesAnEmptyAddress is what makes the empty-address
// refusal load-bearing rather than tidy.
//
// users.email is declared NOT NULL UNIQUE COLLATE NOCASE and nothing in that
// declaration forbids the empty string, so an account row carrying one is legal
// and a command-line import or a hand-run statement can produce it. Without the
// guard the empty address is not refused, it is *looked up* — and it matches
// that row. Any identity the proxy asserts without an e-mail header would then
// sign in as whoever holds it, which in the seeding below is an administrator.
//
// The other refusals in this file are all observably identical to one another
// by design; this is the one that is not, and it is the reason the branch
// exists rather than falling through to "no such account".
func TestForwardAuthNeverMatchesAnEmptyAddress(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	if _, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', '', 'x', 'admin')`); err != nil {
		t.Fatalf("seed the empty-address account: %v", err)
	}

	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "", nil))

	if res.userID != 0 {
		t.Errorf("an identity with no address signed in as user %d (role %q); "+
			"the empty string is a legal value in users.email and must never be looked up",
			res.userID, res.role)
	}
	if res.viaSSO {
		t.Error("an identity with no address was marked as signed in through single sign-on")
	}
}

// ---------------------------------------------------------------------------
// Provisioning (plan 10-04)
//
// Everything below is about one seam. NewWebsiteAccessLookup in handler.go ends
// with `return assigned == 0 || mine > 0` — no assignment means every website —
// and a freshly provisioned account has zero rows in user_websites *by
// construction*. The tests therefore never count rows in user_websites as their
// main assertion; they ask that function.
// ---------------------------------------------------------------------------

// newProvisioningAdmin is newForwardAuthAdmin plus the three things provisioning
// needs and cannot be tested without.
//
// Affordable hashing first: user.Store.Create really derives an Argon2id hash,
// and the package's ordinary fixture hands the store the zero Argon2Params,
// which panic rather than hash. cheapHashing is the same fixture menu_scope_test
// uses and for the same reason.
//
// Then two websites, created here rather than reusing the one newTestAdmin
// seeds, so that both have a name a failure message can print: the default one a
// provisioned account is given, and the second one that exists only to be
// refused.
func newProvisioningAdmin(t *testing.T, provision bool) (h *Handler, sm *scs.SessionManager, database *db.DB, defaultWebsite, otherWebsite int64) {
	t.Helper()
	h, sm, database = newForwardAuthAdmin(t, true)
	h.users.Params = cheapHashing

	domains := domain.NewStore(database)
	home, err := domains.CreateWebsite(context.Background(), "Die eigene Seite", "")
	if err != nil {
		t.Fatalf("create the default website: %v", err)
	}
	other, err := domains.CreateWebsite(context.Background(), "Die fremde Seite", "")
	if err != nil {
		t.Fatalf("create the second website: %v", err)
	}

	h.cfg.SSOProvision = provision
	h.cfg.SSODefaultWebsite = home.ID
	return h, sm, database, home.ID, other.ID
}

// accountByEmail reads the row a provisioning run is supposed to have written.
func accountByEmail(t *testing.T, database *db.DB, email string) (id int64, name, role, password string) {
	t.Helper()
	err := database.Read.QueryRowContext(context.Background(),
		`SELECT id, name, role, password FROM users WHERE email = $1`, email).Scan(&id, &name, &role, &password)
	if err != nil {
		t.Fatalf("no account for %q: %v", email, err)
	}
	return id, name, role, password
}

// assignedWebsites is used for description, never as the load-bearing
// assertion. What a request may reach is NewWebsiteAccessLookup's answer, and
// the two are different claims.
func assignedWebsites(t *testing.T, database *db.DB, userID int64) []int64 {
	t.Helper()
	rows, err := database.Read.QueryContext(context.Background(),
		`SELECT website_id FROM user_websites WHERE user_id = $1 ORDER BY website_id`, userID)
	if err != nil {
		t.Fatalf("read user_websites: %v", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// inSession runs fn inside a live session.
//
// Anything that reaches h.LogActivity needs one: the protocol reads the acting
// account out of the session, and scs panics when the context carries none. The
// tests that call provisionSSOUser directly go through here so that they fail
// for the reason under test rather than in the session manager.
func inSession(t *testing.T, sm *scs.SessionManager, fn func(r *http.Request)) {
	t.Helper()
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(r)
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/admin/", nil))
}

func TestForwardAuthProvisionsAnAccount(t *testing.T) {
	h, sm, database, defaultWebsite, otherWebsite := newProvisioningAdmin(t, true)
	before := countUsers(t, database)

	cookie := establishSession(t, sm)
	req := fwdRequest("ada", "ada@example.com", cookie)
	req.Header.Set("X-authentik-name", "Ada Lovelace")
	rec, res := serveForwardAuth(t, h, sm, true, req)

	id, name, role, password := accountByEmail(t, database, "ada@example.com")

	t.Run("one account was created, and exactly one", func(t *testing.T) {
		if got := countUsers(t, database); got != before+1 {
			t.Fatalf("the users table went from %d to %d; provisioning writes exactly one row", before, got)
		}
	})

	t.Run("the account is an editor and never an administrator", func(t *testing.T) {
		if role != user.RoleEditor {
			t.Errorf("role = %q; want %q — what a group grants is plan 10-05's decision, and "+
				"NewWebsiteAccessLookup returns true for an administrator before it counts any assignment",
				role, user.RoleEditor)
		}
	})

	t.Run("the name is the identity's own", func(t *testing.T) {
		if name != "Ada Lovelace" {
			t.Errorf("name = %q; want the name the identity carried", name)
		}
	})

	t.Run("the account belongs to the default website and to nothing else", func(t *testing.T) {
		got := assignedWebsites(t, database, id)
		if len(got) != 1 || got[0] != defaultWebsite {
			t.Errorf("user_websites = %v; want exactly [%d]. An empty list is the inversion itself",
				got, defaultWebsite)
		}
	})

	t.Run("the request that created it is the request that signs it in", func(t *testing.T) {
		if res.runs != 1 {
			t.Fatalf("the next handler ran %d times; want exactly 1", res.runs)
		}
		if res.userID != id {
			t.Errorf("session user_id = %d; want the account just created (%d)", res.userID, id)
		}
		if res.role != user.RoleEditor {
			t.Errorf("session user_role = %q; want %q", res.role, user.RoleEditor)
		}
		if res.email != "ada@example.com" {
			t.Errorf("session user_email = %q; want the stored address", res.email)
		}
		if !res.viaSSO {
			t.Error("via_sso is false after a provisioning sign-in")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("the middleware answered %d; it writes no status of its own on any path", rec.Code)
		}
	})

	t.Run("the session token was rotated across the sign-in", func(t *testing.T) {
		after := sessionCookie(t, sm, rec)
		if after == nil {
			t.Fatal("no session cookie was written across the sign-in")
		}
		if after.Value == cookie.Value {
			t.Error("the session token did not change; whoever fixed the id before the sign-in still owns it after")
		}
	})

	t.Run("the creation and the sign-in are both in the protocol", func(t *testing.T) {
		rows := activityRows(t, database)
		var created, signedIn int
		for _, row := range rows {
			switch row.Action {
			case activity.ActionUserCreate:
				created++
				if row.EntityID != id {
					t.Errorf("user.create names entity %d; want the new account %d", row.EntityID, id)
				}
			case activity.ActionAuthLoginSuccess:
				signedIn++
			case activity.ActionAuthLoginFail:
				t.Error("a successful provisioning wrote a refusal row as well")
			}
		}
		if created != 1 {
			t.Errorf("wrote %d user.create rows; an account created without a person asking belongs in the journal exactly once", created)
		}
		if signedIn != 1 {
			t.Errorf("wrote %d auth.login_success rows; want exactly 1", signedIn)
		}
	})

	t.Run("the second website was not granted along the way", func(t *testing.T) {
		if got := assignedWebsites(t, database, id); len(got) == 2 {
			t.Errorf("user_websites = %v; the second website (%d) exists only to be refused", got, otherWebsite)
		}
	})

	t.Run("the stored password is a real Argon2id hash and not the empty string", func(t *testing.T) {
		if !strings.HasPrefix(password, "$argon2id$") {
			t.Errorf("users.password = %q; want a PHC-format Argon2id hash", password)
		}
	})
}

func TestProvisionedAccountHasNoNameWhenTheHeaderCarriedNone(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)

	// fwdRequest sets no X-authentik-name, which is the case under test: the
	// operator's directory need not carry one.
	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	_, name, _, _ := accountByEmail(t, database, "ada@example.com")
	if name != "" {
		t.Errorf("name = %q; a nameless account is fine and a wrong name is not", name)
	}
	if res.userID == 0 {
		t.Error("the account was not signed in, so the name assertion proves nothing")
	}
}

// TestNoPasswordSignsInAProvisionedAccount drives the real login form.
//
// The account holds an Argon2id hash of 32 bytes from crypto/rand that nothing
// kept, so there is no password to get wrong — and the screen must behave for it
// exactly as it behaves for an address no account carries.
func TestNoPasswordSignsInAProvisionedAccount(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)
	h.argon2Params = cheapHashing
	h.loginThrottle = auth.NewLoginThrottle(1000, 1000, time.Minute)

	serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	if _, _, _, hash := accountByEmail(t, database, "ada@example.com"); hash == "" {
		t.Fatal("no account was provisioned, so the password assertions prove nothing")
	}

	for _, tc := range []struct {
		name, email, password string
	}{
		{"the empty password", "ada@example.com", ""},
		{"a plausible password", "ada@example.com", "passwort123"},
		{"the address itself", "ada@example.com", "ada@example.com"},
		{"a control: an address no account carries", "nobody@example.com", "passwort123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{"email": {tc.email}, "password": {tc.password}}
			req := postForm("/admin/login", form, nil)
			req.RemoteAddr = "127.0.0.1:41234"

			var signedIn int64
			rec := httptest.NewRecorder()
			sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := h.HandleLogin(w, r); err != nil {
					t.Fatalf("HandleLogin: %v", err)
				}
				signedIn = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
			})).ServeHTTP(rec, req)

			if signedIn != 0 {
				t.Errorf("signed in as %d; nobody ever chose a password for a provisioned account", signedIn)
			}
			if got := rec.Header().Get("Location"); got != "/admin/login" {
				t.Errorf("Location = %q; want the login form back, exactly as an unknown account gets it", got)
			}
		})
	}
}

// TestProvisioningLeavesNoAccountWhenTheAssignmentFails is the compensation.
//
// An account that exists with no assignment is precisely the state D-01
// describes — NewWebsiteAccessLookup reads zero rows as every website — so a
// half-finished provisioning would create the vulnerability by accident rather
// than by design. The failure is induced the way a real one arrives: a default
// website id that names no row, which user_websites' foreign key refuses.
func TestProvisioningLeavesNoAccountWhenTheAssignmentFails(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)
	h.cfg.SSODefaultWebsite = 9999 // no such website
	before := countUsers(t, database)

	rec, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))

	if got := countUsers(t, database); got != before {
		t.Errorf("the users table went from %d to %d; a failed provisioning leaves nothing behind, "+
			"because what it would leave behind is an account with access to every website", before, got)
	}
	if res.userID != 0 {
		t.Errorf("a failed provisioning signed somebody in as %d", res.userID)
	}
	if res.runs != 1 {
		t.Fatalf("the next handler ran %d times; want exactly 1 — a refusal is a fall-through", res.runs)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("the middleware answered %d; never a 403, even here", rec.Code)
	}

	rows := activityRows(t, database)
	if len(rows) != 1 || rows[0].Action != activity.ActionAuthLoginFail {
		t.Errorf("protocol rows = %v; want exactly one auth.login_fail. An operator whose "+
			"HOLZCLOUD_SSO_DEFAULT_WEBSITE names a deleted website learns about it here", rows)
	}
}

// TestProvisioningResolvesADuplicateAddressByReReading is the race.
//
// Two requests for the same unknown identity both reach the creation, and the
// database is what resolves it: users.email is UNIQUE COLLATE NOCASE, so the
// second Create returns user.ErrDuplicateEmail. That is not an error to refuse
// on — the honest resolution is the row the first request made.
func TestProvisioningResolvesADuplicateAddressByReReading(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)

	serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	first, _, _, _ := accountByEmail(t, database, "ada@example.com")
	afterFirst := countUsers(t, database)

	var second *user.User
	var err error
	inSession(t, sm, func(r *http.Request) {
		second, err = h.provisionSSOUser(r, &web.Identity{Username: "ada", Email: "ada@example.com", Name: "Ada Lovelace"}, "ada@example.com")
	})

	if err != nil {
		t.Fatalf("the second provisioning refused instead of re-reading: %v", err)
	}
	if second == nil || second.ID != first {
		t.Fatalf("the second provisioning returned %v; want the row the first one created (%d)", second, first)
	}
	if got := countUsers(t, database); got != afterFirst {
		t.Errorf("the users table went from %d to %d; one identity is one account", afterFirst, got)
	}
}

// TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress is the constraint
// wave 3 discovered and handed forward.
//
// users.email is NOT NULL UNIQUE COLLATE NOCASE and nothing there forbids the
// empty string, so an account row carrying one is legal —
// TestForwardAuthNeverMatchesAnEmptyAddress seeds exactly such a row, as an
// administrator, because without the sign-in path's guard an identity with no
// e-mail header is not refused but *looked up* and matches it.
//
// Provisioning is the other end of the same rope. If it could create such a
// row, it would mint the very account that test seeds, and the next identity
// arriving with no e-mail header would sign in as it.
//
// The second half of this test is the part that matters, and it is not
// redundant. Driven through the middleware the empty address never reaches
// provisioning at all — step 4 refuses it first — so removing the guard inside
// provisionSSOUser leaves the middleware half green. Only the direct call can
// fail, which is the whole reason it is written.
func TestProvisioningNeverCreatesAnAccountWithAnEmptyAddress(t *testing.T) {
	t.Run("through the middleware, an identity with no address creates nothing", func(t *testing.T) {
		h, sm, database, _, _ := newProvisioningAdmin(t, true)
		before := countUsers(t, database)

		_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "", nil))

		if got := countUsers(t, database); got != before {
			t.Errorf("the users table went from %d to %d; an identity with no address creates no account", before, got)
		}
		if res.userID != 0 {
			t.Errorf("an identity with no address was signed in as %d", res.userID)
		}
	})

	t.Run("asked directly, provisioning refuses the empty address itself", func(t *testing.T) {
		h, sm, database, _, _ := newProvisioningAdmin(t, true)
		before := countUsers(t, database)

		var u *user.User
		var err error
		inSession(t, sm, func(r *http.Request) {
			u, err = h.provisionSSOUser(r, &web.Identity{Username: "ada"}, "")
		})

		if err == nil {
			t.Error("provisionSSOUser accepted the empty address; it would create the row " +
				"TestForwardAuthNeverMatchesAnEmptyAddress seeds, and the next identity with no " +
				"e-mail header would sign in as it")
		}
		// The sentinel and not merely "some error", because user.Store.Create
		// refuses the empty address too. Without this line, deleting
		// provisioning's own guard leaves this subtest green on the store's
		// validation error, and the guard reads as tidiness rather than as the
		// layer that states the reason.
		if !errors.Is(err, errSSOEmptyAddress) {
			t.Errorf("err = %v; want errSSOEmptyAddress. Provisioning must refuse the empty "+
				"address itself, not lean on user.Store.Create happening to validate it", err)
		}
		if u != nil {
			t.Errorf("provisionSSOUser returned account %d for the empty address", u.ID)
		}
		if got := countUsers(t, database); got != before {
			t.Errorf("the users table went from %d to %d for an empty address", before, got)
		}
	})
}

func TestRandomSecretIsFreshAndLongEnough(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		s, err := randomSecret()
		if err != nil {
			t.Fatalf("randomSecret: %v", err)
		}
		if len(s) < auth.MinPasswordLength {
			t.Fatalf("randomSecret returned %d characters; user.Store.Create requires at least %d",
				len(s), auth.MinPasswordLength)
		}
		if seen[s] {
			t.Fatalf("randomSecret repeated itself at call %d; every provisioned account gets its own", i)
		}
		seen[s] = true
	}
}

// TestTheProvisioningSecretAppearsInNoLogLine asks the question the way it
// matters rather than the way it is easy.
//
// A grep for slog on the lines that name the secret is a gate on the file. This
// runs a real provisioning with the log captured at Debug, then takes every
// token the log produced and asks the stored hash whether that token is the
// password. If the secret leaked into any field of any line, exactly one token
// verifies.
func TestTheProvisioningSecretAppearsInNoLogLine(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, true)

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	_, _, _, hash := accountByEmail(t, database, "ada@example.com")

	out := logged.String()
	if !strings.Contains(out, "provision") {
		t.Fatalf("the provisioning wrote no log line at all, so this test proves nothing; log was:\n%s", out)
	}
	for _, token := range strings.FieldsFunc(out, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == '"' || r == '='
	}) {
		if len(token) < auth.MinPasswordLength {
			continue
		}
		match, err := auth.VerifyPassword(token, hash)
		if err != nil {
			continue
		}
		if match {
			t.Fatalf("the provisioning secret was written to the log: %q", token)
		}
	}
}

// TestProvisionedAccountCannotReachASecondWebsite is the test this plan exists
// for, and its shape matters more than its assertions.
//
// NewWebsiteAccessLookup is the function auth.RequireWebsiteAccess is built from
// in main.go, so an assertion against it is an assertion about what a real
// request is allowed to reach. It ends with `return assigned == 0 || mine > 0`.
// A test that counted rows in user_websites would prove that a row was
// inserted; it would say nothing about whether the function that authorises
// requests agrees, and the two are different claims — de4a1ce in this repository
// is a cross-website hole of exactly that family, found on 2026-09-06, where
// four handlers checked that an item belonged to its menu and never that the
// menu belonged to the website.
func TestProvisionedAccountCannotReachASecondWebsite(t *testing.T) {
	h, sm, database, defaultWebsite, otherWebsite := newProvisioningAdmin(t, true)
	ctx := context.Background()

	// Through the whole path, not by calling provisionSSOUser: the point is
	// that an ordinary request produces this state.
	_, res := serveForwardAuth(t, h, sm, true, fwdRequest("ada", "ada@example.com", nil))
	newID, _, _, _ := accountByEmail(t, database, "ada@example.com")
	if res.userID != newID {
		t.Fatalf("the sign-in did not happen (session user_id = %d, account = %d); "+
			"the assertions below would be measuring an account nobody reached", res.userID, newID)
	}

	// The same constructor main.go hands to auth.RequireWebsiteAccess.
	lookup := NewWebsiteAccessLookup(database)

	if !lookup(ctx, newID, defaultWebsite) {
		t.Errorf("NewWebsiteAccessLookup refuses the provisioned account its own default website "+
			"(%q, id %d); it was assigned that website in the request that created it",
			"Die eigene Seite", defaultWebsite)
	}
	if lookup(ctx, newID, otherWebsite) {
		t.Errorf("a freshly provisioned account reached a website it was never assigned to "+
			"(%q, id %d) — NewWebsiteAccessLookup read zero assignments as every website",
			"Die fremde Seite", otherWebsite)
	}

	// The control, and it is neither decoration nor redundant. Without it this
	// test passes on a tree where provisioning never ran, where the assignment
	// was written against the wrong id, or where the lookup was stubbed — it
	// would be measuring a proxy. Deleting the one row and watching the same
	// call flip to true demonstrates that the property asserted above is the one
	// that would break, and that this single row is what holds it.
	//
	// Do not remove this as a tidy-up. It is the difference between a test that
	// asserts the fix and a test that asserts the fix was attempted.
	if _, err := database.Write.ExecContext(ctx,
		`DELETE FROM user_websites WHERE user_id = $1`, newID); err != nil {
		t.Fatalf("delete the assignment for the control step: %v", err)
	}
	if !lookup(ctx, newID, otherWebsite) {
		t.Fatal("with the assignment deleted the same lookup still refuses the second website; " +
			"the assertion above was not measuring the assignment at all")
	}
}

// TestProvisioningOffCreatesNothing states a claim about a code path not being
// taken as a claim about the database, because the code path can be moved and
// the database cannot lie.
func TestProvisioningOffCreatesNothing(t *testing.T) {
	h, sm, database, _, _ := newProvisioningAdmin(t, false)
	before := countUsers(t, database)

	rec, res := serveForwardAuth(t, h, sm, true, fwdRequest("stranger", "stranger@example.com", nil))

	if got := countUsers(t, database); got != before {
		t.Errorf("the users table went from %d to %d with HOLZCLOUD_SSO_PROVISION off", before, got)
	}
	if res.userID != 0 {
		t.Errorf("provisioning is off and somebody was signed in as %d", res.userID)
	}
	if res.runs != 1 {
		t.Fatalf("the next handler ran %d times; want exactly 1 — the refusal is the password form", res.runs)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("the middleware answered %d; a refusal is never a status of its own", rec.Code)
	}

	rows := activityRows(t, database)
	if len(rows) != 1 {
		t.Fatalf("wrote %d protocol rows; want exactly 1 refusal", len(rows))
	}
	if rows[0].Action != activity.ActionAuthLoginFail {
		t.Errorf("action = %q; want %q", rows[0].Action, activity.ActionAuthLoginFail)
	}
	if rows[0].ActorEmail != "stranger@example.com" {
		t.Errorf("actor_email = %q; want the attempted address", rows[0].ActorEmail)
	}
}

// ---------------------------------------------------------------------------
// Group synchronisation (plan 10-05)
//
// SSO-06: group membership decides role and website access, re-applied on every
// sign-in so that a demotion at the identity provider takes effect here rather
// than at the next session expiry.
//
// Four traps sit inside one function and three of them pass a naive
// implementation's own tests. The tests below are named after the traps rather
// than after the happy path, because the happy path is the part that would have
// been written correctly anyway.
// ---------------------------------------------------------------------------

const (
	fwdAdminGroup = "holzcloud-admins"
	fwdGroupA     = "seite-a"
	fwdGroupB     = "seite-b"
)

// newGroupSyncAdmin is newForwardAuthAdmin plus affordable hashing, two
// websites with names a failure message can print, and the two settings the
// synchronisation reads.
func newGroupSyncAdmin(t *testing.T) (h *Handler, sm *scs.SessionManager, database *db.DB, siteA, siteB int64) {
	t.Helper()
	h, sm, database = newForwardAuthAdmin(t, true)
	h.users.Params = cheapHashing

	domains := domain.NewStore(database)
	a, err := domains.CreateWebsite(context.Background(), "Seite A", "")
	if err != nil {
		t.Fatalf("create Seite A: %v", err)
	}
	b, err := domains.CreateWebsite(context.Background(), "Seite B", "")
	if err != nil {
		t.Fatalf("create Seite B: %v", err)
	}

	h.cfg.SSOAdminGroup = fwdAdminGroup
	h.cfg.SSOWebsiteGroups = map[string]int64{fwdGroupA: a.ID, fwdGroupB: b.ID}
	return h, sm, database, a.ID, b.ID
}

// fwdGroupRequest is fwdRequest carrying the identity provider's group header.
//
// The header is set even when groups is empty, because "the header arrived and
// said nothing" is one of the cases under test and it is not the same request
// as one where the proxy sent no header at all.
func fwdGroupRequest(username, email, groups string, cookie *http.Cookie) *http.Request {
	req := fwdRequest(username, email, cookie)
	req.Header.Set("X-authentik-groups", groups)
	return req
}

// assign writes a website assignment directly, so a test can set up the state a
// previous sign-in would have left.
func assign(t *testing.T, database *db.DB, userID int64, ids ...int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.Write.ExecContext(ctx,
		`DELETE FROM user_websites WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("clear the assignment: %v", err)
	}
	for _, id := range ids {
		if _, err := database.Write.ExecContext(ctx,
			`INSERT INTO user_websites (user_id, website_id) VALUES ($1, $2)`, userID, id); err != nil {
			t.Fatalf("assign website %d: %v", id, err)
		}
	}
}

// storedRole reads users.role, which is what the next request will be
// authorised against — auth.RequireAuth re-reads it from the database.
func storedRole(t *testing.T, database *db.DB, id int64) string {
	t.Helper()
	var role string
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT role FROM users WHERE id = $1`, id).Scan(&role); err != nil {
		t.Fatalf("read role of user %d: %v", id, err)
	}
	return role
}

func storedMayPublish(t *testing.T, database *db.DB, id int64) bool {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT may_publish FROM users WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("read may_publish of user %d: %v", id, err)
	}
	return n != 0
}

func sameIDList(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// countAction counts protocol rows carrying one action for one entity.
func countAction(t *testing.T, database *db.DB, action string, entityID int64) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM activity_log WHERE action = $1 AND entity_id = $2`,
		action, entityID).Scan(&n); err != nil {
		t.Fatalf("count %q rows: %v", action, err)
	}
	return n
}

// TestGroupSyncMatchesAGroupAsAWholeElement is the trap this file exists for.
//
// X-authentik-groups joins names with U+007C, and the obvious membership test —
// strings.Contains over the raw header — answers yes about holzcloud-admins for
// somebody whose only group is not-holzcloud-admins. Every row below whose name
// begins "a group that" is a row a Contains implementation gets wrong; the rows
// that only use a matching group prove nothing about the trap and are here to
// show the same table can express the correct answer too.
//
// Each case carries seite-a so the sign-in completes: an editor with no
// matching website group is refused, which is a different test.
func TestGroupSyncMatchesAGroupAsAWholeElement(t *testing.T) {
	cases := []struct {
		name       string
		adminGroup string
		groups     string
		want       string
	}{
		{"the configured group alone makes an administrator", fwdAdminGroup, fwdAdminGroup, user.RoleAdmin},
		{"the configured group among others makes an administrator", fwdAdminGroup, fwdGroupA + "|" + fwdAdminGroup + "|andere", user.RoleAdmin},
		{"a group the configured name is a suffix of is not the group", fwdAdminGroup, "not-" + fwdAdminGroup + "|" + fwdGroupA, user.RoleEditor},
		{"a group the configured name is a prefix of is not the group", fwdAdminGroup, fwdAdminGroup + "-x|" + fwdGroupA, user.RoleEditor},
		{"a group carrying the configured name inside it is not the group", fwdAdminGroup, "x" + fwdAdminGroup + "x|" + fwdGroupA, user.RoleEditor},
		{"a website group is not an administration group", fwdAdminGroup, fwdGroupA, user.RoleEditor},
		{"an empty configured group is matched by nobody", "", fwdAdminGroup + "|" + fwdGroupA, user.RoleEditor},
		{"an empty configured group is not matched by an empty header element", "", "|" + fwdGroupA + "|", user.RoleEditor},
		{"an empty configured group is not matched by a header of separators", "", fwdGroupA + "||", user.RoleEditor},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, siteA, _ := newGroupSyncAdmin(t)
			h.cfg.SSOAdminGroup = tc.adminGroup
			id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
			assign(t, database, id, siteA)

			_, res := serveForwardAuth(t, h, sm, true,
				fwdGroupRequest("ada", "ada@example.com", tc.groups, nil))

			if got := storedRole(t, database, id); got != tc.want {
				t.Errorf("users.role = %q; want %q — the header was %q and the configured "+
					"administration group was %q. A whole element is compared, not a substring: "+
					"strings.Contains over the raw header is how %q becomes an administrator",
					got, tc.want, tc.groups, tc.adminGroup, tc.groups)
			}
			if res.userID != id {
				t.Fatalf("the sign-in did not happen (session user_id = %d, account %d); "+
					"the role above was not the one that reached the session", res.userID, id)
			}
			if res.role != tc.want {
				t.Errorf("session user_role = %q; want %q — the synchronised role has to be the "+
					"one handed to completeLogin, not the stale one read before it", res.role, tc.want)
			}
		})
	}
}

// TestGroupSyncAppliesADemotionAtTheNextSignIn is the whole point of running the
// synchronisation on every sign-in rather than at account creation.
func TestGroupSyncAppliesADemotionAtTheNextSignIn(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	// A second administrator, so the demotion below is not the last one.
	seedAccount(t, database, "root@example.com", user.RoleAdmin)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)
	assign(t, database, id, siteA)

	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))

	if got := storedRole(t, database, id); got != user.RoleEditor {
		t.Errorf("users.role = %q; want %q — the account is no longer in %q at the identity "+
			"provider and a demotion there has to take effect here at the next sign-in",
			got, user.RoleEditor, fwdAdminGroup)
	}
	if res.role != user.RoleEditor {
		t.Errorf("session user_role = %q; want %q", res.role, user.RoleEditor)
	}
	if n := countAction(t, database, activity.ActionUserUpdate, id); n != 1 {
		t.Errorf("wrote %d %q rows for the demotion; want exactly 1 naming what changed",
			n, activity.ActionUserUpdate)
	}
}

// TestGroupSyncPromotionWritesOneProtocolRowNamingFromAndTo asserts the record,
// not just the change. SSO-06 asks for every change of rights to be in the
// protocol, and a change nobody can see afterwards is a change nobody can
// question.
func TestGroupSyncPromotionWritesOneProtocolRowNamingFromAndTo(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteA)

	serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdAdminGroup, nil))

	if got := storedRole(t, database, id); got != user.RoleAdmin {
		t.Fatalf("users.role = %q; want %q", got, user.RoleAdmin)
	}

	var meta string
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT COALESCE(metadata, '') FROM activity_log WHERE action = $1 AND entity_id = $2`,
		activity.ActionUserUpdate, id).Scan(&meta); err != nil {
		t.Fatalf("no %q row for the promotion: %v", activity.ActionUserUpdate, err)
	}
	for _, want := range []string{"role", user.RoleEditor, user.RoleAdmin, "sso"} {
		if !strings.Contains(meta, want) {
			t.Errorf("the protocol row's metadata is %s and does not name %q; a row that says "+
				"something changed without saying from what to what is not a record", meta, want)
		}
	}
}

// TestGroupSyncWritesExactlyTheMatchingWebsites covers the ordinary half:
// two groups map to two websites and to nothing else, in either header order,
// and a website the person lost is gone.
func TestGroupSyncWritesExactlyTheMatchingWebsites(t *testing.T) {
	t.Run("two groups in one order", func(t *testing.T) {
		h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", fwdGroupA+"|"+fwdGroupB, nil))
		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA, siteB}) {
			t.Errorf("user_websites = %v; want exactly %v", got, []int64{siteA, siteB})
		}
	})

	t.Run("the same two groups in the other order", func(t *testing.T) {
		h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", fwdGroupB+"|"+fwdGroupA, nil))
		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA, siteB}) {
			t.Errorf("user_websites = %v; want exactly %v — the header order is the identity "+
				"provider's and must not reach the database", got, []int64{siteA, siteB})
		}
	})

	t.Run("a website the person lost is gone", func(t *testing.T) {
		h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
		assign(t, database, id, siteA, siteB)

		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))

		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
			t.Errorf("user_websites = %v; want exactly %v", got, []int64{siteA})
		}
		lookup := NewWebsiteAccessLookup(database)
		if lookup(context.Background(), id, siteB) {
			t.Errorf("the account still reaches %q (id %d) after its group for that website was "+
				"removed at the identity provider", "Seite B", siteB)
		}
	})

	t.Run("a group this installation has never heard of is ignored", func(t *testing.T) {
		h, sm, database, siteA, _ := newGroupSyncAdmin(t)
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", "ganz-woanders|"+fwdGroupA+"|noch-eine", nil))
		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
			t.Errorf("user_websites = %v; want exactly %v — a directory has groups this "+
				"installation has never heard of, and that is normal rather than an error",
				got, []int64{siteA})
		}
	})
}

// TestGroupSyncLeavesAnAdministratorsAssignmentAlone.
//
// user.Store.Rights returns Everything() for an administrator two statements
// before it reads user_websites, so anything written there is invisible — and an
// invisible write is a diff a later reader has to reason about for nothing.
func TestGroupSyncLeavesAnAdministratorsAssignmentAlone(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteA)

	serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdAdminGroup, nil))

	if got := storedRole(t, database, id); got != user.RoleAdmin {
		t.Fatalf("users.role = %q; want %q", got, user.RoleAdmin)
	}
	if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
		t.Errorf("user_websites = %v; want the untouched %v — an administrator's assignment is "+
			"never read by Rights, so writing there changes nothing and shows in every diff",
			got, []int64{siteA})
	}
}

// TestGroupSyncCarriesThePublishingRight.
//
// No group grants may_publish. SetRights takes it as a field and would happily
// overwrite it on every sign-in, which would undo an operator's decision about a
// person on a schedule.
func TestGroupSyncCarriesThePublishingRight(t *testing.T) {
	h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteB)
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE users SET may_publish = 0 WHERE id = $1`, id); err != nil {
		t.Fatalf("take the publishing right away: %v", err)
	}

	serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))

	if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
		t.Fatalf("user_websites = %v; want %v — without a website change SetRights is never "+
			"called and this test would prove nothing about may_publish", got, []int64{siteA})
	}
	if storedMayPublish(t, database, id) {
		t.Error("may_publish went from false to true across a sign-in; no group grants the " +
			"publishing right and nothing here decides it — an operator decided it about a person")
	}
}

// TestGroupSyncIsIdempotent. SetRights replaces wholesale, so calling it twice
// with the same groups leaves the same rows; the protocol is where the
// difference would show, and one row per page view is a log nobody reads.
func TestGroupSyncIsIdempotent(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	for i := 0; i < 2; i++ {
		// A fresh session each time: an already signed-in session is left alone
		// by step 3 and the synchronisation would not run at all.
		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	}

	if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
		t.Errorf("user_websites = %v after two identical sign-ins; want %v", got, []int64{siteA})
	}
	if n := countAction(t, database, activity.ActionUserUpdate, id); n != 1 {
		t.Errorf("wrote %d %q rows across two identical sign-ins; want exactly 1 — the first "+
			"one, from what changed then. SetRights is idempotent; logging every call is not",
			n, activity.ActionUserUpdate)
	}
}

// TestGroupSyncKeepsTheLastAdministrator.
//
// user.Store.Update refuses to demote the last administrator. Locking an
// installation out because a group changed at somebody else's directory is a
// worse outcome than a role that lags one sign-in behind, so the refusal is
// recognised, logged loudly, and the sign-in continues.
func TestGroupSyncKeepsTheLastAdministrator(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)
	assign(t, database, id, siteA)

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))

	if got := storedRole(t, database, id); got != user.RoleAdmin {
		t.Errorf("users.role = %q; want %q — this is the last administrator and the "+
			"installation must not be left with none", got, user.RoleAdmin)
	}
	if res.userID != id {
		t.Errorf("the sign-in did not happen (session user_id = %d); a refused demotion is "+
			"not a refused sign-in", res.userID)
	}
	if res.role != user.RoleAdmin {
		t.Errorf("session user_role = %q; want %q — the role that reaches the session is the "+
			"one the account still has", res.role, user.RoleAdmin)
	}
	if !strings.Contains(logged.String(), "ada@example.com") {
		t.Errorf("the refused demotion was not logged with the account it concerns; log was:\n%s",
			logged.String())
	}
	if n := countAction(t, database, activity.ActionUserUpdate, id); n != 0 {
		t.Errorf("wrote %d %q rows for a demotion that did not happen; want 0", n, activity.ActionUserUpdate)
	}
}

// TestGroupSyncRefusesAnEditorWithNoMatchingWebsiteGroup.
//
// This is D-01's inversion reached by subtraction. An empty assignment is not
// "no websites": NewWebsiteAccessLookup reads assigned == 0 as *every* website.
// So the sign-in is refused and nothing is written.
func TestGroupSyncRefusesAnEditorWithNoMatchingWebsiteGroup(t *testing.T) {
	cases := []struct {
		name   string
		groups string
	}{
		{"no groups at all", ""},
		{"a header of nothing but separators", "||"},
		{"only groups this installation has never heard of", "ganz-woanders|noch-eine"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database, siteA, _ := newGroupSyncAdmin(t)
			id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
			assign(t, database, id, siteA)

			rec, res := serveForwardAuth(t, h, sm, true,
				fwdGroupRequest("ada", "ada@example.com", tc.groups, nil))

			if res.userID != 0 {
				t.Errorf("an editor whose groups match no configured website signed in as %d", res.userID)
			}
			if res.runs != 1 {
				t.Fatalf("the next handler ran %d times; want exactly 1 — a refusal is the "+
					"password form and never a status", res.runs)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("the middleware answered %d; a refusal is never a status of its own", rec.Code)
			}
			if n := countAction(t, database, activity.ActionAuthLoginFail, 0); n != 1 {
				t.Errorf("wrote %d %q rows; want exactly 1 naming the refusal", n, activity.ActionAuthLoginFail)
			}
			// The refusal must not itself be a way of emptying an assignment:
			// that would be the bug wearing a different hat.
			if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
				t.Errorf("user_websites = %v after the refusal; want the untouched %v — a "+
					"refusal that empties the assignment produces exactly the state it refuses",
					got, []int64{siteA})
			}
		})
	}
}

// TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite is plan 10-04's test run
// from the other end, and it is the assertion D-01 does not make.
//
// D-01 is written about account *creation*: a new account must not have zero
// rows in user_websites, because NewWebsiteAccessLookup reads zero as every
// website. SSO-06 re-applies the rights on every sign-in, so the same state is
// reachable by *subtraction* — an operator removes somebody's last website group
// at the identity provider, meaning to take their access away, and an empty
// SetRights would hand them everything.
//
// The assertion goes through NewWebsiteAccessLookup, the function main.go builds
// auth.RequireWebsiteAccess from, and never through a row count. Plan 10-04's
// mutation 7 is why: it wrote exactly one user_websites row naming the wrong
// website, so SELECT COUNT(*) returned 1 while the account reached a site it
// must not have.
func TestLosingEveryWebsiteGroupDoesNotGrantEveryWebsite(t *testing.T) {
	h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
	ctx := context.Background()
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	lookup := NewWebsiteAccessLookup(database)

	// 1. A sign-in through the real middleware with a group that maps to A.
	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	if res.userID != id {
		t.Fatalf("the first sign-in did not happen (session user_id = %d, account %d); "+
			"everything below would be measuring an account nobody reached", res.userID, id)
	}
	if !lookup(ctx, id, siteA) {
		t.Fatalf("after a sign-in carrying %q the editor cannot reach %q (id %d)", fwdGroupA, "Seite A", siteA)
	}
	if lookup(ctx, id, siteB) {
		t.Fatalf("after a sign-in carrying only %q the editor already reaches %q (id %d); "+
			"the second half of this test could not tell a regression from the starting state",
			fwdGroupA, "Seite B", siteB)
	}

	// 2. The same account signs in again, carrying no group that maps to any
	// configured website. This is the operator taking access away.
	_, res = serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", "irgendwas-anderes", nil))

	// 3. Three assertions, in this order, because each would be a different bug.
	if res.userID != 0 {
		t.Errorf("the editor lost every website group and was signed in anyway (as %d) — "+
			"taking somebody's last website group away has to take their access away", res.userID)
	}
	if n := countAction(t, database, activity.ActionAuthLoginFail, 0); n != 1 {
		t.Errorf("wrote %d %q rows; want exactly 1 — an operator has no other way to learn "+
			"that somebody was turned away", n, activity.ActionAuthLoginFail)
	}
	if lookup(ctx, id, siteB) {
		t.Errorf("the editor lost their last website group and gained access to %q (id %d) — "+
			"the sync wrote an empty assignment and NewWebsiteAccessLookup read it as \"all\"",
			"Seite B", siteB)
	}
	if !lookup(ctx, id, siteA) {
		t.Errorf("the refusal also emptied the assignment: the editor can no longer reach %q "+
			"(id %d) either. A refusal must not be a way of writing the empty list",
			"Seite A", siteA)
	}

	// 4. The control. Without it this test passes on a tree where the
	// synchronisation does nothing at all, or where the lookup was stubbed.
	// Emptying the rows by hand is exactly the state step 2 must never produce,
	// and the same call has to flip to true for it.
	if _, err := database.Write.ExecContext(ctx,
		`DELETE FROM user_websites WHERE user_id = $1`, id); err != nil {
		t.Fatalf("delete the assignment for the control step: %v", err)
	}
	if !lookup(ctx, id, siteB) {
		t.Fatal("with the assignment emptied the same lookup still refuses Seite B; the " +
			"assertions above were not measuring the assignment at all, and an empty write " +
			"would therefore have been invisible to them")
	}
}

// TestUnchangedGroupsWriteNoActivityRow counts rows rather than asserting a
// call: SSO-06 asks for every *change* to be logged, and /admin/protokoll is
// only evidence for as long as it is short enough to be read.
func TestUnchangedGroupsWriteNoActivityRow(t *testing.T) {
	h, sm, database, _, _ := newGroupSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	first := countAction(t, database, activity.ActionUserUpdate, id)

	serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	second := countAction(t, database, activity.ActionUserUpdate, id)

	if second != first {
		t.Errorf("%q rows went from %d to %d across a sign-in that changed nothing; "+
			"a protocol with one row per sign-in is a protocol nobody reads",
			activity.ActionUserUpdate, first, second)
	}
}

// TestGroupSyncLeavesTheAssignmentAloneWithNoGroupMappingConfigured guards the
// branch that keeps the refusal above from meaning something it does not.
//
// With HOLZCLOUD_SSO_WEBSITE_GROUPS unset, "your groups match no configured
// website" is true of everybody, so refusing on it would lock every editor out
// of single sign-on — including the account provisioning had just created and
// correctly assigned. The rule that has to hold is narrower and it still holds:
// the synchronisation never *writes* an empty assignment, and writing nothing
// cannot.
func TestGroupSyncLeavesTheAssignmentAloneWithNoGroupMappingConfigured(t *testing.T) {
	h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
	h.cfg.SSOWebsiteGroups = nil
	ctx := context.Background()
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteB)

	_, res := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", "irgendwas-anderes", nil))

	if res.userID != id {
		t.Fatalf("the editor was refused (session user_id = %d) although this installation "+
			"configures no group-to-website mapping at all; every editor would be locked out "+
			"of single sign-on, silently, because a refusal looks like the password form", res.userID)
	}
	if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteB}) {
		t.Errorf("user_websites = %v; want the untouched %v", got, []int64{siteB})
	}
	lookup := NewWebsiteAccessLookup(database)
	if lookup(ctx, id, siteA) {
		t.Errorf("the editor reaches %q (id %d) after a sign-in that was supposed to change "+
			"nothing — an empty assignment was written and read as \"all\"", "Seite A", siteA)
	}
	if !lookup(ctx, id, siteB) {
		t.Errorf("the editor lost %q (id %d) to a sign-in that was supposed to change nothing",
			"Seite B", siteB)
	}

	t.Run("the role half still runs", func(t *testing.T) {
		serveForwardAuth(t, h, sm, true,
			fwdGroupRequest("ada", "ada@example.com", fwdAdminGroup, nil))
		if got := storedRole(t, database, id); got != user.RoleAdmin {
			t.Errorf("users.role = %q; want %q — the two halves of SSO-06 are independent, "+
				"and an unconfigured website mapping does not switch the role off", got, user.RoleAdmin)
		}
	})
}

// TestGroupSyncRefusesAnEmptyAdministrationGroupOnItsOwn asks the guard the one
// question no request can ask it.
//
// HOLZCLOUD_SSO_ADMIN_GROUP has no default, so an operator who never sets it has
// the empty string configured — and an empty *configured* group must grant
// administration to nobody. Two independent layers hold that today:
// web.splitGroups drops empty elements so Identity.Groups never carries one, and
// the SSOAdminGroup != "" term in syncRightsFromGroups.
//
// Removing the second one alone leaves every test in this file green, because
// the first covers every input a real request can produce. That is not the same
// as the guard being redundant: web.Identity is an exported struct with an
// exported Groups field, splitGroups lives in another package and is not called
// from here, and anything that one day builds an Identity another way — a second
// reader, a preview screen, a fake in a test — arrives with the first layer
// gone. A guard whose removal nothing notices reads as tidiness and gets deleted
// by the next person tidying.
//
// So this one calls the function directly with an identity built by hand, the
// shape splitGroups never produces. Measured 2026-09-08: with both layers
// removed, a header of "|seite-a|" makes an administrator.
func TestGroupSyncRefusesAnEmptyAdministrationGroupOnItsOwn(t *testing.T) {
	h, sm, database, siteA, _ := newGroupSyncAdmin(t)
	h.cfg.SSOAdminGroup = ""
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	assign(t, database, id, siteA)

	u := &user.User{ID: id, Name: "T", Email: "ada@example.com", Role: user.RoleEditor}
	ident := &web.Identity{
		Username: "ada",
		Email:    "ada@example.com",
		Groups:   []string{"", fwdGroupA, ""},
	}

	var role string
	var err error
	inSession(t, sm, func(r *http.Request) {
		role, err = h.syncRightsFromGroups(r.Context(), r, u, ident)
	})

	if err != nil {
		t.Fatalf("syncRightsFromGroups refused an editor whose group maps to a website: %v", err)
	}
	if role != user.RoleEditor {
		t.Errorf("role = %q; want %q — the configured administration group is the empty string, "+
			"and an unset HOLZCLOUD_SSO_ADMIN_GROUP grants administration to nobody. This "+
			"identity carries an empty group element, which is what splitGroups is stopping "+
			"today and what nothing would stop if an Identity were ever built another way", role, user.RoleEditor)
	}
	if got := storedRole(t, database, id); got != user.RoleEditor {
		t.Errorf("users.role = %q; want %q", got, user.RoleEditor)
	}
}

// TestGroupSyncIsIdempotentWhateverShapeTheHeaderArrivesIn is what makes the
// sort and the deduplication load-bearing rather than tidy.
//
// Both were written because SetRights sorts and deduplicates again, so their
// absence changes no row in the database at all. What it changes is the
// comparison against the stored assignment: user.Store.Rights reads its rows
// ORDER BY website_id, so an unsorted or repeated collected list never equals
// the stored one, SetRights is called on every sign-in, and one protocol row per
// sign-in is written for a person whose rights never changed. Mutation 10
// removed the sort and every other test in this file stayed green.
//
// The two headers below are the two shapes that produce it: groups arriving in
// descending website order, and two groups the operator mapped to one website.
func TestGroupSyncIsIdempotentWhateverShapeTheHeaderArrivesIn(t *testing.T) {
	t.Run("two groups in descending website order", func(t *testing.T) {
		h, sm, database, siteA, siteB := newGroupSyncAdmin(t)
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

		header := fwdGroupB + "|" + fwdGroupA
		serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", header, nil))
		first := countAction(t, database, activity.ActionUserUpdate, id)
		serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", header, nil))

		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA, siteB}) {
			t.Fatalf("user_websites = %v; want %v", got, []int64{siteA, siteB})
		}
		if n := countAction(t, database, activity.ActionUserUpdate, id); n != first {
			t.Errorf("%q rows went from %d to %d across a second identical sign-in with the "+
				"header %q; the collected ids are compared against a list the database returns "+
				"ORDER BY website_id, so an unsorted list differs from itself forever",
				activity.ActionUserUpdate, first, n, header)
		}
	})

	t.Run("two groups the operator mapped to one website", func(t *testing.T) {
		h, sm, database, siteA, _ := newGroupSyncAdmin(t)
		h.cfg.SSOWebsiteGroups = map[string]int64{"redaktion-a": siteA, "leitung-a": siteA}
		id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

		header := "redaktion-a|leitung-a"
		serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", header, nil))
		first := countAction(t, database, activity.ActionUserUpdate, id)
		serveForwardAuth(t, h, sm, true, fwdGroupRequest("ada", "ada@example.com", header, nil))

		if got := assignedWebsites(t, database, id); !sameIDList(got, []int64{siteA}) {
			t.Fatalf("user_websites = %v; want %v", got, []int64{siteA})
		}
		if n := countAction(t, database, activity.ActionUserUpdate, id); n != first {
			t.Errorf("%q rows went from %d to %d across a second identical sign-in with the "+
				"header %q; the database holds one row per website however many groups lead "+
				"to it, so a repeated id differs from the stored list forever",
				activity.ActionUserUpdate, first, n, header)
		}
	})
}
