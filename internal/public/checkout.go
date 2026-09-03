package public

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

const (
	checkoutPath = "/kasse"
	thanksPath   = "/bestellung"
)

// SetOrderStore attaches the order book.
func (h *Handler) SetOrderStore(s *shop.OrderStore) { h.orders = s }

// SetOutbox attaches the outbox that carries the confirmations.
func (h *Handler) SetOutbox(o *outbox.Store) { h.outbox = o }

// HandleCheckout shows the order form and takes the order.
func (h *Handler) HandleCheckout(w http.ResponseWriter, r *http.Request) error {
	website, set, ok := h.shopRequest(w, r)
	if !ok || h.orders == nil {
		return nil
	}

	cart, err := h.carts.Get(r.Context(), website.ID, shop.TokenFrom(r))
	if err != nil {
		return err
	}
	// An empty basket has nothing to pay for; the basket page says so better
	// than an order form with no lines.
	if cart == nil || cart.Empty() {
		http.Redirect(w, r, cartPath, http.StatusSeeOther)
		return nil
	}

	audience := set.AudienceFor(r)
	values := checkoutValues{Country: "CH"}
	var errs map[string]string

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		values = readCheckout(r)
		errs = values.validate(audience, h.payrexxEnabled())
		if len(errs) == 0 {
			return h.placeOrder(w, r, website, set, audience, cart, values)
		}
	}

	return h.renderCheckout(w, r, website, set, audience, cart, values, errs, "")
}

// checkoutValues is the order form.
type checkoutValues struct {
	Email      string
	Name       string
	Company    string
	VATNumber  string
	Phone      string
	Street     string
	PostalCode string
	City       string
	Country    string
	Note       string
	Method     string
	// Accepted records the tick on the confirmation box.
	Accepted bool
}

func readCheckout(r *http.Request) checkoutValues {
	get := func(k string) string { return strings.TrimSpace(r.FormValue(k)) }
	v := checkoutValues{
		Email:      get("email"),
		Name:       get("name"),
		Company:    get("firma"),
		VATNumber:  get("uid"),
		Phone:      get("telefon"),
		Street:     get("strasse"),
		PostalCode: get("plz"),
		City:       get("ort"),
		Country:    get("land"),
		Note:       get("bemerkung"),
		Method:     get("zahlungsart"),
		Accepted:   r.FormValue("bestaetigt") != "",
	}
	if v.Country == "" {
		v.Country = "CH"
	}
	return v
}

// validate checks the form. online says whether the payment provider is set up;
// without it the online method is not a choice the customer can make, and a
// posted "payrexx" is a form that did not come from our page.
func (v checkoutValues) validate(audience shop.Audience, online bool) map[string]string {
	errs := map[string]string{}

	if !strings.Contains(v.Email, "@") || len(v.Email) < 5 {
		errs["email"] = "Bitte eine E-Mail-Adresse angeben, an die wir die Bestätigung schicken können."
	}
	if v.Name == "" {
		errs["name"] = "Bitte den Namen angeben."
	}
	if v.Street == "" {
		errs["strasse"] = "Bitte die Strasse angeben."
	}
	// Swiss postal codes are four digits. Checked as a shape rather than
	// against a list: a list would have to be maintained, and a wrong code
	// costs a delivery attempt, not a legal problem.
	if v.Country == "CH" && !isFourDigits(v.PostalCode) {
		errs["plz"] = "Eine Schweizer Postleitzahl hat vier Ziffern."
	} else if v.PostalCode == "" {
		errs["plz"] = "Bitte die Postleitzahl angeben."
	}
	if v.City == "" {
		errs["ort"] = "Bitte den Ort angeben."
	}
	if audience == shop.Business && v.Company == "" {
		errs["firma"] = "Bitte den Firmennamen angeben."
	}
	switch {
	case v.Method == shop.PayInvoice, v.Method == shop.PayPrepay:
	case v.Method == shop.PayPayrexx && online:
	default:
		errs["zahlungsart"] = "Bitte eine Zahlungsart wählen."
	}
	// The Button solution: the customer confirms that this order costs money.
	// Not required by Swiss law the way it is in the EU, but the moment of
	// consent is worth having recorded either way.
	if !v.Accepted {
		errs["bestaetigt"] = "Bitte bestätigen Sie die kostenpflichtige Bestellung."
	}
	return errs
}

func isFourDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// placeOrder writes the order and sends the customer to the confirmation.
func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request, website *domain.Website,
	set shop.Settings, audience shop.Audience, cart *shop.Cart, values checkoutValues) error {

	order, err := h.orders.Place(r.Context(), website.ID, set, audience, cart, shop.Customer{
		Email: values.Email, Name: values.Name, Company: values.Company,
		VATNumber: values.VATNumber, Phone: values.Phone,
		Street: values.Street, PostalCode: values.PostalCode,
		City: values.City, Country: values.Country, Note: values.Note,
	}, values.Method)

	if err != nil {
		var soldOut shop.ErrOutOfStock
		if ok := asOutOfStock(err, &soldOut); ok {
			// Back to the form with the reason, not a bare error page: the
			// basket is still there and one line has to change.
			return h.renderCheckout(w, r, website, set, audience, cart, values, nil,
				soldOut.Error()+". Bitte passen Sie den Warenkorb an.")
		}
		return fmt.Errorf("place order: %w", err)
	}

	// The basket is gone; keeping the cookie would show an empty one forever.
	shop.ClearCartCookie(w, h.secure)

	// Written now, sent by the background run. If it cannot be written the
	// order still stands: an order nobody was told about is a problem the
	// operator can see in the admin, an order that was refused because the mail
	// failed is a sale that did not happen.
	h.announceOrder(r, website, order)

	// Online payment goes to the provider instead of straight to the
	// confirmation. The order already exists, so a customer who abandons the
	// payment page still leaves a record behind rather than nothing.
	if values.Method == shop.PayPayrexx && h.payrexxEnabled() {
		return h.startPayment(w, r, website, order)
	}

	// Redirect rather than render, so a reload does not look like a second
	// order — and the number is in the address, which is what people bookmark.
	http.Redirect(w, r, thanksPath+"/"+order.Number, http.StatusSeeOther)
	return nil
}

// announceOrder puts the confirmation and the notification in the outbox.
//
// Never fatal. Everything it does is a nice-to-have compared with the order,
// which at this point already exists and is already paid for or not.
func (h *Handler) announceOrder(r *http.Request, website *domain.Website, order *shop.Order) {
	if h.outbox == nil {
		return
	}
	for _, m := range outbox.ForOrder(outboxShop(h.canonicalBase(r, website), website), order) {
		if _, err := h.outbox.Queue(r.Context(), m); err != nil {
			slog.Error("order mail not queued", "order", order.Number, "kind", m.Kind, "err", err)
		}
	}
}

// outboxShop is the website as a message needs to see it.
func outboxShop(base string, w *domain.Website) outbox.Shop {
	return outbox.Shop{
		Name:           w.Name,
		URL:            base,
		OrderEmail:     w.OrderEmail,
		PaymentDetails: w.PaymentDetails,
		VATNumber:      w.VATNumber,
		Currency:       money.CurrencyFor(w.Currency),
	}
}

func asOutOfStock(err error, target *shop.ErrOutOfStock) bool {
	if e, ok := err.(shop.ErrOutOfStock); ok {
		*target = e
		return true
	}
	return false
}

