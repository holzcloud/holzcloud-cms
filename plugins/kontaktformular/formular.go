package main

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Eigene Formulare.
//
// Das Kontaktformular fragt Name, E-Mail, Betreff und Nachricht. Für eine
// Anfrage nach Wolle ist das die Hälfte von dem, was man wissen will — wie viel,
// welche Farbe, bis wann — und für eine Anmeldung zum Hoffest ist es das
// Falsche. Also kann ein Betreiber eigene Formulare zusammenstellen, jedes mit
// seinen eigenen Feldern, und sie mit derselben Marke in eine Seite setzen.
//
// Das eingebaute Formular bleibt, wie es war. Wer nichts definiert, merkt von
// alldem nichts.

const praefixFormular = "formular:"

// maxFelder begrenzt ein Formular.
//
// Zwanzig Felder sind schon eines, das niemand ausfüllt. Die Grenze ist nicht
// gegen Missbrauch — es ist der eigene Betreiber, der hier tippt —, sondern
// gegen ein Formular, das die Anfragen verhindert, für die es da ist.
const maxFelder = 20

// Feldarten. Wenige, weil jede eine ist, die der Betreiber verstehen und das
// Theme gestalten muss.
const (
	ArtText    = "text"
	ArtLang    = "lang"
	ArtEmail   = "email"
	ArtTelefon = "telefon"
	ArtZahl    = "zahl"
	ArtDatum   = "datum"
	ArtAuswahl = "auswahl"
	ArtAnkreuz = "ankreuz"
)

// feldArt beschreibt eine Art für das Auswahlfeld in der Verwaltung.
type feldArt struct {
	Art  string
	Name string
}

var feldArten = []feldArt{
	{ArtText, "Kurze Antwort"},
	{ArtLang, "Lange Antwort"},
	{ArtEmail, "E-Mail-Adresse"},
	{ArtTelefon, "Telefonnummer"},
	{ArtZahl, "Zahl"},
	{ArtDatum, "Datum"},
	{ArtAuswahl, "Auswahl aus einer Liste"},
	{ArtAnkreuz, "Ankreuzfeld"},
}

func artName(art string) string {
	for _, a := range feldArten {
		if a.Art == art {
			return a.Name
		}
	}
	return art
}

// feld ist eine Frage.
type feld struct {
	// Kennung ist der Feldname im abgesendeten Formular. Er wird aus der
	// Beschriftung erzeugt und bleibt danach stehen: würde er sich mit der
	// Beschriftung ändern, käme nach jeder Umformulierung eine Antwort unter
	// einem anderen Namen an, und die alten wären nicht mehr zuzuordnen.
	Kennung      string   `json:"kennung"`
	Beschriftung string   `json:"beschriftung"`
	Art          string   `json:"art"`
	Pflicht      bool     `json:"pflicht,omitempty"`
	Hinweis      string   `json:"hinweis,omitempty"`
	Auswahl      []string `json:"auswahl,omitempty"`
}

// formular ist ein zusammengestelltes Formular.
type formular struct {
	Kennung string `json:"kennung"`
	Name    string `json:"name"`
	// Betreff steht in der Benachrichtigung, wenn kein Feld einen liefert.
	Betreff string `json:"betreff,omitempty"`
	// Dank ist der Satz nach dem Absenden.
	Dank   string `json:"dank,omitempty"`
	Felder []feld `json:"felder,omitempty"`
}

// reKennung ist so eng, weil eine Kennung in eine Marke im Seitentext kommt und
// in einen Feldnamen: alles, was dort maskiert werden müsste, wird irgendwann
// falsch maskiert.
var reKennung = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// kennungAus macht aus einer Beschriftung eine Kennung.
func kennungAus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	ersatz := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")
	s = ersatz.Replace(s)

	var b strings.Builder
	strich := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			strich = false
		case !strich && b.Len() > 0:
			b.WriteByte('-')
			strich = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" || !reKennung.MatchString(out) {
		return ""
	}
	return out
}

// antwortfeld sucht das erste Feld einer Art. So wird ohne eine zusätzliche
// Einstellung klar, wohin eine Antwort geht: die erste E-Mail-Adresse im
// Formular ist die des Absenders.
func (f formular) ersteArt(art string) (feld, bool) {
	for _, fe := range f.Felder {
		if fe.Art == art {
			return fe, true
		}
	}
	return feld{}, false
}

// laden holt ein Formular. Nicht gefunden ist kein Fehler: die Marke im Text
// kann eine Kennung nennen, die es nicht mehr gibt.
func formularLaden(kennung string) (formular, bool) {
	if !reKennung.MatchString(kennung) {
		return formular{}, false
	}
	roh, ok, err := plugin.Get(praefixFormular + kennung)
	if err != nil || !ok {
		return formular{}, false
	}
	var f formular
	if json.Unmarshal([]byte(roh), &f) != nil || f.Kennung == "" {
		return formular{}, false
	}
	return f, true
}

func formularSichern(f formular) error {
	roh, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return plugin.Set(praefixFormular+f.Kennung, string(roh))
}

// alleFormulare listet, was definiert ist, nach Namen sortiert.
func alleFormulare() []formular {
	roh, err := plugin.List(praefixFormular, 200)
	if err != nil {
		return nil
	}
	out := make([]formular, 0, len(roh))
	for _, v := range roh {
		var f formular
		if json.Unmarshal([]byte(v), &f) == nil && f.Kennung != "" {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// saeubern bringt ein Formular in einen Zustand, in dem es sich ausgeben und
// wieder einlesen lässt.
func (f formular) saeubern() formular {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		f.Name = "Formular"
	}
	if len(f.Name) > 80 {
		f.Name = f.Name[:80]
	}
	f.Betreff = strings.TrimSpace(f.Betreff)
	f.Dank = strings.TrimSpace(f.Dank)

	felder := make([]feld, 0, len(f.Felder))
	belegt := map[string]bool{}
	for _, fe := range f.Felder {
		fe.Beschriftung = strings.TrimSpace(fe.Beschriftung)
		if fe.Beschriftung == "" {
			continue
		}
		if artName(fe.Art) == fe.Art {
			fe.Art = ArtText
		}
		if fe.Kennung == "" {
			fe.Kennung = kennungAus(fe.Beschriftung)
		}
		if fe.Kennung == "" {
			continue
		}
		// Zwei Felder mit derselben Kennung überschrieben einander beim
		// Empfangen — die zweite Antwort käme nie an.
		if belegt[fe.Kennung] {
			continue
		}
		belegt[fe.Kennung] = true

		if fe.Art != ArtAuswahl {
			fe.Auswahl = nil
		} else {
			auswahl := make([]string, 0, len(fe.Auswahl))
			for _, w := range fe.Auswahl {
				if w = strings.TrimSpace(w); w != "" {
					auswahl = append(auswahl, w)
				}
			}
			if len(auswahl) == 0 {
				// Eine Auswahl ohne Möglichkeiten ist ein Feld, das niemand
				// ausfüllen kann.
				fe.Art = ArtText
			}
			fe.Auswahl = auswahl
		}
		felder = append(felder, fe)
		if len(felder) >= maxFelder {
			break
		}
	}
	f.Felder = felder
	return f
}
