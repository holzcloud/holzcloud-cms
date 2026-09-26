package admin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/oidc"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Signing in through OpenID Connect, without this server asking anybody.
//
// The sign-in form links to /admin/oidc/start. That sends the browser to the
// provider with a fresh state and nonce, and remembers both in a cookie of its
// own. The provider signs the person in and has the browser post an ID token
// back to /admin/oidc/callback. There the state is compared with the cookie,
// the token is checked against the key the operator put on disk
// (internal/oidc), and what it says about the person goes through the same
// funnel as a forward-auth sign-in: signInIdentity, which finds or creates the
// account and applies the rights the groups grant.
//
// Three things are different from every other form in the administration, and
// each has its reason.
//
// The callback is not under CSRF protection. The post comes from the
// provider's page, a cross-site form that cannot carry this installation's
// token; what stands in its place is the state, a random value this server
// made, which only the browser that started the sign-in holds, in a cookie
// only this origin can have set.
//
// That cookie is SameSite=None, because a Lax cookie is not sent with a
// cross-site POST — which is exactly what the callback is. It is __Host-
// prefixed, so a neighbouring subdomain cannot plant one (and with it a state
// of its choosing: a sign-in into the attacker's account, which is the attack
// the state exists against). It lives ten minutes and is deleted on the first
// callback whatever the outcome.
//
// The start is a plain link, not a form. The administration's CSP says
// form-action 'self', and browsers apply that to where a form's redirect ends
// up; a GET link that answers with a redirect to the provider is a navigation,
// which the CSP does not govern.

// oidcCookieName carries the state, the nonce and where to go afterwards.
const oidcCookieName = "__Host-holzcloud_oidc"

// oidcCookieLifetime is how long somebody may take at the provider.
const oidcCookieLifetime = 10 * time.Minute

// maxCallbackBytes bounds the posted form: an ID token and a state.
const maxCallbackBytes = 128 << 10

// The reasons an OpenID Connect sign-in is refused before an identity exists.
// Codes for the log and the protocol, like the forward-auth ones.
const (
	oidcRefuseProviderError = "provider_error"
	oidcRefuseNoState       = "no_state"
	oidcRefuseStateMismatch = "state_mismatch"
	oidcRefuseToken         = "token_refused"
	oidcRefuseNoUsername    = "no_username"
)

type viaKey struct{}

// withVia records which way in a sign-in came, for the protocol rows the
// shared funnel writes.
func withVia(r *http.Request, via string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), viaKey{}, via))
}

// viaOf is "sso" for forward authentication — what every row said before a
// second way in existed — and whatever withVia set otherwise.
func viaOf(r *http.Request) string {
	if v, ok := r.Context().Value(viaKey{}).(string); ok && v != "" {
		return v
	}
	return "sso"
}

// viaOIDC reports whether this session came through OpenID Connect, asked
// together with the switch like viaSSO.
func (h *Handler) viaOIDC(r *http.Request) bool {
	return h.cfg != nil && h.cfg.OIDCEnabled && h.sm.GetBool(r.Context(), auth.SessionKeyViaOIDC)
}

// viaIdentityProvider is either way in through the identity provider: the
// reading the second factor and the account screen need.
func (h *Handler) viaIdentityProvider(r *http.Request) bool {
	return h.viaSSO(r) || h.viaOIDC(r)
}

// oidcEnabled reports whether the switch is on.
func (h *Handler) oidcEnabled() bool {
	return h.cfg != nil && h.cfg.OIDCEnabled
}

// OIDCGuard ends every session OpenID Connect made once the switch is off.
//
// Such a session skipped the second factor on the provider's word, and an
// operator who switches the provider off because they stopped trusting it must
// not keep the administrator sessions it vouched for — the rule forward
// authentication learnt in the Phase 10 code review (WR-04).
func (h *Handler) OIDCGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.oidcEnabled() && h.sm.GetBool(r.Context(), auth.SessionKeyViaOIDC) {
			h.endSSOSession(withVia(r, "oidc"), "oidc_disabled")
		}
		next.ServeHTTP(w, r)
	})
}

