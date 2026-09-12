// The 404 log as a plugin.
//
// It collects which addresses visitors ask for and do not get, how often, and
// where they came from. That is the list you read off which old address needs a
// redirect — and it is exactly the sort of feature that need not be in the
// core: whoever does not want it switches it off.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// entry is one requested address.
//
// The JSON names stay German. They are the shape of every row this plugin has
// ever written into its own storage, so renaming them would not rename the
// stored rows — it would orphan them, and the list an operator has been
// collecting for months would read as empty. .planning/GLOSSARY.md carries the
// rule: a German word that is stored is a value and not an identifier.
type entry struct {
	Path     string `json:"pfad"`
	Count    int    `json:"anzahl"`
	LastSeen string `json:"zuletzt"`
	Referer  string `json:"herkunft,omitempty"`
}

// prefix separates the entries from everything else in this plugin's own
// storage. Stored, for the same reason as the JSON names above.
const prefix = "pfad:"

// maxEntries bounds how many addresses are kept.
//
// A scanner knocks on a thousand addresses that never existed. Without a bound
// the list would be unreadable after a week and the storage full of rubbish
// nobody will ever look at.
const maxEntries = 500

func init() {
	plugin.OnEvent(func(in plugin.EventIn) error {
		if in.Name != plugin.EventNotFound {
			return nil
		}
		path := in.Data["path"]
		if path == "" || len(path) > 300 {
			return nil
		}

		key := prefix + path
		e := entry{Path: path}
		if raw, ok, _ := plugin.Get(key); ok {
			_ = json.Unmarshal([]byte(raw), &e)
		}
		e.Count++
		e.LastSeen = time.Now().UTC().Format(time.RFC3339)
		if referer := in.Data["referer"]; referer != "" && len(referer) < 300 {
			e.Referer = referer
		}

		raw, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err := plugin.Set(key, string(raw)); err != nil {
			return err
		}
		sweep()
		return nil
	})

	plugin.OnAdmin(screen)
}

// sweep throws away the rarest entries when there are too many.
//
// Not the oldest: an address asked for once a month is worth less than one that
// comes daily, even when it is newer.
func sweep() {
	all, err := plugin.List(prefix, 1000)
	if err != nil || len(all) <= maxEntries {
		return
	}
	list := read(all)
	sort.Slice(list, func(i, j int) bool { return list[i].Count < list[j].Count })
	for i := 0; i < len(list)-maxEntries; i++ {
		_ = plugin.Delete(prefix + list[i].Path)
	}
	plugin.Logf("info", "%d selten angefragte Adressen entfernt", len(list)-maxEntries)
}

// read turns the stored rows into entries.
func read(raw map[string]string) []entry {
	out := make([]entry, 0, len(raw))
	for _, v := range raw {
		var e entry
		if json.Unmarshal([]byte(v), &e) == nil && e.Path != "" {
			out = append(out, e)
		}
	}
	return out
}

func screen(in plugin.AdminIn) (plugin.AdminOut, error) {
	if in.Method == "POST" {
		switch {
		case in.Form["alles_loeschen"] != nil:
			all, _ := plugin.List(prefix, 1000)
			for k := range all {
				_ = plugin.Delete(k)
			}
			return plugin.AdminOut{Redirect: ".", Flash: "Liste geleert."}, nil
		case len(in.Form["loeschen"]) > 0:
			_ = plugin.Delete(prefix + in.Form["loeschen"][0])
			return plugin.AdminOut{Redirect: ".", Flash: "Eintrag entfernt."}, nil
		}
	}

	all, err := plugin.List(prefix, 1000)
	if err != nil {
		return plugin.AdminOut{}, err
	}
	list := read(all)
	// Commonest first: that is the address a redirect is most likely worth.
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Path < list[j].Path
	})

	var b strings.Builder
	b.WriteString(plugin.T(`<p>Addresses visitors asked for and did not get. The most frequent stand at the top — those are the ones a redirect is most worth.</p>`))

	if len(list) == 0 {
		b.WriteString(plugin.T(`<p class="empty">Nothing asked for yet that does not exist.</p>`))
		return plugin.AdminOut{Title: "Nicht gefunden", HTML: b.String()}, nil
	}

	b.WriteString(`<table class="table"><thead><tr>` +
		`<th>Adresse</th><th>Anfragen</th><th>Zuletzt</th><th>Herkunft</th><th></th>` +
		`</tr></thead><tbody>`)
	for _, e := range list {
		fmt.Fprintf(&b, `<tr><td><code>%s</code></td><td>%d</td><td>%s</td><td>%s</td>`,
			html.EscapeString(e.Path), e.Count,
			html.EscapeString(shortDate(e.LastSeen)), html.EscapeString(e.Referer))
		// The form posts back to the same address; the host inserts the session
		// key, and the plugin never sees it.
		fmt.Fprintf(&b, `<td><form method="POST"><input type="hidden" name="loeschen" value="%s">`+
			`<button type="submit" class="btn btn--sm">Entfernen</button></form></td></tr>`,
			html.EscapeString(e.Path))
	}
	b.WriteString(`</tbody></table>`)
	fmt.Fprintf(&b, `<p>%s Adressen insgesamt.</p>`, strconv.Itoa(len(list)))
	b.WriteString(`<form method="POST"><input type="hidden" name="alles_loeschen" value="1">` +
		`<button type="submit" class="btn btn--sm btn--danger">Liste leeren</button></form>`)

	return plugin.AdminOut{Title: "Nicht gefunden", HTML: b.String()}, nil
}

// shortDate turns the stored timestamp into something readable.
func shortDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006 15:04")
}

func main() {}
