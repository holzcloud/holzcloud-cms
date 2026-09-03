package public

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Without the filter the list is mostly WordPress probes and the one real
// broken link is buried.
func TestScannerNoiseIsNotReported(t *testing.T) {
	for _, path := range []string{
		"/wp-login.php", "/index.php", "/.env", "/.git/config",
		"/administrator/index.php", "/xmlrpc.php", "/favicon.ico",
	} {
		if worthLogging(path) {
			t.Errorf("%q would be reported", path)
		}
	}
	for _, path := range []string{"/alte-seite", "/produkte/tische", "/kontakt.html"} {
		if !worthLogging(path) {
			t.Errorf("%q would not be reported, but it looks like a real broken link", path)
		}
	}
}

// The event crosses into a sandbox with a fixed memory budget, so what a
// scanner types must not decide how much of it is copied there.
func TestMissEventIsTrimmed(t *testing.T) {
	r := httptest.NewRequest("GET", "/"+strings.Repeat("a", 5000), nil)
	r.Header.Set("Referer", "https://beispiel.de/"+strings.Repeat("b", 5000))

	ev := missEvent(r)
	if len(ev["path"]) > maxLoggedPath {
		t.Errorf("path is %d characters, over the %d cap", len(ev["path"]), maxLoggedPath)
	}
	if len(ev["referer"]) > maxLoggedPath {
		t.Errorf("referer is %d characters, over the %d cap", len(ev["referer"]), maxLoggedPath)
	}
}

// No plugin installed is the normal case, and it must cost nothing and crash
// nothing: the core keeps no list of its own any more.
func TestLogMissWithoutPluginsIsSafe(t *testing.T) {
	h := &Handler{}
	h.logMiss(httptest.NewRequest("GET", "/gibt-es-nicht", nil), 1)
}
