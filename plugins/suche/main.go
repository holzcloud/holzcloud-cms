// Die Volltextsuche als Plugin.
//
// Sie beantwortet /suche selbst und lässt das Ergebnis vom Host in der Ansicht
// des Themes ausgeben. Das Plugin sieht das Theme nie — es liefert eine Liste
// von Treffern, und Kopf, Menü, Schriften und Fuss kommen von der Website.
//
// Warum das kein Kern ist: eine Website mit acht Seiten braucht keine Suche,
// und wer sie nicht anbietet, hat auch keine Seite, auf der jemand nach etwas
// sucht, das es nicht gibt. Wer sie will, schaltet sie ein.
package main

import (
	"net/url"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// maxTreffer begrenzt eine Ergebnisliste.
//
// Zwanzig ist mehr, als jemand liest. Wer auf Seite drei blättert, sucht in
// Wahrheit etwas anderes — dafür hilft ein besseres Suchwort und keine längere
// Liste.
const maxTreffer = 20

// maxAnfrage begrenzt, was als Suchwort angenommen wird. Alles darüber ist
// kein Suchwort mehr, sondern etwas, das jemand ausprobiert.
const maxAnfrage = 200

func init() {
	plugin.OnRoute(suchen)
}

func suchen(in plugin.RequestIn) (plugin.RequestOut, error) {
	frage := strings.TrimSpace(anfrage(in.Query))
	if len(frage) > maxAnfrage {
		frage = frage[:maxAnfrage]
	}

	liste := plugin.RenderSearch{Query: frage, Submitted: frage != ""}
	if frage != "" {
		treffer, err := plugin.SearchPages(frage, maxTreffer)
		if err != nil {
			return plugin.RequestOut{}, err
		}
		for _, t := range treffer {
			liste.Results = append(liste.Results, plugin.RenderHit{
				Title:   t.Title,
				URL:     "/" + t.Slug,
				Snippet: t.Snippet,
			})
		}
	}

	titel := "Suche"
	if frage != "" {
		titel = "Suche: " + frage
	}

	html, err := plugin.Render(plugin.RenderArg{
		Title: titel,
		Slug:  "suche",
		View:  plugin.ViewSearch,
		// Eine Trefferliste ist kein eigener Inhalt und darf den Seiten, auf
		// die sie zeigt, keine Konkurrenz in der Suchmaschine machen.
		NoIndex: true,
		Search:  &liste,
	})
	if err != nil {
		return plugin.RequestOut{}, err
	}

	return plugin.RequestOut{
		Handled: true,
		Status:  200,
		Body:    html,
		Headers: map[string]string{
			// Das Ergebnis hängt an der Frage und an Inhalten, die sich
			// jederzeit ändern können. Da ist nichts, was sich zu behalten
			// lohnt.
			"Cache-Control": "no-store",
			"X-Robots-Tag":  "noindex",
		},
	}, nil
}

// anfrage holt q aus der Abfragezeichenkette.
func anfrage(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	return values.Get("q")
}

func main() {}
