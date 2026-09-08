package block

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	// For i18n.N alone, which marks a literal so the collector can see it.
	// i18n is a leaf with no database and no HTTP, so the purity this
	// package's own doc comment claims is untouched.
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// Image is what the renderer needs to know about one picture.
//
// Passed in rather than looked up here: this package has no database and no
// media store, which is what makes it testable with a map and keeps the media
// rules — variants, paths, thumbnails — in one place instead of two.
type Image struct {
	// URL is the same-origin path a browser requests.
	URL string
	// Alt is the library's description, used when the block does not override
	// it. A picture with neither is rendered with alt="" — decorative — rather
	// than with a filename, which is worse than nothing for a screen reader.
	Alt string
	// Width and Height let a browser reserve the space before the file
	// arrives, which is what stops a page from jumping while it loads.
	Width, Height int
	// Film marks a video file rather than a picture. One lookup for both,
	// because both are the same thing to this package — a file of this
	// website's, with an address — and the difference decides only which
	// element it belongs in. Without it a picture block could point at an MP4
	// and produce an <img> nobody can see.
	Film bool
	// Focus is the object-position of the subject, or empty for a picture
	// centred on itself. It matters wherever a block squeezes a picture into a
	// fixed shape — a gallery tile, a card — because a browser otherwise cuts
	// from the middle, and an animal at the left edge is then cut off every
	// time.
	Focus string
}

// Nothing here writes a srcset. The public pipeline already runs every page
// through media.MakeResponsive, which knows which variants exist and adds them
// — and which leaves a hand-written sizes alone. So a block that knows better
// than the default, a gallery tile at a third of the width, says so with sizes
// and lets that pass do the rest. Two places computing srcsets would be two
// places to get the variant naming wrong.

// Lookup resolves a media id to a picture, or returns false.
type Lookup func(mediaID int64) (Image, bool)

// Markdown converts a block's prose. It is the host's own renderer, passed in
// for the same reason as Lookup.
type Markdown func(src string) (string, error)

// Render turns a page's blocks into HTML.
//
// The markup is the host's and carries classes; only the editor's words go
// inside it, escaped or run through the Markdown sanitiser. That is the whole
// reason blocks may be styled at all: a class here was written in this file,
// not typed into a form.
//
// A block that cannot be rendered is skipped rather than failing the page. A
// picture that was deleted from the library should cost its own block, never
// the article around it.
//
// The loop's index travels with the block because a gallery mints fragment ids
// from it, and two galleries on one page must not mint the same one twice.
func Render(blocks []Block, s Set, look Lookup, md Markdown) string {
	var b strings.Builder
	for i, blk := range blocks {
		if own, ok := s.OwnOf(blk.Type); ok {
			renderOwn(&b, i, blk, own, s, look, md)
			continue
		}
		renderOne(&b, i, blk, s, look, md)
	}
	return b.String()
}

