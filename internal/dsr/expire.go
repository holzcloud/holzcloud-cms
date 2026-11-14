package dsr

// The expirers of the retention job: per kind, what one website deletes once
// the kind's period has passed.
//
//   - Form submissions: the legacy contact-form messages (form_messages, from
//     before the contact-form plugin) and what every plugin keeps of kind
//     form_submission, with the files of its records.
//   - Privacy requests: every kind, opt-outs included, counted from the later
//     of arrival and answer — the record of a request is kept for the request
//     period after it was answered, as migration 00065 keeps the website.
//   - Orders: the shop's orders and every plugin's records of kind order,
//     after the period of the seller's country from the end of the calendar
//     year of the order (D-06), and never before a date an erasure stored.
//
// Every statement is keyed by the website, and every delete goes in batches,
// so one run never holds the write connection for long and one website never
// reaches another's rows. The errors built here carry counts, kinds, plugin
// ids and media ids only — never a value of a record, never a file name —
// because they are stored in retention_runs and written to the journal.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// expireBatch is how many rows one delete statement removes at most.
const expireBatch = 500

// The bound of one plugin's expiry in one run: at most expireCalls calls of
// at most expireLimit records each. One call has to finish inside the
// plugin's call timeout, and one run must end. A plugin that still has more
// after the last call fails its kind for this run; the next run goes on
// where this one stopped. Variables so a test can lower them.
var (
	expireCalls = 50
	expireLimit = 200
)

// The causes of a failed kind, the first word of its stored error. The
// screen turns a cause into a sentence; the rest of the error is for the
// journal and whoever reads the database.
const (
	causeBound    = "bound"
	causeFile     = "file"
	causePlugin   = "plugin"
	causeDatabase = "database"
)

// expireError is a failure of one kind, with the cause the screen names.
type expireError struct {
	cause, detail string
}

func (e *expireError) Error() string { return e.cause + ": " + e.detail }

// expireKind deletes what of one kind of one website has expired, and says
// how many records went. covered are the plugins that answer the privacy
// hook on the website; pluginsErr is why they could not be listed.
func (s *Service) expireKind(ctx context.Context, ws retentionWebsite, p Period, now time.Time, covered []string, pluginsErr error) (int, error) {
	months := max(p.Months, p.Floor)
	switch p.Kind {
	case RetentionFormSubmission:
		before := now.AddDate(0, -months, 0)
		n, err := s.expireFormMessages(ctx, ws.id, before)
		m, perr := s.expirePlugins(ctx, ws.id, plugin.PrivacyKindFormSubmission, before, covered, pluginsErr)
		return n + m, errors.Join(err, perr)
	case RetentionRequest:
		return s.expireRequests(ctx, ws.id, now.AddDate(0, -months, 0))
	case RetentionOrder:
		years := max(months/12, legal.OrderPeriodFor(ws.country).Years)
		before := legal.OrderCutoff(now, years)
		n, err := s.expireOrders(ctx, ws.id, before, now)
		m, perr := s.expirePlugins(ctx, ws.id, plugin.PrivacyKindOrder, before, covered, pluginsErr)
		return n + m, errors.Join(err, perr)
	}
	return 0, &expireError{cause: causeDatabase, detail: "no expirer for the kind " + p.Kind}
}

// expireFormMessages deletes the legacy contact-form messages that came in
// before the cut-off.
func (s *Service) expireFormMessages(ctx context.Context, websiteID int64, before time.Time) (int, error) {
	return s.deleteBatched(ctx, "form messages", `DELETE FROM form_messages WHERE website_id = $1 AND id IN
		(SELECT id FROM form_messages WHERE website_id = $1 AND created_at < $2 LIMIT $3)`,
		websiteID, before.UTC().Format(timeLayout), expireBatch)
}

// expireRequests deletes the privacy requests whose arrival and answer both
// lie before the cut-off. An unanswered request counts from its arrival.
func (s *Service) expireRequests(ctx context.Context, websiteID int64, before time.Time) (int, error) {
	return s.deleteBatched(ctx, "privacy requests", `DELETE FROM privacy_requests WHERE website_id = $1 AND id IN
		(SELECT id FROM privacy_requests WHERE website_id = $1 AND created_at < $2
		   AND (handled_at IS NULL OR handled_at < $2) LIMIT $3)`,
		websiteID, before.UTC().Format(timeLayout), expireBatch)
}

// expireOrders deletes the shop's orders placed before the cut-off whose
// stored retain_until, if any, has come. Their items and their mails go with
// them by the cascade of order_items and outbox.
func (s *Service) expireOrders(ctx context.Context, websiteID int64, before, now time.Time) (int, error) {
	return s.deleteBatched(ctx, "orders", `DELETE FROM orders WHERE website_id = $1 AND id IN
		(SELECT id FROM orders WHERE website_id = $1 AND created_at < $2
		   AND (retain_until IS NULL OR retain_until <= $3) LIMIT $4)`,
		websiteID, before.UTC().Format(timeLayout), now.UTC().Format(timeLayout), expireBatch)
}

