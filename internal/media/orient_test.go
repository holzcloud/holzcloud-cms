package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// tiffBlock builds the smallest TIFF block that carries an orientation: the
// header, IFD0 at offset 8 and one 12-byte SHORT entry for tag 0x0112.
func tiffBlock(order binary.ByteOrder, value uint16) []byte {
	return tiffBlockWith(order, 0x0112, 3, 1, uint32(value))
}

// tiffBlockWith lets a test lie about the tag, type and count. The value of a
// SHORT sits in the first two of the four value bytes.
func tiffBlockWith(order binary.ByteOrder, tag, typ uint16, count uint32, value uint32) []byte {
	b := make([]byte, 8+2+12+4)
	if order == binary.BigEndian {
		copy(b, "MM")
	} else {
		copy(b, "II")
	}
	order.PutUint16(b[2:], 42)
	order.PutUint32(b[4:], 8)
	order.PutUint16(b[8:], 1)
	e := b[10:]
	order.PutUint16(e[0:], tag)
	order.PutUint16(e[2:], typ)
	order.PutUint32(e[4:], count)
	if typ == 3 {
		order.PutUint16(e[8:], uint16(value))
	} else {
		order.PutUint32(e[8:], value)
	}
	return b
}

// withAPP1 inserts an APP1 segment with the given payload right after the SOI
// of a JPEG.
func withAPP1(jpg, payload []byte) []byte {
	n := len(payload) + 2
	out := append([]byte{}, jpg[:2]...)
	out = append(out, 0xFF, 0xE1, byte(n>>8), byte(n))
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// withOrientation puts a real Exif block carrying the orientation into a JPEG.
func withOrientation(jpg []byte, o int) []byte {
	return withAPP1(jpg, append([]byte("Exif\x00\x00"), tiffBlock(binary.BigEndian, uint16(o))...))
}

// TestPlainPictureBytesDoNotChange carries the pipeline as it was before the
// orientation and the profile were handled, and holds Scaled to it byte for
// byte for every picture that has neither. It is the proof for B-4.
func TestPlainPictureBytesDoNotChange(t *testing.T) {
	plain := jpegBytes(t, 3000, 1999)
	var still bytes.Buffer
	if err := gif.Encode(&still, image.NewPaletted(image.Rect(0, 0, 3000, 200), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		mime string
		body []byte
	}{
		"jpeg without exif":      {"image/jpeg", plain},
		"jpeg with a text block": {"image/jpeg", withAPP1(plain, []byte("Exif\x00\x00SECRET-CAMERA-DATA"))},
		"jpeg orientation 1":     {"image/jpeg", withOrientation(plain, 1)},
		"png":                    {"image/png", pngBytes(t, gradient(1000, 3000))},
		"still gif":              {"image/gif", still.Bytes()},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := intakeOf(tc.mime, "a", tc.body).Scaled(0)
			if err != nil {
				t.Fatal(err)
			}
			want := oldScaledBytes(t, tc.mime, tc.body)
			if !bytes.Equal(got.content, want) {
				t.Errorf("scaled bytes differ from the pre-change pipeline (%d vs %d bytes)", len(got.content), len(want))
			}
		})
	}
}

// oldScaledBytes is the three steps Scaled consisted of before: decode, scale
// into the ScaledSize of the stored header, encode.
func oldScaledBytes(t *testing.T, mimeType string, body []byte) []byte {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	src, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	w, h := ScaledSize(cfg.Width, cfg.Height)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	var buf bytes.Buffer
	if mimeType == "image/jpeg" {
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
			t.Fatal(err)
		}
	} else if err := png.Encode(&buf, dst); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pixels6 is a 3 x 2 picture with six distinct colours laid out A B C over D E F.
