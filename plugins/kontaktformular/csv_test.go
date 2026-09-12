package main

import (
	"strings"
	"testing"
)

// A cell beginning with = + - @ or a control character is executed as a formula
// by Excel and LibreOffice as soon as somebody opens the file. The content comes
// from a visitor to the website — so this is the path by which a stranger runs
// something on the recipient's machine.
func TestCellsThatWouldBeReadAsAFormulaGetAnApostrophe(t *testing.T) {
	gefaehrlich := []string{
		`=1+1`,
		`+49 123`,
		`-5`,
		`@SUM(A1:A9)`,
		"\tTabulator",
		"\rWagenruecklauf",
	}
	for _, value := range gefaehrlich {
		got := entschaerfen([]string{value})[0]
		if !strings.HasPrefix(got, "'") {
			t.Errorf("%q stays undefused: %q", value, got)
		}
		if got != "'"+value {
			t.Errorf("%q was changed beyond the apostrophe: %q", value, got)
		}
	}
}

func TestHarmlessCellsStayUnchanged(t *testing.T) {
	for _, value := range []string{"", "Anna", "anna@example.ch", "1+1", "Preis: 5"} {
		if got := entschaerfen([]string{value})[0]; got != value {
			t.Errorf("%q wurde zu %q", value, got)
		}
	}
}

func TestTheTableCarriesTheFixedColumnsAndTheFormsOwn(t *testing.T) {
	liste := []message{
		{
			Key: "2026-08-30T10:00:00Z-a", Time: "2026-08-30T10:00:00Z",
			Name: "Anna", Email: "anna@example.ch", Subject: "Anfrage",
			Text: "Guten Tag", Page: "kontakt",
		},
		{
			Key: "2026-08-30T11:00:00Z-b", Time: "2026-08-30T11:00:00Z",
			Name: "Bruno", Email: "bruno@example.ch", FormName: "Anmeldung",
			Read: true,
			Fields: []answer{
				{Label: "Kurs", Value: "Drechseln"},
				{Label: "Personen", Value: "2"},
			},
		},
	}

	raw, err := asCSV(liste)
	if err != nil {
		t.Fatalf("alsCSV: %v", err)
	}
	out := string(raw)

	if !strings.HasPrefix(out, "\ufeff") {
		t.Error("without a byte-order mark Excel does not open the file as UTF-8")
	}
	for _, column := range []string{"Zeit", "Formular", "Name", "E-Mail", "Kurs", "Personen", "Nachricht"} {
		if !strings.Contains(out, column) {
			t.Errorf("Spalte %q fehlt", column)
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

// A message without a field another one has leaves the cell empty and does not
// shift the columns.
func TestMissingFieldsDoNotShiftTheColumns(t *testing.T) {
	liste := []message{
		{Key: "a", Fields: []answer{{Label: "Kurs", Value: "Drechseln"}}},
		{Key: "b", Fields: []answer{{Label: "Ort", Value: "Bern"}}},
	}
	raw, err := asCSV(liste)
	if err != nil {
		t.Fatalf("alsCSV: %v", err)
	}
	rows := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff")), "\n")
	if len(rows) != 3 {
		t.Fatalf("%d Zeilen, 3 erwartet", len(rows))
	}
	fields := strings.Count(rows[0], ",")
	for i, z := range rows[1:] {
		if strings.Count(z, ",") != fields {
			t.Errorf("Zeile %d hat %d Trennzeichen, die Kopfzeile %d", i+1, strings.Count(z, ","), fields)
		}
	}
}
