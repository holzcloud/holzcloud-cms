package admin

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/user"
	"github.com/holzcloud/holzcloud-cms/internal/web"
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
// allowed. Whether a write happens is not decided here: since phase 31
// mayPublish has its own strict path, on which a failed lookup refuses (D-08).
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
		// Logged, because the screen and the doors now disagree: the screen
		// offers every control, and mayPublish refuses each of them with
		// "ask somebody who may publish" — even to a publisher. Without this
		// line the operator saw those refusals with no cause beside them.
		slog.Warn("rights lookup failed; the screen shows every control", "user_id", id, "err", err)
		return user.Everything()
	}
	return rights
}

// mayPublish reports whether this person may put something online themselves.
//
// The counterpart is the review workflow that already exists: somebody who may
// not publish saves a draft and submits it, and the button says so instead of
// disappearing without explanation.
//
// It does not go through rightsOf, because it decides whether a write happens,
// and a lookup that fails must not count as every right there is (D-08). The
// role is read from the session first, so a failing user store never stops an
// administrator.
func (h *Handler) mayPublish(r *http.Request) bool {
	if h.isAdmin(r) {
		return true
	}
	if h.users == nil {
		return true
	}
	id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if id == 0 {
		// RequireAuth stands in front of every admin handler, so only an
		// unsigned test request reaches this line.
		return true
	}
	rights, err := h.users.Rights(r.Context(), id)
	if err != nil {
		slog.Error("rights lookup failed; the publishing right is refused", "user_id", id, "err", err)
		return false
	}
	return rights.MayPublish
}

// readOnly reports whether this person may look at but not change what the
// public sees at once. The rule of phase 31 (D-01): everything that is online
// the moment it is stored — a menu, a label, a redirect, and the rest — is
// read-only without the publishing right. One predicate, no switch per area.
func (h *Handler) readOnly(r *http.Request) bool { return !h.mayPublish(r) }

// readOnlySentence is the refusal every read-only door answers with, and the
// notice its screen shows. A function and not a constant for the reason
// liveLockedSentence gives. liveLockedSentence itself is not reused: it names
// a page and a duplicate as the way forward, and neither exists here.
func readOnlySentence() string {
	return i18n.N("What is changed here is online at once. Changing it is not part of your account – ask somebody who may publish.")
}

// refuseReadOnly refuses a write of somebody without the publishing right and
// reports whether it did; the caller returns nil when it reports true.
//
// The answer is the sentence as a flash and h.redirect back to the area's own
// screen — HX-Redirect for htmx, Vary: HX-Request either way — and never a
// 403: the route classification test holds these routes open to editors, and
// a bare refusal tells nobody whom to ask (D-02).
//
// It is asked inside each HTTP handler, after the scope check, so a foreign
// id still answers 404, and before the first write (D-03). Never from
// createMenu, updateMenu or any other helper the MCP tools share: an AI key
// belongs to no person, and its tools answer as before (D-11).
func (h *Handler) refuseReadOnly(w http.ResponseWriter, r *http.Request, back string) bool {
	if !h.readOnly(r) {
		return false
	}
	web.SetFlashError(h.sm, r.Context(), readOnlySentence())
	_ = h.redirect(w, r, back)
	return true
}

// mediaInUseSentence is the refusal at a file in use, and the notice of its
// crop screen. readOnlySentence does not fit: a file is not online the moment
// it is stored — uploading stays open (D-06) — it is online because something
// shows it.
func mediaInUseSentence() string {
	return i18n.N("This file is in use on the website. Changing or deleting it is not part of your account – ask somebody who may publish.")
}

// mediaLocked reports whether this person may not delete, describe, crop or
// put back the original of this file: they may not publish, and something of
// the website shows the file (D-06, D-07).
//
// Nothing is asked for a person who may publish. For everybody else an error
// of the usage question counts as "in use": the question decides whether a
// picture disappears from a live page, and a lookup that fails must not be
// read as "nobody uses it" (D-08).
func (h *Handler) mediaLocked(r *http.Request, m *media.Media) bool {
	if !h.readOnly(r) {
		return false
	}
	used, err := h.mediaStore.InUse(r.Context(), m)
	if err != nil {
		slog.Error("media usage lookup failed; the file counts as in use", "media_id", m.ID, "err", err)
		return true
	}
	return used
}

