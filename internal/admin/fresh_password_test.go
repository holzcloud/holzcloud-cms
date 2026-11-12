package admin

import (
	"context"
	"net/http"
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
// factor (restart and confirm), and setting somebody ELSE's password.
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
	// what the setup screen's GET leaves behind, and that GET is not guarded:
	// the question stands in front of the activation.
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
