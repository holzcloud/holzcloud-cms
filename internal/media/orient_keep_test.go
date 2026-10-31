package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The strip keeps one number out of the EXIF block: the orientation, 2 to 8,
// written fresh as a 26-byte TIFF block with one entry. These tests hold that
// this is all that survives, and that what did not carry a usable orientation
// comes out exactly as the strip gave it before the stub existed.

// byteOrder is a TIFF byte order that can also append.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// ifdEnt is one entry of an IFD for richEXIF. data is the value; up to four
// bytes it sits in the entry itself, more goes behind the IFD.
type ifdEnt struct {
	tag, typ uint16
	count    uint32
	data     []byte
}

// buildIFD lays out an IFD that will stand at offset base of the TIFF block,
// followed by the data that did not fit into its entries.
func buildIFD(order binary.ByteOrder, base uint32, ents []ifdEnt, next uint32) []byte {
	size := 2 + 12*len(ents) + 4
	out := make([]byte, size)
	order.PutUint16(out, uint16(len(ents)))
	var extra []byte
	for i, e := range ents {
		p := out[2+12*i:]
		order.PutUint16(p[0:], e.tag)
		order.PutUint16(p[2:], e.typ)
		order.PutUint32(p[4:], e.count)
		if len(e.data) <= 4 {
			copy(p[8:12], e.data)
			continue
		}
		order.PutUint32(p[8:], base+uint32(size+len(extra)))
		extra = append(extra, e.data...)
		if len(extra)%2 == 1 {
			extra = append(extra, 0)
		}
	}
	order.PutUint32(out[size-4:], next)
	return append(out, extra...)
}

// richEXIF is a structurally valid multi-IFD TIFF block of the kind a phone
// writes: make, software, orientation (left out when o < 0), the Exif IFD with a
// time and a maker note, the GPS IFD with a position, and IFD1 with a thumbnail.
func richEXIF(order byteOrder, o int) []byte {
	u16 := func(v uint16) []byte { return order.AppendUint16(nil, v) }
	u32 := func(v uint32) []byte { return order.AppendUint32(nil, v) }
	rat := func(v ...uint32) []byte {
		var b []byte
		for _, x := range v {
			b = order.AppendUint32(b, x)
		}
		return b
	}
	thumb := []byte("THUMBNAIL-BYTES")

	exifEnts := []ifdEnt{
		{0x9003, 2, 20, []byte("2024:01:02 03:04:05\x00")},
		{0x927C, 7, 16, []byte("MAKERNOTE-GEHEIM")},
	}
	gpsEnts := []ifdEnt{
		{0x0001, 2, 2, []byte("N\x00")},
		{0x0002, 5, 3, rat(48, 1, 8, 1, 7, 1)},
		{0x0003, 2, 2, []byte("E\x00")},
		{0x0004, 5, 3, rat(11, 1, 34, 1, 30, 1)},
		{0x001C, 7, 16, []byte("GPS-ORT-GEHEIM!!")},
	}
	ifd0 := func(exifOff, gpsOff, next uint32) []byte {
		ents := []ifdEnt{{0x010F, 2, 12, []byte("ACME-Kamera\x00")}}
		if o >= 0 {
			ents = append(ents, ifdEnt{0x0112, 3, 1, u16(uint16(o))})
		}
		ents = append(ents,
			ifdEnt{0x0131, 2, 13, []byte("Software-9.9\x00")},
			ifdEnt{0x8769, 4, 1, u32(exifOff)},
			ifdEnt{0x8825, 4, 1, u32(gpsOff)},
		)
		return buildIFD(order, 8, ents, next)
	}
	ifd1 := func(thumbOff uint32) []byte {
		return buildIFD(order, 0, []ifdEnt{
			{0x0103, 3, 1, u16(6)},
			{0x0201, 4, 1, u32(thumbOff)},
			{0x0202, 4, 1, u32(uint32(len(thumb)))},
		}, 0)
	}

	offExif := uint32(8 + len(ifd0(0, 0, 0)))
	offGPS := offExif + uint32(len(buildIFD(order, 0, exifEnts, 0)))
	offIFD1 := offGPS + uint32(len(buildIFD(order, 0, gpsEnts, 0)))
	offThumb := offIFD1 + uint32(len(ifd1(0)))

	out := make([]byte, 8)
	if order == binary.BigEndian {
		copy(out, "MM")
	} else {
		copy(out, "II")
	}
	order.PutUint16(out[2:], 42)
	order.PutUint32(out[4:], 8)
	out = append(out, ifd0(offExif, offGPS, offIFD1)...)
	out = append(out, buildIFD(order, offExif, exifEnts, 0)...)
	out = append(out, buildIFD(order, offGPS, gpsEnts, 0)...)
	out = append(out, buildIFD(order, offIFD1, []ifdEnt{
		{0x0103, 3, 1, u16(6)},
		{0x0201, 4, 1, u32(offThumb)},
		{0x0202, 4, 1, u32(uint32(len(thumb)))},
	}, 0)...)
	return append(out, thumb...)
}

