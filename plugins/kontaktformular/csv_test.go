package main

import (
	"os"
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
		"gone", "too-many", "form-incomplete", "consent-missing", "attach-refused",
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

// An answer is written back over the message it answers.
//
// speichern mints a fresh key and a fresh timestamp, because it is for a
// message that has just come in. Using it to store an answer would leave the
// original standing and put a second copy of the enquiry beside it, dated
// today — two enquiries where there was one, and the operator answering the
// same person twice.
func TestAnAnswerDoesNotDuplicateTheMessage(t *testing.T) {
	// The check is on the shape of the code rather than on a running store:
	// this plugin's storage is the host's, and a unit test has no host. What
	// can be asserted here is that reply writes to prefixMessage+key and never
	// calls speichern.
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func reply(key, text string)")
	if start < 0 {
		t.Fatal("reply is gone; this test no longer knows what it is guarding")
	}
	end := strings.Index(body[start:], "\n}\n")
	if end < 0 {
		t.Fatal("cannot find the end of reply")
	}
	fn := body[start : start+end]

	if strings.Contains(fn, "speichern(") {
		t.Error("reply calls speichern, which mints a new key and duplicates the enquiry")
	}
	if !strings.Contains(fn, "plugin.Set(prefixMessage+key") {
		t.Error("reply does not write back under the message's own key")
	}
}

// sweep never removes a message nobody has read.
//
// The plugin's own migration 0001 says why: "an enquiry somebody made that
// nobody reads is a lost enquiry". An unread message is exactly the one that
// has not been dealt with.
func TestSweepKeepsWhatNobodyHasRead(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func sweep()")
	end := strings.Index(body[start:], "\n}\n")
	fn := body[start : start+end]

	if !strings.Contains(fn, "!n.Read") {
		t.Error("sweep does not look at whether a message was read")
	}
	if !strings.Contains(fn, "kept") {
		t.Error("sweep does not report what it kept, so an operator cannot see why the store is full")
	}
}

// A conditional field is asked only when its condition is met.
func TestAConditionalFieldIsAskedOnlyWhenItsConditionIsMet(t *testing.T) {
	firma := field{Key: "firma", Label: "Firma?", Art: KindChoice, Choices: []string{"ja", "nein"}}
	uid := field{Key: "uid", Label: "UID", Art: ArtText, ShowIf: "firma", ShowIfValue: "ja"}

	for _, c := range []struct {
		name   string
		answer string
		want   bool
	}{
		{"answered the way the condition wants", "ja", true},
		{"answered otherwise", "nein", false},
		{"not answered at all", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := uid.asked(map[string]string{"firma": c.answer})
			if got != c.want {
				t.Errorf("asked = %v, want %v", got, c.want)
			}
		})
	}
	if !firma.asked(map[string]string{}) {
		t.Error("a field with no condition must always be asked")
	}
	if firma.Conditional() || !uid.Conditional() {
		t.Error("Conditional does not tell the two apart")
	}
}

// A condition that cannot work is dropped when the form is saved.
//
// Dropping it leaves the field always asked, which the operator can see. The
// alternative is a question that draws nothing for ever with no way to find out
// why — the invisible failure rather than the visible one.
func TestAnImpossibleConditionIsDroppedOnSave(t *testing.T) {
	for _, c := range []struct {
		name   string
		fields []field
		// wantCond is the condition the SECOND field keeps.
		wantCond string
	}{
		{
			name: "a condition on an earlier field stands",
			fields: []field{
				{Label: "Firma?", Art: KindChoice, Choices: []string{"ja"}},
				{Label: "UID", Art: ArtText, ShowIf: "firma", ShowIfValue: "ja"},
			},
			wantCond: "firma",
		},
		{
			name: "a condition on a LATER field is dropped",
			fields: []field{
				{Label: "Firma?", Art: KindChoice, Choices: []string{"ja"}},
				{Label: "UID", Art: ArtText, ShowIf: "spaeter", ShowIfValue: "ja"},
				{Label: "Spaeter", Art: ArtText},
			},
			wantCond: "",
		},
		{
			name: "a condition on itself is dropped",
			fields: []field{
				{Label: "Firma?", Art: KindChoice, Choices: []string{"ja"}},
				{Label: "UID", Art: ArtText, ShowIf: "uid", ShowIfValue: "ja"},
			},
			wantCond: "",
		},
		{
			name: "a condition with no value is dropped",
			fields: []field{
				{Label: "Firma?", Art: KindChoice, Choices: []string{"ja"}},
				{Label: "UID", Art: ArtText, ShowIf: "firma"},
			},
			wantCond: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := formular{Name: "Test", Fields: c.fields}.clean()
			if len(out.Fields) < 2 {
				t.Fatalf("%d fields survived", len(out.Fields))
			}
			if got := out.Fields[1].ShowIf; got != c.wantCond {
				t.Errorf("condition = %q, want %q", got, c.wantCond)
			}
		})
	}
}

// A form splits at its first conditional field, and only there.
func TestAFormSplitsAtItsFirstConditionalField(t *testing.T) {
	plain := formular{Fields: []field{{Key: "a"}, {Key: "b"}}}
	if plain.HasSteps() {
		t.Error("a form with no condition has no second step")
	}
	first, second := plain.steps()
	if len(first) != 2 || second != nil {
		t.Errorf("split = %d/%d, want 2/0", len(first), len(second))
	}

	split := formular{Fields: []field{
		{Key: "a"}, {Key: "b"},
		{Key: "c", ShowIf: "a", ShowIfValue: "ja"},
		{Key: "d", ShowIf: "b", ShowIfValue: "ja"},
	}}
	if !split.HasSteps() {
		t.Error("a form with a condition has a second step")
	}
	first, second = split.steps()
	if len(first) != 2 || len(second) != 2 {
		t.Errorf("split = %d/%d, want 2/2", len(first), len(second))
	}
}
