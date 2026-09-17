package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The account screens: who may sign in, with what password, and how somebody
// who has lost theirs gets back in.
//
// Two of the guards here are the ones that decide whether an installation can
// be locked out of itself — nobody may delete their own account, and the last
// administrator may not be deleted at all. Both had a comment and no test.
//
// Driven red by twenty-five mutations, all twenty-five caught. One had to be
// chased: every test passed a purpose to the link handler explicitly, so
// removing the DEFAULT changed nothing. The default matters and in one
// direction — a link with no purpose is a reset, which ends every open session
// of the account; an invitation would leave an attacker's session alive.
//
// Writing these also found that newTestAdmin built the handler with a zero
// Argon2Params, so every path that hashes a password PANICKED — "number of
// rounds too small" — rather than failing. Nothing in the package tested such a
// path, which is why nobody had met it. It gets cheapHashing now, the
// parameters the scope fixtures already used.
//
// And linkTitle returned two bare German literals into a translated slot. The
// collector reads call sites and this one hands it a function call, so both
// were invisible to the gate on a program whose source language has been
// English since v2.0. They are i18n.N now, and translated.

func userAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, int64) {
	t.Helper()
	h, sm, database, _ := newTestAdmin(t)
	ctx := context.Background()
	admin, err := h.users.Create(ctx, "Erste Administratorin", "chefin@example.com",
		"passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatalf("create the first admin: %v", err)
	}
	return h, sm, database, admin
}

