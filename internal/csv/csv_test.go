package csv

import (
	"errors"
	"strings"
	"testing"
)

// Die Wache über eine Datei, die von aussen kam.
//
// `encoding/csv` kennt keine Grenze: nicht für Zeilen, nicht für Spalten, nicht
// für Zellen, und ein Nullbyte ist ihm einerlei. Neun voneinander unabhängige
// Abwehren stehen deshalb in `csv.go`, und jede hat hier ihren eigenen Test,
// benannt nach der Abwehr und nicht nach der Eingabe — eine fehlgeschlagene
// Zeile soll sagen, welche Regel gebrochen ist, nicht welcher Text übrig war.
// Dazu kommt der Fehler um eins, der sonst jede Meldung dieser Phase falsch
// machen würde: die Kopfzeile ist Zeile 1, die erste Datenzeile ist Zeile 2.
//
// Kein Test hier öffnet eine Datenbank, baut eine Anfrage oder legt ein
// Verzeichnis an. Bräuchte einer davon eines, hätte das Paket eine
// Abhängigkeit bekommen, die es nicht haben darf.

// lesen liest eine ganze Datei ein und gibt den Leser dazu zurück, damit ein
// Test auch Kopf() und Abgeschnitten() prüfen kann.
func lesen(t *testing.T, quelle string) (*Leser, []Zeile) {
	t.Helper()
	l, err := Neu(strings.NewReader(quelle))
	if err != nil {
		t.Fatalf("Neu: %v", err)
	}
	var zeilen []Zeile
	for {
		z, ok := l.Naechste()
		if !ok {
			break
		}
		zeilen = append(zeilen, z)
	}
	return l, zeilen
}

// D-11: Die Byte-Order-Mark wird einmal gestreift, am Leser, vor der
// Kopfzeile. Das ist der Fehler Nummer eins beim CSV-Import: Excel schreibt
// sie, die erste Überschrift heisst dann "\ufeffTitel", passt auf keine
// Zuordnung, und die Titelspalte geht still verloren.
func TestBOMWirdEinmalGestreift(t *testing.T) {
	l, zeilen := lesen(t, "\ufeffTitel,Text\nA,B\n")
	if got := l.Kopf()[0]; got != "Titel" {
		t.Errorf("erste Überschrift = %q, erwartet %q", got, "Titel")
	}
	if len(zeilen) != 1 {
		t.Fatalf("%d Zeilen, erwartet 1", len(zeilen))
	}

	// Eine zweite BOM mitten in der Datei ist Inhalt und bleibt stehen.
	_, mitten := lesen(t, "\ufeffTitel,Text\n\ufeffA,B\n")
	if got := mitten[0].Zellen[0]; got != "\ufeffA" {
		t.Errorf("Zelle = %q, eine BOM im Inhalt darf nicht gestreift werden", got)
	}
}

// D-26, IMP-03 (boundary): Die Zeilennummer ist die der Tabelle. Die Kopfzeile
// ist Zeile 1, die erste Datenzeile Zeile 2 — nicht der Satzindex des Lesers
// und nicht die Zeilennummer der Datei.
func TestZeilennummerIstDieDerTabelle(t *testing.T) {
	if got := Zeilennummer(0); got != 2 {
		t.Errorf("Zeilennummer(0) = %d, erwartet 2", got)
	}
	if got := Zeilennummer(1); got != 3 {
		t.Errorf("Zeilennummer(1) = %d, erwartet 3", got)
	}

	_, zeilen := lesen(t, "Titel\nA\nB\n")
	if zeilen[0].Nummer != 2 || zeilen[1].Nummer != 3 {
		t.Errorf("Nummern = %d, %d, erwartet 2, 3", zeilen[0].Nummer, zeilen[1].Nummer)
	}
}

// D-09, IMP-09 (boundary): die Zeilengrenze, an der Grenze und einen Schritt
// zu beiden Seiten. Gemeldet, nie still angewandt.
func TestZeilengrenze(t *testing.T) {
	bauen := func(n int) string {
		var b strings.Builder
		b.WriteString("Titel\n")
		for i := 0; i < n; i++ {
			b.WriteString("A\n")
		}
		return b.String()
	}

	l, zeilen := lesen(t, bauen(MaxRows))
	if len(zeilen) != MaxRows {
		t.Errorf("%d Zeilen bei genau MaxRows, erwartet %d", len(zeilen), MaxRows)
	}
	if l.Abgeschnitten() {
		t.Error("genau MaxRows Zeilen gelten als abgeschnitten")
	}

	l, zeilen = lesen(t, bauen(MaxRows+1))
	if len(zeilen) != MaxRows {
		t.Errorf("%d Zeilen bei MaxRows+1, erwartet %d", len(zeilen), MaxRows)
	}
	if !l.Abgeschnitten() {
		t.Error("MaxRows+1 Zeilen werden still abgeschnitten statt gemeldet")
	}
}

