package web

import (
	"strings"
	"testing"
)

// Ein Plugin darf kein Skript in die Verwaltung bekommen: es liefe in der
// Herkunft, die das Sitzungsplätzchen hält.
func TestPluginBildschirmOhneSkript(t *testing.T) {
	roh := `<p>Hallo</p><script>alert(1)</script><img src="x" onerror="alert(2)">` +
		`<a href="javascript:alert(3)">klick</a><iframe src="https://example.com"></iframe>`
	out := string(SanitizeAdminHTML(roh))

	for _, verboten := range []string{"<script", "onerror", "javascript:", "<iframe"} {
		if strings.Contains(out, verboten) {
			t.Errorf("%q hat die Bereinigung überlebt: %s", verboten, out)
		}
	}
	if !strings.Contains(out, "<p>Hallo</p>") {
		t.Errorf("der harmlose Teil ist weg: %s", out)
	}
}

// Ein Einstellungsbildschirm besteht aus Formularen — die müssen überleben,
// sonst kann ein Plugin nichts anbieten, was man bedienen kann.
func TestPluginBildschirmBehaeltFormulare(t *testing.T) {
	roh := `<form method="POST"><input type="hidden" name="aktion" value="speichern">` +
		`<label for="a">A</label><input type="text" id="a" name="a" value="1">` +
		`<select name="b"><option value="x" selected>X</option></select>` +
		`<textarea name="c" rows="3">Text</textarea>` +
		`<button type="submit">Speichern</button></form>`
	out := string(SanitizeAdminHTML(roh))

	for _, nötig := range []string{
		`<form`, `name="aktion"`, `value="speichern"`, `<label`, `<select`,
		`<option`, `selected`, `<textarea`, `<button`,
	} {
		if !strings.Contains(out, nötig) {
			t.Errorf("%q fehlt nach der Bereinigung: %s", nötig, out)
		}
	}
}

// Der Sitzungsschlüssel wird vom Host eingesetzt, in jedes Formular.
//
// Ohne ihn antwortet jeder Knopf auf jedem Plugin-Bildschirm mit 403: das
// Formular eines Plugins ist ein gewöhnliches Absenden ohne Kopfzeile.
func TestSchluesselKommtInJedesFormular(t *testing.T) {
	screen := SanitizeAdminHTML(
		`<form method="POST"><button type="submit">Eins</button></form>` +
			`<p>dazwischen</p>` +
			`<form method="POST" class="zwei"><button type="submit">Zwei</button></form>`)
	out := string(WithCSRFToken(screen, "geheim123"))

	if n := strings.Count(out, `name="gorilla.csrf.Token"`); n != 2 {
		t.Errorf("%d Schlüsselfelder, want 2: %s", n, out)
	}
	if !strings.Contains(out, `value="geheim123"`) {
		t.Errorf("der Wert fehlt: %s", out)
	}
	// Direkt hinter dem öffnenden Tag, sonst steht er ausserhalb des Formulars
	// und wird nicht mitgesendet.
	if !strings.Contains(out, `<form method="POST"><input type="hidden" name="gorilla.csrf.Token"`) {
		t.Errorf("das Feld steht nicht im Formular: %s", out)
	}
}

// Ohne Formular ändert sich nichts, und ohne Schlüssel auch nicht.
func TestSchluesselNurWoEinFormularIst(t *testing.T) {
	screen := SafeHTML(`<p>nur Text</p>`)
	if out := string(WithCSRFToken(screen, "geheim")); out != `<p>nur Text</p>` {
		t.Errorf("out = %q", out)
	}
	mitForm := SafeHTML(`<form method="POST"></form>`)
	if out := string(WithCSRFToken(mitForm, "")); out != `<form method="POST"></form>` {
		t.Errorf("ohne Schlüssel wurde etwas eingesetzt: %q", out)
	}
}

// Ein Plugin darf sich keinen Schlüssel selbst schreiben — was es sendet, geht
// durch die Bereinigung, und der echte kommt erst danach dazu.
func TestPluginKannKeinenSchluesselErfinden(t *testing.T) {
	roh := `<form method="POST"><input type="hidden" name="gorilla.csrf.Token" value="erfunden"></form>`
	out := string(WithCSRFToken(SanitizeAdminHTML(roh), "echt"))

	if strings.Contains(out, "erfunden") && !strings.Contains(out, "echt") {
		t.Errorf("nur der erfundene Schlüssel steht da: %s", out)
	}
	if !strings.Contains(out, `value="echt"`) {
		t.Errorf("der echte Schlüssel fehlt: %s", out)
	}
	// Der echte steht vorn: das erste Feld gleichen Namens gewinnt beim Lesen.
	echt := strings.Index(out, `value="echt"`)
	erfunden := strings.Index(out, `value="erfunden"`)
	if erfunden >= 0 && erfunden < echt {
		t.Errorf("der erfundene Schlüssel steht vor dem echten: %s", out)
	}
}
