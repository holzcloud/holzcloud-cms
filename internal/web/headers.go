package web

import "net/http"

// adminCSP locks the admin UI down to same-origin resources.
//
// script-src is 'self' only — the admin templates contain no inline scripts, so
// an injected <script> cannot execute even if some field escapes escaping.
// style-src needs 'unsafe-inline' because the templates use style="..."
// attributes; inline styles are not an execution vector.
const adminCSP = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'none'; " +
	"object-src 'none'"

// publicCSP applies to the public site.
//
// Nothing may be loaded from a third party at runtime (see CLAUDE.md), so the
// public site is confined to its own origin exactly like the admin UI. It is
// slightly looser in two places: user templates may carry style attributes, and
// same-origin framing is allowed so a site can embed its own pages.
const publicCSP = publicCSPPrefix + "form-action 'self'; " + publicCSPSuffix

const publicCSPPrefix = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; "

const publicCSPSuffix = "frame-ancestors 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'"

// PaymentFormAction is the origin a checkout may hand the customer over to.
//
// This is not a hole in "nothing loads at runtime". Nothing is loaded: the
// customer navigates away to pay and comes back, which is the outbound-link
// case CLAUDE.md leaves open, and it is the whole reason card details never
// touch this server.
//
// It has to be in form-action because the checkout answers its own POST with a
// redirect to the provider, and a redirect after a form submission is checked
// against form-action by some browsers — Safari among them. Without this the
// TWINT payment on an iPhone dies silently at the moment of the handover, and
// nothing in the server log says why.
const PaymentFormAction = "https://*.payrexx.com"

// PublicCSP builds the public policy.
//
// extraFormAction is empty on an installation that takes no online payments,
// which leaves the policy exactly as strict as it was before payments existed.
func PublicCSP(extraFormAction ...string) string {
	action := "form-action 'self'"
	for _, origin := range extraFormAction {
		if origin != "" {
			action += " " + origin
		}
	}
	return publicCSPPrefix + action + "; " + publicCSPSuffix
}

// SecureHeaders sets the response headers that apply to every route, including
// the public Content-Security-Policy.
//
// It runs outermost, so a handler further in can tighten the policy for its own
// response: AdminHeaders swaps in the stricter admin policy, and media
// responses swap in a sandbox policy for SVG and PDF.
func SecureHeaders(next http.Handler) http.Handler {
	return SecureHeadersWith(publicCSP)(next)
}

// SecureHeadersWith is SecureHeaders with a policy chosen at startup, so an
// installation that takes online payments can allow the handover to the
// provider and one that does not stays as tight as it always was.
func SecureHeadersWith(csp string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			next.ServeHTTP(w, r)
		})
	}
}

// AdminHeaders adds the strict admin Content-Security-Policy and clickjacking
// protection on top of SecureHeaders.
func AdminHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", adminCSP)
		h.Set("X-Frame-Options", "DENY")
		// Admin responses are per-user; never let a shared cache keep them.
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