// richMarkers are strings that sit in richEXIF and must never reach a stored file.
var richMarkers = []string{
	"ACME-Kamera", "Software-9.9", "2024:01:02", "MAKERNOTE-GEHEIM", "GPS-ORT-GEHEIM", "THUMBNAIL-BYTES",
}

// ifdTags lists the tags of IFD0 and the offset of the next IFD. Tests only.
func ifdTags(tiff []byte) (tags []uint16, next uint32) {
	const bad = ^uint32(0)
	if len(tiff) < 10 {
		return nil, bad
	}
	var order binary.ByteOrder = binary.LittleEndian
	if string(tiff[:2]) == "MM" {
		order = binary.BigEndian
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd+2 > len(tiff) {
		return nil, bad
	}
	n := int(order.Uint16(tiff[ifd:]))
	if ifd+2+12*n+4 > len(tiff) {
		return nil, bad
	}
	for i := 0; i < n; i++ {
		tags = append(tags, order.Uint16(tiff[ifd+2+12*i:]))
	}
	return tags, order.Uint32(tiff[ifd+2+12*n:])
}

// wantTIFF, wantAPP1 are the stub spelled out byte by byte, independent of the
// code that writes it.
func wantTIFF(o int) []byte {
	return []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // header, IFD0 at 8
		0x01, 0x00, // one entry
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, byte(o), 0x00, 0x00, 0x00, // orientation, SHORT, 1
		0x00, 0x00, 0x00, 0x00, // no next IFD
	}
}

func wantAPP1(o int) []byte {
	return append([]byte{0xFF, 0xE1, 0x00, 0x22, 'E', 'x', 'i', 'f', 0, 0}, wantTIFF(o)...)
}

