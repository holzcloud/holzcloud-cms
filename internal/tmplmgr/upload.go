package tmplmgr

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tmpl "github.com/holzcloud/holzcloud-cms/internal/template"
)

// maxArchiveEntries bounds the number of files in a template archive so a small
// upload cannot expand into an unbounded number of inodes.
const maxArchiveEntries = 500

// allowedExtensions is the set of file extensions permitted in template archives.
//
// .js is deliberately absent. None of the shipped themes needs JavaScript, and
// a rule without exceptions is one a template author can follow without having
// to weigh anything up. See CheckNoScripts for the ways script reaches a page
// without a file of its own.
var allowedExtensions = map[string]bool{
	".html":  true,
	".css":   true,
	".svg":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".webp":  true,
	".ico":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
}

// ExtractTemplate extracts a zip archive to destDir with security validation.
// It extracts to a temporary directory first, then renames atomically.
// maxSize limits the total uncompressed size of the archive.
//
// fallback is the default theme, whose views stand in for the ones an archive
// leaves out; it is needed because those views are still rendered through the
// uploaded layout. Passing nil skips that half of the render check.
func ExtractTemplate(zipReader io.ReaderAt, size int64, destDir string, maxSize int64, fallback fs.FS) error {
	r, err := zip.NewReader(zipReader, size)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	if len(r.File) > maxArchiveEntries {
		return fmt.Errorf("archive contains more than %d entries", maxArchiveEntries)
	}

	// The temp directory has to be a sibling of destDir so the final rename stays
	// on one filesystem. On a fresh install that parent does not exist yet, so it
	// is created up front — otherwise the very first template upload fails.
	parentDir := filepath.Dir(destDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}

	// Extract to temp dir first (Pitfall 3: atomic extraction)
	tempDir, err := os.MkdirTemp(parentDir, ".tmpl-extract-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		// Clean up temp dir on any error (will be gone if rename succeeded)
		os.RemoveAll(tempDir)
	}()

	cleanDest := filepath.Clean(tempDir) + string(os.PathSeparator)
	var totalSize int64

	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		destPath := filepath.Join(tempDir, cleanName)

		// Zip-slip prevention (T-04-01)
		if !strings.HasPrefix(filepath.Clean(destPath)+string(os.PathSeparator), cleanDest) &&
			filepath.Clean(destPath) != filepath.Clean(tempDir) {
			return fmt.Errorf("zip-slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", cleanName, err)
			}
			continue
		}

		// Extension allow-list check (T-04-02)
		ext := strings.ToLower(filepath.Ext(cleanName))
		if !allowedExtensions[ext] {
			if ext == ".js" {
				return fmt.Errorf("templates carry no JavaScript: remove %s "+
					"(a menu, a gallery and an accordion are all possible in CSS alone)", cleanName)
			}
			return fmt.Errorf("disallowed file type: %s", ext)
		}

		// Size check (T-04-04). UncompressedSize64 comes from the archive header
		// and is attacker-controlled, so it is only a cheap pre-filter; the real
		// limit is enforced against the bytes actually written (see extractFile).
		if int64(f.UncompressedSize64) > maxSize-totalSize {
			return fmt.Errorf("archive exceeds maximum size of %d bytes", maxSize)
		}

		written, err := extractFile(f, destPath, maxSize-totalSize)
		if err != nil {
			return err
		}
		totalSize += written
	}

	if problems := CheckTemplateDir(tempDir, fallback); len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}

	// Remove existing dest if present, then rename temp to final
	_ = os.RemoveAll(destDir)
	if err := os.Rename(tempDir, destDir); err != nil {
		return fmt.Errorf("rename to final dir: %w", err)
	}

	return nil
}

// extractFile writes one zip entry to destPath, refusing to write more than
// budget bytes. It returns the number of bytes written.
//
// The budget is enforced on the decompressed stream rather than on the header's
// declared size, so an archive that lies about its uncompressed size (a zip
// bomb) is stopped at the limit instead of filling the disk.
func extractFile(f *zip.File, destPath string, budget int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, fmt.Errorf("create parent: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("create file %s: %w", f.Name, err)
	}
	defer out.Close()

	// Read one byte past the budget so an over-sized entry is detectable.
	written, err := io.Copy(out, io.LimitReader(rc, budget+1))
	if err != nil {
		return written, fmt.Errorf("write file %s: %w", f.Name, err)
	}
	if written > budget {
		return written, fmt.Errorf("archive exceeds maximum uncompressed size")
	}
	return written, nil
}

// CheckTemplateDir is everything that makes a template acceptable, applied to
// an extracted directory. It returns one entry per problem, empty when there is
// none.
//
// One function rather than a sequence inlined into ExtractTemplate, because the
// same verdict has to be reachable without installing anything: `holzcloud
// template check` runs exactly this, so what an author is told locally and what
// the upload would say cannot drift apart.
//
// fallback is the default theme, whose views stand in for the ones an archive
// omits; nil skips that part of the render check.
func CheckTemplateDir(dir string, fallback fs.FS) []string {
	var problems []string

	if err := ValidateTemplateStructure(dir); err != nil {
		// Nothing further is meaningful without the two required files, and
		// every later check would only restate their absence.
		return []string{err.Error()}
	}

	// Nothing may be loaded from a third party at runtime (see CLAUDE.md).
	if refs := CheckNoExternalRefs(dir); len(refs) > 0 {
		problems = append(problems, fmt.Sprintf(
			"template loads external resources, which is not allowed — "+
				"bundle them into the archive instead (%s)", summarizeRefs(refs)))
	}

	if refs := CheckNoScripts(dir); len(refs) > 0 {
		problems = append(problems, fmt.Sprintf(
			"templates carry no JavaScript (%s)", summarizeScripts(refs)))
	}

	// Last, because it is the only check that runs the template rather than
	// reading it. Before this existed, a template naming a field that does not
	// exist was installed without complaint and failed on the next visitor's
	// request, on the live site.
	if found := tmpl.CheckDir(dir, fallback); len(found) > 0 {
		problems = append(problems, "template does not render: "+tmpl.Summarize(found))
	}

	return problems
}

// ValidateTemplateStructure checks that the required template files exist.
func ValidateTemplateStructure(dir string) error {
	required := []string{"layout.html", "page.html"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("required template file missing: %s", name)
		}
	}
	return nil
}
