package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen on which a website gets block kinds of its own.
//
// As with the content types and the fields: a list, and below it the same form
// for creating and changing. The fields of a kind get no third screen — the
// field list is there for that already, it only has to know whose fields it is
// showing (see field.go, ?baustein=).

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
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Block kinds – %s", ws.Name)),
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
				"Block kind created. Its fields are still missing — here they are.")
			return h.redirect(w, r, fieldPathOfBlockType(websiteID, created.ID))
		}
	}

	switch {
	case errors.Is(err, block.ErrDuplicate):
		web.SetFlashError(h.sm, r.Context(),
			"A block kind with this key already exists. Choose another name.")
	case errors.Is(err, block.ErrReserved):
		web.SetFlashError(h.sm, r.Context(),
			"This key belongs to a built-in block kind. Choose another name.")
	case errors.Is(err, block.ErrTooManyTypes):
		web.SetFlashError(h.sm, r.Context(),
			"No more block kinds are created — a menu that long is one nobody reads any more.")
	case err != nil:
		// A database failure, whose text is a German fmt.Errorf wrap around a
		// driver message. The operator reads a collected sentence; the wrap
		// goes to the log, where its detail is worth something.
		slog.Error("save block kind", "err", err)
		web.SetFlashError(h.sm, r.Context(), "Saving failed.")
	default:
		web.SetFlashSuccess(h.sm, r.Context(), "Block kind changed.")
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
		"Block kind removed. Blocks of this kind disappear from the pages the next time each one is saved.")
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
