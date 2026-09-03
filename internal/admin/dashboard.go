package admin

import (
	"net/http"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// DashboardData extends LayoutData for the admin dashboard.
type DashboardData struct {
	web.LayoutData
	WebsiteCount int
	PageCount    int
	MediaCount   int
	Websites     []DashboardWebsite
	RecentPages  []RecentPage
}

// DashboardWebsite holds per-website stats for the dashboard.
type DashboardWebsite struct {
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

// RecentPage holds data for a recently edited page.
type RecentPage struct {
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
	// Wer nur für bestimmte Websites zuständig ist, sieht auch nur deren Zahlen.
	// Eine Gesamtzahl über alles wäre eine Auskunft über Websites, die diese
	// Person nicht betreten darf.
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

	var websites []DashboardWebsite
	for rows.Next() {
		var ws DashboardWebsite
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

	var recentPages []RecentPage
	for actRows.Next() {
		var rp RecentPage
		var updatedAt string
		if err := actRows.Scan(&rp.ID, &rp.Title, &updatedAt, &rp.WebsiteName, &rp.WebsiteID); err != nil {
			return err
		}
		if !rights.MayUse(rp.WebsiteID) {
			continue
		}
		rp.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		recentPages = append(recentPages, rp)
		// Fünf, aber fünf der eigenen: gefiltert wird hier und nicht in SQL,
		// darum holt die Abfrage mehr und hier wird abgeschnitten.
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

	data := DashboardData{
		LayoutData:   web.NewLayoutData(r, h.sm, "Übersicht"),
		WebsiteCount: websiteCount,
		PageCount:    pageCount,
		MediaCount:   mediaCount,
		Websites:     websites,
		RecentPages:  recentPages,
	}
	data.ActiveNav = "dashboard"
	return web.RenderAdmin(w, h.templates, r, "dashboard", data)
}
