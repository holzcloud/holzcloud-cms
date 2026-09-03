package activity

import "strings"

// deniedKeys are the metadata keys that never reach the database.
//
// A caller is allowed to hand Store.Log the form values of a request as they
// came in — that is the convenient thing to do, and convenience is what gets
// used. The price is that a password field would land in a table built to be
// read later, by people, at leisure. So the filter sits between, and the
// convenience stays.
//
// Stored in lower case; incoming keys are lower-cased before the lookup, so
// "Password" and "PASSWORD" are caught with "password".
var deniedKeys = map[string]struct{}{
	"password":         {},
	"new_password":     {},
	"old_password":     {},
	"confirm_password": {},
	"password_confirm": {},
	"token":            {},
	"csrf":             {},
	"csrf_token":       {},
	"_csrf":            {},
}

// sanitize returns a copy of m without the denied keys. A nil map becomes an
// empty one, so the caller never has to check before marshaling.
//
// The input is not touched: a caller may pass a map it still uses afterwards.
func sanitize(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, banned := deniedKeys[strings.ToLower(k)]; banned {
			continue
		}
		out[k] = v
	}
	return out
}
