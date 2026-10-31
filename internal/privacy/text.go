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
	// Fixed order: orders, payment, protected pages, forms.
	if f.Shop {
		writeOrders(&b)
	}
	if f.Payment {
		writePayment(&b)
	}
	if f.ProtectedPages {
		writeProtectedPages(&b)
	}
	if f.Forms {
		writeForms(&b)
	}

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
	if f.Payment {
		b.WriteString("| Zahlungsanbieter Payrexx, Name und Anschrift " + Fillin + " | Abwicklung der Online-Zahlung | Betrag, Währung, Bestellnummer mit dem Namen der Website, " +
			"Name, E-Mail-Adresse, Straße, Postleitzahl, Ort und Land | Vertrag über die Auftragsverarbeitung nach Art. 28 DSGVO nötig, Vertragsdatum " + Fillin + " |\n")
	}
	b.WriteString("\nErgänze jeden weiteren Empfänger, an den Daten dieser Website gehen " + Fillin + ".\n\n")
}

// writeOrders writes the block for a website with a shop.
func writeOrders(b *strings.Builder) {
	b.WriteString("## " + headingOrders + "\n\n")
	b.WriteString("Wenn du im Shop bestellst, erfasst die Kasse deinen Namen, gegebenenfalls Firma, Umsatzsteuer-Identifikationsnummer und Telefonnummer, ")
	b.WriteString("deine Anschrift, deine E-Mail-Adresse, eine Anmerkung, die bestellten Artikel sowie deine Wahl von Lieferung und Zahlung. ")
	b.WriteString("Die Bestellung wird in der Datenbank dieser Website gespeichert und im Verwaltungsbereich angezeigt. ")
	b.WriteString("Rechtsgrundlage ist Art. 6 Abs. 1 lit. b DSGVO. Das Cookie für deinen Warenkorb steht in der Tabelle oben. ")
	b.WriteString("Eine Bestätigung der Bestellung geht per E-Mail über das Postfach, das der Betreiber dafür eingerichtet hat.\n\n")
	b.WriteString("Bestellungen werden " + Fillin + " aufbewahrt. Für Buchungsbelege können gesetzliche Aufbewahrungspflichten gelten.\n\n")
}

// writePayment writes the block for a shop with a payment provider. What it
// lists is what startPayment in internal/public/payment.go sends.
func writePayment(b *strings.Builder) {
	b.WriteString("## " + headingPayment + "\n\n")
	b.WriteString("Bezahlst du online, leitet dich die Website zur Zahlungsseite des Anbieters Payrexx weiter. ")
	b.WriteString("Payrexx erhält den Betrag, die Währung und die Bestellnummer mit dem Namen der Website als Verwendungszweck ")
	b.WriteString("sowie die Kundendaten deiner Bestellung: Name, E-Mail-Adresse, Straße, Postleitzahl, Ort und Land. ")
	b.WriteString("Auf der Zahlungsseite gilt die Datenschutzerklärung des Anbieters. Nenne hier, wo du sie verlinkst " + Fillin + ".")
	b.WriteString("Rechtsgrundlage ist Art. 6 Abs. 1 lit. b DSGVO.\n\n")
	b.WriteString("Mit Payrexx brauchst du einen Vertrag über die Auftragsverarbeitung nach Art. 28 DSGVO. ")
	b.WriteString("Name und Anschrift des Anbieters " + Fillin + ", Datum des Vertrags " + Fillin + ".\n\n")
}

// writeProtectedPages writes the block for a website with password-protected pages.
func writeProtectedPages(b *strings.Builder) {
	b.WriteString("## " + headingProtect + "\n\n")
	b.WriteString("Einzelne Seiten dieser Website sind durch ein Passwort geschützt. Gibst du das Passwort ein, setzt die Website das Cookie für geschützte Seiten aus der Tabelle oben. ")
	b.WriteString("Es enthält eine signierte Seitennummer und eine Ablaufzeit, aber keine persönlichen Daten. ")
	b.WriteString("Außer dem Eintrag im Server-Protokoll wird nicht gespeichert, wer das Passwort eingegeben hat.\n\n")
}

// writeForms writes the block for a website with forms.
func writeForms(b *strings.Builder) {
	b.WriteString("## " + headingForms + "\n\n")
	b.WriteString("Was du in ein Formular dieser Website schreibst (Name, E-Mail-Adresse, Betreff, Nachricht und die Felder, die der Betreiber ergänzt hat), ")
	b.WriteString("wird in der Datenbank der Website gespeichert und im Verwaltungsbereich angezeigt. ")
	b.WriteString("Deine IP-Adresse und die Kennung deines Browsers werden dabei nicht mitgespeichert. ")
	b.WriteString("Der Spam-Schutz arbeitet mit einem versteckten Feld, einem signierten Zeitstempel und einer Grenze für Nachrichten pro Stunde und setzt kein Cookie. ")
	b.WriteString("Ist ein Mailserver eingerichtet, geht eine Benachrichtigung per E-Mail an den Betreiber. ")
	b.WriteString("Rechtsgrundlage ist Art. 6 Abs. 1 lit. b und f DSGVO.\n\n")
	b.WriteString("Anfragen werden " + Fillin + " aufbewahrt.\n\n")
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
