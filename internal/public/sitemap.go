package public

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// urlset and sitemapURL model the sitemaps.org 0.9 schema.
type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

// HandleSitemap serves /sitemap.xml for the resolved website, listing the
// homepage plus every published page. Draft pages are excluded by the query, so
// an unpublished page is never disclosed here either.
func (h *Handler) HandleSitemap(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	items, err := h.listing(r, website)
	if err != nil {
		return err
	}

	doc := urlset{Xmlns: sitemapNS, URLs: make([]sitemapURL, 0, len(items))}
	for _, it := range items {
		doc.URLs = append(doc.URLs, sitemapURL{Loc: it.Loc, LastMod: it.LastMod})
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sitemap: %w", err)
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(xml.Header))
	w.Write(body)
	w.Write([]byte("\n"))
	return nil
}

// HandleRobots serves /robots.txt for the resolved website.
//
// An inactive site never reaches this handler — the resolver 404s first — so a
// response here always means the site is live and should be indexed.
func (h *Handler) HandleRobots(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	var b strings.Builder
	b.WriteString("User-agent: *\n")
	// The admin UI and the preview render drafts; keep both out of any index.
	b.WriteString("Disallow: /admin/\n")
	b.WriteString("Allow: /\n\n")
	b.WriteString("Sitemap: " + h.baseURL(r) + "/sitemap.xml\n")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(b.String()))
	return nil
}

// baseURL builds the absolute origin the site publishes about itself.
//
// It is the primary domain when one is set, not the host of the request: every
// alias used to publish a sitemap full of its own URLs, so the same page was
// offered to search engines under several addresses at once.
//
// The scheme comes from configuration rather than a forwarded header, for the
// same reason: a client-supplied X-Forwarded-Proto must not change what gets
// indexed.
func (h *Handler) baseURL(r *http.Request) string {
	if website := domain.WebsiteFromContext(r.Context()); website != nil {
		return h.canonicalBase(r, website)
	}
	scheme := "http"
	if h.secure {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
