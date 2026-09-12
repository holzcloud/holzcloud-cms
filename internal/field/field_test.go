package field

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// The key is derived once from the label and then stands for good. It is the
// name the theme addresses the field by and the name every stored value sits
// under — if it moves, every value is silently gone.
func TestKennungAusBeschriftung(t *testing.T) {
	cases := map[string]string{
		"Preis":          "preis",
		"Preis pro Kilo": "preis_pro_kilo",
		"Verfügbar?":     "verfuegbar",
		"Größe (in cm)":  "groesse_in_cm",
		"  Wurf­datum  ": "wurfdatum",
		"ÄÖÜ":            "aeoeue",
		"Straße":         "strasse",
		"2. Wurf":        "f2_wurf",
		"---":            "",
		"a b  c":         "a_b_c",
		"Sehr langer Name der weit über vierzig Zeichen hinausgeht": "sehr_langer_name_der_weit_ueber_vierzig",
	}
	for label, want := range cases {
		if got := SlugifyKey(label); got != want {
			t.Errorf("SlugifyKey(%q) = %q, want %q", label, got, want)
		}
	}
}

// A link field must not be a way to get javascript: into a theme.
func TestLinkPruefung(t *testing.T) {
	d := Def{Label: "Ziel", Kind: KindLink}

	for _, gut := range []string{
		"/hofladen", "/", "https://example.ch", "http://example.ch",
		"mailto:hof@example.ch", "tel:+41791234567",
	} {
		if reason := Check(d, gut); !reason.Empty() {
			t.Errorf("Check(%q) = %q, erwartet in Ordnung", gut, reason)
		}
	}
	for _, schlecht := range []string{
		"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,<script>",
		"//example.ch/fremd", "hofladen", "vbscript:msgbox",
	} {
		if reason := Check(d, schlecht); reason.Empty() {
			t.Errorf("Check(%q) wurde durchgelassen", schlecht)
		}
	}
}

// The checks say what the editor should change — not what Go reported.
func TestPruefungen(t *testing.T) {
	zahl := Def{Label: "Preis", Kind: KindNumber}
	if r := Check(zahl, "8.50"); !r.Empty() {
		t.Errorf("8.50 abgelehnt: %q", r)
	}
	// A comma is what somebody with a German keyboard types.
	if r := Check(zahl, "8,50"); !r.Empty() {
		t.Errorf("8,50 abgelehnt: %q", r)
	}
	if r := Check(zahl, "acht"); r.Empty() {
		t.Error("„acht“ als Zahl durchgelassen")
	}

	datum := Def{Label: "Wurfdatum", Kind: KindDate}
	if r := Check(datum, "2026-04-01"); !r.Empty() {
		t.Errorf("Datum abgelehnt: %q", r)
	}
	if r := Check(datum, "01.04.2026"); r.Empty() {
		t.Error("Datum im falschen Format durchgelassen")
	}

	auswahl := Def{Label: "Zustand", Kind: KindChoice, Choices: []string{"frisch", "vergriffen"}}
	if r := Check(auswahl, "frisch"); !r.Empty() {
		t.Errorf("valid choice refused: %q", r)
	}
	// The most important case: the <select> can be bypassed, the check cannot.
	if r := Check(auswahl, "erfunden"); r.Empty() {
		t.Error("an option that does not exist was accepted")
	}

	pflicht := Def{Label: "Preis", Kind: KindText, Required: true}
	if r := Check(pflicht, "   "); r.Empty() {
		t.Error("Leerzeichen als Pflichtangabe angenommen")
	}
	if r := Check(Def{Label: "Preis", Kind: KindText}, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
}

// Values for fields that no longer exist disappear on the next save — and not
// already when the field is deleted, so that a mistake can be undone.
func TestAufraeumen(t *testing.T) {
	defs := []Def{{Key: "preis", Kind: KindText}, {Key: "einheit", Kind: KindText}}
	v := Clean(defs, Data{Values: Values{
		"preis":    " 8.50 ",
		"einheit":  "kg",
		"veraltet": "steht nicht mehr im Formular",
		"leer":     "  ",
	}}).Values
	if v["preis"] != "8.50" {
		t.Errorf("preis = %q, want 8.50", v["preis"])
	}
	if _, da := v["veraltet"]; da {
		t.Error("a value with no field survived")
	}
	if _, da := v["leer"]; da {
		t.Error("an empty value was stored")
	}
}

// Empty fields store nothing at all: a page on a website with no fields of its
// own should not carry JSON around with it.
func TestLeeresSpeichertNichts(t *testing.T) {
	raw, err := Encode(Data{})
	if err != nil || raw != "" {
		t.Errorf("Encode(leer) = %q, %v", raw, err)
	}
	if !Decode("").Empty() {
		t.Error("Decode(\"\") lieferte etwas")
	}
	// Broken JSON must not make the page uneditable.
	if !Decode("{kein json").Empty() {
		t.Error("Decode auf Unsinn lieferte etwas")
	}
}

// The theme gets types and not strings: otherwise every {{if}} is true and
// every price is a string that cannot be compared.
func TestAufloesenLiefertTypen(t *testing.T) {
	defs := []Def{
		{Key: "preis", Kind: KindNumber},
		{Key: "verfuegbar", Kind: KindBool},
		{Key: "wurf", Kind: KindDate},
		{Key: "bild", Kind: KindImage},
		{Key: "notiz", Kind: KindText},
	}
	bilder := func(id int64) (Image, bool) {
		if id == 7 {
			return Image{URL: "/media/1/hund.jpg", Alt: "Ein Hund"}, true
		}
		return Image{}, false
	}
	got := Resolve(defs, Data{Values: Values{
		"preis": "8,50", "verfuegbar": "1", "wurf": "2026-04-01", "bild": "7", "notiz": "Text",
	}}, Links{Image: bilder})

	if n, ok := got["preis"].(Number); !ok || n.Value != 8.5 {
		t.Errorf("preis = %#v", got["preis"])
	} else if n.String() != "8,50" {
		// What is printed is what was typed — not 8.5.
		t.Errorf("preis gedruckt als %q, want 8,50", n.String())
	}
	if b, ok := got["verfuegbar"].(bool); !ok || !b {
		t.Errorf("verfuegbar = %#v", got["verfuegbar"])
	}
	if d, ok := got["wurf"].(*time.Time); !ok || d == nil || d.Month() != time.April {
		t.Errorf("wurf = %#v", got["wurf"])
	}
	if img, ok := got["bild"].(*Image); !ok || img == nil || img.Alt != "Ein Hund" {
		t.Errorf("bild = %#v", got["bild"])
	}
	if got["notiz"] != "Text" {
		t.Errorf("notiz = %#v", got["notiz"])
	}
}

// A field with no value still has to be in the result, or
// {{ .Page.Fields.preis }} fails on the one page that has no price.
func TestLeereFelderStehenTrotzdemDa(t *testing.T) {
	defs := []Def{
		{Key: "preis", Kind: KindNumber},
		{Key: "bild", Kind: KindImage},
		{Key: "verfuegbar", Kind: KindBool},
	}
	got := Resolve(defs, Data{}, Links{})
	for _, key := range []string{"preis", "bild", "verfuegbar"} {
		if _, da := got[key]; !da {
			t.Errorf("%s fehlt im Ergebnis", key)
		}
	}
	if img := got["bild"].(*Image); img != nil {
		t.Error("ein leeres Bildfeld sollte nil sein")
	}
	if Filled(got) {
		t.Error("Filled meldet Inhalt, wo keiner ist")
	}
}

// A deleted image must not produce a broken <img>.
func TestVerschwundenesBildWirdNil(t *testing.T) {
	got := Resolve([]Def{{Key: "bild", Kind: KindImage}}, Data{Values: Values{"bild": "99"}},
		Links{Image: func(int64) (Image, bool) { return Image{}, false }})
	if img := got["bild"].(*Image); img != nil {
		t.Errorf("bild = %#v, want nil", img)
	}
}

// A field for posts does not belong on a page.
func TestGiltFuer(t *testing.T) {
	defs := []Def{
		{Key: "preis", AppliesTo: ForPage},
		{Key: "autor", AppliesTo: ForPost},
		{Key: "notiz", AppliesTo: ForBoth},
	}
	seite := For(defs, "page")
	if len(seite) != 2 || seite[0].Key != "preis" || seite[1].Key != "notiz" {
		t.Errorf("Seite bekommt %v", keys(seite))
	}
	beitrag := For(defs, "post")
	if len(beitrag) != 2 || beitrag[0].Key != "autor" {
		t.Errorf("Beitrag bekommt %v", keys(beitrag))
	}
}

func keys(defs []Def) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Key
	}
	return out
}

