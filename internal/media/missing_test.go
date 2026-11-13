package media

import (
	"context"
	"fmt"
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

func TestMissingFilesNamesAVariantWithItsLabel(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	m, err := s.Create(ctx, ws, "a.jpg", "Urlaub.jpg", "image/jpeg", 1, "hash-a")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	mustExec(t, s, `INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
		VALUES ($1, 'thumb', 'a-thumb.jpg', 1, 1, 1)`, m.ID)
	medium := mustExec(t, s, `INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
		VALUES ($1, 'medium', 'a-medium.jpg', 1, 1, 1)`, m.ID)
	root := t.TempDir()
	putFile(t, root, ws, "a.jpg")
	putFile(t, root, ws, "a-thumb.jpg")

	checked, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if checked != 3 || len(missing) != 1 {
		t.Fatalf("checked %d, missing %+v; want 3 and one", checked, missing)
	}
	want := MissingFile{
		Rel:       "media/" + strconv.FormatInt(ws, 10) + "/a-medium.jpg",
		Table:     "media_variants",
		RowID:     medium,
		WebsiteID: ws,
		MediaID:   m.ID,
		Label:     "medium",
		Filename:  "a-medium.jpg",
	}
	if missing[0] != want {
		t.Errorf("missing file:\n got %+v\nwant %+v", missing[0], want)
	}
}

// A backup can be damaged or crafted, and restore.sh runs the check as root:
// a stored name that is not a plain file name must never become a path. Each
// such name gets a file planted exactly where joining it would lead, so a
// check that followed it would find something there and call the row present.
func TestMissingFilesNeverFollowsANameThatIsNotAPlainFileName(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	root := t.TempDir()
	plant := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("plant %s: %v", p, err)
		}
	}
	// "../escape.jpg" and the variant's "../v.jpg" lead to media/; "", "."
	// and ".." lead to directories that exist once anything is planted; a NUL
	// cannot be part of a path at all.
	plant(filepath.Join(root, "media", "escape.jpg"))
	plant(filepath.Join(root, "media", "v.jpg"))
	names := []string{"../escape.jpg", "a/b.jpg", "", "..", ".", "n\x00ul.jpg"}
	for _, name := range names {
		switch name {
		case "", ".", "..", "../escape.jpg", "n\x00ul.jpg":
		default:
			plant(Path(root, ws, name))
		}
	}
	var originals []int64
	for i, name := range names {
		originals = append(originals, mustExec(t, s,
			`INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash)
			 VALUES ($1, $2, 'x', 'image/jpeg', 1, $3)`, ws, name, fmt.Sprintf("h%d", i)))
	}
	variant := mustExec(t, s, `INSERT INTO media_variants (media_id, label, filename, width, height, size_bytes)
		VALUES ($1, 'thumb', '../v.jpg', 1, 1, 1)`, originals[0])

	checked, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if checked != len(names)+1 {
		t.Errorf("checked %d; want %d", checked, len(names)+1)
	}
	reported := map[string]MissingFile{}
	for _, m := range missing {
		reported[fmt.Sprintf("%s %d", m.Table, m.RowID)] = m
	}
	for i, name := range names {
		m, ok := reported[fmt.Sprintf("media %d", originals[i])]
		if !ok {
			t.Errorf("file name %q was followed and found present", name)
			continue
		}
		if !m.Unsafe || m.Rel != "" || m.Filename != name {
			t.Errorf("file name %q: %+v; want Unsafe, no Rel, the stored name", name, m)
		}
	}
	m, ok := reported[fmt.Sprintf("media_variants %d", variant)]
	if !ok || !m.Unsafe || m.Rel != "" {
		t.Errorf("variant ../v.jpg: %+v (reported %v); want Unsafe, no Rel", m, ok)
	}
}

// oddNames are file names that name one file inside the website directory
// but must not be printed raw: a C1 control (NEL), a bidi override that
// reverses what the operator's terminal shows, a newline, DEL, a tab, and the
// invisible characters a client-supplied extension can carry after its dot —
// NBSP, a zero-width space, a zero-width joiner, bytes that are not UTF-8
// (code review 32, IN-04 and the second pass, IN-01). A backslash is a
// separator only on Windows; here it is part of the name.
func oddNames() []string {
	names := []string{"e\nf.jpg", "g\x7fh.jpg", "i\tj.jpg", "k\u0085l.jpg", "m‮gpj.exe",
		"a.jpg ", "b.j​pg", "c.jp‍g", "s.\xffjpg"}
	if filepath.Separator == '/' {
		names = append(names, `c\d.jpg`)
	}
	return names
}

// A file whose name holds odd characters and exists is not missing: it is
// looked up like any other, and only the way it is printed differs. Before,
// such a name was reported MISSING unseen, so restore.sh ended with 3 for a
// backup that had every file (code review 32, second pass, IN-01).
func TestMissingFilesLooksUpAnOddNameAndFindsItPresent(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	root := t.TempDir()
	planted := 0
	for i, name := range oddNames() {
		p := Path(root, ws, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// APFS refuses a name that is not UTF-8; such a row is not inserted.
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Logf("cannot plant %q here: %v", name, err)
			continue
		}
		planted++
		mustExec(t, s, `INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash)
			VALUES ($1, $2, 'x', 'image/jpeg', 1, $3)`, ws, name, fmt.Sprintf("odd%d", i))
	}
	checked, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if checked != planted {
		t.Errorf("checked %d; want %d", checked, planted)
	}
	for _, m := range missing {
		t.Errorf("file name %q exists and was reported missing: %+v", m.Filename, m)
	}
}

// The same names, absent: each is reported, as looked up and not found, and
// never with the name raw in Rel.
func TestMissingFilesReportsAnAbsentOddNameQuoted(t *testing.T) {
	s, ws := newTestStore(t)
	ctx := context.Background()
	root := t.TempDir()
	names := oddNames()
	for i, name := range names {
		mustExec(t, s, `INSERT INTO media (website_id, filename, original_name, mime_type, size_bytes, content_hash)
			VALUES ($1, $2, 'x', 'image/jpeg', 1, $3)`, ws, name, fmt.Sprintf("odd%d", i))
	}
	_, missing, err := s.MissingFiles(ctx, root)
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	if len(missing) != len(names) {
		t.Fatalf("%d missing; want %d: %+v", len(missing), len(names), missing)
	}
	for _, m := range missing {
		if m.Unsafe {
			t.Errorf("file name %q was not looked up: %+v", m.Filename, m)
		}
		if m.Rel != "" || !m.Quoted {
			t.Errorf("file name %q: %+v; want Quoted, no Rel", m.Filename, m)
		}
	}
}
