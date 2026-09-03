package web

import (
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// Die Adresse ist die Quelle. Wenn das schiefgeht, zeigt die Seitenleiste die
// Abschnitte der falschen Website an — und zwar plausibel genug, dass es
// jemandem erst auffällt, wenn er auf der falschen Seite etwas geändert hat.
func TestWebsiteAusDerAdresse(t *testing.T) {
	fälle := []struct {
		pfad string
		will int64
	}{
		{"/admin/websites/2/pages", 2},
		{"/admin/websites/17", 17},
		{"/admin/websites/2/pages/5/edit", 2},
		{"/admin/websites", 0},
		{"/admin/websites/neu", 0},
		{"/admin/users", 0},
		{"/admin/", 0},
		// Kein Weg, über die Adresse etwas anderes als eine Zahl einzuschleusen.
		{"/admin/websites/2x/pages", 0},
		{"/admin/websites/-1/pages", 0},
	}
	for _, f := range fälle {
		if got := websiteFromPath(f.pfad); got != f.will {
			t.Errorf("websiteFromPath(%q) = %d, want %d", f.pfad, got, f.will)
		}
	}
}

// Ohne Treffer die erste Website: besser als gar kein Menü, und wer nur eine
// Website hat — der Regelfall — merkt von der Auswahl nie etwas.
func TestAuswahlFaelltAufDieErsteZurueck(t *testing.T) {
	liste := websitesMit(3, 7, 9)

	if ws := pick(liste, 7); ws == nil || ws.ID != 7 {
		t.Errorf("pick(7) = %v, want 7", ws)
	}
	if ws := pick(liste, 0); ws == nil || ws.ID != 3 {
		t.Errorf("pick(0) = %v, want 3", ws)
	}
	// Eine Website, die es nicht mehr gibt — etwa gerade gelöscht, während sie
	// noch in der Sitzung steht.
	if ws := pick(liste, 999); ws == nil || ws.ID != 3 {
		t.Errorf("pick(999) = %v, want 3", ws)
	}
	if ws := pick(nil, 5); ws != nil {
		t.Errorf("pick auf leerer Liste = %v, want nil", ws)
	}
}

// pick liefert einen Zeiger in die Liste. Zeigten alle Aufrufe auf dieselbe
// Schleifenvariable, bekäme jede Anfrage die zuletzt gesehene Website.
func TestAuswahlZeigtAufDieRichtigeWebsite(t *testing.T) {
	liste := websitesMit(1, 2, 3)
	a, b := pick(liste, 1), pick(liste, 3)
	if a.ID != 1 || b.ID != 3 {
		t.Errorf("pick lieferte %d und %d, want 1 und 3", a.ID, b.ID)
	}
}

func websitesMit(ids ...int64) []domain.Website {
	out := make([]domain.Website, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.Website{ID: id})
	}
	return out
}
