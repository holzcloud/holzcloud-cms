package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/structured"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// websiteDesignData extends LayoutData for the website design/template page.
type websiteDesignData struct {
	web.LayoutData
	Website        *domain.Website
	Templates      []tmplmgr.Template
	ActiveTemplate *tmplmgr.Template
	// Fonts are the typefaces on offer for the design tokens.
	Fonts []fontChoice
	// TokenDefaults fill the colour inputs when nothing has been chosen, so the
	// picker opens on something sensible rather than black.
	TokenDefaults design.Tokens
	// Contrast of text and of links against the background, for the colours
	// in force. Empty when the template's own colours apply: those are the
	// theme author's to answer for, and the page cannot know them.
	TextContrast, LinkContrast contrastView
}

// contrastView is one contrast ratio as the design screen shows it.
type contrastView struct {
	Ratio string
	// OK is 4.5:1 or more, the WCAG line for running text.
	OK bool
}

func contrastOf(a, b string) contrastView {
	r := design.Contrast(a, b)
	if r == 0 {
		return contrastView{}
	}
	return contrastView{Ratio: strconv.FormatFloat(r, 'f', 1, 64) + ":1", OK: r >= 4.5}
}

// fontChoice is one entry of the typeface dropdown.
type fontChoice struct {
	Value string
	Label string
}

func fontChoices() []fontChoice {
	out := make([]fontChoice, 0, len(design.FontStacks))
	for _, f := range design.FontStacks {
		out = append(out, fontChoice{Value: f.Value, Label: f.Label})
	}
	return out
}

// websiteListData extends LayoutData for the website list page.
type websiteListData struct {
	web.LayoutData
	Websites []websiteWithDomainCount
}

// websiteWithDomainCount pairs a website with its domain count and active template.
type websiteWithDomainCount struct {
	domain.Website
	DomainCount  int
	TemplateName string
}

// websiteFormData extends LayoutData for the website create/edit form.
type websiteFormData struct {
	web.LayoutData
	web.FormState
	Website    *domain.Website
	Domains    []domain.Domain
	DomainList domainListData
	IsEdit     bool

	// Locales are the choices offered for the site language.
	Locales []localeChoice
	// TimeZones are the zones offered. A short curated list, not the full IANA
	// database: a German operator picking between 600 zone names is worse off
	// than one picking between five.
	TimeZones []string
	// Media is the image pool the favicon and logo are chosen from.
	Media []media.Media
	// Checks is the readiness list shown next to the settings.
	Checks []siteCheck
	// OrgTypes are the schema.org types offered for the business block.
	OrgTypes []orgTypeChoice
	// MailConfigured says whether this installation can send at all, so the
	// notification field can admit that filling it in would do nothing.
	MailConfigured bool
}

// orgTypeChoice is one entry of the business-type dropdown.
type orgTypeChoice struct {
	Value string
	Label string
}

// orgTypeChoices adapts the vocabulary list for the template.
func orgTypeChoices() []orgTypeChoice {
	out := make([]orgTypeChoice, 0, len(structured.OrgTypes))
	for _, t := range structured.OrgTypes {
		out = append(out, orgTypeChoice{Value: t.Value, Label: t.Label})
	}
	return out
}

// localeChoice is one entry of the language dropdown.
type localeChoice struct {
	Code string
	Name string
}

