package page

import (
	"context"
	"strings"
	"testing"
)

// Raw is the passage before it became HTML, so a caller that has to change the
// text first — the public search puts the website's values in — can do so and
// then escape it the one way this package escapes it.
func TestSearchResultCarriesTheRawPassage(t *testing.T) {
	s, ws := newTestStore(t)
	seed(t, s, ws, "Leistungen", "leistungen", "Wir bauen <b>Treppen</b> aus Massivholz.", "published")

	results, err := s.SearchPages(context.Background(), ws, "Treppen", false, 20)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	raw := results[0].Raw
	if !strings.Contains(raw, markStart+"Treppen"+markEnd) {
		t.Errorf("Raw = %q; want the passage with its sentinels around the term", raw)
	}
	if !strings.Contains(raw, "<b>") {
		t.Errorf("Raw = %q; want the passage as it stands, unescaped", raw)
	}
	if got := Highlight(raw); got != results[0].Snippet {
		t.Errorf("Highlight(Raw) = %q; want the result's own Snippet %q", got, results[0].Snippet)
	}
}

func TestHighlightEscapesBeforeItMarks(t *testing.T) {
	got := string(Highlight("<b>" + markStart + "x" + markEnd + "</b>"))
	if want := "&lt;b&gt;<mark>x</mark>&lt;/b&gt;"; got != want {
		t.Errorf("Highlight = %q; want %q", got, want)
	}
}
