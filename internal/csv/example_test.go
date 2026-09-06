package csv

import (
	"bytes"
	"strings"
	"testing"
)

// The other direction: the example file somebody downloads before they write
// one of their own.
//
// Two things are checked here and nothing else. First the BOM, without which
// Excel turns an accented character into mojibake. Second, that no cell is read
// as a formula when the file is opened on somebody else's machine — the same
// rule `plugins/kontaktformular/csv.go` already writes down once, here in its
// second copy, because a file in `package main` cannot be imported.

// D-16: a cell that begins with = + - @ tab or carriage return is evaluated as
// a formula by Excel and LibreOffice as soon as the file is opened. A leading
// apostrophe takes the meaning out of it.
func TestDefuseTakesTheMeaningOutOfFormulas(t *testing.T) {
	cases := []struct {
		name string
		cell string
		want string
	}{
		{"equals sign", "=1+1", "'=1+1"},
		{"plus", "+41 79 000 00 00", "'+41 79 000 00 00"},
		{"minus", "-5", "'-5"},
		{"at sign", "@SUM(A1)", "'@SUM(A1)"},
		{"tab", "\tHello", "'\tHello"},
		{"carriage return", "\rHello", "'\rHello"},
		{"empty", "", ""},
		{"ordinary", "Apple", "Apple"},
		{"equals sign in the middle", "1=1", "1=1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := defuse([]string{c.cell})
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("defuse(%q) = %q, expected %q", c.cell, got, c.want)
			}
		})
	}
}

// Without the byte-order mark Excel does not open the file as UTF-8.
func TestExampleWritesTheBOM(t *testing.T) {
	out, err := Example([]string{"Title", "Size"}, nil)
	if err != nil {
		t.Fatalf("Example: %v", err)
	}
	if !bytes.HasPrefix(out, bom) {
		t.Errorf("the file begins with % x, expected % x", out[:3], bom)
	}
}

// The quoting and the separator are encoding/csv's business. The way back
// through this package's reader is the proof that nothing here is assembled by
// hand.
func TestExampleDoesNotDoItsOwnQuoting(t *testing.T) {
	header := []string{`Title, long`, `Measure "large"`, "Text\nwith a break"}
	rows := [][]string{{"a,b", `c"d`, "e"}}

	out, err := Example(header, rows)
	if err != nil {
		t.Fatalf("Example: %v", err)
	}

	r, err := New(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := r.Header(); !equal(got, header) {
		t.Errorf("header read back = %q, expected %q", got, header)
	}
	row, ok := r.Next()
	if !ok {
		t.Fatal("no data row read back")
	}
	if !equal(row.Cells, rows[0]) {
		t.Errorf("row read back = %q, expected %q", row.Cells, rows[0])
	}
}

// An example file without sample rows is the BOM and the header row, nothing
// else.
func TestExampleWithoutSampleRows(t *testing.T) {
	out, err := Example([]string{"Title", "Slug"}, nil)
	if err != nil {
		t.Fatalf("Example: %v", err)
	}
	if got := string(bytes.TrimPrefix(out, bom)); got != "Title,Slug\n" {
		t.Errorf("file = %q, expected %q", got, "Title,Slug\n")
	}
	if strings.Count(string(out), "\n") != 1 {
		t.Errorf("file has %d line breaks, expected 1", strings.Count(string(out), "\n"))
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
