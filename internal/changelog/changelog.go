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
// # What it is not
//
// Not a feed, not a notification, not a badge that nags. It is a page, reached
// by following a link, and nothing about it changes because a release happened.
// A program that interrupts an operator to tell them about itself is a program
// that will be told to stop.
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
}

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
		html, err := page.RenderMarkdown(strings.Join(body, "\n"))
		if err != nil {
			// A release whose Markdown will not render is shown as nothing
			// rather than as raw source, and the ones around it are unaffected.
			// The same rule the block renderer follows: a failure costs its own
			// entry and never the page.
			html = ""
		}
		current.HTML = template.HTML(html)
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
