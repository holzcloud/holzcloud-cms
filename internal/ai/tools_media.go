package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// Media and albums: upload, describe, focus, crop, delete; albums and their
// pictures.
//
// The upload and the crop are the admin screens' own functions, reached
// through Ops: a file from an assistant passes media.Check, loses its EXIF,
// is refused as a duplicate and gets its scaled copies exactly as one from the
// upload form does. A second intake here would be the one that forgot the SVG
// scanner.
//
// A file arrives as base64 and never as an address. Fetching a picture from
// somewhere on the net while the application runs is what this project rules
// out, and an upload tool that took a URL would be the way around that rule.
//
// Every id comes from outside. A media id is looked up and its website checked
// against the key; an album id is only ever asked for together with its
// website, because every call of the album store is scoped by one.

// mediaOps are the admin functions this area goes through.
type mediaOps interface {
	OpUploadMedia(ctx context.Context, websiteID int64, name string, content []byte) (m *media.Media, existed bool, variants, err error)
	OpCropMedia(ctx context.Context, websiteID, mediaID int64, crop media.Crop) (*media.Media, error)
	OpRestoreMedia(ctx context.Context, websiteID, mediaID int64) (*media.Media, error)
}

// albumOps are the album functions. OpAlbums may answer nil: a build without
// albums.
type albumOps interface {
	OpAlbums() *album.Store
	OpAddAlbumPicture(ctx context.Context, websiteID, albumID, mediaID int64, alt, caption string) (int64, error)
	OpUpdateAlbumPicture(ctx context.Context, websiteID, albumID, itemID, mediaID int64, alt, caption string) error
	OpMoveAlbumPicture(ctx context.Context, websiteID, albumID, itemID int64, direction string) (bool, error)
}

var errNoMediaOps = errors.New("uploading and cropping are not available in this installation")

// albumsOf hands back the album functions and their store, or says they are
// missing.
func albumsOf(d Deps) (albumOps, *album.Store, error) {
	ops, ok := d.Ops.(albumOps)
	if !ok || ops.OpAlbums() == nil {
		return nil, nil, errors.New("albums are not available in this installation")
	}
	return ops, ops.OpAlbums(), nil
}

func mediaTools(d Deps) []Tool {
	return []Tool{
		getMediaLibrary(d),
		listMediaCollection(d),
		getMedia(d),
		uploadMedia(d),
		updateMedia(d),
		setMediaFocus(d),
		cropMedia(d),
		restoreMediaOriginal(d),
		deleteMedia(d),
		listAlbums(d),
		getAlbum(d),
		createAlbum(d),
		renameAlbum(d),
		deleteAlbum(d),
		addAlbumImages(d),
		updateAlbumImage(d),
		removeAlbumImage(d),
		moveAlbumImage(d),
	}
}

// --- shared -----------------------------------------------------------------

// ownMedia resolves a media id for this key: the file must exist and its
// website must be one the key may touch.
func ownMedia(c Call, d Deps, id int64) (*media.Media, error) {
	m, err := d.Media.GetByID(c.Ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("there is no such file")
	}
	if err := c.Scope.MaySee(m.WebsiteID); err != nil {
		return nil, err
	}
	return m, nil
}

// mediaEntry is one file as an assistant reads it.
func mediaEntry(m media.Media) map[string]any {
	e := map[string]any{
		"id": m.ID, "website": m.WebsiteID, "file": m.OriginalName, "url": m.URL(),
		"mime": m.MimeType, "size_bytes": m.SizeBytes, "description": m.AltText,
		"caption": m.Caption, "markdown": m.MarkdownRef(),
		"uploaded": m.CreatedAt.UTC().Format(timeLayout),
	}
	if m.Width > 0 && m.Height > 0 {
		e["width"], e["height"] = m.Width, m.Height
	}
	if m.IsImage() {
		e["focus"] = map[string]any{"x": m.Crop.FocusX, "y": m.Crop.FocusY}
	}
	if m.IsCropped() {
		e["crop"] = map[string]any{
			"ratio": m.Crop.Ratio, "zoom": m.Crop.Zoom, "rotation": m.Crop.Rotation,
		}
	}
	if m.NeedsAltText() {
		e["note"] = "This image has no description. Whoever puts it into a page should " +
			"write one; update_media stores it."
	}
	return e
}

