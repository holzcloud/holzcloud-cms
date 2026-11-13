package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/totp"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The password is asked again in front of the actions that hand an account to
// somebody else — audit of 2026-11-10, Part 1, item 12. Three of them are
// guarded where they are registered (main.go: a new user, an access link,
// ending somebody's sessions) and three inside their handler, because only the
// handler knows whether the question applies: replacing a CONFIRMED second
// factor (restart and confirm, and since phase 31 opening the setup screen),
// and setting somebody ELSE's password.
//
// The other half is just as important and tested here as well: first-time and
// forced second-factor setup ask for nothing, or a new administrator who is
// sent to the setup screen before anything else could never get out of it.

// fresh wraps a handler so the session counts as freshly confirmed, which is
// what a person has just after the confirmation screen.
func fresh(sm *scs.SessionManager, fn func(http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		auth.MarkElevated(sm, r.Context())
		return fn(w, r)
	}
}

func asksForThePassword(t *testing.T, what string, code int, location string) {
	t.Helper()
	if code != http.StatusSeeOther || !strings.HasPrefix(location, auth.ConfirmPath) {
		t.Errorf("%s: got %d to %q, want a redirect to %s", what, code, location, auth.ConfirmPath)
	}
}

func TestReplacingAConfirmedSecondFactorAsksForThePasswordFirst(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	ctx := context.Background()
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)
	const confirmed = "JBSWY3DPEHPK3PXP" // what enableSecondFactor writes

	state := func() *user.TwoFactor {
		t.Helper()
		tf, err := h.users.GetTwoFactor(ctx, id)
		if err != nil || tf == nil {
			t.Fatalf("GetTwoFactor: %v", err)
		}
		return tf
	}

	// Restart from an unattended session: nothing is written.
	rec := serveSignedIn(t, h, sm, h.HandleTwoFactorRestart, postForm("/admin/2fa/einrichten/neu", nil, nil), id, false)
	asksForThePassword(t, "restart", rec.Code, rec.Header().Get("Location"))
	if tf := state(); tf.PendingSecret != "" || tf.Secret != confirmed || !tf.Enabled() {
		t.Errorf("restart without a confirmation wrote the row: %+v", tf)
	}

	// Confirming a new device from an unattended session. The pending secret is
	// written here through BeginTwoFactor, as if it had been left behind
	// earlier; the setup screen's GET asks for the password too (see
	// TestTheSetupScreenOfAConfirmedFactorAsksForThePasswordFirst), and the
	// question stands in front of the activation as well.
	pending, err := h.users.BeginTwoFactor(ctx, id)
	if err != nil {
		t.Fatalf("BeginTwoFactor: %v", err)
	}
	code, err := totp.Code(pending, time.Now())
	if err != nil {
		t.Fatalf("totp.Code: %v", err)
	}
	rec = serveSignedIn(t, h, sm, h.HandleTwoFactorSetup,
		postForm(auth.SetupPath, url.Values{"code": {code}}, nil), id, false)
	asksForThePassword(t, "confirm a new device", rec.Code, rec.Header().Get("Location"))
	if tf := state(); tf.Secret != confirmed || tf.PendingSecret != pending {
		t.Errorf("a new device was activated without a confirmation: %+v", tf)
	}

	// Freshly confirmed, the restart goes through.
	rec = serveSignedIn(t, h, sm, fresh(sm, h.HandleTwoFactorRestart), postForm("/admin/2fa/einrichten/neu", nil, nil), id, false)
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != auth.SetupPath {
		t.Errorf("restart with a fresh confirmation: got %d to %q, want %s", rec.Code, loc, auth.SetupPath)
	}
	if tf := state(); tf.PendingSecret == "" || tf.PendingSecret == pending {
		t.Errorf("restart with a fresh confirmation wrote no new pending secret: %+v", tf)
	}
}

// The forced setup of a new administrator: no confirmed factor, so no question.
func TestFirstTimeSecondFactorSetupAsksForNoPassword(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	ctx := context.Background()
	id := seedSecondFactorAccount(t, h, "neu@test.local", user.RoleAdmin)

	rec := serveSignedIn(t, h, sm, h.HandleTwoFactorRestart, postForm("/admin/2fa/einrichten/neu", nil, nil), id, false)
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != auth.SetupPath {
		t.Fatalf("first restart: got %d to %q, want %s", rec.Code, loc, auth.SetupPath)
	}
	tf, err := h.users.GetTwoFactor(ctx, id)
	if err != nil || tf == nil || tf.PendingSecret == "" {
		t.Fatalf("no pending secret after the restart: %+v %v", tf, err)
	}
	code, err := totp.Code(tf.PendingSecret, time.Now())
	if err != nil {
		t.Fatalf("totp.Code: %v", err)
	}
	rec = serveSignedIn(t, h, sm, h.HandleTwoFactorSetup,
		postForm(auth.SetupPath, url.Values{"code": {code}}, nil), id, false)
	if loc := rec.Header().Get("Location"); strings.HasPrefix(loc, auth.ConfirmPath) {
		t.Fatalf("first-time setup was sent to the confirmation screen: %q", loc)
	}
	if !secondFactorEnabled(t, database, id) {
		t.Errorf("first-time setup did not switch the second factor on: status %d", rec.Code)
	}
}

