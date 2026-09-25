package admin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The shop, for the assistant's tools: products, orders, the settings and the
// overview. Every method takes a website the caller has already had authorised
// and holds every id it is given against it — an id arrives from outside and
// is evidence of nothing.
//
// Each method goes through the same function the screen does (saveProduct,
// setOrderStatus, checkPayment, saveShopSettings, shopOverview), so a rule
// added to the form is a rule the assistant keeps too.

// ErrNoShop says this installation was built without a shop, or without its
// order book.
var ErrNoShop = errors.New("the shop is not available on this installation")

// ErrNoSuchOrder says the order does not exist on this website.
var ErrNoSuchOrder = errors.New("there is no such order on this website")

// ValidationError carries what a form would have shown beside its fields. The
// messages are the English catalogue keys, which is what a machine reads.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Fields[k])
	}
	return "not saved — " + strings.Join(parts, "; ")
}

func validationError(errs web.FormErrors) error {
	if !errs.Any() {
		return nil
	}
	return &ValidationError{Fields: map[string]string(errs)}
}

// shopWebsiteOp is shopWebsite without a request.
func (h *Handler) shopWebsiteOp(ctx context.Context, websiteID int64) (*domain.Website, error) {
	if h.products == nil {
		return nil, ErrNoShop
	}
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, errors.New("there is no such website")
	}
	return ws, nil
}

// --- products ---------------------------------------------------------------

// OpListProducts is a website's catalogue, drafts included, in the operator's
// order.
func (h *Handler) OpListProducts(ctx context.Context, websiteID int64) ([]*shop.Product, error) {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return nil, err
	}
	return h.products.List(ctx, websiteID)
}

// OpGetProduct reads one product of a website together with its categories,
// comma-separated as the form shows them. A product of another website is
// shop.ErrNotFound, the same answer as none at all.
func (h *Handler) OpGetProduct(ctx context.Context, websiteID, productID int64) (*shop.Product, string, error) {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return nil, "", err
	}
	p, err := h.products.Get(ctx, productID)
	if err != nil {
		return nil, "", err
	}
	if p == nil || p.WebsiteID != websiteID {
		return nil, "", shop.ErrNotFound
	}
	return p, h.productTermNames(ctx, websiteID, p.ID), nil
}

// OpProductInput is a stored product as the edit form would show it: the
// starting point for a change that gives only some of the fields.
func (h *Handler) OpProductInput(ctx context.Context, websiteID, productID int64) (shop.ProductInput, error) {
	p, terms, err := h.OpGetProduct(ctx, websiteID, productID)
	if err != nil {
		return shop.ProductInput{}, err
	}
	v := productToValues(p)
	v.Terms = terms
	return v.ProductInput, nil
}

// OpSaveProduct validates and stores a product exactly as the product form
// does, creating it when in.ID is zero. A problem with the input is a
// *ValidationError; nothing is stored then.
func (h *Handler) OpSaveProduct(ctx context.Context, websiteID int64, in shop.ProductInput) (int64, error) {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return 0, err
	}
	values := productValues{ProductInput: in}
	errs := web.FormErrors{}
	id, err := h.saveProduct(ctx, websiteID, &values, errs)
	if verr := validationError(errs); verr != nil {
		return 0, verr
	}
	return id, err
}

// OpDeleteProduct removes a product of a website. Orders that contain it keep
// their lines: those are frozen copies.
func (h *Handler) OpDeleteProduct(ctx context.Context, websiteID, productID int64) error {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return err
	}
	return h.products.Delete(ctx, websiteID, productID)
}

// --- orders -----------------------------------------------------------------

func (h *Handler) orderOp(ctx context.Context, websiteID int64, number string) (*domain.Website, *shop.Order, error) {
	ws, err := h.shopWebsiteOp(ctx, websiteID)
	if err != nil {
		return nil, nil, err
	}
	if h.orders == nil {
		return nil, nil, ErrNoShop
	}
	order, err := h.orders.ByNumber(ctx, websiteID, strings.TrimSpace(number))
	if err != nil {
		return nil, nil, err
	}
	if order == nil {
		return nil, nil, ErrNoSuchOrder
	}
	return ws, order, nil
}

