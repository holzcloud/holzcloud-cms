package page

import (
	"bytes"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// md is the package-level goldmark Markdown renderer with common extensions.
//
// WithUnsafe lets raw HTML blocks through goldmark. Without it an editor who
// pastes "<p>Text</p>" gets an empty page and no error at all: goldmark emits
// the block as a comment and bluemonday then removes the comment, leaving "\n".
//
// This is safe only because bluemonday runs unconditionally afterwards and the
// public handlers cast the stored, already-sanitised string. That makes
// UGCPolicy the actual security boundary, which is why the sanitisation tests
// below are not optional.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

// sanitizer is the package-level bluemonday policy for user-generated content.
//
// UGCPolicy is the baseline. Two additions, both about images:
//
//   - loading="lazy" and decoding="async" on <img>, so a page full of photos
//     does not make a phone on a slow connection wait for all of them.
//   - srcset and sizes on <img> and <source>, plus <picture> and <figure>, so a
//     theme or an editor can offer a smaller file to a small screen. Without
//     them bluemonday strips the attributes and every visitor downloads the
//     full-size image.
//
// These are layout attributes with no script surface. The dangerous parts of
// UGCPolicy stay exactly as they are.
var sanitizer = newSanitizer()

func newSanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^(lazy|eager)$`)).OnElements("img", "iframe")
	p.AllowAttrs("decoding").Matching(regexp.MustCompile(`^(async|sync|auto)$`)).OnElements("img")

	// srcset is a list of URLs with descriptors. bluemonday does not URL-check
	// it, so it is restricted to the shapes a same-origin media path can take —
	// no colon, which rules out javascript: and data: outright.
	srcset := regexp.MustCompile(`^[^:<>"]*$`)
	p.AllowAttrs("srcset").Matching(srcset).OnElements("img", "source")
	p.AllowAttrs("sizes").Matching(regexp.MustCompile(`^[^<>"]*$`)).OnElements("img", "source")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^image/[a-z0-9.+-]+$`)).OnElements("source")
	p.AllowElements("picture", "figure", "figcaption")

	return p
}

// SanitizeHTML cleans HTML that did not come from RenderMarkdown.
//
// The same policy, deliberately: markup a plugin produced is read by visitors
// in this site's origin, exactly like an editor's, so the two are held to one
// standard. There is no second, laxer policy for anyone, because a second
// policy is a thing that gets picked by mistake.
func SanitizeHTML(s string) string { return sanitizer.Sanitize(s) }

// RenderMarkdown converts Markdown source to sanitized HTML.
// It uses goldmark for Markdown-to-HTML conversion, then bluemonday
// to strip any dangerous tags, scripts, or event handlers.
// The output is ALWAYS sanitized and safe for template.HTML casting.
func RenderMarkdown(source string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return sanitizer.Sanitize(buf.String()), nil
}
