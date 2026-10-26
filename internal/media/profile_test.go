package media

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"os"
	"testing"
)

// testProfile builds an ICC profile of exactly size bytes (at least 132): the
// size in bytes 0-3, the colour space in 16-19, "XYZ " as connection space,
// the 'acsp' signature at 36, a tag count of zero, and a pattern after it.
func testProfile(size int, space string) []byte {
	p := make([]byte, size)
	binary.BigEndian.PutUint32(p, uint32(size))
	copy(p[16:], space)
	copy(p[20:], "XYZ ")
	copy(p[36:], "acsp")
	for i := 132; i < size; i++ {
		p[i] = byte(i*7 + i/251)
	}
	return p
}

// app2 is one APP2 ICC_PROFILE segment.
func app2(seq, count int, body []byte) []byte {
	payload := append([]byte("ICC_PROFILE\x00"), byte(seq), byte(count))
	payload = append(payload, body...)
	n := len(payload) + 2
	return append([]byte{0xFF, 0xE2, byte(n >> 8), byte(n)}, payload...)
}

// jpegWithICC inserts the profile, cut into pieces of at most per bytes, after
// the SOI of a JPEG.
func jpegWithICC(jpg, profile []byte, per int) []byte {
	var segs [][]byte
	count := (len(profile) + per - 1) / per
	for i := 0; i < count; i++ {
		segs = append(segs, app2(i+1, count, profile[i*per:min((i+1)*per, len(profile))]))
	}
	return insertSegments(jpg, segs...)
}

