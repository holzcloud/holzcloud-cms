package field

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// Die Kennung wird einmal aus der Beschriftung gebildet und steht danach fest.
// Sie ist der Name, unter dem das Theme das Feld anspricht und unter dem jeder
// gespeicherte Wert steht — bewegt sie sich, sind alle Werte still weg.
func TestKennungAusBeschriftung(t *testing.T) {
	fälle := map[string]string{
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
	for label, want := range fälle {
		if got := SlugifyKey(label); got != want {
			t.Errorf("SlugifyKey(%q) = %q, want %q", label, got, want)
		}
	}
}

// Ein Link-Feld darf kein Weg sein, javascript: in ein Theme zu bekommen.
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

// Die Prüfungen sagen, was der Redakteur ändern soll — nicht, was Go gemeldet hat.
func TestPruefungen(t *testing.T) {
	zahl := Def{Label: "Preis", Kind: KindNumber}
	if r := Check(zahl, "8.50"); !r.Empty() {
		t.Errorf("8.50 abgelehnt: %q", r)
	}
	// Ein Komma ist, was jemand mit einer deutschen Tastatur tippt.
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
		t.Errorf("gültige Auswahl abgelehnt: %q", r)
	}
	// Der wichtigste Fall: das <select> lässt sich umgehen, die Prüfung nicht.
	if r := Check(auswahl, "erfunden"); r.Empty() {
		t.Error("eine Möglichkeit, die es nicht gibt, wurde angenommen")
	}

	pflicht := Def{Label: "Preis", Kind: KindText, Required: true}
	if r := Check(pflicht, "   "); r.Empty() {
		t.Error("Leerzeichen als Pflichtangabe angenommen")
	}
	if r := Check(Def{Label: "Preis", Kind: KindText}, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
}

// Werte zu Feldern, die es nicht mehr gibt, verschwinden beim nächsten
// Speichern — und nicht schon beim Löschen des Feldes, damit ein Versehen
// rückgängig zu machen ist.
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
		t.Error("ein Wert ohne Feld hat überlebt")
	}
	if _, da := v["leer"]; da {
		t.Error("ein leerer Wert wurde gespeichert")
	}
}

// Leere Felder speichern gar nichts: eine Seite auf einer Website ohne eigene
// Felder soll kein JSON mit sich herumtragen.
func TestLeeresSpeichertNichts(t *testing.T) {
	raw, err := Encode(Data{})
	if err != nil || raw != "" {
		t.Errorf("Encode(leer) = %q, %v", raw, err)
	}
	if !Decode("").Empty() {
		t.Error("Decode(\"\") lieferte etwas")
	}
	// Kaputtes JSON darf die Seite nicht unbearbeitbar machen.
	if !Decode("{kein json").Empty() {
		t.Error("Decode auf Unsinn lieferte etwas")
	}
}

// Das Theme bekommt Typen und nicht Zeichenketten: sonst ist jedes {{if}} wahr
// und jeder Preis eine Zeichenkette, die sich nicht vergleichen lässt.
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
		// Ausgegeben wird, was getippt wurde — nicht 8.5.
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

// Ein Feld ohne Wert muss trotzdem im Ergebnis stehen, sonst scheitert
// {{ .Page.Fields.preis }} auf der einen Seite, die keinen Preis hat.
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

// Ein gelöschtes Bild darf kein kaputtes <img> ergeben.
func TestVerschwundenesBildWirdNil(t *testing.T) {
	got := Resolve([]Def{{Key: "bild", Kind: KindImage}}, Data{Values: Values{"bild": "99"}},
		Links{Image: func(int64) (Image, bool) { return Image{}, false }})
	if img := got["bild"].(*Image); img != nil {
		t.Errorf("bild = %#v, want nil", img)
	}
}

// Ein Feld für Beiträge gehört nicht auf eine Seite.
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

// Eine Gruppe reist als eigene Liste durch Speichern und Lesen. Ginge dabei die
// Reihenfolge verloren, stünde die Staffel „ab 10“ vor „ab 1“.
func TestGruppeUeberlebtSpeichern(t *testing.T) {
	daten := Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50", "einheit": "Stück"},
		{"ab_menge": "10", "preis": "7,00", "einheit": "Stück"},
	}}}
	raw, err := Encode(daten)
	if err != nil {
		t.Fatal(err)
	}
	zurück := Decode(raw)
	rows := zurück.Row("preisstaffel")
	if len(rows) != 2 {
		t.Fatalf("%d Zeilen zurück, want 2", len(rows))
	}
	if rows[0]["ab_menge"] != "1" || rows[1]["ab_menge"] != "10" {
		t.Errorf("Reihenfolge vertauscht: %v", rows)
	}
}

// Die flache Form aus der ersten Fassung muss weiter lesbar sein — sonst wäre
// jede Seite, die vorher gespeichert wurde, leer.
func TestAlteFlacheFormWirdGelesen(t *testing.T) {
	d := Decode(`{"preis":"8,50","einheit":"kg"}`)
	if d.Values["preis"] != "8,50" || d.Values["einheit"] != "kg" {
		t.Errorf("alte Form nicht gelesen: %#v", d)
	}
	if d.Rows == nil {
		t.Error("Rows ist nil statt leer")
	}
}

