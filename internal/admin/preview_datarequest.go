package admin

import (
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/marker"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The request form in a preview (Phase 33.2, D-01).
//
// A page that says [[datenanfrage]] gets the request form at delivery
// (internal/public's expandDataRequest): a form carrying a time stamp signed
// for that one answer, which posts to the public site. A preview has neither
// the stamp nor any business sending an operator's browser to the public
// form, so it draws no form. It draws a box that says the form goes there,
// in the operator's language, because the box is a note to the person editing
// and not something a visitor reads.
//
// The rules are the public side's, so the preview does not promise what the
// page will not do: one form per page, the marker on a line of its own taking
// its paragraph with it, and a marker that is not in text — typed into a link
// address — removed before anything is placed (marker.OnlyInText). Unlike the
// opt-out box there is one sentence for every website: the form is offered
// whatever the website's frameworks (internal/public's dataRequestOffered).
//
// The box says what the Requests screen says about the page, with the same
// two catalogue keys, so the operator meets one name for it in both places.
//
// The patterns are the public side's too, written out again here: this
// package does not import internal/public, for the reason previewAlbums gives.

// dataRequestPreviewMarker is what a page writes where the form goes.
const dataRequestPreviewMarker = "[[datenanfrage]]"

// dataRequestPreviewMark is the attribute the box carries, so a test and a
// reader of the preview's markup can tell it from the page's own content.
const dataRequestPreviewMark = "data-hc-preview-datarequest"

var (
	dataRequestPreviewParagraph = regexp.MustCompile(`<p>\s*\[\[datenanfrage\]\]\s*</p>`)
	dataRequestPreviewPattern   = regexp.MustCompile(`\[\[datenanfrage\]\]`)

	// dataRequestPreviewHeading names the page, as the Requests screen and
	// the legal pages card do.
	dataRequestPreviewHeading = i18n.N("Request about my data")
	// dataRequestPreviewSentence says what the form on it is for.
	dataRequestPreviewSentence = i18n.N("Visitors send requests about their personal data through the form on this page: access, correction, erasure.")
)

// expandDataRequestPreview puts the placeholder box where the body says
// [[datenanfrage]].
func expandDataRequestPreview(r *http.Request, body string) string {
	if !strings.Contains(body, dataRequestPreviewMarker) {
		return body
	}
	// The marker's pattern only: OnlyInText looks at one token at a time, and
	// the paragraph pattern spans three. It is for the replacement below.
	body = marker.OnlyInText(body, dataRequestPreviewPattern)
	// Inline style, for the reason expandOptOutPreview gives: the box appears
	// in the editor's pane and in the themed preview, and only the first has
	// the admin stylesheet.
	box := `<div ` + dataRequestPreviewMark + ` role="note" style="border:1px dashed currentColor;border-radius:0.5em;padding:1em;margin:1em 0;opacity:0.75">` +
		`<strong style="display:block">` + html.EscapeString(web.T(r, dataRequestPreviewHeading)) + `</strong>` +
		html.EscapeString(web.T(r, dataRequestPreviewSentence)) + `</div>`
	placed := false
	place := func(string) string {
		if placed {
			return ""
		}
		placed = true
		return box
	}
	body = dataRequestPreviewParagraph.ReplaceAllStringFunc(body, place)
	return dataRequestPreviewPattern.ReplaceAllStringFunc(body, place)
}
