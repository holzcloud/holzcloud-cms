// Package seo looks at one page and says what a search engine would trip over.
//
// It is pure on purpose: no database, no HTTP, no admin. The editor's SEO tab,
// the per-website report and the MCP tool seo_report all call Analyze with the
// same Input and therefore cannot disagree about what is wrong with a page.
// Getting an Input out of the database is the job of internal/seo/audit.
//
// A finding is a code and a severity, never a sentence. The sentences live in
// the admin templates, where they are translated; an assistant reading the JSON
// gets the plain English code, which is what it can act on.
package seo

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Severity ranks a finding. Error is a page a search engine cannot use, warn is
// something that costs clicks, info is worth knowing.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Rank orders severities: higher is worse. An unknown value ranks zero.
func (s Severity) Rank() int {
	switch s {
	case SeverityError:
		return 3
	case SeverityWarn:
		return 2
	case SeverityInfo:
		return 1
	}
	return 0
}

// The codes. Plain English, kebab-case, stable: they appear in the report's
// filter address and in the MCP answer.
const (
	CodeTitleMissing        = "title-missing"
	CodeTitleShort          = "title-short"
	CodeTitleLong           = "title-long"
	CodeDescriptionMissing  = "description-missing"
	CodeDescriptionFallback = "description-fallback"
	CodeDescriptionShort    = "description-short"
	CodeDescriptionLong     = "description-long"
	CodeDuplicateTitle      = "duplicate-title"
	CodeDuplicateDesc       = "duplicate-description"
	CodeH1InBody            = "h1-in-body"
	CodeHeadingSkip         = "heading-skip"
	CodeImagesNoAlt         = "images-no-alt"
	CodeThinText            = "thin-text"
	CodeNoInternalLinks     = "no-internal-links"
	CodeSlugLong            = "slug-long"
	CodeSlugNoise           = "slug-noise"
	CodeNoIndex             = "noindex"
	CodeProtected           = "protected"
	CodeDraft               = "draft"
	CodeNoFeaturedImage     = "no-featured-image"
)

// Codes is every code Analyze can return, in a stable order. The report
// validates its filter against it, so a made-up code in the address is ignored
// instead of producing an empty list that looks like a clean site.
func Codes() []string {
	return []string{
		CodeTitleMissing, CodeTitleShort, CodeTitleLong,
		CodeDescriptionMissing, CodeDescriptionFallback, CodeDescriptionShort, CodeDescriptionLong,
		CodeDuplicateTitle, CodeDuplicateDesc,
		CodeH1InBody, CodeHeadingSkip, CodeImagesNoAlt, CodeThinText, CodeNoInternalLinks,
		CodeSlugLong, CodeSlugNoise,
		CodeNoIndex, CodeProtected, CodeDraft, CodeNoFeaturedImage,
	}
}

// Known reports whether code is one Analyze can return.
func Known(code string) bool {
	for _, c := range Codes() {
		if c == code {
			return true
		}
	}
	return false
}

// Thresholds. Each is a rule of thumb and says whose.
const (
	// TitleMin: below this a title is rarely descriptive enough to earn a click.
	TitleMin = 20
	// TitleMax: search results cut a title at about this many characters.
	TitleMax = 60
	// DescriptionMin: a shorter description leaves the snippet to be filled by
	// whatever the engine picks out of the page.
	DescriptionMin = 70
	// DescriptionMax: the snippet is about this wide; the rest is cut off.
	DescriptionMax = 160
	// ThinWords: below this a page rarely ranks for anything.
	ThinWords = 150
	// SlugMax: addresses longer than this are cut in the results.
	SlugMax = 75
)

// slugStopWords add length to an address and no meaning. Whole hyphen-separated
// tokens only, so a stop word does not match inside a longer word.
var slugStopWords = map[string]bool{
	"und": true, "der": true, "die": true, "das": true,
	"the": true, "and": true, "of": true, "a": true,
}

// Finding is one thing worth saying about a page.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	// Count is the number a hint needs: other pages with the same title,
	// pictures without a description, words on a thin page. Zero when the code
	// carries none.
	Count int `json:"count,omitempty"`
	// Detail is a short value, never a sentence.
	Detail string `json:"detail,omitempty"`
}

