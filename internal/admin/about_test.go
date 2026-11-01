package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/changelog"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// These tests change package-level build state (SetBuild, SetCommit), so none
// of them runs in parallel, and each puts the state back.
func withBuild(t *testing.T, version, commit string) {
	t.Helper()
	web.SetBuild(version, "")
	web.SetCommit(commit)
	t.Cleanup(func() {
		web.SetBuild("dev", "")
		web.SetCommit("")
	})
}

func getAbout(t *testing.T, h *Handler, sm *scs.SessionManager, role string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/ueber", nil)
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role != "" {
			sm.Put(r.Context(), auth.SessionKeyUserRole, role)
		}
		if err := h.HandleAbout(w, r); err != nil {
			t.Errorf("HandleAbout: %v", err)
		}
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestAboutForAReleaseBuild(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	latest, ok := changelog.Latest()
	if !ok {
		t.Skip("no releases")
	}
	withBuild(t, "v"+latest.Version, "abc1234")

	body := getAbout(t, h, sm, "admin")
	for _, want := range []string{
		"v" + latest.Version,
		web.StageOf("v" + latest.Version),
		"<code>abc1234</code>",
		`href="/admin/neuerungen/` + latest.Version + `"`,
		"AGPL-3.0",
		`rel="noopener"`,
		"holzcloud-CMS",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("about page lacks %q", want)
		}
	}
	if !strings.Contains(body, `<title>About`) && !strings.Contains(body, "<h1>About</h1>") {
		t.Error("title About missing")
	}
}

func TestAboutForADevBuild(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	withBuild(t, "dev", "")

	body := getAbout(t, h, sm, "admin")
	if !strings.Contains(body, "Not recorded in this build.") {
		t.Error("the not-recorded sentence is missing")
	}
	if !strings.Contains(body, "This build is not a release") {
		t.Error("the not-a-release sentence is missing")
	}
	if !strings.Contains(body, `href="/admin/neuerungen"`) {
		t.Error("no link to the whole list")
	}
	if strings.Contains(body, "What is new in version") {
		t.Error("a per-version link is offered to a build that has none")
	}
}

func TestAboutIsInTheSettingsListForEveryRole(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	withBuild(t, "dev", "")

	for _, role := range []string{"admin", "editor"} {
		body := getAbout(t, h, sm, role)
		i := strings.Index(body, `href="/admin/ueber"`)
		if i < 0 {
			t.Fatalf("%s: no About entry in the settings list", role)
		}
		entry := body[i:]
		entry = entry[:strings.Index(entry, "</a>")]
		if !strings.Contains(entry, `aria-current="page"`) {
			t.Errorf("%s: the About entry is not marked current: %s", role, entry)
		}
	}
}
