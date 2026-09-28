package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeTestImage puts a real encoded image on disk and returns its path.
//
// transparent means a PNG that really is transparent: its top quarter has
// alpha 0. The format of the scaled copies is decided from the pixels, so a
// PNG that merely could be transparent would not exercise that path.
func writeTestImage(t *testing.T, dir, name string, w, h int, transparent bool) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// A gradient rather than a flat fill: a single colour compresses to
			// almost nothing and would not exercise the encoder honestly.
			a := uint8(255)
			if transparent && y < h/4 {
				a = 0
			}
			img.Set(x, y, color.NRGBA{uint8(x % 256), uint8(y % 256), 120, a})
		}
	}

	var buf bytes.Buffer
	var err error
	if transparent {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// writeOpaquePNG puts a PNG on disk in which every pixel is fully opaque — a
// screenshot, as far as the pipeline can tell.
func writeOpaquePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// copyFixture copies one of the WebP files in testdata into dir under a new
// name and returns its path.
//
// The three fixtures were made once with Python's Pillow, and all are 1000x560:
//   - opaque-lossy.webp is photo-like: per pixel a red, green and blue made of
//     slow sine and cosine waves, plus one random offset between -10 and 10 on
//     all three channels (random seed 1), saved as lossy WebP at quality 80,
//     method 6. It exists because it is the case that was broken: its 800px PNG
//     copy is several times larger than the original and was dropped, its 800px
//     JPEG copy is about half the original's size.
//   - opaque-lossless.webp is screenshot-like: a light grey background, a dark
//     blue bar over the top 48 pixels and a dark grey bar of varying length
//     every 28 pixels, saved as lossless WebP. It decodes to an *image.NRGBA in
//     which every pixel happens to be opaque.
//   - transparent.webp is a red ellipse on a fully transparent background,
//     saved as lossless WebP.
func copyFixture(t *testing.T, fixture, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestMakeVariantsProducesSmallerCopies(t *testing.T) {
	dir := t.TempDir()
	src := writeTestImage(t, dir, "photo.jpg", 2000, 1000, false)

	variants, err := MakeVariants(src, dir, "photo.jpg", "image/jpeg", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	if len(variants) == 0 {
		t.Fatal("a 2000px photo produced no scaled copies at all")
	}
	if variants[0].Label != "thumb" {
		t.Errorf("first variant is %q, want the thumbnail the admin grid needs", variants[0].Label)
	}

	orig, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat original: %v", err)
	}
	for _, v := range variants {
		if v.Height != v.Width/2 {
			t.Errorf("%s is %dx%d, aspect ratio not kept", v.Label, v.Width, v.Height)
		}
		info, err := os.Stat(filepath.Join(dir, v.Filename))
		if err != nil {
			t.Fatalf("variant %s not on disk: %v", v.Label, err)
		}
		if info.Size() != v.SizeBytes {
			t.Errorf("%s: recorded %d bytes, file has %d", v.Label, v.SizeBytes, info.Size())
		}
		// A copy that is not smaller costs disk and makes the page heavier, so
		// only the thumbnail — which the admin grid shows for every card — is
		// allowed to be one.
		if v.Label != "thumb" && info.Size() >= orig.Size() {
			t.Errorf("%s is not smaller than the original", v.Label)
		}
	}
}

func TestMakeVariantsDropsCopiesThatSaveNothing(t *testing.T) {
	dir := t.TempDir()
	// An already heavily compressed source: re-encoding at quality 82 costs
	// more than the smaller pixel count saves.
	src := writeTestImage(t, dir, "noisy.jpg", 2000, 1000, false)

	variants, err := MakeVariants(src, dir, "noisy.jpg", "image/jpeg", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}

	orig, _ := os.Stat(src)
	for _, v := range variants {
		if v.Label == "thumb" {
			continue
		}
		if v.SizeBytes >= orig.Size() {
			t.Errorf("%s was kept at %d bytes although the original is %d",
				v.Label, v.SizeBytes, orig.Size())
		}
		// A dropped variant must leave no file behind either, or the disk fills
		// with copies no page will ever reference.
		if _, err := os.Stat(filepath.Join(dir, v.Filename)); err != nil {
			t.Errorf("%s is recorded but not on disk", v.Label)
		}
	}

	// Whatever was dropped must not linger as a file.
	entries, _ := os.ReadDir(dir)
	if got, want := len(entries), len(variants)+1; got != want {
		t.Errorf("%d files in the directory, want %d (original plus kept copies)", got, want)
	}
}

func TestMakeVariantsNeverUpscales(t *testing.T) {
	dir := t.TempDir()
	// A logo, not a photo: 300 pixels wide is below every configured width.
	src := writeTestImage(t, dir, "logo.jpg", 300, 200, false)

	variants, err := MakeVariants(src, dir, "logo.jpg", "image/jpeg", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("got %d variants for a 300px image, want none", len(variants))
	}
}

func TestMakeVariantsKeepsTransparency(t *testing.T) {
	dir := t.TempDir()
	src := writeTestImage(t, dir, "sign.png", 1200, 600, true)

	variants, err := MakeVariants(src, dir, "sign.png", "image/png", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	if len(variants) == 0 {
		t.Fatal("no variants for a 1200px PNG")
	}
	for _, v := range variants {
		// Re-encoding a transparent PNG as JPEG turns the transparent parts
		// black, which is the defect this guards.
		if !strings.HasSuffix(v.Filename, ".png") {
			t.Errorf("%s was written as %s, want PNG", v.Label, v.Filename)
		}
	}
}

func TestMakeVariantsRefusesOversizedImage(t *testing.T) {
	dir := t.TempDir()
	src := writeTestImage(t, dir, "huge.png", 3000, 1000, true)

	// 3000×1000 is 3 megapixels; a budget of 2 must stop it before Decode
	// allocates the bitmap.
	_, err := MakeVariants(src, dir, "huge.png", "image/png", 2)
	if err == nil {
		t.Fatal("an oversized image was accepted")
	}
	if !strings.Contains(err.Error(), "megapixels") {
		t.Fatalf("error does not name the limit: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("the rejected image left %d files behind, want only the original", len(entries))
	}
}

func TestCanMakeVariantsSkipsAnimationAndVectors(t *testing.T) {
	for mime, want := range map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/webp":      true,
		"image/gif":       false, // scaling would keep only the first frame
		"image/svg+xml":   false, // scales itself
		"application/pdf": false,
	} {
		if got := CanMakeVariants(mime); got != want {
			t.Errorf("CanMakeVariants(%q) = %v, want %v", mime, got, want)
		}
	}
}

func TestSaveAndLoadImageSets(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()

	m, err := store.Create(ctx, websiteID, "abc-photo.jpg", "photo.jpg", "image/jpeg", 900, "hash1")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := store.UpdateMeta(ctx, m.ID, "Die Werkstatt", ""); err != nil {
		t.Fatalf("update meta: %v", err)
	}

	variants := []Variant{
		{Label: "thumb", Filename: "abc-photo-thumb.jpg", Width: 400, Height: 200, SizeBytes: 10},
		{Label: "medium", Filename: "abc-photo-medium.jpg", Width: 800, Height: 400, SizeBytes: 20},
	}
	if err := store.SaveVariants(ctx, t.TempDir(), m.ID, 2000, 1000, variants); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}

	body := `<p><img src="/media/` + strconv.FormatInt(websiteID, 10) + `/abc-photo.jpg" alt="Die Werkstatt"></p>`
	idx, err := store.LoadImageSets(ctx, websiteID, body)
	if err != nil {
		t.Fatalf("LoadImageSets: %v", err)
	}
	set, ok := idx["/media/1/abc-photo.jpg"]
	if !ok {
		t.Fatalf("image not in index: %v", idx)
	}
	if set.Width != 2000 || set.Height != 1000 {
		t.Errorf("dimensions not stored: %dx%d", set.Width, set.Height)
	}
	if len(set.Variants) != 2 {
		t.Fatalf("got %d variants, want 2", len(set.Variants))
	}
	// Narrowest first, and the original closes the list as the widest candidate.
	want := "/media/1/abc-photo-thumb.jpg 400w, /media/1/abc-photo-medium.jpg 800w, /media/1/abc-photo.jpg 2000w"
	if got := set.SrcSet(); got != want {
		t.Errorf("SrcSet()\n got %q\nwant %q", got, want)
	}

	// Re-running the pipeline replaces rather than duplicates.
	if err := store.SaveVariants(ctx, t.TempDir(), m.ID, 2000, 1000, variants); err != nil {
		t.Fatalf("second SaveVariants: %v", err)
	}
	again, _ := store.VariantsFor(ctx, m.ID)
	if len(again) != 2 {
		t.Errorf("re-running the pipeline produced %d rows, want 2", len(again))
	}
}

func TestResolveServedFindsVariants(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()

	m, err := store.Create(ctx, websiteID, "abc-sign.png", "sign.png", "image/png", 900, "hash2")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := store.SaveVariants(ctx, t.TempDir(), m.ID, 1200, 600,
		[]Variant{{Label: "thumb", Filename: "abc-sign-thumb.png", Width: 400, Height: 200}}); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}

	served, err := store.ResolveServed(ctx, websiteID, "abc-sign-thumb.png")
	if err != nil {
		t.Fatalf("ResolveServed: %v", err)
	}
	if served == nil {
		t.Fatal("a variant in the srcset does not resolve — every candidate would 404")
	}
	if served.MimeType != "image/png" {
		t.Errorf("MIME type %q, want image/png", served.MimeType)
	}

	// Another site must not reach it.
	other, err := store.ResolveServed(ctx, websiteID+1, "abc-sign-thumb.png")
	if err != nil {
		t.Fatalf("ResolveServed for other site: %v", err)
	}
	if other != nil {
		t.Error("a variant is reachable through another website's media route")
	}
}

func TestDeleteRemovesVariantFiles(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()
	dataDir := t.TempDir()

	dir := filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"abc.jpg", "abc-thumb.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	m, err := store.Create(ctx, websiteID, "abc.jpg", "abc.jpg", "image/jpeg", 1, "hash3")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := store.SaveVariants(ctx, dir, m.ID, 1000, 500,
		[]Variant{{Label: "thumb", Filename: "abc-thumb.jpg", Width: 400, Height: 200}}); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}

	if err := store.Delete(ctx, m.ID, dataDir, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// The rows go with the cascade; the files are the part that would otherwise
	// sit on the card forever.
	if _, err := os.Stat(filepath.Join(dir, "abc-thumb.jpg")); !os.IsNotExist(err) {
		t.Error("the scaled copy survived the deletion of its original")
	}
}

