package admin

import (
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/marker"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The opt-out widget in a preview.
//
// A page that says [[widerspruch]] gets the opt-out widget at delivery
// (internal/public's expandOptOut): a line saying whether this visitor's
// browser sends Global Privacy Control, and the opt-out form. The line is a
// fact of the visitor's request, which a preview does not have, and the form
// posts to the public site, where a preview has no business sending anyone.
// So the preview draws neither. It draws a box that says what goes there, in
// the operator's language, because the box is a note to the person editing
// and not something a visitor reads.
//
// The rules are the public side's, so the preview does not promise what the
// page will not do: the box only on a website whose set has the United
// States, one per page, the marker on a line of its own taking its paragraph
// with it, and a marker that is not in text — typed into a link address —
// removed before anything is placed (marker.OnlyInText, UF-1). On any other
// website the public page shows nothing where the marker stands, and the box
// says that instead.
//
// The patterns are the public side's too, written out again here: this
// package does not import internal/public, for the reason previewAlbums gives.

// optoutPreviewMarker is what a page writes where the widget goes.
const optoutPreviewMarker = "[[widerspruch]]"

// optoutPreviewMark is the attribute the box carries, so a test and a reader
// of the preview's markup can tell it from the page's own content.
const optoutPreviewMark = "data-hc-preview-optout"

var (
	optoutPreviewParagraph = regexp.MustCompile(`<p>\s*\[\[widerspruch\]\]\s*</p>`)
	optoutPreviewPattern   = regexp.MustCompile(`\[\[widerspruch\]\]`)

	// optoutPreviewSentence is the box on a website that takes opt-outs.
	optoutPreviewSentence = i18n.N("Here visitors see whether their browser sends Global Privacy Control, and the opt-out form.")
	// optoutPreviewNotOffered is the box on any other website.
	optoutPreviewNotOffered = i18n.N("Visitors see nothing here: the opt-out form only appears on websites that follow US law.")
)

// expandOptOutPreview puts the placeholder box where the body says
// [[widerspruch]]. ws may be nil when the website could not be read; the box
// then says what a website without US shows, which is nothing.
func expandOptOutPreview(r *http.Request, ws *domain.Website, body string) string {
	if !strings.Contains(body, optoutPreviewMarker) {
		return body
	}
	// The marker's pattern only: OnlyInText looks at one token at a time, and
	// the paragraph pattern spans three. It is for the replacement below.
	body = marker.OnlyInText(body, optoutPreviewPattern)
	sentence := optoutPreviewNotOffered
	if ws != nil && ws.Frameworks().Has(legal.US) {
		sentence = optoutPreviewSentence
	}
	// Inline style, because the box appears in two places: the editor's pane,
	// which has the admin stylesheet, and the themed preview, which has only
	// the theme's. Both answer with a policy that allows style attributes
	// (internal/web/headers.go).
	box := `<div ` + optoutPreviewMark + ` role="note" style="border:1px dashed currentColor;border-radius:0.5em;padding:1em;margin:1em 0;opacity:0.75">` +
		html.EscapeString(web.T(r, sentence)) + `</div>`
	placed := false
	place := func(string) string {
		if placed {
			return ""
		}
		placed = true
		return box
	}
	body = optoutPreviewParagraph.ReplaceAllStringFunc(body, place)
	return optoutPreviewPattern.ReplaceAllStringFunc(body, place)
}
