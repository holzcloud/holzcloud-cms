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
	"runtime"
	"testing"
	"time"
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

// The sweep takes what is old enough and only that, and never touches the
// directories beside the spool.
func TestSweepSpoolEmptiesTheSpoolAndNothingElse(t *testing.T) {
	dataDir := t.TempDir()
	root := SpoolRoot(dataDir)
	for _, d := range []string{"req-old", "req-new"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, d, "part-1"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "stray"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	longAgo := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "req-old"), longAgo, longAgo); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "media", "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "media", "1", "a.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if n, err := SweepSpool(dataDir, time.Hour); err != nil || n != 1 {
		t.Fatalf("SweepSpool(1h) = %d, %v; want 1 (only the old directory)", n, err)
	}
	if names := dirEntries(t, root); len(names) != 2 {
		t.Errorf("after the hourly sweep: %v, want req-new and stray", names)
	}
	if n, err := SweepSpool(dataDir, 0); err != nil || n != 2 {
		t.Fatalf("SweepSpool(0) = %d, %v; want 2", n, err)
	}
	if names := dirEntries(t, root); len(names) != 0 {
		t.Errorf("after the start sweep: %v", names)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "media", "1", "a.png")); err != nil {
		t.Errorf("the sweep touched media/: %v", err)
	}

	if n, err := SweepSpool(t.TempDir(), 0); err != nil || n != 0 {
		t.Errorf("a missing spool root: %d, %v; want 0 and no error", n, err)
	}
}

// CheckStream refuses an SVG by its kind, before a byte of it is spooled: the
// external-reference check would have to read it back whole and build a DOM of
// it, and a stranger's message has no use for vector graphics (T-38-01). Check,
// the signed-in operator's path, still takes one.
func TestCheckStreamRefusesAnSVG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`)
	dir := t.TempDir()
	_, err := CheckStream(bytes.NewReader(svg), "plan.svg", Limits{Image: 1 << 20, Video: 8 << 20}, dir)
	var ref Refusal
	if !errors.As(err, &ref) || ref.Code != RefusedKind || ref.Arg != "image/svg+xml" {
		t.Fatalf("err = %v, want a kind refusal naming image/svg+xml", err)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Errorf("the refused SVG left %v", names)
	}
}

// Keeping a spooled file that has to be read whole to strip its metadata reads
// it into a buffer of its own size, once (WR-04 of the phase 38 review).
//
// The size is known exactly, since the spool file is the bytes CheckStream
// counted. io.ReadAll does not know it and grows its buffer by doubling, which
// allocated about twice the file and was the documented worst case of a keep:
// about 128 MB for one 64 MB video, 512 MB for four at once.
func TestKeepingASpooledVideoReadsItOnce(t *testing.T) {
	const size = 16 << 20
	video := append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), make([]byte, size-24)...)
	spool := t.TempDir()
	dataDir := t.TempDir()
	store, websiteID := newTestStore(t)

	in, err := CheckStream(bytes.NewReader(video), "film.mp4", Limits{Image: 1 << 20, Video: 32 << 20}, spool)
	if err != nil || in.MimeType != "video/mp4" {
		t.Fatalf("CheckStream: %v (%+v)", err, in)
	}
	video = nil

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	m, _, err := in.KeepFor(context.Background(), store, websiteID, "kontaktformular", dataDir)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("KeepFor: %v", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > size+size/4 {
		t.Errorf("keeping a %d-byte video allocated %d bytes, want about its size once", size, grew)
	}
	if info, err := os.Stat(Path(dataDir, websiteID, m.Filename)); err != nil || info.Size() != size {
		t.Errorf("the stored video: %v, %v", info, err)
	}
}
