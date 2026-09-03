package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Cropping without a mouse.
//
// A drag-a-rectangle tool needs JavaScript, and this project permits htmx and
// nothing else. What replaces it is an <input type="image">: a browser submits
// the coordinates of a click on it as ordinary form fields, so pointing at the
// subject costs no script at all. The rest — which shape, how close in, which
// way up — is three selects.
//
// The keyboard path is not an afterthought. An <input type="image"> reached by
// keyboard submits 0,0, which would silently move the subject to the top left
// corner, so the same values also sit in two number fields that can be typed.

// CropData is the crop screen.
type CropData struct {
	web.LayoutData
	WebsiteID int64
	Media     *media.Media
	Ratios    []media.Ratio
	// Croppable is false for a format the pipeline cannot decode — an SVG, a
	// PDF. The screen then explains that instead of offering controls that
	// would fail on submit.
	Croppable bool
	// Cropped says the upload has been changed, so the way back can be offered.
	Cropped bool
	// ShownHeight is how tall the picture is drawn at the fixed preview width.
	// The click arrives in pixels of that drawing and has to become a
	// percentage, and only the server knows what it drew.
	ShownHeight int
	// PreviewWidth is that fixed width, so the template and the handler cannot
	// disagree about it.
	PreviewWidth int
}

// previewWidth is how wide the crop screen draws the picture.
//
// Fixed rather than fluid, and the stylesheet must not scale it: the click
// comes back in pixels of the rendered image, so the declared width has to be
// the rendered width. It was 640 with a max-width of 100% next to it, which
// meant a click at 16 percent arrived as 13 — wrong in a way nothing on screen
// shows, and only noticeable when a crop sits slightly off.
//
// 480 fits the column it sits in on an ordinary screen. Where it does not, the
// picture scrolls sideways rather than shrinking.
const previewWidth = 480

// HandleMediaCrop shows the crop screen.
func (h *Handler) HandleMediaCrop(w http.ResponseWriter, r *http.Request) error {
	websiteID, _, m, ok, err := h.lookupMedia(w, r)
	if err != nil || !ok {
		return err
	}

	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := CropData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Bildausschnitt – %s", m.OriginalName)),
		WebsiteID:  websiteID,
		Media:      m,
		Ratios:     media.Ratios,
		Croppable:  media.CanMakeVariants(m.MimeType),
		Cropped:    h.hasOriginal(websiteID, m.Filename),
	}
	data.PreviewWidth = previewWidth
	if m.Width > 0 && m.Height > 0 {
		data.ShownHeight = m.Height * previewWidth / m.Width
	}
	data.ActiveNav = "media"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "media_crop", data)
}

