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

// The switch mechanism, measured on the markup it needs.
//
// A dependent field is shown and hidden by one stylesheet rule and by nothing
// else. The rule is called .feld-schalter--<name>, the server writes the name
// into the class, and no line in the running program ever checks that the rule
// to go with it really exists. If it is missing, a field that should be hidden
// from the editors stays visible for ever — with no error, no entry in the log,
// no sign of any kind.
//
// That is why every case here asserts two things at once: the class on the box
// *and* the element inside it that the matching rule selects. Checking only the
// class lets an outdated rule through, checking only the element lets a wrong
// name through. The pair is the point.
//
// What this test does NOT prove: that :has() and :placeholder-shown behave in
// the browser on a real number field the way they are assumed to here. A green
// run here measures the markup and not the browser; that was and stays the
// limit of this test.
//
// It was looked at anyway, once and outside the suite: in the browser pass for
// plan 07-07 (5 September 2026, Playwright against a freshly built binary with
// a throwaway database of its own). A dependent field on a range field had
// display: none as long as the number field was empty, and display: block as
// soon as a 6 stood in it. D-08 is answered by that: KindRange stays
// controlling, and the note in the roadmap to exclude it rested on the slider
// assumption that D-07 discarded. MayControl() was therefore left alone.

// switchBox cuts out the switch box of a field: from the
// <div class="feld-schalter feld-schalter--…"> to where the dependent fields
// begin. Back come the name of the rule and exactly the markup that rule has to
// be able to reach — the dependent fields themselves do not belong to it,
// because the ">" in every :has() excludes them.
func switchBox(t *testing.T, body, fieldName string) (string, string) {
	t.Helper()
	const auftakt = `class="feld-schalter feld-schalter--`

	at := strings.Index(body, `name="`+fieldName+`"`)
	if at < 0 {
		t.Fatalf("the field %q is not in the form", fieldName)
	}
	von := strings.LastIndex(body[:at], auftakt)
	if von < 0 {
		t.Fatalf("the field %q is in no switch box — does anything hang off it at all?", fieldName)
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

// drawWithSwitch creates the controlling field, hangs a text field on it and
// hands the drawn page editor back.
func drawWithSwitch(t *testing.T, steuernd field.Def) string {
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
		// needs is looked for in the whole box, element in the control itself.
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
			body := drawWithSwitch(t, f.def)
			name, kasten := switchBox(t, body, f.def.FieldName())
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
				tag := inTag(t, kasten, f.def.FieldName())
				for _, will := range f.element {
					if !strings.Contains(tag, will) {
						t.Errorf("the control of %q is missing %s:\n%s", f.def.Key, will, tag)
					}
				}
			}
		})
	}

	// The cheap, honest guard against the next kind that gets a new switch name
	// and no rule to go with it.
	t.Run("zu jedem Schalternamen gibt es eine Regel", func(t *testing.T) {
		if len(gesehen) == 0 {
			t.Fatal("no switch name measured — the cases above did not run")
		}
		raw, err := os.ReadFile("../../cmd/holzcloud/assets/admin.css")
		if err != nil {
			t.Fatalf("das Stylesheet lesen: %v", err)
		}
		css := string(raw)

		// And not only that the rule exists, but what it takes hold of. A rule
		// that carries the name and looks for the wrong element is no rule —
		// it is exactly the mute malfunction this test is written for.
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
			regel := between(t, css, ".feld-schalter--"+name+":has(", "{")
			if !strings.Contains(regel, will) {
				t.Errorf("die Regel zu %q greift nicht an %s:\n%s", name, will, regel)
			}
		}
	})

	// Plan 07-03 excluded the time of day, because an <input type="time"> never
	// shows a placeholder. Without this case that could be taken back without
	// anything going red.
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
		liste := between(t, rec.Body.String(), `id="feld-bedingung"`, `</select>`)
		if !strings.Contains(liste, `value="sorte"`) {
			t.Fatalf("the text field is not offered at all — then the case says nothing:\n%s", liste)
		}
		if strings.Contains(liste, `value="abfahrt"`) {
			t.Errorf("the time of day is offered as a condition although its rule could never fire:\n%s", liste)
		}
	})

	// In general rather than per kind: a label whose for= points at nothing is
	// tied to nothing at all, and for a screen reader that is worse than no
	// label. The case notices when a later kind becomes a group and nobody
	// thinks to say so.
	t.Run("jedes for zeigt auf eine Kennung, die es gibt", func(t *testing.T) {
		bereich := ownFields(t, drawEveryKind(t))

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

		// And the two groups expressly: no for=, but an aria-labelledby onto an
		// id that is there.
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

// between cuts out the section from the first marker to the next second one.
func between(t *testing.T, body, von, bis string) string {
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

// ownFields cuts out the part of the page form that carries the website's own
// fields. Not the whole document: the admin shell brings labels of its own, and
// those are no business of this test.
func ownFields(t *testing.T, body string) string {
	t.Helper()
	return between(t, body, `class="own-fields"`, `class="access-fields"`)
}

// drawEveryKind draws a form with one field of every kind this phase knows —
// the button row included.
func drawEveryKind(t *testing.T) string {
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

// The button row itself: which buttons it sends, in what order and which of
// them is ticked.
//
// The empty button right at the front is the promise from D-11. A row of radio
// buttons cannot be unselected again in plain HTML, so without it a click would
// be irreversible — and the rule .feld-schalter--knopfreihe reads exactly that
// one.
func TestKnopfreihe(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	fields := field.NewStore(database)

	for _, d := range []field.Def{
		{Key: "farbe", Label: "Farbe", Kind: field.KindChoice,
			Display: field.DisplayButtons, Choices: []string{"hell", "mittel", "dunkel"}},
		// Two identical lines in the list. The list is stored verbatim and read
		// verbatim; suppressing one of them here would mean letting the
		// definition screen and the editor say different things.
		{Key: "sorte", Label: "Sorte", Kind: field.KindChoice,
			Display: field.DisplayButtons, Choices: []string{"Eiche", "Buche", "Eiche"}},
		// The counter-check: an ordinary choice stays a dropdown.
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

	// --- the row as it looks without a stored value -------------------------
	for _, f := range []struct {
		key  string
		will []string
	}{
		{"farbe", []string{"", "hell", "mittel", "dunkel"}},
		{"sorte", []string{"", "Eiche", "Buche", "Eiche"}},
	} {
		reihe := knopfreihe(t, body, "feld_"+f.key)
		if got := buttonValues(reihe, "feld_"+f.key); !gleich(got, f.will) {
			t.Errorf("%s carries the buttons %q, wanted %q", f.key, got, f.will)
		}
		if !strings.Contains(reihe, `value="" checked`) {
			t.Errorf("on %s the empty button is not ticked when there is no stored value:\n%s", f.key, reihe)
		}
	}

	// --- a dropdown stays a dropdown ----------------------------------------
	//
	// Measured on the element itself and on the buttons that carry its name,
	// and not on a window around it: the button row of the neighbouring field
	// would otherwise stand inside it and would be no finding.
	glanz := inTag(t, body, "feld_glanz")
	if !strings.Contains(glanz, "<select") {
		t.Errorf("the ordinary choice is no longer a drop-down:\n%s", glanz)
	}
	if got := buttonValues(body, "feld_glanz"); len(got) > 0 {
		t.Errorf("the ordinary choice carries radio buttons: %q", got)
	}

	// --- with a stored value ------------------------------------------------
	raw, err := field.Encode(field.Data{Values: field.Values{"farbe": "mittel"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.NewStore(database).SetFields(ctx, p.ID, raw); err != nil {
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
func knopfreihe(t *testing.T, body, fieldName string) string {
	t.Helper()
	return between(t, body, `aria-labelledby="`+fieldName+`-label"`, "</div>")
}

// buttonValues reads the values of the radio buttons in the order in which they
// stand in the document — the order in which the options were typed.
func buttonValues(reihe, fieldName string) []string {
	re := regexp.MustCompile(`<input type="radio" name="` + regexp.QuoteMeta(fieldName) + `" value="([^"]*)"`)
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
