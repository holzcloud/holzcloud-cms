package public

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// onePixel is the smallest real PNG: the checks read the bytes, so a file that
// only claims to be a picture is refused, which is the point.
func onePixel(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// multipartBody builds a submission with fields and files.
func multipartBody(t *testing.T, fields map[string]string, files map[string][]byte) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		part, err := w.CreateFormFile("anhang", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), w.FormDataContentType()
}

// Nothing reaches the disk until the plugin says so.
//
// This is the whole design. A contact form has to run its spam traps on the
// fields BEFORE anything is stored, or a robot with a five-megabyte attachment
// fills the disk whatever the traps decide. Check first, Keep second, and a
// submission the plugin refuses simply never calls Keep.
func TestNothingIsWrittenUntilThePluginKeepsIt(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	dir := t.TempDir()
	h.dataDir = dir
	h.SetUploadLimits(5<<20, 50<<20)

	body, ctype := multipartBody(t,
		map[string]string{"name": "Anna", "nachricht": "Hallo"},
		map[string][]byte{"bild.png": onePixel(t)})
	req := httptest.NewRequest("POST", "/formular", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)

	fields, keeper, took := h.takeAttachments(req, ws.ID, true)
	if !took {
		t.Fatal("the multipart submission was not taken")
	}

	// The fields come through as an ordinary encoded form, so a plugin written
	// before attachments existed reads what it always read.
	for _, want := range []string{"name=Anna", "nachricht=Hallo"} {
		if !strings.Contains(fields, want) {
			t.Errorf("the fields do not carry %q: %s", want, fields)
		}
	}
	if len(keeper.List()) != 1 {
		t.Fatalf("%d files listed, want 1", len(keeper.List()))
	}
	f := keeper.List()[0]
	if f.MimeType != "image/png" || f.Refused != "" {
		t.Errorf("listed as %+v", f)
	}
	// The bytes are not in what the plugin sees, and there is no field for
	// them: that is enforced by the type and asserted here so that adding one
	// is a decision.
	if f.Size != int64(len(onePixel(t))) {
		t.Errorf("size = %d", f.Size)
	}

	// Refused: Keep is never called, and the directory stays empty.
	mediaDir := filepath.Join(dir, "media")
	if entries, err := os.ReadDir(mediaDir); err == nil && len(entries) > 0 {
		t.Fatalf("%d entries were written before anybody kept them", len(entries))
	}

	// Kept: now it is there.
	kept, err := keeper.Keep(context.Background())
	if err != nil {
		t.Fatalf("Keep: %v", err)
	}
	if len(kept) != 1 || kept[0].MediaID == 0 || kept[0].Filename == "" {
		t.Fatalf("kept = %+v", kept)
	}
	if kept[0].Name != "bild.png" {
		t.Errorf("the sender's own name was lost: %q", kept[0].Name)
	}
	path := filepath.Join(dir, "media", "1", kept[0].Filename)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file is not on the disk: %v", err)
	}

	// A second Keep answers the same and writes nothing further.
	again, err := keeper.Keep(context.Background())
	if err != nil || len(again) != 1 || again[0].MediaID != kept[0].MediaID {
		t.Errorf("a second keep = %+v, %v", again, err)
	}
}

// A file the checks refuse is listed with its reason and never held.
func TestARefusedFileIsNamedAndNotHeld(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	h.dataDir = t.TempDir()
	h.SetUploadLimits(5<<20, 50<<20)

	// Not a picture, whatever it is called: the kind comes from the bytes.
	body, ctype := multipartBody(t, map[string]string{"name": "Bot"},
		map[string][]byte{"schad.png": []byte("#!/bin/sh\nrm -rf /\n")})
	req := httptest.NewRequest("POST", "/formular", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)

	_, keeper, took := h.takeAttachments(req, ws.ID, true)
	if !took {
		t.Fatal("not taken")
	}
	listed := keeper.List()
	if len(listed) != 1 || listed[0].Refused == "" {
		t.Fatalf("listed = %+v, want one with a reason", listed)
	}
	kept, err := keeper.Keep(context.Background())
	if err != nil || len(kept) != 0 {
		t.Errorf("a refused file was kept: %+v, %v", kept, err)
	}
}

// A plugin without the permission gets the body it always got.
func TestWithoutThePermissionTheBodyIsUnchanged(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	h.dataDir = t.TempDir()
	h.SetUploadLimits(5<<20, 50<<20)

	body, ctype := multipartBody(t, map[string]string{"name": "Anna"},
		map[string][]byte{"bild.png": onePixel(t)})
	req := httptest.NewRequest("POST", "/formular", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)

	if _, _, took := h.takeAttachments(req, ws.ID, false); took {
		t.Error("a plugin without the permission was given the files")
	}
}

// More files than allowed are named, not silently dropped.
//
// A form that offers three and takes one is a form that loses files without
// saying so, which is the failure nobody reports because nobody notices.
func TestTooManyFilesAreNamed(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Hofladen")
	h.dataDir = t.TempDir()
	h.SetUploadLimits(5<<20, 50<<20)

	files := map[string][]byte{}
	for _, name := range []string{"a.png", "b.png", "c.png", "d.png", "e.png"} {
		files[name] = onePixel(t)
	}
	body, ctype := multipartBody(t, map[string]string{"name": "Anna"}, files)
	req := httptest.NewRequest("POST", "/formular", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)

	_, keeper, _ := h.takeAttachments(req, ws.ID, true)
	listed := keeper.List()
	if len(listed) != len(files) {
		t.Fatalf("%d listed, want all %d named", len(listed), len(files))
	}
	var refused int
	for _, f := range listed {
		if f.Refused != "" {
			refused++
		}
	}
	if refused != len(files)-MaxAttachments {
		t.Errorf("%d refused, want %d", refused, len(files)-MaxAttachments)
	}
	kept, _ := keeper.Keep(context.Background())
	if len(kept) != MaxAttachments {
		t.Errorf("%d kept, want %d", len(kept), MaxAttachments)
	}
}

// The keeper answers the ABI's own type, so a change to one is a change to both.
var _ plugin.FileKeeper = (*attachmentKeeper)(nil)
