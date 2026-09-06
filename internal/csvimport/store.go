// Package csvimport holds an uploaded table between the upload screen and the
// report screen.
//
// The CSV import runs over four screens, and a browser cannot carry a file
// across them: the value of an <input type="file"> is not settable from markup,
// and a submitted form loads a new document, so a file field rendered on the
// second screen arrives empty. Screen 1 therefore puts the raw bytes here and
// screens 2, 3 and 4 carry nothing but a token. What is stored is the upload as
// it arrived — never a parsed table, and never anything in the session row,
// which lives in the same SQLite file and is read and written on every single
// request of that account.
//
// The row is written once and never updated. That is not a detail: it is what
// makes the dry run trustworthy, because the dry run and the write read the
// same bytes, so the report cannot describe a different file than the one that
// gets written.
//
// What this package deliberately is not: a parser. Reading a table, bounding it
// and refusing a hostile one is internal/csv, which imports nothing outside the
// standard library so its checklist of defences stays provable without db.Open.
// The counter-evidence is worth naming, so the split is read as a decision and
// not as habit: internal/shop puts its store and its pure price calculation in
// one package and is none the worse for it. The purity is bought here on
// purpose, and this package is where the database goes instead.
//
// One thing to expect while reading the SQL below: the column names are German
// and the Go names beside them are not. 00049 is a released migration that has
// already run on a database, so it is not edited; the columns turn in the
// phase that renames every column in this tree at once, behind a version jump.
// Until then, modus is Mode, kollision is Collision, dateiname is Filename,
// daten is Data and erstellt_am is CreatedAt. The values 'neu', 'bestehend',
// 'uebergehen' and 'aktualisieren' are pinned by that migration's CHECK
// constraints and are data, not identifiers — translating one of them here
// would write a row SQLite refuses.
package csvimport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// ErrExpired means no staged upload answers to this token: it was never there,
// or the sweep has already taken it. The screen that answers this one says
// "this upload has expired, please start again" — the operator did nothing
// wrong, and a 404 would say they did.
//
// ErrForeign means the upload exists and belongs to somebody else. The screen
// that answers this one refuses.
//
// The two are deliberately distinguishable, and that is where this departs from
// user.ErrTokenInvalid, whose whole point is that its three reasons are *not*
// told apart. The reason for departing: those three reasons all describe a link
// the holder was given, so telling them apart tells a stranger whether a token
// existed and buys nothing. Here the two reasons describe two different people.
// Merging them would answer an operator whose own upload was swept with the
// same refusal a stranger gets, which reads as an accusation for something that
// was the sweep's doing. The cost is that a correct guess of a foreign token is
// distinguishable from a wrong one; that is accepted, because the token is 128
// bits from crypto/rand and reaching the oracle means already holding the
// answer.
var (
	ErrExpired = errors.New("csvimport: no staged upload for this token")
	ErrForeign = errors.New("csvimport: staged upload belongs to another user")
)

// Upload is one staged upload.
type Upload struct {
	ID     int64
	UserID int64
	// WebsiteID is 0 for an import that will create the website itself, which
	// is the one case where there is no website yet.
	WebsiteID   int64
	WebsiteName string
	// Mode is "neu" or "bestehend", Collision "uebergehen" or "aktualisieren".
	// Both are checked by the column, not only here.
	Mode      string
	Collision string
	Filename  string
	Data      []byte

	CreatedAt time.Time
}

// Store reads and writes staged uploads.
type Store struct {
	DB *db.DB
}

// NewStore creates a staging store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// Stage puts an upload away and returns the token that fetches it back.
//
// The token is returned exactly once and is never stored: only its SHA-256 goes
// into the row, so a copied database yields the staged bytes but no resumable
// import. It is 128 random bits rendered as hex and never the row id — a
// sequential number in a URL invites typing the neighbour's.
//
// Note what is *not* here: no previous row of this user is deleted.
// user/token.go:59-62 does exactly that, and there it is right, because a
// superseded invitation has to stop working. Copied to this table it would mean
// an admin with two browser tabs loses the first import the moment they start
// the second, silently and with the file already gone.
func (s *Store) Stage(ctx context.Context, u Upload) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate staging token: %w", err)
	}
	token := hex.EncodeToString(raw)

	website := sql.NullInt64{Int64: u.WebsiteID, Valid: u.WebsiteID != 0}
	collision := u.Collision
	if collision == "" {
		collision = "uebergehen"
	}

	if _, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO csv_imports
		     (token_hash, user_id, website_id, website_name, modus, kollision, dateiname, daten, erstellt_am)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		hashToken(token), u.UserID, website, u.WebsiteName, u.Mode, collision,
		u.Filename, u.Data, time.Now().UTC().Format(timeLayout)); err != nil {
		return "", fmt.Errorf("stage csv upload: %w", err)
	}
	return token, nil
}

// Get fetches a staged upload for the user who staged it.
//
// The row is found by the hash of the token, so the token itself is never
// compared against anything in the database. Ownership is checked after the row
// is found and not as part of the lookup, which is what makes the two answers
// distinguishable: a token nobody has is expired, a token somebody else has is
// refused. The check lives here rather than in each of the four handlers, so a
// fifth screen added later cannot forget it.
func (s *Store) Get(ctx context.Context, token string, userID int64) (*Upload, error) {
	var u Upload
	var created string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, user_id, COALESCE(website_id, 0), website_name, modus, kollision,
		        dateiname, daten, erstellt_am
		   FROM csv_imports WHERE token_hash = $1`, hashToken(token)).
		Scan(&u.ID, &u.UserID, &u.WebsiteID, &u.WebsiteName, &u.Mode, &u.Collision,
			&u.Filename, &u.Data, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrExpired
	}
	if err != nil {
		return nil, fmt.Errorf("look up staged csv upload: %w", err)
	}
	if u.UserID != userID {
		return nil, ErrForeign
	}
	if t, err := time.Parse(timeLayout, created); err == nil {
		u.CreatedAt = t
	}
	return &u, nil
}

// Delete removes one staged upload.
//
// The write screen calls this once the rows have been written and before the
// report is rendered, so a refresh on the report finds no token and lands on
// the expiry screen instead of importing the same file a second time. Deleting
// afterwards rather than beforehand is deliberate: a process that dies
// mid-write leaves the row intact and the operator can retry, with the dry run
// telling them what already exists.
func (s *Store) Delete(ctx context.Context, id int64) error {
	if _, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM csv_imports WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete staged csv upload: %w", err)
	}
	return nil
}

// Prune drops staged uploads older than the given age and returns how many went.
//
// Finished and abandoned alike: a finished import's row has already been taken
// by Delete, so what this finds is what somebody walked away from.
func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(timeLayout)
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM csv_imports WHERE erstellt_am < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune staged csv uploads: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// hashToken is what gets stored. Comparison happens on the hash, so the token
// never sits in the database — user/token.go:152-155 derives its own the same
// way, and 00012's head comment states the discipline for both.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
