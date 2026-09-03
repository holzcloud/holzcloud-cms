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
		t.Errorf("der Text kam nicht durch: %q", due[0].Body)
	}
}

// Eine Nachricht ohne Empfänger wird gar nicht erst abgelegt. Sonst sammelt ein
// Betrieb, der nie eine Meldeadresse eingetragen hat, pro Bestellung eine
// unzustellbare Zeile an — für immer.
func TestQueueWithoutRecipientIsDropped(t *testing.T) {
	s := store(t)
	id, err := s.Queue(context.Background(), Mail{WebsiteID: 1, Kind: KindOrderOperator})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if id != 0 {
		t.Errorf("es wurde eine Zeile angelegt (id %d)", id)
	}
	due, _ := s.Due(context.Background(), 10)
	if len(due) != 0 {
		t.Errorf("Due lieferte %d Nachrichten ohne Empfänger", len(due))
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
		t.Error("eine verschickte Nachricht steht weiter zum Versand an")
	}
	m, err := s.byID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != StatusSent || m.SentAt == nil {
		t.Errorf("Status %q, Zeitpunkt %v", m.Status, m.SentAt)
	}
}

// Ein Fehlschlag wartet, und die Wartezeit wächst. Sonst klopft der Server im
// Sekundentakt an einen Mailserver, der ohnehin gerade nicht kann.
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
			t.Errorf("nach %d Fehlschlägen: Attempts = %d", i+1, m.Attempts)
		}
		if got := m.NextAttemptAt.Sub(start); got != want {
			t.Errorf("Versuch %d wartet %v, erwartet %v", i+1, got, want)
		}
		if m.Status != StatusPending {
			t.Errorf("Versuch %d: Status %q, erwartet noch offen", i+1, m.Status)
		}
		if !strings.Contains(m.LastError, "antwortet nicht") {
			t.Errorf("der Grund wurde nicht vermerkt: %q", m.LastError)
		}
	}

	// Der fünfte Fehlschlag gibt auf.
	if err := s.MarkFailed(ctx, id, errors.New("endgültig")); err != nil {
		t.Fatal(err)
	}
	m, _ := s.byID(ctx, id)
	if m.Status != StatusFailed {
		t.Errorf("nach %d Versuchen: Status %q, erwartet %q", MaxAttempts, m.Status, StatusFailed)
	}

	// Und wird nicht mehr angefasst, auch nicht viel später.
	clockAt(s, start.Add(30*24*time.Hour))
	if due, _ := s.Due(ctx, 10); len(due) != 0 {
		t.Error("eine aufgegebene Nachricht steht wieder zum Versand an")
	}
}

// Eine wartende Nachricht wird vor ihrer Zeit nicht angefasst.
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
		t.Error("die Nachricht wurde vor Ablauf der Wartezeit wieder ausgeliefert")
	}

	clockAt(s, start.Add(2*time.Minute))
	if due, _ := s.Due(ctx, 10); len(due) != 1 {
		t.Error("nach der Wartezeit wurde die Nachricht nicht wieder angeboten")
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

	if err := s.Retry(ctx, id); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	m, _ := s.byID(ctx, id)
	if m.Status != StatusPending {
		t.Errorf("Status nach Retry: %q", m.Status)
	}
	// Der Zähler muss zurück, sonst gibt der nächste Fehlschlag sofort wieder auf.
	if m.Attempts != 0 {
		t.Errorf("Attempts nach Retry: %d", m.Attempts)
	}
	if due, _ := s.Due(ctx, 10); len(due) != 1 {
		t.Error("nach Retry steht die Nachricht nicht zum Versand an")
	}
}

// Was raus ist, ist raus. Ein zweites Mal verschicken wäre für die Kundin eine
// zweite Bestellbestätigung zu einer Bestellung, die es nur einmal gibt.
func TestRetryRefusesASentMail(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	id := queue(t, s, "kundin@example.ch")
	if err := s.MarkSent(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := s.Retry(ctx, id); err == nil {
		t.Error("eine verschickte Nachricht liess sich erneut einstellen")
	}
}

// Aufgeräumt wird nur, was zugestellt wurde. Ein Fehlschlag bleibt stehen, bis
// jemand hingesehen hat — das ist der ganze Grund, ihn aufzuschreiben.
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
		t.Errorf("die gescheiterte Nachricht wurde mit aufgeräumt: %v", err)
	}
}

