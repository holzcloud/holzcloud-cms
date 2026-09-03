package media

import (
	"strings"
	"testing"
)

func testIndex() Index {
	return Index{
		"/media/1/abc-photo.jpg": {
			WebsiteID: 1, Filename: "abc-photo.jpg",
			Width: 2000, Height: 1000, AltText: "Die Werkstatt",
			Variants: []Variant{
				{Label: "thumb", Filename: "abc-photo-thumb.jpg", Width: 400},
				{Label: "medium", Filename: "abc-photo-medium.jpg", Width: 800},
			},
		},
		"/media/1/logo.svg": {WebsiteID: 1, Filename: "logo.svg", Width: 0},
	}
}

func TestMakeResponsiveAddsSrcSet(t *testing.T) {
	in := `<p><img src="/media/1/abc-photo.jpg" alt="Die Werkstatt"/></p>`
	out := MakeResponsive(in, testIndex())

	for _, want := range []string{
		`abc-photo-thumb.jpg 400w`,
		`abc-photo-medium.jpg 800w`,
		`/media/1/abc-photo.jpg 2000w`,
		`sizes="` + DefaultSizes + `"`,
		`width="2000"`, `height="1000"`,
		`loading="lazy"`, `decoding="async"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	// The author's own alt text must survive untouched.
	if strings.Count(out, `alt="`) != 1 {
		t.Errorf("alt attribute was duplicated:\n%s", out)
	}
}

func TestMakeResponsiveFillsMissingAltText(t *testing.T) {
	out := MakeResponsive(`<img src="/media/1/abc-photo.jpg">`, testIndex())
	if !strings.Contains(out, `alt="Die Werkstatt"`) {
		t.Errorf("the stored description was not used:\n%s", out)
	}
}

func TestMakeResponsiveLeavesForeignAndUnknownImagesAlone(t *testing.T) {
	cases := map[string]string{
		"another site's media": `<img src="/media/9/abc-photo.jpg">`,
		"a file with no size":  `<img src="/media/1/logo.svg">`,
		"not media at all":     `<img src="/t/theme/hero.png">`,
	}
	for name, in := range cases {
		if out := MakeResponsive(in, testIndex()); out != in {
			t.Errorf("%s was rewritten:\n got %s\nwant %s", name, out, in)
		}
	}
}

func TestMakeResponsiveKeepsAuthoredAttributes(t *testing.T) {
	in := `<img src="/media/1/abc-photo.jpg" srcset="/media/1/eigen.jpg 500w" alt="x">`
	if out := MakeResponsive(in, testIndex()); out != in {
		t.Errorf("a hand-written srcset was overridden:\n got %s\nwant %s", out, in)
	}

	// An author who sized the image knows the layout better than the default.
	in = `<img src="/media/1/abc-photo.jpg" alt="x" width="600" height="300">`
	out := MakeResponsive(in, testIndex())
	if !strings.Contains(out, `width="600"`) || strings.Contains(out, `width="2000"`) {
		t.Errorf("authored dimensions were replaced:\n%s", out)
	}
}

func TestMakeResponsiveKeepsSurroundingMarkup(t *testing.T) {
	in := `<h2>Werkstatt</h2>
<p>Ein Bild: <img src="/media/1/abc-photo.jpg" alt="x"> und Text danach.</p>
<ul><li><a href="/kontakt">Kontakt</a></li></ul>`
	out := MakeResponsive(in, testIndex())

	// A round trip through the parser must not lose or reorder content — this
	// is published markup, and a rewrite that mangles it is worse than no
	// rewrite at all.
	for _, want := range []string{
		"<h2>Werkstatt</h2>", "Ein Bild: ", " und Text danach.",
		`<ul><li><a href="/kontakt">Kontakt</a></li></ul>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q in:\n%s", want, out)
		}
	}
}

func TestMakeResponsiveWithoutVariantsStillHelpsLayout(t *testing.T) {
	// An image measured but never scaled — below the smallest width, or with
	// every copy dropped as pointless. Width and height alone already stop the
	// text from jumping while the page loads.
	idx := Index{"/media/1/small.jpg": {WebsiteID: 1, Filename: "small.jpg", Width: 300, Height: 200}}
	out := MakeResponsive(`<img src="/media/1/small.jpg" alt="x">`, idx)

	if !strings.Contains(out, `width="300"`) || !strings.Contains(out, `height="200"`) {
		t.Errorf("dimensions missing:\n%s", out)
	}
	if strings.Contains(out, "srcset") {
		t.Errorf("an empty srcset was emitted:\n%s", out)
	}
}

func TestMakeResponsiveIsANoOpWithoutImages(t *testing.T) {
	in := `<p>Nur Text.</p>`
	if out := MakeResponsive(in, testIndex()); out != in {
		t.Errorf("text-only content was altered: %s", out)
	}
	if out := MakeResponsive(`<img src="/media/1/abc-photo.jpg">`, nil); out != `<img src="/media/1/abc-photo.jpg">` {
		t.Error("an empty index should leave the document untouched")
	}
}
