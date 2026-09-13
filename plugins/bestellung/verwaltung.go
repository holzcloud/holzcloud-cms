package main

import (
	"fmt"
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
		return settingsScreen(in)
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
				Flash:      plugin.T("Without a price field the farm shop does not know what a product is."),
				FlashError: true}, nil
		}
		if err := saveSettings(e); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=einstellungen", Flash: "Gespeichert."}, nil

	case "erledigt", "offen":
		b, da := loadOrder(form.Get("id"))
		if !da {
			return plugin.AdminOut{Redirect: "?", Flash: plugin.T("This order no longer exists."), FlashError: true}, nil
		}
		b.Done = form.Get("aktion") == "erledigt"
		if err := saveOrder(b); err != nil {
			return plugin.AdminOut{}, err
		}
		// Two whole sentences, not one glued to a word. Which half of a sentence
		// a language can swap out is its own business, not this function's.
		flash := plugin.T("Order marked as open again.")
		if b.Done {
			flash = plugin.T("Order ticked off.")
		}
		return plugin.AdminOut{Redirect: "?", Flash: flash}, nil

	case "loeschen":
		if err := plugin.Delete(prefixOrder + form.Get("id")); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?", Flash: plugin.T("Order deleted.")}, nil
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
	b.WriteString(`<p><a href="?ansicht=einstellungen">` + html.EscapeString(plugin.T("Settings")) + `</a></p>`)

	offen := 0
	for _, best := range orders {
		if !best.Done {
			offen++
		}
	}

	if len(orders) == 0 {
		b.WriteString(`<p>No order yet. Put <code>[[bestellung]]</code> into a page, ` +
			`and the list of your products with quantity fields stands there.</p>`)
		return plugin.AdminOut{Title: plugin.T("Orders"), HTML: b.String()}, nil
	}

	b.WriteString(`<p>`)
	if offen == 0 {
		b.WriteString(html.EscapeString(plugin.T("Everything ticked off.")))
	} else {
		b.WriteString(html.EscapeString(plugin.Tf("%d orders are waiting for you.", offen)))
	}
	b.WriteString(`</p>`)

	b.WriteString(`<table><thead><tr>` +
		fmt.Sprintf(`<th>%s</th><th>%s</th><th>%s</th><th>%s</th><th></th>`,
			html.EscapeString(plugin.T("Received")), html.EscapeString(plugin.T("Who")),
			html.EscapeString(plugin.T("What")), html.EscapeString(plugin.T("Total"))) +
		`</tr></thead><tbody>`)
	for _, best := range orders {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(shortDate(best.Eingegangen)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(best.Name))
		if best.Done {
			b.WriteString(` <em>` + html.EscapeString(plugin.T("done")) + `</em>`)
		}
		b.WriteString(`</td>`)
		b.WriteString(`<td>` + html.EscapeString(zusammenfassung(best)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(summeText(best)) + `</td>`)
		b.WriteString(`<td><a href="?ansicht=bestellung&amp;id=` +
			html.EscapeString(url.QueryEscape(best.ID)) + `">` + html.EscapeString(plugin.T("View")) + `</a></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return plugin.AdminOut{Title: plugin.T("Orders"), HTML: b.String()}, nil
}

// single shows one order with everything in it.
func single(in plugin.AdminIn, id string) (plugin.AdminOut, error) {
	best, da := loadOrder(id)
	if !da {
		return plugin.AdminOut{Title: plugin.T("Order"),
			HTML: `<p>` + html.EscapeString(plugin.T("This order no longer exists.")) + `</p>` +
				`<p><a href="?">` + html.EscapeString(plugin.T("Back to the list")) + `</a></p>`}, nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; ` + html.EscapeString(plugin.T("All orders")) + `</a></p>`)

	fmt.Fprintf(&b, `<h3>%s</h3><table><thead><tr>`+
		`<th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		html.EscapeString(plugin.T("Ordered")), html.EscapeString(plugin.T("Quantity")),
		html.EscapeString(plugin.T("Product")), html.EscapeString(plugin.T("Unit price")))
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
		b.WriteString(` <em>` + html.EscapeString(plugin.T("— a price could not be read as a number, please check.")) + `</em>`)
	}
	b.WriteString(`</p>`)

	b.WriteString(`<h3>` + html.EscapeString(plugin.T("Who")) + `</h3><dl>`)
	row := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(`<dt>` + html.EscapeString(k) + `</dt><dd>` + html.EscapeString(v) + `</dd>`)
	}
	row(plugin.T("Name"), best.Name)
	b.WriteString(`<dt>` + html.EscapeString(plugin.T("E-mail")) + `</dt><dd><a href="mailto:` +
		html.EscapeString(best.Email) + `">` + html.EscapeString(best.Email) + `</a></dd>`)
	row(plugin.T("Telephone"), best.Telefon)
	row(plugin.T("Address"), best.Address)
	row(plugin.T("Remark"), best.Bemerkung)
	row(plugin.T("Received"), shortDate(best.Eingegangen))
	row(plugin.T("Ordered on"), best.Page)
	b.WriteString(`</dl>`)

	b.WriteString(`<form method="POST"><input type="hidden" name="id" value="` +
		html.EscapeString(best.ID) + `">`)
	if best.Done {
		b.WriteString(`<button type="submit" name="aktion" value="offen">` + html.EscapeString(plugin.T("Mark as open again")) + `</button> `)
	} else {
		b.WriteString(`<button type="submit" name="aktion" value="erledigt">` + html.EscapeString(plugin.T("Tick off")) + `</button> `)
	}
	b.WriteString(`<button type="submit" name="aktion" value="loeschen">Delete</button>`)
	b.WriteString(`</form>`)

	return plugin.AdminOut{Title: plugin.Tf("Order from %s", best.Name), HTML: b.String()}, nil
}

// einstellungsbildschirm ist, wo die Feldnamen stehen.
func settingsScreen(in plugin.AdminIn) (plugin.AdminOut, error) {
	e := einstellungenLaden()
	produkte, err := readProducts(e)
	if err != nil {
		produkte = nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; ` + html.EscapeString(plugin.T("All orders")) + `</a></p>`)
	b.WriteString(`<p>A product is a published page that has filled in the price field. ` +
		plugin.T("You create the fields under <em>Fields</em>; here it only says which of them plays which role.") + `</p>`)

	if len(produkte) == 0 {
		b.WriteString(`<p><strong>` + html.EscapeString(plugin.T("The farm shop currently finds no product.")) + `</strong> ` +
			`Check that the price field's name is right and that at least one published page has filled it in.</p>`)
	} else {
		b.WriteString(`<p>` + html.EscapeString(plugin.Tf("%d products found", len(produkte))) + `: `)
		namen := make([]string, 0, len(produkte))
		for _, p := range produkte {
			namen = append(namen, p.Titel)
		}
		b.WriteString(html.EscapeString(strings.Join(namen, ", ")) + `.</p>`)
	}

	b.WriteString(`<form method="POST"><input type="hidden" name="aktion" value="einstellungen">`)
	eingabe := func(name, label, value, hilfe string) {
		b.WriteString(`<p><label for="e_` + name + `">` + html.EscapeString(label) + `</label>`)
		b.WriteString(`<input type="text" id="e_` + name + `" name="` + name + `" value="` +
			html.EscapeString(value) + `">`)
		if hilfe != "" {
			b.WriteString(`<span>` + html.EscapeString(hilfe) + `</span>`)
		}
		b.WriteString(`</p>`)
	}
	eingabe("preis_feld", plugin.T("Key of the price field"), e.PriceField,
		plugin.T("If this field is filled in on a page, the page is a product."))
	eingabe("einheit_feld", plugin.T("Key of the unit field"), e.UnitField,
		plugin.T("Optional. Stands after the price: “per kilo”."))
	eingabe("zustand_feld", plugin.T("Key of the availability field"), e.StatusField,
		plugin.T("Optional. Shown beside the product."))
	eingabe("ausverkauft_wert", plugin.T("Value that means “cannot be ordered”"), e.SoldOutValue,
		plugin.T("If the availability field carries this value, there is no quantity field."))
	eingabe("waehrung", plugin.T("Currency"), e.Currency, plugin.T("Stands before the price."))

	b.WriteString(`<p><label for="e_hinweis">` + html.EscapeString(plugin.T("Hint above the form")) + `</label>`)
	b.WriteString(`<textarea id="e_hinweis" name="hinweis" rows="3">` +
		html.EscapeString(e.Hint) + `</textarea>`)
	b.WriteString(`<span>How it is delivered and how it is paid for belongs here — both happen outside this program.</span></p>`)

	b.WriteString(`<p><button type="submit">` + html.EscapeString(plugin.T("Save")) + `</button></p></form>`)
	return plugin.AdminOut{Title: plugin.T("Set up the farm shop"), HTML: b.String()}, nil
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
