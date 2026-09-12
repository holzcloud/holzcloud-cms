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
// über das Formular, back beim Neuzeichnen. Was der Speicher behält, nützt
// niemandem, wenn das Formular es beim nächsten Öffnen nicht wieder anzeigt —
// dann trägt man es jedes Mal neu ein und merkt erst am Ergebnis, dass es
// nicht ankam.
func TestFelddefinitionTraegtDieVierEigenschaften(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	// Eine Auswahl als Knopfreihe. Die beiden Grenzen stehen an einem
	// Bereichsfeld weiter unten und nicht hier: validate leert eine
	// Eigenschaft, die zur gewählten Art nicht passt.
	serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{
			"beschriftung": {"Farbe"},
			"art":          {field.KindChoice},
			"auswahl":      {"hell\ndunkel"},
			"darstellung":  {"knopfreihe"},
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
		t.Errorf("the choice is not a row of buttons: display = %q", defs[0].Display)
	}

	// Ein Bereichsfeld mit beiden Grenzen.
	serve(t, h, sm, h.HandleFieldSave, postForm(
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{
			"beschriftung": {"Menge"},
			"art":          {field.KindRange},
			"min_wert":     {"1"},
			"max_wert":     {"9"},
			"gilt_fuer":    {"beides"},
		},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)}))

	defs, err = fields.List(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	var bereich field.Def
	for _, d := range defs {
		if d.Kind == field.KindRange {
			bereich = d
		}
	}
	if bereich.RangeMin != "1" || bereich.RangeMax != "9" {
		t.Errorf("Grenzen = %q/%q, wollte \"1\"/\"9\"", bereich.RangeMin, bereich.RangeMax)
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
		t.Errorf("maximum = %d, wanted 2", mehrfach.MaxValues)
	}

	// Das Neuzeichnen: das Formular muss die gespeicherten Werte wieder
	// zeigen, sonst trägt man sie beim nächsten Ändern versehentlich aus.
	req := postForm("/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder", url.Values{},
		map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	req.Method = "GET"
	q := req.URL.Query()
	q.Set("aendern", strconv.FormatInt(bereich.ID, 10))
	req.URL.RawQuery = q.Encode()
	rec := serve(t, h, sm, h.HandleFieldList, req)
	html := rec.Body.String()

	for _, name := range []string{"darstellung", "max_werte", "min_wert", "max_wert"} {
		if !strings.Contains(html, `name="`+name+`"`) {
			t.Errorf("the form has no box %q", name)
		}
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
			t.Errorf("%s does not read %q again when redrawn", will.name, will.wert)
		}
	}

	// Und dasselbe für die Darstellung, an dem Feld, dem sie gehört.
	reqAuswahl := postForm("/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder",
		url.Values{}, map[string]string{"id": strconv.FormatInt(ws.ID, 10)})
	reqAuswahl.Method = "GET"
	qa := reqAuswahl.URL.Query()
	qa.Set("aendern", strconv.FormatInt(defs[0].ID, 10))
	reqAuswahl.URL.RawQuery = qa.Encode()
	if html := serve(t, h, sm, h.HandleFieldList, reqAuswahl).Body.String(); !strings.Contains(
		html, `value="knopfreihe" selected`) {
		t.Error("the stored display is not selected when redrawn")
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
		t.Fatalf("nothing was reported — the refusal would be invisible: %+v", meldung)
	}
	if !strings.Contains(meldung.Error, "limit") {
		t.Errorf("die Meldung nennt die Grenze nicht: %q", meldung.Error)
	}
}
