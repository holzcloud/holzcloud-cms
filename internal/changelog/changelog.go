// Package changelog turns CHANGELOG.md into the screen behind the version
// number in the sidebar.
//
// # Why the operator sees this at all
//
// An operator who updates a self-hosted program has one question: what is
// different now. The answer already exists, written in whole sentences by
// whoever made the change, and until now it was only readable by somebody who
// went looking for the repository. The version in the sidebar is where they
// would ask, so that is where the answer is.
//
// # When it is shown
//
// Owner decision of 2026-11-01, replacing the earlier rule that nothing
// announces this screen: after an update, each person is shown the new
// version's entry once, on their next full page load in the administration.
// "Once" is remembered per account and per version in the database (see Decide),
// so a second browser or a new sign-in does not repeat it.
//
// It is never shown on a fresh installation or to a newly created account,
// which have nothing to catch up on, and never for a build that is not a
// release in CHANGELOG.md, which has no entry. It is always reachable again
// from Settings, About and from the version in the sidebar.
//
// # The language
//
// The entries are in German and are not translated. That is a limitation and it
// is stated here rather than hidden: the changelog is a document written by
// hand, not a catalogue of interface strings, and running 44 KB of prose
// through i18n.N would neither work nor be honest. The chrome around it — the
// headings, the version list, the empty case — goes through the catalogue like
// everything else an operator reads.
package changelog

import (
	"html/template"
	"strings"
	"sync"

	holzcloud "github.com/holzcloud/holzcloud-cms"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Release is one version's entry.
type Release struct {
	// Version is the number as the heading spells it: "2.2", "1.10". Without a
	// leading v, because that is what the file says and what the CHANGELOG's own
	// opening promises the tags are named after.
	Version string
	// Date is the ISO date beside it, empty if the heading carries none.
	Date string
	// HTML is the entry, rendered and sanitised.
	//
	// template.HTML because it has been through page.RenderMarkdown, which is
	// goldmark followed by bluemonday — the same path an editor's own text
	// takes. The rule this project states as "never cast unsanitised goldmark
	// output" is kept by not having a second renderer here.
	HTML template.HTML
	// Lead is what stands before the first "### " heading of the entry: the
	// sentence that says what the release is. For an entry written before the
	// groups existed it is the whole entry.
	Lead template.HTML
	// Groups are the "### " parts, in the order the screen shows them and not
	// the order the file has them: new things first, then what was improved,
	// then what was fixed, then everything else.
	Groups []Group
}

// Kind is what a group is about. The screen turns it into a word, a colour and
// an order; the file only ever says it by its heading.
type Kind string

const (
	KindNew      Kind = "new"
	KindImproved Kind = "improved"
	KindFixed    Kind = "fixed"
	// KindNote is every other heading (for example a "good to know" part): it
	// keeps the heading it was written with.
	KindNote Kind = "note"
)

// Group is one "### " part of an entry.
type Group struct {
	Kind Kind
	// Title is the heading as written, and the only thing a KindNote group has
	// to say what it is; the other kinds are named by the screen.
	Title string
	HTML  template.HTML
}

// KindOf reads a heading. The three headings the file's opening promises are
// matched without regard to case; anything else is a note.
func KindOf(heading string) Kind {
	switch strings.ToLower(strings.TrimSpace(heading)) {
	case "neu":
		return KindNew
	case "verbessert", "geändert":
		return KindImproved
	case "behoben":
		return KindFixed
	}
	return KindNote
}

// kindOrder is the order of the groups on the screen.
var kindOrder = []Kind{KindNew, KindImproved, KindFixed, KindNote}

// Major is the first number of the version, for grouping a long list.
//
// "2.2" and "2.0" belong together; "1.10" does not belong with them. Returned
// as a string because it is a label and never arithmetic.
func (r Release) Major() string {
	if i := strings.IndexByte(r.Version, '.'); i > 0 {
		return r.Version[:i]
	}
	return r.Version
}

// Series is the label a long list is grouped under: "0.1" for 0.1.2, "0.0" for
// 0.0.25, "2" for 2.9.1 and "1" for 1.10.
//
// The two counts move in different places. While the major is 0 the count
// moves in the minor number (0.0.x and 0.1.x are different generations of the
// program); the legacy 1.x and 2.x count moves in the major.
func (r Release) Series() string {
	parts := strings.Split(r.Version, ".")
	if parts[0] == "0" && len(parts) > 1 {
		return parts[0] + "." + parts[1]
	}
	return parts[0]
}

// once guards the parse. The file is 44 KB of Markdown and the result never
// changes while the process runs, so it is done on the first request and not at
// startup: a program that does not serve this page should not pay for it.
var (
	once     sync.Once
	releases []Release
	byNumber map[string]Release
)

// All returns every release, newest first, in the order the file has them.
func All() []Release {
	once.Do(parse)
	return releases
}

// Find returns one release by its number, and whether it exists.
//
// The caller hands this whatever stood in the URL, so it is a lookup and never
// a path: a version that is not in the map is simply not found, and nothing is
// built out of the string.
func Find(version string) (Release, bool) {
	once.Do(parse)
	r, ok := byNumber[strings.TrimPrefix(strings.TrimSpace(version), "v")]
	return r, ok
}

// Latest is the newest release, and false for a file with none.
func Latest() (Release, bool) {
	all := All()
	if len(all) == 0 {
		return Release{}, false
	}
	return all[0], true
}

// parse splits the file on its version headings.
//
// Deliberately a split and not a Markdown walk. The shape is fixed by the file's
// own opening — "one heading per version with a number and a date" — and a
// second goldmark pass to find the headings would parse 44 KB twice to learn
// something a line prefix already says. What goldmark does get to do is the
// part that matters: rendering one entry's body, once, through the same
// sanitising path as every other piece of Markdown in this program.
//
// Anything before the first heading is the file's explanation of itself. It is
// dropped: an operator asking what changed is not asking what a changelog is.
func parse() {
	byNumber = map[string]Release{}

	lines := strings.Split(holzcloud.Changelog, "\n")
	var (
		current Release
		body    []string
		open    bool
	)
	flush := func() {
		if !open {
			return
		}
		render := func(lines []string) template.HTML {
			html, err := page.RenderMarkdown(strings.Join(lines, "\n"))
			if err != nil {
				// A release whose Markdown will not render is shown as nothing
				// rather than as raw source, and the ones around it are
				// unaffected. The same rule the block renderer follows: a
				// failure costs its own entry and never the page.
				return ""
			}
			return template.HTML(html)
		}
		current.HTML = render(body)
		current.Lead, current.Groups = splitGroups(body, render)
		releases = append(releases, current)
		byNumber[current.Version] = current
		body = nil
	}

	for _, line := range lines {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			version, date := splitHeading(rest)
			current = Release{Version: version, Date: date}
			open = true
			continue
		}
		if open {
			body = append(body, line)
		}
	}
	flush()
}

