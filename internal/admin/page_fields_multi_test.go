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
			t.Fatalf("Speichern gab %d zurück, wollte 303:\n%s", rec.Code, rec.Body.String())
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
		t.Errorf("nach dem zweiten Speichern %q, wollte %q", got, will)
	}

	// Neuzeichnen: dieselben drei Kästchen sind angekreuzt.
	req := httptest.NewRequest(http.MethodGet, "/admin/websites/1/pages/1/edit", nil)
	req.SetPathValue("id", wsID)
	req.SetPathValue("pageID", strconv.FormatInt(p.ID, 10))
	rec := serve(t, h, sm, h.HandlePageEdit, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("das Formular gab %d zurück", rec.Code)
	}
	body := rec.Body.String()

	angekreuzt := regexp.MustCompile(`<input type="checkbox" name="feld_sorten\[\]" value="[^"]*" checked>`)
	if n := len(angekreuzt.FindAllString(body, -1)); n != 3 {
		t.Errorf("%d angekreuzte Kästchen im neu gezeichneten Formular, wollte 3:\n%s", n, ausschnitt(body, "feld_sorten"))
	}
	for _, sorte := range []string{"Eiche", "Buche", "Esche"} {
		if !strings.Contains(body, `value="`+sorte+`" checked`) {
			t.Errorf("%q ist nicht angekreuzt", sorte)
		}
	}
	// Und der versteckte Wächter steht davor, sonst kann eine leergeräumte
	// Gruppe nicht von einem Formular unterschieden werden, das das Feld nie
	// getragen hat.
	if !strings.Contains(body, `<input type="hidden" name="feld_sorten[]" value="">`) {
		t.Errorf("der versteckte Wächter fehlt:\n%s", ausschnitt(body, "feld_sorten"))
	}

	if strings.Contains(body, "checked checked") {
		t.Error("checked steht zweimal am selben Kästchen")
	}

	pruefeBeschriftung(t, body)

	// Nur der Wächter, kein Häkchen: der Wert ist geleert.
	speichern(t, url.Values{
		"feld_sorten[]": {""},
		"feld_herkunft": {"Jura"},
	})
	if got := gespeichert(t); got != "" {
		t.Errorf("nach dem Leerräumen %q, wollte leer", got)
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
		t.Errorf("drei Häkchen ergaben %q, wollte %q", got, will)
	}

	// Der Wächter allein: die Kennung IST da und trägt den leeren Wert.
	geleert := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {""}}))
	val, da := geleert.Values["sorten"]
	if !da {
		t.Error("nach dem Wächter allein fehlt die Kennung ganz — geleert wäre nicht von abwesend zu unterscheiden")
	}
	if val != "" {
		t.Errorf("nach dem Wächter allein steht %q da, wollte leer", val)
	}

	// Teilweise angekreuzt: der Wächter fällt weg, die Häkchen bleiben.
	teil := fieldsFromRequest(anfrageMit(url.Values{"feld_sorten[]": {"", "Buche"}}))
	if got, will := teil.Values["sorten"], "Buche"; got != will {
		t.Errorf("der Wächter neben einem Häkchen ergab %q, wollte %q", got, will)
	}

	// Gar keine Kennung: sie kommt in den Daten nicht vor.
	ohne := fieldsFromRequest(anfrageMit(url.Values{"feld_herkunft": {"Jura"}}))
	if _, da := ohne.Values["sorten"]; da {
		t.Error("die Kennung steht in den Daten, obwohl das Formular sie nie trug")
	}

	// Und ein einwertiges Feld verhält sich unverändert.
	if got, will := ohne.Values["herkunft"], "Jura"; got != will {
		t.Errorf("das einwertige Feld ergab %q, wollte %q", got, will)
	}

	// Eine Kennung, die nach dem Abschneiden der Markierung leer wäre, wird
	// übergangen statt unter dem leeren Namen abgelegt.
	leer := fieldsFromRequest(anfrageMit(url.Values{"feld_[]": {"x"}}))
	if _, da := leer.Values[""]; da {
		t.Error("eine leere Kennung wurde abgelegt")
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
		t.Error(`die Beschriftung der Häkchengruppe trägt ein for=, das auf kein Element zeigt`)
	}

	labelledBy := regexp.MustCompile(`aria-labelledby="([^"]+)"`)
	treffer := labelledBy.FindAllStringSubmatch(body, -1)
	if len(treffer) == 0 {
		t.Fatalf("kein aria-labelledby im Formular:\n%s", ausschnitt(body, "feld_sorten"))
	}
	var benannt bool
	for _, m := range treffer {
		if !strings.Contains(body, `id="`+m[1]+`"`) {
			t.Errorf("aria-labelledby zeigt auf %q, aber kein Element trägt diese Kennung", m[1])
			continue
		}
		if strings.Contains(m[1], "feld_sorten") {
			benannt = true
		}
	}
	if !benannt {
		t.Errorf("die Häkchengruppe wird von keiner Beschriftung benannt: %v", treffer)
	}

	// Die Gegenprobe: das gewöhnliche Textfeld daneben hat sein for= behalten,
	// und die Kennung, die es nennt, steht im selben Formular.
	if !strings.Contains(body, `for="feld_herkunft"`) {
		t.Error("das Textfeld hat sein for= verloren — die Bedingung hat sich überall abgeschaltet")
	}
	if !strings.Contains(body, `id="feld_herkunft"`) {
		t.Error("das for= des Textfeldes nennt eine Kennung, die im Formular nicht vorkommt")
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
