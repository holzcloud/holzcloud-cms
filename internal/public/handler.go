package public

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/menu"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/term"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Handler serves public website pages.
type Handler struct {
	pageStore    *page.Store
	menuStore    *menu.Store
	mediaStore   *media.Store
	snippetStore *snippet.Store
	// termStore supplies the labels. It may be nil, in which case /tag/ 404s
	// and no labels are rendered.
	termStore *term.Store
	// kindStore supplies the website's own content kinds. Nil means a website
	// has pages and posts and nothing else, which is what every installation
	// looked like before they existed.
	kindStore *kind.Store
	// fieldStore supplies the website's own page fields. Nil means a theme sees
	// an empty .Page.Felder, which is what every theme saw before they existed.
	fieldStore *field.Store
	// plugins may be nil, in which case no hook is ever dispatched and the
	// public side behaves exactly as it did before there were plugins.
	plugins *plugin.Manager
	// products backs the shop. It may be nil, in which case every shop route
	// 404s and no shop data reaches a template.
	products *shop.Store
	// carts backs the basket, on the same terms.
	carts *shop.CartStore
	// orders backs the checkout, on the same terms.
	orders *shop.OrderStore
	// outbox takes the order confirmations. It may be nil, in which case no
	// mail is written and the shop works exactly as it did before.
	outbox *outbox.Store
	// payments talks to the payment provider. It may be nil or unconfigured,
	// in which case the checkout offers invoice and prepayment only.
	payments *payrexx.Client
	// share signs the links that show an unpublished page; unlock signs the
	// cookie that remembers a protected page was opened. Distinct keys, so one
	// token can never stand in for the other.
	share  *sharelink.Signer
	unlock *sharelink.Signer
	loader *tmpl.Loader
	// resolver supplies the canonical host. It may be nil, in which case the
	// requested host is used.
	resolver *domain.Resolver
	// domains and mail back the notification a plugin may send. Either being
	// nil means nothing is sent, which is the ordinary state of an installation
	// that has not set up a mail server.
	domains   *domain.Store
	mail      *mail.Queue
	dataDir   string
	defaultFS fs.FS // embedded default template FS for static assets
	secure    bool  // site is served over TLS; decides the scheme in absolute URLs
}

// NewHandler creates a public site handler.
func NewHandler(pageStore *page.Store, menuStore *menu.Store, mediaStore *media.Store, snippetStore *snippet.Store, loader *tmpl.Loader, resolver *domain.Resolver, dataDir string, defaultFS fs.FS, secure bool) *Handler {
	return &Handler{
		pageStore:    pageStore,
		menuStore:    menuStore,
		mediaStore:   mediaStore,
		snippetStore: snippetStore,
		loader:       loader,
		resolver:     resolver,
		dataDir:      dataDir,
		defaultFS:    defaultFS,
		secure:       secure,
	}
}

// SetNotify attaches what a plugin needs to notify the operator: the website
// settings, for the address, and the outbox, for the sending.
func (h *Handler) SetNotify(domains *domain.Store, q *mail.Queue) {
	h.domains, h.mail = domains, q
}

// SetTermStore attaches the label store. It is a setter rather than a
// constructor argument because it is wired after the handler exists.
func (h *Handler) SetTermStore(s *term.Store) { h.termStore = s }

// SetFieldStore supplies the website's own page fields.
func (h *Handler) SetFieldStore(s *field.Store) { h.fieldStore = s }

// SetKindStore attaches the website's own content kinds.
func (h *Handler) SetKindStore(s *kind.Store) { h.kindStore = s }

// SetShareSigners attaches the two link signers.
func (h *Handler) SetShareSigners(share, unlock *sharelink.Signer) {
	h.share, h.unlock = share, unlock
}

