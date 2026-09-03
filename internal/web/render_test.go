package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"time"
)

// adminTemplateFS is the real on-disk admin template directory. Parsing it here
// means a broken template fails the test suite rather than only at startup.
func adminTemplateFS(t *testing.T) *PageTemplates {
	t.Helper()
	pt, err := ParseAdminTemplates(os.DirFS("../../cmd/holzcloud/templates/admin"))
	if err != nil {
		t.Fatalf("ParseAdminTemplates: %v", err)
	}
	return pt
}

func TestParseAdminTemplatesSucceeds(t *testing.T) {
	adminTemplateFS(t)
}

// Test doubles mirroring the shapes the admin package passes to the partials.
type testPage struct {
	ID          int64
	Title       string
	Slug        string
	Status      string
	ReviewState string
	PublishedAt *time.Time
	PublishAt   *time.Time
	UpdatedAt   time.Time
}

// Scheduled, Expired and IsPost mirror page.Page, which the row template calls
// to decide which badges to show.
func (p testPage) Scheduled() bool { return false }
func (p testPage) Expired() bool   { return false }
func (p testPage) IsPost() bool    { return false }
func (p testPage) IsOwnKind() bool { return false }

// req is a bare request, for the partials: they only need it for the language,
// and a test that says nothing about language wants German.
func req() *http.Request { return httptest.NewRequest("GET", "/admin/", nil) }

type testPageRow struct {
	WebsiteID  int64
	Page       testPage
	CSRFToken  string
	Snippet    template.HTML
	Language   string
	MayPublish bool
	Columns    testColumns
	KindName   string
}

// testColumns steht für admin.ColumnSet: die Zeile fragt sie nach jeder
// wählbaren Spalte.
type testColumns struct{}

func (testColumns) Has(string) bool { return true }

type testDomain struct {
	ID        int64
	Domain    string
	IsPrimary bool
}

// Display mirrors domain.Domain.Display, which the partial calls to show the
// readable Unicode form of an internationalised name.
func (d testDomain) Display() string { return d.Domain }

type testDomainList struct {
	WebsiteID int64
	Domains   []testDomain
	CSRFToken string
}