// renderOne writes one built-in block.
//
// at is the block's position in the page, counted from zero. Only the gallery
// arm uses it, to mint fragment ids; it is passed to every arm rather than to
// that one so the two rendering functions keep the same shape.
//
// s is here for the words this file writes itself — the lightbox's three
// controls — which is the same reason renderOwn has always had it.
func renderOne(b *strings.Builder, at int, blk Block, s Set, look Lookup, md Markdown) {
	switch blk.Type {
	case TypeText:
		if h := prose(blk.Markdown, md); h != "" {
			fmt.Fprintf(b, `<div class="hc-block hc-text">%s</div>`, h)
		}

	case TypeImage:
		img, ok := look(blk.MediaID)
		if !ok || img.Film {
			return
		}
		class := "hc-block hc-bild"
		switch blk.Variant {
		case "voll":
			class += " hc-bild--voll"
		case "breit":
			class += " hc-bild--breit"
		}
		fmt.Fprintf(b, `<figure class="%s">%s`, class, imgTag(img, blk.Alt, "", false))
		if c := strings.TrimSpace(blk.Caption); c != "" {
			fmt.Fprintf(b, `<figcaption>%s</figcaption>`, html.EscapeString(c))
		}
		b.WriteString(`</figure>`)

	case TypeVideo:
		film, ok := look(blk.MediaID)
		if !ok || !film.Film {
			return
		}
		class := "hc-block hc-video"
		switch blk.Variant {
		case "voll":
			class += " hc-video--voll"
		case "breit":
			class += " hc-video--breit"
		}
		// controls, sonst nichts: kein autoplay, kein loop, kein muted-Trick.
		// preload="metadata" holt die Länge und das erste Bild, nicht den Film
		// — auf einem Mobilanschluss ist das der Unterschied zwischen einer
		// Seite und einem Download.
		fmt.Fprintf(b, `<figure class="%s"><video controls playsinline preload="metadata"`, class)
		if poster, ok := look(blk.PosterID); ok && !poster.Film {
			fmt.Fprintf(b, ` poster="%s"`, html.EscapeString(poster.URL))
		}
		fmt.Fprintf(b, `><source src="%s" type="video/mp4">%s</video>`,
			html.EscapeString(film.URL),
			html.EscapeString("Dein Browser kann dieses Video nicht abspielen."))
		if c := strings.TrimSpace(blk.Caption); c != "" {
			fmt.Fprintf(b, `<figcaption>%s</figcaption>`, html.EscapeString(c))
		}
		b.WriteString(`</figure>`)

	case TypeImageText:
		img, hasImg := look(blk.MediaID)
		if hasImg && img.Film {
			hasImg = false
		}
		text := prose(blk.Markdown, md)
		if !hasImg && text == "" {
			return
		}
		class := "hc-block hc-bildtext"
		if blk.Variant == "rechts" {
			class += " hc-bildtext--rechts"
		}
		fmt.Fprintf(b, `<div class="%s">`, class)
		if hasImg {
			fmt.Fprintf(b, `<div class="hc-bildtext__bild">%s`, imgTag(img, blk.Alt, "", false))
			if c := strings.TrimSpace(blk.Caption); c != "" {
				fmt.Fprintf(b, `<p class="hc-bildunterschrift">%s</p>`, html.EscapeString(c))
			}
			b.WriteString(`</div>`)
		}
		if text != "" {
			fmt.Fprintf(b, `<div class="hc-bildtext__text">%s</div>`, text)
		}
		b.WriteString(`</div>`)

	case TypeGallery:
		// Two sources, one rule, and the rule is stated because a hand-edited
		// archive can produce what the editor cannot: a block carrying an album
		// AND its own items renders the album. The album is the more explicit
		// choice, and concatenating the two would mint two runs of fragment ids
		// from one block position — hc-b1-p1 twice on one page.
		var inner string
		if slug := strings.TrimSpace(blk.AlbumSlug); slug != "" {
			// The wrapper is written now and only its contents are late. The
			// columns and the display are properties of the block and known at
			// save; only the pictures belong to the album. A marker that had to
			// carry them would be a second encoding of what the class attribute
			// below already says.
			inner = AlbumMarker(slug, at)
		} else {
			inner = GalleryItems(at, blk.Items, look, s.text)
		}
		if inner == "" {
			return
		}
		// One markup, two stylesheets. The grid and the slideshow differ in the
		// wrapper's attributes and in nothing else — the tiles, the large
		// views, the fragment ids and the controls are GalleryItems' output and
		// identical in both. That is what makes a display mode one block of CSS
		// rather than a second renderer, and it is what keeps the lightbox
		// working in both modes for nothing.
		if mod := blk.DisplayClass(); mod != "" {
			// The tab stop is not decoration. A region that scrolls
			// horizontally and cannot be focused is a slideshow only a pointer
			// can use, and browsers differ on whether they hand a scroll
			// container a tab stop of its own — so it is stated rather than
			// assumed. A focusable region needs a name to be worth entering,
			// and role="region" is what exposes that name to a screen reader.
			//
			// mod comes from DisplayClass, which mints it from the constant, so
			// nothing an editor or a hand-edited archive holds is concatenated
			// into this attribute (T-11-18).
			fmt.Fprintf(b,
				`<div class="hc-block hc-galerie hc-spalten-%d %s" tabindex="0" role="region" aria-label="%s">%s</div>`,
				blk.Columns(), mod, html.EscapeString(s.text(textGallery)), inner)
			return
		}
		fmt.Fprintf(b, `<div class="hc-block hc-galerie hc-spalten-%d">%s</div>`,
			blk.Columns(), inner)

	case TypeCards:
		var inner strings.Builder
		for _, it := range blk.Items {
			var card strings.Builder
			if img, ok := look(it.MediaID); ok {
				card.WriteString(imgTag(img, it.Alt, "(min-width: 50em) 30vw, 90vw", true))
			}
			if t := strings.TrimSpace(it.Title); t != "" {
				fmt.Fprintf(&card, `<h3 class="hc-karte__titel">%s</h3>`, html.EscapeString(t))
			}
			if h := prose(it.Markdown, md); h != "" {
				fmt.Fprintf(&card, `<div class="hc-karte__text">%s</div>`, h)
			}
			if card.Len() == 0 {
				continue
			}
			// The whole card is a link when one is given, so the target is the
			// card and not a word inside it — a card with a "more" link that
			// only works on three characters is a card nobody hits on a phone.
			if u := safeURL(it.LinkURL); u != "" {
				fmt.Fprintf(&inner, `<a class="hc-karte hc-karte--link" href="%s">%s</a>`,
					html.EscapeString(u), card.String())
			} else {
				fmt.Fprintf(&inner, `<div class="hc-karte">%s</div>`, card.String())
			}
		}
		if inner.Len() == 0 {
			return
		}
		fmt.Fprintf(b, `<div class="hc-block hc-karten hc-spalten-%d">%s</div>`,
			blk.Columns(), inner.String())

	case TypeQuote:
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			return
		}
		b.WriteString(`<figure class="hc-block hc-zitat"><blockquote><p>`)
		b.WriteString(strings.ReplaceAll(html.EscapeString(text), "\n", "<br>"))
		b.WriteString(`</p></blockquote>`)
		if q := strings.TrimSpace(blk.Source); q != "" {
			fmt.Fprintf(b, `<figcaption>%s</figcaption>`, html.EscapeString(q))
		}
		b.WriteString(`</figure>`)

	case TypeCallout:
		fmt.Fprintf(b, `<div class="hc-block hc-aufruf">`)
		if t := strings.TrimSpace(blk.Title); t != "" {
			fmt.Fprintf(b, `<h2 class="hc-aufruf__titel">%s</h2>`, html.EscapeString(t))
		}
		if h := prose(blk.Markdown, md); h != "" {
			fmt.Fprintf(b, `<div class="hc-aufruf__text">%s</div>`, h)
		}
		if u := safeURL(blk.LinkURL); u != "" {
			label := strings.TrimSpace(blk.LinkText)
			if label == "" {
				label = "Mehr erfahren"
			}
			fmt.Fprintf(b, `<p class="hc-aufruf__knopf"><a class="hc-knopf" href="%s">%s</a></p>`,
				html.EscapeString(u), html.EscapeString(label))
		}
		b.WriteString(`</div>`)

	case TypeDivider:
		b.WriteString(`<hr class="hc-block hc-trenner">`)
	}
}

