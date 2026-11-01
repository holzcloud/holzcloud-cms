package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/changelog"
)

// The screen renders, against the real templates and the real CHANGELOG.md.
//
// Not a formality: the entries are 44 KB of hand-written Markdown that nothing
// else in the suite reads, and the one way this page can break is a heading
// somebody types differently. It fails here rather than in the browser, which
// is what newTestAdmin exists for.
func TestChangelogScreenShowsTheNewestVersion(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	newest, ok := changelog.Latest()
	if !ok {
		t.Fatal("CHANGELOG.md parsed to no releases at all")
	}

	rec := serve(t, h, sm, h.HandleChangelog,
		httptest.NewRequest(http.MethodGet, "/admin/neuerungen", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, newest.Version) {
		t.Errorf("the newest version %q is not on the page", newest.Version)
	}
	if !strings.Contains(body, string(newest.HTML)) {
		t.Error("the newest entry's text is not on the page")
	}
}

// A version in the address shows that version, which is what makes the link
// worth sending to somebody who is still on the older build.
func TestChangelogScreenShowsTheVersionAsked(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	all := changelog.All()
	if len(all) < 2 {
		t.Skip("one release in the file; nothing to choose between")
	}
	older := all[len(all)-1]

	req := httptest.NewRequest(http.MethodGet, "/admin/neuerungen/"+older.Version, nil)
	req.SetPathValue("version", older.Version)
	rec := serve(t, h, sm, h.HandleChangelog, req)

	if !strings.Contains(rec.Body.String(), string(older.HTML)) {
		t.Errorf("the entry for %q is not on the page", older.Version)
	}
}

// A version that does not exist is the newest entry, not a 404.
//
// The address is reached by following a link, so a number that does not resolve
// means the list has moved on rather than that the operator mistyped something.
// And it must not become a path: whatever stands in the URL is a map key here
// and nothing is built out of it.
func TestAnUnknownVersionFallsBackToTheNewest(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	newest, _ := changelog.Latest()

	for _, asked := range []string{"99.9", "../../etc/passwd", "", "<script>"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/neuerungen/x", nil)
		req.SetPathValue("version", asked)
		rec := serve(t, h, sm, h.HandleChangelog, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%q: status = %d, want 200", asked, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), string(newest.HTML)) {
			t.Errorf("%q: did not fall back to the newest entry", asked)
		}
	}
}

// The version in the sidebar leads here. That link is the whole discoverability
// of this screen — nothing else points at it, deliberately — so it is asserted
// rather than left to a template edit.
func TestTheSidebarVersionLinksToTheChangelog(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	rec := serve(t, h, sm, h.HandleChangelog,
		httptest.NewRequest(http.MethodGet, "/admin/neuerungen", nil))

	if !strings.Contains(rec.Body.String(), `href="/admin/neuerungen"`) {
		t.Error("the sidebar no longer links the version number to this screen")
	}
}

// After an update the screen carries a Continue link back to the page that was
// interrupted, and that link only ever points inside /admin.
func TestChangelogContinueLinkStaysInsideTheAdmin(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := changelog.All()
	if len(all) < 2 {
		t.Skip("needs two releases")
	}
	older := all[1].Version

	cases := map[string]string{
		"/admin/websites": `href="/admin/websites"`,
		"//evil.example":  `href="/admin/"`,
	}
	for weiter, want := range cases {
		target := "/admin/neuerungen?seit=" + older + "&weiter=" + url.QueryEscape(weiter)
		rec := serve(t, h, sm, h.HandleChangelog, httptest.NewRequest(http.MethodGet, target, nil))
		if !strings.Contains(rec.Body.String(), `class="btn btn-primary" `+want) {
			t.Errorf("weiter=%q: Continue link %s missing", weiter, want)
		}
		if strings.Contains(rec.Body.String(), "evil.example") {
			t.Errorf("weiter=%q leaked into the page", weiter)
		}
	}

	rec := serve(t, h, sm, h.HandleChangelog,
		httptest.NewRequest(http.MethodGet, "/admin/neuerungen", nil))
	if strings.Contains(rec.Body.String(), `class="btn btn-primary"`) {
		t.Error("a Continue link is rendered without seit")
	}
}

func changelogPage(t *testing.T, h *Handler, sm *scs.SessionManager, version, query string) string {
	t.Helper()
	target := "/admin/neuerungen"
	if version != "" {
		target += "/" + version
	}
	req := httptest.NewRequest(http.MethodGet, target+query, nil)
	req.SetPathValue("version", version)
	rec := serve(t, h, sm, h.HandleChangelog, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	return rec.Body.String()
}

func needReleases(t *testing.T, n int) []changelog.Release {
	t.Helper()
	all := changelog.All()
	if len(all) <= n {
		t.Skipf("needs more than %d releases", n)
	}
	return all
}

// The newest versions are tabs, the rest sits in the archive, and exactly one
// entry anywhere is current.
func TestChangelogTabsAndArchive(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, recentTabs)

	body := changelogPage(t, h, sm, "", "")
	// The rail and the settings list carry their own aria-current; count only
	// what the screen itself renders.
	body = body[strings.Index(body, `<nav class="whatsnew-tabs"`):]
	tabs, archive, ok := strings.Cut(body, `<details class="whatsnew-archive"`)
	if !ok {
		t.Fatal("no archive disclosure")
	}
	for _, rel := range all[:recentTabs] {
		if !strings.Contains(tabs, `href="/admin/neuerungen/`+rel.Version+`"`) {
			t.Errorf("tab for %s missing", rel.Version)
		}
	}
	for _, rel := range all[recentTabs:] {
		if strings.Contains(tabs, `href="/admin/neuerungen/`+rel.Version+`"`) {
			t.Errorf("%s is a tab, should be archived", rel.Version)
		}
		if !strings.Contains(archive, `href="/admin/neuerungen/`+rel.Version+`"`) {
			t.Errorf("%s missing from the archive", rel.Version)
		}
	}
	if !strings.Contains(archive, "<h3>"+all[recentTabs].Series()+".x</h3>") {
		t.Error("archive is not grouped by series")
	}
	if n := strings.Count(body, `aria-current="page"`); n != 1 {
		t.Errorf("aria-current count = %d, want 1", n)
	}
	if strings.Contains(strings.SplitN(archive, ">", 2)[0], " open") {
		t.Error("archive is open although the newest is shown")
	}
}

func TestChangelogOpensTheArchiveForAnOldVersion(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, recentTabs)
	oldest := all[len(all)-1].Version

	body := changelogPage(t, h, sm, oldest, "")
	body = body[strings.Index(body, `<nav class="whatsnew-tabs"`):]
	if !strings.Contains(body, `<details class="whatsnew-archive" open>`) {
		t.Error("archive is closed although the shown version lives in it")
	}
	tabs, _, _ := strings.Cut(body, `<details class="whatsnew-archive"`)
	if strings.Contains(tabs, `aria-current="page"`) {
		t.Error("a tab is current although the shown version is archived")
	}
	if n := strings.Count(body, `aria-current="page"`); n != 1 {
		t.Errorf("aria-current count = %d, want 1", n)
	}
}