func pixels6() (*image.RGBA, map[byte]color.RGBA) {
	cols := map[byte]color.RGBA{
		'A': {200, 0, 0, 255}, 'B': {0, 200, 0, 255}, 'C': {0, 0, 200, 255},
		'D': {200, 200, 0, 255}, 'E': {0, 200, 200, 255}, 'F': {200, 0, 200, 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i, l := range []byte("ABCDEF") {
		img.SetRGBA(i%3, i/3, cols[l])
	}
	return img, cols
}

func TestOrientTransformTable(t *testing.T) {
	// The rows of the upright image, per orientation, as letters.
	want := map[int][]string{
		1: {"ABC", "DEF"},
		2: {"CBA", "FED"},
		3: {"FED", "CBA"},
		4: {"DEF", "ABC"},
		5: {"AD", "BE", "CF"},
		6: {"DA", "EB", "FC"},
		7: {"FC", "EB", "DA"},
		8: {"CF", "BE", "AD"},
	}
	for o := 1; o <= 8; o++ {
		src, cols := pixels6()
		got := orientRGBA(src, o)
		rows := want[o]
		if b := got.Bounds(); b.Dx() != len(rows[0]) || b.Dy() != len(rows) {
			t.Errorf("orientation %d: %d x %d, want %d x %d", o, b.Dx(), b.Dy(), len(rows[0]), len(rows))
			continue
		}
		for y, row := range rows {
			for x := range row {
				if g := got.RGBAAt(x, y); g != cols[row[x]] {
					t.Errorf("orientation %d at (%d,%d): %v, want %c", o, x, y, g, row[x])
				}
			}
		}
	}
	src, _ := pixels6()
	if orientRGBA(src, 1) != src {
		t.Error("orientation 1 should return the same image, not a copy")
	}
	if orientRGBA(src, 0) != src || orientRGBA(src, 9) != src {
		t.Error("a value outside 1 to 8 should return the same image")
	}
}

func TestTiffOrientationParse(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for v := 1; v <= 8; v++ {
			if got := tiffOrientation(tiffBlock(order, uint16(v))); got != v {
				t.Errorf("%v value %d: got %d", order, v, got)
			}
		}
		for _, v := range []uint16{0, 9, 65535} {
			if got := tiffOrientation(tiffBlock(order, v)); got != 1 {
				t.Errorf("%v value %d: got %d, want 1", order, v, got)
			}
		}
		if got := tiffOrientation(tiffBlockWith(order, 0x0112, 4, 1, 6)); got != 1 {
			t.Errorf("a LONG instead of a SHORT: got %d", got)
		}
		if got := tiffOrientation(tiffBlockWith(order, 0x0112, 3, 2, 6)); got != 1 {
			t.Errorf("a count of two: got %d", got)
		}
		if got := tiffOrientation(tiffBlockWith(order, 0x0110, 3, 1, 6)); got != 1 {
			t.Errorf("another tag: got %d", got)
		}
		// IFD offset past the end, entry count that runs past the end.
		b := tiffBlock(order, 6)
		order.PutUint32(b[4:], 1000)
		if got := tiffOrientation(b); got != 1 {
			t.Errorf("offset past the end: got %d", got)
		}
		b = tiffBlock(order, 6)
		order.PutUint32(b[4:], uint32(len(b)-1))
		if got := tiffOrientation(b); got != 1 {
			t.Errorf("offset in the last byte: got %d", got)
		}
		b = tiffBlock(order, 6)
		order.PutUint16(b[8:], 500)
		if got := tiffOrientation(b[:len(b)-5]); got != 1 {
			t.Errorf("entry count past the end: got %d", got)
		}
		order.PutUint32(b[4:], 0xFFFFFFFF)
		if got := tiffOrientation(b); got != 1 {
			t.Errorf("offset 0xFFFFFFFF: got %d", got)
		}
	}
	for name, b := range map[string][]byte{
		"empty":       {},
		"header only": tiffBlock(binary.BigEndian, 6)[:8],
		"text":        []byte("SECRET-CAMERA-DATA and more text"),
		"wrong magic": append([]byte("MM\x00\x2B"), make([]byte, 20)...),
	} {
		if got := tiffOrientation(b); got != 1 {
			t.Errorf("%s: got %d, want 1", name, got)
		}
	}
}

func TestOrientationFromJPEG(t *testing.T) {
	base := jpegBytes(t, 40, 30)
	exif := append([]byte("Exif\x00\x00"), tiffBlock(binary.LittleEndian, 8)...)
	segment := func(marker byte, payload []byte) []byte {
		n := len(payload) + 2
		return append([]byte{0xFF, marker, byte(n >> 8), byte(n)}, payload...)
	}
	build := func(segments ...[]byte) []byte {
		out := append([]byte{}, base[:2]...)
		for _, s := range segments {
			out = append(out, s...)
		}
		return append(out, base[2:]...)
	}

	jfif := segment(0xE0, []byte{'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0})
	xmp := segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta/>"))
	if got := orientationOf("image/jpeg", build(jfif, segment(0xE1, exif))); got != 8 {
		t.Errorf("Exif after a JFIF segment: %d, want 8", got)
	}
	if got := orientationOf("image/jpeg", build(xmp, segment(0xE1, exif))); got != 8 {
		t.Errorf("Exif after XMP: %d, want 8", got)
	}

	// Only after the scan: not searched.
	late := append(append([]byte{}, base[:len(base)-2]...), segment(0xE1, exif)...)
	late = append(late, 0xFF, 0xD9)
	if got := orientationOf("image/jpeg", late); got != 1 {
		t.Errorf("Exif after SOS: %d, want 1", got)
	}

	if got := orientationOf("image/jpeg", nil); got != 1 {
		t.Errorf("nil: %d", got)
	}
	if got := orientationOf("image/gif", build(segment(0xE1, exif))); got != 1 {
		t.Errorf("gif: %d", got)
	}
}

// quadrants is a w x h picture of four flat colours: A top left, B top right,
// C bottom left, D bottom right.
func quadrants(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case y < h/2 && x < w/2:
				img.SetRGBA(x, y, colA)
			case y < h/2:
				img.SetRGBA(x, y, colB)
			case x < w/2:
				img.SetRGBA(x, y, colC)
			default:
				img.SetRGBA(x, y, colD)
			}
		}
	}
	return img
}

