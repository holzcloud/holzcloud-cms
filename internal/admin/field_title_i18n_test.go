package admin

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// Plan 10-10's browser pass found five German sentences standing on an English
// admin while `go run ./tools/i18n` reported 0 open, 0 orphaned on all four
// catalogues. Four of the five predate this milestone. This one does not:
// `git log -S 'Textbaustein „' -- internal/admin/field.go` returns
// 48e5b1d feat(08-03), 2026-09-06 — Phase 8, inside v1.6.
//
// Criterion 6 of the milestone reads "0 open, 0 orphaned across everything
// v1.6 added". The gate says yes. The screen says otherwise, and the screen is
// what the criterion is about.
//
// # Why no gate here can see it
//
// The title is built by concatenation — `"Felder – " + websiteName` — so there
// is no string literal at the argument index the collector reads, and the
// sentence is not merely untranslated but unreported: neither open nor
// orphaned, because tools/i18n does not know it exists. Its own package comment
// describes the mechanism; this is the consequence the comment stops short of.
//
// And "Felder – " carries no umlaut, no sharp s and no German quotation mark,
// so a gate hunting German-looking literals cannot find it either. It is
// invisible to every mechanical check this repository has. Only a person
// switching the language and looking finds it — which is what happened.
func TestTheFieldScreenTitleIsTranslated(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	seedPage(t, database, ws.ID, "Irgendeine Seite", "seite", "text", "draft")

	req := httptest.NewRequest(http.MethodGet,
		"/admin/websites/"+strconv.FormatInt(ws.ID, 10)+"/felder", nil)
	req.SetPathValue("id", strconv.FormatInt(ws.ID, 10))
	// The one thing this test is about: the person reading is English.
	req = req.WithContext(i18n.WithLang(req.Context(), "en"))

	rec := serve(t, h, sm, h.HandleFieldList, req)
	body := rec.Body.String()

	if strings.Contains(body, "Felder – ") {
		t.Errorf("the field screen prints its German title to a reader whose admin "+
			"is English: the <title> and <h1> carry \"Felder – \". Built by "+
			"concatenation, so tools/i18n never saw it — and with no umlaut in it, "+
			"no German-literal gate can find it either.\n  %s", titleOf(body))
	}
}

// titleOf pulls the <title> out for a readable failure message.
func titleOf(body string) string {
	i := strings.Index(body, "<title>")
	if i < 0 {
		return "(no <title>)"
	}
	j := strings.Index(body[i:], "</title>")
	if j < 0 {
		return "(unterminated <title>)"
	}
	return strings.TrimSpace(body[i+len("<title>") : i+j])
}
