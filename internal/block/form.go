package block

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// The block editor is a plain HTML form.
//
// Every field is named "b<n>.<feld>", and a nested entry is "b<n>.e<m>.<feld>".
// The numbers are only there to group and order the fields; they are renumbered
// on every read, so a deleted block leaves no hole and nothing has to keep a
// counter anywhere.
//
// Structural changes — add, delete, move — are ordinary submit buttons carrying
// an action. The handler applies the action and renders the form again instead
// of saving. That works with htmx, which swaps the block list in place, and it
// works with JavaScript switched off entirely, where it is a normal round trip.
// There is no third path where the editor secretly depends on a script.

// ActionField is the name of the button that carries a structural change.
const ActionField = "bausteinaktion"

// Actions. The argument after the colon is a block index, and for the nested
// ones a second index after another colon.
const (
	ActionAdd     = "neu"    // neu:<typ>
	ActionDelete  = "weg"    // weg:<n>
	ActionUp      = "hoch"   // hoch:<n>
	ActionDown    = "runter" // runter:<n>
	ActionAddItem = "neu-e"  // neu-e:<n>
	ActionDelItem = "weg-e"  // weg-e:<n>:<m>
)

// FromForm reads the block list out of a submitted form.
//
// Nothing is validated here beyond shape. An editor mid-edit has half-filled
// blocks all the time, and a parser that refused them would throw away what
// they typed on the way to telling them about it — Clean drops the empty ones
// when the page is actually saved.
func FromForm(form url.Values) []Block {
	type slot struct {
		index int
		block Block
		items map[int]*Item
	}
	slots := map[int]*slot{}

	get := func(n int) *slot {
		s, ok := slots[n]
		if !ok {
			s = &slot{index: n, items: map[int]*Item{}}
			slots[n] = s
		}
		return s
	}

	for key, values := range form {
		if !strings.HasPrefix(key, "b") || len(values) == 0 {
			continue
		}
		rest := key[1:]
		num, rest, ok := cutIndex(rest)
		if !ok {
			continue
		}
		s := get(num)
		value := values[0]

		// A nested field: b3.e1.titel
		if strings.HasPrefix(rest, "e") {
			itemNum, itemRest, ok := cutIndex(rest[1:])
			if !ok {
				continue
			}
			it, ok := s.items[itemNum]
			if !ok {
				it = &Item{}
				s.items[itemNum] = it
			}
			setItemField(it, itemRest, value)
			continue
		}
		setBlockField(&s.block, rest, value)
	}

	order := make([]int, 0, len(slots))
	for n := range slots {
		order = append(order, n)
	}
	sort.Ints(order)

	out := make([]Block, 0, len(order))
	for _, n := range order {
		s := slots[n]
		if s.block.Type == "" {
			continue
		}
		if len(s.items) > 0 {
			nums := make([]int, 0, len(s.items))
			for m := range s.items {
				nums = append(nums, m)
			}
			sort.Ints(nums)
			for _, m := range nums {
				s.block.Items = append(s.block.Items, *s.items[m])
			}
		}
		out = append(out, s.block)
	}
	return out
}

// cutIndex reads the leading number of "3.rest" and returns 3 and "rest".
func cutIndex(s string) (int, string, bool) {
	dot := strings.IndexByte(s, '.')
	if dot <= 0 {
		return 0, "", false
	}
	n, err := strconv.Atoi(s[:dot])
	if err != nil || n < 0 {
		return 0, "", false
	}
	return n, s[dot+1:], true
}

func setBlockField(b *Block, field, value string) {
	// A field of an own kind: b3.f.preis. Under its own prefix so a kind may
	// call a field "typ" or "text" without colliding with the built-in names —
	// the operator picks those words, not us.
	if key, ok := strings.CutPrefix(field, "f."); ok {
		if key == "" {
			return
		}
		if b.Fields == nil {
			b.Fields = map[string]string{}
		}
		b.Fields[key] = value
		return
	}
	switch field {
	case "typ":
		b.Type = value
	case "markdown":
		b.Markdown = value
	case "medium":
		b.MediaID, _ = strconv.ParseInt(value, 10, 64)
	case "vorschaubild":
		b.PosterID, _ = strconv.ParseInt(value, 10, 64)
	case "alt":
		b.Alt = value
	case "bildunterschrift":
		b.Caption = value
	case "variante":
		b.Variant = value
	case "titel":
		b.Title = value
	case "text":
		b.Text = value
	case "quelle":
		b.Source = value
	case "linktext":
		b.LinkText = value
	case "linkziel":
		b.LinkURL = value
	}
}

