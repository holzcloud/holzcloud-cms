package shop

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/money"
)

// cartCookie carries the basket's token.
const cartCookie = "hc_warenkorb"

// CartLifetime is how long an untouched basket survives the sweep.
//
// Thirty days: long enough that someone can come back after a fortnight's
// holiday and find their basket, short enough that a shop does not accumulate
// years of one-item baskets from crawlers.
const CartLifetime = 30 * 24 * time.Hour

// maxQuantity is the largest quantity one line may hold.
//
// Not a business rule but a guard: a form field is attacker-controlled, and
// 2000000000 in a quantity is an integer overflow waiting to be found in a
// total rather than a genuine order.
const maxQuantity = 99

// ErrNotOrderable is returned when a product cannot go into a basket.
var ErrNotOrderable = errors.New("dieser Artikel kann zurzeit nicht bestellt werden")

// Cart is a basket with its lines resolved to products.
type Cart struct {
	ID    int64
	Token string
	Items []CartItem
}

// CartItem is one line, with the product as it stands right now.
type CartItem struct {
	Product  *Product
	Quantity int
	// Line is the arithmetic for the audience the cart was totalled for.
	Line money.Line
}

// Empty reports whether there is nothing to buy.
func (c *Cart) Empty() bool { return c == nil || len(c.Items) == 0 }

// Count is the number of articles, summed over the lines — what a basket badge
// shows. Counting lines instead would say "1" for ten of the same stool.
func (c *Cart) Count() int {
	if c == nil {
		return 0
	}
	n := 0
	for _, it := range c.Items {
		n += it.Quantity
	}
	return n
}

// Totals is what a basket or an order adds up to.
type Totals struct {
	ItemsNet, ItemsTax, ItemsGross          money.Amount
	ShippingNet, ShippingTax, ShippingGross money.Amount
	TotalNet, TotalTax, TotalGross          money.Amount
	// Breakdown is the per-rate summary an invoice must show.
	Breakdown []money.TaxBreakdown
}

// Total computes what the basket costs for one audience.
//
// The audience decides the arithmetic, not just the wording: a consumer's line
// is a multiple of the advertised gross price, a business customer's a multiple
// of the quoted net price. Mixing the two produces a total that is a rappen off
// whichever figure the customer added up themselves.
func (c *Cart) Total(set Settings, audience Audience) Totals {
	var t Totals
	lines := make([]money.Line, 0, len(c.Items))

	for i := range c.Items {
		it := &c.Items[i]
		rate := set.EffectiveRate(it.Product.TaxRate)
		if audience == Business {
			it.Line = money.LineForBusiness(it.Product.PriceGross, it.Quantity, rate)
		} else {
			it.Line = money.LineForPrivate(it.Product.PriceGross, it.Quantity, rate)
		}
		lines = append(lines, it.Line)
		t.ItemsNet += it.Line.Net
		t.ItemsTax += it.Line.Tax
		t.ItemsGross += it.Line.Gross
	}

	if !c.Empty() {
		shipping := set.ShippingFor(t.ItemsGross)
		if shipping > 0 {
			rate := set.EffectiveRate(set.ShippingTaxRate)
			// Shipping is quoted gross in both cases: "Versand CHF 12.00" is
			// what the offer says, and a business customer is not quoted a net
			// carriage charge anywhere on the page.
			line := money.LineForPrivate(shipping, 1, rate)
			t.ShippingNet, t.ShippingTax, t.ShippingGross = line.Net, line.Tax, line.Gross
			lines = append(lines, line)
		}
	}

	t.TotalNet = t.ItemsNet + t.ShippingNet
	t.TotalTax = t.ItemsTax + t.ShippingTax
	t.TotalGross = t.ItemsGross + t.ShippingGross
	if !set.VATExempt {
		t.Breakdown = money.Summarize(lines)
	}
	return t
}

// CartStore reads and writes baskets.
type CartStore struct {
	DB       *db.DB
	products *Store
	now      func() time.Time
}

// NewCartStore creates a basket store.
func NewCartStore(products *Store) *CartStore {
	return &CartStore{DB: products.DB, products: products}
}

func (s *CartStore) clock() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

