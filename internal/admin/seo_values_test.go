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
