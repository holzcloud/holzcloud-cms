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
	for _, part := range entryParts(newest) {
		if !strings.Contains(body, part) {
			t.Error("the newest entry's text is not on the page")
		}
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

	for _, part := range entryParts(older) {
		if !strings.Contains(rec.Body.String(), part) {
			t.Errorf("the entry for %q is not on the page", older.Version)
		}
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
		for _, part := range entryParts(newest) {
			if !strings.Contains(rec.Body.String(), part) {
				t.Errorf("%q: did not fall back to the newest entry", asked)
			}
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
		if !strings.Contains(rec.Body.String(), `class="btn btn--primary" `+want) {
			t.Errorf("weiter=%q: Continue link %s missing", weiter, want)
		}
		if strings.Contains(rec.Body.String(), "evil.example") {
			t.Errorf("weiter=%q leaked into the page", weiter)
		}
	}

	rec := serve(t, h, sm, h.HandleChangelog,
		httptest.NewRequest(http.MethodGet, "/admin/neuerungen", nil))
	if strings.Contains(rec.Body.String(), `class="btn btn--primary"`) {
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

// entryParts is what an entry puts on the screen: its opening and each group.
// Not Release.HTML, which is the whole body in file order, because the screen
// shows the groups in its own order.
func entryParts(rel changelog.Release) []string {
	var parts []string
	if rel.Lead != "" {
		parts = append(parts, string(rel.Lead))
	}
	for _, g := range rel.Groups {
		parts = append(parts, string(g.HTML))
	}
	return parts
}

// One tab per main version, newest first, and exactly one of them is current.
func TestChangelogHasOneTabPerMainVersion(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, 1)

	body := changelogPage(t, h, sm, "", "")
	body = body[strings.Index(body, `<nav class="whatsnew-tabs"`):]
	nav, _, _ := strings.Cut(body, "</nav>")

	var want []string
	seen := map[string]bool{}
	for _, rel := range all {
		if !seen[rel.Series()] {
			seen[rel.Series()] = true
			want = append(want, changelog.SeriesLabel(rel.Series()))
		}
	}
	var got []string
	for _, part := range strings.Split(nav, `class="whatsnew-tab__number">`)[1:] {
		got = append(got, strings.SplitN(part, "<", 2)[0])
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("tabs = %v, want one per main version %v", got, want)
	}
	if n := strings.Count(nav, `aria-current="page"`); n != 1 {
		t.Errorf("current tabs = %d, want 1", n)
	}
	// A tab is a series, never a single version.
	for _, rel := range all {
		if strings.Contains(nav, ">"+rel.Version+"<") && rel.Version != changelog.SeriesLabel(rel.Series()) {
			t.Errorf("version %s has a tab of its own", rel.Version)
		}
	}
}

// The versions of the shown series are a list of cards below the tabs, and only
// the newest is open when nothing was asked for.
func TestChangelogListsTheVersionsOfTheShownSeries(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, 1)
	newest := all[0]

	body := changelogPage(t, h, sm, "", "")
	for _, rel := range all {
		inSeries := rel.Series() == newest.Series()
		has := strings.Contains(body, `id="v`+rel.Version+`"`)
		if has != inSeries {
			t.Errorf("version %s: card on the page = %v, want %v", rel.Version, has, inSeries)
		}
	}
	if n := strings.Count(body, `class="whatsnew-card`); n == 0 {
		t.Fatal("no cards")
	}
	if !strings.Contains(body, `id="v`+newest.Version+`" open`) {
		t.Error("the newest version is not open")
	}
	if n := strings.Count(body, `" open>`); n != 1 {
		t.Errorf("open cards = %d, want 1", n)
	}
}

// Asking for an old version shows its series and opens its card.
func TestChangelogOpensTheVersionAskedInItsOwnSeries(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, 1)
	oldest := all[len(all)-1]

	body := changelogPage(t, h, sm, oldest.Version, "")
	if !strings.Contains(body, `id="v`+oldest.Version+`" open`) {
		t.Errorf("the card of %s is not open", oldest.Version)
	}
	if strings.Contains(body, `id="v`+all[0].Version+`"`) && all[0].Series() != oldest.Series() {
		t.Error("a version of another series is on the page")
	}
	tab := tabSlice(body, `<a class="whatsnew-tab is-active"`, `</a>`)
	if !strings.Contains(tab, changelog.SeriesLabel(oldest.Series())) {
		t.Errorf("the current tab is not the series of %s: %s", oldest.Version, tab)
	}
}

// New things come first, then what was improved, then what was fixed.
func TestChangelogShowsNewBeforeImprovedBeforeFixed(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	for _, rel := range changelog.All() {
		rank := map[changelog.Kind]int{}
		for i, g := range rel.Groups {
			if _, ok := rank[g.Kind]; !ok {
				rank[g.Kind] = i
			}
		}
		n, okN := rank[changelog.KindNew]
		f, okF := rank[changelog.KindFixed]
		if okN && okF && n > f {
			t.Errorf("%s: the changelog puts fixed before new", rel.Version)
		}
	}
	body := changelogPage(t, h, sm, "0.1.2", "")
	card := body[strings.Index(body, `id="v0.1.2"`):]
	card, _, _ = strings.Cut(card, "</details>")
	improved, fixed := strings.Index(card, "whatsnew-group--improved"), strings.Index(card, "whatsnew-group--fixed")
	if improved < 0 || fixed < 0 || improved > fixed {
		t.Errorf("0.1.2: improved at %d, fixed at %d; want improved first", improved, fixed)
	}
}

func TestChangelogMarksOnlyNewerVersionsAsNew(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	all := needReleases(t, 2)
	since := all[2].Version
	if all[0].Series() != all[2].Series() {
		t.Skip("the three newest are not in one series")
	}

	body := changelogPage(t, h, sm, "", "?seit="+since+"&weiter=%2Fadmin%2Fwebsites")
	// The two newer cards carry the chip, the one the person was on does not.
	if n := strings.Count(body, "whatsnew-chip--new"); n != 2 {
		t.Errorf("new chips = %d, want 2", n)
	}
	// The series tab carries a dot, and keeps the welcome state in its link.
	tab := tabSlice(body, `<a class="whatsnew-tab is-active"`, `</a>`)
	if !strings.Contains(tab, "whatsnew-tab__new") {
		t.Error("the tab of a series with news has no mark")
	}
	if !strings.Contains(tab, "seit="+since) || !strings.Contains(tab, "weiter=%2Fadmin%2Fwebsites") {
		t.Errorf("the tab lost seit/weiter: %s", tab)
	}
	if !strings.Contains(body, all[0].Version) || !strings.Contains(body, since) {
		t.Error("welcome header does not name both versions")
	}
}

func TestChangelogWelcomeWithAnUnknownPreviousVersion(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	needReleases(t, 1)

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
