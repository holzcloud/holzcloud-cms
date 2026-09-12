package media

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// An image with a recognisable spot: that way it can be checked what the crop
// actually kept, rather than only counting the dimensions.
func testBild(t *testing.T, pfad string, w, h int, punkt image.Point) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	// A 20×20 red spot in the given place.
	for y := punkt.Y - 10; y < punkt.Y+10; y++ {
		for x := punkt.X - 10; x < punkt.X+10; x++ {
			if x >= 0 && y >= 0 && x < w && y < h {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			}
		}
	}
	f, err := os.Create(pfad)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
}

func hatRot(t *testing.T, pfad string) bool {
	t.Helper()
	f, err := os.Open(pfad)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 > 180 && g>>8 < 90 && bl>>8 < 90 {
				return true
			}
		}
	}
	return false
}

// The rectangle has the chosen shape and is as large as fits inside.
func TestZuschnittHatDieGewaehlteForm(t *testing.T) {
	c := Crop{Ratio: "1-1", Zoom: 100, FocusX: 50, FocusY: 50}
	r := c.Rect(1600, 900)
	if r.Dx() != r.Dy() {
		t.Errorf("kein Quadrat: %dx%d", r.Dx(), r.Dy())
	}
	if r.Dy() != 900 {
		t.Errorf("the square does not use the height: %d", r.Dy())
	}

	r = Crop{Ratio: "16-9", Zoom: 100, FocusX: 50, FocusY: 50}.Rect(1000, 1000)
	if r.Dx() != 1000 || r.Dy() != 562 {
		t.Errorf("16:9 out of a square = %dx%d", r.Dx(), r.Dy())
	}
}

// The focus point pulls the rectangle towards itself — but never past the edge.
// A crop that stood out would have to be filled with something, and there is
// nothing it could honestly be filled with.
func TestZuschnittFolgtDemFokusUndBleibtImBild(t *testing.T) {
	c := Crop{Ratio: "1-1", Zoom: 100, FocusX: 10, FocusY: 50}
	r := c.Rect(1600, 900)
	if r.Min.X != 0 {
		t.Errorf("it was not stopped at the left edge: %+v", r)
	}

	c.FocusX = 90
	r = c.Rect(1600, 900)
	if r.Max.X != 1600 {
		t.Errorf("it was not stopped at the right edge: %+v", r)
	}

	c.FocusX = 50
	r = c.Rect(1600, 900)
	if r.Min.X != (1600-900)/2 {
		t.Errorf("in the middle it does not sit centred: %+v", r)
	}
}

// Values out of a form must never lead to an image of zero pixels.
func TestUnsinnigeWerteWerdenGebaendigt(t *testing.T) {
	c := Crop{Rotation: 37, Ratio: "gibtsnicht", Zoom: -5, FocusX: -20, FocusY: 500}.Normalise()
	if c.Rotation != 0 || c.Ratio != "" || c.Zoom != 100 || c.FocusX != 0 || c.FocusY != 100 {
		t.Errorf("got %+v", c)
	}
	if r := (Crop{Zoom: 100000}).Rect(100, 100); r.Dx() < 1 || r.Dy() < 1 {
		t.Errorf("a rectangle with no area: %+v", r)
	}
}

// A quarter turn moves pixels, it does not recompute them — the dimensions
// simply swap places.
func TestDrehungTauschtBreiteUndHoehe(t *testing.T) {
	dir := t.TempDir()
	pfad := filepath.Join(dir, "bild.jpg")
	testBild(t, pfad, 400, 200, image.Pt(200, 100))

	w, h, err := ApplyCrop(dir, "bild.jpg", "image/jpeg", Crop{Rotation: 90}, 24)
	if err != nil {
		t.Fatalf("ApplyCrop: %v", err)
	}
	if w != 200 || h != 400 {
		t.Errorf("after the rotation %dx%d, want 200x400", w, h)
	}
}

