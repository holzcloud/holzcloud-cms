// Package album stores the reusable picture sets a page carries by name.
//
// # What this store enforces, and where the shape came from
//
// Every statement in this file names website_id — directly in the WHERE clause
// on albums, and through a subquery on albums for a picture row, because
// album_items has no column of its own — and every mutating method turns
// RowsAffected() == 0 into a named error. The signatures take websiteID
// immediately after the context, so no caller can reach a row without saying
// whose website it is looking at.
//
// That shape is taken from internal/term/store.go:284-311 and deliberately NOT
// from internal/menu/store.go, whose own doc comment at :14-33 records what the
// bare-primary-key shape cost. On 2026-09-06 four menu item handlers reached
// another website's navigation — de4a1ce proves it with a failing test,
// 5e453a9 fixes it — and the reason was not carelessness in four places: it was
// that nothing in a signature made anyone supply the website, so nothing
// reminded them. Seven handlers had to remember and three did.
//
// menu_items has no website_id of its own either, and that is exactly the
// reason the menu store gives for not having been fixed the safer way: every
// item statement would need a join or a subquery. It is worth doing, and here
// it is paid while the table is new. Each of the four item methods therefore
// carries its own copy of
//
//	album_id IN (SELECT id FROM albums WHERE id = $n AND website_id = $m)
//
// Four copies of one clause, and that is a choice rather than an oversight: a
// builder that assembled the clause would hide the one thing a reader of this
// file needs to see at a glance, which is that it is there every time. A clause
// that can be forgotten in one place is how the 2026-09-06 defect happened.
//
// # The one derivation of a slug
//
// internal/term/store.go:318-328 carries a warning at length, and it applies
// here word for word: "the slug is derived with page.Slugify, the same call
// SetForPage makes. A label created by one function and looked for by the other
// has to be the same row — if the two derivations ever disagreed, an import
// would create a second label beside the one it meant to reuse, and both would
// look right in every listing."
//
// This store makes exactly ONE call to page.Slugify, in Create, and everything
// that needs an album's slug goes through it. The bundle importer of plan 11-06
// calls Create; it does not derive its own. That is the whole mitigation, and
// it only holds as long as the call stays single.
package album

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const timeLayout = "2006-01-02T15:04:05Z"

// The errors this store reports, as values rather than as sentences.
//
// A handler that has to write strings.Contains(err.Error(), "UNIQUE
// constraint") to tell a duplicate from a disk failure is coupled to SQLite's
// wording — internal/admin/menu.go:172 does exactly that today, and it is the
// thing plan 11-03 must not have to copy. The sentence an operator reads is
// minted in a template; these carry only which of the cases happened.
var (
	// ErrNotFound is the answer to a write against a row that either does not
	// exist or belongs to another website, and the store deliberately does not
	// distinguish the two. Which of the two it was is itself information about
	// another website's contents.
	ErrNotFound = errors.New("album not found for this website")

	// ErrDuplicateName is the UNIQUE (website_id, slug) violation, named.
	ErrDuplicateName = errors.New("an album with this name already exists")

	// ErrNoName is a name that is not a name: empty, or only whitespace.
	ErrNoName = errors.New("an album needs a name")

	// ErrTooManyItems is the item cap, reported and never silently applied.
	ErrTooManyItems = fmt.Errorf("an album holds at most %d pictures", MaxItems)

	// ErrForeignMedia is a media id that names a file in another website's
	// library. The foreign key proves the file exists, not whose it is.
	ErrForeignMedia = errors.New("that picture belongs to another website's library")
)

// Store handles SQL operations for albums and their pictures.
type Store struct {
	DB *db.DB
}

// NewStore creates a new album store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// normalizeName folds a typed name into the spelling that is stored: runs of
// whitespace become one space and an over-long name is cut at MaxNameLength.
// It returns "" for a name that cannot become one.
//
// The half of term.Normalize that is about a single name, minus the slug check
// that function carries — page.Slugify never returns "", so the check cannot
// fire, and a second call to it here would be the second derivation the package
// comment forbids.
func normalizeName(raw string) string {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return ""
	}
	if len([]rune(name)) > MaxNameLength {
		name = string([]rune(name)[:MaxNameLength])
	}
	return name
}

