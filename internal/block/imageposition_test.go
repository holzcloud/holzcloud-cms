package block

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// The image position of an image-text block.
//
// It is the block's existing Variant, so a block stored before the position
// existed has "" or "rechts" in it and has to render the markup it always did:
// that markup is frozen into pages.content_html and every archive, and the
// first test below is the contract for it.

func imageTextBlock(variant string) Block {
	return Block{
		Type: TypeImageText, MediaID: 1, Markdown: "Hallo", Variant: variant,
	}
}

func imageTextLookup() Lookup {
	return images(map[int64]Image{
		1: {URL: "/media/1/a.jpg", Alt: "A", Width: 1200, Height: 800},
	})
}

func renderImageText(variant string) string {
	return Render([]Block{imageTextBlock(variant)}, Builtin, imageTextLookup(), markdown)
}

func TestTheOldImageTextMarkupIsUnchanged(t *testing.T) {
	left := renderImageText("")
	if !strings.HasPrefix(left, `<div class="hc-block hc-bildtext">`) {
		t.Errorf("an empty position no longer renders the bare wrapper:\n%s", left)
	}
	right := renderImageText("rechts")
	if !strings.HasPrefix(right, `<div class="hc-block hc-bildtext hc-bildtext--rechts">`) {
		t.Errorf("rechts no longer renders its modifier:\n%s", right)
	}
	// The inner markup is the same in every case.
	inner := func(s, open string) string { return strings.TrimPrefix(s, open) }
	if inner(left, `<div class="hc-block hc-bildtext">`) !=
		inner(right, `<div class="hc-block hc-bildtext hc-bildtext--rechts">`) {
		t.Errorf("the inner markup differs between left and right")
	}
}

func TestEveryImagePositionHasItsClassAndKeepsTheSourceOrder(t *testing.T) {
	for _, tc := range []struct{ variant, class string }{
		{"", ""},
		{"links", ""},
		{ImageRight, "hc-bildtext--rechts"},
		{ImageAbove, "hc-bildtext--oben"},
		{ImageBelow, "hc-bildtext--unten"},
	} {
		html := renderImageText(tc.variant)
		if tc.class == "" && strings.Contains(html, "hc-bildtext--") {
			t.Errorf("%q gained a modifier it should not have:\n%s", tc.variant, html)
		}
		if tc.class != "" && !strings.Contains(html, tc.class) {
			t.Errorf("%q does not render %s:\n%s", tc.variant, tc.class, html)
		}
		img := strings.Index(html, "hc-bildtext__bild")
		txt := strings.Index(html, "hc-bildtext__text")
		if img < 0 || txt < 0 || img > txt {
			t.Errorf("%q: the image must precede the text in the source (image %d, text %d)",
				tc.variant, img, txt)
		}
	}
}

func TestCleanKeepsOnlyTheImagePositions(t *testing.T) {
	set := Set{}
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"links", ""},
		{ImageRight, ImageRight},
		{ImageAbove, ImageAbove},
		{ImageBelow, ImageBelow},
		{"OBEN", ""},
		{"unten ", ""},
		{"voll", ""},
	} {
		got := set.Clean([]Block{imageTextBlock(tc.in)})
		if len(got) != 1 {
			t.Fatalf("%q: the block was dropped", tc.in)
		}
		if got[0].Variant != tc.want {
			t.Errorf("Variant %q survived Clean as %q, want %q", tc.in, got[0].Variant, tc.want)
		}
	}

	// Other block types keep their own vocabulary.
	img := set.Clean([]Block{{Type: TypeImage, MediaID: 1, Variant: "voll"}})
	if len(img) != 1 || img[0].Variant != "voll" {
		t.Errorf("an image block lost its Variant: %+v", img)
	}
	gal := set.Clean([]Block{{Type: TypeGallery, Variant: "4", Items: []Item{{MediaID: 1}}}})
	if len(gal) != 1 || gal[0].Variant != "4" {
		t.Errorf("a gallery lost its column count: %+v", gal)
	}
}

func TestAnUnknownImagePositionIsNotEncoded(t *testing.T) {
	set := Set{}
	cleaned := set.Clean([]Block{imageTextBlock(`" onload=alert(1) x="`)})
	raw, err := Encode(cleaned, set)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(raw, "onload") {
		t.Errorf("the encoded blocks carry the unknown position:\n  %s", raw)
	}
}

func TestAHandEditedPositionAddsNoClass(t *testing.T) {
	html := renderImageText(`rechts" onmouseover="alert(1)`)
	if strings.Contains(html, "onmouseover") || strings.Contains(html, "hc-bildtext--") {
		t.Errorf("a hand-edited Variant reached the markup:\n%s", html)
	}
}

func TestThePositionIsReadFromTheForm(t *testing.T) {
	blocks := FromForm(url.Values{
		"b0.typ":      {"bildtext"},
		"b0.medium":   {"1"},
		"b0.markdown": {"Hallo"},
		"b0.variante": {"unten"},
	})
	if len(blocks) != 1 || blocks[0].Variant != "unten" {
		t.Errorf("b0.variante=unten did not become Variant unten: %+v", blocks)
	}
}

// The stylesheets are the only place the modifiers mean anything, so a
// modifier a stylesheet does not name is a position that silently looks like
// left. Every shipped theme links bausteine.css (spec_test.go holds that), so
// bausteine.css plus the three themes with a layout of their own cover all
// eight.
func TestTheStylesheetsNameEveryPositionModifier(t *testing.T) {
	for _, path := range []string{
		"../../cmd/holzcloud/assets/bausteine.css",
		"../../cmd/holzcloud/templates/public/holzcloud/style.css",
		"../../cmd/holzcloud/templates/public/rudel/style.css",
		"../../cmd/holzcloud/templates/public/weide/style.css",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, want := range []string{"hc-bildtext--oben", "hc-bildtext--unten"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s does not name %s", path, want)
			}
		}
	}
}
