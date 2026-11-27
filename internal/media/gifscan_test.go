package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"testing"
)

// rawGIF builds a GIF byte by byte, so a malformed shape is exact. The screen
// has a global colour table of two entries when global is set.
func rawGIF(sig string, w, h int, global bool, blocks ...[]byte) []byte {
	b := []byte(sig)
	b = binary.LittleEndian.AppendUint16(b, uint16(w))
	b = binary.LittleEndian.AppendUint16(b, uint16(h))
	if global {
		b = append(b, 0x80, 0, 0, 0, 0, 0, 255, 255, 255) // flag, background, aspect, 2 colours
	} else {
		b = append(b, 0, 0, 0)
	}
	for _, blk := range blocks {
		b = append(b, blk...)
	}
	return b
}

// rawFrame is an image descriptor with a one-byte LZW stream; the data is not
// valid LZW and the walker must not care.
func rawFrame(left, top, w, h int, local bool) []byte {
	b := []byte{0x2C}
	for _, v := range []int{left, top, w, h} {
		b = binary.LittleEndian.AppendUint16(b, uint16(v))
	}
	if local {
		b = append(b, 0x80, 1, 2, 3, 4, 5, 6) // flag, 2 colours
	} else {
		b = append(b, 0)
	}
	return append(b, 0x02, 0x01, 0x44, 0x00)
}

