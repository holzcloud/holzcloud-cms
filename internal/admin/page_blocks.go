package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The block editor's structural changes — add, move, delete — are ordinary
// submit buttons on the page form. The save handlers check for one before doing
// anything else and, if there is one, apply it and draw the form again rather
// than saving.
//
// That is why the whole thing works without JavaScript: with htmx the block
// list is swapped in place, without it the browser does a normal round trip and
// lands on the same form. There is no third path where the editor quietly
// depends on a script being there.

// SwitchField turns a Markdown page into a block page and back.
const SwitchField = "editorwechsel"

// blockAction applies a pending structural change, if there is one.
//
// It reports whether the request was an editor action rather than a save. The
// values are updated in place, so the caller re-renders the form it already
// built and nothing has to be threaded back out.
func blockAction(r *http.Request, values *PageValues) bool {
	switch r.FormValue(SwitchField) {
	case "zu-bausteinen":
		values.Blocks = block.FromMarkdown(values.Markdown)
		return true
	case "zu-markdown":
		md, ok := block.ToMarkdown(values.Blocks)
		if !ok {
			// Refused rather than done with a warning: the button is only
			// offered when nothing would be lost, so arriving here means the
			// form was submitted against a list that has since changed.
			return true
		}
		values.Markdown = md
		values.Blocks = nil
		return true
	}

	action := r.FormValue(block.ActionField)
	if action == "" || values.Blocks == nil {
		return false
	}
	values.Blocks = block.Apply(values.Blocks, action, values.BlockSet)
	return true
}

// renderBlockList answers an htmx request for the block list alone.
//
// The full form is rendered for a request without htmx, so the same buttons do
// the right thing either way — the browser simply reloads the whole editor.
func (h *Handler) renderBlockList(w http.ResponseWriter, r *http.Request, data PageFormData) error {
	if r.Header.Get("HX-Request") == "true" {
		return web.RenderPartial(w, h.templates, r, "block_list", data)
	}
	// 200 and not 422: adding a block is not a rejected submission. The status
	// matters — htmx does not swap a 4xx by default, so a wrong one here would
	// make the editor look broken the day somebody adds hx-post to the switch.
	return web.RenderAdmin(w, h.templates, r, "page_form", data)
}

// renderBlocks turns a page's blocks into the HTML that is stored and served.
//
// Pictures are looked up here, where the media store is, and handed to the
// renderer as plain values. That keeps internal/block free of the database and
// means a block page is rendered once on save rather than on every visit.
func (h *Handler) renderBlocks(ctx context.Context, websiteID int64, set block.Set, blocks []block.Block) string {
	return block.Render(blocks, set, h.blockImages(ctx, websiteID), page.RenderMarkdown)
}

// blockSet is the block kinds one website may use, dates and words included.
//
// The date formatter comes from the same place the theme's own formatDate does,
// so a date inside a block and a date in the page around it are spelled the
// same way. The translator is here for the same reason and from the same
// locale: the few words the renderer writes itself — a lightbox's next,
// previous and close — belong in the language of the website they appear on,
// not in the language of whoever was logged in when the page was saved.
//
// Two consequences worth stating, and both follow from renderBlocks above.
// Block HTML is rendered once, on save, so those words are frozen at save in
// the website's main language. And a website that changes its language
// re-renders its blocks on the next save of each page — not before.
func (h *Handler) blockSet(ctx context.Context, websiteID int64) block.Set {
	if h.blockTypes == nil {
		return block.Builtin
	}
	set := h.blockTypes.Set(ctx, websiteID)
	if ws, err := h.domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		locale, zone := ws.Locale, ws.TimeZone
		set.Date = func(t time.Time) string { return tmpl.DateText(locale, zone, t) }
		set.T = func(word string) string { return i18n.T(locale, word) }
	}
	return set
}

// blockImages resolves media ids for one website.
//
// The website check is not a formality: the id comes out of a form, and without
// it an editor on one site could reference a picture from another site's
// library by typing its number.
func (h *Handler) blockImages(ctx context.Context, websiteID int64) block.Lookup {
	cache := map[int64]block.Image{}
	return func(id int64) (block.Image, bool) {
		if id <= 0 || h.mediaStore == nil {
			return block.Image{}, false
		}
		if img, ok := cache[id]; ok {
			return img, true
		}
		m, err := h.mediaStore.GetByID(ctx, id)
		if err != nil || m == nil || m.WebsiteID != websiteID || !(m.IsImage() || m.IsVideo()) {
			return block.Image{}, false
		}
		img := block.Image{
			URL: m.URL(), Alt: m.AltText, Width: m.Width, Height: m.Height,
			Focus: m.FocusCSS(), Film: m.IsVideo(),
		}
		cache[id] = img
		return img, true
	}
}

