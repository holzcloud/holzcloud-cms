package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// TermListData is the label overview.
type TermListData struct {
	web.LayoutData
	WebsiteID int64
	Terms     []term.Term
}

// HandleTermList shows every label of a website with how often it is used.
//
// The screen exists mostly to find the mistakes: "Möbel" next to "Moebel" is
// two archives where the editor meant one, and nothing else in the admin would
// ever show them side by side.
func (h *Handler) HandleTermList(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	terms, err := h.terms.ListAll(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := TermListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Schlagwörter – %s", ws.Name)),
		WebsiteID:  websiteID,
		Terms:      terms,
	}
	data.ActiveNav = "terms"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "term_list", data)
}

// HandleTermRename changes a label's visible name, keeping its address.
func (h *Handler) HandleTermRename(w http.ResponseWriter, r *http.Request) error {
	websiteID, termID, ok, err := h.lookupTerm(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	redirect := fmt.Sprintf("/admin/websites/%d/tags", websiteID)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		web.SetFlashError(h.sm, r.Context(), "Ein Schlagwort braucht einen Namen")
		return h.redirect(w, r, redirect)
	}
	if err := h.terms.Rename(r.Context(), websiteID, termID, name); err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Umbenennen fehlgeschlagen: %s", err))
		return h.redirect(w, r, redirect)
	}

	// The address stays as it was, which is worth saying: an editor renaming a
	// label usually expects the URL to follow, and it deliberately does not.
	web.SetFlashSuccess(h.sm, r.Context(),
		"Umbenannt. Die Adresse bleibt unverändert, damit bestehende Links weiter funktionieren.")
	return h.redirect(w, r, redirect)
}

// HandleTermDelete removes a label from every item that carries it.
func (h *Handler) HandleTermDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, termID, ok, err := h.lookupTerm(w, r)
	if err != nil || !ok {
		return err
	}
	if err := h.terms.Delete(r.Context(), websiteID, termID); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Schlagwort gelöscht. Die Inhalte selbst bleiben.")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/tags", websiteID))
}

// lookupTerm resolves the route values and checks the label belongs to the
// website in the path.
func (h *Handler) lookupTerm(w http.ResponseWriter, r *http.Request) (websiteID, termID int64, ok bool, err error) {
	websiteID, _, ok, err = h.lookupWebsite(w, r)
	if err != nil || !ok {
		return 0, 0, false, err
	}
	termID, err = strconv.ParseInt(r.PathValue("termID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, 0, false, nil
	}
	return websiteID, termID, true, nil
}

// lookupWebsite resolves the {id} route value to a website.
//
// Several screens repeated this block; sharing it means a new one cannot
// forget the nil check and 500 on an unknown id.
func (h *Handler) lookupWebsite(w http.ResponseWriter, r *http.Request) (int64, *domain.Website, bool, error) {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, nil, false, nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return 0, nil, false, err
	}
	if ws == nil {
		http.NotFound(w, r)
		return 0, nil, false, nil
	}
	return websiteID, ws, true, nil
}