var (
	colA = color.RGBA{220, 30, 30, 255}
	colB = color.RGBA{30, 200, 30, 255}
	colC = color.RGBA{30, 30, 220, 255}
	colD = color.RGBA{230, 230, 30, 255}
)

func quadrantJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, quadrants(w, h), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func near(a, b color.Color, tol int) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) int {
		v := int(x>>8) - int(y>>8)
		if v < 0 {
			v = -v
		}
		return v
	}
	return d(ar, br) <= tol && d(ag, bg) <= tol && d(ab, bb) <= tol
}

func TestScaledAppliesJPEGOrientation(t *testing.T) {
	src := withOrientation(quadrantJPEG(t, 3000, 1500), 6)
	in := intakeOf("image/jpeg", "hoch.jpg", src)
	out, err := in.Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := dimensionsOf(t, out.content); w != 1280 || h != 2560 {
		t.Fatalf("scaled to %d x %d, want 1280 x 2560", w, h)
	}
	img, err := jpeg.Decode(bytes.NewReader(out.content))
	if err != nil {
		t.Fatal(err)
	}
	// Stored AB over CD, turned a quarter clockwise: CA over DB.
	for name, tc := range map[string]struct {
		x, y int
		want color.RGBA
	}{
		"top left": {320, 640, colC}, "top right": {960, 640, colA},
		"bottom left": {320, 1920, colD}, "bottom right": {960, 1920, colB},
	} {
		if got := img.At(tc.x, tc.y); !near(got, tc.want, 12) {
			t.Errorf("%s: %v, want about %v", name, got, tc.want)
		}
	}
	if bytes.Contains(out.content, []byte("Exif")) || bytes.Contains(out.content, []byte{0xFF, 0xE1}) {
		t.Error("the scaled copy carries an EXIF segment")
	}
	if got := orientationOf(out.MimeType, out.content); got != 1 {
		t.Errorf("the copy carries orientation %d", got)
	}
	if !bytes.Equal(in.content, src) {
		t.Error("the source intake was modified")
	}
}

