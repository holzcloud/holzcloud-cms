package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The website's own fields, in the page editor.
//
// They sit under the content, after the text and before the search-engine
// panel: a price belongs with the thing it describes and not among the meta
// tags. Which of them are shown depends on the kind of page, so an editor
// writing a post is not asked for a price.

// GroupAction is the hidden field the row buttons write.
//
// Same idea as the block editor's: a button that adds a row is an ordinary
// submit, the server rebuilds the form, and the whole thing works with
// JavaScript switched off.
const GroupAction = "gruppenaktion"

// FieldView is one extra field as the editor's template renders it.
//
// Built in Go rather than assembled in the template: which input a kind needs
// is a decision with eight branches, and eight branches in a template is where
// a missing `name` attribute hides until somebody notices their entry never
// saved.
type FieldView struct {
	Def field.Def
	// Name is the form field name.
	Name string
	// Value is what is currently entered, as typed.
	Value string
	// Checked is the state of a yes/no field.
	Checked bool
	// Media is the picture pool for a picture field, empty for every other kind.
	Media []media.Media
	// MediaID is the chosen picture, or 0.
	MediaID int64
	// Pages is the choice of pages for a reference field, empty for every
	// other kind.
	Pages []PageChoice
	// PageID is the referenced page, or 0.
	PageID int64
	// RefDraft says the referenced page is not published, so the reference
	// exists here and is invisible on the website. Better said in the editor
	// than discovered on the finished page.
	RefDraft bool
	// Error is the reason this value was rejected, or empty.
	Error string
	// Rows are a group's rows. Nil for every other kind.
	Rows []FieldRow

	// Dependent are the fields that hang on this one. They are rendered inside
	// it, because that is the only way the browser can show and hide them
	// without a script: a stylesheet can reach from a ticked box to a block
	// beside it, not to one somewhere else on the page.
	Dependent []FieldView
	// Switch says which rule hides the dependent fields — "kreuz" for a tick
	// box, "auswahl" for a dropdown, "text" for anything typed. Empty when
	// nothing hangs on this field. See .feld-schalter in admin.css.
	Switch string
}

// FieldBlock is one part of the field form: a heading and the fields under it.
//
// The first block has no heading — the fields somebody defined before they
// started dividing them up have to go somewhere, and putting them under an
// invented title would be putting words in their mouth.
type FieldBlock struct {
	Title  string
	Hint   string
	Fields []FieldView
}

// FieldRow is one filled-in row of a group.
type FieldRow struct {
	// Index is the row's position in the form, which is what the buttons act on.
	Index int
	// Number is Index + 1, for the heading. A template cannot add.
	Number int
	Fields []FieldView
	// First and Last switch the move buttons off at the ends.
	First bool
	Last  bool
}

// Is reports whether this field is of a given kind, for the template.
func (v FieldView) Is(kind string) bool { return v.Def.Kind == kind }

// PageChoice is one page a reference field may point at.
type PageChoice struct {
	ID    int64
	Title string
	Slug  string
	// Draft marks a page a visitor cannot see, so the list can say so before
	// somebody picks it rather than afterwards.
	Draft bool
}

// pool is what the choosers of the extra fields need: the pictures and the
// pages of this website.
//
// One value rather than two parameters because every call site passes both, and
// a third kind of chooser should not mean touching all of them again.
type pool struct {
	media []media.Media
	pages []PageChoice
}

// pool gathers the choices this form already loaded.
func (d PageFormData) pool() pool { return pool{media: d.Media, pages: d.RefPages} }

