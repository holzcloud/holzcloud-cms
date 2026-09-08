package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The album screens: a list with a create form, and one screen per album with
// its ordered pictures.
//
// Modelled on internal/admin/menu.go, which is the only other admin area with a
// named parent and an ordered child list. Three things of that file are
// deliberately NOT copied, and each absence is a decision:
//
//   - isValidLocationKey and the branch around it (menu.go:113-118). An album
//     has no location key. Its address is derived from its name by the store's
//     single call to page.Slugify, and nothing here mints a second one.
//
//   - menuOfWebsite's role as the ONLY guard (menu.go:54-77). The album store
//     puts website_id in the WHERE clause of every statement, so Get already
//     answers nothing for another website's album and albumFromPath is a second
//     strap rather than the strap. internal/menu/store.go:14-33 records why that
//     shape was chosen: on 2026-09-06 four menu item handlers reached another
//     website's navigation, and the reason was not carelessness in four places —
//     it was that nothing in a signature made anyone supply the website. Here
//     every store call takes websiteID, so a handler that forgot could not
//     compile.
//
//   - the substring match on the text of an SQL message (menu.go:167-176).
//     internal/album exports named errors for exactly this, and the branches
//     below use errors.Is.
//
// The picture chooser is ImageFieldView (page_blocks.go:193), the block
// editor's own, fed to the block_bild partial unchanged. That reuse is GAL-07
// in the editor: one picture chooser, not two that agree.

// AlbumListData extends LayoutData for the album list page.
type AlbumListData struct {
	web.LayoutData
	WebsiteID int64
	Albums    []AlbumRow
}

// AlbumRow is one album in the list, with how many pictures are in it.
type AlbumRow struct {
	album.Album
	Pictures int
}

// AlbumEditData extends LayoutData for one album's screen.
type AlbumEditData struct {
	web.LayoutData
	WebsiteID int64
	Album     *album.Album
	Pictures  []AlbumItemView
	// NewPicture is the chooser of the add form at the foot of the screen. It
	// is the same ImageFieldView every row carries, with an empty selection.
	NewPicture ImageFieldView
}

// AlbumItemView is one picture row as the editor draws it.
//
// Prefix is "bild" on every row, so the posted names are bild.medium, bild.alt
// and bild.bildunterschrift and block_bild works unchanged; ID is per row,
// because two labels on one page may not point at the same element. That split
// is what ImageFieldView carries the two fields for.
type AlbumItemView struct {
	Number int
	ID     int64
	// First and Last grey out the arrows at the ends of the list.
	First bool
	Last  bool
	Image ImageFieldView
	// The action addresses, built here so no template does string arithmetic.
	Up, Down, Remove, Update string
}

// albumScope resolves the leading /admin/websites/{id} of an album address.
//
// A missing store is a 404 and not a panic: a build that never called
// SetAlbumStore has no album screens, the same way a build without a plugin
// manager has no plugin screens.
func (h *Handler) albumScope(w http.ResponseWriter, r *http.Request) (*domain.Website, error) {
	if h.albumStore == nil {
		http.NotFound(w, r)
		return nil, nil
	}
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil, nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil, nil
	}
	return ws, nil
}

