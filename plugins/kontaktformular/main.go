// Das Kontaktformular als Plugin.
//
// Drei Teile, die zusammengehören: die Marke [[formular]] im Text wird zum
// Formular, /formular nimmt die Absendung entgegen, und in der Verwaltung
// liegen die Nachrichten. Alle drei in einem Modul, weil sie sich dieselbe
// Vorstellung von Feldnamen, Fallen und Speicher teilen — auf drei Plugins
// verteilt wäre die erste Änderung an einem Feldnamen ein stiller Bruch.
//
// Warum das kein Kern ist: eine Website, die keine Nachrichten entgegennimmt,
// braucht weder das Formular noch die Tabelle dahinter noch den Bildschirm, auf
// dem nie etwas steht. Wer nur eine Telefonnummer veröffentlicht, soll das
// Ganze nicht mitschleppen.
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

// Feldnamen. Sie stehen im gezeichneten Formular und werden beim Empfang wieder
// gelesen; als Konstanten, damit die beiden Stellen nicht auseinanderlaufen.
const (
	feldName      = "name"
	feldEmail     = "email"
	feldBetreff   = "betreff"
	feldText      = "nachricht"
	feldSeite     = "seite"
	feldZeit      = "gestellt"
	feldHonigtopf = "website"
	// feldFormular sagt beim Empfang, welches zusammengestellte Formular
	// abgesendet wurde. Ohne es müsste der Empfang aus den Feldnamen raten.
	feldFormular = "formular"
)

// absendeAdresse ist eine feste Adresse und nicht die der Seite: dann bleibt
// jede Seite ein schlichtes GET, und das Formular funktioniert überall gleich.
const absendeAdresse = "/formular"

// Grenzen der Felder. Sie sind das, was eine einzelne Absendung davon abhält,
// eine Speicherkarte zu füllen, und weit genug für jede echte Anfrage.
const (
	maxName    = 120
	maxEmail   = 254 // die längste Adresse, die RFC 5321 zulässt
	maxBetreff = 200
	maxText    = 8000
)

// mindestDauer ist, was ein Mensch mindestens braucht.
//
// Drei Sekunden liegen unter dem, was jemand zum Lesen und Tippen braucht, und
// weit über dem, was ein Skript braucht. Länger würde anfangen, Leute
// abzuweisen, die eine vorbereitete Nachricht einfügen.
const mindestDauer = 3 * time.Second

// maxAlter ist, wie lange ein gezeichnetes Formular gültig bleibt. Eine Seite
// kann lange in einem Tab stehen; der Punkt ist nur, dass eine einmal
// abgeschriebene Zeitmarke nicht wochenlang nachnutzbar ist.
const maxAlter = 12 * time.Hour

// stundenGrenze ist, wie viele Nachrichten eine Website pro Stunde annimmt.
//
// Honigtopf und Zeitfalle halten den gewöhnlichen Spam-Roboter auf. Das hier
// hält den, der trotzdem durchkommt, davon ab, über Nacht die Platte eines
// kleinen Servers zu verfüllen — und das lässt sich hinterher durch kein
// Filtern wieder gutmachen.
const stundenGrenze = 30

// maxNachrichten ist, wie viele Nachrichten aufgehoben werden.
const maxNachrichten = 500

const (
	praefixNachricht = "nachricht:"
	praefixZaehler   = "zaehler:"
	schluesselName   = "signaturschluessel"
)

