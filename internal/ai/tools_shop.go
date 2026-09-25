package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/outbox"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// Products, orders, shop settings, the shop overview.
//
// Everything here goes through the admin handler, never the shop stores
// directly: the product form's validation, the dispatch notice that goes out
// when an order is marked as shipped, the payment check against the provider —
// each of those is one function the screen calls too. An assistant that could
// save a product the form would refuse, or ship an order without telling the
// customer, would be the way around the shop's own rules.
//
// Money travels as text in both directions, the way the admin's form shows it:
// "49.50", a dot, no grouping, the gross price a customer pays. What comes in
// is read by money.ParseAmount, like the form, so "1'234.50" and "49,50" are
// accepted too. Beside every amount going out stands the same amount written
// the way the shop prints it ("CHF 1’234.50", "1.234,50 €").

// shopOps are the admin handler's shop methods.
type shopOps interface {
	OpListProducts(ctx context.Context, websiteID int64) ([]*shop.Product, error)
	OpGetProduct(ctx context.Context, websiteID, productID int64) (*shop.Product, string, error)
	OpProductInput(ctx context.Context, websiteID, productID int64) (shop.ProductInput, error)
	OpSaveProduct(ctx context.Context, websiteID int64, in shop.ProductInput) (int64, error)
	OpDeleteProduct(ctx context.Context, websiteID, productID int64) error

	OpListOrders(ctx context.Context, websiteID int64, status string, limit int) ([]*shop.Order, error)
	OpOrder(ctx context.Context, websiteID int64, number string) (*shop.Order, []outbox.Mail, bool, error)
	OpSetOrderStatus(ctx context.Context, websiteID int64, number, status string) (int, string, error)
	OpRecheckPayment(ctx context.Context, websiteID int64, number string) (string, error)
	OpRetryOrderMail(ctx context.Context, websiteID int64, number string, mailID int64) error

	OpShopSettings(ctx context.Context, websiteID int64) (shop.SettingsInput, error)
	OpSaveShopSettings(ctx context.Context, websiteID int64, in shop.SettingsInput) error
	OpShopOverview(ctx context.Context, websiteID int64) (shop.Overview, money.Currency, error)
}

var errNoShopOps = errors.New("the shop functions are not available on this installation")

// HasShopOps reports whether ops offers every method the shop tools need. The
// admin package's tests call it with the real handler: a signature that drifts
// on either side would otherwise turn every shop tool into "not available"
// without anything failing to compile.
func HasShopOps(ops any) bool {
	_, ok := ops.(shopOps)
	return ok
}

func shopOpsOf(d Deps) (shopOps, error) {
	ops, ok := d.Ops.(shopOps)
	if !ok {
		return nil, errNoShopOps
	}
	return ops, nil
}

func shopTools(d Deps) []Tool {
	return []Tool{
		listProducts(d), getProduct(d), createProduct(d), updateProduct(d), deleteProduct(d),
		listOrders(d), getOrder(d), setOrderStatus(d), recheckPayment(d), resendOrderMail(d),
		getShopSettings(d), updateShopSettings(d), shopOverview(d),
	}
}

// --- vocabulary -------------------------------------------------------------

// taxRates are the rates an assistant may name, as the percentages the admin
// shows. The admin offers exactly the Swiss rates; so does this.
func taxRates() []string {
	out := make([]string, 0, len(money.SwissRates))
	for _, r := range money.SwissRates {
		out = append(out, wireRate(r.Rate))
	}
	return out
}

// wireRate is a rate as a plain percentage: "8.1", "0".
func wireRate(r money.TaxRate) string {
	return strings.TrimSuffix(r.String(), " %")
}

func rateFromWire(s string) (money.TaxRate, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	s = strings.ReplaceAll(s, ",", ".")
	for _, r := range money.SwissRates {
		if wireRate(r.Rate) == s {
			return r.Rate, nil
		}
	}
	return 0, errors.New("the tax rate must be one of " + strings.Join(taxRates(), ", ") +
		" (percent; the Swiss rates)")
}

// text is an argument that may arrive as a string or as a number. A price is
// text to the admin, but an assistant writes 49.5 as often as "49.50", and
// refusing the one it happened to choose helps nobody. A number is taken by
// its literal digits, never through a float.
type text struct {
	Value string
	Set   bool
}

