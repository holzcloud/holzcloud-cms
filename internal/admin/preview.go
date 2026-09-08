package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// HandlePreview renders the public homepage for a website without requiring domain resolution.
func (h *Handler) HandlePreview(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	pg, err := h.pages.GetHomePage(r.Context(), ws.ID)
	if err != nil {
		return fmt.Errorf("preview home: %w", err)
	}
	if pg == nil {
		return h.renderPreview404(w, r, ws)
	}

	site := previewSiteData(ws)
	data := tmpl.PageData{
		Site:  site,
		Page:  h.previewPageContent(r, ws, pg),
		Menus: h.loadPreviewMenus(r, ws.ID),
		Meta:  previewMeta(site, pg, "/"),
	}

	content, err := h.loader.RenderPage(r.Context(), ws.ID, "home.html", data)
	if err != nil {
		return fmt.Errorf("render preview home: %w", err)
	}

	base := fmt.Sprintf("/admin/websites/%d/preview", id)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(rewritePreviewURLs(content, base))
	return nil
}

// HandlePreviewPage renders a specific page by slug (including drafts) without domain resolution.
func (h *Handler) HandlePreviewPage(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	pg, err := h.pages.GetPageBySlug(r.Context(), ws.ID, slug)
	if err != nil {
		return fmt.Errorf("preview page: %w", err)
	}
	if pg == nil {
		return h.renderPreview404(w, r, ws)
	}

	site := previewSiteData(ws)
	data := tmpl.PageData{
		Site:  site,
		Page:  h.previewPageContent(r, ws, pg),
		Menus: h.loadPreviewMenus(r, ws.ID),
		Meta:  previewMeta(site, pg, "/"+pg.Slug),
	}

	content, err := h.loader.RenderPage(r.Context(), ws.ID, "page.html", data)
	if err != nil {
		return fmt.Errorf("render preview page: %w", err)
	}

	base := fmt.Sprintf("/admin/websites/%d/preview", id)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(rewritePreviewURLs(content, base))
	return nil
}

// HandlePreviewAsset serves template static assets (CSS, images) for preview mode.
func (h *Handler) HandlePreviewAsset(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	assetPath, ok := tmpl.SafeAssetPath(r.PathValue("path"))
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	ws, err := h.domains.GetWebsite(r.Context(), id)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	content, err := h.loader.Asset(r.Context(), ws.ID, assetPath)
	if err != nil || content == nil {
		http.NotFound(w, r)
		return nil
	}

	w.Header().Set("Cache-Control", "no-cache")
	web.WriteAsset(w, r, assetPath, content)
	return nil
}

// rewritePreviewURLs rewrites absolute paths in rendered HTML so they resolve
// under the admin preview route instead of requiring domain-based routing.
//
// Site-relative href/src values are prefixed with the preview base. Routes that
// are already absolute in the admin origin — /media/ (public media) and /admin/
// — are left untouched, as are protocol-relative URLs ("//host/...").
func rewritePreviewURLs(content []byte, previewBase string) []byte {
	for _, attr := range []string{`href="/`, `src="/`} {
		content = rewriteAttr(content, attr, previewBase)
	}
	return content
}

// preservedPreviewPrefixes are path prefixes that already resolve correctly in
// the admin origin and must not be moved under the preview base.
var preservedPreviewPrefixes = []string{"media/", "admin/", "assets/", "/"}

func rewriteAttr(content []byte, attr, previewBase string) []byte {
	needle := []byte(attr)
	var out bytes.Buffer
	out.Grow(len(content))

	for {
		i := bytes.Index(content, needle)
		if i < 0 {
			out.Write(content)
			return out.Bytes()
		}
		// Write everything up to and including the attribute name and quote.
		end := i + len(needle)
		out.Write(content[:end-1]) // without the leading slash of the path

		rest := content[end:]
		if hasAnyPrefix(rest, preservedPreviewPrefixes) {
			out.WriteByte('/')
		} else {
			out.WriteString(previewBase + "/")
		}
		content = rest
	}
}

func hasAnyPrefix(b []byte, prefixes []string) bool {
	for _, p := range prefixes {
		if bytes.HasPrefix(b, []byte(p)) {
			return true
		}
	}
	return false
}

// loadPreviewMenus loads all menus for a website (mirrors public handler's loadMenus).
func (h *Handler) loadPreviewMenus(r *http.Request, websiteID int64) map[string][]menu.MenuNode {
	menus := make(map[string][]menu.MenuNode)
	menuList, err := h.menuStore.ListMenus(r.Context(), websiteID)
	if err != nil {
		return menus
	}
	for _, m := range menuList {
		tree, err := h.menuStore.GetMenuTree(r.Context(), websiteID, m.LocationKey)
		if err != nil {
			continue
		}
		menus[m.LocationKey] = tree
	}
	return menus
}

