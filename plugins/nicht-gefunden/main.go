// Das 404-Protokoll als Plugin.
//
// Es sammelt, welche Adressen Besucher anfragen und nicht bekommen, wie oft
// und woher sie kamen. Das ist die Liste, aus der man abliest, welche alte
// Adresse eine Weiterleitung braucht — und es ist genau die Sorte Funktion,
// die nicht im Kern stehen muss: wer sie nicht will, schaltet sie ab.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// eintrag ist eine angefragte Adresse.
type eintrag struct {
	Pfad     string `json:"pfad"`
	Anzahl   int    `json:"anzahl"`
	Zuletzt  string `json:"zuletzt"`
	Herkunft string `json:"herkunft,omitempty"`
}

// praefix trennt die Einträge von allem anderen im eigenen Speicher.
const praefix = "pfad:"

// maxEintraege begrenzt, wie viele Adressen behalten werden.
//
// Ein Scanner klopft an tausend Adressen, die es nie gab. Ohne Grenze wäre die
// Liste nach einer Woche unlesbar und der Speicher voll mit Müll, den niemand
// je ansehen will.
const maxEintraege = 500

func init() {
	plugin.OnEvent(func(in plugin.EventIn) error {
		if in.Name != plugin.EventNotFound {
			return nil
		}
		pfad := in.Data["path"]
		if pfad == "" || len(pfad) > 300 {
			return nil
		}

		key := praefix + pfad
		e := eintrag{Pfad: pfad}
		if roh, ok, _ := plugin.Get(key); ok {
			_ = json.Unmarshal([]byte(roh), &e)
		}
		e.Anzahl++
		e.Zuletzt = time.Now().UTC().Format(time.RFC3339)
		if h := in.Data["referer"]; h != "" && len(h) < 300 {
			e.Herkunft = h
		}

		roh, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err := plugin.Set(key, string(roh)); err != nil {
			return err
		}
		aufraeumen()
		return nil
	})

	plugin.OnAdmin(bildschirm)
}

// aufraeumen wirft die seltensten Einträge weg, wenn es zu viele werden.
//
// Nicht die ältesten: eine Adresse, die einmal im Monat angefragt wird, ist
// weniger wert als eine, die täglich kommt, auch wenn sie neuer ist.
func aufraeumen() {
	alle, err := plugin.List(praefix, 1000)
	if err != nil || len(alle) <= maxEintraege {
		return
	}
	liste := lies(alle)
	sort.Slice(liste, func(i, j int) bool { return liste[i].Anzahl < liste[j].Anzahl })
	for i := 0; i < len(liste)-maxEintraege; i++ {
		_ = plugin.Delete(praefix + liste[i].Pfad)
	}
	plugin.Logf("info", "%d selten angefragte Adressen entfernt", len(liste)-maxEintraege)
}

// lies wandelt die gespeicherten Zeilen in Einträge.
func lies(roh map[string]string) []eintrag {
	out := make([]eintrag, 0, len(roh))
	for _, v := range roh {
		var e eintrag
		if json.Unmarshal([]byte(v), &e) == nil && e.Pfad != "" {
			out = append(out, e)
		}
	}
	return out
}

func bildschirm(in plugin.AdminIn) (plugin.AdminOut, error) {
	if in.Method == "POST" {
		switch {
		case in.Form["alles_loeschen"] != nil:
			alle, _ := plugin.List(praefix, 1000)
			for k := range alle {
				_ = plugin.Delete(k)
			}
			return plugin.AdminOut{Redirect: ".", Flash: "Liste geleert."}, nil
		case len(in.Form["loeschen"]) > 0:
			_ = plugin.Delete(praefix + in.Form["loeschen"][0])
			return plugin.AdminOut{Redirect: ".", Flash: "Eintrag entfernt."}, nil
		}
	}

	alle, err := plugin.List(praefix, 1000)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	liste := lies(alle)
	// Häufigstes zuerst: das ist die Adresse, für die sich eine Weiterleitung
	// am ehesten lohnt.
	sort.Slice(liste, func(i, j int) bool {
		if liste[i].Anzahl != liste[j].Anzahl {
			return liste[i].Anzahl > liste[j].Anzahl
		}
		return liste[i].Pfad < liste[j].Pfad
	})

	var b strings.Builder
	b.WriteString(`<p>Adressen, die Besucher angefragt und nicht bekommen haben. ` +
		`Die häufigsten stehen oben — für die lohnt sich eine Weiterleitung am ehesten.</p>`)

	if len(liste) == 0 {
		b.WriteString(`<p class="empty">Noch nichts angefragt, was es nicht gibt.</p>`)
		return plugin.AdminOut{Title: "Nicht gefunden", HTML: b.String()}, nil
	}

	b.WriteString(`<table class="table"><thead><tr>` +
		`<th>Adresse</th><th>Anfragen</th><th>Zuletzt</th><th>Herkunft</th><th></th>` +
		`</tr></thead><tbody>`)
	for _, e := range liste {
		fmt.Fprintf(&b, `<tr><td><code>%s</code></td><td>%d</td><td>%s</td><td>%s</td>`,
			html.EscapeString(e.Pfad), e.Anzahl,
			html.EscapeString(kurzDatum(e.Zuletzt)), html.EscapeString(e.Herkunft))
		// Das Formular sendet an dieselbe Adresse zurück; den Sitzungsschlüssel
		// setzt der Host ein, das Plugin sieht ihn nie.
		fmt.Fprintf(&b, `<td><form method="POST"><input type="hidden" name="loeschen" value="%s">`+
			`<button type="submit" class="btn btn--sm">Entfernen</button></form></td></tr>`,
			html.EscapeString(e.Pfad))
	}
	b.WriteString(`</tbody></table>`)
	fmt.Fprintf(&b, `<p>%s Adressen insgesamt.</p>`, strconv.Itoa(len(liste)))
	b.WriteString(`<form method="POST"><input type="hidden" name="alles_loeschen" value="1">` +
		`<button type="submit" class="btn btn--sm btn--danger">Liste leeren</button></form>`)

	return plugin.AdminOut{Title: "Nicht gefunden", HTML: b.String()}, nil
}

// kurzDatum macht aus dem gespeicherten Zeitstempel etwas Lesbares.
func kurzDatum(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006 15:04")
}

func main() {}
