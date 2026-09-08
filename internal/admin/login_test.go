package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// testSignOutPath is config.Load's own default for HOLZCLOUD_SSO_SIGN_OUT_PATH:
// the route the authentik outpost serves on this application's domain.
const testSignOutPath = "/outpost.goauthentik.io/sign_out"

// newLogoutAdmin builds the package's ordinary test handler with the two things
// HandleLogout reads beyond the session: a protocol store, and a single
// sign-on configuration.
//
// newTestAdmin builds its config by hand rather than through config.Load, so
// SSOSignOutPath is empty there and has to be set here — the same shape
// newForwardAuthAdmin uses for the secret.
func newLogoutAdmin(t *testing.T, ssoEnabled bool) (*Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	h, sm, database, _ := newTestAdmin(t)
	h.cfg.SSOEnabled = ssoEnabled
	h.cfg.SSOSignOutPath = testSignOutPath
	h.SetActivityStore(activity.NewStore(database))
	return h, sm, database
}

// signedIn establishes a session shaped like the one completeLogin leaves
// behind and returns the cookie a browser would carry back to /admin/logout.
//
// viaSSO is the only difference between the two paths this plan is about.
func signedIn(t *testing.T, sm *scs.SessionManager, id int64, email string, viaSSO bool) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, id)
		sm.Put(r.Context(), auth.SessionKeyUserEmail, email)
		sm.Put(r.Context(), auth.SessionKeyUserRole, "admin")
		if viaSSO {
			sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		}
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))

	c := sessionCookie(t, sm, rec)
	if c == nil {
		t.Fatal("no session cookie was issued for the signed-in session")
	}
	return c
}

// logoutRequest builds the POST the sign-out button sends. base.html's form is
// a plain POST; htmx is the shape it would have if that form ever gained
// hx-post, which the helper under test already covers.
func logoutRequest(cookie *http.Cookie, htmx bool) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.Host = "admin.test"
	req.AddCookie(cookie)
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	return req
}

// sessionUserID reports who the session behind cookie belongs to now. Zero
// means the session is anonymous, which is what Destroy leaves behind.
func sessionUserID(t *testing.T, sm *scs.SessionManager, cookie *http.Cookie) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.AddCookie(cookie)

	var got int64
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	})).ServeHTTP(httptest.NewRecorder(), req)
	return got
}

