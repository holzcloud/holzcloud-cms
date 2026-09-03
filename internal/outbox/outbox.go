// Package outbox holds e-mail until it has actually been sent.
//
// Nothing here talks to a mail server in the request that created the message.
// A confirmation is written in the same moment as the order and carried out by
// a background run, which is the only arrangement that survives the two things
// that reliably happen: a mail server that takes ten seconds to answer, and one
// that is not answering at all.
//
// The consequence worth stating plainly: a message in here has been promised,
// not delivered. Everything that shows one — the admin's order page above all —
// has to say which of the two it is.
package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
)

// What a message is about.
const (
	KindOrderCustomer = "order_customer"
	KindOrderOperator = "order_operator"
	KindOrderShipped  = "order_shipped"
)

// Where a message stands.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// MaxAttempts is how often a message is tried before it is given up on.
//
// Five attempts spread over roughly three hours. Beyond that the problem is not
// going to fix itself: a misspelled address stays misspelled, and a mailbox
// that is full at three in the afternoon is still full at four.
const MaxAttempts = 5

const timeLayout = "2006-01-02T15:04:05Z"

// Mail is one message.
type Mail struct {
	ID        int64
	WebsiteID int64
	Kind      string
	// OrderID ties the message to an order, or is nil.
	OrderID   *int64
	Recipient string
	// FromName is the shop's name, so the confirmation is signed by the
	// business the customer ordered from rather than by the server.
	FromName string
	ReplyTo  string
	Subject  string
	Body     string

	Status        string
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
	CreatedAt     time.Time
	SentAt        *time.Time
}

// Store reads and writes the outbox.
type Store struct {
	DB  *db.DB
	now func() time.Time
}

func NewStore(database *db.DB) *Store { return &Store{DB: database} }

func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now().UTC()
}

