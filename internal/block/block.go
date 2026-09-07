// Package block is the content editor that is not a text field.
//
// A page has always been one piece of Markdown. That is the right thing for an
// article and the wrong thing for a start page: a photo beside a paragraph, a
// row of three cards, a gallery — none of them are expressible in Markdown, and
// an editor who wants them ends up pasting HTML into the text, which the
// sanitiser then throws half of away.
//
// So a page can instead be a list of blocks. Each block has a type and a few
// fields, and the host turns it into markup — the wrapper is written here and
// the editor's words go inside it, escaped. That is why blocks may carry CSS
// classes while Markdown content may not: the classes are the host's, not
// something a visitor's comment could smuggle in.
//
// A page with no blocks behaves exactly as it always did. Blocks are something
// an editor switches on for one page, not a migration everyone is dragged
// through.
package block

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// Types a page can be built from.
//
// Deliberately few. Every type is a thing an editor has to recognise in a menu
// and a theme has to style; twenty of them is a page builder nobody understands
// and every theme renders differently.
const (
	// TypeText is Markdown, exactly as the plain editor writes it. It is what a
	// page converted from Markdown becomes, and what most pages are mostly
	// made of.
	TypeText = "text"
	// TypeImage is one picture, optionally with a caption.
	TypeImage = "bild"
	// TypeImageText is a picture beside a paragraph — the layout a start page
	// asks for first and the one Markdown cannot express at all.
	TypeImageText = "bildtext"
	// TypeGallery is a grid of pictures.
	TypeGallery = "galerie"
	// TypeQuote is a pulled-out sentence with its source.
	TypeQuote = "zitat"
	// TypeCards is a row of small panels, each with a heading, a few words and
	// optionally a picture and a link.
	TypeCards = "karten"
	// TypeDivider is a rule between sections.
	TypeDivider = "trenner"
	// TypeCallout is a boxed invitation with a button: "Wolle bestellen".
	TypeCallout = "aufruf"
	// TypeVideo is a film from this website's own library, in a <video>
	// element. Deliberately not an embed: a YouTube frame is the one thing the
	// rule "nothing from third parties at runtime" forbids, and it is also
	// what makes a cookie banner necessary. A file of one's own needs neither.
	TypeVideo = "video"
)

// Kind describes a type for the editor's menu.
type Kind struct {
	Type string
	Name string
	Hint string
	// HasItems marks the types whose editor grows and shrinks a nested list.
	HasItems bool
}

// Kinds is the menu, in the order it is offered.
var Kinds = []Kind{
	{TypeText, i18n.N("Text"), i18n.N("Ein Abschnitt in Markdown, wie gewohnt."), false},
	{TypeImage, i18n.N("Bild"), i18n.N("Ein Bild, wahlweise über die volle Breite."), false},
	{TypeImageText, i18n.N("Bild und Text"), i18n.N("Ein Bild neben einem Absatz."), false},
	{TypeGallery, i18n.N("Galerie"), i18n.N("Mehrere Bilder als Raster."), true},
	{TypeCards, i18n.N("Karten"), i18n.N("Eine Reihe kleiner Felder mit Titel und Text."), true},
	{TypeQuote, i18n.N("Zitat"), i18n.N("Ein hervorgehobener Satz mit Quelle."), false},
	{TypeCallout, i18n.N("Aufruf"), i18n.N("Ein Kasten mit Knopf."), false},
	{TypeVideo, i18n.N("Video"), i18n.N("Ein eigenes MP4 aus der Mediathek. Kein YouTube."), false},
	{TypeDivider, i18n.N("Trennlinie"), i18n.N("Ein Strich zwischen zwei Abschnitten."), false},
}

// KindOf returns the description of a built-in type, or false.
func KindOf(t string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Type == t {
			return k, true
		}
	}
	return Kind{}, false
}

