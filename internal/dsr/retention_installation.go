package dsr

// The installation-wide part of retention (Phase 33.2, DSR-04, D-04, D-05,
// D-09): whether the job has fallen behind, and the kinds that belong to no
// single website.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The kinds of record retention deletes that have no period per website
// (D-09).
const (
	// RetentionAccount is a staff account without a sign-in for the
	// installation's account period. Staff accounts belong to the
	// installation, not to one website, so the period is one setting and
	// the job records them on the installation's row (website_id NULL).
	RetentionAccount = "account"
	// RetentionMailFailed is mail of either queue that was never sent. Per
	// website for the mail a website queued, on the installation's row for
	// the installation's own (invitations, password links).
	RetentionMailFailed = "mail_failed"
)

// MailFailedAfter is how long mail that was never sent is kept after it was
// queued (D-09). Both queues give up long before: mail_outbox after its
// attempts, the shop's outbox after its own. What stays is a recipient, a
// subject and a text that will not go out — often to a mistyped address —
// and without this nothing ever deleted it. Ninety days is the time the
// outbox keeps sent mail: long enough to answer "did my confirmation go
// out?", no longer.
const MailFailedAfter = 90 * 24 * time.Hour

// The account period (D-04, D-09): one setting for the installation, in
// app_settings, 24 months unless an administrator chose another.
const (
	accountMonthsKey     = "retention.account_months"
	defaultAccountMonths = 24
)

// overdueAfter is how long the job may go without a successful run before
// an administrator is warned (D-09). The job ticks every hour and runs once
// a day, at the latest 30 hours after its last success (dueAt); 48 hours is
// that day and its retry window, and then most of another day. A warning
// earlier would fire on every restart that happens to fall in a quiet hour;
// one much later lets expired personal data stay for days with nobody told.
const overdueAfter = 48 * time.Hour

// retentionSinceKey is the app_settings row migration 00068 writes: the
// moment the program that runs the job was installed.
const retentionSinceKey = "retention.since"

// RetentionState is whether the retention job keeps up.
type RetentionState struct {
	// LastOK is when the last successful run started, nil when none did.
	LastOK *time.Time
	// Since is when this version of the program, the first with the job,
	// was installed.
	Since time.Time
	// Overdue is true when the later of LastOK and Since lies more than 48
	// hours before now.
	Overdue bool
}

// RetentionStatus says whether the job has not succeeded for more than 48
// hours (D-05, D-09).
//
// It counts from the last successful run, and from retention.since when
// that is later or when there is none. A job that never ran has written no
// run to be late by: without the second reference the warning would wait
// for a first success that never comes — exactly the silent failure D-05
// rules out. With it, a fresh update is not warned about either: its first
// run comes within the hour, long before 48 hours have passed.
//
// A database without retention.since — migrated before 00068 wrote it, or
// with the row deleted by hand — counts from now, and the row is written so
// the next call counts from the same moment. Not writing it would move the
// reference with every call and never warn; treating it as the distant past
// would warn on a reference nobody can see.
func (s *Service) RetentionStatus(ctx context.Context, now time.Time) (RetentionState, error) {
	var st RetentionState
	last, err := s.LastSuccess(ctx)
	if err != nil {
		return st, err
	}
	st.LastOK = last
	since, err := s.retentionSince(ctx, now)
	if err != nil {
		return st, err
	}
	st.Since = since
	ref := since
	if last != nil && last.After(ref) {
		ref = *last
	}
	st.Overdue = now.Sub(ref) > overdueAfter
	return st, nil
}

// retentionSince reads retention.since, and writes now when it is missing.
func (s *Service) retentionSince(ctx context.Context, now time.Time) (time.Time, error) {
	var value string
	err := s.db.Read.QueryRowContext(ctx,
		`SELECT value FROM app_settings WHERE key = $1`, retentionSinceKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := s.db.Write.ExecContext(ctx, `INSERT OR IGNORE INTO app_settings (key, value) VALUES ($1, $2)`,
			retentionSinceKey, now.UTC().Format(timeLayout)); err != nil {
			return time.Time{}, fmt.Errorf("write %s: %w", retentionSinceKey, err)
		}
		// Read back: a concurrent call may have written first, and its
		// moment is the one that stays.
		if err := s.db.Write.QueryRowContext(ctx,
			`SELECT value FROM app_settings WHERE key = $1`, retentionSinceKey).Scan(&value); err != nil {
			return time.Time{}, fmt.Errorf("read %s: %w", retentionSinceKey, err)
		}
	} else if err != nil {
		return time.Time{}, fmt.Errorf("read %s: %w", retentionSinceKey, err)
	}
	t, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s is not a time: %w", retentionSinceKey, err)
	}
	return t, nil
}

