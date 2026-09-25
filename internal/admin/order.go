package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SetOrderStore attaches the order book.
func (h *Handler) SetOrderStore(s *shop.OrderStore) { h.orders = s }

// orderListData backs the order list.
type orderListData struct {
	web.LayoutData
	web.FormState
	Orders   []*shop.Order
	Currency money.Currency
}

// orderDetailData backs one order.
type orderDetailData struct {
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
	Mails []mailLine
}

// mailLine is one outbox entry as the order page shows it.
type mailLine struct {
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
		return i18n.N("Confirmation to the customer")
	case outbox.KindOrderOperator:
		return i18n.N("Notice to the business")
	case outbox.KindOrderShipped:
		return i18n.N("Dispatch notice")
	}
	return kind
}

func mailStateLabel(m outbox.Mail) string {
	switch m.Status {
	case outbox.StatusSent:
		return i18n.N("sent")
	case outbox.StatusFailed:
		return i18n.N("given up")
	default:
		if m.Attempts > 0 {
			return i18n.N("waiting for the next attempt")
		}
		return i18n.N("waiting to be sent")
	}
}

// mailLines maps the outbox onto the order page.
func mailLines(mails []outbox.Mail) []mailLine {
	out := make([]mailLine, 0, len(mails))
	for _, m := range mails {
		line := mailLine{
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
		return i18n.N("Payment in advance")
	case shop.PayPayrexx:
		return i18n.N("Online payment (Payrexx)")
	case shop.PayInvoice:
		return i18n.N("Invoice")
	}
	return code
}

// paymentStateLabel names a stored payment state.
func paymentStateLabel(code string) string {
	switch code {
	case shop.PaymentPaid:
		return i18n.N("paid")
	case shop.PaymentFailed:
		return i18n.N("failed")
	case shop.PaymentRefunded:
		return i18n.N("refunded")
	case shop.PaymentOpen:
		return i18n.N("open")
	}
	return code
}

// orderStatuses are the moves an operator can make, in the order they happen.
var orderStatuses = []struct{ Value, Label string }{
	{shop.OrderNew, i18n.N("New")},
	{shop.OrderPaid, i18n.N("Paid")},
	{shop.OrderShipped, i18n.N("Dispatched")},
	{shop.OrderCancelled, i18n.N("Cancelled")},
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

	data := orderListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Orders – %s", ws.Name)),
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
			h.retryMail(r, ws.ID, r.FormValue("mail_id"))

		default:
			_, mailErr, err := h.setOrderStatus(r.Context(), ws, order, r.FormValue("status"), r.Host)
			if err != nil {
				web.SetFlashError(h.sm, r.Context(), err.Error())
				break
			}
			web.SetFlashSuccess(h.sm, r.Context(), "Status changed")
			if mailErr != nil {
				web.SetFlashError(h.sm, r.Context(), "The shipping notification could not be stored.")
			}
		}
		return h.redirect(w, r, "/admin/websites/"+strconv.FormatInt(ws.ID, 10)+
			"/bestellungen/"+order.Number)
	}

	mails, err := h.orderMails(r.Context(), order.ID)
	if err != nil {
		return err
	}

	currency := money.CurrencyFor(order.Currency)
	data := orderDetailData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Order %s – %s", order.Number, ws.Name)),
		FormState:  web.NewFormState(),
		Order:      order,
		Currency:   currency,
		Statuses:   orderStatuses,

		PaymentMethod: paymentMethodLabel(order.PaymentMethod),
		PaymentState:  paymentStateLabel(order.PaymentStatus),
		CanRecheck:    h.canRecheck(order),
		Mails:         mailLines(mails),
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
func (h *Handler) recheckPayment(r *http.Request, order *shop.Order) {
	switch outcome, err := h.checkPayment(r.Context(), order); {
	case err != nil:
		web.SetFlashError(h.sm, r.Context(), err.Error())
	case outcome == shop.PaymentPaid:
		web.SetFlashSuccess(h.sm, r.Context(), "The payment has come in.")
	case outcome == shop.PaymentFailed:
		web.SetFlashSuccess(h.sm, r.Context(), "The payment was cancelled or declined.")
	default:
		web.SetFlashSuccess(h.sm, r.Context(), "The provider has no payment recorded yet.")
	}
}