// Eine leere Zeile ist keine Zeile: wer alle Felder leert, hat sie entfernt.
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
	// Und was in einer Zeile steht, das es nicht gibt, geht denselben Weg.
	out = Clean([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"preis": "7,00", "erfunden": "x"},
	}}})
	if _, da := out.Row("preisstaffel")[0]["erfunden"]; da {
		t.Error("ein Wert ohne Unterfeld hat überlebt")
	}
}

// Eine Zeile mit einem falschen Wert wird benannt — mit Nummer, sonst sucht
// jemand in zwanzig Zeilen nach der einen.
func TestZeileWirdBenannt(t *testing.T) {
	g := gruppe()
	errs := CheckAll([]Def{g}, Data{Rows: map[string][]Values{"preisstaffel": {
		{"ab_menge": "1", "preis": "8,50", "einheit": "Stück"},
		{"ab_menge": "10", "preis": "teuer", "einheit": "Stück"},
	}}})
	reason, da := errs[RowKey("preisstaffel", 1, "preis")]
	if !da {
		t.Fatalf("kein Fehler für Zeile 2: %v", errs)
	}
	if !strings.Contains(reason.String(), "Zeile 2") {
		t.Errorf("Fehler nennt die Zeile nicht: %q", reason)
	}
}

// Eine Pflicht-Gruppe ohne Zeile ist ein Fehler, und zwar an der Gruppe.
func TestPflichtgruppeBrauchtEineZeile(t *testing.T) {
	g := gruppe()
	g.Required = true
	errs := CheckAll([]Def{g}, Data{})
	if _, da := errs["preisstaffel"]; !da {
		t.Errorf("keine Meldung für die leere Pflichtgruppe: %v", errs)
	}
}

// Das Theme bekommt die Zeilen aufgelöst — mit Typen, wie ein einzelnes Feld.
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
		t.Errorf("preis in der Zeile = %#v", rows[0]["preis"])
	}

	// Und als Liste mit Beschriftungen, für ein Theme, das die Namen nicht kennt.
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

// Eine Gruppe ohne Zeilen steht nicht in der Liste: eine Überschrift ohne
// alles darunter sagt weniger als gar nichts.
func TestLeereGruppeStehtNichtInDerListe(t *testing.T) {
	if list := List([]Def{gruppe()}, Data{}, Links{}); len(list) != 0 {
		t.Errorf("Liste = %#v, want leer", list)
	}
}

// Ein Verweis wird über die Nachschlagefunktion aufgelöst — die entscheidet,
// ob die Zielseite überhaupt gezeigt werden darf.
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

// Gelöscht, verschoben oder noch ein Entwurf: das Theme bekommt nichts, nicht
// einen Link ins Leere. Das ist dieselbe Regel wie beim Bild.
func TestVerweisAufNichtSichtbaresWirdNil(t *testing.T) {
	defs := []Def{{Key: "produkt", Kind: KindRef}}
	got := Resolve(defs, Data{Values: Values{"produkt": "42"}}, Links{
		Page: func(int64) (Ref, bool) { return Ref{}, false },
	})
	if ref := got["produkt"].(*Ref); ref != nil {
		t.Errorf("produkt = %#v, want nil", ref)
	}
	// Und ohne Nachschlagefunktion — etwa im Export — genauso.
	got = Resolve(defs, Data{Values: Values{"produkt": "42"}}, Links{})
	if ref := got["produkt"].(*Ref); ref != nil {
		t.Errorf("ohne Lookup: produkt = %#v, want nil", ref)
	}
}

// Was aus dem Formular kommt, ist eine Zahl oder es ist nichts.
func TestVerweisPruefung(t *testing.T) {
	d := Def{Label: "Produkt", Kind: KindRef}
	if reason := Check(d, "17"); !reason.Empty() {
		t.Errorf("Check(17) = %q, want nichts", reason)
	}
	for _, bad := range []string{"/eine-seite", "0", "-3", "abc"} {
		if Check(d, bad).Empty() {
			t.Errorf("Check(%q) hat nichts zu beanstanden, sollte aber", bad)
		}
	}
}

// Ein Feld kann zu einer eigenen Inhaltsart gehören — ein Preis gehört an ein
// Produkt und an sonst nichts.
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
		t.Errorf("für ein Produkt: %v", got)
	}
	// Ein Produkt ist technisch eine Seite. Ein Feld "nur Seiten" darf trotzdem
	// nicht im Produktformular auftauchen.
	if got := keys("page"); len(got) != 2 || got[0] != "hinweis" || got[1] != "notiz" {
		t.Errorf("für eine Seite: %v", got)
	}
	if got := keys("post"); len(got) != 2 || got[0] != "autor" {
		t.Errorf("für einen Beitrag: %v", got)
	}
}

// Ein bedingtes Feld wird nicht verlangt, solange es niemand sieht. Ein
// Pflichtfeld, das unsichtbar blockiert, ist der Fehler, den diese Prüfung
// verhindern soll.
func TestBedingtesPflichtfeldBlockiertNicht(t *testing.T) {
	defs := []Def{
		{Key: "angebot", Label: "Im Angebot", Kind: KindBool},
		{Key: "sonderpreis", Label: "Sonderpreis", Kind: KindNumber, Required: true, Condition: "angebot"},
	}

	leer := Data{Values: Values{}}
	if errs := CheckAll(defs, leer); len(errs) != 0 {
		t.Errorf("ohne Häkchen wird gemeckert: %v", errs)
	}

	an := Data{Values: Values{"angebot": "1"}}
	if errs := CheckAll(defs, an); len(errs) != 1 || errs["sonderpreis"].Empty() {
		t.Errorf("mit Häkchen fehlt die Meldung: %v", errs)
	}
}