// --- Gruppen -----------------------------------------------------------------

func gruppe() Def {
	return Def{Key: "preisstaffel", Label: "Preisstaffel", Kind: KindGroup, Sub: []Def{
		{Key: "ab_menge", Label: "Ab Menge", Kind: KindNumber},
		{Key: "preis", Label: "Preis", Kind: KindNumber, Required: true},
		{Key: "einheit", Label: "Einheit", Kind: KindChoice, Choices: []string{"Stück", "Kilo"}},
	}}
}

// A group travels as a list of its own through saving and reading. If the
// order were lost on the way, the tier "from 10" would stand before "from 1".
func TestGruppeUeberlebtSpeichern(t *testing.T) {
	daten := Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50", "einheit": "Stück"},
		{"ab_menge": "10", "preis": "7,00", "einheit": "Stück"},
	}}}
	raw, err := Encode(daten)
	if err != nil {
		t.Fatal(err)
	}
	back := Decode(raw)
	rows := back.Row("preisstaffel")
	if len(rows) != 2 {
		t.Fatalf("%d rows back, want 2", len(rows))
	}
	if rows[0]["ab_menge"] != "1" || rows[1]["ab_menge"] != "10" {
		t.Errorf("Reihenfolge vertauscht: %v", rows)
	}
}

// The flat shape from the first version has to stay readable — otherwise every
// page saved before it would be empty.
func TestAlteFlacheFormWirdGelesen(t *testing.T) {
	d := Decode(`{"preis":"8,50","einheit":"kg"}`)
	if d.Values["preis"] != "8,50" || d.Values["einheit"] != "kg" {
		t.Errorf("alte Form nicht gelesen: %#v", d)
	}
	if d.Rows == nil {
		t.Error("Rows ist nil statt leer")
	}
}

// An empty row is not a row: emptying every field removes it.
func TestLeereZeilenVerschwinden(t *testing.T) {
	g := gruppe()
	out := Clean([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50"},
		{"ab_menge": "  ", "preis": ""},
		{"preis": "7,00"},
	}}})
	rows := out.Row("preisstaffel")
	if len(rows) != 2 {
		t.Fatalf("%d Zeilen, want 2 — die leere sollte weg sein: %v", len(rows), rows)
	}
	// And what stands in a row and does not exist goes the same way.
	out = Clean([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"preis": "7,00", "erfunden": "x"},
	}}})
	if _, da := out.Row("preisstaffel")[0]["erfunden"]; da {
		t.Error("a value with no sub-field survived")
	}
}

// A row with a wrong value is named — with its number, or somebody searches
// twenty rows for the one.
func TestZeileWirdBenannt(t *testing.T) {
	g := gruppe()
	errs := CheckAll([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50", "einheit": "Stück"},
		{"ab_menge": "10", "preis": "teuer", "einheit": "Stück"},
	}}})
	reason, da := errs[RowKey("preisstaffel", 1, "preis")]
	if !da {
		t.Fatalf("no error for row 2: %v", errs)
	}
	if !strings.Contains(reason.String(), "row 2") {
		t.Errorf("the error does not name the row: %q", reason)
	}
}

// A required group with no row is an error, and it belongs to the group.
func TestPflichtgruppeBrauchtEineZeile(t *testing.T) {
	g := gruppe()
	g.Required = true
	errs := CheckAll([]Def{g}, Data{})
	if _, da := errs["preisstaffel"]; !da {
		t.Errorf("no message for the empty required group: %v", errs)
	}
}

// The theme gets the rows resolved — with types, like a single field.
func TestGruppeAufgeloest(t *testing.T) {
	g := gruppe()
	got := Resolve([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50", "einheit": "Stück"},
	}}}, Links{})

	rows, ok := got["preisstaffel"].([]map[string]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("preisstaffel = %#v", got["preisstaffel"])
	}
	if n, ok := rows[0]["preis"].(Number); !ok || n.Value != 8.5 {
		t.Errorf("preis in the row = %#v", rows[0]["preis"])
	}

	// And as a list with labels, for a theme that does not know the names.
	list := List([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50"},
	}}}, Links{})
	if len(list) != 1 || len(list[0].Rows) != 1 {
		t.Fatalf("Liste = %#v", list)
	}
	if list[0].Rows[0][0].Label != "Ab Menge" {
		t.Errorf("erste Beschriftung = %q", list[0].Rows[0][0].Label)
	}
}

