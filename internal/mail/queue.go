package mail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// MaxAttempts is how often a message is retried before it is given up on.
//
// With the backoff below that spans about half a day, which covers a mail
// server that is down for a morning and a home connection that drops overnight.
// Beyond that the failure is not temporary, and retrying forever would mean a
// row that is tried for years and a log line every minute saying so.
const MaxAttempts = 8

// KeepSent is how long a delivered message stays in the table.
//
// "Did the invitation go out?" is the first question anyone asks, and without a
// trace there is no answer. A week is long enough to ask and short enough that
// the table does not become an archive of everything ever sent.
const KeepSent = 7 * 24 * time.Hour

// Queue is the outbox. Enqueue is what request handlers use; Flush is the job.
type Queue struct {
	db     *db.DB
	sender *Sender
	log    *slog.Logger
}

// NewQueue creates the outbox. A nil sender means messages are still queued but
// never leave — which is the right behaviour while an operator is still setting
// up their mail server, and visible on the status screen.
func NewQueue(database *db.DB, sender *Sender, log *slog.Logger) *Queue {
	if log == nil {
		log = slog.Default()
	}
	return &Queue{db: database, sender: sender, log: log}
}

// Enabled reports whether anything will actually be delivered.
func (q *Queue) Enabled() bool { return q != nil && q.sender.Enabled() }

// Enqueue puts a message in the outbox. It never connects to anything.
//
// A website id of 0 means the message is not about a particular site — an
// invitation or a password link, which belong to the installation.
func (q *Queue) Enqueue(ctx context.Context, websiteID int64, m Message) error {
	if q == nil {
		return ErrNotConfigured
	}
	if err := validAddress(m.To); err != nil {
		return err
	}

	var site any
	if websiteID > 0 {
		site = websiteID
	}
	_, err := q.db.Write.ExecContext(ctx,
		`INSERT INTO mail_outbox (website_id, recipient, subject, body, reply_to)
		 VALUES ($1, $2, $3, $4, $5)`,
		site, m.To, m.Subject, m.Body, m.ReplyTo)
	if err != nil {
		return fmt.Errorf("nachricht einreihen: %w", err)
	}
	return nil
}

// Flush delivers what is due. It is the job the runner calls.
//
// One message at a time, deliberately: a single writer connection gains
// nothing from parallel SMTP sessions, and a mail server that sees five
// connections at once from a small site is a mail server that starts
// throttling.
func (q *Queue) Flush(ctx context.Context) error {
	if q == nil || !q.sender.Enabled() {
		return nil
	}
	now := time.Now().UTC()

	rows, err := q.db.Read.QueryContext(ctx,
		`SELECT id, recipient, subject, body, reply_to, attempts
		 FROM mail_outbox
		 WHERE sent_at IS NULL AND next_try <= $1
		 ORDER BY id LIMIT 20`,
		now.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("postausgang lesen: %w", err)
	}

	type pending struct {
		id       int64
		msg      Message
		attempts int
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.msg.To, &p.msg.Subject, &p.msg.Body,
			&p.msg.ReplyTo, &p.attempts); err != nil {
			rows.Close()
			return fmt.Errorf("zeile lesen: %w", err)
		}
		batch = append(batch, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range batch {
		// The context is the runner's, so a shutdown stops between messages
		// rather than in the middle of an SMTP session.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := q.sender.Send(p.msg); err != nil {
			q.failed(ctx, p.id, p.attempts, err)
			continue
		}
		if _, err := q.db.Write.ExecContext(ctx,
			`UPDATE mail_outbox SET sent_at = $1, last_error = '' WHERE id = $2`,
			time.Now().UTC().Format(timeLayout), p.id); err != nil {
			// Delivered but not recorded. Worth shouting about: the next flush
			// will send it a second time, and a duplicate invitation is
			// confusing where a lost one is merely annoying.
			q.log.Error("mail sent but not marked", "id", p.id, "err", err)
		}
	}
	return nil
}

