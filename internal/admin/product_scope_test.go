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
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The product form takes two ids that can disagree: the website in the address
// and the product in the rest of the path. RequireWebsiteAccess only ever reads
// the first one — it says so itself: "everything under
// /admin/websites/<number> belongs to that website, whatever the rest of the
// path turns out to be". So a person who may enter website A passes the guard
// with A in the address and can name website B's product behind it.
//
// The GET arm of the same handler and the delete handler beside it both close
// that gap by comparing p.WebsiteID against the website in the address. These
// tests hold the save arm to the same promise, and they assert the effect
// rather than only the status: a refusal that still changed the price would be
// no refusal at all.

// productScopeFixture is two websites that must not be able to reach each
// other, and one person who may only enter the first.
type productScopeFixture struct {
	handler  *Handler
	sm       *scs.SessionManager
	db       *db.DB
	products *shop.Store

	siteA, siteB  *domain.Website
	productA      *shop.Product
	productB      *shop.Product
	editorOnlyOnA int64
}

func newProductScopeFixture(t *testing.T) *productScopeFixture {
	t.Helper()
	ctx := context.Background()

	h, sm, database, siteA := newTestAdmin(t)

	// newTestAdmin builds a handler without a shop, because most admin tests do
	// not need one. Without this the product routes answer 404 for the honest
	// reason and would hide the dishonest one.
	products := shop.NewStore(database)
	h.SetProductStore(products)

	domains := domain.NewStore(database)
	siteB, err := domains.CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}

	f := &productScopeFixture{handler: h, sm: sm, db: database, products: products, siteA: siteA, siteB: siteB}
	f.productA = seedProduct(t, products, siteA.ID, "eigener-tisch", "Eigener Tisch", 129000)
	f.productB = seedProduct(t, products, siteB.ID, "fremder-tisch", "Fremder Tisch", 249000)

	// The person under test. An assignment must exist, because no assignment at
	// all means every website — see NewWebsiteAccessLookup.
	users := user.NewStore(database, cheapHashing)
	editorID, err := users.Create(ctx, "Editor A", "editor-a@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if err := users.SetRights(ctx, editorID, user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("SetRights: %v", err)
	}
	f.editorOnlyOnA = editorID

	// The premise of the whole attack: the guard admits this person on A and
	// refuses them on B. If this ever stops holding, the tests below are
	// measuring something other than what they claim.
	allowed := NewWebsiteAccessLookup(database)
	if !allowed(ctx, editorID, siteA.ID) {
		t.Fatal("the editor should be allowed on website A")
	}
	if allowed(ctx, editorID, siteB.ID) {
		t.Fatal("the editor should not be allowed on website B")
	}
	return f
}

func seedProduct(t *testing.T, products *shop.Store, websiteID int64, slug, title string, price money.Amount) *shop.Product {
	t.Helper()
	ctx := context.Background()

	id, err := products.Create(ctx, &shop.Product{
		WebsiteID:    websiteID,
		Slug:         slug,
		Title:        title,
		Subtitle:     "Eiche massiv, geölt",
		SKU:          "TI-100",
		PriceGross:   price,
		TaxRate:      money.RateStandard,
		DeliveryNote: "Lieferzeit 3–4 Wochen",
		Status:       shop.StatusPublished,
	})
	if err != nil {
		t.Fatalf("Create product %s: %v", slug, err)
	}
	p, err := products.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get product %s: %v", slug, err)
	}
	return p
}

// attackerForm is what the intruder would type. The price is the point: five
// rappen for a table, published, so the damage is a real sale at a real
// address rather than an untidy row.
func attackerForm() url.Values {
	return url.Values{
		"title":                {"Übernommen"},
		"slug":                 {"uebernommen"},
		"subtitle":             {"Räumungsverkauf"},
		"description_markdown": {"Alles muss raus."},
		"sku":                  {"XX-000"},
		"price":                {"0.05"},
		"tax_bp":               {"810"},
		"stock":                {""},
		"weight_grams":         {"0"},
		"delivery_note":        {"Sofort"},
		"status":               {shop.StatusPublished},
		"terms":                {"Schnäppchen"},
	}
}

