package admin

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The question after an upload: this picture is bigger than media.MaxSide on
// its longest side, shall it be scaled down?
//
// A plain form on a page of its own, so it works without a script and the
// answer is one POST that either path makes identically. Between the upload and
// the answer the pictures wait on disk in media.Staging, scoped to the website
// and to the user who uploaded them.

// scaleFile is one line of the question page.
type scaleFile struct {
	Name                    string
	Width, Height           int
	ScaledWidth, ScaledHigh int
}

// scaleData is the question page.
type scaleData struct {
	web.LayoutData
	WebsiteID int64
	Token     string
	MaxSide   int
	Files     []scaleFile
}

// scaleDecision is what the form's two buttons send as "decision".
const (
	decisionScale = "scale"
	decisionKeep  = "keep"
)

// scaleStaged finds the website and the held upload for a request, answering
// the request itself in every case that is not "found": an unknown website is
// a 404, another user's staging is a 404, an expired or unknown token is a
// flash and a redirect to the library. ok is true only when staged is usable.
func (h *Handler) scaleStaged(w http.ResponseWriter, r *http.Request, token string, take bool) (websiteID int64, staged *media.Staged, ok bool, err error) {
	websiteID, err = strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, nil, false, nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return 0, nil, false, err
	}
	if ws == nil {
		http.NotFound(w, r)
		return 0, nil, false, nil
	}

	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if take {
		staged, err = h.mediaStaging.Take(websiteID, token, userID)
	} else {
		staged, err = h.mediaStaging.Get(websiteID, token, userID)
	}
	switch {
	case errors.Is(err, media.ErrStagingForeign):
		http.NotFound(w, r)
		return websiteID, nil, false, nil
	case errors.Is(err, media.ErrStagingExpired):
		web.SetFlashError(h.sm, r.Context(), web.T(r, "This upload has expired – please upload the images again."))
		return websiteID, nil, false, h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/media", websiteID))
	case err != nil:
		return websiteID, nil, false, err
	}
	return websiteID, staged, true, nil
}

// HandleMediaScaleForm shows the question.
func (h *Handler) HandleMediaScaleForm(w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	websiteID, staged, ok, err := h.scaleStaged(w, r, token, false)
	if err != nil || !ok {
		return err
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}

	data := scaleData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Scale down to fit within %d px?", media.MaxSide)),
		WebsiteID:  websiteID,
		Token:      token,
		MaxSide:    media.MaxSide,
	}
	for _, f := range staged.Files() {
		sw, sh := media.ScaledSize(f.Width, f.Height)
		data.Files = append(data.Files, scaleFile{
			Name: f.Name, Width: f.Width, Height: f.Height, ScaledWidth: sw, ScaledHigh: sh,
		})
	}
	data.ActiveNav = "media"
	data.CurrentWebsite = ws
	w.Header().Set("Cache-Control", "no-store")
	return web.RenderAdmin(w, h.templates, r, "media_scale", data)
}

// HandleMediaScale takes the answer: every held picture is scaled down and
// stored, or stored as it was uploaded.
func (h *Handler) HandleMediaScale(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	token := r.PostFormValue("token")
	decision := r.PostFormValue("decision")

	// Looked at before the staging is claimed: an answer that is neither
	// changes nothing and shows the question again.
	if decision != decisionScale && decision != decisionKeep {
		websiteID, _, ok, err := h.scaleStaged(w, r, token, false)
		if err != nil || !ok {
			return err
		}
		return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/media/scale?token=%s", websiteID, url.QueryEscape(token)))
	}

	websiteID, staged, ok, err := h.scaleStaged(w, r, token, true)
	if err != nil || !ok {
		return err
	}
	defer staged.Discard()

	var names []string
	var results []uploadResult
	for i, f := range staged.Files() {
		names = append(names, f.Name)
		content, err := staged.Content(i)
		if err != nil {
			results = append(results, h.uploadResultOf(r, mediaUpload{}, err, ""))
			continue
		}
		results = append(results, h.keepStaged(r, websiteID, f.Name, content, decision == decisionScale))
	}

	redirect := fmt.Sprintf("/admin/websites/%d/media", websiteID)
	return h.redirect(w, r, h.reportUploads(r, redirect, names, results))
}

// keepStaged stores one picture that was held, scaled down or as it was.
//
// It goes through media.Check again: what is on disk is bytes an earlier
// request wrote, and nothing about them is taken on trust, so a staging file
// that was replaced by something Check refuses stores nothing.
func (h *Handler) keepStaged(r *http.Request, websiteID int64, name string, content []byte, scale bool) uploadResult {
	header := &multipart.FileHeader{Filename: name, Size: int64(len(content))}
	in, err := h.checkUpload(memFile{bytes.NewReader(content)}, header)
	if err != nil {
		return h.uploadResultOf(r, mediaUpload{}, err, "")
	}

	note := ""
	if scale {
		scaled, err := in.Scaled(h.cfg.MaxMegapixels)
		if err != nil {
			// The original is stored: failing to make the picture smaller is
			// no reason to lose it.
			slog.Warn("could not scale an image down", "err", err, "website", websiteID, "file", name)
			note = web.T(r, "could not be scaled down, kept as uploaded")
		} else {
			in = scaled
			note = web.Titlef(r, "scaled down to fit within %d px", media.MaxSide)
		}
	}
	up, err := h.keepChecked(r.Context(), websiteID, in)
	return h.uploadResultOf(r, up, err, note)
}
