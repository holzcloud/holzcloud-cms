package csv

import (
	"errors"
	"strings"
	"testing"
)

// The guard over a file that came from outside.
//
// `encoding/csv` knows no limit: none for rows, none for columns, none for
// cells, and a NUL byte is all the same to it. Nine defences independent of one
// another therefore stand in `csv.go`, and each of them has its own test here,
// named after the defence and not after the input — a failing line should say
// which rule was broken, not which text was left over. On top of that comes the
// off-by-one that would otherwise make every message of this phase wrong: the
// header is row 1, the first data row is row 2.
//
// No test here opens a database, builds a query or creates a directory. Were
// one of them to need one, the package would have taken on a dependency it is
// not allowed to have.

// read reads a whole file in and hands back the reader along with it, so a test
// can check Header() and Truncated() too.
func read(t *testing.T, source string) (*Reader, []Row) {
	t.Helper()
	r, err := New(strings.NewReader(source))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var rows []Row
	for {
		row, ok := r.Next()
		if !ok {
			break
		}
		rows = append(rows, row)
	}
	return r, rows
}

// D-11: The byte-order mark is stripped once, at the reader, before the header
// row. This is CSV import bug number one: Excel writes it, the first heading
// then reads "\ufeffTitle", matches no mapping, and the title column is
// silently lost.
func TestBOMIsStrippedOnce(t *testing.T) {
	r, rows := read(t, "\ufeffTitle,Text\nA,B\n")
	if got := r.Header()[0]; got != "Title" {
		t.Errorf("first heading = %q, expected %q", got, "Title")
	}
	if len(rows) != 1 {
		t.Fatalf("%d rows, expected 1", len(rows))
	}

	// A second BOM in the middle of the file is content and stays where it is.
	_, middle := read(t, "\ufeffTitle,Text\n\ufeffA,B\n")
	if got := middle[0].Cells[0]; got != "\ufeffA" {
		t.Errorf("cell = %q, a BOM in the content must not be stripped", got)
	}
}

// D-26, IMP-03 (boundary): The row number is the spreadsheet's own. The header
// is row 1, the first data row is row 2 — not the reader's record index and not
// the line number of the file.
func TestRowNumberIsTheSpreadsheetsOwn(t *testing.T) {
	if got := RowNumber(0); got != 2 {
		t.Errorf("RowNumber(0) = %d, expected 2", got)
	}
	if got := RowNumber(1); got != 3 {
		t.Errorf("RowNumber(1) = %d, expected 3", got)
	}

	_, rows := read(t, "Title\nA\nB\n")
	if rows[0].Number != 2 || rows[1].Number != 3 {
		t.Errorf("numbers = %d, %d, expected 2, 3", rows[0].Number, rows[1].Number)
	}
}

// D-09, IMP-09 (boundary): the row limit, at the limit and one step to either
// side of it. Reported, never silently applied.
func TestRowLimit(t *testing.T) {
	build := func(n int) string {
		var b strings.Builder
		b.WriteString("Title\n")
		for i := 0; i < n; i++ {
			b.WriteString("A\n")
		}
		return b.String()
	}

	r, rows := read(t, build(MaxRows))
	if len(rows) != MaxRows {
		t.Errorf("%d rows at exactly MaxRows, expected %d", len(rows), MaxRows)
	}
	if r.Truncated() {
		t.Error("exactly MaxRows rows count as truncated")
	}

	r, rows = read(t, build(MaxRows+1))
	if len(rows) != MaxRows {
		t.Errorf("%d rows at MaxRows+1, expected %d", len(rows), MaxRows)
	}
	if !r.Truncated() {
		t.Error("MaxRows+1 rows are silently truncated instead of reported")
	}
}

// D-38: the column limit, at the limit and one step to either side of it. A
// file with 5000 columns is allowed, passes every other limit, and produces a
// mapping screen with 5000 rows.
func TestColumnLimit(t *testing.T) {
	header := func(n int) string {
		columns := make([]string, n)
		for i := range columns {
			columns[i] = "c"
		}
		return strings.Join(columns, ",") + "\n"
	}

	r, err := New(strings.NewReader(header(MaxColumns)))
	if err != nil {
		t.Fatalf("exactly MaxColumns refused: %v", err)
	}
	if len(r.Header()) != MaxColumns {
		t.Errorf("%d columns, expected %d", len(r.Header()), MaxColumns)
	}

	if _, err := New(strings.NewReader(header(MaxColumns + 1))); !errors.Is(err, ErrTooManyColumns) {
		t.Errorf("MaxColumns+1 gives %v, expected ErrTooManyColumns", err)
	}
}

