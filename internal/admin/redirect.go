package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// RedirectListData is the redirect overview.
type RedirectListData struct {
	web.LayoutData
	web.FormState
	WebsiteID int64
	Redirects []page.Redirect
	// Broken are the internal links that point nowhere. They sit on this screen
	// because a redirect is one of the two ways to fix one, and the other —
	// editing the page — is one click from here either way.
	Broken []BrokenLink
	From   string
	To     string
}

// HandleRedirectList shows the redirects of a website and the form to add one.
//
// The hit counts are the useful part: they show which old address still brings
// traffic, which is exactly what someone migrating an existing site needs.
func (h *Handler) HandleRedirectList(w http.ResponseWriter, r *http.Request) error {
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

	if r.Method == http.MethodPost {
		return h.handleRedirectAdd(w, r, websiteID, ws.Name)
	}

	data, err := h.redirectListData(r, websiteID, ws.Name)
	if err != nil {
		return err
	}
	// A broken link fills the form rather than submitting anything: the target
	// is a judgement call, so the operator sees the address filled in and
	// chooses where it should go.
	data.From = normalizeRedirectPath(r.URL.Query().Get("from"))
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "redirect_list", data)
}

func (h *Handler) redirectListData(r *http.Request, websiteID int64, websiteName string) (RedirectListData, error) {
	data := RedirectListData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Weiterleitungen – %s", websiteName)),
		FormState:  web.NewFormState(),
		WebsiteID:  websiteID,
	}
	data.ActiveNav = "redirects"

	list, err := h.pages.ListRedirects(r.Context(), websiteID)
	if err != nil {
		return data, err
	}
	data.Redirects = list

	broken, err := h.checkInternalLinks(r, websiteID)
	if err != nil {
		return data, err
	}
	data.Broken = broken
	return data, nil
}

// normalizeRedirectPath makes a hand-typed address comparable with r.URL.Path.
//
// Someone migrating a site will paste "https://alt.de/kontakt.html" or
// "kontakt.html"; both have to end up as "/kontakt.html", or the lookup on the
// 404 path silently never matches.
func normalizeRedirectPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			raw = rest[j:]
		} else {
			raw = "/"
		}
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	// Query and fragment are not part of what the router matches on.
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	if len(raw) > 1 {
		raw = strings.TrimSuffix(raw, "/")
	}
	return raw
}

func (h *Handler) handleRedirectAdd(w http.ResponseWriter, r *http.Request, websiteID int64, websiteName string) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	from := normalizeRedirectPath(r.FormValue("from_path"))
	to := normalizeRedirectPath(r.FormValue("to_path"))

	data, err := h.redirectListData(r, websiteID, websiteName)
	if err != nil {
		return err
	}
	data.From, data.To = from, to
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	data.CurrentWebsite = ws

	if from == "" || from == "/" {
		data.Errors.Add("from_path", "Bitte die alte Adresse angeben.")
	}
	if to == "" {
		data.Errors.Add("to_path", "Bitte das Ziel angeben.")
	}
	if from != "" && from == to {
		// Otherwise the browser follows the redirect back to itself forever.
		data.Errors.Add("to_path", "Ziel und Quelle dürfen nicht gleich sein.")
	}
	if data.Errors.Any() {
		return web.RenderFormError(w, h.templates, r, "redirect_list", data)
	}

	code := 301
	if r.FormValue("code") == "302" {
		code = 302
	}
	if err := h.pages.AddRedirect(r.Context(), websiteID, from, to, code); err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(), "Weiterleitung gespeichert")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/redirects", websiteID))
}

// HandleRedirectDelete removes one redirect.
func (h *Handler) HandleRedirectDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	id, err := strconv.ParseInt(r.PathValue("redirectID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	if err := h.pages.DeleteRedirect(r.Context(), websiteID, id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Weiterleitung gelöscht")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/redirects", websiteID))
}
