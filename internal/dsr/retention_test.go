package dsr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// The retention job (Phase 33.2, DSR-04, D-04, D-05, D-09). Every helper of
// this file starts with retention, so the tests beside it keep their names.

// retentionService is the service over a fresh database.
func retentionService(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	_, database := newTestStore(t)
	return NewService(database), database
}

// retentionFormMessage stores a legacy contact-form message that came in at.
func retentionFormMessage(t *testing.T, database *db.DB, websiteID int64, at time.Time) int64 {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
		VALUES ($1, 'Ada Lovelace', 'ada@example.com', 'Eier', 'Habt ihr Eier?', $2)`,
		websiteID, at.UTC().Format(timeLayout))
	if err != nil {
		t.Fatalf("insert form message: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// retentionExists reports whether a row of the table with this id is left.
func retentionExists(t *testing.T, database *db.DB, table string, id int64) bool {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n == 1
}

// retentionRun stores a run that started at the time with the status, as an
// earlier process would have left it.
func retentionRun(t *testing.T, database *db.DB, started time.Time, status string) {
	t.Helper()
	finished := any(started.Add(time.Minute).UTC().Format(timeLayout))
	if status == "running" {
		finished = nil
	}
	if _, err := database.Write.Exec(`INSERT INTO retention_runs (started_at, finished_at, status) VALUES ($1, $2, $3)`,
		started.UTC().Format(timeLayout), finished, status); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}

// retentionRuns counts the runs recorded.
func retentionRuns(t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM retention_runs`).Scan(&n); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	return n
}

// retentionDeletion is one row of retention_deletions.
type retentionDeletion struct {
	deleted int
	err     string
}

// retentionDeletionOf reads the row of one run, website and kind.
func retentionDeletionOf(t *testing.T, database *db.DB, runID, websiteID int64, kind string) (retentionDeletion, bool) {
	t.Helper()
	var d retentionDeletion
	err := database.Read.QueryRow(`SELECT deleted, error FROM retention_deletions
		WHERE run_id = $1 AND website_id = $2 AND kind = $3`, runID, websiteID, kind).Scan(&d.deleted, &d.err)
	if err != nil {
		return d, false
	}
	return d, true
}

func TestTheJobDeletesOldLegacyFormMessages(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	b := website(t, database, "B")
	old := retentionFormMessage(t, database, a, now.AddDate(0, -13, 0))
	recent := retentionFormMessage(t, database, a, now.AddDate(0, -1, 0))
	oldB := retentionFormMessage(t, database, b, now.AddDate(0, -13, 0))
	if err := s.SetPeriods(ctx, b, "DE", map[string]int{RetentionFormSubmission: 24}); err != nil {
		t.Fatalf("SetPeriods: %v", err)
	}

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if retentionExists(t, database, "form_messages", old) {
		t.Error("the message of 13 months is still there; the default period is 12")
	}
	if !retentionExists(t, database, "form_messages", recent) {
		t.Error("the message of last month was deleted")
	}
	if !retentionExists(t, database, "form_messages", oldB) {
		t.Error("website B's message of 13 months was deleted; B keeps form submissions for 24")
	}

	var id int64
	var status, started string
	var finished *string
	var deleted int
	if err := database.Read.QueryRow(`SELECT id, status, started_at, finished_at, deleted FROM retention_runs`).
		Scan(&id, &status, &started, &finished, &deleted); err != nil {
		t.Fatalf("the run was not recorded: %v", err)
	}
	if status != "ok" || deleted != 1 || finished == nil {
		t.Errorf("run = %s, %d deleted, finished %v; want ok, 1, a finish time", status, deleted, finished)
	}
	if started != now.Format(timeLayout) {
		t.Errorf("started_at = %s, want %s", started, now.Format(timeLayout))
	}
	if rep == nil || rep.ID != id || rep.Status != "ok" || rep.Deleted != 1 {
		t.Errorf("report = %+v, want run %d, ok, 1 deleted", rep, id)
	}
	if d, ok := retentionDeletionOf(t, database, id, a, RetentionFormSubmission); !ok || d.deleted != 1 || d.err != "" {
		t.Errorf("A's form submissions in the run = %+v (%v), want 1 deleted and no error", d, ok)
	}
	if d, ok := retentionDeletionOf(t, database, id, b, RetentionFormSubmission); !ok || d.deleted != 0 {
		t.Errorf("B's form submissions in the run = %+v (%v), want a row with 0", d, ok)
	}

	last, err := s.LastRun(ctx, a)
	if err != nil || last == nil || last.ID != id {
		t.Fatalf("LastRun(A) = %+v, %v; want run %d", last, err, id)
	}
	for _, k := range last.Kinds {
		if k.WebsiteID != a {
			t.Errorf("LastRun(A) carries a row of website %d", k.WebsiteID)
		}
	}
	ok, err := s.LastSuccess(ctx)
	if err != nil || ok == nil || !ok.Equal(now) {
		t.Errorf("LastSuccess = %v, %v; want %v", ok, err, now)
	}
}