// renderOwn writes a block of a kind the website defined.
//
// The markup is fixed and the styling is the theme's: a wrapper carrying the
// kind's key, and one element per filled-in field carrying that field's key. So
// a "Rezeptschritt" becomes
//
//	<div class="hc-block hc-eigen hc-eigen--rezeptschritt">
//	  <p class="hc-eigen__zeile hc-eigen__zeile--nummer">3</p>
//	  <div class="hc-eigen__text hc-eigen__text--anleitung">…</div>
//	</div>
//
// and the theme's stylesheet decides what that looks like. The alternative — a
// piece of HTML the operator writes per kind — would mean a template language
// in a text field, which is a way to put a <script> on a page through the front
// door, and this whole program is built on the promise that nothing does that.
//
// The escape hatch for anything a class cannot do is the long-text field: it
// goes through the Markdown renderer, so an editor writes "### Schritt 3" and
// gets a heading, sanitised by the same pass as every other piece of prose.
// at is the block's position, unused here and taken anyway: renderOne needs it
// for the gallery's fragment ids, and a reviewer reading one arm and not the
// other should not have to work out whether the asymmetry means something.
func renderOwn(b *strings.Builder, at int, blk Block, own Own, s Set, look Lookup, md Markdown) {
	_ = at
	class := "hc-block hc-eigen hc-eigen--" + html.EscapeString(own.Key)
	var inner strings.Builder

	for _, d := range own.Fields {
		value := strings.TrimSpace(blk.Fields[d.Key])
		if value == "" {
			continue
		}
		key := html.EscapeString(d.Key)

		switch d.Kind {
		case field.KindBool:
			if value != "0" {
				// A yes/no changes how the block looks rather than adding a
				// line to it — printing the word "ja" would be a label nobody
				// wants on their page.
				class += " hc-ja--" + key
			}

		case field.KindImage:
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue
			}
			img, ok := look(id)
			if !ok || img.Film {
				continue
			}
			fmt.Fprintf(&inner, `<figure class="hc-eigen__bild hc-eigen__bild--%s">%s</figure>`,
				key, imgTag(img, "", "", false))

		case field.KindLong:
			if h := prose(value, md); h != "" {
				fmt.Fprintf(&inner, `<div class="hc-eigen__text hc-eigen__text--%s">%s</div>`, key, h)
			}

		case field.KindLink:
			u := safeURL(value)
			if u == "" {
				continue
			}
			// The label is the link text: a block kind has no second field for
			// it, and "Zum Rezept" is what the operator called the field.
			fmt.Fprintf(&inner, `<p class="hc-eigen__link hc-eigen__link--%s"><a href="%s">%s</a></p>`,
				key, html.EscapeString(u), html.EscapeString(d.Label))

		case field.KindDate:
			shown := value
			if t, err := time.Parse("2006-01-02", value); err == nil && s.Date != nil {
				shown = s.Date(t)
			}
			fmt.Fprintf(&inner,
				`<p class="hc-eigen__datum hc-eigen__datum--%s"><time datetime="%s">%s</time></p>`,
				key, html.EscapeString(value), html.EscapeString(shown))

		case field.KindCode:
			// Hier und nicht im Theme. Ein Baustein wird beim Speichern der
			// Seite zu HTML eingefroren, und diese Bytes bekommt der Besucher
			// — im Theme zu maskieren wäre zu spät, dann steht das rohe Tag
			// längst in der Datenbank. html.EscapeString ist dieselbe
			// Hausregel wie im default-Zweig; prose() steht bewusst nicht
			// hier, denn das ist der Markdown-Weg, und ein Codefeld
			// verspricht gerade, nicht gedeutet zu werden.
			fmt.Fprintf(&inner, `<pre class="hc-eigen__code hc-eigen__code--%s"><code>%s</code></pre>`,
				key, html.EscapeString(value))

		default:
			fmt.Fprintf(&inner, `<p class="hc-eigen__zeile hc-eigen__zeile--%s">%s</p>`,
				key, html.EscapeString(value))
		}
	}

	if inner.Len() == 0 && !strings.Contains(class, " hc-ja--") {
		return
	}
	fmt.Fprintf(b, `<div class="%s">%s</div>`, class, inner.String())
}