// A group with no rows is not in the list: a heading with nothing under it
// says less than nothing.
func TestLeereGruppeStehtNichtInDerListe(t *testing.T) {
	if list := List([]Def{gruppe()}, Data{}, Links{}); len(list) != 0 {
		t.Errorf("Liste = %#v, want leer", list)
	}
}

// A reference is resolved through the lookup — which decides whether the
// target page may be shown at all.
func TestVerweisWirdAufgeloest(t *testing.T) {
	defs := []Def{{Key: "produkt", Kind: KindRef}}
	got := Resolve(defs, Data{Values: Values{"produkt": "42"}}, Links{
		Page: func(id int64) (Ref, bool) {
			if id != 42 {
				return Ref{}, false
			}
			return Ref{Title: "Eichentisch", URL: "/eichentisch", Kind: "page"}, true
		},
	})
	ref, ok := got["produkt"].(*Ref)
	if !ok || ref == nil {
		t.Fatalf("produkt = %#v, want a *Ref", got["produkt"])
	}
	if ref.Title != "Eichentisch" || ref.URL != "/eichentisch" {
		t.Errorf("ref = %#v", ref)
	}
}

// Deleted, moved or still a draft: the theme gets nothing, not a link into the
// void. That is the same rule as for the image.
func TestVerweisAufNichtSichtbaresWirdNil(t *testing.T) {
	defs := []Def{{Key: "produkt", Kind: KindRef}}
	got := Resolve(defs, Data{Values: Values{"produkt": "42"}}, Links{
		Page: func(int64) (Ref, bool) { return Ref{}, false },
	})
	if ref := got["produkt"].(*Ref); ref != nil {
		t.Errorf("produkt = %#v, want nil", ref)
	}
	// And with no lookup — in an export, say — just the same.
	got = Resolve(defs, Data{Values: Values{"produkt": "42"}}, Links{})
	if ref := got["produkt"].(*Ref); ref != nil {
		t.Errorf("ohne Lookup: produkt = %#v, want nil", ref)
	}
}

// What comes out of the form is a number or it is nothing.
func TestVerweisPruefung(t *testing.T) {
	d := Def{Label: "Produkt", Kind: KindRef}
	if reason := Check(d, "17"); !reason.Empty() {
		t.Errorf("Check(17) = %q, want nichts", reason)
	}
	for _, bad := range []string{"/eine-seite", "0", "-3", "abc"} {
		if Check(d, bad).Empty() {
			t.Errorf("Check(%q) has nothing to object to but should have", bad)
		}
	}
}

// A field can belong to a content kind of its own — a price belongs on a
// product and on nothing else.
func TestFeldGiltFuerEigeneArt(t *testing.T) {
	defs := []Def{
		{Key: "preis", AppliesTo: "produkt"},
		{Key: "autor", AppliesTo: ForPost},
		{Key: "hinweis", AppliesTo: ForPage},
		{Key: "notiz", AppliesTo: ForBoth},
	}
	keys := func(kind string) []string {
		var out []string
		for _, d := range For(defs, kind) {
			out = append(out, d.Key)
		}
		return out
	}
	if got := keys("produkt"); len(got) != 2 || got[0] != "preis" || got[1] != "notiz" {
		t.Errorf("for a product: %v", got)
	}
	// A product is technically a page. A field marked "pages only" still must
	// not turn up in the product form.
	if got := keys("page"); len(got) != 2 || got[0] != "hinweis" || got[1] != "notiz" {
		t.Errorf("for a page: %v", got)
	}
	if got := keys("post"); len(got) != 2 || got[0] != "autor" {
		t.Errorf("for a post: %v", got)
	}
}

// A conditional field is not demanded as long as nobody can see it. A required
// field that blocks invisibly is the fault this check exists to prevent.
func TestBedingtesPflichtfeldBlockiertNicht(t *testing.T) {
	defs := []Def{
		{Key: "angebot", Label: "Im Angebot", Kind: KindBool},
		{Key: "sonderpreis", Label: "Sonderpreis", Kind: KindNumber, Required: true, Condition: "angebot"},
	}

	leer := Data{Values: Values{}}
	if errs := CheckAll(defs, leer); len(errs) != 0 {
		t.Errorf("without the tick there is a complaint: %v", errs)
	}

	an := Data{Values: Values{"angebot": "1"}}
	if errs := CheckAll(defs, an); len(errs) != 1 || errs["sonderpreis"].Empty() {
		t.Errorf("with the tick the message is missing: %v", errs)
	}
}

// The value stays put but is not printed as long as the condition is not met:
// an accidentally cleared tick must cost nobody their input, and the theme
// still must show none of it.
func TestBedingterWertBleibtUndWirktNicht(t *testing.T) {
	defs := []Def{
		{Key: "angebot", Label: "Im Angebot", Kind: KindBool},
		{Key: "sonderpreis", Label: "Sonderpreis", Kind: KindNumber, Condition: "angebot"},
	}
	d := Data{Values: Values{"sonderpreis": "9.50"}}

	if got := Resolve(defs, d, Links{})["sonderpreis"].(Number).Raw; got != "" {
		t.Errorf("the theme sees %q instead of nothing", got)
	}
	if got := List(defs, d, Links{}); len(got) != 0 {
		t.Errorf("the field list shows %d entries", len(got))
	}
	// Cleaning does not throw it away.
	if got := Clean(defs, d).Values["sonderpreis"]; got != "9.50" {
		t.Errorf("the value is gone: %q", got)
	}
	// And with the tick it is back.
	d.Values["angebot"] = "1"
	if got := Resolve(defs, d, Links{})["sonderpreis"].(Number).Raw; got != "9.50" {
		t.Errorf("with the tick the value is missing: %q", got)
	}
}

// A chain: C hangs off B, B hangs off A. If A falls away both fall away — and
// regardless of the order the fields stand in.
func TestBedingungsketteFaelltGanz(t *testing.T) {
	defs := []Def{
		{Key: "c", Kind: KindText, Condition: "b"},
		{Key: "b", Kind: KindText, Condition: "a"},
		{Key: "a", Kind: KindText},
	}
	hidden := Hidden(defs, Values{"b": "x", "c": "y"})
	if !hidden["b"] || !hidden["c"] {
		t.Errorf("the chain does not hold: %v", hidden)
	}
	if hidden = Hidden(defs, Values{"a": "1", "b": "x", "c": "y"}); len(hidden) != 0 {
		t.Errorf("with a filled in, something is still hidden: %v", hidden)
	}
}

