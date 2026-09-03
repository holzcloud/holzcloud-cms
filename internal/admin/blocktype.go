package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Der Bildschirm, auf dem eine Website ihre eigenen Bausteinarten bekommt.
//
// Wie bei den Inhaltsarten und den Feldern: eine Liste, darunter dasselbe
// Formular zum Anlegen und Ändern. Die Felder einer Art bekommen keinen dritten
// Bildschirm — dafür gibt es die Feldliste schon, sie muss nur wissen, wessen
// Felder sie zeigt (siehe field.go, ?baustein=).

// BlockTypeRow is one kind with how many pages use it.
type BlockTypeRow struct {
	block.Own
	Used  int
	First bool
	Last  bool
}

// BlockTypeListData is the "Bausteinarten" screen.
type BlockTypeListData struct {
	web.LayoutData
	WebsiteID int64
	Rows      []BlockTypeRow
	// Edit is the kind being changed, or nil when the form is for a new one.
	Edit *block.Own
	// Builtin is the nine that every website has, so the screen shows what is
	// already there before somebody invents a tenth that exists.
	Builtin []block.Kind
}

// HandleBlockTypeList shows a website's own block kinds.
func (h *Handler) HandleBlockTypeList(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	types, err := h.blockTypes.List(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := BlockTypeListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Bausteinarten – %s", ws.Name)),
		WebsiteID:  websiteID,
		Builtin:    block.Kinds,
	}
	data.ActiveNav = "blocktypes"
	for i, t := range types {
		n, err := h.blockTypes.Used(r.Context(), websiteID, t.Key)
		if err != nil {
			return err
		}
		data.Rows = append(data.Rows, BlockTypeRow{
			Own: t, Used: n, First: i == 0, Last: i == len(types)-1,
		})
	}

	// ?aendern=<id> opens the same form filled in, with the list still visible.
	if id, cerr := strconv.ParseInt(r.URL.Query().Get("aendern"), 10, 64); cerr == nil {
		if t, gerr := h.blockTypes.Get(r.Context(), websiteID, id); gerr == nil {
			data.Edit = t
		}
	}
	return web.RenderAdmin(w, h.templates, r, "blocktype_list", data)
}

// HandleBlockTypeSave creates or changes a block kind.
func (h *Handler) HandleBlockTypeSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	back := blockTypePath(websiteID)

	name, hint := r.FormValue("name"), r.FormValue("hinweis")
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if id > 0 {
		err = h.blockTypes.Update(r.Context(), websiteID, id, name, hint)
	} else {
		var created *block.Own
		created, err = h.blockTypes.Create(r.Context(), websiteID, name, hint)
		if err == nil {
			// Straight on to its fields: a kind without any is a block that
			// renders to nothing, and the next thing anybody wants is to say
			// what goes in it.
			web.SetFlashSuccess(h.sm, r.Context(),
				"Bausteinart angelegt. Jetzt fehlen noch ihre Felder — hier sind sie.")
			return h.redirect(w, r, fieldPathOfBlockType(websiteID, created.ID))
		}
	}

	switch {
	case errors.Is(err, block.ErrDuplicate):
		web.SetFlashError(h.sm, r.Context(),
			"Eine Bausteinart mit dieser Kennung gibt es schon. Wähle einen anderen Namen.")
	case errors.Is(err, block.ErrReserved):
		web.SetFlashError(h.sm, r.Context(),
			"Diese Kennung gehört einer eingebauten Bausteinart. Wähle einen anderen Namen.")
	case errors.Is(err, block.ErrTooManyTypes):
		web.SetFlashError(h.sm, r.Context(),
			"Mehr Bausteinarten werden nicht angelegt — ein Menü, das so lang ist, liest niemand mehr.")
	case err != nil:
		web.SetFlashError(h.sm, r.Context(), err.Error())
	default:
		web.SetFlashSuccess(h.sm, r.Context(), "Bausteinart geändert.")
	}
	return h.redirect(w, r, back)
}

// HandleBlockTypeDelete removes a block kind.
func (h *Handler) HandleBlockTypeDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("typeID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := h.blockTypes.Delete(r.Context(), websiteID, id); err != nil {
		return err
	}
	// Says what did not happen, like the field screen: the blocks are still on
	// the pages, invisible, until each page is next saved.
	web.SetFlashSuccess(h.sm, r.Context(),
		"Bausteinart entfernt. Bausteine dieser Art verschwinden von den Seiten, sobald diese das nächste Mal gespeichert werden.")
	return h.redirect(w, r, blockTypePath(websiteID))
}

// HandleBlockTypeMove shifts a block kind one place in the editor's menu.
func (h *Handler) HandleBlockTypeMove(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("typeID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := h.blockTypes.Move(r.Context(), websiteID, id, r.FormValue("richtung") == "hoch"); err != nil {
		return err
	}
	return h.redirect(w, r, blockTypePath(websiteID))
}

func blockTypePath(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/bausteinarten"
}

// fieldPathOfBlockType is the field screen showing one block kind's fields.
func fieldPathOfBlockType(websiteID, typeID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) +
		"/felder?baustein=" + strconv.FormatInt(typeID, 10)
}
