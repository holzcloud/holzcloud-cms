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

// Der Umzug von WordPress.
//
// Wie der eigene Import: es entsteht immer eine **neue** Website. Zusammenlegen
// bräuchte für jeden Zusammenstoss eine Antwort — gleiche Adresse, anderer Text
// —, und die ehrliche Antwort für ein CMS dieser Grösse ist eine zweite
// Website, die man vergleicht und dann behält oder löscht.

// HandleWordPressImport creates a website from a WordPress export file.
func (h *Handler) HandleWordPressImport(w http.ResponseWriter, r *http.Request) error {
	// Eine WXR-Datei ist Text und wird ohne Bilder exportiert; zehn Megabyte
	// sind mehr als jede davon und wenig genug für einen kleinen Server.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	file, _, err := r.FormFile("wxr")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/websites")
	}
	defer file.Close()

	export, err := wxr.Parse(file)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Import fehlgeschlagen: %s", err))
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

	// Die Bilder liegen auf dem alten Server. Sie hierher zu holen hiesse, dass
	// dieser Server von sich aus nach draussen wählt — genau die Regel, die
	// dieses CMS ohne Cookie-Banner auskommen lässt. Also werden die Adressen
	// aufgezählt, und der Betreiber bringt sie mit.
	if len(export.MediaURLs) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%d Bilder und Dateien liegen weiterhin auf dem alten Server. Sie werden nicht "+
				"heruntergeladen — dieser Server holt von sich aus nichts von Dritten. "+
				"Lade sie unter Medien hoch und setze die Links neu:", len(export.MediaURLs)))
		for i, u := range export.MediaURLs {
			if i == 25 {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("… und %d weitere", len(export.MediaURLs)-25))
				break
			}
			report.Warnings = append(report.Warnings, u)
		}
	}
	if export.Skipped > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%d Einträge waren keine Seiten oder Beiträge (Anhänge, Menüpunkte, Papierkorb) "+
				"und wurden übergangen.", export.Skipped))
	}
	if export.Truncated {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"Die Datei enthält mehr als %d Einträge. Der Rest wurde nicht eingelesen.", wxr.MaxItems))
	}

	h.resolver.InvalidateCache()

	data := ImportReportData{
		LayoutData: web.NewLayoutData(r, h.sm, "Import abgeschlossen"),
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
		title = "Ohne Titel"
	}

	slug := page.Slugify(item.Slug)
	if slug == "" {
		slug = page.Slugify(title)
	}
	if err := page.ValidateSlug(slug); err != nil {
		return fmt.Sprintf("%q: die Adresse %q ist nicht zulässig", title, item.Slug)
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
		return fmt.Sprintf("%q konnte nicht gesetzt werden: %v", title, err)
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
		return fmt.Sprintf("%q konnte nicht angelegt werden: %v", title, err)
	}

	if len(item.Terms) > 0 && h.terms != nil {
		if err := h.terms.SetForPage(r.Context(), websiteID, created.ID, item.Terms); err != nil {
			return fmt.Sprintf("Schlagwörter von %q: %v", title, err)
		}
	}
	return ""
}
