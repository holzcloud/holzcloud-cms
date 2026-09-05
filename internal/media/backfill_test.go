package media

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Ein eingespieltes Bild bekommt seine Masse nachgetragen.
//
// Der Import legt die Zeile ohne Breite und Höhe an. Ohne die beiden Zahlen
// steht im HTML kein width/height und kein srcset: die Seite springt beim
// Nachladen, und ein Handy lädt das Original in voller Grösse.
func TestBackfillTraegtMasseUndFassungenNach(t *testing.T) {
	s, ws := newTestStore(t)
	daten := t.TempDir()
	dir := filepath.Join(daten, "media", strconv.FormatInt(ws, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testBild(t, filepath.Join(dir, "hof.png"), 1600, 900, image.Point{X: 800, Y: 450})

	// So, wie der Import sie anlegt: ohne Masse.
	m, err := s.Create(context.Background(), ws, "hof.png", "hof.png", "image/png", 1234, "abc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.Width != 0 || m.Height != 0 {
		t.Fatalf("die Vorbedingung stimmt nicht: Create setzt jetzt %dx%d", m.Width, m.Height)
	}

	done, failed, err := Backfill(context.Background(), s, daten, 40, 100)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if done != 1 || failed != 0 {
		t.Fatalf("ergaenzt=%d fehlgeschlagen=%d; wollte 1 und 0", done, failed)
	}

	nach, err := s.GetByID(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if nach.Width != 1600 || nach.Height != 900 {
		t.Errorf("Masse = %dx%d; wollte 1600x900", nach.Width, nach.Height)
	}
	if v, err := s.VariantsFor(context.Background(), m.ID); err != nil || len(v) == 0 {
		t.Errorf("keine verkleinerten Fassungen: %v, %v", v, err)
	}

	// Ein zweiter Lauf findet nichts mehr — sonst liefe die Arbeit ewig im Kreis.
	done, _, err = Backfill(context.Background(), s, daten, 40, 100)
	if err != nil || done != 0 {
		t.Errorf("zweiter Lauf: ergaenzt=%d, err=%v; wollte 0", done, err)
	}
}

// Was keine Masse haben kann, darf den Lauf nicht bei jedem Takt beschäftigen.
func TestBackfillLaesstEinPDFLiegen(t *testing.T) {
	s, ws := newTestStore(t)
	daten := t.TempDir()
	if _, err := s.Create(context.Background(), ws, "preise.pdf", "preise.pdf",
		"application/pdf", 99, "def"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	done, failed, err := Backfill(context.Background(), s, daten, 40, 100)
	if err != nil || done != 0 || failed != 0 {
		t.Errorf("ergaenzt=%d fehlgeschlagen=%d err=%v; ein PDF wird uebergangen", done, failed, err)
	}
}
