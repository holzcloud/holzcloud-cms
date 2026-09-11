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

// feldPraefix trennt die Antworten eines eigenen Formulars von den Feldern, die
// das Plugin selbst braucht — der Zeitmarke, dem Honigtopf, dem Seitennamen.
// Ohne ihn könnte ein Feld namens "gestellt" die Zeitfalle aushebeln.
const feldPraefix = "f_"

// zeichnenEigen baut ein zusammengestelltes Formular.
func zeichnenEigen(f formular, d daten, werte url.Values) string {
	e := html.EscapeString
	var b strings.Builder

	fmt.Fprintf(&b, `<form class="contact-form contact-form--%s" method="POST" action="%s">`,
		e(f.Key), absendeAdresse)
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, feldZeit, e(d.Timestamp))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, feldSeite, e(d.Page))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, feldFormular, e(f.Key))

	if d.Hint != "" {
		klasse := "contact-form__notice"
		if d.IsError {
			klasse += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, klasse, e(d.Hint))
	}

	for _, fe := range f.Fields {
		id := "cf-" + f.Key + "-" + fe.Key
		name := feldPraefix + fe.Key
		wert := werte.Get(name)
		pflicht := ""
		if fe.Required {
			pflicht = " required"
		}

		b.WriteString(`<div class="contact-form__field">`)
		if fe.Art != ArtAnkreuz {
			fmt.Fprintf(&b, `<label for="%s">%s</label>`, e(id), e(fe.Label))
		}

		switch fe.Art {
		case ArtLang:
			fmt.Fprintf(&b, `<textarea id="%s" name="%s" rows="6" maxlength="%d"%s>%s</textarea>`,
				e(id), e(name), maxText, pflicht, e(wert))
		case KindChoice:
			fmt.Fprintf(&b, `<select id="%s" name="%s"%s>`, e(id), e(name), pflicht)
			if !fe.Required {
				b.WriteString(`<option value="">– bitte wählen –</option>`)
			}
			for _, w := range fe.Choices {
				aus := ""
				if w == wert {
					aus = " selected"
				}
				fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, e(w), aus, e(w))
			}
			b.WriteString(`</select>`)
		case ArtAnkreuz:
			an := ""
			if wert != "" {
				an = " checked"
			}
			fmt.Fprintf(&b, `<label for="%s" class="contact-form__check">`+
				`<input type="checkbox" id="%s" name="%s" value="ja"%s%s> %s</label>`,
				e(id), e(id), e(name), an, pflicht, e(fe.Label))
		default:
			fmt.Fprintf(&b, `<input type="%s" id="%s" name="%s" value="%s" maxlength="%d"%s>`,
				eingabeArt(fe.Art), e(id), e(name), e(wert), maxName, pflicht)
		}

		if fe.Hint != "" {
			fmt.Fprintf(&b, `<small class="contact-form__hint">%s</small>`, e(fe.Hint))
		}
		b.WriteString(`</div>`)
	}

	// Der Honigtopf gehört zu jedem Formular, auch zu einem selbst gebauten:
	// ein Roboter unterscheidet sie nicht.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="cf-website-%s">Website (bitte leer lassen)</label>`+
		`<input type="text" id="cf-website-%s" name="%s" tabindex="-1" autocomplete="off"></div>`,
		e(f.Key), e(f.Key), feldHonigtopf)

	b.WriteString(`<button type="submit" class="contact-form__submit">Absenden</button>`)
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">Lieber direkt schreiben? `+
			`<a href="mailto:%s">%s</a></p>`, e(d.Kontakt), e(d.Kontakt))
	}
	b.WriteString(`</form>`)
	return b.String()
}

