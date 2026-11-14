package public

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
)

// seedSummaryPage creates one published page whose excerpt is derived from its
// text the way the editor derives it, and returns it.
func seedSummaryPage(t *testing.T, f *valueRoutesFixture, slug, kind, body, metaDescription string) *page.Page {
	t.Helper()
	html, err := page.RenderMarkdown(body)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	pg, err := page.NewStore(f.database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: f.ws.ID, Title: slug, Slug: slug, Markdown: body, HTML: html,
		Status: "published", Kind: kind,
		Meta: page.PageMeta{Excerpt: page.Excerpt(body), MetaDescription: metaDescription},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return pg
}

// The summary a page derives from its text carries the value, wherever it is
// handed out: <meta name="description">, og:description, the JSON-LD, the
// theme's .Page.Excerpt, the archive, the feed's <summary>, a plugin, llms.txt
// and the share-link preview.
//
// The excerpt is derived at save, and it used to drop the marker there: the
// page read "Beta 0.1.5: neu" and its description "Beta : neu". It now keeps
// the marker and the value goes in at delivery, so changing the value changes
// every summary without the page being saved again (VAL-04) — which is the
// second half of this test.
//
// "Beta 0.1.5: neu Mehr Text." stands nowhere but in a summary: the body has
// the two sentences in two paragraphs.
func TestTheDerivedSummaryCarriesTheValue(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	ctx := context.Background()
	beta := seedSummaryPage(t, f, "beta", "post", "Beta [[wert:v]]: neu\n\nMehr Text.\n\n[[snippet:fuss]]\n", "")
	if beta.Excerpt != "Beta [[wert:v]]: neu Mehr Text." {
		t.Fatalf("the stored excerpt is %q; it must keep the marker for delivery to resolve", beta.Excerpt)
	}
	archive := *f.ws
	archive.BlogBase = "aktuelles"
	share := sharelink.New([]byte("share-secret-share-secret-share!"))
	f.h.SetShareSigners(share, sharelink.New([]byte("unlock-secret-unlock-secret-unl!")))

	routes := func(t *testing.T, summary string) {
		t.Run("the page's head and its excerpt", func(t *testing.T) {
			got := servedBody(t, f.h, f.ws, "beta")
			mustShowTheValue(t, got,
				`<meta name="description" content="`+summary+`">`,
				`<meta property="og:description" content="`+summary+`">`,
				`"description":"`+summary+`"`,
				`<p class="kurz">`+summary+`</p>`)
		})
		t.Run("the archive", func(t *testing.T) {
			rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
				return f.h.HandleArchive(w, r, &archive)
			}, &archive, "GET", "/aktuelles")
			if err != nil {
				t.Fatalf("HandleArchive: %v", err)
			}
			mustShowTheValue(t, rec.Body.String(), `<p class="auszug">`+summary+`</p>`)
		})
		t.Run("the feed", func(t *testing.T) {
			rec, err := request(f.h.HandleFeed, f.ws, "GET", "/feed.xml")
			if err != nil {
				t.Fatalf("HandleFeed: %v", err)
			}
			mustShowTheValue(t, rec.Body.String(), "<summary>"+summary+"</summary>")
		})
		t.Run("a plugin", func(t *testing.T) {
			list, err := f.h.PagesForPlugin(ctx, f.ws.ID, plugin.PagesQuery{
				Op: plugin.OpPagesList, PostsOnly: true, Limit: 10,
			})
			if err != nil || len(list.Pages) != 1 {
				t.Fatalf("pages.list = %d pages, %v; want 1", len(list.Pages), err)
			}
			mustShowTheValue(t, list.Pages[0].Excerpt, summary)
			one, err := f.h.PagesForPlugin(ctx, f.ws.ID, plugin.PagesQuery{
				Op: plugin.OpPagesGet, Slug: "beta", Limit: 10,
			})
			if err != nil || len(one.Pages) != 1 {
				t.Fatalf("pages.get = %d pages, %v; want 1", len(one.Pages), err)
			}
			mustShowTheValue(t, one.Pages[0].Excerpt, summary)
			found, err := f.h.PagesForPlugin(ctx, f.ws.ID, plugin.PagesQuery{
				Op: plugin.OpPagesSearch, Query: "Mehr", Limit: 10,
			})
			if err != nil || len(found.Pages) != 1 {
				t.Fatalf("pages.search = %d pages, %v; want 1", len(found.Pages), err)
			}
			mustShowTheValue(t, found.Pages[0].Excerpt, summary)
		})
		t.Run("llms.txt", func(t *testing.T) {
			rec, err := request(f.h.HandleLLMS, f.ws, "GET", "/llms.txt")
			if err != nil {
				t.Fatalf("HandleLLMS: %v", err)
			}
			mustShowTheValue(t, rec.Body.String(), "/beta): "+summary)
		})
		t.Run("the share-link preview", func(t *testing.T) {
			token := share.Token(beta.ID, time.Now().Add(time.Hour))
			rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
				r.SetPathValue("token", token)
				return f.h.HandleShareLink(w, r)
			}, f.ws, "GET", sharelink.Path(token))
			if err != nil {
				t.Fatalf("HandleShareLink: %v", err)
			}
			mustShowTheValue(t, rec.Body.String(), `<meta name="description" content="`+summary+`">`)
		})
	}

	t.Run("as the value stands", func(t *testing.T) { routes(t, "Beta 0.1.5: neu Mehr Text.") })
	setValue(t, f.values, f.ws.ID, "v", "0.1.6")
	t.Run("after the value changed, without saving the page", func(t *testing.T) {
		routes(t, "Beta 0.1.6: neu Mehr Text.")
	})
}