func TestUprightSizeAndOversizeAgree(t *testing.T) {
	base := quadrantJPEG(t, 3000, 1500)
	staging := NewStaging(t.TempDir())
	for o := 1; o <= 8; o++ {
		in := intakeOf("image/jpeg", "a.jpg", withOrientation(base, o))
		wantW, wantH := 3000, 1500
		if o >= 5 {
			wantW, wantH = 1500, 3000
		}
		w, h, ok := in.Oversize(0)
		if !ok || w != wantW || h != wantH {
			t.Errorf("orientation %d: Oversize = %d x %d, %v; want %d x %d, true", o, w, h, ok, wantW, wantH)
		}
		if _, _, ok := in.Oversize(2); ok {
			t.Errorf("orientation %d: offered over the megapixel budget", o)
		}

		token, err := staging.Put(1, 1, []*Intake{in})
		if err != nil {
			t.Fatal(err)
		}
		st, err := staging.Get(1, token, 1)
		if err != nil {
			t.Fatal(err)
		}
		if f := st.Files()[0]; f.Width != wantW || f.Height != wantH {
			t.Errorf("orientation %d: staged %d x %d, want %d x %d", o, f.Width, f.Height, wantW, wantH)
		}

		out, err := in.Scaled(0)
		if err != nil {
			t.Fatal(err)
		}
		sw, sh := ScaledSize(wantW, wantH)
		if gw, gh := dimensionsOf(t, out.content); gw != sw || gh != sh {
			t.Errorf("orientation %d: stored %d x %d, the question says %d x %d", o, gw, gh, sw, sh)
		}
	}

	// 2560 on the upright long side is not over.
	exact := intakeOf("image/jpeg", "a.jpg", withOrientation(quadrantJPEG(t, 2560, 1000), 6))
	if _, _, ok := exact.Oversize(0); ok {
		t.Error("2560 x 1000 turned to 1000 x 2560 is not oversize")
	}
}

func TestScaledSizeIsSymmetric(t *testing.T) {
	sides := []int{1, 2, 100, 1280, 2559, 2560, 2561, 3000, 4000, 10000}
	for _, w := range sides {
		for _, h := range sides {
			aw, ah := ScaledSize(w, h)
			bh, bw := ScaledSize(h, w)
			if aw != bw || ah != bh {
				t.Errorf("ScaledSize(%d, %d) = %d x %d but ScaledSize(%d, %d) = %d x %d", w, h, aw, ah, h, w, bw, bh)
			}
		}
	}
}

// pngWithChunk inserts a chunk (correct CRC) after IHDR, or after the last IDAT.
func pngWithChunk(t *testing.T, data []byte, typ string, payload []byte, afterIDAT bool) []byte {
	t.Helper()
	chunk := make([]byte, 8, 12+len(payload))
	binary.BigEndian.PutUint32(chunk, uint32(len(payload)))
	copy(chunk[4:], typ)
	chunk = append(chunk, payload...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))

	at := len(pngSignature) + 8 + 13 + 4 // after IHDR
	if afterIDAT {
		at = 0
		i := len(pngSignature)
		for i+8 <= len(data) {
			n := int(binary.BigEndian.Uint32(data[i:]))
			i += 12 + n
			if string(data[i-n-8:i-n-4]) == "IDAT" {
				at = i
			}
		}
		if at == 0 {
			t.Fatal("no IDAT chunk")
		}
	}
	out := append([]byte{}, data[:at]...)
	out = append(out, chunk...)
	return append(out, data[at:]...)
}

// webpContainer assembles RIFF, WEBP and the chunks, padding each to even.
func webpContainer(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for i := 0; i+1 < len(chunks); i += 2 {
		typ, payload := chunks[i], chunks[i+1]
		body = append(body, typ...)
		body = binary.LittleEndian.AppendUint32(body, uint32(len(payload)))
		body = append(body, payload...)
		if len(payload)%2 == 1 {
			body = append(body, 0)
		}
	}
	out := []byte("RIFF")
	out = binary.LittleEndian.AppendUint32(out, uint32(len(body)))
	return append(out, body...)
}

