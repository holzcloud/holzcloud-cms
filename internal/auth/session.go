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
