package admin

import (
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// accountWebsites says in a few words what this account may enter.
func (h *Handler) accountWebsites(r *http.Request, userID int64, role string) string {
	if role == "admin" {
		return web.T(r, "every website")
	}
	rights, err := h.users.Assignment(r.Context(), userID)
	if err != nil || !rights.Limited() {
		return web.T(r, "every website")
	}
	var names []string
	for _, id := range rights.Websites {
		if ws, err := h.domains.GetWebsite(r.Context(), id); err == nil && ws != nil {
			names = append(names, ws.Name)
		}
	}
	if len(names) == 0 {
		return web.T(r, "no website")
	}
	return strings.Join(names, ", ")
}

// deviceLabel turns what a browser says about itself into something a person
// recognises: "Safari on iPhone". What it cannot place it leaves as "a browser"
// rather than printing forty characters of version numbers.
func deviceLabel(r *http.Request, ua string) string {
	if ua == "" {
		return web.T(r, "An earlier sign-in")
	}
	browser := ""
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "Chromium/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	system := ""
	switch {
	case strings.Contains(ua, "iPhone"):
		system = "iPhone"
	case strings.Contains(ua, "iPad"):
		system = "iPad"
	case strings.Contains(ua, "Android"):
		system = "Android"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		system = "Mac"
	case strings.Contains(ua, "Windows"):
		system = "Windows"
	case strings.Contains(ua, "Linux"):
		system = "Linux"
	}
	switch {
	case browser != "" && system != "":
		return web.Titlef(r, "%s on %s", browser, system)
	case browser != "":
		return browser
	case system != "":
		return system
	}
	return web.T(r, "a browser")
}

// HandleAccountSessionEnd signs this person out on one other device.
func (h *Handler) HandleAccountSessionEnd(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	id := r.PathValue("session")
	// The current session is not ended from here: that is "Sign out", and a
	// person who presses it on their own row should not lose the page they are
	// reading without being told.
	if id == auth.SessionID(h.sm.Token(r.Context())) {
		web.SetFlashWarning(h.sm, r.Context(), "That is this device. Sign out with the button in the corner.")
		return h.redirect(w, r, "/admin/konto")
	}
	found, err := auth.DestroyUserSession(r.Context(), h.sm, *userID, id)
	if err != nil {
		return err
	}
	if found {
		web.SetFlashSuccess(h.sm, r.Context(), "Signed out on that device")
	} else {
		web.SetFlashWarning(h.sm, r.Context(), "That session had already ended.")
	}
	return h.redirect(w, r, "/admin/konto")
}

// HandleAccountSessionsEnd signs this person out everywhere but here.
func (h *Handler) HandleAccountSessionsEnd(w http.ResponseWriter, r *http.Request) error {
	userID := h.currentUserID(r)
	if userID == nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	if err := auth.DestroyUserSessions(r.Context(), h.sm, *userID, h.sm.Token(r.Context())); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Signed out on every other device")
	return h.redirect(w, r, "/admin/konto")
}
