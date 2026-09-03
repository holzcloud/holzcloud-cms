package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// jpegWithSegments builds a real JPEG and injects the given segments right
// after SOI, which is where a camera writes them.
func jpegWithSegments(t *testing.T, segments ...[]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 30), uint8(y * 30), 128, 255})
		}
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	data := buf.Bytes()

	out := append([]byte{}, data[:2]...) // SOI
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, data[2:]...)
}

func segment(marker byte, payload []byte) []byte {
	s := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(s[2:4], uint16(len(payload)+2))
	return append(s, payload...)
}

// The case this exists for: a photo taken at home publishes the home address.
func TestStripJPEGRemovesEXIF(t *testing.T) {
	gps := append([]byte("Exif\x00\x00"), []byte("GPSLatitude 48.137 GPSLongitude 11.575")...)
	data := jpegWithSegments(t, segment(0xE1, gps))

	if !bytes.Contains(data, []byte("GPSLatitude")) {
		t.Fatal("the fixture does not actually contain the EXIF block")
	}

	out, err := StripMetadata(data, "image/jpeg")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	if bytes.Contains(out, []byte("GPSLatitude")) {
		t.Error("GPS coordinates survived the strip")
	}
	if bytes.Contains(out, []byte("Exif\x00\x00")) {
		t.Error("the EXIF marker survived the strip")
	}
}

func TestStripJPEGRemovesXMPCommentAndIPTC(t *testing.T) {
	data := jpegWithSegments(t,
		segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>Autor</x:xmpmeta>")),
		segment(0xED, []byte("Photoshop 3.0\x00Fotograf Max Mustermann")),
		segment(0xFE, []byte("Kommentar mit Klarnamen")),
	)

	out, err := StripMetadata(data, "image/jpeg")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	for _, needle := range []string{"xmpmeta", "Fotograf Max Mustermann", "Kommentar mit Klarnamen"} {
		if bytes.Contains(out, []byte(needle)) {
			t.Errorf("%q survived the strip", needle)
		}
	}
}

// Dropping the ICC profile shifts the colours; dropping JFIF changes the print
// size; dropping the Adobe marker turns a CMYK image into garbage.
func TestStripJPEGKeepsColourAndDensitySegments(t *testing.T) {
	// Go's encoder writes no APP0, so the JFIF segment is injected here rather
	// than assumed — otherwise the assertion below would pass vacuously.
	icc := append([]byte("ICC_PROFILE\x00\x01\x01"), []byte("profildaten")...)
	data := jpegWithSegments(t,
		segment(0xE0, []byte("JFIF\x00\x01\x02\x01\x00\x48\x00\x48\x00\x00")),
		segment(0xE2, icc),
		segment(0xEE, []byte("Adobe\x00transform")),
	)

	out, err := StripMetadata(data, "image/jpeg")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	if !bytes.Contains(out, []byte("ICC_PROFILE")) {
		t.Error("the ICC profile was dropped, which shifts every colour")
	}
	if !bytes.Contains(out, []byte("Adobe\x00")) {
		t.Error("the Adobe colour-transform marker was dropped")
	}
	if !bytes.Contains(out, []byte("JFIF")) {
		t.Error("the JFIF density segment was dropped")
	}
}

// The point of byte surgery over re-encoding: the pixels must come out
// bit-identical, not merely visually similar.
func TestStripJPEGIsLossless(t *testing.T) {
	clean := jpegWithSegments(t)
	dirty := jpegWithSegments(t, segment(0xE1, []byte("Exif\x00\x00GPS")))

	out, err := StripMetadata(dirty, "image/jpeg")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	if !bytes.Equal(out, clean) {
		t.Errorf("stripped output differs from the metadata-free encoding (%d vs %d bytes)",
			len(out), len(clean))
	}

	// And it must still decode.
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("the stripped JPEG no longer decodes: %v", err)
	}
}

func TestStripPNGRemovesTextChunks(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	data := buf.Bytes()

	// Inject a tEXt chunk after the signature and IHDR.
	text := pngChunk("tEXt", []byte("Author\x00Max Mustermann"))
	ihdrEnd := len(pngSignature) + 12 + 13
	injected := append(append(append([]byte{}, data[:ihdrEnd]...), text...), data[ihdrEnd:]...)

	out, err := StripMetadata(injected, "image/png")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	if bytes.Contains(out, []byte("Max Mustermann")) {
		t.Error("the tEXt chunk survived the strip")
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("the stripped PNG no longer decodes: %v", err)
	}
}

func pngChunk(kind string, payload []byte) []byte {
	out := make([]byte, 0, 12+len(payload))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(payload)))
	out = append(out, length[:]...)
	out = append(out, kind...)
	out = append(out, payload...)
	// The CRC is not verified by this code path, and png.Decode ignores
	// ancillary chunks it does not know, so a placeholder is enough here.
	return append(out, 0, 0, 0, 0)
}

func TestStripWebPRemovesEXIFChunk(t *testing.T) {
	body := riffChunk("VP8L", []byte("bilddaten"))
	body = append(body, riffChunk("EXIF", []byte("GPSLatitude 48.137"))...)

	data := append([]byte("RIFF"), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(body)+4))
	data = append(data, "WEBP"...)
	data = append(data, body...)

	out, err := StripMetadata(data, "image/webp")
	if err != nil {
		t.Fatalf("StripMetadata: %v", err)
	}
	if bytes.Contains(out, []byte("GPSLatitude")) {
		t.Error("the EXIF chunk survived the strip")
	}
	if !bytes.Contains(out, []byte("bilddaten")) {
		t.Error("the image data was dropped")
	}
	// The declared RIFF size has to match what is actually there, or every
	// decoder rejects the file.
	if got := binary.LittleEndian.Uint32(out[4:8]); int(got) != len(out)-8 {
		t.Errorf("RIFF size is %d, want %d", got, len(out)-8)
	}
}

func riffChunk(kind string, payload []byte) []byte {
	out := append([]byte(kind), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// Stripping is a safeguard; an upload must never fail because of it.
func TestStripLeavesOtherTypesAlone(t *testing.T) {
	for _, mime := range []string{"image/gif", "image/svg+xml", "application/pdf", "application/octet-stream"} {
		in := []byte("beliebige bytes")
		out, err := StripMetadata(in, mime)
		if err != nil {
			t.Errorf("%s: %v", mime, err)
		}
		if !bytes.Equal(out, in) {
			t.Errorf("%s was modified", mime)
		}
	}
}

func TestStripRejectsMislabelledData(t *testing.T) {
	if _, err := StripMetadata([]byte("nicht wirklich ein jpeg"), "image/jpeg"); err == nil {
		t.Error("expected an error for data that is not a JPEG")
	}
}
