package value

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Value is one named text of one website.
type Value struct {
	ID        int64
	WebsiteID int64
	// Key is what a page writes between [[wert: and ]], e.g.
	// "version-holzcloud-cms". It cannot be changed after creation: renaming a
	// key in use would silently empty every sentence that names it.
	Key string
	// Text is the one line that stands in the key's place.
	Text      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store handles SQL operations for values. Every statement carries the website
// id, so no method can read or write another website's value (D-03).
type Store struct {
	DB *db.DB
}

// NewStore creates a value store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

const columns = `id, website_id, key, value, created_at, updated_at`

func scan(row interface{ Scan(...any) error }) (*Value, error) {
	var v Value
	var createdAt, updatedAt string
	if err := row.Scan(&v.ID, &v.WebsiteID, &v.Key, &v.Text, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	v.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	v.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &v, nil
}

// List returns the values of a website, by key.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Value, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM site_values WHERE website_id = $1 ORDER BY key`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list values: %w", err)
	}
	defer rows.Close()
	var out []Value
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan value: %w", err)
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// Get returns one value of one website by id, or nil when there is none — also
// when the id belongs to another website, which is the same answer on purpose:
// the website is part of the lookup, not a comparison a caller has to remember
// (snippet.Store.Get says why).
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Value, error) {
	v, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM site_values WHERE id = $1 AND website_id = $2`, id, websiteID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get value: %w", err)
	}
	return v, nil
}

// GetByKey returns one value of one website by key, or nil.
func (s *Store) GetByKey(ctx context.Context, websiteID int64, key string) (*Value, error) {
	v, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM site_values WHERE website_id = $1 AND key = $2`, websiteID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get value: %w", err)
	}
	return v, nil
}

