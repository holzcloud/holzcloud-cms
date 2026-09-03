package block

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderForDiff turns a stored block list into a stable, readable text.
//
// A page built from blocks is stored as JSON, and JSON compared line by line is
// unreadable: one long line changes wholesale, and the reader learns that
// something moved without learning what. So the blocks are written out as one
// labelled field per line, in a fixed order, and that text is what gets
// compared.
//
// The order is fixed on purpose and comes from a slice, never from ranging a
// map — a rendering whose field order varies between two calls produces
// differences that are not there.
//
// An empty list renders to an empty string, which the comparison reads as
// "there were no blocks" rather than as an error.
func RenderForDiff(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	var blocks []Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return "", fmt.Errorf("bausteine lesen: %w", err)
	}

	var b strings.Builder
	for i, blk := range blocks {
		name := blk.Type
		if k, ok := KindOf(blk.Type); ok {
			name = k.Name
		}
		fmt.Fprintf(&b, "[%d] %s\n", i+1, name)
		for _, kv := range blockFields(blk) {
			// Ein mehrzeiliger Wert wird eingerückt weitergeschrieben, damit
			// eine geänderte Zeile in einem Textbaustein als diese eine Zeile
			// im Vergleich steht und nicht als der ganze Baustein.
			lines := strings.Split(kv[1], "\n")
			fmt.Fprintf(&b, "    %s: %s\n", kv[0], lines[0])
			for _, extra := range lines[1:] {
				fmt.Fprintf(&b, "        %s\n", extra)
			}
		}
	}
	return b.String(), nil
}

// blockFields are the filled fields of one block, in a fixed order and under
// the names an editor sees. Empty fields are left out: a block that never had
// a caption should not carry an empty line for one.
func blockFields(b Block) [][2]string {
	var out [][2]string
	add := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			out = append(out, [2]string{label, value})
		}
	}
	addID := func(label string, id int64) {
		if id != 0 {
			out = append(out, [2]string{label, fmt.Sprintf("#%d", id)})
		}
	}

	add("Variante", b.Variant)
	add("Titel", b.Title)
	add("Text", b.Text)
	add("Quelle", b.Source)
	add("Markdown", b.Markdown)
	addID("Bild", b.MediaID)
	add("Alt", b.Alt)
	add("Bildunterschrift", b.Caption)
	addID("Vorschaubild", b.PosterID)
	add("Linktext", b.LinkText)
	add("Linkziel", b.LinkURL)

	// Die eigenen Felder einer eigenen Baustein-Art. Sortiert, weil eine Map
	// keine Reihenfolge hat und eine wechselnde Reihenfolge als Änderung
	// erschiene.
	for _, key := range sortedKeys(b.Fields) {
		add(key, b.Fields[key])
	}

	for i, item := range b.Items {
		prefix := fmt.Sprintf("Eintrag %d", i+1)
		addID(prefix+" Bild", item.MediaID)
		add(prefix+" Alt", item.Alt)
		add(prefix+" Titel", item.Title)
		add(prefix+" Text", item.Markdown)
		add(prefix+" Bildunterschrift", item.Caption)
		add(prefix+" Linkziel", item.LinkURL)
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Kleine Karten, kurze Schlüssel: ein Einfügesortieren spart hier den
	// Import und ist nicht langsamer.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
