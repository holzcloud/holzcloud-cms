package main

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Der Bildschirm, auf dem Formulare zusammengestellt werden.
//
// Er hat zwei Ansichten, und die Adresse sagt welche: ohne Angabe die
// Nachrichten, mit ?ansicht=formulare die Liste der Formulare, mit
// ?ansicht=formular&kennung=… der Editor für eines. Ein Plugin bekommt die
// Abfragezeichenkette vom Host, deshalb kann es das überhaupt.
//
// Der Feldeditor arbeitet wie der Baustein-Editor der Seiten: Hinzufügen,
// Verschieben und Entfernen sind Absende-Knöpfe, und der Bildschirm wird danach
// neu gezeichnet. Ohne JavaScript, weil ein Plugin keines mitbringen darf — und
// weil es so auch nicht kaputtgehen kann.

const (
	ansichtNachrichten = ""
	ansichtFormulare   = "formulare"
	ansichtFormular    = "formular"
)

// verwaltung verteilt auf die Ansichten.
func verwaltung(in plugin.AdminIn) (plugin.AdminOut, error) {
	q, _ := url.ParseQuery(in.Query)

	if in.Method == "POST" {
		if out, behandelt, err := formularAktion(in, q); behandelt {
			return out, err
		}
	}

	switch q.Get("ansicht") {
	case ansichtFormulare:
		return formularliste()
	case ansichtFormular:
		return formulareditor(in, q.Get("kennung"))
	default:
		return bildschirm(in)
	}
}

// navigation ist die Zeile, die zwischen den Ansichten wechselt.
func navigation(aktuell string) string {
	art := func(name, beschriftung, ansicht string) string {
		klasse := "btn btn--sm"
		if aktuell == name {
			klasse += " btn--primary"
		}
		ziel := "."
		if ansicht != "" {
			ziel = "?ansicht=" + ansicht
		}
		return fmt.Sprintf(`<a class="%s" href="%s">%s</a> `, klasse, ziel, beschriftung)
	}
	return `<p class="table-actions">` +
		art(ansichtNachrichten, "Nachrichten", "") +
		art(ansichtFormulare, "Formulare", ansichtFormulare) +
		`</p>`
}

// --- Liste ------------------------------------------------------------------

