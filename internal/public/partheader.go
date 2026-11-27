package public

import (
	"bytes"
	"errors"
	"io"
)

// maxPartHeaderBytes bounds the header section of one part of a public form:
// the bytes from the end of its delimiter line up to and including the blank
// line that ends its headers (T-38-09, WR-02 of the phase 38 review).
//
// mime/multipart has no setting for it. NextPart reads a part's whole header
// into memory before the caller sees the part, and allows ten megabytes of it:
// one 9 MiB header line cost 24 MB of heap and 58 MB of allocation, and four
// senders with seven such parts each grew the process by 851 to 904 MB, with no
// file involved and before any spam trap ran. An honest part header is a
// Content-Disposition with a file name and a Content-Type, a few hundred bytes;
// sixteen kilobytes leaves room for a long file name in any script and is still
// nothing a part can be made to cost.
const maxPartHeaderBytes = 16 << 10

// errPartHeaderTooLarge ends a submission whose part header is over the bound.
// whole answers it with 413, as any other body over a bound.
var errPartHeaderTooLarge = errors.New("public: a part's header is over its bound")

// partHeaderBound passes a multipart body through unchanged and counts the
// header section of every part as it goes by, so the bound holds before
// mime/multipart has buffered the header.
//
// It reads the body line by line the way the multipart reader does. A
// delimiter line is a line that starts with "--" and the boundary, followed by
// nothing but spaces, tabs and the line end; after it come the part's headers,
// up to the first empty line, and then its content. A line in the content that
// merely starts with the boundary and goes on with something else is content,
// as it is for the reader. Where the two could disagree this one counts more,
// never less: it takes a carriage return anywhere after the boundary for
// white space, so at worst a line no honest body has is counted as a header.
//
// The bytes are scanned where they pass and never kept, so the reader costs
// nothing however long a line is.
type partHeaderBound struct {
	r     io.Reader
	dash  []byte // "--" and the boundary
	limit int

	// line is where the scanner stands in a line outside a header section.
	line int
	// matched is how much of dash the current line has shown so far.
	matched int

	inHeader bool
	// header is the bytes of the current header section; lineLen and lastCR
	// are those of its current line, to know an empty one.
	header  int
	lineLen int
	lastCR  bool

	err error
}

// Where the scanner stands in a line outside a header section.
const (
	lineMatching = iota // the line so far is a prefix of dash
	lineAfter           // the line started with dash; white space may follow
	lineOther           // the line is content; skip to its end
)

func newPartHeaderBound(r io.Reader, boundary string, limit int) io.Reader {
	return &partHeaderBound{r: r, dash: []byte("--" + boundary), limit: limit}
}

func (b *partHeaderBound) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	n, err := b.r.Read(p)
	if over := b.scan(p[:n]); over >= 0 {
		// The bytes up to the bound are handed on, the error after them, and
		// every later read answers the error again.
		b.err = errPartHeaderTooLarge
		return over, b.err
	}
	return n, err
}

// scan follows p through the grammar and returns the index of the first byte
// over the bound, or -1.
func (b *partHeaderBound) scan(p []byte) int {
	for i := 0; i < len(p); {
		if b.inHeader {
			c := p[i]
			b.header++
			if b.header > b.limit {
				return i
			}
			i++
			if c != '\n' {
				b.lineLen++
				b.lastCR = c == '\r'
				continue
			}
			if b.lineLen == 0 || (b.lineLen == 1 && b.lastCR) {
				// The empty line: the headers are over, the content begins
				// on the next line.
				b.inHeader = false
				b.line, b.matched = lineMatching, 0
			}
			b.lineLen, b.lastCR = 0, false
			continue
		}

		switch b.line {
		case lineMatching:
			if p[i] == b.dash[b.matched] {
				i++
				if b.matched++; b.matched == len(b.dash) {
					b.line = lineAfter
				}
				continue
			}
			// Not a delimiter. The byte is looked at again as content, since
			// it may be the line's end.
			b.line = lineOther

		case lineAfter:
			c := p[i]
			i++
			switch c {
			case ' ', '\t', '\r':
			case '\n':
				// A delimiter line: a header section follows.
				b.inHeader = true
				b.header, b.lineLen, b.lastCR = 0, 0, false
			default:
				// "--boundary--" and anything else that is not white space.
				b.line = lineOther
			}

		case lineOther:
			j := bytes.IndexByte(p[i:], '\n')
			if j < 0 {
				return -1
			}
			i += j + 1
			b.line, b.matched = lineMatching, 0
		}
	}
	return -1
}
