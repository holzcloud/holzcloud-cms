package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// realPNG is the smallest PNG that really decodes.
func realPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// dirEntries is what lies directly in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The file waits on the disk with exactly its bytes, readable by nobody else,
// and Discard takes it away again.
func TestCheckStreamSpoolsTheFileAndDiscardRemovesIt(t *testing.T) {
	data := realPNG(t)
	dir := t.TempDir()

	in, err := CheckStream(bytes.NewReader(data), "bild.png", Limits{Image: 1 << 20, Video: 8 << 20}, dir)
	if err != nil {
		t.Fatalf("CheckStream: %v", err)
	}
	if in.MimeType != "image/png" || in.Size != int64(len(data)) || in.OriginalName != "bild.png" {
		t.Errorf("intake = %+v", in)
	}
	names := dirEntries(t, dir)
	if len(names) != 1 {
		t.Fatalf("%d entries in the spool, want 1: %v", len(names), names)
	}
	path := filepath.Join(dir, names[0])
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Errorf("the spool file does not hold the bytes: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	in.Discard()
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("Discard left %v", names)
	}
	in.Discard() // a second one is harmless
}

// A spooled intake is kept from its file; the spool file is the caller's to
// remove.
func TestKeepForReadsASpooledIntake(t *testing.T) {
	data := realPNG(t)
	spool := t.TempDir()
	dataDir := t.TempDir()
	store, websiteID := newTestStore(t)

	in, err := CheckStream(bytes.NewReader(data), "bild.png", Limits{Image: 1 << 20, Video: 8 << 20}, spool)
	if err != nil {
		t.Fatalf("CheckStream: %v", err)
	}
	m, existed, err := in.KeepFor(context.Background(), store, websiteID, "kontaktformular", dataDir)
	if err != nil || existed {
		t.Fatalf("KeepFor = %v, existed %v", err, existed)
	}
	stored, err := os.ReadFile(Path(dataDir, websiteID, m.Filename))
	if err != nil || !bytes.Equal(stored, data) {
		t.Errorf("the stored file differs from the spooled one: %v", err)
	}
	if names := dirEntries(t, spool); len(names) != 1 {
		t.Errorf("KeepFor changed the spool: %v", names)
	}
	in.Discard()
	if names := dirEntries(t, spool); len(names) != 0 {
		t.Errorf("Discard left %v", names)
	}
}

// endlessPNG yields a PNG signature and then zeros for ever, counting what it
// gave out.
type endlessPNG struct {
	sent int64
}

func (e *endlessPNG) Read(p []byte) (int, error) {
	n := 0
	sig := []byte("\x89PNG\r\n\x1a\n")
	for n < len(p) {
		if e.sent < int64(len(sig)) {
			p[n] = sig[e.sent]
		} else {
			p[n] = 0
		}
		n++
		e.sent++
	}
	return n, nil
}

// The part is not read past its limit and one byte, whatever the sender keeps
// sending, and nothing is left on the disk.
func TestCheckStreamReadsNoFurtherThanTheLimit(t *testing.T) {
	dir := t.TempDir()
	src := &endlessPNG{}
	_, err := CheckStream(src, "unendlich.png", Limits{Image: 1 << 20, Video: 8 << 20}, dir)
	var ref Refusal
	if !errors.As(err, &ref) || ref.Code != RefusedSize || ref.Arg != int64(1) {
		t.Fatalf("err = %v, want a size refusal of 1 MB", err)
	}
	if src.sent > 1<<20+1 {
		t.Errorf("%d bytes were read from the part, want at most %d", src.sent, 1<<20+1)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("the refused file left %v", names)
	}
}

// failingPart gives out its bytes and then an error of the test's choosing.
type failingPart struct {
	data []byte
	err  error
}

func (f *failingPart) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

// A read error of the part is not a refusal and stays recognisable; a spool
// that cannot be written is told apart from both.
func TestCheckStreamTellsAReadErrorFromARefusal(t *testing.T) {
	png600 := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 592)...)
	limits := Limits{Image: 1 << 20, Video: 8 << 20}
	var tooBig *http.MaxBytesError

	for name, c := range map[string]struct {
		part  io.Reader
		check func(error) bool
	}{
		"a cut body": {
			&failingPart{data: png600, err: io.ErrUnexpectedEOF},
			func(err error) bool { return errors.Is(err, io.ErrUnexpectedEOF) },
		},
		"a cut inside the head": {
			&failingPart{data: png600[:100], err: io.ErrUnexpectedEOF},
			func(err error) bool { return errors.Is(err, io.ErrUnexpectedEOF) },
		},
		"a body over the bound": {
			&failingPart{data: png600, err: &http.MaxBytesError{Limit: 10}},
			func(err error) bool { return errors.As(err, &tooBig) },
		},
	} {
		dir := t.TempDir()
		_, err := CheckStream(c.part, "bild.png", limits, dir)
		var ref Refusal
		if err == nil || errors.As(err, &ref) || errors.Is(err, ErrSpool) || !c.check(err) {
			t.Errorf("%s: err = %v", name, err)
		}
		if names := dirEntries(t, dir); len(names) != 0 {
			t.Errorf("%s: left %v", name, names)
		}
	}

	_, err := CheckStream(bytes.NewReader(png600), "bild.png", limits, filepath.Join(t.TempDir(), "fehlt"))
	if !errors.Is(err, ErrSpool) {
		t.Errorf("a spool directory that does not exist: err = %v, want ErrSpool", err)
	}
}
