package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// What the signed-in person may do, where the screens need to know.
//
// The hard border is the access middleware: it stands in front of every address
// under /admin/websites/<id> and cannot be forgotten in a new handler. What
// happens here is the other half — not letting somebody see doors they may not
// open. A list of sites one cannot enter is a list of 403s waiting to be
// clicked, and a publish button that always fails is worse than no button.

// rightsOf is what this request's person may do. Anything unknown counts as
// unlimited: this decides what is *shown*, and the middleware decides what is
// allowed.
func (h *Handler) rightsOf(r *http.Request) user.Rights {
	if h.users == nil {
		return user.Everything()
	}
	id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if id == 0 {
		return user.Everything()
	}
	rights, err := h.users.Rights(r.Context(), id)
	if err != nil {
		return user.Everything()
	}
	return rights
}

// mayPublish reports whether this person may put something online themselves.
//
// The counterpart is the review workflow that already exists: somebody who may
// not publish saves a draft and submits it, and the button says so instead of
// disappearing without explanation.
func (h *Handler) mayPublish(r *http.Request) bool { return h.rightsOf(r).MayPublish }

// isAdmin reports whether this person administers the installation.
func (h *Handler) isAdmin(r *http.Request) bool {
	return h.sm.GetString(r.Context(), auth.SessionKeyUserRole) == user.RoleAdmin
}

// keepMine drops the websites this person may not enter.
func keepMine(rights user.Rights, all []domain.Website) []domain.Website {
	if !rights.Limited() {
		return all
	}
	out := make([]domain.Website, 0, len(all))
	for _, ws := range all {
		if rights.MayUse(ws.ID) {
			out = append(out, ws)
		}
	}
	return out
}

// NewNavWebsiteList is the website list the sidebar switcher is built from,
// narrowed to what this person may enter.
//
// Wired in main.go in place of the plain store method: the switcher is the one
// list on every single screen, and a switcher that offers a site one cannot
// open is a promise the next click breaks.
func NewNavWebsiteList(sm *scs.SessionManager, domains *domain.Store, users *user.Store) func(context.Context) ([]domain.Website, error) {
	return func(ctx context.Context) ([]domain.Website, error) {
		all, err := domains.ListWebsites(ctx)
		if err != nil || users == nil {
			return all, err
		}
		id := sm.GetInt64(ctx, auth.SessionKeyUserID)
		if id == 0 {
			return all, nil
		}
		rights, err := users.Rights(ctx, id)
		if err != nil {
			return all, nil
		}
		return keepMine(rights, all), nil
	}
}

// siteTicks is the website list of the rights form: every site with a tick, so
// what comes back is the complete answer and no difference has to be guessed.
func (h *Handler) siteTicks(r *http.Request, rights user.Rights) []SiteTick {
	all, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return nil
	}
	out := make([]SiteTick, 0, len(all))
	for _, ws := range all {
		out = append(out, SiteTick{ID: ws.ID, Name: ws.Name, Ticks: rights.Limited() && rights.MayUse(ws.ID)})
	}
	return out
}

// rightsFromForm reads what the user form says about somebody's limits.
//
// An administrator has none: the role is the right to run the installation, so
// the two controls are hidden for that role and ignored here. Everything else
// comes back complete — no tick at all means "every website", which is what the
// form says next to the empty list.
func rightsFromForm(r *http.Request, role string) user.Rights {
	if role == user.RoleAdmin {
		return user.Everything()
	}
	rights := user.Rights{MayPublish: r.FormValue("darf_veroeffentlichen") != ""}
	for _, raw := range r.PostForm["websites"] {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			rights.Websites = append(rights.Websites, id)
		}
	}
	return rights
}