// A condition pointing at a field that does not exist is none: otherwise the
// field would be unreachable for good and nobody would see why.
func TestBedingungInsLeereZeigtNichts(t *testing.T) {
	defs := []Def{{Key: "preis", Kind: KindNumber, Condition: "gibtsnicht"}}
	if hidden := Hidden(defs, Values{}); len(hidden) != 0 {
		t.Errorf("versteckt: %v", hidden)
	}
}

// A section is a heading: no value, no check, nothing in the theme.
func TestAbschnittHatKeinenWert(t *testing.T) {
	defs := []Def{
		{Key: "masse", Label: "Masse", Kind: KindSection, Required: true},
		{Key: "hoehe", Label: "Höhe", Kind: KindNumber},
	}
	if errs := CheckAll(defs, Data{Values: Values{}}); len(errs) != 0 {
		t.Errorf("a heading is being checked: %v", errs)
	}
	resolved := Resolve(defs, Data{Values: Values{"masse": "irgendwas"}}, Links{})
	if _, da := resolved["masse"]; da {
		t.Error("the heading is in the theme")
	}
	if got := Clean(defs, Data{Values: Values{"masse": "irgendwas"}}).Values["masse"]; got != "" {
		t.Errorf("the heading kept a value: %q", got)
	}
}

// What a condition may hang off is decided by the browser: whatever it cannot
// recognise as "filled in" is not offered in the first place.
func TestWoranEineBedingungHaengenDarf(t *testing.T) {
	// KindRange deliberately stays in: the note in the roadmap to exclude it
	// rested on the assumption of a slider. A number field does show a
	// placeholder — observed in the browser pass of plan 07-07 and not merely
	// inferred: the dependent field was hidden while the number field was
	// empty, and visible as soon as a number stood in it.
	darf := []string{KindText, KindLong, KindNumber, KindBool, KindChoice,
		KindImage, KindLink, KindRef, KindRange, KindCode}
	for _, k := range darf {
		if !(Def{Kind: k}).MayControl() {
			t.Errorf("%s should be allowed to carry a condition", k)
		}
	}
	// KindTime stays out for the same reason as KindDate: an
	// <input type="time"> never matches :placeholder-shown, so the rule that
	// hides the dependent fields could never fire.
	for _, k := range []string{KindDate, KindTime, KindGroup, KindSection} {
		if (Def{Kind: k}).MayControl() {
			t.Errorf("%s should not be allowed to carry a condition", k)
		}
	}
	// Nor a field inside a group: there a row is filled in as a whole.
	if (Def{Kind: KindBool, ParentID: 3}).MayControl() {
		t.Error("a field inside a group should not be allowed to carry a condition")
	}
}

// --- The three small kinds: time, range, code ----------------------------
//
// What stands here is the whole contract of the three. The edge cases are the
// real work: the bound itself still counts, one step beyond it does not, and
// empty is never zero.

// A time of day is a time of day and not a string that happens to contain a
// colon. Depending on the device the browser sends it with or without seconds
// — both have to get through.
func TestZeitPruefung(t *testing.T) {
	zeit := Def{Label: "Abfahrt", Kind: KindTime}
	for _, gut := range []string{"09:30", "00:00", "23:59", "09:30:00"} {
		if r := Check(zeit, gut); !r.Empty() {
			t.Errorf("%q abgelehnt: %q", gut, r)
		}
	}
	// 25:00 does not exist; 9:30 without a leading zero is not what an
	// <input type="time"> sends, and somebody entering it by hand should
	// notice rather than be handed a quietly corrected time.
	for _, schlecht := range []string{"25:00", "9:30", "halb zehn", "09:30+02:00", "2026-04-01"} {
		if r := Check(zeit, schlecht); r.Empty() {
			t.Errorf("%q durchgelassen", schlecht)
		}
	}
	// Empty is allowed as long as the field is not required.
	if r := Check(zeit, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
	if r := Check(Def{Label: "Abfahrt", Kind: KindTime, Required: true}, ""); r.Empty() {
		t.Error("leeres Pflichtfeld angenommen")
	}
}

// Midnight and "nothing entered" are two different facts. A pointer can tell
// them apart, a time.Time cannot.
func TestZeitAufgeloest(t *testing.T) {
	defs := []Def{{Key: "abfahrt", Kind: KindTime}, {Key: "ankunft", Kind: KindTime}}
	got := Resolve(defs, Data{Values: Values{"abfahrt": "00:00"}}, Links{})

	ab, ok := got["abfahrt"].(*time.Time)
	if !ok || ab == nil {
		t.Fatalf("abfahrt = %#v, wollte einen Zeiger auf Mitternacht", got["abfahrt"])
	}
	if ab.Hour() != 0 || ab.Minute() != 0 {
		t.Errorf("abfahrt = %v, wollte 00:00", ab)
	}
	an, ok := got["ankunft"].(*time.Time)
	if !ok || an != nil {
		t.Errorf("arrival = %#v, wanted nil — empty is not midnight", got["ankunft"])
	}

	// No time zone: the time of day is read without a date, so it carries no
	// offset and no date anybody could mistake for a day.
	mittag := Resolve([]Def{{Key: "t", Kind: KindTime}}, Data{Values: Values{"t": "12:15"}}, Links{})
	tz := mittag["t"].(*time.Time)
	if _, versatz := tz.Zone(); versatz != 0 {
		t.Errorf("the time of day carries an offset of %d seconds", versatz)
	}
	if tz.Location() != time.UTC {
		t.Errorf("die Uhrzeit steht in %v statt in UTC", tz.Location())
	}
	// Mit Sekunden ebenfalls, denn Check nimmt beides an.
	mitS := Resolve([]Def{{Key: "t", Kind: KindTime}}, Data{Values: Values{"t": "12:15:30"}}, Links{})
	if s := mitS["t"].(*time.Time); s == nil || s.Second() != 30 {
		t.Errorf("mit Sekunden = %#v", mitS["t"])
	}
}

// A date is left to the theme's formatDate, a time of day has no such helper —
// so it stands there as text, or a list of labels beside the time of day
// prints nothing.
func TestZeitStehtAlsTextInDerListe(t *testing.T) {
	defs := []Def{{Key: "abfahrt", Label: "Abfahrt", Kind: KindTime},
		{Key: "wurf", Label: "Wurf", Kind: KindDate}}
	list := List(defs, Data{Values: Values{"abfahrt": "09:30", "wurf": "2026-04-01"}}, Links{})
	if len(list) != 2 {
		t.Fatalf("%d entries, wanted 2: %+v", len(list), list)
	}
	if list[0].Text != "09:30" {
		t.Errorf("Text der Uhrzeit = %q, wollte \"09:30\"", list[0].Text)
	}
	if _, ok := list[0].Value.(*time.Time); !ok {
		t.Errorf("Value der Uhrzeit = %#v, wollte *time.Time", list[0].Value)
	}
	// The date stays as it was: empty text, the theme formats it itself.
	if list[1].Text != "" {
		t.Errorf("the date now carries text %q — that was not the intention", list[1].Text)
	}
}

// A range field's bounds, measured at each edge separately. The bound itself is
// a valid value; one step beyond it is not.
func TestBereichPruefung(t *testing.T) {
	cases := []struct {
		name       string
		unten, obn string
		gut        []string
		schlecht   []string
	}{
		{"beide Grenzen", "1", "10",
			[]string{"1", "10", "5.5", "5,5"},
			[]string{"0", "11", "0.999", "10.001", "viel"}},
		{"gleiche Grenzen", "5", "5",
			[]string{"5", "5.0"},
			[]string{"4", "6"}},
		{"nur unten", "1", "",
			[]string{"1", "1000000"},
			[]string{"0", "-3"}},
		{"nur oben", "", "10",
			[]string{"10", "-1000"},
			[]string{"11"}},
		{"gar keine", "", "",
			[]string{"0", "-7", "12345.6"},
			[]string{"viel"}},
	}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			d := Def{Label: "Menge", Kind: KindRange, RangeMin: f.unten, RangeMax: f.obn}
			for _, gut := range f.gut {
				if r := Check(d, gut); !r.Empty() {
					t.Errorf("%q abgelehnt: %q", gut, r)
				}
			}
			for _, schlecht := range f.schlecht {
				r := Check(d, schlecht)
				if r.Empty() {
					t.Errorf("%q durchgelassen", schlecht)
					continue
				}
				if _, istZahl := ParseNumber(schlecht); !istZahl {
					// Not a number is not a bounds violation but something
					// else — and the reason says so too.
					if !strings.Contains(r.String(), "number") {
						t.Errorf("the reason for %q does not name the number: %q", schlecht, r)
					}
					continue
				}
				// The reason is for the person at the form: it names the bounds
				// the value would have to lie between.
				genannt := (f.unten != "" && strings.Contains(r.String(), f.unten)) ||
					(f.obn != "" && strings.Contains(r.String(), f.obn))
				if !genannt {
					t.Errorf("the reason for %q names no bound: %q", schlecht, r)
				}
			}
		})
	}
}

