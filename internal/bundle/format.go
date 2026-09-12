// Package bundle reads and writes the archive that holds one website.
//
// It exists so that a site is not a hostage to the machine it runs on. A
// database file is a backup of the whole installation; this is one website, in
// a form that can be read without Holzcloud, moved to another server, or kept
// as the copy the customer owns.
package bundle

import (
	"time"
)

// Version is the format version written into every bundle.
//
// A reader refuses a major version it does not know rather than guessing: an
// import that silently drops the fields it did not recognise is worse than one
// that says it cannot do the job.
const Version = 1

// ManifestName is the file every bundle must open with.
const ManifestName = "holzcloud.json"

// Manifest is the whole website, as it appears in the archive.
//
// JSON rather than the SQL the database happens to use: the point is a file
// somebody can read, diff and repair in a text editor when everything else has
// gone wrong.
type Manifest struct {
	Version int `json:"version"`
	// ExportedAt is informational; nothing depends on it.
	ExportedAt string `json:"exported_at"`
	// GeneratedBy names the version that wrote the file, which is the first
	// thing anyone wants when an import behaves oddly.
	GeneratedBy string `json:"generated_by"`

	Site     Site      `json:"site"`
	Pages    []Page    `json:"pages"`
	Menus    []Menu    `json:"menus,omitempty"`
	Snippets []Snippet `json:"snippets,omitempty"`
	Terms    []Term    `json:"terms,omitempty"`
	Media    []Media   `json:"media,omitempty"`
	// Albums are the website's named sets of pictures. A gallery block names
	// one instead of listing its own pictures, and the reference travels as
	// this name — see Block.Album.
	Albums []Album `json:"albums,omitempty"`
	// Types are the website's own content kinds. Without them the entries would
	// arrive carrying a kind nothing on the other side knows, and a hundred
	// products would sit in the list as untitled kinds.
	Types []ContentType `json:"content_types,omitempty"`
	// Fields are the website's own page fields. Their definitions travel with
	// the bundle, because the values on the pages mean nothing without them.
	Fields []Field `json:"fields,omitempty"`
	// BlockTypes are the website's own block kinds, with their fields.
	BlockTypes []BlockType `json:"block_types,omitempty"`
}