// The three names the lightbox writes, and they are new words on purpose.
//
// Measured against internal/i18n/locales/en.json: "Weiter" is already in there
// as "Continue" and "Zurück" as "Back", and both are the wrong sentence on a
// picture. Reusing an existing key would ship the wrong word in four languages
// with every gate green, so three fresh German literals are minted instead.
//
// i18n.N marks them so `go run ./tools/i18n` collects them; it translates
// nothing. Marking is only half the job: this file carries no locale, so the
// words are translated through the function on Set — see Set.T. A string that
// is marked and never injected is collected, translated into four catalogues
// and printed in German anyway.
var (
	textPrevious = i18n.N("Vorheriges Bild")
	textNext     = i18n.N("Nächstes Bild")
	textClose    = i18n.N("Grossansicht schliessen")

	// textGallery names the slideshow's scrolling region. Deliberately the
	// literal the block kind at block.go:77 already carries, so this mints no
	// fifth string: it is translated in en, es, fr and it today, and a new one
	// would cost four translations for a word the catalogue already has.
	textGallery = i18n.N("Galerie")
)

// closeTarget is the fragment the close control points at.
//
// It names no element on the page, so following it returns :target to no match
// and the large view disappears again. Not href="#": that scrolls to the top of
// the document and adds a history entry the back button then has to walk back
// through. Under the hc- prefix like every id this file mints, so a heading in
// a text block cannot collide with it.
const closeTarget = "hc-zu"

