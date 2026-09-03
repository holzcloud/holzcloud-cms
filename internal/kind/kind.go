// Package kind is what a website's own content types are made of.
//
// Two kinds shipped from the start: a page and a post. They are the two shapes
// a small website really has — something that stands on its own and something
// that belongs in a dated list. Everything else somebody keeps on a website —
// products, events, recipes, animals, breeding records — had to be called a
// post and read as one in every screen.
//
// An own kind is deliberately small. It is a name, a plural, optionally an
// overview page and an order. What it is *not* is a second storage: an entry of
// any kind is a row in pages, with the same title, address, text, blocks,
// fields, languages, revisions and rubbish bin. That is the whole trick — a new
// kind costs a row in one table, not a parallel world.
package kind

import (
	"strings"
	"unicode"
)

// The two built-in kinds. They are not rows in the table: a website without a
// single own kind still has these, and an installation that never opens the
// screen never learns that the table exists.
const (
	Page = "page"
	Post = "post"
)

// MaxTypes bounds how many own kinds one website may define.
//
// Editorial rather than technical: every kind is an entry in a menu somebody
// has to recognise and a filter in a list. Twelve is more than any website
// anybody keeps current.
const MaxTypes = 12

// Sort orders an overview page can have.
const (
	SortNewest = "neueste"
	SortTitle  = "titel"
)

// Type is one own content kind of a website.
type Type struct {
	ID        int64
	WebsiteID int64
	// Key is what stands in pages.kind.
	Key string
	// Name and Plural are the singular and the plural. Both appear on screen —
	// "Neues Produkt" and "Produkte" — and a kind that only knows one of them
	// writes "Produkt (3)" into a list.
	Name   string
	Plural string
	// Archive is the address of the overview page, or empty for a kind without
	// one. Reserved like the archive of the posts: a page of the same name
	// would never be reachable.
	Archive string
	// Sort is SortNewest or SortTitle.
	Sort     string
	Position int
}

// HasArchive reports whether this kind has an overview page.
func (t Type) HasArchive() bool { return t.Archive != "" }

// SortsByTitle reports whether the overview is alphabetical.
func (t Type) SortsByTitle() bool { return t.Sort == SortTitle }

// reserved are the keys an own kind may not take.
//
// The first two are the built-in kinds; the rest are the words the rest of the
// program uses for something else, and a kind called "seite" would be a filter
// nobody could tell apart from the built-in one.
var reserved = map[string]bool{
	Page: true, Post: true,
	"seite": true, "beitrag": true, "beides": true, "alle": true, "": true,
}

// Reserved reports whether a key is one this program keeps for itself.
func Reserved(key string) bool { return reserved[strings.ToLower(strings.TrimSpace(key))] }

// Key turns a name into a key: lower case, letters and digits, no umlauts.
//
// Derived once from the name and fixed afterwards, like a field's key: it
// stands in pages.kind on every entry, and changing it would mean rehanging
// every one of them.
func Key(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'ä':
			b.WriteString("ae")
		case r == 'ö':
			b.WriteString("oe")
		case r == 'ü':
			b.WriteString("ue")
		case r == 'ß':
			b.WriteString("ss")
		case unicode.IsSpace(r), r == '-', r == '_':
			// A key travels in a form field name and in a query string; a
			// hyphen there is one escaping rule more for nothing.
			b.WriteString("_")
		}
	}
	key := strings.Trim(b.String(), "_")
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	if len(key) > 30 {
		key = strings.Trim(key[:30], "_")
	}
	return key
}

// ValidKey reports whether a key is one this can work with: two to thirty
// characters, lower-case letters, digits and underscores, starting with a
// letter.
func ValidKey(key string) bool {
	if len(key) < 2 || len(key) > 30 || Reserved(key) {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9', r == '_':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// NameOf is what to call one kind on screen, plural when asked.
//
// The built-in two are answered here so a caller never has to special-case
// them; an unknown key comes back as itself, because a kind that was deleted
// while entries still carry it must not make those entries nameless.
func NameOf(types []Type, key string, plural bool) string {
	switch key {
	case Post:
		if plural {
			return "Beiträge"
		}
		return "Beitrag"
	case Page, "":
		if plural {
			return "Seiten"
		}
		return "Seite"
	}
	for _, t := range types {
		if t.Key == key {
			if plural {
				return t.Plural
			}
			return t.Name
		}
	}
	return key
}

// Find returns the own kind with this key, or false.
func Find(types []Type, key string) (Type, bool) {
	for _, t := range types {
		if t.Key == key {
			return t, true
		}
	}
	return Type{}, false
}

// ByArchive returns the kind whose overview page lives at this address.
func ByArchive(types []Type, slug string) (Type, bool) {
	if slug == "" {
		return Type{}, false
	}
	for _, t := range types {
		if t.Archive == slug {
			return t, true
		}
	}
	return Type{}, false
}

// Pick narrows a value from a form to a kind this website actually has.
//
// Anything unknown becomes a page. A form can be sent with any value in it, and
// an entry filed under a kind the website does not have would be invisible in
// every list without anybody being able to see why.
func Pick(raw string, types []Type) string {
	raw = strings.TrimSpace(raw)
	if raw == Post {
		return Post
	}
	for _, t := range types {
		if t.Key == raw {
			return raw
		}
	}
	return Page
}