func TestThumbURLFallsBackToOriginal(t *testing.T) {
	big := Media{WebsiteID: 1, Filename: "abc.png", MimeType: "image/png", Width: 2000,
		ThumbFilename: "abc-thumb.jpg"}
	if got, want := big.ThumbURL(), "/media/1/abc-thumb.jpg"; got != want {
		t.Errorf("ThumbURL() = %q, want %q", got, want)
	}

	// A wide picture without a stored thumbnail — its variants failed while
	// its size was still recorded — must not point at a file that was never
	// written: a grid pointing at nothing shows a broken image on the card.
	wide := Media{WebsiteID: 1, Filename: "wide.jpg", MimeType: "image/jpeg", Width: 2000}
	if got, want := wide.ThumbURL(), "/media/1/wide.jpg"; got != want {
		t.Errorf("wide ThumbURL() = %q, want %q", got, want)
	}
	if wide.HasThumb() {
		t.Error("HasThumb() is true without a stored thumbnail")
	}

	vector := Media{WebsiteID: 1, Filename: "logo.svg", MimeType: "image/svg+xml", Width: 0}
	if got, want := vector.ThumbURL(), "/media/1/logo.svg"; got != want {
		t.Errorf("svg ThumbURL() = %q, want %q", got, want)
	}
}

