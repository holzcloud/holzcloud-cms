package album

import (
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
)

// Album is one named set of pictures, and it belongs to exactly one website.
//
// Slug is the address and Name is what an editor typed. The two are separate
// for the reason terms already keeps them separate, plus one of this table's
// own: a gallery block names an album by its slug and the bundle re-derives
// that slug on import, so a rename that moved it would take the album away from
// every page carrying it at the moment somebody corrected a typo. Rename moves
// Name and never Slug.
type Album struct {
	ID        int64
	WebsiteID int64
	Slug      string
	Name      string
	CreatedAt time.Time
}

// Picture is one row of album_items: what the database keeps about the row —
// its identity and its place in the order — around the picture itself.
//
// The picture is a block.Item and not three fields of this package's own, and
// that is where GAL-07 is decided. "One mechanism" means the album's list and a
// gallery block's list are the SAME list fed to the SAME renderer; two
// structurally identical types with a conversion between them would be a second
// shape, and a second shape is where a second renderer starts. Items returns
// exactly this Item field, so the album hands the gallery arm the type it
// already takes.
//
// A caller that only draws the pictures wants Items. A caller that has to name
// one of them — the reorder and delete controls of the admin screen — wants
// Pictures, because a block.Item carries no row id and cannot.
type Picture struct {
	ID        int64
	AlbumID   int64
	SortOrder int
	Item      block.Item
}

// MaxNameLength bounds an album's name.
//
// Sixty, the same as a label's and for the same reason: a name is a heading in
// a list of albums, not a sentence about what is in it. What is in it is the
// pictures.
const MaxNameLength = 60

// MaxItems bounds how many pictures one album holds.
//
// An album is a reusable set and may legitimately be larger than the 24 a
// single block is capped at — that cap bounds one page's editor, this one
// bounds a shared thing that many pages expand. A hundred and twenty is five
// times the block's cap and still a page a browser lays out without complaint.
//
// Without a cap an unbounded album is an unbounded page, built on the read path,
// for anybody who asks for it: one album with ten thousand rows is ten thousand
// picture elements on every request to every page that names it. The cap is
// reported and never silently applied — AddItem past it returns ErrTooManyItems
// rather than dropping the picture, which is the discipline wxr.Export.Truncated
// sets for the whole repository.
const MaxItems = 120