// eingabeArt bildet eine Feldart auf den type eines <input> ab.
//
// Der richtige type ist auf einem Telefon der Unterschied zwischen der
// Zifferntastatur und der Buchstabentastatur — und damit zwischen einer
// ausgefüllten und einer abgebrochenen Anfrage.
func eingabeArt(art string) string {
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

// antwort ist eine ausgefüllte Zeile, so wie sie in der Verwaltung steht.
type antwort struct {
	Label string `json:"beschriftung"`
	Value string `json:"wert"`
}

// empfangenEigen nimmt die Absendung eines zusammengestellten Formulars an.
func empfangenEigen(f formular, form url.Values, seite string) (nachricht, string) {
	n := nachricht{Page: seite, Formular: f.Key, FormularName: f.Name}

	for _, fe := range f.Fields {
		roh := strings.TrimSpace(form.Get(feldPraefix + fe.Key))
		if fe.Art == ArtAnkreuz {
			if roh != "" {
				roh = "ja"
			} else if fe.Required {
				return n, "Bitte kreuze „" + fe.Label + "“ an."
			} else {
				roh = "nein"
			}
		}
		if roh == "" {
			if fe.Required {
				return n, "Bitte fülle „" + fe.Label + "“ aus."
			}
			continue
		}
		if problem := feldPruefen(fe, roh); problem != "" {
			return n, problem
		}
		n.Fields = append(n.Fields, antwort{Label: fe.Label, Value: roh})

		// Woran die Benachrichtigung hängt: die erste E-Mail-Adresse ist die
		// des Absenders, die erste kurze Antwort sein Name. So braucht es dafür
		// keine zusätzliche Einstellung, die jemand vergessen könnte.
		if fe.Art == ArtEmail && n.Email == "" {
			n.Email = roh
		}
		if fe.Art == ArtText && n.Name == "" {
			n.Name = roh
		}
	}

	if len(n.Fields) == 0 {
		return n, "Bitte fülle das Formular aus."
	}
	n.Betreff = f.Betreff
	if n.Betreff == "" {
		n.Betreff = f.Name
	}
	if n.Name == "" {
		n.Name = "Ohne Namen"
	}
	n.Text = alsText(n.Fields)
	return n, ""
}

// feldPruefen sagt, was an einer Antwort nicht stimmt.
func feldPruefen(fe feld, wert string) string {
	switch {
	case len([]rune(wert)) > maxText:
		return "„" + fe.Label + "“ ist zu lang."
	case fe.Art == ArtEmail && !plausibleAdresse(wert):
		return "Die Adresse in „" + fe.Label + "“ sieht nicht richtig aus."
	case fe.Art == ArtZahl && !istZahl(wert):
		return "„" + fe.Label + "“ muss eine Zahl sein."
	case fe.Art == ArtDatum && !istDatum(wert):
		return "„" + fe.Label + "“ muss ein Datum sein."
	case fe.Art == KindChoice && !enthaelt(fe.Choices, wert):
		// Der Browser lässt nur die angebotenen Werte zu; wer etwas anderes
		// schickt, hat das Formular nicht benutzt, sondern nachgebaut.
		return "Bitte wähle bei „" + fe.Label + "“ einen der angebotenen Werte."
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

func enthaelt(liste []string, wert string) bool {
	for _, w := range liste {
		if w == wert {
			return true
		}
	}
	return false
}

// alsText macht aus den Antworten den Fliesstext, der in der Benachrichtigung
// steht. Dieselben Worte wie auf dem Bildschirm, damit wer die Mail liest und
// wer in die Verwaltung schaut, dasselbe sieht.
func alsText(felder []antwort) string {
	var b strings.Builder
	for _, a := range felder {
		fmt.Fprintf(&b, "%s: %s\n", a.Label, a.Value)
	}
	return strings.TrimRight(b.String(), "\n")
}

// markeFuer erzeugt die Marke, die ein Redakteur in eine Seite schreibt.
func markeFuer(kennung string) string { return "[[formular:" + kennung + "]]" }

// hinweisFuer ist der Satz nach dem Absenden.
func hinweisFuer(f formular, gefunden bool) string {
	if gefunden && f.Dank != "" {
		return f.Dank
	}
	return "Danke, die Nachricht ist angekommen. Wir melden uns."
}

var _ = plugin.Log
