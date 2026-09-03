package tmplmgr

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipEntry describes one file to place in a test archive.
type zipEntry struct {
	name string
	body []byte
}

// buildZip produces a well-formed archive containing the given entries.
func buildZip(t *testing.T, entries ...zipEntry) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", e.name, err)
		}
		if _, err := w.Write(e.body); err != nil {
			t.Fatalf("write zip entry %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return bytes.NewReader(buf.Bytes())
}

// validTemplate is the minimum set of files ValidateTemplateStructure requires.
func validTemplate(extra ...zipEntry) []zipEntry {
	// A real template, not a placeholder: ExtractTemplate renders what it is
	// given before accepting it, so a fixture that does not render is a fixture
	// that tests the render check rather than the thing each test is about.
	return append([]zipEntry{
		{name: "layout.html", body: []byte(
			`<html lang="{{.Site.Locale}}"><body>{{block "content" .}}{{end}}</body></html>`)},
		{name: "page.html", body: []byte(
			`{{define "content"}}<h1>{{.Page.Title}}</h1>{{.Page.ContentHTML}}{{end}}`)},
	}, extra...)
}

func TestExtractTemplateAcceptsValidArchive(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t, validTemplate(zipEntry{name: "style.css", body: []byte("body{}")})...)

	if err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil); err != nil {
		t.Fatalf("ExtractTemplate: %v", err)
	}
	for _, name := range []string{"layout.html", "page.html", "style.css"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("expected %s to be extracted: %v", name, err)
		}
	}
}

func TestExtractTemplateRejectsZipSlip(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "theme")
	r := buildZip(t, validTemplate(zipEntry{name: "../../escaped.html", body: []byte("pwned")})...)

	err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
	if err == nil {
		t.Fatal("expected zip-slip to be rejected")
	}
	if !strings.Contains(err.Error(), "zip-slip") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(base, "escaped.html")); statErr == nil {
		t.Error("zip-slip wrote a file outside the destination directory")
	}
}

func TestExtractTemplateRejectsDisallowedExtension(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t, validTemplate(zipEntry{name: "payload.sh", body: []byte("#!/bin/sh")})...)

	err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
	if err == nil || !strings.Contains(err.Error(), "disallowed file type") {
		t.Fatalf("expected disallowed file type error, got %v", err)
	}
}

// A highly compressible archive that expands past the limit must be refused,
// and refusing it must leave nothing behind at the destination.
func TestExtractTemplateRejectsOversizedArchive(t *testing.T) {
	const maxSize = 4096

	payload := bytes.Repeat([]byte("A"), 512*1024)
	r := buildZip(t, validTemplate(zipEntry{name: "big.css", body: payload})...)
	dest := filepath.Join(t.TempDir(), "theme")

	err := ExtractTemplate(r, int64(r.Len()), dest, maxSize, nil)
	if err == nil {
		t.Fatal("expected the oversized archive to be rejected")
	}
	if !strings.Contains(err.Error(), "maximum") {
		t.Errorf("expected a size-limit error, got %v", err)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("rejected archive was still installed at the destination")
	}
}

// The uncompressed size in a zip header is attacker-controlled, so it may not be
// the only thing the limit is checked against. An entry that under-declares its
// size must still be stopped and must not be installed.
func TestExtractTemplateRejectsForgedUncompressedSize(t *testing.T) {
	const maxSize = 4096

	payload := bytes.Repeat([]byte("A"), 512*1024)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range validTemplate(zipEntry{name: "big.css", body: payload}) {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Only the bomb entry lies about its size, so the other entries extract
	// normally and the failure is attributable to the forged one.
	archive := forgeUncompressedSizes(buf.Bytes(), uint32(len(payload)), 10)
	dest := filepath.Join(t.TempDir(), "theme")

	if err := ExtractTemplate(bytes.NewReader(archive), int64(len(archive)), dest, maxSize, nil); err == nil {
		t.Fatal("expected the forged archive to be rejected")
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("rejected archive was still installed at the destination")
	}
}

// Extraction goes to a temporary directory and is renamed into place only after
// validation, so a failed upload must not disturb an already-installed template.
func TestExtractTemplateLeavesExistingTemplateOnFailure(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")

	good := buildZip(t, validTemplate(zipEntry{name: "style.css", body: []byte("original")})...)
	if err := ExtractTemplate(good, int64(good.Len()), dest, 1<<20, nil); err != nil {
		t.Fatalf("install original: %v", err)
	}

	bad := buildZip(t, zipEntry{name: "style.css", body: []byte("replacement")})
	if err := ExtractTemplate(bad, int64(bad.Len()), dest, 1<<20, nil); err == nil {
		t.Fatal("expected the invalid archive to be rejected")
	}

	content, err := os.ReadFile(filepath.Join(dest, "style.css"))
	if err != nil {
		t.Fatalf("original template was removed: %v", err)
	}
	if string(content) != "original" {
		t.Errorf("original template was overwritten: %q", content)
	}
}

func TestExtractTemplateRequiresLayoutAndPage(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t, zipEntry{name: "style.css", body: []byte("body{}")})

	err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
	if err == nil || !strings.Contains(err.Error(), "required template file missing") {
		t.Fatalf("expected missing-file error, got %v", err)
	}
}

