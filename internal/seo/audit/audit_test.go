package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func TestFlattenBlocksKeepsProseAndLinksInOrder(t *testing.T) {
	blocks := []block.Block{
		{Type: block.TypeText, Markdown: "## Kopf\n\nText"},
		{Type: block.TypeCallout, Title: "Jetzt", Markdown: "Los", LinkText: "Mehr [x]", LinkURL: "/mehr"},
		{Type: block.TypeCards, Items: []block.Item{{Title: "Karte", Markdown: "Inhalt", LinkURL: "/karte"}}},
		{Type: block.TypeQuote, Text: "Zitat"},
		{Type: block.TypeImage, MediaID: 3, Alt: "wird nicht gelesen"},
		{Type: "rezept", Fields: map[string]string{"b": "zwei", "a": "eins"}},
	}
	got := FlattenBlocks(blocks)
	for _, want := range []string{"## Kopf", "Jetzt", "[Mehr x](/mehr)", "[Karte](/karte)", "Zitat", "eins\n\nzwei"} {
		if !strings.Contains(got, want) {
			t.Errorf("flattened text lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "wird nicht gelesen") {
		t.Error("a picture's description is not prose")
	}
	if FlattenBlocks(nil) != "" {
		t.Error("no blocks, no text")
	}
}

// valuePage names one value in its Markdown and one in its blocks.
func valuePage() page.Page {
	p := page.Page{ID: 3, WebsiteID: 7, Title: "Seite", Status: "published"}
	p.ContentMarkdown = "Die Version ist [[wert:a]]."
	p.Blocks = `[{"type":"text","markdown":"Auch [[wert:b]] hier."}]`
	return p
}

func TestAuditReadsTheValuesAPageUses(t *testing.T) {
	ctx := context.Background()
	ws := &domain.Website{ID: 7, Name: "Eins"}
	p := valuePage()

	var asked []int64
	s := Source{Values: func(_ context.Context, websiteID int64) (map[string]bool, error) {
		asked = append(asked, websiteID)
		return map[string]bool{"a": true}, nil
	}}
	in := s.input(ctx, s.context(ctx, ws, nil), p)
	if strings.Join(in.UsedValues, ",") != "a,b" {
		t.Errorf("UsedValues = %v, want a and b", in.UsedValues)
	}
	// A typed excerpt and the stored HTML are resolved at delivery too, so a
	// key named only there is a key the page uses (WR-02): the HTML carries a
	// marker the Markdown wrote with escaped brackets.
	withMore := p
	withMore.Excerpt = "Kurz: [[wert:c]]"
	withMore.ContentHTML = "<p>Die Version ist [[wert:a]] [[wert:d]].</p>"
	in = s.input(ctx, s.context(ctx, ws, nil), withMore)
	if strings.Join(in.UsedValues, ",") != "a,b,d,c" {
		t.Errorf("UsedValues = %v, want a, b, d and c", in.UsedValues)
	}
	asked = asked[:1]
	if len(asked) != 1 || asked[0] != 7 {
		t.Errorf("Values was asked for %v, want the page's website 7 once", asked)
	}
	if !in.KnownValues["a"] || len(in.KnownValues) != 1 {
		t.Errorf("KnownValues = %v, want the map Values returned", in.KnownValues)
	}

	failing := Source{Values: func(context.Context, int64) (map[string]bool, error) {
		return nil, errors.New("no table")
	}}
	in = failing.input(ctx, failing.context(ctx, ws, nil), p)
	if in.KnownValues != nil {
		t.Errorf("a failing Values gives KnownValues %v, want nil (not checked)", in.KnownValues)
	}

	none := Source{}
	in = none.input(ctx, none.context(ctx, ws, nil), p)
	if in.KnownValues != nil {
		t.Errorf("no Values gives KnownValues %v, want nil (not checked)", in.KnownValues)
	}
}

func TestAuditFindsValuesOutsideTextAndBeforeAParen(t *testing.T) {
	ctx := context.Background()
	ws := &domain.Website{ID: 7, Name: "Eins"}
	s := Source{}
	c := s.context(ctx, ws, nil)

	p := valuePage()
	p.Title = "Version [[wert:v]]"
	p.MetaDescription = "Jetzt [[wert:w]] laden."
	in := s.input(ctx, c, p)
	if strings.Join(in.ValuesOutsideText, ",") != "v,w" {
		t.Errorf("ValuesOutsideText = %v, want v and w", in.ValuesOutsideText)
	}
	if len(in.ValuesBeforeParen) != 0 {
		t.Errorf("ValuesBeforeParen = %v, want none", in.ValuesBeforeParen)
	}

	for _, tc := range []struct {
		markdown, blocks, want string
	}{
		{"Version [[wert:v]](beta)", "", "v"},
		{"Version [[wert:v]] (beta)", "", ""},
		{"Version [[wert:v]](beta) und [[wert:v]](rc)", "", "v"},
		{"", `[{"type":"text","markdown":"Neu [[wert:b]](heute)"}]`, "b"},
		{"[Download [[wert:v]]](/laden)", "", ""},
	} {
		p := valuePage()
		p.ContentMarkdown, p.Blocks = tc.markdown, tc.blocks
		got := strings.Join(s.input(ctx, c, p).ValuesBeforeParen, ",")
		if got != tc.want {
			t.Errorf("%q %q: ValuesBeforeParen = %q, want %q", tc.markdown, tc.blocks, got, tc.want)
		}
	}
}

// IN-06 of the phase 33 review: in [Release](v[[wert:k]]) the colon of
// "wert:" stands in the first segment of a relative address, url.Parse reads
// it as a scheme, and the sanitiser drops the address. The audit names the key;
// an address that starts with "/", "./", "../", "#", "?" or a scheme, or that
// puts a folder first, keeps it.
func TestAuditFindsAValueThatStartsARelativeLink(t *testing.T) {
	ctx := context.Background()
	s := Source{}
	c := s.context(ctx, &domain.Website{ID: 7}, nil)
	for _, tc := range []struct{ markdown, blocks, want string }{
		{"[Release](v[[wert:k]])", "", "k"},
		{"[Release]([[wert:k]])", "", "k"},
		{"![Logo](logo-[[wert:k]].png)", "", "k"},
		{"[A](v[[wert:k]]?x=1) [B](w[[wert:j]]#top) [C](v[[wert:k]])", "", "k,j"},
		{"", `[{"type":"text","markdown":"[R](v[[wert:b]])"}]`, "b"},
		{"[Release](releases/v[[wert:k]])", "", ""},
		{"[Release](/r/v[[wert:k]])", "", ""},
		{"[Release](./v[[wert:k]])", "", ""},
		{"[Release](https://x.ch/v[[wert:k]])", "", ""},
		{"[Mail](mailto:info@[[wert:k]])", "", ""},
		{"[Release](#v[[wert:k]])", "", ""},
		{"[Release](?v=[[wert:k]])", "", ""},
		{"Alpha [[wert:k]] (beta)", "", ""},
		// IN-02 of the second review: every one of these is stored without
		// its address, checked through page.RenderMarkdown.
		{"[R](.v[[wert:k]])", "", "k"},
		{"[R](..[[wert:k]])", "", "k"},
		{"[R](<v[[wert:k]]>)", "", "k"},
		{"[R]\n\n[R]: v[[wert:k]]", "", "k"},
		{"[R]\n\n   [R]: <v[[wert:k]]> \"Titel\"", "", "k"},
		{`<a href="v[[wert:k]]">R</a>`, "", "k"},
		{`<a href='v[[wert:k]]'>R</a>`, "", "k"},
		{`<A HREF=v[[wert:k]]>R</A>`, "", "k"},
		{`<img src="v[[wert:k]].png" alt="x">`, "", "k"},
		{"", `[{"type":"text","markdown":"[R]\n\n[R]: v[[wert:b]]"}]`, "b"},
		{"", `[{"type":"text","markdown":"[R]: v[[wert:b]]"}]`, "b"},
		{"", `[{"type":"text","markdown":"<a href=\"v[[wert:b]]\">R</a>"}]`, "b"},
		// And these keep it.
		{"[R](../v[[wert:k]])", "", ""},
		{"[R]\n\n[R]: /v[[wert:k]]", "", ""},
		{`<a href="/v[[wert:k]]">R</a>`, "", ""},
		{`<a href="https://x.ch/[[wert:k]]">R</a>`, "", ""},
		{`<a data-href="v[[wert:k]]">R</a>`, "", ""},
		// In the order they stand, whichever form wrote them.
		{"<a href=\"w[[wert:j]]\">x</a> [R](v[[wert:k]])", "", "j,k"},
	} {
		p := valuePage()
		p.ContentMarkdown, p.Blocks = tc.markdown, tc.blocks
		got := strings.Join(s.input(ctx, c, p).ValuesStartingLink, ",")
		if got != tc.want {
			t.Errorf("%q %q: ValuesStartingLink = %q, want %q", tc.markdown, tc.blocks, got, tc.want)
		}
	}
}
