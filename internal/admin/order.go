package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SetOrderStore attaches the order book.
func (h *Handler) SetOrderStore(s *shop.OrderStore) { h.orders = s }

// OrderListData backs the order list.
type OrderListData struct {
	web.LayoutData
	web.FormState
	Orders   []*shop.Order
	Currency money.Currency
}

// OrderDetailData backs one order.
type OrderDetailData struct {
	web.LayoutData
	web.FormState
	Order    *shop.Order
	Currency money.Currency
	Statuses []struct{ Value, Label string }
	// PaymentMethod and PaymentState are the stored codes in words.
	PaymentMethod string
	PaymentState  string
	// CanRecheck offers to ask the payment provider again. Shown only where
	// that can actually help: an online payment that is still open, on an
	// installation that has the keys.
	CanRecheck bool
	// Mails is what was written to the outbox for this order, and where each
	// one stands. Showing "sent" for something that only sits in a queue would
	// be the one lie this page must not tell.
	Mails []MailLine
}

// MailLine is one outbox entry as the order page shows it.
type MailLine struct {
	ID        int64
	What      string
	Recipient string
	State     string
	// Detail carries the reason a message did not go out, or is empty.
	Detail string
	// CanRetry is true where trying again could plausibly help.
	CanRetry bool
	When     string
}

func mailKindLabel(kind string) string {
	switch kind {
	case outbox.KindOrderCustomer:
		return "Bestätigung an die Kundschaft"
	case outbox.KindOrderOperator:
		return "Meldung an den Betrieb"
	case outbox.KindOrderShipped:
		return "Versandmeldung"
	}
	return kind
}

func mailStateLabel(m outbox.Mail) string {
	switch m.Status {
	case outbox.StatusSent:
		return "verschickt"
	case outbox.StatusFailed:
		return "aufgegeben"
	default:
		if m.Attempts > 0 {
			return "wartet auf den nächsten Versuch"
		}
		return "wartet auf Versand"
	}
}

// mailLines maps the outbox onto the order page.
func mailLines(mails []outbox.Mail) []MailLine {
	out := make([]MailLine, 0, len(mails))
	for _, m := range mails {
		line := MailLine{
			ID:        m.ID,
			What:      mailKindLabel(m.Kind),
			Recipient: m.Recipient,
			State:     mailStateLabel(m),
			Detail:    m.LastError,
			CanRetry:  m.Status != outbox.StatusSent,
		}
		if m.SentAt != nil {
			line.When = m.SentAt.Format("02.01.2006, 15:04")
		}
		out = append(out, line)
	}
	return out
}

// SetPayments attaches the payment provider, so the operator can ask it again
// about an order whose confirmation never arrived.
func (h *Handler) SetPayments(c *payrexx.Client) { h.payments = c }

// SetOutbox attaches the outbox, so the order page can show what was sent.
func (h *Handler) SetOutbox(o *outbox.Store) { h.outbox = o }

// paymentMethodLabel names a stored method.
func paymentMethodLabel(code string) string {
	switch code {
	case shop.PayPrepay:
		return "Vorauskasse"
	case shop.PayPayrexx:
		return "Online-Zahlung (Payrexx)"
	case shop.PayInvoice:
		return "Rechnung"
	}
	return code
}

// paymentStateLabel names a stored payment state.
func paymentStateLabel(code string) string {
	switch code {
	case shop.PaymentPaid:
		return "bezahlt"
	case shop.PaymentFailed:
		return "gescheitert"
	case shop.PaymentRefunded:
		return "zurückerstattet"
	case shop.PaymentOpen:
		return "offen"
	}
	return code
}

// orderStatuses are the moves an operator can make, in the order they happen.
var orderStatuses = []struct{ Value, Label string }{
	{shop.OrderNew, "Neu"},
	{shop.OrderPaid, "Bezahlt"},
	{shop.OrderShipped, "Versandt"},
	{shop.OrderCancelled, "Storniert"},
}

// HandleOrderList shows a website's orders, newest first.
func (h *Handler) HandleOrderList(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if h.orders == nil {
		http.NotFound(w, r)
		return nil
	}

	// A shop's order book is read, not scrolled: two hundred rows is a year for
	// a workshop and one afternoon for nobody.
	orders, err := h.orders.List(r.Context(), ws.ID, 200)
	if err != nil {
		return err
	}

	data := OrderListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Bestellungen – "+ws.Name),
		FormState:  web.NewFormState(),
		Orders:     orders,
		Currency:   money.CurrencyFor(ws.Currency),
	}
	data.ActiveNav = "orders"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "order_list", data)
}