// Der Wert bleibt stehen, wird aber nicht ausgegeben, solange die Bedingung
// nicht erfüllt ist: ein versehentlich entferntes Häkchen darf niemandem seine
// Eingabe kosten, und das Theme darf trotzdem nichts davon zeigen.
func TestBedingterWertBleibtUndWirktNicht(t *testing.T) {
	defs := []Def{
		{Key: "angebot", Label: "Im Angebot", Kind: KindBool},
		{Key: "sonderpreis", Label: "Sonderpreis", Kind: KindNumber, Condition: "angebot"},
	}
	d := Data{Values: Values{"sonderpreis": "9.50"}}

	if got := Resolve(defs, d, Links{})["sonderpreis"].(Number).Raw; got != "" {
		t.Errorf("das Theme sieht %q statt nichts", got)
	}
	if got := List(defs, d, Links{}); len(got) != 0 {
		t.Errorf("die Feldliste zeigt %d Einträge", len(got))
	}
	// Aufräumen wirft ihn nicht weg.
	if got := Clean(defs, d).Values["sonderpreis"]; got != "9.50" {
		t.Errorf("der Wert ist weg: %q", got)
	}
	// Und mit Häkchen ist er wieder da.
	d.Values["angebot"] = "1"
	if got := Resolve(defs, d, Links{})["sonderpreis"].(Number).Raw; got != "9.50" {
		t.Errorf("mit Häkchen fehlt der Wert: %q", got)
	}
}

// Eine Kette: C hängt an B, B hängt an A. Fällt A weg, fallen beide weg — und
// zwar unabhängig davon, in welcher Reihenfolge die Felder stehen.
func TestBedingungsketteFaelltGanz(t *testing.T) {
	defs := []Def{
		{Key: "c", Kind: KindText, Condition: "b"},
		{Key: "b", Kind: KindText, Condition: "a"},
		{Key: "a", Kind: KindText},
	}
	hidden := Hidden(defs, Values{"b": "x", "c": "y"})
	if !hidden["b"] || !hidden["c"] {
		t.Errorf("die Kette hält nicht: %v", hidden)
	}
	if hidden = Hidden(defs, Values{"a": "1", "b": "x", "c": "y"}); len(hidden) != 0 {
		t.Errorf("mit ausgefülltem a ist noch etwas versteckt: %v", hidden)
	}
}

// Eine Bedingung, die auf ein Feld zeigt, das es nicht gibt, ist keine: sonst
// wäre das Feld für immer unerreichbar und niemand sähe, warum.
func TestBedingungInsLeereZeigtNichts(t *testing.T) {
	defs := []Def{{Key: "preis", Kind: KindNumber, Condition: "gibtsnicht"}}
	if hidden := Hidden(defs, Values{}); len(hidden) != 0 {
		t.Errorf("versteckt: %v", hidden)
	}
}

// Ein Abschnitt ist eine Überschrift: kein Wert, keine Prüfung, nichts im
// Theme.
func TestAbschnittHatKeinenWert(t *testing.T) {
	defs := []Def{
		{Key: "masse", Label: "Masse", Kind: KindSection, Required: true},
		{Key: "hoehe", Label: "Höhe", Kind: KindNumber},
	}
	if errs := CheckAll(defs, Data{Values: Values{}}); len(errs) != 0 {
		t.Errorf("eine Überschrift wird geprüft: %v", errs)
	}
	resolved := Resolve(defs, Data{Values: Values{"masse": "irgendwas"}}, Links{})
	if _, da := resolved["masse"]; da {
		t.Error("die Überschrift steht im Theme")
	}
	if got := Clean(defs, Data{Values: Values{"masse": "irgendwas"}}).Values["masse"]; got != "" {
		t.Errorf("die Überschrift hat einen Wert behalten: %q", got)
	}
}

// Woran eine Bedingung hängen darf, entscheidet der Browser: was er nicht als
// "ausgefüllt" erkennen kann, wird gar nicht erst angeboten.
func TestWoranEineBedingungHaengenDarf(t *testing.T) {
	// KindRange bleibt bewusst dabei: die Notiz im Fahrplan, es auszuschliessen,
	// ruhte auf der Annahme eines Schiebers. Ein Zahlenfeld zeigt sehr wohl
	// einen Platzhalter — beobachtet im Browserdurchgang zu Plan 07-07 und
	// nicht bloss erschlossen: das abhängige Feld war verborgen, solange das
	// Zahlenfeld leer war, und sichtbar, sobald eine Zahl darinstand.
	darf := []string{KindText, KindLong, KindNumber, KindBool, KindChoice,
		KindImage, KindLink, KindRef, KindRange, KindCode}
	for _, k := range darf {
		if !(Def{Kind: k}).MayControl() {
			t.Errorf("%s sollte eine Bedingung tragen dürfen", k)
		}
	}
	// KindTime steht aus demselben Grund draussen wie KindDate: ein
	// <input type="time"> trifft :placeholder-shown nie, die Regel, die die
	// abhängigen Felder ausblendet, könnte also nie greifen.
	for _, k := range []string{KindDate, KindTime, KindGroup, KindSection} {
		if (Def{Kind: k}).MayControl() {
			t.Errorf("%s sollte keine Bedingung tragen dürfen", k)
		}
	}
	// Ein Feld in einer Gruppe auch nicht: dort wird eine Zeile als Ganzes
	// ausgefüllt.
	if (Def{Kind: KindBool, ParentID: 3}).MayControl() {
		t.Error("ein Feld in einer Gruppe sollte keine Bedingung tragen dürfen")
	}
}

