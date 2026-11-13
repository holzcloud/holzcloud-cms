package public

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// The values of a website, read the way a visitor reads them: a page served
// through the same middleware the server uses, its body searched for the text.

const versionPage = "Alpha [[wert:version-holzcloud-cms]]: neu\n\n" +
	"## Version [[wert:version-holzcloud-cms]]\n\n" +
	"- Punkt [[wert:version-holzcloud-cms]]\n"

// valuesFixture is one website with one published page that names the version
// in a sentence, a heading and a list item, and a handler that has the store.
func valuesFixture(t *testing.T) (*Handler, *db.DB, *domain.Website, *value.Store) {
	t.Helper()
	h, database := newTestHandler(t)
	store := value.NewStore(database)
	h.SetValueStore(store)
	ws := seedWebsite(t, database, "Werte")
	seedPage(t, database, ws.ID, "Stand", "stand", versionPage, "published")
	return h, database, ws, store
}

func setValue(t *testing.T, store *value.Store, websiteID int64, key, text string) {
	t.Helper()
	if _, _, err := store.Set(context.Background(), websiteID, key, text); err != nil {
		t.Fatalf("Set(%q, %q): %v", key, text, err)
	}
}

func servedBody(t *testing.T, h *Handler, ws *domain.Website, slug string) string {
	t.Helper()
	rec := throughMiddleware(h.HandlePage, ws, "/"+slug, slug)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /%s = %d; want 200", slug, rec.Code)
	}
	return rec.Body.String()
}