// Empty is not zero and not the lower bound: a range field nobody has filled in
// is not filled in.
func TestLeererBereichIstNichtNull(t *testing.T) {
	kann := Def{Key: "menge", Label: "Menge", Kind: KindRange, RangeMin: "1", RangeMax: "10"}
	if r := Check(kann, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
	pflicht := kann
	pflicht.Required = true
	if r := Check(pflicht, ""); r.Empty() {
		t.Error("leeres Pflichtfeld angenommen")
	} else if !strings.Contains(r.String(), "filled in") {
		t.Errorf("the reason is not the usual required message: %q", r)
	}

	got := Resolve([]Def{kann}, Data{}, Links{})
	n, ok := got["menge"].(Number)
	if !ok {
		t.Fatalf("menge = %#v, wollte Number", got["menge"])
	}
	if n.Raw != "" || n.Value != 0 {
		t.Errorf("empty range = %#v, wanted the zero value with an empty Raw", n)
	}
	if len(List([]Def{kann}, Data{}, Links{})) != 0 {
		t.Error("an empty range is in the list")
	}
	if Filled(got) {
		t.Error("Filled meldet Inhalt, wo keiner ist")
	}
}

// What is printed is what was typed. 0.1 is 0.1 and not 0.10000000000000001.
func TestBereichDrucktDasGetippte(t *testing.T) {
	d := Def{Key: "menge", Label: "Menge", Kind: KindRange, RangeMin: "0", RangeMax: "1"}
	got := Resolve([]Def{d}, Data{Values: Values{"menge": "0.1"}}, Links{})
	n, ok := got["menge"].(Number)
	if !ok {
		t.Fatalf("menge = %#v, wollte Number", got["menge"])
	}
	if n.Raw != "0.1" || n.String() != "0.1" {
		t.Errorf("Raw = %q, gedruckt %q — wollte beide \"0.1\"", n.Raw, n.String())
	}
	if n.Value != 0.1 {
		t.Errorf("Value = %v, wollte 0.1", n.Value)
	}
	list := List([]Def{d}, Data{Values: Values{"menge": "0.1"}}, Links{})
	if len(list) != 1 || list[0].Text != "0.1" {
		t.Errorf("list = %+v, wanted an entry with text \"0.1\"", list)
	}
}

// A code field is text as it was typed — no Markdown, no reinterpretation. And
// an empty one stays out of the list.
func TestCodeIstRoherText(t *testing.T) {
	d := Def{Key: "schnipsel", Label: "Schnipsel", Kind: KindCode}
	roh := "<b>fett</b> & \"Anführung\"\n  eingerückt"
	got := Resolve([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})
	if got["schnipsel"] != roh {
		t.Errorf("snippet = %#v, wanted the raw text unchanged", got["schnipsel"])
	}
	// And explicitly as a string and not as template.HTML: a theme gets a
	// value that html/template escapes when printing. A retyping anywhere on
	// this path would be exactly the hole FIELD-06 closes — and it would show
	// up nowhere else, because the two print identically.
	if _, istString := got["schnipsel"].(string); !istString {
		t.Errorf("snippet is %T and not a string — is it still escaped?", got["schnipsel"])
	}
	if e := List([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})[0]; e.Kind != KindCode {
		t.Errorf("der Eintrag nennt seine Art als %q", e.Kind)
	}
	// Check accepts any text: there is no wrong line of code.
	if r := Check(d, roh); !r.Empty() {
		t.Errorf("Code abgelehnt: %q", r)
	}
	list := List([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})
	if len(list) != 1 || list[0].Text != roh {
		t.Errorf("list = %+v, wanted an entry with the raw text", list)
	}
	if len(List([]Def{d}, Data{}, Links{})) != 0 {
		t.Error("an empty code field is in the list")
	}
}

// The two lists are subtractive: a new kind is in unless it is explicitly
// excluded. For all three that is right — and for code it is the condition
// under which success criterion 5 can be posed at all.
func TestNeueArtenStehenInBeidenListen(t *testing.T) {
	enthaelt := func(kinds []Kind, art string) bool {
		for _, k := range kinds {
			if k.Kind == art {
				return true
			}
		}
		return false
	}
	for _, art := range []string{KindTime, KindRange, KindCode} {
		if !KnownKind(art) {
			t.Errorf("%s is not in Kinds", art)
		}
		if !enthaelt(SubKinds(), art) {
			t.Errorf("%s fehlt in SubKinds", art)
		}
		if !enthaelt(BlockKinds(), art) {
			t.Errorf("%s fehlt in BlockKinds", art)
		}
		if KindName(art) == art {
			t.Errorf("%s has no label", art)
		}
	}
}

// --- The two guards of a multi-valued field ------------------------------
//
// A checkbox field cannot be bounded in the markup — that would need
// JavaScript, and this program carries none of it besides htmx. So the maximum
// holds here, on the server, or nowhere.

func TestMehrfachauswahlHoechstzahl(t *testing.T) {
	auswahl := []string{"Eiche", "Buche", "Esche", "Erle"}
	cases := []struct {
		name     string
		max      int
		gut      []string
		schlecht []string
	}{
		{"höchstens zwei", 2,
			[]string{"", "Eiche", "Eiche\nBuche"},
			[]string{"Eiche\nBuche\nEsche", "Eiche\nBuche\nEsche\nErle"}},
		{"höchstens eine", 1,
			[]string{"Eiche"},
			[]string{"Eiche\nBuche"}},
		{"null heisst ohne Grenze", 0,
			[]string{"Eiche", "Eiche\nBuche\nEsche\nErle"},
			nil},
	}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			d := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Choices: auswahl, MaxValues: f.max}
			for _, gut := range f.gut {
				if r := Check(d, gut); !r.Empty() {
					t.Errorf("%q abgelehnt: %q", gut, r)
				}
			}
			for _, schlecht := range f.schlecht {
				r := Check(d, schlecht)
				if r.Empty() {
					t.Errorf("%q durchgelassen", schlecht)
					continue
				}
				// The reason is for the person at the form: it names the field
				// and the number that matters.
				if !strings.Contains(r.String(), "Sorten") {
					t.Errorf("the reason for %q does not name the field: %q", schlecht, r)
				}
				if f.max > 1 && !strings.Contains(r.String(), strconv.Itoa(f.max)) {
					t.Errorf("the reason for %q does not name the maximum: %q", schlecht, r)
				}
			}
		})
	}
}

