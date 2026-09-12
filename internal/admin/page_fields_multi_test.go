package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// The whole way of a multi-valued field: name in the form, reading the request,
// storing, resolving, redrawing. A checkbox field is the first field value in
// this program that is not a single string — what is green here, phase 9
// inherits unchanged.
func TestMehrfachauswahlVomFormularBisZurAnzeige(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "sorten", Label: "Sorten", Kind: field.KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"},
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}
	// An ordinary text field next to it. It is the counter-check that the
	// conditional label link has not switched itself off everywhere.
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "herkunft", Label: "Herkunft", Kind: field.KindText,
	}); err != nil {
		t.Fatalf("Feld anlegen: %v", err)
	}

	p := seedPage(t, database, ws.ID, "Bretter", "bretter", "text", "draft")
	pages := page.NewStore(database)
	wsID := strconv.FormatInt(ws.ID, 10)

	speichern := func(t *testing.T, extra url.Values) {
		t.Helper()
		aktuell, err := pages.GetPage(ctx, p.ID)
		if err != nil || aktuell == nil {
			t.Fatalf("Seite lesen: %v", err)
		}
		values := url.Values{
			"title":            {"Bretter"},
			"slug":             {"bretter"},
			"content_markdown": {"text"},
			"status":           {"draft"},
			"version":          {strconv.FormatInt(aktuell.Version, 10)},
		}
		for k, v := range extra {
			values[k] = v
		}
		req := postForm("/admin/websites/1/pages/1/edit", values, map[string]string{
			"id": wsID, "pageID": strconv.FormatInt(p.ID, 10),
		})
		if rec := serve(t, h, sm, h.HandlePageEdit, req); rec.Code != http.StatusSeeOther {
			t.Fatalf("saving returned %d, wanted 303:\n%s", rec.Code, rec.Body.String())
		}
	}

	gespeichert := func(t *testing.T) string {
		t.Helper()
		stored, err := pages.GetPage(ctx, p.ID)
		if err != nil || stored == nil {
			t.Fatalf("Seite lesen: %v", err)
		}
		return field.Decode(stored.Fields).Values["sorten"]
	}

	// Three checkboxes — three values, one line per value, in the order of the
	// form.
	speichern(t, url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
		"feld_herkunft": {"Jura"},
	})
	if got, will := gespeichert(t), "Eiche\nBuche\nEsche"; got != will {
		t.Fatalf("gespeichert %q, wollte %q", got, will)
	}

	// The same form twice yields the same string, character for character.
	speichern(t, url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
		"feld_herkunft": {"Jura"},
	})
	if got, will := gespeichert(t), "Eiche\nBuche\nEsche"; got != will {
		t.Errorf("after the second save %q, wanted %q", got, will)
	}

	// Redrawing: the same three boxes are ticked.
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", wsID)
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	body := rec.Body.String()

	angekreuzt := regexp.MustCompile(`<input type="checkbox" name="feld_sorten\[\]" value="[^"]*" checked>`)
	if n := len(angekreuzt.FindAllString(body, -1)); n != 3 {
		t.Errorf("%d ticked boxes in the redrawn form, wanted 3:\n%s", n, around(body, "feld_sorten"))
	}
	for _, sorte := range []string{"Eiche", "Buche", "Esche"} {
		if !strings.Contains(body, `value="`+sorte+`" checked`) {
			t.Errorf("%q is not ticked", sorte)
		}
	}
	// And the hidden guard stands before them, or a group that has been emptied
	// cannot be told apart from a form that never carried the field.
	if !strings.Contains(body, `<input type="hidden" name="feld_sorten[]" value="">`) {
		t.Errorf("the hidden sentinel is missing:\n%s", around(body, "feld_sorten"))
	}

	if strings.Contains(body, "checked checked") {
		t.Error("checked stands twice on the same box")
	}

	pruefeBeschriftung(t, body)

	// Only the guard, no checkbox: the value is emptied.
	speichern(t, url.Values{
		"feld_sorten[]": {""},
		"feld_herkunft": {"Jura"},
	})
	if got := gespeichert(t); got != "" {
		t.Errorf("after clearing, %q, wanted empty", got)
	}
}

