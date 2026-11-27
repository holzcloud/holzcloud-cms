package admin

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

const (
	confirmClient = "198.51.100.30:4711"
	busySentence  = "The server is busy checking other passwords. Please try again in a moment."
	waitSentence  = "Too many failed attempts. Please wait a moment."
)

// confirmResult is one POST to the confirmation screen. Elevated is read inside
// the request's own session scope.
type confirmResult struct {
	Code       int
	Location   string
	RetryAfter string
	Body       string
	Elevated   bool
}

func postConfirm(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64, password string) confirmResult {
	t.Helper()
	req := postForm("/admin/bestaetigen", url.Values{"password": {password}, "weiter": {"/admin/users"}}, nil)
	req.RemoteAddr = confirmClient
	rec := httptest.NewRecorder()
	var res confirmResult
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		sm.Put(r.Context(), auth.SessionKeyUserRole, user.RoleAdmin)
		if err := h.HandleConfirmPassword(w, r); err != nil {
			t.Errorf("HandleConfirmPassword: %v", err)
		}
		res.Elevated = auth.Elevated(sm, r.Context())
	})).ServeHTTP(rec, req)
	res.Code = rec.Code
	res.Location = rec.Header().Get("Location")
	res.RetryAfter = rec.Header().Get("Retry-After")
	res.Body = rec.Body.String()
	return res
}

// sentenceOf is the catalogue sentence as the page prints it: translated for the
// request's language and escaped as a template escapes it.
func sentenceOf(sentence string) string {
	lang := i18n.Lang(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	return template.HTMLEscapeString(i18n.T(lang, sentence))
}

// holdTheOnlySlot swaps in a gate with one slot and short waits, takes the
// slot, and returns the function that gives it back.
func holdTheOnlySlot(t *testing.T) (release func()) {
	t.Helper()
	gate := auth.NewGate(1, 10*time.Millisecond, 10*time.Millisecond)
	previous := auth.SetArgon2Gate(gate)
	t.Cleanup(func() { auth.SetArgon2Gate(previous) })
	release, err := gate.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}
	return release
}

// Login does not trim the password, so a password that ends in a space is a
// different password, and the confirmation has to agree.
func TestTheConfirmationComparesThePasswordAsLoginDoes(t *testing.T) {
	h, sm, _, _ := userAdmin(t)
	id, err := h.users.Create(context.Background(), "Leerzeichen", "leer@example.com", "passwort123 ", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	res := postConfirm(t, h, sm, id, "passwort123 ")
	if res.Code != http.StatusSeeOther || res.Location != "/admin/users" || !res.Elevated {
		t.Errorf("the exact password: %d to %q, elevated %v; want 303 to /admin/users and elevated", res.Code, res.Location, res.Elevated)
	}
	res = postConfirm(t, h, sm, id, "passwort123")
	if res.Code != http.StatusUnprocessableEntity || res.Elevated {
		t.Errorf("the password without its trailing space: %d, elevated %v; want 422 and not elevated", res.Code, res.Elevated)
	}
}

func TestTheConfirmationIsThrottledPerUser(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	other, err := h.users.Create(context.Background(), "Zweite", "zweite@example.com", "passwort456", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 5 {
		if res := postConfirm(t, h, sm, admin, "falsch"); res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong password %d: status %d, want 422", i+1, res.Code)
		}
	}
	res := postConfirm(t, h, sm, admin, "passwort123")
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("the right password after five wrong ones: status %d, want 429", res.Code)
	}
	if res.RetryAfter == "" || res.RetryAfter == "0" {
		t.Errorf("Retry-After %q, want whole seconds of at least 1", res.RetryAfter)
	}
	if !strings.Contains(res.Body, sentenceOf(waitSentence)) {
		t.Error("the form does not show the wait sentence")
	}
	if res.Elevated {
		t.Error("a throttled confirmation elevated the session")
	}

	// A second user from the same client is still checked.
	if res := postConfirm(t, h, sm, other, "passwort456"); res.Code != http.StatusSeeOther || !res.Elevated {
		t.Errorf("a second user: %d, elevated %v; want 303 and elevated", res.Code, res.Elevated)
	}
}

func TestABusyConfirmationAnswers503AndIsNotCounted(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	release := holdTheOnlySlot(t)

	// More attempts than the budget: none of them may take anything from it.
	for i := range 7 {
		res := postConfirm(t, h, sm, admin, "passwort123")
		if res.Code != http.StatusServiceUnavailable {
			t.Fatalf("busy attempt %d: status %d, want 503", i+1, res.Code)
		}
		if res.RetryAfter != "2" {
			t.Errorf("busy attempt %d: Retry-After %q, want 2", i+1, res.RetryAfter)
		}
		if !strings.Contains(res.Body, sentenceOf(busySentence)) {
			t.Errorf("busy attempt %d: the form does not show the busy sentence", i+1)
		}
		if res.Elevated {
			t.Fatalf("busy attempt %d elevated the session", i+1)
		}
	}

	release()
	for i := range 5 {
		if res := postConfirm(t, h, sm, admin, "falsch"); res.Code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong password %d after the gate cleared: status %d, want 422 (checked)", i+1, res.Code)
		}
	}
	if res := postConfirm(t, h, sm, admin, "falsch"); res.Code != http.StatusTooManyRequests {
		t.Errorf("the sixth wrong password: status %d, want 429: the busy answers had taken part of the budget", res.Code)
	}
}

