package field

import "testing"

// Two holes in the same switch, found by reading it for what is NOT there.
//
// Check's switch has an arm for every kind that can be got wrong except
// KindBool, and the three writers that are not the admin form — the assistant,
// the archive import and the CSV import — all name CheckAll as their only
// per-kind gate and say so in their own comments. Only the CSV path carries a
// second, private reading of a boolean.
//
// The bounds arm has the other hole: NaN is a float64 that fails both
// comparisons, so it is inside every range there is.

// A stored "nein" is read by Resolve as true and every shipped theme prints
// the word "ja" for it. The value says one thing and the page says the other,
// and nothing between them notices.
func TestCheckReadsABooleanOrRefusesIt(t *testing.T) {
	d := Def{Key: "im_angebot", Label: "Im Angebot", Kind: KindBool}

	// The words a person writes, in the four spellings the CSV importer has
	// always accepted. Each has to survive Check AND come out of Clean as the
	// value Resolve reads correctly.
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"1", true}, {"ja", true}, {"yes", true}, {"wahr", true}, {"true", true}, {"x", true},
		{"0", false}, {"nein", false}, {"no", false}, {"falsch", false}, {"false", false},
		{"JA", true}, {"Nein", false},
	} {
		if r := Check(d, tc.in); r != "" {
			t.Errorf("Check refused %q: %q", tc.in, r)
			continue
		}
		cleaned := Clean([]Def{d}, Data{Values: Values{d.Key: tc.in}})
		got := Resolve([]Def{d}, cleaned, Links{})[d.Key]
		if got != tc.want {
			t.Errorf("%q was stored as %q and resolved to %v, want %v",
				tc.in, cleaned.Values[d.Key], got, tc.want)
		}
	}

	// And a value that is not a boolean at all must be refused rather than
	// silently read as yes.
	for _, bad := range []string{"vielleicht", "2", "-1", "ja bitte"} {
		if r := Check(d, bad); r == "" {
			t.Errorf("Check let %q through; Resolve reads it as %v",
				bad, Resolve([]Def{d}, Data{Values: Values{d.Key: bad}}, Links{})[d.Key])
		}
	}
}

// A range field's whole point is that the value lies between two bounds. NaN
// lies between any two: n < unten and n > oben are both false for it.
func TestNaNIsNotANumberInsideEveryRange(t *testing.T) {
	d := Def{Key: "plaetze", Label: "Sitzplätze", Kind: KindRange, RangeMin: "2", RangeMax: "12"}
	for _, bad := range []string{"NaN", "nan", "+Inf", "-Inf", "inf", "Infinity"} {
		if r := Check(d, bad); r == "" {
			n, _ := ParseNumber(bad)
			t.Errorf("Check accepted %q as a number between 2 and 12 (parsed as %v)", bad, n)
		}
	}
	// A plain number field has the same reading and must answer the same way:
	// a page that prints "NaN" where a number belongs is not a page that has a
	// number in it.
	plain := Def{Key: "menge", Label: "Menge", Kind: KindNumber}
	for _, bad := range []string{"NaN", "Inf"} {
		if r := Check(plain, bad); r == "" {
			t.Errorf("Check accepted %q as a number", bad)
		}
	}
	// The bounds themselves read through the same function, so a bound typed
	// as NaN must not silently disarm the check either.
	if _, ok := ParseNumber("NaN"); ok {
		t.Error("ParseNumber still reads NaN as a number — the bounds inherit it")
	}
}