func TestTheGateRunsOnceADay(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, 11, 14, hour, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		name string
		now  time.Time
		runs []struct {
			ago    time.Duration
			status string
		}
		want bool
	}{
		{name: "no run yet", now: at(14), want: true},
		{name: "a successful run two hours ago", now: at(14), runs: []struct {
			ago    time.Duration
			status string
		}{{2 * time.Hour, "ok"}}, want: false},
		{name: "23 hours ago, in the afternoon", now: at(14), runs: []struct {
			ago    time.Duration
			status string
		}{{23 * time.Hour, "ok"}}, want: false},
		{name: "23 hours ago, at night", now: at(2), runs: []struct {
			ago    time.Duration
			status string
		}{{23 * time.Hour, "ok"}}, want: true},
		{name: "30 hours ago, in the afternoon", now: at(14), runs: []struct {
			ago    time.Duration
			status string
		}{{30 * time.Hour, "ok"}}, want: true},
		{name: "25 hours ago, in the afternoon", now: at(14), runs: []struct {
			ago    time.Duration
			status string
		}{{25 * time.Hour, "ok"}}, want: false},
		{name: "a failed run two hours ago", now: at(14), runs: []struct {
			ago    time.Duration
			status string
		}{{25 * time.Hour, "ok"}, {2 * time.Hour, "failed"}}, want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, database := retentionService(t)
			for _, r := range c.runs {
				retentionRun(t, database, c.now.Add(-r.ago), r.status)
			}
			before := retentionRuns(t, database)
			if err := s.RetentionTick(context.Background(), c.now); err != nil {
				t.Fatalf("RetentionTick: %v", err)
			}
			ran := retentionRuns(t, database) > before
			if ran != c.want {
				t.Errorf("ran = %v, want %v", ran, c.want)
			}
		})
	}
}

func TestPeriodsHaveDefaults(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	period := func() Period {
		t.Helper()
		list, err := s.Periods(ctx, ws, "DE")
		if err != nil {
			t.Fatalf("Periods: %v", err)
		}
		for _, p := range list {
			if p.Kind == RetentionFormSubmission {
				return p
			}
		}
		t.Fatalf("Periods has no form_submission: %+v", list)
		return Period{}
	}
	stored := func() int {
		var n int
		if err := database.Read.QueryRow(`SELECT COUNT(*) FROM retention_periods WHERE website_id = $1`, ws).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	p := period()
	if p.Months != 12 || p.Default != 12 || p.Floor != 1 || p.Step != 1 || p.Stored {
		t.Errorf("form_submission with nothing stored = %+v, want 12 months, default 12, floor 1, step 1, not stored", p)
	}
	if err := s.SetPeriods(ctx, ws, "DE", map[string]int{RetentionFormSubmission: 6}); err != nil {
		t.Fatalf("SetPeriods 6: %v", err)
	}
	if p := period(); p.Months != 6 || !p.Stored {
		t.Errorf("after 6 = %+v, want 6 months, stored", p)
	}
	if err := s.SetPeriods(ctx, ws, "DE", map[string]int{RetentionFormSubmission: 12}); err != nil {
		t.Fatalf("SetPeriods 12: %v", err)
	}
	if p := period(); p.Months != 12 || p.Stored {
		t.Errorf("after the default = %+v, want 12 months, not stored", p)
	}
	if n := stored(); n != 0 {
		t.Errorf("%d rows after setting the default, want none: the default reaches a website that never chose its own", n)
	}
	for _, months := range []int{0, -3, 601} {
		if err := s.SetPeriods(ctx, ws, "DE", map[string]int{RetentionFormSubmission: months}); !errors.Is(err, ErrBadPeriod) {
			t.Errorf("SetPeriods %d = %v, want ErrBadPeriod", months, err)
		}
	}
	if err := s.SetPeriods(ctx, ws, "DE", map[string]int{"logs": 6}); !errors.Is(err, ErrBadPeriod) {
		t.Errorf("SetPeriods of kind logs = %v, want ErrBadPeriod", err)
	}
}
