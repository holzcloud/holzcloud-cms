package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Der Schaltermechanismus, an dem Markup gemessen, das er braucht.
//
// Ein abhängiges Feld wird von einer Stylesheet-Regel ein- und ausgeblendet,
// von nichts sonst. Die Regel heisst .feld-schalter--<name>, der Server
// schreibt den Namen in die Klasse, und keine Zeile im laufenden Programm
// prüft je, dass es die Regel dazu wirklich gibt. Fehlt sie, bleibt ein Feld,
// das der Redaktion verborgen sein sollte, für immer sichtbar — ohne Fehler,
// ohne Eintrag im Protokoll, ohne irgendein Zeichen.
//
// Deshalb behauptet jeder Fall hier zwei Dinge auf einmal: die Klasse am
// Kasten *und* das Element darin, das die zugehörige Regel auswählt. Nur die
// Klasse zu prüfen lässt eine veraltete Regel durch, nur das Element zu prüfen
// lässt einen falschen Namen durch. Das Paar ist der Punkt.
//
// Was dieser Test NICHT beweist: dass :has() und :placeholder-shown sich im
// Browser an einem echten Zahlenfeld so verhalten, wie hier angenommen. Ein
// grüner Lauf hier misst das Markup und nicht den Browser; das war und bleibt
// die Grenze dieses Tests.
//
// Nachgesehen wurde es trotzdem, einmal und ausserhalb der Suite: im
// Browserdurchgang zu Plan 07-07 (5. September 2026, Playwright gegen einen
// frisch gebauten Binary mit eigener Wegwerf-Datenbank). Ein abhängiges Feld
// an einem Bereichsfeld hatte display: none, solange das Zahlenfeld leer war,
// und display: block, sobald eine 6 darinstand. Damit ist D-08 beantwortet:
// KindRange bleibt steuernd, und die Notiz im Fahrplan, es auszuschliessen,
// ruhte auf der Schieber-Annahme, die D-07 verworfen hat. MayControl() wurde
// deshalb nicht angefasst.

// schalterKasten schneidet den Schalterkasten eines Feldes aus: vom
// <div class="feld-schalter feld-schalter--…"> bis dorthin, wo die abhängigen
// Felder anfangen. Zurück kommen der Name der Regel und genau das Markup, das
// diese Regel erreichen können muss — die abhängigen Felder selbst gehören
// nicht dazu, denn das ">" in jedem :has() schliesst sie aus.
func schalterKasten(t *testing.T, body, feldname string) (string, string) {
	t.Helper()
	const auftakt = `class="feld-schalter feld-schalter--`

	at := strings.Index(body, `name="`+feldname+`"`)
	if at < 0 {
		t.Fatalf("the field %q is not in the form", feldname)
	}
	von := strings.LastIndex(body[:at], auftakt)
	if von < 0 {
		t.Fatalf("the field %q is in no switch box — does anything hang off it at all?", feldname)
	}
	rest := body[von:]

	name := rest[len(auftakt):]
	if bis := strings.IndexAny(name, `" `); bis >= 0 {
		name = name[:bis]
	}

	kasten := rest
	if bis := strings.Index(rest, `<div class="feld-abhaengig"`); bis >= 0 {
		kasten = rest[:bis]
	}
	return name, kasten
}

// zeichneMitSchalter legt das steuernde Feld an, hängt ein Textfeld daran und
// gibt den gezeichneten Seiteneditor back.
func zeichneMitSchalter(t *testing.T, steuernd field.Def) string {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	steuernd.WebsiteID = ws.ID
	if _, err := fields.Create(ctx, steuernd); err != nil {
		t.Fatalf("create the controlling field %q: %v", steuernd.Key, err)
	}
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "abhaengig", Label: "Abhängig",
		Kind: field.KindText, Condition: steuernd.Key,
	}); err != nil {
		t.Fatalf("create the dependent field: %v", err)
	}

	p := seedPage(t, database, ws.ID, "Seite", "seite", "text", "draft")
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	return rec.Body.String()
}

