// Package design turns a website's few design choices into CSS custom
// properties, and refuses anything that is not one of those choices.
//
// The gap it fills: picking between four themes is too coarse — everyone wants
// their own colour — and uploading a template is too much, because it means
// maintaining a copy of a theme through every future change.
package design

import (
	"fmt"
	"html/template"
	"strings"
)

// Tokens are the values an operator may set.
//
// Every field is empty or zero by default and then means "leave the theme
// alone": a token that always had a value would override a theme that had a
// better answer for it.
type Tokens struct {
	Ink   string // body text
	Paper string // page background
	Brand string // links and accents
	Font  string // one of FontStacks
	// Measure is the width of the text column in characters, 0 for the theme's
	// own choice.
	Measure int
	// Radius is corner rounding in pixels; -1 for the theme's own choice, so
	// that 0 can mean square corners deliberately.
	Radius int
}

// FontStacks are the typefaces on offer.
//
// A fixed list rather than a name typed by hand: a name that is not installed
// falls back to something else with no warning, and a name that is a URL would
// be a way to load a font from another server — which this project forbids
// outright.
var FontStacks = []struct{ Value, Label, Stack string }{
	{"", "Wie die Vorlage", ""},
	{"system", "Systemschrift", `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`},
	{"serif", "Serifenschrift", `Georgia, "Iowan Old Style", "Times New Roman", Times, serif`},
	{"humanist", "Humanistisch", `Seravek, "Gill Sans Nova", Ubuntu, Calibri, "DejaVu Sans", source-sans-pro, sans-serif`},
	{"mono", "Schreibmaschine", `ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace`},
}

// Measure bounds. Below forty characters a line is a column of single words;
// above a hundred the eye loses the start of the next line.
const (
	MinMeasure = 40
	MaxMeasure = 110
)

// MaxRadius bounds corner rounding.
const MaxRadius = 32

// Sanitize returns the tokens with every unusable value dropped.
//
// Dropping rather than rejecting: a value the CSS cannot use is one the theme
// should answer for instead, and refusing the whole form over a mistyped colour
// would lose the four settings that were right.
func Sanitize(t Tokens) Tokens {
	out := Tokens{
		Ink:    hexColour(t.Ink),
		Paper:  hexColour(t.Paper),
		Brand:  hexColour(t.Brand),
		Radius: -1,
	}
	if knownFont(t.Font) {
		out.Font = t.Font
	}
	if t.Measure >= MinMeasure && t.Measure <= MaxMeasure {
		out.Measure = t.Measure
	}
	if t.Radius >= 0 && t.Radius <= MaxRadius {
		out.Radius = t.Radius
	}
	return out
}

// Empty reports whether nothing has been chosen, so the layout emits no style
// element at all rather than an empty one.
func (t Tokens) Empty() bool {
	return t.Ink == "" && t.Paper == "" && t.Brand == "" &&
		t.Font == "" && t.Measure == 0 && t.Radius < 0
}

// CSS renders the tokens as a :root rule.
//
// It is assembled from values that have already passed Sanitize, so every one
// is a hex colour, a stack from the fixed list, or an integer in range. That is
// what makes marking the result as template.CSS honest rather than a hope.
func (t Tokens) CSS() template.CSS {
	t = Sanitize(t)
	if t.Empty() {
		return ""
	}

	var b strings.Builder
	b.WriteString(":root{")
	writeToken(&b, "--hc-ink", t.Ink)
	writeToken(&b, "--hc-paper", t.Paper)
	writeToken(&b, "--hc-brand", t.Brand)
	if stack := fontStack(t.Font); stack != "" {
		fmt.Fprintf(&b, "--hc-font:%s;", stack)
	}
	if t.Measure > 0 {
		fmt.Fprintf(&b, "--hc-measure:%dch;", t.Measure)
	}
	if t.Radius >= 0 {
		fmt.Fprintf(&b, "--hc-radius:%dpx;", t.Radius)
	}
	b.WriteString("}")
	return template.CSS(b.String())
}

func writeToken(b *strings.Builder, name, value string) {
	if value != "" {
		fmt.Fprintf(b, "%s:%s;", name, value)
	}
}

// hexColour accepts only what a colour input produces.
//
// #rgb, #rrggbb and #rrggbbaa and nothing else. It is the narrowest shape that
// covers every colour and the easiest to be certain about — an oklch() or a
// named colour would each need their own parser, and each parser is a place a
// closing brace could slip through.
func hexColour(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw[0] != '#' {
		return ""
	}
	digits := raw[1:]
	switch len(digits) {
	case 3, 6, 8:
	default:
		return ""
	}
	for _, r := range digits {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return ""
		}
	}
	return "#" + strings.ToLower(digits)
}

func knownFont(value string) bool {
	for _, f := range FontStacks {
		if f.Value == value {
			return true
		}
	}
	return false
}

func fontStack(value string) string {
	for _, f := range FontStacks {
		if f.Value == value {
			return f.Stack
		}
	}
	return ""
}
