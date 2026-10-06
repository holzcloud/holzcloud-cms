package public

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/structured"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// withStructuredData attaches the schema.org graph for one page.
func (h *Handler) withStructuredData(r *http.Request, website *domain.Website,
	site tmpl.SiteData, meta tmpl.MetaData, pg *page.Page) tmpl.MetaData {

	business := h.businessData(website, site)

	sd := structured.Page{
		Title:       pg.Title,
		URL:         meta.CanonicalURL,
		Description: meta.Description,
		ImageURL:    meta.OGImage,
		IsPost:      pg.IsPost(),
		SiteName:    site.Name,
	}
	if pg.PublishedAt != nil {
		sd.PublishedAt = pg.PublishedAt
	}
	updated := pg.UpdatedAt
	sd.UpdatedAt = &updated

	meta.StructuredData = structured.Build(business, sd, h.crumbs(website, site, pg))
	return meta
}

// businessData maps the website's settings onto the organisation node.
func (h *Handler) businessData(website *domain.Website, site tmpl.SiteData) structured.Business {
	return structured.Business{
		Type:         website.OrgType,
		Name:         website.Name,
		URL:          site.URL,
		LogoURL:      site.LogoURL,
		Street:       website.Street,
		PostalCode:   website.PostalCode,
		City:         website.City,
		Country:      website.Country,
		Phone:        website.Phone,
		Email:        website.ContactEmail,
		OpeningHours: website.OpeningHours,
	}
}

// crumbs is the trail from the home page to this one.
//
// Two steps for a page, three for an archive entry, which is as deep as the
// content model goes — there is no page hierarchy to walk.
func (h *Handler) crumbs(website *domain.Website, site tmpl.SiteData, pg *page.Page) []structured.Crumb {
	trail := []structured.Crumb{{Name: site.Name, URL: site.URL + "/"}}
	if pg.IsPost() && website.HasArchive() {
		trail = append(trail, structured.Crumb{
			Name: "Aktuelles", URL: site.URL + website.ArchiveURL(),
		})
	}
	return append(trail, structured.Crumb{Name: pg.Title, URL: site.URL + "/" + pg.Slug})
}

// homeStructuredData is the graph for the front page, which is the one place a
// search engine looks for the organisation behind a site.
func (h *Handler) homeStructuredData(website *domain.Website, site tmpl.SiteData,
	meta tmpl.MetaData, pg *page.Page) tmpl.MetaData {

	sd := structured.Page{
		Title:       pg.Title,
		URL:         site.URL + "/",
		Description: meta.Description,
		ImageURL:    meta.OGImage,
		SiteName:    site.Name,
	}
	updated := pg.UpdatedAt
	sd.UpdatedAt = &updated

	meta.StructuredData = structured.Build(h.businessData(website, site), sd, nil)
	return meta
}

// productStructuredData is the graph for a product page: the organisation, the
// page, a Product with its Offer, and the trail Home > Shop > Product.
//
// The offer states the price the visitor is shown — net for a trade audience,
// gross otherwise — as a plain decimal, with the ISO code of the website's
// currency (the shop's own Currency.Code is the symbol used for display).
func (h *Handler) productStructuredData(website *domain.Website, site tmpl.SiteData,
	meta tmpl.MetaData, set shop.Settings, p *shop.Product, price shop.Price,
	productURL string, images []string) template.JS {

	url := site.URL + productURL
	updated := p.UpdatedAt
	sd := structured.Page{
		Title:       p.Title,
		URL:         url,
		Description: meta.Description,
		SiteName:    site.Name,
		UpdatedAt:   &updated,
	}
	prod := structured.Product{
		Name:        p.Title,
		URL:         url,
		Description: meta.Description,
		SKU:         p.SKU,
		ImageURLs:   images,
		InStock:     p.Orderable(),
		Currency:    strings.ToUpper(strings.TrimSpace(website.Currency)),
	}
	amount := price.Gross
	if price.Audience == shop.Business && !set.VATExempt {
		amount = price.Net
	}
	if prod.Currency != "" {
		prod.Price = decimalAmount(amount)
	}
	crumbs := []structured.Crumb{
		{Name: site.Name, URL: site.URL + "/"},
		{Name: "Shop", URL: site.URL + website.ShopURL()},
		{Name: p.Title, URL: url},
	}
	return structured.BuildProduct(h.businessData(website, site), sd, prod, crumbs)
}

// decimalAmount writes minor units as "12.50", the form schema.org expects: no
// thousands separator, a point, two places.
func decimalAmount(a money.Amount) string {
	neg := a < 0
	if neg {
		a = -a
	}
	s := strconv.FormatInt(int64(a)/100, 10) + "." + strconv.FormatInt(int64(a)%100+100, 10)[1:]
	if neg {
		s = "-" + s
	}
	return s
}
