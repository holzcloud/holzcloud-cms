package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Six digits are a small space. The code check shares the login throttle, keyed
// by account, so a guesser who has the password is stopped after a few misses
// and a correct code afterwards no longer gets through either.
func TestSecondFactorCodeGuessingIsThrottled(t *testing.T) {
	h, sm, database := newSecondFactorAdmin(t)
	h.loginThrottle = auth.NewLoginThrottle(100, 3, time.Minute)
	h.clientIP = web.NewClientIPResolver(nil)
	id := seedSecondFactorAccount(t, h, "wer@test.local", user.RoleAdmin)
	enableSecondFactor(t, database, id)

	attempt := func() *httptest.ResponseRecorder {
		req := postForm("/admin/2fa", url.Values{"code": {"000000"}}, nil)
		rec := httptest.NewRecorder()
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), auth.SessionKeyPendingUserID, id)
			if err := h.HandleTwoFactorVerify(w, r); err != nil {
				t.Errorf("handler: %v", err)
			}
		})).ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 3; i++ {
		attempt()
	}
	if h.loginThrottle.Allowed(h.clientIP.ClientIP(httptest.NewRequest("GET", "/", nil)), "2fa:wer@test.local") {
		t.Error("three wrong codes did not close the account's throttle")
	}
	// The fourth attempt is not even looked at.
	rec := attempt()
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != auth.VerifyPath {
		t.Errorf("blocked attempt: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
}
