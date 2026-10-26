package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"os"
)

// EXIF orientation, the one piece of camera data a scaled copy still needs.
//
// A phone stores a portrait photo as a landscape bitmap and writes in the EXIF
// block how to turn it. The scaled copy carries no EXIF at all, so the turn has
// to be made in the pixels, once, and the copy then says nothing about it: a
// browser that found a tag on a bitmap that is already upright would turn it a
// second time.
//
// Every reader here returns 1 ("as stored") for what it cannot read. Nothing in
// this file returns an error and nothing may panic on the bytes of a stranger:
// every length and offset is checked against the slice before it is used.

// exifHeader opens the payload of a JPEG APP1 segment that holds Exif: the four
// letters and two zero bytes. The reader and the writer share this one value.
var exifHeader = []byte("Exif\x00\x00")

// orientationOf is the EXIF orientation, 1 to 8, of an image file; 1 for a
// format without EXIF and for anything that cannot be read.
func orientationOf(mimeType string, data []byte) int {
	switch mimeType {
	case "image/jpeg":
		return jpegOrientation(data)
	case "image/png":
		return pngOrientation(data)
	case "image/webp":
		return webpOrientation(data)
	}
	// GIF carries no EXIF.
	return 1
}

// pngOrientation reads the first eXIf chunk, which holds a bare TIFF block. The
// specification allows it anywhere between IHDR and IEND outside the IDAT run,
// so the whole chunk chain is walked. The CRC is not checked: the decoder has
// accepted the file already, and a reader that refused a chunk over its CRC
// would skip a tag every browser reads.
func pngOrientation(data []byte) int {
	if !bytes.HasPrefix(data, pngSignature) {
		return 1
	}
	i := len(pngSignature)
	for i+8 <= len(data) {
		length := uint64(binary.BigEndian.Uint32(data[i : i+4]))
		if uint64(i)+12+length > uint64(len(data)) {
			return 1
		}
		payload := data[i+8 : i+8+int(length)]
		switch string(data[i+4 : i+8]) {
		case "eXIf":
			return tiffOrientation(payload)
		case "IEND":
			return 1
		}
		i += 12 + int(length)
	}
	return 1
}

// webpOrientation reads the first EXIF chunk of a RIFF/WEBP container. Some
// writers put the six bytes "Exif" and two zeros in front of the TIFF block;
// that is tolerated.
func webpOrientation(data []byte) int {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 1
	}
	if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 > uint64(len(data)) {
		return 1
	}
	i := 12
	for i+8 <= len(data) {
		size := uint64(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if uint64(i)+8+size > uint64(len(data)) {
			return 1
		}
		if string(data[i:i+4]) == "EXIF" {
			payload := bytes.TrimPrefix(data[i+8:i+8+int(size)], []byte("Exif\x00\x00"))
			return tiffOrientation(payload)
		}
		i += 8 + int(size) + int(size%2)
	}
	return 1
}

// jpegOrientation looks in the APP1 segments before the scan for the one that
// holds Exif; an APP1 with something else in it (XMP) is passed over.
func jpegOrientation(data []byte) int {
	o := 1
	walkJPEG(data, func(marker byte, payload []byte) bool {
		if marker != markerAPP1 || !bytes.HasPrefix(payload, exifHeader) {
			return true
		}
		o = tiffOrientation(payload[len(exifHeader):])
		return false
	})
	return o
}

// walkJPEG calls fn for every segment between SOI and the start of the scan,
// in order, with the marker and the payload (without the length field). fn
// returns false to stop. A malformed chain just ends the walk.
func walkJPEG(data []byte, fn func(marker byte, payload []byte) bool) {
	if len(data) < 4 || data[0] != markerPrefix || data[1] != markerSOI {
		return
	}
	i := 2
	for i+1 < len(data) {
		if data[i] != markerPrefix {
			i++
			continue
		}
		marker := data[i+1]
		switch {
		case marker == markerPrefix:
			i++
			continue
		case marker == markerSOS || marker == markerEOI:
			return
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			// Markers that stand alone, without a length.
			i += 2
			continue
		}
		if i+4 > len(data) {
			return
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return
		}
		if !fn(marker, data[i+4:i+2+length]) {
			return
		}
		i += 2 + length
	}
}