// D-38: die Spaltengrenze, an der Grenze und einen Schritt zu beiden Seiten.
// Eine Datei mit 5000 Spalten ist erlaubt, besteht jede andere Grenze und
// erzeugt einen Zuordnungsbildschirm mit 5000 Zeilen.
func TestSpaltengrenze(t *testing.T) {
	kopf := func(n int) string {
		spalten := make([]string, n)
		for i := range spalten {
			spalten[i] = "s"
		}
		return strings.Join(spalten, ",") + "\n"
	}

	l, err := Neu(strings.NewReader(kopf(MaxSpalten)))
	if err != nil {
		t.Fatalf("genau MaxSpalten abgelehnt: %v", err)
	}
	if len(l.Kopf()) != MaxSpalten {
		t.Errorf("%d Spalten, erwartet %d", len(l.Kopf()), MaxSpalten)
	}

	if _, err := Neu(strings.NewReader(kopf(MaxSpalten + 1))); !errors.Is(err, ErrZuVieleSpalten) {
		t.Errorf("MaxSpalten+1 ergibt %v, erwartet ErrZuVieleSpalten", err)
	}
}

// D-10, IMP-09 (boundary + precision): die Zellengrenze zählt BYTES. Ein
// Emoji kostet vier. Eine Grenze in Runen wäre eine zweite Definition von
// Grösse neben der in Bytes.
func TestZellengrenzeInBytes(t *testing.T) {
	datei := func(zelle string) string {
		return "a,b\n1,2\n" + zelle + ",x\n3,4\n"
	}

	_, zeilen := lesen(t, datei(strings.Repeat("z", MaxCellBytes)))
	if zeilen[1].Fehler != "" {
		t.Errorf("genau MaxCellBytes abgelehnt: %s", zeilen[1].Fehler)
	}

	_, zeilen = lesen(t, datei(strings.Repeat("z", MaxCellBytes+1)))
	if zeilen[1].Fehler == "" {
		t.Error("MaxCellBytes+1 Bytes durchgelassen")
	}
	if zeilen[1].Nummer != 3 {
		t.Errorf("die zu grosse Zelle meldet Zeile %d, erwartet 3", zeilen[1].Nummer)
	}
	// Eine Zeile kostet eine Zeile, nicht die Datei.
	if len(zeilen) != 3 || zeilen[2].Fehler != "" || zeilen[2].Nummer != 4 {
		t.Errorf("die Zeile nach der zu grossen wurde nicht sauber gelesen: %+v", zeilen)
	}

	// MaxCellBytes Bytes aus Vier-Byte-Runen: erlaubt.
	vier := strings.Repeat("😀", MaxCellBytes/4)
	if len(vier) != MaxCellBytes {
		t.Fatalf("Testaufbau: %d Bytes, erwartet %d", len(vier), MaxCellBytes)
	}
	_, zeilen = lesen(t, datei(vier))
	if zeilen[1].Fehler != "" {
		t.Errorf("MaxCellBytes Bytes aus Vier-Byte-Runen abgelehnt: %s", zeilen[1].Fehler)
	}

	// MaxCellBytes RUNEN aus Vier-Byte-Runen: abgelehnt, denn das sind
	// viermal so viele Bytes.
	_, zeilen = lesen(t, datei(strings.Repeat("😀", MaxCellBytes)))
	if zeilen[1].Fehler == "" {
		t.Error("MaxCellBytes Runen à 4 Bytes durchgelassen — die Grenze zählt Runen statt Bytes")
	}
}

// D-13, IMP-09 (ordering): eine kurze Zeile verschiebt nicht. Die fehlenden
// Zellen sind AN IHRER STELLE leer, Zelle 5 einer Zeile mit drei Zellen ist
// leer und nicht der Wert von Zelle 3.
func TestKurzeZeileVerschiebtNicht(t *testing.T) {
	_, zeilen := lesen(t, "a,b,c,d,e\n1,2,3\n")
	z := zeilen[0]
	if len(z.Zellen) != 5 {
		t.Fatalf("%d Zellen, erwartet 5: %q", len(z.Zellen), z.Zellen)
	}
	if z.Zellen[2] != "3" {
		t.Errorf("Zelle 3 = %q, erwartet %q", z.Zellen[2], "3")
	}
	if z.Zellen[3] != "" || z.Zellen[4] != "" {
		t.Errorf("Zellen 4 und 5 = %q, %q, erwartet leer", z.Zellen[3], z.Zellen[4])
	}
}

