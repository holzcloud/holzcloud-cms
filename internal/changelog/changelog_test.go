package changelog

import (
	"html/template"
	"strings"
	"testing"
)

// The file parses into the versions its own opening promises.
//
// "The numbers are the same as the tags in the repository" — CHANGELOG.md says
// so about itself, and a release here that carries no number would be one no
// operator could match against what they are running.
func TestEveryReleaseHasANumberAndAnEntry(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("CHANGELOG.md parsed to nothing; the screen behind the version number is empty")
	}
	for _, r := range all {
		if strings.TrimSpace(r.Version) == "" {
			t.Error("a release parsed with no version number")
		}
		if strings.ContainsAny(r.Version, " \t—–") {
			t.Errorf("the version %q still carries part of its heading; the date was not split off", r.Version)
		}
		if len(strings.TrimSpace(string(r.HTML))) == 0 {
			t.Errorf("the entry for %q rendered to nothing", r.Version)
		}
	}
}

// The newest is first, because that is the order the file is written in and the
// order an operator reads it in after an update.
func TestTheNewestReleaseIsFirst(t *testing.T) {
	latest, ok := Latest()
	if !ok {
		t.Fatal("no releases")
	}
	if latest.Version != All()[0].Version {
		t.Errorf("Latest is %q but the list opens with %q", latest.Version, All()[0].Version)
	}
}

// Find is a lookup and never a path.
func TestFindAnswersOnlyForVersionsThatExist(t *testing.T) {
	if _, ok := Find("99.9"); ok {
		t.Error("a version that is not in the file was found")
	}
	if _, ok := Find("../../etc/passwd"); ok {
		t.Error("a path was treated as a version")
	}
	latest, _ := Latest()
	if _, ok := Find(latest.Version); !ok {
		t.Errorf("the newest version %q was not found by its own number", latest.Version)
	}
	// The build stamps a tag name, which carries the v the headings do not.
	if _, ok := Find("v" + latest.Version); !ok {
		t.Errorf("v%s was not found; the sidebar's own version string never matches", latest.Version)
	}
}

// The preamble is not a release. It explains what a changelog is, which is not
// what somebody asking "what changed" wants to read first.
func TestThePreambleIsNotAnEntry(t *testing.T) {
	for _, r := range All() {
		if strings.Contains(string(r.HTML), "Keep a Changelog") {
			t.Errorf("the file's own explanation ended up inside release %q", r.Version)
		}
	}
}

// Nothing reaches the screen that has not been through the sanitiser, which is
// the rule this project states as "never cast unsanitised goldmark output".
// The changelog is written by the people who build this program and not by a
// visitor — but the rule holds because it is a rule, and because a file is a
// thing that can be edited by whoever holds the repository.
func TestNoScriptSurvivesIntoAnEntry(t *testing.T) {
	for _, r := range All() {
		if strings.Contains(strings.ToLower(string(r.HTML)), "<script") {
			t.Errorf("release %q carries a script tag", r.Version)
		}
	}
}

// Major groups a long list without arithmetic.
func TestMajorIsTheFirstNumber(t *testing.T) {
	for in, want := range map[string]string{"2.2": "2", "1.10": "1", "3": "3", "": ""} {
		if got := (Release{Version: in}).Major(); got != want {
			t.Errorf("Major(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeries(t *testing.T) {
	for in, want := range map[string]string{
		"0.1.2": "0.1", "0.0.25": "0.0", "2.9.1": "2", "1.10": "1", "2": "2", "0.1": "0.1",
	} {
		if got := (Release{Version: in}).Series(); got != want {
			t.Errorf("Series(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeriesLabel(t *testing.T) {
	for in, want := range map[string]string{
		"0.1": "0.1.0", "0.2": "0.2.0", "0.0": "0.0.x", "2": "2.x", "1": "1.x",
	} {
		if got := SeriesLabel(in); got != want {
			t.Errorf("SeriesLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// The groups come out in the screen's order whatever the file's was, and what
// stands before the first heading is the opening.
func TestGroupsAreOrderedNewImprovedFixedNote(t *testing.T) {
	body := strings.Split("Opening.\n\n### Behoben\n\nA fix.\n\n### Gut zu wissen\n\nA note.\n\n### Neu\n\nA feature.\n\n### Verbessert\n\nBetter.", "\n")
	lead, groups := splitGroups(body, func(lines []string) template.HTML {
		return template.HTML(strings.TrimSpace(strings.Join(lines, "\n")))
	})
	if lead != "Opening." {
		t.Errorf("lead = %q", lead)
	}
	var kinds []Kind
	for _, g := range groups {
		kinds = append(kinds, g.Kind)
	}
	want := []Kind{KindNew, KindImproved, KindFixed, KindNote}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("kinds = %v, want %v", kinds, want)
			break
		}
	}
	if groups[3].Title != "Gut zu wissen" {
		t.Errorf("a note lost its heading: %q", groups[3].Title)
	}
}

// An entry with no groups is its opening and nothing else, and still renders.
func TestAnEntryWithoutGroupsIsShownWhole(t *testing.T) {
	for _, rel := range All() {
		if len(rel.Groups) == 0 && rel.Lead == "" {
			t.Errorf("%s has neither an opening nor groups", rel.Version)
		}
	}
}
