package public

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
)

// The composed "Do Not Sell or Share" page, end to end: the text of
// internal/privacy, saved the way the editor saves a page (page.RenderMarkdown),
// published, and served through the same middleware as a visitor's request.
// What arrives must be the widget of optout.go where the composition put its
// marker, in the page's language.

// dnsPageSlug is the slug the composition gives the page in every language.
const dnsPageSlug = "do-not-sell-or-share"

// dnsPageFeatures is a website with a shop that takes online payment, so the
// composition names the payment provider.
var dnsPageFeatures = privacy.Features{Shop: true, Payment: true}

// dnsPageSeed composes the page in lang for the website, renders it as the
// save path does and stores it published. A page in another language than the
// website's main one is filed as a translation of main.
func dnsPageSeed(t *testing.T, database *db.DB, ws *dnsPageSite, lang string, main int64) (*page.Page, string) {
	t.Helper()
	md := privacy.ComposeDNS(privacy.Request{Lang: lang, Set: ws.set, Features: dnsPageFeatures})
	if md == "" {
		t.Fatalf("%s: the composition is empty", lang)
	}
	html, err := page.RenderMarkdown(md)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	create := page.PageCreate{
		WebsiteID: ws.id,
		Title:     privacy.Title(privacy.KindDNS, lang),
		Slug:      privacy.Slug(privacy.KindDNS, lang),
		Markdown:  md,
		HTML:      html,
		Status:    "published",
	}
	if main != 0 {
		create.Locale = lang
		create.TranslationOf = main
	}
	p, err := page.NewStore(database).CreatePage(context.Background(), create)
	if err != nil {
		t.Fatalf("CreatePage %s: %v", lang, err)
	}
	return p, md
}

// dnsPageSite is what dnsPageSeed needs of a website.
type dnsPageSite struct {
	id  int64
	set legal.Set
}

// dnsPageHeading is the text of the composition's first heading, the glance
// block's, as a visitor reads it.
func dnsPageHeading(t *testing.T, md string) string {
	t.Helper()
	first, _, _ := strings.Cut(md, "\n")
	heading, ok := strings.CutPrefix(first, "## ")
	if !ok {
		t.Fatalf("the composition does not open with a ## heading: %q", first)
	}
	return heading
}

func TestTheComposedDoNotSellPageShowsTheWidget(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,us", "en")
	if ws.Locale != "de" {
		t.Fatalf("the test website's main language is %q; the test assumes de", ws.Locale)
	}
	site := &dnsPageSite{id: ws.ID, set: ws.Frameworks()}
	main, deMD := dnsPageSeed(t, database, site, "de", 0)
	_, enMD := dnsPageSeed(t, database, site, "en", main.ID)

	for _, tc := range []struct {
		lang, target, md string
	}{
		{"de", "/" + dnsPageSlug, deMD},
		{"en", "/en/" + dnsPageSlug, enMD},
	} {
		honoured := htmlText(i18n.T(tc.lang, optoutHonoured))
		notSent := htmlText(i18n.T(tc.lang, optoutNotSent))
		if tc.lang == "de" && (honoured == htmlText(optoutHonoured) || notSent == htmlText(optoutNotSent)) {
			t.Fatal("the German catalogue lacks the status sentences")
		}

		with := optoutGet(t, h, ws, tc.target, optoutGPC("1"))
		body := with.Body.String()
		for what, want := range map[string]string{
			"the widget":               optoutWidgetMark,
			"the honoured status":      honoured,
			"the glance heading":       htmlText(dnsPageHeading(t, tc.md)),
			"the template notice":      htmlText(privacy.TemplateNotice(tc.lang)),
			"the payment provider":     "Payrexx",
			"the no-legal-advice line": htmlText(privacy.NoLegalAdvice(tc.lang)),
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s with Sec-GPC: 1: %s is missing (%q)", tc.lang, what, want)
			}
		}
		if strings.Contains(body, notSent) {
			t.Errorf("%s with Sec-GPC: 1: the not-sent status is shown too", tc.lang)
		}
		if strings.Contains(body, optoutMarker) {
			t.Errorf("%s: the marker reached the visitor", tc.lang)
		}
		if n := strings.Count(body, optoutWidgetMark); n != 1 {
			t.Errorf("%s: %d widgets, want 1", tc.lang, n)
		}

		without := optoutGet(t, h, ws, tc.target, nil)
		plain := without.Body.String()
		if !strings.Contains(plain, notSent) || strings.Contains(plain, honoured) {
			t.Errorf("%s without the header: want only the not-sent status", tc.lang)
		}
		if !strings.Contains(plain, optoutWidgetMark) {
			t.Errorf("%s without the header: no form", tc.lang)
		}

		for name, rec := range map[string]*httptest.ResponseRecorder{"with": with, "without": without} {
			if !optoutVaryNames(rec)["sec-gpc"] {
				t.Errorf("%s %s the header: Vary = %v, want Sec-GPC named", tc.lang, name, rec.Header().Values("Vary"))
			}
		}
	}
}

func TestTheComposedPageWithoutUSShowsNoForm(t *testing.T) {
	h, database, ws := optoutSite(t, "eu,ch", "")
	// The page was composed while the website still had US; then US was
	// taken away. The text stays, the form does not.
	site := &dnsPageSite{id: ws.ID, set: legal.ParseSet("eu,us")}
	_, md := dnsPageSeed(t, database, site, "de", 0)

	body := optoutGet(t, h, ws, "/"+dnsPageSlug, optoutGPC("1")).Body.String()
	if !strings.Contains(body, htmlText(dnsPageHeading(t, md))) || !strings.Contains(body, "Payrexx") {
		t.Errorf("the composed text is missing")
	}
	for _, bad := range []string{optoutMarker, optoutWidgetMark, "/widerspruch"} {
		if strings.Contains(body, bad) {
			t.Errorf("a website without US carries %q", bad)
		}
	}
}