func setItemField(it *Item, field, value string) {
	switch field {
	case "medium":
		it.MediaID, _ = strconv.ParseInt(value, 10, 64)
	case "alt":
		it.Alt = value
	case "bildunterschrift":
		it.Caption = value
	case "titel":
		it.Title = value
	case "markdown":
		it.Markdown = value
	case "linkziel":
		it.LinkURL = value
	}
}

// Apply performs one structural action and returns the new list.
//
// An action that does not make sense — moving the first block up, deleting an
// index that is not there — is not an error. The button may have been drawn
// against a list that has since changed, and the honest answer to that is the
// list as it now is, not a red message about an index.
func Apply(blocks []Block, action string, s Set) []Block {
	name, arg, _ := strings.Cut(action, ":")
	switch name {
	case ActionAdd:
		if _, ok := s.KindOf(arg); !ok || len(blocks) >= MaxBlocks {
			return blocks
		}
		nb := Block{Type: arg}
		// A gallery or a card row starts with one empty entry: an editor who
		// adds one and sees nothing to fill in has to find a second button
		// before the block does anything at all.
		if k, _ := s.KindOf(arg); k.HasItems {
			nb.Items = []Item{{}}
		}
		return append(blocks, nb)

	case ActionDelete:
		n, ok := index(arg, len(blocks))
		if !ok {
			return blocks
		}
		return append(blocks[:n:n], blocks[n+1:]...)

	case ActionUp:
		n, ok := index(arg, len(blocks))
		if !ok || n == 0 {
			return blocks
		}
		blocks[n-1], blocks[n] = blocks[n], blocks[n-1]
		return blocks

	case ActionDown:
		n, ok := index(arg, len(blocks))
		if !ok || n >= len(blocks)-1 {
			return blocks
		}
		blocks[n], blocks[n+1] = blocks[n+1], blocks[n]
		return blocks

	case ActionAddItem:
		n, ok := index(arg, len(blocks))
		if !ok || len(blocks[n].Items) >= MaxItems {
			return blocks
		}
		blocks[n].Items = append(blocks[n].Items, Item{})
		return blocks

	case ActionDelItem:
		outer, inner, _ := strings.Cut(arg, ":")
		n, ok := index(outer, len(blocks))
		if !ok {
			return blocks
		}
		m, ok := index(inner, len(blocks[n].Items))
		if !ok {
			return blocks
		}
		items := blocks[n].Items
		blocks[n].Items = append(items[:m:m], items[m+1:]...)
		return blocks
	}
	return blocks
}

func index(raw string, length int) (int, bool) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n >= length {
		return 0, false
	}
	return n, true
}

// FromMarkdown is the one-way door out of the plain editor: the page's text
// becomes a single text block, and everything after that is blocks.
//
// One block and not a guess at where the sections are. Splitting on headings
// would look clever and would be wrong about half the time, and an editor who
// finds their article chopped into eight boxes has more work than they started
// with — moving a paragraph into its own block is one button.
func FromMarkdown(md string) []Block {
	if strings.TrimSpace(md) == "" {
		return []Block{{Type: TypeText}}
	}
	return []Block{{Type: TypeText, Markdown: md}}
}

// ToMarkdown reports the page's text if every block is prose, and false if
// anything would be lost by going back to the plain editor.
func ToMarkdown(blocks []Block) (string, bool) {
	var parts []string
	for _, b := range blocks {
		if b.Type != TypeText {
			return "", false
		}
		parts = append(parts, strings.TrimSpace(b.Markdown))
	}
	return strings.Join(parts, "\n\n"), true
}