// A page title is user input. It used to be concatenated into the row HTML by
// hand, which let an editor plant script that fired in an admin's browser.
func TestPageRowPartialEscapesUserInput(t *testing.T) {
	pt := adminTemplateFS(t)
	rec := httptest.NewRecorder()

	data := testPageRow{
		WebsiteID: 1,
		Page: testPage{
			ID:     7,
			Title:  `<img src=x onerror="alert(1)">`,
			Slug:   `"><script>alert(2)</script>`,
			Status: "draft",
		},
		CSRFToken: "token-123",
	}

	if err := RenderPartial(rec, pt, req(), "page_row", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := rec.Body.String()

	for _, forbidden := range []string{"<img src=x", "<script>alert(2)</script>", `onerror="alert(1)"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("unescaped payload %q in output:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, "&lt;img") {
		t.Errorf("expected the title to appear escaped:\n%s", body)
	}
	if !strings.Contains(body, `value="token-123"`) {
		t.Errorf("expected the CSRF token to be rendered:\n%s", body)
	}
}

// Domain names are admin input but were also concatenated by hand.
func TestDomainListPartialEscapesUserInput(t *testing.T) {
	pt := adminTemplateFS(t)
	rec := httptest.NewRecorder()

	data := testDomainList{
		WebsiteID: 1,
		Domains:   []testDomain{{ID: 3, Domain: `evil.com"><script>alert(1)</script>`, IsPrimary: true}},
	}

	if err := RenderPartial(rec, pt, req(), "domain_list", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := rec.Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("unescaped domain name in output:\n%s", body)
	}
	if !strings.Contains(body, "badge--primary") {
		t.Errorf("expected the primary badge to render:\n%s", body)
	}
}

func TestDomainListPartialHandlesEmptyList(t *testing.T) {
	pt := adminTemplateFS(t)
	rec := httptest.NewRecorder()

	if err := RenderPartial(rec, pt, req(), "domain_list", testDomainList{WebsiteID: 1}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "Noch keine Domain zugeordnet") {
		t.Errorf("expected the empty-state message:\n%s", rec.Body.String())
	}
}

func TestRenderPartialSetsVaryHeader(t *testing.T) {
	pt := adminTemplateFS(t)
	rec := httptest.NewRecorder()

	if err := RenderPartial(rec, pt, req(), "domain_list", testDomainList{WebsiteID: 1}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if got := rec.Header().Get("Vary"); got != "HX-Request" {
		t.Errorf("Vary = %q; want HX-Request", got)
	}
}

// testDashboard is the shape the dashboard template reads. It lives here and
// not in the admin package for the same reason as the other doubles: this test
// is about the rendering, not about what fills the numbers in.
type testDashboard struct {
	LayoutData
	WebsiteCount int
	PageCount    int
	MediaCount   int
	Websites     []struct {
		ID                                int64
		Name, TemplateName                string
		PageCount, MediaCount, OpenChecks int
	}
	RecentPages []struct {
		ID                 int64
		WebsiteID          int64
		Title, WebsiteName string
		UpdatedAt          time.Time
	}
}

// The real templates, rendered in both languages.
//
// It is the one check that the whole chain holds: the catalogue is embedded,
// the set is parsed once per language, and the request's language picks the
// right one. A broken link anywhere in that chain shows up here as a German
// screen where an English one was asked for.
func TestAdminRendersInTheRequestsLanguage(t *testing.T) {
	pt := adminTemplateFS(t)

	render := func(lang string) string {
		r := httptest.NewRequest("GET", "/admin/", nil)
		r = r.WithContext(i18n.WithLang(r.Context(), lang))
		rec := httptest.NewRecorder()
		data := testDashboard{LayoutData: LayoutData{Title: "Übersicht", Lang: lang, UserEmail: "eva@example.com"}}
		if err := RenderAdmin(rec, pt, r, "dashboard", data); err != nil {
			t.Fatalf("RenderAdmin(%s): %v", lang, err)
		}
		return rec.Body.String()
	}

	german := render("de")
	english := render("en")

	if !strings.Contains(german, ">Seiten<") {
		t.Errorf("the German dashboard has no German navigation:\n%s", german[:800])
	}
	if !strings.Contains(english, ">Pages<") {
		t.Errorf("the English dashboard was not translated:\n%s", english[:800])
	}
	if strings.Contains(english, ">Schlagwörter<") {
		t.Error("German words left in the English dashboard")
	}
	if !strings.Contains(english, `<html lang="en"`) {
		t.Error(`the English page does not announce itself as lang="en"`)
	}
	if !strings.Contains(german, `<html lang="de"`) {
		t.Error(`the German page does not announce itself as lang="de"`)
	}
}

// An unknown language must render rather than fail: a build can drop a
// language while somebody still has it stored in their account.
func TestUnknownLanguageFallsBackToGerman(t *testing.T) {
	pt := adminTemplateFS(t)
	r := httptest.NewRequest("GET", "/admin/", nil)
	r = r.WithContext(i18n.WithLang(r.Context(), "klingon"))
	rec := httptest.NewRecorder()

	if err := RenderAdmin(rec, pt, r, "dashboard", testDashboard{LayoutData: LayoutData{Title: "Übersicht"}}); err != nil {
		t.Fatalf("RenderAdmin: %v", err)
	}
	if !strings.Contains(rec.Body.String(), ">Seiten<") {
		t.Error("an unknown language did not fall back to German")
	}
}

// Der Fusszeile in der Seitenleiste liegt eine Pflicht zugrunde, keine
// Verzierung: Abschnitt 13 der AGPL verlangt von jemandem, der eine veränderte
// Fassung als Netzdienst betreibt, dass er den Benutzenden den Quelltext
// anbietet. Der benannte Fehlerfall ist eine Fusszeile, die als leere Lücke
// erscheint, weil ein Feld nie gelesen wurde — deshalb prüft das hier die ganze
// Kette von LayoutData bis ins fertige HTML und nicht nur das Vorhandensein
// einer Regel im Stylesheet.
func TestSidebarFooterShowsBuildAndLicence(t *testing.T) {
	pt := adminTemplateFS(t)
	rec := httptest.NewRecorder()

	data := testDashboard{LayoutData: LayoutData{
		Title:     "Übersicht",
		Version:   "v9.9.9-test",
		SourceURL: "https://example.invalid/quelle",
	}}
	if err := RenderAdmin(rec, pt, req(), "dashboard", data); err != nil {
		t.Fatalf("RenderAdmin: %v", err)
	}
	body := rec.Body.String()

	// 1 — die Fassung erreicht die Seite.
	if !strings.Contains(body, "Holzcloud CMS") {
		t.Error("die Fusszeile nennt das Programm nicht")
	}
	if !strings.Contains(body, "v9.9.9-test") {
		t.Error("die Fassung aus LayoutData.Version steht nicht auf der Seite")
	}
	// 2 — die Lizenz ist benannt.
	if !strings.Contains(body, "AGPL") {
		t.Error("die Lizenz wird nirgends genannt")
	}
	// 3 — das Angebot ist ein Link und kein blosser Text.
	if !strings.Contains(body, `href="https://example.invalid/quelle"`) {
		t.Error("die Quelltextadresse ist kein Verweisziel")
	}
}

// Ein ohne ldflags gebautes Programm muss trotzdem etwas anzeigen: „dev“ und
// die vorgegebene Adresse sind besser als zwei leere Stellen. Der Test steht im
// Paket web, also sind beide Paketvariablen erreichbar.
func TestBuildStampNeverEmpty(t *testing.T) {
	if buildVersion == "" || buildSource == "" {
		t.Fatalf("Vorgabe leer: buildVersion=%q buildSource=%q", buildVersion, buildSource)
	}
	SetBuild("", "")
	if buildVersion == "" || buildSource == "" {
		t.Errorf("SetBuild(\"\", \"\") hat die Vorgabe gelöscht: buildVersion=%q buildSource=%q", buildVersion, buildSource)
	}
}