// albumFromPath resolves both ids of an album address at once.
//
// The two arrive from two different parts of one path and nothing before the
// handler has an opinion about the second: auth.RequireWebsiteAccess reads
// /admin/websites/<number> and stops there, deliberately, so it admits anyone
// who may enter the website in front whatever album the rest of the path then
// names. The store's Get is scoped by website_id and would answer nothing
// anyway — this is the second strap, and two is right when the cost is a nil
// check.
func (h *Handler) albumFromPath(w http.ResponseWriter, r *http.Request) (*domain.Website, *album.Album, error) {
	ws, err := h.albumScope(w, r)
	if err != nil || ws == nil {
		return nil, nil, err
	}
	albumID, err := strconv.ParseInt(r.PathValue("albumID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil, nil, nil
	}
	a, err := h.albumStore.Get(r.Context(), ws.ID, albumID)
	if err != nil {
		return nil, nil, err
	}
	if a == nil {
		http.NotFound(w, r)
		return nil, nil, nil
	}
	return ws, a, nil
}

// albumSaid turns one of the store's named errors into a sentence an operator
// can act on, or returns "" for an error that is not one of them.
//
// errors.Is and not strings.Contains(err.Error(), "UNIQUE constraint"), which is
// what internal/admin/menu.go:172 still has to do: a string match reads a
// sentence the driver is free to reword and cannot tell a unique violation from
// a NOT NULL one that happens to mention the word.
//
// # Why every sentence is wrapped in i18n.N
//
// tools/i18n reads the ARGUMENTS of SetFlashError and SetFlashSuccess. These
// sentences are returned from a helper and the call sites pass the helper's
// result, so the collector saw a function call where it looks for a literal and
// collected nothing: the gate would have read "0 offen, 0 verwaist" while all
// six of these still printed in German to a French or an Italian operator, with
// nothing anywhere saying so.
//
// i18n.N marks a literal where it is WRITTEN and returns it unchanged; the
// SetFlash* family still translates it where it is put away. internal/block/
// render.go uses it for exactly this, and its three keys are in the catalogue
// because of it. internal/admin/menu.go needs none because it passes its
// literals directly — this file is a regression against its own sibling, not a
// shape the codebase had.
func albumSaid(err error) string {
	switch {
	case errors.Is(err, album.ErrNoName):
		return i18n.N("Bitte einen Namen für das Album angeben")
	case errors.Is(err, album.ErrDuplicateName):
		return i18n.N("Ein Album mit diesem Namen gibt es schon")
	case errors.Is(err, album.ErrTooManyItems):
		return i18n.N("Dieses Album ist voll. Leg für weitere Bilder ein zweites an.")
	case errors.Is(err, album.ErrForeignMedia):
		return i18n.N("Dieses Bild gehört nicht zur Mediathek dieser Website")
	case errors.Is(err, album.ErrNotFound):
		return i18n.N("Das Album oder das Bild gibt es nicht mehr")
	}
	return ""
}

// albumImages is the picture pool both choosers offer: this website's images,
// and nothing else.
func (h *Handler) albumImages(r *http.Request, websiteID int64) []media.Media {
	if h.mediaStore == nil {
		return nil
	}
	items, _, err := h.mediaStore.List(r.Context(), websiteID,
		media.Filter{MimePrefix: "image/"}, 1, 200)
	if err != nil {
		return nil
	}
	return items
}

// HandleAlbumList lists a website's albums.
func (h *Handler) HandleAlbumList(w http.ResponseWriter, r *http.Request) error {
	ws, err := h.albumScope(w, r)
	if err != nil || ws == nil {
		return err
	}

	albums, err := h.albumStore.List(r.Context(), ws.ID)
	if err != nil {
		return err
	}

	// One count per album. A separate COUNT query per row is the honest price
	// of not adding a second statement to the store for a number the list shows
	// once: albums are hand-made and few, and a picture list is capped at
	// album.MaxItems. If a website ever has hundreds of albums this is the
	// first place to look.
	rows := make([]AlbumRow, 0, len(albums))
	for _, a := range albums {
		pictures, err := h.albumStore.Pictures(r.Context(), ws.ID, a.ID)
		if err != nil {
			return err
		}
		rows = append(rows, AlbumRow{Album: a, Pictures: len(pictures)})
	}

	data := AlbumListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Alben – %s", ws.Name)),
		WebsiteID:  ws.ID,
		Albums:     rows,
	}
	data.ActiveNav = "albums"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "album_list", data)
}