// --- Die drei kleinen Arten: zeit, bereich, code -------------------------
//
// Was hier steht, ist der ganze Vertrag der drei Arten. Die Grenzfälle sind
// die eigentliche Arbeit: die Grenze selbst gehört noch dazu, ein Schritt
// daneben nicht mehr, und leer ist nie null.

// Eine Uhrzeit ist eine Uhrzeit und keine Zeichenkette, die zufällig einen
// Doppelpunkt enthält. Der Browser schickt je nach Gerät mit oder ohne
// Sekunden — beides muss durch.
func TestZeitPruefung(t *testing.T) {
	zeit := Def{Label: "Abfahrt", Kind: KindTime}
	for _, gut := range []string{"09:30", "00:00", "23:59", "09:30:00"} {
		if r := Check(zeit, gut); !r.Empty() {
			t.Errorf("%q abgelehnt: %q", gut, r)
		}
	}
	// 25:00 gibt es nicht; 9:30 ohne führende Null ist nicht, was ein
	// <input type="time"> sendet, und wer es von Hand einträgt, soll es
	// merken statt eine still zurechtgebogene Zeit zu bekommen.
	for _, schlecht := range []string{"25:00", "9:30", "halb zehn", "09:30+02:00", "2026-04-01"} {
		if r := Check(zeit, schlecht); r.Empty() {
			t.Errorf("%q durchgelassen", schlecht)
		}
	}
	// Leer ist erlaubt, solange das Feld kein Pflichtfeld ist.
	if r := Check(zeit, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
	if r := Check(Def{Label: "Abfahrt", Kind: KindTime, Required: true}, ""); r.Empty() {
		t.Error("leeres Pflichtfeld angenommen")
	}
}

// Mitternacht und „nichts eingetragen“ sind zwei verschiedene Tatsachen. Ein
// Zeiger kann sie auseinanderhalten, ein time.Time nicht.
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
		t.Errorf("ankunft = %#v, wollte nil — leer ist nicht Mitternacht", got["ankunft"])
	}

	// Keine Zeitzone: die Uhrzeit wird ohne Datum gelesen, sie trägt also
	// keinen Versatz und kein Datum, das jemand für einen Tag halten könnte.
	mittag := Resolve([]Def{{Key: "t", Kind: KindTime}}, Data{Values: Values{"t": "12:15"}}, Links{})
	tz := mittag["t"].(*time.Time)
	if _, versatz := tz.Zone(); versatz != 0 {
		t.Errorf("die Uhrzeit trägt einen Versatz von %d Sekunden", versatz)
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

// Ein Datum überlässt das Theme seinem formatDate, eine Uhrzeit hat keinen
// solchen Helfer — also steht sie als Text da, sonst druckt eine Liste von
// Beschriftungen neben der Uhrzeit nichts.
func TestZeitStehtAlsTextInDerListe(t *testing.T) {
	defs := []Def{{Key: "abfahrt", Label: "Abfahrt", Kind: KindTime},
		{Key: "wurf", Label: "Wurf", Kind: KindDate}}
	list := List(defs, Data{Values: Values{"abfahrt": "09:30", "wurf": "2026-04-01"}}, Links{})
	if len(list) != 2 {
		t.Fatalf("%d Einträge, wollte 2: %+v", len(list), list)
	}
	if list[0].Text != "09:30" {
		t.Errorf("Text der Uhrzeit = %q, wollte \"09:30\"", list[0].Text)
	}
	if _, ok := list[0].Value.(*time.Time); !ok {
		t.Errorf("Value der Uhrzeit = %#v, wollte *time.Time", list[0].Value)
	}
	// Das Datum bleibt, wie es war: leerer Text, das Theme formatiert selbst.
	if list[1].Text != "" {
		t.Errorf("das Datum trägt jetzt Text %q — das war nicht die Absicht", list[1].Text)
	}
}

// Die Grenzen eines Bereichsfeldes, an jeder Kante einzeln nachgemessen. Die
// Grenze selbst ist ein gültiger Wert; ein Schritt daneben ist es nicht.
func TestBereichPruefung(t *testing.T) {
	faelle := []struct {
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
	for _, f := range faelle {
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
					// Keine Zahl ist keine Grenzverletzung, sondern etwas
					// anderes — die Begründung sagt das auch so.
					if !strings.Contains(r.String(), "Zahl") {
						t.Errorf("die Begründung zu %q nennt die Zahl nicht: %q", schlecht, r)
					}
					continue
				}
				// Die Begründung ist für die Person am Formular: sie nennt die
				// Grenzen, zwischen denen der Wert liegen müsste.
				genannt := (f.unten != "" && strings.Contains(r.String(), f.unten)) ||
					(f.obn != "" && strings.Contains(r.String(), f.obn))
				if !genannt {
					t.Errorf("die Begründung zu %q nennt keine Grenze: %q", schlecht, r)
				}
			}
		})
	}
}