// D-12: ein verirrtes Anführungszeichen frisst den Rest der Datei nicht.
// LazyQuotes = true macht daraus eine schlechte Zeile statt eines Fehlers für
// die ganze Datei.
func TestStrayQuoteFrisstDenRestNicht(t *testing.T) {
	_, zeilen := lesen(t, "a,b,c\n1,zwei\"komisch,3\n4,5,6\n")
	if len(zeilen) != 2 {
		t.Fatalf("%d Zeilen, erwartet 2: %+v", len(zeilen), zeilen)
	}
	if zeilen[1].Nummer != 3 || zeilen[1].Zellen[0] != "4" {
		t.Errorf("die Zeile nach der schlechten = %+v, erwartet Zeile 3 mit 4,5,6", zeilen[1])
	}
}

// D-14, IMP-09 (adjacency): ein Zitat mit Trennzeichen bleibt eine Zelle, und
// ein Zitat über zwei Zeilen erhöht die Zeilennummer nicht. Darum wird in
// unserer eigenen Schleife gezählt und nie an der Dateizeile.
func TestZitatMitTrennzeichenUndZeilenumbruch(t *testing.T) {
	_, zeilen := lesen(t, "a,b,c\n1,\"zwei,drei\",4\n")
	if len(zeilen[0].Zellen) != 3 {
		t.Fatalf("%d Zellen, erwartet 3: %q", len(zeilen[0].Zellen), zeilen[0].Zellen)
	}
	if zeilen[0].Zellen[1] != "zwei,drei" {
		t.Errorf("mittlere Zelle = %q, erwartet %q", zeilen[0].Zellen[1], "zwei,drei")
	}

	_, zeilen = lesen(t, "a,b\n\"x\ny\",2\n3,4\n")
	if zeilen[0].Zellen[0] != "x\ny" {
		t.Errorf("Zelle über zwei Zeilen = %q", zeilen[0].Zellen[0])
	}
	// Der Punkt: die folgende Zeile steht auf Dateizeile 4 und ist Zeile 3.
	if zeilen[1].Nummer != 3 {
		t.Errorf("Zeile nach dem mehrzeiligen Zitat = Nummer %d, erwartet 3", zeilen[1].Nummer)
	}
}

// D-14: eine Leerzeile wird übersprungen und verbraucht keine Zeilennummer.
func TestLeerzeileZaehltNicht(t *testing.T) {
	_, zeilen := lesen(t, "a,b\n1,2\n\n3,4\n")
	if len(zeilen) != 2 {
		t.Fatalf("%d Zeilen, erwartet 2", len(zeilen))
	}
	if zeilen[1].Nummer != 3 {
		t.Errorf("Zeile nach der Leerzeile = Nummer %d, erwartet 3", zeilen[1].Nummer)
	}
}

// D-15, IMP-09 (empty): drei Ablehnungen vor dem Parsen, jede mit eigenem
// Grund, damit der Bildschirm sagen kann, welcher zutrifft.
func TestPruefeBytesLehntAb(t *testing.T) {
	faelle := []struct {
		name     string
		bytes    []byte
		erwartet error
	}{
		{"leer", []byte{}, ErrLeer},
		{"nichts", nil, ErrLeer},
		{"nur BOM", []byte("\ufeff"), ErrNurBOM},
		{"Nullbyte", []byte("a,b\n1,\x002\n"), ErrNullbyte},
		{"Nullbyte hinter der BOM", []byte("\ufeffa\n\x00"), ErrNullbyte},
		{"in Ordnung", []byte("a,b\n1,2\n"), nil},
		{"BOM und Inhalt", []byte("\ufeffa,b\n"), nil},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			err := PruefeBytes(f.bytes)
			if !errors.Is(err, f.erwartet) {
				t.Errorf("PruefeBytes = %v, erwartet %v", err, f.erwartet)
			}
		})
	}
}

// IMP-01 (empty): eine Datei mit Kopfzeile und ohne Datenzeilen ist eine
// gültige Datei. Der Satz dazu gehört auf den Bildschirm, nicht hierher.
func TestNurKopfzeileIstKeinFehler(t *testing.T) {
	l, zeilen := lesen(t, "Titel,Text\n")
	if len(zeilen) != 0 {
		t.Errorf("%d Zeilen, erwartet 0", len(zeilen))
	}
	if len(l.Kopf()) != 2 {
		t.Errorf("Kopf = %q, erwartet zwei Spalten", l.Kopf())
	}
	if l.Abgeschnitten() {
		t.Error("eine leere Datei gilt als abgeschnitten")
	}
}

// Eine Datei ganz ohne Sätze hat keine Kopfzeile, und das ist ein Fehler, den
// der Bildschirm benennen kann.
func TestOhneKopfzeileIstFehler(t *testing.T) {
	if _, err := Neu(strings.NewReader("")); !errors.Is(err, ErrKeineKopfzeile) {
		t.Errorf("Neu über eine leere Quelle = %v, erwartet ErrKeineKopfzeile", err)
	}
}