// Other is another page of the same website, as far as duplicate detection
// needs to know it. The caller supplies only that website's pages.
type Other struct {
	ID          int64
	Title       string
	Description string
	Locale      string
	Status      string
	Deleted     bool
}

// Input is everything Analyze reads. Built from the saved page, never from a
// half-typed form.
type Input struct {
	ID          int64
	Title       string
	Slug        string
	Description string
	// Locale is the page's language, "" for the website's main one.
	Locale string
	Status string
	// Kind is "page" or "post"; TypeKey is the website's own kind or "".
	Kind    string
	TypeKey string

	// Markdown is the page as written. BlocksText is the prose and links of a
	// page built from blocks, flattened by audit.FlattenBlocks. Headings and
	// links are read from both; the word count from BlocksText when there is
	// any, because a block page's Markdown is the same words as plain text.
	Markdown   string
	BlocksText string

	NoIndex     bool
	Protected   bool
	HasFeatured bool
	// ImagesWithoutAlt counts pictures the caller found without a description
	// (blocks, galleries). Pictures in the Markdown are counted here.
	ImagesWithoutAlt int

	// Host is this website's primary host, so an absolute link to it counts as
	// internal.
	Host            string
	SiteDescription string
	Others          []Other
}

var (
	fenceLine  = regexp.MustCompile("^\\s{0,3}(```|~~~)")
	headingRe  = regexp.MustCompile(`(?mi)^\s{0,3}(#{1,6})[ \t]+\S|<h([1-6])[\s>]`)
	mdLinkRe   = regexp.MustCompile(`(?:^|[^!])\[[^\]]*\]\(\s*<?([^)\s>]+)`)
	hrefRe     = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)
	mdImageRe  = regexp.MustCompile(`!\[([^\]]*)\]\(`)
	tagRe      = regexp.MustCompile(`<[^>]*>`)
	mdImgFull  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLinkText = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markupRe   = regexp.MustCompile("[#*_>`~|]+")
)