// failed records a failure and decides when to try again.
//
// The delay doubles: one minute, two, four, and so on. A mail server that is
// briefly busy is retried almost at once; one that is down for hours is not
// hammered while it is down.
func (q *Queue) failed(ctx context.Context, id int64, attempts int, cause error) {
	attempts++
	delay := time.Minute << min(attempts-1, 7)
	next := time.Now().UTC().Add(delay)

	if attempts >= MaxAttempts {
		// Given up on, but not deleted: the row and its reason are what an
		// operator looks at when someone says they never got the invitation.
		q.log.Error("giving up on mail", "id", id, "attempts", attempts, "err", cause)
		next = time.Now().UTC().Add(100 * 365 * 24 * time.Hour)
	} else {
		q.log.Warn("mail delivery failed, will retry", "id", id,
			"attempts", attempts, "in", delay, "err", cause)
	}

	if _, err := q.db.Write.ExecContext(ctx,
		`UPDATE mail_outbox SET attempts = $1, next_try = $2, last_error = $3 WHERE id = $4`,
		attempts, next.Format(timeLayout), truncate(cause.Error(), 500), id); err != nil {
		q.log.Error("cannot record mail failure", "id", id, "err", err)
	}
}

// Prune drops delivered messages that are old enough to have been asked about.
func (q *Queue) Prune(ctx context.Context) error {
	if q == nil {
		return nil
	}
	cutoff := time.Now().UTC().Add(-KeepSent).Format(timeLayout)
	res, err := q.db.Write.ExecContext(ctx,
		`DELETE FROM mail_outbox WHERE sent_at IS NOT NULL AND sent_at < $1`, cutoff)
	if err != nil {
		return fmt.Errorf("postausgang aufräumen: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		q.log.Info("pruned sent mail", "rows", n)
	}
	return nil
}

// Status is what the admin shows about the outbox.
type Status struct {
	Configured bool
	Host       string
	From       string
	// Pending is what is still waiting; Failed is what has been given up on.
	Pending int
	Failed  int
	// LastError is the most recent reason a delivery failed, or empty.
	LastError string
	LastSent  *time.Time
}

// Status reads the outbox for the status screen.
func (q *Queue) Status(ctx context.Context) (Status, error) {
	st := Status{}
	if q == nil {
		return st, nil
	}
	cfg := q.sender.Config()
	st.Configured = q.sender.Enabled()
	st.Host, st.From = cfg.Host, cfg.From

	err := q.db.Read.QueryRowContext(ctx,
		`SELECT
		   COUNT(*) FILTER (WHERE sent_at IS NULL AND attempts < $1),
		   COUNT(*) FILTER (WHERE sent_at IS NULL AND attempts >= $1)
		 FROM mail_outbox`, MaxAttempts).Scan(&st.Pending, &st.Failed)
	if err != nil {
		return st, fmt.Errorf("postausgang zählen: %w", err)
	}

	var last, sent sql.NullString
	err = q.db.Read.QueryRowContext(ctx,
		`SELECT last_error FROM mail_outbox
		 WHERE last_error != '' ORDER BY id DESC LIMIT 1`).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	st.LastError = last.String

	err = q.db.Read.QueryRowContext(ctx,
		`SELECT sent_at FROM mail_outbox
		 WHERE sent_at IS NOT NULL ORDER BY sent_at DESC LIMIT 1`).Scan(&sent)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	if sent.Valid {
		if t, err := time.Parse(timeLayout, sent.String); err == nil {
			st.LastSent = &t
		}
	}
	return st, nil
}

// Retry puts everything that was given up on back in the queue.
//
// The usual case is an operator who has just fixed a wrong password or a
// blocked port and wants what piled up to go out now rather than never.
func (q *Queue) Retry(ctx context.Context) (int64, error) {
	if q == nil {
		return 0, nil
	}
	res, err := q.db.Write.ExecContext(ctx,
		`UPDATE mail_outbox SET attempts = 0, next_try = $1, last_error = ''
		 WHERE sent_at IS NULL`, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("erneut versuchen: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