// nachricht ist eine eingegangene Anfrage.
type nachricht struct {
	// Kennung ist der Speicherschlüssel ohne Präfix, damit ein Formular in der
	// Verwaltung auf eine einzelne Nachricht zeigen kann.
	Kennung string `json:"kennung"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Betreff string `json:"betreff,omitempty"`
	Text    string `json:"text"`
	Seite   string `json:"seite,omitempty"`
	Zeit    string `json:"zeit"`
	Gelesen bool   `json:"gelesen,omitempty"`
	// Formular und FormularName sagen, woher die Nachricht kam. Leer für das
	// eingebaute Kontaktformular.
	Formular     string `json:"formular,omitempty"`
	FormularName string `json:"formularname,omitempty"`
	// Felder sind die Antworten eines zusammengestellten Formulars, in der
	// Reihenfolge, in der gefragt wurde.
	Felder []antwort `json:"felder,omitempty"`
}

func init() {
	plugin.OnContent(formularEinsetzen)
	plugin.OnRoute(absendungAnnehmen)
	plugin.OnAdmin(verwaltung)
}

// --- die Marke im Text ------------------------------------------------------

// marke trifft die Marke mit oder ohne Betreff:
//
//	[[formular]]              ein schlichtes Kontaktformular
//	[[formular:Rohwolle]]     dasselbe, mit "Rohwolle" schon im Betreff
//
// Der Betreff ist für eine Seite, die eine Sache anbietet. Ohne ihn kommt jede
// Anfrage von jeder Produktseite mit leerer Betreffzeile an, und wer sie liest,
// muss jede einzelne öffnen, um zu sehen, um welches von fünf Dingen es geht.
//
// Die eckige Klammer ist im Betreff ausgeschlossen, damit die Marke bei
// fehlender schliessender Klammer nicht den Rest der Seite verschluckt.
var marke = regexp.MustCompile(`\[\[formular(?::([^\]\[]{1,` + strconv.Itoa(maxBetreff) + `}))?\]\]`)

// markeImAbsatz ist dieselbe Marke mit dem Absatz, den goldmark darum legt.
//
// Eine Marke allein in einem Absatz ersetzt den ganzen Absatz: ein <form> in
// einem <p> ist ungültiges HTML, das ein Browser stillschweigend umsortiert —
// er zieht das Formular heraus und lässt die Felder zurück.
var markeImAbsatz = regexp.MustCompile(`<p>` + marke.String() + `</p>`)

func formularEinsetzen(in plugin.ContentIn) (plugin.ContentOut, error) {
	if !marke.MatchString(in.HTML) {
		return plugin.ContentOut{}, nil
	}

	daten := formulardaten(in)
	// Nichts Getipptes: was ein Besucher eingegeben hat, wird nirgends
	// zwischengespeichert, und es über die Adresse zurückzuschicken hiesse, den
	// Text einer fremden Person in eine Adresse zu schreiben, die im Verlauf
	// und in jedem Serverprotokoll landet. Eine abgelehnte Absendung sagt
	// deshalb, was fehlt; die Eingaben hat der Browser beim Zurückgehen noch.
	var werte url.Values
	// Der Absatz zuerst, damit das <p> mit seiner Marke verschwindet. In einem
	// Durchgang bliebe ein leeres <p></p> zurück.
	out := ersetzen(markeImAbsatz, in.HTML, daten, werte)
	out = ersetzen(marke, out, daten, werte)
	return plugin.ContentOut{HTML: out, Changed: out != in.HTML}, nil
}

func ersetzen(re *regexp.Regexp, seite string, d daten, werte url.Values) string {
	return re.ReplaceAllStringFunc(seite, func(treffer string) string {
		arg := ""
		if m := marke.FindStringSubmatch(treffer); m != nil {
			arg = strings.TrimSpace(m[1])
		}

		// Ein Argument, das ein zusammengestelltes Formular benennt, bringt
		// dieses; jedes andere ist wie bisher ein vorausgefüllter Betreff. So
		// bleibt [[formular:Rohwolle]] genau das, was es war, und
		// [[formular:hoffest]] ist etwas Neues.
		if arg != "" {
			if f, ok := formularLaden(arg); ok {
				eigen := d
				if d.Formular != "" && d.Formular != f.Kennung {
					// Die Antwort gehört zu einem anderen Formular auf
					// derselben Seite; dieses hier zeigt keine fremde Meldung.
					eigen.Hinweis, eigen.IstFehler = "", false
					werte = nil
				}
				return zeichnenEigen(f, eigen, werte)
			}
		}

		eigen := d
		if arg != "" && eigen.Betreff == "" {
			// Was der Besucher selbst getippt hat, gewinnt: seine Absendung
			// wurde aus einem anderen Grund abgelehnt, und ihm dabei auch noch
			// den Betreff zu überschreiben wäre das Zweite, was ihm passiert.
			eigen.Betreff = arg
		}
		if d.Formular != "" {
			eigen.Hinweis, eigen.IstFehler = "", false
		}
		return zeichnen(eigen)
	})
}

// daten ist alles, was das gezeichnete Formular braucht.
type daten struct {
	Seite string
	// Formular ist die Kennung des Formulars, dessen Absendung gerade
	// beantwortet wird. Nur dieses zeigt die Meldung — auf einer Seite mit
	// zwei Formularen stünde sie sonst zweimal.
	Formular  string
	Zeitmarke string
	Kontakt   string
	Hinweis   string
	IstFehler bool
	Name      string
	Email     string
	Betreff   string
	Text      string
}

func formulardaten(in plugin.ContentIn) daten {
	d := daten{Seite: in.Slug, Zeitmarke: zeitmarke(time.Now())}
	if s, err := plugin.Site(); err == nil {
		d.Kontakt = s.ContactEmail
	}

	q, err := url.ParseQuery(in.Query)
	if err != nil {
		return d
	}
	d.Formular = q.Get("welches")
	switch q.Get("formular") {
	case "gesendet":
		f, ok := formularLaden(d.Formular)
		d.Hinweis = hinweisFuer(f, ok)
	case "fehler":
		d.IstFehler = true
		d.Hinweis = hinweistext(q.Get("hinweis"))
	}
	return d
}

// hinweistext begrenzt, was die Abfragezeichenkette auf die Seite bringen darf.
//
// Ausgegeben wird ohnehin maskiert; die Grenze richtet sich gegen einen Link,
// der eine Seite fremden Textes in das Layout einer Website setzt — so wird aus
// einem schlichten Kontaktformular eine Phishing-Seite.
func hinweistext(roh string) string {
	roh = strings.TrimSpace(roh)
	if roh == "" || len([]rune(roh)) > 160 {
		return "Die Nachricht konnte nicht gesendet werden. Bitte prüfe deine Angaben."
	}
	return roh
}

// zeichnen baut das Formular.
//
// In Go und nicht im Theme: die Auszeichnung trägt den Honigtopf, die
// Zeitmarke und die Feldnamen, die der Empfang erwartet. Ein Theme, das eines
// davon falsch hätte, brächte ein Formular hervor, das jeden echten Besucher
// stillschweigend abweist. Gestaltet wird über die Klassen.
func zeichnen(d daten) string {
	e := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, `<form class="contact-form" method="POST" action="%s">`, absendeAdresse)
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, feldZeit, e(d.Zeitmarke))
	fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, feldSeite, e(d.Seite))

	if d.Hinweis != "" {
		klasse := "contact-form__notice"
		if d.IstFehler {
			klasse += " contact-form__notice--error"
		}
		fmt.Fprintf(&b, `<p class="%s" role="status">%s</p>`, klasse, e(d.Hinweis))
	}

	feld := func(id, label, typ, name, wert string, max int, extra string) {
		fmt.Fprintf(&b, `<div class="contact-form__field"><label for="%s">%s</label>`+
			`<input type="%s" id="%s" name="%s" maxlength="%d" value="%s" %s></div>`,
			id, label, typ, id, name, max, e(wert), extra)
	}
	feld("cf-name", "Name", "text", feldName, d.Name, maxName, `required autocomplete="name"`)
	feld("cf-email", "E-Mail", "email", feldEmail, d.Email, maxEmail, `required autocomplete="email"`)
	feld("cf-subject", "Betreff", "text", feldBetreff, d.Betreff, maxBetreff, "")

	fmt.Fprintf(&b, `<div class="contact-form__field"><label for="cf-body">Nachricht</label>`+
		`<textarea id="cf-body" name="%s" rows="8" required maxlength="%d">%s</textarea></div>`,
		feldText, maxText, e(d.Text))

	// Kein sichtbares Feld. Vor Menschen versteckt es das Stylesheet, vor
	// Vorleseprogrammen aria-hidden und tabindex; wer es so oder so nicht sieht,
	// lässt es in Ruhe. Ein Programm, das jedes Eingabefeld ausfüllt, das es
	// findet, füllt auch dieses aus — genau dafür ist es da.
	fmt.Fprintf(&b, `<div class="contact-form__trap" aria-hidden="true">`+
		`<label for="cf-website">Website (bitte leer lassen)</label>`+
		`<input type="text" id="cf-website" name="%s" tabindex="-1" autocomplete="off"></div>`,
		feldHonigtopf)

	b.WriteString(`<button type="submit" class="contact-form__submit">Nachricht senden</button>`)
	if d.Kontakt != "" {
		fmt.Fprintf(&b, `<p class="contact-form__alternative">Lieber direkt schreiben? `+
			`<a href="mailto:%s">%s</a></p>`, e(d.Kontakt), e(d.Kontakt))
	}
	b.WriteString(`</form>`)
	return b.String()
}

// --- die Zeitmarke ----------------------------------------------------------

// zeitmarke unterschreibt den Moment, in dem das Formular gezeichnet wurde.
//
// Unterschrieben, weil ein ungezeichnetes verstecktes Feld eines ist, das ein
// Roboter einfach zurückdatiert — die Zeitfalle wäre dann ein Kommentar im
// Quelltext und sonst nichts.
func zeitmarke(jetzt time.Time) string {
	stempel := strconv.FormatInt(jetzt.UTC().Unix(), 10)
	return stempel + "." + unterschrift(stempel)
}

// pruefeZeitmarke gibt einen Grund zurück, oder "" wenn alles stimmt.
func pruefeZeitmarke(marke string, jetzt time.Time) string {
	stempel, sig, ok := strings.Cut(marke, ".")
	if !ok {
		return "gefaelscht"
	}
	// Über die volle Länge verglichen: ein Vergleich, der beim ersten falschen
	// Byte aufhört, verrät, wie viel von der Unterschrift schon stimmt.
	if !hmac.Equal([]byte(sig), []byte(unterschrift(stempel))) {
		return "gefaelscht"
	}
	sekunden, err := strconv.ParseInt(stempel, 10, 64)
	if err != nil {
		return "gefaelscht"
	}
	alter := jetzt.UTC().Sub(time.Unix(sekunden, 0).UTC())
	switch {
	case alter < 0:
		// Aus der Zukunft kann nur kommen, wessen Uhr zurückgestellt wurde —
		// oder eine Fälschung, die die Unterschrift trotzdem getroffen hat.
		return "gefaelscht"
	case alter < mindestDauer:
		return "zu schnell"
	case alter > maxAlter:
		return "abgelaufen"
	}
	return ""
}

func unterschrift(stempel string) string {
	mac := hmac.New(sha256.New, signaturschluessel())
	mac.Write([]byte(stempel))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// schluessel wird einmal gezogen und im installationsweiten Speicher behalten.
//
// Eigener Schlüssel und nicht der des Servers: das Plugin bekommt dessen
// Geheimnis nicht zu sehen, und das ist richtig so. Er muss über Neustarts
// halten, sonst wäre jedes offene Formular nach einem Neustart ungültig.
var schluesselCache []byte

func signaturschluessel() []byte {
	if schluesselCache != nil {
		return schluesselCache
	}
	if roh, ok, _ := plugin.GlobalGet(schluesselName); ok && roh != "" {
		if b, err := base64.RawStdEncoding.DecodeString(roh); err == nil && len(b) == 32 {
			schluesselCache = b
			return schluesselCache
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Ohne Zufall keine tragfähige Unterschrift. Ein fester Rückfall wäre
		// schlimmer als gar keine Zeitfalle, weil er wie eine aussieht.
		plugin.Log("error", "kein Zufall verfügbar, die Zeitfalle bleibt aus")
		return nil
	}
	if err := plugin.GlobalSet(schluesselName, base64.RawStdEncoding.EncodeToString(b)); err != nil {
		plugin.Logf("error", "der Signaturschlüssel liess sich nicht sichern: %v", err)
	}
	schluesselCache = b
	return schluesselCache
}

// --- der Empfang ------------------------------------------------------------

func absendungAnnehmen(in plugin.RequestIn) (plugin.RequestOut, error) {
	if in.Method != "POST" {
		return zurueck("", "fehler", "Das Formular konnte nicht gelesen werden."), nil
	}
	form, err := url.ParseQuery(in.Body)
	if err != nil {
		return zurueck("", "fehler", "Das Formular konnte nicht gelesen werden."), nil
	}
	seite := seitenname(form.Get(feldSeite))

	// Die beiden Fallen, in der Reihenfolge, die am wenigsten kostet. Ein
	// gefüllter Honigtopf und ein Formular, das in unter drei Sekunden
	// zurückkommt, werden beide genau wie ein Erfolg beantwortet: ein Roboter,
	// der erfährt, welche seiner Absendungen abgelehnt wurden, erfährt damit,
	// wie er am Filter vorbeikommt.
	if strings.TrimSpace(form.Get(feldHonigtopf)) != "" {
		plugin.Log("info", "Honigtopf ausgelöst")
		return zurueck(seite, "gesendet", ""), nil
	}
	switch grund := pruefeZeitmarke(form.Get(feldZeit), time.Now()); grund {
	case "":
	case "abgelaufen":
		// Wer einen Tag lang einen Tab offen hatte, verdient eine Antwort und
		// kein Schweigen — die Nachricht ist echt und steht noch im Feld.
		return zurueck(seite, "fehler",
			"Das Formular war zu lange offen. Bitte lade die Seite neu und sende noch einmal."), nil
	default:
		plugin.Logf("info", "Absendung abgewiesen: %s", grund)
		return zurueck(seite, "gesendet", ""), nil
	}

	// Ein zusammengestelltes Formular sagt selbst, welches es ist. Steht dort
	// eine Kennung, die es nicht mehr gibt, wird die Absendung abgelehnt statt
	// als eingebautes Formular gelesen: die Felder passten nicht zusammen, und
	// heraus käme eine leere Nachricht.
	welches := strings.TrimSpace(form.Get(feldFormular))
	var n nachricht
	if welches != "" {
		f, ok := formularLaden(welches)
		if !ok {
			return zurueckAn(seite, welches, "fehler",
				"Dieses Formular gibt es nicht mehr. Bitte lade die Seite neu."), nil
		}
		var problem string
		n, problem = empfangenEigen(f, form, seite)
		if problem != "" {
			return zurueckAn(seite, welches, "fehler", problem), nil
		}
	} else {
		n = nachricht{
			Name:    strings.TrimSpace(form.Get(feldName)),
			Email:   strings.TrimSpace(form.Get(feldEmail)),
			Betreff: strings.TrimSpace(form.Get(feldBetreff)),
			Text:    strings.TrimSpace(form.Get(feldText)),
			Seite:   seite,
		}
		if problem := pruefen(n); problem != "" {
			return zurueck(seite, "fehler", problem), nil
		}
	}

	if !platzInDieserStunde() {
		return zurueckAn(seite, welches, "fehler",
			"Gerade sind sehr viele Nachrichten eingegangen. Bitte versuche es in einer Stunde noch einmal."), nil
	}
	if err := speichern(n); err != nil {
		return plugin.RequestOut{}, err
	}
	benachrichtigen(n)
	return zurueckAn(seite, welches, "gesendet", ""), nil
}

// benachrichtigen sagt dem Betreiber Bescheid, wenn er das eingerichtet hat.
//
// Nach dem Speichern und nicht davor: eine Anfrage, die in der Verwaltung
// steht, ist angekommen — ob die Benachrichtigung durchkommt, ändert daran
// nichts. Ein Fehler hier darf den Besucher nie erreichen; für ihn ist die
// Nachricht abgeschickt, und das stimmt auch.
//
// Der Empfänger steht in den Einstellungen der Website, nicht hier. Das Plugin
// kann keine Adresse nennen, und das ist der Grund, warum es diese Berechtigung
// überhaupt bekommen darf.
func benachrichtigen(n nachricht) {
	betreff := n.Betreff
	if betreff == "" {
		betreff = "Neue Anfrage"
	}
	if n.FormularName != "" && !strings.Contains(betreff, n.FormularName) {
		betreff = n.FormularName + ": " + betreff
	}
	von := n.Seite
	if von == "" {
		von = "Startseite"
	} else {
		von = "/" + von
	}

	text := fmt.Sprintf(`Über das Formular auf %s ist eine Anfrage eingegangen.

Von:      %s <%s>
Betreff:  %s
Am:       %s

%s

--
Antworten geht direkt auf diese Nachricht — sie geht an den Absender.
`, von, n.Name, n.Email, betreff, kurzDatum(n.Zeit), n.Text)

	// Die Adresse des Absenders als Antwortadresse: dann ist Antworten ein
	// Klick und nicht ein Wechsel in die Verwaltung, Kopieren, Einfügen.
	queued, grund, err := plugin.Notify(betreff, text, n.Email)
	switch {
	case err != nil:
		plugin.Logf("error", "Benachrichtigung fehlgeschlagen: %v", err)
	case !queued && grund != "":
		// Kein Fehler: kein Mailserver oder keine Adresse ist eine Entscheidung
		// und keine Panne. Einmal auf debug, damit man beim Suchen fündig wird.
		plugin.Logf("debug", "keine Benachrichtigung verschickt: %s", grund)
	}
}

// zurueck schickt den Besucher auf die Seite zurück und trägt das Ergebnis in
// der Adresse mit. Ohne JavaScript, und ein Neuladen sendet nicht zweimal.
func zurueck(seite, stand, hinweis string) plugin.RequestOut {
	return zurueckAn(seite, "", stand, hinweis)
}

// zurueckAn ist dasselbe, sagt aber auch, welches Formular gemeint ist — auf
// einer Seite mit zwei Formularen stünde die Meldung sonst unter beiden.
func zurueckAn(seite, welches, stand, hinweis string) plugin.RequestOut {
	ziel := "/"
	if seite != "" {
		ziel = "/" + seite
	}
	q := url.Values{}
	q.Set("formular", stand)
	if welches != "" {
		q.Set("welches", welches)
	}
	if hinweis != "" {
		q.Set("hinweis", hinweis)
	}
	return plugin.RequestOut{
		Handled: true,
		// 303: der Browser muss mit GET folgen, sonst sendet ein Neuladen die
		// Nachricht ein zweites Mal.
		Status:   303,
		Location: ziel + "?" + q.Encode(),
	}
}

// seitenname säubert den Seitenverweis, den das Formular mitgebracht hat.
//
// Er kommt aus der Anfrage und landet in einer Location-Kopfzeile. Alles, was
// die Website verlassen oder eine Kopfzeile anfügen könnte, wird verworfen und
// nicht maskiert — dann ist das schlimmste Ergebnis eine Weiterleitung auf die
// Startseite.
func seitenname(roh string) string {
	roh = strings.TrimSpace(roh)
	if roh == "" || len(roh) > 200 {
		return ""
	}
	for _, r := range roh {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return ""
		}
	}
	return roh
}

// pruefen gibt zurück, was an einer Absendung als Erstes fehlt.
//
// Die Sätze sind für den Besucher, also schlichtes Deutsch, das sagt, was zu
// tun ist — eine mit "Validierungsfehler" abgewiesene Anfrage ist eine
// verlorene Anfrage.
func pruefen(n nachricht) string {
	switch {
	case n.Name == "":
		return "Bitte trage deinen Namen ein."
	case len([]rune(n.Name)) > maxName:
		return "Der Name ist zu lang."
	case n.Email == "":
		return "Bitte trage eine E-Mail-Adresse ein, damit wir antworten können."
	case !plausibleAdresse(n.Email):
		return "Die E-Mail-Adresse sieht nicht richtig aus."
	case len(n.Email) > maxEmail:
		return "Die E-Mail-Adresse ist zu lang."
	case len([]rune(n.Betreff)) > maxBetreff:
		return "Der Betreff ist zu lang."
	case n.Text == "":
		return "Bitte schreibe noch eine Nachricht."
	case len([]rune(n.Text)) > maxText:
		return "Die Nachricht ist zu lang. Bitte fasse dich etwas kürzer."
	}
	return ""
}

// plausibleAdresse ist eine Formprüfung, kein Urteil. Strenger würde echte
// Adressen abweisen; was mehr verspricht, hält es nicht.
func plausibleAdresse(s string) bool {
	name, wirt, ok := strings.Cut(s, "@")
	if !ok || name == "" || wirt == "" {
		return false
	}
	if strings.Contains(wirt, "@") || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	punkt := strings.LastIndex(wirt, ".")
	return punkt > 0 && punkt < len(wirt)-1
}

// platzInDieserStunde zählt mit und sagt Nein, wenn die Stunde voll ist.
func platzInDieserStunde() bool {
	key := praefixZaehler + time.Now().UTC().Format("2006-01-02T15")
	n := 0
	if roh, ok, _ := plugin.Get(key); ok {
		n, _ = strconv.Atoi(roh)
	}
	if n >= stundenGrenze {
		return false
	}
	_ = plugin.Set(key, strconv.Itoa(n+1))
	altZaehlerWegwerfen(key)
	return true
}

// altZaehlerWegwerfen räumt die Zähler vergangener Stunden weg. Sie werden nie
// wieder gelesen, und ohne das wüchse der Speicher um eine Zeile pro Stunde.
func altZaehlerWegwerfen(aktuell string) {
	alle, err := plugin.List(praefixZaehler, 100)
	if err != nil {
		return
	}
	for k := range alle {
		if k != aktuell {
			_ = plugin.Delete(k)
		}
	}
}

func speichern(n nachricht) error {
	n.Zeit = time.Now().UTC().Format(time.RFC3339)
	n.Kennung = n.Zeit + "-" + zufallsende()

	roh, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := plugin.Set(praefixNachricht+n.Kennung, string(roh)); err != nil {
		return err
	}
	aufraeumen()
	return nil
}

// zufallsende trennt zwei Nachrichten aus derselben Sekunde.
func zufallsende() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// aufraeumen wirft die ältesten Nachrichten weg, wenn es zu viele werden.
func aufraeumen() {
	alle, err := plugin.List(praefixNachricht, 1000)
	if err != nil || len(alle) <= maxNachrichten {
		return
	}
	keys := make([]string, 0, len(alle))
	for k := range alle {
		keys = append(keys, k)
	}
	// Der Schlüssel beginnt mit dem Zeitstempel, also sortiert er chronologisch.
	sort.Strings(keys)
	for i := 0; i < len(keys)-maxNachrichten; i++ {
		_ = plugin.Delete(keys[i])
	}
	plugin.Logf("info", "%d alte Nachrichten entfernt", len(keys)-maxNachrichten)
}

// --- die Verwaltung ---------------------------------------------------------

func bildschirm(in plugin.AdminIn) (plugin.AdminOut, error) {
	if in.Method == "POST" {
		switch {
		case len(in.Form["loeschen"]) > 0:
			_ = plugin.Delete(praefixNachricht + in.Form["loeschen"][0])
			return plugin.AdminOut{Redirect: ".", Flash: "Nachricht gelöscht."}, nil
		case len(in.Form["gelesen"]) > 0:
			markieren(in.Form["gelesen"][0], true)
			return plugin.AdminOut{Redirect: "."}, nil
		case len(in.Form["ungelesen"]) > 0:
			markieren(in.Form["ungelesen"][0], false)
			return plugin.AdminOut{Redirect: "."}, nil
		}
	}

	alle, err := plugin.List(praefixNachricht, 1000)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	liste := lesen(alle)
	// Neueste zuerst: wer den Bildschirm öffnet, sucht fast immer die letzte.
	sort.Slice(liste, func(i, j int) bool { return liste[i].Kennung > liste[j].Kennung })

	// Die Tabelle zum Herunterladen. Sie steht hinter derselben Adresse mit
	// ?ansicht=csv, weil ein Plugin nur einen Bildschirm hat und die Abfrage
	// das einzige ist, womit es Ansichten auseinanderhält.
	if strings.Contains(in.Query, "ansicht=csv") {
		if len(liste) == 0 {
			return plugin.AdminOut{Redirect: ".", Flash: "Es gibt noch nichts auszugeben.", FlashError: true}, nil
		}
		return csvAusgabe(liste)
	}

	var b strings.Builder
	b.WriteString(navigation(ansichtNachrichten))
	ungelesen := 0
	for _, n := range liste {
		if !n.Gelesen {
			ungelesen++
		}
	}

	if len(liste) == 0 {
		b.WriteString(`<p class="empty">Noch keine Nachricht eingegangen. ` +
			`Setze <code>[[formular]]</code> in eine Seite, dann steht dort das Formular.</p>`)
		return plugin.AdminOut{Title: "Nachrichten", HTML: b.String()}, nil
	}

	fmt.Fprintf(&b, `<p>%d Nachrichten, davon %d ungelesen. `+
		`<a class="btn btn--sm" href="?ansicht=csv">Als Tabelle herunterladen</a></p>`,
		len(liste), ungelesen)

	for _, n := range liste {
		klasse := "card"
		if !n.Gelesen {
			klasse += " card--unread"
		}
		fmt.Fprintf(&b, `<article class="%s">`, klasse)
		fmt.Fprintf(&b, `<h3>%s</h3>`, ausgeben(betreffOder(n)))
		fmt.Fprintf(&b, `<p class="text-muted">%s &lt;<a href="mailto:%s">%s</a>&gt; · %s`,
			ausgeben(n.Name), ausgeben(n.Email), ausgeben(n.Email), ausgeben(kurzDatum(n.Zeit)))
		if n.Seite != "" {
			fmt.Fprintf(&b, ` · von <code>/%s</code>`, ausgeben(n.Seite))
		}
		if n.FormularName != "" {
			fmt.Fprintf(&b, ` · %s`, ausgeben(n.FormularName))
		}
		b.WriteString(`</p>`)
		if len(n.Felder) > 0 {
			// Ein zusammengestelltes Formular hat benannte Antworten. Als
			// Tabelle statt als Fliesstext: wer zwanzig Anfragen durchgeht,
			// sucht immer dasselbe Feld, und in einer Spalte findet er es.
			b.WriteString(`<table class="table"><tbody>`)
			for _, a := range n.Felder {
				fmt.Fprintf(&b, `<tr><th scope="row">%s</th><td>%s</td></tr>`,
					ausgeben(a.Beschriftung), ausgeben(a.Wert))
			}
			b.WriteString(`</tbody></table>`)
		} else {
			// Der Text kommt von aussen. Ausgegeben wird er maskiert, und die
			// Zeilenumbrüche macht das Stylesheet, nicht eingesetztes Markup.
			fmt.Fprintf(&b, `<pre class="message-body">%s</pre>`, ausgeben(n.Text))
		}

		b.WriteString(`<p class="table-actions">`)
		if n.Gelesen {
			knopf(&b, "ungelesen", n.Kennung, "Als ungelesen markieren", "")
		} else {
			knopf(&b, "gelesen", n.Kennung, "Als gelesen markieren", "")
		}
		knopf(&b, "loeschen", n.Kennung, "Löschen", "btn--danger")
		b.WriteString(`</p></article>`)
	}

	return plugin.AdminOut{Title: "Nachrichten", HTML: b.String()}, nil
}

// knopf zeichnet ein Formular mit einem Knopf. Es sendet an dieselbe Adresse
// zurück; den Sitzungsschlüssel setzt der Host ein, das Plugin sieht ihn nie.
// aktionsknopf ist ein Knopf ohne eigenes Formular: er sendet das ab, in dem er
// steht. Ein <form> in einem <form> ist ungültiges HTML, und ein Browser wirft
// das innere weg — der Knopf täte dann gar nichts.
func aktionsknopf(b *strings.Builder, name, wert, beschriftung, klasse string) {
	fmt.Fprintf(b, `<button type="submit" name="%s" value="%s" class="btn btn--sm %s">%s</button> `,
		name, ausgeben(wert), klasse, beschriftung)
}

func knopf(b *strings.Builder, name, wert, beschriftung, klasse string) {
	fmt.Fprintf(b, `<form method="POST" class="inline-form">`+
		`<input type="hidden" name="%s" value="%s">`+
		`<button type="submit" class="btn btn--sm %s">%s</button></form>`,
		name, ausgeben(wert), klasse, beschriftung)
}

func markieren(kennung string, gelesen bool) {
	key := praefixNachricht + kennung
	roh, ok, _ := plugin.Get(key)
	if !ok {
		return
	}
	var n nachricht
	if json.Unmarshal([]byte(roh), &n) != nil {
		return
	}
	n.Gelesen = gelesen
	if neu, err := json.Marshal(n); err == nil {
		_ = plugin.Set(key, string(neu))
	}
}

func lesen(roh map[string]string) []nachricht {
	out := make([]nachricht, 0, len(roh))
	for _, v := range roh {
		var n nachricht
		if json.Unmarshal([]byte(v), &n) == nil && n.Kennung != "" {
			out = append(out, n)
		}
	}
	return out
}

func betreffOder(n nachricht) string {
	if n.Betreff != "" {
		return n.Betreff
	}
	return "Ohne Betreff"
}

func kurzDatum(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006 15:04")
}

func ausgeben(s string) string { return html.EscapeString(s) }

func main() {}
