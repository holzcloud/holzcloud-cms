package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

// Session key constants used throughout the application.
const (
	SessionKeyUserID       = "user_id"
	SessionKeyUserRole     = "user_role"
	SessionKeyUserEmail    = "user_email"
	SessionKeyFlashError   = "flash_error"
	SessionKeyFlashSuccess = "flash_success"

	// SessionKeyViaSSO records HOW this session was established, not who
	// established it: true means the sign-in came through forward
	// authentication rather than through the password form.
	//
	// It is set in exactly one place — the forward-auth sign-in in package
	// admin, after completeLogin — and removed by completeLogin on every other
	// sign-in, because scs's RenewToken keeps every value and a password
	// sign-in would otherwise inherit it. It is read by the second-factor
	// decision, which must not demand an authenticator the identity provider
	// has already asked for; by the sign-out, which has to end the session at
	// the identity provider as well; and by the forward-auth sign-in itself,
	// which ends such a session when single sign-on is switched off. Every
	// reader asks together with the switch: a mark that outlived the switch
	// used to keep its exemption from the second factor (Phase 10 code review,
	// WR-04).
	//
	// A session reached by password never carries it, and scs's GetBool
	// answers false for a key that is not there, so the absent case needs no
	// branch anywhere.
	SessionKeyViaSSO = "via_sso"

	// SessionKeySSOUsername is the identity a single sign-on session was
	// established for. A request on that session vouched for a different
	// identity — somebody else signed in at the identity provider on the same
	// browser — ends the session instead of running as its owner, and a
	// request vouched for the same identity has its rights re-applied, so a
	// demotion at the identity provider reaches a session that is still open.
	SessionKeySSOUsername = "sso_username"
)

// DestroyUserSessions ends every stored session belonging to userID, except the
// one whose token is exceptToken (pass "" to end all of them).
//
// Call it after a password change so a stolen session cannot outlive the
// credential it was obtained with. It walks the store rather than querying by
// user, because scs stores sessions as opaque blobs keyed only by token — for
// the handful of sessions a self-hosted CMS has, that is cheap enough and keeps
// the schema unchanged.
func DestroyUserSessions(ctx context.Context, sm *scs.SessionManager, userID int64, exceptToken string) error {
	return sm.Iterate(ctx, func(sessionCtx context.Context) error {
		if sm.GetInt64(sessionCtx, SessionKeyUserID) != userID {
			return nil
		}
		if exceptToken != "" && sm.Token(sessionCtx) == exceptToken {
			return nil
		}
		return sm.Destroy(sessionCtx)
	})
}

// NewSessionManager creates a configured SCS session manager.
// Pass the SQLiteStore and the Secure flag from config.
func NewSessionManager(store *SQLiteStore, secure bool) *scs.SessionManager {
	sm := scs.New()
	sm.Store = store
	sm.Lifetime = 24 * time.Hour
	sm.IdleTimeout = 4 * time.Hour
	sm.Cookie.Name = "holzcloud_session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = secure
	return sm
}
