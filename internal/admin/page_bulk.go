package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// HandlePageBulk applies one action to a set of pages.
//
// Every selected id is checked against the website before anything is written,
// so a hand-edited form cannot reach another site's pages through this route.
func (h *Handler) HandlePageBulk(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	action := r.FormValue("action")
	ids := make([]int64, 0, len(r.PostForm["page_ids"]))
	for _, raw := range r.PostForm["page_ids"] {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		web.SetFlashError(h.sm, r.Context(), "No page selected")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
	}
	// Dieselbe Regel wie im Editor: wer nicht veröffentlichen darf, ändert den
	// Zustand auch nicht über die Sammelaktion — sonst wäre die Auswahlliste
	// die Hintertür zum Knopf, den es oben nicht gibt.
	if (action == "publish" || action == "unpublish") && !h.mayPublish(r) {
		web.SetFlashError(h.sm, r.Context(),
			"Publishing is not part of your account. Submit the page for review.")
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
	}

	userID := h.currentUserID(r)
	var done int
	var skipped []string

	for _, id := range ids {
		p, err := h.pages.GetPage(r.Context(), id)
		if err != nil {
			return err
		}
		// The guard: a page of another website is silently not touched, and the
		// summary says how many were skipped.
		if p == nil || p.WebsiteID != websiteID || p.InTrash() {
			skipped = append(skipped, strconv.FormatInt(id, 10))
			continue
		}

		switch action {
		case "publish":
			err = h.pages.SetPageStatus(r.Context(), id, "published", userID)
		case "unpublish":
			err = h.pages.SetPageStatus(r.Context(), id, "draft", userID)
		case "trash":
			err = h.pages.TrashPage(r.Context(), id)
		default:
			web.SetFlashError(h.sm, r.Context(), "Unknown action")
			return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
		}
		if err != nil {
			skipped = append(skipped, p.Title)
			continue
		}
		done++
	}

	// A bare "done" would hide the fact that some pages were left alone.
	//
	// One whole sentence per action and per number, and no assembly out of a
	// noun and a participle: a language that puts the verb elsewhere, or that
	// inflects the noun after a numeral, cannot be served by gluing two
	// catalogue words together — and the collector sees neither half.
	message := web.Titlef(r, bulkMessage(action, done), done)
	if len(skipped) > 0 {
		message += web.Titlef(r, ", %d skipped", len(skipped))
	}
	web.SetFlashSuccess(h.sm, r.Context(), message)
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages", websiteID))
}

// bulkMessage is the catalogue key for what was done, to how many pages.
func bulkMessage(action string, n int) string {
	one := n == 1
	switch action {
	case "publish":
		if one {
			return i18n.N("%d page published")
		}
		return i18n.N("%d pages published")
	case "unpublish":
		if one {
			return i18n.N("%d page withdrawn")
		}
		return i18n.N("%d pages withdrawn")
	case "trash":
		if one {
			return i18n.N("%d page moved to the trash")
		}
		return i18n.N("%d pages moved to the trash")
	}
	if one {
		return i18n.N("%d page changed")
	}
	return i18n.N("%d pages changed")
}

// HandlePageDuplicate copies a page as a new draft.
//
// The copy is always a draft, whatever the original was: duplicating a live
// page and instantly publishing an unedited copy of it is never what anyone
// meant.
func (h *Handler) HandlePageDuplicate(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	src, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if src == nil || src.WebsiteID != websiteID || src.InTrash() {
		http.NotFound(w, r)
		return nil
	}

	copy, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID: websiteID,
		Title:     src.Title + " (Kopie)",
		// CreatePage uniquifies the slug, so the copy lands on "kontakt-2"
		// rather than failing on the constraint.
		Slug:     src.Slug,
		Markdown: src.ContentMarkdown,
		HTML:     src.ContentHTML,
		Status:   "draft",
		Meta: page.PageMeta{
			Excerpt:         src.Excerpt,
			MetaDescription: src.MetaDescription,
			FeaturedMediaID: src.FeaturedMediaID,
			NoIndex:         src.NoIndex,
		},
		// A copy of a product is a product. Leaving these out would file the
		// duplicate under "Seiten", where the person who just duplicated a
		// product would not go looking for it.
		Kind:    src.Kind,
		TypeKey: src.TypeKey,
		UserID:  h.currentUserID(r),
	})
	if err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Copy created")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, copy.ID))
}

// HandlePageReview marks a draft as awaiting review, or clears the mark.
func (h *Handler) HandlePageReview(w http.ResponseWriter, r *http.Request) error {
	websiteID, pageID, ok := pageIDs(r)
	if !ok {
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

	state := "pending"
	message := web.T(r, "Submitted for review")
	if strings.EqualFold(r.FormValue("clear"), "1") || p.ReviewState == "pending" {
		state, message = "none", web.T(r, "Review note removed")
	}

	if err := h.pages.SetReviewState(r.Context(), pageID, state); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), message)
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", websiteID, pageID))
}