// withSegmentsAfterSOI puts segments directly behind the SOI of jpg.
func withSegmentsAfterSOI(jpg []byte, segs ...[]byte) []byte {
	out := append([]byte{}, jpg[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, jpg[2:]...)
}

func exifSegment(tiff []byte) []byte {
	return segment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

var orders = []byteOrder{binary.LittleEndian, binary.BigEndian}

func TestRichEXIFFixtureIsReal(t *testing.T) {
	for _, order := range orders {
		for o := 2; o <= 8; o++ {
			b := richEXIF(order, o)
			for _, m := range richMarkers {
				if !bytes.Contains(b, []byte(m)) {
					t.Errorf("%v: the fixture lacks %q", order, m)
				}
			}
			if got := tiffOrientation(b); got != o {
				t.Errorf("%v: the fixture reads %d, want %d", order, got, o)
			}
		}
		if got := tiffOrientation(richEXIF(order, -1)); got != 1 {
			t.Errorf("a fixture without the tag reads %d", got)
		}
	}
}

func TestOrientationStubBytes(t *testing.T) {
	for o := 2; o <= 8; o++ {
		if got := orientationTIFF(o); !bytes.Equal(got, wantTIFF(o)) {
			t.Errorf("orientationTIFF(%d) = % x", o, got)
		}
		if got := orientationAPP1(o); !bytes.Equal(got, wantAPP1(o)) || len(got) != 36 {
			t.Errorf("orientationAPP1(%d) = % x", o, got)
		}
		if got := tiffOrientation(orientationTIFF(o)); got != o {
			t.Errorf("the stub reads back as %d, want %d", got, o)
		}
		tags, next := ifdTags(orientationTIFF(o))
		if !reflect.DeepEqual(tags, []uint16{0x0112}) || next != 0 {
			t.Errorf("stub IFD0 = %v, next %d", tags, next)
		}
	}
}

func TestStripKeepsOnlyTheOrientationJPEG(t *testing.T) {
	clean := jpegWithSegments(t)
	for _, order := range orders {
		for o := 2; o <= 8; o++ {
			src := withSegmentsAfterSOI(clean,
				exifSegment(richEXIF(order, o)),
				segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>")),
				segment(0xED, []byte("Photoshop 3.0\x00IPTC-GEHEIM")),
				segment(0xFE, []byte("KOMMENTAR-GEHEIM")),
			)
			out, err := StripMetadata(src, "image/jpeg")
			if err != nil {
				t.Fatalf("%v %d: %v", order, o, err)
			}
			want := withSegmentsAfterSOI(clean, wantAPP1(o))
			if !bytes.Equal(out, want) {
				t.Errorf("%v %d: the output is not the clean JPEG plus the stub (%d vs %d bytes)", order, o, len(out), len(want))
			}
			for _, m := range append(richMarkers, "XMP-GEHEIM", "IPTC-GEHEIM", "KOMMENTAR-GEHEIM", "GPS", "xmpmeta") {
				if bytes.Contains(out, []byte(m)) {
					t.Errorf("%v %d: %q survived", order, o, m)
				}
			}
			if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
				t.Errorf("%v %d: no longer decodes: %v", order, o, err)
			}
			if got := orientationOf("image/jpeg", out); got != o {
				t.Errorf("%v %d: reads %d", order, o, got)
			}
			apps := 0
			walkJPEG(out, func(marker byte, payload []byte) bool {
				if marker != markerAPP1 {
					return true
				}
				apps++
				tags, next := ifdTags(payload[6:])
				if !reflect.DeepEqual(tags, []uint16{0x0112}) || next != 0 || len(payload[6:]) != 26 {
					t.Errorf("%v %d: stub has tags %v, next %d, %d bytes", order, o, tags, next, len(payload[6:]))
				}
				return true
			})
			if apps != 1 {
				t.Errorf("%v %d: %d APP1 segments, want 1", order, o, apps)
			}
		}
	}
}

// Characterization: passes against the strip as it was before the stub.
func TestStripLeavesNoStubForOneAbsentOrUnreadable(t *testing.T) {
	clean := jpegWithSegments(t)
	for _, order := range orders {
		cases := map[string][]byte{
			"orientation 1":           tiffBlock(order, 1),
			"rich, no orientation":    richEXIF(order, -1),
			"orientation 0":           tiffBlock(order, 0),
			"orientation 9":           tiffBlock(order, 9),
			"orientation 65535":       tiffBlock(order, 65535),
			"orientation as LONG":     tiffBlockWith(order, 0x0112, 4, 1, 6),
			"orientation count two":   tiffBlockWith(order, 0x0112, 3, 2, 6),
			"text only":               []byte("GPSLatitude 48.137"),
			"truncated TIFF":          tiffBlock(order, 6)[:12],
			"rich with orientation 1": richEXIF(order, 1),
		}
		for name, tiff := range cases {
			out, err := StripMetadata(withSegmentsAfterSOI(clean, exifSegment(tiff)), "image/jpeg")
			if err != nil {
				t.Fatalf("%v %s: %v", order, name, err)
			}
			if !bytes.Equal(out, clean) {
				t.Errorf("%v %s: the output differs from the clean JPEG", order, name)
			}
		}
	}
	out, err := StripMetadata(clean, "image/jpeg")
	if err != nil || !bytes.Equal(out, clean) {
		t.Errorf("no EXIF at all: err %v, equal %v", err, bytes.Equal(out, clean))
	}
}

func TestStripStubTakesTheExifSegmentsPlace(t *testing.T) {
	clean := jpegWithSegments(t)
	jfif := segment(0xE0, []byte("JFIF\x00\x01\x02\x01\x00\x48\x00\x48\x00\x00"))
	icc := segment(0xE2, append([]byte("ICC_PROFILE\x00\x01\x01"), []byte("profildaten")...))
	xmp := segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta/>"))
	com := segment(0xFE, []byte("Kommentar"))

	src := withSegmentsAfterSOI(clean, jfif, exifSegment(richEXIF(binary.LittleEndian, 6)), xmp, icc, com)
	out, err := StripMetadata(src, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if want := withSegmentsAfterSOI(clean, jfif, wantAPP1(6), icc); !bytes.Equal(out, want) {
		t.Error("the stub is not in the place of the Exif segment, between JFIF and ICC")
	}

	two := withSegmentsAfterSOI(clean, exifSegment(tiffBlock(binary.BigEndian, 6)), exifSegment(tiffBlock(binary.BigEndian, 3)))
	out, err = StripMetadata(two, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if want := withSegmentsAfterSOI(clean, wantAPP1(6)); !bytes.Equal(out, want) {
		t.Error("two Exif segments (6, then 3) should give one stub of 6")
	}

	one := withSegmentsAfterSOI(clean, exifSegment(tiffBlock(binary.BigEndian, 1)), exifSegment(tiffBlock(binary.BigEndian, 6)))
	out, err = StripMetadata(one, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, clean) {
		t.Error("the first Exif segment is claimed even at orientation 1: no stub expected")
	}
}

func TestScaledCopyGetsNoStubOnKeep(t *testing.T) {
	src := withOrientation(quadrantJPEG(t, 3000, 1500), 6)
	in := intakeOf("image/jpeg", "hoch.jpg", src)
	scaled, err := in.Scaled(0)
	if err != nil {
		t.Fatal(err)
	}
	keep := func(in *Intake) []byte {
		r, err := PrepareUpload(bytes.NewReader(in.content), in.MimeType, in.limit)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	stored := keep(scaled)
	if got := orientationOf("image/jpeg", stored); got != 1 || bytes.Contains(stored, []byte("Exif")) {
		t.Errorf("the stored scaled copy has orientation %d or an Exif segment", got)
	}
	if w, h := dimensionsOf(t, stored); w != 1280 || h != 2560 {
		t.Errorf("scaled copy %d x %d, want 1280 x 2560", w, h)
	}
	img, err := jpeg.Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatal(err)
	}
	// Turned once: stored AB over CD becomes CA over DB.
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

	original := keep(in)
	if got := orientationOf("image/jpeg", original); got != 6 {
		t.Errorf("the non-scaled copy has orientation %d, want 6", got)
	}
	if w, h := dimensionsOf(t, original); w != 3000 || h != 1500 {
		t.Errorf("the non-scaled copy is %d x %d, the bitmap stays as it was", w, h)
	}
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDimensionsAreUpright(t *testing.T) {
	base := quadrantJPEG(t, 300, 200)
	for name, tc := range map[string]struct {
		body []byte
		w, h int
	}{
		"orientation 6": {withOrientation(base, 6), 200, 300},
		"orientation 8": {withOrientation(base, 8), 200, 300},
		"orientation 3": {withOrientation(base, 3), 300, 200},
		"orientation 1": {withOrientation(base, 1), 300, 200},
		"untagged":      {base, 300, 200},
	} {
		w, h, err := Dimensions(writeTemp(t, "a.jpg", tc.body))
		if err != nil || w != tc.w || h != tc.h {
			t.Errorf("%s: %d x %d, %v; want %d x %d", name, w, h, err, tc.w, tc.h)
		}
	}
	if _, _, err := Dimensions(writeTemp(t, "a.jpg", []byte("kein Bild"))); err == nil {
		t.Error("a file that is not an image should error")
	}
	if _, _, err := Dimensions(filepath.Join(t.TempDir(), "fehlt.jpg")); err == nil {
		t.Error("a missing file should error")
	}
}

// quadrantsAt samples the four quadrants of a w x h picture.
func quadrantsAt(img image.Image, w, h int) [4]color.Color {
	return [4]color.Color{
		img.At(w/4, h/4), img.At(3*w/4, h/4), img.At(w/4, 3*h/4), img.At(3*w/4, 3*h/4),
	}
}

func TestVariantsStandUpright(t *testing.T) {
	base := quadrantJPEG(t, 2000, 1000)
	srcPath := writeTemp(t, "hoch.jpg", withOrientation(base, 6))
	dir := t.TempDir()
	vs, err := MakeVariants(srcPath, dir, "hoch.jpg", "image/jpeg", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) == 0 || vs[0].Label != "thumb" {
		t.Fatalf("variants: %+v", vs)
	}
	if vs[0].Width != 400 || vs[0].Height != 800 {
		t.Errorf("thumb records %d x %d, want 400 x 800", vs[0].Width, vs[0].Height)
	}
	for _, v := range vs {
		if v.Height <= v.Width {
			t.Errorf("%s is not portrait: %d x %d", v.Label, v.Width, v.Height)
		}
		data, err := os.ReadFile(filepath.Join(dir, v.Filename))
		if err != nil {
			t.Fatal(err)
		}
		if w, h := dimensionsOf(t, data); w != v.Width || h != v.Height {
			t.Errorf("%s file is %d x %d, record says %d x %d", v.Label, w, h, v.Width, v.Height)
		}
		if bytes.Contains(data, []byte("Exif")) || orientationOf("image/jpeg", data) != 1 {
			t.Errorf("%s carries a tag", v.Label)
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		// Stored AB over CD, turned a quarter clockwise: CA over DB.
		for i, want := range []color.RGBA{colC, colA, colD, colB} {
			if got := quadrantsAt(img, v.Width, v.Height)[i]; !near(got, want, 14) {
				t.Errorf("%s quadrant %d: %v, want about %v", v.Label, i, got, want)
			}
		}
	}

	// Characterization: an explicit orientation 1 and no block at all give the
	// same variant files, byte for byte.
	var sets [2]map[string][]byte
	for i, body := range [][]byte{withOrientation(base, 1), base} {
		d := t.TempDir()
		vs, err := MakeVariants(writeTemp(t, "q.jpg", body), d, "q.jpg", "image/jpeg", 0)
		if err != nil || len(vs) == 0 {
			t.Fatalf("variants: %v %v", vs, err)
		}
		sets[i] = map[string][]byte{}
		for _, v := range vs {
			b, err := os.ReadFile(filepath.Join(d, v.Filename))
			if err != nil {
				t.Fatal(err)
			}
			sets[i][v.Filename] = b
			if v.Width <= v.Height {
				t.Errorf("an untagged landscape came out %d x %d", v.Width, v.Height)
			}
		}
	}
	if len(sets[0]) != len(sets[1]) {
		t.Fatalf("%d vs %d variants", len(sets[0]), len(sets[1]))
	}
	for name, b := range sets[0] {
		if !bytes.Equal(b, sets[1][name]) {
			t.Errorf("%s differs between orientation 1 and no block", name)
		}
	}
}

// hostileStrip throws truncations, overwritten bytes and lying lengths at
// StripMetadata. Nothing may panic, and a success is never much longer than
// its input (the stub is a few bytes bigger than the smallest block that can
// produce it).
func hostileStrip(t *testing.T, mimeType string, valid []byte, extra map[string][]byte) {
	t.Helper()
	check := func(label string, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s %s: panic %v", mimeType, label, r)
			}
		}()
		out, err := StripMetadata(data, mimeType)
		if err == nil && len(out) > len(data)+8 {
			t.Fatalf("%s %s: %d bytes in, %d bytes out", mimeType, label, len(data), len(out))
		}
	}
	for n := 0; n <= len(valid); n++ {
		check("prefix", valid[:n])
	}
	over := func(i int) {
		for _, v := range []byte{0x00, 0xFF, 0x7F} {
			m := append([]byte{}, valid...)
			m[i] = v
			check("overwrite", m)
		}
	}
	for i := 0; i < len(valid) && i < 200; i++ {
		over(i)
	}
	for i := len(valid) - 40; i < len(valid); i++ {
		if i >= 0 {
			over(i)
		}
	}
	for label, data := range extra {
		check(label, data)
	}
	check("nil", nil)
}

func TestStripJPEGHostileInput(t *testing.T) {
	clean := jpegWithSegments(t)
	valid := withSegmentsAfterSOI(clean, exifSegment(richEXIF(binary.LittleEndian, 6)))
	if got := orientationOf("image/jpeg", valid); got != 6 {
		t.Fatalf("the fixture reads %d", got)
	}

	var stacked [][]byte
	for i := 0; i < 40; i++ {
		stacked = append(stacked, exifSegment(tiffBlock(binary.BigEndian, 6)))
	}
	long := append([]byte{}, valid...)
	long[4], long[5] = 0xFF, 0xFF // the Exif segment claims 0xFFFF bytes

	hostileStrip(t, "image/jpeg", valid, map[string][]byte{
		"stacked": withSegmentsAfterSOI(clean, stacked...),
		"long":    long,
	})
}

// wantPNGChunk, wantWebPChunk are the stub chunks spelled out, the CRC computed
// here and not by the code under test.
func wantPNGChunk(o int) []byte {
	b := append([]byte{0, 0, 0, 26}, "eXIf"...)
	b = append(b, wantTIFF(o)...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

func wantWebPChunk(o int) []byte {
	return append([]byte("EXIF\x1a\x00\x00\x00"), wantTIFF(o)...)
}

func TestOrientationChunkBytes(t *testing.T) {
	for o := 2; o <= 8; o++ {
		if got := orientationChunkPNG(o); !bytes.Equal(got, wantPNGChunk(o)) || len(got) != 38 {
			t.Errorf("orientationChunkPNG(%d) = % x", o, got)
		}
		if got := orientationChunkWebP(o); !bytes.Equal(got, wantWebPChunk(o)) || len(got) != 34 {
			t.Errorf("orientationChunkWebP(%d) = % x", o, got)
		}
	}
}

// pngAfterIHDR is the offset behind the IHDR chunk of a PNG.
const pngAfterIHDR = 8 + 8 + 13 + 4

func TestStripKeepsOnlyTheOrientationPNG(t *testing.T) {
	clean := pngBytes(t, quadrants(40, 30))
	plain, err := png.Decode(bytes.NewReader(clean))
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range orders {
		for o := 2; o <= 8; o++ {
			src := clean
			src = pngWithChunk(t, src, "tIME", []byte{7, 232, 1, 2, 3, 4, 5}, false)
			src = pngWithChunk(t, src, "iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>"), false)
			src = pngWithChunk(t, src, "tEXt", []byte("Author\x00KOMMENTAR-GEHEIM"), false)
			src = pngWithChunk(t, src, "eXIf", richEXIF(order, o), false)
			out, err := StripMetadata(src, "image/png")
			if err != nil {
				t.Fatalf("%v %d: %v", order, o, err)
			}
			want := append(append(append([]byte{}, clean[:pngAfterIHDR]...), wantPNGChunk(o)...), clean[pngAfterIHDR:]...)
			if !bytes.Equal(out, want) {
				t.Errorf("%v %d: the output is not the clean PNG plus the stub", order, o)
			}
			for _, m := range append(richMarkers, "XMP-GEHEIM", "KOMMENTAR-GEHEIM", "tIME", "iTXt", "tEXt") {
				if bytes.Contains(out, []byte(m)) {
					t.Errorf("%v %d: %q survived", order, o, m)
				}
			}
			got, err := png.Decode(bytes.NewReader(out))
			if err != nil || got.Bounds() != plain.Bounds() {
				t.Fatalf("%v %d: decode %v, bounds %v", order, o, err, got.Bounds())
			}
			if orientationOf("image/png", out) != o {
				t.Errorf("%v %d: reads %d", order, o, orientationOf("image/png", out))
			}
		}
	}

	// The chunk stands where the source had it: behind the pixel data.
	late := pngWithChunk(t, clean, "eXIf", richEXIF(binary.BigEndian, 6), true)
	out, err := StripMetadata(late, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if want := pngWithChunk(t, clean, "eXIf", wantTIFF(6), true); !bytes.Equal(out, want) {
		t.Error("an eXIf chunk behind the IDAT run should be replaced there")
	}

	// Two chunks: the first is claimed.
	two := pngWithChunk(t, clean, "eXIf", tiffBlock(binary.BigEndian, 3), false)
	two = pngWithChunk(t, two, "eXIf", tiffBlock(binary.BigEndian, 6), false)
	out, err = StripMetadata(two, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if want := pngWithChunk(t, clean, "eXIf", wantTIFF(6), false); !bytes.Equal(out, want) {
		t.Error("two eXIf chunks (6, then 3) should give one stub of 6")
	}
}

// vp8lOf takes the lossless image chunk out of the testdata WebP and returns it
// with the bounds its header declares.
func vp8lOf(t *testing.T) (payload []byte, w, h int) {
	t.Helper()
	body, err := os.ReadFile("testdata/opaque-lossless.webp")
	if err != nil {
		t.Fatal(err)
	}
	if string(body[12:16]) != "VP8L" {
		t.Fatalf("the chunk at offset 12 is %q, not VP8L", body[12:16])
	}
	size := int(binary.LittleEndian.Uint32(body[16:20]))
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return body[20 : 20+size], cfg.Width, cfg.Height
}

func vp8xPayload(flags byte, w, h int) []byte {
	b := []byte{flags, 0, 0, 0}
	b = append(b, byte(w-1), byte((w-1)>>8), byte((w-1)>>16))
	return append(b, byte(h-1), byte((h-1)>>8), byte((h-1)>>16))
}

func TestStripKeepsOnlyTheOrientationWebP(t *testing.T) {
	vp8l, w, h := vp8lOf(t)
	for _, order := range orders {
		for _, prefix := range []bool{false, true} {
			for _, flags := range []byte{0x00, 0x08} {
				for o := 2; o <= 8; o++ {
					exif := richEXIF(order, o)
					if prefix {
						exif = append([]byte("Exif\x00\x00"), exif...)
					}
					src := webpContainer(
						[]byte("VP8X"), vp8xPayload(flags, w, h),
						[]byte("VP8L"), vp8l,
						[]byte("EXIF"), exif,
						[]byte("XMP "), []byte("<x:xmpmeta>XMP-GEHEIM</x:xmpmeta>"),
					)
					out, err := StripMetadata(src, "image/webp")
					if err != nil {
						t.Fatal(err)
					}
					want := webpContainer(
						[]byte("VP8X"), vp8xPayload(0x08, w, h),
						[]byte("VP8L"), vp8l,
						[]byte("EXIF"), wantTIFF(o),
					)
					if !bytes.Equal(out, want) {
						t.Errorf("%v prefix %v flags %#x o %d: the output is not the clean container plus the stub", order, prefix, flags, o)
						continue
					}
					if got := binary.LittleEndian.Uint32(out[4:8]); int(got) != len(out)-8 {
						t.Errorf("RIFF size %d, want %d", got, len(out)-8)
					}
					for _, m := range append(richMarkers, "XMP-GEHEIM") {
						if bytes.Contains(out, []byte(m)) {
							t.Errorf("%q survived", m)
						}
					}
					cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
					if err != nil || cfg.Width != w || cfg.Height != h {
						t.Errorf("DecodeConfig %v %v", cfg, err)
					}
					if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
						t.Errorf("Decode: %v", err)
					}
					if got := orientationOf("image/webp", out); got != o {
						t.Errorf("reads %d, want %d", got, o)
					}
				}
			}
		}
	}
}

// Characterization: passes against the strip as it was before the stub.
func TestStripLeavesNoStubPNGWebP(t *testing.T) {
	clean := pngBytes(t, quadrants(40, 30))
	vp8l, w, h := vp8lOf(t)
	for _, order := range orders {
		cases := map[string][]byte{
			"orientation 1":        tiffBlock(order, 1),
			"rich, no orientation": richEXIF(order, -1),
			"orientation 9":        tiffBlock(order, 9),
			"text only":            []byte("GPSLatitude 48.137"),
			"truncated TIFF":       tiffBlock(order, 6)[:12],
		}
		for name, tiff := range cases {
			src := pngWithChunk(t, pngWithChunk(t, clean, "tEXt", []byte("a\x00b"), false), "eXIf", tiff, false)
			out, err := StripMetadata(src, "image/png")
			if err != nil || !bytes.Equal(out, clean) {
				t.Errorf("png %v %s: err %v, equal %v", order, name, err, bytes.Equal(out, clean))
			}

			src = webpContainer(
				[]byte("VP8X"), vp8xPayload(0x10, w, h),
				[]byte("VP8L"), vp8l,
				[]byte("EXIF"), tiff,
				[]byte("XMP "), []byte("<x:xmpmeta/>"),
			)
			out, err = StripMetadata(src, "image/webp")
			want := webpContainer([]byte("VP8X"), vp8xPayload(0x10, w, h), []byte("VP8L"), vp8l)
			if err != nil || !bytes.Equal(out, want) {
				t.Errorf("webp %v %s: err %v, equal %v", order, name, err, bytes.Equal(out, want))
			}
		}
	}

	// A simple container has no VP8X to announce an EXIF chunk: a stub there
	// would make the file invalid.
	simple := webpContainer([]byte("VP8L"), vp8l, []byte("EXIF"), tiffBlock(binary.BigEndian, 6))
	out, err := StripMetadata(simple, "image/webp")
	if want := webpContainer([]byte("VP8L"), vp8l); err != nil || !bytes.Equal(out, want) {
		t.Errorf("simple container: err %v, equal %v", err, bytes.Equal(out, want))
	}
}

func TestVariantsStandUprightPNG(t *testing.T) {
	src := pngWithChunk(t, pngBytes(t, quadrants(1000, 500)), "eXIf", tiffBlock(binary.BigEndian, 8), false)
	path := writeTemp(t, "hoch.png", src)
	if w, h, err := Dimensions(path); err != nil || w != 500 || h != 1000 {
		t.Errorf("Dimensions = %d x %d, %v; want 500 x 1000", w, h, err)
	}
	dir := t.TempDir()
	vs, err := MakeVariants(path, dir, "hoch.png", "image/png", 0)
	if err != nil || len(vs) == 0 {
		t.Fatalf("variants: %v, %v", vs, err)
	}
	thumb := vs[0]
	if thumb.Width != 400 || thumb.Height != 800 {
		t.Errorf("thumb records %d x %d, want 400 x 800", thumb.Width, thumb.Height)
	}
	data, err := os.ReadFile(filepath.Join(dir, thumb.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("eXIf")) || bytes.Contains(data, []byte("Exif")) {
		t.Error("the thumbnail carries a tag")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// Stored AB over CD, orientation 8: BD over AC.
	for i, want := range []color.RGBA{colB, colD, colA, colC} {
		if got := quadrantsAt(img, thumb.Width, thumb.Height)[i]; !near(got, want, 14) {
			t.Errorf("quadrant %d: %v, want about %v", i, got, want)
		}
	}
}

func TestStripHostileInputPNGWebP(t *testing.T) {
	clean := pngBytes(t, quadrants(16, 16))
	valid := pngWithChunk(t, clean, "eXIf", richEXIF(binary.LittleEndian, 6), false)
	if orientationOf("image/png", valid) != 6 {
		t.Fatal("the PNG fixture does not read 6")
	}
	stacked := clean
	for i := 0; i < 40; i++ {
		stacked = pngWithChunk(t, stacked, "eXIf", tiffBlock(binary.BigEndian, 6), false)
	}
	long := append([]byte{}, valid...)
	binary.BigEndian.PutUint32(long[pngAfterIHDR:], 0xFFFFFFF0)
	hostileStrip(t, "image/png", valid, map[string][]byte{"stacked": stacked, "long": long})

	vp8l, w, h := vp8lOf(t)
	webp := webpContainer(
		[]byte("VP8X"), vp8xPayload(0, w, h),
		[]byte("VP8L"), vp8l,
		[]byte("EXIF"), richEXIF(binary.LittleEndian, 6),
	)
	if orientationOf("image/webp", webp) != 6 {
		t.Fatal("the WebP fixture does not read 6")
	}
	var chunks [][]byte
	chunks = append(chunks, []byte("VP8X"), vp8xPayload(0, w, h))
	for i := 0; i < 40; i++ {
		chunks = append(chunks, []byte("EXIF"), tiffBlock(binary.BigEndian, 6))
	}
	riffLie := append([]byte{}, webp...)
	binary.LittleEndian.PutUint32(riffLie[4:], 0x7FFFFFFF)
	chunkLie := append([]byte{}, webp...)
	binary.LittleEndian.PutUint32(chunkLie[bytes.LastIndex(chunkLie, []byte("EXIF"))+4:], 0x00FFFFFF)
	hostileStrip(t, "image/webp", webp, map[string][]byte{
		"stacked":   webpContainer(chunks...),
		"riff lies": riffLie,
		"size lies": chunkLie,
	})
}

// quadrantLetters are the rows of the upright picture per orientation, for a
// picture stored as A B over C D.
var quadrantLetters = map[int][2]string{
	1: {"AB", "CD"}, 2: {"BA", "DC"}, 3: {"DC", "BA"}, 4: {"CD", "AB"},
	5: {"AC", "BD"}, 6: {"CA", "DB"}, 7: {"DB", "CA"}, 8: {"BD", "AC"},
}

var letterColour = map[byte]color.RGBA{'A': colA, 'B': colB, 'C': colC, 'D': colD}

// checkQuadrants samples the middle of each quadrant of img against rows.
func checkQuadrants(t *testing.T, label string, img image.Image, rows [2]string) {
	t.Helper()
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			x, y := b.Min.X+w/4+col*w/2, b.Min.Y+h/4+row*h/2
			if got := img.At(x, y); !near(got, letterColour[rows[row][col]], 14) {
				t.Errorf("%s quadrant %d,%d: %v, want about %c", label, row, col, got, rows[row][col])
			}
		}
	}
}

func TestCropStandsUpright(t *testing.T) {
	dir := t.TempDir()
	served := filepath.Join(dir, "hoch.jpg")
	if err := os.WriteFile(served, withOrientation(quadrantJPEG(t, 1000, 500), 6), 0o644); err != nil {
		t.Fatal(err)
	}
	read := func() (image.Image, []byte) {
		data, err := os.ReadFile(served)
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img, data
	}

	w, h, err := ApplyCrop(dir, "hoch.jpg", "image/jpeg", Crop{}, 0)
	if err != nil || w != 500 || h != 1000 {
		t.Fatalf("zero crop: %d x %d, %v; want 500 x 1000", w, h, err)
	}
	img, data := read()
	if b := img.Bounds(); b.Dx() != 500 || b.Dy() != 1000 {
		t.Errorf("served file is %d x %d", b.Dx(), b.Dy())
	}
	checkQuadrants(t, "zero crop", img, quadrantLetters[6])
	if bytes.Contains(data, []byte("Exif")) || orientationOf("image/jpeg", data) != 1 {
		t.Error("the cropped picture carries a tag")
	}

	// The editor's turn is applied on top of the upright picture: C A over D B
	// turned a quarter clockwise is D C over B A.
	w, h, err = ApplyCrop(dir, "hoch.jpg", "image/jpeg", Crop{Rotation: 90}, 0)
	if err != nil || w != 1000 || h != 500 {
		t.Fatalf("rotation 90: %d x %d, %v; want 1000 x 500", w, h, err)
	}
	img, _ = read()
	checkQuadrants(t, "rotation 90", img, [2]string{"DC", "BA"})

	w, h, err = ApplyCrop(dir, "hoch.jpg", "image/jpeg", Crop{Ratio: "1-1"}, 0)
	if err != nil || w != 500 || h != 500 {
		t.Errorf("square crop: %d x %d, %v; want 500 x 500", w, h, err)
	}
}

func TestOriginalKeepsTheTagAndRestores(t *testing.T) {
	dir := t.TempDir()
	upload := withOrientation(quadrantJPEG(t, 1000, 500), 6)
	served := filepath.Join(dir, "hoch.jpg")
	if err := os.WriteFile(served, upload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApplyCrop(dir, "hoch.jpg", "image/jpeg", Crop{Ratio: "1-1"}, 0); err != nil {
		t.Fatal(err)
	}
	if !HasOriginal(dir, "hoch.jpg") {
		t.Fatal("no original after the crop")
	}
	orig, err := os.ReadFile(filepath.Join(dir, SourceName("hoch.jpg")))
	if err != nil {
		t.Fatal(err)
	}
	if orientationOf("image/jpeg", orig) != 6 {
		t.Error("the original lost its tag")
	}
	w, h, err := RestoreOriginal(dir, "hoch.jpg")
	if err != nil || w != 500 || h != 1000 {
		t.Errorf("restore: %d x %d, %v; want 500 x 1000", w, h, err)
	}
	if got, err := os.ReadFile(served); err != nil || !bytes.Equal(got, upload) {
		t.Errorf("the restored file is not the upload (err %v)", err)
	}
}

func TestUprightImage(t *testing.T) {
	stored := quadrants(64, 32)
	nrgba := image.NewNRGBA(stored.Bounds())
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			c := stored.RGBAAt(x, y)
			nrgba.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stored, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	ycc, err := jpeg.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ycc.(*image.YCbCr); !ok {
		t.Fatalf("the JPEG decoded to %T", ycc)
	}
	sources := map[string]image.Image{"YCbCr": ycc, "NRGBA": nrgba, "RGBA": stored}

	for name, src := range sources {
		for _, o := range []int{0, 1, 9, -3} {
			if got := uprightImage(src, o); got != src {
				t.Errorf("%s orientation %d should return the same image", name, o)
			}
		}
		for o := 2; o <= 8; o++ {
			got := uprightImage(src, o)
			wantW, wantH := uprightOf(64, 32, o)
			if b := got.Bounds(); b.Dx() != wantW || b.Dy() != wantH {
				t.Errorf("%s %d: %d x %d, want %d x %d", name, o, b.Dx(), b.Dy(), wantW, wantH)
				continue
			}
			checkQuadrants(t, name+" "+string(rune('0'+o)), got, quadrantLetters[o])
		}
	}
}