func (t *text) UnmarshalJSON(raw []byte) error {
	t.Set = true
	if string(raw) == "null" {
		t.Value = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		t.Value = s
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return errors.New("expected a string or a number")
	}
	t.Value = n.String()
	return nil
}

// stockFromWire is the stock as the form's text field takes it: a number, or
// empty for "not tracked".
func stockFromWire(t text) string {
	v := strings.TrimSpace(t.Value)
	if strings.EqualFold(v, "untracked") {
		return ""
	}
	return v
}

// productOut is a product as an assistant reads it.
func productOut(p *shop.Product, cur money.Currency, full bool) map[string]any {
	out := map[string]any{
		"id": p.ID, "website": p.WebsiteID, "title": p.Title, "slug": p.Slug,
		"status": p.Status, "sku": p.SKU,
		"price": money.Input(p.PriceGross), "price_formatted": cur.Format(p.PriceGross),
		"tax_rate":  wireRate(p.TaxRate),
		"orderable": p.Orderable(),
	}
	// Two different facts, and the one most easily confused: nil is "not
	// tracked", made to order, always orderable; zero is sold out.
	if p.Stock == nil {
		out["stock"] = "untracked"
	} else {
		out["stock"] = *p.Stock
	}
	if !full {
		return out
	}
	out["subtitle"] = p.Subtitle
	out["markdown"] = p.DescriptionMarkdown
	out["weight_grams"] = p.WeightGrams
	out["delivery_note"] = p.DeliveryNote
	if p.FeaturedMediaID != nil {
		out["image"] = *p.FeaturedMediaID
	}
	out["updated"] = p.UpdatedAt.UTC().Format(timeLayout)
	return out
}

// currencyOf is how a website writes its prices.
func currencyOf(c Call, d Deps, websiteID int64) money.Currency {
	if d.Domains == nil {
		return money.CurrencyFor("")
	}
	ws, err := d.Domains.GetWebsite(c.Ctx, websiteID)
	if err != nil || ws == nil {
		return money.CurrencyFor("")
	}
	return money.CurrencyFor(ws.Currency)
}

// checkImage holds a picture id against the website. The admin form only ever
// offers the website's own images; an id from outside has to be checked, or a
// product could show another website's picture.
func checkImage(c Call, d Deps, websiteID, id int64) error {
	if id == 0 {
		return nil
	}
	if d.Media == nil {
		return errors.New("images are not available on this installation")
	}
	m, err := d.Media.GetByID(c.Ctx, id)
	if err != nil || m == nil || m.WebsiteID != websiteID {
		return errors.New("there is no such image on this website; list_media names them")
	}
	if !strings.HasPrefix(m.MimeType, "image/") {
		return errors.New("that file is not an image")
	}
	return nil
}

// --- products ---------------------------------------------------------------

