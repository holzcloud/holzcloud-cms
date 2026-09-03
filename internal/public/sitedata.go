package public

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/design"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// siteData builds the site-level half of PageData.
//
// It is the only place the mapping from a database row to what a theme can see
// exists, so the home page, an inner page, the 404 and the maintenance page
// cannot end up describing the same site differently.
// siteTerms is the label list a layout can render as a cloud.
func (h *Handler) siteTerms(r *http.Request, websiteID int64) []tmpl.TermLink {
	if h.termStore == nil {
		return nil
	}
	terms, err := h.termStore.ListWithCounts(r.Context(), websiteID)
	if err != nil {
		return nil
	}
	return termLinksAt(localePrefixOf(r), terms)
}

func (h *Handler) siteData(r *http.Request, website *domain.Website) tmpl.SiteData {
	site := tmpl.SiteData{
		Name:            website.Name,
		Description:     website.Description,
		MetaDescription: website.MetaDescription,
		Locale:          website.Locale,
		TimeZone:        website.TimeZone,
		URL:             h.canonicalBase(r, website),
	}
	// <html lang> is the language being served, not the website's main one: a
	// French page announced as German is read aloud in German.
	if tag := LocaleFrom(r.Context()); tag != "" {
		site.Locale = tag
	}
	site.Sprachen = h.languageHomes(r, website)
	site.FeedURL = localePrefixOf(r) + "/feed.xml"

	if site.Locale == "" {
		site.Locale = tmpl.DefaultLocale
	}
	if site.TimeZone == "" {
		site.TimeZone = tmpl.DefaultTimeZone
	}
	site.FaviconURL = h.mediaURL(r.Context(), website.ID, website.FaviconMediaID)
	site.LogoURL = h.mediaURL(r.Context(), website.ID, website.LogoMediaID)
	site.Terms = h.siteTerms(r, website.ID)
	site.HasSearch = h.hasSearch(website.ID)
	site.Design = design.Tokens{
		Ink: website.TokenInk, Paper: website.TokenPaper, Brand: website.TokenBrand,
		Font: website.TokenFont, Measure: website.TokenMeasure, Radius: website.TokenRadius,
	}.CSS()
	return site
}

// mediaURL turns a media row reference into the same-origin path a browser can
// request, or "" when nothing is set or the row is gone.
func (h *Handler) mediaURL(ctx context.Context, websiteID int64, mediaID *int64) string {
	if mediaID == nil || h.mediaStore == nil {
		return ""
	}
	m, err := h.mediaStore.GetByID(ctx, *mediaID)
	if err != nil {
		slog.Error("look up media for site data", "err", err, "media_id", *mediaID)
		return ""
	}
	if m == nil || m.WebsiteID != websiteID {
		return ""
	}
	return fmt.Sprintf("/media/%d/%s", websiteID, m.Filename)
}

// canonicalBase is the scheme and host the site publishes about itself.
//
// It deliberately ignores X-Forwarded-Proto and X-Forwarded-Host: the address a
// site claims as its own must not be settable by whoever is making the request.
func (h *Handler) canonicalBase(r *http.Request, website *domain.Website) string {
	if h.resolver != nil {
		return h.resolver.CanonicalBase(r.Context(), website, r.Host)
	}
	scheme := "http"
	if h.secure {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
