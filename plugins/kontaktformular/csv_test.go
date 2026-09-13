package main

import (
	"strings"
	"testing"
)

// A cell beginning with = + - @ or a control character is executed as a formula
// by Excel and LibreOffice as soon as somebody opens the file. The content comes
// from a visitor to the website — so this is the path by which a stranger runs
// something on the recipient's machine.
func TestCellsThatWouldBeReadAsAFormulaGetAnApostrophe(t *testing.T) {
	gefaehrlich := []string{
		`=1+1`,
		`+49 123`,
		`-5`,
		`@SUM(A1:A9)`,
		"\tTabulator",
		"\rWagenruecklauf",
	}
	for _, value := range gefaehrlich {
		got := entschaerfen([]string{value})[0]
		if !strings.HasPrefix(got, "'") {
			t.Errorf("%q stays undefused: %q", value, got)
		}
		if got != "'"+value {
			t.Errorf("%q was changed beyond the apostrophe: %q", value, got)
		}
	}
}

func TestHarmlessCellsStayUnchanged(t *testing.T) {
	for _, value := range []string{"", "Anna", "anna@example.ch", "1+1", "Preis: 5"} {
		if got := entschaerfen([]string{value})[0]; got != value {
			t.Errorf("%q wurde zu %q", value, got)
		}
	}
}

func TestTheTableCarriesTheFixedColumnsAndTheFormsOwn(t *testing.T) {
	liste := []message{
		{
			Key: "2026-08-30T10:00:00Z-a", Time: "2026-08-30T10:00:00Z",
			Name: "Anna", Email: "anna@example.ch", Subject: "Anfrage",
			Text: "Guten Tag", Page: "kontakt",
		},
		{
			Key: "2026-08-30T11:00:00Z-b", Time: "2026-08-30T11:00:00Z",
			Name: "Bruno", Email: "bruno@example.ch", FormName: "Anmeldung",
			Read: true,
			Fields: []answer{
				{Label: "Kurs", Value: "Drechseln"},
				{Label: "Personen", Value: "2"},
			},
		},
	}

	raw, err := asCSV(liste)
	if err != nil {
		t.Fatalf("asCSV: %v", err)
	}
	out := string(raw)

	if !strings.HasPrefix(out, "\ufeff") {
		t.Error("without a byte-order mark Excel does not open the file as UTF-8")
	}
	// The fixed columns come out in the source language here: there is no host
	// in a unit test, so plugin.T hands the key straight back. "Kurs" and
	// "Personen" are the operator's own field labels and are never translated
	// in any language — that is the point of them standing in this same list.
	for _, column := range []string{"Time", "Form", "Name", "E-mail", "Kurs", "Personen", columnMessage} {
		if !strings.Contains(out, column) {
			t.Errorf("column %q missing", column)
		}
	}
	if !strings.Contains(out, "Drechseln") || !strings.Contains(out, "Guten Tag") {
		t.Error("the answers are not in the table")
	}
	// Both rows and the header row.
	if n := strings.Count(strings.TrimSpace(out), "\n"); n != 2 {
		t.Errorf("%d line breaks, expected 2 — header row plus two messages", n)
	}
}

// A message without a field another one has leaves the cell empty and does not
// shift the columns.
func TestMissingFieldsDoNotShiftTheColumns(t *testing.T) {
	liste := []message{
		{Key: "a", Fields: []answer{{Label: "Kurs", Value: "Drechseln"}}},
		{Key: "b", Fields: []answer{{Label: "Ort", Value: "Bern"}}},
	}
	raw, err := asCSV(liste)
	if err != nil {
		t.Fatalf("asCSV: %v", err)
	}
	rows := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff")), "\n")
	if len(rows) != 3 {
		t.Fatalf("%d Zeilen, 3 erwartet", len(rows))
	}
	fields := strings.Count(rows[0], ",")
	for i, z := range rows[1:] {
		if strings.Count(z, ",") != fields {
			t.Errorf("Zeile %d hat %d Trennzeichen, die Kopfzeile %d", i+1, strings.Count(z, ","), fields)
		}
	}
}