// renderCheckout draws the order form.
func (h *Handler) renderCheckout(w http.ResponseWriter, r *http.Request, website *domain.Website,
	set shop.Settings, audience shop.Audience, cart *shop.Cart,
	values checkoutValues, errs map[string]string, notice string) error {

	totals := cart.Total(set, audience)

	data := tmpl.PageData{
		Site:  h.siteData(r, website),
		Page:  tmpl.PageContent{Title: "Kasse", Slug: "kasse"},
		Menus: h.loadMenus(r, website.ID),
		Shop:  h.shopData(r, website),
		Cart:  h.cartView(r.Context(), website, set, audience, cart, totals),
		Checkout: tmpl.CheckoutData{
			Action:       checkoutPath,
			Errors:       errs,
			Notice:       notice,
			Business:     audience == shop.Business,
			ReturnPolicy: set.ReturnPolicy,
			Methods:      h.paymentMethods(),
			Values: map[string]string{
				"email": values.Email, "name": values.Name, "firma": values.Company,
				"uid": values.VATNumber, "telefon": values.Phone,
				"strasse": values.Street, "plz": values.PostalCode,
				"ort": values.City, "land": values.Country,
				"bemerkung": values.Note, "zahlungsart": values.Method,
			},
			Accepted: values.Accepted,
		},
	}
	data.Site.Snippets = h.loadSnippets(r, website.ID).HTML
	data.Meta = metaData(data.Site, nil, checkoutPath)
	data.Meta.NoIndex = true

	content, err := h.loader.RenderPage(r.Context(), website.ID, "checkout.html", data)
	if err != nil {
		return fmt.Errorf("render checkout: %w", err)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write(content)
	return err
}

// paymentMethods lists what the customer may choose.
//
// Online payment comes first when it is available: it is the one that finishes
// the order there and then, and the two others leave the operator waiting.
func (h *Handler) paymentMethods() []tmpl.PaymentMethod {
	var methods []tmpl.PaymentMethod
	if h.payrexxEnabled() {
		methods = append(methods, tmpl.PaymentMethod{
			Value: shop.PayPayrexx,
			Label: "Online bezahlen",
			// The methods actually offered are whatever the operator has
			// switched on in their Payrexx account, so this names the usual
			// ones without promising any single one of them.
			Note: "TWINT, Karte oder PostFinance — Sie werden zur Zahlung weitergeleitet.",
		})
	}
	return append(methods,
		tmpl.PaymentMethod{Value: shop.PayInvoice, Label: "Rechnung",
			Note: "Sie erhalten die Rechnung mit der Ware."},
		tmpl.PaymentMethod{Value: shop.PayPrepay, Label: "Vorauskasse",
			Note: "Wir liefern, sobald der Betrag eingetroffen ist."},
	)
}

// HandleOrderConfirmation shows a placed order.
//
// Reachable by its number alone, with no login. The number is sequential and
// therefore guessable, so nothing here may be worth guessing: the page shows
// what was ordered and where it goes, which the person who placed it already
// knows — and no payment details, because there are none to show.
func (h *Handler) HandleOrderConfirmation(w http.ResponseWriter, r *http.Request) error {
	website, set, ok := h.shopRequest(w, r)
	if !ok || h.orders == nil {
		return nil
	}

	order, err := h.orders.ByNumber(r.Context(), website.ID, r.PathValue("number"))
	if err != nil {
		return err
	}
	if order == nil {
		return h.serve404(w, r, website)
	}

	data := tmpl.PageData{
		Site:  h.siteData(r, website),
		Page:  tmpl.PageContent{Title: "Bestellung " + order.Number, Slug: "bestellung"},
		Menus: h.loadMenus(r, website.ID),
		Shop:  h.shopData(r, website),
		Order: orderView(set, order),
	}
	data.Site.Snippets = h.loadSnippets(r, website.ID).HTML
	data.Meta = metaData(data.Site, nil, thanksPath+"/"+order.Number)
	data.Meta.NoIndex = true

	content, err := h.loader.RenderPage(r.Context(), website.ID, "order.html", data)
	if err != nil {
		return fmt.Errorf("render order: %w", err)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write(content)
	return err
}

// orderView maps an order onto what a template renders.
func orderView(set shop.Settings, o *shop.Order) tmpl.OrderData {
	c := set.Currency
	if o.Currency != "" {
		c = currencyOf(o.Currency)
	}

	view := tmpl.OrderData{
		Number:  o.Number,
		Email:   o.Customer.Email,
		Name:    o.Customer.Name,
		Company: o.Customer.Company,
		Address: strings.TrimSpace(o.Customer.Street + ", " +
			o.Customer.PostalCode + " " + o.Customer.City + ", " + o.Customer.Country),
		Note:           o.Customer.Note,
		Status:         o.Status,
		PaymentLabel:   paymentLabel(o.PaymentMethod),
		PaymentNote:    paymentNote(o),
		PaymentPending: o.PaymentStatus == shop.PaymentOpen || o.PaymentStatus == shop.PaymentFailed,
		ReturnPolicy:   o.ReturnPolicy,
		Totals: tmpl.CartTotals{
			Items:        c.Format(o.Totals.ItemsGross),
			Shipping:     c.Format(o.Totals.ShippingGross),
			ShippingFree: o.Totals.ShippingGross == 0,
			Total:        c.Format(o.Totals.TotalGross),
		},
	}
	if o.VATExempt {
		view.Totals.TaxNote = "keine MWST (Kleinunternehmen)"
	} else if o.Totals.TotalTax > 0 {
		view.Totals.TaxLines = []tmpl.CartTaxLine{{
			Label: "MWST",
			Net:   c.Format(o.Totals.TotalNet),
			Tax:   c.Format(o.Totals.TotalTax),
		}}
	}

	for _, it := range o.Items {
		view.Lines = append(view.Lines, tmpl.CartLine{
			Title:     it.Title,
			Subtitle:  it.Subtitle,
			Quantity:  it.Quantity,
			UnitPrice: c.Format(it.UnitGross),
			LinePrice: c.Format(it.LineGross),
			Available: true,
		})
	}
	return view
}

// currencyOf resolves a stored currency code.
//
// An order carries its own code: a shop that switches currency later must not
// reprint an old order in the new one.
func currencyOf(code string) money.Currency { return money.CurrencyFor(code) }

// paymentNote tells the customer where their payment stands.
//
// Written for the person who just pressed the button and wants to know whether
// they still owe something. "Offen" on its own would not answer that — with
// online payment it means the payment did not go through and they should try
// again, with an invoice it means nothing is expected of them yet.
func paymentNote(o *shop.Order) string {
	switch o.PaymentMethod {
	case shop.PayPayrexx:
		switch o.PaymentStatus {
		case shop.PaymentPaid:
			return "Die Zahlung ist eingegangen. Vielen Dank."
		case shop.PaymentFailed:
			return "Die Zahlung wurde nicht abgeschlossen. Melden Sie sich bei uns, dann finden wir einen anderen Weg."
		case shop.PaymentRefunded:
			return "Der Betrag wurde zurückerstattet."
		default:
			return "Die Zahlung ist noch nicht bestätigt. Sobald sie eintrifft, geht die Bestellung in Arbeit."
		}
	case shop.PayPrepay:
		if o.PaymentStatus == shop.PaymentPaid {
			return "Die Zahlung ist eingegangen. Vielen Dank."
		}
		return "Wir schicken Ihnen die Zahlungsangaben per E-Mail und liefern, sobald der Betrag eingetroffen ist."
	default:
		if o.PaymentStatus == shop.PaymentPaid {
			return "Die Rechnung ist beglichen. Vielen Dank."
		}
		return "Die Rechnung liegt der Sendung bei."
	}
}

func paymentLabel(method string) string {
	switch method {
	case shop.PayPrepay:
		return "Vorauskasse"
	case shop.PayPayrexx:
		return "Online-Zahlung"
	default:
		return "Rechnung"
	}
}
