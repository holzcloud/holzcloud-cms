package public

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The five plugins that ship with the program, exactly as committed, through
// the whole of the server's chain: the archive path of the manager, the
// migration guard, the runtime, the route and content hooks, and the header
// layer that tightens a plugin's answer.
//
// These tests guard behaviour that must not change. The hardening of the
// plugin runtime (queue, revive, breaker, allowlists, script-free answers) is
// worth nothing if an operator's contact form stops taking messages, so the
// archives are installed through Manager.Install, never through the store, and
// answered through web.SecureHeaders and PluginMiddleware, never by calling a
// hook directly.

var shippedIDs = []string{"bestellung", "jahreszahl", "kontaktformular", "nicht-gefunden", "suche"}

// shippedHost is a server in miniature: one database, one data directory, one
// website, and a plugin manager that can be started again on the same ground.
type shippedHost struct {
	h        *Handler
	database *db.DB
	ws       *domain.Website
	dir      string
	manager  *plugin.Manager
	rt       *plugin.Runtime
}

func newShippedHost(t *testing.T) *shippedHost {
	t.Helper()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Velowerkstatt")
	// German with French beside it, as the contact form's own tests have it.
	store := domain.NewStore(database)
	set := domain.Settings{Locale: "de", ExtraLocales: "fr", TimeZone: "Europe/Zurich",
		OfflineMode: "notfound", PostsPerPage: 10}
	if err := store.UpdateSettings(context.Background(), ws.ID, set); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	ws, err := store.GetWebsite(context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	seedPage(t, database, ws.ID, "Wolle vom Hof", "wolle",
		"Wir verkaufen Wolle von unseren Schafen.", "published")
	seedPage(t, database, ws.ID, "Noch nicht fertig", "entwurf",
		"Auch hier steht Wolle, aber die Seite ist ein Entwurf.", "draft")
	return &shippedHost{h: h, database: database, ws: ws, dir: t.TempDir()}
}

// start builds a runtime and a manager on the host's database and data
// directory, wired as the server wires them, the way a process start does. A
// second call is a restart: the first runtime is closed first.
func (s *shippedHost) start(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if s.rt != nil {
		if err := s.rt.Close(ctx); err != nil {
			t.Fatalf("closing the first runtime: %v", err)
		}
	}
	store := plugin.NewStore(s.database)
	rt, err := plugin.NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	rt.WithPages(s.h.PagesForPlugin)
	rt.WithRender(s.h.RenderForPlugin)
	rt.WithNotify(s.h.NotifyForPlugin)
	domainStore := domain.NewStore(s.database)
	rt.WithSettings(func(ctx context.Context, id int64) (plugin.SettingsResult, error) {
		ws, err := domainStore.GetWebsite(ctx, id)
		if err != nil || ws == nil {
			return plugin.SettingsResult{}, fmt.Errorf("website %d not found", id)
		}
		return plugin.SettingsResult{
			WebsiteID: ws.ID, Name: ws.Name, Description: ws.Description,
			Locale: ws.Locale, TimeZone: ws.TimeZone, BlogBase: ws.BlogBase,
			ContactEmail: ws.ContactEmail, Locales: pluginLocales(ws),
		}, nil
	})
	manager, err := plugin.NewManager(ctx, store, rt, s.dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	s.rt, s.manager = rt, manager
	s.h.SetPlugins(manager)
}

// install puts a committed archive in through Manager.Install, then points the
// plugin at the website and switches it on.
func (s *shippedHost) install(t *testing.T, id string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugins", id, id+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	man, err := s.manager.Install(ctx, bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("%s: Install: %v", id, err)
	}
	if man.ID != id {
		t.Fatalf("%s.zip carries plugin %q", id, man.ID)
	}
	if err := s.manager.SetWebsites(ctx, id, []int64{s.ws.ID}); err != nil {
		t.Fatalf("%s: SetWebsites: %v", id, err)
	}
	if err := s.manager.Enable(ctx, id); err != nil {
		t.Fatalf("%s: Enable: %v", id, err)
	}
}

// chain answers a request the way the server does: the resolver has set the
// website, the header layer is outside, the plugin middleware inside, and the
// core is behind. A request that reaches the core is a failure of the test,
// except the one the test says is meant to.
func (s *shippedHost) chain(t *testing.T, method, target string, form url.Values, core http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req = req.WithContext(domain.WebsiteToContext(req.Context(), s.ws))
	if core == nil {
		core = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("%s %s went past the plugin to the core", method, target)
		})
	}
	rec := httptest.NewRecorder()
	web.SecureHeaders(s.h.PluginMiddleware(core)).ServeHTTP(rec, req)
	return rec
}

