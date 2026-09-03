// Der Hofladen als Plugin.
//
// Ein Bestellformular über die eigenen Produkte — und ein Produkt ist eine
// Seite, die ein Preisfeld ausgefüllt hat. Damit steht das Ganze auf dem, was
// der Kern seit den eigenen Feldern kann, und bringt selbst nur mit, was ohne
// Bestellungen niemand braucht: das Formular, die Bestellungen und den
// Bildschirm, auf dem sie liegen.
//
// Kein Warenkorb. Wer bei einem Hof bestellt, wählt einmal aus, was er will,
// und schickt es ab; ein Warenkorb bräuchte eine Sitzung je Besucher, ein
// Plätzchen und eine zweite Seite, und alles davon müsste stimmen, bevor die
// erste Bestellung ankommt. Ein Formular tut es.
//
// Keine Bezahlung. Ein Zahlungsanbieter wäre ein Aufruf nach draussen zur
// Laufzeit — genau die Regel, die dieses CMS ohne Cookie-Banner auskommen
// lässt. Bezahlt wird bei der Übergabe oder per Rechnung, und das steht als
// Hinweis über dem Formular.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Feldnamen des Formulars. Als Konstanten, weil sie an zwei Stellen stehen —
// beim Zeichnen und beim Empfangen — und sonst auseinanderlaufen.
const (
	feldName      = "name"
	feldEmail     = "email"
	feldTelefon   = "telefon"
	feldAdresse   = "adresse"
	feldBemerkung = "bemerkung"
	feldSeite     = "seite"
	feldZeit      = "gestellt"
	feldHonigtopf = "website"
	// mengePraefix + Adresse der Seite ist das Mengenfeld eines Produkts.
	mengePraefix = "menge_"
)

// absendeAdresse ist fest und nicht die der Seite: dann bleibt jede Seite ein
// schlichtes GET, und das Formular funktioniert überall gleich.
const absendeAdresse = "/bestellung"

// Grenzen einer Bestellung. Sie halten eine einzelne Absendung davon ab, eine
// Speicherkarte zu füllen, und sind weit genug für jede echte Bestellung.
const (
	maxName      = 120
	maxEmail     = 254
	maxTelefon   = 40
	maxAdresse   = 400
	maxBemerkung = 2000
	maxMenge     = 999
	maxPosten    = 40
)

// maxProStunde ist die Grenze gegen Sturzfluten.
const maxProStunde = 30

func init() {
	plugin.OnContent(formularEinsetzen)
	plugin.OnRoute(bestellungAnnehmen)
	plugin.OnAdmin(verwaltung)
}

// --- die Marke im Text ------------------------------------------------------

var marke = regexp.MustCompile(`(?i)\[\[bestellung\]\]`)

// markeImAbsatz trifft die Marke, wenn sie allein in einem Absatz steht — was
// sie tut, sobald sie in Markdown auf einer eigenen Zeile steht. Ein Formular
// in einem <p> ist ungültiges HTML, das ein Browser stillschweigend umsortiert;
// dabei landet der Absatz mitten im Formular und die Abstände sitzen falsch.
var markeImAbsatz = regexp.MustCompile(`(?i)<p>\s*\[\[bestellung\]\]\s*</p>`)

// formularEinsetzen ersetzt [[bestellung]] durch das Formular.
func formularEinsetzen(in plugin.ContentIn) (plugin.ContentOut, error) {
	if !marke.MatchString(in.HTML) {
		return plugin.ContentOut{}, nil
	}

	e := einstellungenLaden()
	produkte, err := produkteLesen(e)
	if err != nil {
		plugin.Logf("warn", "Produkte nicht lesbar: %v", err)
		fehltext := `<p>Die Produktliste ist gerade nicht verfügbar.</p>`
		out := markeImAbsatz.ReplaceAllLiteralString(in.HTML, fehltext)
		return plugin.ContentOut{
			HTML:    marke.ReplaceAllLiteralString(out, fehltext),
			Changed: true,
		}, nil
	}

	stand, hinweis := standAusQuery(in.Query)
	formular := zeichnen(produkte, e, in.Slug, stand, hinweis)
	// Der Absatz zuerst, damit das <p> mit seiner Marke verschwindet. In einem
	// Durchgang bliebe ein leeres <p></p> zurück.
	out := markeImAbsatz.ReplaceAllLiteralString(in.HTML, formular)
	out = marke.ReplaceAllLiteralString(out, formular)
	return plugin.ContentOut{HTML: out, Changed: true}, nil
}

