package media

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	// Registering the decoders is what makes image.DecodeConfig recognise the
	// formats; WebP is decode-only, which is all that is needed here.
	_ "golang.org/x/image/webp"
)

// VariantSpec is one size the pipeline produces.
type VariantSpec struct {
	Label string
	Width int
}

// variantSpecs are the widths generated for every uploaded photo.
//
// Three sizes, not five: each one costs disk and seconds of CPU on a modest
// server, and the gap between 400 and 800 already covers the difference
// between a phone and a laptop.
var variantSpecs = []VariantSpec{
	{Label: "thumb", Width: 400},
	{Label: "medium", Width: 800},
	{Label: "large", Width: 1600},
}

// DefaultMaxMegapixels bounds what will be decoded.
//
// This limit is the reason DecodeConfig is called before Decode: a five-megabyte
// single-colour PNG may legally be 20000×20000 and decodes to 1.6 GB of NRGBA,
// which the kernel answers by killing the process. Before this pipeline existed
// nothing ever decoded an upload, so the risk arrives with it.
const DefaultMaxMegapixels = 24

// ErrTooManyPixels is returned when an image exceeds the megapixel budget.
var ErrTooManyPixels = errors.New("image has too many pixels")

// Variant is a stored scaled copy.
type Variant struct {
	ID        int64
	MediaID   int64
	Label     string
	Filename  string
	Width     int
	Height    int
	SizeBytes int64
}

// Dimensions reads an image's size without decoding its pixels.
func Dimensions(path string) (width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("read image header: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// CanMakeVariants reports whether a MIME type is one the pipeline handles.
//
// GIF is excluded on purpose: scaling an animated GIF with this code would keep
// only the first frame, which is worse than leaving it alone. SVG scales itself.
func CanMakeVariants(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}

// resizeSlots bounds how many images may be decoded at once.
//
// Decoding holds the full bitmap in memory — a 24-megapixel photo is about
// 96 MB of RGBA for the source plus the destination. Two uploads at once is
// something a small node survives; four is how it meets the OOM killer.
var resizeSlots = make(chan struct{}, 2)

// MakeVariantsThrottled is MakeVariants with the memory budget enforced.
func MakeVariantsThrottled(sourcePath, destDir, baseName, mimeType string, maxMegapixels int) ([]Variant, error) {
	resizeSlots <- struct{}{}
	defer func() { <-resizeSlots }()
	return MakeVariants(sourcePath, destDir, baseName, mimeType, maxMegapixels)
}

// MakeVariants writes the scaled copies of an image next to the original.
//
// Only smaller sizes are produced: upscaling a small logo to 1600 pixels wastes
// disk and produces a blurrier file than the original.
//
// The copies are JPEG unless the decoded picture actually has a pixel that is
// not fully opaque; then they are PNG, because re-encoding a transparent
// picture as JPEG turns the transparent parts black.
//
// The pixels decide, not the MIME type. Until 0.0.6 every PNG and WebP got PNG
// copies because either format can carry transparency — but a screenshot or a
// photo saved as WebP is opaque, and its PNG copies were larger than the
// original (a 2400px WebP of 125 KB had an 800px PNG copy of 159 KB and a
// 1600px one of 452 KB). The rule below that drops a copy which saves nothing
// then dropped both, and the srcset offered only the thumbnail and the full
// original, so a phone downloaded the original.
//
// The MIME type still says whether the pipeline may touch the file at all.
func MakeVariants(sourcePath, destDir, baseName, mimeType string, maxMegapixels int) ([]Variant, error) {
	// A caller that forgot to ask would otherwise flatten an animated GIF to
	// its first frame.
	if !CanMakeVariants(mimeType) {
		return nil, fmt.Errorf("no scaled copies for %s", mimeType)
	}
	if maxMegapixels <= 0 {
		maxMegapixels = DefaultMaxMegapixels
	}

	width, height, err := Dimensions(sourcePath)
	if err != nil {
		return nil, err
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("image reports %dx%d", width, height)
	}
	// The budget is checked on the header, before a single pixel is allocated.
	if width*height > maxMegapixels*1_000_000 {
		return nil, fmt.Errorf("%w: %dx%d exceeds %d megapixels",
			ErrTooManyPixels, width, height, maxMegapixels)
	}

	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	// The original's size is the yardstick every copy has to beat.
	originalSize := int64(0)
	if info, err := f.Stat(); err == nil {
		originalSize = info.Size()
	}

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	transparent := hasTransparency(src)
	var out []Variant
	for _, spec := range variantSpecs {
		if spec.Width >= width {
			continue
		}
		targetHeight := height * spec.Width / width
		if targetHeight < 1 {
			targetHeight = 1
		}

		dst := image.NewRGBA(image.Rect(0, 0, spec.Width, targetHeight))
		// CatmullRom is the sharpest of the kernels in x/image and costs about
		// twice ApproxBiLinear; on a photo that difference is visible and the
		// resize happens once per upload, not per request.
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

		name := variantFilename(baseName, spec.Label, transparent)
		size, err := encodeVariant(filepath.Join(destDir, name), dst, transparent)
		if err != nil {
			// Clean up what was already written, so a failure does not leave
			// half a set of variants that the templates would then reference.
			for _, done := range out {
				os.Remove(filepath.Join(destDir, done.Filename))
			}
			return nil, err
		}

		// A "scaled copy" that is not smaller is a pure loss: it costs disk, and
		// a browser choosing it downloads more than the original would have
		// cost. This happens with an already heavily compressed source, where
		// re-encoding at quality 82 adds more than the smaller pixel count
		// saves. Dropping it leaves the srcset one candidate shorter, which is
		// exactly right — there is nothing better than the original to offer.
		//
		// The thumbnail is exempt: the admin grid and the media picker show it
		// for every card, so it has to exist whenever the original is wider
		// than it. They read its name from media_variants rather than derive
		// it, which is what keeps an install with -thumb.png copies and one
		// with -thumb.jpg copies both working. Its few hundred bytes are not
		// what this rule is about.
		if spec.Label != "thumb" && originalSize > 0 && size >= originalSize {
			os.Remove(filepath.Join(destDir, name))
			continue
		}

		out = append(out, Variant{
			Label: spec.Label, Filename: name,
			Width: spec.Width, Height: targetHeight, SizeBytes: size,
		})
	}
	return out, nil
}

// hasTransparency reports whether any pixel of a decoded picture is not fully
// opaque.
//
// Every type the standard library and x/image decoders return has an Opaque
// method, which stops at the first pixel that is not opaque, so the usual answer
// costs one pass at most and nothing extra for a JPEG. A lossless WebP decodes
// to NRGBA whether or not it uses its alpha channel, which is exactly why the
// type alone cannot answer. The scan below is for an image type without the
// method.
func hasTransparency(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

// variantFilename derives the name of a scaled copy from the original's.
func variantFilename(baseName, label string, transparent bool) string {
	stem := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	ext := ".jpg"
	if transparent {
		ext = ".png"
	}
	return stem + "-" + label + ext
}

func encodeVariant(path string, img image.Image, transparent bool) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create variant: %w", err)
	}
	defer f.Close()

	if transparent {
		err = png.Encode(f, img)
	} else {
		// 82 is the usual quality/size sweet spot. This is a derived file, so
		// re-encoding it costs nothing the original does not still hold.
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 82})
	}
	if err != nil {
		os.Remove(path)
		return 0, fmt.Errorf("encode variant: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat variant: %w", err)
	}
	return info.Size(), nil
}