// refuseMediaLocked refuses a change to a file in use by somebody without the
// publishing right and reports whether it did; the caller returns nil when it
// reports true. The contract is refuseReadOnly's (D-02): the sentence as a
// flash, h.redirect back, never a 403. Asked in the HTTP handlers only, after
// lookupMedia and before the first write — never in cropMedia,
// restoreOriginal, keepUpload or Store.Delete, which the MCP tools share (D-11).
func (h *Handler) refuseMediaLocked(w http.ResponseWriter, r *http.Request, m *media.Media, back string) bool {
	if !h.mediaLocked(r, m) {
		return false
	}
	web.SetFlashError(h.sm, r.Context(), mediaInUseSentence())
	_ = h.redirect(w, r, back)
	return true
}

// liveLocked reports whether this person may not change this page at all,
// because it is online and they may not publish.
//
// The user form promises exactly that — "without this right one writes and
// submits for review; somebody else puts it online" — and a live page that an
// editor can rewrite, rename, move to the wastebasket or fill with a picture is
// put online by that editor, whatever its status column says. So every handler
// that writes a page asks this before its first write.
//
// The status alone decides, not the schedule: a published page with a start
// time in the future goes live on its own, so its text is already a
// publisher's business. There is nothing to park the change in either —
// page_revisions holds pre-images and review_state is a flag on a draft — so
// the answer is a refusal that names the way forward: duplicate the page,
// which always makes a draft, and submit the copy for review.
func (h *Handler) liveLocked(r *http.Request, p *page.Page) bool {
	return p != nil && p.Status == "published" && !h.mayPublish(r)
}

// productLockedFor is liveLocked for products (D-14): this person may not
// change or delete this product, because it is online and they may not
// publish.
//
// It takes the answer of mayPublish instead of asking itself. A product
// request asks once and lets that answer decide everything: asked twice, a
// transient failure of one lookup and a success of the other split the
// request — "not locked" from one, "store it as a draft" from the other — and
// a publisher's save took a live product offline without a word.
//
// The same rule as for pages. A published product is in the shop the moment it
// is stored, so its text, price, stock and picture, and its removal, are a
// publisher's business; a draft is preparation, and somebody without the right
// creates and changes drafts as they do draft pages — handleProductSave keeps
// whatever they store a draft. The refusal is readOnlySentence, because a
// published product is online at once; liveLockedSentence proposes duplicating
// a page, and products have no such way forward.
func productLockedFor(mayPublish bool, p *shop.Product) bool {
	return p != nil && p.Status == shop.StatusPublished && !mayPublish
}

// liveLockedSentence is the refusal every locked door answers with. A function
// and not a constant, because a constant is invisible to tools/i18n: the
// i18n.N call is what makes the sentence a catalogue key, once.
func liveLockedSentence() string {
	return i18n.N("This page is online. Changing or removing it is not part of your account – ask somebody who may publish, or duplicate the page and submit the copy for review.")
}

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
func (h *Handler) siteTicks(r *http.Request, rights user.Rights) []siteTick {
	all, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return nil
	}
	out := make([]siteTick, 0, len(all))
	for _, ws := range all {
		out = append(out, siteTick{ID: ws.ID, Name: ws.Name, Ticks: rights.Limited() && rights.MayUse(ws.ID)})
	}
	return out
}

// rightsFromForm reads what the user form says about somebody's limits.
//
// An administrator has none: the role is the right to run the installation, so
// the websites are hidden for that role and ignored here. Everything else
// comes back complete, and the ticks are the whole answer: the websites that
// are ticked are the ones this person may enter, and none ticked means none.
// There is no way through this form that widens anybody's access by a button
// that was left alone.
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
	if len(rights.Websites) == 0 {
		nothing := user.Nothing()
		nothing.MayPublish = rights.MayPublish
		return nothing
	}
	return rights
}