// refusalText says in plain words why media.Check refused a file.
func refusalText(err error) string {
	var ref media.Refusal
	if !errors.As(err, &ref) {
		return "the file could not be accepted: " + err.Error()
	}
	switch ref.Code {
	case media.RefusedKind:
		return fmt.Sprintf("this kind of file is not allowed: %v", ref.Arg)
	case media.RefusedSize:
		return fmt.Sprintf("the file is larger than allowed (%v MB)", ref.Arg)
	case media.RefusedSVGRef:
		return fmt.Sprintf("the SVG file loads %v from somebody else's server; that is not allowed", ref.Arg)
	}
	return "the file could not be read"
}

// cropError puts the crop pipeline's two named refusals into words.
func cropError(err error, d Deps) error {
	switch {
	case errors.Is(err, media.ErrNotCroppable):
		return errors.New("this kind of file cannot be cropped; only JPEG, PNG and WebP pictures can")
	case errors.Is(err, media.ErrTooManyPixels):
		return fmt.Errorf("the image is too large to crop (limit: %d megapixels)", d.Limits.MaxMegapixels)
	}
	return err
}

func websiteProp() Property { return Property{Type: "integer", Description: "id of the website"} }

func albumProp() Property { return Property{Type: "integer", Description: "id of the album"} }

// --- the library ------------------------------------------------------------

func getMediaLibrary(d Deps) Tool {
	return Tool{
		Name: "get_media_library",
		Description: "Gives an overview of a website's media library: how many files there are " +
			"in each collection (all, images, videos, documents, unused — on no page, and " +
			"no_description — images without a description) and the albums with how many " +
			"pictures each holds. list_media_collection lists the files of one collection.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteProp()},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			counts, err := d.Media.Counts(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := map[string]any{"collections": map[string]any{
				"all": counts.All, "images": counts.Images, "videos": counts.Videos,
				"documents": counts.Documents, "unused": counts.Unused,
				"no_description": counts.NoAlt,
			}}
			if _, store, err := albumsOf(d); err == nil {
				albums, err := store.List(c.Ctx, a.Website)
				if err != nil {
					return nil, err
				}
				n, err := store.ItemCounts(c.Ctx, a.Website)
				if err != nil {
					return nil, err
				}
				list := make([]map[string]any, 0, len(albums))
				for _, al := range albums {
					list = append(list, map[string]any{"id": al.ID, "name": al.Name, "pictures": n[al.ID]})
				}
				out["albums"] = list
			}
			return out, nil
		},
	}
}

