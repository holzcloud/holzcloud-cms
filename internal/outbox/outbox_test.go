package outbox

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

func store(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if _, err := database.Write.Exec(
		`INSERT INTO websites (id, name, description) VALUES (1, 'Laden', '')`); err != nil {
		t.Fatalf("Website anlegen: %v", err)
	}
	return NewStore(database)
}

// clockAt pins the store's clock so backoff can be asserted exactly.
func clockAt(s *Store, t time.Time) { s.now = func() time.Time { return t } }

func queue(t *testing.T, s *Store, recipient string) int64 {
	t.Helper()
	id, err := s.Queue(context.Background(), Mail{
		WebsiteID: 1, Kind: KindOrderCustomer, Recipient: recipient,
		Subject: "Ihre Bestellung", Body: "Vielen Dank.",
	})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	return id
}

func TestQueueAndDue(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")

	due, err := s.Due(ctx, 10)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 1 || due[0].ID != id {
		t.Fatalf("Due lieferte %d Nachrichten", len(due))
	}
	if due[0].Status != StatusPending || due[0].Attempts != 0 {
		t.Errorf("frisch eingestellt: %q / %d Versuche", due[0].Status, due[0].Attempts)
	}
	if due[0].Body != "Vielen Dank." {
		t.Errorf("the text did not come through: %q", due[0].Body)
	}
}

// A message without a recipient is not filed at all. Otherwise a business that
// never entered a notification address collects one undeliverable row per order
// — for ever.
func TestQueueWithoutRecipientIsDropped(t *testing.T) {
	s := store(t)
	id, err := s.Queue(context.Background(), Mail{WebsiteID: 1, Kind: KindOrderOperator})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if id != 0 {
		t.Errorf("a row was created (id %d)", id)
	}
	due, _ := s.Due(context.Background(), 10)
	if len(due) != 0 {
		t.Errorf("Due returned %d messages with no recipient", len(due))
	}
}

func TestMarkSent(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")

	if err := s.MarkSent(ctx, id); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	due, _ := s.Due(ctx, 10)
	if len(due) != 0 {
		t.Error("a sent message is still queued for sending")
	}
	m, err := s.byID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != StatusSent || m.SentAt == nil {
		t.Errorf("Status %q, Zeitpunkt %v", m.Status, m.SentAt)
	}
}

// A failure waits, and the wait grows. Otherwise the server knocks once a
// second at a mail server that cannot manage just now anyway.
func TestFailedMailWaitsLongerEachTime(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	start := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	clockAt(s, start)
	id := queue(t, s, "kundin@example.ch")

	waits := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}
	for i, want := range waits {
		if err := s.MarkFailed(ctx, id, errors.New("mailserver antwortet nicht")); err != nil {
			t.Fatalf("MarkFailed: %v", err)
		}
		m, err := s.byID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if m.Attempts != i+1 {
			t.Errorf("after %d failures: Attempts = %d", i+1, m.Attempts)
		}
		if got := m.NextAttemptAt.Sub(start); got != want {
			t.Errorf("Versuch %d wartet %v, erwartet %v", i+1, got, want)
		}
		if m.Status != StatusPending {
			t.Errorf("Versuch %d: Status %q, erwartet noch offen", i+1, m.Status)
		}
		if !strings.Contains(m.LastError, "antwortet nicht") {
			t.Errorf("the reason was not recorded: %q", m.LastError)
		}
	}

	// The fifth failure gives up.
	if err := s.MarkFailed(ctx, id, errors.New("endgültig")); err != nil {
		t.Fatal(err)
	}
	m, _ := s.byID(ctx, id)
	if m.Status != StatusFailed {
		t.Errorf("nach %d Versuchen: Status %q, erwartet %q", MaxAttempts, m.Status, StatusFailed)
	}

	// And is not touched again, not even much later.
	clockAt(s, start.Add(30*24*time.Hour))
	if due, _ := s.Due(ctx, 10); len(due) != 0 {
		t.Error("an abandoned message is queued for sending again")
	}
}

// A waiting message is not touched before its time.
func TestDueRespectsTheWait(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	start := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	clockAt(s, start)
	id := queue(t, s, "kundin@example.ch")

	if err := s.MarkFailed(ctx, id, errors.New("kurz weg")); err != nil {
		t.Fatal(err)
	}

	clockAt(s, start.Add(30*time.Second))
	if due, _ := s.Due(ctx, 10); len(due) != 0 {
		t.Error("the message was delivered again before the waiting time was up")
	}

	clockAt(s, start.Add(2*time.Minute))
	if due, _ := s.Due(ctx, 10); len(due) != 1 {
		t.Error("after the waiting time the message was not offered again")
	}
}