func TestAValueShowsInsideASentence(t *testing.T) {
	h, _, ws, store := valuesFixture(t)
	setValue(t, store, ws.ID, "version-holzcloud-cms", "0.1.5")

	body := servedBody(t, h, ws, "stand")
	for _, want := range []string{`<p>Alpha 0.1.5: neu</p>`, `Version 0.1.5</h2>`, `<li>Punkt 0.1.5</li>`} {
		if !strings.Contains(body, want) {
			t.Errorf("served page lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "[[wert:") {
		t.Errorf("served page still carries a raw marker:\n%s", body)
	}
}

func TestChangingAValueChangesThePageWithoutSavingIt(t *testing.T) {
	h, database, ws, store := valuesFixture(t)
	setValue(t, store, ws.ID, "version-holzcloud-cms", "0.1.5")
	_ = servedBody(t, h, ws, "stand")

	stamp := func() (int64, string) {
		var version int64
		var updated string
		if err := database.Read.QueryRow(
			`SELECT version, updated_at FROM pages WHERE website_id = $1 AND slug = 'stand'`, ws.ID).
			Scan(&version, &updated); err != nil {
			t.Fatalf("read page stamp: %v", err)
		}
		return version, updated
	}
	vBefore, uBefore := stamp()

	setValue(t, store, ws.ID, "version-holzcloud-cms", "0.1.6")
	body := servedBody(t, h, ws, "stand")
	if !strings.Contains(body, `<p>Alpha 0.1.6: neu</p>`) || strings.Contains(body, "0.1.5") {
		t.Errorf("the next response does not carry the changed value:\n%s", body)
	}

	vAfter, uAfter := stamp()
	if vAfter != vBefore || uAfter != uBefore {
		t.Errorf("the page was touched: version %d→%d, updated_at %q→%q; a value change must not save the page (VAL-04)",
			vBefore, vAfter, uBefore, uAfter)
	}
}

func TestChangingAValueMovesThePagesLastModified(t *testing.T) {
	h, database, ws, store := valuesFixture(t)
	setValue(t, store, ws.ID, "version-holzcloud-cms", "0.1.5")

	before := plainGet(t, h, ws, "stand").Header().Get("Last-Modified")

	setValue(t, store, ws.ID, "version-holzcloud-cms", "0.1.6")
	moved := "2031-04-05T06:07:08Z"
	if _, err := database.Write.Exec(
		`UPDATE site_values SET updated_at = $1 WHERE website_id = $2`, moved, ws.ID); err != nil {
		t.Fatalf("move the value's stamp: %v", err)
	}

	rec := conditionalGet(t, h, ws, "stand", "", before)
	if rec.Code != http.StatusOK {
		t.Fatalf("a browser holding only the date got %d; want 200 — it would keep the old version", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "0.1.6") {
		t.Errorf("the answer lacks the new value:\n%s", rec.Body.String())
	}
	want, err := time.Parse("2006-01-02T15:04:05Z", moved)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rec.Header().Get("Last-Modified"); got != want.UTC().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q; want the value's own stamp %q", got, want.UTC().Format(http.TimeFormat))
	}
}

func TestAValueOfAnotherWebsiteIsNeverUsed(t *testing.T) {
	h, database := newTestHandler(t)
	store := value.NewStore(database)
	h.SetValueStore(store)
	a := seedWebsite(t, database, "A")
	b := seedWebsite(t, database, "B")
	seedPage(t, database, b.ID, "Stand", "stand", "Alpha [[wert:v]]: neu\n\nNur A: [[wert:nur-a]].", "published")

	setValue(t, store, a.ID, "v", "von-a")
	setValue(t, store, a.ID, "nur-a", "geheim")
	setValue(t, store, b.ID, "v", "von-b")

	body := servedBody(t, h, b, "stand")
	if !strings.Contains(body, `<p>Alpha von-b: neu</p>`) {
		t.Errorf("B's page does not carry B's text:\n%s", body)
	}
	if strings.Contains(body, "von-a") || strings.Contains(body, "geheim") {
		t.Errorf("B's page carries A's value (D-03):\n%s", body)
	}
	if !strings.Contains(body, `<p>Nur A: .</p>`) {
		t.Errorf("a key only A defines must render as nothing on B:\n%s", body)
	}
}

// linkPages are one Markdown page with a Markdown link, a bare autolink and a
// raw <a>, and one block page whose call to action links to the same kind of
// address — every way an editor can put a value into a link address.
const linkMarkdown = "[Release](https://x.ch/tag/v[[wert:v]])\n\n" +
	"Auto https://x.ch/auto/v[[wert:v]] dort.\n\n" +
	"Roh <a href=\"https://x.ch/raw/v[[wert:v]]\">hier</a>.\n"

func seedLinkPages(t *testing.T, database *db.DB, websiteID int64) {
	t.Helper()
	seedPage(t, database, websiteID, "Links", "links", linkMarkdown, "published")

	blocks := []block.Block{{
		Type: block.TypeCallout, Title: "Neu",
		LinkText: "Herunterladen", LinkURL: "https://x.ch/block/v[[wert:v]]",
	}}
	encoded, err := block.Encode(blocks, block.Builtin)
	if err != nil {
		t.Fatalf("block.Encode: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: "Block", Slug: "block", Status: "published",
		Blocks: encoded,
		HTML:   block.Render(blocks, block.Builtin, nil, page.RenderMarkdown),
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
}

// addressesOn returns every href on a served page that points at x.ch.
func addressesOn(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	z := xhtml.NewTokenizer(strings.NewReader(body))
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			return out
		}
		if tt != xhtml.StartTagToken {
			continue
		}
		for _, a := range z.Token().Attr {
			if a.Key == "href" && strings.Contains(a.Val, "x.ch") {
				out = append(out, a.Val)
			}
		}
	}
}

func TestAValueReachesEveryKindOfLinkAddress(t *testing.T) {
	h, database := newTestHandler(t)
	store := value.NewStore(database)
	h.SetValueStore(store)
	ws := seedWebsite(t, database, "Links")
	seedLinkPages(t, database, ws.ID)
	setValue(t, store, ws.ID, "v", "0.1.5")

	for slug, want := range map[string]int{"links": 3, "block": 1} {
		body := servedBody(t, h, ws, slug)
		hrefs := addressesOn(t, body)
		if len(hrefs) != want {
			t.Errorf("/%s: %d addresses %v; want %d", slug, len(hrefs), hrefs, want)
		}
		for _, href := range hrefs {
			if !strings.HasSuffix(href, "/v0.1.5") {
				t.Errorf("/%s: href %q does not carry the value", slug, href)
			}
		}
		if strings.Contains(body, "[[wert:") || strings.Contains(body, "%5B%5Bwert:") {
			t.Errorf("/%s: a marker reached the visitor:\n%s", slug, body)
		}
	}
}

func TestAJavascriptValueStaysInert(t *testing.T) {
	h, database := newTestHandler(t)
	store := value.NewStore(database)
	h.SetValueStore(store)
	ws := seedWebsite(t, database, "Links")
	seedLinkPages(t, database, ws.ID)
	setValue(t, store, ws.ID, "v", "javascript:alert(1)")

	for _, slug := range []string{"links", "block"} {
		body := servedBody(t, h, ws, slug)
		z := xhtml.NewTokenizer(strings.NewReader(body))
		for {
			tt := z.Next()
			if tt == xhtml.ErrorToken {
				break
			}
			if tt != xhtml.StartTagToken {
				continue
			}
			for _, a := range z.Token().Attr {
				if a.Key == "href" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Val)), "javascript:") {
					t.Errorf("/%s: href %q starts with javascript:", slug, a.Val)
				}
			}
		}
	}
}

func TestAnUnknownKeyRendersAsNothing(t *testing.T) {
	h, database := newTestHandler(t)
	h.SetValueStore(value.NewStore(database))
	ws := seedWebsite(t, database, "Werte")
	seedPage(t, database, ws.ID, "Stand", "stand", "Alpha [[wert:gibt-es-nicht]]: neu", "published")

	body := servedBody(t, h, ws, "stand")
	if !strings.Contains(body, `<p>Alpha : neu</p>`) {
		t.Errorf("an undefined key must render as nothing:\n%s", body)
	}
}

func TestAMissingValueStoreRendersNothingNotTheMarker(t *testing.T) {
	h, database := newTestHandler(t)
	h.SetValueStore(nil)
	ws := seedWebsite(t, database, "Werte")
	seedPage(t, database, ws.ID, "Stand", "stand", "Alpha [[wert:v]]: neu", "published")

	body := servedBody(t, h, ws, "stand")
	if strings.Contains(body, "[[wert:") {
		t.Errorf("without a store the marker is printed:\n%s", body)
	}
	if !strings.Contains(body, `<p>Alpha : neu</p>`) {
		t.Errorf("without a store every marker must render as nothing:\n%s", body)
	}
}
