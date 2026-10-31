package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// gradient is a picture that is not one flat colour, so the encoders have
// something to do and a wrong resize shows.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += 1 {
		for x := 0; x < w; x += 1 {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 120, 255})
		}
	}
	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(w, h), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func intakeOf(mimeType, name string, content []byte) *Intake {
	return &Intake{
		MimeType: mimeType, OriginalName: name, Filename: GenerateFilename(name),
		Size: int64(len(content)), content: content, limit: 64 << 20,
	}
}

func TestScaledSize(t *testing.T) {
	for _, tc := range []struct{ w, h, wantW, wantH int }{
		{3000, 1500, 2560, 1280},
		{1500, 3000, 1280, 2560},
		{2560, 1000, 2560, 1000},
		{1000, 2560, 1000, 2560},
		{100, 100, 100, 100},
		{10000, 1, 2560, 1},
		{1, 10000, 1, 2560},
	} {
		w, h := ScaledSize(tc.w, tc.h)
		if w != tc.wantW || h != tc.wantH {
			t.Errorf("ScaledSize(%d, %d) = %d x %d, want %d x %d", tc.w, tc.h, w, h, tc.wantW, tc.wantH)
		}
	}
}

func TestOversize(t *testing.T) {
	big := jpegBytes(t, 3000, 1500)
	if w, h, ok := intakeOf("image/jpeg", "a.jpg", big).Oversize(0); !ok || w != 3000 || h != 1500 {
		t.Errorf("a 3000x1500 JPEG: got %d x %d, %v", w, h, ok)
	}
	if _, _, ok := intakeOf("image/png", "a.png", pngBytes(t, gradient(1500, 2700))).Oversize(0); !ok {
		t.Error("a 1500x2700 PNG should be offered")
	}

	exact := jpegBytes(t, 2560, 800)
	if _, _, ok := intakeOf("image/jpeg", "a.jpg", exact).Oversize(0); ok {
		t.Error("2560 on the long side is not over")
	}
	if _, _, ok := intakeOf("image/svg+xml", "a.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="9000" height="9000"/>`)).Oversize(0); ok {
		t.Error("an SVG is never offered")
	}
	if _, _, ok := intakeOf("video/mp4", "a.mp4", big).Oversize(0); ok {
		t.Error("an MP4 is never offered")
	}
	if _, _, ok := intakeOf("application/pdf", "a.pdf", big).Oversize(0); ok {
		t.Error("a PDF is never offered")
	}
	if _, _, ok := intakeOf("image/webp", "a.webp", []byte("RIFF....WEBPnonsense")).Oversize(0); ok {
		t.Error("an undecodable WebP is never offered")
	}

	// Over the megapixel budget: never offered, Scaled could not decode it.
	if _, _, ok := intakeOf("image/jpeg", "a.jpg", big).Oversize(2); ok {
		t.Error("an image over the megapixel budget was offered")
	}

	// An animated GIF would lose its animation.
	pal := color.Palette{color.Black, color.White}
	frame := func() *image.Paletted { return image.NewPaletted(image.Rect(0, 0, 3000, 100), pal) }
	var anim bytes.Buffer
	if err := gif.EncodeAll(&anim, &gif.GIF{Image: []*image.Paletted{frame(), frame()}, Delay: []int{5, 5}}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := intakeOf("image/gif", "a.gif", anim.Bytes()).Oversize(0); ok {
		t.Error("an animated GIF was offered")
	}
	var still bytes.Buffer
	if err := gif.Encode(&still, frame(), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := intakeOf("image/gif", "a.gif", still.Bytes()).Oversize(0); !ok {
		t.Error("a still GIF should be offered")
	}
}

func dimensionsOf(t *testing.T, b []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestScaledKeepsFormatAndProportions(t *testing.T) {
	// A JPEG carrying a camera block: the marker string must not survive.
	const secret = "SECRET-CAMERA-DATA"
	plain := jpegBytes(t, 3000, 1999)
	withEXIF := append([]byte{}, plain[:2]...)
	payload := append([]byte("Exif\x00\x00"), []byte(secret)...)
	n := len(payload) + 2
	withEXIF = append(withEXIF, 0xFF, 0xE1, byte(n>>8), byte(n))
	withEXIF = append(withEXIF, payload...)
	withEXIF = append(withEXIF, plain[2:]...)
	if !bytes.Contains(withEXIF, []byte(secret)) {
		t.Fatal("test setup: the source carries no marker")
	}

	in := intakeOf("image/jpeg", "foto.jpg", withEXIF)
	out, err := in.Scaled(0)
	if err != nil {
		t.Fatalf("Scaled: %v", err)
	}
	if out.MimeType != "image/jpeg" || out.OriginalName != "foto.jpg" {
		t.Errorf("JPEG should stay JPEG with its name: %s %s", out.MimeType, out.OriginalName)
	}
	w, h := dimensionsOf(t, out.content)
	wantW, wantH := ScaledSize(3000, 1999)
	if w != 2560 || h != wantH || w != wantW {
		t.Errorf("scaled to %d x %d, want 2560 x %d", w, h, wantH)
	}
	if bytes.Contains(out.content, []byte(secret)) || bytes.Contains(out.content, []byte("Exif")) {
		t.Error("camera data survived scaling")
	}
	if out.Size != int64(len(out.content)) || out.Filename == in.Filename {
		t.Errorf("size or filename not renewed: %d / %s", out.Size, out.Filename)
	}
	// The original stays as it was.
	if !bytes.Equal(in.content, withEXIF) {
		t.Error("the source intake was modified")
	}

	// PNG stays PNG.
	pngOut, err := intakeOf("image/png", "logo.png", pngBytes(t, gradient(1000, 3000))).Scaled(0)
	if err != nil {
		t.Fatalf("Scaled png: %v", err)
	}
	if pngOut.MimeType != "image/png" {
		t.Errorf("PNG became %s", pngOut.MimeType)
	}
	if w, h := dimensionsOf(t, pngOut.content); h != 2560 || w != 853 {
		t.Errorf("png scaled to %d x %d, want 853 x 2560", w, h)
	}

	// A still GIF becomes a PNG and its name follows.
	var gbuf bytes.Buffer
	if err := gif.Encode(&gbuf, image.NewPaletted(image.Rect(0, 0, 3000, 200), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	gifOut, err := intakeOf("image/gif", "anim.GIF", gbuf.Bytes()).Scaled(0)
	if err != nil {
		t.Fatalf("Scaled gif: %v", err)
	}
	if gifOut.MimeType != "image/png" || gifOut.OriginalName != "anim.png" {
		t.Errorf("GIF became %s / %s", gifOut.MimeType, gifOut.OriginalName)
	}
}

func TestScaledRefusesWhatItCannotDo(t *testing.T) {
	if _, err := intakeOf("image/svg+xml", "a.svg", []byte("<svg/>")).Scaled(0); !errors.Is(err, ErrNotScalable) {
		t.Errorf("svg: %v", err)
	}
	if _, err := intakeOf("image/jpeg", "a.jpg", []byte("not a jpeg")).Scaled(0); !errors.Is(err, ErrNotScalable) {
		t.Errorf("garbage: %v", err)
	}
	if _, err := intakeOf("image/jpeg", "a.jpg", jpegBytes(t, 3000, 1500)).Scaled(2); !errors.Is(err, ErrNotScalable) {
		t.Errorf("over budget: %v", err)
	}
	small := intakeOf("image/jpeg", "a.jpg", jpegBytes(t, 3000, 1500))
	small.limit = 100
	if _, err := small.Scaled(0); !errors.Is(err, ErrNotScalable) {
		t.Errorf("over the byte limit: %v", err)
	}
}

func TestScaledFormat(t *testing.T) {
	for _, tc := range []struct {
		in          string
		transparent bool
		mime, ext   string
	}{
		{"image/jpeg", false, "image/jpeg", ".jpg"},
		{"image/jpeg", true, "image/jpeg", ".jpg"},
		{"image/png", false, "image/png", ".png"},
		{"image/png", true, "image/png", ".png"},
		{"image/webp", false, "image/jpeg", ".jpg"},
		{"image/webp", true, "image/png", ".png"},
		{"image/gif", false, "image/png", ".png"},
	} {
		m, e := scaledFormat(tc.in, tc.transparent)
		if m != tc.mime || e != tc.ext {
			t.Errorf("scaledFormat(%s, %v) = %s %s, want %s %s", tc.in, tc.transparent, m, e, tc.mime, tc.ext)
		}
	}
}

// The decode side of WebP, with the files that exist: an opaque and a
// transparent one are told apart, which is what picks JPEG or PNG.
func TestWebPDecodesForScaling(t *testing.T) {
	for name, wantTransparent := range map[string]bool{
		"opaque-lossless.webp": false, "transparent.webp": true,
	} {
		body, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if hasTransparency(img) != wantTransparent {
			t.Errorf("%s: transparency = %v, want %v", name, !wantTransparent, wantTransparent)
		}
	}
}
