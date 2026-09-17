package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The overview an operator lands on, and the language screens behind it.
//
// Driven red by thirteen mutations, all thirteen caught. One had to be chased,
// and it is the version of the recurring lesson that matters most here: the
// first draft checked WHICH WEBSITES were named on the dashboard and not WHAT
// THE NUMBERS SAID, so removing the rule that narrows the totals changed
// nothing a test could see. A total over everything is itself information about
// a website this person may not enter.

// The dashboard's rule, written into three separate places in the handler and
// nowhere in a test: somebody responsible for certain websites sees only their
// numbers. A total over everything is information about websites this person may
// not enter, and so is the title of a page on one of them.
func TestTheDashboardCountsOnlyWhatThisPersonMayEnter(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	domains := domain.NewStore(database)

	other, err := domains.CreateWebsite(ctx, "Die andere", "")
	if err != nil {
		t.Fatal(err)
	}
	seedPage(t, database, ws.ID, "Meine Seite", "meine-seite", "x", "published")
	seedPage(t, database, other.ID, "Fremde Seite", "fremde-seite", "x", "published")

	// statCard reads one of the three numbers at the top of the screen. The
	// names below prove which websites are listed; these prove the TOTALS, and
	// they are the half a name check cannot see — a total over everything is
	// itself information about websites this person may not enter.
	statCard := func(body string, n int) int {
		const marker = `<div class="stat-card__value">`
		count := 0
		for rest := body; ; {
			i := strings.Index(rest, marker)
			if i < 0 {
				t.Fatalf("only %d stat cards on the screen, wanted card %d", count, n+1)
			}
			rest = rest[i+len(marker):]
			end := strings.Index(rest, "<")
			if end < 0 {
				t.Fatal("a stat card is not closed")
			}
			if count == n {
				value, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
				if err != nil {
					t.Fatalf("stat card %d reads %q", n, rest[:end])
				}
				return value
			}
			count++
		}
	}

	// An administrator sees both, which is the control: without it a fix that
	// showed nobody anything would pass.
	body := serve(t, h, sm, h.HandleDashboard,
		httptest.NewRequest(http.MethodGet, "/admin", nil)).Body.String()
	for _, want := range []string{"Testseite", "Die andere", "Meine Seite", "Fremde Seite"} {
		if !strings.Contains(body, want) {
			t.Errorf("an unlimited account does not see %q", want)
		}
	}
	if got := statCard(body, 0); got != 2 {
		t.Errorf("an unlimited account is told %d websites, want 2", got)
	}
	if got := statCard(body, 1); got != 2 {
		t.Errorf("an unlimited account is told %d pages, want 2", got)
	}

	// An editor assigned to one website sees that one.
	users := user.NewStore(database, cheapHashing)
	editor, err := users.Create(ctx, "Redakteurin", "red@example.com", "passwort123", user.RoleEditor)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetRights(ctx, editor, user.Rights{MayPublish: true, Websites: []int64{ws.ID}}); err != nil {
		t.Fatal(err)
	}

	rec, _ := serveAs(t, h, sm, editor, h.HandleDashboard,
		httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "Testseite") || !strings.Contains(body, "Meine Seite") {
		t.Error("the editor does not see their own website")
	}
	// Neither the other website's name nor the title of a page on it.
	if strings.Contains(body, "Die andere") {
		t.Error("the editor sees a website they may not enter")
	}
	if strings.Contains(body, "Fremde Seite") {
		t.Error("the editor sees the title of a page on a website they may not enter")
	}
	// And the numbers are theirs, not everybody's.
	if got := statCard(body, 0); got != 1 {
		t.Errorf("the editor is told %d websites, want 1", got)
	}
	if got := statCard(body, 1); got != 1 {
		t.Errorf("the editor is told %d pages, want 1 — the total is itself "+
			"information about a website they may not enter", got)
	}
}

// languageDir points the i18n package at a folder of this test's own. The
// package keeps it in a global, so these tests may not run in parallel and each
// puts it back.
func languageDir(t *testing.T) string {
	t.Helper()
	before := i18n.Dir()
	dir := t.TempDir()
	i18n.SetDir(dir)
	t.Cleanup(func() {
		i18n.SetDir(before)
		i18n.Reload()
	})
	return dir
}

