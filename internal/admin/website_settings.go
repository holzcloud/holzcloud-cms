package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/structured"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// offeredTimeZones is a short list rather than the full IANA database.
//
// The zones people running a German website plausibly need fit on one screen;
// six hundred names in a dropdown is a worse tool than five, and the stored
// value is validated against the embedded tzdata either way.
var offeredTimeZones = []string{
	"Europe/Berlin",
	"Europe/Vienna",
	"Europe/Zurich",
	"Europe/London",
	"Europe/Warsaw",
	"UTC",
}

func localeChoices() []LocaleChoice {
	choices := make([]LocaleChoice, 0, len(tmpl.SupportedLocales))
	for _, l := range tmpl.SupportedLocales {
		choices = append(choices, LocaleChoice{Code: l.Code, Name: l.Name})
	}
	return choices
}

// settingsFromRequest reads the settings half of the website form.
//
// Everything is validated here rather than in the store: an unknown language or
// a mistyped zone falls back to the default instead of being written and then
// silently rendering the wrong thing.
func settingsFromRequest(r *http.Request) domain.Settings {
	set := domain.Settings{
		Locale:            strings.TrimSpace(r.FormValue("locale")),
		TimeZone:          strings.TrimSpace(r.FormValue("timezone")),
		MetaDescription:   strings.TrimSpace(r.FormValue("meta_description")),
		CanonicalRedirect: r.FormValue("canonical_redirect") != "",
		OfflineMode:       r.FormValue("offline_mode"),
		OfflineMessage:    strings.TrimSpace(r.FormValue("offline_message")),
	}
	if !tmpl.KnownLocale(set.Locale) {
		set.Locale = tmpl.DefaultLocale
	}
	if !knownTimeZone(set.TimeZone) {
		set.TimeZone = tmpl.DefaultTimeZone
	}
	set.FaviconMediaID = optionalID(r.FormValue("favicon_media_id"))
	set.LogoMediaID = optionalID(r.FormValue("logo_media_id"))
	set.ContactEmail = strings.TrimSpace(r.FormValue("contact_email"))
	set.NotifyEmail = strings.TrimSpace(r.FormValue("notify_email"))
	set.OrgType = strings.TrimSpace(r.FormValue("org_type"))
	if !structured.KnownOrgType(set.OrgType) {
		// An unknown type produces structured data a search engine discards
		// silently, which is worse than emitting none at all.
		set.OrgType = ""
	}
	set.Street = strings.TrimSpace(r.FormValue("street"))
	set.PostalCode = strings.TrimSpace(r.FormValue("postal_code"))
	set.City = strings.TrimSpace(r.FormValue("city"))
	set.Country = strings.ToUpper(strings.TrimSpace(r.FormValue("country")))
	if set.Country == "" {
		set.Country = "DE"
	}
	set.Phone = strings.TrimSpace(r.FormValue("phone"))
	set.OpeningHours = strings.TrimSpace(r.FormValue("opening_hours"))
	set.BlogBase = archiveSlug(r.FormValue("blog_base"))
	set.PostsPerPage = postsPerPage(r.FormValue("posts_per_page"))
	// The further languages are cleaned in the store — a tag that is not a tag,
	// a repeat and the main language itself all drop out there, so the same
	// rules hold for a bundle import as for this form.
	set.ExtraLocales = r.FormValue("extra_locales")
	return set
}

// archiveSlug normalises the archive address.
//
// It goes through the same Slugify as a page: an operator typing "Neues &
// Aktuelles" must not end up with an archive at an address no browser can
// reach. An empty value is kept empty and switches the archive off.
func archiveSlug(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	slug := page.Slugify(raw)
	// A reserved name would be shadowed by the router and the archive would
	// simply never appear, with nothing to explain why.
	if slug == "" || page.ValidateSlug(slug) != nil {
		return ""
	}
	return slug
}

// postsPerPage bounds the archive page size.
func postsPerPage(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 100 {
		return 10
	}
	return n
}

// knownTimeZone checks the value against the embedded zone database, not
// against the offered list — a hand-edited request should still be allowed a
// real zone, just not a nonsense one.
func knownTimeZone(name string) bool {
	if name == "" {
		return false
	}
	return tmpl.LoadLocation(name).String() == name
}

func optionalID(raw string) *int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

// SiteCheck is one readiness item on the settings screen.
type SiteCheck struct {
	// Label names the thing being checked, in the operator's terms.
	Label string
	// OK is whether it passes. A failing check is not an error — a new site
	// fails most of them — it is a to-do with a link attached.
	OK bool
	// Hint says what to do about it.
	Hint string
	// Link points at the screen that fixes it, so the check is actionable
	// rather than a complaint.
	Link string
}

