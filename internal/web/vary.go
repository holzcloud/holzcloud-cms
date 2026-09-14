package web

import (
	"net/http"
	"strings"
)

// AddVary names one more request header this answer depends on, without
// throwing away the ones already named.
//
// This exists because `w.Header().Set("Vary", "HX-Request")` was written in six
// places, and Set REPLACES. The session middleware adds `Vary: Cookie` at the
// top of every request; every one of those six lines then deleted it. On the
// public side that was not cosmetic: a password-protected page, once unlocked,
// went out as `Cache-Control: public, max-age=300` with a Vary that no longer
// mentioned the cookie the access depends on — so a shared cache in front of
// this binary could hand the page to somebody who never entered the password.
// The comment above serveGate had named exactly that danger and guarded the
// other half of it.
//
// It also folds duplicates, which is why admin answers no longer carry
// `Vary: Cookie` twice: the session library and the CSRF library each add it,
// both correctly, neither knowing about the other.
//
// A field-name comparison is enough — Vary field names are HTTP tokens, which
// are case-insensitive, and "*" is left exactly as it is because a Vary of "*"
// means "never reuse this" and must not be diluted by anything standing beside
// it.
func AddVary(w http.ResponseWriter, names ...string) {
	seen := make(map[string]bool, 4)
	var out []string
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key := strings.ToLower(part)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, part)
		}
	}
	for _, v := range w.Header().Values("Vary") {
		add(v)
	}
	for _, n := range names {
		add(n)
	}
	if len(out) == 0 {
		return
	}
	if seen["*"] {
		w.Header().Set("Vary", "*")
		return
	}
	w.Header().Set("Vary", strings.Join(out, ", "))
}
