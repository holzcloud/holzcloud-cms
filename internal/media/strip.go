package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// StripMetadata removes camera and authoring metadata from image bytes.
//
// The reason this exists: a product photo taken at home carries GPS coordinates
// in its EXIF block, and the media route serves the uploaded bytes verbatim with
// a one-year immutable cache header. Publishing such a photo publishes the
// photographer's home address, silently.
//
// It is deliberately byte surgery, not a decode-and-re-encode: re-encoding a
// quality-95 JPEG at 82 is an irreversible loss of image quality nobody asked
// for. Segments are dropped or copied verbatim, so the pixel data comes out
// bit-identical.
//
// It keeps exactly one thing out of the EXIF block: the orientation number, when
// it is 2 to 8, written fresh into a 26-byte block of its own (see
// orientationTIFF). A phone stores a portrait photo as a landscape bitmap and
// says in that number how to turn it; a stored file that lost the number is
// shown on its side by every browser. The stub is built from the integer alone,
// no byte of the source block is copied, so GPS, device, time, thumbnail and
// maker note stay out. The scaled copy (Intake.Scaled) is turned in its pixels
// and written by encoders that write no EXIF, so there is nothing here for the
// strip to turn into a stub: it never carries a tag, and is never turned twice.
//
// An unknown or unhandled format is returned unchanged rather than refused —
// stripping is a safeguard, and failing an upload over it would be worse than
// the metadata.
func StripMetadata(data []byte, mimeType string) ([]byte, error) {
	switch mimeType {
	case "image/jpeg":
		return stripJPEG(data)
	case "image/png":
		return stripPNG(data)
	case "image/webp":
		return stripWebP(data)
	default:
		// GIF carries little of interest, SVG is text the sanitiser handles and
		// PDF is out of scope.
		return data, nil
	}
}

// JPEG marker bytes.
const (
	markerPrefix = 0xFF
	markerSOI    = 0xD8 // start of image
	markerSOS    = 0xDA // start of scan; entropy-coded data follows
	markerEOI    = 0xD9
	markerAPP0   = 0xE0
	markerAPP1   = 0xE1
	markerAPP2   = 0xE2
	markerAPP13  = 0xED
	markerAPP14  = 0xEE
	markerCOM    = 0xFE
)

// stripJPEG walks the segment chain and copies everything except the segments
// that carry metadata.
func stripJPEG(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != markerPrefix || data[1] != markerSOI {
		return nil, errors.New("not a JPEG")
	}

	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:2])

	i := 2
	exifSeen := false
	for i+3 < len(data) {
		if data[i] != markerPrefix {
			// Fill bytes are legal between segments; skip them.
			i++
			continue
		}
		marker := data[i+1]
		if marker == markerPrefix {
			i++
			continue
		}
		if marker == markerSOS {
			// Everything from here to the end is entropy-coded image data plus
			// the EOI. It is copied verbatim, which is what makes this lossless.
			out.Write(data[i:])
			return out.Bytes(), nil
		}
		if marker == markerEOI {
			out.Write(data[i : i+2])
			return out.Bytes(), nil
		}

		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return nil, fmt.Errorf("malformed JPEG segment at offset %d", i)
		}
		segment := data[i : i+2+length]
		payload := segment[4:]

		// The first Exif segment is the one the reader reads, so it is the one
		// that is claimed, whatever it says. Its place is taken by the stub
		// when it says the picture has to be turned; it is dropped either way.
		if marker == markerAPP1 && !exifSeen && bytes.HasPrefix(payload, exifHeader) {
			exifSeen = true
			if o := tiffOrientation(payload[len(exifHeader):]); o >= 2 && o <= 8 {
				out.Write(orientationAPP1(o))
			}
		}

		if keepJPEGSegment(marker, payload) {
			out.Write(segment)
		}
		i += 2 + length
	}

	return out.Bytes(), nil
}

// keepJPEGSegment decides what survives.
func keepJPEGSegment(marker byte, payload []byte) bool {
	switch marker {
	case markerAPP1:
		// EXIF (GPS, camera, timestamps) and XMP (authoring history).
		return false
	case markerAPP13:
		// Photoshop / IPTC: captions, credits, location.
		return false
	case markerCOM:
		// Free-text comment; some cameras write the owner's name here.
		return false
	case markerAPP2:
		// APP2 is usually an ICC colour profile, which must be kept or the
		// colours shift. It is also used for Flashpix metadata, which must not.
		return bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00"))
	case markerAPP0:
		// JFIF: pixel density. Dropping it makes an image print at the wrong
		// physical size.
		return true
	case markerAPP14:
		// Adobe: declares the colour transform. Dropping it turns a CMYK or
		// YCCK image into garbage.
		return true
	default:
		// Quantisation tables, Huffman tables, frame headers — all essential.
		return true
	}
}

