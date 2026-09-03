package media

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

// Cropping without a mouse.
//
// A drag-a-rectangle tool needs JavaScript, and this project permits htmx and
// nothing else. That constraint turned out to be a better design than the one
// it ruled out: what people actually want from a crop is "make this a square
// with the dog in it", not a rectangle placed to the pixel.
//
// So a crop is three plain answers — where the subject is, which shape the
// picture should be, how close in — and every one of them fits in a form field.
// The subject is set by clicking the picture, which an <input type="image">
// reports as ordinary form values, so even that needs no script.
//
// The uploaded file is never changed. It is copied aside the first time a
// picture is cropped, and every later crop starts from it again — otherwise
// each edit would be applied to the result of the last one, and a picture would
// lose quality every time somebody changed their mind.

// Ratios a picture can be cropped to.
//
// A short list on purpose: these are the shapes a website has room for, and a
// free ratio field would mostly produce 1:1.02 by accident.
type Ratio struct {
	Key   string
	Label string
	// W and H are the proportions; both zero means the original shape.
	W, H int
}

// Ratios is the menu, in the order it is offered.
var Ratios = []Ratio{
	{"", "Wie das Original", 0, 0},
	{"1-1", "Quadrat (1:1)", 1, 1},
	{"4-3", "Klassisch (4:3)", 4, 3},
	{"3-2", "Foto (3:2)", 3, 2},
	{"16-9", "Breit (16:9)", 16, 9},
	{"4-5", "Hoch (4:5)", 4, 5},
	{"9-16", "Hochkant (9:16)", 9, 16},
}

// RatioOf resolves a stored key.
func RatioOf(key string) (Ratio, bool) {
	for _, r := range Ratios {
		if r.Key == key {
			return r, true
		}
	}
	return Ratio{}, false
}

// MaxZoom is how far in a crop may go.
//
// Beyond three times, a photo from a phone stops being a crop and becomes a
// blur — and the person doing it has no way to see that until it is on the
// site.
const MaxZoom = 300

// Crop is what an editor chose. The zero value means "leave the picture alone".
type Crop struct {
	// Rotation is 0, 90, 180 or 270 degrees clockwise. It is applied first:
	// everything else is measured on the upright picture, which is the one the
	// editor is looking at.
	Rotation int
	// Ratio is a key from Ratios; empty keeps the original shape.
	Ratio string
	// Zoom is a percentage from 100. At 100 the crop is the largest rectangle
	// of the chosen shape that fits; at 200 it is half as wide and tall.
	Zoom int
	// FocusX and FocusY are where the subject is, in percent from the top left.
	// The crop is centred on that point, as far as the edges allow.
	FocusX int
	FocusY int
}

// Empty reports whether this crop would change nothing.
func (c Crop) Empty() bool {
	return c.Rotation == 0 && c.Ratio == "" && (c.Zoom == 0 || c.Zoom == 100)
}

// Normalise clamps everything to a value the pipeline can act on.
//
// Called on the way in and on the way out. The values arrive from a form, and a
// zoom of -4 or a rotation of 37 must become something harmless rather than
// something that produces a zero-pixel image five steps later.
func (c Crop) Normalise() Crop {
	switch c.Rotation {
	case 90, 180, 270:
	default:
		c.Rotation = 0
	}
	if _, ok := RatioOf(c.Ratio); !ok {
		c.Ratio = ""
	}
	if c.Zoom < 100 {
		c.Zoom = 100
	}
	if c.Zoom > MaxZoom {
		c.Zoom = MaxZoom
	}
	c.FocusX = clampPercent(c.FocusX)
	c.FocusY = clampPercent(c.FocusY)
	return c
}

func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Rect computes the crop rectangle on a picture of the given size.
//
// The picture is assumed to be upright already — rotation happens before this.
// The rectangle is the largest one of the chosen shape that fits, divided by
// the zoom, and moved so the focus point sits in the middle of it as far as the
// edges allow. Sliding rather than letting it hang over the edge: a crop that
// went past the border would have to be filled with something, and there is
// nothing honest to fill it with.
func (c Crop) Rect(width, height int) image.Rectangle {
	full := image.Rect(0, 0, width, height)
	if width <= 0 || height <= 0 {
		return full
	}
	c = c.Normalise()

	cw, ch := width, height
	if r, ok := RatioOf(c.Ratio); ok && r.W > 0 && r.H > 0 {
		// The larger of the two candidate rectangles that fit.
		if width*r.H <= height*r.W {
			cw, ch = width, width*r.H/r.W
		} else {
			cw, ch = height*r.W/r.H, height
		}
	}
	cw = cw * 100 / c.Zoom
	ch = ch * 100 / c.Zoom
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	if cw > width {
		cw = width
	}
	if ch > height {
		ch = height
	}

	x := width*c.FocusX/100 - cw/2
	y := height*c.FocusY/100 - ch/2
	x = clamp(x, 0, width-cw)
	y = clamp(y, 0, height-ch)
	return image.Rect(x, y, x+cw, y+ch)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SourceSuffix marks the untouched copy of a cropped picture.
const SourceSuffix = ".original"

// SourceName is where the untouched upload lives once a picture has been
// cropped. Derived rather than stored: one filename in the database, and a
// directory listing that reads plainly.
func SourceName(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext) + SourceSuffix + ext
}

