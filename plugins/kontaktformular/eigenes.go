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
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, fieldForm, e(f.Key))

	if d.Hint != "" {
		class := "contact-form__notice"
		if d.IsError {
			class += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, class, e(d.Hint))
	}

	for _, fe := range f.Fields {
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
				b.WriteString(`<option value="">– please choose –</option>`)
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
		`<label for="cf-website-%s">Website (bitte leer lassen)</label>`+
		`<input type="text" id="cf-website-%s" name="%s" tabindex="-1" autocomplete="off"></div>`,
		e(f.Key), e(f.Key), fieldHoneypot)

	b.WriteString(`<button type="submit" class="contact-form__submit">Absenden</button>`)
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">Lieber direkt schreiben? `+
			`<a href="mailto:%s">%s</a></p>`, e(d.Kontakt), e(d.Kontakt))
	}
	b.WriteString(`</form>`)
	return b.String()
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
func empfangenEigen(f formular, form url.Values, page string) (message, string) {
	n := message{Page: page, Form: f.Key, FormName: f.Name}

	for _, fe := range f.Fields {
		raw := strings.TrimSpace(form.Get(fieldPrefix + fe.Key))
		if fe.Art == ArtAnkreuz {
			if raw != "" {
				raw = "ja"
			} else if fe.Required {
				return n, "Bitte kreuze „" + fe.Label + "“ an."
			} else {
				raw = "nein"
			}
		}
		if raw == "" {
			if fe.Required {
				return n, "Please fill in “" + fe.Label + "“ aus."
			}
			continue
		}
		if problem := checkField(fe, raw); problem != "" {
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
		return n, "Please fill in the form."
	}
	n.Subject = f.Subject
	if n.Subject == "" {
		n.Subject = f.Name
	}
	if n.Name == "" {
		n.Name = "Ohne Namen"
	}
	n.Text = asText(n.Fields)
	return n, ""
}

// checkField says what is wrong with an answer.
func checkField(fe field, value string) string {
	switch {
	case len([]rune(value)) > maxText:
		return "„" + fe.Label + "“ ist zu lang."
	case fe.Art == ArtEmail && !plausibleAddress(value):
		return "Die Adresse in „" + fe.Label + "” does not look right."
	case fe.Art == ArtZahl && !istZahl(value):
		return "„" + fe.Label + "” has to be a number."
	case fe.Art == ArtDatum && !istDatum(value):
		return "„" + fe.Label + "“ muss ein Datum sein."
	case fe.Art == KindChoice && !enthaelt(fe.Choices, value):
		// The browser allows only the offered values; whoever sends something
		// else did not use the form but rebuilt it.
		return "Please choose one of the offered values for “" + fe.Label + "”."
	}
	return ""
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
	return "Danke, die Nachricht ist angekommen. Wir melden uns."
}

var _ = plugin.Log