// tiffOrientation reads tag 0x0112 from IFD0 of a TIFF block, the structure
// inside every EXIF block. 1 unless the tag is there, a SHORT, with a count of
// one and a value from 1 to 8.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 1
	}
	ifd := uint64(order.Uint32(tiff[4:8]))
	if ifd+2 > uint64(len(tiff)) {
		return 1
	}
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	pos := int(ifd) + 2
	for i := 0; i < count && pos+12 <= len(tiff); i++ {
		e := tiff[pos : pos+12]
		pos += 12
		if order.Uint16(e[0:2]) != 0x0112 {
			continue
		}
		if order.Uint16(e[2:4]) != 3 || order.Uint32(e[4:8]) != 1 {
			return 1
		}
		if v := int(order.Uint16(e[8:10])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}

// swapsAxes is true for the orientations that turn the picture by a quarter:
// 5 to 8. Width and height trade places.
func swapsAxes(o int) bool { return o >= 5 && o <= 8 }

// uprightOf is the size a picture of width by height has once it stands right.
func uprightOf(width, height, o int) (int, int) {
	if swapsAxes(o) {
		return height, width
	}
	return width, height
}

// orientRGBA returns src turned so that it stands upright for orientation o.
// 1 (and anything outside 1 to 8) returns src itself, not a copy: that is what
// keeps a picture without an orientation byte for byte what it was.
//
// The values: 2 mirror left-right, 3 turn 180 degrees, 4 mirror top-bottom,
// 5 transpose, 6 turn 90 degrees clockwise, 7 transverse, 8 turn 90 degrees
// counter-clockwise. A pure permutation of the pixels, four bytes at a time.
func orientRGBA(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	uw, uh := uprightOf(w, h, o)
	dst := image.NewRGBA(image.Rect(0, 0, uw, uh))
	for y := 0; y < uh; y++ {
		for x := 0; x < uw; x++ {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			si := src.PixOffset(b.Min.X+sx, b.Min.Y+sy)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// The stub. When a picture is stored without being turned in its pixels, the
// strip keeps one thing out of its EXIF block: the orientation number, written
// fresh. These functions take the number 2 to 8 that the reader returned and
// nothing else, so no byte of the source block can reach the output through
// them. The caller has restricted o already; they do not clamp.

// orientationTIFF is the smallest TIFF block that carries an orientation: 26
// bytes, little-endian, with exactly one entry in IFD0 and no next IFD.
func orientationTIFF(o int) []byte {
	le := binary.LittleEndian
	b := make([]byte, 0, 26)
	b = append(b, 'I', 'I')           // byte order
	b = le.AppendUint16(b, 42)        // magic
	b = le.AppendUint32(b, 8)         // offset of IFD0
	b = le.AppendUint16(b, 1)         // entry count
	b = le.AppendUint16(b, 0x0112)    // tag: Orientation
	b = le.AppendUint16(b, 3)         // type: SHORT
	b = le.AppendUint32(b, 1)         // count
	b = le.AppendUint16(b, uint16(o)) // value, in the low two bytes of the field
	b = append(b, 0, 0)               // rest of the value field
	b = le.AppendUint32(b, 0)         // offset of the next IFD: none
	return b
}

// orientationAPP1 is the JPEG segment around the block: marker, length, the
// Exif header and the block. 36 bytes.
func orientationAPP1(o int) []byte {
	block := orientationTIFF(o)
	b := []byte{markerPrefix, markerAPP1}
	b = binary.BigEndian.AppendUint16(b, uint16(2+len(exifHeader)+len(block)))
	b = append(b, exifHeader...)
	return append(b, block...)
}

// look reads a stored picture's header and its orientation. The whole file is
// read, not only the header: the EXIF chunk of a PNG or a WebP may follow the
// pixels, so only the whole file can say how the picture is turned.
func look(path string) (width, height, o int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 1, fmt.Errorf("open image: %w", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, 1, fmt.Errorf("read image header: %w", err)
	}
	return cfg.Width, cfg.Height, orientationOf("image/"+format, data), nil
}

// uprightSize is the size this picture has when it is looked at: the stored
// size of the header, with width and height swapped for a quarter turn. Header
// only, the pixels are not decoded.
func (in *Intake) uprightSize() (width, height int, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(in.content))
	if err != nil {
		return 0, 0, err
	}
	width, height = uprightOf(cfg.Width, cfg.Height, orientationOf(in.MimeType, in.content))
	return width, height, nil
}
