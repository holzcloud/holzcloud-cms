// Package csv reads an uploaded table without believing any part of it.
//
// Content arrives as a table. Somebody with two hundred products, events or
// breeding animals already has them in a spreadsheet, and a way into this CMS
// that asks for them to be retyped is not a way in at all. This package is the
// first half of that road: it takes an io.Reader and hands out one row at a
// time, bounded in every place where the standard library is not — encoding/csv
// has no limit on rows, none on columns, none on the size of a single cell, and
// no opinion at all about a NUL byte.
//
// What is deliberately *not* done here. No separator is sniffed: a wrong guess
// produces one column holding the whole row and says nothing, and the deferred
// alternative is an explicit choice on the upload screen, never sniffing.
// Nothing a cell names is fetched from a third party, which is the same refusal
// internal/admin/wordpress.go:63-71 already makes for a WXR file's pictures.
// And there is no database and no HTTP in this package: the checklist against a
// hostile file is nine independent defences, and each of them stays provable
// without db.Open for exactly as long as this package imports nothing but the
// standard library.
//
// Nothing here formats a sentence for a person either. Every message this
// package produces is a short technical reason for a developer or an error
// value; the sentences an operator reads are minted from reason codes in a
// template. Where a refusal has to reach the operator's report, what travels
// is the NUMBERS — Row.HeaderWidth beside len(Row.Cells) — and never a
// fmt.Sprintf'd fragment, because a fragment built here would be substituted
// into a translated sentence that tools/i18n cannot see into (D-32). The one
// message that does reach a screen through here is encoding/csv's own
// ParseError text, carried as ReasonRowUnreadable's argument, and Row.Error
// says why that exemption is a decision.
package csv

