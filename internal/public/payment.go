package public

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

const (
	// paymentReturnPath is where the customer lands on the way back from the
	// payment page — success, failure and cancellation all come here, because
	// what actually happened is decided by asking Payrexx, not by which of
	// three addresses the browser was sent to.
	paymentReturnPath = "/zahlung/zurueck"
	// paymentHookPath is where Payrexx posts. Nothing in the body is trusted.
	paymentHookPath = "/zahlung/payrexx"
)

// SetPayments attaches the payment provider.
//
// A nil or unconfigured client is the normal case for most installations: the
// shop then offers invoice and prepayment, the online method is not shown, and
// the two payment routes 404.
func (h *Handler) SetPayments(c *payrexx.Client) { h.payments = c }

// payrexxEnabled reports whether online payment can be offered.
func (h *Handler) payrexxEnabled() bool {
	return h.payments.Configured() && h.orders != nil
}

// startPayment sets up the gateway and sends the customer there.
//
// Called after the order exists. That order is placed and the stock reserved
// before a single franc is asked for, which is the right way round: an order
// that exists and is unpaid is a problem the operator can see and chase, while
// a payment taken against no order is money that has to be found and refunded.
func (h *Handler) startPayment(w http.ResponseWriter, r *http.Request,
	website *domain.Website, order *shop.Order) error {

	base := h.canonicalBase(r, website)
	back := base + paymentReturnPath + "/" + order.Number

	gw, err := h.payments.CreateGateway(r.Context(), payrexx.GatewayRequest{
		Amount:      int64(order.Totals.TotalGross),
		Currency:    order.Currency,
		Purpose:     website.Name + " — Bestellung " + order.Number,
		ReferenceID: order.Number,
		// All three lead to the same place. The customer who cancelled and the
		// customer who paid both need to be told what their order now is, and
		// only Payrexx knows which of the two they are.
		SuccessRedirectURL: back,
		FailedRedirectURL:  back,
		CancelRedirectURL:  back,
		Prefill: map[string]string{
			"forename": order.Customer.Name,
			"email":    order.Customer.Email,
			"street":   order.Customer.Street,
			"postcode": order.Customer.PostalCode,
			"place":    order.Customer.City,
			"country":  order.Customer.Country,
		},
	})

	if err != nil {
		// The order stands. Sending the customer to the confirmation with the
		// payment still open beats an error page: the goods are reserved, the
		// operator sees an unpaid order, and nobody has lost anything.
		slog.Error("payrexx gateway not created", "order", order.Number, "err", err)
		http.Redirect(w, r, thanksPath+"/"+order.Number, http.StatusSeeOther)
		return nil
	}

	// The gateway id is how the order is found again on the way back and in
	// the webhook. Written before the redirect, because after the redirect
	// this process may never hear from this request again.
	if err := h.orders.SetPayment(r.Context(), order.ID, shop.PaymentOpen,
		strconv.FormatInt(gw.ID, 10)); err != nil {
		return fmt.Errorf("record payment reference: %w", err)
	}

	if gw.Link == "" {
		slog.Error("payrexx gateway without a link", "order", order.Number, "gateway", gw.ID)
		http.Redirect(w, r, thanksPath+"/"+order.Number, http.StatusSeeOther)
		return nil
	}

	http.Redirect(w, r, gw.Link, http.StatusSeeOther)
	return nil
}

// HandlePaymentReturn takes the customer back from the payment page.
//
// The address is guessable — it carries an order number and nothing else — so
// it must not do anything a stranger should not be able to do. It does not: it
// asks Payrexx what the state of that order's payment is and writes down the
// answer. Someone who guesses a number causes one API call and learns nothing.
func (h *Handler) HandlePaymentReturn(w http.ResponseWriter, r *http.Request) error {
	website, _, ok := h.shopRequest(w, r)
	if !ok {
		return nil
	}
	if !h.payrexxEnabled() {
		return h.serve404(w, r, website)
	}

	order, err := h.orders.ByNumber(r.Context(), website.ID, r.PathValue("number"))
	if err != nil {
		return err
	}
	if order == nil {
		return h.serve404(w, r, website)
	}

	if err := h.settlePayment(r.Context(), order); err != nil {
		// Not fatal for the customer: the confirmation page still shows their
		// order, and the webhook or the operator will settle it.
		slog.Error("payment could not be checked", "order", order.Number, "err", err)
	}

	http.Redirect(w, r, thanksPath+"/"+order.Number, http.StatusSeeOther)
	return nil
}

