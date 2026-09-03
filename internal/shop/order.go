package shop

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

// Order status values.
const (
	OrderNew       = "new"
	OrderPaid      = "paid"
	OrderShipped   = "shipped"
	OrderCancelled = "cancelled"
)

// Payment methods and states.
const (
	PayInvoice = "invoice"
	PayPrepay  = "prepay"
	PayPayrexx = "payrexx"

	PaymentOpen     = "open"
	PaymentPaid     = "paid"
	PaymentFailed   = "failed"
	PaymentRefunded = "refunded"
)

// ErrOutOfStock is returned when the basket can no longer be fulfilled.
//
// It names the article, because "something in your basket ran out" sends the
// customer back to compare a list against a shop.
type ErrOutOfStock struct{ Title string }

func (e ErrOutOfStock) Error() string {
	return e.Title + " ist nicht mehr in der benötigten Menge verfügbar"
}

// Customer is what the buyer entered.
type Customer struct {
	Email      string
	Name       string
	Company    string
	VATNumber  string
	Phone      string
	Street     string
	PostalCode string
	City       string
	Country    string
	Note       string
}

// Order is a placed order.
type Order struct {
	ID        int64
	WebsiteID int64
	Number    string
	Audience  Audience
	Currency  string
	Customer  Customer

	Totals    Totals
	VATExempt bool

	Status           string
	PaymentMethod    string
	PaymentStatus    string
	PaymentReference string
	ReturnPolicy     string

	Items     []OrderItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrderItem is one frozen line.
type OrderItem struct {
	ID        int64
	ProductID *int64
	Title     string
	Subtitle  string
	SKU       string
	Quantity  int
	TaxRate   money.TaxRate
	UnitGross money.Amount
	UnitNet   money.Amount
	LineNet   money.Amount
	LineTax   money.Amount
	LineGross money.Amount
}

// OrderStore reads and writes orders.
type OrderStore struct {
	carts *CartStore
	now   func() time.Time
}

// NewOrderStore creates an order store.
func NewOrderStore(carts *CartStore) *OrderStore { return &OrderStore{carts: carts} }

func (s *OrderStore) clock() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

// Place turns a basket into an order.
//
// Everything happens in one transaction: the order, its frozen lines, the stock
// decrements and the emptying of the basket. Anything less leaves a shop that
// has taken an order it cannot ship, or has reserved goods for an order that
// was never written — and the two are indistinguishable afterwards.
//
// The stock check is a conditional UPDATE rather than a read followed by a
// write, so two people buying the last piece at the same moment cannot both
// succeed. The one who loses is told which article, not merely that something
// went wrong.
func (s *OrderStore) Place(ctx context.Context, websiteID int64, set Settings,
	audience Audience, cart *Cart, customer Customer, method string) (*Order, error) {

	if cart.Empty() {
		return nil, errors.New("der Warenkorb ist leer")
	}

	totals := cart.Total(set, audience)
	now := s.clock()

	tx, err := s.carts.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin order: %w", err)
	}
	defer tx.Rollback()

	// Reserve the goods first. Writing the order and then finding the stock
	// gone would mean rolling back a number that has already been shown.
	for _, it := range cart.Items {
		if it.Product.Stock == nil {
			continue
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE products SET stock = stock - $1
			 WHERE id = $2 AND stock IS NOT NULL AND stock >= $1`,
			it.Quantity, it.Product.ID)
		if err != nil {
			return nil, fmt.Errorf("reserve stock: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, ErrOutOfStock{Title: it.Product.Title}
		}
	}

	number, err := nextOrderNumber(ctx, tx, now)
	if err != nil {
		return nil, err
	}

	exempt := 0
	if set.VATExempt {
		exempt = 1
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO orders (website_id, number, audience, currency,
			email, name, company, vat_number, phone,
			street, postal_code, city, country, note,
			items_net, items_tax, items_gross,
			shipping_net, shipping_tax, shipping_gross,
			total_net, total_tax, total_gross, vat_exempt,
			status, payment_method, payment_status, return_policy,
			created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,
		         $15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)`,
		websiteID, number, string(audience), set.Currency.Code,
		customer.Email, customer.Name, customer.Company, customer.VATNumber, customer.Phone,
		customer.Street, customer.PostalCode, customer.City, customer.Country, customer.Note,
		int64(totals.ItemsNet), int64(totals.ItemsTax), int64(totals.ItemsGross),
		int64(totals.ShippingNet), int64(totals.ShippingTax), int64(totals.ShippingGross),
		int64(totals.TotalNet), int64(totals.TotalTax), int64(totals.TotalGross), exempt,
		OrderNew, method, PaymentOpen, set.ReturnPolicy,
		now.Format(timeLayout), now.Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	orderID, _ := res.LastInsertId()

	order := &Order{
		ID: orderID, WebsiteID: websiteID, Number: number,
		Audience: audience, Currency: set.Currency.Code, Customer: customer,
		Totals: totals, VATExempt: set.VATExempt,
		Status: OrderNew, PaymentMethod: method, PaymentStatus: PaymentOpen,
		ReturnPolicy: set.ReturnPolicy,
		CreatedAt:    now, UpdatedAt: now,
	}

	for i, it := range cart.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, product_id, title, subtitle, sku,
				quantity, tax_bp, unit_gross, unit_net, line_net, line_tax, line_gross, position)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			orderID, it.Product.ID, it.Product.Title, it.Product.Subtitle, it.Product.SKU,
			it.Quantity, int(it.Line.Rate), int64(it.Line.UnitGross), int64(it.Line.UnitNet),
			int64(it.Line.Net), int64(it.Line.Tax), int64(it.Line.Gross), i); err != nil {
			return nil, fmt.Errorf("create order item: %w", err)
		}
		id := it.Product.ID
		order.Items = append(order.Items, OrderItem{
			ProductID: &id, Title: it.Product.Title, Subtitle: it.Product.Subtitle,
			SKU: it.Product.SKU, Quantity: it.Quantity, TaxRate: it.Line.Rate,
			UnitGross: it.Line.UnitGross, UnitNet: it.Line.UnitNet,
			LineNet: it.Line.Net, LineTax: it.Line.Tax, LineGross: it.Line.Gross,
		})
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1`, cart.ID); err != nil {
		return nil, fmt.Errorf("clear cart: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order: %w", err)
	}
	return order, nil
}

// nextOrderNumber produces "2026-0001", counting within the calendar year.
//
// Derived from what is already stored rather than from a counter table: a
// counter that drifts out of step with the orders is a second source of truth,
// and the query is one indexed scan per order placed.
func nextOrderNumber(ctx context.Context, tx *sql.Tx, now time.Time) (string, error) {
	year := now.Format("2006")
	var last sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT max(number) FROM orders WHERE number LIKE $1`, year+"-%").Scan(&last); err != nil {
		return "", fmt.Errorf("read last order number: %w", err)
	}

	n := 1
	if last.Valid {
		if _, after, ok := strings.Cut(last.String, "-"); ok {
			if prev, err := parseInt(after); err == nil {
				n = prev + 1
			}
		}
	}
	return fmt.Sprintf("%s-%04d", year, n), nil
}

