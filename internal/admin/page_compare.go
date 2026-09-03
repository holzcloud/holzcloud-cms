package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/textdiff"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// diffContext is how many unchanged lines stay beside a change. Three is the
// usual answer and the reason it is usual: enough to recognise the paragraph,
// little enough that the changes stay close together.
const diffContext = 3

// RevisionSide is one end of a comparison, named for the reader.
type RevisionSide struct {
	// ID is the revision, or zero for the page as it stands now.
	ID    int64
	When  string
	Who   string
	Label string
	Title string
	// Current marks the side that is the page itself rather than a revision.
	Current bool
}

// PageCompareData is the comparison of two versions of a page.
type PageCompareData struct {
	web.LayoutData
	WebsiteID int64
	Page      *page.Page

	From RevisionSide
	To   RevisionSide

	// TextDiff and BlockDiff are empty where that half did not change, which
	// is what the screen shows instead of an empty table.
	TextDiff  []textdiff.Line
	BlockDiff []textdiff.Line

	// Unchanged is true when neither half differs. Two versions can be
	// genuinely identical — a save that changed only the title, for one.
	Unchanged bool
}

// HandlePageRevisionCompare shows what changed between two versions.
//
// Both ends come from the query string: "von" and "bis", each a revision id or
// the word "aktuell" for the page as it stands. Anything unreadable falls back
// to comparing the newest revision against the current page, which is the
// comparison somebody who arrived here without choosing anything wanted.
func (h *Handler) HandlePageRevisionCompare(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if ws == nil || p == nil || p.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	revisions, err := h.pages.ListRevisions(r.Context(), pageID)
	if err != nil {
		return err
	}

	q := r.URL.Query()
	fromRev := h.pickRevision(q.Get("von"), revisions, len(revisions) > 0)
	toRev := h.pickRevision(q.Get("bis"), revisions, false)

	fromText, fromBlocks, fromSide := versionOf(fromRev, p)
	toText, toBlocks, toSide := versionOf(toRev, p)

	fromBlockText, err := block.RenderForDiff(fromBlocks)
	if err != nil {
		return err
	}
	toBlockText, err := block.RenderForDiff(toBlocks)
	if err != nil {
		return err
	}

	data := PageCompareData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Vergleich – %s", p.Title)),
		WebsiteID:  websiteID,
		Page:       p,
		From:       fromSide,
		To:         toSide,
	}
	if textdiff.Changed(fromText, toText) {
		data.TextDiff = textdiff.Compact(textdiff.Lines(fromText, toText), diffContext)
	}
	if textdiff.Changed(fromBlockText, toBlockText) {
		data.BlockDiff = textdiff.Compact(textdiff.Lines(fromBlockText, toBlockText), diffContext)
	}
	data.Unchanged = len(data.TextDiff) == 0 && len(data.BlockDiff) == 0

	data.ActiveNav = "pages"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "page_revision_compare", data)
}

// pickRevision reads one end of the comparison out of the query string.
//
// preferOldest decides what an unreadable or missing value means: for the left
// end it is the newest stored revision, for the right end the page as it
// stands. That pair is the comparison "what changed since the last save",
// which is what the button on the history offers.
func (h *Handler) pickRevision(raw string, revisions []page.Revision, preferOldest bool) *page.Revision {
	raw = strings.TrimSpace(raw)
	if raw != "" && raw != "aktuell" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			for i := range revisions {
				if revisions[i].ID == id {
					return &revisions[i]
				}
			}
		}
	}
	if raw == "aktuell" || !preferOldest || len(revisions) == 0 {
		return nil
	}
	return &revisions[0]
}

// versionOf returns the text, the blocks and the label of one end. A nil
// revision means the page as it stands now.
func versionOf(rev *page.Revision, p *page.Page) (text, blocks string, side RevisionSide) {
	if rev == nil {
		return p.ContentMarkdown, p.Blocks, RevisionSide{
			When:    "jetzt",
			Title:   p.Title,
			Current: true,
		}
	}
	return rev.ContentMarkdown, rev.Blocks, RevisionSide{
		ID:    rev.ID,
		When:  rev.CreatedAt.Format("02.01.2006 15:04"),
		Who:   rev.UserEmail,
		Label: rev.Label,
		Title: rev.Title,
	}
}

// HandlePageRevisionLabel names a version, or clears the name.
//
// Naming is not an edit of the page: the history stays as it was and nothing
// new is written to it. Otherwise going through the history to label the
// interesting versions would keep adding versions of its own.
func (h *Handler) HandlePageRevisionLabel(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	revID, err := strconv.ParseInt(r.PathValue("revID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	// Die Fassung muss zu dieser Seite gehören. Ohne die Prüfung liesse sich
	// über eine geratene Kennung die Fassung einer fremden Website benennen.
	rev, err := h.pages.GetRevision(r.Context(), revID)
	if err != nil {
		return err
	}
	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if rev == nil || p == nil || rev.PageID != pageID || p.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.pages.LabelRevision(r.Context(), revID, r.FormValue("label")); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), web.T(r, "Beschriftung gespeichert"))
	return h.redirect(w, r, revisionsPath(websiteID, pageID))
}

func revisionsPath(websiteID, pageID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) +
		"/pages/" + strconv.FormatInt(pageID, 10) + "/revisions"
}
