package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	tmpl2 "github.com/holzcloud/holzcloud-cms/internal/template"
)

// The template screens: upload, activate, deactivate, delete, and the
// specification an agent is meant to copy whole.
//
// The upload is the one screen of this administration that takes an archive
// from outside and writes it to disk, so what it refuses is the point of it.
// Every refusal below already had a rule written into the handler — T-04-07 for
// the size, T-04-08 for deleting something in use, the empty slug that would
// have made the extraction target the templates root — and none of them had a
// test.
//
// Driven red by twenty mutations, nineteen caught. The survivor is recorded at
// TestAPartialThemeInstallsAndTheRestFallsBack: passing nil instead of the
// fallback FS changes what the acceptance check does with the views an archive
// omits, and no archive written here is distinguishable either way. That check
// is held in internal/template's check_test.go against the real default theme.
//
// Writing these also found that newTestAdmin built the loader with a nil
// template resolver, so the loader in EVERY admin test fell back to the shipped
// theme and could not see which theme a website runs. A test could activate one
// and then render another with nothing noticing. It is wired the way main.go
// wires it now, which is what makes the two render assertions below possible.

// themeZip builds the smallest archive ExtractTemplate accepts, plus whatever
// is handed in. It has to RENDER, not merely exist: the extraction renders what
// it is given before accepting it, so a placeholder would test the render check
// instead of the thing each case is about.
func themeZip(t *testing.T, extra map[string]string) []byte {
	t.Helper()
	files := map[string]string{
		"layout.html": `<html lang="{{.Site.Locale}}"><body>{{block "content" .}}{{end}}</body></html>`,
		"page.html":   `{{define "content"}}<h1>{{.Page.Title}}</h1>{{.Page.ContentHTML}}{{end}}`,
	}
	for k, v := range extra {
		files[k] = v
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// templateUpload is the multipart POST the upload form sends. An empty filename
// means no file part at all.
func templateUpload(t *testing.T, name, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("name", name); err != nil {
		t.Fatal(err)
	}
	if filename != "" {
		part, err := w.CreateFormFile("template_file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/templates/upload", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func templateAdmin(t *testing.T) (*Handler, *scs.SessionManager, *db.DB, int64) {
	t.Helper()
	h, sm, database, ws := newTestAdmin(t)
	h.cfg.MaxTemplateSize = 10 << 20
	return h, sm, database, ws.ID
}

func TestATemplateIsUploadedAndAppears(t *testing.T) {
	h, sm, _, _ := templateAdmin(t)
	ctx := context.Background()

	rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "  Werkstatt  ", "werkstatt.zip", themeZip(t, map[string]string{
			"style.css": "body{}",
		})))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}

	stored, err := h.tmplStore.GetBySlug(ctx, "werkstatt")
	if err != nil || stored == nil {
		t.Fatalf("the template was not recorded: %v", err)
	}
	if stored.Name != "Werkstatt" {
		t.Errorf("the name was not trimmed: %q", stored.Name)
	}
	// And on disk, under the slug and not under whatever the file was called.
	for _, name := range []string{"layout.html", "page.html", "style.css"} {
		p := filepath.Join(h.cfg.DataDir, "templates", "werkstatt", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s is not on disk: %v", name, err)
		}
	}

	// It is on the list screen.
	rec = serve(t, h, sm, h.HandleTemplateList,
		httptest.NewRequest(http.MethodGet, "/admin/templates", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Werkstatt") {
		t.Error("the uploaded template is not on the list")
	}
}

// What the upload refuses, and why each one is there.
func TestWhatTheTemplateUploadRefuses(t *testing.T) {
	h, sm, _, _ := templateAdmin(t)
	ctx := context.Background()
	good := themeZip(t, nil)

	// An existing template, so the duplicate case has something to collide
	// with.
	rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Werkstatt", "w.zip", good))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("the first upload: status %d — %q", rec.Code, bad)
	}

	for _, c := range []struct {
		what, name, filename string
		data                 []byte
		wantFlash            string
	}{
		{"no name", "", "w.zip", good, "Please give the template a name"},
		{"a name of only spaces", "   ", "w.zip", good, "Please give the template a name"},
		{"no file at all", "Ohne Datei", "", nil, "Please choose a .zip file to upload"},
		{"a file that is not a .zip", "Kein Zip", "theme.tar.gz", good, "Only .zip files are accepted"},
		// The case that matters most: a name with no usable characters would
		// produce an empty slug, the destination would be the templates ROOT,
		// and the extraction removes its destination before renaming the temp
		// directory into place — so every installed template would go.
		{"a name with no usable characters", "!!! ???", "w.zip", good,
			"The template name must contain letters or digits"},
		{"a name that is already taken", "Werkstatt", "w.zip", good,
			"A template with that name already exists"},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateUpload,
			templateUpload(t, c.name, c.filename, c.data))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad != c.wantFlash {
			t.Errorf("%s: answered %q, want %q", c.what, bad, c.wantFlash)
		}
	}

	// Exactly one template exists afterwards, so none of the refusals wrote
	// anything.
	list, err := h.tmplStore.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var uploaded int
	for _, tmpl := range list {
		if !tmpl.IsBuiltin {
			uploaded++
		}
	}
	if uploaded != 1 {
		t.Errorf("%d uploaded templates after six refusals", uploaded)
	}
	// And the templates directory has exactly the one folder — the empty-slug
	// case did not turn the root into a destination.
	entries, err := os.ReadDir(filepath.Join(h.cfg.DataDir, "templates"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "werkstatt" {
		t.Errorf("the templates directory holds %v", entries)
	}
}

// An archive that breaks the rules of a template is refused with the reason,
// and nothing is left behind on disk. The rules themselves belong to
// internal/tmplmgr and are tested there; what is tested here is that the screen
// carries the refusal through instead of installing a broken theme.
func TestATemplateThatBreaksTheRulesIsRefusedWithItsReason(t *testing.T) {
	h, sm, _, _ := templateAdmin(t)
	ctx := context.Background()

	for _, c := range []struct{ what, file, body string }{
		{"a script element", "page.html",
			`{{define "content"}}<script>alert(1)</script>{{end}}`},
		{"a stylesheet from another server", "layout.html",
			`<html><head><link rel="stylesheet" href="https://cdn.example/x.css"></head>` +
				`<body>{{block "content" .}}{{end}}</body></html>`},
		{"a template that does not render", "page.html",
			`{{define "content"}}{{.Page.GibtEsNicht}}{{end}}`},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateUpload,
			templateUpload(t, "Kaputt", "k.zip", themeZip(t, map[string]string{c.file: c.body})))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if !strings.Contains(bad, "Invalid template") && !strings.Contains(bad, "Ungültige") {
			t.Errorf("%s: answered %q", c.what, bad)
		}
		if stored, _ := h.tmplStore.GetBySlug(ctx, "kaputt"); stored != nil {
			t.Errorf("%s: the broken template was recorded anyway", c.what)
		}
		if _, err := os.Stat(filepath.Join(h.cfg.DataDir, "templates", "kaputt")); err == nil {
			t.Errorf("%s: the broken template was left on disk", c.what)
		}
	}
}

func TestActivatingATemplateSwitchesTheWebsiteOver(t *testing.T) {
	h, sm, _, websiteID := templateAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Werkstatt", "w.zip", themeZip(t, map[string]string{
			"page.html": `{{define "content"}}<h1>WERKSTATT-THEME {{.Page.Title}}</h1>{{end}}`,
		})))
	tmpl, err := h.tmplStore.GetBySlug(ctx, "werkstatt")
	if err != nil || tmpl == nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	id := strconv.FormatInt(tmpl.ID, 10)
	form := url.Values{"website_id": {strconv.FormatInt(websiteID, 10)}}

	rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateActivate,
		postForm("/admin/templates/1/activate", form, map[string]string{"id": id}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	active, err := h.tmplStore.ActiveByWebsite(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active[websiteID] != tmpl.ID {
		t.Errorf("the website runs template %d, want %d", active[websiteID], tmpl.ID)
	}

	// And the public side sees it at once. The loader keeps a parsed template
	// set per website, so without dropping that cache a theme switch shows up
	// only after a restart — the same cache-of-a-decision the resolver has, in
	// a different layer.
	//
	// Rendered through the loader rather than asserted on a flag, because the
	// flag is what was already checked above and the cache is what this half is
	// about.
	before, err := h.loader.RenderPage(ctx, websiteID, "page.html", tmpl2.SampleData())
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if !strings.Contains(string(before), "WERKSTATT-THEME") {
		t.Fatalf("the activated theme did not render: %.200q", before)
	}

	serve(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Zweitthema", "z.zip", themeZip(t, map[string]string{
			"page.html": `{{define "content"}}<h1>ZWEIT-THEME {{.Page.Title}}</h1>{{end}}`,
		})))
	zweit, err := h.tmplStore.GetBySlug(ctx, "zweitthema")
	if err != nil || zweit == nil {
		t.Fatalf("GetBySlug zweitthema: %v", err)
	}
	serve(t, h, sm, h.HandleTemplateActivate, postForm("/admin/templates/2/activate",
		form, map[string]string{"id": strconv.FormatInt(zweit.ID, 10)}))

	after, err := h.loader.RenderPage(ctx, websiteID, "page.html", tmpl2.SampleData())
	if err != nil {
		t.Fatalf("RenderPage after the switch: %v", err)
	}
	if !strings.Contains(string(after), "ZWEIT-THEME") {
		t.Errorf("the website still renders the old theme — the template cache "+
			"was not dropped, so the switch would take effect only on restart: %.200q", after)
	}

	// Deactivated again, it runs none.
	rec = serve(t, h, sm, h.HandleTemplateDeactivate,
		postForm("/admin/templates/1/deactivate", form, map[string]string{"id": id}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("deactivate: status %d", rec.Code)
	}
	active, _ = h.tmplStore.ActiveByWebsite(ctx)
	if active[websiteID] == tmpl.ID {
		t.Error("the template is still active after being deactivated")
	}
	_ = zweit
}

