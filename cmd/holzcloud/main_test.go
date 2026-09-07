package main

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
	// Wired here so the album routes reach the handler rather than its
	// nil-store guard: without this the authorization table below would pass
	// on a 404 that has nothing to do with who may enter.
	adminHandler.SetAlbumStore(album.NewStore(database))

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