// The album marker: what a gallery block naming an album renders to, and the
// one place its spelling is written down.
//
// Block HTML is rendered once, on save, and stored —
// internal/admin/page_blocks.go says so in its own comment. So an album
// expanded here would freeze a copy of its pictures into every page carrying
// it, GAL-03 ("changing the album changes every page that carries it, without
// touching those pages") would be false, and NOTHING anywhere would report it:
// no test fails, no line is logged, and the editor simply reloads the page and
// sees the old pictures. The marker is what makes the expansion late.
//
// internal/snippet/store.go:229-264 is the same mechanism for the same reason,
// and internal/public/pagedata.go states it in those words.
//
// # Why the spelling lives in this package and not in internal/album
//
// internal/album already imports this package for block.Item, so a marker
// declared there and written here would be an import cycle. It went this way
// round rather than the other, and internal/album/expand.go calls the four
// functions below instead of writing the string a second time. Two spellings
// of one marker in two packages drift invisibly: the page still renders, and
// the marker simply stays visible on it.
//
// # What this marker must survive, and what it need not
//
// Half of the snippet marker's reasoning does not apply here. A snippet marker
// is typed by an editor into Markdown and has to survive goldmark and
// bluemonday, which is what its pattern is shaped for. This one is written by
// this file into HTML this program controls and meets neither. What it must
// survive is media.MakeResponsive, which parses the fragment with
// golang.org/x/net/html and re-renders it: a text node passes through
// unchanged. That is a property to state and to test rather than to assume —
// TestTheAlbumMarkerIsPlainTextInsideAnElement here, and
// TestExpandedAlbumPicturesGetTheirSrcSet in internal/public for the other end.
const albumMarkerPrefix = "[[album:"

// albumMarkerPattern matches the marker albumMarkerPrefix opens.
//
// The slug is page.Slugify's alphabet and nothing else. The number after it is
// the block's position in the page, because the fragment ids of the large views
// are minted from it (D-03) — a page carrying an inline gallery and an album
// gallery must not mint hc-b1-p1 twice.
var albumMarkerPattern = regexp.MustCompile(`\[\[album:([a-z0-9][a-z0-9-]*):(\d+)\]\]`)

// AlbumMarker is what a gallery block naming an album renders to.
//
// Built from the prefix above rather than spelled out a second time, so the
// writer and the reader cannot disagree.
func AlbumMarker(slug string, at int) string {
	return albumMarkerPrefix + slug + ":" + strconv.Itoa(at) + "]]"
}

