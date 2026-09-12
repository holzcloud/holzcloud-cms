package media

import "encoding/binary"

// Take the metadata out of an MP4 without moving the file.
//
// A phone video carries the same particulars as a phone photo: where it was
// taken, on what device, when. For photos they are removed on upload — the same
// rule has to hold for a video, or the farm shop publishes the coordinates of
// the barn.
//
// The catch: an MP4 refers to itself with absolute byte positions (stco/co64).
// Cutting bytes out shifts everything behind them and makes the file
// unplayable. So nothing is removed but **overwritten**: the box keeps its
// length and is afterwards called "free" — an area every player skips. Same
// length, same positions, no particulars any more.

// stripBoxes are the boxes that carry metadata rather than picture or sound.
//
// udta holds the user data — the location box ©xyz among it. meta holds the
// same in a different wrapping. Both appear inside moov, and both are optional
// for playback.
var stripBoxes = map[string]bool{"udta": true, "meta": true}

// StripMP4Metadata blanks the metadata boxes of an MP4 in place.
//
// It returns the same slice, modified. A file that is not an MP4, or one whose
// box structure does not add up, comes back untouched: this is a cleaner, not a
// validator, and a video that plays is worth more than a video that was tidied.
func StripMP4Metadata(data []byte) []byte {
	walkBoxes(data, 0, len(data), 0)
	return data
}

// walkBoxes goes through the boxes between from and to, descending into the
// containers that can hold metadata.
//
// depth bounds the recursion: a hand-made file can nest boxes as deeply as it
// likes, and this runs on an upload from outside.
func walkBoxes(data []byte, from, to, depth int) {
	if depth > 4 {
		return
	}
	for pos := from; pos+8 <= to; {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		name := string(data[pos+4 : pos+8])

		switch {
		case size == 0:
			// "to the end of the file" — nothing after it to walk.
			return
		case size == 1:
			// A 64-bit size follows the name. Nothing that carries metadata is
			// ever this big, so it is skipped rather than parsed.
			if pos+16 > to {
				return
			}
			big := binary.BigEndian.Uint64(data[pos+8 : pos+16])
			if big < 16 || pos+int(big) > to {
				return
			}
			pos += int(big)
			continue
		case size < 8 || pos+size > to:
			// Broken or truncated: stop rather than guess.
			return
		}

		if stripBoxes[name] {
			blank(data, pos, size)
		} else if name == "moov" || name == "trak" {
			walkBoxes(data, pos+8, pos+size, depth+1)
		}
		pos += size
	}
}

// blank turns one box into a free box of the same length and clears what was
// in it. The length stays, so every offset in the file stays right.
func blank(data []byte, pos, size int) {
	copy(data[pos+4:pos+8], "free")
	for i := pos + 8; i < pos+size; i++ {
		data[i] = 0
	}
}
