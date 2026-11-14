package dsr

// Retention: what a website no longer needs is deleted on its own (Phase
// 33.2, DSR-04).
//
// The operator sets a period per website and kind of record (D-04); a kind
// without a stored period has its default, so a default changed in a later
// release reaches every website that never chose its own. A job deletes what
// has expired once a day and records every run (D-05): when it started and
// finished, whether it succeeded, how much it deleted per website and kind,
// and what failed. A failing kind does not stop the others, the run is then
// marked failed, and the error goes back to the job runner so the journal
// says so as well. Nothing about a failure is only in a log line.
//
// The job ticks every hour and asks the last successful run whether it is
// due (D-09). The runner's ticker starts with the process, so a job that
// asked for 24 hours would never run on a service that is restarted more
// often than daily; the gate is stored in retention_runs and survives the
// restart.
//
// The log lines of the job carry counts, kinds and website ids — never an
// address, a name, or a storage key that contains one.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The kinds of record a website keeps for a period of its own. They are the
// Record.Kind values of the sources, and the kinds the plugins' privacy hook
// expires.
const (
	RetentionFormSubmission = "form_submission"
	RetentionRequest        = "request"
	RetentionOrder          = "order"
)

// The status of a run.
const (
	RunRunning = "running"
	RunOK      = "ok"
	RunFailed  = "failed"
)

// maxPeriodMonths is the longest period a website can set: fifty years, more
// than any period the research found, and the CHECK of retention_periods.
const maxPeriodMonths = 600

// The gate of the job (D-09). A run is due when the last successful one
// started at least dueAfter ago and the hour is a quiet one, and at the
// latest dueAt after it whatever the hour.
const (
	dueAfter = 23 * time.Hour
	dueAt    = 30 * time.Hour
	// The quiet hours, UTC: [quietFrom, quietTo).
	quietFrom = 1
	quietTo   = 5
)

var (
	// ErrBadPeriod is a period outside 1 to 600 months, one that is not a
	// whole step of its kind, or a kind that has no period.
	ErrBadPeriod = errors.New("dsr: a retention period is between 1 and 600 months")
	// ErrBelowFloor is a period shorter than the law allows for the kind.
	ErrBelowFloor = errors.New("dsr: the retention period is shorter than the law allows")
)

// PeriodError says which kind's period was refused, and why (ErrBadPeriod or
// ErrBelowFloor).
type PeriodError struct {
	Kind string
	Err  error
}

func (e *PeriodError) Error() string { return e.Kind + ": " + e.Err.Error() }
func (e *PeriodError) Unwrap() error { return e.Err }

// Period is how long a website keeps one kind of record.
type Period struct {
	Kind string
	// Months is the period in force: the stored one, or the default.
	Months int
	// Default is the period of a website that never chose one.
	Default int
	// Floor is the shortest period allowed, Step the unit a period is a
	// whole number of.
	Floor, Step int
	// Stored is true when the website chose its own period.
	Stored bool
}

// requestFloor is the shortest period for privacy requests, in months.
//
// CCPA regs § 7101 ask a business to keep the record of a consumer request
// and of its response for at least 24 months, and migration 00065 refuses to
// delete a website that holds a request younger than that, counted from the
// later of arrival and answer. The job counts the same way, so the job and
// the deletion lock agree on which request still has to be there.
const requestFloor = 24

// defaultPeriods are the kinds with a period, in the order the screen shows
// them, each with its default, floor and step. The order period follows the
// seller's country (D-06): its default is the law's period, and so is its
// floor — a website may keep orders longer, never shorter — in whole years.
func defaultPeriods(country string) []Period {
	orders := legal.OrderPeriodFor(country).Years * 12
	return []Period{
		// D-04: form submissions are kept for twelve months.
		{Kind: RetentionFormSubmission, Default: 12, Floor: 1, Step: 1},
		// D-04: requests for 24 months, never fewer.
		{Kind: RetentionRequest, Default: requestFloor, Floor: requestFloor, Step: 1},
		{Kind: RetentionOrder, Default: orders, Floor: orders, Step: 12},
	}
}

