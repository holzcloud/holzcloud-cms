// Package wxr reads a WordPress export file.
//
// Whether somebody can move in at all is decided before the first login: a
// website with two hundred entries does not get retyped, and without a way in
// this CMS is a thing for new sites only. WordPress is what most of them are
// coming from, and its export — WXR, an RSS file with its own namespace — is
// the one format every installation can produce without a plugin.
//
// What is deliberately *not* done here: fetching the pictures. They sit on the
// old server, and downloading them would make this server pull from a third
// party. The addresses are collected and reported instead, so the operator
// knows what to bring over — twenty files by hand beats a rule broken.
package wxr

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MaxItems bounds one import.
//
// A WordPress site with more entries than this exists, but importing it in one
// go means holding all of it in memory at once, which a small node does not
// have to spare. The limit is reported rather than silently applied.
const MaxItems = 2000

// Item is one page or post of the export.
type Item struct {
	Title string
	Slug  string
	// HTML is the content as WordPress wrote it, with the shortcodes removed.
	HTML string
	// Excerpt is WordPress's own summary, often empty.
	Excerpt string
	// Kind is "page" or "post".
	Kind string
	// Published is true for what was online; everything else arrives as a
	// draft, which is the safe direction.
	Published bool
	// Date is when it was published, zero when the export says nothing usable.
	Date time.Time
	// Terms are the categories and tags, as names.
	Terms []string
}

// Export is what one file contains.
type Export struct {
	// SiteTitle is the name of the old website, offered as the name of the new
	// one.
	SiteTitle string
	Items     []Item
	// MediaURLs are the pictures the entries point at, on the old server.
	// Sorted and unique.
	MediaURLs []string
	// Skipped counts what was left out: attachments, navigation menu items,
	// revisions — everything that is not a page or a post.
	Skipped int
	// Truncated says the file held more than MaxItems entries.
	Truncated bool
}

// The XML shapes. Only the fields that are actually used: a WXR file carries
// three dozen more, and reading them would mean maintaining them.
type feed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rawItem `xml:"item"`
	} `xml:"channel"`
}

type rawItem struct {
	Title string `xml:"title"`
	Slug  string `xml:"post_name"`
	// Both the content and the excerpt are called "encoded" and differ only in
	// their namespace — and that namespace carries the WXR version, so pinning
	// it would mean refusing files from another WordPress. They are collected
	// together and told apart below.
	Encoded   []encoded     `xml:"encoded"`
	PostType  string        `xml:"post_type"`
	Status    string        `xml:"status"`
	Date      string        `xml:"post_date_gmt"`
	PubDate   string        `xml:"pubDate"`
	Attach    string        `xml:"attachment_url"`
	Categorys []rawCategory `xml:"category"`
}

// encoded is one of the two <…:encoded> elements, with the namespace it came
// in under.
type encoded struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// content and excerpt split the two apart. The excerpt namespace is the one
// with "excerpt" in it, in every version of the format.
func (r rawItem) content() string {
	for _, e := range r.Encoded {
		if !strings.Contains(strings.ToLower(e.XMLName.Space), "excerpt") {
			return e.Value
		}
	}
	return ""
}

func (r rawItem) excerpt() string {
	for _, e := range r.Encoded {
		if strings.Contains(strings.ToLower(e.XMLName.Space), "excerpt") {
			return e.Value
		}
	}
	return ""
}

type rawCategory struct {
	Domain string `xml:",attr"`
	Name   string `xml:",chardata"`
}

// Parse reads an export file.
func Parse(r io.Reader) (*Export, error) {
	var f feed
	dec := xml.NewDecoder(r)
	// A WXR file may be declared as anything; the parts that matter are UTF-8
	// in practice, and a stricter reader would refuse files that import fine.
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	dec.Strict = false
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("das ist keine lesbare WordPress-Datei: %w", err)
	}
	if len(f.Channel.Items) == 0 {
		return nil, fmt.Errorf("die Datei enthält keine Einträge")
	}

	out := &Export{SiteTitle: strings.TrimSpace(f.Channel.Title)}
	media := map[string]bool{}

	for _, raw := range f.Channel.Items {
		if raw.Attach != "" {
			media[raw.Attach] = true
		}
		kind := ""
		switch raw.PostType {
		case "page":
			kind = "page"
		case "post":
			kind = "post"
		default:
			// attachment, nav_menu_item, revision, a custom type: not content
			// this CMS has a place for.
			out.Skipped++
			continue
		}
		// Trash and auto-drafts are not content either. Everything that is not
		// "publish" arrives as a draft rather than being dropped: a private or
		// pending entry is work somebody did.
		if raw.Status == "trash" || raw.Status == "auto-draft" {
			out.Skipped++
			continue
		}
		if len(out.Items) >= MaxItems {
			out.Truncated = true
			break
		}

		html := stripShortcodes(raw.content())
		for _, u := range mediaURLs(html) {
			media[u] = true
		}

		out.Items = append(out.Items, Item{
			Title:     strings.TrimSpace(raw.Title),
			Slug:      strings.TrimSpace(raw.Slug),
			HTML:      html,
			Excerpt:   strings.TrimSpace(stripShortcodes(raw.excerpt())),
			Kind:      kind,
			Published: raw.Status == "publish",
			Date:      parseDate(raw.Date, raw.PubDate),
			Terms:     terms(raw.Categorys),
		})
	}

	for u := range media {
		out.MediaURLs = append(out.MediaURLs, u)
	}
	sort.Strings(out.MediaURLs)
	return out, nil
}

// terms are the categories and tags as names, without the uncategorised
// default that WordPress puts on everything nobody filed.
func terms(cats []rawCategory) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cats {
		name := strings.TrimSpace(c.Name)
		lower := strings.ToLower(name)
		if name == "" || seen[lower] || lower == "uncategorized" || lower == "uncategorised" ||
			lower == "allgemein" || lower == "unkategorisiert" {
			continue
		}
		seen[lower] = true
		out = append(out, name)
	}
	return out
}

// shortcode matches WordPress's [gallery], [caption]…[/caption] and friends.
//
// They are calls into PHP that means nothing here, and left in place they
// appear as square brackets in the middle of the text. Removed rather than
// translated: guessing what a plugin's shortcode did is how an import produces
// pages nobody can trust.
var shortcode = regexp.MustCompile(`\[/?[a-zA-Z][a-zA-Z0-9_-]*(\s[^\]]*)?\]`)

func stripShortcodes(s string) string {
	return strings.TrimSpace(shortcode.ReplaceAllString(s, ""))
}

// srcPattern finds the pictures an entry points at.
var srcPattern = regexp.MustCompile(`(?i)(?:src|href)="(https?://[^"]+\.(?:jpe?g|png|gif|webp|svg|pdf|mp4))"`)

func mediaURLs(html string) []string {
	var out []string
	for _, m := range srcPattern.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// parseDate reads whichever of the two dates WordPress filled in.
func parseDate(gmt, pub string) time.Time {
	gmt = strings.TrimSpace(gmt)
	if gmt != "" && gmt != "0000-00-00 00:00:00" {
		if t, err := time.Parse("2006-01-02 15:04:05", gmt); err == nil {
			return t.UTC()
		}
	}
	if t, err := time.Parse(time.RFC1123Z, strings.TrimSpace(pub)); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