// Own is a block kind a website defined for itself.
//
// The nine above are the ones every website gets. They are also the ones a
// theme can style without knowing anything about the site — which is exactly
// why there cannot be a tenth built in for one operator's recipe steps. An own
// kind is that tenth: a name, a key, and a handful of fields taken from the
// field system the pages already use.
type Own struct {
	ID     int64
	Key    string
	Name   string
	Hint   string
	Fields []field.Def
}

// Set is the block kinds one website may use: the built-in ones and its own.
//
// Passed to everything that has to decide what a stored type means. The
// alternative — a package-level registry — would be one website's kinds
// answering another website's question, on a server that hosts several.
type Set struct {
	Own []Own
	// Date writes a date the way this website writes dates. Nil leaves the
	// stored form, which is the ISO one — readable, if not pretty.
	//
	// A function rather than a locale, because the rule lives in the template
	// package next to the theme's own formatDate: a date in a block and a date
	// in the theme around it must not be spelled two different ways on the
	// same page.
	Date func(time.Time) string

	// T translates a word the renderer itself writes.
	//
	// A function rather than a language, for the reason Date gives one line
	// above: the rule lives with the caller. Nil leaves the German source,
	// which is what block.Builtin and every test want.
	//
	// Only the words the renderer mints go through it — the lightbox's three
	// controls today. An editor's own text is never touched: translating what
	// somebody typed into a form would be a different program.
	T func(string) string
}

// text translates a word this package wrote itself, or returns it unchanged.
//
// The nil check lives here so no rendering arm has to carry one.
func (s Set) text(word string) string {
	if s.T == nil {
		return word
	}
	return s.T(word)
}

// Builtin is the set of a website that has defined none of its own.
var Builtin = Set{}

// KindOf finds a type, built-in or the website's own.
func (s Set) KindOf(t string) (Kind, bool) {
	if k, ok := KindOf(t); ok {
		return k, true
	}
	if o, ok := s.OwnOf(t); ok {
		return Kind{Type: o.Key, Name: o.Name, Hint: o.Hint}, true
	}
	return Kind{}, false
}

// OwnOf finds one of the website's own kinds.
func (s Set) OwnOf(t string) (Own, bool) {
	for _, o := range s.Own {
		if o.Key == t {
			return o, true
		}
	}
	return Own{}, false
}

// Menu is the whole choice the editor offers, the built-in kinds first.
//
// Own kinds last on purpose: the built-in ones are what nine pages out of ten
// are made of, and a menu that opens with somebody's "Rezeptschritt" makes the
// text block harder to find than it was.
func (s Set) Menu() []Kind {
	out := make([]Kind, 0, len(Kinds)+len(s.Own))
	out = append(out, Kinds...)
	for _, o := range s.Own {
		out = append(out, Kind{Type: o.Key, Name: o.Name, Hint: o.Hint})
	}
	return out
}

// MaxBlocks bounds one page.
//
// Not a guess: the whole list is re-rendered on every structural change in the
// editor, and a page of a thousand blocks would make each of those a slow
// request. Sixty is far more than any page anyone writes.
const MaxBlocks = 60

// MaxItems bounds the nested list of a gallery or a card row.
const MaxItems = 24