// Field is one of the website's own page fields.
type Field struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	// TypeKey is the website's own kind of this entry, empty for the built-in
	// two.
	TypeKey   string   `json:"content_type,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Hint      string   `json:"hint,omitempty"`
	Choices   []string `json:"choices,omitempty"`
	AppliesTo string   `json:"applies_to,omitempty"`
	// Condition is the key of the field this one hangs on: it is asked for only
	// once that one is filled in. Empty for a field that is always shown.
	Condition string `json:"condition,omitempty"`
	// Display ist der Anzeigemodus einer Auswahl, MaxValues die Höchstzahl der
	// Werte einer Mehrfachauswahl, Min und Max die beiden Grenzen eines
	// Bereichs.
	//
	// Alle vier mit omitempty: eine Website, die keine davon benutzt, schreibt
	// ein Manifest, das Byte für Byte so aussieht wie eines von vor dieser
	// Phase. Ein Archiv ist dazu da, von Hand gelesen und geflickt zu werden.
	Display   string `json:"display,omitempty"`
	MaxValues int    `json:"max_values,omitempty"`
	Min       string `json:"min,omitempty"`
	Max       string `json:"max,omitempty"`
	// Sub are a group's own fields. Empty for everything else.
	Sub []Field `json:"sub,omitempty"`
}

// BlockType is one of the website's own block kinds, with what it is made of.
//
// The kind has to travel with the blocks that use it: without it, an imported
// page would carry pieces of a type nothing on the other side has heard of,
// and Clean would drop every one of them.
type BlockType struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Hint   string  `json:"hint,omitempty"`
	Fields []Field `json:"fields,omitempty"`
}

// Site is the website's own settings.
//
// Deliberately absent: the domains. A bundle is meant to be imported somewhere
// else, and carrying the host names would either collide with the site already
// serving them or quietly claim a domain the new machine does not own.
type Site struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Locale      string `json:"locale,omitempty"`
	// ExtraLocales are the website's further languages, as stored: "fr, it".
	// Without them a translated page would arrive filed under the main
	// language, and two menus that differ only by language would collide on
	// the way in — the site would quietly lose half of what it had.
	ExtraLocales    string `json:"extra_locales,omitempty"`
	TimeZone        string `json:"timezone,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`

	BlogBase     string `json:"blog_base,omitempty"`
	PostsPerPage int    `json:"posts_per_page,omitempty"`
	ContactEmail string `json:"contact_email,omitempty"`

	OrgType      string `json:"org_type,omitempty"`
	Street       string `json:"street,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	City         string `json:"city,omitempty"`
	Country      string `json:"country,omitempty"`
	Phone        string `json:"phone,omitempty"`
	OpeningHours string `json:"opening_hours,omitempty"`

	Design Design `json:"design,omitempty"`
}

// Design carries the per-site tokens.
type Design struct {
	Ink     string `json:"ink,omitempty"`
	Paper   string `json:"paper,omitempty"`
	Brand   string `json:"brand,omitempty"`
	Font    string `json:"font,omitempty"`
	Measure int    `json:"measure,omitempty"`
	Radius  int    `json:"radius,omitempty"`
}

// Page is one page or post.
//
// Only the Markdown travels, never the rendered HTML: the HTML is derived, it
// roughly doubles the archive, and a bundle written by an older renderer would
// otherwise import content that no longer matches what this one produces.
type Page struct {
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Markdown string `json:"markdown"`
	Status   string `json:"status"`
	Kind     string `json:"kind,omitempty"`
	// TypeKey is the website's own kind of this entry, empty for the built-in
	// two.
	TypeKey string `json:"content_type,omitempty"`

	Excerpt         string `json:"excerpt,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
	NoIndex         bool   `json:"noindex,omitempty"`

	PublishedAt *time.Time `json:"published_at,omitempty"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	UnpublishAt *time.Time `json:"unpublish_at,omitempty"`

	// FeaturedImage is a file name from the media list, not an id: ids are
	// meaningless on the machine the bundle lands on.
	FeaturedImage string `json:"featured_image,omitempty"`
	// Terms are the labels this page carries, spelled as their names are shown
	// — "Laufräder", not "laufraeder". Each one matches the name of an entry in
	// the manifest's own term list, and the import derives the slug from it.
	// The name and not the slug because a manifest exists to be read and
	// repaired by hand, and because the reading side has always worked this
	// way: changing it would alter the meaning of every bundle already handed
	// to somebody.
	Terms []string `json:"terms,omitempty"`

	// Access carries the protection setting but never the password hash. A
	// hash in a file that gets emailed around is a hash somebody can attack
	// offline, and the operator can set a new password in a moment.
	Access string `json:"access,omitempty"`
	// AccessHint is the line above the password form; it holds no secret.
	AccessHint string `json:"access_hint,omitempty"`

	// Blocks are the pieces of a page built with the block editor, empty for a
	// page written in Markdown. A picture inside one travels as a file name,
	// for the same reason as FeaturedImage.
	//
	// The rendered HTML does not travel — see the note above — so an import
	// draws the page again from these.
	Blocks []Block `json:"blocks,omitempty"`

	// Fields are the answers to the website's own fields. A picture field
	// carries the file name rather than the id, for the same reason as
	// FeaturedImage: an id means nothing on the machine the bundle lands on.
	Fields map[string]string `json:"fields,omitempty"`
	// FieldGroups are the filled rows of each repeatable group.
	FieldGroups map[string][]map[string]string `json:"field_groups,omitempty"`

	// Locale is the language this page is written in, empty for the website's
	// main one.
	Locale string `json:"locale,omitempty"`
	// TranslationOf is the address of the main-language page this one
	// translates — a slug and not an id, for the same reason as
	// FeaturedImage: an id means nothing on the machine the bundle lands on.
	TranslationOf string `json:"translation_of,omitempty"`
}

// Block is one piece of a page built with the block editor.
//
// A near copy of block.Block, and deliberately not that type itself: the two
// differ in exactly the place that matters here, where an id becomes a file
// name — and a bundle format that followed an internal struct would change
// shape every time that struct did.
type Block struct {
	Type string `json:"type"`

	Markdown string `json:"markdown,omitempty"`

	// Media and Poster are file names from the media list, never ids.
	Media   string `json:"media,omitempty"`
	Poster  string `json:"poster,omitempty"`
	Alt     string `json:"alt,omitempty"`
	Caption string `json:"caption,omitempty"`

	Variant string `json:"variant,omitempty"`

	// Display is a gallery's layout mode. The key is English because every key
	// in this struct is; the value is the German constant block.DisplaySlideshow,
	// exactly as Type above already carries "galerie" under the key "type".
	Display string `json:"display,omitempty"`

	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	Source   string `json:"source,omitempty"`
	LinkText string `json:"link_text,omitempty"`
	LinkURL  string `json:"link_url,omitempty"`

	// Album is the NAME of the album a gallery block takes its pictures from,
	// empty for a gallery carrying its own list below. It matches the name of
	// an entry in the manifest's own album list, and the import derives the
	// address from it.
	//
	// The name and not the slug, which is the same rule Page.Terms follows and
	// for one more reason of this field's own: album.Store.Rename moves the
	// name and never the slug, so a page keeps the old address while the album
	// shows a new name. A reference that travelled as a slug would have the
	// other machine derive a different one from the name and point at nothing
	// — silently, with an empty gallery where the pictures were. That is
	// GAL-04, and TestAlbumRoundTripAfterRename is the proof, which renames
	// before it exports because a test that does not proves nothing.
	//
	// A name no album entry declares is dropped on both sides rather than
	// carried through, for the reason export.go gives for an unresolvable
	// term: the value would land on whatever the other machine happens to
	// derive from it, and that is worse than landing on nothing. missingAlbum
	// puts it on the import report so nobody is left with a silent empty
	// gallery.
	Album string `json:"album,omitempty"`

	// Items are the pictures of a gallery or the panels of a card row.
	Items []BlockItem `json:"items,omitempty"`
	// Fields are the values of an own block kind, by field key. A picture among
	// them travels as a file name too.
	Fields map[string]string `json:"fields,omitempty"`
}

// BlockItem is one entry inside a gallery or a card row.
type BlockItem struct {
	Media    string `json:"media,omitempty"`
	Alt      string `json:"alt,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Title    string `json:"title,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	LinkURL  string `json:"link_url,omitempty"`
}