// HandleMediaCropSave applies a crop, or puts the upload back.
func (h *Handler) HandleMediaCropSave(w http.ResponseWriter, r *http.Request) error {
	websiteID, mediaID, m, ok, err := h.lookupMedia(w, r)
	if err != nil || !ok {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	back := fmt.Sprintf("/admin/websites/%d/media/%d/zuschnitt", websiteID, mediaID)

	if r.FormValue("zuruecksetzen") != "" {
		return h.restoreMedia(w, r, websiteID, mediaID, m, back)
	}
	if !media.CanMakeVariants(m.MimeType) {
		web.SetFlashError(h.sm, r.Context(), "Dieses Format lässt sich nicht zuschneiden.")
		return h.redirect(w, r, back)
	}

	crop := cropFromForm(r, m)

	dir := media.WebsiteDir(h.cfg.DataDir, websiteID)
	width, height, err := media.ApplyCrop(dir, m.Filename, m.MimeType, crop, h.cfg.MaxMegapixels)
	switch {
	case errors.Is(err, media.ErrTooManyPixels):
		web.SetFlashError(h.sm, r.Context(), fmt.Sprintf(
			"Das Bild ist zu groß zum Zuschneiden (Grenze: %d Megapixel).", h.cfg.MaxMegapixels))
		return h.redirect(w, r, back)
	case err != nil:
		return err
	}

	if err := h.mediaStore.SaveCrop(r.Context(), mediaID, crop, width, height); err != nil {
		return err
	}
	h.rebuildVariants(r, m, dir, width, height)

	web.SetFlashSuccess(h.sm, r.Context(), fmt.Sprintf(
		"Zugeschnitten auf %d × %d Pixel. Das Original bleibt erhalten.", width, height))
	return h.redirect(w, r, back)
}

// cropFromForm reads the chosen frame.
//
// The click on the picture wins over the number fields when there is one: it is
// the more recent act, and it is the one that felt like pointing at the subject.
// The browser sends it as "fokus.x" and "fokus.y" in pixels of the displayed
// picture, so it has to be turned back into a percentage using the width the
// form states it was drawn at.
func cropFromForm(r *http.Request, m *media.Media) media.Crop {
	c := media.Crop{
		Ratio:    r.FormValue("form"),
		Zoom:     atoiOr(r.FormValue("naehe"), 100),
		Rotation: atoiOr(r.FormValue("drehung"), 0),
		FocusX:   atoiOr(r.FormValue("fokus_x"), m.Crop.FocusX),
		FocusY:   atoiOr(r.FormValue("fokus_y"), m.Crop.FocusY),
	}
	shownW := atoiOr(r.FormValue("gezeigt_breite"), 0)
	shownH := atoiOr(r.FormValue("gezeigt_hoehe"), 0)
	clickX, hasX := r.Form["fokus.x"]
	clickY, hasY := r.Form["fokus.y"]
	if hasX && hasY && shownW > 0 && shownH > 0 {
		// 0,0 is what a keyboard press on the image button sends, and it is
		// indistinguishable from a click on the very corner. Taking it would
		// move somebody's subject to the top left because they pressed Enter,
		// so the number fields keep their value in that one case.
		x, y := atoiOr(clickX[0], -1), atoiOr(clickY[0], -1)
		if x > 0 || y > 0 {
			c.FocusX = x * 100 / shownW
			c.FocusY = y * 100 / shownH
		}
	}
	return c.Normalise()
}

// restoreMedia puts the uploaded picture back.
func (h *Handler) restoreMedia(w http.ResponseWriter, r *http.Request, websiteID, mediaID int64, m *media.Media, back string) error {
	dir := media.WebsiteDir(h.cfg.DataDir, websiteID)
	width, height, err := media.RestoreOriginal(dir, m.Filename)
	if err != nil {
		return err
	}
	// The focus point survives: it says where the subject is, which is still
	// true of the uncropped picture and is what a theme uses to frame it.
	if err := h.mediaStore.SaveCrop(r.Context(), mediaID,
		media.Crop{FocusX: m.Crop.FocusX, FocusY: m.Crop.FocusY}, width, height); err != nil {
		return err
	}
	h.rebuildVariants(r, m, dir, width, height)

	web.SetFlashSuccess(h.sm, r.Context(), "Das hochgeladene Bild ist wieder da.")
	return h.redirect(w, r, back)
}

// rebuildVariants regenerates the scaled copies after the served file changed.
//
// Without this the srcset would keep offering copies of the picture as it was
// before the crop — a browser would pick one and show the old framing, which is
// the sort of thing that looks like the crop simply did not work.
func (h *Handler) rebuildVariants(r *http.Request, m *media.Media, dir string, width, height int) {
	if !media.CanMakeVariants(m.MimeType) {
		return
	}
	variants, err := media.MakeVariantsThrottled(
		filepath.Join(dir, m.Filename), dir, m.Filename, m.MimeType, h.cfg.MaxMegapixels)
	if err != nil {
		slog.Warn("could not rebuild variants after crop", "err", err, "media", m.ID)
		return
	}
	if err := h.mediaStore.SaveVariants(r.Context(), m.ID, width, height, variants); err != nil {
		slog.Error("could not store variants after crop", "err", err, "media", m.ID)
	}
}

// hasOriginal reports whether an untouched copy of the upload is on disk.
func (h *Handler) hasOriginal(websiteID int64, filename string) bool {
	return media.HasOriginal(media.WebsiteDir(h.cfg.DataDir, websiteID), filename)
}
