package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/seo"
	"github.com/holzcloud/holzcloud-cms/internal/seo/audit"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The SEO check of one page, in the editor, and the SEO report of a website.
//
// Both ask internal/seo/audit, which is also what the MCP tool seo_report
// asks. Nothing here decides what is wrong with a page; it only decides how
// that is shown.

// seoPerPage is how many pages one screen of the report lists.
const seoPerPage = 50

// auditSource is what the editor tab and the report read pages through.
func (h *Handler) auditSource() audit.Source {
	return audit.Source{Pages: h.pages, Media: h.mediaStore, Domains: h.domains, BlockSet: h.blockSet}
}

// seoPanel is what the editor's SEO tab shows.
type seoPanel struct {
	// Status is "ok", "warn" or "error": the colour of the line, and one of
	// three sentences, so it never relies on colour alone.
	Status   string
	Findings []seo.Finding
}

// seoPanelFor checks the saved version of a page. It returns nil, and logs,
// when the check cannot be made: a failing SEO tab must not take the editor
// down with it.
func (h *Handler) seoPanelFor(r *http.Request, p *page.Page) *seoPanel {
	ws, err := h.domains.GetWebsite(r.Context(), p.WebsiteID)
	if err != nil || ws == nil {
		if err != nil {
			slog.Error("seo check: website", "err", err)
		}
		return nil
	}
	rep, err := h.auditSource().Page(r.Context(), ws, p)
	if err != nil {
		slog.Error("seo check", "page", p.ID, "err", err)
		return nil
	}
	status := "ok"
	switch seo.Worst(rep.Findings) {
	case seo.SeverityError:
		status = "error"
	case seo.SeverityWarn:
		status = "warn"
	}
	return &seoPanel{Status: status, Findings: rep.Findings}
}

// seoRow is one page of the report.
type seoRow struct {
	Page     page.Page
	Language string
	Worst    seo.Severity
	Findings []seo.Finding
}

// seoCodeChoice is one entry of the report's filter.
type seoCodeChoice struct {
	Code string
	// Pages is how many pages carry this finding.
	Pages    int
	Selected bool
}

type seoReportData struct {
	web.LayoutData
	WebsiteID int64
	// Code is the finding the list is narrowed to, or empty.
	Code  string
	Codes []seoCodeChoice
	// Checked is every live page, Affected those with at least one finding.
	Checked  int
	Affected int
	// Errors, Warnings and Notes count findings over the whole website, not
	// over the filtered list: they are the state of the site, the list is a
	// view of it.
	Errors, Warnings, Notes int
	Rows                    []seoRow
	Pagination
	PrevURL, NextURL string
}

// HandleSEOReport lists the pages of a website that have something to improve.
func (h *Handler) HandleSEOReport(w http.ResponseWriter, r *http.Request) error {
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

	// The filter is checked against the list of codes: a made-up one would
	// otherwise produce an empty table that reads as "nothing wrong here".
	code := r.URL.Query().Get("code")
	if !seo.Known(code) {
		code = ""
	}

	reports, err := h.auditSource().Site(r.Context(), ws)
	if err != nil {
		return err
	}

	data := seoReportData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "SEO report – %s", ws.Name)),
		WebsiteID:  websiteID,
		Code:       code,
		Checked:    len(reports),
	}

	perCode := map[string]int{}
	var rows []seoRow
	for _, rep := range reports {
		if len(rep.Findings) == 0 {
			continue
		}
		data.Affected++
		e, wn, n := seo.Count(rep.Findings)
		data.Errors += e
		data.Warnings += wn
		data.Notes += n
		match := code == ""
		for _, f := range rep.Findings {
			perCode[f.Code]++
			if f.Code == code {
				match = true
			}
		}
		if match {
			rows = append(rows, seoRow{
				Page: rep.Page, Language: rowLanguage(ws, rep.Page.Locale),
				Worst: rep.Worst, Findings: rep.Findings,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Worst.Rank() != b.Worst.Rank() {
			return a.Worst.Rank() > b.Worst.Rank()
		}
		if len(a.Findings) != len(b.Findings) {
			return len(a.Findings) > len(b.Findings)
		}
		return strings.ToLower(a.Page.Title) < strings.ToLower(b.Page.Title)
	})

	for _, c := range seo.Codes() {
		if perCode[c] > 0 {
			data.Codes = append(data.Codes, seoCodeChoice{Code: c, Pages: perCode[c], Selected: c == code})
		}
	}

	// The page number is clamped to what exists, so ?page=9999 shows the last
	// page rather than an empty one.
	pageNum := pageParam(r)
	if last := (len(rows) + seoPerPage - 1) / seoPerPage; pageNum > last && last > 0 {
		pageNum = last
	}
	data.Pagination = newPagination(pageNum, seoPerPage, len(rows))
	from := (pageNum - 1) * seoPerPage
	if from > len(rows) {
		from = len(rows)
	}
	to := from + seoPerPage
	if to > len(rows) {
		to = len(rows)
	}
	data.Rows = rows[from:to]

	base := fmt.Sprintf("/admin/websites/%d/seo", websiteID)
	link := func(n int) string {
		q := url.Values{}
		if code != "" {
			q.Set("code", code)
		}
		q.Set("page", strconv.Itoa(n))
		return base + "?" + q.Encode()
	}
	if data.HasPrev {
		data.PrevURL = link(data.PrevPage)
	}
	if data.HasNext {
		data.NextURL = link(data.NextPage)
	}

	data.ActiveNav = "seo"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "seo_report", data)
}