// Create stores a new value. A key the website already has is ErrKeyTaken.
func (s *Store) Create(ctx context.Context, websiteID int64, key, text string) (*Value, error) {
	if err := CheckKey(key); err != nil {
		return nil, err
	}
	text, err := CleanText(text)
	if err != nil {
		return nil, err
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO site_values (website_id, key, value) VALUES ($1, $2, $3)`, websiteID, key, text)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrKeyTaken
		}
		return nil, fmt.Errorf("create value: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, websiteID, id)
}

// UpdateText changes the text of a value. The key stays: renaming a key in use
// would silently empty every sentence that names it (D-13, A5).
//
// updated_at moves only when the text differs, for the reason Set gives.
func (s *Store) UpdateText(ctx context.Context, websiteID, id int64, text string) error {
	text, err := CleanText(text)
	if err != nil {
		return err
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE site_values SET
		   updated_at = CASE WHEN value = $1 THEN updated_at
		                     ELSE strftime('%Y-%m-%dT%H:%M:%SZ', 'now') END,
		   value = $1
		 WHERE id = $2 AND website_id = $3`, text, id, websiteID)
	if err != nil {
		return fmt.Errorf("update value: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a value nobody names any more.
//
// A value a page or a snippet still names is refused with an *InUseError that
// names them (D-02): deleting it would empty every sentence that says it, and
// the editor would learn of it from a visitor. Drafts and every language count;
// a page in the wastebasket does not, the rule media follows. There is no force
// flag — the editor removes the placeholders first.
//
// "Names" means what delivery resolves, column for column with usage: the
// page's source (Markdown, blocks), its stored HTML and its excerpt, and a
// snippet's source and HTML. The source alone missed a typed excerpt and a
// marker the Markdown wrote with escaped brackets, both of which a visitor
// reads as the value (WR-02 of the phase 33 review).
//
// The check and the delete are one statement, so a page saved between a check
// and a delete cannot slip through. The search is instr, never a pattern match:
// an underscore in a key is a wildcard to LIKE, and a_b would be kept alive by
// a page that names axb.
func (s *Store) Delete(ctx context.Context, websiteID int64, key string) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM site_values WHERE website_id = $1 AND key = $2
		   AND NOT EXISTS (SELECT 1 FROM pages
		         WHERE website_id = $1 AND deleted_at IS NULL
		           AND (instr(content_markdown, '[[wert:' || $2 || ']]') > 0
		                OR instr(COALESCE(blocks, ''), '[[wert:' || $2 || ']]') > 0
		                OR instr(content_html, '[[wert:' || $2 || ']]') > 0
		                OR instr(excerpt, '[[wert:' || $2 || ']]') > 0))
		   AND NOT EXISTS (SELECT 1 FROM snippets
		         WHERE website_id = $1
		           AND (instr(content_markdown, '[[wert:' || $2 || ']]') > 0
		                OR instr(content_html, '[[wert:' || $2 || ']]') > 0))`,
		websiteID, key)
	if err != nil {
		return fmt.Errorf("delete value: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	existing, err := s.GetByKey(ctx, websiteID, key)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	used, err := s.usage(ctx, websiteID)
	if err != nil {
		return err
	}
	inUse := &InUseError{}
	for _, u := range used[key] {
		inUse.Users = append(inUse.Users, u.Name())
	}
	return inUse
}

// Use is one page or snippet that names a value.
type Use struct {
	// Kind is "page" or "snippet".
	Kind string
	ID   int64
	// Title is the page's title or the snippet's name.
	Title string
	// Slug is the page's slug or the snippet's key.
	Slug string
	// Locale is the page's language, empty for the main language and for a
	// snippet.
	Locale string
	// Status is the page's status, empty for a snippet.
	Status string
}

// Name is how a refusal and a report call the user: a page by its title, with
// its language in brackets when it is not the main one, and a snippet by its
// name and the marker that places it, which is what an editor searches for.
func (u Use) Name() string {
	if u.Kind == "snippet" {
		return u.Title + " [[snippet:" + u.Slug + "]]"
	}
	if u.Locale != "" {
		return u.Title + " (" + u.Locale + ")"
	}
	return u.Title
}

// Outside is a marker standing where values are not resolved: a page title, a
// meta description, a menu item (D-11). It prints as written there, so the
// report names it.
type Outside struct {
	Key string
	// Where is "title", "description" or "menu".
	Where string
	// ID is the page's id, or the menu item's.
	ID int64
	// Title is the page's title or the menu item's.
	Title string
	// Locale is the page's or the menu's language, empty for the main one.
	Locale string
}

// Report is what a website does with its values.
type Report struct {
	// Used lists, per key, every page and snippet that names it — defined
	// keys and undefined ones alike. Pages come first, then snippets.
	Used map[string][]Use
	// Undefined are the keys named somewhere and defined nowhere, sorted.
	Undefined []string
	// Outside are the markers in places values do not reach.
	Outside []Outside
}

// Report reads every page, snippet and menu item of one website for value
// markers. Pages in the wastebasket are not read.
func (s *Store) Report(ctx context.Context, websiteID int64) (Report, error) {
	used, err := s.usage(ctx, websiteID)
	if err != nil {
		return Report{}, err
	}
	texts, _, err := s.Texts(ctx, websiteID)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Used: used}
	for key := range used {
		if _, ok := texts[key]; !ok {
			rep.Undefined = append(rep.Undefined, key)
		}
	}
	sort.Strings(rep.Undefined)
	if rep.Outside, err = s.outside(ctx, websiteID); err != nil {
		return Report{}, err
	}
	return rep, nil
}

// usage maps every key a website's pages and snippets name to the pages and
// snippets naming it.
//
// "Name" means what delivery resolves (WR-02 of the phase 33 review), so four
// columns of a page are read and two of a snippet:
//
//   - content_markdown and blocks, the source. JSON does not escape a bracket,
//     so a marker in any block field stands there literally.
//   - content_html, what delivery actually resolves. A marker the Markdown
//     wrote with escaped brackets, \[\[wert:k\]\], is not in the source in its
//     literal spelling, but it is in the HTML — and the visitor reads the value
//     there. Reading the HTML is also what catches it in a snippet.
//   - excerpt, which delivery resolves whether it was derived or typed. A
//     typed one need not repeat anything the text says.
//
// Keys takes the distinct keys of all of them at once, so a page that names a
// key in three columns is one user. The rows are narrowed with instr on the
// prefix and the keys taken out in Go, so no SQL wildcard ever meets a key.
//
// Pages are ordered by language — the main language first — then by title;
// snippets by name. That is the order a refusal names them in.
func (s *Store) usage(ctx context.Context, websiteID int64) (map[string][]Use, error) {
	used := map[string][]Use{}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, title, slug, locale, status, content_markdown, COALESCE(blocks, ''), content_html, excerpt
		   FROM pages
		  WHERE website_id = $1 AND deleted_at IS NULL
		    AND (instr(content_markdown, '[[wert:') > 0 OR instr(COALESCE(blocks, ''), '[[wert:') > 0
		         OR instr(content_html, '[[wert:') > 0 OR instr(excerpt, '[[wert:') > 0)
		  ORDER BY locale, title, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("value usage in pages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var u Use
		var markdown, blocks, html, excerpt string
		if err := rows.Scan(&u.ID, &u.Title, &u.Slug, &u.Locale, &u.Status, &markdown, &blocks, &html, &excerpt); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		u.Kind = "page"
		for _, key := range Keys(markdown + "\n" + blocks + "\n" + html + "\n" + excerpt) {
			used[key] = append(used[key], u)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	srows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, name, key, content_markdown, content_html FROM snippets
		  WHERE website_id = $1
		    AND (instr(content_markdown, '[[wert:') > 0 OR instr(content_html, '[[wert:') > 0)
		  ORDER BY name, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("value usage in snippets: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		u := Use{Kind: "snippet"}
		var markdown, html string
		if err := srows.Scan(&u.ID, &u.Title, &u.Slug, &markdown, &html); err != nil {
			return nil, fmt.Errorf("scan snippet: %w", err)
		}
		for _, key := range Keys(markdown + "\n" + html) {
			used[key] = append(used[key], u)
		}
	}
	return used, srows.Err()
}

// outside finds markers in page titles, meta descriptions and menu items of one
// website — the places D-11 leaves unresolved.
func (s *Store) outside(ctx context.Context, websiteID int64) ([]Outside, error) {
	var out []Outside
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, title, locale, meta_description FROM pages
		  WHERE website_id = $1 AND deleted_at IS NULL
		    AND (instr(title, '[[wert:') > 0 OR instr(meta_description, '[[wert:') > 0)
		  ORDER BY locale, title, id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("values outside page text: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title, locale, description string
		if err := rows.Scan(&id, &title, &locale, &description); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		for _, key := range Keys(title) {
			out = append(out, Outside{Key: key, Where: "title", ID: id, Title: title, Locale: locale})
		}
		for _, key := range Keys(description) {
			out = append(out, Outside{Key: key, Where: "description", ID: id, Title: title, Locale: locale})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	mrows, err := s.DB.Read.QueryContext(ctx,
		`SELECT i.id, i.title, i.url, m.locale FROM menu_items i
		   JOIN menus m ON m.id = i.menu_id
		  WHERE m.website_id = $1
		    AND (instr(i.title, '[[wert:') > 0 OR instr(i.url, '[[wert:') > 0)
		  ORDER BY m.locale, i.title, i.id`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("values in menus: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var id int64
		var title, link, locale string
		if err := mrows.Scan(&id, &title, &link, &locale); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		for _, key := range Keys(title + "\n" + link) {
			out = append(out, Outside{Key: key, Where: "menu", ID: id, Title: title, Locale: locale})
		}
	}
	return out, mrows.Err()
}

// isUniqueViolation reports a broken UNIQUE constraint, the way
// internal/snippet recognises it.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

// Set creates the value of a key or changes its text, and reports whether it
// created it. This is what a release step calls: it does not care whether the
// value existed yet.
//
// updated_at moves only when the text differs. Setting the version a release
// already set is no change, and the public side's caching validator should not
// say there was one.
func (s *Store) Set(ctx context.Context, websiteID int64, key, text string) (*Value, bool, error) {
	if err := CheckKey(key); err != nil {
		return nil, false, err
	}
	text, err := CleanText(text)
	if err != nil {
		return nil, false, err
	}
	// Whether the row existed is asked inside the same transaction as the
	// upsert, on the one write connection, so the answer belongs to this write
	// and not to one that slipped in between.
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	defer tx.Rollback()
	var existing int64
	err = tx.QueryRowContext(ctx,
		`SELECT count(*) FROM site_values WHERE website_id = $1 AND key = $2`, websiteID, key).Scan(&existing)
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO site_values (website_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (website_id, key) DO UPDATE SET
		   value = excluded.value,
		   updated_at = CASE WHEN site_values.value = excluded.value THEN site_values.updated_at
		                     ELSE strftime('%Y-%m-%dT%H:%M:%SZ', 'now') END`,
		websiteID, key, text); err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	v, err := scan(tx.QueryRowContext(ctx,
		`SELECT `+columns+` FROM site_values WHERE website_id = $1 AND key = $2`, websiteID, key))
	if err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("set value: %w", err)
	}
	return v, existing == 0, nil
}

// Texts returns every text of one website by key, and the newest updated_at
// among them — the public side's caching validator for a page that names a
// value. The zero time means the website has none.
func (s *Store) Texts(ctx context.Context, websiteID int64) (map[string]string, time.Time, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT key, value, updated_at FROM site_values WHERE website_id = $1`, websiteID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("load values: %w", err)
	}
	defer rows.Close()
	texts := map[string]string{}
	var newest time.Time
	for rows.Next() {
		var key, text, updatedAt string
		if err := rows.Scan(&key, &text, &updatedAt); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan value: %w", err)
		}
		texts[key] = text
		if at, _ := time.Parse(timeLayout, updatedAt); at.After(newest) {
			newest = at
		}
	}
	return texts, newest, rows.Err()
}
