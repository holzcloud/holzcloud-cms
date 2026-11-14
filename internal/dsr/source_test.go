package dsr

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// The privacy search (Phase 33.2, DSR-01). Every helper of this file starts
// with find, so the store tests beside it keep their names.

// findOrder stores an order with two item lines the way the shop writes one,
// and returns its number.
func findOrder(t *testing.T, database *db.DB, websiteID int64, number, email string) string {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO orders (website_id, number, token, audience, currency,
		email, name, company, vat_number, phone, street, postal_code, city, country, note,
		items_net, items_tax, items_gross, shipping_net, shipping_tax, shipping_gross,
		total_net, total_tax, total_gross, status, payment_method, payment_status,
		created_at, updated_at)
		VALUES ($1, $2, $3, 'private', 'CHF', $4, 'Ada Lovelace', 'Analytical Engines', '', '+41 79 000 00 00',
		'Bahnhofstrasse 1', '8001', 'Zürich', 'CH', 'Bitte klingeln',
		4000, 0, 4000, 900, 0, 900, 4900, 0, 4900, 'paid', 'invoice', 'paid',
		'2026-03-01T10:00:00Z', '2026-03-01T10:00:00Z')`,
		websiteID, number, "secret-token-"+number, email)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	orderID, _ := res.LastInsertId()
	for i, item := range []struct {
		title string
		qty   int
		gross int
	}{{"Holzbrett", 2, 1500}, {"Kochlöffel", 1, 1000}} {
		if _, err := database.Write.Exec(`INSERT INTO order_items (order_id, title, quantity, tax_bp,
			unit_gross, unit_net, line_net, line_tax, line_gross, position)
			VALUES ($1, $2, $3, 0, $4, $4, $5, 0, $5, $6)`,
			orderID, item.title, item.qty, item.gross, item.gross*item.qty, i); err != nil {
			t.Fatalf("insert order item: %v", err)
		}
	}
	return number
}

func findField(rec Record, name string) (Field, bool) {
	for _, f := range rec.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

func TestAnOrderIsFoundByItsAddress(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	number := findOrder(t, database, a, "2026-0001", "Ada@Example.COM")

	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	orders := res.BySource()["orders"]
	if len(orders) != 1 {
		t.Fatalf("found %d orders, want 1: %+v", len(orders), res.Records)
	}
	rec := orders[0]
	if rec.Source != "orders" || rec.Key != number || rec.Kind != "order" {
		t.Errorf("record = %q/%q/%q, want orders/%s/order", rec.Source, rec.Key, rec.Kind, number)
	}
	if want := "/admin/websites/" + itoa(a) + "/bestellungen/" + number; rec.Link != want {
		t.Errorf("Link = %q, want %q", rec.Link, want)
	}
	if rec.Time.IsZero() {
		t.Error("the record has no time")
	}
	for name, want := range map[string]string{
		"name":           "Ada Lovelace",
		"company":        "Analytical Engines",
		"street":         "Bahnhofstrasse 1",
		"postal_code":    "8001",
		"city":           "Zürich",
		"country":        "CH",
		"phone":          "+41 79 000 00 00",
		"note":           "Bitte klingeln",
		"status":         "paid",
		"payment_status": "paid",
	} {
		f, ok := findField(rec, name)
		if !ok {
			t.Errorf("the record has no field %q", name)
			continue
		}
		if f.Value != want {
			t.Errorf("field %q = %q, want %q", name, f.Value, want)
		}
		if f.Label == "" {
			t.Errorf("field %q has no label", name)
		}
	}
	if f, ok := findField(rec, "total"); !ok || !strings.Contains(f.Value, "49") {
		t.Errorf("total = %+v, want the order's total of 49", f)
	}
	items := 0
	for _, f := range rec.Fields {
		if strings.HasPrefix(f.Name, "item_") {
			items++
		}
	}
	if items != 2 {
		t.Errorf("%d item fields, want one per line (2)", items)
	}
	for _, f := range rec.Fields {
		if strings.Contains(f.Value, "secret-token") || f.Name == "token" {
			t.Errorf("the record carries the confirmation token in %q", f.Name)
		}
	}
}

// findCountingSource counts how often it is asked.
type findCountingSource struct{ calls int }

func (s *findCountingSource) Name() string { return "counting" }
func (s *findCountingSource) Find(context.Context, int64, string) ([]Record, error) {
	s.calls++
	return nil, nil
}

func TestFindRefusesWhatIsNotAnAddress(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	svc := NewService(database)
	counter := &findCountingSource{}
	svc.extra = []Source{counter}
	for _, bad := range []string{"", "   ", "ada@example.com?bcc=x"} {
		if _, err := svc.Find(context.Background(), a, bad); !errors.Is(err, ErrBadEmail) {
			t.Errorf("Find(%q) = %v, want ErrBadEmail", bad, err)
		}
	}
	if counter.calls != 0 {
		t.Errorf("a source was asked %d times for something that is not an address", counter.calls)
	}
	if _, err := svc.Find(context.Background(), a, "  ada@example.com "); err != nil {
		t.Errorf("Find with white space around a valid address = %v, want it trimmed", err)
	}
	if counter.calls != 1 {
		t.Errorf("the source was asked %d times for a valid address, want 1", counter.calls)
	}
}

// findFailingSource always fails.
type findFailingSource struct{}

func (findFailingSource) Name() string { return "broken" }
func (findFailingSource) Find(context.Context, int64, string) ([]Record, error) {
	return nil, errors.New("disk on fire")
}

func TestAFailingSourceIsNamedAndTheWalkGoesOn(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	findOrder(t, database, a, "2026-0002", "ada@example.com")
	svc := NewService(database)
	svc.extra = []Source{findFailingSource{}}
	res, err := svc.Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(res.Failed) != 1 || res.Failed[0].Source != "broken" {
		t.Errorf("Failed = %+v, want the broken source named", res.Failed)
	}
	if len(res.BySource()["orders"]) != 1 {
		t.Error("the walk stopped at the failing source")
	}
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