func listProducts(d Deps) Tool {
	return Tool{
		Name: "list_products",
		Description: "Lists the products of a website's shop, drafts included, with id, title, " +
			"price (gross, what a customer pays), tax rate, stock and status. Without the " +
			"description — get_product fetches that.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"status": {Type: "string", Description: "draft, published or all (default)",
					Enum: []string{"draft", "published", "all"}},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Status  string `json:"status"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpListProducts(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			cur := currencyOf(c, d, a.Website)
			out := make([]map[string]any, 0, len(list))
			for _, p := range list {
				if (a.Status == "draft" || a.Status == "published") && p.Status != a.Status {
					continue
				}
				out = append(out, productOut(p, cur, false))
			}
			return map[string]any{"products": out, "currency": cur.Code}, nil
		},
	}
}

func getProduct(d Deps) Tool {
	return Tool{
		Name: "get_product",
		Description: "Fetches one product with everything the product form shows: description " +
			"in markdown, price, tax rate, stock, SKU, weight, delivery note, image and categories.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"id":      {Type: "integer", Description: "id of the product"},
			},
			Required: []string{"website", "id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				ID      int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			p, terms, err := ops.OpGetProduct(c.Ctx, a.Website, a.ID)
			if err != nil {
				return nil, productError(err)
			}
			out := productOut(p, currencyOf(c, d, a.Website), true)
			out["categories"] = splitTerms(terms)
			return out, nil
		},
	}
}

// productProperties are the fields create_product and update_product share.
func productProperties() map[string]Property {
	return map[string]Property{
		"website":  {Type: "integer", Description: "id of the website"},
		"title":    {Type: "string", Description: "name of the product"},
		"slug":     {Type: "string", Description: "address of the product page; otherwise from the title"},
		"subtitle": {Type: "string", Description: "one line under the title, e.g. the material"},
		"markdown": {Type: "string", Description: "description in markdown"},
		"sku":      {Type: "string", Description: "article number"},
		"price": {Type: "string", Description: "gross price, what a customer pays, in the " +
			"website's currency: \"49.50\"; 1'234.50 and 49,50 are read as well"},
		"tax_rate": {Type: "string", Description: "VAT rate in percent: 8.1 standard, 2.6 " +
			"reduced (food, books, medicines), 3.8 lodging, 0 excluded or exempt",
			Enum: taxRates()},
		"stock": {Type: "string", Description: "pieces in stock, a number from 0; \"untracked\" " +
			"(or empty) for a product made to order, which is always orderable. 0 means sold out."},
		"weight_grams":  {Type: "integer", Description: "weight in grams"},
		"delivery_note": {Type: "string", Description: "e.g. \"Delivery time 3–4 weeks\""},
		"image": {Type: "integer", Description: "id of an image of this website (list_media) " +
			"as the product picture; 0 removes it"},
		"categories": {Type: "array", Description: "category names; missing ones are created. " +
			"A list that is given replaces the existing categories.",
			Items: &Property{Type: "string"}},
		"status": {Type: "string", Description: "draft or published. Publish only when you " +
			"were expressly asked to.", Enum: []string{shop.StatusDraft, shop.StatusPublished}},
	}
}

// productArgs are the arguments of create_product and update_product. A
// pointer or an unset text is a field that was not given.
type productArgs struct {
	Website      int64     `json:"website"`
	ID           int64     `json:"id"`
	Title        *string   `json:"title"`
	Slug         *string   `json:"slug"`
	Subtitle     *string   `json:"subtitle"`
	Markdown     *string   `json:"markdown"`
	SKU          *string   `json:"sku"`
	Price        text      `json:"price"`
	TaxRate      text      `json:"tax_rate"`
	Stock        text      `json:"stock"`
	WeightGrams  *int      `json:"weight_grams"`
	DeliveryNote *string   `json:"delivery_note"`
	Image        *int64    `json:"image"`
	Categories   *[]string `json:"categories"`
	Status       *string   `json:"status"`
}

// apply lays the given fields over a product as the form carries it.
func (a productArgs) apply(c Call, d Deps, in *shop.ProductInput) error {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&in.Title, a.Title)
	set(&in.Slug, a.Slug)
	set(&in.Subtitle, a.Subtitle)
	set(&in.Markdown, a.Markdown)
	set(&in.SKU, a.SKU)
	set(&in.DeliveryNote, a.DeliveryNote)
	set(&in.Status, a.Status)
	if a.Price.Set {
		in.Price = a.Price.Value
	}
	if a.TaxRate.Set {
		rate, err := rateFromWire(a.TaxRate.Value)
		if err != nil {
			return err
		}
		in.TaxBP = int(rate)
	}
	if a.Stock.Set {
		in.StockText = stockFromWire(a.Stock)
	}
	if a.WeightGrams != nil {
		if *a.WeightGrams < 0 {
			return errors.New("the weight cannot be negative")
		}
		in.WeightGrams = *a.WeightGrams
	}
	if a.Image != nil {
		if err := checkImage(c, d, a.Website, *a.Image); err != nil {
			return err
		}
		in.FeaturedID = *a.Image
	}
	if a.Categories != nil {
		names := make([]string, 0, len(*a.Categories))
		for _, n := range *a.Categories {
			// The form separates categories with commas, so a comma inside a
			// name would split it in two without anybody noticing.
			if strings.Contains(n, ",") {
				return errors.New("a category name cannot contain a comma: " + n)
			}
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
		in.Terms = strings.Join(names, ", ")
	}
	if in.Status != shop.StatusDraft && in.Status != shop.StatusPublished {
		return errors.New("the status must be draft or published")
	}
	return nil
}

func createProduct(d Deps) Tool {
	props := productProperties()
	return Tool{
		Name:   "create_product",
		Writes: true,
		Description: "Creates a product in a website's shop, validated like the product form. " +
			"It is a draft unless status published is given — do that only when asked to.",
		InputSchema: Schema{
			Type: "object", Properties: props,
			Required: []string{"website", "title", "price"},
		},
		Run: func(c Call) (any, error) {
			var a productArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			// The form's defaults for a new product.
			in := shop.ProductInput{
				Status: shop.StatusDraft,
				TaxBP:  int(money.RateStandard),
				Price:  "0.00",
			}
			a.ID = 0
			if err := a.apply(c, d, &in); err != nil {
				return nil, err
			}
			id, err := ops.OpSaveProduct(c.Ctx, a.Website, in)
			if err != nil {
				return nil, productError(err)
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionProductCreate, EntityType: "product", EntityID: id,
				Metadata: map[string]any{"title": in.Title},
			})
			c.Log.Info("ai created product", "key", c.Scope.Name, "product", id, "website", a.Website)
			return productAfter(c, d, ops, a.Website, id)
		},
	}
}

func updateProduct(d Deps) Tool {
	props := productProperties()
	props["id"] = Property{Type: "integer", Description: "id of the product"}
	return Tool{
		Name:   "update_product",
		Writes: true,
		Description: "Changes a product. Only the fields given change; the rest stays as it is. " +
			"Validated like the product form. The status changes only when status is given.",
		InputSchema: Schema{
			Type: "object", Properties: props,
			Required: []string{"website", "id"},
		},
		Run: func(c Call) (any, error) {
			var a productArgs
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			in, err := ops.OpProductInput(c.Ctx, a.Website, a.ID)
			if err != nil {
				return nil, productError(err)
			}
			if err := a.apply(c, d, &in); err != nil {
				return nil, err
			}
			if _, err := ops.OpSaveProduct(c.Ctx, a.Website, in); err != nil {
				return nil, productError(err)
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionProductUpdate, EntityType: "product", EntityID: a.ID,
				Metadata: map[string]any{"title": in.Title},
			})
			c.Log.Info("ai updated product", "key", c.Scope.Name, "product", a.ID)
			return productAfter(c, d, ops, a.Website, a.ID)
		},
	}
}

func deleteProduct(d Deps) Tool {
	return Tool{
		Name:   "delete_product",
		Writes: true,
		Description: "Deletes a product for good; it cannot be undone. Orders that contain it " +
			"keep their lines. Requires confirm: true — ask before you set it. Setting the " +
			"status to draft with update_product takes a product offline without losing it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"id":      {Type: "integer", Description: "id of the product"},
				"confirm": {Type: "boolean", Description: "must be true"},
			},
			Required: []string{"website", "id", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("deleting cannot be undone; call again with confirm: true " +
					"once the operator has agreed")
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			p, _, err := ops.OpGetProduct(c.Ctx, a.Website, a.ID)
			if err != nil {
				return nil, productError(err)
			}
			if err := ops.OpDeleteProduct(c.Ctx, a.Website, a.ID); err != nil {
				return nil, productError(err)
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionProductDelete, EntityType: "product", EntityID: a.ID,
				Metadata: map[string]any{"title": p.Title},
			})
			c.Log.Info("ai deleted product", "key", c.Scope.Name, "product", a.ID)
			return map[string]any{"deleted": a.ID, "title": p.Title}, nil
		},
	}
}

// productAfter is the product as it now stands, the answer to every write.
func productAfter(c Call, d Deps, ops shopOps, websiteID, id int64) (any, error) {
	p, terms, err := ops.OpGetProduct(c.Ctx, websiteID, id)
	if err != nil {
		return nil, err
	}
	out := productOut(p, currencyOf(c, d, websiteID), true)
	out["categories"] = splitTerms(terms)
	return out, nil
}

// productError words the store's German sentinel for an English reader.
func productError(err error) error {
	if errors.Is(err, shop.ErrNotFound) {
		return errors.New("there is no such product on this website")
	}
	return err
}

func splitTerms(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if n := strings.TrimSpace(part); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// --- orders -----------------------------------------------------------------

var orderStatusWire = []string{shop.OrderNew, shop.OrderPaid, shop.OrderShipped, shop.OrderCancelled}

// orderOut is an order as an assistant reads it; full adds the lines, the
// customer and the totals.
func orderOut(o *shop.Order, full bool) map[string]any {
	cur := money.CurrencyFor(o.Currency)
	amount := func(a money.Amount) map[string]any {
		return map[string]any{"amount": money.Input(a), "formatted": cur.Format(a)}
	}
	out := map[string]any{
		"number": o.Number, "status": o.Status,
		"payment_method": o.PaymentMethod, "payment_status": o.PaymentStatus,
		"customer_name": o.Customer.Name, "customer_email": o.Customer.Email,
		"total": amount(o.Totals.TotalGross), "currency": o.Currency,
		"created": o.CreatedAt.UTC().Format(timeLayout),
	}
	if !full {
		return out
	}
	items := make([]map[string]any, 0, len(o.Items))
	for _, it := range o.Items {
		e := map[string]any{
			"title": it.Title, "sku": it.SKU, "quantity": it.Quantity,
			"tax_rate": wireRate(it.TaxRate), "unit_price": amount(it.UnitGross),
			"line_total": amount(it.LineGross),
		}
		if it.Subtitle != "" {
			e["subtitle"] = it.Subtitle
		}
		// A product deleted since keeps its line; the id is then gone.
		if it.ProductID != nil {
			e["product"] = *it.ProductID
		}
		items = append(items, e)
	}
	out["items"] = items
	out["audience"] = string(o.Audience)
	out["vat_exempt"] = o.VATExempt
	out["totals"] = map[string]any{
		"items_gross":    amount(o.Totals.ItemsGross),
		"shipping_gross": amount(o.Totals.ShippingGross),
		"net":            amount(o.Totals.TotalNet),
		"tax":            amount(o.Totals.TotalTax),
		"total":          amount(o.Totals.TotalGross),
	}
	breakdown := make([]map[string]any, 0, len(o.Totals.Breakdown))
	for _, b := range o.Totals.Breakdown {
		breakdown = append(breakdown, map[string]any{
			"tax_rate": wireRate(b.Rate), "net": amount(b.Net), "tax": amount(b.Tax),
		})
	}
	out["tax_breakdown"] = breakdown
	cu := o.Customer
	out["customer"] = map[string]any{
		"name": cu.Name, "company": cu.Company, "email": cu.Email, "phone": cu.Phone,
		"street": cu.Street, "postal_code": cu.PostalCode, "city": cu.City,
		"country": cu.Country, "vat_number": cu.VATNumber, "note": cu.Note,
	}
	out["payment_reference"] = o.PaymentReference
	out["updated"] = o.UpdatedAt.UTC().Format(timeLayout)
	return out
}

func mailsOut(mails []outbox.Mail) []map[string]any {
	out := make([]map[string]any, 0, len(mails))
	for _, m := range mails {
		e := map[string]any{
			"id": m.ID, "kind": m.Kind, "recipient": m.Recipient, "subject": m.Subject,
			"status": m.Status, "attempts": m.Attempts,
		}
		if m.LastError != "" {
			e["last_error"] = m.LastError
		}
		if m.SentAt != nil {
			e["sent_at"] = m.SentAt.UTC().Format(timeLayout)
		}
		// Not "sent": a message in the queue is waiting, and saying otherwise
		// is the one thing the order screen must not do either.
		if m.Status == outbox.StatusPending {
			e["note"] = "queued, not sent yet"
		}
		out = append(out, e)
	}
	return out
}

func listOrders(d Deps) Tool {
	return Tool{
		Name: "list_orders",
		Description: "Lists the orders of a website's shop, newest first, with number, status, " +
			"payment, customer and total. get_order fetches the lines.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"status": {Type: "string", Description: "only orders in this status; empty for all",
					Enum: orderStatusWire},
				"limit": {Type: "integer", Description: "at most this many, 50 by default"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Status  string `json:"status"`
				Count   int    `json:"limit"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			list, err := ops.OpListOrders(c.Ctx, a.Website, strings.TrimSpace(a.Status), clampCount(a.Count))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				out = append(out, orderOut(o, false))
			}
			return map[string]any{"orders": out}, nil
		},
	}
}