// siteChecks reports what still stands between this website and being live.
//
// It exists because the failure modes are all silent: a site with no domain, no
// published start page or no imprint answers a plain 404 or a legal warning
// letter, and nothing in the admin says so.
func (h *Handler) siteChecks(ctx context.Context, ws *domain.Website, domains []domain.Domain) []SiteCheck {
	base := fmt.Sprintf("/admin/websites/%d", ws.ID)
	checks := []SiteCheck{}

	primaries := 0
	for _, d := range domains {
		if d.IsPrimary {
			primaries++
		}
	}
	checks = append(checks,
		SiteCheck{
			Label: i18n.N("Mindestens eine Domain"),
			OK:    len(domains) > 0,
			Hint:  i18n.N("Ohne Domain ist die Website nicht erreichbar."),
			Link:  base,
		},
		SiteCheck{
			Label: i18n.N("Eine Hauptdomain festgelegt"),
			OK:    primaries == 1,
			Hint:  i18n.N("Die Hauptdomain bestimmt die kanonische Adresse in der Sitemap und im Quelltext."),
			Link:  base,
		},
	)

	home, err := h.pages.GetHomePage(ctx, ws.ID)
	checks = append(checks, SiteCheck{
		Label: i18n.N("Veröffentlichte Startseite"),
		OK:    err == nil && home != nil,
		Hint:  i18n.N("Ohne veröffentlichte Startseite antwortet die eigene Domain mit „Seite nicht gefunden“."),
		Link:  base + "/pages",
	})

	activeSlug := ""
	if h.tmplStore != nil {
		activeSlug, _ = h.tmplStore.ActiveTemplateSlug(ctx, ws.ID)
	}
	checks = append(checks, SiteCheck{
		Label: i18n.N("Vorlage aktiviert"),
		OK:    activeSlug != "",
		Hint:  i18n.N("Ohne aktivierte Vorlage wird die mitgelieferte Standardvorlage benutzt."),
		Link:  base + "/design",
	})

	checks = append(checks, SiteCheck{
		Label: i18n.N("Impressum im Footer-Menü verlinkt"),
		OK:    h.footerLinksImprint(ctx, ws.ID),
		Hint:  i18n.N("§ 5 DDG verlangt ein von jeder Seite erreichbares Impressum; das Footer-Menü leistet das."),
		Link:  base + "/menus",
	})

	checks = append(checks,
		SiteCheck{
			Label: i18n.N("Beschreibung für Suchmaschinen"),
			OK:    strings.TrimSpace(ws.MetaDescription) != "" || strings.TrimSpace(ws.Description) != "",
			Hint:  i18n.N("Ohne Beschreibung sucht sich Google selbst einen Textausschnitt aus."),
			Link:  base,
		},
		SiteCheck{
			Label: i18n.N("Favicon gesetzt"),
			OK:    ws.FaviconMediaID != nil,
			Hint:  i18n.N("Ohne Favicon fragt jeder Browser /favicon.ico an und bekommt eine 404."),
			Link:  base,
		},
	)

	return checks
}

// footerLinksImprint reports whether the footer menu points at a published page
// that looks like an imprint.
//
// It matches on the slug rather than the title because the slug is what the
// legal requirement is really about — a reachable address — and because a menu
// entry can be titled anything.
func (h *Handler) footerLinksImprint(ctx context.Context, websiteID int64) bool {
	if h.menuStore == nil {
		return false
	}
	tree, err := h.menuStore.GetMenuTree(ctx, websiteID, "footer")
	if err != nil {
		return false
	}
	for _, node := range tree {
		if node.PageSlug == "" {
			continue
		}
		if pg, err := h.pages.GetPublishedPage(ctx, websiteID, node.PageSlug); err == nil && pg != nil {
			if strings.Contains(node.PageSlug, "impressum") {
				return true
			}
		}
	}
	return false
}

// HandleDomainSetPrimary makes one domain the canonical one.
func (h *Handler) HandleDomainSetPrimary(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	domainID, err := strconv.ParseInt(r.PathValue("domainID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.domains.SetPrimaryDomain(r.Context(), websiteID, domainID); err != nil {
		return err
	}
	h.resolver.InvalidateCache()

	domains, err := h.domains.ListDomains(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if r.Header.Get("HX-Request") == "true" {
		return web.RenderPartial(w, h.templates, r, "domain_list", DomainListData{
			WebsiteID: websiteID,
			Domains:   domains,
			CSRFToken: web.CSRFTokenFromRequest(r),
		})
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d", websiteID), http.StatusSeeOther)
	return nil
}
