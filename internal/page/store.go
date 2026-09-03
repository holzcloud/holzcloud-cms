package page

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// Store handles SQL operations for pages.
type Store struct {
	DB *db.DB
}

// NewStore creates a new page store.
func NewStore(database *db.DB) *Store {
	return &Store{DB: database}
}

// scanPage reads one page row. extra receives destinations for any columns a
// caller appended to the projection, such as the search snippet.
func scanPage(row interface{ Scan(...any) error }, extra ...any) (*Page, error) {
	var p Page
	var publishedAt, deletedAt, publishAt, unpublishAt sql.NullString
	var createdBy, updatedBy, featuredMediaID, translationOf sql.NullInt64
	var createdAt, updatedAt string
	var noindex int
	dest := []any{
		&p.ID, &p.WebsiteID, &p.Title, &p.Slug,
		&p.ContentMarkdown, &p.ContentHTML, &p.Status,
		&publishedAt, &createdAt, &updatedAt,
		&p.Version, &createdBy, &updatedBy, &deletedAt,
		&p.Excerpt, &p.MetaDescription, &featuredMediaID, &noindex,
		&publishAt, &unpublishAt, &p.ReviewState, &p.Kind,
		&p.Access, &p.AccessPassword, &p.AccessHint, &p.Blocks, &p.Fields,
		&p.Locale, &translationOf, &p.TypeKey,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	if translationOf.Valid {
		p.TranslationOf = translationOf.Int64
	}
	p.PublishAt = parseOptionalTime(publishAt)
	p.UnpublishAt = parseOptionalTime(unpublishAt)
	p.NoIndex = noindex == 1
	if featuredMediaID.Valid {
		p.FeaturedMediaID = &featuredMediaID.Int64
	}
	if publishedAt.Valid {
		t, _ := time.Parse(timeLayout, publishedAt.String)
		p.PublishedAt = &t
	}
	if deletedAt.Valid {
		t, _ := time.Parse(timeLayout, deletedAt.String)
		p.DeletedAt = &t
	}
	if createdBy.Valid {
		p.CreatedBy = &createdBy.Int64
	}
	if updatedBy.Valid {
		p.UpdatedBy = &updatedBy.Int64
	}
	p.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	p.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
	return &p, nil
}

const pageColumns = `id, website_id, title, slug, content_markdown, content_html, status, published_at, created_at, updated_at, version, created_by, updated_by, deleted_at, excerpt, meta_description, featured_media_id, noindex, publish_at, unpublish_at, review_state, kind, access, access_password, access_hint, blocks, fields, locale, translation_of, art`

// ColumnsFor exposes the page projection to packages that join against pages,
// so a column added here reaches their queries too rather than scanning as a
// zero value in one place and not the other.
func ColumnsFor(alias string) string {
	if alias == "" {
		return pageColumns
	}
	parts := strings.Split(pageColumns, ", ")
	for i, c := range parts {
		parts[i] = alias + "." + c
	}
	return strings.Join(parts, ", ")
}

// ScanRows reads a result set produced with ColumnsFor.
func ScanRows(rows *sql.Rows) ([]Page, error) {
	var out []Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// LivePredicate is the condition that separates live content from the trash.
//
// It lives in one place so a query added later cannot forget it: leaving it out
// would resurrect a deleted page on the public site.
var LivePredicate = LivePredicateFor("")

// PublicPredicate is everything that has to be true for a page to be shown to a
// visitor: published, inside its schedule, not in the trash.
//
// Scheduling is a read-time predicate rather than a background job that flips
// the status. There is no tick to miss, no clock drift, and a machine that was
// switched off across the publication moment comes up already correct.
//
// Every query that serves the public must use this — the conditions used to be
// spelled out in five separate places, and a sixth would simply have been
// forgotten.
var PublicPredicate = PublicPredicateFor("")

// LivePredicateFor is LivePredicate qualified with a table alias, for queries
// that join pages against another table.
func LivePredicateFor(alias string) string {
	return ` AND ` + qualify(alias, "deleted_at") + ` IS NULL`
}

// ListablePredicate is PublicPredicate plus "not behind a password".
//
// A protected page is still served — that is what the gate is for — but it must
// never appear in a listing, a search result, a feed or a sitemap: the title and
// the excerpt are exactly what the password was meant to keep back, and a
// search index would hold them long after the page was taken down.
var ListablePredicate = ListablePredicateFor("")

// ListablePredicateFor is ListablePredicate qualified with a table alias.
func ListablePredicateFor(alias string) string {
	return PublicPredicateFor(alias) + ` AND ` + qualify(alias, "access") + ` = 'public'`
}

// PublicPredicateFor is PublicPredicate qualified with a table alias.
func PublicPredicateFor(alias string) string {
	now := `strftime('%Y-%m-%dT%H:%M:%SZ','now')`
	return ` AND ` + qualify(alias, "status") + ` = 'published'` +
		` AND (` + qualify(alias, "publish_at") + ` IS NULL OR ` + qualify(alias, "publish_at") + ` <= ` + now + `)` +
		` AND (` + qualify(alias, "unpublish_at") + ` IS NULL OR ` + qualify(alias, "unpublish_at") + ` > ` + now + `)` +
		LivePredicateFor(alias)
}

func qualify(alias, column string) string {
	if alias == "" {
		return column
	}
	return alias + "." + column
}

// ErrConflict is returned when a page was modified by someone else since the
// editor loaded it.
var ErrConflict = errors.New("page was modified by someone else")

// ErrSlugTaken is returned when a rename collides with an existing page.
var ErrSlugTaken = errors.New("another page already uses that address")

// SetTranslation links a page to the one it translates, or unlinks it.
//
// Separate from UpdatePage: it is not an edit of the content, it does not make
// a revision, and it must not need the version token — otherwise linking a
// translation would fail whenever somebody else had just saved.
func (s *Store) SetTranslation(ctx context.Context, id int64, loc string, of int64) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET locale = $1, translation_of = $2 WHERE id = $3`,
		loc, nullableID(nonZero(of)), id)
	if err != nil {
		return fmt.Errorf("set translation: %w", err)
	}
	return nil
}

// SetFields replaces a page's own-field values.
//
// Separate from UpdatePage for the same reasons as SetTranslation: it makes no
// revision and needs no version token. It exists for the import, where a
// reference field can only be resolved once every page of the bundle has been
// created and has an id — the same second pass the translation links need.
func (s *Store) SetFields(ctx context.Context, id int64, raw string) error {
	_, err := s.DB.Write.ExecContext(ctx, `UPDATE pages SET fields = $1 WHERE id = $2`, raw, id)
	if err != nil {
		return fmt.Errorf("set fields: %w", err)
	}
	return nil
}

// ListFilter narrows and orders the admin page list.
type ListFilter struct {
	// Locale narrows the list to one language. Empty means the main language;
	// "*" means all of them, which is what somebody wants who is looking for a
	// page and does not remember which language it was in.
	Locale string

	// Status is "", "draft" or "published". The filter was implemented from the
	// start and unreachable: the handler always passed an empty string.
	Status string
	// Review is "pending" to show only drafts awaiting review.
	Review string
	// Sort is "updated_at", "created_at" or "title".
	Sort string
	// Kind is "page" or "post", or empty for both. The admin lists them
	// separately because they are managed differently: a page belongs in a
	// menu, a post belongs in an archive.
	Kind string
	// TypeKey narrows to one of the website's own kinds. It wins over Kind:
	// somebody filtering for "Produkte" means the products, not the pages they
	// technically are.
	TypeKey       string
	Page, PerPage int
}

// orderClause maps the sort choice to SQL. It is a whitelist because the value
// cannot be a bound parameter and must never be interpolated from user input.
func (f ListFilter) orderClause() string {
	switch f.Sort {
	case "created_at":
		return ` ORDER BY created_at DESC`
	case "title":
		return ` ORDER BY title COLLATE NOCASE ASC`
	default:
		// Most recently worked on first: that is what someone opening the list
		// is almost always looking for.
		return ` ORDER BY updated_at DESC`
	}
}

// ListPages returns one page of the admin list plus the total count.
func (s *Store) ListPages(ctx context.Context, websiteID int64, f ListFilter) ([]Page, int, error) {
	if f.PerPage <= 0 {
		f.PerPage = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PerPage

	where := ` WHERE website_id = $1` + LivePredicate
	args := []any{websiteID}
	if f.Status == "draft" || f.Status == "published" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if f.Review == "pending" {
		where += " AND review_state = 'pending'"
	}
	// Eine eigene Art gewinnt über die eingebaute: wer nach "Produkte" filtert,
	// meint die Produkte und nicht die Seiten, unter denen sie wohnen.
	if f.TypeKey != "" {
		args = append(args, f.TypeKey)
		where += fmt.Sprintf(" AND art = $%d", len(args))
	} else if f.Kind == KindPage || f.Kind == KindPost {
		args = append(args, f.Kind)
		// Und umgekehrt: "Seiten" heisst die Seiten, nicht die Produkte, die
		// technisch ebenfalls Seiten sind.
		where += fmt.Sprintf(" AND kind = $%d AND art = ''", len(args))
	}
	// "*" is every language; anything else is exactly that one, and the empty
	// string is the main language — which is what a website with one language
	// asks for without knowing it.
	if f.Locale != "*" {
		args = append(args, f.Locale)
		where += fmt.Sprintf(" AND locale = $%d", len(args))
	}

	var total int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pages: %w", err)
	}

	query := `SELECT ` + pageColumns + ` FROM pages` + where + f.orderClause() +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, f.PerPage, offset)

	rows, err := s.DB.Read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan page: %w", err)
		}
		pages = append(pages, *p)
	}
	return pages, total, rows.Err()
}

// GetPage returns a single page by ID.
func (s *Store) GetPage(ctx context.Context, id int64) (*Page, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE id = $1`, id)
	p, err := scanPage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}
	return p, nil
}