func listMediaCollection(d Deps) Tool {
	return Tool{
		Name: "list_media_collection",
		Description: "Lists the files of one collection of a website's media library, newest " +
			"first, a page at a time: images, videos, documents, unused (files no page " +
			"shows — the ones that could go) or no_description (images that still need a " +
			"description). Hands back the total, so further pages can be asked for.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(),
				"collection": {Type: "string", Description: "which files; all by default",
					Enum: []string{"all", "images", "videos", "documents", "unused", "no_description"}},
				"query": {Type: "string", Description: "file name or description"},
				"page":  {Type: "integer", Description: "which page, from 1"},
				"limit": {Type: "integer", Description: "files per page, 50 by default"},
			},
			Required: []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website    int64  `json:"website"`
				Collection string `json:"collection"`
				Query      string `json:"query"`
				Page       int    `json:"page"`
				Count      int    `json:"limit"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			// The same filter the library screen builds from its collection
			// list, so a collection means here what it means there.
			f := media.Filter{Query: strings.TrimSpace(a.Query)}
			switch a.Collection {
			case "", "all":
			case "images":
				f.MimePrefix = "image/"
			case "videos":
				f.MimePrefix = "video/"
			case "documents":
				f.MimePrefix = "application/"
			case "unused":
				f.Unused = true
			case "no_description":
				f.NoAlt = true
			default:
				return nil, fmt.Errorf("there is no collection %q", a.Collection)
			}
			perPage := clampCount(a.Count)
			items, total, err := d.Media.List(c.Ctx, a.Website, f, max(a.Page, 1), perPage)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(items))
			for _, m := range items {
				out = append(out, mediaEntry(m))
			}
			return map[string]any{"media": out, "total": total, "page": max(a.Page, 1)}, nil
		},
	}
}

func getMedia(d Deps) Tool {
	return Tool{
		Name: "get_media",
		Description: "Fetches everything about one file: its URL, size, description, caption, " +
			"focus point and crop — and where it is used: the pages that show it and the " +
			"albums it is in. Check that before deleting it.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"id": {Type: "integer", Description: "id of the file"}},
			Required:   []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			out := mediaEntry(*m)
			pages, err := d.Media.UsedOnPages(c.Ctx, m.ID)
			if err != nil {
				return nil, err
			}
			if pages == nil {
				pages = []string{}
			}
			out["used_on_pages"] = pages

			// Albums are few and hand-made, and an album is capped at
			// album.MaxItems pictures, so asking each is cheap enough — and a
			// file that sits only in an album is not "unused" to the person
			// about to delete it.
			if _, store, err := albumsOf(d); err == nil {
				albums, err := store.List(c.Ctx, m.WebsiteID)
				if err != nil {
					return nil, err
				}
				in := []map[string]any{}
				for _, al := range albums {
					pictures, err := store.Pictures(c.Ctx, m.WebsiteID, al.ID)
					if err != nil {
						return nil, err
					}
					for _, p := range pictures {
						if p.Item.MediaID == m.ID {
							in = append(in, map[string]any{"id": al.ID, "name": al.Name})
							break
						}
					}
				}
				out["in_albums"] = in
			}
			return out, nil
		},
	}
}

// --- changing files ---------------------------------------------------------

func uploadMedia(d Deps) Tool {
	return Tool{
		Name:   "upload_media",
		Writes: true,
		Description: "Uploads one file into a website's media library: a JPEG, PNG, WebP, GIF, " +
			"SVG, PDF or MP4. The content is sent as base64 — a web address cannot be given, " +
			"this installation fetches nothing from elsewhere. The file goes through the same " +
			"checks as an upload on the admin side: the kind is read from the bytes, the size " +
			"is limited, location data is removed from photographs, and a file that is " +
			"already in the library is not stored a second time. Give every picture a " +
			"description (alt_text) of what it shows.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(),
				"file_name": {Type: "string", Description: "the file's name with its ending, " +
					"such as workshop.jpg"},
				"content_base64": {Type: "string", Description: "the file's bytes, base64-encoded"},
				"alt_text": {Type: "string", Description: "what the picture shows, for someone " +
					"who cannot see it"},
				"caption": {Type: "string", Description: "the visible text under the picture, " +
					"where the design shows one"},
			},
			Required: []string{"website", "file_name", "content_base64"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Name    string `json:"file_name"`
				Content string `json:"content_base64"`
				Alt     string `json:"alt_text"`
				Caption string `json:"caption"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			ops, ok := d.Ops.(mediaOps)
			if !ok {
				return nil, errNoMediaOps
			}
			content, err := decodeBase64(a.Content)
			if err != nil {
				return nil, err
			}

			m, existed, variants, err := ops.OpUploadMedia(c.Ctx, a.Website, a.Name, content)
			var ref media.Refusal
			switch {
			case errors.As(err, &ref):
				return nil, errors.New(refusalText(err))
			case err != nil:
				return nil, err
			case existed:
				out := mediaEntry(*m)
				out["note"] = "This file already exists in the library as \"" +
					m.OriginalName + "\" — nothing was uploaded. Use this one."
				return out, nil
			}

			alt, caption := strings.TrimSpace(a.Alt), strings.TrimSpace(a.Caption)
			if alt != "" || caption != "" {
				if err := d.Media.UpdateMeta(c.Ctx, m.ID, alt, caption); err != nil {
					return nil, err
				}
				m.AltText, m.Caption = alt, caption
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionMediaUpload, EntityType: "media", EntityID: m.ID,
				Metadata: map[string]any{"file": m.OriginalName},
			})
			c.Log.Info("ai uploaded media", "key", c.Scope.Name, "media", m.ID, "website", a.Website)

			out := mediaEntry(*m)
			var notes []string
			switch {
			case variants == nil:
			case errors.Is(variants, media.ErrTooManyPixels):
				notes = append(notes, fmt.Sprintf("The image is too large for scaled copies "+
					"(limit: %d megapixels); it is stored and usable, but heavy.", d.Limits.MaxMegapixels))
			default:
				notes = append(notes, "The scaled copies could not be made; the file is stored "+
					"and usable, but heavy.")
			}
			if m.NeedsAltText() {
				notes = append(notes, "This image has no description yet; update_media stores one.")
			}
			if len(notes) > 0 {
				out["note"] = strings.Join(notes, " ")
			} else {
				delete(out, "note")
			}
			return out, nil
		},
	}
}

// decodeBase64 reads the content of an upload. Standard and URL alphabets, with
// or without padding, and a data: prefix is cut off: an assistant produces
// every one of those, and refusing a file over its spelling helps nobody.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[i+1:]
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	if s == "" {
		return nil, errors.New("the file is empty")
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("content_base64 is not valid base64")
}

