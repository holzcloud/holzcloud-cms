package audit

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
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
