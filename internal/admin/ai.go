package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The screen where an operator connects their own assistant.
//
// Everything here is about one secret and what may be done with it. The secret
// is shown once — the same rule as the invitation link, for the same reason:
// what the server keeps is a hash, so a second look is not something it could
// offer even if it wanted to.

// sessionNewKey is where the freshly issued secret waits for exactly one page
// load. It goes through the session rather than into the redirect address: a
// key in a URL ends up in the browser history, in a proxy log and in the
// referrer of the next request.
const (
	sessionNewKey     = "ai_new_key"
	sessionNewKeyName = "ai_new_key_name"
)

// AIKeysData is the key screen.
type AIKeysData struct {
	web.LayoutData
	Keys     []ai.Token
	Websites []domain.Website
	// NewKey is set for one page load after a key was created.
	NewKey     string
	NewKeyName string
	// Endpoint is the address the assistant is pointed at, spelled out with the
	// host the operator is currently looking at — the one thing they would
	// otherwise have to assemble by hand and get wrong.
	Endpoint string
}

// WebsiteName resolves a key's website for display.
func (d AIKeysData) WebsiteName(id int64) string {
	for _, w := range d.Websites {
		if w.ID == id {
			return w.Name
		}
	}
	return fmt.Sprintf("Website %d", id)
}

// HandleAIKeys lists the keys.
func (h *Handler) HandleAIKeys(w http.ResponseWriter, r *http.Request) error {
	if h.aiTokens == nil {
		http.NotFound(w, r)
		return nil
	}
	keys, err := h.aiTokens.List(r.Context())
	if err != nil {
		return err
	}
	sites, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return err
	}

	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}

	data := AIKeysData{
		LayoutData: web.NewLayoutData(r, h.sm, "KI-Zugang"),
		Keys:       keys,
		Websites:   sites,
		NewKey:     h.sm.PopString(r.Context(), sessionNewKey),
		NewKeyName: h.sm.PopString(r.Context(), sessionNewKeyName),
		Endpoint:   scheme + "://" + r.Host + "/ai",
	}
	data.ActiveNav = "ai"
	return web.RenderAdmin(w, h.templates, r, "ai_keys", data)
}

// HandleAIKeyCreate issues a key.
func (h *Handler) HandleAIKeyCreate(w http.ResponseWriter, r *http.Request) error {
	if h.aiTokens == nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	websiteID, _ := strconv.ParseInt(r.FormValue("website"), 10, 64)
	canWrite := r.FormValue("rechte") == "schreiben"

	var lifetime time.Duration
	if days, _ := strconv.Atoi(r.FormValue("tage")); days > 0 {
		lifetime = time.Duration(days) * 24 * time.Hour
	}

	secret, token, err := h.aiTokens.Issue(r.Context(), r.FormValue("name"), websiteID, canWrite, lifetime)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), err.Error())
		return h.redirect(w, r, "/admin/ai")
	}

	h.sm.Put(r.Context(), sessionNewKey, secret)
	h.sm.Put(r.Context(), sessionNewKeyName, token.Name)
	return h.redirect(w, r, "/admin/ai")
}

// HandleAIKeyRevoke withdraws a key.
func (h *Handler) HandleAIKeyRevoke(w http.ResponseWriter, r *http.Request) error {
	if h.aiTokens == nil {
		http.NotFound(w, r)
		return nil
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := h.aiTokens.Revoke(r.Context(), id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(),
		"Der Schlüssel gilt nicht mehr. Ein Assistent, der ihn noch benutzt, wird abgewiesen.")
	return h.redirect(w, r, "/admin/ai")
}