// HandleOrderDetail shows one order and takes a status change.
func (h *Handler) HandleOrderDetail(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if h.orders == nil {
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

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		switch aktion := r.FormValue("aktion"); {
		case aktion == "zahlung-pruefen":
			h.recheckPayment(r, order)

		case aktion == "mail-erneut":
			h.retryMail(r, r.FormValue("mail_id"))

		default:
			status := r.FormValue("status")
			if err := h.orders.SetStatus(r.Context(), ws.ID, order.ID, status); err != nil {
				web.SetFlashError(h.sm, r.Context(), err.Error())
				break
			}
			web.SetFlashSuccess(h.sm, r.Context(), "Status geändert")
			// Auf "versandt" gehört eine Nachricht an die Kundschaft. Nur beim
			// Wechsel: wer den Status zweimal speichert, soll nicht zweimal
			// melden, dass dasselbe Paket unterwegs ist.
			if status == shop.OrderShipped && order.Status != shop.OrderShipped {
				h.announceShipment(r, ws, order)
			}
		}
		return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(ws.ID, 10)+
			"/bestellungen/"+order.Number)
	}

	var mails []outbox.Mail
	if h.outbox != nil {
		mails, err = h.outbox.ForOrder(r.Context(), order.ID)
		if err != nil {
			return err
		}
	}

	currency := money.CurrencyFor(order.Currency)
	data := OrderDetailData{
		LayoutData: web.NewLayoutData(r, h.sm, "Bestellung "+order.Number+" – "+ws.Name),
		FormState:  web.NewFormState(),
		Order:      order,
		Currency:   currency,
		Statuses:   orderStatuses,

		PaymentMethod: paymentMethodLabel(order.PaymentMethod),
		PaymentState:  paymentStateLabel(order.PaymentStatus),
		CanRecheck: order.PaymentMethod == shop.PayPayrexx &&
			order.PaymentStatus == shop.PaymentOpen &&
			order.PaymentReference != "" && h.payments.Configured(),
		Mails: mailLines(mails),
	}
	data.ActiveNav = "orders"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "order_detail", data)
}

// recheckPayment asks the provider once more about an order.
//
// The recourse for the case both automatic paths missed: the customer closed
// the tab on the way back and the notification never arrived either. Without
// it the money is in the Payrexx account and the order sits at "offen" with
// nothing the operator can do but change the status by hand and hope.
//
// Only ever reads. It cannot make an unpaid order paid, because the answer
// comes from Payrexx, not from the button.
func (h *Handler) recheckPayment(r *http.Request, order *shop.Order) {
	if !h.payments.Configured() {
		web.SetFlashError(h.sm, r.Context(), "Für diese Installation ist kein Zahlungsanbieter eingerichtet.")
		return
	}
	if order.PaymentMethod != shop.PayPayrexx || order.PaymentReference == "" {
		web.SetFlashError(h.sm, r.Context(), "Diese Bestellung wurde nicht online bezahlt.")
		return
	}

	id, err := strconv.ParseInt(order.PaymentReference, 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Zahlungsreferenz ist unlesbar.")
		return
	}

	gw, err := h.payments.GetGateway(r.Context(), id)
	if err != nil {
		slog.Error("payment recheck failed", "order", order.Number, "err", err)
		web.SetFlashError(h.sm, r.Context(), "Der Zahlungsanbieter war nicht erreichbar.")
		return
	}

	switch {
	case gw.Paid():
		// The amount is checked here for the same reason as on the public
		// side: a gateway is only evidence for the amount it names.
		if gw.Amount != int64(order.Totals.TotalGross) {
			web.SetFlashError(h.sm, r.Context(),
				"Der beim Anbieter verbuchte Betrag stimmt nicht mit der Bestellung überein.")
			return
		}
		if err := h.orders.SetPayment(r.Context(), order.ID, shop.PaymentPaid, order.PaymentReference); err != nil {
			web.SetFlashError(h.sm, r.Context(), err.Error())
			return
		}
		web.SetFlashSuccess(h.sm, r.Context(), "Die Zahlung ist eingegangen.")

	case gw.Failed():
		if err := h.orders.SetPayment(r.Context(), order.ID, shop.PaymentFailed, order.PaymentReference); err != nil {
			web.SetFlashError(h.sm, r.Context(), err.Error())
			return
		}
		web.SetFlashSuccess(h.sm, r.Context(), "Die Zahlung wurde abgebrochen oder abgelehnt.")

	default:
		web.SetFlashSuccess(h.sm, r.Context(), "Beim Anbieter ist noch keine Zahlung verbucht.")
	}
}

// announceShipment tells the customer their order is on its way.
func (h *Handler) announceShipment(r *http.Request, ws *domain.Website, order *shop.Order) {
	if h.outbox == nil {
		return
	}
	base := ""
	if h.resolver != nil {
		base = h.resolver.CanonicalBase(r.Context(), ws, r.Host)
	}
	shopInfo := outbox.Shop{
		Name:           ws.Name,
		URL:            base,
		OrderEmail:     ws.OrderEmail,
		PaymentDetails: ws.PaymentDetails,
		VATNumber:      ws.VATNumber,
		Currency:       money.CurrencyFor(ws.Currency),
	}
	for _, m := range outbox.ForShipment(shopInfo, order) {
		if _, err := h.outbox.Queue(r.Context(), m); err != nil {
			slog.Error("shipment mail not queued", "order", order.Number, "err", err)
			web.SetFlashError(h.sm, r.Context(), "Die Versandmeldung konnte nicht abgelegt werden.")
		}
	}
}

// retryMail puts a message that did not go out back in the queue.
//
// Only worth offering after the operator has fixed something — a typo in the
// address, a mail account that had expired. It does not send anything itself;
// the background run does that, and this only says "try again".
func (h *Handler) retryMail(r *http.Request, raw string) {
	if h.outbox == nil {
		return
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Unbekannte Nachricht.")
		return
	}
	if err := h.outbox.Retry(r.Context(), id); err != nil {
		web.SetFlashError(h.sm, r.Context(), err.Error())
		return
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Die Nachricht steht wieder zum Versand an.")
}
