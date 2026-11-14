package dsr

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/user"
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

// erasetestCount counts the rows a query finds.
func erasetestCount(t *testing.T, database *db.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func erasetestAll() map[string]bool {
	return map[string]bool{EraseOrders: true, EraseFormMessages: true, EraseMail: true, ErasePlugins: true, EraseAccounts: true}
}

func TestErasureDeletesFormMessagesAndMail(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	for _, w := range []int64{a, b} {
		findExec(t, database, `INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
			VALUES ($1, 'Ada', 'Ada@Example.com', 'Frage', 'Habt ihr Eichenbretter?', '2026-02-01T09:00:00Z')`, w)
		findExec(t, database, `INSERT INTO outbox (website_id, kind, recipient, subject, body, next_attempt_at, created_at)
			VALUES ($1, 'order_shipped', 'ada@example.com', 'Versandt', 'Unterwegs', '2026-03-02T09:00:00Z', '2026-03-02T09:00:00Z')`, w)
		findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, subject, body, created_at)
			VALUES ($1, 'ada@example.com', 'Kopie Ihrer Nachricht', 'Text', '2026-02-01T09:00:01Z')`, w)
		findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, reply_to, subject, body, created_at)
			VALUES ($1, 'shop@example.com', 'ADA@example.com', 'Neue Nachricht', 'Text', '2026-02-01T09:00:02Z')`, w)
	}
	// The installation's own mail to the same address: an invitation.
	findExec(t, database, `INSERT INTO mail_outbox (website_id, recipient, subject, body, created_at)
		VALUES (NULL, 'ada@example.com', 'Einladung', 'Text', '2026-02-01T09:00:03Z')`)

	s := NewService(database)
	rep, err := s.Erase(context.Background(), a, "CH", erasetestOrders(s, EraseFormMessages, EraseMail))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if len(rep.Failed) != 0 {
		t.Fatalf("failed: %+v", rep.Failed)
	}
	if rep.Deleted[EraseFormMessages] != 1 || rep.Deleted[EraseMail] != 3 {
		t.Errorf("deleted %v, want 1 form message and 3 mails", rep.Deleted)
	}
	for table, want := range map[string]int{"form_messages": 0, "outbox": 0, "mail_outbox": 0} {
		if n := erasetestCount(t, database, `SELECT COUNT(*) FROM `+table+` WHERE website_id = $1`, a); n != want {
			t.Errorf("website A: %d rows left in %s, want %d", n, table, want)
		}
	}
	for table, want := range map[string]int{"form_messages": 1, "outbox": 1, "mail_outbox": 2} {
		if n := erasetestCount(t, database, `SELECT COUNT(*) FROM `+table+` WHERE website_id = $1`, b); n != want {
			t.Errorf("website B: %d rows left in %s, want %d", n, table, want)
		}
	}
	if n := erasetestCount(t, database, `SELECT COUNT(*) FROM mail_outbox WHERE website_id IS NULL`); n != 1 {
		t.Errorf("the installation's own mail was touched: %d left, want 1", n)
	}
}

// erasetestPlugins is a service with both shipped plugins on the website and
// the media store under dir.
func erasetestPlugins(t *testing.T, database *db.DB, websites ...int64) (*Service, string, func(id string, websiteID int64, key string, value any), string) {
	t.Helper()
	m, store := plugintestManager(t, database)
	contact := plugintestInstall(t, m, "kontaktformular", websites...)
	shopID := plugintestInstall(t, m, "bestellung", websites...)
	dir := t.TempDir()
	s := NewService(database)
	s.SetPlugins(m)
	s.SetMedia(media.NewStore(database), dir)
	seed := func(id string, websiteID int64, key string, value any) {
		plugintestSeed(t, store, id, websiteID, key, value)
	}
	if shopID != "bestellung" {
		t.Fatalf("the farm shop installed as %q", shopID)
	}
	return s, dir, seed, contact
}

