package public

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// listedURL is one address the website advertises about itself.
type listedURL struct {
	Loc         string
	LastMod     string
	Title       string
	Description string
	// Locale is the language tag the address belongs to, never empty.
	Locale string
	// Entry is set for page rows only; roots and archives have none.
	Entry *page.SitemapEntry
}

// listing is the one selection of advertised URLs. /sitemap.xml and /llms.txt
// both read it, so they cannot drift apart: a page that is a draft, scheduled,
// noindex, protected or on another website is absent from both or from neither.
func (h *Handler) listing(r *http.Request, website *domain.Website) ([]listedURL, error) {
	entries, err := h.pageStore.ListPublishedForSitemap(r.Context(), website.ID)
	if err != nil {
		return nil, fmt.Errorf("list sitemap entries: %w", err)
	}

	base := h.baseURL(r)
	main := website.Locale
	out := make([]listedURL, 0, len(entries)+1)
	out = append(out, listedURL{
		Loc: base + "/", Title: website.Name, Description: website.MetaDescription, Locale: main,
	})
	// Every language's start page. They are not page rows in their own right,
	// and a crawler that cannot find /fr finds nothing behind it either.
	for _, tag := range website.Locales() {
		out = append(out, listedURL{
			Loc: base + locale.Path(tag, main, "/"), Title: website.Name,
			Description: website.MetaDescription, Locale: tag,
		})
	}
	// The archive is not a page row, so nothing else would ever list it — and an
	// unlisted archive is the one address a crawler most needs to find the
	// entries behind it.
	if website.HasArchive() {
		out = append(out, listedURL{
			Loc: base + "/" + url.PathEscape(website.BlogBase), Title: website.BlogBase, Locale: main,
		})
	}
	// And for the same reason the overview of every content type of its own.
	for _, t := range h.typesOf(r, website.ID) {
		if t.HasArchive() {
			out = append(out, listedURL{
				Loc: base + "/" + url.PathEscape(t.Archive), Title: t.Archive, Locale: main,
			})
		}
	}
	for i := range entries {
		e := &entries[i]
		// The start page already stands above, under the root of its language.
		// Listing it here a second time under /home would mean naming a search
		// engine two addresses for the same text — and since the redirect the
		// address /home answers with a 301 anyway.
		if e.Slug == page.HomeSlug {
			continue
		}
		tag := e.Locale
		if tag == "" {
			tag = main
		}
		out = append(out, listedURL{
			Loc:         base + locale.Path(e.Locale, main, "/"+url.PathEscape(e.Slug)),
			LastMod:     e.UpdatedAt.UTC().Format("2006-01-02"),
			Title:       e.Title,
			Description: e.Description,
			Locale:      tag,
			Entry:       e,
		})
	}
	return out, nil
}
