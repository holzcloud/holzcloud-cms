package public

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// Die Übersichtsseite einer eigenen Inhaltsart.
//
// Dasselbe wie das Archiv der Beiträge, nur für eine Art, die der Betreiber
// selbst angelegt hat — und mit demselben Theme: eine Website, die "Produkte"
// führt, bekommt ihre Liste ohne dass jemand eine Vorlage anfassen müsste.
//
// Wie beim Archiv gibt es keine eigene Route: die Adresse steht in der
// Datenbank, und ein Mux, der beim Start gebaut wird, kann sie nicht kennen.

// typesOf are a website's own kinds, or none when the store is absent.
func (h *Handler) typesOf(r *http.Request, websiteID int64) []kind.Type {
	if h.kindStore == nil {
		return nil
	}
	types, err := h.kindStore.List(r.Context(), websiteID)
	if err != nil {
		return nil
	}
	return types
}

// HandleTypeArchive renders the overview of one own kind.
func (h *Handler) HandleTypeArchive(w http.ResponseWriter, r *http.Request,
	website *domain.Website, t kind.Type) error {

	pageNum := archivePage(r)
	perPage := website.PostsPerPage
	if perPage <= 0 {
		perPage = 10
	}

	loc := LocaleFrom(r.Context())
	entries, total, err := h.pageStore.ListOfKind(r.Context(), website.ID, loc, t.Key,
		t.SortsByTitle(), pageNum, perPage)
	if err != nil {
		return fmt.Errorf("list %s: %w", t.Key, err)
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	// Same rule as the archive: page forty of a three-page list is a 404 and
	// not an empty page answering 200.
	if pageNum > totalPages && total > 0 {
		return h.serve404(w, r, website)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML

	base := h.localePath(r, website, "/"+t.Archive)
	path := base
	if pageNum > 1 {
		path = base + "?seite=" + strconv.Itoa(pageNum)
	}

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title:      t.Plural,
			Slug:       t.Archive,
			ArchiveURL: base,
		},
		Menus:   h.loadMenus(r, website.ID),
		Meta:    metaData(site, &page.Page{Title: t.Plural}, path),
		Archive: h.typeArchiveData(r, website, entries, base, pageNum, totalPages, total),
	}
	data.Meta.CanonicalURL = strings.TrimSuffix(site.URL, "/") + path
	if pageNum > 1 {
		data.Meta.NoIndex = true
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "list.html", data)
	if err != nil {
		return fmt.Errorf("render %s: %w", t.Key, err)
	}

	h.serveCached(w, r, content, archiveModTime(entries, snippets))
	return nil
}

// typeArchiveData is archiveData with the addresses of this kind's overview.
func (h *Handler) typeArchiveData(r *http.Request, website *domain.Website, entries []page.Page,
	base string, pageNum, totalPages, total int) tmpl.ArchiveData {

	data := h.archiveData(r, website, entries, pageNum, totalPages, total)
	data.PrevURL, data.NextURL = "", ""
	if pageNum > 1 {
		data.PrevURL = base
		if pageNum > 2 {
			data.PrevURL = base + "?seite=" + strconv.Itoa(pageNum-1)
		}
	}
	if pageNum < totalPages {
		data.NextURL = base + "?seite=" + strconv.Itoa(pageNum+1)
	}
	return data
}