func TestSchalter(t *testing.T) {
	cases := []struct {
		name     string
		def      field.Def
		schalter string
		// braucht steht im ganzen Kasten, element im Bedienelement selbst.
		braucht []string
		element []string
	}{
		{
			name: "eine Auswahl als Knopfreihe",
			def: field.Def{Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
				Display: field.DisplayButtons, Choices: []string{"hell", "dunkel"}},
			schalter: "knopfreihe",
			braucht:  []string{`type="radio"`},
			element:  []string{`type="radio"`, `value=""`, `checked`},
		},
		{
			name: "eine Auswahl als Klappliste",
			def: field.Def{Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
				Choices: []string{"hell", "dunkel"}},
			schalter: "auswahl",
			braucht:  []string{`<option value=""`},
			element:  []string{`<select`},
		},
		{
			name: "eine Mehrfachauswahl",
			def: field.Def{Key: "hoelzer", Label: "Hölzer", Kind: field.KindMulti,
				Choices: []string{"Eiche", "Buche"}},
			schalter: "kreuz",
			braucht:  []string{`type="checkbox"`},
		},
		{
			name:     "ein Schlagwortfeld",
			def:      field.Def{Key: "thema", Label: "Thema", Kind: field.KindTerm},
			schalter: "auswahl",
			braucht:  []string{`<option value=""`},
			element:  []string{`<select`},
		},
		{
			name: "ein Bereichsfeld",
			def: field.Def{Key: "menge", Label: "Menge", Kind: field.KindRange,
				RangeMin: "1", RangeMax: "9"},
			schalter: "text",
			braucht:  []string{`placeholder=" "`},
			element:  []string{`<input`, `placeholder=" "`},
		},
		{
			name:     "ein Codefeld",
			def:      field.Def{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode},
			schalter: "text",
			braucht:  []string{`placeholder=" "`},
			element:  []string{`<textarea`, `placeholder=" "`},
		},
	}

	gesehen := map[string]bool{}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			body := zeichneMitSchalter(t, f.def)
			name, kasten := schalterKasten(t, body, f.def.FieldName())
			if name != f.schalter {
				t.Fatalf("the switch is called %q, wanted %q — its rule catches the wrong markup", name, f.schalter)
			}
			gesehen[name] = true

			for _, will := range f.braucht {
				if !strings.Contains(kasten, will) {
					t.Errorf("the box of %q is missing %s — the rule .feld-schalter--%s would have nothing to catch:\n%s",
						f.def.Key, will, name, kasten)
				}
			}
			if len(f.element) > 0 {
				tag := imTag(t, kasten, f.def.FieldName())
				for _, will := range f.element {
					if !strings.Contains(tag, will) {
						t.Errorf("the control of %q is missing %s:\n%s", f.def.Key, will, tag)
					}
				}
			}
		})
	}

	// Der billige, ehrliche Wächter gegen die nächste Art, die einen neuen
	// Schalternamen bekommt und keine Regel dazu.
	t.Run("zu jedem Schalternamen gibt es eine Regel", func(t *testing.T) {
		if len(gesehen) == 0 {
			t.Fatal("no switch name measured — the cases above did not run")
		}
		roh, err := os.ReadFile("../../cmd/holzcloud/assets/admin.css")
		if err != nil {
			t.Fatalf("das Stylesheet lesen: %v", err)
		}
		css := string(roh)

		// Und nicht nur, dass es die Regel gibt, sondern woran sie greift.
		// Eine Regel, die den Namen trägt und das falsche Element sucht, ist
		// keine Regel — sie ist genau die stumme Fehlfunktion, um deretwillen
		// dieser Test geschrieben ist.
		woran := map[string]string{
			"kreuz":      `input[type="checkbox"]:checked`,
			"auswahl":    `option[value=""]:checked`,
			"text":       ":placeholder-shown",
			"knopfreihe": `input[type="radio"][value=""]:checked`,
		}
		for name := range gesehen {
			if !strings.Contains(css, ".feld-schalter--"+name) {
				t.Errorf("the server can send the switch %q, the stylesheet knows no rule for it — every field hanging off it would stay visible for good", name)
				continue
			}
			will, bekannt := woran[name]
			if !bekannt {
				t.Errorf("the switch %q is new and this test does not know what its rule is meant to catch — please enter it here", name)
				continue
			}
			regel := zwischen(t, css, ".feld-schalter--"+name+":has(", "{")
			if !strings.Contains(regel, will) {
				t.Errorf("die Regel zu %q greift nicht an %s:\n%s", name, will, regel)
			}
		}
	})

	// Plan 07-03 hat die Uhrzeit ausgeschlossen, weil ein <input type="time">
	// nie einen Platzhalter zeigt. Ohne diesen Fall liesse sich das
	// zurücknehmen, ohne dass irgendetwas rot wird.
	t.Run("an einer Uhrzeit haengt nichts", func(t *testing.T) {
		h, sm, database, ws := newTestAdmin(t)
		ctx := context.Background()
		fields := field.NewStore(database)
		for _, d := range []field.Def{
			{Key: "abfahrt", Label: "Abfahrt", Kind: field.KindTime},
			{Key: "sorte", Label: "Sorte", Kind: field.KindText},
		} {
			d.WebsiteID = ws.ID
			if _, err := fields.Create(ctx, d); err != nil {
				t.Fatalf("Feld %q anlegen: %v", d.Key, err)
			}
		}
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/felder", nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		rec := serve(t, h, sm, h.HandleFieldList, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("the field list returned %d", rec.Code)
		}
		liste := zwischen(t, rec.Body.String(), `id="feld-bedingung"`, `</select>`)
		if !strings.Contains(liste, `value="sorte"`) {
			t.Fatalf("the text field is not offered at all — then the case says nothing:\n%s", liste)
		}
		if strings.Contains(liste, `value="abfahrt"`) {
			t.Errorf("the time of day is offered as a condition although its rule could never fire:\n%s", liste)
		}
	})

	// Allgemein statt je Art: eine Beschriftung, deren for= auf nichts zeigt,
	// ist mit gar nichts verknüpft, und für einen Screenreader ist das
	// schlechter als gar keine Beschriftung. Der Fall merkt es, wenn eine
	// spätere Art eine Gruppe wird und niemand daran denkt, es zu sagen.
	t.Run("jedes for zeigt auf eine Kennung, die es gibt", func(t *testing.T) {
		bereich := eigeneFelder(t, zeichneAlleArten(t))

		ids := map[string]bool{}
		for _, m := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(bereich, -1) {
			ids[m[1]] = true
		}
		fuer := regexp.MustCompile(`for="([^"]+)"`).FindAllStringSubmatch(bereich, -1)
		if len(fuer) == 0 {
			t.Fatal("not a single for= in the form — the case measures nothing")
		}
		for _, m := range fuer {
			if !ids[m[1]] {
				t.Errorf("for=%q points at an id that does not occur in the form", m[1])
			}
		}

		// Und die beiden Gruppen ausdrücklich: kein for=, dafür ein
		// aria-labelledby auf eine Kennung, die da ist.
		for _, name := range []string{"feld_hoelzer[]", "feld_knopfreihe"} {
			if strings.Contains(bereich, `for="`+name+`"`) {
				t.Errorf("%s is a group of controls and carries a for= regardless", name)
			}
			if !strings.Contains(bereich, `aria-labelledby="`+name+`-label"`) {
				t.Errorf("%s does not name its label through aria-labelledby", name)
			}
			if !ids[name+"-label"] {
				t.Errorf("the label of %s does not carry the id the group appeals to", name)
			}
		}
	})
}

