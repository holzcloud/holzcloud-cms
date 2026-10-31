package bundle

import (
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/block"
)

// The image position of an image-text block is the block's Variant, a free
// string to the archive. Each of the four positions has to come back out of an
// export and an import unchanged.
func TestImagePositionsTravelThroughAnArchive(t *testing.T) {
	set := block.Set{}
	for _, variant := range []string{"", block.ImageRight, block.ImageAbove, block.ImageBelow} {
		in := []block.Block{{
			Type: block.TypeImageText, MediaID: 5, Markdown: "Text", Variant: variant,
		}}
		exported := exportBlocks(in, set, map[int64]string{5: "bild.jpg"}, nil)
		if len(exported) != 1 || exported[0].Variant != variant {
			t.Fatalf("export of %q: %+v", variant, exported)
		}
		back := importBlocks(exported, set, map[string]int64{"bild.jpg": 9}, nil)
		if len(back) != 1 || back[0].Variant != variant {
			t.Errorf("import of %q came back as %+v", variant, back)
		}
	}
}
