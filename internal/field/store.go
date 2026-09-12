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
// Die WHERE-Bedingung nennt jeden fremden Namensraum ausdrücklich und lässt
// keinen als „was übrig bleibt" durchgehen. Ohne AND snippet_id IS NULL stünde
// jedes Feld eines Textbausteins auf dem Bearbeitungsformular jeder Seite und
// in der .Page.FieldList jedes Themes — still, und nur im Browser zu sehen.
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
//
// Hier steht absichtlich kein snippet_id-Zusatz, und das ist die eine Stelle,
// an der das Muster nicht abgeschrieben wird: eine Gruppe kann an einer Seite
// stehen und ebenso an einem Textbaustein, und ihre Unterfelder tragen dann
// beides — parent_id und snippet_id. Ein AND snippet_id IS NULL liesse jede
// Gruppe an einem Textbaustein leer zurückkommen. Der Namensraum ist hier die
// Gruppe, und eine Gruppennummer ist innerhalb der Website eindeutig; die
// Bedingung ist damit im Sinne von D-09 bereits ausdrücklich genannt.
func (s *Store) Sub(ctx context.Context, websiteID, groupID int64) ([]Def, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
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
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id = $2 AND snippet_id IS NULL
		 ORDER BY position, id`,
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
		`SELECT id, website_id, COALESCE(parent_id, 0), key, label, kind,
		        required, hint, choices, applies_to, position, condition,
		        display, max_values, range_min, range_max,
		        COALESCE(block_type_id, 0), COALESCE(snippet_id, 0)
		 FROM page_field_defs
		 WHERE website_id = $1 AND block_type_id IS NOT NULL AND snippet_id IS NULL
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

// OfSnippet returns the fields of one text snippet, in order, each group
// carrying its own.
//
// Zwei Vorbilder, und beide absichtlich: die Spaltenliste, die Fehlerhülle und
// der Zuschnitt auf einen Träger kommen von OfBlockType; der Baumbau kommt von
// List. Eine Bausteinart kann keine Gruppe tragen — BlockKinds() lässt sie
// nicht zu —, deshalb braucht OfBlockType davon nichts. Ein Textbaustein hat
// ein eigenes Formular und trägt darum alles, was das Formular einer Seite
// trägt, Gruppen eingeschlossen.
//
// Die Websitenummer ist Teil der Abfrage und keine Prüfung danach — aus dem
// Grund, der bei Get steht und hier unverändert gilt.
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
		return nil, fmt.Errorf("textbausteinfelder lesen: %w", err)
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
// Eine Abfrage statt einer je Textbaustein, und der Grund ist derselbe, den
// OfBlockTypes nennt: das hier läuft auf jedem öffentlichen Aufbau einer Seite,
// und eine Website mit fünf Textbausteinen zahlte sonst fünf Umläufe, um eine
// Seite zu zeichnen.
//
// Der Baumbau kommt von OfSnippet und aus demselben Grund: ein Textbaustein
// kann eine Gruppe tragen. Die Scheibe eines Textbausteins kommt hier Element
// für Element und Sub für Sub so heraus, wie OfSnippet sie für denselben
// Textbaustein herausgibt — sonst wären der öffentliche Aufbau und der
// Verwaltungsbildschirm über eine Website uneins, an der niemand etwas
// geändert hat.
//
// Das ORDER BY trägt mehr als Ordentlichkeit: snippet_id gruppiert die Zeilen,
// sodass ein Durchgang die Karte baut, und position, id ist derselbe
// Gleichstandsbrecher, den List und OfSnippet benutzen — die Massenlesung und
// die Einzellesung können sich über die Reihenfolge damit nicht uneins werden,
// auch nicht bei zwei Feldern auf derselben Position.
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
		return nil, fmt.Errorf("textbausteinfelder lesen: %w", err)
	}
	defer rows.Close()

	// Vor der Abfrage angelegt und nie nil: ein Aufrufer soll auf einer
	// Website ohne ein einziges Textbausteinfeld dieselbe Karte in der Hand
	// halten wie auf einer mit vielen.
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
		d       Def
		pflicht int
		auswahl string
	)
	// Die Reihenfolge hier ist die der sieben SELECT-Spaltenlisten — List, Sub,
	// OfBlockType, OfBlockTypes, OfSnippet, OfSnippets und Get —, Zeichen für
	// Zeichen. Die sieben sind Abschriften voneinander und müssen es bleiben:
	// eine Liste, die von den anderen abweicht, lädt ein Feld still mit einem
	// Nullwert, und nichts schlägt fehl.
	//
	// Die Zahl steht mit ihren Namen da, damit sie nachzuzählen ist und nicht
	// geglaubt werden muss — sie stand vier Wanderungen lang auf fünf, während
	// es längst sieben waren. TestSpaltenlistenSindAbschriften zählt sie aus
	// der Datei und vergleicht sie Zeichen für Zeichen; ein fünfter Träger
	// macht diesen Test rot, und das ist die Absicht.
	if err := row.Scan(&d.ID, &d.WebsiteID, &d.ParentID, &d.Key, &d.Label, &d.Kind,
		&pflicht, &d.Hint, &auswahl, &d.AppliesTo, &d.Position, &d.Condition,
		&d.Display, &d.MaxValues, &d.RangeMin, &d.RangeMax, &d.BlockTypeID,
		&d.SnippetID); err != nil {
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

// gehoertZurWebsite prüft, dass eine Trägerzeile zu dieser Website gehört.
//
// Der Tabellenname kommt aus dem Rumpf dieser Datei und nie von aussen — zwei
// feste Zeichenketten an zwei Aufrufstellen —, deshalb ist er hier eingesetzt
// und nicht gebunden. Die beiden Nummern sind gebunden, wie überall sonst.
func (s *Store) gehoertZurWebsite(ctx context.Context, tabelle string, id, websiteID int64) error {
	var n int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tabelle+` WHERE id = $1 AND website_id = $2`,
		id, websiteID).Scan(&n); err != nil {
		return fmt.Errorf("träger prüfen: %w", err)
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

	// Der Träger gehört dieser Website, und das steht hier und nicht nur bei
	// den Aufrufern.
	//
	// REFERENCES beweist, dass es die Zeile gibt; dass sie zu d.WebsiteID
	// gehört, beweist es nicht, und die beiden Teilindizes sind auf snippet_id
	// beziehungsweise block_type_id allein gezogen — die Datenbank legte eine
	// Definition über die Websitegrenze hinweg klaglos ab. Der
	// Verwaltungsbildschirm wacht davor (snippetOf), der Archivweg reicht eine
	// eben angelegte Nummer herein; beide richtig, beide ausserhalb des
	// Speichers. Eine Zeile je Definition ist billig, und sie deckt jeden
	// künftigen Aufrufer mit, der das nicht weiss.
	if d.SnippetID > 0 {
		if err := s.gehoertZurWebsite(ctx, "snippets", d.SnippetID, d.WebsiteID); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNoSnippet, err)
		}
	}
	if d.BlockTypeID > 0 {
		if err := s.gehoertZurWebsite(ctx, "block_types", d.BlockTypeID, d.WebsiteID); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNoBlockType, err)
		}
	}

	if err := s.checkCondition(ctx, d, 0); err != nil {
		return nil, err
	}

	// Gezählt wird der Träger, in den geschrieben wird, und nicht die Website
	// (D-05). Vier Arme in derselben Hausform wie der Schalter in Move, jeder
	// nennt seinen Namensraum mit einer ausdrücklichen SQL-Bedingung, und
	// keiner heisst „was übrig bleibt" (D-09).
	//
	// Warum überhaupt: der Vorrat ist da, damit ein Formular benutzbar bleibt,
	// und ein Formular zeichnet immer nur die Felder eines Trägers. Ein
	// geteilter Vorrat lässt einen Träger den anderen still verwehren — wer das
	// zweite Feld an einen Textbaustein hängt, bekäme „mehr Felder gehen nicht"
	// zu lesen, weil die Bausteinarten den Vorrat aufgebraucht haben, und der
	// Satz wäre schlicht nicht wahr.
	var (
		zaehlung string
		werte    []any
	)
	switch {
	case d.SnippetID > 0:
		// Steht über dem Gruppenarm, und das ist kein Zufall: eine Gruppe darf
		// an einem Textbaustein stehen, und ihre Unterfelder tragen dann beides.
		// snippet_id = $2 fängt sie mit ein, und das ist richtig — sie sind
		// Zeilen desselben Formulars.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND snippet_id = $2`
		werte = []any{d.WebsiteID, d.SnippetID}
	case d.BlockTypeID > 0:
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id = $2 AND snippet_id IS NULL`
		werte = []any{d.WebsiteID, d.BlockTypeID}
	case d.ParentID > 0:
		// Ein Unterfeld einer Gruppe an einer Seite zählt gegen den Vorrat der
		// Seite, genau wie bisher: die Gruppe wird auf dem Seitenformular
		// gezeichnet, ihre Zeilen gehören dorthin. Ausgeschrieben statt in den
		// default-Arm gefaltet, damit der Namensraum dasteht und nicht
		// erschlossen werden muss.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL`
		werte = []any{d.WebsiteID}
	default:
		// Die eigenen Felder der Seite — der Träger dieses Arms und nicht der
		// Rest.
		zaehlung = `SELECT COUNT(*) FROM page_field_defs WHERE website_id = $1 AND block_type_id IS NULL AND snippet_id IS NULL`
		werte = []any{d.WebsiteID}
	}
	var count int
	if err := s.DB.Read.QueryRowContext(ctx, zaehlung, werte...).Scan(&count); err != nil {
		return nil, fmt.Errorf("felder zählen: %w", err)
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
	// Der Träger wird aus dem Gespeicherten übernommen und nie aus dem, was
	// hereinkommt: sonst könnte ein Bearbeitungsformular ein Feld aus seinem
	// Namensraum in einen anderen schieben.
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
	// Moved within its own level: a field inside a group, inside a block kind
	// or inside a text snippet has nothing to swap places with outside it.
	//
	// Jeder Arm nennt seinen Träger, und der default-Arm ist die Seite selbst
	// und nicht „was übrig bleibt" (D-09): ein Feld ohne Gruppe, ohne
	// Bausteinart und ohne Textbaustein ist ein Seitenfeld. Kommt ein fünfter
	// Träger, bekommt er einen eigenen Arm, statt still hier zu landen — und
	// bis dahin bleibt es das erste Feld eines Textbausteins, das nach oben
	// nichts zu tauschen hat, auch wenn Seitenfelder derselben Website tiefere
	// Positionen belegen.
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
	// Erst hier, nach der Ableitung: der leere Fall darüber behält seine
	// eigene, hilfreichere Begründung.
	//
	// Ein abgeleiteter Schlüssel trägt ohnehin nur [a-z0-9_], vom Bildschirm
	// kommt also nichts, was hier hängen bliebe. Der eine Weg, auf dem ein
	// Schlüssel mitgebracht statt abgeleitet wird, ist der Archivweg
	// (internal/bundle/import.go:351) — eine Datei von einem fremden Rechner.
	if !validKey(d.Key) {
		return errors.New(i18n.N("a key carries only lower-case letters, digits and underscores"))
	}
	if !KnownKind(d.Kind) {
		return errors.New(i18n.N("there is no field of this kind"))
	}
	// Beide Auswahlarten: eine Mehrfachauswahl ohne Möglichkeiten zeichnet
	// eine Gruppe, in der nichts steht als der versteckte Wächter, kann also
	// nie einen Wert tragen. Ist sie zusätzlich Pflicht, meldet Check bei jedem
	// Speichern jeder Seite, dass sie ausgefüllt werden müsse, und das Formular
	// bietet nichts an, womit das ginge — die Seite wäre unspeicherbar, bis
	// jemand die Definition ändert.
	if (d.Kind == KindChoice || d.Kind == KindMulti) && len(d.Choices) == 0 {
		return errors.New(i18n.N("a choice needs at least one option"))
	}
	// Eine negative Höchstzahl kann kein ehrliches Formular erzeugen — das Feld
	// trägt min="0" — und stillschweigend auf null zu ziehen hiesse, eine
	// gebastelte Eingabe als Absicht zu lesen. Abgelehnt statt zurechtgebogen,
	// und die Ablehnung steht vor dem Leeren weiter unten, damit sie nicht von
	// der Art abhängt, die zufällig gewählt war.
	if d.MaxValues < 0 {
		return errors.New(i18n.N("there is no maximum below zero — zero means no upper limit"))
	}
	// Verdrehte Grenzen: nur wenn beide als Zahl zu lesen sind, ist die Frage
	// überhaupt gestellt. Zwei Wörter sind kein verdrehtes Zahlenpaar, sondern
	// zwei Wörter, und die gehen diese Prüfung nichts an.
	d.RangeMin = strings.TrimSpace(d.RangeMin)
	d.RangeMax = strings.TrimSpace(d.RangeMax)
	if d.RangeMin != "" && d.RangeMax != "" {
		// Dieselbe Lesart wie Check: ParseNumber nimmt das Komma als
		// Dezimaltrennzeichen. Zwei Stellen, die dieselben Ziffern
		// verschieden läsen, wären ein Paar, das hier durchgeht und dort
		// nichts mehr durchlässt.
		unten, hatUnten := ParseNumber(d.RangeMin)
		oben, hatOben := ParseNumber(d.RangeMax)
		if hatUnten && hatOben && unten > oben {
			return ErrRangeInverted
		}
	}
	// Die Darstellung gehört einer Auswahl, die Höchstzahl einer
	// Mehrfachauswahl, die beiden Grenzen einem Bereichsfeld. Was zur
	// gewählten Art nicht passt, wird geleert und nicht abgelehnt — dieselbe
	// Abmachung, die die Überschrift weiter unten schon macht: wer ein
	// bestehendes Feld umstellt, soll nicht erst von Hand Kästchen ausräumen
	// müssen.
	//
	// Geleert wird immer beides oder keines: ein Bereichsfeld, das nur die
	// untere oder nur die obere Grenze setzt, behält sie. Nach oben oder nach
	// unten offen ist eine gewollte Angabe und kein halb ausgefülltes Paar.
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
	// An einem Textbaustein aus demselben Grund — sein Formular wird als Ganzes
	// ausgefüllt — und zusätzlich aus einem eigenen: checkCondition läuft über
	// die Feldliste der Seite (List), und die CSS-Regel, die ein abhängiges
	// Feld verbirgt, ist für das Seitenformular geschrieben. Eine Bedingung an
	// einem Textbausteinfeld wäre also gespeichert und würde nie beachtet, und
	// das ist schlechter, als sie gar nicht anzubieten.
	if d.ParentID > 0 || d.BlockTypeID > 0 || d.SnippetID > 0 {
		d.Condition = ""
	}
	// Absichtlich ein eigener Arm und nicht der Bausteinart-Arm darunter. Dies
	// ist die eine Stelle dieser Phase, an der das Abschreiben des Vorbilds
	// falsch wäre — und still, denn nichts schlüge fehl.
	//
	// Pflicht wird hier nicht auf falsch gezwungen: die Bausteinart tut das,
	// weil das Speichern einer Seite nicht an einem halb geschriebenen
	// Baustein scheitern darf — und der Arm darunter bleibt der einzige Ort im
	// Baum, an dem ein Träger das erzwingt, was ein Zählgatter dieses Plans
	// nachweist. Ein Textbaustein hat ein eigenes Formular, auf dem sich ein
	// Pflichtfeld mit einer Begründung zurückweisen lässt — Pflicht bleibt
	// hier also bedeutungsvoll.
	//
	// Und keine Verengung der Feldarten: BlockKinds() lässt Verweis und
	// Schlagwort weg, weil ein Baustein beim Speichern der Seite zu HTML
	// erstarrt und beide beim nächsten Umbenennen still veralten würden. Die
	// Werte eines Textbausteins werden auf dem Weg nach draussen durch
	// field.Resolve aufgelöst, genau wie die einer Seite — dieser Grund reicht
	// also nicht hierher, und ein Textbaustein bietet field.Kinds vollständig
	// an, Gruppen eingeschlossen.
	//
	// Verengt wird zweierlei, und keines davon ist eine Feldart: die Bedingung
	// oben, aus dem dort genannten Grund, und gilt_fuer hier — „gilt für Seiten
	// / für Beiträge" hat an einem Textbaustein keinen Sinn, denn ein
	// Textbaustein ist keine Seite und gehört zu keiner Inhaltsart.
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
//
// Die Obergrenze ist maxKeyBytes und damit dieselbe Zahl, bei der SlugifyKey
// abschneidet: eine Kennung, die die Ableitung erzeugt hat, muss diese Prüfung
// bestehen.
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