func TestRetryPutsAGivenUpMailBack(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "falsch@example.ch")
	for i := 0; i < MaxAttempts; i++ {
		if err := s.MarkFailed(ctx, id, errors.New("kein solcher Empfänger")); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Retry(ctx, 1, id); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	m, _ := s.byID(ctx, id)
	if m.Status != StatusPending {
		t.Errorf("Status nach Retry: %q", m.Status)
	}
	// The counter has to go back, or the next failure gives up at once again.
	if m.Attempts != 0 {
		t.Errorf("Attempts nach Retry: %d", m.Attempts)
	}
	if due, _ := s.Due(ctx, 10); len(due) != 1 {
		t.Error("after Retry the message is not queued for sending")
	}
}

// What is out is out. Sending a second time would be, for the customer, a
// second order confirmation for an order that exists only once.
func TestRetryRefusesASentMail(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")
	if err := s.MarkSent(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := s.Retry(ctx, 1, id); err == nil {
		t.Error("a sent message could be queued again")
	}
}

// Only what was delivered is cleared away. A failure stays standing until
// somebody has looked at it — that is the whole reason for writing it down.
func TestPruneKeepsFailures(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clockAt(s, start)

	verschickt := queue(t, s, "eine@example.ch")
	gescheitert := queue(t, s, "andere@example.ch")
	if err := s.MarkSent(ctx, verschickt); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxAttempts; i++ {
		if err := s.MarkFailed(ctx, gescheitert, errors.New("weg")); err != nil {
			t.Fatal(err)
		}
	}

	clockAt(s, start.Add(90*24*time.Hour))
	n, err := s.Prune(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("Prune entfernte %d Zeilen, erwartet 1", n)
	}
	if _, err := s.byID(ctx, gescheitert); err != nil {
		t.Errorf("the failed message was swept away with the rest: %v", err)
	}
}

func TestForOrder(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	var orderID int64 = 7
	// The order itself does not exist in this test; the field may stay empty,
	// which is why it is set here without a foreign key target.
	if _, err := s.DB.Write.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{KindOrderCustomer, KindOrderOperator} {
		if _, err := s.Queue(ctx, Mail{WebsiteID: 1, Kind: kind, OrderID: &orderID,
			Recipient: "a@b.ch", Subject: "x", Body: "y"}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.ForOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("ForOrder: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ForOrder lieferte %d Nachrichten", len(got))
	}
	if got[0].Kind != KindOrderCustomer || got[1].Kind != KindOrderOperator {
		t.Errorf("die Reihenfolge stimmt nicht: %q, %q", got[0].Kind, got[1].Kind)
	}
}

// fakeSender counts along and can fail on request.
type fakeSender struct {
	mu         sync.Mutex
	sent       []mail.Message
	fail       error
	configured bool
}

func (f *fakeSender) Configured() bool { return f.configured }

func (f *fakeSender) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.sent = append(f.sent, m)
	return nil
}

func TestDispatcherSends(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	queue(t, s, "kundin@example.ch")

	f := &fakeSender{configured: true}
	d := &Dispatcher{Store: s, Sender: f}
	if err := d.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(f.sent) != 1 {
		t.Fatalf("%d messages were handed over", len(f.sent))
	}
	if f.sent[0].To != "kundin@example.ch" || f.sent[0].Subject != "Ihre Bestellung" {
		t.Errorf("handed over wrongly: %+v", f.sent[0])
	}

	// And not once more on the second run.
	if err := d.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 1 {
		t.Errorf("dieselbe Nachricht wurde %d× verschickt", len(f.sent))
	}
}

// Without sending set up nothing is touched — and above all nothing is noted as
// failed. A shop without a mail account is a working shop.
func TestDispatcherWithoutMailAccountLeavesEverythingAlone(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")

	d := &Dispatcher{Store: s, Sender: &fakeSender{configured: false}}
	if err := d.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	m, _ := s.byID(ctx, id)
	if m.Status != StatusPending || m.Attempts != 0 {
		t.Errorf("die Nachricht wurde angefasst: %q, %d Versuche", m.Status, m.Attempts)
	}
}

// A message that does not go out does not hold up the ones behind it.
func TestDispatcherKeepsGoingAfterAFailure(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")

	f := &fakeSender{configured: true, fail: errors.New("mailserver antwortet nicht")}
	d := &Dispatcher{Store: s, Sender: f}
	if err := d.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	m, _ := s.byID(ctx, id)
	if m.Attempts != 1 {
		t.Errorf("Attempts = %d, erwartet 1", m.Attempts)
	}
	if !strings.Contains(m.LastError, "antwortet nicht") {
		t.Errorf("der Grund fehlt: %q", m.LastError)
	}
	if m.Status != StatusPending {
		t.Errorf("after one failure: %q", m.Status)
	}
}

func TestDispatcherWithoutStore(t *testing.T) {
	d := &Dispatcher{}
	if err := d.Run(context.Background()); err != nil {
		t.Errorf("ohne Postausgang: %v", err)
	}
}

// --- Der Inhalt der Nachrichten -------------------------------------------

func testOrder() *shop.Order {
	return &shop.Order{
		ID: 7, WebsiteID: 1, Number: "2026-0007", Currency: "CHF",
		Audience: shop.Private,
		Customer: shop.Customer{
			Email: "anna@example.ch", Name: "Anna Meier",
			Street: "Seestrasse 4", PostalCode: "8002", City: "Zürich", Country: "CH",
			Note: "Bitte vormittags liefern.",
		},
		Totals: shop.Totals{
			ItemsGross: 9800, ShippingGross: 1200, TotalGross: 11000,
			TotalNet: 10176, TotalTax: 824,
		},
		Status: shop.OrderNew, PaymentMethod: shop.PayInvoice, PaymentStatus: shop.PaymentOpen,
		Items: []shop.OrderItem{{
			Title: "Hocker Brunni", Subtitle: "Esche geölt", Quantity: 2,
			UnitGross: 4900, LineGross: 9800,
		}},
		CreatedAt: time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC),
	}
}

