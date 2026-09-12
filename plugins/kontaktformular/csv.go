package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// asCSV turns the messages into a table to download.
//
// Why that is worth a view: an enquiry comes in and then has to be processed
// somewhere else — into an address list, into a list of sign-ups, into the
// bookkeeping. Without an export it gets typed out again, and typed out again
// it gets typed wrong.
//
// The columns are fixed: time, form, name, e-mail, subject, page, read — and
// after that one column for every field that occurred in any message. A message
// that does not have a field leaves the cell empty rather than shifting the
// columns.
func asCSV(liste []message) ([]byte, error) {
	columns := fieldColumns(liste)

	kopf := append([]string{
		"Zeit", "Formular", "Name", "E-Mail", "Betreff", "Seite", "Gelesen",
	}, columns...)

	var buf bytes.Buffer
	// The byte-order mark, so that Excel opens the file as UTF-8. Without it a
	// Ü becomes a Ãœ, and then somebody types it out again after all. //nolint:german — names the two spellings it is about
	buf.WriteString("\ufeff")

	w := csv.NewWriter(&buf)
	if err := w.Write(entschaerfen(kopf)); err != nil {
		return nil, fmt.Errorf("write header row: %w", err)
	}

	for _, n := range liste {
		read := "nein"
		if n.Read {
			read = "ja"
		}
		formular := n.FormName
		if formular == "" {
			formular = "Kontaktformular"
		}
		row := []string{n.Time, formular, n.Name, n.Email, n.Subject, n.Page, read}

		values := map[string]string{}
		for _, a := range n.Fields {
			values[a.Label] = a.Value
		}
		// The free text stands in the "Nachricht" column, so that the built-in
		// form and an assembled one can stand side by side in the same file.
		if n.Text != "" {
			values["Nachricht"] = n.Text
		}
		for _, s := range columns {
			row = append(row, values[s])
		}

		if err := w.Write(entschaerfen(row)); err != nil {
			return nil, fmt.Errorf("write row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("tabelle abschliessen: %w", err)
	}
	return buf.Bytes(), nil
}

// fieldColumns are all the field labels that occur, in a stable order: first in
// the order in which they first turn up, so that a form's columns stand as they
// do on the screen.
func fieldColumns(liste []message) []string {
	gesehen := map[string]bool{}
	var out []string
	var hatText bool
	for _, n := range liste {
		for _, a := range n.Fields {
			if !gesehen[a.Label] {
				gesehen[a.Label] = true
				out = append(out, a.Label)
			}
		}
		if n.Text != "" {
			hatText = true
		}
	}
	if hatText && !gesehen["Nachricht"] {
		out = append(out, "Nachricht")
	}
	return out
}

// defuse turns every cell into one no spreadsheet program reads as a formula.
//
// The occasion: a cell beginning with = + - @ or a tab is evaluated as a formula
// by Excel and LibreOffice as soon as the file is opened. The content here comes
// from a visitor to the website — so somebody can write into a form field what
// then runs on the recipient's machine. A leading apostrophe takes that meaning
// away; it is the spelling both programs understand as "this is text".
//
// The quotation mark and the separator are handled by encoding/csv. Nothing is
// assembled by hand here.
//
// This rule stands twice in this directory tree. The second copy is
// internal/csv/example.go, for the CSV import's example file. This one here
// cannot be imported: it is `package main` in a wazero plugin. A change to
// either one belongs in both.
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

// csvFilename names the file after the day, so that two exports in the same
// folder are not the same file.
func csvFilename(liste []message) string {
	tag := "export"
	if len(liste) > 0 {
		// The key starts with the timestamp; the first ten characters are the
		// date.
		if k := newestKey(liste); len(k) >= 10 {
			tag = k[:10]
		}
	}
	return "nachrichten-" + tag + ".csv"
}

func newestKey(liste []message) string {
	keys := make([]string, 0, len(liste))
	for _, n := range liste {
		keys = append(keys, n.Key)
	}
	sort.Strings(keys)
	return keys[len(keys)-1]
}

// csvOutput is the answer the host passes on as a file.
func csvAusgabe(liste []message) (plugin.AdminOut, error) {
	body, err := asCSV(liste)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	return plugin.AdminOut{
		Download: &plugin.Download{
			Filename:    csvFilename(liste),
			ContentType: "text/csv",
			Body:        string(body),
		},
	}, nil
}
