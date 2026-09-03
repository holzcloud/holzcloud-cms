package field

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// ErrDuplicateKey is returned when a key is already taken on this website.
var ErrDuplicateKey = errors.New("dieses Feld gibt es schon")

// ErrTooMany is returned when a website has reached MaxFields.
var ErrTooMany = errors.New("mehr Felder gehen nicht")

// ErrNested is returned for a group inside a group.
var ErrNested = errors.New("eine Gruppe in einer Gruppe gibt es nicht")

// ErrNoGroup is returned when the named parent is not a group of this website.
var ErrNoGroup = errors.New("diese Gruppe gibt es nicht")

// ErrKindFixed is returned when a group would become a plain field or back.
var ErrKindFixed = errors.New("aus einer Gruppe wird kein einfaches Feld und umgekehrt")

// ErrNotInBlock is returned for a field kind a block kind cannot carry.
var ErrNotInBlock = errors.New("diese Art von Feld gibt es in einem Baustein nicht")

// ErrNoCondition is returned when the field a condition names is not one this
// field can hang on.
var ErrNoCondition = errors.New("an dieses Feld lässt sich keine Bedingung hängen")

// ErrConditionLoop is returned when a condition would close a circle.
var ErrConditionLoop = errors.New("die Bedingungen drehen sich im Kreis: dann wäre keines der Felder je zu sehen")

// Store keeps the definitions.
type Store struct{ DB *db.DB }

// NewStore creates the store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// List returns a website's fields in their order, each group carrying its own.
//
// One query for everything and the tree built in memory: a query per group
// would be a query per group on every page render, and a website with five
// groups would pay five round trips to draw one page.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung, COALESCE(block_type_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id IS NULL ORDER BY position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("felder lesen: %w", err)
	}
	defer rows.Close()

	var top []Def
	children := map[int64][]Def{}
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		if d.ParentID == 0 {
			top = append(top, d)
			continue
		}
		children[d.ParentID] = append(children[d.ParentID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range top {
		if top[i].IsGroup() {
			top[i].Sub = children[top[i].ID]
		}
	}
	return top, nil
}

// Sub returns the fields of one group.
func (s *Store) Sub(ctx context.Context, websiteID, groupID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung, COALESCE(block_type_id, 0)
		 FROM page_field_defs WHERE website_id = $1 AND parent_id = $2 ORDER BY position, id`,
		websiteID, groupID)
	if err != nil {
		return nil, fmt.Errorf("gruppenfelder lesen: %w", err)
	}
	defer rows.Close()

	var out []Def
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// OfBlockType returns the fields of one block kind, in order.
func (s *Store) OfBlockType(ctx context.Context, websiteID, blockTypeID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung, COALESCE(block_type_id, 0)
		 FROM page_field_defs WHERE website_id = $1 AND block_type_id = $2 ORDER BY position, id`,
		websiteID, blockTypeID)
	if err != nil {
		return nil, fmt.Errorf("bausteinfelder lesen: %w", err)
	}
	defer rows.Close()

	var out []Def
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// OfBlockTypes returns every block kind's fields of one website, keyed by kind.
//
// One query rather than one per kind: this runs on every save of a page and on
// every draw of the block editor.
func (s *Store) OfBlockTypes(ctx context.Context, websiteID int64) (map[int64][]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung, COALESCE(block_type_id, 0)
		 FROM page_field_defs WHERE website_id = $1 AND block_type_id IS NOT NULL
		 ORDER BY block_type_id, position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("bausteinfelder lesen: %w", err)
	}
	defer rows.Close()

	out := map[int64][]Def{}
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		out[d.BlockTypeID] = append(out[d.BlockTypeID], d)
	}
	return out, rows.Err()
}

func scanDef(row interface{ Scan(...any) error }) (Def, error) {
	var (
		d       Def
		pflicht int
		auswahl string
	)
	if err := row.Scan(&d.ID, &d.WebsiteID, &d.ParentID, &d.Key, &d.Label, &d.Kind,
		&pflicht, &d.Hint, &auswahl, &d.AppliesTo, &d.Position, &d.Condition, &d.BlockTypeID); err != nil {
		return Def{}, fmt.Errorf("feld lesen: %w", err)
	}
	d.Required = pflicht == 1
	d.Choices = SplitChoices(auswahl)
	return d, nil
}