// Leer ist nicht null und nicht die untere Grenze: ein Bereichsfeld, das
// niemand ausgefüllt hat, ist nicht ausgefüllt.
func TestLeererBereichIstNichtNull(t *testing.T) {
	kann := Def{Key: "menge", Label: "Menge", Kind: KindRange, RangeMin: "1", RangeMax: "10"}
	if r := Check(kann, ""); !r.Empty() {
		t.Errorf("leeres Kannfeld abgelehnt: %q", r)
	}
	pflicht := kann
	pflicht.Required = true
	if r := Check(pflicht, ""); r.Empty() {
		t.Error("leeres Pflichtfeld angenommen")
	} else if !strings.Contains(r.String(), "ausgefüllt") {
		t.Errorf("die Begründung ist nicht die übliche Pflichtmeldung: %q", r)
	}

	got := Resolve([]Def{kann}, Data{}, Links{})
	n, ok := got["menge"].(Number)
	if !ok {
		t.Fatalf("menge = %#v, wollte Number", got["menge"])
	}
	if n.Raw != "" || n.Value != 0 {
		t.Errorf("leerer Bereich = %#v, wollte den Nullwert mit leerem Raw", n)
	}
	if len(List([]Def{kann}, Data{}, Links{})) != 0 {
		t.Error("ein leerer Bereich steht in der Liste")
	}
	if Filled(got) {
		t.Error("Filled meldet Inhalt, wo keiner ist")
	}
}

// Gedruckt wird, was getippt wurde. 0.1 ist 0.1 und nicht 0.10000000000000001.
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
		t.Errorf("Liste = %+v, wollte einen Eintrag mit Text \"0.1\"", list)
	}
}

// Ein Codefeld ist Text, wie er getippt wurde — kein Markdown, keine
// Umdeutung. Und ein leeres bleibt aus der Liste heraus.
func TestCodeIstRoherText(t *testing.T) {
	d := Def{Key: "schnipsel", Label: "Schnipsel", Kind: KindCode}
	roh := "<b>fett</b> & \"Anführung\"\n  eingerückt"
	got := Resolve([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})
	if got["schnipsel"] != roh {
		t.Errorf("schnipsel = %#v, wollte den rohen Text unverändert", got["schnipsel"])
	}
	// Und ausdrücklich als string und nicht als template.HTML: ein Theme
	// bekommt einen Wert, den html/template beim Drucken maskiert. Eine
	// Umtypung irgendwo auf diesem Weg wäre genau die Lücke, die FIELD-06
	// schliesst — sie fiele sonst nirgends auf, weil sich beide gleich
	// ausdrucken.
	if _, istString := got["schnipsel"].(string); !istString {
		t.Errorf("schnipsel ist %T und kein string — wird es noch maskiert?", got["schnipsel"])
	}
	if e := List([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})[0]; e.Kind != KindCode {
		t.Errorf("der Eintrag nennt seine Art als %q", e.Kind)
	}
	// Check nimmt jeden Text an: es gibt keine falsche Zeile Code.
	if r := Check(d, roh); !r.Empty() {
		t.Errorf("Code abgelehnt: %q", r)
	}
	list := List([]Def{d}, Data{Values: Values{"schnipsel": roh}}, Links{})
	if len(list) != 1 || list[0].Text != roh {
		t.Errorf("Liste = %+v, wollte einen Eintrag mit dem rohen Text", list)
	}
	if len(List([]Def{d}, Data{}, Links{})) != 0 {
		t.Error("ein leeres Codefeld steht in der Liste")
	}
}

// Die beiden Listen sind abziehend: eine neue Art ist drin, solange sie nicht
// ausdrücklich ausgeschlossen wird. Für alle drei ist das richtig — und für
// code ist es die Bedingung, unter der Erfolgskriterium 5 überhaupt gestellt
// werden kann.
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
			t.Errorf("%s steht nicht in Kinds", art)
		}
		if !enthaelt(SubKinds(), art) {
			t.Errorf("%s fehlt in SubKinds", art)
		}
		if !enthaelt(BlockKinds(), art) {
			t.Errorf("%s fehlt in BlockKinds", art)
		}
		if KindName(art) == art {
			t.Errorf("%s hat keine Beschriftung", art)
		}
	}
}

// --- Die zwei Wächter eines mehrwertigen Feldes -------------------------
//
// Ein Häkchenfeld kann in der Auszeichnung nicht begrenzt werden — dafür
// bräuchte es JavaScript, und davon trägt dieses Programm nichts ausser htmx.
// Die Höchstzahl gilt also hier, auf dem Server, oder nirgends.

func TestMehrfachauswahlHoechstzahl(t *testing.T) {
	auswahl := []string{"Eiche", "Buche", "Esche", "Erle"}
	faelle := []struct {
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
	for _, f := range faelle {
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
				// Die Begründung ist für die Person am Formular: sie nennt das
				// Feld und die Zahl, auf die es ankommt.
				if !strings.Contains(r.String(), "Sorten") {
					t.Errorf("die Begründung zu %q nennt das Feld nicht: %q", schlecht, r)
				}
				if f.max > 1 && !strings.Contains(r.String(), strconv.Itoa(f.max)) {
					t.Errorf("die Begründung zu %q nennt die Höchstzahl nicht: %q", schlecht, r)
				}
			}
		})
	}
}

