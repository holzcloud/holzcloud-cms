package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// A running session sees a demotion at the identity provider.
//
// ForwardAuthSignIn left every signed-in session exactly as it was (step 3),
// so the group synchronisation ran only for a request that had no session. An
// administrator whose group was removed at the identity provider stayed an
// administrator for as long as the CMS session lived — 24 hours, or 4 hours
// idle — while DEPLOY.md said the access goes "at the next sign-in, not at the
// next session expiry". For a person who keeps working, the next sign-in is the
// session expiry. Code review WR-08; the criterion 1 re-check measured it with
// the same cookie.
func TestARunningSessionSeesADemotion(t *testing.T) {
	h, sm, database, _, _ := newRightsSyncAdmin(t)
	seedAccount(t, database, "root@example.com", user.RoleAdmin)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	rec, first := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdAdminGroup+"|"+fwdGroupA, nil))
	if first.userID != id || first.role != user.RoleAdmin {
		t.Fatalf("the first sign-in did not make ada an administrator (user_id %d, role %q)", first.userID, first.role)
	}
	cookie := sessionCookie(t, sm, rec)

	_, next := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, cookie))

	if got := storedRole(t, database, id); got != user.RoleEditor {
		t.Errorf("users.role = %q after the identity provider removed the administration group and the "+
			"same session made another request; want %q", got, user.RoleEditor)
	}
	if next.role == user.RoleAdmin {
		t.Errorf("the running session still carries role %q after the demotion", next.role)
	}
}

// Another person signing in at the identity provider on the same browser does
// not inherit the previous person's CMS session.
func TestAnotherIdentityDoesNotInheritTheSession(t *testing.T) {
	h, sm, database, _, _ := newRightsSyncAdmin(t)
	ada := seedAccount(t, database, "ada@example.com", user.RoleEditor)
	grace := seedAccount(t, database, "grace@example.com", user.RoleEditor)

	rec, first := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	if first.userID != ada {
		t.Fatalf("ada was not signed in (user_id %d)", first.userID)
	}
	cookie := sessionCookie(t, sm, rec)

	_, next := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("grace", "grace@example.com", fwdGroupA, cookie))

	if next.userID == ada {
		t.Errorf("the identity provider now vouches for grace (account %d), and the request still ran "+
			"as ada (account %d) on ada's session", grace, ada)
	}
}

// Switching single sign-on off ends every session it established.
//
// A session carrying via_sso is exempt from the second factor. It lives in
// SQLite for 24 hours and survives a restart, so an operator who switches single
// sign-on off in an emergency — a proxy they no longer trust — kept every
// administrator session it had made, still exempt, and able to remove the
// second factor altogether. Code review WR-04; criterion 5 measured it.
func TestSwitchingSSOOffEndsTheSessionsItEstablished(t *testing.T) {
	h, sm, database, _, _ := newRightsSyncAdmin(t)
	seedAccount(t, database, "root@example.com", user.RoleAdmin)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	rec, first := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdAdminGroup, nil))
	if first.userID != id || !first.viaSSO {
		t.Fatalf("the sign-in did not establish an SSO session (user_id %d, via_sso %v)", first.userID, first.viaSSO)
	}
	cookie := sessionCookie(t, sm, rec)

	h.cfg.SSOEnabled = false
	_, next := serveForwardAuth(t, h, sm, false, fwdRequest("ada", "ada@example.com", cookie))

	if next.userID != 0 {
		t.Errorf("with single sign-on switched off, the session it established still signs requests in "+
			"(user_id %d, via_sso %v)", next.userID, next.viaSSO)
	}
}

// A password sign-in does not inherit the mark of a single sign-on session.
//
// completeLogin is the one funnel where a session becomes signed in, and scs's
// RenewToken keeps every value, so a session that once carried via_sso and then
// completed a password sign-in — as another account — carried the exemption
// into that account.
func TestCompleteLoginRemovesTheSSOMark(t *testing.T) {
	h, sm, database := newForwardAuthAdmin(t, true)
	id := seedAccount(t, database, "ada@example.com", user.RoleAdmin)

	var still bool
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		h.completeLogin(r, id, user.RoleAdmin, "ada@example.com")
		still = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
	})).ServeHTTP(rec, httptest.NewRequest("POST", "/admin/login", nil))

	if still {
		t.Error("completeLogin left via_sso in the session; the password path would sign in exempt " +
			"from the second factor")
	}
}

// A running single sign-on session is not signed in again on every request.
//
// Step 3 of ForwardAuthSignIn exists because a sign-in per request would rotate
// the token on every click and write one auth.login_success row per page view.
// Re-applying the rights of a running session is not a sign-in. Nothing held
// that until the session was re-checked on every request, which is exactly when
// it became easy to break: a session that forgets whom it was established for
// looks like somebody new on its next request.
func TestARunningSSOSessionIsNotSignedInAgain(t *testing.T) {
	h, sm, database, _, _ := newRightsSyncAdmin(t)
	id := seedAccount(t, database, "ada@example.com", user.RoleEditor)

	rec, first := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, nil))
	if first.userID != id {
		t.Fatalf("the first sign-in did not happen (user_id %d, account %d)", first.userID, id)
	}
	cookie := sessionCookie(t, sm, rec)

	rec2, second := serveForwardAuth(t, h, sm, true,
		fwdGroupRequest("ada", "ada@example.com", fwdGroupA, cookie))

	if second.userID != id {
		t.Errorf("the second request on a running session did not run as ada (user_id %d)", second.userID)
	}
	if n := countAction(t, database, activity.ActionAuthLoginSuccess, id); n != 1 {
		t.Errorf("%d %q rows after two requests on one session; want 1 — re-applying the rights is "+
			"not a sign-in", n, activity.ActionAuthLoginSuccess)
	}
	for _, c := range rec2.Result().Cookies() {
		if c.Name == sm.Cookie.Name && c.Value != cookie.Value {
			t.Errorf("the session token was rotated on an ordinary request of a running session")
		}
	}
}
