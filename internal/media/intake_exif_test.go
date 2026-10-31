package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

// A photograph taken at home must not publish the home address.
//
// The strip itself is held by strip_test.go and orient_keep_test.go, which call
// the strip function directly. What those cannot show is that the strip still stands in the
// way of a byte reaching the disk: that is Check followed by Keep, the two
// calls the administration's upload, a form's attachment and the assistant's
// upload link all make. Everything here goes in through those two and reads
// the stored file back; nothing reaches for the strip or for the storing
// functions itself, or the test would prove nothing about Keep.

// checkedUpload takes content in the way an upload is taken in: as the part of
// a multipart body, parsed back, handed to Check with the limits of the
// administration. The sniffing, the size bound and the Intake come out of Check
// itself.
func checkedUpload(t *testing.T, name string, content []byte) *Intake {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	form, err := multipart.NewReader(&body, w.Boundary()).ReadForm(64 << 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { form.RemoveAll() })
	headers := form.File["file"]
	if len(headers) != 1 {
		t.Fatalf("the multipart form holds %d files, want 1", len(headers))
	}
	file, err := headers[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	in, err := Check(file, headers[0], Limits{Image: 32 << 20, Video: 64 << 20})
	if err != nil {
		t.Fatalf("Check refused %s: %v", name, err)
	}
	return in
}

// keptBytes keeps in into a fresh data directory and reads the stored file back.
func keptBytes(t *testing.T, in *Intake) (stored []byte, m *Media, existed bool) {
	t.Helper()
	store, websiteID := newTestStore(t)
	return keptInto(t, store, websiteID, t.TempDir(), in)
}

func keptInto(t *testing.T, store *Store, websiteID int64, dataDir string, in *Intake) ([]byte, *Media, bool) {
	t.Helper()
	m, existed, err := in.Keep(context.Background(), store, websiteID, dataDir)
	if err != nil {
		t.Fatalf("Keep: %v", err)
	}
	stored, err := os.ReadFile(Path(dataDir, websiteID, m.Filename))
	if err != nil {
		t.Fatalf("the kept file is not on disk: %v", err)
	}
	return stored, m, existed
}

// leakMarkers are the strings the fixtures carry in their metadata.
var leakMarkers = append(append([]string{}, richMarkers...),
	"GPS", "XMP-GEHEIM", "IPTC-GEHEIM", "KOMMENTAR-GEHEIM")

// mustCarry fails when a fixture does not hold every marker, so that a fixture
// which stopped carrying the data cannot make a test pass.
func mustCarry(t *testing.T, label string, upload []byte) {
	t.Helper()
	for _, m := range leakMarkers {
		if !bytes.Contains(upload, []byte(m)) {
			t.Fatalf("%s: the fixture does not carry %q, the test would prove nothing", label, m)
		}
	}
}

func mustNotLeak(t *testing.T, label string, stored []byte) {
	t.Helper()
	for _, m := range leakMarkers {
		if bytes.Contains(stored, []byte(m)) {
			t.Errorf("%s: the stored file still contains %q", label, m)
		}
	}
}

func TestKeepStoresNoCameraDataAndKeepsTheTurn(t *testing.T) {
	const turn = 6

	t.Run("JPEG", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			order byteOrder
		}{{"little-endian", binary.LittleEndian}, {"big-endian", binary.BigEndian}} {
			t.Run(tc.name, func(t *testing.T) {
				upload := withSegmentsAfterSOI(quadrantJPEG(t, 64, 48),
					exifSegment(richEXIF(tc.order, turn)),
					segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>")),
					segment(0xED, []byte("Photoshop 3.0\x008BIM IPTC-GEHEIM")),
					segment(0xFE, []byte("KOMMENTAR-GEHEIM")),
				)
				mustCarry(t, "JPEG "+tc.name, upload)

				stored, _, _ := keptBytes(t, checkedUpload(t, "foto.jpg", upload))

				mustNotLeak(t, "JPEG "+tc.name, stored)
				if _, err := jpeg.Decode(bytes.NewReader(stored)); err != nil {
					t.Errorf("JPEG %s: the stored file no longer decodes: %v", tc.name, err)
				}
				if got := orientationOf("image/jpeg", stored); got != turn {
					t.Errorf("JPEG %s: the stored file reports orientation %d, want %d", tc.name, got, turn)
				}
			})
		}
	})

	t.Run("PNG", func(t *testing.T) {
		upload := pngBytes(t, quadrants(40, 30))
		upload = pngWithChunk(t, upload, "iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>"), false)
		upload = pngWithChunk(t, upload, "tEXt", []byte("Comment\x00GPS 48.137 11.575 IPTC-GEHEIM KOMMENTAR-GEHEIM"), false)
		upload = pngWithChunk(t, upload, "eXIf", richEXIF(binary.BigEndian, turn), false)
		mustCarry(t, "PNG", upload)

		stored, _, _ := keptBytes(t, checkedUpload(t, "foto.png", upload))

		mustNotLeak(t, "PNG", stored)
		if _, err := png.Decode(bytes.NewReader(stored)); err != nil {
			t.Errorf("PNG: the stored file no longer decodes: %v", err)
		}
		if got := orientationOf("image/png", stored); got != turn {
			t.Errorf("PNG: the stored file reports orientation %d, want %d", got, turn)
		}
	})

	t.Run("WebP", func(t *testing.T) {
		vp8l, w, h := vp8lOf(t)
		upload := webpContainer(
			[]byte("VP8X"), vp8xPayload(0x0C, w, h),
			[]byte("VP8L"), vp8l,
			[]byte("EXIF"), richEXIF(binary.LittleEndian, turn),
			[]byte("XMP "), []byte("<x:xmpmeta>XMP-GEHEIM IPTC-GEHEIM KOMMENTAR-GEHEIM</x:xmpmeta>"),
		)
		mustCarry(t, "WebP", upload)

		stored, _, _ := keptBytes(t, checkedUpload(t, "foto.webp", upload))

		mustNotLeak(t, "WebP", stored)
		if _, _, err := image.Decode(bytes.NewReader(stored)); err != nil {
			t.Errorf("WebP: the stored file no longer decodes: %v", err)
		}
		if got := orientationOf("image/webp", stored); got != turn {
			t.Errorf("WebP: the stored file reports orientation %d, want %d", got, turn)
		}
	})
}

