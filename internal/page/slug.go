package page

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	reNonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
	reDashes      = regexp.MustCompile(`-{2,}`)
)

// transliterations maps non-ASCII letters to their ASCII spelling before the
// slug is stripped down.
//
// Without this, dropping the character outright mangles the word: "Möbelbau"
// would become "m-belbau" rather than "moebelbau". German umlauts follow the
// ae/oe/ue/ss convention; the accented Latin letters fall back to their base
// letter, which is the usual convention for URLs.
var transliterations = map[rune]string{
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'å': "a", 'æ': "ae",
	'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ø': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u",
	'ý': "y", 'ÿ': "y",
	'đ': "d", 'ð': "d", 'þ': "th",
	'ł': "l", 'š': "s", 'ž': "z", 'č': "c", 'ř': "r", 'ę': "e", 'ą': "a",
}

// reservedSlugs are the first path segments the router already owns. A page
// using one would be shadowed by that route and permanently unreachable, with
// no error anywhere to explain why.
var reservedSlugs = map[string]bool{
	"admin":       true,
	"t":           true,
	"media":       true,
	"assets":      true,
	"healthz":     true,
	"readyz":      true,
	"sitemap.xml": true,
	"robots.txt":  true,
	"feed.xml":    true,
	"search":      true,
	"tag":         true,
}

// MaxSlugLength bounds a slug so it stays a usable URL segment.
const MaxSlugLength = 200

// ValidateSlug checks a slug supplied by an editor.
//
// The create and edit handlers only run Slugify when the field is left empty,
// so a hand-typed value reaches the database verbatim. Without this check a
// slug containing "/", an uppercase letter or a reserved name silently produces
// a page that can never be opened.
func ValidateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug must not be empty")
	}
	if len(slug) > MaxSlugLength {
		return fmt.Errorf("slug is longer than %d characters", MaxSlugLength)
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("%q is reserved by the application and would be unreachable", slug)
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return errors.New("slug must not start or end with a hyphen")
	}
	if strings.Contains(slug, "--") {
		return errors.New("slug must not contain two hyphens in a row")
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("slug may only contain lowercase letters, digits and hyphens — %q is not allowed", string(r))
		}
	}
	return nil
}

// Transliterate lowercases s and rewrites accented and umlaut letters to their
// ASCII spelling. It is exported so every place that builds a slug — page slugs
// here, template slugs in the admin package — folds characters the same way.
func Transliterate(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if replacement, ok := transliterations[r]; ok {
			b.WriteString(replacement)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Slugify converts a title string into a URL-safe slug.
// Lowercases, transliterates accented letters to ASCII, replaces the rest of the
// non-alphanumerics with hyphens, collapses multiples and trims them from the
// ends. Returns "untitled" if the result is empty.
func Slugify(title string) string {
	s := reNonAlphaNum.ReplaceAllString(Transliterate(title), "-")
	s = reDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	// Truncate at a word boundary, so the automatic path can never produce a
	// slug that ValidateSlug would reject on the manual path.
	if len(s) > MaxSlugLength {
		s = s[:MaxSlugLength]
		if cut := strings.LastIndexByte(s, '-'); cut > 0 {
			s = s[:cut]
		}
		s = strings.Trim(s, "-")
	}

	if s == "" {
		s = "untitled"
	}
	return s
}
