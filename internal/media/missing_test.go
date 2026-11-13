package media

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// putFile writes a file at the place Path names for it below root.
func putFile(t *testing.T, root string, websiteID int64, filename string) {
	t.Helper()
	p := Path(root, websiteID, filename)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestMissingFilesFindsNothingWhenEveryFileIsThere(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, ws, "a.jpg", "Urlaub.jpg", "image/jpeg", 1, "hash-a"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	root := t.TempDir()
	putFile(t, root, ws, "a.jpg")

	checked, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if checked != 1 || len(missing) != 0 {
		t.Fatalf("checked %d, missing %+v; want 1 and none", checked, missing)
	}
}

func TestMissingFilesNamesAnOriginalTheDirectoryLacks(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, ws, "a.jpg", "Urlaub.jpg", "image/jpeg", 1, "hash-a"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	gone, err := s.Create(ctx, ws, "b.jpg", "Hochzeit.jpg", "image/jpeg", 1, "hash-b")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	root := t.TempDir()
	putFile(t, root, ws, "a.jpg")

	checked, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if checked != 2 || len(missing) != 1 {
		t.Fatalf("checked %d, missing %+v; want 2 and one", checked, missing)
	}
	got := missing[0]
	want := MissingFile{
		Rel:          "media/" + strconv.FormatInt(ws, 10) + "/b.jpg",
		Table:        "media",
		RowID:        gone.ID,
		WebsiteID:    ws,
		OriginalName: "Hochzeit.jpg",
		Filename:     "b.jpg",
	}
	if got != want {
		t.Errorf("missing file:\n got %+v\nwant %+v", got, want)
	}
}
