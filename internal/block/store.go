package block

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// The block kinds a website defined, and where they are kept.
//
// The kind is a row here; its fields are rows in page_field_defs with
// block_type_id set — see migration 00038 for why they share that table with
// the pages' own fields rather than having one of their own.

// ErrDuplicate is returned when a key is already taken on this website.
var ErrDuplicate = errors.New("diese Bausteinart gibt es schon")

// ErrTooManyTypes is returned when a website has reached MaxTypes.
var ErrTooManyTypes = errors.New("mehr Bausteinarten gehen nicht")

// ErrReserved is returned for a key that a built-in kind already uses.
var ErrReserved = errors.New("diese Kennung gehört einer eingebauten Bausteinart")

// MaxTypes bounds how many kinds one website may define.
//
// Editorial, not technical: the menu is a list somebody reads before they can
// write a sentence, and a list of thirty is a list nobody reads.
const MaxTypes = 12

// Store keeps the kinds.
type Store struct {
	DB     *db.DB
	Fields *field.Store
}

// NewStore creates the store.
func NewStore(database *db.DB, fields *field.Store) *Store {
	return &Store{DB: database, Fields: fields}
}

// List returns a website's own kinds in their order, each with its fields.
//
// The fields come in one query for the whole website rather than one per kind:
// this runs on every page save and on every draw of the editor, and a website
// with eight kinds would otherwise pay eight round trips to show one form.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Own, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, kennung, name, hinweis FROM block_types
		 WHERE website_id = $1 ORDER BY position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("bausteinarten lesen: %w", err)
	}
	defer rows.Close()

	var out []Own
	for rows.Next() {
		var o Own
		if err := rows.Scan(&o.ID, &o.Key, &o.Name, &o.Hint); err != nil {
			return nil, fmt.Errorf("bausteinart lesen: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}

	byType, err := s.Fields.OfBlockTypes(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Fields = byType[out[i].ID]
	}
	return out, nil
}

// Set is what everything else in this package needs: the website's kinds, ready
// to be handed to Clean, Render and the editor.
//
// A failure yields the built-in set rather than an error. The kinds decide what
// a stored block means, and losing them for one request must not turn a page
// full of recipe steps into a page full of nothing.
func (s *Store) Set(ctx context.Context, websiteID int64) Set {
	own, err := s.List(ctx, websiteID)
	if err != nil {
		return Builtin
	}
	return Set{Own: own}
}

// Get returns one kind of a website, with its fields.
//
// The website is part of the lookup and not merely checked afterwards: the id
// comes out of an address, and without it an editor could reach another site's
// kind by typing its number.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Own, error) {
	var o Own
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, kennung, name, hinweis FROM block_types WHERE id = $1 AND website_id = $2`,
		id, websiteID).Scan(&o.ID, &o.Key, &o.Name, &o.Hint)
	if err != nil {
		return nil, fmt.Errorf("bausteinart lesen: %w", err)
	}
	if fields, ferr := s.Fields.OfBlockType(ctx, websiteID, o.ID); ferr == nil {
		o.Fields = fields
	}
	return &o, nil
}

// Create adds a kind.
func (s *Store) Create(ctx context.Context, websiteID int64, name, hint string) (*Own, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("die Bausteinart braucht einen Namen")
	}
	if len(name) > 40 {
		name = name[:40]
	}
	key := field.SlugifyKey(name)
	if key == "" {
		return nil, errors.New("aus diesem Namen lässt sich keine Kennung bilden — bitte Buchstaben verwenden")
	}
	// A built-in kind's key would win every lookup and the own one would be
	// invisible for ever, with nothing on the screen to say why.
	if _, taken := KindOf(key); taken {
		return nil, ErrReserved
	}

	var count int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM block_types WHERE website_id = $1`, websiteID).Scan(&count); err != nil {
		return nil, fmt.Errorf("bausteinarten zählen: %w", err)
	}
	if count >= MaxTypes {
		return nil, ErrTooManyTypes
	}

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO block_types (website_id, kennung, name, hinweis, position)
		 VALUES ($1, $2, $3, $4,
		         COALESCE((SELECT MAX(position) + 1 FROM block_types WHERE website_id = $1), 0))`,
		websiteID, key, name, strings.TrimSpace(hint))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("bausteinart anlegen: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, websiteID, id)
}

// Update changes a kind's name and hint.
//
// The key is not among them: it stands in every block of that kind on every
// page and in the CSS class the theme styles. Changing it would empty both at
// once, and the person rewording a name has no reason to expect that.
func (s *Store) Update(ctx context.Context, websiteID, id int64, name, hint string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("die Bausteinart braucht einen Namen")
	}
	if len(name) > 40 {
		name = name[:40]
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE block_types SET name = $1, hinweis = $2 WHERE id = $3 AND website_id = $4`,
		name, strings.TrimSpace(hint), id, websiteID)
	if err != nil {
		return fmt.Errorf("bausteinart ändern: %w", err)
	}
	return nil
}

// Delete removes a kind and, through the foreign key, its fields.
//
// What is on the pages stays. A block of a kind that no longer exists renders
// to nothing and disappears from the page on its next save — which makes
// deleting the wrong kind something one can undo by defining it again, as long
// as nobody has saved that page in between.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM block_types WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("bausteinart löschen: %w", err)
	}
	return nil
}

// Move shifts a kind one place in the editor's menu.
func (s *Store) Move(ctx context.Context, websiteID, id int64, up bool) error {
	types, err := s.List(ctx, websiteID)
	if err != nil {
		return err
	}
	at := -1
	for i, t := range types {
		if t.ID == id {
			at = i
			break
		}
	}
	if at < 0 {
		return nil
	}
	other := at + 1
	if up {
		other = at - 1
	}
	if other < 0 || other >= len(types) {
		return nil
	}
	types[at], types[other] = types[other], types[at]

	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reihenfolge ändern: %w", err)
	}
	defer tx.Rollback()
	for i, t := range types {
		if _, err := tx.ExecContext(ctx,
			`UPDATE block_types SET position = $1 WHERE id = $2`, i, t.ID); err != nil {
			return fmt.Errorf("reihenfolge ändern: %w", err)
		}
	}
	return tx.Commit()
}

// Used counts how many pages carry a block of this kind, so the screen can say
// what removing it would cost before somebody presses the button.
func (s *Store) Used(ctx context.Context, websiteID int64, key string) (int, error) {
	var n int
	// A LIKE over the stored JSON. Exact enough for a warning — the key is
	// unique on this website and appears as "typ":"kennung" — and much cheaper
	// than decoding every page's blocks to count them.
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages WHERE website_id = $1 AND blocks LIKE $2`,
		websiteID, `%"typ":"`+key+`"%`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("bausteine zählen: %w", err)
	}
	return n, nil
}
