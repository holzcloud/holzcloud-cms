package admin

import (
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

func docWebsite() *domain.Website {
	return &domain.Website{
		ID: 1, Name: "Holzbau Schmidt",
		Street: "Werkstattweg 3", PostalCode: "8340", City: "Hinwil", Country: "CH",
		Phone: "044 123 45 67", ContactEmail: "post@example.ch",
		Currency: "CHF", VATNumber: "CHE-123.456.789 MWST",
		PaymentDetails: "Holzbau Schmidt AG\nCH93 0076 2011 6238 5295 7",
	}
}

func docOrder() *shop.Order {
	return &shop.Order{
		ID: 7, WebsiteID: 1, Number: "2026-0007", Currency: "CHF",
		Customer: shop.Customer{
			Email: "anna@example.ch", Name: "Anna Meier",
			Street: "Seestrasse 4", PostalCode: "8002", City: "Zürich", Country: "CH",
		},
		Totals: shop.Totals{
			ItemsGross: 9800, ItemsNet: 9065, ItemsTax: 735,
			ShippingGross: 1200, ShippingNet: 1110, ShippingTax: 90,
			TotalGross: 11000, TotalNet: 10175, TotalTax: 825,
		},
		Status: shop.OrderNew, PaymentMethod: shop.PayInvoice, PaymentStatus: shop.PaymentOpen,
		Items: []shop.OrderItem{{
			Title: "Hocker Brunni", Subtitle: "Esche geölt", SKU: "HB-49",
			Quantity: 2, TaxRate: money.RateStandard,
			UnitGross: 4900, LineNet: 9065, LineTax: 735, LineGross: 9800,
		}},
		CreatedAt: time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC),
	}
}

func TestInvoiceCarriesWhatTheLawWants(t *testing.T) {
	d := orderDocument(docWebsite(), docOrder(), true)

	if d.Seller.Name != "Holzbau Schmidt" || d.Seller.VAT != "CHE-123.456.789 MWST" {
		t.Errorf("the sender is incomplete: %+v", d.Seller)
	}
	if strings.Join(d.Buyer, " ") != "Anna Meier Seestrasse 4 8002 Zürich" {
		t.Errorf("die Anschrift stimmt nicht: %v", d.Buyer)
	}
	if d.Date != "03.08.2026" {
		t.Errorf("Datum = %q", d.Date)
	}
	if len(d.Lines) != 1 || d.Lines[0].SKU != "HB-49" || d.Lines[0].Quantity != 2 {
		t.Errorf("the row is wrong: %+v", d.Lines)
	}
	if d.Total != money.CurrencyFor("CHF").Format(11000) {
		t.Errorf("Gesamtbetrag = %q", d.Total)
	}
}

// The tax statement is the reason an invoice is an invoice. Goods and shipping
// carry the same rate here and therefore belong on one line.
func TestInvoiceBreaksTheTaxOutByRate(t *testing.T) {
	d := orderDocument(docWebsite(), docOrder(), true)

	if len(d.Taxes) != 1 {
		t.Fatalf("es wurden %d Steuerzeilen ausgewiesen: %+v", len(d.Taxes), d.Taxes)
	}
	tax := d.Taxes[0]
	if tax.Rate != "8.1 %" {
		t.Errorf("Satz = %q", tax.Rate)
	}
	c := money.CurrencyFor("CHF")
	// Ware und Versand zusammen: 735 + 90 Rappen.
	if tax.Tax != c.Format(825) {
		t.Errorf("Steuerbetrag = %q, erwartet %q", tax.Tax, c.Format(825))
	}
	if tax.Net != c.Format(10175) {
		t.Errorf("Nettobetrag = %q, erwartet %q", tax.Net, c.Format(10175))
	}
}

// Two rates on one invoice have to be stated separately — otherwise the
// recipient cannot deduct the input tax correctly.
func TestInvoiceKeepsTwoRatesApart(t *testing.T) {
	o := docOrder()
	o.Items = append(o.Items, shop.OrderItem{
		Title: "Holzöl", Quantity: 1, TaxRate: money.RateReduced,
		UnitGross: 2000, LineNet: 1949, LineTax: 51, LineGross: 2000,
	})

	d := orderDocument(docWebsite(), o, true)
	if len(d.Taxes) != 2 {
		t.Fatalf("es wurden %d Steuerzeilen ausgewiesen: %+v", len(d.Taxes), d.Taxes)
	}
	saetze := d.Taxes[0].Rate + " " + d.Taxes[1].Rate
	if !strings.Contains(saetze, "8.1 %") || !strings.Contains(saetze, "2.6 %") {
		t.Errorf("the sentences are wrong: %q", saetze)
	}
}

