package field

import (
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
		if reason := Check(d, gut); reason != "" {
			t.Errorf("Check(%q) = %q, erwartet in Ordnung", gut, reason)
		}
	}
	for _, schlecht := range []string{
		"javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,<script>",
		"//example.ch/fremd", "hofladen", "vbscript:msgbox",
	} {
		if reason := Check(d, schlecht); reason == "" {
			t.Errorf("Check(%q) wurde durchgelassen", schlecht)
		}
	}
}

// Die Prüfungen sagen, was der Redakteur ändern soll — nicht, was Go gemeldet hat.
func TestPruefungen(t *testing.T) {
	zahl := Def{Label: "Preis", Kind: KindNumber}
	if r := Check(zahl, "8.50"); r != "" {
		t.Errorf("8.50 abgelehnt: %q", r)
	}
	// Ein Komma ist, was jemand mit einer deutschen Tastatur tippt.
	if r := Check(zahl, "8,50"); r != "" {
		t.Errorf("8,50 abgelehnt: %q", r)
	}
	if r := Check(zahl, "acht"); r == "" {
		t.Error("„acht“ als Zahl durchgelassen")
	}

	datum := Def{Label: "Wurfdatum", Kind: KindDate}
	if r := Check(datum, "2026-04-01"); r != "" {
		t.Errorf("Datum abgelehnt: %q", r)
	}
	if r := Check(datum, "01.04.2026"); r == "" {
		t.Error("Datum im falschen Format durchgelassen")
	}

	auswahl := Def{Label: "Zustand", Kind: KindChoice, Choices: []string{"frisch", "vergriffen"}}
	if r := Check(auswahl, "frisch"); r != "" {
		t.Errorf("gültige Auswahl abgelehnt: %q", r)
	}
	// Der wichtigste Fall: das <select> lässt sich umgehen, die Prüfung nicht.
	if r := Check(auswahl, "erfunden"); r == "" {
		t.Error("eine Möglichkeit, die es nicht gibt, wurde angenommen")
	}

	pflicht := Def{Label: "Preis", Kind: KindText, Required: true}
	if r := Check(pflicht, "   "); r == "" {
		t.Error("Leerzeichen als Pflichtangabe angenommen")
	}
	if r := Check(Def{Label: "Preis", Kind: KindText}, ""); r != "" {
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
// {{ .Page.Felder.preis }} auf der einen Seite, die keinen Preis hat.
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
	if !strings.Contains(reason, "Zeile 2") {
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
	if reason := Check(d, "17"); reason != "" {
		t.Errorf("Check(17) = %q, want nichts", reason)
	}
	for _, bad := range []string{"/eine-seite", "0", "-3", "abc"} {
		if Check(d, bad) == "" {
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
	if errs := CheckAll(defs, an); len(errs) != 1 || errs["sonderpreis"] == "" {
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
	darf := []string{KindText, KindLong, KindNumber, KindBool, KindChoice, KindImage, KindLink, KindRef}
	for _, k := range darf {
		if !(Def{Kind: k}).MayControl() {
			t.Errorf("%s sollte eine Bedingung tragen dürfen", k)
		}
	}
	for _, k := range []string{KindDate, KindGroup, KindSection} {
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