// onlyColorModel is an image with nothing but the three methods of
// image.Image, so hasTransparency has to take its fallback scan.
type onlyColorModel struct {
	img image.Image
}

func (o onlyColorModel) ColorModel() color.Model { return o.img.ColorModel() }
func (o onlyColorModel) Bounds() image.Rectangle { return o.img.Bounds() }
func (o onlyColorModel) At(x, y int) color.Color { return o.img.At(x, y) }

func TestHasTransparency(t *testing.T) {
	r := image.Rect(0, 0, 4, 3)

	opaqueRGBA := image.NewRGBA(r)
	for i := 3; i < len(opaqueRGBA.Pix); i += 4 {
		opaqueRGBA.Pix[i] = 0xff
	}

	opaqueNRGBA := image.NewNRGBA(r)
	for i := 3; i < len(opaqueNRGBA.Pix); i += 4 {
		opaqueNRGBA.Pix[i] = 0xff
	}
	holeNRGBA := image.NewNRGBA(r)
	copy(holeNRGBA.Pix, opaqueNRGBA.Pix)
	holeNRGBA.SetNRGBA(2, 1, color.NRGBA{10, 20, 30, 0})

	ycbcr := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)

	nycbcra := image.NewNYCbCrA(r, image.YCbCrSubsampleRatio420)
	for i := range nycbcra.A {
		nycbcra.A[i] = 0xff
	}
	nycbcraHole := image.NewNYCbCrA(r, image.YCbCrSubsampleRatio420)
	for i := range nycbcraHole.A {
		nycbcraHole.A[i] = 0xff
	}
	nycbcraHole.A[5] = 0x80

	palette := color.Palette{color.NRGBA{0, 0, 0, 0}, color.NRGBA{200, 40, 40, 255}}
	palUsed := image.NewPaletted(r, palette) // every index 0: the transparent entry
	palUnused := image.NewPaletted(r, palette)
	for i := range palUnused.Pix {
		palUnused.Pix[i] = 1
	}

	for _, tc := range []struct {
		name string
		img  image.Image
		want bool
	}{
		{"opaque RGBA", opaqueRGBA, false},
		{"opaque NRGBA, what a lossless WebP decodes to", opaqueNRGBA, false},
		{"NRGBA with one transparent pixel", holeNRGBA, true},
		{"YCbCr has no alpha at all", ycbcr, false},
		{"opaque NYCbCrA", nycbcra, false},
		{"NYCbCrA with one half-transparent pixel", nycbcraHole, true},
		{"paletted, transparent entry used", palUsed, true},
		{"paletted, transparent entry unused", palUnused, false},
		{"no Opaque method, opaque", onlyColorModel{opaqueNRGBA}, false},
		{"no Opaque method, one hole", onlyColorModel{holeNRGBA}, true},
	} {
		if got := hasTransparency(tc.img); got != tc.want {
			t.Errorf("%s: hasTransparency = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// assertFormats checks that every variant carries the extension and has the
// encoding the pipeline promised.
func assertFormats(t *testing.T, dir string, variants []Variant, ext, format string) {
	t.Helper()
	if len(variants) == 0 {
		t.Fatal("no variants at all")
	}
	for _, v := range variants {
		if !strings.HasSuffix(v.Filename, ext) {
			t.Errorf("%s was written as %s, want %s", v.Label, v.Filename, ext)
		}
		f, err := os.Open(filepath.Join(dir, v.Filename))
		if err != nil {
			t.Fatalf("variant %s not on disk: %v", v.Label, err)
		}
		_, got, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatalf("read %s: %v", v.Filename, err)
		}
		if got != format {
			t.Errorf("%s is encoded as %s, want %s", v.Filename, got, format)
		}
	}
}

func TestOpaqueLossyWebPGetsJPEGCopies(t *testing.T) {
	dir := t.TempDir()
	src := copyFixture(t, "opaque-lossy.webp", dir, "shot.webp")

	variants, err := MakeVariants(src, dir, "shot.webp", "image/webp", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	assertFormats(t, dir, variants, ".jpg", "jpeg")

	byLabel := map[string]Variant{}
	for _, v := range variants {
		byLabel[v.Label] = v
	}
	if got := byLabel["thumb"].Filename; got != "shot-thumb.jpg" {
		t.Errorf("thumbnail is %q, want shot-thumb.jpg", got)
	}
	// The point of the change: the 800px copy is what a phone asks for, and as
	// a PNG it was larger than the original and got dropped.
	medium, ok := byLabel["medium"]
	if !ok {
		t.Fatalf("no medium copy of an opaque WebP; got %v", variants)
	}
	if medium.Filename != "shot-medium.jpg" {
		t.Errorf("medium copy is %q, want shot-medium.jpg", medium.Filename)
	}
	orig, _ := os.Stat(src)
	if medium.SizeBytes >= orig.Size() {
		t.Errorf("medium copy is %d bytes, original %d", medium.SizeBytes, orig.Size())
	}
	t.Logf("original %d bytes, medium JPEG %d bytes", orig.Size(), medium.SizeBytes)
}

func TestOpaqueLosslessWebPGetsJPEGCopies(t *testing.T) {
	dir := t.TempDir()
	src := copyFixture(t, "opaque-lossless.webp", dir, "screen.webp")

	variants, err := MakeVariants(src, dir, "screen.webp", "image/webp", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	// A lossless WebP decodes to NRGBA — a type that can carry transparency.
	// Whether it does is a question for the pixels, not for the type.
	assertFormats(t, dir, variants, ".jpg", "jpeg")
	if variants[0].Filename != "screen-thumb.jpg" {
		t.Errorf("thumbnail is %q, want screen-thumb.jpg", variants[0].Filename)
	}
}

func TestTransparentWebPKeepsPNGCopies(t *testing.T) {
	dir := t.TempDir()
	src := copyFixture(t, "transparent.webp", dir, "badge.webp")

	variants, err := MakeVariants(src, dir, "badge.webp", "image/webp", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	assertFormats(t, dir, variants, ".png", "png")

	f, err := os.Open(filepath.Join(dir, variants[0].Filename))
	if err != nil {
		t.Fatalf("open thumbnail: %v", err)
	}
	defer f.Close()
	thumb, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	// JPEG would have turned the background black; the PNG copy must still
	// let the page show through.
	b := thumb.Bounds()
	found := false
	for y := b.Min.Y; y < b.Max.Y && !found; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := thumb.At(x, y).RGBA(); a != 0xffff {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("the thumbnail of a transparent WebP has no transparent pixel left")
	}
}

func TestOpaquePNGGetsJPEGCopies(t *testing.T) {
	dir := t.TempDir()
	src := writeOpaquePNG(t, dir, "screen.png", 1200, 600)

	variants, err := MakeVariants(src, dir, "screen.png", "image/png", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	assertFormats(t, dir, variants, ".jpg", "jpeg")
}

func TestMakeVariantsRefusesAnimation(t *testing.T) {
	dir := t.TempDir()
	src := writeOpaquePNG(t, dir, "anim.gif", 1200, 600)

	// Whatever the bytes are, a GIF is never flattened to its first frame,
	// even by a caller that forgot to ask CanMakeVariants.
	if _, err := MakeVariants(src, dir, "anim.gif", "image/gif", 24); err == nil {
		t.Fatal("MakeVariants accepted image/gif")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("%d files in the directory, want only the original", len(entries))
	}
}

func TestOpaqueWebPSrcSetOffersTheMediumCopy(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := copyFixture(t, "opaque-lossy.webp", dir, "shot.webp")
	info, _ := os.Stat(src)

	m, err := store.Create(ctx, websiteID, "shot.webp", "shot.webp", "image/webp", info.Size(), "hash-webp")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	variants, err := MakeVariants(src, dir, "shot.webp", "image/webp", 24)
	if err != nil {
		t.Fatalf("MakeVariants: %v", err)
	}
	if err := store.SaveVariants(ctx, dir, m.ID, 1000, 560, variants); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}

	body := `<p><img src="/media/` + strconv.FormatInt(websiteID, 10) + `/shot.webp" alt="Bildschirm"></p>`
	idx, err := store.LoadImageSets(ctx, websiteID, body)
	if err != nil {
		t.Fatalf("LoadImageSets: %v", err)
	}
	out := MakeResponsive(body, idx)
	for _, want := range []string{"shot-thumb.jpg 400w", "shot-medium.jpg 800w", "shot.webp 1000w"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestEveryStoreReadCarriesTheStoredThumbName(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()

	m, err := store.Create(ctx, websiteID, "shot.webp", "shot.webp", "image/webp", 900, "hash-thumb")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if m.ThumbURL() != "/media/1/shot.webp" {
		t.Errorf("before any variant, ThumbURL() = %q, want the original", m.ThumbURL())
	}

	reads := func(want string) {
		t.Helper()
		byID, err := store.GetByID(ctx, m.ID)
		if err != nil || byID == nil {
			t.Fatalf("GetByID: %v", err)
		}
		byName, err := store.GetByFilename(ctx, websiteID, "shot.webp")
		if err != nil || byName == nil {
			t.Fatalf("GetByFilename: %v", err)
		}
		byHash, err := store.FindByHash(ctx, websiteID, "hash-thumb")
		if err != nil || byHash == nil {
			t.Fatalf("FindByHash: %v", err)
		}
		list, _, err := store.List(ctx, websiteID, Filter{}, 1, 10)
		if err != nil || len(list) != 1 {
			t.Fatalf("List: %d rows, %v", len(list), err)
		}
		for name, got := range map[string]Media{
			"GetByID": *byID, "GetByFilename": *byName, "FindByHash": *byHash, "List": list[0],
		} {
			if got.ThumbURL() != want {
				t.Errorf("%s: ThumbURL() = %q, want %q", name, got.ThumbURL(), want)
			}
		}
	}

	// An install from before the change: its thumbnail is a PNG and stays
	// addressable as one.
	if err := store.SaveVariants(ctx, t.TempDir(), m.ID, 2000, 1120,
		[]Variant{{Label: "thumb", Filename: "shot-thumb.png", Width: 400, Height: 224}}); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}
	reads("/media/1/shot-thumb.png")

	if err := store.SaveVariants(ctx, t.TempDir(), m.ID, 2000, 1120,
		[]Variant{{Label: "thumb", Filename: "shot-thumb.jpg", Width: 400, Height: 224}}); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}
	reads("/media/1/shot-thumb.jpg")

	if err := store.SaveCrop(ctx, m.ID, Crop{FocusX: 50, FocusY: 50}, 2000, 1120); err != nil {
		t.Fatalf("SaveCrop: %v", err)
	}
	reads("/media/1/shot-thumb.jpg?v=1")

	// Measured, wide, but no thumbnail row: the original, never a guess.
	bare, err := store.Create(ctx, websiteID, "bare.png", "bare.png", "image/png", 900, "hash-bare")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if err := store.SaveVariants(ctx, t.TempDir(), bare.ID, 2000, 1000, nil); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}
	got, _ := store.GetByID(ctx, bare.ID)
	if got.Width != 2000 || got.ThumbURL() != "/media/1/bare.png" {
		t.Errorf("wide picture without a thumb row: width %d, ThumbURL() = %q", got.Width, got.ThumbURL())
	}
}

// assertNothingDangles checks the two promises a regeneration has to keep for
// one website directory: every stored row names a file that is there, and every
// file there is an original, the untouched source of a cropped one, or named by
// a row.
func assertNothingDangles(t *testing.T, s *Store, websiteID int64, dir string) {
	t.Helper()
	rows, err := s.DB.Read.Query(
		`SELECT m.filename, COALESCE(v.filename, '') FROM media m
		 LEFT JOIN media_variants v ON v.media_id = m.id WHERE m.website_id = $1`, websiteID)
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	defer rows.Close()
	known := map[string]bool{}
	var named []string
	for rows.Next() {
		var original, variant string
		if err := rows.Scan(&original, &variant); err != nil {
			t.Fatalf("scan: %v", err)
		}
		known[original], known[SourceName(original)] = true, true
		if variant != "" {
			known[variant] = true
			named = append(named, variant)
		}
	}
	for _, name := range named {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("row names %s, which is not on disk", name)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !known[e.Name()] {
			t.Errorf("%s is on disk, but no row names it", e.Name())
		}
	}
}

func TestRegenerationReplacesTheWholeSet(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := copyFixture(t, "opaque-lossy.webp", dir, "shot.webp")
	info, _ := os.Stat(src)

	m, err := store.Create(ctx, websiteID, "shot.webp", "shot.webp", "image/webp", info.Size(), "hash-swap")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	// The state an install is in before the change: PNG copies, and a label
	// the new run will not produce at all.
	for _, name := range []string{"shot-thumb.png", "shot-large.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := store.SaveVariants(ctx, dir, m.ID, 1000, 560, []Variant{
		{Label: "thumb", Filename: "shot-thumb.png", Width: 400, Height: 224},
		{Label: "large", Filename: "shot-large.png", Width: 1600, Height: 896},
	}); err != nil {
		t.Fatalf("seed SaveVariants: %v", err)
	}

	for run := 1; run <= 2; run++ {
		variants, err := MakeVariants(src, dir, "shot.webp", "image/webp", 24)
		if err != nil {
			t.Fatalf("run %d: MakeVariants: %v", run, err)
		}
		if err := store.SaveVariants(ctx, dir, m.ID, 1000, 560, variants); err != nil {
			t.Fatalf("run %d: SaveVariants: %v", run, err)
		}

		rows, _ := store.VariantsFor(ctx, m.ID)
		got := map[string]string{}
		for _, v := range rows {
			got[v.Label] = v.Filename
		}
		want := map[string]string{"thumb": "shot-thumb.jpg", "medium": "shot-medium.jpg"}
		if len(got) != len(want) || got["thumb"] != want["thumb"] || got["medium"] != want["medium"] {
			t.Errorf("run %d: rows are %v, want exactly %v", run, got, want)
		}
		for _, gone := range []string{"shot-thumb.png", "shot-large.png"} {
			if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
				t.Errorf("run %d: %s is still on disk", run, gone)
			}
		}
		if _, err := os.Stat(src); err != nil {
			t.Fatalf("run %d: the original is gone: %v", run, err)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 3 {
			t.Errorf("run %d: %d files in the directory, want the original and two copies", run, len(entries))
		}
		assertNothingDangles(t, store, websiteID, dir)
	}
}

func TestRegenerationDeletesNothingItShouldNot(t *testing.T) {
	store, websiteID := newTestStore(t)
	ctx := context.Background()
	parent := t.TempDir()
	dir := filepath.Join(parent, "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(parent, "escape.png")
	for _, path := range []string{outside, filepath.Join(dir, "sign.png"), filepath.Join(dir, SourceName("sign.png"))} {
		if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	m, err := store.Create(ctx, websiteID, "sign.png", "sign.png", "image/png", 4, "hash-safe")
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	// Rows that a damaged or hostile database could hold: a name reaching out
	// of the directory, the original's own name, and its crop source.
	if err := store.SaveVariants(ctx, dir, m.ID, 1000, 500, []Variant{
		{Label: "thumb", Filename: "../escape.png", Width: 400, Height: 200},
		{Label: "medium", Filename: "sign.png", Width: 800, Height: 400},
		{Label: "large", Filename: SourceName("sign.png"), Width: 1600, Height: 800},
	}); err != nil {
		t.Fatalf("seed SaveVariants: %v", err)
	}

	if err := store.SaveVariants(ctx, dir, m.ID, 1000, 500, nil); err != nil {
		t.Fatalf("SaveVariants: %v", err)
	}
	if rows, _ := store.VariantsFor(ctx, m.ID); len(rows) != 0 {
		t.Errorf("%d rows left, want none", len(rows))
	}
	for _, path := range []string{outside, filepath.Join(dir, "sign.png"), filepath.Join(dir, SourceName("sign.png"))} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was deleted: %v", path, err)
		}
	}
}
