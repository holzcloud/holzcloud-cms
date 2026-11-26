package public

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// answerWithPolicy writes a plugin answer onto a recorder that already carries
// the policy the server sets before any handler runs.
func answerWithPolicy(out *plugin.RequestOut) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Security-Policy", web.PublicCSP(web.PaymentFormAction))
	writePluginResponse(rec, out)
	return rec
}

// A plugin's media type decides what the browser does with the body, and
// text/javascript is a script on this origin under script-src 'self'. Only a
// closed list of types leaves the host as sent; the rest is plain text.
func TestAPluginAnswerCannotBeScript(t *testing.T) {
	const plain = "text/plain; charset=utf-8"

	for _, ct := range []string{
		"text/javascript", "TEXT/JavaScript; charset=utf-8", "application/javascript",
		"application/ecmascript", "image/svg+xml", "text/css", "text/html/x", ";;",
		"text/html-sandboxed", "application/xhtml+xml", "text/html, text/javascript",
	} {
		rec := answerWithPolicy(&plugin.RequestOut{Handled: true, ContentType: ct, Body: "alert(1)"})
		if got := rec.Header().Get("Content-Type"); got != plain {
			t.Errorf("ContentType %q was sent as %q, want %q", ct, got, plain)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") || strings.Contains(csp, "script-src 'self'") {
			t.Errorf("ContentType %q: policy %q", ct, csp)
		}
	}

	for ct, want := range map[string]string{
		"text/html; charset=utf-8":        "text/html; charset=utf-8",
		"text/plain":                      "text/plain",
		"text/csv; charset=utf-8":         "text/csv; charset=utf-8",
		"application/json":                "application/json",
		"application/xml":                 "application/xml",
		"text/xml":                        "text/xml",
		"application/rss+xml":             "application/rss+xml",
		"application/atom+xml":            "application/atom+xml",
		"text/calendar":                   "text/calendar",
		"":                                "text/html; charset=utf-8",
		"Text/HTML; Charset=ISO-8859-1":   "text/html; charset=ISO-8859-1",
		"application/json; charset=utf-8": "application/json; charset=utf-8",
	} {
		rec := answerWithPolicy(&plugin.RequestOut{Handled: true, ContentType: ct, Body: "x"})
		if got := rec.Header().Get("Content-Type"); got != want {
			t.Errorf("ContentType %q was sent as %q, want %q", ct, got, want)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") {
			t.Errorf("ContentType %q: policy %q", ct, csp)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("ContentType %q: nosniff = %q", ct, got)
		}
	}

	// A redirect carries the policy too: one rule is easier to keep than two.
	rec := answerWithPolicy(&plugin.RequestOut{Handled: true, Location: "/kontakt?gesendet=1"})
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'none'") {
		t.Errorf("a redirect: policy %q", rec.Header().Get("Content-Security-Policy"))
	}
	// And the payment handover the server allowed stays.
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), web.PaymentFormAction) {
		t.Errorf("payment form-action lost: %q", rec.Header().Get("Content-Security-Policy"))
	}
}
