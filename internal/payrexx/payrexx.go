// Package payrexx talks to the Payrexx Gateway API.
//
// Payrexx is a Swiss payment service provider. A gateway is created on their
// side, the customer is sent there to pay, and they come back. TWINT, PostFinance
// and cards all run through the same gateway, so nothing here knows about a
// particular payment method.
//
// The customer's card details never touch this server: the browser is redirected
// to Payrexx and back. That is the whole reason for choosing a redirect gateway
// over an embedded form — a small self-hosted shop has no business being in
// the scope of a card-data audit.
//
// Written against the published API description. Everything here is exercised
// against a stand-in server in the tests; the one thing a stand-in cannot prove
// is that Payrexx computes the request signature over the same byte string this
// package does. See Signature for what was assumed and why.
package payrexx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the live API.
const DefaultBaseURL = "https://api.payrexx.com/v1.0"

// Gateway states, as the API reports them.
const (
	StatusWaiting    = "waiting"
	StatusConfirmed  = "confirmed"
	StatusAuthorized = "authorized"
	StatusReserved   = "reserved"
	StatusCancelled  = "cancelled"
	StatusDeclined   = "declined"
	StatusRefunded   = "refunded"
	StatusError      = "error"
)

// Client is a configured connection to one Payrexx instance.
type Client struct {
	// Instance is the name in front of .payrexx.com — the account, not a URL.
	Instance string
	// Secret is the API key. It comes from the environment, never from the
	// database: the database is what gets copied to a backup file, and a
	// payment key in a backup is a payment key in every copy of it.
	Secret string
	// BaseURL is overridden in tests. Empty means the live API.
	BaseURL string
	// HTTP is overridable so a caller can set its own timeouts.
	HTTP *http.Client
}

// Configured reports whether payments can be taken at all.
func (c *Client) Configured() bool {
	return c != nil && c.Instance != "" && c.Secret != ""
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimSuffix(c.BaseURL, "/")
	}
	return DefaultBaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	// A payment provider that has stopped answering must not hold a checkout
	// request open until the browser gives up.
	return &http.Client{Timeout: 20 * time.Second}
}

