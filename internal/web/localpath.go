package web

import (
	"net/url"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/localpath"
)

// LocalPath checks a target that is meant to stay on this site. The public
// redirects (price switch, plugin Location, stored redirects, admin redirect
// save) go through it, so they share one rule (localpath.Is) and not one per
// handler. The admin's own return and continue targets (auth.SafeReturn,
// admin.continueTarget) do not: they also require an /admin/ prefix.
//
// It returns the checked string, so a caller cannot redirect to the unchecked
// one by mistake: use the first result, and only when the second is true.
func LocalPath(target string) (string, bool) {
	if localpath.Is(target) {
		return target, true
	}
	return "", false
}

// RedirectTarget checks the target of a stored redirect before it is sent as a
// Location header. A stored redirect may name an absolute http(s) address on
// purpose (TestRedirectsKeepTheLanguage holds that), but a target that looks
// like a path and leaves the site is refused (RDR-02): "//host" and "/\host"
// are read by a browser as another server.
//
// Like LocalPath it returns the checked string; use it only when the second
// result is true.
func RedirectTarget(to string) (string, bool) {
	if checked, ok := LocalPath(to); ok {
		return checked, true
	}
	for i := 0; i < len(to); i++ {
		if c := to[i]; c < 0x20 || c == 0x7f || c == '\\' {
			return "", false
		}
	}
	u, err := url.Parse(to)
	if err != nil || u.User != nil || u.Host == "" {
		return "", false
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return "", false
	}
	return to, true
}