// zwischen schneidet den Ausschnitt von der ersten Marke bis zur nächsten
// zweiten aus.
func zwischen(t *testing.T, body, von, bis string) string {
	t.Helper()
	a := strings.Index(body, von)
	if a < 0 {
		t.Fatalf("%q is not in the document", von)
	}
	rest := body[a:]
	if b := strings.Index(rest, bis); b >= 0 {
		rest = rest[:b]
	}
	return rest
}

// eigeneFelder schneidet den Teil des Seitenformulars aus, der die eigenen
// Felder der Website trägt. Nicht das ganze Dokument: die Verwaltungshülle
// bringt eigene Beschriftungen mit, und die gehen diesen Test nichts an.
func eigeneFelder(t *testing.T, body string) string {
	t.Helper()
	return zwischen(t, body, `class="own-fields"`, `class="access-fields"`)
}

// zeichneAlleArten zeichnet ein Formular mit je einem Feld jeder Art, die
// diese Phase kennt — die Knopfreihe eingeschlossen.
func zeichneAlleArten(t *testing.T) string {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	for _, d := range []field.Def{
		{Key: "kurz", Label: "Kurz", Kind: field.KindText},
		{Key: "lang", Label: "Lang", Kind: field.KindLong},
		{Key: "schnipsel", Label: "Schnipsel", Kind: field.KindCode},
		{Key: "zahl", Label: "Zahl", Kind: field.KindNumber},
		{Key: "menge", Label: "Menge", Kind: field.KindRange, RangeMin: "1", RangeMax: "9"},
		{Key: "tag", Label: "Tag", Kind: field.KindDate},
		{Key: "abfahrt", Label: "Abfahrt", Kind: field.KindTime},
		{Key: "vorraetig", Label: "Vorrätig", Kind: field.KindBool},
		{Key: "farbe", Label: "Farbe", Kind: field.KindChoice, Choices: []string{"hell", "dunkel"}},
		{Key: "knopfreihe", Label: "Knopfreihe", Kind: field.KindChoice,
			Display: field.DisplayButtons, Choices: []string{"eins", "zwei", "drei"}},
		{Key: "hoelzer", Label: "Hölzer", Kind: field.KindMulti, Choices: []string{"Eiche", "Buche"}},
		{Key: "bild", Label: "Bild", Kind: field.KindImage},
		{Key: "adresse", Label: "Adresse", Kind: field.KindLink},
		{Key: "seite", Label: "Seite", Kind: field.KindRef},
		{Key: "thema", Label: "Thema", Kind: field.KindTerm},
		{Key: "ueberschrift", Label: "Heading", Kind: field.KindSection},
		{Key: "zeiten", Label: "Zeiten", Kind: field.KindGroup},
	} {
		d.WebsiteID = ws.ID
		if _, err := fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
	}

	p := seedPage(t, database, ws.ID, "Alles", "alles", "text", "draft")
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	return rec.Body.String()
}