// The uploaded state is kept, and a second crop does not build on the first —
// otherwise an image would lose quality every time somebody changed their mind.
func TestZweiterZuschnittBeginntWiederBeimOriginal(t *testing.T) {
	dir := t.TempDir()
	pfad := filepath.Join(dir, "bild.jpg")
	testBild(t, pfad, 1200, 800, image.Pt(600, 400))

	if _, _, err := ApplyCrop(dir, "bild.jpg", "image/jpeg",
		Crop{Ratio: "1-1", Zoom: 200, FocusX: 50, FocusY: 50}, 24); err != nil {
		t.Fatalf("erster Zuschnitt: %v", err)
	}
	original := filepath.Join(dir, SourceName("bild.jpg"))
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("the original was not set aside: %v", err)
	}
	ow, oh, _ := Dimensions(original)
	if ow != 1200 || oh != 800 {
		t.Errorf("the image set aside is not the original: %dx%d", ow, oh)
	}

	// A second, wider crop has to be able to become large again. If it started
	// from the result of the first, it could at most be as large as that one.
	w, h, err := ApplyCrop(dir, "bild.jpg", "image/jpeg", Crop{}, 24)
	if err != nil {
		t.Fatalf("zweiter Zuschnitt: %v", err)
	}
	if w != 1200 || h != 800 {
		t.Errorf("the second crop did not reach the original: %dx%d", w, h)
	}
}

// Resetting restores the uploaded image and clears the copy away.
func TestZuruecksetzenStelltDasOriginalWiederHer(t *testing.T) {
	dir := t.TempDir()
	testBild(t, filepath.Join(dir, "bild.jpg"), 1000, 500, image.Pt(500, 250))

	if _, _, err := ApplyCrop(dir, "bild.jpg", "image/jpeg",
		Crop{Ratio: "1-1", FocusX: 50, FocusY: 50}, 24); err != nil {
		t.Fatal(err)
	}
	w, h, err := RestoreOriginal(dir, "bild.jpg")
	if err != nil {
		t.Fatalf("RestoreOriginal: %v", err)
	}
	if w != 1000 || h != 500 {
		t.Errorf("wiederhergestellt als %dx%d, want 1000x500", w, h)
	}
	if _, err := os.Stat(filepath.Join(dir, SourceName("bild.jpg"))); err == nil {
		t.Error("die Kopie liegt noch da")
	}
}

// And the proof of the whole thing: what the focus points at stays in; what
// lies far away from it flies out.
func TestDerFokusEntscheidetWasImBildBleibt(t *testing.T) {
	dir := t.TempDir()
	// Der rote Fleck sitzt ganz links.
	testBild(t, filepath.Join(dir, "links.jpg"), 1200, 400, image.Pt(80, 200))

	if _, _, err := ApplyCrop(dir, "links.jpg", "image/jpeg",
		Crop{Ratio: "1-1", FocusX: 5, FocusY: 50}, 24); err != nil {
		t.Fatal(err)
	}
	if !hatRot(t, filepath.Join(dir, "links.jpg")) {
		t.Error("the focus was on the spot and it is gone regardless")
	}

	// The same spot, but the focus points to the right.
	testBild(t, filepath.Join(dir, "rechts.jpg"), 1200, 400, image.Pt(80, 200))
	if _, _, err := ApplyCrop(dir, "rechts.jpg", "image/jpeg",
		Crop{Ratio: "1-1", FocusX: 95, FocusY: 50}, 24); err != nil {
		t.Fatal(err)
	}
	if hatRot(t, filepath.Join(dir, "rechts.jpg")) {
		t.Error("the focus was far right, the spot on the left is in it regardless")
	}
}

// An image that lies on its own centre gets no attribute: that is what a browser
// does anyway, and a statement that changes nothing would otherwise stand on
// every page.
func TestFokusCSSNurWennErEtwasAendert(t *testing.T) {
	m := Media{Crop: Crop{FocusX: 50, FocusY: 50}}
	if got := m.FocusCSS(); got != "" {
		t.Errorf("Mitte ergibt %q", got)
	}
	m.Crop.FocusX = 20
	if got := m.FocusCSS(); got != "20% 50%" {
		t.Errorf("got %q, want \"20%% 50%%\"", got)
	}
}
