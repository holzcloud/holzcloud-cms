package media

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stagedPair(t *testing.T) []*Intake {
	t.Helper()
	return []*Intake{
		intakeOf("image/jpeg", "eins.jpg", jpegBytes(t, 300, 200)),
		intakeOf("image/png", "zwei.png", pngBytes(t, gradient(40, 30))),
	}
}

func TestStagingRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := NewStaging(root)
	in := stagedPair(t)

	token, err := s.Put(7, 3, in)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(token) != 32 {
		t.Fatalf("token %q is not 32 characters", token)
	}

	got, err := s.Get(7, token, 3)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	files := got.Files()
	if len(files) != 2 || files[0].Name != "eins.jpg" || files[0].Width != 300 || files[0].Height != 200 ||
		files[1].Name != "zwei.png" || files[1].Width != 40 {
		t.Errorf("manifest = %+v", files)
	}

	taken, err := s.Take(7, token, 3)
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	for i := range in {
		b, err := taken.Content(i)
		if err != nil || !bytes.Equal(b, in[i].content) {
			t.Errorf("file %d did not round-trip: %v", i, err)
		}
	}
	// Exactly one caller gets it.
	if _, err := s.Take(7, token, 3); !errors.Is(err, ErrStagingExpired) {
		t.Errorf("second Take = %v, want expired", err)
	}
	if _, err := s.Get(7, token, 3); !errors.Is(err, ErrStagingExpired) {
		t.Errorf("Get after Take = %v, want expired", err)
	}
	taken.Discard()
	if entries, _ := os.ReadDir(filepath.Join(root, "media-staging", "7")); len(entries) != 0 {
		t.Errorf("leftovers after Discard: %v", entries)
	}
}

func TestStagingIsScopedToWebsiteAndUser(t *testing.T) {
	s := NewStaging(t.TempDir())
	token, err := s.Put(7, 3, stagedPair(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(8, token, 3); !errors.Is(err, ErrStagingExpired) {
		t.Errorf("another website: %v, want expired", err)
	}
	if _, err := s.Get(7, token, 4); !errors.Is(err, ErrStagingForeign) {
		t.Errorf("another user: %v, want foreign", err)
	}
	if _, err := s.Take(7, token, 4); !errors.Is(err, ErrStagingForeign) {
		t.Errorf("another user Take: %v, want foreign", err)
	}
	// The foreign Take must not have claimed it.
	if _, err := s.Get(7, token, 3); err != nil {
		t.Errorf("the owner lost the staging to a stranger: %v", err)
	}
}

func TestStagingRefusesMalformedTokensWithoutTouchingTheDisk(t *testing.T) {
	dir := t.TempDir()
	s := NewStaging(dir)
	// A file the traversal would reach if the token were not checked first.
	secret := filepath.Join(dir, "secret")
	if err := os.MkdirAll(secret, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secret, "manifest.json"), []byte(`{"website_id":7,"user_id":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := strings.Repeat("a", 32)
	for _, token := range []string{
		"", "../secret", "../../secret", strings.ToUpper(valid), valid[:31], valid + "a",
		valid[:30] + "/.", strings.Repeat("g", 32), valid + ".taken",
	} {
		if _, err := s.Get(7, token, 3); !errors.Is(err, ErrStagingExpired) {
			t.Errorf("Get(%q) = %v, want expired", token, err)
		}
		if _, err := s.Take(7, token, 3); !errors.Is(err, ErrStagingExpired) {
			t.Errorf("Take(%q) = %v, want expired", token, err)
		}
	}
	if _, err := os.Stat(filepath.Join(secret, "manifest.json")); err != nil {
		t.Errorf("a malformed token touched something outside the staging root: %v", err)
	}
}

func TestStagingPrune(t *testing.T) {
	root := t.TempDir()
	s := NewStaging(root)
	old, err := s.Put(7, 3, stagedPair(t))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := s.Put(7, 3, stagedPair(t))
	if err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(root, "media-staging", "7", old)
	past := time.Now().Add(-2 * StagingRetention)
	if err := os.Chtimes(oldDir, past, past); err != nil {
		t.Fatal(err)
	}
	// A leftover of a crashed request.
	leftover := filepath.Join(root, "media-staging", "7", strings.Repeat("b", 32)+".tmp")
	if err := os.Mkdir(leftover, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(leftover, past, past); err != nil {
		t.Fatal(err)
	}

	n, err := s.Prune(StagingRetention)
	if err != nil || n != 2 {
		t.Fatalf("Prune = %d, %v; want 2", n, err)
	}
	if _, err := s.Get(7, old, 3); !errors.Is(err, ErrStagingExpired) {
		t.Errorf("the old staging survived: %v", err)
	}
	if _, err := s.Get(7, fresh, 3); err != nil {
		t.Errorf("the fresh staging was pruned: %v", err)
	}

	// No root at all is not an error.
	if n, err := NewStaging(t.TempDir()).Prune(time.Hour); err != nil || n != 0 {
		t.Errorf("Prune on a missing root = %d, %v", n, err)
	}
}

func TestStagingIsNotUnderTheMediaDirectory(t *testing.T) {
	dir := t.TempDir()
	s := NewStaging(dir)
	if _, err := s.Put(7, 3, stagedPair(t)); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(s.root, filepath.Join(dir, "media")+string(filepath.Separator)) {
		t.Errorf("the staging root %s sits inside media/", s.root)
	}
}
