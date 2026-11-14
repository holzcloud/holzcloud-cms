package dsr

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// restoreSkew is how far ahead of this server's clock a restored date may
// lie: the exporting server's clock may have run a little ahead, and that
// is not a reason to lose the record.
const restoreSkew = 5 * time.Minute

// Request is one request a person made about their data.
type Request struct {
	ID        int64
	WebsiteID int64
	Kind      string
	// Locale is the language of the page the request was made on.
	Locale string
	Name   string
	Email  string
	Note   string
	// GPC records whether the browser sent the Global Privacy Control signal
	// with the request.
	GPC       bool
	CreatedAt time.Time
	// HandledAt is when the operator answered, nil while nobody has.
	HandledAt *time.Time
}

// Store handles SQL operations for requests. Every statement carries the
// website id, so no method can read, count or change another website's
// request.
type Store struct {
	DB *db.DB
}

// NewStore creates a request store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

const columns = `id, website_id, kind, locale, name, email, note, gpc, created_at, handled_at`

func scan(row interface{ Scan(...any) error }) (*Request, error) {
	var r Request
	var gpc int64
	var createdAt string
	var handledAt sql.NullString
	if err := row.Scan(&r.ID, &r.WebsiteID, &r.Kind, &r.Locale, &r.Name, &r.Email, &r.Note,
		&gpc, &createdAt, &handledAt); err != nil {
		return nil, err
	}
	r.GPC = gpc == 1
	r.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	if handledAt.Valid {
		if t, err := time.Parse(timeLayout, handledAt.String); err == nil {
			r.HandledAt = &t
		}
	}
	return &r, nil
}

// Add stores a request and returns its id.
//
// The values are trimmed first, and then checked the way the table checks
// them, so a caller gets a named error rather than a constraint message: an
// unknown kind is ErrBadKind, a value over its limit ErrTooLong, and a request
// with no name, no address and no GPC signal ErrEmpty.
func (s *Store) Add(ctx context.Context, r Request) (int64, error) {
	return s.add(ctx, r, time.Time{}, 0)
}

// AddWithin stores a request as Add does, unless the website already received
// limit requests of its kind since the given moment; then it stores nothing
// and returns ErrLimit.
//
// The count and the insert are one statement, INSERT … SELECT … WHERE count <
// limit, on the write pool, which has one connection. A count read first and
// an insert made after it leave a gap a burst of parallel requests walks
// through — every one of them reads the same count below the limit — and the
// limit is what stands between an anonymous form and the disk and the
// operator's inbox.
func (s *Store) AddWithin(ctx context.Context, r Request, since time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, ErrLimit
	}
	return s.add(ctx, r, since, limit)
}

// Check reports what Add would refuse a request for — ErrBadKind,
// ErrTooLong, ErrEmpty or ErrBadEmail — without storing anything, and nil
// when it would store it. The opt-out form asks it before it spends anything
// on a request, so that what was typed wrong costs the visitor nothing but
// the correction.
func Check(r Request) error {
	_, err := clean(r)
	return err
}

// clean trims a request's values and checks them the way the table checks
// them.
func clean(r Request) (Request, error) {
	if !kinds[r.Kind] {
		return r, ErrBadKind
	}
	r.Locale = strings.TrimSpace(r.Locale)
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
	r.Note = strings.TrimSpace(r.Note)
	// Characters and not bytes, because that is what SQLite's length() counts
	// on a TEXT value: a name of 200 umlauts is 400 bytes and fits.
	if utf8.RuneCountInString(r.Name) > MaxName ||
		utf8.RuneCountInString(r.Email) > MaxEmail ||
		utf8.RuneCountInString(r.Note) > MaxNote ||
		utf8.RuneCountInString(r.Locale) > maxLocale {
		return r, ErrTooLong
	}
	if r.Name == "" && r.Email == "" && !r.GPC {
		return r, ErrEmpty
	}
	if r.Email != "" && !ValidEmail(r.Email) {
		return r, ErrBadEmail
	}
	return r, nil
}