func testShop() Shop {
	return Shop{
		Name: "Holzbau Schmidt", URL: "https://example.ch",
		OrderEmail: "bestellungen@example.ch",
		VATNumber:  "CHE-123.456.789 MWST",
		Currency:   money.CurrencyFor("CHF"),
	}
}

func TestForOrderProducesBothMessages(t *testing.T) {
	mails := ForOrder(testShop(), testOrder())
	if len(mails) != 2 {
		t.Fatalf("es entstanden %d Nachrichten", len(mails))
	}

	kundin, betrieb := mails[0], mails[1]
	if kundin.Recipient != "anna@example.ch" {
		t.Errorf("die Kundin bekommt %q", kundin.Recipient)
	}
	if betrieb.Recipient != "bestellungen@example.ch" {
		t.Errorf("der Betrieb bekommt %q", betrieb.Recipient)
	}
	// If the business replies to the notification, the reply has to reach the
	// customer and not the business itself.
	if betrieb.ReplyTo != "anna@example.ch" {
		t.Errorf("Antwortadresse der Meldung: %q", betrieb.ReplyTo)
	}
	if kundin.ReplyTo != "bestellungen@example.ch" {
		t.Errorf("reply address of the confirmation: %q", kundin.ReplyTo)
	}
	if kundin.OrderID == nil || *kundin.OrderID != 7 {
		t.Error("die Nachricht ist keiner Bestellung zugeordnet")
	}
}

