package admin

import (
	"context"
	"database/sql"
	"errors"
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

func localeChoices() []localeChoice {
	choices := make([]localeChoice, 0, len(tmpl.SupportedLocales))
	for _, l := range tmpl.SupportedLocales {
		choices = append(choices, localeChoice{Code: l.Code, Name: l.Name})
	}
	return choices
}

// settingsFromRequest reads the settings half of the website form.
func settingsFromRequest(r *http.Request) domain.Settings { return settingsFrom(r.FormValue) }

// settingsFrom reads the settings from anything shaped like the form: the
// request on the screen, or the stored values with an assistant's changes laid
// over them (OpUpdateWebsite). One reader, so an AI key meets exactly the rules
// the form does.
//
// Everything is validated here rather than in the store: an unknown language or
// a mistyped zone falls back to the default instead of being written and then
// silently rendering the wrong thing.
func settingsFrom(get func(string) string) domain.Settings {
	set := domain.Settings{
		Locale:            strings.TrimSpace(get("locale")),
		TimeZone:          strings.TrimSpace(get("timezone")),
		MetaDescription:   strings.TrimSpace(get("meta_description")),
		CanonicalRedirect: get("canonical_redirect") != "",
		ConfirmSenders:    get("confirm_senders") != "",
		OfflineMode:       get("offline_mode"),
		OfflineMessage:    strings.TrimSpace(get("offline_message")),
	}
	if !tmpl.KnownLocale(set.Locale) {
		set.Locale = tmpl.DefaultLocale
	}
	if !knownTimeZone(set.TimeZone) {
		set.TimeZone = tmpl.DefaultTimeZone
	}
	set.FaviconMediaID = optionalID(get("favicon_media_id"))
	set.LogoMediaID = optionalID(get("logo_media_id"))
	set.ContactEmail = strings.TrimSpace(get("contact_email"))
	set.NotifyEmail = strings.TrimSpace(get("notify_email"))
	set.OrgType = strings.TrimSpace(get("org_type"))
	if !structured.KnownOrgType(set.OrgType) {
		// An unknown type produces structured data a search engine discards
		// silently, which is worse than emitting none at all.
		set.OrgType = ""
	}
	set.Street = strings.TrimSpace(get("street"))
	set.PostalCode = strings.TrimSpace(get("postal_code"))
	set.City = strings.TrimSpace(get("city"))
	set.Country = strings.ToUpper(strings.TrimSpace(get("country")))
	if set.Country == "" {
		set.Country = "DE"
	}
	set.Phone = strings.TrimSpace(get("phone"))
	set.OpeningHours = strings.TrimSpace(get("opening_hours"))
	set.BlogBase = archiveSlug(get("blog_base"))
	set.PostsPerPage = postsPerPage(get("posts_per_page"))
	// The further languages are cleaned in the store — a tag that is not a tag,
	// a repeat and the main language itself all drop out there, so the same
	// rules hold for a bundle import as for this form.
	set.ExtraLocales = get("extra_locales")
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

// siteCheck is one readiness item on the settings screen.
type siteCheck struct {
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
func (h *Handler) siteChecks(ctx context.Context, ws *domain.Website, domains []domain.Domain) []siteCheck {
	base := fmt.Sprintf("/admin/websites/%d", ws.ID)
	checks := []siteCheck{}

	primaries := 0
	for _, d := range domains {
		if d.IsPrimary {
			primaries++
		}
	}
	checks = append(checks,
		siteCheck{
			Label: i18n.N("At least one domain"),
			OK:    len(domains) > 0,
			Hint:  i18n.N("Without a domain the website cannot be reached."),
			Link:  base,
		},
		siteCheck{
			Label: i18n.N("A primary domain has been set"),
			OK:    primaries == 1,
			Hint:  i18n.N("The primary domain decides the canonical address in the sitemap and in the page source."),
			Link:  base,
		},
	)

	home, err := h.pages.GetHomePage(ctx, ws.ID)
	checks = append(checks, siteCheck{
		Label: i18n.N("A published start page"),
		OK:    err == nil && home != nil,
		Hint:  i18n.N("Without a published start page your own domain answers “page not found”."),
		Link:  base + "/pages",
	})

	activeSlug := ""
	if h.tmplStore != nil {
		activeSlug, _ = h.tmplStore.ActiveTemplateSlug(ctx, ws.ID)
	}
	checks = append(checks, siteCheck{
		Label: i18n.N("Template activated"),
		OK:    activeSlug != "",
		Hint:  i18n.N("Without an activated template the built-in default template is used."),
		Link:  base + "/design",
	})

	checks = append(checks, siteCheck{
		Label: i18n.N("Imprint linked in the footer menu"),
		OK:    h.footerLinksImprint(ctx, ws.ID),
		Hint:  i18n.N("German law (§ 5 DDG) requires an imprint reachable from every page; the footer menu does that."),
		Link:  base + "/menus",
	})

	checks = append(checks,
		siteCheck{
			Label: i18n.N("Description for search engines"),
			OK:    strings.TrimSpace(ws.MetaDescription) != "" || strings.TrimSpace(ws.Description) != "",
			Hint:  i18n.N("Without a description Google picks a passage of text itself."),
			Link:  base,
		},
		siteCheck{
			Label: i18n.N("Favicon set"),
			OK:    ws.FaviconMediaID != nil,
			Hint:  i18n.N("Without a favicon every browser asks for /favicon.ico and gets a 404."),
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
		// A domain that is not this website's is a route that names something
		// that does not exist here, which is a 404 and not a fault of the
		// server. The store's transaction has already rolled back, so this
		// website's own primary flag is untouched.
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return nil
		}
		return err
	}
	h.resolver.InvalidateCache()

	domains, err := h.domains.ListDomains(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if r.Header.Get("HX-Request") == "true" {
		return web.RenderPartial(w, h.templates, r, "domain_list", domainListData{
			WebsiteID: websiteID,
			Domains:   domains,
			CSRFToken: web.CSRFTokenFromRequest(r),
		})
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/websites/%d", websiteID), http.StatusSeeOther)
	return nil
}