// Genau die Höchstzahl geht noch, eine mehr nicht. Der Rand ist die ganze
// Frage — eine Grenze, bei der man raten muss, ob sie noch dazugehört, ist
// keine.
func TestMehrfachauswahlGenauAmRand(t *testing.T) {
	d := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti,
		Choices: []string{"a", "b", "c"}, MaxValues: 3}
	if r := Check(d, "a\nb\nc"); !r.Empty() {
		t.Errorf("genau drei abgelehnt: %q", r)
	}
	// Doppelte zählen einzeln: JoinValues bewahrt sie, also sind es vier
	// Werte, auch wenn nur drei verschiedene darunter sind.
	if r := Check(d, "a\nb\nc\na"); r.Empty() {
		t.Error("vier Werte durchgelassen, obwohl höchstens drei erlaubt sind")
	}
}

// MaxValueBytes ist der Platz für alle Werte eines Feldes zusammen, beim
// mehrwertigen einschliesslich der Zeilenumbrüche dazwischen. Genau so lang
// geht noch, ein Byte mehr nicht — und nichts wird dabei gekürzt.
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
		t.Errorf("die Begründung nennt das Feld nicht: %q", r)
	}
	if !strings.Contains(r.String(), strconv.Itoa(MaxValueBytes)) {
		t.Errorf("die Begründung nennt die Grenze nicht: %q", r)
	}

	// Gezählt werden Byte, nicht Zeichen: ein Umlaut braucht zwei davon, also
	// ist die Grenze bei halb so vielen Zeichen erreicht. Ein Runen- oder
	// Graphemzähler wäre eine andere Zahl und würde die Datenbank überziehen.
	umlaute := strings.Repeat("ä", MaxValueBytes/2)
	if len([]rune(umlaute)) >= MaxValueBytes {
		t.Fatalf("der Prüffall taugt nicht: %d Runen", len([]rune(umlaute)))
	}
	if r := Check(text, umlaute); !r.Empty() {
		t.Errorf("genau %d Byte aus Umlauten abgelehnt: %q", len(umlaute), r)
	}
	if r := Check(text, umlaute+"ä"); r.Empty() {
		t.Errorf("%d Byte aus Umlauten durchgelassen", len(umlaute)+2)
	}
}

// Beim mehrwertigen Feld wird die verbundene Zeichenkette gemessen, nicht der
// längste einzelne Wert: drei Werte zu je zwei Dritteln des Platzes passen
// nicht nebeneinander, auch wenn jeder für sich passt.
func TestBytebudgetGiltAllenWertenZusammen(t *testing.T) {
	kurz := strings.Repeat("a", MaxValueBytes/2)
	lang := kurz + "a"
	d := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Choices: []string{kurz, lang}}

	if r := Check(d, kurz); !r.Empty() {
		t.Errorf("ein Wert von %d Byte abgelehnt: %q", len(kurz), r)
	}
	// Zwei Werte zu je MaxValueBytes/2 plus der Umbruch dazwischen: ein Byte
	// über der Grenze. Der Umbruch zählt mit, sonst ginge in die Datenbank
	// mehr, als sie zugesagt bekommt.
	zusammen := JoinValues([]string{kurz, kurz})
	if len(zusammen) != MaxValueBytes+1 {
		t.Fatalf("der Prüffall taugt nicht: %d Byte", len(zusammen))
	}
	if r := Check(d, zusammen); r.Empty() {
		t.Errorf("%d Byte verbunden durchgelassen — gemessen wurde offenbar der längste einzelne Wert", len(zusammen))
	}
}

// Nichts auf dem Speicherweg kürzt noch. Ein zu langer Wert wird gemeldet,
// nicht halbiert: ein halbierter Wert sieht aus wie einer, den jemand so
// getippt hat, und beim mehrwertigen Feld wäre die Hälfte eines Wertes ein
// Wert, den es nie gab (D-13).
func TestNichtsWirdMehrStillGekuerzt(t *testing.T) {
	zuLang := strings.Repeat("a", MaxValueBytes+50)

	d := Def{Key: "notiz", Label: "Notiz", Kind: KindLong}
	sauber := Clean([]Def{d}, Data{Values: Values{"notiz": "  " + zuLang + "  "}})
	if got := sauber.Values["notiz"]; got != zuLang {
		t.Errorf("Clean legte %d Byte ab, wollte %d — es wird noch gekürzt", len(got), len(zuLang))
	}

	// In einer Gruppenzeile ebenso.
	g := Def{Key: "staffel", Label: "Staffel", Kind: KindGroup, Sub: []Def{d}}
	zeilen := Clean([]Def{g}, Data{Rows: map[string][]Values{"staffel": {{"notiz": zuLang}}}})
	if got := zeilen.Rows["staffel"][0]["notiz"]; got != zuLang {
		t.Errorf("in der Zeile wurden %d Byte abgelegt, wollte %d", len(got), len(zuLang))
	}

	// Und CheckAll meldet ihn, unter der Kennung des Feldes, damit das
	// Formular die Begründung unter dem richtigen Feld zeigt.
	errs := CheckAll([]Def{d}, Data{Values: Values{"notiz": zuLang}})
	if errs["notiz"].Empty() {
		t.Errorf("CheckAll meldet den zu langen Wert nicht: %v", errs)
	}
}

