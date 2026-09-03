package admin

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// seedBild legt eine Bilddatei samt Eintrag an, so wie ein Upload es täte.
func seedBild(t *testing.T, h *Handler, websiteID int64, name string, w, hgt int) *media.Media {
	t.Helper()
	dir := media.WebsiteDir(h.cfg.DataDir, websiteID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, w, hgt))
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{120, 160, 90, 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	m, err := h.mediaStore.Create(context.Background(), websiteID, name, name, "image/jpeg", 1000, "hash-"+name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := h.mediaStore.SaveVariants(context.Background(), m.ID, w, hgt, nil); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}
	m, _ = h.mediaStore.GetByID(context.Background(), m.ID)
	return m
}

func cropRequest(t *testing.T, m *media.Media, values url.Values) *http.Request {
	t.Helper()
	return postForm("/admin/websites/1/media/1/zuschnitt", values, map[string]string{
		"id": strconv.FormatInt(m.WebsiteID, 10), "mediaID": strconv.FormatInt(m.ID, 10),
	})
}

// Der ganze Weg: zuschneiden, Datei auf der Platte, Eintrag in der Datenbank.
func TestZuschnittSchneidetUndMerktEsSich(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
		"form":           {"1-1"},
		"naehe":          {"100"},
		"drehung":        {"0"},
		"fokus_x":        {"10"},
		"fokus_y":        {"70"},
		"gezeigt_breite": {"0"},
	}))

	nach, err := h.mediaStore.GetByID(context.Background(), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if nach.Width != nach.Height {
		t.Errorf("kein Quadrat: %d × %d", nach.Width, nach.Height)
	}
	if nach.Width != 600 {
		t.Errorf("das Quadrat ist %d breit, want 600", nach.Width)
	}
	if nach.Crop.Ratio != "1-1" || nach.Crop.FocusX != 10 || nach.Crop.FocusY != 70 {
		t.Errorf("die Entscheidung wurde nicht gemerkt: %+v", nach.Crop)
	}
	if !nach.IsCropped() {
		t.Error("das Bild gilt nicht als zugeschnitten")
	}

	// Und das Original liegt daneben, unangetastet.
	dir := media.WebsiteDir(h.cfg.DataDir, ws.ID)
	ow, oh, err := media.Dimensions(filepath.Join(dir, media.SourceName("weide.jpg")))
	if err != nil {
		t.Fatalf("das Original fehlt: %v", err)
	}
	if ow != 1600 || oh != 600 {
		t.Errorf("das Original ist %d × %d", ow, oh)
	}
}

// Der Klick aufs Bild ist der eigentliche Bedienweg. Der Browser schickt ihn in
// Pixeln der Anzeige; daraus muss ein Prozentwert werden.
func TestKlickAufsBildSetztDenFokus(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
		"form":           {""},
		"fokus_x":        {"50"},
		"fokus_y":        {"50"},
		"gezeigt_breite": {"640"},
		"gezeigt_hoehe":  {"240"},
		// Ein Klick auf ein Viertel der Breite und drei Viertel der Höhe.
		"fokus.x": {"160"},
		"fokus.y": {"180"},
	}))

	nach, _ := h.mediaStore.GetByID(context.Background(), m.ID)
	if nach.Crop.FocusX != 25 || nach.Crop.FocusY != 75 {
		t.Errorf("Fokus = %d/%d, want 25/75", nach.Crop.FocusX, nach.Crop.FocusY)
	}
}

// Über die Tastatur schickt ein <input type="image"> 0,0 — nicht zu
// unterscheiden von einem Klick in die äusserste Ecke. Das darf niemandem das
// Motiv nach links oben schieben, nur weil er die Eingabetaste gedrückt hat.
func TestTastaturVerschiebtDasMotivNicht(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
		"form":           {""},
		"fokus_x":        {"30"},
		"fokus_y":        {"60"},
		"gezeigt_breite": {"640"},
		"gezeigt_hoehe":  {"240"},
		"fokus.x":        {"0"},
		"fokus.y":        {"0"},
	}))

	nach, _ := h.mediaStore.GetByID(context.Background(), m.ID)
	if nach.Crop.FocusX != 30 || nach.Crop.FocusY != 60 {
		t.Errorf("Fokus = %d/%d — die getippten Werte wurden überschrieben",
			nach.Crop.FocusX, nach.Crop.FocusY)
	}
}