// Periods are the website's retention periods, one per kind.
func (s *Service) Periods(ctx context.Context, websiteID int64, country string) ([]Period, error) {
	stored, err := s.storedPeriods(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	list := defaultPeriods(country)
	for i := range list {
		p := &list[i]
		p.Months = p.Default
		if m, ok := stored[p.Kind]; ok {
			p.Stored = true
			p.Months = m
		}
		// A stored period below the floor is one the website chose before
		// its country changed, or before the law did: the law wins.
		if p.Months < p.Floor {
			p.Months = p.Floor
		}
	}
	return list, nil
}

// OrderYears is how many years the website keeps its orders: the later of
// the law of its country and the period the operator set. The law's period
// comes back with it, for the screen to name.
func (s *Service) OrderYears(ctx context.Context, websiteID int64, country string) (int, legal.OrderPeriod, error) {
	law := legal.OrderPeriodFor(country)
	stored, err := s.storedPeriods(ctx, websiteID)
	if err != nil {
		return 0, law, err
	}
	years := law.Years
	if m, ok := stored[RetentionOrder]; ok && m/12 > years {
		years = m / 12
	}
	return years, law, nil
}

func (s *Service) storedPeriods(ctx context.Context, websiteID int64) (map[string]int, error) {
	rows, err := s.db.Read.QueryContext(ctx,
		`SELECT kind, months FROM retention_periods WHERE website_id = $1`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("read retention periods: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var kind string
		var months int
		if err := rows.Scan(&kind, &months); err != nil {
			return nil, err
		}
		out[kind] = months
	}
	return out, rows.Err()
}

// SetPeriods stores the website's periods for the kinds given, in months.
//
// Every value is checked before anything is written: 1 to 600 months, a
// whole number of the kind's step, not below its floor. The first refusal
// is a *PeriodError naming the kind. A period equal to the default removes
// the stored row, so the website follows the default again — including one
// a later release changes.
func (s *Service) SetPeriods(ctx context.Context, websiteID int64, country string, months map[string]int) error {
	list, err := s.Periods(ctx, websiteID, country)
	if err != nil {
		return err
	}
	byKind := map[string]Period{}
	for _, p := range list {
		byKind[p.Kind] = p
	}
	for kind := range months {
		if _, ok := byKind[kind]; !ok {
			return &PeriodError{Kind: kind, Err: ErrBadPeriod}
		}
	}
	// In the screen's order, so the refusal is the same on every try.
	for _, p := range list {
		m, ok := months[p.Kind]
		if !ok {
			continue
		}
		if m < 1 || m > maxPeriodMonths || m%p.Step != 0 {
			return &PeriodError{Kind: p.Kind, Err: ErrBadPeriod}
		}
		if m < p.Floor {
			return &PeriodError{Kind: p.Kind, Err: ErrBelowFloor}
		}
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range list {
		m, ok := months[p.Kind]
		if !ok {
			continue
		}
		if m == p.Default {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM retention_periods WHERE website_id = $1 AND kind = $2`, websiteID, p.Kind); err != nil {
				return fmt.Errorf("reset retention period: %w", err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO retention_periods (website_id, kind, months) VALUES ($1, $2, $3)
			ON CONFLICT (website_id, kind) DO UPDATE SET months = excluded.months`, websiteID, p.Kind, m); err != nil {
			return fmt.Errorf("store retention period: %w", err)
		}
	}
	return tx.Commit()
}

// KindResult is what one run did to one kind of one website.
type KindResult struct {
	WebsiteID int64
	Kind      string
	Deleted   int
	// Err is the kind's error as stored, "" when the kind succeeded.
	Err string
}

// RunReport is one run of the job.
type RunReport struct {
	ID                int64
	Started, Finished time.Time
	Status            string
	Deleted           int
	Kinds             []KindResult
}

// RetentionTick is the hourly job: it runs the retention when a run is due.
//
// Due is: no successful run yet; a run since the last success that failed
// or never finished, so a failure is tried again at the next tick; the last
// success 23 hours ago or longer and the hour between 01:00 and 05:00 UTC,
// so the run settles into the night; or the last success 30 hours ago or
// longer, whatever the hour, so a server that sleeps at night still runs.
func (s *Service) RetentionTick(ctx context.Context, now time.Time) error {
	due, err := s.retentionDue(ctx, now)
	if err != nil {
		return err
	}
	if !due {
		return nil
	}
	_, err = s.RunRetention(ctx, now)
	return err
}

func (s *Service) retentionDue(ctx context.Context, now time.Time) (bool, error) {
	last, err := s.LastSuccess(ctx)
	if err != nil {
		return false, err
	}
	if last == nil {
		return true, nil
	}
	var unfinished int
	if err := s.db.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM retention_runs
		WHERE status IN ('failed', 'running') AND started_at > $1`, last.UTC().Format(timeLayout)).Scan(&unfinished); err != nil {
		return false, fmt.Errorf("read retention runs: %w", err)
	}
	if unfinished > 0 {
		return true, nil
	}
	age := now.Sub(*last)
	switch {
	case age >= dueAt:
		return true, nil
	case age >= dueAfter:
		h := now.UTC().Hour()
		return h >= quietFrom && h < quietTo, nil
	}
	return false, nil
}

// LastSuccess is when the last successful run started, nil when none did.
func (s *Service) LastSuccess(ctx context.Context) (*time.Time, error) {
	var started string
	err := s.db.Read.QueryRowContext(ctx, `SELECT started_at FROM retention_runs
		WHERE status = 'ok' ORDER BY started_at DESC LIMIT 1`).Scan(&started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read retention runs: %w", err)
	}
	t, err := time.Parse(timeLayout, started)
	if err != nil {
		return nil, fmt.Errorf("retention run started at %q: %w", started, err)
	}
	return &t, nil
}

// retentionWebsite is one website as the job walks it.
type retentionWebsite struct {
	id      int64
	country string
}

// RunRetention deletes what has expired on every website, now.
//
// One retention_runs row is written before anything is deleted and finished
// afterwards; one retention_deletions row per website and kind says how much
// went and what failed. Every website and kind runs even after an error.
// The returned error joins the kinds' errors, so the job runner logs that
// the job failed.
func (s *Service) RunRetention(ctx context.Context, now time.Time) (*RunReport, error) {
	now = now.UTC()
	clock := time.Now()
	// A run still 'running' was left by a process that stopped in the middle
	// of it: one job runs at a time, so none of this process's is. It is
	// closed as failed, so the screen does not show it running for ever and
	// the gate does not count it as a success.
	if _, err := s.db.Write.ExecContext(ctx, `UPDATE retention_runs
		SET status = 'failed', finished_at = $1, error = $2 WHERE status = 'running'`,
		now.Format(timeLayout), interruptedError); err != nil {
		return nil, fmt.Errorf("close interrupted retention runs: %w", err)
	}
	res, err := s.db.Write.ExecContext(ctx,
		`INSERT INTO retention_runs (started_at, status) VALUES ($1, 'running')`, now.Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("record retention run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	rep := &RunReport{ID: id, Started: now, Status: RunRunning}

	var errs []error
	websites, err := s.retentionWebsites(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	for _, ws := range websites {
		results, err := s.expireWebsite(ctx, ws, now)
		if err != nil {
			errs = append(errs, err)
		}
		for _, kr := range results {
			if err := s.recordDeletion(ctx, id, kr); err != nil {
				errs = append(errs, err)
			}
			rep.Kinds = append(rep.Kinds, kr)
			rep.Deleted += kr.Deleted
		}
	}
	// The kinds that belong to no website, after the websites: staff
	// accounts and the installation's own unsent mail (D-09). Their rows
	// carry no website.
	results, err := s.expireInstallation(ctx, now)
	if err != nil {
		errs = append(errs, err)
	}
	for _, kr := range results {
		if err := s.recordDeletion(ctx, id, kr); err != nil {
			errs = append(errs, err)
		}
		rep.Kinds = append(rep.Kinds, kr)
		rep.Deleted += kr.Deleted
	}
	// One line per website in the activity log, for whoever asks later what
	// the job deleted and when; nothing in it names a person.
	s.logRetention(ctx, id, rep.Kinds)

	rep.Status = RunOK
	if len(errs) > 0 {
		rep.Status = RunFailed
	}
	joined := errors.Join(errs...)
	rep.Finished = now.Add(time.Since(clock))
	text := ""
	if joined != nil {
		text = bounded(joined.Error(), 4000)
	}
	if _, err := s.db.Write.ExecContext(ctx, `UPDATE retention_runs
		SET finished_at = $1, status = $2, deleted = $3, error = $4 WHERE id = $5`,
		rep.Finished.Format(timeLayout), rep.Status, rep.Deleted, text, id); err != nil {
		joined = errors.Join(joined, fmt.Errorf("finish retention run: %w", err))
	}
	if err := s.pruneRuns(ctx, now); err != nil {
		joined = errors.Join(joined, err)
	}
	slog.Info("retention run", "run", id, "status", rep.Status, "deleted", rep.Deleted, "kinds", len(rep.Kinds))
	return rep, joined
}

// logRetention writes the summary of a run to the activity log: one entry
// per website that had something deleted, and one without a website for the
// installation's kinds. The metadata holds the run's id and the count per
// kind that deleted something — never an address or a name, because the log
// outlives what it describes. A run that deleted nothing writes nothing: a
// line a day saying so would bury the entries that matter.
func (s *Service) logRetention(ctx context.Context, runID int64, results []KindResult) {
	if s.activity == nil {
		return
	}
	var order []int64
	byWebsite := map[int64]map[string]any{}
	for _, kr := range results {
		if kr.Deleted == 0 {
			continue
		}
		meta, ok := byWebsite[kr.WebsiteID]
		if !ok {
			meta = map[string]any{"run": runID}
			byWebsite[kr.WebsiteID] = meta
			order = append(order, kr.WebsiteID)
		}
		meta[kr.Kind] = kr.Deleted
	}
	for _, id := range order {
		e := activity.Entry{Action: activity.ActionPrivacyRetention, Metadata: byWebsite[id]}
		if id == 0 {
			e.EntityType = "installation"
		} else {
			websiteID := id
			e.EntityType, e.EntityID, e.WebsiteID = "website", id, &websiteID
		}
		s.activity.Log(ctx, e)
	}
}

// interruptedError is the error of a run a process left unfinished.
const interruptedError = "interrupted: the process stopped before the run finished"

// runsKept is how long the record of a run is kept.
const runsKept = 90 * 24 * time.Hour

// pruneRuns deletes the runs that started more than 90 days before now,
// with their rows, except the latest successful one: the gate and the
// warning of a job that stopped count from it, however old it is.
func (s *Service) pruneRuns(ctx context.Context, now time.Time) error {
	if _, err := s.db.Write.ExecContext(ctx, `DELETE FROM retention_runs
		WHERE started_at < $1
		  AND id <> COALESCE((SELECT id FROM retention_runs WHERE status = 'ok'
		                      ORDER BY started_at DESC, id DESC LIMIT 1), 0)`,
		now.Add(-runsKept).UTC().Format(timeLayout)); err != nil {
		return fmt.Errorf("prune retention runs: %w", err)
	}
	return nil
}

// Holders are the plugins that keep data for the website, each saying
// whether it answers the privacy hook. One that does not is not deleted by
// the retention job, and the Retention screen names it.
func (s *Service) Holders(ctx context.Context, websiteID int64) ([]Holder, error) {
	_, holders, err := s.pluginSources(ctx, websiteID)
	return holders, err
}

// retentionWebsites are every website with the country its order period
// follows.
func (s *Service) retentionWebsites(ctx context.Context) ([]retentionWebsite, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT id, country FROM websites ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list websites: %w", err)
	}
	defer rows.Close()
	var out []retentionWebsite
	for rows.Next() {
		var ws retentionWebsite
		if err := rows.Scan(&ws.id, &ws.country); err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	return out, rows.Err()
}

// expireWebsite runs every kind of one website and returns one result per
// kind. The error joins the kinds' errors, each naming its website and kind.
func (s *Service) expireWebsite(ctx context.Context, ws retentionWebsite, now time.Time) ([]KindResult, error) {
	periods, err := s.Periods(ctx, ws.id, ws.country)
	if err != nil {
		return nil, fmt.Errorf("website %d: %w", ws.id, err)
	}
	// The plugins that answer the privacy hook here, asked once for every
	// kind. A plugin that keeps data and does not answer is not deleted by
	// the job; the screen names it.
	var covered []string
	_, holders, holdersErr := s.pluginSources(ctx, ws.id)
	for _, h := range holders {
		if h.Covered {
			covered = append(covered, h.PluginID)
		}
	}
	if holdersErr != nil {
		holdersErr = &expireError{cause: causeDatabase, detail: "list the plugins: " + holdersErr.Error()}
	}
	var results []KindResult
	var errs []error
	for _, p := range periods {
		kr, err := s.runKind(ws.id, p.Kind, func() (int, error) {
			return s.expireKind(ctx, ws, p, now, covered, holdersErr)
		})
		results, errs = append(results, kr), append(errs, err)
	}
	// Unsent mail has no period to choose (D-09): it goes 90 days after it
	// was queued, on every website alike.
	kr, err := s.runKind(ws.id, RetentionMailFailed, func() (int, error) {
		return s.expireFailedMail(ctx, ws.id, now)
	})
	results, errs = append(results, kr), append(errs, err)
	return results, errors.Join(errs...)
}

// recordDeletion writes one website's result of one kind; WebsiteID 0 is
// the installation's, stored with no website.
func (s *Service) recordDeletion(ctx context.Context, runID int64, kr KindResult) error {
	var website any
	if kr.WebsiteID != 0 {
		website = kr.WebsiteID
	}
	if _, err := s.db.Write.ExecContext(ctx, `INSERT INTO retention_deletions (run_id, website_id, kind, deleted, error)
		VALUES ($1, $2, $3, $4, $5)`, runID, website, kr.Kind, kr.Deleted, kr.Err); err != nil {
		return fmt.Errorf("record retention of website %d, %s: %w", kr.WebsiteID, kr.Kind, err)
	}
	return nil
}

// LastRun is the latest run that has a row of this website, with this
// website's rows only. Nil when no run has reached the website yet.
//
// Its Status is the website's: failed when one of its kinds failed,
// running while the run has not finished, ok otherwise — a run that failed
// on another website was a success here. Deleted is this website's count.
func (s *Service) LastRun(ctx context.Context, websiteID int64) (*RunReport, error) {
	var rep RunReport
	var started string
	var finished sql.NullString
	err := s.db.Read.QueryRowContext(ctx, `SELECT id, started_at, finished_at, status FROM retention_runs
		WHERE id = (SELECT MAX(run_id) FROM retention_deletions WHERE website_id = $1)`, websiteID).
		Scan(&rep.ID, &started, &finished, &rep.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the last retention run: %w", err)
	}
	rep.Started, _ = time.Parse(timeLayout, started)
	if finished.Valid {
		rep.Finished, _ = time.Parse(timeLayout, finished.String)
	}
	rows, err := s.db.Read.QueryContext(ctx, `SELECT kind, deleted, error FROM retention_deletions
		WHERE run_id = $1 AND website_id = $2 ORDER BY rowid`, rep.ID, websiteID)
	if err != nil {
		return nil, fmt.Errorf("read the last retention run: %w", err)
	}
	defer rows.Close()
	failed := false
	for rows.Next() {
		kr := KindResult{WebsiteID: websiteID}
		if err := rows.Scan(&kr.Kind, &kr.Deleted, &kr.Err); err != nil {
			return nil, err
		}
		rep.Deleted += kr.Deleted
		if kr.Err != "" {
			failed = true
		}
		rep.Kinds = append(rep.Kinds, kr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch {
	case rep.Status == RunRunning:
	case failed:
		rep.Status = RunFailed
	default:
		rep.Status = RunOK
	}
	return &rep, nil
}

// bounded cuts s to at most n bytes without splitting a character.
func bounded(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
