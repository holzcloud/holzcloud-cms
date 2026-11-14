package admin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// dashboardData extends LayoutData for the admin dashboard.
type dashboardData struct {
	web.LayoutData
	WebsiteCount int
	PageCount    int
	MediaCount   int
	Websites     []dashboardWebsite
	RecentPages  []recentPage
	// RetentionWarning says the retention job has not succeeded for more
	// than 48 hours (D-05, D-09). Only an administrator is shown it: the
	// job is the installation's, and so is what has to be done about it.
	RetentionWarning *retentionWarningView
}

// retentionWarningView is the warning that the retention job has not
// succeeded for more than 48 hours (D-05, D-09), as the dashboard and every
// Retention screen show it. Nil when the job keeps up.
type retentionWarningView struct {
	// LastOK is when the last successful run started, on the reader's
	// clock; nil when the job never succeeded since it was installed.
	LastOK *time.Time
	// Link is a Retention screen to open, "" on the screen itself.
	Link string
}

// retentionWarning asks the job's state and returns the warning, nil when
// the job keeps up. loc is the clock the date is shown on.
//
// A failure to read the state is not an error page: the dashboard is where
// an administrator lands after signing in, and a screen that does not open
// warns nobody. It is logged, and the screen shows no warning.
func (h *Handler) retentionWarning(r *http.Request, loc *time.Location, link string) *retentionWarningView {
	if h.dataRights == nil {
		return nil
	}
	st, err := h.dataRights.RetentionStatus(r.Context(), time.Now())
	if err != nil {
		slog.Error("retention status", "err", err)
		return nil
	}
	if !st.Overdue {
		return nil
	}
	v := &retentionWarningView{Link: link}
	if st.LastOK != nil {
		t := st.LastOK.In(loc)
		v.LastOK = &t
	}
	return v
}

// dashboardWebsite holds per-website stats for the dashboard.
type dashboardWebsite struct {
	ID           int64
	Name         string
	PageCount    int
	MediaCount   int
	TemplateName string
	// OpenChecks is how many readiness items this website still fails. It is
	// shown on the card so a half-configured site is visible from the start
	// screen rather than only from its own settings.
	OpenChecks int
}

// recentPage holds data for a recently edited page.
type recentPage struct {
	ID          int64
	Title       string
	UpdatedAt   time.Time
	WebsiteName string
	WebsiteID   int64
}

// HandleDashboard renders the admin dashboard with stats.
func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Global stats — single query
	// Trashed rows are excluded everywhere here: a page in the trash is not
	// content the operator has, and counting it made the dashboard disagree
	// with the page list it links to.
	// Whoever is responsible only for certain websites sees only their numbers.
	// A total over everything would be information about websites this person
	// may not enter.
	rights := h.rightsOf(r)

	var websiteCount, pageCount, mediaCount int
	err := h.db.Read.QueryRowContext(ctx,
		`SELECT
			(SELECT COUNT(*) FROM websites) AS website_count,
			(SELECT COUNT(*) FROM pages WHERE deleted_at IS NULL) AS page_count,
			(SELECT COUNT(*) FROM media WHERE deleted_at IS NULL) AS media_count`,
	).Scan(&websiteCount, &pageCount, &mediaCount)
	if err != nil {
		return err
	}

	// Per-website stats with active template name
	rows, err := h.db.Read.QueryContext(ctx,
		`SELECT w.id, w.name,
			(SELECT COUNT(*) FROM pages p WHERE p.website_id = w.id AND p.deleted_at IS NULL) AS page_count,
			(SELECT COUNT(*) FROM media m WHERE m.website_id = w.id AND m.deleted_at IS NULL) AS media_count,
			COALESCE((SELECT t.name FROM templates t
			          JOIN website_templates wt ON wt.template_id = t.id
			          WHERE wt.website_id = w.id AND wt.is_active = 1), '') AS template_name
		FROM websites w ORDER BY w.name`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var websites []dashboardWebsite
	for rows.Next() {
		var ws dashboardWebsite
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.PageCount, &ws.MediaCount, &ws.TemplateName); err != nil {
			return err
		}
		if !rights.MayUse(ws.ID) {
			continue
		}
		websites = append(websites, ws)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if rights.Limited() {
		websiteCount, pageCount, mediaCount = len(websites), 0, 0
		for _, ws := range websites {
			pageCount += ws.PageCount
			mediaCount += ws.MediaCount
		}
	}

	// Recent activity — last 5 page edits
	actRows, err := h.db.Read.QueryContext(ctx,
		`SELECT p.id, p.title, p.updated_at, w.name, w.id
		FROM pages p JOIN websites w ON p.website_id = w.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.updated_at DESC LIMIT 25`)
	if err != nil {
		return err
	}
	defer actRows.Close()

	var recentPages []recentPage
	for actRows.Next() {
		var rp recentPage
		var updatedAt string
		if err := actRows.Scan(&rp.ID, &rp.Title, &updatedAt, &rp.WebsiteName, &rp.WebsiteID); err != nil {
			return err
		}
		if !rights.MayUse(rp.WebsiteID) {
			continue
		}
		rp.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		recentPages = append(recentPages, rp)
		// Five, but five of their own: the filtering happens here and not in
		// SQL, so the query fetches more and the cut is made here.
		if len(recentPages) == 5 {
			break
		}
	}
	if err := actRows.Err(); err != nil {
		return err
	}

	// The readiness count needs the full website row and its domains, which the
	// summary query above deliberately does not carry.
	for i := range websites {
		ws, err := h.domains.GetWebsite(ctx, websites[i].ID)
		if err != nil || ws == nil {
			continue
		}
		domainList, err := h.domains.ListDomains(ctx, ws.ID)
		if err != nil {
			continue
		}
		for _, c := range h.siteChecks(ctx, ws, domainList) {
			if !c.OK {
				websites[i].OpenChecks++
			}
		}
	}

	data := dashboardData{
		LayoutData:   web.NewLayoutData(r, h.sm, "Overview"),
		WebsiteCount: websiteCount,
		PageCount:    pageCount,
		MediaCount:   mediaCount,
		Websites:     websites,
		RecentPages:  recentPages,
	}
	if h.isAdmin(r) {
		// The first website on the screen, for its Retention screen: every
		// one shows the same warning and the job's last run.
		link := ""
		if len(websites) > 0 {
			link = privacyRetentionAddress(websites[0].ID)
		}
		// The dashboard belongs to no website and so to no website's clock;
		// the date is given in UTC and says so.
		data.RetentionWarning = h.retentionWarning(r, time.UTC, link)
	}
	data.ActiveNav = "dashboard"
	return web.RenderAdmin(w, h.templates, r, "dashboard", data)
}