func parseInt(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// orderColumns is the projection every order scan expects.
const orderColumns = `id, website_id, number, audience, currency,
	email, name, company, vat_number, phone,
	street, postal_code, city, country, note,
	items_net, items_tax, items_gross,
	shipping_net, shipping_tax, shipping_gross,
	total_net, total_tax, total_gross, vat_exempt,
	status, payment_method, payment_status, payment_reference, return_policy,
	created_at, updated_at`

func scanOrder(row interface{ Scan(...any) error }) (*Order, error) {
	var o Order
	var audience string
	var exempt int
	var createdAt, updatedAt string

	err := row.Scan(&o.ID, &o.WebsiteID, &o.Number, &audience, &o.Currency,
		&o.Customer.Email, &o.Customer.Name, &o.Customer.Company,
		&o.Customer.VATNumber, &o.Customer.Phone,
		&o.Customer.Street, &o.Customer.PostalCode, &o.Customer.City,
		&o.Customer.Country, &o.Customer.Note,
		&o.Totals.ItemsNet, &o.Totals.ItemsTax, &o.Totals.ItemsGross,
		&o.Totals.ShippingNet, &o.Totals.ShippingTax, &o.Totals.ShippingGross,
		&o.Totals.TotalNet, &o.Totals.TotalTax, &o.Totals.TotalGross, &exempt,
		&o.Status, &o.PaymentMethod, &o.PaymentStatus, &o.PaymentReference,
		&o.ReturnPolicy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	o.Audience = Audience(audience)
	o.VATExempt = exempt == 1
	o.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	o.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &o, nil
}

// ByNumber reads one order with its lines.
func (s *OrderStore) ByNumber(ctx context.Context, websiteID int64, number string) (*Order, error) {
	o, err := scanOrder(s.carts.DB.Read.QueryRowContext(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE website_id = $1 AND number = $2`,
		websiteID, number))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	items, err := s.itemsOf(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

// List returns a website's orders, newest first.
func (s *OrderStore) List(ctx context.Context, websiteID int64, limit int) ([]*Order, error) {
	rows, err := s.carts.DB.Read.QueryContext(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE website_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT $2`, websiteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *OrderStore) itemsOf(ctx context.Context, orderID int64) ([]OrderItem, error) {
	rows, err := s.carts.DB.Read.QueryContext(ctx,
		`SELECT id, product_id, title, subtitle, sku, quantity, tax_bp,
			unit_gross, unit_net, line_net, line_tax, line_gross
		 FROM order_items WHERE order_id = $1 ORDER BY position, id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderItem
	for rows.Next() {
		var it OrderItem
		var productID sql.NullInt64
		var taxBP int
		if err := rows.Scan(&it.ID, &productID, &it.Title, &it.Subtitle, &it.SKU,
			&it.Quantity, &taxBP, &it.UnitGross, &it.UnitNet,
			&it.LineNet, &it.LineTax, &it.LineGross); err != nil {
			return nil, err
		}
		it.TaxRate = money.TaxRate(taxBP)
		if productID.Valid {
			it.ProductID = &productID.Int64
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// SetStatus moves an order along.
func (s *OrderStore) SetStatus(ctx context.Context, websiteID, orderID int64, status string) error {
	switch status {
	case OrderNew, OrderPaid, OrderShipped, OrderCancelled:
	default:
		return errors.New("unbekannter Bestellstatus")
	}
	_, err := s.carts.DB.Write.ExecContext(ctx,
		`UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3 AND website_id = $4`,
		status, s.clock().Format(timeLayout), orderID, websiteID)
	return err
}

// SetPayment records what the payment provider said.
func (s *OrderStore) SetPayment(ctx context.Context, orderID int64, state, reference string) error {
	switch state {
	case PaymentOpen, PaymentPaid, PaymentFailed, PaymentRefunded:
	default:
		return errors.New("unbekannter Zahlungsstatus")
	}

	now := s.clock().Format(timeLayout)
	// A paid order moves to 'paid' as well, but only from 'new': an order the
	// operator has already shipped or cancelled must not jump backwards
	// because a webhook arrived late.
	if state == PaymentPaid {
		_, err := s.carts.DB.Write.ExecContext(ctx,
			`UPDATE orders SET payment_status = $1, payment_reference = $2,
			 status = CASE WHEN status = 'new' THEN 'paid' ELSE status END,
			 updated_at = $3 WHERE id = $4`,
			state, reference, now, orderID)
		return err
	}

	_, err := s.carts.DB.Write.ExecContext(ctx,
		`UPDATE orders SET payment_status = $1, payment_reference = $2, updated_at = $3
		 WHERE id = $4`, state, reference, now, orderID)
	return err
}