// splitHeading reads "2.2 — 2026-09-14" and also "2.2", which is what an entry
// looks like on the day it is written and before it is released.
//
// The separator is an em dash with spaces around it, which is what every
// heading in the file uses; a hyphen is accepted as well so that a heading typed
// on a keyboard without one still parses rather than becoming a version called
// "2.3 - 2026-10-01".
func splitHeading(s string) (version, date string) {
	s = strings.TrimSpace(s)
	for _, sep := range []string{" — ", " – ", " - "} {
		if v, d, ok := strings.Cut(s, sep); ok {
			return strings.TrimSpace(v), strings.TrimSpace(d)
		}
	}
	return s, ""
}

// splitGroups cuts an entry's body on its "### " headings.
//
// A split on a line prefix, for the reason parse is one: the shape is fixed by
// the file, and each part is rendered on its own through the sanitising path.
// A "### " inside a fenced code block would be taken for a heading; the file
// has none, and an entry that did would only be cut early, never rendered
// unsafely.
func splitGroups(body []string, render func([]string) template.HTML) (template.HTML, []Group) {
	var (
		leadLines []string
		groups    []Group
		cur       *Group
		curLines  []string
	)
	closeGroup := func() {
		if cur == nil {
			return
		}
		cur.HTML = render(curLines)
		groups = append(groups, *cur)
		cur, curLines = nil, nil
	}
	for _, line := range body {
		if rest, ok := strings.CutPrefix(line, "### "); ok {
			closeGroup()
			title := strings.TrimSpace(rest)
			cur = &Group{Kind: KindOf(title), Title: title}
			continue
		}
		if cur == nil {
			leadLines = append(leadLines, line)
		} else {
			curLines = append(curLines, line)
		}
	}
	closeGroup()

	ordered := make([]Group, 0, len(groups))
	for _, k := range kindOrder {
		for _, g := range groups {
			if g.Kind == k {
				ordered = append(ordered, g)
			}
		}
	}
	return render(leadLines), ordered
}

// SeriesLabel is the name a series has on its tab: "0.1.0" for the series
// 0.1, "0.0.x" for the one before it, and "2.x" for the legacy counts.
//
// While the major number is 0 a series is one minor number, and its first
// release is the number the operator knows it by. The legacy 1.x and 2.x have
// no such first release (the series starts at 2.0 and 1.4), so they say "x".
func SeriesLabel(series string) string {
	if strings.HasPrefix(series, "0.") {
		if series == "0.0" {
			return series + ".x"
		}
		return series + ".0"
	}
	return series + ".x"
}
