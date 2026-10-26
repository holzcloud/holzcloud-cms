package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
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
