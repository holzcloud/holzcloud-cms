package main

import (
	"html"
	"net/url"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Der Bildschirm in der Verwaltung.
//
// Drei Ansichten in einer: die Liste der Bestellungen, eine einzelne Bestellung
// und die Einstellungen. Welche gemeint ist, sagt die Abfragezeichenkette — ein
// Plugin bekommt eine Adresse und kann auf sich selbst verweisen.

func verwaltung(in plugin.AdminIn) (plugin.AdminOut, error) {
	q, _ := url.ParseQuery(in.Query)

	if in.Method == "POST" {
		return aktion(in, q)
	}

	switch q.Get("ansicht") {
	case "einstellungen":
		return einstellungsbildschirm(in)
	case "bestellung":
		return einzelne(in, q.Get("id"))
	default:
		return liste(in)
	}
}

// aktion führt aus, was ein Knopf ausgelöst hat.
func aktion(in plugin.AdminIn, q url.Values) (plugin.AdminOut, error) {
	form := url.Values{}
	for k, v := range in.Form {
		form[k] = v
	}

	switch form.Get("aktion") {
	case "einstellungen":
		e := einstellungen{
			PreisFeld:       strings.TrimSpace(form.Get("preis_feld")),
			EinheitFeld:     strings.TrimSpace(form.Get("einheit_feld")),
			ZustandFeld:     strings.TrimSpace(form.Get("zustand_feld")),
			AusverkauftWert: strings.TrimSpace(form.Get("ausverkauft_wert")),
			Waehrung:        strings.TrimSpace(form.Get("waehrung")),
			Hinweis:         strings.TrimSpace(form.Get("hinweis")),
		}
		if e.PreisFeld == "" {
			return plugin.AdminOut{Redirect: "?ansicht=einstellungen",
				Flash: "Ohne Preisfeld weiss der Hofladen nicht, was ein Produkt ist.", FlashError: true}, nil
		}
		if err := einstellungenSichern(e); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=einstellungen", Flash: "Gespeichert."}, nil

	case "erledigt", "offen":
		b, da := bestellungLaden(form.Get("id"))
		if !da {
			return plugin.AdminOut{Redirect: "?", Flash: "Diese Bestellung gibt es nicht mehr.", FlashError: true}, nil
		}
		b.Erledigt = form.Get("aktion") == "erledigt"
		if err := bestellungSichern(b); err != nil {
			return plugin.AdminOut{}, err
		}
		wort := "als offen markiert"
		if b.Erledigt {
			wort = "abgehakt"
		}
		return plugin.AdminOut{Redirect: "?", Flash: "Bestellung " + wort + "."}, nil

	case "loeschen":
		if err := plugin.Delete(praefixBestellung + form.Get("id")); err != nil {
			return plugin.AdminOut{}, err
		}
		return plugin.AdminOut{Redirect: "?", Flash: "Bestellung gelöscht."}, nil
	}
	return plugin.AdminOut{Redirect: "?"}, nil
}

// liste zeigt alle Bestellungen.
func liste(in plugin.AdminIn) (plugin.AdminOut, error) {
	bestellungen, err := alleBestellungen()
	if err != nil {
		return plugin.AdminOut{}, err
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?ansicht=einstellungen">Einstellungen</a></p>`)

	offen := 0
	for _, best := range bestellungen {
		if !best.Erledigt {
			offen++
		}
	}

	if len(bestellungen) == 0 {
		b.WriteString(`<p>Noch keine Bestellung. Setze <code>[[bestellung]]</code> in eine Seite, ` +
			`dann steht dort die Liste deiner Produkte mit Mengenfeldern.</p>`)
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
	for _, best := range bestellungen {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(kurzesDatum(best.Eingegangen)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(best.Name))
		if best.Erledigt {
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

// einzelne zeigt eine Bestellung mit allem, was drinsteht.
func einzelne(in plugin.AdminIn, id string) (plugin.AdminOut, error) {
	best, da := bestellungLaden(id)
	if !da {
		return plugin.AdminOut{Title: "Bestellung",
			HTML: `<p>Diese Bestellung gibt es nicht mehr.</p><p><a href="?">Zurück zur Liste</a></p>`}, nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; Alle Bestellungen</a></p>`)

	b.WriteString(`<h3>Bestellt</h3><table><thead><tr>` +
		`<th>Menge</th><th>Produkt</th><th>Einzelpreis</th></tr></thead><tbody>`)
	for _, p := range best.Posten {
		b.WriteString(`<tr><td>` + html.EscapeString(zahl(p.Menge)) + `</td>`)
		b.WriteString(`<td><a href="/` + html.EscapeString(p.Slug) + `">` + html.EscapeString(p.Titel) + `</a></td>`)
		preis := best.Waehrung + " " + p.Preis
		if p.Einheit != "" {
			preis += " / " + p.Einheit
		}
		b.WriteString(`<td>` + html.EscapeString(preis) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	b.WriteString(`<p><strong>Summe: ` + html.EscapeString(summeText(best)) + `</strong>`)
	if !best.SummeBekannt {
		b.WriteString(` <em>— ein Preis liess sich nicht als Zahl lesen, bitte nachrechnen.</em>`)
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
	zeile("Adresse", best.Adresse)
	zeile("Bemerkung", best.Bemerkung)
	zeile("Eingegangen", kurzesDatum(best.Eingegangen))
	zeile("Bestellt auf", best.Seite)
	b.WriteString(`</dl>`)

	b.WriteString(`<form method="POST"><input type="hidden" name="id" value="` +
		html.EscapeString(best.ID) + `">`)
	if best.Erledigt {
		b.WriteString(`<button type="submit" name="aktion" value="offen">Wieder als offen markieren</button> `)
	} else {
		b.WriteString(`<button type="submit" name="aktion" value="erledigt">Abhaken</button> `)
	}
	b.WriteString(`<button type="submit" name="aktion" value="loeschen">Löschen</button>`)
	b.WriteString(`</form>`)

	return plugin.AdminOut{Title: "Bestellung von " + best.Name, HTML: b.String()}, nil
}

// einstellungsbildschirm ist, wo die Feldnamen stehen.
func einstellungsbildschirm(in plugin.AdminIn) (plugin.AdminOut, error) {
	e := einstellungenLaden()
	produkte, err := produkteLesen(e)
	if err != nil {
		produkte = nil
	}

	var b strings.Builder
	b.WriteString(`<p><a href="?">&#8592; Alle Bestellungen</a></p>`)
	b.WriteString(`<p>Ein Produkt ist eine veröffentlichte Seite, die das Preisfeld ausgefüllt hat. ` +
		`Die Felder legst du unter <em>Felder</em> an; hier steht nur, welches davon welche Rolle spielt.</p>`)

	if len(produkte) == 0 {
		b.WriteString(`<p><strong>Zurzeit findet der Hofladen kein Produkt.</strong> ` +
			`Prüfe, ob der Name des Preisfeldes stimmt und ob mindestens eine veröffentlichte Seite ihn ausgefüllt hat.</p>`)
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
	eingabe("preis_feld", "Kennung des Preisfeldes", e.PreisFeld,
		"Ist dieses Feld an einer Seite ausgefüllt, ist die Seite ein Produkt.")
	eingabe("einheit_feld", "Kennung des Einheitsfeldes", e.EinheitFeld,
		"Freiwillig. Steht hinter dem Preis: „pro Kilo“.")
	eingabe("zustand_feld", "Kennung des Verfügbarkeitsfeldes", e.ZustandFeld,
		"Freiwillig. Wird neben dem Produkt angezeigt.")
	eingabe("ausverkauft_wert", "Wert, der „nicht bestellbar“ heisst", e.AusverkauftWert,
		"Trägt das Verfügbarkeitsfeld diesen Wert, gibt es kein Mengenfeld.")
	eingabe("waehrung", "Währung", e.Waehrung, "Steht vor dem Preis.")

	b.WriteString(`<p><label for="e_hinweis">Hinweis über dem Formular</label>`)
	b.WriteString(`<textarea id="e_hinweis" name="hinweis" rows="3">` +
		html.EscapeString(e.Hinweis) + `</textarea>`)
	b.WriteString(`<span>Hierhin gehört, wie geliefert und wie bezahlt wird — beides passiert ausserhalb dieses Programms.</span></p>`)

	b.WriteString(`<p><button type="submit">Speichern</button></p></form>`)
	return plugin.AdminOut{Title: "Hofladen einrichten", HTML: b.String()}, nil
}

// --- Kleinkram --------------------------------------------------------------

func zusammenfassung(b bestellung) string {
	teile := make([]string, 0, len(b.Posten))
	for i, p := range b.Posten {
		if i == 3 {
			teile = append(teile, "…")
			break
		}
		teile = append(teile, zahl(p.Menge)+" × "+p.Titel)
	}
	return strings.Join(teile, ", ")
}

func summeText(b bestellung) string {
	if !b.SummeBekannt {
		return "?"
	}
	return b.Waehrung + " " + betragTexten(b.Summe)
}

// kurzesDatum macht aus 2026-08-06T14:22:31Z ein 06.08.2026, 14:22.
//
// Von Hand statt mit time.Parse: das Format ist bekannt, und ein Fehlschlag
// soll den Bildschirm nicht kosten — dann steht eben da, was gespeichert ist.
func kurzesDatum(iso string) string {
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