// add is Add with an optional limit; limit 0 means none.
func (s *Store) add(ctx context.Context, r Request, since time.Time, limit int) (int64, error) {
	r, err := clean(r)
	if err != nil {
		return 0, err
	}
	gpc := 0
	if r.GPC {
		gpc = 1
	}
	if limit == 0 {
		res, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc)
		if err != nil {
			return 0, fmt.Errorf("add request: %w", err)
		}
		return res.LastInsertId()
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc)
		 SELECT $1, $2, $3, $4, $5, $6, $7
		 WHERE (SELECT COUNT(*) FROM privacy_requests
		        WHERE website_id = $1 AND kind = $2 AND created_at >= $8) < $9`,
		r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc,
		since.UTC().Format(timeLayout), limit)
	if err != nil {
		return 0, fmt.Errorf("add request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("add request: %w", err)
	}
	if n == 0 {
		return 0, ErrLimit
	}
	return res.LastInsertId()
}

// List returns the requests of one kind on one website, newest first.
func (s *Store) List(ctx context.Context, websiteID int64, kind string) ([]Request, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM privacy_requests
		 WHERE website_id = $1 AND kind = $2
		 ORDER BY created_at DESC, id DESC`, websiteID, kind)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// ListAll returns every request of one website, of every kind, oldest
// first: the order an archive writes them in (internal/bundle).
func (s *Store) ListAll(ctx context.Context, websiteID int64) ([]Request, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM privacy_requests
		 WHERE website_id = $1
		 ORDER BY created_at, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// Restore stores a request that arrived elsewhere, with the moment it
// arrived and the moment it was answered: a website moved by export and
// import keeps its record (WR-06 of the 33.1 review), and the dates are the
// record.
//
// The checks are Add's, except one. An address is kept as it was received
// even when ValidEmail would refuse it today: a record is not corrected on
// the way, and the admin list links only an address that passes the check.
// A request with no arrival time is ErrNoTime.
//
// The dates have to be dates that can have happened: an arrival that lies
// ahead of now by more than restoreSkew is ErrFutureTime, and an answer that
// does, or that comes before the arrival, is ErrAnswerTime. Every limit counts from them — the
// form's hourly ceiling, the mail cap, and the 24 months that keep a website
// from being deleted (migration 00065) — so a request dated 2099 would close
// the form and lock the website until then, and this package deletes nothing
// that could undo it (WR-02 of the second 33.1 review). The manifest it comes
// from is a file anybody can edit, and a clock that runs ahead does the same
// without anybody meaning it.
func (s *Store) Restore(ctx context.Context, r Request) (int64, error) {
	if !kinds[r.Kind] {
		return 0, ErrBadKind
	}
	if r.CreatedAt.IsZero() {
		return 0, ErrNoTime
	}
	latest := time.Now().Add(restoreSkew)
	if r.CreatedAt.After(latest) {
		return 0, ErrFutureTime
	}
	if r.HandledAt != nil && (r.HandledAt.Before(r.CreatedAt) || r.HandledAt.After(latest)) {
		return 0, ErrAnswerTime
	}
	if utf8.RuneCountInString(r.Name) > MaxName ||
		utf8.RuneCountInString(r.Email) > MaxEmail ||
		utf8.RuneCountInString(r.Note) > MaxNote ||
		utf8.RuneCountInString(r.Locale) > maxLocale {
		return 0, ErrTooLong
	}
	if r.Name == "" && r.Email == "" && !r.GPC {
		return 0, ErrEmpty
	}
	gpc := 0
	if r.GPC {
		gpc = 1
	}
	var handled any
	if r.HandledAt != nil {
		handled = r.HandledAt.UTC().Format(timeLayout)
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc, created_at, handled_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc,
		r.CreatedAt.UTC().Format(timeLayout), handled)
	if err != nil {
		return 0, fmt.Errorf("restore request: %w", err)
	}
	return res.LastInsertId()
}

// Get returns one request of one website, or nil when there is none — also
// when the id belongs to another website, which is the same answer on purpose:
// the website is part of the lookup, not a comparison a caller has to
// remember.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Request, error) {
	r, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM privacy_requests WHERE id = $1 AND website_id = $2`, id, websiteID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return r, nil
}

// MarkHandled records that the operator answered a request.
//
// The first answer is the one that counts for the record: a second call
// leaves handled_at as it was. A request that does not exist on this website
// is ErrNotFound.
func (s *Store) MarkHandled(ctx context.Context, websiteID, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE privacy_requests
		 SET handled_at = COALESCE(handled_at, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		 WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("mark request handled: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark request handled: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountSince counts the requests of one kind a website received since a
// moment. The opt-out form asks it on every submission for its hourly limit,
// which is why it is a count in the database and not something a process
// remembers: a restart must not reset it.
func (s *Store) CountSince(ctx context.Context, websiteID int64, kind string, since time.Time) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM privacy_requests
		 WHERE website_id = $1 AND kind = $2 AND created_at >= $3`,
		websiteID, kind, since.UTC().Format(timeLayout)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count requests: %w", err)
	}
	return n, nil
}
