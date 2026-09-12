package media

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func box(name string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(payload)))
	copy(out[4:8], name)
	copy(out[8:], payload)
	return out
}

// A phone video carries where it was taken. That has to disappear — and the
// file has to be playable afterwards, so exactly the same length.
func TestMP4VerliertDenAufnahmeort(t *testing.T) {
	udta := box("udta", []byte("\x00\x00\x00\x18\xa9xyz+47.3769+008.5417/"))
	moov := box("moov", append(box("mvhd", make([]byte, 100)), udta...))
	film := append(box("ftyp", []byte("isomiso2")), moov...)
	film = append(film, box("mdat", []byte("filmdaten"))...)
	vorher := len(film)

	out := StripMP4Metadata(append([]byte(nil), film...))

	if len(out) != vorher {
		t.Fatalf("length %d instead of %d — every position in the file now points to the wrong place", len(out), vorher)
	}
	if bytes.Contains(out, []byte("47.3769")) {
		t.Error("die Koordinaten stehen noch drin")
	}
	if bytes.Contains(out, []byte("udta")) {
		t.Error("the udta box is still called udta")
	}
	if !bytes.Contains(out, []byte("free")) {
		t.Error("there is no free box in its place")
	}
	// And the film itself is untouched.
	if !bytes.Contains(out, []byte("filmdaten")) || !bytes.Contains(out, []byte("isomiso2")) {
		t.Error("something was changed in the film or in the header")
	}
}

// What is not an MP4 comes back unchanged: this is a cleaning agent, not a
// check.
func TestKeinMP4BleibtUnberuehrt(t *testing.T) {
	for _, rein := range [][]byte{
		[]byte("überhaupt kein mp4"),
		{},
		{0, 0, 0, 3},
		box("moov", []byte{0, 0, 0, 99, 'u', 'd', 't', 'a'}), // Länge lügt
	} {
		vorher := append([]byte(nil), rein...)
		if out := StripMP4Metadata(rein); !bytes.Equal(out, vorher) {
			t.Errorf("%q was changed to %q", vorher, out)
		}
	}
}
