package shop

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

func newOrders(t *testing.T) (*OrderStore, *CartStore, *Store, int64) {
	t.Helper()
	products, ws := newTestStore(t)
	carts := NewCartStore(products)
	return NewOrderStore(carts), carts, products, ws
}

func customer() Customer {
	return Customer{
		Email: "kundin@example.ch", Name: "Anna Meier",
		Street: "Seestrasse 4", PostalCode: "8002", City: "Zürich", Country: "CH",
	}
}

func filled(t *testing.T, carts *CartStore, ws int64, p *Product, qty int) *Cart {
	t.Helper()
	ctx := context.Background()
	cart, _, err := carts.Ensure(ctx, ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := carts.Add(ctx, cart.ID, p.ID, qty); err != nil {
		t.Fatal(err)
	}
	got, err := carts.Get(ctx, ws, cart.Token)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// An order is a snapshot. Renaming or repricing the product afterwards must not
// change what was ordered — an invoice that rewrites itself is not an invoice.
func TestOrderKeepsWhatWasSoldWhenTheProductChanges(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil) // 49.00
	p.Title = "Hocker Brunni"
	p.SKU = "HO-1"
	if err := products.Update(ctx, p); err != nil {
		t.Fatal(err)
	}

	cart := filled(t, carts, ws, p, 2)
	order, err := orders.Place(ctx, ws, settings(), Private, cart, customer(), PayInvoice)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	// Now change the product beyond recognition, and delete nothing.
	p.Title = "Ganz anderer Hocker"
	p.PriceGross = 999900
	p.SKU = "XX-9"
	if err := products.Update(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := orders.ByNumber(ctx, ws, order.Number)
	if err != nil {
		t.Fatalf("ByNumber: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("%d lines, want 1", len(got.Items))
	}
	it := got.Items[0]
	if it.Title != "Hocker Brunni" || it.SKU != "HO-1" {
		t.Errorf("the line followed the product: %q / %q", it.Title, it.SKU)
	}
	if it.UnitGross != 4900 || it.LineGross != 9800 {
		t.Errorf("the price followed the product: unit %d, line %d", it.UnitGross, it.LineGross)
	}
	if got.Totals.TotalGross != order.Totals.TotalGross {
		t.Errorf("the total changed: %d, was %d", got.Totals.TotalGross, order.Totals.TotalGross)
	}
}

// A deleted product must leave the order intact: what was sold does not stop
// having been sold.
func TestOrderSurvivesTheProductBeingDeleted(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)

	cart := filled(t, carts, ws, p, 1)
	order, err := orders.Place(ctx, ws, settings(), Private, cart, customer(), PayInvoice)
	if err != nil {
		t.Fatal(err)
	}
	if err := products.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}

	got, err := orders.ByNumber(ctx, ws, order.Number)
	if err != nil || got == nil {
		t.Fatalf("the order vanished with its product: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Title != "hocker" {
		t.Errorf("the line vanished with its product: %+v", got.Items)
	}
	if got.Items[0].ProductID != nil {
		t.Error("the line still points at a product that no longer exists")
	}
}

// Placing an order takes the goods out of stock, and it happens in the same
// transaction as the order itself.
func TestPlacingAnOrderReservesTheStock(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, intp(5))

	cart := filled(t, carts, ws, p, 2)
	if _, err := orders.Place(ctx, ws, settings(), Private, cart, customer(), PayInvoice); err != nil {
		t.Fatalf("Place: %v", err)
	}

	after, _ := products.Get(ctx, p.ID)
	if after.Stock == nil || *after.Stock != 3 {
		t.Errorf("stock is %v, want 3", after.Stock)
	}

	// And the basket is empty afterwards, so a reload does not order twice.
	again, _ := carts.Get(ctx, ws, cart.Token)
	if !again.Empty() {
		t.Errorf("the basket still holds %d lines after the order", len(again.Items))
	}
}

// Two people buying the last piece at the same moment: exactly one order may
// succeed, and the loser has to be told which article.
func TestOnlyOneOrderGetsTheLastPiece(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "letztes", StatusPublished, intp(1))
	p.Title = "Letztes Stück"
	if err := products.Update(ctx, p); err != nil {
		t.Fatal(err)
	}

	const buyers = 6
	baskets := make([]*Cart, buyers)
	for i := range baskets {
		baskets[i] = filled(t, carts, ws, p, 1)
	}

	start := make(chan struct{})
	var ready, done sync.WaitGroup
	ready.Add(buyers)
	done.Add(buyers)
	errs := make([]error, buyers)

	for i := 0; i < buyers; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			_, errs[i] = orders.Place(ctx, ws, settings(), Private, baskets[i], customer(), PayInvoice)
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	won, named := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case strings.Contains(err.Error(), "Letztes Stück"):
			named++
		default:
			t.Errorf("unexpected failure: %v", err)
		}
	}
	if won != 1 {
		t.Errorf("%d of %d orders got the last piece; exactly one may", won, buyers)
	}
	if named != buyers-1 {
		t.Errorf("%d refusals named the article; all %d should", named, buyers-1)
	}

	after, _ := products.Get(ctx, p.ID)
	if after.Stock == nil || *after.Stock != 0 {
		t.Errorf("stock ended at %v, want 0", after.Stock)
	}
}