// Signature is the ApiSignature Payrexx expects.
//
// It is an HMAC-SHA256 of the request's parameters as a query string, keyed with
// the API secret, base64-encoded. The parameter that carries the signature is
// itself excluded, and a request with no parameters signs the empty string.
//
// The one assumption that cannot be checked without a live account: the order of
// the parameters in that query string. url.Values.Encode sorts by key, which is
// what every non-PHP implementation of this signature does and what makes the
// result reproducible at all — PHP's http_build_query would otherwise preserve
// insertion order. If the first live call is rejected as unauthorised, this is
// the line to look at before anything else.
func (c *Client) Signature(params url.Values) string {
	query := ""
	if params != nil {
		clone := url.Values{}
		for k, v := range params {
			if k == "ApiSignature" {
				continue
			}
			clone[k] = v
		}
		query = clone.Encode()
	}

	mac := hmac.New(sha256.New, []byte(c.Secret))
	mac.Write([]byte(query))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// GatewayRequest is a payment to be set up.
type GatewayRequest struct {
	// Amount is in the currency's smallest unit — rappen for CHF.
	Amount   int64
	Currency string
	// Purpose is what the customer sees on the payment page and on their
	// statement.
	Purpose string
	// ReferenceID is our order number, handed back to us on the way out and in
	// every webhook. It is what ties a payment to an order.
	ReferenceID string

	SuccessRedirectURL string
	FailedRedirectURL  string
	CancelRedirectURL  string

	// Methods restricts the payment methods offered — "twint", "visa",
	// "mastercard", "post_finance_card". Empty offers whatever the account has
	// enabled, which is the sensible default: the operator decides that in
	// their Payrexx account, not in this CMS.
	Methods []string

	// Prefill are the customer fields Payrexx should show already filled in.
	// Keys are Payrexx field names: forename, surname, email, street,
	// postcode, place, country.
	Prefill map[string]string
}

// Gateway is what the API returns.
type Gateway struct {
	ID     int64  `json:"id"`
	Hash   string `json:"hash"`
	Status string `json:"status"`
	// Link is where the customer is sent to pay.
	Link        string `json:"link"`
	ReferenceID string `json:"referenceId"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

// Paid reports whether the money has actually arrived.
//
// "confirmed" is the only state that means paid. "authorized" and "reserved"
// mean an amount is being held and will need capturing — treating either as
// paid is how goods leave the workshop against money that was never taken.
func (g *Gateway) Paid() bool { return g != nil && g.Status == StatusConfirmed }

// Failed reports whether this payment will not happen.
func (g *Gateway) Failed() bool {
	if g == nil {
		return false
	}
	switch g.Status {
	case StatusCancelled, StatusDeclined, StatusError:
		return true
	}
	return false
}

// apiResponse is the envelope every endpoint answers with.
type apiResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// CreateGateway sets up a payment and returns where to send the customer.
func (c *Client) CreateGateway(ctx context.Context, req GatewayRequest) (*Gateway, error) {
	if !c.Configured() {
		return nil, errors.New("payrexx ist nicht eingerichtet")
	}

	params := url.Values{}
	params.Set("amount", strconv.FormatInt(req.Amount, 10))
	params.Set("currency", req.Currency)
	params.Set("purpose", req.Purpose)
	params.Set("referenceId", req.ReferenceID)
	params.Set("successRedirectUrl", req.SuccessRedirectURL)
	params.Set("failedRedirectUrl", req.FailedRedirectURL)
	params.Set("cancelRedirectUrl", req.CancelRedirectURL)
	// Nothing may be skipped: a gateway that lets the customer walk past the
	// address form leaves an order with no delivery address.
	params.Set("skipResultPage", "0")
	for _, m := range req.Methods {
		params.Add("pm[]", m)
	}
	for k, v := range req.Prefill {
		if v != "" {
			params.Set("fields["+k+"][value]", v)
		}
	}
	params.Set("ApiSignature", c.Signature(params))

	body, err := c.do(ctx, http.MethodPost,
		c.base()+"/Gateway/?instance="+url.QueryEscape(c.Instance), params)
	if err != nil {
		return nil, err
	}
	return firstGateway(body)
}

// GetGateway reads a payment's current state.
//
// This is the only thing that decides whether an order is paid. A webhook body
// is a notification that something happened, never the evidence of what: it
// arrives over the open internet and anyone can post one.
func (c *Client) GetGateway(ctx context.Context, id int64) (*Gateway, error) {
	if !c.Configured() {
		return nil, errors.New("payrexx ist nicht eingerichtet")
	}

	// A GET carries no parameters, so the signature is over the empty string.
	sig := c.Signature(nil)
	endpoint := fmt.Sprintf("%s/Gateway/%d/?instance=%s&ApiSignature=%s",
		c.base(), id, url.QueryEscape(c.Instance), url.QueryEscape(sig))

	body, err := c.do(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return firstGateway(body)
}

func (c *Client) do(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
	var reader io.Reader
	if params != nil {
		reader = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("payrexx request: %w", err)
	}
	if params != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("payrexx unreachable: %w", err)
	}
	defer res.Body.Close()

	// Bounded: a provider answering with a gigabyte must not take the shop
	// down with it.
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read payrexx response: %w", err)
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("payrexx answered %d: %s", res.StatusCode, snippet(body))
	}
	return body, nil
}

func firstGateway(body []byte) (*Gateway, error) {
	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("payrexx sent something that is not JSON: %s", snippet(body))
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("payrexx refused: %s", firstNonEmpty(envelope.Message, envelope.Status))
	}

	var gateways []Gateway
	if err := json.Unmarshal(envelope.Data, &gateways); err != nil {
		return nil, fmt.Errorf("payrexx sent an unexpected payload: %s", snippet(envelope.Data))
	}
	if len(gateways) == 0 {
		return nil, errors.New("payrexx returned no gateway")
	}
	return &gateways[0], nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unbekannter Fehler"
}

// snippet trims a response for an error message.
func snippet(body []byte) string {
	const max = 200
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// TransactionIDFromWebhook reads the gateway id out of a webhook body.
//
// Only the id is taken. Everything else in the payload — the amount, the status,
// the reference — is ignored on purpose: the webhook arrives over the open
// internet with no shared secret, so anyone who guesses the address can post
// one. The id is used to ask Payrexx directly what happened, and that answer is
// what counts.
func TransactionIDFromWebhook(body []byte) (int64, bool) {
	var payload struct {
		Transaction struct {
			ID      int64 `json:"id"`
			Invoice struct {
				// Payrexx nests the gateway under the invoice of a transaction.
				PaymentRequestID int64 `json:"paymentRequestId"`
			} `json:"invoice"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, false
	}
	if id := payload.Transaction.Invoice.PaymentRequestID; id > 0 {
		return id, true
	}
	if id := payload.Transaction.ID; id > 0 {
		return id, true
	}
	return 0, false
}