// Both ids arrive as FORM values here rather than as path segments, so nothing
// upstream has checked that they name anything. requireWebsiteAndTemplate is
// that check, and this is the case it exists for.
// An archive carrying only the two required files is a conforming upload, and
// the rest is promised to fall back to the shipped theme. It installs, it
// activates, and the views it does not carry still render.
//
// This is also where the upload's `fallback` argument earns its keep, and why a
// mutation that passes nil instead is not distinguishable here: it changes what
// the acceptance check does with the views an archive OMITS, and an archive
// that omits nothing dangerous is accepted either way. That check is held one
// layer down, in internal/template's check_test.go, against the real default
// theme.
func TestAPartialThemeInstallsAndTheRestFallsBack(t *testing.T) {
	h, sm, _, websiteID := templateAdmin(t)
	ctx := context.Background()

	rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Knapp", "k.zip", themeZip(t, map[string]string{
			"page.html": `{{define "content"}}<article class="knapp">{{.Page.Title}}</article>{{end}}`,
		})))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	tmpl, err := h.tmplStore.GetBySlug(ctx, "knapp")
	if err != nil || tmpl == nil {
		t.Fatalf("the partial theme was refused: %v — %q", err, bad)
	}
	serve(t, h, sm, h.HandleTemplateActivate, postForm("/admin/templates/1/activate",
		url.Values{"website_id": {strconv.FormatInt(websiteID, 10)}},
		map[string]string{"id": strconv.FormatInt(tmpl.ID, 10)}))

	// Its own view is its own.
	own, err := h.loader.RenderPage(ctx, websiteID, "page.html", tmpl2.SampleData())
	if err != nil {
		t.Fatalf("page view: %v", err)
	}
	if !strings.Contains(string(own), `class="knapp"`) {
		t.Errorf("the theme's own view did not render: %.200q", own)
	}

	// A view it does not carry comes from the shipped theme rather than 500.
	borrowed, err := h.loader.RenderPage(ctx, websiteID, "home.html", tmpl2.SampleData())
	if err != nil {
		t.Fatalf("a view the theme does not carry: %v", err)
	}
	if len(borrowed) == 0 {
		t.Error("the borrowed view rendered nothing")
	}
}

