package public

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/payrexx"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// fakePayrexx answers Gateway reads with whatever a test set up.
//
// Only GET is answered: the tests here are about what the shop believes after
// asking the provider, and CreateGateway is covered in the payrexx package
// against a stand-in that checks the signature.
type fakePayrexx struct {
	status   string
	amount   int64
	currency string
	ref      string
	calls    int
	fail     bool
}

func (f *fakePayrexx) client(t *testing.T) *payrexx.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls++
		if f.fail {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "kaputt")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"success","data":[{"id":4711,"status":"`+f.status+
			`","referenceId":"`+f.ref+`","amount":`+strconv.FormatInt(f.amount, 10)+
			`,"currency":"`+f.currency+`"}]}`)
	}))
	t.Cleanup(srv.Close)
	return &payrexx.Client{Instance: "example", Secret: "geheim", BaseURL: srv.URL}
}

// shopSite turns a seeded website into one that sells, and wires the stores.
func shopSite(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) {
	t.Helper()
	if _, err := database.Write.ExecContext(context.Background(),
		`UPDATE websites SET shop_base = 'shop', currency = 'CHF',
		 shipping_gross = 0, shipping_tax_bp = 810, price_display = 'private'
		 WHERE id = ?`, ws.ID); err != nil {
		t.Fatalf("shop einschalten: %v", err)
	}
	ws.ShopBase = "shop"
	ws.Currency = "CHF"
	ws.PriceDisplay = "private"
	ws.ShippingTaxBP = 810

	products := shop.NewStore(database)
	carts := shop.NewCartStore(products)
	h.SetProductStore(products)
	h.SetCartStore(carts)
	h.SetOrderStore(shop.NewOrderStore(carts))
}

// placeTestOrder writes one order paid by card, in the state it is in the
// moment the customer is sent off to the payment page.
func placeTestOrder(t *testing.T, h *Handler, database *db.DB, ws *domain.Website) *shop.Order {
	t.Helper()
	ctx := context.Background()

	products := shop.NewStore(database)
	id, err := products.Create(ctx, &shop.Product{
		WebsiteID: ws.ID, Slug: "tisch", Title: "Tisch",
		PriceGross: 4900, TaxRate: money.RateStandard, Status: "published",
	})
	if err != nil {
		t.Fatalf("Produkt anlegen: %v", err)
	}

	carts := shop.NewCartStore(products)
	cart, _, err := carts.Ensure(ctx, ws.ID, "")
	if err != nil {
		t.Fatalf("Warenkorb: %v", err)
	}
	if err := carts.Add(ctx, cart.ID, id, 1); err != nil {
		t.Fatalf("in den Warenkorb: %v", err)
	}
	full, err := carts.Get(ctx, ws.ID, cart.Token)
	if err != nil {
		t.Fatalf("Warenkorb lesen: %v", err)
	}

	orders := shop.NewOrderStore(carts)
	order, err := orders.Place(ctx, ws.ID, shopSettings(ws), shop.Private, full, shop.Customer{
		Email: "kundin@example.ch", Name: "Anna Meier",
		Street: "Seestrasse 4", PostalCode: "8002", City: "Zürich", Country: "CH",
	}, shop.PayPayrexx)
	if err != nil {
		t.Fatalf("Bestellung: %v", err)
	}
	// What startPayment writes before the redirect.
	if err := orders.SetPayment(ctx, order.ID, shop.PaymentOpen, "4711"); err != nil {
		t.Fatalf("Referenz merken: %v", err)
	}
	order.PaymentReference = "4711"
	return order
}

func reload(t *testing.T, h *Handler, ws *domain.Website, number string) *shop.Order {
	t.Helper()
	o, err := h.orders.ByNumber(context.Background(), ws.ID, number)
	if err != nil || o == nil {
		t.Fatalf("Bestellung %s nachlesen: %v", number, err)
	}
	return o
}

