package public

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
)

// HandleArchive renders the list of posts.
//
// There is no route of its own: the archive lives at whatever address the
// operator chose, so HandlePage recognises it and calls in here. A separate
// route would have to be registered per website, which the mux cannot do, and
// a fixed /blog would be wrong for a German site.
func (h *Handler) HandleArchive(w http.ResponseWriter, r *http.Request, website *domain.Website) error {
	pageNum := archivePage(r)
	perPage := website.PostsPerPage
	if perPage <= 0 {
		perPage = 10
	}

	loc := LocaleFrom(r.Context())
	posts, total, err := h.pageStore.ListArchiveIn(r.Context(), website.ID, loc, pageNum, perPage)
	if err != nil {
		return fmt.Errorf("list archive: %w", err)
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	// Asking for page 40 of a three-page archive is a 404, not an empty list:
	// an empty page that answers 200 is what fills a search index with
	// thousands of identical results.
	if pageNum > totalPages && total > 0 {
		return h.serve404(w, r, website)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML

	title := "Aktuelles"
	if website.Locale == "en" {
		title = "News"
	}

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title:      title,
			Slug:       website.BlogBase,
			ArchiveURL: h.archiveURL(r, website),
		},
		Menus:   h.loadMenus(r, website.ID),
		Meta:    metaData(site, &page.Page{Title: title}, h.archivePath(r, website, pageNum)),
		Archive: h.archiveData(r, website, posts, pageNum, totalPages, total),
	}
	data.Meta.CanonicalURL = strings.TrimSuffix(site.URL, "/") + h.archivePath(r, website, pageNum)
	// Page two of an archive is not a page anyone should land on from a search
	// result; the canonical always points at the entry itself.
	if pageNum > 1 {
		data.Meta.NoIndex = true
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "list.html", data)
	if err != nil {
		return fmt.Errorf("render archive: %w", err)
	}

	h.serveCached(w, r, content, archiveModTime(posts, snippets))
	return nil
}

// archiveData maps the posts onto what list.html sees.
func (h *Handler) archiveData(r *http.Request, website *domain.Website, posts []page.Page,
	pageNum, totalPages, total int) tmpl.ArchiveData {

	data := tmpl.ArchiveData{Page: pageNum, TotalPages: totalPages, Total: total}
	labels := h.labelsFor(r, posts)
	for _, p := range posts {
		published := p.PublishedAt
		if published == nil {
			created := p.CreatedAt
			published = &created
		}
		data.Entries = append(data.Entries, tmpl.ArchiveEntry{
			Title:       p.Title,
			URL:         h.localePath(r, website, "/"+p.Slug),
			Excerpt:     p.Excerpt,
			PublishedAt: published,
			ImageURL:    h.mediaURL(r.Context(), website.ID, p.FeaturedMediaID),
			Terms:       h.localeTermLinks(r, website, labels[p.ID]),
		})
	}
	if pageNum > 1 {
		data.PrevURL = h.archivePath(r, website, pageNum-1)
	}
	if pageNum < totalPages {
		data.NextURL = h.archivePath(r, website, pageNum+1)
	}
	return data
}

// labelsFor loads the labels of a whole listing in one query.
//
// One query for the page rather than one per entry: ten entries would otherwise
// cost eleven round trips to show something that is decoration.
func (h *Handler) labelsFor(r *http.Request, items []page.Page) map[int64][]term.Term {
	if h.termStore == nil || len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	for i, p := range items {
		ids[i] = p.ID
	}
	labels, err := h.termStore.ForPages(r.Context(), ids)
	if err != nil {
		return nil
	}
	return labels
}

// archivePath builds the address of one page of the archive.
//
// "seite" rather than "page", because the rest of the public site speaks German
// to its visitors and a query parameter is part of what they see.
func (h *Handler) archivePath(r *http.Request, website *domain.Website, pageNum int) string {
	base := h.archiveURL(r, website)
	if pageNum <= 1 {
		return base
	}
	return base + "?seite=" + strconv.Itoa(pageNum)
}

// archivePage reads the pager position, defaulting to the first page.
func archivePage(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("seite"))
	if err != nil || n < 1 {
		return 1
	}
	// A bound, so a crawler following a fabricated ?seite=99999999 cannot make
	// the database compute an enormous offset.
	if n > 10000 {
		return 10000
	}
	return n
}

// archiveModTime is the newest thing on the page, for conditional requests.
//
// The snippets count too: an archive layout may place one, and a browser
// holding an If-Modified-Since would otherwise keep the old text.
func archiveModTime(posts []page.Page, snippets snippet.Rendered) time.Time {
	newest := snippets.LatestUpdate
	for _, p := range posts {
		if p.UpdatedAt.After(newest) {
			newest = p.UpdatedAt
		}
	}
	return newest
}

// withArchiveNav fills in the neighbouring entries of a post.
func (h *Handler) withArchiveNav(r *http.Request, website *domain.Website,
	content tmpl.PageContent, pg *page.Page) tmpl.PageContent {

	if !pg.IsPost() {
		return content
	}
	content.IsPost = true
	content.ArchiveURL = h.archiveURL(r, website)

	prev, next, err := h.pageStore.AdjacentPosts(r.Context(), website.ID, pg)
	if err != nil {
		// Navigation is a convenience; the entry itself still renders.
		return content
	}
	if prev != nil {
		content.Prev = &tmpl.PageLink{Title: prev.Title, URL: h.localePath(r, website, "/"+prev.Slug)}
	}
	if next != nil {
		content.Next = &tmpl.PageLink{Title: next.Title, URL: h.localePath(r, website, "/"+next.Slug)}
	}
	return content
}
