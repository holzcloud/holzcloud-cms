package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/kind"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen where a website gets its own fields.
//
// One list, one form beneath it, no separate page for editing — a field has
// five properties, and sending someone to another screen for five properties
// is more navigation than the thing is worth.

// FieldListData is the field screen.
type FieldListData struct {
	web.LayoutData
	WebsiteID int64
	Fields    []field.Def
	Kinds     []field.Kind
	// Edit is the field being changed, or nil when the form is for a new one.
	Edit *field.Def
	// EditChoices is the choice list of the field being changed, as text.
	EditChoices string
	// Group is the group whose fields are being edited, or nil for the top
	// level. When it is set, the screen shows that group's fields and the form
	// adds to it.
	Group *field.Def
	// Types are the website's own content kinds, so a field can belong to one
	// of them rather than to every page.
	Types []kind.Type
	// Controls are the fields a condition may hang on. Not every field can be
	// one — see field.Def.MayControl — and the list is the whole explanation:
	// what is not offered cannot be chosen wrong.
	Controls []field.Def
	// BlockType is the block kind whose fields are being edited, or nil for the
	// page's own fields. The third mode of this screen, after the top level and
	// a group.
	BlockType *block.Own
}

// TypeName is the plural of one own content kind, for the "gilt für" column.
//
// Falls back to the key: a field can be tied to a kind that was removed later,
// and "nur produkt" is still more use than an empty cell — it says which kind
// to define again to see the field.
func (d FieldListData) TypeName(key string) string {
	for _, t := range d.Types {
		if t.Key == key {
			return t.Plural
		}
	}
	return key
}

// Simple reports whether this is the website's own top-level field list — the
// one place where a field can say which pages it belongs to and hang on
// another. Inside a group and inside a block kind neither question applies.
func (d FieldListData) Simple() bool { return d.Group == nil && d.BlockType == nil }

// HandleFieldList shows a website's fields.
func (h *Handler) HandleFieldList(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	// ?gruppe=<id> narrows the screen to one group's own fields. A query
	// parameter and not a second screen: it is the same list and the same form,
	// one level down.
	var group *field.Def
	if id, gerr := strconv.ParseInt(r.URL.Query().Get("gruppe"), 10, 64); gerr == nil {
		if def, err := h.fields.Get(r.Context(), websiteID, id); err == nil && def.IsGroup() {
			group = def
		}
	}
	// ?baustein=<id> narrows it to one block kind's fields, the same way.
	var blockType *block.Own
	if id, berr := strconv.ParseInt(r.URL.Query().Get("baustein"), 10, 64); berr == nil {
		if t, gerr := h.blockTypes.Get(r.Context(), websiteID, id); gerr == nil {
			blockType = t
			group = nil
		}
	}

	data, err := h.fieldListData(r, websiteID, ws.Name, group, blockType)
	if err != nil {
		return err
	}

	// ?aendern=<id> opens the same form filled in. The list stays visible, so
	// one can see what the other fields are called while renaming one.
	if id, cerr := strconv.ParseInt(r.URL.Query().Get("aendern"), 10, 64); cerr == nil {
		if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
			data.Edit = def
			data.EditChoices = field.JoinChoices(def.Choices)
		}
	}
	return web.RenderAdmin(w, h.templates, r, "field_list", data)
}

