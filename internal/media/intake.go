package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
)

// Taking a file in, once, for every path that takes one in.
//
// The administration's upload does MIME by magic bytes, refuses an SVG that
// loads from somebody else's server, strips EXIF, bounds the size, writes to
// the website's own directory and refuses a duplicate by hash. Every one of
// those exists because of something that went wrong, and every one of them
// matters more for a file a STRANGER sends than for one the operator chose:
// an attachment on a contact form is an upload from the internet.
//
// So the chain lives here and the admin handler calls it, rather than a second
// copy growing beside it that is right on the day it is written.

// Refusal is why a file was not taken.
//
// A reason and its argument, never a finished sentence. The same shape
// field.Reason and the contact form's refusal have, and for the same reason:
// the two callers have two different readers. The administration's upload is
// read by the operator, in the operator's language; an attachment on a public
// form is read by a visitor, in the language of the page. A sentence made here
// would be made before anybody knows which.
type Refusal struct {
	// Code names the reason. Nobody reads it.
	Code string
	// Arg fills the one placeholder the sentence may have: the kind that was
	// refused, or the limit in megabytes.
	Arg any
}

// Error lets a Refusal travel as an error where one is expected. The text is
// the source language and is meant for a log, never for a screen — a screen
// gets the sentence its own caller looked up.
func (r Refusal) Error() string { return "media: " + r.Code }

// The reasons Check can give.
const (
	RefusedKind   = "kind"
	RefusedSize   = "size"
	RefusedRead   = "read"
	RefusedSVGRef = "svg-external"
)

// Limits are what a path is willing to take.
type Limits struct {
	// Image bounds everything that is not a video.
	Image int64
	// Video bounds an mp4, which may legitimately weigh more.
	Video int64
}

// For returns the limit that applies to one kind.
func (l Limits) For(mimeType string) int64 {
	if mimeType == "video/mp4" {
		return l.Video
	}
	return l.Image
}

// Intake is one file on its way in, checked but not yet stored.
//
// The two halves are separate on purpose. A contact form has to run its spam
// traps on the fields BEFORE anything reaches the disk, or a robot with a
// five-megabyte attachment fills the disk whatever the traps decide. Check
// first, Keep second, and a submission that is refused simply never calls Keep.
type Intake struct {
	// MimeType is what the bytes say, not what the browser claimed.
	MimeType string
	// OriginalName is the name the sender's computer gave it.
	OriginalName string
	// Filename is the name it will have here: a fresh one, so two senders
	// cannot collide and nobody can choose a path.
	Filename string
	// Size is what the browser reported, for refusing early.
	Size int64

	content []byte
	limit   int64
}

// Check reads one uploaded file and says whether it may be stored.
//
// Nothing is written. The bytes are held in memory, bounded by the limit that
// applies to their kind, so a caller that decides against keeping the file has
// left nothing behind.
func Check(file multipart.File, header *multipart.FileHeader, limits Limits) (*Intake, error) {
	// The kind comes from the bytes and never from the Content-Type the browser
	// sent, which is whatever the sender decided to write there.
	mimeType, err := ValidateMIME(file, header.Filename)
	if err != nil {
		return nil, Refusal{Code: RefusedKind, Arg: err.Error()}
	}
	limit := limits.For(mimeType)
	if header.Size > limit {
		return nil, Refusal{Code: RefusedSize, Arg: limit >> 20}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, Refusal{Code: RefusedRead}
	}

	// An SVG is markup and can pull a stylesheet or an image from another
	// server, which the template upload already refuses. The same scanner runs
	// here, so the rule holds for every path a file comes in by.
	if mimeType == "image/svg+xml" {
		content, err := io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil {
			return nil, Refusal{Code: RefusedRead}
		}
		if refs := tmplmgr.CheckExternalRefs("upload.svg", string(content)); len(refs) > 0 {
			return nil, Refusal{Code: RefusedSVGRef, Arg: refs[0].URL}
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, Refusal{Code: RefusedRead}
		}
	}

	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, Refusal{Code: RefusedRead}
	}
	if int64(len(content)) > limit {
		return nil, Refusal{Code: RefusedSize, Arg: limit >> 20}
	}

	return &Intake{
		MimeType:     mimeType,
		OriginalName: header.Filename,
		Filename:     GenerateFilename(header.Filename),
		Size:         int64(len(content)),
		content:      content,
		limit:        limit,
	}, nil
}

// Keep writes the file and records it.
//
// It returns the existing record when the same bytes are already stored for
// this website: the same file twice is almost always an accident, and letting
// the card fill with copies nobody can tell apart is worse than saying so.
func (in *Intake) Keep(ctx context.Context, store *Store, websiteID int64, dataDir string) (m *Media, existed bool, err error) {
	// EXIF, XMP and IPTC come off before anything reaches the disk. A photograph
	// taken at home otherwise publishes the photographer's address, and the
	// media route serves the stored bytes verbatim for a year.
	source, stripErr := PrepareUpload(bytes.NewReader(in.content), in.MimeType, in.limit)
	if source == nil {
		return nil, false, fmt.Errorf("upload failed: %w", stripErr)
	}

	destPath := filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10), in.Filename)
	written, hash, err := StoreFile(source, destPath, in.limit)
	if err != nil {
		return nil, false, fmt.Errorf("upload failed: %w", err)
	}

	if existing, err := store.FindByHash(ctx, websiteID, hash); err != nil {
		os.Remove(destPath)
		return nil, false, err
	} else if existing != nil {
		os.Remove(destPath)
		return existing, true, nil
	}

	// The bytes actually written, never the size the sender claimed.
	m, err = store.Create(ctx, websiteID, in.Filename, in.OriginalName, in.MimeType, written, hash)
	if err != nil {
		os.Remove(destPath)
		return nil, false, fmt.Errorf("create media record: %w", err)
	}
	return m, false, nil
}

// Path is where Keep put the file, for a caller that has more to do with it.
func Path(dataDir string, websiteID int64, filename string) string {
	return filepath.Join(dataDir, "media", strconv.FormatInt(websiteID, 10), filename)
}
