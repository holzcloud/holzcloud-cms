package admin

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/tmplspec"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// TemplateListData extends LayoutData for the template list page.
type TemplateListData struct {
	web.LayoutData
	Templates []TemplateWithWebsites
}

// TemplateWithWebsites pairs a template with its active website info.
type TemplateWithWebsites struct {
	tmplmgr.Template
	ActiveWebsites []WebsiteActivation
}

// WebsiteActivation tracks whether a template is active for a given website.
type WebsiteActivation struct {
	Website  domain.Website
	IsActive bool
}

// TemplateUploadData extends LayoutData for the template upload form.
type TemplateUploadData struct {
	web.LayoutData
}

// HandleTemplateList renders the template list page.
func (h *Handler) HandleTemplateList(w http.ResponseWriter, r *http.Request) error {
	templates, err := h.tmplStore.List(r.Context())
	if err != nil {
		return err
	}

	websites, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return err
	}

	// One query for the whole activation matrix instead of one per
	// template-website pair.
	activeByWebsite, err := h.tmplStore.ActiveByWebsite(r.Context())
	if err != nil {
		return err
	}

	items := make([]TemplateWithWebsites, 0, len(templates))
	for _, t := range templates {
		tw := TemplateWithWebsites{Template: t}
		for _, ws := range websites {
			tw.ActiveWebsites = append(tw.ActiveWebsites, WebsiteActivation{
				Website:  ws,
				IsActive: activeByWebsite[ws.ID] == t.ID,
			})
		}
		items = append(items, tw)
	}

	data := TemplateListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Vorlagen"),
		Templates:  items,
	}
	data.ActiveNav = "templates"
	return web.RenderAdmin(w, h.templates, r, "template_list", data)
}

// HandleTemplateUpload handles GET (form) and POST (submit) for uploading a template.
func (h *Handler) HandleTemplateUpload(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		return h.handleTemplateUploadPost(w, r)
	}

	data := TemplateUploadData{
		LayoutData: web.NewLayoutData(r, h.sm, "Vorlage hochladen"),
	}
	data.ActiveNav = "templates"
	return web.RenderAdmin(w, h.templates, r, "template_upload", data)
}

