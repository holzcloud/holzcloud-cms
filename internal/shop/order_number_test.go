package shop

import (
	"context"
	"strings"
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

// collideOn installs a trigger that makes the INSERT of an order with one of the
// given numbers collide for real: before the row goes in, another row with the
// same number is written, so the statement then fails on the UNIQUE index. The
// statement's failure takes the trigger's row back with it, which leaves the
// table as it was and is what a concurrent writer that rolled back would look
// like.
//
// The column list is read from the table, so a column a later migration adds
// does not have to be remembered here.
func collideOn(t *testing.T, carts *CartStore, numbers ...string) {
	t.Helper()

	rows, err := carts.DB.Read.Query(`SELECT name FROM pragma_table_info('orders') WHERE name <> 'id'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols, vals []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
		switch name {
		case "token":
			vals = append(vals, "NEW.token || '-collision'")
		default:
			vals = append(vals, "NEW."+name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	var in string
	for i, n := range numbers {
		if i > 0 {
			in += ", "
		}
		in += "'" + n + "'"
	}
	stmt := `CREATE TRIGGER collide_order_number BEFORE INSERT ON orders
		WHEN NEW.number IN (` + in + `)
		 AND NOT EXISTS (SELECT 1 FROM orders WHERE number = NEW.number)
		BEGIN
			INSERT INTO orders (` + strings.Join(cols, ", ") + `)
			VALUES (` + strings.Join(vals, ", ") + `);
		END`
	if _, err := carts.DB.Write.Exec(stmt); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}

// With the numeric maximum a collision should not happen. If one does, because
// of a row the maximum did not see, the order is retried once with a higher
// number instead of failing the customer's checkout.
func TestOrderNumberCollisionIsRetriedOnce(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	orders.now = func() time.Time { return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) }

	first := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, first.ID, "2026-9999")
	collideOn(t, carts, "2026-10000")

	o := placeOne(t, orders, carts, ws, p)
	if o.Number != "2026-10001" {
		t.Errorf("order number is %s, want 2026-10001 after the retry", o.Number)
	}

	var n int
	if err := carts.DB.Read.QueryRow(
		`SELECT count(*) FROM orders WHERE token = $1 AND number = $2`,
		o.Token, o.Number).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d stored rows carry the returned number and token, want 1", n)
	}
	if err := carts.DB.Read.QueryRow(
		`SELECT count(*) FROM orders WHERE number = '2026-10000'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("the collided number 2026-10000 was stored %d times", n)
	}
}

// A second collision is not retried: a table that keeps refusing numbers must
// not loop on the only write connection. The order fails and takes nothing with
// it.
func TestOrderNumberSecondCollisionFailsCleanly(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	ctx := context.Background()
	p := seedProduct(t, products, ws, "hocker", StatusPublished, intp(5))
	orders.now = func() time.Time { return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) }

	// Keep the basket for the failing call: placeOne would empty it.
	first := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, first.ID, "2026-9999")
	collideOn(t, carts, "2026-10000", "2026-10001")

	cart := filled(t, carts, ws, p, 1)
	if _, err := orders.Place(ctx, ws, settings(), Private, cart, customer(), PayInvoice); err == nil {
		t.Fatal("the order went through although both numbers collided")
	}

	after, err := products.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stock == nil || *after.Stock != 4 {
		t.Errorf("stock is %v, want 4: only the first order may have taken a piece", after.Stock)
	}
	again, err := carts.Get(ctx, ws, cart.Token)
	if err != nil {
		t.Fatal(err)
	}
	if again.Empty() {
		t.Error("the basket was emptied although the order failed")
	}
	var n int
	if err := carts.DB.Read.QueryRow(
		`SELECT count(*) FROM orders WHERE number IN ('2026-10000', '2026-10001')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d orders with a collided number remain", n)
	}
}

// A hand-edited number of nineteen or more digits is clamped to the largest
// integer by SQLite, and one more than that wraps negative. A negative suffix
// or a huge one must neither wrap the counter nor stick every later checkout
// on the UNIQUE index: only a suffix of one to nine digits counts.
func TestOrderNumberIgnoresAHugeOrNegativeNumber(t *testing.T) {
	orders, carts, products, ws := newOrders(t)
	p := seedProduct(t, products, ws, "hocker", StatusPublished, nil)
	orders.now = func() time.Time { return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) }

	a := placeOne(t, orders, carts, ws, p)
	b := placeOne(t, orders, carts, ws, p)
	c := placeOne(t, orders, carts, ws, p)
	renumber(t, carts, a.ID, "2026-9999999999999999999")
	renumber(t, carts, b.ID, "2026--5")
	renumber(t, carts, c.ID, "2026-0012")

	for _, want := range []string{"2026-0013", "2026-0014", "2026-0015"} {
		if got := placeOne(t, orders, carts, ws, p).Number; got != want {
			t.Fatalf("order number is %s, want %s", got, want)
		}
	}

	// With nothing sane stored, the year starts at one.
	if _, err := carts.DB.Write.Exec(`UPDATE orders SET number = '2026-99999999999999999999' || id`); err != nil {
		t.Fatal(err)
	}
	if got := placeOne(t, orders, carts, ws, p).Number; got != "2026-0001" {
		t.Errorf("order number is %s, want 2026-0001", got)
	}
}