// Queue accepts a message for sending.
//
// A message with no recipient is dropped rather than stored: a shop where the
// operator never filled in a notification address would otherwise accumulate
// one undeliverable row per order forever.
func (s *Store) Queue(ctx context.Context, m Mail) (int64, error) {
	if m.Recipient == "" {
		return 0, nil
	}
	now := s.clock()
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO outbox (website_id, kind, order_id, recipient, from_name,
		                     reply_to, subject, body, status, next_attempt_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $9)`,
		m.WebsiteID, m.Kind, m.OrderID, m.Recipient, m.FromName, m.ReplyTo,
		m.Subject, m.Body, now.Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("queue mail: %w", err)
	}
	return res.LastInsertId()
}

const columns = `id, website_id, kind, order_id, recipient, from_name, reply_to,
	subject, body, status, attempts, last_error, next_attempt_at, created_at, sent_at`

func scan(row interface{ Scan(...any) error }) (Mail, error) {
	var m Mail
	var next, created string
	var sent sql.NullString

	err := row.Scan(&m.ID, &m.WebsiteID, &m.Kind, &m.OrderID, &m.Recipient, &m.FromName,
		&m.ReplyTo, &m.Subject, &m.Body, &m.Status, &m.Attempts, &m.LastError,
		&next, &created, &sent)
	if err != nil {
		return m, err
	}
	m.NextAttemptAt, _ = time.Parse(timeLayout, next)
	m.CreatedAt, _ = time.Parse(timeLayout, created)
	if sent.Valid {
		t, _ := time.Parse(timeLayout, sent.String)
		m.SentAt = &t
	}
	return m, nil
}

// Due returns messages that are waiting and ready to be tried.
func (s *Store) Due(ctx context.Context, limit int) ([]Mail, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM outbox
		 WHERE status = 'pending' AND next_attempt_at <= $1
		 ORDER BY next_attempt_at, id LIMIT $2`,
		s.clock().Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Mail
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ForOrder returns every message that belongs to one order, newest last.
func (s *Store) ForOrder(ctx context.Context, orderID int64) ([]Mail, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM outbox WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Mail
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkSent records a delivered message.
func (s *Store) MarkSent(ctx context.Context, id int64) error {
	now := s.clock().Format(timeLayout)
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE outbox SET status = 'sent', sent_at = $1, attempts = attempts + 1,
		 last_error = '' WHERE id = $2`, now, id)
	return err
}

// MarkFailed records an attempt that did not work and schedules the next one.
//
// The message is only given up on once it has been tried MaxAttempts times.
// Until then the wait doubles, so a mail server that is down for an hour is
// knocked on a handful of times rather than every thirty seconds.
func (s *Store) MarkFailed(ctx context.Context, id int64, cause error) error {
	m, err := s.byID(ctx, id)
	if err != nil {
		return err
	}

	attempts := m.Attempts + 1
	status := StatusPending
	if attempts >= MaxAttempts {
		status = StatusFailed
	}

	next := s.clock().Add(backoff(attempts))
	_, err = s.DB.Write.ExecContext(ctx,
		`UPDATE outbox SET status = $1, attempts = $2, last_error = $3,
		 next_attempt_at = $4 WHERE id = $5`,
		status, attempts, trim(cause.Error()), next.Format(timeLayout), id)
	return err
}

// backoff is the wait before attempt n: one minute, five, thirty, two hours.
func backoff(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return time.Minute
	case attempt == 2:
		return 5 * time.Minute
	case attempt == 3:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

// trim keeps an error short enough to read in the admin.
func trim(s string) string {
	const max = 500
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func (s *Store) byID(ctx context.Context, id int64) (Mail, error) {
	return scan(s.DB.Read.QueryRowContext(ctx, `SELECT `+columns+` FROM outbox WHERE id = $1`, id))
}

// Retry puts a given-up message back in the queue.
//
// The operator's move after fixing whatever was wrong — a typo in the address,
// a mail account that had expired. The attempt counter starts over, otherwise
// the message would be given up on again immediately.
func (s *Store) Retry(ctx context.Context, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE outbox SET status = 'pending', attempts = 0, last_error = '',
		 next_attempt_at = $1 WHERE id = $2 AND status <> 'sent'`,
		s.clock().Format(timeLayout), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("diese Nachricht ist bereits verschickt")
	}
	return nil
}

// Prune drops delivered messages that are old enough to be uninteresting.
//
// Only delivered ones. A failure stays until someone has looked at it — that is
// the whole value of writing it down.
func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := s.clock().Add(-olderThan).Format(timeLayout)
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM outbox WHERE status = 'sent' AND sent_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Sender is what the dispatcher needs from a mail account.
//
// An interface rather than *mail.Sender so a test can watch what was handed
// over and make sending fail on demand. That matters more here than anywhere
// else in this package: the interesting behaviour is what happens when sending
// does not work.
type Sender interface {
	Configured() bool
	Send(context.Context, mail.Message) error
}

// Dispatcher carries the outbox out over SMTP.
type Dispatcher struct {
	Store  *Store
	Sender Sender
	// Batch bounds one run. Without it a backlog of a thousand messages would
	// hold the connection open for as long as it takes to send them all.
	Batch int
}

// Run sends what is due. Suitable as a periodic job.
//
// It returns an error only for something that stopped the run as a whole; a
// message that could not be delivered is recorded against that message and does
// not stop the ones behind it.
func (d *Dispatcher) Run(ctx context.Context) error {
	if d.Store == nil {
		return nil
	}
	// Not an error: a shop without mail configured is a working shop. Saying so
	// once a minute in the log would be noise.
	if d.Sender == nil || !d.Sender.Configured() {
		return nil
	}

	batch := d.Batch
	if batch <= 0 {
		batch = 20
	}

	due, err := d.Store.Due(ctx, batch)
	if err != nil {
		return fmt.Errorf("outbox lesen: %w", err)
	}

	for _, m := range due {
		if ctx.Err() != nil {
			return nil
		}
		err := d.Sender.Send(ctx, mail.Message{
			To:       m.Recipient,
			ReplyTo:  m.ReplyTo,
			Subject:  m.Subject,
			Body:     m.Body,
			FromName: m.FromName,
		})
		if err != nil {
			slog.Warn("mail not sent", "id", m.ID, "kind", m.Kind,
				"attempt", m.Attempts+1, "err", err)
			if err := d.Store.MarkFailed(ctx, m.ID, err); err != nil {
				return fmt.Errorf("fehlschlag vermerken: %w", err)
			}
			continue
		}
		if err := d.Store.MarkSent(ctx, m.ID); err != nil {
			// The message is out. Failing to write that down would send it
			// again on the next run, which is worse than a log line.
			return fmt.Errorf("versand vermerken: %w", err)
		}
		slog.Info("mail sent", "id", m.ID, "kind", m.Kind)
	}
	return nil
}
