// Package snippet stores reusable blocks of text — the address, the opening
// hours, the contact box.
//
// These are the parts of a site that change most often and that otherwise get
// copied into every page, so that changing the phone number means editing eight
// pages and missing one.
package snippet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// ErrNotFound is returned when a snippet does not exist.
var ErrNotFound = errors.New("snippet not found")

// ErrKeyTaken is returned when a key is already used by another snippet.
var ErrKeyTaken = errors.New("another snippet already uses that key")

// Snippet is one reusable block.
type Snippet struct {
	ID        int64
	WebsiteID int64
	// Key is what a page or a theme refers to, e.g. "oeffnungszeiten".
	Key             string
	Name            string
	ContentMarkdown string
	ContentHTML     string
	// Fields holds what field.Encode writes for this snippet's own fields. The
	// empty string means nothing has been filled in — the same thing it means
	// on a page, and what every snippet carried before migration 00047.
	Fields    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store handles SQL operations for snippets.
type Store struct {
	DB *db.DB
}

// NewStore creates a snippet store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

const columns = `id, website_id, key, name, content_markdown, content_html, created_at, updated_at, fields`

func scan(row interface{ Scan(...any) error }) (*Snippet, error) {
	var s Snippet
	var createdAt, updatedAt string
	if err := row.Scan(&s.ID, &s.WebsiteID, &s.Key, &s.Name,
		&s.ContentMarkdown, &s.ContentHTML, &createdAt, &updatedAt, &s.Fields); err != nil {
		return nil, err
	}
	s.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	s.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &s, nil
}

// List returns the snippets of a website, by name.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Snippet, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+columns+` FROM snippets WHERE website_id = $1 ORDER BY name`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list snippets: %w", err)
	}
	defer rows.Close()

	var out []Snippet
	for rows.Next() {
		sn, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan snippet: %w", err)
		}
		out = append(out, *sn)
	}
	return out, rows.Err()
}

// Get returns one snippet of one website by ID, or nil.
//
// The website belongs in the lookup and not in a comparison the caller makes
// afterwards. It used to take the id alone, and every one of the four callers
// remembered to check sn.WebsiteID for itself — internal/admin/field.go went as
// far as centralising its own comparison into snippetOf and writing down why:
// "snippets.Get takes only an id ... that asymmetry is the whole reason this is
// one function rather than two inline comparisons that can drift apart."
//
// It was right, and this is the fix it was asking for. Four callers that each
// remember are one forgetful fifth away from the shape this codebase has now
// shipped four times: a second id out of an address that the access middleware
// never sees.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Snippet, error) {
	sn, err := scan(s.DB.Read.QueryRowContext(ctx,
		`SELECT `+columns+` FROM snippets WHERE id = $1 AND website_id = $2`, id, websiteID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get snippet: %w", err)
	}
	return sn, nil
}

