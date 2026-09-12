package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/bundle"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
	"github.com/holzcloud/holzcloud-cms/internal/wxr"
)

// The move from WordPress.
//
// Like the CMS's own import: a **new** website always comes about. Merging
// would need an answer for every collision — same address, different text —
// and the honest answer for a CMS of this size is a second website that you
// compare and then keep or delete.

// HandleWordPressImport creates a website from a WordPress export file.
func (h *Handler) HandleWordPressImport(w http.ResponseWriter, r *http.Request) error {
	// A WXR file is text and is exported without images; ten megabytes is more
	// than any of them and little enough for a small server.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	file, _, err := r.FormFile("wxr")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "File too large or not selected")
		return h.redirect(w, r, "/admin/websites")
	}
	defer file.Close()

	export, err := wxr.Parse(file)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Import failed: %s", err))
		return h.redirect(w, r, "/admin/websites")
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = export.SiteTitle
	}
	if name == "" {
		name = "Aus WordPress"
	}

	created, err := h.domains.CreateWebsite(r.Context(), name, "")
	if err != nil {
		return err
	}
	report := &bundle.Report{WebsiteID: created.ID}

	slugs := map[string]bool{}
	for _, item := range export.Items {
		if reason := h.importWordPressItem(r, created.ID, item, slugs); reason != "" {
			report.Warnings = append(report.Warnings, reason)
			continue
		}
		report.Pages++
	}

	// The images lie on the old server. Fetching them here would mean that this
	// server dials out of its own accord — exactly the rule that lets this CMS
	// do without a cookie banner. So the addresses are listed, and the operator
	// brings them along.
	if len(export.MediaURLs) > 0 {
		// One literal and not three joined with +: the collector reads a string,
		// not an expression, and a sentence assembled in Go is a sentence the
		// catalogue never learns about.
		report.Warnings = append(report.Warnings, web.Titlef(r, "%d images and files still lie on the old server. They are not downloaded — this server fetches nothing from third parties of its own accord. Upload them under Media and set the links anew:", len(export.MediaURLs)))
		for i, u := range export.MediaURLs {
			if i == 25 {
				report.Warnings = append(report.Warnings,
					web.Titlef(r, "… and %d more", len(export.MediaURLs)-25))
				break
			}
			report.Warnings = append(report.Warnings, u)
		}
	}
	if export.Skipped > 0 {
		report.Warnings = append(report.Warnings, web.Titlef(r, "%d entries were neither pages nor posts (attachments, menu items, trash) and were passed over.", export.Skipped))
	}
	if export.Truncated {
		report.Warnings = append(report.Warnings, web.Titlef(r, "The file holds more than %d entries. The rest was not read in.", wxr.MaxItems))
	}

	h.resolver.InvalidateCache()

	data := ImportReportData{
		LayoutData: web.NewLayoutData(r, h.sm, "WordPress import finished"),
		Report:     report,
	}
	data.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "import_report", data)
}

// importWordPressItem creates one page, or says why it could not.
//
// The content arrives as HTML and is stored as the page's source. That is not a
// compromise: Markdown passes block HTML through, and everything goes through
// the same sanitiser as anything else — so an imported page is editable, and a
// <script> WordPress happened to carry does not survive the first render.
func (h *Handler) importWordPressItem(r *http.Request, websiteID int64, item wxr.Item, seen map[string]bool) string {
	title := item.Title
	if title == "" {
		title = web.T(r, "Untitled")
	}

	slug := page.Slugify(item.Slug)
	if slug == "" {
		slug = page.Slugify(title)
	}
	if err := page.ValidateSlug(slug); err != nil {
		return web.Titlef(r, "%q: the address %q is not allowed", title, item.Slug)
	}
	// WordPress allows the same slug under different parents; this CMS does
	// not. The second one gets a number rather than silently overwriting.
	base := slug
	for i := 2; seen[slug]; i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	seen[slug] = true

	html, err := page.RenderMarkdown(item.HTML)
	if err != nil {
		return web.Titlef(r, "%q could not be set: %v", title, err)
	}

	status := "draft"
	if item.Published {
		status = "published"
	}

	created, err := h.pages.CreatePage(r.Context(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug,
		Markdown: item.HTML, HTML: html, Status: status, Kind: item.Kind,
		Meta: page.PageMeta{Excerpt: item.Excerpt},
	})
	if err != nil {
		return web.Titlef(r, "%q could not be created: %v", title, err)
	}

	if len(item.Terms) > 0 && h.terms != nil {
		if err := h.terms.SetForPage(r.Context(), websiteID, created.ID, item.Terms); err != nil {
			return web.Titlef(r, "Terms of %q: %v", title, err)
		}
	}
	return ""
}
