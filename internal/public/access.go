package public

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// unlockCookiePrefix and unlockLifetime are the exported constants of package
// page under their local names. The reasons stand with them, because the
// privacy register reads them from there.
const (
	unlockCookiePrefix = page.UnlockCookiePrefix
	unlockLifetime     = page.UnlockLifetime
)

// What the password form of a protected page lets a visitor try.
//
// The form needs no account and no CSRF token, so without a limit it was the
// cheapest way to make this server run Argon2 for a stranger, and the way to
// guess a page password at the speed of the network.
const (
	// gateClientMax is the attempts one client may make at one page in the
	// window. A customer mistypes a password a few times, not ten.
	gateClientMax = 10
	// gatePageMax is the attempts all clients together may make at one page in
	// the window. It is loose on purpose: it is shared, so reaching it closes
	// the page to everybody, and one client must not be able to do that quickly
	// (it takes ten clients to spend it). It still gives a guess spread over
	// many addresses a ceiling.
	gatePageMax = 100
	// gateWindow is how long an attempt counts.
	gateWindow = 15 * time.Minute
	// gateMaxBody bounds what the form may send: a page address and a
	// password. Anything larger is not this form, and is refused before any
	// work is done for it.
	gateMaxBody = 4 << 10
)

// unlockCookieName is the cookie for one page.
func unlockCookieName(pageID int64) string {
	return unlockCookiePrefix + strconv.FormatInt(pageID, 10)
}

// unlockSigner is the signer that mints and checks the unlock of one page.
//
// It is derived from the unlock signer and from a fingerprint of the page's
// stored password hash, so an unlock is bound to the password it was given for:
// a new password has a fresh salt and so a new hash, the fingerprint changes,
// and every earlier unlock stops opening the page. Changing only the hint keeps
// the hash, and the unlocks with it. Re-entering the same password in the
// editor ends them too, because the salt is new.
//
// A fingerprint of what is already stored, and not a counter in a new column:
// it needs no migration, and the share-link and opt-out signers and the token
// format stay exactly as they were. The price is that every unlock issued
// before this version, minted by the underived signer, ends once at the
// upgrade and visitors type the password again (owner decision D11).
//
// A nil unlock signer stays nil: without one nothing is granted or accepted.
func (h *Handler) unlockSigner(pg *page.Page) *sharelink.Signer {
	if h.unlock == nil {
		return nil
	}
	sum := sha256.Sum256([]byte(pg.AccessPassword))
	return h.unlock.Derive("page:" + strconv.FormatInt(pg.ID, 10) + ":" + hex.EncodeToString(sum[:8]))
}

// pageAddress is where a page lives, in its own language: "/geheim" in the main
// language, "/fr/secret" in a further one.
func pageAddress(website *domain.Website, pg *page.Page) string {
	return locale.Path(pg.Locale, website.Locale, "/"+pg.Slug)
}

// grantAccess sets the cookie that remembers an unlocked page, for the path
// the page is served at.
//
// The value is a signed token rather than a flag: a cookie reading "yes" is one
// any visitor can write themselves.
//
// The path is the caller's, because it is not always the page's slug: the home
// page is served at "/" (or "/fr"), and a cookie scoped to "/home" is never
// sent there — the visitor would type the right password and meet the gate
// again. A page in a further language lives under its prefix for the same
// reason. The name carries the page id, so a wider path opens nothing else.
func (h *Handler) grantAccess(w http.ResponseWriter, pg *page.Page, path string) {
	if h.unlock == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     unlockCookieName(pg.ID),
		Value:    h.unlockSigner(pg).Token(pg.ID, time.Now().Add(unlockLifetime)),
		Path:     path,
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
	id, err := h.unlockSigner(pg).Check(c.Value, time.Now())
	return err == nil && id == pg.ID
}

// serveGate renders the password form in front of a protected page.
//
// It answers 401 rather than 200: the page has not been delivered, and a
// crawler that reads a 200 here would index the form as if it were the content.
func (h *Handler) serveGate(w http.ResponseWriter, r *http.Request, website *domain.Website, pg *page.Page, wrong bool) error {
	return h.serveGateStatus(w, r, website, pg, wrong, http.StatusUnauthorized)
}