// fieldViews builds the editor model for one page: the fields divided into
// blocks by their headings, each dependent field inside the one it hangs on.
func fieldViews(defs []field.Def, data field.Data, p pool, errs map[string]string) []FieldBlock {
	under := field.ControlsOf(defs)
	hangs := map[string]bool{}
	for _, list := range under {
		for _, d := range list {
			hangs[d.Key] = true
		}
	}

	blocks := []FieldBlock{{}}
	for _, d := range defs {
		if d.IsSection() {
			blocks = append(blocks, FieldBlock{Title: d.Label, Hint: d.Hint})
			continue
		}
		// Rendered inside its controller instead, wherever that one stands.
		if hangs[d.Key] {
			continue
		}
		at := len(blocks) - 1
		blocks[at].Fields = append(blocks[at].Fields, viewOf(d, under, map[string]bool{}, data, p, errs))
	}
	// A heading with nothing under it, or the leading block on a website that
	// starts with one, is not worth a line on the screen.
	out := make([]FieldBlock, 0, len(blocks))
	for _, b := range blocks {
		if len(b.Fields) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// viewOf builds one field with everything that hangs on it.
//
// seen stops a circle: the screen refuses to save one, but a definition written
// by an older version could hold one, and an editor that recurses for ever is
// a worse answer than a field shown once.
func viewOf(d field.Def, under map[string][]field.Def, seen map[string]bool,
	data field.Data, p pool, errs map[string]string) FieldView {

	var v FieldView
	if d.IsGroup() {
		v = groupView(d, data.Rows[d.Key], p, errs)
	} else {
		v = oneView(d, d.FieldName(), data.Values[d.Key], p, errs[d.Key])
	}
	if seen[d.Key] {
		return v
	}
	seen[d.Key] = true
	for _, sub := range under[d.Key] {
		v.Dependent = append(v.Dependent, viewOf(sub, under, seen, data, p, errs))
	}
	if len(v.Dependent) > 0 {
		v.Switch = switchOf(d.Kind)
	}
	return v
}

// switchOf names the rule that shows this field's dependants, by what the
// browser can see: a tick box is checked or not, a dropdown has the empty
// option chosen or not, and everything typed still shows its placeholder or
// does not.
func switchOf(kind string) string {
	switch kind {
	case field.KindBool:
		return "kreuz"
	case field.KindChoice, field.KindImage, field.KindRef:
		return "auswahl"
	default:
		return "text"
	}
}

// groupView builds one group with its rows.
func groupView(d field.Def, rows []field.Values, p pool, errs map[string]string) FieldView {
	v := FieldView{Def: d, Name: d.Key, Error: errs[d.Key]}
	for i, row := range rows {
		r := FieldRow{Index: i, Number: i + 1, First: i == 0, Last: i == len(rows)-1}
		for _, sub := range d.Sub {
			key := field.RowKey(d.Key, i, sub.Key)
			r.Fields = append(r.Fields, oneView(sub, "gruppe."+key, row[sub.Key], p, errs[key]))
		}
		v.Rows = append(v.Rows, r)
	}
	return v
}

func oneView(d field.Def, name, value string, p pool, reason string) FieldView {
	v := FieldView{Def: d, Name: name, Value: value, Error: reason}
	switch d.Kind {
	case field.KindBool:
		v.Checked = value != "" && value != "0"
	case field.KindImage:
		v.Media = p.media
		v.MediaID, _ = strconv.ParseInt(value, 10, 64)
	case field.KindRef:
		v.Pages = p.pages
		v.PageID, _ = strconv.ParseInt(value, 10, 64)
		for _, c := range p.pages {
			if c.ID == v.PageID {
				v.RefDraft = c.Draft
			}
		}
	}
	return v
}

// refPages is the choice a reference field offers: this website's pages, in
// alphabetical order, drafts marked as such.
//
// Only this website's — that is the whole reason the chooser exists rather
// than a number somebody types in. Bounded at 500, which is more pages than a
// dropdown is usable with anyway; beyond that the field still holds whatever
// was chosen earlier.
func (h *Handler) refPages(ctx context.Context, websiteID int64) []PageChoice {
	if h.pages == nil {
		return nil
	}
	list, _, err := h.pages.ListPages(ctx, websiteID, page.ListFilter{
		Locale: "*", Sort: "title", Page: 1, PerPage: 500,
	})
	if err != nil {
		return nil
	}
	out := make([]PageChoice, 0, len(list))
	for _, p := range list {
		out = append(out, PageChoice{
			ID: p.ID, Title: p.Title, Slug: p.Slug, Draft: !p.PubliclyVisible(),
		})
	}
	return out
}

// groupAction applies a row button and reports whether one was pressed.
//
// It changes the values in place and returns true, and the caller redraws the
// form instead of saving. Adding a row is not a submission of the page.
func groupAction(r *http.Request, data *field.Data) bool {
	action := r.FormValue(GroupAction)
	if action == "" {
		return false
	}
	parts := strings.Split(action, ":")
	if len(parts) < 2 {
		return false
	}
	verb, group := parts[0], parts[1]
	if data.Rows == nil {
		data.Rows = map[string][]field.Values{}
	}
	rows := data.Rows[group]

	index := -1
	if len(parts) > 2 {
		if i, err := strconv.Atoi(parts[2]); err == nil {
			index = i
		}
	}

	switch verb {
	case "neu":
		if len(rows) < field.MaxRows {
			rows = append(rows, field.Values{})
		}
	case "weg":
		if index >= 0 && index < len(rows) {
			rows = append(rows[:index], rows[index+1:]...)
		}
	case "hoch":
		if index > 0 && index < len(rows) {
			rows[index-1], rows[index] = rows[index], rows[index-1]
		}
	case "runter":
		if index >= 0 && index+1 < len(rows) {
			rows[index], rows[index+1] = rows[index+1], rows[index]
		}
	default:
		return false
	}

	data.Rows[group] = rows
	return true
}

// fieldDefs loads a website's field definitions, or none.
//
// A failure here is not worth a failed save: without the definitions the extra
// fields are simply not offered, and everything else on the page still works.
func (h *Handler) fieldDefs(ctx context.Context, websiteID int64) []field.Def {
	if h.fields == nil {
		return nil
	}
	defs, err := h.fields.List(ctx, websiteID)
	if err != nil {
		return nil
	}
	return defs
}

// checkFields validates the submitted values against the definitions that
// apply to this kind of page, and returns the reasons by field key.
func checkFields(defs []field.Def, pageKind string, data field.Data) map[string]string {
	return field.CheckAll(field.For(defs, pageKind), data)
}

// fieldImages resolves media ids for a website, for rendering.
//
// The website check is the point: the id comes out of a form, and without it a
// field on one site could name a picture from another site's library.
func (h *Handler) fieldImages(ctx context.Context, websiteID int64) field.Lookup {
	cache := map[int64]field.Image{}
	return func(id int64) (field.Image, bool) {
		if id <= 0 || h.mediaStore == nil {
			return field.Image{}, false
		}
		if img, ok := cache[id]; ok {
			return img, img.URL != ""
		}
		m, err := h.mediaStore.GetByID(ctx, id)
		if err != nil || m == nil || m.WebsiteID != websiteID {
			cache[id] = field.Image{}
			return field.Image{}, false
		}
		img := field.Image{
			URL:    m.URL(),
			Alt:    m.AltText,
			Width:  m.Width,
			Height: m.Height,
			Focus:  m.FocusCSS(),
		}
		cache[id] = img
		return img, true
	}
}