// HandleFieldSave creates or changes a field.
func (h *Handler) HandleFieldSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	parentID, _ := strconv.ParseInt(r.FormValue("gruppe"), 10, 64)
	blockTypeID, _ := strconv.ParseInt(r.FormValue("baustein"), 10, 64)
	def := field.Def{
		WebsiteID:   websiteID,
		ParentID:    parentID,
		BlockTypeID: blockTypeID,
		Label:       r.FormValue("beschriftung"),
		Kind:        r.FormValue("art"),
		Required:    r.FormValue("pflicht") == "1",
		Hint:        r.FormValue("hinweis"),
		Choices:     field.SplitChoices(r.FormValue("auswahl")),
		AppliesTo:   r.FormValue("gilt_fuer"),
		Condition:   r.FormValue("bedingung"),
	}

	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if id > 0 {
		err = h.fields.Update(r.Context(), websiteID, id, def)
	} else {
		_, err = h.fields.Create(r.Context(), def)
	}
	switch {
	case errors.Is(err, field.ErrNested):
		web.SetFlashError(h.sm, r.Context(),
			"Eine Gruppe in einer Gruppe gibt es nicht — eine Ebene, damit sich im Formular noch jemand zurechtfindet.")
	case errors.Is(err, field.ErrKindFixed):
		web.SetFlashError(h.sm, r.Context(),
			"Aus einer Gruppe wird kein einfaches Feld und umgekehrt: die Zeilen hätten nirgends hin.")
	case errors.Is(err, field.ErrDuplicateKey):
		web.SetFlashError(h.sm, r.Context(),
			"Ein Feld mit dieser Kennung gibt es schon. Wähle eine andere Beschriftung.")
	case errors.Is(err, field.ErrNotInBlock):
		web.SetFlashError(h.sm, r.Context(),
			"Diese Art von Feld gibt es in einem Baustein nicht. Ein Verweis folgt einer Umbenennung, und der Baustein wird beim Speichern der Seite ein für alle Mal in HTML verwandelt — er könnte dieses Versprechen nicht halten. Nimm einen Link.")
	case errors.Is(err, field.ErrNoCondition):
		web.SetFlashError(h.sm, r.Context(),
			"An dieses Feld lässt sich keine Bedingung hängen. Möglich sind die Felder, bei denen der Browser sehen kann, ob sie ausgefüllt sind — ein Datum und eine Gruppe gehören nicht dazu.")
	case errors.Is(err, field.ErrConditionLoop):
		web.SetFlashError(h.sm, r.Context(),
			"Die Bedingungen drehen sich im Kreis: keines der beteiligten Felder wäre je zu sehen.")
	case errors.Is(err, field.ErrTooMany):
		web.SetFlashError(h.sm, r.Context(),
			"Mehr Felder werden nicht angelegt — ein Formular, das so lang ist, füllt niemand richtig aus.")
	case err != nil:
		web.SetFlashError(h.sm, r.Context(), err.Error())
	case id > 0:
		web.SetFlashSuccess(h.sm, r.Context(), "Feld geändert.")
	default:
		web.SetFlashSuccess(h.sm, r.Context(),
			"Feld angelegt. Es steht ab sofort im Editor und im Theme.")
	}
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID))
}

// HandleFieldDelete removes a field definition.
func (h *Handler) HandleFieldDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("fieldID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	parentID, blockTypeID := int64(0), int64(0)
	if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
		parentID, blockTypeID = def.ParentID, def.BlockTypeID
	}
	if err := h.fields.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}
	// Deliberately says what did not happen: the values are still on the pages,
	// and someone who deleted the wrong field should know they can get it back.
	web.SetFlashSuccess(h.sm, r.Context(),
		"Feld entfernt. Das Ausgefüllte bleibt an den Seiten stehen, bis sie das nächste Mal gespeichert werden — "+
			"wer sich vertan hat, legt das Feld einfach wieder an.")
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID))
}

// HandleFieldMove shifts a field one place.
func (h *Handler) HandleFieldMove(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("fieldID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	parentID, blockTypeID := int64(0), int64(0)
	if def, gerr := h.fields.Get(r.Context(), websiteID, id); gerr == nil {
		parentID, blockTypeID = def.ParentID, def.BlockTypeID
	}
	if err := h.fields.Move(r.Context(), websiteID, id, r.URL.Query().Get("richtung") == "hoch"); err != nil {
		return err
	}
	return h.redirect(w, r, fieldPath(websiteID, parentID, blockTypeID))
}

func (h *Handler) fieldListData(r *http.Request, websiteID int64, websiteName string,
	group *field.Def, blockType *block.Own) (FieldListData, error) {

	var (
		defs  []field.Def
		err   error
		title = "Felder – " + websiteName
		kinds = field.Kinds
	)
	switch {
	case blockType != nil:
		defs, err = h.fields.OfBlockType(r.Context(), websiteID, blockType.ID)
		title = "Baustein „" + blockType.Name + "“ – " + websiteName
		kinds = field.BlockKinds()
	case group != nil:
		defs, err = h.fields.Sub(r.Context(), websiteID, group.ID)
		title = "Gruppe „" + group.Label + "“ – " + websiteName
		kinds = field.SubKinds()
	default:
		defs, err = h.fields.List(r.Context(), websiteID)
	}
	if err != nil {
		return FieldListData{}, err
	}

	data := FieldListData{
		LayoutData: web.NewLayoutData(r, h.sm, title),
		WebsiteID:  websiteID,
		Fields:     defs,
		Kinds:      kinds,
		Group:      group,
		BlockType:  blockType,
		Types:      h.kindsOf(r, websiteID),
	}
	if group == nil && blockType == nil {
		for _, d := range defs {
			if d.MayControl() {
				data.Controls = append(data.Controls, d)
			}
		}
	}
	data.ActiveNav = "fields"
	return data, nil
}

// fieldPath is the screen a change returns to: the group's own list, the block
// kind's own list, or the website's.
func fieldPath(websiteID, parentID, blockTypeID int64) string {
	path := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/felder"
	switch {
	case blockTypeID > 0:
		path += "?baustein=" + strconv.FormatInt(blockTypeID, 10)
	case parentID > 0:
		path += "?gruppe=" + strconv.FormatInt(parentID, 10)
	}
	return path
}