// pngDropChunks are the chunk types that carry metadata rather than image data.
//
// The colour chunks (gAMA, iCCP, sRGB, cHRM) are deliberately absent: dropping
// them changes how the image looks.
var pngDropChunks = map[string]bool{
	"tEXt": true, // plain-text keyword/value, e.g. Author, Comment
	"zTXt": true, // the same, compressed
	"iTXt": true, // the same, UTF-8 — where XMP usually lives
	"eXIf": true, // a full EXIF block, GPS included
	"tIME": true, // last-modification time
}

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}

// stripPNG copies the chunk stream, dropping the metadata chunks. The first
// eXIf chunk is replaced by the orientation stub when it names an orientation
// of 2 to 8; nothing else of it survives.
func stripPNG(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errors.New("not a PNG")
	}

	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(pngSignature)

	i := len(pngSignature)
	exifSeen := false
	for i+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		if length < 0 || i+12+length > len(data) {
			return nil, fmt.Errorf("malformed PNG chunk at offset %d", i)
		}
		chunkType := string(data[i+4 : i+8])
		// 4 length + 4 type + payload + 4 CRC
		chunk := data[i : i+12+length]

		if chunkType == "eXIf" && !exifSeen {
			// The first eXIf chunk is the one the reader reads. The stub stands
			// in its place when it says the picture has to be turned; the chunk
			// itself is dropped either way, as are any later ones.
			exifSeen = true
			if o := tiffOrientation(chunk[8 : 8+length]); o >= 2 && o <= 8 {
				out.Write(orientationChunkPNG(o))
			}
		}
		if !pngDropChunks[chunkType] {
			out.Write(chunk)
		}
		i += 12 + length

		if chunkType == "IEND" {
			break
		}
	}

	return out.Bytes(), nil
}

// stripWebP drops the EXIF and XMP RIFF chunks. The first EXIF chunk is replaced
// by the orientation stub when it names an orientation of 2 to 8 and the
// container is the extended one: an EXIF chunk is legal only behind a VP8X
// chunk that announces it, so the stub is written only when VP8X comes first,
// and the EXIF flag of that chunk is then set. In a simple container a stub
// would make the file invalid, so there the chunk is simply dropped.
func stripWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WEBP")) {
		return nil, errors.New("not a WebP")
	}

	const exifFlag = 0x08
	body := bytes.NewBuffer(make([]byte, 0, len(data)))
	flagsAt := -1 // offset of the VP8X flags byte inside body, once known
	exifSeen := false
	stub := false
	i := 12
	for i+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		// RIFF chunks are padded to an even length.
		padded := size + size%2
		if size < 0 || i+8+padded > len(data) {
			return nil, fmt.Errorf("malformed WebP chunk at offset %d", i)
		}
		chunkType := string(data[i : i+4])
		if chunkType == "VP8X" && i == 12 && size >= 10 {
			flagsAt = body.Len() + 8
		}
		if chunkType == "EXIF" && !exifSeen {
			exifSeen = true
			payload := bytes.TrimPrefix(data[i+8:i+8+size], exifHeader)
			if o := tiffOrientation(payload); o >= 2 && o <= 8 && flagsAt >= 0 {
				body.Write(orientationChunkWebP(o))
				stub = true
			}
		}
		if chunkType != "EXIF" && chunkType != "XMP " {
			body.Write(data[i : i+8+padded])
		}
		i += 8 + padded
	}
	if stub {
		body.Bytes()[flagsAt] |= exifFlag
	}

	out := bytes.NewBuffer(make([]byte, 0, body.Len()+12))
	out.WriteString("RIFF")
	// The RIFF size covers "WEBP" plus the chunks that survived.
	if err := binary.Write(out, binary.LittleEndian, uint32(body.Len()+4)); err != nil {
		return nil, fmt.Errorf("write RIFF size: %w", err)
	}
	out.WriteString("WEBP")
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