// GetPublishedPage returns a published page by website ID and slug.
// This is the ONLY function public handlers should use for page retrieval.
// It always enforces status = 'published' to prevent draft leakage.
func (s *Store) GetPublishedPage(ctx context.Context, websiteID int64, slug string) (*Page, error) {
	return s.GetPublishedPageIn(ctx, websiteID, "", slug)
}

// GetPublishedPageIn is the same for one language.
//
// The language is part of the query and not checked afterwards: /fr/kontakt
// must not find the German page just because no French one exists. A missing
// translation is a 404 in that language, which is honest — and the language
// switcher only ever offers what is really there.
func (s *Store) GetPublishedPageIn(ctx context.Context, websiteID int64, loc, slug string) (*Page, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $2 AND slug = $3`+PublicPredicate,
		websiteID, loc, slug)
	p, err := scanPage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get published page: %w", err)
	}
	return p, nil
}

// Translations returns the published pages of one translation group, the main
// language first.
//
// Given any page of the group: the star's middle is either the page itself or
// what it points at, and from there every arm is one query.
func (s *Store) Translations(ctx context.Context, p *Page) ([]Page, error) {
	if p == nil {
		return nil, nil
	}
	mitte := p.ID
	if p.TranslationOf != 0 {
		mitte = p.TranslationOf
	}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pageColumns+` FROM pages
		 WHERE (id = $1 OR translation_of = $1)`+PublicPredicate+`
		 ORDER BY locale`, mitte)
	if err != nil {
		return nil, fmt.Errorf("translations: %w", err)
	}
	defer rows.Close()

	var out []Page
	for rows.Next() {
		t, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan translation: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// TranslationsForEditor is Translations including drafts, for the admin.
//
// A separate method and not a flag: the public one must never be able to leak a
// draft, and the way to be sure of that is that it has no such switch.
func (s *Store) TranslationsForEditor(ctx context.Context, p *Page) ([]Page, error) {
	if p == nil {
		return nil, nil
	}
	mitte := p.ID
	if p.TranslationOf != 0 {
		mitte = p.TranslationOf
	}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pageColumns+` FROM pages
		 WHERE (id = $1 OR translation_of = $1) AND deleted_at IS NULL
		 ORDER BY locale`, mitte)
	if err != nil {
		return nil, fmt.Errorf("translations: %w", err)
	}
	defer rows.Close()

	var out []Page
	for rows.Next() {
		t, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan translation: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// maxSlugAttempts bounds the slug-uniquifying retry loop. Without a bound, a
// UNIQUE violation on any column other than slug would retry forever.
const maxSlugAttempts = 100

// PageCreate carries everything a new page needs.
type PageCreate struct {
	WebsiteID int64
	Title     string
	Slug      string
	Markdown  string
	HTML      string
	Status    string
	// Blocks is the encoded block list, empty for a page written in Markdown.
	Blocks string
	// Fields are the website's own fields as JSON, empty when none are filled.
	Fields string

	// Locale is the language this page is written in. Empty is the website's
	// main language, which is what every page was until somebody added a second.
	Locale string
	// TranslationOf names the page in the main language this one translates.
	// Zero for a page that stands on its own.
	TranslationOf int64

	Meta     PageMeta
	Schedule PageSchedule

	// Kind is "page" or "post"; anything else becomes a page.
	Kind string
	// TypeKey is the website's own content kind, empty for the built-in two.
	TypeKey string

	// UserID is recorded as the author.
	UserID *int64
}

// PageSchedule bounds when a published page is actually visible.
type PageSchedule struct {
	PublishAt   *time.Time
	UnpublishAt *time.Time
}

// PageMeta are the search-engine and preview fields of a page. They travel as
// one struct so create and update cannot drift apart on which ones they carry.
type PageMeta struct {
	Excerpt         string
	MetaDescription string
	FeaturedMediaID *int64
	NoIndex         bool
}

// CreatePage inserts a new page. If the slug already exists for this website,
// it appends -2, -3, etc. until unique.
//
// published_at is set here when the page starts out published. It used to be
// filled by a follow-up UpdatePage, which wrote a second row version and an
// empty revision for a page that had no history yet.
func (s *Store) CreatePage(ctx context.Context, c PageCreate) (*Page, error) {
	var publishedAt any
	if c.Status == "published" {
		publishedAt = time.Now().UTC().Format(timeLayout)
	}

	baseSlug := c.Slug
	slug := c.Slug
	for attempt := 1; attempt <= maxSlugAttempts; attempt++ {
		res, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO pages (website_id, title, slug, content_markdown, content_html, status, published_at,
			 created_by, updated_by, excerpt, meta_description, featured_media_id, noindex,
			 publish_at, unpublish_at, kind, blocks, fields, locale, translation_of, art)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
			c.WebsiteID, c.Title, slug, c.Markdown, c.HTML, c.Status, publishedAt, nullableID(c.UserID),
			c.Meta.Excerpt, c.Meta.MetaDescription, nullableID(c.Meta.FeaturedMediaID), boolToInt(c.Meta.NoIndex),
			nullableTime(c.Schedule.PublishAt), nullableTime(c.Schedule.UnpublishAt), NormalizeKind(c.Kind),
			c.Blocks, c.Fields, c.Locale, nullableID(nonZero(c.TranslationOf)), c.TypeKey)
		if err != nil {
			// Check for UNIQUE constraint violation on slug
			if isUniqueViolation(err) {
				slug = fmt.Sprintf("%s-%d", baseSlug, attempt+1)
				continue
			}
			return nil, fmt.Errorf("create page: %w", err)
		}
		id, _ := res.LastInsertId()
		return s.GetPage(ctx, id)
	}
	return nil, fmt.Errorf("create page: no free slug for %q after %d attempts", baseSlug, maxSlugAttempts)
}

// PageUpdate carries everything an edit changes.
//
// published_at is deliberately absent: it is derived in SQL from the status, so
// unpublishing and republishing cannot make an old page look newly written.
type PageUpdate struct {
	Title    string
	Slug     string
	Markdown string
	HTML     string
	Status   string
	// Blocks is the encoded block list, empty for a page written in Markdown.
	Blocks string
	// Fields are the website's own fields as JSON, empty when none are filled.
	Fields string

	Meta     PageMeta
	Schedule PageSchedule

	// Kind is "page" or "post". Changing it moves the record between the page
	// list and the archive without touching its address, so every existing link
	// keeps working.
	//
	// Empty means "leave the kind alone". Half the callers here save a page for
	// a reason that has nothing to do with its kind — inserting an image,
	// restoring an older text — and those must not have to know about kinds in
	// order not to destroy one. Everything that really classifies sends "page"
	// or "post".
	Kind string
	// TypeKey is the website's own content kind, empty for the built-in two.
	// It is only read when Kind is set, for the same reason.
	TypeKey string

	// ExpectedVersion is the version the editor loaded. A mismatch means
	// someone else saved in the meantime and the update is refused rather than
	// silently overwriting their work.
	ExpectedVersion int64

	// UserID is recorded as the author of the change and of the revision.
	UserID *int64
}

// maxRevisions is how many previous states of a page are kept. Old ones are
// pruned in the same transaction so the table cannot grow without bound on an
// SD card.
const maxRevisions = 20

// UpdatePage applies an edit under optimistic locking, recording the previous
// state as a revision.
//
// Everything happens in one transaction on the write pool: the pre-image is
// captured, the row is updated only if its version still matches, and old
// revisions are pruned. Returns ErrConflict when another save won the race and
// ErrSlugTaken when the new address is already in use.
func (s *Store) UpdatePage(ctx context.Context, id int64, u PageUpdate) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// Pre-image, which is the conflict check, the revision content and the
	// source address of any redirect this rename needs.
	var prevTitle, prevSlug, prevMarkdown, prevStatus, prevBlocks, prevKind, prevType string
	var prevVersion, websiteID int64
	err = tx.QueryRowContext(ctx,
		`SELECT title, slug, content_markdown, status, version, website_id, blocks, kind, art
		 FROM pages WHERE id = $1`, id).
		Scan(&prevTitle, &prevSlug, &prevMarkdown, &prevStatus, &prevVersion, &websiteID, &prevBlocks,
			&prevKind, &prevType)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read page for update: %w", err)
	}
	if u.ExpectedVersion != 0 && u.ExpectedVersion != prevVersion {
		return ErrConflict
	}

	// A publish toggle changes nothing an editor would want to compare, so it
	// must not fill the history with noise.
	contentChanged := prevTitle != u.Title || prevSlug != u.Slug ||
		prevMarkdown != u.Markdown || prevBlocks != u.Blocks
	if contentChanged {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO page_revisions (page_id, user_id, title, slug, content_markdown, status, blocks)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, nullableID(u.UserID), prevTitle, prevSlug, prevMarkdown, prevStatus, prevBlocks); err != nil {
			return fmt.Errorf("record revision: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM page_revisions WHERE page_id = $1 AND id NOT IN (
			     SELECT id FROM page_revisions WHERE page_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2
			 )`, id, maxRevisions); err != nil {
			return fmt.Errorf("prune revisions: %w", err)
		}
	}

	// A save that says nothing about the kind keeps the one the record has.
	// Without this, adding an image through the picker or restoring a revision
	// would turn a post into a page and strip an own kind — a reclassification
	// nobody asked for, on a path where nobody would look for it.
	kind, typeKey := NormalizeKind(u.Kind), u.TypeKey
	if u.Kind == "" {
		kind, typeKey = prevKind, prevType
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE pages SET title = $1, slug = $2, content_markdown = $3, content_html = $4,
		 status = $5,
		 published_at = CASE
		     WHEN $5 = 'published' AND published_at IS NULL
		         THEN strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		     ELSE published_at
		 END,
		 updated_by = $6, version = version + 1,
		 excerpt = $7, meta_description = $8, featured_media_id = $9, noindex = $10,
		 publish_at = $11, unpublish_at = $12, kind = $13, blocks = $14, fields = $15, art = $18,
		 -- Saving is an editorial act; it clears a pending review request.
		 review_state = 'none',
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE id = $16 AND version = $17`,
		u.Title, u.Slug, u.Markdown, u.HTML, u.Status, nullableID(u.UserID),
		u.Meta.Excerpt, u.Meta.MetaDescription, nullableID(u.Meta.FeaturedMediaID), boolToInt(u.Meta.NoIndex),
		nullableTime(u.Schedule.PublishAt), nullableTime(u.Schedule.UnpublishAt), kind,
		u.Blocks, u.Fields, id, prevVersion, typeKey)
	if err != nil {
		if isUniqueViolation(err) {
			// CreatePage uniquifies automatically, but a rename is an explicit
			// choice — silently changing the address the editor typed would be
			// worse than telling them it is taken.
			return ErrSlugTaken
		}
		return fmt.Errorf("update page: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}

	// In the same transaction as the rename: a link cannot break because the
	// save succeeded and the redirect did not.
	if err := recordSlugChange(ctx, tx, websiteID, prevSlug, u.Slug); err != nil {
		return err
	}

	return tx.Commit()
}

// ErrNotFound is returned when a page does not exist.
var ErrNotFound = errors.New("page not found")

// parseOptionalTime turns a nullable timestamp column into an optional time.
func parseOptionalTime(v sql.NullString) *time.Time {
	if !v.Valid {
		return nil
	}
	t, err := time.Parse(timeLayout, v.String)
	if err != nil {
		return nil
	}
	return &t
}

// nullableTime renders an optional time for a nullable column.
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(timeLayout)
}

// nonZero turns an id into a pointer, or nil for zero — which is what the
// column means: no translation, not "translation of page 0".
func nonZero(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdatePageTitle updates just the title and slug of a page (for inline editing).
//
// It bumps version like every other write, so an inline rename cannot slip past
// an open editor's optimistic lock unnoticed.
func (s *Store) UpdatePageTitle(ctx context.Context, id int64, title, slug string, userID *int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var prevSlug string
	var websiteID int64
	err = tx.QueryRowContext(ctx,
		`SELECT slug, website_id FROM pages WHERE id = $1`+LivePredicate, id).
		Scan(&prevSlug, &websiteID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read page for rename: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE pages SET title = $1, slug = $2, updated_by = $3, version = version + 1,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $4`+LivePredicate,
		title, slug, nullableID(userID), id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrSlugTaken
		}
		return fmt.Errorf("update page title: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}

	// This is the path where a live URL changes by accident: the inline editor
	// derives the slug from the title, so renaming "Kontakt" to "Kontakt &
	// Anfahrt" silently moves the page.
	if err := recordSlugChange(ctx, tx, websiteID, prevSlug, slug); err != nil {
		return err
	}
	return tx.Commit()
}

