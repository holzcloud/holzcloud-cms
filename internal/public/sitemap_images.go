package public

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// maxSitemapImages is the limit the image sitemap format sets per page URL.
const maxSitemapImages = 1000

// pageImages lists the pictures a page shows, for the image sitemap.
//
// Sources, in order: the featured image, every picture of the block list
// (single blocks and the items of galleries and card rows), then the
// /media/<this website>/… references in the rendered HTML. Only images of this
// website count: a block id or a path naming another website's library is
// ignored, as are unknown ids and files that are not images.
func (h *Handler) pageImages(ctx context.Context, website *domain.Website, base string, e *page.SitemapEntry) []sitemapImage {
	if h.mediaStore == nil || e == nil {
		return nil
	}
	var out []sitemapImage
	seen := map[string]bool{}
	add := func(m *media.Media) {
		if m == nil || m.WebsiteID != website.ID || !m.IsImage() || seen[m.Filename] || len(out) >= maxSitemapImages {
			return
		}
		seen[m.Filename] = true
		out = append(out, sitemapImage{
			Loc:     base + "/media/" + strconv.FormatInt(website.ID, 10) + "/" + url.PathEscape(m.Filename),
			Title:   m.AltText,
			Caption: m.Caption,
		})
	}
	byID := func(id int64) {
		if id <= 0 || len(out) >= maxSitemapImages {
			return
		}
		if m, err := h.mediaStore.GetByID(ctx, id); err == nil {
			add(m)
		}
	}

	if e.FeaturedMediaID != nil {
		byID(*e.FeaturedMediaID)
	}
	if e.Blocks != "" {
		// Read as plain data rather than through block.Decode: the block set
		// only decides which types survive cleaning, and a picture of a
		// website's own block type is still a picture of the page.
		var blocks []block.Block
		if err := json.Unmarshal([]byte(e.Blocks), &blocks); err == nil {
			for _, b := range blocks {
				byID(b.MediaID)
				for _, it := range b.Items {
					byID(it.MediaID)
				}
			}
		}
	}
	for _, name := range media.FilenamesIn(website.ID, e.ContentHTML) {
		if len(out) >= maxSitemapImages {
			break
		}
		if m, err := h.mediaStore.GetByFilename(ctx, website.ID, name); err == nil {
			add(m)
		}
	}
	return out
}
