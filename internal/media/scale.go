package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// MaxSide is the longest side, in pixels, an uploaded picture may have before
// the operator is asked whether to scale it down.
//
// A named constant and not a setting: it is the one number the question page,
// the scaling and the documentation all have to agree on, and 2560 is what a
// 27-inch screen at full width shows without the browser scaling it again.
// Anything above it is weight a visitor downloads and never sees.
const MaxSide = 2560

// jpegQuality is the quality a scaled JPEG is written with. Higher than the
// 82 of the smaller derived copies: this file replaces the upload, and the
// other three sizes are made from it.
const jpegQuality = 90

// ErrNotScalable says a picture could not be scaled down: it would not decode,
// it is not a format that is scaled, or the result would not fit the limit.
var ErrNotScalable = errors.New("the image cannot be scaled down")

// ScaledSize is the size a picture of width by height gets when its longest
// side is brought to MaxSide: the proportions stay, the shorter side is rounded
// to the nearest pixel and never drops below one. A picture that already fits
// comes back unchanged.
func ScaledSize(width, height int) (int, int) {
	long := max(width, height)
	if long <= MaxSide {
		return width, height
	}
	scale := func(side int) int {
		// Integer rounding to nearest: (side*MaxSide + long/2) / long.
		return max(1, (side*MaxSide+long/2)/long)
	}
	if width >= height {
		return MaxSide, scale(height)
	}
	return scale(width), MaxSide
}

// Oversize reports whether this picture should be offered for scaling down, and
// how big it is.
//
// The size is the upright size: a photo stored as a landscape bitmap with the
// EXIF orientation of a portrait is reported as the portrait it is, so what the
// question prints is what a visitor will see. The verdict itself does not
// depend on the orientation - the longest side and the pixel count do not
// change under a quarter turn.
//
// Only a JPEG, a PNG, a WebP and a GIF of one frame are asked about. An SVG
// scales itself, a PDF and an MP4 are not pictures this code touches, and an
// animated GIF would lose its animation. A picture over the megapixel budget is
// not offered either: Scaled could not decode it, and the "too large for scaled
// copies" note already covers that file.
//
// The size comes from the header (image.DecodeConfig), never from decoding the
// pixels.
func (in *Intake) Oversize(maxMegapixels int) (width, height int, ok bool) {
	switch in.MimeType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return 0, 0, false
	}
	uw, uh, err := in.uprightSize()
	if err != nil || uw <= 0 || uh <= 0 {
		return 0, 0, false
	}
	if max(uw, uh) <= MaxSide {
		return 0, 0, false
	}
	if maxMegapixels <= 0 {
		maxMegapixels = DefaultMaxMegapixels
	}
	if uw*uh > maxMegapixels*1_000_000 {
		return 0, 0, false
	}
	if in.MimeType == "image/gif" {
		// Scaling flattens an animation to one frame, the reason CanMakeVariants
		// leaves GIF out. The frames are decoded only after the budget check.
		all, err := gif.DecodeAll(bytes.NewReader(in.content))
		if err != nil || len(all.Image) != 1 {
			return 0, 0, false
		}
	}
	return uw, uh, true
}

// Scaled returns a new Intake holding this picture with its longest side at
// MaxSide, the proportions kept. The original is left as it was.
//
// JPEG stays JPEG and PNG stays PNG. A WebP becomes a JPEG when it is opaque
// and a PNG when any pixel is not, because the standard library has no WebP
// encoder and a JPEG would turn transparency black. A still GIF becomes a PNG.
//
// An EXIF orientation of 2 to 8 is applied to the pixels: the copy stands
// upright and the longest side is counted on that upright picture. The encoders
// write no orientation tag, and no EXIF, XMP, comment or colour profile at all,
// so no camera data survives and a browser has nothing to turn a second time.
// The colour profile is not carried over yet. Keep still runs PrepareUpload on
// the result.
//
// ErrNotScalable (wrapping the cause) when the picture does not decode, is not
// one of the four formats, is over the megapixel budget, or the scaled bytes
// would exceed the limit.
func (in *Intake) Scaled(maxMegapixels int) (*Intake, error) {
	switch in.MimeType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return nil, fmt.Errorf("%w: %s", ErrNotScalable, in.MimeType)
	}
	if maxMegapixels <= 0 {
		maxMegapixels = DefaultMaxMegapixels
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(in.content))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotScalable, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxMegapixels*1_000_000 {
		return nil, fmt.Errorf("%w: %dx%d", ErrNotScalable, cfg.Width, cfg.Height)
	}

	// Decoding holds the whole bitmap, so it goes through the same throttle as
	// the scaled copies.
	resizeSlots <- struct{}{}
	defer func() { <-resizeSlots }()

	src, _, err := image.Decode(bytes.NewReader(in.content))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotScalable, err)
	}
	// Scale first, in the geometry the bitmap is stored in, then turn the result:
	// the turn is a permutation of at most MaxSide x MaxSide pixels, so the
	// resampling is exactly what it is without an orientation. ScaledSize is
	// symmetric under swapping the sides, so the stored-geometry size is the
	// upright size swapped back.
	o := orientationOf(in.MimeType, in.content)
	w, h := ScaledSize(uprightOf(cfg.Width, cfg.Height, o))
	if swapsAxes(o) {
		w, h = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	dst = orientRGBA(dst, o)

	mimeType, ext := scaledFormat(in.MimeType, hasTransparency(src))
	var buf bytes.Buffer
	switch mimeType {
	case "image/jpeg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})
	default:
		err = png.Encode(&buf, dst)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotScalable, err)
	}
	if int64(buf.Len()) > in.limit {
		return nil, fmt.Errorf("%w: the scaled image is %d bytes", ErrNotScalable, buf.Len())
	}

	name := in.OriginalName
	if mimeType != in.MimeType {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ext
	}
	return &Intake{
		MimeType:     mimeType,
		OriginalName: name,
		Filename:     GenerateFilename(name),
		Size:         int64(buf.Len()),
		content:      buf.Bytes(),
		limit:        in.limit,
	}, nil
}

// scaledFormat is the format a scaled picture is written in, and the extension
// that goes with it. Pure, so the table of it can be tested without an encoder
// for every input.
func scaledFormat(mimeType string, transparent bool) (string, string) {
	switch mimeType {
	case "image/jpeg":
		return "image/jpeg", ".jpg"
	case "image/webp":
		if transparent {
			return "image/png", ".png"
		}
		return "image/jpeg", ".jpg"
	default: // png, gif
		return "image/png", ".png"
	}
}
