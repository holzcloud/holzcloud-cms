package main

import (
	"encoding/json"
	"strconv"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Was ein Produkt ist.
//
// Kein eigener Inhaltstyp, keine zweite Tabelle: ein Produkt ist eine
// veröffentlichte Seite, die ein Preisfeld ausgefüllt hat. Die Seite trägt Bild,
// Beschreibung und Adresse ohnehin schon — ein Produkt daneben zu stellen hiesse,
// beides doppelt zu pflegen und beim zweiten Mal zu vergessen.
//
// Welches Feld der Preis ist, sagt der Betreiber. Vorbelegt mit den Namen, die
// jemand von selbst vergibt; wer andere gewählt hat, trägt sie einmal ein.

// einstellungen sind die Feldnamen und der Text um das Formular herum.
type einstellungen struct {
	// PreisFeld entscheidet, was ein Produkt ist: ist es ausgefüllt, steht die
	// Seite in der Liste.
	PriceField string `json:"preis_feld"`
	// EinheitFeld und ZustandFeld sind freiwillig.
	UnitField   string `json:"einheit_feld"`
	StatusField string `json:"zustand_feld"`
	// AusverkauftWert ist der Wert des Zustandsfeldes, bei dem nicht mehr
	// bestellt werden kann. Leer heisst: alles ist bestellbar.
	SoldOutValue string `json:"ausverkauft_wert"`
	// Waehrung steht vor dem Preis.
	Waehrung string `json:"waehrung"`
	// Hinweis steht über dem Formular — dort gehört hin, wie geliefert und wie
	// bezahlt wird, denn beides passiert ausserhalb dieses Programms.
	Hint string `json:"hinweis"`
}

const schluesselEinstellungen = "einstellungen"

func standardEinstellungen() einstellungen {
	return einstellungen{
		PriceField:   "preis",
		UnitField:    "einheit",
		StatusField:  "verfuegbarkeit",
		SoldOutValue: "vergriffen",
		Waehrung:     "CHF",
		Hint: "Wir melden uns nach der Bestellung bei dir und vereinbaren Abholung " +
			"oder Lieferung. Bezahlt wird bei der Übergabe oder per Rechnung.",
	}
}

func einstellungenLaden() einstellungen {
	e := standardEinstellungen()
	raw, da, err := plugin.Get(schluesselEinstellungen)
	if err != nil || !da || raw == "" {
		return e
	}
	var gespeichert einstellungen
	if err := json.Unmarshal([]byte(raw), &gespeichert); err != nil {
		return e
	}
	// Feld für Feld: eine ältere Fassung hat womöglich noch nicht alle
	// geschrieben, und ein leeres Preisfeld liesse die Liste für immer leer.
	if gespeichert.PriceField != "" {
		e.PriceField = gespeichert.PriceField
	}
	e.UnitField = gespeichert.UnitField
	e.StatusField = gespeichert.StatusField
	e.SoldOutValue = gespeichert.SoldOutValue
	if gespeichert.Waehrung != "" {
		e.Waehrung = gespeichert.Waehrung
	}
	e.Hint = gespeichert.Hint
	return e
}

func einstellungenSichern(e einstellungen) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return plugin.Set(schluesselEinstellungen, string(raw))
}

// produkt ist eine Seite, wie das Formular sie braucht.
type produkt struct {
	Slug    string
	Titel   string
	Price   string
	Einheit string
	Status  string
	// Bestellbar ist falsch, wenn das Zustandsfeld den Ausverkauft-Wert trägt.
	Orderable bool
}

// maxProdukte bounds the list.
//
// Der Host gibt ohnehin höchstens hundert Seiten heraus; die Grenze steht hier,
// damit ein Formular mit hundert Zeilen nicht als Überraschung kommt.
const maxProdukte = 100

// produkteLesen holt die veröffentlichten Seiten mit ihren eigenen Feldern und
// behält die, die einen Preis tragen.
func produkteLesen(e einstellungen) ([]produkt, error) {
	seiten, _, err := plugin.PagesWithFields(maxProdukte, 0)
	if err != nil {
		return nil, err
	}

	out := make([]produkt, 0, len(seiten))
	for _, s := range seiten {
		preis := strings.TrimSpace(s.Field(e.PriceField))
		if preis == "" {
			continue
		}
		p := produkt{
			Slug: s.Slug, Titel: s.Title, Price: preis,
			Einheit: strings.TrimSpace(s.Field(e.UnitField)),
			Status:  strings.TrimSpace(s.Field(e.StatusField)),
		}
		p.Orderable = e.SoldOutValue == "" ||
			!strings.EqualFold(p.Status, e.SoldOutValue)
		out = append(out, p)
	}
	return out, nil
}

// preisWert liest einen getippten Preis als Zahl.
//
// Mit Komma, weil das ist, was jemand mit einer deutschen Tastatur tippt, und
// ohne Tausendertrennung, weil ein Hofladen keine hat. Geht es nicht auf, ist
// die Summe unbekannt — dann steht sie nicht in der Bestätigung, und der
// Betreiber rechnet nach. Falsch zu rechnen wäre schlimmer.
func preisWert(roh string) (float64, bool) {
	roh = strings.TrimSpace(strings.ReplaceAll(roh, ",", "."))
	if roh == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(roh, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// betragTexten schreibt einen Betrag mit zwei Nachkommastellen und Komma.
func betragTexten(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}
