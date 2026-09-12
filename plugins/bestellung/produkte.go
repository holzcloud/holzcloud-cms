package main

import (
	"encoding/json"
	"strconv"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// What a product is.
//
// No content type of its own, no second table: a product is a published page
// that has filled in a price field. The page carries the image, the description
// and the address anyway — putting a product beside it would mean maintaining
// both and forgetting the second one.
//
// Which field is the price is what the operator says. Pre-filled with the names
// somebody would choose by themselves; whoever chose others enters them once.

// settings are the field names and the text around the form.
type settings struct {
	// PriceField decides what a product is: if it is filled in, the page stands
	// in the list.
	PriceField string `json:"preis_feld"`
	// UnitField and StatusField are optional.
	UnitField   string `json:"einheit_feld"`
	StatusField string `json:"zustand_feld"`
	// SoldOutValue is the value of the status field at which no more can be
	// ordered. Empty means: everything is orderable.
	SoldOutValue string `json:"ausverkauft_wert"`
	// Currency stands before the price.
	Currency string `json:"waehrung"`
	// Hint stands above the form — that is where how it is delivered and how it
	// is paid for belong, because both happen outside this program.
	Hint string `json:"hinweis"`
}

const schluesselEinstellungen = "einstellungen"

func standardEinstellungen() settings {
	return settings{
		PriceField:   "preis",
		UnitField:    "einheit",
		StatusField:  "verfuegbarkeit",
		SoldOutValue: "vergriffen",
		Currency:     "CHF",
		Hint: "We will get in touch after the order and arrange collection " +
			"or delivery. Payment is on handover or by invoice.",
	}
}

func einstellungenLaden() settings {
	e := standardEinstellungen()
	raw, da, err := plugin.Get(schluesselEinstellungen)
	if err != nil || !da || raw == "" {
		return e
	}
	var gespeichert settings
	if err := json.Unmarshal([]byte(raw), &gespeichert); err != nil {
		return e
	}
	// Field by field: an older version may not have written all of them yet,
	// and an empty price field would leave the list empty for good.
	if gespeichert.PriceField != "" {
		e.PriceField = gespeichert.PriceField
	}
	e.UnitField = gespeichert.UnitField
	e.StatusField = gespeichert.StatusField
	e.SoldOutValue = gespeichert.SoldOutValue
	if gespeichert.Currency != "" {
		e.Currency = gespeichert.Currency
	}
	e.Hint = gespeichert.Hint
	return e
}

func einstellungenSichern(e settings) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return plugin.Set(schluesselEinstellungen, string(raw))
}

// product is a page as the form needs it.
type product struct {
	Slug    string
	Titel   string
	Price   string
	Einheit string
	Status  string
	// Orderable is false when the status field carries the sold-out value.
	Orderable bool
}

// maxProducts bounds the list.
//
// The host hands out at most a hundred pages anyway; the limit stands here so
// that a form with a hundred rows does not come as a surprise.
const maxProducts = 100

// readProducts fetches the published pages with their own fields and keeps the
// ones that carry a price.
func readProducts(e settings) ([]product, error) {
	seiten, _, err := plugin.PagesWithFields(maxProducts, 0)
	if err != nil {
		return nil, err
	}

	out := make([]product, 0, len(seiten))
	for _, s := range seiten {
		price := strings.TrimSpace(s.Field(e.PriceField))
		if price == "" {
			continue
		}
		p := product{
			Slug: s.Slug, Titel: s.Title, Price: price,
			Einheit: strings.TrimSpace(s.Field(e.UnitField)),
			Status:  strings.TrimSpace(s.Field(e.StatusField)),
		}
		p.Orderable = e.SoldOutValue == "" ||
			!strings.EqualFold(p.Status, e.SoldOutValue)
		out = append(out, p)
	}
	return out, nil
}

// priceValue reads a typed price as a number.
//
// With a comma, because that is what somebody with a German keyboard types, and
// without thousands separators, because a farm shop has none. If it does not
// work out the total is unknown — then it does not stand in the confirmation and
// the operator works it out. Working it out wrongly would be worse.
func priceValue(roh string) (float64, bool) {
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

// amountText writes an amount with two decimal places and a comma.
func amountText(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}
