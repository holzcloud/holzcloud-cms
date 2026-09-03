package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// activityPerPage is one screenful. The protocol is read by scrolling back,
// not by jumping to a page number, so the size only has to be large enough
// that an afternoon fits on one or two pages.
const activityPerPage = 50

// ActivityRow is one line of the protocol, ready to print.
type ActivityRow struct {
	ID int64
	// When is for reading, WhenISO for the datetime attribute. Formatted here
	// and not in the template: the admin has no date helpers in its FuncMap,
	// and every other screen does it the same way.
	When        string
	WhenISO     string
	ActorEmail  string
	Action      string
	EntityType  string
	EntityID    int64
	WebsiteName string
	Metadata    map[string]any
}

// ActivityListData is the protocol screen.
type ActivityListData struct {
	web.LayoutData
	Pagination

	Rows []ActivityRow

	// The filters go back into the form so they survive both the pager and a
	// reload — a protocol one has narrowed down and then lost is worse than
	// none, because the second attempt starts from scratch.
	FilterWebsiteID string
	FilterAction    string
	FilterUserID    string
	FilterFrom      string
	FilterTo        string

	Websites []domain.Website
}

// HandleActivityList shows the protocol, narrowed by whatever was asked for.
func (h *Handler) HandleActivityList(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	pageNum := pageParam(r)

	f := activityFilter(q)

	entries, total, err := h.activityStore.List(r.Context(), f, activityPerPage, (pageNum-1)*activityPerPage)
	if err != nil {
		return err
	}

	// Die Namen der Websites in einem Zug, nicht je Zeile. Ein Protokoll hat
	// viele Zeilen und wenige Websites.
	websites, err := h.domains.ListWebsites(r.Context())
	if err != nil {
		return err
	}
	nameByID := make(map[int64]string, len(websites))
	for _, ws := range websites {
		nameByID[ws.ID] = ws.Name
	}

	rows := make([]ActivityRow, 0, len(entries))
	for _, e := range entries {
		name := ""
		if e.WebsiteID != nil {
			name = nameByID[*e.WebsiteID]
		}
		rows = append(rows, ActivityRow{
			ID:          e.ID,
			When:        e.CreatedAt.Format("02.01.2006 15:04"),
			WhenISO:     e.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ActorEmail:  e.ActorEmail,
			Action:      e.Action,
			EntityType:  e.EntityType,
			EntityID:    e.EntityID,
			WebsiteName: name,
			Metadata:    e.Metadata,
		})
	}

	data := ActivityListData{
		LayoutData: web.NewLayoutData(r, h.sm, "Protokoll"),
		Pagination: NewPagination(pageNum, activityPerPage, total).
			WithFilteredTarget("/admin/protokoll", "#activity-list", activityPagerQuery(q)),
		Rows:            rows,
		FilterWebsiteID: q.Get("website_id"),
		FilterAction:    q.Get("action"),
		FilterUserID:    q.Get("user_id"),
		FilterFrom:      q.Get("from"),
		FilterTo:        q.Get("to"),
		Websites:        websites,
	}
	data.ActiveNav = "activity"
	return web.RenderAdmin(w, h.templates, r, "activity_log", data)
}

// activityFilter reads the filters out of a query string. A value that is not
// a number or not a date is left out rather than refused: a hand-edited
// address should show an unfiltered protocol, not an error page.
func activityFilter(q url.Values) activity.Filter {
	var f activity.Filter
	if v := strings.TrimSpace(q.Get("website_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.WebsiteID = &id
		}
	}
	if v := strings.TrimSpace(q.Get("user_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.UserID = &id
		}
	}
	f.Action = strings.TrimSpace(q.Get("action"))
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.From = &t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			// Bis einschliesslich dieses Tages. Wer "bis 5. Mai" schreibt,
			// meint den 5. Mai mit, und nicht seinen ersten Augenblick.
			t = t.Add(24*time.Hour - time.Second)
			f.To = &t
		}
	}
	return f
}

// activityPagerQuery is the query string without "page", for the pager. Unlike
// the list views' filterQuery it keeps whatever it is given: the protocol's
// filters are not a fixed set of five names.
func activityPagerQuery(q url.Values) string {
	out := url.Values{}
	for k, vv := range q {
		if k == "page" {
			continue
		}
		for _, v := range vv {
			if strings.TrimSpace(v) != "" {
				out.Add(k, v)
			}
		}
	}
	return out.Encode()
}

// HandleActivityPurge deletes everything before a chosen day.
//
// The screen behind this asks for the password first (see the route table):
// it is the one action in the administration that removes the record of the
// other actions, and it should cost more than a stray click.
func (h *Handler) HandleActivityPurge(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	beforeStr := strings.TrimSpace(r.FormValue("before"))
	if beforeStr == "" {
		web.SetFlashError(h.sm, r.Context(), web.T(r, "Bitte ein Datum angeben"))
		return h.redirectBack(w, r, "/admin/protokoll")
	}
	before, err := time.Parse("2006-01-02", beforeStr)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.T(r, "Das ist kein gültiges Datum"))
		return h.redirectBack(w, r, "/admin/protokoll")
	}
	before = before.Add(24*time.Hour - time.Second)

	actor := activity.Entry{
		ActorEmail: h.sm.GetString(r.Context(), auth.SessionKeyUserEmail),
		Action:     activity.ActionActivityPurge,
		EntityType: "activity_log",
		Metadata:   map[string]any{},
	}
	if id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID); id != 0 {
		actor.UserID = &id
	}

	deleted, err := h.activityStore.Purge(r.Context(), before, actor)
	if err != nil {
		return err
	}

	web.SetFlashSuccess(h.sm, r.Context(),
		web.Titlef(r, "%d Einträge vor dem %s gelöscht", deleted, beforeStr))
	return h.redirectBack(w, r, "/admin/protokoll")
}

// LogActivity records one action, if a protocol is configured at all.
//
// Every call site goes through here rather than touching the store, so a build
// without the protocol needs no nil check in forty places — and so the acting
// person is filled in from the session in exactly one spot.
func (h *Handler) LogActivity(r *http.Request, e activity.Entry) {
	if h.activityStore == nil {
		return
	}
	ctx := r.Context()
	if e.ActorEmail == "" {
		e.ActorEmail = h.sm.GetString(ctx, auth.SessionKeyUserEmail)
	}
	if e.UserID == nil {
		if id := h.sm.GetInt64(ctx, auth.SessionKeyUserID); id != 0 {
			e.UserID = &id
		}
	}
	h.activityStore.Log(ctx, e)
}

// SetActivityStore hands the handler its protocol. Nil leaves the screen and
// every record out, which is what a build without it looks like from outside.
func (h *Handler) SetActivityStore(s *activity.Store) { h.activityStore = s }