// contactForm is a valid submission. The page is drawn once first, so the
// plugin has made its signing key; the time trap is satisfied by a token
// stamped in the past, as the contact form's own tests do.
func (s *shippedHost) contactForm(t *testing.T, name string) url.Values {
	t.Helper()
	return url.Values{
		"seite":     {"kontakt"},
		"pfad":      {"/kontakt"},
		"gestellt":  {timeToken(t, s.database, -10*time.Second)},
		"name":      {name},
		"email":     {"eva@example.test"},
		"nachricht": {"Guten Tag, ich haette gern eine Auskunft."},
	}
}

func (s *shippedHost) drawContactForm(t *testing.T) string {
	t.Helper()
	return s.manager.FilterContent(context.Background(), s.ws.ID, plugin.ContentIn{
		WebsiteID: s.ws.ID, Slug: "kontakt", Title: "Kontakt", HTML: "<p>[[formular]]</p>",
	})
}

func wantSentOK(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.HasSuffix(loc, "?formular=gesendet") {
		t.Fatalf("%s: %d, Location %q, body %q", what, rec.Code, loc, rec.Body.String())
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'none'") {
		t.Errorf("%s: the answer's policy has no script-src 'none': %q", what, csp)
	}
}

func (s *shippedHost) wantSearch(t *testing.T, what string) {
	t.Helper()
	rec := s.chain(t, "GET", "http://velowerkstatt.test/suche?q=Wolle", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: search status %d, body %s", what, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("%s: search Content-Type %q", what, ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Wolle vom Hof") {
		t.Errorf("%s: the hit is missing:\n%s", what, body)
	}
	if strings.Contains(body, "Noch nicht fertig") {
		t.Errorf("%s: the draft is among the hits", what)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("%s: Cache-Control %q", what, got)
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("%s: X-Robots-Tag %q", what, got)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") || strings.Contains(csp, "script-src 'self'") {
		t.Errorf("%s: the search page's policy is %q", what, csp)
	}
}

func (s *shippedHost) wantEveryoneRunning(t *testing.T, what string) {
	t.Helper()
	all, err := s.manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, st := range all {
		seen[st.ID] = true
		if !st.Running || st.LastError != "" {
			t.Errorf("%s: %s running=%v, last error %q", what, st.ID, st.Running, st.LastError)
		}
	}
	for _, id := range shippedIDs {
		if !seen[id] {
			t.Errorf("%s: %s is not in the list", what, id)
		}
	}
}

// The tracer: one archive, from the zip to a stored message, through every
// layer the hardening touched.
func TestTheShippedContactFormAnswersThroughTheChain(t *testing.T) {
	s := newShippedHost(t)
	s.start(t)
	s.install(t, "kontaktformular")

	if html := s.drawContactForm(t); !strings.Contains(html, `<form class="contact-form`) {
		t.Fatalf("the plugin drew no form:\n%s", html)
	}

	rec := s.chain(t, "POST", "http://velowerkstatt.test/formular", s.contactForm(t, "Eva"), nil)
	wantSentOK(t, "the contact form", rec)
	if n := messages(t, s.database); n != 1 {
		t.Errorf("%d messages stored, expected 1", n)
	}

	// A page the plugin does not claim keeps the policy of the server.
	reached := false
	other := s.chain(t, "GET", "http://velowerkstatt.test/wolle", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	if !reached {
		t.Error("a page the plugin does not claim did not reach the core")
	}
	if csp := other.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("an ordinary page lost script-src 'self': %q", csp)
	}
}

func installAll(t *testing.T, s *shippedHost) {
	t.Helper()
	for _, id := range shippedIDs {
		s.install(t, id)
	}
}

