package block

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// An own block kind carries the same field kinds a page does, and Phase 7 let
// every new one in through BlockKinds' exclusion list. Clean trimmed their
// values and capped them and checked nothing: a bundle, or a POST written by
// hand, could store a yes/no that reads "nein", a range of 99 on a field
// bounded 1–5, a time of 25:99 and a choice that is not on the list — and each
// of them went onto the public page exactly as written.
//
// The yes/no is the sharpest of them. renderOwn read everything that is not "0"
// as yes, so the stored "nein" printed as yes: the defect field.go had already
// fixed for page fields, one carrier over. The form itself posts value="1", so
// nobody clicking through the editor ever saw it.
//
// Found by the v1.6 milestone audit, which probed it before these tests existed.

// checkedKind carries one field of every kind whose value Check can refuse.
func checkedKind() Set {
	return Set{Own: []Own{{
		ID: 3, Key: "probe", Name: "Probe",
		Fields: []field.Def{
			{Key: "janein", Label: "Ja/Nein", Kind: field.KindBool},
			{Key: "farbe", Label: "Farbe", Kind: field.KindMulti,
				Choices: []string{"rot", "blau"}, MaxValues: 1},
			{Key: "stufe", Label: "Stufe", Kind: field.KindRange, RangeMin: "1", RangeMax: "5"},
			{Key: "zeit", Label: "Zeit", Kind: field.KindTime},
			{Key: "wahl", Label: "Wahl", Kind: field.KindChoice, Choices: []string{"a", "b"}},
		},
	}}}
}

// TestCleanRefusesWhatCheckRefusesInAnOwnKind is the gate itself: a value the
// page form would have refused is not stored in a block either.
func TestCleanRefusesWhatCheckRefusesInAnOwnKind(t *testing.T) {
	s := checkedKind()
	in := map[string]string{
		"janein": "vielleicht",
		"farbe":  "gruen\nrot\nblau",
		"stufe":  "99",
		"zeit":   "25:99",
		"wahl":   "<zzz>",
	}
	// Each refused value is named against its own field, so a failure says
	// which kind still gets through rather than that something did.
	for _, d := range checkedKind().Own[0].Fields {
		if reason := field.Check(d, in[d.Key]); reason == "" {
			t.Fatalf("the probe is wrong: field.Check accepts %s=%q", d.Key, in[d.Key])
		}
	}

	got := s.Clean([]Block{{Type: "probe", Fields: in}})
	if len(got) == 0 {
		return // nothing stored at all is a refusal too
	}
	for key, value := range got[0].Fields {
		t.Errorf("Clean kept %s=%q, which field.Check refuses — a bundle can put it on the public page", key, value)
	}
}

// TestCleanNormalisesAnOwnKindLikeAPage is the other direction: what is valid
// stays, in the one spelling the rest of the tree agrees on. Without this, a
// fix that simply dropped every value would pass the test above.
func TestCleanNormalisesAnOwnKindLikeAPage(t *testing.T) {
	got := checkedKind().Clean([]Block{{Type: "probe", Fields: map[string]string{
		"janein": "ja",
		"farbe":  "blau",
		"stufe":  "5",
		"zeit":   "09:30:00",
		"wahl":   "b",
	}}})
	if len(got) != 1 {
		t.Fatalf("%d blocks, want 1", len(got))
	}
	want := map[string]string{"janein": "1", "farbe": "blau", "stufe": "5", "zeit": "09:30", "wahl": "b"}
	for key, w := range want {
		if g := got[0].Fields[key]; g != w {
			t.Errorf("%s = %q after Clean, want %q", key, g, w)
		}
	}

	no := checkedKind().Clean([]Block{{Type: "probe", Fields: map[string]string{
		"janein": "nein", "wahl": "a",
	}}})
	if len(no) != 1 {
		t.Fatalf("%d blocks, want 1", len(no))
	}
	if v, kept := no[0].Fields["janein"]; kept {
		t.Errorf("a no is stored as %q; the tree spells no as the empty string", v)
	}
}

// TestAStoredNoIsNotRenderedAsYes holds the renderer on its own, without Clean
// in front of it. Blocks stored before the gate existed still read "nein", and
// they are rendered again whenever their page is saved.
func TestAStoredNoIsNotRenderedAsYes(t *testing.T) {
	render := func(value string) string {
		return Render([]Block{{Type: "probe", Fields: map[string]string{
			"janein": value, "wahl": "a",
		}}}, checkedKind(), bilder(nil), markdown)
	}
	for _, no := range []string{"nein", "no", "false", "0"} {
		if html := render(no); strings.Contains(html, "hc-ja--janein") {
			t.Errorf("a stored %q rendered as yes:\n%s", no, html)
		}
	}
	// The control: yes still is yes, or a renderer that never sets the class
	// would pass.
	for _, yes := range []string{"1", "ja"} {
		if html := render(yes); !strings.Contains(html, "hc-ja--janein") {
			t.Errorf("a stored %q did not render as yes:\n%s", yes, html)
		}
	}
}

// TestAMultiValueInABlockIsReadValueByValue is FIELD-07 on the block renderer:
// the stored encoding is read through field.SplitValues, not printed as it
// lies in the column.
func TestAMultiValueInABlockIsReadValueByValue(t *testing.T) {
	html := Render([]Block{{
		Type:   "hinweis",
		Fields: map[string]string{"sorten": "Eiche\n<b>Buche</b>"},
	}}, artMitCode(), bilder(nil), markdown)

	if strings.Contains(html, "Eiche\n") {
		t.Errorf("the stored encoding reached the page as it lies in the column:\n%s", html)
	}
	for _, will := range []string{
		`hc-eigen__liste--sorten`,
		`<li>Eiche</li>`,
		`<li>&lt;b&gt;Buche&lt;/b&gt;</li>`,
	} {
		if !strings.Contains(html, will) {
			t.Errorf("%q missing from the output:\n%s", will, html)
		}
	}
}
