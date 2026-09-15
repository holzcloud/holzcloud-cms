package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The four positions a citation can occupy, and the one that was wrong.
//
// Case "doc comment" is not hypothetical. The first version of this tool walked
// backwards only, and rewrote a citation of menu.go's menuOfWebsite — which is
// menuOfWebsite's own doc comment — into a citation of menuItemNode, the
// declaration above it. A tool written to remove a lie from the comments put a
// new one in. This test is what makes that impossible to do twice.
func TestACitationIsAttributedToTheDeclarationItBelongsTo(t *testing.T) {
	lines := []string{
		"package example",            // 1
		"",                           // 2
		"// menuItemNode is a",       // 3
		"// node.",                   // 4
		"type menuItemNode struct {", // 5
		"\tID int64",                 // 6
		"}",                          // 7
		"",                           // 8
		"// menuOfWebsite returns the menu named by menuID, but only",   // 9
		"// when it belongs to the website — which is the whole guard.", // 10
		"func (h *Handler) menuOfWebsite(id int64) error {",             // 11
		"\tif id == 0 {", // 12
		"\t\treturn nil", // 13
		"\t}",            // 14
		"\treturn nil",   // 15
		"}",              // 16
		"",               // 17
	}

	for _, tc := range []struct {
		name string
		at   int
		want string
	}{
		{"a doc comment belongs to what it documents", 9, "menuOfWebsite"},
		{"the last line of a doc comment too", 10, "menuOfWebsite"},
		{"the declaration line itself", 11, "menuOfWebsite"},
		{"a line in the body", 13, "menuOfWebsite"},
		{"a type's doc comment", 3, "menuItemNode"},
		{"a line inside a type", 6, "menuItemNode"},
		// Between two declarations there is nothing to name, and naming the one
		// before would be the original bug pointing the other way.
		{"the blank line after a body", 17, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := enclosing(lines, tc.at); got != tc.want {
				t.Errorf("enclosing(line %d) = %q, want %q", tc.at, got, tc.want)
			}
		})
	}
}

// A grouped declaration introduces no single name, so there is nothing to put
// in a citation and the number stays.
func TestAGroupedDeclarationNamesNothing(t *testing.T) {
	lines := []string{
		"var (",   // 1
		"\ta = 1", // 2
		"\tb = 2", // 3
		")",       // 4
	}
	if got := enclosing(lines, 2); got != "" {
		t.Errorf("enclosing inside a var block = %q, want nothing to name", got)
	}
}

// A command's package is "main", and fifteen of them live in this tree.
//
// "main.asCSV" is what the first version wrote for a citation of the contact
// form plugin's own CSV writer. It is not a name anybody can look up, and it
// was in the tree for exactly one commit.
func TestACommandIsNamedByItsDirectory(t *testing.T) {
	dir := t.TempDir()
	plugin := dir + "/plugins/kontaktformular"
	other := dir + "/internal/csv"
	mustWrite(t, plugin+"/csv.go", "package main\n")
	mustWrite(t, other+"/csv.go", "package csv\n")

	if got := reference(other+"/csv.go", plugin+"/csv.go", "asCSV"); got == "main.asCSV" {
		t.Errorf("reference = %q — fifteen packages in this tree are called main", got)
	} else if !strings.Contains(got, "kontaktformular") || !strings.Contains(got, "asCSV") {
		t.Errorf("reference = %q, want the directory and the name", got)
	}

	// An ordinary package still reads the ordinary way.
	if got := reference(plugin+"/csv.go", other+"/csv.go", "Read"); got != "csv.Read" {
		t.Errorf("reference = %q, want csv.Read", got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
