package field

import (
	"reflect"
	"strings"
	"testing"
)

// SplitValues und JoinValues sind das eine Paar, über das jeder mehrwertige
// Feldwert läuft — das Seitenformular, die Auflösung fürs Theme und die Reise
// durchs Archiv. Alles, was hier steht, ist die Zusage an Phase 9: der
// Einleser erbt diese beiden Funktionen, statt eine dritte Schreibweise zu
// erfinden.
func TestSplitValues(t *testing.T) {
	fälle := []struct {
		name string
		roh  string
		will []string
	}{
		{"zwei Zeilen", "eiche\nbuche", []string{"eiche", "buche"}},
		{"nichts", "", nil},
		{"nur Leerraum", "  \n\n\t", nil},
		{"leere Zeilen fallen weg", "a\n\n b ", []string{"a", "b"}},
		{"Reihenfolge bleibt", "esche\nbuche\neiche", []string{"esche", "buche", "eiche"}},
		{"Doppelte bleiben doppelt", "a\na", []string{"a", "a"}},
		{"eine einzige Zeile", "eiche", []string{"eiche"}},
	}
	for _, f := range fälle {
		t.Run(f.name, func(t *testing.T) {
			if got := SplitValues(f.roh); !reflect.DeepEqual(got, f.will) {
				t.Errorf("SplitValues(%q) = %#v, wollte %#v", f.roh, got, f.will)
			}
		})
	}
}

func TestJoinValues(t *testing.T) {
	fälle := []struct {
		name  string
		werte []string
		will  string
	}{
		{"nichts", nil, ""},
		{"leere Liste", []string{}, ""},
		{"leere Einträge fallen weg", []string{"", "a", ""}, "a"},
		{"nur leere Einträge", []string{"", "", ""}, ""},
		{"Doppelte bleiben doppelt", []string{"a", "a"}, "a\na"},
		{"Reihenfolge bleibt, kein Sortieren", []string{"esche", "buche", "eiche"}, "esche\nbuche\neiche"},
		{"wird beschnitten", []string{" eiche ", "buche"}, "eiche\nbuche"},
	}
	for _, f := range fälle {
		t.Run(f.name, func(t *testing.T) {
			if got := JoinValues(f.werte); got != f.will {
				t.Errorf("JoinValues(%#v) = %q, wollte %q", f.werte, got, f.will)
			}
		})
	}
}

// Die eigentliche Zusage: die beiden sind Umkehrungen voneinander, solange
// kein Eintrag leer ist und keiner ungetrimmt. Ein Wert kann selbst keine
// Zeilenschaltung enthalten, weil die Möglichkeiten, aus denen er stammt,
// schon zeilenweise gelesen werden.
func TestValuesRundreise(t *testing.T) {
	for _, v := range [][]string{
		{"eiche"},
		{"eiche", "buche", "esche"},
		{"a", "a"},
		{"esche", "buche", "eiche"},
	} {
		zurück := SplitValues(JoinValues(v))
		if !reflect.DeepEqual(zurück, v) {
			t.Errorf("SplitValues(JoinValues(%#v)) = %#v", v, zurück)
		}
		// Zweimal speichern muss dieselbe Zeichenkette ergeben.
		einmal := JoinValues(v)
		if zweimal := JoinValues(SplitValues(einmal)); zweimal != einmal {
			t.Errorf("nicht idempotent: %q dann %q", einmal, zweimal)
		}
	}
}

// Die leeren Einträge fallen beim Verbinden weg, die doppelten nicht. Das ist
// die Kante, an der ein Häkchenfeld hängt: der versteckte Wächter schickt
// einen leeren Eintrag mit, und eine teilweise angekreuzte Gruppe darf davon
// nichts merken.
func TestJoinValuesWaechterUndDoppelte(t *testing.T) {
	if got := SplitValues(JoinValues([]string{"a", "", "a"})); !reflect.DeepEqual(got, []string{"a", "a"}) {
		t.Errorf("[a,\"\",a] kam als %#v zurück, wollte [a a]", got)
	}
}