// D-10, IMP-09 (boundary + precision): the cell limit counts BYTES. One emoji
// costs four. A limit in runes would be a second definition of size standing
// beside the one in bytes.
func TestCellLimitInBytes(t *testing.T) {
	file := func(cell string) string {
		return "a,b\n1,2\n" + cell + ",x\n3,4\n"
	}

	_, rows := read(t, file(strings.Repeat("z", MaxCellBytes)))
	if rows[1].Error != "" {
		t.Errorf("exactly MaxCellBytes refused: %s", rows[1].Error)
	}

	_, rows = read(t, file(strings.Repeat("z", MaxCellBytes+1)))
	if rows[1].Error == "" {
		t.Error("MaxCellBytes+1 bytes let through")
	}
	if rows[1].Number != 3 {
		t.Errorf("the oversized cell reports row %d, expected 3", rows[1].Number)
	}
	// One row costs one row, not the file.
	if len(rows) != 3 || rows[2].Error != "" || rows[2].Number != 4 {
		t.Errorf("the row after the oversized one was not read cleanly: %+v", rows)
	}

	// MaxCellBytes bytes made of four-byte runes: allowed.
	four := strings.Repeat("😀", MaxCellBytes/4)
	if len(four) != MaxCellBytes {
		t.Fatalf("test setup: %d bytes, expected %d", len(four), MaxCellBytes)
	}
	_, rows = read(t, file(four))
	if rows[1].Error != "" {
		t.Errorf("MaxCellBytes bytes made of four-byte runes refused: %s", rows[1].Error)
	}

	// MaxCellBytes RUNES made of four-byte runes: refused, because that is four
	// times as many bytes.
	_, rows = read(t, file(strings.Repeat("😀", MaxCellBytes)))
	if rows[1].Error == "" {
		t.Error("MaxCellBytes runes of 4 bytes each let through — the limit counts runes instead of bytes")
	}
}

// D-13, IMP-09 (ordering): a short row does not shift. The missing cells are
// empty IN PLACE; cell 5 of a row with three cells is empty and not the value
// of cell 3.
func TestShortRowDoesNotShift(t *testing.T) {
	_, rows := read(t, "a,b,c,d,e\n1,2,3\n")
	row := rows[0]
	if len(row.Cells) != 5 {
		t.Fatalf("%d cells, expected 5: %q", len(row.Cells), row.Cells)
	}
	if row.Cells[2] != "3" {
		t.Errorf("cell 3 = %q, expected %q", row.Cells[2], "3")
	}
	if row.Cells[3] != "" || row.Cells[4] != "" {
		t.Errorf("cells 4 and 5 = %q, %q, expected empty", row.Cells[3], row.Cells[4])
	}
}

// D-12: a stray quote does not eat the rest of the file. LazyQuotes = true
// turns it into one bad row instead of an error for the whole file.
func TestStrayQuoteDoesNotEatTheRest(t *testing.T) {
	_, rows := read(t, "a,b,c\n1,two\"odd,3\n4,5,6\n")
	if len(rows) != 2 {
		t.Fatalf("%d rows, expected 2: %+v", len(rows), rows)
	}
	if rows[1].Number != 3 || rows[1].Cells[0] != "4" {
		t.Errorf("the row after the bad one = %+v, expected row 3 with 4,5,6", rows[1])
	}
}

// D-14, IMP-09 (adjacency): a quoted cell holding a separator stays one cell,
// and a quote spanning two lines does not raise the row number. That is why the
// counting happens in our own loop and never on the line of the file.
func TestQuotedCellWithSeparatorAndLineBreak(t *testing.T) {
	_, rows := read(t, "a,b,c\n1,\"two,three\",4\n")
	if len(rows[0].Cells) != 3 {
		t.Fatalf("%d cells, expected 3: %q", len(rows[0].Cells), rows[0].Cells)
	}
	if rows[0].Cells[1] != "two,three" {
		t.Errorf("middle cell = %q, expected %q", rows[0].Cells[1], "two,three")
	}

	_, rows = read(t, "a,b\n\"x\ny\",2\n3,4\n")
	if rows[0].Cells[0] != "x\ny" {
		t.Errorf("cell spanning two lines = %q", rows[0].Cells[0])
	}
	// The point: the following row sits on file line 4 and is row 3.
	if rows[1].Number != 3 {
		t.Errorf("row after the multi-line quote = number %d, expected 3", rows[1].Number)
	}
}

