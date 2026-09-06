package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"fmt"
)

// Example writes the example file an operator downloads before they write
// their own: the right column headings, already in the right order, plus
// whatever sample rows the caller wants to show.
//
// The signature takes plain strings, and that is deliberate. Deriving the
// columns from a website's field definitions is knowledge of internal/field,
// and letting that in here would give this package a dependency on one that
// carries a store — which is exactly the purity that makes the checklist
// against a hostile file provable without a database. The derivation lives with
// the field definitions.
func Example(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	// The byte-order mark, so Excel opens the file as UTF-8. Without it an
	// accented character arrives as mojibake, and then somebody retypes the
	// whole thing after all.
	buf.Write(bom)

	w := stdcsv.NewWriter(&buf)
	if err := w.Write(defuse(header)); err != nil {
		return nil, fmt.Errorf("csv: write header row: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(defuse(row)); err != nil {
			return nil, fmt.Errorf("csv: write sample row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv: finish table: %w", err)
	}
	return buf.Bytes(), nil
}

// defuse turns every cell into one that no spreadsheet program reads as a
// formula.
//
// The occasion: a cell that begins with = + - @ or a tab is evaluated as a
// formula by Excel and LibreOffice as soon as the file is opened. The content
// here comes from a website's field labels and not from a form some visitor
// filled in — that is a smaller attack surface, and no reason to leave the
// defusing out, because the file is opened on a person's machine either way. A
// leading apostrophe takes the meaning out of it; it is the spelling both
// programs understand as "this is text".
//
// The quoting and the separator are encoding/csv's business. Nothing here is
// assembled by hand.
//
// This is the SECOND copy of this rule in this directory tree. The first one is
// in plugins/kontaktformular/csv.go, and it cannot be imported: that file is
// `package main` in a wazero plugin, so it is a translation unit of its own
// with no library character. That is why the rule stands twice instead of once,
// and why each copy names the other: a change to either one belongs in both. An
// uncommented duplication is the road along which two copies drift apart.
func defuse(row []string) []string {
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
