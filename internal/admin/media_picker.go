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

// MediaPickerData is the fragment shown inside the editor's picker dialog.
type MediaPickerData struct {
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

	return web.RenderPartial(w, h.templates, r, "media_picker", MediaPickerData{
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

	markdown := strings.TrimRight(p.ContentMarkdown, "\n") + "\n\n" + m.MarkdownRef() + "\n"
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		return err
	}

	// Saving through the normal path keeps the version, the revision and the
	// media bookkeeping consistent with an ordinary edit.
	err = h.pages.UpdatePage(r.Context(), pageID, page.PageUpdate{
		Title:           p.Title,
		Slug:            p.Slug,
		Markdown:        markdown,
		HTML:            html,
		Status:          p.Status,
		Meta:            page.PageMeta{Excerpt: p.Excerpt, MetaDescription: p.MetaDescription, FeaturedMediaID: p.FeaturedMediaID, NoIndex: p.NoIndex},
		Schedule:        page.PageSchedule{PublishAt: p.PublishAt, UnpublishAt: p.UnpublishAt},
		ExpectedVersion: p.Version,
		UserID:          h.currentUserID(r),
	})
	if err != nil {
		return err
	}
	h.recordMediaUsage(r, websiteID, pageID, html)

	if m.NeedsAltText() {
		web.SetFlashWarning(h.sm, r.Context(),
			"Eingefügt. Dieses Bild hat noch keine Beschreibung – bitte in der Mediathek nachtragen.")
	} else {
		web.SetFlashSuccess(h.sm, r.Context(), "Bild eingefügt")
	}
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID))
}
