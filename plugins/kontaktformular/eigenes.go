package main

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// fieldPrefix separates an own form's answers from the fields the plugin itself
// needs — the timestamp, the honeypot, the page name. Without it a field called
// "gestellt" could defeat the time trap.
const fieldPrefix = "f_"

// zeichnenEigen baut ein zusammengestelltes Formular.
func zeichnenEigen(f formular, d data, values url.Values) string {
	e := html.EscapeString
	var b strings.Builder

	fmt.Fprintf(&b, `<form class="contact-form contact-form--%s" method="POST" action="%s">`,
		e(f.Key), submitAddress)
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldTime, e(d.Timestamp))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldPage, e(d.Page))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldPath, e(d.Path))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldLang, e(d.Lang))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldForm, e(f.Key))

	if d.Hint != "" {
		class := "contact-form__notice"
		if d.IsError {
			class += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, class, e(d.Hint))
	}

	// What has been answered so far, by key, so a condition can be read.
	answered := map[string]string{}
	for _, fe := range f.Fields {
		answered[fe.Key] = strings.TrimSpace(values.Get(fieldPrefix + fe.Key))
	}
	// A field whose condition is not met is not drawn — not hidden. A hidden
	// field is still in the document, still submitted, and still read out by a
	// screen reader that ignores the stylesheet; and hiding is what needs
	// JavaScript to undo.
	var shown []field
	for _, fe := range f.Fields {
		if fe.asked(answered) {
			shown = append(shown, fe)
		}
	}

	for _, fe := range shown {
		id := "cf-" + f.Key + "-" + fe.Key
		name := fieldPrefix + fe.Key
		value := values.Get(name)
		required := ""
		if fe.Required {
			required = " required"
		}

		b.WriteString(`<div class="contact-form__field">`)
		if fe.Art != ArtAnkreuz {
			fmt.Fprintf(&b, `<label for="%s">%s</label>`, e(id), e(fe.Label))
		}

		switch fe.Art {
		case ArtLang:
			fmt.Fprintf(&b, `<textarea id="%s" name="%s" rows="6" maxlength="%d"%s>%s</textarea>`,
				e(id), e(name), maxText, required, e(value))
		case KindChoice:
			fmt.Fprintf(&b, `<select id="%s" name="%s"%s>`, e(id), e(name), required)
			if !fe.Required {
				fmt.Fprintf(&b, `<option value="">%s</option>`, e(plugin.T("– please choose –")))
			}
			for _, w := range fe.Choices {
				aus := ""
				if w == value {
					aus = " selected"
				}
				fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, e(w), aus, e(w))
			}
			b.WriteString(`</select>`)
		case ArtAnkreuz:
			an := ""
			if value != "" {
				an = " checked"
			}
			fmt.Fprintf(&b, `<label for="%s" class="contact-form__check">`+
				`<input type="checkbox" id="%s" name="%s" value="ja"%s%s> %s</label>`,
				e(id), e(id), e(name), an, required, e(fe.Label))
		default:
			fmt.Fprintf(&b, `<input type="%s" id="%s" name="%s" value="%s" maxlength="%d"%s>`,
				inputKind(fe.Art), e(id), e(name), e(value), maxName, required)
		}

		if fe.Hint != "" {
			fmt.Fprintf(&b, `<small class="contact-form__hint">%s</small>`, e(fe.Hint))
		}
		b.WriteString(`</div>`)
	}

	// The honeypot belongs to every form, an own-built one included: a robot
	// does not tell them apart.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="cf-website-%s">%s</label>`+
		`<input type="text" id="cf-website-%s" name="%s" tabindex="-1" autocomplete="off"></div>`,
		e(f.Key), e(plugin.T("Website (please leave empty)")), e(f.Key), fieldHoneypot)

	// Two buttons and no JavaScript. formmethod and formaction are ordinary
	// HTML: Continue submits the same form as a GET to the page it stands on,
	// so every answer so far lands in the query string, the page is drawn
	// again, and the fields whose condition is now met appear. Send posts it
	// for real.
	//
	// Not two steps with the first one locked: every field stays visible and
	// changeable, so somebody who picked the wrong answer can pick another and
	// press Continue again. A wizard that will not let you back is worse than
	// no wizard.
	if f.HasSteps() {
		// The page's own address, as the host gave it. Not "/"+Page: the start
		// page's slug is "home" and the start page is served at the root, so
		// that guess sent every visitor of a two-step form on the start page
		// through a redirect that dropped their answers on the way.
		ziel := d.Path
		if ziel == "" {
			ziel = "/"
		}
		fmt.Fprintf(&b, `<button type="submit" class="contact-form__more" `+
			`formmethod="get" formaction="%s">%s</button> `,
			e(ziel), e(plugin.T("Continue")))
	}
	fmt.Fprintf(&b, `<button type="submit" class="contact-form__submit">%s</button>`,
		e(plugin.T("Send")))
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">%s `+
			`<a href="mailto:%s">%s</a></p>`,
			e(plugin.T("Would you rather write directly?")), e(d.Kontakt), e(d.Kontakt))
	}
	b.WriteString(`</form>`)
	return b.String()
}

