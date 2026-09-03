package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// alsCSV macht aus den Nachrichten eine Tabelle zum Herunterladen.
//
// Warum das eine Ansicht wert ist: eine Anfrage kommt herein und muss danach
// irgendwo weiterverarbeitet werden — in eine Adressliste, in eine
// Anmeldeübersicht, in die Buchhaltung. Ohne Ausgabe wird das abgetippt, und
// abgetippt wird es falsch.
//
// Die Spalten stehen fest: Zeit, Formular, Name, E-Mail, Betreff, Seite,
// gelesen — und danach je eine Spalte für jedes Feld, das in irgendeiner
// Nachricht vorkam. Eine Nachricht, die ein Feld nicht hat, lässt die Zelle
// leer, statt die Spalten zu verschieben.
func alsCSV(liste []nachricht) ([]byte, error) {
	spalten := feldSpalten(liste)

	kopf := append([]string{
		"Zeit", "Formular", "Name", "E-Mail", "Betreff", "Seite", "Gelesen",
	}, spalten...)

	var buf bytes.Buffer
	// Die Byte-Order-Mark, damit Excel die Datei als UTF-8 öffnet. Ohne sie
	// wird aus einem Ü ein Ãœ, und dann tippt doch wieder jemand ab.
	buf.WriteString("\ufeff")

	w := csv.NewWriter(&buf)
	if err := w.Write(entschaerfen(kopf)); err != nil {
		return nil, fmt.Errorf("kopfzeile schreiben: %w", err)
	}

	for _, n := range liste {
		gelesen := "nein"
		if n.Gelesen {
			gelesen = "ja"
		}
		formular := n.FormularName
		if formular == "" {
			formular = "Kontaktformular"
		}
		zeile := []string{n.Zeit, formular, n.Name, n.Email, n.Betreff, n.Seite, gelesen}

		werte := map[string]string{}
		for _, a := range n.Felder {
			werte[a.Beschriftung] = a.Wert
		}
		// Der freie Text steht in der Spalte "Nachricht", damit das eingebaute
		// Formular und ein zusammengestelltes in derselben Datei nebeneinander
		// stehen können.
		if n.Text != "" {
			werte["Nachricht"] = n.Text
		}
		for _, s := range spalten {
			zeile = append(zeile, werte[s])
		}

		if err := w.Write(entschaerfen(zeile)); err != nil {
			return nil, fmt.Errorf("zeile schreiben: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("tabelle abschliessen: %w", err)
	}
	return buf.Bytes(), nil
}

// feldSpalten sind alle Feldbeschriftungen, die vorkommen, in stabiler
// Reihenfolge: zuerst in der Reihenfolge, in der sie das erste Mal auftauchen,
// damit die Spalten eines Formulars so stehen wie auf dem Bildschirm.
func feldSpalten(liste []nachricht) []string {
	gesehen := map[string]bool{}
	var out []string
	var hatText bool
	for _, n := range liste {
		for _, a := range n.Felder {
			if !gesehen[a.Beschriftung] {
				gesehen[a.Beschriftung] = true
				out = append(out, a.Beschriftung)
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

// entschaerfen macht aus jeder Zelle eine, die kein Tabellenprogramm als
// Formel liest.
//
// Der Anlass: eine Zelle, die mit = + - @ oder einem Tabulator beginnt, wird
// von Excel und LibreOffice als Formel ausgewertet, sobald die Datei geöffnet
// wird. Der Inhalt kommt hier von einem Besucher der Website — jemand kann
// also in ein Formularfeld schreiben, was auf dem Rechner der Empfängerin
// ausgeführt wird. Ein vorangestelltes Apostroph nimmt dem die Bedeutung; es
// ist die Schreibweise, die beide Programme als "das ist Text" verstehen.
//
// Das Anführungszeichen und das Trennzeichen erledigt encoding/csv. Von Hand
// zusammengesetzt wird hier nichts.
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

// csvDateiname benennt die Datei nach dem Tag, damit zwei Ausgaben im selben
// Ordner nicht dieselbe Datei sind.
func csvDateiname(liste []nachricht) string {
	tag := "export"
	if len(liste) > 0 {
		// Der Schlüssel beginnt mit dem Zeitstempel; die ersten zehn Zeichen
		// sind das Datum.
		if k := neuesteKennung(liste); len(k) >= 10 {
			tag = k[:10]
		}
	}
	return "nachrichten-" + tag + ".csv"
}

func neuesteKennung(liste []nachricht) string {
	keys := make([]string, 0, len(liste))
	for _, n := range liste {
		keys = append(keys, n.Kennung)
	}
	sort.Strings(keys)
	return keys[len(keys)-1]
}

// csvAusgabe ist die Antwort, die der Host als Datei weiterreicht.
func csvAusgabe(liste []nachricht) (plugin.AdminOut, error) {
	body, err := alsCSV(liste)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	return plugin.AdminOut{
		Download: &plugin.Download{
			Filename:    csvDateiname(liste),
			ContentType: "text/csv",
			Body:        string(body),
		},
	}, nil
}