func TestKeepOfAScaledCopyCarriesNoCameraData(t *testing.T) {
	upload := withSegmentsAfterSOI(quadrantJPEG(t, 3000, 1500),
		exifSegment(richEXIF(binary.LittleEndian, 6)),
		segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>")),
		segment(0xED, []byte("Photoshop 3.0\x008BIM IPTC-GEHEIM")),
		segment(0xFE, []byte("KOMMENTAR-GEHEIM")),
	)
	mustCarry(t, "scaled JPEG", upload)

	scaled, err := checkedUpload(t, "gross.jpg", upload).Scaled(0)
	if err != nil {
		t.Fatalf("Scaled: %v", err)
	}
	stored, _, _ := keptBytes(t, scaled)

	mustNotLeak(t, "scaled JPEG", stored)
	if got := orientationOf("image/jpeg", stored); got != 1 {
		t.Errorf("scaled JPEG: the stored copy reports orientation %d, want 1 because the turn is in its pixels", got)
	}
	if w, h := dimensionsOf(t, stored); w != 1280 || h != 2560 {
		t.Errorf("scaled JPEG: the stored copy is %d x %d, want 1280 x 2560 (turned upright)", w, h)
	}
}

func TestKeepRecordsTheBytesOnDisk(t *testing.T) {
	upload := withSegmentsAfterSOI(quadrantJPEG(t, 64, 48), exifSegment(richEXIF(binary.BigEndian, 6)))
	store, websiteID := newTestStore(t)
	dataDir := t.TempDir()

	stored, m, existed := keptInto(t, store, websiteID, dataDir, checkedUpload(t, "foto.jpg", upload))
	if existed {
		t.Fatal("the first Keep reports the file as already stored")
	}
	if m.SizeBytes != int64(len(stored)) {
		t.Errorf("the record says %d bytes, the file on disk holds %d (the size the sender sent is not the size stored)", m.SizeBytes, len(stored))
	}
	if int64(len(stored)) >= int64(len(upload)) {
		t.Errorf("the stored file (%d bytes) is not smaller than the upload (%d): nothing was stripped", len(stored), len(upload))
	}

	// The same bytes again, as a new upload with a new generated name.
	again, m2, existed := keptInto(t, store, websiteID, dataDir, checkedUpload(t, "noch-mal.jpg", upload))
	if !existed {
		t.Error("the second Keep of identical bytes does not report existed")
	}
	if m2.ID != m.ID {
		t.Errorf("the second Keep returned record %d, want the first, %d", m2.ID, m.ID)
	}
	if !bytes.Equal(again, stored) {
		t.Error("the file behind the first record changed")
	}
	entries, err := os.ReadDir(filepath.Dir(Path(dataDir, websiteID, m.Filename)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d files on disk after a duplicate upload, want 1", len(entries))
	}
}
