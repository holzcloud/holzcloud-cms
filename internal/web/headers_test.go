package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(mw func(http.Handler) http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	return rec
}

// Nothing may be loaded from a third party at runtime, so every response
// confines the browser to this origin.
func TestSecureHeadersSetsPublicCSP(t *testing.T) {
	rec := serve(SecureHeaders)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy on a public response")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"font-src 'self'",
		"img-src 'self' data:",
		"object-src 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("public CSP missing %q: %s", want, csp)
		}
	}
	// No directive may open the policy to arbitrary origins.
	for _, forbidden := range []string{"*", "https:", "unsafe-eval"} {
		for _, directive := range strings.Split(csp, ";") {
			if strings.Contains(directive, "src") && strings.Contains(directive, forbidden) {
				t.Errorf("public CSP allows %q in %q", forbidden, strings.TrimSpace(directive))
			}
		}
	}

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff missing")
	}
	if rec.Header().Get("Referrer-Policy") == "" {
		t.Error("Referrer-Policy missing")
	}
}

// The admin policy is the stricter one and must win where both apply.
func TestAdminHeadersTightenThePolicy(t *testing.T) {
	rec := serve(func(next http.Handler) http.Handler {
		return SecureHeaders(AdminHeaders(next))
	})

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("admin CSP did not override the public one: %s", csp)
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("admin CSP must not allow inline script: %s", csp)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("admin responses must not be framable")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("admin responses must not be cached")
	}
}

// Neither policy may permit a script from another origin.
func TestNoCSPAllowsThirdPartyScripts(t *testing.T) {
	for name, csp := range map[string]string{"public": publicCSP, "admin": adminCSP} {
		if !strings.Contains(csp, "script-src 'self'") {
			t.Errorf("%s CSP does not pin script-src to 'self': %s", name, csp)
		}
		if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("%s CSP allows inline script: %s", name, csp)
		}
	}
}

// A shop with no payment provider keeps the policy it always had.
func TestPublicCSPWithoutPaymentsIsUnchanged(t *testing.T) {
	if got := PublicCSP(); got != publicCSP {
		t.Errorf("PublicCSP() = %q, erwartet %q", got, publicCSP)
	}
	if strings.Contains(PublicCSP(), "payrexx") {
		t.Error("die Standardrichtlinie nennt einen Zahlungsanbieter")
	}
}

// The handover to the payment page is a redirect that answers a form POST, and
// some browsers check that redirect against form-action. Without the provider
// in there, TWINT on an iPhone dies at the moment of the handover with nothing
// in the server log to show for it.
func TestPublicCSPAllowsTheHandoverToTheProvider(t *testing.T) {
	csp := PublicCSP(PaymentFormAction)

	if !strings.Contains(csp, "form-action 'self' "+PaymentFormAction+";") {
		t.Errorf("form-action erlaubt den Zahlungsanbieter nicht: %s", csp)
	}
	// Everything else has to stay exactly as strict.
	for _, must := range []string{
		"default-src 'self'", "script-src 'self'", "connect-src 'self'",
		"img-src 'self' data:", "font-src 'self'", "object-src 'none'",
	} {
		if !strings.Contains(csp, must) {
			t.Errorf("die Richtlinie hat %q verloren: %s", must, csp)
		}
	}
	// The provider may be navigated to, never loaded from.
	if strings.Contains(csp, "default-src 'self' https") ||
		strings.Contains(csp, "script-src 'self' https") ||
		strings.Contains(csp, "connect-src 'self' https") {
		t.Errorf("der Zahlungsanbieter darf nur ein Navigationsziel sein: %s", csp)
	}
}

// An empty origin must not leave a dangling space in the header.
func TestPublicCSPIgnoresEmptyOrigins(t *testing.T) {
	if got := PublicCSP("", ""); got != publicCSP {
		t.Errorf("PublicCSP(\"\") = %q", got)
	}
}
