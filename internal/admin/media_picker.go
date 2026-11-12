package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// mediaPickerData is the fragment shown inside the editor's picker dialog.
type mediaPickerData struct {
	WebsiteID int64
	PageID    int64
	Media     []media.Media
	CSRFToken string
	Filter    media.Filter
	// More is true when the site has files beyond this page of the picker, so
	// the fragment can point at the full library instead of pretending it
	// showed everything.
	More bool
}

// pickerLimit is how many files the dialog shows at once. It is a dialog, not
// the library: more than this and the useful move is to search.
const pickerLimit = 24

// HandleMediaPicker returns the picker fragment.
//
// It exists because inserting an image was the most repeated editorial action
// and the most hostile path in the admin: navigate away (losing everything
// typed), upload, select a read-only text field by hand, copy, navigate back,
// type the Markdown from memory.
func (h *Handler) HandleMediaPicker(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	pageID, _ := strconv.ParseInt(r.URL.Query().Get("page_id"), 10, 64)

	filter := mediaFilterFromRequest(r)
	items, total, err := h.mediaStore.List(r.Context(), websiteID, filter, 1, pickerLimit)
	if err != nil {
		return err
	}

	return web.RenderPartial(w, h.templates, r, "media_picker", mediaPickerData{
		WebsiteID: websiteID,
		PageID:    pageID,
		Media:     items,
		CSRFToken: web.CSRFTokenFromRequest(r),
		Filter:    filter,
		More:      total > len(items),
	})
}

// HandlePageInsertMedia appends a media reference to a page's Markdown.
//
// Appending rather than inserting at the caret is a real limitation, and a
// deliberate one: placing text at the cursor needs script, and the only way to
// wire that from a template would be an inline handler, which means loosening
// script-src away from 'self'. Appending needs no new JavaScript at all, works
// from the keyboard, and still beats copying a URL across two screens.
//
// The save starts from keepUpdate, the save that changes nothing, and changes
// only the text: UpdatePage writes every column on every save, so a column left
// out of a hand-built update is a column emptied. That is how this handler used
// to wipe a page's own fields, which no revision brings back.
//
// A page built from blocks is refused, and nothing is written. Both content
// columns are derived from the blocks there, so a reference appended to the
// Markdown would vanish with the next save in the block editor — and until
// then the public page would show the plain text instead of the blocks. The
// block editor has image fields of its own.
func (h *Handler) HandlePageInsertMedia(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}
	// A picture in a live page is a change to the live page (rights.go,
	// liveLocked).
	if h.liveLocked(r, p) {
		web.SetFlashError(h.sm, r.Context(), liveLockedSentence())
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID))
	}

	mediaID, err := strconv.ParseInt(r.FormValue("media_id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	m, err := h.mediaStore.GetByID(r.Context(), mediaID)
	if err != nil {
		return err
	}
	if m == nil || m.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	editAddress := fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID)

	// The raw column and not the decoded list: whatever stands in it is
	// something a Markdown save would destroy.
	if strings.TrimSpace(p.Blocks) != "" {
		web.SetFlashError(h.sm, r.Context(),
			"This page is built from blocks. Add the image in a block of the editor instead.")
		return h.redirect(w, r, editAddress)
	}

	markdown := strings.TrimRight(p.ContentMarkdown, "\n") + "\n\n" + m.MarkdownRef() + "\n"
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		return err
	}

	// Saving through the normal path keeps the version, the revision and the
	// media bookkeeping consistent with an ordinary edit.
	update := keepUpdate(p)
	update.Markdown = markdown
	update.HTML = html
	update.UserID = h.currentUserID(r)
	if err := h.pages.UpdatePage(r.Context(), pageID, update); err != nil {
		return err
	}
	h.recordMediaUsage(r, websiteID, pageID, html)

	if m.NeedsAltText() {
		web.SetFlashWarning(h.sm, r.Context(),
			"Inserted. This image has no description yet — please add one in the media library.")
	} else {
		web.SetFlashSuccess(h.sm, r.Context(), "Image inserted")
	}
	return h.redirect(w, r, editAddress)
}
