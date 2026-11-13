package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/seo"
)

// The SEO checks and values: a page that names a key its website does not
// define is told so in the editor's tab, on the report screen and over MCP.
// Helpers carry the seoValue prefix: other plans add files to this package.

// seoValuePage creates a published page whose text is seoBody with extra
// appended, so the only findings worth looking at are the value ones.
func seoValuePage(t *testing.T, database *db.DB, websiteID int64, title, slug, extra string) *page.Page {
	t.Helper()
	md := seoBody + "\n\n" + extra
	html, err := page.RenderMarkdown(md)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	p, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: title, Slug: slug, Markdown: md, HTML: html, Status: "published",
		Meta: page.PageMeta{MetaDescription: goodDesc},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	return p
}

// seoValueFindings picks the findings of one code.
func seoValueFindings(fs []seo.Finding, code string) []seo.Finding {
	var out []seo.Finding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

func TestTheSEOTabReportsAnUnknownValue(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	other, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.valueStore.Create(ctx, ws.ID, "v", "0.1.5"); err != nil {
		t.Fatal(err)
	}
	// Another website's definition does not count for this one.
	if _, err := h.valueStore.Create(ctx, other.ID, "nope", "fremd"); err != nil {
		t.Fatal(err)
	}
	p := seoValuePage(t, database, ws.ID, goodTitle, "version",
		"Die Version ist [[wert:v]], der Rest [[wert:nope]].")

	panel := h.seoPanelFor(seoEditRequest(ws.ID, p.ID), p)
	if panel == nil {
		t.Fatal("no SEO panel")
	}
	got := seoValueFindings(panel.Findings, seo.CodeUnknownValue)
	if len(got) != 1 || got[0].Detail != "nope" || got[0].Severity != seo.SeverityWarn {
		t.Fatalf("unknown-value findings = %+v, want one warn naming nope", got)
	}
	if panel.Status != "warn" {
		t.Errorf("status = %q, want warn", panel.Status)
	}

	body := serve(t, h, sm, h.HandlePageEdit, seoEditRequest(ws.ID, p.ID)).Body.String()
	for _, want := range []string{"Unknown value", "The text uses the value “nope”"} {
		if !strings.Contains(body, want) {
			t.Errorf("the SEO tab lacks %q", want)
		}
	}
	if strings.Contains(body, "the value “v”") {
		t.Error("a defined value is reported")
	}
}

// seoValueReportPage finds one page in a seo_report answer and gives back its
// findings as code → details.
func seoValueReportPage(t *testing.T, out map[string]any, pageID int64) map[string][]string {
	t.Helper()
	pages, _ := out["pages"].([]any)
	for _, it := range pages {
		row := it.(map[string]any)
		if id, _ := row["id"].(float64); int64(id) != pageID {
			continue
		}
		got := map[string][]string{}
		fs, _ := row["findings"].([]any)
		for _, f := range fs {
			m := f.(map[string]any)
			code, _ := m["code"].(string)
			detail, _ := m["detail"].(string)
			got[code] = append(got[code], detail)
		}
		return got
	}
	t.Fatalf("page %d is not in the seo_report answer: %v", pageID, out)
	return nil
}

func TestSeoReportOverMCPReportsAnUnknownValue(t *testing.T) {
	r := newStructureRig(t)
	ctx := context.Background()
	if _, err := r.h.valueStore.Create(ctx, r.site, "v", "0.1.5"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.h.valueStore.Create(ctx, r.other, "nope", "fremd"); err != nil {
		t.Fatal(err)
	}
	p := mcpValuePage(r, r.site, "Herunterladen", "herunterladen", "published", "",
		"Die Version ist [[wert:v]], der Rest [[wert:nope]].")

	out := r.must(r.content, "seo_report", map[string]any{"website": r.site})
	got := seoValueReportPage(t, out, p.ID)
	if d := got[seo.CodeUnknownValue]; len(d) != 1 || d[0] != "nope" {
		t.Fatalf("seo_report unknown-value = %v, want exactly nope; findings %v", d, got)
	}

	// The same finding the editor tab shows for the same page: the two audit
	// sources are built in two places and must carry the same functions.
	panel := r.h.seoPanelFor(seoEditRequest(r.site, p.ID), p)
	if panel == nil {
		t.Fatal("no SEO panel")
	}
	tab := seoValueFindings(panel.Findings, seo.CodeUnknownValue)
	if len(tab) != 1 || tab[0].Detail != "nope" {
		t.Errorf("the editor tab says %+v, seo_report says %v", tab, got[seo.CodeUnknownValue])
	}
}

func TestTheSiteReportFiltersUnknownValues(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	if _, err := h.valueStore.Create(context.Background(), ws.ID, "v", "0.1.5"); err != nil {
		t.Fatal(err)
	}
	unknown := seoValuePage(t, database, ws.ID, "Seite mit fehlendem Wert", "fehlt", "Es fehlt [[wert:nope]].")
	clean := seoValuePage(t, database, ws.ID, "Seite ohne Platzhalter", "ohne", "")
	titled := seoValuePage(t, database, ws.ID, "Version [[wert:v]] im Titel", "titel", "Version [[wert:v]].")

	rows := func(query string) string {
		return serve(t, h, sm, h.HandleSEOReport, seoReportRequest(ws.ID, query)).Body.String()
	}
	listed := func(body string, p *page.Page) bool { return strings.Contains(body, ">"+p.Title+"<") }

	body := rows("?code=unknown-value")
	if !listed(body, unknown) || listed(body, clean) || listed(body, titled) {
		t.Error("the filter by unknown-value does not narrow the list to the page naming an undefined key")
	}
	if !strings.Contains(body, `value="unknown-value" selected`) || !strings.Contains(body, "Unknown value") {
		t.Error("unknown-value is not offered and selected in the filter by its label")
	}

	body = rows("?code=value-outside-text")
	if !listed(body, titled) || listed(body, unknown) || listed(body, clean) {
		t.Error("the filter by value-outside-text does not narrow the list to the page with the marker in its title")
	}
	if !strings.Contains(body, "Value in title or description") {
		t.Error("value-outside-text has no label in the filter")
	}
}