func languageUploadRequest(t *testing.T, code, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if code != "" {
		if err := w.WriteField("kuerzel", code); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		part, err := w.CreateFormFile("datei", filename)
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
	req := httptest.NewRequest(http.MethodPost, "/admin/sprachen/hochladen", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestTheLanguageScreenShowsWhatIsInstalled(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	languageDir(t)

	rec := serve(t, h, sm, h.HandleLanguages,
		httptest.NewRequest(http.MethodGet, "/admin/sprachen", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	// The languages that ship with the binary are on it.
	body := rec.Body.String()
	for _, want := range []string{"de", "fr"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not mention %q", want)
		}
	}
}

// The file a translator starts from is every source string as a key with an
// empty value — deliberately not a half-translated copy of another language,
// because translating a translation compounds the mistakes.
func TestTheStarterFileIsEmptyValuesAndEverySourceString(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	rec := serve(t, h, sm, h.HandleLanguageTemplate,
		httptest.NewRequest(http.MethodGet, "/admin/sprachen/vorlage", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "sprache-vorlage.json") {
		t.Errorf("Content-Disposition %q", got)
	}

	var msgs map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("the starter file is not readable JSON: %v", err)
	}
	if len(msgs) != len(i18n.SourceStrings()) {
		t.Errorf("%d keys, want %d source strings", len(msgs), len(i18n.SourceStrings()))
	}
	for k, v := range msgs {
		if v != "" {
			t.Errorf("the starter file is pre-filled: %q = %q", k, v)
			break
		}
	}
}

// A language that ships with the binary can be downloaded; one that does not
// exist is a 404 rather than an empty file that looks like an answer.
func TestALanguageIsDownloadedOrIsNotThere(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/sprachen/fr", nil)
	req.SetPathValue("code", "fr")
	rec := serve(t, h, sm, h.HandleLanguageDownload, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var msgs map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("the download is not readable JSON: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("the download is empty")
	}

	for _, code := range []string{"xx", "klingon", i18n.Source} {
		req := httptest.NewRequest(http.MethodGet, "/admin/sprachen/x", nil)
		req.SetPathValue("code", code)
		if rec := serve(t, h, sm, h.HandleLanguageDownload, req); rec.Code != http.StatusNotFound {
			t.Errorf("%q: status %d, want 404", code, rec.Code)
		}
	}
}

// Everything is checked before anything is written: a file that would be
// refused at load time must not sit in the folder looking installed.
func TestWhatTheLanguageUploadRefusesIsNotWritten(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	dir := languageDir(t)

	good, err := json.Marshal(map[string]string{"Pages": "Paginas"})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		what, code, filename string
		data                 []byte
	}{
		{"no file at all", "es", "", nil},
		{"a tag that is not one", "nicht-ein-tag", "x.json", good},
		{"a file name that is not a tag either", "", "meine-uebersetzung.json", good},
		{"content that is not JSON", "es", "es.json", []byte("{kaputt")},
	} {
		rec, bad, _ := albumFlash(t, h, sm, h.HandleLanguageUpload,
			languageUploadRequest(t, c.code, c.filename, c.data))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d", c.what, rec.Code)
		}
		if bad == "" {
			t.Errorf("%s: accepted in silence", c.what)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a refused upload left %v in the folder", entries)
	}
}

// The round trip an operator actually makes: upload, see it, delete it again.
func TestALanguageIsInstalledAndRemovedAgain(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	dir := languageDir(t)

	data, err := json.Marshal(map[string]string{"Pages": "Paginas", "Media": "Medios"})
	if err != nil {
		t.Fatal(err)
	}
	rec, bad, good := albumFlash(t, h, sm, h.HandleLanguageUpload,
		languageUploadRequest(t, "", "es.json", data))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d — %q", rec.Code, bad)
	}
	if !strings.Contains(good, "2") {
		t.Errorf("the message does not say how many translations arrived: %q", good)
	}
	if _, err := os.Stat(filepath.Join(dir, "es.json")); err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if !i18n.FromDisk("es") {
		t.Error("the installed language was not read back in")
	}

	// And out again. A language compiled into the binary is not deletable here,
	// and says so rather than being a button that does nothing.
	req := httptest.NewRequest(http.MethodPost, "/admin/sprachen/fr/loeschen", nil)
	req.SetPathValue("code", "fr")
	_, bad, _ = albumFlash(t, h, sm, h.HandleLanguageDelete, req)
	if bad == "" {
		t.Error("deleting a built-in language was accepted in silence")
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/sprachen/es/loeschen", nil)
	req.SetPathValue("code", "es")
	rec = serve(t, h, sm, h.HandleLanguageDelete, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: status %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "es.json")); !os.IsNotExist(err) {
		t.Errorf("the file is still there: %v", err)
	}
	if i18n.FromDisk("es") {
		t.Error("the removed language is still loaded")
	}
}

// Without a folder there is nowhere to put a file, and saying so beats writing
// one into the working directory.
func TestWithoutALanguageFolderTheUploadSaysSo(t *testing.T) {
	h, sm, _, _ := newTestAdmin(t)
	before := i18n.Dir()
	i18n.SetDir("")
	t.Cleanup(func() { i18n.SetDir(before); i18n.Reload() })

	data, _ := json.Marshal(map[string]string{"Pages": "Paginas"})
	rec, bad, _ := albumFlash(t, h, sm, h.HandleLanguageUpload,
		languageUploadRequest(t, "es", "es.json", data))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status %d", rec.Code)
	}
	if bad == "" {
		t.Error("the upload was accepted with nowhere to put the file")
	}
}
