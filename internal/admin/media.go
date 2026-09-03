package admin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// MediaListData extends LayoutData for the media list page.
type MediaListData struct {
	web.LayoutData
	WebsiteID int64
	Media     []media.Media
	Pagination
	// Filter is echoed back so the controls keep their state and the pager
	// keeps the filter.
	Filter media.Filter
	// MissingAltText counts images with no description — an accessibility gap
	// the operator can otherwise not see.
	MissingAltText int
}

// mediaFilterFromRequest reads the list controls.
func mediaFilterFromRequest(r *http.Request) media.Filter {
	f := media.Filter{
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Unused: r.URL.Query().Get("unused") != "",
	}
	switch r.URL.Query().Get("kind") {
	case "image":
		f.MimePrefix = "image/"
	case "document":
		f.MimePrefix = "application/"
	}
	return f
}

// HandleMediaList renders the media list for a website.
func (h *Handler) HandleMediaList(w http.ResponseWriter, r *http.Request) error {
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

	pageNum := pageParam(r)
	filter := mediaFilterFromRequest(r)
	items, total, err := h.mediaStore.List(r.Context(), websiteID, filter, pageNum, media.DefaultPerPage)
	if err != nil {
		return fmt.Errorf("list media: %w", err)
	}

	missing, err := h.mediaStore.CountMissingAltText(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := MediaListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Medien – %s", ws.Name)),
		WebsiteID:  websiteID,
		Media:      items,
		Pagination: NewPagination(pageNum, media.DefaultPerPage, total).
			WithTarget(fmt.Sprintf("/admin/websites/%d/media", websiteID), "#media-list"),
		Filter:         filter,
		MissingAltText: missing,
	}
	data.ActiveNav = "media"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "media_list", data)
}

// HandleMediaUpload handles media file upload for a website.
func (h *Handler) HandleMediaUpload(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	redirect := fmt.Sprintf("/admin/websites/%d/media", websiteID)

	// Die äussere Grenze ist die grössere der beiden, weil die Art der Datei
	// erst nach dem Lesen feststeht. Die eigentliche Grenze steht darunter.
	r.Body = http.MaxBytesReader(w, r.Body, max(h.cfg.MaxMediaSize, h.cfg.MaxVideoSize))

	file, header, err := r.FormFile("media")
	if err != nil {
		return h.uploadFailed(w, r, redirect, "Datei zu groß oder nicht ausgewählt")
	}
	defer file.Close()

	// Validate MIME via magic bytes (not Content-Type header) per D-16
	mimeType, err := media.ValidateMIME(file, header.Filename)
	if err != nil {
		return h.uploadFailed(w, r, redirect, web.Titlef(r, "Dateityp nicht erlaubt: %s", err))
	}

	// Ein Video darf mehr wiegen als ein Bild — aber nur ein Video.
	limit := h.cfg.MaxMediaSize
	if mimeType == "video/mp4" {
		limit = h.cfg.MaxVideoSize
	}
	if header.Size > limit {
		return h.uploadFailed(w, r, redirect, web.Titlef(r,
			"Die Datei ist größer als erlaubt (%d MB)", limit>>20))
	}

	// An SVG is markup and can pull a stylesheet or an image from another
	// server, which is exactly what the template upload already refuses. The
	// same scanner runs here, so the rule holds for every upload path.
	source, err := h.rejectExternalSVG(file, mimeType)
	if err != nil {
		return h.uploadFailed(w, r, redirect, err.Error())
	}

	// Generate UUID-prefixed filename per D-15
	filename := media.GenerateFilename(header.Filename)

	// Remove EXIF, XMP and IPTC before anything reaches the disk. A product
	// photo taken at home otherwise publishes the photographer's address, and
	// the media route serves the stored bytes verbatim for a year.
	source, stripErr := media.PrepareUpload(source, mimeType, limit)
	if stripErr != nil {
		if source == nil {
			return h.uploadFailed(w, r, redirect, web.Titlef(r, "Hochladen fehlgeschlagen: %s", stripErr))
		}
		logStripFailure(stripErr, header.Filename, mimeType)
	}

	// Store file on disk
	destPath := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(websiteID, 10), filename)
	written, hash, err := media.StoreFile(source, destPath, limit)
	if err != nil {
		return h.uploadFailed(w, r, redirect, web.Titlef(r, "Hochladen fehlgeschlagen: %s", err))
	}

	// The same file twice is almost always an accident. Saying so beats letting
	// the card fill up with copies nobody can tell apart.
	if existing, err := h.mediaStore.FindByHash(r.Context(), websiteID, hash); err != nil {
		os.Remove(destPath)
		return err
	} else if existing != nil {
		os.Remove(destPath)
		web.SetFlashWarning(h.sm, r.Context(),
			"Diese Datei gibt es bereits als „"+existing.OriginalName+"“ – es wurde nichts hochgeladen.")
		return h.redirect(w, r, redirect)
	}

	// Create DB record — record the bytes actually written, not the client-supplied size
	m, err := h.mediaStore.Create(r.Context(), websiteID, filename, header.Filename, mimeType, written, hash)
	if err != nil {
		// Clean up partial file on DB error
		os.Remove(destPath)
		return fmt.Errorf("create media record: %w", err)
	}

	message := "Datei hochgeladen"
	if warning := h.makeVariants(r, m, filepath.Dir(destPath), destPath); warning != "" {
		message += " – " + warning
	}
	if m.NeedsAltText() {
		message += " – bitte noch eine Bildbeschreibung eintragen"
	}
	web.SetFlashSuccess(h.sm, r.Context(), message)
	return h.redirect(w, r, redirect)
}