// Exactly the maximum still passes, one more does not. The edge is the whole
// question — a limit you have to guess the inclusiveness of is not one.
func TestMehrfachauswahlGenauAmRand(t *testing.T) {
	d := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti,
		Choices: []string{"a", "b", "c"}, MaxValues: 3}
	if r := Check(d, "a\nb\nc"); !r.Empty() {
		t.Errorf("genau drei abgelehnt: %q", r)
	}
	// Duplicates count singly: JoinValues keeps them, so there are four
	// values even though only three of them are distinct.
	if r := Check(d, "a\nb\nc\na"); r.Empty() {
		t.Error("four values let through although at most three are allowed")
	}
}

// MaxValueBytes is the room for all of a field's values together, in a
// multi-valued one including the line breaks between them. Exactly that long
// still passes, one byte more does not — and nothing is truncated on the way.
func TestGemeinsamesBytebudget(t *testing.T) {
	text := Def{Key: "notiz", Label: "Notiz", Kind: KindLong}

	genau := strings.Repeat("a", MaxValueBytes)
	if r := Check(text, genau); !r.Empty() {
		t.Errorf("genau %d Byte abgelehnt: %q", MaxValueBytes, r)
	}
	r := Check(text, genau+"a")
	if r.Empty() {
		t.Fatalf("%d Byte durchgelassen", MaxValueBytes+1)
	}
	if !strings.Contains(r.String(), "Notiz") {
		t.Errorf("the reason does not name the field: %q", r)
	}
	if !strings.Contains(r.String(), strconv.Itoa(MaxValueBytes)) {
		t.Errorf("the reason does not name the bound: %q", r)
	}

	// Bytes are counted, not characters: an umlaut needs two of them, so the
	// limit is reached at half as many characters. A rune or grapheme counter
	// would be a different number and would overrun the database.
	umlaute := strings.Repeat("ä", MaxValueBytes/2)
	if len([]rune(umlaute)) >= MaxValueBytes {
		t.Fatalf("the probe is no good: %d runes", len([]rune(umlaute)))
	}
	if r := Check(text, umlaute); !r.Empty() {
		t.Errorf("genau %d Byte aus Umlauten abgelehnt: %q", len(umlaute), r)
	}
	if r := Check(text, umlaute+"ä"); r.Empty() {
		t.Errorf("%d Byte aus Umlauten durchgelassen", len(umlaute)+2)
	}
}

// In a multi-valued field the joined string is measured, not the longest single
// value: three values of two thirds of the room each do not fit side by side,
// even though each fits on its own.
func TestBytebudgetGiltAllenWertenZusammen(t *testing.T) {
	kurz := strings.Repeat("a", MaxValueBytes/2)
	lang := kurz + "a"
	d := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Choices: []string{kurz, lang}}

	if r := Check(d, kurz); !r.Empty() {
		t.Errorf("a value of %d bytes refused: %q", len(kurz), r)
	}
	// Two values of MaxValueBytes/2 each plus the break between them: one byte
	// over the limit. The break counts, or more would go into the database
	// than it was promised.
	zusammen := JoinValues([]string{kurz, kurz})
	if len(zusammen) != MaxValueBytes+1 {
		t.Fatalf("the probe is no good: %d bytes", len(zusammen))
	}
	if r := Check(d, zusammen); r.Empty() {
		t.Errorf("%d bytes let through joined — what was measured was evidently the longest single value", len(zusammen))
	}
}

