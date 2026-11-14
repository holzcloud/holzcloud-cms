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

// findExec runs a statement and fails the test on an error.
func findExec(t *testing.T, database *db.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := database.Write.Exec(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// findSeed stores the address in every core store of one website, in the
// letter case given: an order, a form message, a privacy request, an order
// mail and a queued mail.
func findSeed(t *testing.T, database *db.DB, websiteID int64, number, email string) {
	t.Helper()
	findOrder(t, database, websiteID, number, email)
	findExec(t, database, `INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada', $2, 'Frage', 'Habt ihr Eichenbretter?', '2026-02-01T09:00:00Z')`, websiteID, email)
	findExec(t, database, `INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, created_at)
		VALUES ($1, 'optout', 'de', 'Ada', $2, 'Nicht verkaufen', '2026-02-02T09:00:00Z')`, websiteID, email)
	findExec(t, database, `INSERT INTO outbox (website_id, kind, recipient, subject, body, next_attempt_at, created_at)
		VALUES ($1, 'order_shipped', $2, 'Versandt', 'Ihre Bestellung ist unterwegs', '2026-03-02T09:00:00Z', '2026-03-02T09:00:00Z')`,
		websiteID, email)
	findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, subject, body, created_at)
		VALUES ($1, $2, 'Kopie Ihrer Nachricht', 'Habt ihr Eichenbretter?', '2026-02-01T09:00:01Z')`, websiteID, email)
}

func TestFindStaysOnItsWebsite(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	findSeed(t, database, a, "2026-0101", "ada@example.com")
	findSeed(t, database, b, "2026-0102", "ada@example.com")
	// An invitation belongs to no website and is on neither screen.
	findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, subject, body)
		VALUES (NULL, 'ada@example.com', 'Einladung', 'Willkommen')`)

	svc := NewService(database)
	for _, site := range []struct {
		id     int64
		number string
		other  string
	}{{a, "2026-0101", "2026-0102"}, {b, "2026-0102", "2026-0101"}} {
		res, err := svc.Find(context.Background(), site.id, "ada@example.com")
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		by := res.BySource()
		for _, source := range []string{"orders", "form_messages", "requests"} {
			if len(by[source]) != 1 {
				t.Errorf("website %d: %d records in %s, want its own 1", site.id, len(by[source]), source)
			}
		}
		// The order mail, the queued mail; never the invitation.
		if len(by["mail"]) != 2 {
			t.Errorf("website %d: %d mails, want its own 2: %+v", site.id, len(by["mail"]), by["mail"])
		}
		for _, rec := range res.Records {
			if rec.Key == site.other {
				t.Errorf("website %d shows the other website's order", site.id)
			}
			if f, ok := findField(rec, "subject"); ok && f.Value == "Einladung" {
				t.Errorf("website %d shows an installation mail", site.id)
			}
		}
	}
}