var (
	rawTrailer = []byte{0x3B}
	// A comment, a NETSCAPE2.0 loop application block and a graphic control
	// extension: what real encoders put before frames.
	rawComment  = []byte{0x21, 0xFE, 0x03, 'h', 'i', '!', 0x00}
	rawNetscape = []byte{0x21, 0xFF, 0x0B, 'N', 'E', 'T', 'S', 'C', 'A', 'P', 'E', '2', '.', '0',
		0x03, 0x01, 0x00, 0x00, 0x00}
	rawGraphicControl = []byte{0x21, 0xF9, 0x04, 0x00, 0x05, 0x00, 0x00, 0x00}
)

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// encodedGIF is gif.EncodeAll's output for frames of different size and offset
// on a 300x200 screen, each with its own palette (so each has a local table).
func encodedGIF(t *testing.T, n int) []byte {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: 300, Height: 200}}
	for i := range n {
		r := image.Rect(i*5, i*3, i*5+40+i*10, i*3+30+i*5)
		pal := color.Palette{color.Black, color.RGBA{uint8(40 * i), 100, 200, 255}}
		g.Image = append(g.Image, image.NewPaletted(r, pal))
		g.Delay = append(g.Delay, 5)
	}
	g.LoopCount = 0
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGIFFramesWalksTheBlocks(t *testing.T) {
	// What the decoder says about real encoder output; the test may decode, the
	// code may not.
	seven := encodedGIF(t, 7)
	all, err := gif.DecodeAll(bytes.NewReader(seven))
	if err != nil {
		t.Fatal(err)
	}
	var sevenPixels int64
	for _, f := range all.Image {
		sevenPixels += int64(f.Bounds().Dx()) * int64(f.Bounds().Dy())
	}
	var still bytes.Buffer
	if err := gif.Encode(&still, image.NewPaletted(image.Rect(0, 0, 50, 20), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}

	const big = int64(1) << 40
	frame := rawFrame(0, 0, 10, 10, false)
	cases := []struct {
		name       string
		data       []byte
		maxFrames  int
		maxPixels  int64
		wantFrames int
		wantPixels int64
		wantErr    error // errors.Is; nil = no error
		anyErr     bool  // any non-nil error that is not errGIFLimit
	}{
		{name: "one frame", data: still.Bytes(), maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 1000},
		{name: "seven encoded frames agree with the decoder", data: seven, maxFrames: 9, maxPixels: big,
			wantFrames: 7, wantPixels: sevenPixels},
		{name: "local colour table", data: rawGIF("GIF89a", 20, 20, true, rawFrame(0, 0, 10, 10, true), rawTrailer),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100},
		{name: "extensions before every frame count like none",
			data: rawGIF("GIF89a", 20, 20, true, rawNetscape, rawComment,
				rawGraphicControl, frame, rawComment, rawGraphicControl, frame, rawGraphicControl, rawFrame(2, 2, 5, 5, true), rawTrailer),
			maxFrames: 9, maxPixels: big, wantFrames: 3, wantPixels: 225},
		{name: "GIF87a is accepted", data: rawGIF("GIF87a", 20, 20, true, frame, rawTrailer),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100},
		{name: "no frame at all", data: rawGIF("GIF89a", 20, 20, false, rawTrailer),
			maxFrames: 9, maxPixels: big, wantFrames: 0, wantPixels: 0},
		{name: "seven frames, limit three, stops at the fourth", data: seven, maxFrames: 3, maxPixels: big,
			wantFrames: 4, wantErr: errGIFLimit, wantPixels: -1},
		{name: "pixel limit below the second frame's area",
			data:      rawGIF("GIF89a", 20, 20, true, rawFrame(0, 0, 10, 10, false), rawFrame(0, 0, 10, 10, false), rawFrame(0, 0, 10, 10, false), rawTrailer),
			maxFrames: 9, maxPixels: 150, wantFrames: 2, wantPixels: 200, wantErr: errGIFLimit},
		{name: "two frames, limit one", data: rawGIF("GIF89a", 20, 20, true, frame, frame, rawTrailer),
			maxFrames: 1, maxPixels: big, wantFrames: 2, wantPixels: 200, wantErr: errGIFLimit},
		{name: "cut inside a sub-block", data: cat(rawGIF("GIF89a", 20, 20, true, frame), []byte{0x21, 0xFE, 0x05, 'a', 'b'}),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100, wantErr: io.ErrUnexpectedEOF},
		{name: "cut inside the image data", data: rawGIF("GIF89a", 20, 20, true, rawFrame(0, 0, 10, 10, false)[:12]),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100, wantErr: io.ErrUnexpectedEOF},
		{name: "cut before the trailer", data: rawGIF("GIF89a", 20, 20, true, frame),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100, wantErr: io.ErrUnexpectedEOF},
		{name: "cut inside the colour table", data: rawGIF("GIF89a", 20, 20, true)[:15],
			maxFrames: 9, maxPixels: big, wantErr: io.ErrUnexpectedEOF},
		{name: "empty", data: nil, maxFrames: 9, maxPixels: big, wantErr: io.ErrUnexpectedEOF},
		{name: "bad introducer", data: rawGIF("GIF89a", 20, 20, true, frame, []byte{0x99}, rawTrailer),
			maxFrames: 9, maxPixels: big, wantFrames: 1, wantPixels: 100, anyErr: true},
		{name: "wrong signature", data: rawGIF("GIF90a", 20, 20, true, frame, rawTrailer),
			maxFrames: 9, maxPixels: big, anyErr: true},
		{name: "a PNG", data: []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), maxFrames: 9, maxPixels: big, anyErr: true},
		{name: "frame outside the screen to the right", data: rawGIF("GIF89a", 20, 20, true, rawFrame(15, 0, 10, 10, false), rawTrailer),
			maxFrames: 9, maxPixels: big, anyErr: true},
		{name: "frame outside the screen below", data: rawGIF("GIF89a", 20, 20, true, rawFrame(0, 15, 10, 10, false), rawTrailer),
			maxFrames: 9, maxPixels: big, anyErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frames, pixels, err := gifFrames(bytes.NewReader(c.data), c.maxFrames, c.maxPixels)
			switch {
			case c.anyErr:
				if err == nil || errors.Is(err, errGIFLimit) {
					t.Fatalf("err = %v, want a format error", err)
				}
			case c.wantErr != nil:
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want %v", err, c.wantErr)
				}
			case err != nil:
				t.Fatalf("err = %v", err)
			}
			if c.anyErr || c.name == "wrong signature" || c.name == "a PNG" {
				return
			}
			if frames != c.wantFrames {
				t.Errorf("frames = %d, want %d", frames, c.wantFrames)
			}
			if c.wantPixels >= 0 && pixels != c.wantPixels {
				t.Errorf("pixels = %d, want %d", pixels, c.wantPixels)
			}
		})
	}

	t.Run("descriptors claiming 65535x65535", func(t *testing.T) {
		claim := rawFrame(0, 0, 65535, 65535, false)
		many := make([][]byte, 1000)
		for i := range many {
			many[i] = claim
		}
		// On a 10x10 screen the first descriptor is outside it.
		small := rawGIF("GIF89a", 10, 10, true, many...)
		frames, _, err := gifFrames(bytes.NewReader(small), 5000, 1<<62)
		if err == nil || errors.Is(err, errGIFLimit) || frames != 0 {
			t.Errorf("10x10 screen: frames=%d err=%v, want 0 and a format error at the first descriptor", frames, err)
		}
		// On a 65535x65535 screen the pixel limit stops it at once: 4.29e9
		// pixels a frame against a limit of 1e10 is the third frame.
		huge := rawGIF("GIF89a", 65535, 65535, true, many...)
		frames, pixels, err := gifFrames(bytes.NewReader(huge), 5000, 10_000_000_000)
		if !errors.Is(err, errGIFLimit) || frames != 3 || pixels != 3*65535*65535 {
			t.Errorf("65535 screen: frames=%d pixels=%d err=%v, want 3, %d and errGIFLimit", frames, pixels, err, int64(3*65535*65535))
		}
	})
}