func TestChangelogMarksOnlyNewerVersionsAsNew(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, recentTabs)
	since := all[2].Version

	body := changelogPage(t, h, sm, "", "?seit="+since+"&weiter=%2Fadmin%2Fwebsites")
	if n := strings.Count(body, `class="whatsnew-tab__new"`); n != 2 {
		t.Errorf("new marks = %d, want 2", n)
	}
	// Both newer tabs carry one; the third does not.
	for i, rel := range all[:3] {
		tab := tabSlice(body, `href="/admin/neuerungen/`+rel.Version, `</a>`)
		if got := strings.Contains(tab, "whatsnew-tab__new"); got != (i < 2) {
			t.Errorf("%s: new mark = %v", rel.Version, got)
		}
		if !strings.Contains(tab, "seit="+since) || !strings.Contains(tab, "weiter=%2Fadmin%2Fwebsites") {
			t.Errorf("%s: link lost seit/weiter: %s", rel.Version, tab)
		}
	}
	if !strings.Contains(body, all[0].Version) || !strings.Contains(body, since) {
		t.Error("welcome header does not name both versions")
	}
}

func TestChangelogWelcomeWithAnUnknownPreviousVersion(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	needReleases(t, recentTabs)

	body := changelogPage(t, h, sm, "", "?seit=weird")
	if !strings.Contains(body, "whatsnew-hero") {
		t.Error("no welcome header")
	}
	if strings.Contains(body, "You were on version") {
		t.Error("previous-version sentence rendered for an unknown version")
	}
	if strings.Contains(body, "whatsnew-tab__new") {
		t.Error("new marks rendered for an unknown previous version")
	}
}

// tabSlice returns the text from the first occurrence of start to the next end.
func tabSlice(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i:]
	if j := strings.Index(rest, end); j >= 0 {
		return rest[:j]
	}
	return rest
}
