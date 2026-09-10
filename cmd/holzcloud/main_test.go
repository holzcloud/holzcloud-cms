package main

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	"github.com/holzcloud/holzcloud-cms/internal/admin"
	"github.com/holzcloud/holzcloud-cms/internal/album"
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
	return testRouterWith(t, routerTweaks{})
}

// routerTweaks are the few dependencies a test may need to change. The zero
// value is what the whole existing suite gets: no trusted-proxy resolver, and
// single sign-on switched off, so nothing about those tests changes.
type routerTweaks struct {
	ssoEnabled bool
	ssoSecret  string
	clientIP   *web.ClientIPResolver
	// setupGuard replaces the pass-through the table tests use. A test that
	// needs to see a request the way an admin handler sees it installs a
	// recorder here: setupGuard sits inside every middleware newRouter wraps
	// around the mux, so what it observes is the far side of the chain.
	setupGuard func(http.Handler) http.Handler
}

func testRouterWith(t *testing.T, tw routerTweaks) (http.Handler, *scs.SessionManager, *db.DB) {
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

	cfg := config.Config{
		DataDir: dir, MaxMediaSize: 1 << 20, MaxTemplateSize: 1 << 20,
		SSOEnabled: tw.ssoEnabled, SSOSecret: tw.ssoSecret,
	}
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
	// Wired here so the album routes reach the handler rather than its
	// nil-store guard: without this the authorization table below would pass
	// on a 404 that has nothing to do with who may enter.
	adminHandler.SetAlbumStore(album.NewStore(database))

	passthrough := func(next http.Handler) http.Handler { return next }
	guard := tw.setupGuard
	if guard == nil {
		guard = passthrough
	}
	handler, err := newRouter(routerDeps{
		cfg:             cfg,
		database:        database,
		sm:              sm,
		adminHandler:    adminHandler,
		csrfMiddleware:  passthrough,
		setupGuard:      guard,
		clientIP:        tw.clientIP,
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
		{"POST", "/admin/websites/import-csv"},
		{"GET", "/admin/csv-import/abc"},
		{"POST", "/admin/csv-import/abc/probe"},
		{"POST", "/admin/csv-import/abc/start"},
		{"GET", "/admin/csv-vorlage"},
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
		// An album is content. It belongs here and not in the table above:
		// filing it there would lock every editor out of a feature built for
		// editors, and that table is for destructive and site-level routes.
		{"GET", "/admin/websites/1/albums"},
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
		"/admin/websites/1/albums",
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

// Every admin route the router registers is classified below, and the test at
// the bottom of this file fails when one is not.
//
// The reason is written at newRouter: "the missing requireAdmin on website
// deletion shipped unnoticed precisely because there was no test here that
// could see the route table." The table that was then written covered
// nineteen routes out of ninety-five, and stayed at nineteen while three
// phases added their screens — so "Phase 7's screens are for administrators,
// Phase 8's for editors" was asserted by nothing at all and could have been
// inverted without a single test noticing.
//
// What this classification is NOT: a second copy of the router. It was seeded
// from the registration form once, so it inherits today's behaviour rather
// than proving it correct. What it does from now on is make an authorization
// decision a thing somebody WROTE DOWN. Move a route between the two lists in
// main.go and the declaration here disagrees with the behaviour, out loud.

// route is one registered pattern and, where the pattern carries wildcards, a
// concrete path to drive it with.
type route struct {
	pattern string
	// probe overrides the mechanical wildcard substitution below, for a route
	// whose segments are not numbers.
	probe string
}

// concrete turns a registered pattern into a path a request can be made to.
// The values are deliberately ones that exist in no fixture: this test asks
// who may enter, not what they find, and a 404 is a perfectly good answer to
// the first question as long as it is not a 403.
func (r route) concrete() (method, path string) {
	method, pattern, _ := strings.Cut(r.pattern, " ")
	if r.probe != "" {
		return method, r.probe
	}
	out := []string{}
	for _, seg := range strings.Split(pattern, "/") {
		switch {
		case !strings.HasPrefix(seg, "{"):
			out = append(out, seg)
		case strings.HasSuffix(seg, "...}"):
			out = append(out, "x")
		case seg == "{token}":
			out = append(out, "abc")
		case seg == "{slug}", seg == "{kind}", seg == "{path}":
			out = append(out, "x")
		default:
			out = append(out, "1")
		}
	}
	return method, strings.Join(out, "/")
}

// adminOnlyRoutes must answer 403 to an editor.
var adminOnlyRoutes = []route{
	{pattern: "GET /admin/websites/new"},
	{pattern: "POST /admin/websites/new"},
	{pattern: "POST /admin/websites/{id}/delete"},
	{pattern: "POST /admin/websites/{id}/domains"},
	{pattern: "POST /admin/websites/{id}/domains/{domainID}/delete"},
	{pattern: "POST /admin/websites/{id}/domains/{domainID}/primary"},
	{pattern: "POST /admin/websites/{id}/trash/{pageID}/purge"},
	{pattern: "GET /admin/websites/{id}/shop"},
	{pattern: "POST /admin/websites/{id}/shop"},
	{pattern: "GET /admin/websites/{id}/export"},
	{pattern: "POST /admin/websites/import"},
	{pattern: "POST /admin/websites/import-wordpress"},
	{pattern: "POST /admin/websites/import-csv"},
	{pattern: "GET /admin/csv-import/{token}"},
	{pattern: "POST /admin/csv-import/{token}/probe"},
	{pattern: "POST /admin/csv-import/{token}/start"},
	{pattern: "GET /admin/csv-vorlage"},
	{pattern: "POST /admin/websites/{id}/design/tokens"},
	{pattern: "GET /admin/websites/{id}/inhaltsarten"},
	{pattern: "POST /admin/websites/{id}/inhaltsarten"},
	{pattern: "POST /admin/websites/{id}/inhaltsarten/{kindID}/loeschen"},
	{pattern: "POST /admin/websites/{id}/inhaltsarten/{kindID}/verschieben"},
	{pattern: "GET /admin/websites/{id}/bausteinarten"},
	{pattern: "POST /admin/websites/{id}/bausteinarten"},
	{pattern: "POST /admin/websites/{id}/bausteinarten/{typeID}/loeschen"},
	{pattern: "POST /admin/websites/{id}/bausteinarten/{typeID}/verschieben"},
	{pattern: "GET /admin/websites/{id}/felder"},
	{pattern: "POST /admin/websites/{id}/felder"},
	{pattern: "POST /admin/websites/{id}/felder/{fieldID}/loeschen"},
	{pattern: "POST /admin/websites/{id}/felder/{fieldID}/verschieben"},
	{pattern: "GET /admin/users"},
	{pattern: "GET /admin/users/new"},
	{pattern: "POST /admin/users/new"},
	{pattern: "GET /admin/users/{id}/edit"},
	{pattern: "POST /admin/users/{id}/edit"},
	{pattern: "POST /admin/users/{id}/delete"},
	{pattern: "POST /admin/users/{id}/link"},
	{pattern: "POST /admin/users/{id}/sessions/revoke"},
	{pattern: "GET /admin/mail"},
	{pattern: "POST /admin/mail/test"},
	{pattern: "POST /admin/mail/retry"},
	{pattern: "GET /admin/ai"},
	{pattern: "POST /admin/ai/keys"},
	{pattern: "POST /admin/ai/keys/{id}/revoke"},
	{pattern: "GET /admin/plugins"},
	{pattern: "POST /admin/plugins/upload"},
	{pattern: "POST /admin/plugins/{id}/enable"},
	{pattern: "POST /admin/plugins/{id}/websites"},
	{pattern: "POST /admin/plugins/{id}/remove"},
	{pattern: "GET /admin/templates/upload"},
	{pattern: "POST /admin/templates/upload"},
	{pattern: "GET /admin/templates/spec"},
	{pattern: "POST /admin/templates/{id}/activate"},
	{pattern: "POST /admin/templates/{id}/deactivate"},
	{pattern: "POST /admin/templates/{id}/delete"},
	{pattern: "GET /admin/protokoll"},
	{pattern: "POST /admin/protokoll/aufraeumen"},
	{pattern: "GET /admin/marke"},
	{pattern: "POST /admin/marke"},
	{pattern: "GET /admin/sprachen"},
	{pattern: "POST /admin/sprachen"},
	{pattern: "POST /admin/sprachen/neu-lesen"},
	{pattern: "GET /admin/sprachen/vorlage"},
	{pattern: "GET /admin/sprachen/{code}/datei"},
	{pattern: "POST /admin/sprachen/{code}/loeschen"},
}

// editorOpenRoutes must NOT answer 403 to an editor. They are the content
// screens, and locking an editor out of one is as much a defect as letting
// them into a site-level one.
var editorOpenRoutes = []route{
	{pattern: "POST /admin/logout"},
	{pattern: "GET /admin/bestaetigen"},
	{pattern: "POST /admin/bestaetigen"},
	{pattern: "GET /admin/konto"},
	{pattern: "POST /admin/konto/sprache"},
	{pattern: "GET /admin/2fa/einrichten"},
	{pattern: "POST /admin/2fa/einrichten"},
	{pattern: "POST /admin/2fa/einrichten/neu"},
	{pattern: "POST /admin/2fa/codes"},
	{pattern: "POST /admin/2fa/aus"},
	{pattern: "GET /admin/"},
	{pattern: "GET /admin/websites"},
	{pattern: "GET /admin/websites/{id}"},
	{pattern: "POST /admin/websites/{id}"},
	{pattern: "GET /admin/websites/{id}/pages"},
	{pattern: "GET /admin/websites/{id}/uebersetzungen"},
	{pattern: "GET /admin/websites/{id}/pages/new"},
	{pattern: "POST /admin/websites/{id}/pages/new"},
	{pattern: "GET /admin/websites/{id}/pages/{pageID}/edit"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/edit"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/delete"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/status"},
	{pattern: "GET /admin/websites/{id}/pages/{pageID}/edit-title"},
	{pattern: "PUT /admin/websites/{id}/pages/{pageID}/title"},
	{pattern: "POST /admin/websites/{id}/pages/preview"},
	{pattern: "POST /admin/websites/{id}/pages/bulk"},
	{pattern: "POST /admin/websites/{id}/spalten"},
	{pattern: "POST /admin/websites/{id}/ansichten"},
	{pattern: "POST /admin/websites/{id}/ansichten/{viewID}/loeschen"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/share"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/uebersetzen"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/duplicate"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/review"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/insert-media"},
	{pattern: "GET /admin/websites/{id}/pages/{pageID}/revisions"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/revisions/{revID}/restore"},
	{pattern: "GET /admin/websites/{id}/pages/{pageID}/revisions/vergleich"},
	{pattern: "POST /admin/websites/{id}/pages/{pageID}/revisions/{revID}/beschriften"},
	{pattern: "GET /admin/websites/{id}/snippets"},
	{pattern: "POST /admin/websites/{id}/snippets"},
	{pattern: "POST /admin/websites/{id}/snippets/{snippetID}/delete"},
	{pattern: "GET /admin/websites/{id}/redirects"},
	{pattern: "POST /admin/websites/{id}/redirects"},
	{pattern: "POST /admin/websites/{id}/redirects/{redirectID}/delete"},
	{pattern: "GET /admin/websites/{id}/trash"},
	{pattern: "POST /admin/websites/{id}/trash/{pageID}/restore"},
	{pattern: "GET /admin/websites/{id}/preview"},
	{pattern: "GET /admin/websites/{id}/preview/t/{path...}"},
	{pattern: "GET /admin/websites/{id}/preview/{slug}"},
	{pattern: "GET /admin/websites/{id}/design"},
	{pattern: "POST /admin/websites/{id}/design/activate"},
	{pattern: "GET /admin/websites/{id}/produkte"},
	{pattern: "GET /admin/websites/{id}/produkte/{productID}"},
	{pattern: "POST /admin/websites/{id}/produkte/{productID}"},
	{pattern: "POST /admin/websites/{id}/produkte/{productID}/delete"},
	{pattern: "GET /admin/websites/{id}/bestellungen"},
	{pattern: "GET /admin/websites/{id}/bestellungen/{number}"},
	{pattern: "POST /admin/websites/{id}/bestellungen/{number}"},
	{pattern: "GET /admin/websites/{id}/bestellungen/{number}/{kind}"},
	{pattern: "GET /admin/websites/{id}/menus"},
	{pattern: "POST /admin/websites/{id}/menus"},
	{pattern: "GET /admin/websites/{id}/menus/{menuID}"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/update"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/delete"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/items"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/update"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/delete"},
	{pattern: "POST /admin/websites/{id}/menus/{menuID}/items/{itemID}/reorder"},
	{pattern: "GET /admin/websites/{id}/albums"},
	{pattern: "POST /admin/websites/{id}/albums"},
	{pattern: "GET /admin/websites/{id}/albums/{albumID}"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/update"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/delete"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/pictures"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/update"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/delete"},
	{pattern: "POST /admin/websites/{id}/albums/{albumID}/pictures/{itemID}/reorder"},
	{pattern: "GET /admin/websites/{id}/tags"},
	{pattern: "POST /admin/websites/{id}/tags/{termID}/rename"},
	{pattern: "POST /admin/websites/{id}/tags/{termID}/delete"},
	{pattern: "GET /admin/websites/{id}/media"},
	{pattern: "POST /admin/websites/{id}/media/upload"},
	{pattern: "POST /admin/websites/{id}/media/{mediaID}/delete"},
	{pattern: "POST /admin/websites/{id}/media/{mediaID}/meta"},
	{pattern: "GET /admin/websites/{id}/media/{mediaID}/zuschnitt"},
	{pattern: "POST /admin/websites/{id}/media/{mediaID}/zuschnitt"},
	{pattern: "GET /admin/websites/{id}/media/picker"},
	{pattern: "GET /admin/plugins/{id}/bildschirm"},
	{pattern: "POST /admin/plugins/{id}/bildschirm"},
	{pattern: "GET /admin/websites/{websiteID}/plugins/{id}"},
	{pattern: "POST /admin/websites/{websiteID}/plugins/{id}"},
	{pattern: "GET /admin/templates"},
}

// handlerDecidesRoutes are the routes the ROUTER cannot classify, because the
// answer depends on who is asking about whom. Each one names the check and
// where it lives; a route here without a reason is a route nobody thought
// about.
var handlerDecidesRoutes = []struct {
	pattern string
	because string
}{
	{"GET /admin/users/{id}/password",
		"an editor must be able to change their OWN password. HandlePasswordChange " +
			"refuses !isSelf for a non-admin at internal/admin/user.go:409-416, which is a " +
			"rule about two identities and not about a role, so requireAdmin cannot express it"},
	{"POST /admin/users/{id}/password",
		"the same handler and the same check, on the arm that writes"},
}

// adminRoutesInSource reads the patterns straight out of newRouter.
//
// From the source and not from the mux, because http.ServeMux does not hand
// its patterns back — and reading the source is what makes this a check on the
// router rather than a check on itself.
func adminRoutesInSource(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	re := regexp.MustCompile(`adminProtectedMux\.(?:HandleFunc|Handle)\("((?:GET|POST|PUT|DELETE) /admin[^"]*)"`)
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		out[m[1]] = true
	}
	if len(out) < 50 {
		t.Fatalf("only %d admin routes found in main.go — the pattern above has "+
			"stopped matching the registrations, and this whole test would pass vacuously", len(out))
	}
	return out
}

// TestEveryAdminRouteIsClassified is the net under the three tables.
func TestEveryAdminRouteIsClassified(t *testing.T) {
	registered := adminRoutesInSource(t)

	classified := map[string]string{}
	add := func(pattern, list string) {
		if was, dup := classified[pattern]; dup {
			t.Errorf("%s is in both %s and %s — a route has one answer", pattern, was, list)
			return
		}
		classified[pattern] = list
	}
	for _, r := range adminOnlyRoutes {
		add(r.pattern, "adminOnlyRoutes")
	}
	for _, r := range editorOpenRoutes {
		add(r.pattern, "editorOpenRoutes")
	}
	for _, r := range handlerDecidesRoutes {
		add(r.pattern, "handlerDecidesRoutes")
		if len(r.because) < 40 {
			t.Errorf("%s is in handlerDecidesRoutes with no real reason — that list is "+
				"where a route goes when somebody has read the handler, not where it goes to be quiet", r.pattern)
		}
	}

	for pattern := range registered {
		if classified[pattern] == "" {
			t.Errorf("%s is registered and classified nowhere. Put it in adminOnlyRoutes, "+
				"in editorOpenRoutes, or — if the handler decides — in handlerDecidesRoutes "+
				"with the reason", pattern)
		}
	}
	for pattern, list := range classified {
		if !registered[pattern] {
			t.Errorf("%s is in %s and is registered nowhere; the route was renamed or "+
				"removed and its declaration was left behind", pattern, list)
		}
	}
}

// TestTheClassificationMatchesTheBehaviour drives every classified route as an
// editor and holds it to what the table says.
//
// This is the half that catches a silent inversion: change a route in main.go
// from requireAdmin to open and the declaration above still says admin-only,
// so this fails. A 404 is fine everywhere — the fixtures are deliberately
// empty, and the question is who may enter.
func TestTheClassificationMatchesTheBehaviour(t *testing.T) {
	handler, sm, database := testRouter(t)
	editor := seedUser(t, handler, sm, database, "editor-cls@test", "editor")

	for _, r := range adminOnlyRoutes {
		method, path := r.concrete()
		if got := do(handler, method, path, editor).Code; got != http.StatusForbidden {
			t.Errorf("%s: editor got %d, want 403 — it is declared admin-only", r.pattern, got)
		}
	}
	for _, r := range editorOpenRoutes {
		method, path := r.concrete()
		if got := do(handler, method, path, editor).Code; got == http.StatusForbidden {
			t.Errorf("%s: editor got 403 — it is declared open to editors", r.pattern)
		}
	}
}

// refusingLookup fails the test if the database is consulted at all.
type refusingLookup struct{ t *testing.T }

func (r refusingLookup) GetWebsite(context.Context, int64) (*domain.Website, error) {
	r.t.Error("the database was queried although provisioning is off")
	return nil, nil
}

// The start-up half of the provisioning refusal. config.Load can only see that
// an id was given; only the database can say whether anybody ever created it,
// and an account provisioned into a website that is not there has no rows in
// user_websites — which NewWebsiteAccessLookup reads as "every website".
func TestProvisioningRefusesToStartWithoutItsDefaultWebsite(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	store := domain.NewStore(database)

	created, err := store.CreateWebsite(ctx, "Redaktion A", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	t.Run("an id nobody ever created", func(t *testing.T) {
		cfg := config.Config{SSOProvision: true, SSODefaultWebsite: created.ID + 4242}

		err := checkDefaultWebsite(ctx, cfg, store)
		if err == nil {
			t.Fatal("a website that does not exist was accepted; want a refusal to start")
		}
		if !strings.Contains(err.Error(), "HOLZCLOUD_SSO_DEFAULT_WEBSITE") {
			t.Errorf("the refusal must name the variable: %v", err)
		}
		if !strings.Contains(err.Error(), strconv.FormatInt(cfg.SSODefaultWebsite, 10)) {
			t.Errorf("the refusal must name the id it looked for: %v", err)
		}
	})

	t.Run("a website that exists", func(t *testing.T) {
		cfg := config.Config{SSOProvision: true, SSODefaultWebsite: created.ID}
		if err := checkDefaultWebsite(ctx, cfg, store); err != nil {
			t.Errorf("a named website that exists must start: %v", err)
		}
	})

	t.Run("provisioning off asks the database nothing", func(t *testing.T) {
		cfg := config.Config{SSOProvision: false, SSODefaultWebsite: 0}
		if err := checkDefaultWebsite(ctx, cfg, refusingLookup{t}); err != nil {
			t.Errorf("with provisioning off there is nothing to check: %v", err)
		}
	})
}

// identityHeaderSpellings is every spelling of an identity header this binary
// must remove. The first group goes through Header.Set, which canonicalises
// exactly as net/http does when it reads a name off the wire; the second is
// planted straight into the map so the strip cannot come to depend on
// canonical form. Adding a spelling is one line.
var identityHeaderSpellings = []string{
	"X-authentik-username",
	"X-authentik-email",
	"X-authentik-name",
	"X-authentik-uid",
	"X-authentik-groups",
	"X-authentik-entitlements",
	"X-authentik-jwt",
	"X-authentik-meta-provider",
	"X_authentik_email",
	"X_authentik_groups",
	"x-authentik-email",
	"X-AUTHENTIK-EMAIL",
}

var rawIdentityHeaderSpellings = []string{
	"x-authentik-email",
	"X-AUTHENTIK-EMAIL",
	"x_authentik_username",
}

func requestWithIdentityHeaders(method, target, remoteAddr string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "admin.test"
	req.RemoteAddr = remoteAddr
	for _, name := range identityHeaderSpellings {
		req.Header.Set(name, "stranger@example.com")
	}
	for _, name := range rawIdentityHeaderSpellings {
		req.Header[name] = []string{"stranger@example.com"}
	}
	req.Header.Set(web.ProxySecretHeader, "a-shared-secret")
	return req
}

// assertStripped walks a header map and fails on any key whose normalised name
// carries the identity prefix or names this installation's own shared secret.
func assertStripped(t *testing.T, where string, h http.Header) {
	t.Helper()
	for name := range h {
		normalised := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		if strings.HasPrefix(normalised, "x-authentik-") {
			t.Errorf("%s: %q survived; stripping identity headers is a property of this binary", where, name)
		}
		if normalised == strings.ToLower(web.ProxySecretHeader) {
			t.Errorf("%s: %q survived; a handler must not read the secret protecting it", where, name)
		}
	}
}

// TestIdentityHeadersAreStrippedEverywhere drives the real router, not the
// middleware, because the claim is about the binary rather than about one
// route prefix.
//
// Two observables are used, and they are not equally strong.
//
// For an admin route the assertion is made from the far side: setupGuard is a
// test seam newRouter already takes, and it sits inside every middleware
// wrapped around the mux, so the header map it records is the one an admin
// handler is given.
//
// For /healthz and for a public route there is no such seam, and registering a
// probe route would mean changing newRouter's shape for the benefit of a test.
// The assertion there is made on the request object the router was handed:
// stripIdentityHeaders deletes from r.Header in place, so a key still present
// afterwards is a key no strip removed. That is the weaker of the two — it
// cannot prove the deletion happened before the next handler ran, only that it
// happened — and internal/web/forwardauth_test.go carries the far-side proof of
// the ordering.
func TestIdentityHeadersAreStrippedEverywhere(t *testing.T) {
	t.Run("an admin route, seen from the far side of the chain", func(t *testing.T) {
		var seen http.Header
		handler, _, _ := testRouterWith(t, routerTweaks{
			setupGuard: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seen = r.Header.Clone()
					next.ServeHTTP(w, r)
				})
			},
		})
		req := requestWithIdentityHeaders("GET", "/admin/", "203.0.113.7:54321")
		handler.ServeHTTP(httptest.NewRecorder(), req)
		if seen == nil {
			t.Fatal("the recorder never ran; the far-side probe is not in the chain")
		}
		assertStripped(t, "an admin handler", seen)
	})

	handler, _, _ := testRouter(t)
	for _, tc := range []struct{ name, target string }{
		{"the liveness probe", "/healthz"},
		{"a public route", "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := requestWithIdentityHeaders("GET", tc.target, "203.0.113.7:54321")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code >= 500 {
				t.Fatalf("GET %s answered %d; the strip must not break the route", tc.target, rec.Code)
			}
			assertStripped(t, "GET "+tc.target, req.Header)
		})
	}

	t.Run("the liveness probe still answers", func(t *testing.T) {
		req := requestWithIdentityHeaders("GET", "/healthz", "203.0.113.7:54321")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
			t.Errorf("healthz with every identity header set: %d %q", rec.Code, rec.Body.String())
		}
	})
}

// TestUntrustedPeerWithIdentityHeaderGetsTheLoginForm is the in-process half of
// criterion 2: an untrusted peer that claims an identity gets the ordinary
// unauthenticated redirect — not a 403 and not a dashboard. The way back into
// the admin must not die with the proxy.
//
// RemoteAddr is set explicitly. httptest.NewRequest hands out 192.0.2.1:1234
// and a test that leaves it alone is testing whatever net/http/httptest happens
// to do this year. Setting it is also what stands in for a second machine:
// httptest.Server binds loopback, and loopback is trusted by default, so an
// in-process test that started a real listener would prove the opposite of what
// it claims. The live-network half of criterion 2 is run against the built
// binary and recorded in 10-02-SUMMARY.md.
func TestUntrustedPeerWithIdentityHeaderGetsTheLoginForm(t *testing.T) {
	trusted := web.NewClientIPResolver([]netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("::1/128"),
	})

	for _, tc := range []struct {
		name       string
		remoteAddr string
		secret     string
	}{
		{"an untrusted peer over IPv4", "203.0.113.7:54321", "a-shared-secret"},
		{"an untrusted peer over IPv6", "[2001:db8::1]:54321", "a-shared-secret"},
		{"an untrusted peer that even has the right secret", "203.0.113.7:54321", "the-real-secret"},
		{"a trusted peer with no secret", "127.0.0.1:41234", ""},
		// The control, and plan 10-03 changed why it holds rather than
		// whether it holds. Layers 1 and 3 are both satisfied, so an identity
		// really does reach ForwardAuthSignIn now — and the answer is still
		// the login form, because stranger@example.com names no account and a
		// refusal falls through to the password form instead of answering
		// 403. The sign-in half of that middleware is proved green in
		// TestForwardAuthSignsInThroughTheOrdinaryChain, against a seeded
		// account; this line is the other half, and the two together are what
		// SSO-02 asks for.
		{"a trusted peer with the right secret, whose identity names no account", "127.0.0.1:41234", "the-real-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, _, _ := testRouterWith(t, routerTweaks{
				ssoEnabled: true,
				ssoSecret:  "the-real-secret",
				clientIP:   trusted,
			})
			req := requestWithIdentityHeaders("GET", "/admin/", tc.remoteAddr)
			req.Header.Del(web.ProxySecretHeader)
			if tc.secret != "" {
				req.Header.Set(web.ProxySecretHeader, tc.secret)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Errorf("answered %d; want 303 — an identity header must never buy a dashboard, "+
					"and must never cost a refusal either", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != "/admin/login" {
				t.Errorf("redirected to %q; want /admin/login", loc)
			}
			assertStripped(t, "the request the router served", req.Header)
		})
	}
}