// erasetestStored reads one value of a plugin's store, "" when it is gone.
func erasetestStored(t *testing.T, database *db.DB, id string, websiteID int64, key string) string {
	t.Helper()
	var v string
	err := database.Read.QueryRow(`SELECT value FROM plugin_store WHERE plugin_id = $1 AND website_id = $2 AND key = $3`,
		id, websiteID, key).Scan(&v)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestErasureOfPluginRecords(t *testing.T) {
	_, database := newTestStore(t)
	ctx := context.Background()
	a := website(t, database, "A")
	expiretestCountry(t, database, a, "DE")
	s, dir, seed, contact := erasetestPlugins(t, database, a)

	file := expiretestAttachment(t, database, dir, a, "brief.pdf")
	msgKey, msg := expiretestMessage(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), "aaaaaaaa", file)
	seed(contact, a, msgKey, msg)
	const quarantined = "abgewiesen:2026-10-02T09:00:00Z-bbbbbbbb"
	seed(contact, a, quarantined, map[string]any{
		"kennung": "2026-10-02T09:00:00Z-bbbbbbbb", "name": "Ada", "email": "ada@example.com",
		"text": "Noch eine Frage", "zeit": "2026-10-02T09:00:00Z", "abgewiesen": "honeypot",
	})
	const farm = "bestellung:20260301T090000-cccccccc"
	seed("bestellung", a, farm, map[string]any{
		"id": "20260301T090000-cccccccc", "eingegangen": "2026-03-01T09:00:00Z",
		"name": "Ada Lovelace", "email": "Ada@Example.com", "telefon": "+41 79 000 00 00",
		"adresse": "Bahnhofstrasse 1", "bemerkung": "Bitte klingeln",
		"posten":   []map[string]any{{"slug": "seife", "titel": "Schafmilchseife", "menge": 2, "preis": "8,50"}},
		"waehrung": "CHF", "summe": 17, "summe_bekannt": true,
	})

	prev, err := s.Preview(ctx, a, "DE", "ada@example.com", erasetestNow)
	if err != nil {
		t.Fatal(err)
	}
	if it, ok := erasetestItem(prev.Items, "plugin:bestellung", farm); !ok || it.Action != ActionReduce ||
		it.Until == nil || it.Until.Format("2006-01-02") != "2035-01-01" {
		t.Errorf("the preview does not reduce the farm-shop order until 2035-01-01: %+v", it)
	}
	if it, ok := erasetestItem(prev.Items, "plugin:kontaktformular", quarantined); !ok || it.Action != ActionDelete {
		t.Errorf("the preview does not delete the quarantined submission: %+v", it)
	}

	rep, err := s.Erase(ctx, a, "DE", erasetestOrders(s, ErasePlugins))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if len(rep.Failed) != 0 || len(rep.FileErrors) != 0 {
		t.Fatalf("failed %+v, file errors %v", rep.Failed, rep.FileErrors)
	}
	if rep.Deleted[ErasePlugins] != 2 {
		t.Errorf("deleted %v, want the message and the quarantined submission", rep.Deleted)
	}
	for _, key := range []string{msgKey, quarantined} {
		if v := erasetestStored(t, database, contact, a, key); v != "" {
			t.Errorf("%s is still stored", key)
		}
	}
	if n := erasetestCount(t, database, `SELECT COUNT(*) FROM media WHERE id = $1`, file); n != 0 {
		t.Error("the attachment's media row is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", strconv.FormatInt(a, 10), "brief.pdf")); !os.IsNotExist(err) {
		t.Errorf("the attachment's file is still on the disk: %v", err)
	}

	var order map[string]any
	if err := json.Unmarshal([]byte(erasetestStored(t, database, "bestellung", a, farm)), &order); err != nil {
		t.Fatalf("the farm-shop order is gone or unreadable: %v", err)
	}
	if order["email"] != "" && order["email"] != nil || order["telefon"] != nil && order["telefon"] != "" {
		t.Errorf("the farm-shop order still carries e-mail or phone: %v", order)
	}
	if order["name"] != "Ada Lovelace" || order["bemerkung"] != "Bitte klingeln" {
		t.Errorf("the farm-shop order lost a kept field: %v", order)
	}
	if order["aufbewahren_bis"] != "2035-01-01T00:00:00Z" {
		t.Errorf("aufbewahren_bis %v, want 2035-01-01T00:00:00Z", order["aufbewahren_bis"])
	}
	it, ok := erasetestItem(rep.Reduced, "plugin:bestellung", farm)
	if !ok || it.Until == nil || it.Until.Format("2006-01-02") != "2035-01-01" || len(it.Kept) == 0 {
		t.Errorf("the report does not list the farm-shop order as reduced with its kept fields: %+v", rep.Reduced)
	}
	res, err := s.Find(ctx, a, "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.Records); n != 0 {
		t.Errorf("the address still finds %d records: %+v", n, res.Records)
	}
}

