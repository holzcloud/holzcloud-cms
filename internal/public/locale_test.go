package public

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// multilingualSite is a website with a main language and further ones, set up
// through the ordinary settings path so the same cleaning applies as in the
// admin form.
func multilingualSite(t *testing.T, database *db.DB, extras string) *domain.Website {
	t.Helper()
	ws := seedWebsite(t, database, "Zweisprachig")
	store := domain.NewStore(database)
	if err := store.UpdateSettings(context.Background(), ws.ID, domain.Settings{
		Locale: "de", TimeZone: "Europe/Berlin", ExtraLocales: extras,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	fresh, err := store.GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return fresh
}

// seedPageIn is seedPage with a language and an optional page it translates.
func seedPageIn(t *testing.T, database *db.DB, websiteID int64, title, slug, body, status, loc string, of int64) *page.Page {
	t.Helper()
	html, err := page.RenderMarkdown(body)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	store := page.NewStore(database)
	p, err := store.CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug,
		Markdown: body, HTML: html, Status: status,
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if loc != "" || of != 0 {
		if err := store.SetTranslation(context.Background(), websiteID, p.ID, loc, of); err != nil {
			t.Fatalf("SetTranslation: %v", err)
		}
	}
	return p
}

// throughMiddleware runs a request the way the server does: the language prefix
// is taken off before anything is routed.
func throughMiddleware(h func(http.ResponseWriter, *http.Request) error, ws *domain.Website, target, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", target, nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))

	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The mux would set this from the route pattern; the middleware has
		// already rewritten the path it is taken from.
		if slug != "" {
			r.SetPathValue("slug", strings.TrimPrefix(r.URL.Path, "/"))
		}
		_ = h(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

func TestMainLanguageKeepsItsAddresses(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)

	rec := throughMiddleware(h.HandlePage, ws, "/kontakt", "kontakt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 — switching a language on must not move an existing address", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Kontakt") {
		t.Errorf("German page not served:\n%s", rec.Body.String())
	}
}

func TestSecondLanguageServesItsOwnPage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Contact", "contact", "Appelez-nous", "published", "fr", de.ID)

	rec := throughMiddleware(h.HandlePage, ws, "/fr/contact", "contact")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Appelez-nous") {
		t.Errorf("French page not served:\n%s", body)
	}
}

// The rule that makes a missing translation honest rather than confusing.
func TestUntranslatedAddressIs404InThatLanguage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)

	rec := throughMiddleware(h.HandlePage, ws, "/fr/kontakt", "kontakt")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404 — /fr/kontakt must not fall back to the German page", rec.Code)
	}
}

// A prefix the website does not have is an ordinary address, not a language.
func TestUnknownPrefixIsNotALanguage(t *testing.T) {
	_, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/it/kontakt", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	var seen string
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path + "|" + LocaleFrom(r.Context())
	})).ServeHTTP(rec, req)

	if seen != "/it/kontakt|" {
		t.Errorf("path/locale = %q; want %q — /it is a page on this site, not a language", seen, "/it/kontakt|")
	}
}

