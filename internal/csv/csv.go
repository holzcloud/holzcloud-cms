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
// template.
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

// MaxSpalten bounds the number of columns of the header.
//
// The mapping screen lists one row per column and stops being a screen a person
// can use somewhere past a few dozen; the mapping itself also travels in a
// query string while the operator steps through sample rows, and a hundred
// columns is about 3 kB there while five thousand would not be. Refused and
// reported, never silently truncated.
const MaxSpalten = 100

// MaxCellBytes bounds one cell, in BYTES and not in runes.
//
// One 9 MB cell is perfectly legal inside the 10 MB the upload handler admits,
// and it would reach every consumer of a row at that size. Bytes because len()
// on the string is what the byte cap upstream also measures: a cap in runes
// would be a second definition of size standing beside the first, so one emoji
// costs four here.
const MaxCellBytes = 100000

// The refusals PruefeBytes can return. Three separate reasons rather than one
// generic one, because the screen that shows them can only name what was wrong
// if the check told it apart.
var (
	// ErrLeer: nothing at all was uploaded.
	ErrLeer = errors.New("csv: empty file")
	// ErrNurBOM: the file is exactly the byte-order mark and nothing else,
	// which is what an empty sheet saved out of Excel looks like on disk.
	ErrNurBOM = errors.New("csv: file contains nothing but a byte-order mark")
	// ErrNullbyte: the bytes contain \x00. Refused and not cleaned — a file
	// carrying a NUL is not a CSV somebody meant to upload, it is a binary
	// file with the wrong extension or something worse, and stripping the byte
	// would hand it forward as if it were fine.
	ErrNullbyte = errors.New("csv: file contains a NUL byte")
)

// The refusals Neu can return.
var (
	// ErrKeineKopfzeile: the source held no record at all, so there is no
	// header to map anything against.
	ErrKeineKopfzeile = errors.New("csv: file has no header row")
	// ErrZuVieleSpalten: the header has more than MaxSpalten columns.
	ErrZuVieleSpalten = errors.New("csv: header has more columns than MaxSpalten")
)

// bom is the UTF-8 byte-order mark, as bytes, so it can be recognised before
// anything has been decoded.
var bom = []byte{0xEF, 0xBB, 0xBF}

// PruefeBytes is the check on the raw bytes, before anything is parsed.
//
// It runs at stage time, on the upload screen, while the operator is still
// holding the file — a refusal there names the file they just picked, which a
// refusal three screens later cannot.
func PruefeBytes(b []byte) error {
	if len(b) == 0 {
		return ErrLeer
	}
	if bytes.Equal(b, bom) {
		return ErrNurBOM
	}
	if bytes.IndexByte(b, 0x00) >= 0 {
		return ErrNullbyte
	}
	return nil
}

// Zeilennummer turns the index of a data row into the row number the operator
// reads.
//
// This is the one place that number is minted, and it is exported for that
// reason: the dry run, the sample-row stepper and the report all call it, so
// the "Zeile 2" on the mapping screen is the same row as the "Zeile 2" in the
// report. It is the number the spreadsheet shows — the header is row 1 and the
// first data row is row 2 — and it is neither encoding/csv's record index nor a
// line number: a quoted cell may span lines and blank lines are skipped, so
// line and row are two different quantities. stdcsv.Reader.FieldPos stays
// available should a *line* ever need naming beside a row; every requirement of
// this phase asks for the row.
func Zeilennummer(index int) int {
	return index + 2
}

// Zeile is one data row as it was read.
type Zeile struct {
	// Nummer is the row number the operator's spreadsheet shows.
	Nummer int
	// Zellen holds one entry per header column, padded out so a short row's
	// missing cells are empty in position rather than shifted.
	Zellen []string
	// Fehler empty means the row read cleanly. A row that carries a Fehler
	// still carries whatever cells were read, so a screen can show the
	// operator what the reader managed to see.
	Fehler string
}

// Leser hands out the rows of one file, one at a time.
//
// It streams. wxr.Parse materialises its whole document and its Export carries
// an []Item; that is the one thing not copied from it here, because a CSV file
// is the size of a spreadsheet and encoding/csv genuinely reads one record at a
// time.
type Leser struct {
	r    *stdcsv.Reader
	kopf []string
	// gelesen counts the data rows handed out. Counted here rather than taken
	// from a line number, which is the whole of D-14: blank lines are skipped
	// and a quoted cell may span lines.
	gelesen       int
	abgeschnitten bool
	fertig        bool
}

