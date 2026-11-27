package admin

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

const loginTestPassword = "correct-horse-battery-staple"

// newLoginThrottleAdmin is the ordinary test handler with the real login
// limits (10 per client, 100 per account), a protocol, and a resolver that
// trusts no proxy, so a request counts under its own RemoteAddr.
func newLoginThrottleAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	h, sm, database := newSecondFactorAdmin(t)
	h.SetActivityStore(activity.NewStore(database))
	h.loginThrottle = auth.NewLoginThrottle(10, 100, 15*time.Minute)
	h.clientIP = web.NewClientIPResolver(nil)
	return h, sm, database
}

// loginResult is what one POST to the login form left behind. The session is
// read inside the request's own session scope, so no cookie has to be carried.
type loginResult struct {
	Code     int
	Location string
	Header   http.Header
	Body     string
	// Flash is the error flash the response left in the session; Wrong says it
	// is the sentence of a checked and refused password, which is how a test
	// tells an attempt that was checked from one that was throttled.
	Flash    string
	Wrong    bool
	SignedIn int64
	Pending  int64
}

func postLogin(t *testing.T, h *Handler, sm *scs.SessionManager, remote, email, password string) loginResult {
	t.Helper()
	req := postForm("/admin/login", url.Values{"email": {email}, "password": {password}}, nil)
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	var res loginResult
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.HandleLogin(w, r); err != nil {
			t.Errorf("HandleLogin: %v", err)
		}
		res.Flash = sm.GetString(r.Context(), auth.SessionKeyFlashError)
		res.Wrong = res.Flash == i18n.T(i18n.Lang(r.Context()), "Email address or password is not right")
		res.SignedIn = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		res.Pending = sm.GetInt64(r.Context(), auth.SessionKeyPendingUserID)
	})).ServeHTTP(rec, req)
	res.Code = rec.Code
	res.Location = rec.Header().Get("Location")
	res.Header = rec.Header()
	res.Body = rec.Body.String()
	return res
}

// failedLoginRows counts the protocol rows of refused sign-ins. Every address,
// known or not, writes one, and only a request that reached the password check
// does.
func failedLoginRows(t *testing.T, database *db.DB) int {
	t.Helper()
	n := 0
	for _, e := range activityRows(t, database) {
		if e.Action == activity.ActionAuthLoginFail {
			n++
		}
	}
	return n
}

func TestAParallelBurstOfWrongLoginsStaysWithinTheLimit(t *testing.T) {
	h, sm, database := newLoginThrottleAdmin(t)
	// A check that takes a few tens of milliseconds, so that the burst really is
	// in flight together: with a microsecond hash each request would finish
	// before the next began and the old check-then-record order would pass too.
	h.argon2Params = auth.Argon2Params{Memory: 32 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	const burst = 30
	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]loginResult, burst)
	for i := range burst {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			results[i] = postLogin(t, h, sm, "198.51.100.7:4711", "ghost@test.local", "wrong-password")
		}()
	}
	start.Done()
	done.Wait()

	if got := failedLoginRows(t, database); got != 10 {
		t.Errorf("%d wrong logins reached the password check, want exactly 10", got)
	}
	for i, res := range results {
		if res.Code != http.StatusSeeOther || res.Location != "/admin/login" {
			t.Errorf("response %d: %d to %q, want 303 to /admin/login", i, res.Code, res.Location)
		}
	}
}

func TestASuccessfulLoginLeavesTheClientKeyAlone(t *testing.T) {
	h, sm, _ := newLoginThrottleAdmin(t)
	owned := seedSecondFactorAccount(t, h, "mine@test.local", user.RoleEditor)
	const client = "198.51.100.8:4711"

	for i := range 9 {
		if res := postLogin(t, h, sm, client, "victim@test.local", "guess"); !res.Wrong {
			t.Fatalf("guess %d was not checked: flash %q", i+1, res.Flash)
		}
	}
	// The attacker's own account, the right password.
	if res := postLogin(t, h, sm, client, "mine@test.local", loginTestPassword); res.SignedIn != owned {
		t.Fatalf("the right password did not sign in: %+v", res)
	}
	// The ninth failure stands: the tenth guess is checked, the eleventh is not.
	if res := postLogin(t, h, sm, client, "victim@test.local", "guess"); !res.Wrong {
		t.Errorf("the tenth guess was not checked: flash %q", res.Flash)
	}
	res := postLogin(t, h, sm, client, "victim@test.local", "guess")
	if res.Wrong || res.Flash == "" {
		t.Errorf("the eleventh guess was checked after a success had cleared the client key: flash %q", res.Flash)
	}
}

