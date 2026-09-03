package shop

import (
	"context"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

func newCart(t *testing.T) (*CartStore, *Store, int64) {
	t.Helper()
	products, ws := newTestStore(t)
	return NewCartStore(products), products, ws
}

func settings() Settings {
	limit := money.Amount(20000)
	return Settings{
		Base:            "shop",
		Currency:        money.CurrencyFor("CHF"),
		Display:         DisplayBoth,
		ShippingGross:   1200,
		ShippingFreeAt:  &limit,
		ShippingTaxRate: money.RateStandard,
	}
}

// Adding the same article twice is one line with two of it, not two lines.
func TestAddingTwiceRaisesTheQuantity(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)

	cart, _, err := carts.Ensure(ctx, ws, "")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := carts.Add(ctx, cart.ID, p.ID, 2); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	got, _ := carts.Get(ctx, ws, cart.Token)
	if len(got.Items) != 1 {
		t.Fatalf("%d lines, want 1", len(got.Items))
	}
	if got.Items[0].Quantity != 6 {
		t.Errorf("quantity %d, want 6", got.Items[0].Quantity)
	}
	if got.Count() != 6 {
		t.Errorf("Count() = %d, want 6 — a badge that counts lines says 1 for six stools", got.Count())
	}
}

// A quantity is attacker-controlled. The cap has to hold even when it is
// approached in several requests, so it is applied in SQL and not only in Go.
func TestQuantityIsCappedAcrossRequests(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	cart, _, _ := carts.Ensure(ctx, ws, "")

	for i := 0; i < 5; i++ {
		if err := carts.Add(ctx, cart.ID, p.ID, 90); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	got, _ := carts.Get(ctx, ws, cart.Token)
	if got.Items[0].Quantity != maxQuantity {
		t.Errorf("quantity %d, want the cap of %d", got.Items[0].Quantity, maxQuantity)
	}
}

// A sold-out article cannot be added, and a draft is not for sale at all.
func TestUnavailableProductsCannotBeAdded(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	cart, _, _ := carts.Ensure(ctx, ws, "")

	soldOut := seedProduct(t, products, ws, "leer", StatusPublished, intp(0))
	draft := seedProduct(t, products, ws, "entwurf", StatusDraft, nil)

	for name, id := range map[string]int64{"sold out": soldOut.ID, "draft": draft.ID} {
		if err := carts.Add(ctx, cart.ID, id, 1); err != ErrNotOrderable {
			t.Errorf("%s: got %v, want ErrNotOrderable", name, err)
		}
	}
	got, _ := carts.Get(ctx, ws, cart.Token)
	if !got.Empty() {
		t.Errorf("basket holds %d lines, want none", len(got.Items))
	}
}

// Withdrawn and sold out differ. A product that was unpublished has no page to
// send anyone to, so its line goes; one that merely ran out stays visible and
// is refused at checkout, because quietly shortening a basket and charging for
// the rest is worse than saying so.
func TestWithdrawnLinesGoAndSoldOutLinesStay(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	cart, _, _ := carts.Ensure(ctx, ws, "")

	gone := seedProduct(t, products, ws, "zurueckgezogen", StatusPublished, nil)
	empty := seedProduct(t, products, ws, "ausverkauft", StatusPublished, intp(5))
	if err := carts.Add(ctx, cart.ID, gone.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := carts.Add(ctx, cart.ID, empty.ID, 1); err != nil {
		t.Fatal(err)
	}

	gone.Status = StatusDraft
	if err := products.Update(ctx, gone); err != nil {
		t.Fatal(err)
	}
	zero := 0
	empty.Stock = &zero
	if err := products.Update(ctx, empty); err != nil {
		t.Fatal(err)
	}

	got, _ := carts.Get(ctx, ws, cart.Token)
	if len(got.Items) != 1 {
		t.Fatalf("%d lines, want just the sold-out one", len(got.Items))
	}
	if got.Items[0].Product.Slug != "ausverkauft" {
		t.Errorf("the wrong line survived: %s", got.Items[0].Product.Slug)
	}
	if got.Items[0].Product.Orderable() {
		t.Error("the sold-out line still reports itself as orderable")
	}
}

// The two audiences are totalled by different arithmetic, and each total must
// be an exact multiple of the figure that audience was shown.
func TestTotalsFollowTheAudience(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil) // 49.00 brutto
	cart, _, _ := carts.Ensure(ctx, ws, "")
	if err := carts.Add(ctx, cart.ID, p.ID, 3); err != nil {
		t.Fatal(err)
	}

	set := settings()
	got, _ := carts.Get(ctx, ws, cart.Token)

	priv := got.Total(set, Private)
	if priv.ItemsGross != 14700 {
		t.Errorf("consumer items gross %d, want 14700 — three at 49.00", priv.ItemsGross)
	}

	got, _ = carts.Get(ctx, ws, cart.Token)
	biz := got.Total(set, Business)
	unitNet := money.NetFromGross(4900, money.RateStandard)
	if biz.ItemsNet != unitNet*3 {
		t.Errorf("business items net %d, want %d — three at the quoted net price",
			biz.ItemsNet, unitNet*3)
	}

	// Whatever the audience, the three figures must agree.
	for name, tot := range map[string]Totals{"privat": priv, "gewerblich": biz} {
		if tot.TotalNet+tot.TotalTax != tot.TotalGross {
			t.Errorf("%s: net %d + tax %d != gross %d",
				name, tot.TotalNet, tot.TotalTax, tot.TotalGross)
		}
		if tot.ItemsGross+tot.ShippingGross != tot.TotalGross {
			t.Errorf("%s: items and shipping do not add up to the total", name)
		}
	}
}

// "Versandkostenfrei ab CHF 200" is a promise about what the customer pays, so
// the threshold is compared against the gross value of the goods.
func TestShippingIsWaivedAboveTheThreshold(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	set := settings()

	cheap := seedProduct(t, products, ws, "brett", StatusPublished, nil) // 49.00
	cart, _, _ := carts.Ensure(ctx, ws, "")
	if err := carts.Add(ctx, cart.ID, cheap.ID, 1); err != nil {
		t.Fatal(err)
	}

	got, _ := carts.Get(ctx, ws, cart.Token)
	if tot := got.Total(set, Private); tot.ShippingGross != 1200 {
		t.Errorf("small basket: shipping %d, want 1200", tot.ShippingGross)
	}

	if err := carts.SetQuantity(ctx, cart.ID, cheap.ID, 5); err != nil { // 245.00
		t.Fatal(err)
	}
	got, _ = carts.Get(ctx, ws, cart.Token)
	tot := got.Total(set, Private)
	if tot.ShippingGross != 0 {
		t.Errorf("basket above the threshold: shipping %d, want none", tot.ShippingGross)
	}
	if tot.TotalGross != 24500 {
		t.Errorf("total %d, want 24500 with free delivery", tot.TotalGross)
	}
}

// An empty basket has no delivery charge — a shop that quotes postage on
// nothing is a shop that adds 12 francs to a page nobody has bought from.
func TestEmptyBasketCostsNothing(t *testing.T) {
	carts, _, ws := newCart(t)
	cart, _, _ := carts.Ensure(context.Background(), ws, "")

	tot := cart.Total(settings(), Private)
	if tot.TotalGross != 0 || tot.ShippingGross != 0 {
		t.Errorf("empty basket totals %+v, want zeroes", tot)
	}
}

// A shop below the VAT threshold shows no rate breakdown at all.
func TestExemptShopHasNoTaxBreakdown(t *testing.T) {
	carts, products, ws := newCart(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	cart, _, _ := carts.Ensure(ctx, ws, "")
	if err := carts.Add(ctx, cart.ID, p.ID, 2); err != nil {
		t.Fatal(err)
	}

	set := settings()
	set.VATExempt = true
	got, _ := carts.Get(ctx, ws, cart.Token)
	tot := got.Total(set, Private)

	if len(tot.Breakdown) != 0 {
		t.Errorf("exempt shop produced a tax breakdown: %+v", tot.Breakdown)
	}
	if tot.TotalTax != 0 {
		t.Errorf("exempt shop charged %d in tax", tot.TotalTax)
	}
	if tot.TotalNet != tot.TotalGross {
		t.Errorf("exempt shop: net %d and gross %d differ", tot.TotalNet, tot.TotalGross)
	}
}

// A cookie naming a basket that has been swept must produce a new one rather
// than an error — the visitor did nothing wrong.
func TestStaleTokenYieldsAFreshBasket(t *testing.T) {
	carts, _, ws := newCart(t)
	ctx := context.Background()

	cart, token, err := carts.Ensure(ctx, ws, "es-war-einmal")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if token == "es-war-einmal" {
		t.Error("a basket was adopted under a token nobody issued")
	}
	if cart.ID == 0 {
		t.Error("no basket was created")
	}

	again, same, err := carts.Ensure(ctx, ws, token)
	if err != nil {
		t.Fatal(err)
	}
	if same != token || again.ID != cart.ID {
		t.Error("a known token did not return its own basket")
	}
}