func formularliste() (plugin.AdminOut, error) {
	var b strings.Builder
	b.WriteString(navigation(ansichtFormulare))
	b.WriteString(`<p>Ein eigenes Formular fragt genau das, was du wissen willst. ` +
		`Setze es mit seiner Marke in eine Seite, so wie einen Textbaustein.</p>`)

	liste := alleFormulare()
	if len(liste) == 0 {
		b.WriteString(`<p class="empty">Noch kein eigenes Formular. ` +
			`Das eingebaute Kontaktformular steht weiterhin unter <code>[[formular]]</code>.</p>`)
	} else {
		b.WriteString(`<table class="table"><thead><tr>` +
			`<th>Formular</th><th>Felder</th><th>Marke für die Seite</th><th></th>` +
			`</tr></thead><tbody>`)
		for _, f := range liste {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td><code>%s</code></td><td class="table-actions">`,
				html.EscapeString(f.Name), len(f.Felder), html.EscapeString(markeFuer(f.Kennung)))
			fmt.Fprintf(&b, `<a class="btn btn--sm" href="?ansicht=formular&amp;kennung=%s">Bearbeiten</a>`,
				html.EscapeString(f.Kennung))
			fmt.Fprintf(&b, `<form method="POST" class="inline-form">`+
				`<input type="hidden" name="loeschen_formular" value="%s">`+
				`<button type="submit" class="btn btn--sm btn--danger">Löschen</button></form>`,
				html.EscapeString(f.Kennung))
			b.WriteString(`</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}

	b.WriteString(`<form method="POST" class="stack">` +
		`<fieldset><legend>Neues Formular</legend>` +
		`<label for="neu-name">Name</label>` +
		`<input type="text" id="neu-name" name="neues_formular" required maxlength="80" ` +
		`placeholder="Anmeldung zum Hoffest">` +
		`<button type="submit" class="btn btn--primary">Anlegen</button>` +
		`</fieldset></form>`)

	return plugin.AdminOut{Title: "Formulare", HTML: b.String()}, nil
}

// --- Editor -----------------------------------------------------------------

func formulareditor(in plugin.AdminIn, kennung string) (plugin.AdminOut, error) {
	f, ok := formularLaden(kennung)
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "Dieses Formular gibt es nicht.", FlashError: true}, nil
	}

	e := html.EscapeString
	var b strings.Builder
	b.WriteString(navigation(ansichtFormulare))
	fmt.Fprintf(&b, `<p>In der Seite platzieren mit <code>%s</code>.</p>`, e(markeFuer(f.Kennung)))

	b.WriteString(`<form method="POST" class="stack">`)
	fmt.Fprintf(&b, `<input type="hidden" name="kennung" value="%s">`, e(f.Kennung))

	b.WriteString(`<fieldset><legend>Formular</legend>`)
	fmt.Fprintf(&b, `<label for="f-name">Name</label>`+
		`<input type="text" id="f-name" name="name" value="%s" maxlength="80" required>`, e(f.Name))
	fmt.Fprintf(&b, `<label for="f-betreff">Betreff der Benachrichtigung</label>`+
		`<input type="text" id="f-betreff" name="betreff" value="%s" maxlength="120" `+
		`placeholder="Sonst der Name des Formulars">`, e(f.Betreff))
	fmt.Fprintf(&b, `<label for="f-dank">Satz nach dem Absenden</label>`+
		`<input type="text" id="f-dank" name="dank" value="%s" maxlength="200" `+
		`placeholder="Danke, die Nachricht ist angekommen. Wir melden uns.">`, e(f.Dank))
	b.WriteString(`</fieldset>`)

	for i, fe := range f.Felder {
		p := "fe" + strconv.Itoa(i)
		fmt.Fprintf(&b, `<fieldset><legend>%d. %s</legend>`, i+1, e(fe.Beschriftung))
		fmt.Fprintf(&b, `<input type="hidden" name="%s.kennung" value="%s">`, p, e(fe.Kennung))

		fmt.Fprintf(&b, `<label for="%s-b">Frage</label>`+
			`<input type="text" id="%s-b" name="%s.beschriftung" value="%s" maxlength="120" required>`,
			p, p, p, e(fe.Beschriftung))

		fmt.Fprintf(&b, `<label for="%s-a">Art</label><select id="%s-a" name="%s.art">`, p, p, p)
		for _, a := range feldArten {
			aus := ""
			if a.Art == fe.Art {
				aus = " selected"
			}
			fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, a.Art, aus, e(a.Name))
		}
		b.WriteString(`</select>`)

		fmt.Fprintf(&b, `<label for="%s-w">Zur Auswahl (eine Möglichkeit pro Zeile)</label>`+
			`<textarea id="%s-w" name="%s.auswahl" rows="3">%s</textarea>`,
			p, p, p, e(strings.Join(fe.Auswahl, "\n")))

		fmt.Fprintf(&b, `<label for="%s-h">Hinweis unter dem Feld</label>`+
			`<input type="text" id="%s-h" name="%s.hinweis" value="%s" maxlength="200">`,
			p, p, p, e(fe.Hinweis))

		an := ""
		if fe.Pflicht {
			an = " checked"
		}
		fmt.Fprintf(&b, `<label><input type="checkbox" name="%s.pflicht" value="1"%s> `+
			`Muss ausgefüllt werden</label>`, p, an)

		b.WriteString(`<p class="table-actions">`)
		aktionsknopf(&b, "feldaktion", "hoch:"+strconv.Itoa(i), "↑ nach oben", "")
		aktionsknopf(&b, "feldaktion", "runter:"+strconv.Itoa(i), "↓ nach unten", "")
		aktionsknopf(&b, "feldaktion", "weg:"+strconv.Itoa(i), "Feld entfernen", "btn--danger")
		b.WriteString(`</p></fieldset>`)
	}

	if len(f.Felder) == 0 {
		b.WriteString(`<p class="empty">Noch kein Feld. Ein Formular ohne Felder wird nicht angezeigt.</p>`)
	}

	b.WriteString(`<p class="table-actions">`)
	aktionsknopf(&b, "feldaktion", "neu", "Feld hinzufügen", "")
	b.WriteString(`</p>`)
	b.WriteString(`<p><button type="submit" name="sichern" value="1" class="btn btn--primary">` +
		`Formular speichern</button></p>`)
	b.WriteString(`</form>`)

	if _, hat := f.ersteArt(ArtEmail); !hat && len(f.Felder) > 0 {
		b.WriteString(`<p class="text-muted">Kein Feld für eine E-Mail-Adresse: ` +
			`Auf eine Anfrage über dieses Formular lässt sich dann nicht per Mail antworten.</p>`)
	}

	return plugin.AdminOut{Title: "Formular: " + f.Name, HTML: b.String()}, nil
}

// --- Aktionen ---------------------------------------------------------------