// answersFromQuery reads back what the Continue button put into the address.
//
// Only for the form the Continue belonged to — the hidden field says which —
// and only for fields this form has. A form with no second step never asks:
// then nothing this program drew put those values there, and a link somebody
// else composed could pre-fill a stranger's form with a stranger's words.
//
// Everything is bounded here and escaped where it is written. What it does mean
// is that a two-step form's first answers stand in the server log, the way the
// query string of every GET does. That is the price of a wizard without
// JavaScript, and it is why the split is at the first conditional field: a form
// that asks something which should not be logged should ask it after Continue.
func answersFromQuery(f formular, query url.Values) url.Values {
	if !f.HasSteps() || query.Get(fieldForm) != f.Key {
		return nil
	}
	out := url.Values{}
	for _, fe := range f.Fields {
		name := fieldPrefix + fe.Key
		v := strings.TrimSpace(query.Get(name))
		if v == "" {
			continue
		}
		if r := []rune(v); len(r) > maxText {
			v = string(r[:maxText])
		}
		out.Set(name, v)
	}
	return out
}

// inputKind maps a field kind onto an <input>'s type.
//
// The right type is, on a phone, the difference between the number keypad and
// the letter keyboard — and therefore between a completed and an abandoned
// enquiry.
func inputKind(art string) string {
	switch art {
	case ArtEmail:
		return "email"
	case ArtTelefon:
		return "tel"
	case ArtZahl:
		return "number"
	case ArtDatum:
		return "date"
	}
	return "text"
}

// answer is one filled-in row, as it stands in the admin.
type answer struct {
	Label string `json:"beschriftung"`
	Value string `json:"wert"`
}

// empfangenEigen nimmt die Absendung eines zusammengestellten Formulars an.
func empfangenEigen(f formular, form url.Values, page string) (message, refusal) {
	n := message{Page: page, Form: f.Key, FormName: f.Name}

	// The same reading of the conditions the drawing made. A field that was
	// not asked is not missing: it was never put to this person, and refusing
	// the submission because it is empty would be refusing them for an answer
	// nobody wanted.
	answered := map[string]string{}
	for _, fe := range f.Fields {
		answered[fe.Key] = strings.TrimSpace(form.Get(fieldPrefix + fe.Key))
	}

	for _, fe := range f.Fields {
		if !fe.asked(answered) {
			continue
		}
		raw := strings.TrimSpace(form.Get(fieldPrefix + fe.Key))
		if fe.Art == ArtAnkreuz {
			if raw != "" {
				raw = "ja"
			} else if fe.Required {
				return n, refusal{Field: fieldPrefix + fe.Key, Code: "field-tick", Arg: fe.Label}
			} else {
				raw = "nein"
			}
		}
		if raw == "" {
			if fe.Required {
				return n, refusal{Field: fieldPrefix + fe.Key, Code: "field-fill", Arg: fe.Label}
			}
			continue
		}
		if problem := checkField(fe, raw); !problem.ok() {
			return n, problem
		}
		n.Fields = append(n.Fields, answer{Label: fe.Label, Value: raw})

		// What the notification hangs off: the first e-mail address is the
		// sender's, the first short answer their name. That way it needs no
		// extra setting somebody could forget.
		if fe.Art == ArtEmail && n.Email == "" {
			n.Email = raw
		}
		if fe.Art == ArtText && n.Name == "" {
			n.Name = raw
		}
	}

	if len(n.Fields) == 0 {
		return n, refusal{Code: "form-incomplete"}
	}
	n.Subject = f.Subject
	if n.Subject == "" {
		n.Subject = f.Name
	}
	if n.Name == "" {
		n.Name = plugin.T("Without a name")
	}
	n.Text = asText(n.Fields)
	return n, refusal{}
}

// checkField says what is wrong with an answer.
//
// The label travels as %s and is never translated: it is the operator's own
// word, and a form that asks for "Lieblingsfarbe" must say "Lieblingsfarbe"
// back. The sentence around it is a whole format string in the catalogue — not
// pieces joined with +, which is the shape the collector cannot see and which
// left these five refusals half German and half English, quotation marks and
// all, until PUB-01.
func checkField(fe field, value string) refusal {
	bad := func(code string) refusal {
		return refusal{Field: fieldPrefix + fe.Key, Code: code, Arg: fe.Label}
	}
	switch {
	case len([]rune(value)) > maxText:
		return bad("field-long")
	case fe.Art == ArtEmail && !plausibleAddress(value):
		return bad("field-email")
	case fe.Art == ArtZahl && !istZahl(value):
		return bad("field-digit")
	case fe.Art == ArtDatum && !istDatum(value):
		return bad("field-date")
	case fe.Art == KindChoice && !enthaelt(fe.Choices, value):
		// The browser allows only the offered values; whoever sends something
		// else did not use the form but rebuilt it.
		return bad("field-pick")
	}
	return refusal{}
}

func istZahl(s string) bool {
	_, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return err == nil
}

func istDatum(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func enthaelt(liste []string, value string) bool {
	for _, w := range liste {
		if w == value {
			return true
		}
	}
	return false
}

// asText turns the answers into the flowing text that stands in the
// notification. The same words as on the screen, so that whoever reads the mail
// and whoever looks in the admin see the same thing.
func asText(fields []answer) string {
	var b strings.Builder
	for _, a := range fields {
		fmt.Fprintf(&b, "%s: %s\n", a.Label, a.Value)
	}
	return strings.TrimRight(b.String(), "\n")
}

// markerFor produces the marker an editor writes into a page.
func markerFor(key string) string { return "[[formular:" + key + "]]" }

// hintFor is the sentence after submitting.
func hintFor(f formular, gefunden bool) string {
	if gefunden && f.Dank != "" {
		return f.Dank
	}
	return plugin.T("Thank you, the message has arrived. We will be in touch.")
}
