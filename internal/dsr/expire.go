package dsr

// The expirers of the retention job: per kind, what one website deletes once
// the kind's period has passed.
//
// Every statement is keyed by the website, and every delete goes in batches,
// so one run never holds the write connection for long and one website never
// reaches another's rows. The errors built here carry counts, kinds, plugin
// ids and media ids only — never a value of a record — because they are
// stored in retention_runs and written to the journal.

import (
	"context"
	"fmt"
	"time"
)

// expireBatch is how many rows one delete statement removes at most.
const expireBatch = 500

// expireKind deletes what of one kind of one website has expired, and how
// many records went.
func (s *Service) expireKind(ctx context.Context, ws retentionWebsite, p Period, now time.Time) (int, error) {
	switch p.Kind {
	case RetentionFormSubmission:
		return s.expireFormMessages(ctx, ws.id, now.AddDate(0, -p.Months, 0))
	}
	return 0, fmt.Errorf("no expirer for the kind %s", p.Kind)
}

// expireFormMessages deletes the legacy contact-form messages — the table
// from before the contact-form plugin — that came in before the cut-off.
func (s *Service) expireFormMessages(ctx context.Context, websiteID int64, before time.Time) (int, error) {
	return s.deleteBatched(ctx, "form messages", `DELETE FROM form_messages WHERE website_id = $1 AND id IN
		(SELECT id FROM form_messages WHERE website_id = $1 AND created_at < $2 LIMIT $3)`,
		websiteID, before.UTC().Format(timeLayout), expireBatch)
}

// deleteBatched runs a delete whose last argument is the batch size until a
// batch deletes fewer rows than that, and counts what went.
func (s *Service) deleteBatched(ctx context.Context, what, query string, args ...any) (int, error) {
	total := 0
	for {
		res, err := s.db.Write.ExecContext(ctx, query, args...)
		if err != nil {
			return total, fmt.Errorf("delete %s: %w", what, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, fmt.Errorf("delete %s: %w", what, err)
		}
		total += int(n)
		if n < expireBatch {
			return total, nil
		}
	}
}
