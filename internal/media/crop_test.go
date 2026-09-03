package media

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// Ein Bild mit einem erkennbaren Punkt: so lässt sich prüfen, was der Zuschnitt
// tatsächlich behalten hat, statt nur die Masse zu zählen.
func testBild(t *testing.T, pfad string, w, h int, punkt image.Point) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	// Ein 20×20 großer roter Fleck an der angegebenen Stelle.
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

// Das Rechteck hat die gewählte Form und ist so gross, wie es hineinpasst.
func TestZuschnittHatDieGewaehlteForm(t *testing.T) {
	c := Crop{Ratio: "1-1", Zoom: 100, FocusX: 50, FocusY: 50}
	r := c.Rect(1600, 900)
	if r.Dx() != r.Dy() {
		t.Errorf("kein Quadrat: %dx%d", r.Dx(), r.Dy())
	}
	if r.Dy() != 900 {
		t.Errorf("das Quadrat nutzt die Höhe nicht aus: %d", r.Dy())
	}

	r = Crop{Ratio: "16-9", Zoom: 100, FocusX: 50, FocusY: 50}.Rect(1000, 1000)
	if r.Dx() != 1000 || r.Dy() != 562 {
		t.Errorf("16:9 aus einem Quadrat = %dx%d", r.Dx(), r.Dy())
	}
}

// Der Fokuspunkt zieht das Rechteck zu sich — aber nie über den Rand hinaus.
// Ein Zuschnitt, der überstünde, müsste mit irgendetwas gefüllt werden, und es
// gibt nichts, womit sich das ehrlich füllen liesse.
func TestZuschnittFolgtDemFokusUndBleibtImBild(t *testing.T) {
	c := Crop{Ratio: "1-1", Zoom: 100, FocusX: 10, FocusY: 50}
	r := c.Rect(1600, 900)
	if r.Min.X != 0 {
		t.Errorf("links wurde nicht am Rand angehalten: %+v", r)
	}

	c.FocusX = 90
	r = c.Rect(1600, 900)
	if r.Max.X != 1600 {
		t.Errorf("rechts wurde nicht am Rand angehalten: %+v", r)
	}

	c.FocusX = 50
	r = c.Rect(1600, 900)
	if r.Min.X != (1600-900)/2 {
		t.Errorf("in der Mitte sitzt es nicht mittig: %+v", r)
	}
}

// Werte aus einem Formular dürfen nie zu einem Bild von null Pixeln führen.
func TestUnsinnigeWerteWerdenGebaendigt(t *testing.T) {
	c := Crop{Rotation: 37, Ratio: "gibtsnicht", Zoom: -5, FocusX: -20, FocusY: 500}.Normalise()
	if c.Rotation != 0 || c.Ratio != "" || c.Zoom != 100 || c.FocusX != 0 || c.FocusY != 100 {
		t.Errorf("got %+v", c)
	}
	if r := (Crop{Zoom: 100000}).Rect(100, 100); r.Dx() < 1 || r.Dy() < 1 {
		t.Errorf("ein Rechteck ohne Fläche: %+v", r)
	}
}

// Eine Vierteldrehung verschiebt Pixel, sie rechnet sie nicht neu — die Masse
// tauschen einfach die Plätze.
func TestDrehungTauschtBreiteUndHoehe(t *testing.T) {
	dir := t.TempDir()
	pfad := filepath.Join(dir, "bild.jpg")
	testBild(t, pfad, 400, 200, image.Pt(200, 100))

	w, h, err := ApplyCrop(dir, "bild.jpg", "image/jpeg", Crop{Rotation: 90}, 24)
	if err != nil {
		t.Fatalf("ApplyCrop: %v", err)
	}
	if w != 200 || h != 400 {
		t.Errorf("nach der Drehung %dx%d, want 200x400", w, h)
	}
}

// Der hochgeladene Zustand bleibt erhalten, und ein zweiter Zuschnitt setzt
// nicht auf dem ersten auf — sonst verlöre ein Bild bei jeder Meinungsänderung
// an Qualität.
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
		t.Fatalf("das Original wurde nicht beiseite gelegt: %v", err)
	}
	ow, oh, _ := Dimensions(original)
	if ow != 1200 || oh != 800 {
		t.Errorf("das beiseite gelegte Bild ist nicht das Original: %dx%d", ow, oh)
	}

	// Ein zweiter, weiterer Zuschnitt muss wieder gross werden können. Ginge er
	// vom Ergebnis des ersten aus, wäre er höchstens so gross wie dieses.
	w, h, err := ApplyCrop(dir, "bild.jpg", "image/jpeg", Crop{}, 24)
	if err != nil {
		t.Fatalf("zweiter Zuschnitt: %v", err)
	}
	if w != 1200 || h != 800 {
		t.Errorf("der zweite Zuschnitt kam nicht ans Original heran: %dx%d", w, h)
	}
}

// Zurücksetzen stellt das hochgeladene Bild wieder her und räumt die Kopie weg.
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

// Und die Probe aufs Ganze: was der Fokus zeigt, bleibt drin; was weit davon
// weg liegt, fliegt raus.
func TestDerFokusEntscheidetWasImBildBleibt(t *testing.T) {
	dir := t.TempDir()
	// Der rote Fleck sitzt ganz links.
	testBild(t, filepath.Join(dir, "links.jpg"), 1200, 400, image.Pt(80, 200))

	if _, _, err := ApplyCrop(dir, "links.jpg", "image/jpeg",
		Crop{Ratio: "1-1", FocusX: 5, FocusY: 50}, 24); err != nil {
		t.Fatal(err)
	}
	if !hatRot(t, filepath.Join(dir, "links.jpg")) {
		t.Error("der Fokus lag auf dem Fleck, und er ist trotzdem weg")
	}

	// Derselbe Fleck, aber der Fokus zeigt nach rechts.
	testBild(t, filepath.Join(dir, "rechts.jpg"), 1200, 400, image.Pt(80, 200))
	if _, _, err := ApplyCrop(dir, "rechts.jpg", "image/jpeg",
		Crop{Ratio: "1-1", FocusX: 95, FocusY: 50}, 24); err != nil {
		t.Fatal(err)
	}
	if hatRot(t, filepath.Join(dir, "rechts.jpg")) {
		t.Error("der Fokus lag weit rechts, der Fleck links ist trotzdem drin")
	}
}

// Ein Bild, das auf seiner eigenen Mitte liegt, bekommt kein Attribut: das ist
// ohnehin, was ein Browser tut, und eine Angabe, die nichts ändert, steht sonst
// auf jeder Seite.
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