// hasAlbumMarker reports whether a document contains any album marker at all.
//
// The cheap question, asked before the expensive one: a page that names no
// album must cost nothing — no regular expression over its whole body and no
// database query. snippet.Expand opens with the same test for the same reason.
//
// Unexported, and that is a decision rather than tidiness. It was exported for
// a caller in internal/public that would have read "does this body name an
// album, and if so expand it against an empty set". That caller does not exist
// and now cannot: pagedata.go expands unconditionally, because deciding NOT to
// expand is what printed the marker to a visitor. AlbumMarkerSlugs and
// ReplaceAlbumMarkers are the two exported readers, and both open with this
// test themselves, so no caller outside this package has to ask it first.
func hasAlbumMarker(html string) bool {
	return strings.Contains(html, albumMarkerPrefix)
}

// AlbumMarkerSlugs returns the album slugs a document names, deduplicated and
// in the order they first appear.
//
// This is what turns "load only what the page names" from an intention into a
// query, the way media.LoadImageSets reads the file names out of the HTML
// before looking anything up.
func AlbumMarkerSlugs(html string) []string {
	if !hasAlbumMarker(html) {
		return nil
	}
	seen := map[string]bool{}
	var slugs []string
	for _, m := range albumMarkerPattern.FindAllStringSubmatch(html, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			slugs = append(slugs, m[1])
		}
	}
	return slugs
}

// ReplaceAlbumMarkers replaces every marker with what expand returns for it.
//
// A document with no marker is returned unchanged and untouched, which is what
// keeps this mechanism free for the pages that do not use it.
func ReplaceAlbumMarkers(html string, expand func(slug string, at int) string) string {
	if !hasAlbumMarker(html) {
		return html
	}
	return albumMarkerPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := albumMarkerPattern.FindStringSubmatch(match)
		at, err := strconv.Atoi(parts[2])
		if err != nil {
			return ""
		}
		return expand(parts[1], at)
	})
}

