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
	// Series are the tabs: one per main version (0.1.0, 0.0.x, 2.x ...), newest
	// first. A tab leads to the newest version of its series.
	Series []seriesTab
	// Cards are the versions of the series on the screen, newest first. Every
	// one of them is a card, so the whole series reads as one list.
	Cards []versionCard
}

// seriesTab is one main version as a link: to the newest version it holds,
// marked current when the shown version belongs to it and new when it holds
// something the person has not seen yet.
type seriesTab struct {
	Label  string
	Href   string
	Count  int
	Active bool
	New    bool
	// label is the series as the changelog names it, which the tab's own Label
	// is a spelling of.
	label string
}

// versionCard is one version of the shown series. Open is the one the person
// asked for, or the newest when they asked for none; the others are closed
// disclosures a click away, so a long series stays one screen long.
type versionCard struct {
	Release changelog.Release
	Href    string
	Open    bool
	New     bool
	Running bool
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

// buildTabs fills the tabs and the cards. Done in Go and not in the template:
// which version is new, current or open is a decision, and a template should
// print decisions, not make them.
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
	href := func(version string) string {
		return "/admin/neuerungen/" + url.PathEscape(version) + suffix
	}

	sinceIdx := -1
	if data.Since.Version != "" {
		for i, rel := range data.Releases {
			if rel.Version == data.Since.Version {
				sinceIdx = i
			}
		}
	}

	var running string
	if rel, found := changelog.Find(data.LayoutData.Version); found {
		running = rel.Version
	}

	shownSeries := data.Shown.Series()
	for i, rel := range data.Releases {
		isNew := sinceIdx >= 0 && i < sinceIdx
		series := rel.Series()

		// The releases come newest first, so the first one of a series is the
		// one its tab leads to, and a series is a run.
		if n := len(data.Series); n == 0 || data.Series[n-1].label != series {
			data.Series = append(data.Series, seriesTab{
				Label:  changelog.SeriesLabel(series),
				Href:   href(rel.Version),
				Active: series == shownSeries,
				label:  series,
			})
		}
		tab := &data.Series[len(data.Series)-1]
		tab.Count++
		tab.New = tab.New || isNew

		if series != shownSeries {
			continue
		}
		data.Cards = append(data.Cards, versionCard{
			Release: rel,
			Href:    href(rel.Version),
			Open:    rel.Version == data.Shown.Version,
			New:     isNew,
			Running: rel.Version == running,
		})
	}
}
