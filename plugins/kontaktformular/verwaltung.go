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
		art(ansichtNachrichten, plugin.T("Messages"), "") +
		art(ansichtFormulare, plugin.T("Forms"), ansichtFormulare) +
		`</p>`
}

// --- Liste ------------------------------------------------------------------

func formularliste() (plugin.AdminOut, error) {
	var b strings.Builder
	b.WriteString(navigation(ansichtFormulare))
	fmt.Fprintf(&b, `<p>%s</p>`, plugin.T("A form of your own asks exactly what you want to know. "+
		"Put it into a page with its marker, the way you would a snippet."))

	liste := allForms()
	if len(liste) == 0 {
		fmt.Fprintf(&b, `<p class="empty">%s</p>`, plugin.T("No form of your own yet. "+
			"The built-in contact form is still available under <code>[[formular]]</code>."))
	} else {
		b.WriteString(`<table class="table"><thead><tr>` +
			fmt.Sprintf(`<th>%s</th><th>%s</th><th>%s</th><th></th>`,
				plugin.T("Form"), plugin.T("Fields"), plugin.T("Marker for the page")) +
			`</tr></thead><tbody>`)
		for _, f := range liste {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td><code>%s</code></td><td class="table-actions">`,
				html.EscapeString(f.Name), len(f.Fields), html.EscapeString(markerFor(f.Key)))
			fmt.Fprintf(&b, `<a class="btn btn--sm" href="?ansicht=formular&amp;kennung=%s">%s</a>`,
				html.EscapeString(f.Key), html.EscapeString(plugin.T("Edit")))
			fmt.Fprintf(&b, `<form method="POST" class="inline-form">`+
				`<input type="hidden" name="loeschen_formular" value="%s">`+
				`<button type="submit" class="btn btn--sm btn--danger">%s</button></form>`,
				html.EscapeString(f.Key), html.EscapeString(plugin.T("Delete")))
			b.WriteString(`</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}

	e := html.EscapeString
	fmt.Fprintf(&b, `<form method="POST" class="stack">`+
		`<fieldset><legend>%s</legend>`+
		`<label for="neu-name">%s</label>`+
		`<input type="text" id="neu-name" name="neues_formular" required maxlength="80" `+
		`placeholder="%s">`+
		`<button type="submit" class="btn btn--primary">%s</button>`+
		`</fieldset></form>`,
		e(plugin.T("New form")), e(plugin.T("Name")),
		e(plugin.T("Signing up for the farm festival")), e(plugin.T("Create")))

	return plugin.AdminOut{Title: plugin.T("Forms"), HTML: b.String()}, nil
}

// --- Editor -----------------------------------------------------------------

func formulareditor(in plugin.AdminIn, key string) (plugin.AdminOut, error) {
	f, ok := formularLaden(key)
	if !ok {
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: plugin.T("There is no such form."), FlashError: true}, nil
	}

	e := html.EscapeString
	var b strings.Builder
	b.WriteString(navigation(ansichtFormulare))
	fmt.Fprintf(&b, `<p>%s</p>`, plugin.Tf("Place it in the page with <code>%s</code>.", e(markerFor(f.Key))))

	b.WriteString(`<form method="POST" class="stack">`)
	fmt.Fprintf(&b, `<input type="hidden" name="kennung" value="%s">`, e(f.Key))

	fmt.Fprintf(&b, `<fieldset><legend>%s</legend>`, e(plugin.T("Form")))
	fmt.Fprintf(&b, `<label for="f-name">%s</label>`+
		`<input type="text" id="f-name" name="name" value="%s" maxlength="80" required>`,
		e(plugin.T("Name")), e(f.Name))
	fmt.Fprintf(&b, `<label for="f-betreff">%s</label>`+
		`<input type="text" id="f-betreff" name="betreff" value="%s" maxlength="120" `+
		`placeholder="%s">`,
		e(plugin.T("Subject of the notification")), e(f.Subject),
		e(plugin.T("Otherwise the name of the form")))
	fmt.Fprintf(&b, `<label for="f-dank">%s</label>`+
		`<input type="text" id="f-dank" name="dank" value="%s" maxlength="200" `+
		`placeholder="%s">`,
		e(plugin.T("Sentence after submitting")), e(f.Dank),
		e(plugin.T("Thank you, the message has arrived. We will be in touch.")))
	b.WriteString(`</fieldset>`)

	for i, fe := range f.Fields {
		p := "fe" + strconv.Itoa(i)
		fmt.Fprintf(&b, `<fieldset><legend>%d. %s</legend>`, i+1, e(fe.Label))
		fmt.Fprintf(&b, `<input type="hidden" name="%s.kennung" value="%s">`, p, e(fe.Key))

		fmt.Fprintf(&b, `<label for="%s-b">%s</label>`+
			`<input type="text" id="%s-b" name="%s.beschriftung" value="%s" maxlength="120" required>`,
			p, e(plugin.T("Question")), p, p, e(fe.Label))

		fmt.Fprintf(&b, `<label for="%s-a">%s</label><select id="%s-a" name="%s.art">`,
			p, e(plugin.T("Kind")), p, p)
		for _, a := range fieldKinds {
			aus := ""
			if a.Art == fe.Art {
				aus = " selected"
			}
			fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, a.Art, aus, e(plugin.T(a.Name)))
		}
		b.WriteString(`</select>`)

		fmt.Fprintf(&b, `<label for="%s-w">%s</label>`+
			`<textarea id="%s-w" name="%s.auswahl" rows="3">%s</textarea>`,
			p, e(plugin.T("Options (one per line)")), p, p, e(strings.Join(fe.Choices, "\n")))

		fmt.Fprintf(&b, `<label for="%s-h">%s</label>`+
			`<input type="text" id="%s-h" name="%s.hinweis" value="%s" maxlength="200">`,
			p, e(plugin.T("Hint under the field")), p, p, e(fe.Hint))

		an := ""
		if fe.Required {
			an = " checked"
		}
		fmt.Fprintf(&b, `<label><input type="checkbox" name="%s.pflicht" value="1"%s> `+
			`%s</label>`, p, an, e(plugin.T("Has to be filled in")))

		b.WriteString(`<p class="table-actions">`)
		actionButton(&b, "feldaktion", "hoch:"+strconv.Itoa(i), plugin.T("↑ up"), "")
		actionButton(&b, "feldaktion", "runter:"+strconv.Itoa(i), plugin.T("↓ down"), "")
		actionButton(&b, "feldaktion", "weg:"+strconv.Itoa(i), plugin.T("Remove field"), "btn--danger")
		b.WriteString(`</p></fieldset>`)
	}

	if len(f.Fields) == 0 {
		fmt.Fprintf(&b, `<p class="empty">%s</p>`, e(plugin.T("No field yet. A form with no fields is not shown.")))
	}

	b.WriteString(`<p class="table-actions">`)
	actionButton(&b, "feldaktion", "neu", plugin.T("Add field"), "")
	b.WriteString(`</p>`)
	fmt.Fprintf(&b, `<p><button type="submit" name="sichern" value="1" class="btn btn--primary">`+
		`%s</button></p>`, e(plugin.T("Save form")))
	b.WriteString(`</form>`)

	if _, hat := f.ersteArt(ArtEmail); !hat && len(f.Fields) > 0 {
		fmt.Fprintf(&b, `<p class="text-muted">%s</p>`, e(plugin.T("No field for an e-mail address: "+
			"an enquiry through this form then cannot be answered by mail.")))
	}

	return plugin.AdminOut{Title: plugin.Tf("Form: %s", f.Name), HTML: b.String()}, nil
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
				Flash:      plugin.T("No key can be made from this name. Please use letters."),
				FlashError: true}, true, nil
		}
		if _, da := formularLaden(key); da {
			return plugin.AdminOut{Redirect: "?ansicht=formulare",
				Flash: plugin.T("A form with this key already exists."), FlashError: true}, true, nil
		}
		f := formular{Key: key, Name: name}
		if err := save(f.clean()); err != nil {
			return plugin.AdminOut{}, true, err
		}
		return plugin.AdminOut{Redirect: "?ansicht=formular&kennung=" + key,
			Flash: plugin.T("Created. Now set the fields.")}, true, nil

	case len(in.Form["loeschen_formular"]) > 0:
		key := in.Form["loeschen_formular"][0]
		if reKey.MatchString(key) {
			_ = plugin.Delete(praefixFormular + key)
		}
		// The messages that have already come in stay: they are the reason the
		// form existed, and disappearing with it would be the opposite of what
		// somebody expects when tidying up.
		return plugin.AdminOut{Redirect: "?ansicht=formulare",
			Flash: plugin.T("Form deleted. The messages that came in stay.")}, true, nil

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
			Flash: plugin.T("There is no such form."), FlashError: true}, true, nil
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
		out.Flash = plugin.T("Form saved.")
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
		return append(fields, field{Label: plugin.T("New question"), Art: ArtText})
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