func TestFindIgnoresLetterCase(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	findSeed(t, database, a, "2026-0201", "Ada@Example.COM")
	// A plugin's notification carries the sender's address as Reply-To.
	findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, reply_to, subject, body)
		VALUES ($1, 'operator@example.org', 'ADA@example.com', 'Neue Nachricht', 'Habt ihr Eichenbretter?')`, a)

	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	by := res.BySource()
	for source, want := range map[string]int{"orders": 1, "form_messages": 1, "requests": 1, "mail": 3} {
		if len(by[source]) != want {
			t.Errorf("%s: %d records, want %d", source, len(by[source]), want)
		}
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %+v", res.Failed)
	}
}

// findUser stores a staff account with every secret the table can hold.
func findUser(t *testing.T, database *db.DB, email, name, role string, limited bool, websites ...int64) int64 {
	t.Helper()
	lim := 0
	if limited {
		lim = 1
	}
	id := findExec(t, database, `INSERT INTO users (email, password, role, name, websites_limited,
		totp_secret, totp_pending_secret, last_login_at, created_at)
		VALUES ($1, '$argon2id$v=19$secret-hash', $2, $3, $4, 'TOTPSECRET', 'TOTPPENDING',
		'2026-05-01T07:00:00Z', '2025-01-01T07:00:00Z')`, email, role, name, lim)
	for _, w := range websites {
		findExec(t, database, `INSERT INTO user_websites (user_id, website_id) VALUES ($1, $2)`, id, w)
	}
	return id
}

func TestAnAccountShowsOnlyWhereItWorks(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	findUser(t, database, "Chef@Example.com", "Chefin", "admin", false)
	findUser(t, database, "Alle@Example.com", "Alle", "editor", false)
	findUser(t, database, "Nur-B@Example.com", "Nur B", "editor", true, b)
	// An editor never limited by the flag but given a list of websites is
	// limited by that list, the rule internal/user applies.
	findUser(t, database, "Liste-B@Example.com", "Liste B", "editor", false, b)

	svc := NewService(database)
	for _, c := range []struct {
		site  int64
		email string
		want  bool
	}{
		{a, "chef@example.com", true},
		{a, "alle@example.com", true},
		{a, "nur-b@example.com", false},
		{a, "liste-b@example.com", false},
		{b, "chef@example.com", true},
		{b, "alle@example.com", true},
		{b, "nur-b@example.com", true},
		{b, "liste-b@example.com", true},
	} {
		res, err := svc.Find(context.Background(), c.site, c.email)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		accounts := res.BySource()["accounts"]
		if got := len(accounts) == 1; got != c.want {
			t.Errorf("website %d, %s: listed = %v, want %v", c.site, c.email, got, c.want)
			continue
		}
		if !c.want {
			continue
		}
		rec := accounts[0]
		if rec.Kind != "account" || !strings.HasPrefix(rec.Link, "/admin/users/") {
			t.Errorf("record = %+v", rec)
		}
		for _, name := range []string{"name", "email", "role", "created_at", "last_login_at", "activity_entries"} {
			if _, ok := findField(rec, name); !ok {
				t.Errorf("%s: the account has no field %q", c.email, name)
			}
		}
		for _, f := range rec.Fields {
			for _, secret := range []string{"secret-hash", "TOTPSECRET", "TOTPPENDING"} {
				if strings.Contains(f.Value, secret) {
					t.Errorf("%s: field %q carries a secret", c.email, f.Name)
				}
			}
		}
	}
}

func TestOrderMailsComeWithTheOrder(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	number := findOrder(t, database, a, "2026-0301", "ada@example.com")
	var orderID int64
	if err := database.Read.QueryRow(`SELECT id FROM orders WHERE number = $1`, number).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	// The operator's copy goes to the operator, and carries the customer's
	// address in its body.
	findExec(t, database, `INSERT INTO outbox (website_id, kind, order_id, recipient, subject, body, next_attempt_at, created_at)
		VALUES ($1, 'order_operator', $2, 'shop@example.org', 'Neue Bestellung', 'Ada Lovelace, ada@example.com', '2026-03-01T10:00:00Z', '2026-03-01T10:00:00Z')`,
		a, orderID)

	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	mails := res.BySource()["mail"]
	if len(mails) != 1 {
		t.Fatalf("%d mails, want the operator's copy of the order", len(mails))
	}
	if f, ok := findField(mails[0], "body"); !ok || !strings.Contains(f.Value, "Ada Lovelace") {
		t.Errorf("the mail does not carry its body: %+v", mails[0].Fields)
	}
}

// findPluginRows installs a plugin record and gives it entries on a website.
func findPluginRows(t *testing.T, database *db.DB, id string, websiteID int64, n int) {
	t.Helper()
	findExec(t, database, `INSERT OR IGNORE INTO plugins (id, name, version, manifest, sha256, installed_at, updated_at)
		VALUES ($1, $1, '1.0.0', '{}', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, id)
	for i := 0; i < n; i++ {
		findExec(t, database, `INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at)
			VALUES ($1, $2, $3, '{}', '2026-01-01T00:00:00Z')`, id, websiteID, "nachricht:"+strconv.Itoa(i))
	}
}

func TestPluginsWithDataAreReported(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	findPluginRows(t, database, "kontaktformular", a, 3)
	findPluginRows(t, database, "kontaktformular", b, 1)
	findPluginRows(t, database, "kontaktformular", 0, 1)

	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(res.Holders) != 1 {
		t.Fatalf("Holders = %+v, want kontaktformular alone", res.Holders)
	}
	h := res.Holders[0]
	if h.PluginID != "kontaktformular" || h.Entries != 3 || h.Covered || h.Name == "" {
		t.Errorf("holder = %+v, want kontaktformular with A's 3 entries, not covered", h)
	}
	if h.Reason != "The plugin is not running on this server." {
		t.Errorf("Reason with no manager = %q", h.Reason)
	}
	if len(res.NotCovered()) != 1 {
		t.Errorf("NotCovered = %+v", res.NotCovered())
	}
}
