// A plugin that shows how little a plugin has to be.
//
// It replaces [[jahr]] in a page's text with the current year and counts how
// often it has done so. That touches both sides of the interface — the content
// and its own storage — and is still readable in thirty lines.
package main

import (
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// marker is what an author types into a page, so it is content and not an
// identifier: every page that already carries it would stop working if this
// string changed. .planning/GLOSSARY.md carries the rule — a German word that
// is stored is a value, not a name — and this is a value.
const marker = "[[jahr]]"

// countKey is the plugin's own storage key, and the same argument applies: it
// names a row in every installation that has ever run this plugin.
const countKey = "ersetzungen"

// init and not main: the host starts a plugin as a reactor module, which means
// it calls _initialize. That runs the package initialisation and returns —
// main never runs. Writing main here gives you a plugin that installs, switches
// on, and then does nothing at every hook. The SDK says so in the log the first
// time it is called.
func init() {
	plugin.OnContent(func(in plugin.ContentIn) (plugin.ContentOut, error) {
		if !strings.Contains(in.HTML, marker) {
			// Nothing to do is the commonest answer. It costs the host one
			// round trip and no copy of the page.
			return plugin.ContentOut{}, nil
		}
		year := strconv.Itoa(time.Now().Year())
		html := strings.ReplaceAll(in.HTML, marker, year)

		// Keeping count. A failure here must not cost the page: the visitor
		// wants to see the year, not our bookkeeping.
		if n, _, err := plugin.Get(countKey); err == nil {
			count, _ := strconv.Atoi(n)
			_ = plugin.Set(countKey, strconv.Itoa(count+1))
		}
		return plugin.ContentOut{HTML: html, Changed: true}, nil
	})

	plugin.OnAdmin(func(in plugin.AdminIn) (plugin.AdminOut, error) {
		n, _, _ := plugin.Get(countKey)
		if n == "" {
			n = "0"
		}
		return plugin.AdminOut{
			Title: "Jahreszahl",
			HTML: "<p>Schreibe <code>" + marker + "</code> in eine Seite; " +
				"beim Ausliefern steht dort das laufende Jahr.</p>" +
				"<p>Bisher ersetzt: <strong>" + n + "</strong></p>",
		}, nil
	})
}

func main() {}
