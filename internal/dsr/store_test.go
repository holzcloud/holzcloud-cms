package dsr

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

// TestAnAddressIsOneAddress is WR-02 of the 33.1 review: the e-mail field
// took any string, and the admin list made it a mailto: link — so
// "op@example.com?bcc=…" became a link that, clicked to answer the request,
// sent a copy to an address the stranger chose. An address is now one bare
// address, or the request is refused.
func TestAnAddressIsOneAddress(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")

	for _, good := range []string{"ada@example.com", "ada.lovelace+opt@mail.example.ch", "o'brien@example.ie", "jürg@beispiel.ch"} {
		if !ValidEmail(good) {
			t.Errorf("ValidEmail(%q) = false; want true", good)
		}
		if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Email: good}); err != nil {
			t.Errorf("Add(%q): %v", good, err)
		}
	}
	for _, bad := range []string{
		"op@example.com?bcc=thief@evil.example&subject=Re",
		"op@example.com&bcc=thief@evil.example",
		"a@example.com, b@example.com",
		"a@example.com;b@example.com",
		"Ada <ada@example.com>",
		"ada @example.com",
		"ada@example.com\r\nBcc: thief@evil.example",
		"ada%40example.com",
		"mailto:ada@example.com",
		"MAILTO:ada@example.com",
		"mailto:@example.com",
		"ada",
		"@example.com",
		"ada@",
		"a@b@example.com",
		`"a b"@example.com`,
		"ada@example.com#x",
		"ada@exa/mple.com",
	} {
		if ValidEmail(bad) {
			t.Errorf("ValidEmail(%q) = true; want false", bad)
		}
		if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada", Email: bad}); !errors.Is(err, ErrBadEmail) {
			t.Errorf("Add(%q) = %v; want ErrBadEmail", bad, err)
		}
	}
	// No address is not an address, and not a wrong one either: Add only
	// asks ValidEmail when there is one.
	if ValidEmail("") {
		t.Error(`ValidEmail("") = true; want false`)
	}
	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada"}); err != nil {
		t.Errorf("a request without an address: %v", err)
	}
}

