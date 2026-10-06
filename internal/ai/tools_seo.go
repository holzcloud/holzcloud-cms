package ai

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/seo"
	"github.com/holzcloud/holzcloud-cms/internal/seo/audit"
)

// seoReportLimit bounds how many pages one answer lists. A website with a
// thousand pages that all lack a description would otherwise be a very long
// answer; the summary still counts every page, and "truncated" says so.
const seoReportLimit = 200

// seoReport is the SEO report of the admin as JSON. It asks internal/seo/audit,
// the same code the editor's SEO tab and the report screen ask, so an assistant
// and a person are told the same things about the same page.
func seoReport(d Deps) Tool {
	return Tool{
		Name: "seo_report",
		Description: "What a search engine would trip over, per page: title and description length, " +
			"duplicates, headings, images without a description, thin text, links, the address. " +
			"Give a website to get every page that has findings, worst first, or a page to get that " +
			"page alone. Each finding has a code (e.g. title-short), a severity (error, warn, info) " +
			"and sometimes a count. The pages are checked as they are saved. Changes nothing.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteArg,
				"page":    {Type: "integer", Description: "id of one page; its website is taken from it, and website may be left out"},
			},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Page    int64 `json:"page"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}

			if a.Website == 0 && a.Page == 0 {
				return nil, errors.New("give a website, or a page")
			}
			websiteID := a.Website
			var only int64
			if a.Page != 0 {
				p, err := livePageOf(c, d, a.Page)
				if err != nil {
					return nil, err
				}
				if a.Website != 0 && a.Website != p.WebsiteID {
					return nil, errors.New("that page belongs to another website")
				}
				websiteID, only = p.WebsiteID, p.ID
			}
			ws, err := visibleWebsite(c, d, websiteID)
			if err != nil {
				return nil, err
			}

			src := audit.Source{Pages: d.Pages, Media: d.Media, Domains: d.Domains}
			if ops, err := d.pageOps(); err == nil {
				src.BlockSet = ops.OpBlockSet
			} else {
				src.BlockSet = func(_ context.Context, _ int64) block.Set { return block.Builtin }
			}
			reports, err := src.Site(c.Ctx, ws)
			if err != nil {
				return nil, err
			}

			var errs, warns, infos int
			rows := make([]map[string]any, 0)
			type ranked struct {
				row   map[string]any
				worst int
				n     int
				title string
			}
			var list []ranked
			for _, rep := range reports {
				if only != 0 && rep.Page.ID != only {
					continue
				}
				e, w, i := seo.Count(rep.Findings)
				errs, warns, infos = errs+e, warns+w, infos+i
				if len(rep.Findings) == 0 && only == 0 {
					continue
				}
				loc := rep.Page.Locale
				if loc == "" {
					loc = ws.Locale
				}
				list = append(list, ranked{
					row: map[string]any{
						"id": rep.Page.ID, "title": rep.Page.Title, "locale": loc,
						"worst": string(rep.Worst), "findings": rep.Findings,
					},
					worst: rep.Worst.Rank(), n: len(rep.Findings), title: strings.ToLower(rep.Page.Title),
				})
			}
			sort.SliceStable(list, func(i, j int) bool {
				x, y := list[i], list[j]
				if x.worst != y.worst {
					return x.worst > y.worst
				}
				if x.n != y.n {
					return x.n > y.n
				}
				return x.title < y.title
			})
			truncated := false
			for _, r := range list {
				if len(rows) >= seoReportLimit {
					truncated = true
					break
				}
				rows = append(rows, r.row)
			}
			out := map[string]any{
				"website":       ws.ID,
				"pages_checked": len(reports),
				"summary":       map[string]int{"error": errs, "warn": warns, "info": infos},
				"pages":         rows,
			}
			if only != 0 {
				out["pages_checked"] = 1
			}
			if truncated {
				out["truncated"] = true
			}
			return out, nil
		},
	}
}
