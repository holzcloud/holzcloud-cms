package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The page where an administrator lets an assistant in.
//
// An assistant that signs in with OAuth — Claude, ChatGPT — sends the operator
// here with its request in the address. The page says who is asking and where
// the browser goes afterwards, and lets the operator choose what the key may
// do. Nothing is issued until the button is pressed, and the button sits behind
// the password prompt, like issuing a key by hand does.

// aiConnectData is the consent page.
type aiConnectData struct {
	web.LayoutData
	Request  *ai.AuthRequest
	Params   url.Values
	Websites []domain.Website
	// Problem is set when the request cannot be answered. It is shown as it
	// is: it names a protocol detail, for whoever set the assistant up.
	Problem string
}

// SetOAuth enables the consent page.
func (h *Handler) SetOAuth(o *ai.OAuth) { h.oauth = o }

// HandleAIConnect shows the consent page and records the answer.
func (h *Handler) HandleAIConnect(w http.ResponseWriter, r *http.Request) error {
	if h.oauth == nil || h.aiTokens == nil {
		http.NotFound(w, r)
		return nil
	}
	var params url.Values
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return err
		}
		params = r.PostForm
	} else {
		params = r.URL.Query()
	}

	req, err := h.oauth.ParseAuthRequest(r.Context(), params)
	if errors.Is(err, ai.ErrAuthRequest) {
		return h.renderConnect(w, r, aiConnectData{Problem: err.Error()}, http.StatusBadRequest)
	}
	if err != nil {
		return err
	}

	if r.Method != http.MethodPost {
		// The password first, and then this page again with everything still
		// in the address. The POST is guarded as well; asking here saves the
		// operator pressing the button twice.
		if !auth.Elevated(h.sm, r.Context()) {
			http.Redirect(w, r, auth.ConfirmPath+"?weiter="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return nil
		}
		sites, err := h.domains.ListWebsites(r.Context())
		if err != nil {
			return err
		}
		web.AllowFormActionTo(w, req.RedirectOrigin())
		return h.renderConnect(w, r, aiConnectData{Request: req, Params: req.Values(), Websites: sites}, http.StatusOK)
	}

	websiteID, _ := strconv.ParseInt(r.PostForm.Get("website"), 10, 64)
	if websiteID > 0 {
		if _, err := h.domains.GetWebsite(r.Context(), websiteID); err != nil {
			http.Error(w, "unknown website", http.StatusBadRequest)
			return nil
		}
	}
	canWrite := r.PostForm.Get("rechte") == "schreiben"
	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)

	target, err := h.oauth.Grant(r.Context(), req, websiteID, canWrite, userID)
	if err != nil {
		return err
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
	return nil
}

func (h *Handler) renderConnect(w http.ResponseWriter, r *http.Request, data aiConnectData, status int) error {
	data.LayoutData = web.NewLayoutData(r, h.sm, "Connect an assistant")
	data.ActiveNav = "ai"
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	return web.RenderAdmin(w, h.templates, r, "ai_connect", data)
}

// OAuthWindow keeps the sign-in window of an assistant connected to the app
// that opened it, for as long as that sign-in is under way.
//
// The chain is the authorisation address, the consent page, and — when the
// operator is not signed in yet — the sign-in, the second factor and the
// password prompt in between. Every response in it gets
// Cross-Origin-Opener-Policy: unsafe-none; everything else keeps same-origin.
func OAuthWindow(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			switch {
			case strings.HasPrefix(p, "/oauth/"), p == ai.ConsentPath:
				web.AllowOpener(w)
			case p == "/admin/login", p == auth.VerifyPath, p == auth.ConfirmPath:
				if auth.ReturnPending(sm, r.Context(), ai.ConsentPath) || strings.Contains(r.URL.RawQuery, url.QueryEscape(ai.ConsentPath)) {
					web.AllowOpener(w)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