func TestPaymentReturnSettlesAConfirmedPayment(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "confirmed", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	rec := requestWithPath(t, h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Status = %d, erwartet 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != thanksPath+"/"+order.Number {
		t.Errorf("Weiterleitung = %q", got)
	}

	got := reload(t, h, ws, order.Number)
	if got.PaymentStatus != shop.PaymentPaid {
		t.Errorf("Zahlungsstatus = %q, erwartet %q", got.PaymentStatus, shop.PaymentPaid)
	}
	if got.Status != shop.OrderPaid {
		t.Errorf("Bestellstatus = %q, erwartet %q", got.Status, shop.OrderPaid)
	}
}

// The test this whole file exists for. A gateway confirmed over one franc must
// never settle an order of forty-nine — that is the first thing anyone who can
// open gateways on their own Payrexx account would try.
func TestPaymentForTheWrongAmountIsRefused(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "confirmed", amount: 100, currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number}); err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}

	got := reload(t, h, ws, order.Number)
	if got.PaymentStatus == shop.PaymentPaid {
		t.Fatal("eine Zahlung über 1.00 hat eine Bestellung über 49.00 als bezahlt markiert")
	}
	if got.Status == shop.OrderPaid {
		t.Error("die Bestellung wurde trotzdem auf bezahlt gesetzt")
	}
}

func TestPaymentInTheWrongCurrencyIsRefused(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	// The same number of minor units, in a currency worth more.
	fake := &fakePayrexx{status: "confirmed", amount: int64(order.Totals.TotalGross),
		currency: "EUR", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number}); err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}

	if got := reload(t, h, ws, order.Number); got.PaymentStatus == shop.PaymentPaid {
		t.Error("eine Zahlung in einer anderen Währung wurde angenommen")
	}
}

func TestFailedPaymentIsRecordedAndTheOrderStays(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "cancelled", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number}); err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}

	got := reload(t, h, ws, order.Number)
	if got.PaymentStatus != shop.PaymentFailed {
		t.Errorf("Zahlungsstatus = %q, erwartet %q", got.PaymentStatus, shop.PaymentFailed)
	}
	// A customer who cancelled may still want the goods on account. Deleting
	// their order is the one thing that makes that impossible.
	if got.Status != shop.OrderNew {
		t.Errorf("Bestellstatus = %q; eine gescheiterte Zahlung darf die Bestellung nicht entfernen", got.Status)
	}
}

// Held money is not arrived money.
func TestAuthorizedIsNotPaid(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "authorized", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number}); err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}

	got := reload(t, h, ws, order.Number)
	if got.PaymentStatus != shop.PaymentOpen {
		t.Errorf("Zahlungsstatus = %q; reserviertes Geld ist kein eingegangenes Geld", got.PaymentStatus)
	}
}

// A settled order must not be asked about again, and a refund recorded by hand
// must not be overwritten by an old "confirmed".
func TestSettledPaymentIsNotAskedAboutAgain(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	if err := h.orders.SetPayment(context.Background(), order.ID, shop.PaymentRefunded, "4711"); err != nil {
		t.Fatalf("SetPayment: %v", err)
	}

	fake := &fakePayrexx{status: "confirmed", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number}); err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}

	if fake.calls != 0 {
		t.Errorf("der Anbieter wurde %d× gefragt, obwohl die Zahlung abgeschlossen ist", fake.calls)
	}
	if got := reload(t, h, ws, order.Number); got.PaymentStatus != shop.PaymentRefunded {
		t.Errorf("Zahlungsstatus = %q; eine Rückerstattung wurde überschrieben", got.PaymentStatus)
	}
}

// A provider that cannot be reached must not turn the customer's return into an
// error page. Their order exists and the confirmation has to show it.
func TestUnreachableProviderStillShowsTheOrder(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{fail: true}
	h.SetPayments(fake.client(t))

	rec, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number})
	if err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Status = %d, erwartet 303 zur Bestätigung", rec.Code)
	}
	if got := reload(t, h, ws, order.Number); got.PaymentStatus != shop.PaymentOpen {
		t.Errorf("Zahlungsstatus = %q, erwartet offen", got.PaymentStatus)
	}
}

