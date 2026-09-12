package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// SnippetListData is the snippet overview.
type SnippetListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Snippets  []SnippetRow
	// Values is the create/edit form, re-rendered with what was submitted when
	// a save is rejected.
	Values SnippetValues
	IsEdit bool

	// FieldViews are the own fields of this snippet, as the model of the form.
	// Built like those of the page, by the same function, for the same reason:
	// which input a field kind needs is a decision with eight branches, and
	// eight branches in a template are the place where a missing name attribute
	// hides.
	FieldViews []FieldBlock
	// Media is the image supply of an image field.
	Media []media.Media
	// RefPages ist die Auswahl eines Verweisfeldes: die Seiten dieser Website.
	RefPages []PageChoice
	// RefTerms ist die Auswahl eines Bezeichnungsfeldes: die Bezeichnungen
	// dieser Website.
	RefTerms []TermChoice
}

// pool gathers the choices this form already loaded, exactly as the page form
// does — one value rather than three parameters, so a fourth kind of chooser
// does not mean touching every call site again.
func (d SnippetListData) pool() pool {
	return pool{media: d.Media, pages: d.RefPages, terms: d.RefTerms}
}

// SnippetRow pairs a snippet with how often it is used.
type SnippetRow struct {
	snippet.Snippet
	// UsedOn is the number of live pages carrying the marker, which is what
	// makes it safe to know whether deleting one matters.
	UsedOn int
}

// SnippetValues is exactly what the form submitted.
type SnippetValues struct {
	ID       int64
	Key      string
	Name     string
	Markdown string

	// Fields are the answers to the own fields of this snippet, typed as they
	// were submitted — the same role PageValues.Fields plays on a page, so that
	// a refused form hands back what stood there.
	Fields field.Data
}

func snippetValuesFromRequest(r *http.Request) SnippetValues {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	return SnippetValues{
		ID:       id,
		Key:      strings.TrimSpace(strings.ToLower(r.FormValue("key"))),
		Name:     strings.TrimSpace(r.FormValue("name")),
		Markdown: r.FormValue("content_markdown"),
		// The parser of the page editor, called and not copied. Its doc comment
		// explains why an empty value and a missing key yield the same JSON;
		// exactly that property makes snippets.fields DEFAULT '' harmless.
		//
		// The reservation the same comment raises does not apply to this
		// handler: its storage path is a complete replacement, an UPDATE over
		// the whole column through SetFields. A caller that only ever carries
		// part of the fields forward would have to carry the presence on its
		// own level — the next one may not know that.
		Fields: fieldsFromRequest(r),
	}
}

// validKey is the same shape as a slug: a marker has to be typeable inside page
// text without ambiguity.
func validKey(key string) bool {
	if key == "" || len(key) > 60 {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && i > 0 && i < len(key)-1:
		default:
			return false
		}
	}
	return true
}

func (v SnippetValues) validate(errs web.FormErrors) {
	if v.Name == "" {
		errs.Add("name", "Please give a name.")
	}
	if !validKey(v.Key) {
		errs.Add("key", "Lower-case letters, digits, hyphen and underscore only.")
	}
}

// HandleSnippetList renders the snippets of a website together with the form.
func (h *Handler) HandleSnippetList(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	if r.Method == http.MethodPost {
		return h.handleSnippetSave(w, r, websiteID, ws.Name)
	}

	// Opening one for editing prefills the same form rather than showing a
	// second screen. It is read before snippetListData and not after, because
	// the field inputs are built from exactly these values: a form that gets the
	// values only afterwards shows empty boxes.
	values := SnippetValues{}
	isEdit := false
	if raw := r.URL.Query().Get("edit"); raw != "" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		sn, err := h.snippets.Get(r.Context(), websiteID, id)
		if err != nil {
			return err
		}
		if sn != nil {
			values = SnippetValues{
				ID: sn.ID, Key: sn.Key, Name: sn.Name, Markdown: sn.ContentMarkdown,
				Fields: field.Decode(sn.Fields),
			}
			isEdit = true
		}
	}

	data, err := h.snippetListData(r, websiteID, ws.Name, values)
	if err != nil {
		return err
	}
	data.IsEdit = isEdit
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "snippet_list", data)
}

// snippetFieldDefs loads the own fields of one snippet, or none.
//
// A snippet that does not exist yet has no fields and no number that could be
// asked about — the screen for a new one therefore carries no field, and that
// is not a restriction but the order of the thing.
//
// As with fieldDefs, an error here is not worth a failed call: without the
// definitions the fields are simply not offered, and everything else on the
// screen stays usable.
func (h *Handler) snippetFieldDefs(ctx context.Context, websiteID, snippetID int64) []field.Def {
	if h.fields == nil || snippetID == 0 {
		return nil
	}
	defs, err := h.fields.OfSnippet(ctx, websiteID, snippetID)
	if err != nil {
		return nil
	}
	return defs
}

