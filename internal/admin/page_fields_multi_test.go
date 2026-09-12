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

// Der ganze Weg eines mehrwertigen Feldes: Name im Formular, Lesen der
// Anfrage, Speichern, Auflösen, Neuzeichnen. Ein Häkchenfeld ist der erste
// Feldwert dieses Programms, der kein einzelner String ist — was hier grün
// ist, erbt Phase 9 unverändert.
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
	// Ein gewöhnliches Textfeld daneben. Es ist die Gegenprobe dafür, dass die
	// bedingte Beschriftungsverknüpfung sich nicht überall abgeschaltet hat.
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

	// Drei Häkchen — drei Werte, eine Zeile je Wert, in der Reihenfolge des
	// Formulars.
	speichern(t, url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
		"feld_herkunft": {"Jura"},
	})
	if got, will := gespeichert(t), "Eiche\nBuche\nEsche"; got != will {
		t.Fatalf("gespeichert %q, wollte %q", got, will)
	}

	// Zweimal dasselbe Formular ergibt dieselbe Zeichenkette, Zeichen für
	// Zeichen.
	speichern(t, url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
		"feld_herkunft": {"Jura"},
	})
	if got, will := gespeichert(t), "Eiche\nBuche\nEsche"; got != will {
		t.Errorf("after the second save %q, wanted %q", got, will)
	}

	// Neuzeichnen: dieselben drei Kästchen sind angekreuzt.
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
		t.Errorf("%d ticked boxes in the redrawn form, wanted 3:\n%s", n, ausschnitt(body, "feld_sorten"))
	}
	for _, sorte := range []string{"Eiche", "Buche", "Esche"} {
		if !strings.Contains(body, `value="`+sorte+`" checked`) {
			t.Errorf("%q is not ticked", sorte)
		}
	}
	// Und der versteckte Wächter steht davor, sonst kann eine leergeräumte
	// Gruppe nicht von einem Formular unterschieden werden, das das Feld nie
	// getragen hat.
	if !strings.Contains(body, `<input type="hidden" name="feld_sorten[]" value="">`) {
		t.Errorf("the hidden sentinel is missing:\n%s", ausschnitt(body, "feld_sorten"))
	}

	if strings.Contains(body, "checked checked") {
		t.Error("checked stands twice on the same box")
	}

	pruefeBeschriftung(t, body)

	// Nur der Wächter, kein Häkchen: der Wert ist geleert.
	speichern(t, url.Values{
		"feld_sorten[]": {""},
		"feld_herkunft": {"Jura"},
	})
	if got := gespeichert(t); got != "" {
		t.Errorf("after clearing, %q, wanted empty", got)
	}
}

// Geleert und gar nicht da sind zwei verschiedene Dinge, und die Stelle, an
// der sie sich unterscheiden, ist fieldsFromRequest: mit dem Wächter steht die
// Kennung mit leerem Wert in den Daten, ohne ihn steht sie gar nicht darin.
// Ohne diesen Unterschied kann ein Formular, das das Feld nie trug, einen Wert
// nicht in Ruhe lassen.
func TestMehrfachauswahlGeleertOderAbwesend(t *testing.T) {
	drei := fieldsFromRequest(anfrageMit(url.Values{
		"feld_sorten[]": {"Eiche", "Buche", "Esche"},
	}))
	if got, will := drei.Values["sorten"], "Eiche\nBuche\nEsche"; got != will {
		t.Errorf("three ticks yielded %q, wanted %q", got, will)
	}

	// Der Wächter allein: die Kennung IST da und trägt den leeren Wert.
	geleert := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {""}}))
	val, da := geleert.Values["sorten"]
	if !da {
		t.Error("after the sentinel alone the key is missing entirely — emptied would not be distinguishable from absent")
	}
	if val != "" {
		t.Errorf("after the sentinel alone %q is there, wanted empty", val)
	}

	// Teilweise angekreuzt: der Wächter fällt weg, die Häkchen bleiben.
	teil := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {"", "Buche"}}))
	if got, will := teil.Values["sorten"], "Buche"; got != will {
		t.Errorf("the sentinel beside a tick yielded %q, wanted %q", got, will)
	}

	// Gar keine Kennung: sie kommt in den Daten nicht vor.
	ohne := fieldsFromRequest(anfrageMit(url.Values{"feld_herkunft": {"Jura"}}))
	if _, da := ohne.Values["sorten"]; da {
		t.Error("the key is in the data although the form never carried it")
	}

	// Und ein einwertiges Feld verhält sich unverändert.
	if got, will := ohne.Values["herkunft"], "Jura"; got != will {
		t.Errorf("the single-valued field yielded %q, wanted %q", got, will)
	}

	// Eine Kennung, die nach dem Abschneiden der Markierung leer wäre, wird
	// übergangen statt unter dem leeren Namen abgelegt.
	leer := fieldsFromRequest(anfrageMit(url.Values{"feld_[]": {"x"}}))
	if _, da := leer.Values[""]; da {
		t.Error("an empty key was stored")
	}
}