// HandleAlbumCreate adds an album to a website.
func (h *Handler) HandleAlbumCreate(w http.ResponseWriter, r *http.Request) error {
	ws, err := h.albumScope(w, r)
	if err != nil || ws == nil {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums", ws.ID)

	// The name is not trimmed or bounded here. normalizeName in the store does
	// both, and a second normalisation in the handler is a second spelling of
	// the same rule that would eventually disagree with the first.
	_, err = h.albumStore.Create(r.Context(), ws.ID, r.FormValue("name"))
	switch {
	case err == nil:
		web.SetFlashSuccess(h.sm, r.Context(), "Album angelegt")
	case albumSaid(err) != "":
		web.SetFlashError(h.sm, r.Context(), albumSaid(err))
	default:
		return err
	}

	// menu.go:126-135 verbatim, and it is CLAUDE.md's htmx rule made concrete:
	// htmx acts on the header, a browser with scripting switched off follows
	// the redirect, and the same button works either way.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleAlbumEdit shows one album with its pictures in order.
func (h *Handler) HandleAlbumEdit(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}

	pictures, err := h.albumStore.Pictures(r.Context(), ws.ID, a.ID)
	if err != nil {
		return err
	}
	images := h.albumImages(r, ws.ID)

	views := make([]AlbumItemView, 0, len(pictures))
	for i, p := range pictures {
		rowID := fmt.Sprintf("bild%d", p.ID)
		base := fmt.Sprintf("/admin/websites/%d/albums/%d/pictures/%d", ws.ID, a.ID, p.ID)
		views = append(views, AlbumItemView{
			Number: i + 1,
			ID:     p.ID,
			First:  i == 0,
			Last:   i == len(pictures)-1,
			Image: ImageFieldView{
				Prefix: "bild", ID: rowID, MediaID: p.Item.MediaID,
				Alt: p.Item.Alt, Caption: p.Item.Caption,
				Media: images, WebsiteID: ws.ID,
			},
			Up:     base + "/reorder?direction=up",
			Down:   base + "/reorder?direction=down",
			Remove: base + "/delete",
			Update: base + "/update",
		})
	}

	data := AlbumEditData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Album %s – %s", a.Name, ws.Name)),
		WebsiteID:  ws.ID,
		Album:      a,
		Pictures:   views,
		NewPicture: ImageFieldView{
			Prefix: "bild", ID: "neues-bild", Media: images, WebsiteID: ws.ID,
		},
	}
	data.ActiveNav = "albums"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "album_edit", data)
}

