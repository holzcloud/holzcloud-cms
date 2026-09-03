package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/structured"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// WebsiteDesignData extends LayoutData for the website design/template page.
type WebsiteDesignData struct {
	web.LayoutData
	Website        *domain.Website
	Templates      []tmplmgr.Template
	ActiveTemplate *tmplmgr.Template
	// Fonts are the typefaces on offer for the design tokens.
	Fonts []FontChoice
	// TokenDefaults fill the colour inputs when nothing has been chosen, so the
	// picker opens on something sensible rather than black.
	TokenDefaults design.Tokens
}

// FontChoice is one entry of the typeface dropdown.
type FontChoice struct {
	Value string
	Label string
}

func fontChoices() []FontChoice {
	out := make([]FontChoice, 0, len(design.FontStacks))
	for _, f := range design.FontStacks {
		out = append(out, FontChoice{Value: f.Value, Label: f.Label})
	}
	return out
}

// WebsiteListData extends LayoutData for the website list page.
type WebsiteListData struct {
	web.LayoutData
	Websites []WebsiteWithDomainCount
}

// WebsiteWithDomainCount pairs a website with its domain count and active template.
type WebsiteWithDomainCount struct {
	domain.Website
	DomainCount  int
	TemplateName string
}

// WebsiteFormData extends LayoutData for the website create/edit form.
type WebsiteFormData struct {
	web.LayoutData
	web.FormState
	Website    *domain.Website
	Domains    []domain.Domain
	DomainList DomainListData
	IsEdit     bool

	// Locales are the choices offered for the site language.
	Locales []LocaleChoice
	// TimeZones are the zones offered. A short curated list, not the full IANA
	// database: a German operator picking between 600 zone names is worse off
	// than one picking between five.
	TimeZones []string
	// Media is the image pool the favicon and logo are chosen from.
	Media []media.Media
	// Checks is the readiness list shown next to the settings.
	Checks []SiteCheck
	// OrgTypes are the schema.org types offered for the business block.
	OrgTypes []OrgTypeChoice
	// MailConfigured says whether this installation can send at all, so the
	// notification field can admit that filling it in would do nothing.
	MailConfigured bool
}

// OrgTypeChoice is one entry of the business-type dropdown.
type OrgTypeChoice struct {
	Value string
	Label string
}

// orgTypeChoices adapts the vocabulary list for the template.
func orgTypeChoices() []OrgTypeChoice {
	out := make([]OrgTypeChoice, 0, len(structured.OrgTypes))
	for _, t := range structured.OrgTypes {
		out = append(out, OrgTypeChoice{Value: t.Value, Label: t.Label})
	}
	return out
}

// LocaleChoice is one entry of the language dropdown.
type LocaleChoice struct {
	Code string
	Name string
}

// DomainListData is the data for the domain_list partial, rendered both inside
// the website form and standalone as an htmx swap response.
type DomainListData struct {
	WebsiteID int64
	Domains   []domain.Domain
	CSRFToken string
}

// HandleWebsiteList renders the website list page.
func (h *Handler) HandleWebsiteList(w http.ResponseWriter, r *http.Request) error {
	all, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return err
	}
	// Nur die eigenen: eine Liste mit Websites, die beim Anklicken 403 sagen,
	// ist keine Liste, sondern eine Falle.
	websites := keepMine(h.rightsOf(r), all)

	var items []WebsiteWithDomainCount
	for _, ws := range websites {
		domains, err := h.domains.ListDomains(r.Context(), ws.ID)
		if err != nil {
			return err
		}
		tmplName := ""
		slug, err := h.tmplStore.ActiveTemplateSlug(r.Context(), ws.ID)
		if err == nil && slug != "" {
			t, err := h.tmplStore.GetBySlug(r.Context(), slug)
			if err == nil && t != nil {
				tmplName = t.Name
			}
		}
		items = append(items, WebsiteWithDomainCount{Website: ws, DomainCount: len(domains), TemplateName: tmplName})
	}

	data := WebsiteListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Websites"),
		Websites:   items,
	}
	data.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "website_list", data)
}