// Die gemeinsame Beschriftung über einem Feld zeigt mit for= auf die Kennung
// eines Bedienelements. Eine Häkchengruppe hat kein einzelnes Bedienelement,
// auf das sie zeigen könnte — dort benennt aria-labelledby die Gruppe. Beides
// muss stimmen: die Gruppe darf nicht auf ein Element zeigen, das es nicht
// gibt, und jedes andere Feld muss seine Verknüpfung behalten.
func pruefeBeschriftung(t *testing.T, body string) {
	t.Helper()

	if strings.Contains(body, `for="feld_sorten[]"`) {
		t.Error(`die Beschriftung der Häkchengruppe trägt ein for=, das auf kein Element zeigt`) //nolint:german — the message quotes the German fixture it is about
	}

	labelledBy := regexp.MustCompile(`aria-labelledby="([^"]+)"`)
	treffer := labelledBy.FindAllStringSubmatch(body, -1)
	if len(treffer) == 0 {
		t.Fatalf("kein aria-labelledby im Formular:\n%s", ausschnitt(body, "feld_sorten"))
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

	// Die Gegenprobe: das gewöhnliche Textfeld daneben hat sein for= behalten,
	// und die Kennung, die es nennt, steht im selben Formular.
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

// ausschnitt schneidet die Umgebung einer Zeichenkette heraus, damit eine
// fehlgeschlagene Zusicherung nicht das ganze Formular ausgibt.
func ausschnitt(body, um string) string {
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

// Dasselbe Feld, eine Ebene tiefer. Eine Gruppenzeile trägt ihre eigenen
// Namen, und ohne die Markierung an dieser zweiten Stelle bliebe von drei
// Häkchen genau das erste übrig — still, denn ein einzelner Wert sieht aus
// wie einer, den jemand so gesetzt hat.
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
	// Ein einwertiges Unterfeld daneben: die Gegenprobe, dass sich in der
	// Zeile nicht alles auf mehrwertig umgestellt hat.
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

	// Zwei Zeilen mit verschiedenen Häkchen. Der Wächter steht in jeder Zeile
	// vor den Kästchen, genau wie oben auf der Seite.
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
	// Das einwertige Unterfeld ist unberührt.
	if got[0]["notiz"] != "Vormittag" || got[1]["notiz"] != "Nachmittag" {
		t.Errorf("die einwertigen Unterfelder stimmen nicht: %q / %q", got[0]["notiz"], got[1]["notiz"])
	}

	// Neuzeichnen: dieselben Häkchen stehen wieder da, je Zeile unter dem
	// Namen dieser Zeile.
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
				zeile, n, will, ausschnitt(body, "gruppe.zeiten."+zeile+".tage"))
		}
		if !strings.Contains(body, `<input type="hidden" name="gruppe.zeiten.`+zeile+`.tage[]" value="">`) {
			t.Errorf("row %s is missing the hidden sentinel", zeile)
		}
	}
	// Das einwertige Unterfeld trägt die Markierung nicht.
	if strings.Contains(body, "gruppe.zeiten.0.notiz[]") {
		t.Error("the single-valued sub-field carries the marking")
	}

	// Nur der Wächter in der ersten Zeile: deren Auswahl ist geleert, die
	// zweite bleibt, wie sie war.
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

// Der Name einer Gruppenzeile wird an einer Stelle gelesen, und diese Stelle
// muss die Markierung erkennen, ohne eine ihrer Wachen aufzugeben: der
// Vorsatz, die drei Teile und vor allem die Schranke auf die Zeilennummer.
// Ein von Hand gebauter Name darf keine Zeile ausserhalb der Schranke und
// keinen vierten Namensraum erreichen.
func TestZeilennameMitMarkierung(t *testing.T) {
	group, index, sub, multi, ok := parseRowName("gruppe.zeiten.0.tage[]")
	if !ok || group != "zeiten" || index != 0 || sub != "tage" || !multi {
		t.Errorf("gruppe.zeiten.0.tage[] ergab (%q, %d, %q, %v, %v)", group, index, sub, multi, ok)
	}

	group, index, sub, multi, ok = parseRowName("gruppe.zeiten.3.tage")
	if !ok || group != "zeiten" || index != 3 || sub != "tage" || multi {
		t.Errorf("gruppe.zeiten.3.tage ergab (%q, %d, %q, %v, %v)", group, index, sub, multi, ok)
	}

	// Jede bestehende Wache steht noch.
	abgelehnt := []string{
		"feld_tage[]",
		"gruppe.zeiten.0",
		"gruppe.zeiten.0.tage.extra[]",
		// Nach dem Abschneiden der Markierung bliebe kein Unterfeldname
		// übrig — dieselbe Wache, die das Feld auf der Seite selbst hat.
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

// Und in der Zeile gilt derselbe Unterschied wie oben auf der Seite: mit dem
// Wächter ist die Kennung da und leer, ohne ihn ist sie gar nicht da.
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
