package media

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// allowedMIME is the set of permitted MIME types for media uploads.
var allowedMIME = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"image/svg+xml":   true,
	"application/pdf": true,
	// Ein eigenes Video, kein eingebettetes. Der Baustein "Video" zeigt es in
	// einem <video>-Element von diesem Server — genau das, was die Einbettung
	// bei YouTube unmöglich macht, ohne die Regel zu brechen.
	"video/mp4": true,
}

// ValidateMIME reads the first 512 bytes to detect MIME type via magic bytes.
// For SVG files, it also checks the file extension as a fallback since
// http.DetectContentType may return text/xml or text/plain for SVGs.
// The reader is seeked back to start after reading.
func ValidateMIME(file io.ReadSeeker, originalName string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read file header: %w", err)
	}
	head := buf[:n]

	detected := http.DetectContentType(head)

	// Seek back to start for subsequent copy
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek to start: %w", err)
	}

	// Check if detected MIME is allowed
	if allowedMIME[detected] {
		return detected, nil
	}

	// SVG fallback: DetectContentType returns text/xml or text/plain for SVGs.
	// The extension alone is not enough — accepting any text/* under a .svg name
	// lets an HTML document through, which is then served inline from our own
	// origin. The content must actually open an <svg> root element.
	if strings.ToLower(filepath.Ext(originalName)) == ".svg" &&
		strings.HasPrefix(detected, "text/") && looksLikeSVG(head) {
		return "image/svg+xml", nil
	}

	return "", fmt.Errorf("disallowed MIME type: %s", detected)
}

// looksLikeSVG reports whether the sniffed header opens an SVG document,
// ignoring a leading XML declaration, doctype, comments and whitespace.
func looksLikeSVG(head []byte) bool {
	s := strings.ToLower(string(head))
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		switch {
		case strings.HasPrefix(s, "<?xml"), strings.HasPrefix(s, "<!doctype"), strings.HasPrefix(s, "<!--"):
			end := strings.IndexByte(s, '>')
			if end < 0 {
				return false
			}
			s = s[end+1:]
		case strings.HasPrefix(s, "<svg"):
			return true
		default:
			return false
		}
	}
}

// GenerateFilename creates a unique filename with a 16-byte random hex prefix
// and the original file extension.
func GenerateFilename(originalName string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	prefix := hex.EncodeToString(b)
	ext := strings.ToLower(filepath.Ext(originalName))
	return prefix + ext
}

// StoreFile writes a file to disk with a size limit enforced via io.LimitReader.
//
// It returns the number of bytes actually written — the multipart header's Size
// is client-supplied — and the SHA-256 of those bytes. The hash rides along on a
// pass over data that is being copied anyway, so it is effectively free.
func StoreFile(file io.Reader, destPath string, maxSize int64) (int64, string, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, "", fmt.Errorf("create media dir: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return 0, "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	digest := sha256.New()
	limited := io.LimitReader(file, maxSize+1)
	written, err := io.Copy(io.MultiWriter(out, digest), limited)
	if err != nil {
		os.Remove(destPath)
		return 0, "", fmt.Errorf("write file: %w", err)
	}
	if written > maxSize {
		os.Remove(destPath)
		return 0, "", fmt.Errorf("file exceeds maximum size of %d bytes", maxSize)
	}

	return written, hex.EncodeToString(digest.Sum(nil)), nil
}

// PrepareUpload reads an upload into memory and removes camera metadata.
//
// Stripping needs the whole file, so this is bounded by maxSize, which the
// caller has already applied to the request body. Types that carry no metadata
// worth removing stream straight through and never reach memory.
//
// A strip failure is not an upload failure: the original bytes are returned and
// the caller logs it. Refusing an upload because a JPEG had an unusual segment
// chain would be worse than the metadata.
func PrepareUpload(file io.Reader, mimeType string, maxSize int64) (io.Reader, error) {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
	case "video/mp4":
		// Dieselbe Regel wie beim Foto: Aufnahmeort und Gerät verlassen die
		// Datei, bevor sie auf der Platte liegt. Siehe mp4.go, warum dabei
		// nichts kürzer wird.
		raw, err := io.ReadAll(io.LimitReader(file, maxSize+1))
		if err != nil {
			return nil, fmt.Errorf("read upload: %w", err)
		}
		return bytes.NewReader(StripMP4Metadata(raw)), nil
	default:
		return file, nil
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	stripped, err := StripMetadata(raw, mimeType)
	if err != nil {
		return bytes.NewReader(raw), err
	}
	return bytes.NewReader(stripped), nil
}
