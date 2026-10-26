package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
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

	// Every file is checked and, unless it is a raster picture over
	// media.MaxSide, stored at once. Those that are over the side are held back
	// and asked about together on one page: one decision for the whole upload,
	// and the others of the same upload are not made to wait for it.
	var names []string
	var results []uploadResult
	var asked []*media.Intake
	for _, fh := range headers {
		res, held := h.uploadOne(r, websiteID, fh)
		if held != nil {
			asked = append(asked, held)
			continue
		}
		names = append(names, fh.Filename)
		results = append(results, res)
	}
	if len(asked) == 0 {
		return h.redirect(w, r, h.reportUploads(r, redirect, names, results))
	}

	if len(results) > 0 {
		h.reportUploads(r, redirect, names, results)
	}
	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	token, err := h.mediaStaging.Put(websiteID, userID, asked)
	if err != nil {
		slog.Error("could not stage an upload", "err", err, "website", websiteID)
		return h.uploadFailed(w, r, redirect, web.Titlef(r, "Upload failed: %s", err))
	}
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/media/scale?token=%s", websiteID, token))
}

// reportUploads puts what became of the stored files into flash messages and
// returns where to go next.
//
// One file answers with its own sentence and its own warning. Several are
// summed up, and every refusal is still named — a batch that silently drops the
// one file that failed is worse than no batch at all.
func (h *Handler) reportUploads(r *http.Request, redirect string, names []string, results []uploadResult) string {
	if len(results) == 1 {
		res := results[0]
		switch res.kind {
		case uploadRefused:
			web.SetFlashError(h.sm, r.Context(), res.message)
		case uploadExisted:
			web.SetFlashWarning(h.sm, r.Context(), res.message)
		default:
			web.SetFlashSuccess(h.sm, r.Context(), res.message)
		}
		return redirect
	}

	var kept, noAlt int
	var problems []string
	for i, res := range results {
		switch res.kind {
		case uploadKept:
			kept++
			if res.needsAlt {
				noAlt++
			}
		default:
			problems = append(problems, names[i]+": "+res.message)
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
	return redirect
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

// uploadOne checks one file of an upload and stores and scales it.
//
// The exception is a raster picture whose longest side is over media.MaxSide:
// that one is not stored. It comes back as the second result, checked and
// still in memory, for the caller to hold until the operator has answered.
func (h *Handler) uploadOne(r *http.Request, websiteID int64, header *multipart.FileHeader) (uploadResult, *media.Intake) {
	file, err := header.Open()
	if err != nil {
		return uploadResult{kind: uploadRefused, message: web.T(r, "File too large or not selected")}, nil
	}
	defer file.Close()

	in, err := h.checkUpload(file, header)
	if err != nil {
		return h.uploadResultOf(r, mediaUpload{}, err, ""), nil
	}
	if _, _, over := in.Oversize(h.cfg.MaxMegapixels); over {
		return uploadResult{}, in
	}
	up, err := h.keepChecked(r.Context(), websiteID, in)
	return h.uploadResultOf(r, up, err, ""), nil
}

// uploadResultOf turns what keepChecked answered into the result a screen
// reports. A note is one more part of the dash-joined message, for what the
// upload went through on the way (a scale-down), and is already a finished
// catalogue sentence.
func (h *Handler) uploadResultOf(r *http.Request, up mediaUpload, err error, note string) uploadResult {
	var refusal media.Refusal
	switch {
	case errors.As(err, &refusal):
		return uploadResult{kind: uploadRefused, message: web.MediaRefusal(r, err)}
	case err != nil:
		return uploadResult{kind: uploadRefused, message: web.Titlef(r, "Upload failed: %s", err)}
	case up.Existed:
		return uploadResult{kind: uploadExisted, message: web.Titlef(r,
			"This file already exists as “%s” – nothing was uploaded.", up.Media.OriginalName)}
	}

	// Each part is its own catalogue sentence; only the dash between them is
	// assembled here. A message glued together out of half-sentences is a
	// message the collector never sees.
	parts := []string{web.T(r, "File uploaded")}
	if note != "" {
		parts = append(parts, note)
	}
	if warning := h.variantWarning(r, up.Variants); warning != "" {
		parts = append(parts, warning)
	}
	if up.Media.NeedsAltText() {
		parts = append(parts, web.T(r, "please still enter an image description"))
	}
	return uploadResult{kind: uploadKept, message: strings.Join(parts, " – "), needsAlt: up.Media.NeedsAltText()}
}

// mediaUpload is what became of one file that was taken in.
type mediaUpload struct {
	// Media is the stored file — or, when Existed, the one that already held
	// the same bytes.
	Media *media.Media
	// Existed says nothing was stored because the bytes were already there.
	Existed bool
	// Variants is why the scaled copies are missing: nil when they were made
	// or the format has none, media.ErrTooManyPixels, errVariantsNotMade or
	// errVariantsNotStored.
	Variants error
}

// The two ways the scaled copies can fail besides too many pixels.
var (
	errVariantsNotMade   = errors.New("the scaled copies could not be created")
	errVariantsNotStored = errors.New("the scaled copies could not be stored")
)

// keepUpload is the whole intake of one file for an AI key: media.Check, Keep,
// the scaled copies and the plugin event. It knows no request, because its
// caller has none — and a second copy of these steps is exactly how one path
// would come to skip the SVG scanner. It never asks about the size of a
// picture: the question belongs to the upload screen, which has an operator to
// ask.
//
// A refusal by media.Check comes back as a media.Refusal, so each caller can
// put it into words for its own reader.
func (h *Handler) keepUpload(ctx context.Context, websiteID int64, file multipart.File, header *multipart.FileHeader) (mediaUpload, error) {
	in, err := h.checkUpload(file, header)
	if err != nil {
		return mediaUpload{}, err
	}
	return h.keepChecked(ctx, websiteID, in)
}

// checkUpload is media.Check with the installation's limits. Every check lives
// in media.Check, so the contact form's attachment goes through exactly the
// same ones: magic bytes rather than the browser's claim, the SVG scanner, the
// size for the kind. Nothing is written yet.
func (h *Handler) checkUpload(file multipart.File, header *multipart.FileHeader) (*media.Intake, error) {
	return media.Check(file, header, media.Limits{
		Image: h.cfg.MaxMediaSize, Video: h.cfg.MaxVideoSize,
	})
}

// keepChecked stores a file media.Check has passed, makes its scaled copies and
// tells the plugins. A *media.Intake can only come from Check or from
// Intake.Scaled, so nothing that gets here has skipped the checks.
func (h *Handler) keepChecked(ctx context.Context, websiteID int64, in *media.Intake) (mediaUpload, error) {
	m, existed, err := in.Keep(ctx, h.mediaStore, websiteID, h.cfg.DataDir)
	if err != nil {
		return mediaUpload{}, err
	}
	if existed {
		return mediaUpload{Media: m, Existed: true}, nil
	}
	h.emitMediaAdded(websiteID, m)
	destPath := media.Path(h.cfg.DataDir, websiteID, m.Filename)
	variants := h.makeVariants(ctx, m, filepath.Dir(destPath), destPath)

	// Re-read, so the caller sees the dimensions the variants recorded.
	if fresh, err := h.mediaStore.GetByID(ctx, m.ID); err == nil && fresh != nil {
		m = fresh
	}
	return mediaUpload{Media: m, Variants: variants}, nil
}

// makeVariants generates the scaled copies of an uploaded image and returns
// why it could not.
//
// A failure here never fails the upload: the original is stored and usable, it
// just has no smaller siblings. What it must not do is stay silent, because an
// image that quietly skipped the pipeline is one nobody will notice is heavy.
func (h *Handler) makeVariants(ctx context.Context, m *media.Media, destDir, sourcePath string) error {
	if !media.CanMakeVariants(m.MimeType) {
		return nil
	}

	variants, err := media.MakeVariantsThrottled(sourcePath, destDir, m.Filename, m.MimeType, h.cfg.MaxMegapixels)
	if err != nil {
		if errors.Is(err, media.ErrTooManyPixels) {
			return media.ErrTooManyPixels
		}
		slog.Warn("could not create image variants", "err", err, "media", m.ID)
		return errVariantsNotMade
	}

	width, height, err := media.Dimensions(sourcePath)
	if err != nil {
		slog.Warn("could not read image dimensions", "err", err, "media", m.ID)
		return nil
	}
	if err := h.mediaStore.SaveVariants(ctx, destDir, m.ID, width, height, variants); err != nil {
		slog.Error("could not store image variants", "err", err, "media", m.ID)
		return errVariantsNotStored
	}
	return nil
}

// variantWarning is the note for the operator when makeVariants failed.
func (h *Handler) variantWarning(r *http.Request, err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, media.ErrTooManyPixels):
		return web.Titlef(r, "the image is too large for scaled copies (limit: %d megapixels)", h.cfg.MaxMegapixels)
	case errors.Is(err, errVariantsNotStored):
		return web.T(r, "the scaled copies could not be stored")
	default:
		return web.T(r, "the scaled copies could not be created")
	}
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