func (h *Handler) handleTemplateUploadPost(w http.ResponseWriter, r *http.Request) error {
	// Enforce size cap (T-04-07)
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxTemplateSize)

	if err := r.ParseMultipartForm(h.cfg.MaxTemplateSize); err != nil {
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder Upload fehlerhaft")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte einen Namen für die Vorlage angeben")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	file, header, err := r.FormFile("template_file")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Bitte eine .zip-Datei zum Hochladen auswählen")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		web.SetFlashError(h.sm, r.Context(), "Es werden nur .zip-Dateien angenommen")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	// Read file into memory for zip.NewReader
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die hochgeladene Datei konnte nicht gelesen werden")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	slug := slugify(name)
	// A name with no usable characters would yield an empty slug, making destDir
	// the templates root — ExtractTemplate would then RemoveAll every installed
	// template before renaming its temp dir into place.
	if slug == "" {
		web.SetFlashError(h.sm, r.Context(), "Der Name der Vorlage muss Buchstaben oder Ziffern enthalten")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	// Check if slug already exists
	existing, err := h.tmplStore.GetBySlug(r.Context(), slug)
	if err != nil {
		return err
	}
	if existing != nil {
		web.SetFlashError(h.sm, r.Context(), "Eine Vorlage mit diesem Namen gibt es bereits")
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	destDir := filepath.Join(h.cfg.DataDir, "templates", slug)
	reader := bytes.NewReader(buf.Bytes())

	if err := tmplmgr.ExtractTemplate(reader, int64(buf.Len()), destDir, h.cfg.MaxTemplateSize, h.loader.DefaultFS()); err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Ungültige Vorlage: %s", err))
		http.Redirect(w, r, "/admin/templates/upload", http.StatusSeeOther)
		return nil
	}

	if _, err := h.tmplStore.Create(r.Context(), name, slug); err != nil {
		// Clean up disk on DB failure
		_ = os.RemoveAll(destDir)
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Vorlage hochgeladen")
	redirect := "/admin/templates"
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleTemplateActivate activates a template for a website.
func (h *Handler) HandleTemplateActivate(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	websiteID, err := strconv.ParseInt(r.FormValue("website_id"), 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Ungültige Website")
		http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
		return nil
	}

	if err := h.requireWebsiteAndTemplate(r, websiteID, id); err != nil {
		web.SetFlashError(h.sm, r.Context(), err.Error())
		http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
		return nil
	}

	if err := h.tmplStore.ActivateForWebsite(r.Context(), websiteID, id); err != nil {
		return err
	}

	// Invalidate template cache for this website
	h.loader.InvalidateTemplateCache(websiteID)

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionTemplateActivate,
		EntityType: "template",
		EntityID:   id,
		WebsiteID:  &websiteID,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Vorlage aktiviert")
	redirect := "/admin/templates"
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleTemplateDeactivate deactivates a template for a website.
func (h *Handler) HandleTemplateDeactivate(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	websiteID, err := strconv.ParseInt(r.FormValue("website_id"), 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Ungültige Website")
		http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
		return nil
	}

	if err := h.tmplStore.DeactivateForWebsite(r.Context(), websiteID, id); err != nil {
		return err
	}

	h.loader.InvalidateTemplateCache(websiteID)

	web.SetFlashSuccess(h.sm, r.Context(), "Vorlage deaktiviert")
	redirect := "/admin/templates"
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleTemplateDelete deletes a template that is not active anywhere.
func (h *Handler) HandleTemplateDelete(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	// Block deletion of built-in templates
	t, err := h.tmplStore.GetByID(r.Context(), id)
	if err != nil {
		return err
	}
	if t == nil {
		http.NotFound(w, r)
		return nil
	}
	if t.IsBuiltin {
		web.SetFlashError(h.sm, r.Context(), "Mitgelieferte Vorlagen lassen sich nicht löschen.")
		http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
		return nil
	}

	// Check if active anywhere (T-04-08)
	active, err := h.tmplStore.IsActiveAnywhere(r.Context(), id)
	if err != nil {
		return err
	}
	if active {
		web.SetFlashError(h.sm, r.Context(), "Eine Vorlage, die auf einer Website aktiv ist, lässt sich nicht löschen. Bitte zuerst dort eine andere Vorlage aktivieren.")
		http.Redirect(w, r, "/admin/templates", http.StatusSeeOther)
		return nil
	}

	if err := h.tmplStore.Delete(r.Context(), id); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Vorlage gelöscht")
	redirect := "/admin/templates"
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// requireWebsiteAndTemplate verifies that both IDs, which arrive as form values
// rather than path segments, refer to existing rows.
func (h *Handler) requireWebsiteAndTemplate(r *http.Request, websiteID, templateID int64) error {
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		return errors.New("Website not found")
	}
	t, err := h.tmplStore.GetByID(r.Context(), templateID)
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("Template not found")
	}
	return nil
}

// slugify converts a template name to a URL-safe slug.
//
// Unlike page.Slugify it returns "" for a name with no usable characters rather
// than a placeholder, because the caller must reject that: an empty slug would
// make the extraction target the templates root directory.
func slugify(s string) string {
	s = page.Transliterate(s)
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		} else if c == ' ' || c == '-' || c == '_' {
			if len(result) > 0 && result[len(result)-1] != '-' {
				result = append(result, '-')
			}
		}
	}
	return strings.Trim(string(result), "-")
}

// HandleTemplateSpec serves the template authoring specification.
//
// Plain text rather than a rendered page: what it is for is being copied whole
// into a conversation with an AI agent, and Markdown that has been turned into
// HTML has to be turned back first.
func (h *Handler) HandleTemplateSpec(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Named so a browser's "save as" produces something recognisable.
	w.Header().Set("Content-Disposition", `inline; filename="TEMPLATE-SPEC.md"`)
	_, err := io.WriteString(w, tmplspec.Markdown())
	return err
}