// Block is one piece of a page.
//
// One struct with optional fields rather than an interface per type. It costs a
// few empty fields in the JSON and buys a form parser that does not have to
// know which fields belong to which type before it can read them — which is
// exactly the code that would otherwise silently drop a field after a rename.
type Block struct {
	Type string `json:"typ"`

	// Markdown is the prose of a text block and the text beside the picture of
	// an image-text block.
	Markdown string `json:"markdown,omitempty"`

	// MediaID names a picture in this website's library. Zero means none.
	MediaID int64  `json:"medium,omitempty"`
	Alt     string `json:"alt,omitempty"`
	Caption string `json:"bildunterschrift,omitempty"`

	// Variant is the layout choice, and what it means depends on the type:
	// "voll"/"breit"/"normal" for a picture, "links"/"rechts" for the side the
	// picture sits on, "2"/"3"/"4" for how many columns a grid has.
	Variant string `json:"variante,omitempty"`

	// Display is how a gallery is laid out: a grid, or a slideshow that snaps.
	// Empty is the grid that already exists, which is why every page saved
	// before this field keeps rendering exactly as it did.
	//
	// A field of its own rather than another meaning of Variant, because
	// Variant is already this block's column count (see Columns below) and one
	// string that means either a number or a mode is a string nobody can read:
	// a Variant of "diashow" would silently render a three-column grid. The
	// shape is field.Def.Display's, constant and all.
	Display string `json:"darstellung,omitempty"`

	// Title heads a callout; Text and Source are the quote.
	Title  string `json:"titel,omitempty"`
	Text   string `json:"text,omitempty"`
	Source string `json:"quelle,omitempty"`

	// LinkText and LinkURL are the button of a callout.
	LinkText string `json:"linktext,omitempty"`
	LinkURL  string `json:"linkziel,omitempty"`

	// PosterID is the still shown before a video is played. Zero means the
	// browser decides, which usually means a black rectangle.
	PosterID int64 `json:"vorschaubild,omitempty"`

	// Items are the pictures of a gallery or the panels of a card row.
	Items []Item `json:"eintraege,omitempty"`

	// AlbumSlug names an album of this website instead of listing pictures
	// here. Empty is the block that carries its own list, which is every
	// gallery saved before this field existed.
	//
	// The slug and not the id: an id means nothing on the machine a bundle
	// lands on, which is the sentence internal/bundle/blocks.go:11-18 opens
	// with, and the slug is what page.Slugify derives from the name on both
	// sides. The pictures are NOT stored here — that is the whole point. A
	// block naming an album renders to a marker (see AlbumMarker in render.go)
	// and the pictures are looked up at request time, so that changing the
	// album changes every page carrying it without those pages being touched.
	AlbumSlug string `json:"album,omitempty"`

	// Fields are the values of an own kind's fields, by field key. Empty for
	// every built-in type, which has its named fields above.
	Fields map[string]string `json:"felder,omitempty"`
}