// Nothing on the storage path truncates any more. An over-long value is
// reported, not halved: a halved value looks like one somebody typed that way,
// and in a multi-valued field half a value would be a value that never existed
// (D-13).
func TestNichtsWirdMehrStillGekuerzt(t *testing.T) {
	zuLang := strings.Repeat("a", MaxValueBytes+50)

	d := Def{Key: "notiz", Label: "Notiz", Kind: KindLong}
	sauber := Clean([]Def{d}, Data{Values: Values{"notiz": "  " + zuLang + "  "}})
	if got := sauber.Values["notiz"]; got != zuLang {
		t.Errorf("Clean stored %d bytes, wanted %d — it is still truncating", len(got), len(zuLang))
	}

	// In einer Gruppenzeile ebenso.
	g := Def{Key: "staffel", Label: "Staffel", Kind: KindGroup, Sub: []Def{d}}
	zeilen := Clean([]Def{g}, Data{Rows: map[string][]Values{"staffel": {{"notiz": zuLang}}}})
	if got := zeilen.Rows["staffel"][0]["notiz"]; got != zuLang {
		t.Errorf("%d bytes were stored in the row, wanted %d", len(got), len(zuLang))
	}

	// And CheckAll reports it, under the field's key, so that the form shows
	// the reason under the right field.
	errs := CheckAll([]Def{d}, Data{Values: Values{"notiz": zuLang}})
	if errs["notiz"].Empty() {
		t.Errorf("CheckAll does not report the over-long value: %v", errs)
	}
}

// A field whose condition is not met is not checked at all — in a multi-valued
// one as much as in any other. Demanding something the person cannot see is the
// one way a form cannot be submitted without saying why.
//
// Where the boundary lies: every per-kind rule is skipped for a hidden field —
// the required check, the maximum, the closed list of options. The byte limit
// is not: it is a question to nobody, it says how much room a value has in the
// row that gets written. That one is guarded by
// TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden.
func TestVerstecktesMehrwertigesFeldWirdNichtGeprueft(t *testing.T) {
	schalter := Def{Key: "spezial", Label: "Spezial", Kind: KindBool}
	sorten := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Required: true,
		Choices: []string{"Eiche", "Buche"}, MaxValues: 1, Condition: "spezial"}
	defs := []Def{schalter, sorten}

	// Over every per-kind bound at once: three values at MaxValues 1, "Ahorn"
	// is not on the list, and the field is required. Well under the byte limit,
	// so that a failure here unambiguously means a per-kind rule fired — and
	// not the length.
	uebervoll := JoinValues([]string{"Eiche", "Buche", "Ahorn"})

	aus := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": uebervoll}})
	if len(aus) != 0 {
		t.Errorf("a hidden field was checked: %v", aus)
	}

	an := CheckAll(defs, Data{Values: Values{"spezial": "1", "sorten": uebervoll}})
	if an["sorten"].Empty() {
		t.Errorf("the visible field was not checked: %v", an)
	}
}

// --- The term field ------------------------------------------------------
//
// It stores the term's address and prints its name. Rename the term and what
// every page shows changes without a single page being touched. That same
// promise is also why the kind must not appear in any block kind: a block
// freezes into HTML when the page is saved and could not keep it.

func TestSchlagwortStehtNichtInBausteinarten(t *testing.T) {
	enthaelt := func(kinds []Kind, art string) bool {
		for _, k := range kinds {
			if k.Kind == art {
				return true
			}
		}
		return false
	}
	if !KnownKind(KindTerm) {
		t.Errorf("%s is not in Kinds", KindTerm)
	}
	if KindName(KindTerm) == KindTerm {
		t.Errorf("%s has no label", KindTerm)
	}
	// Inside a group it is allowed: a group freezes nothing.
	if !enthaelt(SubKinds(), KindTerm) {
		t.Errorf("%s fehlt in SubKinds", KindTerm)
	}
	if enthaelt(BlockKinds(), KindTerm) {
		t.Errorf("%s is in BlockKinds but should not be", KindTerm)
	}

	// Exactly two kinds are missing there beside group and section — the
	// reference and the term. The filter is subtractive: a kind that got added
	// here by accident would vanish silently from every block form.
	fehlend := map[string]bool{}
	for _, k := range Kinds {
		fehlend[k.Kind] = true
	}
	for _, k := range BlockKinds() {
		delete(fehlend, k.Kind)
	}
	wollte := map[string]bool{KindGroup: true, KindSection: true, KindRef: true, KindTerm: true}
	if len(fehlend) != len(wollte) {
		t.Errorf("BlockKinds leaves out %v, wanted %v", fehlend, wollte)
	}
	for art := range wollte {
		if !fehlend[art] {
			t.Errorf("BlockKinds contains %s but should leave it out", art)
		}
	}
	// A code field may stand in a block (D-06); it is the kind most likely to
	// be swept along by the exclusion.
	if !enthaelt(BlockKinds(), KindCode) {
		t.Errorf("%s fehlt in BlockKinds", KindCode)
	}
}

func TestSchlagwortWirdAufgeloest(t *testing.T) {
	defs := []Def{{Key: "thema", Kind: KindTerm}}
	got := Resolve(defs, Data{Values: Values{"thema": "moebel"}}, Links{
		Term: func(slug string) (Term, bool) {
			if slug != "moebel" {
				return Term{}, false
			}
			return Term{Name: "Möbelbau", Slug: "moebel", URL: "/tag/moebel"}, true
		},
	})
	term, ok := got["thema"].(*Term)
	if !ok || term == nil {
		t.Fatalf("thema = %#v, want a *Term", got["thema"])
	}
	// The name as of now, not the one from back then: what is stored is
	// "moebel", what is printed is "Möbelbau". //nolint:german — the fixture
	// it names is German content, which is what an operator types.
	if term.Name != "Möbelbau" || term.Slug != "moebel" || term.URL != "/tag/moebel" {
		t.Errorf("term = %#v", term)
	}
}