// The most important difference between the two papers. A delivery note lies in
// the parcel; a price on it is embarrassing with a present and simply wrong with
// a follow-up delivery.
func TestDeliveryNoteHasNoMoneyOnIt(t *testing.T) {
	d := orderDocument(docWebsite(), docOrder(), false)

	if d.Invoice {
		t.Fatal("the delivery note thinks it is an invoice")
	}
	if d.PaymentDetails != "" {
		t.Errorf("auf dem Lieferschein stehen Kontoangaben: %q", d.PaymentDetails)
	}
	if !strings.HasPrefix(d.Title, "Lieferschein") {
		t.Errorf("Titel = %q", d.Title)
	}
	// The amounts are filled in, but the template does not print them. What is
	// checked here is the switch: the rest hangs on .Invoice.
	if len(d.Lines) != 1 || d.Lines[0].Quantity != 2 {
		t.Errorf("die Ware fehlt: %+v", d.Lines)
	}
}

// A business under the turnover threshold states no VAT — and has to write the
// reason on the paper, not leave the line out.
func TestExemptInvoiceSaysWhy(t *testing.T) {
	o := docOrder()
	o.VATExempt = true

	d := orderDocument(docWebsite(), o, true)
	if len(d.Taxes) != 0 {
		t.Errorf("ein befreiter Betrieb weist Steuer aus: %+v", d.Taxes)
	}
	if !strings.Contains(d.TaxNote, "MWSTG") {
		t.Errorf("der Grund fehlt: %q", d.TaxNote)
	}
}

// An open invoice carries the bank details, a paid one does not — that would be
// an invitation to transfer the money a second time.
func TestPaidInvoiceDropsTheBankDetails(t *testing.T) {
	offen := orderDocument(docWebsite(), docOrder(), true)
	if !strings.Contains(offen.PaymentDetails, "CH93") {
		t.Errorf("der offenen Rechnung fehlen die Kontoangaben: %q", offen.PaymentDetails)
	}
	if !strings.Contains(offen.PaymentNote, "30 Tagen") {
		t.Errorf("Zahlungsfrist fehlt: %q", offen.PaymentNote)
	}

	o := docOrder()
	o.PaymentStatus = shop.PaymentPaid
	bezahlt := orderDocument(docWebsite(), o, true)
	if bezahlt.PaymentDetails != "" {
		t.Errorf("a paid invoice asks for payment again: %q", bezahlt.PaymentDetails)
	}
	if !strings.Contains(bezahlt.PaymentNote, "Bezahlt am 04.08.2026") {
		t.Errorf("das Zahlungsdatum fehlt: %q", bezahlt.PaymentNote)
	}
}

// The shipping rate does not stand in the order, it is computed back from two
// amounts. The rounding must not put it beside the mark.
func TestShippingRateIsRecovered(t *testing.T) {
	cases := []struct {
		net, tax money.Amount
		want     money.TaxRate
	}{
		{1110, 90, money.RateStandard}, // 8.1 %
		{1170, 30, money.RateReduced},  // 2.6 %
		{0, 0, money.RateStandard},     // kein Versand
	}
	for _, tc := range cases {
		o := docOrder()
		o.Totals.ShippingNet, o.Totals.ShippingTax = tc.net, tc.tax
		if got := shippingRate(o); got != tc.want {
			t.Errorf("Versand netto %d, Steuer %d: Satz %v, erwartet %v",
				tc.net, tc.tax, got, tc.want)
		}
	}
}

// An order in a currency the shop changed away from long ago is printed in its
// own.
func TestDocumentKeepsTheOrderCurrency(t *testing.T) {
	ws := docWebsite()
	ws.Currency = "EUR"

	d := orderDocument(ws, docOrder(), true)
	if !strings.Contains(d.Total, "CHF") {
		t.Errorf("the invoice was printed in the new currency: %q", d.Total)
	}
}