// isDuplicate reports whether err is the UNIQUE (website_id, slug) violation.
//
// By SQLite's extended result code and not by the text of the message: a string
// match reads a sentence that the driver is free to reword, and it cannot tell
// a unique violation from a NOT NULL one that happens to mention the word.
func isDuplicate(err error) bool {
	var serr *sqlite.Error
	if !errors.As(err, &serr) {
		return false
	}
	return serr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE ||
		serr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}

// Create adds an album to a website and returns the stored row.
//
// The name is normalised first, then the slug is derived from it with the one
// call to page.Slugify this package makes. page.Slugify returns "untitled"
// rather than "" for a name made only of punctuation, which is why the
// collision needs a name at all: two albums called "..." and "!!!" do not
// produce an empty key, they produce the same one and land on the UNIQUE
// constraint — where ErrDuplicateName is a truthful thing to say to an operator
// and the raw constraint text is not.
func (s *Store) Create(ctx context.Context, websiteID int64, name string) (*Album, error) {
	name = normalizeName(name)
	if name == "" {
		return nil, ErrNoName
	}
	slug := page.Slugify(name)

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO albums (website_id, slug, name) VALUES ($1, $2, $3)`,
		websiteID, slug, name)
	if err != nil {
		if isDuplicate(err) {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateName, name)
		}
		return nil, fmt.Errorf("create album: %w", err)
	}
	id, _ := res.LastInsertId()

	// Re-read rather than build the struct here, as menu.CreateMenu does: the
	// row that comes back is the row the database kept, defaults and all.
	return s.Get(ctx, websiteID, id)
}

// Get returns one album, or (nil, nil) when this website has no such album.
//
// Not found is not an error; it is a 404 at the handler. An album of another
// website is not found, which is the whole of GAL-05 on the read side.
func (s *Store) Get(ctx context.Context, websiteID, id int64) (*Album, error) {
	return s.scanOne(ctx,
		`SELECT id, website_id, slug, name, created_at FROM albums WHERE id = $1 AND website_id = $2`,
		id, websiteID)
}

// BySlug returns the album a gallery block or a bundle names, or (nil, nil).
//
// A slug is unique per website and not globally, so this needs the website for
// correctness and not merely for scoping: two websites may both have an album
// called "Referenzen".
func (s *Store) BySlug(ctx context.Context, websiteID int64, slug string) (*Album, error) {
	return s.scanOne(ctx,
		`SELECT id, website_id, slug, name, created_at FROM albums WHERE website_id = $1 AND slug = $2`,
		websiteID, slug)
}

func (s *Store) scanOne(ctx context.Context, query string, args ...any) (*Album, error) {
	var a Album
	var createdAt string
	err := s.DB.Read.QueryRowContext(ctx, query, args...).
		Scan(&a.ID, &a.WebsiteID, &a.Slug, &a.Name, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get album: %w", err)
	}
	a.CreatedAt, _ = time.Parse(timeLayout, createdAt)
	return &a, nil
}

// List returns a website's albums by name.
func (s *Store) List(ctx context.Context, websiteID int64) ([]Album, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, slug, name, created_at FROM albums WHERE website_id = $1 ORDER BY name`,
		websiteID)
	if err != nil {
		return nil, fmt.Errorf("list albums: %w", err)
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var a Album
		var createdAt string
		if err := rows.Scan(&a.ID, &a.WebsiteID, &a.Slug, &a.Name, &createdAt); err != nil {
			return nil, fmt.Errorf("scan album: %w", err)
		}
		a.CreatedAt, _ = time.Parse(timeLayout, createdAt)
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

// Rename changes the visible name of an album without moving its address.
//
// The slug stays, and this is the method GAL-04's round trip cannot be proved
// without: a gallery block stores the slug and a bundle re-derives it, so
// correcting "Werkstatt 2024" to "Werkstatt 2025" must not take the album away
// from every page that carries it. No statement in this file updates slug.
//
// The rename cannot collide, because the constraint is on the slug and the slug
// does not move. Two albums may therefore end up with the same visible name and
// different addresses, which is a thing an operator can see and undo.
func (s *Store) Rename(ctx context.Context, websiteID, id int64, name string) error {
	name = normalizeName(name)
	if name == "" {
		return ErrNoName
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE albums SET name = $1 WHERE id = $2 AND website_id = $3`,
		name, id, websiteID)
	if err != nil {
		return fmt.Errorf("rename album: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: album %d", ErrNotFound, id)
	}
	return nil
}

// Delete removes an album. Its picture rows go with it through the cascade; the
// files in the media library are untouched.
//
// This is stricter than its analog: term.Delete does not look at RowsAffected.
// It is stricter on purpose, because GAL-05 asks that an album be invisible
// from every other website, and a delete that quietly affects no rows because
// the album belongs to somebody else reports success for an operation that
// never happened — which is the one answer an operator cannot act on.
func (s *Store) Delete(ctx context.Context, websiteID, id int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM albums WHERE id = $1 AND website_id = $2`, id, websiteID)
	if err != nil {
		return fmt.Errorf("delete album: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: album %d", ErrNotFound, id)
	}
	return nil
}

// Pictures returns an album's rows in order, each with the id and the sort
// order the reorder and delete controls need.
func (s *Store) Pictures(ctx context.Context, websiteID, albumID int64) ([]Picture, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT i.id, i.album_id, i.media_id, i.alt, i.caption, i.sort_order
		   FROM album_items i
		   JOIN albums a ON a.id = i.album_id
		  WHERE i.album_id = $1 AND a.website_id = $2
		  ORDER BY i.sort_order, i.id`,
		albumID, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list album pictures: %w", err)
	}
	defer rows.Close()

	var pictures []Picture
	for rows.Next() {
		var p Picture
		if err := rows.Scan(&p.ID, &p.AlbumID, &p.Item.MediaID, &p.Item.Alt, &p.Item.Caption, &p.SortOrder); err != nil {
			return nil, fmt.Errorf("scan album picture: %w", err)
		}
		pictures = append(pictures, p)
	}
	return pictures, rows.Err()
}

// Items returns the album's pictures as the list the gallery renderer takes.
//
// This is GAL-07's "one mechanism" in one line: the same []block.Item a gallery
// block holds, so the album feeds the same renderer rather than a second one
// that agrees with it. It is a projection of Pictures and not a second query,
// so the two can never come back in a different order.
func (s *Store) Items(ctx context.Context, websiteID, albumID int64) ([]block.Item, error) {
	pictures, err := s.Pictures(ctx, websiteID, albumID)
	if err != nil {
		return nil, err
	}
	items := make([]block.Item, 0, len(pictures))
	for _, p := range pictures {
		items = append(items, p.Item)
	}
	return items, nil
}

// AddItem appends one picture to an album and returns the new row's id.
//
// Two things are checked in SQL rather than in Go, and both because the ids
// come out of a form. The album must belong to this website, and so must the
// media row: the foreign key on media_id proves only that the file exists, and
// an editor who types another website's number would otherwise pull a picture
// out of a library they cannot see. internal/admin/page_blocks.go:105-115 makes
// the same check for a block picture and gives the same reason.
func (s *Store) AddItem(ctx context.Context, websiteID, albumID, mediaID int64, alt, caption string) (int64, error) {
	if err := s.requireOwnMedia(ctx, websiteID, mediaID); err != nil {
		return 0, err
	}

	// How many pictures there are and where the last one sits, in one question,
	// scoped so that another website's album answers "none" rather than a
	// number. A count of MaxItems is the cap; the INSERT below is what reports
	// an album that is not this website's.
	var count, lastOrder int
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(i.sort_order), -1)
		   FROM album_items i
		   JOIN albums a ON a.id = i.album_id
		  WHERE i.album_id = $1 AND a.website_id = $2`,
		albumID, websiteID).Scan(&count, &lastOrder); err != nil {
		return 0, fmt.Errorf("count album pictures: %w", err)
	}
	if count >= MaxItems {
		return 0, fmt.Errorf("%w: album %d", ErrTooManyItems, albumID)
	}

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO album_items (album_id, media_id, alt, caption, sort_order)
		 SELECT $1, $2, $3, $4, $5
		  WHERE EXISTS (SELECT 1 FROM albums WHERE id = $1 AND website_id = $6)`,
		albumID, mediaID, alt, caption, lastOrder+1, websiteID)
	if err != nil {
		return 0, fmt.Errorf("add album picture: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, fmt.Errorf("%w: album %d", ErrNotFound, albumID)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// UpdateItem repoints one picture row and rewrites its alt text and caption.
//
// The same two checks AddItem makes, for the same reason, and one more thing
// the subquery buys: an item id that is correct but belongs to another website's
// album affects zero rows and is reported, rather than being edited because the
// number happened to be right.
func (s *Store) UpdateItem(ctx context.Context, websiteID, albumID, itemID, mediaID int64, alt, caption string) error {
	if err := s.requireOwnMedia(ctx, websiteID, mediaID); err != nil {
		return err
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE album_items SET media_id = $1, alt = $2, caption = $3
		  WHERE id = $4
		    AND album_id IN (SELECT id FROM albums WHERE id = $5 AND website_id = $6)`,
		mediaID, alt, caption, itemID, albumID, websiteID)
	if err != nil {
		return fmt.Errorf("update album picture: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemID)
	}
	return nil
}

