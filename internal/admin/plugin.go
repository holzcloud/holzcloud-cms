package admin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// PluginListData is the plugin screen.
type PluginListData struct {
	web.LayoutData
	Plugins  []PluginRow
	Websites []domain.Website
}

// PluginRow is one plugin with the websites it acts on resolved.
type PluginRow struct {
	plugin.Status
	OnWebsite map[int64]bool
	// Hooks and Permissions are shown as plain words: an operator installing
	// something that runs on their server should be able to see what it asked
	// for without opening the archive.
	Hooks       []string
	Permissions []string
}

// PluginScreenData wraps a plugin's own admin screen.
type PluginScreenData struct {
	web.LayoutData
	PluginID   string
	PluginName string
	Screen     web.SafeHTML
	Action     string
}

// HandlePluginList renders the plugin screen.
func (h *Handler) HandlePluginList(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	list, err := h.plugins.List(r.Context())
	if err != nil {
		return err
	}
	websites, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return err
	}

	rows := make([]PluginRow, 0, len(list))
	for _, p := range list {
		row := PluginRow{Status: p, OnWebsite: map[int64]bool{}}
		for _, id := range p.Websites {
			row.OnWebsite[id] = true
		}
		if p.Manifest != nil {
			row.Hooks = p.Manifest.Hooks
			row.Permissions = p.Manifest.Permissions
		}
		rows = append(rows, row)
	}

	data := PluginListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Plugins"),
		Plugins:    rows,
		Websites:   websites,
	}
	data.ActiveNav = "plugins"
	return web.RenderAdmin(w, h.templates, r, "plugin_list", data)
}

// HandlePluginUpload takes an archive and installs it.
func (h *Handler) HandlePluginUpload(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	// A plugin package is a module plus assets; the template limit is the
	// closest existing bound and generous enough for a Go guest.
	r.Body = http.MaxBytesReader(w, r.Body, plugin.MaxTotalBytes)

	file, _, err := r.FormFile("plugin")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/plugins")
	}
	defer file.Close()

	// zip needs to seek, so the archive is read into memory. Bounded by the
	// reader above, which is what keeps the server out of swap.
	data, err := io.ReadAll(file)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden")
		return h.redirect(w, r, "/admin/plugins")
	}

	m, err := h.plugins.Install(r.Context(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Einspielen fehlgeschlagen: %s", err))
		return h.redirect(w, r, "/admin/plugins")
	}
	web.SetFlashSuccess(h.sm, r.Context(), fmt.Sprintf(
		"%s %s eingespielt. Es ist noch ausgeschaltet — schalte es ein und wähle die Websites.",
		m.Name, m.Version))
	return h.redirect(w, r, "/admin/plugins")
}

// HandlePluginEnable switches a plugin on or off.
func (h *Handler) HandlePluginEnable(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	id := r.PathValue("id")
	on := r.FormValue("an") == "1"

	var err error
	if on {
		err = h.plugins.Enable(r.Context(), id)
	} else {
		err = h.plugins.Disable(r.Context(), id)
	}
	switch {
	case errors.Is(err, plugin.ErrNotFound):
		return h.notFound(w, r)
	case err != nil && on:
		// Enable reports the reason the module did not come up. Showing a
		// green tick for something that is not running would be the one
		// outcome worse than the failure.
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Einschalten fehlgeschlagen: %s", err))
	case err != nil:
		return err
	case on:
		web.SetFlashSuccess(h.sm, r.Context(), "Eingeschaltet.")
	default:
		web.SetFlashSuccess(h.sm, r.Context(), "Ausgeschaltet.")
	}
	return h.redirect(w, r, "/admin/plugins")
}

// HandlePluginWebsites changes which sites a plugin acts on.
func (h *Handler) HandlePluginWebsites(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	var sites []int64
	for _, v := range r.Form["website"] {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			sites = append(sites, id)
		}
	}
	if err := h.plugins.SetWebsites(r.Context(), r.PathValue("id"), sites); err != nil {
		if errors.Is(err, plugin.ErrNotFound) {
			return h.notFound(w, r)
		}
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Zuordnung gespeichert.")
	return h.redirect(w, r, "/admin/plugins")
}

