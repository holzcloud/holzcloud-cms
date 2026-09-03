package admin

import (
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The two documents an order produces.
const (
	docInvoice  = "rechnung"
	docDelivery = "lieferschein"
)

// OrderDocData backs the printable invoice and delivery note.
//
// A page rather than a PDF. A PDF would mean a library, a font to embed and a
// second layout engine to keep in step with the one already here; a browser
// prints to PDF perfectly well and the operator already has one open. What this
// costs is control over the page break, which for a one-page order is nothing.
type OrderDocData struct {
	web.LayoutData
	// Invoice distinguishes the two documents. A delivery note is the same
	// paper without the money on it — the slip that goes in the box, where a
	// price would be wrong at best and awkward at worst.
	Invoice bool
	Title   string

	Order    *shop.Order
	Currency money.Currency

	// Seller is the business, as it has to appear on an invoice.
	Seller SellerLines
	// Buyer is the delivery address.
	Buyer []string

	Date string
	// Lines carry the money already formatted; a template must not do
	// arithmetic on prices.
	Lines []DocLine
	Taxes []DocTax

	ItemsGross string
	Shipping   string
	Total      string
	// TaxNote explains an invoice with no tax on it. Article 81 of the MWSTG
	// wants the reason on the paper, not the absence of a line.
	TaxNote string
	// PaymentNote is what the customer still has to do, if anything.
	PaymentNote string
	// PaymentDetails are the bank particulars, printed only where they help:
	// on an invoice that is not yet paid.
	PaymentDetails string
	ReturnPolicy   string
}

// SellerLines is the business address block.
type SellerLines struct {
	Name    string
	Address []string
	Email   string
	Phone   string
	VAT     string
	Website string
}

// DocLine is one row of the document.
type DocLine struct {
	Title    string
	Subtitle string
	SKU      string
	Quantity int
	Unit     string
	Line     string
	// Rate is the tax rate as a percentage, for the column an invoice with more
	// than one rate on it needs.
	Rate string
}

// DocTax is one rate's share, as an invoice has to break it out.
type DocTax struct {
	Rate  string
	Net   string
	Tax   string
	Gross string
}

// HandleOrderDocument renders the invoice or the delivery note for printing.
func (h *Handler) HandleOrderDocument(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if h.orders == nil {
		http.NotFound(w, r)
		return nil
	}

	kind := r.PathValue("kind")
	if kind != docInvoice && kind != docDelivery {
		http.NotFound(w, r)
		return nil
	}

	order, err := h.orders.ByNumber(r.Context(), ws.ID, r.PathValue("number"))
	if err != nil {
		return err
	}
	if order == nil {
		http.NotFound(w, r)
		return nil
	}

	data := orderDocument(ws, order, kind == docInvoice)
	data.LayoutData = web.NewLayoutData(r, h.sm, data.Title)
	data.CurrentWebsite = ws

	// Never cached: it carries a customer's address.
	w.Header().Set("Cache-Control", "no-store")
	return web.RenderAdmin(w, h.templates, r, "order_print", data)
}

// orderDocument builds the document from the order alone.
//
// From the order, not from the products: every line is what was sold, at the
// price it was sold for. A product renamed or repriced afterwards must not
// change a document about a sale that already happened.
func orderDocument(ws *domain.Website, o *shop.Order, invoice bool) OrderDocData {
	c := money.CurrencyFor(o.Currency)
	if o.Currency == "" {
		c = money.CurrencyFor(ws.Currency)
	}

	data := OrderDocData{
		Invoice:  invoice,
		Order:    o,
		Currency: c,
		Seller:   sellerLines(ws),
		Buyer:    buyerLines(o),
		Date:     o.CreatedAt.Format("02.01.2006"),
	}

	if invoice {
		data.Title = "Rechnung " + o.Number
	} else {
		data.Title = "Lieferschein " + o.Number
	}

	// Per rate, so the invoice can break the tax out the way it has to. A
	// consumer order and a business order both end up here; the stored line
	// already carries whichever way round it was calculated.
	type bucket struct{ net, tax, gross money.Amount }
	order := []money.TaxRate{}
	byRate := map[money.TaxRate]*bucket{}

	for _, it := range o.Items {
		data.Lines = append(data.Lines, DocLine{
			Title:    it.Title,
			Subtitle: it.Subtitle,
			SKU:      it.SKU,
			Quantity: it.Quantity,
			Unit:     c.Format(it.UnitGross),
			Line:     c.Format(it.LineGross),
			Rate:     it.TaxRate.String(),
		})

		b, seen := byRate[it.TaxRate]
		if !seen {
			b = &bucket{}
			byRate[it.TaxRate] = b
			order = append(order, it.TaxRate)
		}
		b.net += it.LineNet
		b.tax += it.LineTax
		b.gross += it.LineGross
	}

	// Delivery is taxed too, at whatever rate the order recorded for it.
	if o.Totals.ShippingGross > 0 && o.Totals.ShippingTax > 0 {
		rate := shippingRate(o)
		b, seen := byRate[rate]
		if !seen {
			b = &bucket{}
			byRate[rate] = b
			order = append(order, rate)
		}
		b.net += o.Totals.ShippingNet
		b.tax += o.Totals.ShippingTax
		b.gross += o.Totals.ShippingGross
	}

	for _, rate := range order {
		b := byRate[rate]
		data.Taxes = append(data.Taxes, DocTax{
			Rate:  rate.String(),
			Net:   c.Format(b.net),
			Tax:   c.Format(b.tax),
			Gross: c.Format(b.gross),
		})
	}

	data.ItemsGross = c.Format(o.Totals.ItemsGross)
	data.Shipping = c.Format(o.Totals.ShippingGross)
	data.Total = c.Format(o.Totals.TotalGross)

	if o.VATExempt {
		data.TaxNote = "Kein Ausweis der Mehrwertsteuer: " +
			"der Betrieb ist von der Steuerpflicht befreit (Art. 10 Abs. 2 MWSTG)."
		data.Taxes = nil
	}

	data.PaymentNote = documentPaymentNote(o)
	if invoice && o.PaymentStatus != shop.PaymentPaid && o.PaymentMethod != shop.PayPayrexx {
		data.PaymentDetails = ws.PaymentDetails
	}
	data.ReturnPolicy = o.ReturnPolicy
	return data
}

// shippingRate recovers the rate the delivery charge was taxed at.
//
// The order stores the amounts rather than the rate, so it is derived — the
// alternative would be a column that repeats what two other columns already
// say and can disagree with them.
func shippingRate(o *shop.Order) money.TaxRate {
	if o.Totals.ShippingNet <= 0 {
		return money.RateStandard
	}
	bp := money.TaxRate(int64(o.Totals.ShippingTax) * 10000 / int64(o.Totals.ShippingNet))
	// Rounding leaves the derived value a basis point either side of the real
	// one; snap it to the rate it obviously is.
	for _, known := range []money.TaxRate{money.RateStandard, money.RateReduced, money.RateLodging} {
		if bp >= known-5 && bp <= known+5 {
			return known
		}
	}
	return bp
}

func documentPaymentNote(o *shop.Order) string {
	if o.PaymentStatus == shop.PaymentPaid {
		return "Bezahlt am " + o.UpdatedAt.Format("02.01.2006") + " — " +
			paymentMethodLabel(o.PaymentMethod) + ". Vielen Dank."
	}
	switch o.PaymentMethod {
	case shop.PayPrepay:
		return "Zahlbar im Voraus. Bitte geben Sie als Zahlungszweck die " +
			"Bestellnummer " + o.Number + " an."
	case shop.PayPayrexx:
		return "Online-Zahlung, noch nicht bestätigt."
	default:
		return "Zahlbar innert 30 Tagen. Bitte geben Sie als Zahlungszweck die " +
			"Bestellnummer " + o.Number + " an."
	}
}

func sellerLines(ws *domain.Website) SellerLines {
	s := SellerLines{
		Name:  ws.Name,
		Email: ws.ContactEmail,
		Phone: ws.Phone,
		VAT:   ws.VATNumber,
	}
	if ws.Street != "" {
		s.Address = append(s.Address, ws.Street)
	}
	if line := strings.TrimSpace(ws.PostalCode + " " + ws.City); line != "" {
		s.Address = append(s.Address, line)
	}
	// Switzerland is left off: a Swiss invoice to a Swiss customer does not
	// print the country, and printing it looks like a form filled in by
	// someone who has never sent one.
	if ws.Country != "" && ws.Country != "CH" {
		s.Address = append(s.Address, ws.Country)
	}
	return s
}

func buyerLines(o *shop.Order) []string {
	var lines []string
	if o.Customer.Company != "" {
		lines = append(lines, o.Customer.Company)
	}
	lines = append(lines, o.Customer.Name, o.Customer.Street,
		strings.TrimSpace(o.Customer.PostalCode+" "+o.Customer.City))
	if o.Customer.Country != "" && o.Customer.Country != "CH" {
		lines = append(lines, o.Customer.Country)
	}
	return lines
}
