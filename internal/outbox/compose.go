package outbox

import (
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// Shop is what composing a message needs to know about the website.
//
// Deliberately not domain.Website: this package has no business knowing about
// favicons and menu locations, and a narrow struct says exactly which fields a
// message can depend on.
type Shop struct {
	Name string
	// URL is the site's own address, for the link back to the order.
	URL string
	// OrderEmail is where the operator wants to hear about new orders, and the
	// address a customer's reply should reach. Empty means neither happens.
	OrderEmail string
	// PaymentDetails are the bank particulars for prepayment, as the operator
	// typed them.
	PaymentDetails string
	VATNumber      string
	Currency       money.Currency
}

// ForOrder builds the messages a new order produces.
//
// Two of them: one for the customer, who needs to know what they just committed
// to and what happens next, and one for the operator, who otherwise finds out
// by looking. Either can be absent — a customer always has an address, an
// operator only if they filled one in.
func ForOrder(s Shop, o *shop.Order) []Mail {
	var out []Mail
	id := o.ID

	if o.Customer.Email != "" {
		out = append(out, Mail{
			WebsiteID: o.WebsiteID,
			Kind:      KindOrderCustomer,
			OrderID:   &id,
			Recipient: o.Customer.Email,
			FromName:  s.Name,
			ReplyTo:   s.OrderEmail,
			Subject:   "Ihre Bestellung " + o.Number + " bei " + s.Name,
			Body:      customerBody(s, o),
		})
	}

	if s.OrderEmail != "" {
		out = append(out, Mail{
			WebsiteID: o.WebsiteID,
			Kind:      KindOrderOperator,
			OrderID:   &id,
			Recipient: s.OrderEmail,
			FromName:  s.Name,
			// So a reply goes to the customer, which is what an operator
			// reaching for "reply" means to do.
			ReplyTo: o.Customer.Email,
			Subject: "Neue Bestellung " + o.Number + " — " + o.Customer.Name,
			Body:    operatorBody(s, o),
		})
	}

	return out
}

// ForShipment is the message that goes out when an order is marked as sent.
func ForShipment(s Shop, o *shop.Order) []Mail {
	if o.Customer.Email == "" {
		return nil
	}
	id := o.ID

	var b strings.Builder
	b.WriteString("Guten Tag " + o.Customer.Name + "\n\n")
	b.WriteString("Ihre Bestellung " + o.Number + " ist unterwegs.\n\n")
	b.WriteString(lines(s, o))
	b.WriteString("\nLieferadresse\n")
	b.WriteString(address(o))
	b.WriteString("\n")
	b.WriteString(signature(s))

	return []Mail{{
		WebsiteID: o.WebsiteID,
		Kind:      KindOrderShipped,
		OrderID:   &id,
		Recipient: o.Customer.Email,
		FromName:  s.Name,
		ReplyTo:   s.OrderEmail,
		Subject:   "Ihre Bestellung " + o.Number + " ist unterwegs",
		Body:      b.String(),
	}}
}

func customerBody(s Shop, o *shop.Order) string {
	var b strings.Builder

	b.WriteString("Guten Tag " + o.Customer.Name + "\n\n")
	b.WriteString("Vielen Dank für Ihre Bestellung bei " + s.Name + ".\n")
	b.WriteString("Bestellnummer " + o.Number + "\n\n")

	b.WriteString(lines(s, o))
	b.WriteString("\n")
	b.WriteString(payment(s, o))

	b.WriteString("\nLieferadresse\n")
	b.WriteString(address(o))

	if o.Customer.Note != "" {
		b.WriteString("\nIhre Bemerkung\n" + o.Customer.Note + "\n")
	}
	if o.ReturnPolicy != "" {
		b.WriteString("\nRückgabe\n" + o.ReturnPolicy + "\n")
	}
	if s.URL != "" {
		b.WriteString("\nIhre Bestellung online:\n" + s.URL + "/bestellung/" + o.Number + "\n")
	}

	b.WriteString(signature(s))
	return b.String()
}

func operatorBody(s Shop, o *shop.Order) string {
	var b strings.Builder

	b.WriteString("Neue Bestellung " + o.Number + "\n")
	b.WriteString(o.CreatedAt.Format("02.01.2006, 15:04") + " Uhr\n\n")

	b.WriteString(lines(s, o))

	b.WriteString("\nKundin/Kunde\n")
	b.WriteString(o.Customer.Name + "\n")
	if o.Customer.Company != "" {
		b.WriteString(o.Customer.Company + "\n")
	}
	if o.Customer.VATNumber != "" {
		b.WriteString("UID " + o.Customer.VATNumber + "\n")
	}
	b.WriteString(o.Customer.Email + "\n")
	if o.Customer.Phone != "" {
		b.WriteString(o.Customer.Phone + "\n")
	}

	b.WriteString("\nLieferadresse\n")
	b.WriteString(address(o))

	if o.Customer.Note != "" {
		b.WriteString("\nBemerkung\n" + o.Customer.Note + "\n")
	}

	b.WriteString("\nZahlungsart: " + methodLabel(o.PaymentMethod) + "\n")
	b.WriteString("Zahlungsstatus: " + paymentStateLabel(o.PaymentStatus) + "\n")

	if o.Audience == shop.Business {
		b.WriteString("Preisanzeige: Gewerbekunde (netto)\n")
	}
	return b.String()
}

// lines is the ordered goods and the totals, in a shape that survives a
// proportional font — no columns held together by spaces, because a mail client
// will not use the font that was measured here.
func lines(s Shop, o *shop.Order) string {
	c := s.Currency
	if o.Currency != "" {
		c = money.CurrencyFor(o.Currency)
	}

	var b strings.Builder
	b.WriteString("Bestellte Ware\n")
	for _, it := range o.Items {
		b.WriteString("- " + itemQuantity(it) + it.Title)
		if it.Subtitle != "" {
			b.WriteString(", " + it.Subtitle)
		}
		b.WriteString(": " + c.Format(it.LineGross) + "\n")
	}

	b.WriteString("\nZwischensumme: " + c.Format(o.Totals.ItemsGross) + "\n")
	if o.Totals.ShippingGross == 0 {
		b.WriteString("Versand: kostenlos\n")
	} else {
		b.WriteString("Versand: " + c.Format(o.Totals.ShippingGross) + "\n")
	}
	if o.VATExempt {
		b.WriteString("MWST: keine (Kleinunternehmen)\n")
	} else if o.Totals.TotalTax > 0 {
		b.WriteString("davon MWST: " + c.Format(o.Totals.TotalTax) + "\n")
	}
	b.WriteString("Gesamt: " + c.Format(o.Totals.TotalGross) + "\n")
	return b.String()
}

func itemQuantity(it shop.OrderItem) string {
	if it.Quantity <= 1 {
		return ""
	}
	return itoa(it.Quantity) + " × "
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// payment is the paragraph that tells the customer whether anything is expected
// of them. For prepayment that is the whole point of the message.
func payment(s Shop, o *shop.Order) string {
	var b strings.Builder
	b.WriteString("Zahlung: " + methodLabel(o.PaymentMethod) + "\n")

	switch o.PaymentMethod {
	case shop.PayPrepay:
		if o.PaymentStatus == shop.PaymentPaid {
			b.WriteString("Der Betrag ist eingetroffen. Vielen Dank.\n")
			break
		}
		if s.PaymentDetails != "" {
			b.WriteString("\nBitte überweisen Sie den Betrag auf:\n")
			b.WriteString(strings.TrimSpace(s.PaymentDetails) + "\n")
			b.WriteString("\nBitte geben Sie als Zahlungszweck die Bestellnummer " +
				o.Number + " an.\n")
			b.WriteString("Wir liefern, sobald der Betrag eingetroffen ist.\n")
		} else {
			// The operator has not filled in their bank details. Saying so is
			// better than a confirmation that quietly asks for nothing.
			b.WriteString("Wir melden uns mit den Zahlungsangaben.\n")
		}

	case shop.PayPayrexx:
		switch o.PaymentStatus {
		case shop.PaymentPaid:
			b.WriteString("Die Zahlung ist eingegangen. Vielen Dank.\n")
		case shop.PaymentFailed:
			b.WriteString("Die Zahlung wurde nicht abgeschlossen. " +
				"Melden Sie sich bei uns, dann finden wir einen anderen Weg.\n")
		default:
			b.WriteString("Die Zahlung ist noch nicht bestätigt. " +
				"Sobald sie eintrifft, geht die Bestellung in Arbeit.\n")
		}

	default:
		if o.PaymentStatus == shop.PaymentPaid {
			b.WriteString("Die Rechnung ist beglichen. Vielen Dank.\n")
		} else {
			b.WriteString("Die Rechnung liegt der Sendung bei.\n")
		}
	}
	return b.String()
}

func address(o *shop.Order) string {
	var b strings.Builder
	if o.Customer.Company != "" {
		b.WriteString(o.Customer.Company + "\n")
	}
	b.WriteString(o.Customer.Name + "\n")
	b.WriteString(o.Customer.Street + "\n")
	b.WriteString(o.Customer.PostalCode + " " + o.Customer.City + "\n")
	if o.Customer.Country != "" && o.Customer.Country != "CH" {
		b.WriteString(o.Customer.Country + "\n")
	}
	return b.String()
}

func signature(s Shop) string {
	var b strings.Builder
	b.WriteString("\nFreundliche Grüsse\n")
	b.WriteString(s.Name + "\n")
	if s.URL != "" {
		b.WriteString(s.URL + "\n")
	}
	if s.VATNumber != "" {
		b.WriteString(s.VATNumber + "\n")
	}
	return b.String()
}

func methodLabel(method string) string {
	switch method {
	case shop.PayPrepay:
		return "Vorauskasse"
	case shop.PayPayrexx:
		return "Online-Zahlung"
	default:
		return "Rechnung"
	}
}

func paymentStateLabel(state string) string {
	switch state {
	case shop.PaymentPaid:
		return "bezahlt"
	case shop.PaymentFailed:
		return "gescheitert"
	case shop.PaymentRefunded:
		return "zurückerstattet"
	default:
		return "offen"
	}
}