// HandlePluginRemove deletes a plugin and everything belonging to it.
func (h *Handler) HandlePluginRemove(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	id := r.PathValue("id")
	// Typing the name is the confirmation. A plugin takes its stored data with
	// it, and that is not something to undo with the back button.
	if r.FormValue("bestaetigung") != id {
		web.SetFlashError(h.sm, r.Context(),
			"Zum Entfernen die Kennung des Plugins eintippen.")
		return h.redirect(w, r, "/admin/plugins")
	}
	if err := h.plugins.Remove(r.Context(), id); err != nil {
		if errors.Is(err, plugin.ErrNotFound) {
			return h.notFound(w, r)
		}
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Entfernt, samt seiner Daten.")
	return h.redirect(w, r, "/admin/plugins")
}

// HandlePluginScreen renders a plugin's own admin screen.
//
// The plugin gets the form and the method and returns HTML. It never sees the
// session, the CSRF token or the operator's address — the host has already
// checked all three, and handing them over would make every plugin a place
// where they could leak.
func (h *Handler) HandlePluginScreen(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	id := r.PathValue("id")
	st, err := h.plugins.Get(r.Context(), id)
	if err != nil || st.Manifest == nil || st.Manifest.Admin == nil {
		return h.notFound(w, r)
	}
	if !st.Running {
		web.SetFlashError(h.sm, r.Context(),
			"Das Plugin läuft nicht: "+firstLine(st.LastError))
		return h.redirect(w, r, "/admin/plugins")
	}
	if st.Manifest.Admin.AdminOnly && h.sm.GetString(r.Context(), "user_role") != "admin" {
		return h.notFound(w, r)
	}

	var websiteID int64
	action := "/admin/plugins/" + id + "/bildschirm"
	if raw := r.PathValue("websiteID"); raw != "" {
		websiteID, _ = strconv.ParseInt(raw, 10, 64)
		action = fmt.Sprintf("/admin/websites/%d/plugins/%s", websiteID, id)
	}

	in := plugin.AdminIn{
		WebsiteID: websiteID, Method: r.Method,
		Path: r.URL.Path, Query: r.URL.RawQuery,
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		in.Form = map[string][]string{}
		for k, v := range r.Form {
			// The CSRF token is the host's business and no plugin needs it.
			if strings.HasPrefix(k, "gorilla.") {
				continue
			}
			in.Form[k] = v
		}
	}

	out, err := h.plugins.Admin(r.Context(), id, in)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Das Plugin meldete einen Fehler: %s", err))
		return h.redirect(w, r, "/admin/plugins")
	}
	if out.Flash != "" {
		if out.FlashError {
			web.SetFlashError(h.sm, r.Context(), out.Flash)
		} else {
			web.SetFlashSuccess(h.sm, r.Context(), out.Flash)
		}
	}
	if out.Redirect != "" {
		// Only within this plugin's own screen: a plugin that could send the
		// operator anywhere could send them to a form on another site that
		// looks like this one. A query is allowed through, because that is how
		// a screen with more than one view says which one to show — and a query
		// cannot change which page is reached.
		target := action
		if strings.HasPrefix(out.Redirect, "?") && len(out.Redirect) < 300 {
			target += out.Redirect
		}
		return h.redirect(w, r, target)
	}

	if d := out.Download; d != nil && d.Body != "" {
		return writePluginDownload(w, *d)
	}

	title := out.Title
	if title == "" {
		title = st.Name
	}
	data := PluginScreenData{
		LayoutData: web.NewLayoutData(r, h.sm, title),
		PluginID:   id,
		PluginName: st.Name,
		// Sanitised, not trusted: the same policy as page content, so a plugin
		// cannot put a script into the admin of the person who installed it.
		// The session token goes in afterwards, so a plugin's form posts back
		// without the plugin ever seeing it.
		Screen: web.WithCSRFToken(web.SanitizeAdminHTML(out.HTML), web.CSRFTokenFromRequest(r)),
		Action: action,
	}
	data.ActiveNav = "plugins"
	if websiteID > 0 {
		if ws, err := h.domains.GetWebsite(r.Context(), websiteID); err == nil && ws != nil {
			data.CurrentWebsite = ws
		}
	}
	return web.RenderAdmin(w, h.templates, r, "plugin_screen", data)
}

