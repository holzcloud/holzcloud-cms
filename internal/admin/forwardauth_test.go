package admin

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
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
