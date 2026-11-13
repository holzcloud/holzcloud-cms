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

// A value marker is the one marker the excerpt keeps. The summary is derived
// from the page's text, and in that text the value stands in the sentence:
// "Alpha [[wert:v]]: neu" reads "Alpha 0.1.5: neu" on the page, and a summary
// that dropped the marker read "Alpha : neu" in <meta name="description">.
// The value is put in at delivery, not here: a text frozen at save would keep
// the old version after the value changed (VAL-04).
func TestExcerptKeepsValueMarkers(t *testing.T) {
	for _, tc := range []struct{ name, markdown, want string }{
		{"in a sentence, beside a snippet",
			"Alpha [[wert:v]]: neu\n\n[[snippet:fuss]]\n\n[[formular]]", "Alpha [[wert:v]]: neu"},
		{"in the visible text of a link",
			"[Version [[wert:v]]](/release) ist da.", "Version [[wert:v]] ist da."},
		{"in the alt text of a picture, which goes with the picture",
			"Vorher ![Logo [[wert:v]]](/logo.png) nachher", "Vorher nachher"},
		{"in a link address, which goes with the address",
			"[Release](https://x.ch/v[[wert:v]]) lesen", "Release lesen"},
		{"a key with an underscore survives the emphasis pass",
			"Stand [[wert:a_b]] *fett*", "Stand [[wert:a_b]] fett"},
		{"in a heading and in code",
			"# Version [[wert:v]]\n\n`[[wert:v]]`", "Version [[wert:v]] [[wert:v]]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Excerpt(tc.markdown); got != tc.want {
				t.Errorf("Excerpt(%q) = %q, want %q", tc.markdown, got, tc.want)
			}
		})
	}
}

// The cut never falls inside a value marker: half a marker is neither text nor
// syntax that delivery can resolve. Without a space in the first 160
// characters the cut is mid-word, and that word may be a marker.
func TestExcerptNeverCutsAValueMarkerInHalf(t *testing.T) {
	for _, markdown := range []string{
		strings.Repeat("x", 150) + "[[wert:version-holzcloud-cms]]",
		strings.Repeat("Wort ", 30) + "[[wert:version-holzcloud-cms]] Ende",
	} {
		got := Excerpt(markdown)
		if i := strings.LastIndex(got, "[["); i >= 0 && !strings.Contains(got[i:], "]]") {
			t.Errorf("Excerpt cut a marker in half: %q", got)
		}
		if strings.Contains(got, "wert:") && !strings.Contains(got, "[[wert:version-holzcloud-cms]]") {
			t.Errorf("Excerpt left a piece of a marker: %q", got)
		}
	}
}

// A product's description is not page text: values are not resolved in it
// (D-11), so its excerpt keeps dropping every marker the way it always did.
func TestExcerptWithoutValuesDropsValueMarkers(t *testing.T) {
	got := ExcerptWithoutValues("Alpha [[wert:v]] neu [[snippet:fuss]] Ende.")
	if got != "Alpha neu Ende." {
		t.Errorf("ExcerptWithoutValues = %q, want %q", got, "Alpha neu Ende.")
	}
}
