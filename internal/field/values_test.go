package field

import (
	"reflect"
	"strings"
	"testing"
)

// SplitValues and JoinValues are the one pair every multi-valued field value
// goes through — the page form, the resolution for the theme and the journey
// through an archive. Everything here is the promise made to phase 9: the
// importer inherits these two functions rather than inventing a third spelling.
func TestSplitValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		will []string
	}{
		{"zwei Zeilen", "eiche\nbuche", []string{"eiche", "buche"}},
		{"nichts", "", nil},
		{"nur Leerraum", "  \n\n\t", nil},
		{"leere Zeilen fallen weg", "a\n\n b ", []string{"a", "b"}},
		{"Reihenfolge bleibt", "esche\nbuche\neiche", []string{"esche", "buche", "eiche"}},
		{"Doppelte bleiben doppelt", "a\na", []string{"a", "a"}},
		{"eine einzige Zeile", "eiche", []string{"eiche"}},
	}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			if got := SplitValues(f.raw); !reflect.DeepEqual(got, f.will) {
				t.Errorf("SplitValues(%q) = %#v, wollte %#v", f.raw, got, f.will)
			}
		})
	}
}

func TestJoinValues(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		will   string
	}{
		{"nichts", nil, ""},
		{"leere Liste", []string{}, ""},
		{"leere Einträge fallen weg", []string{"", "a", ""}, "a"},
		{"nur leere Einträge", []string{"", "", ""}, ""},
		{"Doppelte bleiben doppelt", []string{"a", "a"}, "a\na"},
		{"Reihenfolge bleibt, kein Sortieren", []string{"esche", "buche", "eiche"}, "esche\nbuche\neiche"},
		{"wird beschnitten", []string{" eiche ", "buche"}, "eiche\nbuche"},
	}
	for _, f := range cases {
		t.Run(f.name, func(t *testing.T) {
			if got := JoinValues(f.values); got != f.will {
				t.Errorf("JoinValues(%#v) = %q, wollte %q", f.values, got, f.will)
			}
		})
	}
}

// The real promise: the two are inverses of each other as long as no entry is
// empty and none is untrimmed. A value cannot itself contain a line break,
// because the options it comes from are read line by line.
func TestValuesRundreise(t *testing.T) {
	for _, v := range [][]string{
		{"eiche"},
		{"eiche", "buche", "esche"},
		{"a", "a"},
		{"esche", "buche", "eiche"},
	} {
		back := SplitValues(JoinValues(v))
		if !reflect.DeepEqual(back, v) {
			t.Errorf("SplitValues(JoinValues(%#v)) = %#v", v, back)
		}
		// The counting invariant, measured along in every case: one entry can
		// never become more than one value. A later caller whose values do not
		// come from a closed list — phase 9's CSV column — therefore cannot
		// mint an extra value.
		nichtLeer := 0
		for _, e := range v {
			if strings.TrimSpace(e) != "" {
				nichtLeer++
			}
		}
		if len(back) > nichtLeer {
			t.Errorf("%d non-empty entries became %d values: %#v", nichtLeer, len(back), back)
		}
		// Zweimal speichern muss dieselbe Zeichenkette ergeben.
		einmal := JoinValues(v)
		if zweimal := JoinValues(SplitValues(einmal)); zweimal != einmal {
			t.Errorf("not idempotent: %q then %q", einmal, zweimal)
		}
	}
}

// The empty entries fall away when joining, the duplicates do not. That is the
// edge a checkbox field hangs off: the hidden sentinel sends an empty entry
// along, and a partly ticked group must not notice it.
func TestJoinValuesWaechterUndDoppelte(t *testing.T) {
	if got := SplitValues(JoinValues([]string{"a", "", "a"})); !reflect.DeepEqual(got, []string{"a", "a"}) {
		t.Errorf("[a,\"\",a] came back as %#v, wanted [a a]", got)
	}
}

// Being multi-valued is stated in the form field's name, and that name is
// minted in exactly one place. If the marking is spelled out anywhere else as
// well, the two can drift apart.
func TestFieldNameCarriesTheMarker(t *testing.T) {
	multi := Def{Kind: KindMulti, Key: "sorten"}
	if got, will := multi.FieldName(), "feld_sorten[]"; got != will {
		t.Errorf("FieldName() = %q, wollte %q", got, will)
	}
	if !multi.IsMultiValued() {
		t.Error("IsMultiValued() = false for a multi-choice")
	}
	if got, will := multi.NameSuffix(), "[]"; got != will {
		t.Errorf("NameSuffix() = %q, wollte %q", got, will)
	}

	// And no other kind carries it — otherwise every existing field would be
	// called something else from today and every stored value would be
	// silently gone.
	for _, k := range Kinds {
		if k.Kind == KindMulti {
			continue
		}
		d := Def{Kind: k.Kind, Key: "sorten"}
		if got, will := d.FieldName(), "feld_sorten"; got != will {
			t.Errorf("FieldName() for %q = %q, wanted %q", k.Kind, got, will)
		}
		if d.IsMultiValued() {
			t.Errorf("IsMultiValued() = true for %q", k.Kind)
		}
	}
}

