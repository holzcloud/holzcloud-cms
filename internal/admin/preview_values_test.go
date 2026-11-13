package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// The preview answers "what will a visitor read", and a visitor reads the
// value's text, not [[wert:v]]. A draft is the realistic case: the editor
// writes the release note before publishing it and looks at it first.
func TestThePreviewShowsTheValue(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()
	values := value.NewStore(database)

	other, err := domain.NewStore(database).CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if _, _, err := values.Set(ctx, ws.ID, "v", "0.1.5"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, _, err := values.Set(ctx, other.ID, "v", "fremd"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, _, err := values.Set(ctx, other.ID, "nur-andere", "geheim"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	seedPage(t, database, ws.ID, "Entwurf", "entwurf",
		"Alpha [[wert:v]]: neu\n\nLeer [[wert:gibt-es-nicht]].\n\nFremd [[wert:nur-andere]].", "draft")

	target := fmt.Sprintf("/admin/websites/%d/preview/entwurf", ws.ID)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("id", id64(ws.ID))
	req.SetPathValue("slug", "entwurf")
	rec := httptest.NewRecorder()
	if err := h.HandlePreviewPage(rec, req); err != nil {
		t.Fatalf("HandlePreviewPage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	if strings.Contains(body, "[[wert:") {
		t.Errorf("the preview shows the internal syntax where the value belongs:\n%s", body)
	}
	if !strings.Contains(body, "Alpha 0.1.5: neu") {
		t.Errorf("the preview does not show the value's text:\n%s", body)
	}
	if !strings.Contains(body, "Leer .") {
		t.Errorf("an undefined key must show as nothing in the preview, as it does on the page:\n%s", body)
	}
	if strings.Contains(body, "fremd") || strings.Contains(body, "geheim") {
		t.Errorf("the preview used another website's value:\n%s", body)
	}
	if !strings.Contains(body, "Fremd .") {
		t.Errorf("a key only another website defines must show as nothing:\n%s", body)
	}
}

// The preview's head says what the public head will say: the summary derived
// from the page's text carries the value there too, and the marker it keeps
// for delivery is not shown.
func TestThePreviewSummaryShowsTheValue(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	ctx := context.Background()
	if _, _, err := value.NewStore(database).Set(ctx, ws.ID, "v", "0.1.5"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	const markdown = "Alpha [[wert:v]]: neu\n\nMehr Text."
	html, err := page.RenderMarkdown(markdown)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if _, err := page.NewStore(database).CreatePage(ctx, page.PageCreate{
		WebsiteID: ws.ID, Title: "Entwurf", Slug: "entwurf", Markdown: markdown, HTML: html,
		Status: "draft", Meta: page.PageMeta{Excerpt: page.Excerpt(markdown)},
	}); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d/preview/entwurf", ws.ID), nil)
	req.SetPathValue("id", id64(ws.ID))
	req.SetPathValue("slug", "entwurf")
	rec := httptest.NewRecorder()
	if err := h.HandlePreviewPage(rec, req); err != nil {
		t.Fatalf("HandlePreviewPage: %v", err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "[[wert:") {
		t.Errorf("the preview shows the internal syntax where the value belongs:\n%s", body)
	}
	if !strings.Contains(body, `<meta name="description" content="Alpha 0.1.5: neu Mehr Text.">`) {
		t.Errorf("the preview's meta description lacks the value:\n%.2000s", body)
	}
}