// HandlePluginAsset serves a file a plugin shipped.
func (h *Handler) HandlePluginAsset(w http.ResponseWriter, r *http.Request) error {
	if h.plugins == nil {
		return h.notFound(w, r)
	}
	path := h.plugins.AssetPath(r.PathValue("id"), r.PathValue("path"))
	if path == "" {
		return h.notFound(w, r)
	}
	// Long-lived: a plugin's assets change only when the plugin is replaced,
	// and a replacement writes a new directory.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, path)
	return nil
}

// notFound answers a request for something that is not there.
//
// A plugin screen that does not exist and one the operator may not see give the
// same answer on purpose: telling an editor that an admin-only screen exists is
// telling them something they were not meant to know.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) error {
	http.Error(w, "Seite nicht gefunden", http.StatusNotFound)
	return nil
}

// firstLine keeps a flash message to one line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "kein Grund vermerkt"
	}
	return s
}

// pluginDownloadTypes are the content types a plugin may hand over.
//
// The list is short on purpose. A plugin that could answer with text/html from
// the administration's own address could put a page there that looks like the
// administration — same origin, same padlock, and a form asking for the
// password. Anything not on the list is served as bytes to save, which is
// harmless whatever is in it.
var pluginDownloadTypes = map[string]bool{
	"text/csv":         true,
	"text/plain":       true,
	"text/markdown":    true,
	"application/json": true,
}

// maxPluginDownload bounds what a plugin may hand over in one go. A plugin runs
// inside a memory limit of its own, but the answer is assembled here, and an
// export that grew without anyone deciding it should is better refused than
// held in memory.
const maxPluginDownload = 8 << 20

// writePluginDownload serves a plugin's file as an attachment.
//
// Always an attachment, never inline: the point of the content-type list above
// is that nothing a plugin produces gets rendered by the browser on this
// origin, and Content-Disposition is what makes that hold even for a type that
// is on the list.
func writePluginDownload(w http.ResponseWriter, d plugin.Download) error {
	if len(d.Body) > maxPluginDownload {
		return fmt.Errorf("das Plugin wollte %d Bytes übergeben, erlaubt sind %d", len(d.Body), maxPluginDownload)
	}

	contentType := "application/octet-stream"
	if pluginDownloadTypes[strings.ToLower(strings.TrimSpace(d.ContentType))] {
		contentType = strings.ToLower(strings.TrimSpace(d.ContentType)) + "; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	// Ohne nosniff könnte ein Browser den Inhalt anders deuten als die
	// Kopfzeile sagt, und die Liste oben wäre eine Empfehlung statt einer
	// Schranke.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(d.Filename)+`"`)
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, d.Body)
	return err
}

// safeDownloadName reduces a plugin's filename to a plain one.
//
// Path separators, quotes and control characters go: the name lands inside a
// quoted header value and, on the other end, in somebody's download folder.
// An empty result becomes a name rather than nothing, because a download with
// no name is saved as the last segment of the address, which here is the
// plugin's screen.
func safeDownloadName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f:
		case r == '/' || r == '\\' || r == '"' || r == ';':
		default:
			b.WriteRune(r)
		}
		if b.Len() >= 100 {
			break
		}
	}
	out := strings.TrimLeft(b.String(), ".")
	if out == "" {
		return "export.txt"
	}
	return out
}
