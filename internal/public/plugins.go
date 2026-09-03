package public

import (
	"io"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// MaxPluginBodyBytes bounds what a plugin route may read from a request.
//
// A plugin answering a form post needs the body; it does not need an upload. A
// module that could pull a hundred megabytes into its linear memory would be a
// way to exhaust a small node with one request.
const MaxPluginBodyBytes = 256 << 10

// forwardedHeaders are the ones a plugin is given.
//
// An allow list rather than everything: the cookie header carries the session
// of a logged-in operator who happens to be looking at the public site, and
// authorization carries whatever a reverse proxy put there. Neither is any of a
// plugin's business, and passing them by default would make every plugin a
// place where they could leak.
var forwardedHeaders = []string{
	"Accept", "Accept-Language", "Content-Type", "Referer", "User-Agent",
}

// PluginMiddleware lets plugins answer a request before the built-in routes do.
//
// It sits inside the domain resolver, so the website is already known, and
// outside the mux, so a plugin can claim a path the core does not have. A
// plugin that declines — which is the normal case — costs one map lookup.
func (h *Handler) PluginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.plugins == nil {
			next.ServeHTTP(w, r)
			return
		}
		website := domain.WebsiteFromContext(r.Context())
		if website == nil {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		owner, claimed := h.plugins.RouteOwner(path, website.ID)
		wantsRequest := h.plugins.Active(plugin.HookRequest, website.ID)
		if !claimed && !wantsRequest {
			next.ServeHTTP(w, r)
			return
		}

		// The request travels with the call: a plugin that draws a page needs
		// the theme, and the theme needs to know which host it is answering on.
		ctx := withRequest(r.Context(), r)
		in := pluginRequest(r, website.ID)
		// The body only for a route the plugin owns: the request hook runs on
		// every page view, and reading a body there would mean buffering every
		// upload on the way past.
		if claimed && r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			body, err := io.ReadAll(io.LimitReader(r.Body, MaxPluginBodyBytes))
			if err == nil {
				in.Body = string(body)
			}
		}

		if claimed {
			if out, ok := h.plugins.HandleRoute(ctx, website.ID, in); ok {
				writePluginResponse(w, out)
				return
			}
			// A plugin that claims a path and then declines gets a 404 rather
			// than the core's page lookup: it owns the address, and letting the
			// request fall through would mean a slug could shadow it depending
			// on what a plugin happened to answer.
			_ = owner
			http.NotFound(w, r)
			return
		}

		if out, ok := h.plugins.HandleRequest(ctx, website.ID, in); ok {
			writePluginResponse(w, out)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// pluginRequest is what a plugin is told about a request, without a body.
func pluginRequest(r *http.Request, websiteID int64) plugin.RequestIn {
	in := plugin.RequestIn{
		WebsiteID: websiteID,
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Headers:   map[string]string{},
	}
	for _, name := range forwardedHeaders {
		if v := r.Header.Get(name); v != "" {
			in.Headers[name] = v
		}
	}
	return in
}

// writePluginResponse turns a plugin's answer into an HTTP response.
//
// What a plugin may set is deliberately narrow. Headers that decide how the
// browser treats the whole origin — the content policy, the frame rules, the
// cookies — are the host's, and a plugin that could set them could undo the
// protections every other page relies on.
func writePluginResponse(w http.ResponseWriter, out *plugin.RequestOut) {
	status := out.Status
	if status < 100 || status > 599 {
		status = http.StatusOK
	}

	if out.Location != "" {
		// Only within this site. An open redirect is a phishing tool: a link
		// that starts with the operator's own domain and lands somewhere else.
		if !safeLocation(out.Location) {
			http.Error(w, "Interner Fehler", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", out.Location)
		if status < 300 || status > 399 {
			status = http.StatusSeeOther
		}
	}

	for k, v := range out.Headers {
		if reservedHeader(k) {
			continue
		}
		w.Header().Set(k, v)
	}

	ct := out.ContentType
	if ct == "" {
		ct = "text/html; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)
	if out.Body != "" {
		_, _ = io.WriteString(w, out.Body)
	}
}

// safeLocation allows only a path on this site.
func safeLocation(loc string) bool {
	if !strings.HasPrefix(loc, "/") {
		return false
	}
	// "//evil.example" is a protocol-relative URL and leaves the site, even
	// though it starts with a slash.
	return !strings.HasPrefix(loc, "//")
}

// reservedHeader names the ones a plugin may not set.
func reservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-security-policy", "content-security-policy-report-only",
		"set-cookie", "strict-transport-security", "x-frame-options",
		"content-type", "location", "content-length", "transfer-encoding",
		"referrer-policy", "permissions-policy", "x-content-type-options",
		"access-control-allow-origin":
		return true
	}
	return false
}
