package public

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// boundsCase is a handler with the upload limits a test names.
func boundsCase(t *testing.T, image, video int64) (*Handler, *domain.Website) {
	t.Helper()
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	h.SetUploadLimits(image, video)
	return h, ws
}

// takeBody runs takeAttachments over a body and closes the keeper when the test
// ends, as the middleware does when the request is answered.
func takeBody(t *testing.T, h *Handler, ws *domain.Website, body []byte, ctype string, prepare ...func(*http.Request)) (string, *attachmentKeeper, bool, *uploadRefusal) {
	t.Helper()
	req := httptest.NewRequest("POST", "/formular", bytes.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	for _, p := range prepare {
		p(req)
	}
	fields, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if keeper != nil {
		t.Cleanup(func() { keeper.Close() })
	}
	return fields, keeper, took, refused
}

// bigMP4 is a part that sniffs as video/mp4: an ftyp box with the brand mp42,
// then padding up to n bytes.
func bigMP4(n int) []byte {
	box := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	return append(box, make([]byte, n-len(box))...)
}

// A part over its kind's limit is refused after the limit and one byte, and
// its spool file is gone before the next part is read.
func TestAnOversizePartIsRefusedBeforeItIsHeld(t *testing.T) {
	h, ws := boundsCase(t, 1<<20, 16<<20)
	body, ctype := rawMultipart(t, false,
		fld("name", "Anna"),
		filePart("anhang", "gross.png", bigPNG(12<<20)),
		fld("nachricht", "Hallo"))

	before := allocated()
	fields, keeper, took, refused := takeBody(t, h, ws, body, ctype)
	grew := allocated() - before
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%v", took, refused)
	}
	listed := keeper.List()
	want := i18n.Tf(i18n.Lang(context.Background()), "The file is larger than allowed (%d MB)", 1)
	if len(listed) != 1 || listed[0].Refused != want {
		t.Fatalf("listed = %+v, want one refused with %q", listed, want)
	}
	if grew >= 4<<20 {
		t.Errorf("the call allocated %d bytes for a 12 MB part, want under 4 MB", grew)
	}
	if files := spoolFiles(t, h.dataDir); len(files) != 0 {
		t.Errorf("the refused part left %v", files)
	}
	// The part after the refused one is still read.
	for _, w := range []string{"name=Anna", "nachricht=Hallo"} {
		if !bytes.Contains([]byte(fields), []byte(w)) {
			t.Errorf("fields %q lack %q", fields, w)
		}
	}
}

// At most one part of a submission may use the video limit (owner decision D2):
// every part after the first video is held to the image limit.
func TestOnlyOneVideoSizedPartPerSubmission(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	sentence := i18n.Tf(i18n.Lang(context.Background()), "The file is larger than allowed (%d MB)", 5)

	body, ctype := rawMultipart(t, false,
		filePart("anhang", "a.mp4", bigMP4(8<<20)),
		filePart("anhang", "b.mp4", bigMP4(8<<20)))
	_, keeper, took, refused := takeBody(t, h, ws, body, ctype)
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%v", took, refused)
	}
	listed := keeper.List()
	if len(listed) != 2 || listed[0].Refused != "" || listed[0].MimeType != "video/mp4" {
		t.Fatalf("the first video: listed = %+v", listed)
	}
	if listed[1].Refused != sentence {
		t.Errorf("the second video: %q, want %q", listed[1].Refused, sentence)
	}

	body, ctype = rawMultipart(t, false,
		filePart("anhang", "a.mp4", bigMP4(8<<20)),
		filePart("anhang", "b.png", onePixel(t)))
	_, keeper, _, _ = takeBody(t, h, ws, body, ctype)
	for _, f := range keeper.List() {
		if f.Refused != "" {
			t.Errorf("one video and one picture: %+v refused", f)
		}
	}
	if len(keeper.List()) != 2 {
		t.Errorf("listed = %+v", keeper.List())
	}
}

func TestAPDFAndAVideoWithinTheLimitsAreTaken(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	pdf := append([]byte("%PDF-1.4\n"), make([]byte, 1024)...)
	body, ctype := rawMultipart(t, false,
		filePart("anhang", "a.pdf", pdf),
		filePart("anhang", "b.mp4", bigMP4(2<<20)))
	_, keeper, took, refused := takeBody(t, h, ws, body, ctype)
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%v", took, refused)
	}
	listed := keeper.List()
	if len(listed) != 2 || listed[0].MimeType != "application/pdf" || listed[1].MimeType != "video/mp4" {
		t.Fatalf("listed = %+v", listed)
	}
	kept, err := keeper.Keep(context.Background())
	if err != nil || len(kept) != 2 {
		t.Fatalf("kept = %+v, %v; want both", kept, err)
	}
}

