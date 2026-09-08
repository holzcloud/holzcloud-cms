package block

import (
	"strings"
	"testing"
)

// `darstellung` is a closed vocabulary and `setBlockField` says so in as many
// words: "checked rather than trusted: the value arrives from a form and is
// written into a column every archive carries".
//
// It was checked on the form path and nowhere else. The archive path copies
// `Display` straight out of the manifest, and Clean — which validates Type,
// Fields and Items — never looked at it. So an administrator uploading a
// bundle could store any string in the column, and it round-tripped out again
// on the next export.
//
// This is not an injection: DisplayClass maps by equality to a literal, so an
// unknown value renders as no modifier at all and never reaches the markup.
// It is the other thing .planning/GLOSSARY.md's closing rule is about — a
// stored value the code does not know. The check belongs where every writer
// meets, which is here and not in each of them.
func TestCleanKeepsOnlyTheDisplayVocabulary(t *testing.T) {
	set := Set{}

	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{DisplaySlideshow, DisplaySlideshow},
		{`" onload=alert(1) x="`, ""},
		{"diashow ", ""},
		{"DIASHOW", ""},
		{"raster", ""},
	} {
		got := set.Clean([]Block{{
			Type: TypeGallery, Display: tc.in,
			Items: []Item{{MediaID: 7}},
		}})
		if len(got) != 1 {
			t.Fatalf("%q: the block was dropped entirely", tc.in)
		}
		if got[0].Display != tc.want {
			t.Errorf("Display %q survived Clean as %q, want %q — an archive can "+
				"store a value the code does not know", tc.in, got[0].Display, tc.want)
		}
	}

	// And the block that has no business carrying one at all.
	got := set.Clean([]Block{{
		Type: TypeCards, Display: DisplaySlideshow,
		Items: []Item{{MediaID: 7}},
	}})
	if len(got) == 1 && got[0].Display != "" {
		t.Errorf("a card row kept Display %q; only a gallery has a display mode", got[0].Display)
	}
}

// The guard against fixing this the lazy way: the encoded JSON must not carry
// the unknown value either, because that is the byte sequence an export writes.
func TestAnUnknownDisplayIsNotEncoded(t *testing.T) {
	set := Set{}
	cleaned := set.Clean([]Block{{
		Type: TypeGallery, Display: `" onload=alert(1) x="`,
		Items: []Item{{MediaID: 7}},
	}})
	raw, err := Encode(cleaned, set)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(raw, "onload") {
		t.Errorf("the encoded blocks carry the unknown display value:\n  %s", raw)
	}
}
