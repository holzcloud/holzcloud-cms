package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
)

// Middleware is a standard HTTP middleware signature.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in order (first listed = outermost).
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// UserLookup resolves a user ID to its current role. found is false when the
// user no longer exists. Implemented by the admin package over the users table.
type UserLookup func(ctx context.Context, id int64) (role string, found bool, err error)

// RequireAuth redirects to /admin/login if no user_id is in the session.
//
// The session is re-validated against the database on every request: a user who
// has been deleted is logged out immediately, and a role change takes effect at
// once instead of lingering until the session expires (up to 24h).
func RequireAuth(sm *scs.SessionManager, lookup UserLookup) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := sm.GetInt64(r.Context(), SessionKeyUserID)
			if userID == 0 {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			if lookup != nil {
				role, found, err := lookup(r.Context(), userID)
				if err != nil {
					slog.Error("session revalidation failed", "err", err, "user_id", userID)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if !found {
					if err := sm.Destroy(r.Context()); err != nil {
						slog.Error("destroy session for deleted user", "err", err, "user_id", userID)
					}
					http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
					return
				}
				if role != sm.GetString(r.Context(), SessionKeyUserRole) {
					sm.Put(r.Context(), SessionKeyUserRole, role)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns 403 Forbidden if the session role is not "admin".
func RequireAdmin(sm *scs.SessionManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := sm.GetString(r.Context(), SessionKeyUserRole)
			if role != "admin" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WebsiteAccess answers whether a signed-in person may enter a website.
//
// Implemented by the admin package over the users table; nil switches the check
// off, which is what a test router without a database wants.
type WebsiteAccess func(ctx context.Context, userID, websiteID int64) bool

// RequireWebsiteAccess refuses a request for a website this person is not
// assigned to.
//
// Deliberately one middleware over the whole admin mux rather than a check in
// each handler. There are more than sixty routes under /admin/websites/{id},
// they grow with every feature, and a rule enforced in sixty places is a rule
// that is missing in one of them — this project has shipped exactly that bug
// before, with the missing requireAdmin on website deletion.
//
// It therefore reads the address rather than the route: everything under
// /admin/websites/<number> belongs to that website, whatever the rest of the
// path turns out to be. A path without a number (/admin/websites/new) is not
// about one website and is left to the admin check that guards it anyway.
func RequireWebsiteAccess(sm *scs.SessionManager, allowed WebsiteAccess) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			websiteID := websiteIDInPath(r.URL.Path)
			if websiteID == 0 || allowed == nil {
				next.ServeHTTP(w, r)
				return
			}
			userID := sm.GetInt64(r.Context(), SessionKeyUserID)
			if userID == 0 || !allowed(r.Context(), userID, websiteID) {
				http.Error(w, "Diese Website gehört nicht zu deinem Zugang.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// websiteIDInPath is the number in /admin/websites/<id>/…, or zero.
func websiteIDInPath(path string) int64 {
	rest, ok := strings.CutPrefix(path, "/admin/websites/")
	if !ok {
		return 0
	}
	if cut := strings.IndexByte(rest, '/'); cut >= 0 {
		rest = rest[:cut]
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}
