package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// newSecondFactorAdmin builds the package's ordinary test handler with hashing
// parameters that hash rather than panic — newTestAdmin hands the user store a
// zero auth.Argon2Params, and anything here that creates an account needs
// cheapHashing, as waves 4 and 5 both recorded.
func newSecondFactorAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	h, sm, database, _ := newTestAdmin(t)
	h.users.Params = cheapHashing
	return h, sm, database
}

// seedSecondFactorAccount creates an account with the given role and returns
// its id.
func seedSecondFactorAccount(t *testing.T, h *Handler, email, role string) int64 {
	t.Helper()
	id, err := h.users.Create(context.Background(), "Test", email, "correct-horse-battery-staple", role)
	if err != nil {
		t.Fatalf("create %s: %v", role, err)
	}
	return id
}

// enableSecondFactor confirms an authenticator for userID by hand, so the
// account is in the state the disable guard is about.
func enableSecondFactor(t *testing.T, database *db.DB, userID int64) {
	t.Helper()
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE users SET totp_secret = 'JBSWY3DPEHPK3PXP', totp_confirmed_at = CURRENT_TIMESTAMP WHERE id = $1`,
		userID); err != nil {
		t.Fatalf("enable second factor: %v", err)
	}
}

// secondFactorEnabled reports whether the account still has a confirmed
// authenticator. The question the disable guard answers is asked of the row and
// never of the flash message.
func secondFactorEnabled(t *testing.T, database *db.DB, userID int64) bool {
	t.Helper()
	var confirmed *string
	if err := database.Read.QueryRowContext(context.Background(),
		`SELECT totp_confirmed_at FROM users WHERE id = $1`, userID).Scan(&confirmed); err != nil {
		t.Fatalf("read second factor state: %v", err)
	}
	return confirmed != nil
}

// serveSignedIn runs one handler with a live session belonging to userID,
// established either by password or through the identity provider.
func serveSignedIn(t *testing.T, h *Handler, sm *scs.SessionManager, fn func(http.ResponseWriter, *http.Request) error, req *http.Request, userID int64, viaSSO bool) *httptest.ResponseRecorder {
	t.Helper()
	// A session established through the identity provider exists only while
	// single sign-on is switched on. This helper used to build one on a handler
	// whose switch was off — the state the Phase 10 code review measured as a
	// defect (WR-04) — and the tests using it passed because that defect let
	// the mark count without the switch. A test about the switched-off case sets
	// the mark itself and says so.
	if viaSSO {
		h.cfg.SSOEnabled = true
	}
	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		if viaSSO {
			sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		}
		handlerErr = fn(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec
}

// TestSecondFactorDisableStillRefusesAPasswordAdmin is the assertion this whole
// plan must not break. An administrator who signed in with a password may not
// switch a second factor off from inside a signed-in session, because a stolen
// session could then simply switch it off and stay.
func TestSecondFactorDisableStillRefusesAPasswordAdmin(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	serveSignedIn(t, h, sm, h.HandleTwoFactorDisable, req, id, false)

	if !secondFactorEnabled(t, database, id) {
		t.Error("an administrator who signed in with a password switched their second factor off; the guard that refuses it is the one protection a password session has left")
	}
}

// TestSecondFactorDisableNamesTheWayBackForAPasswordAdmin holds the other half
// of the guard: the refusal says how to get back in from the server, which is
// the only way left when the device is gone.
func TestSecondFactorDisableNamesTheWayBackForAPasswordAdmin(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	var flash string
	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, id)
		if err := h.HandleTwoFactorDisable(w, r); err != nil {
			t.Fatalf("handler: %v", err)
		}
		flash = sm.GetString(r.Context(), auth.SessionKeyFlashError)
	})).ServeHTTP(rec, req)

	if !strings.Contains(flash, "holzcloud user 2fa disable") {
		t.Errorf("flash = %q; want it to name „holzcloud user 2fa disable“ — the refusal has to say what the way back is", flash)
	}
}

// TestSecondFactorDisableLetsAnSSOAdminThrough is the decision, not a side
// effect: this installation no longer requires a second factor of somebody the
// identity provider signed in, so it cannot refuse to let them switch one off.
func TestSecondFactorDisableLetsAnSSOAdminThrough(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	serveSignedIn(t, h, sm, h.HandleTwoFactorDisable, req, id, true)

	if secondFactorEnabled(t, database, id) {
		t.Error("an administrator signed in through the identity provider could not switch their second factor off; this installation does not require one of them, so it may not refuse")
	}
}

// TestSecondFactorDisableStillWorksForAnEditor is the negative control: the
// path was already open for an editor and nothing about it moved.
func TestSecondFactorDisableStillWorksForAnEditor(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "editor@test.local", user.RoleEditor)
	enableSecondFactor(t, database, id)

	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	serveSignedIn(t, h, sm, h.HandleTwoFactorDisable, req, id, false)

	if secondFactorEnabled(t, database, id) {
		t.Error("an editor could not switch their own second factor off")
	}
}

// TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin reads the
// rendered screen rather than the struct field, because the sentence is what
// the person meets.
func TestAccountScreenDoesNotCallTheSecondFactorCompulsoryForAnSSOAdmin(t *testing.T) {
	h, sm, _ := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/admin/konto", nil)
	rec := serveSignedIn(t, h, sm, h.HandleAccount, req, id, true)
	body := rec.Body.String()

	if strings.Contains(body, "For your account it is compulsory") {
		t.Error("the account screen tells an administrator signed in through the identity provider that a second factor is compulsory here; for them it is not, and that is the same fact the predicate decides")
	}
}

// TestAccountScreenStillCallsTheSecondFactorCompulsoryForAPasswordAdmin is the
// same screen, unchanged, for the person whose protection this plan must not
// remove.
func TestAccountScreenStillCallsTheSecondFactorCompulsoryForAPasswordAdmin(t *testing.T) {
	h, sm, _ := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/admin/konto", nil)
	rec := serveSignedIn(t, h, sm, h.HandleAccount, req, id, false)
	body := rec.Body.String()

	if !strings.Contains(body, "For your account it is compulsory") {
		t.Error("an administrator who signed in with a password is no longer told a second factor is compulsory; nothing about the password path may move in this plan")
	}
}

// TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin covers the
// two call sites on the setup screen, which decide what a screen says rather
// than what a middleware does.
func TestSecondFactorSetupScreenDropsTheCompulsoryWarningForAnSSOAdmin(t *testing.T) {
	h, sm, _ := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)

	for _, tc := range []struct {
		name   string
		viaSSO bool
		want   bool
	}{
		{"through the identity provider", true, false},
		{"with a password", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, auth.SetupPath, nil)
			req.Host = "admin.test"
			rec := serveSignedIn(t, h, sm, h.HandleTwoFactorSetup, req, id, tc.viaSSO)
			got := strings.Contains(rec.Body.String(), "For administrators this step is compulsory")
			if got != tc.want {
				t.Errorf("the setup screen's compulsory warning = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced is the
// person's half of SSO-07. Without it somebody signed in through the identity
// provider meets a second-factor card that simply asks nothing of them, with no
// explanation of why.
func TestAccountScreenTellsAnSSOPersonWhereTheirSecondFactorIsEnforced(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)

	// enrolled is the half of the screen a grep gate for "ViaSSO" cannot see:
	// the card has two branches and the sentence sits above both, so a notice
	// moved inside one of them would still satisfy the grep while half the
	// people it was written for never read it.
	for _, tc := range []struct {
		name     string
		role     string
		email    string
		viaSSO   bool
		enrolled bool
		want     bool
	}{
		{"an administrator through the identity provider", user.RoleAdmin, "a@test.local", true, false, true},
		{"an editor through the identity provider", user.RoleEditor, "b@test.local", true, false, true},
		{"an administrator with a password", user.RoleAdmin, "c@test.local", false, false, false},
		{"an editor with a password", user.RoleEditor, "d@test.local", false, false, false},
		{"an administrator through the identity provider who set one up here anyway", user.RoleAdmin, "e@test.local", true, true, true},
		{"an editor through the identity provider who set one up here anyway", user.RoleEditor, "f@test.local", true, true, true},
		{"an administrator with a password who has one", user.RoleAdmin, "g@test.local", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := seedSecondFactorAccount(t, h, tc.email, tc.role)
			if tc.enrolled {
				enableSecondFactor(t, database, id)
			}
			req := httptest.NewRequest(http.MethodGet, "/admin/konto", nil)
			rec := serveSignedIn(t, h, sm, h.HandleAccount, req, id, tc.viaSSO)
			got := strings.Contains(rec.Body.String(), "through your organisation")
			if got != tc.want {
				t.Errorf("the account screen names the identity provider = %v; want %v — a person who signed in there has to be told that is where the second factor is decided", got, tc.want)
			}
		})
	}
}

// TestUserListTellsAnAdministratorTheInstallationDependsOnTheIdentityProvider
// is the operator's half of SSO-07. The user list is where an administrator
// thinks about who can get in, so it is where "what is guarding these accounts"
// is a question somebody actually has.
func TestUserListTellsAnAdministratorTheInstallationDependsOnTheIdentityProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   bool
		want bool
	}{
		{"with single sign-on switched on", true, true},
		{"with single sign-on switched off", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, _ := newSecondFactorAdmin(t)
			h.cfg.SSOEnabled = tc.on
			id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)

			req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
			rec := serveSignedIn(t, h, sm, h.HandleUserList, req, id, false)
			got := strings.Contains(rec.Body.String(), "brings their two-step verification with them")
			if got != tc.want {
				t.Errorf("the user list names the identity provider = %v; want %v — with single sign-on off the screen must be byte-for-byte what it was", got, tc.want)
			}
		})
	}
}

// TestSecondFactorDisableRefusesAMarkWithSSOSwitchedOff is the criterion 5
// measurement written down as a test: with single sign-on switched off, a
// session still carrying via_sso is an administrator's session like any other,
// and this installation requires their second factor — so it may not be removed.
func TestSecondFactorDisableRefusesAMarkWithSSOSwitchedOff(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	h.cfg.SSOEnabled = false
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, id)
		// Set by hand rather than through serveSignedIn, which switches single
		// sign-on on for a session established through it.
		sm.Put(r.Context(), auth.SessionKeyViaSSO, true)
		handlerErr = h.HandleTwoFactorDisable(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}

	if !secondFactorEnabled(t, database, id) {
		t.Error("with single sign-on switched off, a session still carrying its mark removed an " +
			"administrator's second factor")
	}
}

// TestSecondFactorRefusalIsTranslated: the refusal that stops an administrator
// switching off their own second factor is a statement about security, at the
// MustHaveSecondFactor call site Phase 10 changed. It was three string
// literals joined with +, so the collector never saw it and every
// administration read it in German (WINDOWS.md entry 17, threat T-10-50).
func TestSecondFactorRefusalIsTranslated(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	var flash string
	req := postForm("/admin/2fa/aus", url.Values{}, nil)
	req = req.WithContext(i18n.WithLang(req.Context(), "en"))
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, id)
		if err := h.HandleTwoFactorDisable(w, r); err != nil {
			t.Fatalf("handler: %v", err)
		}
		flash = sm.GetString(r.Context(), auth.SessionKeyFlashError)
	})).ServeHTTP(rec, req)

	if flash == "" {
		t.Fatal("no refusal was flashed; the guard this test is about did not run")
	}
	if strings.Contains(flash, "Pflicht") || strings.Contains(flash, "Administratoren") {
		t.Errorf("the refusal reaches an English administration in German: %q", flash)
	}
	if !strings.Contains(flash, "holzcloud user 2fa disable") {
		t.Errorf("the translated refusal lost the way back: %q", flash)
	}
}
