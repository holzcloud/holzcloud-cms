package page

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerptStripsMarkdownSyntax(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
	}{
		"heading and paragraph": {
			"# Über uns\n\nWir bauen Möbel aus Massivholz.",
			"Über uns Wir bauen Möbel aus Massivholz.",
		},
		"link keeps its text": {
			"Siehe [unser Impressum](/impressum) für Details.",
			"Siehe unser Impressum für Details.",
		},
		"image disappears entirely": {
			"![Werkstatt](/media/1/foto.jpg)\n\nUnsere Werkstatt.",
			"Unsere Werkstatt.",
		},
		"emphasis markers go": {
			"Ein **wichtiger** und *kursiver* Hinweis.",
			"Ein wichtiger und kursiver Hinweis.",
		},
		"list becomes prose": {
			"- Möbel\n- Türen\n- Treppen",
			"Möbel Türen Treppen",
		},
		"quote marker goes": {
			"> Ein Zitat.",
			"Ein Zitat.",
		},
		"horizontal rule is dropped": {
			"Text\n\n---\n\nMehr Text",
			"Text Mehr Text",
		},
		"inline html is stripped": {
			"<p>Ein Absatz</p>",
			"Ein Absatz",
		},
		"empty stays empty":      {"", ""},
		"whitespace stays empty": {"   \n\n  ", ""},
	}

	for label, tc := range cases {
		if got := Excerpt(tc.in); got != tc.want {
			t.Errorf("%s:\n got: %q\nwant: %q", label, got, tc.want)
		}
	}
}

// A summary made of source code is worse than no summary at all.
func TestExcerptSkipsFencedCode(t *testing.T) {
	got := Excerpt("```go\nfunc main() {}\n```\n\nSo funktioniert es.")
	if strings.Contains(got, "func main") {
		t.Errorf("code block leaked into the excerpt: %q", got)
	}
	if got != "So funktioniert es." {
		t.Errorf("got %q", got)
	}
}

func TestExcerptCutsAtAWordBoundary(t *testing.T) {
	long := strings.Repeat("Massivholz ", 60)
	got := Excerpt(long)

	if utf8.RuneCountInString(got) > MaxExcerptLength+1 { // +1 for the ellipsis
		t.Errorf("excerpt is %d runes, over the %d limit: %q",
			utf8.RuneCountInString(got), MaxExcerptLength, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a truncated excerpt should say so: %q", got)
	}
	// Cutting mid-word would leave something like "Massivh…".
	trimmed := strings.TrimSuffix(got, "…")
	if !strings.HasSuffix(trimmed, "Massivholz") {
		t.Errorf("cut in the middle of a word: %q", got)
	}
}

// A marker left in the excerpt ends up in <meta name="description"> and in the
// link preview, so a search result reads "Willkommen [[snippet:zeiten]] Ende."
func TestExcerptDropsSnippetMarkers(t *testing.T) {
	got := Excerpt("# Willkommen\n\n[[snippet:oeffnungszeiten]]\n\nEnde.")
	if strings.Contains(got, "[[snippet:") {
		t.Errorf("the marker survived into the excerpt: %q", got)
	}
	if got != "Willkommen Ende." {
		t.Errorf("got %q, want %q", got, "Willkommen Ende.")
	}
}

// A marker is page syntax. It leaked into <meta name="description"> once as
// [[snippet:…]] and again as [[formular]]; this covers the shape, not one name.
func TestExcerptDropsEveryMarker(t *testing.T) {
	for _, markdown := range []string{
		"Willkommen [[snippet:zeiten]] bei uns.",
		"Willkommen [[formular]] bei uns.",
		"Willkommen [[irgendwas-neues]] bei uns.",
	} {
		got := Excerpt(markdown)
		if strings.Contains(got, "[[") || strings.Contains(got, "]]") {
			t.Errorf("Excerpt(%q) = %q, still carries a marker", markdown, got)
		}
		if !strings.Contains(got, "Willkommen") || !strings.Contains(got, "bei uns") {
			t.Errorf("Excerpt(%q) = %q, lost the prose around the marker", markdown, got)
		}
	}
}
