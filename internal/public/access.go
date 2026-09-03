package public

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// unlockCookiePrefix names the cookie that remembers a page was unlocked.
//
// One cookie per page, scoped to that page's path, so unlocking the trade price
// list does not quietly open the other protected page next to it.
const unlockCookiePrefix = "hc_frei_"

// unlockLifetime is how long an unlocked page stays open.
//
// A session cookie would be cleared when the browser closes, which is the right
// default, but a customer comparing two price lists over an afternoon should
// not have to retype. Twelve hours splits the difference.
const unlockLifetime = 12 * time.Hour

// unlockCookieName is the cookie for one page.
func unlockCookieName(pageID int64) string {
	return unlockCookiePrefix + strconv.FormatInt(pageID, 10)
}

// grantAccess sets the cookie that remembers an unlocked page.
//
// The value is a signed token rather than a flag: a cookie reading "yes" is one
// any visitor can write themselves.
func (h *Handler) grantAccess(w http.ResponseWriter, r *http.Request, pg *page.Page) {
	if h.unlock == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     unlockCookieName(pg.ID),
		Value:    h.unlock.Token(pg.ID, time.Now().Add(unlockLifetime)),
		Path:     "/" + pg.Slug,
		MaxAge:   int(unlockLifetime / time.Second),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// hasAccess reports whether the visitor has already unlocked this page.
func (h *Handler) hasAccess(r *http.Request, pg *page.Page) bool {
	if h.unlock == nil {
		return false
	}
	c, err := r.Cookie(unlockCookieName(pg.ID))
	if err != nil {
		return false
	}
	id, err := h.unlock.Check(c.Value, time.Now())
	return err == nil && id == pg.ID
}

// serveGate renders the password form in front of a protected page.
//
// It answers 401 rather than 200: the page has not been delivered, and a
// crawler that reads a 200 here would index the form as if it were the content.
func (h *Handler) serveGate(w http.ResponseWriter, r *http.Request, website *domain.Website, pg *page.Page, wrong bool) error {
	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title: pg.Title,
			Slug:  pg.Slug,
		},
		Menus: h.loadMenus(r, website.ID),
		Meta: tmpl.MetaData{
			CanonicalURL: site.URL + "/" + pg.Slug,
			// A protected page must never be indexed, whatever its own setting
			// says: what a crawler would file is the form, under the page's
			// title, which is worse than no entry at all.
			NoIndex: true,
		},
		Gate: tmpl.GateData{
			Hint:  pg.AccessHint,
			Path:  "/" + pg.Slug,
			Wrong: wrong,
		},
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "gate.html", data)
	if err != nil {
		return fmt.Errorf("render gate: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Never cached: a shared proxy holding this would hand the form to someone
	// who has already unlocked, or worse, the page to someone who has not.
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write(content)
	return nil
}

// HandleUnlock takes the password typed into the gate.
func (h *Handler) HandleUnlock(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	slug := strings.TrimPrefix(strings.TrimSpace(r.FormValue("seite")), "/")
	if slug == "" {
		return h.serve404(w, r, website)
	}
	pg, err := h.pageStore.GetPublishedPage(r.Context(), website.ID, slug)
	if err != nil {
		return fmt.Errorf("get page for unlock: %w", err)
	}
	if pg == nil || !pg.Protected() {
		return h.serve404(w, r, website)
	}

	if !page.CheckPagePassword(pg, r.FormValue("passwort")) {
		// Rendered directly rather than redirected: the visitor sees the form
		// again with the message, and the wrong password never reaches a URL
		// or a server log.
		return h.serveGate(w, r, website, pg, true)
	}

	h.grantAccess(w, r, pg)
	// 303, so a reload does not repost the password.
	http.Redirect(w, r, "/"+pg.Slug, http.StatusSeeOther)
	return nil
}

// HandleShareLink serves an unpublished page through a signed link.
//
// A customer approving a draft should not need an account, and the alternative
// people reach for otherwise is publishing it "just for a minute" — which is
// how a half-finished price list ends up in a search index.
func (h *Handler) HandleShareLink(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}
	if h.share == nil {
		return h.serve404(w, r, website)
	}

	pageID, err := h.share.Check(r.PathValue("token"), time.Now())
	if err != nil {
		// Both outcomes are a 404 to a crawler, but an expired link tells the
		// person holding it to ask for a new one, which a bare 404 does not.
		return h.serveShareError(w, r, website, err)
	}

	pg, err := h.pageStore.GetForPreview(r.Context(), website.ID, pageID)
	if err != nil {
		return fmt.Errorf("get shared page: %w", err)
	}
	if pg == nil {
		return h.serveShareError(w, r, website, sharelink.ErrInvalid)
	}

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	site.Snippets = snippets.HTML

	data := tmpl.PageData{
		Site:  site,
		Page:  h.withArchiveNav(r, website, h.pageContent(r, website.ID, pg, snippets), pg),
		Menus: h.loadMenus(r, website.ID),
		Meta: tmpl.MetaData{
			CanonicalURL: site.URL + "/" + pg.Slug,
			Description:  pg.Excerpt,
			NoIndex:      true,
		},
		// The banner is not decoration: without it a customer cannot tell an
		// approved page from a draft, and will say "it's live, I saw it".
		Preview: tmpl.PreviewData{
			Active: true,
			Status: pg.Status,
		},
	}

	content, err := h.loader.RenderPage(r.Context(), website.ID, "page.html", data)
	if err != nil {
		return fmt.Errorf("render shared page: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Write(content)
	return nil
}

// serveShareError explains a link that no longer works.
func (h *Handler) serveShareError(w http.ResponseWriter, r *http.Request, website *domain.Website, cause error) error {
	site := h.siteData(r, website)
	title := "Vorschaulink ungültig"
	message := "Dieser Link ist nicht gültig."
	if cause == sharelink.ErrExpired {
		title = "Vorschaulink abgelaufen"
		message = "Dieser Vorschaulink ist abgelaufen. Bitte lass dir einen neuen schicken."
	}

	data := tmpl.PageData{
		Site:  site,
		Page:  tmpl.PageContent{Title: title},
		Menus: h.loadMenus(r, website.ID),
		Meta:  tmpl.MetaData{NoIndex: true, Message: message},
	}
	content, err := h.loader.RenderPage(r.Context(), website.ID, "404.html", data)
	if err != nil {
		http.Error(w, message, http.StatusNotFound)
		return nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusNotFound)
	w.Write(content)
	return nil
}
