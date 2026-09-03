package public

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The language of a request.
//
// A website's main language keeps its addresses: /hofladen stays /hofladen when
// French is switched on. Every further language sits behind a prefix, /fr/…, and
// the prefix is taken off here — before anything is routed, so every existing
// route works in every language without knowing that languages exist.

type localeKey struct{}

// LocaleMiddleware strips a language prefix and remembers which one it was.
//
// It runs inside the domain resolver, because which prefixes are languages
// depends on the website: /it/… is a language on one site and a page on the
// next. A prefix the site does not have is left alone and ends up as an
// ordinary 404 — better than silently serving the main language under an
// address nobody chose.
func LocaleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		website := domain.WebsiteFromContext(r.Context())
		if website == nil || !website.Multilingual() {
			next.ServeHTTP(w, r)
			return
		}

		tag, rest := locale.Split(r.URL.Path, website.Locales())
		if tag == "" {
			next.ServeHTTP(w, r)
			return
		}

		// The path is rewritten on a copy: the original stays in the access log
		// and in anything that has already looked at it.
		r2 := r.Clone(context.WithValue(r.Context(), localeKey{}, tag))
		r2.URL.Path = rest
		next.ServeHTTP(w, r2)
	})
}

// LocaleFrom returns the language of this request, empty for the main one.
func LocaleFrom(ctx context.Context) string {
	tag, _ := ctx.Value(localeKey{}).(string)
	return tag
}

// localePath puts the language being served in front of a site-relative path.
//
// Every address a page prints goes through here. A link built without it works
// perfectly on the main language and quietly throws the visitor back into it on
// every other one — the sort of defect that only shows up when somebody
// actually clicks through the second language.
func (h *Handler) localePath(r *http.Request, website *domain.Website, path string) string {
	return locale.Path(LocaleFrom(r.Context()), website.Locale, path)
}

// archiveURL is the address of the post archive in the language being served.
func (h *Handler) archiveURL(r *http.Request, website *domain.Website) string {
	return h.localePath(r, website, website.ArchiveURL())
}

// languageHomes is the switcher where there is no single page to switch to: the
// archive, the search, the 404. Every language points at its own start page.
//
// A language the operator has switched on but not written yet is still offered.
// It is the one case where a link can lead to a 404, and the alternative —
// counting the pages of every language on every request — costs a query per
// language on the hot path to hide a mistake the operator can see for
// themselves the moment they look at their own site.
func (h *Handler) languageHomes(r *http.Request, website *domain.Website) []tmpl.LanguageLink {
	if !website.Multilingual() {
		return nil
	}
	aktuell := LocaleFrom(r.Context())

	out := []tmpl.LanguageLink{{
		Code: codeOf("", website.Locale), Name: locale.Native(website.Locale),
		URL: "/", Active: aktuell == "",
	}}
	for _, tag := range website.Locales() {
		out = append(out, tmpl.LanguageLink{
			Code: tag, Name: locale.Native(tag),
			URL: locale.Path(tag, website.Locale, "/"), Active: aktuell == tag,
		})
	}
	return out
}

// translationLinks are the languages a page really exists in.
//
// Only what is there: a switcher that offers French and then answers 404 is
// worse than one that offers nothing. Built from the translation group, not
// from the list of languages the website has.
//
// A page that stands alone yields a single link, which is the same as none —
// the caller drops a list of one.
func (h *Handler) translationLinks(r *http.Request, website *domain.Website, pg *page.Page) []tmpl.LanguageLink {
	if !website.Multilingual() || h.pageStore == nil || pg == nil {
		return nil
	}
	übersetzungen, err := h.pageStore.Translations(r.Context(), pg)
	if err != nil {
		slog.Error("load translations", "err", err, "page", pg.ID)
		return nil
	}
	if len(übersetzungen) < 2 {
		return nil
	}

	aktuell := LocaleFrom(r.Context())
	out := make([]tmpl.LanguageLink, 0, len(übersetzungen))
	for _, t := range übersetzungen {
		out = append(out, tmpl.LanguageLink{
			Code:   codeOf(t.Locale, website.Locale),
			Name:   locale.Native(codeOf(t.Locale, website.Locale)),
			URL:    locale.Path(t.Locale, website.Locale, "/"+t.Slug),
			Active: t.Locale == aktuell,
		})
	}
	return out
}

// switcher is what a theme renders as the language menu: the page's own
// translations where they exist, the language start pages otherwise.
//
// One field for a theme to read, so a theme author does not have to write the
// fallback themselves — and get it wrong on the archive, where there is no page
// to translate.
func (h *Handler) switcher(übersetzungen []tmpl.LanguageLink, homes []tmpl.LanguageLink) []tmpl.LanguageLink {
	if len(übersetzungen) > 0 {
		return übersetzungen
	}
	return homes
}

// codeOf turns a stored locale into the tag a theme prints: the empty string
// means the main language, and lang="" would be worse than a wrong language.
func codeOf(stored, primary string) string {
	if stored == "" {
		if primary == "" {
			return locale.Default
		}
		return primary
	}
	return stored
}

// canonicalWithLocale puts the language prefix back into an absolute address.
//
// The canonical is what a search engine files the page under. Without the
// prefix every language of a page would claim the same address, and only one of
// them would ever be indexed.
func canonicalWithLocale(base, tag, primary, path string) string {
	return strings.TrimSuffix(base, "/") + locale.Path(tag, primary, path)
}

// localePrefixOf is the prefix of the language being served: "" or "/fr".
//
// It reads the request rather than the website, so it works in the places that
// build an address without having the website to hand.
func localePrefixOf(r *http.Request) string {
	if tag := LocaleFrom(r.Context()); tag != "" {
		return "/" + tag
	}
	return ""
}
