package media

import (
	"strconv"
	"time"
)

// Media represents an uploaded media file.
type Media struct {
	ID           int64
	WebsiteID    int64
	Filename     string
	OriginalName string
	MimeType     string
	SizeBytes    int64
	CreatedAt    time.Time

	// AltText describes the image for someone who cannot see it. Without it a
	// theme has nothing to put in the alt attribute, which is the single most
	// common accessibility defect on a company website.
	AltText string
	// Caption is the visible text under an image, where a theme shows one.
	Caption string
	// ContentHash is the SHA-256 of the stored bytes, used to notice a file
	// that has already been uploaded.
	ContentHash string

	// Width and Height are the served picture's pixel dimensions, zero for
	// anything that was never measured — a PDF, or an image uploaded before the
	// variant pipeline existed. After a crop they are the crop's.
	Width  int
	Height int

	// Crop is how this picture is framed: what was cut away, and where the
	// subject is. The zero value is an untouched upload centred on itself.
	Crop Crop

	// Version counts how often the served bytes changed. It rides along in the
	// URL, because a media address is cached for a year as immutable and that
	// is only true as long as the bytes stay put — cropping made it untrue.
	Version int
}

// versionQuery is what makes a changed picture a different address.
func (m Media) versionQuery() string {
	if m.Version <= 0 {
		return ""
	}
	return "?v=" + strconv.Itoa(m.Version)
}

// IsCropped reports whether the served file differs from the upload.
func (m Media) IsCropped() bool { return !m.Crop.Empty() }

// FocusCSS is the object-position value that keeps the subject in frame when a
// theme squeezes this picture into a fixed shape.
//
// Empty for a picture focused on its own middle, which is what a browser does
// anyway — an attribute that changes nothing is one more thing on every page.
func (m Media) FocusCSS() string {
	if m.Crop.FocusX == 50 && m.Crop.FocusY == 50 {
		return ""
	}
	return strconv.Itoa(clampPercent(m.Crop.FocusX)) + "% " +
		strconv.Itoa(clampPercent(m.Crop.FocusY)) + "%"
}

// IsImage reports whether the file can be shown as a picture.
func (m Media) IsImage() bool {
	switch m.MimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml":
		return true
	}
	return false
}

// IsVideo reports whether the file can be played in a video element.
func (m Media) IsVideo() bool { return m.MimeType == "video/mp4" }

// NeedsAltText reports whether this is an image with no description yet.
func (m Media) NeedsAltText() bool { return m.IsImage() && m.AltText == "" }

// MarkdownRef is the snippet an editor pastes into a page.
func (m Media) MarkdownRef() string {
	prefix := "["
	if m.IsImage() {
		prefix = "!["
	}
	alt := m.AltText
	if alt == "" {
		alt = m.OriginalName
	}
	return prefix + alt + "](" + m.URL() + ")"
}

// URL is the public, same-origin path of the file.
func (m Media) URL() string {
	return m.Path() + m.versionQuery()
}

// Path is the address without the version, which is what the router matches and
// what the usage scan compares against.
func (m Media) Path() string {
	return "/media/" + strconv.FormatInt(m.WebsiteID, 10) + "/" + m.Filename
}

// ThumbURL is what the admin grid and the media picker load.
//
// The name is derived rather than looked up: the variant file name is a pure
// function of the original's, so showing forty thumbnails costs no extra query.
// Anything without a generated thumbnail — an SVG, a small logo, a file from
// before the pipeline existed — falls back to the original, which is correct
// and only as slow as it was before.
func (m Media) ThumbURL() string {
	if !m.HasThumb() {
		return m.URL()
	}
	name := variantFilename(m.Filename, "thumb", m.MimeType != "image/jpeg")
	return "/media/" + strconv.FormatInt(m.WebsiteID, 10) + "/" + name + m.versionQuery()
}

// HasThumb reports whether a scaled-down copy was generated for this file.
func (m Media) HasThumb() bool {
	return CanMakeVariants(m.MimeType) && m.Width > variantSpecs[0].Width
}