// formularAktion behandelt alles, was auf den beiden Formularansichten
// abgesendet wird. Der zweite Rückgabewert sagt, ob es dazugehörte.
func formularAktion(in plugin.AdminIn, q url.Values) (plugin.AdminOut, bool, error) {
	switch {
	case len(in.Form["neues_formular"]) > 0:
		name := strings.TrimSpace(in.Form["neues_formular"][0])
		kennung := kennungAus(name)
		if kennung == "" {
			return plugin.AdminOut{Redirect: "?ansicht=formulare",
				Flash:      "Aus diesem Namen lässt sich keine Kennung bilden. Bitte Buchstaben verwenden.",
				FlashError: true}, true, nil
		}
		if _, da := formularLaden(kennung); da {
			return plugin.AdminOut{Redirect: "?ansicht=formulare",
				Flash: "Ein Formular mit dieser Kennung gibt es schon.", FlashError: true}, true, nil
		}
		f := formular{Kennung: kennung, Name: name}
		if err := formularSichern(f.saeubern()); err != nil {
			return plugin.AdminOut{}, true, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=formular&kennung=" + kennung,
			Flash: "Angelegt. Jetzt die Felder festlegen."}, true, nil

	case len(in.Form["loeschen_formular"]) > 0:
		kennung := in.Form["loeschen_formular"][0]
		if reKennung.MatchString(kennung) {
			_ = plugin.Delete(praefixFormular + kennung)
		}
		// Die schon eingegangenen Nachrichten bleiben: sie sind der Grund,
		// warum es das Formular gab, und mit ihm zu verschwinden wäre das
		// Gegenteil von dem, was jemand beim Aufräumen erwartet.
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "Formular gelöscht. Die eingegangenen Nachrichten bleiben."}, true, nil

	case len(in.Form["feldaktion"]) > 0, len(in.Form["sichern"]) > 0:
		return formularSpeichern(in, q)
	}
	return plugin.AdminOut{}, false, nil
}

// formularSpeichern liest den Editor, wendet eine Aktion an und sichert.
//
// Auch eine Aktion sichert: der Editor ist ein Formular, und ein Knopf schickt
// ohnehin alles mit. Nichts zu sichern hiesse, dass ein Feld hinzufügen alles
// verwirft, was daneben schon getippt war.
func formularSpeichern(in plugin.AdminIn, q url.Values) (plugin.AdminOut, bool, error) {
	kennung := ersterWert(in.Form, "kennung")
	f, ok := formularLaden(kennung)
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "Dieses Formular gibt es nicht.", FlashError: true}, true, nil
	}

	f.Name = ersterWert(in.Form, "name")
	f.Betreff = ersterWert(in.Form, "betreff")
	f.Dank = ersterWert(in.Form, "dank")
	f.Felder = felderAusFormular(in.Form)

	if aktion := ersterWert(in.Form, "feldaktion"); aktion != "" {
		f.Felder = feldaktion(f.Felder, aktion)
	}

	f = f.saeubern()
	if err := formularSichern(f); err != nil {
		return plugin.AdminOut{}, true, err
	}
	out := plugin.AdminOut{Redirect: "?ansicht=formular&kennung=" + f.Kennung}
	if ersterWert(in.Form, "sichern") != "" {
		out.Flash = "Formular gespeichert."
	}
	return out, true, nil
}

// felderAusFormular liest die Feldliste. Wie im Baustein-Editor sind die Namen
// mit einer Nummer versehen, und die Nummern werden beim Lesen neu vergeben —
// ein entferntes Feld hinterlässt so keine Lücke.
func felderAusFormular(form map[string][]string) []feld {
	slots := map[int]*feld{}
	for key, werte := range form {
		if !strings.HasPrefix(key, "fe") || len(werte) == 0 {
			continue
		}
		rest := key[2:]
		punkt := strings.IndexByte(rest, '.')
		if punkt <= 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:punkt])
		if err != nil || n < 0 {
			continue
		}
		fe, da := slots[n]
		if !da {
			fe = &feld{}
			slots[n] = fe
		}
		switch rest[punkt+1:] {
		case "kennung":
			fe.Kennung = werte[0]
		case "beschriftung":
			fe.Beschriftung = werte[0]
		case "art":
			fe.Art = werte[0]
		case "hinweis":
			fe.Hinweis = werte[0]
		case "pflicht":
			fe.Pflicht = werte[0] != ""
		case "auswahl":
			fe.Auswahl = zeilen(werte[0])
		}
	}

	nummern := make([]int, 0, len(slots))
	for n := range slots {
		nummern = append(nummern, n)
	}
	sort.Ints(nummern)

	out := make([]feld, 0, len(nummern))
	for _, n := range nummern {
		out = append(out, *slots[n])
	}
	return out
}

// feldaktion verschiebt, entfernt oder ergänzt ein Feld.
func feldaktion(felder []feld, aktion string) []feld {
	name, arg, _ := strings.Cut(aktion, ":")
	if name == "neu" {
		if len(felder) >= maxFelder {
			return felder
		}
		return append(felder, feld{Beschriftung: "Neue Frage", Art: ArtText})
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 || n >= len(felder) {
		return felder
	}
	switch name {
	case "weg":
		return append(felder[:n:n], felder[n+1:]...)
	case "hoch":
		if n > 0 {
			felder[n-1], felder[n] = felder[n], felder[n-1]
		}
	case "runter":
		if n < len(felder)-1 {
			felder[n], felder[n+1] = felder[n+1], felder[n]
		}
	}
	return felder
}

func zeilen(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, z := range strings.Split(s, "\n") {
		if z = strings.TrimSpace(z); z != "" {
			out = append(out, z)
		}
	}
	return out
}

func ersterWert(form map[string][]string, name string) string {
	if v := form[name]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}
