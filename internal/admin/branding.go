package admin

import (
	"bytes"
	"encoding/xml"
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

// brandingData is the "Marke" screen.
type brandingData struct {
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
	data := brandingData{
		LayoutData: web.NewLayoutData(r, h.sm, "Brand"),
		FormState:  web.NewFormState(),
		Brand:      branding.Current(),
		HasLogo:    branding.LogoPath() != "",
	}
	data.ActiveNav = "branding"
	return web.RenderAdmin(w, h.templates, r, "branding", data)
}

func (h *Handler) handleBrandingPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(2 * branding.MaxLogoBytes); err != nil {
		web.SetFlashError(h.sm, r.Context(), "The form could not be read")
		return h.redirect(w, r, "/admin/marke")
	}

	if r.FormValue("logo_entfernen") != "" {
		if err := branding.RemoveLogo(); err != nil {
			return err
		}
		branding.Load(r.Context(), h.db.Read)
		web.SetFlashSuccess(h.sm, r.Context(), "Logo removed")
		return h.redirect(w, r, "/admin/marke")
	}

	if err := branding.Save(r.Context(), h.db.Write,
		r.FormValue("name"), r.FormValue("zeichen")); err != nil {
		return err
	}

	// The picture is optional: a save without one keeps whatever is there.
	if file, header, err := r.FormFile("logo"); err == nil {
		defer file.Close()
		if reason := h.storeLogo(r, file, header.Filename); reason != "" {
			web.SetFlashError(h.sm, r.Context(), reason)
			return h.redirect(w, r, "/admin/marke")
		}
		branding.Load(r.Context(), h.db.Read)
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Brand saved")
	return h.redirect(w, r, "/admin/marke")
}

// storeLogo checks and writes the picture, or says in German why not.
//
// The check is on the bytes and not on the file name: a file called logo.png
// that is something else would be served as a PNG that no browser can draw,
// and — the case that matters — an SVG is a document that can carry script, so
// what claims to be one has to actually look like one.
func (h *Handler) storeLogo(r *http.Request, file io.Reader, filename string) string {
	data, err := io.ReadAll(io.LimitReader(file, branding.MaxLogoBytes+1))
	if err != nil {
		return web.T(r, "The file could not be read")
	}
	if len(data) > branding.MaxLogoBytes {
		return web.T(r, "The logo is larger than 512 KB")
	}
	if len(data) == 0 {
		return web.T(r, "The file is empty")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".png" && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
	case ext == ".webp" && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[:min(64, len(data))], []byte("WEBP")):
	case ext == ".svg" && looksLikeSVG(data):
	default:
		return web.T(r, "PNG, WebP and SVG are allowed — and the file has to really be one of them")
	}

	if err := branding.WriteLogo(ext, data); err != nil {
		return web.T(r, "The logo could not be saved")
	}
	return ""
}

// looksLikeSVG accepts a document that really is an SVG and carries nothing
// executable.
//
// An SVG is served from this origin and drawn by the browser, so a <script> in
// it would run with the administration's own rights. The content security
// policy forbids that a second time; this is the first.
//
// # Why this parses instead of searching
//
// It used to be a prefix test and a list of four forbidden substrings —
// "<script", "onload=", "javascript:", "<foreignobject". Every one of these got
// through it, and they were found by writing the test rather than by reading
// the code:
//
//	<svg … onerror="alert(1)">          a different event, and there are eighty
//	<svg … onmouseover="alert(1)">      of them
//	<svg … onload ="alert(1)">          a space, which the substring has not got
//	<a href="javascript&#58;alert(1)">  the same colon, written as XML spells it
//
// A denylist of spellings loses to the number of spellings; that is what a
// denylist is. So the document is parsed, which answers the question the
// function's name asks — is this an SVG — and lets the rules be about
// structure: an attribute whose NAME begins with "on" is an event handler
// whatever it is called, and an attribute value is compared after the decoder
// has resolved its character references.
//
// Stricter than what went before, deliberately and in two ways that can refuse
// a file the old check took: it has to parse as XML (a truncated file does
// not), and its root element has to be <svg> (an RSS feed is not one). Both
// were accepted before and neither is a logo.
//
// Not covered here, on purpose: a <style> element is allowed, because that is
// what a drawing program writes and CSS is not script. What CSS could fetch
// from elsewhere is the content security policy's question, and it answers it
// with default-src 'self'.
func looksLikeSVG(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// A charset this program cannot decode is a file it cannot check, and an
	// unchecked file is not accepted. nil is the default and is stated because
	// the alternative is a one-line invitation.
	dec.CharsetReader = nil

	depth, root, done := 0, false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.Directive:
			// <!DOCTYPE … [<!ENTITY …>]> is the billion-laughs shape.
			// encoding/xml does not expand a declared entity and errors on a
			// reference to one, so this is belt to that; it is here so the
			// refusal says what it means rather than arriving as a parse error.
			if bytes.Contains(bytes.ToUpper(t), []byte("ENTITY")) {
				return false
			}
		case xml.StartElement:
			if !root {
				if !strings.EqualFold(t.Name.Local, "svg") {
					return false
				}
				root = true
			} else if done {
				// A second element beside the root: not one document.
				return false
			}
			depth++
			if !elementIsDrawable(t.Name.Local) {
				return false
			}
			for _, a := range t.Attr {
				// An event handler is an attribute whose name begins with
				// "on" — onload, onerror, onmouseover, and the eighty others.
				// The name, not the text: the decoder has already separated it
				// from whatever whitespace surrounded the equals sign.
				if len(a.Name.Local) >= 2 && strings.EqualFold(a.Name.Local[:2], "on") {
					return false
				}
				if isScriptURL(a.Value) {
					return false
				}
			}
		case xml.EndElement:
			depth--
			if depth == 0 {
				done = true
			}
		}
	}
	// root closed and nothing left open.
	//
	// `done` carries its weight — without it a second <svg> beside the first is
	// accepted, and a test says so. `depth == 0` does not, and that is recorded
	// rather than removed: a truncated file never reaches this line, because
	// the decoder returns ErrUnexpectedEOF for the element it never closed and
	// the error arm above refuses it. Measured by mutation, not argued. It
	// stays because it is what the sentence above claims and costs one
	// comparison.
	return root && done && depth == 0
}

// elementIsDrawable refuses the elements that are not drawing.
//
// <script> is the obvious one. <foreignObject> is HTML inside an SVG and brings
// everything HTML can do with it. <handler> is SVG Tiny's own script element,
// which nothing writes and every parser that knows it would run.
func elementIsDrawable(name string) bool {
	switch strings.ToLower(name) {
	case "script", "foreignobject", "handler":
		return false
	}
	return true
}

// isScriptURL reports whether an attribute value is an address that executes.
//
// The value arrives with its character references already resolved, so
// "javascript&#58;alert(1)" is compared as "javascript:alert(1)" — which is the
// case a substring search over the raw bytes cannot see at all. Leading
// whitespace and control characters are stripped because a browser strips them
// before it looks at the scheme.
func isScriptURL(value string) bool {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	})
	lower := strings.ToLower(trimmed)
	for _, scheme := range []string{"javascript:", "vbscript:", "data:text/html"} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
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