func insertSegments(jpg []byte, segs ...[]byte) []byte {
	out := append([]byte{}, jpg[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, jpg[2:]...)
}

func iccpPayload(name string, method byte, stream []byte) []byte {
	return append(append([]byte(name), 0, method), stream...)
}

func deflated(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngWithICC puts an iCCP chunk carrying the profile after IHDR.
func pngWithICC(t *testing.T, png, profile []byte) []byte {
	t.Helper()
	return pngWithChunk(t, png, "iCCP", iccpPayload("test", 0, deflated(t, profile)), false)
}

func TestProfileCarriedJPEG(t *testing.T) {
	base := quadrantJPEG(t, 3000, 1500)
	for _, size := range []int{200, 300_000} {
		profile := testProfile(size, "RGB ")
		src := jpegWithICC(base, profile, 65519)
		out, err := intakeOf("image/jpeg", "a.jpg", src).Scaled(0)
		if err != nil {
			t.Fatal(err)
		}
		if got := profileOf(out.MimeType, out.content); !bytes.Equal(got, profile) {
			t.Errorf("%d bytes: the profile of the copy differs (%d bytes)", size, len(got))
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(out.content)); err != nil {
			t.Errorf("%d bytes: the copy does not decode: %v", size, err)
		}
		// Every APP2 segment fits its length field and the numbers run 1..n.
		wantCount := (size + 65518) / 65519
		seq := 0
		walkJPEG(out.content, func(marker byte, payload []byte) bool {
			if marker != markerAPP2 {
				return true
			}
			if len(payload)+2 > 65535 {
				t.Errorf("%d bytes: a segment of %d bytes", size, len(payload)+2)
			}
			seq++
			if int(payload[12]) != seq || int(payload[13]) != wantCount {
				t.Errorf("%d bytes: segment %d is numbered %d of %d, want %d of %d", size, seq, payload[12], payload[13], seq, wantCount)
			}
			return true
		})
		if seq != wantCount {
			t.Errorf("%d bytes: %d APP2 segments, want %d", size, seq, wantCount)
		}
	}
}

func TestProfileCarriedPNG(t *testing.T) {
	profile := testProfile(5000, "RGB ")
	src := pngWithICC(t, pngBytes(t, quadrants(3000, 1500)), profile)
	out, err := intakeOf("image/png", "a.png", src).Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := profileOf(out.MimeType, out.content); !bytes.Equal(got, profile) {
		t.Fatalf("the profile of the copy differs (%d bytes)", len(got))
	}
	// After IHDR, before the first IDAT; the file decodes, so the CRCs hold.
	var order []string
	for i := len(pngSignature); i+8 <= len(out.content); {
		n := int(binary.BigEndian.Uint32(out.content[i:]))
		order = append(order, string(out.content[i+4:i+8]))
		i += 12 + n
	}
	if len(order) < 3 || order[0] != "IHDR" || order[1] != "iCCP" || order[2] == "iCCP" {
		t.Errorf("chunk order %v: want IHDR, iCCP, then the rest", order)
	}
	for _, c := range order[:2] {
		if c == "IDAT" {
			t.Error("the profile comes after IDAT")
		}
	}
	if w, h := dimensionsOf(t, out.content); w != 2560 || h != 1280 {
		t.Errorf("the copy is %d x %d", w, h)
	}
}

// webpWithICC wraps the image chunk(s) of a testdata WebP in a VP8X container
// that carries an ICCP chunk.
func webpWithICC(t *testing.T, name string, profile []byte, alpha bool) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	flags := byte(0x20) // ICC profile present
	if alpha {
		flags |= 0x10
	}
	vp8x := []byte{flags, 0, 0, 0}
	vp8x = append(vp8x, byte(cfg.Width-1), byte((cfg.Width-1)>>8), byte((cfg.Width-1)>>16))
	vp8x = append(vp8x, byte(cfg.Height-1), byte((cfg.Height-1)>>8), byte((cfg.Height-1)>>16))
	chunks := [][]byte{[]byte("VP8X"), vp8x, []byte("ICCP"), profile}
	for i := 12; i+8 <= len(body); {
		size := int(binary.LittleEndian.Uint32(body[i+4:]))
		chunks = append(chunks, body[i:i+4], body[i+8:i+8+size])
		i += 8 + size + size%2
	}
	return webpContainer(chunks...)
}

func TestProfileFromWebP(t *testing.T) {
	profile := testProfile(2001, "RGB ") // odd: the chunk is padded
	opaque := webpWithICC(t, "opaque-lossless.webp", profile, false)
	if got := profileOf("image/webp", opaque); !bytes.Equal(got, profile) {
		t.Fatalf("the ICCP chunk reads as %d bytes, want %d", len(got), len(profile))
	}
	if got := profileOf("image/webp", webpContainer([]byte("VP8L"), []byte{1, 2})); got != nil {
		t.Error("a WebP without ICCP has a profile")
	}

	out, err := intakeOf("image/webp", "a.webp", opaque).Scaled(0)
	if err != nil {
		t.Skipf("the decoder does not take the container, the reader test stands alone: %v", err)
	}
	if out.MimeType != "image/jpeg" || !bytes.Equal(profileOf(out.MimeType, out.content), profile) {
		t.Errorf("opaque WebP: %s, profile carried %v", out.MimeType, bytes.Equal(profileOf(out.MimeType, out.content), profile))
	}
	clear := webpWithICC(t, "transparent.webp", profile, true)
	out, err = intakeOf("image/webp", "a.webp", clear).Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	if out.MimeType != "image/png" || !bytes.Equal(profileOf(out.MimeType, out.content), profile) {
		t.Errorf("transparent WebP: %s, profile carried %v", out.MimeType, bytes.Equal(profileOf(out.MimeType, out.content), profile))
	}
}

func TestProfileSurvivesKeep(t *testing.T) {
	profile := testProfile(3000, "RGB ")
	for mimeType, src := range map[string][]byte{
		"image/jpeg": withOrientation(jpegWithICC(quadrantJPEG(t, 3000, 1500), profile, 65519), 6),
		"image/png":  pngWithChunk(t, pngWithICC(t, pngBytes(t, quadrants(3000, 1500)), profile), "eXIf", tiffBlock(binary.BigEndian, 6), false),
	} {
		out, err := intakeOf(mimeType, "a", src).Scaled(0)
		if err != nil {
			t.Fatal(err)
		}
		stripped, err := StripMetadata(out.content, out.MimeType)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(profileOf(out.MimeType, stripped), profile) {
			t.Errorf("%s: StripMetadata dropped the profile", mimeType)
		}
		if bytes.Contains(stripped, []byte("Exif")) || bytes.Contains(stripped, []byte("eXIf")) {
			t.Errorf("%s: the copy carries EXIF", mimeType)
		}
	}
}

func TestProfileNotCarriedWhenUnfit(t *testing.T) {
	jpg := quadrantJPEG(t, 3000, 1500)
	png := pngBytes(t, quadrants(3000, 1500))
	good := testProfile(1000, "RGB ")
	wrongSize := testProfile(1000, "RGB ")
	binary.BigEndian.PutUint32(wrongSize, 999)
	noSig := testProfile(1000, "RGB ")
	copy(noSig[36:], "xxxx")
	chunk := func(profile []byte) []byte { return jpegWithICC(jpg, profile, 65519) }

	type tc struct {
		name, mime string
		src        []byte
	}
	cases := []tc{
		{"jpeg CMYK", "image/jpeg", chunk(testProfile(1000, "CMYK"))},
		{"jpeg GRAY", "image/jpeg", chunk(testProfile(1000, "GRAY"))},
		{"jpeg no acsp", "image/jpeg", chunk(noSig)},
		{"jpeg wrong declared size", "image/jpeg", chunk(wrongSize)},
		{"jpeg over 4 MiB", "image/jpeg", chunk(testProfile(4<<20+100, "RGB "))},
		{"jpeg sequence gap", "image/jpeg", insertSegments(jpg, app2(1, 3, good[:300]), app2(3, 3, good[300:600]))},
		{"jpeg duplicate", "image/jpeg", insertSegments(jpg, app2(1, 2, good[:300]), app2(1, 2, good[300:]))},
		{"jpeg count disagrees", "image/jpeg", insertSegments(jpg, app2(1, 2, good[:300]), app2(2, 3, good[300:]))},
		{"jpeg missing chunk", "image/jpeg", insertSegments(jpg, app2(1, 2, good[:300]))},
		{"jpeg sequence zero", "image/jpeg", insertSegments(jpg, app2(0, 1, good))},
		{"png CMYK", "image/png", pngWithICC(t, png, testProfile(1000, "CMYK"))},
		{"png method 1", "image/png", pngWithChunk(t, png, "iCCP", iccpPayload("test", 1, deflated(t, good)), false)},
		{"png corrupt zlib", "image/png", pngWithChunk(t, png, "iCCP", iccpPayload("test", 0, []byte("not a zlib stream at all")), false)},
		{"png truncated zlib", "image/png", pngWithChunk(t, png, "iCCP", iccpPayload("test", 0, deflated(t, good)[:20]), false)},
		{"png bomb", "image/png", pngWithChunk(t, png, "iCCP", iccpPayload("test", 0, deflated(t, make([]byte, 64<<20))), false)},
		{"png no name", "image/png", pngWithChunk(t, png, "iCCP", append([]byte{0, 0}, deflated(t, good)...), false)},
		{"png after IDAT", "image/png", pngWithChunk(t, png, "iCCP", iccpPayload("test", 0, deflated(t, good)), true)},
	}
	var still bytes.Buffer
	if err := gif.Encode(&still, image.NewPaletted(image.Rect(0, 0, 3000, 200), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	cases = append(cases, tc{"gif", "image/gif", still.Bytes()})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if p := profileOf(c.mime, c.src); p != nil {
				t.Fatalf("the profile was accepted (%d bytes)", len(p))
			}
			got, err := intakeOf(c.mime, "a", c.src).Scaled(0)
			if err != nil {
				t.Fatalf("Scaled: %v", err)
			}
			if want := oldScaledBytes(t, c.mime, c.src); !bytes.Equal(got.content, want) {
				t.Errorf("the output differs from the pre-change pipeline (%d vs %d bytes)", len(got.content), len(want))
			}
		})
	}
}

func TestScaledOrientationAndProfileTogether(t *testing.T) {
	profile := testProfile(900, "RGB ")
	src := withOrientation(jpegWithICC(quadrantJPEG(t, 3000, 1500), profile, 65519), 6)
	out, err := intakeOf("image/jpeg", "a.jpg", src).Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := dimensionsOf(t, out.content); w != 1280 || h != 2560 {
		t.Errorf("scaled to %d x %d, want 1280 x 2560", w, h)
	}
	if !bytes.Equal(profileOf(out.MimeType, out.content), profile) {
		t.Error("the profile was not carried")
	}
	walkJPEG(out.content, func(marker byte, payload []byte) bool {
		if marker == markerAPP1 {
			t.Error("the copy carries an APP1 segment")
		}
		return true
	})
	if got := orientationOf(out.MimeType, out.content); got != 1 {
		t.Errorf("the copy carries orientation %d", got)
	}
}