// Neu reads the header and prepares the reader.
func Neu(r io.Reader) (*Leser, error) {
	br := bufio.NewReader(r)

	// The byte-order mark is stripped once, here, before the header is read.
	// Doing it at each later comparison instead is the number one CSV import
	// bug: Excel writes the mark, the first header arrives as "\ufeffTitel",
	// matches no mapping, and the title column is silently lost.
	// plugins/kontaktformular/csv.go:33 writes this mark on purpose, which is
	// the proof that files carrying it are the normal case here and not the
	// exception.
	if vorn, err := br.Peek(len(bom)); err == nil && bytes.Equal(vorn, bom) {
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
	// ReuseRecord stays at its default false: a caller keeps a Zeile after the
	// next Naechste, and reusing the backing array would rewrite a row that has
	// already been handed out.

	kopf, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrKeineKopfzeile
		}
		return nil, fmt.Errorf("csv: read header: %w", err)
	}
	if len(kopf) > MaxSpalten {
		return nil, fmt.Errorf("%w: %d", ErrZuVieleSpalten, len(kopf))
	}

	return &Leser{r: cr, kopf: kopf}, nil
}

// Kopf is the header row, with the byte-order mark already gone.
func (l *Leser) Kopf() []string { return l.kopf }

// Abgeschnitten says the file held more rows than MaxRows. The cap is reported
// this way rather than applied in silence.
func (l *Leser) Abgeschnitten() bool { return l.abgeschnitten }

// Naechste hands out the next data row. The second return value is false once
// the file is done or the row cap has been reached.
func (l *Leser) Naechste() (Zeile, bool) {
	if l.fertig {
		return Zeile{}, false
	}

	if l.gelesen >= MaxRows {
		l.fertig = true
		// One more read, only to find out whether there was more file. That is
		// the difference between a file of exactly MaxRows rows, which is
		// complete, and one of MaxRows + 1, which was cut and must say so.
		if _, err := l.r.Read(); err == nil || istParseFehler(err) {
			l.abgeschnitten = true
		}
		return Zeile{}, false
	}

	rec, err := l.r.Read()
	if err != nil && !istParseFehler(err) {
		// io.EOF, or something the reader cannot read past.
		l.fertig = true
		return Zeile{}, false
	}
	l.gelesen++

	z := Zeile{Nummer: Zeilennummer(l.gelesen - 1), Zellen: l.fuellen(rec)}
	if err != nil {
		// A row encoding/csv could not make sense of is one bad row and not a
		// bad file; it can read on past this one.
		z.Fehler = err.Error()
		return z, true
	}
	if len(rec) > len(l.kopf) {
		z.Fehler = fmt.Sprintf("row has %d cells, the header has %d", len(rec), len(l.kopf))
	}
	for i, zelle := range z.Zellen {
		// len() on the string: MaxCellBytes counts bytes, so one emoji costs
		// four. The row is reported with its number and the reader carries on,
		// so one hostile cell costs one row rather than the file.
		if len(zelle) > MaxCellBytes {
			z.Fehler = fmt.Sprintf("cell in column %s is %d bytes, the limit is %d",
				l.spaltenname(i), len(zelle), MaxCellBytes)
			break
		}
	}
	return z, true
}

// fuellen pads a short row out to the header's width, so the missing cells are
// empty in position rather than shifting every later cell one to the left. A
// row that is longer than the header keeps all of its cells: nothing is
// dropped, and Naechste reports the mismatch.
func (l *Leser) fuellen(rec []string) []string {
	if len(rec) >= len(l.kopf) {
		return rec
	}
	voll := make([]string, len(l.kopf))
	copy(voll, rec)
	return voll
}

// spaltenname names a column for a message: its header where there is one, its
// position otherwise.
func (l *Leser) spaltenname(i int) string {
	if i < len(l.kopf) && strings.TrimSpace(l.kopf[i]) != "" {
		return fmt.Sprintf("%q", l.kopf[i])
	}
	return fmt.Sprintf("%d", i+1)
}

// istParseFehler tells a row encoding/csv could not read from an error that
// ends the file.
func istParseFehler(err error) bool {
	var pe *stdcsv.ParseError
	return errors.As(err, &pe)
}
