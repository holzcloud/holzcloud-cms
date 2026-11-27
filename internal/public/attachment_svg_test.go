package public

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// A visitor's SVG is refused by its first bytes and never read further
// (T-38-01 of the phase 38 security audit).
//
// An SVG was the one kind the public path read back whole: the check for
// external references builds a full DOM of the file, about thirty times its
// size, and four spam messages with three five-megabyte SVGs each grew the
// process by more than a gigabyte. A visitor's message has no use for vector
// graphics, so the kind is refused on this path with the sentence every other
// refused kind gets, in the page's language. The editor's upload still takes
// an SVG.
func TestAVisitorsSVGIsRefusedUnread(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g/>", (5<<20-100)/4) + `</svg>`)
	body, ctype := rawMultipart(t, false,
		fld("name", "Anna"),
		filePart("anhang", "plan.svg", svg),
		fld("nachricht", "Hallo"))
	french := func(r *http.Request) { *r = *r.WithContext(i18n.WithLang(r.Context(), "fr")) }

	before := allocated()
	fields, keeper, took, refused := takeBody(t, h, ws, body, ctype, french)
	grew := allocated() - before
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%+v: an SVG refuses the file, not the message", took, refused)
	}
	want := i18n.Tf("fr", "File type not allowed: %s", "image/svg+xml")
	if strings.HasPrefix(want, "File type") {
		t.Fatalf("the sentence has no French translation: %q", want)
	}
	listed := keeper.List()
	if len(listed) != 1 || listed[0].Refused != want || listed[0].MimeType != "" {
		t.Fatalf("listed = %+v, want the SVG refused with %q", listed, want)
	}
	if grew >= 4<<20 {
		t.Errorf("the take allocated %d bytes for a 5 MB SVG, want under 4 MB: it was read back", grew)
	}
	if files := spoolFiles(t, h.dataDir); len(files) != 0 {
		t.Errorf("the refused SVG left %v in the spool", files)
	}
	for _, w := range []string{"name=Anna", "nachricht=Hallo"} {
		if !bytes.Contains([]byte(fields), []byte(w)) {
			t.Errorf("fields %q lack %q", fields, w)
		}
	}
}