// forgeUncompressedSizes rewrites the uncompressed-size field of every header
// that currently declares actual bytes, replacing it with claimed. It simulates
// an archive that lies about how much data it expands to.
func forgeUncompressedSizes(archive []byte, actual, claimed uint32) []byte {
	out := append([]byte(nil), archive...)
	patch := func(sig []byte, offset int) {
		for i := 0; i+offset+4 <= len(out); i++ {
			if !bytes.HasPrefix(out[i:], sig) {
				continue
			}
			field := out[i+offset : i+offset+4]
			if binary.LittleEndian.Uint32(field) == actual {
				binary.LittleEndian.PutUint32(field, claimed)
			}
		}
	}
	// Local file header: uncompressed size at offset 22.
	patch([]byte("PK\x03\x04"), 22)
	// Central directory header: uncompressed size at offset 24.
	patch([]byte("PK\x01\x02"), 24)
	return out
}

// Templates carry no JavaScript. The extension list keeps .js files out; these
// are the ways script reaches a page without a file of its own.
func TestExtractTemplateRejectsJavaScript(t *testing.T) {
	cases := map[string]zipEntry{
		"a .js file": {
			name: "theme.js", body: []byte("console.log('hi')")},
		"an inline script": {
			name: "extra.html", body: []byte(`<html><body><script>alert(1)</script></body></html>`)},
		"an event handler": {
			name: "extra.html", body: []byte(`<html><body><button onclick="go()">x</button></body></html>`)},
		"a javascript: URL": {
			name: "extra.html", body: []byte(`<html><body><a href="javascript:go()">x</a></body></html>`)},
		"a script inside an SVG": {
			name: "icon.svg", body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>go()</script></svg>`)},
	}

	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "theme")
			r := buildZip(t, validTemplate(entry)...)

			err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
			if err == nil {
				t.Fatalf("a template carrying %s was accepted", name)
			}
			if !strings.Contains(err.Error(), "JavaScript") {
				t.Errorf("the message does not say what the rule is: %v", err)
			}
		})
	}
}

// A schema.org block is data, not code: the browser never prepares a script
// from it. The shipped themes carry one, so an uploaded template may too.
func TestExtractTemplateAcceptsAJSONDataBlock(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t, validTemplate(zipEntry{
		name: "extra.html",
		body: []byte(`<html><head><script type="application/ld+json">{"@type":"WebPage"}</script></head><body>x</body></html>`),
	})...)

	if err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil); err != nil {
		t.Fatalf("a template carrying only a JSON-LD data block was rejected: %v", err)
	}
}

// The check that matters most for a template written by an agent: a field that
// does not exist has to be refused at upload, with the mistake named, rather
// than installed and discovered by a visitor.
func TestExtractTemplateRejectsATemplateThatCannotRender(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "theme")
	r := buildZip(t,
		zipEntry{name: "layout.html", body: []byte(
			`<html><body>{{block "content" .}}{{end}}</body></html>`)},
		zipEntry{name: "page.html", body: []byte(
			`{{define "content"}}<h1>{{.Page.Titel}}</h1>{{end}}`)},
	)

	err := ExtractTemplate(r, int64(r.Len()), dest, 1<<20, nil)
	if err == nil {
		t.Fatal("a template naming a field that does not exist was installed")
	}
	for _, want := range []string{"page.html", "Titel", "Title"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message does not mention %q: %v", want, err)
		}
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("the rejected template was installed anyway")
	}
}
