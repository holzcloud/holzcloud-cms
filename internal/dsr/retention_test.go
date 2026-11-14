package dsr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

// retentionBreak makes one kind of one website fail in every run.
func retentionBreak(s *Service, websiteID int64, kind string) {
	s.failKind = func(id int64, k string) error {
		if id == websiteID && k == kind {
			return &expireError{cause: causeDatabase, detail: "delete privacy requests: the test broke this kind"}
		}
		return nil
	}
}

func TestAFailedKindDoesNotStopTheOthers(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	b := website(t, database, "B")
	msgA := retentionFormMessage(t, database, a, now.AddDate(-2, 0, 0))
	reqA := expiretestRequest(t, database, a, KindAccess, now.AddDate(-3, 0, 0), time.Time{})
	// Orders run after requests: the kind behind the broken one.
	orderA := expiretestOrder(t, database, a, "2010-0001", "2010-03-01T10:00:00Z", "")
	msgB := retentionFormMessage(t, database, b, now.AddDate(-2, 0, 0))
	reqB := expiretestRequest(t, database, b, KindAccess, now.AddDate(-3, 0, 0), time.Time{})
	retentionBreak(s, a, RetentionRequest)

	rep, err := s.RunRetention(ctx, now)
	if err == nil {
		t.Fatal("RunRetention returned no error while a kind failed")
	}
	if rep.Status != RunFailed {
		t.Errorf("report status %q, want failed", rep.Status)
	}
	var status, text string
	if err := database.Read.QueryRow(`SELECT status, error FROM retention_runs WHERE id = $1`, rep.ID).Scan(&status, &text); err != nil {
		t.Fatal(err)
	}
	if status != RunFailed || !strings.Contains(text, "the test broke this kind") {
		t.Errorf("run row = %s, %q; want failed with the kind's error", status, text)
	}
	if d, ok := retentionDeletionOf(t, database, rep.ID, a, RetentionRequest); !ok || d.err == "" {
		t.Errorf("A's request row = %+v (%v), want its error", d, ok)
	}
	if !retentionExists(t, database, "privacy_requests", reqA) {
		t.Error("the broken kind deleted anyway")
	}
	if retentionExists(t, database, "form_messages", msgA) {
		t.Error("A's form messages were not deleted after A's requests failed")
	}
	if o, _, _ := expiretestLeft(t, database, orderA); o != 0 {
		t.Error("A's orders were not run after A's requests failed")
	}
	if retentionExists(t, database, "form_messages", msgB) || retentionExists(t, database, "privacy_requests", reqB) {
		t.Error("website B was not run after A failed")
	}

	before := retentionRuns(t, database)
	if err := s.RetentionTick(ctx, now.Add(time.Hour)); err == nil {
		t.Error("the next tick ran and did not report the kind that still fails")
	}
	if retentionRuns(t, database) != before+1 {
		t.Error("the next tick did not try again after the failure")
	}
}