// TestAddWithinHoldsTheLimitUnderConcurrency: the limit used to be a count
// through the read pool followed, later, by an insert through the write pool,
// and a burst read the same count many times over — 120 parallel posts stored
// 119 rows against a limit of 30. The count and the insert are now one
// statement on the single write connection.
func TestAddWithinHoldsTheLimitUnderConcurrency(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	other := website(t, database, "B")
	since := time.Now().Add(-time.Hour)

	const posts, limit = 120, 30
	var wg sync.WaitGroup
	var stored, refused atomic.Int64
	start := make(chan struct{})
	for i := 0; i < posts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.AddWithin(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada"}, since, limit)
			switch {
			case err == nil:
				stored.Add(1)
			case errors.Is(err, ErrLimit):
				refused.Add(1)
			default:
				t.Errorf("AddWithin: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if stored.Load() != limit || refused.Load() != posts-limit {
		t.Errorf("%d stored and %d refused; want %d and %d", stored.Load(), refused.Load(), limit, posts-limit)
	}
	n, err := s.CountSince(ctx, ws, KindOptOut, since)
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if n != limit {
		t.Errorf("%d rows; want exactly the limit, %d", n, limit)
	}

	// The limit is per website: the other one still takes its request.
	if _, err := s.AddWithin(ctx, Request{WebsiteID: other, Kind: KindOptOut, Name: "Bea"}, since, limit); err != nil {
		t.Errorf("another website was limited by the first one's requests: %v", err)
	}
	// Rows older than the window do not count against it.
	if _, err := s.AddWithin(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Cleo"}, time.Now().Add(time.Hour), limit); err != nil {
		t.Errorf("a window with no rows in it refused a request: %v", err)
	}
	// What Add refuses, AddWithin refuses the same way, before the limit.
	if _, err := s.AddWithin(ctx, Request{WebsiteID: ws, Kind: KindOptOut}, since, limit); !errors.Is(err, ErrEmpty) {
		t.Errorf("an empty request = %v; want ErrEmpty", err)
	}
}

// Check answers what Add would, without storing anything: the opt-out form
// asks it before it spends a network's budget on a request.
func TestCheckRefusesWhatAddRefuses(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	for _, r := range []Request{
		{WebsiteID: ws, Kind: "unbekannt", Name: "Ada"},
		{WebsiteID: ws, Kind: KindOptOut, Name: " "},
		{WebsiteID: ws, Kind: KindOptOut, Name: strings.Repeat("n", MaxName+1)},
		{WebsiteID: ws, Kind: KindOptOut, Email: "ada@example.com?bcc=x@example.com"},
		{WebsiteID: ws, Kind: KindOptOut, Name: " Ada ", Email: " ada@example.com "},
	} {
		_, want := s.Add(ctx, r)
		if got := Check(r); !errors.Is(got, want) || (got == nil) != (want == nil) {
			t.Errorf("Check(%+v) = %v; Add said %v", r, got, want)
		}
	}
	if n, err := s.CountSince(ctx, ws, KindOptOut, time.Time{}); err != nil || n != 1 {
		t.Errorf("%d requests stored, %v; want only the one Add stored", n, err)
	}
}

// A restored request's dates are the record, and every limit counts from
// them: the hourly ceiling of the form, the mail cap, and the 24 months that
// keep a website from being deleted (migration 00065). A request dated in the
// future would count in all three until that date — a manifest dated 2099
// closed the form for good and locked the website until 2101 (WR-02 of the
// second 33.1 review). So a restored date may not lie ahead of now by more
// than clock skew, and an answer may not come before its question.
func TestRestoreRefusesDatesThatCannotBe(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	now := time.Now()
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name    string
		created time.Time
		handled *time.Time
		want    error
	}{
		{"arrived in 2099", farFuture, nil, ErrFutureTime},
		{"arrived in an hour", now.Add(time.Hour), nil, ErrFutureTime},
		{"answered before it arrived", now.Add(-48 * time.Hour), at(-72 * time.Hour), ErrAnswerTime},
		{"answered in 2099", now.Add(-48 * time.Hour), &farFuture, ErrAnswerTime},
		{"answered in an hour", now.Add(-48 * time.Hour), at(time.Hour), ErrAnswerTime},
	} {
		_, err := s.Restore(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada", CreatedAt: tc.created, HandledAt: tc.handled})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: %v; want %v", tc.name, err, tc.want)
		}
	}
	if n, err := s.CountSince(ctx, ws, KindOptOut, time.Time{}); err != nil || n != 0 {
		t.Fatalf("%d requests stored, %v; want none", n, err)
	}

	// A clock a little ahead on the exporting server is not a lie.
	for _, tc := range []struct {
		name    string
		created time.Time
		handled *time.Time
	}{
		{"arrived a minute ahead", now.Add(time.Minute), nil},
		{"answered the moment it arrived", now.Add(-time.Hour), at(-time.Hour)},
		{"answered a minute ahead", now.Add(-time.Hour), at(time.Minute)},
	} {
		if _, err := s.Restore(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada", CreatedAt: tc.created, HandledAt: tc.handled}); err != nil {
			t.Errorf("%s: %v; want it restored", tc.name, err)
		}
	}
}

// The data requests of Phase 33.2 (D-01, D-09): access, correction and
// erasure, in the table the opt-outs are in. Every helper below starts with
// request, so plan 02's helpers in this package keep theirs apart.

// requestAgo stores a request through Add and moves its arrival back by d,
// so that a test can put it inside or outside a window.
func requestAgo(t *testing.T, s *Store, database *db.DB, r Request, d time.Duration) int64 {
	t.Helper()
	id := add(t, s, r)
	at := time.Now().Add(-d).UTC().Format(timeLayout)
	if _, err := database.Write.Exec(`UPDATE privacy_requests SET created_at = $1 WHERE id = $2`, at, id); err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
	return id
}