func getOrder(d Deps) Tool {
	return Tool{
		Name: "get_order",
		Description: "Fetches one order by its number: lines, totals with the tax per rate, " +
			"customer and address, payment, and the mails sent or queued for it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"number":  {Type: "string", Description: "the order number"},
			},
			Required: []string{"website", "number"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Number  string `json:"number"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			o, mails, canRecheck, err := ops.OpOrder(c.Ctx, a.Website, a.Number)
			if err != nil {
				return nil, err
			}
			out := orderOut(o, true)
			out["mails"] = mailsOut(mails)
			out["can_recheck_payment"] = canRecheck
			return out, nil
		},
	}
}

func setOrderStatus(d Deps) Tool {
	return Tool{
		Name:   "set_order_status",
		Writes: true,
		Description: "Moves an order to new, paid, shipped or cancelled, as the order screen does. " +
			"Moving it to shipped queues the dispatch notice to the customer (once; setting " +
			"shipped again sends nothing). Cancelling does not refund or restock anything.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"number":  {Type: "string", Description: "the order number"},
				"status":  {Type: "string", Description: "the new status", Enum: orderStatusWire},
			},
			Required: []string{"website", "number", "status"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Number  string `json:"number"`
				Status  string `json:"status"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			before, _, _, err := ops.OpOrder(c.Ctx, a.Website, a.Number)
			if err != nil {
				return nil, err
			}
			// Copied before the change: the order read above is not promised
			// to be a snapshot.
			previous := before.Status
			queued, mailProblem, err := ops.OpSetOrderStatus(c.Ctx, a.Website, a.Number, a.Status)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionOrderStatus, EntityType: "order", EntityID: before.ID,
				Metadata: map[string]any{"number": before.Number, "from": previous, "to": a.Status},
			})
			c.Log.Info("ai changed order status", "key", c.Scope.Name, "order", before.Number, "status", a.Status)

			out := map[string]any{"number": before.Number, "status": a.Status, "previous_status": previous}
			switch {
			case mailProblem != "":
				out["note"] = mailProblem
			case queued > 0:
				out["note"] = "The dispatch notice to the customer is queued (" +
					strconv.Itoa(queued) + " message); get_order shows when it has gone out."
			}
			return out, nil
		},
	}
}