// OpListOrders is a website's orders, newest first, optionally only those in
// one status. At most limit of them; the screen shows two hundred.
func (h *Handler) OpListOrders(ctx context.Context, websiteID int64, status string, limit int) ([]*shop.Order, error) {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return nil, err
	}
	if h.orders == nil {
		return nil, ErrNoShop
	}
	if status == "" {
		return h.orders.List(ctx, websiteID, limit)
	}
	// The same reach as the overview: a year of a workshop's orders.
	all, err := h.orders.List(ctx, websiteID, 1000)
	if err != nil {
		return nil, err
	}
	out := make([]*shop.Order, 0, limit)
	for _, o := range all {
		if o.Status == status {
			out = append(out, o)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// OpOrder is one order with its lines, what the outbox holds for it, and
// whether asking the payment provider again can help — what the order screen
// shows.
func (h *Handler) OpOrder(ctx context.Context, websiteID int64, number string) (*shop.Order, []outbox.Mail, bool, error) {
	_, order, err := h.orderOp(ctx, websiteID, number)
	if err != nil {
		return nil, nil, false, err
	}
	mails, err := h.orderMails(ctx, order.ID)
	if err != nil {
		return nil, nil, false, err
	}
	return order, mails, h.canRecheck(order), nil
}

// OpSetOrderStatus moves an order along with the screen's side effects: a move
// to "shipped" queues the dispatch notice to the customer, once. queued is the
// number of messages put into the outbox. An error means the status did not
// change; mailProblem, when not empty, says that it did but the notice could
// not be stored.
func (h *Handler) OpSetOrderStatus(ctx context.Context, websiteID int64, number, status string) (queued int, mailProblem string, err error) {
	ws, order, err := h.orderOp(ctx, websiteID, number)
	if err != nil {
		return 0, "", err
	}
	switch status {
	case shop.OrderNew, shop.OrderPaid, shop.OrderShipped, shop.OrderCancelled:
	default:
		return 0, "", fmt.Errorf("unknown order status %q", status)
	}
	queued, mailErr, err := h.setOrderStatus(ctx, ws, order, status, h.opHost(ctx, ws.ID))
	if err != nil {
		return 0, "", err
	}
	if mailErr != nil {
		return queued, "The status was changed, but the shipping notification could not be stored: " +
			mailErr.Error(), nil
	}
	return queued, "", nil
}

// opHost stands in for the request's host when a mail needs a link back to the
// shop: the primary domain, otherwise the website's first one. Empty when the
// website has no domain at all — the mail then goes without the link rather
// than with a broken one.
func (h *Handler) opHost(ctx context.Context, websiteID int64) string {
	if primary, err := h.domains.PrimaryDomain(ctx, websiteID); err == nil && primary != "" {
		return primary
	}
	list, err := h.domains.ListDomains(ctx, websiteID)
	if err != nil || len(list) == 0 {
		return ""
	}
	return list[0].Domain
}

// OpRecheckPayment asks the payment provider once more about an order, as the
// button on the order screen does. It answers shop.PaymentPaid,
// shop.PaymentFailed or shop.PaymentOpen (nothing recorded yet).
func (h *Handler) OpRecheckPayment(ctx context.Context, websiteID int64, number string) (string, error) {
	_, order, err := h.orderOp(ctx, websiteID, number)
	if err != nil {
		return "", err
	}
	return h.checkPayment(ctx, order)
}

// OpRetryOrderMail puts a message of an order that did not go out back into
// the queue. The message must belong to that order, and the order to the
// website.
func (h *Handler) OpRetryOrderMail(ctx context.Context, websiteID int64, number string, mailID int64) error {
	_, order, err := h.orderOp(ctx, websiteID, number)
	if err != nil {
		return err
	}
	if h.outbox == nil {
		return errors.New("there is no outbox on this installation")
	}
	mails, err := h.outbox.ForOrder(ctx, order.ID)
	if err != nil {
		return err
	}
	for _, m := range mails {
		if m.ID != mailID {
			continue
		}
		if m.Status == outbox.StatusSent {
			return errors.New("this message has already been sent")
		}
		return h.outbox.Retry(ctx, websiteID, mailID)
	}
	return errors.New("this order has no such message")
}

// --- settings and overview --------------------------------------------------

// OpShopSettings is a website's shop configuration as the settings form shows
// it.
func (h *Handler) OpShopSettings(ctx context.Context, websiteID int64) (shop.SettingsInput, error) {
	ws, err := h.shopWebsiteOp(ctx, websiteID)
	if err != nil {
		return shop.SettingsInput{}, err
	}
	return shopSettingsValuesOf(ws).SettingsInput, nil
}

// OpSaveShopSettings validates and stores a website's shop configuration as
// the settings form does. A problem with the input is a *ValidationError.
func (h *Handler) OpSaveShopSettings(ctx context.Context, websiteID int64, in shop.SettingsInput) error {
	if _, err := h.shopWebsiteOp(ctx, websiteID); err != nil {
		return err
	}
	// The form offers exactly these two in a select; a free text field here
	// would let a typo become the currency every price is written in.
	switch strings.ToUpper(strings.TrimSpace(in.Currency)) {
	case "", "CHF", "EUR":
	default:
		return &ValidationError{Fields: map[string]string{"currency": "CHF or EUR"}}
	}
	values := shopSettingsValues{SettingsInput: in}
	errs := web.FormErrors{}
	if err := h.saveShopSettings(ctx, websiteID, &values, errs); err != nil {
		return err
	}
	return validationError(errs)
}

// OpShopOverview is the shop overview's numbers as of now.
func (h *Handler) OpShopOverview(ctx context.Context, websiteID int64) (shop.Overview, money.Currency, error) {
	ws, err := h.shopWebsiteOp(ctx, websiteID)
	if err != nil {
		return shop.Overview{}, money.Currency{}, err
	}
	ov, err := h.shopOverview(ctx, websiteID, time.Now())
	return ov, money.CurrencyFor(ws.Currency), err
}