// Emptied and not there at all are two different things, and the place where
// they differ is fieldsFromRequest: with the guard the key stands in the data
// with an empty value, without it it does not stand there at all. Without that
// difference a form that never carried the field cannot leave a value alone.
func TestMehrfachauswahlGeleertOderAbwesend(t *testing.T) {
	drei := fieldsFromRequest(anfrageMit(url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
	}))
	if got, will := drei.Values["sorten"], "Eiche\nBuche\nEsche"; got != will {
		t.Errorf("three ticks yielded %q, wanted %q", got, will)
	}

	// The guard alone: the key IS there and carries the empty value.
	geleert := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {""}}))
	val, da := geleert.Values["sorten"]
	if !da {
		t.Error("after the sentinel alone the key is missing entirely — emptied would not be distinguishable from absent")
	}
	if val != "" {
		t.Errorf("after the sentinel alone %q is there, wanted empty", val)
	}

	// Partly ticked: the guard falls away, the checkboxes stay.
	teil := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {"", "Buche"}}))
	if got, will := teil.Values["sorten"], "Buche"; got != will {
		t.Errorf("the sentinel beside a tick yielded %q, wanted %q", got, will)
	}

	// No key at all: it does not appear in the data.
	ohne := fieldsFromRequest(anfrageMit(url.Values{"feld_herkunft": {"Jura"}}))
	if _, da := ohne.Values["sorten"]; da {
		t.Error("the key is in the data although the form never carried it")
	}

	// And a single-valued field behaves unchanged.
	if got, will := ohne.Values["herkunft"], "Jura"; got != will {
		t.Errorf("the single-valued field yielded %q, wanted %q", got, will)
	}

	// A key that would be empty after the marker is cut off is passed over
	// rather than stored under the empty name.
	leer := fieldsFromRequest(anfrageMit(url.Values{"feld_[]": {"x"}}))
	if _, da := leer.Values[""]; da {
		t.Error("an empty key was stored")
	}
}

// The shared label above a field points with for= at the id of one control. A
// checkbox group has no single control it could point at — there
// aria-labelledby names the group. Both have to be right: the group must not
// point at an element that does not exist, and every other field has to keep
// its link.
func pruefeBeschriftung(t *testing.T, body string) {
	t.Helper()

	if strings.Contains(body, `for="feld_sorten[]"`) {
		t.Error(`die Beschriftung der Häkchengruppe trägt ein for=, das auf kein Element zeigt`) //nolint:german — the message quotes the German fixture it is about
	}

	labelledBy := regexp.MustCompile(`aria-labelledby="([^"]+)"`)
	treffer := labelledBy.FindAllStringSubmatch(body, -1)
	if len(treffer) == 0 {
		t.Fatalf("kein aria-labelledby im Formular:\n%s", around(body, "feld_sorten"))
	}
	var benannt bool
	for _, m := range treffer {
		if !strings.Contains(body, `id="`+m[1]+`"`) {
			t.Errorf("aria-labelledby points at %q, but no element carries that id", m[1])
			continue
		}
		if strings.Contains(m[1], "feld_sorten") {
			benannt = true
		}
	}
	if !benannt {
		t.Errorf("the checkbox group is named by no label: %v", treffer)
	}

	// The counter-check: the ordinary text field next to it has kept its for=,
	// and the id it names stands in the same form.
	if !strings.Contains(body, `for="feld_herkunft"`) {
		t.Error("the text field lost its for= — the condition switched itself off everywhere")
	}
	if !strings.Contains(body, `id="feld_herkunft"`) {
		t.Error("the text field's for= names an id that does not occur in the form")
	}
}

func anfrageMit(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		panic(err)
	}
	return req
}

// around cuts out the surroundings of a string, so that a failed assertion does
// not print the whole form.
func around(body, um string) string {
	i := strings.Index(body, um)
	if i < 0 {
		return body
	}
	von, bis := i-400, i+900
	if von < 0 {
		von = 0
	}
	if bis > len(body) {
		bis = len(body)
	}
	return body[von:bis]
}

