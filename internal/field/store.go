package field

import (
	"context"
	"errors"
	"fmt"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// ErrDuplicateKey is returned when a key is already taken on this website.
var ErrDuplicateKey = errors.New(i18n.N("this field already exists"))

// ErrTooMany is returned when a website has reached MaxFields.
var ErrTooMany = errors.New(i18n.N("no more fields can be added"))

// ErrNested is returned for a group inside a group.
var ErrNested = errors.New(i18n.N("there is no group inside a group"))

// ErrNoGroup is returned when the named parent is not a group of this website.
var ErrNoGroup = errors.New(i18n.N("there is no such group"))

// ErrNoSnippet is returned when the named snippet is not one of this website.
var ErrNoSnippet = errors.New(i18n.N("there is no such snippet"))

// ErrNoBlockType is returned when the named block kind is not one of this
// website.
var ErrNoBlockType = errors.New(i18n.N("there is no such kind of block"))

// ErrKindFixed is returned when a group would become a plain field or back.
var ErrKindFixed = errors.New(i18n.N("a group does not become a plain field, nor the other way round"))

// ErrNotInBlock is returned for a field kind a block kind cannot carry.
var ErrNotInBlock = errors.New(i18n.N("a block has no field of this kind"))

// ErrNoCondition is returned when the field a condition names is not one this
// field can hang on.
var ErrNoCondition = errors.New(i18n.N("no condition can be hung on this field"))

// ErrRangeInverted is returned when a range's lower bound is above its upper
// one. A sentinel and not a bare error so the screen can answer it with a
// sentence written for the person filling the form in.
var ErrRangeInverted = errors.New(i18n.N("the lower bound is above the upper one"))

// ErrConditionLoop is returned when a condition would close a circle.
var ErrConditionLoop = errors.New(i18n.N("the conditions go round in a circle: none of the fields would ever be visible"))

// Store keeps the definitions.
type Store struct{ DB *db.DB }

// NewStore creates the store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// List returns a website's fields in their order, each group carrying its own.
//
// One query for everything and the tree built in memory: a query per group
// would be a query per group on every page render, and a website with five
// groups would pay five round trips to draw one page.
//
// The WHERE clause names every foreign namespace explicitly and lets none of
// them through as "whatever is left over". Without AND snippet_id IS NULL
// every snippet's field would stand on every page's editing form and in every
// theme's .Page.FieldList — silently, and visible only in the browser.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL
		 ORDER BY position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("read fields: %w", err)
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
//
// There is deliberately no snippet_id clause here, and this is the one place
// where the pattern is not copied: a group can stand on a page and equally on
// a snippet, and its sub-fields then carry both — parent_id and snippet_id. An
// AND snippet_id IS NULL would make every group on a snippet come back empty.
// The namespace here is the GROUP, and a group's id is unique within the
// website; the condition is therefore already named explicitly in the sense of
// D-09.
func (s *Store) Sub(ctx context.Context, websiteID, groupID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs WHERE website_id = $1 AND parent_id = $2 ORDER BY position, id`,
		websiteID, groupID)
	if err != nil {
		return nil, fmt.Errorf("read group fields: %w", err)
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
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id = $2 AND snippet_id IS NULL
		 ORDER BY position, id`,
		websiteID, blockTypeID)
	if err != nil {
		return nil, fmt.Errorf("read block fields: %w", err)
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
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id IS NOT NULL AND snippet_id IS NULL
		 ORDER BY block_type_id, position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("read block fields: %w", err)
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

// OfSnippet returns the fields of one text snippet, in order, each group
// carrying its own.
//
// Two models, and both deliberate: the column list, the error wrapper and the
// narrowing to one carrier come from OfBlockType; the tree building comes from
// List. A block kind cannot carry a group — BlockKinds() does not allow one —
// so OfBlockType needs none of that. A snippet has a form of its own and
// therefore carries everything a page's form carries, groups included.
//
// The website id is part of the query and not a check afterwards — for the
// reason stated at Get, which holds here unchanged.
func (s *Store) OfSnippet(ctx context.Context, websiteID, snippetID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND snippet_id = $2
		 ORDER BY position, id`, websiteID, snippetID)
	if err != nil {
		return nil, fmt.Errorf("read snippet fields: %w", err)
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

// OfSnippets returns every text snippet's fields of one website, keyed by
// snippet.
//
// One query rather than one per snippet, and the reason is the one OfBlockTypes
// names: this runs on every public assembly of a page, and a website with five
// snippets would otherwise pay five round trips to draw one page.
//
// The tree building comes from OfSnippet and for the same reason: a snippet can
// carry a group. A snippet's slice comes out here element for element and Sub
// for Sub exactly as OfSnippet hands it out for the same snippet — otherwise
// the public assembly and the admin screen would disagree about a website
// nobody had changed.
//
// The ORDER BY carries more than tidiness: snippet_id groups the rows so that
// one pass builds the map, and position, id is the same tie-breaker List and
// OfSnippet use — the bulk read and the single read therefore cannot disagree
// about the order, not even for two fields at the same position.
func (s *Store) OfSnippets(ctx context.Context, websiteID int64) (map[int64][]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND snippet_id IS NOT NULL
		 ORDER BY snippet_id, position, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("read snippet fields: %w", err)
	}
	defer rows.Close()

	// Created before the query and never nil: a caller on a website without a
	// single snippet field should hold the same map in their hand as one on a
	// website with many.
	out := map[int64][]Def{}
	children := map[int64][]Def{}
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		if d.ParentID == 0 {
			out[d.SnippetID] = append(out[d.SnippetID], d)
			continue
		}
		children[d.ParentID] = append(children[d.ParentID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, top := range out {
		for i := range top {
			if top[i].IsGroup() {
				top[i].Sub = children[top[i].ID]
			}
		}
	}
	return out, nil
}

func scanDef(row interface{ Scan(...any) error }) (Def, error) {
	var (
		d        Def
		required int
		choice   string
	)
	// The order here is that of the seven SELECT column lists — List, Sub,
	// OfBlockType, OfBlockTypes, OfSnippet, OfSnippets and Get — character for
	// character. The seven are copies of one another and have to stay that
	// way: a list that differs from the others loads a field silently with a
	// zero value, and nothing fails.
	//
	// The number stands there with their names so that it can be counted
	// rather than believed — it said five for four migrations while there had
	// long been seven. TestSpaltenlistenSindAbschriften counts them out of the
	// file and compares them character for character; a fifth carrier turns
	// that test red, and that is the intention.
	if err := row.Scan(&d.ID, &d.WebsiteID, &d.ParentID, &d.Key, &d.Label, &d.Kind,
		&required, &d.Hint, &choice, &d.AppliesTo, &d.Position, &d.Condition,
		&d.Display, &d.MaxValues, &d.RangeMin, &d.RangeMax, &d.BlockTypeID,
		&d.SnippetID); err != nil {
		return Def{}, fmt.Errorf("read field: %w", err)
	}
	d.Required = required == 1
	d.Choices = SplitChoices(choice)
	return d, nil
}

// Get returns one field of a website.
//
// The website is part of the lookup and not merely checked afterwards: the id
// comes out of an address, and without it an editor could reach another site's
// field by typing its number.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Def, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
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

// belongsToWebsite checks that a carrier row belongs to this website.
//
// The table name comes from the body of this file and never from outside — two
// fixed strings at two call sites — which is why it is interpolated here
// rather than bound. The two ids are bound, as everywhere else.
func (s *Store) belongsToWebsite(ctx context.Context, tabelle string, id, websiteID int64) error {
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tabelle+` WHERE id = $1 AND website_id = $2`,
		id, websiteID).Scan(&n); err != nil {
		return fmt.Errorf("check carrier: %w", err)
	}
	if n == 0 {
		return errors.New(i18n.N("belongs to another website"))
	}
	return nil
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

	// The carrier belongs to this website, and that is stated here and not
	// only at the callers.
	//
	// REFERENCES proves the row exists; it does not prove it belongs to
	// d.WebsiteID, and the two partial indexes are drawn on snippet_id and
	// block_type_id alone — the database would file a definition across the
	// website boundary without complaint. The admin screen guards it
	// (snippetOf) and the archive path hands in an id it has just created;
	// both correct, both outside the store. One row per definition is cheap,
	// and it covers every future caller who does not know this.
	if d.SnippetID > 0 {
		if err := s.belongsToWebsite(ctx, "snippets", d.SnippetID, d.WebsiteID); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNoSnippet, err)
		}
	}
	if d.BlockTypeID > 0 {
		if err := s.belongsToWebsite(ctx, "block_types", d.BlockTypeID, d.WebsiteID); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNoBlockType, err)
		}
	}

	if err := s.checkCondition(ctx, d, 0); err != nil {
		return nil, err
	}

	// What is counted is the carrier being written into, and not the website
	// (D-05). Four arms in the same house style as the switch in Move, each
	// naming its namespace with an explicit SQL condition, and none of them
	// called "whatever is left over" (D-09).
	//
	var (
		zaehlung string
		values   []any
	)
	switch {
	case d.SnippetID > 0:
		// Stands above the group arm, and that is no accident: a group may
		// stand on a snippet, and its sub-fields then carry both. snippet_id =
		// $2 catches them too, and that is right — they are rows of the same
		// form.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND snippet_id = $2`
		values = []any{d.WebsiteID, d.SnippetID}
	case d.BlockTypeID > 0:
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id = $2 AND snippet_id IS NULL`
		values = []any{d.WebsiteID, d.BlockTypeID}
	case d.ParentID > 0:
		// A sub-field of a group on a page counts against the page's
		// allowance, exactly as before: the group is drawn on the page's form,
		// and its rows belong there. Written out rather than folded into the
		// default arm, so that the namespace stands there and does not have to
		// be inferred.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL`
		values = []any{d.WebsiteID}
	default:
		// The page's own fields — the carrier of this arm, and not the
		// remainder.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL`
		values = []any{d.WebsiteID}
	}
	var count int
	if err := s.DB.Read.QueryRowContext(ctx, zaehlung, values...).Scan(&count); err != nil {
		return nil, fmt.Errorf("count fields: %w", err)
	}
	if count >= MaxFields {
		return nil, ErrTooMany
	}

	var parent, blockType, snippet any
	if d.ParentID > 0 {
		parent = d.ParentID
	}
	if d.BlockTypeID > 0 {
		blockType = d.BlockTypeID
	}
	if d.SnippetID > 0 {
		snippet = d.SnippetID
	}
	// The position counts within the level: the page's own fields, one group's
	// fields, one block kind's fields, or one text snippet's fields. Four
	// worlds in one table, and a field must never be able to move out of its
	// own.
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO page_field_defs (website_id, parent_id, block_type_id, snippet_id, key, label, kind, required, hint, choices, applies_to, condition,
		                             display, max_values, range_min, range_max, position)
		 VALUES ($1, $2, $11, $16, $3, $4, $5, $6, $7, $8, $9, $10,
		         $12, $13, $14, $15,
		         COALESCE((SELECT MAX(position) + 1 FROM page_field_defs
		                   WHERE website_id = $1
		                     AND COALESCE(parent_id, 0) = COALESCE($2, 0)
		                     AND COALESCE(block_type_id, 0) = COALESCE($11, 0)
		                     AND COALESCE(snippet_id, 0) = COALESCE($16, 0)), 0))`,
		d.WebsiteID, parent, d.Key, d.Label, d.Kind, boolToInt(d.Required), d.Hint,
		JoinChoices(d.Choices), d.AppliesTo, d.Condition, blockType,
		d.Display, d.MaxValues, d.RangeMin, d.RangeMax, snippet)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDuplicateKey
		}
		return nil, fmt.Errorf("create field: %w", err)
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
	// The carrier is taken from what is stored and never from what comes in:
	// otherwise an editing form could push a field out of its namespace into
	// another one.
	d.SnippetID = existing.SnippetID
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
		 SET label = $1, kind = $2, required = $3, hint = $4, choices = $5,
		     applies_to = $6, condition = $7,
		     display = $10, max_values = $11, range_min = $12, range_max = $13
		 WHERE id = $8 AND website_id = $9`,
		d.Label, d.Kind, boolToInt(d.Required), d.Hint, JoinChoices(d.Choices), d.AppliesTo,
		d.Condition, id, websiteID,
		d.Display, d.MaxValues, d.RangeMin, d.RangeMax)
	if err != nil {
		return fmt.Errorf("update field: %w", err)
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
		return fmt.Errorf("delete field: %w", err)
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
	// Moved within its own level: a field inside a group, inside a block kind
	// or inside a text snippet has nothing to swap places with outside it.
	//
	// Every arm names its carrier, and the default arm is the page itself and
	// not "whatever is left over" (D-09): a field with no group, no block kind
	// and no snippet is a page field. When a fifth carrier comes it gets an arm
	// of its own rather than landing here silently — and until then the first
	// field of a snippet has nothing to swap upwards with, even when page
	// fields of the same website occupy lower positions.
	var defs []Def
	switch {
	case current.ParentID > 0:
		defs, err = s.Sub(ctx, websiteID, current.ParentID)
	case current.BlockTypeID > 0:
		defs, err = s.OfBlockType(ctx, websiteID, current.BlockTypeID)
	case current.SnippetID > 0:
		defs, err = s.OfSnippet(ctx, websiteID, current.SnippetID)
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
		return fmt.Errorf("update order: %w", err)
	}
	defer tx.Rollback()
	for i, d := range defs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE page_field_defs SET position = $1 WHERE id = $2`, i, d.ID); err != nil {
			return fmt.Errorf("update order: %w", err)
		}
	}
	return tx.Commit()
}

func validate(d *Def) error {
	d.Label = strings.TrimSpace(d.Label)
	if d.Label == "" {
		return errors.New(i18n.N("the field needs a label"))
	}
	if len(d.Label) > 60 {
		d.Label = d.Label[:60]
	}
	if d.Key == "" {
		d.Key = SlugifyKey(d.Label)
	}
	if d.Key == "" {
		return errors.New(i18n.N("no key can be made from this label — please use letters"))
	}
	// Only here, after the derivation: the empty case above keeps its own,
	// more helpful reason.
	//
	// A derived key carries only [a-z0-9_] anyway, so nothing from the screen
	// would catch here. The one path on which a key is BROUGHT rather than
	// derived is the archive path (internal/bundle/import.go:351) — a file from
	// somebody else's machine.
	if !validKey(d.Key) {
		return errors.New(i18n.N("a key carries only lower-case letters, digits and underscores"))
	}
	if !KnownKind(d.Kind) {
		return errors.New(i18n.N("there is no field of this kind"))
	}
	// Both kinds of choice: a multi-choice with no options draws a group with
	// nothing in it but the hidden sentinel, so it can never carry a value. If
	// it is required on top of that, Check reports on every save of every page
	// that it has to be filled in, and the form offers nothing to do that with
	// — the page would be unsaveable until somebody changes the definition.
	if (d.Kind == KindChoice || d.Kind == KindMulti) && len(d.Choices) == 0 {
		return errors.New(i18n.N("a choice needs at least one option"))
	}
	// A negative maximum cannot come from an honest form — the field carries
	// min="0" — and quietly pulling it to zero would mean reading a crafted
	// input as an intention. Refused rather than bent into shape, and the
	// refusal stands before the emptying further down, so that it does not
	// depend on which kind happened to be chosen.
	if d.MaxValues < 0 {
		return errors.New(i18n.N("there is no maximum below zero — zero means no upper limit"))
	}
	// Inverted bounds: the question is only asked at all when both read as
	// numbers. Two words are not an inverted pair of numbers, they are two
	// words, and this check is none of their business.
	d.RangeMin = strings.TrimSpace(d.RangeMin)
	d.RangeMax = strings.TrimSpace(d.RangeMax)
	if d.RangeMin != "" && d.RangeMax != "" {
		// The same reading as Check: ParseNumber takes the comma as a decimal
		// separator. Two places reading the same digits differently would be a
		// pair that gets through here and lets nothing through there.
		unten, hatUnten := ParseNumber(d.RangeMin)
		oben, hatOben := ParseNumber(d.RangeMax)
		if hatUnten && hatOben && unten > oben {
			return ErrRangeInverted
		}
	}
	// The display belongs to a choice, the maximum to a multi-choice, the two
	// bounds to a range field. Whatever does not fit the chosen kind is
	// emptied rather than refused — the same bargain the heading further down
	// already makes: somebody converting an existing field should not have to
	// clear boxes by hand first.
	//
	// Both or neither is emptied: a range field that sets only the lower or
	// only the upper bound keeps it. Open at the top or at the bottom is a
	// deliberate statement and not a half-filled pair.
	if d.Kind != KindChoice {
		d.Display = ""
	}
	if d.Kind != KindMulti {
		d.MaxValues = 0
	}
	if d.Kind != KindRange {
		d.RangeMin, d.RangeMax = "", ""
	}
	if d.ParentID > 0 && d.Kind == KindGroup {
		return ErrNested
	}
	if d.ParentID > 0 && d.Kind == KindSection {
		return errors.New(i18n.N("there is no heading inside a group"))
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
	//
	// On a snippet for the same reason — its form is filled in as a whole —
	// and for one of its own on top: checkCondition walks the page's field list
	// (List), and the CSS rule that hides a dependent field is written for the
	// page form. A condition on a snippet field would therefore be stored and
	// never honoured, and that is worse than not offering it at all.
	if d.ParentID > 0 || d.BlockTypeID > 0 || d.SnippetID > 0 {
		d.Condition = ""
	}
	// Deliberately an arm of its own and not the block-kind arm below it. This
	// is the one place in this phase where copying the model would be wrong —
	// and silent, because nothing would fail.
	//
	// Required is not forced to false here: the block kind does that, because
	// saving a page must not fail on a half-written block — and the arm below
	// stays the only place in the tree where a carrier enforces what a counting
	// gate of this plan proves. A snippet has a form of its own on which a
	// required field can be refused with a reason, so required stays meaningful
	// here.
	//
	// And no narrowing of the field kinds: BlockKinds() leaves out reference
	// and term, because a block freezes into HTML when the page is saved and
	// both would silently go stale at the next rename. A snippet's values are
	// resolved on the way out by field.Resolve, exactly like a page's — so that
	// reason does not reach this far, and a snippet offers field.Kinds in full,
	// groups included.
	//
	// Two things are narrowed, and neither is a field kind: the condition
	// above, for the reason given there, and applies_to here — "applies to
	// pages / to posts" has no meaning on a snippet, because a snippet is not a
	// page and belongs to no content kind.
	if d.SnippetID > 0 {
		d.AppliesTo = ForBoth
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
			return errors.New(i18n.N("a field cannot depend on itself"))
		}
		if !validKey(d.Condition) {
			d.Condition = ""
		}
	}
	// For pages, for posts, for everything — or for one of this website's own
	// content kinds. Its key is not checked here: the screen offers only ones
	// that exist, and a kind that disappears later should keep its fields in
	// case it comes back. Whatever belongs to no kind simply appears nowhere.
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
//
// The upper bound is maxKeyBytes and therefore the same number at which
// SlugifyKey truncates: a key the derivation produced has to pass this check.
func validKey(s string) bool {
	if s == "" || len(s) > maxKeyBytes {
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
