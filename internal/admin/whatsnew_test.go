package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/changelog"
)

// whatsNewRig is a user, a cookie and the middleware around a probe.
type whatsNewRig struct {
	h       *Handler
	sm      *scs.SessionManager
	userID  int64
	cookie  *http.Cookie
	reached int
	latest  string
	older   string
}

func newWhatsNewRig(t *testing.T, seen string) *whatsNewRig {
	t.Helper()
	h, sm, _, _ := newTestAdmin(t)
	all := changelog.All()
	if len(all) < 2 {
		t.Skip("needs two releases")
	}
	id, err := h.users.Create(context.Background(), "T", "wn@test", "correct horse battery", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := h.users.SetSeenVersion(context.Background(), id, seen); err != nil {
		t.Fatalf("set seen: %v", err)
	}
	return &whatsNewRig{
		h: h, sm: sm, userID: id,
		cookie: signedIn(t, sm, id, "wn@test", false),
		latest: all[0].Version, older: all[1].Version,
	}
}

func (g *whatsNewRig) do(t *testing.T, running string, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { g.reached++ })
	rec := httptest.NewRecorder()
	g.sm.LoadAndSave(g.h.WhatsNewOnUpdate(running)(probe)).ServeHTTP(rec, req)
	return rec
}

func (g *whatsNewRig) seen(t *testing.T) string {
	t.Helper()
	v, err := g.h.users.SeenVersion(context.Background(), g.userID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func (g *whatsNewRig) page(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.AddCookie(g.cookie)
	return req
}

func TestWhatsNewRedirectsOnceAndRemembersInTheDatabase(t *testing.T) {
	g := newWhatsNewRig(t, "")
	older := g.older
	if err := g.h.users.SetSeenVersion(context.Background(), g.userID, older); err != nil {
		t.Fatal(err)
	}

	rec := g.do(t, g.latest, g.page("/admin/websites"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	want := "/admin/neuerungen/" + url.PathEscape(g.latest) + "?seit=" + url.QueryEscape(older) + "&weiter=%2Fadmin%2Fwebsites"
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if g.reached != 0 {
		t.Error("next ran on the redirecting request")
	}
	if g.seen(t) != g.latest {
		t.Errorf("DB holds %q, want %q", g.seen(t), g.latest)
	}

	// Same session again: through.
	rec = g.do(t, g.latest, g.page("/admin/websites"))
	if rec.Code == http.StatusSeeOther || g.reached != 1 {
		t.Errorf("second request: status %d reached %d", rec.Code, g.reached)
	}

	// A second, fresh session of the same user: the database remembers.
	fresh := signedIn(t, g.sm, g.userID, "wn@test", false)
	req := httptest.NewRequest(http.MethodGet, "/admin/websites", nil)
	req.Header.Set("Accept", "text/html")
	req.AddCookie(fresh)
	rec = g.do(t, g.latest, req)
	if rec.Code == http.StatusSeeOther || g.reached != 2 {
		t.Errorf("fresh session: status %d reached %d", rec.Code, g.reached)
	}
}

func TestWhatsNewNewAccountOnlyRecords(t *testing.T) {
	g := newWhatsNewRig(t, "")
	rec := g.do(t, g.latest, g.page("/admin/websites"))
	if rec.Code == http.StatusSeeOther || g.reached != 1 {
		t.Fatalf("status %d reached %d", rec.Code, g.reached)
	}
	if g.seen(t) != g.latest {
		t.Errorf("DB holds %q, want %q", g.seen(t), g.latest)
	}
}

func TestWhatsNewNeverActsOnANonReleaseBuild(t *testing.T) {
	g := newWhatsNewRig(t, "")
	if err := g.h.users.SetSeenVersion(context.Background(), g.userID, g.older); err != nil {
		t.Fatal(err)
	}
	for _, running := range []string{"dev", "v" + g.latest + "-3-gabc1234", "abc1234"} {
		g.reached = 0
		rec := g.do(t, running, g.page("/admin/websites"))
		if rec.Code == http.StatusSeeOther || g.reached != 1 {
			t.Errorf("%q: status %d reached %d", running, rec.Code, g.reached)
		}
		if g.seen(t) != g.older {
			t.Errorf("%q: DB changed to %q", running, g.seen(t))
		}
	}
}

func TestWhatsNewLeavesNonNavigationsAlone(t *testing.T) {
	g := newWhatsNewRig(t, "")
	if err := g.h.users.SetSeenVersion(context.Background(), g.userID, g.older); err != nil {
		t.Fatal(err)
	}
	mk := func(method, path string, hdr map[string]string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		req.AddCookie(g.cookie)
		return req
	}
	html := map[string]string{"Accept": "text/html"}
	cases := map[string]*http.Request{
		"htmx":         mk("GET", "/admin/websites", map[string]string{"Accept": "text/html", "HX-Request": "true"}),
		"post":         mk("POST", "/admin/websites", html),
		"image dest":   mk("GET", "/admin/websites", map[string]string{"Accept": "text/html", "Sec-Fetch-Dest": "image"}),
		"image accept": mk("GET", "/admin/websites", map[string]string{"Accept": "image/*"}),
		"2fa":          mk("GET", "/admin/2fa/einrichten", html),
		"confirm":      mk("GET", "/admin/bestaetigen", html),
		"ai consent":   mk("GET", "/admin/ai/verbinden", html),
	}
	for name, req := range cases {
		g.reached = 0
		rec := g.do(t, g.latest, req)
		if rec.Code == http.StatusSeeOther || g.reached != 1 {
			t.Errorf("%s: status %d reached %d", name, rec.Code, g.reached)
		}
		if g.seen(t) != g.older {
			t.Errorf("%s: DB changed to %q", name, g.seen(t))
		}
	}
}

func TestWhatsNewOnTheScreenItselfRecordsWithoutRedirect(t *testing.T) {
	g := newWhatsNewRig(t, "")
	if err := g.h.users.SetSeenVersion(context.Background(), g.userID, g.older); err != nil {
		t.Fatal(err)
	}
	rec := g.do(t, g.latest, g.page("/admin/neuerungen/"+g.latest))
	if rec.Code == http.StatusSeeOther || g.reached != 1 {
		t.Fatalf("status %d reached %d", rec.Code, g.reached)
	}
	if g.seen(t) != g.latest {
		t.Errorf("DB holds %q", g.seen(t))
	}
}

func TestContinueTarget(t *testing.T) {
	keep := []string{"/admin/websites/3/pages?x=1", "/admin/", "/admin"}
	for _, in := range keep {
		if got := continueTarget(in); got != in {
			t.Errorf("continueTarget(%q) = %q, want it kept", in, got)
		}
	}
	for _, in := range []string{
		"//evil.example/x", "https://evil.example/", "/\\evil.example",
		"javascript:alert(1)", "/administrator", "", "/admin/../etc", "/admin/\nx",
	} {
		if got := continueTarget(in); got != "/admin/" {
			t.Errorf("continueTarget(%q) = %q, want /admin/", in, got)
		}
	}
}