// domainListData is the data for the domain_list partial, rendered both inside
// the website form and standalone as an htmx swap response.
type domainListData struct {
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
	// Only their own: a list with websites that answer 403 when clicked is not a
	// list but a trap.
	websites := keepMine(h.rightsOf(r), all)

	var items []websiteWithDomainCount
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
		items = append(items, websiteWithDomainCount{Website: ws, DomainCount: len(domains), TemplateName: tmplName})
	}

	data := websiteListData{
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

	data := websiteFormData{
		LayoutData: web.NewLayoutData(r, h.sm, "New website"),
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
		web.SetFlashError(h.sm, r.Context(), "Please give the website a name")
		http.Redirect(w, r, "/admin/websites/new", http.StatusSeeOther)
		return nil
	}

	description := strings.TrimSpace(r.FormValue("description"))
	// Starter content is on by default and can be switched off — an import or a
	// clone brings its own pages and would otherwise collide on the "home" slug.
	starter := r.FormValue("starter_content") != "off"
	ws, err := h.createWebsite(r.Context(), name, description, starter, h.currentUserID(r))
	if err != nil {
		return err
	}

	// i18n.N and not a bare literal: SetFlashSuccess translates its argument,
	// but the collector reads CALL SITES, and this one hands it a variable. So
	// the sentence was invisible to the gate — it reported neither open nor
	// orphaned about it — and an English operator who switched the starter
	// content off was told "Website angelegt". Exactly the shape CLAUDE.md
	// warns about for fmt.Sprintf, arrived at by a different road.
	message := i18n.N("Website created")
	if starter {
		message = starterContentSummary(r)
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

	data := websiteFormData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Settings – %s", ws.Name)),
		FormState:  web.NewFormState(),
		Website:    ws,
		Domains:    domains,
		DomainList: domainListData{WebsiteID: ws.ID, Domains: domains, CSRFToken: web.CSRFTokenFromRequest(r)},
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
		web.SetFlashError(h.sm, r.Context(), "Please give the website a name")
		redirect := fmt.Sprintf("/admin/websites/%d", id)
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return nil
	}

	if err := h.saveWebsite(r.Context(), id, r.FormValue); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Website saved")
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

	if err := h.deleteWebsite(r.Context(), id); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Website deleted")
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
	web.SetFlashSuccess(h.sm, r.Context(), "Domain added")

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

	if err := h.domains.RemoveDomain(r.Context(), id, domainID); err != nil {
		return err
	}

	h.resolver.InvalidateCache()

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionDomainRemove,
		EntityType: "domain",
		EntityID:   domainID,
		WebsiteID:  &id,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Domain removed")

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
	return web.RenderPartial(w, h.templates, r, "domain_list", domainListData{
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

	data := websiteDesignData{
		LayoutData:     web.NewLayoutData(r, h.sm, web.Titlef(r, "Design – %s", ws.Name)),
		Website:        ws,
		Templates:      templates,
		ActiveTemplate: activeTmpl,
		Fonts:          fontChoices(),
		TokenDefaults:  design.Tokens{Ink: "#1a1a1a", Paper: "#fafafa", Brand: "#1a6dd4"},
	}
	if ws.TokenInk != "" && ws.TokenPaper != "" {
		data.TextContrast = contrastOf(ws.TokenInk, ws.TokenPaper)
		data.LinkContrast = contrastOf(ws.TokenBrand, ws.TokenPaper)
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
		web.SetFlashError(h.sm, r.Context(), "Invalid template")
		http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d/design", id), http.StatusSeeOther)
		return nil
	}

	if err := h.activateTemplate(r.Context(), id, templateID); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Template activated")
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

	// "Reset" is its own button rather than six emptied fields: clearing
	// a colour input is something browsers make surprisingly hard.
	if r.FormValue("reset") != "" {
		if err := h.resetDesign(r.Context(), websiteID); err != nil {
			return err
		}
		web.SetFlashSuccess(h.sm, r.Context(), "Custom colours removed — the template applies again")
		return h.redirect(w, r, redirect)
	}

	if _, err := h.saveDesign(r.Context(), websiteID, r.FormValue); err != nil {
		return err
	}

	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionDesignSave,
		EntityType: "website",
		EntityID:   websiteID,
		WebsiteID:  &websiteID,
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Design saved")
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

// tokensFromForm reads the design form. The save handler and the preview both
// read it, so what the preview shows is exactly what saving would store.
func tokensFromForm(get func(string) string) design.Tokens {
	tokens := design.Sanitize(design.Tokens{
		Ink:     get("token_ink"),
		Paper:   get("token_paper"),
		Brand:   get("token_brand"),
		Font:    get("token_font"),
		Measure: atoiOr(get("token_measure"), 0),
		Radius:  atoiOr(get("token_radius"), -1),
	})
	// An unticked checkbox sends nothing, which is how "use the theme's colours"
	// is expressed without asking the operator to clear three colour pickers.
	if get("use_colours") == "" {
		tokens.Ink, tokens.Paper, tokens.Brand = "", "", ""
	}
	return tokens
}