// deleteBatched runs a delete whose last argument is the batch size until a
// batch deletes fewer rows than that, and counts what went.
func (s *Service) deleteBatched(ctx context.Context, what, query string, args ...any) (int, error) {
	total := 0
	for {
		res, err := s.db.Write.ExecContext(ctx, query, args...)
		if err != nil {
			return total, &expireError{cause: causeDatabase, detail: "delete " + what + ": " + err.Error()}
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, &expireError{cause: causeDatabase, detail: "delete " + what + ": " + err.Error()}
		}
		total += int(n)
		if n < expireBatch {
			return total, nil
		}
	}
}

// expirePlugins asks every covered plugin to expire its records of the kind
// that came in before the cut-off. A plugin that fails does not stop the
// others.
func (s *Service) expirePlugins(ctx context.Context, websiteID int64, kind string, before time.Time, covered []string, pluginsErr error) (int, error) {
	total := 0
	errs := []error{pluginsErr}
	for _, id := range covered {
		n, err := s.expirePlugin(ctx, websiteID, id, kind, before)
		total += n
		errs = append(errs, err)
	}
	return total, errors.Join(errs...)
}

// expirePlugin is the loop of one plugin: expire calls until the plugin has
// no more, at most expireCalls of them, deleting the files of each call's
// records as it goes.
//
// A media id the plugin returns is a claim, so it goes through
// media.Store.DeleteOwned with this website: an id of another website is
// refused and is an error of the kind, as is a file that stays on the disk.
// An id that no longer exists is what was asked for and is not an error.
func (s *Service) expirePlugin(ctx context.Context, websiteID int64, id, kind string, before time.Time) (int, error) {
	deleted := 0
	var errs []error
	for call := 0; call < expireCalls; call++ {
		out, err := s.plugins.Privacy(ctx, websiteID, id, plugin.PrivacyIn{
			Op:     plugin.PrivacyExpire,
			Kind:   kind,
			Before: before.UTC().Format(time.RFC3339),
			Limit:  expireLimit,
		})
		if err != nil {
			errs = append(errs, pluginFailure(id, err))
			return deleted, errors.Join(errs...)
		}
		deleted += out.Deleted
		for _, mid := range out.MediaIDs {
			if err := s.deleteMedia(ctx, websiteID, id, mid); err != nil {
				errs = append(errs, err)
			}
		}
		if !out.More {
			return deleted, errors.Join(errs...)
		}
	}
	errs = append(errs, &expireError{cause: causeBound, detail: fmt.Sprintf(
		"plugin %s: more to expire than one run may delete (%d calls of %d); the next run continues",
		id, expireCalls, expireLimit)})
	return deleted, errors.Join(errs...)
}

// deleteMedia removes one file a plugin named for a record it expired.
func (s *Service) deleteMedia(ctx context.Context, websiteID int64, pluginID string, mediaID int64) error {
	if s.media == nil {
		return &expireError{cause: causeFile, detail: fmt.Sprintf(
			"media %d of plugin %s: no media store to delete it with", mediaID, pluginID)}
	}
	err := s.media.DeleteOwned(ctx, websiteID, mediaID, s.dataDir)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, media.ErrNotOwned):
		return &expireError{cause: causePlugin, detail: fmt.Sprintf(
			"plugin %s named media %d, which is not this website's; it was left alone", pluginID, mediaID)}
	case errors.Is(err, media.ErrNotFound):
		// Gone already, for instance deleted in the media library.
		return nil
	case errors.Is(err, media.ErrFileRemains):
		return &expireError{cause: causeFile, detail: fmt.Sprintf(
			"media %d of plugin %s stayed on the disk", mediaID, pluginID)}
	}
	return &expireError{cause: causeDatabase, detail: fmt.Sprintf("media %d of plugin %s: %v", mediaID, pluginID, err)}
}

// pluginFailure names what went wrong with a plugin's expiry by its kind,
// not by the plugin's own words: a plugin's message is free text, and this
// error is stored and logged.
func pluginFailure(id string, err error) error {
	what := "the call failed"
	switch {
	case errors.Is(err, plugin.ErrPrivacyFailed):
		what = "the plugin refused the expiry"
	case errors.Is(err, plugin.ErrNoPrivacyAnswer):
		what = "the plugin gave no answer"
	case errors.Is(err, plugin.ErrNotServed):
		what = "the plugin does not serve the privacy hook here"
	case errors.Is(err, context.DeadlineExceeded):
		what = "the call took too long"
	}
	return &expireError{cause: causePlugin, detail: "plugin " + id + ": " + what}
}