func recheckPayment(d Deps) Tool {
	return Tool{
		Name:   "recheck_payment",
		Writes: true,
		Description: "Asks the payment provider (Payrexx) once more about an online payment " +
			"that is still open, as the button on the order screen does, and records the " +
			"answer. It cannot make an unpaid order paid: the answer comes from the provider. " +
			"get_order says under can_recheck_payment whether it can help.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"number":  {Type: "string", Description: "the order number"},
			},
			Required: []string{"website", "number"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Number  string `json:"number"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			outcome, err := ops.OpRecheckPayment(c.Ctx, a.Website, a.Number)
			if err != nil {
				return nil, err
			}
			after, _, _, err := ops.OpOrder(c.Ctx, a.Website, a.Number)
			if err != nil {
				return nil, err
			}
			if outcome != shop.PaymentOpen {
				changed(d, c, a.Website, activity.Entry{
					Action: activity.ActionOrderPayment, EntityType: "order", EntityID: after.ID,
					Metadata: map[string]any{"number": after.Number, "payment": outcome},
				})
			}
			notes := map[string]string{
				shop.PaymentPaid:   "The payment has come in.",
				shop.PaymentFailed: "The payment was cancelled or declined.",
				shop.PaymentOpen:   "The provider has no payment recorded yet.",
			}
			return map[string]any{
				"number": after.Number, "payment_status": after.PaymentStatus,
				"status": after.Status, "note": notes[outcome],
			}, nil
		},
	}
}

func resendOrderMail(d Deps) Tool {
	return Tool{
		Name:   "resend_order_mail",
		Writes: true,
		Description: "Puts a mail of an order that did not go out back into the queue, as the " +
			"order screen does — useful after an address or the mail account was fixed. It " +
			"does not send a mail that has already been sent. get_order lists the mails with " +
			"their id.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
				"number":  {Type: "string", Description: "the order number"},
				"mail":    {Type: "integer", Description: "id of the mail, from get_order"},
			},
			Required: []string{"website", "number", "mail"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Number  string `json:"number"`
				Mail    int64  `json:"mail"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			if err := ops.OpRetryOrderMail(c.Ctx, a.Website, a.Number, a.Mail); err != nil {
				return nil, err
			}
			o, _, _, err := ops.OpOrder(c.Ctx, a.Website, a.Number)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionOrderMailRetry, EntityType: "order", EntityID: o.ID,
				Metadata: map[string]any{"number": o.Number, "mail": a.Mail},
			})
			return map[string]any{
				"number": o.Number, "mail": a.Mail,
				"note": "The message is queued for sending again.",
			}, nil
		},
	}
}

