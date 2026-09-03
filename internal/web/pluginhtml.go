package web

import (
	"html/template"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// SafeHTML is markup that has been through the sanitiser and may be placed in
// a page. The named type exists so nothing else can be assigned to the field
// by accident.
type SafeHTML = template.HTML

// adminPolicy is what a plugin may put on an admin screen.
//
// A plugin runs on the operator's server, so it is already trusted with a great
// deal — but not with the admin's session. A script in a plugin's screen runs
// in the origin that holds the login cookie and the CSRF token, so a plugin
// with one bad dependency would become a way to take over the installation.
// The screen is therefore sanitised exactly like a visitor's comment would be.
//
// More is allowed than in page content, because this is a control surface and
// not prose: a plugin needs forms to be useful at all. What it does not get is
// anything that executes, loads from elsewhere, or leaves the page.
var adminPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// The elements a settings screen is made of.
	p.AllowElements("form", "fieldset", "legend", "label", "input", "select",
		"option", "optgroup", "textarea", "button", "table", "thead", "tbody",
		"tfoot", "tr", "th", "td", "caption", "details", "summary", "figure",
		"figcaption", "small", "time", "output", "progress", "meter")

	// Classes are allowed here, unlike in page content: a plugin should be
	// able to use the admin's own styles rather than reinvent them, and the
	// admin stylesheet is the operator's, not the plugin's.
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("id", "for", "name", "value", "placeholder", "type", "checked",
		"selected", "disabled", "readonly", "required", "rows", "cols", "step",
		"min", "max", "maxlength", "multiple", "size", "autocomplete").
		OnElements("input", "select", "option", "textarea", "button", "label",
			"form", "fieldset", "optgroup", "output", "progress", "meter")
	p.AllowAttrs("colspan", "rowspan", "scope").OnElements("th", "td")
	p.AllowAttrs("datetime").OnElements("time")
	p.AllowAttrs("open").OnElements("details")

	// A form may only post back to the host, which then routes it to the
	// plugin that drew it. Without this a plugin could draw a login form that
	// posts the operator's password to another server.
	p.AllowAttrs("method").Matching(bluemonday.Paragraph).OnElements("form")
	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("http", "https", "mailto")

	// Images from this server only, for the same reason a theme may not load
	// one from elsewhere: a remote image is a request that leaves the machine
	// on every page view.
	p.AllowImages()
	p.RequireNoFollowOnLinks(true)

	return p
}()

// SanitizeAdminHTML cleans what a plugin returned for its admin screen.
//
// Deliberately not optional and not configurable. A plugin that needs markup
// this refuses is a plugin asking for a capability the sandbox exists to
// withhold, and the answer is to extend the policy here after thinking about
// it — not to give one plugin a way around it.
func SanitizeAdminHTML(html string) SafeHTML {
	const max = 1 << 20
	if len(html) > max {
		html = html[:max]
	}
	return SafeHTML(adminPolicy.Sanitize(html))
}

// formOpen matches the opening tag of a form in already-sanitised markup.
var formOpen = regexp.MustCompile(`(?i)<form\b[^>]*>`)

// WithCSRFToken puts the session token into every form of a plugin's screen.
//
// The screen says, in so many words, that checking the token is the CMS's
// business and not the plugin's. It has to be put in for that to be true: a
// plugin's form is an ordinary browser submit with no header, so without this
// every button on every plugin screen answers 403 — which is what they did.
//
// After sanitising and never before. The sanitiser must not be given a chance
// to move or drop it, and a plugin must not be able to write one itself: what a
// plugin sends is filtered, what is added here is the host's.
func WithCSRFToken(screen SafeHTML, token string) SafeHTML {
	if token == "" {
		return screen
	}
	field := `<input type="hidden" name="gorilla.csrf.Token" value="` +
		template.HTMLEscapeString(token) + `">`
	return SafeHTML(formOpen.ReplaceAllStringFunc(string(screen), func(tag string) string {
		return tag + field
	}))
}
