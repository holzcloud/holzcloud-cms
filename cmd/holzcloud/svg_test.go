package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
)

// Every SVG this binary serves must be well-formed XML.
//
// A browser draws an SVG through an XML parser, and one that is not
// well-formed is not drawn at all — no partial image, no console line a
// visitor would see. The favicons of `holzcloud` and `weide` carried a
// comment that named `--hc-ground`, and `--` is forbidden inside an XML
// comment, so both websites on those themes showed no icon of their own and
// nobody noticed, because an HTML template engine never parses the file.
func TestEmbeddedSVGsAreWellFormed(t *testing.T) {
	n := 0
	err := fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".svg") {
			return err
		}
		n++
		b, err := fs.ReadFile(staticFS, path)
		if err != nil {
			return err
		}
		dec := xml.NewDecoder(bytes.NewReader(b))
		for {
			_, err := dec.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Errorf("%s: not well-formed XML, a browser will not draw it: %v", path, err)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no SVG found in the embedded files — the walk looked in the wrong place")
	}
}
