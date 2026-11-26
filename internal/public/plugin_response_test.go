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

// W-2 of the phase 35 security report: the deny list let Refresh through, which
// redirects without going through the Location check, and Link, which makes the
// browser preload a script. The headers a plugin may send are now a short list;
// everything else is dropped, the next header a browser learns to obey included.
func TestAPluginCannotSetRefreshOrLink(t *testing.T) {
	dropped := map[string]string{
		"Refresh":                     "0; url=https://evil.example",
		"refresh":                     "0; url=https://evil.example",
		"Link":                        "<https://evil.example/x.js>; rel=preload; as=script",
		"Set-Cookie":                  "a=b",
		"Content-Security-Policy":     "script-src 'unsafe-inline'",
		"X-Frame-Options":             "ALLOWALL",
		"Access-Control-Allow-Origin": "*",
		"Strict-Transport-Security":   "max-age=0",
		"Location":                    "https://evil.example",
		"X-Anything":                  "1",
		"Content-Length":              "5",
		"Transfer-Encoding":           "chunked",
		"Content-Type":                "text/javascript",
		"Clear-Site-Data":             `"cache"`,
		"Server-Timing":               "x",
	}
	sent := map[string]string{
		"Cache-Control":       "no-store",
		"content-language":    "de-CH",
		"Content-Disposition": `attachment; filename="a.txt"`,
		"ETag":                `"abc"`,
		"Expires":             "0",
		"Last-Modified":       "Mon, 02 Jan 2006 15:04:05 GMT",
		"Retry-After":         "30",
		"Vary":                "Accept-Language",
		"X-Robots-Tag":        "noindex",
	}
	headers := map[string]string{}
	for k, v := range dropped {
		headers[k] = v
	}
	for k, v := range sent {
		headers[k] = v
	}

	rec := answerWithPolicy(&plugin.RequestOut{Handled: true, Headers: headers, Body: "x"})
	for k := range dropped {
		if k == "Content-Type" || k == "Content-Security-Policy" {
			continue // always set by the host, checked below
		}
		if got := rec.Header().Values(k); len(got) != 0 {
			t.Errorf("%s was sent: %q", k, got)
		}
	}
	for k, v := range sent {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); strings.Contains(csp, "script-src 'unsafe-inline'") || !strings.Contains(csp, "script-src 'none'") {
		t.Errorf("a plugin replaced the policy: %q", csp)
	}
	if rec.Header().Get("Location") != "" {
		t.Errorf("Location = %q", rec.Header().Get("Location"))
	}
}

// A 103 carries headers to the browser before the real answer, and a 101 is not
// an answer at all. Nothing below 200 leaves the host as sent.
func TestAPluginCannotAnswerWithAnInformationalStatus(t *testing.T) {
	for status, want := range map[int]int{
		103: 200, 101: 200, 100: 200, 99: 200, 600: 200, 0: 200, -1: 200,
		200: 200, 204: 204, 303: 303, 404: 404, 503: 503, 599: 599,
	} {
		out := &plugin.RequestOut{Handled: true, Status: status, Body: "x"}
		if status == 103 {
			out.Headers = map[string]string{"Link": "</x.js>; rel=preload; as=script"}
		}
		rec := httptest.NewRecorder()
		writePluginResponse(rec, out)
		if rec.Code != want {
			t.Errorf("status %d was sent as %d, want %d", status, rec.Code, want)
		}
		if status == 103 && rec.Header().Get("Link") != "" {
			t.Errorf("Link was sent with the 103: %q", rec.Header().Get("Link"))
		}
	}
}