// Ein Feld, dessen Bedingung nicht erfüllt ist, wird gar nicht geprüft —
// beim mehrwertigen genauso wie bei jedem anderen. Etwas zu verlangen, das
// die Person nicht sehen kann, ist der eine Weg, auf dem sich ein Formular
// nicht abschicken lässt, ohne zu sagen warum.
//
// Wo die Grenze liegt: jede Artregel wird für ein verstecktes Feld
// übersprungen — die Pflicht, die Höchstzahl, die geschlossene Möglichkeitenliste.
// Die Bytegrenze nicht: sie ist keine Frage an irgendwen, sondern sagt, wieviel
// Platz ein Wert in der Zeile hat, die geschrieben wird. Die bewacht
// TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden.
func TestVerstecktesMehrwertigesFeldWirdNichtGeprueft(t *testing.T) {
	schalter := Def{Key: "spezial", Label: "Spezial", Kind: KindBool}
	sorten := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Required: true,
		Choices: []string{"Eiche", "Buche"}, MaxValues: 1, Condition: "spezial"}
	defs := []Def{schalter, sorten}

	// Über jeder Artgrenze zugleich: drei Werte bei MaxValues 1, „Ahorn“ steht
	// nicht auf der Liste, und das Feld ist Pflicht. Deutlich unter der
	// Bytegrenze, damit ein Fehler hier eindeutig heisst, dass eine Artregel
	// gefeuert hat — und nicht die Länge.
	uebervoll := JoinValues([]string{"Eiche", "Buche", "Ahorn"})

	aus := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": uebervoll}})
	if len(aus) != 0 {
		t.Errorf("ein verstecktes Feld wurde geprüft: %v", aus)
	}

	an := CheckAll(defs, Data{Values: Values{"spezial": "1", "sorten": uebervoll}})
	if an["sorten"].Empty() {
		t.Errorf("das sichtbare Feld wurde nicht geprüft: %v", an)
	}
}

// --- Das Schlagwortfeld -------------------------------------------------
//
// Es speichert die Adresse des Schlagworts und druckt dessen Namen. Wird das
// Schlagwort umbenannt, ändert sich, was jede Seite zeigt, ohne dass eine
// einzige Seite angefasst wird. Genau dieses Versprechen ist auch der Grund,
// warum die Art in keiner Bausteinart stehen darf: ein Baustein friert beim
// Speichern zu HTML ein und könnte es nicht halten.

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
		t.Errorf("%s steht nicht in Kinds", KindTerm)
	}
	if KindName(KindTerm) == KindTerm {
		t.Errorf("%s hat keine Beschriftung", KindTerm)
	}
	// In einer Gruppe ist es erlaubt: eine Gruppe friert nichts ein.
	if !enthaelt(SubKinds(), KindTerm) {
		t.Errorf("%s fehlt in SubKinds", KindTerm)
	}
	if enthaelt(BlockKinds(), KindTerm) {
		t.Errorf("%s steht in BlockKinds, sollte aber nicht", KindTerm)
	}

	// Genau zwei Arten fehlen dort neben Gruppe und Abschnitt — der Verweis
	// und das Schlagwort. Der Filter ist abziehend: eine Art, die hier
	// versehentlich dazukäme, verschwände lautlos aus jedem Bausteinformular.
	fehlend := map[string]bool{}
	for _, k := range Kinds {
		fehlend[k.Kind] = true
	}
	for _, k := range BlockKinds() {
		delete(fehlend, k.Kind)
	}
	wollte := map[string]bool{KindGroup: true, KindSection: true, KindRef: true, KindTerm: true}
	if len(fehlend) != len(wollte) {
		t.Errorf("BlockKinds lässt %v aus, wollte %v", fehlend, wollte)
	}
	for art := range wollte {
		if !fehlend[art] {
			t.Errorf("BlockKinds enthält %s, sollte es aber auslassen", art)
		}
	}
	// Ein Codefeld darf in einem Baustein stehen (D-06); es ist die Art, die
	// beim Ausschluss am ehesten mitgerissen würde.
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
	// Der Name von jetzt, nicht der von damals: gespeichert ist „moebel“,
	// gedruckt wird „Möbelbau“.
	if term.Name != "Möbelbau" || term.Slug != "moebel" || term.URL != "/tag/moebel" {
		t.Errorf("term = %#v", term)
	}
}

