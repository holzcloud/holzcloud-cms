package activity_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*activity.Store, *db.DB) {
	t.Helper()
	// CRITICAL: file-backed SQLite via t.TempDir(). db.Open verifies WAL pragma
	// at db.go:42-52, which an in-memory database cannot satisfy.
	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return activity.NewStore(database), database
}

func ptrInt64(v int64) *int64 { return &v }

func TestLogWritesRow(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()

	// Seed parents so non-null user_id/website_id FKs are satisfied.
	uRes, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ('A', 'a@x.z', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	uid, _ := uRes.LastInsertId()
	wRes, err := database.Write.ExecContext(ctx,
		`INSERT INTO websites (name, active) VALUES ('S', 1)`)
	if err != nil {
		t.Fatalf("seed website: %v", err)
	}
	wid, _ := wRes.LastInsertId()

	store.Log(ctx, activity.Entry{
		UserID:     ptrInt64(uid),
		ActorEmail: "admin@example.com",
		Action:     activity.ActionPageCreate,
		EntityType: "page",
		EntityID:   42,
		WebsiteID:  ptrInt64(wid),
		Metadata:   map[string]any{"title": "Hello"},
	})

	rows, total, err := store.List(ctx, activity.Filter{}, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 row, got total=%d len=%d", total, len(rows))
	}
	e := rows[0]
	if e.Action != activity.ActionPageCreate {
		t.Errorf("Action = %q; want %q", e.Action, activity.ActionPageCreate)
	}
	if e.ActorEmail != "admin@example.com" {
		t.Errorf("ActorEmail = %q; want admin@example.com", e.ActorEmail)
	}
	if e.UserID == nil || *e.UserID != uid {
		t.Errorf("UserID = %v; want *%d", e.UserID, uid)
	}
	if e.EntityType != "page" {
		t.Errorf("EntityType = %q; want page", e.EntityType)
	}
	if e.EntityID != 42 {
		t.Errorf("EntityID = %d; want 42", e.EntityID)
	}
	if e.WebsiteID == nil || *e.WebsiteID != wid {
		t.Errorf("WebsiteID = %v; want *%d", e.WebsiteID, wid)
	}
	if e.Metadata["title"] != "Hello" {
		t.Errorf("Metadata title = %v; want Hello", e.Metadata["title"])
	}
}

func TestLogSwallowsError(t *testing.T) {
	store, _ := newTestStore(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	// Must not panic; Log signature is void by D-08 contract.
	store.Log(cancelled, activity.Entry{Action: activity.ActionAuthLoginFail})
}

func TestLogSanitizesMetadata(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()

	store.Log(ctx, activity.Entry{
		Action:   activity.ActionUserUpdate,
		Metadata: map[string]any{"password": "SECRET", "title": "Hello"},
	})

	var raw string
	if err := database.Read.QueryRowContext(ctx, `SELECT metadata FROM activity_log LIMIT 1`).Scan(&raw); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if strings.Contains(raw, "password") || strings.Contains(raw, "SECRET") {
		t.Errorf("metadata leaked password/SECRET: %s", raw)
	}
	if !strings.Contains(raw, "Hello") {
		t.Errorf("metadata missing allowed key: %s", raw)
	}
}

func TestListNewestFirst(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	// id is the tiebreaker for created_at ties (ORDER BY created_at DESC, id DESC),
	// so even rapid inserts within the same second sort newest-first by id.
	for _, a := range []string{"page.create", "page.update", "page.delete"} {
		store.Log(ctx, activity.Entry{Action: a})
	}
	rows, _, err := store.List(ctx, activity.Filter{}, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len = %d; want 3", len(rows))
	}
	if rows[0].Action != "page.delete" {
		t.Errorf("rows[0].Action = %q; want page.delete (newest)", rows[0].Action)
	}
	if rows[2].Action != "page.create" {
		t.Errorf("rows[2].Action = %q; want page.create (oldest)", rows[2].Action)
	}
}

func TestListPagination(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate})
	}
	p1, total, _ := store.List(ctx, activity.Filter{}, 2, 0)
	if total != 5 || len(p1) != 2 {
		t.Errorf("page 1: total=%d len=%d; want 5,2", total, len(p1))
	}
	p2, _, _ := store.List(ctx, activity.Filter{}, 2, 2)
	if len(p2) != 2 {
		t.Errorf("page 2: len=%d; want 2", len(p2))
	}
	p3, _, _ := store.List(ctx, activity.Filter{}, 2, 4)
	if len(p3) != 1 {
		t.Errorf("page 3: len=%d; want 1", len(p3))
	}
}

func TestListFilterByWebsiteID(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	// Seed two websites so FK constraints pass for non-null website_id.
	res1, err := store.DB.Write.Exec(`INSERT INTO websites (name, active) VALUES ('S1', 1)`)
	if err != nil {
		t.Fatalf("seed website 1: %v", err)
	}
	id1, _ := res1.LastInsertId()
	res2, err := store.DB.Write.Exec(`INSERT INTO websites (name, active) VALUES ('S2', 1)`)
	if err != nil {
		t.Fatalf("seed website 2: %v", err)
	}
	id2, _ := res2.LastInsertId()
	for _, w := range []int64{id1, id2, id1, id2} {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate, WebsiteID: ptrInt64(w)})
	}
	rows, total, _ := store.List(ctx, activity.Filter{WebsiteID: ptrInt64(id1)}, 50, 0)
	if total != 2 || len(rows) != 2 {
		t.Errorf("filter websiteID=%d: total=%d len=%d; want 2,2", id1, total, len(rows))
	}
}

