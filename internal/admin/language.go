package admin

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The languages of the administration, as files an operator may add and remove.
//
// The same arrangement as themes: what ships is compiled into the binary, what
// lies in <data>/sprachen wins over it. Nothing here can make the
// administration unreadable — a file that does not parse is refused, a
// translation that does not fit its German original is dropped, and whatever is
// missing falls through to German.

// LanguageData is the "Sprachen" screen.
type LanguageData struct {
	web.LayoutData
	// Stats is every language this installation can show.
	Stats []i18n.Stat
	// Dir is the folder the operator copies files into, shown so they can find
	// it without reading the documentation.
	Dir string
	// Strings is how many German strings there are to translate.
	Strings int
}

// HandleLanguages shows the languages and what may be done with them.
func (h *Handler) HandleLanguages(w http.ResponseWriter, r *http.Request) error {
	data := LanguageData{
		LayoutData: web.NewLayoutData(r, h.sm, "Sprachen"),
		Stats:      i18n.Stats(),
		Dir:        i18n.Dir(),
		Strings:    len(i18n.SourceStrings()),
	}
	data.ActiveNav = "languages"
	return web.RenderAdmin(w, h.templates, r, "language_list", data)
}

// HandleLanguageReload reads the folder again.
//
// The button exists because the alternative is restarting the service, and an
// operator who has just copied a file in should not have to know how. It also
// re-parses the admin templates, because the translation function is baked into
// them at parse time.
func (h *Handler) HandleLanguageReload(w http.ResponseWriter, r *http.Request) error {
	i18n.Reload()
	if err := h.templates.Reload(); err != nil {
		slog.Error("re-parse admin templates", "err", err)
		web.SetFlashError(h.sm, r.Context(), "Die Sprachdateien wurden gelesen, die Bildschirme ließen sich aber nicht neu aufbauen")
		return h.redirect(w, r, "/admin/sprachen")
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Sprachdateien neu eingelesen")
	return h.redirect(w, r, "/admin/sprachen")
}

// HandleLanguageTemplate hands out the file a translator starts from: every
// German string as a key, every value empty.
//
// Deliberately not a half-translated copy of English: a translator working from
// English translates a translation, and the mistakes compound. The German is
// the original, and it is right there in the key.
func (h *Handler) HandleLanguageTemplate(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sprache-vorlage.json"`)
	_, err := w.Write(i18n.Starter())
	return err
}

// HandleLanguageDownload hands out a language that is already installed, so a
// correction starts from what is running rather than from an empty file.
func (h *Handler) HandleLanguageDownload(w http.ResponseWriter, r *http.Request) error {
	code := locale.Normalise(r.PathValue("code"))
	if !i18n.Known(code) || code == i18n.Source {
		http.NotFound(w, r)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, code))
	_, err := w.Write(i18n.Export(code))
	return err
}

// HandleLanguageUpload takes a .json file and puts it in the folder.
//
// The upload is the same act as copying the file in by hand — it exists because
// somebody administering the site from a browser has no shell. Everything is
// checked before the file is written: the tag has to be a language tag, the
// content has to parse, and the translations have to fit their originals.
func (h *Handler) HandleLanguageUpload(w http.ResponseWriter, r *http.Request) error {
	dir := i18n.Dir()
	if dir == "" {
		web.SetFlashError(h.sm, r.Context(), "Für Sprachdateien ist kein Ordner eingerichtet")
		return h.redirect(w, r, "/admin/sprachen")
	}
	if err := r.ParseMultipartForm(2 * i18n.MaxFileBytes); err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden")
		return h.redirect(w, r, "/admin/sprachen")
	}

	file, header, err := r.FormFile("datei")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Bitte eine .json-Datei auswählen")
		return h.redirect(w, r, "/admin/sprachen")
	}
	defer file.Close()

	// The tag comes from the field when one is filled in, otherwise from the
	// file name: fr.json is French, and typing it twice helps nobody.
	code := locale.Normalise(strings.TrimSpace(r.FormValue("kuerzel")))
	if code == "" {
		code = locale.Normalise(strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)))
	}
	if !locale.Valid(code) {
		web.SetFlashError(h.sm, r.Context(),
			"Das Sprachkürzel ist keines: erwartet werden zwei oder drei Buchstaben wie fr, gern mit Region wie fr-CH")
		return h.redirect(w, r, "/admin/sprachen")
	}
	if code == i18n.Source {
		// Allowed on purpose: a de.json corrects our own wording. Said out loud
		// so nobody is surprised that the German changed.
		slog.Info("German wording is being overridden from disk")
	}

	// Read bounded, then check before anything is written: a file that would be
	// refused at load time must not sit in the folder looking installed.
	data, err := io.ReadAll(io.LimitReader(file, i18n.MaxFileBytes+1))
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden")
		return h.redirect(w, r, "/admin/sprachen")
	}
	if len(data) > i18n.MaxFileBytes {
		web.SetFlashError(h.sm, r.Context(), "Die Datei ist zu groß")
		return h.redirect(w, r, "/admin/sprachen")
	}
	msgs, err := i18n.Parse(data)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Die Sprachdatei wurde abgelehnt: %s", err))
		return h.redirect(w, r, "/admin/sprachen")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, code+".json"), data, 0o600); err != nil {
		return err
	}

	i18n.Reload()
	if err := h.templates.Reload(); err != nil {
		slog.Error("re-parse admin templates", "err", err)
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r,
		"%s eingespielt: %d Übersetzungen. Unter „Mein Konto“ lässt sie sich jetzt wählen.",
		locale.Name(code), len(msgs)))

	// A fassung of a language nobody installed works, but only its own
	// sentences are in that language and the rest comes out German. Said here,
	// where somebody can still do something about it, rather than left to be
	// discovered on a half-German screen.
	if base := i18n.Base(code); base != "" && base != i18n.Source && !i18n.Known(base) {
		web.SetFlashWarning(h.sm, r.Context(), web.Titlef(r,
			"Die Grundsprache %s fehlt. %s trägt nur seine eigenen Sätze; alles Übrige erscheint auf Deutsch, bis auch %s eingespielt ist.",
			locale.Name(base), code, base))
	}
	return h.redirect(w, r, "/admin/sprachen")
}

// HandleLanguageDelete removes a language file from the folder.
//
// Only a file: a language compiled into the binary is not deletable here, and
// saying so is better than a button that does nothing.
func (h *Handler) HandleLanguageDelete(w http.ResponseWriter, r *http.Request) error {
	code := locale.Normalise(r.PathValue("code"))
	dir := i18n.Dir()
	if dir == "" || !i18n.FromDisk(code) {
		web.SetFlashError(h.sm, r.Context(), "Diese Sprache gehört zum Programm und lässt sich nicht entfernen")
		return h.redirect(w, r, "/admin/sprachen")
	}

	if err := os.Remove(filepath.Join(dir, code+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	i18n.Reload()
	if err := h.templates.Reload(); err != nil {
		slog.Error("re-parse admin templates", "err", err)
		return err
	}

	// Anybody still set to that language now gets what their browser asks for,
	// because the middleware falls back on an unknown tag. Nothing to clean up.
	web.SetFlashSuccess(h.sm, r.Context(), "Sprachdatei entfernt")
	return h.redirect(w, r, "/admin/sprachen")
}
