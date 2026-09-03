package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngHeader is a minimal PNG magic-byte prefix, enough for DetectContentType.
var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestValidateMIMEAcceptsRealImages(t *testing.T) {
	cases := map[string]struct {
		name string
		body []byte
		want string
	}{
		"png": {"logo.png", pngHeader, "image/png"},
		"gif": {"anim.gif", []byte("GIF89a\x01\x00\x01\x00"), "image/gif"},
		"jpeg": {
			"photo.jpg",
			append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), bytes.Repeat([]byte{0}, 32)...),
			"image/jpeg",
		},
		"svg": {
			"icon.svg",
			[]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`),
			"image/svg+xml",
		},
		"svg without declaration": {
			"icon.svg",
			[]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"),
			"image/svg+xml",
		},
		"svg with leading comment": {
			"icon.svg",
			[]byte("<!-- drawn by hand -->\n<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"),
			"image/svg+xml",
		},
	}

	for label, tc := range cases {
		got, err := ValidateMIME(bytes.NewReader(tc.body), tc.name)
		if err != nil {
			t.Errorf("%s: ValidateMIME returned error: %v", label, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: ValidateMIME = %q; want %q", label, got, tc.want)
		}
	}
}

// An HTML document renamed to .svg used to pass, because any text/* result was
// accepted for a .svg name. It would then be served inline from our own origin.
func TestValidateMIMERejectsHTMLDisguisedAsSVG(t *testing.T) {
	payloads := map[string][]byte{
		"plain html":       []byte("<html><body><script>alert(1)</script></body></html>"),
		"script only":      []byte("<script>fetch('/admin/users')</script>"),
		"xml but not svg":  []byte(`<?xml version="1.0"?><note><to>x</to></note>`),
		"leading text":     []byte("hello <svg onload=\"alert(1)\"></svg>"),
		"doctype then div": []byte("<!DOCTYPE html><div>not an svg</div>"),
	}

	for label, body := range payloads {
		got, err := ValidateMIME(bytes.NewReader(body), "payload.svg")
		if err == nil {
			t.Errorf("%s: accepted as %q; want rejection", label, got)
		}
	}
}

func TestValidateMIMERejectsDisallowedTypes(t *testing.T) {
	// ELF binary magic — never an allowed upload.
	body := append([]byte("\x7fELF\x02\x01\x01"), bytes.Repeat([]byte{0}, 64)...)
	if _, err := ValidateMIME(bytes.NewReader(body), "payload.png"); err == nil {
		t.Error("expected an ELF binary to be rejected regardless of its extension")
	}
}

// ValidateMIME must rewind so the caller can copy the whole file afterwards.
func TestValidateMIMERewindsReader(t *testing.T) {
	body := append(pngHeader, bytes.Repeat([]byte("payload"), 100)...)
	r := bytes.NewReader(body)

	if _, err := ValidateMIME(r, "logo.png"); err != nil {
		t.Fatalf("ValidateMIME: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read after validate: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), body) {
		t.Errorf("reader not rewound: got %d bytes, want %d", buf.Len(), len(body))
	}
}

func TestGenerateFilenameIsRandomAndKeepsExtension(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := GenerateFilename("My Photo.PNG")
		if filepath.Ext(name) != ".png" {
			t.Fatalf("expected lowercased .png extension, got %q", name)
		}
		if strings.ContainsAny(name, "/\\ ") {
			t.Fatalf("generated name is not a safe single segment: %q", name)
		}
		if seen[name] {
			t.Fatalf("duplicate generated filename: %q", name)
		}
		seen[name] = true
	}
}

func TestStoreFileEnforcesSizeLimit(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "sub", "file.bin")

	written, hash, err := StoreFile(bytes.NewReader(bytes.Repeat([]byte("x"), 100)), dest, 1000)
	if err != nil {
		t.Fatalf("StoreFile within limit: %v", err)
	}
	if written != 100 {
		t.Errorf("StoreFile returned %d bytes written; want 100", written)
	}

	// The hash is what makes a duplicate upload recognisable, so it has to be
	// the hash of the stored bytes, not of anything else.
	want := sha256.Sum256(bytes.Repeat([]byte("x"), 100))
	if hash != hex.EncodeToString(want[:]) {
		t.Errorf("StoreFile hash = %q, want the SHA-256 of the written bytes", hash)
	}

	over := filepath.Join(t.TempDir(), "over.bin")
	if _, _, err := StoreFile(bytes.NewReader(bytes.Repeat([]byte("x"), 5000)), over, 1000); err == nil {
		t.Error("expected an over-limit file to be rejected")
	}
}

// The privacy guarantee, end to end: what lands on disk must not carry the
// GPS block the upload came in with.
func TestPrepareUploadRemovesMetadataBeforeStoring(t *testing.T) {
	dirty := jpegWithSegments(t, segment(0xE1, []byte("Exif\x00\x00GPSLatitude 48.137")))
	source, err := PrepareUpload(bytes.NewReader(dirty), "image/jpeg", 1<<20)
	if err != nil {
		t.Fatalf("PrepareUpload: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "foto.jpg")
	if _, _, err := StoreFile(source, dest, 1<<20); err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	stored, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if bytes.Contains(stored, []byte("GPSLatitude")) {
		t.Error("the stored file still carries the GPS coordinates")
	}
}

// A type with nothing worth stripping must stream through untouched.
func TestPrepareUploadPassesOtherTypesThrough(t *testing.T) {
	in := []byte("%PDF-1.4 beliebige bytes")
	source, err := PrepareUpload(bytes.NewReader(in), "application/pdf", 1<<20)
	if err != nil {
		t.Fatalf("PrepareUpload: %v", err)
	}
	got, _ := io.ReadAll(source)
	if !bytes.Equal(got, in) {
		t.Error("a PDF was modified on the way to disk")
	}
}
