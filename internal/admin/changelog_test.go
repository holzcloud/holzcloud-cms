package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
