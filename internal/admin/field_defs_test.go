package admin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Die vier neuen Eigenschaften auf dem Bildschirm für Felddefinitionen: hin
// über das Formular, zurück beim Neuzeichnen. Was der Speicher behält, nützt
// niemandem, wenn das Formular es beim nächsten Öffnen nicht wieder anzeigt —
// dann trägt man es jedes Mal neu ein und merkt erst am Ergebnis, dass es
// nicht ankam.
func TestFelddefinitionTraegtDieVierEigenschaften(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	// Eine Auswahl als Knopfreihe, mit beiden Grenzen.
	serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{
			"beschriftung": {"Farbe"},
			"art":          {field.KindChoice},
			"auswahl":      {"hell\ndunkel"},
			"darstellung":  {"knopfreihe"},
			"min_wert":     {"1"},
			"max_wert":     {"9"},
			"gilt_fuer":    {"beides"},
		},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))

	defs, err := fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("%d Felder angelegt, wollte 1", len(defs))
	}
	if !defs[0].IsButtonRow() {
		t.Errorf("die Auswahl ist keine Knopfreihe: Darstellung = %q", defs[0].Display)
	}
	if defs[0].RangeMin != "1" || defs[0].RangeMax != "9" {
		t.Errorf("Grenzen = %q/%q, wollte \"1\"/\"9\"", defs[0].RangeMin, defs[0].RangeMax)
	}

	// Und eine Mehrfachauswahl mit einer Höchstzahl.
	serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{
			"beschriftung": {"Hölzer"},
			"art":          {field.KindMulti},
			"auswahl":      {"Eiche\nBuche"},
			"max_werte":    {"2"},
			"gilt_fuer":    {"beides"},
		},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))

	defs, err = fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	var mehrfach field.Def
	for _, d := range defs {
		if d.Kind == field.KindMulti {
			mehrfach = d
		}
	}
	if mehrfach.MaxValues != 2 {
		t.Errorf("Höchstzahl = %d, wollte 2", mehrfach.MaxValues)
	}

	// Das Neuzeichnen: das Formular muss die gespeicherten Werte wieder
	// zeigen, sonst trägt man sie beim nächsten Ändern versehentlich aus.
	req := postForm("/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder", url.Values{},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	req.Method = "GET"
	q := req.URL.Query()
	q.Set("aendern", strconv.FormatInt(defs[0].ID, 10))
	req.URL.RawQuery = q.Encode()
	rec := serve(t, h, sm, h.HandleFieldList, req)
	html := rec.Body.String()

	for _, name := range []string{"darstellung", "max_werte", "min_wert", "max_wert"} {
		if !strings.Contains(html, `name="`+name+`"`) {
			t.Errorf("das Formular hat kein Kästchen %q", name)
		}
	}
	if !strings.Contains(html, `value="knopfreihe" selected`) {
		t.Error("die gespeicherte Darstellung ist beim Neuzeichnen nicht gewählt")
	}
	// Die beiden Grenzen stehen wieder in ihren Kästchen. Gemessen wird der
	// Ausschnitt um das jeweilige Kästchen herum, nicht die ganze Seite: ein
	// value="1" irgendwo sonst im Dokument wäre kein Beweis.
	for _, will := range []struct{ name, wert string }{{"min_wert", "1"}, {"max_wert", "9"}} {
		at := strings.Index(html, `name="`+will.name+`"`)
		if at < 0 {
			continue // schon oben gemeldet
		}
		ende := at + 400
		if ende > len(html) {
			ende = len(html)
		}
		if !strings.Contains(html[at:ende], `value="`+will.wert+`"`) {
			t.Errorf("%s steht beim Neuzeichnen nicht wieder auf %q", will.name, will.wert)
		}
	}
}

// Ein verdrehtes Grenzenpaar wird abgelehnt, und die Ablehnung ist zu lesen:
// eine stillschweigend nicht gespeicherte Definition ist der Fall, den
// FIELD-05 ausdrücklich ausschliesst.
func TestVerdrehteGrenzenWerdenGemeldetUndNichtGespeichert(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	var meldung web.Flash
	rec := serve(t, h, sm, func(w http.ResponseWriter, r *http.Request) error {
		if err := h.HandleFieldSave(w, r); err != nil {
			return err
		}
		meldung = web.GetFlash(sm, r.Context())
		return nil
	}, postForm(
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{
			"beschriftung": {"Spannweite"},
			"art":          {field.KindNumber},
			"min_wert":     {"10"},
			"max_wert":     {"2"},
			"gilt_fuer":    {"beides"},
		},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))
	_ = rec

	defs, err := fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 0 {
		t.Fatalf("die Definition wurde trotz verdrehter Grenzen gespeichert: %+v", defs)
	}
	if meldung.Error == "" {
		t.Fatalf("es wurde nichts gemeldet — die Ablehnung wäre unsichtbar: %+v", meldung)
	}
	if !strings.Contains(meldung.Error, "Grenze") {
		t.Errorf("die Meldung nennt die Grenze nicht: %q", meldung.Error)
	}
}