// The four paths into nothing. Each yields the same typed nil, so that a
// {{with}} in the theme leaves the block out instead of printing an old name.
func TestSchlagwortOhneTrefferWirdNil(t *testing.T) {
	defs := []Def{{Key: "thema", Kind: KindTerm}}
	treffer := func(slug string) (Term, bool) {
		if slug == "moebel" {
			return Term{Name: "Möbelbau", Slug: "moebel", URL: "/tag/moebel"}, true
		}
		return Term{}, false
	}
	cases := []struct {
		name  string
		wert  string
		links Links
	}{
		{"kein Wert", "", Links{Term: treffer}},
		{"ohne Nachschlagefunktion", "moebel", Links{}},
		{"gelöschtes Schlagwort", "verschwunden", Links{Term: treffer}},
		// A slug is the lower-case form of a name. Folding here would make two
		// different terms collapse into one.
		{"andere Schreibung", "Moebel", Links{Term: treffer}},
	}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			got := Resolve(defs, Data{Values: Values{"thema": f.wert}}, f.links)
			term, ok := got["thema"].(*Term)
			if !ok {
				t.Fatalf("thema = %#v, want a typed nil *Term", got["thema"])
			}
			if term != nil {
				t.Errorf("thema = %#v, want nil", term)
			}
		})
	}
}

func TestSchlagwortStehtMitNamenInDerListe(t *testing.T) {
	defs := []Def{
		{Key: "thema", Label: "Thema", Kind: KindTerm},
		{Key: "weg", Label: "Weg", Kind: KindTerm},
	}
	daten := Data{Values: Values{"thema": "moebel", "weg": "verschwunden"}}
	links := Links{Term: func(slug string) (Term, bool) {
		if slug != "moebel" {
			return Term{}, false
		}
		return Term{Name: "Möbelbau", Slug: "moebel", URL: "/tag/moebel"}, true
	}}

	liste := List(defs, daten, links)
	if len(liste) != 1 {
		t.Fatalf("liste = %#v, wollte genau einen Eintrag", liste)
	}
	e := liste[0]
	if e.Term == nil || e.Term.Name != "Möbelbau" {
		t.Errorf("Term = %#v", e.Term)
	}
	// The name, not the slug: a list of labels and values should show
	// "Möbelbau" and not "moebel". //nolint:german — same fixture as above.
	if e.Text != "Möbelbau" {
		t.Errorf("Text = %q, wollte den Namen", e.Text)
	}

	if !Filled(Resolve(defs, daten, links)) {
		t.Error("Filled reports nothing although a term is resolved")
	}
	// And with no hit the page is empty — otherwise only the text would
	// disappear and the panel would stay.
	if Filled(Resolve(defs, Data{Values: Values{"weg": "verschwunden"}}, links)) {
		t.Error("Filled reports something although no term is resolved")
	}
}

// What comes out of the form is a slug, or it is somebody typing into the form
// by hand.
func TestSchlagwortPruefung(t *testing.T) {
	d := Def{Label: "Thema", Kind: KindTerm}
	for _, gut := range []string{"moebel", "moebel-nach-mass", "holz2024"} {
		if reason := Check(d, gut); !reason.Empty() {
			t.Errorf("Check(%q) = %q, want nichts", gut, reason)
		}
	}
	for _, bad := range []string{"Moebel", "moebel nach mass", "möbel", "/tag/moebel", "-moebel"} {
		if Check(d, bad).Empty() {
			t.Errorf("Check(%q) has nothing to object to but should have", bad)
		}
	}
}

// The byte limit holds where nobody was asked for the value either.
//
// Clean deliberately keeps a hidden field's value (field.go:568-573), and since
// D-13 trimTo truncates nothing — CheckAll is therefore the only place the byte
// budget still holds at all. If it were bypassed here, the limit would exist
// nowhere for this class: the browser sends a hidden field's value along, and
// somebody building the form by hand sends whatever they like.
func TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden(t *testing.T) {
	schalter := Def{Key: "spezial", Label: "Spezial", Kind: KindBool}
	sorten := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Required: true,
		Choices: []string{"Eiche", "Buche"}, MaxValues: 1, Condition: "spezial"}
	defs := []Def{schalter, sorten}

	zuLang := strings.Repeat("x", MaxValueBytes+1)
	aus := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": zuLang}})
	if aus["sorten"].Empty() {
		t.Fatalf("the over-long value of a hidden field was not reported: %v", aus)
	}
	// And with the LENGTH reason, not the options reason: no per-kind rule may
	// have been smuggled back in here.
	if !strings.Contains(aus["sorten"].String(), "too long") {
		t.Errorf("the reason is not the length one: %q", aus["sorten"])
	}

	// The real promise of this change: only per-kind rules violated, and the
	// hidden field stays silent. Required, maximum and the closed list still do
	// not hold for a field nobody can see.
	nurArt := JoinValues([]string{"Eiche", "Buche", "Ahorn"})
	if still := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": nurArt}}); len(still) != 0 {
		t.Errorf("a per-kind rule fired for a hidden field: %v", still)
	}
	// The empty required value stays silent too.
	if still := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": ""}}); len(still) != 0 {
		t.Errorf("the required check fired for a hidden field: %v", still)
	}
}

// The same hole one level down, with MaxRows rows times sub-fields as the
// lever: validate empties the condition only for a field inside a group or
// inside a block kind (store.go:521-523), so a top-level group may carry one.
func TestVersteckteGruppeBleibtAnDieBytegrenzeGebunden(t *testing.T) {
	schalter := Def{Key: "spezial", Label: "Spezial", Kind: KindBool}
	notiz := Def{Key: "notiz", Label: "Notiz", Kind: KindLong, Required: true}
	gruppe := Def{Key: "staffel", Label: "Staffel", Kind: KindGroup, Required: true,
		Condition: "spezial", Sub: []Def{notiz}}
	defs := []Def{schalter, gruppe}

	zuLang := strings.Repeat("x", MaxValueBytes+1)
	aus := CheckAll(defs, Data{
		Values: Values{"spezial": ""},
		Rows:   map[string][]Values{"staffel": {{"notiz": zuLang}}},
	})
	schluessel := RowKey("staffel", 0, "notiz")
	if aus[schluessel].Empty() {
		t.Fatalf("the over-long value in the row of a hidden group was not reported: %v", aus)
	}
	if !strings.Contains(aus[schluessel].String(), "too long") {
		t.Errorf("the reason is not the length one: %q", aus[schluessel])
	}

	// Counter-check: a hidden required group with no row stays silent, and so
	// does an empty required sub-field in its row.
	if still := CheckAll(defs, Data{Values: Values{"spezial": ""}}); len(still) != 0 {
		t.Errorf("“needs at least one row” fired for a hidden group: %v", still)
	}
	if still := CheckAll(defs, Data{
		Values: Values{"spezial": ""},
		Rows:   map[string][]Values{"staffel": {{"notiz": ""}}},
	}); len(still) != 0 {
		t.Errorf("the required check fired in the row of a hidden group: %v", still)
	}
}
