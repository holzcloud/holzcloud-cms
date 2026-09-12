package block

import (
	"strings"
	"testing"
)

// A film in a picture grid.
//
// The image arm at render.go has had "if !ok || img.Film { return }" since long
// before this phase, and its sibling in the gallery had only "if !ok" — so a
// gallery item pointing at an mp4 went through imgTag and the visitor got
// <img src="/media/1/film.mp4">, which is a broken image icon and nothing else.
//
// It is reachable without anybody doing anything wrong. album.Store.AddItem
// calls requireOwnMedia, which checks the website and deliberately nothing else
// — internal/admin's requireOwnPicture is where "not a film" is checked, and its
// comment says why the store correctly has no opinion. But the bundle importer
// calls AddItem directly, bypassing the handler, and resolves the picture
// through mediaByName, which can name an mp4 in the archive's media list. The
// inline gallery has the same hole through a hand-edited archive.
//
// The guard goes in the renderer rather than in the store, and that is the
// cheaper of the two the review offered as well as the wider one: it covers
// both sources, and it needs no new named error and no second opinion in SQL.

// filmInTheMiddle is the three-picture gallery with the second item pointing at
// a film. The middle one, on purpose: the numbering and the step links are what
// a naive "skip it" would get wrong.
func filmInTheMiddle() Lookup {
	return images(map[int64]Image{
		1: {URL: "/media/1/eins.jpg", Alt: "Eins", Width: 1200, Height: 800},
		2: {URL: "/media/1/film.mp4", Alt: "Film", Film: true},
		3: {URL: "/media/1/drei.jpg", Alt: "Drei", Width: 1200, Height: 800},
	})
}

// TestAFilmInAGalleryIsNotDrawnAsAPicture is the property.
func TestAFilmInAGalleryIsNotDrawnAsAPicture(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, filmInTheMiddle(), markdown)

	if strings.Contains(html, "film.mp4") {
		t.Errorf("a film was rendered into a picture grid:\n%s", html)
	}
	// The two real pictures still have to be there — dropping the film must not
	// drop the gallery.
	for _, want := range []string{"eins.jpg", "drei.jpg"} {
		if !strings.Contains(html, want) {
			t.Errorf("%q is missing after the film was dropped:\n%s", want, html)
		}
	}
}

// TestAFilmInAGalleryLeavesTheStepLinksWhole is the half a "skip it" would get
// wrong: the third picture's "previous" must point at the first, not at an id
// that was never written.
func TestAFilmInAGalleryLeavesTheStepLinksWhole(t *testing.T) {
	html := Render([]Block{threePictures()}, Builtin, filmInTheMiddle(), markdown)

	views := largeViews(t, html)
	if len(views) != 2 {
		t.Fatalf("%d large views, want 2 — the film was counted as a picture", len(views))
	}
	if strings.Contains(views[0], "hc-galerie__zurueck") {
		t.Errorf("the first picture has a previous:\n%s", views[0])
	}
	if !strings.Contains(views[0], `class="hc-galerie__weiter" href="#hc-b1-p3"`) {
		t.Errorf("the first picture's next does not step over the film:\n%s", views[0])
	}
	if !strings.Contains(views[1], `class="hc-galerie__zurueck" href="#hc-b1-p1"`) {
		t.Errorf("the third picture's previous does not step back over the film:\n%s", views[1])
	}
	// The ids stay keyed to the item's own index, so the third picture is still
	// p3 — the same rule that already holds for a media id that does not
	// resolve at all.
	if !strings.Contains(html, `id="hc-b1-p3"`) || strings.Contains(html, `id="hc-b1-p2"`) {
		t.Errorf("the film renumbered the pictures after it:\n%s", html)
	}
}

// TestAGalleryOfNothingButFilmsRendersNothing is the boundary: no picture is no
// gallery, which is what a gallery whose media ids do not resolve already does.
func TestAGalleryOfNothingButFilmsRendersNothing(t *testing.T) {
	look := images(map[int64]Image{
		1: {URL: "/media/1/a.mp4", Film: true},
		2: {URL: "/media/1/b.mp4", Film: true},
		3: {URL: "/media/1/c.mp4", Film: true},
	})
	if html := Render([]Block{threePictures()}, Builtin, look, markdown); html != "" {
		t.Errorf("a gallery of nothing but films rendered %q", html)
	}
}
