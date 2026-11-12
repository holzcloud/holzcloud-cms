package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Freshly confirmed lets through, old does not — and "old" here means older
// than the quarter of an hour, not "a different session".
func TestElevatedAblauf(t *testing.T) {
	sm := testSessionManager()
	var frisch, alt bool
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		MarkElevated(sm, r.Context())
		frisch = Elevated(sm, r.Context())
		sm.Put(r.Context(), SessionKeyElevated, time.Now().Add(-ElevatedFor-time.Minute).Unix())
		alt = Elevated(sm, r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/admin/", nil))

	if !frisch {
		t.Error("just confirmed does not count")
	}
	if alt {
		t.Error("a confirmation from a quarter of an hour ago still counts")
	}
}

func TestRequireFreshPasswordSchicktZumBestaetigen(t *testing.T) {
	sm := testSessionManager()
	var erreicht bool
	handler := sm.LoadAndSave(RequireFreshPassword(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		erreicht = true
	})))

	req := httptest.NewRequest("POST", "/admin/websites/1/delete", nil)
	req.Header.Set("Referer", "http://example.com/admin/websites/1")
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if erreicht {
		t.Error("the action went through without a confirmation")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Code %d, erwartet 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != ConfirmPath+"?weiter=%2Fadmin%2Fwebsites%2F1" {
		t.Errorf("Ziel = %q", got)
	}
	if !varyNames(rec.Header(), "HX-Request") {
		t.Errorf("Vary = %q; the answer depends on HX-Request", rec.Header().Values("Vary"))
	}
}

// A button that posts through htmx gets the 303 swapped into the page — the
// confirmation screen drawn inside the list it came from, or nothing at all.
// htmx follows HX-Redirect instead.
func TestRequireFreshPasswordAnswersHtmxWithARedirectHeader(t *testing.T) {
	sm := testSessionManager()
	var erreicht bool
	handler := sm.LoadAndSave(RequireFreshPassword(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		erreicht = true
	})))

	req := httptest.NewRequest("POST", "/admin/users/new", nil)
	req.Header.Set("Referer", "http://example.com/admin/users")
	req.Header.Set("HX-Request", "true")
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if erreicht {
		t.Error("the action went through without a confirmation")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status %d, want 200 — htmx does not act on a redirect status", rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != ConfirmPath+"?weiter=%2Fadmin%2Fusers" {
		t.Errorf("HX-Redirect = %q", got)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q; the browser would follow it inside the XHR", got)
	}
	if !varyNames(rec.Header(), "HX-Request") {
		t.Errorf("Vary = %q; the answer depends on HX-Request", rec.Header().Values("Vary"))
	}
}

func varyNames(h http.Header, name string) bool {
	for _, v := range h.Values("Vary") {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), name) {
				return true
			}
		}
	}
	return false
}

// The way back comes out of a header anybody can set. It must never lead away
// from this server — or the confirmation is an open redirect.
func TestRueckwegBleibtImHaus(t *testing.T) {
	for _, fremd := range []string{
		"https://beispiel.example/boese",
		"//beispiel.example/boese",
		"/login",
		"/media/1/x.jpg",
		"",
	} {
		if got := SafeReturn(fremd); got != "/admin/" {
			t.Errorf("SafeReturn(%q) = %q; erwartet /admin/", fremd, got)
		}
	}
	if got := SafeReturn("/admin/websites/2/pages?status=draft"); got != "/admin/websites/2/pages?status=draft" {
		t.Errorf("ein eigener Pfad wurde verworfen: %q", got)
	}
}

// A referer from elsewhere must not become the target either.
func TestBackToTakesOnlyItsOwnAddresses(t *testing.T) {
	req := httptest.NewRequest("POST", "/admin/websites/1/delete", nil)
	req.Host = "example.com"
	req.Header.Set("Referer", "https://boese.example/admin/websites/1")
	if got := backTo(req); got != "/admin/" {
		t.Errorf("backTo = %q; erwartet /admin/", got)
	}
}

// A screen opened by its address has no Referer to go back to, and the screen
// that asked is the one the person wanted: a confirmation asked on a GET leads
// back to that screen. A POST cannot be repeated from its address, so it keeps
// going back to the page its button was on.
func TestAConfirmationAskedOnAGetLeadsBackToThatScreen(t *testing.T) {
	sm := testSessionManager()
	ask := func(method string, htmx bool) *httptest.ResponseRecorder {
		t.Helper()
		var reached bool
		handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = EnsureFreshPassword(sm, w, r)
		}))
		req := httptest.NewRequest(method, SetupPath, nil)
		req.Host = "example.com"
		req.Header.Set("Referer", "http://example.com/admin/konto")
		if htmx {
			req.Header.Set("HX-Request", "true")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if reached {
			t.Errorf("%s: went through without a confirmation", method)
		}
		return rec
	}
	toSetup := ConfirmPath + "?weiter=" + url.QueryEscape("/admin/2fa/einrichten")

	rec := ask(http.MethodGet, false)
	if got := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || got != toSetup {
		t.Errorf("GET: got %d to %q; want 303 to %q", rec.Code, got, toSetup)
	}

	rec = ask(http.MethodGet, true)
	if got := rec.Header().Get("HX-Redirect"); rec.Code != http.StatusOK || got != toSetup {
		t.Errorf("GET through htmx: got %d, HX-Redirect %q; want 200 and %q", rec.Code, got, toSetup)
	}

	rec = ask(http.MethodPost, false)
	if got := rec.Header().Get("Location"); got != ConfirmPath+"?weiter=%2Fadmin%2Fkonto" {
		t.Errorf("POST: Location = %q; want the Referer's path", got)
	}
}
