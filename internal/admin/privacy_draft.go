package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/privacy"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// HandlePrivacyDraft creates the current privacy template as a new draft page
// of the website named in the path.
//
// A website that was created before the template learned about cookies has a
// privacy page that says the site sets none, and some of those sites have a
// shop. That page is the operator's text now: it may be published, it may have
// been rewritten by a lawyer. So this handler never touches it (D-06). It only
// creates one more page, a draft under its own address, from the functions the
// website uses today. The operator compares the two and copies over what is
// missing; deleting or publishing either one stays their decision.
//
// The only calls this file makes on the page store are CreatePage and
// GetPageBySlug, and a test holds the list.
func (h *Handler) HandlePrivacyDraft(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	features, err := h.privacyFeatures(r.Context(), ws)
	if err != nil {
		return err
	}
	markdown := privacy.Compose(features)
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		return fmt.Errorf("render privacy draft: %w", err)
	}

	created, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID: ws.ID,
		Title:     privacy.DraftTitle,
		Slug:      privacy.DraftSlug,
		Markdown:  markdown,
		HTML:      html,
		Status:    "draft",
		Meta:      page.PageMeta{Excerpt: page.Excerpt(markdown)},
		UserID:    h.currentUserID(r),
	})
	if err != nil {
		return err
	}
	h.emitPageSaved(r, ws.ID, created, true)
	h.LogActivity(r, activity.Entry{
		Action:     activity.ActionPageCreate,
		EntityType: "page",
		EntityID:   created.ID,
		WebsiteID:  &ws.ID,
		Metadata:   map[string]any{"slug": created.Slug, "titel": created.Title},
	})
	web.SetFlashSuccess(h.sm, r.Context(), "Privacy template created as a draft")
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/pages/%d/edit", ws.ID, created.ID))
}

// The three states of the hint on the settings screen.
const (
	privacyNone     = "none"
	privacyExists   = "exists"
	privacyTemplate = "template"
)

// privacyHint is what the settings screen needs to know about the website's
// privacy page, and no more.
//
// It holds a state, an id and a flag. The title and the text of the page are
// the editors' own writing and the card must have no way to print them.
type privacyHint struct {
	State     string
	PageID    int64
	Published bool
}

// privacyHint looks at the page with the slug datenschutz.
func (h *Handler) privacyHint(ctx context.Context, ws *domain.Website) (privacyHint, error) {
	p, err := h.pages.GetPageBySlug(ctx, ws.ID, "datenschutz")
	if err != nil {
		return privacyHint{}, fmt.Errorf("look up privacy page: %w", err)
	}
	if p == nil {
		return privacyHint{State: privacyNone}, nil
	}
	hint := privacyHint{State: privacyExists, PageID: p.ID, Published: p.Status == "published"}
	if privacy.HasTemplateNotice(p.ContentMarkdown) {
		hint.State = privacyTemplate
	}
	return hint, nil
}
