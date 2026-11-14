package dsr

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// Erasure (Phase 33.2, DSR-03, D-03, D-06, D-07). Every helper of this file
// starts with erasetest.

// erasetestNow is the moment the tests erase at.
var erasetestNow = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

// erasetestOrder stores an order of 2026-03-01 with every field set and two
// mails of its own, on the website in the country.
func erasetestOrder(t *testing.T, database *db.DB, websiteID int64, number, email string) int64 {
	t.Helper()
	findOrder(t, database, websiteID, number, email)
	var id int64
	if err := database.Read.QueryRow(`SELECT id FROM orders WHERE website_id = $1 AND number = $2`,
		websiteID, number).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Write.Exec(`UPDATE orders SET return_policy = '30 Tage Rückgabe',
		payment_reference = 'PX-77', vat_number = 'CHE-123.456.789' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"order_confirmation", "order_operator"} {
		findExec(t, database, `INSERT INTO outbox (website_id, kind, order_id, recipient, subject, body,
			status, next_attempt_at, created_at) VALUES ($1, $2, $3, $4, 'Danke', 'Ada Lovelace, Bahnhofstrasse 1',
			'sent', '2026-03-01T10:00:01Z', '2026-03-01T10:00:01Z')`, websiteID, kind, id, email)
	}
	return id
}

// erasetestRow reads every column of one row as text, by column name.
func erasetestRow(t *testing.T, database *db.DB, table string, id int64) map[string]string {
	t.Helper()
	rows, err := database.Read.Query(`SELECT * FROM `+table+` WHERE id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		return nil
	}
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for i, c := range cols {
		if vals[i].Valid {
			out[c] = vals[i].String
		} else {
			out[c] = "NULL"
		}
	}
	return out
}

// erasetestDump is every row of the tables, as text, for "nothing changed".
func erasetestDump(t *testing.T, database *db.DB, tables ...string) string {
	t.Helper()
	var b strings.Builder
	for _, table := range tables {
		rows, err := database.Read.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			fmt.Fprintf(&b, "%s %v\n", table, vals)
		}
		rows.Close()
	}
	return b.String()
}

func erasetestItem(items []ErasureItem, source, key string) (ErasureItem, bool) {
	for _, it := range items {
		if it.Source == source && it.Key == key {
			return it, true
		}
	}
	return ErasureItem{}, false
}

func erasetestOrders(s *Service, sources ...string) EraseOptions {
	opt := EraseOptions{Email: "ada@example.com", Sources: map[string]bool{}, Now: erasetestNow}
	for _, src := range sources {
		opt.Sources[src] = true
	}
	return opt
}

func TestPreviewChangesNothing(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	expiretestCountry(t, database, a, "DE")
	erasetestOrder(t, database, a, "2026-0001", "Ada@Example.com")
	s := NewService(database)

	before := erasetestDump(t, database, "orders", "order_items", "outbox")
	p, err := s.Preview(context.Background(), a, "DE", "ada@example.com", erasetestNow)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if after := erasetestDump(t, database, "orders", "order_items", "outbox"); after != before {
		t.Errorf("the preview changed the database:\nbefore %s\nafter  %s", before, after)
	}
	it, ok := erasetestItem(p.Items, "orders", "2026-0001")
	if !ok {
		t.Fatalf("the preview does not list the order: %+v", p.Items)
	}
	if it.Action != "reduce" {
		t.Errorf("action %q, want reduce", it.Action)
	}
	if got, want := strings.Join(it.Gone, "|"), "E-mail address|Phone|Confirmation link|Order mails"; got != want {
		t.Errorf("gone %q, want %q", got, want)
	}
	if got, want := strings.Join(it.Kept, "|"),
		"Name|Company|VAT number|Address|Items|Amounts|Payment status|Promised returns policy|Remark"; got != want {
		t.Errorf("kept %q, want %q", got, want)
	}
	if it.Reason != orderReduced {
		t.Errorf("reason %q, want the order's disposition sentence", it.Reason)
	}
	if !strings.Contains(it.Basis, "§ 147 AO") {
		t.Errorf("basis %q, want Germany's", it.Basis)
	}
	if it.Until == nil || !it.Until.Equal(time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("until %v, want 2035-01-01", it.Until)
	}
}