// HandleOIDCStart sends the browser to the identity provider.
func (h *Handler) HandleOIDCStart(w http.ResponseWriter, r *http.Request) error {
	if !h.oidcEnabled() {
		http.NotFound(w, r)
		return nil
	}
	state, err := randomToken()
	if err != nil {
		return err
	}
	nonce, err := randomToken()
	if err != nil {
		return err
	}
	// Where the person was going, read now and carried in the cookie: the
	// callback arrives cross-site, without the session this value is in.
	back := auth.SafeReturn(h.sm.GetString(r.Context(), auth.SessionKeyReturnTo))

	http.SetCookie(w, &http.Cookie{
		Name:     oidcCookieName,
		Value:    state + "." + nonce + "." + base64.RawURLEncoding.EncodeToString([]byte(back)),
		Path:     "/",
		MaxAge:   int(oidcCookieLifetime.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	q := url.Values{}
	q.Set("response_type", "id_token")
	q.Set("response_mode", "form_post")
	q.Set("client_id", h.cfg.OIDCClientID)
	q.Set("redirect_uri", h.cfg.OIDCRedirectURL)
	q.Set("scope", h.cfg.OIDCScopes)
	q.Set("state", state)
	q.Set("nonce", nonce)
	target := h.cfg.OIDCAuthorizeURL
	if strings.Contains(target, "?") {
		target += "&" + q.Encode()
	} else {
		target += "?" + q.Encode()
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
	return nil
}

// HandleOIDCCallback takes the token the provider had the browser post.
func (h *Handler) HandleOIDCCallback(w http.ResponseWriter, r *http.Request) error {
	if !h.oidcEnabled() {
		http.NotFound(w, r)
		return nil
	}
	r = withVia(r, "oidc")
	w.Header().Set("Cache-Control", "no-store")

	// The cookie is read once and gone whatever follows: a state is good for
	// one answer.
	cookie, _ := r.Cookie(oidcCookieName)
	http.SetCookie(w, &http.Cookie{
		Name: oidcCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode,
	})

	r.Body = http.MaxBytesReader(w, r.Body, maxCallbackBytes)
	if err := r.ParseForm(); err != nil {
		return h.refuseOIDC(w, r, oidcRefuseToken, err)
	}
	if code := r.PostForm.Get("error"); code != "" {
		// The provider's own refusal: the person cancelled, or is not allowed
		// into this application. The code is the provider's and goes to the
		// log only; it is a short word from a fixed list, cut short anyway.
		if len(code) > 64 {
			code = code[:64]
		}
		return h.refuseOIDC(w, r, oidcRefuseProviderError, errors.New(code))
	}

	state, nonce, back, ok := splitOIDCCookie(cookie)
	if !ok {
		return h.refuseOIDC(w, r, oidcRefuseNoState, nil)
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("state")), []byte(state)) != 1 {
		return h.refuseOIDC(w, r, oidcRefuseStateMismatch, nil)
	}

	v := &oidc.Verifier{
		Issuer:   h.cfg.OIDCIssuer,
		ClientID: h.cfg.OIDCClientID,
		Keys:     h.cfg.OIDCKeys,
		Secret:   []byte(h.cfg.OIDCClientSecret),
	}
	claims, err := v.Verify(r.PostForm.Get("id_token"), nonce, oidc.Options{
		UsernameClaim: h.cfg.OIDCUsernameClaim,
		GroupsClaim:   h.cfg.OIDCGroupsClaim,
	})
	if err != nil {
		return h.refuseOIDC(w, r, oidcRefuseToken, err)
	}
	username := strings.TrimSpace(claims.Username)
	if username == "" {
		return h.refuseOIDC(w, r, oidcRefuseNoUsername, nil)
	}

	ident := &web.Identity{
		Username: username,
		Email:    strings.TrimSpace(claims.Email),
		Name:     strings.TrimSpace(claims.Name),
		Groups:   claims.Groups,
	}
	if !h.signInIdentity(r, ident) {
		// signInIdentity has logged and recorded why.
		web.SetFlashError(h.sm, r.Context(), web.T(r, "The identity provider signed you in, but this installation has no access for you."))
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil
	}
	h.sm.Put(r.Context(), auth.SessionKeyViaOIDC, true)
	http.Redirect(w, r, auth.SafeReturn(back), http.StatusSeeOther)
	return nil
}

// refuseOIDC records a sign-in that ended before there was an identity, and
// sends the person back to the form.
func (h *Handler) refuseOIDC(w http.ResponseWriter, r *http.Request, reason string, err error) error {
	var ip string
	if h.clientIP != nil {
		ip = h.clientIP.ClientIP(r)
	}
	attrs := []any{"reason", reason, "ip", ip}
	if err != nil {
		attrs = append(attrs, "err", err)
	}
	slog.Warn("oidc sign-in refused", attrs...)
	// Throttled like the forward-auth refusals: a page reloaded on the
	// provider's side would otherwise write a row each time.
	if h.refusalWorthRecording("oidc:"+ip, reason) {
		h.LogActivity(r, activity.Entry{
			Action:     activity.ActionAuthLoginFail,
			EntityType: "user",
			Metadata:   map[string]any{"via": "oidc", "reason": reason},
		})
	}
	web.SetFlashError(h.sm, r.Context(), web.T(r, "Signing in through the identity provider did not work. Please try again."))
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return nil
}

func splitOIDCCookie(c *http.Cookie) (state, nonce, back string, ok bool) {
	if c == nil {
		return "", "", "", false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", "", false
	}
	return parts[0], parts[1], string(b), true
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// OIDCButton is what the sign-in form needs to offer the second way in: the
// provider's name, or empty when the switch is off.
func OIDCButton(cfg *config.Config) string {
	if cfg == nil || !cfg.OIDCEnabled {
		return ""
	}
	return cfg.OIDCName
}
