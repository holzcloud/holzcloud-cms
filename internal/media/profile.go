package media

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"io"
)

// The ICC colour profile, the one piece of metadata a scaled copy keeps besides
// the picture itself.
//
// A photograph from a phone or a camera is often tagged Display P3 or Adobe RGB.
// Re-encoding drops the tag, and a browser then reads the same numbers as sRGB:
// the picture comes out dull. So a profile is read from the source and written
// into the copy, JPEG to JPEG as APP2 "ICC_PROFILE" chunks and PNG to PNG as an
// iCCP chunk. A WebP source can carry one too (ICCP); the copy is a JPEG or a
// PNG and gets it in that form.
//
// Standard library only. Nothing here returns an error: a profile that cannot
// be read, trusted or written is simply not carried, which is what the program
// did before.

// maxProfile is the cap on a profile that is carried, and on what the zlib
// stream of an iCCP chunk may inflate to. Real profiles run from a few hundred
// bytes to a few hundred kilobytes; a few bytes of zlib that expand to
// gigabytes are the reason there is a cap at all.
const maxProfile = 4 << 20

// profileName is the name written into an iCCP chunk. The specification wants a
// name of 1 to 79 characters and does not care which.
const profileName = "ICC profile"

var iccSignature = []byte("ICC_PROFILE\x00")

// profileOf returns the ICC profile of an image file when it is one that is
// carried: well formed, for RGB, within maxProfile. Otherwise nil.
func profileOf(mimeType string, data []byte) []byte {
	var p []byte
	switch mimeType {
	case "image/jpeg":
		p = jpegProfile(data)
	case "image/png":
		p = pngProfile(data)
	case "image/webp":
		p = webpProfile(data)
	}
	if !validRGBProfile(p) {
		return nil
	}
	return p
}

// validRGBProfile is the whole of the trust placed in a profile: it is at least
// a header long, no longer than maxProfile, declares its own size correctly,
// carries the 'acsp' signature and says its colour space is RGB.
//
// Why a profile for another colour space is refused: the scaled copy is an RGB
// bitmap whatever the source was. A CMYK or greyscale profile attached to RGB
// pixels would make a browser colour them wrongly, and leaving it off is what
// the program did before this existed.
func validRGBProfile(p []byte) bool {
	if len(p) < 132 || len(p) > maxProfile {
		return false
	}
	return int(binary.BigEndian.Uint32(p[0:4])) == len(p) &&
		string(p[36:40]) == "acsp" &&
		string(p[16:20]) == "RGB "
}

// jpegProfile reassembles the profile from the APP2 chunks. The sequence
// numbers must run 1 to n exactly once each and every chunk must state the same
// count; anything else is not a profile this code trusts.
func jpegProfile(data []byte) []byte {
	var chunks [256][]byte
	count, seen, total := 0, 0, 0
	ok := true
	walkJPEG(data, func(marker byte, payload []byte) bool {
		if marker != markerAPP2 || len(payload) < len(iccSignature)+2 || !bytes.HasPrefix(payload, iccSignature) {
			return true
		}
		seq, n := int(payload[len(iccSignature)]), int(payload[len(iccSignature)+1])
		body := payload[len(iccSignature)+2:]
		if count == 0 {
			count = n
		}
		if n == 0 || n != count || seq < 1 || seq > count || chunks[seq] != nil {
			ok = false
			return false
		}
		if total += len(body); total > maxProfile {
			ok = false
			return false
		}
		chunks[seq] = append([]byte{}, body...) // non-nil even when empty
		seen++
		return true
	})
	if !ok || count == 0 || seen != count {
		return nil
	}
	var out []byte
	for i := 1; i <= count; i++ {
		if chunks[i] == nil {
			return nil
		}
		out = append(out, chunks[i]...)
	}
	return out
}