// --- settings and overview --------------------------------------------------

func settingsOut(s shop.SettingsInput) map[string]any {
	out := map[string]any{
		"shop_base": s.ShopBase, "currency": s.Currency,
		"shipping": s.ShippingGross, "shipping_tax_rate": wireRate(money.TaxRate(s.ShippingTaxBP)),
		"price_display": s.PriceDisplay, "vat_exempt": s.VATExempt, "vat_number": s.VATNumber,
		"return_policy": s.ReturnPolicy, "order_email": s.OrderEmail,
		"payment_details": s.PaymentDetails,
		// Empty and zero are two offers: no threshold, and always free.
		"free_shipping_from": s.ShippingFreeAt,
	}
	if s.ShopBase == "" {
		out["note"] = "The shop is switched off for this website: shop_base is empty."
	}
	return out
}

func getShopSettings(d Deps) Tool {
	return Tool{
		Name:  "get_shop_settings",
		Admin: true,
		Description: "Reads a website's shop settings: the path of the shop, currency, shipping " +
			"costs and free-shipping threshold, price display, VAT, return policy, the address " +
			"order notices go to and the payment details.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			s, err := ops.OpShopSettings(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			return settingsOut(s), nil
		},
	}
}

func updateShopSettings(d Deps) Tool {
	return Tool{
		Name:   "update_shop_settings",
		Admin:  true,
		Writes: true,
		Description: "Changes a website's shop settings. Only the fields given change. Validated " +
			"like the settings form. An empty shop_base switches the shop off.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website":   {Type: "integer", Description: "id of the website"},
				"shop_base": {Type: "string", Description: "one path segment the shop lives under, e.g. shop; empty switches the shop off"},
				"currency":  {Type: "string", Description: "CHF or EUR", Enum: []string{"CHF", "EUR"}},
				"shipping":  {Type: "string", Description: "gross shipping cost, e.g. \"9.00\""},
				"free_shipping_from": {Type: "string", Description: "order total from which " +
					"shipping is free; empty for no threshold (\"0\" would mean always free)"},
				"shipping_tax_rate": {Type: "string", Description: "VAT rate of the shipping in percent",
					Enum: taxRates()},
				"price_display": {Type: "string", Description: "private: gross prices; business: " +
					"net prices plus VAT; both: the visitor chooses",
					Enum: []string{shop.DisplayPrivate, shop.DisplayBusiness, shop.DisplayBoth}},
				"vat_exempt":      {Type: "boolean", Description: "the business is not liable for VAT"},
				"vat_number":      {Type: "string", Description: "VAT number printed on invoices"},
				"return_policy":   {Type: "string", Description: "return policy copied onto every order"},
				"order_email":     {Type: "string", Description: "where order notices go; empty sends none"},
				"payment_details": {Type: "string", Description: "bank details for payment in advance and invoices"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website        int64   `json:"website"`
				ShopBase       *string `json:"shop_base"`
				Currency       *string `json:"currency"`
				Shipping       text    `json:"shipping"`
				FreeFrom       text    `json:"free_shipping_from"`
				ShippingTax    text    `json:"shipping_tax_rate"`
				PriceDisplay   *string `json:"price_display"`
				VATExempt      *bool   `json:"vat_exempt"`
				VATNumber      *string `json:"vat_number"`
				ReturnPolicy   *string `json:"return_policy"`
				OrderEmail     *string `json:"order_email"`
				PaymentDetails *string `json:"payment_details"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			s, err := ops.OpShopSettings(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			set := func(dst *string, src *string) {
				if src != nil {
					*dst = *src
				}
			}
			set(&s.ShopBase, a.ShopBase)
			set(&s.Currency, a.Currency)
			set(&s.PriceDisplay, a.PriceDisplay)
			set(&s.VATNumber, a.VATNumber)
			set(&s.ReturnPolicy, a.ReturnPolicy)
			set(&s.OrderEmail, a.OrderEmail)
			set(&s.PaymentDetails, a.PaymentDetails)
			if a.VATExempt != nil {
				s.VATExempt = *a.VATExempt
			}
			if a.Shipping.Set {
				s.ShippingGross = a.Shipping.Value
			}
			if a.FreeFrom.Set {
				s.ShippingFreeAt = a.FreeFrom.Value
			}
			if a.ShippingTax.Set {
				rate, err := rateFromWire(a.ShippingTax.Value)
				if err != nil {
					return nil, err
				}
				s.ShippingTaxBP = int(rate)
			}
			if a.PriceDisplay != nil {
				switch *a.PriceDisplay {
				case shop.DisplayPrivate, shop.DisplayBusiness, shop.DisplayBoth:
				default:
					// The form quietly falls back to private; a tool call that
					// named something else should hear about it instead.
					return nil, errors.New("price_display must be private, business or both")
				}
			}
			if err := ops.OpSaveShopSettings(c.Ctx, a.Website, s); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionShopSettingsSave, EntityType: "website", EntityID: a.Website,
			})
			c.Log.Info("ai changed shop settings", "key", c.Scope.Name, "website", a.Website)
			after, err := ops.OpShopSettings(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			return settingsOut(after), nil
		},
	}
}

func shopOverview(d Deps) Tool {
	return Tool{
		Name: "shop_overview",
		Description: "The shop's overview, the same numbers as the admin's overview screen: " +
			"revenue this month (paid and shipped orders) against last month, open orders, " +
			"orders waiting for payment, products nearly sold out, products online, what is to " +
			"be done, and the revenue of the last eight weeks.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": {Type: "integer", Description: "id of the website"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, err := shopOpsOf(d)
			if err != nil {
				return nil, err
			}
			ov, cur, err := ops.OpShopOverview(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			amount := func(x money.Amount) map[string]any {
				return map[string]any{"amount": money.Input(x), "formatted": cur.Format(x)}
			}
			out := map[string]any{
				"revenue_this_month": amount(ov.ThisMonth),
				"revenue_last_month": amount(ov.LastMonth),
				"open_orders":        ov.OpenOrders,
				"awaiting_payment":   ov.AwaitPayment,
				"low_stock":          ov.LowStock,
				"low_stock_below":    shop.LowStockBelow,
				"products_online":    ov.ProductsOnline,
				"product_drafts":     ov.ProductDrafts,
				"has_orders":         ov.HasOrders,
			}
			if ov.HasPrevious {
				out["revenue_change_percent"] = ov.RevenueChange
			}
			tasks := make([]map[string]any, 0, len(ov.Tasks))
			for _, t := range ov.Tasks {
				e := map[string]any{"kind": t.Kind, "overdue": t.Warn}
				switch t.Kind {
				case shop.TaskPayment:
					e["order"] = t.OrderNumber
					e["days_waiting"] = t.DaysWaiting
					e["text"] = "Order " + t.OrderNumber + " has been waiting for its payment for " +
						strconv.Itoa(t.DaysWaiting) + " days"
				case shop.TaskDispatch:
					e["order"] = t.OrderNumber
					e["text"] = "Order " + t.OrderNumber + " is paid and waiting to be sent"
				case shop.TaskStock:
					e["product"] = t.ProductID
					e["stock"] = t.Stock
					e["text"] = t.ProductTitle + ": " + strconv.Itoa(t.Stock) + " left"
				}
				tasks = append(tasks, e)
			}
			out["tasks"] = tasks
			weeks := make([]map[string]any, 0, len(ov.Weeks))
			for _, w := range ov.Weeks {
				e := amount(w.Amount)
				e["week"] = w.Week
				e["starts"] = w.Start.Format("2006-01-02")
				if w.Current {
					e["current"] = true
				}
				weeks = append(weeks, e)
			}
			out["weekly_revenue"] = weeks
			if ov.Best > 0 {
				out["best_week"] = map[string]any{"week": ov.BestWeek, "revenue": amount(ov.Best)}
			}
			return out, nil
		},
	}
}
