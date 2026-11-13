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