// forwardAuthRequest builds a request the way the proxy would: from a trusted
// peer, carrying this installation's shared secret and an identity.
//
// The identity is installed through web.ForwardAuth by driving a real request
// through the real router, and never by writing the context key directly. A
// test that fabricates the context would skip the layer wave 2 exists to build
// and would keep passing after that layer was deleted.
func forwardAuthRequest(target, username, email string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest("GET", target, nil)
	req.Host = "admin.test"
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set(web.ProxySecretHeader, "the-real-secret")
	req.Header.Set("X-authentik-username", username)
	req.Header.Set("X-authentik-email", email)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// ssoRouter builds the production router with single sign-on switched on and
// loopback trusted, which is what forwardAuthRequest sets RemoteAddr to.
func ssoRouter(t *testing.T) (http.Handler, *scs.SessionManager, *db.DB) {
	t.Helper()
	return testRouterWith(t, routerTweaks{
		ssoEnabled: true,
		ssoSecret:  "the-real-secret",
		clientIP: web.NewClientIPResolver([]netip.Prefix{
			netip.MustParsePrefix("127.0.0.1/32"),
			netip.MustParsePrefix("::1/128"),
		}),
	})
}

// insertUser seeds an account without establishing a session for it — the
// point of forward auth being that the session is what arrives without one.
func insertUser(t *testing.T, database *db.DB, email, role string) int64 {
	t.Helper()
	res, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', $1, 'x', $2)`, email, role)
	if err != nil {
		t.Fatalf("insert user %q: %v", email, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// TestForwardAuthSignsInThroughTheOrdinaryChain is the whole of plan 10-03's
// position argument, driven through the router main() serves.
//
// The middleware sits one nesting level outside requireAuth, so everything
// behind it runs unchanged on the session it wrote. The second half of the test
// is what proves "unchanged" rather than asserting it: the same person, signed
// in the same way, is still refused an admin-only route, because the role check
// behind the middleware is the one that was always there.
func TestForwardAuthSignsInThroughTheOrdinaryChain(t *testing.T) {
	handler, _, database := ssoRouter(t)
	insertUser(t, database, "editor@test", "editor")
	// Linked on purpose: since migration 00053 a sign-in reaches the account
	// linked to its identity, never the one that merely carries the address.
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE users SET sso_username = 'editor' WHERE email = 'editor@test'`); err != nil {
		t.Fatalf("link editor@test to the identity editor: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardAuthRequest("/admin/", "editor", "editor@test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/ with an identity for a seeded editor answered %d; want 200 — "+
			"a person the identity provider already signed in must not be asked again", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardAuthRequest("/admin/users", "editor", "editor@test", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /admin/users as an editor signed in through forward auth answered %d; want 403 — "+
			"the middleware signs a session in and authorises nothing", rec.Code)
	}
}

// TestForwardAuthDoesNotTouchTheLoginPage: the public admin chain carries no
// forward-auth layer, which is half of what makes the password path provably
// unchanged.
func TestForwardAuthDoesNotTouchTheLoginPage(t *testing.T) {
	handler, sm, database := ssoRouter(t)
	insertUser(t, database, "editor@test", "editor")
	// Linked on purpose: since migration 00053 a sign-in reaches the account
	// linked to its identity, never the one that merely carries the address.
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE users SET sso_username = 'editor' WHERE email = 'editor@test'`); err != nil {
		t.Fatalf("link editor@test to the identity editor: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardAuthRequest("/admin/login", "editor", "editor@test", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/login answered %d; want the login form", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `name="email"`) ||
		!strings.Contains(rec.Body.String(), `type="password"`) {
		t.Error("GET /admin/login did not answer the ordinary password form")
	}
	// A session written on that request would be the bug: nothing on the public
	// chain may sign anybody in on the strength of a header.
	for _, c := range rec.Result().Cookies() {
		if c.Name != sm.Cookie.Name {
			continue
		}
		var userID int64
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(c)
		sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		})).ServeHTTP(httptest.NewRecorder(), req)
		if userID != 0 {
			t.Errorf("/admin/login signed in user %d; the public admin chain has no forward-auth layer", userID)
		}
	}
}

// TestPasswordPathIsUnchangedWithSSOOff is SSO-09 as a test rather than as a
// sentence: with the zero-value SSO options — which is every existing
// installation — the password sign-in answers what it always answered, writes
// the session keys it always wrote, and is not marked as coming through single
// sign-on.
//
// It runs from here to the end of the phase and is meant to be untouched by it.
func TestPasswordPathIsUnchangedWithSSOOff(t *testing.T) {
	handler, sm, database := testRouter(t)

	hash, err := auth.HashPassword("ein sicheres passwort", auth.Argon2Params{
		Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Write.ExecContext(context.Background(),
		`INSERT INTO users (name, email, password, role) VALUES ('T', 'ada@example.com', $1, 'editor')`,
		hash); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"email": {"ada@example.com"}, "password": {"ein sicheres passwort"}}
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	req.Host = "admin.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// An identity header on the password request too: with the switch off it
	// must make no difference whatsoever.
	req.Header.Set("X-authentik-username", "somebody-else")
	req.Header.Set("X-authentik-email", "somebody-else@example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /admin/login answered %d; want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("redirected to %q; want /admin/", loc)
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sm.Cookie.Name {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("the password sign-in issued no session cookie")
	}

	var userID int64
	var role, email string
	var viaSSO bool
	probe := httptest.NewRequest("GET", "/", nil)
	probe.AddCookie(cookie)
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID = sm.GetInt64(r.Context(), auth.SessionKeyUserID)
		role = sm.GetString(r.Context(), auth.SessionKeyUserRole)
		email = sm.GetString(r.Context(), auth.SessionKeyUserEmail)
		viaSSO = sm.GetBool(r.Context(), auth.SessionKeyViaSSO)
	})).ServeHTTP(httptest.NewRecorder(), probe)

	if userID == 0 || role != "editor" || email != "ada@example.com" {
		t.Errorf("session after the password sign-in: user_id=%d role=%q email=%q; "+
			"want the seeded editor", userID, role, email)
	}
	if viaSSO {
		t.Error("a password sign-in was marked as established through single sign-on")
	}
}
