package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
)

// lookupSecondFactor returns a SecondFactorLookup that reports every account
// with the given role and enrolment state.
func lookupSecondFactor(role string, enabled bool) SecondFactorLookup {
	return func(context.Context, int64) (SecondFactorState, error) {
		return SecondFactorState{Role: role, Enabled: enabled}, nil
	}
}

// serveSecondFactor drives one request through RequireSecondFactor with a live
// session carrying userID, and via_sso when viaSSO is true.
//
// The session is written inside the LoadAndSave chain, which is the only place
// scs will accept a write — the same shape middleware_test.go uses for
// RequireAuth.
func serveSecondFactor(t *testing.T, sm *scs.SessionManager, lookup SecondFactorLookup, userID int64, viaSSO bool, path string) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	reached := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	chain := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID != 0 {
			sm.Put(r.Context(), SessionKeyUserID, userID)
		}
		if viaSSO {
			sm.Put(r.Context(), SessionKeyViaSSO, true)
		}
		RequireSecondFactor(sm, lookup)(inner).ServeHTTP(w, r)
	}))

	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec, &reached
}

// TestMustHaveSecondFactorOverBothRolesAndBothWaysIn is the whole predicate,
// all four combinations, so no row of it can change without a named test going
// red.
//
// The two viaSSO == false rows are SSO-09 for this function: they ARE the old
// MustHaveSecondFactor(role), asserted as a table rather than spot-checked,
// because a later change to the password path has to break something named
// after the password path.
func TestMustHaveSecondFactorOverBothRolesAndBothWaysIn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		role   string
		viaSSO bool
		want   bool
	}{
		{"an administrator who signed in with a password", "admin", false, true},
		{"an editor who signed in with a password", "editor", false, false},
		{"an administrator who signed in through the identity provider", "admin", true, false},
		{"an editor who signed in through the identity provider", "editor", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MustHaveSecondFactor(tc.role, tc.viaSSO); got != tc.want {
				t.Errorf("MustHaveSecondFactor(%q, %v) = %v; want %v", tc.role, tc.viaSSO, got, tc.want)
			}
		})
	}
}

// TestMustHaveSecondFactorIsUnchangedForAPasswordSession states the same thing
// the other way round, against the literal it replaced. If this ever fails,
// this plan removed a protection from password users while adding a
// convenience for SSO ones.
func TestMustHaveSecondFactorIsUnchangedForAPasswordSession(t *testing.T) {
	for _, role := range []string{"admin", "editor"} {
		if got, want := MustHaveSecondFactor(role, false), role == "admin"; got != want {
			t.Errorf("MustHaveSecondFactor(%q, false) = %v; want %v — passing false must be bit-for-bit the old one-argument predicate", role, got, want)
		}
	}
}

// TestMustHaveSecondFactorIgnoresAnUnknownRole keeps the boundary honest:
// users.role carries CHECK (role IN ('admin','editor')) and there is no third
// role, so anything else is a row that cannot exist — and it must not be
// required to have a second factor by accident either way in.
func TestMustHaveSecondFactorIgnoresAnUnknownRole(t *testing.T) {
	for _, viaSSO := range []bool{false, true} {
		if MustHaveSecondFactor("", viaSSO) {
			t.Errorf("MustHaveSecondFactor(\"\", %v) = true; want false", viaSSO)
		}
	}
}

// TestRequireSecondFactorLetsAnSSOSessionThrough is the whole of D-04 seen from
// the outside: an administrator whose session was established through the
// reverse proxy has already answered whatever the identity provider demanded,
// and is not sent to the setup page for a second one.
func TestRequireSecondFactorLetsAnSSOSessionThrough(t *testing.T) {
	sm := testSessionManager()
	rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", false), 7, true, "/admin/")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200 — an administrator signed in through the identity provider was sent to %q, and asking the same person for a second factor twice is what this plan removes",
			rec.Code, rec.Header().Get("Location"))
	}
	if !*reached {
		t.Error("the next handler did not run; an SSO session must reach the administration")
	}
}

