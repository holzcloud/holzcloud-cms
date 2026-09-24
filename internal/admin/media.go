package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// mediaListData extends LayoutData for the media list page.
type mediaListData struct {
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
	// Library is the collection list on the left.
	Library libraryNav
}

// mediaFilterFromRequest reads the list controls.
func mediaFilterFromRequest(r *http.Request) media.Filter {
	f := media.Filter{
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Unused: r.URL.Query().Get("unused") != "",
		NoAlt:  r.URL.Query().Get("ohne_beschreibung") != "",
	}
	switch r.URL.Query().Get("kind") {
	case "image":
		f.MimePrefix = "image/"
	case "document":
		f.MimePrefix = "application/"
	case "video":
		f.MimePrefix = "video/"
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

	data := mediaListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Media – %s", ws.Name)),
		WebsiteID:  websiteID,
		Media:      items,
		Pagination: newPagination(pageNum, media.DefaultPerPage, total).
			WithTarget(fmt.Sprintf("/admin/websites/%d/media", websiteID), "#media-list"),
		Filter:         filter,
		MissingAltText: missing,
		Library:        h.libraryNavFor(r.Context(), websiteID, activeCollection(filter)),
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

	// Several files at once, up to maxUploadFiles. The outer limit therefore
	// allows that many images, or one video, whichever is larger; each file is
	// then held to the limit for its own kind by media.Check below.
	r.Body = http.MaxBytesReader(w, r.Body, max(h.cfg.MaxMediaSize*maxUploadFiles, h.cfg.MaxVideoSize))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return h.uploadFailed(w, r, redirect, web.T(r, "File too large or not selected"))
	}
	defer r.MultipartForm.RemoveAll()
	headers := r.MultipartForm.File["media"]
	if len(headers) == 0 {
		return h.uploadFailed(w, r, redirect, web.T(r, "File too large or not selected"))
	}
	if len(headers) > maxUploadFiles {
		return h.uploadFailed(w, r, redirect, web.Titlef(r, "At most %d files at once.", maxUploadFiles))
	}

	// One file answers exactly as it always did: its own sentence, its own
	// warning. Several are summed up, and every refusal is still named — a
	// batch that silently drops the one file that failed is worse than no
	// batch at all.
	if len(headers) == 1 {
		res := h.uploadOne(r, websiteID, headers[0])
		switch res.kind {
		case uploadRefused:
			web.SetFlashError(h.sm, r.Context(), res.message)
		case uploadExisted:
			web.SetFlashWarning(h.sm, r.Context(), res.message)
		default:
			web.SetFlashSuccess(h.sm, r.Context(), res.message)
		}
		return h.redirect(w, r, redirect)
	}

	var kept, noAlt int
	var problems []string
	for _, fh := range headers {
		res := h.uploadOne(r, websiteID, fh)
		switch res.kind {
		case uploadKept:
			kept++
			if res.needsAlt {
				noAlt++
			}
		default:
			problems = append(problems, fh.Filename+": "+res.message)
		}
	}
	if kept > 0 {
		msg := web.Titlef(r, "%d files uploaded", kept)
		if noAlt > 0 {
			msg += " – " + web.Titlef(r, "%d of them still need an image description", noAlt)
		}
		web.SetFlashSuccess(h.sm, r.Context(), msg)
	}
	if len(problems) > 0 {
		web.SetFlashWarning(h.sm, r.Context(), strings.Join(problems, " · "))
	}
	// Straight to the images that still need words: describing them right
	// after the upload is the one moment nobody has forgotten what they show.
	if noAlt > 0 {
		redirect += "?ohne_beschreibung=1"
	}
	return h.redirect(w, r, redirect)
}

// maxUploadFiles is how many files one upload may carry.
const maxUploadFiles = 20

type uploadKind int

const (
	uploadKept uploadKind = iota
	uploadExisted
	uploadRefused
)

type uploadResult struct {
	kind     uploadKind
	message  string
	needsAlt bool
}

// uploadOne checks, stores and scales one file of an upload.
func (h *Handler) uploadOne(r *http.Request, websiteID int64, header *multipart.FileHeader) uploadResult {
	file, err := header.Open()
	if err != nil {
		return uploadResult{kind: uploadRefused, message: web.T(r, "File too large or not selected")}
	}
	defer file.Close()

	// Every check lives in media.Check, so the contact form's attachment goes
	// through exactly the same ones: magic bytes rather than the browser's
	// claim, the SVG scanner, the size for the kind. Nothing is written yet.
	in, err := media.Check(file, header, media.Limits{
		Image: h.cfg.MaxMediaSize, Video: h.cfg.MaxVideoSize,
	})
	if err != nil {
		return uploadResult{kind: uploadRefused, message: web.MediaRefusal(r, err)}
	}

	m, existed, err := in.Keep(r.Context(), h.mediaStore, websiteID, h.cfg.DataDir)
	if err != nil {
		return uploadResult{kind: uploadRefused, message: web.Titlef(r, "Upload failed: %s", err)}
	}
	if existed {
		return uploadResult{kind: uploadExisted, message: web.Titlef(r,
			"This file already exists as “%s” – nothing was uploaded.", m.OriginalName)}
	}
	h.emitMediaAdded(websiteID, m)
	destPath := media.Path(h.cfg.DataDir, websiteID, m.Filename)

	// Each part is its own catalogue sentence; only the dash between them is
	// assembled here. A message glued together out of half-sentences is a
	// message the collector never sees.
	parts := []string{web.T(r, "File uploaded")}
	if warning := h.makeVariants(r, m, filepath.Dir(destPath), destPath); warning != "" {
		parts = append(parts, warning)
	}
	if m.NeedsAltText() {
		parts = append(parts, web.T(r, "please still enter an image description"))
	}
	return uploadResult{kind: uploadKept, message: strings.Join(parts, " – "), needsAlt: m.NeedsAltText()}
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
			return web.Titlef(r, "the image is too large for scaled copies (limit: %d megapixels)", h.cfg.MaxMegapixels)
		}
		slog.Warn("could not create image variants", "err", err, "media", m.ID)
		return web.T(r, "the scaled copies could not be created")
	}

	width, height, err := media.Dimensions(sourcePath)
	if err != nil {
		slog.Warn("could not read image dimensions", "err", err, "media", m.ID)
		return ""
	}
	if err := h.mediaStore.SaveVariants(r.Context(), m.ID, width, height, variants); err != nil {
		slog.Error("could not store image variants", "err", err, "media", m.ID)
		return web.T(r, "the scaled copies could not be stored")
	}
	return ""
}

// uploadFailed reports a rejected upload without failing the request.
func (h *Handler) uploadFailed(w http.ResponseWriter, r *http.Request, redirect, message string) error {
	web.SetFlashError(h.sm, r.Context(), message)
	return h.redirect(w, r, redirect)
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

	web.SetFlashSuccess(h.sm, r.Context(), "Description saved")
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
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "The file is still used on: %s. Confirm to delete it anyway.", strings.Join(inUse.Pages, ", ")))
		return h.redirect(w, r, redirect)
	}
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Delete failed: %s", err))
		return h.redirect(w, r, redirect)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "File deleted")
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