func TestListFilterByActionPrefix(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for _, a := range []string{"page.create", "page.update", "user.create"} {
		store.Log(ctx, activity.Entry{Action: a})
	}
	prefix, _, _ := store.List(ctx, activity.Filter{Action: "page.*"}, 50, 0)
	if len(prefix) != 2 {
		t.Errorf("prefix page.*: len=%d; want 2", len(prefix))
	}
	exact, _, _ := store.List(ctx, activity.Filter{Action: "page.create"}, 50, 0)
	if len(exact) != 1 {
		t.Errorf("exact page.create: len=%d; want 1", len(exact))
	}
}

func TestListFilterByDateRange(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate})
	}
	// Backdate row 1 and row 3 to outside-range dates.
	if _, err := store.DB.Write.Exec(`UPDATE activity_log SET created_at = '2025-01-01T00:00:00Z' WHERE id = 1`); err != nil {
		t.Fatalf("backdate row 1: %v", err)
	}
	if _, err := store.DB.Write.Exec(`UPDATE activity_log SET created_at = '2027-01-01T00:00:00Z' WHERE id = 3`); err != nil {
		t.Fatalf("backdate row 3: %v", err)
	}
	// Set row 2 to a known in-range timestamp.
	if _, err := store.DB.Write.Exec(`UPDATE activity_log SET created_at = '2026-06-01T00:00:00Z' WHERE id = 2`); err != nil {
		t.Fatalf("set row 2: %v", err)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	rows, _, _ := store.List(ctx, activity.Filter{From: &from, To: &to}, 50, 0)
	if len(rows) != 1 {
		t.Errorf("date range filter: len=%d; want 1 (id=2 only)", len(rows))
	}
}

func TestListFilterByUserID(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()
	// Seed two users so non-null user_id FKs are satisfied.
	r1, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ('U1', 'u1@x.z', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("seed user 1: %v", err)
	}
	u1, _ := r1.LastInsertId()
	r2, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ('U2', 'u2@x.z', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("seed user 2: %v", err)
	}
	u2, _ := r2.LastInsertId()
	store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate, UserID: ptrInt64(u1)})
	store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate, UserID: ptrInt64(u2)})
	store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate, UserID: ptrInt64(u1)})
	rows, total, _ := store.List(ctx, activity.Filter{UserID: ptrInt64(u1)}, 50, 0)
	if total != 2 || len(rows) != 2 {
		t.Errorf("filter userID=%d: total=%d len=%d; want 2,2", u1, total, len(rows))
	}
}

func TestPurgeIsTransactionalAndExcludesSentinel(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate})
	}
	// Backdate them to before the cutoff.
	if _, err := database.Write.ExecContext(ctx, `UPDATE activity_log SET created_at = '2024-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	cutoff := time.Now().UTC()
	actor := activity.Entry{
		Action:     activity.ActionActivityPurge,
		ActorEmail: "admin@example.com",
		Metadata:   map[string]any{},
	}
	deleted, err := store.Purge(ctx, cutoff, actor)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if deleted != 3 {
		t.Errorf("rows deleted = %d; want 3", deleted)
	}
	// Verify exactly one row remains, and it is the purge sentinel.
	var n int
	if err := database.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_log`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows remaining = %d; want 1 (sentinel only)", n)
	}
	var act string
	if err := database.Read.QueryRowContext(ctx, `SELECT action FROM activity_log`).Scan(&act); err != nil {
		t.Fatalf("scan action: %v", err)
	}
	if act != activity.ActionActivityPurge {
		t.Errorf("remaining row action = %q; want %q", act, activity.ActionActivityPurge)
	}
}

func TestPurgeReturnsRowsDeleted(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()
	// 5 old rows + 2 new rows.
	for i := 0; i < 5; i++ {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageCreate})
	}
	if _, err := database.Write.ExecContext(ctx, `UPDATE activity_log SET created_at = '2024-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	for i := 0; i < 2; i++ {
		store.Log(ctx, activity.Entry{Action: activity.ActionPageUpdate})
	}
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	deleted, err := store.Purge(ctx, yesterday, activity.Entry{ActorEmail: "admin@example.com"})
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if deleted != 5 {
		t.Errorf("rows deleted = %d; want 5", deleted)
	}
}

func TestActorEmailSurvivesUserDelete(t *testing.T) {
	store, database := newTestStore(t)
	ctx := context.Background()
	res, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ('Adam', 'adam@example.com', 'x', 'admin')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	uid, _ := res.LastInsertId()

	store.Log(ctx, activity.Entry{
		UserID: ptrInt64(uid), ActorEmail: "adam@example.com", Action: activity.ActionAuthLoginSuccess,
	})
	if _, err := database.Write.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, uid); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	rows, _, err := store.List(ctx, activity.Filter{}, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d; want 1", len(rows))
	}
	e := rows[0]
	if e.UserID != nil {
		t.Errorf("UserID = %v; want nil after user delete", e.UserID)
	}
	if e.ActorEmail != "adam@example.com" {
		t.Errorf("ActorEmail = %q; want preserved snapshot", e.ActorEmail)
	}
}
