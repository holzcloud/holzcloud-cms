package main

import (
	"html"
	"net/url"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// The screen in the admin.
//
// Three views in one: the list of orders, a single order, and the settings.
// Which is meant is said by the query string — a plugin gets an address and can
// point at itself.

func verwaltung(in plugin.AdminIn) (plugin.AdminOut, error) {
	q, _ := url.ParseQuery(in.Query)

	if in.Method == "POST" {
		return action(in, q)
	}

	switch q.Get("ansicht") {
	case "einstellungen":
		return einstellungsbildschirm(in)
	case "bestellung":
		return single(in, q.Get("id"))
	default:
		return liste(in)
	}
}

// action carries out what a button triggered.
func action(in plugin.AdminIn, q url.Values) (plugin.AdminOut, error) {
	form := url.Values{}
	for k, v := range in.Form {
		form[k] = v
	}

	switch form.Get("aktion") {
	case "einstellungen":
		e := settings{
			PriceField:   strings.TrimSpace(form.Get("preis_feld")),
			UnitField:    strings.TrimSpace(form.Get("einheit_feld")),
			StatusField:  strings.TrimSpace(form.Get("zustand_feld")),
			SoldOutValue: strings.TrimSpace(form.Get("ausverkauft_wert")),
			Currency:     strings.TrimSpace(form.Get("waehrung")),
			Hint:         strings.TrimSpace(form.Get("hinweis")),
		}
		if e.PriceField == "" {
			return plugin.AdminOut{Redirect: "?ansicht=einstellungen",
				Flash: "Without a price field the farm shop does not know what a product is.", FlashError: true}, nil
		}
		if err := einstellungenSichern(e); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=einstellungen", Flash: "Gespeichert."}, nil

	case "erledigt", "offen":
		b, da := loadOrder(form.Get("id"))
		if !da {
			return plugin.AdminOut{Redirect: "?", Flash: "This order no longer exists.", FlashError: true}, nil
		}
		b.Done = form.Get("aktion") == "erledigt"
		if err := bestellungSichern(b); err != nil {
			return plugin.AdminOut{}, err
		}
		wort := "als offen markiert"
		if b.Done {
			wort = "abgehakt"
		}
		return plugin.AdminOut{Redirect: "?", Flash: "Bestellung " + wort + "."}, nil

	case "loeschen":
		if err := plugin.Delete(prefixOrder + form.Get("id")); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?", Flash: "Order deleted."}, nil
	}
	return plugin.AdminOut{Redirect: "?"}, nil
}

// liste zeigt alle Bestellungen.
func liste(in plugin.AdminIn) (plugin.AdminOut, error) {
	orders, err := allOrders()
	if err != nil {
		return plugin.AdminOut{}, err
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?ansicht=einstellungen">Einstellungen</a></p>`)

	offen := 0
	for _, best := range orders {
		if !best.Done {
			offen++
		}
	}

	if len(orders) == 0 {
		b.WriteString(`<p>No order yet. Put <code>[[bestellung]]</code> into a page, ` +
			`and the list of your products with quantity fields stands there.</p>`)
		return plugin.AdminOut{Title: "Bestellungen", HTML: b.String()}, nil
	}

	b.WriteString(`<p>`)
	if offen == 0 {
		b.WriteString(`Alles abgehakt.`)
	} else {
		b.WriteString(zahlwort(offen, "Bestellung wartet", "Bestellungen warten") + ` auf dich.`)
	}
	b.WriteString(`</p>`)

	b.WriteString(`<table><thead><tr>` +
		`<th>Eingegangen</th><th>Wer</th><th>Was</th><th>Summe</th><th></th>` +
		`</tr></thead><tbody>`)
	for _, best := range orders {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(shortDate(best.Eingegangen)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(best.Name))
		if best.Done {
			b.WriteString(` <em>erledigt</em>`)
		}
		b.WriteString(`</td>`)
		b.WriteString(`<td>` + html.EscapeString(zusammenfassung(best)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(summeText(best)) + `</td>`)
		b.WriteString(`<td><a href="?ansicht=bestellung&amp;id=` +
			html.EscapeString(url.QueryEscape(best.ID)) + `">Ansehen</a></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return plugin.AdminOut{Title: "Bestellungen", HTML: b.String()}, nil
}

// single shows one order with everything in it.
func single(in plugin.AdminIn, id string) (plugin.AdminOut, error) {
	best, da := loadOrder(id)
	if !da {
		return plugin.AdminOut{Title: "Bestellung",
			HTML: `<p>This order no longer exists.</p><p><a href="?">Back to the list</a></p>`}, nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; Alle Bestellungen</a></p>`)

	b.WriteString(`<h3>Bestellt</h3><table><thead><tr>` +
		`<th>Menge</th><th>Produkt</th><th>Einzelpreis</th></tr></thead><tbody>`)
	for _, p := range best.Posten {
		b.WriteString(`<tr><td>` + html.EscapeString(zahl(p.Quantity)) + `</td>`)
		b.WriteString(`<td><a href="/` + html.EscapeString(p.Slug) + `">` + html.EscapeString(p.Titel) + `</a></td>`)
		price := best.Currency + " " + p.Price
		if p.Einheit != "" {
			price += " / " + p.Einheit
		}
		b.WriteString(`<td>` + html.EscapeString(price) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	b.WriteString(`<p><strong>Summe: ` + html.EscapeString(summeText(best)) + `</strong>`)
	if !best.SummeBekannt {
		b.WriteString(` <em>— a price could not be read as a number, please check.</em>`)
	}
	b.WriteString(`</p>`)

	b.WriteString(`<h3>Wer</h3><dl>`)
	zeile := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(`<dt>` + html.EscapeString(k) + `</dt><dd>` + html.EscapeString(v) + `</dd>`)
	}
	zeile("Name", best.Name)
	b.WriteString(`<dt>E-Mail</dt><dd><a href="mailto:` + html.EscapeString(best.Email) + `">` +
		html.EscapeString(best.Email) + `</a></dd>`)
	zeile("Telefon", best.Telefon)
	zeile("Adresse", best.Address)
	zeile("Bemerkung", best.Bemerkung)
	zeile("Eingegangen", shortDate(best.Eingegangen))
	zeile("Bestellt auf", best.Page)
	b.WriteString(`</dl>`)

	b.WriteString(`<form method="POST"><input type="hidden" name="id" value="` +
		html.EscapeString(best.ID) + `">`)
	if best.Done {
		b.WriteString(`<button type="submit" name="aktion" value="offen">Wieder als offen markieren</button> `)
	} else {
		b.WriteString(`<button type="submit" name="aktion" value="erledigt">Abhaken</button> `)
	}
	b.WriteString(`<button type="submit" name="aktion" value="loeschen">Delete</button>`)
	b.WriteString(`</form>`)

	return plugin.AdminOut{Title: "Bestellung von " + best.Name, HTML: b.String()}, nil
}

// einstellungsbildschirm ist, wo die Feldnamen stehen.
func einstellungsbildschirm(in plugin.AdminIn) (plugin.AdminOut, error) {
	e := einstellungenLaden()
	produkte, err := readProducts(e)
	if err != nil {
		produkte = nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; Alle Bestellungen</a></p>`)
	b.WriteString(`<p>A product is a published page that has filled in the price field. ` +
		`You create the fields under <em>Fields</em>; here it only says which of them plays which role.</p>`)

	if len(produkte) == 0 {
		b.WriteString(`<p><strong>The farm shop currently finds no product.</strong> ` +
			`Check that the price field's name is right and that at least one published page has filled it in.</p>`)
	} else {
		b.WriteString(`<p>` + html.EscapeString(zahlwort(len(produkte), "Produkt gefunden", "Produkte gefunden")) + `: `)
		namen := make([]string, 0, len(produkte))
		for _, p := range produkte {
			namen = append(namen, p.Titel)
		}
		b.WriteString(html.EscapeString(strings.Join(namen, ", ")) + `.</p>`)
	}

	b.WriteString(`<form method="POST"><input type="hidden" name="aktion" value="einstellungen">`)
	eingabe := func(name, beschriftung, wert, hilfe string) {
		b.WriteString(`<p><label for="e_` + name + `">` + html.EscapeString(beschriftung) + `</label>`)
		b.WriteString(`<input type="text" id="e_` + name + `" name="` + name + `" value="` +
			html.EscapeString(wert) + `">`)
		if hilfe != "" {
			b.WriteString(`<span>` + html.EscapeString(hilfe) + `</span>`)
		}
		b.WriteString(`</p>`)
	}
	eingabe("preis_feld", "Key of the price field", e.PriceField,
		"If this field is filled in on a page, the page is a product.")
	eingabe("einheit_feld", "Key of the unit field", e.UnitField,
		"Optional. Stands after the price: “per kilo”.")
	eingabe("zustand_feld", "Key of the availability field", e.StatusField,
		"Optional. Shown beside the product.")
	eingabe("ausverkauft_wert", "Value that means “cannot be ordered”", e.SoldOutValue,
		"If the availability field carries this value, there is no quantity field.")
	eingabe("waehrung", "Currency", e.Currency, "Stands before the price.")

	b.WriteString(`<p><label for="e_hinweis">Hint above the form</label>`)
	b.WriteString(`<textarea id="e_hinweis" name="hinweis" rows="3">` +
		html.EscapeString(e.Hint) + `</textarea>`)
	b.WriteString(`<span>How it is delivered and how it is paid for belongs here — both happen outside this program.</span></p>`)

	b.WriteString(`<p><button type="submit">Speichern</button></p></form>`)
	return plugin.AdminOut{Title: "Hofladen einrichten", HTML: b.String()}, nil
}

// --- Kleinkram --------------------------------------------------------------

func zusammenfassung(b order) string {
	teile := make([]string, 0, len(b.Posten))
	for i, p := range b.Posten {
		if i == 3 {
			teile = append(teile, "…")
			break
		}
		teile = append(teile, zahl(p.Quantity)+" × "+p.Titel)
	}
	return strings.Join(teile, ", ")
}

func summeText(b order) string {
	if !b.SummeBekannt {
		return "?"
	}
	return b.Currency + " " + amountText(b.Summe)
}

// shortDate turns 2026-08-06T14:22:31Z into 06.08.2026, 14:22.
//
// By hand rather than with time.Parse: the format is known, and a failure should
// not cost the screen — then what is stored simply stands there.
func shortDate(iso string) string {
	if len(iso) < 16 || iso[4] != '-' || iso[10] != 'T' {
		return iso
	}
	return iso[8:10] + "." + iso[5:7] + "." + iso[0:4] + ", " + iso[11:16]
}

func zahl(n int) string {
	if n < 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	if out == "" {
		return "0"
	}
	return out
}

func zahlwort(n int, eins, viele string) string {
	if n == 1 {
		return "1 " + eins
	}
	return zahl(n) + " " + viele
}
