package ai

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// The shop tools with a stand-in for the admin handler. The admin package
// cannot be imported here (it imports this one), and its own tests hold the Op
// methods to the screens' rules; what is tested here is the tools' side: the
// scope, the key levels, and that the arguments arrive as the form would carry
// them.

type fakeShop struct {
	products map[int64]*shop.Product
	inputs   map[int64]shop.ProductInput
	saved    []shop.ProductInput
	deleted  []int64
	order    *shop.Order
	statuses []string
	settings map[int64]shop.SettingsInput
	logged   []activity.Entry
	nextID   int64
}

func newFakeShop() *fakeShop {
	return &fakeShop{
		products: map[int64]*shop.Product{},
		inputs:   map[int64]shop.ProductInput{},
		settings: map[int64]shop.SettingsInput{},
		nextID:   100,
	}
}

func (f *fakeShop) OpLog(_ context.Context, _ string, e activity.Entry) {
	f.logged = append(f.logged, e)
}
func (f *fakeShop) OpChanged(int64) {}

func (f *fakeShop) own(websiteID, id int64) (*shop.Product, error) {
	p, ok := f.products[id]
	if !ok || p.WebsiteID != websiteID {
		return nil, shop.ErrNotFound
	}
	return p, nil
}

func (f *fakeShop) OpListProducts(_ context.Context, websiteID int64) ([]*shop.Product, error) {
	var out []*shop.Product
	for _, p := range f.products {
		if p.WebsiteID == websiteID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeShop) OpGetProduct(_ context.Context, websiteID, id int64) (*shop.Product, string, error) {
	p, err := f.own(websiteID, id)
	if err != nil {
		return nil, "", err
	}
	return p, f.inputs[id].Terms, nil
}

func (f *fakeShop) OpProductInput(_ context.Context, websiteID, id int64) (shop.ProductInput, error) {
	if _, err := f.own(websiteID, id); err != nil {
		return shop.ProductInput{}, err
	}
	return f.inputs[id], nil
}

func (f *fakeShop) OpSaveProduct(_ context.Context, websiteID int64, in shop.ProductInput) (int64, error) {
	price, err := money.ParseAmount(in.Price)
	if err != nil {
		return 0, errors.New("not saved — price: Price not readable.")
	}
	if in.ID == 0 {
		f.nextID++
		in.ID = f.nextID
	} else if _, err := f.own(websiteID, in.ID); err != nil {
		return 0, err
	}
	f.saved = append(f.saved, in)
	f.inputs[in.ID] = in
	f.products[in.ID] = &shop.Product{ID: in.ID, WebsiteID: websiteID, Title: in.Title,
		Status: in.Status, PriceGross: price, TaxRate: money.TaxRate(in.TaxBP)}
	return in.ID, nil
}

func (f *fakeShop) OpDeleteProduct(_ context.Context, websiteID, id int64) error {
	if _, err := f.own(websiteID, id); err != nil {
		return err
	}
	delete(f.products, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeShop) OpListOrders(_ context.Context, websiteID int64, status string, _ int) ([]*shop.Order, error) {
	if f.order == nil || f.order.WebsiteID != websiteID || (status != "" && f.order.Status != status) {
		return nil, nil
	}
	return []*shop.Order{f.order}, nil
}

func (f *fakeShop) OpOrder(_ context.Context, websiteID int64, number string) (*shop.Order, []outbox.Mail, bool, error) {
	if f.order == nil || f.order.WebsiteID != websiteID || f.order.Number != number {
		return nil, nil, false, errors.New("there is no such order on this website")
	}
	return f.order, nil, false, nil
}

func (f *fakeShop) OpSetOrderStatus(ctx context.Context, websiteID int64, number, status string) (int, string, error) {
	o, _, _, err := f.OpOrder(ctx, websiteID, number)
	if err != nil {
		return 0, "", err
	}
	queued := 0
	if status == shop.OrderShipped && o.Status != shop.OrderShipped {
		queued = 1
	}
	o.Status = status
	f.statuses = append(f.statuses, status)
	return queued, "", nil
}

func (f *fakeShop) OpRecheckPayment(context.Context, int64, string) (string, error) {
	return "", errors.New("This order was not paid online.")
}

func (f *fakeShop) OpRetryOrderMail(context.Context, int64, string, int64) error { return nil }

func (f *fakeShop) OpShopSettings(_ context.Context, websiteID int64) (shop.SettingsInput, error) {
	return f.settings[websiteID], nil
}

func (f *fakeShop) OpSaveShopSettings(_ context.Context, websiteID int64, in shop.SettingsInput) error {
	f.settings[websiteID] = in
	return nil
}

func (f *fakeShop) OpShopOverview(context.Context, int64) (shop.Overview, money.Currency, error) {
	return shop.OverviewOf(nil, nil, time.Now()), money.CurrencyFor("CHF"), nil
}

type shopSetup struct {
	ts                        *httptest.Server
	fake                      *fakeShop
	media                     *media.Store
	siteA, siteB              int64
	write, read, admin, onlyB string
}

func setUpShop(t *testing.T) shopSetup {
	t.Helper()
	ctx := context.Background()
	database := newTestDB(t)
	domains := domain.NewStore(database)
	a, err := domains.CreateWebsite(ctx, "Schreinerei", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := domains.CreateWebsite(ctx, "Fremd", "")
	if err != nil {
		t.Fatal(err)
	}
	tokens := NewStore(database)
	issue := func(name string, website int64, level Level) string {
		secret, _, err := tokens.IssueLevel(ctx, name, website, level, 0)
		if err != nil {
			t.Fatalf("IssueLevel: %v", err)
		}
		return secret
	}
	fake := newFakeShop()
	mediaStore := media.NewStore(database)
	srv := NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: mediaStore, Ops: fake,
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return shopSetup{
		ts: ts, fake: fake, media: mediaStore, siteA: a.ID, siteB: b.ID,
		write: issue("schreibend", 0, LevelContent), read: issue("lesend", 0, LevelRead),
		admin: issue("admin", 0, LevelAdmin), onlyB: issue("nur B", b.ID, LevelContent),
	}
}

func TestCreateProductCarriesWhatTheFormWould(t *testing.T) {
	s := setUpShop(t)
	out, failed := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": 49.5, "tax_rate": "2.6",
		"stock": "untracked", "categories": []string{"Möbel", " Eiche "},
	})
	if failed {
		t.Fatalf("create_product: %v", out["text"])
	}
	if len(s.fake.saved) != 1 {
		t.Fatalf("%d saves", len(s.fake.saved))
	}
	in := s.fake.saved[0]
	if in.Price != "49.5" || in.TaxBP != 260 || in.StockText != "" || in.Terms != "Möbel, Eiche" {
		t.Errorf("input %+v", in)
	}
	// A draft unless somebody asked otherwise.
	if in.Status != shop.StatusDraft || out["status"] != shop.StatusDraft {
		t.Errorf("status %q / %v", in.Status, out["status"])
	}
	if out["price"] != "49.50" || out["price_formatted"] != money.CurrencyFor("CHF").Format(4950) {
		t.Errorf("price %v / %v", out["price"], out["price_formatted"])
	}
	if len(s.fake.logged) != 1 || s.fake.logged[0].Action != activity.ActionProductCreate {
		t.Errorf("activity %+v", s.fake.logged)
	}

	// A rate the admin does not offer is refused before anything is saved.
	if _, failed := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Tisch", "price": "10", "tax_rate": "7.7",
	}); !failed {
		t.Error("an old Swiss rate was accepted")
	}
}