func (h *Handler) snippetListData(r *http.Request, websiteID int64, websiteName string, values SnippetValues) (SnippetListData, error) {
	data := SnippetListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Snippets – %s", websiteName)),
		FormState:  web.NewFormState(),
		WebsiteID:  websiteID,
		Values:     values,
	}
	data.ActiveNav = "snippets"

	list, err := h.snippets.List(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	for _, sn := range list {
		used, err := h.snippets.CountUsage(r.Context(), websiteID, sn.Key)
		if err != nil {
			return data, err
		}
		data.Snippets = append(data.Snippets, SnippetRow{Snippet: sn, UsedOn: used})
	}

	// The same three supplies the page editor loads, with the same guards: an
	// image field, a reference field and a term field need a choice, and the
	// choice belongs to this website.
	if h.mediaStore != nil {
		if all, _, err := h.mediaStore.List(r.Context(), websiteID, media.Filter{MimePrefix: "image/"}, 1, 200); err == nil {
			data.Media = all
		}
	}
	data.RefPages = h.refPages(r.Context(), websiteID)
	data.RefTerms = h.siteTerms(r.Context(), websiteID)

	// The definitions go in unnarrowed. No `field.For`: that filter narrows by
	// "applies to", which means something on a page and on a snippet is set to
	// "beides" by validate anyway (plan 08-02). Filtering would therefore gain
	// nothing here and, in case of doubt, lose a field.
	defs := h.snippetFieldDefs(r.Context(), websiteID, values.ID)
	data.FieldViews = fieldViews(defs, values.Fields, data.pool(), nil)
	return data, nil
}

func (h *Handler) handleSnippetSave(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	values := snippetValuesFromRequest(r)

	// Adding a row to a group or taking one away is not a save: the button is
	// an ordinary submit, the server builds the form anew, and the whole screen
	// gets by without a line of JavaScript.
	if groupAction(r, &values.Fields) {
		data, err := h.snippetListData(r, websiteID, websiteName, values)
		if err != nil {
			return err
		}
		data.IsEdit = values.ID != 0
		if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil {
			data.CurrentWebsite = ws
		}
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}

	data, err := h.snippetListData(r, websiteID, websiteName, values)
	if err != nil {
		return err
	}
	data.IsEdit = values.ID != 0
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws

	values.validate(data.Errors)

	// It is checked against the definitions the **server** loaded, never
	// against what the form claims: a value that names no definition is
	// discarded by field.Clean and never stored. Not checkFields, because that
	// puts `field.For` with a page kind around it, and a snippet has none.
	defs := h.snippetFieldDefs(r.Context(), websiteID, values.ID)
	fieldErrs := reasonTexts(r.Context(), field.CheckAll(defs, values.Fields))
	if len(fieldErrs) > 0 {
		data.FieldViews = fieldViews(defs, values.Fields, data.pool(), fieldErrs)
		for _, reason := range fieldErrs {
			data.Errors.Add("felder", reason)
		}
	}
	// No flash: a flash survives a redirect and not the content of the form.
	// The value form of a snippet is long, and a refused save has to hand back
	// what stood there — with the reason next to the field that triggered it.
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}

	// The same selection as in CheckAll one line above: it is checked and
	// cleaned against the same definitions, or a value nobody checked would be
	// left lying unchecked.
	storedFields, err := field.Encode(field.Clean(defs, values.Fields))
	if err != nil {
		return err
	}

	// The same pipeline page content goes through, so the stored HTML carries
	// the same guarantee and can be cast in a template.
	html, err := page.RenderMarkdown(values.Markdown)
	if err != nil {
		return err
	}

	id := values.ID
	if values.ID == 0 {
		var created *snippet.Snippet
		created, err = h.snippets.Create(r.Context(), websiteID, values.Key, values.Name, values.Markdown, html)
		if created != nil {
			id = created.ID
		}
	} else {
		existing, getErr := h.snippets.Get(r.Context(), websiteID, values.ID)
		if getErr != nil {
			return getErr
		}
		if existing == nil {
			http.NotFound(w, r)
			return nil
		}
		err = h.snippets.Update(r.Context(), websiteID, values.ID, values.Key, values.Name, values.Markdown, html)
	}
	if errors.Is(err, snippet.ErrKeyTaken) {
		data.Errors.Add("key", "That key is already used by another snippet.")
		return web.RenderFormError(w, h.templates, r, "snippet_list", data)
	}
	if err != nil {
		return err
	}

	// Only now, because a new snippet does not have its number until here. The
	// column is written whole and not merged — that is the promise the parser
	// above rests on.
	if err := h.snippets.SetFields(r.Context(), websiteID, id, storedFields); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Snippet saved")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}

// HandleSnippetDelete removes a snippet.
func (h *Handler) HandleSnippetDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	id, err := strconv.ParseInt(r.PathValue("snippetID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	sn, err := h.snippets.Get(r.Context(), websiteID, id)
	if err != nil {
		return err
	}
	if sn == nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.snippets.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Snippet deleted")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/snippets", websiteID))
}