// withoutFences drops fenced code blocks: a "# comment" in a code sample is not
// a heading and a link in an example is not a link.
func withoutFences(s string) string {
	var out []string
	in := false
	for _, line := range strings.Split(s, "\n") {
		if fenceLine.MatchString(line) {
			in = !in
			continue
		}
		if !in {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// plainWords counts the words a reader sees: markup, image and link targets and
// tags removed.
func plainWords(s string) int {
	s = withoutFences(s)
	s = mdImgFull.ReplaceAllString(s, " ")
	s = mdLinkText.ReplaceAllString(s, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = markupRe.ReplaceAllString(s, " ")
	return len(strings.Fields(s))
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func runes(s string) int { return utf8.RuneCountInString(strings.TrimSpace(s)) }

// internalLink reports whether a link target stays on this website.
func internalLink(target, host string) bool {
	target = strings.TrimSpace(target)
	switch {
	case target == "", strings.HasPrefix(target, "#"), strings.HasPrefix(target, "//"):
		return false
	case strings.HasPrefix(target, "/"):
		return true
	}
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	if u.Host != "" {
		return host != "" && strings.EqualFold(u.Hostname(), host)
	}
	// No scheme, no host: "kontakt" or "../kontakt". "mailto:x" and "tel:x"
	// have a scheme and are not pages.
	return u.Scheme == ""
}

// Analyze returns the findings for one page, worst first, then by code.
func Analyze(in Input) []Finding {
	var out []Finding
	add := func(code string, sev Severity, count int, detail string) {
		out = append(out, Finding{Code: code, Severity: sev, Count: count, Detail: detail})
	}

	title := strings.TrimSpace(in.Title)
	switch n := runes(title); {
	case n == 0:
		add(CodeTitleMissing, SeverityError, 0, "")
	case n < TitleMin:
		add(CodeTitleShort, SeverityWarn, n, "")
	case n > TitleMax:
		add(CodeTitleLong, SeverityWarn, n, "")
	}

	desc := strings.TrimSpace(in.Description)
	switch n := runes(desc); {
	case n == 0 && strings.TrimSpace(in.SiteDescription) != "":
		add(CodeDescriptionFallback, SeverityInfo, 0, "")
	case n == 0:
		add(CodeDescriptionMissing, SeverityWarn, 0, "")
	case n < DescriptionMin:
		add(CodeDescriptionShort, SeverityWarn, n, "")
	case n > DescriptionMax:
		add(CodeDescriptionLong, SeverityWarn, n, "")
	}

	// Duplicates: only against live, published pages in the same language.
	// A draft's title is nobody's business yet and a trashed page is gone.
	sameTitle, sameDesc := 0, 0
	for _, o := range in.Others {
		if o.ID == in.ID || o.Deleted || o.Status != "published" || o.Locale != in.Locale {
			continue
		}
		if title != "" && norm(o.Title) == norm(title) {
			sameTitle++
		}
		if desc != "" && norm(o.Description) == norm(desc) {
			sameDesc++
		}
	}
	if sameTitle > 0 {
		add(CodeDuplicateTitle, SeverityWarn, sameTitle, "")
	}
	if sameDesc > 0 {
		add(CodeDuplicateDesc, SeverityWarn, sameDesc, "")
	}

	body := withoutFences(in.Markdown + "\n\n" + in.BlocksText)
	h1, skip, prev := false, false, 0
	for _, m := range headingRe.FindAllStringSubmatch(body, -1) {
		level := len(m[1])
		if level == 0 && len(m[2]) == 1 {
			level = int(m[2][0] - '0')
		}
		if level == 1 {
			h1 = true
		}
		if prev != 0 && level > prev+1 {
			skip = true
		}
		prev = level
	}
	if h1 {
		add(CodeH1InBody, SeverityWarn, 0, "")
	}
	if skip {
		add(CodeHeadingSkip, SeverityInfo, 0, "")
	}

	noAlt := in.ImagesWithoutAlt
	for _, m := range mdImageRe.FindAllStringSubmatch(body, -1) {
		if strings.TrimSpace(m[1]) == "" {
			noAlt++
		}
	}
	if noAlt > 0 {
		add(CodeImagesNoAlt, SeverityWarn, noAlt, "")
	}

	if (in.Kind == "" || in.Kind == "page" || in.Kind == "post") && in.TypeKey == "" {
		text := in.Markdown
		if strings.TrimSpace(in.BlocksText) != "" {
			text = in.BlocksText
		}
		if words := plainWords(text); words < ThinWords {
			add(CodeThinText, SeverityInfo, words, "")
		}
	}

	internal := false
	for _, m := range mdLinkRe.FindAllStringSubmatch(body, -1) {
		if internalLink(m[1], in.Host) {
			internal = true
		}
	}
	for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
		if internalLink(m[1], in.Host) {
			internal = true
		}
	}
	if !internal {
		add(CodeNoInternalLinks, SeverityInfo, 0, "")
	}

	if n := runes(in.Slug); n > SlugMax {
		add(CodeSlugLong, SeverityInfo, n, "")
	}
	tokens := strings.Split(strings.ToLower(in.Slug), "-")
	if len(tokens) > 1 {
		for _, t := range tokens {
			if slugStopWords[t] {
				add(CodeSlugNoise, SeverityInfo, 0, t)
				break
			}
		}
	}

	if in.NoIndex {
		add(CodeNoIndex, SeverityInfo, 0, "")
	}
	if in.Protected {
		add(CodeProtected, SeverityInfo, 0, "")
	}
	if in.Status != "" && in.Status != "published" {
		add(CodeDraft, SeverityInfo, 0, "")
	}
	if !in.HasFeatured {
		add(CodeNoFeaturedImage, SeverityInfo, 0, "")
	}

	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := out[i].Severity.Rank(), out[j].Severity.Rank(); ri != rj {
			return ri > rj
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// Worst is the highest severity among the findings, or "" for none.
func Worst(fs []Finding) Severity {
	var w Severity
	for _, f := range fs {
		if f.Severity.Rank() > w.Rank() {
			w = f.Severity
		}
	}
	return w
}

// Count tallies findings by severity.
func Count(fs []Finding) (errors, warns, infos int) {
	for _, f := range fs {
		switch f.Severity {
		case SeverityError:
			errors++
		case SeverityWarn:
			warns++
		case SeverityInfo:
			infos++
		}
	}
	return
}