func TestAnInterruptedRunIsClosed(t *testing.T) {
	s, database := retentionService(t)
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	retentionRun(t, database, now.Add(-3*time.Hour), RunRunning)

	rep, err := s.RunRetention(context.Background(), now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	var status, text string
	var finished *string
	if err := database.Read.QueryRow(`SELECT status, error, finished_at FROM retention_runs WHERE id <> $1`, rep.ID).
		Scan(&status, &text, &finished); err != nil {
		t.Fatal(err)
	}
	if status != RunFailed || !strings.Contains(text, "interrupted") || finished == nil {
		t.Errorf("the earlier run = %s, %q, finished %v; want failed, interrupted, a finish time", status, text, finished)
	}
}

func TestOldRunsArePruned(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	ws := website(t, database, "A")
	days := func(n int) time.Time { return now.AddDate(0, 0, -n) }
	retentionRun(t, database, days(120), RunOK)
	retentionRun(t, database, days(100), RunOK)
	retentionRun(t, database, days(95), RunFailed)
	retentionRun(t, database, days(10), RunFailed)
	if _, err := database.Write.Exec(`INSERT INTO retention_deletions (run_id, website_id, kind, deleted)
		SELECT id, $1, 'form_submission', 1 FROM retention_runs`, ws); err != nil {
		t.Fatal(err)
	}
	// This run fails, so the latest success stays the one of 100 days ago.
	retentionBreak(s, ws, RetentionRequest)
	if _, err := s.RunRetention(ctx, now); err == nil {
		t.Fatal("the broken run reported no error")
	}

	left := map[string]string{}
	rows, err := database.Read.Query(`SELECT started_at, status FROM retention_runs`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var started, status string
		if err := rows.Scan(&started, &status); err != nil {
			t.Fatal(err)
		}
		left[started] = status
	}
	at := func(d time.Time) string { return d.Format(timeLayout) }
	if _, ok := left[at(days(120))]; ok {
		t.Error("a run of 120 days is still there")
	}
	if _, ok := left[at(days(95))]; ok {
		t.Error("a failed run of 95 days is still there")
	}
	if left[at(days(100))] != RunOK {
		t.Error("the latest successful run was pruned; it is what the gate and the warning count from")
	}
	if left[at(days(10))] != RunFailed {
		t.Error("a run of 10 days was pruned")
	}
	var orphans int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM retention_deletions
		WHERE run_id NOT IN (SELECT id FROM retention_runs)`).Scan(&orphans); err != nil || orphans != 0 {
		t.Errorf("%d rows of pruned runs left (%v)", orphans, err)
	}
}

func TestTheJobLogsNoPersonalData(t *testing.T) {
	s, database := retentionService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	ws := website(t, database, "A")
	const address, name = "zebra.quokka@example.org", "Quentin Zebrafish"
	for _, at := range []time.Time{now.AddDate(-2, 0, 0), now.AddDate(0, -1, 0)} {
		if _, err := database.Write.Exec(`INSERT INTO form_messages (website_id, name, email, subject, body, created_at)
			VALUES ($1, $2, $3, 'Hallo', 'Text', $4)`, ws, name, address, at.Format(timeLayout)); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Write.Exec(`INSERT INTO privacy_requests (website_id, kind, name, email, created_at)
			VALUES ($1, 'access', $2, $3, $4)`, ws, name, address, at.AddDate(-1, 0, 0).Format(timeLayout)); err != nil {
			t.Fatal(err)
		}
	}
	expiretestOrder(t, database, ws, "2010-0001", "2010-03-01T10:00:00Z", "")
	if _, err := database.Write.Exec(`UPDATE orders SET email = $1, name = $2`, address, name); err != nil {
		t.Fatal(err)
	}
	retentionBreak(s, ws, RetentionOrder)

	var buf strings.Builder
	was := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(was) })

	rep, err := s.RunRetention(ctx, now)
	if rep == nil || rep.Deleted == 0 {
		t.Fatalf("the run deleted nothing (%+v, %v); the test proves nothing", rep, err)
	}
	var stored strings.Builder
	rows, qerr := database.Read.Query(`SELECT error FROM retention_runs UNION ALL SELECT error FROM retention_deletions`)
	if qerr != nil {
		t.Fatal(qerr)
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		rows.Scan(&e)
		stored.WriteString(e)
	}
	if !strings.Contains(buf.String(), "retention run") {
		t.Fatalf("no log line of the run was captured:\n%s", buf.String())
	}
	for where, text := range map[string]string{"log": buf.String(), "returned error": fmt.Sprint(err), "stored errors": stored.String()} {
		for _, secret := range []string{address, "zebra", name, "Zebrafish"} {
			if strings.Contains(strings.ToLower(text), strings.ToLower(secret)) {
				t.Errorf("the %s carries %q:\n%s", where, secret, text)
			}
		}
	}
}

func TestFailuresAreNamedByTheirCause(t *testing.T) {
	for text, want := range map[string]string{
		"bound: plugin kontaktformular: more to expire than one run may delete": FailureBound,
		"file: media 3 of plugin kontaktformular stayed on the disk":            FailureFile,
		"plugin: plugin bestellung: the plugin gave no answer":                  FailurePlugin,
		"database: delete orders: database is locked":                           FailureDatabase,
		"plugin: plugin a: the call failed\nfile: media 4 of plugin a stayed":   FailurePlugin,
		"something nobody classified":                                           FailureDatabase,
		"":                                                                      "",
	} {
		if got := FailureOf(text); got != want {
			t.Errorf("FailureOf(%q) = %q, want %q", text, got, want)
		}
	}
}
