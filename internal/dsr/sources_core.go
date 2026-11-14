package dsr

// The stores of the core. Each statement compares the address with
// `= $2 COLLATE NOCASE` after `website_id = $1`, the shape the indexes of
// migration 00066 serve. Lower-casing both sides in SQL would compare the
// same way and read the whole table for every search.
//
// No statement sorts. With an ORDER BY created_at, SQLite prefers the
// website's (website_id, created_at) index to skip the sort and then reads
// every row of the website to compare the address; Service.Find sorts each
// source's few records by time instead.
//
// No statement here reads a secret: not orders.token, not a password hash, a
// second-factor secret, a token hash or a recovery code. What a source does
// not read cannot end up in an export (see export.go for why).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/user"
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
	"subject":           i18n.N("Subject"),
	"body":              i18n.N("Message"),
	"page":              i18n.N("Page"),
	"read_at":           i18n.N("Read"),
	"created_at":        i18n.N("Received"),
	"role":              i18n.N("Role"),
	"last_login_at":     i18n.N("Last sign-in"),
	"activity_entries":  i18n.N("Activity log entries on this website"),
	"kind":              i18n.N("Kind"),
	"language":          i18n.N("Language"),
	"gpc":               i18n.N("Global Privacy Control"),
	"handled_at":        i18n.N("Answered"),
	"recipient":         i18n.N("Recipient"),
	"reply_to":          i18n.N("Reply-To"),
	"sent_at":           i18n.N("Sent"),
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
		formMessagesSource{s},
		accountsSource{s},
		requestsSource{s},
		mailSource{s},
	}
}

func parseTime(v string) time.Time {
	t, _ := time.Parse(timeLayout, v)
	return t
}

// nullString is a nullable column as a field value: "" for NULL.
func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// formMessagesSource is the legacy contact-form table, from before the
// kontaktformular plugin. Nothing writes it any more, and it still holds
// whatever was sent before the plugin copied it, so it is searched.
type formMessagesSource struct{ s *Service }

func (formMessagesSource) Name() string { return "form_messages" }

// formMessagesQuery is the statement formMessagesSource runs. The page is
// joined on the website as well, so a page id can never name another
// website's page.
const formMessagesQuery = `SELECT fm.id, fm.name, fm.email, fm.subject, fm.body,
	COALESCE(p.title, ''), fm.created_at, fm.read_at
	FROM form_messages fm
	LEFT JOIN pages p ON p.id = fm.page_id AND p.website_id = fm.website_id
	WHERE fm.website_id = $1 AND fm.email = $2 COLLATE NOCASE`

func (f formMessagesSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	rows, err := f.s.db.Read.QueryContext(ctx, formMessagesQuery, websiteID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var id int64
		var name, addr, subject, body, page, createdAt string
		var readAt sql.NullString
		if err := rows.Scan(&id, &name, &addr, &subject, &body, &page, &createdAt, &readAt); err != nil {
			return nil, err
		}
		out = append(out, Record{
			Source: "form_messages",
			Key:    strconv.FormatInt(id, 10),
			Kind:   "form_submission",
			Label:  i18n.N("Form message"),
			Time:   parseTime(createdAt),
			Fields: []Field{
				field("name", name),
				field("email", addr),
				field("subject", subject),
				field("body", body),
				field("page", page),
				field("created_at", createdAt),
				field("read_at", nullString(readAt)),
			},
		})
	}
	return out, rows.Err()
}

// accountsSource is the staff accounts. They belong to the installation,
// not to a website, so an account is listed on a website only when it can
// work there — by the rule internal/user applies, not a copy of it.
type accountsSource struct{ s *Service }

func (accountsSource) Name() string { return "accounts" }

// accountsQuery is the statement accountsSource runs. users.email is UNIQUE
// COLLATE NOCASE, so it finds one account at most, through that index. It
// never selects password, totp_* or sso_username.
const accountsQuery = `SELECT id, email, name, role, created_at, last_login_at
	FROM users WHERE email = $1 COLLATE NOCASE`