// D-14: a blank line is skipped and does not consume a row number.
func TestBlankLineDoesNotCount(t *testing.T) {
	_, rows := read(t, "a,b\n1,2\n\n3,4\n")
	if len(rows) != 2 {
		t.Fatalf("%d rows, expected 2", len(rows))
	}
	if rows[1].Number != 3 {
		t.Errorf("row after the blank line = number %d, expected 3", rows[1].Number)
	}
}

// D-15, IMP-09 (empty): three refusals before parsing, each with its own
// reason, so the screen can say which one applies.
func TestCheckBytesRefuses(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  error
	}{
		{"empty", []byte{}, ErrEmpty},
		{"nothing", nil, ErrEmpty},
		{"BOM only", []byte("\ufeff"), ErrOnlyBOM},
		{"NUL byte", []byte("a,b\n1,\x002\n"), ErrNULByte},
		{"NUL byte behind the BOM", []byte("\ufeffa\n\x00"), ErrNULByte},
		{"in order", []byte("a,b\n1,2\n"), nil},
		{"BOM and content", []byte("\ufeffa,b\n"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckBytes(c.bytes)
			if !errors.Is(err, c.want) {
				t.Errorf("CheckBytes = %v, expected %v", err, c.want)
			}
		})
	}
}

// IMP-01 (empty): a file with a header row and no data rows is a valid file.
// The sentence about it belongs on the screen, not here.
func TestHeaderOnlyIsNotAnError(t *testing.T) {
	r, rows := read(t, "Title,Text\n")
	if len(rows) != 0 {
		t.Errorf("%d rows, expected 0", len(rows))
	}
	if len(r.Header()) != 2 {
		t.Errorf("header = %q, expected two columns", r.Header())
	}
	if r.Truncated() {
		t.Error("an empty file counts as truncated")
	}
}

// A file without any records at all has no header row, and that is an error the
// screen can name.
func TestNoHeaderIsAnError(t *testing.T) {
	if _, err := New(strings.NewReader("")); !errors.Is(err, ErrNoHeader) {
		t.Errorf("New over an empty source = %v, expected ErrNoHeader", err)
	}
}

// TestWideRowCarriesNumbersAndNotASentence (WR-02, D-32): a row with more cells
// than the header hands its consumer two numbers, not an English sentence.
//
// It used to hand over one: fmt.Sprintf("row has %d cells, the header has %d"),
// set on Row.Error, carried forward as ReasonRowUnreadable's only argument and
// substituted into a German sentence on the report screen — "Diese Zeile liess //nolint:german — the broken report, quoted
// sich nicht lesen: row has 5 cells, the header has 3". The frame was a literal //nolint:german — the broken report, quoted
// tools/i18n could see; the half a person actually needs was not. Reachable by
// any row with a stray separator in it.
func TestWideRowCarriesNumbersAndNotASentence(t *testing.T) {
	_, rows := read(t, "a,b,c\n1,2,3,4,5\n")
	row := rows[0]

	if row.Error != "" {
		t.Errorf("the wide row carries the message %q — the numbers travel, the sentence is the template's", row.Error)
	}
	if row.HeaderWidth != 3 {
		t.Errorf("HeaderWidth = %d, want 3 — a consumer that never sees the header cannot tell a wide row without it", row.HeaderWidth)
	}
	if len(row.Cells) != 5 {
		t.Errorf("%d cells, want 5: no cell is dropped, the row is refused with both numbers named", len(row.Cells))
	}

	// An ordinary row is not wide, and a SHORT one is not either: fill pads it
	// out to the header's width in place.
	_, short := read(t, "a,b,c\n1,2\n")
	if len(short[0].Cells) != short[0].HeaderWidth {
		t.Errorf("a short row reports %d cells against a header of %d", len(short[0].Cells), short[0].HeaderWidth)
	}
}