// HandleWebsiteCreate handles GET (form) and POST (submit) for creating a website.
func (h *Handler) HandleWebsiteCreate(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		return h.handleWebsiteCreatePost(w, r)
	}

	data := WebsiteFormData{
		LayoutData: web.NewLayoutData(r, h.sm, "Neue Website"),
	}
	data.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "website_form", data)
}

func (h *Handler) handleWebsiteCreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte einen Namen für die Website angeben")
		http.Redirect(w, r, "/admin/websites/new", http.StatusSeeOther)
		return nil
	}

	description := strings.TrimSpace(r.FormValue("description"))
	ws, err := h.domains.CreateWebsite(r.Context(), name, description)
	if err != nil {
		return err
	}

	// Auto-activate default template for new website
	defaultTmpl, err := h.tmplStore.GetBySlug(r.Context(), "default")
	if err == nil && defaultTmpl != nil {
		_ = h.tmplStore.ActivateForWebsite(r.Context(), ws.ID, defaultTmpl.ID)
	}

	// Starter content is on by default and can be switched off — an import or a
	// clone brings its own pages and would otherwise collide on the "home" slug.
	message := "Website angelegt"
	if r.FormValue("starter_content") != "off" {
		h.createStarterContent(r.Context(), ws.ID, h.currentUserID(r))
		message = starterContentSummary()
	}

	web.SetFlashSuccess(h.sm, r.Context(), message)
	redirect := fmt.Sprintf("/admin/websites/%d", ws.ID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleWebsiteEdit handles GET (form) and POST (submit) for editing a website.
func (h *Handler) HandleWebsiteEdit(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if r.Method == http.MethodPost {
		return h.handleWebsiteEditPost(w, r, id)
	}

	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	domains, err := h.domains.ListDomains(r.Context(), id)
	if err != nil {
		return err
	}

	data := WebsiteFormData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Einstellungen – %s", ws.Name)),
		FormState:  web.NewFormState(),
		Website:    ws,
		Domains:    domains,
		DomainList: DomainListData{WebsiteID: ws.ID, Domains: domains, CSRFToken: web.CSRFTokenFromRequest(r)},
		IsEdit:     true,
		Locales:    localeChoices(),
		TimeZones:  offeredTimeZones,
		OrgTypes:   orgTypeChoices(),
		Checks:     h.siteChecks(r.Context(), ws, domains),

		MailConfigured: h.mail.Enabled(),
	}
	if h.mediaStore != nil {
		// Only images make sense as a favicon or logo, and the list is short
		// enough on a small site that a picker would be more machinery than help.
		all, _, err := h.mediaStore.List(r.Context(), ws.ID, media.Filter{MimePrefix: "image/"}, 1, 200)
		if err != nil {
			return err
		}
		data.Media = all
	}
	data.ActiveNav = "website-settings"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "website_form", data)
}

func (h *Handler) handleWebsiteEditPost(w http.ResponseWriter, r *http.Request, id int64) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte einen Namen für die Website angeben")
		redirect := fmt.Sprintf("/admin/websites/%d", id)
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return nil
	}

	description := strings.TrimSpace(r.FormValue("description"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "1"

	if err := h.domains.UpdateWebsite(r.Context(), id, name, description, active); err != nil {
		return err
	}
	if err := h.domains.UpdateSettings(r.Context(), id, settingsFromRequest(r)); err != nil {
		return err
	}
	// The resolver caches the whole Website struct per host, including its
	// active flag and name. Without this, deactivating or renaming a site has no
	// effect on the public site until the process restarts.
	h.resolver.InvalidateCache()
	// The date helpers are baked into the parsed template set, so a language or
	// time-zone change has to drop it too.
	h.loader.InvalidateTemplateCache(id)

	web.SetFlashSuccess(h.sm, r.Context(), "Website gespeichert")
	redirect := fmt.Sprintf("/admin/websites/%d", id)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleWebsiteDelete deletes a website and redirects to the list.
func (h *Handler) HandleWebsiteDelete(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.domains.DeleteWebsite(r.Context(), id); err != nil {
		return err
	}
	h.resolver.InvalidateCache()

	// The media rows go with the website through ON DELETE CASCADE, but the
	// files behind them do not. Without this the uploads of every deleted site
	// stay on the SD card forever, unreachable and uncountable.
	mediaDir := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(id, 10))
	if err := os.RemoveAll(mediaDir); err != nil {
		// The website is already gone; failing the request now would suggest it
		// was not deleted. Log it so the leftover can be cleaned up by hand.
		slog.Error("remove media directory of deleted website", "err", err, "dir", mediaDir)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Website gelöscht")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/websites")
		return nil
	}
	http.Redirect(w, r, "/admin/websites", http.StatusSeeOther)
	return nil
}