func TestUpdateProductChangesOnlyWhatIsGiven(t *testing.T) {
	s := setUpShop(t)
	out, failed := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": "49.00", "sku": "HO-1", "stock": 4,
	})
	if failed {
		t.Fatalf("create_product: %v", out["text"])
	}
	id := int64(out["id"].(float64))

	if out, failed := callTool(t, s.ts, s.write, "update_product", map[string]any{
		"website": s.siteA, "id": id, "price": "55",
	}); failed {
		t.Fatalf("update_product: %v", out["text"])
	}
	in := s.fake.inputs[id]
	if in.Price != "55" || in.SKU != "HO-1" || in.StockText != "4" || in.Title != "Hocker" {
		t.Errorf("after the update %+v", in)
	}
}

func TestDeleteProductWantsConfirmation(t *testing.T) {
	s := setUpShop(t)
	out, _ := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": "49",
	})
	id := int64(out["id"].(float64))

	if _, failed := callTool(t, s.ts, s.write, "delete_product", map[string]any{
		"website": s.siteA, "id": id,
	}); !failed || len(s.fake.deleted) != 0 {
		t.Fatal("deleted without confirm")
	}
	if out, failed := callTool(t, s.ts, s.write, "delete_product", map[string]any{
		"website": s.siteA, "id": id, "confirm": true,
	}); failed {
		t.Fatalf("delete_product: %v", out["text"])
	}
	if len(s.fake.deleted) != 1 {
		t.Error("nothing was deleted")
	}
}

func TestShopToolsStayWithTheirWebsite(t *testing.T) {
	s := setUpShop(t)
	out, _ := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": "49",
	})
	id := int64(out["id"].(float64))

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"list_products", map[string]any{"website": s.siteA}},
		{"get_product", map[string]any{"website": s.siteA, "id": id}},
		{"update_product", map[string]any{"website": s.siteA, "id": id, "price": "1"}},
		{"delete_product", map[string]any{"website": s.siteA, "id": id, "confirm": true}},
		{"list_orders", map[string]any{"website": s.siteA}},
		{"shop_overview", map[string]any{"website": s.siteA}},
	} {
		if _, failed := callTool(t, s.ts, s.onlyB, tc.tool, tc.args); !failed {
			t.Errorf("%s: a key for website B reached website A", tc.tool)
		}
	}
	// Naming its own website and the other one's product is no way round.
	if _, failed := callTool(t, s.ts, s.onlyB, "update_product", map[string]any{
		"website": s.siteB, "id": id, "price": "1",
	}); !failed {
		t.Error("website A's product was changed through website B")
	}
	if s.fake.inputs[id].Price != "49" {
		t.Errorf("price is now %q", s.fake.inputs[id].Price)
	}
}