// A value that is not on the list is reported and not stored. The options are a
// closed vocabulary; no arbitrary string may come in through a row of ticks.
func TestMultipleChoiceCheck(t *testing.T) {
	d := Def{Label: "Sorten", Kind: KindMulti, Choices: []string{"Eiche", "Buche", "Esche"}}

	if reason := Check(d, JoinValues([]string{"Eiche", "Esche"})); !reason.Empty() {
		t.Errorf("Check on two valid values = %q, expected fine", reason)
	}
	reason := Check(d, JoinValues([]string{"Eiche", "Ahorn"}))
	if reason.Empty() {
		t.Fatal("„Ahorn“ wurde durchgelassen")
	}
	if !strings.Contains(reason.String(), "Ahorn") {
		t.Errorf("the message does not name the faulty value: %q", reason)
	}
	// Empty on an optional field is fine, on a required one it is not — the
	// guard at the top of Check decides that and has to stay that way.
	if reason := Check(d, ""); !reason.Empty() {
		t.Errorf("empty on an optional field = %q", reason)
	}
	required := d
	required.Required = true
	if reason := Check(required, ""); reason.Empty() {
		t.Error("empty on a required field was let through")
	}
}

// A multi-valued field reaches the theme as a list, not as a string, and the
// list is empty rather than nil-confusing when nothing is stored.
func TestAMultipleChoiceResolved(t *testing.T) {
	defs := []Def{{Key: "sorten", Label: "Sorten", Kind: KindMulti,
		Choices: []string{"Eiche", "Buche", "Esche"}}}

	got := Resolve(defs, Data{Values: Values{"sorten": "Eiche\nEsche"}}, Links{})
	values, ok := got["sorten"].([]string)
	if !ok {
		t.Fatalf("sorten kam als %T, wollte []string", got["sorten"])
	}
	if !reflect.DeepEqual(values, []string{"Eiche", "Esche"}) {
		t.Errorf("sorten = %#v", values)
	}

	leer := Resolve(defs, Data{Values: Values{}}, Links{})
	if values, ok := leer["sorten"].([]string); !ok || len(values) != 0 {
		t.Errorf("resolved empty = %#v (%T), wanted an empty []string", leer["sorten"], leer["sorten"])
	}

	// List leaves the empty field out and turns the filled one into readable
	// text — or a theme that takes .Text prints a lump.
	entries := List(defs, Data{Values: Values{"sorten": "Eiche\nEsche"}}, Links{})
	if len(entries) != 1 {
		t.Fatalf("List = %+v, wollte einen Eintrag", entries)
	}
	if !reflect.DeepEqual(entries[0].Values, []string{"Eiche", "Esche"}) {
		t.Errorf("Entry.Values = %#v", entries[0].Values)
	}
	if got, will := entries[0].Text, "Eiche, Esche"; got != will {
		t.Errorf("Entry.Text = %q, wollte %q", got, will)
	}
	if leer := List(defs, Data{Values: Values{}}, Links{}); len(leer) != 0 {
		t.Errorf("the empty field is in the list: %+v", leer)
	}

	// And Filled has to know the list, or the whole field panel disappears on
	// a page that carries only multi-valued fields.
	if !Filled(Resolve(defs, Data{Values: Values{"sorten": "Eiche"}}, Links{})) {
		t.Error("Filled = false although a value is there")
	}
	if Filled(Resolve(defs, Data{Values: Values{}}, Links{})) {
		t.Error("Filled = true on an empty page")
	}
}

// JoinValues defends its own separator.
//
// SplitValues' doc comment used to say a value could not itself contain a line
// break, because the options it comes from are read line by line. That was a
// statement about the CALLERS and not about the function: hand it an entry with
// a line break in it and two values came back where one was passed in.
//
// Today the closed list of options in Check's KindMulti arm catches that. It
// falls away as soon as the caller is a CSV column — and JoinValues is built to
// be inherited, explicitly (D-02). So the premise is enforced where the
// exported contract stands.
func TestJoinValuesVerteidigtSeinTrennzeichen(t *testing.T) {
	back := SplitValues(JoinValues([]string{"a\nb", "c"}))
	if len(back) != 2 {
		t.Fatalf("two entries became %d values: %#v", len(back), back)
	}
	if back[0] != "a b" {
		t.Errorf("the first value = %q, wanted \"a b\"", back[0])
	}

	// Every spelling of the line break, and the carriage-return form yields one
	// space and not two.
	for _, f := range []struct {
		raw  string
		will string
	}{
		{"a\nb", "a b"},
		{"a\r\nb", "a b"},
		{"a\rb", "a b"},
		{"a\n\nb", "a  b"},
	} {
		if got := JoinValues([]string{f.raw}); got != f.will {
			t.Errorf("JoinValues([%q]) = %q, wollte %q", f.raw, got, f.will)
		}
	}

	// Only the separator is folded: two spaces inside a valid value stay two
	// spaces. Anybody reaching for a function here that collapses every
	// whitespace swallows them.
	if got := JoinValues([]string{"eiche  rot"}); got != "eiche  rot" {
		t.Errorf("JoinValues([\"eiche  rot\"]) = %q — the whitespace inside the value was touched", got)
	}

	// And the folding stays idempotent over its own output.
	einmal := JoinValues([]string{"a\nb", "c"})
	if zweimal := JoinValues(SplitValues(einmal)); zweimal != einmal {
		t.Errorf("not idempotent: %q then %q", einmal, zweimal)
	}
}
