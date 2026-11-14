package dsr

import (
	"context"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// The installation-wide part of the retention job (Phase 33.2, plan 07):
// whether the job is overdue, staff accounts, failed mail and the activity
// summary. Every helper of this file starts with installation.

// installationSince sets the moment the warning counts from on an
// installation where the job never succeeded.
func installationSince(t *testing.T, database *db.DB, at time.Time) {
	t.Helper()
	if _, err := database.Write.Exec(`INSERT INTO app_settings (key, value) VALUES ('retention.since', $1)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, at.UTC().Format(timeLayout)); err != nil {
		t.Fatalf("set retention.since: %v", err)
	}
}

func TestTheRetentionStateKnowsWhenItIsOverdue(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Never ran, and installed three days ago: the job that never runs is
	// exactly the failure nobody would otherwise see.
	s, database := retentionService(t)
	installationSince(t, database, now.Add(-72*time.Hour))
	st, err := s.RetentionStatus(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Overdue || st.LastOK != nil {
		t.Errorf("never ran, installed 3 days ago: %+v, want overdue without a last success", st)
	}
	if !st.Since.Equal(now.Add(-72 * time.Hour)) {
		t.Errorf("Since = %v, want %v", st.Since, now.Add(-72*time.Hour))
	}

	// A success 47 hours ago is not late yet.
	retentionRun(t, database, now.Add(-47*time.Hour), RunOK)
	st, err = s.RetentionStatus(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if st.Overdue {
		t.Errorf("an ok run 47 h ago: %+v, want not overdue", st)
	}
	if st.LastOK == nil || !st.LastOK.Equal(now.Add(-47*time.Hour)) {
		t.Errorf("LastOK = %v, want the run of 47 h ago", st.LastOK)
	}

	// A success 49 hours ago and a failure since: a job that runs and fails
	// is as late as one that does not run.
	s, database = retentionService(t)
	installationSince(t, database, now.Add(-30*24*time.Hour))
	retentionRun(t, database, now.Add(-49*time.Hour), RunOK)
	retentionRun(t, database, now.Add(-time.Hour), RunFailed)
	st, err = s.RetentionStatus(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Overdue {
		t.Errorf("an ok run 49 h ago and a failed one 1 h ago: %+v, want overdue", st)
	}
	if st.LastOK == nil || !st.LastOK.Equal(now.Add(-49*time.Hour)) {
		t.Errorf("LastOK = %v, want the run of 49 h ago", st.LastOK)
	}

	// Updated an hour ago and not run yet: the first run comes within the
	// hour, and a warning now would be a false alarm on every update.
	s, database = retentionService(t)
	installationSince(t, database, now.Add(-time.Hour))
	st, err = s.RetentionStatus(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if st.Overdue {
		t.Errorf("updated 1 h ago, no run: %+v, want not overdue", st)
	}

	// No reference at all: counted from now, and written down, so the
	// warning comes 48 hours later and not never.
	s, database = retentionService(t)
	if _, err := database.Write.Exec(`DELETE FROM app_settings WHERE key = 'retention.since'`); err != nil {
		t.Fatal(err)
	}
	st, err = s.RetentionStatus(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if st.Overdue || !st.Since.Equal(now) {
		t.Errorf("without retention.since: %+v, want not overdue, since now", st)
	}
	var stored string
	if err := database.Read.QueryRow(`SELECT value FROM app_settings WHERE key = 'retention.since'`).Scan(&stored); err != nil {
		t.Fatalf("retention.since was not written: %v", err)
	}
	if stored != now.Format(timeLayout) {
		t.Errorf("retention.since = %q, want %q", stored, now.Format(timeLayout))
	}
	st, err = s.RetentionStatus(ctx, now.Add(49*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Overdue {
		t.Errorf("49 h after the reference was written: %+v, want overdue", st)
	}
}