func TestAFileThatStaysIsNamed(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	s, dir, seed, contact := erasetestPlugins(t, database, a)
	file := expiretestAttachment(t, database, dir, a, "brief.pdf")
	key, msg := expiretestMessage(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), "aaaaaaaa", file)
	seed(contact, a, key, msg)
	// A directory nobody may delete from: the file stays where it is.
	folder := filepath.Join(dir, "media", strconv.FormatInt(a, 10))
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o755) })

	rep, err := s.Erase(context.Background(), a, "CH", erasetestOrders(s, ErasePlugins))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if rep.Deleted[ErasePlugins] != 1 {
		t.Errorf("deleted %v, want the message counted as erased", rep.Deleted)
	}
	if len(rep.FileErrors) != 1 || rep.FileErrors[0] != "brief.pdf" {
		t.Errorf("file errors %v, want brief.pdf named", rep.FileErrors)
	}
}

func TestAForeignMediaIDIsRefused(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	b := website(t, database, "B")
	s, dir, seed, contact := erasetestPlugins(t, database, a, b)
	foreign := expiretestAttachment(t, database, dir, b, "fremd.pdf")
	key, msg := expiretestMessage(time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), "aaaaaaaa", foreign)
	seed(contact, a, key, msg)
	findExec(t, database, `INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada', 'ada@example.com', 'Frage', 'Text', '2026-02-01T09:00:00Z')`, a)

	rep, err := s.Erase(context.Background(), a, "CH", erasetestOrders(s, ErasePlugins, EraseFormMessages))
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if n := erasetestCount(t, database, `SELECT COUNT(*) FROM media WHERE id = $1`, foreign); n != 1 {
		t.Error("website B's media row was deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", strconv.FormatInt(b, 10), "fremd.pdf")); err != nil {
		t.Errorf("website B's file is gone: %v", err)
	}
	found := false
	for _, f := range rep.Failed {
		if f.Source == "plugin:kontaktformular" && ErasureFailure(f.Err) == eraseFailedForeignFile {
			found = true
		}
	}
	if !found {
		t.Errorf("the foreign file is not reported: %+v", rep.Failed)
	}
	if rep.Deleted[EraseFormMessages] != 1 || rep.Deleted[ErasePlugins] != 1 {
		t.Errorf("deleted %v: the rest of the erasure did not go on", rep.Deleted)
	}
}

func TestRequestsAreKeptAndSaySo(t *testing.T) {
	_, database := newTestStore(t)
	ctx := context.Background()
	a := website(t, database, "A")
	answered := expiretestRequest(t, database, a, KindErasure,
		time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC))
	open := expiretestRequest(t, database, a, KindOptOut, time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC), time.Time{})
	s := NewService(database)

	prev, err := s.Preview(ctx, a, "CH", "ada@example.com", erasetestNow)
	if err != nil {
		t.Fatal(err)
	}
	for id, until := range map[int64]string{answered: "2028-02-01", open: "2028-05-01"} {
		it, ok := erasetestItem(prev.Items, "requests", strconv.FormatInt(id, 10))
		if !ok || it.Action != ActionKeep || it.Reason != requestsKept {
			t.Errorf("request %d: %+v, want kept with the request sentence", id, it)
			continue
		}
		if it.Until == nil || it.Until.Format("2006-01-02") != until {
			t.Errorf("request %d kept until %v, want %s", id, it.Until, until)
		}
	}
	if !strings.Contains(requestsKept, "§ 7101") || !strings.Contains(requestsKept, "§ 7022(e)") {
		t.Errorf("the request sentence does not name its law: %q", requestsKept)
	}

	opt := erasetestOrders(s)
	opt.Sources = erasetestAll()
	rep, err := s.Erase(ctx, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	if n := erasetestCount(t, database, `SELECT COUNT(*) FROM privacy_requests WHERE website_id = $1`, a); n != 2 {
		t.Errorf("%d requests left, want both: the record of a request is never erased", n)
	}
	if _, ok := erasetestItem(rep.Kept, "requests", strconv.FormatInt(answered, 10)); !ok {
		t.Errorf("the report does not say the request was kept: %+v", rep.Kept)
	}
}