// DeleteItem removes one picture from an album. The file stays in the library.
func (s *Store) DeleteItem(ctx context.Context, websiteID, albumID, itemID int64) error {
	res, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM album_items
		  WHERE id = $1
		    AND album_id IN (SELECT id FROM albums WHERE id = $2 AND website_id = $3)`,
		itemID, albumID, websiteID)
	if err != nil {
		return fmt.Errorf("delete album picture: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemID)
	}
	return nil
}

// SwapSortOrder exchanges the sort_order of two pictures in one transaction.
//
// internal/menu/store.go:196-222 as it stands, with the website scoping the
// original lacks: both reads go through the subquery on albums, so a picture of
// another website's album never even yields a number to cross.
//
// Order is changed by swapping two rows and never by rewriting the list. A
// rewrite is O(n) statements for a move of one position, and it turns a
// concurrent edit into a lost one.
//
// Nothing between the two reads and the two writes touches the network, the
// filesystem or user input — it is four statements against four integers. That
// is not a stylistic note: the write pool admits exactly one connection, so a
// transaction that waited on anything at all would block every write on the
// installation for as long as it waited. TestSwapDoesNotHoldTheWriteConnection
// is what holds this, under a deadline below busy_timeout, and it is
// deliberately not a count of BeginTx: a count proves a proxy.
func (s *Store) SwapSortOrder(ctx context.Context, websiteID, albumID, itemA, itemB int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin swap: %w", err)
	}
	defer tx.Rollback()

	var orderA, orderB int
	err = tx.QueryRowContext(ctx,
		`SELECT sort_order FROM album_items
		  WHERE id = $1
		    AND album_id IN (SELECT id FROM albums WHERE id = $2 AND website_id = $3)`,
		itemA, albumID, websiteID).Scan(&orderA)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemA)
	}
	if err != nil {
		return fmt.Errorf("read sort_order of picture %d: %w", itemA, err)
	}
	err = tx.QueryRowContext(ctx,
		`SELECT sort_order FROM album_items
		  WHERE id = $1
		    AND album_id IN (SELECT id FROM albums WHERE id = $2 AND website_id = $3)`,
		itemB, albumID, websiteID).Scan(&orderB)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemB)
	}
	if err != nil {
		return fmt.Errorf("read sort_order of picture %d: %w", itemB, err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE album_items SET sort_order = $1
		  WHERE id = $2
		    AND album_id IN (SELECT id FROM albums WHERE id = $3 AND website_id = $4)`,
		orderB, itemA, albumID, websiteID)
	if err != nil {
		return fmt.Errorf("write sort_order of picture %d: %w", itemA, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemA)
	}
	res, err = tx.ExecContext(ctx,
		`UPDATE album_items SET sort_order = $1
		  WHERE id = $2
		    AND album_id IN (SELECT id FROM albums WHERE id = $3 AND website_id = $4)`,
		orderA, itemB, albumID, websiteID)
	if err != nil {
		return fmt.Errorf("write sort_order of picture %d: %w", itemB, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: picture %d", ErrNotFound, itemB)
	}

	return tx.Commit()
}