// AccountMonths is how many months a staff account may go without a sign-in
// before the job deletes it: the stored setting, or 24.
func (s *Service) AccountMonths(ctx context.Context) (int, error) {
	var value string
	err := s.db.Read.QueryRowContext(ctx,
		`SELECT value FROM app_settings WHERE key = $1`, accountMonthsKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultAccountMonths, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", accountMonthsKey, err)
	}
	m, err := strconv.Atoi(value)
	if err != nil || m < 1 || m > maxPeriodMonths {
		// A value nobody could have saved through the screen: the default,
		// rather than a period of zero that deletes every account at once.
		return defaultAccountMonths, nil
	}
	return m, nil
}

// SetAccountMonths stores the account period, 1 to 600 months, or
// ErrBadPeriod. The default is stored like any other value: the setting is
// the installation's, and an administrator who chose 24 chose it.
func (s *Service) SetAccountMonths(ctx context.Context, months int) error {
	if months < 1 || months > maxPeriodMonths {
		return &PeriodError{Kind: RetentionAccount, Err: ErrBadPeriod}
	}
	if _, err := s.db.Write.ExecContext(ctx, `INSERT INTO app_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, accountMonthsKey, strconv.Itoa(months)); err != nil {
		return fmt.Errorf("store %s: %w", accountMonthsKey, err)
	}
	return nil
}

// accountCutoff is the moment before which an account's last sign-in, or
// its creation when it never signed in, makes it expire.
func (s *Service) accountCutoff(ctx context.Context, now time.Time) (string, error) {
	months, err := s.AccountMonths(ctx)
	if err != nil {
		return "", err
	}
	return now.AddDate(0, -months, 0).UTC().Format(timeLayout), nil
}

// inactiveWhere is the condition of an account past the period: its last
// sign-in, or its creation when there never was one, before the cut-off.
const inactiveWhere = `COALESCE(last_login_at, created_at) < $1`

// InactiveAdmins is how many administrator accounts have not signed in for
// longer than the account period. The job never deletes them (D-09): an
// installation whose administrators all expired would have nobody left to
// sign in, and the last-administrator rule would stop only the last of
// them. The Retention screen says how many there are instead.
func (s *Service) InactiveAdmins(ctx context.Context, now time.Time) (int, error) {
	cutoff, err := s.accountCutoff(ctx, now)
	if err != nil {
		return 0, err
	}
	var n int
	if err := s.db.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = $2 AND `+inactiveWhere,
		cutoff, user.RoleAdmin).Scan(&n); err != nil {
		return 0, fmt.Errorf("count inactive administrators: %w", err)
	}
	return n, nil
}

// expireFailedMail deletes the website's mail of both queues that was never
// sent and was queued before the cut-off. Sent mail is left to the queues'
// own pruning.
func (s *Service) expireFailedMail(ctx context.Context, websiteID int64, now time.Time) (int, error) {
	before := now.Add(-MailFailedAfter).UTC().Format(timeLayout)
	n, err := s.deleteBatched(ctx, "unsent mail", `DELETE FROM mail_outbox WHERE website_id = $1 AND id IN
		(SELECT id FROM mail_outbox WHERE website_id = $1 AND sent_at IS NULL AND created_at < $2 LIMIT $3)`,
		websiteID, before, expireBatch)
	if err != nil {
		return n, err
	}
	m, err := s.deleteBatched(ctx, "unsent shop mail", `DELETE FROM outbox WHERE website_id = $1 AND id IN
		(SELECT id FROM outbox WHERE website_id = $1 AND status <> 'sent' AND created_at < $2 LIMIT $3)`,
		websiteID, before, expireBatch)
	return n + m, err
}

// expireOwnFailedMail deletes the installation's own mail — invitations and
// password links, mail_outbox rows without a website — that was never sent
// and was queued before the cut-off.
func (s *Service) expireOwnFailedMail(ctx context.Context, now time.Time) (int, error) {
	before := now.Add(-MailFailedAfter).UTC().Format(timeLayout)
	return s.deleteBatched(ctx, "unsent mail of the installation", `DELETE FROM mail_outbox WHERE website_id IS NULL AND id IN
		(SELECT id FROM mail_outbox WHERE website_id IS NULL AND sent_at IS NULL AND created_at < $1 LIMIT $2)`,
		before, expireBatch)
}