// changeOwnPassword posts the self-service form and reads the flash it left.
func changeOwnPassword(t *testing.T, h *Handler, sm *scs.SessionManager, id int64, current string) (rec *httptest.ResponseRecorder, bad string) {
	t.Helper()
	req := postForm("/admin/users/"+strconv.FormatInt(id, 10)+"/password", url.Values{
		"current_password": {current}, "new_password": {"neuespasswort"}, "confirm_password": {"neuespasswort"},
	}, map[string]string{"id": strconv.FormatInt(id, 10)})
	req.RemoteAddr = confirmClient
	rec, bad, _ = albumFlashAs(t, h, sm, id, h.HandlePasswordChange, req)
	return rec, bad
}

func passwordIs(t *testing.T, h *Handler, id int64, pw string) bool {
	t.Helper()
	u, err := h.users.GetByID(context.Background(), id)
	if err != nil || u == nil {
		t.Fatalf("GetByID: %v", err)
	}
	ok, _ := auth.VerifyPassword(pw, u.Password)
	return ok
}

func TestTheOwnPasswordCheckOfAPasswordChangeIsThrottled(t *testing.T) {
	h, sm, _, admin := userAdmin(t)
	lang := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	wrong := i18n.T(i18n.Lang(lang), "The current password is not right")
	wait := i18n.T(i18n.Lang(lang), "Too many failed attempts. Try again in %d minutes.")
	busy := i18n.T(i18n.Lang(lang), busySentence)

	// A busy gate first: the owner types the right password and is told so.
	release := holdTheOnlySlot(t)
	for i := range 7 {
		if _, bad := changeOwnPassword(t, h, sm, admin, "passwort123"); bad != busy {
			t.Fatalf("busy attempt %d: flash %q, want the busy sentence", i+1, bad)
		}
	}
	release()
	if !passwordIs(t, h, admin, "passwort123") {
		t.Fatal("a busy change altered the password")
	}

	for i := range 5 {
		if _, bad := changeOwnPassword(t, h, sm, admin, "falsch"); bad != wrong {
			t.Fatalf("wrong password %d: flash %q, want %q (checked, so the busy answers cost nothing)", i+1, bad, wrong)
		}
	}
	_, bad := changeOwnPassword(t, h, sm, admin, "passwort123")
	if bad == wrong || bad == "" || !strings.HasPrefix(bad, strings.SplitN(wait, "%d", 2)[0]) {
		t.Errorf("the right password after five wrong ones: flash %q, want the wait sentence", bad)
	}
	if !passwordIs(t, h, admin, "passwort123") || passwordIs(t, h, admin, "neuespasswort") {
		t.Error("a throttled change altered the password")
	}
}

// IN-05: a stored hash that cannot be read is a fault of the server, not a
// wrong password. The confirmation used to answer "That password is wrong" and
// count it, while the password change, which shares its throttle, answered
// 500 and gave the attempt back. Both now give it back.
func TestAnUnreadableStoredHashIsNotCountedAtTheConfirmation(t *testing.T) {
	h, sm, database, admin := userAdmin(t)
	setHash := func(hash string) {
		t.Helper()
		if _, err := database.Write.ExecContext(context.Background(),
			`UPDATE users SET password = $1 WHERE id = $2`, hash, admin); err != nil {
			t.Fatal(err)
		}
	}
	setHash("kaputt")
	for i := 0; i < auth.ConfirmUserMax+1; i++ {
		req := postForm("/admin/bestaetigen", url.Values{"password": {"passwort123"}, "weiter": {"/admin/users"}}, nil)
		req.RemoteAddr = confirmClient
		var handlerErr error
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), auth.SessionKeyUserID, admin)
			sm.Put(r.Context(), auth.SessionKeyUserRole, user.RoleAdmin)
			handlerErr = h.HandleConfirmPassword(w, r)
		})).ServeHTTP(httptest.NewRecorder(), req)
		if handlerErr == nil {
			t.Fatalf("confirmation %d against an unreadable hash: no error, want a server error", i+1)
		}
	}

	hash, err := auth.HashPassword("passwort123", cheapHashing)
	if err != nil {
		t.Fatal(err)
	}
	setHash(hash)
	if res := postConfirm(t, h, sm, admin, "passwort123"); !res.Elevated {
		t.Errorf("the right password was refused (%d) after server errors: they were counted as wrong passwords", res.Code)
	}
}
