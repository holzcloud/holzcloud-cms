package admin

import (
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/changelog"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// sessionKeySeenVersion caches, in the session, the running version this
// session has already settled with the database. It makes the cost of the
// middleware one read per session per running version instead of one per page.
const sessionKeySeenVersion = "seen_version"

// neverInterrupted are the pages a person is in the middle of something on, and
// where a detour would cost more than the news is worth: second-factor setup
// and its recovery codes, password confirmation, and the consent screen of an
// AI/OAuth sign-in. The screen comes on the next ordinary navigation instead.
var neverInterrupted = []string{"/admin/2fa", "/admin/bestaetigen", "/admin/ai/verbinden"}

// WhatsNewOnUpdate sends a signed-in person to the What's-new screen once after
// the program was updated. running is the version this binary was built as.
//
// The rule itself is changelog.Decide; this is only its application to a
// request. A build that is not a release in CHANGELOG.md returns next
// unchanged: no read, no write, no nagging.
//
// Only a real page navigation is interrupted: GET, not htmx, and a document
// request (Sec-Fetch-Dest "document", or where the browser sends none, an
// Accept that asks for HTML). Anything else passes untouched and leaves the
// record alone, so the screen still comes on the next real navigation.
//
// The version is recorded before the redirect. If recording fails there is no
// redirect, because a redirect whose record failed would loop. Bookkeeping
// never blocks the administration.
func (h *Handler) WhatsNewOnUpdate(running string) func(http.Handler) http.Handler {
	release, isRelease := changelog.Find(running)
	return func(next http.Handler) http.Handler {
		if !isRelease {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isPageNavigation(r) || isNeverInterrupted(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			userID := h.sm.GetInt64(ctx, auth.SessionKeyUserID)
			if userID == 0 || h.sm.GetString(ctx, sessionKeySeenVersion) == release.Version {
				next.ServeHTTP(w, r)
				return
			}

			seen, err := h.users.SeenVersion(ctx, userID)
			if err != nil {
				slog.Error("whats-new: read seen version", "user", userID, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			outcome, version := changelog.Decide(seen, running)
			switch outcome {
			case changelog.Nothing:
				h.sm.Put(ctx, sessionKeySeenVersion, release.Version)
			case changelog.Record:
				h.recordSeen(r, userID, version)
			case changelog.Show:
				if strings.HasPrefix(r.URL.Path, "/admin/neuerungen") {
					// Already looking at it.
					h.recordSeen(r, userID, version)
					break
				}
				if !h.recordSeen(r, userID, version) {
					break
				}
				web.AddVary(w, "HX-Request")
				http.Redirect(w, r, whatsNewURL(version, seen, r.URL.RequestURI()), http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// recordSeen writes the version down and caches it in the session. It reports
// whether the write succeeded.
func (h *Handler) recordSeen(r *http.Request, userID int64, version string) bool {
	if err := h.users.SetSeenVersion(r.Context(), userID, version); err != nil {
		slog.Error("whats-new: record seen version", "user", userID, "error", err)
		return false
	}
	h.sm.Put(r.Context(), sessionKeySeenVersion, version)
	return true
}

func isPageNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet || r.Header.Get("HX-Request") == "true" {
		return false
	}
	if dest := r.Header.Get("Sec-Fetch-Dest"); dest != "" {
		return dest == "document"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func isNeverInterrupted(p string) bool {
	for _, prefix := range neverInterrupted {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// whatsNewURL is the address of one version's screen, carrying the version the
// person was on (seit) and the page they were heading for (weiter).
func whatsNewURL(version, since, next string) string {
	q := url.Values{}
	if since != "" {
		q.Set("seit", since)
	}
	if next != "" {
		q.Set("weiter", next)
	}
	u := "/admin/neuerungen/" + url.PathEscape(version)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// continueTarget is the open-redirect guard for the weiter parameter: only a
// path inside /admin survives, anything else becomes /admin/. A single leading
// slash, no backslash or control character, no scheme or host, and a cleaned
// path of /admin or /admin/….
func continueTarget(raw string) string {
	const fallback = "/admin/"
	if raw == "" || raw[0] != '/' || strings.HasPrefix(raw, "//") {
		return fallback
	}
	for _, c := range raw {
		if c == '\\' || c < 0x20 || c == 0x7f {
			return fallback
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return fallback
	}
	clean := path.Clean(u.Path)
	if clean != "/admin" && !strings.HasPrefix(clean, "/admin/") {
		return fallback
	}
	return raw
}
