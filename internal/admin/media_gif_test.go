package admin

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"runtime"
	"testing"
)

// manyFrameGIF is an animated GIF with a 2600x1000 logical screen and frames
// full-screen frames. The frames share one bitmap while the file is encoded,
// so building it is cheap and the file is a few kilobytes; a decoder that
// keeps every frame holds frames x 2.6 MB.
func manyFrameGIF(t *testing.T, frames int) []byte {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 2600, 1000), color.Palette{color.Black, color.White})
	g := &gif.GIF{}
	for range frames {
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 5)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An animated GIF is never scaled, so it only has to be recognised. Finding M6:
// recognising it decoded every frame, and an editor's file of a few kilobytes
// with a few hundred large frames was gigabytes of memory.
func TestAnEditorsManyFrameGIFIsStoredWithoutDecodingItsFrames(t *testing.T) {
	anim := manyFrameGIF(t, 40)
	h, sm, _, ws := mediaAdmin(t)
	req := mediaUploadRequest(t, ws.ID, "film.gif", anim)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	rec, _ := serveUser(t, h, sm, 1, h.HandleMediaUpload, req)
	runtime.ReadMemStats(&after)

	if got, want := rec.Header().Get("Location"), fmt.Sprintf("/admin/websites/%d/media", ws.ID); got != want {
		t.Errorf("redirected to %q, want the library %q", got, want)
	}
	if n := len(libraryOf(t, h, ws.ID)); n != 1 {
		t.Errorf("%d files stored, want 1", n)
	}
	if n := stagingEntries(t, h, ws.ID); n != 0 {
		t.Errorf("%d staged, an animated GIF is not asked about", n)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 32<<20 {
		t.Errorf("the upload allocated %d bytes, want under 32 MB: the frames were decoded", grown)
	}
}