// Nothing the walker allocates grows with the number of frames.
func TestGIFFramesAllocatesNothingPerFrame(t *testing.T) {
	build := func(n int) []byte {
		frames := make([][]byte, n)
		for i := range frames {
			frames[i] = rawFrame(0, 0, 60000, 60000, i%2 == 0)
		}
		return rawGIF("GIF89a", 65535, 65535, true, append(frames, rawTrailer)...)
	}
	allocs := func(data []byte) float64 {
		return testing.AllocsPerRun(20, func() {
			if _, _, err := gifFrames(bytes.NewReader(data), 1000, 1<<62); err != nil {
				t.Fatal(err)
			}
		})
	}
	two, many := allocs(build(2)), allocs(build(200))
	if many != two {
		t.Errorf("200 frames allocate %v times, 2 frames %v: an allocation grows with the frames", many, two)
	}
	if two > 8 {
		t.Errorf("%v allocations for a walk, want a handful", two)
	}
}

// The walker ends and does not panic on any input, and its counts honour its
// own limits.
func FuzzGIFFrames(f *testing.F) {
	f.Add(rawGIF("GIF89a", 20, 20, true, rawNetscape, rawFrame(0, 0, 10, 10, true), rawFrame(1, 1, 5, 5, false), rawTrailer))
	f.Add(rawGIF("GIF87a", 65535, 65535, false, rawFrame(0, 0, 65535, 65535, false), rawTrailer))
	f.Add(rawGIF("GIF89a", 20, 20, true, rawComment, rawGraphicControl, []byte{0x2C, 0, 0}))
	f.Add([]byte("GIF89a"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		frames, pixels, err := gifFrames(bytes.NewReader(data), 3, 1<<20)
		if frames < 0 || pixels < 0 {
			t.Fatalf("negative counts %d, %d", frames, pixels)
		}
		if err == nil && (frames > 3 || pixels > 1<<20) {
			t.Fatalf("no error but over the limits: %d frames, %d pixels", frames, pixels)
		}
		if errors.Is(err, errGIFLimit) && frames <= 3 && pixels <= 1<<20 {
			t.Fatalf("limit error inside the limits: %d frames, %d pixels", frames, pixels)
		}
	})
}