// openSetupScreen is a GET of the two-factor setup screen, the way somebody
// typing its address or following the account page's link sends it.
func openSetupScreen() *http.Request {
	req := httptest.NewRequest(http.MethodGet, auth.SetupPath, nil)
	req.Host = "admin.test"
	return req
}

// showsASecret reports whether the setup screen drew a secret to type in.
func showsASecret(body string) bool {
	return strings.Contains(body, `class="totp-secret"`)
}

// pendingSecretOf is the pending secret the account's row holds.
func pendingSecretOf(t *testing.T, h *Handler, id int64) string {
	t.Helper()
	tf, err := h.users.GetTwoFactor(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTwoFactor: %v", err)
	}
	if tf == nil {
		return ""
	}
	return tf.PendingSecret
}

// The setup screen's GET writes a pending secret and shows it. Over a confirmed
// factor that is the first half of moving the account to another phone, so
// from an unattended session it shows nothing and writes nothing; after the
// password the way back is the setup screen itself (RGT-07, D-09).
func TestTheSetupScreenOfAConfirmedFactorAsksForThePasswordFirst(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	rec := serveSignedIn(t, h, sm, h.HandleTwoFactorSetup, openSetupScreen(), id, false)
	loc := rec.Header().Get("Location")
	asksForThePassword(t, "the setup screen of a confirmed factor", rec.Code, loc)
	if !strings.Contains(loc, "weiter=%2Fadmin%2F2fa%2Feinrichten") {
		t.Errorf("the way back after the password is %q; want the setup screen itself", loc)
	}
	if showsASecret(rec.Body.String()) {
		t.Error("the setup screen showed a secret before the password was confirmed")
	}
	if p := pendingSecretOf(t, h, id); p != "" {
		t.Errorf("the setup screen wrote a pending secret before the password was confirmed: %q", p)
	}

	rec = serveSignedIn(t, h, sm, fresh(sm, h.HandleTwoFactorSetup), openSetupScreen(), id, false)
	if rec.Code != http.StatusOK || !showsASecret(rec.Body.String()) {
		t.Errorf("with a fresh confirmation: status %d, secret shown %v; want 200 and a secret",
			rec.Code, showsASecret(rec.Body.String()))
	}
	if p := pendingSecretOf(t, h, id); p == "" || !strings.Contains(rec.Body.String(), totp.FormatSecret(p)) {
		t.Errorf("with a fresh confirmation the screen does not show the pending secret %q", p)
	}
}

// A lookup of the factor that fails says nothing about whether one is
// confirmed, and a failure there does not mean the write behind it fails too:
// the recovery codes live in a table of their own, and the pending secret is
// written to the users row. The question fails closed (D-08), as the rights
// lookup does — otherwise a transient error showed a new secret over a
// confirmed factor without the password, the exposure RGT-07 closes.
func TestAConfirmedFactorThatCannotBeReadAsksForThePasswordFirst(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "admin@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)
	if _, err := database.Write.Exec(`ALTER TABLE user_recovery_codes RENAME TO user_recovery_codes_away`); err != nil {
		t.Fatalf("break the factor lookup: %v", err)
	}
	if _, err := h.users.GetTwoFactor(context.Background(), id); err == nil {
		t.Fatal("the factor lookup still works; the test breaks nothing")
	}
	pending := func() string {
		t.Helper()
		var p string
		if err := database.Read.QueryRow(`SELECT totp_pending_secret FROM users WHERE id = $1`, id).Scan(&p); err != nil {
			t.Fatalf("read pending secret: %v", err)
		}
		return p
	}

	rec := serveSignedIn(t, h, sm, h.HandleTwoFactorSetup, openSetupScreen(), id, false)
	asksForThePassword(t, "the setup screen with a failing lookup", rec.Code, rec.Header().Get("Location"))
	if showsASecret(rec.Body.String()) {
		t.Error("the setup screen showed a secret with a failing lookup")
	}
	if p := pending(); p != "" {
		t.Errorf("the setup screen wrote a pending secret with a failing lookup: %q", p)
	}

	rec = serveSignedIn(t, h, sm, h.HandleTwoFactorRestart, postForm("/admin/2fa/einrichten/neu", nil, nil), id, false)
	asksForThePassword(t, "restart with a failing lookup", rec.Code, rec.Header().Get("Location"))
	if p := pending(); p != "" {
		t.Errorf("restart wrote a pending secret with a failing lookup: %q", p)
	}
}

