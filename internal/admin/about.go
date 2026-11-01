package admin

import (
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/changelog"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// aboutData is Settings, About.
type aboutData struct {
	web.LayoutData
	// Commit is the commit the binary was built from, empty if not recorded.
	Commit string
	// Release is the changelog entry of the running version; IsRelease says
	// whether there is one. A dev build or a git-describe suffix has none.
	Release   changelog.Release
	IsRelease bool
	// Latest is the newest entry in the file, for the link to the whole list.
	Latest changelog.Release
}

// HandleAbout says which program runs here: name, version, stage, commit,
// licence and where the source is.
//
// No website is involved and no right is needed beyond being signed in, for the
// reason HandleChangelog gives. This is also where What's new can always be
// opened again, however long ago the automatic showing happened.
func (h *Handler) HandleAbout(w http.ResponseWriter, r *http.Request) error {
	data := aboutData{
		LayoutData: web.NewLayoutData(r, h.sm, "About"),
		Commit:     web.BuildCommit(),
	}
	data.ActiveNav = "about"
	data.Release, data.IsRelease = changelog.Find(data.Version)
	data.Latest, _ = changelog.Latest()
	return web.RenderAdmin(w, h.templates, r, "about", data)
}