func TestCustomerMailSaysWhatMatters(t *testing.T) {
	mails := ForOrder(testShop(), testOrder())
	body := mails[0].Body

	for _, want := range []string{
		"Anna Meier",         // Anrede
		"2026-0007",          // Bestellnummer
		"Hocker Brunni",      // die Ware
		"2 × ",               // die Menge
		"CHF\u00a0110.00",    // die Summe
		"Seestrasse 4",       // die Lieferadresse
		"Bitte vormittags",   // her remark
		"Rechnung liegt der", // what is expected of her
		"https://example.ch/bestellung/2026-0007", // where she can look it up
		"CHE-123.456.789 MWST",                    // the VAT number in the footer
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the confirmation is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(mails[0].Subject, "\n") {
		t.Error("the subject contains a line break")
	}
}

// With payment in advance the message is the only place the customer ever
// learns where they are supposed to transfer the money.
func TestPrepaymentCarriesTheBankDetails(t *testing.T) {
	s := testShop()
	s.PaymentDetails = "Holzbau Schmidt AG\nCH93 0076 2011 6238 5295 7"
	o := testOrder()
	o.PaymentMethod = shop.PayPrepay

	body := ForOrder(s, o)[0].Body
	if !strings.Contains(body, "CH93 0076 2011 6238 5295 7") {
		t.Errorf("die Kontoangaben fehlen:\n%s", body)
	}
	if !strings.Contains(body, "2026-0007") {
		t.Errorf("der Zahlungszweck fehlt:\n%s", body)
	}
}

// And when the business has not stored it, the message must not pretend that
// everything is in there.
func TestPrepaymentWithoutBankDetailsSaysSo(t *testing.T) {
	o := testOrder()
	o.PaymentMethod = shop.PayPrepay

	body := ForOrder(testShop(), o)[0].Body
	if !strings.Contains(body, "melden uns mit den Zahlungsangaben") {
		t.Errorf("without the account details the hint is missing:\n%s", body)
	}
}

func TestOperatorMailCarriesTheContactDetails(t *testing.T) {
	o := testOrder()
	o.Customer.Phone = "044 123 45 67"
	o.Customer.Company = "Meier AG"
	o.Customer.VATNumber = "CHE-987.654.321 MWST"

	body := ForOrder(testShop(), o)[1].Body
	for _, want := range []string{"Meier AG", "044 123 45 67", "anna@example.ch",
		"CHE-987.654.321", "Rechnung", "offen"} {
		if !strings.Contains(body, want) {
			t.Errorf("in der Meldung fehlt %q:\n%s", want, body)
		}
	}
}

// Without a notification address only the confirmation goes out — and no
// undeliverable second message.
func TestWithoutAnOrderAddressOnlyTheCustomerHears(t *testing.T) {
	s := testShop()
	s.OrderEmail = ""

	mails := ForOrder(s, testOrder())
	if len(mails) != 1 || mails[0].Kind != KindOrderCustomer {
		t.Fatalf("es entstanden %d Nachrichten: %+v", len(mails), mails)
	}
	if mails[0].ReplyTo != "" {
		t.Errorf("Antwortadresse ohne Meldeadresse: %q", mails[0].ReplyTo)
	}
}

// An order without an e-mail address cannot come about through the checkout,
// but it can through the database. Then nothing may go to the empty address.
func TestWithoutACustomerAddressOnlyTheOperatorHears(t *testing.T) {
	o := testOrder()
	o.Customer.Email = ""

	mails := ForOrder(testShop(), o)
	if len(mails) != 1 || mails[0].Kind != KindOrderOperator {
		t.Fatalf("es entstanden %d Nachrichten: %+v", len(mails), mails)
	}
}

func TestShipmentMail(t *testing.T) {
	mails := ForShipment(testShop(), testOrder())
	if len(mails) != 1 {
		t.Fatalf("es entstanden %d Nachrichten", len(mails))
	}
	if !strings.Contains(mails[0].Subject, "unterwegs") {
		t.Errorf("Betreff: %q", mails[0].Subject)
	}
	for _, want := range []string{"Anna Meier", "2026-0007", "Hocker Brunni", "Seestrasse 4"} {
		if !strings.Contains(mails[0].Body, want) {
			t.Errorf("in der Versandmeldung fehlt %q:\n%s", want, mails[0].Body)
		}
	}
}

// A shop not liable for VAT has to say so, not merely leave the tax line out:
// the invoice needs the reason.
func TestExemptShopSaysWhyThereIsNoTax(t *testing.T) {
	o := testOrder()
	o.VATExempt = true
	o.Totals.TotalTax = 0

	body := ForOrder(testShop(), o)[0].Body
	if !strings.Contains(body, "Kleinunternehmen") {
		t.Errorf("the reason for the missing VAT is missing:\n%s", body)
	}
}

// An order carries its own currency. If the shop changes later, an old order
// must not be reprinted in the new one.
func TestOrderKeepsItsOwnCurrency(t *testing.T) {
	s := testShop()
	s.Currency = money.CurrencyFor("EUR")
	o := testOrder() // CHF

	body := ForOrder(s, o)[0].Body
	if !strings.Contains(body, "CHF") {
		t.Errorf("the order was printed in the new currency:\n%s", body)
	}
}

// Retry is reached from the order screen with a message id taken straight from
// a form, so the store is the only thing standing between a mistyped or guessed
// number and somebody else's customer receiving a mail again.
func TestRetryRefusesAMessageOfAnotherWebsite(t *testing.T) {
	s := store(t)
	ctx := context.Background()

	if _, err := s.DB.Write.Exec(
		`INSERT INTO websites (id, name, description) VALUES (2, 'Zweiter Laden', '')`); err != nil {
		t.Fatalf("insert second website: %v", err)
	}
	foreign, err := s.Queue(ctx, Mail{
		WebsiteID: 2, Kind: KindOrderCustomer, Recipient: "kundin-b@example.ch",
		Subject: "Ihre Bestellung", Body: "Vielen Dank.",
	})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	for i := 0; i < MaxAttempts; i++ {
		if err := s.MarkFailed(ctx, foreign, errors.New("kein solcher Empfänger")); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Retry(ctx, 1, foreign); err == nil {
		t.Error("Retry across websites was accepted")
	}

	m, err := s.byID(ctx, foreign)
	if err != nil {
		t.Fatalf("byID: %v", err)
	}
	if m.Status != StatusFailed {
		t.Errorf("the foreign message was re-queued: status %q", m.Status)
	}
	if m.Attempts != MaxAttempts {
		t.Errorf("the foreign attempt counter was reset to %d", m.Attempts)
	}
	if m.LastError == "" {
		t.Error("the foreign error text was cleared")
	}
}
