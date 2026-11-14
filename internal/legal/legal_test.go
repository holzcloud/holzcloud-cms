package legal

import (
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestParseSetIsCanonical(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"us, EU", "eu,us"},
		{" CH ,ch,uk ", "ch,uk"},
		{"eu,xx,🙂", "eu"},
		{"", ""},
		{"  ,  ", ""},
		{"eu ch\tus", "eu,ch,us"},
		{"US,UK,CH,EU", "eu,ch,uk,us"},
	}
	for _, c := range cases {
		got := ParseSet(c.raw)
		if got.String() != c.want {
			t.Errorf("ParseSet(%q) = %q, want %q", c.raw, got.String(), c.want)
		}
		if (c.want == "") != got.Empty() {
			t.Errorf("ParseSet(%q).Empty() = %v", c.raw, got.Empty())
		}
	}
	for _, s := range legalSubsets() {
		if back := ParseSet(s.String()); back != s {
			t.Errorf("ParseSet(%q) = %q — the canonical form does not read back as itself", s.String(), back.String())
		}
	}
}

func TestHasAndListFollowTheSet(t *testing.T) {
	s := ParseSet("us,eu")
	if !s.Has(EU) || !s.Has(US) || s.Has(CH) || s.Has(UK) {
		t.Errorf("Has disagrees with %q", s.String())
	}
	got := s.List()
	if len(got) != 2 || got[0] != EU || got[1] != US {
		t.Errorf("List() = %v, want [eu us] in the fixed order", got)
	}
	if len(Set{}.List()) != 0 || !(Set{}).Empty() {
		t.Error("the zero value is not the empty set")
	}
}

func TestDefaultIsEUAndCH(t *testing.T) {
	if got := Default().String(); got != "eu,ch" {
		t.Errorf("Default() = %q, want eu,ch (D-05)", got)
	}
}

// The Go side and the SQL side list the same sixteen spellings; neither can
// learn a seventeenth without the other.
func TestTheSixteenSpellingsAreTheCheckList(t *testing.T) {
	raw, err := os.ReadFile("../db/migrations/00063_legal_frameworks.sql")
	if err != nil {
		t.Fatalf("the migration is not there: %v", err)
	}
	body := string(raw)
	at := strings.Index(body, "legal_frameworks IN (")
	if at < 0 {
		t.Fatal("the migration carries no CHECK over legal_frameworks")
	}
	end := strings.Index(body[at:], ")")
	if end < 0 {
		t.Fatal("the CHECK list is not closed")
	}
	var sqlSide []string
	for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(body[at:at+end], -1) {
		sqlSide = append(sqlSide, m[1])
	}
	var goSide []string
	for _, s := range legalSubsets() {
		goSide = append(goSide, s.String())
	}
	sort.Strings(sqlSide)
	sort.Strings(goSide)
	if strings.Join(sqlSide, "|") != strings.Join(goSide, "|") {
		t.Errorf("the CHECK lists %q\nString() produces %q", sqlSide, goSide)
	}
	if len(goSide) != 16 {
		t.Errorf("%d spellings, want 16", len(goSide))
	}
}

func TestGPCOnlyCountsExactlyOne(t *testing.T) {
	for _, v := range []string{"1", " 1 "} {
		h := http.Header{}
		h.Set(GPCHeader, v)
		if !GPC(h) {
			t.Errorf("Sec-GPC %q is not counted", v)
		}
	}
	for _, v := range []string{"0", "", "true", "yes", "11"} {
		h := http.Header{}
		h.Set(GPCHeader, v)
		if GPC(h) {
			t.Errorf("Sec-GPC %q is counted as an opt-out", v)
		}
	}
	if GPC(http.Header{}) {
		t.Error("no header is counted as an opt-out")
	}
	h := http.Header{}
	h.Add(GPCHeader, "0")
	h.Add(GPCHeader, "1")
	if !GPC(h) {
		t.Error("two headers, one of them exactly 1, are not counted")
	}
}

// legalSubsets is every set of All(), the empty one included.
func legalSubsets() []Set {
	all := All()
	var out []Set
	for mask := 0; mask < 1<<len(all); mask++ {
		var parts []string
		for i, f := range all {
			if mask&(1<<i) != 0 {
				parts = append(parts, string(f))
			}
		}
		out = append(out, ParseSet(strings.Join(parts, ",")))
	}
	return out
}