// HandlePaymentHook takes Payrexx's notification.
//
// The body is a hint that something happened, never the evidence of what: this
// address is on the open internet with no shared secret, so anyone who finds it
// can post to it. Exactly one thing is read out of the body — the gateway id —
// and then Payrexx is asked directly. A forged body can therefore only cause a
// lookup of a gateway that either does not exist or says what it always said.
//
// Always answers 200. A payment provider that gets an error retries, and
// retrying will not fix a body we cannot use.
func (h *Handler) HandlePaymentHook(w http.ResponseWriter, r *http.Request) error {
	website, _, ok := h.shopRequest(w, r)
	if !ok {
		return nil
	}
	if !h.payrexxEnabled() {
		return h.serve404(w, r, website)
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		slog.Warn("payment hook body unreadable", "err", err)
		w.WriteHeader(http.StatusOK)
		return nil
	}

	id, ok := payrexx.TransactionIDFromWebhook(body)
	if !ok {
		slog.Warn("payment hook without a gateway id")
		w.WriteHeader(http.StatusOK)
		return nil
	}

	gw, err := h.payments.GetGateway(r.Context(), id)
	if err != nil {
		slog.Error("payment hook could not be verified", "gateway", id, "err", err)
		w.WriteHeader(http.StatusOK)
		return nil
	}

	// The gateway names the order, not the webhook body.
	order, err := h.orders.ByNumber(r.Context(), website.ID, gw.ReferenceID)
	if err != nil {
		return err
	}
	if order == nil {
		slog.Warn("payment for an unknown order", "reference", gw.ReferenceID)
		w.WriteHeader(http.StatusOK)
		return nil
	}

	if err := h.applyGateway(r.Context(), order, gw); err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

// settlePayment asks the provider about an order and records the answer.
func (h *Handler) settlePayment(ctx context.Context, order *shop.Order) error {
	if order.PaymentMethod != shop.PayPayrexx {
		return nil
	}
	// Already settled. Asking again would be harmless but pointless, and a
	// refund recorded by hand must not be overwritten by an old "confirmed".
	if order.PaymentStatus != shop.PaymentOpen {
		return nil
	}
	if order.PaymentReference == "" {
		return errors.New("die Bestellung hat keine Zahlungsreferenz")
	}

	id, err := strconv.ParseInt(order.PaymentReference, 10, 64)
	if err != nil {
		return fmt.Errorf("payment reference %q: %w", order.PaymentReference, err)
	}

	gw, err := h.payments.GetGateway(ctx, id)
	if err != nil {
		return err
	}
	return h.applyGateway(ctx, order, gw)
}

// applyGateway writes a provider's verdict onto an order.
func (h *Handler) applyGateway(ctx context.Context, order *shop.Order, gw *payrexx.Gateway) error {
	switch {
	case gw.Paid():
		// The amount is checked, not assumed. A gateway confirmed over one
		// franc must never settle an order of five thousand — and that is not
		// a hypothetical, it is what an attacker who can create gateways on
		// their own account would try first.
		if gw.Amount != int64(order.Totals.TotalGross) {
			slog.Error("payment amount does not match the order",
				"order", order.Number, "paid", gw.Amount, "due", int64(order.Totals.TotalGross))
			return nil
		}
		if gw.Currency != "" && !strings.EqualFold(gw.Currency, order.Currency) {
			slog.Error("payment currency does not match the order",
				"order", order.Number, "paid", gw.Currency, "due", order.Currency)
			return nil
		}
		return h.orders.SetPayment(ctx, order.ID, shop.PaymentPaid, order.PaymentReference)

	case gw.Failed():
		// The order stays. A failed payment is a customer who may try again,
		// and deleting their order is the one thing that makes that impossible.
		return h.orders.SetPayment(ctx, order.ID, shop.PaymentFailed, order.PaymentReference)
	}

	// Waiting, authorised, reserved: nothing has arrived yet.
	return nil
}