// post drives one request through the same chain main.go builds: the session,
// then RequireWebsiteAccess, then the routed mux. Going through the real
// middleware is the point — a handler-only test could not show that the guard
// lets the request past.
func (f *productScopeFixture) post(t *testing.T, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/websites/{id}/produkte/{productID}", f.handler.ErrHandler(f.handler.HandleProductForm))
	mux.HandleFunc("POST /admin/websites/{id}/produkte/{productID}/delete", f.handler.ErrHandler(f.handler.HandleProductDelete))

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

func (f *productScopeFixture) reread(t *testing.T, id int64) *shop.Product {
	t.Helper()
	p, err := f.products.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p == nil {
		t.Fatalf("product %d disappeared", id)
	}
	return p
}

// TestProductSaveRefusesAForeignWebsitesProduct is the proof. An editor
// confined to website A must not be able to rewrite website B's product by
// naming A in the address.
func TestProductSaveRefusesAForeignWebsitesProduct(t *testing.T) {
	f := newProductScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/produkte/%d", f.siteA.ID, f.productB.ID)
	rec := f.post(t, target, attackerForm())
	refused(t, rec, "save")

	after := f.reread(t, f.productB.ID)
	if after.Title != f.productB.Title {
		t.Errorf("website B's product was retitled from %q to %q", f.productB.Title, after.Title)
	}
	if after.PriceGross != f.productB.PriceGross {
		t.Errorf("website B's product was repriced from %s to %s",
			money.Input(f.productB.PriceGross), money.Input(after.PriceGross))
	}
	if after.Slug != f.productB.Slug {
		t.Errorf("website B's product was moved from %q to %q", f.productB.Slug, after.Slug)
	}
	if after.Status != f.productB.Status {
		t.Errorf("website B's product changed status from %q to %q", f.productB.Status, after.Status)
	}
}

// TestProductSaveDoesNotTouchAForeignWebsitesCategories covers the collateral
// damage. setProductTerms opens with an unscoped DELETE FROM product_terms, so
// a save that reaches a foreign product also empties its categories — a second
// effect that a status-only assertion would miss.
func TestProductSaveDoesNotTouchAForeignWebsitesCategories(t *testing.T) {
	f := newProductScopeFixture(t)
	ctx := context.Background()

	if err := f.handler.terms.SetForProduct(ctx, f.siteB.ID, f.productB.ID, []string{"Tische", "Massivholz"}); err != nil {
		t.Fatalf("SetForProduct: %v", err)
	}
	before, err := f.products.TermIDs(ctx, f.productB.ID)
	if err != nil {
		t.Fatalf("TermIDs: %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("fixture: website B's product should start with 2 categories, has %d", len(before))
	}

	target := fmt.Sprintf("/admin/websites/%d/produkte/%d", f.siteA.ID, f.productB.ID)
	rec := f.post(t, target, attackerForm())
	refused(t, rec, "save")

	after, err := f.products.TermIDs(ctx, f.productB.ID)
	if err != nil {
		t.Fatalf("TermIDs: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("website B's product went from %d categories to %d", len(before), len(after))
	}
}

// TestProductSaveRefusesAForeignProductIdEvenForAnAdmin pins the second half of
// the rule. An administrator passes RequireWebsiteAccess on every website, so
// the guard cannot help here at all; the address and the product must still
// agree, or the admin UI would silently rewrite the wrong site's catalogue
// after a mistyped link.
func TestProductSaveRefusesAForeignProductIdEvenForAnAdmin(t *testing.T) {
	f := newProductScopeFixture(t)
	ctx := context.Background()

	users := user.NewStore(f.db, cheapHashing)
	adminID, err := users.Create(ctx, "Chefin", "admin@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	f.editorOnlyOnA = adminID

	target := fmt.Sprintf("/admin/websites/%d/produkte/%d", f.siteA.ID, f.productB.ID)
	rec := f.post(t, target, attackerForm())
	if rec.Code != http.StatusNotFound {
		t.Errorf("save with a mismatched product id: status %d, want 404", rec.Code)
	}

	after := f.reread(t, f.productB.ID)
	if after.Title != f.productB.Title || after.PriceGross != f.productB.PriceGross {
		t.Errorf("website B's product was rewritten through website A's address: %q at %s",
			after.Title, money.Input(after.PriceGross))
	}
}

// TestProductSaveStillServesItsOwnWebsite is the other half of the guarantee. A
// fix that refuses everything would pass every test above and break the shop,
// so the same person editing their own product must still succeed.
func TestProductSaveStillServesItsOwnWebsite(t *testing.T) {
	f := newProductScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/produkte/%d", f.siteA.ID, f.productA.ID)
	rec := f.post(t, target, attackerForm())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save on the own website: status %d, want 303", rec.Code)
	}

	after := f.reread(t, f.productA.ID)
	if after.Title != "Übernommen" {
		t.Errorf("save on the own website did not take: title %q", after.Title)
	}
	if after.PriceGross != 5 {
		t.Errorf("save on the own website did not take: price %s", money.Input(after.PriceGross))
	}

	terms, err := f.products.TermIDs(context.Background(), f.productA.ID)
	if err != nil {
		t.Fatalf("TermIDs: %v", err)
	}
	if len(terms) != 1 {
		t.Errorf("save on the own website did not set the category: %d attached", len(terms))
	}
}

// TestProductCreateStillWorks guards the boundary the fix could most easily
// break: "neu" is not a product id, so the new check must not run for it.
func TestProductCreateStillWorks(t *testing.T) {
	f := newProductScopeFixture(t)

	form := attackerForm()
	form.Set("slug", "ganz-neu")
	form.Set("title", "Ganz neu")

	rec := f.post(t, fmt.Sprintf("/admin/websites/%d/produkte/neu", f.siteA.ID), form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create on the own website: status %d, want 303", rec.Code)
	}

	list, err := f.products.List(context.Background(), f.siteA.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("create did not add a product: website A has %d", len(list))
	}
}

// TestProductDeleteRefusesAForeignWebsitesProduct re-proves the neighbour the
// report calls clean. A check that is only asserted by reading is a check that
// can be deleted by the next refactor without anything going red.
func TestProductDeleteRefusesAForeignWebsitesProduct(t *testing.T) {
	f := newProductScopeFixture(t)

	target := fmt.Sprintf("/admin/websites/%d/produkte/%d/delete", f.siteA.ID, f.productB.ID)
	rec := f.post(t, target, url.Values{})
	refused(t, rec, "delete")

	if p := f.reread(t, f.productB.ID); p.Title != f.productB.Title {
		t.Errorf("website B's product changed under a delete: %q", p.Title)
	}
}