func TestLoginFromOneSlashSixtyFourSharesOneBudget(t *testing.T) {
	h, sm, _ := newLoginThrottleAdmin(t)

	for i := range 10 {
		if res := postLogin(t, h, sm, "[2001:db8:1:2::1]:4711", "ghost@test.local", "guess"); !res.Wrong {
			t.Fatalf("guess %d was not checked: flash %q", i+1, res.Flash)
		}
	}
	if res := postLogin(t, h, sm, "[2001:db8:1:2::99]:4711", "ghost@test.local", "guess"); res.Wrong || res.Flash == "" {
		t.Errorf("a second address of the same /64 had a budget of its own: flash %q", res.Flash)
	}
	if res := postLogin(t, h, sm, "[2001:db8:1:3::1]:4711", "ghost@test.local", "guess"); !res.Wrong {
		t.Errorf("another /64 was throttled: flash %q", res.Flash)
	}
}

func TestABusyLoginAnswers503AndIsNotCounted(t *testing.T) {
	h, sm, database := newLoginThrottleAdmin(t)
	seedSecondFactorAccount(t, h, "owner@test.local", user.RoleAdmin)
	const client = "198.51.100.9:4711"

	gate := auth.NewGate(1, 10*time.Millisecond, 10*time.Millisecond)
	previous := auth.SetArgon2Gate(gate)
	t.Cleanup(func() { auth.SetArgon2Gate(previous) })
	release, err := gate.Acquire(0)
	if err != nil {
		t.Fatal(err)
	}

	lang := i18n.Lang(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	busy := template.HTMLEscapeString(i18n.T(lang, "The server is busy checking other passwords. Please try again in a moment."))
	for _, c := range []struct{ name, email, password string }{
		{"a known account with the right password", "owner@test.local", loginTestPassword},
		{"an unknown address", "ghost@test.local", "anything"},
	} {
		// Many more than the budget: none of them may take anything from it.
		for range 12 {
			res := postLogin(t, h, sm, client, c.email, c.password)
			if res.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s: status %d, want 503", c.name, res.Code)
			}
			if got := res.Header.Get("Retry-After"); got != "2" {
				t.Errorf("%s: Retry-After %q, want 2", c.name, got)
			}
			if !strings.Contains(res.Body, busy) {
				t.Errorf("%s: the login page does not carry the busy sentence", c.name)
			}
			if res.SignedIn != 0 || res.Pending != 0 {
				t.Errorf("%s: a busy answer left a session behind: %+v", c.name, res)
			}
		}
	}
	if rows := activityRows(t, database); len(rows) != 0 {
		t.Errorf("a busy answer wrote %d protocol rows", len(rows))
	}

	release()
	for i := range 10 {
		if res := postLogin(t, h, sm, client, "ghost@test.local", "guess"); !res.Wrong {
			t.Fatalf("guess %d after the gate cleared was not checked: flash %q", i+1, res.Flash)
		}
	}
	if res := postLogin(t, h, sm, client, "ghost@test.local", "guess"); res.Wrong || res.Flash == "" {
		t.Errorf("the eleventh guess was checked: the busy answers had taken part of the budget")
	}
}

// WR-04: a refused sign-in does the same work whether the address exists or
// not. Only the unknown address used to write a protocol row, an INSERT on the
// one write connection that a known address skipped, so the time of the answer
// told the two apart (the dummy hash exists to prevent exactly that), and the
// failed sign-ins an operator most needs to see, against real accounts, were
// not recorded at all.
func TestARefusedSignInWritesTheSameProtocolRowForKnownAndUnknownAddresses(t *testing.T) {
	h, sm, database := newLoginThrottleAdmin(t)
	id := seedSecondFactorAccount(t, h, "known@test.local", user.RoleEditor)

	for _, email := range []string{"ghost@test.local", "Known@test.local"} {
		before := failedLoginRows(t, database)
		if res := postLogin(t, h, sm, "198.51.100.31:4711", email, "wrong-wrong-wrong"); !res.Wrong {
			t.Fatalf("%s: not refused as a wrong password: %+v", email, res)
		}
		if got := failedLoginRows(t, database) - before; got != 1 {
			t.Errorf("%s: %d protocol rows for one refused sign-in, want 1 for every address", email, got)
		}
	}

	rows := activityRows(t, database)
	last := rows[len(rows)-1]
	if last.Action != activity.ActionAuthLoginFail || last.ActorEmail != "known@test.local" || last.EntityID != id {
		t.Errorf("the row of a refused sign-in to a real account is %+v, want the typed address and the account", last)
	}
}