func TestOrientationFromPNG(t *testing.T) {
	base := pngBytes(t, gradient(40, 30))
	tiff := tiffBlock(binary.BigEndian, 6)

	if got := orientationOf("image/png", pngWithChunk(t, base, "eXIf", tiff, false)); got != 6 {
		t.Errorf("eXIf after IHDR: %d, want 6", got)
	}
	if got := orientationOf("image/png", pngWithChunk(t, base, "eXIf", tiff, true)); got != 6 {
		t.Errorf("eXIf after IDAT: %d, want 6", got)
	}
	if got := orientationOf("image/png", base); got != 1 {
		t.Errorf("no eXIf: %d, want 1", got)
	}

	// The CRC is not the reader's business.
	badCRC := pngWithChunk(t, base, "eXIf", tiff, false)
	at := len(pngSignature) + 8 + 13 + 4 + 8 + len(tiff)
	badCRC[at] ^= 0xFF
	if got := orientationOf("image/png", badCRC); got != 6 {
		t.Errorf("wrong CRC: %d, want 6", got)
	}

	// A chunk length that runs past the end.
	long := pngWithChunk(t, base, "eXIf", tiff, false)
	binary.BigEndian.PutUint32(long[len(pngSignature)+8+13+4:], 0xFFFFFFF0)
	if got := orientationOf("image/png", long); got != 1 {
		t.Errorf("length past the end: %d, want 1", got)
	}
	// Signature missing.
	noSig := pngWithChunk(t, base, "eXIf", tiff, false)
	noSig[1] = 'X'
	if got := orientationOf("image/png", noSig); got != 1 {
		t.Errorf("no signature: %d, want 1", got)
	}
}

func TestOrientationFromWebP(t *testing.T) {
	tiff := tiffBlock(binary.LittleEndian, 3)
	vp8l := []byte("VP8L")
	pixels := []byte{1, 2, 3, 4, 5}

	if got := orientationOf("image/webp", webpContainer(vp8l, pixels, []byte("EXIF"), tiff)); got != 3 {
		t.Errorf("EXIF chunk: %d, want 3", got)
	}
	prefixed := append([]byte("Exif\x00\x00"), tiff...)
	if got := orientationOf("image/webp", webpContainer(vp8l, pixels, []byte("EXIF"), prefixed)); got != 3 {
		t.Errorf("EXIF chunk with the Exif prefix: %d, want 3", got)
	}
	// An odd-sized chunk before it is padded to even.
	if got := orientationOf("image/webp", webpContainer(vp8l, []byte{1, 2, 3}, []byte("EXIF"), tiff)); got != 3 {
		t.Errorf("after an odd chunk: %d, want 3", got)
	}
	if got := orientationOf("image/webp", webpContainer(vp8l, pixels)); got != 1 {
		t.Errorf("no EXIF: %d, want 1", got)
	}

	// A chunk size that lies, a RIFF size that lies, not RIFF at all.
	lie := webpContainer(vp8l, pixels, []byte("EXIF"), tiff)
	binary.LittleEndian.PutUint32(lie[12+8+len(pixels)+1+4:], 0x00FFFFFF)
	if got := orientationOf("image/webp", lie); got != 1 {
		t.Errorf("chunk size past the end: %d, want 1", got)
	}
	riff := webpContainer(vp8l, pixels, []byte("EXIF"), tiff)
	binary.LittleEndian.PutUint32(riff[4:], 0x7FFFFFFF)
	if got := orientationOf("image/webp", riff); got != 1 {
		t.Errorf("RIFF size past the end: %d, want 1", got)
	}
	if got := orientationOf("image/webp", []byte("RIFX....WEBP")); got != 1 {
		t.Errorf("not RIFF: %d, want 1", got)
	}

	// One real round trip, if the decoder takes the container with the chunk.
	body, err := os.ReadFile("testdata/opaque-lossless.webp")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	withEXIF := append(append([]byte{}, body...), []byte("EXIF")...)
	withEXIF = binary.LittleEndian.AppendUint32(withEXIF, uint32(len(tiff)))
	withEXIF = append(withEXIF, tiffBlock(binary.LittleEndian, 6)...)
	if len(tiff)%2 == 1 {
		withEXIF = append(withEXIF, 0)
	}
	binary.LittleEndian.PutUint32(withEXIF[4:], uint32(len(withEXIF)-8))
	if got := orientationOf("image/webp", withEXIF); got != 6 {
		t.Fatalf("testdata WebP with an added EXIF chunk: %d, want 6", got)
	}
	if _, _, err := image.Decode(bytes.NewReader(withEXIF)); err != nil {
		t.Skipf("the decoder does not take the added chunk, the reader test stands alone: %v", err)
	}
	out, err := intakeOf("image/webp", "a.webp", withEXIF).Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := dimensionsOf(t, out.content); w != cfg.Height || h != cfg.Width {
		t.Errorf("WebP orientation 6: %d x %d, want %d x %d", w, h, cfg.Height, cfg.Width)
	}
}