// Get returns one field of a website.
//
// The website is part of the lookup and not merely checked afterwards: the id
// comes out of an address, and without it an editor could reach another site's
// field by typing its number.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Def, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), kennung, beschriftung, art,
		        pflicht, hinweis, auswahl, gilt_fuer, position, bedingung, COALESCE(block_type_id, 0)
		 FROM page_field_defs WHERE id = $1 AND website_id = $2`, id, websiteID)
	d, err := scanDef(row)
	if err != nil {
		return nil, err
	}
	if d.IsGroup() {
		if sub, serr := s.Sub(ctx, websiteID, d.ID); serr == nil {
			d.Sub = sub
		}
	}
	return &d, nil
}

// Create adds a field.
func (s *Store) Create(ctx context.Context, d Def) (*Def, error) {
	if err := validate(&d); err != nil {
		return nil, err
	}

	if d.ParentID > 0 {
		parent, err := s.Get(ctx, d.WebsiteID, d.ParentID)
		if err != nil {
			return nil, ErrNoGroup
		}
		if !parent.IsGroup() {
			return nil, ErrNoGroup
		}
		if d.IsGroup() {
			return nil, ErrNested
		}
	}

	if err := s.checkCondition(ctx, d, 0); err != nil {
		return nil, err
	}

	var count int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1`, d.WebsiteID).Scan(&count); err != nil {
		return nil, fmt.Errorf("felder zählen: %w", err)
	}
	if count >= MaxFields {
		return nil, ErrTooMany
	}

	var parent, blockType any
	if d.ParentID > 0 {
		parent = d.ParentID
	}
	if d.BlockTypeID > 0 {
		blockType = d.BlockTypeID
	}
	// The position counts within the level: the page's own fields, one group's
	// fields, or one block kind's fields. Three worlds in one table, and a
	// field must never be able to move out of its own.
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, parent_id, block_type_id, kennung, beschriftung, art, pflicht, hinweis, auswahl, gilt_fuer, bedingung, position)
		 VALUES ($1, $2, $11, $3, $4, $5, $6, $7, $8, $9, $10,
		         COALESCE((SELECT MAX(position) + 1 FROM page_field_defs
		                   WHERE website_id = $1
		                     AND COALESCE(parent_id, 0) = COALESCE($2, 0)
		                     AND COALESCE(block_type_id, 0) = COALESCE($11, 0)), 0))`,
		d.WebsiteID, parent, d.Key, d.Label, d.Kind, boolToInt(d.Required), d.Hint,
		JoinChoices(d.Choices), d.AppliesTo, d.Condition, blockType)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDuplicateKey
		}
		return nil, fmt.Errorf("feld anlegen: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, d.WebsiteID, id)
}

// Update changes a field.
//
// The key is not among the things that change. It is what the theme and every
// stored value refer to; letting it move would empty every page at once, and
// the person renaming a label has no reason to expect that.
func (s *Store) Update(ctx context.Context, websiteID, id int64, d Def) error {
	d.WebsiteID = websiteID
	existing, err := s.Get(ctx, websiteID, id)
	if err != nil {
		return err
	}
	d.Key = existing.Key
	d.ParentID = existing.ParentID
	d.BlockTypeID = existing.BlockTypeID
	// The kind of a group cannot change: its rows would have nowhere to go,
	// and a plain field turned into a group would start out with none.
	if existing.IsGroup() != (d.Kind == KindGroup) {
		return ErrKindFixed
	}
	if err := validate(&d); err != nil {
		return err
	}
	if err := s.checkCondition(ctx, d, id); err != nil {
		return err
	}

	_, err = s.DB.Write.ExecContext(ctx,
		`UPDATE page_field_defs
		 SET beschriftung = $1, art = $2, pflicht = $3, hinweis = $4, auswahl = $5,
		     gilt_fuer = $6, bedingung = $7
		 WHERE id = $8 AND website_id = $9`,
		d.Label, d.Kind, boolToInt(d.Required), d.Hint, JoinChoices(d.Choices), d.AppliesTo,
		d.Condition, id, websiteID)
	if err != nil {
		return fmt.Errorf("feld ändern: %w", err)
	}
	return nil
}

// checkCondition makes sure a condition names a field that can carry one, and
// that following the conditions from there does not lead back.
//
// self is the id of the field being changed, 0 when it is being created. It is
// needed because the chain is walked over what is stored, and the stored
// version of this field still has its old condition.
func (s *Store) checkCondition(ctx context.Context, d Def, self int64) error {
	if d.Condition == "" {
		return nil
	}
	defs, err := s.List(ctx, d.WebsiteID)
	if err != nil {
		return err
	}
	by := map[string]Def{}
	for _, existing := range defs {
		if existing.ID == self {
			continue
		}
		by[existing.Key] = existing
	}

	target, ok := by[d.Condition]
	if !ok || !target.MayControl() {
		return ErrNoCondition
	}
	// Walk the chain. Bounded by the number of fields, so a circle among the
	// stored ones — which an older version could have written — ends the walk
	// instead of the request.
	key := target.Key
	for range defs {
		next, ok := by[key]
		if !ok || next.Condition == "" {
			return nil
		}
		if next.Condition == d.Key {
			return ErrConditionLoop
		}
		key = next.Condition
	}
	return ErrConditionLoop
}

// Delete removes a field definition.
//
// The values stay on the pages. They are invisible from that moment, and the
// next save of a page drops them — which makes deleting a field by mistake
// something one can undo by defining it again.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM page_field_defs WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("feld löschen: %w", err)
	}
	return nil
}

// Move shifts a field one place up or down.
//
// Buttons rather than dragging, for the same reason as everywhere else here:
// dragging needs a script, and the order of eight fields is not worth one.
func (s *Store) Move(ctx context.Context, websiteID, id int64, up bool) error {
	current, err := s.Get(ctx, websiteID, id)
	if err != nil {
		return err
	}
	// Moved within its own level: a field inside a group, or inside a block
	// kind, has nothing to swap places with outside it.
	var defs []Def
	switch {
	case current.ParentID > 0:
		defs, err = s.Sub(ctx, websiteID, current.ParentID)
	case current.BlockTypeID > 0:
		defs, err = s.OfBlockType(ctx, websiteID, current.BlockTypeID)
	default:
		defs, err = s.List(ctx, websiteID)
	}
	if err != nil {
		return err
	}
	at := -1
	for i, d := range defs {
		if d.ID == id {
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
	if other < 0 || other >= len(defs) {
		return nil
	}
	defs[at], defs[other] = defs[other], defs[at]

	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reihenfolge ändern: %w", err)
	}
	defer tx.Rollback()
	for i, d := range defs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE page_field_defs SET position = $1 WHERE id = $2`, i, d.ID); err != nil {
			return fmt.Errorf("reihenfolge ändern: %w", err)
		}
	}
	return tx.Commit()
}