// makeVariants generates the scaled copies of an uploaded image and returns a
// note for the operator when it could not.
//
// A failure here never fails the upload: the original is stored and usable, it
// just has no smaller siblings. What it must not do is stay silent, because an
// image that quietly skipped the pipeline is one nobody will notice is heavy.
func (h *Handler) makeVariants(r *http.Request, m *media.Media, destDir, sourcePath string) string {
	if !media.CanMakeVariants(m.MimeType) {
		return ""
	}

	variants, err := media.MakeVariantsThrottled(sourcePath, destDir, m.Filename, m.MimeType, h.cfg.MaxMegapixels)
	if err != nil {
		if errors.Is(err, media.ErrTooManyPixels) {
			return fmt.Sprintf("das Bild ist zu groß für verkleinerte Fassungen (Grenze: %d Megapixel)",
				h.cfg.MaxMegapixels)
		}
		slog.Warn("could not create image variants", "err", err, "media", m.ID)
		return "verkleinerte Fassungen konnten nicht erstellt werden"
	}

	width, height, err := media.Dimensions(sourcePath)
	if err != nil {
		slog.Warn("could not read image dimensions", "err", err, "media", m.ID)
		return ""
	}
	if err := h.mediaStore.SaveVariants(r.Context(), m.ID, width, height, variants); err != nil {
		slog.Error("could not store image variants", "err", err, "media", m.ID)
		return "verkleinerte Fassungen konnten nicht gespeichert werden"
	}
	return ""
}

// uploadFailed reports a rejected upload without failing the request.
func (h *Handler) uploadFailed(w http.ResponseWriter, r *http.Request, redirect, message string) error {
	web.SetFlashError(h.sm, r.Context(), message)
	return h.redirect(w, r, redirect)
}

// rejectExternalSVG runs an uploaded SVG through the same external-subresource
// scanner that template archives go through, and returns the reader the caller
// should store from.
//
// The scanner needs the whole document, so an SVG is read into memory first —
// bounded by MaxMediaSize, which the caller has already applied to the body.
func (h *Handler) rejectExternalSVG(file io.ReadSeeker, mimeType string) (io.Reader, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("Datei konnte nicht gelesen werden")
	}
	if mimeType != "image/svg+xml" {
		return file, nil
	}

	content, err := io.ReadAll(io.LimitReader(file, h.cfg.MaxMediaSize+1))
	if err != nil {
		return nil, fmt.Errorf("Datei konnte nicht gelesen werden")
	}
	if refs := tmplmgr.CheckExternalRefs("upload.svg", string(content)); len(refs) > 0 {
		return nil, fmt.Errorf("Die SVG-Datei lädt %s von einem fremden Server; das ist nicht erlaubt", refs[0].URL)
	}
	return bytes.NewReader(content), nil
}

// HandleMediaMeta stores the description and caption of a file.
func (h *Handler) HandleMediaMeta(w http.ResponseWriter, r *http.Request) error {
	websiteID, mediaID, m, ok, err := h.lookupMedia(w, r)
	if err != nil || !ok {
		return err
	}
	_ = m

	if err := r.ParseForm(); err != nil {
		return err
	}
	if err := h.mediaStore.UpdateMeta(r.Context(), mediaID,
		strings.TrimSpace(r.FormValue("alt_text")),
		strings.TrimSpace(r.FormValue("caption"))); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Beschreibung gespeichert")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/media", websiteID))
}