// GalleryItems renders one gallery: every tile first, then every large view.
//
// A lightbox without a script.
//
// Opening a picture is a fragment in the URL and nothing else. The tile is an
// anchor, the large view is a <figure> carrying that id, and bausteine.css
// keeps every large view out of the layout until :target names one. A scripted
// overlay is not an option here — this project permits htmx and nothing else,
// and internal/tmplmgr/script.go rejects one outright in an uploaded theme. The
// fragment buys the browser's back button for free, because the state sits in
// the URL where the browser already keeps it.
//
// The markup is shaped so that the unstyled case is a page and not a wreck. The
// large view is a sibling of the tiles inside the same <div>, in the flow, with
// its own caption: with the stylesheet blocked it renders as the same picture
// again at natural size and the anchors still work. Nothing here is a wrapper
// that only makes sense once it is positioned, and nothing here is chrome —
// until today a theme written by following TEMPLATE-SPEC.md linked no
// bausteine.css at all, so the unstyled case is a real case.
//
// Exported because there is one gallery renderer and not two: the gallery block
// calls it with the items an editor picked, and an album calls it with the
// items it loaded. Two renderers would be two places to get the fragment ids
// and the step links wrong, and the album is the one nobody would notice.
//
// at is the block's position in the page, counted from zero. t translates the
// three control names; nil leaves them in their German source.
func GalleryItems(at int, items []Item, look Lookup, t func(string) string) string {
	if t == nil {
		t = func(s string) string { return s }
	}

	// A picture keeps the number of its own place in the list, so a media id
	// that no longer resolves does not renumber the pictures after it. The
	// resolved ones are collected first all the same, because a step link has
	// to point at an id that exists: with the second picture gone, the third
	// steps back to the first.
	//
	// These ids are minted at save and then stored. Block HTML is rendered once
	// and written into pages.content_html (internal/admin/page_blocks.go says
	// so in its own comment), so a fragment is frozen until that page is saved
	// again — and reordering the blocks on a later save re-mints it, which can
	// land an old bookmark on a different picture. That is acceptable for a
	// bookmark into the middle of a photo grid, and it is written down here
	// rather than left to be discovered.
	type shown struct {
		id  string
		img Image
		it  Item
	}
	var pictures []shown
	for j, it := range items {
		img, ok := look(it.MediaID)
		if !ok {
			continue
		}
		// The block's position and the item's index, so two galleries on one
		// page mint two sets of ids. The hc- prefix is not decoration: a
		// heading in a text block on the same page carries an id the Markdown
		// renderer derived from its words, and a bare b1-p1 is a collision
		// waiting for the one page with a heading spelled that way.
		pictures = append(pictures, shown{
			id:  fmt.Sprintf("hc-b%d-p%d", at+1, j+1),
			img: img,
			it:  it,
		})
	}
	if len(pictures) == 0 {
		return ""
	}

	var b strings.Builder
	for _, p := range pictures {
		b.WriteString(`<figure class="hc-galerie__bild">`)
		// The anchor wraps the picture alone. Leaving the caption outside it
		// keeps the link's name to the picture's own description instead of a
		// paragraph.
		fmt.Fprintf(&b, `<a class="hc-galerie__oeffnen" href="#%s">`, p.id)
		b.WriteString(imgTag(p.img, p.it.Alt, "(min-width: 50em) 30vw, 90vw", true))
		b.WriteString(`</a>`)
		if c := strings.TrimSpace(p.it.Caption); c != "" {
			fmt.Fprintf(&b, `<figcaption>%s</figcaption>`, html.EscapeString(c))
		}
		b.WriteString(`</figure>`)
	}

	for k, p := range pictures {
		// tabindex="-1" so that a keyboard user who followed the tile anchor
		// carries on from the picture they just opened rather than from where
		// they were. There is no script here to move focus for them.
		fmt.Fprintf(&b, `<figure class="hc-galerie__gross" id="%s" tabindex="-1">`, p.id)
		// No variant is named and no path is built: imgTag writes the original
		// address and the public pipeline adds the candidate list at request
		// time, which is the rule stated at the top of this file. Uncropped,
		// because nothing squeezes the large view into a fixed shape and the
		// focus point would change nothing there.
		b.WriteString(imgTag(p.img, p.it.Alt, "100vw", false))
		if c := strings.TrimSpace(p.it.Caption); c != "" {
			fmt.Fprintf(&b, `<figcaption>%s</figcaption>`, html.EscapeString(c))
		}
		b.WriteString(`<nav class="hc-galerie__steuerung">`)
		// Absent at each end rather than disabled, and never a wrap-around: a
		// list of four holiday photos that jumps back to the first is a
		// surprise, and the browser's own back button is the way out.
		if k > 0 {
			fmt.Fprintf(&b, `<a class="hc-galerie__zurueck" href="#%s">%s</a>`,
				pictures[k-1].id, html.EscapeString(t(textPrevious)))
		}
		if k < len(pictures)-1 {
			fmt.Fprintf(&b, `<a class="hc-galerie__weiter" href="#%s">%s</a>`,
				pictures[k+1].id, html.EscapeString(t(textNext)))
		}
		fmt.Fprintf(&b, `<a class="hc-galerie__schliessen" href="#%s">%s</a>`,
			closeTarget, html.EscapeString(t(textClose)))
		b.WriteString(`</nav></figure>`)
	}
	return b.String()
}

