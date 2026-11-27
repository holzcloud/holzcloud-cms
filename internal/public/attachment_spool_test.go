package public

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// pngSignature is what the sniffing sees of a PNG: the eight bytes are enough
// for the kind, and nothing after them is ever decoded on this path.
var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// bigPNG is a part that sniffs as a PNG and weighs n bytes.
func bigPNG(n int) []byte {
	return append(append([]byte{}, pngSignature...), make([]byte, n-len(pngSignature))...)
}

// allocated is the bytes the program has allocated in total so far. TotalAlloc
// only ever grows, so a difference across a call is what the call allocated,
// whatever the collector did in between.
func allocated() uint64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return m.TotalAlloc
}

// spoolEntries lists what lies directly below the spool root.
func spoolEntries(t *testing.T, dataDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, ".upload-spool"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// spoolFiles lists every file below the spool root, however deep.
func spoolFiles(t *testing.T, dataDir string) map[string]int64 {
	t.Helper()
	files := map[string]int64{}
	root := filepath.Join(dataDir, ".upload-spool")
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files[path] = info.Size()
		}
		return nil
	})
	return files
}

// A file that is being taken in waits on the disk, not in memory.
//
// This is finding H4: every part was copied into a bytes.Buffer, and then again
// into the slice Check keeps, before a single check or spam trap had run. Four
// megabytes under a five-megabyte limit is the honest case; the allocation the
// call makes must not scale with it.
func TestAnAttachmentWaitsOnDiskNotInMemory(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	dir := t.TempDir()
	h.dataDir = dir
	h.SetUploadLimits(5<<20, 50<<20)

	const size = 4 << 20
	body, ctype := multipartBody(t, map[string]string{"name": "Anna"},
		map[string][]byte{"gross.png": bigPNG(size)})
	req := httptest.NewRequest("POST", "/formular", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()

	before := allocated()
	_, keeper, took, refused := h.takeAttachments(rec, req, ws.ID, "kontaktformular", true)
	grew := allocated() - before
	if !took || refused != nil {
		t.Fatalf("the submission was not taken: took=%v refused=%v", took, refused)
	}
	defer keeper.Close()

	listed := keeper.List()
	if len(listed) != 1 || listed[0].Refused != "" || listed[0].Size != size {
		t.Fatalf("listed = %+v, want one accepted file of %d bytes", listed, size)
	}
	files := spoolFiles(t, dir)
	if len(files) != 1 {
		t.Errorf("%d files below the spool, want exactly one: %v", len(files), files)
	}
	for path, n := range files {
		if n != size {
			t.Errorf("%s holds %d bytes, want %d", path, n, size)
		}
	}
	if grew >= 2<<20 {
		t.Errorf("taking the part allocated %d bytes, want under 2 MB: the file was held in memory", grew)
	}
}

// formPart is one part of a hand-made multipart body: a field when filename is
// empty, a file otherwise.
type formPart struct {
	name, filename string
	content        []byte
}

func fld(name, value string) formPart { return formPart{name: name, content: []byte(value)} }

func filePart(name, filename string, content []byte) formPart {
	return formPart{name: name, filename: filename, content: content}
}

// rawMultipart writes the parts in the order given, which multipartBody (a map)
// cannot, and returns the body without its closing boundary when open is true,
// the shape of a body that was cut short.
func rawMultipart(t *testing.T, open bool, parts ...formPart) (body []byte, ctype string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		disposition := `form-data; name="` + p.name + `"`
		if p.filename != "" {
			disposition += `; filename="` + p.filename + `"`
			h.Set("Content-Type", "application/octet-stream")
		}
		h.Set("Content-Disposition", disposition)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(p.content); err != nil {
			t.Fatal(err)
		}
	}
	if !open {
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return buf.Bytes(), w.FormDataContentType()
}

// postMultipart sends a multipart body through the plugin middleware as the
// server does, to the contact form's address.
func postMultipart(t *testing.T, h *Handler, ws *domain.Website, path string, body []byte, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://velowerkstatt.test"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	req = req.WithContext(domain.WebsiteToContext(req.Context(), ws))
	rec := httptest.NewRecorder()
	h.PluginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the submission went past the plugin to the core")
	})).ServeHTTP(rec, req)
	return rec
}

// An attachment on the real contact form travels through the spool, is kept
// from there and the spool is empty after the answer.
func TestAContactFormAttachmentArrivesThroughTheSpool(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetUploadLimits(5<<20, 50<<20)
	pic := onePixel(t)

	body, ctype := rawMultipart(t, false,
		fld("seite", "kontakt"), fld("pfad", "/kontakt"),
		fld("gestellt", timeToken(t, database, -10*time.Second)),
		fld("name", "Eva Muster"), fld("email", "eva@example.test"),
		fld("nachricht", "Mit Bild."),
		filePart("anhang", "bild.png", pic))
	rec := postMultipart(t, h, ws, "/formular", body, ctype)

	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("answer %d %q, want 303 formular=gesendet", rec.Code, ort)
	}
	if n := messages(t, database); n != 1 {
		t.Fatalf("%d messages stored, want 1", n)
	}
	var filename string
	var count int
	if err := database.Read.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(filename), '') FROM media WHERE owner_plugin = 'kontaktformular'`).
		Scan(&count, &filename); err != nil || count != 1 {
		t.Fatalf("media rows of the plugin: %d, %v; want 1", count, err)
	}
	stored := filepath.Join(h.dataDir, "media", strconv.FormatInt(ws.ID, 10), filename)
	if got, err := os.ReadFile(stored); err != nil || len(got) == 0 {
		t.Errorf("the kept file is not under media/: %v", err)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v after the answer", names)
	}
}
