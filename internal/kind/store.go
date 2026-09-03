package kind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// ErrDuplicate is returned when a website already has a kind with that key.
var ErrDuplicate = errors.New("diese Inhaltsart gibt es schon")

// ErrTooMany is returned when a website has reached MaxTypes.
var ErrTooMany = errors.New("mehr Inhaltsarten gehen nicht")

// ErrNotFound is returned when no kind matches.
var ErrNotFound = errors.New("diese Inhaltsart gibt es nicht")

// ErrArchiveTaken is returned when two kinds would share one overview address.
var ErrArchiveTaken = errors.New("diese Adresse gehört schon zu einer anderen Übersicht")

// Store keeps the kinds.
type Store struct{ DB *db.DB }

// NewStore creates the store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// List returns a website's own kinds in their order.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Type, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, kennung, name, mehrzahl, archiv, sortierung, position
		 FROM content_types WHERE website_id = $1 ORDER BY position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("inhaltsarten lesen: %w", err)
	}
	defer rows.Close()

	var out []Type
	for rows.Next() {
		var t Type
		if err := rows.Scan(&t.ID, &t.WebsiteID, &t.Key, &t.Name, &t.Plural,
			&t.Archive, &t.Sort, &t.Position); err != nil {
			return nil, fmt.Errorf("inhaltsart lesen: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one kind of a website, or ErrNotFound.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (Type, error) {
	var t Type
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, website_id, kennung, name, mehrzahl, archiv, sortierung, position
		 FROM content_types WHERE id = $1 AND website_id = $2`, id, websiteID).
		Scan(&t.ID, &t.WebsiteID, &t.Key, &t.Name, &t.Plural, &t.Archive, &t.Sort, &t.Position)
	if err == sql.ErrNoRows {
		return Type{}, ErrNotFound
	}
	if err != nil {
		return Type{}, fmt.Errorf("inhaltsart lesen: %w", err)
	}
	return t, nil
}

// Create adds a kind. The key is derived from the name and fixed afterwards.
func (s *Store) Create(ctx context.Context, t Type) (Type, error) {
	t = clean(t)
	if t.Key == "" {
		t.Key = Key(t.Name)
	}
	if !ValidKey(t.Key) {
		return Type{}, fmt.Errorf("die Kennung %q ist keine: zwei bis dreissig kleine Buchstaben, Ziffern und Unterstriche", t.Key)
	}
	if t.Name == "" || t.Plural == "" {
		return Type{}, errors.New("eine Inhaltsart braucht einen Namen und eine Mehrzahl")
	}

	existing, err := s.List(ctx, t.WebsiteID)
	if err != nil {
		return Type{}, err
	}
	if len(existing) >= MaxTypes {
		return Type{}, ErrTooMany
	}
	for _, e := range existing {
		if e.Key == t.Key {
			return Type{}, ErrDuplicate
		}
		if t.Archive != "" && e.Archive == t.Archive {
			return Type{}, ErrArchiveTaken
		}
	}
	t.Position = len(existing)

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO content_types (website_id, kennung, name, mehrzahl, archiv, sortierung, position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.WebsiteID, t.Key, t.Name, t.Plural, t.Archive, t.Sort, t.Position)
	if err != nil {
		return Type{}, fmt.Errorf("inhaltsart anlegen: %w", err)
	}
	t.ID, _ = res.LastInsertId()
	return t, nil
}

// Update changes everything except the key.
//
// The key stays because it is written into every entry of this kind; changing
// it would mean rehanging all of them, and a rename that silently empties a
// list is worse than a name somebody has to live with.
func (s *Store) Update(ctx context.Context, t Type) error {
	t = clean(t)
	if t.Name == "" || t.Plural == "" {
		return errors.New("eine Inhaltsart braucht einen Namen und eine Mehrzahl")
	}

	others, err := s.List(ctx, t.WebsiteID)
	if err != nil {
		return err
	}
	for _, e := range others {
		if e.ID != t.ID && t.Archive != "" && e.Archive == t.Archive {
			return ErrArchiveTaken
		}
	}

	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE content_types SET name = $1, mehrzahl = $2, archiv = $3, sortierung = $4
		 WHERE id = $5 AND website_id = $6`,
		t.Name, t.Plural, t.Archive, t.Sort, t.ID, t.WebsiteID)
	if err != nil {
		return fmt.Errorf("inhaltsart sichern: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a kind. The entries stay: they keep their key, appear under it
// in the list and can be moved to another kind — deleting a kind by accident
// must not delete a hundred products with it.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM content_types WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("inhaltsart entfernen: %w", err)
	}
	return nil
}

// Count is how many entries carry a kind, so the delete screen can say what is
// at stake.
func (s *Store) Count(ctx context.Context, websiteID int64, key string) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages WHERE website_id = $1 AND kind = $2 AND deleted_at IS NULL`,
		websiteID, key).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("einträge zählen: %w", err)
	}
	return n, nil
}

// Move shifts a kind one place up or down, which decides the order of the
// menus and filters it appears in.
func (s *Store) Move(ctx context.Context, websiteID, id int64, up bool) error {
	list, err := s.List(ctx, websiteID)
	if err != nil {
		return err
	}
	at := -1
	for i, t := range list {
		if t.ID == id {
			at = i
		}
	}
	if at < 0 {
		return ErrNotFound
	}
	other := at + 1
	if up {
		other = at - 1
	}
	if other < 0 || other >= len(list) {
		return nil
	}
	list[at], list[other] = list[other], list[at]
	for i, t := range list {
		if _, err := s.DB.Write.ExecContext(ctx,
			`UPDATE content_types SET position = $1 WHERE id = $2`, i, t.ID); err != nil {
			return fmt.Errorf("reihenfolge sichern: %w", err)
		}
	}
	return nil
}

// clean trims and bounds what came out of a form.
func clean(t Type) Type {
	t.Name = trim(t.Name, 40)
	t.Plural = trim(t.Plural, 40)
	t.Key = strings.ToLower(trim(t.Key, 30))
	t.Archive = strings.ToLower(trim(t.Archive, 60))
	if t.Sort != SortTitle {
		t.Sort = SortNewest
	}
	return t
}

func trim(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}