// newToken returns a fresh basket token.
func newToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate cart token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// TokenFrom reads the basket token out of a request, or "" when there is none.
func TokenFrom(r *http.Request) string {
	c, err := r.Cookie(cartCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// SetCartCookie remembers the basket.
func SetCartCookie(w http.ResponseWriter, secure bool, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cartCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(CartLifetime.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCartCookie drops the basket, after an order has been placed.
func ClearCartCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cartCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Ensure returns the basket for a token, creating one when there is none.
//
// It returns the token to store, which differs from the one passed in whenever
// a basket had to be created — including when a stale cookie names a basket
// that has since been swept.
func (s *CartStore) Ensure(ctx context.Context, websiteID int64, token string) (*Cart, string, error) {
	if token != "" {
		cart, err := s.byToken(ctx, websiteID, token)
		if err != nil {
			return nil, "", err
		}
		if cart != nil {
			return cart, token, nil
		}
	}

	fresh, err := newToken()
	if err != nil {
		return nil, "", err
	}
	now := s.clock().Format(timeLayout)
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO carts (website_id, token, created_at, updated_at) VALUES ($1,$2,$3,$4)`,
		websiteID, fresh, now, now)
	if err != nil {
		return nil, "", fmt.Errorf("create cart: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Cart{ID: id, Token: fresh}, fresh, nil
}

// Get returns the basket for a token, or nil.
func (s *CartStore) Get(ctx context.Context, websiteID int64, token string) (*Cart, error) {
	if token == "" {
		return nil, nil
	}
	return s.byToken(ctx, websiteID, token)
}

func (s *CartStore) byToken(ctx context.Context, websiteID int64, token string) (*Cart, error) {
	var cart Cart
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, token FROM carts WHERE website_id = $1 AND token = $2`,
		websiteID, token).Scan(&cart.ID, &cart.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cart: %w", err)
	}

	items, err := s.items(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items
	return &cart, nil
}

// items reads the lines, dropping any whose product has been withdrawn.
//
// Withdrawn and sold out are treated differently on purpose. A product that is
// no longer published has no price and no page, so there is nothing to show and
// the line goes. A product that is merely out of stock stays visible and is
// marked unavailable, because silently shortening someone's basket and then
// taking their money for the remainder is worse than telling them.
//
// The status condition is in the query rather than in a loop afterwards, for
// the same reason the catalogue's is.
func (s *CartStore) items(ctx context.Context, cartID int64) ([]CartItem, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+prefixed("p")+`, ci.quantity
		 FROM cart_items ci
		 JOIN products p ON p.id = ci.product_id
		 WHERE ci.cart_id = $1 AND p.status = 'published'
		 ORDER BY ci.added_at, p.id`, cartID)
	if err != nil {
		return nil, fmt.Errorf("read cart items: %w", err)
	}
	defer rows.Close()

	var out []CartItem
	for rows.Next() {
		var quantity int
		p, err := scanProduct(rows, &quantity)
		if err != nil {
			return nil, err
		}
		// Sold out is not "remove silently": the basket shows it as
		// unavailable, and checkout refuses. Dropping it here would let someone
		// pay for a shorter list than they assembled without being told.
		out = append(out, CartItem{Product: p, Quantity: quantity})
	}
	return out, rows.Err()
}

// Add puts a product into the basket, or raises the quantity of a line that is
// already there.
func (s *CartStore) Add(ctx context.Context, cartID, productID int64, quantity int) error {
	if quantity < 1 {
		quantity = 1
	}
	if quantity > maxQuantity {
		quantity = maxQuantity
	}

	p, err := s.products.Get(ctx, productID)
	if err != nil {
		return err
	}
	if p == nil || !p.Orderable() {
		return ErrNotOrderable
	}

	now := s.clock().Format(timeLayout)
	// The cap is applied in SQL as well, so adding ten twice cannot walk past
	// it through two separate requests.
	if _, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO cart_items (cart_id, product_id, quantity, added_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (cart_id, product_id)
		 DO UPDATE SET quantity = min(cart_items.quantity + $3, $5)`,
		cartID, productID, quantity, now, maxQuantity); err != nil {
		return fmt.Errorf("add to cart: %w", err)
	}
	return s.touch(ctx, cartID)
}

// SetQuantity changes a line, removing it at zero.
func (s *CartStore) SetQuantity(ctx context.Context, cartID, productID int64, quantity int) error {
	if quantity < 1 {
		return s.Remove(ctx, cartID, productID)
	}
	if quantity > maxQuantity {
		quantity = maxQuantity
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE cart_items SET quantity = $1 WHERE cart_id = $2 AND product_id = $3`,
		quantity, cartID, productID); err != nil {
		return fmt.Errorf("set quantity: %w", err)
	}
	return s.touch(ctx, cartID)
}

// Remove drops a line.
func (s *CartStore) Remove(ctx context.Context, cartID, productID int64) error {
	if _, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND product_id = $2`,
		cartID, productID); err != nil {
		return fmt.Errorf("remove from cart: %w", err)
	}
	return s.touch(ctx, cartID)
}

// Clear empties the basket, after an order.
func (s *CartStore) Clear(ctx context.Context, cartID int64) error {
	_, err := s.DB.Write.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID)
	return err
}

// touch keeps the basket out of the sweep.
func (s *CartStore) touch(ctx context.Context, cartID int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE carts SET updated_at = $1 WHERE id = $2`,
		s.clock().Format(timeLayout), cartID)
	return err
}

// Sweep deletes baskets nobody has touched for CartLifetime.
func (s *CartStore) Sweep(ctx context.Context) (int64, error) {
	cutoff := s.clock().Add(-CartLifetime).Format(timeLayout)
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM carts WHERE updated_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("sweep carts: %w", err)
	}
	return res.RowsAffected()
}
