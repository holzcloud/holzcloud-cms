package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// The shop's Op methods are what the assistant's tools call. They are held to
// the same rules as the screens because they are the screens' own functions;
// these tests check that the wiring says so — the form's validation, the
// website scope, the dispatch notice — and that the tools can find them at all.

type shopOpsFixture struct {
	h        *Handler
	products *shop.Store
	orders   *shop.OrderStore
	mails    *outbox.Store
	siteA    *domain.Website
	siteB    *domain.Website
	order    *shop.Order
}

func newShopOpsFixture(t *testing.T) *shopOpsFixture {
	t.Helper()
	ctx := context.Background()
	h, _, database, siteA := newTestAdmin(t)

	products := shop.NewStore(database)
	carts := shop.NewCartStore(products)
	orders := shop.NewOrderStore(carts)
	mails := outbox.NewStore(database)
	h.SetProductStore(products)
	h.SetOrderStore(orders)
	h.SetOutbox(mails)

	siteB, err := domain.NewStore(database).CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}

	p := seedProduct(t, products, siteA.ID, "hocker", "Hocker", 4900)
	cart, token, err := carts.Ensure(ctx, siteA.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := carts.Add(ctx, cart.ID, p.ID, 2); err != nil {
		t.Fatal(err)
	}
	cart, err = carts.Get(ctx, siteA.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.Place(ctx, siteA.ID, shop.Settings{Currency: money.CurrencyFor("CHF")},
		shop.Private, cart, shop.Customer{Email: "kundin@example.ch", Name: "Anna Meier"}, shop.PayInvoice)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	return &shopOpsFixture{h: h, products: products, orders: orders, mails: mails,
		siteA: siteA, siteB: siteB, order: order}
}

// The tools reach these methods through an interface of their own. A signature
// that drifts on either side compiles and quietly makes every shop tool answer
// "not available"; this is where it fails instead.
func TestTheHandlerOffersEveryShopOp(t *testing.T) {
	h, _, _, _ := newTestAdmin(t)
	if !ai.HasShopOps(h) {
		t.Fatal("the admin handler does not satisfy the shop tools' interface")
	}
}

func TestOpSaveProductValidatesLikeTheForm(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()

	// The form's own rules: a price it cannot read, a rate that is not Swiss,
	// a negative stock.
	_, err := f.h.OpSaveProduct(ctx, f.siteA.ID, shop.ProductInput{
		Title: "Tisch", Price: "zwölf", TaxBP: 700, StockText: "-1", Status: shop.StatusDraft,
	})
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want a ValidationError", err)
	}
	for _, field := range []string{"price", "tax", "stock"} {
		if verr.Fields[field] == "" {
			t.Errorf("no complaint about %s: %v", field, verr.Fields)
		}
	}

	id, err := f.h.OpSaveProduct(ctx, f.siteA.ID, shop.ProductInput{
		Title: "Tisch", Price: "1'234.50", TaxBP: int(money.RateReduced), StockText: "",
		Status: shop.StatusDraft, SKU: "  TI-1  ", Terms: "Möbel, Eiche",
	})
	if err != nil {
		t.Fatalf("OpSaveProduct: %v", err)
	}
	p, terms, err := f.h.OpGetProduct(ctx, f.siteA.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.PriceGross != 123450 || p.TaxRate != money.RateReduced || p.Stock != nil {
		t.Errorf("stored price %d, rate %d, stock %v", p.PriceGross, p.TaxRate, p.Stock)
	}
	if p.SKU != "TI-1" || p.Slug != "tisch" {
		t.Errorf("sku %q, slug %q", p.SKU, p.Slug)
	}
	if terms != "Eiche, Möbel" && terms != "Möbel, Eiche" {
		t.Errorf("categories = %q", terms)
	}

	// The same slug again is the form's "address taken".
	_, err = f.h.OpSaveProduct(ctx, f.siteA.ID, shop.ProductInput{
		Title: "Tisch", Price: "1", TaxBP: int(money.RateStandard), Status: shop.StatusDraft,
	})
	if !errors.As(err, &verr) || verr.Fields["slug"] == "" {
		t.Errorf("a taken slug was not refused: %v", err)
	}
}

func TestShopOpsStayWithTheirWebsite(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()
	foreign := seedProduct(t, f.products, f.siteB.ID, "fremd", "Fremd", 100)

	if _, _, err := f.h.OpGetProduct(ctx, f.siteA.ID, foreign.ID); !errors.Is(err, shop.ErrNotFound) {
		t.Errorf("OpGetProduct across websites: %v", err)
	}
	in := shop.ProductInput{ID: foreign.ID, Title: "Gekapert", Price: "1",
		TaxBP: int(money.RateStandard), Status: shop.StatusDraft}
	if _, err := f.h.OpSaveProduct(ctx, f.siteA.ID, in); !errors.Is(err, shop.ErrNotFound) {
		t.Errorf("OpSaveProduct across websites: %v", err)
	}
	if err := f.h.OpDeleteProduct(ctx, f.siteA.ID, foreign.ID); err == nil {
		t.Error("OpDeleteProduct deleted another website's product")
	}
	if p, _ := f.products.Get(ctx, foreign.ID); p == nil || p.Title != "Fremd" || p.PriceGross != 100 {
		t.Errorf("the foreign product changed: %+v", p)
	}
	if _, _, _, err := f.h.OpOrder(ctx, f.siteB.ID, f.order.Number); !errors.Is(err, ErrNoSuchOrder) {
		t.Errorf("OpOrder across websites: %v", err)
	}
}

// Marking as shipped queues the dispatch notice, once — exactly what the
// order screen does.
func TestOpSetOrderStatusQueuesTheDispatchNoticeOnce(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()

	queued, problem, err := f.h.OpSetOrderStatus(ctx, f.siteA.ID, f.order.Number, shop.OrderShipped)
	if err != nil || problem != "" {
		t.Fatalf("OpSetOrderStatus: %v %q", err, problem)
	}
	if queued != 1 {
		t.Errorf("queued = %d, want 1", queued)
	}
	if _, _, err := f.h.OpSetOrderStatus(ctx, f.siteA.ID, f.order.Number, shop.OrderShipped); err != nil {
		t.Fatal(err)
	}
	order, mails, _, err := f.h.OpOrder(ctx, f.siteA.ID, f.order.Number)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != shop.OrderShipped {
		t.Errorf("status = %q", order.Status)
	}
	shipped := 0
	for _, m := range mails {
		if m.Kind == outbox.KindOrderShipped {
			shipped++
		}
	}
	if shipped != 1 {
		t.Errorf("%d dispatch notices, want exactly 1", shipped)
	}

	if _, _, err := f.h.OpSetOrderStatus(ctx, f.siteA.ID, f.order.Number, "verloren"); err == nil {
		t.Error("an unknown status was accepted")
	}
}

func TestOpRetryOrderMailOnlyForItsOrder(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()
	orderID := f.order.ID
	id, err := f.mails.Queue(ctx, outbox.Mail{WebsiteID: f.siteA.ID, OrderID: &orderID,
		Kind: outbox.KindOrderCustomer, Recipient: "kundin@example.ch", Subject: "x", Body: "y"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.mails.MarkFailed(ctx, id, errors.New("refused")); err != nil {
		t.Fatal(err)
	}
	if err := f.h.OpRetryOrderMail(ctx, f.siteA.ID, f.order.Number, id); err != nil {
		t.Fatalf("OpRetryOrderMail: %v", err)
	}
	if err := f.h.OpRetryOrderMail(ctx, f.siteA.ID, f.order.Number, id+100); err == nil {
		t.Error("a message of no order was re-queued")
	}
}

// Recheck is only for online payments, and says so rather than asking a
// provider the installation does not have.
func TestOpRecheckPaymentRefusesAnInvoice(t *testing.T) {
	f := newShopOpsFixture(t)
	if _, err := f.h.OpRecheckPayment(context.Background(), f.siteA.ID, f.order.Number); err == nil {
		t.Error("an invoice order was sent to the payment provider")
	}
}

func TestOpShopSettingsRoundTrip(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()

	in, err := f.h.OpShopSettings(ctx, f.siteA.ID)
	if err != nil {
		t.Fatal(err)
	}
	in.ShopBase = "laden"
	in.Currency = "eur"
	in.ShippingGross = "9,50"
	in.ShippingFreeAt = ""
	in.ShippingTaxBP = int(money.RateStandard)
	in.PriceDisplay = shop.DisplayBoth
	if err := f.h.OpSaveShopSettings(ctx, f.siteA.ID, in); err != nil {
		t.Fatalf("OpSaveShopSettings: %v", err)
	}
	after, err := f.h.OpShopSettings(ctx, f.siteA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ShopBase != "laden" || after.Currency != "EUR" || after.ShippingGross != "9.50" ||
		after.ShippingFreeAt != "" || after.PriceDisplay != shop.DisplayBoth {
		t.Errorf("stored %+v", after)
	}

	in.ShopBase = "a/b"
	var verr *ValidationError
	if err := f.h.OpSaveShopSettings(ctx, f.siteA.ID, in); !errors.As(err, &verr) {
		t.Errorf("a path with a slash was accepted: %v", err)
	}
	in.ShopBase = "laden"
	in.Currency = "USD"
	if err := f.h.OpSaveShopSettings(ctx, f.siteA.ID, in); !errors.As(err, &verr) {
		t.Errorf("an unknown currency was accepted: %v", err)
	}
}

func TestOpShopOverviewCountsWhatTheScreenCounts(t *testing.T) {
	f := newShopOpsFixture(t)
	ctx := context.Background()
	if err := f.orders.SetStatus(ctx, f.siteA.ID, f.order.ID, shop.OrderPaid); err != nil {
		t.Fatal(err)
	}
	ov, cur, err := f.h.OpShopOverview(ctx, f.siteA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Code != "CHF" {
		t.Errorf("currency %q", cur.Code)
	}
	if ov.ThisMonth != 9800 || ov.OpenOrders != 1 || ov.ProductsOnline != 1 {
		t.Errorf("overview %+v", ov)
	}
	if len(ov.Tasks) != 1 || ov.Tasks[0].Kind != shop.TaskDispatch {
		t.Errorf("tasks %+v", ov.Tasks)
	}
}