func updateMedia(d Deps) Tool {
	return Tool{
		Name:   "update_media",
		Writes: true,
		Description: "Changes the description (alt text) or the caption of a file. What is not " +
			"given stays as it is; an empty string clears it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "id of the file"},
				"alt_text": {Type: "string", Description: "what the picture shows, for someone " +
					"who cannot see it"},
				"caption": {Type: "string", Description: "the visible text under the picture"},
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64   `json:"id"`
				Alt     *string `json:"alt_text"`
				Caption *string `json:"caption"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			alt, caption := m.AltText, m.Caption
			if a.Alt != nil {
				alt = strings.TrimSpace(*a.Alt)
			}
			if a.Caption != nil {
				caption = strings.TrimSpace(*a.Caption)
			}
			if err := d.Media.UpdateMeta(c.Ctx, m.ID, alt, caption); err != nil {
				return nil, err
			}
			changed(d, c, m.WebsiteID, activity.Entry{
				Action: activity.ActionMediaUpdate, EntityType: "media", EntityID: m.ID,
			})
			m.AltText, m.Caption = alt, caption
			return mediaEntry(*m), nil
		},
	}
}

func setMediaFocus(d Deps) Tool {
	return Tool{
		Name:   "set_media_focus",
		Writes: true,
		Description: "Says where the subject of a picture is, in percent from the top left " +
			"(50/50 is the middle). A design that has to squeeze the picture into a fixed " +
			"shape keeps that point in view. The picture itself is not changed; crop_media " +
			"cuts it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "id of the picture"},
				"x":  {Type: "integer", Description: "from the left edge, 0 to 100"},
				"y":  {Type: "integer", Description: "from the top edge, 0 to 100"},
			},
			Required: []string{"id", "x", "y"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID int64 `json:"id"`
				X  int64 `json:"x"`
				Y  int64 `json:"y"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			if !m.IsImage() {
				return nil, errors.New("only a picture has a focus point")
			}
			if a.X < 0 || a.X > 100 || a.Y < 0 || a.Y > 100 {
				return nil, errors.New("x and y are percentages from 0 to 100")
			}
			if err := d.Media.SaveFocus(c.Ctx, m.ID, a.X, a.Y); err != nil {
				return nil, err
			}
			changed(d, c, m.WebsiteID, activity.Entry{
				Action: activity.ActionMediaCrop, EntityType: "media", EntityID: m.ID,
				Metadata: map[string]any{"focus_x": a.X, "focus_y": a.Y},
			})
			after, err := d.Media.GetByID(c.Ctx, m.ID)
			if err != nil || after == nil {
				return nil, errors.New("the file is gone")
			}
			return mediaEntry(*after), nil
		},
	}
}