// requireOwnMedia refuses a media id that names a file in another website's
// library, which is T-11-07: the foreign key proves the file exists and never
// whose it is, and the id arrives from a form.
func (s *Store) requireOwnMedia(ctx context.Context, websiteID, mediaID int64) error {
	var ok int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT 1 FROM media WHERE id = $1 AND website_id = $2`, mediaID, websiteID).Scan(&ok)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: picture %d", ErrForeignMedia, mediaID)
	}
	if err != nil {
		return fmt.Errorf("check media ownership: %w", err)
	}
	return nil
}

// LoadFor builds the expansion set for one page, loading only the albums that
// page's HTML actually names.
//
// # The loading rule, and where it comes from
//
// media.LoadImageSets (internal/media/variant_store.go:176) states it: "Only
// the files actually named in the HTML are looked up: a site with a thousand
// uploads should not pay for all of them to render one page." The same
// sentence with albums instead of files. So the slugs come out of the document
// first, and a document naming none returns here without touching the database
// at all — which is what makes this mechanism cost nothing on the pages that do
// not use it.
//
// What is deliberately NOT copied is internal/snippet's loadSnippets, which
// builds a map for the whole website on every request. That shape would make a
// website with fifty albums pay for fifty to render one page, and a page
// usually names one.
//
// One query for all the named albums, not one per marker: a page carrying four
// album galleries is still one trip.
//
// # The two website conditions, and why the second is not redundant
//
// a.website_id = $1 is GAL-05: an album of another website must not be
// reachable through a marker, and the check belongs in the WHERE clause rather
// than in the caller — which is the lesson internal/menu/store.go's doc comment
// records at length and the reason every statement in this file carries it.
//
// m.website_id = a.website_id is the second, and it looks redundant only if one
// assumes every album_items row was written through AddItem. It was not
// necessarily: the foreign key on media_id proves the file exists and never
// whose it is, and a row could have been written by a repair, by an older
// build, or before requireOwnMedia existed. The join condition makes the
// picture's website a fact of this query rather than a property of its history.
func (s *Store) LoadFor(ctx context.Context, websiteID int64, html string, t func(string) string) (Set, error) {
	set := Set{
		items:  map[string][]block.Item{},
		images: map[int64]block.Image{},
		t:      t,
	}

	slugs := UsedSlugs(html)
	if len(slugs) == 0 {
		return set, nil
	}

	args := []any{websiteID}
	placeholders := make([]string, len(slugs))
	for i, slug := range slugs {
		args = append(args, slug)
		placeholders[i] = "$" + strconv.Itoa(i+2)
	}

	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT a.slug, i.media_id, i.alt, i.caption,
		        m.website_id, m.filename, m.mime_type, m.alt_text,
		        m.width, m.height, m.focus_x, m.focus_y, m.version
		   FROM album_items i
		   JOIN albums a ON a.id = i.album_id
		   JOIN media  m ON m.id = i.media_id AND m.website_id = a.website_id
		  WHERE a.website_id = $1 AND a.slug IN (`+strings.Join(placeholders, ", ")+`)
		  ORDER BY i.album_id, i.sort_order, i.id`, args...)
	if err != nil {
		return Set{}, fmt.Errorf("load albums for page: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slug string
		var it block.Item
		var m media.Media
		if err := rows.Scan(&slug, &it.MediaID, &it.Alt, &it.Caption,
			&m.WebsiteID, &m.Filename, &m.MimeType, &m.AltText,
			&m.Width, &m.Height, &m.Crop.FocusX, &m.Crop.FocusY, &m.Version); err != nil {
			return Set{}, fmt.Errorf("scan album picture for page: %w", err)
		}
		m.ID = it.MediaID
		set.items[slug] = append(set.items[slug], it)
		set.images[it.MediaID] = imageOf(m)
	}
	if err := rows.Err(); err != nil {
		return Set{}, fmt.Errorf("read albums for page: %w", err)
	}
	return set, nil
}