// Protected reports whether this page was password-protected at export.
func (p Page) Protected() bool { return p.Access == "password" }

// ContentType is one own content kind of the website.
type ContentType struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Plural  string `json:"plural"`
	Archive string `json:"archive,omitempty"`
	Sort    string `json:"sort,omitempty"`
}

// Album is one named set of pictures, and it belongs to exactly one website.
//
// The name is the load-bearing half and the only half that travels. There is
// deliberately no slug here: the importing machine derives it from the name
// with page.Slugify, which is the same one call album.Store.Create makes, so
// the two agree by construction rather than by coincidence. Carrying both
// would be two sources for one key — the shape internal/term/store.go:318-328
// warns about at length — and the derived one would win anyway.
//
// The name and not an id, for the reason Page.Terms already gives: a manifest
// exists to be read and repaired by hand, and an id means nothing to a person
// holding a text editor. It also survives a rename, which is the whole point:
// album.Store.Rename moves the name and never the slug, so a page keeps the
// old slug while the album shows a new name, and a reference that travelled as
// a slug would land on nothing on the other machine — silently.
type Album struct {
	Name  string      `json:"name"`
	Items []AlbumItem `json:"items,omitempty"`
}

// AlbumItem is one picture of an album, in the album's own order.
//
// Media is a file name from the media list and never a number, which is the
// sentence blocks.go opens with: "a picture is stored as an id, and an id
// means nothing on the machine the bundle lands on. So it travels as a file
// name, the same way a page's preview image already did." A name that is not
// in the media list can be reported on import; a number would silently point
// at somebody else's picture.
type AlbumItem struct {
	Media   string `json:"media,omitempty"`
	Alt     string `json:"alt,omitempty"`
	Caption string `json:"caption,omitempty"`
}

