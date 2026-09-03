package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

func testSessionManager() *scs.SessionManager {
	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	return sm
}

// lookupRole returns a UserLookup that reports every user as existing with the
// given role.
func lookupRole(role string) UserLookup {
	return func(context.Context, int64) (string, bool, error) { return role, true, nil }
}

// lookupMissing returns a UserLookup that reports every user as deleted.
func lookupMissing() UserLookup {
	return func(context.Context, int64) (string, bool, error) { return "", false, nil }
}

func TestRequireAuthRedirectsWhenNoUser(t *testing.T) {
	sm := testSessionManager()
	handler := sm.LoadAndSave(RequireAuth(sm, lookupRole("admin"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/admin/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/login" {
		t.Errorf("expected redirect to /admin/login, got %q", loc)
	}
}

func TestRequireAuthPassesWhenUserPresent(t *testing.T) {
	sm := testSessionManager()
	var reached bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, int64(1))
		RequireAuth(sm, lookupRole("admin"))(inner).ServeHTTP(w, r)
	}))

	req := httptest.NewRequest("GET", "/admin/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !reached {
		t.Error("expected inner handler to be called")
	}
}

// A session whose user was deleted must not stay valid until it expires.
func TestRequireAuthRejectsDeletedUser(t *testing.T) {
	sm := testSessionManager()
	var reached bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, int64(1))
		RequireAuth(sm, lookupMissing())(inner).ServeHTTP(w, r)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/", nil))

	if reached {
		t.Error("deleted user reached the protected handler")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 to login, got %d", rec.Code)
	}
}

// Demoting an admin must take effect on the next request, not at session expiry.
func TestRequireAuthRefreshesRoleFromDatabase(t *testing.T) {
	sm := testSessionManager()
	var gotRole string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole = sm.GetString(r.Context(), SessionKeyUserRole)
	})
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, int64(1))
		sm.Put(r.Context(), SessionKeyUserRole, "admin") // stale session value
		RequireAuth(sm, lookupRole("editor"))(inner).ServeHTTP(w, r)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/admin/", nil))

	if gotRole != "editor" {
		t.Errorf("expected role refreshed to editor, got %q", gotRole)
	}
}

func TestRequireAdminForbidsEditor(t *testing.T) {
	sm := testSessionManager()
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, int64(1))
		sm.Put(r.Context(), SessionKeyUserRole, "editor")
		RequireAdmin(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, r)
	}))

	req := httptest.NewRequest("GET", "/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireAdminPassesForAdmin(t *testing.T) {
	sm := testSessionManager()
	var reached bool
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), SessionKeyUserID, int64(1))
		sm.Put(r.Context(), SessionKeyUserRole, "admin")
		RequireAdmin(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, r)
	}))

	req := httptest.NewRequest("GET", "/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !reached {
		t.Error("expected inner handler to be called for admin")
	}
}

// Die Adresse entscheidet, nicht die Route: alles unter /admin/websites/<Zahl>
// gehört zu dieser Website, ganz gleich, was danach kommt.
func TestWebsiteIDInPath(t *testing.T) {
	cases := map[string]int64{
		"/admin/websites/7":                     7,
		"/admin/websites/7/":                    7,
		"/admin/websites/12/pages/34/edit":      12,
		"/admin/websites/12/plugins/bestellung": 12,
		"/admin/websites/new":                   0,
		"/admin/websites/import":                0,
		"/admin/websites":                       0,
		"/admin/users/3/edit":                   0,
		"/admin/websites/0/pages":               0,
		"/admin/websites/-1/pages":              0,
		"/admin/websites/7x/pages":              0,
	}
	for path, want := range cases {
		if got := websiteIDInPath(path); got != want {
			t.Errorf("websiteIDInPath(%q) = %d; want %d", path, got, want)
		}
	}
}

func TestRequireWebsiteAccess(t *testing.T) {
	sm := testSessionManager()
	// Diese Person darf nur Website 2.
	allowed := func(_ context.Context, userID, websiteID int64) bool {
		return userID == 1 && websiteID == 2
	}

	run := func(path string) (int, bool) {
		var reached bool
		handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), SessionKeyUserID, int64(1))
			RequireWebsiteAccess(sm, allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(w, r)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code, reached
	}

	if code, reached := run("/admin/websites/2/pages"); !reached || code != http.StatusOK {
		t.Errorf("die eigene Website wurde abgewiesen: %d", code)
	}
	// Und alles andere unter der fremden Website ebenso — die Prüfung hängt an
	// der Adresse, nicht an einer Liste von Routen.
	for _, path := range []string{
		"/admin/websites/1/pages",
		"/admin/websites/1/pages/9/edit",
		"/admin/websites/1/medien",
		"/admin/websites/1/felder",
		"/admin/websites/1/export",
	} {
		if code, reached := run(path); reached || code != http.StatusForbidden {
			t.Errorf("%s: code %d, erreicht %v; erwartet 403 und nicht erreicht", path, code, reached)
		}
	}
	// Eine Adresse ohne Website geht die Prüfung nichts an.
	if code, reached := run("/admin/users"); !reached || code != http.StatusOK {
		t.Errorf("/admin/users wurde abgewiesen: %d", code)
	}
}