func fieldsOf(n, size int) []formPart {
	var parts []formPart
	for i := 0; i < n; i++ {
		parts = append(parts, fld("f"+strconv.Itoa(i), string(bytes.Repeat([]byte("x"), size))))
	}
	return parts
}

func TestTooManyPartsAreRefusedWhole(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)

	body, ctype := rawMultipart(t, false, fieldsOf(64, 4)...)
	fields, keeper, took, refused := takeBody(t, h, ws, body, ctype)
	if !took || refused != nil || keeper == nil {
		t.Fatalf("64 parts: took=%v refused=%v", took, refused)
	}
	if n := bytes.Count([]byte(fields), []byte("&")) + 1; n != 64 {
		t.Errorf("%d fields taken, want 64", n)
	}

	body, ctype = rawMultipart(t, false, append(fieldsOf(64, 4),
		filePart("anhang", "bild.png", onePixel(t)))...)
	_, keeper, took, refused = takeBody(t, h, ws, body, ctype)
	if took || keeper != nil || refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("65 parts: took=%v keeper=%v refused=%+v, want 413", took, keeper != nil, refused)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v", names)
	}
}

// The field text of a submission is bounded across all fields together, by the
// body bound a plugin always had.
func TestTooMuchFieldTextIsRefusedWhole(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	for name, parts := range map[string][]formPart{
		"one 300 KB field": fieldsOf(1, 300<<10),
		"ten 30 KB fields": fieldsOf(10, 30<<10),
	} {
		body, ctype := rawMultipart(t, false, parts...)
		_, keeper, took, refused := takeBody(t, h, ws, body, ctype)
		if took || keeper != nil || refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
			t.Errorf("%s: took=%v refused=%+v, want 413", name, took, refused)
		}
	}
	// Exactly the bound is still a form.
	body, ctype := rawMultipart(t, false, fieldsOf(1, MaxPluginBodyBytes)...)
	if _, _, took, refused := takeBody(t, h, ws, body, ctype); !took || refused != nil {
		t.Errorf("a field of exactly %d bytes: took=%v refused=%+v", MaxPluginBodyBytes, took, refused)
	}
}

// A body that ends before its closing boundary is not a shorter form: a stored
// partial file would look like a finished upload.
func TestATruncatedSubmissionIsRefusedWhole(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)

	open, ctype := rawMultipart(t, true,
		fld("name", "Anna"), fld("nachricht", "Hallo"))
	whole, ctype2 := rawMultipart(t, false,
		fld("name", "Anna"), filePart("anhang", "gross.png", bigPNG(200<<10)))
	cut := whole[:len(whole)/2]

	for name, c := range map[string]struct {
		body  []byte
		ctype string
	}{
		"no closing boundary": {open, ctype},
		"cut inside a file":   {cut, ctype2},
	} {
		_, keeper, took, refused := takeBody(t, h, ws, c.body, c.ctype)
		if took || keeper != nil || refused == nil || refused.Status != http.StatusBadRequest {
			t.Errorf("%s: took=%v keeper=%v refused=%+v, want 400", name, took, keeper != nil, refused)
		}
		if names := spoolEntries(t, h.dataDir); len(names) != 0 {
			t.Errorf("%s: the spool holds %v", name, names)
		}
	}
}

// The outer bound is the largest single file, two images, the field text and
// the framing; one video-sized part is all a submission may carry.
func TestABodyOverTheOuterBoundIs413(t *testing.T) {
	h, ws := boundsCase(t, 1<<20, 4<<20)
	want := int64(4<<20 + 2*(1<<20) + MaxPluginBodyBytes + 64<<10)
	if got := attachmentBound(media.Limits{Image: 1 << 20, Video: 4 << 20}); got != want {
		t.Fatalf("attachmentBound = %d, want %d", got, want)
	}

	body, ctype := rawMultipart(t, false,
		fld("name", "Anna"), filePart("anhang", "gross.png", bigPNG(7<<20)), fld("nachricht", "Hallo"))
	if int64(len(body)) <= want {
		t.Fatalf("the test body is %d bytes, not over the bound %d", len(body), want)
	}
	// A chunked body has no declared length, so it is the reader that stops it.
	_, keeper, took, refused := takeBody(t, h, ws, body, ctype, func(r *http.Request) { r.ContentLength = -1 })
	if took || keeper != nil || refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("took=%v refused=%+v, want 413", took, refused)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v", names)
	}
}