// previewSiteData mirrors what the public handler hands a theme, so the preview
// shows the same language, dates and metadata the visitor will get.
func previewSiteData(ws *domain.Website) tmpl.SiteData {
	site := tmpl.SiteData{
		Name:            ws.Name,
		Description:     ws.Description,
		MetaDescription: ws.MetaDescription,
		Locale:          ws.Locale,
		TimeZone:        ws.TimeZone,
	}
	if site.Locale == "" {
		site.Locale = tmpl.DefaultLocale
	}
	if site.TimeZone == "" {
		site.TimeZone = tmpl.DefaultTimeZone
	}
	return site
}

// previewPageContent maps a stored page onto what a theme sees in the preview.
//
// The album markers are expanded here, and that is the point of the method
// existing at all. A gallery bound to an album stores [[album:<slug>:<n>]] and
// nothing else — the pictures are looked up per request, which is GAL-03 — so
// writing pg.ContentHTML out unchanged showed an editor the internal syntax on
// the one screen that exists to show what will be published.
//
// The expansion is the public one, not a copy of it: album.Expand over an
// album.Set from the website's own store, with the website's locale for the
// lightbox's three control names, exactly as internal/public/pagedata.go and
// blockSet above both do. A second rendering of a tile would be a second place
// to get the fragment ids wrong.
//
// # What the preview still does NOT do, said out loud
//
// It does not expand snippet markers, and it did not before. That gap predates
// Phase 11 and is wider than one block: closing it means deciding what a
// preview shows for a snippet that is edited but not yet saved, which is a
// question about the preview and not about the album. A page with an
// unexpanded snippet is a page with one sentence missing; a page with an
// unexpanded album was a page with a bracketed token where a whole gallery
// belongs, which is why only the second is fixed here.
//
// It also does not run the plugin filter or the responsive rewrite, for the
// same reason internal/public/pagedata.go's expandForFeed does not: neither
// leaves anything unreadable behind.
func (h *Handler) previewPageContent(r *http.Request, ws *domain.Website, pg *page.Page) tmpl.PageContent {
	updated := pg.UpdatedAt
	return tmpl.PageContent{
		Title:         pg.Title,
		ContentHTML:   template.HTML(album.Expand(pg.ContentHTML, h.previewAlbums(r, ws, pg.ContentHTML))),
		Slug:          pg.Slug,
		PublishedAt:   pg.PublishedAt,
		UpdatedAt:     &updated,
		Excerpt:       pg.Excerpt,
		HasOwnHeading: startsWithHeading(pg.ContentHTML),
		// The preview has to lay a product out as a product, or it is not a
		// preview of the page that will be published.
		Art:    pg.TypeKey,
		IsPost: pg.IsPost(),
	}
}

// previewAlbums is the expansion set for one previewed page, and it never
// fails.
//
// The zero Set expands every marker to nothing, which is what a preview wants
// when the albums cannot be loaded or when no store is wired: a gallery that is
// not there is better than a bracketed token the editor then has to guess
// about. internal/public/pagedata.go's albumsFor is the same three lines for
// the same reason, and this is deliberately not a shared helper — internal/admin
// does not import internal/public, and one small function on each side is
// cheaper than the dependency.
func (h *Handler) previewAlbums(r *http.Request, ws *domain.Website, html string) album.Set {
	if h.albumStore == nil || ws == nil {
		return album.Set{}
	}
	locale := ws.Locale
	set, err := h.albumStore.LoadFor(r.Context(), ws.ID, html,
		func(word string) string { return i18n.T(locale, word) })
	if err != nil {
		slog.Error("load albums for preview", "err", err, "website", ws.ID)
		return album.Set{}
	}
	return set
}

// startsWithHeading mirrors the public handler: a page whose content already
// opens with an <h1> must not get a second one from the theme.
func startsWithHeading(html string) bool {
	trimmed := strings.TrimLeftFunc(html, unicode.IsSpace)
	return strings.HasPrefix(strings.ToLower(trimmed), "<h1")
}

func previewMeta(site tmpl.SiteData, pg *page.Page, path string) tmpl.MetaData {
	description := pg.MetaDescription
	for _, fallback := range []string{pg.Excerpt, site.MetaDescription, site.Description} {
		if strings.TrimSpace(description) != "" {
			break
		}
		description = fallback
	}
	return tmpl.MetaData{
		CanonicalURL: path,
		Description:  description,
		// A preview must never be indexed even if it somehow leaks out.
		NoIndex: true,
	}
}

// renderPreview404 renders the site's styled 404 page for preview.
func (h *Handler) renderPreview404(w http.ResponseWriter, r *http.Request, ws *domain.Website) error {
	websiteID := ws.ID
	content, err := h.loader.Render404(r.Context(), websiteID, previewSiteData(ws))
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	base := fmt.Sprintf("/admin/websites/%d/preview", websiteID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusNotFound)
	w.Write(rewritePreviewURLs(content, base))
	return nil
}