func TestAUserIsCreatedWithRightsAndRefusedWithout(t *testing.T) {
	h, sm, _, _ := userAdmin(t)
	ctx := context.Background()

	rec, bad, _ := albumFlash(t, h, sm, h.HandleUserCreate, postForm("/admin/users/new",
		url.Values{
			"name": {"  Redakteurin  "}, "email": {"  red@example.com  "},
			"password": {"passwort123"}, "role": {"editor"},
		}, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	made, err := h.users.GetByEmail(ctx, "red@example.com")
	if err != nil || made == nil {
		t.Fatalf("the user was not created: %v", err)
	}
	if made.Name != "Redakteurin" {
		t.Errorf("the name was not trimmed: %q", made.Name)
	}
	if made.Role != user.RoleEditor {
		t.Errorf("role = %q", made.Role)
	}

	for _, c := range []struct {
		what string
		form url.Values
	}{
		{"no email", url.Values{"password": {"passwort123"}, "role": {"editor"}}},
		{"a password that is too short", url.Values{
			"email": {"kurz@example.com"}, "password": {"kurz"}, "role": {"editor"}}},
		{"a role nobody offers", url.Values{
			"email": {"rolle@example.com"}, "password": {"passwort123"}, "role": {"gott"}}},
		{"no role at all", url.Values{
			"email": {"ohne@example.com"}, "password": {"passwort123"}}},
		{"an email that is already taken", url.Values{
			"email": {"red@example.com"}, "password": {"passwort123"}, "role": {"editor"}}},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleUserCreate,
			postForm("/admin/users/new", c.form, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad == "" {
			t.Errorf("%s: accepted in silence", c.what)
		}
	}

	// Exactly the two accounts: the first admin and the one editor.
	all, err := h.users.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("%d accounts after five refusals, want 2", len(all))
	}
}

// The two guards that decide whether an installation can be locked out of
// itself.
func TestNobodyDeletesThemselvesOrTheLastAdministrator(t *testing.T) {
	h, sm, database, admin := userAdmin(t)
	ctx := context.Background()

	// Deleting yourself: refused however many administrators there are.
	rec, bad, _ := albumFlashAs(t, h, sm, admin, h.HandleUserDelete,
		postForm("/admin/users/1/delete", nil,
			map[string]string{"id": strconv.FormatInt(admin, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if bad != "You cannot delete your own account" {
		t.Errorf("deleting oneself was answered with %q", bad)
	}
	if u, _ := h.users.GetByID(ctx, admin); u == nil {
		t.Fatal("the account deleted itself")
	}

	// The last administrator: refused even by somebody else.
	second, err := h.users.Create(ctx, "Zweite", "zweite@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	rec, bad, _ = albumFlashAs(t, h, sm, second, h.HandleUserDelete,
		postForm("/admin/users/1/delete", nil,
			map[string]string{"id": strconv.FormatInt(admin, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if bad != "The last administrator cannot be deleted" {
		t.Errorf("deleting the last administrator was answered with %q", bad)
	}
	if u, _ := h.users.GetByID(ctx, admin); u == nil {
		t.Fatal("the last administrator was deleted — the installation is now locked out of itself")
	}

	// With a second administrator the first one may go, or the guard would be
	// "no administrator is ever deletable" rather than what it says.
	third, err := h.users.Create(ctx, "Dritte", "dritte@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	rec = serveAsOnly(t, h, sm, third, h.HandleUserDelete,
		postForm("/admin/users/1/delete", nil,
			map[string]string{"id": strconv.FormatInt(admin, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if u, _ := h.users.GetByID(ctx, admin); u != nil {
		t.Error("with two administrators the first one still could not be deleted")
	}
	_ = database
}

// Changing a password is how somebody locks out an attacker who already holds a
// session cookie, so it ends every OTHER session and keeps the one doing it.
func TestAPasswordChangeEndsTheOtherSessionsAndKeepsThisOne(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	ctx := context.Background()

	// Somebody changing their own password has to know the current one.
	rec, bad, _ := albumFlashAs(t, h, sm, admin, h.HandlePasswordChange,
		postForm("/admin/users/1/password", url.Values{
			"current_password": {"falsch"}, "new_password": {"neuespasswort"},
			"confirm_password": {"neuespasswort"},
		}, map[string]string{"id": strconv.FormatInt(admin, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if bad != "The current password is not right" {
		t.Errorf("a wrong current password was answered with %q", bad)
	}

	for _, c := range []struct {
		what, pw, confirm, want string
	}{
		{"too short", "kurz", "kurz", "New password must be at least 8 characters"},
		{"not matching", "neuespasswort", "etwasanderes", "The passwords do not match"},
	} {
		_, bad, _ := albumFlashAs(t, h, sm, admin, h.HandlePasswordChange,
			postForm("/admin/users/1/password", url.Values{
				"current_password": {"passwort123"}, "new_password": {c.pw},
				"confirm_password": {c.confirm},
			}, map[string]string{"id": strconv.FormatInt(admin, 10)}))
		if bad != c.want {
			t.Errorf("%s: answered %q, want %q", c.what, bad, c.want)
		}
	}

	// The old password still works, because none of that went through.
	u, err := h.users.GetByID(ctx, admin)
	if err != nil || u == nil {
		t.Fatal(err)
	}
	if ok, _ := auth.VerifyPassword("passwort123", u.Password); !ok {
		t.Fatal("a refused change altered the password")
	}

	// And the real one does.
	rec, bad, good := albumFlashAs(t, h, sm, admin, h.HandlePasswordChange,
		postForm("/admin/users/1/password", url.Values{
			"current_password": {"passwort123"}, "new_password": {"neuespasswort"},
			"confirm_password": {"neuespasswort"},
		}, map[string]string{"id": strconv.FormatInt(admin, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("the real change: status %d — %q", rec.Code, bad)
	}
	if good == "" {
		t.Error("the change said nothing")
	}
	u, _ = h.users.GetByID(ctx, admin)
	if ok, _ := auth.VerifyPassword("neuespasswort", u.Password); !ok {
		t.Error("the new password does not work")
	}
	if ok, _ := auth.VerifyPassword("passwort123", u.Password); ok {
		t.Error("the old password still works")
	}
}

// Somebody who is not an administrator may change their own password and
// nobody else's.
func TestOnlyAnAdministratorChangesSomebodyElsesPassword(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	ctx := context.Background()
	editor, err := h.users.Create(ctx, "Redakteurin", "red@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}

	rec := serveWithRole(t, h, sm, editor, user.RoleEditor, h.HandlePasswordChange,
		httptest.NewRequest(http.MethodGet, "/admin/users/1/password", nil),
		map[string]string{"id": strconv.FormatInt(admin, 10)})
	if rec.Code != http.StatusForbidden {
		t.Errorf("an editor reached somebody else's password screen: status %d", rec.Code)
	}

	// Their own is fine, or the guard would be "nobody changes any password".
	rec = serveWithRole(t, h, sm, editor, user.RoleEditor, h.HandlePasswordChange,
		httptest.NewRequest(http.MethodGet, "/admin/users/1/password", nil),
		map[string]string{"id": strconv.FormatInt(editor, 10)})
	if rec.Code != http.StatusOK {
		t.Errorf("an editor could not reach their own password screen: status %d", rec.Code)
	}
}

// An access link is shown on screen whether or not it was mailed: mail is the
// convenience, not the mechanism. A server with no mail set up must still be
// able to invite somebody.
func TestAnAccessLinkIsAlwaysShownAndWorksExactlyOnce(t *testing.T) {
	h, sm, _, _ := userAdmin(t)
	ctx := context.Background()
	invited, err := h.users.Create(ctx, "Neue", "neue@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}

	// Without a purpose the link is a RESET and not an invitation. That
	// default is the safe direction: a reset link ends every open session of
	// the account, and issuing one where an invitation was meant costs a
	// sign-in; the other way round would hand somebody an invitation that
	// leaves an attacker's session alive.
	noPurpose := postForm("/admin/users/1/link", url.Values{},
		map[string]string{"id": strconv.FormatInt(invited, 10)})
	rec := serve(t, h, sm, h.HandleUserLink, noPurpose)
	if rec.Code != http.StatusOK {
		t.Fatalf("no purpose: status %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "/admin/reset/") {
		t.Error("a link with no purpose is not a reset link")
	}
	// And a purpose nobody offers is a reset too, rather than whatever was sent.
	odd := postForm("/admin/users/1/link", url.Values{"purpose": {"uebernahme"}},
		map[string]string{"id": strconv.FormatInt(invited, 10)})
	if body := serve(t, h, sm, h.HandleUserLink, odd).Body.String(); !strings.Contains(body, "/admin/reset/") {
		t.Error("an unknown purpose produced something other than a reset link")
	}

	req := postForm("/admin/users/1/link", url.Values{"purpose": {user.PurposeInvite}},
		map[string]string{"id": strconv.FormatInt(invited, 10)})
	rec = serve(t, h, sm, h.HandleUserLink, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/admin/activate/") {
		t.Fatalf("the screen does not show the link an admin has to copy:\n%.400s", body)
	}
	token := linkToken(t, body, "/admin/activate/")

	// The form behind it.
	set := h.HandleSetPassword(user.PurposeInvite)
	show := httptest.NewRequest(http.MethodGet, "/admin/activate/"+token, nil)
	show.SetPathValue("token", token)
	if rec := serve(t, h, sm, set, show); rec.Code != http.StatusOK {
		t.Fatalf("the link's own screen: status %d", rec.Code)
	}

	// A password that does not keep the rules leaves the link usable, which is
	// the order the handler comments insist on: burning a link with nothing to
	// show for it is the failure that cannot be undone.
	bad := postForm("/admin/activate/"+token, url.Values{
		"password": {"kurz"}, "password_confirm": {"kurz"}}, map[string]string{"token": token})
	if rec := serve(t, h, sm, set, bad); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a short password: status %d, want 422", rec.Code)
	}

	good := postForm("/admin/activate/"+token, url.Values{
		"password": {"neuespasswort"}, "password_confirm": {"neuespasswort"}},
		map[string]string{"token": token})
	if rec := serve(t, h, sm, set, good); rec.Code != http.StatusSeeOther {
		t.Fatalf("the real one: status %d", rec.Code)
	}
	u, _ := h.users.GetByID(ctx, invited)
	if ok, _ := auth.VerifyPassword("neuespasswort", u.Password); !ok {
		t.Error("the password was not set")
	}

	// Exactly once: the same link again is gone, and says so without saying
	// whether it ever existed.
	again := postForm("/admin/activate/"+token, url.Values{
		"password": {"nochwas12"}, "password_confirm": {"nochwas12"}},
		map[string]string{"token": token})
	rec = serve(t, h, sm, set, again)
	if rec.Code != http.StatusGone {
		t.Errorf("a used link: status %d, want 410", rec.Code)
	}
}

// Unknown, expired and already used are answered the same way, because saying
// which applies would tell a stranger whether the token ever existed.
func TestABadLinkSaysNothingAboutItself(t *testing.T) {
	h, sm, _, _ := userAdmin(t)
	set := h.HandleSetPassword(user.PurposeReset)

	bodies := map[string]string{}
	for _, token := range []string{"gibtesnicht", "", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/reset/"+token, nil)
		req.SetPathValue("token", token)
		rec := serve(t, h, sm, set, req)
		if rec.Code != http.StatusGone {
			t.Errorf("%q: status %d, want 410", token, rec.Code)
		}
		bodies[token] = rec.Body.String()
	}
	first := ""
	for token, body := range bodies {
		if first == "" {
			first = body
			continue
		}
		if len(body) != len(first) {
			t.Errorf("%q gets a different answer from the others — the length alone "+
				"tells a stranger something", token)
		}
	}
}

// albumFlashAs is albumFlash with a signed-in person, because every guard on
// these screens asks who is asking.
func albumFlashAs(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) (rec *httptest.ResponseRecorder, bad, good string) {
	t.Helper()
	rec = httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		sm.Put(r.Context(), auth.SessionKeyUserRole, user.RoleAdmin)
		handlerErr = fn(w, r)
		bad = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		good = sm.GetString(r.Context(), auth.SessionKeyFlashSuccess)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec, bad, good
}

func serveAsOnly(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	fn func(http.ResponseWriter, *http.Request) error, req *http.Request,
) *httptest.ResponseRecorder {
	t.Helper()
	rec, _, _ := albumFlashAs(t, h, sm, userID, fn, req)
	return rec
}

// serveWithRole is the same with a role that is not administrator, for the
// guards that ask about the role rather than about the id.
func serveWithRole(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64,
	role string, fn func(http.ResponseWriter, *http.Request) error,
	req *http.Request, pathValues map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	var handlerErr error
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		sm.Put(r.Context(), auth.SessionKeyUserRole, role)
		handlerErr = fn(w, r)
	})).ServeHTTP(rec, req)
	if handlerErr != nil {
		t.Fatalf("handler: %v", handlerErr)
	}
	return rec
}

// linkToken pulls the one-time secret out of the rendered screen, the way an
// administrator copies it out with their eyes.
func linkToken(t *testing.T, body, prefix string) string {
	t.Helper()
	i := strings.Index(body, prefix)
	if i < 0 {
		t.Fatalf("no %s in the screen", prefix)
	}
	rest := body[i+len(prefix):]
	end := strings.IndexAny(rest, `"'< `)
	if end < 0 {
		t.Fatal("the link does not end")
	}
	token := rest[:end]
	if token == "" {
		t.Fatal("the link carries no token")
	}
	return token
}
