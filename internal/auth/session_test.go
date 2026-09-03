package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// newSession creates a stored session for userID and returns its token.
//
// The token only exists once LoadAndSave commits the session on the way out, so
// it is read from the Set-Cookie header rather than from inside the handler.
func newSession(t *testing.T, sm *scs.SessionManager, userID int64) string {
	t.Helper()

	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, userID)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sm.Cookie.Name {
			return cookie.Value
		}
	}
	t.Fatal("no session cookie was issued")
	return ""
}

// sessionExists reports whether a token is still in the store.
func sessionExists(t *testing.T, sm *scs.SessionManager, token string) bool {
	t.Helper()
	_, found, err := sm.Store.Find(token)
	if err != nil {
		t.Fatalf("store lookup: %v", err)
	}
	return found
}

// Changing a password is how someone locks out an attacker who already holds a
// session cookie, so the other sessions have to go.
func TestDestroyUserSessionsEndsEveryOtherSession(t *testing.T) {
	sm := scs.New()
	sm.Store = memstore.New()

	victimA := newSession(t, sm, 1)
	victimB := newSession(t, sm, 1)
	other := newSession(t, sm, 2)

	if err := DestroyUserSessions(context.Background(), sm, 1, ""); err != nil {
		t.Fatalf("DestroyUserSessions: %v", err)
	}

	if sessionExists(t, sm, victimA) || sessionExists(t, sm, victimB) {
		t.Error("a session of the target user survived")
	}
	if !sessionExists(t, sm, other) {
		t.Error("another user's session was destroyed")
	}
}

// A user changing their own password must stay signed in.
func TestDestroyUserSessionsKeepsTheExemptedToken(t *testing.T) {
	sm := scs.New()
	sm.Store = memstore.New()

	keep := newSession(t, sm, 1)
	drop := newSession(t, sm, 1)

	if err := DestroyUserSessions(context.Background(), sm, 1, keep); err != nil {
		t.Fatalf("DestroyUserSessions: %v", err)
	}

	if !sessionExists(t, sm, keep) {
		t.Error("the current session was destroyed")
	}
	if sessionExists(t, sm, drop) {
		t.Error("the other session survived")
	}
}

func TestDestroyUserSessionsWithNoMatchesIsANoOp(t *testing.T) {
	sm := scs.New()
	sm.Store = memstore.New()

	token := newSession(t, sm, 2)

	if err := DestroyUserSessions(context.Background(), sm, 99, ""); err != nil {
		t.Fatalf("DestroyUserSessions: %v", err)
	}
	if !sessionExists(t, sm, token) {
		t.Error("an unrelated session was destroyed")
	}
}
