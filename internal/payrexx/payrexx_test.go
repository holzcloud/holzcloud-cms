package payrexx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// stand is a Payrexx look-alike.
//
// It is not a mock in the sense of "returns what the test wants": it verifies
// the signature the same way the real API does, so a request this package
// builds wrongly is rejected here rather than silently accepted. What it cannot
// prove is that the real API computes the string the same way — see Signature.
type stand struct {
	secret string
	// gateways answers GET by id.
	gateways map[int64]string
	// lastForm records what the last POST carried, for assertions.
	lastForm url.Values
	// lastQuery records the URL parameters of the last request.
	lastQuery url.Values
}

func newStand(secret string) *stand {
	return &stand{secret: secret, gateways: map[int64]string{}}
}

func (s *stand) sign(params url.Values) string {
	clone := url.Values{}
	for k, v := range params {
		if k != "ApiSignature" {
			clone[k] = v
		}
	}
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(clone.Encode()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *stand) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.lastQuery = r.URL.Query()

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			form, err := url.ParseQuery(string(body))
			if err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			s.lastForm = form
			if form.Get("ApiSignature") != s.sign(form) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, `{"status":"error","message":"Unauthorized"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"status":"success","data":[{"id":4711,"hash":"abc",`+
				`"status":"waiting","link":"https://pay.example.ch/?payment=abc",`+
				`"referenceId":"`+form.Get("referenceId")+`","amount":`+form.Get("amount")+
				`,"currency":"`+form.Get("currency")+`"}]}`)
			return
		}

		// GET /Gateway/{id}/
		if s.lastQuery.Get("ApiSignature") != s.sign(nil) {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"status":"error","message":"Unauthorized"}`)
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/Gateway/"), "/")
		body, ok := s.gateways[atoi(id)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"status":"error","message":"No gateway"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func atoi(s string) int64 {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int64(r-'0')
	}
	return n
}

func client(t *testing.T, s *stand) *Client {
	t.Helper()
	return &Client{Instance: "example", Secret: s.secret, BaseURL: s.server(t).URL}
}

func TestConfigured(t *testing.T) {
	cases := []struct {
		name string
		c    *Client
		want bool
	}{
		{"nichts", &Client{}, false},
		{"nur Instanz", &Client{Instance: "example"}, false},
		{"nur Schlüssel", &Client{Secret: "s"}, false},
		{"beides", &Client{Instance: "example", Secret: "s"}, true},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := tc.c.Configured(); got != tc.want {
			t.Errorf("%s: Configured() = %v, erwartet %v", tc.name, got, tc.want)
		}
	}
}

// A client that is not configured must fail before it opens a connection.
// Otherwise a shop with no keys would send its customers to api.payrexx.com.
func TestUnconfiguredNeverCalls(t *testing.T) {
	c := &Client{}
	if _, err := c.CreateGateway(context.Background(), GatewayRequest{}); err == nil {
		t.Error("CreateGateway ohne Schlüssel hat keinen Fehler geliefert")
	}
	if _, err := c.GetGateway(context.Background(), 1); err == nil {
		t.Error("GetGateway ohne Schlüssel hat keinen Fehler geliefert")
	}
}

func TestSignatureIsOrderIndependent(t *testing.T) {
	c := &Client{Instance: "example", Secret: "geheim"}

	a := url.Values{}
	a.Set("amount", "4900")
	a.Set("currency", "CHF")
	a.Set("referenceId", "2026-0007")

	b := url.Values{}
	b.Set("referenceId", "2026-0007")
	b.Set("amount", "4900")
	b.Set("currency", "CHF")

	if c.Signature(a) != c.Signature(b) {
		t.Error("dieselben Parameter in anderer Reihenfolge ergeben eine andere Signatur")
	}
}

// The signature parameter must not sign itself, and it must not depend on
// whatever was in the map before it was set.
func TestSignatureExcludesItself(t *testing.T) {
	c := &Client{Instance: "example", Secret: "geheim"}

	params := url.Values{}
	params.Set("amount", "4900")
	want := c.Signature(params)

	params.Set("ApiSignature", "irgendwas")
	if got := c.Signature(params); got != want {
		t.Error("die Signatur ändert sich, wenn ApiSignature bereits gesetzt ist")
	}
}

func TestSignatureDependsOnSecret(t *testing.T) {
	params := url.Values{"amount": {"4900"}}
	a := (&Client{Secret: "eins"}).Signature(params)
	b := (&Client{Secret: "zwei"}).Signature(params)
	if a == b {
		t.Error("zwei verschiedene Schlüssel ergeben dieselbe Signatur")
	}
}

func TestCreateGateway(t *testing.T) {
	s := newStand("geheim")
	c := client(t, s)

	gw, err := c.CreateGateway(context.Background(), GatewayRequest{
		Amount:             491200,
		Currency:           "CHF",
		Purpose:            "Bestellung 2026-0007",
		ReferenceID:        "2026-0007",
		SuccessRedirectURL: "https://example.ch/zahlung/zurueck/2026-0007",
		FailedRedirectURL:  "https://example.ch/zahlung/zurueck/2026-0007",
		CancelRedirectURL:  "https://example.ch/zahlung/zurueck/2026-0007",
		Prefill: map[string]string{
			"email":    "kundin@example.ch",
			"forename": "Anna",
			// An empty field must not be sent at all: Payrexx would show it as
			// filled in with nothing and the customer cannot tell it apart from
			// a field they emptied themselves.
			"surname": "",
		},
	})
	if err != nil {
		t.Fatalf("CreateGateway: %v", err)
	}

	if gw.ID != 4711 || gw.Link == "" {
		t.Fatalf("unerwartetes Gateway: %+v", gw)
	}
	if s.lastForm.Get("amount") != "491200" {
		t.Errorf("Betrag = %q, erwartet 491200 Rappen", s.lastForm.Get("amount"))
	}
	if s.lastForm.Get("referenceId") != "2026-0007" {
		t.Errorf("referenceId = %q", s.lastForm.Get("referenceId"))
	}
	if s.lastForm.Get("fields[email][value]") != "kundin@example.ch" {
		t.Errorf("E-Mail nicht vorbelegt: %q", s.lastForm.Get("fields[email][value]"))
	}
	if _, ok := s.lastForm["fields[surname][value]"]; ok {
		t.Error("ein leeres Vorbelegungsfeld wurde mitgeschickt")
	}
	if s.lastQuery.Get("instance") != "example" {
		t.Errorf("instance = %q", s.lastQuery.Get("instance"))
	}
}

// The methods list is what restricts a gateway to TWINT alone. Sent as
// repeated pm[] parameters, which is the shape the API documents.
func TestCreateGatewayCarriesMethods(t *testing.T) {
	s := newStand("geheim")
	c := client(t, s)

	if _, err := c.CreateGateway(context.Background(), GatewayRequest{
		Amount: 100, Currency: "CHF", Methods: []string{"twint", "visa"},
	}); err != nil {
		t.Fatalf("CreateGateway: %v", err)
	}

	got := s.lastForm["pm[]"]
	if len(got) != 2 || got[0] != "twint" || got[1] != "visa" {
		t.Errorf("pm[] = %v", got)
	}
}

// The signature is what the stand-in checks. A wrong secret has to be rejected
// there, which is the closest this suite gets to proving the signature is
// actually being computed and sent.
func TestWrongSecretIsRejected(t *testing.T) {
	s := newStand("geheim")
	c := client(t, s)
	c.Secret = "falsch"

	_, err := c.CreateGateway(context.Background(), GatewayRequest{Amount: 100, Currency: "CHF"})
	if err == nil {
		t.Fatal("ein falscher Schlüssel wurde akzeptiert")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Fehler nennt den Grund nicht: %v", err)
	}
}

func TestGetGateway(t *testing.T) {
	s := newStand("geheim")
	s.gateways[4711] = `{"status":"success","data":[{"id":4711,"status":"confirmed",` +
		`"referenceId":"2026-0007","amount":491200,"currency":"CHF"}]}`
	c := client(t, s)

	gw, err := c.GetGateway(context.Background(), 4711)
	if err != nil {
		t.Fatalf("GetGateway: %v", err)
	}
	if !gw.Paid() {
		t.Errorf("Status %q gilt nicht als bezahlt", gw.Status)
	}
	if gw.ReferenceID != "2026-0007" {
		t.Errorf("referenceId = %q", gw.ReferenceID)
	}
	if gw.Amount != 491200 {
		t.Errorf("Betrag = %d", gw.Amount)
	}
}

func TestGetGatewayUnknown(t *testing.T) {
	s := newStand("geheim")
	c := client(t, s)
	if _, err := c.GetGateway(context.Background(), 9999); err == nil {
		t.Error("ein unbekanntes Gateway hat keinen Fehler geliefert")
	}
}

// This is the test that matters most. An "authorized" gateway is money that is
// being held, not money that has arrived; shipping against it gives goods away.
func TestOnlyConfirmedIsPaid(t *testing.T) {
	cases := map[string]struct{ paid, failed bool }{
		StatusWaiting:    {false, false},
		StatusConfirmed:  {true, false},
		StatusAuthorized: {false, false},
		StatusReserved:   {false, false},
		StatusCancelled:  {false, true},
		StatusDeclined:   {false, true},
		StatusError:      {false, true},
		StatusRefunded:   {false, false},
		"etwas neues":    {false, false},
	}
	for status, want := range cases {
		gw := &Gateway{Status: status}
		if gw.Paid() != want.paid {
			t.Errorf("%q: Paid() = %v, erwartet %v", status, gw.Paid(), want.paid)
		}
		if gw.Failed() != want.failed {
			t.Errorf("%q: Failed() = %v, erwartet %v", status, gw.Failed(), want.failed)
		}
	}

	var missing *Gateway
	if missing.Paid() || missing.Failed() {
		t.Error("ein fehlendes Gateway gilt als bezahlt oder gescheitert")
	}
}

func TestErrorsAreReadable(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
		want string
	}{
		{"kein JSON", "<html>Wartung</html>", 200, "not JSON"},
		{"Fehlerstatus", `{"status":"error","message":"Invalid currency"}`, 200, "Invalid currency"},
		{"Fehler ohne Text", `{"status":"error"}`, 200, "error"},
		{"leere Liste", `{"status":"success","data":[]}`, 200, "no gateway"},
		{"falsche Nutzlast", `{"status":"success","data":{"id":1}}`, 200, "unexpected"},
		{"HTTP 500", `Internal Server Error`, 500, "500"},
	}

	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			io.WriteString(w, tc.body)
		}))
		c := &Client{Instance: "example", Secret: "geheim", BaseURL: srv.URL}
		_, err := c.CreateGateway(context.Background(), GatewayRequest{Amount: 1, Currency: "CHF"})
		srv.Close()

		if err == nil {
			t.Errorf("%s: kein Fehler", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v enthält %q nicht", tc.name, err, tc.want)
		}
	}
}

// A provider that answers with a gigabyte must not take the shop down.
func TestOversizedResponseIsBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"success","data":[`)
		for i := 0; i < 200000; i++ {
			io.WriteString(w, `{"id":1},`)
		}
		io.WriteString(w, `{"id":2}]}`)
	}))
	defer srv.Close()

	c := &Client{Instance: "example", Secret: "geheim", BaseURL: srv.URL}
	if _, err := c.GetGateway(context.Background(), 1); err == nil {
		t.Error("eine überlange Antwort wurde als gültig gelesen")
	}
}

