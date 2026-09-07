package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The order screen carries a third id, and it is the one nobody looks at. The
// website comes from the address and is authorised; the order comes from the
// address and is fetched scoped to that website; the message id comes from the
// posted form and is handed to the outbox as it stands.
//
// Re-queueing a foreign website's message is not a defacement, it is a delivery:
// somebody else's customer receives the mail a second time, and the failure the
// other operator was about to look into has had its error text and its attempt
// count wiped.

type orderScopeFixture struct {
	handler *Handler
	sm      *scs.SessionManager
	db      *db.DB
	outbox  *outbox.Store

	siteA, siteB  *domain.Website
	orderA        *shop.Order
	mailB         int64
	editorOnlyOnA int64
}

func newOrderScopeFixture(t *testing.T) *orderScopeFixture {
	t.Helper()
	ctx := context.Background()

	h, sm, database, siteA := newTestAdmin(t)

	products := shop.NewStore(database)
	carts := shop.NewCartStore(products)
	orders := shop.NewOrderStore(carts)
	mails := outbox.NewStore(database)
	h.SetProductStore(products)
	h.SetOrderStore(orders)
	h.SetOutbox(mails)

	domains := domain.NewStore(database)
	siteB, err := domains.CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}

	// An order on the editor's own website, so the request gets as far as the
	// action switch. Without it the handler stops at the order lookup and the
	// test would prove nothing about the message id.
	p := seedProduct(t, products, siteA.ID, "hocker", "Hocker", 4900)
	cart, token, err := carts.Ensure(ctx, siteA.ID, "")
	if err != nil {
		t.Fatalf("Ensure cart: %v", err)
	}
	if err := carts.Add(ctx, cart.ID, p.ID, 1); err != nil {
		t.Fatalf("Add to cart: %v", err)
	}
	cart, err = carts.Get(ctx, siteA.ID, token)
	if err != nil {
		t.Fatalf("Get cart: %v", err)
	}
	set := shop.Settings{Currency: money.CurrencyFor("CHF")}
	order, err := orders.Place(ctx, siteA.ID, set, shop.Private, cart,
		shop.Customer{Email: "kundin-a@example.ch", Name: "Anna Meier",
			Street: "Seestrasse 4", PostalCode: "8002", City: "Zürich", Country: "CH"},
		shop.PayInvoice)
	if err != nil {
		t.Fatalf("Place order: %v", err)
	}

	// A message belonging to the other website, given up on, which is exactly
	// the state in which the admin offers the "send again" button.
	mailB, err := mails.Queue(ctx, outbox.Mail{
		WebsiteID: siteB.ID,
		Kind:      outbox.KindOrderCustomer,
		Recipient: "kundin-b@example.ch",
		Subject:   "Ihre Bestellung bei Holzbau B",
		Body:      "Vielen Dank für Ihre Bestellung.",
	})
	if err != nil {
		t.Fatalf("Queue mail B: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE outbox SET status = 'failed', attempts = 5,
		 last_error = 'connection refused' WHERE id = $1`, mailB); err != nil {
		t.Fatalf("mark mail B failed: %v", err)
	}

	users := user.NewStore(database, cheapHashing)
	editorID, err := users.Create(ctx, "Editor A", "editor-a@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, editorID, user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}

	return &orderScopeFixture{
		handler: h, sm: sm, db: database, outbox: mails,
		siteA: siteA, siteB: siteB, orderA: order, mailB: mailB,
		editorOnlyOnA: editorID,
	}
}

func (f *orderScopeFixture) post(t *testing.T, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/websites/{id}/bestellungen/{number}", f.handler.ErrHandler(f.handler.HandleOrderDetail))
	guarded := auth.RequireWebsiteAccess(f.sm, NewWebsiteAccessLookup(f.db))(mux)

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.sm.Put(r.Context(), auth.SessionKeyUserID, f.editorOnlyOnA)
		guarded.ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

// mailState reads the three columns a retry rewrites.
func (f *orderScopeFixture) mailState(t *testing.T, id int64) (status string, attempts int, lastError string) {
	t.Helper()
	if err := f.db.Read.QueryRow(
		`SELECT status, attempts, last_error FROM outbox WHERE id = $1`, id).
		Scan(&status, &attempts, &lastError); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	return status, attempts, lastError
}

// TestOrderRetryMailRefusesAForeignWebsitesMessage proves that the message id
// on the order screen is not scoped to the website in the address.
func TestOrderRetryMailRefusesAForeignWebsitesMessage(t *testing.T) {
	f := newOrderScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/bestellungen/%s", f.siteA.ID, f.orderA.Number)
	f.post(t, target, url.Values{
		"aktion":  {"mail-erneut"},
		"mail_id": {fmt.Sprint(f.mailB)},
	})

	status, attempts, lastError := f.mailState(t, f.mailB)
	if status != outbox.StatusFailed {
		t.Errorf("website B's message was put back in the queue: status %q, want %q",
			status, outbox.StatusFailed)
	}
	if attempts != 5 {
		t.Errorf("website B's attempt counter was reset from 5 to %d", attempts)
	}
	if lastError == "" {
		t.Error("website B's error text was cleared, so its operator can no longer see what went wrong")
	}
}

// TestOrderRetryMailStillServesItsOwnWebsite is the control. The operator of
// the website that owns the message must still be able to send it again, or the
// button would be broken for everyone.
func TestOrderRetryMailStillServesItsOwnWebsite(t *testing.T) {
	f := newOrderScopeFixture(t)
	ctx := context.Background()

	own, err := f.outbox.Queue(ctx, outbox.Mail{
		WebsiteID: f.siteA.ID,
		Kind:      outbox.KindOrderCustomer,
		OrderID:   &f.orderA.ID,
		Recipient: "kundin-a@example.ch",
		Subject:   "Ihre Bestellung",
		Body:      "Vielen Dank.",
	})
	if err != nil {
		t.Fatalf("Queue own mail: %v", err)
	}
	if _, err := f.db.Write.ExecContext(ctx,
		`UPDATE outbox SET status = 'failed', attempts = 5,
		 last_error = 'connection refused' WHERE id = $1`, own); err != nil {
		t.Fatalf("mark own mail failed: %v", err)
	}

	target := fmt.Sprintf("/admin/websites/%d/bestellungen/%s", f.siteA.ID, f.orderA.Number)
	f.post(t, target, url.Values{
		"aktion":  {"mail-erneut"},
		"mail_id": {fmt.Sprint(own)},
	})

	status, attempts, lastError := f.mailState(t, own)
	if status != outbox.StatusPending {
		t.Errorf("the own message was not re-queued: status %q, want %q", status, outbox.StatusPending)
	}
	if attempts != 0 {
		t.Errorf("the own message kept %d attempts, want the counter reset to 0", attempts)
	}
	if lastError != "" {
		t.Errorf("the own message kept its error text %q", lastError)
	}
}