func TestAccountsOnlyWhenChosenAndNeverYourOwn(t *testing.T) {
	s, database, sm, store := installationService(t)
	ctx := context.Background()
	a := website(t, database, "A")
	staff := installationAccount(t, database, "ada@example.com", user.RoleEditor, erasetestNow.AddDate(-1, 0, 0), nil, a)
	token := installationSession(t, sm, staff)
	chef := installationAccount(t, database, "chef@example.com", user.RoleAdmin, erasetestNow.AddDate(-1, 0, 0), nil)

	exists := func(id int64) bool {
		return erasetestCount(t, database, `SELECT COUNT(*) FROM users WHERE id = $1`, id) == 1
	}

	// Not ticked: nothing happens to the account.
	opt := erasetestOrders(s, EraseOrders)
	opt.ActorUserID = chef
	if _, err := s.Erase(ctx, a, "CH", opt); err != nil {
		t.Fatal(err)
	}
	if !exists(staff) {
		t.Fatal("an account that was not chosen was deleted")
	}

	// Ticked by somebody who may not: kept, and the report says why.
	opt = erasetestOrders(s, EraseAccounts)
	opt.ActorUserID = chef
	rep, err := s.Erase(ctx, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(staff) {
		t.Fatal("the account was deleted without Accounts")
	}
	if it, ok := erasetestItem(rep.Kept, "accounts", strconv.FormatInt(staff, 10)); !ok || it.Reason != eraseAccountsAdminOnly {
		t.Errorf("the report does not say only an administrator erases an account: %+v", rep.Kept)
	}

	// Never one's own.
	opt.Accounts = true
	opt.ActorUserID = staff
	rep, err = s.Erase(ctx, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(staff) || len(rep.Failed) != 1 || ErasureFailure(rep.Failed[0].Err) != eraseFailedOwnAccount {
		t.Errorf("one's own account: exists %v, failed %+v; want kept and refused", exists(staff), rep.Failed)
	}

	// Chosen by an administrator: gone, with its sessions; the activity log
	// keeps the address it acted under.
	opt.ActorUserID = chef
	rep, err = s.Erase(ctx, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	if exists(staff) || rep.Deleted[EraseAccounts] != 1 {
		t.Fatalf("the chosen account: exists %v, deleted %v", exists(staff), rep.Deleted)
	}
	if _, found, _ := store.Find(token); found {
		t.Error("the erased account's session still works")
	}
	if n := erasetestCount(t, database, `SELECT COUNT(*) FROM activity_log WHERE actor_email = 'ada@example.com'`); n != 1 {
		t.Errorf("%d activity entries keep the staff member's address, want 1", n)
	}

	// The last administrator stays.
	b := website(t, database, "B")
	opt = erasetestOrders(s, EraseAccounts)
	opt.Email, opt.Accounts = "chef@example.com", true
	rep, err = s.Erase(ctx, b, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(chef) || len(rep.Failed) != 1 || ErasureFailure(rep.Failed[0].Err) != eraseFailedLastAdmin {
		t.Errorf("the last administrator: exists %v, failed %+v; want kept and refused", exists(chef), rep.Failed)
	}
}

func TestTheRequestIsAnswered(t *testing.T) {
	st, database := newTestStore(t)
	ctx := context.Background()
	a := website(t, database, "A")
	b := website(t, database, "B")
	s := NewService(database)
	in := i18n.WithLang(ctx, i18n.Source)
	request := func(websiteID int64, kind string) int64 {
		return expiretestRequest(t, database, websiteID, kind, erasetestNow.AddDate(0, 0, -3), time.Time{})
	}

	// Something kept: partly, with the basis that says so.
	kept := request(a, KindErasure)
	erasetestOrder(t, database, a, "2026-0001", "ada@example.com")
	opt := erasetestOrders(s)
	opt.Sources, opt.RequestID = erasetestAll(), kept
	rep, err := s.Erase(in, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := st.Get(ctx, a, kept)
	if !rep.RequestAnswered || r.Outcome != OutcomePartly || r.Basis != erasePartlyBasis || r.HandledAt == nil {
		t.Errorf("with a reduced order: answered %v, %+v; want partly with the basis", rep.RequestAnswered, r)
	}

	// Nothing kept but the request itself: granted.
	granted := request(a, KindErasure)
	findExec(t, database, `INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada', 'ada@example.com', 'Frage', 'Text', '2026-02-01T09:00:00Z')`, a)
	opt.RequestID = granted
	rep, err = s.Erase(in, a, "CH", opt)
	if err != nil {
		t.Fatal(err)
	}
	r, _ = st.Get(ctx, a, granted)
	if !rep.RequestAnswered || r.Outcome != OutcomeGranted || r.Basis != eraseGrantedBasis {
		t.Errorf("with everything erased: answered %v, %+v; want granted", rep.RequestAnswered, r)
	}

	// Already answered, of another website, or not an erasure: untouched.
	other := request(b, KindErasure)
	for _, id := range []int64{granted, other, request(a, KindAccess)} {
		before, _ := st.Get(ctx, a, id)
		opt.RequestID = id
		rep, err = s.Erase(in, a, "CH", opt)
		if err != nil {
			t.Fatal(err)
		}
		if rep.RequestAnswered {
			t.Errorf("request %d was answered by the erasure", id)
		}
		if after, _ := st.Get(ctx, a, id); before != nil && (after.Outcome != before.Outcome || after.Basis != before.Basis) {
			t.Errorf("request %d changed: %+v to %+v", id, before, after)
		}
	}
	if r, _ := st.Get(ctx, b, other); r == nil || r.Outcome != "" {
		t.Errorf("website B's request was answered: %+v", r)
	}
}

func TestErasureLogsNoAddress(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	s := NewService(database)
	s.SetActivity(activity.NewStore(database))
	erasetestOrder(t, database, a, "2026-0001", "ada@example.com")
	findExec(t, database, `INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada', 'ada@example.com', 'Frage', 'Text', '2026-02-01T09:00:00Z')`, a)
	req := expiretestRequest(t, database, a, KindErasure, erasetestNow.AddDate(0, 0, -3), time.Time{})

	opt := erasetestOrders(s)
	opt.Sources, opt.RequestID = erasetestAll(), req
	opt.ActorUserID, opt.ActorEmail = 0, "chef@example.com"
	if _, err := s.Erase(context.Background(), a, "CH", opt); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Read.Query(`SELECT action, actor_email, entity_type, entity_id, COALESCE(website_id, 0), metadata
		FROM activity_log WHERE action = $1`, activity.ActionPrivacyErase)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
		var action, actor, entity, metadata string
		var entityID, websiteID int64
		if err := rows.Scan(&action, &actor, &entity, &entityID, &websiteID, &metadata); err != nil {
			t.Fatal(err)
		}
		all := strings.ToLower(strings.Join([]string{action, actor, entity, metadata}, " "))
		if strings.Contains(all, "ada@example.com") || strings.Contains(all, "ada lovelace") {
			t.Errorf("the activity entry carries the person: %s", all)
		}
		if websiteID != a {
			t.Errorf("website %d, want %d", websiteID, a)
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
			t.Fatal(err)
		}
		if meta["request"] != float64(req) || meta["reduced"] != float64(1) {
			t.Errorf("metadata %v, want the request id and one reduced order", meta)
		}
		if del, _ := meta["deleted"].(map[string]any); del[EraseFormMessages] != float64(1) {
			t.Errorf("metadata %v, want one form message deleted", meta)
		}
	}
	if n != 1 {
		t.Errorf("%d %s entries, want 1", n, activity.ActionPrivacyErase)
	}
}