// SetPageStatus publishes or unpublishes a page.
//
// published_at is set on the first publish and never cleared afterwards:
// unpublishing and republishing must not make a page look newly written. The
// edit form used to clear it while this path preserved it, so the same action
// produced different data depending on which control was used.
func (s *Store) SetPageStatus(ctx context.Context, id int64, status string, userID *int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages
		 SET status = $1,
		     published_at = CASE
		         WHEN $1 = 'published' AND published_at IS NULL
		             THEN strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		         ELSE published_at
		     END,
		     updated_by = $2,
		     version = version + 1,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE id = $3`+LivePredicate,
		status, nullableID(userID), id)
	if err != nil {
		return fmt.Errorf("update page status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetReviewState marks a draft as awaiting review, or clears the mark.
//
// It is a separate column rather than a status value because status carries an
// inline CHECK on a STRICT table; widening it would mean rebuilding the table.
func (s *Store) SetReviewState(ctx context.Context, id int64, state string) error {
	if state != "pending" {
		state = "none"
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages SET review_state = $1, version = version + 1,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE id = $2`+LivePredicate, state, id)
	if err != nil {
		return fmt.Errorf("set review state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountPendingReview reports how many drafts are waiting to be looked at.
func (s *Store) CountPendingReview(ctx context.Context, websiteID int64) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pages WHERE website_id = $1 AND review_state = 'pending'`+LivePredicate,
		websiteID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pending review: %w", err)
	}
	return n, nil
}

// TrashPage moves a page to the trash instead of destroying it.
//
// The slug is rewritten on the way in. pages carries an inline
// UNIQUE(website_id, slug) that cannot be dropped without a full table rebuild,
// so a trashed page would otherwise keep blocking its own address and a new page
// could not take it over.
func (s *Store) TrashPage(ctx context.Context, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE pages
		 SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
		     slug = 'trash-' || id || '-' || slug,
		     version = version + 1
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("trash page: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RestorePage brings a page back from the trash, restoring its original slug.
// When that address has been taken in the meantime the slug is uniquified
// rather than the restore failing.
func (s *Store) RestorePage(ctx context.Context, id int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var websiteID int64
	var slug string
	err = tx.QueryRowContext(ctx,
		`SELECT website_id, slug FROM pages WHERE id = $1 AND deleted_at IS NOT NULL`, id).
		Scan(&websiteID, &slug)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read trashed page: %w", err)
	}

	original := strings.TrimPrefix(slug, fmt.Sprintf("trash-%d-", id))
	candidate := original
	for attempt := 1; attempt <= maxSlugAttempts; attempt++ {
		var taken int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pages WHERE website_id = $1 AND slug = $2 AND id != $3`,
			websiteID, candidate, id).Scan(&taken); err != nil {
			return fmt.Errorf("check slug: %w", err)
		}
		if taken == 0 {
			break
		}
		candidate = fmt.Sprintf("%s-%d", original, attempt+1)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE pages SET deleted_at = NULL, slug = $1, version = version + 1,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = $2`,
		candidate, id); err != nil {
		return fmt.Errorf("restore page: %w", err)
	}
	return tx.Commit()
}

// PurgePage destroys a trashed page for good.
func (s *Store) PurgePage(ctx context.Context, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM pages WHERE id = $1 AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("purge page: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTrash returns the trashed pages of a website, most recently deleted first.
func (s *Store) ListTrash(ctx context.Context, websiteID int64) ([]Page, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pageColumns+` FROM pages
		 WHERE website_id = $1 AND deleted_at IS NOT NULL
		 ORDER BY deleted_at DESC`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list trash: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan trashed page: %w", err)
		}
		// Show the address the page had before it was trashed.
		p.Slug = strings.TrimPrefix(p.Slug, fmt.Sprintf("trash-%d-", p.ID))
		pages = append(pages, *p)
	}
	return pages, rows.Err()
}

// TrashRetention is how long a soft-deleted page stays recoverable. It is
// exported so the admin UI can name the same number the purge job uses instead
// of hard-coding a second one that could drift.
const TrashRetention = 30 * 24 * time.Hour

// PurgeExpiredTrash destroys pages that have been in the trash longer than the
// retention period, and reports which media rows went with them so the caller
// can unlink the files.
func (s *Store) PurgeExpiredTrash(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(timeLayout)
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM pages WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge expired trash: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ListRevisions returns the stored history of a page, newest first.
func (s *Store) ListRevisions(ctx context.Context, pageID int64) ([]Revision, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT r.id, r.page_id, r.user_id, COALESCE(u.email, ''), r.title, r.slug,
		        r.content_markdown, r.status, r.created_at, r.blocks, r.label
		 FROM page_revisions r
		 LEFT JOIN users u ON u.id = r.user_id
		 WHERE r.page_id = $1
		 ORDER BY r.created_at DESC, r.id DESC`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list revisions: %w", err)
	}
	defer rows.Close()

	var revisions []Revision
	for rows.Next() {
		var r Revision
		var userID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&r.ID, &r.PageID, &userID, &r.UserEmail, &r.Title, &r.Slug,
			&r.ContentMarkdown, &r.Status, &createdAt, &r.Blocks, &r.Label); err != nil {
			return nil, fmt.Errorf("scan revision: %w", err)
		}
		if userID.Valid {
			r.UserID = &userID.Int64
		}
		r.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		revisions = append(revisions, r)
	}
	return revisions, rows.Err()
}

