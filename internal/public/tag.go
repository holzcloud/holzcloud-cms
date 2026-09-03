package public

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// HandleTag renders everything carrying one label.
//
// Unlike the post archive this has a fixed address: "tag" is in the reserved
// slug list precisely so it can be a route, and a per-website prefix would be
// one setting more for something nobody renames.
func (h *Handler) HandleTag(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}
	if h.termStore == nil {
		return h.serve404(w, r, website)
	}

	slug := r.PathValue("slug")
	if slug == "" {
		return h.serve404(w, r, website)
	}
	t, err := h.termStore.GetBySlug(r.Context(), website.ID, slug)
	if err != nil {
		return fmt.Errorf("get term: %w", err)
	}
	if t == nil {
		return h.serve404(w, r, website)
	}

	pageNum := archivePage(r)
	perPage := website.PostsPerPage
	if perPage <= 0 {
		perPage = 10
	}

	items, total, err := h.termStore.ListTaggedIn(r.Context(), website.ID, t.ID, LocaleFrom(r.Context()), pageNum, perPage)
	if err != nil {
		return fmt.Errorf("list tagged: %w", err)
	}
	// A label whose content is all draft leads nowhere; that is a 404 rather
	// than an empty page, which would otherwise confirm the drafts exist.
	if total == 0 {
		return h.serve404(w, r, website)
	}

	totalPages := (total + perPage - 1) / perPage
	if pageNum > totalPages {
		return h.serve404(w, r, website)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML

	archive := h.archiveData(r, website, items, pageNum, totalPages, total)
	archive.Term = t.Name
	archive.PrevURL, archive.NextURL = h.tagPager(r, website, t, pageNum, totalPages)

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title:      t.Name,
			Slug:       t.Slug,
			ArchiveURL: h.archiveURL(r, website),
		},
		Menus:   h.loadMenus(r, website.ID),
		Meta:    metaData(site, &page.Page{Title: t.Name}, h.tagPath(r, website, t, pageNum)),
		Archive: archive,
	}
	data.Meta.CanonicalURL = strings.TrimSuffix(site.URL, "/") + h.tagPath(r, website, t, pageNum)
	if pageNum > 1 {
		data.Meta.NoIndex = true
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "list.html", data)
	if err != nil {
		return fmt.Errorf("render tag archive: %w", err)
	}

	h.serveCached(w, r, content, archiveModTime(items, snippets))
	return nil
}

// tagPager builds the two pager links of a label archive.
func (h *Handler) tagPager(r *http.Request, website *domain.Website, t *term.Term, pageNum, totalPages int) (prev, next string) {
	if pageNum > 1 {
		prev = h.tagPath(r, website, t, pageNum-1)
	}
	if pageNum < totalPages {
		next = h.tagPath(r, website, t, pageNum+1)
	}
	return prev, next
}

func (h *Handler) tagPath(r *http.Request, website *domain.Website, t *term.Term, pageNum int) string {
	base := h.localePath(r, website, t.URL())
	if pageNum <= 1 {
		return base
	}
	return base + "?seite=" + strconv.Itoa(pageNum)
}

// termLinks maps stored labels onto what a theme sees.
func termLinks(terms []term.Term) []tmpl.TermLink {
	return termLinksAt("", terms)
}

// termLinksAt is termLinks with the language prefix in front of every address,
// so a label under /fr stays under /fr.
func termLinksAt(prefix string, terms []term.Term) []tmpl.TermLink {
	if len(terms) == 0 {
		return nil
	}
	out := make([]tmpl.TermLink, len(terms))
	for i, t := range terms {
		out[i] = tmpl.TermLink{Name: t.Name, URL: prefix + t.URL(), Count: t.Count}
	}
	return out
}

// localeTermLinks is termLinksAt for the language being served.
func (h *Handler) localeTermLinks(r *http.Request, website *domain.Website, terms []term.Term) []tmpl.TermLink {
	return termLinksAt(locale.Prefix(LocaleFrom(r.Context()), website.Locale), terms)
}
