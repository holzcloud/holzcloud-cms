package admin

import (
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// The events the administration emits, and the boundary they draw.
//
// # Why this file exists at all
//
// The host declared five event names and emitted one. page.saved, page.deleted,
// media.added and form.received stood in internal/plugin/abi.go and, mirrored,
// in sdk/plugin.go — where a plugin author reads them and registers a handler.
// Nothing ever fired four of them. A plugin listening for page.saved was simply
// never called, with no error anywhere and nothing in the log: the worst shape
// a defect can have, because it looks exactly like "no page was saved".
//
// Found in v2.4 by tools/surface, which noticed the four constants were used
// nowhere at all — not because anybody met the bug.
//
// # What an event says, and what it deliberately does not
//
// These report what somebody did **in the administration**. That is the
// boundary, and it is a decision rather than an oversight: a CSV import, a
// bundle import and the assistant all write pages too, in the hundreds, and a
// plugin woken once per row would be a different feature with a different name.
// If those ever need an event they get their own, with a count in it.
//
// The payload stays small because it crosses into a sandbox and because a
// plugin that needs the page itself can ask for it through pages.get, where the
// host still decides what it may see. An id and an address are enough to know
// what changed.
//
// Emitted after the thing has already succeeded and never before: Manager.Emit
// neither waits nor reports, so a slow listener cannot make saving slow and a
// broken one cannot make saving fail.

// emitPageSaved tells the plugins that an editor saved a page.
//
// One name for created and updated alike, because that is what the constant
// says and what a listener wants: something at this address is different now.
// Which of the two it was is in "created".
func (h *Handler) emitPageSaved(r *http.Request, websiteID int64, p *page.Page, created bool) {
	if h.plugins == nil || p == nil {
		return
	}
	h.plugins.Emit(plugin.EventPageSaved, websiteID, map[string]string{
		"id":      strconv.FormatInt(p.ID, 10),
		"slug":    p.Slug,
		"status":  p.Status,
		"created": strconv.FormatBool(created),
	})
}

// emitPageDeleted tells the plugins that a page went to the trash.
//
// Trashed and not erased — that is what the delete button does — and the event
// says "deleted" because that is the word the operator pressed and the state a
// visitor now meets. A plugin keeping an index has to drop the address either
// way.
func (h *Handler) emitPageDeleted(websiteID int64, p *page.Page) {
	if h.plugins == nil || p == nil {
		return
	}
	h.plugins.Emit(plugin.EventPageDeleted, websiteID, map[string]string{
		"id":   strconv.FormatInt(p.ID, 10),
		"slug": p.Slug,
	})
}

// emitMediaAdded tells the plugins that a file arrived in the library.
//
// Only for a file that was actually written: an upload the store recognised as
// a duplicate adds nothing, and an event for it would have a listener building
// a second thumbnail of a picture that was already there.
func (h *Handler) emitMediaAdded(websiteID int64, m *media.Media) {
	if h.plugins == nil || m == nil {
		return
	}
	h.plugins.Emit(plugin.EventMediaAdded, websiteID, map[string]string{
		"id":   strconv.FormatInt(m.ID, 10),
		"name": m.OriginalName,
		"mime": m.MimeType,
	})
}
