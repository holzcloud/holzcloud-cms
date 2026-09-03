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
func writeTestImage(t *testing.T, dir, name string, w, h int, transparent bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// A gradient rather than a flat fill: a single colour compresses to
			// almost nothing and would not exercise the encoder honestly.
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
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
		// only the thumbnail — which the admin grid addresses by name — is
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
	if err := store.SaveVariants(ctx, m.ID, 2000, 1000, variants); err != nil {
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
	if err := store.SaveVariants(ctx, m.ID, 2000, 1000, variants); err != nil {
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
	if err := store.SaveVariants(ctx, m.ID, 1200, 600,
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
	if err := store.SaveVariants(ctx, m.ID, 1000, 500,
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
	big := Media{WebsiteID: 1, Filename: "abc.jpg", MimeType: "image/jpeg", Width: 2000}
	if got, want := big.ThumbURL(), "/media/1/abc-thumb.jpg"; got != want {
		t.Errorf("ThumbURL() = %q, want %q", got, want)
	}

	// Below the smallest width nothing was generated, and a grid pointing at a
	// file that does not exist shows a broken image on every card.
	small := Media{WebsiteID: 1, Filename: "logo.jpg", MimeType: "image/jpeg", Width: 200}
	if got, want := small.ThumbURL(), "/media/1/logo.jpg"; got != want {
		t.Errorf("small ThumbURL() = %q, want %q", got, want)
	}

	vector := Media{WebsiteID: 1, Filename: "logo.svg", MimeType: "image/svg+xml", Width: 0}
	if got, want := vector.ThumbURL(), "/media/1/logo.svg"; got != want {
		t.Errorf("svg ThumbURL() = %q, want %q", got, want)
	}
}
