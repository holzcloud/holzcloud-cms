// Package localpath holds the one rule the program has for a "local" target:
// a string that may be sent as a Location header or written as an href and
// stays on this origin.
//
// It is a leaf package with no imports from this module because the places
// that need the rule sit on both sides of internal/web: internal/field and
// internal/block cannot import web, and internal/config must stay light.
package localpath

import (
	"net/url"
	"strings"
)

// Is reports whether p is a path on this server (RDR-01). It is the only
// local-path rule of the program; web.LocalPath, the configuration's sign-out
// path, the content-field link check and the block link renderer all call it.
//
// Each clause closes a way for a string that starts with a slash to leave:
//
//   - It must start with "/". "kontakt" has no origin of its own, and
//     "https://…" or "javascript:…" name one.
//   - No byte below 0x20 and no 0x7f. A browser removes tab, line feed and
//     carriage return from a URL before it resolves it, so "/<TAB>/host" is
//     followed as "//host"; net/http would also carry a line break into a
//     header.
//   - No backslash anywhere. A browser reads "\" as "/", so "/\host" is
//     "//host": protocol-relative, somebody else's server.
//   - The second byte is not "/": "//host" is protocol-relative.
//   - The path does not start with an encoded slash or backslash ("/%2f",
//     "/%5c"). A server that decodes before it redirects would turn it into
//     one of the two cases above; it is refused as belt and braces.
//   - It parses as a URL with no scheme, host, opaque part or user.
func Is(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	if strings.HasPrefix(p, "//") {
		return false
	}
	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, "/%2f") || strings.HasPrefix(lower, "/%5c") {
		return false
	}
	u, err := url.Parse(p)
	if err != nil {
		return false
	}
	return u.Scheme == "" && u.Host == "" && u.Opaque == "" && u.User == nil
}