// Ein zweiter Zuschnitt beginnt wieder beim Original. Sonst verlöre ein Bild
// bei jeder Meinungsänderung an Fläche und an Qualität.
func TestZweiterZuschnittGehtWiederVomOriginalAus(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
		"form": {"1-1"}, "naehe": {"200"}, "fokus_x": {"50"}, "fokus_y": {"50"},
	}))
	eng, _ := h.mediaStore.GetByID(context.Background(), m.ID)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, eng, url.Values{
		"form": {""}, "naehe": {"100"}, "fokus_x": {"50"}, "fokus_y": {"50"},
	}))
	weit, _ := h.mediaStore.GetByID(context.Background(), m.ID)

	if weit.Width != 1600 || weit.Height != 600 {
		t.Errorf("der zweite Zuschnitt kam nur auf %d × %d", weit.Width, weit.Height)
	}
	if weit.Width <= eng.Width {
		t.Errorf("der zweite Zuschnitt ist nicht grösser als der erste (%d vs %d)",
			weit.Width, eng.Width)
	}
}

// Zurücksetzen holt das hochgeladene Bild zurück — der Punkt fürs Motiv bleibt,
// denn der sagt, wo etwas ist, und das stimmt auch beim ganzen Bild.
func TestZuruecksetzenBehaeltDenFokus(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, m, url.Values{
		"form": {"1-1"}, "fokus_x": {"15"}, "fokus_y": {"80"},
	}))
	zugeschnitten, _ := h.mediaStore.GetByID(context.Background(), m.ID)

	serve(t, h, sm, h.HandleMediaCropSave, cropRequest(t, zugeschnitten, url.Values{
		"zuruecksetzen": {"1"},
	}))
	nach, _ := h.mediaStore.GetByID(context.Background(), m.ID)

	if nach.Width != 1600 || nach.Height != 600 {
		t.Errorf("nicht wiederhergestellt: %d × %d", nach.Width, nach.Height)
	}
	if nach.IsCropped() {
		t.Errorf("gilt noch als zugeschnitten: %+v", nach.Crop)
	}
	if nach.Crop.FocusX != 15 || nach.Crop.FocusY != 80 {
		t.Errorf("der Fokus ging verloren: %d/%d", nach.Crop.FocusX, nach.Crop.FocusY)
	}
}

// Der Bildschirm zeigt das Bild in einer festen Breite, weil der Klick in
// Pixeln dieser Anzeige zurückkommt — und muss die passende Höhe mitschicken.
func TestZuschnittbildschirmNenntSeineAnzeigegroesse(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m := seedBild(t, h, ws.ID, "weide.jpg", 1600, 600)

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/media/1/zuschnitt", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("mediaID", strconv.FormatInt(m.ID, 10))
	body := serve(t, h, sm, h.HandleMediaCrop, req).Body.String()

	// Die Angabe muss zur gezeichneten Grösse passen, sonst rechnet der Server
	// den Klick falsch um — genau das ist einmal passiert, weil das Stylesheet
	// das Bild verkleinert hat.
	breite := strconv.Itoa(previewWidth)
	if !strings.Contains(body, `name="gezeigt_breite" value="`+breite+`"`) {
		t.Errorf("die Anzeigebreite fehlt:\n%s", body)
	}
	if !strings.Contains(body, `width="`+breite+`"`) {
		t.Errorf("das Bild wird nicht in der angegebenen Breite gezeichnet:\n%s", body)
	}
	hoehe := strconv.Itoa(600 * previewWidth / 1600)
	if !strings.Contains(body, `name="gezeigt_hoehe" value="`+hoehe+`"`) {
		t.Errorf("die Anzeigehöhe stimmt nicht:\n%s", body)
	}
	if !strings.Contains(body, `type="image"`) {
		t.Errorf("das anklickbare Bild fehlt:\n%s", body)
	}
}

// Was sich nicht dekodieren lässt, bekommt eine Erklärung statt Bedienelemente,
// die beim Absenden scheitern würden.
func TestNichtZuschneidbaresErklaertSichStattZuScheitern(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	m, err := h.mediaStore.Create(context.Background(), ws.ID,
		"plan.pdf", "plan.pdf", "application/pdf", 1000, "hash-pdf")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/media/1/zuschnitt", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("mediaID", strconv.FormatInt(m.ID, 10))
	body := serve(t, h, sm, h.HandleMediaCrop, req).Body.String()

	if !strings.Contains(body, "lässt sich nicht zuschneiden") {
		t.Errorf("keine Erklärung:\n%s", body)
	}
	if strings.Contains(body, `type="image"`) {
		t.Errorf("es werden trotzdem Bedienelemente angeboten:\n%s", body)
	}
}
