package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
)

// SessionKeyPendingUserID holds the account that passed the password check but
// has not yet presented a code.
//
// A separate key rather than user_id with a flag beside it: everything that
// grants access reads user_id, so a half-authenticated session that set it
// would be a session that is signed in until some other check happens to
// notice. Here the session simply has no user until the second factor is done.
const SessionKeyPendingUserID = "pending_user_id"

// SetupPath is where an account that must set up two-factor is sent.
const SetupPath = "/admin/2fa/einrichten"

// VerifyPath is where a half-authenticated session is sent.
const VerifyPath = "/admin/2fa"

// SecondFactorState is what the enforcement middleware needs to know about an
// account.
type SecondFactorState struct {
	Role string
	// Enabled is true once the account has confirmed an authenticator.
	Enabled bool
}

// SecondFactorLookup reads an account's two-factor state.
type SecondFactorLookup func(ctx context.Context, id int64) (SecondFactorState, error)

// MustHaveSecondFactor decides who is required to set one up.
//
// Administrators are, because an administrator can change every password on the
// installation, upload a template and reach every site. Editors are not: they
// can be given the choice, and forcing a second factor on someone who edits
// opening hours is how shared logins get created.
func MustHaveSecondFactor(role string) bool { return role == "admin" }

// RequireSecondFactor sends an account that owes a second factor to the setup
// page, and lets everything else through.
//
// It sits inside RequireAuth, so by the time it runs the session has a user.
func RequireSecondFactor(sm *scs.SessionManager, lookup SecondFactorLookup) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lookup == nil || isSecondFactorPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			userID := sm.GetInt64(r.Context(), SessionKeyUserID)
			if userID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			state, err := lookup(r.Context(), userID)
			if err != nil {
				slog.Error("second factor lookup failed", "err", err, "user_id", userID)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if MustHaveSecondFactor(state.Role) && !state.Enabled {
				http.Redirect(w, r, SetupPath, http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSecondFactorPath reports whether a path must stay reachable while an
// account still owes its second factor.
//
// Signing out has to work too: an account that cannot finish the setup and
// cannot leave is an account stuck on one screen.
func isSecondFactorPath(path string) bool {
	return strings.HasPrefix(path, "/admin/2fa") || path == "/admin/logout"
}