func cropMedia(d Deps) Tool {
	ratios := make([]string, 0, len(media.Ratios))
	for _, r := range media.Ratios {
		if r.Key != "" {
			ratios = append(ratios, r.Key)
		}
	}
	ratios = append(ratios, "original")
	return Tool{
		Name:   "crop_media",
		Writes: true,
		Description: "Crops or turns a picture, as the crop screen on the admin side does: a " +
			"shape, how close in, a quarter turn, centred on the focus point. Every crop starts " +
			"from the uploaded picture again, which is kept; restore_media_original brings it " +
			"back. Everywhere the picture is shown changes with it. JPEG, PNG and WebP only.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id": {Type: "integer", Description: "id of the picture"},
				"ratio": {Type: "string", Description: "the shape as width-height; original " +
					"keeps the picture's own shape", Enum: ratios},
				"zoom": {Type: "integer", Description: "how close in, in percent: 100 is the " +
					"largest crop of that shape, 200 half as wide; up to 300"},
				"rotation": {Type: "integer", Description: "a clockwise turn in degrees, applied " +
					"first: 0, 90, 180 or 270"},
				"focus_x": {Type: "integer", Description: "where the subject is, from the left, " +
					"0 to 100; without it the stored focus point"},
				"focus_y": {Type: "integer", Description: "where the subject is, from the top, " +
					"0 to 100; without it the stored focus point"},
			},
			Required: []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID       int64  `json:"id"`
				Ratio    string `json:"ratio"`
				Zoom     int    `json:"zoom"`
				Rotation int    `json:"rotation"`
				FocusX   *int   `json:"focus_x"`
				FocusY   *int   `json:"focus_y"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, ok := d.Ops.(mediaOps)
			if !ok {
				return nil, errNoMediaOps
			}

			crop := media.Crop{Zoom: a.Zoom, Rotation: a.Rotation, FocusX: m.Crop.FocusX, FocusY: m.Crop.FocusY}
			switch a.Ratio {
			case "", "original":
			default:
				if _, ok := media.RatioOf(a.Ratio); !ok {
					return nil, fmt.Errorf("there is no shape %q", a.Ratio)
				}
				crop.Ratio = a.Ratio
			}
			if crop.Rotation%90 != 0 || crop.Rotation < 0 || crop.Rotation > 270 {
				return nil, errors.New("rotation is 0, 90, 180 or 270")
			}
			if a.FocusX != nil {
				crop.FocusX = *a.FocusX
			}
			if a.FocusY != nil {
				crop.FocusY = *a.FocusY
			}
			if crop.Zoom != 0 && (crop.Zoom < 100 || crop.Zoom > media.MaxZoom) {
				return nil, fmt.Errorf("zoom is between 100 and %d", media.MaxZoom)
			}

			after, err := ops.OpCropMedia(c.Ctx, m.WebsiteID, m.ID, crop)
			if err != nil {
				return nil, cropError(err, d)
			}
			changed(d, c, m.WebsiteID, activity.Entry{
				Action: activity.ActionMediaCrop, EntityType: "media", EntityID: m.ID,
				Metadata: map[string]any{"ratio": crop.Ratio, "zoom": crop.Zoom, "rotation": crop.Rotation},
			})
			return mediaEntry(*after), nil
		},
	}
}

func restoreMediaOriginal(d Deps) Tool {
	return Tool{
		Name:   "restore_media_original",
		Writes: true,
		Description: "Undoes every crop and turn of a picture and puts the uploaded picture " +
			"back. The focus point stays.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"id": {Type: "integer", Description: "id of the picture"}},
			Required:   []string{"id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			ops, ok := d.Ops.(mediaOps)
			if !ok {
				return nil, errNoMediaOps
			}
			if !media.CanMakeVariants(m.MimeType) {
				return nil, errors.New("this kind of file is never cropped")
			}
			after, err := ops.OpRestoreMedia(c.Ctx, m.WebsiteID, m.ID)
			if err != nil {
				return nil, err
			}
			changed(d, c, m.WebsiteID, activity.Entry{
				Action: activity.ActionMediaCrop, EntityType: "media", EntityID: m.ID,
				Metadata: map[string]any{"restored": true},
			})
			return mediaEntry(*after), nil
		},
	}
}

func deleteMedia(d Deps) Tool {
	return Tool{
		Name:   "delete_media",
		Writes: true,
		Description: "Deletes a file from the media library for good, with its scaled copies. " +
			"Needs confirm: true. A file that a page still shows is refused and the pages are " +
			"named; force: true deletes it anyway and leaves those pages with a missing " +
			"picture — only do that when you were expressly asked to. Albums lose the " +
			"picture without asking.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"id":      {Type: "integer", Description: "id of the file"},
				"confirm": {Type: "boolean", Description: "must be true: the file cannot be brought back"},
				"force":   {Type: "boolean", Description: "delete even though pages still show it"},
			},
			Required: []string{"id", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
				Force   bool  `json:"force"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			m, err := ownMedia(c, d, a.ID)
			if err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("deleting a file cannot be undone; call again with " +
					"confirm: true once that is really meant")
			}
			err = d.Media.Delete(c.Ctx, m.ID, d.Limits.DataDir, a.Force)
			var inUse *media.InUseError
			if errors.As(err, &inUse) {
				return nil, fmt.Errorf("the file is still shown on these pages: %s. Take it off "+
					"them first, or call again with force: true if deleting it anyway was "+
					"expressly asked for", strings.Join(inUse.Pages, ", "))
			}
			if err != nil {
				return nil, err
			}
			changed(d, c, m.WebsiteID, activity.Entry{
				Action: activity.ActionMediaDelete, EntityType: "media", EntityID: m.ID,
				Metadata: map[string]any{"file": m.OriginalName, "force": a.Force},
			})
			c.Log.Info("ai deleted media", "key", c.Scope.Name, "media", m.ID, "force", a.Force)
			return map[string]any{"deleted": m.ID, "file": m.OriginalName}, nil
		},
	}
}

// --- albums -----------------------------------------------------------------