func (a accountsSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	var (
		id                          int64
		addr, name, role, createdAt string
		lastLogin                   sql.NullString
	)
	err := a.s.db.Read.QueryRowContext(ctx, accountsQuery, email).
		Scan(&id, &addr, &name, &role, &createdAt, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rights, err := (&user.Store{DB: a.s.db}).Rights(ctx, id)
	if err != nil {
		return nil, err
	}
	if !rights.MayUse(websiteID) {
		return nil, nil
	}
	// The activity log keeps the staff member's address (D-09): it is the
	// record of who did what. The record says how much of it there is on
	// this website, so the operator can name it in the answer.
	var entries int
	if err := a.s.db.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_log
		WHERE website_id = $1 AND (user_id = $2 OR actor_email = $3 COLLATE NOCASE)`,
		websiteID, id, email).Scan(&entries); err != nil {
		return nil, err
	}
	return []Record{{
		Source: "accounts",
		Key:    strconv.FormatInt(id, 10),
		Kind:   "account",
		Label:  i18n.N("Staff account"),
		Time:   parseTime(createdAt),
		Link:   "/admin/users/" + strconv.FormatInt(id, 10) + "/edit",
		Fields: []Field{
			field("name", name),
			field("email", addr),
			field("role", role),
			// An account is created, not received: same stored name, its
			// own label.
			{Name: "created_at", Label: i18n.N("Created"), Value: createdAt},
			field("last_login_at", nullString(lastLogin)),
			field("activity_entries", strconv.Itoa(entries)),
		},
	}}, nil
}

// requestsSource is the privacy requests: opt-outs, and the data requests
// of Phase 33.2.
type requestsSource struct{ s *Service }

func (requestsSource) Name() string { return "requests" }

// requestsQuery is the statement requestsSource runs.
const requestsQuery = `SELECT id, kind, locale, name, email, note, gpc, created_at, handled_at
	FROM privacy_requests WHERE website_id = $1 AND email = $2 COLLATE NOCASE`

func (q requestsSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	rows, err := q.s.db.Read.QueryContext(ctx, requestsQuery, websiteID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var id, gpc int64
		var kind, loc, name, addr, note, createdAt string
		var handledAt sql.NullString
		if err := rows.Scan(&id, &kind, &loc, &name, &addr, &note, &gpc, &createdAt, &handledAt); err != nil {
			return nil, err
		}
		out = append(out, Record{
			Source: "requests",
			Key:    strconv.FormatInt(id, 10),
			Kind:   "request",
			Label:  i18n.N("Privacy request"),
			Time:   parseTime(createdAt),
			Link:   requestLink(websiteID, kind),
			Fields: []Field{
				field("kind", kind),
				field("name", name),
				field("email", addr),
				field("note", note),
				field("language", loc),
				field("gpc", strconv.FormatBool(gpc == 1)),
				field("created_at", createdAt),
				field("handled_at", nullString(handledAt)),
			},
		})
	}
	return out, rows.Err()
}

// requestLink is the screen a request of this kind is listed on, "" when
// there is none yet.
func requestLink(websiteID int64, kind string) string {
	if kind == KindOptOut {
		return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/widersprueche"
	}
	return ""
}

// mailSource is the mail the website sent or still has to send: the shop's
// order mail (outbox) and the server's queue (mail_outbox). A mail is the
// person's data when it goes to them, when its Reply-To is theirs (a
// plugin's notification of a message they sent), or when it belongs to one
// of their orders — the operator's copy of an order carries the customer's
// address and name in its body. The body is part of the record: it is what
// the website wrote about the person.
type mailSource struct{ s *Service }

func (mailSource) Name() string { return "mail" }

// outboxQuery is the order-mail half of mailSource. Two selects joined by
// UNION rather than one OR, so each half can use its index: the recipient
// through outbox_by_recipient, the order through idx_outbox_order.
const outboxQuery = `SELECT id, kind, recipient, reply_to, subject, body, status, created_at, sent_at
	FROM outbox WHERE website_id = $1 AND recipient = $2 COLLATE NOCASE
	UNION
	SELECT id, kind, recipient, reply_to, subject, body, status, created_at, sent_at
	FROM outbox WHERE website_id = $1 AND order_id IN
		(SELECT id FROM orders WHERE website_id = $1 AND email = $2 COLLATE NOCASE)`

// mailQueueQuery is the queue half. The queue is small and sent rows go
// after a week, so it has no address index and is read per website. A row
// without a website — an invitation, a password link — is never on a
// website's screen.
const mailQueueQuery = `SELECT id, recipient, reply_to, subject, body, attempts, created_at, sent_at
	FROM mail_outbox WHERE website_id = $1
		AND (recipient = $2 COLLATE NOCASE OR reply_to = $2 COLLATE NOCASE)`

func (m mailSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	out, err := m.outbox(ctx, websiteID, email)
	if err != nil {
		return nil, err
	}
	queued, err := m.queue(ctx, websiteID, email)
	if err != nil {
		return nil, err
	}
	return append(out, queued...), nil
}

func (m mailSource) outbox(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	rows, err := m.s.db.Read.QueryContext(ctx, outboxQuery, websiteID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var id int64
		var kind, recipient, replyTo, subject, body, status, createdAt string
		var sentAt sql.NullString
		if err := rows.Scan(&id, &kind, &recipient, &replyTo, &subject, &body, &status, &createdAt, &sentAt); err != nil {
			return nil, err
		}
		out = append(out, Record{
			Source: "mail",
			Key:    "outbox-" + strconv.FormatInt(id, 10),
			Kind:   "mail",
			Label:  i18n.N("Order mail"),
			Time:   parseTime(createdAt),
			Fields: []Field{
				field("kind", kind),
				field("recipient", recipient),
				field("reply_to", replyTo),
				field("subject", subject),
				field("status", status),
				field("created_at", createdAt),
				field("sent_at", nullString(sentAt)),
				field("body", body),
			},
		})
	}
	return out, rows.Err()
}

func (m mailSource) queue(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	rows, err := m.s.db.Read.QueryContext(ctx, mailQueueQuery, websiteID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var id, attempts int64
		var recipient, replyTo, subject, body, createdAt string
		var sentAt sql.NullString
		if err := rows.Scan(&id, &recipient, &replyTo, &subject, &body, &attempts, &createdAt, &sentAt); err != nil {
			return nil, err
		}
		// The queue keeps no status column; it is read the way the mail
		// screen reads it.
		status := "pending"
		switch {
		case sentAt.Valid:
			status = "sent"
		case attempts >= mail.MaxAttempts:
			status = "failed"
		}
		out = append(out, Record{
			Source: "mail",
			Key:    "queue-" + strconv.FormatInt(id, 10),
			Kind:   "mail",
			Label:  i18n.N("Queued mail"),
			Time:   parseTime(createdAt),
			Fields: []Field{
				field("recipient", recipient),
				field("reply_to", replyTo),
				field("subject", subject),
				field("status", status),
				field("created_at", createdAt),
				field("sent_at", nullString(sentAt)),
				field("body", body),
			},
		})
	}
	return out, rows.Err()
}

// ordersSource is the shop's order book.
type ordersSource struct{ s *Service }

func (ordersSource) Name() string { return "orders" }

// ordersQuery is the statement ordersSource runs. It never selects token.
const ordersQuery = `SELECT id, number, currency, email, name, company, vat_number, phone,
	street, postal_code, city, country, note, total_gross, status, payment_method,
	payment_status, payment_reference, created_at
	FROM orders WHERE website_id = $1 AND email = $2 COLLATE NOCASE`

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