// ErrNotCroppable is returned for a file the pipeline cannot decode.
var ErrNotCroppable = errors.New("dieses Format lässt sich nicht zuschneiden")

// ApplyCrop writes the cropped picture over the served filename, keeping the
// upload beside it.
//
// It returns the new dimensions. Reading the source, decoding, transforming and
// encoding all happen here so the caller — a request handler — deals only in
// paths and numbers.
func ApplyCrop(dir, filename, mimeType string, c Crop, maxMegapixels int) (int, int, error) {
	if !CanMakeVariants(mimeType) {
		return 0, 0, ErrNotCroppable
	}
	if maxMegapixels <= 0 {
		maxMegapixels = DefaultMaxMegapixels
	}

	source := filepath.Join(dir, SourceName(filename))
	served := filepath.Join(dir, filename)

	// First crop: the upload is copied aside. Every later crop reads that copy,
	// so quality is lost once at most and an editor can always go back to the
	// picture they uploaded.
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		if err := copyFile(served, source); err != nil {
			return 0, 0, fmt.Errorf("original sichern: %w", err)
		}
	} else if err != nil {
		return 0, 0, err
	}

	width, height, err := Dimensions(source)
	if err != nil {
		return 0, 0, err
	}
	if width*height > maxMegapixels*1_000_000 {
		return 0, 0, fmt.Errorf("%w: %dx%d exceeds %d megapixels",
			ErrTooManyPixels, width, height, maxMegapixels)
	}

	f, err := os.Open(source)
	if err != nil {
		return 0, 0, err
	}
	src, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return 0, 0, fmt.Errorf("bild lesen: %w", err)
	}

	c = c.Normalise()
	src = rotate(src, c.Rotation)
	rect := c.Rect(src.Bounds().Dx(), src.Bounds().Dy())

	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), src, rect.Min, draw.Src)

	transparent := mimeType == "image/png" || mimeType == "image/webp"
	// Written beside the target and moved into place: a crash halfway through
	// encoding must not leave the served file truncated, because that is the
	// file every visitor is about to request.
	tmp := served + ".neu"
	if _, err := encodeVariant(tmp, out, transparent); err != nil {
		os.Remove(tmp)
		return 0, 0, err
	}
	if err := os.Rename(tmp, served); err != nil {
		os.Remove(tmp)
		return 0, 0, fmt.Errorf("zugeschnittenes Bild einsetzen: %w", err)
	}
	return rect.Dx(), rect.Dy(), nil
}

// RestoreOriginal puts the uploaded picture back and forgets the crop.
func RestoreOriginal(dir, filename string) (int, int, error) {
	source := filepath.Join(dir, SourceName(filename))
	if _, err := os.Stat(source); err != nil {
		// Never cropped: the served file already is the original.
		return Dimensions(filepath.Join(dir, filename))
	}
	if err := copyFile(source, filepath.Join(dir, filename)); err != nil {
		return 0, 0, err
	}
	os.Remove(source)
	return Dimensions(filepath.Join(dir, filename))
}

// rotate turns an image clockwise by a multiple of 90 degrees.
//
// Written out rather than done with a matrix: three cases, each a plain index
// swap, and no interpolation at all — a quarter turn moves pixels, it does not
// resample them, so the result is exactly as sharp as the source.
func rotate(src image.Image, degrees int) image.Image {
	if degrees == 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	var dst *image.RGBA
	switch degrees {
	case 90, 270:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
	default:
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch degrees {
			case 90:
				dst.Set(h-1-y, x, c)
			case 180:
				dst.Set(w-1-x, h-1-y, c)
			case 270:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

// copyFile writes src to dst, replacing what is there.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// WebsiteDir is where one website's files live.
//
// The path was spelled out in four places before this existed, which is three
// too many for something that decides where uploads are written.
func WebsiteDir(dataDir string, websiteID int64) string {
	return filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10))
}

// HasOriginal reports whether an untouched copy of the upload is on disk,
// which is the same question as "has this picture been cropped".
func HasOriginal(dir, filename string) bool {
	_, err := os.Stat(filepath.Join(dir, SourceName(filename)))
	return err == nil
}