import (
	"bufio"
	"bytes"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MaxRows bounds one import.
//
// A file with more rows than this exists, but importing it in one go means one
// operator holding the single write connection for a very long time. The limit
// is reported rather than silently applied: saying the file was cut is honest,
// pretending it was not is not.
const MaxRows = 5000

// MaxColumns bounds the number of columns of the header.
//
// The mapping screen lists one row per column and stops being a screen a person
// can use somewhere past a few dozen; the mapping itself also travels in a
// query string while the operator steps through sample rows, and a hundred
// columns is about 3 kB there while five thousand would not be. Refused and
// reported, never silently truncated.
const MaxColumns = 100

// MaxCellBytes bounds one cell, in BYTES and not in runes.
//
// One 9 MB cell is perfectly legal inside the 10 MB the upload handler admits,
// and it would reach every consumer of a row at that size. Bytes because len()
// on the string is what the byte cap upstream also measures: a cap in runes
// would be a second definition of size standing beside the first, so one emoji
// costs four here.
const MaxCellBytes = 100000

// The refusals CheckBytes can return. Three separate reasons rather than one
// generic one, because the screen that shows them can only name what was wrong
// if the check told it apart.
var (
	// ErrEmpty: nothing at all was uploaded.
	ErrEmpty = errors.New("csv: empty file")
	// ErrOnlyBOM: the file is exactly the byte-order mark and nothing else,
	// which is what an empty sheet saved out of Excel looks like on disk.
	ErrOnlyBOM = errors.New("csv: file contains nothing but a byte-order mark")
	// ErrNULByte: the bytes contain \x00. Refused and not cleaned — a file
	// carrying a NUL is not a CSV somebody meant to upload, it is a binary
	// file with the wrong extension or something worse, and stripping the byte
	// would hand it forward as if it were fine.
	ErrNULByte = errors.New("csv: file contains a NUL byte")
)

// The refusals New can return.
var (
	// ErrNoHeader: the source held no record at all, so there is no header to
	// map anything against.
	ErrNoHeader = errors.New("csv: file has no header row")
	// ErrTooManyColumns: the header has more than MaxColumns columns.
	ErrTooManyColumns = errors.New("csv: header has more columns than MaxColumns")
)

// bom is the UTF-8 byte-order mark, as bytes, so it can be recognised before
// anything has been decoded.
var bom = []byte{0xEF, 0xBB, 0xBF}

// CheckBytes is the check on the raw bytes, before anything is parsed.
//
// It runs at stage time, on the upload screen, while the operator is still
// holding the file — a refusal there names the file they just picked, which a
// refusal three screens later cannot.
func CheckBytes(b []byte) error {
	if len(b) == 0 {
		return ErrEmpty
	}
	if bytes.Equal(b, bom) {
		return ErrOnlyBOM
	}
	if bytes.IndexByte(b, 0x00) >= 0 {
		return ErrNULByte
	}
	return nil
}

// RowNumber turns the index of a data row into the row number the operator
// reads.
//
// This is the one place that number is minted, and it is exported for that
// reason: the dry run, the sample-row stepper and the report all call it, so
// the "row 2" on the mapping screen is the same row as the "row 2" in the
// report. It is the number the spreadsheet shows — the header is row 1 and the
// first data row is row 2 — and it is neither encoding/csv's record index nor a
// line number: a quoted cell may span lines and blank lines are skipped, so
// line and row are two different quantities. stdcsv.Reader.FieldPos stays
// available should a *line* ever need naming beside a row; every requirement of
// this phase asks for the row.
func RowNumber(index int) int {
	return index + 2
}

// Row is one data row as it was read.
type Row struct {
	// Number is the row number the operator's spreadsheet shows.
	Number int
	// Cells holds one entry per header column, padded out so a short row's
	// missing cells are empty in position rather than shifted.
	Cells []string
	// HeaderWidth is how many columns the header had.
	//
	// Carried on the row so a consumer that never sees the header can still
	// tell a row that brought MORE cells than the header from one that did
	// not: fill pads a short row out to this width, so len(Cells) is equal to
	// it for every ordinary row and greater for a row with cells to spare.
	// Zero on a row nothing read from a file built, and a consumer treats that
	// as "not known" rather than as "no columns".
	//
	// It is here rather than as a finished sentence in Error because the
	// numbers are ARGUMENTS: the sentence an operator reads about them is a
	// {{tf}} literal in the template, where tools/i18n can see it (D-32). An
	// English fragment built here with fmt.Sprintf would be substituted into a
	// German sentence on the report screen and the translation gate would
	// never know.
	HeaderWidth int
	// Error empty means the row read cleanly. A row that carries an Error
	// still carries whatever cells were read, so a screen can show the
	// operator what the reader managed to see.
	//
	// What may stand in it: encoding/csv's own ParseError text, and the
	// reader's own technical sentence about an oversized cell. Both are
	// developer-facing. The first is stdlib prose and is deliberately not
	// this phase's to translate — it is carried through as the single
	// argument of ReasonRowUnreadable and that exemption is a decision, not an
	// oversight. The second never reaches a screen: csvimport.CheckRow
	// measures every cell itself and answers ReasonCellTooLong, with the
	// column and the size as arguments, before it looks at Error at all.
	Error string
}

// Reader hands out the rows of one file, one at a time.
//
// It streams. wxr.Parse materialises its whole document and its Export carries
// an []Item; that is the one thing not copied from it here, because a CSV file
// is the size of a spreadsheet and encoding/csv genuinely reads one record at a
// time.
type Reader struct {
	cr     *stdcsv.Reader
	header []string
	// read counts the data rows handed out. Counted here rather than taken
	// from a line number, which is the whole of D-14: blank lines are skipped
	// and a quoted cell may span lines.
	read      int
	truncated bool
	done      bool
}

// New reads the header and prepares the reader.
func New(r io.Reader) (*Reader, error) {
	br := bufio.NewReader(r)

	// The byte-order mark is stripped once, here, before the header is read.
	// Doing it at each later comparison instead is the number one CSV import
	// bug: Excel writes the mark, the first header arrives as "\ufeffTitel",
	// matches no mapping, and the title column is silently lost.
	// plugins/kontaktformular/csv.go:33 writes this mark on purpose, which is
	// the proof that files carrying it are the normal case here and not the
	// exception.
	if head, err := br.Peek(len(bom)); err == nil && bytes.Equal(head, bom) {
		if _, err := br.Discard(len(bom)); err != nil {
			return nil, fmt.Errorf("csv: discard byte-order mark: %w", err)
		}
	}

	cr := stdcsv.NewReader(br)
	// An untrusted upload with a stray quote must produce one bad row, not
	// swallow the rest of the file.
	cr.LazyQuotes = true
	// A short row becomes a reported row carrying its own row number instead of
	// an error carrying a line number. Set before the header is read, because
	// reading the header would otherwise fix the count to the header's width.
	cr.FieldsPerRecord = -1
	// ReuseRecord stays at its default false: a caller keeps a Row after the
	// next Next, and reusing the backing array would rewrite a row that has
	// already been handed out.

	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrNoHeader
		}
		return nil, fmt.Errorf("csv: read header: %w", err)
	}
	if len(header) > MaxColumns {
		return nil, fmt.Errorf("%w: %d", ErrTooManyColumns, len(header))
	}

	return &Reader{cr: cr, header: header}, nil
}