// GetRevision returns one revision, or nil when it does not exist.
func (s *Store) GetRevision(ctx context.Context, id int64) (*Revision, error) {
	var r Revision
	var userID sql.NullInt64
	var createdAt string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT r.id, r.page_id, r.user_id, COALESCE(u.email, ''), r.title, r.slug,
		        r.content_markdown, r.status, r.created_at, r.blocks, r.label
		 FROM page_revisions r
		 LEFT JOIN users u ON u.id = r.user_id
		 WHERE r.id = $1`, id).
		Scan(&r.ID, &r.PageID, &userID, &r.UserEmail, &r.Title, &r.Slug,
			&r.ContentMarkdown, &r.Status, &createdAt, &r.Blocks, &r.Label)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get revision: %w", err)
	}
	if userID.Valid {
		r.UserID = &userID.Int64
	}
	r.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &r, nil
}

// LabelRevision names one version, or clears the name when label is empty.
//
// It does not touch the page: naming a version is a note about the history and
// must not count as an edit, or looking through the history would keep adding
// to it.
func (s *Store) LabelRevision(ctx context.Context, id int64, label string) error {
	label = strings.TrimSpace(label)
	if len([]rune(label)) > RevisionLabelMax {
		label = string([]rune(label)[:RevisionLabelMax])
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE page_revisions SET label = $1 WHERE id = $2`, label, id)
	if err != nil {
		return fmt.Errorf("label revision: %w", err)
	}
	return nil
}