// A refusal names the field it is about, and nothing else may reach the page.
//
// The address bar used to carry the finished sentence: escaped, bounded to 160
// characters, and therefore a way to put a stranger's 160 characters into the
// operator's own layout with a crafted link. It carries a code now, and a code
// this program does not know yields the ordinary sentence.
func TestARefusalCarriesACodeAndNotASentence(t *testing.T) {
	for _, c := range []struct {
		name, code, field, word string
		wantField               string
		wantGeneric             bool
	}{
		{name: "a known code", code: "email-shape", field: fieldEmail, wantField: fieldEmail},
		{name: "a code nobody wrote", code: "please-send-your-password",
			wantGeneric: true},
		{name: "no code at all", wantGeneric: true},
		{name: "a field this form cannot have", code: "email-shape", field: "../../etc",
			wantField: ""},
		{name: "an assembled form's field", code: "field-fill",
			field: fieldPrefix + "lieblingsfarbe", word: "Lieblingsfarbe",
			wantField: fieldPrefix + "lieblingsfarbe"},
	} {
		t.Run(c.name, func(t *testing.T) {
			text, at := refusalText(c.code, c.field, c.word)
			if at != c.wantField {
				t.Errorf("field = %q, want %q", at, c.wantField)
			}
			generic := text == "The message could not be sent. Please check what you entered."
			if generic != c.wantGeneric {
				t.Errorf("generic = %v, want %v (text was %q)", generic, c.wantGeneric, text)
			}
			if text == "" {
				t.Error("a refusal with no sentence at all leaves the visitor with nothing")
			}
		})
	}
}

// The operator's own label travels, and it is bounded on the way back in.
func TestALabelFromTheAddressBarIsBounded(t *testing.T) {
	long := strings.Repeat("ü", maxLabel+40)
	text, _ := refusalText("field-fill", fieldPrefix+"x", long)
	if n := len([]rune(text)); n > maxLabel+60 {
		t.Errorf("the sentence is %d runes long; the label was not cut", n)
	}
	if !strings.Contains(text, "ü") {
		t.Error("the operator's own word did not survive at all")
	}
}

// check says which field, not only what.
func TestCheckNamesTheFieldItIsAbout(t *testing.T) {
	for _, c := range []struct {
		name  string
		in    message
		field string
		code  string
	}{
		{"no name", message{Email: "a@b.test", Text: "x"}, fieldName, "name-missing"},
		{"no address", message{Name: "A", Text: "x"}, fieldEmail, "email-missing"},
		{"a bad address", message{Name: "A", Email: "nope", Text: "x"}, fieldEmail, "email-shape"},
		{"no message", message{Name: "A", Email: "a@b.test"}, fieldText, "text-missing"},
		{"nothing wrong", message{Name: "A", Email: "a@b.test", Text: "x"}, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := check(c.in)
			if got.Field != c.field || got.Code != c.code {
				t.Errorf("= {%q %q}, want {%q %q}", got.Field, got.Code, c.field, c.code)
			}
			if got.ok() != (c.code == "") {
				t.Errorf("ok() = %v for code %q", got.ok(), got.Code)
			}
		})
	}
}

// Every code check can produce has a sentence.
//
// A code with no entry falls through to the generic sentence, which is correct
// for something from the address bar and wrong for something this program
// produced itself: the visitor would be told "please check what you entered"
// about an hour that was too busy.
func TestEveryCodeThisProgramProducesHasASentence(t *testing.T) {
	produced := []string{
		"name-missing", "name-long", "email-missing", "email-shape", "email-long",
		"subject-long", "text-missing", "text-long", "unreadable", "expired",
		"gone", "too-many", "form-incomplete",
		"field-tick", "field-fill", "field-long", "field-email", "field-digit",
		"field-date", "field-pick",
	}
	for _, code := range produced {
		if _, ok := reasons[code]; !ok {
			t.Errorf("code %q is produced and has no sentence", code)
		}
	}
	if len(reasons) != len(produced) {
		t.Errorf("%d sentences for %d codes — one of the two lists has moved",
			len(reasons), len(produced))
	}
}