// Create inserts a snippet.
func (s *Store) Create(ctx context.Context, websiteID int64, key, name, markdown, html string) (*Snippet, error) {
	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO snippets (website_id, key, name, content_markdown, content_html)
		 VALUES ($1, $2, $3, $4, $5)`, websiteID, key, name, markdown, html)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrKeyTaken
		}
		return nil, fmt.Errorf("create snippet: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, websiteID, id)
}

// Update stores a changed snippet.
func (s *Store) Update(ctx context.Context, websiteID, id int64, key, name, markdown, html string) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE snippets SET key = $1, name = $2, content_markdown = $3, content_html = $4,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $5 AND website_id = $6`,
		key, name, markdown, html, id, websiteID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrKeyTaken
		}
		return fmt.Errorf("update snippet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFields replaces a snippet's own-field values.
//
// Separate from Update rather than a fifth positional parameter, for the same
// reasons page.Store.SetFields is: it makes no revision, needs no version
// token, and the value is written whole rather than merged. Update already
// takes four strings in a row; a fifth would be a trap the compiler cannot see.
//
// Unlike page.Store.SetFields this version pulls updated_at along, and the
// reason hangs on this carrier and not on the likeness of the two:
// Rendered.LatestUpdate is the check value for *every* page of the website
// (contentModTime, internal/public/pagedata.go), and since phase 8 the field
// values of a snippet belong to what a page shows. No such check value hangs
// on a page, which is why the model does not carry here.
//
// Without this line it would only go well because handleSnippetSave calls
// Update first — an order between two functions in two packages that nothing
// holds fast.
func (s *Store) SetFields(ctx context.Context, websiteID, id int64, raw string) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE snippets SET fields = $1,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $2 AND website_id = $3`,
		raw, id, websiteID)
	if err != nil {
		return fmt.Errorf("set snippet fields: %w", err)
	}
	// Reported and not swallowed, unlike before. A write that matched nothing
	// is either a snippet that has been deleted or one of another website, and
	// both are worth an error rather than a silent success.
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a snippet. Pages referring to it keep their marker, which then
// renders as nothing — deliberately, so the reference stays visible in the
// source rather than silently becoming stale text.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM snippets WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("delete snippet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Rendered is the expansion map for one website: key to sanitised HTML.
type Rendered struct {
	HTML map[string]template.HTML
	// Fields is the raw stored field JSON per snippet key — what field.Decode
	// reads. Raw and not resolved: resolving needs the definitions and the
	// lookups, and both live one layer out, in internal/public.
	Fields map[string]string
	// IDs is the snippet id per key, so a caller can ask the field store for
	// that snippet's definitions without a second lookup by key.
	IDs map[string]int64
	// LatestUpdate is the newest change across all snippets. A page's
	// Last-Modified has to account for it, or a conditional request answers 304
	// with the old opening hours baked in.
	LatestUpdate time.Time
}

// LoadRendered returns everything needed to expand markers and to fill
// Site.Snippets for a theme, in one query.
//
// Still one query for the whole website after the two new columns: this runs on
// every public render, and one round trip per snippet would be a round trip per
// snippet on every page a visitor asks for.
func (s *Store) LoadRendered(ctx context.Context, websiteID int64) (Rendered, error) {
	out := Rendered{
		HTML:   map[string]template.HTML{},
		Fields: map[string]string{},
		IDs:    map[string]int64{},
	}

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, key, content_html, updated_at, fields FROM snippets WHERE website_id = $1`, websiteID)
	if err != nil {
		return out, fmt.Errorf("load snippets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var key, html, updatedAt, fields string
		if err := rows.Scan(&id, &key, &html, &updatedAt, &fields); err != nil {
			return out, fmt.Errorf("scan snippet: %w", err)
		}
		// The cast is safe for exactly the reason page content is: content_html
		// went through goldmark and then bluemonday before it was stored.
		out.HTML[key] = template.HTML(html)
		out.Fields[key] = fields
		out.IDs[key] = id
		if t, err := time.Parse(timeLayout, updatedAt); err == nil && t.After(out.LatestUpdate) {
			out.LatestUpdate = t
		}
	}
	return out, rows.Err()
}

// markerPattern matches [[snippet:key]] in rendered page HTML.
//
// goldmark passes the marker through as plain text and bluemonday does not
// touch text, so it survives the render pipeline intact.
var markerPattern = regexp.MustCompile(`\[\[snippet:([a-z0-9][a-z0-9_-]*)\]\]`)

// paragraphMarker matches a marker that is the whole content of a paragraph.
//
// A marker on its own line comes out of goldmark as <p>[[snippet:x]]</p>, and a
// snippet is usually block content — a heading, a list, several paragraphs.
// Substituting inside the <p> would nest block elements inside a paragraph,
// which browsers repair by closing the <p> early and scattering the rest.
var paragraphMarker = regexp.MustCompile(`<p>\s*\[\[snippet:([a-z0-9][a-z0-9_-]*)\]\]\s*</p>`)

// Expand replaces the markers in rendered page HTML.
//
// This happens at render time, not on save. Baking the expansion into
// content_html would freeze a copy of the text into every page, which is
// exactly the thing snippets exist to avoid.
//
// A marker with no matching snippet expands to nothing rather than being left
// visible: a visitor should not see the internal syntax on a live page.
func Expand(html string, snippets map[string]template.HTML) string {
	if !strings.Contains(html, "[[snippet:") {
		return html
	}
	// A marker on its own line replaces the paragraph around it, so block
	// content lands at the same level as the rest of the page.
	html = paragraphMarker.ReplaceAllStringFunc(html, func(match string) string {
		return string(snippets[paragraphMarker.FindStringSubmatch(match)[1]])
	})
	// Anything left is an inline marker in the middle of a sentence.
	return markerPattern.ReplaceAllStringFunc(html, func(match string) string {
		return string(snippets[markerPattern.FindStringSubmatch(match)[1]])
	})
}

// UsedKeys returns the snippet keys a page's Markdown refers to.
func UsedKeys(markdown string) []string {
	matches := markerPattern.FindAllStringSubmatch(markdown, -1)
	seen := map[string]bool{}
	var keys []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			keys = append(keys, m[1])
		}
	}
	return keys
}

// CountUsage reports on how many live pages a snippet key is referenced.
func (s *Store) CountUsage(ctx context.Context, websiteID int64, key string) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages
		 WHERE website_id = $1 AND deleted_at IS NULL AND content_markdown LIKE '%' || $2 || '%'`,
		websiteID, "[[snippet:"+key+"]]").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count snippet usage: %w", err)
	}
	return n, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
