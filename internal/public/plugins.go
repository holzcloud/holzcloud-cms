package public

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/web"
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
			// A multipart submission for a plugin that may take attachments
			// goes the other way: the host parses it, keeps the files out of
			// the plugin's reach, and hands over the fields as an ordinary
			// encoded form — so a plugin written before attachments existed
			// reads exactly what it read before.
			if body, keeper, took := h.takeAttachments(r, website.ID, owner,
				h.plugins.Allows(owner, plugin.PermAttach)); took {
				in.Body = body
				in.Files = keeper.List()
				ctx = plugin.WithFiles(ctx, keeper)
			} else {
				body, err := io.ReadAll(io.LimitReader(r.Body, MaxPluginBodyBytes))
				if err == nil {
					in.Body = string(body)
				}
			}
		}

		if claimed {
			out, ok, err := h.plugins.HandleRoute(ctx, website.ID, in)
			if ok {
				writePluginResponse(w, out)
				return
			}
			// A plugin that is busy or held back was not asked. The page is
			// there, it is the plugin that cannot answer it now, so the visitor
			// is told to come back and when, not that the page does not exist.
			// Plain status text, as the 500 of the same kind elsewhere: this is
			// a visitor's answer and not an operator sentence, and the reason
			// lies in the plugin's own error line.
			var unavailable *plugin.UnavailableError
			if errors.As(err, &unavailable) {
				retry := int((unavailable.RetryAfter + time.Second - 1) / time.Second)
				w.Header().Set("Retry-After", strconv.Itoa(max(retry, 1)))
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
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
		// The language this request is answered in, so a plugin's answer to a
		// visitor is in the visitor's language and not the website's first.
		Lang: i18n.Lang(r.Context()),
	}
	for _, name := range forwardedHeaders {
		if v := r.Header.Get(name); v != "" {
			in.Headers[name] = v
		}
	}
	return in
}

// pluginLocation checks a plugin's Location. The path must be local by
// web.LocalPath. A backslash after the path (in the query or the fragment)
// cannot change the origin, and a plugin that echoes a visitor's input there
// would otherwise turn a working redirect into a 500, so it is percent-encoded
// instead of refused. A control character anywhere is still refused: it would
// carry a line break into the header.
func pluginLocation(loc string) (string, bool) {
	path, rest := loc, ""
	if i := strings.IndexAny(loc, "?#"); i >= 0 {
		path, rest = loc[:i], loc[i:]
	}
	checked, ok := web.LocalPath(path)
	if !ok {
		return "", false
	}
	for i := 0; i < len(rest); i++ {
		if c := rest[i]; c < 0x20 || c == 0x7f {
			return "", false
		}
	}
	return checked + strings.ReplaceAll(rest, `\`, "%5C"), true
}

// writePluginResponse turns a plugin's answer into an HTTP response.
//
// What a plugin may set is deliberately narrow. Headers that decide how the
// browser treats the whole origin — the content policy, the frame rules, the
// cookies — are the host's, and a plugin that could set them could undo the
// protections every other page relies on.
func writePluginResponse(w http.ResponseWriter, out *plugin.RequestOut) {
	// 200 to 599: an informational status is not an answer, and a 103 would
	// carry a plugin's headers to the browser before the real one.
	status := out.Status
	if status < 200 || status > 599 {
		status = http.StatusOK
	}

	if out.Location != "" {
		// Only within this site, by the local-path rule (web.LocalPath). An
		// open redirect is a phishing tool: a link that starts with the
		// operator's own domain and lands somewhere else.
		loc, ok := pluginLocation(out.Location)
		if !ok {
			// The status text of the standard library, as the other public
			// handlers give it: this is a visitor's answer, not a sentence of
			// the operator's catalogue.
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", loc)
		if status < 300 || status > 399 {
			status = http.StatusSeeOther
		}
	}

	for k, v := range out.Headers {
		if !pluginHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		w.Header().Set(k, v)
	}

	// The one rule for every answer, redirects included: no script runs on a
	// page a plugin wrote. Before WriteHeader, or the header is gone.
	web.ForbidScript(w)
	w.Header().Set("Content-Type", pluginContentType(out.ContentType))
	w.WriteHeader(status)
	if out.Body != "" {
		_, _ = io.WriteString(w, out.Body)
	}
}

// pluginHeaders is the closed list of headers a plugin may set.
//
// An allow list, not a deny list: the deny list that stood here missed Refresh,
// which redirects without passing the Location check, and Link, which makes the
// browser preload a script (W-2 of the phase 35 security report), and the next
// header a browser learns to obey would have been missed the same way. What is
// on the list describes the answer (its caching, language, validators and
// filename) and decides nothing about how the browser treats the origin. Names
// are canonicalised first, so case does not matter.
var pluginHeaders = map[string]bool{
	"Cache-Control": true, "Content-Disposition": true, "Content-Language": true,
	"Etag": true, "Expires": true, "Last-Modified": true, "Retry-After": true,
	"Vary": true, "X-Robots-Tag": true,
}

// pluginMediaTypes is the closed list of media types a plugin may answer with.
//
// The media type decides what a browser does with the body. text/javascript
// satisfies script-src 'self' on a website's own origin, and that origin shares
// its session with the admin, so a plugin that could choose its type could
// choose to run script there (finding M2 of the phase 35 security report).
// Everything a plugin of this CMS has a reason to send is on the list: a page, a
// feed, data, a calendar, a table. Anything else, SVG and CSS included, goes
// out as plain text.
var pluginMediaTypes = map[string]bool{
	"text/html": true, "text/plain": true, "text/csv": true, "text/calendar": true,
	"text/xml": true, "application/json": true, "application/xml": true,
	"application/rss+xml": true, "application/atom+xml": true,
}

// pluginContentType turns the type a plugin asked for into the one that is
// sent. The parsed media type is compared with the list, never a prefix or a
// substring: "text/html-x" and "text/html, text/javascript" are not text/html.
func pluginContentType(asked string) string {
	const fallback = "text/plain; charset=utf-8"
	if asked == "" {
		return "text/html; charset=utf-8"
	}
	media, params, err := mime.ParseMediaType(asked)
	if err != nil || !pluginMediaTypes[media] {
		return fallback
	}
	if ct := mime.FormatMediaType(media, params); ct != "" {
		return ct
	}
	return fallback
}
