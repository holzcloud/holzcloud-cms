package album

import (
	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The late half of an album: what a page stores is a marker, what a visitor
// gets is the pictures, and the two are separated by a request.
//
// # Why this cannot happen on save
//
// Block HTML is rendered once, on save, and written into pages.content_html —
// internal/admin/page_blocks.go:74-81 says so in its own comment, and the
// public side only rewrites what it reads. So an album expanded at save time
// would freeze a copy of its pictures into every page carrying it: GAL-03
// ("changing the album changes every page that carries it, without touching
// those pages") would be false, and NOTHING anywhere would report it. No test
// fails, no line is logged. The editor changes the album, reloads the page and
// sees the old pictures.
//
// internal/public/pagedata.go:22-25 states the same argument for snippets in
// exactly these words, and internal/snippet/store.go:243-264 is the mechanism
// this one is modelled on.
//
// # What is copied from snippet.Expand and what is deliberately not
//
// Copied: a marker survives into content_html and is replaced per request; a
// document with no marker is returned by an early return; a marker with no
// matching album expands to the empty string, because a visitor must not see
// the internal syntax on a live page.
//
// NOT copied: the loading strategy. loadSnippets builds a map for the WHOLE
// website on every request. A website with fifty albums must not load fifty to
// render one page, so LoadFor reads the slugs out of the HTML first and asks
// for those — which is media.LoadImageSets' rule (variant_store.go:176: "Only
// the files actually named in the HTML are looked up").
//
// # Where the marker's spelling lives
//
// In internal/block, not here. This package imports internal/block for
// block.Item, so the other way round would be an import cycle. The prefix, the
// pattern, the writer and the readers are all in internal/block/render.go and
// this file calls them: two spellings of one marker in two packages drift
// invisibly, because the page still renders and the marker simply stays
// visible on it.

// Set is everything one request needs to expand the markers of one page: the
// pictures of each album the page names, the images those pictures resolve to,
// and the translator for the words the renderer writes itself.
//
// Built by Store.LoadFor and handed straight to Expand. The zero Set expands
// every marker to nothing, which is what a page whose albums have all been
// deleted looks like.
type Set struct {
	// items is the album's list, by slug. It is []block.Item and not a type of
	// this package's own, which is where GAL-07 is decided: the album's
	// pictures and a gallery block's pictures are the same list handed to the
	// same renderer.
	items map[string][]block.Item
	// images is every picture the lists above name, resolved once for the whole
	// request. The query that filled it already checked the website, which is
	// why Lookup below can be a map read and not a second trip to the database.
	images map[int64]block.Image
	// t is the website's translator, the same one internal/admin's blockSet
	// builds for the save-time render, so a page carrying an inline gallery and
	// an album gallery does not end up with its two sets of controls in two
	// languages.
	t func(string) string
}

// Lookup is the picture resolver the gallery renderer takes.
//
// Given out by the Set rather than rebuilt by the caller, so internal/public
// does not grow a second copy of internal/admin's blockImages. Two places
// resolving a media id are two places to forget the website check, and the
// check has already happened in the query that filled the map.
func (s Set) Lookup() block.Lookup {
	return func(id int64) (block.Image, bool) {
		img, ok := s.images[id]
		return img, ok
	}
}

// UsedSlugs returns the album slugs a document names, deduplicated.
//
// This is what turns "load only what the page names" from an intention into a
// query. It mirrors snippet.UsedKeys, and the reading is internal/block's
// because the marker's spelling is.
func UsedSlugs(html string) []string { return block.AlbumMarkerSlugs(html) }

// Expand replaces every album marker in rendered page HTML with the album's
// pictures, rendered by the gallery renderer.
//
// A document with no marker is returned unchanged — identical, not merely
// equal — which is what keeps this mechanism free for the pages that do not use
// it. A marker whose slug is not in the set expands to the empty string, for
// the reason snippet.Expand gives: a visitor must not see the internal syntax
// on a live page, and a deleted album should cost its own block rather than the
// article around it.
//
// The pictures go through block.GalleryItems — the very function the inline
// gallery calls — with the block position the marker carries, so the fragment
// ids of an album gallery and of an inline gallery on the same page cannot
// collide. That shared call is GAL-07's "one mechanism": one list type, one
// renderer, two sources. Nothing here renders a tile or a large view itself.
func Expand(html string, set Set) string {
	look := set.Lookup()
	return block.ReplaceAlbumMarkers(html, func(slug string, at int) string {
		items := set.items[slug]
		if len(items) == 0 {
			return ""
		}
		return block.GalleryItems(at, items, look, set.t)
	})
}

// imageOf is the one place a media row becomes what the renderer needs.
//
// The six values are taken from media.Media's own methods rather than rebuilt
// from the columns: URL carries the version query that a crop made necessary,
// FocusCSS decides whether an object-position is worth writing at all, and
// IsVideo decides which element the file belongs in. A second computation of
// any of them here would be a second place to get the media rules wrong.
func imageOf(m media.Media) block.Image {
	return block.Image{
		URL:    m.URL(),
		Alt:    m.AltText,
		Width:  m.Width,
		Height: m.Height,
		Focus:  m.FocusCSS(),
		Film:   m.IsVideo(),
	}
}