func TestEveryShippedPluginAnswersThroughTheChain(t *testing.T) {
	s := newShippedHost(t)
	s.start(t)
	installAll(t, s)

	s.wantEveryoneRunning(t, "after the installation")
	s.wantSearch(t, "search")

	// Contact form.
	s.drawContactForm(t)
	wantSentOK(t, "the contact form", s.chain(t, "POST", "http://velowerkstatt.test/formular", s.contactForm(t, "Eva"), nil))
	if n := messages(t, s.database); n != 1 {
		t.Errorf("%d messages stored, expected 1", n)
	}

	// Orders: somebody who opens the address lands on the start page; an empty
	// order is sent back to where it came from with the reason in the query.
	get := s.chain(t, "GET", "http://velowerkstatt.test/bestellung", nil, nil)
	if get.Code != http.StatusSeeOther || get.Header().Get("Location") != "/" {
		t.Errorf("GET /bestellung: %d, Location %q", get.Code, get.Header().Get("Location"))
	}
	post := s.chain(t, "POST", "http://velowerkstatt.test/bestellung", url.Values{}, nil)
	if loc := post.Header().Get("Location"); post.Code != http.StatusSeeOther || !strings.Contains(loc, "?") {
		t.Errorf("POST /bestellung with an empty form: %d, Location %q", post.Code, loc)
	}

	// The two plugins that work on a page's text.
	out := s.manager.FilterContent(context.Background(), s.ws.ID, plugin.ContentIn{
		WebsiteID: s.ws.ID, Slug: "kontakt", Title: "Kontakt", HTML: "<p>(c) [[jahr]]</p><p>[[formular]]</p>",
	})
	if !strings.Contains(out, strconv.Itoa(time.Now().Year())) || strings.Contains(out, "[[jahr]]") {
		t.Errorf("the year was not put in:\n%s", out)
	}
	if !strings.Contains(out, `<form class="contact-form`) || strings.Contains(out, "[[formular]]") {
		t.Errorf("the form was not put in:\n%s", out)
	}

	// The screen of the plugin that lists what was not found.
	screen, err := s.manager.Admin(context.Background(), "nicht-gefunden", plugin.AdminIn{
		WebsiteID: s.ws.ID, Method: "GET", Path: "/",
	})
	if err != nil {
		t.Fatalf("the screen of nicht-gefunden: %v", err)
	}
	if screen.HTML == "" {
		t.Error("the screen of nicht-gefunden is empty")
	}
}

func migrationRows(t *testing.T, database *db.DB, id string) string {
	t.Helper()
	rows, err := database.Read.Query(
		`SELECT name, sha256, applied_at FROM plugin_migrations WHERE plugin_id = $1 ORDER BY name`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var name, sum, at string
		if err := rows.Scan(&name, &sum, &at); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s|%s|%s\n", name, sum, at)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestTheShippedPluginsComeBackAfterARestart(t *testing.T) {
	s := newShippedHost(t)
	s.start(t)
	installAll(t, s)
	s.drawContactForm(t)
	before := migrationRows(t, s.database, "kontaktformular")
	if before == "" {
		t.Fatal("the contact form recorded no migration; the check below would prove nothing")
	}

	// A server restart: a new runtime and manager on the same database and the
	// same data directory. Nothing is installed again.
	s.start(t)

	s.wantEveryoneRunning(t, "after the restart")
	s.wantSearch(t, "search after the restart")
	wantSentOK(t, "the contact form after the restart",
		s.chain(t, "POST", "http://velowerkstatt.test/formular", s.contactForm(t, "Bruno"), nil))
	if n := messages(t, s.database); n != 1 {
		t.Errorf("%d messages stored, expected 1", n)
	}
	get := s.chain(t, "GET", "http://velowerkstatt.test/bestellung", nil, nil)
	if get.Code != http.StatusSeeOther || get.Header().Get("Location") != "/" {
		t.Errorf("GET /bestellung after the restart: %d, Location %q", get.Code, get.Header().Get("Location"))
	}

	// Installing the same archive again on top of what runs: it succeeds, runs
	// no migration a second time, and the plugin stays up.
	s.install(t, "kontaktformular")
	if after := migrationRows(t, s.database, "kontaktformular"); after != before {
		t.Errorf("a second installation changed the migration record:\nbefore:\n%safter:\n%s", before, after)
	}
	s.wantEveryoneRunning(t, "after installing the contact form a second time")
	if n := messages(t, s.database); n != 1 {
		t.Errorf("%d messages after the second installation, expected 1", n)
	}
}
