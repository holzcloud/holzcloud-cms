package dsr

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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
	// Manner is how the request was made: MannerForm for what a form of the
	// website took, another manner for a request the operator entered.
	Manner string
	// Outcome is how a data request was answered, empty until it was.
	Outcome string
	// Basis is the operator's reason for the answer — required for a
	// refusal by the admin screen, and the "basis for the denial" of
	// § 7101(b).
	Basis string
}

// Store handles SQL operations for requests. Every statement carries the
// website id, so no method can read, count or change another website's
// request.
type Store struct {
	DB *db.DB
}

// NewStore creates a request store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

const columns = `id, website_id, kind, locale, name, email, note, gpc, created_at, handled_at, manner, outcome, basis`

func scan(row interface{ Scan(...any) error }) (*Request, error) {
	var r Request
	var gpc int64
	var createdAt string
	var handledAt sql.NullString
	if err := row.Scan(&r.ID, &r.WebsiteID, &r.Kind, &r.Locale, &r.Name, &r.Email, &r.Note,
		&gpc, &createdAt, &handledAt, &r.Manner, &r.Outcome, &r.Basis); err != nil {
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
// limit requests of its family since the given moment; then it stores nothing
// and returns ErrLimit. The family is the kinds one form takes: an opt-out
// counts the opt-outs, a data request all three kinds of data request.
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
// ErrBadManner, ErrTooLong, ErrEmpty, ErrNeedsEmail or ErrBadEmail — without
// storing anything, and nil when it would store it. The opt-out form asks it before it spends anything
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
	if r.Manner == "" {
		r.Manner = MannerForm
	}
	if !manners[r.Manner] {
		return r, ErrBadManner
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
	// A data request is answered at its address, and the address is what
	// the operator finds the person's data by. An opt-out needs none: a
	// name, or the browser's signal, is a request in its own right.
	if IsDataKind(r.Kind) && r.Email == "" {
		return r, ErrNeedsEmail
	}
	if r.Email != "" && !ValidEmail(r.Email) {
		return r, ErrBadEmail
	}
	return r, nil
}

// kindIn is "kind IN (...)" for the family of a kind, with its placeholders
// numbered from first, and the values that go with them.
func kindIn(kind string, first int) (string, []any) {
	fam := family(kind)
	marks := make([]string, len(fam))
	args := make([]any, len(fam))
	for i, k := range fam {
		marks[i] = "$" + strconv.Itoa(first+i)
		args[i] = k
	}
	return "kind IN (" + strings.Join(marks, ", ") + ")", args
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
			`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc, manner)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc, r.Manner)
		if err != nil {
			return 0, fmt.Errorf("add request: %w", err)
		}
		return res.LastInsertId()
	}
	in, fam := kindIn(r.Kind, 11)
	args := append([]any{r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc, r.Manner,
		since.UTC().Format(timeLayout), limit}, fam...)
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc, manner)
		 SELECT $1, $2, $3, $4, $5, $6, $7, $8
		 WHERE (SELECT COUNT(*) FROM privacy_requests
		        WHERE website_id = $1 AND `+in+` AND created_at >= $9) < $10`,
		args...)
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
//
// How the request was made and answered comes along when the archive carries
// it: an empty manner is the form's, and a manner or an outcome the record
// does not know is ErrBadManner or ErrBadOutcome.
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
	if r.Manner == "" {
		r.Manner = MannerForm
	}
	if !manners[r.Manner] {
		return 0, ErrBadManner
	}
	if r.Outcome != "" && !outcomes[r.Outcome] {
		return 0, ErrBadOutcome
	}
	if utf8.RuneCountInString(r.Basis) > MaxBasis {
		return 0, ErrTooLong
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
		`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, gpc, created_at, handled_at,
		                               manner, outcome, basis)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, gpc,
		r.CreatedAt.UTC().Format(timeLayout), handled, r.Manner, r.Outcome, r.Basis)
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

// CountSince counts the requests of a kind's family (AddWithin) a website
// received since a moment. The forms ask it after every stored request for
// their hourly mail cap, which is why it is a count in the database and not
// something a process remembers: a restart must not reset it.
func (s *Store) CountSince(ctx context.Context, websiteID int64, kind string, since time.Time) (int, error) {
	var n int
	in, fam := kindIn(kind, 3)
	args := append([]any{websiteID, since.UTC().Format(timeLayout)}, fam...)
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM privacy_requests
		 WHERE website_id = $1 AND `+in+` AND created_at >= $2`,
		args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count requests: %w", err)
	}
	return n, nil
}

// ListData returns the data requests of one website — access, correction and
// erasure, not the opt-outs — newest first.
func (s *Store) ListData(ctx context.Context, websiteID int64) ([]Request, error) {
	in, fam := kindIn(KindAccess, 2)
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM privacy_requests
		 WHERE website_id = $1 AND `+in+`
		 ORDER BY created_at DESC, id DESC`, append([]any{websiteID}, fam...)...)
	if err != nil {
		return nil, fmt.Errorf("list data requests: %w", err)
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

// Answer records how a data request was answered: the outcome, the basis the
// operator gives for it, and — when the request has no answer date yet — now
// as that date.
//
// Once. The answer is the record § 7101 asks for, and a record that can be
// rewritten afterwards records nothing; a second answer is ErrAnswered and
// changes nothing. The check and the write are one statement, so two answers
// sent at the same moment cannot both pass it.
//
// An outcome other than granted, partly or refused is ErrBadOutcome, a basis
// longer than MaxBasis ErrTooLong. An opt-out is ErrBadKind — it is marked
// handled (MarkHandled), not answered — and a request that is not on this
// website is ErrNotFound.
func (s *Store) Answer(ctx context.Context, websiteID, id int64, outcome, basis string) error {
	if !outcomes[outcome] {
		return ErrBadOutcome
	}
	basis = strings.TrimSpace(basis)
	if utf8.RuneCountInString(basis) > MaxBasis {
		return ErrTooLong
	}
	in, fam := kindIn(KindAccess, 5)
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE privacy_requests
		 SET outcome = $3, basis = $4,
		     handled_at = COALESCE(handled_at, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		 WHERE id = $1 AND website_id = $2 AND outcome = '' AND `+in,
		append([]any{id, websiteID, outcome, basis}, fam...)...)
	if err != nil {
		return fmt.Errorf("answer request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("answer request: %w", err)
	}
	if n == 1 {
		return nil
	}
	// Nothing changed: say why.
	r, err := s.Get(ctx, websiteID, id)
	switch {
	case err != nil:
		return err
	case r == nil:
		return ErrNotFound
	case !IsDataKind(r.Kind):
		return ErrBadKind
	default:
		return ErrAnswered
	}
}

// AddReceived stores a data request that did not come through the website's
// form — it arrived by e-mail, letter or telephone, and the operator enters
// it — and returns its id.
//
// The arrival time is the operator's word here and not this server's clock,
// because the day the letter came is the day the deadline starts (due.go), and
// the request may be typed in days later. Restore's date checks apply: no
// time is ErrNoTime, a time ahead of now by more than restoreSkew
// ErrFutureTime — a request dated in the future would hold the website's
// deletion lock of migration 00065 until then, and this package deletes
// nothing that could undo it.
//
// Only a data kind (ErrBadKind otherwise), only a manner other than the form
// (ErrBadManner otherwise: the form writes its own requests), and the values
// are checked as Add checks them. The request is stored unanswered; the
// answer is Answer's.
func (s *Store) AddReceived(ctx context.Context, r Request) (int64, error) {
	if !IsDataKind(r.Kind) {
		return 0, ErrBadKind
	}
	if r.Manner == MannerForm || !manners[r.Manner] {
		return 0, ErrBadManner
	}
	if r.CreatedAt.IsZero() {
		return 0, ErrNoTime
	}
	if r.CreatedAt.After(time.Now().Add(restoreSkew)) {
		return 0, ErrFutureTime
	}
	r, err := clean(r)
	if err != nil {
		return 0, err
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO privacy_requests (website_id, kind, locale, name, email, note, manner, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		r.WebsiteID, r.Kind, r.Locale, r.Name, r.Email, r.Note, r.Manner,
		r.CreatedAt.UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("add received request: %w", err)
	}
	return res.LastInsertId()
}
