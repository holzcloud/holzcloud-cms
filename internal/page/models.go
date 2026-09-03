package page

import "time"

// SitemapEntry is the subset of a page needed to build a sitemap.
type SitemapEntry struct {
	Slug      string
	UpdatedAt time.Time
	// Locale is the page's language, empty for the website's main one. The
	// sitemap needs it to write the prefix: without it every language of a page
	// would be listed under the same address.
	Locale string
}

// Page represents a content page within a website.
type Page struct {
	ID              int64
	WebsiteID       int64
	Title           string
	Slug            string
	ContentMarkdown string
	ContentHTML     string
	// Blocks is the encoded block list, or empty for a page written as one
	// piece of Markdown. It decides which editor a page opens in.
	Blocks string
	// Fields holds the answers to the website's own fields as JSON. Empty for
	// a page on a website that defines none, which is every page until someone
	// defines one.
	Fields string

	// Locale is the language this page is written in. Empty means the
	// website's main language — so every page written before there was a
	// second language is in the right place without being touched.
	Locale string
	// TranslationOf is the page in the main language this one translates, or
	// zero. A star and not a chain: every translation points at the same
	// middle, so the order they were created in is not built in for good.
	TranslationOf int64

	Status      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Version is the optimistic-locking token. It is round-tripped through the
	// edit form so a save can tell whether the page moved underneath it.
	Version int64

	CreatedBy *int64
	UpdatedBy *int64

	// DeletedAt marks a page as being in the trash. Every query that serves
	// live content must filter on it — see LivePredicate.
	DeletedAt *time.Time

	// Excerpt is a short plain-text summary. Derived from the Markdown on save
	// when the editor leaves it blank, so listings and meta tags always have
	// something usable.
	Excerpt string
	// MetaDescription overrides the excerpt for search engines.
	MetaDescription string
	// FeaturedMediaID is the preview image used in link previews.
	FeaturedMediaID *int64
	// NoIndex asks search engines to leave this page out while it stays
	// publicly reachable — for a thank-you page or a printable variant.
	NoIndex bool

	// PublishAt and UnpublishAt bound the window in which a published page is
	// actually visible. Both are optional; nil means "no bound".
	PublishAt   *time.Time
	UnpublishAt *time.Time

	// ReviewState is "pending" when an editor has asked for the draft to be
	// looked at. It is deliberately not a status value: status carries a CHECK
	// on a STRICT table that cannot be widened without a full table rebuild.
	ReviewState string

	// Kind is KindPage or KindPost. A post is the same record listed
	// differently: newest first, in an archive, out of the main navigation.
	Kind string
	// TypeKey is the website's own content kind — "produkt", "termin" — and
	// empty for the two built-in ones. An entry of an own kind keeps
	// Kind == KindPage: it lives under its own address and is listed only by
	// its own overview page.
	//
	// A second column beside Kind and not a third value in it: kind carries a
	// CHECK from migration 00014, and changing a CHECK in SQLite means
	// rebuilding the table that the revisions, the menu items and the labels
	// point at with foreign keys.
	TypeKey string

	// Access is AccessPublic or AccessPassword.
	Access string
	// AccessPassword is the Argon2id hash guarding the page, never the password
	// itself.
	AccessPassword string
	// AccessHint is the line shown above the form, so a visitor who has the
	// password knows they are in the right place.
	AccessHint string
}

// How a page may be reached.
const (
	AccessPublic   = "public"
	AccessPassword = "password"
)

// Protected reports whether a visitor has to enter a password.
func (p Page) Protected() bool {
	return p.Access == AccessPassword && p.AccessPassword != ""
}

// The two kinds of content. A page is timeless and belongs in a menu; a post is
// dated and belongs in an archive.
const (
	KindPage = "page"
	KindPost = "post"
)

// NormalizeKind maps anything unrecognised onto a page.
//
// The column carries a CHECK, so an unexpected value would fail the insert
// rather than default quietly — and a form field is exactly where an unexpected
// value comes from.
func NormalizeKind(kind string) string {
	if kind == KindPost {
		return KindPost
	}
	return KindPage
}

// IsPost reports whether this record belongs in the archive.
func (p Page) IsPost() bool { return p.Kind == KindPost }

// IsOwnKind reports whether this record belongs to a kind the operator defined.
func (p Page) IsOwnKind() bool { return p.TypeKey != "" }

// KindValue is the effective kind: the own one when there is one, otherwise the
// built-in. It is what decides which of the website's own fields apply.
func (p Page) KindValue() string {
	if p.TypeKey != "" {
		return p.TypeKey
	}
	return p.Kind
}

// Scheduled reports whether the page is waiting for its publication moment.
func (p Page) Scheduled() bool {
	return p.Status == "published" && p.PublishAt != nil && p.PublishAt.After(time.Now().UTC())
}

// Expired reports whether the page has passed its end date.
func (p Page) Expired() bool {
	return p.UnpublishAt != nil && !p.UnpublishAt.After(time.Now().UTC())
}

// PubliclyVisible mirrors PublicPredicate for a page already in memory, so the
// admin list can say what a visitor would see without a second query.
func (p Page) PubliclyVisible() bool {
	return p.Status == "published" && !p.InTrash() && !p.Scheduled() && !p.Expired()
}

// InTrash reports whether the page has been soft-deleted.
func (p Page) InTrash() bool { return p.DeletedAt != nil }

// Revision is a previous state of a page.
//
// Only the Markdown is kept; content_html is derivable and storing it would
// roughly double the table on an SD card.
// RevisionLabelMax bounds a version's name. It is a note in a list, not a
// field for prose — and a list whose one long entry pushes the dates off the
// screen is worse than a truncated note.
const RevisionLabelMax = 80

type Revision struct {
	ID              int64
	PageID          int64
	UserID          *int64
	UserEmail       string
	Title           string
	Slug            string
	ContentMarkdown string
	Status          string
	CreatedAt       time.Time
	// Blocks is what the page was built from at the time. Without it a restore
	// would bring back a block page as an empty one.
	Blocks string
	// Label is what somebody called this version — "vor dem Umbau", "Stand
	// Preisliste 2026". Empty on all the versions nobody had a reason to name,
	// which is most of them.
	Label string
}