func TestSwitcherOffersOnlyRealTranslations(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr it")
	de := seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Contact", "contact", "Appelez-nous", "published", "fr", de.ID)
	// Italian exists as a language of the site but not as a page.

	req := httptest.NewRequest("GET", "/kontakt", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	links := h.translationLinks(req, ws, de)

	if len(links) != 2 {
		t.Fatalf("got %d links; want 2 (German and French) — Italian has no page and must not be offered: %+v", len(links), links)
	}
	var urls []string
	for _, l := range links {
		urls = append(urls, l.URL)
	}
	want := map[string]bool{"/kontakt": true, "/fr/contact": true}
	for _, u := range urls {
		if !want[u] {
			t.Errorf("unexpected switcher target %q; want one of %v", u, want)
		}
	}
}

// A draft translation is not a translation as far as a visitor is concerned.
func TestSwitcherHidesDraftTranslations(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Contact", "contact", "Appelez-nous", "draft", "fr", de.ID)

	req := httptest.NewRequest("GET", "/kontakt", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	if links := h.translationLinks(req, ws, de); len(links) != 0 {
		t.Errorf("got %+v; want none — the French page is still a draft", links)
	}
}

// A website with one language must behave exactly as it did before languages
// existed: no switcher, no prefix, nothing to notice.
func TestSingleLanguageSiteIsUntouched(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Einsprachig")
	seedPage(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published")

	req := httptest.NewRequest("GET", "/kontakt", nil)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	if got := h.languageHomes(req, ws); got != nil {
		t.Errorf("languageHomes = %+v; want nil on a site with one language", got)
	}

	rec := throughMiddleware(h.HandlePage, ws, "/kontakt", "kontakt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
}

func TestCanonicalCarriesTheLanguagePrefix(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)
	fr := seedPageIn(t, database, ws.ID, "Contact", "contact", "Appelez-nous", "published", "fr", de.ID)

	req := httptest.NewRequest("GET", "/contact", nil)
	req.Host = "demo.test"
	req = req.WithContext(context.WithValue(
		domain.WebsiteToContext(req.Context(), ws), localeKey{}, "fr"))

	site := h.siteData(req, ws)
	meta := h.metaIn(req, ws, site, fr, "/"+fr.Slug)
	if !strings.HasSuffix(meta.CanonicalURL, "/fr/contact") {
		t.Errorf("canonical = %q; want it to end in /fr/contact — otherwise both languages claim one address",
			meta.CanonicalURL)
	}
	if site.Locale != "fr" {
		t.Errorf("Site.Locale = %q; want fr — <html lang> must be the language being served", site.Locale)
	}
	if site.FeedURL != "/fr/feed.xml" {
		t.Errorf("Site.FeedURL = %q; want /fr/feed.xml", site.FeedURL)
	}
}

// The language a visitor is answered in belongs to the page, not to their
// browser.
//
// This is the regression PUB-01 names. i18n.Middleware sits on the admin
// routes only, so before this every public request carried no language at all
// and i18n.Lang fell back to i18n.Source — which is how the contact form came
// to answer a German visitor "The e-mail address does not look right." The
// admin's question (what does this operator read?) is the wrong one out here:
// a French page is French to everybody.
func TestAPublicRequestCarriesThePagesLanguageAndNotTheBrowsers(t *testing.T) {
	_, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")

	for _, c := range []struct {
		name, path, want string
	}{
		{"main language of a multilingual site", "/anything", "de"},
		{"second language behind its prefix", "/fr/anything", "fr"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got string
			req := httptest.NewRequest("GET", c.path, nil)
			req.Host = "demo.test"
			// A browser that asks for something else entirely. It must not win.
			req.Header.Set("Accept-Language", "it-IT,it;q=0.9")
			req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
			LocaleMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = i18n.Lang(r.Context())
			})).ServeHTTP(httptest.NewRecorder(), req)
			if got != c.want {
				t.Fatalf("language of the request = %q, want %q", got, c.want)
			}
		})
	}
}

// A site with one language is the common case and it has a language too.
//
// The middleware used to return early here, before it set anything, because
// there was no prefix to strip. That early return was the whole bug for every
// single-language site: nothing to strip is not the same as nothing to say.
func TestASingleLanguageSiteAlsoSaysWhichLanguageItIs(t *testing.T) {
	_, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Einsprachig")
	store := domain.NewStore(database)
	if err := store.UpdateSettings(context.Background(), ws.ID, domain.Settings{
		Locale: "de", TimeZone: "Europe/Berlin",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	fresh, err := store.GetWebsite(context.Background(), ws.ID)
	if err != nil || fresh == nil {
		t.Fatalf("GetWebsite: %v", err)
	}

	var got string
	req := httptest.NewRequest("GET", "/hofladen", nil)
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), fresh))
	LocaleMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = i18n.Lang(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), req)

	if got != "de" {
		t.Fatalf("language of the request = %q, want \"de\"", got)
	}
}
