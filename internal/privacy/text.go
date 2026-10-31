package privacy

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// This file holds all German of the package. The text is content that an
// operator opens, completes and publishes, so it is listed in germanVoice in
// tools/english. Comments and identifiers stay English.

// TemplateNotice is the sentence that marks the page as unfinished. It opens
// the composed text, and the existing starter page opens with the same words.
const TemplateNotice = "Diese Seite ist eine Vorlage und noch nicht ausgefüllt."

// NoLegalAdvice is the closing sentence of the composed text.
const NoLegalAdvice = "Diese Vorlage ersetzt keine Rechtsberatung."

// Fillin is the visible marker for a fact only the operator knows.
const Fillin = "(eintragen)"

// HeadingCookies is the heading of the section that holds the cookie table and
// the table of processors.
const HeadingCookies = "Cookies und Auftragsverarbeiter"

// DraftTitle is the title of a template page offered as a draft next to an
// existing privacy page.
const DraftTitle = "Datenschutzerklärung (neue Vorlage)"

// DraftSlug is the slug of that draft.
const DraftSlug = "datenschutz-vorlage"

// noCookies is the sentence printed instead of a table when the register is
// empty for a website.
const noCookies = "Diese Website setzt keine Cookies."

// Headings of the blocks a website's functions add. The tests name them.
const (
	headingOrders   = "Bestellungen im Shop"
	headingPayment  = "Online-Zahlung"
	headingProtect  = "Geschützte Seiten"
	headingForms    = "Formulare"
	headingLog      = "Server-Protokoll"
	headingRights   = "Deine Rechte"
	headingResponsi = "Verantwortliche Stelle"
	headingGlance   = "Datenschutz auf einen Blick"
)

// cookieHeaderRow and processorHeaderRow are the header rows of the two tables.
const (
	cookieHeaderRow    = "| Name | Zweck | Laufzeit | Rechtsgrundlage |"
	processorHeaderRow = "| Empfänger | Aufgabe | Übermittelte Daten | Vertrag |"
)

// HasTemplateNotice reports whether the page still carries the sentence that
// marks it as an unfinished template.
func HasTemplateNotice(markdown string) bool {
	return strings.Contains(markdown, TemplateNotice)
}

// LifetimeText writes a duration the way a visitor reads it: whole days as
// "30 Tage", else whole hours as "12 Stunden", else minutes.
func LifetimeText(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d >= day && d%day == 0:
		n := int(d / day)
		if n == 1 {
			return "1 Tag"
		}
		return strconv.Itoa(n) + " Tage"
	case d >= time.Hour && d%time.Hour == 0:
		n := int(d / time.Hour)
		if n == 1 {
			return "1 Stunde"
		}
		return strconv.Itoa(n) + " Stunden"
	default:
		n := int(d / time.Minute)
		if n == 1 {
			return "1 Minute"
		}
		return strconv.Itoa(n) + " Minuten"
	}
}

// Compose writes the Markdown of a German privacy page for a website that uses
// the given features. The same features always give the same text.
//
// The text is a template: it opens by saying it is not finished, marks every
// fact the operator must supply with Fillin, and closes by saying it is no
// legal advice. It takes booleans and no operator string, carries no link and
// no raw HTML, and is rendered through page.RenderMarkdown like any page.
func Compose(f Features) string {
	f = f.normalised()
	var b strings.Builder

	b.WriteString("## " + headingGlance + "\n\n")
	b.WriteString("**" + TemplateNotice + "** Prüfe jede Angabe gegen das, was auf deiner Website tatsächlich geschieht, ")
	b.WriteString("ergänze alles, was mit " + Fillin + " markiert ist, und veröffentliche die Seite erst danach.\n\n")
	b.WriteString("Diese Website bindet nichts von fremden Servern ein und verwendet keine Analysewerkzeuge.\n\n")

	b.WriteString("## " + headingResponsi + "\n\n")
	b.WriteString("Name, Anschrift und Kontaktdaten der verantwortlichen Stelle im Sinne von Art. 4 Abs. 7 DSGVO " + Fillin + ".\n\n")

	writeLog(&b)
	writeCookiesAndProcessors(&b, f)

	b.WriteString("## " + headingRights + "\n\n")
	b.WriteString("Auskunft (Art. 15 DSGVO), Berichtigung (Art. 16), Löschung (Art. 17), ")
	b.WriteString("Einschränkung der Verarbeitung (Art. 18), Datenübertragbarkeit (Art. 20) und ")
	b.WriteString("Widerspruch (Art. 21). Außerdem besteht ein Beschwerderecht bei einer ")
	b.WriteString("Datenschutz-Aufsichtsbehörde.\n\n")

	b.WriteString("---\n\n")
	b.WriteString(NoLegalAdvice + "\n")
	return b.String()
}