// The same field, one level down. A group row carries names of its own, and
// without the marker in this second place exactly the first of three checkboxes
// would be left — silently, because a single value looks like one somebody set
// that way.
func TestMehrfachauswahlInEinerGruppe(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()

	fields := field.NewStore(database)
	gruppe, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, Key: "zeiten", Label: "Opening hours", Kind: field.KindGroup,
	})
	if err != nil {
		t.Fatalf("Gruppe anlegen: %v", err)
	}
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, ParentID: gruppe.ID, Key: "tage", Label: "Tage",
		Kind: field.KindMulti, Choices: []string{"Mo", "Di", "Mi"},
	}); err != nil {
		t.Fatalf("Unterfeld anlegen: %v", err)
	}
	// A single-valued subfield next to it: the counter-check that not
	// everything in the row has switched over to multi-valued.
	if _, err := fields.Create(ctx, field.Def{
		WebsiteID: ws.ID, ParentID: gruppe.ID, Key: "notiz", Label: "Notiz",
		Kind: field.KindText,
	}); err != nil {
		t.Fatalf("Unterfeld anlegen: %v", err)
	}

	p := seedPage(t, database, ws.ID, "Laden", "laden", "text", "draft")
	pages := page.NewStore(database)
	wsID := strconv.FormatInt(ws.ID, 10)

	speichern := func(t *testing.T, extra url.Values) {
		t.Helper()
		aktuell, err := pages.GetPage(ctx, p.ID)
		if err != nil || aktuell == nil {
			t.Fatalf("Seite lesen: %v", err)
		}
		values := url.Values{
			"title":            {"Laden"},
			"slug":             {"laden"},
			"content_markdown": {"text"},
			"status":           {"draft"},
			"version":          {strconv.FormatInt(aktuell.Version, 10)},
		}
		for k, v := range extra {
			values[k] = v
		}
		req := postForm("/admin/websites/1/pages/1/edit", values, map[string]string{
			"id": wsID, "pageID": strconv.FormatInt(p.ID, 10),
		})
		if rec := serve(t, h, sm, h.HandlePageEdit, req); rec.Code != http.StatusSeeOther {
			t.Fatalf("saving returned %d, wanted 303:\n%s", rec.Code, rec.Body.String())
		}
	}

	zeilen := func(t *testing.T) []field.Values {
		t.Helper()
		stored, err := pages.GetPage(ctx, p.ID)
		if err != nil || stored == nil {
			t.Fatalf("Seite lesen: %v", err)
		}
		return field.Decode(stored.Fields).Rows["zeiten"]
	}

	// Two rows with different checkboxes. The guard stands in every row before
	// the boxes, exactly as above on the page.
	speichern(t, url.Values{
		"gruppe.zeiten.0.tage[]": {"", "Mo", "Di", "Mi"},
		"gruppe.zeiten.0.notiz":  {"Vormittag"},
		"gruppe.zeiten.1.tage[]": {"", "Di"},
		"gruppe.zeiten.1.notiz":  {"Nachmittag"},
	})

	got := zeilen(t)
	if len(got) != 2 {
		t.Fatalf("%d Zeilen gespeichert, wollte 2: %+v", len(got), got)
	}
	if will := "Mo\nDi\nMi"; got[0]["tage"] != will {
		t.Errorf("Zeile 1 speicherte %q, wollte %q", got[0]["tage"], will)
	}
	if got[1]["tage"] != "Di" {
		t.Errorf("Zeile 2 speicherte %q, wollte %q", got[1]["tage"], "Di")
	}
	// The single-valued subfield is untouched.
	if got[0]["notiz"] != "Vormittag" || got[1]["notiz"] != "Nachmittag" {
		t.Errorf("die einwertigen Unterfelder stimmen nicht: %q / %q", got[0]["notiz"], got[1]["notiz"])
	}

	// Redrawing: the same checkboxes stand there again, per row under the name
	// of that row.
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", wsID)
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the form returned %d", rec.Code)
	}
	body := rec.Body.String()

	for zeile, will := range map[string]int{"0": 3, "1": 1} {
		muster := regexp.MustCompile(`<input type="checkbox" name="gruppe\.zeiten\.` + zeile +
			`\.tage\[\]" value="[^"]*" checked>`)
		if n := len(muster.FindAllString(body, -1)); n != will {
			t.Errorf("row %s shows %d ticked boxes, wanted %d:\n%s",
				zeile, n, will, around(body, "gruppe.zeiten."+zeile+".tage"))
		}
		if !strings.Contains(body, `<input type="hidden" name="gruppe.zeiten.`+zeile+`.tage[]" value="">`) {
			t.Errorf("row %s is missing the hidden sentinel", zeile)
		}
	}
	// The single-valued subfield does not carry the marker.
	if strings.Contains(body, "gruppe.zeiten.0.notiz[]") {
		t.Error("the single-valued sub-field carries the marking")
	}

	// Only the guard in the first row: its selection is emptied, the second
	// stays as it was.
	speichern(t, url.Values{
		"gruppe.zeiten.0.tage[]": {""},
		"gruppe.zeiten.0.notiz":  {"Vormittag"},
		"gruppe.zeiten.1.tage[]": {"", "Di"},
		"gruppe.zeiten.1.notiz":  {"Nachmittag"},
	})
	got = zeilen(t)
	if len(got) != 2 {
		t.Fatalf("after clearing, %d rows, wanted 2: %+v", len(got), got)
	}
	if got[0]["tage"] != "" {
		t.Errorf("the emptied row still carries %q", got[0]["tage"])
	}
	if got[1]["tage"] != "Di" {
		t.Errorf("the other row was emptied along with it: %q", got[1]["tage"])
	}
}