func TestErasingAnOrderReducesIt(t *testing.T) {
	_, database := newTestStore(t)
	ctx := context.Background()
	a := website(t, database, "A")
	expiretestCountry(t, database, a, "DE")
	id := erasetestOrder(t, database, a, "2026-0001", "ada@example.com")
	s := NewService(database)
	before := erasetestRow(t, database, "orders", id)
	oldToken := before["token"]
	orders := shop.NewOrderStore(shop.NewCartStore(shop.NewStore(database)))

	rep, err := s.Erase(ctx, a, "DE", erasetestOrders(s, "orders"))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	after := erasetestRow(t, database, "orders", id)
	if after == nil {
		t.Fatal("the order was deleted; it must be reduced")
	}
	if after["email"] != "" || after["phone"] != "" {
		t.Errorf("email %q, phone %q, want both empty", after["email"], after["phone"])
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(after["token"]) || after["token"] == oldToken {
		t.Errorf("token %q, want a new 32-character lower-case hex value", after["token"])
	}
	if after["erased_at"] != "2026-11-14T10:00:00Z" {
		t.Errorf("erased_at %q", after["erased_at"])
	}
	if after["retain_until"] != "2035-01-01T00:00:00Z" {
		t.Errorf("retain_until %q, want 2035-01-01T00:00:00Z", after["retain_until"])
	}
	changed := map[string]bool{"email": true, "phone": true, "token": true, "erased_at": true,
		"retain_until": true, "updated_at": true}
	for col, was := range before {
		if changed[col] {
			continue
		}
		if after[col] != was {
			t.Errorf("column %s changed from %q to %q; a kept invoice field is never rewritten", col, was, after[col])
		}
	}
	var mails int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM outbox WHERE order_id = $1`, id).Scan(&mails); err != nil || mails != 0 {
		t.Errorf("%d mails of the order left (%v), want none", mails, err)
	}
	if o, err := orders.ByToken(ctx, a, oldToken); err != nil || o != nil {
		t.Errorf("the old confirmation link still opens the order: %v %v", o, err)
	}
	res, err := s.Find(ctx, a, "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.BySource()["orders"]); n != 0 {
		t.Errorf("the address still finds %d orders", n)
	}
	it, ok := erasetestItem(rep.Reduced, "orders", "2026-0001")
	if !ok || it.Until == nil || it.Until.Format("2006-01-02") != "2035-01-01" {
		t.Errorf("the report does not list the order as reduced until 2035-01-01: %+v", rep.Reduced)
	}

	// A Swiss seller whose operator keeps orders twelve years.
	b := website(t, database, "B")
	expiretestCountry(t, database, b, "CH")
	if err := s.SetPeriods(ctx, b, "CH", map[string]int{RetentionOrder: 12 * 12}); err != nil {
		t.Fatal(err)
	}
	idB := erasetestOrder(t, database, b, "2026-0002", "ada@example.com")
	if _, err := s.Erase(ctx, b, "CH", erasetestOrders(s, "orders")); err != nil {
		t.Fatal(err)
	}
	if got := erasetestRow(t, database, "orders", idB)["retain_until"]; got != "2039-01-01T00:00:00Z" {
		t.Errorf("CH, 12 years: retain_until %q, want 2039-01-01T00:00:00Z", got)
	}
}

func TestTwoErasedOrdersKeepUniqueTokens(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	one := erasetestOrder(t, database, a, "2026-0001", "ada@example.com")
	two := erasetestOrder(t, database, a, "2026-0002", "ADA@example.com")
	s := NewService(database)
	rep, err := s.Erase(context.Background(), a, "CH", erasetestOrders(s, "orders"))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if len(rep.Failed) != 0 || len(rep.Reduced) != 2 {
		t.Fatalf("reduced %d, failed %+v; want both reduced", len(rep.Reduced), rep.Failed)
	}
	r1, r2 := erasetestRow(t, database, "orders", one), erasetestRow(t, database, "orders", two)
	if r1["erased_at"] == "NULL" || r2["erased_at"] == "NULL" {
		t.Fatal("an order was not reduced")
	}
	if r1["token"] == r2["token"] {
		t.Errorf("both erased orders carry the token %q", r1["token"])
	}
}

func TestErasureNeverCrossesWebsites(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	erasetestOrder(t, database, a, "2026-0001", "ada@example.com")
	idB := erasetestOrder(t, database, b, "2026-0002", "ada@example.com")
	before := erasetestRow(t, database, "orders", idB)
	s := NewService(database)
	if _, err := s.Erase(context.Background(), a, "CH", erasetestOrders(s, "orders")); err != nil {
		t.Fatal(err)
	}
	after := erasetestRow(t, database, "orders", idB)
	for col, was := range before {
		if after[col] != was {
			t.Errorf("website B's order changed in %s: %q to %q", col, was, after[col])
		}
	}
	var mails int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM outbox WHERE order_id = $1`, idB).Scan(&mails); err != nil || mails != 2 {
		t.Errorf("website B's order has %d mails left (%v), want 2", mails, err)
	}
}
