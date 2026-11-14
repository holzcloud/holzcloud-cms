package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// The opt-out widget ([[widerspruch]]) is drawn at delivery, from the
// visitor's request, so the preview cannot show the real one. What it shows
// instead is a box that says what goes there — never the marker, which no
// visitor ever reads. The public page is unchanged (internal/public's
// expandOptOut).

// setFrameworks gives the fixture website a set of legal frameworks.
func setFrameworks(t *testing.T, h *Handler, websiteID int64, set string) {
	t.Helper()
	if _, err := h.db.Write.Exec(`UPDATE websites SET legal_frameworks = ? WHERE id = ?`, set, websiteID); err != nil {
		t.Fatal(err)
	}
}

func TestTheLivePreviewShowsAPlaceholderForTheOptOutWidget(t *testing.T) {
	h, sm, _, ws := newTestAdmin(t)
	target := fmt.Sprintf("/admin/websites/%d/pages/preview", ws.ID)
	draw := func(markdown string) (string, string) {
		req := postForm(target, url.Values{"content_markdown": {markdown}}, websiteRoute(ws.ID))
		rec := serve(t, h, sm, h.HandlePagePreview, req)
		lang := i18n.Lang(req.Context())
		return rec.Body.String(), lang
	}

	setFrameworks(t, h, ws.ID, "eu,us")
	body, lang := draw("Vorher\n\n[[widerspruch]]\n\nNachher")
	if strings.Contains(body, "[[widerspruch]]") {
		t.Errorf("the live preview shows the marker:\n%s", body)
	}
	if want := i18n.T(lang, optoutPreviewSentence); !strings.Contains(body, want) {
		t.Errorf("the live preview of a US website lacks %q:\n%s", want, body)
	}
	if !strings.Contains(body, optoutPreviewMark) {
		t.Errorf("the placeholder is not marked as one:\n%s", body)
	}
	if strings.Contains(body, "<p><div") {
		t.Errorf("the placeholder sits inside a paragraph:\n%s", body)
	}

	// A marker inside a link address is removed, as on the public page; a box
	// spliced into an attribute would break the link (UF-1).
	// Markdown's own link syntax percent-encodes the brackets; raw HTML keeps
	// them, which is the case that matters.
	body, _ = draw(`<a href="/x?[[widerspruch]]">Link</a>`)
	if strings.Contains(body, "[[widerspruch]]") || strings.Contains(body, optoutPreviewMark) {
		t.Errorf("a marker in a link address was expanded or kept:\n%s", body)
	}
	if !strings.Contains(body, ">Link</a>") {
		t.Errorf("the link itself is gone:\n%s", body)
	}

	// A website without the United States in its set shows no widget, so the
	// preview says that nothing goes there, not what would.
	setFrameworks(t, h, ws.ID, "eu,ch")
	body, lang = draw("[[widerspruch]]")
	if strings.Contains(body, "[[widerspruch]]") {
		t.Errorf("the live preview shows the marker:\n%s", body)
	}
	if strings.Contains(body, i18n.T(lang, optoutPreviewSentence)) {
		t.Errorf("the preview promises a form a non-US website does not show:\n%s", body)
	}
	if want := i18n.T(lang, optoutPreviewNotOffered); !strings.Contains(body, want) {
		t.Errorf("the live preview of a non-US website lacks %q:\n%s", want, body)
	}
}

func TestTheThemedPreviewShowsAPlaceholderForTheOptOutWidget(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	setFrameworks(t, h, ws.ID, "us")
	seedPage(t, database, ws.ID, "Do Not Sell", "do-not-sell-or-share", "Text\n\n[[widerspruch]]", "draft")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d/preview/do-not-sell-or-share", ws.ID), nil)
	req.SetPathValue("id", id64(ws.ID))
	req.SetPathValue("slug", "do-not-sell-or-share")
	rec := httptest.NewRecorder()
	if err := h.HandlePreviewPage(rec, req); err != nil {
		t.Fatalf("HandlePreviewPage: %v", err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "[[widerspruch]]") {
		t.Errorf("the themed preview shows the marker:\n%s", body)
	}
	if want := i18n.T(i18n.Lang(context.Background()), optoutPreviewSentence); !strings.Contains(body, want) {
		t.Errorf("the themed preview lacks %q", want)
	}
}