// writeLog writes the section about the request log. Its numbers are the
// constants of package web, which a test ties to the journald drop-in in
// deploy/.
func writeLog(b *strings.Builder) {
	b.WriteString("## " + headingLog + "\n\n")
	b.WriteString("Beim Aufruf einer Seite schreibt der Server eine Zeile in sein Protokoll: ")
	b.WriteString("Zeitpunkt, aufgerufene Adresse sowie Status und Größe der Antwort. ")
	b.WriteString(fmt.Sprintf("Deine IP-Adresse wird vor dem Schreiben gekürzt, bei IPv4 auf das Netz /%d und bei IPv6 auf das Netz /%d: ",
		web.LogIPv4Prefix, web.LogIPv6Prefix))
	b.WriteString("Der Rest der Adresse wird auf null gesetzt und nicht gespeichert. ")
	b.WriteString(fmt.Sprintf("Das Protokoll wird %d Tage aufbewahrt. ", web.LogRetentionDays))
	b.WriteString("Gelöscht wird tageweise, weil das System-Journal täglich eine neue Datei beginnt und nur ganze Dateien entfernt. ")
	b.WriteString("Rechtsgrundlage ist Art. 6 Abs. 1 lit. f DSGVO (Betrieb und Sicherheit der Website).\n\n")
	b.WriteString("Meldet sich jemand im Verwaltungsbereich unter /admin falsch an oder wird eine Anmeldung über Single Sign-on abgelehnt, ")
	b.WriteString("schreibt der Server die vollständige IP-Adresse in das System-Protokoll. ")
	b.WriteString("Sie wird ebenso lange aufbewahrt und betrifft nur Personen, die an dieser Website arbeiten.\n\n")
}

// writeCookiesAndProcessors writes the section that holds the cookie table and
// the table of processors.
func writeCookiesAndProcessors(b *strings.Builder, f Features) {
	b.WriteString("## " + HeadingCookies + "\n\n")
	b.WriteString("Allein durch das Öffnen einer Seite setzt diese Website kein Cookie.")
	cookies := Cookies(f)
	if len(cookies) > 0 {
		b.WriteString(" Dein Browser speichert nur dann etwas, wenn du eine der unten genannten Handlungen ausführst. ")
		b.WriteString("Ein solches Cookie ist technisch erforderlich für das, was du angefragt hast ")
		b.WriteString("(§ 25 Abs. 2 Nr. 2 TDDDG, bei einem Vertrag zusätzlich Art. 6 Abs. 1 lit. b DSGVO). ")
		b.WriteString("Ein Cookie-Banner ist deshalb nicht nötig.")
	}
	b.WriteString("\n\n### Cookies\n\n")
	if len(cookies) == 0 {
		b.WriteString(noCookies + "\n\n")
	} else {
		b.WriteString(cookieHeaderRow + "\n| --- | --- | --- | --- |\n")
		for _, c := range cookies {
			b.WriteString("| `" + c.Name + "` | " + purpose(c.Trigger) + " | " + LifetimeText(c.Lifetime) + " | " + legalBasis(c.Trigger) + " |\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Der Verwaltungsbereich unter /admin verwendet eigene, technisch notwendige Cookies für die Anmeldung ")
	b.WriteString("und für den Schutz vor gefälschten Formularen. Sie werden nur dort gesetzt, wo Menschen an der Website arbeiten.\n\n")

	b.WriteString("### Auftragsverarbeiter und Empfänger\n\n")
	b.WriteString(processorHeaderRow + "\n| --- | --- | --- | --- |\n")
	b.WriteString("| Hosting-Anbieter " + Fillin + " | Betrieb des Servers | Verbindungsdaten wie im Abschnitt " + headingLog +
		" | Vertrag über die Auftragsverarbeitung nach Art. 28 DSGVO, wo der Server bei einem Dritten steht |\n")
	b.WriteString("\nErgänze jeden weiteren Empfänger, an den Daten dieser Website gehen " + Fillin + ".\n\n")
}

// purpose is the Zweck cell of a cookie. It begins with what the visitor does
// to cause the cookie.
func purpose(t Trigger) string {
	switch t {
	case TriggerBasket:
		return "Wird gesetzt, wenn du etwas in den Warenkorb legst. Merkt sich deinen Warenkorb, bis du bestellst oder er verfällt."
	case TriggerPriceView:
		return "Wird gesetzt, wenn du die Preisansicht wechselst. Merkt sich, ob dir Preise für Privatkunden oder für Geschäftskunden gezeigt werden."
	case TriggerUnlock:
		return "Wird gesetzt, wenn du das Passwort einer geschützten Seite eingibst. N steht für die Nummer der geschützten Seite. " +
			"Merkt sich, dass du die Seite freigeschaltet hast."
	}
	return ""
}

// legalBasis is the Rechtsgrundlage cell of a cookie.
func legalBasis(t Trigger) string {
	if t == TriggerBasket {
		return "§ 25 Abs. 2 Nr. 2 TDDDG; Art. 6 Abs. 1 lit. b DSGVO"
	}
	return "§ 25 Abs. 2 Nr. 2 TDDDG"
}