// The name of a group row is read in one place, and that place has to recognise
// the marker without giving up any of its guards: the prefix, the three parts
// and above all the bound on the row number. A name built by hand must reach
// neither a row outside the bound nor a fourth namespace.
func TestZeilennameMitMarkierung(t *testing.T) {
	group, index, sub, multi, ok := parseRowName("gruppe.zeiten.0.tage[]")
	if !ok || group != "zeiten" || index != 0 || sub != "tage" || !multi {
		t.Errorf("gruppe.zeiten.0.tage[] ergab (%q, %d, %q, %v, %v)", group, index, sub, multi, ok)
	}

	group, index, sub, multi, ok = parseRowName("gruppe.zeiten.3.tage")
	if !ok || group != "zeiten" || index != 3 || sub != "tage" || multi {
		t.Errorf("gruppe.zeiten.3.tage ergab (%q, %d, %q, %v, %v)", group, index, sub, multi, ok)
	}

	// Every existing guard still stands.
	abgelehnt := []string{
		"feld_tage[]",
		"gruppe.zeiten.0",
		"gruppe.zeiten.0.tage.extra[]",
		// After the marker is cut off no subfield name would be left — the same
		// guard the field has on the page itself.
		"gruppe.zeiten.0.[]",
		"gruppe.zeiten.0.",
		"gruppe.zeiten.x.tage[]",
		"gruppe.zeiten.-1.tage[]",
		"gruppe.zeiten." + strconv.Itoa(field.MaxRows) + ".tage[]",
		"gruppe.zeiten.999999.tage[]",
	}
	for _, name := range abgelehnt {
		if _, _, _, _, ok := parseRowName(name); ok {
			t.Errorf("%q wurde angenommen", name)
		}
	}
}

// And inside the row the same difference holds as above on the page: with the
// guard the key is there and empty, without it it is not there at all.
func TestGruppenzeileGeleertOderAbwesend(t *testing.T) {
	drei := fieldsFromRequest(anfrageMit(url.Values{
		"gruppe.zeiten.0.tage[]": {"", "Mo", "Di", "Mi"},
	}))
	if got, will := drei.Rows["zeiten"][0]["tage"], "Mo\nDi\nMi"; got != will {
		t.Errorf("three ticks in the row yielded %q, wanted %q", got, will)
	}

	geleert := fieldsFromRequest(anfrageMit(url.Values{"gruppe.zeiten.0.tage[]": {""}}))
	val, da := geleert.Rows["zeiten"][0]["tage"]
	if !da {
		t.Error("after the sentinel alone the row's key is missing entirely")
	}
	if val != "" {
		t.Errorf("after the sentinel alone %q is there, wanted empty", val)
	}

	ohne := fieldsFromRequest(anfrageMit(url.Values{"gruppe.zeiten.0.notiz": {"Vormittag"}}))
	if _, da := ohne.Rows["zeiten"][0]["tage"]; da {
		t.Error("the key is in the row although the form never carried it")
	}
	if got := ohne.Rows["zeiten"][0]["notiz"]; got != "Vormittag" {
		t.Errorf("das einwertige Unterfeld ergab %q, wollte %q", got, "Vormittag")
	}
}