// Die Knopfreihe selbst: welche Knöpfe sie sendet, in welcher Reihenfolge und
// welcher davon angekreuzt ist.
//
// Der leere Knopf ganz vorn ist die Zusage aus D-11. Eine Reihe von
// Radioknöpfen lässt sich in reinem HTML nicht wieder abwählen, ohne ihn wäre
// ein Klick also unwiderruflich — und die Regel .feld-schalter--knopfreihe
// liest genau ihn.
func TestKnopfreihe(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	for _, d := range []field.Def{
		{Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
			Display: field.DisplayButtons, Choices: []string{"hell", "mittel", "dunkel"}},
		// Zwei gleiche Zeilen in der Liste. Die Liste wird wortwörtlich
		// gespeichert und wortwörtlich gelesen; hier eine davon zu
		// unterschlagen hiesse, den Definitionsbildschirm und den Editor
		// verschiedene Dinge sagen zu lassen.
		{Key: "sorte", Label: "Sorte", Kind: field.KindChoice,
			Display: field.DisplayButtons, Choices: []string{"Eiche", "Buche", "Eiche"}},
		// Die Gegenprobe: eine gewöhnliche Auswahl bleibt eine Klappliste.
		{Key: "glanz", Label: "Glanz", Kind: field.KindChoice,
			Choices: []string{"matt", "seidig"}},
	} {
		d.WebsiteID = ws.ID
		if _, err := fields.Create(ctx, d); err != nil {
			t.Fatalf("Feld %q anlegen: %v", d.Key, err)
		}
	}

	p := seedPage(t, database, ws.ID, "Tisch", "tisch", "text", "draft")
	zeichnen := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
		req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
		req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
		rec := serve(t, h, sm, h.HandlePageEdit, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("the form returned %d", rec.Code)
		}
		return rec.Body.String()
	}

	body := zeichnen()

	// --- die Reihe, wie sie ohne gespeicherten Wert aussieht ----------------
	for _, f := range []struct {
		key  string
		will []string
	}{
		{"farbe", []string{"", "hell", "mittel", "dunkel"}},
		{"sorte", []string{"", "Eiche", "Buche", "Eiche"}},
	} {
		reihe := knopfreihe(t, body, "feld_"+f.key)
		if got := knopfwerte(reihe, "feld_"+f.key); !gleich(got, f.will) {
			t.Errorf("%s carries the buttons %q, wanted %q", f.key, got, f.will)
		}
		if !strings.Contains(reihe, `value="" checked`) {
			t.Errorf("on %s the empty button is not ticked when there is no stored value:\n%s", f.key, reihe)
		}
	}

	// --- eine Klappliste bleibt eine Klappliste -----------------------------
	//
	// Gemessen am Element selbst und an den Knöpfen, die seinen Namen tragen,
	// und nicht an einem Fenster darum herum: die Knopfreihe des Nachbarfeldes
	// stünde sonst mit darin und wäre kein Befund.
	glanz := imTag(t, body, "feld_glanz")
	if !strings.Contains(glanz, "<select") {
		t.Errorf("the ordinary choice is no longer a drop-down:\n%s", glanz)
	}
	if got := knopfwerte(body, "feld_glanz"); len(got) > 0 {
		t.Errorf("the ordinary choice carries radio buttons: %q", got)
	}

	// --- mit gespeichertem Wert --------------------------------------------
	roh, err := field.Encode(field.Data{Values: field.Values{"farbe": "mittel"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.NewStore(database).SetFields(ctx, p.ID, roh); err != nil {
		t.Fatalf("Wert setzen: %v", err)
	}
	reihe := knopfreihe(t, zeichnen(), "feld_farbe")
	if !strings.Contains(reihe, `value="mittel" checked`) {
		t.Errorf("the stored value is not ticked:\n%s", reihe)
	}
	if strings.Contains(reihe, `value="" checked`) {
		t.Errorf("the empty button is ticked although a value is stored:\n%s", reihe)
	}
}

// knopfreihe schneidet die Knopfreihe eines Feldes aus.
func knopfreihe(t *testing.T, body, feldname string) string {
	t.Helper()
	return zwischen(t, body, `aria-labelledby="`+feldname+`-label"`, "</div>")
}

// knopfwerte liest die Werte der Radioknöpfe in der Reihenfolge, in der sie
// im Dokument stehen — die Reihenfolge, in der die Möglichkeiten getippt
// wurden.
func knopfwerte(reihe, feldname string) []string {
	re := regexp.MustCompile(`<input type="radio" name="` + regexp.QuoteMeta(feldname) + `" value="([^"]*)"`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(reihe, -1) {
		out = append(out, m[1])
	}
	return out
}

func gleich(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
