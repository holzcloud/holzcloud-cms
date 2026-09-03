// Package structured builds the schema.org JSON-LD a search engine reads.
//
// It is written out rather than left to the theme: a theme author who gets one
// field wrong produces a result that looks fine and is silently ignored, and
// the failure is invisible from the site itself.
package structured

import (
	"encoding/json"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"html/template"
	"strings"
	"time"
)

// Business is what the operator entered about the organisation behind a site.
type Business struct {
	// Type is a schema.org type such as "LocalBusiness" or "Organization".
	// Empty leaves the block out entirely.
	Type       string
	Name       string
	URL        string
	LogoURL    string
	Street     string
	PostalCode string
	City       string
	Country    string
	Phone      string
	Email      string
	// OpeningHours is the shorthand an operator typed, one rule per line.
	OpeningHours string
}

// Page is what one page contributes.
type Page struct {
	Title       string
	URL         string
	Description string
	ImageURL    string
	PublishedAt *time.Time
	UpdatedAt   *time.Time
	// IsPost switches the type from WebPage to Article.
	IsPost bool
	// SiteName is the site the page belongs to.
	SiteName string
	// AuthorName is the organisation credited with the article.
	AuthorName string
}

// Crumb is one step of the breadcrumb trail.
type Crumb struct {
	Name string
	URL  string
}

// Build renders the whole graph as one JSON-LD document.
//
// One @graph rather than several separate script blocks: the nodes reference
// each other by @id, so an article can name its publisher without repeating the
// address, and a consumer that reads only one block still gets a coherent
// picture.
func Build(b Business, p Page, crumbs []Crumb) template.JS {
	var nodes []any

	if node := businessNode(b); node != nil {
		nodes = append(nodes, node)
	}
	if node := pageNode(b, p); node != nil {
		nodes = append(nodes, node)
	}
	if node := breadcrumbNode(crumbs); node != nil {
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return ""
	}

	doc := map[string]any{
		"@context": "https://schema.org",
		"@graph":   nodes,
	}
	// json.Marshal escapes <, > and & as <, > and & by default,
	// so a title containing "</script>" cannot break out of the element. That
	// is what makes marking the result as template.JS safe here.
	encoded, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return template.JS(encoded)
}

func businessNode(b Business) map[string]any {
	if b.Type == "" || b.Name == "" {
		return nil
	}
	node := map[string]any{
		"@type": b.Type,
		"@id":   b.URL + "#organisation",
		"name":  b.Name,
	}
	if b.URL != "" {
		node["url"] = b.URL
	}
	if b.LogoURL != "" {
		node["logo"] = absolute(b.URL, b.LogoURL)
		// image as well as logo: several consumers read only one of the two.
		node["image"] = absolute(b.URL, b.LogoURL)
	}
	if b.Phone != "" {
		node["telephone"] = b.Phone
	}
	if b.Email != "" {
		node["email"] = b.Email
	}
	if addr := addressNode(b); addr != nil {
		node["address"] = addr
	}
	if hours := openingHours(b.OpeningHours); len(hours) > 0 {
		node["openingHours"] = hours
	}
	return node
}

func addressNode(b Business) map[string]any {
	if b.Street == "" && b.PostalCode == "" && b.City == "" {
		return nil
	}
	addr := map[string]any{"@type": "PostalAddress"}
	if b.Street != "" {
		addr["streetAddress"] = b.Street
	}
	if b.PostalCode != "" {
		addr["postalCode"] = b.PostalCode
	}
	if b.City != "" {
		addr["addressLocality"] = b.City
	}
	if b.Country != "" {
		addr["addressCountry"] = b.Country
	}
	return addr
}

// openingHours splits the operator's text into the one-rule-per-entry form
// schema.org expects.
//
// No validation of the shorthand: a rule a consumer cannot parse is ignored by
// it, whereas rejecting the whole field would cost the operator the ones that
// were right.
func openingHours(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func pageNode(b Business, p Page) map[string]any {
	if p.Title == "" || p.URL == "" {
		return nil
	}

	kind := "WebPage"
	if p.IsPost {
		kind = "Article"
	}
	node := map[string]any{
		"@type": kind,
		"@id":   p.URL,
		"url":   p.URL,
		"name":  p.Title,
	}
	if p.IsPost {
		// headline is what an article is indexed by; name alone is ignored.
		node["headline"] = p.Title
	}
	if p.Description != "" {
		node["description"] = p.Description
	}
	if p.ImageURL != "" {
		node["image"] = absolute(b.URL, p.ImageURL)
	}
	if p.PublishedAt != nil {
		node["datePublished"] = p.PublishedAt.UTC().Format(time.RFC3339)
	}
	if p.UpdatedAt != nil {
		node["dateModified"] = p.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if p.SiteName != "" {
		node["isPartOf"] = map[string]any{
			"@type": "WebSite",
			"name":  p.SiteName,
			"url":   b.URL,
		}
	}
	if b.Type != "" && b.Name != "" {
		// A reference by @id rather than a second copy of the address.
		node["publisher"] = map[string]any{"@id": b.URL + "#organisation"}
		if p.IsPost {
			author := p.AuthorName
			if author == "" {
				author = b.Name
			}
			node["author"] = map[string]any{"@type": "Organization", "name": author}
		}
	}
	return node
}

func breadcrumbNode(crumbs []Crumb) map[string]any {
	// A single-step trail says nothing a consumer does not already know.
	if len(crumbs) < 2 {
		return nil
	}
	items := make([]any, 0, len(crumbs))
	for i, c := range crumbs {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     c.Name,
			"item":     c.URL,
		})
	}
	return map[string]any{"@type": "BreadcrumbList", "itemListElement": items}
}

// absolute turns a same-origin path into a full URL.
//
// A search engine will not resolve a relative path out of a JSON document the
// way a browser resolves one out of an attribute.
func absolute(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	return strings.TrimSuffix(base, "/") + ref
}

// OrgTypes are the schema.org types offered in the settings.
//
// A short list, not the whole vocabulary: the value only helps if it is right,
// and a dropdown of six hundred types is one nobody reads to the end of.
var OrgTypes = []struct{ Value, Label string }{
	{"", i18n.N("Keine Angabe")},
	{"LocalBusiness", i18n.N("Betrieb vor Ort")},
	{"HomeAndConstructionBusiness", i18n.N("Handwerk und Bau")},
	{"Store", i18n.N("Laden")},
	{"Restaurant", i18n.N("Gastronomie")},
	{"ProfessionalService", i18n.N("Dienstleistung")},
	{"Organization", i18n.N("Organisation oder Verein")},
}

// KnownOrgType reports whether a value is one of the offered types.
func KnownOrgType(value string) bool {
	for _, t := range OrgTypes {
		if t.Value == value {
			return true
		}
	}
	return false
}