// Menu is one navigation, with its items already in order.
type Menu struct {
	// Locale is the language this menu belongs to, empty for the main one.
	Locale string `json:"locale,omitempty"`

	Name        string     `json:"name"`
	LocationKey string     `json:"location_key"`
	Items       []MenuItem `json:"items,omitempty"`
}

// MenuItem is one entry. Children are nested rather than referenced by id, so
// the tree survives a round trip without any id fixing up.
type MenuItem struct {
	Title string `json:"title"`
	Type  string `json:"type"`
	// PageSlug points at a page in this bundle; URL is used for the rest.
	PageSlug string     `json:"page_slug,omitempty"`
	URL      string     `json:"url,omitempty"`
	Children []MenuItem `json:"children,omitempty"`
}

// Snippet is one reusable block: a Markdown body, and since 00047 the
// snippet's own fields beside it.
//
// The two halves are named apart on purpose. Fields are the definitions, the
// way BlockType.Fields is; Values are the answers, the way Page.Fields is. A
// snippet is the one carrier that has both, and one name for both would leave
// the next reader a coin to flip.
//
// All three of the new members carry omitempty, for the reason spelled out at
// Field: a website that has defined no snippet field writes a manifest byte for
// byte like one from before this phase, and an archive written before it
// imports unchanged — a snippet with neither key arrives with its body and no
// fields, exactly as it always did.
//
// What does not travel: the ids inside a value. A picture field holds a media
// id and a reference field a page id, and on the way out a page turns those
// into a file name and an address (exportFieldValues). A snippet's values go
// out as they are stored, so such a value lands on the other machine pointing
// at nothing — the field arrives, the picture does not. That is a recorded
// limitation and not an oversight: the lookups a translation needs are built
// inside the page export and the page import, and reaching them from here is a
// change to the page path. A text, a number, a date, a choice or a yes/no —
// which is what a snippet's fields are in practice — travel whole.
//
// „Recorded" heisst seit dem Review zu Phase 8 wirklich aufgeschrieben, und
// zwar zweimal ausserhalb dieses Kommentars: in deferred-items.md, und im
// Bericht jedes Imports, der einen solchen Wert mitbringt
// (installationBoundValues, import.go). Der Bildschirm bietet diese Feldarten am
// Textbaustein ausdrücklich an; ein Versprechen, das beim Ausfahren
// stillschweigend bricht, wäre der lautlose Verlust, den dieses Projekt sonst
// überall vermeidet. Die Übersetzung selbst bleibt offen und gehört in die
// Roadmap.
type Snippet struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
	// Fields are the snippet's own field definitions — what may be filled in,
	// not what is. Same struct and same meaning as BlockType.Fields, and they
	// travel for the same reason a page's definitions do: the values below mean
	// nothing without them, and Clean would drop every one of them.
	Fields []Field `json:"fields,omitempty"`
	// Values are the answers to those definitions, the same shape and the same
	// meaning as Page.Fields.
	Values map[string]string `json:"values,omitempty"`
	// ValueGroups are the filled rows of each repeatable group, the same shape
	// as Page.FieldGroups. A snippet has a form of its own and may therefore
	// carry a group, so it carries the rows too — the page's decision copied,
	// not a second one taken here.
	ValueGroups map[string][]map[string]string `json:"value_groups,omitempty"`
}

// Term is one label.
//
// The name is the load-bearing half: it is what a page's Terms refer to, and
// the import creates the label from that name alone, deriving the slug itself
// with the same rule the editor uses. So the declared Slug is documentation —
// it says what address the label had where the bundle came from, and an import
// that would derive a different one goes with its own. A label no page carries
// is therefore counted on the way in but not created.
type Term struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Media is one file. The bytes live beside the manifest in the archive, under
// media/<filename>.
type Media struct {
	Filename     string `json:"filename"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	AltText      string `json:"alt_text,omitempty"`
	Caption      string `json:"caption,omitempty"`
	// SHA256 lets an import notice a file that was corrupted in transit rather
	// than storing the damage and finding out when a page renders.
	SHA256 string `json:"sha256,omitempty"`
}

// MediaDir is where file bytes live inside the archive.
const MediaDir = "media/"
