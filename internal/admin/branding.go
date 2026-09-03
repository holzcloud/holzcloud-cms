package admin

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The name and the mark of this installation.
//
// A screen of its own rather than a corner of the website settings: it belongs
// to the administration and not to any one website, and somebody looking for it
// looks under System.

// BrandingData is the "Marke" screen.
type BrandingData struct {
	web.LayoutData
	web.FormState
	Brand branding.Brand
	// HasLogo drives the remove button, which is hidden when there is nothing
	// to remove.
	HasLogo bool
}

// HandleBranding shows and stores the name, the letter and the picture.
func (h *Handler) HandleBranding(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		return h.handleBrandingPost(w, r)
	}
	data := BrandingData{
		LayoutData: web.NewLayoutData(r, h.sm, "Marke"),
		FormState:  web.NewFormState(),
		Brand:      branding.Current(),
		HasLogo:    branding.LogoPath() != "",
	}
	data.ActiveNav = "branding"
	return web.RenderAdmin(w, h.templates, r, "branding", data)
}

func (h *Handler) handleBrandingPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(2 * branding.MaxLogoBytes); err != nil {
		web.SetFlashError(h.sm, r.Context(), "Das Formular konnte nicht gelesen werden")
		return h.redirect(w, r, "/admin/marke")
	}

	if r.FormValue("logo_entfernen") != "" {
		if err := branding.RemoveLogo(); err != nil {
			return err
		}
		branding.Load(r.Context(), h.db.Read)
		web.SetFlashSuccess(h.sm, r.Context(), "Logo entfernt")
		return h.redirect(w, r, "/admin/marke")
	}

	if err := branding.Save(r.Context(), h.db.Write,
		r.FormValue("name"), r.FormValue("zeichen")); err != nil {
		return err
	}

	// The picture is optional: a save without one keeps whatever is there.
	if file, header, err := r.FormFile("logo"); err == nil {
		defer file.Close()
		if reason := h.storeLogo(file, header.Filename); reason != "" {
			web.SetFlashError(h.sm, r.Context(), reason)
			return h.redirect(w, r, "/admin/marke")
		}
		branding.Load(r.Context(), h.db.Read)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Marke gespeichert")
	return h.redirect(w, r, "/admin/marke")
}

// storeLogo checks and writes the picture, or says in German why not.
//
// The check is on the bytes and not on the file name: a file called logo.png
// that is something else would be served as a PNG that no browser can draw,
// and — the case that matters — an SVG is a document that can carry script, so
// what claims to be one has to actually look like one.
func (h *Handler) storeLogo(file io.Reader, filename string) string {
	data, err := io.ReadAll(io.LimitReader(file, branding.MaxLogoBytes+1))
	if err != nil {
		return "Die Datei konnte nicht gelesen werden"
	}
	if len(data) > branding.MaxLogoBytes {
		return "Das Logo ist grösser als 512 KB"
	}
	if len(data) == 0 {
		return "Die Datei ist leer"
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".png" && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
	case ext == ".webp" && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[:min(64, len(data))], []byte("WEBP")):
	case ext == ".svg" && looksLikeSVG(data):
	default:
		return "Erlaubt sind PNG, WebP und SVG — und die Datei muss auch eines davon sein"
	}

	if err := branding.WriteLogo(ext, data); err != nil {
		return "Das Logo konnte nicht gespeichert werden"
	}
	return ""
}

// looksLikeSVG accepts a document that starts as XML or SVG and carries nothing
// executable.
//
// An SVG is served from this origin and drawn by the browser, so a <script> in
// it would run with the administration's own rights. The content security
// policy forbids it a second time; this is the first.
func looksLikeSVG(data []byte) bool {
	head := bytes.ToLower(bytes.TrimSpace(data))
	if !bytes.HasPrefix(head, []byte("<?xml")) && !bytes.HasPrefix(head, []byte("<svg")) {
		return false
	}
	lower := bytes.ToLower(data)
	for _, forbidden := range [][]byte{[]byte("<script"), []byte("onload="), []byte("javascript:"), []byte("<foreignobject")} {
		if bytes.Contains(lower, forbidden) {
			return false
		}
	}
	return true
}

// HandleBrandingLogo serves the uploaded picture.
//
// Not through /assets: those are compiled into the binary, and this one lies in
// the data directory. Its own route keeps the two apart, so an uploaded file can
// never shadow one of ours.
func (h *Handler) HandleBrandingLogo(w http.ResponseWriter, r *http.Request) error {
	path := branding.LogoPath()
	if path == "" {
		http.NotFound(w, r)
		return nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	}
	// A logo changes about once a year and its address carries the file's time,
	// so it may be cached hard.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
	return nil
}
