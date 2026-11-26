// Package localpath holds the shared rule for a "local" target: a string that
// may be sent as a Location header or written as an href and stays on this
// origin. It is the rule of the public-side redirects and of the content sinks
// (fields, blocks, the sign-out path). The admin keeps rules of its own that
// also demand an /admin/ prefix or compare origins: admin.continueTarget,
// auth.SafeReturn and value.sameOrigin, and the back and redirect targets in
// the admin menu handlers. They are safe as written and are not routed here.
//
// It is a leaf package with no imports from this module because the places
// that need the rule sit on both sides of internal/web: internal/field and
// internal/block cannot import web, and internal/config must stay light.
package localpath

import (
	"net/url"
	"strings"
)

// Is reports whether p is a path on this server (RDR-01). web.LocalPath, the
// configuration's sign-out path, the content-field link check and the block
// link renderer all call it; the admin-side rules named in the package comment
// are separate.
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
//   - It parses as a URL with no scheme, host, opaque part or user. The parse is
//     a shape check, not a check of the escaping: a "%" that is not followed by
//     two hex digits ("/aktion-50%") is kept by a browser as it is, is on this
//     origin, and does not make the path refused. Such a "%" is escaped before
//     the parse, which is safe because the encoded-slash check above has
//     already run on the raw string.
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
	u, err := url.Parse(strings.ReplaceAll(p, "%", "%25"))
	if err != nil {
		return false
	}
	return u.Scheme == "" && u.Host == "" && u.Opaque == "" && u.User == nil
}