// standAusQuery liest, was von der letzten Absendung zurückkam.
func standAusQuery(query string) (stand, hinweis string) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return "", ""
	}
	return q.Get("bestellung"), q.Get("hinweis")
}

// zeichnen baut das Formular.
//
// Von Hand zusammengesetzt statt mit einer Vorlage: das Ergebnis geht durch den
// Filter des Hosts, und jeder Wert, der von aussen kommt, ist hier einzeln
// maskiert. Eine Vorlage mit automatischem Maskieren gibt es in einem
// WASM-Modul nicht umsonst, und eine halbe wäre schlimmer als keine.
func zeichnen(produkte []produkt, e einstellungen, seite, stand, hinweis string) string {
	var b strings.Builder
	b.WriteString(`<div class="bestellung">`)

	switch stand {
	case "gesendet":
		b.WriteString(`<p class="bestellung__ok" role="status">Danke! Deine Bestellung ist angekommen. ` +
			`Wir melden uns bei dir.</p>`)
		b.WriteString(`</div>`)
		return b.String()
	case "fehler":
		text := hinweis
		if text == "" {
			text = "Die Bestellung konnte nicht angenommen werden."
		}
		b.WriteString(`<p class="bestellung__fehler" role="alert">` + html.EscapeString(text) + `</p>`)
	}

	bestellbare := 0
	for _, p := range produkte {
		if p.Bestellbar {
			bestellbare++
		}
	}
	if bestellbare == 0 {
		b.WriteString(`<p>Zurzeit ist nichts zu bestellen.</p></div>`)
		return b.String()
	}

	if e.Hinweis != "" {
		b.WriteString(`<p class="bestellung__hinweis">` + html.EscapeString(e.Hinweis) + `</p>`)
	}

	b.WriteString(`<form class="bestellung__form" method="POST" action="` + absendeAdresse + `">`)
	b.WriteString(`<input type="hidden" name="` + feldSeite + `" value="` + html.EscapeString(seite) + `">`)
	b.WriteString(`<input type="hidden" name="` + feldZeit + `" value="` + html.EscapeString(zeitmarke()) + `">`)
	// Der Honigtopf: ein Feld, das aussieht wie eines und keines ist. Nur mit
	// CSS versteckt, damit ein Vorleseprogramm es überspringen kann und ein
	// Formularausfüller im Browser nichts hineinschreibt.
	b.WriteString(`<p class="bestellung__falle" aria-hidden="true">` +
		`<label>Website<input type="text" name="` + feldHonigtopf + `" tabindex="-1" autocomplete="off"></label></p>`)

	b.WriteString(`<table class="bestellung__tabelle"><thead><tr>` +
		`<th scope="col">Produkt</th><th scope="col">Preis</th><th scope="col">Menge</th>` +
		`</tr></thead><tbody>`)
	for _, p := range produkte {
		b.WriteString(`<tr>`)
		b.WriteString(`<th scope="row"><a href="/` + html.EscapeString(p.Slug) + `">` +
			html.EscapeString(p.Titel) + `</a>`)
		if !p.Bestellbar && p.Zustand != "" {
			b.WriteString(` <span class="bestellung__aus">` + html.EscapeString(p.Zustand) + `</span>`)
		}
		b.WriteString(`</th>`)

		b.WriteString(`<td>` + html.EscapeString(e.Waehrung) + ` ` + html.EscapeString(p.Preis))
		if p.Einheit != "" {
			b.WriteString(` <span class="bestellung__einheit">/ ` + html.EscapeString(p.Einheit) + `</span>`)
		}
		b.WriteString(`</td>`)

		b.WriteString(`<td>`)
		if p.Bestellbar {
			name := mengePraefix + p.Slug
			b.WriteString(`<label class="sr-only" for="` + html.EscapeString(name) + `">Menge ` +
				html.EscapeString(p.Titel) + `</label>`)
			b.WriteString(`<input type="number" inputmode="numeric" min="0" max="` +
				strconv.Itoa(maxMenge) + `" step="1" value="" id="` + html.EscapeString(name) +
				`" name="` + html.EscapeString(name) + `">`)
		} else {
			b.WriteString(`<span class="bestellung__aus">nicht bestellbar</span>`)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	feld := func(name, beschriftung, art string, pflicht bool, hilfe string) {
		b.WriteString(`<p class="bestellung__feld"><label for="b_` + name + `">` +
			html.EscapeString(beschriftung))
		if pflicht {
			b.WriteString(` <span aria-hidden="true">*</span>`)
		}
		b.WriteString(`</label>`)
		if art == "textarea" {
			b.WriteString(`<textarea id="b_` + name + `" name="` + name + `" rows="3"></textarea>`)
		} else {
			b.WriteString(`<input type="` + art + `" id="b_` + name + `" name="` + name + `"`)
			if pflicht {
				b.WriteString(` required`)
			}
			b.WriteString(`>`)
		}
		if hilfe != "" {
			b.WriteString(`<span class="bestellung__hilfe">` + html.EscapeString(hilfe) + `</span>`)
		}
		b.WriteString(`</p>`)
	}

	feld(feldName, "Name", "text", true, "")
	feld(feldEmail, "E-Mail", "email", true, "")
	feld(feldTelefon, "Telefon", "tel", false, "Freiwillig — hilft bei Rückfragen.")
	feld(feldAdresse, "Adresse", "textarea", false, "Nur nötig, wenn geliefert werden soll.")
	feld(feldBemerkung, "Bemerkung", "textarea", false, "")

	b.WriteString(`<p class="bestellung__abschicken">` +
		`<button type="submit">Bestellung abschicken</button></p>`)
	b.WriteString(`</form></div>`)
	return b.String()
}

// --- die Absendung ----------------------------------------------------------

// bestellungAnnehmen nimmt die Bestellung entgegen.
func bestellungAnnehmen(in plugin.RequestIn) (plugin.RequestOut, error) {
	if in.Method != "POST" {
		// Ein GET auf diese Adresse ist jemand, der den Link geöffnet hat. Zur
		// Startseite statt auf eine leere Seite.
		return plugin.RequestOut{Handled: true, Status: 303, Location: "/"}, nil
	}

	form, err := url.ParseQuery(in.Body)
	if err != nil {
		return zurueck("", "fehler", "Die Bestellung war nicht lesbar."), nil
	}
	seite := saeubern(form.Get(feldSeite), 200)

	// Der Honigtopf zuerst: was hier hineingeschrieben wurde, war kein Mensch.
	// Die Antwort sieht aus wie ein Erfolg, damit ein Skript nicht lernt,
	// woran es gescheitert ist.
	if strings.TrimSpace(form.Get(feldHonigtopf)) != "" {
		plugin.Log("info", "Bestellung mit gefülltem Honigtopf verworfen")
		return zurueck(seite, "gesendet", ""), nil
	}
	if !zeitmarkeGilt(form.Get(feldZeit)) {
		return zurueck(seite, "fehler",
			"Das Formular ist abgelaufen. Bitte lade die Seite neu und schicke es noch einmal."), nil
	}

	e := einstellungenLaden()
	produkte, err := produkteLesen(e)
	if err != nil {
		return plugin.RequestOut{}, err
	}

	posten, problem := postenLesen(form, produkte, e)
	if problem != "" {
		return zurueck(seite, "fehler", problem), nil
	}

	b := bestellung{
		Name:      saeubern(form.Get(feldName), maxName),
		Email:     saeubern(form.Get(feldEmail), maxEmail),
		Telefon:   saeubern(form.Get(feldTelefon), maxTelefon),
		Adresse:   saeubern(form.Get(feldAdresse), maxAdresse),
		Bemerkung: saeubern(form.Get(feldBemerkung), maxBemerkung),
		Seite:     seite,
		Posten:    posten,
		Waehrung:  e.Waehrung,
	}
	if b.Name == "" {
		return zurueck(seite, "fehler", "Bitte trage deinen Namen ein."), nil
	}
	if !adresseSiehtEchtAus(b.Email) {
		return zurueck(seite, "fehler", "Bitte trage eine gültige E-Mail-Adresse ein."), nil
	}
	b.Summe, b.SummeBekannt = summe(posten)

	if !unterDerStundengrenze() {
		return zurueck(seite, "fehler",
			"Gerade gehen sehr viele Bestellungen ein. Bitte versuche es in einer Stunde noch einmal."), nil
	}
	if err := speichern(&b); err != nil {
		return plugin.RequestOut{}, err
	}
	benachrichtigen(b)
	return zurueck(seite, "gesendet", ""), nil
}

// postenLesen sammelt die bestellten Mengen.
//
// Preis und Bezeichnung kommen aus der Seite und nicht aus dem Formular: sonst
// bestimmte der Besucher, was etwas kostet. Aus dem Formular kommt die Menge,
// und sonst nichts.
func postenLesen(form url.Values, produkte []produkt, e einstellungen) ([]posten, string) {
	var out []posten
	for _, p := range produkte {
		roh := strings.TrimSpace(form.Get(mengePraefix + p.Slug))
		if roh == "" || roh == "0" {
			continue
		}
		menge, err := strconv.Atoi(roh)
		if err != nil || menge < 0 {
			return nil, "Bei „" + p.Titel + "“ steht keine Zahl."
		}
		if menge == 0 {
			continue
		}
		if menge > maxMenge {
			return nil, "Bei „" + p.Titel + "“ ist die Menge zu gross. Bitte melde dich direkt bei uns."
		}
		if !p.Bestellbar {
			return nil, "„" + p.Titel + "“ ist zurzeit nicht bestellbar."
		}
		out = append(out, posten{
			Slug: p.Slug, Titel: p.Titel, Menge: menge,
			Preis: p.Preis, Einheit: p.Einheit,
		})
		if len(out) > maxPosten {
			return nil, "Das sind sehr viele verschiedene Posten. Bitte melde dich direkt bei uns."
		}
	}
	if len(out) == 0 {
		return nil, "Bitte trage bei mindestens einem Produkt eine Menge ein."
	}
	_ = e
	return out, ""
}

// summe rechnet zusammen, wenn sich alle Preise lesen lassen.
//
// Lässt sich einer nicht lesen, gibt es keine Summe statt einer falschen: der
// Betreiber sieht die Posten und rechnet nach, und niemand bekommt einen Betrag
// bestätigt, der nicht stimmt.
func summe(posten []posten) (float64, bool) {
	total := 0.0
	for _, p := range posten {
		wert, ok := preisWert(p.Preis)
		if !ok {
			return 0, false
		}
		total += wert * float64(p.Menge)
	}
	return total, true
}

// zurueck schickt den Besucher auf die Seite zurück, mit dem Ausgang in der
// Adresse. Umleitung statt einer Antwort im Rumpf, damit ein Neuladen die
// Bestellung nicht ein zweites Mal abschickt.
func zurueck(seite, stand, hinweis string) plugin.RequestOut {
	ziel := "/"
	if seite != "" {
		ziel = "/" + seite
	}
	q := url.Values{}
	q.Set("bestellung", stand)
	if hinweis != "" {
		q.Set("hinweis", hinweis)
	}
	return plugin.RequestOut{Handled: true, Status: 303, Location: ziel + "?" + q.Encode()}
}

// benachrichtigen sagt dem Betreiber Bescheid.
//
// Nach dem Speichern: eine Bestellung, die in der Verwaltung steht, ist
// angekommen — ob die E-Mail durchkommt, ändert daran nichts. Ein Fehler hier
// darf den Besteller nie erreichen.
func benachrichtigen(b bestellung) {
	var t strings.Builder
	fmt.Fprintf(&t, "Neue Bestellung von %s\n\n", b.Name)
	for _, p := range b.Posten {
		fmt.Fprintf(&t, "  %d × %s", p.Menge, p.Titel)
		if p.Preis != "" {
			fmt.Fprintf(&t, "  (%s %s", b.Waehrung, p.Preis)
			if p.Einheit != "" {
				fmt.Fprintf(&t, " / %s", p.Einheit)
			}
			t.WriteString(")")
		}
		t.WriteString("\n")
	}
	if b.SummeBekannt {
		fmt.Fprintf(&t, "\nSumme: %s %s\n", b.Waehrung, betragTexten(b.Summe))
	} else {
		t.WriteString("\nSumme: konnte nicht gerechnet werden – bitte nachrechnen.\n")
	}
	fmt.Fprintf(&t, "\nE-Mail: %s\n", b.Email)
	if b.Telefon != "" {
		fmt.Fprintf(&t, "Telefon: %s\n", b.Telefon)
	}
	if b.Adresse != "" {
		fmt.Fprintf(&t, "Adresse:\n%s\n", b.Adresse)
	}
	if b.Bemerkung != "" {
		fmt.Fprintf(&t, "\nBemerkung:\n%s\n", b.Bemerkung)
	}
	t.WriteString("\nDie Bestellung steht auch in der Verwaltung unter „Bestellungen“.\n")

	// Die Adresse des Bestellers als Antwortadresse: der Betreiber drückt auf
	// Antworten und schreibt dem Kunden, ohne die Adresse abzutippen.
	queued, grund, err := plugin.Notify("Neue Bestellung von "+b.Name, t.String(), b.Email)
	switch {
	case err != nil:
		plugin.Logf("warn", "Benachrichtigung über die Bestellung ging nicht raus: %v", err)
	case !queued:
		plugin.Logf("info", "keine Benachrichtigung verschickt: %s", grund)
	}
}

// --- Kleinkram --------------------------------------------------------------

// saeubern nimmt Steuerzeichen heraus und kürzt.
func saeubern(v string, max int) string {
	v = strings.TrimSpace(v)
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, v)
	if len(v) > max {
		v = v[:max]
	}
	return v
}

// adresseSiehtEchtAus prüft so viel, wie sich prüfen lässt, ohne hinzuschreiben.
func adresseSiehtEchtAus(v string) bool {
	at := strings.LastIndex(v, "@")
	if at <= 0 || at == len(v)-1 || len(v) > maxEmail {
		return false
	}
	rest := v[at+1:]
	return strings.Contains(rest, ".") && !strings.ContainsAny(v, " \t\n")
}

// --- Zeitmarke gegen Skripte ------------------------------------------------

// zeitmarke ist der Zeitpunkt, zu dem das Formular gezeichnet wurde, signiert.
//
// Signiert, weil ein Wert, den der Absender frei setzen kann, keine Aussage
// ist. Der Schlüssel liegt im Speicher des Plugins und wird beim ersten Mal
// erzeugt: ein fest eingebauter stünde in jeder Kopie dieses Quelltexts.
func zeitmarke() string {
	jetzt := strconv.FormatInt(time.Now().Unix(), 10)
	return jetzt + "." + zeichen(jetzt)
}

func zeitmarkeGilt(v string) bool {
	teile := strings.SplitN(v, ".", 2)
	if len(teile) != 2 || !hmac.Equal([]byte(zeichen(teile[0])), []byte(teile[1])) {
		return false
	}
	gestellt, err := strconv.ParseInt(teile[0], 10, 64)
	if err != nil {
		return false
	}
	alter := time.Now().Unix() - gestellt
	// Nach unten, weil ein Mensch nicht in zwei Sekunden eine Bestellung
	// ausfüllt; nach oben, weil ein Formular, das seit einem Tag offen steht,
	// von einer Seite stammt, deren Preise sich geändert haben könnten.
	return alter >= 2 && alter <= 12*60*60
}

func zeichen(v string) string {
	m := hmac.New(sha256.New, schluessel())
	m.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

const schluesselMarke = "zeitmarken-schluessel"

func schluessel() []byte {
	if roh, da, err := plugin.Get(schluesselMarke); err == nil && da && roh != "" {
		if key, err := base64.RawStdEncoding.DecodeString(roh); err == nil && len(key) == 32 {
			return key
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// Ohne Zufall keine Signatur. Ein fester Wert wäre schlimmer als gar
		// keine Marke, also gibt es hier keinen.
		plugin.Logf("error", "kein Zufall für den Schlüssel: %v", err)
		return nil
	}
	if err := plugin.Set(schluesselMarke, base64.RawStdEncoding.EncodeToString(key)); err != nil {
		plugin.Logf("warn", "Schlüssel nicht gesichert: %v", err)
	}
	return key
}

// --- Stundengrenze ----------------------------------------------------------

const schluesselZaehler = "zaehler"

// unterDerStundengrenze zählt die Bestellungen der laufenden Stunde.
func unterDerStundengrenze() bool {
	stunde := time.Now().UTC().Format("2006-01-02T15")
	roh, _, err := plugin.Get(schluesselZaehler)
	if err != nil {
		return true
	}
	var z struct {
		Stunde string `json:"stunde"`
		Anzahl int    `json:"anzahl"`
	}
	_ = json.Unmarshal([]byte(roh), &z)
	if z.Stunde != stunde {
		z.Stunde, z.Anzahl = stunde, 0
	}
	if z.Anzahl >= maxProStunde {
		return false
	}
	z.Anzahl++
	if neu, err := json.Marshal(z); err == nil {
		_ = plugin.Set(schluesselZaehler, string(neu))
	}
	return true
}

// --- Speicher ---------------------------------------------------------------

// posten ist eine Zeile der Bestellung.
type posten struct {
	Slug    string `json:"slug"`
	Titel   string `json:"titel"`
	Menge   int    `json:"menge"`
	Preis   string `json:"preis,omitempty"`
	Einheit string `json:"einheit,omitempty"`
}

// bestellung ist, was gespeichert wird.
//
// Die Preise stehen mit darin und werden nicht später aus der Seite geholt:
// ändert sich der Preis morgen, gilt für diese Bestellung der von heute.
type bestellung struct {
	ID           string   `json:"id"`
	Eingegangen  string   `json:"eingegangen"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Telefon      string   `json:"telefon,omitempty"`
	Adresse      string   `json:"adresse,omitempty"`
	Bemerkung    string   `json:"bemerkung,omitempty"`
	Seite        string   `json:"seite,omitempty"`
	Posten       []posten `json:"posten"`
	Summe        float64  `json:"summe,omitempty"`
	SummeBekannt bool     `json:"summe_bekannt,omitempty"`
	Waehrung     string   `json:"waehrung,omitempty"`
	// Erledigt ist gesetzt, wenn der Betreiber die Bestellung abgehakt hat.
	Erledigt bool `json:"erledigt,omitempty"`
}

const praefixBestellung = "bestellung:"

// speichern legt die Bestellung ab.
//
// Der Schlüssel beginnt mit dem Zeitpunkt, damit die Liste nach Datum sortiert
// zurückkommt, und endet auf Zufall, damit zwei Bestellungen in derselben
// Sekunde einander nicht überschreiben.
func speichern(b *bestellung) error {
	jetzt := time.Now().UTC()
	b.Eingegangen = jetzt.Format(time.RFC3339)
	roh := make([]byte, 6)
	if _, err := rand.Read(roh); err != nil {
		return err
	}
	b.ID = jetzt.Format("20060102T150405") + "-" + base64.RawURLEncoding.EncodeToString(roh)

	daten, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return plugin.Set(praefixBestellung+b.ID, string(daten))
}

// alleBestellungen liest sie, neueste zuerst.
func alleBestellungen() ([]bestellung, error) {
	werte, err := plugin.List(praefixBestellung, 500)
	if err != nil {
		return nil, err
	}
	out := make([]bestellung, 0, len(werte))
	for _, roh := range werte {
		var b bestellung
		if err := json.Unmarshal([]byte(roh), &b); err != nil {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func bestellungLaden(id string) (bestellung, bool) {
	roh, da, err := plugin.Get(praefixBestellung + id)
	if err != nil || !da {
		return bestellung{}, false
	}
	var b bestellung
	if err := json.Unmarshal([]byte(roh), &b); err != nil {
		return bestellung{}, false
	}
	return b, true
}

func bestellungSichern(b bestellung) error {
	daten, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return plugin.Set(praefixBestellung+b.ID, string(daten))
}

func main() {}
