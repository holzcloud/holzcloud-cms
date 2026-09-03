package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SnippetListData is the snippet overview.
type SnippetListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Snippets  []SnippetRow
	// Values is the create/edit form, re-rendered with what was submitted when
	// a save is rejected.
	Values SnippetValues
	IsEdit bool
}

// SnippetRow pairs a snippet with how often it is used.
type SnippetRow struct {
	snippet.Snippet
	// UsedOn is the number of live pages carrying the marker, which is what
	// makes it safe to know whether deleting one matters.
	UsedOn int
}

// SnippetValues is exactly what the form submitted.
type SnippetValues struct {
	ID       int64
	Key      string
	Name     string
	Markdown string
}

func snippetValuesFromRequest(r *http.Request) SnippetValues {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	return SnippetValues{
		ID:       id,
		Key:      strings.TrimSpace(strings.ToLower(r.FormValue("key"))),
		Name:     strings.TrimSpace(r.FormValue("name")),
		Markdown: r.FormValue("content_markdown"),
	}
}

// validKey is the same shape as a slug: a marker has to be typeable inside page
// text without ambiguity.
func validKey(key string) bool {
	if key == "" || len(key) > 60 {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && i > 0 && i < len(key)-1:
		default:
			return false
		}
	}
	return true
}

func (v SnippetValues) validate(errs web.FormErrors) {
	if v.Name == "" {
		errs.Add("name", "Bitte einen Namen angeben.")
	}
	if !validKey(v.Key) {
		errs.Add("key", "Nur Kleinbuchstaben, Ziffern, Bindestrich und Unterstrich.")
	}
}

// HandleSnippetList renders the snippets of a website together with the form.
func (h *Handler) HandleSnippetList(w http.ResponseWriter, r *http.Request) error {
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

	if r.Method == http.MethodPost {
		return h.handleSnippetSave(w, r, websiteID, ws.Name)
	}

	data, err := h.snippetListData(r, websiteID, ws.Name, SnippetValues{})
	if err != nil {
		return err
	}

	// Opening one for editing prefills the same form rather than showing a
	// second screen.
	if raw := r.URL.Query().Get("edit"); raw != "" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		sn, err := h.snippets.Get(r.Context(), id)
		if err != nil {
			return err
		}
		if sn != nil && sn.WebsiteID == websiteID {
			data.Values = SnippetValues{ID: sn.ID, Key: sn.Key, Name: sn.Name, Markdown: sn.ContentMarkdown}
			data.IsEdit = true
		}
	}

	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "snippet_list", data)
}

func (h *Handler) snippetListData(r *http.Request, websiteID int64, websiteName string, values SnippetValues) (SnippetListData, error) {
	data := SnippetListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Textbausteine – %s", websiteName)),
		FormState:  web.NewFormState(),
		WebsiteID:  websiteID,
		Values:     values,
	}
	data.ActiveNav = "snippets"

	list, err := h.snippets.List(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	for _, sn := range list {
		used, err := h.snippets.CountUsage(r.Context(), websiteID, sn.Key)
		if err != nil {
			return data, err
		}
		data.Snippets = append(data.Snippets, SnippetRow{Snippet: sn, UsedOn: used})
	}
	return data, nil
}

func (h *Handler) handleSnippetSave(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := snippetValuesFromRequest(r)
	data, err := h.snippetListData(r, websiteID, websiteName, values)
	if err != nil {
		return err
	}
	data.IsEdit = values.ID != 0
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws

	values.validate(data.Errors)
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}

	// The same pipeline page content goes through, so the stored HTML carries
	// the same guarantee and can be cast in a template.
	html, err := page.RenderMarkdown(values.Markdown)
	if err != nil {
		return err
	}

	if values.ID == 0 {
		_, err = h.snippets.Create(r.Context(), websiteID, values.Key, values.Name, values.Markdown, html)
	} else {
		existing, getErr := h.snippets.Get(r.Context(), values.ID)
		if getErr != nil {
			return getErr
		}
		if existing == nil || existing.WebsiteID != websiteID {
			http.NotFound(w, r)
			return nil
		}
		err = h.snippets.Update(r.Context(), values.ID, values.Key, values.Name, values.Markdown, html)
	}
	if errors.Is(err, snippet.ErrKeyTaken) {
		data.Errors.Add("key", "Diese Kennung wird bereits von einem anderen Baustein benutzt.")
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}
	if err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Textbaustein gespeichert")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}

// HandleSnippetDelete removes a snippet.
func (h *Handler) HandleSnippetDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	id, err := strconv.ParseInt(r.PathValue("snippetID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	sn, err := h.snippets.Get(r.Context(), id)
	if err != nil {
		return err
	}
	if sn == nil || sn.WebsiteID != websiteID {
		http.NotFound(w, r)
		return nil
	}

	if err := h.snippets.Delete(r.Context(), id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Textbaustein gelöscht")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}
