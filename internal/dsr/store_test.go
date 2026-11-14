package dsr

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

func newTestStore(t *testing.T) (*Store, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return NewStore(database), database
}

func website(t *testing.T, database *db.DB, name string) int64 {
	t.Helper()
	res, err := database.Write.Exec(`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func add(t *testing.T, s *Store, r Request) int64 {
	t.Helper()
	id, err := s.Add(context.Background(), r)
	if err != nil {
		t.Fatalf("Add(%+v): %v", r, err)
	}
	return id
}

func TestRequestsStayOnTheirWebsite(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	a := website(t, database, "A")
	b := website(t, database, "B")
	idA := add(t, s, Request{WebsiteID: a, Kind: KindOptOut, Name: "Ada"})

	list, err := s.List(ctx, b, KindOptOut)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("website B lists %d requests of website A", len(list))
	}
	got, err := s.Get(ctx, b, idA)
	if err != nil || got != nil {
		t.Errorf("Get(B, id of A) = %+v, %v; want nil, nil", got, err)
	}
	if err := s.MarkHandled(ctx, b, idA); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkHandled(B, id of A) = %v; want ErrNotFound", err)
	}
	n, err := s.CountSince(ctx, b, KindOptOut, time.Now().Add(-time.Hour))
	if err != nil || n != 0 {
		t.Errorf("CountSince(B) = %d, %v; want 0 — A's request counted on B", n, err)
	}

	mine, err := s.Get(ctx, a, idA)
	if err != nil || mine == nil || mine.HandledAt != nil {
		t.Fatalf("Get(A, id of A) = %+v, %v; want the row, unhandled", mine, err)
	}
	if n, _ := s.CountSince(ctx, a, KindOptOut, time.Now().Add(-time.Hour)); n != 1 {
		t.Errorf("CountSince(A) = %d; want 1", n)
	}
}

func TestAddRefusesWhatTheTableRefuses(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")

	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: "", Name: "Ada"}); !errors.Is(err, ErrBadKind) {
		t.Errorf("kind \"\": %v; want ErrBadKind", err)
	}
	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: "erase-everything", Name: "Ada"}); !errors.Is(err, ErrBadKind) {
		t.Errorf("unknown kind: %v; want ErrBadKind", err)
	}
	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "  ", Email: "", Note: "\n"}); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty without GPC: %v; want ErrEmpty", err)
	}
	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, GPC: true}); err != nil {
		t.Errorf("empty with GPC: %v; want it stored", err)
	}
	for _, tc := range []struct {
		name string
		r    Request
	}{
		{"name", Request{WebsiteID: ws, Kind: KindOptOut, Name: strings.Repeat("n", MaxName+1)}},
		{"e-mail", Request{WebsiteID: ws, Kind: KindOptOut, Email: strings.Repeat("e", MaxEmail+1)}},
		{"note", Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada", Note: strings.Repeat("x", MaxNote+1)}},
	} {
		if _, err := s.Add(ctx, tc.r); !errors.Is(err, ErrTooLong) {
			t.Errorf("a %s one longer than allowed: %v; want ErrTooLong", tc.name, err)
		}
	}
	// At the limit is allowed: the limit is a length, not a length minus one.
	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: strings.Repeat("ä", MaxName)}); err != nil {
		t.Errorf("a name of exactly %d characters (two bytes each): %v", MaxName, err)
	}

	id := add(t, s, Request{WebsiteID: ws, Kind: KindOptOut, Locale: "de", Name: "  Ada  ", Email: " ada@example.com\n", Note: "\tbitte "})
	got, err := s.Get(ctx, ws, id)
	if err != nil || got == nil {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if got.Name != "Ada" || got.Email != "ada@example.com" || got.Note != "bitte" || got.Locale != "de" || got.Kind != KindOptOut {
		t.Errorf("stored %+v; want the values trimmed", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at was not read back")
	}
}

func TestMarkHandledKeepsTheFirstTime(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	id := add(t, s, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada"})

	if err := s.MarkHandled(ctx, ws, id); err != nil {
		t.Fatalf("MarkHandled: %v", err)
	}
	got, _ := s.Get(ctx, ws, id)
	if got == nil || got.HandledAt == nil {
		t.Fatalf("handled_at not set: %+v", got)
	}

	// Put the first answer in the past, so a second call that overwrote it
	// could not hide behind a clock with one-second resolution.
	first := "2026-01-02T03:04:05Z"
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET handled_at = $1 WHERE id = $2`, first, id); err != nil {
		t.Fatalf("move handled_at: %v", err)
	}
	if err := s.MarkHandled(ctx, ws, id); err != nil {
		t.Fatalf("MarkHandled again: %v", err)
	}
	got, _ = s.Get(ctx, ws, id)
	if got == nil || got.HandledAt == nil || got.HandledAt.UTC().Format(time.RFC3339) != first {
		t.Errorf("handled_at after the second call = %v; want the first time %s kept", got.HandledAt, first)
	}

	if err := s.MarkHandled(ctx, ws, id+1000); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkHandled(missing) = %v; want ErrNotFound", err)
	}
}

// TestTheStoreCannotDelete holds D-14: this phase deletes no request. A US
// opt-out has to be kept for at least 24 months (CCPA regs § 7101), and when a
// row may go is Phase 33.2's retention to decide. A method that deletes,
// added here for convenience, would be the way that rule breaks without
// anybody deciding it — so its very existence fails the suite.
func TestTheStoreCannotDelete(t *testing.T) {
	typ := reflect.TypeOf(&Store{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		for _, verb := range []string{"Delete", "Remove", "Erase", "Purge", "Expire"} {
			if strings.HasPrefix(name, verb) {
				t.Errorf("dsr.Store has %s; deleting requests is Phase 33.2's retention, not this store's (D-14)", name)
			}
		}
	}
}

func TestListIsNewestFirstAndByKind(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	for _, row := range []struct{ kind, name, at string }{
		{"optout", "older", "2026-03-01T10:00:00Z"},
		{"access", "another kind", "2026-03-02T10:00:00Z"},
		{"optout", "newer", "2026-03-03T10:00:00Z"},
	} {
		if _, err := database.Write.Exec(
			`INSERT INTO privacy_requests (website_id, kind, name, created_at) VALUES ($1, $2, $3, $4)`,
			ws, row.kind, row.name, row.at); err != nil {
			t.Fatalf("insert %s: %v", row.name, err)
		}
	}
	list, err := s.List(ctx, ws, KindOptOut)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var names []string
	for _, r := range list {
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "newer,older" {
		t.Errorf("List = %v; want [newer older]", names)
	}
}
