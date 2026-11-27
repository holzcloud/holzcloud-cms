package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// errGIFLimit says gifFrames stopped because the file has more frames, or more
// pixels in its frames, than the caller allows. The counts it returned with it
// are those at the moment it stopped, not the file's totals.
var errGIFLimit = errors.New("the GIF is over its frame or pixel limit")

// gifFrames counts the frames of a GIF and the pixels they cover by walking the
// file's blocks. It exists for finding M6: Oversize has to know whether a GIF
// is animated, the standard library has no frame counter, and gif.DecodeAll
// keeps one paletted bitmap per frame with no limit on their number, so a file
// of a few kilobytes could cost gigabytes.
//
// The walk reads the signature, the logical screen descriptor, the extension
// blocks and the image descriptors, and skips everything else by its length:
// the colour tables, and the compressed data by its sub-block lengths. It
// decodes no LZW, allocates nothing in proportion to a frame or to the number
// of frames, and every read is bounded by the bytes of r.
//
// It stops at once, returning the counts so far and errGIFLimit, when the
// frame count passes maxFrames or the pixel sum passes maxPixels. A frame that
// does not lie inside the logical screen is an error, the rule the standard
// decoder applies too, so the pixel sum of one frame never exceeds the screen's
// area. Any other malformation (a wrong signature, a block introducer that is
// none, a file that ends inside a block) is an error as well; the end of the
// data inside a block is io.ErrUnexpectedEOF. A file that ends without its
// trailer byte is an error too: it was cut.
func gifFrames(r io.Reader, maxFrames int, maxPixels int64) (frames int, pixels int64, err error) {
	br := bufio.NewReaderSize(r, 4096)
	// A short read inside the file is always a cut file.
	cut := func(err error) error {
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	skip := func(n int) error {
		_, err := br.Discard(n)
		return cut(err)
	}
	// skipBlocks passes length-prefixed sub-blocks up to their zero terminator.
	skipBlocks := func() error {
		for {
			n, err := br.ReadByte()
			if err != nil {
				return cut(err)
			}
			if n == 0 {
				return nil
			}
			if err := skip(int(n)); err != nil {
				return err
			}
		}
	}

	var head [13]byte // signature (6) and logical screen descriptor (7)
	if _, err := io.ReadFull(br, head[:]); err != nil {
		return 0, 0, cut(err)
	}
	if s := string(head[:6]); s != "GIF87a" && s != "GIF89a" {
		return 0, 0, fmt.Errorf("gif: not a GIF (signature %q)", s)
	}
	screenW := int(binary.LittleEndian.Uint16(head[6:8]))
	screenH := int(binary.LittleEndian.Uint16(head[8:10]))
	if head[10]&0x80 != 0 { // global colour table
		if err := skip(3 << (head[10]&7 + 1)); err != nil {
			return 0, 0, err
		}
	}

	var d [9]byte // one descriptor buffer for the whole walk: a ReadFull argument escapes
	for {
		b, err := br.ReadByte()
		if err != nil {
			return frames, pixels, cut(err)
		}
		switch b {
		case 0x3B: // trailer
			return frames, pixels, nil
		case 0x21: // extension: a label, then sub-blocks
			if _, err := br.ReadByte(); err != nil {
				return frames, pixels, cut(err)
			}
			if err := skipBlocks(); err != nil {
				return frames, pixels, err
			}
		case 0x2C: // image descriptor
			if _, err := io.ReadFull(br, d[:]); err != nil {
				return frames, pixels, cut(err)
			}
			left := int(binary.LittleEndian.Uint16(d[0:2]))
			top := int(binary.LittleEndian.Uint16(d[2:4]))
			w := int(binary.LittleEndian.Uint16(d[4:6]))
			h := int(binary.LittleEndian.Uint16(d[6:8]))
			if left+w > screenW || top+h > screenH {
				return frames, pixels, fmt.Errorf("gif: frame %d (%dx%d at %d,%d) is outside the %dx%d screen",
					frames+1, w, h, left, top, screenW, screenH)
			}
			frames++
			pixels += int64(w) * int64(h)
			if frames > maxFrames || pixels > maxPixels {
				return frames, pixels, errGIFLimit
			}
			if d[8]&0x80 != 0 { // local colour table
				if err := skip(3 << (d[8]&7 + 1)); err != nil {
					return frames, pixels, err
				}
			}
			if err := skip(1); err != nil { // LZW minimum code size
				return frames, pixels, err
			}
			if err := skipBlocks(); err != nil {
				return frames, pixels, err
			}
		default:
			return frames, pixels, fmt.Errorf("gif: unknown block introducer 0x%02x", b)
		}
	}
}
