package dsr

// The installation-wide part of retention (Phase 33.2, DSR-04, D-04, D-05,
// D-09): whether the job has fallen behind, and the kinds that belong to no
// single website.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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
