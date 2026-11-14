package dsr

// The stores of the core. Each statement compares the address with
// `= $2 COLLATE NOCASE` after `website_id = $1`, the shape the indexes of
// migration 00066 serve; `lower(email) = lower(?)` would read the whole table.
//
// No statement here reads a secret: not orders.token, not a password hash, a
// second-factor secret, a token hash or a recovery code. What a source does
// not read cannot end up in an export (see export.go for why).

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/money"
)

// fieldLabels are the screen's names for the fields of the core sources, one
// entry per field name, so the catalogue collector sees each key once.
var fieldLabels = map[string]string{
	"email":             i18n.N("E-mail address"),
	"name":              i18n.N("Name"),
	"company":           i18n.N("Company"),
	"vat_number":        i18n.N("VAT number"),
	"phone":             i18n.N("Phone"),
	"street":            i18n.N("Street"),
	"postal_code":       i18n.N("Postal code"),
	"city":              i18n.N("City"),
	"country":           i18n.N("Country"),
	"note":              i18n.N("Note"),
	"status":            i18n.N("Status"),
	"payment_method":    i18n.N("Payment method"),
	"payment_status":    i18n.N("Payment status"),
	"payment_reference": i18n.N("Payment reference"),
	"total":             i18n.N("Total"),
	"item":              i18n.N("Item"),
}

// field builds a field with its label from fieldLabels. A name missing there
// is a programming error the tests catch: the label would be empty.
func field(name, value string) Field {
	return Field{Name: name, Label: fieldLabels[name], Value: value}
}

// coreSources are the stores of the core, in the order the screen and the
// export show them.
func (s *Service) coreSources() []Source {
	return []Source{
		ordersSource{s},
	}
}

func parseTime(v string) time.Time {
	t, _ := time.Parse(timeLayout, v)
	return t
}

// ordersSource is the shop's order book.
type ordersSource struct{ s *Service }

func (ordersSource) Name() string { return "orders" }

// ordersQuery is the statement ordersSource runs. It never selects token.
const ordersQuery = `SELECT id, number, currency, email, name, company, vat_number, phone,
	street, postal_code, city, country, note, total_gross, status, payment_method,
	payment_status, payment_reference, created_at
	FROM orders WHERE website_id = $1 AND email = $2 COLLATE NOCASE
	ORDER BY created_at, id`

func (o ordersSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	rows, err := o.s.db.Read.QueryContext(ctx, ordersQuery, websiteID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type found struct {
		id       int64
		currency string
		rec      Record
	}
	var list []found
	for rows.Next() {
		var (
			id                                                int64
			number, currency, addr, name, company, vat, phone string
			street, postal, city, country, note               string
			total                                             int64
			status, method, payStatus, payRef, createdAt      string
		)
		if err := rows.Scan(&id, &number, &currency, &addr, &name, &company, &vat, &phone,
			&street, &postal, &city, &country, &note, &total, &status, &method,
			&payStatus, &payRef, &createdAt); err != nil {
			return nil, err
		}
		cur := money.CurrencyFor(currency)
		rec := Record{
			Source: "orders",
			Key:    number,
			Kind:   "order",
			Label:  i18n.N("Order"),
			Time:   parseTime(createdAt),
			Link:   "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/bestellungen/" + number,
			Fields: []Field{
				field("email", addr),
				field("name", name),
				field("company", company),
				field("vat_number", vat),
				field("phone", phone),
				field("street", street),
				field("postal_code", postal),
				field("city", city),
				field("country", country),
				field("note", note),
				field("status", status),
				field("payment_method", method),
				field("payment_status", payStatus),
				field("payment_reference", payRef),
				field("total", cur.Format(money.Amount(total))),
			},
		}
		list = append(list, found{id: id, currency: currency, rec: rec})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	out := make([]Record, 0, len(list))
	for _, f := range list {
		items, err := o.items(ctx, f.id, money.CurrencyFor(f.currency))
		if err != nil {
			return nil, err
		}
		f.rec.Fields = append(f.rec.Fields, items...)
		out = append(out, f.rec)
	}
	return out, nil
}

// items are the lines of one order, one field each: "2 × Title – CHF 30.00".
// Not a sentence, so nothing for the catalogue: a quantity, a title as the
// customer saw it, and an amount.
func (o ordersSource) items(ctx context.Context, orderID int64, cur money.Currency) ([]Field, error) {
	rows, err := o.s.db.Read.QueryContext(ctx, `SELECT title, subtitle, sku, quantity, line_gross
		FROM order_items WHERE order_id = $1 ORDER BY position, id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Field
	for rows.Next() {
		var title, subtitle, sku string
		var qty int
		var gross int64
		if err := rows.Scan(&title, &subtitle, &sku, &qty, &gross); err != nil {
			return nil, err
		}
		what := title
		if subtitle != "" {
			what += ", " + subtitle
		}
		if sku != "" {
			what += " (" + sku + ")"
		}
		f := field("item", fmt.Sprintf("%d × %s – %s", qty, what, cur.Format(money.Amount(gross))))
		f.Name = "item_" + strconv.Itoa(len(out)+1)
		out = append(out, f)
	}
	return out, rows.Err()
}
