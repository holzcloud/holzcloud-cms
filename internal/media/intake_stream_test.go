package media

import (
	"bytes"
	"context"
	"image"
	"image/png"
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
