package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/totp"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The secret enableSecondFactor writes.
const testSecondFactorSecret = "JBSWY3DPEHPK3PXP"

// newSecondStepAdmin is a handler with the real throttles, a trusted-nothing
// resolver, and one administrator whose authenticator is confirmed.
func newSecondStepAdmin(t *testing.T) (h *Handler, sm *scs.SessionManager, database *db.DB, id int64) {
	t.Helper()
	h, sm, database = newSecondFactorAdmin(t)
	h.SetActivityStore(activity.NewStore(database))
	h.loginThrottle = auth.NewLoginThrottle(10, 100, 15*time.Minute)
	h.clientIP = web.NewClientIPResolver(nil)
	id = seedSecondFactorAccount(t, h, "wer@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)
	return h, sm, database, id
}

// stepResult is what one POST to the second step left behind.
type stepResult struct {
	Code     int
	Location string
	Flash    string
	Cookie   *http.Cookie
	SignedIn int64
	Pending  int64
}

// pendingSession is a session that has passed the password and holds the
// pending id, as a browser would carry it back.
func pendingSession(t *testing.T, sm *scs.SessionManager, id int64) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyPendingUserID, id)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/login", nil))
	c := sessionCookie(t, sm, rec)
	if c == nil {
		t.Fatal("no session cookie for the pending sign-in")
	}
	return c
}

// postCode submits one code. With a cookie the request continues that session;
// without one it runs in a fresh session that already holds the pending id.
func postCode(t *testing.T, h *Handler, sm *scs.SessionManager, remote string, cookie *http.Cookie, id int64, recovery bool, code string) stepResult {
	t.Helper()
	target := auth.VerifyPath
	if recovery {
		target += "?wiederherstellung=1"
	}
	req := postForm(target, url.Values{"code": {code}}, nil)
	req.RemoteAddr = remote
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	var res stepResult
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie == nil {
			sm.Put(r.Context(), auth.SessionKeyPendingUserID, id)
		}
		if err := h.HandleTwoFactorVerify(w, r); err != nil {
			t.Errorf("HandleTwoFactorVerify: %v", err)
		}
		res.Flash = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		res.SignedIn = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		res.Pending = sm.GetInt64(r.Context(), auth.SessionKeyPendingUserID)
	})).ServeHTTP(rec, req)
	res.Code = rec.Code
	res.Location = rec.Header().Get("Location")
	res.Cookie = sessionCookie(t, sm, rec)
	return res
}