// countingBody records how much of itself was read.
type countingBody struct {
	r io.Reader
	n int
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func (c *countingBody) Close() error { return nil }

func TestADeclaredLengthOverTheBoundIs413Unread(t *testing.T) {
	h, ws := boundsCase(t, 1<<20, 4<<20)
	bound := attachmentBound(media.Limits{Image: 1 << 20, Video: 4 << 20})
	body, ctype := rawMultipart(t, false, fld("name", "Anna"))

	counted := &countingBody{r: bytes.NewReader(body)}
	req := httptest.NewRequest("POST", "/formular", nil)
	req.Body = counted
	req.ContentLength = bound + 1
	req.Header.Set("Content-Type", ctype)
	_, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if took || keeper != nil || refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("took=%v refused=%+v, want 413", took, refused)
	}
	if counted.n != 0 {
		t.Errorf("%d bytes of the body were read before the refusal", counted.n)
	}

	// Exactly the bound is allowed, and a chunked body (-1) falls through to
	// the reader's own limit.
	counted = &countingBody{r: bytes.NewReader(body)}
	req = httptest.NewRequest("POST", "/formular", nil)
	req.Body = counted
	req.ContentLength = -1
	req.Header.Set("Content-Type", ctype)
	_, keeper, took, refused = h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if !took || refused != nil {
		t.Fatalf("a chunked body: took=%v refused=%+v", took, refused)
	}
	keeper.Close()
}

// A panic anywhere in the take still removes the spool.
func TestAPanicInsideTheTakeRemovesTheSpool(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	body, ctype := rawMultipart(t, false,
		filePart("anhang", "a.png", bigPNG(64<<10)),
		filePart("anhang", "b.png", bigPNG(64<<10)))

	spooled := -1
	attachPartHook = func() {
		spooled = len(spoolFiles(t, h.dataDir))
		panic("a part went wrong")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the hook did not panic: it was never called")
			}
		}()
		takeBody(t, h, ws, body, ctype)
	}()
	attachPartHook = nil

	if spooled != 1 {
		t.Errorf("%d files lay in the spool at the panic, want the first part's", spooled)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v after the panic", names)
	}
	// With the seam cleared the next submission is taken normally.
	if _, keeper, took, refused := takeBody(t, h, ws, body, ctype); !took || refused != nil || len(keeper.List()) != 2 {
		t.Errorf("the next submission: took=%v refused=%+v", took, refused)
	}
}

// More files than allowed are named in the language of the page, and never
// spooled.
func TestAFileOverTheAttachmentCountIsNamedInThePagesLanguage(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	var parts []formPart
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		parts = append(parts, filePart("anhang", n+".png", onePixel(t)))
	}
	body, ctype := rawMultipart(t, false, parts...)

	_, keeper, took, refused := takeBody(t, h, ws, body, ctype, func(r *http.Request) {
		*r = *r.WithContext(i18n.WithLang(r.Context(), "fr"))
	})
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%+v", took, refused)
	}
	want := i18n.Tf("fr", "at most %d files", MaxAttachments)
	if want == "at most 3 files" {
		t.Fatalf("the sentence has no French translation: %q", want)
	}
	listed := keeper.List()
	if len(listed) != 5 {
		t.Fatalf("listed = %+v", listed)
	}
	for _, f := range listed[3:] {
		if f.Refused != want {
			t.Errorf("%s: %q, want %q", f.Name, f.Refused, want)
		}
	}
	if files := spoolFiles(t, h.dataDir); len(files) != MaxAttachments {
		t.Errorf("%d files in the spool, want %d: the extras were spooled", len(files), MaxAttachments)
	}
}

// Keep after Close stores nothing; Close twice is harmless; the two may race.
func TestAKeepAfterCloseStoresNothing(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	body, ctype := rawMultipart(t, false, filePart("anhang", "bild.png", onePixel(t)))
	mediaRows := func() int {
		var n int
		if err := h.mediaStore.DB.Read.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	_, keeper, took, _ := takeBody(t, h, ws, body, ctype)
	if !took {
		t.Fatal("not taken")
	}
	if err := keeper.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := keeper.Close(); err != nil {
		t.Fatalf("a second Close: %v", err)
	}
	kept, err := keeper.Keep(context.Background())
	if err != nil || len(kept) != 0 || mediaRows() != 0 {
		t.Fatalf("Keep after Close: kept=%+v err=%v rows=%d", kept, err, mediaRows())
	}

	// Keep and Close from two goroutines: the answer is all or nothing, and
	// the race detector has nothing to report.
	_, keeper, _, _ = takeBody(t, h, ws, body, ctype)
	done := make(chan []int, 1)
	go func() {
		kept, _ := keeper.Keep(context.Background())
		done <- []int{len(kept)}
	}()
	keeper.Close()
	if n := (<-done)[0]; n != 0 && n != 1 {
		t.Errorf("a racing Keep kept %d files of one", n)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v", names)
	}
}