// An unknown order number must not say whether it exists.
func TestPaymentReturnForAnUnknownOrder(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	h.SetPayments((&fakePayrexx{status: "confirmed"}).client(t))

	rec, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/2099-9999", map[string]string{"number": "2099-9999"})
	if err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, erwartet 404", rec.Code)
	}
}

// Without keys the payment routes must not exist at all.
func TestPaymentRoutesAreClosedWithoutKeys(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)
	h.SetPayments(&payrexx.Client{})

	rec, err := requestWithPathErr(h.HandlePaymentReturn, ws, "GET",
		paymentReturnPath+"/"+order.Number, map[string]string{"number": order.Number})
	if err != nil {
		t.Fatalf("HandlePaymentReturn: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("Rückkehr-Adresse antwortet mit %d, obwohl kein Zugang eingerichtet ist", rec.Code)
	}

	rec, err = postHook(h, ws, `{"transaction":{"id":4711}}`)
	if err != nil {
		t.Fatalf("HandlePaymentHook: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("Webhook antwortet mit %d, obwohl kein Zugang eingerichtet ist", rec.Code)
	}
}

// The webhook is the whole reason the amount check exists: this body is what a
// stranger would post. It names an order and calls it confirmed — and none of
// that is believed, because the gateway is read from the provider instead.
func TestWebhookIsVerifiedAgainstTheProvider(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	// The provider says the payment is still waiting.
	fake := &fakePayrexx{status: "waiting", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	forged := `{"transaction":{"id":4711,"status":"confirmed","amount":` +
		strconv.FormatInt(int64(order.Totals.TotalGross), 10) +
		`,"invoice":{"paymentRequestId":4711}}}`

	rec, err := postHook(h, ws, forged)
	if err != nil {
		t.Fatalf("HandlePaymentHook: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200", rec.Code)
	}
	if fake.calls == 0 {
		t.Error("der Anbieter wurde gar nicht gefragt — die Nutzlast wurde geglaubt")
	}
	if got := reload(t, h, ws, order.Number); got.PaymentStatus != shop.PaymentOpen {
		t.Errorf("Zahlungsstatus = %q; der Webhook-Inhalt wurde als Beleg genommen", got.PaymentStatus)
	}
}

func TestWebhookSettlesWhenTheProviderConfirms(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	order := placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "confirmed", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	if _, err := postHook(h, ws, `{"transaction":{"id":4711,"invoice":{"paymentRequestId":4711}}}`); err != nil {
		t.Fatalf("HandlePaymentHook: %v", err)
	}

	if got := reload(t, h, ws, order.Number); got.PaymentStatus != shop.PaymentPaid {
		t.Errorf("Zahlungsstatus = %q, erwartet %q", got.PaymentStatus, shop.PaymentPaid)
	}
}

// Rubbish must be answered with 200 and forgotten. An error makes the provider
// retry, and retrying will not turn rubbish into a payment.
func TestWebhookRubbishIsAccepted(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)
	placeTestOrder(t, h, database, ws)

	fake := &fakePayrexx{status: "confirmed"}
	h.SetPayments(fake.client(t))

	for _, body := range []string{"", "kein json", "{}", `{"transaction":{}}`} {
		rec, err := postHook(h, ws, body)
		if err != nil {
			t.Fatalf("HandlePaymentHook(%q): %v", body, err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%q: Status = %d, erwartet 200", body, rec.Code)
		}
	}
	if fake.calls != 0 {
		t.Errorf("der Anbieter wurde %d× wegen einer unlesbaren Nutzlast gefragt", fake.calls)
	}
}

// A body naming an order of another website must not settle it. Payrexx names
// the order; the website comes from the host the notification arrived on.
func TestWebhookCannotReachAnotherWebsitesOrder(t *testing.T) {
	h, database := newTestHandler(t)
	seller := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, seller)
	order := placeTestOrder(t, h, database, seller)

	other := seedWebsite(t, database, "Anderer Laden")
	shopSite(t, h, database, other)

	fake := &fakePayrexx{status: "confirmed", amount: int64(order.Totals.TotalGross),
		currency: "CHF", ref: order.Number}
	h.SetPayments(fake.client(t))

	// Posted to the other website's host.
	rec, err := postHook(h, other, `{"transaction":{"id":4711,"invoice":{"paymentRequestId":4711}}}`)
	if err != nil {
		t.Fatalf("HandlePaymentHook: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d", rec.Code)
	}
	if got := reload(t, h, seller, order.Number); got.PaymentStatus == shop.PaymentPaid {
		t.Error("eine Bestellung einer anderen Website wurde als bezahlt markiert")
	}
}

func TestPaymentMethodsDependOnConfiguration(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Laden")
	shopSite(t, h, database, ws)

	h.SetPayments(&payrexx.Client{})
	for _, m := range h.paymentMethods() {
		if m.Value == shop.PayPayrexx {
			t.Fatal("Online-Zahlung wird angeboten, obwohl kein Zugang eingerichtet ist")
		}
	}

	h.SetPayments(&payrexx.Client{Instance: "example", Secret: "geheim"})
	var found bool
	for _, m := range h.paymentMethods() {
		if m.Value == shop.PayPayrexx {
			found = true
		}
	}
	if !found {
		t.Error("Online-Zahlung fehlt, obwohl der Zugang eingerichtet ist")
	}
}

// The form must not accept a method the page never offered.
func TestOnlinePaymentIsRejectedWhenNotConfigured(t *testing.T) {
	v := checkoutValues{
		Email: "kundin@example.ch", Name: "Anna Meier", Street: "Seestrasse 4",
		PostalCode: "8002", City: "Zürich", Country: "CH",
		Method: shop.PayPayrexx, Accepted: true,
	}

	if errs := v.validate(shop.Private, false); errs["zahlungsart"] == "" {
		t.Error("eine nicht eingerichtete Zahlungsart wurde angenommen")
	}
	if errs := v.validate(shop.Private, true); len(errs) != 0 {
		t.Errorf("das Formular wurde abgelehnt, obwohl alles stimmt: %v", errs)
	}
}

func TestPaymentNoteSpeaksToTheCustomer(t *testing.T) {
	cases := []struct {
		method, status string
		want           string
	}{
		{shop.PayPayrexx, shop.PaymentPaid, "eingegangen"},
		{shop.PayPayrexx, shop.PaymentFailed, "nicht abgeschlossen"},
		{shop.PayPayrexx, shop.PaymentOpen, "noch nicht bestätigt"},
		{shop.PayPrepay, shop.PaymentOpen, "Zahlungsangaben"},
		{shop.PayInvoice, shop.PaymentOpen, "Rechnung"},
	}
	for _, tc := range cases {
		got := paymentNote(&shop.Order{PaymentMethod: tc.method, PaymentStatus: tc.status})
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s/%s: %q enthält %q nicht", tc.method, tc.status, got, tc.want)
		}
	}
}

// requestWithPath runs a handler with path values set, the way the mux would.
func requestWithPath(t *testing.T, h func(http.ResponseWriter, *http.Request) error,
	ws *domain.Website, method, target string, values map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rec, err := requestWithPathErr(h, ws, method, target, values)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	return rec
}

func requestWithPathErr(h func(http.ResponseWriter, *http.Request) error,
	ws *domain.Website, method, target string, values map[string]string) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "demo.test"
	for k, v := range values {
		req.SetPathValue(k, v)
	}
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	return rec, h(rec, req)
}

func postHook(h *Handler, ws *domain.Website, body string) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest("POST", paymentHookPath, strings.NewReader(body))
	req.Host = "demo.test"
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	return rec, h.HandlePaymentHook(rec, req)
}
