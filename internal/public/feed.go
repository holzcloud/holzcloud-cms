package public

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
)

// atomFeed and atomEntry model Atom 1.0 (RFC 4287), through encoding/xml with
// struct tags — the same mechanism sitemap.xml already uses.
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Author  *atomAuthor `xml:"author,omitempty"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published,omitempty"`
	Links     []atomLink  `xml:"link"`
	Summary   string      `xml:"summary,omitempty"`
	Content   atomContent `xml:"content"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

// feedLimit is how many entries a feed carries.
const feedLimit = 20

// HandleFeed serves /feed.xml for the resolved website.
//
// A feed is outgoing content, not a subresource, so it does not conflict with
// the rule that nothing may be loaded from a third party at runtime.
func (h *Handler) HandleFeed(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	// One feed per language, reached at /fr/feed.xml. A reader who subscribed
	// to the French feed did not ask for German articles in among them.
	loc := LocaleFrom(r.Context())
	pages, err := h.pageStore.ListRecentPublishedIn(r.Context(), website.ID, loc, feedLimit)
	if err != nil {
		return fmt.Errorf("list feed entries: %w", err)
	}

	base := h.baseURL(r)
	prefix := base + locale.Prefix(loc, website.Locale)
	snippets := h.loadSnippets(r, website.ID)

	feed := atomFeed{
		Title: website.Name,
		ID:    prefix + "/",
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: prefix + "/feed.xml"},
			{Rel: "alternate", Type: "text/html", Href: prefix + "/"},
		},
	}
	if website.Name != "" {
		feed.Author = &atomAuthor{Name: website.Name}
	}

	newest := website.UpdatedAt
	for _, p := range pages {
		url := prefix + "/" + p.Slug
		entry := atomEntry{
			Title:   p.Title,
			ID:      url,
			Updated: p.UpdatedAt.UTC().Format(time.RFC3339),
			Links:   []atomLink{{Rel: "alternate", Type: "text/html", Href: url}},
			Summary: p.Excerpt,
			// content_html is already sanitised; encoding/xml escapes it into
			// the element, which is what type="html" means.
			Content: atomContent{Type: "html", Body: expandForFeed(p.ContentHTML, snippets)},
		}
		if p.PublishedAt != nil {
			entry.Published = p.PublishedAt.UTC().Format(time.RFC3339)
		}
		feed.Entries = append(feed.Entries, entry)
		if p.UpdatedAt.After(newest) {
			newest = p.UpdatedAt
		}
	}
	if snippets.LatestUpdate.After(newest) {
		newest = snippets.LatestUpdate
	}
	feed.Updated = newest.UTC().Format(time.RFC3339)

	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal feed: %w", err)
	}
	body = append([]byte(xml.Header), body...)

	// Same conditional-request handling as the HTML path, so a reader polling
	// every few minutes mostly gets 304s.
	hash := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(hash[:16]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", newest.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=600")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Write(body)
	return nil
}
