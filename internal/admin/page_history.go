package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// PageRevisionsData is the version history of one page.
type PageRevisionsData struct {
	web.LayoutData
	WebsiteID int64
	Page      *page.Page
	Revisions []page.Revision
}

// HandlePageRevisions lists the stored history of a page.
func (h *Handler) HandlePageRevisions(w http.ResponseWriter, r *http.Request) error {
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

	data := PageRevisionsData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Verlauf – %s", p.Title)),
		WebsiteID:  websiteID,
		Page:       p,
		Revisions:  revisions,
	}
	data.ActiveNav = "pages"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "page_revisions", data)
}

// HandlePageRevisionRestore puts an older state back as the current one.
//
// The restore is an ordinary edit: it goes through UpdatePage, so the state
// being replaced becomes a revision itself and an accidental restore can be
// undone the same way.
func (h *Handler) HandlePageRevisionRestore(w http.ResponseWriter, r *http.Request) error {
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

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID || p.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	rev, err := h.pages.GetRevision(r.Context(), revID)
	if err != nil {
		return err
	}
	if rev == nil || rev.PageID != pageID {
		http.NotFound(w, r)
		return nil
	}

	html, err := page.RenderMarkdown(rev.ContentMarkdown)
	if err != nil {
		return err
	}

	// The revision keeps the slug it had, but the address may have been taken
	// over since. Restoring the text must not fail because of that, so the
	// current address is kept and only the content moves back.
	err = h.pages.UpdatePage(r.Context(), pageID, page.PageUpdate{
		Title:           rev.Title,
		Slug:            p.Slug,
		Markdown:        rev.ContentMarkdown,
		HTML:            html,
		Status:          p.Status,
		ExpectedVersion: p.Version,
		UserID:          h.currentUserID(r),
	})
	switch {
	case errors.Is(err, page.ErrConflict):
		web.SetFlashError(h.sm, r.Context(), "Die Seite wurde gerade geändert. Bitte den Verlauf neu laden und noch einmal versuchen.")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/revisions", websiteID, pageID))
	case err != nil:
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageRevisionRestore,
		EntityType: "page",
		EntityID:   pageID,
		WebsiteID:  &websiteID,
		Metadata:   map[string]any{"fassung": rev.ID, "stand": rev.CreatedAt.Format("02.01.2006 15:04")},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Frühere Fassung wiederhergestellt")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID))
}

// TrashData lists the soft-deleted pages of a website.
type TrashData struct {
	web.LayoutData
	WebsiteID int64
	Pages     []page.Page
	// RetentionDays is shown to the editor so the automatic purge is not a
	// surprise.
	RetentionDays int
}

// HandleTrash renders the trash of a website.
func (h *Handler) HandleTrash(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	pages, err := h.pages.ListTrash(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := TrashData{
		LayoutData:    web.NewLayoutData(r, h.sm, web.Titlef(r, "Papierkorb – %s", ws.Name)),
		WebsiteID:     websiteID,
		Pages:         pages,
		RetentionDays: int(page.TrashRetention.Hours() / 24),
	}
	data.ActiveNav = "trash"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "trash", data)
}

// HandleTrashRestore brings a page back out of the trash.
func (h *Handler) HandleTrashRestore(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.pages.RestorePage(r.Context(), pageID); err != nil {
		if errors.Is(err, page.ErrNotFound) {
			http.NotFound(w, r)
			return nil
		}
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Seite wiederhergestellt")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/trash", websiteID))
}

// HandleTrashPurge destroys one trashed page for good.
func (h *Handler) HandleTrashPurge(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	p, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if p == nil || p.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.pages.PurgePage(r.Context(), pageID); err != nil {
		if errors.Is(err, page.ErrNotFound) {
			http.NotFound(w, r)
			return nil
		}
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Seite endgültig gelöscht")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/trash", websiteID))
}
