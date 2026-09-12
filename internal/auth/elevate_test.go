package auth

import (
	"net/http"
	"net/http/httptest"
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
