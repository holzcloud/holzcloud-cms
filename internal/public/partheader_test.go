package public

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
)

// headerBody is a multipart body whose one field part carries a header line of
// pad bytes.
func headerBody(boundary string, pad int) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "--%s\r\nContent-Disposition: form-data; name=\"name\"\r\nX-Pad: %s\r\n\r\nAnna\r\n--%s--\r\n",
		boundary, strings.Repeat("a", pad), boundary)
	return b.Bytes()
}

// A part whose header section is over its bound refuses the submission whole,
// 413, after the bound and not after the header (T-38-09, WR-02 of the phase 38
// review).
//
// mime/multipart reads a part's whole header into memory before the caller sees
// the part, up to ten megabytes of it; one 9 MiB header line cost 24 MB of heap
// and 58 MB of allocation, with no file and no spam trap involved, and four
// senders with seven such parts each grew the real binary by 851 to 904 MB.
// Honest part headers are a few hundred bytes.
func TestAPartHeaderOverItsBoundIsRefusedWhole(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	const boundary = "PARTHEADERBOUND38"
	body := headerBody(boundary, 9900<<10)

	before := allocated()
	req := httptest.NewRequest("POST", "/formular", bytes.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	_, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	grew := allocated() - before
	if keeper != nil {
		keeper.Close()
	}
	if took || refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("a 9.9 MB part header: took=%v refused=%+v, want 413", took, refused)
	}
	if grew >= 1<<20 {
		t.Errorf("the take allocated %d bytes for a 9.9 MB part header, want under 1 MB", grew)
	}
	if n, keys := held(h.uploads); n != 0 || keys != 0 {
		t.Errorf("%d inside and %d keys after the refusal, want none", n, keys)
	}

	// A header just under the bound is an ordinary, if odd, part.
	body = headerBody(boundary, maxPartHeaderBytes-200)
	req = httptest.NewRequest("POST", "/formular", bytes.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	fields, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if keeper != nil {
		keeper.Close()
	}
	if !took || refused != nil || fields != "name=Anna" {
		t.Errorf("a header under the bound: took=%v refused=%+v fields=%q", took, refused, fields)
	}
}

// The bound reads the body as mime/multipart does, whatever the chunks it
// arrives in: an honest body passes unchanged a byte at a time, and a header
// over the bound is caught a byte at a time too. Lines that only look like a
// delimiter inside a part's content are content.
func TestPartHeaderBoundFollowsTheMultipartGrammar(t *testing.T) {
	const boundary = "XyZ38"
	var b bytes.Buffer
	b.WriteString("preamble line\r\n--XyZ38 \t\r\n")
	b.WriteString("Content-Disposition: form-data; name=\"a\"\r\n\r\n")
	// Content with a line that starts like the delimiter and is not one, and
	// a long line after it: no header is counted here.
	b.WriteString("eins\r\n--XyZ38x" + strings.Repeat("b", 40<<10) + "\r\nzwei")
	b.WriteString("\r\n--XyZ38\r\n")
	b.WriteString("Content-Disposition: form-data; name=\"f\"; filename=\"f.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n")
	b.WriteString(strings.Repeat("\n", 30<<10))
	b.WriteString("\r\n--XyZ38--\r\nepilogue")
	honest := b.Bytes()

	for name, r := range map[string]io.Reader{
		"whole":       bytes.NewReader(honest),
		"byte a time": iotest.OneByteReader(bytes.NewReader(honest)),
		"half chunks": iotest.HalfReader(bytes.NewReader(honest)),
	} {
		mr := multipart.NewReader(newPartHeaderBound(r, boundary, maxPartHeaderBytes), boundary)
		var got []string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: NextPart: %v", name, err)
			}
			content, err := io.ReadAll(p)
			if err != nil {
				t.Fatalf("%s: read part %q: %v", name, p.FormName(), err)
			}
			got = append(got, fmt.Sprintf("%s:%d", p.FormName(), len(content)))
		}
		if want := []string{"a:" + fmt.Sprint(len("eins\r\n--XyZ38x")+40<<10+len("\r\nzwei")), "f:" + fmt.Sprint(30<<10)}; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: parts %v, want %v", name, got, want)
		}
	}

	over := headerBody(boundary, maxPartHeaderBytes)
	for name, r := range map[string]io.Reader{
		"whole":       bytes.NewReader(over),
		"byte a time": iotest.OneByteReader(bytes.NewReader(over)),
	} {
		mr := multipart.NewReader(newPartHeaderBound(r, boundary, maxPartHeaderBytes), boundary)
		if _, err := mr.NextPart(); !errors.Is(err, errPartHeaderTooLarge) {
			t.Errorf("%s: NextPart = %v, want errPartHeaderTooLarge", name, err)
		}
	}
}