// HandleMediaDelete deletes a media file.
func (h *Handler) HandleMediaDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, mediaID, _, ok, err := h.lookupMedia(w, r)
	if err != nil || !ok {
		return err
	}
	redirect := fmt.Sprintf("/admin/websites/%d/media", websiteID)

	force := r.URL.Query().Get("force") == "1" || r.FormValue("force") == "1"
	err = h.mediaStore.Delete(r.Context(), mediaID, h.cfg.DataDir, force)

	var inUse *media.InUseError
	if errors.As(err, &inUse) {
		// Naming the pages is the whole point: "in use" without saying where
		// leaves the operator to search by hand.
		web.SetFlashError(h.sm, r.Context(), fmt.Sprintf(
			"Die Datei wird noch verwendet auf: %s. Zum Löschen trotzdem bestätigen.",
			strings.Join(inUse.Pages, ", ")))
		return h.redirect(w, r, redirect)
	}
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Löschen fehlgeschlagen: %s", err))
		return h.redirect(w, r, redirect)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Datei gelöscht")
	return h.redirect(w, r, redirect)
}

// lookupMedia resolves the route values and checks the file belongs to the
// website in the path, so one site's route cannot reach another's file.
func (h *Handler) lookupMedia(w http.ResponseWriter, r *http.Request) (websiteID, mediaID int64, m *media.Media, ok bool, err error) {
	websiteID, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, 0, nil, false, nil
	}
	mediaID, err = strconv.ParseInt(r.PathValue("mediaID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, 0, nil, false, nil
	}

	m, err = h.mediaStore.GetByID(r.Context(), mediaID)
	if err != nil {
		return 0, 0, nil, false, fmt.Errorf("get media: %w", err)
	}
	if m == nil || m.WebsiteID != websiteID {
		http.NotFound(w, r)
		return 0, 0, nil, false, nil
	}
	return websiteID, mediaID, m, true, nil
}

// HandleMediaServe serves a media file publicly with correct Content-Type and immutable cache.
func (h *Handler) HandleMediaServe(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("websiteID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	filename := r.PathValue("filename")
	if filename == "" {
		http.NotFound(w, r)
		return nil
	}

	// Security: validate filename has no path separators (T-04-11)
	if strings.ContainsAny(filename, "/\\") {
		http.NotFound(w, r)
		return nil
	}

	// This route is registered on the root mux and therefore never passes
	// through the domain resolver, so the website has to be checked here. Without
	// it, deactivating a site left its images and PDFs publicly downloadable —
	// a switched-off customer kept serving their price list.
	//
	// Host-to-site enforcement stays deliberately out: it would break the admin
	// preview and legitimate cross-site reuse of the same file.
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return fmt.Errorf("get website for media: %w", err)
	}
	if ws == nil || !ws.Active {
		http.NotFound(w, r)
		return nil
	}

	// Look up in DB for mime_type (T-04-13: Content-Type from DB, not filesystem).
	// Scaled copies resolve here too — they live beside their original and are
	// what every srcset entry points at.
	m, err := h.mediaStore.ResolveServed(r.Context(), websiteID, filename)
	if err != nil {
		return fmt.Errorf("get media: %w", err)
	}
	if m == nil {
		http.NotFound(w, r)
		return nil
	}

	diskPath := filepath.Join(h.cfg.DataDir, "media", strconv.FormatInt(websiteID, 10), filename)

	w.Header().Set("Content-Type", m.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Disposition", "inline")
	// Media is served from the same origin as /admin. SVG and PDF can carry
	// active content, so they get a CSP that neutralises scripts when the file
	// is opened directly; nosniff keeps everything else from being reinterpreted.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if m.MimeType == "image/svg+xml" || m.MimeType == "application/pdf" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	http.ServeFile(w, r, diskPath)
	return nil
}

// logStripFailure records that metadata could not be removed. An upload must
// never fail over it — the file is still usable, it just kept its EXIF block.
func logStripFailure(err error, filename, mimeType string) {
	slog.Warn("could not strip image metadata", "err", err, "file", filename, "mime", mimeType)
}
