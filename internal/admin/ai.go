package admin

import (
	"errors"
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

// aIKeysData is the key screen.
type aIKeysData struct {
	web.LayoutData
	Keys     []ai.Token
	Websites []domain.Website
	// WebsiteLabels is what each key's website is called, worked out where a
	// request exists. A row whose website is gone gets a stand-in sentence, and
	// that sentence has to be built with a translator — see WebsiteName.
	WebsiteLabels map[int64]string
	// NewKey is set for one page load after a key was created.
	NewKey     string
	NewKeyName string
	// Endpoint is the address the assistant is pointed at, spelled out with the
	// host the operator is currently looking at — the one thing they would
	// otherwise have to assemble by hand and get wrong.
	Endpoint string
}

// WebsiteName resolves a key's website for display.
//
// It reads a map the handler filled in rather than building anything. It used
// to end with fmt.Sprintf("Website %d", id) for a key whose website has been
// deleted, and that sentence was invisible twice over: a method on the template
// data has no request to translate against, and a sentence assembled in Go is
// not something the collector can see at all — the gate reported neither open
// nor orphaned about it.
func (d aIKeysData) WebsiteName(id int64) string { return d.WebsiteLabels[id] }

// websiteLabels names every website a key points at, in the language of
// whoever is reading.
//
// Built here because here there is a request. A key whose website has been
// deleted still has to say something, and "Website 7" is a sentence like any
// other.
func websiteLabels(r *http.Request, keys []ai.Token, sites []domain.Website) map[int64]string {
	byID := make(map[int64]string, len(sites))
	for _, w := range sites {
		byID[w.ID] = w.Name
	}
	out := make(map[int64]string, len(keys))
	for _, k := range keys {
		// Zero means "every website", which the template answers on its own.
		if k.WebsiteID == 0 {
			continue
		}
		id := k.WebsiteID
		if name, ok := byID[id]; ok {
			out[id] = name
			continue
		}
		out[id] = web.Titlef(r, "Website %d", id)
	}
	return out
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

	data := aIKeysData{
		LayoutData:    web.NewLayoutData(r, h.sm, "AI access"),
		Keys:          keys,
		Websites:      sites,
		WebsiteLabels: websiteLabels(r, keys, sites),
		NewKey:        h.sm.PopString(r.Context(), sessionNewKey),
		NewKeyName:    h.sm.PopString(r.Context(), sessionNewKeyName),
		Endpoint:      scheme + "://" + r.Host + "/ai",
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
	if errors.Is(err, ai.ErrNameMissing) {
		// The one refusal here that an operator can act on, so the one that
		// goes through the catalogue. Everything else Issue can fail with is a
		// database fault, and a database fault is not a sentence anybody has
		// written.
		web.SetFlashError(h.sm, r.Context(), "The key needs a name.")
		return h.redirect(w, r, "/admin/ai")
	}
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
		"The key is no longer valid. An assistant still using it is turned away.")
	return h.redirect(w, r, "/admin/ai")
}