func TestActivatingRefusesIdsThatNameNothing(t *testing.T) {
	h, sm, _, websiteID := templateAdmin(t)
	ctx := context.Background()

	serve(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Werkstatt", "w.zip", themeZip(t, nil)))
	tmpl, _ := h.tmplStore.GetBySlug(ctx, "werkstatt")

	for _, c := range []struct {
		what, templateID string
		form             url.Values
	}{
		{"no website at all", strconv.FormatInt(tmpl.ID, 10), url.Values{}},
		{"a website id that is not a number", strconv.FormatInt(tmpl.ID, 10),
			url.Values{"website_id": {"keine-zahl"}}},
		{"a website that does not exist", strconv.FormatInt(tmpl.ID, 10),
			url.Values{"website_id": {"999"}}},
		{"a template that does not exist", "999",
			url.Values{"website_id": {strconv.FormatInt(websiteID, 10)}}},
	} {
		rec, bad, good := albumFlash(t, h, sm, h.HandleTemplateActivate,
			postForm("/admin/templates/x/activate", c.form,
				map[string]string{"id": c.templateID}))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad == "" {
			t.Errorf("%s: accepted in silence (success flash %q)", c.what, good)
		}
	}

	// Nothing was activated by any of them.
	active, _ := h.tmplStore.ActiveByWebsite(ctx)
	if active[websiteID] != 0 {
		t.Errorf("the website ended up running template %d", active[websiteID])
	}
}

// Two things may not be deleted, and both refusals are a sentence rather than a
// failure: a template that ships with the binary, and one a website is running
// (T-04-08 — deleting it would leave that site with no theme at all).
func TestWhatMayNotBeDeletedIsNotDeleted(t *testing.T) {
	h, sm, _, websiteID := templateAdmin(t)
	ctx := context.Background()

	// The built-in row is written at start-up by main.go, not by a migration,
	// so a test database has none until it is asked for.
	builtin, err := h.tmplStore.CreateBuiltin(ctx, "Standard", "default")
	if err != nil || builtin == nil {
		t.Fatalf("CreateBuiltin: %v", err)
	}
	rec, bad, _ := albumFlash(t, h, sm, h.HandleTemplateDelete,
		postForm("/admin/templates/1/delete", nil,
			map[string]string{"id": strconv.FormatInt(builtin.ID, 10)}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if bad != "Built-in templates cannot be deleted." {
		t.Errorf("the built-in was answered with %q", bad)
	}
	if still, _ := h.tmplStore.GetBySlug(ctx, "default"); still == nil {
		t.Fatal("the built-in template was deleted")
	}

	// One that is in use.
	serve(t, h, sm, h.HandleTemplateUpload,
		templateUpload(t, "Werkstatt", "w.zip", themeZip(t, nil)))
	tmpl, _ := h.tmplStore.GetBySlug(ctx, "werkstatt")
	id := strconv.FormatInt(tmpl.ID, 10)
	serve(t, h, sm, h.HandleTemplateActivate, postForm("/admin/templates/1/activate",
		url.Values{"website_id": {strconv.FormatInt(websiteID, 10)}}, map[string]string{"id": id}))

	rec, bad, _ = albumFlash(t, h, sm, h.HandleTemplateDelete,
		postForm("/admin/templates/1/delete", nil, map[string]string{"id": id}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(bad, "active on a website") && !strings.Contains(bad, "aktiv") {
		t.Errorf("a template in use was answered with %q", bad)
	}
	if still, _ := h.tmplStore.GetBySlug(ctx, "werkstatt"); still == nil {
		t.Fatal("a template a website is running was deleted")
	}

	// Freed, it goes — and its files go with it.
	serve(t, h, sm, h.HandleTemplateDeactivate, postForm("/admin/templates/1/deactivate",
		url.Values{"website_id": {strconv.FormatInt(websiteID, 10)}}, map[string]string{"id": id}))
	rec = serve(t, h, sm, h.HandleTemplateDelete,
		postForm("/admin/templates/1/delete", nil, map[string]string{"id": id}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("the freed delete: status %d", rec.Code)
	}
	if still, _ := h.tmplStore.GetBySlug(ctx, "werkstatt"); still != nil {
		t.Error("the template is still recorded")
	}

	// An id that names nothing is a 404 and not a 500.
	rec = serve(t, h, sm, h.HandleTemplateDelete,
		postForm("/admin/templates/999/delete", nil, map[string]string{"id": "999"}))
	if rec.Code != http.StatusNotFound {
		t.Errorf("an unknown template: status %d, want 404", rec.Code)
	}
}

// The specification is served as plain text on purpose: what it is for is being
// copied whole into a conversation with an agent, and Markdown turned into HTML
// has to be turned back first.
func TestTheSpecificationIsServedAsTextToBeCopied(t *testing.T) {
	h, sm, _, _ := templateAdmin(t)

	rec := serve(t, h, sm, h.HandleTemplateSpec,
		httptest.NewRequest(http.MethodGet, "/admin/templates/spec", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "TEMPLATE-SPEC.md") {
		t.Errorf("Content-Disposition %q — a browser's save-as would produce nothing recognisable", got)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(strings.TrimSpace(body), "#") {
		t.Errorf("the specification does not arrive as Markdown: %.60q", body)
	}
	// It is the real document and not an empty file.
	if len(body) < 2000 {
		t.Errorf("the specification is %d bytes long", len(body))
	}
}