// Die Mehrwertigkeit steht im Namen des Formularfeldes, und geprägt wird der
// Name an genau einer Stelle. Steht die Markierung irgendwo sonst noch einmal
// buchstabiert, kann sie auseinanderlaufen.
func TestFeldNameTraegtDieMarkierung(t *testing.T) {
	multi := Def{Kind: KindMulti, Key: "sorten"}
	if got, will := multi.FieldName(), "feld_sorten[]"; got != will {
		t.Errorf("FieldName() = %q, wollte %q", got, will)
	}
	if !multi.IsMultiValued() {
		t.Error("IsMultiValued() = false für eine Mehrfachauswahl")
	}
	if got, will := multi.NameSuffix(), "[]"; got != will {
		t.Errorf("NameSuffix() = %q, wollte %q", got, will)
	}

	// Und jede andere Art trägt sie nicht — sonst hiesse jedes bestehende
	// Feld ab heute anders und jeder gespeicherte Wert wäre still weg.
	for _, k := range Kinds {
		if k.Kind == KindMulti {
			continue
		}
		d := Def{Kind: k.Kind, Key: "sorten"}
		if got, will := d.FieldName(), "feld_sorten"; got != will {
			t.Errorf("FieldName() für %q = %q, wollte %q", k.Kind, got, will)
		}
		if d.IsMultiValued() {
			t.Errorf("IsMultiValued() = true für %q", k.Kind)
		}
	}
}

// Ein Wert, der nicht auf der Liste steht, wird gemeldet und nicht gespeichert.
// Die Möglichkeiten sind ein geschlossener Wortschatz; über eine Häkchenreihe
// darf keine beliebige Zeichenkette hereinkommen.
func TestMehrfachauswahlPruefung(t *testing.T) {
	d := Def{Label: "Sorten", Kind: KindMulti, Choices: []string{"Eiche", "Buche", "Esche"}}

	if reason := Check(d, JoinValues([]string{"Eiche", "Esche"})); reason != "" {
		t.Errorf("Check auf zwei gültige Werte = %q, erwartet in Ordnung", reason)
	}
	reason := Check(d, JoinValues([]string{"Eiche", "Ahorn"}))
	if reason == "" {
		t.Fatal("„Ahorn“ wurde durchgelassen")
	}
	if !strings.Contains(reason, "Ahorn") {
		t.Errorf("die Meldung nennt den fehlerhaften Wert nicht: %q", reason)
	}
	// Leer auf einem freiwilligen Feld ist in Ordnung, auf einem Pflichtfeld
	// nicht — das entscheidet die Wache oben in Check und muss so bleiben.
	if reason := Check(d, ""); reason != "" {
		t.Errorf("leer auf einem freiwilligen Feld = %q", reason)
	}
	pflicht := d
	pflicht.Required = true
	if reason := Check(pflicht, ""); reason == "" {
		t.Error("leer auf einem Pflichtfeld wurde durchgelassen")
	}
}

// Ein mehrwertiges Feld erreicht das Theme als Liste, nicht als Zeichenkette,
// und die Liste ist leer statt nil-verwirrt, wenn nichts gespeichert ist.
func TestMehrfachauswahlAufgeloest(t *testing.T) {
	defs := []Def{{Key: "sorten", Label: "Sorten", Kind: KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}}}

	got := Resolve(defs, Data{Values: Values{"sorten": "Eiche\nEsche"}}, Links{})
	werte, ok := got["sorten"].([]string)
	if !ok {
		t.Fatalf("sorten kam als %T, wollte []string", got["sorten"])
	}
	if !reflect.DeepEqual(werte, []string{"Eiche", "Esche"}) {
		t.Errorf("sorten = %#v", werte)
	}

	leer := Resolve(defs, Data{Values: Values{}}, Links{})
	if werte, ok := leer["sorten"].([]string); !ok || len(werte) != 0 {
		t.Errorf("leer aufgelöst = %#v (%T), wollte eine leere []string", leer["sorten"], leer["sorten"])
	}

	// List lässt das leere Feld weg und macht aus dem gefüllten einen lesbaren
	// Text — sonst druckt ein Theme, das .Text nimmt, einen Klumpen.
	entries := List(defs, Data{Values: Values{"sorten": "Eiche\nEsche"}}, Links{})
	if len(entries) != 1 {
		t.Fatalf("List = %+v, wollte einen Eintrag", entries)
	}
	if !reflect.DeepEqual(entries[0].Values, []string{"Eiche", "Esche"}) {
		t.Errorf("Entry.Values = %#v", entries[0].Values)
	}
	if got, will := entries[0].Text, "Eiche, Esche"; got != will {
		t.Errorf("Entry.Text = %q, wollte %q", got, will)
	}
	if leer := List(defs, Data{Values: Values{}}, Links{}); len(leer) != 0 {
		t.Errorf("das leere Feld steht in der Liste: %+v", leer)
	}

	// Und Filled muss die Liste kennen, sonst verschwindet die ganze
	// Feldtafel auf einer Seite, die nur mehrwertige Felder trägt.
	if !Filled(Resolve(defs, Data{Values: Values{"sorten": "Eiche"}}, Links{})) {
		t.Error("Filled = false, obwohl ein Wert da ist")
	}
	if Filled(Resolve(defs, Data{Values: Values{}}, Links{})) {
		t.Error("Filled = true auf einer leeren Seite")
	}
}