// Header is the header row, with the byte-order mark already gone.
func (r *Reader) Header() []string { return r.header }

// Truncated says the file held more rows than MaxRows. The cap is reported this
// way rather than applied in silence.
func (r *Reader) Truncated() bool { return r.truncated }

// Next hands out the next data row. The second return value is false once the
// file is done or the row cap has been reached.
func (r *Reader) Next() (Row, bool) {
	if r.done {
		return Row{}, false
	}

	if r.read >= MaxRows {
		r.done = true
		// One more read, only to find out whether there was more file. That is
		// the difference between a file of exactly MaxRows rows, which is
		// complete, and one of MaxRows + 1, which was cut and must say so.
		if _, err := r.cr.Read(); err == nil || isParseError(err) {
			r.truncated = true
		}
		return Row{}, false
	}

	rec, err := r.cr.Read()
	if err != nil && !isParseError(err) {
		// io.EOF, or something the reader cannot read past.
		r.done = true
		return Row{}, false
	}
	r.read++

	row := Row{Number: RowNumber(r.read - 1), Cells: r.fill(rec), HeaderWidth: len(r.header)}
	if err != nil {
		// A row encoding/csv could not make sense of is one bad row and not a
		// bad file; it can read on past this one.
		row.Error = err.Error()
		return row, true
	}
	// A row with more cells than the header is NOT reported here. The two
	// numbers travel on the row itself, and the sentence about them is a
	// {{tf}} literal the translation gate can see; a fmt.Sprintf here put an
	// English fragment inside a German sentence on the report screen.
	for i, cell := range row.Cells {
		// len() on the string: MaxCellBytes counts bytes, so one emoji costs
		// four. The row is reported with its number and the reader carries on,
		// so one hostile cell costs one row rather than the file.
		if len(cell) > MaxCellBytes {
			row.Error = fmt.Sprintf("cell in column %s is %d bytes, the limit is %d",
				r.columnName(i), len(cell), MaxCellBytes)
			break
		}
	}
	return row, true
}

// fill pads a short row out to the header's width, so the missing cells are
// empty in position rather than shifting every later cell one to the left. A
// row that is longer than the header keeps all of its cells: nothing is
// dropped, and Next reports the mismatch.
func (r *Reader) fill(rec []string) []string {
	if len(rec) >= len(r.header) {
		return rec
	}
	padded := make([]string, len(r.header))
	copy(padded, rec)
	return padded
}

// columnName names a column for a message: its header where there is one, its
// position otherwise.
func (r *Reader) columnName(i int) string {
	if i < len(r.header) && strings.TrimSpace(r.header[i]) != "" {
		return fmt.Sprintf("%q", r.header[i])
	}
	return fmt.Sprintf("%d", i+1)
}

// isParseError tells a row encoding/csv could not read from an error that ends
// the file.
func isParseError(err error) bool {
	var pe *stdcsv.ParseError
	return errors.As(err, &pe)
}