// siteAlbums is the choice a gallery block's album select offers.
//
// Scoped by the store's own signature, not by this function: album.Store.List
// takes the website id and its WHERE clause carries it, so there is no way to
// call it and reach another website's albums by forgetting something here.
//
// Nil AND no error when no album store is wired, which is what a build without
// the feature looks like: there is nothing to ask, and nothing went wrong.
//
// A read failure is REPORTED and not turned into an empty list. It used to be
// logged and swallowed, and the two answers are not interchangeable: "this
// website has no albums" and "I could not find out" differ in what the editor
// is then shown and in what the form then posts. An editor told the first when
// the second is true creates a second album beside the fifty they already have
// — and, until the select learned to carry a value it cannot show, the next
// save of that page deleted every album-backed gallery on it.
func (h *Handler) siteAlbums(ctx context.Context, websiteID int64) ([]AlbumChoice, error) {
	if h.albumStore == nil {
		return nil, nil
	}
	list, err := h.albumStore.List(ctx, websiteID)
	if err != nil {
		return nil, fmt.Errorf("list albums for the block editor: %w", err)
	}
	out := make([]AlbumChoice, 0, len(list))
	for _, a := range list {
		out = append(out, AlbumChoice{Slug: a.Slug, Name: a.Name})
	}
	return out, nil
}

// blockContent is what a block page stores in the two content columns.
//
// Both are derived, neither is typed: the HTML is what visitors get, and the
// plain text is what the excerpt, the search index and the meta description are
// built from. Without the second one a page made of blocks would be invisible
// to the site's own search — the kind of gap nobody notices until a visitor
// does.
func (h *Handler) blockContent(ctx context.Context, websiteID int64, values PageValues) (markdown, html, encoded string, err error) {
	if !values.UsesBlocks() {
		html, err = page.RenderMarkdown(values.Markdown)
		return values.Markdown, html, "", err
	}
	blocks := values.BlockSet.Clean(values.Blocks)
	encoded, err = block.Encode(blocks, values.BlockSet)
	if err != nil {
		return "", "", "", err
	}
	return block.PlainText(blocks, values.BlockSet),
		h.renderBlocks(ctx, websiteID, values.BlockSet, blocks), encoded, nil
}

// --- what the template renders ----------------------------------------------

// The editor's markup needs a field name for every input, a stable id for every
// label, and the picture pool for every image select. Computing those in the
// template would mean string arithmetic and a generic "dict" helper in the
// function map — the kind of thing that works until somebody renames a field
// and finds out at render time, in a browser.
//
// So the shapes are built here, in Go, where a rename is a compile error.

// BlockView is one block as the editor draws it.
type BlockView struct {
	Number int
	Block  block.Block
	Kind   block.Kind
	// First and Last grey out the arrows at the ends of the list.
	First bool
	Last  bool
	// Prefix is the form-field prefix, "b3"; ID is the same thing where an
	// HTML id is needed, which may not contain a dot in a label's "for".
	Prefix string
	ID     string
	Image  ImageFieldView
	// Videos is the film pool, filled for a video block and empty otherwise.
	// A separate list because the picture chooser must not offer a film and
	// the film chooser must not offer a photo.
	Videos []media.Media
	// Albums is the choice a gallery block's album select offers, filled for a
	// gallery and empty for every other type. A card row has items too and must
	// not be able to name an album — block.Set.Clean drops one that somehow
	// arrives, and this is the same guard one step earlier, at the control.
	Albums []AlbumChoice
	Items  []BlockItemView
	// Fields are the inputs of a kind the website defined, empty for the
	// built-in nine.
	Fields []FieldView
	// Actions carry the index, so the template never builds one itself.
	Up, Down, Remove, AddItem string
}

// BlockItemView is one entry inside a gallery or a card row.
type BlockItemView struct {
	Number int
	Item   block.Item
	Prefix string
	ID     string
	Image  ImageFieldView
	Remove string
}

// AlbumChoice is one option of the gallery block's album select.
//
// The slug and not the id, because the slug is what block.Block.AlbumSlug
// stores and what the marker carries: an id means nothing on the machine a
// bundle lands on.
type AlbumChoice struct {
	Slug string
	Name string
	// Missing marks an option that stands for an album this website does not
	// have: the block names it, and the list does not contain it.
	//
	// It exists so that the select can carry a value it cannot offer. Without
	// it the select was drawn with no matching option, the browser submitted
	// the first one — value="", the block's own list — and the gallery was
	// deleted by the next save. The option has no name to show, because there
	// is no album to take one from, so the template shows the slug and says
	// what it is.
	Missing bool
}