// prose runs an editor's Markdown through the host's renderer.
//
// A failure yields nothing rather than the raw text: unrendered Markdown on a
// live page looks like a bug an editor cannot fix, and the same text is still
// in the form where they typed it.
func prose(src string, md Markdown) string {
	src = strings.TrimSpace(src)
	if src == "" || md == nil {
		return ""
	}
	out, err := md(src)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// imgTag writes one picture.
//
// cropped says the picture will be squeezed into a fixed shape by the
// stylesheet, which is the only case where the focus point changes anything.
func imgTag(img Image, alt, sizes string, cropped bool) string {
	if strings.TrimSpace(alt) == "" {
		alt = img.Alt
	}
	var b strings.Builder
	b.WriteString(`<img src="` + html.EscapeString(img.URL) + `"`)
	b.WriteString(` alt="` + html.EscapeString(strings.TrimSpace(alt)) + `"`)
	if sizes != "" {
		b.WriteString(` sizes="` + html.EscapeString(sizes) + `"`)
	}
	if img.Width > 0 && img.Height > 0 {
		b.WriteString(` width="` + strconv.Itoa(img.Width) + `"`)
		b.WriteString(` height="` + strconv.Itoa(img.Height) + `"`)
	}
	if cropped && img.Focus != "" {
		// An inline style rather than a class: the value is per picture, and a
		// stylesheet cannot carry one rule per image in the library.
		b.WriteString(` style="object-position:` + html.EscapeString(img.Focus) + `"`)
	}
	// Every block image is below the fold often enough that lazy is the right
	// default, and a theme that wants otherwise for its hero has its own markup.
	b.WriteString(` loading="lazy" decoding="async">`)
	return b.String()
}

// safeURL allows a path on this site, an absolute http(s) address, a mail
// address or a telephone number — and nothing else.
//
// The value is typed into a form by an editor, and an editor who pastes
// something odd should get a card without a link, not a page that runs it.
func safeURL(raw string) string {
	u := strings.TrimSpace(raw)
	switch {
	case u == "":
		return ""
	case strings.HasPrefix(u, "//"):
		// Protocol-relative: leaves the site while looking like a path.
		return ""
	case strings.HasPrefix(u, "/"), strings.HasPrefix(u, "#"):
		return u
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"),
		strings.HasPrefix(u, "mailto:"), strings.HasPrefix(u, "tel:"):
		return u
	}
	return ""
}

// PlainText is the words of a page without any markup.
//
// The excerpt, the search index and the meta description are all built from the
// page's text, and all of them used to read the Markdown column. A block page
// has no Markdown column worth reading, so it gets one made from its blocks —
// otherwise a page built from blocks would be invisible to the site's own
// search, which is the kind of gap nobody notices until a visitor does.
func PlainText(blocks []Block, s Set) string {
	var parts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	for _, b := range blocks {
		if own, ok := s.OwnOf(b.Type); ok {
			// Only the fields that hold words. A picture's id and a date are
			// not something anybody searches for, and "12" in the excerpt of a
			// recipe is worse than nothing.
			//
			// Ein Codefeld hält Worte — eine Adresse, eine Zeile Einstellung,
			// ein Schnipsel — und steht deshalb dabei: eine Seite aus
			// Bausteinen wäre für ihre eigene Suche sonst gerade dort
			// unsichtbar, wo der Verfasser sich am meisten Mühe gab. Eine
			// Uhrzeit und ein Bereich stehen aus demselben Grund draussen wie
			// eine Bildnummer und ein Datum: das sucht niemand.
			for _, d := range own.Fields {
				switch d.Kind {
				case field.KindText, field.KindLong, field.KindChoice, field.KindCode:
					add(b.Fields[d.Key])
				case field.KindMulti:
					// Mit Leerzeichen verbunden statt mit den gespeicherten
					// Zeilenumbrüchen: ein Anriss soll sich wie ein Satz
					// lesen und nicht wie eine Spalte. Das Trennzeichen wird
					// nicht ein zweites Mal ausgeschrieben — SplitValues ist
					// die eine Stelle, die weiss, wie ein mehrwertiger Wert
					// gespeichert ist.
					add(strings.Join(field.SplitValues(b.Fields[d.Key]), " "))
				}
			}
			continue
		}
		add(b.Title)
		add(b.Markdown)
		add(b.Text)
		add(b.Source)
		add(b.Caption)
		for _, it := range b.Items {
			add(it.Title)
			add(it.Markdown)
			add(it.Caption)
		}
	}
	return strings.Join(parts, "\n\n")
}
