package dsr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/user"
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

// installationHashing is cheap: the accounts of these tests never sign in.
var installationHashing = auth.Argon2Params{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// installationService is the service with the accounts, their sessions and
// the activity log wired as main.go wires them.
func installationService(t *testing.T) (*Service, *db.DB, *scs.SessionManager, *memstore.MemStore) {
	t.Helper()
	s, database := retentionService(t)
	store := memstore.New()
	sm := scs.New()
	sm.Store = store
	s.SetUsers(user.NewStore(database, installationHashing), sm)
	s.SetActivity(activity.NewStore(database))
	return s, database, sm, store
}

// installationAccount stores a staff account created at, last signed in at
// (nil for never), with an activity entry under its address.
func installationAccount(t *testing.T, database *db.DB, email, role string, created time.Time, login *time.Time, websites ...int64) int64 {
	t.Helper()
	var last any
	if login != nil {
		last = login.UTC().Format(timeLayout)
	}
	res, err := database.Write.Exec(`INSERT INTO users (email, password, role, name, created_at, last_login_at)
		VALUES ($1, 'x', $2, 'Staff', $3, $4)`, email, role, created.UTC().Format(timeLayout), last)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, _ := res.LastInsertId()
	for _, w := range websites {
		if _, err := database.Write.Exec(`INSERT INTO user_websites (user_id, website_id) VALUES ($1, $2)`, id, w); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Write.Exec(`INSERT INTO activity_log (user_id, actor_email, action, entity_type, entity_id)
		VALUES ($1, $2, 'page.update', 'page', 1)`, id, email); err != nil {
		t.Fatal(err)
	}
	return id
}

// installationSession signs the account in and returns the session token.
func installationSession(t *testing.T, sm *scs.SessionManager, userID int64) string {
	t.Helper()
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	sm.Put(ctx, auth.SessionKeyUserID, userID)
	token, _, err := sm.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// installationDeletionOf reads the installation's row of a run and kind.
func installationDeletionOf(t *testing.T, database *db.DB, runID int64, kind string) (retentionDeletion, bool) {
	t.Helper()
	var d retentionDeletion
	err := database.Read.QueryRow(`SELECT deleted, error FROM retention_deletions
		WHERE run_id = $1 AND website_id IS NULL AND kind = $2`, runID, kind).Scan(&d.deleted, &d.err)
	if err != nil {
		return d, false
	}
	return d, true
}

func TestInactiveAccountsAreDeletedButNeverAdministrators(t *testing.T) {
	s, database, sm, store := installationService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	longAgo := now.AddDate(0, -25, 0)
	lastMonth := now.AddDate(0, -1, 0)
	adminLogin := now.AddDate(0, -30, 0)

	gone := installationAccount(t, database, "lapsed@example.com", user.RoleEditor, now.AddDate(-3, 0, 0), &longAgo, a)
	never := installationAccount(t, database, "never@example.com", user.RoleEditor, longAgo, nil, a)
	active := installationAccount(t, database, "active@example.com", user.RoleEditor, now.AddDate(-3, 0, 0), &lastMonth, a)
	admin := installationAccount(t, database, "chef@example.com", user.RoleAdmin, now.AddDate(-4, 0, 0), &adminLogin)
	// A second, active administrator: without one, the store's last-admin
	// rule alone would keep the inactive one, and the test could not tell
	// it from the job's own rule.
	activeAdmin := installationAccount(t, database, "chefin@example.com", user.RoleAdmin, now.AddDate(-4, 0, 0), &lastMonth)
	token := installationSession(t, sm, gone)
	kept := installationSession(t, sm, active)

	n, err := s.InactiveAdmins(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("InactiveAdmins = %d, want 1", n)
	}

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	for _, id := range []int64{gone, never} {
		if retentionExists(t, database, "users", id) {
			t.Errorf("account %d without a sign-in for 25 months is still there", id)
		}
		var links int
		if err := database.Read.QueryRow(`SELECT COUNT(*) FROM user_websites WHERE user_id = $1`, id).Scan(&links); err != nil {
			t.Fatal(err)
		}
		if links != 0 {
			t.Errorf("account %d: %d website rows left", id, links)
		}
	}
	if !retentionExists(t, database, "users", active) {
		t.Error("an account that signed in last month was deleted")
	}
	if !retentionExists(t, database, "users", admin) {
		t.Fatal("an administrator was deleted automatically")
	}
	if _, found, _ := store.Find(token); found {
		t.Error("the session of a deleted account is still valid")
	}
	if _, found, _ := store.Find(kept); !found {
		t.Error("the session of an account that stays was ended")
	}

	// Accountability: the log keeps who did what (D-09).
	for _, email := range []string{"lapsed@example.com", "never@example.com"} {
		var entries int
		if err := database.Read.QueryRow(`SELECT COUNT(*) FROM activity_log WHERE actor_email = $1 AND action = 'page.update'`, email).Scan(&entries); err != nil {
			t.Fatal(err)
		}
		if entries != 1 {
			t.Errorf("the activity log lost the entry of %s", email)
		}
	}

	d, ok := installationDeletionOf(t, database, rep.ID, RetentionAccount)
	if !ok || d.deleted != 2 || d.err != "" {
		t.Errorf("the installation's account row = %+v (found %v), want 2 deleted", d, ok)
	}

	// The last administrator cannot be deleted by any path: the store the
	// job deletes through keeps refusing it.
	users := user.NewStore(database, installationHashing)
	if err := users.Delete(ctx, activeAdmin); err != nil {
		t.Fatalf("deleting one of two administrators: %v", err)
	}
	if err := users.Delete(ctx, admin); !errors.Is(err, user.ErrLastAdmin) {
		t.Errorf("deleting the last administrator: %v, want ErrLastAdmin", err)
	}
}

func TestTheAccountPeriodIsOneInstallationSetting(t *testing.T) {
	s, database, _, _ := installationService(t)
	ctx := context.Background()

	m, err := s.AccountMonths(ctx)
	if err != nil || m != 24 {
		t.Fatalf("AccountMonths = %d, %v; want the default 24", m, err)
	}
	if err := s.SetAccountMonths(ctx, 6); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := database.Read.QueryRow(`SELECT value FROM app_settings WHERE key = 'retention.account_months'`).Scan(&stored); err != nil || stored != "6" {
		t.Errorf("app_settings retention.account_months = %q (%v), want 6", stored, err)
	}
	if m, _ := s.AccountMonths(ctx); m != 6 {
		t.Errorf("AccountMonths after SetAccountMonths(6) = %d", m)
	}
	for _, bad := range []int{0, -1, 601} {
		if err := s.SetAccountMonths(ctx, bad); !errors.Is(err, ErrBadPeriod) {
			t.Errorf("SetAccountMonths(%d) = %v, want ErrBadPeriod", bad, err)
		}
	}
	if m, _ := s.AccountMonths(ctx); m != 6 {
		t.Errorf("a refused period changed the stored one to %d", m)
	}

	// The period applies: seven months without a sign-in is enough now.
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	login := now.AddDate(0, -7, 0)
	id := installationAccount(t, database, "seven@example.com", user.RoleEditor, now.AddDate(-2, 0, 0), &login)
	if _, err := s.RunRetention(ctx, now); err != nil {
		t.Fatal(err)
	}
	if retentionExists(t, database, "users", id) {
		t.Error("an account without a sign-in for 7 months stayed under a period of 6")
	}
}

// installationMail stores a row of mail_outbox; websiteID 0 is the
// installation's own mail.
func installationMail(t *testing.T, database *db.DB, websiteID int64, created time.Time, sent bool) int64 {
	t.Helper()
	var ws, sentAt any
	if websiteID != 0 {
		ws = websiteID
	}
	if sent {
		sentAt = created.Add(time.Minute).UTC().Format(timeLayout)
	}
	res, err := database.Write.Exec(`INSERT INTO mail_outbox (website_id, recipient, subject, body, created_at, sent_at, attempts)
		VALUES ($1, 'kundin@example.com', 'Bestätigung', 'Danke', $2, $3, 8)`, ws, created.UTC().Format(timeLayout), sentAt)
	if err != nil {
		t.Fatalf("insert mail: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// installationOutbox stores a row of the shop's outbox.
func installationOutbox(t *testing.T, database *db.DB, websiteID int64, created time.Time, status string) int64 {
	t.Helper()
	var sentAt any
	if status == "sent" {
		sentAt = created.Add(time.Minute).UTC().Format(timeLayout)
	}
	res, err := database.Write.Exec(`INSERT INTO outbox (website_id, kind, recipient, subject, body, status,
		next_attempt_at, created_at, sent_at)
		VALUES ($1, 'order_customer', 'kundin@example.com', 'Bestellung', 'Danke', $2, $3, $3, $4)`,
		websiteID, status, created.UTC().Format(timeLayout), sentAt)
	if err != nil {
		t.Fatalf("insert outbox: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestFailedMailIsDeletedAfter90Days(t *testing.T) {
	s, database, _, _ := installationService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	old := now.AddDate(0, 0, -91)
	young := now.AddDate(0, 0, -10)

	oldFailed := installationMail(t, database, a, old, false)
	youngFailed := installationMail(t, database, a, young, false)
	oldSent := installationMail(t, database, a, old, true)
	ownFailed := installationMail(t, database, 0, old, false)
	shopFailed := installationOutbox(t, database, a, old, "failed")
	shopSent := installationOutbox(t, database, a, old, "sent")

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	for _, c := range []struct {
		table string
		id    int64
		want  bool
		what  string
	}{
		{"mail_outbox", oldFailed, false, "unsent mail of the website queued 91 days ago"},
		{"mail_outbox", youngFailed, true, "unsent mail queued 10 days ago"},
		{"mail_outbox", oldSent, true, "sent mail (mail-prune's, not retention's)"},
		{"mail_outbox", ownFailed, false, "the installation's unsent mail queued 91 days ago"},
		{"outbox", shopFailed, false, "a failed shop mail queued 91 days ago"},
		{"outbox", shopSent, true, "a sent shop mail (the outbox prunes it)"},
	} {
		if got := retentionExists(t, database, c.table, c.id); got != c.want {
			t.Errorf("%s: there = %v, want %v", c.what, got, c.want)
		}
	}
	if d, ok := retentionDeletionOf(t, database, rep.ID, a, RetentionMailFailed); !ok || d.deleted != 2 {
		t.Errorf("A's mail_failed row = %+v (found %v), want 2", d, ok)
	}
	if d, ok := installationDeletionOf(t, database, rep.ID, RetentionMailFailed); !ok || d.deleted != 1 {
		t.Errorf("the installation's mail_failed row = %+v (found %v), want 1", d, ok)
	}
}

func TestARunLeavesOneActivityEntryPerWebsite(t *testing.T) {
	s, database, _, _ := installationService(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
	a := website(t, database, "A")
	b := website(t, database, "B")
	retentionFormMessage(t, database, a, now.AddDate(0, -13, 0))
	retentionFormMessage(t, database, a, now.AddDate(0, -14, 0))
	retentionFormMessage(t, database, b, now.AddDate(0, -1, 0))
	installationMail(t, database, 0, now.AddDate(0, 0, -91), false)
	login := now.AddDate(0, -25, 0)
	installationAccount(t, database, "lapsed@example.com", user.RoleEditor, now.AddDate(-3, 0, 0), &login)

	rep, err := s.RunRetention(ctx, now)
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}

	type entry struct {
		website  *int64
		actor    string
		metadata string
	}
	rows, err := database.Read.Query(`SELECT website_id, actor_email, metadata FROM activity_log WHERE action = $1`,
		activity.ActionPrivacyRetention)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.website, &e.actor, &e.metadata); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if len(entries) != 2 {
		t.Fatalf("%d privacy.retention entries, want 2 (A and the installation): %+v", len(entries), entries)
	}
	var sawA, sawInstallation bool
	for _, e := range entries {
		var meta map[string]any
		if err := json.Unmarshal([]byte(e.metadata), &meta); err != nil {
			t.Fatalf("metadata %q: %v", e.metadata, err)
		}
		if strings.Contains(e.metadata, "@") || e.actor != "" {
			t.Errorf("an entry carries an address: actor %q, metadata %s", e.actor, e.metadata)
		}
		if meta["run"] != float64(rep.ID) {
			t.Errorf("metadata %s does not name run %d", e.metadata, rep.ID)
		}
		switch {
		case e.website == nil:
			sawInstallation = true
			if meta[RetentionAccount] != float64(1) || meta[RetentionMailFailed] != float64(1) {
				t.Errorf("the installation's entry = %s, want account 1 and mail_failed 1", e.metadata)
			}
		case *e.website == a:
			sawA = true
			if meta[RetentionFormSubmission] != float64(2) {
				t.Errorf("A's entry = %s, want form_submission 2", e.metadata)
			}
			if _, ok := meta[RetentionOrder]; ok {
				t.Errorf("A's entry names a kind that deleted nothing: %s", e.metadata)
			}
		case *e.website == b:
			t.Errorf("B deleted nothing and has an entry: %s", e.metadata)
		}
	}
	if !sawA || !sawInstallation {
		t.Errorf("entries for A %v, for the installation %v; want both", sawA, sawInstallation)
	}
}