func validate(d *Def) error {
	d.Label = strings.TrimSpace(d.Label)
	if d.Label == "" {
		return errors.New("das Feld braucht eine Beschriftung")
	}
	if len(d.Label) > 60 {
		d.Label = d.Label[:60]
	}
	if d.Key == "" {
		d.Key = SlugifyKey(d.Label)
	}
	if d.Key == "" {
		return errors.New("aus dieser Beschriftung lässt sich keine Kennung bilden — bitte Buchstaben verwenden")
	}
	if !KnownKind(d.Kind) {
		return errors.New("diese Art von Feld gibt es nicht")
	}
	if d.Kind == KindChoice && len(d.Choices) == 0 {
		return errors.New("eine Auswahl braucht mindestens eine Möglichkeit")
	}
	if d.ParentID > 0 && d.Kind == KindGroup {
		return ErrNested
	}
	if d.ParentID > 0 && d.Kind == KindSection {
		return errors.New("eine Überschrift in einer Gruppe gibt es nicht")
	}
	// A heading has nothing to fill in, so nothing to require and nothing to
	// choose from. Silently dropped rather than refused: the screen does not
	// offer them for a section, and somebody who switched an existing field
	// over to a heading should not have to clear them by hand first.
	if d.Kind == KindSection {
		d.Required = false
		d.Choices = nil
		// A heading that comes and goes would have to take the fields under it
		// along, and those are its neighbours rather than its children — the
		// browser has no way to hide them without a script.
		d.Condition = ""
	}
	// Inside a group every row is filled in as a whole; a field that came and
	// went within a row would be a rule the person filling it in cannot see.
	// Inside a block kind the same, one level over.
	if d.ParentID > 0 || d.BlockTypeID > 0 {
		d.Condition = ""
	}
	if d.BlockTypeID > 0 {
		if !blockKind(d.Kind) {
			return ErrNotInBlock
		}
		// A block's fields are filled in or not; there is no form to refuse.
		// The value would be checked when the page is saved, and a page that
		// cannot be saved because of a half-written block is worse than a
		// half-written block.
		d.Required = false
		d.AppliesTo = ForBoth
	}
	d.Condition = strings.TrimSpace(d.Condition)
	if d.Condition != "" {
		if d.Condition == d.Key {
			return errors.New("ein Feld kann nicht von sich selbst abhängen")
		}
		if !validKey(d.Condition) {
			d.Condition = ""
		}
	}
	// Für Seiten, für Beiträge, für alles — oder für eine eigene Inhaltsart
	// dieser Website. Deren Kennung wird hier nicht geprüft: der Bildschirm
	// bietet nur vorhandene an, und eine Art, die später verschwindet, soll
	// ihre Felder behalten, falls sie wiederkommt. Was zu keiner Art gehört,
	// erscheint schlicht nirgends.
	if d.AppliesTo == "" {
		d.AppliesTo = ForBoth
	}
	if !validKey(d.AppliesTo) {
		d.AppliesTo = ForBoth
	}
	d.Hint = strings.TrimSpace(d.Hint)
	if len(d.Hint) > 200 {
		d.Hint = d.Hint[:200]
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// validKey is the shape both a field key and a content kind's key have: lower
// case letters, digits and underscores.
func validKey(s string) bool {
	if s == "" || len(s) > 30 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}
