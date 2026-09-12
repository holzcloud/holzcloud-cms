package main

import (
	"strings"
	"testing"
)

// Eine Zelle, die mit = + - @ oder einem Steuerzeichen beginnt, führt Excel und
// LibreOffice als Formel aus, sobald jemand die Datei öffnet. Der Inhalt kommt
// von einem Besucher der Website — das ist also der Weg, auf dem ein Fremder
// etwas auf dem Rechner der Empfängerin ausführt.
func TestZellenDieAlsFormelGelesenWuerdenBekommenEinApostroph(t *testing.T) {
	gefaehrlich := []string{
		`=1+1`,
		`+49 123`,
		`-5`,
		`@SUM(A1:A9)`,
		"\tTabulator",
		"\rWagenruecklauf",
	}
	for _, wert := range gefaehrlich {
		got := entschaerfen([]string{wert})[0]
		if !strings.HasPrefix(got, "'") {
			t.Errorf("%q stays undefused: %q", wert, got)
		}
		if got != "'"+wert {
			t.Errorf("%q was changed beyond the apostrophe: %q", wert, got)
		}
	}
}

func TestHarmloseZellenBleibenUnveraendert(t *testing.T) {
	for _, wert := range []string{"", "Anna", "anna@example.ch", "1+1", "Preis: 5"} {
		if got := entschaerfen([]string{wert})[0]; got != wert {
			t.Errorf("%q wurde zu %q", wert, got)
		}
	}
}

func TestTabelleTraegtDieFestenSpaltenUndDieDerFormulare(t *testing.T) {
	liste := []nachricht{
		{
			Key: "2026-08-30T10:00:00Z-a", Zeit: "2026-08-30T10:00:00Z",
			Name: "Anna", Email: "anna@example.ch", Betreff: "Anfrage",
			Text: "Guten Tag", Page: "kontakt",
		},
		{
			Key: "2026-08-30T11:00:00Z-b", Zeit: "2026-08-30T11:00:00Z",
			Name: "Bruno", Email: "bruno@example.ch", FormularName: "Anmeldung",
			Gelesen: true,
			Fields: []antwort{
				{Label: "Kurs", Value: "Drechseln"},
				{Label: "Personen", Value: "2"},
			},
		},
	}

	raw, err := alsCSV(liste)
	if err != nil {
		t.Fatalf("alsCSV: %v", err)
	}
	out := string(raw)

	if !strings.HasPrefix(out, "\ufeff") {
		t.Error("without a byte-order mark Excel does not open the file as UTF-8")
	}
	for _, spalte := range []string{"Zeit", "Formular", "Name", "E-Mail", "Kurs", "Personen", "Nachricht"} {
		if !strings.Contains(out, spalte) {
			t.Errorf("Spalte %q fehlt", spalte)
		}
	}
	if !strings.Contains(out, "Drechseln") || !strings.Contains(out, "Guten Tag") {
		t.Error("the answers are not in the table")
	}
	// Beide Zeilen und die Kopfzeile.
	if n := strings.Count(strings.TrimSpace(out), "\n"); n != 2 {
		t.Errorf("%d line breaks, expected 2 — header row plus two messages", n)
	}
}

// Eine Nachricht ohne ein Feld, das eine andere hat, lässt die Zelle leer und
// verschiebt die Spalten nicht.
func TestFehlendeFelderVerschiebenDieSpaltenNicht(t *testing.T) {
	liste := []nachricht{
		{Key: "a", Fields: []antwort{{Label: "Kurs", Value: "Drechseln"}}},
		{Key: "b", Fields: []antwort{{Label: "Ort", Value: "Bern"}}},
	}
	raw, err := alsCSV(liste)
	if err != nil {
		t.Fatalf("alsCSV: %v", err)
	}
	zeilen := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff")), "\n")
	if len(zeilen) != 3 {
		t.Fatalf("%d Zeilen, 3 erwartet", len(zeilen))
	}
	felder := strings.Count(zeilen[0], ",")
	for i, z := range zeilen[1:] {
		if strings.Count(z, ",") != felder {
			t.Errorf("Zeile %d hat %d Trennzeichen, die Kopfzeile %d", i+1, strings.Count(z, ","), felder)
		}
	}
}
