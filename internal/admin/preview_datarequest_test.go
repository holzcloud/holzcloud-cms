package admin

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// The request form ([[datenanfrage]]) is drawn at delivery with a stamp
// signed for that answer and posts to the public site, so the preview cannot
// show the real one. It shows a box that says the form goes there, in the
// operator's language, and never the marker, which no visitor ever reads.
// The public page is unchanged (internal/public's expandDataRequest).

// requestPageLive draws Markdown in the live preview for an operator who
// reads the admin in lang.
func requestPageLive(t *testing.T, h *Handler, sm *scs.SessionManager, websiteID int64, markdown, lang string) string {
	t.Helper()
	target := fmt.Sprintf("/admin/websites/%d/pages/preview", websiteID)
	req := postForm(target, url.Values{"content_markdown": {markdown}}, websiteRoute(websiteID))
	req = req.WithContext(i18n.WithLang(req.Context(), lang))
	return serve(t, h, sm, h.HandlePagePreview, req).Body.String()
}

// requestPageSays is a catalogue key in a language, escaped as a template
// prints it.
func requestPageSays(lang, key string) string {
	return template.HTMLEscapeString(i18n.T(lang, key))
}

func TestThePreviewShowsABoxForTheRequestForm(t *testing.T) {
	const heading = "Request about my data"
	const sentence = "Visitors send requests about their personal data through the form on this page: access, correction, erasure."
	h, sm, _, ws := newTestAdmin(t)

	for _, set := range []string{"", "eu,ch", "us"} {
		// The form is offered on every website, whatever its frameworks, so
		// the box is there on every website too.
		setFrameworks(t, h, ws.ID, set)
		for _, lang := range []string{"de", "en"} {
			body := requestPageLive(t, h, sm, ws.ID, "Vorher\n\n[[datenanfrage]]\n\nNachher", lang)
			where := fmt.Sprintf("set %q, %s", set, lang)
			if strings.Contains(body, "[[datenanfrage]]") {
				t.Errorf("%s: the live preview shows the marker:\n%s", where, body)
			}
			if n := strings.Count(body, dataRequestPreviewMark); n != 1 {
				t.Errorf("%s: %d boxes, want 1:\n%s", where, n, body)
			}
			for _, key := range []string{heading, sentence} {
				if want := requestPageSays(lang, key); !strings.Contains(body, want) {
					t.Errorf("%s: the box lacks %q:\n%s", where, want, body)
				}
			}
			if lang == "de" && strings.Contains(body, sentence) {
				t.Errorf("%s: the box speaks English to a German operator", where)
			}
			if strings.Contains(body, "<form") || strings.Contains(body, "data-hc-datarequest\"") || strings.Contains(body, "/datenanfrage") {
				t.Errorf("%s: the preview draws a form:\n%s", where, body)
			}
			if strings.Contains(body, "<p><div") {
				t.Errorf("%s: the box sits inside a paragraph:\n%s", where, body)
			}
			if !strings.Contains(body, "Vorher") || !strings.Contains(body, "Nachher") {
				t.Errorf("%s: the text around the marker is gone:\n%s", where, body)
			}
		}
	}

	// One form per page: a second marker draws nothing.
	body := requestPageLive(t, h, sm, ws.ID, "[[datenanfrage]]\n\nMitte\n\n[[datenanfrage]]", "de")
	if n := strings.Count(body, dataRequestPreviewMark); n != 1 {
		t.Errorf("two markers: %d boxes, want 1:\n%s", n, body)
	}
	if strings.Contains(body, "[[datenanfrage]]") {
		t.Errorf("two markers: the second one is shown:\n%s", body)
	}

	// A marker inside a link address is removed, as on the public page; a
	// box spliced into an attribute would break the link (T-33.2-56).
	body = requestPageLive(t, h, sm, ws.ID, `<a href="/x?[[datenanfrage]]">Link</a>`, "de")
	if strings.Contains(body, "[[datenanfrage]]") || strings.Contains(body, dataRequestPreviewMark) {
		t.Errorf("a marker in a link address was expanded or kept:\n%s", body)
	}
	if !strings.Contains(body, ">Link</a>") {
		t.Errorf("the link itself is gone:\n%s", body)
	}

	// A page with both markers gets both boxes, each once.
	setFrameworks(t, h, ws.ID, "us")
	body = requestPageLive(t, h, sm, ws.ID, "[[widerspruch]]\n\n[[datenanfrage]]", "de")
	if strings.Count(body, dataRequestPreviewMark) != 1 || strings.Count(body, optoutPreviewMark) != 1 {
		t.Errorf("both markers: want one box of each:\n%s", body)
	}
}

func TestTheThemedPreviewShowsABoxForTheRequestForm(t *testing.T) {
	h, _, database, ws := newTestAdmin(t)
	setFrameworks(t, h, ws.ID, "")
	seedPage(t, database, ws.ID, "Meine Daten", "meine-daten", "Text\n\n[[datenanfrage]]", "draft")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/websites/%d/preview/meine-daten", ws.ID), nil)
	req.SetPathValue("id", id64(ws.ID))
	req.SetPathValue("slug", "meine-daten")
	req = req.WithContext(i18n.WithLang(req.Context(), "de"))
	rec := httptest.NewRecorder()
	if err := h.HandlePreviewPage(rec, req); err != nil {
		t.Fatalf("HandlePreviewPage: %v", err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "[[datenanfrage]]") {
		t.Errorf("the themed preview shows the marker:\n%s", body)
	}
	if want := requestPageSays("de", "Request about my data"); !strings.Contains(body, want) || !strings.Contains(body, dataRequestPreviewMark) {
		t.Errorf("the themed preview lacks the box with %q", want)
	}
}