// HandleAlbumUpdate corrects an album's name. Its address does not move: a
// gallery block stores the slug and a bundle re-derives it, so a corrected typo
// must not take the album away from every page that carries it.
func (h *Handler) HandleAlbumUpdate(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums/%d", ws.ID, a.ID)

	err = h.albumStore.Rename(r.Context(), ws.ID, a.ID, r.FormValue("name"))
	switch {
	case err == nil:
		web.SetFlashSuccess(h.sm, r.Context(), "Album gespeichert")
	case albumSaid(err) != "":
		web.SetFlashError(h.sm, r.Context(), albumSaid(err))
	default:
		return err
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleAlbumDelete removes an album. Its picture rows go with it through the
// cascade; the files in the media library are untouched.
func (h *Handler) HandleAlbumDelete(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums", ws.ID)

	err = h.albumStore.Delete(r.Context(), ws.ID, a.ID)
	switch {
	case err == nil:
		web.SetFlashSuccess(h.sm, r.Context(), "Album gelöscht")
	case albumSaid(err) != "":
		web.SetFlashError(h.sm, r.Context(), albumSaid(err))
	default:
		return err
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// albumPicture reads the three fields block_bild posts.
func albumPicture(r *http.Request) (mediaID int64, alt, caption string) {
	mediaID, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("bild.medium")), 10, 64)
	return mediaID,
		strings.TrimSpace(r.FormValue("bild.alt")),
		strings.TrimSpace(r.FormValue("bild.bildunterschrift"))
}

// requireOwnPicture refuses a media id that is missing, not a picture, or not
// this website's.
//
// The store checks the ownership too, and both checks are right here for the
// reason internal/admin/page_blocks.go:107-110 already states: the id comes out
// of a form, and without the check an editor on one site references another
// site's library by typing its number. The handler's copy is what turns it into
// a sentence rather than a 500, and it is also what catches a film chosen where
// a photo belongs — which the store, correctly, has no opinion about.
// The two sentences below are marked with i18n.N for the reason albumSaid
// gives at length: they are returned from here and passed to SetFlashError at
// the call site, where the collector looks for a literal and finds a variable.
func (h *Handler) requireOwnPicture(r *http.Request, websiteID, mediaID int64) string {
	if mediaID <= 0 {
		return i18n.N("Bitte ein Bild auswählen")
	}
	if h.mediaStore == nil {
		return i18n.N("Dieses Bild gehört nicht zur Mediathek dieser Website")
	}
	m, err := h.mediaStore.GetByID(r.Context(), mediaID)
	if err != nil || m == nil || m.WebsiteID != websiteID || !m.IsImage() {
		return i18n.N("Dieses Bild gehört nicht zur Mediathek dieser Website")
	}
	return ""
}

// HandleAlbumItemCreate appends one picture to an album.
func (h *Handler) HandleAlbumItemCreate(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums/%d", ws.ID, a.ID)

	mediaID, alt, caption := albumPicture(r)
	if refused := h.requireOwnPicture(r, ws.ID, mediaID); refused != "" {
		web.SetFlashError(h.sm, r.Context(), refused)
	} else {
		_, err = h.albumStore.AddItem(r.Context(), ws.ID, a.ID, mediaID, alt, caption)
		switch {
		case err == nil:
			web.SetFlashSuccess(h.sm, r.Context(), "Bild hinzugefügt")
		case albumSaid(err) != "":
			web.SetFlashError(h.sm, r.Context(), albumSaid(err))
		default:
			return err
		}
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleAlbumItemUpdate repoints one picture row and rewrites its description
// and caption.
func (h *Handler) HandleAlbumItemUpdate(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums/%d", ws.ID, a.ID)

	mediaID, alt, caption := albumPicture(r)
	if refused := h.requireOwnPicture(r, ws.ID, mediaID); refused != "" {
		web.SetFlashError(h.sm, r.Context(), refused)
	} else {
		err = h.albumStore.UpdateItem(r.Context(), ws.ID, a.ID, itemID, mediaID, alt, caption)
		switch {
		case err == nil:
			web.SetFlashSuccess(h.sm, r.Context(), "Bild gespeichert")
		case albumSaid(err) != "":
			web.SetFlashError(h.sm, r.Context(), albumSaid(err))
		default:
			return err
		}
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleAlbumItemDelete removes one picture from an album. The file stays in
// the library.
func (h *Handler) HandleAlbumItemDelete(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums/%d", ws.ID, a.ID)

	err = h.albumStore.DeleteItem(r.Context(), ws.ID, a.ID, itemID)
	switch {
	case err == nil:
		web.SetFlashSuccess(h.sm, r.Context(), "Bild entfernt")
	case albumSaid(err) != "":
		web.SetFlashError(h.sm, r.Context(), albumSaid(err))
	default:
		return err
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// HandleAlbumItemReorder moves one picture up or down by swapping it with its
// neighbour.
//
// A move that no longer makes sense — the first picture upwards, the last
// downwards — is NOT an error. The button was drawn against a list that may
// since have changed, and block.Apply's rule for an action that no longer
// applies is the honest one: show the list as it now is rather than report a
// failure the operator cannot act on.
func (h *Handler) HandleAlbumItemReorder(w http.ResponseWriter, r *http.Request) error {
	ws, a, err := h.albumFromPath(w, r)
	if err != nil || a == nil {
		return err
	}
	itemID, err := strconv.ParseInt(r.PathValue("itemID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	redirect := fmt.Sprintf("/admin/websites/%d/albums/%d", ws.ID, a.ID)

	direction := r.URL.Query().Get("direction")
	pictures, err := h.albumStore.Pictures(r.Context(), ws.ID, a.ID)
	if err != nil {
		return err
	}

	// The neighbour in the order as it stands, or 0 when there is none.
	var neighbour int64
	for i, p := range pictures {
		if p.ID != itemID {
			continue
		}
		if direction == "up" && i > 0 {
			neighbour = pictures[i-1].ID
		}
		if direction == "down" && i < len(pictures)-1 {
			neighbour = pictures[i+1].ID
		}
		break
	}

	switch {
	case direction != "up" && direction != "down":
		web.SetFlashError(h.sm, r.Context(), "Ungültige Richtung")
	case neighbour == 0:
		web.SetFlashSuccess(h.sm, r.Context(), "Die Reihenfolge steht schon so")
	default:
		err = h.albumStore.SwapSortOrder(r.Context(), ws.ID, a.ID, itemID, neighbour)
		switch {
		case err == nil:
			web.SetFlashSuccess(h.sm, r.Context(), "Reihenfolge geändert")
		case albumSaid(err) != "":
			web.SetFlashError(h.sm, r.Context(), albumSaid(err))
		default:
			return err
		}
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		return nil
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}
