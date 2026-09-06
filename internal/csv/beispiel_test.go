package csv

import (
	"bytes"
	"strings"
	"testing"
)

// Die andere Richtung: die Beispieldatei, die jemand herunterlädt, bevor er
// seine eigene schreibt.
//
// Zwei Dinge werden hier geprüft und nichts sonst. Erstens die BOM, ohne die
// Excel aus einem Ü ein Ãœ macht. Zweitens, dass keine Zelle als Formel
// gelesen wird, wenn die Datei auf einem fremden Rechner aufgeht — dieselbe
// Regel, die `plugins/kontaktformular/csv.go` schon einmal aufschreibt, hier in
// ihrer zweiten Ausfertigung, weil eine Datei in `package main` nicht
// importierbar ist.

// D-16: eine Zelle, die mit = + - @ Tabulator oder Wagenrücklauf beginnt, wird
// von Excel und LibreOffice als Formel ausgewertet, sobald die Datei geöffnet
// wird. Ein vorangestelltes Apostroph nimmt dem die Bedeutung.
func TestEntschaerfenNimmtFormelnDieBedeutung(t *testing.T) {
	faelle := []struct {
		name     string
		zelle    string
		erwartet string
	}{
		{"Gleichheitszeichen", "=1+1", "'=1+1"},
		{"Plus", "+41 79 000 00 00", "'+41 79 000 00 00"},
		{"Minus", "-5", "'-5"},
		{"Klammeraffe", "@SUM(A1)", "'@SUM(A1)"},
		{"Tabulator", "\tHallo", "'\tHallo"},
		{"Wagenrücklauf", "\rHallo", "'\rHallo"},
		{"leer", "", ""},
		{"gewöhnlich", "Apfel", "Apfel"},
		{"Gleichheitszeichen in der Mitte", "1=1", "1=1"},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			got := entschaerfen([]string{f.zelle})
			if len(got) != 1 || got[0] != f.erwartet {
				t.Errorf("entschaerfen(%q) = %q, erwartet %q", f.zelle, got, f.erwartet)
			}
		})
	}
}

// Ohne die Byte-Order-Mark öffnet Excel die Datei nicht als UTF-8.
func TestBeispielSchreibtDieBOM(t *testing.T) {
	out, err := Beispiel([]string{"Titel", "Grösse"}, nil)
	if err != nil {
		t.Fatalf("Beispiel: %v", err)
	}
	if !bytes.HasPrefix(out, bom) {
		t.Errorf("die Datei beginnt mit % x, erwartet % x", out[:3], bom)
	}
}

// Das Anführungszeichen und das Trennzeichen erledigt encoding/csv. Der
// Rückweg durch den Leser dieses Pakets ist der Beweis, dass hier nichts von
// Hand zusammengesetzt wird.
func TestBeispielZitiertSelbstNicht(t *testing.T) {
	kopf := []string{`Titel, lang`, `Mass "gross"`, "Text\nmit Umbruch"}
	zeilen := [][]string{{"a,b", `c"d`, "e"}}

	out, err := Beispiel(kopf, zeilen)
	if err != nil {
		t.Fatalf("Beispiel: %v", err)
	}

	l, err := Neu(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Neu: %v", err)
	}
	if got := l.Kopf(); !gleich(got, kopf) {
		t.Errorf("Kopf zurück = %q, erwartet %q", got, kopf)
	}
	z, ok := l.Naechste()
	if !ok {
		t.Fatal("keine Datenzeile zurückgelesen")
	}
	if !gleich(z.Zellen, zeilen[0]) {
		t.Errorf("Zeile zurück = %q, erwartet %q", z.Zellen, zeilen[0])
	}
}

// Eine Beispieldatei ohne Musterzeilen ist die BOM und die Kopfzeile, sonst
// nichts.
func TestBeispielOhneZeilen(t *testing.T) {
	out, err := Beispiel([]string{"Titel", "Adresse"}, nil)
	if err != nil {
		t.Fatalf("Beispiel: %v", err)
	}
	if got := string(bytes.TrimPrefix(out, bom)); got != "Titel,Adresse\n" {
		t.Errorf("Datei = %q, erwartet %q", got, "Titel,Adresse\n")
	}
	if strings.Count(string(out), "\n") != 1 {
		t.Errorf("Datei hat %d Zeilenumbrüche, erwartet 1", strings.Count(string(out), "\n"))
	}
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
