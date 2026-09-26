package auth

import (
	"context"
	"net/http"
	"time"

	"crypto/sha256"
	"encoding/hex"
	"github.com/alexedwards/scs/v2"
	"sort"
)

// Session key constants used throughout the application.
const (
	SessionKeyUserID       = "user_id"
	SessionKeyUserRole     = "user_role"
	SessionKeyUserEmail    = "user_email"
	SessionKeyFlashError   = "flash_error"
	SessionKeyFlashSuccess = "flash_success"
	// SessionKeyFlashWarning completes the three. It was missing while
	// internal/web wrote all three as literals, which is how two sources for
	// one key stay in step: by nobody using the first one.
	SessionKeyFlashWarning = "flash_warning"

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

	// SessionKeyViaOIDC records that the session was established through
	// OpenID Connect: the identity provider signed a token for this person and
	// the browser brought it here. It is the counterpart of SessionKeyViaSSO
	// for the second way in, set in one place (admin's OIDC callback, after
	// completeLogin), removed by completeLogin on every other sign-in, and
	// read together with its own switch — the second factor is not asked for
	// again, and switching OpenID Connect off ends every session it made.
	//
	// Unlike a forward-auth session it is not re-checked on every request:
	// nothing arrives on later requests to re-check it against. Its rights are
	// the ones the groups gave at the sign-in, until the session ends.
	SessionKeyViaOIDC = "via_oidc"
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

// SessionKeyDevice is the browser's own description of itself at sign-in, and
// SessionKeySignedInAt the moment, as Unix seconds. Both exist for the account
// screen, which lists where a person is signed in so that a lost phone can be
// signed out from the desk. Neither decides anything.
const (
	SessionKeyDevice     = "device"
	SessionKeySignedInAt = "signed_in_at"
)

// UserSession is one place a person is signed in.
type UserSession struct {
	// ID names the session without being it: a hash of the token, so the
	// account screen can offer "sign out there" without ever putting a
	// session token into a page.
	ID         string
	Device     string
	SignedInAt time.Time
	Current    bool
}

// SessionID is the public name of a session token.
func SessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// ListUserSessions reports every stored session of one account, the current
// one first and the rest newest first.
func ListUserSessions(ctx context.Context, sm *scs.SessionManager, userID int64, currentToken string) ([]UserSession, error) {
	var out []UserSession
	err := sm.Iterate(ctx, func(sessionCtx context.Context) error {
		if sm.GetInt64(sessionCtx, SessionKeyUserID) != userID {
			return nil
		}
		token := sm.Token(sessionCtx)
		s := UserSession{
			ID:      SessionID(token),
			Device:  sm.GetString(sessionCtx, SessionKeyDevice),
			Current: token == currentToken,
		}
		if at := sm.GetInt64(sessionCtx, SessionKeySignedInAt); at > 0 {
			s.SignedInAt = time.Unix(at, 0)
		}
		out = append(out, s)
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].SignedInAt.After(out[j].SignedInAt)
	})
	return out, err
}

// DestroyUserSession ends one session of one account, named by its SessionID.
// The account is part of the question: a hash a person copied from their own
// screen cannot end anybody else's session.
func DestroyUserSession(ctx context.Context, sm *scs.SessionManager, userID int64, id string) (bool, error) {
	found := false
	err := sm.Iterate(ctx, func(sessionCtx context.Context) error {
		if sm.GetInt64(sessionCtx, SessionKeyUserID) != userID || SessionID(sm.Token(sessionCtx)) != id {
			return nil
		}
		found = true
		return sm.Destroy(sessionCtx)
	})
	return found, err
}