// A browser that holds only the date of the archive must not be told 304 after
// a value in one of its summaries moved: the summary is part of the list.
func TestChangingAValueMovesTheArchive(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	// The fixture's footer snippet names the value too, and a snippet's value
	// already moves every view's date. Without a marker in it, only the
	// summary is left to move the archive's.
	if _, err := f.database.Write.Exec(
		`UPDATE snippets SET content_markdown = 'Stand', content_html = '<p>Stand</p>' WHERE website_id = $1`,
		f.ws.ID); err != nil {
		t.Fatalf("take the value out of the snippet: %v", err)
	}
	seedSummaryPage(t, f, "beta", "post", "Beta [[wert:v]]: neu", "")
	archive := *f.ws
	archive.BlogBase = "aktuelles"
	setValue(t, f.values, f.ws.ID, "v", "0.1.6")
	moved := "2031-04-05T06:07:08Z"
	f.moveValues(t, moved)

	rec, err := request(func(w http.ResponseWriter, r *http.Request) error {
		return f.h.HandleArchive(w, r, &archive)
	}, &archive, "GET", "/aktuelles")
	if err != nil {
		t.Fatalf("HandleArchive: %v", err)
	}
	want, err := time.Parse(time.RFC3339, moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rec.Header().Get("Last-Modified"); got != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the value's own stamp %q", got, want.UTC().Format(http.TimeFormat))
	}
}

// A page whose summary names a key the website does not have reads the
// sentence without it — and without the space the marker left behind.
func TestASummaryWithAnUnknownValueLeavesNoGap(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	seedSummaryPage(t, f, "gamma", "", "Gamma [[wert:gibtsnicht]]: neu", "")
	mustShowTheValue(t, servedBody(t, f.h, f.ws, "gamma"), `<meta name="description" content="Gamma: neu">`)
}

// The description an editor types is the editor's own text, and D-11 keeps it
// as typed: a value is resolved in page text only, and the SEO check names a
// marker in the description (value-outside-text) instead. Only the summary
// derived from the page's text — the excerpt — carries the value.
func TestATypedMetaDescriptionIsNotResolved(t *testing.T) {
	f := newValueRoutesFixture(t, valueRoutesPage)
	seedSummaryPage(t, f, "delta", "", "Delta [[wert:v]]: neu", "Getippt [[wert:v]]")
	got := servedBody(t, f.h, f.ws, "delta")
	if strings.Contains(got, `content="Getippt 0.1.5"`) {
		t.Errorf("the typed meta description was resolved; D-11 keeps it as the editor typed it:\n%.1200s", got)
	}
	if !strings.Contains(got, `<p class="kurz">Delta 0.1.5: neu</p>`) {
		t.Errorf("the derived excerpt beside it lacks the value:\n%.1200s", got)
	}
}
