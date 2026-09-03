package block

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
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
func Render(blocks []Block, s Set, look Lookup, md Markdown) string {
	var b strings.Builder
	for _, blk := range blocks {
		if own, ok := s.OwnOf(blk.Type); ok {
			renderOwn(&b, blk, own, s, look, md)
			continue
		}
		renderOne(&b, blk, look, md)
	}
	return b.String()
}

func renderOne(b *strings.Builder, blk Block, look Lookup, md Markdown) {
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
		var inner strings.Builder
		for _, it := range blk.Items {
			img, ok := look(it.MediaID)
			if !ok {
				continue
			}
			inner.WriteString(`<figure class="hc-galerie__bild">`)
			inner.WriteString(imgTag(img, it.Alt, "(min-width: 50em) 30vw, 90vw", true))
			if c := strings.TrimSpace(it.Caption); c != "" {
				fmt.Fprintf(&inner, `<figcaption>%s</figcaption>`, html.EscapeString(c))
			}
			inner.WriteString(`</figure>`)
		}
		if inner.Len() == 0 {
			return
		}
		fmt.Fprintf(b, `<div class="hc-block hc-galerie hc-spalten-%d">%s</div>`,
			blk.Columns(), inner.String())

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
func renderOwn(b *strings.Builder, blk Block, own Own, s Set, look Lookup, md Markdown) {
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
			for _, d := range own.Fields {
				switch d.Kind {
				case field.KindText, field.KindLong, field.KindChoice:
					add(b.Fields[d.Key])
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