func TestScaledAppliesEveryOrientation(t *testing.T) {
	base := pngBytes(t, quadrants(3000, 1500))
	// Stored A B over C D; the rows of the upright picture per orientation.
	want := map[int][2]string{
		1: {"AB", "CD"}, 2: {"BA", "DC"}, 3: {"DC", "BA"}, 4: {"CD", "AB"},
		5: {"AC", "BD"}, 6: {"CA", "DB"}, 7: {"DB", "CA"}, 8: {"BD", "AC"},
	}
	cols := map[byte]color.RGBA{'A': colA, 'B': colB, 'C': colC, 'D': colD}
	for o := 1; o <= 8; o++ {
		src := pngWithChunk(t, base, "eXIf", tiffBlock(binary.BigEndian, uint16(o)), false)
		out, err := intakeOf("image/png", "a.png", src).Scaled(0)
		if err != nil {
			t.Fatalf("orientation %d: %v", o, err)
		}
		wantW, wantH := ScaledSize(uprightOf(3000, 1500, o))
		if w, h := dimensionsOf(t, out.content); w != wantW || h != wantH {
			t.Errorf("orientation %d: %d x %d, want %d x %d", o, w, h, wantW, wantH)
			continue
		}
		img, err := png.Decode(bytes.NewReader(out.content))
		if err != nil {
			t.Fatal(err)
		}
		for row, letters := range want[o] {
			for col := range letters {
				x, y := wantW/4+col*wantW/2, wantH/4+row*wantH/2
				if got := color.RGBAModel.Convert(img.At(x, y)); got != cols[letters[col]] {
					t.Errorf("orientation %d quadrant %d,%d: %v, want %c", o, row, col, got, letters[col])
				}
			}
		}
		if bytes.Contains(out.content, []byte("eXIf")) {
			t.Errorf("orientation %d: the copy carries an eXIf chunk", o)
		}
		if got := orientationOf(out.MimeType, out.content); got != 1 {
			t.Errorf("orientation %d: the copy carries orientation %d", o, got)
		}
	}
}

func TestOrientationNeverPanics(t *testing.T) {
	tiff := tiffBlock(binary.BigEndian, 6)
	body, err := os.ReadFile("testdata/opaque-lossless.webp")
	if err != nil {
		t.Fatal(err)
	}
	webp := append(append([]byte{}, body...), []byte("EXIF")...)
	webp = binary.LittleEndian.AppendUint32(webp, uint32(len(tiff)))
	webp = append(webp, tiff...)
	binary.LittleEndian.PutUint32(webp[4:], uint32(len(webp)-8))

	for mimeType, valid := range map[string][]byte{
		"image/jpeg": withOrientation(jpegBytes(t, 16, 16), 6),
		"image/png":  pngWithChunk(t, pngBytes(t, gradient(16, 16)), "eXIf", tiff, false),
		"image/webp": webp,
	} {
		check := func(label string, data []byte) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s %s: panic %v", mimeType, label, r)
				}
			}()
			if o := orientationOf(mimeType, data); o < 1 || o > 8 {
				t.Fatalf("%s %s: orientation %d", mimeType, label, o)
			}
		}
		if got := orientationOf(mimeType, valid); got != 6 {
			t.Fatalf("%s: the valid input reads %d, want 6", mimeType, got)
		}
		for n := 0; n <= len(valid); n++ {
			check("prefix", valid[:n])
		}
		for i := 0; i < len(valid) && i < 400; i++ {
			for _, v := range []byte{0x00, 0xFF} {
				m := append([]byte{}, valid...)
				m[i] = v
				check("overwrite", m)
			}
		}
		check("nil", nil)
		check("empty", []byte{})
	}
}
