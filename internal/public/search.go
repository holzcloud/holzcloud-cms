package public

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// searchLimit bounds a result page. There is no pagination: on a site with a
// few dozen pages, twenty hits is already more than anyone reads.
const searchLimit = 20

// HandleSearch serves the public site search.
//
// It goes through the same store call as the admin search but with drafts
// excluded, so the visibility rules cannot drift apart between the two.
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := h.pageStore.SearchPages(r.Context(), website.ID, query, false, searchLimit)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	h.fillSnippets(r, &site, website.ID, snippets)

	list := tmpl.SearchData{Query: query, Submitted: query != ""}
	// The passages come from the Markdown source, so a value stands in them as
	// the marker the editor wrote, whole or cut in half by the passage's edge.
	// value.ResolvePlain puts the text in, or removes the half, and only then
	// does page.Highlight escape the passage and mark the terms — never the
	// other way round, which would print the value's text unescaped.
	//
	// The texts are loaded once per request and only when a passage holds a
	// bracket pair's half; a search that meets no marker asks nothing.
	//
	// Said out loud: the index holds the Markdown and not the value's text, so
	// searching for "0.1.5" does not find the page that shows it. That is a
	// limit, stated where values are documented, and not a fault of this loop.
	var texts map[string]string
	textsLoaded := false
	for _, res := range results {
		snippet := res.Snippet
		if strings.Contains(res.Raw, "[[") || strings.Contains(res.Raw, "]]") {
			if !textsLoaded {
				texts, _ = h.valueTexts(r.Context(), website.ID)
				textsLoaded = true
			}
			snippet = page.Highlight(value.ResolvePlain(res.Raw, texts))
		}
		list.Results = append(list.Results, tmpl.SearchHit{
			Title:   res.Page.Title,
			URL:     "/" + res.Page.Slug,
			Snippet: snippet,
		})
	}

	title := "Suche"
	if query != "" {
		title = "Suche: " + query
	}
	data := tmpl.PageData{
		Site:  site,
		Page:  tmpl.PageContent{Title: title, Slug: "suche"},
		Menus: h.loadMenus(r, website.ID),
		// A result list is not content in its own right and must not compete
		// with the pages it points at.
		Meta:   tmpl.MetaData{CanonicalURL: site.URL + "/suche", NoIndex: true},
		Search: list,
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "search.html", data)
	if err != nil {
		return fmt.Errorf("render search: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Results depend on the query string and on content that may change at any
	// time, so there is nothing worth caching here.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Write(content)
	return nil
}
