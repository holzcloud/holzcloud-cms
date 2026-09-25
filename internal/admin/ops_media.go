package admin

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"path"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The media library and the albums for an AI key.
//
// Every one of these is the function a screen calls, reached without a
// request: the upload goes through keepUpload, the crop through cropMedia, the
// arrows of an album through moveAlbumPicture. What the store does in a single
// call — a description, a focus point, a delete — the tools do through the
// store directly, as the screens do.

// ErrNoWebsite is an id that names no website.
var ErrNoWebsite = errors.New("there is no such website")

// ErrNoMedia is a media id that is missing or belongs to another website. The
// two are not told apart, for the reason album.ErrNotFound gives.
var ErrNoMedia = errors.New("there is no such file on this website")

// OpUploadMedia takes one file in exactly as the upload screen does: media.Check
// with the installation's limits, EXIF stripped, stored, refused as a duplicate
// when the bytes are already there, scaled copies made, the plugins told.
//
// The content arrives as bytes, never as an address to fetch: nothing is
// downloaded from a third party while the application runs.
//
// It answers the stored file; existed when the same bytes were already there
// and nothing was stored; variants when the scaled copies could not be made
// (media.ErrTooManyPixels or another reason — the file is stored regardless);
// and a media.Refusal as err when media.Check turned the file away.
func (h *Handler) OpUploadMedia(ctx context.Context, websiteID int64, name string, content []byte) (m *media.Media, existed bool, variants, err error) {
	ws, err := h.domains.GetWebsite(ctx, websiteID)
	if err != nil {
		return nil, false, nil, err
	}
	if ws == nil {
		return nil, false, nil, ErrNoWebsite
	}
	// Only the last part of the name, as a browser's multipart part gives it:
	// a name is a label here, never a path.
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/"))
	if name == "" || name == "." || name == "/" {
		return nil, false, nil, errors.New("the file needs a name")
	}
	header := &multipart.FileHeader{Filename: name, Size: int64(len(content))}
	up, err := h.keepUpload(ctx, websiteID, memFile{bytes.NewReader(content)}, header)
	if err != nil {
		return nil, false, nil, err
	}
	return up.Media, up.Existed, up.Variants, nil
}

// memFile is a file held in memory, in the shape media.Check reads.
type memFile struct{ *bytes.Reader }

func (memFile) Close() error { return nil }

// ownMedia resolves a media id and checks it belongs to the website.
func (h *Handler) ownMedia(ctx context.Context, websiteID, mediaID int64) (*media.Media, error) {
	m, err := h.mediaStore.GetByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if m == nil || m.WebsiteID != websiteID {
		return nil, ErrNoMedia
	}
	return m, nil
}

// OpCropMedia applies a crop — rotation, shape, how close in, where the
// subject is — exactly as the crop screen's save does. The upload is kept, and
// OpRestoreMedia brings it back. It answers media.ErrNotCroppable and
// media.ErrTooManyPixels as the screen does.
func (h *Handler) OpCropMedia(ctx context.Context, websiteID, mediaID int64, crop media.Crop) (*media.Media, error) {
	m, err := h.ownMedia(ctx, websiteID, mediaID)
	if err != nil {
		return nil, err
	}
	if _, _, err := h.cropMedia(ctx, websiteID, m, crop); err != nil {
		return nil, err
	}
	return h.mediaStore.GetByID(ctx, mediaID)
}

// OpRestoreMedia puts the uploaded picture back, as the crop screen's reset
// does. The focus point stays.
func (h *Handler) OpRestoreMedia(ctx context.Context, websiteID, mediaID int64) (*media.Media, error) {
	m, err := h.ownMedia(ctx, websiteID, mediaID)
	if err != nil {
		return nil, err
	}
	if err := h.restoreOriginal(ctx, websiteID, m); err != nil {
		return nil, err
	}
	return h.mediaStore.GetByID(ctx, mediaID)
}

// OpAlbums is the album store the screens use, nil in a build without albums.
// Every one of its calls takes the website, so a caller cannot forget it.
func (h *Handler) OpAlbums() *album.Store { return h.albumStore }

// OpAddAlbumPicture appends one picture to an album, with the check the
// album screen makes: the file must be a picture of this website's library.
func (h *Handler) OpAddAlbumPicture(ctx context.Context, websiteID, albumID, mediaID int64, alt, caption string) (int64, error) {
	if h.albumStore == nil {
		return 0, errors.New("this installation has no albums")
	}
	if refused := h.requireOwnPicture(ctx, websiteID, mediaID); refused != "" {
		return 0, errors.New(refused)
	}
	return h.albumStore.AddItem(ctx, websiteID, albumID, mediaID,
		strings.TrimSpace(alt), strings.TrimSpace(caption))
}

// OpUpdateAlbumPicture repoints one picture row and rewrites its description
// and caption, with the album screen's check.
func (h *Handler) OpUpdateAlbumPicture(ctx context.Context, websiteID, albumID, itemID, mediaID int64, alt, caption string) error {
	if h.albumStore == nil {
		return errors.New("this installation has no albums")
	}
	if refused := h.requireOwnPicture(ctx, websiteID, mediaID); refused != "" {
		return errors.New(refused)
	}
	return h.albumStore.UpdateItem(ctx, websiteID, albumID, itemID, mediaID,
		strings.TrimSpace(alt), strings.TrimSpace(caption))
}

// OpMoveAlbumPicture moves one picture a place up or down, as the arrows on
// the album screen do. False and no error means it already stands at that end.
func (h *Handler) OpMoveAlbumPicture(ctx context.Context, websiteID, albumID, itemID int64, direction string) (bool, error) {
	if h.albumStore == nil {
		return false, errors.New("this installation has no albums")
	}
	return h.moveAlbumPicture(ctx, websiteID, albumID, itemID, direction)
}