// First enrolment and the forced enrolment of a new administrator have no
// confirmed factor, and a question there would stand between that person and
// the only screen they are let onto. The secret is shown at once.
func TestTheFirstAndTheForcedSetupShowTheSecretWithoutAsking(t *testing.T) {
	for _, tc := range []struct {
		name, email, role string
	}{
		{"first enrolment of an editor", "red@test.local", user.RoleEditor},
		{"forced enrolment of a new administrator", "neu@test.local", user.RoleAdmin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sm, _ := newSecondFactorAdmin(t)
			id := seedSecondFactorAccount(t, h, tc.email, tc.role)

			rec := serveSignedIn(t, h, sm, h.HandleTwoFactorSetup, openSetupScreen(), id, false)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d to %q; want 200 without a question", rec.Code, rec.Header().Get("Location"))
			}
			p := pendingSecretOf(t, h, id)
			if p == "" || !showsASecret(rec.Body.String()) || !strings.Contains(rec.Body.String(), totp.FormatSecret(p)) {
				t.Errorf("the setup screen did not show the pending secret %q", p)
			}
		})
	}
}

// A single sign-on account has no password it knows, and exempting it would
// remove the one question for exactly the accounts an operator did not create
// by hand (elevate.go, "What must NOT be done"). It is asked like any other.
func TestASingleSignOnSessionIsNotExemptFromTheSetupQuestion(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	id := seedSecondFactorAccount(t, h, "sso@test.local", user.RoleEditor)
	enableSecondFactor(t, database, id)

	rec := serveSignedIn(t, h, sm, h.HandleTwoFactorSetup, openSetupScreen(), id, true)
	asksForThePassword(t, "the setup screen in a single sign-on session", rec.Code, rec.Header().Get("Location"))
	if p := pendingSecretOf(t, h, id); p != "" {
		t.Errorf("a single sign-on session wrote a pending secret without a question: %q", p)
	}
}

func TestSettingSomebodyElsesPasswordAsksForThePasswordFirst(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	ctx := context.Background()
	other, err := h.users.Create(ctx, "Redakteurin", "red@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	change := func() *http.Request {
		return postForm("/admin/users/2/password", url.Values{
			"new_password": {"neuespasswort"}, "confirm_password": {"neuespasswort"},
		}, map[string]string{"id": strconv.FormatInt(other, 10)})
	}
	verifies := func(pw string) bool {
		t.Helper()
		u, err := h.users.GetByID(ctx, other)
		if err != nil || u == nil {
			t.Fatalf("GetByID: %v", err)
		}
		ok, _ := auth.VerifyPassword(pw, u.Password)
		return ok
	}

	rec, _, _ := albumFlashAs(t, h, sm, admin, h.HandlePasswordChange, change())
	asksForThePassword(t, "somebody else's password", rec.Code, rec.Header().Get("Location"))
	if !verifies("passwort123") || verifies("neuespasswort") {
		t.Fatal("somebody else's password was set from an unattended session")
	}

	rec, bad, _ := albumFlashAs(t, h, sm, admin, fresh(sm, h.HandlePasswordChange), change())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("with a fresh confirmation: status %d — %q", rec.Code, bad)
	}
	if !verifies("neuespasswort") {
		t.Error("with a fresh confirmation the password did not change")
	}
}

// An editor who changed their own password used to land on /admin/users, a
// list only an administrator may see, and read "Forbidden".
func TestChangingTheOwnPasswordLeadsBackToTheAccountPage(t *testing.T) {
	h, sm, _, _ := userAdmin(t)
	ctx := context.Background()
	editor, err := h.users.Create(ctx, "Redakteurin", "red@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}

	rec := serveWithRole(t, h, sm, editor, user.RoleEditor, h.HandlePasswordChange,
		postForm("/admin/users/2/password", url.Values{
			"current_password": {"passwort123"},
			"new_password":     {"neuespasswort"}, "confirm_password": {"neuespasswort"},
		}, nil),
		map[string]string{"id": strconv.FormatInt(editor, 10)})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/konto" {
		t.Errorf("Location = %q, want /admin/konto", loc)
	}
	u, _ := h.users.GetByID(ctx, editor)
	if ok, _ := auth.VerifyPassword("neuespasswort", u.Password); !ok {
		t.Error("the own password did not change")
	}
}
