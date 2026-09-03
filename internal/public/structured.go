package public

import (
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
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