// requireRelativeTarget is the assertion the absence of a Content-Security-Policy
// change rests on, written as four explicit conditions rather than one equality
// so that it keeps holding when an operator configures a different path.
//
// The fourth condition is wave 1's finding rather than this plan's: a browser
// reads "/\evil.example" as protocol-relative exactly as it reads
// "//evil.example", so one leading slash is not on its own enough. The rule
// itself lives in config.isLocalPath and runs at load; this only asserts that
// the handler does not undo it.
func requireRelativeTarget(t *testing.T, loc string) {
	t.Helper()
	if !strings.HasPrefix(loc, "/") {
		t.Errorf("Location = %q; want a path on this server, beginning with a slash", loc)
	}
	if strings.HasPrefix(loc, "//") {
		t.Errorf("Location = %q; a leading // is protocol-relative and leaves this origin", loc)
	}
	if strings.HasPrefix(loc, `/\`) {
		t.Errorf(`Location = %q; a leading /\ is protocol-relative to a browser too`, loc)
	}
	if strings.Contains(loc, "://") {
		t.Errorf("Location = %q; a sign-out target carrying a scheme is an open redirect", loc)
	}
}

func TestLogoutOfAnSSOSessionSignsOutAtTheIdentityProvider(t *testing.T) {
	h, sm, database := newLogoutAdmin(t, true)
	id := seedAccount(t, database, "sso@test", "admin")
	cookie := signedIn(t, sm, id, "sso@test", true)

	rec := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != testSignOutPath {
		t.Errorf("Location = %q; want %q — destroying the session here leaves authentik's "+
			"own cookie in the browser, and the next click signs the person straight back in",
			got, testSignOutPath)
	}
}

func TestLogoutOfAPasswordSessionIsExactlyWhatItWas(t *testing.T) {
	h, sm, database := newLogoutAdmin(t, true)
	id := seedAccount(t, database, "password@test", "admin")
	cookie := signedIn(t, sm, id, "password@test", false)

	rec := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false))

	// Byte for byte: same status, same target. SSO-09 is that nothing about the
	// password path changed, and single sign-on is switched ON in this handler
	// — it is the session that decides, not the installation.
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/admin/login" {
		t.Errorf("Location = %q; want %q", got, "/admin/login")
	}
}

func TestLogoutWithSSOSwitchedOffIgnoresAStaleViaSSOFlag(t *testing.T) {
	// A session carrying via_sso cannot normally exist while SSO is off,
	// because the only writer is the middleware the same setting switches off.
	// It can survive a restart of a server whose operator has just turned SSO
	// off, and a redirect to an outpost that is no longer there is a dead end.
	h, sm, database := newLogoutAdmin(t, false)
	id := seedAccount(t, database, "stale@test", "admin")
	cookie := signedIn(t, sm, id, "stale@test", true)

	rec := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false))

	if got := rec.Header().Get("Location"); got != "/admin/login" {
		t.Errorf("Location = %q; want %q — with HOLZCLOUD_SSO_ENABLED off there is no outpost to reach", got, "/admin/login")
	}
}

func TestLogoutTargetIsAlwaysARelativePath(t *testing.T) {
	// The values are taken from config.Load rather than written into the
	// handler by hand: the rule that a sign-out target is a path on this server
	// is wave 1's and is enforced at load, and this test asserts the link
	// between the two rather than restating the rule.
	accepted := []string{
		testSignOutPath,
		"/sign_out",
		"/outpost.goauthentik.io/sign_out?rd=/admin/login",
	}
	for _, want := range accepted {
		t.Run("config.Load accepts "+want, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_SSO_SIGN_OUT_PATH", want)
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load: %v", err)
			}

			h, sm, database := newLogoutAdmin(t, true)
			h.cfg.SSOSignOutPath = cfg.SSOSignOutPath
			id := seedAccount(t, database, "sso@test", "admin")
			cookie := signedIn(t, sm, id, "sso@test", true)

			loc := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false)).Header().Get("Location")
			if loc != want {
				t.Errorf("Location = %q; want %q", loc, want)
			}
			requireRelativeTarget(t, loc)
		})
	}

	t.Run("the password path", func(t *testing.T) {
		h, sm, database := newLogoutAdmin(t, true)
		id := seedAccount(t, database, "password@test", "admin")
		cookie := signedIn(t, sm, id, "password@test", false)
		requireRelativeTarget(t, serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false)).Header().Get("Location"))
	})

	// The other direction, and the reason the handler needs no narrowing of its
	// own: an address that would leave this origin never becomes a
	// configuration at all, so it can never become a Location.
	refused := []string{
		"https://evil.example/sign_out",
		"//evil.example/sign_out",
		`/\evil.example/sign_out`,
		"outpost.goauthentik.io/sign_out",
	}
	for _, bad := range refused {
		t.Run("config.Load refuses "+bad, func(t *testing.T) {
			t.Setenv("HOLZCLOUD_SSO_SIGN_OUT_PATH", bad)
			if _, err := config.Load(); err == nil {
				t.Fatalf("config.Load accepted %q; a sign-out is the one redirect a person follows without looking", bad)
			}
		})
	}
}

func TestLogoutRedirectIsPermittedByTheAdminFormActionPolicy(t *testing.T) {
	// This is the test the missing Content-Security-Policy change rests on.
	//
	// adminCSP carries form-action 'self', and a redirect answering a form POST
	// is checked against form-action by some browsers — the incident is written
	// up in internal/web/headers.go beside PaymentFormAction. It fires for a
	// CROSS-ORIGIN target only, and this handler's targets are same-origin. So
	// there is nothing to change; what has to hold is the premise, and here it
	// is asserted against the policy actually set on the very response that
	// carries the redirect, rather than argued in a comment.
	for _, tc := range []struct {
		name   string
		viaSSO bool
	}{
		{"the identity provider's sign-out", true},
		{"the login form", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database := newLogoutAdmin(t, true)
			id := seedAccount(t, database, "sso@test", "admin")
			cookie := signedIn(t, sm, id, "sso@test", tc.viaSSO)

			rec := httptest.NewRecorder()
			chain := web.AdminHeaders(sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := h.HandleLogout(w, r); err != nil {
					t.Fatalf("HandleLogout: %v", err)
				}
			})))
			chain.ServeHTTP(rec, logoutRequest(cookie, false))

			csp := rec.Header().Get("Content-Security-Policy")
			if !strings.Contains(csp, "form-action 'self'") {
				t.Fatalf("the admin policy no longer carries form-action 'self': %q — "+
					"if the directive moved, this test is the wrong place to find out, but it is a place", csp)
			}
			loc := rec.Header().Get("Location")
			requireRelativeTarget(t, loc)
			if loc == "" {
				t.Fatal("no Location on the sign-out response")
			}
			// The day this fails, the fix is not to loosen the assertion: it is
			// web.AdminCSP / web.AdminHeadersWith mirroring PublicCSP /
			// SecureHeadersWith, wired where AdminHeaders is wired today, with
			// frame-ancestors 'none', X-Frame-Options: DENY and Cache-Control:
			// no-store kept — cmd/holzcloud/main_test.go asserts those three.
			if strings.Contains(loc, "://") || strings.HasPrefix(loc, "//") {
				t.Errorf("Location = %q is cross-origin; form-action 'self' will block it silently in Safari", loc)
			}
		})
	}
}

func TestLogoutWritesTheProtocolRowOnBothPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		viaSSO bool
		email  string
	}{
		{"through the identity provider", true, "sso@test"},
		{"through the password form", false, "password@test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database := newLogoutAdmin(t, true)
			id := seedAccount(t, database, tc.email, "admin")
			cookie := signedIn(t, sm, id, tc.email, tc.viaSSO)

			serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false))

			rows := activityRows(t, database)
			if len(rows) != 1 {
				t.Fatalf("activity rows = %d; want exactly 1", len(rows))
			}
			if rows[0].Action != activity.ActionAuthLogout {
				t.Errorf("action = %q; want %q", rows[0].Action, activity.ActionAuthLogout)
			}
			// The actor is what the row is for. It is only there because the
			// row is written before Destroy: afterwards the session no longer
			// knows who went.
			if rows[0].ActorEmail != tc.email {
				t.Errorf("actor_email = %q; want %q", rows[0].ActorEmail, tc.email)
			}
			if rows[0].UserID == nil || *rows[0].UserID != id {
				t.Errorf("user_id = %v; want %d", rows[0].UserID, id)
			}
		})
	}
}

func TestLogoutDestroysTheSessionOnBothPaths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		viaSSO bool
	}{
		{"through the identity provider", true},
		{"through the password form", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, database := newLogoutAdmin(t, true)
			id := seedAccount(t, database, "gone@test", "admin")
			cookie := signedIn(t, sm, id, "gone@test", tc.viaSSO)

			if got := sessionUserID(t, sm, cookie); got != id {
				t.Fatalf("before the sign-out the session belongs to %d; want %d", got, id)
			}
			serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, tc.viaSSO))
			if got := sessionUserID(t, sm, cookie); got != 0 {
				t.Errorf("after the sign-out the same cookie still belongs to %d; want an anonymous session", got)
			}
		})
	}
}

func TestLogoutOfAnHtmxRequestAnswersWithHXRedirect(t *testing.T) {
	// base.html's sign-out is a plain POST today and there is no hx-boost in
	// the admin templates, so this shape does not occur yet. Going through the
	// package's existing redirect helper covers it at no cost, which is why the
	// handler will not have to be revisited if the form ever gains hx-post.
	h, sm, database := newLogoutAdmin(t, true)
	id := seedAccount(t, database, "sso@test", "admin")
	cookie := signedIn(t, sm, id, "sso@test", true)

	rec := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, true))

	if got := rec.Header().Get("HX-Redirect"); got != testSignOutPath {
		t.Errorf("HX-Redirect = %q; want %q — a 303 answering an htmx POST is swapped into the page instead of followed", got, testSignOutPath)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q; an htmx answer carries HX-Redirect and no Location", got)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q; want empty", body)
	}
}

func TestLogoutReadsTheViaSSOFlagBeforeTheSessionIsDestroyed(t *testing.T) {
	h, sm, database := newLogoutAdmin(t, true)
	id := seedAccount(t, database, "sso@test", "admin")

	// Part one: the branch is taken.
	cookie := signedIn(t, sm, id, "sso@test", true)
	loc := serve(t, h, sm, h.HandleLogout, logoutRequest(cookie, false)).Header().Get("Location")
	if loc != testSignOutPath {
		t.Errorf("Location = %q; want %q", loc, testSignOutPath)
	}

	// Part two, the control, and the reason part one is an ordering assertion
	// rather than a branch assertion: the same read performed AFTER sm.Destroy
	// answers false. A HandleLogout that read the flag one line lower would
	// therefore take the password path for everybody, part one would fail, and
	// every test that does not look at the target would still pass.
	second := signedIn(t, sm, id, "sso@test", true)
	var before, after bool
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
		if err := sm.Destroy(r.Context()); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
		after = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
	})).ServeHTTP(httptest.NewRecorder(), logoutRequest(second, false))

	if !before {
		t.Error("the session did not carry via_sso before Destroy; the control proves nothing")
	}
	if after {
		t.Error("via_sso survived Destroy — the ordering this handler depends on has changed, " +
			"and the branch could be moved below Destroy without any test noticing")
	}

}