// albumPictures is an album's pictures in order, each with its file.
func albumPictures(c Call, d Deps, store *album.Store, websiteID, albumID int64) ([]map[string]any, error) {
	pictures, err := store.Pictures(c.Ctx, websiteID, albumID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(pictures))
	for i, p := range pictures {
		e := map[string]any{
			"item": p.ID, "position": i + 1, "media": p.Item.MediaID,
			"alt_text": p.Item.Alt, "caption": p.Item.Caption,
		}
		if m, err := d.Media.GetByID(c.Ctx, p.Item.MediaID); err == nil && m != nil {
			e["file"], e["url"] = m.OriginalName, m.URL()
			if p.Item.Alt == "" {
				e["library_description"] = m.AltText
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// findAlbum resolves an album of a website the key may touch.
func findAlbum(c Call, store *album.Store, websiteID, albumID int64) (*album.Album, error) {
	if err := c.Scope.MaySee(websiteID); err != nil {
		return nil, err
	}
	a, err := store.Get(c.Ctx, websiteID, albumID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errors.New("there is no such album on this website")
	}
	return a, nil
}

func albumBrief(a album.Album) map[string]any {
	return map[string]any{
		"id": a.ID, "website": a.WebsiteID, "name": a.Name, "slug": a.Slug,
		"updated": a.UpdatedAt.UTC().Format(timeLayout),
	}
}

func listAlbums(d Deps) Tool {
	return Tool{
		Name: "list_albums",
		Description: "Lists the albums of a website with their id, name, address (slug) and how " +
			"many pictures each holds. An album is a selection from the media library that a " +
			"gallery on a page can show.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteProp()},
			Required:   []string{"website"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			albums, err := store.List(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			counts, err := store.ItemCounts(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(albums))
			for _, al := range albums {
				e := albumBrief(al)
				e["pictures"] = counts[al.ID]
				out = append(out, e)
			}
			return map[string]any{"albums": out}, nil
		},
	}
}

func getAlbum(d Deps) Tool {
	return Tool{
		Name: "get_album",
		Description: "Fetches one album with its pictures in order. Each picture has an item id " +
			"— that is what update_album_image, remove_album_image and move_album_image take, " +
			"not the id of the file.",
		InputSchema: Schema{
			Type:       "object",
			Properties: map[string]Property{"website": websiteProp(), "id": albumProp()},
			Required:   []string{"website", "id"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				ID      int64 `json:"id"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.ID)
			if err != nil {
				return nil, err
			}
			return albumWithPictures(c, d, store, al)
		},
	}
}

func albumWithPictures(c Call, d Deps, store *album.Store, al *album.Album) (map[string]any, error) {
	pictures, err := albumPictures(c, d, store, al.WebsiteID, al.ID)
	if err != nil {
		return nil, err
	}
	out := albumBrief(*al)
	out["pictures"] = pictures
	out["max_pictures"] = album.MaxItems
	return out, nil
}

func createAlbum(d Deps) Tool {
	return Tool{
		Name:   "create_album",
		Writes: true,
		Description: "Creates an empty album on a website. Its address (slug) is made from the " +
			"name once and does not change later. add_album_images fills it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(),
				"name": {Type: "string", Description: fmt.Sprintf("the album's name, at most %d "+
					"characters", album.MaxNameLength)},
			},
			Required: []string{"website", "name"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Name    string `json:"name"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			ws, err := d.Domains.GetWebsite(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			if ws == nil {
				return nil, errors.New("there is no such website")
			}
			al, err := store.Create(c.Ctx, a.Website, a.Name)
			if err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionAlbumCreate, EntityType: "album", EntityID: al.ID,
				Metadata: map[string]any{"name": al.Name},
			})
			return albumBrief(*al), nil
		},
	}
}

func renameAlbum(d Deps) Tool {
	return Tool{
		Name:   "rename_album",
		Writes: true,
		Description: "Gives an album a new name. Its address (slug) stays, so every gallery " +
			"that shows it keeps showing it.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "id": albumProp(),
				"name": {Type: "string", Description: "the new name"},
			},
			Required: []string{"website", "id", "name"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				ID      int64  `json:"id"`
				Name    string `json:"name"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.ID)
			if err != nil {
				return nil, err
			}
			if err := store.Rename(c.Ctx, a.Website, al.ID, a.Name); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionAlbumRename, EntityType: "album", EntityID: al.ID,
				Metadata: map[string]any{"from": al.Name},
			})
			after, err := store.Get(c.Ctx, a.Website, al.ID)
			if err != nil || after == nil {
				return nil, errors.New("the album is gone")
			}
			return albumBrief(*after), nil
		},
	}
}

func deleteAlbum(d Deps) Tool {
	return Tool{
		Name:   "delete_album",
		Writes: true,
		Description: "Deletes an album. Needs confirm: true. The files stay in the media " +
			"library; a gallery that showed the album shows nothing from then on.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "id": albumProp(),
				"confirm": {Type: "boolean", Description: "must be true: the album and its " +
					"order cannot be brought back"},
			},
			Required: []string{"website", "id", "confirm"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				ID      int64 `json:"id"`
				Confirm bool  `json:"confirm"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.ID)
			if err != nil {
				return nil, err
			}
			if !a.Confirm {
				return nil, errors.New("deleting an album cannot be undone; call again with " +
					"confirm: true once that is really meant")
			}
			if err := store.Delete(c.Ctx, a.Website, al.ID); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionAlbumDelete, EntityType: "album", EntityID: al.ID,
				Metadata: map[string]any{"name": al.Name},
			})
			return map[string]any{"deleted": al.ID, "name": al.Name}, nil
		},
	}
}

func addAlbumImages(d Deps) Tool {
	return Tool{
		Name:   "add_album_images",
		Writes: true,
		Description: "Appends pictures from the website's media library to the end of an album, " +
			"in the order given. Only pictures of this website can go in, no videos or " +
			"documents. Each may carry its own description and caption for this album; " +
			"without one the library's description is used. Whatever is refused is named " +
			"and the rest still goes in.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "album": albumProp(),
				"images": {Type: "array", Description: "the pictures to add", Items: &Property{
					Type: "object",
					Properties: map[string]Property{
						"media":    {Type: "integer", Description: "id of the file"},
						"alt_text": {Type: "string", Description: "description in this album"},
						"caption":  {Type: "string", Description: "caption in this album"},
					},
					Required: []string{"media"},
				}},
			},
			Required: []string{"website", "album", "images"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Album   int64 `json:"album"`
				Images  []struct {
					Media   int64  `json:"media"`
					Alt     string `json:"alt_text"`
					Caption string `json:"caption"`
				} `json:"images"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.Album)
			if err != nil {
				return nil, err
			}
			if len(a.Images) == 0 {
				return nil, errors.New("name at least one picture")
			}
			var added []int64
			refused := []map[string]any{}
			for _, img := range a.Images {
				id, err := ops.OpAddAlbumPicture(c.Ctx, a.Website, al.ID, img.Media, img.Alt, img.Caption)
				if err != nil {
					refused = append(refused, map[string]any{"media": img.Media, "reason": err.Error()})
					continue
				}
				added = append(added, id)
			}
			if len(added) > 0 {
				changed(d, c, a.Website, activity.Entry{
					Action: activity.ActionAlbumPicture, EntityType: "album", EntityID: al.ID,
					Metadata: map[string]any{"added": len(added)},
				})
			}
			out, err := albumWithPictures(c, d, store, al)
			if err != nil {
				return nil, err
			}
			out["added"] = len(added)
			if len(refused) > 0 {
				out["refused"] = refused
			}
			return out, nil
		},
	}
}

// pictureOf finds one row of an album.
func pictureOf(c Call, store *album.Store, websiteID, albumID, itemID int64) (*album.Picture, error) {
	pictures, err := store.Pictures(c.Ctx, websiteID, albumID)
	if err != nil {
		return nil, err
	}
	for _, p := range pictures {
		if p.ID == itemID {
			return &p, nil
		}
	}
	return nil, errors.New("there is no such picture in this album; get_album lists the item ids")
}

func updateAlbumImage(d Deps) Tool {
	return Tool{
		Name:   "update_album_image",
		Writes: true,
		Description: "Changes one picture of an album: its description, its caption, or which " +
			"file it shows. What is not given stays. Takes the item id from get_album.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "album": albumProp(),
				"item":     {Type: "integer", Description: "item id of the picture in the album"},
				"media":    {Type: "integer", Description: "id of another file to show instead"},
				"alt_text": {Type: "string", Description: "description in this album"},
				"caption":  {Type: "string", Description: "caption in this album"},
			},
			Required: []string{"website", "album", "item"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64   `json:"website"`
				Album   int64   `json:"album"`
				Item    int64   `json:"item"`
				Media   *int64  `json:"media"`
				Alt     *string `json:"alt_text"`
				Caption *string `json:"caption"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.Album)
			if err != nil {
				return nil, err
			}
			p, err := pictureOf(c, store, a.Website, al.ID, a.Item)
			if err != nil {
				return nil, err
			}
			mediaID, alt, caption := p.Item.MediaID, p.Item.Alt, p.Item.Caption
			if a.Media != nil {
				mediaID = *a.Media
			}
			if a.Alt != nil {
				alt = *a.Alt
			}
			if a.Caption != nil {
				caption = *a.Caption
			}
			if err := ops.OpUpdateAlbumPicture(c.Ctx, a.Website, al.ID, p.ID, mediaID, alt, caption); err != nil {
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionAlbumPicture, EntityType: "album", EntityID: al.ID,
				Metadata: map[string]any{"updated": p.ID},
			})
			return albumWithPictures(c, d, store, al)
		},
	}
}

