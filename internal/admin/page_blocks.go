package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/block"
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

// blockSet is the block kinds one website may use, dates included.
//
// The date formatter comes from the same place the theme's own formatDate does,
// so a date inside a block and a date in the page around it are spelled the
// same way.
func (h *Handler) blockSet(ctx context.Context, websiteID int64) block.Set {
	if h.blockTypes == nil {
		return block.Builtin
	}
	set := h.blockTypes.Set(ctx, websiteID)
	if ws, err := h.domains.GetWebsite(ctx, websiteID); err == nil && ws != nil {
		locale, zone := ws.Locale, ws.TimeZone
		set.Date = func(t time.Time) string { return tmpl.DateText(locale, zone, t) }
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
func blockViews(set block.Set, blocks []block.Block, items, films []media.Media, websiteID int64) []BlockView {
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
		// A kind the website defined: its fields are the ordinary field inputs,
		// drawn by the same template as the page's own fields. That is the
		// whole reason a block kind's fields live in the field system — the
		// picture chooser, the dropdown and the date input already exist.
		if own, ok := set.OwnOf(b.Type); ok {
			for _, d := range own.Fields {
				v.Fields = append(v.Fields,
					oneView(d, prefix+".f."+d.Key, b.Fields[d.Key], pool{media: items}, ""))
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
