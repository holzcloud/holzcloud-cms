package main

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// testRouter builds the same handler main() serves, against a temporary
// database, with CSRF and the setup guard replaced by pass-throughs so the
// table can focus on authorization.
func testRouter(t *testing.T) (http.Handler, *scs.SessionManager, *db.DB) {
	t.Helper()

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	adminTemplatesFS, err := fs.Sub(staticFS, "templates/admin")
	if err != nil {
		t.Fatal(err)
	}
	adminTmpl, err := web.ParseAdminTemplates(adminTemplatesFS)
	if err != nil {
		t.Fatalf("parse admin templates: %v", err)
	}
	publicDefaultFS, err := fs.Sub(staticFS, "templates/public/default")
	if err != nil {
		t.Fatal(err)
	}
	publicFS, err := fs.Sub(staticFS, "templates/public")
	if err != nil {
		t.Fatal(err)
	}

	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour

	cfg := config.Config{DataDir: dir, MaxMediaSize: 1 << 20, MaxTemplateSize: 1 << 20}
	domainStore := domain.NewStore(database)
	tmplStore := tmplmgr.NewStore(database, dir)
	loader := tmpl.NewLoader(dir, publicDefaultFS, publicFS, tmplStore)

	adminHandler := admin.NewHandler(database, sm, adminTmpl,
		auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		domainStore, domain.NewResolver(domainStore), page.NewStore(database),
		tmplStore, menu.NewStore(database), media.NewStore(database),
		snippet.NewStore(database), term.NewStore(database),
		sharelink.New([]byte("test")), loader, &cfg,
		auth.NewLoginThrottle(10, 100, time.Minute), web.NewClientIPResolver(nil))

	passthrough := func(next http.Handler) http.Handler { return next }
	handler, err := newRouter(routerDeps{
		cfg:             cfg,
		database:        database,
		sm:              sm,
		adminHandler:    adminHandler,
		csrfMiddleware:  passthrough,
		setupGuard:      passthrough,
		domainStore:     domainStore,
		domainResolver:  domain.NewResolver(domainStore),
		pageStore:       page.NewStore(database),
		menuStore:       menu.NewStore(database),
		termStore:       term.NewStore(database),
		shareSigner:     sharelink.New([]byte("share")),
		unlockSigner:    sharelink.New([]byte("unlock")),
		templateLoader:  loader,
		publicDefaultFS: publicDefaultFS,
	})
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	return handler, sm, database
}

// seedUser inserts a user and returns a session cookie for them.
func seedUser(t *testing.T, handler http.Handler, sm *scs.SessionManager, database *db.DB, email, role string) *http.Cookie {
	t.Helper()

	hash, err := auth.HashPassword("ein sicheres passwort", auth.Argon2Params{
		Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	res, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', $1, $2, $3)`, email, hash, role)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, _ := res.LastInsertId()

	// Establish a session through the same middleware stack the router uses.
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, id)
		sm.Put(r.Context(), auth.SessionKeyUserRole, role)
		sm.Put(r.Context(), auth.SessionKeyUserEmail, email)
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	for _, c := range rec.Result().Cookies() {
		if c.Name == sm.Cookie.Name {
			return c
		}
	}
	t.Fatal("no session cookie issued")
	return nil
}

func do(handler http.Handler, method, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "admin.test"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// Every destructive or site-level route must be admin-only. This table is the
// safety net that was missing when website deletion shipped reachable by any
// editor.
func TestRouteAuthorization(t *testing.T) {
	handler, sm, database := testRouter(t)
	editor := seedUser(t, handler, sm, database, "editor@test", "editor")
	adminUser := seedUser(t, handler, sm, database, "admin@test", "admin")

	adminOnly := []struct{ method, path string }{
		{"GET", "/admin/websites/new"},
		{"POST", "/admin/websites/new"},
		{"POST", "/admin/websites/1/delete"},
		{"POST", "/admin/websites/1/domains"},
		{"POST", "/admin/websites/1/domains/1/delete"},
		{"GET", "/admin/templates/upload"},
		{"POST", "/admin/templates/upload"},
		{"POST", "/admin/templates/1/activate"},
		{"POST", "/admin/templates/1/deactivate"},
		{"POST", "/admin/templates/1/delete"},
		{"GET", "/admin/users"},
		{"GET", "/admin/users/new"},
		{"POST", "/admin/users/new"},
		{"POST", "/admin/users/1/delete"},
	}
	for _, rt := range adminOnly {
		if got := do(handler, rt.method, rt.path, editor).Code; got != http.StatusForbidden {
			t.Errorf("%s %s: editor got %d; want 403", rt.method, rt.path, got)
		}
		if got := do(handler, rt.method, rt.path, adminUser).Code; got == http.StatusForbidden {
			t.Errorf("%s %s: admin was forbidden", rt.method, rt.path)
		}
	}

	// Content routes stay open to editors.
	for _, rt := range []struct{ method, path string }{
		{"GET", "/admin/"},
		{"GET", "/admin/websites"},
		{"GET", "/admin/websites/1/pages"},
		{"GET", "/admin/websites/1/menus"},
		{"GET", "/admin/websites/1/media"},
	} {
		if got := do(handler, rt.method, rt.path, editor).Code; got == http.StatusForbidden {
			t.Errorf("%s %s: editor must keep access, got 403", rt.method, rt.path)
		}
	}
}

// Without a session every admin route redirects to the login page.
func TestAdminRoutesRequireASession(t *testing.T) {
	handler, _, _ := testRouter(t)

	for _, path := range []string{
		"/admin/", "/admin/websites", "/admin/users", "/admin/templates",
		"/admin/websites/1/pages", "/admin/websites/1/media",
		// Rechnung und Lieferschein tragen Namen und Anschrift der Kundschaft
		// und sind über eine ratbare Bestellnummer erreichbar.
		"/admin/websites/1/bestellungen/2026-0001/rechnung",
		"/admin/websites/1/bestellungen/2026-0001/lieferschein",
	} {
		rec := do(handler, "GET", path, nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET %s without a session: %d; want 303", path, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/admin/login" {
			t.Errorf("GET %s redirected to %q; want /admin/login", path, loc)
		}
	}
}

// The admin UI must never be cacheable or framable, and must carry the strict CSP.
func TestAdminResponsesCarrySecurityHeaders(t *testing.T) {
	handler, sm, database := testRouter(t)
	adminUser := seedUser(t, handler, sm, database, "a@test", "admin")

	rec := do(handler, "GET", "/admin/", adminUser)
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("admin CSP missing: %q", csp)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("admin responses must not be framable")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("admin responses must not be cached")
	}
}

func TestHealthzIsPublic(t *testing.T) {
	handler, _, _ := testRouter(t)
	rec := do(handler, "GET", "/healthz", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("healthz: %d %q", rec.Code, rec.Body.String())
	}
}

// An unknown Host must not fall through to some other website's content.
func TestUnknownHostGets404(t *testing.T) {
	handler, _, _ := testRouter(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unbekannt.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown host: %d; want 404", rec.Code)
	}
}