func TestContextCancelStopsTheCall(t *testing.T) {
	s := newStand("geheim")
	c := client(t, s)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.CreateGateway(ctx, GatewayRequest{Amount: 1, Currency: "CHF"}); err == nil {
		t.Error("ein abgebrochener Kontext hat den Aufruf nicht gestoppt")
	}
}

func TestTransactionIDFromWebhook(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int64
		ok   bool
	}{
		{"Gateway unter der Rechnung",
			`{"transaction":{"id":88,"status":"confirmed","invoice":{"paymentRequestId":4711}}}`,
			4711, true},
		{"nur die Transaktion",
			`{"transaction":{"id":88,"status":"confirmed"}}`, 88, true},
		{"kein JSON", `nicht json`, 0, false},
		{"leer", `{}`, 0, false},
		{"Null-Kennung", `{"transaction":{"id":0}}`, 0, false},
		{"falscher Typ", `{"transaction":{"id":"88"}}`, 0, false},
	}
	for _, tc := range cases {
		got, ok := TransactionIDFromWebhook([]byte(tc.body))
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: (%d, %v), erwartet (%d, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// A webhook claiming an order is paid must not be believed. The only thing
// taken out of the body is the id; the status in it is ignored.
func TestWebhookStatusIsIgnored(t *testing.T) {
	body := []byte(`{"transaction":{"id":88,"status":"confirmed",` +
		`"amount":1,"invoice":{"paymentRequestId":4711}}}`)

	id, ok := TransactionIDFromWebhook(body)
	if !ok || id != 4711 {
		t.Fatalf("Kennung = %d, %v", id, ok)
	}
	// Nothing in the package turns a webhook body into a Gateway — that is the
	// point. If a future change adds such a function, this test should be the
	// one that makes someone stop and think.
	if _, err := (&Client{}).GetGateway(context.Background(), id); err == nil {
		t.Error("die Bestätigung lief ohne eingerichteten Zugang durch")
	}
}