// Item is one entry inside a gallery or a card row.
type Item struct {
	MediaID  int64  `json:"medium,omitempty"`
	Alt      string `json:"alt,omitempty"`
	Caption  string `json:"bildunterschrift,omitempty"`
	Title    string `json:"titel,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	LinkURL  string `json:"linkziel,omitempty"`
}

// Empty reports whether a block would render to nothing.
//
// Used to drop the blocks an editor added and then left alone, so a page does
// not end up with three empty boxes because somebody clicked the wrong entry in
// the menu.
func (b Block) Empty() bool {
	switch b.Type {
	case TypeDivider:
		return false
	case TypeText:
		return strings.TrimSpace(b.Markdown) == ""
	case TypeImage, TypeVideo:
		return b.MediaID == 0
	case TypeImageText:
		return b.MediaID == 0 && strings.TrimSpace(b.Markdown) == ""
	case TypeQuote:
		return strings.TrimSpace(b.Text) == ""
	case TypeCallout:
		return strings.TrimSpace(b.Title) == "" && strings.TrimSpace(b.Markdown) == ""
	case TypeGallery, TypeCards:
		// A gallery that takes its pictures from an album carries no items of
		// its own, and it is not empty.
		//
		// The bug this line prevents: without it Clean below drops the block —
		// it drops every block Empty() reports — so the editor picks an album,
		// saves, and the block is gone. Silently: no test failed, no line was
		// logged, no message reached the screen. The next person to give a
		// block a second source for its content will land here again, which is
		// why the bug is named and not only the rule.
		if b.Type == TypeGallery && strings.TrimSpace(b.AlbumSlug) != "" {
			return false
		}
		for _, it := range b.Items {
			if it.MediaID != 0 || strings.TrimSpace(it.Title) != "" ||
				strings.TrimSpace(it.Markdown) != "" {
				return false
			}
		}
		return true
	}
	// An own kind: filled in when any one of its fields is.
	for _, v := range b.Fields {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// DisplaySlideshow lays a gallery out as a horizontal track that snaps to each
// picture. Everything it needs is CSS: the keyboard and the touch gesture are
// the browser's own scrolling, and nothing here is scripted.
//
// The identifier is English and the value it holds is German, deliberately.
// The string stands in the blocks column of every page that uses it and in
// every archive ever exported, and .planning/GLOSSARY.md's closing rule —
// written after a runtime failure of exactly this kind — says a German word
// standing in the database is a value and not an identifier. knopfreihe in
// internal/field is the same shape, one package over.
const DisplaySlideshow = "diashow"

// DisplayClass is the modifier class this block's display mode adds to its
// wrapper, or the empty string for the layout that already exists.
//
// The class is minted here from the constant and never concatenated out of the
// stored value, so a display an archive carries that this program does not know
// adds no modifier at all rather than a fragment of an attribute. The mapping
// lives beside the constant so the renderer carries no switch.
//
// Only a gallery has a display today, which is why the class names one.
func (b Block) DisplayClass() string {
	if b.Display == DisplaySlideshow {
		return "hc-galerie--diashow"
	}
	return ""
}

// Columns is how many columns a grid block asks for, clamped to what a layout
// can actually do.
func (b Block) Columns() int {
	switch b.Variant {
	case "2":
		return 2
	case "4":
		return 4
	default:
		return 3
	}
}

// Encode turns a list of blocks into what the database stores.
//
// An empty list encodes to the empty string rather than "[]", so "this page has
// no blocks" is one value everywhere and not two that behave the same until
// somebody compares them.
func Encode(blocks []Block, s Set) (string, error) {
	blocks = s.Clean(blocks)
	if len(blocks) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		return "", fmt.Errorf("bausteine sichern: %w", err)
	}
	return string(raw), nil
}

// Decode reads what Encode wrote. An empty string is no blocks, not an error.
func Decode(raw string, s Set) ([]Block, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var blocks []Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return nil, fmt.Errorf("bausteine lesen: %w", err)
	}
	return s.Clean(blocks), nil
}

// Clean drops what cannot be rendered and bounds what is left.
//
// Applied on the way in and on the way out. On the way in because an editor
// leaves empty blocks behind; on the way out because a row written by an older
// version, or by hand, must not be able to produce a block type this build has
// never heard of.
func (s Set) Clean(blocks []Block) []Block {
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		own, isOwn := s.OwnOf(b.Type)
		if _, ok := s.KindOf(b.Type); !ok {
			continue
		}
		if b.Empty() {
			continue
		}
		// A value whose field was removed from the kind goes with it, and
		// everything else is trimmed. Same rule as the page's own fields: it
		// happens on the next save, which is late enough to be forgiving.
		if isOwn {
			b.Fields = keepFields(own, b.Fields)
		} else {
			b.Fields = nil
		}
		if len(b.Items) > MaxItems {
			b.Items = b.Items[:MaxItems]
		}
		if b.Type == TypeGallery || b.Type == TypeCards {
			items := make([]Item, 0, len(b.Items))
			for _, it := range b.Items {
				if it.MediaID == 0 && strings.TrimSpace(it.Title) == "" &&
					strings.TrimSpace(it.Markdown) == "" {
					continue
				}
				items = append(items, it)
			}
			b.Items = items
		} else {
			b.Items = nil
		}
		// An album belongs to a gallery. A card row has items too and would
		// otherwise be able to carry a slug through the archive that no
		// renderer anywhere reads.
		if b.Type != TypeGallery {
			b.AlbumSlug = ""
		}
		out = append(out, b)
		if len(out) >= MaxBlocks {
			break
		}
	}
	return out
}

// MaxFieldBytes bounds one value of an own kind's field.
const MaxFieldBytes = 4000

// keepFields drops what the kind no longer has and trims what is left.
func keepFields(own Own, values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, d := range own.Fields {
		v := strings.TrimSpace(values[d.Key])
		if v == "" {
			continue
		}
		if len(v) > MaxFieldBytes {
			v = v[:MaxFieldBytes]
		}
		out[d.Key] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