// TestRequireSecondFactorStillRedirectsAPasswordAdmin is SSO-09 for this file.
// It passed before this plan and must pass after it, unchanged: nothing about
// the password path moved.
func TestRequireSecondFactorStillRedirectsAPasswordAdmin(t *testing.T) {
	sm := testSessionManager()
	rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", false), 7, false, "/admin/")

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want 303 — an administrator who signed in with a password and has no second factor is still sent to the setup page", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != SetupPath {
		t.Errorf("Location = %q; want %q", loc, SetupPath)
	}
	if *reached {
		t.Error("the next handler ran; a redirect is not a fall-through")
	}
}

// TestRequireSecondFactorLetsAnEnrolledAdminThrough covers the other reason the
// middleware lets somebody past, so a change that makes viaSSO the only way
// through has a test to break.
func TestRequireSecondFactorLetsAnEnrolledAdminThrough(t *testing.T) {
	sm := testSessionManager()
	rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", true), 7, false, "/admin/")

	if rec.Code != http.StatusOK || !*reached {
		t.Errorf("status = %d, reached = %v; want 200 and true — an administrator who has confirmed an authenticator is not sent anywhere", rec.Code, *reached)
	}
}

// TestRequireSecondFactorLetsAnEditorThrough asserts the row of the predicate
// this plan does not touch, both ways in.
func TestRequireSecondFactorLetsAnEditorThrough(t *testing.T) {
	for _, viaSSO := range []bool{false, true} {
		name := "with a password"
		if viaSSO {
			name = "through the identity provider"
		}
		t.Run(name, func(t *testing.T) {
			sm := testSessionManager()
			rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("editor", false), 7, viaSSO, "/admin/")
			if rec.Code != http.StatusOK || !*reached {
				t.Errorf("status = %d, reached = %v; want 200 and true — forcing a second factor on someone who edits opening hours is how shared logins get created", rec.Code, *reached)
			}
		})
	}
}

// TestRequireSecondFactorIgnoresARequestWithNoSession keeps the three early
// returns honest. None of them changes in this plan, and each is a way the
// middleware could start reading a session flag it must not need.
func TestRequireSecondFactorIgnoresARequestWithNoSession(t *testing.T) {
	t.Run("no lookup configured", func(t *testing.T) {
		sm := testSessionManager()
		rec, reached := serveSecondFactor(t, sm, nil, 7, false, "/admin/")
		if rec.Code != http.StatusOK || !*reached {
			t.Errorf("status = %d, reached = %v; want 200 and true — with no lookup the middleware decides nothing", rec.Code, *reached)
		}
	})

	t.Run("a second-factor path stays reachable", func(t *testing.T) {
		sm := testSessionManager()
		rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", false), 7, false, SetupPath)
		if rec.Code != http.StatusOK || !*reached {
			t.Errorf("status = %d, reached = %v; want 200 and true — an account that cannot finish the setup and cannot leave is stuck on one screen", rec.Code, *reached)
		}
	})

	t.Run("signing out stays reachable", func(t *testing.T) {
		sm := testSessionManager()
		rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", false), 7, false, "/admin/logout")
		if rec.Code != http.StatusOK || !*reached {
			t.Errorf("status = %d, reached = %v; want 200 and true", rec.Code, *reached)
		}
	})

	t.Run("no user in the session", func(t *testing.T) {
		sm := testSessionManager()
		rec, reached := serveSecondFactor(t, sm, lookupSecondFactor("admin", false), 0, false, "/admin/")
		if rec.Code != http.StatusOK || !*reached {
			t.Errorf("status = %d, reached = %v; want 200 and true — RequireSecondFactor sits inside RequireAuth and decides nothing on its own", rec.Code, *reached)
		}
	})
}