// pngProfile reads the first iCCP chunk: a name, a NUL, the compression method
// (0, deflate) and the zlib stream. The inflate is capped.
func pngProfile(data []byte) []byte {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil
	}
	i := len(pngSignature)
	for i+8 <= len(data) {
		length := uint64(binary.BigEndian.Uint32(data[i : i+4]))
		if uint64(i)+12+length > uint64(len(data)) {
			return nil
		}
		payload := data[i+8 : i+8+int(length)]
		switch string(data[i+4 : i+8]) {
		case "iCCP":
			nul := bytes.IndexByte(payload, 0)
			if nul < 1 || nul+2 > len(payload) || payload[nul+1] != 0 {
				return nil
			}
			zr, err := zlib.NewReader(bytes.NewReader(payload[nul+2:]))
			if err != nil {
				return nil
			}
			defer zr.Close()
			p, err := io.ReadAll(io.LimitReader(zr, maxProfile+1))
			if err != nil || len(p) > maxProfile {
				return nil
			}
			return p
		case "IDAT", "IEND":
			// The profile has to come before the pixels.
			return nil
		}
		i += 12 + int(length)
	}
	return nil
}

// webpProfile is the payload of the first ICCP chunk of a RIFF/WEBP container.
func webpProfile(data []byte) []byte {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil
	}
	if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 > uint64(len(data)) {
		return nil
	}
	i := 12
	for i+8 <= len(data) {
		size := uint64(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if uint64(i)+8+size > uint64(len(data)) {
			return nil
		}
		if string(data[i:i+4]) == "ICCP" {
			return append([]byte{}, data[i+8:i+8+int(size)]...)
		}
		i += 8 + int(size) + int(size%2)
	}
	return nil
}

// withProfile returns an encoded JPEG or PNG carrying the profile. When the
// profile is nil, the format is neither, or the bytes do not have the shape of
// what the standard encoders write, the encoded bytes come back untouched.
func withProfile(mimeType string, encoded, profile []byte) []byte {
	if len(profile) == 0 {
		return encoded
	}
	switch mimeType {
	case "image/jpeg":
		return jpegWithProfile(encoded, profile)
	case "image/png":
		return pngWithProfile(encoded, profile)
	}
	return encoded
}

// jpegWithProfile inserts APP2 segments right after SOI. Each carries up to
// 65519 bytes of the profile (65535 less the length field, the twelve bytes of
// the signature and the two of sequence and count); at most 255 segments.
func jpegWithProfile(encoded, profile []byte) []byte {
	const perSegment = 65535 - 2 - 12 - 2
	if len(encoded) < 2 || encoded[0] != markerPrefix || encoded[1] != markerSOI {
		return encoded
	}
	count := (len(profile) + perSegment - 1) / perSegment
	if count > 255 {
		return encoded
	}
	out := make([]byte, 0, len(encoded)+len(profile)+count*18)
	out = append(out, encoded[:2]...)
	for seq := 1; seq <= count; seq++ {
		part := profile[(seq-1)*perSegment : min(seq*perSegment, len(profile))]
		out = append(out, markerPrefix, markerAPP2)
		out = binary.BigEndian.AppendUint16(out, uint16(2+12+2+len(part)))
		out = append(out, iccSignature...)
		out = append(out, byte(seq), byte(count))
		out = append(out, part...)
	}
	return append(out, encoded[2:]...)
}

// pngWithProfile inserts one iCCP chunk right after IHDR.
func pngWithProfile(encoded, profile []byte) []byte {
	const afterIHDR = 8 + 8 + 13 + 4
	if len(encoded) < afterIHDR || !bytes.HasPrefix(encoded, pngSignature) ||
		string(encoded[12:16]) != "IHDR" || binary.BigEndian.Uint32(encoded[8:12]) != 13 {
		return encoded
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	if _, err := zw.Write(profile); err != nil {
		return encoded
	}
	if err := zw.Close(); err != nil {
		return encoded
	}
	chunk := make([]byte, 8, 8+len(profileName)+2+z.Len()+4)
	binary.BigEndian.PutUint32(chunk, uint32(len(profileName)+2+z.Len()))
	copy(chunk[4:], "iCCP")
	chunk = append(chunk, profileName...)
	chunk = append(chunk, 0, 0) // end of the name, compression method 0
	chunk = append(chunk, z.Bytes()...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))

	out := make([]byte, 0, len(encoded)+len(chunk))
	out = append(out, encoded[:afterIHDR]...)
	out = append(out, chunk...)
	return append(out, encoded[afterIHDR:]...)
}
