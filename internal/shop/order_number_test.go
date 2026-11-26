package shop

import (
	"context"
	"testing"
	"time"
)

// placeOne places a one-piece order and returns it.
func placeOne(t *testing.T, orders *OrderStore, carts *CartStore, ws int64, p *Product) *Order {
	t.Helper()
	o, err := orders.Place(context.Background(), ws, settings(), Private,
		filled(t, carts, ws, p, 1), customer(), PayInvoice)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	return o
}

// renumber rewrites the number of a stored order, which is how a test puts the
// counter somewhere it would otherwise take ten thousand orders to reach.
func renumber(t *testing.T, carts *CartStore, id int64, number string) {
	t.Helper()
	if _, err := carts.DB.Write.Exec(
		`UPDATE orders SET number = $1 WHERE id = $2`, number, id); err != nil {
		t.Fatalf("renumber: %v", err)
	}
}

// The text order of "2026-9999" is above "2026-10000". The counter must follow
// the numeric order, or every checkout of the installation fails from the
// 10,000th order of a year until New Year.
func TestOrderNumberGrowsPast9999(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	fixed := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	orders.now = func() time.Time { return fixed }

	first := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, first.ID, "2026-9998")

	for _, want := range []string{"2026-9999", "2026-10000", "2026-10001"} {
		if got := placeOne(t, orders, carts, ws, p).Number; got != want {
			t.Fatalf("order number is %s, want %s", got, want)
		}
	}
}

// A year's numbers belong to that year: last year's, however high, do not
// carry over.
func TestOrderNumberRestartsInANewYear(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)

	orders.now = func() time.Time { return time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC) }
	a := placeOne(t, orders, carts, ws, p)
	b := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, a.ID, "2025-9999")
	renumber(t, carts, b.ID, "2025-10000")

	orders.now = func() time.Time { return time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC) }
	c := placeOne(t, orders, carts, ws, p)
	if c.Number != "2026-0001" {
		t.Errorf("first order of 2026 is %s, want 2026-0001", c.Number)
	}
	renumber(t, carts, c.ID, "2026-10001")

	orders.now = func() time.Time { return time.Date(2027, 1, 2, 10, 0, 0, 0, time.UTC) }
	if got := placeOne(t, orders, carts, ws, p).Number; got != "2027-0001" {
		t.Errorf("first order of 2027 is %s, want 2027-0001", got)
	}
}

// A stored number the generator cannot read must not send the count back to
// one: that is the collision the UNIQUE index would then report.
func TestOrderNumberIgnoresANumberItCannotRead(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	orders.now = func() time.Time { return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) }

	a := placeOne(t, orders, carts, ws, p)
	b := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, a.ID, "2026-0007")
	renumber(t, carts, b.ID, "2026-abc")

	if got := placeOne(t, orders, carts, ws, p).Number; got != "2026-0008" {
		t.Errorf("order number is %s, want 2026-0008", got)
	}
}