// albumChoicesFor is the album select's options for one gallery block.
//
// The list as it is, plus — when the block names an album that is not in it —
// one option standing for that album, so the value has somewhere to sit.
//
// This is the whole of the fix for a silent deletion with a success flash. The
// select is the ONLY carrier of AlbumSlug in the block editor, and
// block.FromForm rebuilds a block purely from the posted fields: an absent or
// empty bN.album means AlbumSlug == "", Empty() then reports the gallery as
// empty, and Clean drops the block. Two reachable routes, neither of which
// needs anything to go wrong:
//
//   - the named album was deleted, so the select is drawn (there are others)
//     with no option matching, and the browser posts the first;
//   - the website has no albums left, so {{if .Albums}} is false and the
//     select is not drawn at all.
//
// Both are answered here rather than in the template, because "is this slug in
// this list" is a question Go can ask and html/template cannot — and because
// the second route needs the LIST to become non-empty, which only this can do.
func albumChoicesFor(albums []AlbumChoice, chosen string) []AlbumChoice {
	if chosen == "" {
		return albums
	}
	for _, a := range albums {
		if a.Slug == chosen {
			return albums
		}
	}
	out := make([]AlbumChoice, 0, len(albums)+1)
	out = append(out, albums...)
	return append(out, AlbumChoice{Slug: chosen, Missing: true})
}

// ImageFieldView is the shared picture chooser.
type ImageFieldView struct {
	Prefix    string
	ID        string
	MediaID   int64
	Alt       string
	Caption   string
	Media     []media.Media
	WebsiteID int64
}

// IsType reports whether this block is of a given type, for the template.
func (v BlockView) IsType(t string) bool { return v.Block.Type == t }

// HasItems reports whether this block's editor has a nested list.
func (v BlockView) HasItems() bool { return v.Kind.HasItems }

// blockViews builds the editor's view of a block list.
func blockViews(set block.Set, blocks []block.Block, items, films []media.Media, albums []AlbumChoice, websiteID int64) []BlockView {
	out := make([]BlockView, 0, len(blocks))
	for i, b := range blocks {
		kind, _ := set.KindOf(b.Type)
		prefix := fmt.Sprintf("b%d", i)
		v := BlockView{
			Number: i + 1,
			Block:  b,
			Kind:   kind,
			First:  i == 0,
			Last:   i == len(blocks)-1,
			Prefix: prefix,
			ID:     prefix,
			Image: ImageFieldView{
				Prefix: prefix, ID: prefix, MediaID: b.MediaID,
				Alt: b.Alt, Caption: b.Caption, Media: items, WebsiteID: websiteID,
			},
			Up:      fmt.Sprintf("%s:%d", block.ActionUp, i),
			Down:    fmt.Sprintf("%s:%d", block.ActionDown, i),
			Remove:  fmt.Sprintf("%s:%d", block.ActionDelete, i),
			AddItem: fmt.Sprintf("%s:%d", block.ActionAddItem, i),
		}
		if b.Type == block.TypeVideo {
			v.Videos = films
		}
		if b.Type == block.TypeGallery {
			v.Albums = albumChoicesFor(albums, b.AlbumSlug)
		}
		// A kind the website defined: its fields are the ordinary field inputs,
		// drawn by the same template as the page's own fields. That is the
		// whole reason a block kind's fields live in the field system — the
		// picture chooser, the dropdown and the date input already exist.
		if own, ok := set.OwnOf(b.Type); ok {
			for _, d := range own.Fields {
				// NameSuffix, nicht "[]" von Hand: die Markierung für ein
				// mehrwertiges Feld wird an einer Stelle geprägt, so wie
				// FieldName es für die Seite selbst und groupView es für eine
				// Gruppenzeile tut. Der dritte Ort, an dem Namen entstehen,
				// war der einzige, der sie nicht kannte — und ohne sie behält
				// block.FromForm den Wächter statt der Häkchen.
				v.Fields = append(v.Fields,
					oneView(d, prefix+".f."+d.Key+d.NameSuffix(), b.Fields[d.Key], pool{media: items}, ""))
			}
		}
		for j, it := range b.Items {
			ip := fmt.Sprintf("b%d.e%d", i, j)
			id := fmt.Sprintf("b%d-e%d", i, j)
			v.Items = append(v.Items, BlockItemView{
				Number: j + 1,
				Item:   it,
				Prefix: ip,
				ID:     id,
				Image: ImageFieldView{
					Prefix: ip, ID: id, MediaID: it.MediaID,
					Alt: it.Alt, Caption: it.Caption, Media: items, WebsiteID: websiteID,
				},
				Remove: fmt.Sprintf("%s:%d:%d", block.ActionDelItem, i, j),
			})
		}
		out = append(out, v)
	}
	return out
}