func TestForOrder(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	var orderID int64 = 7
	// Die Bestellung selbst gibt es in diesem Test nicht; das Feld darf leer
	// bleiben, deshalb wird hier ohne Fremdschlüsselziel eingestellt.
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

// fakeSender zählt mit und kann auf Wunsch scheitern.
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
		t.Fatalf("es wurden %d Nachrichten übergeben", len(f.sent))
	}
	if f.sent[0].To != "kundin@example.ch" || f.sent[0].Subject != "Ihre Bestellung" {
		t.Errorf("falsch übergeben: %+v", f.sent[0])
	}

	// Und beim zweiten Lauf nicht noch einmal.
	if err := d.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 1 {
		t.Errorf("dieselbe Nachricht wurde %d× verschickt", len(f.sent))
	}
}

// Ohne eingerichteten Versand wird nichts angefasst — und vor allem nichts als
// gescheitert vermerkt. Ein Shop ohne Mailkonto ist ein funktionierender Shop.
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

// Eine Nachricht, die nicht rausgeht, hält die dahinter nicht auf.
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
		t.Errorf("nach einem Fehlschlag: %q", m.Status)
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
	// Antwortet der Betrieb auf die Meldung, muss die Antwort bei der Kundin
	// ankommen und nicht bei ihm selbst.
	if betrieb.ReplyTo != "anna@example.ch" {
		t.Errorf("Antwortadresse der Meldung: %q", betrieb.ReplyTo)
	}
	if kundin.ReplyTo != "bestellungen@example.ch" {
		t.Errorf("Antwortadresse der Bestätigung: %q", kundin.ReplyTo)
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
		"Bitte vormittags",   // ihre Bemerkung
		"Rechnung liegt der", // was von ihr erwartet wird
		"https://example.ch/bestellung/2026-0007", // wo sie nachsehen kann
		"CHE-123.456.789 MWST",                    // die UID im Fuss
	} {
		if !strings.Contains(body, want) {
			t.Errorf("in der Bestätigung fehlt %q:\n%s", want, body)
		}
	}
	if strings.Contains(mails[0].Subject, "\n") {
		t.Error("der Betreff enthält einen Zeilenumbruch")
	}
}

// Bei Vorauskasse ist die Nachricht der einzige Ort, an dem die Kundin je
// erfährt, wohin sie überweisen soll.
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

// Und wenn der Betrieb sie nicht hinterlegt hat, darf die Nachricht nicht so
// tun, als stünde alles drin.
func TestPrepaymentWithoutBankDetailsSaysSo(t *testing.T) {
	o := testOrder()
	o.PaymentMethod = shop.PayPrepay

	body := ForOrder(testShop(), o)[0].Body
	if !strings.Contains(body, "melden uns mit den Zahlungsangaben") {
		t.Errorf("ohne Kontoangaben fehlt der Hinweis:\n%s", body)
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

// Ohne Meldeadresse geht nur die Bestätigung raus — und keine unzustellbare
// zweite Nachricht.
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

// Eine Bestellung ohne E-Mail-Adresse kann es über die Kasse nicht geben, über
// die Datenbank schon. Dann darf nichts an die leere Adresse gehen.
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

// Ein Shop ohne MWST-Pflicht muss das sagen, nicht bloss die Steuerzeile
// weglassen: die Rechnung braucht den Grund.
func TestExemptShopSaysWhyThereIsNoTax(t *testing.T) {
	o := testOrder()
	o.VATExempt = true
	o.Totals.TotalTax = 0

	body := ForOrder(testShop(), o)[0].Body
	if !strings.Contains(body, "Kleinunternehmen") {
		t.Errorf("der Grund für die fehlende MWST fehlt:\n%s", body)
	}
}

// Eine Bestellung trägt ihre eigene Währung. Wechselt der Shop später, darf
// eine alte Bestellung nicht in der neuen nachgedruckt werden.
func TestOrderKeepsItsOwnCurrency(t *testing.T) {
	s := testShop()
	s.Currency = money.CurrencyFor("EUR")
	o := testOrder() // CHF

	body := ForOrder(s, o)[0].Body
	if !strings.Contains(body, "CHF") {
		t.Errorf("die Bestellung wurde in der neuen Währung gedruckt:\n%s", body)
	}
}