func goodCode(t *testing.T) string {
	t.Helper()
	code, err := totp.Code(testSecondFactorSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func flashIs(r *http.Request, sentence string) string {
	return i18n.T(i18n.Lang(r.Context()), sentence)
}

// Six digits are a small space. The second step has its own budget, five per
// account, so a guesser who has the password is stopped after a few misses and
// a correct code afterwards no longer gets through either. Every attempt comes
// from a fresh session, so the budget that stops them is the account's and not
// the pending sign-in's.
func TestSecondFactorCodeGuessingIsThrottled(t *testing.T) {
	h, sm, _, id := newSecondStepAdmin(t)
	const client = "198.51.100.20:4711"
	ctx := httptest.NewRequest(http.MethodGet, "/", nil)
	wrong := flashIs(ctx, "That code is not right.")

	for i := range 5 {
		res := postCode(t, h, sm, client, nil, id, false, "000000")
		if res.Location != auth.VerifyPath || res.Flash != wrong {
			t.Fatalf("wrong code %d: %d to %q, flash %q; want it checked and refused", i+1, res.Code, res.Location, res.Flash)
		}
	}
	// The sixth, even with the right code, is not looked at.
	res := postCode(t, h, sm, client, nil, id, false, goodCode(t))
	if res.Code != http.StatusSeeOther || res.Location != auth.VerifyPath {
		t.Errorf("blocked attempt: %d to %q, want 303 to %s", res.Code, res.Location, auth.VerifyPath)
	}
	if res.SignedIn != 0 {
		t.Error("a correct code signed in after the account's second-step budget was spent")
	}
	if res.Flash == "" || res.Flash == wrong {
		t.Errorf("blocked attempt flash %q, want the wait sentence", res.Flash)
	}
}

// A phished password buys three guesses and then the password has to be typed
// again, which costs a login attempt.
func TestThreeWrongCodesEndThePendingSignIn(t *testing.T) {
	h, sm, _, id := newSecondStepAdmin(t)
	const client = "198.51.100.21:4711"
	ctx := httptest.NewRequest(http.MethodGet, "/", nil)
	wrong := flashIs(ctx, "That code is not right.")
	ended := flashIs(ctx, "Too many wrong codes. Please sign in again.")

	cookie := pendingSession(t, sm, id)
	first := cookie.Value
	for i := range 2 {
		res := postCode(t, h, sm, client, cookie, id, false, "000000")
		if res.Location != auth.VerifyPath || res.Flash != wrong || res.Pending != id {
			t.Fatalf("wrong code %d: %d to %q, flash %q, pending %d", i+1, res.Code, res.Location, res.Flash, res.Pending)
		}
		if res.Cookie != nil {
			cookie = res.Cookie
		}
	}
	res := postCode(t, h, sm, client, cookie, id, false, "000000")
	if res.Code != http.StatusSeeOther || res.Location != "/admin/login" {
		t.Fatalf("the third wrong code: %d to %q, want 303 to /admin/login", res.Code, res.Location)
	}
	if res.Flash != ended {
		t.Errorf("flash %q, want %q", res.Flash, ended)
	}
	if res.Pending != 0 {
		t.Error("the pending sign-in survived three wrong codes")
	}
	if res.Cookie == nil || res.Cookie.Value == first || res.Cookie.Value == cookie.Value {
		t.Error("the session token was not renewed when the pending sign-in ended")
	}

	// Even the right code no longer helps: the visitor starts again at the password.
	cookie = res.Cookie
	res = postCode(t, h, sm, client, cookie, id, false, goodCode(t))
	if res.Location != "/admin/login" || res.SignedIn != 0 {
		t.Errorf("after the pending sign-in ended: %d to %q, signed in %d", res.Code, res.Location, res.SignedIn)
	}
}

func TestRecoveryCodesShareTheSecondFactorBudget(t *testing.T) {
	h, sm, _, id := newSecondStepAdmin(t)
	const client = "198.51.100.22:4711"
	// A real set of printed codes for this account.
	codes, err := h.users.RegenerateRecoveryCodes(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	for i, recovery := range []bool{false, false, true, true, true} {
		code := "000000"
		if recovery {
			code = "wrong-wrong"
		}
		if res := postCode(t, h, sm, client, nil, id, recovery, code); res.SignedIn != 0 || res.Location == "/admin/login" {
			t.Fatalf("attempt %d: %+v", i+1, res)
		}
	}
	res := postCode(t, h, sm, client, nil, id, true, codes[0])
	if res.SignedIn != 0 {
		t.Error("a valid recovery code signed in after two wrong codes and three wrong recovery codes spent the shared budget")
	}
}

func TestASuccessfulCodeClearsOnlyTheAccountKey(t *testing.T) {
	h, sm, _, id := newSecondStepAdmin(t)
	const remote = "198.51.100.23:4711"

	cookie := pendingSession(t, sm, id)
	for range 2 {
		postCode(t, h, sm, remote, cookie, id, false, "000000")
	}
	if res := postCode(t, h, sm, remote, cookie, id, false, goodCode(t)); res.SignedIn != id {
		t.Fatalf("the right code did not sign in: %+v", res)
	}

	// The client key still holds the two failures: eight more reservations fit
	// in its budget of ten, a ninth does not.
	probe := "198.51.100.23"
	throttle := h.loginThrottle.SecondFactor()
	for i := range 8 {
		a, _ := throttle.Reserve(probe, "probe-"+string(rune('a'+i)))
		if a == nil {
			t.Fatalf("client key already full after %d probes: the success cleared or inflated it wrongly", i)
		}
		a.Fail()
	}
	if a, _ := throttle.Reserve(probe, "probe-z"); a != nil {
		t.Error("the client key had room for ten after two failures: a success cleared it")
	}
	// The account key is empty again: all five reservations of its budget fit.
	for i := range 5 {
		a, _ := throttle.Reserve("other-client-"+string(rune('a'+i)), "wer@test.local")
		if a == nil {
			t.Fatalf("account key full after %d probes: the success did not clear it", i)
		}
		a.Fail()
	}
}
