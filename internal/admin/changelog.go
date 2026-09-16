package admin

import (
	"net/http"

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
}

// HandleChangelog shows what changed, by version.
//
// Reached by following the version number in the sidebar, which is where an
// operator asks the question. Nothing announces it and nothing marks it unread:
// a self-hosted program that interrupts the person running it to talk about
// itself is one they learn to click past.
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

	data := changelogData{
		LayoutData: web.NewLayoutData(r, h.sm, "What is new"),
		Releases:   releases,
		Shown:      shown,
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
