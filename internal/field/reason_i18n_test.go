package field

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// The property the whole Reason type exists for: a refusal reaches the reader
// in the reader's language.
//
// Before v2.0 this was false and every gate said otherwise. Check returned
// `d.Label + " muss eine Zahl sein."` — a string built at run time, which
// tools/i18n cannot see, so the report said `0 offen` while an operator with
// the admin set to English read "Preis muss eine Zahl sein." next to field
// kinds that were translated (measured 2026-09-08,
// .planning/audits/v1.6-I18N-828.md).
//
// The test asserts two halves that have to hold together. The sentence must
// change with the language — otherwise nothing was translated — and the
// operator's own label must NOT change, because it is their word and this
// program has no business translating it.
func TestAFieldRefusalIsTranslatedAndTheLabelIsNot(t *testing.T) {
	d := Def{Key: "preis", Label: "Preis", Kind: KindNumber}

	reason := Check(d, "abc")
	if reason.Empty() {
		t.Fatalf("the probe is wrong: Check accepted a non-number")
	}

	german := reason.Text(i18n.Source)
	english := reason.Text("en")

	if german == english {
		t.Errorf("the refusal reads the same in German and English (%q) — either the "+
			"format never reached the catalogue, or it is not being looked up", german)
	}
	if !strings.Contains(english, "number") {
		t.Errorf("the English refusal is %q and does not contain \"number\"", english)
	}
	for _, got := range []string{german, english} {
		if !strings.Contains(got, "Preis") {
			t.Errorf("the refusal %q has lost the operator's own label; it names which "+
				"field is wrong and is the half that must never be translated", got)
		}
	}
}

// Every reason a kind can mint has to be a key somebody can translate.
//
// The point is not that the catalogue is full — the i18n gate already says
// that. It is that the *format* is what reaches the catalogue and never the
// finished sentence: a sentence with "Preis" baked into it would need one key
// per label any operator ever typed, and the catalogue would grow with their
// content instead of with this program's.
//
// So: drive one refusal out of every kind, and assert that the format carries
// no label in it and that the label arrives as an argument instead.
func TestEveryKindsRefusalIsAFormatPlusArgumentsAndNotASentence(t *testing.T) {
	label := "Zwetschgenernte"
	for _, tc := range []struct {
		kind  string
		def   Def
		value string
	}{
		{KindNumber, Def{Kind: KindNumber}, "abc"},
		{KindBool, Def{Kind: KindBool}, "vielleicht"},
		{KindDate, Def{Kind: KindDate}, "gestern"},
		{KindTime, Def{Kind: KindTime}, "25:99"},
		{KindRange, Def{Kind: KindRange, RangeMin: "1", RangeMax: "10"}, "99"},
		{KindChoice, Def{Kind: KindChoice, Choices: []string{"a"}}, "z"},
		{KindMulti, Def{Kind: KindMulti, Choices: []string{"a"}}, "z"},
		{KindImage, Def{Kind: KindImage}, "kein-bild"},
		{KindRef, Def{Kind: KindRef}, "keine-seite"},
		{KindTerm, Def{Kind: KindTerm}, "Kein Schlagwort"},
		{KindLink, Def{Kind: KindLink}, "javascript:alert(1)"},
		{KindText, Def{Kind: KindText, Required: true}, ""},
	} {
		d := tc.def
		d.Key = "ernte"
		d.Label = label

		reason := Check(d, tc.value)
		if reason.Empty() {
			t.Errorf("%s: the probe is wrong, Check accepted %q", tc.kind, tc.value)
			continue
		}
		if strings.Contains(reason.Format, label) {
			t.Errorf("%s: the format is %q — the operator's label is baked into the key, "+
				"so the catalogue would need one entry per label anybody types",
				tc.kind, reason.Format)
		}
		if !strings.Contains(reason.Text(i18n.Source), label) {
			t.Errorf("%s: the rendered reason %q does not name the field",
				tc.kind, reason.Text(i18n.Source))
		}
	}
}

// The group frame is the half broken window 29 was opened for: even once the
// inner reason carried a key, fmt.Sprintf("%s, Zeile %d: %s", …) around it
// never could, so one line of the report would have been half translated.
func TestTheRowFrameAroundAGroupReasonIsTranslatedToo(t *testing.T) {
	group := Def{
		Key: "oeffnungszeiten", Label: "Öffnungszeiten", Kind: KindGroup,
		Sub: []Def{{Key: "von", Label: "Von", Kind: KindTime}},
	}
	errs := CheckAll([]Def{group}, Data{
		Rows: map[string][]Values{"oeffnungszeiten": {{"von": "25:99"}}},
	})
	if len(errs) != 1 {
		t.Fatalf("the probe is wrong: %d reasons, wanted 1", len(errs))
	}
	var reason Reason
	for _, r := range errs {
		reason = r
	}

	english := reason.Text("en")
	if strings.Contains(english, "Zeile") {
		t.Errorf("the row frame is still German inside an English sentence: %q — the frame "+
			"and the reason inside it must be looked up in one language", english)
	}
	if !strings.Contains(english, "row") {
		t.Errorf("the English row frame is %q and does not contain \"row\"", english)
	}
	if !strings.Contains(english, "Öffnungszeiten") {
		t.Errorf("the reason %q has lost the group's label", english)
	}
}
