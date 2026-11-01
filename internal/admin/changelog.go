package admin

import (
	"net/http"
	"net/url"

	"github.com/holzcloud/holzcloud-cms/internal/changelog"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// changelogData is the screen behind the version number in the sidebar.
type changelogData struct {
	web.LayoutData
	// Releases is every version, newest first. The whole list, because the
	// question an operator has after an update is rarely about one version: a
	// server that was three releases behind has three to read.
	Releases []changelog.Release
	// Shown is the one whose entry is on the screen.
	Shown changelog.Release
	// Running says whether Shown is the version this binary actually is. It is
	// what makes the page answer "what am I running" and not only "what
	// exists": a build from a branch, or a `dev` build, matches nothing here
	// and then nothing is marked.
	Running bool
	// Welcome is set when the person was sent here by an update (the request
	// carries "seit"), which turns on the welcome header and its Continue link.
	Welcome bool
	// Since is the version the person was on before, zero when unknown.
	Since changelog.Release
	// Continue is where Continue leads: the page that was interrupted, and only
	// ever a path inside /admin.
	Continue string
	// Tabs are the newest recentTabs versions, shown as the tab strip.
	Tabs []versionTab
	// Archive is every older version, grouped by series, inside a native
	// disclosure.
	Archive []versionGroup
	// ArchiveOpen opens the disclosure when the shown version lives in it, so
	// the current entry is never hidden behind a closed summary.
	ArchiveOpen bool
}

// recentTabs is how many versions get a tab of their own. More would wrap into
// a wall; the rest are one click away in the archive.
const recentTabs = 6

// versionTab is one version as a link: to its own address, marked current when
// it is the one on the screen and new when it came after the version the person
// was on before.
type versionTab struct {
	Release     changelog.Release
	Href        string
	Active, New bool
}

// versionGroup is one series of the archive.
type versionGroup struct {
	Series string
	Tabs   []versionTab
}

// HandleChangelog shows what changed, by version.
//
// Reached three ways: once after an update, when WhatsNewOnUpdate sends the
// person here with the version they were on ("seit") and the page they were
// heading for ("weiter"); by following the version number in the sidebar; and
// from Settings, About. Only the first carries the welcome header and the
// Continue link.
//
// No website is involved and no right is needed beyond being signed in. What
// this program changed between two versions is not a secret from an editor —
// they are the ones who will notice the difference.
//
// An unknown version is the newest one rather than a 404. The address is
// reached by a link, so a number that does not resolve means the list has moved
// on, not that the operator made a mistake; showing them the newest entry
// answers the question they had.
func (h *Handler) HandleChangelog(w http.ResponseWriter, r *http.Request) error {
	releases := changelog.All()

	shown, ok := changelog.Find(r.PathValue("version"))
	if !ok {
		shown, ok = changelog.Latest()
	}

	q := r.URL.Query()
	data := changelogData{
		LayoutData: web.NewLayoutData(r, h.sm, "What is new"),
		Releases:   releases,
		Shown:      shown,
		Welcome:    q.Has("seit"),
		Continue:   continueTarget(q.Get("weiter")),
	}
	data.Since, _ = changelog.Find(q.Get("seit"))
	data.ActiveNav = "about"
	if ok {
		buildTabs(&data, r.URL.Query())
	}
	if ok {
		// The sidebar prints whatever the build stamped, which is a tag name
		// ("v2.2"), a `git describe` ("v2.2-3-gabc1234") or "dev". Find strips
		// the v and matches the first; the other two match nothing, which is
		// the truthful answer for a build that is not a release.
		if running, found := changelog.Find(data.LayoutData.Version); found {
			data.Running = running.Version == shown.Version
		}
	}
	return web.RenderAdmin(w, h.templates, r, "changelog", data)
}

// buildTabs fills the tab strip and the archive. Done in Go and not in the
// template: which version is new, current or in the archive is a decision, and
// a template should print decisions, not make them.
//
// seit and weiter are carried into every link, so the welcome state survives
// flipping between tabs; without them the links are bare addresses.
func buildTabs(data *changelogData, q url.Values) {
	keep := url.Values{}
	if q.Has("seit") {
		keep.Set("seit", q.Get("seit"))
	}
	if q.Has("weiter") {
		// The guarded value, not the raw one: a link should never carry
		// something the Continue button would refuse.
		keep.Set("weiter", data.Continue)
	}
	suffix := ""
	if len(keep) > 0 {
		suffix = "?" + keep.Encode()
	}

	sinceIdx := -1
	all := data.Releases
	if data.Since.Version != "" {
		for i, rel := range all {
			if rel.Version == data.Since.Version {
				sinceIdx = i
			}
		}
	}

	for i, rel := range all {
		t := versionTab{
			Release: rel,
			Href:    "/admin/neuerungen/" + url.PathEscape(rel.Version) + suffix,
			Active:  rel.Version == data.Shown.Version,
			New:     sinceIdx >= 0 && i < sinceIdx,
		}
		if i < recentTabs {
			data.Tabs = append(data.Tabs, t)
			continue
		}
		if t.Active {
			data.ArchiveOpen = true
		}
		series := rel.Series()
		if n := len(data.Archive); n > 0 && data.Archive[n-1].Series == series {
			data.Archive[n-1].Tabs = append(data.Archive[n-1].Tabs, t)
			continue
		}
		data.Archive = append(data.Archive, versionGroup{Series: series, Tabs: []versionTab{t}})
	}
}
