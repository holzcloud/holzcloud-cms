package domain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripPort(t *testing.T) {
	cases := map[string]string{
		"example.com":          "example.com",
		"example.com:8080":     "example.com",
		"localhost:80":         "localhost",
		"127.0.0.1:18080":      "127.0.0.1",
		"[::1]:8080":           "::1",
		"sub.example.com":      "sub.example.com",
		"xn--mbel-5qa.de:8080": "xn--mbel-5qa.de",
	}
	for in, want := range cases {
		if got := stripPort(in); got != want {
			t.Errorf("stripPort(%q) = %q; want %q", in, got, want)
		}
	}
}

// The resolver caches the whole Website struct per host, so deactivating or
// renaming a site only takes effect if the cache is cleared. Without the
// invalidation call in the admin handler, a deactivated site stayed reachable
// until the process restarted.
func TestResolverCacheInvalidation(t *testing.T) {
	res := NewResolver(nil)
	site := &Website{ID: 1, Name: "Site", Active: true}
	res.cache.Store("demo.test", resolved{website: site})

	reached := false
	handler := res.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if ws := WebsiteFromContext(r.Context()); ws == nil || ws.ID != 1 {
			t.Error("resolved website missing from the request context")
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test"
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if !reached {
		t.Fatal("an active cached site was not served")
	}

	// Simulate the site being deactivated in the database and the admin handler
	// clearing the cache. With a stale cache the next request would still be served.
	res.InvalidateCache()

	if _, ok := res.cache.Load("demo.test"); ok {
		t.Error("InvalidateCache left an entry behind")
	}
}

// An inactive site must 404 even when it is already in the cache.
func TestResolverRefusesInactiveWebsite(t *testing.T) {
	res := NewResolver(nil)
	res.cache.Store("demo.test", resolved{website: &Website{ID: 1, Name: "Site", Active: false}})

	reached := false
	handler := res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if reached {
		t.Error("an inactive website was served")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
}

// A cached host must survive the port the client happened to connect on.
func TestResolverMatchesHostWithPort(t *testing.T) {
	res := NewResolver(nil)
	res.cache.Store("demo.test", resolved{website: &Website{ID: 1, Active: true}})

	reached := false
	handler := res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test:8080"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !reached {
		t.Error("host with an explicit port did not resolve")
	}
}

// A deactivated site used to answer 404, which tells a search engine the pages
// are gone for good — a week of rebuilding would cost the whole index.
func TestMaintenanceModeAnswers503(t *testing.T) {
	res := NewResolver(nil)
	res.Offline = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	res.cache.Store("demo.test", resolved{website: &Website{
		ID: 1, Active: false, OfflineMode: "maintenance",
	}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/impressum", nil)
	req.Host = "demo.test"
	res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("an inactive site reached the page handler")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d; want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After on the maintenance response")
	}
}

// Without an offline handler wired in, maintenance mode must still not serve
// the site — it falls back to the old 404.
func TestMaintenanceWithoutHandlerStillRefuses(t *testing.T) {
	res := NewResolver(nil)
	res.cache.Store("demo.test", resolved{website: &Website{
		ID: 1, Active: false, OfflineMode: "maintenance",
	}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "demo.test"
	res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("an inactive site was served")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rec.Code)
	}
}

// Every alias used to serve the same content under its own address, which is
// what the canonical redirect is for.
func TestCanonicalRedirectSendsAliasesToThePrimaryHost(t *testing.T) {
	res := NewResolver(nil)
	res.Secure = true
	site := &Website{ID: 1, Active: true, CanonicalRedirect: true}
	res.cache.Store("alt.test", resolved{website: site, primary: "haupt.test"})
	res.cache.Store("haupt.test", resolved{website: site, primary: "haupt.test"})

	served := 0
	handler := res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served++ }))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/impressum?x=1", nil)
	req.Host = "alt.test"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d; want 301", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://haupt.test/impressum?x=1" {
		t.Errorf("Location = %q; want the full path and query on the primary host", got)
	}

	// The primary host itself must be served, not redirected to itself.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/impressum", nil)
	req.Host = "haupt.test"
	handler.ServeHTTP(rec, req)
	if served != 1 {
		t.Errorf("the primary host was not served (%d handler calls)", served)
	}
}

// Off by default, so an install reachable only over a bare IP does not redirect
// itself somewhere unreachable.
func TestNoRedirectWhenTheOptionIsOff(t *testing.T) {
	res := NewResolver(nil)
	site := &Website{ID: 1, Active: true, CanonicalRedirect: false}
	res.cache.Store("alt.test", resolved{website: site, primary: "haupt.test"})

	served := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alt.test"
	res.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true })).ServeHTTP(rec, req)

	if !served {
		t.Errorf("alias was redirected although the option is off (status %d)", rec.Code)
	}
}
