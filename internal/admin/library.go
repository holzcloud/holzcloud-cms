package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// Media and albums are one library since v2.6. The files are the library; an
// album is a selection from it, and a file may sit in several. The list on the
// left of all three screens says exactly that: the collections of the files,
// then the albums, each with how much is in it.

// libraryNav is what the collection list draws.
type libraryNav struct {
	WebsiteID int64
	Counts    media.LibraryCounts
	Albums    []libraryAlbum
	// Active names the collection being shown: all, images, videos,
	// documents, unused, noalt — or album:<id>, or albums for the list.
	Active string
}

type libraryAlbum struct {
	ID    int64
	Name  string
	Count int
}

// libraryNavFor gathers the collection list for one website. A failure costs
// the numbers, never the screen.
func (h *Handler) libraryNavFor(ctx context.Context, websiteID int64, active string) libraryNav {
	nav := libraryNav{WebsiteID: websiteID, Active: active}
	if h.mediaStore != nil {
		if c, err := h.mediaStore.Counts(ctx, websiteID); err == nil {
			nav.Counts = c
		} else {
			slog.Warn("library counts", "err", err)
		}
	}
	if h.albumStore != nil {
		albums, err := h.albumStore.List(ctx, websiteID)
		if err != nil {
			slog.Warn("library albums", "err", err)
			return nav
		}
		counts, _ := h.albumStore.ItemCounts(ctx, websiteID)
		for _, a := range albums {
			nav.Albums = append(nav.Albums, libraryAlbum{ID: a.ID, Name: a.Name, Count: counts[a.ID]})
		}
	}
	return nav
}

// activeCollection names what a media list request shows.
func activeCollection(f media.Filter) string {
	switch {
	case f.NoAlt:
		return "noalt"
	case f.Unused:
		return "unused"
	case f.MimePrefix == "image/":
		return "images"
	case f.MimePrefix == "video/":
		return "videos"
	case f.MimePrefix == "application/":
		return "documents"
	}
	return "all"
}

// HandleMediaToAlbum puts the ticked files into one album.
//
// The tick boxes stand in the grid and belong to this form through their form
// attribute: every tile already carries forms of its own, and a form inside a
// form is not HTML.
func (h *Handler) HandleMediaToAlbum(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	back := fmt.Sprintf("/admin/websites/%d/media", websiteID)
	if h.albumStore == nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	albumID, _ := strconv.ParseInt(r.FormValue("album_id"), 10, 64)
	a, err := h.albumStore.Get(r.Context(), websiteID, albumID)
	if err != nil {
		return err
	}
	if a == nil {
		web.SetFlashError(h.sm, r.Context(), "Choose the album the files should go into.")
		return h.redirect(w, r, back)
	}

	var added int
	for _, v := range r.Form["media_id"] {
		mediaID, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		// AddItem checks that the file belongs to this website and is a
		// picture; whatever it refuses is simply not added.
		if _, err := h.albumStore.AddItem(r.Context(), websiteID, a.ID, mediaID, "", ""); err == nil {
			added++
		}
	}
	if added == 0 {
		web.SetFlashWarning(h.sm, r.Context(), "Nothing was added: tick at least one image.")
		return h.redirect(w, r, back)
	}
	web.SetFlashSuccess(h.sm, r.Context(), web.Titlef(r, "%d images put into the album “%s”.", added, a.Name))
	return h.redirect(w, r, fmt.Sprintf("/admin/websites/%d/albums/%d", websiteID, a.ID))
}