func TestADataRequestNeedsAnAddress(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")

	if _, err := s.Add(ctx, Request{WebsiteID: ws, Kind: KindAccess, Name: "Ada"}); !errors.Is(err, ErrNeedsEmail) {
		t.Errorf("an access request with a name and no address: %v; want ErrNeedsEmail", err)
	}
	if err := Check(Request{WebsiteID: ws, Kind: KindErasure, Name: "Ada", GPC: true}); !errors.Is(err, ErrNeedsEmail) {
		t.Errorf("Check of an erasure request without an address: %v; want ErrNeedsEmail", err)
	}
	// An opt-out still needs no address: a name or the signal is enough.
	if err := Check(Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada"}); err != nil {
		t.Errorf("an opt-out with a name only: %v; want it accepted", err)
	}

	id := add(t, s, Request{WebsiteID: ws, Kind: KindAccess, Locale: "de", Email: " ada@example.com "})
	got, err := s.Get(ctx, ws, id)
	if err != nil || got == nil {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if got.Kind != KindAccess || got.Email != "ada@example.com" {
		t.Errorf("stored %+v; want an access request for ada@example.com", got)
	}
	if got.Manner != MannerForm || got.Outcome != "" || got.Basis != "" {
		t.Errorf("manner %q, outcome %q, basis %q; want form, nothing, nothing", got.Manner, got.Outcome, got.Basis)
	}
	for _, kind := range DataKinds {
		if !IsDataKind(kind) {
			t.Errorf("IsDataKind(%q) = false", kind)
		}
	}
	if IsDataKind(KindOptOut) || IsDataKind("") {
		t.Error("an opt-out or nothing counts as a data request")
	}
}

// The hourly ceiling of a form counts the requests that form takes: the
// three kinds of the data request form together, and the opt-outs apart.
// One form flooded must not close the other.
func TestDataRequestsShareOneCeiling(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	for _, kind := range []string{KindAccess, KindAccess, KindErasure, KindErasure, KindCorrection} {
		requestAgo(t, s, database, Request{WebsiteID: ws, Kind: kind, Email: "ada@example.com"}, 10*time.Minute)
	}
	// Older than the window: not counted.
	requestAgo(t, s, database, Request{WebsiteID: ws, Kind: KindAccess, Email: "old@example.com"}, 2*time.Hour)

	hourAgo := time.Now().Add(-time.Hour)
	if _, err := s.AddWithin(ctx, Request{WebsiteID: ws, Kind: KindCorrection, Email: "bea@example.com"}, hourAgo, 5); !errors.Is(err, ErrLimit) {
		t.Errorf("a sixth data request under a ceiling of five: %v; want ErrLimit", err)
	}
	if n, err := s.CountSince(ctx, ws, KindCorrection, hourAgo); err != nil || n != 5 {
		t.Errorf("CountSince(correction) = %d, %v; want 5, the whole family", n, err)
	}
	if _, err := s.AddWithin(ctx, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Cleo"}, hourAgo, 5); err != nil {
		t.Errorf("an opt-out under the same ceiling: %v; want it stored, its family counts opt-outs only", err)
	}
	if n, err := s.CountSince(ctx, ws, KindOptOut, hourAgo); err != nil || n != 1 {
		t.Errorf("CountSince(optout) = %d, %v; want 1", n, err)
	}
}

func TestAnAnswerIsRecordedOnce(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	other := website(t, database, "B")
	id := add(t, s, Request{WebsiteID: ws, Kind: KindAccess, Email: "ada@example.com"})
	optout := add(t, s, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Ada"})
	foreign := add(t, s, Request{WebsiteID: other, Kind: KindAccess, Email: "ada@example.com"})

	if err := s.Answer(ctx, ws, id, "maybe", ""); !errors.Is(err, ErrBadOutcome) {
		t.Errorf("outcome maybe: %v; want ErrBadOutcome", err)
	}
	if err := s.Answer(ctx, ws, id, OutcomeRefused, strings.Repeat("b", MaxBasis+1)); !errors.Is(err, ErrTooLong) {
		t.Errorf("a basis one longer than allowed: %v; want ErrTooLong", err)
	}
	if err := s.Answer(ctx, ws, optout, OutcomeGranted, ""); !errors.Is(err, ErrBadKind) {
		t.Errorf("an answer to an opt-out: %v; want ErrBadKind", err)
	}
	if err := s.Answer(ctx, ws, foreign, OutcomeGranted, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("an id of another website: %v; want ErrNotFound", err)
	}
	if got, _ := s.Get(ctx, other, foreign); got == nil || got.Outcome != "" || got.HandledAt != nil {
		t.Errorf("the other website's request was changed: %+v", got)
	}

	if err := s.Answer(ctx, ws, id, OutcomeGranted, " copy sent "); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	first, err := s.Get(ctx, ws, id)
	if err != nil || first == nil {
		t.Fatalf("Get: %+v, %v", first, err)
	}
	if first.Outcome != OutcomeGranted || first.Basis != "copy sent" || first.HandledAt == nil {
		t.Fatalf("after the answer: outcome %q, basis %q, handled %v; want granted, copy sent, a time",
			first.Outcome, first.Basis, first.HandledAt)
	}

	if err := s.Answer(ctx, ws, id, OutcomeRefused, "changed my mind"); !errors.Is(err, ErrAnswered) {
		t.Errorf("a second answer: %v; want ErrAnswered", err)
	}
	again, _ := s.Get(ctx, ws, id)
	if again.Outcome != OutcomeGranted || again.Basis != "copy sent" || !again.HandledAt.Equal(*first.HandledAt) {
		t.Errorf("a second answer changed the record: %+v", again)
	}
}

// ListData lists the three kinds of data request of one website, newest
// first, and neither the opt-outs nor another website's requests.
func TestListDataListsTheDataRequests(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	other := website(t, database, "B")
	older := requestAgo(t, s, database, Request{WebsiteID: ws, Kind: KindErasure, Email: "ada@example.com"}, 2*time.Hour)
	newer := requestAgo(t, s, database, Request{WebsiteID: ws, Kind: KindAccess, Email: "bea@example.com"}, time.Hour)
	add(t, s, Request{WebsiteID: ws, Kind: KindOptOut, Name: "Cleo"})
	add(t, s, Request{WebsiteID: other, Kind: KindAccess, Email: "dora@example.com"})

	list, err := s.ListData(ctx, ws)
	if err != nil {
		t.Fatalf("ListData: %v", err)
	}
	if len(list) != 2 || list[0].ID != newer || list[1].ID != older {
		t.Errorf("ListData = %+v; want %d then %d", list, newer, older)
	}
}

// A request that came by letter is entered by the operator, with the day it
// arrived: that day, not the day it was typed in, starts the deadline.
func TestARequestByPostIsEntered(t *testing.T) {
	s, database := newTestStore(t)
	ctx := context.Background()
	ws := website(t, database, "A")
	arrived := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)

	id, err := s.AddReceived(ctx, Request{WebsiteID: ws, Kind: KindErasure, Manner: MannerPost,
		Name: "Ada", Email: "ada@example.com", CreatedAt: arrived})
	if err != nil {
		t.Fatalf("AddReceived: %v", err)
	}
	got, err := s.Get(ctx, ws, id)
	if err != nil || got == nil {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if !got.CreatedAt.Equal(arrived) || got.Manner != MannerPost || got.Kind != KindErasure || got.Outcome != "" {
		t.Errorf("stored %+v; want an open erasure request by post, arrived %v", got, arrived)
	}

	for _, tc := range []struct {
		name string
		r    Request
		want error
	}{
		{"by the form", Request{Kind: KindAccess, Manner: MannerForm, Email: "ada@example.com", CreatedAt: arrived}, ErrBadManner},
		{"no manner", Request{Kind: KindAccess, Email: "ada@example.com", CreatedAt: arrived}, ErrBadManner},
		{"by fax", Request{Kind: KindAccess, Manner: "fax", Email: "ada@example.com", CreatedAt: arrived}, ErrBadManner},
		{"tomorrow", Request{Kind: KindAccess, Manner: MannerPhone, Email: "ada@example.com", CreatedAt: time.Now().Add(24 * time.Hour)}, ErrFutureTime},
		{"no date", Request{Kind: KindAccess, Manner: MannerPhone, Email: "ada@example.com"}, ErrNoTime},
		{"an opt-out", Request{Kind: KindOptOut, Manner: MannerEmail, Name: "Ada", CreatedAt: arrived}, ErrBadKind},
		{"no address", Request{Kind: KindAccess, Manner: MannerEmail, Name: "Ada", CreatedAt: arrived}, ErrNeedsEmail},
		{"not an address", Request{Kind: KindAccess, Manner: MannerEmail, Email: "ada@example.com?bcc=x", CreatedAt: arrived}, ErrBadEmail},
	} {
		tc.r.WebsiteID = ws
		if _, err := s.AddReceived(ctx, tc.r); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v; want %v", tc.name, err, tc.want)
		}
	}
	if list, _ := s.ListData(ctx, ws); len(list) != 1 {
		t.Errorf("%d requests stored; want the one", len(list))
	}
}