// TranslationCell is one page inside the translation overview.
type TranslationCell struct {
	ID     int64
	Title  string
	Status string
}

// TranslationRow is one page of the main language together with whatever
// translations of it exist. Missing languages are simply absent from ByLocale,
// which is the point of the whole screen.
type TranslationRow struct {
	ID     int64
	Title  string
	Slug   string
	Status string
	// ByLocale is keyed by language tag. The main language is keyed by the
	// empty string, the same way the column is stored.
	ByLocale map[string]TranslationCell
}

// TranslationMatrix lists every page of a website beside the languages it
// exists in.
//
// One query and a grouping, not a query per language: a website with six
// languages and two hundred pages would otherwise ask twelve hundred times for
// what fits in one answer.
//
// A translation whose original has been deleted keeps its own row rather than
// disappearing. It is still a page somebody can open, and a screen that hides
// it would be lying about what the website contains.
func (s *Store) TranslationMatrix(ctx context.Context, websiteID int64) ([]TranslationRow, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, title, slug, status, locale, translation_of
		 FROM pages
		 WHERE website_id = $1 AND deleted_at IS NULL
		 ORDER BY slug, locale`, websiteID)
	if err != nil {
		return nil, fmt.Errorf("translation matrix: %w", err)
	}
	defer rows.Close()

	type record struct {
		id            int64
		title, slug   string
		status, loc   string
		translationOf sql.NullInt64
	}
	var all []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.title, &rec.slug, &rec.status, &rec.loc, &rec.translationOf); err != nil {
			return nil, fmt.Errorf("scan translation row: %w", err)
		}
		all = append(all, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byRoot := map[int64]*TranslationRow{}
	var order []int64
	root := func(rec record) int64 {
		if rec.translationOf.Valid && rec.translationOf.Int64 != 0 {
			return rec.translationOf.Int64
		}
		return rec.id
	}

	// Zwei Durchgänge: erst die Originale, damit eine Übersetzung, die vor
	// ihrem Original einsortiert wurde, keine eigene Zeile anlegt.
	for _, rec := range all {
		if root(rec) != rec.id {
			continue
		}
		byRoot[rec.id] = &TranslationRow{
			ID: rec.id, Title: rec.title, Slug: rec.slug, Status: rec.status,
			ByLocale: map[string]TranslationCell{
				rec.loc: {ID: rec.id, Title: rec.title, Status: rec.status},
			},
		}
		order = append(order, rec.id)
	}
	for _, rec := range all {
		r := root(rec)
		if r == rec.id {
			continue
		}
		row, ok := byRoot[r]
		if !ok {
			// Die Übersetzung einer gelöschten Seite. Sie steht für sich.
			byRoot[rec.id] = &TranslationRow{
				ID: rec.id, Title: rec.title, Slug: rec.slug, Status: rec.status,
				ByLocale: map[string]TranslationCell{
					rec.loc: {ID: rec.id, Title: rec.title, Status: rec.status},
				},
			}
			order = append(order, rec.id)
			continue
		}
		row.ByLocale[rec.loc] = TranslationCell{ID: rec.id, Title: rec.title, Status: rec.status}
	}

	out := make([]TranslationRow, 0, len(order))
	for _, id := range order {
		out = append(out, *byRoot[id])
	}
	return out, nil
}

// GetPageBySlugIn returns a page by website ID, language and slug, regardless
// of status.
//
// Since an address is only unique within a language, this is the lookup that
// can answer without guessing: /holzcloud-cms exists in five languages and is
// five pages. GetPageBySlug below answers for whichever of them the database
// hands back first, which is right for "does this address exist at all" and
// wrong for "which page is this".
func (s *Store) GetPageBySlugIn(ctx context.Context, websiteID int64, loc, slug string) (*Page, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $2 AND slug = $3`+LivePredicate,
		websiteID, loc, slug)
	p, err := scanPage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page by slug in locale: %w", err)
	}
	return p, nil
}