// A failed order must leave nothing behind — no number, no lines, no emptied
// basket.
func TestAFailedOrderLeavesNothingBehind(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	ok := seedProduct(t, products, ws, "vorraetig", StatusPublished, intp(10))
	short := seedProduct(t, products, ws, "knapp", StatusPublished, intp(1))

	cart, _, _ := carts.Ensure(ctx, ws, "")
	if err := carts.Add(ctx, cart.ID, ok.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := carts.Add(ctx, cart.ID, short.ID, 1); err != nil {
		t.Fatal(err)
	}
	full, _ := carts.Get(ctx, ws, cart.Token)

	// Someone else takes the last one first.
	if _, err := products.AdjustStock(ctx, short.ID, -1); err != nil {
		t.Fatal(err)
	}

	if _, err := orders.Place(ctx, ws, settings(), Private, full, customer(), PayInvoice); err == nil {
		t.Fatal("the order went through although an article had run out")
	}

	list, _ := orders.List(ctx, ws, 10)
	if len(list) != 0 {
		t.Errorf("%d orders were written despite the failure", len(list))
	}
	// The available article must not have been reserved for an order that
	// never happened.
	after, _ := products.Get(ctx, ok.ID)
	if *after.Stock != 10 {
		t.Errorf("stock of the available article is %d, want 10 — it was reserved for nothing", *after.Stock)
	}
	again, _ := carts.Get(ctx, ws, cart.Token)
	if again.Empty() {
		t.Error("the basket was emptied although the order failed")
	}
}

// Order numbers count within the calendar year and never repeat.
func TestOrderNumbersCountWithinTheYear(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)

	fixed := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	orders.now = func() time.Time { return fixed }

	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		o, err := orders.Place(ctx, ws, settings(), Private, filled(t, carts, ws, p, 1),
			customer(), PayInvoice)
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if seen[o.Number] {
			t.Fatalf("order number %s was issued twice", o.Number)
		}
		seen[o.Number] = true
	}
	for _, want := range []string{"2026-0001", "2026-0002", "2026-0003"} {
		if !seen[want] {
			t.Errorf("missing order number %s; got %v", want, seen)
		}
	}

	// A new year starts at one again.
	orders.now = func() time.Time { return fixed.AddDate(1, 0, 0) }
	o, err := orders.Place(ctx, ws, settings(), Private, filled(t, carts, ws, p, 1),
		customer(), PayInvoice)
	if err != nil {
		t.Fatal(err)
	}
	if o.Number != "2027-0001" {
		t.Errorf("first order of the new year is %s, want 2027-0001", o.Number)
	}
}

// A late payment notice must not drag a shipped order backwards.
func TestPaymentNoticeDoesNotUndoAShippedOrder(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	o, err := orders.Place(ctx, ws, settings(), Private, filled(t, carts, ws, p, 1),
		customer(), PayPayrexx)
	if err != nil {
		t.Fatal(err)
	}

	if err := orders.SetStatus(ctx, ws, o.ID, OrderShipped); err != nil {
		t.Fatal(err)
	}
	if err := orders.SetPayment(ctx, o.ID, PaymentPaid, "pr_123"); err != nil {
		t.Fatal(err)
	}

	got, _ := orders.ByNumber(ctx, ws, o.Number)
	if got.Status != OrderShipped {
		t.Errorf("status fell back to %q after a late payment notice", got.Status)
	}
	if got.PaymentStatus != PaymentPaid || got.PaymentReference != "pr_123" {
		t.Errorf("the payment was not recorded: %s / %s", got.PaymentStatus, got.PaymentReference)
	}
}

// The order's totals are the ones the customer was shown, and they add up.
func TestOrderTotalsMatchTheBasket(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	cart := filled(t, carts, ws, p, 2)
	want := cart.Total(settings(), Private)

	o, err := orders.Place(ctx, ws, settings(), Private, cart, customer(), PayInvoice)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := orders.ByNumber(ctx, ws, o.Number)

	if got.Totals.TotalGross != want.TotalGross {
		t.Errorf("total %d, the basket said %d", got.Totals.TotalGross, want.TotalGross)
	}
	if got.Totals.TotalNet+got.Totals.TotalTax != got.Totals.TotalGross {
		t.Errorf("the stored total does not add up: %d + %d != %d",
			got.Totals.TotalNet, got.Totals.TotalTax, got.Totals.TotalGross)
	}
	var lines money.Amount
	for _, it := range got.Items {
		lines += it.LineGross
	}
	if lines != got.Totals.ItemsGross {
		t.Errorf("the lines sum to %d, the order says %d", lines, got.Totals.ItemsGross)
	}
}
