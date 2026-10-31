package media

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Keep is the one way an uploaded byte reaches the served directory.
//
// Keep strips the metadata first. A second path that calls StoreFile on the
// bytes it was sent would publish the sender's camera and position, and every
// test of the strip would stay green. So this reads the source: outside of test
// files, PrepareUpload and StoreFile are called in intake.go and StripMetadata
// in upload.go, and nowhere else.

// guardedCalls are the functions only the intake path may call.
var guardedCalls = map[string]bool{"PrepareUpload": true, "StoreFile": true, "StripMetadata": true}

// storingCalls parses one Go file and returns the names of the guarded
// functions it calls, as a plain call or as a selector (media.StoreFile).
// Declarations of those names are not calls.
func storingCalls(path string, src []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return nil
	}
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if guardedCalls[name] {
			found = append(found, name)
		}
		return true
	})
	return found
}

func TestOnlyIntakeKeepStoresAnUpload(t *testing.T) {
	root := filepath.Join("..", "..")
	// .claude holds linked worktrees, each a whole copy of the tree, when the
	// test runs in the main checkout.
	skip := map[string]bool{".git": true, ".claude": true, ".planning": true, "build": true, "node_modules": true, "vendor": true, "data": true}

	got := map[string][]string{}
	parsed := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed++
		if names := storingCalls(path, src); len(names) > 0 {
			rel, _ := filepath.Rel(root, path)
			got[filepath.ToSlash(rel)] = names
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if parsed < 150 {
		t.Fatalf("the guard parsed %d Go files, want at least 150: it has lost the tree", parsed)
	}

	want := map[string][]string{
		"internal/media/intake.go": {"PrepareUpload", "StoreFile"},
		"internal/media/upload.go": {"StripMetadata"},
	}
	for file, names := range got {
		expected, allowed := want[file]
		if !allowed {
			t.Errorf("%s calls %v: an uploaded file must be stored through Intake.Keep so that the metadata strip cannot be skipped", file, names)
			continue
		}
		have := uniqueSorted(names)
		if strings.Join(have, ",") != strings.Join(expected, ",") {
			t.Errorf("%s calls %v, want exactly %v: an uploaded file must be stored through Intake.Keep so that the metadata strip cannot be skipped", file, have, expected)
		}
	}
	for file, expected := range want {
		if _, ok := got[file]; !ok {
			t.Errorf("%s no longer calls %v: the strip has moved out of the path an upload takes", file, expected)
		}
	}
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// TestStoringCallsDetectionSeesACall shows the walk is not green because the
// detector is blind.
func TestStoringCallsDetectionSeesACall(t *testing.T) {
	calls := `package admin

import "example/media"

func save() { media.StoreFile(nil, "x", 1) }
`
	if got := storingCalls("admin.go", []byte(calls)); len(got) != 1 || got[0] != "StoreFile" {
		t.Errorf("a call of media.StoreFile is reported as %v, want [StoreFile]", got)
	}
	bare := `package media

func f() { PrepareUpload(nil, "", 0) }
`
	if got := storingCalls("f.go", []byte(bare)); len(got) != 1 {
		t.Errorf("a bare call of PrepareUpload is reported as %v, want one", got)
	}
	decl := `package media

func StoreFile() {}
func (x T) StripMetadata() {}
func PrepareUpload() {}
`
	if got := storingCalls("decl.go", []byte(decl)); len(got) != 0 {
		t.Errorf("declarations are reported as calls: %v", got)
	}
}

// The held-back copy of an oversize picture is written before the strip, lives
// for StagingRetention and is never served. That is only acceptable while it
// stays out of the directory the media route serves, so the location is pinned.
func TestStagedUploadsAreNotUnderTheServedDirectory(t *testing.T) {
	dir := t.TempDir()
	root := NewStaging(dir).root
	served := filepath.Join(dir, "media")

	rel, err := filepath.Rel(served, root)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "." || !strings.HasPrefix(rel, "..") {
		t.Errorf("the staging directory %s lies inside the served directory %s", root, served)
	}
	if filepath.Base(root) != "media-staging" {
		t.Errorf("the staging directory is %s, want data/media-staging", root)
	}
	rel, err = filepath.Rel(served, Path(dir, 1, "x.jpg"))
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("a kept file does not lie under the served directory: %s (%v)", rel, err)
	}
}
