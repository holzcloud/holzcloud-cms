package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"fmt"
)

// Beispiel writes the example file an operator downloads before they write
// their own: the right column headings, already in the right order, plus
// whatever sample rows the caller wants to show.
//
// The signature takes plain strings, and that is deliberate. Deriving the
// columns from a website's field definitions is knowledge of internal/field,
// and letting that in here would give this package a dependency on one that
// carries a store — which is exactly the purity that makes the checklist
// against a hostile file provable without a database. The derivation lives with
// the field definitions.
func Beispiel(kopf []string, zeilen [][]string) ([]byte, error) {
	var buf bytes.Buffer
	// Die Byte-Order-Mark, damit Excel die Datei als UTF-8 öffnet. Ohne sie
	// wird aus einem Ü ein Ãœ, und dann tippt doch wieder jemand ab.
	buf.Write(bom)

	w := stdcsv.NewWriter(&buf)
	if err := w.Write(entschaerfen(kopf)); err != nil {
		return nil, fmt.Errorf("csv: write header row: %w", err)
	}
	for _, zeile := range zeilen {
		if err := w.Write(entschaerfen(zeile)); err != nil {
			return nil, fmt.Errorf("csv: write sample row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv: finish table: %w", err)
	}
	return buf.Bytes(), nil
}

// entschaerfen macht aus jeder Zelle eine, die kein Tabellenprogramm als
// Formel liest.
//
// Der Anlass: eine Zelle, die mit = + - @ oder einem Tabulator beginnt, wird
// von Excel und LibreOffice als Formel ausgewertet, sobald die Datei geöffnet
// wird. Der Inhalt kommt hier aus den Feldbezeichnungen einer Website und
// nicht aus einem Formular, das ein Besucher ausgefüllt hat — das ist eine
// kleinere Angriffsfläche und kein Grund, die Entschärfung wegzulassen, denn
// geöffnet wird die Datei so oder so auf dem Rechner einer Person. Ein
// vorangestelltes Apostroph nimmt dem die Bedeutung; es ist die Schreibweise,
// die beide Programme als "das ist Text" verstehen.
//
// Das Anführungszeichen und das Trennzeichen erledigt encoding/csv. Von Hand
// zusammengesetzt wird hier nichts.
//
// Dies ist die ZWEITE Ausfertigung dieser Regel in diesem Verzeichnisbaum. Die
// erste steht in plugins/kontaktformular/csv.go, und sie ist nicht
// importierbar: diese Datei ist `package main` in einem wazero-Plugin, also
// eine eigene Übersetzungseinheit ohne Bibliothekscharakter. Darum steht die
// Regel zweimal statt einmal, und darum benennt jede Ausfertigung die andere:
// eine Änderung an einer von beiden gehört in beide. Eine unkommentierte
// Verdopplung ist der Weg, auf dem zwei Kopien auseinanderlaufen.
func entschaerfen(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		if cell == "" {
			out[i] = cell
			continue
		}
		switch cell[0] {
		case '=', '+', '-', '@', 0x09, 0x0d:
			out[i] = "'" + cell
		default:
			out[i] = cell
		}
	}
	return out
}