// checkPayment asks the payment provider about an order and records what it
// says. It answers shop.PaymentPaid, shop.PaymentFailed or shop.PaymentOpen
// (nothing recorded yet). The screen and OpRecheckPayment share it; an error's
// text is a catalogue key, so the screen can show it translated.
//
// Only ever reads from the provider. It cannot make an unpaid order paid,
// because the answer comes from Payrexx, not from the button.
func (h *Handler) checkPayment(ctx context.Context, order *shop.Order) (string, error) {
	if !h.payments.Configured() {
		return "", errors.New(i18n.N("No payment provider is set up for this installation."))
	}
	if order.PaymentMethod != shop.PayPayrexx || order.PaymentReference == "" {
		return "", errors.New(i18n.N("This order was not paid online."))
	}

	id, err := strconv.ParseInt(order.PaymentReference, 10, 64)
	if err != nil {
		return "", errors.New(i18n.N("The payment reference is unreadable."))
	}

	gw, err := h.payments.GetGateway(ctx, id)
	if err != nil {
		slog.Error("payment recheck failed", "order", order.Number, "err", err)
		return "", errors.New(i18n.N("The payment provider could not be reached."))
	}

	switch {
	case gw.Paid():
		// The amount is checked here for the same reason as on the public
		// side: a gateway is only evidence for the amount it names.
		if gw.Amount != int64(order.Totals.TotalGross) {
			return "", errors.New(i18n.N("The amount recorded with the provider does not match the order."))
		}
		if err := h.orders.SetPayment(ctx, order.ID, shop.PaymentPaid, order.PaymentReference); err != nil {
			return "", err
		}
		return shop.PaymentPaid, nil

	case gw.Failed():
		if err := h.orders.SetPayment(ctx, order.ID, shop.PaymentFailed, order.PaymentReference); err != nil {
			return "", err
		}
		return shop.PaymentFailed, nil
	}
	return shop.PaymentOpen, nil
}

// canRecheck reports whether asking the payment provider again can help: an
// online payment that is still open, on an installation that has the keys.
func (h *Handler) canRecheck(order *shop.Order) bool {
	return order.PaymentMethod == shop.PayPayrexx &&
		order.PaymentStatus == shop.PaymentOpen &&
		order.PaymentReference != "" && h.payments.Configured()
}

// orderMails is what the outbox holds for one order; none without an outbox.
func (h *Handler) orderMails(ctx context.Context, orderID int64) ([]outbox.Mail, error) {
	if h.outbox == nil {
		return nil, nil
	}
	return h.outbox.ForOrder(ctx, orderID)
}

// setOrderStatus moves an order along, and queues the dispatch notice when the
// move is to "shipped". The screen and OpSetOrderStatus share it. queued is how
// many messages went into the outbox; mailErr says the notice could not be
// stored although the status did change.
func (h *Handler) setOrderStatus(ctx context.Context, ws *domain.Website, order *shop.Order,
	status, host string) (queued int, mailErr, err error) {

	if err := h.orders.SetStatus(ctx, ws.ID, order.ID, status); err != nil {
		return 0, nil, err
	}
	// Moving to "dispatched" deserves a message to the customer. Only on the
	// change: whoever saves the status twice should not report twice that the
	// same parcel is on its way.
	if status == shop.OrderShipped && order.Status != shop.OrderShipped {
		queued, mailErr = h.announceShipment(ctx, ws, order, host)
	}
	return queued, mailErr, nil
}

// announceShipment tells the customer their order is on its way. host is the
// host the request came in on, the fallback for the link back to the order
// when the website has no primary domain.
func (h *Handler) announceShipment(ctx context.Context, ws *domain.Website, order *shop.Order, host string) (int, error) {
	if h.outbox == nil {
		return 0, nil
	}
	base := ""
	if h.resolver != nil && host != "" {
		base = h.resolver.CanonicalBase(ctx, ws, host)
	}
	shopInfo := outbox.Shop{
		Name:           ws.Name,
		URL:            base,
		OrderEmail:     ws.OrderEmail,
		PaymentDetails: ws.PaymentDetails,
		VATNumber:      ws.VATNumber,
		Currency:       money.CurrencyFor(ws.Currency),
	}
	queued := 0
	var failed error
	for _, m := range outbox.ForShipment(shopInfo, order) {
		if _, err := h.outbox.Queue(ctx, m); err != nil {
			slog.Error("shipment mail not queued", "order", order.Number, "err", err)
			failed = err
			continue
		}
		queued++
	}
	return queued, failed
}

// retryMail puts a message that did not go out back in the queue.
//
// Only worth offering after the operator has fixed something — a typo in the
// address, a mail account that had expired. It does not send anything itself;
// the background run does that, and this only says "try again".
// retryMail puts one message back in the queue.
//
// websiteID is the website the address named and the guard authorised. The
// message id beside it comes from the form and is not evidence of anything, so
// the two are handed to the store together and it decides.
func (h *Handler) retryMail(r *http.Request, websiteID int64, raw string) {
	if h.outbox == nil {
		return
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Unknown message.")
		return
	}
	if err := h.outbox.Retry(r.Context(), websiteID, id); err != nil {
		web.SetFlashError(h.sm, r.Context(), err.Error())
		return
	}
	web.SetFlashSuccess(h.sm, r.Context(), "The message is queued for sending again.")
}
