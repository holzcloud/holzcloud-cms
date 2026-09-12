package web

import (
	"strings"
	"testing"
)

// A plugin must get no script into the admin: it would run in the origin that
// holds the session cookie.
func TestPluginBildschirmOhneSkript(t *testing.T) {
	roh := `<p>Hallo</p><script>alert(1)</script><img src="x" onerror="alert(2)">` +
		`<a href="javascript:alert(3)">klick</a><iframe src="https://example.com"></iframe>`
	out := string(SanitizeAdminHTML(roh))

	for _, verboten := range []string{"<script", "onerror", "javascript:", "<iframe"} {
		if strings.Contains(out, verboten) {
			t.Errorf("%q survived the cleaning: %s", verboten, out)
		}
	}
	if !strings.Contains(out, "<p>Hallo</p>") {
		t.Errorf("the harmless part is gone: %s", out)
	}
}

// A settings screen consists of forms — those have to survive, or a plugin can
// offer nothing that can be operated.
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
			t.Errorf("%q is missing after the cleaning: %s", nötig, out)
		}
	}
}

// The session key is put in by the host, into every form.
//
// Without it every button on every plugin screen answers 403: a plugin's form
// is an ordinary submit without a header.
func TestSchluesselKommtInJedesFormular(t *testing.T) {
	screen := SanitizeAdminHTML(
		`<form method="POST"><button type="submit">Eins</button></form>` +
			`<p>dazwischen</p>` +
			`<form method="POST" class="zwei"><button type="submit">Zwei</button></form>`)
	out := string(WithCSRFToken(screen, "geheim123"))

	if n := strings.Count(out, `name="gorilla.csrf.Token"`); n != 2 {
		t.Errorf("%d key fields, want 2: %s", n, out)
	}
	if !strings.Contains(out, `value="geheim123"`) {
		t.Errorf("the value is missing: %s", out)
	}
	// Directly behind the opening tag, or it stands outside the form and is not
	// sent along.
	if !strings.Contains(out, `<form method="POST"><input type="hidden" name="gorilla.csrf.Token"`) {
		t.Errorf("the field is not in the form: %s", out)
	}
}

// Without a form nothing changes, and without a key nothing either.
func TestSchluesselNurWoEinFormularIst(t *testing.T) {
	screen := SafeHTML(`<p>nur Text</p>`)
	if out := string(WithCSRFToken(screen, "geheim")); out != `<p>nur Text</p>` {
		t.Errorf("out = %q", out)
	}
	mitForm := SafeHTML(`<form method="POST"></form>`)
	if out := string(WithCSRFToken(mitForm, "")); out != `<form method="POST"></form>` {
		t.Errorf("something was substituted without a key: %q", out)
	}
}

// A plugin must not write itself a key — what it sends goes through the
// sanitiser, and the real one is added only afterwards.
func TestPluginKannKeinenSchluesselErfinden(t *testing.T) {
	roh := `<form method="POST"><input type="hidden" name="gorilla.csrf.Token" value="erfunden"></form>`
	out := string(WithCSRFToken(SanitizeAdminHTML(roh), "echt"))

	if strings.Contains(out, "erfunden") && !strings.Contains(out, "echt") {
		t.Errorf("only the invented key is there: %s", out)
	}
	if !strings.Contains(out, `value="echt"`) {
		t.Errorf("the real key is missing: %s", out)
	}
	// The real one stands in front: the first field of the same name wins on reading.
	echt := strings.Index(out, `value="echt"`)
	erfunden := strings.Index(out, `value="erfunden"`)
	if erfunden >= 0 && erfunden < echt {
		t.Errorf("the invented key stands before the real one: %s", out)
	}
}
