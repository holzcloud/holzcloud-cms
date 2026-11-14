package dsr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The kinds expire by their own rules (Phase 33.2, DSR-04, D-06). Every
// helper of this file starts with expiretest.

// expiretestCountry sets the country a website's order period follows.
func expiretestCountry(t *testing.T, database *db.DB, websiteID int64, country string) {
	t.Helper()
	if _, err := database.Write.Exec(`UPDATE websites SET country = $1 WHERE id = $2`, country, websiteID); err != nil {
		t.Fatal(err)
	}
}

// expiretestRequest stores a privacy request that arrived at created and
// was answered at handled, when handled is not zero.
func expiretestRequest(t *testing.T, database *db.DB, websiteID int64, kind string, created, handled time.Time) int64 {
	t.Helper()
	var h any
	outcome := ""
	if !handled.IsZero() {
		h = handled.UTC().Format(timeLayout)
		if kind != KindOptOut {
			outcome = OutcomeGranted
		}
	}
	res, err := database.Write.Exec(`INSERT INTO privacy_requests (website_id, kind, email, created_at, handled_at, outcome)
		VALUES ($1, $2, 'ada@example.com', $3, $4, $5)`, websiteID, kind, created.UTC().Format(timeLayout), h, outcome)
	if err != nil {
		t.Fatalf("insert request: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// expiretestOrder stores an order placed at created, with one item and one
// mail, and the date an erasure stored, when retainUntil is not empty.
func expiretestOrder(t *testing.T, database *db.DB, websiteID int64, number, created, retainUntil string) int64 {
	t.Helper()
	var keep any
	if retainUntil != "" {
		keep = retainUntil
	}
	res, err := database.Write.Exec(`INSERT INTO orders (website_id, number, token, audience, currency,
		email, name, street, postal_code, city, items_net, items_tax, items_gross, total_net, total_tax, total_gross,
		created_at, updated_at, retain_until)
		VALUES ($1, $2, $3, 'private', 'CHF', 'ada@example.com', 'Ada Lovelace', 'Weg 1', '3000', 'Bern',
		2500, 0, 2500, 2500, 0, 2500, $4, $4, $5)`,
		websiteID, number, "token-"+number, created, keep)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := database.Write.Exec(`INSERT INTO order_items (order_id, title, quantity, tax_bp,
		unit_gross, unit_net, line_net, line_tax, line_gross) VALUES ($1, 'Brett', 1, 0, 2500, 2500, 2500, 0, 2500)`, id); err != nil {
		t.Fatalf("insert order item: %v", err)
	}
	if _, err := database.Write.Exec(`INSERT INTO outbox (website_id, kind, order_id, recipient, subject, body,
		status, next_attempt_at, created_at) VALUES ($1, 'order_confirmation', $2, 'ada@example.com', 'Danke', 'Text',
		'sent', $3, $3)`, websiteID, id, created); err != nil {
		t.Fatalf("insert order mail: %v", err)
	}
	return id
}

// expiretestLeft counts the rows of the table that belong to the order.
func expiretestLeft(t *testing.T, database *db.DB, orderID int64) (orders, items, mails int) {
	t.Helper()
	for _, q := range []struct {
		query string
		into  *int
	}{
		{`SELECT COUNT(*) FROM orders WHERE id = $1`, &orders},
		{`SELECT COUNT(*) FROM order_items WHERE order_id = $1`, &items},
		{`SELECT COUNT(*) FROM outbox WHERE order_id = $1`, &mails},
	} {
		if err := database.Read.QueryRow(q.query, orderID).Scan(q.into); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func TestRequestsAreKeptAtLeast24Months(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	// The real clock: migration 00065's trigger reads it, and the job and the
	// trigger have to agree on the same day.
	now := time.Now().UTC()
	ws := website(t, database, "A")
	months := func(n int) time.Time { return now.AddDate(0, -n, 0) }
	kept := expiretestRequest(t, database, ws, KindOptOut, months(30), months(23))
	answeredLongAgo := expiretestRequest(t, database, ws, KindAccess, months(30), months(25))
	neverAnswered := expiretestRequest(t, database, ws, KindAccess, months(26), time.Time{})

	if err := s.SetPeriods(ctx, ws, "DE", map[string]int{RetentionRequest: 12}); !errors.Is(err, ErrBelowFloor) {
		t.Errorf("SetPeriods request 12 = %v, want ErrBelowFloor", err)
	}
	if _, err := s.RunRetention(ctx, now); err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if !retentionExists(t, database, "privacy_requests", kept) {
		t.Error("a request answered 23 months ago was deleted; the record is kept 24 months from the answer")
	}
	if retentionExists(t, database, "privacy_requests", answeredLongAgo) {
		t.Error("a request answered 25 months ago is still there")
	}
	if retentionExists(t, database, "privacy_requests", neverAnswered) {
		t.Error("a request of 26 months that was never answered is still there")
	}

	// The trigger of 00065 refuses the website for the kept row, and for
	// nothing else the job left behind.
	if _, err := database.Write.Exec(`DELETE FROM websites WHERE id = $1`, ws); err == nil {
		t.Fatal("the website was deleted while it keeps a request answered 23 months ago")
	}
	if _, err := database.Write.Exec(`DELETE FROM privacy_requests WHERE id = $1`, kept); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Write.Exec(`DELETE FROM websites WHERE id = $1`, ws); err != nil {
		t.Errorf("without the kept row the website is still refused: %v", err)
	}
}

func TestOrdersFollowTheSellersCountry(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	de := website(t, database, "DE")
	ch := website(t, database, "CH")
	at := website(t, database, "AT")
	expiretestCountry(t, database, de, "DE")
	expiretestCountry(t, database, ch, "CH")
	expiretestCountry(t, database, at, "AT")

	// DE keeps eight years from the end of the year: an order of 2016 may go
	// from 1 January 2025, one of 2018 from 1 January 2027.
	old := expiretestOrder(t, database, de, "2016-0001", "2016-06-01T10:00:00Z", "")
	young := expiretestOrder(t, database, de, "2018-0001", "2018-06-01T10:00:00Z", "")
	erased := expiretestOrder(t, database, de, "2016-0002", "2016-06-01T10:00:00Z", "2030-01-01T00:00:00Z")
	var swiss, austrian []int64
	for i, created := range []string{"2016-06-01T10:00:00Z", "2018-06-01T10:00:00Z"} {
		swiss = append(swiss, expiretestOrder(t, database, ch, "CH-"+strconv.Itoa(i), created, ""))
		austrian = append(austrian, expiretestOrder(t, database, at, "AT-"+strconv.Itoa(i), created, ""))
	}

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if o, i, m := expiretestLeft(t, database, old); o+i+m != 0 {
		t.Errorf("the DE order of 2016 left %d order, %d items, %d mails; want all gone", o, i, m)
	}
	if o, _, _ := expiretestLeft(t, database, young); o != 1 {
		t.Error("the DE order of 2018 was deleted before 1 January 2027")
	}
	if o, _, _ := expiretestLeft(t, database, erased); o != 1 {
		t.Error("an erased order was deleted before the date its person was told")
	}
	for _, id := range append(swiss, austrian...) {
		if o, _, _ := expiretestLeft(t, database, id); o != 1 {
			t.Errorf("order %d of a website with ten years was deleted", id)
		}
	}
	for _, k := range rep.Kinds {
		if k.WebsiteID == de && k.Kind == RetentionOrder && (k.Deleted != 1 || k.Err != "") {
			t.Errorf("DE's orders in the run = %+v, want 1 deleted and no error", k)
		}
	}

	if err := s.SetPeriods(ctx, de, "DE", map[string]int{RetentionOrder: 84}); !errors.Is(err, ErrBelowFloor) {
		t.Errorf("SetPeriods order 84 on DE = %v, want ErrBelowFloor", err)
	}
	if err := s.SetPeriods(ctx, de, "DE", map[string]int{RetentionOrder: 100}); !errors.Is(err, ErrBadPeriod) {
		t.Errorf("SetPeriods order 100 on DE = %v, want ErrBadPeriod (not whole years)", err)
	}
	if err := s.SetPeriods(ctx, de, "DE", map[string]int{RetentionOrder: 108}); err != nil {
		t.Fatalf("SetPeriods order 108 on DE: %v", err)
	}
	years, law, err := s.OrderYears(ctx, de, "DE")
	if err != nil || years != 9 || law.Years != 8 {
		t.Errorf("OrderYears = %d (law %d), %v; want 9, the law 8", years, law.Years, err)
	}
	// A stored period below a new country's law gives way to the law.
	years, _, err = s.OrderYears(ctx, de, "CH")
	if err != nil || years != 10 {
		t.Errorf("OrderYears with the country moved to CH = %d, %v; want 10", years, err)
	}
}

// expiretestMessage is a contact-form message in the plugin's own shape.
func expiretestMessage(at time.Time, suffix string, mediaID int64) (string, map[string]any) {
	stamp := at.UTC().Format(time.RFC3339)
	key := stamp + "-" + suffix
	msg := map[string]any{
		"kennung": key, "name": "Ada Lovelace", "email": "ada@example.com",
		"betreff": "Eier", "text": "Habt ihr am Samstag Eier?", "zeit": stamp,
	}
	if mediaID > 0 {
		msg["anhaenge"] = []map[string]any{{"medien_id": mediaID, "name": "brief.pdf", "dateiname": "brief.pdf"}}
	}
	return "nachricht:" + key, msg
}

// expiretestAttachment stores a media row and its file for the website.
func expiretestAttachment(t *testing.T, database *db.DB, dataDir string, websiteID int64, name string) int64 {
	t.Helper()
	m, err := media.NewStore(database).Create(context.Background(), websiteID, name, name, "application/pdf", 4, "hash-"+name)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestPluginFormSubmissionsExpireWithTheirFiles(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	m, store := plugintestManager(t, database)
	id := plugintestInstall(t, m, "kontaktformular", a)
	dir := t.TempDir()
	s.SetPlugins(m)
	s.SetMedia(media.NewStore(database), dir)

	file := expiretestAttachment(t, database, dir, a, "brief.pdf")
	oldKey, old := expiretestMessage(now.AddDate(-2, 0, 0), "aaaaaaaa", file)
	newKey, recent := expiretestMessage(now.AddDate(0, -1, 0), "bbbbbbbb", 0)
	plugintestSeed(t, store, id, a, oldKey, old)
	plugintestSeed(t, store, id, a, newKey, recent)

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	for _, k := range rep.Kinds {
		if k.WebsiteID == a && k.Kind == RetentionFormSubmission && (k.Deleted != 1 || k.Err != "") {
			t.Errorf("A's form submissions in the run = %+v, want 1 deleted and no error", k)
		}
	}
	res, err := s.Find(ctx, a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	keys := map[string]bool{}
	for _, r := range res.BySource()["plugin:"+id] {
		keys[r.Key] = true
	}
	if keys[oldKey] {
		t.Error("the message of two years is still found")
	}
	if !keys[newKey] {
		t.Errorf("the message of last month is gone: %v", keys)
	}
	if got, _ := media.NewStore(database).GetByID(ctx, file); got != nil {
		t.Error("the attachment's media row is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", strconv.FormatInt(a, 10), "brief.pdf")); !os.IsNotExist(err) {
		t.Errorf("the attachment's file is still on disk (%v)", err)
	}
}

func TestFarmShopOrdersExpireByTheOrderPeriod(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	expiretestCountry(t, database, a, "DE")
	m, store := plugintestManager(t, database)
	id := plugintestInstall(t, m, "bestellung", a)
	s.SetPlugins(m)
	s.SetMedia(media.NewStore(database), t.TempDir())

	order := func(stamp, received string) (string, map[string]any) {
		return "bestellung:" + stamp + "-aaaaaaaa", map[string]any{
			"id": stamp + "-aaaaaaaa", "eingegangen": received, "name": "Ada Lovelace", "email": "ada@example.com",
			"telefon": "+41 79 123 45 67", "adresse": "Bahnhofstrasse 1, 8001 Zürich", "seite": "bestellen",
			"posten": []map[string]any{{"slug": "seife", "titel": "Seife", "menge": 1, "preis": "8,50", "einheit": "Stück"}},
			"summe":  8.5, "summe_bekannt": true, "waehrung": "CHF",
		}
	}
	oldKey, old := order("20140315T100000", "2014-03-15T10:00:00Z")
	newKey, recent := order("20240315T100000", "2024-03-15T10:00:00Z")
	plugintestSeed(t, store, id, a, oldKey, old)
	plugintestSeed(t, store, id, a, newKey, recent)

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	for _, k := range rep.Kinds {
		if k.WebsiteID == a && k.Kind == RetentionOrder && (k.Deleted != 1 || k.Err != "") {
			t.Errorf("A's orders in the run = %+v, want the farm shop's order of 2014 and no error", k)
		}
	}
	if _, ok, err := store.StoreGet(ctx, id, a, oldKey); err != nil || ok {
		t.Errorf("the order of 2014 is still stored (%v)", err)
	}
	if _, ok, err := store.StoreGet(ctx, id, a, newKey); err != nil || !ok {
		t.Errorf("the order of 2024 is gone (%v)", err)
	}
}

func TestAPluginThatAlwaysHasMoreFailsTheKind(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	m, store := plugintestManager(t, database)
	id := plugintestInstall(t, m, "kontaktformular", a)
	s.SetPlugins(m)
	s.SetMedia(media.NewStore(database), t.TempDir())

	// One call of one record: three old messages are more than one run may
	// delete.
	calls, limit := expireCalls, expireLimit
	expireCalls, expireLimit = 1, 1
	t.Cleanup(func() { expireCalls, expireLimit = calls, limit })
	for i, suffix := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc"} {
		key, msg := expiretestMessage(now.AddDate(-2, 0, i), suffix, 0)
		plugintestSeed(t, store, id, a, key, msg)
	}
	legacy := retentionFormMessage(t, database, a, now.AddDate(-2, 0, 0))
	request := expiretestRequest(t, database, a, KindAccess, now.AddDate(-3, 0, 0), time.Time{})

	rep, err := s.RunRetention(ctx, now)
	if err == nil {
		t.Fatal("RunRetention reported success while a plugin had more to expire")
	}
	if rep.Status != RunFailed {
		t.Errorf("run status %q, want failed", rep.Status)
	}
	var status string
	if err := database.Read.QueryRow(`SELECT status FROM retention_runs WHERE id = $1`, rep.ID).Scan(&status); err != nil || status != RunFailed {
		t.Errorf("stored status %q (%v), want failed", status, err)
	}
	d, ok := retentionDeletionOf(t, database, rep.ID, a, RetentionFormSubmission)
	if !ok || !strings.Contains(d.err, "more to expire than one run may delete") {
		t.Errorf("the form submissions row = %+v (%v), want the bound named", d, ok)
	}
	if d.deleted != 2 {
		t.Errorf("the form submissions row counts %d, want the legacy message and the one plugin record (2)", d.deleted)
	}
	if retentionExists(t, database, "form_messages", legacy) {
		t.Error("the legacy message of the same kind was not deleted")
	}
	if retentionExists(t, database, "privacy_requests", request) {
		t.Error("the requests of the same website were not deleted")
	}
}
