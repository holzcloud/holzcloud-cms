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

// The screen on which forms are assembled.
//
// It has two views, and the address says which: with nothing the messages, with
// ?ansicht=formulare the list of forms, with ?ansicht=formular&kennung=… the
// editor for one. A plugin gets the query string from the host, which is what
// makes this possible at all.
//
// The field editor works like the pages' block editor: adding, moving and
// removing are submit buttons, and the screen is redrawn afterwards. Without
// JavaScript, because a plugin may bring none — and because that way it cannot
// break either.

const (
	ansichtNachrichten = ""
	ansichtFormulare   = "formulare"
	ansichtFormular    = "formular"
)

// verwaltung verteilt auf die Ansichten.
func verwaltung(in plugin.AdminIn) (plugin.AdminOut, error) {
	q, _ := url.ParseQuery(in.Query)

	if in.Method == "POST" {
		if out, behandelt, err := formAction(in, q); behandelt {
			return out, err
		}
	}

	switch q.Get("ansicht") {
	case ansichtFormulare:
		return formularliste()
	case ansichtFormular:
		return formulareditor(in, q.Get("kennung"))
	default:
		return screen(in)
	}
}

// navigation is the row that switches between the views.
func navigation(aktuell string) string {
	art := func(name, label, ansicht string) string {
		class := "btn btn--sm"
		if aktuell == name {
			class += " btn--primary"
		}
		ziel := "."
		if ansicht != "" {
			ziel = "?ansicht=" + ansicht
		}
		return fmt.Sprintf(`<a class="%s" href="%s">%s</a> `, class, ziel, label)
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
		`Put it into a page with its marker, the way you would a snippet.</p>`)

	liste := allForms()
	if len(liste) == 0 {
		b.WriteString(`<p class="empty">No form of your own yet. ` +
			`The built-in contact form is still available under <code>[[formular]]</code>.</p>`)
	} else {
		b.WriteString(`<table class="table"><thead><tr>` +
			`<th>Form</th><th>Fields</th><th>Marker for the page</th><th></th>` +
			`</tr></thead><tbody>`)
		for _, f := range liste {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td><code>%s</code></td><td class="table-actions">`,
				html.EscapeString(f.Name), len(f.Fields), html.EscapeString(markerFor(f.Key)))
			fmt.Fprintf(&b, `<a class="btn btn--sm" href="?ansicht=formular&amp;kennung=%s">Bearbeiten</a>`,
				html.EscapeString(f.Key))
			fmt.Fprintf(&b, `<form method="POST" class="inline-form">`+
				`<input type="hidden" name="loeschen_formular" value="%s">`+
				`<button type="submit" class="btn btn--sm btn--danger">Delete</button></form>`,
				html.EscapeString(f.Key))
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

func formulareditor(in plugin.AdminIn, key string) (plugin.AdminOut, error) {
	f, ok := formularLaden(key)
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "There is no such form.", FlashError: true}, nil
	}

	e := html.EscapeString
	var b strings.Builder
	b.WriteString(navigation(ansichtFormulare))
	fmt.Fprintf(&b, `<p>Place it in the page with <code>%s</code>.</p>`, e(markerFor(f.Key)))

	b.WriteString(`<form method="POST" class="stack">`)
	fmt.Fprintf(&b, `<input type="hidden" name="kennung" value="%s">`, e(f.Key))

	b.WriteString(`<fieldset><legend>Formular</legend>`)
	fmt.Fprintf(&b, `<label for="f-name">Name</label>`+
		`<input type="text" id="f-name" name="name" value="%s" maxlength="80" required>`, e(f.Name))
	fmt.Fprintf(&b, `<label for="f-betreff">Betreff der Benachrichtigung</label>`+
		`<input type="text" id="f-betreff" name="betreff" value="%s" maxlength="120" `+
		`placeholder="Otherwise the name of the form">`, e(f.Subject))
	fmt.Fprintf(&b, `<label for="f-dank">Sentence after submitting</label>`+
		`<input type="text" id="f-dank" name="dank" value="%s" maxlength="200" `+
		`placeholder="Danke, die Nachricht ist angekommen. Wir melden uns.">`, e(f.Dank))
	b.WriteString(`</fieldset>`)

	for i, fe := range f.Fields {
		p := "fe" + strconv.Itoa(i)
		fmt.Fprintf(&b, `<fieldset><legend>%d. %s</legend>`, i+1, e(fe.Label))
		fmt.Fprintf(&b, `<input type="hidden" name="%s.kennung" value="%s">`, p, e(fe.Key))

		fmt.Fprintf(&b, `<label for="%s-b">Frage</label>`+
			`<input type="text" id="%s-b" name="%s.beschriftung" value="%s" maxlength="120" required>`,
			p, p, p, e(fe.Label))

		fmt.Fprintf(&b, `<label for="%s-a">Art</label><select id="%s-a" name="%s.art">`, p, p, p)
		for _, a := range fieldKinds {
			aus := ""
			if a.Art == fe.Art {
				aus = " selected"
			}
			fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, a.Art, aus, e(a.Name))
		}
		b.WriteString(`</select>`)

		fmt.Fprintf(&b, `<label for="%s-w">Options (one per line)</label>`+
			`<textarea id="%s-w" name="%s.auswahl" rows="3">%s</textarea>`,
			p, p, p, e(strings.Join(fe.Choices, "\n")))

		fmt.Fprintf(&b, `<label for="%s-h">Hint under the field</label>`+
			`<input type="text" id="%s-h" name="%s.hinweis" value="%s" maxlength="200">`,
			p, p, p, e(fe.Hint))

		an := ""
		if fe.Required {
			an = " checked"
		}
		fmt.Fprintf(&b, `<label><input type="checkbox" name="%s.pflicht" value="1"%s> `+
			`Has to be filled in</label>`, p, an)

		b.WriteString(`<p class="table-actions">`)
		actionButton(&b, "feldaktion", "hoch:"+strconv.Itoa(i), "↑ nach oben", "")
		actionButton(&b, "feldaktion", "runter:"+strconv.Itoa(i), "↓ nach unten", "")
		actionButton(&b, "feldaktion", "weg:"+strconv.Itoa(i), "Feld entfernen", "btn--danger")
		b.WriteString(`</p></fieldset>`)
	}

	if len(f.Fields) == 0 {
		b.WriteString(`<p class="empty">No field yet. A form with no fields is not shown.</p>`)
	}

	b.WriteString(`<p class="table-actions">`)
	actionButton(&b, "feldaktion", "neu", "Add field", "")
	b.WriteString(`</p>`)
	b.WriteString(`<p><button type="submit" name="sichern" value="1" class="btn btn--primary">` +
		`Formular speichern</button></p>`)
	b.WriteString(`</form>`)

	if _, hat := f.ersteArt(ArtEmail); !hat && len(f.Fields) > 0 {
		b.WriteString(`<p class="text-muted">No field for an e-mail address: ` +
			`an enquiry through this form then cannot be answered by mail.</p>`)
	}

	return plugin.AdminOut{Title: "Formular: " + f.Name, HTML: b.String()}, nil
}

// --- Aktionen ---------------------------------------------------------------

// formAction handles everything submitted on the two form views. The second
// return value says whether it belonged to them.
func formAction(in plugin.AdminIn, q url.Values) (plugin.AdminOut, bool, error) {
	switch {
	case len(in.Form["neues_formular"]) > 0:
		name := strings.TrimSpace(in.Form["neues_formular"][0])
		key := keyFrom(name)
		if key == "" {
			return plugin.AdminOut{Redirect: "?ansicht=formulare",
				Flash:      "No key can be made from this name. Please use letters.",
				FlashError: true}, true, nil
		}
		if _, da := formularLaden(key); da {
			return plugin.AdminOut{Redirect: "?ansicht=formulare",
				Flash: "A form with this key already exists.", FlashError: true}, true, nil
		}
		f := formular{Key: key, Name: name}
		if err := save(f.clean()); err != nil {
			return plugin.AdminOut{}, true, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=formular&kennung=" + key,
			Flash: "Angelegt. Jetzt die Felder festlegen."}, true, nil

	case len(in.Form["loeschen_formular"]) > 0:
		key := in.Form["loeschen_formular"][0]
		if reKey.MatchString(key) {
			_ = plugin.Delete(praefixFormular + key)
		}
		// The messages that have already come in stay: they are the reason the
		// form existed, and disappearing with it would be the opposite of what
		// somebody expects when tidying up.
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "Form deleted. The messages that came in stay."}, true, nil

	case len(in.Form["feldaktion"]) > 0, len(in.Form["sichern"]) > 0:
		return storeForm(in, q)
	}
	return plugin.AdminOut{}, false, nil
}

// storeForm reads the editor, applies an action and stores.
//
// An action stores too: the editor is a form, and a button submits everything
// anyway. Storing nothing would mean that adding a field discards everything
// already typed beside it.
func storeForm(in plugin.AdminIn, q url.Values) (plugin.AdminOut, bool, error) {
	key := firstValue(in.Form, "kennung")
	f, ok := formularLaden(key)
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: "There is no such form.", FlashError: true}, true, nil
	}

	f.Name = firstValue(in.Form, "name")
	f.Subject = firstValue(in.Form, "betreff")
	f.Dank = firstValue(in.Form, "dank")
	f.Fields = fieldsFromForm(in.Form)

	if aktion := firstValue(in.Form, "feldaktion"); aktion != "" {
		f.Fields = fieldAction(f.Fields, aktion)
	}

	f = f.clean()
	if err := save(f); err != nil {
		return plugin.AdminOut{}, true, err
	}
	out := plugin.AdminOut{Redirect: "?ansicht=formular&kennung=" + f.Key}
	if firstValue(in.Form, "sichern") != "" {
		out.Flash = "Formular gespeichert."
	}
	return out, true, nil
}

// fieldsFromForm reads the field list. As in the block editor the names carry a
// number, and the numbers are handed out afresh on reading — so a removed field
// leaves no gap.
func fieldsFromForm(form map[string][]string) []field {
	slots := map[int]*field{}
	for key, values := range form {
		if !strings.HasPrefix(key, "fe") || len(values) == 0 {
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
			fe = &field{}
			slots[n] = fe
		}
		switch rest[punkt+1:] {
		case "kennung":
			fe.Key = values[0]
		case "beschriftung":
			fe.Label = values[0]
		case "art":
			fe.Art = values[0]
		case "hinweis":
			fe.Hint = values[0]
		case "pflicht":
			fe.Required = values[0] != ""
		case "auswahl":
			fe.Choices = rows(values[0])
		}
	}

	nummern := make([]int, 0, len(slots))
	for n := range slots {
		nummern = append(nummern, n)
	}
	sort.Ints(nummern)

	out := make([]field, 0, len(nummern))
	for _, n := range nummern {
		out = append(out, *slots[n])
	}
	return out
}

// fieldAction moves, removes or adds a field.
func fieldAction(fields []field, aktion string) []field {
	name, arg, _ := strings.Cut(aktion, ":")
	if name == "neu" {
		if len(fields) >= maxFields {
			return fields
		}
		return append(fields, field{Label: "Neue Frage", Art: ArtText})
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 || n >= len(fields) {
		return fields
	}
	switch name {
	case "weg":
		return append(fields[:n:n], fields[n+1:]...)
	case "hoch":
		if n > 0 {
			fields[n-1], fields[n] = fields[n], fields[n-1]
		}
	case "runter":
		if n < len(fields)-1 {
			fields[n], fields[n+1] = fields[n+1], fields[n]
		}
	}
	return fields
}

func rows(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, z := range strings.Split(s, "\n") {
		if z = strings.TrimSpace(z); z != "" {
			out = append(out, z)
		}
	}
	return out
}

func firstValue(form map[string][]string, name string) string {
	if v := form[name]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}