// expireAccounts deletes the staff accounts past the account period that
// are not administrators, and ends their sessions (D-09).
//
// Each account goes through user.Store.Delete, never through a statement of
// this package: the store keeps the rule that the last administrator stays,
// and whatever it learns later. The condition is asked again for each
// account just before it is deleted, so an account promoted or signed in
// since the list was read stays. Its user_websites rows go by the cascade;
// the activity log keeps the address it acted under (D-09: accountability,
// GDPR Art. 17(3)(e)) — the log's own manual purge removes it.
func (s *Service) expireAccounts(ctx context.Context, now time.Time) (int, error) {
	cutoff, err := s.accountCutoff(ctx, now)
	if err != nil {
		return 0, &expireError{cause: causeDatabase, detail: "read the account period: " + err.Error()}
	}
	ids, err := s.inactiveAccounts(ctx, cutoff)
	if err != nil {
		return 0, &expireError{cause: causeDatabase, detail: "list inactive accounts: " + err.Error()}
	}
	deleted := 0
	var errs []error
	for _, id := range ids {
		var still int
		if err := s.db.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = $3 AND role <> $2 AND `+inactiveWhere,
			cutoff, user.RoleAdmin, id).Scan(&still); err != nil {
			errs = append(errs, &expireError{cause: causeDatabase, detail: fmt.Sprintf("account %d: %v", id, err)})
			continue
		}
		if still == 0 {
			continue
		}
		err := s.users.Delete(ctx, id)
		switch {
		case errors.Is(err, user.ErrNotFound):
			// Deleted by somebody else in the meantime.
			continue
		case errors.Is(err, user.ErrLastAdmin):
			// Cannot happen for an account that is not an administrator;
			// if it does, the store and this job disagree about roles.
			errs = append(errs, &expireError{cause: causeDatabase, detail: fmt.Sprintf(
				"account %d: the store refused it as the last administrator", id)})
			continue
		case err != nil:
			errs = append(errs, &expireError{cause: causeDatabase, detail: fmt.Sprintf("delete account %d: %v", id, err)})
			continue
		}
		deleted++
		// A session of a deleted account must not stay usable. The sign-in
		// check would refuse it at the next request, but a session store
		// full of them is a store that lies about who is signed in.
		if s.sessions != nil {
			if err := auth.DestroyUserSessions(ctx, s.sessions, id, ""); err != nil {
				errs = append(errs, &expireError{cause: causeDatabase, detail: fmt.Sprintf(
					"end the sessions of account %d: %v", id, err)})
			}
		}
	}
	return deleted, errors.Join(errs...)
}

// inactiveAccounts are the ids of the accounts past the cut-off that are not
// administrators.
func (s *Service) inactiveAccounts(ctx context.Context, cutoff string) ([]int64, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT id FROM users WHERE role <> $2 AND `+inactiveWhere+` ORDER BY id`,
		cutoff, user.RoleAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// expireInstallation runs the kinds that belong to no website and returns
// one result per kind, with WebsiteID 0. The accounts run only when the
// service has them (SetUsers): main.go always hands them over, and a test
// of something else that never did gets no account row.
func (s *Service) expireInstallation(ctx context.Context, now time.Time) ([]KindResult, error) {
	var results []KindResult
	var errs []error
	if s.users != nil {
		kr, err := s.runKind(0, RetentionAccount, func() (int, error) { return s.expireAccounts(ctx, now) })
		results, errs = append(results, kr), append(errs, err)
	}
	kr, err := s.runKind(0, RetentionMailFailed, func() (int, error) { return s.expireOwnFailedMail(ctx, now) })
	results, errs = append(results, kr), append(errs, err)
	return results, errors.Join(errs...)
}

// runKind runs one kind of one website (0: the installation) and returns its
// result; the error names the website and kind for the journal.
func (s *Service) runKind(websiteID int64, kind string, fn func() (int, error)) (KindResult, error) {
	kr := KindResult{WebsiteID: websiteID, Kind: kind}
	var err error
	if s.failKind != nil {
		err = s.failKind(websiteID, kind)
	}
	if err == nil {
		kr.Deleted, err = fn()
	}
	if err == nil {
		return kr, nil
	}
	kr.Err = bounded(err.Error(), 2000)
	if websiteID == 0 {
		return kr, fmt.Errorf("installation, %s: %w", kind, err)
	}
	return kr, fmt.Errorf("website %d, %s: %w", websiteID, kind, err)
}