func removeAlbumImage(d Deps) Tool {
	return Tool{
		Name:   "remove_album_image",
		Writes: true,
		Description: "Takes one picture out of an album. The file stays in the media library. " +
			"Takes the item id from get_album.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "album": albumProp(),
				"item": {Type: "integer", Description: "item id of the picture in the album"},
			},
			Required: []string{"website", "album", "item"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
				Album   int64 `json:"album"`
				Item    int64 `json:"item"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			_, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.Album)
			if err != nil {
				return nil, err
			}
			if err := store.DeleteItem(c.Ctx, a.Website, al.ID, a.Item); err != nil {
				if errors.Is(err, album.ErrNotFound) {
					return nil, errors.New("there is no such picture in this album; get_album lists the item ids")
				}
				return nil, err
			}
			changed(d, c, a.Website, activity.Entry{
				Action: activity.ActionAlbumPicture, EntityType: "album", EntityID: al.ID,
				Metadata: map[string]any{"removed": a.Item},
			})
			return albumWithPictures(c, d, store, al)
		},
	}
}

func moveAlbumImage(d Deps) Tool {
	return Tool{
		Name:   "move_album_image",
		Writes: true,
		Description: "Changes the order of an album: moves one picture a place up or down, or " +
			"to a position counted from 1. Takes the item id from get_album.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(), "album": albumProp(),
				"item": {Type: "integer", Description: "item id of the picture in the album"},
				"direction": {Type: "string", Description: "one place up (earlier) or down (later)",
					Enum: []string{"up", "down"}},
				"position": {Type: "integer", Description: "the place it should end up at, from 1; " +
					"instead of direction"},
			},
			Required: []string{"website", "album", "item"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website   int64  `json:"website"`
				Album     int64  `json:"album"`
				Item      int64  `json:"item"`
				Direction string `json:"direction"`
				Position  int    `json:"position"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			ops, store, err := albumsOf(d)
			if err != nil {
				return nil, err
			}
			al, err := findAlbum(c, store, a.Website, a.Album)
			if err != nil {
				return nil, err
			}

			// A position is walked one place at a time with the same swap the
			// arrows make: one way of changing the order, not two.
			var steps int
			direction := a.Direction
			switch {
			case a.Position > 0:
				pictures, err := store.Pictures(c.Ctx, a.Website, al.ID)
				if err != nil {
					return nil, err
				}
				from := -1
				for i, p := range pictures {
					if p.ID == a.Item {
						from = i + 1
					}
				}
				if from < 0 {
					return nil, errors.New("there is no such picture in this album; get_album lists the item ids")
				}
				to := min(a.Position, len(pictures))
				direction, steps = "down", to-from
				if steps < 0 {
					direction, steps = "up", -steps
				}
			case direction == "up" || direction == "down":
				if _, err := pictureOf(c, store, a.Website, al.ID, a.Item); err != nil {
					return nil, err
				}
				steps = 1
			default:
				return nil, errors.New("give direction (up or down) or position")
			}

			moved := 0
			for range steps {
				ok, err := ops.OpMoveAlbumPicture(c.Ctx, a.Website, al.ID, a.Item, direction)
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
				moved++
			}
			if moved > 0 {
				changed(d, c, a.Website, activity.Entry{
					Action: activity.ActionAlbumPicture, EntityType: "album", EntityID: al.ID,
					Metadata: map[string]any{"moved": a.Item},
				})
			}
			out, err := albumWithPictures(c, d, store, al)
			if err != nil {
				return nil, err
			}
			if moved == 0 {
				out["note"] = "The order already stood that way."
			}
			return out, nil
		},
	}
}