// GetPageBySlug returns a page by website ID and slug regardless of status.
// Used for admin preview (drafts visible).
func (s *Store) GetPageBySlug(ctx context.Context, websiteID int64, slug string) (*Page, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND slug = $2`+LivePredicate, websiteID, slug)
	p, err := scanPage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page by slug: %w", err)
	}
	return p, nil
}

// ListPublishedForSitemap returns every published page of a website, oldest
// first, with only the fields a sitemap needs.
//
// It is a separate query from ListPages because a sitemap must not be paginated
// and does not need the page bodies, which are the bulk of a row.
func (s *Store) ListPublishedForSitemap(ctx context.Context, websiteID int64) ([]SitemapEntry, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT slug, updated_at, locale FROM pages
		 WHERE website_id = $1`+ListablePredicate+`
		 ORDER BY created_at ASC`,
		websiteID)
	if err != nil {
		return nil, fmt.Errorf("list pages for sitemap: %w", err)
	}
	defer rows.Close()

	var entries []SitemapEntry
	for rows.Next() {
		var e SitemapEntry
		var updatedAt string
		if err := rows.Scan(&e.Slug, &updatedAt, &e.Locale); err != nil {
			return nil, fmt.Errorf("scan sitemap entry: %w", err)
		}
		e.UpdatedAt, _ = time.Parse(timeLayout, updatedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListRecentPublished returns the newest public pages, for the feed.
//
// It orders by COALESCE(published_at, created_at): published_at has been
// written since the beginning but was never used for ordering, so the feed
// would otherwise be sorted by when a row happened to be inserted.
func (s *Store) ListRecentPublished(ctx context.Context, websiteID int64, limit int) ([]Page, error) {
	return s.ListRecentPublishedIn(ctx, websiteID, "", limit)
}

// ListRecentPublishedIn is the same for one language.
//
// A feed carries one language: a reader who subscribed to the French feed did
// not ask for German articles in among them.
func (s *Store) ListRecentPublishedIn(ctx context.Context, websiteID int64, loc string, limit int) ([]Page, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $3`+ListablePredicate+`
		 ORDER BY COALESCE(published_at, created_at) DESC LIMIT $2`,
		websiteID, limit, loc)
	if err != nil {
		return nil, fmt.Errorf("list recent published: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recent page: %w", err)
		}
		pages = append(pages, *p)
	}
	return pages, rows.Err()
}

// HomeSlug is the address the start page has as a row, and the reason the
// public side redirects it to the root: the same page under two addresses is
// two pages to a search engine.
const HomeSlug = "home"

// GetHomePage returns the published homepage for a website.
// Looks for a page with slug "home" first, then falls back to the first published page.
func (s *Store) GetHomePage(ctx context.Context, websiteID int64) (*Page, error) {
	return s.GetHomePageIn(ctx, websiteID, "")
}

// GetHomePageIn is the same for one language.
func (s *Store) GetHomePageIn(ctx context.Context, websiteID int64, loc string) (*Page, error) {
	// Try slug "home" first
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $2 AND slug = $3`+PublicPredicate,
		websiteID, loc, HomeSlug)
	p, err := scanPage(row)
	if err == nil {
		return p, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get home page: %w", err)
	}

	// Fallback: first published page by created_at
	row = s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM pages WHERE website_id = $1 AND locale = $2`+ListablePredicate+` ORDER BY created_at ASC LIMIT 1`,
		websiteID, loc)
	p, err = scanPage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get home page fallback: %w", err)
	}
	return p, nil
}

// isUniqueViolation checks if an error is a SQLite UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint")
}