// ErrHandler wraps a handler function that returns an error.
func (h *Handler) ErrHandler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			slog.Error("public handler error", "err", err, "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// loadMenus loads the menus for a website as a map[locationKey][]MenuNode, in
// the language being served.
//
// A location the current language has no menu for falls back to the main
// language: a site that has just switched French on would otherwise lose its
// navigation entirely until every menu had been built a second time. The
// fallback is per location, so translating the main menu first and the footer
// later works exactly as an operator would expect.
func (h *Handler) loadMenus(r *http.Request, websiteID int64) map[string][]menu.MenuNode {
	menus := make(map[string][]menu.MenuNode)
	menuList, err := h.menuStore.ListMenus(r.Context(), websiteID)
	if err != nil {
		slog.Error("load menus for public", "err", err, "website", websiteID)
		return menus
	}

	loc := LocaleFrom(r.Context())
	// Own language first, so a fallback can never overwrite a real translation
	// no matter what order ListMenus returns.
	for _, wanted := range []string{loc, ""} {
		for _, m := range menuList {
			if m.Locale != wanted {
				continue
			}
			if _, done := menus[m.LocationKey]; done {
				continue
			}
			tree, err := h.menuStore.GetMenuTreeIn(r.Context(), websiteID, m.LocationKey, m.Locale)
			if err != nil {
				slog.Error("load menu tree", "err", err, "location", m.LocationKey)
				continue
			}
			menus[m.LocationKey] = tree
		}
		if loc == "" {
			break
		}
	}
	return menus
}

// HandleHome serves the homepage for the resolved website.
func (h *Handler) HandleHome(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	loc := LocaleFrom(r.Context())
	pg, err := h.pageStore.GetHomePageIn(r.Context(), website.ID, loc)
	if err != nil {
		return fmt.Errorf("get home page: %w", err)
	}
	if pg == nil {
		return h.serve404(w, r, website)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML
	inhalt := h.pageContent(r, website.ID, pg, snippets)
	inhalt.Uebersetzungen = h.translationLinks(r, website, pg)
	site.Sprachen = h.switcher(inhalt.Uebersetzungen, site.Sprachen)
	data := tmpl.PageData{
		Site:  site,
		Page:  inhalt,
		Menus: h.loadMenus(r, website.ID),
		Meta: h.homeStructuredData(website, site,
			h.withOGImage(r, website, h.metaIn(r, website, site, pg, "/"), pg), pg),
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "home.html", data)
	if err != nil {
		return fmt.Errorf("render home: %w", err)
	}

	h.serveCached(w, r, content, contentModTime(pg, snippets))
	return nil
}

// HandlePage serves a published page by slug for the resolved website.
func (h *Handler) HandlePage(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}

	slug := r.PathValue("slug")
	if slug == "" {
		return h.serve404(w, r, website)
	}

	// The archive has no route of its own — its address is a per-website
	// setting, which a mux registered once at startup cannot express. A real
	// page at the same slug would shadow it, so the archive is checked first
	// and the slug is reserved in the editor.
	if website.HasArchive() && slug == website.BlogBase {
		return h.HandleArchive(w, r, website)
	}
	// Und dasselbe für die Übersichtsseite einer eigenen Inhaltsart. Zuerst
	// geprüft, aus demselben Grund: eine Seite mit derselben Adresse würde sie
	// verdecken, und der Editor lehnt sie deshalb ab.
	if t, ok := kind.ByArchive(h.typesOf(r, website.ID), slug); ok {
		return h.HandleTypeArchive(w, r, website, t)
	}
	// The catalogue's address is a per-website setting too.
	if website.HasShop() && slug == website.ShopBase {
		return h.HandleShop(w, r, website, "")
	}

	// GetPublishedPageIn enforces status='published' -- drafts never served
	// (T-03-11) -- and the language, so /fr/kontakt never serves the German page.
	pg, err := h.pageStore.GetPublishedPageIn(r.Context(), website.ID, LocaleFrom(r.Context()), slug)
	if err != nil {
		return fmt.Errorf("get published page: %w", err)
	}
	if pg == nil {
		return h.serve404(w, r, website)
	}

	// Die Startseite hat eine Adresse und nicht zwei.
	//
	// Sie liegt als Seite mit der Adresse "home" in der Datenbank, und die
	// Wurzel der Sprache zeigt genau sie (GetHomePageIn sucht diese Adresse
	// zuerst). Ohne diese Umleitung war dieselbe Seite unter / und unter /home
	// zu haben, beide mit sich selbst als kanonischer Adresse und beide im
	// Sitemap — für eine Suchmaschine zwei Seiten mit demselben Text, und bei
	// fünf Sprachen zehn Adressen für fünf Seiten.
	//
	// 301 und nicht 302: die Adresse /home ist nie die richtige gewesen, und
	// wer doch darauf verlinkt hat, soll den Verweis umschreiben können.
	if slug == page.HomeSlug {
		http.Redirect(w, r, h.localePath(r, website, "/"), http.StatusMovedPermanently)
		return nil
	}

	// A protected page delivers nothing until the password has been entered —
	// checked before the content is even loaded into the template data.
	if pg.Protected() && !h.hasAccess(r, pg) {
		return h.serveGate(w, r, website, pg, false)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML
	inhalt := h.withArchiveNav(r, website, h.pageContent(r, website.ID, pg, snippets), pg)
	inhalt.Uebersetzungen = h.translationLinks(r, website, pg)
	site.Sprachen = h.switcher(inhalt.Uebersetzungen, site.Sprachen)
	data := tmpl.PageData{
		Site:  site,
		Page:  inhalt,
		Menus: h.loadMenus(r, website.ID),
		Meta: h.withStructuredData(r, website, site,
			h.withOGImage(r, website, h.metaIn(r, website, site, pg, "/"+pg.Slug), pg), pg),
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "page.html", data)
	if err != nil {
		return fmt.Errorf("render page: %w", err)
	}

	h.serveCached(w, r, content, contentModTime(pg, snippets))
	return nil
}

// HandleTemplateAsset serves static files from the template directory (CSS, images).
//
// D-23 asked for `max-age=31536000, immutable`, and that was served on a fixed
// address. `immutable` is a promise that the address changes when the content
// does; here it never did. A theme change therefore reached nobody who had once
// opened the site — for up to a year, and a reload did not help, because
// `immutable` tells the browser not even to ask. That was found the day a
// corrected stylesheet did not arrive.
//
// The long cache is kept for a caller that keeps the promise by putting a
// version in the address. Everything else gets an hour and a strong ETag: the
// second request is a conditional one and comes back 304 with no body, which
// costs a few hundred bytes and is always right.
func (h *Handler) HandleTemplateAsset(w http.ResponseWriter, r *http.Request) error {
	assetPath, ok := tmpl.SafeAssetPath(r.PathValue("path"))
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	// Resolve through the shared loader: disk (active template slug) -> built-in
	// slug -> embedded default. Falls back to the default template when the
	// request has no website context (unknown Host).
	if website := domain.WebsiteFromContext(r.Context()); website != nil {
		content, err := h.loader.Asset(r.Context(), website.ID, assetPath)
		if err != nil {
			return fmt.Errorf("load template asset: %w", err)
		}
		if content == nil {
			http.NotFound(w, r)
			return nil
		}
		serveAssetContent(w, r, assetPath, content)
		return nil
	}

	content, err := fs.ReadFile(h.defaultFS, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	serveAssetContent(w, r, assetPath, content)
	return nil
}

// serveAssetContent writes asset bytes with the correct Content-Type, the
// caching rule and an ETag.
func serveAssetContent(w http.ResponseWriter, r *http.Request, assetPath string, content []byte) {
	w.Header().Set("Cache-Control", assetCacheControl(r))
	web.WriteAsset(w, r, assetPath, content)
}

// assetCacheControl decides how long the answer may be kept.
//
// A version in the query is the caller saying the address changes with the
// content, which is exactly what `immutable` requires. Without one the file
// stays cheap to check rather than impossible to update.
func assetCacheControl(r *http.Request) string {
	if v := r.URL.Query().Get("v"); v != "" {
		return "public, max-age=31536000, immutable"
	}
	return "public, max-age=3600, must-revalidate"
}

// serve404 renders the styled 404 page using the site's template.
//
// Before giving up it checks the redirect table. The lookup only runs once
// nothing else matched, so it costs nothing on the hot path and turns what
// would be a dead link into a 301.
func (h *Handler) serve404(w http.ResponseWriter, r *http.Request, website *domain.Website) error {
	if redirect, err := h.pageStore.LookupRedirect(r.Context(), website.ID, r.URL.Path); err != nil {
		slog.Error("redirect lookup failed", "err", err, "path", r.URL.Path)
	} else if redirect != nil && redirect.ToPath != r.URL.Path {
		http.Redirect(w, r, redirect.ToPath, redirect.Code)
		return nil
	}

	// The last word before the 404 page. A plugin here pays a call only on a
	// miss, which is why it is a hook of its own and not the request hook: a
	// redirect table has no business being consulted on every page view.
	if h.plugins != nil && h.plugins.Active(plugin.HookNotFound, website.ID) {
		if out, ok := h.plugins.HandleNotFound(withRequest(r.Context(), r), website.ID, pluginRequest(r, website.ID)); ok {
			writePluginResponse(w, out)
			return nil
		}
	}

	// Nothing matched, no redirect covers it and no plugin claimed it, so this
	// is a genuinely broken inbound link.
	h.logMiss(r, website.ID)

	return h.renderNotFound(w, r, website)
}

// renderNotFound writes the themed 404 response.
func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request, website *domain.Website) error {
	content, err := h.loader.Render404(r.Context(), website.ID, h.siteData(r, website))
	if err != nil {
		slog.Error("render 404 template failed", "err", err, "website", website.ID)
		http.NotFound(w, r)
		return nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "HX-Request")
	// A 404 must never be indexed as a page in its own right.
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusNotFound)
	w.Write(content)
	return nil
}

// serveCached writes content with HTTP caching headers (ETag, Last-Modified, Cache-Control).
// Handles conditional requests (If-None-Match, If-Modified-Since) with 304 responses.
func (h *Handler) serveCached(w http.ResponseWriter, r *http.Request, content []byte, modTime time.Time) {
	// Compute ETag from content hash
	hash := sha256.Sum256(content)
	etag := `"` + hex.EncodeToString(hash[:16]) + `"`

	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Vary", "HX-Request")

	// Check If-None-Match -> 304
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Check If-Modified-Since -> 304
	if since := r.Header.Get("If-Modified-Since"); since != "" {
		t, err := http.ParseTime(since)
		if err == nil && !modTime.Truncate(time.Second).After(t) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// SetPlugins supplies the plugin manager, or nil for a site without plugins.
func (h *Handler) SetPlugins(m *plugin.Manager) { h.plugins = m }