// Die vier Wege ins Nichts. Jeder ergibt denselben getippten nil, damit ein
// {{with}} im Theme den Block auslässt statt einen alten Namen zu drucken.
func TestSchlagwortOhneTrefferWirdNil(t *testing.T) {
	defs := []Def{{Key: "thema", Kind: KindTerm}}
	treffer := func(slug string) (Term, bool) {
		if slug == "moebel" {
			return Term{Name: "Möbelbau", Slug: "moebel", URL: "/tag/moebel"}, true
		}
		return Term{}, false
	}
	faelle := []struct {
		name  string
		wert  string
		links Links
	}{
		{"kein Wert", "", Links{Term: treffer}},
		{"ohne Nachschlagefunktion", "moebel", Links{}},
		{"gelöschtes Schlagwort", "verschwunden", Links{Term: treffer}},
		// Ein Kürzel ist die kleingeschriebene Fassung eines Namens. Wird
		// hier gefaltet, fielen zwei verschiedene Schlagwörter zusammen.
		{"andere Schreibung", "Moebel", Links{Term: treffer}},
	}
	for _, f := range faelle {
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
	// Der Name, nicht das Kürzel: eine Liste aus Beschriftung und Wert soll
	// „Möbelbau“ zeigen und nicht „moebel“.
	if e.Text != "Möbelbau" {
		t.Errorf("Text = %q, wollte den Namen", e.Text)
	}

	if !Filled(Resolve(defs, daten, links)) {
		t.Error("Filled meldet nichts, obwohl ein Schlagwort aufgelöst ist")
	}
	// Und ohne Treffer ist die Seite leer — sonst verschwände nur der Text
	// und die Tafel bliebe stehen.
	if Filled(Resolve(defs, Data{Values: Values{"weg": "verschwunden"}}, links)) {
		t.Error("Filled meldet etwas, obwohl kein Schlagwort aufgelöst ist")
	}
}

// Was aus dem Formular kommt, ist ein Kürzel oder es ist jemand, der von Hand
// ins Formular tippt.
func TestSchlagwortPruefung(t *testing.T) {
	d := Def{Label: "Thema", Kind: KindTerm}
	for _, gut := range []string{"moebel", "moebel-nach-mass", "holz2024"} {
		if reason := Check(d, gut); !reason.Empty() {
			t.Errorf("Check(%q) = %q, want nichts", gut, reason)
		}
	}
	for _, bad := range []string{"Moebel", "moebel nach mass", "möbel", "/tag/moebel", "-moebel"} {
		if Check(d, bad).Empty() {
			t.Errorf("Check(%q) hat nichts zu beanstanden, sollte aber", bad)
		}
	}
}

// Die Bytegrenze gilt auch dort, wo nach dem Wert niemand gefragt hat.
//
// Clean behält den Wert eines versteckten Feldes ausdrücklich (field.go:568-573),
// und seit D-13 kürzt trimTo nichts mehr — CheckAll ist damit die einzige
// Stelle, an der das Bytebudget überhaupt noch gilt. Ging es hier vorbei, gab
// es die Grenze für diese Klasse nirgends: der Browser schickt den Wert eines
// versteckten Feldes mit, und wer das Formular von Hand baut, sowieso.
func TestVerstecktesFeldBleibtAnDieBytegrenzeGebunden(t *testing.T) {
	schalter := Def{Key: "spezial", Label: "Spezial", Kind: KindBool}
	sorten := Def{Key: "sorten", Label: "Sorten", Kind: KindMulti, Required: true,
		Choices: []string{"Eiche", "Buche"}, MaxValues: 1, Condition: "spezial"}
	defs := []Def{schalter, sorten}

	zuLang := strings.Repeat("x", MaxValueBytes+1)
	aus := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": zuLang}})
	if aus["sorten"].Empty() {
		t.Fatalf("der zu lange Wert eines versteckten Feldes wurde nicht gemeldet: %v", aus)
	}
	// Und zwar mit der Längenbegründung, nicht mit der Optionsbegründung: eine
	// Artregel darf hier nicht zurückgeschmuggelt worden sein.
	if !strings.Contains(aus["sorten"].String(), "zu lang") {
		t.Errorf("die Begründung ist nicht die der Länge: %q", aus["sorten"])
	}

	// Die eigentliche Zusage dieser Änderung: nur Artregeln verletzt, und das
	// versteckte Feld bleibt still. Pflicht, Höchstzahl und geschlossene
	// Liste gelten weiterhin nicht für ein Feld, das niemand sieht.
	nurArt := JoinValues([]string{"Eiche", "Buche", "Ahorn"})
	if still := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": nurArt}}); len(still) != 0 {
		t.Errorf("eine Artregel feuerte für ein verstecktes Feld: %v", still)
	}
	// Auch der leere Pflichtwert bleibt still.
	if still := CheckAll(defs, Data{Values: Values{"spezial": "", "sorten": ""}}); len(still) != 0 {
		t.Errorf("die Pflicht feuerte für ein verstecktes Feld: %v", still)
	}
}

// Dasselbe Loch eine Ebene tiefer, mit MaxRows Zeilen mal Unterfeldern als
// Hebel: validate leert die Bedingung nur für ein Feld in einer Gruppe oder in
// einer Bausteinart (store.go:521-523), eine Gruppe auf oberster Ebene darf
// also eine tragen.
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
		t.Fatalf("der zu lange Wert in der Zeile einer versteckten Gruppe wurde nicht gemeldet: %v", aus)
	}
	if !strings.Contains(aus[schluessel].String(), "zu lang") {
		t.Errorf("die Begründung ist nicht die der Länge: %q", aus[schluessel])
	}

	// Gegenprobe: eine versteckte Pflichtgruppe ohne Zeile bleibt still, und
	// ein leeres Pflichtunterfeld in ihrer Zeile ebenso.
	if still := CheckAll(defs, Data{Values: Values{"spezial": ""}}); len(still) != 0 {
		t.Errorf("„braucht mindestens eine Zeile“ feuerte für eine versteckte Gruppe: %v", still)
	}
	if still := CheckAll(defs, Data{
		Values: Values{"spezial": ""},
		Rows:   map[string][]Values{"staffel": {{"notiz": ""}}},
	}); len(still) != 0 {
		t.Errorf("die Pflicht feuerte in der Zeile einer versteckten Gruppe: %v", still)
	}
}
