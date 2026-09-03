package bundle

import (
	"context"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/block"
	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// Blocks in a bundle.
//
// A page built with the block editor used to arrive on the other side as plain
// text: the blocks were never written into the archive, and the plain text in
// the markdown column was all that survived. Everything here exists to close
// that — and the whole difficulty is one line long: a picture is stored as an
// id, and an id means nothing on the machine the bundle lands on. So it travels
// as a file name, the same way a page's preview image already did.

// exportBlocks turns a page's stored blocks into what the archive carries.
//
// A picture whose file is not in this export becomes an empty name rather than
// a number: a name that is not there can be reported on import, a number would
// silently point at somebody else's picture.
func exportBlocks(blocks []block.Block, set block.Set, mediaByID map[int64]string) []Block {
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		e := Block{
			Type: b.Type, Markdown: b.Markdown,
			Media: mediaByID[b.MediaID], Poster: mediaByID[b.PosterID],
			Alt: b.Alt, Caption: b.Caption, Variant: b.Variant,
			Title: b.Title, Text: b.Text, Source: b.Source,
			LinkText: b.LinkText, LinkURL: b.LinkURL,
		}
		for _, it := range b.Items {
			e.Items = append(e.Items, BlockItem{
				Media: mediaByID[it.MediaID], Alt: it.Alt, Caption: it.Caption,
				Title: it.Title, Markdown: it.Markdown, LinkURL: it.LinkURL,
			})
		}
		if own, ok := set.OwnOf(b.Type); ok && len(b.Fields) > 0 {
			e.Fields = map[string]string{}
			for _, d := range own.Fields {
				value, filled := b.Fields[d.Key]
				if !filled {
					continue
				}
				// The same rule one level down: an own kind's picture field
				// holds an id too.
				if d.Kind == field.KindImage {
					e.Fields[d.Key] = mediaByID[parseID(value)]
					continue
				}
				e.Fields[d.Key] = value
			}
		}
		out = append(out, e)
	}
	return out
}

// importBlocks turns the archive's blocks back into what is stored, resolving
// every file name to the id it has on this machine.
//
// A name that is not in the media list resolves to zero — no picture — which is
// what a block whose file did not survive the journey should look like: the
// text around it stays, the image is gone, and the report says which file.
func importBlocks(blocks []Block, set block.Set, mediaByName map[string]int64) []block.Block {
	out := make([]block.Block, 0, len(blocks))
	for _, b := range blocks {
		n := block.Block{
			Type: b.Type, Markdown: b.Markdown,
			MediaID: mediaByName[b.Media], PosterID: mediaByName[b.Poster],
			Alt: b.Alt, Caption: b.Caption, Variant: b.Variant,
			Title: b.Title, Text: b.Text, Source: b.Source,
			LinkText: b.LinkText, LinkURL: b.LinkURL,
		}
		for _, it := range b.Items {
			n.Items = append(n.Items, block.Item{
				MediaID: mediaByName[it.Media], Alt: it.Alt, Caption: it.Caption,
				Title: it.Title, Markdown: it.Markdown, LinkURL: it.LinkURL,
			})
		}
		if own, ok := set.OwnOf(b.Type); ok && len(b.Fields) > 0 {
			n.Fields = map[string]string{}
			for _, d := range own.Fields {
				value, filled := b.Fields[d.Key]
				if !filled {
					continue
				}
				if d.Kind == field.KindImage {
					if id, ok := mediaByName[value]; ok {
						n.Fields[d.Key] = formatID(id)
					}
					continue
				}
				n.Fields[d.Key] = value
			}
		}
		out = append(out, n)
	}
	return out
}

// missingMedia names the files a page's blocks point at that the archive did
// not bring, so the report can say so instead of leaving grey boxes.
func missingMedia(blocks []Block, mediaByName map[string]int64) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		if _, ok := mediaByName[name]; ok {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, b := range blocks {
		add(b.Media)
		add(b.Poster)
		for _, it := range b.Items {
			add(it.Media)
		}
		for _, v := range b.Fields {
			// A field value is only a file name when its field is a picture;
			// checking every value would report "3.50" as a missing file. The
			// media list is the filter: a price is not in it, but neither is a
			// missing picture, so only names that look like files are reported.
			if looksLikeFile(v) {
				add(v)
			}
		}
	}
	return out
}

// looksLikeFile is the cheap test that keeps a price out of the missing-files
// report: every media file name in a bundle carries an extension.
func looksLikeFile(v string) bool {
	dot := -1
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] == '.' {
			dot = i
			break
		}
	}
	return dot > 0 && dot < len(v)-1 && len(v)-dot <= 6
}

// parseID and formatID keep the spelling of a number out of the two conversion
// functions above, where the point is which value becomes which.
func parseID(v string) int64 {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }

// blockImages resolves media ids while an import re-renders a page's blocks.
//
// The website check is the same one the editor makes: the id has just been
// looked up in this import's own media list, but a bundle is a file anybody can
// edit, and a picture from another website must not appear on this one.
func blockImages(ctx context.Context, s Stores, websiteID int64) block.Lookup {
	cache := map[int64]block.Image{}
	return func(id int64) (block.Image, bool) {
		if id <= 0 || s.Media == nil {
			return block.Image{}, false
		}
		if img, ok := cache[id]; ok {
			return img, img.URL != ""
		}
		m, err := s.Media.GetByID(ctx, id)
		if err != nil || m == nil || m.WebsiteID != websiteID || !(m.IsImage() || m.IsVideo()) {
			cache[id] = block.Image{}
			return block.Image{}, false
		}
		img := block.Image{
			URL: m.URL(), Alt: m.AltText, Width: m.Width, Height: m.Height,
			Focus: m.FocusCSS(), Film: m.IsVideo(),
		}
		cache[id] = img
		return img, true
	}
}
