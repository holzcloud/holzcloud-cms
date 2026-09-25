package admin

import (
	"context"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
)

// The MCP tools in internal/ai do everything the admin screens do, and they do
// it through this handler wherever a screen does more than one store call:
// clearing caches, writing the activity log, handling files. Two paths that
// each carry their own copy of those steps drift apart the first time one of
// them is changed; one path cannot.
//
// The methods for the tools are named Op… and take a context and plain values
// rather than a request. The screens call the same functions underneath.

// OpLog writes one entry of the activity log for a change made through an AI
// key. The actor is named after the key, so the log says which connection did
// it — there is no signed-in person behind such a request, and no session to
// read one from.
func (h *Handler) OpLog(ctx context.Context, actor string, e activity.Entry) {
	if h.activityStore == nil {
		return
	}
	e.ActorEmail = actor
	h.activityStore.Log(ctx, e)
}

// OpChanged drops every cached copy of a website after a change, exactly as a
// save on a screen does.
func (h *Handler) OpChanged(websiteID int64) {
	h.invalidateWebsiteCaches(websiteID)
}
