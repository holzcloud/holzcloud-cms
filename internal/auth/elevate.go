package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/alexedwards/scs/v2"
)

// A second password prompt in front of the few actions that cannot be undone.
//
// The threat is not somebody guessing a password — it is the laptop left open
// in a shared office for two minutes. A session is a whole working day long
// because anything shorter makes people write their password on a note; the
// answer is not a shorter session but one question in front of the handful of
// buttons that destroy something.
//
// Deliberately few buttons. A confirmation that appears everywhere is a
// confirmation nobody reads, and then it protects nothing.

// SessionKeyElevated is when the password was last confirmed, as a Unix time.
const SessionKeyElevated = "elevated_at"

// ElevatedFor is how long one confirmation lasts.
//
// Long enough to delete three things in a row without typing it three times,
// short enough that the open laptop has stopped being a way in by the time
// somebody walks past.
const ElevatedFor = 15 * time.Minute

// MarkElevated records that the password was just confirmed.
func MarkElevated(sm *scs.SessionManager, ctx context.Context) {
	sm.Put(ctx, SessionKeyElevated, time.Now().Unix())
}

// Elevated reports whether the password was confirmed recently enough.
func Elevated(sm *scs.SessionManager, ctx context.Context) bool {
	at := sm.GetInt64(ctx, SessionKeyElevated)
	if at == 0 {
		return false
	}
	return time.Since(time.Unix(at, 0)) < ElevatedFor
}

// ConfirmPath is the screen that asks for the password again.
const ConfirmPath = "/admin/bestaetigen"

// RequireFreshPassword sends somebody to the confirmation screen when their
// password confirmation has gone stale.
//
// It does not hold the request and replay it afterwards. That would mean
// keeping a form — files and all — in the session, and the honest version is
// smaller: confirm, land back on the screen you came from, press the button
// again. It now works, because the confirmation is fresh.
func RequireFreshPassword(sm *scs.SessionManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if Elevated(sm, r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, ConfirmPath+"?weiter="+url.QueryEscape(backTo(r)), http.StatusSeeOther)
		})
	}
}

// backTo is the screen to return to: the one the button was on.
//
// Only a path on this server. A Referer is a header anybody can set, and an
// address from a header that ends up in a redirect is how an open redirect is
// built.
func backTo(r *http.Request) string {
	ref := r.Referer()
	if ref == "" {
		return "/admin/"
	}
	u, err := url.Parse(ref)
	if err != nil || u.Path == "" || u.Host != r.Host {
		return "/admin/"
	}
	back := u.Path
	if u.RawQuery != "" {
		back += "?" + u.RawQuery
	}
	return back
}

// SafeReturn narrows a target address to a path inside the administration, for
// the confirmation screen's "weiter" parameter.
func SafeReturn(raw string) string {
	if raw == "" {
		return "/admin/"
	}
	u, err := url.Parse(raw)
	// Absolute, protocol-relative, or anywhere outside the administration: back
	// to the start page. There is nothing else a stranger's address could be
	// good for here.
	if err != nil || u.IsAbs() || u.Host != "" || len(u.Path) < 7 || u.Path[:7] != "/admin/" {
		return "/admin/"
	}
	out := u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}