// HandleDomainAdd adds a domain to a website.
func (h *Handler) HandleDomainAdd(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	domainName := strings.TrimSpace(r.FormValue("domain"))
	if domainName == "" {
		return h.domainAddFailed(w, r, id, "Domain name is required")
	}

	isPrimary := r.FormValue("is_primary") == "on" || r.FormValue("is_primary") == "1"
	added, err := h.domains.AddDomain(r.Context(), id, domainName, isPrimary)
	if err != nil {
		msg := "Could not add domain: " + err.Error()
		if isUniqueViolation(err) {
			msg = "That domain is already assigned to a website."
		}
		return h.domainAddFailed(w, r, id, msg)
	}

	h.resolver.InvalidateCache()

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionDomainAdd,
		EntityType: "domain",
		EntityID:   added.ID,
		WebsiteID:  &id,
		Metadata:   map[string]any{"domain": domainName, "haupt": isPrimary},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Domain hinzugefügt")

	// For htmx: return domain list partial
	if r.Header.Get("HX-Request") == "true" {
		return h.renderDomainListPartial(w, r, id)
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d", id), http.StatusSeeOther)
	return nil
}

// domainAddFailed reports a failed domain add.
//
// The htmx form swaps #domain-list, so plain 303 would make htmx follow the
// redirect and drop a whole page into that element while the flash goes unseen.
// HX-Redirect makes the browser navigate for real, which is what actually shows
// the message.
func (h *Handler) domainAddFailed(w http.ResponseWriter, r *http.Request, websiteID int64, msg string) error {
	web.SetFlashError(h.sm, r.Context(), msg)
	redirect := fmt.Sprintf("/admin/websites/%d", websiteID)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

// HandleDomainRemove removes a domain from a website.
func (h *Handler) HandleDomainRemove(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	domainID, err := strconv.ParseInt(r.PathValue("domainID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.domains.RemoveDomain(r.Context(), domainID); err != nil {
		return err
	}

	h.resolver.InvalidateCache()

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionDomainRemove,
		EntityType: "domain",
		EntityID:   domainID,
		WebsiteID:  &id,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Domain entfernt")

	// For htmx: return domain list partial
	if r.Header.Get("HX-Request") == "true" {
		return h.renderDomainListPartial(w, r, id)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d", id), http.StatusSeeOther)
	return nil
}

// renderDomainListPartial renders just the domain list for htmx swaps.
func (h *Handler) renderDomainListPartial(w http.ResponseWriter, r *http.Request, websiteID int64) error {
	domains, err := h.domains.ListDomains(r.Context(), websiteID)
	if err != nil {
		return err
	}
	return web.RenderPartial(w, h.templates, r, "domain_list", DomainListData{
		WebsiteID: websiteID,
		Domains:   domains,
		CSRFToken: web.CSRFTokenFromRequest(r),
	})
}

// HandleWebsiteDesign renders the website design/template selection page.
func (h *Handler) HandleWebsiteDesign(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	templates, err := h.tmplStore.List(r.Context())
	if err != nil {
		return err
	}

	// Find active template for this website
	var activeTmpl *tmplmgr.Template
	slug, err := h.tmplStore.ActiveTemplateSlug(r.Context(), id)
	if err == nil && slug != "" {
		for i := range templates {
			if templates[i].Slug == slug {
				activeTmpl = &templates[i]
				break
			}
		}
	}

	data := WebsiteDesignData{
		LayoutData:     web.NewLayoutData(r, h.sm, web.Titlef(r, "Design – %s", ws.Name)),
		Website:        ws,
		Templates:      templates,
		ActiveTemplate: activeTmpl,
		Fonts:          fontChoices(),
		TokenDefaults:  design.Tokens{Ink: "#1a1a1a", Paper: "#fafafa", Brand: "#1a6dd4"},
	}
	data.ActiveNav = "website-design"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "website_design", data)
}

// HandleWebsiteDesignActivate activates a template for the current website from the design page.
func (h *Handler) HandleWebsiteDesignActivate(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	templateID, err := strconv.ParseInt(r.FormValue("template_id"), 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Ungültige Vorlage")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/design", id), http.StatusSeeOther)
		return nil
	}

	if err := h.tmplStore.ActivateForWebsite(r.Context(), id, templateID); err != nil {
		return err
	}

	h.loader.InvalidateTemplateCache(id)

	web.SetFlashSuccess(h.sm, r.Context(), "Vorlage aktiviert")
	redirect := fmt.Sprintf("/admin/websites/%d/design", id)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleWebsiteTokens stores the per-website design overrides.
//
// A separate form from the settings screen: the colours and the address of the
// business have nothing to do with each other, and one form saving both would
// make every colour change rewrite the opening hours.
func (h *Handler) HandleWebsiteTokens(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/design", websiteID)

	// "Zurücksetzen" is its own button rather than six emptied fields: clearing
	// a colour input is something browsers make surprisingly hard.
	if r.FormValue("reset") != "" {
		if err := h.domains.UpdateDesignTokens(r.Context(), websiteID, domain.DesignTokens{Radius: -1}); err != nil {
			return err
		}
		h.invalidateWebsiteCaches(websiteID)
		web.SetFlashSuccess(h.sm, r.Context(), "Eigene Farben entfernt – es gilt wieder die Vorlage")
		return h.redirect(w, r, redirect)
	}

	// Sanitize drops anything the CSS could not use, so a mistyped value falls
	// back to the theme instead of losing the settings that were right.
	tokens := design.Sanitize(design.Tokens{
		Ink:     r.FormValue("token_ink"),
		Paper:   r.FormValue("token_paper"),
		Brand:   r.FormValue("token_brand"),
		Font:    r.FormValue("token_font"),
		Measure: atoiOr(r.FormValue("token_measure"), 0),
		Radius:  atoiOr(r.FormValue("token_radius"), -1),
	})
	// An unticked checkbox sends nothing, which is how "use the theme's colours"
	// is expressed without asking the operator to clear three colour pickers.
	if r.FormValue("use_colours") == "" {
		tokens.Ink, tokens.Paper, tokens.Brand = "", "", ""
	}

	if err := h.domains.UpdateDesignTokens(r.Context(), websiteID, domain.DesignTokens{
		Ink: tokens.Ink, Paper: tokens.Paper, Brand: tokens.Brand,
		Font: tokens.Font, Measure: tokens.Measure, Radius: tokens.Radius,
	}); err != nil {
		return err
	}
	h.invalidateWebsiteCaches(websiteID)

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionDesignSave,
		EntityType: "website",
		EntityID:   websiteID,
		WebsiteID:  &websiteID,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Design gespeichert")
	return h.redirect(w, r, redirect)
}

// invalidateWebsiteCaches drops everything that holds a copy of a website.
//
// Two caches, and forgetting the second is a defect with no error message: the
// resolver keeps the whole Website struct per host, so a saved colour would sit
// in the database while the public site kept serving the old one until the
// process restarted. The template cache alone is not enough.
func (h *Handler) invalidateWebsiteCaches(websiteID int64) {
	h.loader.InvalidateTemplateCache(websiteID)
	h.resolver.InvalidateCache()
}

// atoiOr parses an integer field, falling back when it is empty or nonsense.
func atoiOr(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}
