package menu

import (
	"html/template"
	"strings"
)

// RenderMenu outputs a menu as nested <ul><li> HTML for a given location key.
// Max depth is 3 levels. Returns empty string if location not found.
func RenderMenu(menus map[string][]MenuNode, locationKey string) template.HTML {
	return RenderMenuCurrent(menus, locationKey, "")
}

// RenderMenuCurrent is RenderMenu with the path of the page being rendered, so
// the matching entry gets aria-current="page" and a .is-current class.
//
// Without it a screen reader gives no clue which entry is the page you are on,
// and a theme has no hook to highlight it.
func RenderMenuCurrent(menus map[string][]MenuNode, locationKey, currentPath string) template.HTML {
	items, ok := menus[locationKey]
	if !ok || len(items) == 0 {
		return ""
	}
	return template.HTML(renderMenuLevel(items, 0, 3, normalizePath(currentPath)))
}

// normalizePath reduces a request path to the form menu links are stored in:
// a leading slash and no trailing one, with "" meaning the home page.
func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

// allowedLinkSchemes are the URL schemes permitted in menu links. Anything else
// — most importantly javascript: and data: — is rendered as plain text instead
// of a link, so a menu entry cannot become a script execution vector.
var allowedLinkSchemes = []string{"http://", "https://", "mailto:", "tel:"}

// safeLinkURL reports whether url may be used as an href. Relative URLs
// ("/about", "about", "#anchor") are allowed; absolute URLs must use an allowed
// scheme. Protocol-relative URLs ("//host") are allowed as they inherit https.
func safeLinkURL(url string) bool {
	if url == "" {
		return false
	}
	trimmed := strings.TrimLeft(url, " \t\r\n")
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, scheme := range allowedLinkSchemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	// No scheme at all (no colon before the first slash/?/#) means it is relative.
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case ':':
			return false
		case '/', '?', '#':
			return true
		}
	}
	return true
}

// renderMenuLevel recursively renders menu items as nested <ul><li> HTML.
func renderMenuLevel(items []MenuNode, depth, maxDepth int, currentPath string) string {
	if depth >= maxDepth || len(items) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<ul>")
	for _, item := range items {
		sb.WriteString("<li>")

		switch item.ItemType {
		case "page":
			if item.PageSlug != "" {
				// The language prefix goes into the address but not into the
				// comparison: a theme passes the page's slug as the current
				// path, and that slug has no prefix in any language.
				href := "/" + item.PageSlug
				writeLinkAt(&sb, item.PathPrefix+href, href, item.Title, currentPath)
			} else {
				// Page was deleted or unpublished; render as text
				sb.WriteString(`<span>`)
				sb.WriteString(template.HTMLEscapeString(item.Title))
				sb.WriteString(`</span>`)
			}
		case "url":
			if safeLinkURL(item.URL) {
				writeLink(&sb, item.URL, item.Title, currentPath)
			} else {
				sb.WriteString(`<span>`)
				sb.WriteString(template.HTMLEscapeString(item.Title))
				sb.WriteString(`</span>`)
			}
		case "custom":
			// Custom text, no link
			sb.WriteString(`<span>`)
			sb.WriteString(template.HTMLEscapeString(item.Title))
			sb.WriteString(`</span>`)
		default:
			sb.WriteString(template.HTMLEscapeString(item.Title))
		}

		if len(item.Children) > 0 {
			sb.WriteString(renderMenuLevel(item.Children, depth+1, maxDepth, currentPath))
		}

		sb.WriteString("</li>")
	}
	sb.WriteString("</ul>")
	return sb.String()
}

// writeLink emits one menu anchor, marking it as the current page when its
// target matches the path being rendered.
func writeLink(sb *strings.Builder, href, title, currentPath string) {
	writeLinkAt(sb, href, href, title, currentPath)
}

// writeLinkAt is writeLink where the address written and the address compared
// are not the same — a page link in a second language points at /fr/contact and
// is current when the page being rendered is contact.
func writeLinkAt(sb *strings.Builder, href, compare, title, currentPath string) {
	sb.WriteString(`<a href="`)
	sb.WriteString(template.HTMLEscapeString(href))
	sb.WriteString(`"`)
	if currentPath != "" && normalizePath(compare) == currentPath {
		sb.WriteString(` class="is-current" aria-current="page"`)
	}
	sb.WriteString(`>`)
	sb.WriteString(template.HTMLEscapeString(title))
	sb.WriteString(`</a>`)
}