// serveGateStatus is serveGate with the status the answer carries: 401 for a
// locked page and a wrong password, 429 and 503 for a visitor who is not
// allowed to try just now.
func (h *Handler) serveGateStatus(w http.ResponseWriter, r *http.Request, website *domain.Website, pg *page.Page, wrong bool, status int) error {
	// The address of the page in its own language. Every shipped theme posts
	// {{.Page.Slug}} as the hidden "seite" field and none may be edited, so the
	// slug carries the language prefix ("fr/secret") and the server reads the
	// language back from it. The same value is what the layouts of the gate
	// page mark the current menu entry with.
	address := pageAddress(website, pg)

	site := h.siteData(r, website)
	snippets := h.loadSnippets(r, website.ID)
	h.fillSnippets(r, &site, website.ID, snippets)

	data := tmpl.PageData{
		Site: site,
		Page: tmpl.PageContent{
			Title: pg.Title,
			Slug:  strings.TrimPrefix(address, "/"),
		},
		Menus: h.loadMenus(r, website.ID),
		Meta: tmpl.MetaData{
			CanonicalURL: site.URL + address,
			// A protected page must never be indexed, whatever its own setting
			// says: what a crawler would file is the form, under the page's
			// title, which is worse than no entry at all.
			NoIndex: true,
		},
		Gate: tmpl.GateData{
			Hint:  pg.AccessHint,
			Path:  address,
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
	w.WriteHeader(status)
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
	r.Body = http.MaxBytesReader(w, r.Body, gateMaxBody)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return nil
		}
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil
	}

	// The gate posts the page's address without its leading slash, in a further
	// language with the prefix ("fr/secret"). A form posted under the prefix
	// (/fr/freischalten) carries the language in the request instead.
	tag, rest := locale.Split("/"+strings.TrimLeft(strings.TrimSpace(r.FormValue("seite")), "/"), website.Locales())
	if tag == "" {
		tag = LocaleFrom(r.Context())
	}
	slug := strings.TrimPrefix(rest, "/")
	if slug == "" {
		return h.serve404(w, r, website)
	}
	pg, err := h.pageStore.GetPublishedPageIn(r.Context(), website.ID, tag, slug)
	if err != nil {
		return fmt.Errorf("get page for unlock: %w", err)
	}
	if pg == nil || !pg.Protected() {
		return h.serve404(w, r, website)
	}
	// From here on every answer is about this page, so it is given in this
	// page's language: the gate drawn again after a wrong password, a full
	// throttle or a busy check carries the page's words and menus, not those of
	// the unprefixed address the form posts to.
	r = r.WithContext(withPageLocale(r.Context(), website, pg.Locale))

	// A client has a budget per page; an address that names no visitor
	// (loopback, a private network: what a proxy looks like when it is not in
	// HOLZCLOUD_TRUSTED_PROXIES) has none, because every visitor would share it
	// and the first stranger would close the page after ten tries. Only the
	// page's budget applies to them, which a stranger can still spend, with a
	// hundred wrong tries instead of ten; naming the proxy is the remedy.
	pageKey := strconv.FormatInt(pg.ID, 10)
	clientKey := ""
	if key, public := h.clientIP.ThrottleKey(r); public {
		clientKey = key + "|" + pageKey
	}
	attempt, wait := h.unlockThrottle.Reserve(clientKey, pageKey)
	if attempt == nil {
		// The theme's own gate, without the wrong-password line (nothing was
		// checked), without a cookie and without the password.
		web.RetryAfter(w, wait)
		return h.serveGateStatus(w, r, website, pg, false, http.StatusTooManyRequests)
	}
	// Whatever way this request leaves without having decided the attempt (a
	// busy check, a failed lookup), both reservations go back.
	defer attempt.Refund()

	ok, err := page.CheckPagePassword(pg, r.FormValue("passwort"))
	if errors.Is(err, auth.ErrBusy) {
		// The server could not look at the password just now. That is not a
		// wrong password and is not counted against the visitor.
		web.RetryAfter(w, auth.BusyRetryAfter)
		return h.serveGateStatus(w, r, website, pg, false, http.StatusServiceUnavailable)
	}
	if err != nil {
		return fmt.Errorf("check page password: %w", err)
	}
	if !ok {
		attempt.Fail()
		// Rendered directly rather than redirected: the visitor sees the form
		// again with the message, and the wrong password never reaches a URL
		// or a server log.
		return h.serveGate(w, r, website, pg, true)
	}
	// A right password is not a failure, and it is not "cleared" either: a page
	// password is shared by many customers, and clearing the page's budget each
	// time one of them opens the page would hand somebody guessing from many
	// addresses a fresh budget every time. Both reservations are given back.
	attempt.Refund()

	home, err := h.pageStore.GetHomePageIn(r.Context(), website.ID, pg.Locale)
	if err != nil {
		return fmt.Errorf("get home page for unlock: %w", err)
	}
	isHome := home != nil && home.ID == pg.ID

	// The page's own address, with its language prefix: "/geheim", "/fr/secret",
	// and "/" or "/fr" for a start page, which is served there and not at /home.
	target := pageAddress(website, pg)
	if isHome {
		target = locale.Path(pg.Locale, website.Locale, "/")
	}
	// The slug is data: a page stored under one that a browser reads as
	// another server ("\host") goes back to the language's start page instead.
	if checked, ok := web.LocalPath(target); ok {
		target = checked
	} else {
		target = locale.Path(pg.Locale, website.Locale, "/")
	}
	h.grantAccess(w, pg, target)
	// 303, so a reload does not repost the password.
	http.Redirect(w, r, target, http.StatusSeeOther)
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
	h.fillSnippets(r, &site, website.ID, snippets)

	// The validator is discarded here and only here: a share link answers
	// no-store, so there is no conditional request for it to decide.
	inhalt, _ := h.pageContent(r, website.ID, pg, snippets)

	data := tmpl.PageData{
		Site:  site,
		Page:  h.withArchiveNav(r, website, inhalt, pg),
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
	// The same area as on every route, for the same reason as in renderNotFound:
	// the theme's 404 template is drawn with its footer, and whoever lands here
	// wants to reach somebody.
	h.fillSnippets(r, &site, website.ID, h.loadSnippets(r, website.ID))
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