func TestAProductImageMustBeTheWebsitesOwn(t *testing.T) {
	s := setUpShop(t)
	foreign, err := s.media.Create(context.Background(), s.siteB, "b.jpg", "b.jpg", "image/jpeg", 10, "hash-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, failed := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": "49", "image": foreign.ID,
	}); !failed {
		t.Error("another website's image was accepted")
	}
	own, err := s.media.Create(context.Background(), s.siteA, "a.jpg", "a.jpg", "image/jpeg", 10, "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	if out, failed := callTool(t, s.ts, s.write, "create_product", map[string]any{
		"website": s.siteA, "title": "Hocker", "price": "49", "image": own.ID,
	}); failed {
		t.Errorf("the website's own image was refused: %v", out["text"])
	}
}

func TestAReadOnlyKeyCannotChangeTheShop(t *testing.T) {
	s := setUpShop(t)
	s.fake.order = &shop.Order{ID: 1, WebsiteID: s.siteA, Number: "2026-0001",
		Status: shop.OrderPaid, Currency: "CHF"}

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"create_product", map[string]any{"website": s.siteA, "title": "X", "price": "1"}},
		{"set_order_status", map[string]any{"website": s.siteA, "number": "2026-0001", "status": "shipped"}},
		{"resend_order_mail", map[string]any{"website": s.siteA, "number": "2026-0001", "mail": 1}},
	} {
		if _, failed := callTool(t, s.ts, s.read, tc.tool, tc.args); !failed {
			t.Errorf("%s: a read-only key wrote", tc.tool)
		}
	}
	if len(s.fake.saved) != 0 || len(s.fake.statuses) != 0 {
		t.Error("something was changed")
	}
	// Reading is what it is for.
	if out, failed := callTool(t, s.ts, s.read, "get_order", map[string]any{
		"website": s.siteA, "number": "2026-0001",
	}); failed {
		t.Errorf("get_order: %v", out["text"])
	}
}

func TestSetOrderStatusReportsTheDispatchNotice(t *testing.T) {
	s := setUpShop(t)
	s.fake.order = &shop.Order{ID: 1, WebsiteID: s.siteA, Number: "2026-0001",
		Status: shop.OrderPaid, Currency: "CHF"}

	out, failed := callTool(t, s.ts, s.write, "set_order_status", map[string]any{
		"website": s.siteA, "number": "2026-0001", "status": "shipped",
	})
	if failed {
		t.Fatalf("set_order_status: %v", out["text"])
	}
	if out["status"] != "shipped" || out["previous_status"] != "paid" {
		t.Errorf("answer %v", out)
	}
	if note, _ := out["note"].(string); !strings.Contains(note, "queued") {
		t.Errorf("note = %q", note)
	}
	if len(s.fake.logged) != 1 || s.fake.logged[0].Action != activity.ActionOrderStatus {
		t.Errorf("activity %+v", s.fake.logged)
	}
}

// The settings sit behind requireAdmin on the web side; here they are for an
// admin key only.
func TestShopSettingsAreForAnAdminKey(t *testing.T) {
	s := setUpShop(t)
	s.fake.settings[s.siteA] = shop.SettingsInput{Currency: "CHF", ShippingGross: "0.00",
		ShippingTaxBP: 810, PriceDisplay: shop.DisplayPrivate}

	res := call(t, s.ts, s.write, "tools/list", nil)
	for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
		if name := raw.(map[string]any)["name"]; name == "get_shop_settings" || name == "update_shop_settings" {
			t.Errorf("a content key is offered %v", name)
		}
	}
	if _, failed := callTool(t, s.ts, s.write, "update_shop_settings", map[string]any{
		"website": s.siteA, "shop_base": "laden",
	}); !failed {
		t.Error("a content key changed the shop settings")
	}

	out, failed := callTool(t, s.ts, s.admin, "update_shop_settings", map[string]any{
		"website": s.siteA, "shop_base": "laden", "shipping": 9.5, "shipping_tax_rate": "8.1",
	})
	if failed {
		t.Fatalf("update_shop_settings: %v", out["text"])
	}
	got := s.fake.settings[s.siteA]
	if got.ShopBase != "laden" || got.ShippingGross != "9.5" || got.Currency != "CHF" ||
		got.PriceDisplay != shop.DisplayPrivate {
		t.Errorf("stored %+v", got)
	}
	if _, failed := callTool(t, s.ts, s.admin, "update_shop_settings", map[string]any{
		"website": s.siteA, "price_display": "wholesale",
	}); !failed {
		t.Error("an unknown price display was accepted")
	}
}

func TestShopToolsWithoutTheHandlerSaySo(t *testing.T) {
	ts, _, wsID, key, _ := setUp(t)
	out, failed := callTool(t, ts, key, "list_products", map[string]any{"website": wsID})
	if !failed || !strings.Contains(out["text"].(string), "not available") {
		t.Errorf("answer %v", out)
	}
}
